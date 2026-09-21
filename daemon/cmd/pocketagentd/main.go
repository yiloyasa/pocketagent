package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	"github.com/pocketagent/daemon/internal/agent"
	"github.com/pocketagent/daemon/internal/collabmcp"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/mdns"
	"github.com/pocketagent/daemon/internal/piapp"
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
	// T-L2wire 跨 agent 委派桥(T-L2fix 单写者重构):
	// -collab-mcp 子模式:本进程作为 "pocketagent-collab" 本地 MCP 服务器
	// 经 stdio 运行(pi 经 pi-mcp-adapter 拉起、codex/dsh 各自接入),提供
	// agent_delegate / agent_status 两个工具。【薄代理】子模式不再自持
	// FileStore/providers/引擎,agent_delegate 经回环 gRPC 调主 daemon 的
	// DelegateSubTask RPC,委派收归主 daemon 单一引擎实例 —— 子代理进程
	// 不触碰 threads.json/events.jsonl(写冲突 P1 关闭),深度锁对全部代理
	// 进程全局成立(子生子 P1 关闭)。对端关闭或收到信号即退出,不起
	// gRPC 监听/mDNS。
	collabMCP := flag.Bool("collab-mcp", false, "子模式:作为 pocketagent-collab 本地 MCP 服务器运行(stdio 薄代理,委派经回环 gRPC 汇入主 daemon);对端关闭即退出")
	// -pi-collab:pi 协作开关。置位时初始化 daemon 自有 pi agent 目录
	// (幂等安装 pi-mcp-adapter),并把本 daemon 的 -collab-mcp 子模式写进
	// 该目录的 mcp.json;全部容错,失败不阻塞 daemon 主功能,下次启动重试。
	// env PA_PI_COLLAB 仅供手工调试:flag 不传时不调用 SetCollabEnabled,
	// env 继续生效;flag 显式置位则以 flag 为准。
	piCollab := flag.Bool("pi-collab", false, "启用 pi 协作 MCP:初始化 daemon 自有 pi agent 目录并把本 daemon(-collab-mcp 子模式)注册为其 pocketagent-collab MCP 服务器")
	flag.Parse()

	// -collab-mcp 子模式:命中即走 MCP 服务器路径,ServeStdio 返回后退出。
	if *collabMCP {
		runCollabMCP(*dataDir, *workspace)
		return
	}

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

	// ---- pi 协作 MCP 接线(-pi-collab,T-L2wire;仅正常启动路径)----
	// 三个调用全部容错:协作功能初始化失败绝不阻塞 daemon 主功能。
	// 写入的命令是 os.Executable()(当前进程可执行文件),参数指向上面的
	// -collab-mcp 子模式 —— pi 拉起的就是本 daemon 自己。
	// 【T-L2fix】须在鉴权模式解析之后(pairCodeValue 在手):
	//   - args 带 "-data":代理进程与主 daemon 共享数据目录 —— 薄代理模式
	//     下它是 TLS 证书与配对码回退的取数前提(见 runCollabMCP);
	//   - env 注入 POCKETAGENT_TOKEN = server.LegacyTokenFor(配对码)(只读
	//     helper,值与 authorized → devices.Validate 的 legacy 回退链所认的
	//     完全一致),代理进程拿它作 Bearer 回环委派;-insecure-no-auth 下
	//     无配对码可导出 → 不注入 env(回环连接该模式下本就放行)。
	//     【已知边界】internal/agent 的 ensureCollabMcp(pi 会话路径,T-FU)
	//     会以裸参数重写同一条目把 env 覆盖掉 —— 子模式侧另有 pair-code.txt
	//     回退链兜住(见 collabProxyToken),两条链取的是同一个 legacy token。
	if *piCollab {
		piapp.SetCollabEnabled(true)
		if err := piapp.SetupCollabAgentDir(piapp.ResolveCollabAgentDir()); err != nil {
			log.Printf("WARNING: pi 协作目录初始化失败(pi 主功能不受影响,下次启动重试): %v", err)
		}
		exe, exeErr := os.Executable()
		if exeErr != nil {
			log.Printf("WARNING: 无法定位当前可执行文件,跳过 pi 协作 MCP 配置写入: %v", exeErr)
		} else {
			env := map[string]string{}
			if tok := server.LegacyTokenFor(pairCodeValue); tok != "" {
				env["POCKETAGENT_TOKEN"] = tok
			}
			args := []string{"-collab-mcp", "-data", *dataDir}
			if err := piapp.WriteCollabMcpConfigWithEnv(piapp.ResolveCollabAgentDir(), exe, args, env); err != nil {
				log.Printf("WARNING: 写入 pi 协作 MCP 配置失败(pi 主功能不受影响): %v", err)
			} else {
				log.Printf("pi 协作 MCP 已启用: agent 目录 %s | 服务器 %s %v | env 注入 %d 项",
					piapp.ResolveCollabAgentDir(), exe, args, len(env))
			}
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

// collabDaemonAddrDefault 子模式回环拨号的主 daemon 默认地址(T-L2fix:委派
// 收归主 daemon 单引擎;固定本机回环,委派面不暴露给网络)。
const collabDaemonAddrDefault = "127.0.0.1:7246"

// collabDaemonAddr 解析主 daemon 地址:默认 127.0.0.1:7246;env
// POCKETAGENT_DAEMON_ADDR 可覆盖 —— 供同机多 daemon/集成测试环境把代理指向
// 另一个回环端口(生产单 daemon 场景无需设置,行为与固定值完全一致)。
func collabDaemonAddr() string {
	if a := strings.TrimSpace(os.Getenv("POCKETAGENT_DAEMON_ADDR")); a != "" {
		return a
	}
	return collabDaemonAddrDefault
}

// runCollabMCP 以 pocketagent-collab 本地 MCP 服务器模式运行(-collab-mcp
// 子模式):coordinator agent(pi 经 pi-mcp-adapter、codex/dsh 各自接入)
// 把本进程当一条 stdio 本地 MCP 子进程挂载,经 agent_delegate / agent_status
// 工具做跨 agent 委派。
//
// 【T-L2fix 单写者重构:薄代理】与旧实现的本质差异 —— 本进程不再自持
// FileStore/providers/委派引擎(那是两个 P1 的根源:子进程与主 daemon 双写
// threads.json/events.jsonl 互相抹掉对方;子 pi 再拉起的全新进程各自 depth=0
// 击穿深度锁)。工具面(collabmcp 包)原样保留,引擎换成
// collabmcp.GRPCDelegationEngine:agent_delegate 经回环 gRPC
// (collabDaemonAddr,TLS + Bearer token)调主 daemon 的 DelegateSubTask RPC,
// 委派汇入主 daemon 的单一引擎实例。子进程唯一触碰的文件是主 daemon 的
// 证书与 pair-code.txt(只读),线程与事件数据一律不碰。
//
// 进程模型不变:不起 gRPC 监听、不起 mDNS、不预热 codex;阻塞到对端关闭
// stdin 或收到退出信号,然后进程退出。
func runCollabMCP(dataDir, workspace string) {
	token, tokenSrc := collabProxyToken(dataDir)
	conn, err := collabDaemonConn(dataDir)
	if err != nil {
		log.Fatalf("连接主 daemon 失败: %v", err)
	}
	defer conn.Close()
	authDesc := "无 token(配对码模式下委派会被主 daemon 拒绝;-insecure-no-auth 回环放行)"
	switch tokenSrc {
	case tokenSourceEnv:
		authDesc = "POCKETAGENT_TOKEN(env)已注入"
	case tokenSourcePairCodeFile:
		authDesc = "legacy token(pair-code.txt 回退)已就绪"
	}
	log.Printf("pocketagent-collab MCP 服务器已就绪(stdio 薄代理): 主daemon=%s | 数据目录: %s | 鉴权: %s | 版本: %s",
		collabDaemonAddr(), dataDir, authDesc, version.Version)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := collabmcp.New(collabmcp.NewGRPCEngine(pb.NewThreadServiceClient(conn), token, workspace), version.Version).ServeStdio(ctx); err != nil {
		log.Fatalf("collab MCP 服务器退出: %v", err)
	}
}

// collabDaemonConn 建立到主 daemon 的回环 gRPC 连接(TLS 优先):
// 主 daemon 首启生成自签证书(<数据目录>/certs/server-cert.pem,SAN 含
// 127.0.0.1),子模式与主 daemon 共享数据目录,拿同一张证书做 RootCA 精确
// 校验 —— 与手机端指纹锁定同源的信任锚,不引入"跳过校验"的弱配置。
// 证书文件不存在 → 视为主 daemon 以 -plaintext 应急运行,回明文(同样仅
// 本机回环,不经网络)。grpc.NewClient 惰性连接:主 daemon 未起时本子进程
// 仍能就绪,首次委派时才报连接错误(如实透传给 coordinator)。
func collabDaemonConn(dataDir string) (*grpc.ClientConn, error) {
	addr := collabDaemonAddr()
	host, _, splitErr := net.SplitHostPort(addr)
	if splitErr != nil {
		host = addr
	}
	certPath, _ := tlscert.Paths(dataDir)
	var creds credentials.TransportCredentials
	if raw, err := os.ReadFile(certPath); err == nil {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(raw) {
			return nil, fmt.Errorf("主 daemon TLS 证书无法解析: %s", certPath)
		}
		creds = credentials.NewTLS(&tls.Config{
			RootCAs:    pool,
			ServerName: host, // 回环自签证书 SAN 覆盖 127.0.0.1/::1/localhost
			MinVersion: tls.VersionTLS12,
		})
	} else {
		log.Printf("WARNING: 未找到主 daemon TLS 证书(%s),按明文回环连接(主 daemon 需以 -plaintext 运行)", certPath)
		creds = insecure.NewCredentials()
	}
	return grpc.NewClient(addr, grpc.WithTransportCredentials(creds))
}

// collabProxyToken 子模式回环鉴权的 Bearer token(T-L2fix)。两条链取的是
// 同一个值 —— server.LegacyTokenFor(配对码)= sha256("pair-code:"+码) 的
// hex,即主 daemon devices.Validate 的 legacy 回退所认的单 token:
//  1. 优先 mcp.json env 注入的 POCKETAGENT_TOKEN(-pi-collab 写入,T-06
//     legacy 回退链认它);
//  2. 回退:从共享数据目录的 pair-code.txt 现算同一 token —— 兜住
//     internal/agent 的 ensureCollabMcp(T-FU)以裸参数重写 mcp.json、
//     env 被覆盖的场景(代理进程与主 daemon 同机同用户,读自己数据目录的
//     配对码做本地回环鉴权,不越权)。
//
// 都拿不到返回空串:配对码模式下委派会被主 daemon 如实拒绝
// (Unauthenticated),错误原样透传给 coordinator —— 不伪造鉴权、不静默降级。
func collabProxyToken(dataDir string) (token string, src int) {
	if tok := strings.TrimSpace(os.Getenv("POCKETAGENT_TOKEN")); tok != "" {
		return tok, tokenSourceEnv
	}
	raw, err := os.ReadFile(pairCodePath(dataDir))
	if err != nil {
		return "", tokenSourceNone
	}
	if tok := server.LegacyTokenFor(strings.TrimSpace(string(raw))); tok != "" {
		return tok, tokenSourcePairCodeFile
	}
	return "", tokenSourceNone
}

// token 来源标记(仅用于启动日志如实说明,不参与鉴权逻辑)。
const (
	tokenSourceNone         = iota
	tokenSourceEnv          // mcp.json env 注入的 POCKETAGENT_TOKEN
	tokenSourcePairCodeFile // pair-code.txt 现算的 legacy token
)
