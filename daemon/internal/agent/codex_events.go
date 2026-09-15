package agent

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

type codexErrorNotification struct {
	ThreadID  string `json:"threadId"`
	TurnID    string `json:"turnId"`
	WillRetry bool   `json:"willRetry"`
	Error     struct {
		Message        string          `json:"message"`
		Additional     string          `json:"additionalDetails"`
		CodexErrorInfo json.RawMessage `json:"codexErrorInfo"`
	} `json:"error"`
}

type codexRunError struct {
	info *pb.Error
}

func (e *codexRunError) Error() string {
	if e == nil || e.info == nil {
		return "codex: 未知错误"
	}
	return "codex: " + e.info.Message
}

func (e *codexRunError) ErrorInfo() *pb.Error {
	if e == nil {
		return nil
	}
	return e.info
}

var upstreamURLPattern = regexp.MustCompile(`https?://[^\s,]+`)

// parseCodexErrorNotification converts the Codex app-server error shape into
// the stable event/error contract used by both daemon and Android.
func parseCodexErrorNotification(raw json.RawMessage, threadID, turnID string) (*pb.CodexEvent, *pb.Error) {
	var n codexErrorNotification
	if err := json.Unmarshal(raw, &n); err != nil {
		return nil, nil
	}

	actualThreadID := n.ThreadID
	if actualThreadID == "" {
		actualThreadID = threadID
	}
	actualTurnID := n.TurnID
	if actualTurnID == "" {
		actualTurnID = turnID
	}
	message := strings.TrimSpace(n.Error.Message)
	if message == "" {
		// Older app-server builds sent {message: "..."} directly.
		var legacy struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(raw, &legacy)
		message = strings.TrimSpace(legacy.Message)
	}
	if message == "" {
		message = "Codex 返回了未说明原因的错误"
	}

	code := codexErrorCode(n.Error.CodexErrorInfo)
	status := codexHTTPStatus(n.Error.CodexErrorInfo)
	if status == 0 {
		status = statusFromMessage(message)
	}
	url := extractUpstreamURL(message)
	details := strings.TrimSpace(n.Error.Additional)
	if details == "" {
		details = message
	}
	kind := pb.CodexEvent_ERROR
	if n.WillRetry {
		kind = pb.CodexEvent_RETRY_STARTED
	}
	title := "Codex 错误"
	if status == 503 {
		title = "上游服务暂时不可用"
	} else if n.WillRetry {
		title = "Codex 正在重试"
	}
	event := &pb.CodexEvent{
		Kind:       kind,
		Code:       code,
		Title:      title,
		Message:    message,
		Url:        url,
		HttpStatus: int32(status),
		WillRetry:  n.WillRetry,
		ThreadId:   actualThreadID,
		TurnId:     actualTurnID,
		RawJson:    clip(string(raw), 8000),
	}
	errInfo := &pb.Error{
		Code:       code,
		Message:    message,
		Retryable:  n.WillRetry,
		HttpStatus: int32(status),
		Url:        url,
		Details:    details,
	}
	return event, errInfo
}

func codexErrorCode(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return "unknown"
	}
	var code string
	if json.Unmarshal(raw, &code) == nil && code != "" {
		return code
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) == nil {
		for key := range object {
			return key
		}
	}
	return "unknown"
}

func codexHTTPStatus(raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) == nil {
		for key, value := range object {
			if strings.EqualFold(key, "httpStatusCode") {
				var status int
				_ = json.Unmarshal(value, &status)
				return status
			}
			if nested := codexHTTPStatus(value); nested != 0 {
				return nested
			}
		}
	}
	return 0
}

func statusFromMessage(message string) int {
	for _, token := range strings.Fields(message) {
		token = strings.Trim(token, "(),:;")
		var status int
		if _, err := fmt.Sscanf(token, "%d", &status); err == nil && status >= 100 && status <= 599 {
			return status
		}
	}
	return 0
}

func extractUpstreamURL(message string) string {
	match := upstreamURLPattern.FindString(message)
	return strings.TrimRight(match, ".)]}>'\"")
}

