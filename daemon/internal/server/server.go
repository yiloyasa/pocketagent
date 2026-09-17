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
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"runtime"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/pocketagent/daemon/internal/agent"
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

	cfg Config
	// hostID 进程生命周期内稳定(P1 持久化)
	hostID string
	// 每线程正在执行的 turn(重入打断用)
	activeTurns *activeTurns
}

func New(cfg Config) *Server {
	srv := &Server{
		cfg:         cfg,
		hostID:      randomID("host"),
		activeTurns: newActiveTurns(),
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
			for _, b := range m.Blocks {
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
	return srv
}

// ---- 鉴权 ----

func (s *Server) token() string {
	// 配对码即口令:token = sha256("pair-code:" + 配对码)。
	// 【讲解】不存明文,比较用 constant time 防时序侧信道。P1 换成
	// 每设备独立 token + 存储撤销列表。
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
			if subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(v, "Bearer ")), []byte(s.token())) == 1 {
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
