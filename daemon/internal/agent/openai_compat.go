package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// oaHTTP 带超时的共享 HTTP 客户端:http.DefaultClient 没有超时,
// 网络一抖整轮任务就永久挂起(实测教训)。
var oaHTTP = &http.Client{Timeout: 180 * time.Second}

// OpenAICompatible 覆盖所有"说 OpenAI 方言"的服务:智谱 GLM、DeepSeek、
// Kimi、Qwen、Ollama(/v1)、以及 OpenAI 本尊。
//
// 【讲解】为什么先写它:国内直连、注册就有免费额度、协议公开。所谓
// "OpenAI 兼容"就是大家都照抄了 OpenAI 的 HTTP 接口格式(/chat/completions,
// SSE 流式,tools 工具调用),所以一份代码接全家。
type OpenAICompatible struct {
	id      string
	display string
	baseURL string
	apiKey  string
	models  []ModelSpec
}

func NewOpenAICompatible(id, display, baseURL, apiKey string, models []ModelSpec) *OpenAICompatible {
	return &OpenAICompatible{id: id, display: display, baseURL: baseURL, apiKey: apiKey, models: models}
}

func (p *OpenAICompatible) ID() string          { return p.id }
func (p *OpenAICompatible) DisplayName() string { return p.display }

func (p *OpenAICompatible) Models() []pb.ModelInfo {
	out := make([]pb.ModelInfo, 0, len(p.models))
	for _, m := range p.models {
		out = append(out, pb.ModelInfo{
			Id: m.ID, Provider: p.id, DisplayName: m.DisplayName,
			ContextWindow: int32(m.ContextWindow), SupportsThinking: m.Thinking,
			SupportsImages: m.Images, Available: true,
		})
	}
	return out
}

// ---- OpenAI 协议的 JSON 结构 ----

type oaMessage struct {
	Role       string       `json:"role"`
	Content    interface{}  `json:"content"` // string 或 分块数组(带图时)
	ToolCalls  []oaToolCall `json:"tool_calls,omitempty"`
	ToolCallID string       `json:"tool_call_id,omitempty"`
}

type oaToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"` // "function"
	Index    int    `json:"index,omitempty"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type oaTool struct {
	Type     string `json:"type"` // "function"
	Function struct {
		Name        string      `json:"name"`
		Description string      `json:"description"`
		Parameters  interface{} `json:"parameters"`
	} `json:"function"`
}

type oaRequest struct {
	Model    string      `json:"model"`
	Messages []oaMessage `json:"messages"`
	Tools    []oaTool    `json:"tools,omitempty"`
	Stream   bool        `json:"stream"`
}

type oaSSEDelta struct {
	Choices []struct {
		Delta struct {
			Content          string       `json:"content"`
			ReasoningContent string       `json:"reasoning_content"`
			ToolCalls        []oaToolCall `json:"tool_calls"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

// Run 执行一轮完整 agent 循环。
func (p *OpenAICompatible) Run(ctx context.Context, req TurnRequest, sink Sink, approve ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
	// 无密钥直接快速失败:绝不让轮次挂在无超时的 HTTP 请求上(手机端会永远转圈)
	if p.apiKey == "" {
		return nil, &pb.Usage{}, fmt.Errorf("%s 未配置 API key(本轮无法执行;在设置页配置或改用其他模型)", p.display)
	}
	model := req.Thread.ModelID
	if model == "" {
		if len(p.models) > 0 {
			model = p.models[0].ID
		}
	}

	tools := StdTools()
	oaTools := make([]oaTool, 0, len(tools))
	for _, t := range tools {
		var schema interface{}
		json.Unmarshal([]byte(t.Schema), &schema)
		ot := oaTool{Type: "function"}
		ot.Function.Name = t.Name
		ot.Function.Description = t.Description
		ot.Function.Parameters = schema
		oaTools = append(oaTools, ot)
	}

	// 1) 把存档对话 + 本轮输入转成 OpenAI 消息格式
	msgs := []oaMessage{{
		Role: "system",
		Content: "你是 PocketAgent,一个在用户电脑上工作的编程助手。工具执行结果会以 role=tool 返回。" +
			"文件路径一律相对工作区。改文件和跑命令前先说明意图。任务完成就总结,不要无限循环。",
	}}
	// 【四层记忆上下文注入】server 层在请求前组装好的前缀(TurnRequest.MemoryContextPrefix,
	// 仅 daemon 家注入)作为 system 补充段插在首条 user 消息之前,让模型带着
	// "会话元数据/已保存记忆/早期摘要/近期消息"开工;空前缀不发(省一条空消息)。
	// 其他 provider 不读该字段,此处是唯一消费点,注入对它们零影响。
	if prefix := strings.TrimSpace(req.MemoryContextPrefix); prefix != "" {
		msgs = append(msgs, oaMessage{
			Role:    "system",
			Content: "以下是 daemon 为本会话组装的记忆上下文(元数据/已保存记忆/早期摘要/近期消息),供参考:\n" + prefix,
		})
	}
	for _, m := range req.Thread.Messages {
		msgs = append(msgs, blocksToOpenAI(m.Role, m.Blocks)...)
	}
	msgs = append(msgs, blocksToOpenAI("user", req.InputBlocks)...)

	var usage pb.Usage // 累计用值,对外返回指针
	var finalBlocks []pb.ContentBlock

	// 2) 工具循环:最多 25 轮,防止模型无限自嗨
	for round := 0; round < 25; round++ {
		assistantBlocks, calls, finish, roundUsage, err := p.streamOnce(ctx, model, msgs, oaTools, sink)
		if roundUsage != nil { // 无用量帧时 streamOnce 返回 nil,累计值保持不变
			usage.InputTokens += roundUsage.InputTokens
			usage.OutputTokens += roundUsage.OutputTokens
		}
		if err != nil {
			return finalBlocks, &usage, err
		}
		finalBlocks = assistantBlocks

		if finish != "tool_calls" || len(calls) == 0 {
			return assistantBlocks, &usage, nil // 正常收尾(stop)
		}

		// 模型请求用工具:先回填 assistant 消息,再逐个执行
		am := oaMessage{Role: "assistant", ToolCalls: calls}
		if len(calls) > 0 {
			// 把这轮的正文文本也带上
			var sb strings.Builder
			// 按下标取址遍历,避免 range 复制 protobuf 消息(内嵌 MessageState 含锁)。
			for i := range assistantBlocks {
				if t, ok := assistantBlocks[i].Block.(*pb.ContentBlock_Text); ok {
					sb.WriteString(t.Text.Text)
				}
			}
			if sb.Len() > 0 {
				am.Content = sb.String()
			}
		}
		msgs = append(msgs, am)

		for _, c := range calls {
			c := c
			resultText := execWithApproval(ctx, tools, c, req.Workspace, sink, approve)
			msgs = append(msgs, oaMessage{Role: "tool", ToolCallID: c.ID, Content: resultText})
		}
		// 循环回去:带着工具结果再次请求模型
	}
	return finalBlocks, &usage, fmt.Errorf("工具循环超过 25 轮,已强制停止")
}

