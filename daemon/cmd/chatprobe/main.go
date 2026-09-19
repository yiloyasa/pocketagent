package main

// 端到端对话探针:完全复刻手机的调用路径(CreateThread → StartTurn 流),
// 逐事件打印 + 总超时,用于真验证 codex 是否收到任务并回复。
import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:7246", "daemon 地址(ip:port)")
	plaintext := flag.Bool("plaintext", false, "连明文 gRPC(对付 -plaintext 起的测试 daemon);默认 TLS")
	// -agent 指定会话 agent(2026-09-19 闭环验收 zcode 用):daemon 侧 CreateThread
	// 规定 agent_id 为空 = codex(老客户端语义),探针原来没法指定 zcode/pi。
	agent := flag.String("agent", "", "会话 agent(zcode/pi/codex);空 = codex(老客户端语义)")
	mode := flag.String("mode", "turn", "turn=建线+发轮次(默认);optimize=只打 OptimizePrompt RPC")
	style := flag.String("style", "", "optimize 模式的改写风格(concise/detailed/空=默认)")
	flag.Parse()
	// 提示词:第一个非 flag 位置参数(【注意】必须放在 flag.Parse() 之后用
	// flag.Arg(0),不能沿用旧的 os.Args[1] —— 引入 flag 包后 os.Args 里仍带着
	// 已解析的 flag,直接取 os.Args[1] 会把 "-agent" 当成提示词发给模型,
	// 2026-09-19 实测踩过;放在 Parse 之前 flag.Arg(0) 恒为空,同样踩过)。
	prompt := flag.Arg(0)
	if prompt == "" {
		prompt = "Reply with exactly: PONG"
	}

	// daemon 生产默认 TLS(自签证书)。诊断用 InsecureSkipVerify 跳过自签校验;
	// 指纹核对走 tlsprobe。-plaintext=true 回明文。
	var creds credentials.TransportCredentials
	if *plaintext {
		creds = insecure.NewCredentials()
	} else {
		creds = credentials.NewTLS(&tls.Config{InsecureSkipVerify: true}) //nolint:gosec // 探针仅本机诊断用
	}
	conn, err := grpc.NewClient(*addr, grpc.WithTransportCredentials(creds))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	tc := pb.NewThreadServiceClient(conn)
	ctx := context.Background()
	start := time.Now()

	// -mode optimize:只打一次 OptimizePrompt RPC(2026-09-19 优化按钮真机验收
	// 用:走 daemon 的生产 gRPC 面,底层 zcodeapp.GenerateText → session/create
	// + workspace/generateText),打印改写结果后退出,不走建线/轮次流程。
	if *mode == "optimize" {
		resp, err := tc.OptimizePrompt(ctx, &pb.OptimizePromptRequest{Text: prompt, Style: *style})
		if err != nil {
			log.Fatalf("OptimizePrompt: %v", err)
		}
		fmt.Printf("[probe] optimized(%v): %s\n", time.Since(start), resp.GetOptimized())
		return
	}

	th, err := tc.CreateThread(ctx, &pb.CreateThreadRequest{AgentId: *agent})
	if err != nil {
		log.Fatal("CreateThread: ", err)
	}
	fmt.Printf("[probe] thread=%s created\n", th.Id)

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
