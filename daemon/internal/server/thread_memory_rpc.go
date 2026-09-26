// thread_memory_rpc.go —— 内存管理（手机端）的两条 RPC：按会话列记忆、清会话记忆。
//
// 【为什么挂在 RuntimeService 下】GetAgentRuntime 已经在回答"这家 agent 有多少记忆"，
// 清单与清除是同一块观测往"管理"方向的延伸 —— 三者共用 runtimeBackend 这一事实源，
// 观测口径（Observe）与清单口径（ListThreadMemories）同源，诊断页与管理页才不会
// 各说各话。
//
// 【agent_id 校验在注册表上，文案动态列举】注册表是 Config.Providers 的键集；
// 错误文案把"已注册的有谁"一并报给客户端 —— 写死名单的话，每接入一家新 agent
// 就要多改一处文案，而忘记改的表现是"用户被提示了一个不存在的限制"。
//
// 【红线】ClearThreadMemory 只动 runtimeBackend 名下的记忆槽与 checkpoint（都在
// daemon 自己的数据目录里）。~/.codex、~/.pi 等原生侧用户数据不进任何路径 ——
// 那是"让会话记忆重新开始"，不是"删用户的会话"。
package server

import (
	"context"
	"sort"
	"strings"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// ListThreadMemories 按 会话 列出某 agent 名下的语义记忆。
func (s *Server) ListThreadMemories(_ context.Context, req *pb.ListThreadMemoriesRequest) (*pb.ListThreadMemoriesResponse, error) {
	agentID := strings.TrimSpace(req.GetAgentId())
	if agentID == "" {
		return nil, errInvalidArg("缺少 agent_id")
	}
	if err := s.requireRegisteredAgent(agentID); err != nil {
		return nil, err
	}
	resp := &pb.ListThreadMemoriesResponse{SampledAtUnixMs: time.Now().UnixMilli()}
	if s.runtimeBackend == nil {
		// 没有运行时后端 = 没有任何统一 Runtime 记忆可列。回空清单是如实的
		// （生产路径 backend 恒非 nil；nil 只出现在精简测试装配里）。
		return resp, nil
	}
	rows, totals := s.runtimeBackend.ListThreadMemories(agentID)
	resp.TotalThreads = int32(totals.Threads)
	resp.TotalEntries = int32(totals.Entries)
	resp.TotalBytes = totals.Bytes
	for _, r := range rows {
		row := &pb.ThreadMemoryRow{
			ThreadId: r.ThreadID,
			Entries:  int32(r.Entries),
			Bytes:    r.Bytes,
			Seq:      r.Seq,
			Loaded:   r.Loaded,
		}
		if !r.LastCheckpoint.IsZero() {
			row.LastCheckpointUnixMs = r.LastCheckpoint.UnixMilli()
		}
		resp.Rows = append(resp.Rows, row)
	}
	return resp, nil
}

// ClearThreadMemory 清掉一个会话名下全部 PocketAgent 侧记忆，返回剩余合计。
func (s *Server) ClearThreadMemory(_ context.Context, req *pb.ClearThreadMemoryRequest) (*pb.ClearThreadMemoryResponse, error) {
	agentID := strings.TrimSpace(req.GetAgentId())
	threadID := strings.TrimSpace(req.GetThreadId())
	if agentID == "" {
		return nil, errInvalidArg("缺少 agent_id")
	}
	if threadID == "" {
		return nil, errInvalidArg("缺少 thread_id")
	}
	if err := s.requireRegisteredAgent(agentID); err != nil {
		return nil, err
	}
	if s.runtimeBackend == nil {
		// 与清单同口径：没有后端 = 没有可清的记忆。但"会话不存在"的语义仍在：
		// found=false 照常回 NotFound，客户端能区分"清掉了"与"本来就没有"。
		return nil, errNotFound("会话 " + threadID + " 在 " + agentID + " 名下没有 PocketAgent 侧记忆")
	}
	totals, found, err := s.runtimeBackend.ClearThreadMemory(agentID, threadID)
	if err != nil {
		return nil, errInternal(err)
	}
	if !found {
		// "没有可清的"必须让客户端知道：把不存在谎报成成功，用户会以为
		// "清过了"，从此不再怀疑那个会话为什么还在老样子。
		return nil, errNotFound("会话 " + threadID + " 在 " + agentID + " 名下没有 PocketAgent 侧记忆")
	}
	// 预算联动：记忆清零，记忆租约同步退出活跃集合——残留的租约会让诊断页
	// 显示"没有记忆却有压力"（账必须跟得上事实；下一轮写入会重新起账）。
	s.releaseThreadMemoryLeases(agentID, threadID)
	return &pb.ClearThreadMemoryResponse{
		TotalThreads: int32(totals.Threads),
		TotalEntries: int32(totals.Entries),
		TotalBytes:   totals.Bytes,
	}, nil
}

// requireRegisteredAgent 校验 agent_id 在注册表里，不在则 InvalidArgument 并把
// 已注册名单动态列进文案（排序保证文案稳定，可复现）。
func (s *Server) requireRegisteredAgent(agentID string) error {
	ids := make([]string, 0, len(s.cfg.Providers))
	for id := range s.cfg.Providers {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if id == agentID {
			return nil
		}
	}
	// 与 GetAgentRuntime 同一句式（"注册表里没有名为 X 的 agent"），外加动态名单
	// —— 用户看到"有谁"，才知道该选谁，而不是只得到一句"没有"。
	return errInvalidArg("注册表里没有名为 " + agentID + " 的 agent（已注册: " + strings.Join(ids, ", ") + "）")
}
