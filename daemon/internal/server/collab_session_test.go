package server

// collab_session_test.go —— 协同扇出(一条指令 → 多个 agent 并行执行)。
//
// 【为什么钉这些】扇出的价值全在"每个成员各自独立"上:一家失败不该拖垮其它
// 家、返回项必须和请求顺序对得上、角色必须真的把人设和模型带进子线程。这三条
// 任何一条松了,界面上就会变成"看起来派了,其实没派 / 派串了"。

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

func newCollabSessionServer(t *testing.T) *Server {
	t.Helper()
	fs, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("建测试存储失败: %v", err)
	}
	s := New(Config{
		Store:     fs,
		Providers: map[string]agent.Provider{"echo": agent.NewEcho()},
		Workspace: t.TempDir(),
	})
	// 扇出按设计异步执行：RPC 只受理，轮次跑在**脱离取消链**的 goroutine 里
	// （否则 RPC 一返回、ctx 被取消，刚起飞的轮次整个被杀 —— 现场表现为
	// "永远显示收到但什么都没跑"）。而下面这些用例断言的只是响应，于是那些
	// goroutine 会在测试函数返回**之后**才写 store 与 workspace，撞上 t.TempDir()
	// 的清理，在 Windows 上报 "directory is not empty" 而间歇失败
	// （修复前实测连跑 8 次：3 次挂 FansOutPerMember、4 次挂 PartialFailure）。
	//
	// 把「等在飞轮次落定」放进构造器，比要求每个用例各自记得更可靠 ——
	// 将来新增的用例自动获得同一保证。t.Cleanup 是 LIFO：这里注册的等待先执行，
	// 之后才轮到 t.TempDir() 的清理。
	t.Cleanup(func() { waitActiveTurnsDrained(t, s) })
	return s
}

