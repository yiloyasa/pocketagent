// model_ext.go —— ModelService 的"照镜子"部分:用量、托管策略、能力可见性。
//
// 这三个接口的共同点:它们**只读**。不改 codex 的任何行为,只是把 codex
// 那边本来就有的信息如实搬到手机上。它们回答的正是用户最常问、以前却
// 完全答不上来的三个问题:
//
//	还能不能用?        → GetCodexUsage(限流窗口 + 积分 + 累计用量)
//	为什么选项不见了?   → GetCodexSettings 里的托管策略字段
//	它到底有哪些能力?   → ListCodexCapabilities(技能 / hooks / 插件)
//
// 【一条贯穿的纪律】"读不到"和"没有"必须分开表达。额度读不到时返回 error 字段,
// 而不是一堆 0 —— 把拉取失败显示成 0% 会让用户以为额度用完了,直接放弃使用。
package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// codexProbeBudget 是"探测 codex 可达性"整段(Ready + Account + Requirements)的
// 时间上界。取值 8s:本机 app-server 正常握手是毫秒级,8s 足够;超过就说明
// "无法确认是否生效",如实回报即可,绝不让用户干等。
const codexProbeBudget = 8 * time.Second

// codexSetProbeBudget 是保存路径回读的上界(比探测略宽松,给落盘后的回读留余量)。
const codexSetProbeBudget = 10 * time.Second

// codexProbe 是设置页探测所需的最小能力面。
//
// 【为什么用接口】用具体类型(agent.CodexProvider)时,测试没办法注入
// "永远超时 / 立即报错"的行为;抽成接口后,回归测试可以塞一个假探测,
// 确定性地钉住"探测不可用时保存仍返回成功且值等于落盘值"。
type codexProbe interface {
	// ReadyCtx 探测 app-server 是否可用(带 ctx 上界)。
	ReadyCtx(ctx context.Context) error
	// AccountCtx 从模型列表里取登录账号。
	AccountCtx(ctx context.Context) string
	// RequirementsCtx 读托管策略;第二个返回值为读取错误(读不到 ≠ 没有策略)。
	RequirementsCtx(ctx context.Context) (agent.CodexRequirements, error)
}

// codexProbe 解析出探测对象:优先测试注入,其次真实 provider(没有则 nil)。
func (s *Server) codexProbe() codexProbe {
	if s.probeOverride != nil {
		return s.probeOverride
	}
	if p := s.codexProvider(); p != nil {
		return p
	}
	return nil
}

// GetCodexUsage 读限流窗口与累计用量。
func (s *Server) GetCodexUsage(ctx context.Context, _ *pb.GetCodexUsageRequest) (*pb.CodexUsage, error) {
	p := s.codexProvider()
	if p == nil {
		return &pb.CodexUsage{Error: "电脑上没有可用的 codex"}, nil
	}
	// 这里**故意不用 errInternal** 直接返回错误:额度面板是"锦上添花"的信息,
	// 拉不到应该显示"暂时读不到 + 原因",而不是让整个设置页弹红。
	return p.Usage(), nil
}

// ListCodexCapabilities 列出这台电脑上 codex 实际具备的技能 / hooks / 插件。
//
// cwd 取"用户当前在看哪个工作区"——技能列表是**按目录**算的(项目级 SKILL.md),
// 不传目录会漏掉项目级技能,用户就会觉得"我明明装了怎么不显示"。
func (s *Server) ListCodexCapabilities(ctx context.Context, req *pb.ListCodexCapabilitiesRequest) (*pb.ListCodexCapabilitiesResponse, error) {
	out := &pb.ListCodexCapabilitiesResponse{Items: []*pb.CodexCapability{}}
	p := s.codexProvider()
	if p == nil {
		out.Error = "电脑上没有可用的 codex"
		return out, nil
	}
	cwd := s.cfg.Store.DefaultWorkspace()
	if cwd == "" {
		cwd = s.cfg.Workspace
	}
	var err error
	out.Items, err = p.CapabilitiesDetailed(cwd)
	if err != nil {
		out.Error = err.Error()
		log.Printf("[capabilities] 读取能力失败: %v", err)
	}
	return out, nil
}

