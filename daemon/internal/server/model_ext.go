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
	"log"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

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
		return out, nil
	}
	cwd := s.cfg.Store.DefaultWorkspace()
	if cwd == "" {
		cwd = s.cfg.Workspace
	}
	out.Items = p.Capabilities(cwd)
	return out, nil
}

// GetCodexSettings 覆盖版:在原有回显之上补齐托管策略与增强后的 MCP 状态。
//
// 【托管策略为什么必须带出来】用户电脑上的 codex 可能被 requirements.toml /
// 管理员策略约束,某些取值是被禁的。手机端如果照旧渲染全部选项,用户就会
// "点了 → codex 拒绝 → App 毫无反馈"。带上白名单后,手机端只渲染允许的取值。
func (s *Server) GetCodexSettings(ctx context.Context, _ *pb.GetCodexSettingsRequest) (*pb.CodexSettings, error) {
	out := &pb.CodexSettings{}
	p := s.codexProvider()
	if p == nil {
		return out, nil
	}
	approval, sandbox := p.DesktopSettings()
	out.ApprovalPolicy = approval
	out.Sandbox = sandbox
	out.Account = p.Account()
	out.Model = s.cfg.Store.DefaultModelID()
	out.Effort = s.cfg.Store.DefaultEffort()
	// 行为与降噪:直接回显 provider 内存态(不读磁盘,不影响桌面端)
	bar, verbosity, summary, burst := p.BehaviorSettings()
	out.HideAgentReasoning = bar
	out.ModelVerbosity = verbosity
	out.ModelReasoningSummary = summary
	out.DisablePasteBurst = burst

	// 托管策略:读一次 configRequirements/read。没有策略时 Managed=false 且
	// 三个白名单为空,手机端据此渲染全部选项 —— 这是绝大多数用户的情况。
	req := p.Requirements()
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
	return out, nil
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
		return out, nil
	}
	servers, err := p.McpStatusDetailed()
	if err != nil {
		// 拉不到就返回空列表(不阻塞设置页)。但记日志,便于排查"为什么列表是空的"。
		log.Printf("[mcp] 读取 MCP 状态失败: %v", err)
		return out, nil
	}
	out.Servers = servers
	return out, nil
}
