// collab_e2e_test.go —— 协同 daemon E2E(2026-10-04 发布验证计划 Task 2 的
// 场景 1/2/3/4/5/8/9;场景 6/7 的重启恢复见 collab_recovery_e2e_test.go)。
//
// 【与既有 collab 测试的分工】collab_session_test / collab_approval_test 用
// collabTestProvider 钉状态机与审批映射的单元语义;本文件把 P5-T1 的
// deterministic fake(testagent.Factory)经 server.Config.Providers 灌入
// (provider 表是唯一灌入点,无旁路),在真 Server + 真 FileStore + 真
// FileCollabSessionStore(DataDir 注入)上跑用户可观察的终局:快照、事件流、
// 取消收敛、审批决定、工作区隔离证据 —— 不只检查 RPC 返回码。
//
// 【审批红线】fake 只负责"发起审批并等待";允许/拒绝的决定由测试走**真实手机
// 链路**(成员卡上的真实 request_id → 既有 SubmitApproval RPC)注入,
// 绝不伪造用户意志(计划全局约束第 4 条)。
//
// 【时序纪律】场景延迟一律毫秒级(学 testagent 的时序纪律:断言只做下界与
// 宽上界,不贴线);等待全部用有界轮询;清理沿用既有 LIFO 纪律(先等在飞轮次
// 落定,再等协同事件队列排空,再关 eventstore 的 events.jsonl 句柄,最后才轮
// 到 t.TempDir)。
package server

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/store"
	"github.com/pocketagent/daemon/internal/testagent"
	"google.golang.org/protobuf/proto"
)

// ---- 装配:testagent fake 经 Config.Providers 灌入真 Server ----

// collabE2EFactory 建一个台账隔离(runtime.NewBackend,不碰生产共享账)、
// CollabReady 的 fake 工厂。
func collabE2EFactory() *testagent.Factory {
	return &testagent.Factory{Ledger: runtime.NewBackend(), CollabReadyDefault: true}
}

// newCollabE2EServer 真 Server + 真 FileStore + 真 FileCollabSessionStore,
// fake provider 灌入 cfg.Providers。返回 server 与 dataDir(场景 8/9 的工作区
// 管理器要用同一 dataDir 落证据)。
func newCollabE2EServer(t *testing.T, fakes ...*testagent.FakeAgent) (*Server, string) {
	t.Helper()
	dataDir := t.TempDir()
	return newCollabE2EServerAt(t, dataDir, fakes...), dataDir
}

// newCollabE2EServerAt 在指定 dataDir 上建 Server(重启恢复场景用它把两代
// daemon 指到同一份数据)。清理纪律与 collab_session_test 相同。
func newCollabE2EServerAt(t *testing.T, dataDir string, fakes ...*testagent.FakeAgent) *Server {
	t.Helper()
	fs, err := store.NewFileStore(dataDir)
	if err != nil {
		t.Fatalf("建测试存储失败: %v", err)
	}
	providers := make(map[string]agent.Provider, len(fakes))
	for _, f := range fakes {
		providers[f.ID()] = f
	}
	s := New(Config{
		Store:     fs,
		Providers: providers,
		Workspace: t.TempDir(),
		DataDir:   dataDir,
	})
	// t.Cleanup 是 LIFO:先等在飞轮次落定(它们还会往事件队列补终局帧),
	// 再等协同事件队列排空,然后关闭 eventstore 的 events.jsonl 句柄(New 里
	// eventstore.Open 持常驻句柄,不关会在 t.TempDir() RemoveAll 时报
	// "being used by another process" —— 与 control_plane_test.go 等处
	// `srv.events.Close()` 同款纪律),最后才轮到 t.TempDir 的目录清理。
	t.Cleanup(func() { _ = s.events.Close() })
	t.Cleanup(func() { waitCollabEventQueueDrained(t, s) })
	t.Cleanup(func() { waitActiveTurnsDrained(t, s) })
	return s
}

// dispatchCollabE2E 派发一个真协同会话,成员按给定 agent id 顺序受理。
func dispatchCollabE2E(t *testing.T, s *Server, instruction, workspace string, agentIDs ...string) *pb.DispatchCollabResponse {
	t.Helper()
	members := make([]*pb.CollabMember, 0, len(agentIDs))
	for _, id := range agentIDs {
		members = append(members, &pb.CollabMember{AgentId: id})
	}
	resp, err := s.DispatchCollab(context.Background(), &pb.DispatchCollabRequest{
		Instruction: instruction,
		Workspace:   workspace,
		Members:     members,
	})
	if err != nil {
		t.Fatalf("协同派发失败: %v", err)
	}
	if resp.GetSessionId() == "" {
		t.Fatal("session_id 不应为空")
	}
	return resp
}

// ---- Watch / 事件流断言辅助 ----

// collectCollabFrames 从 Watch 通道有界收集恰好 n 帧。
func collectCollabFrames(t *testing.T, ch <-chan *pb.CollabSessionEvent, n int) []*pb.CollabSessionEvent {
	t.Helper()
	out := make([]*pb.CollabSessionEvent, 0, n)
	deadline := time.After(10 * time.Second)
	for len(out) < n {
		select {
		case ev, ok := <-ch:
			if !ok {
				t.Fatalf("Watch 通道提前关闭:已收 %d/%d 帧", len(out), n)
			}
			out = append(out, ev)
		case <-deadline:
			t.Fatalf("等 Watch 帧 10s 超时:已收 %d/%d", len(out), n)
		}
	}
	return out
}

