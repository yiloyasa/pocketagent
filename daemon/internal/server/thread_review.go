// thread_review.go —— StartReview("/计划")的 server 路由。
//
// 【为什么单独一个文件】thread.go 正被并行修复批占用,新路由按
// thread_ext.go / thread_thinking.go 的先例落在独立文件,不碰他人区域。
//
// 【路由纪律】与 CompactThread 同款:会话归属以 AgentID 为准(threadAgentID,
// 旧库空串 = codex),非 codex 会话显式 FailedPrecondition,绝不静默换人;
// 线程未在电脑上加载是正常状态(daemon 刚重启 / 新会话还没发过消息),
// 用 ErrReviewThreadNotLoaded 翻译成"先发一条消息"的人话。
package server

import (
	"context"
	"errors"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// StartReview 发起计划审查("/计划"伪命令的落点)。
// 成功即表示 codex 已开审(review/start 发起即回),结果以事件形式
// 流回原会话,手机端现有时间线直接展示。
func (s *Server) StartReview(ctx context.Context, req *pb.StartReviewRequest) (*pb.StartReviewResponse, error) {
	t, ok := s.cfg.Store.Get(req.ThreadId)
	if !ok {
		return nil, errNotFound("会话不存在")
	}
	// 计划模式目前只有 codex 有对等能力(review/start);pi / zcode 显式拒绝,
	// 让手机端 Toast 人话,而不是打到一个不相干的 provider 上。
	if threadAgentID(t) != "codex" {
		return nil, errFailedPrecondition("计划模式目前仅 Codex 会话支持")
	}
	p := s.codexProvider()
	if p == nil {
		return nil, errUnavailable("电脑上没有可用的 codex")
	}
	out, err := p.StartReview(t, req.Focus)
	if err != nil {
		if errors.Is(err, agent.ErrReviewThreadNotLoaded) {
			return nil, errFailedPrecondition("这个会话还没有在电脑上落地(先发一条消息),暂时无法发起计划审查")
		}
		return nil, errInternal(err)
	}
	return &pb.StartReviewResponse{
		ReviewId: out.ReviewThreadID,
		Started:  true,
	}, nil
}
