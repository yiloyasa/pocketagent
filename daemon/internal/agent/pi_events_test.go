// pi_events_test.go —— Pi 事件映射的回归钉子。
//
// 【fixture 来自 0.85.1 真机探针】docs/verify/evidence/pi-p0-probe.md 附录的
// 原始报文。协议层最容易写错的三件事,这里逐条钉死:
//   - text/thinking 是增量(要拼),partialResult 是累积值(要替换);
//   - 审批失败安全:拒绝/超时/断线一律 cancelled,绝不默认放行;
//   - 用量只信 get_session_stats(流式 usage 恒 0)。
package agent

import (
	"encoding/json"
	"testing"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// 探针实录的 message_update/text_delta(原文,含 usage 恒 0 的事实)。
const probeTextDeltaLine = `{"type":"message_update","usage":{"input":0,"output":0,"cacheRead":0,"cacheWrite":0,"totalTokens":0,"cost":{"input":0,"output":0,"cacheRead":0,"cacheWrite":0,"total":0}},"assistantMessageEvent":{"type":"text_delta","contentIndex":1,"delta":"探针成功"}}`

func TestMapTextDelta(t *testing.T) {
	d, ok := MapTextDelta(json.RawMessage(probeTextDeltaLine))
	if !ok || d.Delta != "探针成功" {
		t.Fatalf("text_delta 映射错误: ok=%v delta=%q", ok, d.Delta)
	}
	// 非 text_delta 类型必须拒绝(比如把 thinking 误当文本)
	th := `{"type":"message_update","assistantMessageEvent":{"type":"thinking_delta","contentIndex":0,"delta":"think"}}`
	if _, ok := MapTextDelta(json.RawMessage(th)); ok {
		t.Fatal("thinking_delta 不应被映射成 TextDelta")
	}
}

func TestMapThinkingDelta(t *testing.T) {
	// 探针实录:zai 网关 thinking 先于 text,逐词到达
	line := `{"type":"message_update","assistantMessageEvent":{"type":"thinking_delta","contentIndex":0,"delta":"The"}}`
	d, ok := MapThinkingDelta(json.RawMessage(line))
	if !ok || d.Delta != "The" {
		t.Fatalf("thinking_delta 映射错误: ok=%v delta=%q", ok, d.Delta)
	}
}

// toolcall_end → STARTED 帧:参数以 toolCall.arguments 为准(拼装完成的完整对象)。
func TestMapToolcallEndStartsCard(t *testing.T) {
	line := `{"type":"message_update","assistantMessageEvent":{"type":"toolcall_end","contentIndex":1,
		"toolCall":{"id":"call_3ebd2d7bd97a4e039463a11a","name":"bash","arguments":{"command":"for i in $(seq 1 15); do echo line-$i; done"}}}}`
	upd, ok := MapToolcallEnd(json.RawMessage(line))
	if !ok {
		t.Fatal("toolcall_end 应映射成功")
	}
	if upd.Phase != pb.ToolCallUpdate_STARTED || upd.Call.Id != "call_3ebd2d7bd97a4e039463a11a" || upd.Call.Name != "bash" {
		t.Fatalf("STARTED 帧字段错误: phase=%v id=%q name=%q", upd.Phase, upd.Call.Id, upd.Call.Name)
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(upd.Call.JsonArgs), &args); err != nil || args["command"] == "" {
		t.Fatalf("JsonArgs 应为完整参数对象: %q err=%v", upd.Call.JsonArgs, err)
	}
}

// partialResult 累积语义:UPDATE 帧的 Output 是"到目前为止的完整输出",
// 后一帧直接替换前一帧(绝不追加 —— 追加会把 15 行输出变成 120 行)。
func TestMapToolExecUpdateIsCumulativeReplacement(t *testing.T) {
	first := `{"type":"tool_execution_update","toolCallId":"call_1","toolName":"bash","partialResult":{"content":[{"type":"text","text":"line-1\nline-2\n"}]}}`
	second := `{"type":"tool_execution_update","toolCallId":"call_1","toolName":"bash","partialResult":{"content":[{"type":"text","text":"line-1\nline-2\nline-3\n"}]}}`
	// elapsed=0(无脚注)才能做包含断言;带脚注的形状在 display 测试里钉。
	u1, ok1 := MapToolExecUpdate(json.RawMessage(first), piToolClip, 0)
	u2, ok2 := MapToolExecUpdate(json.RawMessage(second), piToolClip, 0)
	if !ok1 || !ok2 {
		t.Fatalf("update 映射失败: %v %v", ok1, ok2)
	}
	if u1.Phase != pb.ToolCallUpdate_UPDATE || u2.Phase != pb.ToolCallUpdate_UPDATE {
		t.Fatalf("相位必须是 UPDATE: %v %v", u1.Phase, u2.Phase)
	}
	if !contains(u2.Result.Output, u1.Result.Output) {
		t.Fatalf("partialResult 是累积值,后帧(%q)应包含前帧(%q)", u2.Result.Output, u1.Result.Output)
	}
	if u2.Result.CallId != u1.Result.CallId {
		t.Fatalf("同一 callId 才能原地替换: %q vs %q", u1.Result.CallId, u2.Result.CallId)
	}
}

func TestMapToolExecEndIsError(t *testing.T) {
	okEnd := `{"type":"tool_execution_end","toolCallId":"call_1","toolName":"bash","isError":false,"result":{"content":[{"type":"text","text":"done"}]}}`
	badEnd := `{"type":"tool_execution_end","toolCallId":"call_2","toolName":"bash","isError":true,"result":{"content":[{"type":"text","text":"boom"}]}}`
	u1, ok1 := MapToolExecEnd(json.RawMessage(okEnd), piToolClip, 0)
	u2, ok2 := MapToolExecEnd(json.RawMessage(badEnd), piToolClip, 0)
	if !ok1 || !ok2 || !u1.Result.Ok || u2.Result.Ok {
		t.Fatalf("FINISHED 帧 Ok 语义错误: %v %v %v %v", ok1, ok2, u1.Result.Ok, u2.Result.Ok)
	}
}

// 用量:get_session_stats 的 diff 语义 + contextWindow(流式 usage 恒 0 的正解)。
func TestUsageUpdateFromStats(t *testing.T) {
	baseRaw := `{"tokens":{"input":50000,"output":10000,"cacheRead":40000,"cacheWrite":5000,"total":105000},"cost":{"total":0.45},"contextUsage":{"tokens":60000,"contextWindow":200000,"percent":30}}`
	curRaw := `{"tokens":{"input":65000,"output":12000,"cacheRead":44000,"cacheWrite":5000,"total":126000},"cost":{"total":0.50},"contextUsage":{"tokens":62000,"contextWindow":200000,"percent":31}}`
	base, cur := parseSessionStats(json.RawMessage(baseRaw)), parseSessionStats(json.RawMessage(curRaw))
	if base == nil || cur == nil {
		t.Fatal("stats 解析失败")
	}
	upd := UsageUpdateFromStats(cur, base)
	if upd.Usage.InputTokens != 15000 || upd.Usage.OutputTokens != 2000 {
		t.Fatalf("本轮增量错误: %+v", upd.Usage)
	}
	if upd.TotalUsage.InputTokens != 65000 {
		t.Fatalf("会话累计错误: %+v", upd.TotalUsage)
	}
	if upd.ModelContextWindow != 200000 {
		t.Fatalf("真实窗口应来自 contextUsage.contextWindow: %v", upd.ModelContextWindow)
	}
	// 无 baseline(轮首拿不到 stats):全量兜底,不炸
	full := UsageUpdateFromStats(cur, nil)
	if full.Usage.InputTokens != 65000 {
		t.Fatalf("无 baseline 时应报会话累计: %+v", full.Usage)
	}
	// 压缩后 contextUsage.tokens 为 null(实测口径):窗口照给
	nullTok := `{"tokens":{"input":1,"output":1,"cacheRead":0,"cacheWrite":0,"total":2},"contextUsage":{"tokens":null,"contextWindow":200000,"percent":null}}`
	st := parseSessionStats(json.RawMessage(nullTok))
	if st == nil || st.ContextUsage == nil {
		t.Fatal("tokens=null 的 contextUsage 应可解析")
	}
}

// 状态事件:压缩/重试/排队有明确状态(否则压缩期的静默会被当成卡死)。
func TestMapStatusEvent(t *testing.T) {
	cases := []struct {
		evType string
		raw    string
		kind   pb.CodexEvent_Kind
	}{
		{"compaction_start", `{"reason":"threshold"}`, pb.CodexEvent_CONTEXT_COMPACTED},
		{"auto_retry_start", `{"attempt":2,"maxAttempts":3,"errorMessage":"529 overloaded"}`, pb.CodexEvent_RETRY_STARTED},
		{"queue_update", `{"steering":["a"],"followUp":["b","c"]}`, pb.CodexEvent_QUEUE},
		{"extension_error", `{"extensionPath":"/x.ts","event":"tool_call","error":"boom"}`, pb.CodexEvent_ERROR},
	}
	for _, c := range cases {
		ev, ok := MapStatusEvent(c.evType, json.RawMessage(c.raw))
		if !ok || ev.Kind != c.kind {
			t.Fatalf("%s 映射错误: ok=%v kind=%v", c.evType, ok, ev.Kind)
		}
	}
	if _, ok := MapStatusEvent("agent_start", json.RawMessage(`{}`)); ok {
		t.Fatal("agent_start 不应被映射成状态事件")
	}
}

// 审批失败安全:拒绝一律 cancelled(true);confirm 放行回 confirmed。
func TestUIResponseFromDecisionCancelled(t *testing.T) {
	resp := UIResponseFromDecision("confirm", false, "")
	if resp["cancelled"] != true {
		t.Fatalf("拒绝必须回 cancelled:true,得到 %v", resp)
	}
	// 放行的 confirm
	resp = UIResponseFromDecision("confirm", true, "")
	if resp["confirmed"] != true {
		t.Fatalf("放行 confirm 应回 confirmed:true,得到 %v", resp)
	}
}

// 【P1 缺陷已移除的钉子】select 必须等用户真实选择(答案经 note 回传),
// 拿不到真实选择就回 cancelled —— 绝不静默替用户选第一项。
func TestUIResponseFromDecisionSelectWaitsRealChoice(t *testing.T) {
	// Android 反问表单提交形状 {"<问题id>":{"answers":["…"]}}:取用户点的那项
	resp := UIResponseFromDecision("select", true, `{"q0":{"answers":["Block"]}}`)
	if resp["value"] != "Block" {
		t.Fatalf("select 应回用户实际点选的选项,得到 %v", resp)
	}
	// 裸字符串提交(兜底解析按 q0)同样算真实选择
	resp = UIResponseFromDecision("select", true, "Allow")
	if resp["value"] != "Allow" {
		t.Fatalf("select 裸文本答案应回 value,得到 %v", resp)
	}
	// 批准但没带回任何答案:cancelled —— 宁可拒绝也不替用户做决定
	resp = UIResponseFromDecision("select", true, "")
	if resp["cancelled"] != true {
		t.Fatalf("select 无答案必须 cancelled,绝不取第一选项,得到 %v", resp)
	}
	// 拒绝/超时/断线:cancelled
	resp = UIResponseFromDecision("select", false, `{"q0":{"answers":["Allow"]}}`)
	if resp["cancelled"] != true {
		t.Fatalf("select 拒绝必须 cancelled,得到 %v", resp)
	}
}

// input/editor:value = 用户输入;批准且未输入 = 用户明确确认空值,放行 ""。
func TestUIResponseFromDecisionInputEditor(t *testing.T) {
	resp := UIResponseFromDecision("input", true, `{"q0":{"answers":["自定义目录"]}}`)
	if resp["value"] != "自定义目录" {
		t.Fatalf("input 应回用户输入,得到 %v", resp)
	}
	// 批准但没输入 = 明确确认空值:必须带 value 键(缺失会让扩展侧拿到 undefined)
	for _, m := range []string{"input", "editor"} {
		resp = UIResponseFromDecision(m, true, "")
		if v, ok := resp["value"]; !ok || v != "" {
			t.Fatalf("%s 批准未输入应回空串 value,得到 %v", m, resp)
		}
	}
}

// piUIBudget 钉死 120s:扩展 dialog 无 timeout 时会永久挂起整轮(调研 §4.6),
// daemon 侧预算是"超时→cancelled"的唯一防线,被人调大/删掉必须在单测里先炸。
func TestPiUIBudgetIs120s(t *testing.T) {
	if piUIBudget != 120*time.Second {
		t.Fatalf("piUIBudget = %v, want 120s(超时回 cancelled 的契约)", piUIBudget)
	}
}

func TestApprovalCallFromUI(t *testing.T) {
	// confirm 保持专用确认卡(手机按"允许/拒绝"渲染)
	call := ApprovalCallFromUI("uuid-1", "confirm", "Dangerous!", "Allow rm -rf?", nil, "", "")
	if call.Id != "uuid-1" || call.Name != "pi_ui_confirm" {
		t.Fatalf("审批卡动作字段错误: id=%q name=%q", call.Id, call.Name)
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(call.JsonArgs), &args); err != nil || args["method"] != "confirm" {
		t.Fatalf("JsonArgs 应带 method: %v", err)
	}
}

// select 选项必须完整透传进 request_user_input 反问卡(不再"自动取第一项"
// 的前提是用户看得见全部选项)。
func TestApprovalCallFromUISelectPassesOptions(t *testing.T) {
	call := ApprovalCallFromUI("uuid-2", "select", "选个方案", "要哪种部署方式?", []string{"方案A", "方案B"}, "", "")
	if call.Id != "uuid-2" || call.Name != toolNameAskUser {
		t.Fatalf("select 应回反问卡(%s): id=%q name=%q", toolNameAskUser, call.Id, call.Name)
	}
	var args struct {
		Message   string `json:"message"`
		Questions []struct {
			ID       string `json:"id"`
			Question string `json:"question"`
			Options  []struct {
				Label string `json:"label"`
			} `json:"options"`
		} `json:"questions"`
	}
	if err := json.Unmarshal([]byte(call.JsonArgs), &args); err != nil {
		t.Fatalf("JsonArgs 解析失败: %v", err)
	}
	if len(args.Questions) != 1 || args.Questions[0].ID != "q0" {
		t.Fatalf("应恰好一条 q0 问题: %+v", args.Questions)
	}
	if args.Questions[0].Question != "选个方案" {
		t.Fatalf("题干应取 title: %q", args.Questions[0].Question)
	}
	if len(args.Questions[0].Options) != 2 ||
		args.Questions[0].Options[0].Label != "方案A" || args.Questions[0].Options[1].Label != "方案B" {
		t.Fatalf("两个选项都必须透传上屏: %+v", args.Questions[0].Options)
	}
}

// input/editor:不给选项(手机渲染为纯输入框),题干照常透传。
func TestApprovalCallFromUIInputNoOptions(t *testing.T) {
	for _, m := range []string{"input", "editor"} {
		call := ApprovalCallFromUI("uuid-3", m, "补充说明", "", nil, "", "")
		if call.Name != toolNameAskUser {
			t.Fatalf("%s 应回反问卡,得到 %q", m, call.Name)
		}
		var args struct {
			Questions []struct {
				Options []map[string]string `json:"options"`
			} `json:"questions"`
		}
		if err := json.Unmarshal([]byte(call.JsonArgs), &args); err != nil {
			t.Fatalf("JsonArgs 解析失败: %v", err)
		}
		if len(args.Questions) != 1 || len(args.Questions[0].Options) != 0 {
			t.Fatalf("%s 不应带选项: %+v", m, args.Questions)
		}
	}
}

// 【保真度·多选答案】手机多选直接提交数组形状 note,解析器必须接受:
// map 内数组值 / 裸顶层数组都归一;旧形状({"answers":[...]}、裸字符串)不回归;
// 非法 JSON 仍兜底"整段文本答第一题"。
func TestAnswerNoteMapArrayShape(t *testing.T) {
	// map 内数组值(多选)
	got := answerNoteMap(`{"q0":["方案A","方案B"]}`)
	if got["q0"] != "方案A, 方案B" {
		t.Fatalf("数组形状答案未解析: %v", got)
	}
	// 顶层数组(单问题卡多选直接提交数组)
	got = answerNoteMap(`["方案A","方案B"]`)
	if got["q0"] != "方案A, 方案B" {
		t.Fatalf("顶层数组未落到 q0: %v", got)
	}
	// 旧形状 1:{"q0":{"answers":["方案A"]}}
	got = answerNoteMap(`{"q0":{"answers":["方案A"]}}`)
	if got["q0"] != "方案A" {
		t.Fatalf("旧 answers 形状回归: %v", got)
	}
	// 旧形状 2:裸字符串值
	got = answerNoteMap(`{"q0":"方案A"}`)
	if got["q0"] != "方案A" {
		t.Fatalf("裸字符串值回归: %v", got)
	}
	// 非法 JSON:整段文本兜底答 q0(语义不变)
	got = answerNoteMap("就用方案A")
	if got["q0"] != "就用方案A" {
		t.Fatalf("自由文本兜底回归: %v", got)
	}
	// firstAnswerFromNote 便捷版同样接受数组形状
	if v := firstAnswerFromNote(`{"q0":["方案A","方案B"]}`); v != "方案A, 方案B" {
		t.Fatalf("firstAnswerFromNote 数组形状未解析: %q", v)
	}
}

// agent_end 里的 usage 提取(完整消息才有真实用量 —— 探针 §11#1)。
// 【cost 形状勘误(2026-09-25)】实测报文(0.85.1/0.87.1 探针)与官方 json.md 里
// usage.cost 都是对象 {"input":…,"total":…},不是数字 —— 旧夹具写成 "cost":0.001
// 是在给"cost 按 float64 解析"的实现 bug 背书(整条 unmarshal 失败、兜底恒 nil);
// 解析修复见 pi.go 的 usageFromAgentEnd,夹具改回真实线上形状。
func TestUsageFromAgentEnd(t *testing.T) {
	line := `{"type":"agent_end","messages":[
		{"role":"user","content":[{"type":"text","text":"hi"}]},
		{"role":"assistant","usage":{"input":100,"output":50,"cacheRead":10,"cacheWrite":5,"cost":{"input":0.0009,"output":0.0001,"cacheRead":0,"cacheWrite":0,"total":0.001}}}]}`
	u := usageFromAgentEnd(json.RawMessage(line))
	if u == nil || u.InputTokens != 100 || u.OutputTokens != 50 || u.CacheReadTokens != 10 {
		t.Fatalf("agent_end usage 提取错误: %+v", u)
	}
	if u.CostUsd < 0.0009 || u.CostUsd > 0.0011 {
		t.Fatalf("cost 应取 cost.total=0.001,得到 %v", u.CostUsd)
	}
	if _, has := any(u).(interface{ ErrorInfo() *pb.Error }); has {
		t.Fatal("usage 不应实现 ErrorInfoCarrier")
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
