package server

// collab_session.go —— 协同扇出:一条指令,交给**多个 agent 并行执行**。
//
// 【语义边界(最重要)】本文件实现的东西与委派(DelegateSubTask)**根本不同**:
//   · 委派 = 父子:主会话里的 AI 自己决定要不要派活、派给谁、派什么子任务;
//   · 协同 = 并行协作:用户在协同页选好若干 agent、发**同一条**指令,
//     daemon 给每个 agent 各开一个子线程**同时**跑,成员之间对等、无父子。
// 用户明确要的是后者("多个 agent 共同完成一个任务"),本项目此前只有前者。
//
// 【刻意不做的编排】这里不实现"谁先谁后、谁汇总谁的结论"。那是让 daemon 替
// 用户猜分工,既不可预测也不可控;选人 + 角色配置已经把决策权交回用户手里,
// daemon 只负责把同一条指令忠实扇出给所有成员。
//
// 【P1 协同闭环(Task 3/4)】DispatchCollab 从"派发后撒手"升级为"创建持久化
// 协同会话 + 逐成员受理":受理成功只写 ACCEPTED→RUNNING(绝不写 DONE),
// 失败成员写 FAILED+人话原因,成员轮次的全部出口经 finalizeMember 恰好落一次
// 终局。状态机/持久化/事件管道在 collab_session_state.go 与
// collab_session_events.go;本文件保留 RPC 面:策略闸、角色解析、成员轮次主体
// (runCollabMember)与六个协同 RPC 的接线。

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/store"
)

// maxCollabTurnDuration 单个协同成员的轮次上界。
//
// 【为什么与委派共用同一个上界】两者跑的都是"无人值守的一整轮 agent",风险
// 同源(模型跑飞 / 工具卡死),没必要给出第二个时长让人去记。真要区分时再拆。
const maxCollabTurnDuration = maxDelegateTurnDuration

// collabMemberSpec RPC 层解析出的单个成员:角色人设已拼进任务文本,
// agent/model/effort/workspace 已定;err 非空 = 受理前就注定失败。
type collabMemberSpec struct {
	agentID   string
	roleID    string
	model     string
	effort    string
	workspace string
	task      string
	err       string
}

