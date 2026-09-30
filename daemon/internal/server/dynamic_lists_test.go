package server

// dynamic_lists_test.go —— 2026-09-20 批3 "写死名单动态化"五处整改的回归钉子
// (README §7 / AUDIT 附录 A):
//
//	1. CreateThread:未知 agent 报错文案动态列举可用 agent;注册表里有的就接入;
//	2. ListModels 顺序:从 s.cfg.Providers 动态派生(桌面桥接在前,echo 最后),
//	   新注册 provider 自动进列表;
//	3. defaultModelIDFor:default 哨兵 → 列表首项 → 明确报错,绝不伪造 ID;
//	4. agents_sessions:导入能力按接口动态发现,报错文案动态列举;
//	5. defaultProvider:store 默认模型的 provider 前缀优先,echo 永远最后兜底。

import (
	"context"
	"strings"
	"testing"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// prefsAwareStub 实现 agent.PrefsAware:模拟"桌面桥接 agent"(ListModels 排序
// 据此把桌面 agent 排在前)。
type prefsAwareStub struct {
	plainProvider
	models []pb.ModelInfo
}

func (p *prefsAwareStub) Models() []pb.ModelInfo        { return p.models }
func (p *prefsAwareStub) ApplyPrefs(agent.DesktopPrefs) {}

// listableStub 只带模型表,无任何扩展能力(模拟 BYOK / 未来新 provider)。
type listableStub struct {
	plainProvider
	models []pb.ModelInfo
}

func (p *listableStub) Models() []pb.ModelInfo { return p.models }

// futureImporter 自带桌面会话导入能力:模拟"下一个桌面 agent"不经任何名单注册、
// 仅凭实现 AgentSessionImporter 就被导入路径发现。
type futureImporter struct {
	plainProvider
	sessions []*pb.AgentThreadInfo
}

func (p *futureImporter) ListDesktopSessions(_ context.Context) ([]*pb.AgentThreadInfo, error) {
	return p.sessions, nil
}
func (p *futureImporter) ImportModelID() string { return p.id + "/default" }

// ---- 1. CreateThread:未知 agent 动态报错 + 尽力接入 ----

func TestCreateThreadUnknownAgentErrorListsAvailable(t *testing.T) {
	srv := New(Config{Store: mustStore(t), Providers: map[string]agent.Provider{
		"pi":     &listableStub{plainProvider: plainProvider{id: "pi"}, models: []pb.ModelInfo{{Id: "default"}}},
		"future": &listableStub{plainProvider: plainProvider{id: "future"}, models: []pb.ModelInfo{{Id: "m"}}},
	}})
	_, err := srv.CreateThread(ctxBG(), &pb.CreateThreadRequest{AgentId: "nosuch"})
	if err == nil {
		t.Fatal("未知 agent 应显式报错")
	}
	// 文案必须动态列举当前可用 agent:注册了什么列什么,与任何写死名单无关。
	for _, want := range []string{"nosuch", "pi", "future"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("报错文案应动态包含 %q: %v", want, err)
		}
	}
	if strings.Contains(err.Error(), "支持 codex") || strings.Contains(err.Error(), "支持 pi / zcode") {
		t.Fatalf("报错文案不应再写死支持名单: %v", err)
	}
}

func TestCreateThreadAcceptsAnyRegisteredAgent(t *testing.T) {
	// 尽力接入:只要 agent 在注册表里就接(哪怕 daemon 从没听说过这个名字)。
	srv := New(Config{Store: mustStore(t), Providers: map[string]agent.Provider{
		"brand-new-agent": &listableStub{plainProvider: plainProvider{id: "brand-new-agent"},
			models: []pb.ModelInfo{{Id: "m1"}}},
	}})
	resp, err := srv.CreateThread(ctxBG(), &pb.CreateThreadRequest{AgentId: "brand-new-agent"})
	if err != nil {
		t.Fatalf("注册表里有的 agent 应被接入: %v", err)
	}
	if resp.AgentId != "brand-new-agent" {
		t.Fatalf("归属应为 brand-new-agent,得到 %q", resp.AgentId)
	}
}

// ---- 2. ListModels 顺序动态派生 ----

