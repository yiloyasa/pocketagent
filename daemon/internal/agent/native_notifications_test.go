package agent

import (
	"encoding/json"
	"strings"
	"testing"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/runtime"
)

func TestNativeSelectionPreservesUnfamiliarModelsAndLevels(t *testing.T) {
	pi := decodeRuntimeSelection("pi", "model_changed", json.RawMessage(`{"model":{"id":"future-model/variant","provider":"private"},"thinkingLevel":"ultra-new","future":{"flag":true}}`))
	if pi.ModelID != "pi/private/future-model/variant" || pi.Effort == nil || *pi.Effort != "ultra-new" {
		t.Fatalf("lost opaque runtime values: %+v", pi)
	}
	zc := decodeRuntimeSelection("zcode", "session.updated", json.RawMessage(`{"snapshot":{"settings":{"model":{"current":{"modelId":"custom-v99","options":{"reasoningLevel":"adaptive"}}}}}}`))
	if zc.ModelID != "zcode/custom-v99" || zc.Effort == nil || *zc.Effort != "adaptive" {
		t.Fatalf("lost snapshot values: %+v", zc)
	}
	partial := decodeRuntimeSelection("pi", "thinking_level_changed", json.RawMessage(`{"level":"new-level"}`))
	if partial.ModelID != "" || partial.Effort == nil {
		t.Fatal("partial update must not invent a model")
	}
}

func TestUnknownNativeNotificationsAreBoundedNonFatalAndRedacted(t *testing.T) {
	var recorder nativeNoticeRecorder
	var frames []*pb.TurnServerMessage
	for i := 0; i < 100; i++ {
		recorder.record("future-agent", "unseen.event", json.RawMessage(`{"model":"never-seen","auth":{"api_key":"do-not-log"},"level":"adaptive"}`),
			func(frame *pb.TurnServerMessage) { frames = append(frames, frame) })
	}
	if len(frames) != 2 {
		t.Fatalf("unbounded or lost diagnostics: %d", len(frames))
	}
	ev := frames[0].GetCodexEvent()
	if ev.Kind != pb.CodexEvent_STATUS || strings.Contains(ev.RawJson, "do-not-log") || !strings.Contains(ev.RawJson, "never-seen") {
		t.Fatalf("bad native diagnostic: %s", ev.RawJson)
	}
}

func TestFuturePiEffortAndCodexOpaqueIDsAreNotTruncated(t *testing.T) {
	levels := piModelThinkingLevels(true, map[string]json.RawMessage{
		"off": json.RawMessage(`null`), "adaptive-new": json.RawMessage(`"adaptive"`),
	})
	found := false
	for _, level := range levels {
		if level == "adaptive-new" {
			found = true
		}
		if level == "off" {
			t.Fatal("explicitly disabled level was advertised")
		}
	}
	if !found {
		t.Fatal("future native capability was dropped")
	}
	if got := lastModelSeg("codex/private-provider/future-model/v2"); got != "private-provider/future-model/v2" {
		t.Fatalf("opaque native model identity was truncated: %s", got)
	}
}

func TestDuplicateZcodeModelsKeepProviderIdentity(t *testing.T) {
	raw := json.RawMessage(`{"settings":{"model":{"available":[
		{"ref":{"providerId":"account-A","modelId":"custom/model"},"label":"Custom","reasoning":{"levels":["adaptive"],"defaultLevel":"adaptive"}},
		{"ref":{"providerId":"account-B","modelId":"custom/model"},"label":"Custom","reasoning":{"levels":[{"value":"new-level"}],"defaultLevel":"new-level"}}
	]}}}`)
	models, refs := parseZcodeModels(raw)
	if len(models) != 2 || models[0].Id == models[1].Id {
		t.Fatal("two native providers were collapsed into one model")
	}
	for i := range models {
		key := strings.TrimPrefix(models[i].Id, "zcode/")
		if ref := refs[key]; ref.NativeID != "custom/model" || ref.ProviderID == "" {
			t.Fatalf("lost native model reference: %+v", ref)
		}
	}
	if _, ambiguous := refs["custom/model"]; ambiguous {
		t.Fatal("ambiguous legacy ID must not silently select an account")
	}
}

// ---- codex item 通知的"已知/未知"分档(方案 §4.6 分类词表 / §9.4 验收第 1 条)----
//
// 【为什么单列一组】此前 record 把每条 notice 一律记成 UNKNOWN_NOTIFICATION,
// 而 codex 每轮都会产生 reasoning / userMessage / summary —— 诊断页被已知噪音填满,
// 上游升级后真发来怪东西时淹在里面看不见。这一组钉的是"分类规则"与"分类结果真的
// 进了台账"两件事:前者是判定,后者是接线,失效原因不同,要分别能红。