// streamOnce 发起一次流式请求,把增量实时喂给 sink,返回这轮的完整块。
// usage 改为指针返回:protobuf 消息内嵌 MessageState 含锁,按值 return 会拷贝锁。
func (p *OpenAICompatible) streamOnce(ctx context.Context, model string, msgs []oaMessage, tools []oaTool, sink Sink) (blocks []pb.ContentBlock, calls []oaToolCall, finish string, usage *pb.Usage, err error) {
	body, _ := json.Marshal(oaRequest{Model: model, Messages: msgs, Tools: tools, Stream: true})
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, nil, "", usage, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := oaHTTP.Do(httpReq)
	if err != nil {
		return nil, nil, "", usage, fmt.Errorf("请求 %s 失败: %w", p.baseURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		buf := new(bytes.Buffer)
		buf.ReadFrom(resp.Body)
		return nil, nil, "", usage, fmt.Errorf("API 返回 %d: %s", resp.StatusCode, clip(buf.String(), 500))
	}

	var text strings.Builder
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	callAcc := map[int]*oaToolCall{} // index → 聚合中的 tool call(参数是分片到的)

	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(line[5:])
		if payload == "" || payload == "[DONE]" {
			continue
		}
		var ev oaSSEDelta
		if err := json.Unmarshal([]byte(payload), &ev); err != nil {
			continue
		}
		if len(ev.Choices) == 0 {
			if ev.Usage != nil {
				if usage == nil {
					usage = &pb.Usage{}
				}
				usage.InputTokens = int64(ev.Usage.PromptTokens)
				usage.OutputTokens = int64(ev.Usage.CompletionTokens)
			}
			continue
		}
		ch := ev.Choices[0]
		if ch.FinishReason != nil {
			finish = *ch.FinishReason
		}
		if d := ch.Delta.ReasoningContent; d != "" {
			sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_ThinkingDelta{ThinkingDelta: &pb.ThinkingDelta{Delta: d}}})
		}
		if d := ch.Delta.Content; d != "" {
			text.WriteString(d)
			sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_TextDelta{TextDelta: &pb.TextDelta{Delta: d}}})
		}
		for _, tc := range ch.Delta.ToolCalls {
			acc, ok := callAcc[tc.Index]
			if !ok {
				clone := tc
				acc = &clone
				callAcc[tc.Index] = acc
			} else {
				acc.Function.Arguments += tc.Function.Arguments
				if tc.Function.Name != "" {
					acc.Function.Name = tc.Function.Name
				}
				if tc.ID != "" {
					acc.ID = tc.ID
				}
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, nil, finish, usage, err
	}

	// 按出现顺序输出聚合好的 calls
	for i := 0; i < len(callAcc); i++ {
		if c, ok := callAcc[i]; ok {
			c.Type = "function"
			calls = append(calls, *c)
		}
	}
	if text.Len() > 0 {
		blocks = append(blocks, blockText(text.String()))
	}
	return blocks, calls, finish, usage, nil
}

