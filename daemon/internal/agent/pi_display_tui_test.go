// pi_display_tui_test.go —— "Pi 原生 TUI 展示水准"差距落地的回归钉子。
//
// 依据(全部实帧,绝不伪造形状):
//   - %TEMP%\pistudy_json_tool.jsonl   pi 0.99.1 --mode json --provider deepseek
//     --model deepseek-chat 的工具调用全帧(tool_execution_update/end 的 args、
//     structuredContent{output,truncated,exit_code,wall_time_seconds});
//   - %TEMP%\pistudy_json_reason.jsonl deepseek-reasoner 的 thinking/text 逐词
//     delta 与终帧 usage;
//   - pi 包内 TUI 渲染源码(思考斜体块/工具卡标题=名+参数摘要/Elapsed/Took/
//     exit/截断警告):dist/modes/interactive/components/assistant-message.js、
//     dist/core/tools/render-utils.js formatToolCallWithArgs、
//     dist/core/tools/renderers/bash.js。
package agent

import (
	"encoding/json"
	"testing"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// 实帧(pistudy_json_tool.jsonl 原文):bash 工具执行中的 UPDATE 帧。
const realToolExecUpdateLine = `{"type":"tool_execution_update","toolCallId":"call_00_ET_3IoJuqmz51HyG45aQmH20066","toolName":"bash","args":{"command":"ls -la","timeout":30},"partialResult":{"content":[]}}`

// 实帧(pistudy_json_tool.jsonl 原文,仅正文换行保留转义):bash 工具 FINISHED 帧,
// structuredContent 自带 exit_code=0 与 wall_time_seconds=0.1。
const realToolExecEndLine = `{"type":"tool_execution_end","toolCallId":"call_00_ET_3IoJuqmz51HyG45aQmH20066","toolName":"bash","result":{"content":[{"type":"text","text":"total 0\n-rw-r--r-- alpha.txt\n"}],"structuredContent":{"output":"total 0","truncated":false,"exit_code":0,"wall_time_seconds":0.1}},"isError":false}`

// 差距①:UPDATE/FINISHED 帧必须带 JsonArgs —— 手机端按 callId 原地替换工具卡时,
// "工具名 + 参数摘要"的标题语义不能丢(pi TUI 标题 = formatToolCallWithArgs)。
func TestToolExecFramesCarryJsonArgs(t *testing.T) {
	upd, ok := MapToolExecUpdate(json.RawMessage(realToolExecUpdateLine), piToolClip, 0)
	if !ok {
		t.Fatal("update 映射失败")
	}
	if upd.Call.JsonArgs == "" {
		t.Fatal("UPDATE 帧丢失 JsonArgs(手机端替换卡片后参数摘要会消失)")
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(upd.Call.JsonArgs), &args); err != nil || args["command"] != "ls -la" {
		t.Fatalf("UPDATE 帧 JsonArgs 形状错误: %q err=%v", upd.Call.JsonArgs, err)
	}

	end, ok := MapToolExecEnd(json.RawMessage(realToolExecEndLine), piToolClip, 0)
	if !ok {
		t.Fatal("end 映射失败")
	}
	if end.Call.JsonArgs == "" {
		t.Fatal("FINISHED 帧丢失 JsonArgs")
	}
}

// 差距②:耗时脚注。FINISHED 帧从 structuredContent 取真实 wall_time
// (对齐 pi TUI "Took 4.1s");exit 0 不标(pi 也只在出错时标 exit)。
func TestToolExecEndFooterUsesStructuredWallTime(t *testing.T) {
	end, ok := MapToolExecEnd(json.RawMessage(realToolExecEndLine), piToolClip, 0)
	if !ok {
		t.Fatal("end 映射失败")
	}
	want := "total 0\n-rw-r--r-- alpha.txt\n\n[用时 0.1s]"
	if end.Result.Output != want {
		t.Fatalf("FINISHED 脚注错误:\n got=%q\nwant=%q", end.Result.Output, want)
	}
}

// 差距②:非零退出码与截断要如实标出(pi TUI 的 "(exit N)" 与 truncation 警告)。
func TestToolExecEndFooterExitAndTruncated(t *testing.T) {
	line := `{"type":"tool_execution_end","toolCallId":"c1","toolName":"bash","isError":true,"result":{"content":[{"type":"text","text":"boom"}],"structuredContent":{"truncated":true,"exit_code":2}}}`
	end, ok := MapToolExecEnd(json.RawMessage(line), piToolClip, 0)
	if !ok {
		t.Fatal("end 映射失败")
	}
	if end.Result.Ok {
		t.Fatal("isError=true 必须 Ok=false")
	}
	// 无 wall_time → 无时长行(elapsed=0);exit 2 与截断如实标出。
	want := "boom\n[退出码 2 · 输出已截断]"
	if end.Result.Output != want {
		t.Fatalf("退出码/截断脚注错误: got=%q want=%q", end.Result.Output, want)
	}
}

// 差距②:structuredContent 缺 wall_time 时回退 daemon 实测 elapsed(真测量);
// 两者都没有 → 无脚注,绝不编造。
func TestToolExecEndFooterElapsedFallback(t *testing.T) {
	line := `{"type":"tool_execution_end","toolCallId":"c2","toolName":"read","isError":false,"result":{"content":[{"type":"text","text":"file body"}]}}`
	end, _ := MapToolExecEnd(json.RawMessage(line), piToolClip, 3250*time.Millisecond)
	if end.Result.Output != "file body\n[用时 3.2s]" {
		t.Fatalf("elapsed 兜底脚注错误: %q", end.Result.Output)
	}
	bare, _ := MapToolExecEnd(json.RawMessage(line), piToolClip, 0)
	if bare.Result.Output != "file body" {
		t.Fatalf("无任何耗时来源时不应有脚注: %q", bare.Result.Output)
	}
}

// 差距②:执行中的 UPDATE 帧带"已运行"脚注(pi TUI 每秒刷的 Elapsed)。
func TestToolExecUpdateRunningFooter(t *testing.T) {
	upd, ok := MapToolExecUpdate(json.RawMessage(realToolExecUpdateLine), piToolClip, 3200*time.Millisecond)
	if !ok {
		t.Fatal("update 映射失败")
	}
	// 实帧此刻 partialResult.content 为空 → 正文空,脚注不黏换行(有正文时才加)。
	if upd.Result.Output != "[已运行 3.2s]" {
		t.Fatalf("UPDATE 脚注错误: %q", upd.Result.Output)
	}
}

// 实帧(pistudy_json_reason.jsonl 原文):deepseek-reasoner 逐词 thinking delta。
var realThinkingDeltas = []string{
	`{"type":"message_update","assistantMessageEvent":{"type":"thinking_delta","contentIndex":0,"delta":"The"}}`,
	`{"type":"message_update","assistantMessageEvent":{"type":"thinking_delta","contentIndex":0,"delta":" user"}}`,
	`{"type":"message_update","assistantMessageEvent":{"type":"thinking_delta","contentIndex":0,"delta":" asks"}}`,
}

// 实帧(pistudy_json_reason.jsonl 原文):逐词 text delta。
var realTextDeltas = []string{
	`{"type":"message_update","assistantMessageEvent":{"type":"text_delta","contentIndex":1,"delta":"##"}}`,
	`{"type":"message_update","assistantMessageEvent":{"type":"text_delta","contentIndex":1,"delta":" "}}`,
	`{"type":"message_update","assistantMessageEvent":{"type":"text_delta","contentIndex":1,"delta":"逐步"}}`,
}

// 差距④:流式分块保真 —— pi 以词级 token 为一块,daemon 必须 1:1 透传
// (不合并、不丢帧);思考与正文走各自独立事件(思考"分卡"在 daemon 侧
// 的呈现即:ThinkingDelta 与 TextDelta 永远是两种帧)。
func TestStreamDeltaFidelity(t *testing.T) {
	for _, raw := range realThinkingDeltas {
		d, ok := MapThinkingDelta(json.RawMessage(raw))
		if !ok || d.Delta == "" {
			t.Fatalf("thinking delta 丢失: %s", raw)
		}
	}
	think, ok := MapThinkingDelta(json.RawMessage(realThinkingDeltas[2]))
	if !ok || think.Delta != " asks" {
		t.Fatalf("thinking 分块粒度被改变: %q", think.Delta)
	}
	for _, raw := range realTextDeltas {
		d, ok := MapTextDelta(json.RawMessage(raw))
		if !ok || d.Delta == "" {
			t.Fatalf("text delta 丢失: %s", raw)
		}
	}
	txt, ok := MapTextDelta(json.RawMessage(realTextDeltas[2]))
	if !ok || txt.Delta != "逐步" {
		t.Fatalf("text 分块粒度被改变: %q", txt.Delta)
	}
	// text 事件绝不能混进思考通道,反之亦然(独立成卡的协议前提)。
	if _, ok := MapTextDelta(json.RawMessage(realThinkingDeltas[0])); ok {
		t.Fatal("thinking_delta 不应被当成正文")
	}
	if _, ok := MapThinkingDelta(json.RawMessage(realTextDeltas[0])); ok {
		t.Fatal("text_delta 不应被当成思考")
	}
}

// 差距③:助手消息 length 截断必须上屏(形状=message_end 实帧形状,stopReason 值
// 取自随包 docs/message-types.md 的官方枚举;本机实帧只抓到 stop,截断帧罕见)。
func TestAssistantStopLengthNotice(t *testing.T) {
	line := `{"type":"message_end","message":{"role":"assistant","content":[{"type":"text","text":"..."}],"stopReason":"length","timestamp":1790730308893}}`
	ev, ok := MapAssistantStop(json.RawMessage(line))
	if !ok {
		t.Fatal("length 收尾应发卡")
	}
	if ev.Kind != pb.CodexEvent_WARNING || ev.Code != "assistant_stop_length" || ev.Title == "" {
		t.Fatalf("length 卡字段错误: %+v", ev)
	}
}

// 差距③:stopReason=error 且带 errorMessage 时把原因带给人看。
func TestAssistantStopErrorNotice(t *testing.T) {
	line := `{"type":"message_end","message":{"role":"assistant","content":[],"stopReason":"error","errorMessage":"quota exceeded","timestamp":1790730308893}}`
	ev, ok := MapAssistantStop(json.RawMessage(line))
	if !ok {
		t.Fatal("error 收尾应发卡")
	}
	if ev.Code != "assistant_stop_error" || ev.Message != "quota exceeded" {
		t.Fatalf("error 卡字段错误: %+v", ev)
	}
}

// 正常收尾(stop)/用户中止(aborted)/非助手消息 一律不发卡 —— 不与取消 UX、
// auto_retry/TurnFailed 双重报警(0.99.1 实帧的 stopReason 就是 "stop")。
func TestAssistantStopNormalSilent(t *testing.T) {
	stopLine := `{"type":"message_end","message":{"role":"assistant","content":[{"type":"text","text":"好"}],"stopReason":"stop","timestamp":1790730308893}}`
	if ev, ok := MapAssistantStop(json.RawMessage(stopLine)); ok {
		t.Fatalf("stop 收尾不应发卡: %+v", ev)
	}
	abortLine := `{"type":"message_end","message":{"role":"assistant","content":[],"stopReason":"aborted","timestamp":1790730308893}}`
	if ev, ok := MapAssistantStop(json.RawMessage(abortLine)); ok {
		t.Fatalf("aborted 收尾不应发卡: %+v", ev)
	}
	// 实帧(message_start/end 的 user 帧,pistudy_json_basic.jsonl)。
	userLine := `{"type":"message_end","message":{"role":"user","content":[{"type":"text","text":"只回复四个字:探针成功。不要使用任何工具。"}],"timestamp":1790730191343}}`
	if ev, ok := MapAssistantStop(json.RawMessage(userLine)); ok {
		t.Fatalf("user 消息不应发卡: %+v", ev)
	}
	// system 帧(piLedgerOnlyEvent 同批的 message_start 形状)。
	sysLine := `{"type":"message_end","message":{"role":"system","content":"","sections":{}}}`
	if ev, ok := MapAssistantStop(json.RawMessage(sysLine)); ok {
		t.Fatalf("system 消息不应发卡: %+v", ev)
	}
}

// 差距④(usage 终帧):deepseek-reasoner 实帧 —— 最后一个增量帧(text_end)自带
// 非零 per-message usage,piUsageTracker 据此入账并推事件驱动 UsageUpdate。
func TestStreamUsageFinalFrameObserved(t *testing.T) {
	final := `{"type":"message_update","usage":{"input":234,"output":471,"cacheRead":18688,"cacheWrite":0,"reasoning":110,"totalTokens":19393,"cost":{"input":0.00030888000000000004,"output":0.00186516,"cacheRead":0.000822272,"cacheWrite":0,"total":0.002996312}},"assistantMessageEvent":{"type":"text_end","contentIndex":1,"content":"##"}}`
	tr := &piUsageTracker{}
	if !tr.observe(json.RawMessage(final)) {
		t.Fatal("终帧非零 usage 应被观测到(observed 决定轮询兜底是否歇脚)")
	}
	snap := tr.snapshot()
	if snap == nil || snap.InputTokens != 234 || snap.OutputTokens != 471 || snap.ReasoningTokens != 110 {
		t.Fatalf("流式 usage 快照错误: %+v", snap)
	}
	// 过程帧(全零)不入账、不触发推送 —— 实帧里 delta 期间的 usage 全零。
	zero := `{"type":"message_update","usage":{"input":0,"output":0,"cacheRead":0,"cacheWrite":0,"totalTokens":0,"cost":{"input":0,"output":0,"cacheRead":0,"cacheWrite":0,"total":0}},"assistantMessageEvent":{"type":"text_delta","contentIndex":1,"delta":"好"}}`
	tr2 := &piUsageTracker{}
	if tr2.observe(json.RawMessage(zero)) {
		t.Fatal("全零过程帧不应触发 usage 推送")
	}
}
