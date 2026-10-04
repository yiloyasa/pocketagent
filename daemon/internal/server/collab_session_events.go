// collab_session_events.go —— 协同事件的三层管道(P1 协同闭环 Task 3):
//
//  1. wire 映射:store 的 domain 类型(CollabSession/CollabMember/CollabEvent)
//     ↔ proto 的 CollabSessionSnapshot/CollabMemberSnapshot/CollabSessionEvent。
//     映射是**唯一转换点**(与 collabPolicyToProto 同纪律):字段对照只写一遍,
//     快照、事件、列表三条出口共用,不会出现"快照有、事件里丢"的分叉。
//  2. 成员侧有界事件转换器(collabMemberCard):把 agent 轮次的过程帧
//     (TurnServerMessage)收敛成协同侧的少量进度事件。它坐在 provider 的事件
//     回调链上,**绝不在回调里做文件 IO / 阻塞等待** —— 只更新内存里的有界
//     缓冲,真正的落库与广播交给协同事件队列(单消费者,见 collab_session_state.go)。
//  3. Watch 订阅登记簿(collabWatchers):每订阅者一条有界通道,慢消费者丢弃
//     并计数 —— 事件流带单调 sequence,客户端凭序号缺口触发全量快照兜底
//     (store.ReplayEvents 的 gap 契约 + proto 对 CollabSessionEvent 的注释)。
//
// 【事件帧的权威写者分工(防重复)】状态迁移类事件(ACCEPTED/RUNNING/
// WAITING_APPROVAL/COMPLETED/FAILED/CANCELLED/SNAPSHOT)由**状态机单写者**
// (collabCoordinator 的 transitionRunning/finalizeMember/memberApprovalWaiting)
// 在同步落库之后发 —— 每次状态翻转恰好一帧,绝不伪造。本文件的转换器只负责
// **增量进度**(PROGRESS 帧);审批请求/审批结束的帧在这里被有意跳过:审批
// 挂卡/摘卡的状态迁移已由 approve 钩子的 memberApprovalWaiting 发过权威帧。
package server

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/store"
)

// ---- 事件 kind 词表 ----
//
// 状态迁移事件的 kind 与成员/会话状态字符串同域(ACCEPTED / RUNNING /
// WAITING_APPROVAL / COMPLETED / FAILED / CANCELLED),与
// collab_session_wire_test.go 钉住的词表一致。SNAPSHOT 是 proto 注释里点名
// 的整屏兜底帧;PROGRESS 是纯增量进度帧 —— 属于"词表之外的取值",客户端
// 按协议约定以 snapshot 兜底刷新,不报错(加新帧不破坏旧客户端)。
const (
	collabEventKindProgress = "PROGRESS"
	collabEventKindSnapshot = "SNAPSHOT"
)

// ---- 有界化参数(协同事件管道的容量纪律,见各常量注释)----
const (
	// collabEventQueueSize 事件持久化队列容量(单消费者)。溢出时进度帧直接
	// 丢弃计数,状态帧短等后放弃 —— 状态真值都已同步落库,观察端凭快照兜底。
	collabEventQueueSize = 256
	// collabEventEnqueueWait 重要状态帧在队列满时的入队等待上界。到点放弃,
	// 绝不为一条事件阻塞成员轮次。
	collabEventEnqueueWait = 2 * time.Second
	// collabWatchSubQueue 每个订阅者的事件通道容量(生产者→转发器)。
	collabWatchSubQueue = 64
	// collabWatchOutQueue 每条 Watch 流出站通道容量(转发器→RPC Send)。
	collabWatchOutQueue = 64
	// collabProgressFlushInterval 进度落库节流:进度帧最多每秒一帧,工具边界
	// 处强制刷一帧。原始逐字流不进协同事件(有界、可恢复协议的底线)。
	collabProgressFlushInterval = time.Second
	// collabProgressTextCap 成员卡进度一句话的 rune 上界(展示字段,不是日志)。
	collabProgressTextCap = 160
	// collabTextTailCap 转换器内存里正文尾部的 rune 上界(进度的取材来源)。
	collabTextTailCap = 800
	// collabSummaryCap 成员终稿摘要的 rune 上界(截断走既有 ClipRunesEllipsis)。
	collabSummaryCap = 2000
	// collabErrorCap 成员失败原因的 rune 上界。
	collabErrorCap = 300
	// collabPersistTimeout 单次协同持久化(读改写/追加事件)的上界。
	collabPersistTimeout = 5 * time.Second
)

