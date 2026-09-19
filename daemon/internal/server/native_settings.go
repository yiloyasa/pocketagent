package server

import (
	"context"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

func (s *Server) nativeSettings(ctx context.Context, agentID, threadID string) *pb.CodexSettings {
	out := s.agentSettingsSnapshot(agentID)
	if agentID != "zcode" {
		return out
	}
	out.NativePermissionsNote = "ZCode 原生权限模式，修改会同步项目权限偏好并影响该项目的桌面端；与 Codex 沙箱不同，原生协议未报告独立的文件系统沙箱开关。"
	if threadID != "" {
		if thread, ok := s.cfg.Store.Get(threadID); ok && threadAgentID(thread) == agentID {
			if reader, ok := s.cfg.Providers[agentID].(interface {
				SessionCapabilities(context.Context, *store.Thread) (agent.ZcodeSessionCapabilities, error)
			}); ok {
				caps, err := reader.SessionCapabilities(ctx, thread)
				if err == nil {
					out.NativeMode, out.NativeModes = caps.CurrentMode, caps.Modes
					out.NativePermissionsNote += " 当前模式已从该会话回读。"
					return out
				}
				out.NativePermissionsNote += " 会话状态暂不可读：" + err.Error()
			}
		}
	}
	if reader, ok := s.cfg.Providers[agentID].(interface {
		NativeModes(context.Context) ([]string, error)
	}); ok {
		modes, err := reader.NativeModes(ctx)
		if err == nil {
			out.NativeModes = modes
		} else {
			out.NativePermissionsNote += " 可选项暂不可读：" + err.Error()
		}
	}
	return out
}
