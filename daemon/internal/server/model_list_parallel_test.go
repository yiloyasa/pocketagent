package server

// model_list_parallel_test.go —— PA-REAL-0002 / PA-REAL-0003 的 ListModels 回归。
//
// 【缺陷回顾】2026-09-19 真机:codex 冷启动初始化期间,一次 ListModels 顺序跑完
// codex(慢)→ pi(一次性子进程,预算 30s)→ zcode(预算 20s),整段越过手机端
// 30s RPC 截止,用户拿到裸 DeadlineExceeded;同一窗口 pi/zcode 的模型读取全部
// 失败 —— 一个 agent 冷启动,拖死所有 agent 的模型列表。
//
// 【钉死的行为】
//  1. 各 provider 的模型拉取并行执行:一个 provider 慢(伪造 codex 初始化持锁/
//     慢探测),其余 provider 的条目照常及时返回 —— 跨 agent 不再互相阻塞;
//  2. 整段等待有界(≤10s 且不超 RPC 剩余时间):超预算的 provider 不进列表,
//     StateError 给出"初始化中,可重试"的结构化人话 —— 不再是裸超时,
//     也绝不把缺席伪造成"一切正常"(AUDIT §3.3);
//  3. 全部提前完成时立即返回,不干等满预算;输出顺序与旧顺序实现一致;
//  4. 带错误回执的 provider(状态预算通道)失败原因从 StateError 透传,
//     其真实条目(如 default)照常进列表。

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// slowModelsProvider 可控延时的 provider 桩:模拟"该 agent 正在冷启动初始化/
// 探测"(生命周期锁被占、握手慢,对外表现一样:Models() 迟迟不回)。
type slowModelsProvider struct {
	id     string
	delay  time.Duration
	models []pb.ModelInfo
}

func (p *slowModelsProvider) ID() string          { return p.id }
func (p *slowModelsProvider) DisplayName() string { return "stub-" + p.id }
func (p *slowModelsProvider) Models() []pb.ModelInfo {
	time.Sleep(p.delay)
	return p.models
}
func (p *slowModelsProvider) Run(_ context.Context, _ agent.TurnRequest, _ agent.Sink, _ agent.ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
	return nil, nil, nil
}

// statusSourceStub 实现 ModelsStatusCtx(生产对应 *agent.CodexProvider 的状态
// 预算通道):带回执的模型拉取。
type statusSourceStub struct {
	plainProvider
	models []pb.ModelInfo
	err    error
}

func (p *statusSourceStub) ModelsStatusCtx(context.Context) ([]pb.ModelInfo, error) {
	return p.models, p.err
}

// ① 锁分片/跨 agent 不阻塞:codex 慢 5s,pi/zcode 立即返回;ListModels 整段
// 在 ctx 预算(~500ms)内返回,pi/zcode 条目在场,codex 如实缺席并说明原因。
func TestListModelsSlowProviderDoesNotBlockOthers(t *testing.T) {
	srv := New(Config{Store: mustStore(t), Providers: map[string]agent.Provider{
		"codex": &slowModelsProvider{id: "codex", delay: 5 * time.Second}, // 伪造 codex 初始化持锁(慢探测)
		"pi":    &slowModelsProvider{id: "pi", models: []pb.ModelInfo{{Id: "default", Provider: "pi"}}},
		"zcode": &slowModelsProvider{id: "zcode", models: []pb.ModelInfo{{Id: "default", Provider: "zcode"}}},
	}})
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	start := time.Now()
	res, err := srv.ListModels(ctx, &pb.ListModelsRequest{})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("ListModels 不应失败(单个 provider 慢只影响它自己): %v", err)
	}
	if elapsed >= 2*time.Second {
		t.Fatalf("pi/zcode 被慢 codex 阻塞: 整段耗时 %v(应在 ctx 预算 500ms 量级内返回)", elapsed)
	}
	got := map[string]bool{}
	for _, m := range res.Models {
		got[m.Provider] = true
	}
	if !got["pi"] || !got["zcode"] {
		t.Fatalf("pi/zcode 条目应及时返回, 实际 providers=%v", got)
	}
	if got["codex"] {
		t.Fatal("codex 未完成拉取,不应出现在本次列表(不伪造半表为成功)")
	}
	if !strings.Contains(res.StateError, "codex") || !strings.Contains(res.StateError, "初始化中") {
		t.Fatalf("StateError 应说明 codex 初始化中(人话、可重试), got=%q", res.StateError)
	}
}