// ---- 1) wire 映射(domain ↔ pb,唯一转换点)----

// collabSessionToPb 会话快照的纯映射(不查活审批;活审批覆盖在
// collabCoordinator.sessionToPb 里做 —— 那里能拿到 approvalSessions)。
func collabSessionToPb(s store.CollabSession) *pb.CollabSessionSnapshot {
	out := &pb.CollabSessionSnapshot{
		SessionId:       s.ID,
		Instruction:     s.Instruction,
		Status:          string(s.Status),
		Workspace:       s.Workspace,
		CreatedAtUnixMs: s.CreatedAt,
		UpdatedAtUnixMs: s.UpdatedAt,
		Error:           s.Error,
	}
	for _, m := range s.Members {
		out.Members = append(out.Members, collabMemberToPb(m))
	}
	return out
}

// collabMemberToPb 成员卡的纯映射。挂起审批先按持久化引用给最小集
// (request_id 必须真实 —— 那是 SubmitApproval 的应答锚);daemon 重启后
// turnApprovals 已不在内存,这份最小集是仅存的如实痕迹。
func collabMemberToPb(m store.CollabMember) *pb.CollabMemberSnapshot {
	out := &pb.CollabMemberSnapshot{
		AgentId:   m.AgentID,
		RoleId:    m.RoleID,
		ThreadId:  m.ThreadID,
		Status:    string(m.Status),
		ModelId:   m.ModelID,
		Effort:    m.Effort,
		Progress:  m.Progress,
		Summary:   m.Summary,
		Error:     m.Error,
		Workspace: m.Workspace,
	}
	for _, ref := range m.PendingApprovals {
		out.PendingApprovals = append(out.PendingApprovals, &pb.ApprovalRequest{
			RequestId: ref.RequestID,
			Reason:    ref.ToolName,
		})
	}
	return out
}

// collabCopySession / collabCopyMember domain 深拷贝。事件负载在入队时定型
// (队列里的作业不得随后续状态漂移),与 store 的快照纪律同源。
func collabCopySession(in store.CollabSession) store.CollabSession {
	out := in
	if in.Members != nil {
		out.Members = make([]store.CollabMember, len(in.Members))
		for i := range in.Members {
			out.Members[i] = collabCopyMember(in.Members[i])
		}
	}
	return out
}

func collabCopyMember(in store.CollabMember) store.CollabMember {
	out := in
	if in.PendingApprovals != nil {
		out.PendingApprovals = make([]store.CollabApprovalRef, len(in.PendingApprovals))
		copy(out.PendingApprovals, in.PendingApprovals)
	}
	return out
}

// ---- 2) 成员侧有界事件转换器 ----

// collabMemberCard 一个在跑成员的转换器状态(每个成员轮次一张,随轮次生灭)。
// 全部字段有界:textTail 只留尾部 collabTextTailCap 个 rune,progress 每帧
// clip 到 collabProgressTextCap —— 大文本不进协同通道,要看全文去子线程历史。
type collabMemberCard struct {
	c         *collabCoordinator
	sessionID string
	threadID  string
	gen       int

	mu        sync.Mutex
	textTail  string
	toolNote  string
	lastFlush time.Time
}

// sink 返回给 runTurnOnce 的事件汇(经 thinkingRecorder 包裹后思考照常落库)。
func (card *collabMemberCard) sink() agent.Sink { return card.onEvent }

// onEvent 转换器主体:只消费正文增量与工具开始/结束,其余帧见文件头分工注释。
func (card *collabMemberCard) onEvent(ev *pb.TurnServerMessage) {
	if ev == nil {
		return
	}
	switch m := ev.Msg.(type) {
	case *pb.TurnServerMessage_TextDelta:
		card.appendText(m.TextDelta.GetDelta())
		card.maybeFlushProgress(false)
	case *pb.TurnServerMessage_ToolCall:
		card.noteTool(m.ToolCall)
		card.maybeFlushProgress(true) // 工具边界是天然的事件帧点,强制刷
	default:
		// 审批请求/结束:approve 钩子已发权威状态帧,这里不重复;
		// Completed/Failed:终局由 finalizeMember 单写者落库发帧;
		// ThinkingDelta:已被 thinkingRecorder 记走,协同侧不需要。
	}
}

