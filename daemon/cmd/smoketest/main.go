// smoketest:一条命令验证 daemon 全链路(GetHostInfo → CreateThread →
// StartTurn 流式 → ListThreads)。echo 模式下零成本跑通。
package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:7246", "daemon 地址(ip:port)")
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
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	host := pb.NewHostServiceClient(conn)
	threads := pb.NewThreadServiceClient(conn)

	// 1) 主机信息
	info, err := host.GetHostInfo(context.Background(), &pb.GetHostInfoRequest{})
	must(err)
	fmt.Printf("✓ GetHostInfo: %s (%s/%s) shell=%s auth=%v\n",
		info.DisplayName, info.Os, info.Arch, info.DefaultShell, info.AuthRequired)

	// 2) 建会话
	th, err := threads.CreateThread(context.Background(), &pb.CreateThreadRequest{})
	must(err)
	fmt.Printf("✓ CreateThread: %s\n", th.Id)

	// 3) StartTurn 双向流(echo provider)
	stream, err := threads.StartTurn(context.Background())
	must(err)
	must(stream.Send(&pb.TurnClientMessage{Msg: &pb.TurnClientMessage_Input{Input: &pb.TurnInput{
		ThreadId: th.Id,
		Blocks:   []*pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "你好,请介绍一下自己"}}}},
	}}}))
	text := ""
	tools := 0
	for {
		ev, err := stream.Recv()
		if err == io.EOF {
			break
		}
		must(err)
		switch m := ev.Msg.(type) {
		case *pb.TurnServerMessage_Started:
			fmt.Printf("✓ TurnStarted: %s (model %s)\n", m.Started.TurnId, m.Started.ModelId)
		case *pb.TurnServerMessage_TextDelta:
			text += m.TextDelta.Delta
		case *pb.TurnServerMessage_ToolCall:
			tools++
			fmt.Printf("✓ ToolCall[%d]: %s phase=%v\n", tools, m.ToolCall.Call.Name, m.ToolCall.Phase)
		case *pb.TurnServerMessage_Completed:
			fmt.Printf("✓ TurnCompleted: %d 字, %d 工具调用, tokens in/out=%d/%d\n",
				len([]rune(text)), tools, m.Completed.Usage.GetInputTokens(), m.Completed.Usage.GetOutputTokens())
		case *pb.TurnServerMessage_Failed:
			fmt.Printf("✗ TurnFailed: %s\n", m.Failed.Error.Message)
		}
	}

	// 4) 会话列表
	list, err := threads.ListThreads(context.Background(), &pb.ListThreadsRequest{})
	must(err)
	fmt.Printf("✓ ListThreads: %d 个\n", len(list.Threads))

	fmt.Println("\n=== 全链路冒烟通过 ===")
	_ = time.Now
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
