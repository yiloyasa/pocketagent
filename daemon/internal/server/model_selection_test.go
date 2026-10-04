package server

// model_selection_test.go —— SelectionResolver 的判定与校验钉子
// (2026-10-04 会话选择计划 Task 2;wire 层兼容钉子在 model_selection_wire_test.go,
// 两个文件分工:那边钉"协议只增不改",这里钉"四级来源解析 + 校验 + 变化判定")。
//
// 【测什么】
//  1. source 四优先级逐例:会话覆盖 → default 哨兵(桌面默认)→ App 默认
//     (defaultModelIDFor 先例:AgentDefaults → 哨兵 → 列表首项)→ 原生当前值;
//  2. 未知模型/无效 effort:人话报错并附可用值,绝不静默改成默认(计划规则);
//  3. ResolveThreadSelection 全字段(含 applies_to_next_turn 的运行/空闲两态);
//  4. captureTurnSnapshot 快照面(strict=false 的尽力判定语义);
//  5. selection_changed 只在真变化时广播(首次观察/相等都不算)。
//
// 【怎么测】能力数据全部来自打桩 provider 的动态模型表 —— 不写死任何
// agent/模型名单(全局约束红线),与 collabTestProvider/seqProvider 同构。

import (
	"context"
	"strings"
	"testing"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// selectionTestProvider 可编程 provider:动态模型表 + 可选 CurrentSelection 打桩。
type selectionTestProvider struct {
	id     string
	models []pb.ModelInfo
	// curSel 非空 = provider 实现 agent.CurrentSelectionReader 且按它应答;
	// nil = 读取失败(与 seqProvider 的 followUp 同款"方法总在、语义由字段定")。
	curSel func(ctx context.Context, t *store.Thread) (string, *string, error)
}

func (p *selectionTestProvider) ID() string          { return p.id }
func (p *selectionTestProvider) DisplayName() string { return "选择解析测试 provider" }
func (p *selectionTestProvider) Models() []pb.ModelInfo {
	return append([]pb.ModelInfo(nil), p.models...)
}
func (p *selectionTestProvider) Run(_ context.Context, _ agent.TurnRequest, _ agent.Sink, _ agent.ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
	return []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "ok"}}}}, &pb.Usage{}, nil
}

// CurrentSelection 总在(编译期实现 CurrentSelectionReader);语义由 curSel 决定。
func (p *selectionTestProvider) CurrentSelection(ctx context.Context, t *store.Thread) (string, *string, error) {
	if p.curSel == nil {
		return "", nil, context.DeadlineExceeded
	}
	return p.curSel(ctx, t)
}

var _ agent.CurrentSelectionReader = (*selectionTestProvider)(nil)

// strPtr 测试小工具:effort 的 *string 形状。
func strPtr(v string) *string { return &v }

// newSelectionTestServer 带打桩 provider 的解析测试服务器。
func newSelectionTestServer(t *testing.T, ps ...*selectionTestProvider) *Server {
	t.Helper()
	fs, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("建测试存储失败: %v", err)
	}
	providers := map[string]agent.Provider{}
	for _, p := range ps {
		providers[p.ID()] = p
	}
	return New(Config{Store: fs, Providers: providers, Workspace: t.TempDir()})
}

