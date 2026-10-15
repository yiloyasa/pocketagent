package server

// hermes_slash_route_test.go —— ListSlashCommands 的 hermes 分支钉子
// (2026-10-14 slash 位化;与 slash_commands_test.go 的 zcode/pi 用例同口径)。
//
// 【钉什么】
//  1. hermes 会话 + provider 命令账本命中 → 条目按序字段一一透出,零子进程;
//  2. hermes 会话 + 未记账 → 安静空列表(不冷启、无兜底拉取 —— ACP 方法面
//     没有"取命令清单"的请求方法,daemon 没有第二条数据源);
//  3. hermes provider 不在场 / 不实现账本形状 → 空列表不报错;
//  4. 别家会话(如 zcode)不误吃 hermes 账本(分支按 AgentID 路由);
//  5. hermes 分支不吃 req.Refresh(那只是 pi 的原生扩展面,本家不存在,
//     给 Refresh=true 也必须回同一份账本,不得凭空拉取)。

import (
	"context"
	"errors"
	"testing"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// hermes 分支的 provider fake 复用 slash_commands_test.go 的 slashFakeProvider
// (agent.Provider + 账本形状 SlashCommandsFor;queryCalls 记查询实参):路由只认
// 形状不认 agent 名字,zcode 分支与本分支的取数姿势相同,fake 没有分叉的理由。

// TestListSlashCommandsHermesLedgerWins 账本命中:字段按序透出,查询用线程 id。
func TestListSlashCommandsHermesLedgerWins(t *testing.T) {
	slashCacheFlush()
	ledger := &slashFakeProvider{
		seqProvider: seqProvider{id: "hermes"},
		commands: []*pb.SlashCommand{
			{Name: "help", Description: "List available commands"},
			{Name: "model", Description: "Show current model and provider, or switch models", InputHint: "model name to switch to"},
			{Name: "compress", Description: "Compress conversation context"},
		},
	}
	srv := newSlashServer(t, map[string]agent.Provider{"hermes": ledger},
		&store.Thread{ID: "th-h1", Title: "hermes 会话", AgentID: "hermes"})

	resp, err := srv.ListSlashCommands(context.Background(), &pb.ListSlashCommandsRequest{ThreadId: "th-h1"})
	if err != nil {
		t.Fatalf("账本通路不应报错: %v", err)
	}
	if len(resp.Commands) != 3 {
		t.Fatalf("应透出账本里的 3 条命令,得到 %d: %+v", len(resp.Commands), resp.Commands)
	}
	if got := resp.Commands[1]; got.Name != "model" || got.Description != "Show current model and provider, or switch models" ||
		got.InputHint != "model name to switch to" {
		t.Fatalf("inputHint 三字段映射不符: %+v", got)
	}
	if got := resp.Commands[0]; got.InputHint != "" {
		t.Fatalf("无 input 的条目必须保住空串(不得编参数提示): %+v", got)
	}
	if len(ledger.queryCalls) != 1 || ledger.queryCalls[0] != "th-h1" {
		t.Fatalf("账本查询应按线程 id,得到 %v", ledger.queryCalls)
	}
}

// TestListSlashCommandsHermesLedgerMissReturnsEmpty 未记账:安静空列表,
// 绝不冷启适配器,也不因 Refresh 凭空拉取(本家没有可拉的方法)。
func TestListSlashCommandsHermesLedgerMissReturnsEmpty(t *testing.T) {
	slashCacheFlush()
	// 未记账的账本形状:SlashCommandsFor 回 nil(daemon 刚重启/会话未建立)。
	ledger := &slashFakeProvider{seqProvider: seqProvider{id: "hermes"}}
	srv := newSlashServer(t, map[string]agent.Provider{"hermes": ledger},
		&store.Thread{ID: "th-h2", Title: "hermes 未记账", AgentID: "hermes",
			ExternalSession: store.ExternalSessionRef{SessionID: "sess_h2"}})
	for _, refresh := range []bool{false, true} {
		resp, err := srv.ListSlashCommands(context.Background(),
			&pb.ListSlashCommandsRequest{ThreadId: "th-h2", Refresh: refresh})
		if err != nil {
			t.Fatalf("未记账不是错误(refresh=%v): %v", refresh, err)
		}
		if len(resp.Commands) != 0 {
			t.Fatalf("未记账应回空列表(refresh=%v),得到 %d 条", refresh, len(resp.Commands))
		}
	}
	if len(ledger.queryCalls) != 2 {
		t.Fatalf("两次调用都应查账本(Refresh 不改变通路),得到 %v", ledger.queryCalls)
	}
}

// TestListSlashCommandsHermesRouteIsolatedByAgentID hermes 账本只在 hermes
// 会话上生效:zcode 会话走自己的分支,不吃 hermes 的命令(反之亦然)。
func TestListSlashCommandsHermesRouteIsolatedByAgentID(t *testing.T) {
	slashCacheFlush()
	restore := zcodeSlashFetch
	zcodeSlashFetch = func(ctx context.Context, sessionID string) ([]*pb.SlashCommand, error) {
		return nil, errors.New("zcode 会话不该走到拉取")
	}
	defer func() { zcodeSlashFetch = restore }()

	hermesLedger := &slashFakeProvider{seqProvider: seqProvider{id: "hermes"},
		commands: []*pb.SlashCommand{{Name: "help"}}}
	zcodeLedger := &slashFakeProvider{seqProvider: seqProvider{id: "zcode"},
		commands: []*pb.SlashCommand{{Name: "goal"}}}
	srv := newSlashServer(t, map[string]agent.Provider{"hermes": hermesLedger, "zcode": zcodeLedger},
		&store.Thread{ID: "th-zc-iso", Title: "zcode 会话", AgentID: "zcode",
			ExternalSession: store.ExternalSessionRef{SessionID: "sess_zc_iso"}})

	resp, err := srv.ListSlashCommands(context.Background(), &pb.ListSlashCommandsRequest{ThreadId: "th-zc-iso"})
	if err != nil {
		t.Fatalf("zcode 会话不应报错: %v", err)
	}
	// zcode 账本 1 条 + 聚合层 fork/rewind 补充 = 3;绝不能是 hermes 的 help。
	if len(resp.Commands) != 3 || resp.Commands[0].Name != "goal" {
		t.Fatalf("zcode 会话应走 zcode 分支: %+v", resp.Commands)
	}
	if len(hermesLedger.queryCalls) != 0 {
		t.Fatalf("zcode 会话不得触碰 hermes 账本: %v", hermesLedger.queryCalls)
	}
}

// TestListSlashCommandsHermesProviderAbsentOrUnshaped 注册表里没有 hermes /
// provider 不实现账本形状:空列表,不报错(与 pi/codex 错过 provider 同口径)。
func TestListSlashCommandsHermesProviderAbsentOrUnshaped(t *testing.T) {
	slashCacheFlush()
	// 不实现账本形状的 hermes provider(只有基础 Provider 面)。
	srv := newSlashServer(t, map[string]agent.Provider{"hermes": &seqProvider{id: "hermes"}},
		&store.Thread{ID: "th-h3", Title: "hermes 无账本形状", AgentID: "hermes"})
	resp, err := srv.ListSlashCommands(context.Background(), &pb.ListSlashCommandsRequest{ThreadId: "th-h3"})
	if err != nil {
		t.Fatalf("形状不匹配不应报错: %v", err)
	}
	if len(resp.Commands) != 0 {
		t.Fatalf("形状不匹配应回空列表: %+v", resp.Commands)
	}
	// 注册表里干脆没有 hermes。
	srv2 := newSlashServer(t, map[string]agent.Provider{},
		&store.Thread{ID: "th-h4", Title: "无 provider", AgentID: "hermes"})
	resp2, err := srv2.ListSlashCommands(context.Background(), &pb.ListSlashCommandsRequest{ThreadId: "th-h4"})
	if err != nil {
		t.Fatalf("provider 缺席不应报错: %v", err)
	}
	if len(resp2.Commands) != 0 {
		t.Fatalf("provider 缺席应回空列表: %+v", resp2.Commands)
	}
}

// TestListSlashCommandsHermesRealProviderEmptyLedger 真 *agent.HermesProvider
// 未记账:形状断言一定成立(实现 SlashCommandsFor),但账本空 → 安静空列表;
// 且不冷启(没有可断言的子进程计数,这里以"方法调得到且回 nil"为语义边界)。
func TestListSlashCommandsHermesRealProviderEmptyLedger(t *testing.T) {
	slashCacheFlush()
	srv := newSlashServer(t, map[string]agent.Provider{"hermes": agent.NewHermesProvider()},
		&store.Thread{ID: "th-h5", Title: "真 provider 未记账", AgentID: "hermes"})
	resp, err := srv.ListSlashCommands(context.Background(), &pb.ListSlashCommandsRequest{ThreadId: "th-h5"})
	if err != nil {
		t.Fatalf("未记账不应报错: %v", err)
	}
	if len(resp.Commands) != 0 {
		t.Fatalf("真 provider 未记账应回空列表: %+v", resp.Commands)
	}
}
