// pi_0991_adapt_test.go —— pi 0.99.1 协议适配的回归钉子。
//
// 【证据】2026-09-30 探针(docs/verify/evidence 待归档的 pi-0991-p0/m99/native
// 四份记录;活体报文摘录直接嵌在夹具里)。0.99.1 相对 0.87.1 的适配面:
//   - 事件全集新增 bash_execution_update(RPC bash 留观未接)+ thinking_level_changed
//     补收编(此前 GlobalHandler/兜底路径落 UNKNOWN);
//   - compaction_end 新形状 aborted/willRetry/errorMessage(旧实现一律报"已压缩");
//   - auto_retry_start 新字段 delayMs、auto_retry_end 新字段 attempt;
//   - get_state 新增 sessionName 键(原生设置只读目录补一行)。
package agent

import (
	"encoding/json"
	"testing"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/runtime"
)

// 0.99.1 事件全集的已知分档:新事件进 KNOWN_PASSTHROUGH(透析库有词条,不淹诊断页),
// 从未见过的类型仍落 UNKNOWN。
func TestPiEventNoticeKind0991(t *testing.T) {
	for _, et := range []string{"bash_execution_update", "thinking_level_changed"} {
		if got := piEventNoticeKind(et); got != runtime.KindKnownPassthrough {
			t.Errorf("piEventNoticeKind(%q) = %q, want PASSTHROUGH", et, got)
		}
	}
	if got := piEventNoticeKind("brand_new_0992_event"); got != runtime.KindUnknownNotification {
		t.Errorf("未见过的事件应落 UNKNOWN,得到 %q", got)
	}
}

// compaction_end 四态:成功(含溢出恢复 willRetry)/中止/失败 —— 0.99.1 语义下
// 把中止或失败冒充"已压缩"是事实错误。
func TestMapStatusEventCompactionEnd0991(t *testing.T) {
	cases := []struct {
		name     string
		raw      string
		wantCode string
		wantSub  string // 标题或正文必须包含的关键词
	}{
		{"成功", `{}`, "completed", "上下文已压缩"},
		{"溢出恢复成功并重试", `{"reason":"overflow","result":{"tokensBefore":9},"aborted":false,"willRetry":true}`, "completed", "重试刚才的任务"},
		{"被中止", `{"reason":"manual","aborted":true}`, "aborted", "已中止"},
		{"失败", `{"reason":"threshold","aborted":false,"errorMessage":"summary model 5xx"}`, "failed", "压缩失败"},
	}
	for _, c := range cases {
		ev, ok := MapStatusEvent("compaction_end", json.RawMessage(c.raw))
		if !ok {
			t.Fatalf("%s: compaction_end 应可映射", c.name)
		}
		if ev.Kind != pb.CodexEvent_CONTEXT_COMPACTED || ev.Code != c.wantCode {
			t.Fatalf("%s: kind/code 错误: kind=%v code=%q", c.name, ev.Kind, ev.Code)
		}
		if !contains(ev.Title, c.wantSub) && !contains(ev.Message, c.wantSub) {
			t.Fatalf("%s: 标题/正文缺关键词 %q: title=%q message=%q", c.name, c.wantSub, ev.Title, ev.Message)
		}
	}
}

// auto_retry_start 的 0.99.1 新字段 delayMs:重试等待进入人话(约 N 秒后),
// 不再让"重试间隔"在手机端被误读成卡死;缺字段(旧版帧)零分叉。
func TestMapStatusEventAutoRetryStartDelayMs(t *testing.T) {
	ev, ok := MapStatusEvent("auto_retry_start", json.RawMessage(
		`{"attempt":1,"maxAttempts":3,"delayMs":2000,"errorMessage":"529 overloaded"}`))
	if !ok || ev.Kind != pb.CodexEvent_RETRY_STARTED {
		t.Fatalf("auto_retry_start 映射错误: ok=%v kind=%v", ok, ev.Kind)
	}
	if !contains(ev.Title, "约 2 秒后") {
		t.Fatalf("delayMs=2000 应折算进标题: %q", ev.Title)
	}
	if !ev.WillRetry || ev.Attempt != 1 {
		t.Fatalf("willRetry/attempt 语义错误: %+v", ev)
	}
	// 旧形状(无 delayMs)零分叉:标题不带"秒后"。
	old, _ := MapStatusEvent("auto_retry_start", json.RawMessage(`{"attempt":2,"maxAttempts":3}`))
	if contains(old.Title, "秒后") {
		t.Fatalf("旧形状不应出现等待秒数: %q", old.Title)
	}
}

// auto_retry_end 的 0.99.1 新字段 attempt:带上重试序号;成功/失败文案不变。
func TestMapStatusEventAutoRetryEndAttempt(t *testing.T) {
	ok, _ := MapStatusEvent("auto_retry_end", json.RawMessage(`{"success":true,"attempt":2}`))
	if !contains(ok.Title, "第 2 次") || !contains(ok.Message, "重试成功") {
		t.Fatalf("成功帧应带序号与成功文案: title=%q message=%q", ok.Title, ok.Message)
	}
	bad, _ := MapStatusEvent("auto_retry_end", json.RawMessage(`{"success":false,"attempt":3,"finalError":"gone"}`))
	if !contains(bad.Message, "gone") {
		t.Fatalf("失败帧应带 finalError: %q", bad.Message)
	}
}

// get_state 的 0.99.1 新键 sessionName:在场才出只读行(缺席 = 原生 unset,
// 绝不伪造);既有两行(autoCompactionEnabled/steeringMode)次序不变。
func TestPiNativeStateRows0991SessionName(t *testing.T) {
	with := piNativeStateRows(map[string]any{
		"autoCompactionEnabled": true,
		"steeringMode":          "all",
		"sessionName":           "piprobe",
	})
	if len(with) != 3 {
		t.Fatalf("sessionName 在场应出三行,得 %d: %+v", len(with), with)
	}
	last := with[2]
	if last.ID != "sessionName" || last.Writable || last.CurrentValue != "piprobe" {
		t.Fatalf("sessionName 行形状错误: %+v", last)
	}
	if last.Note == "" {
		t.Fatal("sessionName 行必须带人话说明(双端同步语义)")
	}
	// 缺席/空值:不出行(不伪造)。
	for name, state := range map[string]map[string]any{
		"缺席": {"autoCompactionEnabled": true, "steeringMode": "all"},
		"空值": {"autoCompactionEnabled": true, "steeringMode": "all", "sessionName": " "},
	} {
		if rows := piNativeStateRows(state); len(rows) != 2 {
			t.Fatalf("%s: 应保持两行,得 %d: %+v", name, len(rows), rows)
		}
	}
}
