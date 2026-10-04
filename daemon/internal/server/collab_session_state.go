// collab_session_state.go —— 协同协调器(CollabCoordinator)与状态机
// (P1 协同闭环 Task 3/4 的单一写者)。
//
// 【它是什么】协同会话的**唯一状态机权威**:DispatchCollab 在这里从"派发后
// 撒手"升级为"创建持久化会话 + 逐成员受理";审批/取消/重试在这里落到成员卡。
// 所有成员卡与会话状态都经由本文件的有界写循环落库,迟到的旧轮次写入被
// 代际闸(generation)+ 终局闸(终局不复活)+ store 的 UpdatedAt 世代校验
// 三层挡住 —— 宁可拒一次迟到写,也不让状态倒退。
//
// 【状态机(文字版)】
//
//	会话:  ACCEPTED ──(有成员起飞)──▶ RUNNING ──(全员终局)──▶ COMPLETED/FAILED/CANCELLED
//	                 ▲                    │
//	                 │                    └─(有成员停在审批且无人仍在跑)─▶ WAITING_APPROVAL
//	   (RetryMember 把终局成员拉回 ACCEPTED,会话随之回到 RUNNING)
//
//	成员:  ACCEPTED → RUNNING → COMPLETED | FAILED | CANCELLED
//	                     │        ▲
//	                     └─ WAITING_APPROVAL(工具审批挂起;应答后回 RUNNING)
//
//	会话级状态由成员卡**派生**(deriveCollabSessionStatus),不独立记账:
//	任何 ACCEPTED/RUNNING 成员在跑 → RUNNING;否则有 WAITING_APPROVAL →
//	WAITING_APPROVAL(一个成员等审批时,其他成员继续跑,会话绝不误报完成);
//	全员终局时按 FAILED > CANCELLED > COMPLETED 的优先级聚合(任一失败即整体
//	如实标注 FAILED,成员卡上保留各自真相)。
//
// 【终局语义(红线)】COMPLETED 只能由真实终局(runTurnOnce 正常返回的终稿)
// 写入;CANCELLED 由取消指令经成员轮次自己的收尾路径写入(协调器只发取消、
// 不代写状态);WAITING_APPROVAL 不是终局;受理(ACCEPTED)永远不写成 DONE。
//
// 【代际(generation)】每个成员一条运行代际,内存记账(daemon 进程内迟到的
// 回调只可能来自本进程;重启后旧轮次随进程消亡,无需持久化)。重试 = gen+1:
// 旧代际的一切回调(终局/审批/进度)在此被拒,新轮次的结果不与旧轮次混写;
// 旧结果保留在事件流(append-only)与子线程历史里,不覆盖。
package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/store"
)

// CollabCoordinator 协同会话协调器的对外窄接口(计划 Task 3 规格)。
// RPC 层(DispatchCollab/GetCollabSession/WatchCollabSession/Cancel*/Retry*)
// 只认它;实现绑定 *Server 与一个 CollabSessionStore,测试可整只替换。
type CollabCoordinator interface {
	// Dispatch 创建会话并逐成员受理:受理成功写 ACCEPTED→RUNNING(绝不写
	// DONE),受理失败写 FAILED+人话原因;返回当前全量快照。
	Dispatch(ctx context.Context, req DispatchRequest) (*pb.CollabSessionSnapshot, error)
	// Snapshot 返回会话全量快照(挂起审批映射到成员卡)。
	Snapshot(ctx context.Context, sessionID string) (*pb.CollabSessionSnapshot, error)
	// Watch 先回放 store 事件(after 之后),再转发后续实时事件;通道有界、
	// 慢消费者丢弃、ctx 取消时关闭。事件以指针传递(pb 消息不可按值拷贝,
	// 与全仓 vet 纪律一致)。
	Watch(ctx context.Context, sessionID string, after int64) (<-chan *pb.CollabSessionEvent, error)
	// CancelSession 逐成员发起取消并等状态收敛(有界等待,超时给人话错误)。
	CancelSession(ctx context.Context, sessionID string) (*pb.CollabSessionSnapshot, error)
	// CancelMember 只取消对应成员的子线程(复用 CancelTurn 链路)。
	CancelMember(ctx context.Context, sessionID, threadID string) (*pb.CollabMemberSnapshot, error)
	// RetryMember 以新代际重建成员轮次;旧结果保留在事件流/线程历史里。
	RetryMember(ctx context.Context, sessionID, threadID string) (*pb.CollabMemberSnapshot, error)
}

// DispatchRequest 一次协同扇出的受理请求。成员的角色/agent/模型解析已在 RPC
// 层完成(resolveCollabMemberSpec),协调器只做"建会话、建线程、登记、起飞"。
type DispatchRequest struct {
	Instruction string
	Workspace   string
	Members     []DispatchMember
}

// DispatchMember 单个成员的受理材料。AcceptError 非空 = 受理前就注定失败
// (角色未定义/未指定 agent),协调器如实落一张 FAILED 成员卡,不建线程。
type DispatchMember struct {
	AgentID     string
	RoleID      string
	ModelID     string
	Effort      string
	Workspace   string
	Task        string
	AcceptError string
}

const (
	// collabCancelConvergenceWait CancelSession 等全员收敛的有界等待。
	// 超时降级为人话错误,绝不永久等(RPC 不被卡死,状态仍会自行收敛)。
	collabCancelConvergenceWait = 10 * time.Second
	// collabCancelPollInterval 收敛轮询间隔(本地文件读,足够廉价)。
	collabCancelPollInterval = 50 * time.Millisecond
	// collabUpdateRetries 撞上 ErrCollabStaleUpdate 时的重读重试上界。
	collabUpdateRetries = 4
	// collabApprovalWaitTimeout 成员审批等待上界(与 StartTurn 的审批应答
	// 上界同口径)。超时按拒绝继续 —— 协同成员无人值守,不能为一笔回不了
	// 的审批把整轮吊死。
	collabApprovalWaitTimeout = 2 * time.Minute
)

// collabCoordinators 引擎单例登记表:键 *Server → *collabCoordinator。
// 与 collabEngines 同款(生产一个 Server 一个实例;测试各建各的,互不串扰)。
var collabCoordinators sync.Map

var _ CollabCoordinator = (*collabCoordinator)(nil)

