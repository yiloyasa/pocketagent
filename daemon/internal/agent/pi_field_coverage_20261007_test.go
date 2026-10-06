// pi_field_coverage_20261007_test.go —— pi 1.0.2 字段级全适配审计的补齐钉子。
//
// 【事实源】本机实装 pi 1.0.2(G:\…\@earendil-works/pi-coding-agent,只读核对,
// 未运行):随包 docs/rpc-commands.md / docs/json.md 实样 + dist 逐行
// (usage-totals.js / agent-session.js / rpc-types.d.ts / modes/rpc/rpc-mode.js)。
// 每个夹具都注明出处;形状与 1.0.2 逐字节一致,历史形状(≤0.99.x)另钉防回滚。
//
// 覆盖矩阵全文见 docs/reviews/2026-10-07/field-coverage-pi.md(G1..G6 编号
// 与本文件一一对应)。
package agent

import (
	"encoding/json"
	"strings"
	"testing"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/runtime"
)

// 【G1】pi 1.0.2 的 get_session_stats data.cost 是**纯数字**
// (docs/rpc-commands.md:536-559 实样逐字节;dist/core/usage-totals.js:
// createUsageTotals cost:0 + addUsageToTotals totals.cost += usage.cost.total)。
// 旧解析按对象硬解 → 整帧 unmarshal 失败 → 用量链路静默哑火。数字形状必须可解,
// ≤0.99.x 的对象形状(0.99.1 探针实帧)不得回滚。
func TestSessionStatsCostNumberShape102(t *testing.T) {
	docs102 := `{"sessionFile":"/path/to/session.jsonl","sessionId":"abc123","userMessages":5,"assistantMessages":5,"toolCalls":12,"toolResults":12,"totalMessages":22,"tokens":{"input":50000,"output":10000,"cacheRead":40000,"cacheWrite":5000,"total":105000},"cost":0.45,"contextUsage":{"tokens":60000,"contextWindow":200000,"percent":30}}`
	cur := parseSessionStats(json.RawMessage(docs102))
	if cur == nil {
		t.Fatal("1.0.2 数字形状 cost 必须可解(整帧解析失败=用量链路哑火)")
	}
	if cur.Cost.Total != 0.45 {
		t.Fatalf("cost 应取裸数字 0.45,得到 %v", cur.Cost.Total)
	}
	if cur.ContextUsage == nil || cur.ContextUsage.ContextWindow != 200000 {
		t.Fatalf("contextUsage 窗口应照常解析: %+v", cur.ContextUsage)
	}
	// 增量口径与旧形状一致(对象形状回归守卫,0.99.1 实帧)
	base := parseSessionStats(json.RawMessage(`{"tokens":{"input":40000,"output":8000,"cacheRead":30000,"cacheWrite":5000,"total":83000},"cost":{"total":0.40},"contextUsage":{"tokens":50000,"contextWindow":200000,"percent":25}}`))
	if base == nil {
		t.Fatal("≤0.99.x 对象形状必须继续可解(防回滚)")
	}
	upd := UsageUpdateFromStats(cur, base)
	if upd.Usage.InputTokens != 10000 || upd.Usage.OutputTokens != 2000 {
		t.Fatalf("本轮增量错误: %+v", upd.Usage)
	}
	if upd.Usage.CostUsd < 0.0499 || upd.Usage.CostUsd > 0.0501 {
		t.Fatalf("跨形状 cost 增量应 0.05,得到 %v", upd.Usage.CostUsd)
	}
}

