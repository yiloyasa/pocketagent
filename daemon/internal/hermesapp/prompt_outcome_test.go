// prompt_outcome_test.go —— 2026-10-07 覆盖矩阵收口的解析层钉子(夹具形状逐字
// 对 acp_adapter 静读实证,证据 docs/reviews/2026-10-07/field-coverage-hermes.md;
// 本文件只做纯函数断言,不起子进程、不跑网络):
//   - PromptOutcome/TurnUsage:session/prompt 应答的 usage 形状(server.py
//     _finish_turn:Usage(input_tokens, output_tokens, total_tokens,
//     thought_tokens, cached_read_tokens));此前只收 stopReason,用量整块静默丢;
//   - tool_call 的 locations(ACP ToolCallLocation{path,line},tools.py
//     extract_locations 把 args.path/offset 上报为文件位置);
//   - StreamEvent.Variant(变体原名,台账登记名 "session/update:<variant>" 的事实源);
//   - readLoop 的非 session/update 通知 → UnknownNotificationHook 上抛
//     (transport 旁路收口:此前 default 分支静默丢)。
package hermesapp

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

// TestPromptOutcomeUnmarshal prompt 应答全形状(server.py _finish_turn 实测:
// usage 含 input/output/total/thought/cached_read 五字段)。
func TestPromptOutcomeUnmarshal(t *testing.T) {
	raw := `{"stopReason":"end_turn","usage":{"input_tokens":1234,"output_tokens":567,
		"total_tokens":1801,"thought_tokens":89,"cached_read_tokens":12}}`
	out := &PromptOutcome{}
	if err := json.Unmarshal([]byte(raw), out); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if out.StopReason != "end_turn" {
		t.Fatalf("stopReason = %q", out.StopReason)
	}
	if out.Usage == nil {
		t.Fatal("usage 应在场(缺席才为 nil)")
	}
	u := out.Usage
	if u.InputTokens != 1234 || u.OutputTokens != 567 || u.TotalTokens != 1801 ||
		u.ThoughtTokens != 89 || u.CachedReadTokens != 12 {
		t.Fatalf("usage 面不符: %+v", u)
	}
}

// TestPromptOutcomeWithoutUsage 无 usage 的应答(适配器没拿到用量时不伪造)。
func TestPromptOutcomeWithoutUsage(t *testing.T) {
	out := &PromptOutcome{}
	if err := json.Unmarshal([]byte(`{"stopReason":"cancelled"}`), out); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if out.StopReason != "cancelled" || out.Usage != nil {
		t.Fatalf("无 usage 应答应 Usage=nil: %+v", out)
	}
}

// TestParseSessionUpdateLocations 工具帧的文件位置(tools.py extract_locations:
// read_file/write_file 的 args.path(与 offset/line)→ locations 数组)。
func TestParseSessionUpdateLocations(t *testing.T) {
	raw := env("tool_call", `"toolCallId":"tc-loc","title":"read_file","kind":"read","status":"pending",`+
		`"locations":[{"path":"src/main.py","line":3},{"path":"b.txt"}]`)
	ev := ParseSessionUpdate(raw)
	if ev.ToolCall == nil {
		t.Fatalf("应为 tool_call: %+v", ev)
	}
	if len(ev.ToolCall.Locations) != 2 {
		t.Fatalf("locations 条数不符: %+v", ev.ToolCall.Locations)
	}
	if ev.ToolCall.Locations[0].Path != "src/main.py" || ev.ToolCall.Locations[0].Line != 3 {
		t.Fatalf("locations[0] 不符: %+v", ev.ToolCall.Locations[0])
	}
	if ev.ToolCall.Locations[1].Path != "b.txt" || ev.ToolCall.Locations[1].Line != 0 {
		t.Fatalf("无 line 的位置应记 0: %+v", ev.ToolCall.Locations[1])
	}
}

// TestStreamEventVariantField 变体原名随事件上抛:已知变体与未知变体都要带
// Variant(台账登记名的事实源)。
func TestStreamEventVariantField(t *testing.T) {
	if ev := ParseSessionUpdate(env("usage_update", `"size":1,"used":2`)); ev.Variant != "usage_update" {
		t.Fatalf("已知变体应带原名, got %q", ev.Variant)
	}
	// env() 拼的是 update 内层的 "key":value 片段,载荷必须是片段不是裸对象。
	if ev := ParseSessionUpdate(env("brand_new_variant", `"x":1`)); ev.Variant != "brand_new_variant" {
		t.Fatalf("未知变体也应带原名, got %q", ev.Variant)
	}
}

// TestUnknownNotificationHookRedirect 传输层 default 分支旁路收口:非
// session/update 的通知必须经 UnknownNotificationHook 上抛(method+params
// 原样),不再静默丢。
func TestUnknownNotificationHookRedirect(t *testing.T) {
	type hookCall struct {
		method string
		params json.RawMessage
	}
	calls := make(chan hookCall, 8)
	SetUnknownNotificationHook(func(method string, params json.RawMessage) {
		select {
		case calls <- hookCall{method, params}:
		default:
		}
	})
	t.Cleanup(func() { SetUnknownNotificationHook(nil) })

	_, sOut, _ := pipeClient(t)
	fmt.Fprintf(sOut, "%s\n", `{"jsonrpc":"2.0","method":"brandNew/notify","params":{"k":1}}`)
	select {
	case got := <-calls:
		if got.method != "brandNew/notify" || string(got.params) != `{"k":1}` {
			t.Fatalf("钩子应收原样帧: %s %s", got.method, got.params)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("非 session/update 通知未被上抛(default 分支仍在静默丢)")
	}
}
