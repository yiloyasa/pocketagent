// native_notifications_preadopt_20261007_test.go —— 2026-10-07 预收编轮的分类
// 钉子(native_notifications.go 的三组收编:zcode v4 三连、permission/userInput
// 四件套、codex rawResponse 内部通知族)。
//
// 【证据】docs/reviews/2026-10-07/field-coverage-zcode.md §A/§B(bundle 静态
// payload 形状)、field-coverage-codex.md §2/§3(0.160.0 二进制通知枚举表序 +
// codex-schema/v2/RawResponse*.json "Internal-only" 双证)。未运行任何程序。
package agent

import (
	"encoding/json"
	"strings"
	"testing"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/runtime"
)

// ① zcode v4 三连:顶层通知注册表 jV={conversationFrame, conversationTelemetryFact,
// localTtftFacts, cuaPermissionObservation} 的同族成员,比照既有
// v4/telemetry/event 收编 PASSTHROUGH(内核遥测/镜像帧,只留痕不投影)。
func TestZcodeNoticeKindV4Family20261007(t *testing.T) {
	for _, m := range []string{
		"v4/conversation/frame", "v4/telemetry/local-ttft", "v4/cua/permission-observation",
	} {
		if got := zcodeNoticeKind(m); got != runtime.KindKnownPassthrough {
			t.Errorf("zcodeNoticeKind(%q) = %q, want PASSTHROUGH(jV 注册表同族,2026-10-07 预收编)", m, got)
		}
	}
	// 精确名收编不放宽口径:注册表外的 v4 同形新名仍如实 UNKNOWN。
	if got := zcodeNoticeKind("v4/telemetry/brandNewFact"); got != runtime.KindUnknownNotification {
		t.Errorf("zcodeNoticeKind(v4/telemetry/brandNewFact) = %q, want UNKNOWN(同族不等于同宽)", got)
	}
}

// ② permission/userInput 四件套:上轮已取得完整 bundle 静态 payload 形状
// (field-coverage-zcode.md §B:Psr/Rsr/Dsr/Osr),收编 PASSTHROUGH —— 审批/
// 反问的用户可见面仍走反向请求通道的审批卡/反问卡,事件流镜像只留痕。
func TestZcodeNoticeKindPermissionQuad20261007(t *testing.T) {
	for _, m := range []string{
		"permission.requested", "permission.resolved",
		"userInput.requested", "userInput.resolved",
	} {
		if got := zcodeNoticeKind(m); got != runtime.KindKnownPassthrough {
			t.Errorf("zcodeNoticeKind(%q) = %q, want PASSTHROUGH(2026-10-07 预收编,只留痕不投影)", m, got)
		}
	}
	// 拆壳接线:全局钩子路径按复合键 "session/event:<type>" 登记、内部 type 分档,
	// 与轮内平键同一定性(zcodeLedgerMethod 与生产全局钩子是同一对)。
	for _, typ := range []string{
		"permission.requested", "permission.resolved",
		"userInput.requested", "userInput.resolved",
	} {
		m, k := zcodeLedgerMethod("session/event",
			json.RawMessage(`{"sessionId":"s","type":"`+typ+`","payload":{}}`))
		if m != "session/event:"+typ || k != runtime.KindKnownPassthrough {
			t.Errorf("zcodeLedgerMethod(session/event %s) = (%q,%q), want (session/event:%s, PASSTHROUGH)", typ, m, k, typ)
		}
	}
	// 名字眼熟不等于在册:union 外的近似名仍 UNKNOWN,不冒充已知。
	for _, m := range []string{"permission.brandNew", "userInput.brandNew"} {
		if got := zcodeNoticeKind(m); got != runtime.KindUnknownNotification {
			t.Errorf("zcodeNoticeKind(%q) = %q, want UNKNOWN(精确名收编,近似名不放宽)", m, got)
		}
	}
}