// ② 结构化可重试错误:超预算的 provider 让 ListModels 正常返回(不抛裸超时),
// StateError 文案面向用户;未超预算的 provider 不受牵连。
func TestListModelsTimeoutYieldsRetryableStateErrorNotBareTimeout(t *testing.T) {
	srv := New(Config{Store: mustStore(t), Providers: map[string]agent.Provider{
		"codex": &slowModelsProvider{id: "codex", delay: 3 * time.Second},
		"pi":    &slowModelsProvider{id: "pi", models: []pb.ModelInfo{{Id: "default", Provider: "pi"}}},
	}})
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	res, err := srv.ListModels(ctx, &pb.ListModelsRequest{})
	if err != nil {
		t.Fatalf("超预算应转成结构化回执而不是 RPC 错误(裸 DeadlineExceeded): %v", err)
	}
	if !strings.Contains(res.StateError, "初始化中") || !strings.Contains(res.StateError, "稍后重试") {
		t.Fatalf("StateError 应是可重试的人话文案, got=%q", res.StateError)
	}
	found := false
	for _, m := range res.Models {
		if m.Provider == "pi" {
			found = true
		}
	}
	if !found {
		t.Fatal("未超预算的 pi 条目不应被牵连丢失")
	}
}

// ③ 正常路径:全部提前完成 → 立即返回(不干等 10s 预算);输出顺序与旧顺序
// 实现一致(plain provider 按字典序);全部完成时 StateError 为空。
func TestListModelsReturnsAsSoonAsAllFetched(t *testing.T) {
	srv := New(Config{Store: mustStore(t), Providers: map[string]agent.Provider{
		"zcode": &slowModelsProvider{id: "zcode", models: []pb.ModelInfo{{Id: "default", Provider: "zcode"}}},
		"codex": &slowModelsProvider{id: "codex", models: []pb.ModelInfo{{Id: "default", Provider: "codex"}}},
		"pi":    &slowModelsProvider{id: "pi", models: []pb.ModelInfo{{Id: "default", Provider: "pi"}}},
	}})
	start := time.Now()
	res, err := srv.ListModels(context.Background(), &pb.ListModelsRequest{})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("全部完成应立即返回(不干等 10s 预算), 实际 %v", elapsed)
	}
	if len(res.Models) != 3 {
		t.Fatalf("应返回 3 个条目, 实际 %d(%+v)", len(res.Models), res.Models)
	}
	// plain provider 字典序:codex < pi < zcode(与旧顺序实现一致)。
	wantOrder := []string{"codex", "pi", "zcode"}
	for i, want := range wantOrder {
		if res.Models[i].Provider != want {
			t.Fatalf("输出顺序应稳定为 %v, 实际第 %d 项=%s", wantOrder, i, res.Models[i].Provider)
		}
	}
	if res.StateError != "" {
		t.Fatalf("全部成功时 StateError 应为空, got=%q", res.StateError)
	}
}

// ④ 状态预算通道的错误回执:provider 拉取失败的真实原因透传进 StateError,
// 其真实条目(default,跟随桌面)照常进列表 —— 失败 ≠ 无模型可用。
func TestListModelsSurfacesStatusSourceError(t *testing.T) {
	srv := New(Config{Store: mustStore(t), Providers: map[string]agent.Provider{
		"codex": &statusSourceStub{
			plainProvider: plainProvider{id: "codex"},
			models:        []pb.ModelInfo{{Id: "default", Provider: "codex", Available: true}},
			err:           agent.ErrModelsInitializing,
		},
	}})
	res, err := srv.ListModels(context.Background(), &pb.ListModelsRequest{})
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if !strings.Contains(res.StateError, "初始化中") {
		t.Fatalf("拉取失败原因应从 StateError 透传, got=%q", res.StateError)
	}
	if len(res.Models) != 1 || res.Models[0].Id != "codex/default" {
		t.Fatalf("真实的 default 条目应照常进列表, got=%+v", res.Models)
	}
}

// ⑤ 红线反面钉子:即便 provider 全部缺席(未完成),也绝不伪造空成功 ——
// StateError 必须点名每个缺席的 agent,让"看起来正常但选不了"不可能发生。
func TestListModelsAllPendingNamesEveryProvider(t *testing.T) {
	srv := New(Config{Store: mustStore(t), Providers: map[string]agent.Provider{
		"pi":    &slowModelsProvider{id: "pi", delay: 5 * time.Second},
		"zcode": &slowModelsProvider{id: "zcode", delay: 5 * time.Second},
	}})
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	res, err := srv.ListModels(ctx, &pb.ListModelsRequest{})
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(res.Models) != 0 {
		t.Fatalf("无人完成,列表应为空(而不是半表冒充), got=%+v", res.Models)
	}
	for _, id := range []string{"pi", "zcode"} {
		if !strings.Contains(res.StateError, id) {
			t.Fatalf("StateError 应点名缺席的 %s, got=%q", id, res.StateError)
		}
	}
	if !strings.Contains(res.StateError, agent.ErrModelsInitializing.Error()) {
		t.Fatalf("StateError 应含统一的人话文案, got=%q", res.StateError)
	}
}