// collabCoordinator CollabCoordinator 的唯一实现。
type collabCoordinator struct {
	srv   *Server
	store store.CollabSessionStore

	// runs 成员运行代际登记(key = sessionID + "\x00" + threadID)。
	// 单协调器内的写者:Dispatch/RetryMember(增/换代)与 runCollabMember 的
	// 回调(读)。mu 保护。
	mu   sync.Mutex
	runs map[string]*collabRunState

	watchers *collabWatchers
	// evCh 协同事件管道(有界)→ 单消费者 goroutine 落盘 + 扇出。
	// 【为什么不每事件一个 goroutine】总约束:协同事件必须有界、可取消、
	// 可恢复;无界派发会在慢盘/事件风暴下堆积出不可控并发。单消费者天然
	// 保序,慢了就丢进度帧(可观察),状态真值始终已同步落库。
	evCh   chan collabEventJob
	evOnce sync.Once

	// 丢弃/失败记账(可观察;绝不伪造完成的另一半:丢了什么要说出来)。
	droppedProgress atomic.Int64
	droppedEvents   atomic.Int64
	persistFailures atomic.Int64
}

type collabRunState struct {
	gen       int
	cancelled bool // CancelMember/CancelSession 已发起(受理-起飞窗口的取消竞态闸)
}

// collabEventJob 事件队列的一件活。event 非空 = 追加事件帧;progress 非空 =
// 成员进度落库(+PROGRESS 帧)。important 标记决定队列满时的待遇(丢弃 vs 短等)。
type collabEventJob struct {
	important bool
	event     *store.CollabEvent
	progress  *collabProgressJob
}

type collabProgressJob struct {
	sessionID string
	threadID  string
	gen       int
	text      string
}

// ---- 构造与注入 ----

// newCollabCoordinator 显式注入 store 的构造器(测试用;生产走 collabCoordinatorFor)。
func newCollabCoordinator(srv *Server, cs store.CollabSessionStore) *collabCoordinator {
	return &collabCoordinator{
		srv:      srv,
		store:    cs,
		runs:     map[string]*collabRunState{},
		watchers: newCollabWatchers(),
		evCh:     make(chan collabEventJob, collabEventQueueSize),
	}
}

// collabCoordinatorFor 返回绑定该 Server 的单例协同协调器(与 collabEngineFor
// 同款惰性单例)。store 的选择:DataDir 非空 → FileCollabSessionStore(持久化、
// 重启可恢复);为空(测试/极简构造)→ 内存实现,与 newTurnJournal 的退化约定
// 同一条。打开失败退内存并醒目告警 —— 数据坏 ≠ 服务死,但必须说出来。
func (s *Server) collabCoordinatorFor() *collabCoordinator {
	if v, ok := collabCoordinators.Load(s); ok {
		return v.(*collabCoordinator)
	}
	var cs store.CollabSessionStore
	if s.cfg.DataDir != "" {
		if fcs, err := store.NewFileCollabSessionStore(s.cfg.DataDir); err != nil {
			log.Printf("[collab] WARNING: 协同会话库打开失败,本进程退化为内存模式(重启后协同历史不可恢复): %v", err)
		} else {
			cs = fcs
			if fcs.LoadOutcome() == store.LoadOutcomeQuarantined {
				log.Printf("[collab] WARNING: 协同持久化数据部分损坏,已隔离留档并以空库起步: %v", fcs.QuarantinePaths())
			} else {
				log.Printf("[collab] 协同会话库已打开(%s) dataDir=%s", fcs.LoadOutcome(), s.cfg.DataDir)
			}
		}
	}
	if cs == nil {
		cs = newMemoryCollabSessionStore()
	}
	v, _ := collabCoordinators.LoadOrStore(s, newCollabCoordinator(s, cs))
	return v.(*collabCoordinator)
}

// ---- Dispatch:创建会话 + 逐成员受理 ----

func (c *collabCoordinator) Dispatch(ctx context.Context, req DispatchRequest) (*pb.CollabSessionSnapshot, error) {
	if strings.TrimSpace(req.Instruction) == "" {
		return nil, errInvalidArg("协同指令不能为空:请在协同页输入要交给这些 agent 的任务")
	}
	if len(req.Members) == 0 {
		return nil, errInvalidArg("至少要选一个参与协同的 agent:请先勾选本次要用哪些 agent")
	}
	sessionID := randomID("collab")
	sess := store.CollabSession{
		ID:          sessionID,
		Instruction: req.Instruction,
		Status:      store.CollabStatusAccepted, // 受理 ≠ 完成:先立"已受理"的牌
		Workspace:   req.Workspace,
	}
	pctx, cancel := context.WithTimeout(context.Background(), collabPersistTimeout)
	defer cancel()
	if err := c.store.CreateSession(pctx, sess); err != nil {
		return nil, errInternal(err)
	}
	// 会话级首帧(快照定格在本时刻的空成员形态,入队即深拷贝)。
	snap := collabCopySession(sess)
	c.enqueueJob(collabEventJob{important: true, event: &store.CollabEvent{
		SessionID: sessionID,
		Kind:      string(store.CollabStatusAccepted),
		Snapshot:  &snap,
	}})
	for _, spec := range req.Members {
		c.acceptMember(ctx, sessionID, spec)
	}
	final, ok := c.store.GetSession(ctx, sessionID)
	if !ok {
		return nil, errNotFound("协同会话刚创建就读不到了(存储异常)")
	}
	return c.sessionToPb(final), nil
}

// acceptMember 受理单个成员:建子线程 → 登记 ACCEPTED 成员卡 → 脱离取消链起飞。
// 任何一步失败都落一张 FAILED 成员卡(人话原因),不影响其它成员 —— "各自去干"
// 的并行协作语义。
func (c *collabCoordinator) acceptMember(ctx context.Context, sessionID string, spec DispatchMember) {
	if spec.AcceptError != "" {
		c.appendMember(ctx, sessionID, store.CollabMember{
			AgentID:   spec.AgentID,
			RoleID:    spec.RoleID,
			Status:    store.CollabStatusFailed,
			Error:     spec.AcceptError,
			ModelID:   spec.ModelID,
			Effort:    spec.Effort,
			Workspace: spec.Workspace,
		})
		return
	}
	// 建子线程:复用 CreateThread —— agent 不存在的人话报错、动态列举当前可用
	// agent,单一裁决点在 CreateThread,这里不重复做存在性裁决。
	th, err := c.srv.CreateThread(ctx, &pb.CreateThreadRequest{
		AgentId:   spec.AgentID,
		Workspace: spec.Workspace,
		ModelId:   spec.ModelID,
	})
	if err != nil {
		c.appendMember(ctx, sessionID, store.CollabMember{
			AgentID: spec.AgentID, RoleID: spec.RoleID,
			Status: store.CollabStatusFailed, Error: err.Error(),
			ModelID: spec.ModelID, Effort: spec.Effort, Workspace: spec.Workspace,
		})
		return
	}
	t, ok := c.srv.cfg.Store.Get(th.GetId())
	if !ok {
		c.appendMember(ctx, sessionID, store.CollabMember{
			AgentID: spec.AgentID, RoleID: spec.RoleID, ThreadID: th.GetId(),
			Status: store.CollabStatusFailed, Error: "子线程建出来后读不回来(存储异常),本次协同未起飞",
			ModelID: spec.ModelID, Effort: spec.Effort, Workspace: spec.Workspace,
		})
		return
	}
	t.Effort = spec.Effort
	// 【为什么不设 ParentThreadID】协同会话 id 不是一个真实线程,挂上去会造出
	// 一条指向不存在的父链(沿袭既有红线);归组由前端的 session_id 负责。
	if err := c.srv.cfg.Store.Update(t); err != nil {
		c.appendMember(ctx, sessionID, store.CollabMember{
			AgentID: spec.AgentID, RoleID: spec.RoleID, ThreadID: t.ID,
			Status: store.CollabStatusFailed, Error: err.Error(),
			ModelID: t.ModelID, Effort: spec.Effort, Workspace: t.Workspace,
		})
		return
	}
	// 受理成功:先登记代际(gen=1),再落 ACCEPTED 成员卡,然后脱离 RPC 取消链起飞。
	gen := c.nextGeneration(sessionID, t.ID)
	c.appendMember(ctx, sessionID, store.CollabMember{
		AgentID:   spec.AgentID,
		RoleID:    spec.RoleID,
		ThreadID:  t.ID,
		Status:    store.CollabStatusAccepted,
		ModelID:   t.ModelID,
		Effort:    spec.Effort,
		Workspace: t.Workspace,
	})
	go c.srv.runCollabMember(sessionID, t.ID, spec.Task, gen)
}