// codexNotificationEvent normalizes the non-text notifications emitted by
// Codex app-server into the event contract understood by the Android client.
// The original JSON is retained so the client can show diagnostics without
// losing fields that are newer than this daemon.
func codexNotificationEvent(method string, raw json.RawMessage, threadID, turnID string) *pb.CodexEvent {
	var ids struct {
		ThreadID string `json:"threadId"`
		TurnID   string `json:"turnId"`
	}
	_ = json.Unmarshal(raw, &ids)
	if ids.ThreadID != "" && threadID != "" && ids.ThreadID != threadID {
		return nil
	}
	actualThreadID := ids.ThreadID
	if actualThreadID == "" {
		actualThreadID = threadID
	}
	actualTurnID := ids.TurnID
	if actualTurnID == "" {
		actualTurnID = turnID
	}

	event := &pb.CodexEvent{
		Kind:     pb.CodexEvent_STATUS,
		ThreadId: actualThreadID,
		TurnId:   actualTurnID,
		RawJson:  clip(string(raw), 8000),
	}

	switch method {
	case "warning":
		var n struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(raw, &n)
		event.Kind = pb.CodexEvent_WARNING
		event.Title = "Codex 警告"
		event.Message = n.Message
	case "guardianWarning":
		var n struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(raw, &n)
		event.Kind = pb.CodexEvent_WARNING
		event.Title = "安全警告"
		event.Message = n.Message
	case "thread/status/changed":
		var n struct {
			Status struct {
				Type        string   `json:"type"`
				ActiveFlags []string `json:"activeFlags"`
			} `json:"status"`
		}
		_ = json.Unmarshal(raw, &n)
		event.Title = "线程状态更新"
		event.Message = "状态：" + n.Status.Type
		if len(n.Status.ActiveFlags) > 0 {
			event.Message += " · " + strings.Join(n.Status.ActiveFlags, "、")
		}
	case "thread/tokenUsage/updated":
		var n struct {
			TokenUsage struct {
				Last struct {
					InputTokens           int64 `json:"inputTokens"`
					OutputTokens          int64 `json:"outputTokens"`
					TotalTokens           int64 `json:"totalTokens"`
					CachedInputTokens     int64 `json:"cachedInputTokens"`
					ReasoningOutputTokens int64 `json:"reasoningOutputTokens"`
				} `json:"last"`
				Total struct {
					InputTokens  int64 `json:"inputTokens"`
					OutputTokens int64 `json:"outputTokens"`
					TotalTokens  int64 `json:"totalTokens"`
				} `json:"total"`
				ModelContextWindow *int64 `json:"modelContextWindow"`
			} `json:"tokenUsage"`
		}
		_ = json.Unmarshal(raw, &n)
		event.Title = "Token 用量更新"
		event.Message = fmt.Sprintf(
			"本轮：输入 %d · 输出 %d · 总计 %d；累计：输入 %d · 输出 %d · 总计 %d",
			n.TokenUsage.Last.InputTokens,
			n.TokenUsage.Last.OutputTokens,
			n.TokenUsage.Last.TotalTokens,
			n.TokenUsage.Total.InputTokens,
			n.TokenUsage.Total.OutputTokens,
			n.TokenUsage.Total.TotalTokens,
		)
		if n.TokenUsage.ModelContextWindow != nil {
			event.Message += fmt.Sprintf(" · 上下文窗口 %d", *n.TokenUsage.ModelContextWindow)
		}
	case "turn/plan/updated":
		var n struct {
			Explanation *string `json:"explanation"`
			Plan        []struct {
				Step   string `json:"step"`
				Status string `json:"status"`
			} `json:"plan"`
		}
		_ = json.Unmarshal(raw, &n)
		event.Title = "执行计划更新"
		steps := make([]string, 0, len(n.Plan))
		for _, step := range n.Plan {
			steps = append(steps, fmt.Sprintf("%s|%s", step.Status, step.Step))
		}
		// plan_steps 走结构化字段,手机端渲染为步骤卡;message 保留纯文本摘要。
		event.PlanSteps = steps
		event.Message = strings.Join(steps, "；")
		if n.Explanation != nil && strings.TrimSpace(*n.Explanation) != "" {
			if event.Message != "" {
				event.Message += " · "
			}
			event.Message += strings.TrimSpace(*n.Explanation)
		}
	case "item/plan/delta":
		var n struct {
			Delta string `json:"delta"`
		}
		_ = json.Unmarshal(raw, &n)
		event.Title = "执行计划生成中"
		event.Message = n.Delta
	case "item/commandExecution/outputDelta", "item/fileChange/outputDelta":
		var n struct {
			Delta string `json:"delta"`
		}
		_ = json.Unmarshal(raw, &n)
		event.Title = "命令执行过程"
		event.Message = n.Delta
	case "item/fileChange/patchUpdated":
		var n struct {
			Changes []struct {
				Diff string `json:"diff"`
				Kind struct {
					Type string `json:"type"`
				} `json:"kind"`
				Path string `json:"path"`
			} `json:"changes"`
		}
		_ = json.Unmarshal(raw, &n)
		event.Title = "文件变更预览"
		changes := make([]string, 0, len(n.Changes))
		for _, change := range n.Changes {
			changes = append(changes, fmt.Sprintf("%s [%s]", change.Path, change.Kind.Type))
		}
		event.Message = strings.Join(changes, "；")
	case "item/mcpToolCall/progress":
		var n struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(raw, &n)
		event.Title = "MCP 工具进度"
		event.Message = n.Message
	case "command/exec/outputDelta", "process/outputDelta":
		var n struct {
			DeltaBase64 string `json:"deltaBase64"`
			Stream      string `json:"stream"`
		}
		_ = json.Unmarshal(raw, &n)
		event.Title = "命令输出"
		if decoded, err := base64.StdEncoding.DecodeString(n.DeltaBase64); err == nil {
			event.Message = string(decoded)
		} else {
			event.Message = n.DeltaBase64
		}
		if n.Stream != "" {
			event.Code = n.Stream
		}
	case "model/rerouted":
		var n struct {
			FromModel string `json:"fromModel"`
			ToModel   string `json:"toModel"`
			Reason    string `json:"reason"`
		}
		_ = json.Unmarshal(raw, &n)
		event.Kind = pb.CodexEvent_MODEL_REROUTED
		event.Title = "模型已切换"
		event.Message = fmt.Sprintf("%s → %s（原因：%s）", n.FromModel, n.ToModel, n.Reason)
		event.Code = n.Reason
	case "context/compacted", "thread/compacted":
		event.Kind = pb.CodexEvent_CONTEXT_COMPACTED
		event.Title = "上下文已压缩"
		event.Message = "Codex 已压缩当前线程上下文，继续任务时会使用较新的上下文窗口。"
	case "mcpServer/startupStatus/updated", "mcpServer/status/updated":
		var n struct {
			Name          string  `json:"name"`
			Status        string  `json:"status"`
			Error         *string `json:"error"`
			FailureReason *string `json:"failureReason"`
		}
		_ = json.Unmarshal(raw, &n)
		event.Title = "MCP 服务器状态"
		event.Message = fmt.Sprintf("%s：%s", n.Name, n.Status)
		if n.Error != nil && strings.TrimSpace(*n.Error) != "" {
			event.Message += " · " + strings.TrimSpace(*n.Error)
		}
		if n.FailureReason != nil && strings.TrimSpace(*n.FailureReason) != "" {
			event.Code = *n.FailureReason
		}
	case "account/rateLimits/updated":
		var n struct {
			RateLimits struct {
				PlanType string `json:"planType"`
				Primary  struct {
					UsedPercent int `json:"usedPercent"`
				} `json:"primary"`
				Secondary struct {
					UsedPercent int `json:"usedPercent"`
				} `json:"secondary"`
			} `json:"rateLimits"`
		}
		_ = json.Unmarshal(raw, &n)
		event.Kind = pb.CodexEvent_RATE_LIMIT
		event.Title = "用量限制更新"
		event.Message = fmt.Sprintf(
			"主要窗口已使用 %d%% · 次要窗口已使用 %d%%",
			n.RateLimits.Primary.UsedPercent,
			n.RateLimits.Secondary.UsedPercent,
		)
		if n.RateLimits.PlanType != "" {
			event.Message += " · 方案 " + n.RateLimits.PlanType
		}
	default:
		return nil
	}

	if strings.TrimSpace(event.Message) == "" {
		event.Message = "Codex 收到了一条 " + method + " 事件"
	}
	return event
}