// DispatchCollab 受理一次协同扇出:先创建持久化会话,再逐成员受理(建子线程、
// 起飞轮次),响应里每个成员仍按请求顺序给出 thread_id。
//
// 【为什么 RPC 只负责"受理"而不等结果】四个 agent 同时跑完可能要很久,让手机
// 端干等到最后一个完成是不可用的。所以这里把每个成员的子线程 id 立刻返回;
// 与旧版不同的是,受理结果同时落进协同会话快照 —— 前端既可按 thread_id 用
// 既有 ThreadService 回填相位,也可用 GetCollabSession/WatchCollabSession 从
// daemon 真值恢复整屏(旧客户端的响应形状不变:仍是 SessionId + Items)。
//
// 【为什么一个成员失败不影响其它成员】并行协作的语义就是"各自去干":某一家
// 的 agent 没装好,不该让其它三家一起陪葬。失败的成员在 items 里给**人话原因**,
// 成功的照常起飞 —— 这正是"部分成功"在扇出场景下的诚实表达。
func (s *Server) DispatchCollab(ctx context.Context, req *pb.DispatchCollabRequest) (*pb.DispatchCollabResponse, error) {
	// W10:协同策略闸(规格 §11)。关掉就要"如实拒绝 + 说清怎么开回来",而不是静默照跑
	// —— 用户关它的目的正是让这件事不再发生。
	policy := s.collabPolicySnapshot()
	if !policy.CollabEnabled() {
		return nil, errFailedPrecondition("协同已在运行时策略里关闭(可在控制面重新开启)")
	}
	if !policy.Fanout() {
		return nil, errFailedPrecondition("协同扇出已在运行时策略里关闭(可在控制面重新开启)")
	}
	instruction := strings.TrimSpace(req.GetInstruction())
	if instruction == "" {
		return nil, errInvalidArg("协同指令不能为空:请在协同页输入要交给这些 agent 的任务")
	}
	if len(req.GetMembers()) == 0 {
		return nil, errInvalidArg("至少要选一个参与协同的 agent:请先勾选本次要用哪些 agent")
	}
	// 成员数上限:超限**如实报错**,不静默截断 —— 用户选了 5 家却只跑 3 家、界面还显示
	// "已派发",是最难排查的一类"看着有功能、就是不干活"。
	if max := policy.MaxMembers(); max > 0 && len(req.GetMembers()) > max {
		return nil, errInvalidArg(fmt.Sprintf(
			"本次选了 %d 个成员,超过运行时策略里的扇出上限 %d:请减少成员,或在控制面调高上限",
			len(req.GetMembers()), max))
	}

	// 角色解析在 RPC 层(配置便利,不是引擎能力),受理与起飞在协调器。
	specs := make([]collabMemberSpec, 0, len(req.GetMembers()))
	for _, m := range req.GetMembers() {
		specs = append(specs, s.resolveCollabMemberSpec(m, instruction, req.GetWorkspace()))
	}
	members := make([]DispatchMember, len(specs))
	for i, sp := range specs {
		dm := DispatchMember{
			AgentID: sp.agentID, RoleID: sp.roleID, ModelID: sp.model,
			Effort: sp.effort, Workspace: sp.workspace, Task: sp.task,
		}
		if sp.err != "" {
			dm.AcceptError = sp.err
		}
		members[i] = dm
	}
	snap, err := s.collabCoordinatorFor().Dispatch(ctx, DispatchRequest{
		Instruction: instruction,
		Workspace:   strings.TrimSpace(req.GetWorkspace()),
		Members:     members,
	})
	if err != nil {
		return nil, err
	}
	// 响应项与请求成员一一对应(按位置回填,重名成员也不会错位)——旧形状不变。
	items := make([]*pb.DispatchCollabItem, 0, len(snap.GetMembers()))
	for _, m := range snap.GetMembers() {
		items = append(items, &pb.DispatchCollabItem{
			AgentId:  m.GetAgentId(),
			ThreadId: m.GetThreadId(),
			// ack = 受理(子线程已建、轮次已起飞),**不等于**执行成功 ——
			// 结果看子线程状态/协同快照。受理失败(含建线程成功但落卡失败)
			// 一律 Ack=false + 人话原因。
			Ack:   m.GetThreadId() != "" && m.GetStatus() != string(store.CollabStatusFailed),
			Error: m.GetError(),
		})
	}
	log.Printf("[collab] 协同扇出 %s: 成员 %d,受理 %d", snap.GetSessionId(), len(items), countAcked(items))
	return &pb.DispatchCollabResponse{SessionId: snap.GetSessionId(), Items: items}, nil
}

// resolveCollabMemberSpec 解析单个成员:agent 与角色 → 谁、用什么模型/强度/
// 工作区、干什么(角色人设拼进任务文本)。显式给的值优先(与委派同一条规则)。
func (s *Server) resolveCollabMemberSpec(m *pb.CollabMember, instruction, defaultWS string) collabMemberSpec {
	out := collabMemberSpec{
		agentID:   strings.TrimSpace(m.GetAgentId()),
		roleID:    strings.TrimSpace(m.GetRoleId()),
		workspace: strings.TrimSpace(defaultWS),
		task:      instruction,
	}
	if out.roleID != "" {
		role, ok := s.cfg.Store.GetCollabRole(out.roleID)
		if !ok {
			out.err = errRoleUndefined(out.roleID, s.cfg.Store.ListCollabRoles()).Error()
			return out
		}
		if out.agentID == "" {
			out.agentID = role.AgentID
		}
		out.model = firstNonEmpty(out.model, role.Model)
		out.effort = firstNonEmpty(out.effort, role.Effort)
		out.workspace = firstNonEmpty(out.workspace, role.Workspace)
		out.task = collabRoleTask(role, instruction)
	}
	if out.agentID == "" {
		out.err = "该协同成员没指定 agent:请给它选一个 agent,或配一个「已设置目标 agent」的协同角色"
	}
	return out
}

