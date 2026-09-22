package server

// 电脑上 codex 已有会话:列出 + 一键导入为可续聊的本地线程。
// (用户需求:手机端要能看到桌面 codex 已经存在的项目和会话)

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// codexAppServerBudget 是 ListCodexThreads 调 codex app-server 的时间预算。
// codex 不可用时(子进程重启 / 令牌失效)必须在预算内失败并返回错误,
// 让手机端展示 pcError,而不是挂起等 callTimeout(120s)。
const codexAppServerBudget = 5 * time.Second

// listCodexSessions 是 agent.ListCodexSessions 的间接层:codex 会话扫描长在
// agent 包的**包级函数**上(不是 provider 方法),单测没法注入受控会话 ——
// 而"删除过的 codex 会话会不会被列表挡掉"恰恰只能靠受控输入来钉(真机上
// 用户看到的正是这个 bug)。默认就是原函数,线上行为逐字不变。
var listCodexSessions = agent.ListCodexSessions

// runBounded 在 budget 内执行 fn;fn 超时/失败一律返回零值与 error,
// 调用方据此降级(放弃同步 / 把错误透传给手机),绝不无限挂起。
// fn 仍在后台跑完(最坏等下游自身超时),但调用方早已带着降级结果返回。
func runBounded[T any](budget time.Duration, fn func() (T, error)) (T, error) {
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()
	type result struct {
		v   T
		err error
	}
	ch := make(chan result, 1)
	go func() {
		v, err := fn()
		ch <- result{v, err}
	}()
	select {
	case r := <-ch:
		return r.v, r.err
	case <-ctx.Done():
		var zero T
		return zero, fmt.Errorf("调用超时(%s)", budget)
	}
}

// listCodexViaAppServerBounded 在预算内调用 app-server 的 thread/list;
// 超时/失败一律返回 error,由调用方决定是否把错误透传给手机(pcError 展示)。
func listCodexViaAppServerBounded(p *agent.CodexProvider, budget time.Duration) ([]agent.CodexSessionInfo, error) {
	return runBounded[[]agent.CodexSessionInfo](budget, p.ListCodexThreadsViaAppServer)
}

func (s *Server) ListCodexThreads(ctx context.Context, req *pb.ListCodexThreadsRequest) (*pb.ListCodexThreadsResponse, error) {
	// 首选 app-server 的 thread/list(codex 自动命名的真实标题)。
	// 【短预算 + 降级】codex 不可用时必须在预算内返回,但**不要**直接报错 ——
	// 下面的 rollout 扫描不依赖 codex 进程存活,降级到它仍能给出可用列表
	// (报错会让手机端"电脑上的会话"整个空掉,比旧数据更糟)。
	var sessions []agent.CodexSessionInfo
	if p, ok := s.cfg.Providers["codex"].(*agent.CodexProvider); ok {
		via, err := listCodexViaAppServerBounded(p, codexAppServerBudget)
		if err != nil {
			log.Printf("[sessions] app-server thread/list 超时/失败(%.1fs 预算),降级 rollout 扫描: %v",
				codexAppServerBudget.Seconds(), err)
		} else {
			sessions = via
		}
	}
	// 标题/目录兜底:app-server 有时只回 id(标题为空),或干脆一条都没回(此时
	// 用本地 rollout 扫描兜底,保证"加载 codex 已有项目/任务时能直接看到标题",
	// 而不是一片"(未命名会话)")。
	scanned, scanErr := listCodexSessions()
	if len(sessions) == 0 {
		if scanErr != nil {
			return nil, errInternal(scanErr)
		}
		sessions = scanned
	} else if scanErr == nil {
		byID := make(map[string]agent.CodexSessionInfo, len(scanned))
		for _, ss := range scanned {
			byID[ss.ID] = ss
		}
		for i := range sessions {
			sc, ok := byID[sessions[i].ID]
			if !ok {
				continue
			}
			if strings.TrimSpace(sessions[i].Title) == "" {
				sessions[i].Title = sc.Title
			}
			if sessions[i].Cwd == "" {
				sessions[i].Cwd = sc.Cwd
			}
			if sessions[i].UpdatedAt.IsZero() {
				sessions[i].UpdatedAt = sc.UpdatedAt
			}
		}
	}
	out := &pb.ListCodexThreadsResponse{}
	for _, ss := range sessions {
		// 【删除墓碑】原生删除失败时 rollout 文件仍在,扫描照样会把删过的会话列出来
		// —— 这就是"删掉后它又出现在『电脑上的 Codex 会话』里"的主力路径。
		// 挡在列表出口是唯一不依赖"原生删除成功"的可靠位置(降级扫描也走这里)。
		if s.cfg.Store.IsTombstoned(ss.ID) {
			continue
		}
		out.Threads = append(out.Threads, &pb.CodexThreadInfo{
			Id:              ss.ID,
			Title:           ss.Title,
			Cwd:             ss.Cwd,
			UpdatedAtUnixMs: ss.UpdatedAt.UnixMilli(),
		})
	}
	return out, nil
}

