package server

import (
	"context"
	"testing"
	"time"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

func TestApprovalSurvivesTransportDisconnectAndResumesByRequestID(t *testing.T) {
	approved := make(chan bool, 1)
	p := &concurrencyProvider{run: func(ctx context.Context, _ agent.TurnRequest, _ agent.Sink, ask agent.ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
		ok, _, err := ask(ctx, &pb.ToolCallBlock{Id: "native-tool", Name: "shell"})
		approved <- ok
		return nil, &pb.Usage{}, err
	}}
	s := concurrencyServer(t, p)
	f := concurrencyStream("request approval")
	done := make(chan error, 1)
	go func() { done <- s.StartTurn(f) }()
	defer closeIfOpenFeed(f)
	var request *pb.ApprovalRequest
	deadline := time.After(3 * time.Second)
	for request == nil {
		select {
		case <-deadline:
			t.Fatal("approval never appeared")
		default:
		}
		for _, message := range f.snapshot() {
			if message.GetApprovalRequest() != nil {
				request = message.GetApprovalRequest()
			}
		}
		time.Sleep(time.Millisecond)
	}
	// EOF removes the original request transport, but is not a user denial.
	close(f.in)
	thread, err := s.GetThread(context.Background(), &pb.GetThreadRequest{ThreadId: "t"})
	if err != nil || len(thread.PendingApprovals) != 1 {
		t.Fatalf("pending approval not recoverable: %v %v", thread, err)
	}
	select {
	case <-approved:
		t.Fatal("disconnect decided the user's approval")
	case <-time.After(30 * time.Millisecond):
	}
	if _, err := s.SubmitApproval(context.Background(), &pb.SubmitApprovalRequest{ThreadId: "other",
		Decision: &pb.ApprovalDecision{RequestId: request.RequestId, Approved: true}}); err == nil {
		t.Fatal("cross-thread approval was accepted")
	}
	if _, err := s.SubmitApproval(context.Background(), &pb.SubmitApprovalRequest{ThreadId: "t",
		Decision: &pb.ApprovalDecision{RequestId: request.RequestId, Approved: true}}); err != nil {
		t.Fatal(err)
	}
	awaitTurn(t, done)
	if !<-approved {
		t.Fatal("resumed approval not delivered")
	}
	if _, err := s.SubmitApproval(context.Background(), &pb.SubmitApprovalRequest{ThreadId: "t",
		Decision: &pb.ApprovalDecision{RequestId: request.RequestId, Approved: true}}); err == nil {
		t.Fatal("expired replay accepted")
	}
}

func closeIfOpenFeed(f *feedTurnStream) {
	defer func() { _ = recover() }()
	close(f.in)
}
