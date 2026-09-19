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
	"strings"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
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

// MapToolcallStart 提取 toolcall_start(id+toolName)。
// 【P1 取舍】参数要 28 个 delta 才拼完(实测),STARTED 卡片等 toolcall_end
// (参数完整)再发,执行进度由 tool_execution_update 的 UPDATE 帧承担 ——
// 手机端按 callId 原地覆盖,不缺过程感,也不会出现参数半截的卡片。
func MapToolcallStart(raw json.RawMessage) (id, toolName string, ok bool) {
	m := parseMessageUpdate(raw)
	if m == nil || m.Assistant.Type != "toolcall_start" {
		return "", "", false
	}
	return m.Assistant.ID, m.Assistant.ToolName, true
}

// MapToolcallEnd toolcall_end → STARTED 帧(参数完整)。
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
func MapToolExecUpdate(raw json.RawMessage, clip int) (pb.ToolCallUpdate, bool) {
	e := parseToolExec(raw)
	if e == nil || e.Type != "tool_execution_update" || e.ToolCallID == "" {
		return pb.ToolCallUpdate{}, false
	}
	out := clipRunes(piResultText(e.PartialResult), clip)
	return pb.ToolCallUpdate{
		Phase: pb.ToolCallUpdate_UPDATE,
		Call:  &pb.ToolCallBlock{Id: e.ToolCallID, Name: e.ToolName},
		Result: &pb.ToolResultBlock{
			CallId: e.ToolCallID,
			Ok:     true, // 进行中:没有失败语义
			Output: out,
		},
	}, true
}

// MapToolExecEnd tool_execution_end → FINISHED 帧。
func MapToolExecEnd(raw json.RawMessage, clip int) (pb.ToolCallUpdate, bool) {
	e := parseToolExec(raw)
	if e == nil || e.Type != "tool_execution_end" || e.ToolCallID == "" {
		return pb.ToolCallUpdate{}, false
	}
	return pb.ToolCallUpdate{
		Phase: pb.ToolCallUpdate_FINISHED,
		Call:  &pb.ToolCallBlock{Id: e.ToolCallID, Name: e.ToolName},
		Result: &pb.ToolResultBlock{
			CallId: e.ToolCallID,
			Ok:     !e.IsError,
			Output: clipRunes(piResultText(e.Result), clip),
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
		ev.Kind = pb.CodexEvent_CONTEXT_COMPACTED
		ev.Code = "completed"
		ev.Title = "上下文已压缩"
		ev.Message = "上下文压缩完成,继续执行。"
		return ev, true
	case "auto_retry_start":
		var e struct {
			Attempt      int    `json:"attempt"`
			MaxAttempts  int    `json:"maxAttempts"`
			ErrorMessage string `json:"errorMessage"`
		}
		_ = json.Unmarshal(raw, &e)
		ev.Kind = pb.CodexEvent_RETRY_STARTED
		ev.Code = "auto_retry"
		ev.Title = fmt.Sprintf("重试中(第 %d/%d 次)", e.Attempt, e.MaxAttempts)
		ev.Message = e.ErrorMessage
		ev.WillRetry = true
		ev.Attempt = int32(e.Attempt)
		return ev, true
	case "auto_retry_end":
		var e struct {
			Success    bool   `json:"success"`
			FinalError string `json:"finalError"`
		}
		_ = json.Unmarshal(raw, &e)
		ev.Kind = pb.CodexEvent_RETRY_STOPPED
		ev.Code = "auto_retry"
		ev.Title = "重试结束"
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

// uiAskQuestion 反问卡上的一条问题(ID 是答案回传的稳定 key,固定 q0..qN)。
// Options 存选项原文:label 即值,用户点了哪项就把哪项回给扩展。
type uiAskQuestion struct {
	ID       string
	Question string
	Options  []string
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
		opts := make([]map[string]string, 0, len(q.Options))
		for _, o := range q.Options {
			opts = append(opts, map[string]string{"label": o})
		}
		qs = append(qs, map[string]any{"id": q.ID, "question": q.Question, "options": opts})
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
// 兼容裸字符串值。解析失败时退化为"整段文本回答第一个问题(q0)" —— 让 agent
// 至少拿到用户的意思,而不是拿到空回答后开始猜(与 codex splitAnswerNote 同款兜底)。
func answerNoteMap(note string) map[string]string {
	out := map[string]string{}
	trimmed := strings.TrimSpace(note)
	if trimmed == "" {
		return out
	}
	var parsed map[string]any
	if json.Unmarshal([]byte(trimmed), &parsed) != nil || len(parsed) == 0 {
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

// noteAnswerOf 提取单个问题条目的答案文本:{"answers":["…"]} / "…" 两兼容,
// 取第一条非空;数组 Join 语义交给调用方(多选 P1 不支持,单选取首选)。
func noteAnswerOf(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case map[string]any:
		if arr, ok := t["answers"].([]any); ok {
			parts := make([]string, 0, len(arr))
			for _, a := range arr {
				if s, ok := a.(string); ok && strings.TrimSpace(s) != "" {
					parts = append(parts, strings.TrimSpace(s))
				}
			}
			return strings.Join(parts, ", ")
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
		q.Options = options
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

// clipRunes 按 rune 截断(工具输出进 pb 前限长,防单帧过大)。
func clipRunes(s string, n int) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
