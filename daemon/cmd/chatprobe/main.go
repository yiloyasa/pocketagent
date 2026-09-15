package main

// 端到端对话探针:完全复刻手机的调用路径(CreateThread → StartTurn 流),
// 逐事件打印 + 总超时,用于真验证 codex 是否收到任务并回复。
import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

func main() {
	prompt := "Reply with exactly: PONG"
	if len(os.Args) > 1 {
		prompt = os.Args[1]
	}
	conn, err := grpc.NewClient("127.0.0.1:7246", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	tc := pb.NewThreadServiceClient(conn)
	ctx := context.Background()

	th, err := tc.CreateThread(ctx, &pb.CreateThreadRequest{})
	if err != nil {
		log.Fatal("CreateThread: ", err)
	}
	fmt.Printf("[probe] thread=%s created\n", th.Id)

	start := time.Now()
	stream, err := tc.StartTurn(ctx)
	if err != nil {
		log.Fatal("StartTurn: ", err)
	}
	in := &pb.TurnInput{ThreadId: th.Id, Blocks: []*pb.ContentBlock{{
		Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: prompt}},
	}}}
	if err := stream.Send(&pb.TurnClientMessage{Msg: &pb.TurnClientMessage_Input{Input: in}}); err != nil {
		log.Fatal("send input: ", err)
	}

	done := make(chan error, 1)
	var gotText string
	var events int
	go func() {
		for {
			ev, err := stream.Recv()
			if err == io.EOF {
				done <- nil
				return
			}
			if err != nil {
				done <- err
				return
			}
			events++
			elapsed := time.Since(start).Round(time.Millisecond)
			switch m := ev.Msg.(type) {
			case *pb.TurnServerMessage_Started:
				fmt.Printf("[%6s] STARTED\n", elapsed)
			case *pb.TurnServerMessage_TextDelta:
				gotText += m.TextDelta.Delta
				fmt.Printf("[%6s] TEXT_DELTA +%q\n", elapsed, m.TextDelta.Delta)
			case *pb.TurnServerMessage_ThinkingDelta:
				fmt.Printf("[%6s] THINKING +%q\n", elapsed, cut(m.ThinkingDelta.Delta, 60))
			case *pb.TurnServerMessage_ToolCall:
				ph := "STARTED"
				if m.ToolCall.Phase == pb.ToolCallUpdate_FINISHED {
					ph = "COMPLETED"
				}
				fmt.Printf("[%6s] TOOL %s (%s)\n", elapsed, m.ToolCall.Call.Name, ph)
			case *pb.TurnServerMessage_ApprovalRequest:
				fmt.Printf("[%6s] APPROVAL_REQUEST id=%s action=%s\n", elapsed,
					m.ApprovalRequest.RequestId, m.ApprovalRequest.Action.Name)
				if os.Getenv("AUTO_APPROVE") == "1" {
					dec := &pb.ApprovalDecision{RequestId: m.ApprovalRequest.RequestId, Approved: true}
					_ = stream.Send(&pb.TurnClientMessage{Msg: &pb.TurnClientMessage_Approval{Approval: dec}})
					fmt.Printf("[%6s]   -> auto-approved\n", elapsed)
				}
			case *pb.TurnServerMessage_Completed:
				fmt.Printf("[%6s] COMPLETED\n", elapsed)
			case *pb.TurnServerMessage_Failed:
				fmt.Printf("[%6s] FAILED: %s\n", elapsed, m.Failed.Error.Message)
			default:
				fmt.Printf("[%6s] EVENT #%d (case %T)\n", elapsed, events, ev.Msg)
			}
			if m, ok := ev.Msg.(*pb.TurnServerMessage_Completed); ok {
				_ = m
				done <- nil
				return
			}
			if m, ok := ev.Msg.(*pb.TurnServerMessage_Failed); ok {
				_ = m
				done <- fmt.Errorf("turn failed")
				return
			}
		}
	}()

	select {
	case err := <-done:
		fmt.Printf("\n[probe] done in %v, events=%d\n", time.Since(start).Round(time.Millisecond), events)
		fmt.Printf("[probe] final text: %q\n", gotText)
		if err != nil {
			log.Fatal(err)
		}
	case <-time.After(180 * time.Second):
		fmt.Printf("\n[probe] TIMEOUT 180s, events=%d, text=%q —— 轮次挂死\n", events, gotText)
	}
}

func cut(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