// GetCodexSettings 覆盖版:回读当前设置 + **逐项真实状态** + 托管策略。
//
// 【托管策略为什么必须带出来】用户电脑上的 codex 可能被 requirements.toml /
// 管理员策略约束,某些取值是被禁的。手机端如果照旧渲染全部选项,用户就会
// "点了 → codex 拒绝 → App 毫无反馈"。带上白名单后,手机端只渲染允许的取值。
//
// 【为什么以 store 为准】设置已落盘,store 是持久事实来源;provider 内存态在
// daemon 刚启动时是空的(启动时由 hydrateCodexPrefs 从 store 灌一次,见 server.go)。
// 以 store 回显,"重启后仍在"这件事本身就成了可验证的证据。
func (s *Server) GetCodexSettings(ctx context.Context, req *pb.GetCodexSettingsRequest) (*pb.CodexSettings, error) {
	// 设置读取首先回答“daemon 当前保存的值是什么”。它是只读快照，不能因为
	// Codex 初始化锁正忙而把所有值显示成“尚未确认”。保存路径仍会走有界探测，
	// 用于报告本次写入是否已被 Codex 确认。
	if id := req.GetAgentId(); id != "" && id != "codex" {
		return s.nativeSettings(ctx, id, req.GetThreadId()), nil
	}
	return s.codexSettingsSnapshot(ctx, 0), nil
}

// codexSettingsSnapshot 组装设置快照。供 GetCodexSettings 与 SetCodexSettings **共用**,
// 避免两处各写一遍再各自走偏(历史上 SetCodexSettings 直接递归调 GetCodexSettings,
// 把"探测卡死"传染给了保存路径 —— 正是用户抱怨"点了保存永远转圈"的根因)。
//
// 【铁律:store 才是事实来源】无论探测成功与否,快照里的设置值一律来自 store;
// 探测只影响逐项 state 是 applied 还是 unreachable/failed。于是"落盘成功"
// 必然"返回成功" —— 绝不会让"回读失败"变成"保存失败"。
func (s *Server) codexSettingsSnapshot(ctx context.Context, probeBudget time.Duration) *pb.CodexSettings {
	out := &pb.CodexSettings{}
	prefs := s.cfg.Store.CodexPrefs()
	out.Model = s.cfg.Store.DefaultModelID()
	out.Effort = s.cfg.Store.DefaultEffort()
	out.ApprovalPolicy = prefs.ApprovalPolicy
	out.Sandbox = prefs.Sandbox
	out.HideAgentReasoning = prefs.HideReasoning
	out.ModelVerbosity = prefs.Verbosity
	out.ModelReasoningSummary = prefs.ReasoningSummary
	out.DisablePasteBurst = prefs.DisablePasteBurst

	probe := s.codexProbe()
	if probe == nil {
		// 电脑上没有 codex:逐项如实报 failed,而不是假装"没写"。
		out.SettingStatus = buildSettingStatus(out, false, "电脑上没有可用的 codex")
		return out
	}
	if probeBudget <= 0 {
		// 只读设置快照不再触发 Codex 初始化锁;值来自 daemon store，
		// 状态表示 daemon 已接受并会在下一轮 turn/start 应用。
		out.SettingStatus = buildSettingStatusWithReach(out, reachOK, "")
		// 桌面只读回显(config/read 等):GetCodexSettings 的主路径,
		// 电脑端此刻的真实值从这里来(失败时只置 error,不影响上面的状态)。
		s.fillCodexDesktopEcho(ctx, out)
		return out
	}

	// 【第 3 轮:整段探测共用一个短上界】Ready + Account + Requirements 合起来
	// 不能超过 probeBudget;任一步超时,后面的步骤继承同一个(可能已过期的)ctx,
	// 会立即返回而不会各自再等一遍。
	pctx, cancel := context.WithTimeout(ctx, probeBudget)
	defer cancel()

	readyErr := probe.ReadyCtx(pctx)
	reach := classifyReach(readyErr)
	if reach != reachOK {
		// 探测不可达:托管策略也读不到 —— 明确说明,别让用户以为"没有策略限制"。
		out.ManagedNote = "未能读取电脑端托管策略(codex 未响应)"
		out.SettingStatus = buildSettingStatusWithReach(out, reach, reachDetailOf(reach, readyErr, probeBudget))
		return out
	}

	out.Account = probe.AccountCtx(pctx)

	// 托管策略:读一次 configRequirements/read。没有策略时 Managed=false 且
	// 三个白名单为空,手机端据此渲染全部选项 —— 这是绝大多数用户的情况。
	req, reqErr := probe.RequirementsCtx(pctx)
	if reqErr != nil {
		// 读不到 ≠ 没有策略:如实留一句,而不是伪装成"无限制"。
		out.ManagedNote = "未能读取电脑端托管策略(codex 未响应)"
	} else {
		out.Managed = req.Managed
		out.ManagedNote = req.Note
		out.AllowedApprovalPolicies = req.AllowedApprovalPolicies
		out.AllowedSandboxModes = req.AllowedSandboxModes
		out.AllowedWebSearchModes = req.AllowedWebSearchModes
		// 桌面端设置里已经生效的取值如果**不在**白名单里,如实说明:
		// 这不是错误,而是"当前生效值来自电脑上的配置,手机改不动"。
		if req.Managed {
			if len(req.AllowedApprovalPolicies) > 0 && !containsStr(req.AllowedApprovalPolicies, out.ApprovalPolicy) {
				out.ManagedNote = appendNote(out.ManagedNote,
					"当前审批策略 "+out.ApprovalPolicy+" 不在策略白名单内,由电脑端配置决定")
			}
			if len(req.AllowedSandboxModes) > 0 && !containsStr(req.AllowedSandboxModes, out.Sandbox) {
				out.ManagedNote = appendNote(out.ManagedNote,
					"当前沙箱模式 "+out.Sandbox+" 不在策略白名单内,由电脑端配置决定")
			}
		}
	}
	out.SettingStatus = buildSettingStatusWithReach(out, reachOK, "")
	// 保存路径的回读同样带桌面回显:探测已确认 codex 可达,这次读取是毫秒级温读。
	s.fillCodexDesktopEcho(ctx, out)
	return out
}

