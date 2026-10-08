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
	"github.com/pocketagent/daemon/internal/dshapp"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/hostenv"
	"github.com/pocketagent/daemon/internal/jobobject"
	"github.com/pocketagent/daemon/internal/mdns"
	"github.com/pocketagent/daemon/internal/panel"
	"github.com/pocketagent/daemon/internal/piapp"
	"github.com/pocketagent/daemon/internal/rotatelog"
	"github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/server"
	"github.com/pocketagent/daemon/internal/store"
	"github.com/pocketagent/daemon/internal/term"
	"github.com/pocketagent/daemon/internal/tlscert"
	"github.com/pocketagent/daemon/internal/version"
	"github.com/pocketagent/daemon/internal/zcodeapp"
)

// defaultDataDir 返回 daemon 的默认数据目录。
//
// 决策交给 hostenv：新位置（%LOCALAPPDATA%\PocketAgent\data）已存在 → 用它；
// 否则历史位置（~/.pocketagentd）存在 → 沿用它；两者都无 → 新位置（全新安装）。
//
// 【为什么不自动搬移老数据】目录里有配对的设备 token、自签证书与配对码
// （本机实测 99MB，其中 85MB 是 pi-agent）。自动搬移会同时引入三件危险事：
// daemon 运行中搬、搬一半失败、两边都有数据时以谁为准。「绝不自动删用户数据」
// 这条既有红线在这里延伸为「绝不自动搬用户数据」—— 搬迁由用户显式决定。
//
// 这里只负责在沿用历史位置时把事实打印出来：数据在哪，用户就该知道。
// isLoopbackHost 报告监听地址的 host 部分是否指向本机回环
// (-plaintext 回环硬限制的判定器,SplitHostPort 已剥去 IPv6 方括号):
// localhost(不分大小写)或 127.0.0.0/8、::1 等回环 IP。
func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func main() {
	addr := flag.String("addr", ":7246", "gRPC 监听地址")
	dataDir := flag.String("data", "", "数据目录(线程存储、token)")
	// 【PA-REAL-0001 修复】为空不再等于关闭鉴权:不传=自动生成 8 位随机码并
	// 持久化到 <data>/pair-code.txt,启动日志会打印,手机首次连接时输入。
	pairCode := flag.String("pair-code", "", "配对码(鉴权默认开启);不传=自动生成并持久化到 <数据目录>/pair-code.txt")
	pairOnce := flag.Bool("pair-once", false, "额外生成一枚一次性配对码(15 分钟有效,成功配对即作废;持久码不受影响)")	// 【PA-REAL-0001 修复】显式降级旗标:完全跳过鉴权,但仅允许本机回环连接
	// (127.0.0.1/::1),局域网/远程一律被拒。应急排障用,平时不要开。
	insecureNoAuth := flag.Bool("insecure-no-auth", false, "应急:关闭配对码鉴权,仅允许本机回环连接;局域网连接会被拒绝")
	workspace := flag.String("workspace", cwd(), "默认工作区")
	// 应急回退:P0-1 起默认 TLS(自签证书 + 手机端指纹锁定);
	// 极端情况(证书文件损坏等)可用 -plaintext 暂时回明文。
	plaintext := flag.Bool("plaintext", false, "应急回退:关闭 TLS 走明文 gRPC(仅限本机回环监听;强烈不建议)")
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
	// -collab-max-parallel:并行委派(同时派出的子智能体)全局 / 单设备上限。
	// 有界并发、不排队(排队会让父轮次同步阻塞死锁);超限如实报错由 coordinator
	// 择机重试。0 或不传 = 默认 3(server.SetCollabMaxParallel 把 <=0 归一为 3)。
	collabMaxParallel := flag.Int("collab-max-parallel", 0, "并行委派(子智能体)上限;0=默认3;超出如实报错不排队")
	// -pi-collab:pi 协作开关。置位时初始化 daemon 自有 pi agent 目录
	// (幂等安装 pi-mcp-adapter),并把本 daemon 的 -collab-mcp 子模式写进
	// 该目录的 mcp.json;全部容错,失败不阻塞 daemon 主功能,下次启动重试。
	// env PA_PI_COLLAB 仅供手工调试:flag 不传时不调用 SetCollabEnabled,
	// env 继续生效;flag 显式置位则以 flag 为准。
	// 【默认开启(2026-09-21)】以前默认关,直接后果是"手机端根本派不出子智能体"
	// —— 用户实测报障「派发子智能体的操作会直接失败」,根因正是模型看不见
	// agent_delegate 工具(那条会话的消息数为 0,请求压根没发出去)。多 agent
	// 协同是产品主张,不该要求用户先找到一个隐藏 flag 才能用,故默认置位。
	// 容错面不变:初始化目录 / 写 mcp.json 失败一律只记日志、不阻塞主功能;
	// env 注入另有四道门控(piapp.CollabEnvInjection),目录不存在就不注入,
	// 不会把 pi 的凭据与扩展弄丢。要关掉仍可显式传 -pi-collab=false。
	piCollab := flag.Bool("pi-collab", true, "启用 pi 协作 MCP(默认开):把本 daemon 注册为 pi 的 pocketagent-collab MCP 服务器;-pi-collab=false 可关闭")
	// 本地 Web 管理面板(默认关):仅绑定回环的 HTTP 页,浏览器可见可点的
	// 八域控制面 + 白名单开关(协同/委派/扇出、未知请求透传 B 开关)。写动作
	// 与手机控制中心走同一批 server 方法(同源 RPC);非回环地址 panel.Start
	// 直接拒绝启动。
	panelAddr := flag.String("panel", "", "本地管理面板(默认关):仅回环地址,如 -panel 127.0.0.1:7455")
	flag.Parse()

	resolvedData, resolveErr := hostenv.ResolveDataDir(*dataDir)
	if resolveErr != nil {
		log.Fatalf("解析 daemon 数据目录失败: %v", resolveErr)
	}
	*dataDir = resolvedData.SelectedDir
	switch {
	case resolvedData.Migrated:
		log.Printf("[data] 已将历史数据迁移到 %s（旧目录保留：%s；marker=%s）",
			resolvedData.SelectedDir, hostenv.LegacyDataDir(), resolvedData.MarkerPath)
	case resolvedData.UsedLegacy:
		log.Printf("[data] 使用历史数据目录 %s", resolvedData.SelectedDir)
	default:
		log.Printf("[data] 使用数据目录 %s", resolvedData.SelectedDir)
	}
	piapp.SetDataDir(*dataDir)
	dshapp.SetDataDir(*dataDir)
	zcodeapp.SetDataDir(*dataDir)

	// 并行委派上限(有界并发,不排队):把 -collab-max-parallel 传导进引擎。
	// SetCollabMaxParallel 统一归一化(<=0 → 默认 3),整进程只此一处写入。
	server.SetCollabMaxParallel(*collabMaxParallel)

	// -collab-mcp 子模式:命中即走 MCP 服务器路径,ServeStdio 返回后退出。
	if *collabMCP {
		runCollabMCP(*dataDir, *workspace)
		return
	}

	// 【daemon.log 进程内尺寸轮转】日志经 rotatelog.Writer 落 <cwd>/daemon.log,
	// 超 20MB 改名留一代(daemon.log.1)再续写。为什么做在进程内:daemon 长驻,
	// paguard 只在崩溃/开机时拉起,"拉起前轮转"管不住单次运行内的增长;而
	// paguard 既往只轮转自己的 watchdog.log,分发机器上 daemon.log 无限增长,
	// 磁盘写满会连带打断 store 落库。路径取 cwd:paguard 拉起时 cmd.Dir=exe 目录,
	// 与其 stderr 重定向目标(<exeDir>/daemon.log)是同一个文件 —— 由本 Writer
	// 单点写入,inherited stderr 句柄退役,轮转后不存在"旧句柄写向已改名文件"
	// 的增量(Windows 句柄语义的核实与取舍见 rotatelog 包注释)。
	// 【为什么不做 stderr 镜像】生产 stderr 已被 paguard 重定向到同一文件,镜像
	// 会双写;开发运行看文件即可(与生产同一查看路径)。子模式(-collab-mcp)
	// 走 stdio 协议且 cwd 不可控,明确不装本 Writer(已在上方 return)。
	if lw, lerr := rotatelog.New(filepath.Join(cwd(), "daemon.log"), rotatelog.DefaultMaxBytes); lerr != nil {
		// 打不开文件(只读目录等):退回 stderr,daemon 照常起 —— 日志缺位
		// 不能成为拒绝服务的理由。
		log.Printf("WARNING: daemon.log 打开失败,日志退回 stderr: %v", lerr)
	} else {
		log.SetOutput(lw)
		defer lw.Close()
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
			// ---- dsh 协作 MCP 接线(仅部署级)----
			// 【会话级注入已弃用,不再调用 SetSessionMCPServers】dsh 的 session/new
			// 的 mcpServers 在 dsh 侧"空值接受、非空拒绝"(dsh-acp 文档 + 官方
			// handbook 一致),且 MCP 实例是**部署级 Cordis 插件**、ACP Session 创建
			// 于该固定 Host 组合之后 —— 经 session/new 传非空的 pocketagent-collab
			// 要么被拒(整条 session 建不起)、要么被静默忽略(挂上不生效),两种结果
			// 都不是"真接上"。所以这里**只**走部署级:把插件注册进隔离 HOME 的
			// profile 补丁层(WriteCollabMcpPatch → profiles/acp/cordis.patch.yml),
			// 由 dsh 固定 Host 组合加载。命令/参数/ token 与 pi 同款,都指向本
			// daemon 的 -collab-mcp 子模式。会话级路径退化为恒空数组(与改造前零差异)。
			mcpEnvCopy := map[string]string{}
			if tok := server.LegacyTokenFor(pairCodeValue); tok != "" {
				mcpEnvCopy["POCKETAGENT_TOKEN"] = tok
			}
			if ok, perr := dshapp.WriteCollabMcpPatch("", exe, args, mcpEnvCopy); perr != nil {
				log.Printf("WARNING: 写入 dsh 协作 MCP 补丁失败(dsh 主功能不受影响): %v", perr)
			} else if ok {
				log.Printf("dsh 协作 MCP 已接线(部署级 profile 补丁): %s", dshapp.CollabPatchPath(""))
			} else {
				log.Printf("dsh 协作 MCP 未接线:profile 补丁层已有其他内容,请手动合并 %s", dshapp.CollabPatchPath(""))
			}
		}
	}

	// TLS 凭证:首启自动生成自签证书(ECDSA P-256,20 年),之后一直复用。
	var grpcOpts []grpc.ServerOption
	// listenAddr 是真正交给 net.Listen 的地址:TLS 分支原样用 -addr;明文应急分支
	// 按下方 F2 裁决强制回环。
	listenAddr := *addr
	if *plaintext {
		log.Printf("WARNING: PLAINTEXT MODE ENABLED - gRPC 正在明文传输,token 与会话内容可被同网络者窃听,仅限应急使用!")
		// 【F2 回环硬限制(2026-10-04 裁决,对齐 -insecure-no-auth 先例)】明文应急
		// 只允许回环:token 与会话内容不过 TLS,听非回环等于向局域网/公网裸奔。
		//   - 用户显式给了非回环地址 → 拒绝启动,给人话出路(改用默认 TLS);
		//   - 未显式指地址(默认 ":7246" 全接口)→ 自动压回 127.0.0.1:PORT,应急仍可用。
		// mDNS 广告块仍读 *addr:它只取端口,且明文回环本就不该向局域网宣告,零触碰。
		addrExplicit := false
		flag.Visit(func(f *flag.Flag) {
			if f.Name == "addr" {
				addrExplicit = true
			}
		})
		if host, port, splitErr := net.SplitHostPort(listenAddr); splitErr != nil {
			log.Fatalf("-plaintext 应急明文要求 host:port 形式的监听地址,当前 %q 无法解析: %v", listenAddr, splitErr)
		} else if !isLoopbackHost(host) {
			if addrExplicit {
				log.Fatalf("-plaintext 应急明文仅允许回环;局域网/公网明文监听被禁止——请改用默认 TLS")
			}
			listenAddr = net.JoinHostPort("127.0.0.1", port)
			log.Printf("WARNING: -plaintext 明文应急未显式指地址,监听已从 %q 压回回环 %s(明文只允许本机)", *addr, listenAddr)
		}
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

	// W6：运行时状态（checkpoint）要落到磁盘，而不是只在内存里活着（规格 §8.2）。
	// 必须在 provider 注册之前指定 —— 共享后端是惰性单例，首次交付运行时即定型。
	agent.SetRuntimeDataDir(*dataDir)

	// W7：装载资源预算（规格 §6.5 的逐级覆盖）。配置坏掉不让 daemon 起不来，
	// 但必须出声 —— 静默用零值等于把所有资源一起封禁。
	budgetPolicy, budgetErr := runtime.LoadBudgetPolicy(*dataDir)
	if budgetErr != nil {
		log.Printf("[budget] WARNING: %v（已回退内置基线）", budgetErr)
	}
	{
		lim, origins := budgetPolicy.EffectiveWithSource("", "", "", "")
		log.Printf("[budget] 生效预算(全局): ram_working=%dMB event_buffer=%dMB session_index=%dMB disk_conversation=%dMB disk_cache=%dMB disk_artifact=%dMB max_parallel_turns=%d",
			*lim.RAMWorkingBytes>>20, *lim.RAMEventBufferBytes>>20, *lim.RAMSessionIndexBytes>>20,
			*lim.DiskConversationBytes>>20, *lim.DiskCacheBytes>>20, *lim.DiskArtifactBytes>>20,
			*lim.MaxParallelTurns)
		log.Printf("[budget] 溯源: ram_working←%s max_parallel_turns←%s（配置 %s）",
			origins["RAMWorkingBytes"], origins["MaxParallelTurns"],
			runtime.BudgetPolicyPath(*dataDir))
	}
	// 【DiskConversationBytes 接通】threads.archive 旁车归档目录的总字节上限取
	// 预算策略全局级的 disk_conversation 值(未设置 → 维持 store 内置默认 32MB)。
	// 此前该字段只是配置口径,全仓无执行点(假预算);这里把它接到唯一真实的
	// 执行体 —— store 的归档总量守护(见 store_archive.go evictArchiveLocked)。
	if lim := budgetPolicy.Effective("", "", "", ""); lim.DiskConversationBytes != nil {
		st.SetArchiveMaxBytes(*lim.DiskConversationBytes)
		log.Printf("[budget] disk_conversation 已执行: threads.archive 总量上限 = %dMB", *lim.DiskConversationBytes>>20)
	}

	// W10：装载协同策略（规格 §11）。与预算同款取舍：配置坏掉不让 daemon 起不来
	// （它不是安全边界），但必须出声 —— 静默回退会让"我明明关过协同"变成灵异事件。
	collabPolicy, collabErr := runtime.LoadCollabPolicy(*dataDir)
	if collabErr != nil {
		log.Printf("[collab] WARNING: %v（已回退内置默认：协同/委派/扇出全开）", collabErr)
	}
	log.Printf("[collab] 生效策略: collaboration=%v delegate=%v fanout=%v max_parallel_members=%d（配置 %s）",
		collabPolicy.CollabEnabled(), collabPolicy.DelegateEnabled(), collabPolicy.Fanout(),
		collabPolicy.MaxMembers(), runtime.CollabPolicyPath(*dataDir))
	// Job Object 兜底(用户红线「结束时不应残留任务和后台进程」):把自己与
	// 之后拉起的全部子进程绑进 Kill-on-Close Job —— daemon 无论怎么死
	// (优雅退出/taskkill /F/崩溃),内核保证 codex/dsh/pi 子进程一起带走,
	// 热部署不再残留孤儿。必须绑在任何 agent 子进程拉起之前(见包注释):
	// 下面 DetectProviders 里的 pi --version 探测就是本进程第一批子进程,
	// 顺序不能反。绑定失败只记日志不阻塞启动 —— 兜底层缺席时退回优雅退出+手动收尸。
	if _, err := jobobject.Assign(); err != nil {
		log.Printf("WARNING: Job Object 绑定失败(退出时可能残留 agent 子进程,不影响其他功能): %v", err)
	} else {
		log.Printf("Job Object 已绑定:本进程退出时全部 agent 子进程随之终止")
	}
	providers := agent.DetectProviders()
	// 历史崩溃/taskkill 残留在 %TEMP% 的 codex 图片临时文件,启动对账清扫一次
	// (只清早于本次启动的;在役文件不会被误删)。
	agent.SweepStaleImageTemps()
	tm := term.NewManager()
	// 空闲回收器:随进程存活,daemon 退出时随进程消亡,无需单独停
	tm.SetIdleTimeout(*termIdle)
	tm.StartReaper(context.Background())
	if *termIdle > 0 {
		log.Printf("终端空闲回收已启用: 无 reader 闲置 %s 自动关闭会话", *termIdle)
	}

	// 一次性配对码(2026-09-30):-pair-once 时额外生成一枚短时码,日志醒目输出。
	// 面向分发场景:当面给新设备一个 15 分钟码,成功配对即作废;持久码不变。
	var oneShotCodes []string
	if *pairOnce {
		if c, err := generatePairCode(); err == nil {
			oneShotCodes = append(oneShotCodes, c)
			log.Printf("============================================================")
			log.Printf("一次性配对码: %s", c)
			log.Printf("15 分钟内有效,成功配对即作废;持久配对码(%s)不受影响。", pairCodeValue)
			log.Printf("============================================================")
		} else {
			log.Printf("WARNING: 一次性配对码生成失败:%v(不影响持久配对码)", err)
		}
	}

	srv := server.New(server.Config{
		Store:          st,
		Providers:      providers,
		Terminals:      tm,
		PairCode:       pairCodeValue,
		OneShotPairCodes: oneShotCodes,
		InsecureNoAuth: *insecureNoAuth,
		Workspace:      *workspace,
		DataDir:        *dataDir,
		// W7:把启动时装载的预算策略交给 server —— 它此后是策略的持有者,
		// RuntimeService 可就地修改并立即生效(不必重启 daemon)。
		BudgetPolicy: budgetPolicy,
		// W10:协同策略(规格 §11)同理:daemon 持久化并作为唯一事实来源,
		// 手机上的本地开关不再是权限判据。
		CollabPolicy: collabPolicy,
		// W7:轮次强制点要用与各 provider 同一份资源账(见 agent.SharedRuntimeBackend)。
		RuntimeBackend: agent.SharedRuntimeBackend(),
	})

	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalf("监听 %s 失败: %v", listenAddr, err)
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
	// W7:本机运行时控制面(读/改预算策略、看资源用量)。只增 —— 旧 APK 不调用它,
	// 新 APK 先查 HostInfo.capabilities 里的 runtime.v1 才显示入口(规格 §21.3)。
	pb.RegisterRuntimeServiceServer(gs, srv)

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

	// 本地 Web 管理面板(默认关):进程内直调自己的 server 方法(与手机同一批
	// RPC、同一把锁、同一份落盘),仅回环监听 + Host 守卫,页面不含任何
	// token/密钥。非回环地址在 panel.Start 里被拒,这里如实 Fatal —— 红线不降级。
	if strings.TrimSpace(*panelAddr) != "" {
		panelURL, stopPanel, err := panel.Start(*panelAddr, srv)
		if err != nil {
			log.Fatalf("管理面板启动失败(仅允许回环地址,如 -panel 127.0.0.1:7455): %v", err)
		}
		defer stopPanel()
		log.Printf("管理面板已开启: %s(仅本机回环可访问;页面不含 token/密钥;每 10 秒自动刷新)", panelURL)
	}

	// 优雅退出:Ctrl+C / kill 时先注销 mDNS(发 GOODBYE,局域网内缓存立刻过期),
	// 再停 gRPC。进程被强杀时注销包发不出去,靠 TTL 自然过期(行为兜底,无需处理)。
	// 【二次信号放行】首个信号后 signal.Stop 交还默认处置:gs.Stop 若因某个
	// 连接/收尾卡住,再次 Ctrl+C 直接按默认方式终止进程,不再被同一个 goroutine
	// 吞掉 —— 优雅退出是礼遇,不是人质。
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Printf("收到退出信号,注销 mDNS 并停止 gRPC ...(再次 Ctrl+C 立即强杀)")
		signal.Stop(sigCh)
		mdnsReg.Shutdown()
		gs.Stop()
	}()

	// 预热 codex:后台拉起 app-server 并 initialize,MCP/模型列表/首条消息零等待
	if cp, ok := providers["codex"].(*agent.CodexProvider); ok {
		go cp.Warmup()
	}
	// 预热 dsh(T-DSH 问题 B):dsh 首跑的 initialize 握手实测可拖到十位数分钟,
	// 懒启动把这笔开销留给首个用户请求,还让所有并发调用 busy-reject("dsh 正忙")。
	// 启动后台预热一次(内部带重试窗口),握手成功后用户路径就绪检查瞬时通过。
	if dp, ok := providers["dsh"].(*agent.DshProvider); ok {
		go dp.Warmup()
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
