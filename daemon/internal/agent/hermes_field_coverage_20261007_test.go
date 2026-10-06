// hermes_field_coverage_20261007_test.go —— hermes 字段级覆盖矩阵收口的 provider
// 侧钉子(假客户端/纯投影断言,不起真进程;夹具形状逐字对 acp_adapter 静读实证,
// 证据 docs/reviews/2026-10-07/field-coverage-hermes.md):
//   - prompt 应答 usage → Run 返回的 *pb.Usage(此前整块静默丢);
//   - session_info_update(自动起名)→ STATUS 卡 + 台账 KNOWN_PASSTHROUGH;
//   - user_message_chunk(排队回显/历史回放)→ 不重投影 + 台账 KNOWN_PASSTHROUGH;
//   - 认不出的 session/update 变体 → 台账 UNKNOWN_NOTIFICATION + 限频上屏;
//   - tool_call 的 locations → 工具卡 JsonArgs;
//   - NewHermesProvider 必须把 hermesapp 的未知通知钩子接进台账(transport
//     default 分支旁路收口的 agent 半边)。
package agent

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/hermesapp"
	"github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/store"
)

// findHermesLedgerEntry 取台账里 agent=hermes 且 method 匹配的登记(没有则红)。
func findHermesLedgerEntry(t *testing.T, method string) runtime.EventEnvelope {
	t.Helper()
	for _, e := range sharedRuntimeBackend().UnknownEvents().List(runtime.ListFilter{AgentID: "hermes"}, 0) {
		if e.SourceMethod == method {
			return e
		}
	}
	t.Fatalf("hermes 的钩子必须把 method=%s 记进诊断台账(投影兜底断链)", method)
	return runtime.EventEnvelope{}
}

// TestHermesRunSurfacesTurnUsage prompt 应答的 usage(server.py _finish_turn 实测
// 形状)必须经 Run 返回值交 server 层,不再静默丢。
func TestHermesRunSurfacesTurnUsage(t *testing.T) {
	fake := newFakeHermesClient(true)
	fake.promptUsage = &hermesapp.TurnUsage{
		InputTokens: 1234, OutputTokens: 567, TotalTokens: 1801,
		ThoughtTokens: 89, CachedReadTokens: 12,
	}
	p := newHermesTestProvider(t, fake)

	_, got, err := p.Run(context.Background(), TurnRequest{
		Thread:      &store.Thread{ID: "t-usage"},
		InputBlocks: []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "带上用量跑"}}}},
		Workspace:   `C:\ws`,
	}, func(*pb.TurnServerMessage) {}, nil)
	if err != nil {
		t.Fatalf("轮次应成功: %v", err)
	}
	if got == nil {
		t.Fatal("prompt 应答的 usage 必须经 Run 返回(不得为 nil)")
	}
	if got.GetInputTokens() != 1234 || got.GetOutputTokens() != 567 {
		t.Fatalf("输入/输出 token 不符: %+v", got)
	}
	if got.GetReasoningTokens() != 89 || got.GetCacheReadTokens() != 12 {
		t.Fatalf("思考/缓存读 token 不符: %+v", got)
	}
}

// TestHermesUsageAbsentStaysNil 适配器没回 usage(或全零)时不得伪造用量。
func TestHermesUsageAbsentStaysNil(t *testing.T) {
	if u := hermesUsageFromOutcome(&hermesapp.PromptOutcome{StopReason: "end_turn"}); u != nil {
		t.Fatalf("无 usage 应返回 nil: %+v", u)
	}
	if u := hermesUsageFromOutcome(&hermesapp.PromptOutcome{Usage: &hermesapp.TurnUsage{}}); u != nil {
		t.Fatalf("全零 usage 应返回 nil(缺席语义): %+v", u)
	}
	if u := hermesUsageFromOutcome(nil); u != nil {
		t.Fatalf("nil outcome 应返回 nil: %+v", u)
	}
}

