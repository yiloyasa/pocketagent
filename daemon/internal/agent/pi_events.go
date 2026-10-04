// pi_events.go —— Pi RPC 事件 → PocketAgent pb 消息的映射纯函数。
//
// 【为什么全是纯函数】映射是这次接入最容易写错的地方(调研 §3.2 逐条都是坑),
// 全部抽成"输入 JSON → 输出 pb"的纯函数,单测把每个坑钉死:
//   - text_delta/thinking_delta 是增量 → 拼接(TextDelta/ThinkingDelta);
//   - tool_execution_update.partialResult 是累积值 → 直接替换(UPDATE 帧,不追加);
//   - 流式 usage 恒 0 → 用量只从 get_session_stats 取;
//   - agent_end 不算完,agent_settled 才是终局。
//
// 0.85.1 实测报文样本见 docs/verify/evidence/pi-p0-probe.md。
package agent

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/runtime"
)

// ---- message_update.assistantMessageEvent ----

// piAssistantEvent message_update 里的流式增量事件(只解析用得到的字段)。
type piAssistantEvent struct {
	Type         string          `json:"type"` // text_delta / thinking_delta / toolcall_start / toolcall_delta / toolcall_end
	Delta        string          `json:"delta"`
	ID           string          `json:"id"`
	ToolName     string          `json:"toolName"`
	ToolCall     *piToolCall     `json:"toolCall"`
	ContentIndex int             `json:"contentIndex"`
	RawExtra     json.RawMessage `json:"-"`
}

// piToolCall toolcall_end 携带的完整工具调用对象。
type piToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// piMessageUpdate message_update 事件的外层形状。
type piMessageUpdate struct {
	Type      string            `json:"type"`
	Assistant *piAssistantEvent `json:"assistantMessageEvent"`
}

// parseMessageUpdate 解析 message_update;非该类型/解析失败返回 nil。
func parseMessageUpdate(raw json.RawMessage) *piMessageUpdate {
	var m piMessageUpdate
	if json.Unmarshal(raw, &m) != nil || m.Type != "message_update" || m.Assistant == nil {
		return nil
	}
	return &m
}

// MapTextDelta text_delta → pb.TextDelta(增量直译)。
func MapTextDelta(raw json.RawMessage) (pb.TextDelta, bool) {
	m := parseMessageUpdate(raw)
	if m == nil || m.Assistant.Type != "text_delta" || m.Assistant.Delta == "" {
		return pb.TextDelta{}, false
	}
	return pb.TextDelta{Delta: m.Assistant.Delta}, true
}

// MapThinkingDelta thinking_delta → pb.ThinkingDelta(增量直译)。
func MapThinkingDelta(raw json.RawMessage) (pb.ThinkingDelta, bool) {
	m := parseMessageUpdate(raw)
	if m == nil || m.Assistant.Type != "thinking_delta" || m.Assistant.Delta == "" {
		return pb.ThinkingDelta{}, false
	}
	return pb.ThinkingDelta{Delta: m.Assistant.Delta}, true
}

// MapToolcallEnd toolcall_end → STARTED 帧(参数完整)。
// 【P1 取舍】参数要 28 个 delta 才拼完(实测),STARTED 卡片等 toolcall_end
// (参数完整)再发,执行进度由 tool_execution_update 的 UPDATE 帧承担 ——
// 手机端按 callId 原地覆盖,不缺过程感,也不会出现参数半截的卡片。
func MapToolcallEnd(raw json.RawMessage) (pb.ToolCallUpdate, bool) {
	m := parseMessageUpdate(raw)
	if m == nil || m.Assistant.Type != "toolcall_end" || m.Assistant.ToolCall == nil {
		return pb.ToolCallUpdate{}, false
	}
	tc := m.Assistant.ToolCall
	return pb.ToolCallUpdate{
		Phase: pb.ToolCallUpdate_STARTED,
		Call: &pb.ToolCallBlock{
			Id:       tc.ID,
			Name:     tc.Name,
			JsonArgs: piArgsString(tc.Arguments),
		},
	}, true
}

// piArgsString 把工具参数对象原样转成 JSON 字符串(pb.JsonArgs 约定)。
func piArgsString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "{}"
	}
	return string(raw)
}