// appendMember 向会话追加一张成员卡并派发对应事件(受理成功 = ACCEPTED 帧,
// 受理失败 = FAILED 帧;绝不把受理写成 DONE)。
func (c *collabCoordinator) appendMember(ctx context.Context, sessionID string, m store.CollabMember) {
	written := c.updateSessionRetry(ctx, sessionID, func(sess *store.CollabSession) bool {
		sess.Members = append(sess.Members, m)
		sess.Status = deriveCollabSessionStatus(sess)
		applySessionError(sess)
		return true
	})
	if !written {
		return
	}
	card := collabCopyMember(m)
	c.enqueueJob(collabEventJob{important: true, event: &store.CollabEvent{
		SessionID:      sessionID,
		MemberThreadID: card.ThreadID,
		Kind:           string(card.Status),
		Member:         &card,
	}})
}

// transitionRunning 成员轮次真正起飞(拿到租约、选好 provider)时把成员置为
// RUNNING —— ACCEPTED→RUNNING 是受理与执行的分界线。
func (c *collabCoordinator) transitionRunning(sessionID, threadID string, gen int) {
	ctx, cancel := context.WithTimeout(context.Background(), collabPersistTimeout)
	defer cancel()
	var card store.CollabMember
	changed := false
	written := c.updateSessionRetry(ctx, sessionID, func(sess *store.CollabSession) bool {
		i := collabMemberIndex(sess, threadID)
		if i < 0 {
			return false
		}
		m := &sess.Members[i]
		if !c.ownsGeneration(sessionID, threadID, gen) || store.CollabStatusIsFinished(m.Status) {
			return false // 迟到起飞/已终局:不写
		}
		m.Status = store.CollabStatusRunning
		m.Progress = "正在执行任务"
		sess.Status = deriveCollabSessionStatus(sess)
		applySessionError(sess)
		card = collabCopyMember(*m)
		changed = true
		return true
	})
	if !written || !changed {
		return
	}
	c.enqueueJob(collabEventJob{important: true, event: &store.CollabEvent{
		SessionID:      sessionID,
		MemberThreadID: threadID,
		Kind:           string(store.CollabStatusRunning),
		Member:         &card,
	}})
}

// ---- 终局:所有出口恰好一次 ----

// collabMemberOutcome 成员一次运行的终局意向(由 runCollabMember 的各出口填写,
// 经 defer 的 finalizeMember 恰好落一次终局;status 为零值 = panic 等异常退出)。
type collabMemberOutcome struct {
	status  store.CollabStatus
	summary string
	errText string
}

// finalizeMember 成员终局的唯一写者。三道闸按序:代际(旧轮次不得覆盖新轮次)
// → 终局(已终局成员不复活,迟到的 TurnCompleted 被拒)→ store 的 UpdatedAt
// 世代校验(最后一道粗粒度防线,撞上则重读重试)。终局同步落库 —— 状态真值
// 先立住,事件随后经有界队列发出;写失败可观察但绝不伪造完成。
func (c *collabCoordinator) finalizeMember(sessionID, threadID string, gen int, o collabMemberOutcome) {
	status := o.status
	if status == "" {
		status = store.CollabStatusFailed
		o.errText = "成员轮次异常退出,没有产出可确认的终局(守护兜底)"
	}
	ctx, cancel := context.WithTimeout(context.Background(), collabPersistTimeout)
	defer cancel()
	var (
		card       store.CollabMember
		prevStatus store.CollabStatus
		changed    bool
	)
	written := c.updateSessionRetry(ctx, sessionID, func(sess *store.CollabSession) bool {
		i := collabMemberIndex(sess, threadID)
		if i < 0 {
			log.Printf("[collab] WARNING: 会话 %s 里找不到成员线程 %s,终局(%s)无处落库", sessionID, threadID, status)
			return false
		}
		m := &sess.Members[i]
		if !c.ownsGeneration(sessionID, threadID, gen) {
			log.Printf("[collab] 会话 %s 成员 %s 的迟到终局(%s,gen=%d)被代际闸拒绝(当前 gen=%d):旧轮次不得覆盖新轮次",
				sessionID, threadID, status, gen, c.generationFor(sessionID, threadID))
			return false
		}
		if store.CollabStatusIsFinished(m.Status) {
			log.Printf("[collab] 会话 %s 成员 %s 已终局(%s),迟到的终局更新(%s)被拒:终局不复活",
				sessionID, threadID, m.Status, status)
			return false
		}
		prevStatus = sess.Status
		m.Status = status
		m.Summary = ""
		m.Progress = ""
		m.Error = ""
		if status == store.CollabStatusCompleted {
			m.Summary = runtime.ClipRunesEllipsis(strings.TrimSpace(o.summary), collabSummaryCap)
		}
		if status == store.CollabStatusFailed || status == store.CollabStatusCancelled {
			m.Error = runtime.ClipRunesEllipsis(strings.TrimSpace(o.errText), collabErrorCap)
		}
		m.PendingApprovals = nil // 终局成员没有挂起审批:审批随轮次死亡
		sess.Status = deriveCollabSessionStatus(sess)
		applySessionError(sess)
		card = collabCopyMember(*m)
		changed = true
		return true
	})
	if !written || !changed {
		return
	}
	c.enqueueJob(collabEventJob{important: true, event: &store.CollabEvent{
		SessionID:      sessionID,
		MemberThreadID: threadID,
		Kind:           string(status),
		Member:         &card,
	}})
	// 会话级状态翻转:补一帧全量快照(客户端刷新会话横幅,如"2/3 已完成")。
	if cur, ok := c.store.GetSession(ctx, sessionID); ok && cur.Status != prevStatus {
		ssnap := collabCopySession(cur)
		c.enqueueJob(collabEventJob{important: true, event: &store.CollabEvent{
			SessionID: sessionID,
			Kind:      collabEventKindSnapshot,
			Snapshot:  &ssnap,
		}})
	}
}

