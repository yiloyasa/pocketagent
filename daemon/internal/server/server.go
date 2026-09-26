// Package server 实现全部 gRPC 服务 + 鉴权拦截器。
//
// 【讲解】gRPC 的"拦截器"类似 Web 框架的中间件:每个请求进来先过它。
// 我们在 metadata(可以理解为 HTTP 的 header)里找 "authorization: Bearer xxx"。
// 【PA-REAL-0001 修复后】鉴权默认开启:入口(main)在没给 -pair-code 时自动
// 生成配对码并持久化;显式降级走 -insecure-no-auth,且仅放行本机回环连接;
// 配对码为空的异常配置一律 fail closed,绝不静默放行。
package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/netip"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/pocketagent/daemon/internal/agent"
	"github.com/pocketagent/daemon/internal/eventstore"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	// 本包同时用到标准库 runtime(见上方 import),故内部运行时包取别名。
	rtruntime "github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/store"
	"github.com/pocketagent/daemon/internal/term"
)

type Config struct {
	Store     *store.FileStore
	Providers map[string]agent.Provider
	Terminals *term.Manager
	// PairCode 是生效配对码。【PA-REAL-0001 修复后】正常启动路径(main)保证
	// 非空:未显式给 -pair-code 时自动生成并持久化。为空=异常配置,拦截器
	// fail closed,不再有"空码即关闭鉴权"的旧语义。
	PairCode string
	// InsecureNoAuth 对应入口旗标 -insecure-no-auth:显式关闭鉴权,但仅放行
	// 本机回环(peer 地址为 127.0.0.1/::1)连接,局域网/远程一律拒绝。
	InsecureNoAuth bool
	Workspace      string
	DataDir        string
	// BudgetPolicy 是启动时装载的资源预算策略(W7,规格 §6.5)。零值 = 全部走内置基线
	// (LoadBudgetPolicy 在缺文件时也回这份基线)。运行中可经 RuntimeService 热替换。
	BudgetPolicy rtruntime.BudgetPolicy
	// RuntimeBackend 是进程共享的运行时后端(记忆/缓存/checkpoint/资源记账,W6 起由
	// agent 包持有同一实例)。为 nil 时不记账 —— 测试与极简构造路径照常跑,强制点跳过。
	RuntimeBackend *rtruntime.Backend
	// MemorySummarizer 是可选的语义摘要器(P2,memory 分层设计稿 §4)。非 nil 且
	// RuntimeBackend 就位时,New 装配期注入进程共享后端,Compact 从此用语义摘要
	// (失败自动回退结构摘要);缺省 nil = 全部走结构摘要,P1 行为零分叉。
	// 实现自带超时与重试(契约见 runtime.Summarizer),daemon 不替它兜底。
	MemorySummarizer rtruntime.Summarizer
	// CollabPolicy 是启动时装载的协同策略(W10,规格 §11)。零值 = 全部走内置默认
	// (与既有行为一致:全开、扇出不限)。运行中可经 RuntimeService 热替换。
	CollabPolicy rtruntime.CollabPolicy
}