// codexRolloutMessages 把某个 codex 线程的 rollout 读成本地会话消息。
//
// 【为什么抽出来】"把一个 codex 线程变成可续聊的本地线程"有两个入口:
// 导入电脑上已有的会话(ImportCodexThread)、以及分叉(ForkThread)。
// 两边的差别只在标题/工作区怎么定,消息映射必须完全一致 —— 否则会出现
// "导入的会话有历史、分叉的没历史"这种同源不同果的差异。
func codexRolloutMessages(codexThreadID string) ([]store.StoredMessage, error) {
	path, err := agent.FindCodexSession(codexThreadID)
	if err != nil {
		return nil, err
	}
	msgs, err := agent.LoadCodexSessionMessages(path)
	if err != nil {
		return nil, err
	}
	out := make([]store.StoredMessage, 0, len(msgs))
	for _, m := range msgs {
		at := m.At.UnixMilli()
		if at <= 0 {
			at = time.Now().UnixMilli()
		}
		out = append(out, store.StoredMessage{
			Role:      m.Role,
			Blocks:    []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: m.Text}}}},
			CreatedAt: at,
		})
	}
	return out, nil
}

func (s *Server) ImportCodexThread(ctx context.Context, req *pb.ImportCodexThreadRequest) (*pb.ImportCodexThreadResponse, error) {
	// 【删除墓碑】没有这道守卫,导入入口就是"复活快捷键":列表挡住了,用户在导入页
	// 点一下就把它拉回来。判定必须在读原生 rollout **之前** —— 否则错误会退化成
	// NotFound("rollout 不存在"),把"你删过它"说成"它本来就不存在",掩盖真实原因。
	if s.cfg.Store.IsTombstoned(req.CodexThreadId) {
		return nil, errFailedPrecondition("会话已被删除;如需恢复请先清除删除记录")
	}
	// 已导入过:直接复用现有线程(不重复建)
	for _, t := range s.cfg.Store.List(true) {
		if t.CodexThreadID == req.CodexThreadId {
			return &pb.ImportCodexThreadResponse{ThreadId: t.ID}, nil
		}
	}
	msgs, err := codexRolloutMessages(req.CodexThreadId)
	if err != nil {
		// FindCodexSession 给的是自定义错误("找不到 codex 会话 …的 rollout"),
		// 语义就是"没这个会话" —— 与改动前一致地回 NotFound。
		return nil, errNotFound(err.Error())
	}
	sessions, _ := listCodexSessions()
	title, cwd := "codex 会话", ""
	for _, ss := range sessions {
		if ss.ID == req.CodexThreadId {
			title, cwd = ss.Title, ss.Cwd
			break
		}
	}
	if cwd == "" {
		cwd = s.cfg.Store.DefaultWorkspace()
		if cwd == "" {
			cwd = s.cfg.Workspace
		}
	}
	t := &store.Thread{
		ID:            randomID("th"),
		Title:         title,
		Workspace:     cwd,
		ModelID:       "codex/default",
		CodexThreadID: req.CodexThreadId,
		CreatedAt:     time.Now().UnixMilli(),
		UpdatedAt:     time.Now().UnixMilli(),
	}
	t.Messages = append(t.Messages, msgs...)
	if err := s.cfg.Store.Create(t); err != nil {
		return nil, errInternal(err)
	}
	return &pb.ImportCodexThreadResponse{ThreadId: t.ID}, nil
}
