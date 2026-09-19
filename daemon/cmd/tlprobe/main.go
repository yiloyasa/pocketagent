package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
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