// TestResolveThreadSelectionSourcePriority 四优先级逐例(决策树 1→4 级)。
func TestResolveThreadSelectionSourcePriority(t *testing.T) {
	pi := &selectionTestProvider{id: "pi", models: []pb.ModelInfo{
		{Id: "glm-5.3", AvailableEfforts: []string{"low", "high"}},
	}}
	cases := []struct {
		name          string
		provider      *selectionTestProvider
		thread        *store.Thread
		setup         func(*Server)
		wantSource    string
		wantModel     string
		wantEffort    string
		wantConfirmed bool
	}{
		{
			name:       "1 会话覆盖最高优先(非哨兵非空且通过校验)",
			provider:   pi,
			thread:     &store.Thread{ID: "th-a", AgentID: "pi", ModelID: "pi/glm-5.3", Effort: "high"},
			wantSource: SelectionSourceThreadOverride, wantModel: "pi/glm-5.3", wantEffort: "high", wantConfirmed: true,
		},
		{
			name: "2 default 哨兵 = 显式跟随桌面(desktop_default)",
			provider: &selectionTestProvider{id: "codex", models: []pb.ModelInfo{
				{Id: "default"}, {Id: "gpt-x"},
			}},
			thread:     &store.Thread{ID: "th-b", AgentID: "codex", ModelID: "codex/default"},
			wantSource: SelectionSourceDesktopDefault, wantModel: "codex/default",
		},
		{
			name:       "3 空模型 → App 默认(列表首项,无哨兵时)",
			provider:   pi,
			thread:     &store.Thread{ID: "th-c", AgentID: "pi", ModelID: ""},
			wantSource: SelectionSourceAppDefault, wantModel: "pi/glm-5.3",
		},
		{
			name:     "4 空模型 → App 默认(AgentDefaults 落盘值优先于列表)",
			provider: pi,
			thread:   &store.Thread{ID: "th-d", AgentID: "pi", ModelID: ""},
			setup: func(s *Server) {
				if err := s.cfg.Store.SetAgentDefaults("pi", store.TurnSettings{Model: "pi/zc-9", Effort: "low"}); err != nil {
					t.Fatalf("落 AgentDefaults 失败: %v", err)
				}
			},
			wantSource: SelectionSourceAppDefault, wantModel: "pi/zc-9", wantEffort: "low",
		},
		{
			name: "5 空模型 → default 哨兵优先于首项 = 桌面默认",
			provider: &selectionTestProvider{id: "zcode", models: []pb.ModelInfo{
				{Id: "default"}, {Id: "zc-1"},
			}},
			thread:     &store.Thread{ID: "th-e", AgentID: "zcode", ModelID: ""},
			wantSource: SelectionSourceDesktopDefault, wantModel: "zcode/default",
		},
		{
			name: "6 模型列表不可用 → 原生当前值兜底(native_current)",
			provider: &selectionTestProvider{id: "pi", curSel: func(context.Context, *store.Thread) (string, *string, error) {
				return "pi/raw-current", strPtr("high"), nil
			}},
			thread:     &store.Thread{ID: "th-f", AgentID: "pi", ModelID: ""},
			wantSource: SelectionSourceNativeCurrent, wantModel: "pi/raw-current", wantEffort: "high",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newSelectionTestServer(t, c.provider)
			if c.setup != nil {
				c.setup(s)
			}
			sel, err := s.ResolveThreadSelection(context.Background(), c.thread)
			if err != nil {
				t.Fatalf("解析不应失败: %v", err)
			}
			if sel.Source != c.wantSource || sel.ModelId != c.wantModel || sel.Effort != c.wantEffort {
				t.Fatalf("source/model/effort = %q/%q/%q, want %q/%q/%q",
					sel.Source, sel.ModelId, sel.Effort, c.wantSource, c.wantModel, c.wantEffort)
			}
			if sel.ConfigurationConfirmed != c.wantConfirmed {
				t.Fatalf("configuration_confirmed = %v, want %v(只有会话显式覆盖为 true)", sel.ConfigurationConfirmed, c.wantConfirmed)
			}
			if sel.AgentId != threadAgentID(c.thread) {
				t.Fatalf("agent_id = %q, want %q", sel.AgentId, threadAgentID(c.thread))
			}
			if sel.Explanation == "" {
				t.Fatal("每个命中来源都应带人话解释")
			}
		})
	}
}

// TestResolveThreadSelectionAllMissIsHonestError 四级全落空:显式报错,绝不编造
// 模型 id(defaultModelIDFor 的"不伪造 ID"纪律在 resolver 层延续)。
func TestResolveThreadSelectionAllMissIsHonestError(t *testing.T) {
	p := &selectionTestProvider{id: "pi", curSel: func(context.Context, *store.Thread) (string, *string, error) {
		return "", nil, context.DeadlineExceeded
	}}
	s := newSelectionTestServer(t, p)
	sel, err := s.ResolveThreadSelection(context.Background(), &store.Thread{ID: "th-none", AgentID: "pi"})
	if err == nil {
		t.Fatalf("四级全落空应显式报错,却得到结论: %+v", &sel)
	}
	if sel.Source != "" || sel.ModelId != "" || sel.Effort != "" {
		t.Fatalf("落空时不得伪造结论: %+v", &sel)
	}
}

// TestResolveThreadSelectionInvalidOverrideNoSilentFallback 计划规则:
// "未知模型不能静默改成默认" —— 会话覆盖校验失败必须报错,而不是降级到默认。
func TestResolveThreadSelectionInvalidOverrideNoSilentFallback(t *testing.T) {
	p := &selectionTestProvider{id: "pi", models: []pb.ModelInfo{
		{Id: "glm-5.3", AvailableEfforts: []string{"low", "high"}},
	}}
	s := newSelectionTestServer(t, p)
	sel, err := s.ResolveThreadSelection(context.Background(), &store.Thread{
		ID: "th-bad", AgentID: "pi", ModelID: "pi/nope", Effort: "high",
	})
	if err == nil {
		t.Fatalf("未知模型必须报错,却静默给出: %+v", &sel)
	}
	if !strings.Contains(err.Error(), "pi/nope") {
		t.Fatalf("错误应点名未知模型本身,得到: %v", err)
	}
	if sel.Source == SelectionSourceAppDefault || sel.ModelId == "pi/glm-5.3" {
		t.Fatalf("不得静默降级成默认模型: %+v(sel=%+v err=%v)", &sel, &sel, err)
	}
}

