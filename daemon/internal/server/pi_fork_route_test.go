// pi_fork_route_test.go —— ForkThread RPC 按 AgentID 路由到 pi 的回归(pi fork
// 接通,pi_fork_plan.md §5;仿 pi_compact_route_test.go 的分家测试)。
//
// 【为什么必须钉死】pi 的 clone 是"当前叶分出新分支会话文件"(与 server 语义直接
// 对齐),codex 的 thread/fork 是 string 形另一条通道;路由错 = pi 会话点分叉报
// "没有可用的 codex"或误走 codex 形状。fake 注入遵循 piCompactStub 同款最小接口
// 模式(piShapedForker / titlePusher),不真拉 pi 子进程。
package server

import (
	"context"
	"strings"
	"testing"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// piForkStub 在 PiProviderStub(纯 provider 接口)之上补 ForkThread(piShapedForker
// 形状)与 Rename(titlePusher),记录调用并按注入值返回 —— 不真拉 pi 子进程。
type piForkStub struct {
	PiProviderStub
	calls   int
	gotFile string
	ref     store.ExternalSessionRef
	err     error
	renamed map[string]string
}

func (s *piForkStub) ForkThread(t *store.Thread) (store.ExternalSessionRef, error) {
	s.calls++
	s.gotFile = t.ExternalSession.SessionFile
	if s.err != nil {
		return store.ExternalSessionRef{}, s.err
	}
	return s.ref, nil
}

func (s *piForkStub) Rename(t *store.Thread, title string) error {
	if s.renamed == nil {
		s.renamed = map[string]string{}
	}
	s.renamed[t.ID] = title
	return nil
}

func forkSrcThread(id string) *store.Thread {
	return &store.Thread{
		ID: id, AgentID: "pi", Title: "源会话", Workspace: `C:\ws`,
		ModelID: "pi/zai-coding-cn/glm-5.3", Effort: "high",
		ExternalSession: store.ExternalSessionRef{
			SessionID: "s1", SessionFile: `C:\sessions\` + id + ".jsonl", Cwd: `C:\ws`,
		},
		Messages: []store.StoredMessage{{Role: "user"}, {Role: "assistant"}},
	}
}

// TestForkThreadRoutesPiThreadToPiForker pi 线程走 piShapedForker:新线程 = pi 身份
// (AgentID+ExternalSession,不填 CodexThreadID)+ 源历史播种 + 「· 分支」标题,
// 标题写回 pi(Rename)同样到达。
func TestForkThreadRoutesPiThreadToPiForker(t *testing.T) {
	st := mustStore(t)
	th := forkSrcThread("th-pi-fork-1")
	if err := st.Create(th); err != nil {
		t.Fatalf("建线程失败: %v", err)
	}
	ref := store.ExternalSessionRef{SessionID: "s2", SessionFile: `C:\sessions\new.jsonl`, Cwd: `C:\ws`}
	stub := &piForkStub{ref: ref}
	srv := New(Config{Store: st, Providers: map[string]agent.Provider{"pi": stub}})
	resp, err := srv.ForkThread(context.Background(), &pb.ForkThreadRequest{ThreadId: th.ID, LastTurnId: "lt-1"})
	if err != nil {
		t.Fatalf("pi 路径应成功: %v", err)
	}
	if stub.calls != 1 {
		t.Fatalf("ForkThread 应恰好被调一次,实际 %d", stub.calls)
	}
	if stub.gotFile != th.ExternalSession.SessionFile {
		t.Fatalf("应把源线程交给 provider,实际拿到 %q", stub.gotFile)
	}
	created, ok := st.Get(resp.GetId())
	if !ok {
		t.Fatal("新线程应落库")
	}
	if created.AgentID != "pi" {
		t.Fatalf("新线程应归属 pi,实际 %q", created.AgentID)
	}
	if created.CodexThreadID != "" {
		t.Fatalf("pi 线程不得填 CodexThreadID,实际 %q", created.CodexThreadID)
	}
	if created.ExternalSession != ref {
		t.Fatalf("新线程应挂 provider 回传的引用: %+v", created.ExternalSession)
	}
	if created.Title != "源会话 · 分支" {
		t.Fatalf("新线程标题应带「· 分支」标记: %q", created.Title)
	}
	if created.Workspace != th.Workspace || created.ModelID != th.ModelID || created.Effort != th.Effort {
		t.Fatalf("工作区/模型/强度应随源复制: %+v", created)
	}
	if len(created.Messages) != 2 {
		t.Fatalf("新线程应用源历史播种(2 条),实际 %d 条", len(created.Messages))
	}
	if got := stub.renamed[created.ID]; got != created.Title {
		t.Fatalf("分叉标题应写回 pi(Rename),实际 %q", got)
	}
}

// TestForkThreadPropagatesPiFailure pi 侧失败(如 PI_BUSY)如实转 gRPC Internal。
func TestForkThreadPropagatesPiFailure(t *testing.T) {
	st := mustStore(t)
	th := forkSrcThread("th-pi-fork-2")
	if err := st.Create(th); err != nil {
		t.Fatalf("建线程失败: %v", err)
	}
	stub := &piForkStub{err: status.Error(codes.Internal, "这个会话正在执行任务")}
	srv := New(Config{Store: st, Providers: map[string]agent.Provider{"pi": stub}})
	_, err := srv.ForkThread(context.Background(), &pb.ForkThreadRequest{ThreadId: th.ID})
	if err == nil || !strings.Contains(err.Error(), "正在执行任务") {
		t.Fatalf("应透传 pi 的失败原因: %v", err)
	}
}

// TestForkThreadPiWithoutForker 注册表里的 pi 不具备分叉能力时显式报"没有可用的
// pi",绝不静默回退 codex 路径。
func TestForkThreadPiWithoutForker(t *testing.T) {
	st := mustStore(t)
	th := forkSrcThread("th-pi-fork-3")
	if err := st.Create(th); err != nil {
		t.Fatalf("建线程失败: %v", err)
	}
	srv := New(Config{Store: st, Providers: map[string]agent.Provider{"pi": &PiProviderStub{}}})
	_, err := srv.ForkThread(context.Background(), &pb.ForkThreadRequest{ThreadId: th.ID})
	if status.Code(err) != codes.Unavailable || !strings.Contains(err.Error(), "pi") {
		t.Fatalf("应报 pi 不可用: %v", err)
	}
}

// TestForkThreadPiNotMounted pi 会话未落地时显式 FailedPrecondition(路由层双保险
// 的前一道,与 codex 路径同文案口径)。
func TestForkThreadPiNotMounted(t *testing.T) {
	st := mustStore(t)
	th := forkSrcThread("th-pi-fork-4")
	th.ExternalSession.SessionFile = ""
	if err := st.Create(th); err != nil {
		t.Fatalf("建线程失败: %v", err)
	}
	stub := &piForkStub{}
	srv := New(Config{Store: st, Providers: map[string]agent.Provider{"pi": stub}})
	_, err := srv.ForkThread(context.Background(), &pb.ForkThreadRequest{ThreadId: th.ID})
	if status.Code(err) != codes.FailedPrecondition || !strings.Contains(err.Error(), "先发一条消息") {
		t.Fatalf("应 FailedPrecondition 提示先发消息: %v", err)
	}
	if stub.calls != 0 {
		t.Fatalf("未落地闸门应在 provider 之前拦下,实际已调 provider %d 次", stub.calls)
	}
}

// TestForkThreadNonPiThreadsStayOnCodexPath codex/dsh/zcode 线程不走 pi 分支:
// 落回 codex 路径的既有守卫(无 CodexThreadID → "先发一条消息"),pi fake 零调用。
func TestForkThreadNonPiThreadsStayOnCodexPath(t *testing.T) {
	for _, agentID := range []string{"codex", "", "dsh", "zcode"} {
		st := mustStore(t)
		th := &store.Thread{ID: "th-fork-nopi", AgentID: agentID, CodexThreadID: ""}
		if err := st.Create(th); err != nil {
			t.Fatalf("建线程失败: %v", err)
		}
		stub := &piForkStub{}
		srv := New(Config{Store: st, Providers: map[string]agent.Provider{
			"pi":    stub,
			"codex": agent.NewCodexProvider(), // codex 路径守卫要求 provider 在场
		}})
		_, err := srv.ForkThread(context.Background(), &pb.ForkThreadRequest{ThreadId: th.ID})
		if status.Code(err) != codes.FailedPrecondition {
			t.Fatalf("agentID=%q 应落 codex 路径的 FailedPrecondition: %v", agentID, err)
		}
		if stub.calls != 0 {
			t.Fatalf("agentID=%q 不得走 pi 分支,实际调了 %d 次", agentID, stub.calls)
		}
	}
}
