// unknown_events_summary_test.go —— 摘要注入(时间线模型)的映射钉子
// (2026-10-04 未知事件用户处理中心计划 Task 3)。
//
// 【与 unknown_events_wire_test.go 的分工】那边钉 proto 层"只增不改"的 wire 兼容;
// 这里钉 server 层映射行为本身:
//  1. summary 新字段 12~17 必须与 runtime.PresentUnknownEvent 的确定性输出
//     逐字段一致(映射零损耗,server 层不改写、不过滤、不补充);
//  2. 动作表过 server 层不得长出"允许"类动作 —— UNKNOWN_REQUEST 的安全红线
//     在 server 层再钉一次(展示侧),执行侧另有白名单封口(unknown_actions_test);
//  3. 详情响应字段 7(recommended_next_step)同源映射,字段 8(support_bundle_
//     hint)为 server 直填的固定文案且非空;
//  4. 词表外 kind 新字段全空、不编造;旧字段(1~11)一个不丢 —— 旧客户端兼容。
package server

import (
	"context"
	"strings"
	"testing"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/runtime"
)

// summaryFieldsMatchPresent 断言摘要 12~17 与 PresentUnknownEvent 输出逐字段一致。
func summaryFieldsMatchPresent(t *testing.T, got *pb.UnknownEventSummary, want runtime.UnknownPresentation) {
	t.Helper()
	if got.GetPlainTitle() != want.PlainTitle ||
		got.GetPlainExplanation() != want.PlainExplanation ||
		got.GetImpact() != want.Impact ||
		got.GetRequiresUser() != want.RequiresUser ||
		got.GetCanOpenNative() != want.CanOpenNative {
		t.Fatalf("摘要新字段与 PresentUnknownEvent 输出不一致:\n got  %+v\n want %+v", got, want)
	}
	if len(got.GetActions()) != len(want.Actions) {
		t.Fatalf("动作条数不一致: got %d want %d", len(got.GetActions()), len(want.Actions))
	}
	for i, a := range got.GetActions() {
		w := want.Actions[i]
		if a.GetActionId() != w.ID || a.GetLabel() != w.Label || a.GetDescription() != w.Description ||
			a.GetSafe() != w.Safe || a.GetRequiresConfirmation() != w.RequiresConfirmation ||
			a.GetDestructive() != w.Destructive {
			t.Fatalf("动作 #%d 映射失真:\n got  %+v\n want %+v", i, a, w)
		}
		// daemon 只发安全动作;确认/破坏位在当前动作表里恒为 false,
		// 任何一位翻红都意味着动作语义被改过。
		if !a.GetSafe() || a.GetDestructive() {
			t.Fatalf("动作 %q 的安全声明不对: %+v", a.GetActionId(), a)
		}
	}
}

func TestUnknownEventSummaryMatchesPresentation(t *testing.T) {
	srv := newUnknownTestServer(t)
	ctx := context.Background()
	log := srv.runtimeBackend.UnknownEvents()

	// 三类 kind 各录一条;Record 返回补全后的信封,呈现函数吃的正是它。
	// 【request 必须显式声明 kind】Record 对空 kind 的默认是 UNKNOWN_NOTIFICATION,
	// 而本用例后半段按 request 行为断言(requires_user/无允许类动作)。
	envs := map[string]runtime.EventEnvelope{}
	for _, env := range []runtime.EventEnvelope{
		{SourceAgent: "pi", SourceMethod: "brandNewNotify"},
		{SourceAgent: "zcode", SourceMethod: "native/remote.exec", ThreadID: "th-1", NormalizedKind: runtime.KindUnknownRequest},
		{SourceAgent: "dsh", SourceMethod: "late.reply", CorrelationID: "resp-77"},
	} {
		envs[env.SourceMethod] = log.Record(env)
	}

	resp, err := srv.ListUnknownEvents(ctx, &pb.ListUnknownEventsRequest{})
	if err != nil {
		t.Fatalf("ListUnknownEvents 失败: %v", err)
	}
	byMethod := map[string]*pb.UnknownEventSummary{}
	for _, item := range resp.GetItems() {
		byMethod[item.GetSourceMethod()] = item
	}
	if len(byMethod) != 3 {
		t.Fatalf("应回 3 条,得到 %d", len(byMethod))
	}

	for method, env := range envs {
		got := byMethod[method]
		if got == nil {
			t.Fatalf("缺 %s 的列表项", method)
		}
		summaryFieldsMatchPresent(t, got, runtime.PresentUnknownEvent(env))
		// 旧字段与新字段并存不互斥:既有字段一个不少(旧客户端兼容的前提)。
		if got.GetEventId() != env.ID || got.GetSequence() != env.Sequence ||
			got.GetSourceAgent() != env.SourceAgent || got.GetThreadId() != env.ThreadID ||
			got.GetNormalizedKind() != env.NormalizedKind || got.GetSafetyClass() != env.SafetyClass ||
			got.GetProcessingStatus() != runtime.StatusNew || got.GetSummary() == "" {
			t.Fatalf("旧字段(1~11)因新字段注入而丢失: %+v", got)
		}
	}

	// 各 kind 的动作 ID 集合钉死(与 runtime 动作表一致,下划线口径)。
	wantIDs := map[string][]string{
		runtime.KindUnknownNotification: {"inspect", "copy", "acknowledge"},
		runtime.KindUnknownRequest:      {"inspect", "copy", "open_native", "retry_later"},
		runtime.KindUnknownResponse:     {"inspect", "copy"},
	}
	for method, env := range envs {
		got := byMethod[method]
		want := wantIDs[env.NormalizedKind]
		if len(got.GetActions()) != len(want) {
			t.Fatalf("%s 动作条数应为 %d: %+v", env.NormalizedKind, len(want), got.GetActions())
		}
		for i, id := range want {
			if got.GetActions()[i].GetActionId() != id {
				t.Fatalf("%s 动作 #%d 应为 %q,得到 %q", env.NormalizedKind, i, id, got.GetActions()[i].GetActionId())
			}
		}
	}

	// 安全红线在 server 层再钉一次:UNKNOWN_REQUEST 的动作表里没有"允许"类,
	// 也没有任何确认/破坏位翻红的动作(UNKNOWN_REQUEST 默认不显示"允许"按钮)。
	req := byMethod["native/remote.exec"]
	if !req.GetRequiresUser() || !req.GetCanOpenNative() {
		t.Fatalf("未知请求应进「需要我处理」且可转电脑端: %+v", req)
	}
	for _, a := range req.GetActions() {
		id := strings.ToLower(a.GetActionId())
		for _, banned := range []string{"allow", "approve", "confirm", "execute", "grant"} {
			if strings.Contains(id, banned) {
				t.Fatalf("未知请求的动作表出现了允许类动作 %q", a.GetActionId())
			}
		}
	}
}