// collectCollabFramesUntil 从 Watch 通道有界收集直到条件帧出现,返回含条件帧
// 在内的全部已收帧。
func collectCollabFramesUntil(t *testing.T, ch <-chan *pb.CollabSessionEvent, cond func(*pb.CollabSessionEvent) bool) []*pb.CollabSessionEvent {
	t.Helper()
	var out []*pb.CollabSessionEvent
	deadline := time.After(10 * time.Second)
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				t.Fatalf("Watch 通道提前关闭:已收 %d 帧", len(out))
			}
			out = append(out, ev)
			if cond(ev) {
				return out
			}
		case <-deadline:
			t.Fatalf("等 Watch 条件帧 10s 超时:已收 %d 帧", len(out))
		}
	}
}

// collabMemberKinds 取事件流里某成员的状态迁移轨迹(ACCEPTED/RUNNING/
// WAITING_APPROVAL/COMPLETED/FAILED/CANCELLED;PROGRESS/SNAPSHOT 不算)。
func collabMemberKinds(evs []store.CollabEvent, threadID string) []string {
	var kinds []string
	for i := range evs {
		ev := &evs[i]
		if ev.MemberThreadID != threadID {
			continue
		}
		switch ev.Kind {
		case string(store.CollabStatusAccepted), string(store.CollabStatusRunning),
			string(store.CollabStatusWaitingApproval), string(store.CollabStatusCompleted),
			string(store.CollabStatusFailed), string(store.CollabStatusCancelled):
			kinds = append(kinds, ev.Kind)
		}
	}
	return kinds
}

// replayCollabEvents 回放会话全部持久化事件(断言前必须已等事件队列排空)。
func replayCollabEvents(t *testing.T, s *Server, sessionID string) []store.CollabEvent {
	t.Helper()
	evs, err := s.collabCoordinatorFor().store.ReplayEvents(context.Background(), sessionID, 0)
	if err != nil {
		t.Fatalf("回放协同事件流失败: %v", err)
	}
	return evs
}

// quiesceCollabServer 等一个 server 的在飞轮次与事件队列全部落定
// (重启恢复场景:二代 daemon 打开同一 dataDir 前,一代必须先停止一切写入)。
func quiesceCollabServer(t *testing.T, s *Server) {
	t.Helper()
	waitActiveTurnsDrained(t, s)
	waitCollabEventQueueDrained(t, s)
}

// ============ 场景 1:两成员并行分析均完成,App/daemon 终局一致 ============