// ---- 审批:挂卡 / 摘卡(映射到成员卡,应答仍走既有 SubmitApproval)----

// memberApprovalWaiting 成员审批状态的权威迁移。waiting=true:成员停在审批上
// (WAITING_APPROVAL + 挂起审批映射到卡);false:已应答/超时,回 RUNNING。
// 多笔并发审批在所有 provider 上都是串行发生的(工具逐个执行),这里按
// "内存里活审批的全集"重建引用列表,天然幂等。
func (c *collabCoordinator) memberApprovalWaiting(sessionID, threadID string, gen int, req *pb.ApprovalRequest, waiting bool) {
	ctx, cancel := context.WithTimeout(context.Background(), collabPersistTimeout)
	defer cancel()
	exclude := ""
	if !waiting {
		exclude = req.GetRequestId() // 应答后的那次迁移,把刚处理完的从卡上摘下
	}
	var (
		card    store.CollabMember
		changed bool
	)
	written := c.updateSessionRetry(ctx, sessionID, func(sess *store.CollabSession) bool {
		i := collabMemberIndex(sess, threadID)
		if i < 0 {
			return false
		}
		m := &sess.Members[i]
		if !c.ownsGeneration(sessionID, threadID, gen) {
			log.Printf("[collab] 会话 %s 成员 %s 的审批状态变更被代际闸拒绝(旧代际轮次)", sessionID, threadID)
			return false
		}
		if store.CollabStatusIsFinished(m.Status) {
			return false // 终局成员不再接收审批状态变更
		}
		if waiting {
			m.Status = store.CollabStatusWaitingApproval
			m.PendingApprovals = c.approvalRefsFor(threadID, "")
			m.Progress = "等待审批:" + collabApprovalAskLine(req)
		} else {
			m.Status = store.CollabStatusRunning
			m.PendingApprovals = c.approvalRefsFor(threadID, exclude)
			m.Progress = "审批已处理,继续执行"
		}
		sess.Status = deriveCollabSessionStatus(sess)
		applySessionError(sess)
		card = collabCopyMember(*m)
		changed = true
		return true
	})
	if !written || !changed {
		return
	}
	kind := string(store.CollabStatusWaitingApproval)
	if !waiting {
		kind = string(store.CollabStatusRunning)
	}
	c.enqueueJob(collabEventJob{important: true, event: &store.CollabEvent{
		SessionID:      sessionID,
		MemberThreadID: threadID,
		Kind:           kind,
		Member:         &card,
	}})
}

// approvalRefsFor 从内存里活审批(turnApprovals)重建成员卡的挂起审批引用。
// excludeRequestID 用于应答后的摘卡(deliver 已把它从 pending 表移除,这里
// 再排除一次兜底时序)。request_id 都是真实轮次上注册的 —— 审批必须对应
// 真实的 thread_id/request_id,这里是唯一来源。
func (c *collabCoordinator) approvalRefsFor(threadID, excludeRequestID string) []store.CollabApprovalRef {
	v, ok := c.srv.approvalSessions.Load(threadID)
	if !ok {
		return nil
	}
	now := time.Now().UnixMilli()
	var refs []store.CollabApprovalRef
	for _, r := range v.(*turnApprovals).snapshot() {
		if excludeRequestID != "" && r.GetRequestId() == excludeRequestID {
			continue
		}
		refs = append(refs, store.CollabApprovalRef{
			RequestID: r.GetRequestId(),
			ToolName:  r.GetAction().GetName(),
			CreatedAt: now,
		})
	}
	return refs
}

// collabApprovalAskLine 审批进度的一个人话短句。
func collabApprovalAskLine(req *pb.ApprovalRequest) string {
	if name := req.GetAction().GetName(); name != "" {
		return "工具 " + name
	}
	return runtime.ClipRunesEllipsis(strings.TrimSpace(req.GetReason()), 80)
}

// clearStalePendingApprovals 审批提交失败(过期/重复/跨线程)时,把成员卡上
// 遗留的挂起审批摘掉,让卡片与真相收敛 —— 否则协同页会一直挂着一条永远
// 应答不了的审批。内存里仍有活审批时不动(那不是遗留)。非协同线程 no-op。
func (c *collabCoordinator) clearStalePendingApprovals(threadID string) {
	sessionID, ok := c.memberSessionOf(threadID)
	if !ok {
		return
	}
	if v, has := c.srv.approvalSessions.Load(threadID); has && len(v.(*turnApprovals).snapshot()) > 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), collabPersistTimeout)
	defer cancel()
	var (
		card    store.CollabMember
		changed bool
	)
	written := c.updateSessionRetry(ctx, sessionID, func(sess *store.CollabSession) bool {
		i := collabMemberIndex(sess, threadID)
		if i < 0 {
			return false
		}
		m := &sess.Members[i]
		if len(m.PendingApprovals) == 0 {
			return false
		}
		m.PendingApprovals = nil
		card = collabCopyMember(*m)
		changed = true
		return true
	})
	if !written || !changed {
		return
	}
	c.enqueueJob(collabEventJob{important: true, event: &store.CollabEvent{
		SessionID:      sessionID,
		MemberThreadID: threadID,
		Kind:           string(card.Status), // 卡片刷新帧:状态未变,只摘审批
		Member:         &card,
	}})
}

// ---- 取消与重试 ----

