// unknown_action_test.go —— 未知事件确定性呈现(用户处理中心 Task 2)的回归用例。
//
// 验收点:
//
//	① 三类 UNKNOWN_* kind 的呈现是固定文案快照:逐字段断言;确定性 = 同一封信封同一
//	  输出;唯一的方法名依赖是 UNKNOWN_REQUEST 标题追加 SourceMethod 原文(不翻译
//	  不改写,缺席回退固定标题),其余字段与载荷/方法名/来源 agent 无关;
//	② 安全钉子:任何 kind 的任何动作,ID/Label/Description 都不得含 allow/approve/confirm
//	  字样(大小写不敏感);UNKNOWN_REQUEST 的动作集合被整体钉死,绝不出现"允许"类按钮;
//	③ method/agent 原文保留:呈现只读信封,调用前后信封逐字段不变,也不给方法名编"翻译";
//	④ 缺席值不造值:信封除 kind 外全空,三类 kind 仍是同一份固定文案(文案来自规则表,
//	  不是从信封编出来的);词表外/空 kind 全空返回,交由客户端按协议降级;
//	⑤ 脱敏载荷仍出完整呈现与摘要(学 event_translate_test 的 [redacted] 构造);
//	⑥ 呈现/翻译不落盘:处置状态与 revision 不被触碰;病态输入不 panic。
package runtime

import (
	"strings"
	"testing"
)

// presentEqual 逐字段比较两份呈现(UnknownPresentation 含切片,不能用 ==)。
func presentEqual(a, b UnknownPresentation) bool {
	if a.PlainTitle != b.PlainTitle || a.PlainExplanation != b.PlainExplanation ||
		a.Impact != b.Impact || a.RequiresUser != b.RequiresUser ||
		a.CanOpenNative != b.CanOpenNative || a.RecommendedNextStep != b.RecommendedNextStep ||
		len(a.Actions) != len(b.Actions) {
		return false
	}
	for i := range a.Actions {
		if a.Actions[i] != b.Actions[i] {
			return false
		}
	}
	return true
}

// wantActions 逐字段断言动作表(不引 reflect:字段一眼可数,错了好定位)。
func wantActions(t *testing.T, got, want []UnknownUserAction) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("动作数应为 %d,得到 %d: %+v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("动作 #%d 不匹配:\n got  %+v\n want %+v", i, got[i], want[i])
		}
	}
}

// TestPresentUnknownEventNotificationSnapshot 未知通知的固定文案快照。
func TestPresentUnknownEventNotificationSnapshot(t *testing.T) {
	got := PresentUnknownEvent(EventEnvelope{
		ID: "ue-1-abcd", Sequence: 1, TimestampMs: 1,
		SourceAgent: "codex", SourceMethod: "brandNewNotificationXYZ",
		NormalizedKind: KindUnknownNotification, SafetyClass: SafetyClassUnknown,
		RawPayload: `{"token":"[redacted]"}`,
	})
	if got.PlainTitle != "电脑端传来了新消息" {
		t.Errorf("title: %q", got.PlainTitle)
	}
	if got.PlainExplanation != "当前版本还没有对应的卡片显示方式，内容已原样保留。" {
		t.Errorf("explanation: %q", got.PlainExplanation)
	}
	if got.Impact != "不影响正在进行的任务" {
		t.Errorf("impact: %q", got.Impact)
	}
	if got.RequiresUser {
		t.Error("通知不应催办(RequiresUser 应为 false)")
	}
	if !got.CanOpenNative {
		t.Error("通知应可在电脑端查看(CanOpenNative 应为 true)")
	}
	if got.RecommendedNextStep != "可在消息透析中查看原文" {
		t.Errorf("next step: %q", got.RecommendedNextStep)
	}
	wantActions(t, got.Actions, []UnknownUserAction{
		{ID: "inspect", Label: "查看", Description: "展开查看这条事件的脱敏技术原文", Safe: true},
		{ID: "copy", Label: "复制", Description: "把脱敏技术原文复制到剪贴板", Safe: true},
		{ID: "acknowledge", Label: "标记已查看", Description: "把这条事件标记为已查看", Safe: true},
	})
}

