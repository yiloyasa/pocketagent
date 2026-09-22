package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// EchoProvider 不调任何大模型 API,把收到的话"演"一遍 agent 的完整过程:
// 思考增量 → 文本增量 → 工具调用(真的列目录)→ 结果 → 完成。
// 用途:①手机端 UI 全链路联调不用花一分钱;②没配任何 key 时 app 依然能玩。
type EchoProvider struct{}

func NewEcho() *EchoProvider { return &EchoProvider{} }

func (e *EchoProvider) ID() string          { return "echo" }
func (e *EchoProvider) DisplayName() string { return "回声模式(无需 API key)" }

func (e *EchoProvider) Models() []pb.ModelInfo {
	return []pb.ModelInfo{{
		Id: "echo-1", Provider: "echo", DisplayName: "Echo(内置)",
		ContextWindow: 8192, Available: true,
	}}
}

func (e *EchoProvider) Run(ctx context.Context, req TurnRequest, sink Sink, approve ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
	// P0.5 最低契约：真实轮次里走一遍公共运行时记忆（用户输入先入记忆 + checkpoint）。
	// 这是"契约被使用而不只是被交付"的证据点 —— 说明见 echo_runtime.go。
	e.recordTurn(req.Thread, "user", req.InputBlocks)

	var userText strings.Builder
	for i := range req.InputBlocks {
		b := &req.InputBlocks[i]
		if t, ok := b.Block.(*pb.ContentBlock_Text); ok {
			userText.WriteString(t.Text.Text)
		}
	}

	stream := func(delta string, isThinking bool) {
		if isThinking {
			sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_ThinkingDelta{ThinkingDelta: &pb.ThinkingDelta{Delta: delta}}})
		} else {
			sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_TextDelta{TextDelta: &pb.TextDelta{Delta: delta}}})
		}
		time.Sleep(15 * time.Millisecond)
	}

	stream("用户说:「"+truncate(userText.String(), 120)+"」,我先看看工作区里有什么……", true)

	// 真的调一次工具(不需要审批的 list_dir)
	tools := StdTools()
	callID := "call_echo_1"
	sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_ToolCall{ToolCall: &pb.ToolCallUpdate{
		Phase: pb.ToolCallUpdate_STARTED,
		Call:  &pb.ToolCallBlock{Id: callID, Name: "list_dir", JsonArgs: `{"path":"."}`},
	}}})
	out, err := ToolByName(tools, "list_dir").Exec(ctx, req.Workspace, json.RawMessage(`{"path":"."}`))
	var resultText string
	if err != nil {
		resultText = "出错: " + err.Error()
	} else {
		lines := strings.Split(strings.TrimSpace(out), "\n")
		if len(lines) > 30 {
			lines = lines[:30]
		}
		resultText = strings.Join(lines, "\n")
	}
	sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_ToolCall{ToolCall: &pb.ToolCallUpdate{
		Phase:  pb.ToolCallUpdate_FINISHED,
		Call:   &pb.ToolCallBlock{Id: callID, Name: "list_dir", JsonArgs: `{"path":"."}`},
		Result: &pb.ToolResultBlock{CallId: callID, Ok: err == nil, Output: resultText},
	}}})

	for _, w := range strings.Fields("我是回声模式,没有配置大模型 API key。你的消息我收到了,工作区目录我也看过了(见上方的工具调用)。配置 GLM_API_KEY 环境变量后重启 daemon,我就会变成真正的人工智能。") {
		stream(w+" ", false)
	}

	blocks := []pb.ContentBlock{
		blockText(fmt.Sprintf("收到:%s\n\n(工作区 %s 共 %d 个条目)", truncate(userText.String(), 500), req.Workspace, strings.Count(resultText, "\n")+1)),
	}
	usage := &pb.Usage{InputTokens: int64(len(userText.String()) / 3), OutputTokens: 120}
	// 助手回复同样入记忆：Echo 的会话事实由 daemon 侧持有（SessionModel=DAEMON），
	// 不落 checkpoint 则重启后无从恢复。
	e.recordTurn(req.Thread, "assistant", blocks)
	return blocks, usage, nil
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func blockText(t string) pb.ContentBlock {
	return pb.ContentBlock{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: t}}}
}
