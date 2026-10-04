package server

import (
	"context"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

func (s *Server) SubmitApproval(_ context.Context, req *pb.SubmitApprovalRequest) (*pb.Empty, error) {
	if req.ThreadId == "" || req.Decision == nil || req.Decision.RequestId == "" {
		return nil, errInvalidArg("缺少会话或审批请求 ID")
	}
	waiting, ok := s.approvalSessions.Load(req.ThreadId)
	if !ok || !waiting.(*turnApprovals).deliver(req.Decision) {
		// 协同成员审批映射的收尾(P1 Task 4 最小追加):这条审批在内存里已经
		// 不存在(过期 / 重复提交 / 跨线程提交)时,把协同成员卡上遗留的
		// pending 引用一并摘掉 —— 卡片与真相收敛,否则协同页会一直挂着一条
		// 永远应答不了的审批。非协同线程在此是 no-op(登记簿查不到即返回)。
		s.collabCoordinatorFor().clearStalePendingApprovals(req.ThreadId)
		return nil, errFailedPrecondition("审批已结束或过期，请刷新会话")
	}
	return &pb.Empty{}, nil
}
