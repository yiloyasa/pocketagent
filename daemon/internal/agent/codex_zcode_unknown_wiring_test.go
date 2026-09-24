// codex_zcode_unknown_wiring_test.go —— AUDIT §3.3「mock 未知 method 通知」回归资产。
//
// 【钉什么】W8(d82d915)给 codex 全局通知路径加了两件事:① codexGlobalNoticeKind
// 白名单扩容(此前所有全局通知一律记 UNKNOWN,已知噪音灌满诊断页);② 供给
// attachGlobalHandler 闭包用的共用汇点 recordProviderEvent。本文件钉这两个可直接
// 触达的生产函数;「钩子挂上后真 dispatch 能落到台账」的端到端半边由 codexapp /
// zcodeapp 包的 dispatch 级测试覆盖(wireMsg 与 dispatch 未导出,跨包无法构造)。
//
// 【为什么值得钉】分类表是 W8 刚扩容的:多写一个 case 名就把"未知"洗成"已知",
// 诊断页立刻失去"真没见过"的分辨率;recordProviderEvent 若把 kind 丢了,分类写得
// 再对也只是空转 —— 两件事失效原因不同,分开钉。
package agent

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/pocketagent/daemon/internal/runtime"
)

// TestCodexGlobalNoticeKindClassification 钉住 W8 扩容后的全局通知分档表:
// 会话级事实(thread/*、skills/*)daemon 会投影落库 → PROJECTED;
// turn/diff、model/verification 只透传留痕 → PASSTHROUGH;其余如实 UNKNOWN。
func TestCodexGlobalNoticeKindClassification(t *testing.T) {
	for _, m := range []string{
		"thread/archived", "thread/unarchived", "thread/deleted", "thread/reverted",
		"thread/closed", "thread/name/updated", "thread/queue/changed",
		"thread/status/changed", "thread/started", "skills/changed",
		"thread/plan/updated", "thread/goal/updated", "thread/goal/cleared",
	} {
		if got := codexGlobalNoticeKind(m); got != runtime.KindKnownProjected {
			t.Errorf("codexGlobalNoticeKind(%q) = %q, want PROJECTED(会话级事实会投影落库)", m, got)
		}
	}
	for _, m := range []string{"turn/diff/updated", "model/verification"} {
		if got := codexGlobalNoticeKind(m); got != runtime.KindKnownPassthrough {
			t.Errorf("codexGlobalNoticeKind(%q) = %q, want PASSTHROUGH(已知 method,本次无可投影)", m, got)
		}
	}
	// 0.156.1 收编批(docs/verify/evidence/codex-0156-probe.log):通知面无消失,
	// 13 条原"daemon 零引用"通知定性为 PASSTHROUGH —— 只留痕/透传,不投影落库;
	// 词表人话在 runtime/knowledge_codex.go。这里钉死分档,防止后续改动把
	// "已知"洗回"未知"灌满诊断页,或反过来把没投影的东西冒充 PROJECTED。
	for _, m := range []string{
		"thread/environment/connected", "thread/environment/disconnected",
		"thread/settings/updated", "configWarning", "deprecationNotice",
		"item/reasoning/summaryPartAdded", "turn/moderationMetadata",
		"account/updated", "account/login/completed",
		"mcpServer/oauthLogin/completed", "process/exited",
		"hook/started", "hook/completed",
	} {
		if got := codexGlobalNoticeKind(m); got != runtime.KindKnownPassthrough {
			t.Errorf("codexGlobalNoticeKind(%q) = %q, want PASSTHROUGH(0.156.1 收编批,只留痕不投影)", m, got)
		}
	}
	// 上游升级后真发来的怪东西必须保持 UNKNOWN;空 method 同样如实记未知。
	for _, m := range []string{"brand/new/notice", "item/guardian/v2", ""} {
		if got := codexGlobalNoticeKind(m); got != runtime.KindUnknownNotification {
			t.Errorf("codexGlobalNoticeKind(%q) = %q, want UNKNOWN(真没见过,不冒充已知)", m, got)
		}
	}
}

// TestRecordProviderEventLandsInLedger 钉住共用汇点:kind 原样落 normalized_kind,
// 会话归属、脱敏载荷一路带到台账;kind 为空回落 UNKNOWN_NOTIFICATION(调用方没
// 分类 = 我们确实不知道,不冒充已知)。
func TestRecordProviderEventLandsInLedger(t *testing.T) {
	recordProviderEvent("codex", "wiringProbeXYZ/unseen",
		"th-wiring-probe", runtime.KindUnknownNotification,
		json.RawMessage(`{"threadId":"ignored","token":"secret-wiring"}`))
	found := false
	for _, e := range sharedRuntimeBackend().UnknownEvents().List(runtime.ListFilter{AgentID: "codex"}, 0) {
		if e.SourceMethod != "wiringProbeXYZ/unseen" {
			continue
		}
		found = true
		if e.NormalizedKind != runtime.KindUnknownNotification {
			t.Fatalf("kind 应原样落台账,得到 %q", e.NormalizedKind)
		}
		if e.ThreadID != "th-wiring-probe" {
			t.Fatalf("会话归属应透传,得到 %q", e.ThreadID)
		}
		if strings.Contains(e.RawPayload, "secret-wiring") {
			t.Fatalf("载荷必须脱敏,得到 %s", e.RawPayload)
		}
	}
	if !found {
		t.Fatal("recordProviderEvent 必须把事件记进诊断台账(汇点断了分类就只是空转)")
	}

	// 空 kind 回落 UNKNOWN:白名单扩容后新增 provider 忘了分类时,宁可记未知也不假称已知。
	recordProviderEvent("dsh", "wiringProbeXYZ/noKind", "th-wiring-probe", "", json.RawMessage(`{}`))
	unclassified := false
	for _, e := range sharedRuntimeBackend().UnknownEvents().List(runtime.ListFilter{AgentID: "dsh"}, 0) {
		if e.SourceMethod != "wiringProbeXYZ/noKind" {
			continue
		}
		unclassified = true
		if e.NormalizedKind != runtime.KindUnknownNotification {
			t.Fatalf("空 kind 应回落 UNKNOWN_NOTIFICATION,得到 %q", e.NormalizedKind)
		}
	}
	if !unclassified {
		t.Fatal("空 kind 的 recordProviderEvent 也必须落台账")
	}
}