// runCollabMember 在后台跑完一个协同成员的一整轮。
//
// 【为什么重新起一个 context 而不是用 RPC 的 ctx】RPC 一返回,它的 ctx 就被
// 取消;异步轮次若沿用,会在刚起飞的瞬间被整体杀掉 —— 表现为"状态永远是收到,
// 但什么都没跑"。所以这里用**脱离取消链**的独立 context,自己管超时;用户要
// 叫停走 CancelTurn/CancelCollabMember 的取消链路(经 activeTurns 命中这里的
// cancel 函数),不再有"RPC 取消后无法叫停"的空洞。
//
// 【终局恰好一次】本函数所有出口都只填写 outcome,由 defer 的 finalizeMember
// 统一落库/发事件 —— COMPLETED/FAILED/CANCELLED/WAITING_APPROVAL 各有其位:
// 取消优先于失败(用户叫停的轮次不被错误伪装),完成必须来自真实终稿。
func (s *Server) runCollabMember(sessionID, threadID, task string, gen int) {
	coord := s.collabCoordinatorFor()
	var outcome collabMemberOutcome
	defer func() { coord.finalizeMember(sessionID, threadID, gen, outcome) }()

	runCtx, cancel := context.WithTimeout(context.Background(), maxCollabTurnDuration)
	defer cancel()

	t, ok := s.cfg.Store.Get(threadID)
	if !ok {
		log.Printf("[collab] 成员线程 %s 起飞前已不存在(可能被删除),按失败终局落库", threadID)
		outcome = collabMemberOutcome{status: store.CollabStatusFailed, errText: "成员线程起飞前已不存在(可能被删除)"}
		return
	}
	lease, lerr := s.activeTurns.acquire(runCtx, t.ID, cancel)
	if lerr != nil {
		outcome = collabMemberOutcome{status: store.CollabStatusFailed, errText: fmt.Sprintf("获取轮次租约失败:%v", lerr)}
		return
	}
	defer s.activeTurns.Clear(t.ID, lease)

	// 受理-起飞窗口的取消竞态闸:会话级取消不等待刚起飞的轮次。
	if coord.runSuperseded(sessionID, threadID, gen) {
		outcome = collabMemberOutcome{status: store.CollabStatusCancelled, errText: "轮次起飞前已被取消"}
		return
	}

	provider, perr := s.providerForThread(t)
	if perr != nil {
		outcome = collabMemberOutcome{status: store.CollabStatusFailed, errText: perr.Error()}
		return
	}

	// 过程帧 → 有界事件转换器:只留进度/工具边界的低频摘要,原始大文本不进
	// 协同通道(要看全文去子线程历史);思考仍随 assistant 消息落库。
	card := &collabMemberCard{c: coord, sessionID: sessionID, threadID: threadID, gen: gen}
	childSink, thinkBuf := thinkingRecorder(card.sink())

	// 审批中继:复用 turnApprovals + SubmitApproval 的既有链路 —— 成员卡上的
	// pending_approvals 只是同一份真相的映射(request_id 真实、thread_id 就是
	// 本成员线程),不新增审批通道,也不代用户同意。
	approvals := newTurnApprovals()
	s.approvalSessions.Store(t.ID, approvals)
	defer s.approvalSessions.CompareAndDelete(t.ID, approvals)
	approve := func(actx context.Context, call *pb.ToolCallBlock) (bool, string, error) {
		reqID := randomID("apr")
		approvalCh := approvals.add(reqID)
		request := &pb.ApprovalRequest{RequestId: reqID, Action: call, Reason: agent.ApprovalReason(call)}
		request.Options, request.Details = approvalCardExtras(call)
		approvals.attach(request)
		// 挂到成员卡(WAITING_APPROVAL 帧),然后等待真实应答。
		coord.memberApprovalWaiting(sessionID, threadID, gen, request, true)
		defer func() {
			approvals.remove(reqID)
			childSink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_ApprovalResolved{
				ApprovalResolved: &pb.ApprovalResolved{RequestId: reqID, Note: "审批已结束"},
			}})
		}()
		waitCtx, waitCancel := context.WithTimeout(actx, approvalWaitBudget)
		defer waitCancel()
		var decision *pb.ApprovalDecision
		select {
		case decision = <-approvalCh:
		case <-waitCtx.Done():
			// 无人应答 ≠ 同意:按拒绝继续(绝不代拟"同意"),成员卡同步摘下
			// 挂起审批 —— 协同成员无人值守,不能为一笔回不了的审批把整轮吊死。
			// 预算口径与 StartTurn 主路径同一(approvalWaitBudget,注释互指)。
			coord.memberApprovalWaiting(sessionID, threadID, gen, request, false)
			return false, fmt.Sprintf("协同审批等待超时(%s),已按拒绝继续", approvalWaitBudget), nil
		case <-runCtx.Done():
			return false, "", runCtx.Err()
		}
		coord.memberApprovalWaiting(sessionID, threadID, gen, request, false)
		return decision.Approved, approvalNoteWithOption(decision), nil
	}

	t.Messages = append(t.Messages, store.StoredMessage{
		Role: "user",
		Blocks: []pb.ContentBlock{{Block: &pb.ContentBlock_Text{
			Text: &pb.TextBlock{Text: task},
		}}},
		CreatedAt: time.Now().UnixMilli(),
	})
	t.Status = pb.ThreadStatus_THREAD_STATUS_RUNNING
	t.TurnStartedUnixMs = time.Now().UnixMilli() // 轮次开始时刻:与 StartTurn 同口径
	logStoreUpdateErr("collab-member-start", t.ID, s.cfg.Store.Update(t))
	taskBlocks := t.Messages[len(t.Messages)-1].Blocks

	// 受理(ACCEPTED)与执行(RUNNING)的分界线在这里立牌。
	coord.transitionRunning(sessionID, threadID, gen)

	summary, userStopped, rerr := s.runTurnOnce(runCtx, provider, randomID("turn"), t,
		taskBlocks, childSink, thinkBuf, approve)
	switch {
	case userStopped:
		// 取消优先于失败:用户/会话叫停的轮次,终局是"已取消",不被错误伪装
		// (取消引发的 ctx 错误原样带上,供排查)。
		outcome = collabMemberOutcome{status: store.CollabStatusCancelled, errText: "成员已被取消"}
		if rerr != nil {
			if re, ok := rerr.(*turnRunError); ok {
				outcome.errText = "成员已取消(" + runtime.ClipRunesEllipsis(re.err.Error(), 160) + ")"
			}
		}
	case rerr != nil:
		errText := rerr.Error()
		if re, ok := rerr.(*turnRunError); ok {
			errText = re.err.Error()
		}
		outcome = collabMemberOutcome{status: store.CollabStatusFailed, errText: errText}
	default:
		outcome = collabMemberOutcome{status: store.CollabStatusCompleted, summary: summary}
	}
}

