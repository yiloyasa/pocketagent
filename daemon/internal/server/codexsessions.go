package server

// 电脑上 codex 已有会话:列出 + 一键导入为可续聊的本地线程。
// (用户需求:手机端要能看到桌面 codex 已经存在的项目和会话)

import (
	"context"
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
	if sessions == nil {
		scanned, err := agent.ListCodexSessions()
		if err != nil {
			return nil, errInternal(err)
		}
		sessions = scanned
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

func (s *Server) ImportCodexThread(ctx context.Context, req *pb.ImportCodexThreadRequest) (*pb.ImportCodexThreadResponse, error) {
	// 已导入过:直接复用现有线程(不重复建)
	for _, t := range s.cfg.Store.List(true) {
		if t.CodexThreadID == req.CodexThreadId {
			return &pb.ImportCodexThreadResponse{ThreadId: t.ID}, nil
		}
	}
	path, err := agent.FindCodexSession(req.CodexThreadId)
	if err != nil {
		return nil, errNotFound(err.Error())
	}
	msgs, err := agent.LoadCodexSessionMessages(path)
	if err != nil {
		return nil, errInternal(err)
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
	for _, m := range msgs {
		at := m.At.UnixMilli()
		if at <= 0 {
			at = time.Now().UnixMilli()
		}
		t.Messages = append(t.Messages, store.StoredMessage{
			Role:      m.Role,
			Blocks:    []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: m.Text}}}},
			CreatedAt: at,
		})
	}
	if err := s.cfg.Store.Create(t); err != nil {
		return nil, errInternal(err)
	}
	return &pb.ImportCodexThreadResponse{ThreadId: t.ID}, nil
}
