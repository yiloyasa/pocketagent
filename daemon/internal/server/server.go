// Package server 实现全部 gRPC 服务 + 鉴权拦截器。
//
// 【讲解】gRPC 的"拦截器"类似 Web 框架的中间件:每个请求进来先过它。
// 我们在 metadata(可以理解为 HTTP 的 header)里找 "authorization: Bearer xxx",
// 没配 pair-code 时全放行(局域网自用),配了则必须匹配。
package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/pocketagent/daemon/internal/agent"
	"github.com/pocketagent/daemon/internal/eventstore"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
	"github.com/pocketagent/daemon/internal/term"
)

type Config struct {
	Store     *store.FileStore
	Providers map[string]agent.Provider
	Terminals *term.Manager
	PairCode  string
	Workspace string
	DataDir   string
}

type Server struct {
	pb.UnimplementedHostServiceServer
	pb.UnimplementedThreadServiceServer
	pb.UnimplementedTerminalServiceServer
	pb.UnimplementedFileSystemServiceServer
	pb.UnimplementedGitServiceServer
	pb.UnimplementedModelServiceServer

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
	// P1-4:运行中继续发消息的内存队列(上限 10 条/线程;重启即丢,见 turn_queue.go)
	turnQueue *turnInputQueue
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
}

func New(cfg Config) *Server {
	// P1-5:事件存储打开失败不致命 —— Open 内部对文件层错误有降级,
	// 这里再兜一层:实在建不起来就用内存模式,daemon 其余功能照常。
	events, err := eventstore.Open(cfg.DataDir)
	if err != nil {
		log.Printf("[eventseq] 事件存储初始化失败,退回内存模式: %v", err)
		events, _ = eventstore.Open("")
	}
	srv := &Server{
		cfg:         cfg,
		devices:     newDeviceRegistry(cfg.DataDir, cfg.PairCode),
		hostID:      randomID("host"),
		activeTurns: newActiveTurns(),
		turnQueue:   newTurnInputQueue(),
		events:      events,
		hostWatch:   newHostWatchers(),
	}
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
			_ = cfg.Store.Update(t)
			break
		}
	}
	// codex 自动给线程起名 → 同步到我们的存储(手机/桌面双端一致)
	if p, ok := cfg.Providers["codex"].(*agent.CodexProvider); ok {
		p.TitleHandler = func(codexTID, title string) {
			for _, t := range cfg.Store.List(true) {
				if t.CodexThreadID == codexTID && t.Title != title {
					t.Title = title
					_ = cfg.Store.Update(t)
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
				_ = cfg.Store.Update(t)
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
	// ZCode 标题同步:session.titleUpdated → TitleHandler(与 codex 同语义,
	// 手机/桌面双端标题一致)。zcode 回调带的是我们自己的线程 ID(见 zcode.go),
	// 直接按 t.ID 落库即可,不需要 codex 那样的外部 id 映射。
	if zp, ok := cfg.Providers["zcode"].(*agent.ZcodeProvider); ok {
		zp.TitleHandler = func(threadID, title string) {
			for _, t := range cfg.Store.List(true) {
				if t.ID == threadID && t.Title != title {
					t.Title = title
					_ = cfg.Store.Update(t)
				}
			}
		}
	}
	// Pi 标题同步:session_info_changed → 落库(与 zcode 同语义;实现见 thread.go)。
	srv.wirePiTitleHandler()
	// Resolve legacy RUNNING records before list synchronization can hide their
	// terminal error. This consults native journals only, never launches a turn.
	for _, thread := range cfg.Store.List(true) {
		if thread.Status == pb.ThreadStatus_THREAD_STATUS_RUNNING {
			srv.recoverNativeResult(context.Background(), thread)
		}
	}
	return srv
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
	if s.cfg.PairCode == "" {
		return nil // 未启用鉴权(仅建议局域网这样跑)
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

func (s *Server) UnaryAuth() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// GetHostInfo / Authorize / Ping 放行,其余都要 token
		if !strings.Contains(info.FullMethod, "HostService") {
			if err := s.authorized(ctx); err != nil {
				return nil, err
			}
		}
		return handler(ctx, req)
	}
}

func (s *Server) StreamAuth() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if !strings.Contains(info.FullMethod, "HostService") {
			if err := s.authorized(ss.Context()); err != nil {
				return err
			}
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
