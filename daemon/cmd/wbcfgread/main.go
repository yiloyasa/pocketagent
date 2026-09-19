package main

// wbcfgread:只读探针 —— 打印 GetCodexSettings 的当前值 + 逐项状态。
// 用于"daemon 重启后回读"的证据(不写任何东西)。
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

	st, err := pb.NewModelServiceClient(conn).GetCodexSettings(ctx, &pb.GetCodexSettingsRequest{})
	if err != nil {
		log.Fatal("GetCodexSettings: ", err)
	}
	fmt.Printf("approval=%q sandbox=%q model=%q effort=%q\n",
		st.ApprovalPolicy, st.Sandbox, st.Model, st.Effort)
	fmt.Printf("verbosity=%q summary=%q hide=%v paste=%v\n",
		st.ModelVerbosity, st.ModelReasoningSummary, st.HideAgentReasoning, st.DisablePasteBurst)
	for _, s := range st.SettingStatus {
		fmt.Printf("  %-26s state=%-16s detail=%s\n", s.Key, s.State, s.Detail)
	}
}