// ---- tool_execution_* ----

// piToolExec tool_execution_start/update/end 的公共形状。
//
// 【0.99.1 deepseek 实帧(2026-09-30,%TEMP%\pistudy_json_tool.jsonl)】update/end
// 帧都带 args 字段 —— UPDATE/FINISHED 帧据此回填 JsonArgs,手机端按 callId 原地
// 替换工具卡时参数摘要不丢(pi TUI 的工具卡标题 = 工具名 + key=value 参数摘要,
// 见包内 dist/core/tools/render-utils.js formatToolCallWithArgs)。
type piToolExec struct {
	Type       string          `json:"type"`
	ToolCallID string          `json:"toolCallId"`
	ToolName   string          `json:"toolName"`
	Args       json.RawMessage `json:"args"`
	// partialResult:累积值(不是增量!直接替换显示 —— 调研 §3.2#7)
	PartialResult *piToolResult `json:"partialResult"`
	Result        *piToolResult `json:"result"`
	IsError       bool          `json:"isError"`
}

// piToolResult 工具执行结果:content 是块数组(取全部 text 拼接)。
type piToolResult struct {
	Content []piContent `json:"content"`
	// structuredContent 工具自带的结构化结果(bash 实帧:
	// {"output":...,"truncated":false,"exit_code":0,"wall_time_seconds":0.1})。
	// 只取展示用字段,不影响正文文本。
	Structured json.RawMessage `json:"structuredContent"`
}

// piToolStructured structuredContent 的展示字段(全部指针/可选:不同工具给的
// 字段不同,缺什么就不展示什么,绝不编造)。
type piToolStructured struct {
	ExitCode  *float64 `json:"exit_code"`
	WallTime  *float64 `json:"wall_time_seconds"`
	Truncated bool     `json:"truncated"`
}

// parseToolStructured 解析 structuredContent;空/解析失败返回零值(无脚注)。
func parseToolStructured(raw json.RawMessage) piToolStructured {
	var s piToolStructured
	if len(raw) == 0 || json.Unmarshal(raw, &s) != nil {
		return piToolStructured{}
	}
	return s
}

// piFormatSeconds 秒数格式化(与 pi TUI formatDuration 的 <60s 分支同形:一位小数)。
func piFormatSeconds(sec float64) string {
	return fmt.Sprintf("%.1fs", sec)
}

// piToolFooter 工具卡的耗时/退出码/截断脚注(对齐 pi TUI bash 渲染器的
// "Took 4.1s" / "(exit N)" / "[Truncated: ...]" 语义 —— 包内
// dist/core/tools/renderers/bash.js)。
//
// 数据来源纪律:wallTime 只信 structuredContent 的 wall_time_seconds;elapsed 是
// daemon 自己实测的起止差(真测量,非编造)。三者的展示条件:
//   - secs(耗时):structuredContent 优先,缺了用 elapsed,再缺就没有耗时行;
//   - exitCode:非 0 才展示(pi 也是只在出错时标 exit);
//   - truncated:true 才展示。
//
// 全部缺席返回空串(无脚注);body 非空时脚注前加换行,保证不黏在正文上。
func piToolFooter(s piToolStructured, elapsed time.Duration, running bool, bodyLen int) string {
	var parts []string
	switch {
	case s.WallTime != nil:
		label := "用时"
		if running {
			label = "已运行"
		}
		parts = append(parts, label+" "+piFormatSeconds(*s.WallTime))
	case elapsed > 0:
		label := "用时"
		if running {
			label = "已运行"
		}
		parts = append(parts, label+" "+piFormatSeconds(elapsed.Seconds()))
	}
	if s.ExitCode != nil && *s.ExitCode != 0 {
		parts = append(parts, fmt.Sprintf("退出码 %s", strconv.FormatFloat(*s.ExitCode, 'f', -1, 64)))
	}
	if s.Truncated {
		parts = append(parts, "输出已截断")
	}
	if len(parts) == 0 {
		return ""
	}
	footer := "[" + strings.Join(parts, " · ") + "]"
	if bodyLen > 0 {
		footer = "\n" + footer
	}
	return footer
}

type piContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// parseToolExec 解析 tool_execution_*;非该族/解析失败返回 nil。
func parseToolExec(raw json.RawMessage) *piToolExec {
	var e piToolExec
	if json.Unmarshal(raw, &e) != nil {
		return nil
	}
	switch e.Type {
	case "tool_execution_start", "tool_execution_update", "tool_execution_end":
		return &e
	}
	return nil
}

// piResultText 提取结果文本(累积值整体,调用方决定 clip)。
func piResultText(r *piToolResult) string {
	if r == nil {
		return ""
	}
	var sb strings.Builder
	for _, c := range r.Content {
		if c.Type == "text" {
			sb.WriteString(c.Text)
		}
	}
	return sb.String()
}

// MapToolExecUpdate tool_execution_update → UPDATE 帧(原地流式替换)。
// 【proto 新增 UPDATE 相位的原因】旧枚举只有 STARTED/FINISHED,表达不了
// "同一 callId 的进行中进度";Android 端已有按 callId 覆盖机制,UPDATE 帧
// 直接被它消费(调研 §3.1#1)。
//
// elapsed 是 daemon 对该 callId 实测的"从 tool_execution_start 到本帧"的时长
// (pi TUI 在执行中每秒刷 "Elapsed Xs",这里以脚注形式带给手机端;0=未知,无脚注)。
func MapToolExecUpdate(raw json.RawMessage, clip int, elapsed time.Duration) (pb.ToolCallUpdate, bool) {
	e := parseToolExec(raw)
	if e == nil || e.Type != "tool_execution_update" || e.ToolCallID == "" {
		return pb.ToolCallUpdate{}, false
	}
	out := clipRunes(piResultText(e.PartialResult), clip)
	out += piToolFooter(piToolStructured{}, elapsed, true, len(out))
	return pb.ToolCallUpdate{
		Phase: pb.ToolCallUpdate_UPDATE,
		Call:  &pb.ToolCallBlock{Id: e.ToolCallID, Name: e.ToolName, JsonArgs: piArgsString(e.Args)},
		Result: &pb.ToolResultBlock{
			CallId: e.ToolCallID,
			Ok:     true, // 进行中:没有失败语义
			Output: out,
		},
	}, true
}

// MapToolExecEnd tool_execution_end → FINISHED 帧。
//
// 【展示增强(对齐 pi TUI)】finish 脚注 = structuredContent 的真实字段
// (wall_time_seconds/exit_code/truncated),缺字段回退 daemon 实测 elapsed,
// 再缺就没有脚注 —— 绝不编造。脚注在 clip 之后追加,永不被截掉。
func MapToolExecEnd(raw json.RawMessage, clip int, elapsed time.Duration) (pb.ToolCallUpdate, bool) {
	e := parseToolExec(raw)
	if e == nil || e.Type != "tool_execution_end" || e.ToolCallID == "" {
		return pb.ToolCallUpdate{}, false
	}
	out := clipRunes(piResultText(e.Result), clip)
	out += piToolFooter(parseToolStructured(e.Result.Structured), elapsed, false, len(out))
	return pb.ToolCallUpdate{
		Phase: pb.ToolCallUpdate_FINISHED,
		Call:  &pb.ToolCallBlock{Id: e.ToolCallID, Name: e.ToolName, JsonArgs: piArgsString(e.Args)},
		Result: &pb.ToolResultBlock{
			CallId: e.ToolCallID,
			Ok:     !e.IsError,
			Output: out,
		},
	}, true
}

// MapToolExecStart tool_execution_start → STARTED 帧(兜底:toolcall_end 缺失时
// 保证手机上仍有一张 STARTED 卡,不出现"只有进度没有卡"的悬空 UPDATE)。
func MapToolExecStart(raw json.RawMessage) (pb.ToolCallUpdate, bool) {
	e := parseToolExec(raw)
	if e == nil || e.Type != "tool_execution_start" || e.ToolCallID == "" {
		return pb.ToolCallUpdate{}, false
	}
	return pb.ToolCallUpdate{
		Phase: pb.ToolCallUpdate_STARTED,
		Call: &pb.ToolCallBlock{
			Id:       e.ToolCallID,
			Name:     e.ToolName,
			JsonArgs: piArgsString(e.Args),
		},
	}, true
}

