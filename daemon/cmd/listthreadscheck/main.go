// listthreadscheck —— 走 daemon 生产 gRPC 面调一次 ListThreads,验证
// 「pi 子代理存根登记」在真实进程里确实被触发(T-SUBA-PI 端到端自证)。
//
// 【为什么需要它】单测钉的是逻辑,in-process 不覆盖"ListThreads 里到底有没有
// 调它"。本探针不引入任何测试钩子 —— 就是手机端那条路:连主 daemon(默认 TLS +
// 配对码派生 token)、调 ListThreads、按 id 前缀数出 pi 子代理存根。
//
// 用法:go run ./cmd/listthreadscheck [-addr 127.0.0.1:7246] [-data <数据目录>]
//
// 红线:只读 pair-code.txt 派生 token(与既有权衡链一致),不打印 token 本身。
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
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/server"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

func main() {
	log.SetFlags(0)
	addr := flag.String("addr", "127.0.0.1:7246", "主 daemon 地址")
	dataDir := flag.String("data", "", "daemon 数据目录(默认 ~/.pocketagentd,用于读配对码)")
	del := flag.String("delete", "", "若给定线程 id,则对该线程执行一次 DeleteThread 并打印结果(验证删除链路)")
	flag.Parse()

	dir := *dataDir
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("取用户目录失败: %v", err)
		}
		dir = filepath.Join(home, ".pocketagentd")
	}

	creds := credentials.NewTLS(&tls.Config{InsecureSkipVerify: true}) //nolint:gosec // 本机诊断探针
	conn, err := grpc.NewClient(*addr, grpc.WithTransportCredentials(creds))
	if err != nil {
		log.Fatalf("连接 daemon 失败: %v", err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 配对码 → legacy token(与 authorized → devices.Validate 的回退链同源)。
	if raw, err := os.ReadFile(filepath.Join(dir, "pair-code.txt")); err == nil {
		code := strings.TrimSpace(string(raw))
		if tok := server.LegacyTokenFor(code); tok != "" {
			ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+tok)
		}
	} else {
		fmt.Println("(未读到 pair-code.txt,按无鉴权回环尝试)")
	}

	svc := pb.NewThreadServiceClient(conn)
	// 顺带自证:daemon 报出的 capabilities 里是否含 collab:<agent>(手机端据此
	// 点亮"多 agent 协同"入口,而不是摆一个恒假的"开发中"开关)。
	if hi, err := pb.NewHostServiceClient(conn).GetHostInfo(ctx, &pb.GetHostInfoRequest{}); err == nil {
		var collab []string
		for _, c := range hi.GetCapabilities() {
			if strings.HasPrefix(c, "collab:") {
				collab = append(collab, c)
			}
		}
		fmt.Printf("协同能力(capabilities 里的 collab:*): %d 条 %v\n", len(collab), collab)
	} else {
		fmt.Printf("GetHostInfo 失败(不影响主结论): %v\n", err)
	}
	if *del != "" {
		if _, err := svc.DeleteThread(ctx, &pb.DeleteThreadRequest{ThreadId: *del}); err != nil {
			log.Fatalf("DeleteThread(%s) 失败: %v", *del, err)
		}
		fmt.Printf("DeleteThread(%s) 成功 -- 删除链路通\n", *del)
		return
	}
	resp, err := svc.ListThreads(ctx, &pb.ListThreadsRequest{IncludeArchived: true})
	if err != nil {
		log.Fatalf("ListThreads 失败: %v", err)
	}
	total, subs, linked := 0, 0, 0
	for _, t := range resp.GetThreads() {
		total++
		if strings.HasPrefix(t.GetId(), "pi-sub-") {
			subs++
			if t.GetParentThreadId() != "" {
				linked++
			}
		}
	}
	fmt.Printf("ListThreads 返回 %d 条线程;其中 pi 子代理存根 %d 条(挂上父链 %d 条)\n", total, subs, linked)
	if subs == 0 {
		fmt.Println("→ 未发现 pi 子代理存根:该账号下没有 pi 会话留下 subagent-artifacts,或本次扫描无候选运行")
	}
}