// 两成员并行完成:成员卡各自 COMPLETED 且摘要来自真实终稿;事件流序号严格
// 单调、每位成员轨迹 ACCEPTED→RUNNING→COMPLETED;**末帧全量快照与协调器
// Snapshot 逐字段一致** —— 重放整条事件流的客户端(App)收屏时看到的和
// daemon 真值是同一份,不许出现"快照一个样、事件流另一个样"。
func TestCollabE2EParallelBothCompleteSnapshotMatchesStreamTail(t *testing.T) {
	f := collabE2EFactory()
	fa := f.Provider("fa", testagent.Scenario{
		Name:  "成员A:两段文本后完成",
		Delay: 5 * time.Millisecond,
		Events: []testagent.AgentEvent{
			testagent.Text("A:第一段分析"),
			testagent.Text("A:第二段分析"),
			testagent.Completed("A 的最终结论:边界条件无问题"),
		},
	}).(*testagent.FakeAgent)
	fb := f.Provider("fb", testagent.Scenario{
		Name:  "成员B:工具调用后完成",
		Delay: 15 * time.Millisecond,
		Events: []testagent.AgentEvent{
			testagent.ToolCall("c1", "list_dir", `{"path":"."}`),
			testagent.ToolResult("c1", true, "a.txt\nb.txt"),
			testagent.Completed("B 的最终结论:目录结构符合预期"),
		},
	}).(*testagent.FakeAgent)
	s, _ := newCollabE2EServer(t, fa, fb)

	resp := dispatchCollabE2E(t, s, "并行分析这个模块", "", "fa", "fb")
	sid := resp.GetSessionId()
	wantSummary := map[string]string{
		resp.GetItems()[0].GetThreadId(): "A 的最终结论:边界条件无问题",
		resp.GetItems()[1].GetThreadId(): "B 的最终结论:目录结构符合预期",
	}

	// 双成员收敛 COMPLETED,摘要 = fake 终稿(完成只能来自真实终局)。
	for th, want := range wantSummary {
		card := waitCollabMemberStatus(t, s, sid, th, string(store.CollabStatusCompleted))
		if card.GetSummary() != want {
			t.Fatalf("成员 %s 摘应来自真实终稿: want %q got %q", th, want, card.GetSummary())
		}
	}
	quiesceCollabServer(t, s)

	final, err := s.collabCoordinatorFor().Snapshot(context.Background(), sid)
	if err != nil {
		t.Fatalf("终局快照读取失败: %v", err)
	}
	if final.GetStatus() != string(store.CollabStatusCompleted) {
		t.Fatalf("全员完成后会话应 COMPLETED,实际 %s", final.GetStatus())
	}

	// 事件流:序号全局严格单调;每位成员轨迹 ACCEPTED→RUNNING→COMPLETED。
	evs := replayCollabEvents(t, s, sid)
	if len(evs) == 0 {
		t.Fatal("协同事件流不应为空")
	}
	lastSeq := int64(0)
	for i := range evs {
		if evs[i].Sequence <= lastSeq {
			t.Fatalf("事件序号必须严格单调: %d 之后出现 %d", lastSeq, evs[i].Sequence)
		}
		lastSeq = evs[i].Sequence
	}
	for _, it := range resp.GetItems() {
		kinds := collabMemberKinds(evs, it.GetThreadId())
		want := []string{string(store.CollabStatusAccepted), string(store.CollabStatusRunning), string(store.CollabStatusCompleted)}
		if len(kinds) < len(want) {
			t.Fatalf("成员 %s 状态轨迹缺帧: %v", it.GetThreadId(), kinds)
		}
		for i, w := range want {
			if kinds[i] != w {
				t.Fatalf("成员 %s 轨迹应以 ACCEPTED→RUNNING→COMPLETED 开头,实际 %v", it.GetThreadId(), kinds)
			}
		}
	}

	// 终局一致(核心断言):最后一帧是会话级 SNAPSHOT,且与协调器 Snapshot
	// 逐字段相等 —— App 重放到末帧即得到 daemon 真值。
	last := &evs[len(evs)-1]
	if last.Kind != collabEventKindSnapshot || last.Snapshot == nil {
		t.Fatalf("末帧应为全量快照,实际 kind=%s snapshot=%v", last.Kind, last.Snapshot != nil)
	}
	if !proto.Equal(collabSessionToPb(*last.Snapshot), final) {
		t.Fatalf("事件流末帧快照与协调器 Snapshot 不一致:\n末帧: %s\n真值: %s", final, collabSessionToPb(*last.Snapshot))
	}

	// App 路径(Watch after=0 重放整条流)收到的末帧同一致性。
	wctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := s.collabCoordinatorFor().Watch(wctx, sid, 0)
	if err != nil {
		t.Fatalf("Watch 订阅失败: %v", err)
	}
	frames := collectCollabFrames(t, ch, len(evs))
	tail := frames[len(frames)-1]
	if tail.GetSequence() != lastSeq || !proto.Equal(tail.GetSnapshot(), final) {
		t.Fatalf("Watch 重放末帧应与真值一致: seq=%d", tail.GetSequence())
	}
}

// ============ 场景 2:一成员失败,另一成员完成,部分摘要保留 ============

// 失败成员如实落 FAILED+人话原因(错误原文来自 fake 的 provider 错误,与真
// agent 失败同构);完成成员的摘要不被失败成员掩盖;会话整体如实标注 FAILED
// 并聚合人话原因 —— 部分成功不被美化成全绿。
func TestCollabE2EOneMemberFailsOtherCompletesKeepsPartialSummary(t *testing.T) {
	f := collabE2EFactory()
	fa := f.Provider("fa", testagent.Scenario{
		Name: "成员A:中途失败",
		Events: []testagent.AgentEvent{
			testagent.Text("A:开始分析"),
			testagent.FailedText("评审数据源不可用,已中止"),
		},
	}).(*testagent.FakeAgent)
	fb := f.Provider("fb", testagent.Scenario{
		Name: "成员B:正常完成",
		Events: []testagent.AgentEvent{
			testagent.Text("B:分析完成"),
			testagent.Completed("B 的评审结论:无阻塞问题"),
		},
	}).(*testagent.FakeAgent)
	s, _ := newCollabE2EServer(t, fa, fb)

	resp := dispatchCollabE2E(t, s, "各自评审", "", "fa", "fb")
	sid, thA, thB := resp.GetSessionId(), resp.GetItems()[0].GetThreadId(), resp.GetItems()[1].GetThreadId()

	bad := waitCollabMemberStatus(t, s, sid, thA, string(store.CollabStatusFailed))
	if !strings.Contains(bad.GetError(), "评审数据源不可用") {
		t.Fatalf("失败成员应带人话原因(错误原文),实际 %q", bad.GetError())
	}
	if bad.GetSummary() != "" {
		t.Fatalf("失败成员不得有终稿摘要: %q", bad.GetSummary())
	}
	done := waitCollabMemberStatus(t, s, sid, thB, string(store.CollabStatusCompleted))
	if done.GetSummary() != "B 的评审结论:无阻塞问题" {
		t.Fatalf("完成成员的摘要应保留(部分摘要不被失败掩盖),实际 %q", done.GetSummary())
	}
	quiesceCollabServer(t, s)

	snap, err := s.collabCoordinatorFor().Snapshot(context.Background(), sid)
	if err != nil {
		t.Fatal(err)
	}
	if snap.GetStatus() != string(store.CollabStatusFailed) {
		t.Fatalf("含失败成员的会话应如实标注 FAILED,实际 %s", snap.GetStatus())
	}
	if !strings.Contains(snap.GetError(), "1 个成员未完成") {
		t.Fatalf("会话级失败原因应是人话聚合,实际 %q", snap.GetError())
	}
	// 事件流:失败成员有 FAILED 帧、完成成员有 COMPLETED 帧,按成员锚定不错位。
	evs := replayCollabEvents(t, s, sid)
	var badFailed, okDone bool
	for i := range evs {
		ev := &evs[i]
		if ev.MemberThreadID == thA && ev.Kind == string(store.CollabStatusFailed) && ev.Member != nil && ev.Member.AgentID == "fa" {
			badFailed = true
		}
		if ev.MemberThreadID == thB && ev.Kind == string(store.CollabStatusCompleted) {
			okDone = true
		}
	}
	if !badFailed || !okDone {
		t.Fatalf("事件流应含 A 的 FAILED 帧与 B 的 COMPLETED 帧: %v/%v", badFailed, okDone)
	}
}

