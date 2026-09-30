// unknown_backlog_20260930_test.go —— 2026-09-30 未知台账积压收编批的分类器钉子。
//
// 【背景】活跃台账 200 条滚动窗实测:UNKNOWN 积压 = dsh 裸 "session/update" 13 条
// (全是认识的变体,全局钩子没拆壳)+ codex "response" 迟到错误应答 24 条(两个
// 可实证消息家族)。本文件钉住:①dshNoticeKind 补档的五类变体;②dshGlobalNoticeKind
// 拆壳分发;③codexLateResponseLedger 的家族归类与不猜兜底。
package agent

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/pocketagent/daemon/internal/runtime"
)

// 台账实测的 13 条 dsh UNKNOWN,变体全部可实证为"认识"的类别:
// agent_thought_chunk ×3 / usage_update ×3 / tool_call ×2 / agent_message_chunk ×2
// / 空壳 update ×2(空壳维持 UNKNOWN,见下)。补档依据见 dshNoticeKind 注释。
func TestDshNoticeKindBacklogVariants20260930(t *testing.T) {
	cases := []struct {
		method string
		want   string
		why    string
	}{
		{"session/update:usage_update", runtime.KindKnownProjected,
			"轮次内已投影成 Usage 帧(dshUsageUpdateFromUsedSize),与 context_usage 同档"},
		{"session/update:agent_message_chunk", runtime.KindKnownProjected,
			"轮次内已投影成 TextDelta"},
		{"session/update:agent_thought_chunk", runtime.KindKnownProjected,
			"轮次内已投影成 ThinkingDelta(HideReasoning 过滤属投影内部行为)"},
		{"session/update:user_message_chunk", runtime.KindKnownPassthrough,
			"轮次内刻意不转发(手机端已展示用户输入):已知但无投影"},
		{"session/update:config_option_update", runtime.KindKnownPassthrough,
			"轮次内刷新原生目录缓存、不发协议帧:已知但无投影"},
		{"session/update:brand_new_variant", runtime.KindUnknownNotification,
			"上游新加的变体 = 真没见过,不得被本批收编误伤"},
	}
	for _, c := range cases {
		if got := dshNoticeKind(c.method); got != c.want {
			t.Errorf("dshNoticeKind(%q) = %q, want %q(%s)", c.method, got, c.want, c.why)
		}
	}
}

// 全局钩子拆壳:session/update 信封先取 update.sessionUpdate 再按变体名复用
// dshNoticeKind,登记名与轮次内 recordAs 同形("session/update:<kind>")。
func TestDshGlobalNoticeKindSplitsSessionUpdate20260930(t *testing.T) {
	cases := []struct {
		name   string
		method string
		params string
		want   string
	}{
		{"认识的变体", "session/update", `{"sessionId":"s","update":{"sessionUpdate":"usage_update","used":1,"size":2}}`, runtime.KindKnownProjected},
		{"工具调用回声", "session/update", `{"sessionId":"s","update":{"sessionUpdate":"tool_call","toolCallId":"c1","title":"read","kind":"other","status":"in_progress","rawInput":{}}}`, runtime.KindKnownProjected},
		{"空壳 update 如实未知(台账实测 {'update':{}})", "session/update", `{"sessionId":"s","update":{}}`, runtime.KindUnknownNotification},
		{"信封解不动如实未知", "session/update", `not-json`, runtime.KindUnknownNotification},
		{"update 体缺失如实未知", "session/update", `{"sessionId":"s"}`, runtime.KindUnknownNotification},
		{"非 session/update 走平键分档", "some/unexpected/method", `{}`, runtime.KindUnknownNotification},
	}
	for _, c := range cases {
		if got := dshGlobalNoticeKind(c.method, json.RawMessage(c.params)); got != c.want {
			t.Errorf("%s: dshGlobalNoticeKind(%q, %s) = %q, want %q", c.name, c.method, c.params, got, c.want)
		}
	}
}

