package main

import (
	"context"
	"crypto/tls"
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/mdns"
	"github.com/pocketagent/daemon/internal/server"
	"github.com/pocketagent/daemon/internal/store"
	"github.com/pocketagent/daemon/internal/term"
	"github.com/pocketagent/daemon/internal/tlscert"
	"github.com/pocketagent/daemon/internal/version"
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
	// 【PA-REAL-0001 修复】为空不再等于关闭鉴权:不传=自动生成 8 位随机码并
	// 持久化到 <data>/pair-code.txt,启动日志会打印,手机首次连接时输入。
	pairCode := flag.String("pair-code", "", "配对码(鉴权默认开启);不传=自动生成并持久化到 <数据目录>/pair-code.txt")
	// 【PA-REAL-0001 修复】显式降级旗标:完全跳过鉴权,但仅允许本机回环连接
	// (127.0.0.1/::1),局域网/远程一律被拒。应急排障用,平时不要开。
	insecureNoAuth := flag.Bool("insecure-no-auth", false, "应急:关闭配对码鉴权,仅允许本机回环连接;局域网连接会被拒绝")
	workspace := flag.String("workspace", cwd(), "默认工作区")
	// 应急回退:P0-1 起默认 TLS(自签证书 + 手机端指纹锁定);
	// 极端情况(证书文件损坏等)可用 -plaintext 暂时回明文。
	plaintext := flag.Bool("plaintext", false, "应急回退:关闭 TLS 走明文 gRPC(强烈不建议)")
	// P0-3:mDNS 局域网自动发现。手机在"没填过地址/换了新 WiFi"时靠它找到本机;
	// 关闭只影响便利性,手动填地址仍可用。
	mdnsEnable := flag.Bool("mdns", true, "启用 mDNS 局域网自动发现(_pocketagent._tcp)")
	// P1 终端会话生命周期:所有 Attach 断开(无 reader)的会话闲置这么久后
	// 自动杀 shell 并回收,防 pwsh+conhost 随开关终端页线性泄漏;
	// 0 = 禁用回收(旧版行为)。有 reader 在场的会话绝不回收。
	termIdle := flag.Duration("term-idle", 10*time.Minute, "终端会话无 reader 空闲回收时长(如 10m、0.02m);0=禁用")
	flag.Parse()

	if err := os.MkdirAll(*dataDir, 0o700); err != nil {
		log.Fatalf("创建数据目录失败: %v", err)
	}

	// ---- 鉴权模式解析(PA-REAL-0001 修复:默认鉴权开启)----
	var pairCodeValue string
	authMode := "配对码模式(鉴权已启用)"
	switch {
	case *insecureNoAuth:
		if *pairCode != "" {
			log.Fatalf("-pair-code 与 -insecure-no-auth 互斥:要么配对码鉴权,要么显式无鉴权(仅回环),不能同时给")
		}
		authMode = "无鉴权(-insecure-no-auth,仅放行本机回环)"
		log.Printf("============================================================")
		log.Printf("WARNING: INSECURE MODE - 鉴权已完全关闭(-insecure-no-auth)!")
		log.Printf("WARNING: 仅本机回环(127.0.0.1/::1)连接放行,局域网/远程连接一律拒绝。")
		log.Printf("WARNING: 如需手机从局域网连接,请去掉 -insecure-no-auth 重启,")
		log.Printf("WARNING: daemon 会自动生成配对码(文件: %s),手机首次连接时输入。", pairCodePath(*dataDir))
		log.Printf("============================================================")
	default:
		code, generated, err := loadOrGeneratePairCode(*dataDir, *pairCode)
		if err != nil {
			log.Fatalf("配对码初始化失败: %v", err)
		}
		pairCodeValue = code
		switch {
		case generated:
			log.Printf("已自动生成配对码: %s(手机首次连接需输入;已持久化到 %s,重启复用不换码)", pairCodeValue, pairCodePath(*dataDir))
		case *pairCode != "":
			log.Printf("使用命令行指定配对码: %s(手机首次连接需输入;已同步到 %s)", pairCodeValue, pairCodePath(*dataDir))
		default:
			log.Printf("复用既有配对码: %s(手机首次连接需输入;文件: %s)", pairCodeValue, pairCodePath(*dataDir))
		}
	}

	// TLS 凭证:首启自动生成自签证书(ECDSA P-256,20 年),之后一直复用。
	var grpcOpts []grpc.ServerOption
	if *plaintext {
		log.Printf("WARNING: PLAINTEXT MODE ENABLED - gRPC 正在明文传输,token 与会话内容可被同网络者窃听,仅限应急使用!")
	} else {
		cert, err := tlscert.Ensure(*dataDir)
		if err != nil {
			log.Fatalf("TLS 证书初始化失败(可用 -plaintext 应急回退): %v", err)
		}
		grpcOpts = append(grpcOpts, grpc.Creds(credentials.NewTLS(&tls.Config{Certificates: []tls.Certificate{cert}})))
		log.Printf("TLS 已启用: %s", filepath.Join(*dataDir, "certs", tlscert.CertFileName))
	}

	st, err := store.NewFileStore(*dataDir)
	if err != nil {
		// 【P2-5】threads.json 内容损坏已在 store 内自愈(隔离留档 + 空库启动);
		// 走到这里只剩 I/O 类系统性故障(权限/磁盘),静默空库反而危险,维持 Fatal。
		log.Fatalf("初始化存储失败: %v", err)
	}

	providers := agent.DetectProviders()
	tm := term.NewManager()
	// 空闲回收器:随进程存活,daemon 退出时随进程消亡,无需单独停
	tm.SetIdleTimeout(*termIdle)
	tm.StartReaper(context.Background())
	if *termIdle > 0 {
		log.Printf("终端空闲回收已启用: 无 reader 闲置 %s 自动关闭会话", *termIdle)
	}

	srv := server.New(server.Config{
		Store:          st,
		Providers:      providers,
		Terminals:      tm,
		PairCode:       pairCodeValue,
		InsecureNoAuth: *insecureNoAuth,
		Workspace:      *workspace,
		DataDir:        *dataDir,
	})

	lis, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("监听 %s 失败: %v", *addr, err)
	}

	gs := grpc.NewServer(append([]grpc.ServerOption{
		grpc.ChainUnaryInterceptor(srv.UnaryAuth()),
		grpc.ChainStreamInterceptor(srv.StreamAuth()),
		// 【讲解】手机端 keepalive 30s 一次 ping;grpc-go 默认只允许 5 分钟一次,
		// 否则直接 GOAWAY(too_many_pings) 杀连接——终端这类长流会被掐断。
		// 两边对齐:服务端放宽到 10s,移动网络下能尽快发现假连接。
		// Time 25s/Timeout 10s:死连接(Tailscale 半开、手机灭屏断网)最坏 ~35s 检出,
		// 原来的 60+15s 会让 turn 事件管道在死手机上堵一分多钟。
		grpc.KeepaliveParams(keepalive.ServerParameters{Time: 25 * time.Second, Timeout: 10 * time.Second}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{MinTime: 10 * time.Second, PermitWithoutStream: true}),
	}, grpcOpts...)...)
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

	// P0-3:mDNS 局域网自动发现 —— 向同网段宣告 "_pocketagent._tcp" 服务,
	// 手机端 NsdManager 扫到后直接拼出 ip:port,免去手填/换网重填。
	// 【降级原则】多播绑定失败(无多播网卡/防火墙拦 5353)只是丢便利功能,
	// 打警告继续跑,绝不能因它崩掉整个 daemon。
	var mdnsReg *mdns.Registrar
	if *mdnsEnable {
		_, portStr, splitErr := net.SplitHostPort(*addr)
		port := 7246
		if splitErr == nil {
			if p, err := strconv.Atoi(portStr); err == nil {
				port = p
			}
		}
		reg, err := mdns.Register(mdns.Config{
			HostID:      srv.HostID(),
			DisplayName: srv.DisplayName(),
			Port:        port,
			TLSEnabled:  !*plaintext,
		}, nil)
		if err != nil {
			log.Printf("WARNING: mDNS 自动发现未启用(不影响手动填地址连接): %v", err)
		} else {
			mdnsReg = reg
			log.Printf("mDNS 自动发现已启用: _pocketagent._tcp 端口 %d 主机 %s", port, srv.DisplayName())
		}
	}

	// 优雅退出:Ctrl+C / kill 时先注销 mDNS(发 GOODBYE,局域网内缓存立刻过期),
	// 再停 gRPC。进程被强杀时注销包发不出去,靠 TTL 自然过期(行为兜底,无需处理)。
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Printf("收到退出信号,注销 mDNS 并停止 gRPC ...")
		mdnsReg.Shutdown()
		gs.Stop()
	}()

	// 预热 codex:后台拉起 app-server 并 initialize,MCP/模型列表/首条消息零等待
	if cp, ok := providers["codex"].(*agent.CodexProvider); ok {
		go cp.Warmup()
	}
	// 版本锚点(AUDIT §2 L0 产物自证):启动日志打印 -ldflags 烙印的版本,
	// 确认"跑着的是新的"不再只靠 pid。
	log.Printf("pocketagentd 已启动: %s | 工作区: %s | 模型: %v | 鉴权: %s | 版本: %s",
		*addr, *workspace, names, authMode, version.Version)
	if err := gs.Serve(lis); err != nil {
		log.Fatalf("gRPC 服务退出: %v", err)
	}
	// Serve 正常返回(被 Stop 或监听关闭)时也注销一次,双保险。
	mdnsReg.Shutdown()
}

func cwd() string {
	d, _ := os.Getwd()
	return d
}