// ③ codex rawResponse 内部通知族(0.160.0 新增):登记表(codexGlobalNoticeKinds,
// 另一轮所有权)受 schema 对表测试约束收不了 0.159.2 快照外的新名,分类中枢按
// 方法名前缀预收编 PASSTHROUGH —— 中枢是全局表的受控超集。
func TestCodexRawResponseNoticeKind20261007(t *testing.T) {
	for _, m := range []string{"rawResponse/completed", "rawResponseItem/completed"} {
		if got := codexMethodNoticeKind(m); got != runtime.KindKnownPassthrough {
			t.Errorf("codexMethodNoticeKind(%q) = %q, want PASSTHROUGH(0.160.0 内部通知,2026-10-07 预收编)", m, got)
		}
		if got := codexGlobalNoticeKind(m); got != runtime.KindUnknownNotification {
			t.Errorf("codexGlobalNoticeKind(%q) = %q, want UNKNOWN(登记表未收,归另一轮;中枢前缀规则是它的超集)", m, got)
		}
	}
	// 前缀即一族:0.160.x 同族新变体自动同口径(取舍见 codexMethodNoticeKind 注)。
	if got := codexMethodNoticeKind("rawResponse/futureVariant"); got != runtime.KindKnownPassthrough {
		t.Errorf("codexMethodNoticeKind(rawResponse/futureVariant) = %q, want PASSTHROUGH(前缀族口径)", got)
	}
	// 前缀之外不放宽:同文异族与真未知仍 UNKNOWN。
	for _, m := range []string{"rawresponses/echo", "usage/rawResponse", "brand/new/notice", ""} {
		if got := codexMethodNoticeKind(m); got != runtime.KindUnknownNotification {
			t.Errorf("codexMethodNoticeKind(%q) = %q, want UNKNOWN(前缀之外不放宽)", m, got)
		}
	}
}

// ④ 汇点复核钉:codex 全局钩子(codex.go)以 codexGlobalNoticeKind 定性后才进
// recordProviderEvent,rawResponse 族在登记表外会被初判 UNKNOWN;汇点在
// codex+UNKNOWN 时按中枢分类器复核一次,保证同一方法两路定性一致(2026-10-02
// 单表原则)。本钉模拟生产调用对(全局钩子的 kind 实参 + recordProviderEvent):
// 预收编族落台账为 PASSTHROUGH,真未知方法经同一对仍是 UNKNOWN。
func TestRecordProviderEventRefinesCodexUnknown20261007(t *testing.T) {
	const th = "th-preadopt-20261007"
	recordProviderEvent("codex", "rawResponse/completed", th,
		codexGlobalNoticeKind("rawResponse/completed"), json.RawMessage(`{"threadId":"x","usage":null}`))
	recordProviderEvent("codex", "rawResponseItem/completed", th,
		codexGlobalNoticeKind("rawResponseItem/completed"), json.RawMessage(`{}`))
	recordProviderEvent("codex", "brand/new/notice", th,
		codexGlobalNoticeKind("brand/new/notice"), json.RawMessage(`{}`))
	want := map[string]string{
		"rawResponse/completed":     runtime.KindKnownPassthrough,
		"rawResponseItem/completed": runtime.KindKnownPassthrough,
		"brand/new/notice":          runtime.KindUnknownNotification,
	}
	seen := 0
	for _, e := range sharedRuntimeBackend().UnknownEvents().List(runtime.ListFilter{ThreadID: th}, 0) {
		w, ok := want[e.SourceMethod]
		if !ok {
			continue
		}
		seen++
		if e.NormalizedKind != w {
			t.Errorf("%s 台账 normalized_kind = %q, want %q", e.SourceMethod, e.NormalizedKind, w)
		}
	}
	if seen != len(want) {
		t.Fatalf("预收编方法没进台账: seen=%d want=%d", seen, len(want))
	}
}

// ⑤ 透传卡钉:收编后轮内兜底路径(recordAs)对预收编方法出限频诊断卡,卡标题
// 带方法名、载荷脱敏上屏 —— "透传卡如实上屏"的形状保证(zcode 四件套与 codex
// rawResponse 族走同一条 recordAs 通路,钉一处即钉全局)。
func TestPreAdoptedPassthroughCardShape20261007(t *testing.T) {
	var notices nativeNoticeRecorder
	notices.threadID = "th-preadopt-card"
	var cards []*pb.CodexEvent
	sink := func(m *pb.TurnServerMessage) {
		if ev := m.GetCodexEvent(); ev != nil {
			cards = append(cards, ev)
		}
	}
	notices.recordAs(codexMethodNoticeKind("rawResponse/completed"), "codex", "rawResponse/completed",
		json.RawMessage(`{"responseId":"resp_1","threadId":"t","turnId":"n","usage":null,"apiKey":"sk-secret"}`), sink)
	if len(cards) != 1 {
		t.Fatalf("应有 1 张透传卡, got=%d", len(cards))
	}
	if cards[0].Code != "native_notification" || cards[0].Title != "codex · rawResponse/completed" {
		t.Fatalf("透传卡形状不符: %+v", cards[0])
	}
	if cards[0].Message != "原生状态通知" {
		t.Fatalf("透传卡文案不符: %q", cards[0].Message)
	}
	if cards[0].RawJson == "" || strings.Contains(cards[0].RawJson, "sk-secret") {
		t.Fatalf("透传卡载荷必须脱敏: %s", cards[0].RawJson)
	}
}