// codexSettingSpec 描述一条设置项的静态元信息(单一事实来源,新增设置项只加一处)。
//
// key 必须与 CodexSettings / SetCodexSettingsRequest 的字段稳定对应,
// 手机端按 key 查自己的文案表渲染。
type codexSettingSpec struct {
	key       string
	value     func(*pb.CodexSettings) string
	supported bool
	// how 生效方式的人话说明(applied / unsupported 时填进 detail)。
	how string
}

// codexSettingSpecs 是设置项清单。
//
// 【支持性是**实测**结论,不是想当然】见 daemon/cmd/wbprobe:
//   - model_verbosity / model_reasoning_summary 是 codex 有效配置键,
//     可经 thread/start、thread/resume 的 config 覆盖;
//   - hide_agent_reasoning 本版本 codex **没有这个键**(schema 全量 grep 无),
//     改由 daemon 过滤思考事件实现等效行为 → 归为 supported,但说明里写明非原生;
//   - disable_paste_burst 本版本 codex 无任何等效开关 → unsupported,UI 必须禁用。
var codexSettingSpecs = []codexSettingSpec{
	{"approval_policy", func(c *pb.CodexSettings) string { return c.ApprovalPolicy }, true,
		"每轮 turn/start 携带 approvalPolicy"},
	{"sandbox", func(c *pb.CodexSettings) string { return c.Sandbox }, true,
		"每轮 turn/start 携带 sandboxPolicy"},
	{"model", func(c *pb.CodexSettings) string { return c.Model }, true,
		"每轮 turn/start 携带 model"},
	{"effort", func(c *pb.CodexSettings) string { return c.Effort }, true,
		"每轮 turn/start 携带 effort"},
	{"model_reasoning_summary", func(c *pb.CodexSettings) string { return c.ModelReasoningSummary }, true,
		"turn/start 携带 summary + thread config 覆盖"},
	{"model_verbosity", func(c *pb.CodexSettings) string { return c.ModelVerbosity }, true,
		"thread/start、thread/resume 的 config 覆盖"},
	{"hide_agent_reasoning", func(c *pb.CodexSettings) string { return boolStr(c.HideAgentReasoning) }, true,
		"由 daemon 过滤思考事件实现(非 codex 原生开关)"},
	{"disable_paste_burst", func(c *pb.CodexSettings) string { return boolStr(c.DisablePasteBurst) }, false,
		"当前 codex 版本无 disable_paste_burst 这个配置键"},
}

// codexReach 描述"能不能连上 codex"的三态。
type codexReach int

const (
	reachOK          codexReach = iota // 连上了,能判断是否生效
	reachUnreachable                   // 超时 / 正忙:无法确认,可重试
	reachFailed                        // 明确失败(没有 codex、被拒等)
)

