package zcodeapp

// subagent.go —— ZCode 子代理工具调用的识别(情况 B 的透出增强前置判定)。
//
// 【背景】ZCode 的子代理是 "Agent"/"Task" 工具(Claude Code 兼容别名,
// zcode.cjs 内 Ula/Ofo 定义,工具名常量 Mge="Agent"),与 codex 的
// subAgentActivity/collabAgentToolCall 独立 ThreadItem 不是一回事。ZCode 的
// app-server 协议不发射任何子代理专用事件:session/event 的 Type 集合里没有
// 子代理项(见 daemon/internal/agent/zcode.go 的 Run 分发 switch,只认
// session.updated / model.streaming / tool.updated / turn.completed 等),
// 子代理内部对话完全不透出 —— 手机端只能拿到这一次 tool.updated 的入参
// (prompt/description)与结果(summary + 返回文本)。这是项目铁律
// "识别不了≠出错、不伪造"的硬边界:此处只把原生工具调用参数/结果透出得更
// 完整,绝不编造子代理对话内容。
//
// 【识别依据,纯结构、不写死工具名】ZCode Agent 工具的入参 schema(zcode.cjs 内
// 常量 Mfo,oneOf 的 completed/async_launched 两态)稳定带有:prompt(必填) + 至少
// 一个子代理特征键 {description, agentType/subagent_type, outputFile,
// parentToolCallId}。据此判定"这是一次子代理委派",与具体工具名
// (Agent/Task/subagent)解耦 —— ZCode 改名也不误判,也不会把普通工具
// (Bash 入参是 command,Write 入参是 file_path)误判为子代理。
//
// 这些都是 ZCode 自己协议里的稳定字段,不是我们猜测的形状;识别失败只返回
// false(当普通工具处理),不报错、不降级。
import (
	"encoding/json"
	"strings"
)

// ToolInputIsSubagent 判定一次 tool.updated 的入参是否属于 ZCode 子代理委派。
// input 为空/非法 JSON → false(当普通工具)。必须同时具备 prompt(子代理委派的
// 核心入参)与一个子代理特征键,才认定 —— 双条件把误判面压到最低。
func ToolInputIsSubagent(input json.RawMessage) bool {
	if len(input) == 0 {
		return false
	}
	var head struct {
		Prompt           string `json:"prompt"`
		Description      string `json:"description"`
		AgentType        string `json:"agentType"`
		SubagentType     string `json:"subagent_type"`
		OutputFile       string `json:"outputFile"`
		ParentToolCallID string `json:"parentToolCallId"`
	}
	if json.Unmarshal(input, &head) != nil {
		return false
	}
	if strings.TrimSpace(head.Prompt) == "" {
		return false
	}
	return head.Description != "" || head.AgentType != "" ||
		head.SubagentType != "" || head.OutputFile != "" || head.ParentToolCallID != ""
}

// subagentInputMarkerKeys 仅供单测/文档列举子代理特征键(不进运行时热路径)。
func subagentInputMarkerKeys() []string {
	return []string{"description", "agentType", "subagent_type", "outputFile", "parentToolCallId"}
}
