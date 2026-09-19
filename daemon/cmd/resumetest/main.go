package main

// 全链路续聊测试(走 daemon 的真实 gRPC 栈,复现"daemon 重启后旧会话卡死"):
//   -phase1:建新线程,发一轮,把 threadId 写到 -out 文件
//   -phase2 -thread <id>:daemon 重启后,向同一线程再发一轮 ——
//            旧实现会在全新 app-server 子进程上直接 turn/start 旧 threadId,
//            被 codex 静默吞掉(无报错、无事件、永不结束)。
import (
	"context"
	"crypto/tls"
	"encoding/json"
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
	addr := flag.String("addr", "127.0.0.1:7257", "daemon gRPC 地址")
	phase := flag.Int("phase", 1, "1=新建线程跑一轮;2=向旧线程续一轮")
	threadID := flag.String("thread", "", "phase2 的目标线程")
	out := flag.String("out", "resumetest-thread.json", "phase1 线程 id 落盘位置")
	plaintext := flag.Bool("plaintext", false, "连明文 gRPC(对付 -plaintext 起的测试 daemon);默认 TLS")
	flag.Parse()

	// daemon 生产默认 TLS(自签证书)。诊断用 InsecureSkipVerify 跳过自签校验;
	// 指纹核对走 tlsprobe。-plaintext=true 回明文。
	var creds credentials.TransportCredentials
	if *plaintext {
		creds = insecure.NewCredentials()
	} else {
		creds = credentials.NewTLS(&tls.Config{InsecureSkipVerify: true}) //nolint:gosec // 探针仅本机诊断用
	}
	conn, err := grpc.NewClient(*addr, grpc.WithTransportCredentials(creds))
	must(err)
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()
	threads := pb.NewThreadServiceClient(conn)

	thID := *threadID
	if *phase == 1 {
		th, err := threads.CreateThread(ctx, &pb.CreateThreadRequest{ModelId: "codex/default"})
		must(err)
		thID = th.Id
	}

	prompt := fmt.Sprintf("Phase%d:只回复两个词: PONG-%d", *phase, *phase)
	blocks, failed := runTurn(ctx, threads, thID, prompt)
	if failed != "" {
		log.Fatalf("✗ 第 %d 阶段失败: %s", *phase, failed)
	}
	fmt.Printf("✓ 第 %d 阶段完成 thread=%s 回复=%.80s\n", *phase, thID, blocks)

	if *phase == 1 {
		must(os.WriteFile(*out, []byte(thID), 0o644))
	}
	fmt.Printf("=== PHASE%d OK ===\n", *phase)
}

// runTurn 发起一轮并收流,返回最终文本;FAILED 帧以字符串返回。
func runTurn(ctx context.Context, threads pb.ThreadServiceClient, threadID, prompt string) (string, string) {
	stream, err := threads.StartTurn(ctx)
	must(err)
	must(stream.Send(&pb.TurnClientMessage{Msg: &pb.TurnClientMessage_Input{Input: &pb.TurnInput{
		ThreadId: threadID,
		Blocks:   []*pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: prompt}}}},
	}}}))

	var text, failedAt string
	start := time.Now()
	for {
		ev, err := stream.Recv()
		if err == io.EOF {
			break
		}
		must(err)
		switch m := ev.Msg.(type) {
		case *pb.TurnServerMessage_Started:
			fmt.Printf("  [%5.1fs] STARTED\n", time.Since(start).Seconds())
		case *pb.TurnServerMessage_TextDelta:
			text += m.TextDelta.Delta
		case *pb.TurnServerMessage_Completed:
			fmt.Printf("  [%5.1fs] COMPLETED\n", time.Since(start).Seconds())
			if t := finalText(m.Completed.FinalBlocks); t != "" {
				text = t
			}
			return text, ""
		case *pb.TurnServerMessage_Failed:
			failedAt = fmt.Sprintf("code=%s msg=%s partial=%.60s", m.Failed.Error.Code, m.Failed.Error.Message, m.Failed.PartialText)
			return text, failedAt
		}
	}
	return text, ""
}

func finalText(blocks []*pb.ContentBlock) string {
	for _, b := range blocks {
		if t, ok := b.Block.(*pb.ContentBlock_Text); ok {
			return t.Text.Text
		}
	}
	return ""
}

var _ = json.Marshal

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
