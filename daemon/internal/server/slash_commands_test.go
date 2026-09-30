package server

// slash_commands_test.go —— ListSlashCommands 路由行为钉子(不依赖真 ZCode)。
//
// 【钉什么】方案验收的三态 + 两条通路:
//   - 非 zcode 会话        → 空列表,且完全不触碰拉取(不冷启子进程);
//   - zcode + 无外部会话   → 空列表(线程还没在电脑上落地,不凭空建会话);
//   - zcode + 有外部会话   → fake 拉取结果按序透出,字段一一映射;
//   - 拉取失败             → 空列表 + 无错误(面板是锦上添花,绝不报错打断输入);
//   - 缓存                 → 10 分钟内第二次调用不再触发拉取;
//   - provider 账本通路    → zcode provider 实现 zcodeSlashSource 时优先走它。
//
// 会话不存在 / 空 thread_id 的错误语义与 GetThread 同款(errNotFound)。

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// slashFakeProvider 实现 agent.Provider + zcodeSlashSource(账本通路用)。
type slashFakeProvider struct {
	seqProvider
	commands []*pb.SlashCommand
}

func (p *slashFakeProvider) SlashCommandsFor(threadID string) []*pb.SlashCommand {
	return p.commands
}

// newSlashServer 建一个只挂所需 provider 的 Server,并落一条测试线程。
func newSlashServer(t *testing.T, providers map[string]agent.Provider, th *store.Thread) *Server {
	t.Helper()
	fs, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("建测试存储失败: %v", err)
	}
	if err := fs.Create(th); err != nil {
		t.Fatalf("建线程失败: %v", err)
	}
	if providers == nil {
		providers = map[string]agent.Provider{}
	}
	return New(Config{Store: fs, Providers: providers})
}

func TestListSlashCommandsNonZcodeReturnsEmpty(t *testing.T) {
	slashCacheFlush()
	calls := 0
	restore := zcodeSlashFetch
	zcodeSlashFetch = func(ctx context.Context, sessionID string) ([]*pb.SlashCommand, error) {
		calls++
		return nil, errors.New("不该被调用")
	}
	defer func() { zcodeSlashFetch = restore }()
	// pi 会话:即使注册表里真有 zcode,也不该显示 zcode 命令面板。
	srv := newSlashServer(t, map[string]agent.Provider{
		"zcode": &slashFakeProvider{seqProvider: seqProvider{id: "zcode"}},
	}, &store.Thread{ID: "th-pi", Title: "pi 会话", AgentID: "pi",
		ExternalSession: store.ExternalSessionRef{SessionID: "sess_pi"}})
	resp, err := srv.ListSlashCommands(context.Background(), &pb.ListSlashCommandsRequest{ThreadId: "th-pi"})
	if err != nil {
		t.Fatalf("非 zcode 会话不是错误: %v", err)
	}
	if len(resp.Commands) != 0 {
		t.Fatalf("非 zcode 会话应回空列表,得到 %d 条", len(resp.Commands))
	}
	if calls != 0 {
		t.Fatalf("非 zcode 会话不应触发拉取(调用 %d 次)", calls)
	}
}

func TestListSlashCommandsZcodeWithoutSessionReturnsEmpty(t *testing.T) {
	slashCacheFlush()
	calls := 0
	restore := zcodeSlashFetch
	zcodeSlashFetch = func(ctx context.Context, sessionID string) ([]*pb.SlashCommand, error) {
		calls++
		return nil, errors.New("不该被调用")
	}
	defer func() { zcodeSlashFetch = restore }()
	// zcode 会话但还没在电脑上落地(无外部会话):空列表,不凭空建会话。
	srv := newSlashServer(t, nil, &store.Thread{ID: "th-zc", Title: "新 zcode 会话", AgentID: "zcode"})
	resp, err := srv.ListSlashCommands(context.Background(), &pb.ListSlashCommandsRequest{ThreadId: "th-zc"})
	if err != nil {
		t.Fatalf("无快照不是错误: %v", err)
	}
	if len(resp.Commands) != 0 {
		t.Fatalf("无外部会话应回空列表,得到 %d 条", len(resp.Commands))
	}
	if calls != 0 {
		t.Fatalf("无外部会话不应触发拉取(调用 %d 次)", calls)
	}
}

