// codex_guardian.go —— 审查者(codex 的 guardian / approval auto-review)接入。
//
// 【它解决什么问题】codex 的审查者一旦不是"让用户自己决定"(approvals_reviewer
// 被桌面端设成 auto_review,或管理员策略开了 autoReview),审批卡就**再也不会弹**:
// 动作被审查者自己判了。手机端如果只显示"任务失败",用户根本不知道发生了什么,
// 更不知道他其实有权推翻结论。这个文件负责三件事:
//
//  1. 把 codex 发来的审查结论记下来(常驻钩子,任务之外也收得到);
//  2. 把结论翻成人话发给手机(与 codex_events_ext.go 配合);
//  3. 用户点"仍然执行"时,把结论**原样**还原成 codex 要的
//     GuardianAssessmentEvent 再发回去。
//
// 【关于第 3 步:为什么不能自己"编"一个事件】
// thread/approveGuardianDeniedAction 的参数 event 在协议里是**无类型透传**
// (JSON Schema 里是 JsonValue,只有一句"Serialized GuardianAssessmentEvent"),
// 也就是说结构没写在协议里。我们对着本机 codex 0.149.1 实测(逐字段试探 serde 报错)
// 得到的真实形状是:
//
//	{ "id": <reviewId>, "status": <snake_case>, "action": <GuardianApprovalReviewAction> }
//
// 而**通知侧**给的是 camelCase:
//
//	status:  inProgress / approved / denied / timedOut / aborted
//	action.type: command / execve / writeStdin / applyPatch / networkAccess / mcpToolCall / requestPermissions
//	             (writeStdin 为 0.160.0 新增的第七种:向运行中的终端写入输入;
//	             见 vendored schema GuardianApprovalReviewAction oneOf[2])
//	action.source: shell / unifiedExec
//	action.protocol: http / https / socks5Tcp / socks5Udp
//	action.toolName
//
// 两边**大小写不一样**(实测:直接回填通知原文,status=timedOut 与
// action.type=applyPatch 都会被拒)。所以下面做的是"忠实转写",不是"编造" ——
// 字段值全部来自 codex 自己发来的结论,我们只改大小写与包裹层级。
//
// 【失败时的态度】codex 拒绝这个事件时,把它的话原样带回手机上,绝不假装成功。
// 我们宁可让用户看到"电脑拒绝了这次推翻",也不要造一个"已执行"的假象。
package agent

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/pocketagent/daemon/internal/codexapp"
)

// GuardianReview 是 codex 的一次审查结论(记录用)。
type GuardianReview struct {
	ReviewID     string
	ThreadID     string // codex 线程 id
	TurnID       string
	TargetItemID string
	// Status 原样保存通知里的 camelCase 取值(inProgress/approved/denied/timedOut/aborted)。
	// 发回 codex 时才转成 snake_case —— 见 guardianAssessmentEvent。
	Status    string
	RiskLevel string
	Rationale string
	// ActionRaw 是通知里 action 对象的原始 JSON(camelCase),回填时逐字段转写。
	ActionRaw json.RawMessage
	// ActionLabel 是给人看的一句话(命令 / 程序 / 文件 / 域名)。
	ActionLabel   string
	StartedAtMs   int64
	CompletedAtMs int64
	// OverriddenByUser 为 true 表示用户点过"仍然执行"(手机端据此收起按钮)。
	OverriddenByUser bool
	RecordedAtMs     int64
}

// IsActionable 表示"这个结论值得给用户一个推翻入口"。
//
// 只有 denied / timedOut / aborted 才有意义:
//   - approved:已经放行了,没什么可推翻的;
//   - inProgress:还没结论,按钮点了也没用(而且 codex 不接受这个中间态的业务语义)。
func (g *GuardianReview) IsActionable() bool {
	switch g.Status {
	case "denied", "timedOut", "aborted":
		return true
	}
	return false
}

