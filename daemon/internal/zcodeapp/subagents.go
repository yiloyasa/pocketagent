package zcodeapp

// subagents.go —— session/subagents RPC(0.16.9 新增)的薄封装。
//
// 【为什么需要】ZCode 的子代理是 "Agent"/"Task" 工具,子代理内部对话不进
// 父会话事件流,此前手机端只能靠 tool.updated 的入参/结果"结构推测"(见
// subagent.go 的识别依据)。0.16.9 起 app-server 提供 session/subagents:
// 按父会话给出**原生子代理清单**(运行中 + 已结束),手机端子代理卡的数据
// 由此升级为原生事实。
//
// 【形状:三重实证,不要凭印象改】
//  1. bundle 静态提取(zcode.cjs zod schema,strict):参数 iGt =
//     {sessionId, endedCursor?, endedLimit? (1..100, 默认 20)};应答 W5i =
//     {revision, childSessionIds[], running: Lsr[], ended:{total, items: Fsr[],
//     nextCursor?}};条目 Nsr = {childSessionId, agentId?, toolCallId?,
//     subagentType, title, summary?, startedAt?, endedAt?(仅 ended)},
//     running 条目 status ∈ running|waiting|blocked,ended ∈ success|failed|
//     cancelled|lost。
//  2. live 空态实测(2026-09-25,zc-subagents-probe.log):应答键与 schema 逐字
//     一致;未知键报 -32602 "Unrecognized key"(strict 成立);缺 sessionId 报
//     -32602 "expected string, received undefined"。
//  3. live 时序实测:session/create 应答到达后立刻查询会 -32004 "Session not
//     found" —— 本 RPC 只认 sessionStore 里已持久化的会话。调用纪律:只在
//     轮次(或其他确认落库的动作)之后查,不要在 create 后立刻查。
//
// 【分页边界(2026-10-07 收口,比照 dshapp SessionList 同款先例)】ended 用
// (endedAt, childSessionId) 游标分页:调用方给的 endedCursor 是首页起点;
// 应答 ended.nextCursor 非空 = 还有下一页,本封装自动跟随翻页取全,上限
// zcodeSubagentsMaxPages 页(防御性:防游标异常死循环),触顶返回已取得的页
// 并落日志,不以错误打断调用方 —— 清单是"有则更好"的增强数据,少页只影响
// 完整性判断。旧实现只取一页、nextCursor 被静默丢弃,单页(50)装不下的
// ended 子代理对用户不可见。快照级字段(revision/childSessionIds/running)
// 描述同一父会话的当前事实,只取首页,不跨页累计。
//
// 【透传原则】未知状态值原样保留在 Status 里,不映射不猜测(server 层落库时
// 只对认识的状态表态,见 zcode_subagent_sync.go)。

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
)

// SubagentEntry session/subagents 里一条子代理条目(running/ended 通用:ended
// 多一个 endedAt 与终态 status,running 的是进行态 status)。字段全集来自
// bundle schema,`有则给、没有即零值`,零值不算错误。
type SubagentEntry struct {
	ChildSessionID string `json:"childSessionId"`
	AgentID        string `json:"agentId,omitempty"`
	ToolCallID     string `json:"toolCallId,omitempty"`
	SubagentType   string `json:"subagentType"`
	Title          string `json:"title"`
	Summary        string `json:"summary,omitempty"`
	// StartedAt/EndedAt unix 毫秒;缺失为 0(不猜当前时间)。
	StartedAt int64 `json:"startedAt,omitempty"`
	EndedAt   int64 `json:"endedAt,omitempty"`
	// Status 原样透传:running 页 ∈ running|waiting|blocked,
	// ended 页 ∈ success|failed|cancelled|lost;0.16.9 之外的新值不猜。
	Status string `json:"status"`
}

// SubagentsEndedPage ended 分页的一页。
type SubagentsEndedPage struct {
	Total int             `json:"total"`
	Items []SubagentEntry `json:"items"`
	// NextCursor 非空 = 还有下一页(游标原样回传即可)。
	NextCursor string `json:"nextCursor,omitempty"`
}

// SubagentsResult session/subagents 应答的保真形状。
type SubagentsResult struct {
	Revision        int64              `json:"revision"`
	ChildSessionIDs []string           `json:"childSessionIds"`
	Running         []SubagentEntry    `json:"running"`
	Ended           SubagentsEndedPage `json:"ended"`
}

// Subagents 调 session/subagents 取某会话的原生子代理清单(自动跟随 ended
// 分页游标取全,见文件头【分页边界】)。
//
// sessionID 必须是已持久化的 zcode 会话 id(见文件头时序纪律);endedLimit
// <= 0 时省略(服务器默认 20),> 100 会被 strict schema 拒绝,这里不再重复
// 校验 —— 参数错误让它以 -32602 原样暴露,不静默钳制。endedCursor 非空时作为
// 首页起点原样回传(b64url 游标,构造与解析都是服务器的事);后续页的游标取自
// 各页应答的 ended.nextCursor,原样回传。
func (c *Client) Subagents(ctx context.Context, sessionID string, endedLimit int, endedCursor string) (SubagentsResult, error) {
	var out SubagentsResult
	if sessionID == "" {
		return out, fmt.Errorf("session/subagents: sessionId 必填")
	}
	cursor := endedCursor
	for page := 0; page < zcodeSubagentsMaxPages; page++ {
		params := map[string]interface{}{"sessionId": sessionID}
		if endedLimit > 0 {
			params["endedLimit"] = endedLimit
		}
		if cursor != "" {
			params["endedCursor"] = cursor // 服务器产出的不透明游标,原样回传
		}
		raw, err := c.CallCtx(ctx, "session/subagents", params)
		if err != nil {
			return out, err
		}
		var pg SubagentsResult
		if err := json.Unmarshal(raw, &pg); err != nil {
			return out, fmt.Errorf("session/subagents 应答解析失败: %w", err)
		}
		if page == 0 {
			out = pg // 快照级字段(revision/childSessionIds/running)与首页 ended
		} else {
			out.Ended.Items = append(out.Ended.Items, pg.Ended.Items...)
		}
		// NextCursor 保留末页原值:游标耗尽时为空串(取全);触顶时非空,
		// 调用方据此知道"还有更多没取"。
		out.Ended.NextCursor = pg.Ended.NextCursor
		if pg.Ended.NextCursor == "" {
			return out, nil
		}
		cursor = pg.Ended.NextCursor
	}
	log.Printf("[zcode] session/subagents 达到翻页上限(%d 页),返回已取得的 %d 条", zcodeSubagentsMaxPages, len(out.Ended.Items))
	return out, nil
}

// zcodeSubagentsMaxPages 是 Subagents 跟随 ended.nextCursor 的页数上限
// (防御性:页大小至多 100、常用 50,50 页 = 数千条,远超真实子代理规模;
// 防游标异常导致死循环,与 dshapp 的 dshSessionListMaxPages 同口径)。
const zcodeSubagentsMaxPages = 50