// TestPresentUnknownEventRequestSnapshot 未知请求的固定文案快照 + 安全红线:
// 动作集合被整体钉死 —— 只有查看/复制/在电脑端处理/待上游支持,
// 绝不出现 allow/approve/confirm/execute 类"允许"按钮。
func TestPresentUnknownEventRequestSnapshot(t *testing.T) {
	got := PresentUnknownEvent(EventEnvelope{
		SourceAgent: "dsh", SourceMethod: "weird/request",
		ThreadID: "th-9", TurnID: "tn-1",
		NormalizedKind: KindUnknownRequest, SafetyClass: SafetyClassUnknown,
		RawPayload: `{"command":"[redacted]"}`,
	})
	if got.PlainTitle != "电脑端请求了一个操作：weird/request" {
		t.Errorf("title 应追加方法原文(主模型裁决,原样拼接): %q", got.PlainTitle)
	}
	// 方法名缺席时不造值:标题回退固定文案,不拼空冒号、不编方法名。
	if bare := PresentUnknownEvent(EventEnvelope{NormalizedKind: KindUnknownRequest}); bare.PlainTitle != "电脑端请求了一个操作" {
		t.Errorf("无方法名时标题应回退固定文案(缺席值不造值): %q", bare.PlainTitle)
	}
	if got.PlainExplanation != "电脑端请求了一个当前版本无法安全判断的操作，因此没有自动执行。" {
		t.Errorf("explanation: %q", got.PlainExplanation)
	}
	if got.Impact != "该操作未执行；如涉及你关心的任务，请在电脑端处理" {
		t.Errorf("impact: %q", got.Impact)
	}
	if !got.RequiresUser {
		t.Error("未知请求必须进「需要我处理」分组(RequiresUser 应为 true)")
	}
	if !got.CanOpenNative {
		t.Error("未知请求应可在电脑端处理(CanOpenNative 应为 true)")
	}
	if got.RecommendedNextStep != "在电脑端完成该操作后任务会继续" {
		t.Errorf("next step: %q", got.RecommendedNextStep)
	}
	wantActions(t, got.Actions, []UnknownUserAction{
		{ID: "inspect", Label: "查看", Description: "展开查看这条事件的脱敏技术原文", Safe: true},
		{ID: "copy", Label: "复制", Description: "把脱敏技术原文复制到剪贴板", Safe: true},
		{ID: "open_native", Label: "在电脑端处理", Description: "在电脑端的原生界面处理这个请求，本端不代为执行", Safe: true},
		{ID: "retry_later", Label: "待上游支持", Description: "把这条事件标记为稍后再看", Safe: true},
	})
	// 方法原文只进标题:换一个方法名,除 PlainTitle 外的呈现必须与无方法信封逐字段
	// 一致(原文保留不翻译,也不许掺进解释/影响/动作表)。
	other := PresentUnknownEvent(EventEnvelope{
		SourceMethod: "another/<Method>", NormalizedKind: KindUnknownRequest, RawPayload: `{"z":1}`,
	})
	if other.PlainTitle != "电脑端请求了一个操作：another/<Method>" {
		t.Errorf("方法原文应原样拼接: %q", other.PlainTitle)
	}
	fixed := PresentUnknownEvent(EventEnvelope{NormalizedKind: KindUnknownRequest})
	fixed.PlainTitle = other.PlainTitle
	if !presentEqual(other, fixed) {
		t.Fatalf("方法名只允许进标题,其余呈现必须与无方法信封一致:\n got  %+v\n want %+v", other, fixed)
	}
}

// TestPresentUnknownEventResponseSnapshot 未知响应的固定文案快照。
func TestPresentUnknownEventResponseSnapshot(t *testing.T) {
	got := PresentUnknownEvent(EventEnvelope{
		SourceAgent: "zcode", SourceMethod: "lateReply",
		CorrelationID: "resp-42",
		NormalizedKind: KindUnknownResponse, SafetyClass: SafetyClassUnknown,
		RawPayload: `{"result":"[redacted]"}`,
	})
	if got.PlainTitle != "电脑端返回了一条结果" {
		t.Errorf("title: %q", got.PlainTitle)
	}
	if got.PlainExplanation != "电脑端返回了一条结果，但当前版本找不到它对应的请求。原始结果已保留。" {
		t.Errorf("explanation: %q", got.PlainExplanation)
	}
	if got.Impact != "不影响正在进行的任务" {
		t.Errorf("impact: %q", got.Impact)
	}
	if got.RequiresUser {
		t.Error("未知响应仅供参考,不应催办(RequiresUser 应为 false)")
	}
	if got.CanOpenNative {
		t.Error("未知响应的动作表里没有 open_native,CanOpenNative 不开空头承诺(应为 false)")
	}
	if got.RecommendedNextStep != "可在消息透析中查看原始结果" {
		t.Errorf("next step: %q", got.RecommendedNextStep)
	}
	wantActions(t, got.Actions, []UnknownUserAction{
		{ID: "inspect", Label: "查看", Description: "展开查看这条事件的脱敏技术原文", Safe: true},
		{ID: "copy", Label: "复制", Description: "把脱敏技术原文复制到剪贴板", Safe: true},
	})
}

