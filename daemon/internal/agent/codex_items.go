// codex_items.go —— item/started|item/completed 里"工具类"ThreadItem 的工具卡投影。
//
// 【背景:2026-10-07 字段级覆盖审计】ThreadItem 联合体(vendored 0.159.2 schema
// definitions.ThreadItem,0.160.0 二进制同面)共 19 类。daemon 原有专案分支只认
// 五类(agentMessage/commandExecution/contextCompaction/subAgentActivity/
// collabAgentToolCall),工具属性最强的三类 —— fileChange(改了哪些文件)、
// mcpToolCall(调了哪个 MCP 工具、成没成)、webSearch(搜了什么)—— 一律掉进
// maybeRecordOpaqueItem 的"原样透传":可见但不成形,用户看到的是一条
// "codex · item/completed:mcpToolCall 原生状态通知"而不是工具卡。
// 本文件把这三类投影进既有 pb.TurnServerMessage_ToolCall 变体(开始卡/收尾卡),
// 与 commandExecution 的 shell 卡同一通路。
//
// 【其余 item 类型为什么不在这里】userMessage(用户自己的输入)、reasoning
// (已投影为 ThinkingDelta 流)、plan(计划面由 turn/plan/updated 承担)、
// hookPrompt/functionCallOutput/dynamicToolCall/imageView/sleep/imageGeneration/
// enteredReviewMode/exitedReviewMode:或已有专案通路,或属低频/桌面面 —— 维持
// maybeRecordOpaqueItem 的原样透传 + 台账留痕(限频+脱敏自动生效),不伪造卡片。
// 上游真加新类型时,不认识的类型名依旧落 UNKNOWN_NOTIFICATION,诊断页可见。
package agent

