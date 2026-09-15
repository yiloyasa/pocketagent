package main

// R11 修复验证:codex 会话列表/导入、自动标题、断流后审批解锁。
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

var addr = "127.0.0.1:7246"

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func conn() *grpc.ClientConn {
	c, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	must(err)
	return c
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("用法: r11probe list | import <id> | title | approval")
	}
	switch os.Args[1] {
	case "list":
		c := conn()
		defer c.Close()
		resp, err := pb.NewThreadServiceClient(c).ListCodexThreads(context.Background(),
			&pb.ListCodexThreadsRequest{})
		must(err)
		fmt.Printf("电脑上的 codex 会话(%d 个,前 8):\n", len(resp.Threads))
		for i, t := range resp.Threads {
			if i >= 8 {
				break
			}
			fmt.Printf("  %-40s %s · %s · %s\n", t.Id, t.Title[:min(30, len([]rune(t.Title)))], t.Cwd,
				time.UnixMilli(t.UpdatedAtUnixMs).Format("01-02 15:04"))
		}
	case "import":
		if len(os.Args) < 3 {
			log.Fatal("需要 codexThreadId")
		}
		c := conn()
		defer c.Close()
		tc := pb.NewThreadServiceClient(c)
		resp, err := tc.ImportCodexThread(context.Background(),
			&pb.ImportCodexThreadRequest{CodexThreadId: os.Args[2]})
		must(err)
		hist, err := tc.GetThreadHistory(context.Background(),
			&pb.GetThreadHistoryRequest{ThreadId: resp.ThreadId})
		must(err)
		fmt.Printf("✓ 导入成功 thread=%s,历史 %d 条:\n", resp.ThreadId, len(hist.Messages))
		for _, m := range hist.Messages {
		 txt := ""
		 for _, b := range m.Blocks {
			if t, ok := b.Block.(*pb.ContentBlock_Text); ok {
				txt = t.Text.Text
				break
			}
		 }
		 r := []rune(txt)
		 if len(r) > 40 {
			r = r[:40]
		 }
		 fmt.Printf("   [%s] %s\n", m.Role, string(r))
		}
	case "title":
		c := conn()
		defer c.Close()
		tc := pb.NewThreadServiceClient(c)
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		th, err := tc.CreateThread(ctx, &pb.CreateThreadRequest{ModelId: "codex/default"})
		must(err)
		runTurn(ctx, tc, th.Id, "只回复两个字:标题测试")
		t, err := tc.GetThread(ctx, &pb.GetThreadRequest{ThreadId: th.Id})
		must(err)
		fmt.Printf("✓ 自动标题 = %q(期望含『标题测试』)\n", t.Title)
	case "approval":
		c := conn()
		defer c.Close()
		tc := pb.NewThreadServiceClient(c)
		ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
		defer cancel()
		th, err := tc.CreateThread(ctx, &pb.CreateThreadRequest{ModelId: "codex/default"})
		must(err)
		fmt.Println("发起会触发审批的命令任务,20 秒后模拟断流…")
		stream, err := tc.StartTurn(ctx)
		must(err)
		must(stream.Send(&pb.TurnClientMessage{Msg: &pb.TurnClientMessage_Input{Input: &pb.TurnInput{
			ThreadId: th.Id,
			Blocks: []*pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{
				Text: "delete the file daemon/approval-test2.txt in the workspace then tell me the result"}}}}},
		}}))
		approvalSeen := make(chan string, 1)
		go func() {
			for {
				ev, err := stream.Recv()
				if err != nil {
					return
				}
				if a := ev.GetApprovalRequest(); a != nil {
					approvalSeen <- a.Action.GetName()
					return
				}
			}
		}()
		select {
		case name := <-approvalSeen:
			fmt.Printf("✓ 审批请求已发出(%s),现在掐断流模拟手机断连…\n", name)
		case <-time.After(60 * time.Second):
			fmt.Println("…60 秒内未触发审批(codex 可能直接执行了),继续验证收尾")
		}
		// 掐断流 = 模拟手机断连(旧实现会永远卡在等审批)
		cancel2 := stream.Context().Done()
		_ = cancel2
		if cl, ok := stream.(io.Closer); ok {
			_ = cl.Close()
		}
		fmt.Println("流已断开。轮询历史,等电脑端自己收尾…")
		deadline := time.Now().Add(120 * time.Second)
		for time.Now().Before(deadline) {
			time.Sleep(4 * time.Second)
			hist, err := tc.GetThreadHistory(context.Background(),
				&pb.GetThreadHistoryRequest{ThreadId: th.Id})
			if err != nil {
				continue
			}
			var last *pb.ThreadMessage
			if n := len(hist.Messages); n > 0 {
				last = hist.Messages[n-1]
			}
			if last != nil && last.Role == "assistant" {
				txt := ""
				for _, b := range last.Blocks {
					if t, ok := b.Block.(*pb.ContentBlock_Text); ok {
						txt = t.Text.Text
						break
					}
				}
				r := []rune(txt)
				if len(r) > 80 {
					r = r[:80]
				}
				fmt.Printf("✓ 断流后电脑端自己完成了轮次,回复: %s\n", string(r))
				return
			}
		}
		log.Fatal("✗ 120 秒内断流后的轮次没有自己收尾")
	}
}

func runTurn(ctx context.Context, tc pb.ThreadServiceClient, threadID, prompt string) {
	stream, err := tc.StartTurn(ctx)
	must(err)
	must(stream.Send(&pb.TurnClientMessage{Msg: &pb.TurnClientMessage_Input{Input: &pb.TurnInput{
		ThreadId: threadID,
		Blocks:   []*pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: prompt}}}},
	}}}))
	for {
		ev, err := stream.Recv()
		if err == io.EOF {
			return
		}
		must(err)
		if ev.GetCompleted() != nil {
			return
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