// ---- get_session_stats ----

// piSessionStats get_session_stats.data(只取用得到的字段)。
type piSessionStats struct {
	Tokens struct {
		Input      int64 `json:"input"`
		Output     int64 `json:"output"`
		CacheRead  int64 `json:"cacheRead"`
		CacheWrite int64 `json:"cacheWrite"`
		Total      int64 `json:"total"`
	} `json:"tokens"`
	Cost struct {
		Total float64 `json:"total"`
	} `json:"cost"`
	ContextUsage *struct {
		Tokens        *int64 `json:"tokens"`
		ContextWindow int64  `json:"contextWindow"`
		Percent       any    `json:"percent"`
	} `json:"contextUsage"`
}

// parseSessionStats 解析 get_session_stats 应答。
func parseSessionStats(raw json.RawMessage) *piSessionStats {
	var s piSessionStats
	if json.Unmarshal(raw, &s) != nil {
		return nil
	}
	return &s
}

// UsageUpdateFromStats get_session_stats → UsageUpdate 帧。
//
// 【流式 usage 恒 0(实测)】Pi 的用量真相只在 get_session_stats:
//
//	usage = 本轮增量(diff),total_usage = 会话累计,model_context_window = 真实窗口。
//	compressed 后 contextUsage.tokens 可能为 null —— 此时窗口照给,占用率由 App 忽略。
//
// 返回指针:pb 消息含互斥量,vet 禁止按值复制/返回(与全项目 pb 用法一致)。
func UsageUpdateFromStats(cur, baseline *piSessionStats) *pb.UsageUpdate {
	out := &pb.UsageUpdate{Usage: &pb.Usage{}, TotalUsage: &pb.Usage{}}
	if cur == nil {
		return out
	}
	if baseline != nil {
		out.Usage.InputTokens = cur.Tokens.Input - baseline.Tokens.Input
		out.Usage.OutputTokens = cur.Tokens.Output - baseline.Tokens.Output
		out.Usage.CacheReadTokens = cur.Tokens.CacheRead - baseline.Tokens.CacheRead
		out.Usage.CacheWriteTokens = cur.Tokens.CacheWrite - baseline.Tokens.CacheWrite
		out.Usage.CostUsd = cur.Cost.Total - baseline.Cost.Total
	} else {
		out.Usage.InputTokens = cur.Tokens.Input
		out.Usage.OutputTokens = cur.Tokens.Output
		out.Usage.CacheReadTokens = cur.Tokens.CacheRead
		out.Usage.CacheWriteTokens = cur.Tokens.CacheWrite
		out.Usage.CostUsd = cur.Cost.Total
	}
	out.TotalUsage.InputTokens = cur.Tokens.Input
	out.TotalUsage.OutputTokens = cur.Tokens.Output
	out.TotalUsage.CacheReadTokens = cur.Tokens.CacheRead
	out.TotalUsage.CacheWriteTokens = cur.Tokens.CacheWrite
	out.TotalUsage.CostUsd = cur.Cost.Total
	if cur.ContextUsage != nil {
		out.ModelContextWindow = cur.ContextUsage.ContextWindow
	}
	return out
}

// ---- 状态类事件(compaction/retry/queue)→ CodexEvent ----