// classifyReach 把探测错误分到三态。
//
// 【为什么必须区分 unreachable 与 failed】"正忙 / 超时"是暂时性的、可重试的,
// 不该用"连接失败"那种吓人的语气(用户会以为电脑上的 codex 坏了);真正的失败
// (电脑上没有 codex、被策略拒绝)才是 failed。手机端对两种 state 用不同措辞。
func classifyReach(err error) codexReach {
	switch {
	case err == nil:
		return reachOK
	case errors.Is(err, agent.ErrBusy),
		errors.Is(err, context.DeadlineExceeded),
		errors.Is(err, context.Canceled):
		return reachUnreachable
	default:
		return reachFailed
	}
}

// reachDetailOf 给 unreachable / failed 生成逐项 detail 的人话。
func reachDetailOf(reach codexReach, err error, probeBudget time.Duration) string {
	switch reach {
	case reachUnreachable:
		if errors.Is(err, agent.ErrBusy) {
			return "codex 正忙(另一个操作正在初始化),暂时无法确认是否生效,请稍后重试"
		}
		return fmt.Sprintf("codex 未响应(超过 %ds),无法确认是否生效,稍后可重试", int(probeBudget.Seconds()))
	default:
		return "无法连接 codex:" + errTextOrEmpty(err)
	}
}

// buildSettingStatus 把当前设置 + codex 可达性翻译成逐项状态。
//
// state 取值(applied / saved_not_applied / unsupported / failed / follow_desktop /
// unreachable)与 proto 注释一一对应;手机端对未知 state 兜底显示原文。
//
// 保留这个"bool 可达性"签名,是为了让既有测试(以及 QA 的复核测试)继续可用;
// 真正的分级逻辑在 buildSettingStatusWithReach。
func buildSettingStatus(c *pb.CodexSettings, reachable bool, reachErr string) []*pb.CodexSettingStatus {
	if reachable {
		return buildSettingStatusWithReach(c, reachOK, "")
	}
	return buildSettingStatusWithReach(c, reachFailed, "无法连接 codex:"+reachErr)
}

// buildSettingStatusWithReach 按三态生成逐项状态。
func buildSettingStatusWithReach(c *pb.CodexSettings, reach codexReach, reachDetail string) []*pb.CodexSettingStatus {
	// 值为空 = 跟随桌面(仅字符串项;布尔项永远是真实取值,不会落到这个状态)。
	stringFields := map[string]bool{
		"approval_policy": true, "sandbox": true, "model": true, "effort": true,
		"model_reasoning_summary": true, "model_verbosity": true,
	}
	out := make([]*pb.CodexSettingStatus, 0, len(codexSettingSpecs))
	for _, sp := range codexSettingSpecs {
		st := &pb.CodexSettingStatus{Key: sp.key}
		switch {
		case !sp.supported:
			// 不支持是**静态事实**,与 codex 是否可达无关:置顶判断,先于可达性。
			st.State = "unsupported"
			st.Detail = sp.how
		case reach == reachUnreachable:
			st.State = "unreachable"
			st.Detail = reachDetail
		case reach == reachFailed:
			st.State = "failed"
			st.Detail = reachDetail
		case stringFields[sp.key] && sp.value(c) == "":
			st.State = "follow_desktop"
			st.Detail = "未设置,跟随电脑端配置"
		default:
			st.State = "applied"
			st.Detail = sp.how
		}
		out = append(out, st)
	}
	return out
}

// boolStr 把 bool 转成稳定的 "true"/"false" 字符串(用于逐项状态回显)。
func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// errTextOrEmpty 把可能为 nil 的 error 转成可读文本(空 error 返回空串)。
func errTextOrEmpty(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func containsStr(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

func appendNote(note, extra string) string {
	if note == "" {
		return extra
	}
	return note + "；" + extra
}

// ListMcpServers 升级版:带上 authStatus / serverInfo / 计数 / 错误。
//
// 以前只转发 name/tools/status,结果"某个 MCP 用不了"这件事在手机上完全看不出来
// —— 用户只会觉得"这个工具怎么不好使"。authStatus 直接回答"是不是授权问题"。
func (s *Server) ListMcpServers(ctx context.Context, _ *pb.ListMcpServersRequest) (*pb.ListMcpServersResponse, error) {
	out := &pb.ListMcpServersResponse{}
	p := s.codexProvider()
	if p == nil {
		out.Error = "电脑上没有可用的 codex"
		return out, nil
	}
	servers, err := p.McpStatusDetailed()
	if err != nil {
		// 拉不到仍返回成功响应,但必须显式带错误,避免手机把失败显示成空列表。
		out.Error = err.Error()
		log.Printf("[mcp] 读取 MCP 状态失败: %v", err)
		return out, nil
	}
	out.Servers = servers
	return out, nil
}