// ---- 协同会话观察与操作 RPC(Task 1 协议增量的 daemon 落点)----

// GetCollabSession 会话全量快照(手机端打开协同页的第一步:整屏恢复)。
func (s *Server) GetCollabSession(ctx context.Context, req *pb.GetCollabSessionRequest) (*pb.CollabSessionSnapshot, error) {
	if strings.TrimSpace(req.GetSessionId()) == "" {
		return nil, errInvalidArg("缺少协同会话 id")
	}
	return s.collabCoordinatorFor().Snapshot(ctx, req.GetSessionId())
}

// ListCollabSessions 协同会话列表(最简全量形态;includeFinished 控制是否带
// 已结束的)。
func (s *Server) ListCollabSessions(ctx context.Context, req *pb.ListCollabSessionsRequest) (*pb.ListCollabSessionsResponse, error) {
	coord := s.collabCoordinatorFor()
	out := &pb.ListCollabSessionsResponse{}
	for _, sess := range coord.listSessions(ctx, req.GetIncludeFinished()) {
		out.Sessions = append(out.Sessions, coord.sessionToPb(sess))
	}
	return out, nil
}

// WatchCollabSession 订阅单个会话的事件流:先回放 after 之后的持久化事件,
// 再实时转发;断线重连按 after_sequence 续传,超出保留窗口收到全量快照兜底。
func (s *Server) WatchCollabSession(req *pb.WatchCollabSessionRequest, stream pb.ThreadService_WatchCollabSessionServer) error {
	ctx := stream.Context()
	events, err := s.collabCoordinatorFor().Watch(ctx, req.GetSessionId(), req.GetAfterSequence())
	if err != nil {
		return err
	}
	for ev := range events {
		if err := stream.Send(ev); err != nil {
			return err
		}
	}
	return nil
}

