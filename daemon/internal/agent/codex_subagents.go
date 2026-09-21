// codex_subagents.go —— codex 子代理事件的透传接入(L1)。
//
// 【背景】codex 0.149.1 的子代理活动不是独立通知方法,而是藏在 item/started 与
// item/completed 通知的两类 ThreadItem 里(此前这两个分支不认识它们,直接静默 ——
// 违反 README §7"能透传就透传"):
//
//	subAgentActivity     子代理活动刻度:{agentPath, agentThreadId, id, kind(started/interacted/interrupted)}
//	                     —— 无正文,纯"它还活着"的心跳;
//	collabAgentToolCall  协作工具调用:{tool(spawnAgent/sendInput/resumeAgent/wait/closeAgent),
//	                     status(inProgress/completed/failed), senderThreadId, receiverThreadIds,
//	                     agentsStates(map<threadId,{status(7态),message?}>)}
//
// 【做法】把 item 翻成工具卡风格的 CodexEvent(Kind=STATUS;CodexEvent 没有
// TOOL_CALL 这个 Kind,工具卡的"卡"由前端对 STATUS 事件的呈现承担):
// 标题带 agentPath / tool 名,正文带 agentsStates 概览("2 个子代理:1 运行 1 完成"),
// RawJson 走主线 A 的 redactNativePayload(32KiB 上限 + token 打码)脱敏透传 ——
// 这是 0.149.1 实际发射形状的留档,后续按真值演进,不靠猜。
//
// 【限频】interacted 可能高频刷屏,复用 native_notifications.go 的既有限频语义
// (每键 2 条、最多 64 键),键 = item 类型 + 形状判别(kind 或 tool:status)。
// 被限频只是"不再重复播报",不是丢弃诊断 —— RawJson 始终随前几条透传过。
package agent

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// ---- 通知 JSON 形状(只解析用得到的字段;缺字段不报错,未知值原样透传)----

type subAgentActivityItem struct {
	Type          string `json:"type"`
	Id            string `json:"id"`
	AgentPath     string `json:"agentPath"`
	AgentThreadId string `json:"agentThreadId"`
	Kind          string `json:"kind"`
}

type collabAgentStateItem struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type collabAgentToolCallItem struct {
	Type              string                          `json:"type"`
	Id                string                          `json:"id"`
	Tool              string                          `json:"tool"`
	Status            string                          `json:"status"`
	SenderThreadId    string                          `json:"senderThreadId"`
	ReceiverThreadIds []string                        `json:"receiverThreadIds"`
	AgentsStates      map[string]collabAgentStateItem `json:"agentsStates"`
	Prompt            string                          `json:"prompt"`
	Model             string                          `json:"model"`
	ReasoningEffort   string                          `json:"reasoningEffort"`
}

// isSubagentItemType 判断 item 类型是否属于本文件管的两类子代理 item。
func isSubagentItemType(t string) bool {
	return t == "subAgentActivity" || t == "collabAgentToolCall"
}

// subagentEvent 把一条 item/started|item/completed 通知翻成子代理 CodexEvent。
//
// 返回 (事件, 限频键)。不是这两类 item、或 threadId 不属于当前线程时返回
// (nil, "") —— 调用方继续走既有分支/default 兜底,识别不了的≠出错。
func subagentEvent(raw json.RawMessage, codexTID string) (*pb.CodexEvent, string) {
	var n struct {
		ThreadId string          `json:"threadId"`
		Item     json.RawMessage `json:"item"`
	}
	if json.Unmarshal(raw, &n) != nil || n.ThreadId != codexTID || len(n.Item) == 0 {
		return nil, ""
	}
	var head struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(n.Item, &head) != nil {
		return nil, ""
	}
	switch head.Type {
	case "subAgentActivity":
		var it subAgentActivityItem
		if json.Unmarshal(n.Item, &it) != nil {
			return nil, ""
		}
		return subagentActivityEvent(it, n.Item, codexTID), "subAgentActivity:" + it.Kind
	case "collabAgentToolCall":
		var it collabAgentToolCallItem
		if json.Unmarshal(n.Item, &it) != nil {
			return nil, ""
		}
		return collabToolCallEvent(it, n.Item, codexTID), "collabAgentToolCall:" + it.Tool + ":" + it.Status
	}
	return nil, ""
}