// TestHermesSessionInfoTitleCard 自动起名/压缩轮转的标题必须上屏(STATUS 卡),
// 并以 KNOWN_PASSTHROUGH 落台账 —— 不再既不投影也不留痕。
func TestHermesSessionInfoTitleCard(t *testing.T) {
	p := NewHermesProvider()
	var events []*pb.TurnServerMessage
	sink := func(ev *pb.TurnServerMessage) { events = append(events, ev) }
	raw := json.RawMessage(`{"sessionId":"s1","update":{"sessionUpdate":"session_info_update","title":"修复登录页","updatedAt":"2026-10-07T00:00:00Z"}}`)
	p.projectSessionUpdate(hermesapp.ParseSessionUpdate(raw), sink, &strings.Builder{},
		map[string]int{}, &sync.Mutex{}, &nativeNoticeRecorder{})

	if len(events) != 1 {
		t.Fatalf("session_info_update(带标题)应恰好一帧, got %d", len(events))
	}
	ce := events[0].GetCodexEvent()
	if ce == nil || ce.Code != "session_title" || ce.Title != "hermes · 会话命名" {
		t.Fatalf("应为会话命名 STATUS 卡: %+v", events[0].Msg)
	}
	if ce.Message != "修复登录页" {
		t.Fatalf("卡正文应带标题原文: %+v", ce)
	}
	if e := findHermesLedgerEntry(t, "session/update:session_info_update"); e.NormalizedKind != runtime.KindKnownPassthrough {
		t.Fatalf("session_info_update 应以 KNOWN_PASSTHROUGH 留痕: %s", e.NormalizedKind)
	}
}

// TestHermesUserChunkLedgerOnly 排队回显/历史回放:不重投影(手机已展示用户输入),
// 但必须以 KNOWN_PASSTHROUGH 落台账 —— 不再静默。
func TestHermesUserChunkLedgerOnly(t *testing.T) {
	p := NewHermesProvider()
	var events []*pb.TurnServerMessage
	sink := func(ev *pb.TurnServerMessage) { events = append(events, ev) }
	raw := json.RawMessage(`{"sessionId":"s1","update":{"sessionUpdate":"user_message_chunk","content":{"type":"text","text":"排队的话"}}}`)
	p.projectSessionUpdate(hermesapp.ParseSessionUpdate(raw), sink, &strings.Builder{},
		map[string]int{}, &sync.Mutex{}, &nativeNoticeRecorder{})

	if len(events) != 0 {
		t.Fatalf("user_message_chunk 不得重投影到手机: %+v", events)
	}
	if e := findHermesLedgerEntry(t, "session/update:user_message_chunk"); e.NormalizedKind != runtime.KindKnownPassthrough {
		t.Fatalf("user_message_chunk 应以 KNOWN_PASSTHROUGH 留痕: %s", e.NormalizedKind)
	}
}

// TestHermesUnknownVariantLedgerAndCard 认不出的变体:UNKNOWN_NOTIFICATION 台账
// + 限频上屏(recordAs 每方法至多 2 张卡)—— 不再静默消失。
func TestHermesUnknownVariantLedgerAndCard(t *testing.T) {
	p := NewHermesProvider()
	var events []*pb.TurnServerMessage
	sink := func(ev *pb.TurnServerMessage) { events = append(events, ev) }
	raw := json.RawMessage(`{"sessionId":"s1","update":{"sessionUpdate":"brand_new_variant","foo":1}}`)
	p.projectSessionUpdate(hermesapp.ParseSessionUpdate(raw), sink, &strings.Builder{},
		map[string]int{}, &sync.Mutex{}, &nativeNoticeRecorder{})

	if len(events) != 1 {
		t.Fatalf("未知变体应限频上屏一张卡, got %d", len(events))
	}
	if ce := events[0].GetCodexEvent(); ce == nil || !strings.Contains(ce.Title, "session/update:brand_new_variant") {
		t.Fatalf("卡应带变体原名: %+v", events[0].Msg)
	}
	if e := findHermesLedgerEntry(t, "session/update:brand_new_variant"); e.NormalizedKind != runtime.KindUnknownNotification {
		t.Fatalf("真未知变体必须如实记 UNKNOWN_NOTIFICATION: %s", e.NormalizedKind)
	}
}