// CancelCollabSession 取消整个会话:逐成员发起取消并等状态收敛(有界等待,
// 超时降级为人话错误 —— 收敛仍会继续,只是不让你干等)。
func (s *Server) CancelCollabSession(ctx context.Context, req *pb.CancelCollabSessionRequest) (*pb.CollabSessionSnapshot, error) {
	if strings.TrimSpace(req.GetSessionId()) == "" {
		return nil, errInvalidArg("缺少协同会话 id")
	}
	return s.collabCoordinatorFor().CancelSession(ctx, req.GetSessionId())
}

// CancelCollabMember 取消单个成员:只取消对应子线程(复用 CancelTurn 链路),
// 状态由成员轮次自己收敛落库。
func (s *Server) CancelCollabMember(ctx context.Context, req *pb.CancelCollabMemberRequest) (*pb.CollabMemberSnapshot, error) {
	if strings.TrimSpace(req.GetSessionId()) == "" || strings.TrimSpace(req.GetThreadId()) == "" {
		return nil, errInvalidArg("缺少协同会话 id 或成员线程 id")
	}
	return s.collabCoordinatorFor().CancelMember(ctx, req.GetSessionId(), req.GetThreadId())
}

// RetryCollabMember 以新代际重建成员轮次(旧结果保留在事件流/线程历史里)。
// 与 Dispatch 同过协同策略闸 —— 策略关掉时"重新派发"同样不该发生。
func (s *Server) RetryCollabMember(ctx context.Context, req *pb.RetryCollabMemberRequest) (*pb.CollabMemberSnapshot, error) {
	if strings.TrimSpace(req.GetSessionId()) == "" || strings.TrimSpace(req.GetThreadId()) == "" {
		return nil, errInvalidArg("缺少协同会话 id 或成员线程 id")
	}
	policy := s.collabPolicySnapshot()
	if !policy.CollabEnabled() {
		return nil, errFailedPrecondition("协同已在运行时策略里关闭(可在控制面重新开启)")
	}
	if !policy.Fanout() {
		return nil, errFailedPrecondition("协同扇出已在运行时策略里关闭(可在控制面重新开启)")
	}
	return s.collabCoordinatorFor().RetryMember(ctx, req.GetSessionId(), req.GetThreadId())
}

// ---- 辅助 ----

// firstNonEmpty 取第一个非空值(显式值优先于角色兜底值的那一条规则的载体)。
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// countAcked 统计受理成功的成员数(只用于日志,不进协议)。
func countAcked(items []*pb.DispatchCollabItem) int {
	n := 0
	for _, it := range items {
		if it.GetAck() {
			n++
		}
	}
	return n
}