// TestResolveThreadSelectionInvalidEffortListsChoices 计划规则:
// "effort 不在当前模型能力集合时返回清楚的人话,并建议可用值"。
func TestResolveThreadSelectionInvalidEffortListsChoices(t *testing.T) {
	p := &selectionTestProvider{id: "pi", models: []pb.ModelInfo{
		{Id: "glm-5.3", AvailableEfforts: []string{"low", "high"}},
	}}
	s := newSelectionTestServer(t, p)
	_, err := s.ResolveThreadSelection(context.Background(), &store.Thread{
		ID: "th-eff", AgentID: "pi", ModelID: "pi/glm-5.3", Effort: "ultra",
	})
	if err == nil {
		t.Fatal("无效 effort 必须报错")
	}
	for _, want := range []string{"ultra", "low", "high"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("错误应包含 %q 与可用档位,得到: %v", want, err)
		}
	}
}

// TestValidateSelectionMatrix 校验矩阵:模型归属、effort 档位、空值合法、
// 未知 agent 动态列举(不写死名单)。
func TestValidateSelectionMatrix(t *testing.T) {
	pi := &selectionTestProvider{id: "pi", models: []pb.ModelInfo{
		{Id: "glm-5.3", AvailableEfforts: []string{"low", "high"}},
		{Id: "default"},
	}}
	codex := &selectionTestProvider{id: "codex", models: []pb.ModelInfo{{Id: "gpt-x"}}} // 无动态档位
	s := newSelectionTestServer(t, pi, codex)

	cases := []struct {
		name            string
		agentID         string
		modelID         string
		effort          string
		wantErrContains []string
	}{
		{"合法:模型+档位都在", "pi", "pi/glm-5.3", "high", nil},
		{"空模型 = 跟随默认,合法放行", "pi", "", "", nil},
		{"空 effort = 跟随默认强度,合法放行", "pi", "pi/glm-5.3", "", nil},
		{"default 哨兵是列表成员,合法", "pi", "pi/default", "", nil},
		{"无动态档位的模型不设限(不用内置名单拦人)", "codex", "codex/gpt-x", "xhigh", nil},
		{"未知模型:点名 + 附可用列表", "pi", "pi/nope", "", []string{"pi/nope", "pi/glm-5.3", "不"}},
		{"无效 effort:点名 + 附全部可用档位", "pi", "pi/glm-5.3", "ultra", []string{"ultra", "low", "high"}},
		{"未知 agent:动态列举当前可用 agent", "ghost", "ghost/m", "", []string{"ghost", "pi", "codex"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := s.ValidateSelection(c.agentID, c.modelID, c.effort)
			if len(c.wantErrContains) == 0 {
				if err != nil {
					t.Fatalf("应放行,却报错: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("应报错,却放行")
			}
			for _, want := range c.wantErrContains {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("错误应包含 %q(能力来自动态数据,不得写死名单),得到: %v", want, err)
				}
			}
		})
	}
}

// TestResolveThreadSelectionFullFields 全字段口径:applies_to_next_turn 的
// 运行/空闲两态、动态档位透传、解释非空。
func TestResolveThreadSelectionFullFields(t *testing.T) {
	p := &selectionTestProvider{id: "pi", models: []pb.ModelInfo{
		{Id: "glm-5.3", AvailableEfforts: []string{"low", "high"}},
	}}
	s := newSelectionTestServer(t, p)
	ctx := context.Background()

	running := &store.Thread{ID: "th-run", AgentID: "pi", ModelID: "pi/glm-5.3",
		Effort: "high", Status: pb.ThreadStatus_THREAD_STATUS_RUNNING}
	sel, err := s.ResolveThreadSelection(ctx, running)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if !sel.AppliesToNextTurn {
		t.Fatal("运行中会话的选择应标 applies_to_next_turn=true(不改写进行中的轮次)")
	}
	if !sel.ConfigurationConfirmed || sel.Source != SelectionSourceThreadOverride {
		t.Fatalf("会话覆盖应 confirmed=true / source=thread_override: %+v", &sel)
	}
	if sel.Explanation == "" {
		t.Fatal("应带人话解释")
	}
	if len(sel.AvailableEfforts) != 2 || sel.AvailableEfforts[0] != "low" || sel.AvailableEfforts[1] != "high" {
		t.Fatalf("available_efforts 应透传 provider 动态档位: %v", sel.AvailableEfforts)
	}

	idle := &store.Thread{ID: "th-idle", AgentID: "pi", ModelID: "pi/glm-5.3", Effort: "high"}
	sel2, err := s.ResolveThreadSelection(ctx, idle)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if sel2.AppliesToNextTurn {
		t.Fatal("空闲会话没有在飞的轮次,不应标 applies_to_next_turn=true")
	}
}

// TestCaptureTurnSnapshotBestEffortSource 快照面:ModelID/Effort 捕获线程实际
// 取值;strict=false —— 校验失败不拦轮次(source 仍如实记 thread_override,
// 模型是否真能跑由 provider 的真实错误说话);全落空 source 留空(不编造)。
func TestCaptureTurnSnapshotBestEffortSource(t *testing.T) {
	pi := &selectionTestProvider{id: "pi", models: []pb.ModelInfo{
		{Id: "m1", AvailableEfforts: []string{"low", "high"}},
	}}
	s := newSelectionTestServer(t, pi)

	snap := s.captureTurnSnapshot(&store.Thread{
		ID: "th-snap", AgentID: "pi", ModelID: "pi/m1", Effort: "high", Workspace: "/ws",
	})
	if snap.ThreadID != "th-snap" || snap.AgentID != "pi" || snap.Workspace != "/ws" {
		t.Fatalf("快照应冻结线程上下文: %+v", snap)
	}
	if snap.ModelID != "pi/m1" || snap.Effort != "high" {
		t.Fatalf("快照应捕获实际取值: %+v", snap)
	}
	if snap.Source != SelectionSourceThreadOverride {
		t.Fatalf("快照来源应判定为 thread_override: %q", snap.Source)
	}
	if snap.CapturedAtUnixMs <= 0 {
		t.Fatal("快照应记捕获时刻")
	}

	// 覆盖无效也不拦快照(轮次照跑,provider 会给真实错误)
	snap2 := s.captureTurnSnapshot(&store.Thread{ID: "th-bad", AgentID: "pi", ModelID: "pi/nope"})
	if snap2.Source != SelectionSourceThreadOverride || snap2.ModelID != "pi/nope" {
		t.Fatalf("非严格模式不得改写/丢弃覆盖: %+v", snap2)
	}

	// 四级全落空:source 留空,不编造
	empty := &selectionTestProvider{id: "pi"}
	s2 := newSelectionTestServer(t, empty)
	snap3 := s2.captureTurnSnapshot(&store.Thread{ID: "th-lost", AgentID: "pi"})
	if snap3.Source != "" {
		t.Fatalf("来源说不清时 source 应留空,得到 %q", snap3.Source)
	}
}

// TestNoteSelectionChangedFiresOnlyOnChange 变化判定:首次观察不算变化,
// 真变化广播一次,相等(哪怕换了快照指针)不重复广播。
func TestNoteSelectionChangedFiresOnlyOnChange(t *testing.T) {
	var calls [][2]*pb.EffectiveModelSelection
	restore := setSelectionChangedHook(func(threadID string, prev, cur *pb.EffectiveModelSelection) {
		calls = append(calls, [2]*pb.EffectiveModelSelection{prev, cur})
	})
	defer restore()
	resetSelectionTracking()
	defer resetSelectionTracking()
	calls = nil

	noteSelectionChanged("th-hook", &TurnSnapshot{ThreadID: "th-hook", AgentID: "pi",
		ModelID: "pi/m1", Source: SelectionSourceAppDefault})
	if len(calls) != 0 {
		t.Fatalf("首次观察不是变化,不该广播: %d 次", len(calls))
	}

	noteSelectionChanged("th-hook", &TurnSnapshot{ThreadID: "th-hook", AgentID: "pi",
		ModelID: "pi/m2", Effort: "low", Source: SelectionSourceThreadOverride})
	if len(calls) != 1 {
		t.Fatalf("真变化应广播一次,得到 %d 次", len(calls))
	}
	if prev, cur := calls[0][0], calls[0][1]; prev.GetModelId() != "pi/m1" || cur.GetModelId() != "pi/m2" ||
		cur.GetEffort() != "low" || cur.GetSource() != SelectionSourceThreadOverride {
		t.Fatalf("广播应带前后两份结论: prev=%+v cur=%+v", prev, cur)
	}

	noteSelectionChanged("th-hook", &TurnSnapshot{ThreadID: "th-hook", AgentID: "pi",
		ModelID: "pi/m2", Effort: "low", Source: SelectionSourceThreadOverride})
	if len(calls) != 1 {
		t.Fatalf("选择相等不得重复广播(换了指针也一样): %d 次", len(calls))
	}
}