// subagentActivityEvent:活动刻度 → 状态卡。kind 未知也照发(值原样带出)。
// raw 是 item 的 json.RawMessage 原文:RawJson 走 redactNativePayload(raw) 保真
// 透传 —— 重编组(mustJSON(it))会丢掉结构体没接住的未知字段并规整字段序,弃用。
func subagentActivityEvent(it subAgentActivityItem, raw json.RawMessage, threadID string) *pb.CodexEvent {
	title := "子代理 · " + it.AgentPath
	if it.AgentPath == "" {
		title = "子代理"
	}
	msg := subAgentKindLabel(it.Kind)
	if it.AgentThreadId != "" {
		msg += " · 线程 " + clip(it.AgentThreadId, 12)
	}
	return &pb.CodexEvent{
		Kind:     pb.CodexEvent_STATUS,
		Code:     "subagent_activity",
		Title:    clip(title, 120),
		Message:  msg,
		ThreadId: threadID,
		RawJson:  redactNativePayload(raw),
	}
}

// collabToolCallEvent:协作工具调用 → 状态卡,正文带 agentsStates 概览。
// raw 同 subagentActivityEvent:item 原文保真透传,未知字段不掉。
func collabToolCallEvent(it collabAgentToolCallItem, raw json.RawMessage, threadID string) *pb.CodexEvent {
	msg := collabToolCallStatusLabel(it.Status)
	_, overview := collabAgentStateOverview(it.AgentsStates)
	if overview != "" {
		msg += " · " + overview
	}
	if p := strings.TrimSpace(it.Prompt); p != "" {
		msg += " · 指派「" + clip(p, 120) + "」"
	}
	return &pb.CodexEvent{
		Kind:     pb.CodexEvent_STATUS,
		Code:     "collab_agent_tool_call",
		Title:    clip("子代理协作 · "+it.Tool, 120),
		Message:  msg,
		ThreadId: threadID,
		RawJson:  redactNativePayload(raw),
	}
}

// subAgentKindLabel:活动刻度的人话。未知值不猜,原样带出。
func subAgentKindLabel(kind string) string {
	switch kind {
	case "started":
		return "子代理已启动"
	case "interacted":
		return "子代理有新活动"
	case "interrupted":
		return "子代理已中断"
	case "":
		return "子代理活动"
	}
	return "子代理活动(kind=" + kind + ")"
}

// collabToolCallStatusLabel:协作工具调用状态的人话。未知值不猜,原样带出。
func collabToolCallStatusLabel(status string) string {
	switch status {
	case "inProgress":
		return "进行中"
	case "completed":
		return "已完成"
	case "failed":
		return "失败"
	case "":
		return "协作工具调用"
	}
	return "协作工具调用(status=" + status + ")"
}

// collabAgentStateOverview 把 agentsStates 汇总成一句概览,如
// "2 个子代理:1 运行 1 完成"。map 为空(nil 或 {})返回 ("", "")。
// 未知状态值按"未知"计数并原样带出 —— 7 态枚举之外的值是数据,不是错误。
func collabAgentStateOverview(states map[string]collabAgentStateItem) (int, string) {
	if len(states) == 0 {
		return 0, ""
	}
	// 概览顺序:进行中的在前,收尾的在后;未知状态兜底在最后。
	order := []struct {
		status string
		label  string
	}{
		{"running", "运行"},
		{"pendingInit", "等待初始化"},
		{"completed", "完成"},
		{"errored", "出错"},
		{"interrupted", "已中断"},
		{"shutdown", "已关闭"},
		{"notFound", "未找到"},
	}
	counts := map[string]int{}
	known := map[string]bool{}
	for _, s := range states {
		counts[s.Status]++
		known[s.Status] = true
	}
	total := len(states)
	var parts []string
	for _, e := range order {
		if counts[e.status] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", counts[e.status], e.label))
		}
		delete(known, e.status)
	}
	// 枚举之外的未知状态:原样透传值,不静默吞。
	for status := range known {
		parts = append(parts, fmt.Sprintf("%d 未知(%s)", counts[status], status))
	}
	return total, fmt.Sprintf("%d 个子代理:%s", total, strings.Join(parts, " "))
}

// subagentNoticeGate 与 native_notifications.go 的 nativeNoticeRecorder 同款限频
// (每键 2 条、最多 64 键,键数也封顶防膨胀)。它是纯闸门,不复播自己的事件 ——
// 播报内容由 subagentEvent 决定,这里只决定"这条还要不要再说一遍"。
type subagentNoticeGate struct {
	mu    sync.Mutex
	count map[string]int
}

func (g *subagentNoticeGate) allow(key string) bool {
	if key == "" {
		return true
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.count == nil {
		g.count = map[string]int{}
	}
	if g.count[key] >= 2 || len(g.count) >= 64 {
		return false
	}
	g.count[key]++
	return true
}