// CancelMember 只取消对应成员的子线程:复用 CancelTurn 的取消链路
// (activeTurns.Cancel 拿到该线程在跑轮次的取消函数),状态收敛由成员轮次
// 自己的终局路径落库(CANCELLED)—— 协调器发取消,不代写终局。
func (c *collabCoordinator) CancelMember(ctx context.Context, sessionID, threadID string) (*pb.CollabMemberSnapshot, error) {
	sess, ok := c.store.GetSession(ctx, sessionID)
	if !ok {
		return nil, errNotFound("协同会话不存在,无法取消成员:" + sessionID)
	}
	idx := collabMemberIndex(&sess, threadID)
	if idx < 0 {
		return nil, errNotFound("该协同会话里没有 thread_id 对应的成员")
	}
	m := sess.Members[idx]
	if store.CollabStatusIsFinished(m.Status) {
		return c.memberToPb(m), nil // 幂等:已终局的成员无需再取消,原卡如实返回
	}
	c.markRunCancelled(sessionID, threadID)
	if cancel := c.srv.activeTurns.Cancel(threadID); cancel != nil {
		cancel()
		log.Printf("[collab] 会话 %s 成员 %s 已发起取消(复用轮次取消链路)", sessionID, threadID)
	}
	// 受理即返回(卡片此刻可能仍显示 RUNNING):最终状态由成员轮次自己收敛,
	// 观察端经 Watch/GetCollabSession 跟进。不在这里替轮次写 CANCELLED。
	return c.memberToPb(m), nil
}

// CancelSession 逐成员发起取消并等状态收敛。有界等待(collabCancelConvergenceWait)
// + 超时降级为人话错误 —— 取消指令一定已送达,收敛只是等成员轮次走完自己的
// 收尾;等不到也不永久等,观察端稍后在协同页看到最终状态。
func (c *collabCoordinator) CancelSession(ctx context.Context, sessionID string) (*pb.CollabSessionSnapshot, error) {
	sess, ok := c.store.GetSession(ctx, sessionID)
	if !ok {
		return nil, errNotFound("协同会话不存在,无法取消:" + sessionID)
	}
	for i := range sess.Members {
		m := &sess.Members[i]
		if m.ThreadID == "" || store.CollabStatusIsFinished(m.Status) {
			continue
		}
		c.markRunCancelled(sessionID, m.ThreadID)
		if cancel := c.srv.activeTurns.Cancel(m.ThreadID); cancel != nil {
			cancel()
			log.Printf("[collab] 会话 %s 成员 %s 已发起取消", sessionID, m.ThreadID)
		}
	}
	deadline := time.Now().Add(collabCancelConvergenceWait)
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		cur, ok := c.store.GetSession(ctx, sessionID)
		if !ok {
			return nil, errNotFound("协同会话在取消过程中消失了(存储异常)")
		}
		if collabAllMembersFinished(&cur) {
			return c.sessionToPb(cur), nil
		}
		if time.Now().After(deadline) {
			pending := collabUnfinishedThreadIDs(&cur)
			return nil, errFailedPrecondition(fmt.Sprintf(
				"取消指令已发给全部成员,但 %s 内仍有 %d 个成员未收敛(%s);它们的最终状态会继续自行收敛并落在协同页上,不会丢",
				collabCancelConvergenceWait, len(pending), strings.Join(pending, "、")))
		}
		time.Sleep(collabCancelPollInterval)
	}
}

// RetryMember 以新代际重建成员轮次。规则:
//   - 只允许对终局成员重试(在跑/等审批的成员先取消);
//   - gen+1:旧代际的一切迟到回调从此失效,新轮次结果不与旧轮次混写;
//   - 旧结果不覆盖:成员卡上的旧摘要让位给新轮次(避免"运行中却挂着旧结果"
//     的误导),但旧结果仍完整保留在事件流(append-only)与子线程历史里。
func (c *collabCoordinator) RetryMember(ctx context.Context, sessionID, threadID string) (*pb.CollabMemberSnapshot, error) {
	sess, ok := c.store.GetSession(ctx, sessionID)
	if !ok {
		return nil, errNotFound("协同会话不存在,无法重试成员:" + sessionID)
	}
	idx := collabMemberIndex(&sess, threadID)
	if idx < 0 {
		return nil, errNotFound("该协同会话里没有 thread_id 对应的成员")
	}
	m := sess.Members[idx]
	if !store.CollabStatusIsFinished(m.Status) {
		return nil, errFailedPrecondition(fmt.Sprintf(
			"该成员当前状态是 %s,还在运行中,不能重试;如需重来请先取消该成员", m.Status))
	}
	task := c.retryTask(&sess, &m)
	gen := c.nextGeneration(sessionID, threadID) // 新 generation:旧轮次的迟到回调全部失效
	var card store.CollabMember
	changed := false
	written := c.updateSessionRetry(ctx, sessionID, func(s2 *store.CollabSession) bool {
		i := collabMemberIndex(s2, threadID)
		if i < 0 {
			return false
		}
		mm := &s2.Members[i]
		if !store.CollabStatusIsFinished(mm.Status) {
			return false // 并发重试幂等:别人已经重试过了,不重复改卡
		}
		mm.Status = store.CollabStatusAccepted
		mm.Summary = ""
		mm.Error = ""
		mm.Progress = "重试已受理,正在重新起飞"
		mm.PendingApprovals = nil
		s2.Status = deriveCollabSessionStatus(s2)
		applySessionError(s2)
		card = collabCopyMember(*mm)
		changed = true
		return true
	})
	if !written || !changed {
		return nil, errInternal(errors.New("重试落库失败,成员卡未更新,请稍后重试"))
	}
	c.enqueueJob(collabEventJob{important: true, event: &store.CollabEvent{
		SessionID:      sessionID,
		MemberThreadID: threadID,
		Kind:           string(store.CollabStatusAccepted),
		Member:         &card,
	}})
	go c.srv.runCollabMember(sessionID, threadID, task, gen)
	return c.memberToPb(card), nil
}

// retryTask 重建重试轮次的任务文本:角色还在就按角色现拼(与首次受理同一
// 语义);角色已被删/改则如实回落为原始指令并留痕。
func (c *collabCoordinator) retryTask(sess *store.CollabSession, m *store.CollabMember) string {
	if m.RoleID != "" {
		if role, ok := c.srv.cfg.Store.GetCollabRole(m.RoleID); ok {
			return collabRoleTask(role, sess.Instruction)
		}
		log.Printf("[collab] 重试:成员角色 %s 已不存在,回落为原始指令", m.RoleID)
	}
	return sess.Instruction
}

// ---- 快照 / 列表 / Watch ----

// Snapshot 会话全量快照(NotFound 用人话)。
func (c *collabCoordinator) Snapshot(ctx context.Context, sessionID string) (*pb.CollabSessionSnapshot, error) {
	sess, ok := c.store.GetSession(ctx, sessionID)
	if !ok {
		return nil, errNotFound("协同会话不存在:" + sessionID)
	}
	return c.sessionToPb(sess), nil
}

func (c *collabCoordinator) listSessions(ctx context.Context, includeFinished bool) []store.CollabSession {
	return c.store.ListSessions(ctx, includeFinished)
}

