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

// ---------- 恢复伪租约(2026-10-04 A2,TOCTOU 加固) ----------
//
// recoverNativeResult 的磁盘 IO(ReadNativeTurn ≤2s / Store.Update 全量重写)在
// activeTurns.mu 之外执行(P1-1),由此产生的"写前重查 Has 与 Store.Update 之间
// 的微秒级窗口"内,StartTurn 可能建起新轮次租约并被恢复的 IDLE 终局覆盖。
// 加固:恢复以「伪租约」身份在 activeTurns 占住该线程的槽位,与真轮次同一张
// 表互斥 —— 不重回"锁内做磁盘 IO"的老路,三个最小追加方法都在本文件
// (thread.go 的 activeTurns 类型定义与既有方法零改动):
//
//   - tryReserveRecovery:槽位空闲则登记伪租约(cancel 为 no-op:恢复不可取消,
//     非 nil 纯为防 acquire 对前任调 cancel 时 panic);
//   - releaseRecovery:完成/失败/异常双路径释放(defer 保证恰好一次);
//   - hasOtherThan:写前重查用,排除自己的伪租约、只认真轮次(第二道防线保留)。
//
// 【为什么 no-op cancel 就能让 StartTurn 自然排队】activeTurns.acquire 对槽内
// 前任先 cancel 再等其 done;伪租约的 cancel 是空操作,StartTurn 随即阻塞在
// 伪租约的 done 上 —— 伪租约释放(releaseRecovery 关闭 done)后 acquire 重新
// 抢槽,自然拿到轮次租约。StartTurn 的等待上界是开场 5s ctx,远大于恢复的
// 2s IO 上界,排队必然等到;真轮次的 cancel 语义(重入打断)不受影响。
//
// 【对既有查询面的瞬时影响(仅恢复在飞的 ≤2s 窗口)】Has/IDs/Cancel 均可见
// 伪租约:DeleteThread 报"会话正在运行请稍后再删"(对"恢复正要落库"的线程
// 本就该拒删,更安全);CancelTurn 空转(恢复写的是已终局原生轮次的真值,
// 无从取消);委派父链推断与 memory housekeeping 把它当活跃轮次 —— 与该线程
// 库内 RUNNING 状态一致,均为良性瞬态。
// 【key 纪律】伪租约与真轮次共用 threadID 槽位(同一张 map):这正是互斥的
// 全部来源;换独立 key 等于放弃互斥,StartTurn 会在恢复写库的微秒窗内插队。

