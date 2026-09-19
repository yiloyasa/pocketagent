package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

func main() {
	// 历史上 verify 打的是手工起的测试实例(7268 端口),默认值保持不变。
	addr := flag.String("addr", "127.0.0.1:7268", "daemon 地址(ip:port)")
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
	must(err)
	defer conn.Close()
	ctx := context.Background()
	ms := pb.NewModelServiceClient(conn)

	// 1) Codex 设置读取
	st, err := ms.GetCodexSettings(ctx, &pb.GetCodexSettingsRequest{})
	must(err)
	fmt.Printf("✓ GetCodexSettings: account=%q approval=%q sandbox=%q\n", st.Account, st.ApprovalPolicy, st.Sandbox)

	// 2) 写入并回读(proto3 optional:用指针表示"显式修改这一项")
	onReq, wsWrite := "on-request", "workspace-write"
	st2, err := ms.SetCodexSettings(ctx, &pb.SetCodexSettingsRequest{
		ApprovalPolicy: &onReq,
		Sandbox:        &wsWrite,
	})
	must(err)
	fmt.Printf("✓ SetCodexSettings: approval=%q sandbox=%q (写入成功,逐项状态 %d 条)\n",
		st2.ApprovalPolicy, st2.Sandbox, len(st2.SettingStatus))

	// 3) MCP 服务器列表
	mcp, err := ms.ListMcpServers(ctx, &pb.ListMcpServersRequest{})
	must(err)
	fmt.Printf("✓ ListMcpServers: %d 个\n", len(mcp.Servers))
	for i, m := range mcp.Servers {
		if i < 5 {
			fmt.Printf("    %-14s %d 工具\n", m.Name, m.Tools)
		}
	}

	// 4) 盘符
	roots, err := pb.NewFileSystemServiceClient(conn).ListRoots(ctx, &pb.ListRootsRequest{})
	must(err)
	names := ""
	for _, r := range roots.Roots {
		names += r.Path + " "
	}
	fmt.Printf("✓ ListRoots: %s\n", names)

	// 5) 重命名 + 归档/恢复
	ts := pb.NewThreadServiceClient(conn)
	th, err := ts.CreateThread(ctx, &pb.CreateThreadRequest{})
	must(err)
	rn, err := ts.RenameThread(ctx, &pb.RenameThreadRequest{ThreadId: th.Id, Title: "验证标题"})
	must(err)
	fmt.Printf("✓ RenameThread: %q\n", rn.Title)
	ar, err := ts.ArchiveThread(ctx, &pb.ArchiveThreadRequest{ThreadId: th.Id, Archived: true})
	must(err)
	fmt.Printf("✓ Archive→status=%v\n", ar.Status == pb.ThreadStatus_THREAD_STATUS_ARCHIVED)
	lst, err := ts.ListThreads(ctx, &pb.ListThreadsRequest{IncludeArchived: true})
	must(err)
	fmt.Printf("✓ ListThreads(含归档): %d 个\n", len(lst.Threads))

	fmt.Println("=== daemon 侧新功能全部验证通过 ===")
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
