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
		return nil, errFailedPrecondition("审批已结束或过期，请刷新会话")
	}
	return &pb.Empty{}, nil
}
