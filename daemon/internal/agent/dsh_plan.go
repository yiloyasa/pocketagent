package agent

// dsh_plan.go —— dsh 计划变体的计划卡投影(P2,2026-09-25)。
//
// 【此前状态】session/update 的 plan 变体只透传进诊断视图(dsh.go 蓝本 §4.3),
// 手机端看不到执行计划。参照 codex turn/plan/updated 的既有投影
// (codex_events.go:PlanSteps=["<status>|<text>",...],手机端渲染为步骤卡),
// 把 dsh 的计划变体解析成同一形状的 CodexEvent —— 手机端零改动即可复用步骤卡。
//
// 【形状依据:bundle 静态提取,证据分级 (bundle)】dsh 0.1.5-rc.3 随包自带的
// @agentclientprotocol/sdk schema(dsh/node_modules/@agentclientprotocol/sdk/
// dist/schema/types.gen.d.ts)给出三个计划变体:
//
//	sessionUpdate:"plan"(stable):   {entries:[{content, priority, status}]}
//	sessionUpdate:"plan_update"(UNSTABLE,0.1.7-next 方向):
//	    {plan: {type:"items", planId, entries[]} | {type:"file", planId, uri}
//	          | {type:"markdown", planId, content}}
//	sessionUpdate:"plan_removed"(UNSTABLE): 移除通知
//
//  status 枚举 pending|in_progress|completed,priority 枚举 high|medium|low
//  —— 与 codex plan step 的三态同轴,直接进 "<status>|<content>" 格式;
//  priority 在手机步骤卡格式里没有承载位,不发明新格式,如实丢弃。
//
// 【live 边界如实声明】dsh-p0-probe.md 没有 plan 实测帧(探针当时缺 API key),
// 且 0.1.5-rc.3 的 dsh-acp 适配器 bundle 里 sessionUpdate 发射点只有
// agent_message_chunk/agent_thought_chunk/tool_call/tool_call_update/
// usage_update/config_option_update 六种 —— **当前版本不发计划变体**。本投影
// 是给 0.1.7(next,明示 breaking)发 plan_update 时兜底的接线,形状全部来自
// 随包 SDK schema,不做任何字段猜测;认不出的形状一律返回 false 交回调用方
// 原样透传(识别不了≠出错)。

import (
	"encoding/json"
	"fmt"
	"strings"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// dshPlanEntry ACP PlanEntry 的只解析用形状。
type dshPlanEntry struct {
	Content string `json:"content"`
	Status  string `json:"status"`
}

// handleDshPlanUpdate 处理 plan / plan_update / plan_removed 三个变体。
// 返回 true = 已投影成计划卡事件,调用方不必再透传;false = 形状认不出或
// 没有可投影内容,交回调用方按原样透传进诊断视图。
func (p *DshProvider) handleDshPlanUpdate(kind string, raw json.RawMessage, sink Sink) bool {
	switch kind {
	case "plan":
		// stable ACP:整份计划列表(entries 为空=清空计划,没有内容可投影,
		// 按透传处理,不伪造"计划已更新"事件)。
		var u struct {
			Entries []dshPlanEntry `json:"entries"`
		}
		if json.Unmarshal(raw, &u) != nil || len(u.Entries) == 0 {
			return false
		}
		emitDshPlanSteps(u.Entries, sink)
		return true
	case "plan_update":
		// UNSTABLE:plan.content 三选一,只有 items(结构化步骤)能进计划卡;
		// markdown/file 不猜内容形状,交回透传。
		var u struct {
			Plan struct {
				Type    string         `json:"type"`
				Entries []dshPlanEntry `json:"entries"`
				Content string         `json:"content"`
				URI     string         `json:"uri"`
			} `json:"plan"`
		}
		if json.Unmarshal(raw, &u) != nil || u.Plan.Type != "items" || len(u.Plan.Entries) == 0 {
			return false
		}
		emitDshPlanSteps(u.Plan.Entries, sink)
		return true
	case "plan_removed":
		// 移除通知:schema 只有 planId,可命名的事实就是"计划被移除"。
		// 空步骤 = 手机端不渲染步骤卡,事件正文说明事实。
		sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_CodexEvent{CodexEvent: &pb.CodexEvent{
			Kind:    pb.CodexEvent_STATUS,
			Code:    "plan_removed",
			Title:   "执行计划已移除",
			Message: "dsh 移除了本会话的执行计划",
		}}})
		return true
	}
	return false
}

// emitDshPlanSteps 把 ACP 计划条目投影成 codex 同款计划帧。
// (CodexEvent.PlanSteps 的 "status|text" 契约见 proto/v1/thread.proto:697。)
func emitDshPlanSteps(entries []dshPlanEntry, sink Sink) {
	steps := make([]string, 0, len(entries))
	for _, e := range entries {
		text := strings.TrimSpace(e.Content)
		if text == "" {
			continue // 空条目不进卡(与 codex 侧"有正文才投影"的口径一致)
		}
		steps = append(steps, fmt.Sprintf("%s|%s", e.Status, text))
	}
	msg := strings.Join(steps, "；")
	sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_CodexEvent{CodexEvent: &pb.CodexEvent{
		Kind:      pb.CodexEvent_STATUS,
		Code:      "plan_update",
		Title:     "执行计划更新",
		Message:   msg,
		PlanSteps: steps,
	}}})
}
