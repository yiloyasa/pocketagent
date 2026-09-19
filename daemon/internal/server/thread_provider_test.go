// thread_provider_test.go —— providerForThread 显式失败语义(多 agent 接入)。
//
// 【为什么必须钉死】旧 providerFor 找不到 provider 会静默回退默认 —— 单 agent
// 时代无害,多 agent 之后就是"我明明选了 Pi,结果发给 Codex 了"(调研 §7.1)。
// 现在找不到 = 硬错误 + 人话,绝不换人干活。
package server

import (
	"context"
	"strings"
	"testing"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

func TestProviderForThreadExplicitFailure(t *testing.T) {
	srv, _ := newSteerTestServer(t, "echo", &seqProvider{id: "echo"})
	// 线程归属 pi,但注册表里没有 pi:必须报错,不能回退 echo/codex。
	th, ok := srv.cfg.Store.Get("th-q")
	if !ok {
		t.Fatal("测试线程不存在")
	}
	th.AgentID = "pi"
	p, err := srv.providerForThread(th)
	if err == nil || p != nil {
		t.Fatalf("pi 不可用时必须显式失败,得到 provider=%v err=%v", p, err)
	}
	if !strings.Contains(err.Error(), "Pi") {
		t.Fatalf("错误文案要能让人看懂缺的是谁: %v", err)
	}
}

func TestProviderForThreadAgentIDWinsOverModelPrefix(t *testing.T) {
	echo := &seqProvider{id: "echo"}
	srv := New(Config{Store: mustStore(t), Providers: map[string]agent.Provider{
		"echo": echo,
		"pi":   &PiProviderStub{}, // 注册表里真有 pi
	}})
	// AgentID 是会话归属的硬事实:即使 ModelID 前缀是 codex,也以 AgentID 为准。
	th := &store.Thread{ID: "th-1", ModelID: "codex/gpt-x", AgentID: "pi"}
	p, err := srv.providerForThread(th)
	if err != nil {
		t.Fatalf("pi 可用时不应报错: %v", err)
	}
	if p.ID() != "pi" {
		t.Fatalf("应选 pi provider,得到 %s", p.ID())
	}
}

func TestProviderForThreadLegacyRecordFallsBackByModelPrefix(t *testing.T) {
	echo := &seqProvider{id: "codex"} // 模拟"codex"是默认注册
	srv := New(Config{Store: mustStore(t), Providers: map[string]agent.Provider{"codex": echo}})
	// 旧库记录:AgentID 缺省 → 按 ModelID 前缀回推(codex)。
	th := &store.Thread{ID: "th-2", ModelID: "codex/gpt-x"}
	p, err := srv.providerForThread(th)
	if err != nil || p.ID() != "codex" {
		t.Fatalf("旧记录应按 ModelID 前缀选择: p=%v err=%v", p, err)
	}
	// 连前缀都没有的更老记录(ModelID 被置空 = "跟随桌面"):默认 provider。
	th2 := &store.Thread{ID: "th-3", ModelID: ""}
	p2, err2 := srv.providerForThread(th2)
	if err2 != nil || p2.ID() != "codex" {
		t.Fatalf("无前缀旧记录应退默认 provider: p=%v err=%v", p2, err2)
	}
}

func TestThreadAgentIDDefaultsToCodex(t *testing.T) {
	if got := threadAgentID(&store.Thread{}); got != "codex" {
		t.Fatalf("空 AgentID(旧库)应视为 codex,得到 %q", got)
	}
	if got := threadAgentID(&store.Thread{AgentID: "pi"}); got != "pi" {
		t.Fatalf("pi 归属应保留,得到 %q", got)
	}
}

// CreateThread 带 agent_id:pi 会话落 AgentID,不存在的 agent 显式报错。
func TestCreateThreadWithAgentID(t *testing.T) {
	echo := &seqProvider{id: "pi"}
	srv := New(Config{Store: mustStore(t), Providers: map[string]agent.Provider{"pi": echo}})

	resp, err := srv.CreateThread(ctxBG(), &pb.CreateThreadRequest{Title: "pi 会话", AgentId: "pi"})
	if err != nil {
		t.Fatalf("创建 pi 会话失败: %v", err)
	}
	if resp.AgentId != "pi" {
		t.Fatalf("Thread.agent_id 应为 pi,得到 %q", resp.AgentId)
	}
	th, ok := srv.cfg.Store.Get(resp.Id)
	if !ok || th.AgentID != "pi" {
		t.Fatalf("落库的 AgentID 应为 pi: %+v", th)
	}

	// 不存在的 agent:显式报错(不静默落到别的 agent 上)
	if _, err := srv.CreateThread(ctxBG(), &pb.CreateThreadRequest{AgentId: "nosuch"}); err == nil {
		t.Fatal("不存在的 agent 应显式报错")
	}
}

// PiProviderStub 只是 provider 接口的最小实现(避免测试真拉 pi 子进程)。
type PiProviderStub struct{}

func (PiProviderStub) ID() string             { return "pi" }
func (PiProviderStub) DisplayName() string    { return "Pi(测试)" }
func (PiProviderStub) Models() []pb.ModelInfo { return nil }

func (PiProviderStub) Run(_ context.Context, _ agent.TurnRequest, _ agent.Sink, _ agent.ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
	return nil, nil, nil
}

func mustStore(t *testing.T) *store.FileStore {
	t.Helper()
	fs, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("建测试存储失败: %v", err)
	}
	return fs
}

func ctxBG() (c context.Context) { return context.Background() }
