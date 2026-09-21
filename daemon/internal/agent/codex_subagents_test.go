// codex_subagents_test.go —— 子代理 item 解析的回归用例。
// 覆盖:完整形状 / 缺字段 / 未知 kind / agentsStates 空 map / 限频闸门,
// 以及"非子代理 item 必须返回 nil(继续走既有兜底)"的边界。
package agent

import (
	"encoding/json"
	"strings"
	"testing"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

func notifRaw(t *testing.T, threadID, itemJSON string) json.RawMessage {
	t.Helper()
	return json.RawMessage(`{"threadId":"` + threadID + `","item":` + itemJSON + `}`)
}

// ---- subAgentActivity ----

func TestSubagentEventActivityFullShape(t *testing.T) {
	raw := notifRaw(t, "th1", `{"type":"subAgentActivity","id":"it1","agentPath":"review","agentThreadId":"sub-thread-abcdef","kind":"started"}`)
	ev, key := subagentEvent(raw, "th1")
	if ev == nil {
		t.Fatal("完整形状应解析出事件")
	}
	if ev.Kind != pb.CodexEvent_STATUS || ev.Code != "subagent_activity" {
		t.Fatalf("Kind/Code 不符: %v / %s", ev.Kind, ev.Code)
	}
	if !strings.Contains(ev.Title, "review") {
		t.Fatalf("标题应含 agentPath: %s", ev.Title)
	}
	if !strings.Contains(ev.Message, "已启动") {
		t.Fatalf("正文应含 kind 人话: %s", ev.Message)
	}
	if ev.ThreadId != "th1" {
		t.Fatalf("ThreadId 应为当前线程: %s", ev.ThreadId)
	}
	if !strings.Contains(ev.RawJson, `"agentPath":"review"`) || strings.Contains(ev.RawJson, `"threadId":`) {
		t.Fatalf("RawJson 应为脱敏透传的 item 原文(不含外层 envelope): %s", ev.RawJson)
	}
	if key != "subAgentActivity:started" {
		t.Fatalf("限频键不符: %s", key)
	}
}

func TestSubagentEventActivityMissingFields(t *testing.T) {
	// schema 里四个字段都是 required,但"未知≠出错":缺了也要能翻,不能崩。
	raw := notifRaw(t, "th1", `{"type":"subAgentActivity","id":"it2"}`)
	ev, key := subagentEvent(raw, "th1")
	if ev == nil {
		t.Fatal("缺字段也应解析出事件")
	}
	if ev.Title != "子代理" {
		t.Fatalf("无 agentPath 时标题兜底: %s", ev.Title)
	}
	if ev.Message == "" {
		t.Fatal("无 kind 时正文应有兜底人话")
	}
	if key != "subAgentActivity:" {
		t.Fatalf("缺 kind 的限频键: %q", key)
	}
}

func TestSubagentEventActivityUnknownKind(t *testing.T) {
	raw := notifRaw(t, "th1", `{"type":"subAgentActivity","id":"it3","agentPath":"compact","agentThreadId":"s1","kind":"teleported"}`)
	ev, _ := subagentEvent(raw, "th1")
	if ev == nil {
		t.Fatal("未知 kind 不是错误,仍应出事件")
	}
	if !strings.Contains(ev.Message, "teleported") {
		t.Fatalf("未知 kind 应原样透传: %s", ev.Message)
	}
}

// ---- collabAgentToolCall ----

func TestSubagentEventCollabFullShape(t *testing.T) {
	raw := notifRaw(t, "th1", `{"type":"collabAgentToolCall","id":"cc1","tool":"spawnAgent","status":"inProgress",
		"senderThreadId":"th1","receiverThreadIds":["s1","s2"],
		"agentsStates":{"s1":{"status":"running"},"s2":{"status":"completed","message":"done"}},
		"prompt":"审查这次命令","model":"gpt-5","reasoningEffort":"high"}`)
	ev, key := subagentEvent(raw, "th1")
	if ev == nil {
		t.Fatal("完整形状应解析出事件")
	}
	if ev.Kind != pb.CodexEvent_STATUS || ev.Code != "collab_agent_tool_call" {
		t.Fatalf("Kind/Code 不符: %v / %s", ev.Kind, ev.Code)
	}
	if !strings.Contains(ev.Title, "spawnAgent") {
		t.Fatalf("标题应含 tool 名: %s", ev.Title)
	}
	if !strings.Contains(ev.Message, "2 个子代理") || !strings.Contains(ev.Message, "1 运行") || !strings.Contains(ev.Message, "1 完成") {
		t.Fatalf("正文应含 agentsStates 概览: %s", ev.Message)
	}
	if !strings.Contains(ev.Message, "进行中") {
		t.Fatalf("正文应含状态人话: %s", ev.Message)
	}
	if !strings.Contains(ev.RawJson, `"tool":"spawnAgent"`) {
		t.Fatalf("RawJson 应透传 item 原文: %s", ev.RawJson)
	}
	if key != "collabAgentToolCall:spawnAgent:inProgress" {
		t.Fatalf("限频键不符: %s", key)
	}
}

func TestSubagentEventCollabEmptyStatesMap(t *testing.T) {
	raw := notifRaw(t, "th1", `{"type":"collabAgentToolCall","id":"cc2","tool":"wait","status":"completed","agentsStates":{}}`)
	ev, _ := subagentEvent(raw, "th1")
	if ev == nil {
		t.Fatal("空 agentsStates 也应出事件")
	}
	if strings.Contains(ev.Message, "个子代理") {
		t.Fatalf("空 map 不应有概览: %s", ev.Message)
	}
	if !strings.Contains(ev.Message, "已完成") {
		t.Fatalf("状态人话缺失: %s", ev.Message)
	}
}

func TestSubagentEventCollabUnknownToolAndStatus(t *testing.T) {
	raw := notifRaw(t, "th1", `{"type":"collabAgentToolCall","id":"cc3","tool":"hypnotize","status":"warping","agentsStates":{"s1":{"status":"dreaming"}}}`)
	ev, _ := subagentEvent(raw, "th1")
	if ev == nil {
		t.Fatal("未知 tool/status 不是错误,仍应出事件")
	}
	if !strings.Contains(ev.Title, "hypnotize") || !strings.Contains(ev.Message, "warping") || !strings.Contains(ev.Message, "dreaming") {
		t.Fatalf("未知枚举值应原样透传: %s / %s", ev.Title, ev.Message)
	}
}

// ---- 边界:不属于本文件管的,一律 (nil, "") 交回既有分支 ----

func TestSubagentEventIgnoresOtherTypes(t *testing.T) {
	for _, item := range []string{
		`{"type":"agentMessage","id":"a","text":"hi"}`,
		`{"type":"commandExecution","id":"c","command":["ls"],"status":"completed"}`,
		`{}`,
	} {
		if ev, key := subagentEvent(notifRaw(t, "th1", item), "th1"); ev != nil || key != "" {
			t.Fatalf("非子代理 item 必须返回 nil: %s -> %v", item, ev)
		}
	}
}

func TestSubagentEventWrongThread(t *testing.T) {
	raw := notifRaw(t, "other-thread", `{"type":"subAgentActivity","id":"x","kind":"started"}`)
	if ev, _ := subagentEvent(raw, "th1"); ev != nil {
		t.Fatal("别的线程的 item 不归本回合处理")
	}
}

func TestSubagentEventBadJSON(t *testing.T) {
	if ev, _ := subagentEvent(json.RawMessage(`{not json`), "th1"); ev != nil {
		t.Fatal("坏 JSON 应安静返回 nil,交给 default 兜底")
	}
}

func TestSubagentEventNilCountsAsEmptyOverview(t *testing.T) {
	// "agentsStates":null 与缺字段等价:map 为 nil,概览为空,不报错。
	raw := notifRaw(t, "th1", `{"type":"collabAgentToolCall","id":"cc4","tool":"sendInput","status":"failed","agentsStates":null}`)
	ev, _ := subagentEvent(raw, "th1")
	if ev == nil || strings.Contains(ev.Message, "个子代理") {
		t.Fatalf("null agentsStates 应视为空概览: %v %s", ev, ev.Message)
	}
}

// ---- 限频闸门(语义对齐 nativeNoticeRecorder:每键 2 条、64 键封顶)----

func TestSubagentNoticeGate(t *testing.T) {
	var g subagentNoticeGate
	for i := 0; i < 2; i++ {
		if !g.allow("k1") {
			t.Fatalf("前 2 条应放行(第 %d 条)", i+1)
		}
	}
	if g.allow("k1") {
		t.Fatal("同键第 3 条应被限频")
	}
	if !g.allow("k2") {
		t.Fatal("新键不受其它键影响")
	}
	if !g.allow("") {
		t.Fatal("空键(不该出现)放行,不当错误处理")
	}
	var g2 subagentNoticeGate
	for i := 0; i < 64; i++ {
		if !g2.allow(strings.Repeat("k", i+1)) {
			t.Fatalf("第 %d 个键应放行", i+1)
		}
	}
	if g2.allow("overflow") {
		t.Fatal("超过 64 键后应全部限频")
	}
}

// ---- 概览函数直接验证(含未知状态的兜底位置)----

func TestCollabAgentStateOverview(t *testing.T) {
	total, overview := collabAgentStateOverview(map[string]collabAgentStateItem{
		"a": {Status: "completed"},
		"b": {Status: "running"},
		"c": {Status: "running"},
		"d": {Status: "somethingNew"},
	})
	if total != 4 || !strings.Contains(overview, "4 个子代理") || !strings.Contains(overview, "2 运行") ||
		!strings.Contains(overview, "1 完成") || !strings.Contains(overview, `1 未知(somethingNew)`) {
		t.Fatalf("概览不符: total=%d %s", total, overview)
	}
	if n, s := collabAgentStateOverview(nil); n != 0 || s != "" {
		t.Fatalf("nil map 应为空概览: %d %q", n, s)
	}
}
