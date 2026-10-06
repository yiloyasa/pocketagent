// codex_events_ext.go —— 补齐 codex 通知面:审查者(guardian)、回合 diff、
// 弃用/配置/沙箱警告、hooks、技能变化、队列、模型安全缓冲。
//
// 【为什么单独一个文件】codex_events.go 里的 switch 已经覆盖了"任务主流程"要用的通知,
// 这里接的是第二梯队:它们不参与逐字流式渲染,但共同回答用户最常见的三个疑问 ——
//
//	"为什么这个动作没执行?"(审查者驳回了)
//	"它到底改了什么?"(回合 diff)
//	"为什么这次不一样了?"(弃用/配置/沙箱/安全缓冲警告)
//
// 以前这些通知全部落在 default 分支被丢掉,用户只能看到"任务莫名失败"。
package agent

import (
	"encoding/json"
	"fmt"
	"strings"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// riskLevelLabel 把 codex 的风险等级翻成人话。
// 直接显示 "medium" 对用户没有意义,他需要知道"这意味着要不要拦一下"。
func RiskLevelLabel(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "low":
		return "低"
	case "medium":
		return "中"
	case "high":
		return "高"
	case "critical":
		return "极高"
	}
	return level
}

// reviewStatusLabel 审查结论的人话标题。
func reviewStatusLabel(status string) string {
	switch status {
	case "inProgress":
		return "审查者正在评估这个动作"
	case "approved":
		return "审查者已放行"
	case "denied":
		return "审查者已驳回"
	case "timedOut":
		return "审查超时"
	case "aborted":
		return "审查已中止"
	case "strictReviewRequired":
		return "需要严格审查"
	}
	return "审查者评估"
}

