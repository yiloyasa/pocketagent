// runtime_state.go —— 运行时形态与契约核验的 RPC 面(规格 §10)。
//
// 【它补齐什么】runtime 包早就提供了 VerifyDeclaration(把"缺什么"变成机器可读的
// 报告)与 VerifyRuntime,但**没有任何消费方**:启动时不跑、诊断里看不到。后果是
// "某家 agent 没申报运行时形态""某家申报了却没交代必需入口"这类问题只有翻代码
// 才知道 —— 而契约层的全部意义恰恰是"缺失可见"。
//
// 【为什么只做声明层校验】VerifyRuntime 要先 OpenRuntime,而那可能起子进程、
// 有副作用(见 agent 包 openSharedRuntime 的注释)。诊断与健康探测路径绝不能触发
// 这类动作,所以本文件在启动期只跑零成本的声明层核验。
package server

import (
	"context"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/version"
)

// verifyProviderDeclarations 对注册表里每个 provider 跑声明层核验,按 agent id 排序。
//
// 【为什么要排序】map 遍历顺序随机 —— 不排序会让启动日志与 RPC 返回的次序每次都
// 不同,客户端做前后对比就会看到假变化(与 GetHostInfo 的 caps 排序同款理由)。
func verifyProviderDeclarations(providers map[string]agent.Provider) []runtime.ContractReport {
	ids := make([]string, 0, len(providers))
	for id := range providers {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]runtime.ContractReport, 0, len(ids))
	for _, id := range ids {
		out = append(out, runtime.VerifyDeclaration(id, providers[id]))
	}
	return out
}

// contractReportsSnapshot 返回核验结果快照(启动时生成,之后只读)。
func (s *Server) contractReportsSnapshot() []runtime.ContractReport {
	return append([]runtime.ContractReport(nil), s.contractReports...)
}

// GetRuntimeState 本机运行时总览(含每个 agent 的形态申报核验)。
func (s *Server) GetRuntimeState(_ context.Context, _ *pb.GetRuntimeStateRequest) (*pb.RuntimeStateResponse, error) {
	s.budgetMu.RLock()
	budgetRev := s.budgetPolicy.Revision
	s.budgetMu.RUnlock()
	s.collabMu.RLock()
	collabRev := s.collabPolicy.Revision
	s.collabMu.RUnlock()

	reports := s.contractReportsSnapshot()
	// runtime_state 用两个值:ready / degraded。
	// 【为什么不给更多档】状态词一多就没人说得清"到底哪里不一样";这里只有一个
	// 判据 —— 有没有 agent 的形态申报不合格。
	state := "ready"
	for _, rep := range reports {
		if !rep.OK() {
			state = "degraded"
			break
		}
	}
	resp := &pb.RuntimeStateResponse{
		HostId:               s.hostID,
		DaemonVersion:        version.App,
		BudgetPolicyRevision: budgetRev,
		CollabPolicyRevision: collabRev,
		RuntimeState:         state,
		SampledAtUnixMs:      time.Now().UnixMilli(),
		DataDir:              s.cfg.DataDir,
	}
	// 身份读取失败不阻断总览:身份不可用已在启动恢复报告里如实降级,这里少一个字段
	// 比整条 RPC 失败更有用(客户端仍能看到其余状态)。
	if inst, err := runtime.LoadOrCreateInstallation(s.cfg.DataDir); err == nil && inst != nil {
		resp.InstallationId = inst.InstallationID
	}
	for _, rep := range reports {
		resp.Contracts = append(resp.Contracts, contractReportToProto(rep))
	}
	return resp, nil
}

// GetAgentRuntime 单个 agent 的形态、核验结果与资源用量。
func (s *Server) GetAgentRuntime(_ context.Context, req *pb.GetAgentRuntimeRequest) (*pb.AgentRuntimeResponse, error) {
	agentID := strings.TrimSpace(req.GetAgentId())
	if agentID == "" {
		return nil, errInvalidArg("缺少 agent_id")
	}
	var found *runtime.ContractReport
	for i := range s.contractReports {
		if s.contractReports[i].AgentID == agentID {
			found = &s.contractReports[i]
			break
		}
	}
	if found == nil {
		// 不存在的 agent 必须说"没有它",不回一份空形态 —— 空形态在客户端看起来
		// 和"这家什么都没申报"一模一样,而那是完全不同的两件事。
		return nil, errNotFound("注册表里没有名为 " + agentID + " 的 agent")
	}
	resp := &pb.AgentRuntimeResponse{
		Contract:        contractReportToProto(*found),
		SampledAtUnixMs: time.Now().UnixMilli(),
	}
	if s.runtimeBackend != nil {
		rep := s.runtimeBackend.Resources().Usage(context.Background(), runtime.ScopeAgent(agentID))
		for _, it := range rep.Items {
			resp.Resources = append(resp.Resources, resourceMetricToProto(it))
		}
	}
	return resp, nil
}

// contractReportToProto 把核验报告翻成契约形态(唯一转换点)。
func contractReportToProto(rep runtime.ContractReport) *pb.RuntimeContractReport {
	d := rep.Descriptor
	out := &pb.RuntimeContractReport{
		AgentId:       rep.AgentID,
		DisplayName:   d.DisplayName,
		Version:       d.Version,
		Declared:      rep.Declared,
		SessionModel:  string(d.SessionModel),
		ResumeModel:   string(d.ResumeModel),
		ApprovalModel: string(d.ApprovalModel),
		SubagentModel: string(d.SubagentModel),
		MemoryModel:   string(d.MemoryModel),
		CacheModel:    string(d.CacheModel),
	}
	for _, c := range d.SortedCapabilities() {
		out.Capabilities = append(out.Capabilities, string(c))
	}
	for _, v := range rep.Violations {
		out.Issues = append(out.Issues, &pb.RuntimeContractIssue{Code: v.Code, Detail: v.Detail})
	}
	for _, l := range rep.Limitations {
		out.Limitations = append(out.Limitations, &pb.RuntimeLimitation{Capability: string(l.Capability), Reason: l.Reason})
	}
	return out
}

// logContractReports 把不达标的核验结果打进启动日志。
//
// 【为什么只打不达标的】达标是常态:每个 provider 一条"契约达标"会把启动日志淹掉,
// 而真正需要被看见的是"谁没交代清楚"。
func logContractReports(reports []runtime.ContractReport) {
	for _, rep := range reports {
		if !rep.OK() {
			log.Printf("[contract] %s", rep.Summary())
		}
	}
}
