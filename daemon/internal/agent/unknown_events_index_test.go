// unknown_events_index_test.go —— 未知事件"登记进诊断台账"的回归用例(规格 §9)。
//
// 【为什么与 codex_item_passthrough_test.go 分开】那个文件钉的是"未知事件有没有
// 推给手机"(推送视角,带限频);本文件钉的是"它有没有被记进可查询的索引"(索引视角,
// 不限频)。两件事的失效原因不同:漏推 → 用户当下看不到;漏记 → 用户事后查不到,
// 而"事后能不能查"正是本轮要补的能力。
package agent

import (
	"encoding/json"
	"testing"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/runtime"
)

// TestUnknownNotificationIsIndexed 未知通知除了推给手机,还必须进诊断台账。
func TestUnknownNotificationIsIndexed(t *testing.T) {
	var notices nativeNoticeRecorder
	notices.threadID = "th-idx"
	var pushed int
	notices.record("codex", "brandNewNotificationXYZ", json.RawMessage(`{"k":"v"}`),
		func(*pb.TurnServerMessage) { pushed++ })
	if pushed != 1 {
		t.Fatalf("仍应推一条诊断事件给手机(既有行为不变),得到 %d", pushed)
	}
	for _, e := range sharedRuntimeBackend().UnknownEvents().List(runtime.ListFilter{AgentID: "codex"}, 0) {
		if e.SourceMethod != "brandNewNotificationXYZ" {
			continue
		}
		if e.ThreadID != "th-idx" {
			t.Fatalf("登记应带会话归属,得到 %q", e.ThreadID)
		}
		if e.NormalizedKind != runtime.KindUnknownNotification {
			t.Fatalf("类别应为 UNKNOWN_NOTIFICATION,得到 %q", e.NormalizedKind)
		}
		if e.RawPayload == "" {
			t.Fatal("应保留脱敏后的原始载荷(否则点开详情什么也看不到)")
		}
		return
	}
	t.Fatal("未知通知必须登记进诊断台账 —— 只推给手机的那条受推送限频,回答不了'到底来了多少次'")
}

// TestUnknownNotificationIndexNotRateLimited 钉住"推送限频 ≠ 台账限频"这条设计取舍。
func TestUnknownNotificationIndexNotRateLimited(t *testing.T) {
	var notices nativeNoticeRecorder
	var pushed int
	sink := func(*pb.TurnServerMessage) { pushed++ }
	for i := 0; i < 5; i++ {
		notices.record("pi", "rateLimitProbeXYZ", json.RawMessage(`{}`), sink)
	}
	if pushed != 2 {
		t.Fatalf("推送仍应限频到每方法 2 条(不能刷屏),得到 %d", pushed)
	}
	n := 0
	for _, e := range sharedRuntimeBackend().UnknownEvents().List(runtime.ListFilter{AgentID: "pi"}, 0) {
		if e.SourceMethod == "rateLimitProbeXYZ" {
			n++
		}
	}
	if n != 5 {
		t.Fatalf("台账应记全 5 条(索引不受推送限频约束),得到 %d", n)
	}
}