import (
	"encoding/json"
	"fmt"
	"strings"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// 工具卡名称。Android 端按 name 决定渲染方式:file_change 已有(审批卡同款
// 名称);mcp_tool / web_search 是新起的名,未知 name 走通用工具卡,卡内 JSON
// 如实带 server/tool/query —— 绝不冒充 shell。
const (
	toolNameMcpTool   = "mcp_tool"
	toolNameWebSearch = "web_search"
)

// codexItemToolCard 把一条 item/started|item/completed 通知翻成工具卡帧。
//
// 返回 nil 表示"不属于这三类 / 异线程 / 解析失败":调用方继续走既有分支或
// 不透明透传,不伪造、不吞帧。started=true 产生 STARTED 卡,false 产生
// FINISHED 卡(阶段与结果取 item 自身的 status)。
func codexItemToolCard(method string, params json.RawMessage, codexTID string) *pb.TurnServerMessage {
	var n struct {
		ThreadId string          `json:"threadId"`
		Item     json.RawMessage `json:"item"`
	}
	if json.Unmarshal(params, &n) != nil || n.ThreadId != codexTID || len(n.Item) == 0 {
		return nil
	}
	var head struct {
		Type   string `json:"type"`
		Id     string `json:"id"`
		Status string `json:"status"`
	}
	if json.Unmarshal(n.Item, &head) != nil {
		return nil
	}
	started := method == "item/started"
	switch head.Type {
	case "fileChange":
		// FileChangeThreadItem:{changes:[{path,diff,kind{type}}],status}。
		// 卡片只列路径清单:diff 大且已有 turn/diff/updated 与
		// item/fileChange/patchUpdated 两条专案通路展示,这里不再复制。
		var it struct {
			Changes []struct {
				Path string `json:"path"`
				Kind struct {
					Type string `json:"type"`
				} `json:"kind"`
			} `json:"changes"`
		}
		if json.Unmarshal(n.Item, &it) != nil {
			return nil
		}
		paths := make([]string, 0, len(it.Changes))
		for _, c := range it.Changes {
			if c.Path != "" {
				paths = append(paths, c.Path)
			}
		}
		args := map[string]interface{}{"files": paths}
		msg := &pb.TurnServerMessage{Msg: &pb.TurnServerMessage_ToolCall{ToolCall: &pb.ToolCallUpdate{
			Call: &pb.ToolCallBlock{Id: head.Id, Name: toolNameFileChange, JsonArgs: mustJSON(args)},
		}}}
		if started {
			msg.GetToolCall().Phase = pb.ToolCallUpdate_STARTED
			return msg
		}
		msg.GetToolCall().Phase = pb.ToolCallUpdate_FINISHED
		msg.GetToolCall().Result = &pb.ToolResultBlock{
			CallId: head.Id,
			Ok:     head.Status == "completed" || head.Status == "",
			Output: fileChangeOutcomeLabel(head.Status, len(paths)),
		}
		return msg

	case "mcpToolCall":
		// McpToolCallThreadItem:{server, tool, status, error{message}, result…}。
		// result.content 是 MCP 内容块联合体,这里不深解析 —— 成败与错误消息
		// 才是用户要的;原文始终可经 RawJson 通道(透传/台账)追溯。
		var it struct {
			Server string `json:"server"`
			Tool   string `json:"tool"`
			Error  *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(n.Item, &it) != nil {
			return nil
		}
		msg := &pb.TurnServerMessage{Msg: &pb.TurnServerMessage_ToolCall{ToolCall: &pb.ToolCallUpdate{
			Call: &pb.ToolCallBlock{
				Id:   head.Id,
				Name: toolNameMcpTool,
				JsonArgs: mustJSON(map[string]interface{}{
					"server": it.Server, "tool": it.Tool,
				}),
			},
		}}}
		if started {
			msg.GetToolCall().Phase = pb.ToolCallUpdate_STARTED
			return msg
		}
		output := ""
		if it.Error != nil {
			output = strings.TrimSpace(it.Error.Message)
		}
		msg.GetToolCall().Phase = pb.ToolCallUpdate_FINISHED
		msg.GetToolCall().Result = &pb.ToolResultBlock{
			CallId: head.Id,
			Ok:     head.Status == "completed",
			Output: clip(output, 2000),
		}
		return msg

	case "webSearch":
		// WebSearchThreadItem:{query, action{…}, results[]}。action 是四态联合体
		// (search/openPage/findInPage/other),search 态的 query 最常见;action
		// 细节留原样透传通道,卡片带 query 足以回答"它上网干了什么"。
		var it struct {
			Query string `json:"query"`
		}
		if json.Unmarshal(n.Item, &it) != nil {
			return nil
		}
		msg := &pb.TurnServerMessage{Msg: &pb.TurnServerMessage_ToolCall{ToolCall: &pb.ToolCallUpdate{
			Call: &pb.ToolCallBlock{
				Id:   head.Id,
				Name: toolNameWebSearch,
				JsonArgs: mustJSON(map[string]interface{}{
					"query": it.Query,
				}),
			},
		}}}
		if started {
			msg.GetToolCall().Phase = pb.ToolCallUpdate_STARTED
			return msg
		}
		msg.GetToolCall().Phase = pb.ToolCallUpdate_FINISHED
		msg.GetToolCall().Result = &pb.ToolResultBlock{
			CallId: head.Id,
			Ok:     true,
			Output: "搜索:" + it.Query,
		}
		return msg
	}
	return nil
}

// fileChangeOutcomeLabel 把 FileChangeThreadItem.status(PatchApplyStatus:
// inProgress/completed/failed/declined)翻成收尾卡的一句话。
func fileChangeOutcomeLabel(status string, files int) string {
	n := fmt.Sprintf("%d 个文件", files)
	switch status {
	case "completed", "":
		return n + "的变更已完成"
	case "failed":
		return n + "的变更未能完成"
	case "declined":
		return n + "的变更未获批准,已跳过"
	case "inProgress":
		return n + "仍在变更中"
	}
	return n + "(status=" + status + ")"
}
