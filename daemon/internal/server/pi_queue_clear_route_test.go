// pi_queue_clear_route_test.go —— ClearQueuedInputs RPC 路由到 pi QueueClearer 的回归。
//
// 【钉什么】pi 0.87.1 的 clear_queue 早已在 provider 侧就绪(agent.QueueClearer),
// 缺的是 server 入口:本文件钉"手机调用 → 路由正确 → 条数如实回传 → 失败不假
// 成功"的接线半边。fake 注入遵循 pi_compact_route_test.go 同款最小接口模式。
package server

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// piQueueClearStub 在纯 provider 接口之上补 ClearQueue,记录调用并按注入值返回。
type piQueueClearStub struct {
	PiProviderStub
	calls    []string
	steered  int
	followUp int
	err      error
}

func (s *piQueueClearStub) ClearQueue(_ context.Context, threadID string) (int, int, error) {
	s.calls = append(s.calls, threadID)
	if s.err != nil {
		return 0, 0, s.err
	}
	return s.steered, s.followUp, nil
}

func TestClearQueuedInputsRoutesToPi(t *testing.T) {
	st := mustStore(t)
	th := piThreadWithSession("th-piq-1")
	if err := st.Create(th); err != nil {
		t.Fatalf("建线程失败: %v", err)
	}
	stub := &piQueueClearStub{steered: 1, followUp: 2}
	srv := New(Config{Store: st, Providers: map[string]agent.Provider{"pi": stub}})

	resp, err := srv.ClearQueuedInputs(context.Background(), &pb.ClearQueuedInputsRequest{ThreadId: th.ID})
	if err != nil {
		t.Fatalf("pi 路径应成功: %v", err)
	}
	if len(stub.calls) != 1 || stub.calls[0] != th.ID {
		t.Fatalf("ClearQueue 应恰好以本会话 id 调一次,实际 %v", stub.calls)
	}
	if resp.GetSteered() != 1 || resp.GetFollowUp() != 2 {
		t.Fatalf("撤回条数应如实回传(插队1/排队2),得到 %+v", resp)
	}
}

// pi 侧失败(PI_QUEUE_CLEAR_FAILED 等结构化错误)必须如实变成 gRPC 错误。
func TestClearQueuedInputsPropagatesPiFailure(t *testing.T) {
	st := mustStore(t)
	th := piThreadWithSession("th-piq-2")
	if err := st.Create(th); err != nil {
		t.Fatalf("建线程失败: %v", err)
	}
	stub := &piQueueClearStub{err: errors.New("撤销排队输入失败: pipe closed")}
	srv := New(Config{Store: st, Providers: map[string]agent.Provider{"pi": stub}})
	_, err := srv.ClearQueuedInputs(context.Background(), &pb.ClearQueuedInputsRequest{ThreadId: th.ID})
	if err == nil || !strings.Contains(err.Error(), "pipe closed") {
		t.Fatalf("应透传 pi 的失败原因: %v", err)
	}
}

// 非 pi 会话(codex 没有原生排队面)显式 FailedPrecondition,不静默换人。
func TestClearQueuedInputsRejectsNonPiThread(t *testing.T) {
	st := mustStore(t)
	th := &store.Thread{ID: "th-codex-q", AgentID: "codex"}
	if err := st.Create(th); err != nil {
		t.Fatalf("建线程失败: %v", err)
	}
	srv := New(Config{Store: st, Providers: map[string]agent.Provider{
		"pi": &piQueueClearStub{},
	}})
	_, err := srv.ClearQueuedInputs(context.Background(), &pb.ClearQueuedInputsRequest{ThreadId: th.ID})
	if err == nil || !strings.Contains(err.Error(), "pi") {
		t.Fatalf("非 pi 会话应显式报错: %v", err)
	}
}

// 注册表里的 pi 不具备撤回能力(stub/旧实现)时显式报"没有可用的 pi"。
func TestClearQueuedInputsWithoutClearer(t *testing.T) {
	st := mustStore(t)
	th := piThreadWithSession("th-piq-3")
	if err := st.Create(th); err != nil {
		t.Fatalf("建线程失败: %v", err)
	}
	srv := New(Config{Store: st, Providers: map[string]agent.Provider{"pi": &PiProviderStub{}}})
	_, err := srv.ClearQueuedInputs(context.Background(), &pb.ClearQueuedInputsRequest{ThreadId: th.ID})
	if err == nil || !strings.Contains(err.Error(), "pi") {
		t.Fatalf("应报 pi 不可用: %v", err)
	}
}

// 会话不存在:NotFound,与 CancelTurn 同口径。
func TestClearQueuedInputsUnknownThread(t *testing.T) {
	srv := New(Config{Store: mustStore(t), Providers: map[string]agent.Provider{
		"pi": &piQueueClearStub{},
	}})
	if _, err := srv.ClearQueuedInputs(context.Background(), &pb.ClearQueuedInputsRequest{ThreadId: "th-没有"}); err == nil {
		t.Fatal("未知会话应报 NotFound")
	}
}