type Server struct {
	pb.UnimplementedHostServiceServer
	pb.UnimplementedThreadServiceServer
	pb.UnimplementedTerminalServiceServer
	pb.UnimplementedFileSystemServiceServer
	pb.UnimplementedGitServiceServer
	pb.UnimplementedModelServiceServer
	pb.UnimplementedRuntimeServiceServer

	cfg              Config
	prefsMu          sync.Mutex
	approvalSessions sync.Map // threadID -> *turnApprovals
	// probeOverride 是 codex 探测的测试注入点(生产为 nil,走真实 provider)。
	// 用于确定性回归:"探测不可用时,保存仍返回成功且值等于落盘值"。
	probeOverride codexProbe
	// devices 是设备注册表(P0-2):每设备 token、挑战应答、撤销;
	// 旧单 token 走它内部的 legacy 回退路径。
	devices *deviceRegistry
	// hostID 进程生命周期内稳定(P1 持久化)
	hostID string
	// 每线程正在执行的 turn(重入打断用)
	activeTurns *activeTurns
	// P1-4:运行中继续发消息的内存队列(上限 10 条/线程;队列本身重启即丢,见
	// turn_queue.go)。丢的是什么由下面的 turnJournal 留痕,不再静默。
	turnQueue *turnInputQueue
	// P1-4 增强:排队输入的持久化台账(规格 §8.3)——让重启造成的"静默丢失"
	// 可见。空 DataDir 退化为内存,行为零分叉(与 eventstore/identity 同款约定)。
	turnJournal *turnJournal
	// P1-5:事件持久化(~/.pocketagentd/events.jsonl)。发给手机的每一帧与
	// 主机事件都带全局单调 seq 落盘,断线重连后凭 after_sequence 增量补帧。
	// DataDir 为空(测试)时是纯内存计数器,行为零分叉。
	events *eventstore.Store
	// P1-5:WatchEvents 活跃流注册表(主机事件实时广播用)
	hostWatch *hostWatchers
	// syncCodexInFlight 防止 codex 不可用时重复堆积同步:同一时刻只跑一个
	// syncThreadsFromCodex。否则每次 refresh 都新起一个,会在 withClient 的
	// 生命周期锁上排队等待(最坏等 codex 的 callTimeout=120s)。
	syncCodexInFlight atomic.Bool
	// recoveryReport 是启动时汇总的数据源健康度报告(见 recovery_report.go)。
	// 在 New 里一次性生成,之后只读;由导出的 RecoveryReport() 提供,供将来的
	// GetHostInfo / 诊断 RPC 消费 —— 让"这次启动干净不干净"不必再靠翻日志。
	recoveryReport RecoveryReport
	// W7:资源预算策略(规格 §6.5)。启动时装载,运行中可经 RuntimeService 热替换;
	// 所有读取都从这份内存态取,所以"改完立即生效"不需要重启 daemon。
	budgetMu     sync.RWMutex
	budgetPolicy rtruntime.BudgetPolicy
	// runtimeBackend 是进程共享的运行时后端(与各 provider 用的是同一个实例)。
	// 轮次强制点向它的 ResourceLedger 申请租约 —— 只有共用同一个后端,记的账才是
	// 同一份账;各持一份会让"用量"这件事出现多个互相矛盾的版本。
	runtimeBackend *rtruntime.Backend
	// W10:协同策略(规格 §11)。同为热替换:读取方每次都从这份内存态取,
	// 改完立即生效(不必重启 daemon)。
	collabMu     sync.RWMutex
	collabPolicy rtruntime.CollabPolicy
	// W9:启动期对每个已注册 provider 跑的运行时形态申报核验(规格 §10)。
	// 启动时生成、之后只读 —— 它反映的是"接入形态",不随运行变化。
	contractReports []rtruntime.ContractReport
	// startedAt 是本 daemon 的启动时刻(控制面总览 GetControlPlane 的 uptime 挂点)。
	// 【为什么挂在 New 而不是 main】New 是所有装配路径(生产 main、全部测试)的
	// 必经点,在这里取时刻,测试与生产同源;与进程真正启动的差只有毫秒级初始化,
	// 对"已运行多久"的展示毫无影响。
	startedAt time.Time
	// P3 闲时整理(memory 分层设计稿 §4):lastTurnSettledAt 是最近一次轮次收尾时刻
	// (unix nano,闲时判定的锚点,New 时置为启动时刻——启动即算"刚忙过");后两者是
	// 整理循环的惰性启动(首个轮次才起 goroutine)与停止开关。状态与实现见
	// memory_housekeeping.go。
	lastTurnSettledAt atomic.Int64
	idleSweepOnce     sync.Once
	idleSweepStop     context.CancelFunc
}