// sessionToPb 快照映射 + 活审批覆盖:成员线程此刻在内存里有活审批
// (turnApprovals)时,以它为准(带完整动作卡);没有则保留持久化引用的最小集。
func (c *collabCoordinator) sessionToPb(sess store.CollabSession) *pb.CollabSessionSnapshot {
	out := collabSessionToPb(sess)
	for i := range out.Members {
		if live := c.liveApprovals(out.Members[i].GetThreadId()); live != nil {
			out.Members[i].PendingApprovals = live
		}
	}
	return out
}

func (c *collabCoordinator) memberToPb(m store.CollabMember) *pb.CollabMemberSnapshot {
	out := collabMemberToPb(m)
	if live := c.liveApprovals(m.ThreadID); live != nil {
		out.PendingApprovals = live
	}
	return out
}

// liveApprovals 该线程此刻的活审批全集;线程没有登记(轮次已结束/非协同)时
// 返回 nil,让调用方保留持久化引用。返回非 nil 的空切片 = 轮次活着但没有
// 挂起审批(此刻的真相,覆盖掉一切过期引用)。
func (c *collabCoordinator) liveApprovals(threadID string) []*pb.ApprovalRequest {
	if threadID == "" {
		return nil
	}
	if v, ok := c.srv.approvalSessions.Load(threadID); ok {
		return v.(*turnApprovals).snapshot()
	}
	return nil
}

