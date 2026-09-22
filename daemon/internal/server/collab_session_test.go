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
	return New(Config{
		Store:     fs,
		Providers: map[string]agent.Provider{"echo": agent.NewEcho()},
		Workspace: t.TempDir(),
	})
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