// codex 迟到错误应答:两条可实证消息家族 → 带请求方法后缀的 KNOWN_PASSTHROUGH;
// 其余(含 result 体、解不动的 body)一律裸 "response" + UNKNOWN_RESPONSE,不猜。
func TestCodexLateResponseLedger20260930(t *testing.T) {
	cases := []struct {
		name       string
		raw        string
		wantMethod string
		wantKind   string
	}{
		{"限额拉取失败(台账实测原文)", `{"error":{"code":-32603,"message":"failed to fetch codex rate limits: error sending request for url (https://chatgpt.com/backend-api/wham/usage)"},"id":306,"jsonrpc":""}`,
			"response:account/rateLimits/read", runtime.KindKnownPassthrough},
		{"用量画像超时(台账实测原文)", `{"error":{"code":-32603,"message":"token usage profile fetch timed out"},"id":319,"jsonrpc":""}`,
			"response:account/usage/read", runtime.KindKnownPassthrough},
		{"从未见过的错误消息", `{"error":{"code":-32603,"message":"something new"},"id":1,"jsonrpc":""}`,
			"response", runtime.KindUnknownResponse},
		{"只有 result 的响应", `{"result":{},"id":2,"jsonrpc":""}`,
			"response", runtime.KindUnknownResponse},
		{"非 JSON", `garbage`, "response", runtime.KindUnknownResponse},
	}
	for _, c := range cases {
		gotMethod, gotKind := codexLateResponseLedger(json.RawMessage(c.raw))
		if gotMethod != c.wantMethod || gotKind != c.wantKind {
			t.Errorf("%s: codexLateResponseLedger = (%q, %q), want (%q, %q)", c.name, gotMethod, gotKind, c.wantMethod, c.wantKind)
		}
	}
}

// 入账集成:迟到应答按分类落台账(方法带后缀、kind=PASSTHROUGH、载荷仍脱敏);
// 默认路径(recordUnknownResponse)维持 UNKNOWN_RESPONSE 不回退。
func TestRecordUnknownResponseAsClassifiedLedger20260930(t *testing.T) {
	raw := json.RawMessage(`{"error":{"code":-32603,"message":"token usage profile fetch timed out"},"id":319,"jsonrpc":""}`)
	method, kind := codexLateResponseLedger(raw)
	recordUnknownResponseAs(kind, "codex", method, "319", raw)

	entries := sharedRuntimeBackend().UnknownEvents().List(runtime.ListFilter{AgentID: "codex"}, 0)
	var found *runtime.EventEnvelope
	for i := range entries {
		if entries[i].CorrelationID == "319" && entries[i].SourceMethod == "response:account/usage/read" {
			found = &entries[i]
			break
		}
	}
	if found == nil {
		t.Fatal("分类后的迟到应答必须以 response:account/usage/read 入账")
	}
	if found.NormalizedKind != runtime.KindKnownPassthrough {
		t.Errorf("已知家族必须记 KNOWN_PASSTHROUGH,得到 %q", found.NormalizedKind)
	}
	if !strings.Contains(found.RawPayload, "-32603") {
		t.Errorf("载荷应保留错误原文(脱敏后),得到 %q", found.RawPayload)
	}

	recordUnknownResponse("codex", "999", json.RawMessage(`{"result":{},"id":999}`))
	entries = sharedRuntimeBackend().UnknownEvents().List(
		runtime.ListFilter{AgentID: "codex", Kind: runtime.KindUnknownResponse}, 0)
	var fallback *runtime.EventEnvelope
	for i := range entries {
		if entries[i].CorrelationID == "999" {
			fallback = &entries[i]
			break
		}
	}
	if fallback == nil || fallback.SourceMethod != "response" {
		t.Fatalf("默认路径必须仍以裸 response + UNKNOWN_RESPONSE 入账,得到 %+v", fallback)
	}
}
