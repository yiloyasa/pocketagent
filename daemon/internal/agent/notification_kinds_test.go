package agent

// notification_kinds_test.go —— 各 provider notice 的「已知/未知」分档(方案 §4.6 词表 / §9.4 验收)。
//
// 【为什么单列一组】unknown-events 台账若把我们已经认识、只是没有正式投影的事件也记成
// UNKNOWN_NOTIFICATION,诊断页就被已知噪音填满 —— 上游升级后真发来怪东西时,它会淹在
// 一堆「其实我们认识」的条目里看不见。codex 的 item 路径已在上一步分档,本组覆盖
// **其余 provider 的通知路径**(dsh / pi / zcode / codex 的 method 兜底)。
//
// 【判定依据是「daemon 对这个**类别**做了什么」,不是名字眼熟不眼熟】每条用例都写明依据 ——
// 免得后来者按"看着像"就把档改了。

import (
	"encoding/json"
	"testing"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/runtime"
)

func TestDshNoticeKindClassification(t *testing.T) {
	cases := []struct {
		method string
		want   string
		why    string
	}{
		{"session/update:context_usage", runtime.KindKnownProjected,
			"正常情形已投影成 Usage;走到留痕说明认不出 token 字段"},
		{"session/update:tool_call", runtime.KindKnownProjected,
			"正常情形已投影成 ToolCall"},
		{"session/update:tool_call_update:some_new_status", runtime.KindKnownProjected,
			"status 不认识不改变「这是我们认识的类别」"},
		{"session/update:plan", runtime.KindKnownPassthrough,
			"只透传进诊断视图,没有正式事件"},
		{"session/update:available_commands_update", runtime.KindKnownPassthrough,
			"同上"},
		{"session/update:brand_new_variant", runtime.KindUnknownNotification,
			"上游新加的变体 = 真没见过"},
		{"some/unexpected/method", runtime.KindUnknownNotification,
			"dsh 只认 session/update,此路径无任何处理分支"},
		{"", runtime.KindUnknownNotification,
			"拿不到 method 时如实记未知,不替它编一个已知身份"},
	}
	for _, c := range cases {
		if got := dshNoticeKind(c.method); got != c.want {
			t.Errorf("dshNoticeKind(%q) = %q, want %q(%s)", c.method, got, c.want, c.why)
		}
	}
}

func TestZcodeNoticeKindClassification(t *testing.T) {
	// 这些 method 在记录前都已经 emitRuntimeSelection —— 即已投影成协议内的 Started 帧。
	for _, m := range []string{"state.updated", "session.updated", "settings.updated", "session.modelUpdated"} {
		if got := zcodeNoticeKind(m); got != runtime.KindKnownProjected {
			t.Errorf("zcodeNoticeKind(%q) = %q, want PROJECTED(记录前已 emitRuntimeSelection)", m, got)
		}
	}
	for _, m := range []string{"turn.started", "model.streaming", "brand.new.event", ""} {
		if got := zcodeNoticeKind(m); got != runtime.KindUnknownNotification {
			t.Errorf("zcodeNoticeKind(%q) = %q, want UNKNOWN", m, got)
		}
	}
}

func TestPiNoticeKindClassification(t *testing.T) {
	// agent_start 是 pi 协议里已知的事件(选择不投影);把它记成未知等于让已知噪音回流。
	if got := piNoticeKind("agent_start"); got != runtime.KindKnownPassthrough {
		t.Errorf("piNoticeKind(\"agent_start\") = %q, want PASSTHROUGH(已知但不投影)", got)
	}
	for _, et := range []string{"totally_new_event", ""} {
		if got := piNoticeKind(et); got != runtime.KindUnknownNotification {
			t.Errorf("piNoticeKind(%q) = %q, want UNKNOWN", et, got)
		}
	}
}

func TestCodexMethodNoticeKindClassification(t *testing.T) {
	for _, m := range []string{"turn/diff/updated", "model/verification"} {
		if got := codexMethodNoticeKind(m); got != runtime.KindKnownPassthrough {
			t.Errorf("codexMethodNoticeKind(%q) = %q, want PASSTHROUGH(已知 method,本次无可投影)", m, got)
		}
	}
	for _, m := range []string{"brand/new/method", ""} {
		if got := codexMethodNoticeKind(m); got != runtime.KindUnknownNotification {
			t.Errorf("codexMethodNoticeKind(%q) = %q, want UNKNOWN", m, got)
		}
	}
}

// 【接线钉】分类结果必须真的成为台账里的 normalized_kind ——
// 分类函数返回值对、而写入侧仍回落 UNKNOWN,等于没修。
//
// 【它不覆盖什么(如实写明)】本用例走的是「分类函数 + recordAs」这一对组合,与生产
// 调用点用的是同一对;但"11 个调用点是否都换成了 recordAs"由静态检查保证
// (`grep -rn "notices\.record(" daemon/internal/agent/*.go` 应为 0 命中),不在本用例内。
func TestNoticeKindLandsInIndex(t *testing.T) {
	var notices nativeNoticeRecorder
	notices.threadID = "th-kind-lands"
	sink := func(*pb.TurnServerMessage) {}

	want := map[string]string{
		"session/update:context_usage": runtime.KindKnownProjected,
		"agent_start":                  runtime.KindKnownPassthrough,
		"state.updated":                runtime.KindKnownProjected,
		"turn/diff/updated":            runtime.KindKnownPassthrough,
		"session/update:plan":          runtime.KindKnownPassthrough,
		"session/update:whatever_new":  runtime.KindUnknownNotification,
	}
	notices.recordAs(dshNoticeKind("session/update:context_usage"), "dsh", "session/update:context_usage", json.RawMessage(`{}`), sink)
	notices.recordAs(piNoticeKind("agent_start"), "pi", "agent_start", json.RawMessage(`{}`), sink)
	notices.recordAs(zcodeNoticeKind("state.updated"), "zcode", "state.updated", json.RawMessage(`{}`), sink)
	notices.recordAs(codexMethodNoticeKind("turn/diff/updated"), "codex", "turn/diff/updated", json.RawMessage(`{}`), sink)
	notices.recordAs(dshNoticeKind("session/update:plan"), "dsh", "session/update:plan", json.RawMessage(`{}`), sink)
	notices.recordAs(dshNoticeKind("session/update:whatever_new"), "dsh", "session/update:whatever_new", json.RawMessage(`{}`), sink)

	seen := 0
	for _, e := range sharedRuntimeBackend().UnknownEvents().List(runtime.ListFilter{ThreadID: "th-kind-lands"}, 0) {
		w, ok := want[e.SourceMethod]
		if !ok {
			continue
		}
		seen++
		if e.NormalizedKind != w {
			t.Errorf("%s 的 normalized_kind = %q, want %q", e.SourceMethod, e.NormalizedKind, w)
		}
	}
	if seen != len(want) {
		t.Fatalf("有 notice 没进台账(分类没接到 record 上): seen=%d want=%d", seen, len(want))
	}
}