// codexExtendedNotificationEvent 处理第二梯队通知。
//
// 返回 true 表示已识别并填充了 event;false 表示这条通知我们不需要
// (调用方应当丢弃,而不是伪造一条"收到某事件"的噪音 —— 那会把真正的事件淹掉)。
func codexExtendedNotificationEvent(method string, raw json.RawMessage, event *pb.CodexEvent) bool {
	switch method {
	// ---- 审查者(guardian / auto_review)----
	//
	// 【为什么这是 P1 而不是锦上添花】用户的 approvals_reviewer 一旦不是 user
	// (他自己在桌面上设的),审批卡就**再也不会弹**,而动作可能被自动拒绝。
	// 手机端不展示"谁拒的、为什么拒、风险多高、你可以推翻",用户看到的就是任务莫名失败。
	case "item/autoApprovalReview/started", "item/autoApprovalReview/completed":
		var n struct {
			ReviewId     string `json:"reviewId"`
			TargetItemId string `json:"targetItemId"`
			Review       struct {
				Rationale string `json:"rationale"`
				RiskLevel string `json:"riskLevel"`
				Status    string `json:"status"`
			} `json:"review"`
			Action struct {
				Type    string `json:"type"`
				Command string `json:"command"`
				Program string `json:"program"`
			} `json:"action"`
		}
		_ = json.Unmarshal(raw, &n)
		event.Kind = pb.CodexEvent_GUARDIAN_REVIEW
		event.Title = reviewStatusLabel(n.Review.Status)
		event.Code = n.Review.Status
		event.ReviewId = n.ReviewId
		event.TargetItemId = n.TargetItemId
		event.RiskLevel = n.Review.RiskLevel
		event.ReviewStatus = n.Review.Status
		event.Rationale = n.Review.Rationale

		var parts []string
		if target := firstNonEmpty(n.Action.Command, n.Action.Program); target != "" {
			parts = append(parts, "动作:"+target)
		}
		if n.Review.RiskLevel != "" {
			parts = append(parts, "风险等级:"+RiskLevelLabel(n.Review.RiskLevel))
		}
		if r := strings.TrimSpace(n.Review.Rationale); r != "" {
			parts = append(parts, r)
		}
		if n.Review.Status == "denied" {
			// 只说"被驳回"是不够的:必须告诉用户他还有推翻的权利,
			// 否则他只会以为任务失败了。
			parts = append(parts, "可以在会话页点「仍然执行」推翻这个结论")
		}
		event.Message = strings.Join(parts, "\n")
		return true

	case "autoApprovalReview/strictReviewRequired", "strictReviewRequired":
		var n struct {
			StartedAtMs int64 `json:"startedAtMs"`
		}
		_ = json.Unmarshal(raw, &n)
		event.Kind = pb.CodexEvent_GUARDIAN_REVIEW
		event.Title = reviewStatusLabel("strictReviewRequired")
		event.ReviewStatus = "strictReviewRequired"
		event.Message = "接下来的动作会逐个送审后才执行,速度会变慢。"
		return true

	case "guardianWarning":
		var n struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(raw, &n)
		event.Kind = pb.CodexEvent_GUARDIAN_REVIEW
		event.Title = "审查者提醒"
		event.Message = n.Message
		return true

	// ---- 回合 diff ----
	case "turn/diff/updated":
		var n struct {
			Diff string `json:"diff"`
		}
		_ = json.Unmarshal(raw, &n)
		if strings.TrimSpace(n.Diff) == "" {
			return false
		}
		event.Kind = pb.CodexEvent_TURN_DIFF
		event.Title = "本回合文件改动"
		event.Diff = n.Diff
		event.Message = "本回合累计改动的文件"
		return true

	// ---- codex 自己的警告面 ----
	case "deprecationNotice":
		var n struct {
			Summary string  `json:"summary"`
			Details *string `json:"details"`
		}
		_ = json.Unmarshal(raw, &n)
		event.Kind = pb.CodexEvent_DEPRECATION
		event.Title = "有一项能力即将弃用"
		event.Message = joinParts(n.Summary, derefTrim(n.Details))
		return true

	case "configWarning":
		var n struct {
			Summary string  `json:"summary"`
			Details *string `json:"details"`
			Path    *string `json:"path"`
		}
		_ = json.Unmarshal(raw, &n)
		event.Kind = pb.CodexEvent_CONFIG_WARNING
		event.Title = "配置有问题"
		event.Message = joinParts(n.Summary, derefTrim(n.Details), derefTrim(n.Path))
		return true

	case "windows/worldWritableWarning":
		var n struct {
			SamplePaths []string `json:"samplePaths"`
			ExtraCount  int      `json:"extraCount"`
			FailedScan  bool     `json:"failedScan"`
		}
		_ = json.Unmarshal(raw, &n)
		event.Kind = pb.CodexEvent_SANDBOX_WARNING
		event.Title = "沙箱目录权限过宽"
		if n.FailedScan {
			event.Message = "扫描目录权限时失败,无法确认沙箱是否安全。"
			return true
		}
		msg := "以下目录对所有用户可写,沙箱保护会打折扣:" + strings.Join(n.SamplePaths, "、")
		if n.ExtraCount > 0 {
			msg += fmt.Sprintf("(另有 %d 个)", n.ExtraCount)
		}
		event.Message = msg
		return true

	// ---- hooks 与技能变化 ----
	case "hook/started", "hook/completed":
		var n struct {
			Run struct {
				EventName     string  `json:"eventName"`
				Status        string  `json:"status"`
				StatusMessage *string `json:"statusMessage"`
				SourcePath    string  `json:"sourcePath"`
			} `json:"run"`
		}
		_ = json.Unmarshal(raw, &n)
		event.Kind = pb.CodexEvent_HOOK
		event.Title = "Hook " + n.Run.EventName
		event.Code = n.Run.Status
		parts := []string{}
		if n.Run.Status != "" {
			parts = append(parts, "状态:"+n.Run.Status)
		}
		if s := strings.TrimSpace(n.Run.SourcePath); s != "" {
			parts = append(parts, s)
		}
		event.Message = joinParts(derefTrim(n.Run.StatusMessage), strings.Join(parts, " · "))
		return true

	case "skills/changed":
		event.Kind = pb.CodexEvent_SKILLS_CHANGED
		event.Title = "技能列表已变化"
		event.Message = "电脑上的技能有增删或开关变化,重新打开「能力」可以看到最新列表。"
		return true

	// ---- 队列 ----
	case "thread/queue/changed":
		event.Kind = pb.CodexEvent_QUEUE
		event.Title = "排队状态变化"
		event.Message = "会话的处理队列有变化(连续发消息时,后面的任务在排队)。"
		return true

	// ---- 模型安全缓冲 ----
	case "model/safetyBuffering/updated":
		var n struct {
			Model           string   `json:"model"`
			FasterModel     *string  `json:"fasterModel"`
			Reasons         []string `json:"reasons"`
			UseCases        []string `json:"useCases"`
			ShowBufferingUi bool     `json:"showBufferingUi"`
		}
		_ = json.Unmarshal(raw, &n)
		event.Kind = pb.CodexEvent_MODEL_SAFETY
		event.Title = "模型正在安全缓冲"
		parts := []string{fmt.Sprintf("模型 %s 正在缓冲处理", n.Model)}
		if len(n.Reasons) > 0 {
			parts = append(parts, "原因:"+strings.Join(n.Reasons, "、"))
		}
		if n.FasterModel != nil && strings.TrimSpace(*n.FasterModel) != "" {
			parts = append(parts, "可换用更快模型:"+strings.TrimSpace(*n.FasterModel))
		}
		event.Message = strings.Join(parts, " · ")
		return true

	case "model/verification":
		var n struct {
			Verifications []json.RawMessage `json:"verifications"`
		}
		_ = json.Unmarshal(raw, &n)
		if len(n.Verifications) == 0 {
			return false
		}
		event.Kind = pb.CodexEvent_MODEL_SAFETY
		event.Title = "模型输出校验"
		event.Message = fmt.Sprintf("本轮有 %d 项校验记录。", len(n.Verifications))
		return true

	// ---- 模型服务登录态自动恢复(0.159.2 起,2026-10-07 审计升为投影)----
	// 载荷 {message, provider, threadId, turnId}。它解释"轮次莫名失败":
	// 上游登录态掉了 codex 在自动恢复,恢复不成随后必有 error 通知。此前只
	// 留痕不投影,用户看不到这层原因 —— 错误维度不许静默,投影成 WARNING 卡。
	case "modelProvider/authRecoveryStarted", "modelProvider/authRecoveryCompleted":
		var n struct {
			Message  string `json:"message"`
			Provider string `json:"provider"`
		}
		_ = json.Unmarshal(raw, &n)
		event.Kind = pb.CodexEvent_WARNING
		if strings.HasSuffix(method, "Completed") {
			event.Title = "模型服务登录状态已恢复"
			event.Code = "authRecoveryCompleted"
		} else {
			event.Title = "模型服务登录状态恢复中"
			event.Code = "authRecoveryStarted"
		}
		parts := make([]string, 0, 2)
		if n.Provider != "" {
			parts = append(parts, "服务:"+n.Provider)
		}
		if m := strings.TrimSpace(n.Message); m != "" {
			parts = append(parts, m)
		}
		event.Message = strings.Join(parts, " · ")
		return true
	}
	return false
}