// ============ 场景 3:一成员等待审批,另一成员继续 ============

// fake 发起审批(决定完全来自注入链路):成员卡 WAITING_APPROVAL 且挂着真实
// request_id;**等待不阻塞他人** —— 另一成员照常 COMPLETED,会话如实显示
// WAITING_APPROVAL(等待不是终局,绝不被误报成完成);经既有 SubmitApproval
// 批准后等待成员继续并真实完成。
func TestCollabE2EWaitingApprovalDoesNotBlockOthers(t *testing.T) {
	f := collabE2EFactory()
	fa := f.Provider("fa", testagent.Scenario{
		Name: "成员A:等审批",
		Approval: &testagent.ApprovalScenario{
			CallName: "shell",
			JSONArgs: `{"command":"rm -rf build"}`,
		},
		Events: []testagent.AgentEvent{
			testagent.Text("A:审批通过后继续"),
			testagent.Completed("A 已获批准,清理完成"),
		},
	}).(*testagent.FakeAgent)
	fb := f.Provider("fb", testagent.Scenario{
		Name:  "成员B:不受等待影响",
		Delay: 10 * time.Millisecond,
		Events: []testagent.AgentEvent{
			testagent.Text("B:先跑完"),
			testagent.Completed("B 先跑完了"),
		},
	}).(*testagent.FakeAgent)
	s, _ := newCollabE2EServer(t, fa, fb)

	resp := dispatchCollabE2E(t, s, "一个要审批,一个先干", "", "fa", "fb")
	sid, thA, thB := resp.GetSessionId(), resp.GetItems()[0].GetThreadId(), resp.GetItems()[1].GetThreadId()

	// A 停在审批上(卡上挂真实 request_id),B 在同一窗口内完成。
	var snap *pb.CollabSessionSnapshot
	waitCollabMembers(t, s, sid, func(cur *pb.CollabSessionSnapshot) bool {
		a, b := collabMemberByThread(cur, thA), collabMemberByThread(cur, thB)
		ready := a != nil && b != nil &&
			a.GetStatus() == string(store.CollabStatusWaitingApproval) && len(a.GetPendingApprovals()) == 1 &&
			b.GetStatus() == string(store.CollabStatusCompleted)
		if ready {
			snap = cur
		}
		return ready
	})
	a := collabMemberByThread(snap, thA)
	if !strings.Contains(a.GetProgress(), "等待审批") {
		t.Fatalf("等待成员的进度应是人话审批文案,实际 %q", a.GetProgress())
	}
	if got := snap.GetStatus(); got != string(store.CollabStatusWaitingApproval) {
		t.Fatalf("一个成员等审批、其他已完成时会话应如实显示 WAITING_APPROVAL,实际 %s", got)
	}
	if b := collabMemberByThread(snap, thB); b.GetSummary() == "" {
		t.Fatal("B 的结果不应被 A 的等待掩盖")
	}

	// 审批决定经真实链路注入(成员卡上的 request_id → SubmitApproval),
	// fake 侧从不自拟决定。
	reqID := a.GetPendingApprovals()[0].GetRequestId()
	if reqID == "" {
		t.Fatal("挂起审批必须携带真实 request_id")
	}
	if _, err := s.SubmitApproval(context.Background(), &pb.SubmitApprovalRequest{
		ThreadId: thA,
		Decision: &pb.ApprovalDecision{RequestId: reqID, Approved: true},
	}); err != nil {
		t.Fatalf("SubmitApproval 应答失败: %v", err)
	}
	card := waitCollabMemberStatus(t, s, sid, thA, string(store.CollabStatusCompleted))
	if card.GetSummary() != "A 已获批准,清理完成" {
		t.Fatalf("批准后 A 应走完剧本并留真实摘要,实际 %q", card.GetSummary())
	}
	waitCollabMembers(t, s, sid, func(cur *pb.CollabSessionSnapshot) bool {
		return cur.GetStatus() == string(store.CollabStatusCompleted)
	})
	// 决定确实来自注入链路:fake 的审批观察应记录"已允许、无通道错误"。
	obs := fa.Approvals()
	if len(obs) != 1 || !obs[0].Allow || obs[0].Err != nil {
		t.Fatalf("审批观察不符(决定应来自注入链路): %+v", obs)
	}
}

