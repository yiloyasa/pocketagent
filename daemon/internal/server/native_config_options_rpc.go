// native_config_options_rpc.go —— ListAgentConfigOptions:各家 agent 原生设置目录
// 的透出口(原生设置透出轮,2026-09-25)。
//
// 【挂在 ModelService 的理由】GetCodexSettings/SetCodexSettings(agent 设置面)已在此,
// 原生设置目录是同一语义面的扩展;写通道同样复用 SetCodexSettings 的
// native_config_option_id/value 字段(与 zcode native_mode 走 SetCodexSettings 同款
// 先例),RPC 总数只 +1(82→83)。
//
// 【agent_id 校验在注册表上,文案动态列举】requireRegisteredAgent 同款纪律:
// 注册表是 Config.Providers 的键集,错误文案把"已注册的有谁"一并报给客户端。
//
// 【透传纪律】provider 层(NativeConfigOptions)已保证 id/值/候选全部原生原文;
// 本文件只做注册表校验、会话归属校验与 pb 映射,不加工任何业务字段。
package server

import (
	"context"
	"strings"
	"time"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// ListAgentConfigOptions 列出某 agent 的原生设置目录。
func (s *Server) ListAgentConfigOptions(ctx context.Context, req *pb.ListAgentConfigOptionsRequest) (*pb.ListAgentConfigOptionsResponse, error) {
	agentID := strings.TrimSpace(req.GetAgentId())
	if agentID == "" {
		return nil, errInvalidArg("缺少 agent_id")
	}
	if err := s.requireRegisteredAgent(agentID); err != nil {
		return nil, err
	}
	resp := &pb.ListAgentConfigOptionsResponse{SampledAtUnixMs: time.Now().UnixMilli()}

	provider, ok := s.cfg.Providers[agentID]
	if !ok {
		return nil, errFailedPrecondition("Agent 不可用: " + agentID)
	}
	src, ok := provider.(agent.NativeConfigOptions)
	if !ok {
		// 该 agent 没有"目录型"原生设置(codex/zcode 各有专属设置面):如实回空,
		// 不伪造空目录之外的语义。
		return resp, nil
	}

	// 线程归属校验:给了 thread_id 就必须是该 agent 名下的会话 —— 否则"以那个
	// 会话的实时目录为准"会读到别人家的状态。
	var thread *store.Thread
	if tid := strings.TrimSpace(req.GetThreadId()); tid != "" {
		t, ok := s.cfg.Store.Get(tid)
		if !ok || threadAgentID(t) != agentID {
			return nil, errInvalidArg("会话不属于所选 Agent")
		}
		thread = t
	}

	rows, err := src.ConfigOptionsCatalog(ctx, thread)
	if err != nil {
		// 读取失败不炸整条 RPC:错误原因如实带给手机端(与 CodexUsage.error 同款
		// "读不到 ≠ 没有值"的呈现纪律)。
		resp.Error = errTextOrEmpty(err)
		return resp, nil
	}
	resp.Items = nativeOptionsToProto(rows)
	return resp, nil
}

// nativeOptionsToProto agent 层目录 → pb 目录(纯映射)。
func nativeOptionsToProto(rows []agent.NativeConfigOption) []*pb.AgentConfigOption {
	out := make([]*pb.AgentConfigOption, 0, len(rows))
	for _, r := range rows {
		item := &pb.AgentConfigOption{
			Id:           r.ID,
			Name:         r.Name,
			Category:     r.Category,
			Type:         r.Type,
			CurrentValue: r.CurrentValue,
			Writable:     r.Writable,
			Note:         r.Note,
		}
		for _, o := range r.Options {
			item.Options = append(item.Options, &pb.AgentConfigOptionLeaf{
				Value:       o.Value,
				Name:        o.Name,
				Description: o.Description,
				Group:       o.Group,
			})
		}
		out = append(out, item)
	}
	return out
}