func TestUnknownEventDetailCarriesNextStepAndSupportHint(t *testing.T) {
	srv := newUnknownTestServer(t)
	ctx := context.Background()
	log := srv.runtimeBackend.UnknownEvents()
	e := log.Record(runtime.EventEnvelope{
		SourceAgent:    "zcode",
		SourceMethod:   "native/remote.exec",
		ThreadID:       "th-1",
		NormalizedKind: runtime.KindUnknownRequest, // 断言的是未知请求行为(下一步指向电脑端)
	})

	got, err := srv.GetUnknownEvent(ctx, &pb.GetUnknownEventRequest{EventId: e.ID})
	if err != nil {
		t.Fatalf("GetUnknownEvent 失败: %v", err)
	}
	// 字段 7 与 PresentUnknownEvent 的建议下一步同源。
	want := runtime.PresentUnknownEvent(e)
	if got.GetRecommendedNextStep() != want.RecommendedNextStep {
		t.Fatalf("recommended_next_step 映射失真:\n got  %q\n want %q", got.GetRecommendedNextStep(), want.RecommendedNextStep)
	}
	if !strings.Contains(got.GetRecommendedNextStep(), "电脑端") {
		t.Fatalf("未知请求的下一步应指向电脑端: %q", got.GetRecommendedNextStep())
	}
	// 字段 8:server 直填的固定文案,非空且指到 correlation id。
	if got.GetSupportBundleHint() == "" || !strings.Contains(got.GetSupportBundleHint(), "correlation id") {
		t.Fatalf("support_bundle_hint 应为固定文案并提到 correlation id: %q", got.GetSupportBundleHint())
	}
	// 详情里的摘要与列表同一映射(两个入口一份产出)。
	summaryFieldsMatchPresent(t, got.GetSummary(), want)
}

// TestUnknownEventSummaryDegradesWithoutFabrication 词表外 kind(KNOWN_PROJECTED)
// 的降级:新字段全空、不编造文案,旧字段一个不丢 —— 旧客户端照常可用,
// 新客户端按协议降级显示方法名原文。
func TestUnknownEventSummaryDegradesWithoutFabrication(t *testing.T) {
	srv := newUnknownTestServer(t)
	ctx := context.Background()
	log := srv.runtimeBackend.UnknownEvents()
	e := log.Record(runtime.EventEnvelope{
		SourceAgent:    "codex",
		SourceMethod:   "turn.completed",
		NormalizedKind: runtime.KindKnownProjected,
	})

	resp, err := srv.ListUnknownEvents(ctx, &pb.ListUnknownEventsRequest{})
	if err != nil {
		t.Fatalf("ListUnknownEvents 失败: %v", err)
	}
	if len(resp.GetItems()) != 1 {
		t.Fatalf("应回 1 条,得到 %d", len(resp.GetItems()))
	}
	got := resp.GetItems()[0]
	// 新字段(12~17)全空:没有固定文案就不造,客户端不占位。
	if got.GetPlainTitle() != "" || got.GetPlainExplanation() != "" || got.GetImpact() != "" ||
		got.GetRequiresUser() || got.GetCanOpenNative() || len(got.GetActions()) != 0 {
		t.Fatalf("词表外 kind 不得编造呈现: %+v", got)
	}
	// 旧字段(1~11)一个不少:既有字段不因新字段缺席而丢。
	if got.GetEventId() != e.ID || got.GetSequence() != e.Sequence ||
		got.GetSourceAgent() != "codex" || got.GetSourceMethod() != "turn.completed" ||
		got.GetNormalizedKind() != runtime.KindKnownProjected ||
		got.GetSafetyClass() != runtime.SafetyClassUnknown ||
		got.GetProcessingStatus() != runtime.StatusNew || got.GetSummary() == "" {
		t.Fatalf("旧字段应完整保留: %+v", got)
	}

	// 详情:建议下一步不编造(空 = 客户端不占位),支持包提示与 kind 无关、恒非空。
	detail, err := srv.GetUnknownEvent(ctx, &pb.GetUnknownEventRequest{EventId: e.ID})
	if err != nil {
		t.Fatalf("GetUnknownEvent 失败: %v", err)
	}
	if detail.GetRecommendedNextStep() != "" {
		t.Fatalf("词表外 kind 不得编造下一步: %q", detail.GetRecommendedNextStep())
	}
	if detail.GetSupportBundleHint() == "" {
		t.Fatalf("support_bundle_hint 与 kind 无关,应恒非空")
	}
}
