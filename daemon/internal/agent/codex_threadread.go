// codex_threadread.go —— thread/read 全量拉取桥接(T-SUBVIS)。
//
// 【为什么需要】codex collab spawnAgent 型子代理是真线程(S1P 已登记存根),
// 但子线程的**对话内容**不随任何通知流经 daemon:
//   - 子线程的 item/* 通知带的是子线程自己的 threadId,daemon 的轮次管线按
//     threadId 过滤后全部丢弃(只认当前轮的线程);
//   - 桌面端独占的 rollout 文件 daemon 不读。
//
// thread/read {threadId, includeTurns:true} 是 app-server 给外部 GUI 的唯一
// 全量读取口子(schema v2 ThreadReadParams/ThreadReadResponse,0.155.1 实测),
// 返回完整 Thread(turns+items)。它是"子代理对话内容可达"的通道真值
// (T-SUBVIS 捕获:docs/verify/evidence/codex-subagent-spawn-capture.md)。
//
// 【透传边界】本文件只做形状解析,不组装、不补写:turn 里有什么 item 类型就
// 暴露什么,认不出的类型原样保留 Type 由上层决定展示(codex_subagent_history
// 的落库规则见其文件头)。原生没给的东西(逐 token 流式、思考全文)这里也没有。
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/pocketagent/daemon/internal/codexapp"
)

// CodexTurnItem 是 thread/read 一个 turn item 的**保真摘录**:
// 文本类(agentMessage/userMessage)带全文;命令类带命令与聚合输出;
// 其余类型只带 Type —— 不认识的不是错误,是留给上层透传的数据。
type CodexTurnItem struct {
	Type string // agentMessage / userMessage / commandExecution / collabAgentToolCall / …
	Text string // agentMessage/userMessage 正文(其余为空)
	// 命令执行类
	Command  string
	Output   string
	ExitCode *int
	// 协作工具调用类(子代理自己再派子代理时出现)
	CollabTool        string
	CollabStatus      string
	ReceiverThreadIDs []string
}

// Role 推导 item 的会话角色:文本类归 user/assistant,其余归 ""(非对话内容)。
func (it CodexTurnItem) Role() string {
	switch it.Type {
	case "userMessage":
		return "user"
	case "agentMessage":
		return "assistant"
	}
	return ""
}

// CodexThreadTurn 是 thread/read 里的一个 turn。
type CodexThreadTurn struct {
	ID     string
	Status string
	Items  []CodexTurnItem
}

// CodexThreadSnapshot 是 thread/read 应答里我们关心的部分。
type CodexThreadSnapshot struct {
	ID             string
	ParentThreadID string // 非空 = 子代理线程(schema:"only be set if this thread is a subagent")
	AgentNickname  string
	Preview        string
	Path           string // 桌面端 rollout 落盘路径(诊断留痕)
	Turns          []CodexThreadTurn
}

// ReadThreadFull 拉一条 codex 线程的全量快照(只读)。
// codexThreadID 是 codex 侧线程 id;includeTurns=true 才带 turns。
func (p *CodexProvider) ReadThreadFull(ctx context.Context, codexThreadID string) (*CodexThreadSnapshot, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(codexThreadID) == "" {
		return nil, fmt.Errorf("thread/read 需要 codex threadId")
	}
	var out *CodexThreadSnapshot
	err := p.withStatusClient(func(ctx context.Context, c *codexapp.Client) error {
		var err error
		// *codexapp.Client 天然满足 codexJSONCaller(codex_read.go 定义的窄接口),
		// 可测核心直接面对接口。
		out, err = codexThreadReadVia(ctx, c, codexThreadID, true)
		return err
	})
	if err != nil {
		log.Printf("[codex] thread/read 失败 thread=%s: %v", codexThreadID, err)
	}
	return out, err
}

// codexThreadReadVia 是 ReadThreadFull 的可测核心:直接面对 codexJSONCaller。
func codexThreadReadVia(ctx context.Context, c codexJSONCaller, codexThreadID string, includeTurns bool) (*CodexThreadSnapshot, error) {
	if c == nil {
		return nil, fmt.Errorf("codex caller 为空")
	}
	params := map[string]interface{}{"threadId": codexThreadID}
	if includeTurns {
		params["includeTurns"] = true
	}
	raw, err := c.CallCtx(ctx, "thread/read", params)
	if err != nil {
		return nil, fmt.Errorf("thread/read: %w", err)
	}
	return parseCodexThreadRead(raw)
}

// parseCodexThreadRead 解析 thread/read 应答(单测钉形状)。
func parseCodexThreadRead(raw json.RawMessage) (*CodexThreadSnapshot, error) {
	var res struct {
		Thread struct {
			ID             string  `json:"id"`
			ParentThreadId *string `json:"parentThreadId"`
			AgentNickname  *string `json:"agentNickname"`
			Preview        string  `json:"preview"`
			Path           string  `json:"path"`
			Turns          []struct {
				ID     string            `json:"id"`
				Status string            `json:"status"`
				Items  []json.RawMessage `json:"items"`
			} `json:"turns"`
		} `json:"thread"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, fmt.Errorf("thread/read 应答无法解析: %w", err)
	}
	t := res.Thread
	out := &CodexThreadSnapshot{
		ID:             t.ID,
		ParentThreadID: derefStr(t.ParentThreadId),
		AgentNickname:  derefStr(t.AgentNickname),
		Preview:        t.Preview,
		Path:           t.Path,
	}
	for _, turn := range t.Turns {
		ct := CodexThreadTurn{ID: turn.ID, Status: turn.Status}
		for _, ir := range turn.Items {
			ct.Items = append(ct.Items, parseCodexTurnItem(ir))
		}
		out.Turns = append(out.Turns, ct)
	}
	return out, nil
}

// parseCodexTurnItem 解析单个 item:只摘已知字段,未知类型保 Type。
func parseCodexTurnItem(raw json.RawMessage) CodexTurnItem {
	var it CodexTurnItem
	var head struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(raw, &head) != nil {
		return it
	}
	it.Type = head.Type
	switch head.Type {
	case "agentMessage":
		var s struct {
			Text string `json:"text"`
		}
		_ = json.Unmarshal(raw, &s)
		it.Text = s.Text
	case "userMessage":
		var s struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		}
		_ = json.Unmarshal(raw, &s)
		texts := make([]string, 0, len(s.Content))
		for _, c := range s.Content {
			texts = append(texts, c.Text)
		}
		it.Text = strings.Join(texts, "\n")
	case "commandExecution":
		var s struct {
			Command          string `json:"command"`
			AggregatedOutput string `json:"aggregatedOutput"`
			ExitCode         *int   `json:"exitCode"`
		}
		_ = json.Unmarshal(raw, &s)
		it.Command, it.Output, it.ExitCode = s.Command, s.AggregatedOutput, s.ExitCode
	case "collabAgentToolCall":
		var s struct {
			Tool              string   `json:"tool"`
			Status            string   `json:"status"`
			ReceiverThreadIds []string `json:"receiverThreadIds"`
		}
		_ = json.Unmarshal(raw, &s)
		it.CollabTool, it.CollabStatus, it.ReceiverThreadIDs = s.Tool, s.Status, s.ReceiverThreadIds
	}
	return it
}