// recordGuardianNotification 记录一条审查通知(常驻钩子调用,必须廉价且并发安全)。
func (p *CodexProvider) recordGuardianNotification(method string, params json.RawMessage) {
	// guardianWarning / strictReviewRequired 不带审查结论对象,只是提醒,不参与记录。
	if method == "guardianWarning" || strings.Contains(method, "strictReviewRequired") {
		return
	}
	var n struct {
		ThreadId      string  `json:"threadId"`
		TurnId        string  `json:"turnId"`
		ReviewId      string  `json:"reviewId"`
		TargetItemId  *string `json:"targetItemId"`
		StartedAtMs   int64   `json:"startedAtMs"`
		CompletedAtMs int64   `json:"completedAtMs"`
		Review        struct {
			Status    string `json:"status"`
			RiskLevel string `json:"riskLevel"`
			Rationale string `json:"rationale"`
		} `json:"review"`
		Action json.RawMessage `json:"action"`
	}
	if err := json.Unmarshal(params, &n); err != nil || n.ReviewId == "" {
		return
	}
	rec := &GuardianReview{
		ReviewID:      n.ReviewId,
		ThreadID:      n.ThreadId,
		TurnID:        n.TurnId,
		TargetItemID:  derefTrim(n.TargetItemId),
		Status:        n.Review.Status,
		RiskLevel:     n.Review.RiskLevel,
		Rationale:     strings.TrimSpace(n.Review.Rationale),
		ActionRaw:     n.Action,
		ActionLabel:   guardianActionLabel(n.Action),
		StartedAtMs:   n.StartedAtMs,
		CompletedAtMs: n.CompletedAtMs,
		RecordedAtMs:  time.Now().UnixMilli(),
	}

	p.gMu.Lock()
	defer p.gMu.Unlock()
	if p.guardianReviews == nil {
		p.guardianReviews = map[string]*GuardianReview{}
	}
	if p.guardianLatest == nil {
		p.guardianLatest = map[string]string{}
	}
	if prev, ok := p.guardianReviews[rec.ReviewID]; ok {
		// started → completed 是同一条审查的两次通知:保留 startedAtMs,补全结论。
		rec.StartedAtMs = firstNonZero(rec.StartedAtMs, prev.StartedAtMs)
		rec.OverriddenByUser = prev.OverriddenByUser
	} else {
		p.guardianOrder = append(p.guardianOrder, rec.ReviewID)
	}
	p.guardianReviews[rec.ReviewID] = rec
	if rec.IsActionable() && !rec.OverriddenByUser && rec.ThreadID != "" {
		p.guardianLatest[rec.ThreadID] = rec.ReviewID
	} else if p.guardianLatest[rec.ThreadID] == rec.ReviewID {
		// 结论变成"已放行"或被用户推翻过:撤掉入口。
		delete(p.guardianLatest, rec.ThreadID)
	}
	// 有界淘汰:审查是高频事件,不设上限会一直涨。
	for len(p.guardianOrder) > guardianKeep {
		old := p.guardianOrder[0]
		p.guardianOrder = p.guardianOrder[1:]
		if r, ok := p.guardianReviews[old]; ok && p.guardianLatest[r.ThreadID] == old {
			delete(p.guardianLatest, r.ThreadID)
		}
		delete(p.guardianReviews, old)
	}
}

// GuardianLatest 返回某线程上最近一条"可推翻"的审查结论(供 server 层填进 Thread)。
func (p *CodexProvider) GuardianLatest(threadID string) *GuardianReview {
	p.gMu.Lock()
	defer p.gMu.Unlock()
	id, ok := p.guardianLatest[threadID]
	if !ok {
		return nil
	}
	return p.guardianReviews[id]
}

