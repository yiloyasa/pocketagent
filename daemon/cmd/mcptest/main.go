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
	must(err)
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := pb.NewModelServiceClient(conn).ListMcpServers(ctx, &pb.ListMcpServersRequest{})
	must(err)
	fmt.Printf("耗时 %v, %d 个 MCP:\n", time.Since(start).Round(time.Millisecond), len(resp.Servers))
	for _, s := range resp.Servers {
		fmt.Printf("  - %s (%d 工具)\n", s.Name, s.Tools)
	}
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