// MapStatusEvent 把 Pi 的状态事件映射成通用 CodexEvent 帧(P1 复用现有 kind,
// 不新开 oneof —— 调研 §3 的"压缩/重试/排队要有明确状态")。
// 返回指针:pb 消息含互斥量,按值返回会被 vet 拒绝。
func MapStatusEvent(evType string, raw json.RawMessage) (*pb.CodexEvent, bool) {
	ev := &pb.CodexEvent{RawJson: clipRunes(string(raw), 8000)}
	switch evType {
	case "compaction_start":
		var e struct {
			Reason string `json:"reason"`
		}
		_ = json.Unmarshal(raw, &e)
		ev.Kind = pb.CodexEvent_CONTEXT_COMPACTED
		ev.Code = "started"
		ev.Title = "上下文压缩中"
		ev.Message = "正在压缩上下文(" + e.Reason + "),期间输出可能停顿,不是卡死。"
		return ev, true
	case "compaction_end":
		// 【0.99.1 形状(docs/json.md + 探针)】end 帧带 reason/result?/aborted/
		// willRetry/errorMessage?:aborted=true = 压缩被中止(result 缺席);
		// errorMessage 非空 = 压缩失败;willRetry=true = 溢出恢复成功、即将重试
		// 当前任务(压缩本身已成功)。旧实现一律报"已压缩",0.99.1 语义下会把
		// 中止/失败冒充成功 —— 按四态如实呈现。
		var e struct {
			Aborted      bool   `json:"aborted"`
			WillRetry    bool   `json:"willRetry"`
			ErrorMessage string `json:"errorMessage"`
		}
		_ = json.Unmarshal(raw, &e)
		ev.Kind = pb.CodexEvent_CONTEXT_COMPACTED
		switch {
		case e.Aborted:
			ev.Code = "aborted"
			ev.Title = "上下文压缩已中止"
			ev.Message = "压缩被中止,本轮继续执行,上下文未变。"
		case e.ErrorMessage != "":
			ev.Code = "failed"
			ev.Title = "上下文压缩失败"
			ev.Message = "压缩失败:" + e.ErrorMessage + "(上下文未变,本轮继续执行。)"
		default:
			ev.Code = "completed"
			ev.Title = "上下文已压缩"
			if e.WillRetry {
				ev.Message = "上下文压缩完成(溢出恢复),即将重试刚才的任务。"
			} else {
				ev.Message = "上下文压缩完成,继续执行。"
			}
		}
		return ev, true
	case "auto_retry_start":
		// 【0.99.1 新字段 delayMs】docs/json.md:auto_retry_start 带 delayMs
		// (重试前的等待毫秒数);>0 时人话里带"约 N 秒后",手机端不再把
		// 重试间隔误读成卡死。attempt/maxAttempts/errorMessage 沿用 0.87.1。
		var e struct {
			Attempt      int    `json:"attempt"`
			MaxAttempts  int    `json:"maxAttempts"`
			DelayMs      int    `json:"delayMs"`
			ErrorMessage string `json:"errorMessage"`
		}
		_ = json.Unmarshal(raw, &e)
		ev.Kind = pb.CodexEvent_RETRY_STARTED
		ev.Code = "auto_retry"
		ev.Title = fmt.Sprintf("重试中(第 %d/%d 次)", e.Attempt, e.MaxAttempts)
		if e.DelayMs > 0 {
			ev.Title += fmt.Sprintf(" 约 %d 秒后", (e.DelayMs+999)/1000)
		}
		ev.Message = e.ErrorMessage
		ev.WillRetry = true
		ev.Attempt = int32(e.Attempt)
		return ev, true
	case "auto_retry_end":
		// 【0.99.1 新字段 attempt】成功/失败都带本次重试序号(>0 时进标题)。
		var e struct {
			Success    bool   `json:"success"`
			Attempt    int    `json:"attempt"`
			FinalError string `json:"finalError"`
		}
		_ = json.Unmarshal(raw, &e)
		ev.Kind = pb.CodexEvent_RETRY_STOPPED
		ev.Code = "auto_retry"
		ev.Title = "重试结束"
		if e.Attempt > 0 {
			ev.Title = fmt.Sprintf("重试结束(第 %d 次)", e.Attempt)
		}
		if e.Success {
			ev.Message = "重试成功,继续执行。"
		} else {
			ev.Message = "重试失败:" + e.FinalError
		}
		return ev, true
	case "queue_update":
		var e struct {
			Steering []string `json:"steering"`
			FollowUp []string `json:"followUp"`
		}
		_ = json.Unmarshal(raw, &e)
		ev.Kind = pb.CodexEvent_QUEUE
		ev.Code = "queue_update"
		ev.Title = "输入队列"
		ev.Message = fmt.Sprintf("插队 %d 条 · 排队 %d 条", len(e.Steering), len(e.FollowUp))
		return ev, true
	case "extension_error":
		var e struct {
			ExtensionPath string `json:"extensionPath"`
			Event         string `json:"event"`
			Error         string `json:"error"`
		}
		_ = json.Unmarshal(raw, &e)
		ev.Kind = pb.CodexEvent_ERROR
		ev.Code = "extension_error"
		ev.Title = "扩展错误"
		ev.Message = e.Error
		return ev, true
	}
	return nil, false
}