// 【G2】压缩/分支摘要重试三态(docs/json.md:175-177 实样逐字节):此前只经
// recordAs 限 2 卡后静默,重试等待期会被误读成卡死 —— 现在映射成 RETRY_*
// 状态卡,字段(attempt/maxAttempts/delayMs/errorMessage/source/reason)全透出。
func TestMapStatusEventSummarizationRetryFamily(t *testing.T) {
	scheduled, ok := MapStatusEvent("summarization_retry_scheduled", json.RawMessage(
		`{"type":"summarization_retry_scheduled","attempt":1,"maxAttempts":3,"delayMs":2000,"errorMessage":"terminated"}`))
	if !ok || scheduled.Kind != pb.CodexEvent_RETRY_STARTED || scheduled.Code != "summarization_retry" {
		t.Fatalf("scheduled 应映射 RETRY_STARTED: ok=%v ev=%+v", ok, scheduled)
	}
	if !strings.Contains(scheduled.Title, "1/3") || !strings.Contains(scheduled.Title, "约 2 秒后") {
		t.Fatalf("标题应带次数与延迟: %q", scheduled.Title)
	}
	if !scheduled.WillRetry || scheduled.Attempt != 1 || scheduled.Message != "terminated" {
		t.Fatalf("willRetry/attempt/errorMessage 应透出: %+v", scheduled)
	}

	compaction, ok := MapStatusEvent("summarization_retry_attempt_start", json.RawMessage(
		`{"type":"summarization_retry_attempt_start","source":"compaction","reason":"threshold"}`))
	if !ok || compaction.Kind != pb.CodexEvent_RETRY_STARTED {
		t.Fatalf("attempt_start(compaction) 应映射 RETRY_STARTED: ok=%v ev=%+v", ok, compaction)
	}
	if !strings.Contains(compaction.Title, "上下文摘要") || !strings.Contains(compaction.Message, "threshold") {
		t.Fatalf("compaction 变体应带来源与 reason: %q / %q", compaction.Title, compaction.Message)
	}

	branch, ok := MapStatusEvent("summarization_retry_attempt_start", json.RawMessage(
		`{"type":"summarization_retry_attempt_start","source":"branchSummary"}`))
	if !ok || !strings.Contains(branch.Title, "分支摘要") {
		t.Fatalf("branchSummary 变体(docs/json.md:180 reason 缺席)应区分呈现: %q", branch.Title)
	}

	finished, ok := MapStatusEvent("summarization_retry_finished", json.RawMessage(`{"type":"summarization_retry_finished"}`))
	if !ok || finished.Kind != pb.CodexEvent_RETRY_STOPPED {
		t.Fatalf("finished 应映射 RETRY_STOPPED: ok=%v ev=%+v", ok, finished)
	}
}

// 【G3】fire-and-forget notify 是用户可见通知(rpc-types.d.ts:436-439),按
// notifyType 分级上卡;非 notify method 不吃(照旧走未知请求确认管道)。
func TestMapUINotifyCard(t *testing.T) {
	warn, ok := MapUINotify(json.RawMessage(
		`{"type":"extension_ui_request","id":"c0a4","method":"notify","message":"索引已更新","notifyType":"warning"}`))
	if !ok || warn.Kind != pb.CodexEvent_WARNING || warn.Message != "索引已更新" {
		t.Fatalf("warning 通知应上 WARNING 卡: ok=%v ev=%+v", ok, warn)
	}
	if errv, ok := MapUINotify(json.RawMessage(
		`{"type":"extension_ui_request","id":"c0a5","method":"notify","message":"模型请求失败一次","notifyType":"error"}`)); !ok || errv.Kind != pb.CodexEvent_ERROR {
		t.Fatalf("error 通知应上 ERROR 卡: ok=%v ev=%+v", ok, errv)
	}
	if info, ok := MapUINotify(json.RawMessage(
		`{"type":"extension_ui_request","id":"c0a6","method":"notify","message":"已后台刷新"}`)); !ok || info.Kind != pb.CodexEvent_STATUS {
		t.Fatalf("缺 notifyType 按信息级: ok=%v ev=%+v", ok, info)
	}
	if _, ok := MapUINotify(json.RawMessage(
		`{"type":"extension_ui_request","id":"c0a7","method":"setStatus","statusKey":"lsp","statusText":"ready"}`)); ok {
		t.Fatal("setStatus 不是 notify,必须交回既有管道")
	}
}

