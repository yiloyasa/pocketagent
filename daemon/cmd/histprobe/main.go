// histprobe —— 只读历史探针:对指定 thread 打 GetThread + GetThreadHistory,
// 逐条打印回包消息(role + 前 60 字)。零写操作(不发轮次/不建线),用于诊断
// "app 打开老会话历史为空"时先确认 daemon 侧事实。认证与 chatprobe 同源:
// 读 -data 目录的 pair-code.txt 派生 legacy token。
package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/server"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:7246", "daemon 地址(ip:port)")
	plaintext := flag.Bool("plaintext", false, "连明文 gRPC;默认 TLS")
	thread := flag.String("thread", "", "要查的 thread id(必填)")
	rounds := flag.Int("rounds", 20, "GetThreadHistory 的 rounds(与 app 默认档一致)")
	dataDir := flag.String("data", "", "daemon 数据目录(默认 ~/.pocketagentd,读 pair-code.txt)")
	flag.Parse()
	if *thread == "" {
		log.Fatal("用法: histprobe -thread <id> [-addr ip:port] [-rounds n] [-data dir]")
	}

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

	ctx := context.Background()
	{
		dir := *dataDir
		if dir == "" {
			home, _ := os.UserHomeDir()
			dir = filepath.Join(home, ".pocketagentd")
		}
		if raw, rerr := os.ReadFile(filepath.Join(dir, "pair-code.txt")); rerr == nil {
			if tok := server.LegacyTokenFor(strings.TrimSpace(string(raw))); tok != "" {
				ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+tok)
			}
		} else {
			fmt.Println("(未读到 pair-code.txt,按无鉴权尝试)")
		}
	}

	tc := pb.NewThreadServiceClient(conn)
	t, err := tc.GetThread(ctx, &pb.GetThreadRequest{ThreadId: *thread})
	if err != nil {
		log.Fatal("GetThread: ", err)
	}
	fmt.Printf("[GetThread] id=%s title=%q agent=%q workspace=%q status=%v model=%q lastErr=%q\n",
		t.Id, t.Title, t.AgentId, t.Workspace, t.Status, t.ModelId, t.LastTurnError)

	hist, err := tc.GetThreadHistory(ctx, &pb.GetThreadHistoryRequest{ThreadId: *thread, Rounds: int32(*rounds)})
	if err != nil {
		log.Fatal("GetThreadHistory: ", err)
	}
	fmt.Printf("[GetThreadHistory] rounds=%d → %d 条消息:\n", *rounds, len(hist.Messages))
	for i, m := range hist.Messages {
		txt := ""
		for _, b := range m.Blocks {
			if tb, ok := b.Block.(*pb.ContentBlock_Text); ok {
				txt = tb.Text.Text
				break
			}
		}
		r := []rune(txt)
		if len(r) > 60 {
			r = r[:60]
		}
		fmt.Printf("  %2d [%s] id=%s turn=%s %q\n", i, m.Role, m.MessageId, m.TurnId, string(r))
	}
}
