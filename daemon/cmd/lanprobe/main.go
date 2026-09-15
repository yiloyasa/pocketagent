package main

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

func main() {
	c, err := grpc.NewClient("127.0.0.1:7246", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil { panic(err) }
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	h, err := pb.NewHostServiceClient(c).GetHostInfo(ctx, &pb.GetHostInfoRequest{})
	if err != nil { panic(err) }
	fmt.Println("lan_ipv4 =", h.LanIpv4, "| tailscale =", h.TailscaleIpv4)
}
