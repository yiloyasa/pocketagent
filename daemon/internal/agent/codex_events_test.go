package agent

import (
	"encoding/json"
	"strings"
	"testing"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

func TestParseCodexErrorNotificationExtractsUpstreamDetails(t *testing.T) {
	raw := json.RawMessage(`{
		"threadId":"thread-1",
		"turnId":"turn-1",
		"willRetry":false,
		"error":{
			"message":"unexpected status 503 Service Unavailable: 上游 Responses 端点连续失败，熔断冷却中，请稍后重试, url: http://127.0.0.1:4446/codex-router/v1/responses",
			"codexErrorInfo":{
				"responseStreamConnectionFailed":{"httpStatusCode":503}
			}
		}
	}`)

	event, errInfo := parseCodexErrorNotification(raw, "thread-1", "turn-1")
	if event == nil || errInfo == nil {
		t.Fatalf("expected event and error, got event=%v error=%v", event, errInfo)
	}
	if event.Kind != pb.CodexEvent_ERROR {
		t.Fatalf("event kind = %v, want ERROR", event.Kind)
	}
	if event.Code != "responseStreamConnectionFailed" {
		t.Fatalf("event code = %q, want responseStreamConnectionFailed", event.Code)
	}
	if event.HttpStatus != 503 {
		t.Fatalf("http status = %d, want 503", event.HttpStatus)
	}
	if event.Url != "http://127.0.0.1:4446/codex-router/v1/responses" {
		t.Fatalf("url = %q", event.Url)
	}
	if errInfo.HttpStatus != 503 || errInfo.Url != event.Url {
		t.Fatalf("error details were not copied: %+v", errInfo)
	}
}

func TestParseCodexErrorNotificationMarksRetryingEvents(t *testing.T) {
	raw := json.RawMessage(`{
		"threadId":"thread-1",
		"turnId":"turn-1",
		"willRetry":true,
		"error":{
			"message":"temporary upstream failure",
			"codexErrorInfo":"serverOverloaded"
		}
	}`)

	event, errInfo := parseCodexErrorNotification(raw, "thread-1", "turn-1")
	if event == nil || errInfo == nil {
		t.Fatalf("expected event and error")
	}
	if event.Kind != pb.CodexEvent_RETRY_STARTED {
		t.Fatalf("event kind = %v, want RETRY_STARTED", event.Kind)
	}
	if !event.WillRetry {
		t.Fatal("event should indicate will_retry")
	}
	if errInfo.Code != "serverOverloaded" {
		t.Fatalf("error code = %q, want serverOverloaded", errInfo.Code)
	}
}

func TestCodexNotificationEventMapsVisibleLifecycleEvents(t *testing.T) {
	tests := []struct {
		name      string
		method    string
		raw       string
		kind      pb.CodexEvent_Kind
		titlePart string
		msgPart   string
	}{
		{
			name:      "warning",
			method:    "warning",
			raw:       `{"threadId":"thread-1","message":"配置文件存在未知字段"}`,
			kind:      pb.CodexEvent_WARNING,
			titlePart: "警告",
			msgPart:   "配置文件存在未知字段",
		},
		{
			name:      "model reroute",
			method:    "model/rerouted",
			raw:       `{"threadId":"thread-1","turnId":"turn-1","fromModel":"gpt-5.4","toModel":"gpt-5.4-mini","reason":"highRiskCyberActivity"}`,
			kind:      pb.CodexEvent_MODEL_REROUTED,
			titlePart: "模型",
			msgPart:   "gpt-5.4",
		},
		{
			name:      "context compacted",
			method:    "thread/compacted",
			raw:       `{"threadId":"thread-1","turnId":"turn-1"}`,
			kind:      pb.CodexEvent_CONTEXT_COMPACTED,
			titlePart: "上下文",
			msgPart:   "压缩",
		},
		{
			name:      "rate limit",
			method:    "account/rateLimits/updated",
			raw:       `{"rateLimits":{"primary":{"usedPercent":82,"windowDurationMins":300},"planType":"pro"}}`,
			kind:      pb.CodexEvent_RATE_LIMIT,
			titlePart: "用量",
			msgPart:   "82%",
		},
		{
			name:      "command output",
			method:    "item/commandExecution/outputDelta",
			raw:       `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1","delta":"正在编译..."}`,
			kind:      pb.CodexEvent_STATUS,
			titlePart: "命令",
			msgPart:   "正在编译",
		},
		{
			name:      "mcp progress",
			method:    "item/mcpToolCall/progress",
			raw:       `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-2","message":"正在读取资源"}`,
			kind:      pb.CodexEvent_STATUS,
			titlePart: "MCP",
			msgPart:   "正在读取资源",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := codexNotificationEvent(
				tt.method,
				json.RawMessage(tt.raw),
				"thread-1",
				"turn-1",
			)
			if event == nil {
				t.Fatal("expected an event")
			}
			if event.Kind != tt.kind {
				t.Fatalf("kind = %v, want %v", event.Kind, tt.kind)
			}
			if !strings.Contains(event.Title, tt.titlePart) {
				t.Fatalf("title = %q, want %q", event.Title, tt.titlePart)
			}
			if !strings.Contains(event.Message, tt.msgPart) {
				t.Fatalf("message = %q, want %q", event.Message, tt.msgPart)
			}
			if event.ThreadId != "thread-1" {
				t.Fatalf("thread id = %q", event.ThreadId)
			}
			if event.RawJson == "" {
				t.Fatal("expected raw json for diagnostics")
			}
		})
	}
}

func TestCodexTokenUsageNotificationMapsToUsageUpdate(t *testing.T) {
	update := codexTokenUsageUpdate(json.RawMessage(`{
		"threadId":"thread-1",
		"turnId":"turn-1",
		"tokenUsage":{
			"last":{
				"inputTokens":120,
				"cachedInputTokens":20,
				"cacheWriteInputTokens":3,
				"outputTokens":45,
				"reasoningOutputTokens":10,
				"totalTokens":165
			},
			"total":{
				"inputTokens":1200,
				"cachedInputTokens":200,
				"cacheWriteInputTokens":30,
				"outputTokens":450,
				"reasoningOutputTokens":100,
				"totalTokens":1650
			}
		}
	}`), "thread-1", "turn-1")

	if update == nil || update.Usage == nil || update.TotalUsage == nil {
		t.Fatalf("expected current and total usage, got %v", update)
	}
	if update.Usage.InputTokens != 120 || update.Usage.OutputTokens != 45 {
		t.Fatalf("current usage = %v", update.Usage)
	}
	if update.Usage.CacheReadTokens != 20 || update.Usage.CacheWriteTokens != 3 {
		t.Fatalf("current cache usage = %v", update.Usage)
	}
	if update.TotalUsage.InputTokens != 1200 || update.TotalUsage.OutputTokens != 450 {
		t.Fatalf("total usage = %v", update.TotalUsage)
	}
}