// TestPresentUnknownEventSafetyNail 安全钉子:扫遍 KnownKinds() 全集 + 空 + 词表外,
// 任何 kind 的任何动作都不得含 allow/approve/confirm 字样;进呈现表的动作必须
// 全部声明安全且无破坏性(全局约束:只有明确安全的动作才能进用户操作区)。
// 【为什么扫全集】将来给新 kind 加规则时钉子自动覆盖,不用记得回来补测试。
func TestPresentUnknownEventSafetyNail(t *testing.T) {
	kinds := append(KnownKinds(), "", "SOME_FUTURE_KIND")
	for _, kind := range kinds {
		got := PresentUnknownEvent(EventEnvelope{NormalizedKind: kind, SourceMethod: "novel/method"})
		for i, a := range got.Actions {
			for _, field := range []struct {
				name, val string
			}{{"ID", a.ID}, {"Label", a.Label}, {"Description", a.Description}} {
				lower := strings.ToLower(field.val)
				for _, bad := range []string{"allow", "approve", "confirm"} {
					if strings.Contains(lower, bad) {
						t.Errorf("kind=%s 动作#%d 的 %s=%q 含违禁字样 %q(安全红线:未知事件不得提供允许类动作)",
							kind, i, field.name, field.val, bad)
					}
				}
			}
			if !a.Safe {
				t.Errorf("kind=%s 动作#%d(%s) Safe=false:进呈现表的动作必须全部声明安全", kind, i, a.ID)
			}
			if a.Destructive || a.RequiresConfirmation {
				t.Errorf("kind=%s 动作#%d(%s) 带破坏性/需确认标记:当前安全动作集全是幂等的读/标记/跳转", kind, i, a.ID)
			}
		}
	}
}

// TestPresentUnknownEventDeterministicAndRedactionSafe 确定性:同 kind 同输出,
// 与载荷/方法/来源无关 —— 载荷脱敏与否、方法名怎么怪,呈现一字不变。
// 这同时钉住"脱敏后仍出摘要":脱敏载荷的信封照样得到完整呈现。
func TestPresentUnknownEventDeterministicAndRedactionSafe(t *testing.T) {
	base := PresentUnknownEvent(EventEnvelope{
		SourceAgent: "codex", SourceMethod: "brandNewThing",
		NormalizedKind: KindUnknownNotification, RawPayload: `{"a":1}`,
	})
	variants := []EventEnvelope{
		{SourceAgent: "pi", SourceMethod: "totally/different:method", NormalizedKind: KindUnknownNotification,
			RawPayload: `{"token":"[redacted]","password":"[redacted]","secret":"[redacted]"}`},
		{SourceMethod: "x", NormalizedKind: KindUnknownNotification, RawPayload: `{"error":"[redacted]"}`},
		{NormalizedKind: KindUnknownNotification}, // 无载荷、无来源
	}
	for i, env := range variants {
		got := PresentUnknownEvent(env)
		if !presentEqual(got, base) {
			t.Fatalf("变体 %d 的呈现随输入漂移(必须确定性):\n got  %+v\n base %+v", i, got, base)
		}
	}
	// 脱敏载荷经翻译入口同样出摘要,脱敏值原样留在透析预览里供人工核对。
	tr := TranslateEnvelope(variants[0])
	if tr.Summary == "" || tr.Detail == "" {
		t.Fatalf("脱敏载荷必须仍能出摘要: %+v", tr)
	}
	if !strings.Contains(tr.Detail, "[redacted]") {
		t.Fatalf("脱敏值应保留在透析预览里(翻译不得'还原'或再加工): %s", tr.Detail)
	}
}

// TestPresentUnknownEventKeepsEnvelopeUntouched 呈现只读:调用前后信封逐字段不变,
// method/agent 原文原样保留(呈现层不做任何改写/翻译,也不给方法名编"人话")。
func TestPresentUnknownEventKeepsEnvelopeUntouched(t *testing.T) {
	env := EventEnvelope{
		ID: "ue-7-beef", Sequence: 7, TimestampMs: 42,
		InstallationID: "ins", DeviceID: "dev", ThreadID: "th", TaskID: "task", TurnID: "tn",
		CorrelationID: "corr", SourceAgent: "Weird Agent", SourceMethod: "weird/<Method>-XYZ",
		ProtocolVersion: "9", NormalizedKind: KindUnknownRequest, SafetyClass: SafetyClassUnknown,
		RawPayload: `{"x":"<原始>","y":"[redacted]"}`, DecodedFields: `{"x":"<原始>"}`,
		ProcessingStatus: StatusNew, Note: "n", Revision: 3,
	}
	before := env
	_ = PresentUnknownEvent(env)
	if env != before {
		t.Fatalf("呈现函数不得改动信封(method/agent 等原文必须原样保留):\n got  %+v\n want %+v", env, before)
	}
}

