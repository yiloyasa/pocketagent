// pi_noise_ledger_test.go —— pi 0.87.1 轮次骨架事件的噪音治理回归钉(尾巴 3)。
//
// 【钉什么】turn_start / message_start / message_end 三类事件在 Run 的事件泵里
// 只落诊断台账、不再向 sink 推"原生状态通知"卡:message_start/end 携带 ~70KB
// system 全量 prompt,且三类每轮必来 —— 推给手机纯属噪音(理由全文见
// pi.go 的 piLedgerOnlyEvent)。本文件钉判定函数与 recordLedgerOnly 的
// "台账必有、sink 必无"两个半边;consume 分支的接线是一行调用,由
// `grep -n "recordLedgerOnly" daemon/internal/agent/pi.go` 静态核对。
package agent

import (
	"encoding/json"
	"testing"

	"github.com/pocketagent/daemon/internal/runtime"
)

func TestPiLedgerOnlyEvents(t *testing.T) {
	for _, et := range []string{"turn_start", "message_start", "message_end"} {
		if !piLedgerOnlyEvent(et) {
			t.Errorf("piLedgerOnlyEvent(%q) = false, want true(0.87.1 噪音治理三类)", et)
		}
	}
	// turn_end 不在列:它每轮只来一次、载荷小,维持原有"限 2 条推送"语义,
	// 不许顺手扩大治理面。
	for _, et := range []string{"turn_end", "agent_start", "entry_appended", ""} {
		if piLedgerOnlyEvent(et) {
			t.Errorf("piLedgerOnlyEvent(%q) = true, want false(不在本轮治理范围)", et)
		}
	}
}

// recordLedgerOnly:台账必落(分类语义与 recordAs 一致),sink 不参与 ——
// 函数签名就没有 sink,连续灌多条也不受"每方法 2 条"推送限频影响
// (限频只约束推送,不约束台账)。
func TestRecordLedgerOnlyWritesIndexWithoutSink(t *testing.T) {
	var notices nativeNoticeRecorder
	notices.threadID = "th-ledger-only"

	for i := 0; i < 5; i++ {
		notices.recordLedgerOnly(piEventNoticeKind("message_start"), "pi", "message_start",
			json.RawMessage(`{"type":"message_start"}`))
	}
	found := 0
	for _, e := range sharedRuntimeBackend().UnknownEvents().List(runtime.ListFilter{AgentID: "pi"}, 0) {
		if e.SourceMethod != "message_start" || e.ThreadID != "th-ledger-only" {
			continue
		}
		found++
		if e.NormalizedKind != runtime.KindKnownPassthrough {
			t.Fatalf("分类应保持 KNOWN_PASSTHROUGH,得到 %q", e.NormalizedKind)
		}
	}
	if found != 5 {
		t.Fatalf("台账应记全 5 条(索引不锁频),实际 %d 条", found)
	}
}