func TestListSlashCommandsFetchSuccessAndCache(t *testing.T) {
	slashCacheFlush()
	calls := 0
	restore := zcodeSlashFetch
	zcodeSlashFetch = func(ctx context.Context, sessionID string) ([]*pb.SlashCommand, error) {
		calls++
		if sessionID != "sess_zc1" {
			t.Fatalf("应按线程绑定的会话 id 拉取,得到 %q", sessionID)
		}
		return []*pb.SlashCommand{
			// 字段顺序钉死:name/description/inputHint。
			{Name: "goal", Description: "常驻目标", InputHint: "/goal [pause|resume|…]"},
			{Name: "compact", Description: "历史压缩", InputHint: ""},
		}, nil
	}
	defer func() { zcodeSlashFetch = restore }()
	srv := newSlashServer(t, nil, &store.Thread{ID: "th-zc1", Title: "zcode 会话", AgentID: "zcode",
		ExternalSession: store.ExternalSessionRef{SessionID: "sess_zc1"}})
	req := &pb.ListSlashCommandsRequest{ThreadId: "th-zc1"}
	resp, err := srv.ListSlashCommands(context.Background(), req)
	if err != nil {
		t.Fatalf("拉取成功不应报错: %v", err)
	}
	// 原生 2 条在前(顺序保持),聚合层补的 fork/rewind 漏报条目追加在后。
	if len(resp.Commands) != 4 {
		t.Fatalf("应透出 2 原生 + 2 补充共 4 条命令,得到 %d 条: %+v", len(resp.Commands), resp.Commands)
	}
	got := resp.Commands[0]
	if got.Name != "goal" || got.Description != "常驻目标" || got.InputHint != "/goal [pause|resume|…]" {
		t.Fatalf("第一条字段映射不符: %+v", got)
	}
	if resp.Commands[1].Name != "compact" || resp.Commands[1].InputHint != "" {
		t.Fatalf("第二条字段映射不符(空 inputHint 要保住空串): %+v", resp.Commands[1])
	}
	// 补充条目字段钉子(出处:zcode.cjs 0.16.9 内建注册面 $R 的 fork/rewind 行)。
	if resp.Commands[2].Name != "fork" || resp.Commands[2].InputHint != "/fork [latest|checkpointId]" ||
		resp.Commands[2].Description == "" {
		t.Fatalf("fork 补充条目不符: %+v", resp.Commands[2])
	}
	if resp.Commands[3].Name != "rewind" || resp.Commands[3].InputHint != "/rewind [latest|checkpointId]" ||
		resp.Commands[3].Description == "" {
		t.Fatalf("rewind 补充条目不符: %+v", resp.Commands[3])
	}
	// 第二次调用命中缓存:fake 只会被拉取一次,且缓存的已是补齐后的清单。
	second, err := srv.ListSlashCommands(context.Background(), req)
	if err != nil {
		t.Fatalf("缓存命中调用不应报错: %v", err)
	}
	if calls != 1 {
		t.Fatalf("10 分钟内第二次调用应命中缓存(fake 被调 %d 次)", calls)
	}
	if len(second.Commands) != 4 {
		t.Fatalf("缓存命中的清单也应含补充条目,得到 %d 条", len(second.Commands))
	}
}

func TestListSlashCommandsFetchFailureReturnsEmpty(t *testing.T) {
	slashCacheFlush()
	restore := zcodeSlashFetch
	zcodeSlashFetch = func(ctx context.Context, sessionID string) ([]*pb.SlashCommand, error) {
		return nil, errors.New("node 未找到")
	}
	defer func() { zcodeSlashFetch = restore }()
	srv := newSlashServer(t, nil, &store.Thread{ID: "th-zc2", Title: "zcode 会话", AgentID: "zcode",
		ExternalSession: store.ExternalSessionRef{SessionID: "sess_zc2"}})
	resp, err := srv.ListSlashCommands(context.Background(), &pb.ListSlashCommandsRequest{ThreadId: "th-zc2"})
	if err != nil {
		t.Fatalf("拉取失败应降级为空列表而不是错误: %v", err)
	}
	if len(resp.Commands) != 0 {
		t.Fatalf("拉取失败应回空列表,得到 %d 条", len(resp.Commands))
	}
}

func TestListSlashCommandsProviderLedgerWins(t *testing.T) {
	slashCacheFlush()
	calls := 0
	restore := zcodeSlashFetch
	zcodeSlashFetch = func(ctx context.Context, sessionID string) ([]*pb.SlashCommand, error) {
		calls++
		return nil, errors.New("不该被调用")
	}
	defer func() { zcodeSlashFetch = restore }()
	// zcode provider 实现了账本接口:优先走内存账本,不冷启子进程;
	// 账本清单是原生子集,聚合层照样补漏报的 fork/rewind(共 3 条)。
	ledger := &slashFakeProvider{
		seqProvider: seqProvider{id: "zcode"},
		commands:    []*pb.SlashCommand{{Name: "init", Description: "初始化", InputHint: "/init"}},
	}
	srv := newSlashServer(t, map[string]agent.Provider{"zcode": ledger},
		&store.Thread{ID: "th-zc3", Title: "zcode 会话", AgentID: "zcode",
			ExternalSession: store.ExternalSessionRef{SessionID: "sess_zc3"}})
	resp, err := srv.ListSlashCommands(context.Background(), &pb.ListSlashCommandsRequest{ThreadId: "th-zc3"})
	if err != nil {
		t.Fatalf("账本通路不应报错: %v", err)
	}
	if len(resp.Commands) != 3 || resp.Commands[0].Name != "init" {
		t.Fatalf("应透出账本命令 + 补充条目: %+v", resp.Commands)
	}
	if resp.Commands[1].Name != "fork" || resp.Commands[2].Name != "rewind" {
		t.Fatalf("补充条目应追加在原生之后: %+v", resp.Commands)
	}
	if calls != 0 {
		t.Fatalf("账本通路不应触发子进程拉取(调用 %d 次)", calls)
	}
}