func New(cfg Config) *Server {
	// P1-5:事件存储打开失败不致命 —— Open 内部对文件层错误有降级,
	// 这里再兜一层:实在建不起来就用内存模式,daemon 其余功能照常。
	events, err := eventstore.Open(cfg.DataDir)
	if err != nil {
		log.Printf("[eventseq] 事件存储初始化失败,退回内存模式: %v", err)
		events, _ = eventstore.Open("")
	}
	// P1-4 增强:排队输入的持久化台账(规格 §8.3)。与内存队列共享同一实例:
	// 入队记一条、出队划掉、启动报告未划掉的。
	turnJournal := newTurnJournal(cfg.DataDir)
	srv := &Server{
		cfg:            cfg,
		devices:        newDeviceRegistry(cfg.DataDir, cfg.PairCode),
		hostID:         hostIDFromInstallation(cfg.DataDir),
		activeTurns:    newActiveTurns(),
		turnQueue:      newTurnInputQueue(turnJournal),
		turnJournal:    turnJournal,
		events:         events,
		hostWatch:      newHostWatchers(),
		budgetPolicy:   cfg.BudgetPolicy,
		runtimeBackend: cfg.RuntimeBackend,
		collabPolicy:   cfg.CollabPolicy,
		startedAt:      time.Now(),
	}
	// P3:闲时钟起点 = 启动时刻。没有它,零值时间戳会把"空闲了多久"算成五十五年,
	// daemon 一启动就会挨一记不该有的整理。
	srv.lastTurnSettledAt.Store(time.Now().UnixNano())
	// P2:语义摘要器装配注入。后端是进程共享实例,New 一次注入全进程生效;nil 缺省
	// 静默跳过(结构摘要,与 P1 行为零分叉)。放在装配期而不是首个 Compact 前,是让
	// "有没有语义摘要"在启动那一刻就定死,不随运行期状态漂移。
	if cfg.RuntimeBackend != nil && cfg.MemorySummarizer != nil {
		cfg.RuntimeBackend.SetSummarizer(cfg.MemorySummarizer)
	}
	// 启动报告:上次退出时还没轮到执行的排队输入 —— 醒目汇总 + 逐条明细,然后
	// 清空台账。刻意不自动重发(重放原始 prompt 会重复执行副作用,规格 §8.3),
	// 只把"丢了几条、丢在哪些会话、丢的是哪句"讲清楚,要不要重发由用户决定。
	// 它的返回值(本次丢掉的条目)一并接进下面的启动恢复报告,不重复读台账。
	pendingQueueInputs := srv.reportPendingQueueInputs()
	// 启动恢复报告:把各数据源的加载来源/健康度汇总成一份机器可读报告并打一条
	// 醒目日志,同时留在 Server 上(RecoveryReport() 导出)。保持既有启动日志不变,
	// 这里只新增一行汇总。
	srv.recoveryReport = buildRecoveryReport(cfg.DataDir, cfg.Store, srv.devices, pendingQueueInputs)
	log.Printf("[recovery] %s", srv.recoveryReport.LogLine())
	// W9:运行时形态核验(规格 §10)。只做零成本的**声明层**校验 —— 实现层要先
	// OpenRuntime,而那可能起子进程,不该在启动/诊断路径被触发。不达标者打日志出声,
	// 并经 GetRuntimeState 送到手机。
	srv.contractReports = verifyProviderDeclarations(cfg.Providers)
	logContractReports(srv.contractReports)
	// 存量修复:旧版本一直没实现自动标题,启动时把仍是"新会话…"默认名的
	// 线程改成各自首条用户消息(已有真实标题的不动)
	for _, t := range cfg.Store.List(true) {
		if !strings.HasPrefix(t.Title, "新会话") {
			continue
		}
		for _, m := range t.Messages {
			if m.Role != "user" {
				continue
			}
			first := ""
			// 【vet】按下标取址遍历,避免 range 复制 pb 消息(内嵌 MessageState 含锁)。
			for i := range m.Blocks {
				b := &m.Blocks[i]
				if tb, ok := b.Block.(*pb.ContentBlock_Text); ok {
					first = strings.TrimSpace(tb.Text.Text)
					break
				}
			}
			if first == "" {
				continue
			}
			r := []rune(first)
			if len(r) > 24 {
				r = r[:24]
			}
			t.Title = string(r)
			logStoreUpdateErr("startup-autotitle", t.ID, cfg.Store.Update(t))
			break
		}
	}
	// codex 自动给线程起名 → 同步到我们的存储(手机/桌面双端一致)
	if p, ok := cfg.Providers["codex"].(*agent.CodexProvider); ok {
		p.TitleHandler = func(codexTID, title string) {
			for _, t := range cfg.Store.List(true) {
				if t.CodexThreadID == codexTID && t.Title != title {
					t.Title = title
					logStoreUpdateErr("codex-title-sync", t.ID, cfg.Store.Update(t))
				}
			}
		}
		// codex 是 goal 的事实来源:它会规范化文本、也会自己推进状态机
		// (active → complete / blocked / usageLimited / budgetLimited)。
		// 把这些变化写回存储,手机端才能看到"目标已完成"而不是停在设下去的那一刻。
		p.GoalHandler = func(codexTID, goal, status string, tokensUsed, timeUsedSeconds int64) {
			for _, t := range cfg.Store.List(true) {
				if t.CodexThreadID != codexTID {
					continue
				}
				if t.Goal == goal {
					continue
				}
				t.Goal = goal
				logStoreUpdateErr("codex-goal-sync", t.ID, cfg.Store.Update(t))
				log.Printf("[goal] 同步会话 %s: goal=%.60q status=%s tokens=%d time=%ds",
					t.ID, goal, status, tokensUsed, timeUsedSeconds)
			}
		}
		// 会话级事实(归档/取消归档/删除/改名/状态)由 codex 发起时同步回本地库。
		// 这一条以前不存在 —— 于是"电脑上归档/删掉的会话,手机上还是活的"。
		// 注意 SyncThreadEvent 会在**独立 goroutine** 里被调用(常驻钩子跑在
		// codexapp 的读循环里,绝不能阻塞它),所以它只碰 store,不碰网络。
		p.ThreadEventHandler = func(codexTID, method string, params json.RawMessage) {
			srv.SyncThreadEvent(codexTID, method, params)
		}
	}
	// 启动灌入:把已持久化的偏好广播给**所有**实现了 PrefsAware 的 provider
	// (codex / pi / zcode),与 SetCodexSettings 的广播共用 DesktopPrefsFromStore
	// 这一份值来源。【为什么必须做】偏好以前只在内存;现在落盘后,daemon 重启会
	// 拿到一份"空的" provider。不灌回的话,重启后 pi 的 HideReasoning/桌面默认
	// 模型、zcode 的默认思考强度、codex 的 turn/start 偏好就都不会再生效 ——
	// 用户看到的就是"改了又变回去"(codex 当年同款问题的根治,推广到全家)。
	srv.broadcastPrefs(agent.DesktopPrefsFromStore(cfg.Store))
	// ZCode 标题同步:session.titleUpdated → 落库(与 codex 同语义,手机/桌面
	// 双端标题一致)。zcode 回调带的是我们自己的线程 ID(见 zcode.go),直接按
	// t.ID 落库即可,不需要 codex 那样的外部 id 映射。规则(含"用户手动改名的
	// 线程不回退",T-05)收在 applyZcodeTitle。
	if zp, ok := cfg.Providers["zcode"].(*agent.ZcodeProvider); ok {
		zp.TitleHandler = func(threadID, title string) {
			applyZcodeTitle(cfg.Store, threadID, title)
		}
		// 【子代理原生事实】0.16.9 session/subagents:provider 在轮次终局拉到
		// 原生子代理清单,这里接住落成存根子线程(幂等、失败只留痕),手机端
		// 既有子代理卡管线按 parentThreadId 过滤即点亮(见 zcode_subagent_sync.go)。
		zp.SubagentsHandler = srv.registerZcodeSubagentStubs
	}
	// Pi 标题同步:session_info_changed → 落库(与 zcode 同语义;实现见 thread.go)。
	srv.wirePiTitleHandler()
	// Resolve legacy RUNNING records before list synchronization can hide their
	// terminal error. This consults native journals only, never launches a turn.
	// 【轮次起点】仍处于 RUNNING 的会话由 backfillRestartedRunningStamp 回填
	// 重启时刻 —— 真实开始时刻不可追溯(见该方法注释)。
	srv.backfillRestartedRunningStamp(time.Now())
	return srv
}

