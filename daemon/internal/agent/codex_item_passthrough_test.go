// codex_item_passthrough_test.go —— item/started|completed 内未知 item 类型
// 接主线 A 透传(T-FU)的用例证明:
//  1. 分类:已知五类(contextCompaction/commandExecution/agentMessage/
//     subAgentActivity/collabAgentToolCall)各有专案分支,不进透传;
//     其余(userMessage/reasoning 等)按不透明数据透传。
//  2. 路由:maybeRecordOpaqueItem 与 item/started|completed 两个分支调用的是
//     同一个函数 —— 原样包装(notices.record)、未知字段保真、脱敏自动生效、
//     限频键 = 方法+类型;已知类型/异线程/坏 JSON 不透传(既有行为不动)。
//  3. RawJson 保真:子代理事件透传 item 原文(n.Item),重编组会掉的未知字段不再丢。
package agent

import (
	"encoding/json"
	"strings"
	"testing"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

func TestOpaqueItemTypesClassification(t *testing.T) {
	for _, k := range []string{"contextCompaction", "commandExecution", "agentMessage", "subAgentActivity", "collabAgentToolCall"} {
		if isOpaqueItemType(k) {
			t.Fatalf("已知类型 %s 有专案分支,不应算不透明", k)
		}
	}
	for _, u := range []string{"userMessage", "reasoning", "webSearch", "todoList", ""} {
		if !isOpaqueItemType(u) {
			t.Fatalf("未知类型 %q 应走主线 A 透传", u)
		}
	}
}

func TestUnknownItemRoutesThroughNotices(t *testing.T) {
	var notices nativeNoticeRecorder
	var got []*pb.TurnServerMessage
	sink := func(m *pb.TurnServerMessage) { got = append(got, m) }
	// 带未知字段 brandNewField + 敏感字段 apiKey:验证"原样包装 + 脱敏自动生效"。
	params := json.RawMessage(`{"threadId":"th1","item":{"type":"userMessage","id":"u1","text":"hi","brandNewField":{"k":"v"},"apiKey":"sk-secret"}}`)

	if !maybeRecordOpaqueItem("item/started", params, "th1", &notices, sink) {
		t.Fatal("userMessage 应命中透传路由")
	}
	if len(got) != 1 {
		t.Fatalf("应透传 1 条,得到 %d", len(got))
	}
	ev := got[0].GetCodexEvent()
	if ev == nil || ev.Kind != pb.CodexEvent_STATUS || ev.Code != "native_notification" {
		t.Fatalf("应为主线 A 形态的 STATUS/native_notification 事件: %v", ev)
	}
	if ev.Title != "codex · item/started:userMessage" {
		t.Fatalf("标题应为 方法:类型(限频键同源): %s", ev.Title)
	}
	if ev.ThreadId != "" {
		t.Fatalf("透传是纯包装,不伪造 ThreadId: %q", ev.ThreadId)
	}
	if !strings.Contains(ev.RawJson, `"brandNewField":{"k":"v"}`) {
		t.Fatalf("未知字段应保真透传: %s", ev.RawJson)
	}
	if !strings.Contains(ev.RawJson, `"apiKey":"[redacted]"`) {
		t.Fatalf("脱敏应自动生效: %s", ev.RawJson)
	}

	// 限频(主线 A 语义,每键 2 条):第 3 条起不再透传。
	maybeRecordOpaqueItem("item/started", params, "th1", &notices, sink)
	maybeRecordOpaqueItem("item/started", params, "th1", &notices, sink)
	if len(got) != 2 {
		t.Fatalf("同键限频后应止步 2 条: %d", len(got))
	}
	// 不同方法/类型是不同键,互不挤占。
	if !maybeRecordOpaqueItem("item/completed", params, "th1", &notices, sink) {
		t.Fatal("item/completed 形状的同报文应按新键放行")
	}
	if len(got) != 3 {
		t.Fatalf("新键应放行: %d", len(got))
	}

	// ---- 边界:不透传的都要保持既有行为 ----
	// 已知类型(agentMessage 在 item/completed 有专案分支)不进透传。
	if maybeRecordOpaqueItem("item/completed", json.RawMessage(`{"threadId":"th1","item":{"type":"agentMessage","text":"x"}}`), "th1", &notices, sink) {
		t.Fatal("agentMessage 不应进透传(既有处理不动)")
	}
	// 异线程不透传。
	if maybeRecordOpaqueItem("item/started", json.RawMessage(`{"threadId":"other","item":{"type":"userMessage"}}`), "th1", &notices, sink) {
		t.Fatal("异线程不应透传")
	}
	// 坏 JSON 不透传。
	if maybeRecordOpaqueItem("item/started", json.RawMessage(`{not json`), "th1", &notices, sink) {
		t.Fatal("坏 JSON 不应透传")
	}
	if len(got) != 3 {
		t.Fatalf("边界之外不得新增透传: %d", len(got))
	}
}

func TestSubagentRawJsonKeepsUnknownFields(t *testing.T) {
	// 保真一行改的红线验证:RawJson 来自 n.Item 原文(经 redactNativePayload),
	// 结构体没接住的未知字段 mustKeep 不再被重编组丢掉。
	raw := notifRaw(t, "th1", `{"type":"subAgentActivity","id":"it1","agentPath":"review","kind":"started","mustKeep":"future-proof"}`)
	ev, _ := subagentEvent(raw, "th1")
	if ev == nil {
		t.Fatal("应解析出事件")
	}
	if !strings.Contains(ev.RawJson, `"mustKeep":"future-proof"`) {
		t.Fatalf("未知字段应随 item 原文保真透传: %s", ev.RawJson)
	}
	if strings.Contains(ev.RawJson, `"threadId"`) {
		t.Fatalf("仍不应含外层 envelope: %s", ev.RawJson)
	}

	raw2 := notifRaw(t, "th1", `{"type":"collabAgentToolCall","id":"cc1","tool":"wait","status":"completed","agentsStates":{},"mustKeep2":42}`)
	ev2, _ := subagentEvent(raw2, "th1")
	if ev2 == nil {
		t.Fatal("应解析出事件")
	}
	if !strings.Contains(ev2.RawJson, `"mustKeep2":42`) {
		t.Fatalf("collab 事件同样保真未知字段: %s", ev2.RawJson)
	}
}
