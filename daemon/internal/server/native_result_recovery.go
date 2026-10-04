package server

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// ---------- 单飞闸(2026-10-04 P1-1) ----------
//
// recoverNativeResult 的磁盘 IO(ReadNativeTurn ≤2s / Store.Update 全量重写)已
// 移出 activeTurns 热点锁,由此产生新窗口:同一个 RUNNING 会话被并发触发恢复
// (列表页 GetThreadHistory + 详情页 GetThread,或两台设备各来一次)会各做一遍
// IO、各写一次库、各广播一次终局。这里按 thread ID 做进程级单飞:先到者执行,
// 后来者等它落定后直接共享结果 —— 与旧实现"在热点锁上排队、轮到自己时读到已
// 恢复的库态"对外语义一致,只是不再重复烧 IO。
//
// 【为什么是包级变量而非 Server 字段】Server 结构体在 server.go;而恢复本身是
// 进程级语义(同一线程的原生 journal 全进程只有一份),包级闸不损失正确性。
// 【key 选择】thread ID 全局唯一 → 不同线程的恢复互不等待,这正是要去掉的
// "N×2s 锁车队";同线程的重复触发才是要合并的对象。
// 【无重入】持有闸期间不会再次进入 recoverNativeResult:执行段只做事件落盘与
// hostWatch 广播,广播是向已注册的 gRPC 流 Send,不会回调 GetThread。
type nativeRecoveryFlight struct {
	done   chan struct{} // 关闭即 result 可见
	result *store.Thread
}

var (
	nativeRecoveryMu      sync.Mutex
	nativeRecoveryFlights = map[string]*nativeRecoveryFlight{}
)

// claimNativeRecovery 返回该线程的在飞恢复:(flight, false)=已有在飞,做等待者;
// (flight, true)=新注册,做执行者(负责恰好一次地调 finish,defer 保证,panic 不漏)。
func claimNativeRecovery(threadID string) (*nativeRecoveryFlight, bool) {
	nativeRecoveryMu.Lock()
	defer nativeRecoveryMu.Unlock()
	if f := nativeRecoveryFlights[threadID]; f != nil {
		return f, false
	}
	f := &nativeRecoveryFlight{done: make(chan struct{})}
	nativeRecoveryFlights[threadID] = f
	return f, true
}

// finish 记录结果、摘除在飞槽并唤醒所有等待者。result 写入先于 close(done),
// 等待者经 channel 关闭的 happens-before 读到它,无数据竞争。
func (f *nativeRecoveryFlight) finish(threadID string, result *store.Thread) {
	nativeRecoveryMu.Lock()
	if nativeRecoveryFlights[threadID] == f {
		delete(nativeRecoveryFlights, threadID)
	}
	f.result = result
	close(f.done)
	nativeRecoveryMu.Unlock()
}

func (s *Server) recoverNativeResult(ctx context.Context, thread *store.Thread) *store.Thread {
	if thread == nil || thread.Status != pb.ThreadStatus_THREAD_STATUS_RUNNING {
		return thread
	}
	reader, ok := s.cfg.Providers[threadAgentID(thread)].(agent.NativeTurnReader)
	if !ok {
		return thread
	}
	// ---------- 锁内只做状态检查(2026-10-04 P1-1) ----------
	// 【为什么锁内只留状态检查】activeTurns.mu 是 StartTurn/CancelTurn/DeleteThread/
	// GetThread/GetThreadHistory 的公共热点锁。旧实现让它贯穿整个恢复函数,锁内做
	// ReadNativeTurn(≤2s 磁盘 IO)、Store.Update(全量重写 threads.json)、
	// recordTurnFrame/emitTurnFinished(事件落盘+广播)→ 并发恢复时 N×2s 锁车队,
	// 开新轮次/取消/删会话的请求全部陪堵。现在锁内只留两次内存判定(无磁盘):
	//   ① 有无活跃租约(Has 自带一次短暂加锁即走)—— 活着的本地 provider 自持
	//      终局,它的轮次绝不能被恢复覆盖(原 Reserve against StartTurn 语义保留);
	//   ② 库内当前态还是不是 RUNNING(FileStore 自带锁,Get 返回快照)。
	// 全部磁盘 IO 与广播移到锁外(applyNativeResultRecovery);并发重复恢复由上面
	// 的单飞闸合并;锁外化让出的"恢复 vs 新轮次"互斥由写前重查租约补回(见其注释)。
	if s.activeTurns.Has(thread.ID) {
		return thread
	}
	current, exists := s.cfg.Store.Get(thread.ID)
	if !exists || current.Status != pb.ThreadStatus_THREAD_STATUS_RUNNING {
		if exists {
			return current
		}
		return thread
	}

	// ---------- 单飞闸:同线程并发触发只跑一次 ----------
	flight, owner := claimNativeRecovery(thread.ID)
	if !owner {
		<-flight.done
		if flight.result != nil {
			return flight.result
		}
		return thread // 执行者异常退出:按自己的快照返回,等待者自身行为不受影响
	}
	final := thread
	defer func() { flight.finish(thread.ID, final) }()
	final = s.applyNativeResultRecovery(ctx, reader, thread, current)
	return final
}