// Watch 先登记订阅、后回放(登记-回放窗口内的事件可能重复,按 sequence 去重;
// 反过来先回放后登记则会**丢**事件 —— 宁重不丢)。续传超出保留窗口(事件
// 被 retention 截断)时改发一帧全量快照兜底(proto 对 AfterSequence 的约定)。
func (c *collabCoordinator) Watch(ctx context.Context, sessionID string, after int64) (<-chan *pb.CollabSessionEvent, error) {
	if _, ok := c.store.GetSession(ctx, sessionID); !ok {
		return nil, errNotFound("协同会话不存在,无法订阅:" + sessionID)
	}
	sub := c.watchers.add()
	rctx, cancel := context.WithTimeout(context.Background(), collabPersistTimeout)
	replayed, err := c.store.ReplayEvents(rctx, sessionID, after)
	cancel()
	if err != nil {
		c.watchers.remove(sub.id)
		return nil, errInternal(err)
	}
	// gap 兜底:首个回放序号 > after+1 说明 (after, 首个序号) 里有事件已被
	// 截断/丢失 —— 客户端不得当"没发生",这里直接补一帧全量快照
	// (sequence 记为 after,不影响客户端的水位单调)。
	var frames []store.CollabEvent
	if len(replayed) > 0 && replayed[0].Sequence > after+1 {
		if snap := c.currentSession(sessionID); snap != nil {
			frames = append(frames, store.CollabEvent{
				Sequence:  after,
				SessionID: sessionID,
				Kind:      collabEventKindSnapshot,
				Snapshot:  snap,
			})
		}
	}
	frames = append(frames, replayed...)
	lastSeq := after

	out := make(chan *pb.CollabSessionEvent, collabWatchOutQueue)
	go func() {
		// 单发送者独占 out:退出时先摘订阅(不再有新事件进 sub.ch),再 close(out),
		// RPC 侧 for-range 自然结束 —— close 语义由 ctx 取消驱动,会话终局不关流
		// (重试可以把会话重新打开,客户端自行断开)。
		defer close(out)
		defer c.watchers.remove(sub.id)
		for _, f := range frames {
			select {
			case out <- c.eventToPb(f):
			case <-ctx.Done():
				return
			}
		}
		for {
			select {
			case ev := <-sub.ch:
				if ev.Sequence <= lastSeq {
					continue // 登记-回放窗口的重复帧
				}
				lastSeq = ev.Sequence
				select {
				case out <- c.eventToPb(ev):
				default:
					// 出站通道满(手机端消费慢):丢弃,客户端凭 sequence 缺口
					// 重新拉全量快照 —— 绝不为慢客户端阻塞事件管道。
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}

func (c *collabCoordinator) eventToPb(e store.CollabEvent) *pb.CollabSessionEvent {
	out := &pb.CollabSessionEvent{
		Sequence:       e.Sequence,
		SessionId:      e.SessionID,
		MemberThreadId: e.MemberThreadID,
		Kind:           e.Kind,
	}
	if e.Snapshot != nil {
		out.Snapshot = c.sessionToPb(*e.Snapshot)
	}
	if e.Member != nil {
		out.Member = c.memberToPb(*e.Member)
	}
	return out
}

// ---- 事件管道:有界队列 + 单消费者 ----

// enqueueJob 入队。进度帧队列满直接丢(计数);重要状态帧短等后放弃并告警 ——
// 两者的状态真值都已同步落库,丢的是"通知",不是"事实"。
func (c *collabCoordinator) enqueueJob(job collabEventJob) {
	c.evOnce.Do(func() {
		go c.runEventWorker()
	})
	select {
	case c.evCh <- job:
		return
	default:
	}
	if !job.important {
		c.droppedProgress.Add(1)
		return
	}
	timer := time.NewTimer(collabEventEnqueueWait)
	defer timer.Stop()
	select {
	case c.evCh <- job:
	case <-timer.C:
		c.droppedEvents.Add(1)
		kind := ""
		if job.event != nil {
			kind = job.event.Kind
		}
		log.Printf("[collab] WARNING: 协同事件队列已满(容量 %d),一条重要事件(kind=%s)等待 %s 后放弃;状态真值已落库,观察端凭 GetCollabSession 快照兜底",
			collabEventQueueSize, kind, collabEventEnqueueWait)
	}
}

func (c *collabCoordinator) enqueueProgress(sessionID, threadID string, gen int, text string) {
	c.enqueueJob(collabEventJob{progress: &collabProgressJob{
		sessionID: sessionID, threadID: threadID, gen: gen, text: text,
	}})
}

// runEventWorker 事件管道的单消费者:追加事件落盘 → 扇出给订阅者。
// 事件追加失败 = 该事件没有发生(序号未消费),记日志计数、不补发伪造帧;
// 观察端凭快照兜底(store.AppendEvent 的写序契约)。
func (c *collabCoordinator) runEventWorker() {
	for job := range c.evCh {
		ctx, cancel := context.WithTimeout(context.Background(), collabPersistTimeout)
		switch {
		case job.event != nil:
			c.appendAndPublish(ctx, *job.event)
		case job.progress != nil:
			c.persistProgress(ctx, job.progress)
		}
		cancel()
	}
}

func (c *collabCoordinator) appendAndPublish(ctx context.Context, e store.CollabEvent) {
	seq, err := c.store.AppendEvent(ctx, e)
	if err != nil {
		c.persistFailures.Add(1)
		log.Printf("[collab] ERROR: 会话 %s 事件(kind=%s)落盘失败,观察端将凭快照兜底: %v", e.SessionID, e.Kind, err)
		return
	}
	e.Sequence = seq
	c.watchers.publish(e)
}

// persistProgress 把成员进度落进卡片并派一帧 PROGRESS(有界节流后的低频写)。
// 旧代际/已终局/等审批中的成员不覆盖(等审批时进度保持审批文案)。
func (c *collabCoordinator) persistProgress(ctx context.Context, job *collabProgressJob) {
	var (
		card    store.CollabMember
		changed bool
	)
	written := c.updateSessionRetry(ctx, job.sessionID, func(sess *store.CollabSession) bool {
		i := collabMemberIndex(sess, job.threadID)
		if i < 0 {
			return false
		}
		m := &sess.Members[i]
		if !c.ownsGeneration(job.sessionID, job.threadID, job.gen) {
			return false
		}
		if store.CollabStatusIsFinished(m.Status) || m.Status == store.CollabStatusWaitingApproval {
			return false
		}
		if m.Progress == job.text {
			return false // 同一句话不重复写(幂等)
		}
		m.Progress = job.text
		card = collabCopyMember(*m)
		changed = true
		return true
	})
	if !written || !changed {
		return
	}
	c.appendAndPublish(ctx, store.CollabEvent{
		SessionID:      job.sessionID,
		MemberThreadID: job.threadID,
		Kind:           collabEventKindProgress,
		Member:         &card,
	})
}

// counters 丢弃/失败记账快照(诊断与测试观察点)。
func (c *collabCoordinator) counters() (droppedProgress, droppedEvents, persistFailures int64) {
	return c.droppedProgress.Load(), c.droppedEvents.Load(), c.persistFailures.Load()
}

// ---- 有界写循环(状态落库的公共底座)----

// updateSessionRetry 整聚合替换式更新的重读重试循环:mutate 直接改传入的会话
// 聚合;携带读到的 UpdatedAt 写回(等于放行、小于 = 迟到写入被 store 拒),
// 撞上 ErrCollabStaleUpdate 就重读重试,重试有界。其它错误如实记日志、计数
// —— 返回 false,绝不静默吞掉(协调器各调用点据此放弃本次更新,状态真值
// 以 store 里已有的为准,绝不伪造)。
func (c *collabCoordinator) updateSessionRetry(ctx context.Context, sessionID string, mutate func(sess *store.CollabSession) bool) bool {
	for attempt := 0; attempt < collabUpdateRetries; attempt++ {
		cur, ok := c.store.GetSession(ctx, sessionID)
		if !ok {
			log.Printf("[collab] WARNING: 会话 %s 更新时已不存在(可能被清理),本次变更放弃", sessionID)
			return false
		}
		if !mutate(&cur) {
			return true // 无变化也视为成功(幂等调用)
		}
		err := c.store.UpdateSession(ctx, cur)
		if err == nil {
			return true
		}
		if errors.Is(err, store.ErrCollabStaleUpdate) {
			continue // 迟到写入:重读最新真值再试
		}
		c.persistFailures.Add(1)
		log.Printf("[collab] ERROR: 会话 %s 状态落库失败(不重试、不伪造完成,真值以已落库为准): %v", sessionID, err)
		return false
	}
	c.persistFailures.Add(1)
	log.Printf("[collab] ERROR: 会话 %s 连续 %d 次撞上迟到写入被拒(并发写者异常),放弃本次更新", sessionID, collabUpdateRetries)
	return false
}

// ---- 代际登记 ----

func collabRunKey(sessionID, threadID string) string {
	return sessionID + "\x00" + threadID
}

func (c *collabCoordinator) generationFor(sessionID, threadID string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	if rs := c.runs[collabRunKey(sessionID, threadID)]; rs != nil {
		return rs.gen
	}
	return 0
}

func (c *collabCoordinator) ownsGeneration(sessionID, threadID string, gen int) bool {
	return gen != 0 && c.generationFor(sessionID, threadID) == gen
}

// nextGeneration 取下一运行代际(首次 = 1;重试 = 旧值+1 并清除取消标记)。
func (c *collabCoordinator) nextGeneration(sessionID, threadID string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	k := collabRunKey(sessionID, threadID)
	rs := c.runs[k]
	if rs == nil {
		rs = &collabRunState{gen: 1}
	} else {
		rs.gen++
		rs.cancelled = false
	}
	c.runs[k] = rs
	return rs.gen
}

func (c *collabCoordinator) markRunCancelled(sessionID, threadID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if rs := c.runs[collabRunKey(sessionID, threadID)]; rs != nil {
		rs.cancelled = true
	}
}

// runSuperseded 报告 (sessionID, threadID, gen) 这条运行是否已被取代或取消:
// 受理与起飞之间被取消的轮次在此被拦下,不再白跑。
func (c *collabCoordinator) runSuperseded(sessionID, threadID string, gen int) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	rs := c.runs[collabRunKey(sessionID, threadID)]
	return rs == nil || rs.gen != gen || rs.cancelled
}

// memberSessionOf 反查 thread_id 归属的协同会话(runs 登记簿是受理时建立的,
// 覆盖所有建过线程的成员)。找不到 = 非协同线程(或 daemon 重启后的旧卡)。
func (c *collabCoordinator) memberSessionOf(threadID string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k := range c.runs {
		if i := strings.IndexByte(k, 0); i >= 0 && k[i+1:] == threadID {
			return k[:i], true
		}
	}
	return "", false
}

func (c *collabCoordinator) currentSession(sessionID string) *store.CollabSession {
	ctx, cancel := context.WithTimeout(context.Background(), collabPersistTimeout)
	defer cancel()
	sess, ok := c.store.GetSession(ctx, sessionID)
	if !ok {
		return nil
	}
	cp := collabCopySession(sess)
	return &cp
}

// ---- 会话级派生 ----

// collabMemberIndex 按成员线程 id 定位成员卡下标。
func collabMemberIndex(sess *store.CollabSession, threadID string) int {
	for i := range sess.Members {
		if sess.Members[i].ThreadID == threadID {
			return i
		}
	}
	return -1
}

// deriveCollabSessionStatus 会话整体状态由成员卡派生(不独立记账,没有第二份
// 真值)。优先级:有人在跑 → RUNNING;有人等审批 → WAITING_APPROVAL(等待不是
// 终局,其他成员照常跑);全员终局 → FAILED > CANCELLED > COMPLETED。
func deriveCollabSessionStatus(sess *store.CollabSession) store.CollabStatus {
	if len(sess.Members) == 0 {
		return store.CollabStatusAccepted
	}
	var running, waiting, failed, cancelled int
	for i := range sess.Members {
		switch sess.Members[i].Status {
		case store.CollabStatusAccepted, store.CollabStatusRunning:
			running++
		case store.CollabStatusWaitingApproval:
			waiting++
		case store.CollabStatusFailed:
			failed++
		case store.CollabStatusCancelled:
			cancelled++
		}
	}
	if running > 0 {
		return store.CollabStatusRunning
	}
	if waiting > 0 {
		return store.CollabStatusWaitingApproval
	}
	if failed > 0 {
		return store.CollabStatusFailed
	}
	if cancelled > 0 {
		return store.CollabStatusCancelled
	}
	return store.CollabStatusCompleted
}

// applySessionError 会话级失败原因的人话聚合(直接可展示;成员级细节在各自卡上)。
func applySessionError(sess *store.CollabSession) {
	switch sess.Status {
	case store.CollabStatusFailed:
		n, first := 0, ""
		for i := range sess.Members {
			if sess.Members[i].Status != store.CollabStatusFailed {
				continue
			}
			n++
			if first == "" {
				first = sess.Members[i].Error
			}
		}
		sess.Error = fmt.Sprintf("%d 个成员未完成,第一例原因:%s", n, runtime.ClipRunesEllipsis(first, 120))
	case store.CollabStatusCancelled:
		sess.Error = "协同已取消"
	default:
		sess.Error = ""
	}
}

func collabAllMembersFinished(sess *store.CollabSession) bool {
	for i := range sess.Members {
		if !store.CollabStatusIsFinished(sess.Members[i].Status) {
			return false
		}
	}
	return true
}

func collabUnfinishedThreadIDs(sess *store.CollabSession) []string {
	var out []string
	for i := range sess.Members {
		m := &sess.Members[i]
		if !store.CollabStatusIsFinished(m.Status) {
			id := m.ThreadID
			if id == "" {
				id = m.AgentID
			}
			out = append(out, id+"("+string(m.Status)+")")
		}
	}
	return out
}

// ---- 内存版 CollabSessionStore(DataDir 为空的测试/极简构造路径)----
//
// 与 FileCollabSessionStore 同一套语义(世代校验、序号单调、快照纪律),
// 只是没有落盘 —— 与 newTurnJournal 的空目录退化是同一条约定。
type memoryCollabSessionStore struct {
	mu       sync.Mutex
	sessions map[string]store.CollabSession
	events   map[string][]store.CollabEvent
	lastSeq  map[string]int64
}

var _ store.CollabSessionStore = (*memoryCollabSessionStore)(nil)

func newMemoryCollabSessionStore() *memoryCollabSessionStore {
	return &memoryCollabSessionStore{
		sessions: map[string]store.CollabSession{},
		events:   map[string][]store.CollabEvent{},
		lastSeq:  map[string]int64{},
	}
}

func (m *memoryCollabSessionStore) CreateSession(ctx context.Context, s store.CollabSession) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.sessions[s.ID]; exists {
		return fmt.Errorf("collab session %q 重复创建: %w", s.ID, store.ErrCollabSessionExists)
	}
	stored := collabCopySession(s)
	if stored.CreatedAt == 0 {
		stored.CreatedAt = time.Now().UnixMilli()
	}
	stored.UpdatedAt = time.Now().UnixMilli()
	m.sessions[s.ID] = stored
	return nil
}

func (m *memoryCollabSessionStore) GetSession(ctx context.Context, id string) (store.CollabSession, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return store.CollabSession{}, false
	}
	return collabCopySession(s), true
}

func (m *memoryCollabSessionStore) ListSessions(ctx context.Context, includeFinished bool) []store.CollabSession {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]store.CollabSession, 0, len(m.sessions))
	for _, s := range m.sessions {
		if !includeFinished && store.CollabStatusIsFinished(s.Status) {
			continue
		}
		out = append(out, collabCopySession(s))
	}
	// 按 UpdatedAt 倒序,同刻按 id 升序(与 FileCollabSessionStore 同一口径)。
	for i := 1; i < len(out); i++ {
		for j := i; j > 0; j-- {
			a, b := out[j], out[j-1]
			if a.UpdatedAt > b.UpdatedAt || (a.UpdatedAt == b.UpdatedAt && a.ID < b.ID) {
				out[j], out[j-1] = b, a
			} else {
				break
			}
		}
	}
	return out
}

