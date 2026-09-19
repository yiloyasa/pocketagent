// turn_queue.go —— P1-4「运行中继续发消息」的 server 层:插话分派 + 内存队列 + 续跑。
//
// 【两条路径】手机在轮次 running 时又发了一条 TurnInput(走同一条 StartTurn 流):
//  1. provider 支持 Steerer(codex)→ turn/steer 原生插入,Steered 帧回执;
//  2. 插不进(轮次未开始/上游拒绝)或 provider 不支持(echo / OpenAI 兼容)
//     → 入内存队列,Queued 帧回执(第几位);当前轮次终局后由 StartTurn 的
//     续跑循环逐条取出执行,每条发 QueueFlushed + Started。
//
// 【为什么队列只放内存】排队的是"用户下一条消息",不是任务结果:终局回复照常
// 落库,daemon 重启丢掉的只是"还没轮到的那几句",手机端重新发送即可。
// 给队列加持久化要引入落盘时序与去重问题,换来的收益不成比例 —— 这是一个
// 明确接受取舍的能力,注释即契约。
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

// turnQueueLimit 每线程排队上限。到顶后新输入直接拒收(Queued.position=0),
// 手机端把该消息标成"未送达" —— 宁可明确拒绝,不悄悄丢。
const turnQueueLimit = 10

// queuedInput 一条排队中的输入(只保留执行所需的最小字段)。
type queuedInput struct {
	blocks  []pb.ContentBlock
	modelID string // 该条自带的模型/强度覆盖(空 = 沿用),执行时才生效
	effort  string
}

// turnInputQueue 每线程 FIFO。由 StartTurn 的读手与续跑循环并发访问,全程持锁。
type turnInputQueue struct {
	mu sync.Mutex
	q  map[string][]queuedInput
}

func newTurnInputQueue() *turnInputQueue {
	return &turnInputQueue{q: map[string][]queuedInput{}}
}

// Enqueue 入队;返回 (position, depth)。position 从 1 起;
// 队列满返回 (0, 0) 表示拒收。
func (q *turnInputQueue) Enqueue(threadID string, in queuedInput) (int, int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	cur := q.q[threadID]
	if len(cur) >= turnQueueLimit {
		return 0, 0
	}
	cur = append(cur, in)
	q.q[threadID] = cur
	return len(cur), len(cur)
}

// Pop 出队首;队列空返回 false。
func (q *turnInputQueue) Pop(threadID string) (queuedInput, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	cur := q.q[threadID]
	if len(cur) == 0 {
		return queuedInput{}, false
	}
	head := cur[0]
	rest := make([]queuedInput, len(cur)-1)
	copy(rest, cur[1:])
	if len(rest) == 0 {
		delete(q.q, threadID)
	} else {
		q.q[threadID] = rest
	}
	return head, true
}

// Len 当前队列长度(诊断/回执用)。
func (q *turnInputQueue) Len(threadID string) int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.q[threadID])
}

// Clear 清空该线程的队列(用户取消时:终止任务 = 连排队的也不跑了)。
func (q *turnInputQueue) Clear(threadID string) {
	q.mu.Lock()
	delete(q.q, threadID)
	q.mu.Unlock()
}

// turnRuntime 当前轮次的运行时可变信息,供读手 goroutine 查询。
//
// 【为什么单独一层】StartTurn 的主 goroutine 在续跑循环里会更新 turnID 与
// provider(排队输入可能换模型 → 换 provider),而读手 goroutine 在同时收帧。
// 两边都只经它读写,避免数据竞争。
type turnRuntime struct {
	mu      sync.Mutex
	closed  bool
	turnID  string
	steerer agent.Steerer // nil = 当前 provider 不支持插话
}

// Admission and the final empty-queue check share one lock. Otherwise a reader
// could acknowledge an input just after StartTurn decided there was no work.
func (r *turnRuntime) next(q *turnInputQueue, id string) (queuedInput, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	in, ok := q.Pop(id)
	if !ok {
		r.closed = true
	}
	return in, ok
}

func (r *turnRuntime) stop(q *turnInputQueue, id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	q.Clear(id)
}

func (r *turnRuntime) update(turnID string, provider agent.Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.turnID = turnID
	st, _ := provider.(agent.Steerer)
	r.steerer = st
}

func (r *turnRuntime) current() (string, agent.Steerer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.turnID, r.steerer
}

// steerCallBudget server 层给单次 steer 的上界(provider 内部还有默认兜底,
// 这里统一再收一次口:读手 goroutine 里排队等待不能没边)。
const steerCallBudget = 15 * time.Second

// handleMidTurnInput 处理「轮次 running 时收到的 TurnInput」帧(读手 goroutine 调用)。
//
// 顺序:先试 codex 原生插入(Steered 回执),插不进转排队(Queued 回执,
// position=0 表示队列满拒收)。任何路径都不丢输入、也不阻塞读手
// (读手阻塞 = 审批/取消帧全部堵住,是 P0 级故障)。
func (s *Server) handleMidTurnInput(t *store.Thread, in *pb.TurnInput, rt *turnRuntime, sink agent.Sink) {
	if in == nil {
		return
	}
	if in.ThreadId != "" && in.ThreadId != t.ID {
		// 手机端不会这么做(同一会话页只发本会话 id);防御性忽略并留痕。
		log.Printf("[turn] 运行中收到异线程输入 %s(当前 %s),忽略", in.ThreadId, t.ID)
		return
	}
	blocks := derefBlocks(in.GetBlocks())
	if firstUserText(blocks) == "" && !hasVisualBlocks(blocks) {
		return // 空输入与 UI 侧 isBlank 同语义:不插话、不排队
	}
	turnID, steerer := rt.current()
	// 1) codex 原生插入:成功即回 Steered,事件继续走同一条流(不新开轮次)
	if steerer != nil {
		sctx, cancel := context.WithTimeout(context.Background(), steerCallBudget)
		steered, codexTurnID, err := steerer.Steer(sctx, t.ID, blocks)
		cancel()
		if steered {
			log.Printf("[turn] 运行中输入已插入 codex 轮次 %s(codex turn=%s)", turnID, codexTurnID)
			sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_Steered{Steered: &pb.Steered{
				TurnId: turnID, CodexTurnId: codexTurnID,
			}}})
			return
		}
		// 插不进不是错误:轮次可能还没被 codex 接收(turn/started 未到)、
		// 或上游明确拒绝(expectedTurnId 不匹配)。排队兜底,输入不丢。
		log.Printf("[turn] steer 不可用,转排队 %s: %v", t.ID, err)
	}
	// 2) 排队:当前轮次终局后由续跑循环逐条执行
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.closed {
		return
	}
	pos, depth := s.turnQueue.Enqueue(t.ID, queuedInput{
		blocks:  blocks,
		modelID: in.GetModelId(),
		effort:  in.GetEffort(),
	})
	if pos == 0 {
		log.Printf("[turn] 队列已满(上限 %d),拒收 %s 的新输入", turnQueueLimit, t.ID)
	} else {
		log.Printf("[turn] 运行中输入已排队 %s:第 %d 位(共 %d)", t.ID, pos, depth)
	}
	sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_Queued{Queued: &pb.Queued{
		Position: int32(pos), Depth: int32(depth),
	}}})
}

// hasVisualBlocks 输入里是否带图片等非文本块(纯图片输入也应被接收)。
func hasVisualBlocks(blocks []pb.ContentBlock) bool {
	for i := range blocks {
		if _, ok := blocks[i].Block.(*pb.ContentBlock_Text); !ok {
			return true
		}
	}
	return false
}
