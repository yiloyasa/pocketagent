package hermesapp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Initialize 完成 ACP 握手,返回适配器的模型目录(availableModels)与当前选择。
// 协议:agent-client-protocol 0.9.0 —— fs 能力 daemon 侧不支持(回 method not
// found 由 readLoop 兜底),适配器按"无文件系统回调"降级,实测可跑通全轮。
type InitializeResult struct {
	ProtocolVersion int    `json:"protocolVersion"`
	AgentInfo       struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"agentInfo"`
	AuthMethods []struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
	} `json:"authMethods"`
}

func (c *Client) Initialize(ctx context.Context) (*InitializeResult, error) {
	raw, err := c.callCtx(ctx, "initialize", map[string]any{
		"protocolVersion":   1,
		"clientCapabilities": map[string]any{
			"fs": map[string]any{"readTextFile": false, "writeTextFile": false},
		},
	})
	if err != nil {
		return nil, err
	}
	out := &InitializeResult{}
	if err := json.Unmarshal(raw, out); err != nil {
		return nil, fmt.Errorf("hermes initialize 应答解析: %w", err)
	}
	return out, nil
}

// SessionNew 建一个 ACP 会话(cwd=工作区)。返回 sessionId + 模型目录快照。
type SessionNewResult struct {
	SessionID      string `json:"sessionId"`
	CurrentModelID string `json:"currentModelId"`
	Models         struct {
		AvailableModels []struct {
			ModelID     string `json:"modelId"`
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"availableModels"`
	} `json:"models"`
	Modes struct {
		CurrentModeID string `json:"currentModeId"`
		AvailableModes []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"availableModes"`
	} `json:"modes"`
}

func (c *Client) SessionNew(ctx context.Context, cwd string) (*SessionNewResult, error) {
	raw, err := c.callCtx(ctx, "session/new", map[string]any{
		"cwd": cwd, "mcpServers": []any{},
	})
	if err != nil {
		return nil, err
	}
	out := &SessionNewResult{}
	if err := json.Unmarshal(raw, out); err != nil {
		return nil, fmt.Errorf("hermes session/new 应答解析: %w", err)
	}
	if out.SessionID == "" {
		return nil, fmt.Errorf("hermes session/new 没回 sessionId")
	}
	return out, nil
}

// SetSessionModel 切会话模型(modelId 形如 "deepseek:deepseek-flash")。
func (c *Client) SetSessionModel(ctx context.Context, sessionID, modelID string) error {
	_, err := c.callCtx(ctx, "session/set_session_model", map[string]any{
		"sessionId": sessionID, "modelId": modelID,
	})
	return err
}

// StreamEvent 一条 session/update 投影(已解析的最小面,供 Provider 映射)。
type StreamEvent struct {
	SessionID string
	// 以下按 Update 分类互斥:
	MessageChunk string // agent_message_chunk 正文增量
	ThoughtChunk string // agent_thought_chunk 思考增量
	ToolCall     *ToolCallEvent
	Plan         []PlanEntry
	Raw          json.RawMessage
}

type ToolCallEvent struct {
	Kind   string // started / completed(ACP 是 tool_call / tool_call_update)
	Title  string
	Status string
	Output string
}

type PlanEntry struct {
	Content string
	Status  string
}

// ParseSessionUpdate 把一条 session/update params 解析成 StreamEvent。
// 未知 sessionUpdate 变体如实带 Raw 上抛(不静默吞,与未知事件同一纪律)。
func ParseSessionUpdate(params json.RawMessage) StreamEvent {
	var p struct {
		SessionID string `json:"sessionId"`
		Update    struct {
			SessionUpdate string `json:"sessionUpdate"`
			Content       *struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			Title  string `json:"title"`
			Status string `json:"status"`
			Kind   string `json:"kind"`
			RawID  string `json:"toolCallId"`
			Content2 []struct {
				Text string `json:"text"`
			} `json:"content"`
			Entries []struct {
				Content string `json:"content"`
				Status  string `json:"status"`
			} `json:"entries"`
		} `json:"update"`
	}
	ev := StreamEvent{Raw: params}
	if json.Unmarshal(params, &p) != nil {
		return ev
	}
	ev.SessionID = p.SessionID
	switch p.Update.SessionUpdate {
	case "agent_message_chunk":
		ev.MessageChunk = p.Update.Content.Text
	case "agent_thought_chunk":
		ev.ThoughtChunk = p.Update.Content.Text
	case "tool_call", "tool_call_update":
		out := ""
		for _, c := range p.Update.Content2 {
			out += c.Text
		}
		ev.ToolCall = &ToolCallEvent{
			Kind:   p.Update.SessionUpdate,
			Title:  p.Update.Title,
			Status: p.Update.Status,
			Output: out,
		}
	case "plan":
		for _, e := range p.Update.Entries {
			ev.Plan = append(ev.Plan, PlanEntry{Content: e.Content, Status: e.Status})
		}
	default:
		// user_message_chunk / available_commands_update / current_mode_update 等
		// daemon 侧无消费方:静默留 Raw,不算未知事件(它们是 ACP 协议已知成员)。
	}
	return ev
}

// Prompt 发一轮输入并阻塞到轮次终局。流式增量经 OnSessionUpdate 钩子实时回调
// (挂钩要在调用本方法前完成);返回值 = stopReason(end_turn / refusal / cancelled…)。
func (c *Client) Prompt(ctx context.Context, sessionID, text string) (string, error) {
	raw, err := c.callCtx(ctx, "session/prompt", map[string]any{
		"sessionId": sessionID,
		"prompt":    []map[string]any{{"type": "text", "text": text}},
	})
	if err != nil {
		return "", err
	}
	var out struct {
		StopReason string `json:"stopReason"`
	}
	if json.Unmarshal(raw, &out) != nil {
		return "", nil
	}
	return out.StopReason, nil
}

func (c *Client) callCtx(ctx context.Context, method string, params any) (json.RawMessage, error) {
	type result struct {
		raw json.RawMessage
		err error
	}
	done := make(chan result, 1)
	go func() {
		raw, err := c.Call(method, params)
		done <- result{raw, err}
	}()
	select {
	case r := <-done:
		return r.raw, r.err
	case <-ctx.Done():
		// 放弃:迟到应答经 readLoop 静默丢(pending 已随 reap/清理);调用方按
		// 超时降级。适配器进程由调用方 Close 收尸。
		return nil, ctx.Err()
	}
}

// StopReasonIsError 桌面适配器把 provider 失败也包装成 end_turn + 错误正文
// (实测:DeepSeek 400 时 stopReason=end_turn,正文是 turn_failure_copy 文案),
// 判定"这轮真的成了"只能看正文内容。给调用方一个保守启发:refusal/cancelled
// 视为失败;end_turn 交由正文判定(正文由调用方累计)。
func StopReasonIsError(stop string) bool {
	return strings.EqualFold(stop, "refusal") || strings.EqualFold(stop, "cancelled")
}

var _ = time.Second // 预留(超时预算由调用方定)