// reportPendingQueueInputs 启动时报告"上次退出时还没轮到执行的排队输入",
// 报告完即清空台账。
//
// 【为什么不自动重发】规格 §8.3:不能自动重新发送原始 prompt —— 那些输入可能
// 在别处已产生过副作用,重放等于执行两遍。这里只把它们"讲清楚"让用户知情,
// 要不要重发由用户在手机上自己决定,绝不替用户做这个决定。
// 【为什么报告后清空】不保留成僵尸条目:每条都在启动时报告过一次,留着会在
// 每次重启反复报同一批,反而稀释了信号。
// 【为什么返回本次读出的条目】启动恢复报告(recovery_report.go)需要把"丢了几条、
// 丢在哪些会话"接进去;与其再读一次台账(第二份真值),不如把 TakePending 的结果
// 原样返回给调用方复用。
func (s *Server) reportPendingQueueInputs() []journalEntry {
	pending := s.turnJournal.TakePending()
	if len(pending) == 0 {
		return nil
	}
	byThread := map[string]int{}
	for _, e := range pending {
		byThread[e.ThreadID]++
	}
	log.Printf("[turnjournal] WARNING: 上次退出时有 %d 条排队输入还没轮到执行,已随重启丢失(涉及 %d 个会话);这些输入不会自动重发,如仍需要请在手机上重新发送",
		len(pending), len(byThread))
	for _, e := range pending {
		log.Printf("[turnjournal]   丢失 %s:%.120q(入队 %s)",
			e.ThreadID, e.Preview, time.UnixMilli(e.CreatedAt).Format("2006-01-02 15:04:05"))
	}
	return pending
}

