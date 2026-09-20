package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
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
	c, err := grpc.NewClient(*addr, grpc.WithTransportCredentials(creds))
	if err != nil {
		panic(err)
	}
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	h, err := pb.NewHostServiceClient(c).GetHostInfo(ctx, &pb.GetHostInfoRequest{})
	if err != nil {
		panic(err)
	}
	fmt.Println("lan_ipv4 =", h.LanIpv4, "| tailscale =", h.TailscaleIpv4)
}