// ApproveGuardianDenied 推翻审查者的驳回结论,要求继续执行。
//
// reviewID 为空时用该线程最近一条可推翻的结论 —— 手机端按钮上本来就只显示
// "仍然执行"四个字,不该逼前端先回传一个它并不理解的 id。
func (p *CodexProvider) ApproveGuardianDenied(threadID, reviewID, targetItemID string) error {
	if strings.TrimSpace(threadID) == "" {
		return fmt.Errorf("缺少 codex 线程 id")
	}
	p.gMu.Lock()
	if reviewID == "" {
		reviewID = p.guardianLatest[threadID]
	}
	rec := p.guardianReviews[reviewID]
	p.gMu.Unlock()

	if rec == nil {
		// 说清楚"为什么不行 + 怎么办"。daemon 重启会清空记录,这是最可能的成因。
		return fmt.Errorf("找不到这次审查的原始结论(可能来自 daemon 重启之前)。请回到会话让 agent 重新发起该动作,再决定是否放行")
	}
	if rec.ThreadID != "" && rec.ThreadID != threadID {
		return fmt.Errorf("这条审查结论不属于当前会话")
	}
	if !rec.IsActionable() {
		return fmt.Errorf("这条审查的结论是「%s」,没有可推翻的驳回", reviewStatusLabel(rec.Status))
	}
	if targetItemID != "" && rec.TargetItemID != "" && targetItemID != rec.TargetItemID {
		// 手机端的 item id 可能来自被重新渲染过的列表:以我们记录下来的为准,只记日志。
		log.Printf("[guardian] target item 不一致(以记录为准): 请求=%s 记录=%s", targetItemID, rec.TargetItemID)
	}

	event, err := guardianAssessmentEvent(rec)
	if err != nil {
		return err
	}

	// 用当前子进程发(会按需拉起/重启),避免用到一个已经死掉的 client。
	err = p.withClient(func(c *codexapp.Client) error {
		_, err := c.Call("thread/approveGuardianDeniedAction", map[string]interface{}{
			"threadId": threadID,
			"event":    event,
		})
		return err
	})
	if err != nil {
		// 原样带回 codex 的拒绝理由:绝不假装成功。
		return fmt.Errorf("电脑拒绝了这次推翻: %w", err)
	}

	p.gMu.Lock()
	rec.OverriddenByUser = true
	if p.guardianLatest[threadID] == rec.ReviewID {
		delete(p.guardianLatest, threadID)
	}
	p.gMu.Unlock()
	log.Printf("[guardian] 用户推翻审查结论 review=%s thread=%s(已下发)", rec.ReviewID, threadID)
	return nil
}

// ---- 通知 → 事件 的转写 ----

// guardianStatusToWire 通知侧 camelCase → 事件侧 snake_case(实测取值域)。
func guardianStatusToWire(status string) string {
	switch status {
	case "inProgress":
		return "in_progress"
	case "timedOut":
		return "timed_out"
	}
	// approved / denied / aborted 两边同名
	return status
}

// guardianActionTypeToWire 通知侧 camelCase → 事件侧 snake_case(实测取值域)。
func guardianActionTypeToWire(t string) string {
	switch t {
	case "applyPatch":
		return "apply_patch"
	case "networkAccess":
		return "network_access"
	case "mcpToolCall":
		return "mcp_tool_call"
	case "requestPermissions":
		return "request_permissions"
	}
	return t // command / execve 两边同名
}

// guardianSourceToWire:通知里是 unifiedExec,事件里要 unified_exec。
func guardianSourceToWire(s string) string {
	if s == "unifiedExec" {
		return "unified_exec"
	}
	return s
}

// guardianProtocolToWire:通知里是 socks5Tcp/socks5Udp,事件里是 socks5_tcp/socks5_udp。
func guardianProtocolToWire(s string) string {
	switch s {
	case "socks5Tcp":
		return "socks5_tcp"
	case "socks5Udp":
		return "socks5_udp"
	}
	return s
}