// ---- extension_ui_request → 审批卡 ----

// uiAskOption 反问卡上的一个选项。label 既是按钮文案也是答案回传值
// (各 agent 侧都按 label=value 收集);description/preview 是 zcode
// requestUserInput 的原生扩展信息,pi/codex 场景留空(omitempty,不改变
// 既有卡的形状;新 App 可选渲染,旧 App 忽略未知字段不受影响)。
type uiAskOption struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Preview     string `json:"preview,omitempty"`
}

// uiAskQuestion 反问卡上的一条问题(ID 是答案回传的稳定 key,固定 q0..qN)。
// Options 存选项原文:label 即值,用户点了哪项就把哪项回给扩展。
// MultiSelect 仅 zcode requestUserInput 会置真(zod schema 的 multiSelect?),
// 置真时卡 JSON 里标注 "multiSelect": true,供手机渲染多选态。
type uiAskQuestion struct {
	ID          string
	Question    string
	Options     []uiAskOption
	MultiSelect bool
}

// plainOptions 把裸 label 列表包成选项(pi select / zcode elicitation enum:
// 只有值,没有描述性元数据)。
func plainOptions(labels []string) []uiAskOption {
	opts := make([]uiAskOption, 0, len(labels))
	for _, l := range labels {
		opts = append(opts, uiAskOption{Label: l})
	}
	return opts
}

// buildAskUserCall 把反问请求包装成 Android 端"反问表单"形状的审批卡。
//
// 【为什么 name 固定 request_user_input】手机 ApprovalSheet 只对
// request_user_input / mcp_elicitation 两类渲染"问题+选项按钮+输入框"表单
// (ChatScreen.ApprovalAskNames);codex 的 requestUserInput 卡用的就是同一
// 形状,此处复用同一条呈现通道 —— Pi 的 select/input/editor 与 ZCode 的
// requestUserInput/elicitation 全部经它上屏,不再各自为政。
func buildAskUserCall(reqID, message string, questions []uiAskQuestion) pb.ToolCallBlock {
	qs := make([]map[string]any, 0, len(questions))
	for _, q := range questions {
		// 选项结构化透出(label/description/preview):description/preview 为空时
		// 序列化结果与旧形状 {"label":...} 完全一致,pi/codex 卡不受影响。
		// nil 归一成空数组:options 必须恒为 [],不能退化成 null。
		opts := q.Options
		if opts == nil {
			opts = []uiAskOption{}
		}
		qm := map[string]any{"id": q.ID, "question": q.Question, "options": opts}
		if q.MultiSelect {
			qm["multiSelect"] = true // 只在真值时标注,老卡形状零变化
		}
		qs = append(qs, qm)
	}
	return pb.ToolCallBlock{
		Id:   reqID,
		Name: toolNameAskUser, // "request_user_input",ApprovalReason 据此给"agent 需要你回答一个问题"
		JsonArgs: mustJSON(map[string]any{
			"message":   message,
			"questions": qs,
		}),
	}
}

// answerNoteMap 把手机审批决定携带的答案文本(ApprovalDecision.note)解析成
// "问题 id → 答案"。
//
// 【Android 提交形状】{"<问题id>": {"answers": ["……"]}}(ChatScreen.submit);
// 兼容裸字符串值与多选数组值 {"<问题id>": ["A","B"]};整条 note 是裸 JSON
// 数组时视为单问题卡的多选答案(q0)。解析失败时退化为"整段文本回答第一个
// 问题(q0)" —— 让 agent 至少拿到用户的意思,而不是拿到空回答后开始猜
// (与 codex splitAnswerNote 同款兜底)。
func answerNoteMap(note string) map[string]string {
	out := map[string]string{}
	trimmed := strings.TrimSpace(note)
	if trimmed == "" {
		return out
	}
	var parsed map[string]any
	if json.Unmarshal([]byte(trimmed), &parsed) != nil || len(parsed) == 0 {
		// 不是映射:可能是裸数组(多选直接提交数组),再不行才走整段文本兜底。
		var arr []any
		if json.Unmarshal([]byte(trimmed), &arr) == nil && len(arr) > 0 {
			if s := joinNoteAnswers(arr); s != "" {
				out["q0"] = s
			} else {
				out["q0"] = trimmed
			}
			return out
		}
		out["q0"] = trimmed
		return out
	}
	for k, v := range parsed {
		if s := noteAnswerOf(v); s != "" {
			out[k] = s
		}
	}
	return out
}