// ---- 鉴权 ----

func (s *Server) token() string {
	// 旧单 token(legacy):token = sha256("pair-code:" + 配对码)。
	// 【兼容】P0-2 起新设备各持独立 token(见 devices.go),这个值仅用于
	// 存量手机回退比对,不再作为新配对的发放物。
	sum := sha256.Sum256([]byte("pair-code:" + s.cfg.PairCode))
	return hex.EncodeToString(sum[:])
}

func (s *Server) authorized(ctx context.Context) error {
	// 【PA-REAL-0001】PairCode 为空不再等于"关闭鉴权":正常启动路径(main)
	// 会自动生成配对码,走到这里还拿着空码只可能是手工构造的异常配置 ——
	// 一律 fail closed,绝不放行。
	if s.cfg.PairCode == "" {
		return status.Error(codes.Unauthenticated, "服务端未配置配对码,拒绝服务;请重启 daemon(去掉 -insecure-no-auth 即自动生成配对码)")
	}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "缺少 metadata")
	}
	for _, v := range md.Get("authorization") {
		if strings.HasPrefix(v, "Bearer ") {
			token := strings.TrimPrefix(v, "Bearer ")
			// 先查设备注册表(每设备 token,常时比较),查不到再回退旧单 token。
			if _, _, ok := s.devices.Validate(token); ok {
				return nil
			}
		}
	}
	return status.Error(codes.Unauthenticated, "token 无效,请先配对")
}

// checkAccess 是两类拦截器(Unary/Stream)的统一闸门。
//
// 顺序:先 insecure 回环闸门(对 HostService 一视同仁 —— 无鉴权模式下主机
// 信息同样不该暴露给局域网),再按方法族放行/验 token:
//
//   - InsecureNoAuth:回环放行,非回环拒绝(提示去配对码模式);
//   - 配对码模式:HostService(GetHostInfo/Authorize/Ping)放行 —— 配对的
//     前置:手机得先读到 auth_required、再用配对码换 token;其余方法验 token;
//   - 空 PairCode 且未显式 insecure:authorized() 内 fail closed。
func (s *Server) checkAccess(ctx context.Context, fullMethod string) error {
	if s.cfg.InsecureNoAuth {
		if !peerIsLoopback(ctx) {
			return status.Error(codes.PermissionDenied, "daemon 以 -insecure-no-auth 运行,仅允许本机回环连接;请设置 -pair-code 后再从局域网连接")
		}
		return nil
	}
	if strings.Contains(fullMethod, "HostService") {
		return nil
	}
	return s.authorized(ctx)
}

// peerIsLoopback 判定 gRPC 对端是否来自本机回环地址。
// 【fail closed】缺 peer 信息(单测直调、异常传输层)一律按非回环处理。
func peerIsLoopback(ctx context.Context) bool {
	p, ok := peer.FromContext(ctx)
	if !ok || p == nil || p.Addr == nil {
		return false
	}
	switch addr := p.Addr.(type) {
	case *net.TCPAddr:
		return addr.IP.IsLoopback()
	case *net.UnixAddr:
		return true // unix socket 只可能来自本机
	}
	// 兜底:字符串解析(覆盖非 TCP 地址族;IPv6 带 zone 由 netip 处理)
	host, _, err := net.SplitHostPort(p.Addr.String())
	if err != nil {
		host = p.Addr.String()
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	return ip.IsLoopback()
}

func (s *Server) UnaryAuth() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if err := s.checkAccess(ctx, info.FullMethod); err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

func (s *Server) StreamAuth() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if err := s.checkAccess(ss.Context(), info.FullMethod); err != nil {
			return err
		}
		return handler(srv, ss)
	}
}

// ---- 通用辅助 ----

func randomID(prefix string) string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		sum := sha256.Sum256([]byte(fmt.Sprintf("%d", hostCounter())))
		return fmt.Sprintf("%s-%x", prefix, sum[:4])
	}
	return fmt.Sprintf("%s-%x", prefix, b)
}

func platformInfo() (string, string, string) {
	sh := term.DefaultShell()
	return runtime.GOOS, runtime.GOARCH, sh
}

var counter int64

func hostCounter() int64 { counter++; return counter }
