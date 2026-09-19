// agent_commands_test.go —— ListSlashCommands 的 codex 分支(skills 即命令)路由钉子。
//
// 【钉什么】并行协同后的三态路由:
//   - codex 会话(含 AgentID 为空的老库会话)→ 走 codex skills 源,
//     cwd 用线程 Workspace 原样透传,字段一一映射;
//   - codex 会话但 codex provider 不在场/不实现命令源 → 空列表(不是错误),
//     且绝不触碰 zcode 快照通路(zcodeSlashFetch 零调用);
//   - 同一 RPC 对 zcode/pi 的既有行为不回归(那边由 slash_commands_test 钉,
//     这里只钉"codex 分支新增后 zcode 账本断言仍优先生效"的互补半边)。
//
// 不依赖真 codex:codexSlashFake 注入 Providers["codex"](最小接口断言探测)。
package server

import (
	"context"
	"errors"
	"testing"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// codexSlashFake 实现 agent.Provider + codexSlashSource(模拟 *agent.CodexProvider)。
type codexSlashFake struct {
	seqProvider
	gotCwd     string
	commands   []*pb.SlashCommand
	errDesired bool // true 时模拟"读取失败且无缓存"
}

func (p *codexSlashFake) SlashCommands(cwd string) []*pb.SlashCommand {
	p.gotCwd = cwd
	if p.errDesired {
		return nil
	}
	return p.commands
}

// newCodexSlashServer 落一条测试线程并挂指定 codex provider(zcode 位留给假拉取探针)。
func newCodexSlashServer(t *testing.T, th *store.Thread, codex agent.Provider) *Server {
	t.Helper()
	fs, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("建测试存储失败: %v", err)
	}
	if err := fs.Create(th); err != nil {
		t.Fatalf("建线程失败: %v", err)
	}
	providers := map[string]agent.Provider{}
	if codex != nil {
		providers["codex"] = codex
	}
	return New(Config{Store: fs, Providers: providers})
}

// blockSlashFetch 换上一个"被调用即测试失败"的 zcode 拉取探针,返回还原函数。
func blockSlashFetch(t *testing.T) func() {
	t.Helper()
	restore := zcodeSlashFetch
	zcodeSlashFetch = func(ctx context.Context, sessionID string) ([]*pb.SlashCommand, error) {
		return nil, errors.New("codex 会话不应触碰 zcode 快照通路")
	}
	return func() { zcodeSlashFetch = restore }
}

func TestListSlashCommandsCodexRoutesToSkills(t *testing.T) {
	slashCacheFlush()
	defer blockSlashFetch(t)()
	fake := &codexSlashFake{
		seqProvider: seqProvider{id: "codex"},
		commands: []*pb.SlashCommand{
			{Name: "commit", Description: "提交代码"},
			{Name: "review", Description: "审查改动"},
		},
	}
	// AgentID 为空 = 老库 codex 会话,threadAgentID 兜底为 "codex",同样要路由。
	srv := newCodexSlashServer(t, &store.Thread{
		ID: "th-cx", Title: "老 codex 会话", Workspace: "G:/demo/repo",
	}, fake)
	resp, err := srv.ListSlashCommands(context.Background(), &pb.ListSlashCommandsRequest{ThreadId: "th-cx"})
	if err != nil {
		t.Fatalf("codex 会话取命令不是错误: %v", err)
	}
	if fake.gotCwd != "G:/demo/repo" {
		t.Fatalf("应把线程 Workspace 作为 cwd 透传,得到 %q", fake.gotCwd)
	}
	if len(resp.Commands) != 2 || resp.Commands[0].Name != "commit" || resp.Commands[1].Name != "review" {
		t.Fatalf("codex skills 应一一映射为命令: %+v", resp.Commands)
	}
	for _, c := range resp.Commands {
		if c.InputHint != "" {
			t.Fatalf("codex 技能无参数模板,InputHint 应为空: %+v", c)
		}
	}
}

