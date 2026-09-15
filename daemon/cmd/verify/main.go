package main

import (
	"context"
	"fmt"
	"log"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

func main() {
	conn, err := grpc.NewClient("127.0.0.1:7268", grpc.WithTransportCredentials(insecure.NewCredentials()))
	must(err)
	defer conn.Close()
	ctx := context.Background()
	ms := pb.NewModelServiceClient(conn)

	// 1) Codex 设置读取
	st, err := ms.GetCodexSettings(ctx, &pb.GetCodexSettingsRequest{})
	must(err)
	fmt.Printf("✓ GetCodexSettings: account=%q approval=%q sandbox=%q\n", st.Account, st.ApprovalPolicy, st.Sandbox)

	// 2) 写入并回读
	st2, err := ms.SetCodexSettings(ctx, &pb.SetCodexSettingsRequest{ApprovalPolicy: "on-request", Sandbox: "workspace-write"})
	must(err)
	fmt.Printf("✓ SetCodexSettings: approval=%q sandbox=%q (写入成功)\n", st2.ApprovalPolicy, st2.Sandbox)

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
