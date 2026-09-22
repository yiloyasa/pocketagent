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

import (
	"context"
	"log"
	"strings"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// maxCollabTurnDuration 单个协同成员的轮次上界。
//
// 【为什么与委派共用同一个上界】两者跑的都是"无人值守的一整轮 agent",风险
// 同源(模型跑飞 / 工具卡死),没必要给出第二个时长让人去记。真要区分时再拆。
const maxCollabTurnDuration = maxDelegateTurnDuration

// DispatchCollab 受理一次协同扇出:为每个成员各建一个子线程并起飞轮次。
//
// 【为什么 RPC 只负责"受理"而不等结果】四个 agent 同时跑完可能要很久,让手机
// 端干等到最后一个完成是不可用的。所以这里把每个成员的子线程 id 立刻返回,
// 前端按 thread_id 用既有 ThreadService 查询各自状态回填相位 —— 于是界面上
// 每个 agent 的进度是**独立、实时**的,而不是等齐了才一起显示。
//
// 【为什么一个成员失败不影响其它成员】并行协作的语义就是"各自去干":某一家
// 的 agent 没装好,不该让其它三家一起陪葬。失败的成员在 items 里给**人话原因**,
// 成功的照常起飞 —— 这正是"部分成功"在扇出场景下的诚实表达。
func (s *Server) DispatchCollab(ctx context.Context, req *pb.DispatchCollabRequest) (*pb.DispatchCollabResponse, error) {
	instruction := strings.TrimSpace(req.GetInstruction())
	if instruction == "" {
		return nil, errInvalidArg("协同指令不能为空:请在协同页输入要交给这些 agent 的任务")
	}
	if len(req.GetMembers()) == 0 {
		return nil, errInvalidArg("至少要选一个参与协同的 agent:请先勾选本次要用哪些 agent")
	}

	sessionID := randomID("collab")
	items := make([]*pb.DispatchCollabItem, 0, len(req.GetMembers()))
	for _, m := range req.GetMembers() {
		// 顺序一致:返回项与请求成员一一对应,前端按位置回填每个 agent 的相位,
		// 不需要拿 agent_id 反查自己的那一项(重名成员也不会错位)。
		items = append(items, s.acceptCollabMember(ctx, sessionID, instruction, m, req.GetWorkspace()))
	}
	log.Printf("[collab] 协同扇出 %s: 成员 %d,受理 %d", sessionID, len(items), countAcked(items))
	return &pb.DispatchCollabResponse{SessionId: sessionID, Items: items}, nil
}

// acceptCollabMember 受理单个成员:解析 agent 与角色 → 建子线程 → 起飞轮次。
func (s *Server) acceptCollabMember(
	ctx context.Context, sessionID, instruction string, m *pb.CollabMember, defaultWS string,
) *pb.DispatchCollabItem {
	agentID := strings.TrimSpace(m.GetAgentId())
	taskText := instruction
	var model, effort string
	workspace := strings.TrimSpace(defaultWS)

	// 角色:提供"我是谁、长期怎么干"以及 model/effort/workspace 的兜底值。
	// 显式给的值优先(与委派同一条规则,免得两处优先级不一致让人困惑)。
	if rid := strings.TrimSpace(m.GetRoleId()); rid != "" {
		role, ok := s.cfg.Store.GetCollabRole(rid)
		if !ok {
			return &pb.DispatchCollabItem{
				AgentId: agentID,
				Error:   errRoleUndefined(rid, s.cfg.Store.ListCollabRoles()).Error(),
			}
		}
		if agentID == "" {
			agentID = role.AgentID
		}
		model = firstNonEmpty(model, role.Model)
		effort = firstNonEmpty(effort, role.Effort)
		workspace = firstNonEmpty(workspace, role.Workspace)
		taskText = collabRoleTask(role, instruction)
	}
	if agentID == "" {
		return &pb.DispatchCollabItem{
			Error: "该协同成员没指定 agent:请给它选一个 agent,或配一个「已设置目标 agent」的协同角色",
		}
	}

	// 建子线程:agent 不存在由 CreateThread 动态列举当前可用 agent 并人话报错,
	// 这里不重复做存在性裁决(单一裁决点)。
	th, err := s.CreateThread(ctx, &pb.CreateThreadRequest{
		AgentId:   agentID,
		Workspace: workspace,
		ModelId:   model,
	})
	if err != nil {
		return &pb.DispatchCollabItem{AgentId: agentID, Error: err.Error()}
	}
	t, ok := s.cfg.Store.Get(th.GetId())
	if !ok {
		return &pb.DispatchCollabItem{
			AgentId:  agentID,
			ThreadId: th.GetId(),
			Error:    "子线程建出来后读不回来(存储异常),本次协同未起飞",
		}
	}
	t.Effort = effort
	// 【为什么不设 ParentThreadID】协同会话 id 不是一个真实线程,挂上去会造出
	// 一条指向不存在的父链 —— 手机端按本地 id 匹配父子,匹配不上就是一张永远
	// 点不亮的孤儿卡,那正是本项目反复在修的假象。协同的归组由前端的
	// session_id 负责,不借用父链语义。
	if err := s.cfg.Store.Update(t); err != nil {
		return &pb.DispatchCollabItem{AgentId: agentID, ThreadId: t.ID, Error: err.Error()}
	}

	// 起飞:并行。RPC 不等它跑完,所以这里用 goroutine;每个成员互不阻塞。
	go s.runCollabMember(t.ID, taskText, sessionID)

	// ack = 受理(子线程已建、轮次已起飞),**不等于**执行成功 —— 结果看子线程
	// 状态。把受理冒充成功是本项目一贯的红线。
	return &pb.DispatchCollabItem{AgentId: agentID, ThreadId: t.ID, Ack: true}
}

// runCollabMember 在后台跑完一个协同成员的一整轮。
//
// 【为什么重新起一个 context 而不是用 RPC 的 ctx】RPC 一返回,它的 ctx 就被
// 取消;异步轮次若沿用,会在刚起飞的瞬间被整体杀掉 —— 表现为"状态永远是收到,
// 但什么都没跑"。所以这里用**脱离取消链**的独立 context,自己管超时。
// 取舍:RPC 取消后无法再远程叫停这一轮,这是"要并行就得让它活着"的必然代价。
func (s *Server) runCollabMember(threadID, task, sessionID string) {
	runCtx, cancel := context.WithTimeout(context.Background(), maxCollabTurnDuration)
	defer cancel()

	t, ok := s.cfg.Store.Get(threadID)
	if !ok {
		log.Printf("[collab] 成员线程 %s 起飞前已不存在(可能被删除),跳过", threadID)
		return
	}
	lease, lerr := s.activeTurns.acquire(runCtx, t.ID, cancel)
	if lerr != nil {
		log.Printf("[collab] 成员 %s 获取轮次租约失败: %v", threadID, lerr)
		return
	}
	defer s.activeTurns.Clear(t.ID, lease)

	provider, perr := s.providerForThread(t)
	if perr != nil {
		log.Printf("[collab] 成员 %s 选 provider 失败: %v", threadID, perr)
		return
	}

	// 过程帧本次不下发(与委派的 v1 形态一致);思考仍随 assistant 消息落库。
	childSink, thinkBuf := thinkingRecorder(func(ev *pb.TurnServerMessage) {})
	// 准则 4:无用户在环,审批一律拒绝,绝不代拟"同意"。
	approve := func(_ context.Context, _ *pb.ToolCallBlock) (bool, string, error) {
		return false, "协同子任务暂无审批通道,已拒绝该操作", nil
	}

	t.Messages = append(t.Messages, store.StoredMessage{
		Role: "user",
		Blocks: []pb.ContentBlock{{Block: &pb.ContentBlock_Text{
			Text: &pb.TextBlock{Text: task},
		}}},
		CreatedAt: time.Now().UnixMilli(),
	})
	t.Status = pb.ThreadStatus_THREAD_STATUS_RUNNING
	logStoreUpdateErr("collab-member-start", t.ID, s.cfg.Store.Update(t))
	taskBlocks := t.Messages[len(t.Messages)-1].Blocks

	if _, _, rerr := s.runTurnOnce(runCtx, provider, randomID("turn"), t,
		taskBlocks, childSink, thinkBuf, approve); rerr != nil {
		log.Printf("[collab] 会话 %s 成员 %s 轮次失败: %v", sessionID, threadID, rerr)
	}
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
