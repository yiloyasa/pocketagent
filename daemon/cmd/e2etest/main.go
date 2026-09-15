package main

import (
	"context"
	"fmt"
	"io"
	"log"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

func main() {
	conn, err := grpc.NewClient("127.0.0.1:7257", grpc.WithTransportCredentials(insecure.NewCredentials()))
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
		Blocks: []*pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "用一句话介绍你自己,然后说 OK"}}}},
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