// waitActiveTurnsDrained 等所有在飞轮次落定。
//
// 【为什么末尾还有沉降窗口】租约释放与「落库 + 收尾」的最后一步之间仍有极短间隔；
// 只盯租约可能在落库尾巴上放手，于是目录清理依旧会撞上它。
func waitActiveTurnsDrained(t *testing.T, s *Server) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		ids := s.activeTurns.IDs()
		if len(ids) == 0 {
			time.Sleep(20 * time.Millisecond)
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("在飞轮次 15s 内未清空（扇出 goroutine 可能卡住）: %v", ids)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// ① 空指令 / 空成员:调用级参数错误,直接拒(不进入扇出)。
func TestDispatchCollabRejectsEmptyInput(t *testing.T) {
	s := newCollabSessionServer(t)
	if _, err := s.DispatchCollab(context.Background(), &pb.DispatchCollabRequest{
		Members: []*pb.CollabMember{{AgentId: "echo"}},
	}); err == nil || !strings.Contains(err.Error(), "指令不能为空") {
		t.Fatalf("空指令应被人话拒绝,实际: %v", err)
	}
	if _, err := s.DispatchCollab(context.Background(), &pb.DispatchCollabRequest{
		Instruction: "干活",
	}); err == nil || !strings.Contains(err.Error(), "至少要选一个") {
		t.Fatalf("空成员应被人话拒绝,实际: %v", err)
	}
}

// ② 扇出成功:每个成员各得一个子线程,顺序与请求一致(前端按位置回填相位)。
func TestDispatchCollabFansOutPerMember(t *testing.T) {
	s := newCollabSessionServer(t)
	resp, err := s.DispatchCollab(context.Background(), &pb.DispatchCollabRequest{
		Instruction: "审查这个函数的边界条件",
		Members: []*pb.CollabMember{
			{AgentId: "echo"},
			{AgentId: "echo"},
			{AgentId: "echo"},
		},
	})
	if err != nil {
		t.Fatalf("扇出失败: %v", err)
	}
	if resp.GetSessionId() == "" {
		t.Fatal("session_id 不应为空(前端靠它归组)")
	}
	if len(resp.GetItems()) != 3 {
		t.Fatalf("返回项应与成员数一致(3),实际 %d", len(resp.GetItems()))
	}
	seen := map[string]bool{}
	for i, it := range resp.GetItems() {
		if !it.GetAck() {
			t.Fatalf("第 %d 个成员应受理成功,却报: %s", i, it.GetError())
		}
		if it.GetThreadId() == "" {
			t.Fatalf("第 %d 个成员受理了却没有 thread_id", i)
		}
		if it.GetAgentId() != "echo" {
			t.Fatalf("第 %d 个成员 agent_id 应对位回填,实际 %q", i, it.GetAgentId())
		}
		if seen[it.GetThreadId()] {
			t.Fatalf("成员应各开一个独立子线程,却复用了 %s", it.GetThreadId())
		}
		seen[it.GetThreadId()] = true
	}
}

// ③ 一家失败不影响其它家(并行协作的应有语义)。
func TestDispatchCollabPartialFailureIsIndependent(t *testing.T) {
	s := newCollabSessionServer(t)
	resp, err := s.DispatchCollab(context.Background(), &pb.DispatchCollabRequest{
		Instruction: "各写一份评审意见",
		Members: []*pb.CollabMember{
			{AgentId: "echo"},
			{AgentId: "ghost"}, // 不存在的 agent
			{AgentId: "echo"},
		},
	})
	if err != nil {
		t.Fatalf("个别成员失败不该让整个 RPC 失败: %v", err)
	}
	if len(resp.GetItems()) != 3 {
		t.Fatalf("返回项应与成员数一致(3),实际 %d", len(resp.GetItems()))
	}
	if !resp.GetItems()[0].GetAck() {
		t.Fatal("第 1 个成员(有效 agent)应受理成功")
	}
	if !resp.GetItems()[2].GetAck() {
		t.Fatal("第 3 个成员(有效 agent)不该被中间那个失败成员拖垮")
	}
	bad := resp.GetItems()[1]
	if bad.GetAck() {
		t.Fatal("不存在的 agent 不该被受理")
	}
	if bad.GetError() == "" {
		t.Fatal("失败成员必须给**人话原因**,不能只留一个空的失败项")
	}
}

// ④ 角色链路:只给 role_id 时,agent / 人设都由角色提供,且人设真的进了子线程 prompt。
func TestDispatchCollabRoleSuppliesAgentAndPersona(t *testing.T) {
	s := newCollabSessionServer(t)
	if _, err := s.SetCollabRole(context.Background(), &pb.SetCollabRoleRequest{Role: &pb.CollabRole{
		Id: "reviewer", Name: "安全审查者", AgentId: "echo",
		Description: "只找安全问题,不修改任何代码", Instructions: "必须列出证据行号",
		Model: "echo/default",
	}}); err != nil {
		t.Fatal(err)
	}
	resp, err := s.DispatchCollab(context.Background(), &pb.DispatchCollabRequest{
		Instruction: "审查登录函数",
		Members:     []*pb.CollabMember{{RoleId: "reviewer"}}, // 不显式给 agent
	})
	if err != nil {
		t.Fatalf("扇出失败: %v", err)
	}
	it := resp.GetItems()[0]
	if !it.GetAck() {
		t.Fatalf("角色已配 agent,应能受理: %s", it.GetError())
	}
	if it.GetAgentId() != "echo" {
		t.Fatalf("agent 应由角色提供,实际 %q", it.GetAgentId())
	}
	// 人设必须真的落进子线程的首条 user 消息 —— 否则"配了角色"就是假的。
	got := waitFirstUserText(t, s, it.GetThreadId())
	for _, want := range []string{"安全审查者", "只找安全问题", "必须列出证据行号", "审查登录函数"} {
		if !strings.Contains(got, want) {
			t.Fatalf("子任务 prompt 应含 %q(角色人设未注入?): %q", want, got)
		}
	}
}

// ⑤ 未知角色:人话报错并列出可用 id,让调用方能自纠。
func TestDispatchCollabUnknownRole(t *testing.T) {
	s := newCollabSessionServer(t)
	if _, err := s.SetCollabRole(context.Background(), &pb.SetCollabRoleRequest{Role: &pb.CollabRole{Id: "reviewer"}}); err != nil {
		t.Fatal(err)
	}
	resp, err := s.DispatchCollab(context.Background(), &pb.DispatchCollabRequest{
		Instruction: "干活",
		Members:     []*pb.CollabMember{{RoleId: "nope"}},
	})
	if err != nil {
		t.Fatalf("未知角色是**成员级**业务失败,不该让整个 RPC 失败: %v", err)
	}
	got := resp.GetItems()[0].GetError()
	if !strings.Contains(got, "未定义的协同角色") || !strings.Contains(got, "reviewer") {
		t.Fatalf("应报人话并列出可用 id 供自纠,实际: %q", got)
	}
}

// ---- 辅助 ----

// firstTextOf 取一条落库消息里的第一段文本。
func firstTextOf(m store.StoredMessage) string {
	// 【vet】按下标取址遍历,避免 range 复制 pb 消息（内嵌 MessageState 含锁）。
	for i := range m.Blocks {
		if tb := m.Blocks[i].GetText(); tb != nil {
			return tb.GetText()
		}
	}
	return ""
}

// waitFirstUserText 轮询等异步轮次把首条 user 消息落库。
//
// 【为什么必须等而不是直接断言】扇出的轮次跑在**独立 goroutine** 里,RPC 返回
// 时它才刚起飞;此刻去读消息会撞上"还没落库"的竞态,那会让人误判成"人设没注入"
// —— 而实际上只是还没跑到。给一个有上界的等待,测的才是注入逻辑本身。
func waitFirstUserText(t *testing.T, s *Server, threadID string) string {
	t.Helper()
	for i := 0; i < 150; i++ {
		if th, ok := s.cfg.Store.Get(threadID); ok && len(th.Messages) > 0 {
			return firstTextOf(th.Messages[0])
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("等不到子线程 %s 的首条 user 消息(轮次未起飞?)", threadID)
	return ""
}

// ---- P1 协同闭环(Task 3/4)新增用例 ----

// collabTestProvider 测试用 provider:可编程的 Run,带最小模型目录
// (DispatchCollab → CreateThread 需要 defaultModelIDFor 能解析出模型)。
type collabTestProvider struct {
	id  string
	run func(context.Context, agent.TurnRequest, agent.Sink, agent.ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error)
}

func (p *collabTestProvider) ID() string          { return p.id }
func (p *collabTestProvider) DisplayName() string { return "collab-test" }
func (p *collabTestProvider) Models() []pb.ModelInfo {
	return []pb.ModelInfo{{Id: "m1", Provider: p.id, DisplayName: "test", ContextWindow: 1024, Available: true}}
}
func (p *collabTestProvider) Run(ctx context.Context, req agent.TurnRequest, sink agent.Sink, ask agent.ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
	return p.run(ctx, req, sink, ask)
}

// newCollabProvServer 用可编程 provider 建协同测试服务器。
// t.Cleanup 是 LIFO:先等在飞轮次落定(可能还往事件队列补终局帧),
// 再等事件队列排空 —— 之后才轮到 t.TempDir() 清理。
func newCollabProvServer(t *testing.T, ps ...agent.Provider) *Server {
	t.Helper()
	fs, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("建测试存储失败: %v", err)
	}
	providers := map[string]agent.Provider{}
	for _, p := range ps {
		providers[p.ID()] = p
	}
	s := New(Config{Store: fs, Providers: providers, Workspace: t.TempDir()})
	t.Cleanup(func() { waitCollabEventQueueDrained(t, s) })
	t.Cleanup(func() { waitActiveTurnsDrained(t, s) })
	return s
}

// waitCollabEventQueueDrained 等协同事件队列排空(含沉降窗口):
// 断言事件流之前必须排空,否则"最后一个事件还在队列里"会伪装成"事件丢了"。
func waitCollabEventQueueDrained(t *testing.T, s *Server) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if len(s.collabCoordinatorFor().evCh) == 0 {
			time.Sleep(30 * time.Millisecond) // 沉降:最后一个作业可能刚入队
			if len(s.collabCoordinatorFor().evCh) == 0 {
				return
			}
			continue
		}
		if time.Now().After(deadline) {
			t.Fatalf("协同事件队列 10s 未排空(消费者可能卡住)")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// collabSnapDebug 取快照的人读形态(断言失败信息用)。
func collabSnapDebug(s *Server, sessionID string) string {
	snap, err := s.collabCoordinatorFor().Snapshot(context.Background(), sessionID)
	if err != nil {
		return "<不可读: " + err.Error() + ">"
	}
	return snap.String()
}

// waitCollabMemberStatus 轮询等某成员卡到达期望状态(有界等待)。
func waitCollabMemberStatus(t *testing.T, s *Server, sessionID, threadID, status string) *pb.CollabMemberSnapshot {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		snap, err := s.collabCoordinatorFor().Snapshot(context.Background(), sessionID)
		if err == nil {
			for _, m := range snap.GetMembers() {
				if m.GetThreadId() == threadID {
					if m.GetStatus() == status {
						return m
					}
					break
				}
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("等成员 %s 到达 %s 超时,最后快照: %s", threadID, status, collabSnapDebug(s, sessionID))
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// ⑥ 生命周期:受理后立即可读全量快照;成员走完 ACCEPTED→RUNNING→COMPLETED;
// 事件序号全局严格单调;完成摘要只来自真实终稿(不把受理伪装成完成)。
func TestCollabSessionLifecycleAndEventSequence(t *testing.T) {
	s := newCollabSessionServer(t)
	resp, err := s.DispatchCollab(context.Background(), &pb.DispatchCollabRequest{
		Instruction: "各自总结一句结论",
		Members:     []*pb.CollabMember{{AgentId: "echo"}, {AgentId: "echo"}},
	})
	if err != nil {
		t.Fatalf("派发失败: %v", err)
	}
	// 受理态即可读:快照成员卡与响应项按位置对齐。
	snap, err := s.collabCoordinatorFor().Snapshot(context.Background(), resp.GetSessionId())
	if err != nil {
		t.Fatalf("受理后应能读到会话快照: %v", err)
	}
	if len(snap.GetMembers()) != 2 {
		t.Fatalf("快照应含 2 个成员卡,实际 %d", len(snap.GetMembers()))
	}
	for i, m := range snap.GetMembers() {
		if m.GetThreadId() != resp.GetItems()[i].GetThreadId() {
			t.Fatalf("快照成员 %d 的 thread_id 应与响应项一致", i)
		}
	}
	// 完成收敛:每个成员最终 COMPLETED(完成由真实终局写入,不是受理冒充)。
	for _, it := range resp.GetItems() {
		waitCollabMemberStatus(t, s, resp.GetSessionId(), it.GetThreadId(), string(store.CollabStatusCompleted))
	}
	waitCollabEventQueueDrained(t, s)

	// 事件流:全局序号严格单调;每个成员的状态轨迹是 ACCEPTED→RUNNING→COMPLETED。
	evs, err := s.collabCoordinatorFor().store.ReplayEvents(context.Background(), resp.GetSessionId(), 0)
	if err != nil {
		t.Fatalf("回放事件流失败: %v", err)
	}
	if len(evs) == 0 {
		t.Fatal("协同事件流不应为空")
	}
	last := int64(0)
	for _, ev := range evs {
		if ev.Sequence <= last {
			t.Fatalf("事件序号必须严格单调: %d 之后出现 %d", last, ev.Sequence)
		}
		last = ev.Sequence
	}
	want := []string{
		string(store.CollabStatusAccepted),
		string(store.CollabStatusRunning),
		string(store.CollabStatusCompleted),
	}
	for _, it := range resp.GetItems() {
		var kinds []string
		for _, ev := range evs {
			if ev.MemberThreadID != it.GetThreadId() {
				continue
			}
			switch ev.Kind {
			case string(store.CollabStatusAccepted), string(store.CollabStatusRunning),
				string(store.CollabStatusWaitingApproval), string(store.CollabStatusCompleted),
				string(store.CollabStatusFailed), string(store.CollabStatusCancelled):
				kinds = append(kinds, ev.Kind)
			}
		}
		if len(kinds) < len(want) {
			t.Fatalf("成员 %s 状态轨迹缺帧: %v", it.GetThreadId(), kinds)
		}
		for i, w := range want {
			if kinds[i] != w {
				t.Fatalf("成员 %s 状态轨迹应以 %v 开头,实际 %v", it.GetThreadId(), want, kinds)
			}
		}
	}
	// 全部完成后会话整体 COMPLETED,摘要非空(真实终稿的截断)。
	final, err := s.collabCoordinatorFor().Snapshot(context.Background(), resp.GetSessionId())
	if err != nil {
		t.Fatal(err)
	}
	if final.GetStatus() != string(store.CollabStatusCompleted) {
		t.Fatalf("全部成员完成后会话应为 COMPLETED,实际 %s", final.GetStatus())
	}
	for _, m := range final.GetMembers() {
		if m.GetSummary() == "" {
			t.Fatalf("完成成员 %s 应有真实摘要(不接受空摘要的假完成)", m.GetThreadId())
		}
	}
}

// ⑦ 成员失败:受理失败/执行失败都如实落 FAILED 成员卡 + 人话原因;
// 部分失败时会话整体如实标注 FAILED,不被完成成员美化。
func TestCollabMemberFailureLandsAsFailed(t *testing.T) {
	s := newCollabSessionServer(t)
	resp, err := s.DispatchCollab(context.Background(), &pb.DispatchCollabRequest{
		Instruction: "各写一份评审意见",
		Members:     []*pb.CollabMember{{AgentId: "echo"}, {AgentId: "ghost"}, {AgentId: "echo"}},
	})
	if err != nil {
		t.Fatalf("个别成员失败不该让整个 RPC 失败: %v", err)
	}
	bad := resp.GetItems()[1]
	if bad.GetAck() || bad.GetThreadId() != "" || bad.GetError() == "" {
		t.Fatalf("不存在的 agent 不该受理,且必须给人话原因: %+v", bad)
	}
	// 等有效成员完成:随后会话应收敛为 FAILED(有失败成员在)。
	for _, i := range []int{0, 2} {
		waitCollabMemberStatus(t, s, resp.GetSessionId(), resp.GetItems()[i].GetThreadId(), string(store.CollabStatusCompleted))
	}
	waitCollabEventQueueDrained(t, s)
	snap, err := s.collabCoordinatorFor().Snapshot(context.Background(), resp.GetSessionId())
	if err != nil {
		t.Fatal(err)
	}
	if snap.GetStatus() != string(store.CollabStatusFailed) {
		t.Fatalf("含失败成员的会话应如实标注 FAILED,实际 %s", snap.GetStatus())
	}
	if snap.GetError() == "" {
		t.Fatal("会话级失败原因不应为空(人话,直接可展示)")
	}
	m := snap.GetMembers()[1]
	if m.GetStatus() != string(store.CollabStatusFailed) || m.GetError() == "" {
		t.Fatalf("失败成员卡应为 FAILED+原因,实际 %+v", m)
	}
	// 事件流里必须有该成员的 FAILED 帧。
	evs, err := s.collabCoordinatorFor().store.ReplayEvents(context.Background(), resp.GetSessionId(), 0)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, ev := range evs {
		if ev.Kind == string(store.CollabStatusFailed) && ev.Member != nil && ev.Member.AgentID == "ghost" {
			found = true
		}
	}
	if !found {
		t.Fatal("事件流里应有失败成员的 FAILED 帧")
	}
}

// ⑧ 取消与迟到终局:取消收敛为 CANCELLED;同代际迟到的"完成"被终局闸拒绝
// (终局不复活);重试出新一代际后,旧代际的迟到完成被代际闸拒绝
// (旧轮次不得覆盖新轮次);重试完成后旧结果仍完整保留在事件流里。
func TestCollabCancelAndLateCompletionNail(t *testing.T) {
	release := make(chan struct{})
	p := &collabTestProvider{id: "echo", run: func(ctx context.Context, _ agent.TurnRequest, _ agent.Sink, _ agent.ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
		select {
		case <-release:
			return []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "重试后的真实成果"}}}}, &pb.Usage{}, nil
		case <-ctx.Done():
			return nil, &pb.Usage{}, ctx.Err()
		}
	}}
	s := newCollabProvServer(t, p)
	resp, err := s.DispatchCollab(context.Background(), &pb.DispatchCollabRequest{
		Instruction: "慢慢干",
		Members:     []*pb.CollabMember{{AgentId: "echo"}},
	})
	if err != nil {
		t.Fatalf("派发失败: %v", err)
	}
	sid, th := resp.GetSessionId(), resp.GetItems()[0].GetThreadId()
	waitCollabMemberStatus(t, s, sid, th, string(store.CollabStatusRunning))

	// 取消:只取消对应线程,状态由成员轮次自己收敛落库。
	if _, err := s.CancelCollabMember(context.Background(), &pb.CancelCollabMemberRequest{SessionId: sid, ThreadId: th}); err != nil {
		t.Fatalf("取消成员失败: %v", err)
	}
	card := waitCollabMemberStatus(t, s, sid, th, string(store.CollabStatusCancelled))
	if card.GetSummary() != "" {
		t.Fatalf("已取消成员不得出现终稿摘要: %q", card.GetSummary())
	}
	// 同代际迟到的"完成"被终局闸拒绝:已取消成员不复活。
	s.collabCoordinatorFor().finalizeMember(sid, th, 1, collabMemberOutcome{
		status: store.CollabStatusCompleted, summary: "迟到的伪造完成",
	})
	waitCollabEventQueueDrained(t, s)
	snap, err := s.collabCoordinatorFor().Snapshot(context.Background(), sid)
	if err != nil {
		t.Fatal(err)
	}
	m := snap.GetMembers()[0]
	if m.GetStatus() != string(store.CollabStatusCancelled) || m.GetSummary() != "" {
		t.Fatalf("迟到的完成不得复活已取消成员,实际 %+v", m)
	}
	evs, err := s.collabCoordinatorFor().store.ReplayEvents(context.Background(), sid, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, ev := range evs {
		if ev.MemberThreadID == th && ev.Kind == string(store.CollabStatusCompleted) {
			t.Fatal("已取消成员不应有 COMPLETED 事件帧(迟到终局被拒时也不发)")
		}
	}

	// 重试:新一代际重建轮次;旧代际(gen=1)的迟到完成必须被代际闸拒绝。
	if _, err := s.RetryCollabMember(context.Background(), &pb.RetryCollabMemberRequest{SessionId: sid, ThreadId: th}); err != nil {
		t.Fatalf("重试失败: %v", err)
	}
	waitCollabMemberStatus(t, s, sid, th, string(store.CollabStatusRunning))
	s.collabCoordinatorFor().finalizeMember(sid, th, 1, collabMemberOutcome{
		status: store.CollabStatusCompleted, summary: "旧代际的迟到完成",
	})
	waitCollabEventQueueDrained(t, s)
	snap, err = s.collabCoordinatorFor().Snapshot(context.Background(), sid)
	if err != nil {
		t.Fatal(err)
	}
	if m := snap.GetMembers()[0]; m.GetStatus() != string(store.CollabStatusRunning) || m.GetSummary() != "" {
		t.Fatalf("旧代际的迟到完成不得覆盖新代际运行中的成员,实际 %+v", m)
	}
	// 新代际轮次正常完成;COMPLETED 只能由真实终局写入。
	close(release)
	card = waitCollabMemberStatus(t, s, sid, th, string(store.CollabStatusCompleted))
	if card.GetSummary() != "重试后的真实成果" {
		t.Fatalf("重试完成后的摘要应来自新轮次终稿,实际 %q", card.GetSummary())
	}
	// 旧结果不覆盖:第一次运行的 CANCELLED 帧仍完整保留在事件流里。
	waitCollabEventQueueDrained(t, s)
	evs, err = s.collabCoordinatorFor().store.ReplayEvents(context.Background(), sid, 0)
	if err != nil {
		t.Fatal(err)
	}
	var kinds []string
	for _, ev := range evs {
		if ev.MemberThreadID == th {
			switch ev.Kind {
			case string(store.CollabStatusAccepted), string(store.CollabStatusRunning),
				string(store.CollabStatusCancelled), string(store.CollabStatusCompleted):
				kinds = append(kinds, ev.Kind)
			}
		}
	}
	wantKinds := []string{
		string(store.CollabStatusAccepted), string(store.CollabStatusRunning), string(store.CollabStatusCancelled),
		string(store.CollabStatusAccepted), string(store.CollabStatusRunning), string(store.CollabStatusCompleted),
	}
	if len(kinds) != len(wantKinds) {
		t.Fatalf("重试后事件轨迹应为 %v(旧帧保留),实际 %v", wantKinds, kinds)
	}
	for i := range wantKinds {
		if kinds[i] != wantKinds[i] {
			t.Fatalf("重试后事件轨迹应为 %v(旧帧保留),实际 %v", wantKinds, kinds)
		}
	}
}

// ⑨ 会话级取消收敛:全员(含在跑成员)在 bounded 等待内收敛为 CANCELLED,
// 会话整体 CANCELLED,CancelSession 不永久等。
func TestCollabSessionCancelConverges(t *testing.T) {
	p := &collabTestProvider{id: "echo", run: func(ctx context.Context, _ agent.TurnRequest, _ agent.Sink, _ agent.ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
		<-ctx.Done()
		return nil, &pb.Usage{}, ctx.Err()
	}}
	s := newCollabProvServer(t, p)
	resp, err := s.DispatchCollab(context.Background(), &pb.DispatchCollabRequest{
		Instruction: "都停下",
		Members:     []*pb.CollabMember{{AgentId: "echo"}, {AgentId: "echo"}},
	})
	if err != nil {
		t.Fatalf("派发失败: %v", err)
	}
	for _, it := range resp.GetItems() {
		waitCollabMemberStatus(t, s, resp.GetSessionId(), it.GetThreadId(), string(store.CollabStatusRunning))
	}
	snap, err := s.CancelCollabSession(context.Background(), &pb.CancelCollabSessionRequest{SessionId: resp.GetSessionId()})
	if err != nil {
		t.Fatalf("会话取消应在有界等待内收敛: %v", err)
	}
	if snap.GetStatus() != string(store.CollabStatusCancelled) {
		t.Fatalf("取消后会话应为 CANCELLED,实际 %s", snap.GetStatus())
	}
	for _, m := range snap.GetMembers() {
		if m.GetStatus() != string(store.CollabStatusCancelled) {
			t.Fatalf("取消后成员 %s 应为 CANCELLED,实际 %s", m.GetThreadId(), m.GetStatus())
		}
	}
}
