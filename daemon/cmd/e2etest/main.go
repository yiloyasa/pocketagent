package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

func main() {
	// 历史上 e2etest 打的是手工起的测试实例(7257 端口),默认值保持不变。
	addr := flag.String("addr", "127.0.0.1:7257", "daemon 地址(ip:port)")
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
	ctx := context.Background()

	// 1) 模型列表(应含 codex 真实模型)
	models, err := pb.NewModelServiceClient(conn).ListModels(ctx, &pb.ListModelsRequest{})
	must(err)
	fmt.Printf("✓ 模型列表(%d 个):\n", len(models.Models))
	for i, m := range models.Models {
		if i < 6 {
			fmt.Printf("    %-28s %s\n", m.Id, m.DisplayName)
		}
	}

	// 2) 指定真实模型跑一轮
	threads := pb.NewThreadServiceClient(conn)
	th, err := threads.CreateThread(ctx, &pb.CreateThreadRequest{ModelId: "codex/default"})
	must(err)
	stream, err := threads.StartTurn(ctx)
	must(err)
	must(stream.Send(&pb.TurnClientMessage{Msg: &pb.TurnClientMessage_Input{Input: &pb.TurnInput{
		ThreadId: th.Id,
		Blocks:   []*pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "用一句话介绍你自己,然后说 OK"}}}},
	}}}))
	thinking := ""
	text := ""
	for {
		ev, err := stream.Recv()
		if err == io.EOF {
			break
		}
		must(err)
		switch m := ev.Msg.(type) {
		case *pb.TurnServerMessage_ThinkingDelta:
			thinking += m.ThinkingDelta.Delta
		case *pb.TurnServerMessage_TextDelta:
			text += m.TextDelta.Delta
		case *pb.TurnServerMessage_Completed:
			fmt.Printf("✓ 本轮完成:思考 %d 字,回复 %d 字\n", len([]rune(thinking)), len([]rune(text)))
		}
	}

	// 3) 历史
	hist, err := threads.GetThreadHistory(ctx, &pb.GetThreadHistoryRequest{ThreadId: th.Id})
	must(err)
	fmt.Printf("✓ 历史 %d 条: ", len(hist.Messages))
	for _, m := range hist.Messages {
		fmt.Printf("[%s %d字] ", m.Role, len([]rune(firstText(m.Blocks))))
	}
	fmt.Println("\n=== P1 端到端全部通过 ===")
}

func firstText(blocks []*pb.ContentBlock) string {
	for _, b := range blocks {
		if t, ok := b.Block.(*pb.ContentBlock_Text); ok {
			return t.Text.Text
		}
	}
	return ""
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
