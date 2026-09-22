// turn_queue.go —— P1-4「运行中继续发消息」的 server 层:插话分派 + 内存队列 + 续跑。
//
// 【两条路径】手机在轮次 running 时又发了一条 TurnInput(走同一条 StartTurn 流):
//  1. provider 支持 Steerer(codex)→ turn/steer 原生插入,Steered 帧回执;
//  2. 插不进(轮次未开始/上游拒绝)或 provider 不支持(echo / OpenAI 兼容)
//     → 入内存队列,Queued 帧回执(第几位);当前轮次终局后由 StartTurn 的
//     续跑循环逐条取出执行,每条发 QueueFlushed + Started。
//
// 【为什么队列仍只放内存,但丢失不再静默】排队的是"用户下一条消息",不是任务
// 结果:终局回复照常落库,daemon 重启丢掉的只是"还没轮到的那几句"。
// 原决策据此判断"给队列加持久化要引入落盘时序与去重问题,收益不成比例" ——
// 取舍本身没错,错在它默认"丢掉"可以无声:重启后 daemon 一言不发,手机端与
// 用户都以为那几条还在排队,分不清是真丢还是假丢。
// 于是决策演进为:**要不要持久化队列**仍否(**绝不自动重发** —— 重放一条可能
// 已带副作用的 prompt 是规格 §8.3 明令禁止的),**要不要让丢失可见**是。
// 落地在 turn_journal.go:入队记一条、出队划掉、启动时把没划掉的汇总报告再
// 清空。队列本身照旧随进程消亡,只是"消亡了什么"不再无声。
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
	// id 是这条输入在持久化台账(turn_journal.go)里的键:入队时由调用方生成、
	// 记一条账,出队执行时凭它划掉。仅内存模式(无 dataDir)下也照样生成,
	// 行为零分叉。
	id      string
	blocks  []pb.ContentBlock
	modelID string // 该条自带的模型/强度覆盖(空 = 沿用),执行时才生效
	effort  string
}

// turnInputQueue 每线程 FIFO。由 StartTurn 的读手与续跑循环并发访问,全程持锁。
type turnInputQueue struct {
	mu      sync.Mutex
	q       map[string][]queuedInput
	journal *turnJournal // 排队输入的持久化台账(让重启造成的丢失可见)
}

func newTurnInputQueue(journal *turnJournal) *turnInputQueue {
	return &turnInputQueue{q: map[string][]queuedInput{}, journal: journal}
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

// Clear 清空该线程的队列(用户取消时:终止任务 = 连排队的也不跑了),返回被
// 清掉的条目 id,供调用方一并划掉台账 —— 这些输入是被"明确丢弃"的(用户取消 /
// 轮次异常收尾),不再是"排队中",否则下次重启会把它们误报成"随重启丢失"。
func (q *turnInputQueue) Clear(threadID string) []string {
	q.mu.Lock()
	defer q.mu.Unlock()
	cur := q.q[threadID]
	delete(q.q, threadID)
	if len(cur) == 0 {
		return nil
	}
	ids := make([]string, 0, len(cur))
	for _, in := range cur {
		if in.id != "" {
			ids = append(ids, in.id)
		}
	}
	return ids
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
	in, ok := q.Pop(id)
	if !ok {
		r.closed = true
	}
	r.mu.Unlock()
	// 台账出账放在 rt.mu 之外:这把锁与 admission(handleMidTurnInput)共用,
	// 临界区里绝不能落盘(读手阻塞 = P0 级故障)。此刻队列状态已在锁内定稿,
	// 之后的磁盘 I/O 不影响上面"最后一次空队列判定"的原子性,并发语义不变。
	if ok && q.journal != nil {
		q.journal.Remove(in.id)
	}
	return in, ok
}

func (r *turnRuntime) stop(q *turnInputQueue, id string) {
	r.mu.Lock()
	r.closed = true
	cleared := q.Clear(id)
	r.mu.Unlock()
	// 台账出账放在 rt.mu 之外(理由同 next())。被 stop 清掉的输入是本轮内被
	// "明确丢弃"的(用户取消 / 轮次异常收尾),不再算"排队中",否则下次重启会把
	// 它们误报成"随重启丢失"。
	if q.journal != nil {
		for _, cid := range cleared {
			q.journal.Remove(cid)
		}
	}
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
	item := queuedInput{
		id:      randomID("qin"),
		blocks:  blocks,
		modelID: in.GetModelId(),
		effort:  in.GetEffort(),
	}
	// 【先记账,再入队】台账必须先于内存入队落盘:否则 next() 可能在 Record 之前
	// 就把这条 pop 走并出账,留下一条永不消解的僵尸条目(下次启动误报)。Record
	// 刻意放在 rt.mu 之外 —— 这把锁与 admission/next 共用,临界区只碰内存。
	s.turnJournal.Record(item.id, t.ID, firstUserText(blocks))
	pos, _, accepted := s.enqueueMidTurn(rt, t.ID, item, sink)
	if !accepted || pos == 0 {
		// 轮次已结束(未入队)或队列满拒收:撤销刚记的账,不留"其实没在排队"的
		// 条目。同样放在锁外做,不把磁盘 I/O 塞进临界区。
		s.turnJournal.Remove(item.id)
	}
}

// enqueueMidTurn 在 rt.mu 保护下把一条输入放进内存队列并回 Queued 帧。
// 单独成函数只为把临界区收在一处(defer 解锁,panic 也安全),且临界区内只碰
// 内存、绝不落盘 —— 这把锁与 admission 共用,读手阻塞是 P0 级故障。
// accepted=false 表示轮次已结束(rt.closed):未入队、也不回帧,与原先一致。
func (s *Server) enqueueMidTurn(rt *turnRuntime, threadID string, item queuedInput, sink agent.Sink) (pos, depth int, accepted bool) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.closed {
		return 0, 0, false
	}
	pos, depth = s.turnQueue.Enqueue(threadID, item)
	if pos == 0 {
		log.Printf("[turn] 队列已满(上限 %d),拒收 %s 的新输入", turnQueueLimit, threadID)
	} else {
		log.Printf("[turn] 运行中输入已排队 %s:第 %d 位(共 %d)", threadID, pos, depth)
	}
	sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_Queued{Queued: &pb.Queued{
		Position: int32(pos), Depth: int32(depth),
	}}})
	return pos, depth, true
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