// execWithApproval 执行一个工具调用:通知手机(STARTED)→ 危险工具先审批 →
// 执行 → 通知结果(FINISHED)。返回给模型看的结果文本。
func execWithApproval(ctx context.Context, tools []Tool, c oaToolCall, ws string, sink Sink, approve ApprovalHook) string {
	callBlock := &pb.ToolCallBlock{Id: c.ID, Name: c.Function.Name, JsonArgs: c.Function.Arguments}
	sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_ToolCall{ToolCall: &pb.ToolCallUpdate{
		Phase: pb.ToolCallUpdate_STARTED, Call: callBlock,
	}}})

	tool := ToolByName(tools, c.Function.Name)
	if tool == nil {
		return "未知工具: " + c.Function.Name
	}
	if tool.Dangerous && approve == nil {
		// 【P0-1】危险工具遇到审批通道缺失(TestModel 诊断 RPC 等路径传 nil):
		// 宁可不执行,也绝不能绕过审批直接跑 —— 那是连问都不问就动用户电脑。
		// 回人话错误文本给模型;非危险工具不受影响,照常执行。
		return "审批通道不可用,已拒绝执行危险工具"
	}
	if tool.Dangerous {
		// 【vet】直接传 callBlock 指针(原来解引用拷贝一份会被 copylocks 拒绝)。
		ok, note, err := approve(ctx, callBlock)
		if err != nil {
			return "审批通道错误: " + err.Error()
		}
		if !ok {
			return "用户拒绝了该操作。" + note
		}
	}
	out, err := tool.Exec(ctx, ws, json.RawMessage(c.Function.Arguments))
	var resultText string
	if err != nil {
		resultText = "执行出错: " + err.Error()
	} else {
		resultText = out
	}
	sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_ToolCall{ToolCall: &pb.ToolCallUpdate{
		Phase: pb.ToolCallUpdate_FINISHED, Call: callBlock,
		Result: &pb.ToolResultBlock{CallId: c.ID, Ok: err == nil, Output: clip(resultText, 8000)},
	}}})
	return resultText
}

func base64Of(b []byte) string { return base64.StdEncoding.EncodeToString(b) }

// blocksToOpenAI 把我们的 ContentBlock 历史翻译成 OpenAI 消息。
func blocksToOpenAI(role string, blocks []pb.ContentBlock) []oaMessage {
	var out []oaMessage
	var texts []interface{}
	var calls []oaToolCall
	pendingResults := map[string]string{}

	// 按下标取址遍历,避免 range 复制 protobuf 消息(内嵌 MessageState 含锁)。
	for i := range blocks {
		b := &blocks[i]
		switch v := b.Block.(type) {
		case *pb.ContentBlock_Text:
			texts = append(texts, map[string]interface{}{"type": "text", "text": v.Text.Text})
		case *pb.ContentBlock_Image:
			texts = append(texts, map[string]interface{}{
				"type": "image_url",
				"image_url": map[string]string{
					"url": "data:" + v.Image.MediaType + ";base64," + base64Of(v.Image.Data),
				},
			})
		case *pb.ContentBlock_ToolCall:
			c := oaToolCall{ID: v.ToolCall.Id, Type: "function"}
			c.Function.Name = v.ToolCall.Name
			c.Function.Arguments = v.ToolCall.JsonArgs
			calls = append(calls, c)
		case *pb.ContentBlock_ToolResult:
			pendingResults[v.ToolResult.CallId] = v.ToolResult.Output
		}
	}

	// 没有工具调用的普通消息:直接 content 数组
	if len(calls) == 0 {
		m := oaMessage{Role: role}
		if len(texts) == 1 {
			if t, ok := texts[0].(map[string]interface{}); ok {
				m.Content = t["text"]
			}
		} else if len(texts) > 0 {
			m.Content = texts
		}
		return []oaMessage{m}
	}

	// assistant 带工具调用:正文 + tool_calls
	m := oaMessage{Role: role, ToolCalls: calls}
	if len(texts) > 0 {
		var sb strings.Builder
		for _, t := range texts {
			if tm, ok := t.(map[string]interface{}); ok {
				sb.WriteString(fmt.Sprint(tm["text"]))
			}
		}
		m.Content = sb.String()
	}
	out = append(out, m)
	// 工具结果各自成一条 role=tool
	for _, c := range calls {
		out = append(out, oaMessage{Role: "tool", ToolCallID: c.ID, Content: pendingResults[c.ID]})
	}
	return out
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	// 保持字节预算 n 不变,但回退到最后一个完整 rune 的边界再截 —— 修复 CJK
	// 半字符缺陷:此前 s[:n] 可能把一个多字节汉字劈成非法 UTF-8,发给 OpenAI
	// 兼容端点会成乱码。
	limit := n
	for limit > 0 && !utf8.RuneStart(s[limit]) {
		limit--
	}
	return s[:limit] + "\n...[截断]"
}