// TestHermesKnownButEmptyVariantLedgerOnly 已知变体的空转(空文本 chunk)按
// 已知档留痕,不冒充未知(dshNoticeKind 同口径)。
func TestHermesKnownButEmptyVariantLedgerOnly(t *testing.T) {
	p := NewHermesProvider()
	var events []*pb.TurnServerMessage
	sink := func(ev *pb.TurnServerMessage) { events = append(events, ev) }
	raw := json.RawMessage(`{"sessionId":"s1","update":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":""}}}`)
	p.projectSessionUpdate(hermesapp.ParseSessionUpdate(raw), sink, &strings.Builder{},
		map[string]int{}, &sync.Mutex{}, &nativeNoticeRecorder{})

	if len(events) != 0 {
		t.Fatalf("空文本 chunk 不得推帧: %+v", events)
	}
	if e := findHermesLedgerEntry(t, "session/update:agent_message_chunk"); e.NormalizedKind != runtime.KindKnownProjected {
		t.Fatalf("已知变体的空转应记 KNOWN_PROJECTED: %s", e.NormalizedKind)
	}
}

// TestHermesToolLocationsInToolCard 工具卡的文件位置(工具卡维度此前缺文件路径行)。
func TestHermesToolLocationsInToolCard(t *testing.T) {
	p := NewHermesProvider()
	var events []*pb.TurnServerMessage
	sink := func(ev *pb.TurnServerMessage) { events = append(events, ev) }
	raw := json.RawMessage(`{"sessionId":"s1","update":{"sessionUpdate":"tool_call","toolCallId":"tc-loc",` +
		`"title":"read_file","kind":"read","status":"pending","locations":[{"path":"src/main.py","line":3}]}}`)
	p.projectSessionUpdate(hermesapp.ParseSessionUpdate(raw), sink, &strings.Builder{},
		map[string]int{}, &sync.Mutex{}, &nativeNoticeRecorder{})

	if len(events) != 1 || events[0].GetToolCall() == nil {
		t.Fatalf("工具帧应照常投影: %+v", events)
	}
	args := events[0].GetToolCall().Call.JsonArgs
	if !strings.Contains(args, `"locations"`) || !strings.Contains(args, `"path":"src/main.py"`) || !strings.Contains(args, `"line":3`) {
		t.Fatalf("locations 应并入 JsonArgs: %s", args)
	}
}

// TestHermesUnknownTransportHookWired agent 半边接线:NewHermesProvider 必须把
// hermesapp 的未知通知钩子接进台账(transport default 分支 → 台账)。钩子体演练
// 即验真:调它必须产出 agent=hermes 的 UNKNOWN_NOTIFICATION 登记。
func TestHermesUnknownTransportHookWired(t *testing.T) {
	NewHermesProvider() // 幂等(sync.Once);重复创建不得重挂/抖动
	hook := hermesapp.UnknownNotificationHook()
	if hook == nil {
		t.Fatal("NewHermesProvider 应注入 hermesapp 未知通知钩子(transport default 分支旁路未收口)")
	}
	const probe = "hookWiredProbe/notify"
	hook(probe, json.RawMessage(`{"x":1}`))
	e := findHermesLedgerEntry(t, probe)
	if e.NormalizedKind != runtime.KindUnknownNotification {
		t.Fatalf("未知通知应记 UNKNOWN_NOTIFICATION: %s", e.NormalizedKind)
	}
	if e.SourceAgent != "hermes" {
		t.Fatalf("台账归属应为 hermes: %s", e.SourceAgent)
	}
}

// TestHermesNoticeKindClassifier 分档器对表:五件套 PROJECTED、四类具名
// PASSTHROUGH、其余 UNKNOWN。
func TestHermesNoticeKindClassifier(t *testing.T) {
	projected := []string{"agent_message_chunk", "agent_thought_chunk", "tool_call",
		"tool_call_update", "plan", "usage_update"}
	for _, v := range projected {
		if got := hermesNoticeKind(v); got != runtime.KindKnownProjected {
			t.Errorf("%s 应为 KNOWN_PROJECTED, got %s", v, got)
		}
	}
	passthrough := []string{"user_message_chunk", "current_mode_update",
		"available_commands_update", "session_info_update"}
	for _, v := range passthrough {
		if got := hermesNoticeKind(v); got != runtime.KindKnownPassthrough {
			t.Errorf("%s 应为 KNOWN_PASSTHROUGH, got %s", v, got)
		}
	}
	for _, v := range []string{"brand_new_variant", ""} {
		if got := hermesNoticeKind(v); got != runtime.KindUnknownNotification {
			t.Errorf("%q 应为 UNKNOWN_NOTIFICATION, got %s", v, got)
		}
	}
}
