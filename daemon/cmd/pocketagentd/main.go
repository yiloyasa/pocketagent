package main

import (
	"flag"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/agent"
	"github.com/pocketagent/daemon/internal/server"
	"github.com/pocketagent/daemon/internal/store"
	"github.com/pocketagent/daemon/internal/term"
)

func defaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".pocketagentd"
	}
	return filepath.Join(home, ".pocketagentd")
}

func main() {
	addr := flag.String("addr", ":7246", "gRPC 监听地址")
	dataDir := flag.String("data", defaultDataDir(), "数据目录(线程存储、token)")
	pairCode := flag.String("pair-code", "", "配对码;为空则不启用鉴权(仅限局域网内自用)")
	workspace := flag.String("workspace", cwd(), "默认工作区")
	flag.Parse()

	if err := os.MkdirAll(*dataDir, 0o700); err != nil {
		log.Fatalf("创建数据目录失败: %v", err)
	}

	st, err := store.NewFileStore(*dataDir)
	if err != nil {
		log.Fatalf("初始化存储失败: %v", err)
	}

	providers := agent.DetectProviders()
	tm := term.NewManager()

	srv := server.New(server.Config{
		Store:      st,
		Providers:  providers,
		Terminals:  tm,
		PairCode:   *pairCode,
		Workspace:  *workspace,
		DataDir:    *dataDir,
	})

	lis, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("监听 %s 失败: %v", *addr, err)
	}

	gs := grpc.NewServer(
		grpc.ChainUnaryInterceptor(srv.UnaryAuth()),
		grpc.ChainStreamInterceptor(srv.StreamAuth()),
		// 【讲解】手机端 keepalive 30s 一次 ping;grpc-go 默认只允许 5 分钟一次,
		// 否则直接 GOAWAY(too_many_pings) 杀连接——终端这类长流会被掐断。
		// 两边对齐:服务端放宽到 10s,移动网络下能尽快发现假连接。
		grpc.KeepaliveParams(keepalive.ServerParameters{Time: 60 * time.Second, Timeout: 15 * time.Second}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{MinTime: 10 * time.Second, PermitWithoutStream: true}),
	)
	pb.RegisterHostServiceServer(gs, srv)
	pb.RegisterThreadServiceServer(gs, srv)
	pb.RegisterTerminalServiceServer(gs, srv)
	pb.RegisterFileSystemServiceServer(gs, srv)
	pb.RegisterGitServiceServer(gs, srv)
	pb.RegisterModelServiceServer(gs, srv)

	names := make([]string, 0, len(providers))
	for id := range providers {
		names = append(names, id)
	}
	if len(names) == 0 {
		names = []string{"echo"}
	}
	// 预热 codex:后台拉起 app-server 并 initialize,MCP/模型列表/首条消息零等待
	if cp, ok := providers["codex"].(*agent.CodexProvider); ok {
		go cp.Warmup()
	}
	log.Printf("pocketagentd 已启动: %s | 工作区: %s | 模型: %v | 鉴权: %t",
		*addr, *workspace, names, *pairCode != "")
	if err := gs.Serve(lis); err != nil {
		log.Fatalf("gRPC 服务退出: %v", err)
	}
}

func cwd() string {
	d, _ := os.Getwd()
	return d
}