// guardianAssessmentEvent 把一条记录还原成 codex 要的 GuardianAssessmentEvent。
//
// 【实现取向:**逐字段转写**,而不是挑字段重建】。理由:协议把 event 定义成
// 无类型透传,说明它的结构随时可能变(官方注释就写着 "UNSTABLE ... expected to change
// soon")。如果这里白名单式地重建,codex 以后新增一个必填字段,我们的按钮就会
// 突然开始报"missing field" —— 而透传能自动跟上。所以只做必须做的转换:
// 大小写与嵌套位置的调整,其余原样搬运。
func guardianAssessmentEvent(rec *GuardianReview) (map[string]interface{}, error) {
	action, err := guardianActionToWire(rec.ActionRaw)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"id":     rec.ReviewID,
		"status": guardianStatusToWire(rec.Status),
		"action": action,
	}, nil
}

// guardianActionToWire 把通知里的 action 对象转写成事件侧写法。
func guardianActionToWire(raw json.RawMessage) (map[string]interface{}, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("这次审查没有记录被审查的动作,无法还原成电脑要求的格式(请重新发起该动作)")
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("被审查的动作无法解析: %w", err)
	}
	t, _ := m["type"].(string)
	if t == "" {
		return nil, fmt.Errorf("被审查的动作缺少类型字段,无法还原成电脑要求的格式")
	}
	// 已知的六种只改大小写;**未知类型原样透传**。
	// 理由:protocol 把 event 定义成无类型透传,说明结构随时会变。若某个新动作
	// 本来就是 snake_case,透传能立刻可用;真需要转换时,codex 会用 serde 报错
	// ("unknown variant ... , expected one of ...") 把话说明白,我们原样带给用户。
	m["type"] = guardianActionTypeToWire(t)

	if s, ok := m["source"].(string); ok {
		m["source"] = guardianSourceToWire(s)
	}
	if s, ok := m["protocol"].(string); ok {
		m["protocol"] = guardianProtocolToWire(s)
	}
	// mcpToolCall 的内层字段也是 snake_case(实测 toolName → tool_name)。
	if v, ok := m["toolName"]; ok {
		m["tool_name"] = v
		delete(m, "toolName")
	}
	return m, nil
}

// guardianActionLabel 生成"被审查的是什么"这一句话(手机端展示 + 日志)。
func guardianActionLabel(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var a struct {
		Type      string   `json:"type"`
		Command   string   `json:"command"`
		Program   string   `json:"program"`
		Argv      []string `json:"argv"`
		Files     []string `json:"files"`
		Host      string   `json:"host"`
		Port      int      `json:"port"`
		Protocol  string   `json:"protocol"`
		Server    string   `json:"server"`
		ToolName  string   `json:"toolName"`
		ToolTitle string   `json:"toolTitle"`
		Stdin     string   `json:"stdin"`
	}
	if json.Unmarshal(raw, &a) != nil {
		return ""
	}
	switch a.Type {
	case "command":
		return "命令 " + clip(a.Command, 160)
	case "execve":
		return "程序 " + clip(firstNonEmpty(a.Program, strings.Join(a.Argv, " ")), 160)
	case "writeStdin":
		// 0.160.0 新增动作变体(schema GuardianApprovalReviewAction oneOf[2],
		// 字段 approvalId/processId/stdin):审查者评估"要不要往运行中的终端
		// 写这行输入"。此前没有这个 case,该类审查卡上"动作"一句是空的 ——
		// 用户只看到结论,不知道被审的是什么。
		return "写入输入 " + clip(a.Stdin, 120)
	case "applyPatch":
		if len(a.Files) == 0 {
			return "文件改动"
		}
		return "文件改动 " + clip(strings.Join(a.Files, "、"), 200)
	case "networkAccess":
		return fmt.Sprintf("联网 %s://%s:%d", a.Protocol, a.Host, a.Port)
	case "mcpToolCall":
		return "MCP 工具 " + firstNonEmpty(a.ToolTitle, a.ToolName) + "(" + a.Server + ")"
	case "requestPermissions":
		return "申请额外权限"
	}
	return ""
}

func firstNonZero(values ...int64) int64 {
	for _, v := range values {
		if v != 0 {
			return v
		}
	}
	return 0
}