func TestListSlashCommandsThreadNotFound(t *testing.T) {
	slashCacheFlush()
	srv := newSlashServer(t, nil, &store.Thread{ID: "th-x", Title: "占位", AgentID: "zcode"})
	_, err := srv.ListSlashCommands(context.Background(), &pb.ListSlashCommandsRequest{ThreadId: "nope"})
	if err == nil {
		t.Fatal("会话不存在必须报错")
	}
	if status.Code(err) != codes.NotFound {
		t.Fatalf("应回 NotFound,得到 %v", status.Code(err))
	}
	if !strings.Contains(err.Error(), "会话不存在") {
		t.Fatalf("错误文案要能让人看懂: %v", err)
	}
}

// 缓存 TTL 过期后重新拉取(把 TTL 条目做旧,避免真等 10 分钟)。
func TestListSlashCommandsCacheExpires(t *testing.T) {
	slashCacheFlush()
	calls := 0
	restore := zcodeSlashFetch
	zcodeSlashFetch = func(ctx context.Context, sessionID string) ([]*pb.SlashCommand, error) {
		calls++
		return []*pb.SlashCommand{{Name: "goal"}}, nil
	}
	defer func() { zcodeSlashFetch = restore }()
	srv := newSlashServer(t, nil, &store.Thread{ID: "th-zc4", Title: "zcode 会话", AgentID: "zcode",
		ExternalSession: store.ExternalSessionRef{SessionID: "sess_zc4"}})
	req := &pb.ListSlashCommandsRequest{ThreadId: "th-zc4"}
	if _, err := srv.ListSlashCommands(context.Background(), req); err != nil {
		t.Fatalf("首次拉取不应报错: %v", err)
	}
	// 手工把缓存条目做旧到 TTL 之外。
	slashMu.Lock()
	slashCache["sess_zc4"] = slashCacheEntry{commands: slashCache["sess_zc4"].commands, at: time.Now().Add(-slashCacheTTL - time.Second)}
	slashMu.Unlock()
	if _, err := srv.ListSlashCommands(context.Background(), req); err != nil {
		t.Fatalf("过期后拉取不应报错: %v", err)
	}
	if calls != 2 {
		t.Fatalf("TTL 过期后应重新拉取(fake 被调 %d 次)", calls)
	}
}

// ---- zcode 漏报补充(zcodeWithPromptCommandSupplement)的行为钉子 ----

// 原生清单已含同名条目(如用户自定义命令恰好叫 fork)时去重:fork 只出现一次,
// rewind 照补;大小写变体(Fork)也算命中,避免面板出现两条同义命令。
func TestZcodeSupplementDedupesNativeEntries(t *testing.T) {
	native := []*pb.SlashCommand{
		{Name: "goal", Description: "g", InputHint: "/goal"},
		{Name: "fork", Description: "自定义 fork 命令", InputHint: "/fork [自定义]"},
	}
	got := zcodeWithPromptCommandSupplement(native)
	if len(got) != 3 {
		t.Fatalf("fork 已列时只应补 rewind,得到 %d 条: %+v", len(got), got)
	}
	if got[1].Name != "fork" || got[1].Description != "自定义 fork 命令" {
		t.Fatalf("同名原生条目必须原样保留(不得被补充覆盖): %+v", got[1])
	}
	if got[2].Name != "rewind" {
		t.Fatalf("rewind 应照补: %+v", got[2])
	}
	if got := zcodeWithPromptCommandSupplement([]*pb.SlashCommand{{Name: "Fork"}}); len(got) != 2 {
		t.Fatalf("大小写变体 Fork 也应视为已列,得到 %+v", got)
	}
}

// 原生清单为"显式空"(会话没有任何命令)时补充仍然生效:内建文本命令与
// 工作区自定义命令互不依赖,空只是没有 custom,不代表 fork/rewind 不可用。
func TestZcodeSupplementAppliesToEmptyNativeList(t *testing.T) {
	got := zcodeWithPromptCommandSupplement([]*pb.SlashCommand{})
	if len(got) != len(zcodePromptCommandSupplement) {
		t.Fatalf("空原生清单应得到全部补充条目,得到 %d 条: %+v", len(got), got)
	}
}

// 补充不得改动入参切片:账本路径传入的是 provider 共享的底层数组,
// 聚合层改了它就等于改了 provider 的记账(下一次查询会重复追加)。
func TestZcodeSupplementDoesNotMutateInput(t *testing.T) {
	native := []*pb.SlashCommand{{Name: "goal", Description: "g"}}
	before := len(native)
	_ = zcodeWithPromptCommandSupplement(native)
	if len(native) != before || native[0].Name != "goal" {
		t.Fatalf("入参切片被改动: %+v", native)
	}
}

// nil(拉取失败/无快照)原样返回 nil:失败路径安静收起面板,绝不凭空发条目。
func TestZcodeSupplementNilStaysNil(t *testing.T) {
	if got := zcodeWithPromptCommandSupplement(nil); got != nil {
		t.Fatalf("nil 入参应返回 nil,得到 %+v", got)
	}
}
