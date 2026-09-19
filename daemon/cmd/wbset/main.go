package main

// wbset:写入一份**有辨识度的完整快照**,用于验证
//   ① Set→Get 回读;② daemon 重启后仍在;③ turn/start 真的带上这些参数。
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

func ps(s string) *string { return &s }
func pbool(b bool) *bool  { return &b }

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
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	c := pb.NewModelServiceClient(conn)
	// 注意:模型 id 不动(保持桌面可用的真实值),只改其余项 —— 这样随后跑真实
	// turn 不会被一个不存在的模型卡住,而 turn/start 的日志仍能看到全部参数。
	st, err := c.SetCodexSettings(ctx, &pb.SetCodexSettingsRequest{
		ApprovalPolicy:        ps("never"),
		Sandbox:               ps("danger-full-access"),
		Effort:                ps("low"),
		HideAgentReasoning:    pbool(true),
		ModelVerbosity:        ps("low"),
		ModelReasoningSummary: ps("detailed"),
		DisablePasteBurst:     pbool(true),
	})
	if err != nil {
		log.Fatal("SetCodexSettings: ", err)
	}
	fmt.Printf("after-set: approval=%q sandbox=%q effort=%q hide=%v verbosity=%q summary=%q paste=%v\n",
		st.ApprovalPolicy, st.Sandbox, st.Effort, st.HideAgentReasoning,
		st.ModelVerbosity, st.ModelReasoningSummary, st.DisablePasteBurst)
	for _, s := range st.SettingStatus {
		fmt.Printf("  %-26s state=%-16s\n", s.Key, s.State)
	}
}
