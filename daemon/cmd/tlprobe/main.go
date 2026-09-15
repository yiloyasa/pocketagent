package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

func main() {
	conn, err := grpc.NewClient("127.0.0.1:7246", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	resp, err := pb.NewThreadServiceClient(conn).ListCodexThreads(ctx, &pb.ListCodexThreadsRequest{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("真实标题会话(%d 个,前 10):\n", len(resp.Threads))
	for i, t := range resp.Threads {
		if i >= 10 {
			break
		}
		title := t.Title
		if len([]rune(title)) > 34 {
			r := []rune(title)
			title = string(r[:34])
		}
		cwd := ""
		if len(t.Cwd) > 24 {
			cwd = t.Cwd[len(t.Cwd)-22:]
		} else {
			cwd = t.Cwd
		}
		fmt.Printf("  %-36s | …%s | %s\n", title, cwd, time.UnixMilli(t.UpdatedAtUnixMs).Format("01-02 15:04"))
	}
}