// appendText 只留**尾部**:进度关心的是"最近在说什么",不是全文回放。
func (card *collabMemberCard) appendText(delta string) {
	if delta == "" {
		return
	}
	card.mu.Lock()
	card.textTail = clipRuneTail(card.textTail+delta, collabTextTailCap)
	card.mu.Unlock()
}

func (card *collabMemberCard) noteTool(u *pb.ToolCallUpdate) {
	name := u.GetCall().GetName()
	note := "正在使用工具 " + name
	if u.GetPhase() != pb.ToolCallUpdate_STARTED {
		note = "工具 " + name + " 已结束"
	}
	card.mu.Lock()
	card.toolNote = note
	card.mu.Unlock()
}

// maybeFlushProgress 把当前进度交给事件队列(节流:force=false 时每秒最多一帧)。
// 入队本身非阻塞(队列满则丢进度帧计数),绝不阻塞 provider 的事件回调链。
func (card *collabMemberCard) maybeFlushProgress(force bool) {
	card.mu.Lock()
	if !force && time.Since(card.lastFlush) < collabProgressFlushInterval {
		card.mu.Unlock()
		return
	}
	card.lastFlush = time.Now()
	note := card.toolNote
	text := strings.TrimSpace(card.textTail)
	card.mu.Unlock()

	progress := note
	if progress == "" {
		progress = text
	}
	progress = runtime.ClipRunesEllipsis(progress, collabProgressTextCap)
	if progress == "" {
		return
	}
	card.c.enqueueProgress(card.sessionID, card.threadID, card.gen, progress)
}

// clipRuneTail 保留尾部至多 n 个 rune(与 runtime.ClipRunes 的头部截断互补)。
func clipRuneTail(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[len(r)-n:])
}

// ---- 3) Watch 订阅登记簿 ----

// collabWatchers WatchCollabSession 的内存订阅面。生命周期与 close 语义:
//
//   - 订阅者通道(ch)与出站通道都有界;publish 非阻塞,慢消费者丢帧并计数,
//     客户端凭单调 sequence 的缺口走全量快照兜底(与 ReplayEvents 的 gap 契约
//     同一条规则)—— 绝不为慢客户端阻塞事件管道,更不按事件起 goroutine。
//   - ch 永不 close(生产者只做非阻塞 send,remove 后引用自然消亡),
//     不存在"发已关通道"的竞态;出站通道由 Watch 的转发 goroutine 独占发送,
//     退出时(close(ctx 取消)后)close —— RPC 侧 for-range 自然结束。
type collabWatchers struct {
	mu   sync.Mutex
	next int64
	subs map[int64]*collabWatchSub
}

type collabWatchSub struct {
	id      int64
	ch      chan store.CollabEvent
	dropped atomic.Int64 // 被丢弃的事件数(可观察;客户端凭 gap 自救)
}

func newCollabWatchers() *collabWatchers {
	return &collabWatchers{subs: map[int64]*collabWatchSub{}}
}

func (w *collabWatchers) add() *collabWatchSub {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.next++
	sub := &collabWatchSub{id: w.next, ch: make(chan store.CollabEvent, collabWatchSubQueue)}
	w.subs[sub.id] = sub
	return sub
}

func (w *collabWatchers) remove(id int64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.subs, id)
}

// publish 把一条**已确认落盘**的事件扇给所有订阅者(非阻塞)。
func (w *collabWatchers) publish(ev store.CollabEvent) {
	w.mu.Lock()
	subs := make([]*collabWatchSub, 0, len(w.subs))
	for _, sub := range w.subs {
		subs = append(subs, sub)
	}
	w.mu.Unlock()
	for _, sub := range subs {
		select {
		case sub.ch <- ev:
		default:
			sub.dropped.Add(1)
		}
	}
}
