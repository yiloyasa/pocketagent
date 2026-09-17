package server

// 电脑上 codex 已有会话:列出 + 一键导入为可续聊的本地线程。
// (用户需求:手机端要能看到桌面 codex 已经存在的项目和会话)

import (
	"context"
	"strings"
	"time"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

func (s *Server) ListCodexThreads(ctx context.Context, req *pb.ListCodexThreadsRequest) (*pb.ListCodexThreadsResponse, error) {
	// 首选 app-server 的 thread/list(codex 自动命名的真实标题);失败回退扫 rollout
	var sessions []agent.CodexSessionInfo
	if p, ok := s.cfg.Providers["codex"].(*agent.CodexProvider); ok {
		if via, err := p.ListCodexThreadsViaAppServer(); err == nil && len(via) > 0 {
			sessions = via
		}
	}
	// 标题/目录兜底:app-server 有时只回 id(标题为空),这里用 rollout 扫描
	// (首条用户消息 + 文件头 cwd)补齐 —— 保证"加载 codex 已有项目/任务时
	// 能直接看到标题",而不是一片"(未命名会话)"。
	scanned, scanErr := agent.ListCodexSessions()
	if sessions == nil {
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
	sessions, _ := agent.ListCodexSessions()
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