// ============ 场景 4:取消单成员,他员不受影响 ============

// 只取消目标成员:其轮次经真实取消链路(activeTurns → provider ctx)收敛为
// CANCELLED,摘要为空(取消没有终稿);另一成员的 COMPLETED 卡与摘要原样保留。
func TestCollabE2ECancelSingleMemberOthersUnaffected(t *testing.T) {
	f := collabE2EFactory()
	fa := f.Provider("fa", testagent.Scenario{
		Name: "成员A:被取消的长任务",
		Events: []testagent.AgentEvent{
			testagent.Text("A:开工"),
			testagent.Pause(10 * time.Second), // 毫秒级场景里的"长任务",取消必然落在静默中
			testagent.Completed("永远到不了"),
		},
	}).(*testagent.FakeAgent)
	fb := f.Provider("fb", testagent.Scenario{
		Name: "成员B:快速完成",
		Events: []testagent.AgentEvent{
			testagent.Completed("B 的独立结论"),
		},
	}).(*testagent.FakeAgent)
	s, _ := newCollabE2EServer(t, fa, fb)

	resp := dispatchCollabE2E(t, s, "分工进行", "", "fa", "fb")
	sid, thA, thB := resp.GetSessionId(), resp.GetItems()[0].GetThreadId(), resp.GetItems()[1].GetThreadId()

	waitCollabMemberStatus(t, s, sid, thA, string(store.CollabStatusRunning))
	waitCollabMemberStatus(t, s, sid, thB, string(store.CollabStatusCompleted))

	if _, err := s.CancelCollabMember(context.Background(), &pb.CancelCollabMemberRequest{
		SessionId: sid, ThreadId: thA,
	}); err != nil {
		t.Fatalf("取消成员失败: %v", err)
	}
	card := waitCollabMemberStatus(t, s, sid, thA, string(store.CollabStatusCancelled))
	if !strings.Contains(card.GetError(), "已取消") {
		t.Fatalf("已取消成员应带人话原因,实际 %q", card.GetError())
	}
	if card.GetSummary() != "" {
		t.Fatalf("已取消成员不得出现终稿摘要: %q", card.GetSummary())
	}
	// 他员不受影响:COMPLETED 卡与摘要原样保留。
	snap, err := s.collabCoordinatorFor().Snapshot(context.Background(), sid)
	if err != nil {
		t.Fatal(err)
	}
	b := collabMemberByThread(snap, thB)
	if b == nil || b.GetStatus() != string(store.CollabStatusCompleted) || b.GetSummary() != "B 的独立结论" {
		t.Fatalf("取消一个成员不得影响其他成员,实际 %+v", b)
	}
	// 会话整体按派生规则如实标注(有 CANCELLED 成员即 CANCELLED),
	// 但成员卡各自保留真相。
	if snap.GetStatus() != string(store.CollabStatusCancelled) {
		t.Fatalf("会话应派生为 CANCELLED,实际 %s", snap.GetStatus())
	}
}

// ============ 场景 5:取消整个 session,全员有界收敛 ============

// 会话级取消:全部成员(含在跑的)在 CancelSession 的有界等待内收敛为
// CANCELLED,会话整体 CANCELLED、错误文案"协同已取消" —— 不永久等,也不留
// 僵尸轮次。
func TestCollabE2ECancelWholeSessionConvergesAllMembers(t *testing.T) {
	f := collabE2EFactory()
	scenario := testagent.Scenario{
		Name: "在跑长任务",
		Events: []testagent.AgentEvent{
			testagent.Text("开工"),
			testagent.Pause(10 * time.Second),
			testagent.Completed("永远到不了"),
		},
	}
	fa := f.Provider("fa", scenario).(*testagent.FakeAgent)
	fb := f.Provider("fb", scenario).(*testagent.FakeAgent)
	s, _ := newCollabE2EServer(t, fa, fb)

	resp := dispatchCollabE2E(t, s, "都停下", "", "fa", "fb")
	sid := resp.GetSessionId()
	for _, it := range resp.GetItems() {
		waitCollabMemberStatus(t, s, sid, it.GetThreadId(), string(store.CollabStatusRunning))
	}

	start := time.Now()
	snap, err := s.CancelCollabSession(context.Background(), &pb.CancelCollabSessionRequest{SessionId: sid})
	if err != nil {
		t.Fatalf("会话取消应在有界等待内收敛(超时会给人话错误): %v", err)
	}
	if el := time.Since(start); el > 9*time.Second {
		t.Fatalf("取消收敛不应贴着上界(10s),实际 %v", el)
	}
	if snap.GetStatus() != string(store.CollabStatusCancelled) {
		t.Fatalf("取消后会话应 CANCELLED,实际 %s", snap.GetStatus())
	}
	if snap.GetError() != "协同已取消" {
		t.Fatalf("会话级取消原因应为人话,实际 %q", snap.GetError())
	}
	for _, m := range snap.GetMembers() {
		if m.GetStatus() != string(store.CollabStatusCancelled) {
			t.Fatalf("成员 %s 应收敛为 CANCELLED,实际 %s", m.GetThreadId(), m.GetStatus())
		}
		if len(m.GetPendingApprovals()) != 0 {
			t.Fatalf("已取消成员 %s 不应还挂着审批", m.GetThreadId())
		}
	}
}

