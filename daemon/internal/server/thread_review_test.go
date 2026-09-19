// thread_review_test.go —— StartReview("/计划")路由的回归测试。
//
// 【要钉死的东西】
//  1. 非 codex 会话显式 FailedPrecondition("计划模式目前仅 Codex 会话支持"),
//     绝不静默换 provider —— 与 CompactThread 的路由纪律同款;
//  2. 会话不存在 → NotFound;注册表没有 codex → Unavailable;
//  3. codex 会话但线程未在电脑上加载(daemon 刚重启/新会话没发过消息)→
//     FailedPrecondition"先发一条消息"(provider 不拉子进程即可判定);
//  4. 旧库空 AgentID 视为 codex(threadAgentID 的兼容语义)。
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

func codexThreadForReview(id string) *store.Thread {
	return &store.Thread{ID: id, AgentID: "codex"}
}

// 非 codex 会话:显式拒绝,人话文案,gRPC 码必须是 FailedPrecondition。
func TestStartReviewRejectsNonCodex(t *testing.T) {
	for _, agentID := range []string{"pi", "zcode"} {
		st := mustStore(t)
		th := &store.Thread{ID: "th-" + agentID, AgentID: agentID}
		if err := st.Create(th); err != nil {
			t.Fatalf("建线程失败: %v", err)
		}
		srv := New(Config{Store: st, Providers: map[string]agent.Provider{}})
		_, err := srv.StartReview(context.Background(), &pb.StartReviewRequest{ThreadId: th.ID})
		if err == nil || status.Code(err) != codes.FailedPrecondition {
			t.Fatalf("%s 会话应 FailedPrecondition, got %v", agentID, err)
		}
		if !strings.Contains(err.Error(), "计划模式目前仅 Codex 会话支持") {
			t.Fatalf("拒绝文案要人话: %v", err)
		}
	}
}

// 会话不存在 → NotFound。
func TestStartReviewUnknownThread(t *testing.T) {
	srv := New(Config{Store: mustStore(t), Providers: map[string]agent.Provider{}})
	_, err := srv.StartReview(context.Background(), &pb.StartReviewRequest{ThreadId: "th-不存在"})
	if err == nil || status.Code(err) != codes.NotFound {
		t.Fatalf("未知会话应 NotFound, got %v", err)
	}
}

// 注册表里没有可用的 codex(或不是 *agent.CodexProvider)→ Unavailable。
func TestStartReviewWithoutCodexProvider(t *testing.T) {
	st := mustStore(t)
	th := codexThreadForReview("th-cx-1")
	if err := st.Create(th); err != nil {
		t.Fatalf("建线程失败: %v", err)
	}
	// codex 槽位被别的 provider 占着:类型断言失败同样按"没有可用的 codex"处理
	srv := New(Config{Store: st, Providers: map[string]agent.Provider{
		"codex": &PiProviderStub{},
	}})
	_, err := srv.StartReview(context.Background(), &pb.StartReviewRequest{ThreadId: th.ID})
	if err == nil || status.Code(err) != codes.Unavailable {
		t.Fatalf("没有 codex 应 Unavailable, got %v", err)
	}
	if !strings.Contains(err.Error(), "没有可用的 codex") {
		t.Fatalf("文案要人话: %v", err)
	}
}

// codex 会话但线程未在电脑上加载:FailedPrecondition"先发一条消息"。
// 真 *CodexProvider 的 threadMap 为空即未加载,该判定不拉子进程,可离线回归。
func TestStartReviewCodexThreadNotLoaded(t *testing.T) {
	st := mustStore(t)
	th := codexThreadForReview("th-cx-2")
	if err := st.Create(th); err != nil {
		t.Fatalf("建线程失败: %v", err)
	}
	srv := New(Config{Store: st, Providers: map[string]agent.Provider{
		"codex": agent.NewCodexProvider(),
	}})
	_, err := srv.StartReview(context.Background(), &pb.StartReviewRequest{ThreadId: th.ID})
	if err == nil || status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("未加载线程应 FailedPrecondition, got %v", err)
	}
	if !strings.Contains(err.Error(), "先发一条消息") {
		t.Fatalf("文案要人话: %v", err)
	}
}

// 旧库空 AgentID 兼容:视为 codex 走到 codex 闸门(而非被 agent 检查拒掉)。
func TestStartReviewLegacyThreadCountsAsCodex(t *testing.T) {
	st := mustStore(t)
	th := &store.Thread{ID: "th-legacy"} // AgentID 空
	if err := st.Create(th); err != nil {
		t.Fatalf("建线程失败: %v", err)
	}
	srv := New(Config{Store: st, Providers: map[string]agent.Provider{
		"codex": agent.NewCodexProvider(),
	}})
	// 错误应是"未加载"(已通过 agent 闸门),而不是"仅 Codex 会话支持"
	_, err := srv.StartReview(context.Background(), &pb.StartReviewRequest{ThreadId: th.ID})
	if err == nil || !strings.Contains(err.Error(), "先发一条消息") {
		t.Fatalf("旧库空 AgentID 应按 codex 处理: %v", err)
	}
}