func TestListModelsOrderDerivedFromProviders(t *testing.T) {
	mk := func(id, model string) agent.Provider {
		return &prefsAwareStub{plainProvider: plainProvider{id: id},
			models: []pb.ModelInfo{{Id: model}}}
	}
	srv := New(Config{Store: mustStore(t), Providers: map[string]agent.Provider{
		"zcode": mk("zcode", "zc-1"),
		"echo":  &listableStub{plainProvider: plainProvider{id: "echo"}, models: []pb.ModelInfo{{Id: "echo-1"}}},
		"codex": mk("codex", "cx-1"),
		// 未来新增的 provider:不实现 PrefsAware,自动落在桌面组之后。
		"new-agent": &listableStub{plainProvider: plainProvider{id: "new-agent"},
			models: []pb.ModelInfo{{Id: "na-1"}}},
	}})
	res, err := srv.ListModels(ctxBG(), &pb.ListModelsRequest{})
	if err != nil {
		t.Fatalf("ListModels 失败: %v", err)
	}
	var got []string
	for _, m := range res.Models {
		got = append(got, m.Provider)
	}
	want := []string{"codex", "zcode", "new-agent", "echo"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("顺序应动态派生(桌面在前、新 provider 自动入列、echo 最后):\n got=%v\nwant=%v", got, want)
	}
}

// ---- 3. defaultModelIDFor:哨兵 → 首项 → 明确报错 ----

func TestDefaultModelIDForResolution(t *testing.T) {
	st := mustStore(t)
	srv := New(Config{Store: st, Providers: map[string]agent.Provider{
		"a": &listableStub{plainProvider: plainProvider{id: "a"},
			models: []pb.ModelInfo{{Id: "default"}, {Id: "m2"}}},
		"b": &listableStub{plainProvider: plainProvider{id: "b"},
			models: []pb.ModelInfo{{Id: "m1"}, {Id: "m2"}}},
		"c": &listableStub{plainProvider: plainProvider{id: "c"}},
	}})

	// 列表里有 default 哨兵 → 用哨兵(跟随桌面当前配置)。
	if got, err := srv.defaultModelIDFor("a"); err != nil || got != "a/default" {
		t.Fatalf("应解析 a/default,得到 %q err=%v", got, err)
	}
	// 没有 default 哨兵 → 回退列表首项,不编造 "b/default"。
	if got, err := srv.defaultModelIDFor("b"); err != nil || got != "b/m1" {
		t.Fatalf("应回退首项 b/m1,得到 %q err=%v", got, err)
	}
	// 空模型列表 → 明确报错,绝不伪造 ID。
	if got, err := srv.defaultModelIDFor("c"); err == nil || got != "" {
		t.Fatalf("无模型应明确报错,得到 %q err=%v", got, err)
	}
	// 设置页为该 agent 落盘的默认模型仍然最优先(向后兼容)。
	if err := st.SetAgentDefaults("b", store.TurnSettings{Model: "b/chosen"}); err != nil {
		t.Fatal(err)
	}
	if got, err := srv.defaultModelIDFor("b"); err != nil || got != "b/chosen" {
		t.Fatalf("AgentDefaults 应最优先,得到 %q err=%v", got, err)
	}
}

// ---- 4. agents_sessions:能力接口动态发现 + 动态报错文案 ----

func TestAgentSessionImportRejectsUnknownWithDynamicList(t *testing.T) {
	srv := newAgentSessionsServer(t) // 内置适配器:pi / zcode / claude 可导入(claude 为 2026-09-30 只读接入)
	for _, call := range []struct {
		name string
		do   func() error
	}{
		{"ListAgentThreads", func() error {
			_, err := srv.ListAgentThreads(ctxBG(), &pb.ListAgentThreadsRequest{AgentId: "no-such-agent"})
			return err
		}},
		{"ImportAgentThread", func() error {
			_, err := srv.ImportAgentThread(ctxBG(), &pb.ImportAgentThreadRequest{AgentId: "no-such-agent", SessionId: "s1"})
			return err
		}},
	} {
		err := call.do()
		if err == nil {
			t.Fatalf("%s: 未知 agent 应显式报错", call.name)
		}
		// 文案动态列举当前可导入的 agent(来自能力表,不是写死字符串)。
		for _, want := range importableAgentIDs(srv.importableAgents()) {
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("%s: 报错文案应动态包含可导入的 %q: %v", call.name, want, err)
			}
		}
	}
}