// ============ 场景 8:隔离 worktree 产生独立 diff ============

// 两个成员在同一会话下各得独立 worktree + 独立分支(collab/<会话>/<成员>),
// 各自产出互不可见;留证清理后,证据目录按成员分目录,各自 diff 只含自己的
// 改动 —— 独立分支 + 独立 diff 证据两件都在。git 用 fake runner(按
// collab_workspace_test 先例:钉"我们调 git 的方式与解析方式",不依赖环境)。
func TestCollabE2EIsolatedWorktreesIndependentBranchAndDiff(t *testing.T) {
	f := collabE2EFactory()
	fa := f.Provider("fa", testagent.Scenario{Events: []testagent.AgentEvent{testagent.Completed("A 完成")}}).(*testagent.FakeAgent)
	fb := f.Provider("fb", testagent.Scenario{Events: []testagent.AgentEvent{testagent.Completed("B 完成")}}).(*testagent.FakeAgent)
	s, dataDir := newCollabE2EServer(t, fa, fb)

	resp := dispatchCollabE2E(t, s, "各自改各自分支", "", "fa", "fb")
	sid, thA, thB := resp.GetSessionId(), resp.GetItems()[0].GetThreadId(), resp.GetItems()[1].GetThreadId()
	for _, it := range resp.GetItems() {
		waitCollabMemberStatus(t, s, sid, it.GetThreadId(), string(store.CollabStatusCompleted))
	}

	ctx := context.Background()
	repo := t.TempDir()
	fake := newFakeGit(repo)
	mgr := NewFileCollabWorkspaceManager(dataDir)
	mgr.runGit = fake.run

	// 两成员先后授予:worktree 目录、分支名互不相同,都固定在
	// <repo>/.collab-worktrees/<会话>/<成员> 下(成员标识 = 子线程 id,Task 3 接线契约)。
	leaseA, err := mgr.Prepare(ctx, sid, thA, repo, WorkspaceIsolatedWorktree)
	if err != nil {
		t.Fatalf("隔离授予(A)失败: %v", err)
	}
	leaseB, err := mgr.Prepare(ctx, sid, thB, repo, WorkspaceIsolatedWorktree)
	if err != nil {
		t.Fatalf("隔离授予(B)失败: %v", err)
	}
	if strings.EqualFold(filepath.Clean(leaseA.WorkPath), filepath.Clean(leaseB.WorkPath)) {
		t.Fatalf("两成员 worktree 必须不同: %s", leaseA.WorkPath)
	}
	if leaseA.Branch == leaseB.Branch {
		t.Fatalf("两成员分支必须不同: %s", leaseA.Branch)
	}
	wantBranchA, wantBranchB := "collab/"+sid+"/"+thA, "collab/"+sid+"/"+thB
	if leaseA.Branch != wantBranchA || leaseB.Branch != wantBranchB {
		t.Fatalf("分支名应为 collab/<会话>/<成员>: %s / %s", leaseA.Branch, leaseB.Branch)
	}
	if !fake.hasCall("worktree add -b "+wantBranchA) || !fake.hasCall("worktree add -b "+wantBranchB) {
		t.Fatalf("两成员都应以 git worktree add -b 建独立分支: %v", fake.calls)
	}

	// 各成员在自己的 worktree 里产出(互不可见的两个目录)。
	writeMemberOutput := func(dir, name string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte("member work: "+name), 0o644); err != nil {
			t.Fatalf("写成员成果失败: %v", err)
		}
	}
	writeMemberOutput(leaseA.WorkPath, "analysis-a.md")
	writeMemberOutput(leaseB.WorkPath, "analysis-b.md")

	// 让 fake git 认领两个 worktree,再按成员各自留证清理。
	fake.list = porcelainEntry(leaseA.WorkPath, leaseA.Branch) + porcelainEntry(leaseB.WorkPath, leaseB.Branch)
	cleanupWithEvidence := func(lease WorkspaceLease, marker string) {
		t.Helper()
		fake.status = "?? " + marker + "\n"
		fake.diffWork = "diff --git a/" + marker + " b/" + marker + "\n--- /dev/null\n+++ b/" + marker + "\n"
		fake.diffCommitted = "diff --git a/committed-" + marker + " b/committed-" + marker + "\n"
		if err := mgr.Cleanup(ctx, lease, true); err != nil {
			t.Fatalf("留证清理(%s)失败: %v", marker, err)
		}
		if _, err := os.Stat(lease.WorkPath); !os.IsNotExist(err) {
			t.Fatalf("清理后 %s 的 worktree 目录应消失: %v", marker, err)
		}
	}
	cleanupWithEvidence(leaseA, "analysis-a.md")
	cleanupWithEvidence(leaseB, "analysis-b.md")

	// 证据按成员分目录、互不串账:各自 diff 只含自己的改动。
	for _, tc := range []struct {
		memberID, marker, other string
	}{
		{thA, "analysis-a.md", "analysis-b.md"},
		{thB, "analysis-b.md", "analysis-a.md"},
	} {
		dir := filepath.Join(dataDir, collabDataDirName, collabArtifactsDirName, sid, tc.memberID)
		uncommitted, err := os.ReadFile(filepath.Join(dir, "diff-uncommitted.patch"))
		if err != nil || !strings.Contains(string(uncommitted), tc.marker) {
			t.Fatalf("成员 %s 的未提交 diff 证据应只含自己的改动(%s): %v %q", tc.memberID, tc.marker, err, uncommitted)
		}
		if strings.Contains(string(uncommitted), tc.other) {
			t.Fatalf("成员 %s 的 diff 不得串入他人改动(%s): %q", tc.memberID, tc.other, uncommitted)
		}
		status, err := os.ReadFile(filepath.Join(dir, "status.txt"))
		if err != nil || !strings.Contains(string(status), tc.marker) {
			t.Fatalf("成员 %s 的现场清单应落盘: %v %q", tc.memberID, err, status)
		}
	}
	if got := len(mgr.List()); got != 0 {
		t.Fatalf("两成员都清理后租约表应清空,实际 %d", got)
	}
}

