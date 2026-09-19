package main

// 验证 SetCodexSettings 的真实生效链路:调 ModelService/SetCodexSettings 后
// GetCodexSettings 回读,并打印逐项 setting_status(已生效/未生效/不支持/失败)。
//
// 【重要变更(第 2 轮)】以前这里写的是"人工对比 ~/.codex/config.toml 是否真的被写" ——
// 那是**违规且过时**的写法:本项目红线明令禁止写用户桌面端 config.toml
// (历史上写过 approval_policy,直接改坏了桌面端)。现在的生效路径是
// 每轮 turn/start + thread config 覆盖,不落任何磁盘配置。本探针只做 Set→Get 回读。
import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// pstr 取字符串指针(proto3 optional 字段用 presence 表示"要不要改")。
func pstr(s string) *string { return &s }

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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c := pb.NewModelServiceClient(conn)
	before, _ := c.GetCodexSettings(ctx, &pb.GetCodexSettingsRequest{})
	fmt.Printf("before: approval=%q sandbox=%q model=%q effort=%q verbosity=%q summary=%q hide=%v paste=%v\n",
		before.ApprovalPolicy, before.Sandbox, before.Model, before.Effort,
		before.ModelVerbosity, before.ModelReasoningSummary,
		before.HideAgentReasoning, before.DisablePasteBurst)

	approval := os.Getenv("APPROVAL")
	if approval == "" {
		approval = "on-request"
	}
	_, err = c.SetCodexSettings(ctx, &pb.SetCodexSettingsRequest{
		ApprovalPolicy: pstr(approval),
		Sandbox:        pstr("workspace-write"),
	})
	if err != nil {
		log.Fatal("SetCodexSettings: ", err)
	}
	fmt.Println("SetCodexSettings approval=", approval, ";等 2s…")
	time.Sleep(2 * time.Second)

	after, err := c.GetCodexSettings(ctx, &pb.GetCodexSettingsRequest{})
	if err != nil {
		log.Fatal("GetCodexSettings: ", err)
	}
	fmt.Printf("after: approval=%q sandbox=%q model=%q effort=%q\n",
		after.ApprovalPolicy, after.Sandbox, after.Model, after.Effort)
	fmt.Println("逐项状态(setting_status):")
	for _, st := range after.SettingStatus {
		fmt.Printf("  %-26s state=%-16s detail=%s\n", st.Key, st.State, st.Detail)
	}
}