// applyNativeResultRecovery 恢复的执行段:全部磁盘 IO(读原生 journal、落库、
// 事件落盘)与 hostWatch 广播,均不在 activeTurns.mu 内。snapshot 是调用方传入
// 的原始快照(仅落库失败时原样返回),current 是锁内取到的库内最新快照。
func (s *Server) applyNativeResultRecovery(ctx context.Context, reader agent.NativeTurnReader, snapshot, current *store.Thread) *store.Thread {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	result, err := reader.ReadNativeTurn(ctx, current)
	if err != nil || result == nil {
		return current // Unknown is not idle, failed, or complete.
	}
	var blocks []*pb.ContentBlock
	if result.Text != "" {
		blocks = []*pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: result.Text}}}}
		current.Messages = append(current.Messages, store.StoredMessage{
			Role: "assistant", CreatedAt: result.At, Blocks: derefBlocks(blocks),
		})
	}
	current.Status = pb.ThreadStatus_THREAD_STATUS_IDLE
	current.LastTurnError = ""
	if result.Error != nil {
		current.LastTurnError = result.Error.Message
	}
	// 【写前重查活跃租约】锁外化的代价:从 recoverNativeResult 里那次 Has 检查到
	// 现在,StartTurn 可能已为同一线程建起新租约并开始真轮次(旧实现靠整段持锁
	// 天然互斥)。写入前再查一次,把"恢复覆盖活轮次"的窗口从 ≤2s 的整段 IO 压缩
	// 到本条判定与 Store.Update 之间的微秒级间隙;命中即放弃恢复、按库内最新态
	// 返回,让进行中的轮次自持终局 —— "must never be overwritten"语义保住。
	if s.activeTurns.Has(current.ID) {
		if fresh, ok := s.cfg.Store.Get(current.ID); ok {
			return fresh
		}
		return current
	}
	if err := s.cfg.Store.Update(current); err != nil {
		log.Printf("[recovery] 原生终局落库失败 thread=%s: %v", current.ID, err)
		return snapshot
	}
	turnID := result.TurnID
	if turnID == "" {
		turnID = randomID("recovered")
	}
	var frame *pb.TurnServerMessage
	if result.Error != nil {
		frame = &pb.TurnServerMessage{Msg: &pb.TurnServerMessage_Failed{Failed: &pb.TurnFailed{
			TurnId: turnID, Error: result.Error, PartialText: result.Text,
		}}}
	} else {
		frame = &pb.TurnServerMessage{Msg: &pb.TurnServerMessage_Completed{Completed: &pb.TurnCompleted{
			TurnId: turnID, FinalBlocks: blocks,
		}}}
	}
	s.recordTurnFrame(current.ID, turnID, frame)
	s.emitTurnFinished(current.ID, "已从原生 Agent 恢复任务终局")
	log.Printf("[recovery] 已恢复原生终局 thread=%s failed=%t", current.ID, result.Error != nil)
	return current
}

// backfillRestartedRunningStamp 重启后仍处于 RUNNING 的会话,回填轮次起点。
//
// 【为什么记重启时刻而不是留 0/留旧值】真实开始时刻已不可追溯(上一进程的
// 轮次随进程终止),手机端"运行 N 分钟"需要锚点 —— 本次进程的启动时刻是
// 唯一诚实的近似。无条件覆盖(而非仅在 0 时)是刻意的:上一进程留下的时刻
// 描述的是一段已死亡的运行,对"当前还挂着 RUNNING"的展示没有意义。
// 恢复出终局的会话(recoverNativeResult 落为 IDLE)不在此列,不会误盖。
func (s *Server) backfillRestartedRunningStamp(now time.Time) {
	for _, thread := range s.cfg.Store.List(true) {
		if thread.Status != pb.ThreadStatus_THREAD_STATUS_RUNNING {
			continue
		}
		current := s.recoverNativeResult(context.Background(), thread)
		if current.Status != pb.ThreadStatus_THREAD_STATUS_RUNNING {
			continue // 恢复出终局的会话:时刻已无展示意义,不动
		}
		current.TurnStartedUnixMs = now.UnixMilli()
		if err := s.cfg.Store.Update(current); err != nil {
			log.Printf("[recovery] 重启时刻回填失败 thread=%s: %v", current.ID, err)
		}
	}
}