// joinNoteAnswers 把答案数组拼成单值文本(多选场景:值以 ", " 连接回传 ——
// 各 agent 的收集端目前都收单值;数组逐项保真交给 Android 侧形状,daemon
// 不丢项也不去重)。
func joinNoteAnswers(arr []any) string {
	parts := make([]string, 0, len(arr))
	for _, a := range arr {
		if s, ok := a.(string); ok && strings.TrimSpace(s) != "" {
			parts = append(parts, strings.TrimSpace(s))
		}
	}
	return strings.Join(parts, ", ")
}

// noteAnswerOf 提取单个问题条目的答案文本,四形状兼容:
// {"answers":["…"]} / ["…"](多选数组) / "…"(裸字符串) / {"answer":"…"},
// 数组以 ", " 连接取全部(多选不再只取首选)。
func noteAnswerOf(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case []any:
		return joinNoteAnswers(t)
	case map[string]any:
		if arr, ok := t["answers"].([]any); ok {
			return joinNoteAnswers(arr)
		}
		if s, ok := t["answer"].(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

// firstAnswerFromNote 单问题便捷版:q0 的答案(Pi 卡固定单问题)。
func firstAnswerFromNote(note string) string {
	return answerNoteMap(note)["q0"]
}

// ApprovalCallFromUI 把 dialog 类 extension_ui_request 包装成审批卡动作。
//
// 【分形态】confirm 保持专用确认卡(pi_ui_confirm,手机按"允许/拒绝"渲染);
// select/input/editor 走反问表单(request_user_input):select 的选项渲染成
// 点选按钮,input/editor 给自由输入框 —— 取代旧行为"select 自动取第一项 /
// input 空值放行"。
//
// 【为什么走 approve hook 而不是另开通道】server 的 approve 中继已经把
// ApprovalRequest→手机→decision 的整条链路(含断线 readerDone 语义)做完了,
// 复用它 = 审批卡/超时/断线回 cancelled 全部继承(调研 §4.6 的要求)。
// 【vet】返回指针:pb 消息含锁,调用方(pi.go:430)按调用结果直传 approve,
// ApprovalHook 已改指针参数,这里必须回传指针(内容不变)。
func ApprovalCallFromUI(reqID string, method, title, message string, options []string) *pb.ToolCallBlock {
	if method == "confirm" {
		args := map[string]any{"method": method}
		if title != "" {
			args["title"] = title
		}
		if message != "" {
			args["message"] = message
		}
		raw, _ := json.Marshal(args)
		return &pb.ToolCallBlock{Id: reqID, Name: "pi_ui_" + method, JsonArgs: string(raw)}
	}
	// 副标题(message 键)给"这是什么问题"的完整语境;题干取 title 优先。
	subtitle := firstNonEmpty(message, title)
	q := uiAskQuestion{ID: "q0", Question: firstNonEmpty(title, message)}
	if method == "select" {
		q.Options = plainOptions(options)
	}
	// input/editor:不给选项,手机端渲染为纯输入框。
	// buildAskUserCall 保持值返回(它的结果还会进别的元组),这里取址适配指针签名。
	call := buildAskUserCall(reqID, subtitle, []uiAskQuestion{q})
	return &call
}

// UIResponseFromDecision 把手机审批决定翻译回 extension_ui_response 的 payload。
//
// 【失败安全】超时/断线/拒绝一律 cancelled(扩展收到 undefined/false),
// 绝不默认放行 —— Pi 的 dialog 无 timeout 时会永久挂起整轮(调研 §4.6)。
// 【select 真选】value = 用户实际点选/输入的选项(答案经 note 通道回传);
// 拿不到真实选择时同样回 cancelled —— 绝不静默替用户选第一项(旧 P1 缺陷已移除)。
// 【input/editor】value = 用户输入;批准且未输入 = 用户明确确认空值,放行 ""。
func UIResponseFromDecision(method string, approved bool, note string) map[string]any {
	if !approved {
		return map[string]any{"cancelled": true}
	}
	switch method {
	case "confirm":
		return map[string]any{"confirmed": true}
	case "select":
		if v := firstAnswerFromNote(note); v != "" {
			return map[string]any{"value": v}
		}
		// 审批卡没有带回真实选择:宁可拒绝也不替用户做决定。
		return map[string]any{"cancelled": true}
	default: // input / editor:value = 用户输入
		return map[string]any{"value": firstAnswerFromNote(note)}
	}
}

// ---- session_info_changed → 标题回调(TitleHandler 数据源) ----

// MapSessionNameChange 识别会话名变化事件,提取新名字。
//
// 【0.85.1 实测】set_session_name(或电脑端同进程改名)后 Pi 发
// {"name":"...","type":"session_info_changed"}(注意:该事件先于命令应答帧到达)。
// provider 的常驻 GlobalHandler 据此触发 TitleHandler → server 落库,手机/桌面
// 双端标题一致。非该类型 / 名字空白:一律 ("", false),调用方静默忽略 ——
// readLoop 上的常驻钩子必须对无关事件零成本通过。
func MapSessionNameChange(raw json.RawMessage) (string, bool) {
	var e struct {
		Type string `json:"type"`
		Name string `json:"name"`
	}
	if json.Unmarshal(raw, &e) != nil || e.Type != "session_info_changed" {
		return "", false
	}
	name := strings.TrimSpace(e.Name)
	if name == "" {
		return "", false
	}
	return name, true
}

// MapAssistantStop 从 message_end 帧提取"异常收尾"的助手消息(0.99.1 实帧:
// {"type":"message_end","message":{"role":"assistant",...,"stopReason":"length|error|stop|aborted",
// "errorMessage":...}})。
//
// 【为什么需要它】pi TUI 会把 length 截断渲染成 "Response was truncated before
// completion."、error 渲染成 "Error: <msg>"(包内
// dist/modes/interactive/components/assistant-message.js);daemon 此前把 message_end
// 整类归为只落台账(piLedgerOnlyEvent),手机端对"回复被截断"完全不可见 ——
// 用户看到的是一条戛然而止的回复,分不清是模型说完了还是被截了。
// 只认 length/error 两类正常路径不会出现的收尾;stop/aborted 不发卡(aborted 有
// 专门的取消 UX,error 大多伴随 auto_retry/TurnFailed,别双重报警)。
// 返回 (事件, true) 表示应上屏一张 WARNING 状态卡;调用方仍照旧落台账。
func MapAssistantStop(raw json.RawMessage) (*pb.CodexEvent, bool) {
	var e struct {
		Message struct {
			Role         string `json:"role"`
			StopReason   string `json:"stopReason"`
			ErrorMessage string `json:"errorMessage"`
		} `json:"message"`
	}
	if json.Unmarshal(raw, &e) != nil || e.Message.Role != "assistant" {
		return nil, false
	}
	ev := &pb.CodexEvent{Kind: pb.CodexEvent_WARNING}
	switch e.Message.StopReason {
	case "length":
		ev.Code = "assistant_stop_length"
		ev.Title = "回复被截断"
		ev.Message = "本条回复在达到长度上限前被截断,内容可能不完整;可让 agent 继续输出。"
		return ev, true
	case "error":
		ev.Code = "assistant_stop_error"
		ev.Title = "模型返回错误"
		ev.Message = firstNonEmpty(e.Message.ErrorMessage, "模型流以错误收尾。")
		return ev, true
	}
	return nil, false
}

// clipRunes 按 rune 截断(工具输出进 pb 前限长,防单帧过大)。
func clipRunes(s string, n int) string {
	if n <= 0 {
		return s // 原语义:n<=0 视为不设限(真实调用方均传大正数),收敛时保留
	}
	// 统一走 runtime.ClipRunesEllipsis,收敛见审查报告 util 节。
	return runtime.ClipRunesEllipsis(s, n)
}