// 【G4】input.placeholder / editor.prefill(rpc-types.d.ts:425/432)随反问卡
// JSON 透出;不带提示的老形状逐字节不变(键只在非空时出现)。
func TestApprovalCallFromUIPlaceholderPrefill(t *testing.T) {
	input := ApprovalCallFromUI("u1", "input", "输出目录", "", nil, "例如 dist/windows", "")
	var q struct {
		Questions []struct {
			Placeholder string `json:"placeholder"`
			Prefill     string `json:"prefill"`
		} `json:"questions"`
	}
	if err := json.Unmarshal([]byte(input.JsonArgs), &q); err != nil || len(q.Questions) != 1 {
		t.Fatalf("input 反问卡解析失败: %v", err)
	}
	if q.Questions[0].Placeholder != "例如 dist/windows" || q.Questions[0].Prefill != "" {
		t.Fatalf("placeholder 应透出、prefill 空键不出现: %+v", q.Questions[0])
	}
	editor := ApprovalCallFromUI("u2", "editor", "草稿", "", nil, "", "预填正文")
	if err := json.Unmarshal([]byte(editor.JsonArgs), &q); err != nil {
		t.Fatalf("editor 反问卡解析失败: %v", err)
	}
	if q.Questions[0].Prefill != "预填正文" {
		t.Fatalf("editor.prefill 应透出: %+v", q.Questions[0])
	}
	plain := ApprovalCallFromUI("u3", "input", "补充说明", "", nil, "", "")
	if strings.Contains(plain.JsonArgs, "placeholder") || strings.Contains(plain.JsonArgs, "prefill") {
		t.Fatalf("无提示时老卡形状必须逐字节不变(键不得出现): %s", plain.JsonArgs)
	}
	if confirm := ApprovalCallFromUI("u4", "confirm", "危险操作", "确定?", nil, "忽略", "忽略"); confirm.Name != "pi_ui_confirm" {
		t.Fatalf("confirm 不受提示字段影响: %q", confirm.Name)
	}
}

// 【G5】compaction_end.result(docs/json.md:144-148 实样):completed 态带压缩
// 前后体量;aborted/failed 无 result,消息形状不变。
func TestCompactionEndCarriesResultTokens(t *testing.T) {
	done, ok := MapStatusEvent("compaction_end", json.RawMessage(
		`{"type":"compaction_end","reason":"threshold","result":{"summary":"Summary of conversation...","firstKeptEntryId":"abc123","tokensBefore":150000,"estimatedTokensAfter":32000,"usage":{},"details":{}},"aborted":false,"willRetry":false}`))
	if !ok || done.Code != "completed" {
		t.Fatalf("成功压缩应 completed: ok=%v ev=%+v", ok, done)
	}
	if !strings.Contains(done.Message, "150000") || !strings.Contains(done.Message, "32000") {
		t.Fatalf("completed 态应带压缩前后体量: %q", done.Message)
	}
	retry, ok := MapStatusEvent("compaction_end", json.RawMessage(
		`{"type":"compaction_end","reason":"overflow","result":{"summary":"s","firstKeptEntryId":"e1","tokensBefore":200000,"estimatedTokensAfter":40000},"aborted":false,"willRetry":true}`))
	if !ok || !strings.Contains(retry.Message, "溢出恢复") || !strings.Contains(retry.Message, "200000") {
		t.Fatalf("溢出恢复态应保留原语义并带体量: %q", retry.Message)
	}
	aborted, ok := MapStatusEvent("compaction_end", json.RawMessage(
		`{"type":"compaction_end","reason":"manual","aborted":true,"willRetry":false}`))
	if !ok || aborted.Message != "压缩被中止,本轮继续执行,上下文未变。" {
		t.Fatalf("aborted 态消息形状不得漂移: %q", aborted.Message)
	}
}

// 【G6】0.85.1 时代防御残留的三个事件名不在 1.0.2 全集,但 consume 有真实投影
// (InvalidateModels + Started 帧)—— 分类必须如实记 PROJECTED,不得灌 UNKNOWN。
func TestPiDefensiveSelectionEventsProjected(t *testing.T) {
	for _, et := range []string{"model_changed", "model_select", "state_changed"} {
		if got := piEventNoticeKind(et); got != runtime.KindKnownProjected {
			t.Errorf("piEventNoticeKind(%q) = %q, want PROJECTED(有真实投影分支)", et, got)
		}
	}
}
