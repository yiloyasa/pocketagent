package main

// 验证 SetCodexSettings 真实路由:调 ModelService/SetCodexSettings,
// 然后人工/脚本对比 ~/.codex/config.toml 是否真的被写(桌面端可见)。
import (
	"context"
	"os"
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c := pb.NewModelServiceClient(conn)
	before, _ := c.GetCodexSettings(ctx, &pb.GetCodexSettingsRequest{})
	fmt.Printf("before: approval=%q sandbox=%q model=%q effort=%q\n",
		before.ApprovalPolicy, before.Sandbox, before.Model, before.Effort)

	approval := os.Getenv("APPROVAL")
	if approval == "" {
		approval = "on-request"
	}
	_, err = c.SetCodexSettings(ctx, &pb.SetCodexSettingsRequest{
		ApprovalPolicy: approval,
		Sandbox:        "workspace-write",
	})
	if err != nil {
		log.Fatal("SetCodexSettings: ", err)
	}
	fmt.Println("SetCodexSettings approval=", approval, ";等 2s…")
	time.Sleep(2 * time.Second)

	after, _ := c.GetCodexSettings(ctx, &pb.GetCodexSettingsRequest{})
	fmt.Printf("after: approval=%q sandbox=%q\n", after.ApprovalPolicy, after.Sandbox)
	fmt.Println(">>> 请对比 ~/.codex/config.toml(approval_policy / sandbox_mode)")
}