func TestListSlashCommandsCodexWithoutSourceReturnsEmpty(t *testing.T) {
	slashCacheFlush()
	defer blockSlashFetch(t)()
	// 场景 1:codex provider 完全不在场。
	srv := newCodexSlashServer(t, &store.Thread{ID: "th-cx2", Title: "无 provider", AgentID: "codex"}, nil)
	resp, err := srv.ListSlashCommands(context.Background(), &pb.ListSlashCommandsRequest{ThreadId: "th-cx2"})
	if err != nil {
		t.Fatalf("provider 不在场不是错误: %v", err)
	}
	if len(resp.Commands) != 0 {
		t.Fatalf("provider 不在场应回空列表,得到 %d 条", len(resp.Commands))
	}
	// 场景 2:codex 在场但不实现命令源(如探测失败被 echo 兜底的场景)。
	srv2 := newCodexSlashServer(t, &store.Thread{ID: "th-cx3", Title: "无命令源", AgentID: "codex"},
		&seqProvider{id: "codex"})
	resp2, err := srv2.ListSlashCommands(context.Background(), &pb.ListSlashCommandsRequest{ThreadId: "th-cx3"})
	if err != nil {
		t.Fatalf("provider 无命令源不是错误: %v", err)
	}
	if len(resp2.Commands) != 0 {
		t.Fatalf("无命令源应回空列表,得到 %d 条", len(resp2.Commands))
	}
	// 场景 3:技能源读取失败(返回 nil)→ 同样空列表,不报错。
	fake := &codexSlashFake{seqProvider: seqProvider{id: "codex"}, errDesired: true}
	srv3 := newCodexSlashServer(t, &store.Thread{ID: "th-cx4", Title: "读取失败", AgentID: "codex"}, fake)
	resp3, err := srv3.ListSlashCommands(context.Background(), &pb.ListSlashCommandsRequest{ThreadId: "th-cx4"})
	if err != nil {
		t.Fatalf("技能读取失败不是错误: %v", err)
	}
	if len(resp3.Commands) != 0 {
		t.Fatalf("技能读取失败应回空列表,得到 %d 条", len(resp3.Commands))
	}
}

func TestListSlashCommandsZcodeLedgerStillWins(t *testing.T) {
	// 互补半边:zcode 会话 + 注册表里同时挂着 zcode 账本源与 codex 源时,
	// zcode 走 zcode 账本(路径 1),codex 源不被触碰 —— 双源各回各家。
	slashCacheFlush()
	defer blockSlashFetch(t)()
	codex := &codexSlashFake{
		seqProvider: seqProvider{id: "codex"},
		commands:    []*pb.SlashCommand{{Name: "commit"}},
	}
	zcode := &slashFakeProvider{
		seqProvider: seqProvider{id: "zcode"},
		commands:    []*pb.SlashCommand{{Name: "goal", Description: "设置目标", InputHint: "/goal …"}},
	}
	fs, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("建测试存储失败: %v", err)
	}
	th := &store.Thread{ID: "th-zc2", Title: "zcode 会话", AgentID: "zcode",
		ExternalSession: store.ExternalSessionRef{SessionID: "sess_z2"}}
	if err := fs.Create(th); err != nil {
		t.Fatalf("建线程失败: %v", err)
	}
	srv := New(Config{Store: fs, Providers: map[string]agent.Provider{"zcode": zcode, "codex": codex}})
	resp, err := srv.ListSlashCommands(context.Background(), &pb.ListSlashCommandsRequest{ThreadId: "th-zc2"})
	if err != nil {
		t.Fatalf("zcode 账本通路不是错误: %v", err)
	}
	if len(resp.Commands) != 1 || resp.Commands[0].Name != "goal" {
		t.Fatalf("zcode 会话应回 zcode 账本命令: %+v", resp.Commands)
	}
	if codex.gotCwd != "" {
		t.Fatalf("zcode 会话不应触碰 codex 技能源(cwd=%q)", codex.gotCwd)
	}
}