// TestPresentUnknownEventMissingFieldsNoFabrication 缺席值不造值:信封除 kind 外全空,
// 三类 kind 的呈现与满字段信封完全一致(文案来自规则表,不从信封编造);词表外
// (两档"已知")、空、未来新 kind 全空返回 —— 不猜 kind,不编文案,客户端按协议
// 降级显示方法名原文。
func TestPresentUnknownEventMissingFieldsNoFabrication(t *testing.T) {
	full := PresentUnknownEvent(EventEnvelope{
		SourceAgent: "codex", SourceMethod: "brandNewThing", RawPayload: `{"a":1}`,
		NormalizedKind: KindUnknownNotification,
	})
	bare := PresentUnknownEvent(EventEnvelope{NormalizedKind: KindUnknownNotification})
	if !presentEqual(bare, full) {
		t.Fatalf("缺字段不得改变固定文案:\n got  %+v\n want %+v", bare, full)
	}
	for _, kind := range []string{"", "TOTALLY_NEW_KIND", KindKnownProjected, KindKnownPassthrough} {
		got := PresentUnknownEvent(EventEnvelope{NormalizedKind: kind, SourceMethod: "m", RawPayload: `{"a":1}`})
		if !presentEqual(got, UnknownPresentation{}) {
			t.Fatalf("kind=%q 无固定文案,应全空返回(缺席值不造值): %+v", kind, got)
		}
	}
}

// TestPresentationDoesNotTouchLog 呈现只影响摘要层:登记过的条目在呈现之后,
// 处置状态与 revision 原样(呈现不写台账、不影响事件落盘,更谈不上影响主轮次)。
func TestPresentationDoesNotTouchLog(t *testing.T) {
	log := NewUnknownEventLog(t.TempDir())
	env := log.Record(EventEnvelope{SourceAgent: "dsh", SourceMethod: "weird/request", NormalizedKind: KindUnknownRequest})
	if got := PresentUnknownEvent(env); got.PlainTitle == "" || !got.RequiresUser {
		t.Fatalf("登记后的信封应得到完整呈现: %+v", got)
	}
	stored, ok := log.Get(env.ID)
	if !ok {
		t.Fatal("呈现后条目应仍在台账")
	}
	if stored.ProcessingStatus != StatusNew || stored.Revision != 0 {
		t.Fatalf("呈现不得改写处置状态/revision: %+v", stored)
	}
}

// TestTranslateEnvelopeEntry 信封翻译入口:走 TranslateEvent 同一条纯函数路径 ——
// kind 决定处置句,载荷决定字段透析;已知/未知两类都能进。
func TestTranslateEnvelopeEntry(t *testing.T) {
	env := EventEnvelope{
		SourceAgent: "codex", SourceMethod: "brandNewThing",
		NormalizedKind: KindUnknownNotification, RawPayload: `{"threadId":"th-9"}`,
	}
	tr := TranslateEnvelope(env)
	if tr.Summary == "" || !strings.Contains(tr.Detail, "留痕") {
		t.Fatalf("信封翻译应含摘要与处置句: %+v", tr)
	}
	if len(tr.Fields) != 1 || tr.Fields[0].Key != "threadId" {
		t.Fatalf("载荷应被逐字段拆解: %+v", tr.Fields)
	}
	known := TranslateEnvelope(EventEnvelope{
		SourceAgent: "codex", SourceMethod: "turn.completed", NormalizedKind: KindKnownProjected,
		RawPayload: `{"usage":1}`,
	})
	if !strings.Contains(known.Detail, "投影") {
		t.Fatalf("KNOWN_PROJECTED 的处置句应说明已投影: %s", known.Detail)
	}
}

// TestPresentAndTranslateHandlePathologicalEnvelope 病态输入不 panic:超长载荷、
// 深嵌套、非法 JSON、怪字节 —— 呈现与翻译都是纯函数,坏输入只可能得到诚实的
// 降级输出,绝不能把调用方(登记路径/RPC)带崩。
func TestPresentAndTranslateHandlePathologicalEnvelope(t *testing.T) {
	envs := []EventEnvelope{
		{NormalizedKind: KindUnknownNotification, RawPayload: strings.Repeat("长", 10000)},
		{NormalizedKind: KindUnknownRequest, RawPayload: strings.Repeat(`{"n":`, 64) + "1" + strings.Repeat("}", 64)},
		{NormalizedKind: KindUnknownResponse, RawPayload: `{"cut":`},
		{NormalizedKind: "WEIRD\x00KIND", RawPayload: "\x00\xff"},
	}
	for _, env := range envs {
		PresentUnknownEvent(env) // 不 panic 即可
		TranslateEnvelope(env)   // 同上
	}
}