func TestAgentSessionImportDiscoversCapableProvider(t *testing.T) {
	// "下一个桌面 agent"只凭实现 AgentSessionImporter 就进导入路径:
	// 不改 agents_sessions.go 任何名单,列表与导入都直接可用。
	fut := &futureImporter{plainProvider: plainProvider{id: "future-agent"},
		sessions: []*pb.AgentThreadInfo{{AgentId: "future-agent", SessionId: "sid-9", Title: "未来会话"}}}
	srv := New(Config{Store: mustStore(t), Providers: map[string]agent.Provider{"future-agent": fut}})

	list, err := srv.ListAgentThreads(ctxBG(), &pb.ListAgentThreadsRequest{AgentId: "future-agent"})
	if err != nil || len(list.Threads) != 1 || list.Threads[0].SessionId != "sid-9" {
		t.Fatalf("具备导入能力的 provider 应动态可用: list=%v err=%v", list, err)
	}
	resp, err := srv.ImportAgentThread(ctxBG(), &pb.ImportAgentThreadRequest{
		AgentId: "future-agent", SessionId: "sid-9", Cwd: `G:\proj`,
	})
	if err != nil {
		t.Fatalf("导入失败: %v", err)
	}
	th, ok := srv.cfg.Store.Get(resp.ThreadId)
	if !ok || threadAgentID(th) != "future-agent" || th.ModelID != "future-agent/default" {
		t.Fatalf("导入绑定错误: ok=%v agent=%q model=%q", ok, threadAgentID(th), th.ModelID)
	}
}

// ---- 5. defaultProvider:store 推导优先,echo 永远最后 ----

func TestDefaultProviderDerivesFromStore(t *testing.T) {
	// (a) store 默认模型的 provider 前缀优先兑现。
	st := mustStore(t)
	if err := st.SetDefaultModelID("pi/model-a"); err != nil {
		t.Fatal(err)
	}
	srv := New(Config{Store: st, Providers: map[string]agent.Provider{
		"codex": &listableStub{plainProvider: plainProvider{id: "codex"}},
		"pi":    &listableStub{plainProvider: plainProvider{id: "pi"}},
		"echo":  &listableStub{plainProvider: plainProvider{id: "echo"}},
	}})
	if p := srv.defaultProvider(); p.ID() != "pi" {
		t.Fatalf("应按 store 默认模型前缀选 pi,得到 %q", p.ID())
	}

	// (b) 无设置:ID 字典序第一个非 echo provider。
	st2 := mustStore(t)
	srv2 := New(Config{Store: st2, Providers: map[string]agent.Provider{
		"zcode": &listableStub{plainProvider: plainProvider{id: "zcode"}},
		"codex": &listableStub{plainProvider: plainProvider{id: "codex"}},
		"echo":  &listableStub{plainProvider: plainProvider{id: "echo"}},
	}})
	if p := srv2.defaultProvider(); p.ID() != "codex" {
		t.Fatalf("无设置应选字典序第一个非 echo(codex),得到 %q", p.ID())
	}

	// (c) store 指向已不存在的 provider → 作废,继续回退链。
	if err := st2.SetDefaultModelID("ghost/m1"); err != nil {
		t.Fatal(err)
	}
	if p := srv2.defaultProvider(); p.ID() != "codex" {
		t.Fatalf("store 指向缺失 provider 应回退字典序,得到 %q", p.ID())
	}

	// (d) 只剩 echo:echo 永远最后兜底。
	srv3 := New(Config{Store: mustStore(t), Providers: map[string]agent.Provider{
		"echo": &listableStub{plainProvider: plainProvider{id: "echo"}},
	}})
	if p := srv3.defaultProvider(); p == nil || p.ID() != "echo" {
		t.Fatalf("echo 应兜底,得到 %v", p)
	}
}