// tryReserveRecovery 以"恢复伪租约"身份登记线程槽位;槽位被占(真轮次在跑/
// 异常残留)返回 false,调用方放弃恢复 —— 与旧实现的 Has 前置检查同语义。
func (a *activeTurns) tryReserveRecovery(threadID string) (*activeTurn, bool) {
	if a == nil {
		return nil, false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.m[threadID] != nil {
		return nil, false
	}
	lease := &activeTurn{cancel: func() {}, done: make(chan struct{})}
	a.m[threadID] = lease
	return lease, true
}

// releaseRecovery 释放恢复伪租约并唤醒在 acquire 里排队的新轮次。
// 幂等安全:lease 不在表里(已被顶掉/重复释放)只跳过摘除,done 仍只关一次
// —— 调用方以 defer 恰好一次释放为契约。
func (a *activeTurns) releaseRecovery(threadID string, lease *activeTurn) {
	if a == nil || lease == nil {
		return
	}
	a.mu.Lock()
	if a.m[threadID] == lease {
		delete(a.m, threadID)
	}
	close(lease.done)
	a.mu.Unlock()
}

// hasOtherThan 报告该线程是否驻留着"非 except"的占用(真轮次租约/异常残留)。
// 恢复的写前重查用它排除自己的伪租约,"活轮次绝不被恢复覆盖"的第二道防线
// 得以在伪租约在位的情况下继续成立。
func (a *activeTurns) hasOtherThan(threadID string, except *activeTurn) bool {
	if a == nil {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	cur := a.m[threadID]
	return cur != nil && cur != except
}

func (s *Server) recoverNativeResult(ctx context.Context, thread *store.Thread) *store.Thread {
	if thread == nil || thread.Status != pb.ThreadStatus_THREAD_STATUS_RUNNING {
		return thread
	}
	reader, ok := s.cfg.Providers[threadAgentID(thread)].(agent.NativeTurnReader)
	if !ok {
		return thread
	}
	// ---------- 单飞闸:同线程并发触发只跑一次 ----------
	// 【认领必须先于活跃租约检查(A2)】恢复伪租约在飞期间,Has 对并发触发也
	// 返回 true;若仍按旧顺序先查 Has,等待者会带着过期 RUNNING 快照直接返回、
	// 绕过结果共享 —— backfillRestartedRunningStamp 等调用方还会拿 RUNNING 快照
	// 回写库,与在飞恢复竞争,恰好造出本文件要消灭的"覆盖/复活"。先认领保证
	// 并发触发永远合并进同一次执行。
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

	// ---------- 锁内只做状态检查(2026-10-04 P1-1) ----------
	// 【为什么锁内只留状态检查】activeTurns.mu 是 StartTurn/CancelTurn/DeleteThread/
	// GetThread/GetThreadHistory 的公共热点锁。旧实现让它贯穿整个恢复函数,锁内做
	// ReadNativeTurn(≤2s 磁盘 IO)、Store.Update(全量重写 threads.json)、
	// recordTurnFrame/emitTurnFinished(事件落盘+广播)→ 并发恢复时 N×2s 锁车队,
	// 开新轮次/取消/删会话的请求全部陪堵。现在锁内只留两次内存判定(无磁盘):
	//   ① 有无活跃租约(Has 自带一次短暂加锁即走)—— 活着的本地 provider 自持
	//      终局,它的轮次绝不能被恢复覆盖(原 Reserve against StartTurn 语义保留);
	//      本 owner 认领时刻不存在伪租约:上任 owner 的伪租约先释放、后 finish
	//      (见 applyNativeResultRecovery 与上面 defer 的排序),Has 只见真轮次;
	//   ② 库内当前态还是不是 RUNNING(FileStore 自带锁,Get 返回快照)。
	// 全部磁盘 IO 与广播移到锁外(applyNativeResultRecovery);并发重复恢复由上面
	// 的单飞闸合并;"恢复 vs 新轮次"互斥由恢复伪租约承担第一道、写前重查
	// (hasOtherThan)保留为第二道(见其注释)。
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
	final = s.applyNativeResultRecovery(ctx, reader, thread, current)
	return final
}

// applyNativeResultRecovery 恢复的执行段:全部磁盘 IO(读原生 journal、落库、
// 事件落盘)与 hostWatch 广播,均不在 activeTurns.mu 内。snapshot 是调用方传入
// 的原始快照(仅落库失败时原样返回),current 是锁内取到的库内最新快照。
func (s *Server) applyNativeResultRecovery(ctx context.Context, reader agent.NativeTurnReader, snapshot, current *store.Thread) *store.Thread {
	// ---------- 恢复伪租约:占住线程槽位直至恢复彻底落定 ----------
	// 注册失败 = 真轮次恰在本函数入口前的无锁窗口建起租约(或异常残留):直接
	// 放弃恢复、按库内最新态返回,活轮次自持终局。注册成功则 StartTurn 一律在
	// acquire 里排队到伪租约释放之后,恢复的读-判定-写全程对真轮次独占。
	marker, ok := s.activeTurns.tryReserveRecovery(current.ID)
	if !ok {
		return current
	}
	// 完成/失败双路径释放:成功、ReadNativeTurn 失败、重查命中放弃、Store.Update
	// 失败、panic 一律经 defer 释放;执行先于单飞闸的 flight.finish(内层 defer
	// 先于外层),保证下一任 owner 认领时槽位已空闲(见 recoverNativeResult 注释)。
	defer s.activeTurns.releaseRecovery(current.ID, marker)

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
	// 【写前重查(第二道防线,保留)】第一道是伪租约:StartTurn 从 ReadNativeTurn
	// 开始就被挡在 acquire 的排队里,到 Store.Update 之后才能拿到租约,微秒级
	// TOCTOU 窗口已闭合。这里兜的是伪租约覆盖不到的极端面(注册前抢跑被入口
	// 拦下之外的异常残留等),hasOtherThan 排除自己的伪租约、只认真轮次;命中即
	// 放弃恢复、按库内最新态返回 —— "must never be overwritten"语义保住。
	if s.activeTurns.hasOtherThan(current.ID, marker) {
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