// 场景 8 的真 git 形态:同一条链路换真实 git CLI(init → 两成员独立 worktree/
// 分支 → 各自产出 → 留证清理 → 分支保留、真实 diff 证据)。环境差异按既有
// 先例用 Short/无 git 跳过。
func TestCollabE2EIsolatedWorktreesRealGit(t *testing.T) {
	if testing.Short() {
		t.Skip("short 模式跳过依赖 git CLI 的集成测试")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("环境没有 git CLI")
	}
	f := collabE2EFactory()
	fa := f.Provider("fa", testagent.Scenario{Events: []testagent.AgentEvent{testagent.Completed("A 完成")}}).(*testagent.FakeAgent)
	fb := f.Provider("fb", testagent.Scenario{Events: []testagent.AgentEvent{testagent.Completed("B 完成")}}).(*testagent.FakeAgent)
	s, dataDir := newCollabE2EServer(t, fa, fb)

	resp := dispatchCollabE2E(t, s, "各自改各自分支", "", "fa", "fb")
	sid, thA, thB := resp.GetSessionId(), resp.GetItems()[0].GetThreadId(), resp.GetItems()[1].GetThreadId()
	for _, it := range resp.GetItems() {
		waitCollabMemberStatus(t, s, sid, it.GetThreadId(), string(store.CollabStatusCompleted))
	}

	ctx := context.Background()
	repo := t.TempDir()
	if _, out, err := execGit(ctx, repo, "init"); err != nil {
		t.Fatalf("git init 失败: %s (%v)", out, err)
	}
	if _, out, err := execGit(ctx, repo, "config", "user.email", "collab@example.com"); err != nil {
		t.Fatalf("git config 失败: %s (%v)", out, err)
	}
	if _, out, err := execGit(ctx, repo, "config", "user.name", "collab-test"); err != nil {
		t.Fatalf("git config 失败: %s (%v)", out, err)
	}
	if err := os.WriteFile(filepath.Join(repo, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatalf("写初始文件失败: %v", err)
	}
	if _, out, err := execGit(ctx, repo, "add", "."); err != nil {
		t.Fatalf("git add 失败: %s (%v)", out, err)
	}
	if _, out, err := execGit(ctx, repo, "commit", "-m", "init"); err != nil {
		t.Fatalf("git commit 失败: %s (%v)", out, err)
	}

	mgr := NewFileCollabWorkspaceManager(dataDir) // 不注入 fake:走真实 execGit
	leaseA, err := mgr.Prepare(ctx, sid, thA, repo, WorkspaceIsolatedWorktree)
	if err != nil {
		t.Fatalf("真实 git 隔离授予(A)失败: %v", err)
	}
	leaseB, err := mgr.Prepare(ctx, sid, thB, repo, WorkspaceIsolatedWorktree)
	if err != nil {
		t.Fatalf("真实 git 隔离授予(B)失败: %v", err)
	}
	if strings.EqualFold(filepath.Clean(leaseA.WorkPath), filepath.Clean(leaseB.WorkPath)) || leaseA.Branch == leaseB.Branch {
		t.Fatalf("两成员 worktree/分支必须彼此独立: %s %s / %s %s", leaseA.WorkPath, leaseB.WorkPath, leaseA.Branch, leaseB.Branch)
	}
	// 各自产出后留证清理:成员分支保留(成果在分支上),证据只含自己的改动。
	// 与既有先例同款:改已跟踪文件(进 diff)+ 加未跟踪文件(只进现场清单)。
	mutate := func(workPath, marker, untracked string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(workPath, "base.txt"), []byte(marker+"\n"), 0o644); err != nil {
			t.Fatalf("成员改已跟踪文件失败: %v", err)
		}
		if err := os.WriteFile(filepath.Join(workPath, untracked), []byte(marker+"\n"), 0o644); err != nil {
			t.Fatalf("成员加未跟踪文件失败: %v", err)
		}
	}
	mutate(leaseA.WorkPath, "A 的改动", "analysis-a.md")
	mutate(leaseB.WorkPath, "B 的改动", "analysis-b.md")
	if err := mgr.Cleanup(ctx, leaseA, true); err != nil {
		t.Fatalf("成员 A 留证清理失败: %v", err)
	}
	if err := mgr.Cleanup(ctx, leaseB, true); err != nil {
		t.Fatalf("成员 B 留证清理失败: %v", err)
	}
	for _, tc := range []struct {
		memberID, branch, marker, untracked, other string
	}{
		{thA, leaseA.Branch, "A 的改动", "?? analysis-a.md", "B 的改动"},
		{thB, leaseB.Branch, "B 的改动", "?? analysis-b.md", "A 的改动"},
	} {
		if _, out, err := execGit(ctx, repo, "rev-parse", "--verify", tc.branch); err != nil {
			t.Fatalf("成员分支 %s 应保留(成果在分支上): %s (%v)", tc.branch, out, err)
		}
		dir := filepath.Join(dataDir, collabDataDirName, collabArtifactsDirName, sid, tc.memberID)
		uncommitted, err := os.ReadFile(filepath.Join(dir, "diff-uncommitted.patch"))
		if err != nil || !strings.Contains(string(uncommitted), tc.marker) {
			t.Fatalf("成员 %s 的真实 diff 证据应含自己的改动: %v %q", tc.memberID, err, uncommitted)
		}
		if strings.Contains(string(uncommitted), tc.other) {
			t.Fatalf("成员 %s 的 diff 不得串入他人改动: %q", tc.memberID, uncommitted)
		}
		status, err := os.ReadFile(filepath.Join(dir, "status.txt"))
		if err != nil || !strings.Contains(string(status), tc.untracked) {
			t.Fatalf("成员 %s 的现场清单应含未跟踪文件名单: %v %q", tc.memberID, err, status)
		}
	}
}