// TestCodexItemNoticeKindClassification 钉住分类规则:判定依据是"daemon 对这个
// item 类型做了什么",不是"类型名眼熟不眼熟"。
func TestCodexItemNoticeKindClassification(t *testing.T) {
	cases := []struct {
		itemType string
		want     string
		why      string
	}{
		{"reasoning", runtime.KindKnownProjected, "已被投影成协议内的 ThinkingDelta"},
		{"userMessage", runtime.KindKnownPassthrough, "无正式投影,只原样透传留痕"},
		{"summary", runtime.KindKnownPassthrough, "无正式投影,只原样透传留痕"},
		{"webSearch", runtime.KindUnknownNotification, "杜撰的新类型 = 我们确实没见过"},
		{"", runtime.KindUnknownNotification, "拿不到类型名时如实记未知,不替它编一个已知身份"},
	}
	for _, c := range cases {
		if got := codexItemNoticeKind(c.itemType); got != c.want {
			t.Errorf("codexItemNoticeKind(%q) = %q, want %q(%s)", c.itemType, got, c.want, c.why)
		}
	}
	// 白名单五类各有专案分支,isOpaqueItemType 已先把它们挡在本函数之前;万一真走到,
	// 宁可标未知把异常暴露出来,也不套一个错误的"已知"标签把它盖住。
	for _, k := range []string{"contextCompaction", "commandExecution", "agentMessage", "subAgentActivity", "collabAgentToolCall"} {
		if got := codexItemNoticeKind(k); got != runtime.KindUnknownNotification {
			t.Errorf("专案分支类型 %q 不该被本函数标成已知: %q", k, got)
		}
	}
}

// TestCodexOpaqueItemKindLandsInIndex 钉住接线:分类结果真的成了诊断台账里的
// normalized_kind —— 函数返回值对、台账仍写 UNKNOWN 也是失效。
func TestCodexOpaqueItemKindLandsInIndex(t *testing.T) {
	cases := []struct {
		itemType string
		want     string
	}{
		{"reasoning", runtime.KindKnownProjected},
		{"userMessage", runtime.KindKnownPassthrough},
		{"webSearch", runtime.KindUnknownNotification},
	}
	var notices nativeNoticeRecorder
	notices.threadID = "th-kind"
	sink := func(*pb.TurnServerMessage) {}
	for _, c := range cases {
		params := json.RawMessage(`{"threadId":"th-kind","item":{"type":"` + c.itemType + `"}}`)
		if !maybeRecordOpaqueItem("item/started", params, "th-kind", &notices, sink) {
			t.Fatalf("%s 应命中透传路由", c.itemType)
		}
	}
	seen := map[string]bool{}
	for _, e := range sharedRuntimeBackend().UnknownEvents().List(runtime.ListFilter{ThreadID: "th-kind"}, 0) {
		for _, c := range cases {
			key := "item/started:" + c.itemType
			if e.SourceMethod != key {
				continue
			}
			seen[c.itemType] = true
			if e.NormalizedKind != c.want {
				t.Fatalf("%s 的 normalized_kind 应为 %s,得到 %s —— 已知 item 不得冒充 UNKNOWN",
					key, c.want, e.NormalizedKind)
			}
		}
	}
	for _, c := range cases {
		if !seen[c.itemType] {
			t.Fatalf("%s 未登记进诊断台账(分类没有接到 record 上)", c.itemType)
		}
	}
}

// TestRecordAsEmptyKindFallsBackToUnknown 钉住"既有调用方零成本":pi/zcode/dsh 等
// 不关心分档、沿用 record(即空 kind),必须继续记成 UNKNOWN_NOTIFICATION —— 分档
// 不能让没分类的路径被误标成已知。
func TestRecordAsEmptyKindFallsBackToUnknown(t *testing.T) {
	var notices nativeNoticeRecorder
	notices.threadID = "th-fallback"
	notices.recordAs("", "dsh", "fallbackProbeXYZ", json.RawMessage(`{}`), func(*pb.TurnServerMessage) {})
	found := false
	for _, e := range sharedRuntimeBackend().UnknownEvents().List(runtime.ListFilter{ThreadID: "th-fallback"}, 0) {
		if e.SourceMethod != "fallbackProbeXYZ" {
			continue
		}
		found = true
		if e.NormalizedKind != runtime.KindUnknownNotification {
			t.Fatalf("空 kind 应回落 UNKNOWN_NOTIFICATION,得到 %q", e.NormalizedKind)
		}
	}
	if !found {
		t.Fatal("空 kind 的 notice 也必须登记进台账(既有行为不变)")
	}
}