func (m *memoryCollabSessionStore) UpdateSession(ctx context.Context, s store.CollabSession) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	cur, ok := m.sessions[s.ID]
	if !ok {
		return fmt.Errorf("collab session %q: %w", s.ID, store.ErrCollabSessionNotFound)
	}
	if s.UpdatedAt != 0 && s.UpdatedAt < cur.UpdatedAt {
		return fmt.Errorf("collab session %q 迟到写入(携带 updated_at=%d < 当前 %d): %w",
			s.ID, s.UpdatedAt, cur.UpdatedAt, store.ErrCollabStaleUpdate)
	}
	stored := collabCopySession(s)
	stored.CreatedAt = cur.CreatedAt
	stored.UpdatedAt = time.Now().UnixMilli()
	m.sessions[s.ID] = stored
	return nil
}

func (m *memoryCollabSessionStore) AppendEvent(ctx context.Context, e store.CollabEvent) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if e.SessionID == "" {
		return 0, fmt.Errorf("collab event 缺少 session_id: %w", store.ErrCollabInvalidSessionID)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.sessions[e.SessionID]; !ok {
		return 0, fmt.Errorf("collab session %q: %w", e.SessionID, store.ErrCollabSessionNotFound)
	}
	seq := m.lastSeq[e.SessionID] + 1
	stored := collabCopyEvent(e)
	stored.Sequence = seq
	stored.SessionID = e.SessionID
	if stored.CreatedAt == 0 {
		stored.CreatedAt = time.Now().UnixMilli()
	}
	m.lastSeq[e.SessionID] = seq
	m.events[e.SessionID] = append(m.events[e.SessionID], stored)
	return seq, nil
}

func (m *memoryCollabSessionStore) ReplayEvents(ctx context.Context, id string, after int64) ([]store.CollabEvent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.sessions[id]; !ok {
		return nil, fmt.Errorf("collab session %q: %w", id, store.ErrCollabSessionNotFound)
	}
	var out []store.CollabEvent
	for _, ev := range m.events[id] {
		if ev.Sequence > after {
			out = append(out, collabCopyEvent(ev))
		}
	}
	return out, nil
}

func collabCopyEvent(in store.CollabEvent) store.CollabEvent {
	out := in
	if in.Snapshot != nil {
		cp := collabCopySession(*in.Snapshot)
		out.Snapshot = &cp
	}
	if in.Member != nil {
		cp := collabCopyMember(*in.Member)
		out.Member = &cp
	}
	return out
}