// ============ 场景 9:共享工作区写任务被安全拒绝 ============

// 无隔离能力(目录不是 git 仓库)时请求 isolated_worktree:**整体拒绝**,
// 不静默退回共享目录(多成员共写正是要消灭的事故);错误说清原因与替代路径
// (read_only);不产生任何租约与 worktree 物理痕迹;协同会话本身如实记录
// 请求的工作区,不被这次拒绝改变真相。
func TestCollabE2ESharedWorkspaceWriteRejectedWithoutIsolation(t *testing.T) {
	f := collabE2EFactory()
	fa := f.Provider("fa", testagent.Scenario{Events: []testagent.AgentEvent{testagent.Completed("分析完成(只读共享)")}}).(*testagent.FakeAgent)
	s, dataDir := newCollabE2EServer(t, fa)

	notRepo := t.TempDir() // 没有 .git 的普通目录 = 无隔离能力
	resp := dispatchCollabE2E(t, s, "在共享目录里写文件", notRepo, "fa")
	sid, th := resp.GetSessionId(), resp.GetItems()[0].GetThreadId()
	card := waitCollabMemberStatus(t, s, sid, th, string(store.CollabStatusCompleted))
	if card.GetWorkspace() != notRepo {
		t.Fatalf("成员卡应如实记录请求的工作区,实际 %q", card.GetWorkspace())
	}

	// 写任务请求隔离而不得 → 整体拒绝(按 collab_workspace 的拒绝路径)。
	mgr := NewFileCollabWorkspaceManager(dataDir)
	mgr.runGit = newFakeGit("").run // toplevel 空 = 模拟"目录不是 git 仓库"
	lease, err := mgr.Prepare(context.Background(), sid, th, notRepo, WorkspaceIsolatedWorktree)
	if err == nil {
		t.Fatal("非 git 目录请求隔离模式必须被拒绝")
	}
	if !errors.Is(err, ErrWorkspaceNotGitRepo) {
		t.Fatalf("应返回 ErrWorkspaceNotGitRepo,实际: %v", err)
	}
	if !strings.Contains(err.Error(), "不是 git 仓库") || !strings.Contains(err.Error(), "静默退回") || !strings.Contains(err.Error(), "read_only") {
		t.Fatalf("拒绝错误应是人话并给替代路径,实际: %v", err)
	}
	if lease != (WorkspaceLease{}) {
		t.Fatalf("被拒时不得返回租约: %+v", lease)
	}
	if got := len(mgr.List()); got != 0 {
		t.Fatalf("被拒时不得登记任何租约,实际 %d", got)
	}
	if _, err := os.Stat(filepath.Join(notRepo, collabWorktreesDirName)); !os.IsNotExist(err) {
		t.Fatalf("被拒时不得在用户目录留下任何物理痕迹: %v", err)
	}
	// 协同会话真相不被拒绝改变:成员卡仍是完成态 + 原工作区记录。
	snap, err := s.collabCoordinatorFor().Snapshot(context.Background(), sid)
	if err != nil {
		t.Fatal(err)
	}
	m := collabMemberByThread(snap, th)
	if m == nil || m.GetStatus() != string(store.CollabStatusCompleted) || m.GetWorkspace() != notRepo {
		t.Fatalf("会话成员卡不应被工作区拒绝改变: %+v", m)
	}
}
