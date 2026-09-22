// runtime_state_test.go —— 运行时形态核验的消费面(规格 §10)的回归用例。
//
// 验收点:
//
//	① 启动期真的跑了声明层核验,并经 GetRuntimeState 透出(不是"提供了函数没人调用");
//	② 未实现 Participant 的 provider → runtime_state=degraded + 带 not-declared 违约;
//	③ GetAgentRuntime:存在的 agent 回形态,不存在的回 NotFound,缺参回 InvalidArgument;
//	④ 该 agent 作用域的资源租约随形态一起透出(客户端一张面板看全)。
package server

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/store"
)

func newRuntimeStateTestServer(t *testing.T, providers map[string]agent.Provider) *Server {
	t.Helper()
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("建 store 失败: %v", err)
	}
	srv := New(Config{Store: st, Providers: providers, RuntimeBackend: runtime.NewBackend()})
	t.Cleanup(func() { _ = srv.events.Close() })
	return srv
}

// TestGetRuntimeStateReportsDeclarations 覆盖 ①②。
func TestGetRuntimeStateReportsDeclarations(t *testing.T) {
	// echo 实现了 runtime.Participant(见 agent/echo_runtime.go),是"达标"的样本。
	// 这里用 nil 当"连形态都没申报"的样本 —— 类型断言对 nil interface 返回 false,
	// 正是该违约码要抓的最差情况。
	srv := newRuntimeStateTestServer(t, map[string]agent.Provider{
		"echo":  agent.NewEcho(),
		"ghost": nil,
	})

	resp, err := srv.GetRuntimeState(context.Background(), &pb.GetRuntimeStateRequest{})
	if err != nil {
		t.Fatalf("GetRuntimeState 失败: %v", err)
	}
	if len(resp.GetContracts()) != 2 {
		t.Fatalf("应回两份形态报告,得到 %d", len(resp.GetContracts()))
	}
	if resp.GetRuntimeState() != "degraded" {
		t.Fatalf("有 agent 没申报形态时应判 degraded,得到 %q", resp.GetRuntimeState())
	}
	if resp.GetDaemonVersion() == "" || resp.GetSampledAtUnixMs() == 0 {
		t.Fatalf("总览应带版本与采样时刻: %+v", resp)
	}

	byID := map[string]*pb.RuntimeContractReport{}
	for _, c := range resp.GetContracts() {
		byID[c.GetAgentId()] = c
	}
	echo := byID["echo"]
	if echo == nil || !echo.GetDeclared() {
		t.Fatalf("echo 应申报了运行时形态: %+v", echo)
	}
	if echo.GetSessionModel() == "" || echo.GetMemoryModel() == "" {
		t.Fatalf("形态的六个 model 字段应透出(客户端据此决定显示什么入口): %+v", echo)
	}
	if len(echo.GetIssues()) != 0 {
		t.Fatalf("echo 不该有契约违约: %+v", echo.GetIssues())
	}
	ghost := byID["ghost"]
	if ghost == nil || ghost.GetDeclared() {
		t.Fatalf("ghost 不该被算作已申报: %+v", ghost)
	}
	if len(ghost.GetIssues()) == 0 || ghost.GetIssues()[0].GetCode() != runtime.ViolationNotDeclared {
		t.Fatalf("应报 not-declared 违约,得到 %+v", ghost.GetIssues())
	}
	if ghost.GetIssues()[0].GetDetail() == "" {
		t.Fatal("违约必须带可读说明(否则用户只看到一个码)")
	}
}

// TestGetRuntimeStateReadyWhenAllDeclared 全部达标时判 ready。
func TestGetRuntimeStateReadyWhenAllDeclared(t *testing.T) {
	srv := newRuntimeStateTestServer(t, map[string]agent.Provider{"echo": agent.NewEcho()})
	resp, err := srv.GetRuntimeState(context.Background(), &pb.GetRuntimeStateRequest{})
	if err != nil {
		t.Fatalf("GetRuntimeState 失败: %v", err)
	}
	if resp.GetRuntimeState() != "ready" {
		t.Fatalf("全部达标应判 ready,得到 %q(违约 %+v)", resp.GetRuntimeState(), resp.GetContracts()[0].GetIssues())
	}
}

// TestGetAgentRuntime 覆盖 ③④。
func TestGetAgentRuntime(t *testing.T) {
	srv := newRuntimeStateTestServer(t, map[string]agent.Provider{"echo": agent.NewEcho()})
	ctx := context.Background()

	resp, err := srv.GetAgentRuntime(ctx, &pb.GetAgentRuntimeRequest{AgentId: "echo"})
	if err != nil {
		t.Fatalf("GetAgentRuntime 失败: %v", err)
	}
	if resp.GetContract().GetAgentId() != "echo" || !resp.GetContract().GetDeclared() {
		t.Fatalf("应回 echo 的形态: %+v", resp.GetContract())
	}

	// 该 agent 作用域下的租约要一起透出(一张面板看全,不必再发一次查询)。
	if _, err := srv.runtimeBackend.Resources().Acquire(ctx, runtime.LeaseRequest{
		LeaseID: "probe-lease", OwnerScope: runtime.ScopeAgent("echo"),
		Kind: runtime.ResEventBuffer, SoftLimit: 100, HardLimit: 200,
	}); err != nil {
		t.Fatalf("造租约失败: %v", err)
	}
	withLease, err := srv.GetAgentRuntime(ctx, &pb.GetAgentRuntimeRequest{AgentId: "echo"})
	if err != nil {
		t.Fatalf("GetAgentRuntime 失败: %v", err)
	}
	if len(withLease.GetResources()) != 1 || withLease.GetResources()[0].GetLeaseId() != "probe-lease" {
		t.Fatalf("应回该 agent 的租约明细: %+v", withLease.GetResources())
	}
	if withLease.GetResources()[0].GetSoftLimitBytes() != 100 {
		t.Fatalf("额度应一并带回: %d", withLease.GetResources()[0].GetSoftLimitBytes())
	}

	// 不存在的 agent:必须说"没有它",不回空形态(空形态与"什么都没申报"看起来一样)。
	if _, err := srv.GetAgentRuntime(ctx, &pb.GetAgentRuntimeRequest{AgentId: "nope"}); status.Code(err) != codes.NotFound {
		t.Fatalf("不存在的 agent 应为 NotFound,得到 %v", err)
	}
	if _, err := srv.GetAgentRuntime(ctx, &pb.GetAgentRuntimeRequest{}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("缺 agent_id 应为 InvalidArgument,得到 %v", err)
	}
}