func codexTokenUsageUpdate(raw json.RawMessage, threadID, turnID string) *pb.UsageUpdate {
	var n struct {
		ThreadID   string `json:"threadId"`
		TurnID     string `json:"turnId"`
		TokenUsage struct {
			Last struct {
				InputTokens           int64 `json:"inputTokens"`
				CachedInputTokens     int64 `json:"cachedInputTokens"`
				CacheWriteTokens      int64 `json:"cacheWriteInputTokens"`
				OutputTokens          int64 `json:"outputTokens"`
				ReasoningOutputTokens int64 `json:"reasoningOutputTokens"`
			} `json:"last"`
			Total struct {
				InputTokens           int64 `json:"inputTokens"`
				CachedInputTokens     int64 `json:"cachedInputTokens"`
				CacheWriteTokens      int64 `json:"cacheWriteInputTokens"`
				OutputTokens          int64 `json:"outputTokens"`
				ReasoningOutputTokens int64 `json:"reasoningOutputTokens"`
			} `json:"total"`
			ModelContextWindow *int64 `json:"modelContextWindow"`
		} `json:"tokenUsage"`
	}
	if json.Unmarshal(raw, &n) != nil {
		return nil
	}
	if n.ThreadID != "" && threadID != "" && n.ThreadID != threadID {
		return nil
	}
	if n.TurnID != "" && turnID != "" && n.TurnID != turnID {
		return nil
	}
	update := &pb.UsageUpdate{
		Usage: &pb.Usage{
			InputTokens:      n.TokenUsage.Last.InputTokens,
			OutputTokens:     n.TokenUsage.Last.OutputTokens,
			CacheReadTokens:  n.TokenUsage.Last.CachedInputTokens,
			CacheWriteTokens: n.TokenUsage.Last.CacheWriteTokens,
			ReasoningTokens:  n.TokenUsage.Last.ReasoningOutputTokens,
		},
		TotalUsage: &pb.Usage{
			InputTokens:      n.TokenUsage.Total.InputTokens,
			OutputTokens:     n.TokenUsage.Total.OutputTokens,
			CacheReadTokens:  n.TokenUsage.Total.CachedInputTokens,
			CacheWriteTokens: n.TokenUsage.Total.CacheWriteTokens,
			ReasoningTokens:  n.TokenUsage.Total.ReasoningOutputTokens,
		},
	}
	// 真实窗口大小随用量一起透传:模型列表拿不到各模型的窗口,
	// 手机端"上下文窗口"卡以这里为准(切模型后也能立即显示)。
	if n.TokenUsage.ModelContextWindow != nil {
		update.ModelContextWindow = *n.TokenUsage.ModelContextWindow
	}
	return update
}