// codexThreadStatusLabel 把 thread/status/changed 的状态枚举(ThreadStatus
// oneOf:notLoaded/idle/systemError/active)翻成人话;枚举外原样返回,不猜。
func codexThreadStatusLabel(t string) string {
	switch t {
	case "notLoaded":
		return "未加载"
	case "idle":
		return "空闲"
	case "active":
		return "执行中"
	case "systemError":
		return "系统异常"
	}
	return t
}

// codexThreadFlagsLabel 把 active 态的 activeFlags(waitingOnApproval/
// waitingOnUserInput)翻成人话;未知值原样保留 —— 它们是数据,不是噪音。
func codexThreadFlagsLabel(flags []string) string {
	labels := make([]string, 0, len(flags))
	for _, f := range flags {
		switch f {
		case "waitingOnApproval":
			labels = append(labels, "等待审批")
		case "waitingOnUserInput":
			labels = append(labels, "等待输入")
		default:
			labels = append(labels, f)
		}
	}
	return strings.Join(labels, "、")
}

// codexRateLimitReachedLabel 把 RateLimitSnapshot.rateLimitReachedType 翻成人话;
// 空值返回空串(卡片不追加该段)。
func codexRateLimitReachedLabel(t string) string {
	switch t {
	case "":
		return ""
	case "rate_limit_reached":
		return "已触发限流,请等窗口重置"
	case "workspace_owner_credits_depleted":
		return "工作区额度已用尽"
	case "workspace_member_credits_depleted":
		return "工作区成员额度已用尽"
	case "workspace_owner_usage_limit_reached":
		return "工作区所有者的用量上限已到"
	case "workspace_member_usage_limit_reached":
		return "工作区成员的用量上限已到"
	}
	return "限流原因:" + t
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

// derefTrim 安全解引用并去空白(codex 的说明字段大量是 nullable)。
func derefTrim(p *string) string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(*p)
}

// joinParts 只拼接非空片段,用 " · " 连接。
func joinParts(parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return strings.Join(out, " · ")
}
