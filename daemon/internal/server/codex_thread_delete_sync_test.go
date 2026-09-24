// codex_thread_delete_sync_test.go —— thread/delete 真删同步的 server 侧单测。
//
// 【钉什么】DeleteThread 的 codex 原生删除现在有两步:删 rollout 文件
// (DeleteCodexSession,既有行为)+ 调 thread/delete(provider 协议真删,
// 2026-09 P2 补上)。钉死三条语义:
//  1. 调用发出:真删除(local_only=false)必须向 provider 发出 thread/delete;
//  2. 失败仍返回本地成功:thread/delete 失败(协议错误/子进程不在)只留日志,
//     本地删除与墓碑照常完成 —— 墓碑已是防复活的权威,原生失败不得卡住用户;
//  3. 移除(local_only=true)绝不发 thread/delete:那是可恢复的隐藏,不是真删。
package server

import (
	"context"
	"errors"
	"testing"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// fakeCodexDeleter 测试替身:注册进 Providers["codex"],记录 thread/delete
// 调用并按配置返回成功/失败(不碰任何真子进程)。
type fakeCodexDeleter struct {
	calls []string
	err   error
}

func (f *fakeCodexDeleter) ID() string          { return "codex" }
func (f *fakeCodexDeleter) DisplayName() string { return "codex 真删替身" }
func (f *fakeCodexDeleter) Models() []pb.ModelInfo {
	return []pb.ModelInfo{{Id: "default", Provider: "codex", Available: true}}
}
func (f *fakeCodexDeleter) Run(ctx context.Context, req agent.TurnRequest, sink agent.Sink, approve agent.ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
	return nil, nil, errors.New("替身不执行轮次")
}
func (f *fakeCodexDeleter) DeleteCodexThread(threadID string) error {
	f.calls = append(f.calls, threadID)
	return f.err
}

func newDeleteSyncServer(t *testing.T, fake *fakeCodexDeleter) *Server {
	t.Helper()
	s := newTestServer(t)
	s.cfg.Providers = map[string]agent.Provider{"codex": fake}
	return s
}

func TestDeleteThreadIssuesThreadDelete(t *testing.T) {
	fake := &fakeCodexDeleter{}
	s := newDeleteSyncServer(t, fake)
	th := &store.Thread{ID: "th-del-ok", AgentID: "codex", CodexThreadID: "cx-del-ok", Title: "真删同步"}
	if err := s.cfg.Store.Create(th); err != nil {
		t.Fatal(err)
	}

	if _, err := s.DeleteThread(context.Background(), &pb.DeleteThreadRequest{ThreadId: "th-del-ok"}); err != nil {
		t.Fatalf("删除应成功: %v", err)
	}
	if len(fake.calls) != 1 || fake.calls[0] != "cx-del-ok" {
		t.Fatalf("thread/delete 应对 codex 原生 id 发出且只发出一次, got=%v", fake.calls)
	}
	if _, ok := s.cfg.Store.Get("th-del-ok"); ok {
		t.Fatal("本地会话应已删除")
	}
	if !s.cfg.Store.IsTombstoned("cx-del-ok") {
		t.Fatal("墓碑应落盘(防原生同步链复活)")
	}
}

// 【核心语义】thread/delete 失败仍返回本地成功:用户点删除的本意是"让它从手机
// 上消失",codex 侧残留靠墓碑挡住 + 日志留痕,不得放大成"删不掉"。
func TestDeleteThreadSucceedsWhenThreadDeleteFails(t *testing.T) {
	fake := &fakeCodexDeleter{err: errors.New("模拟协议故障: app-server 不在")}
	s := newDeleteSyncServer(t, fake)
	th := &store.Thread{ID: "th-del-fail", AgentID: "codex", CodexThreadID: "cx-del-fail", Title: "原生删不掉"}
	if err := s.cfg.Store.Create(th); err != nil {
		t.Fatal(err)
	}

	if _, err := s.DeleteThread(context.Background(), &pb.DeleteThreadRequest{ThreadId: "th-del-fail"}); err != nil {
		t.Fatalf("thread/delete 失败不得阻塞本地删除: %v", err)
	}
	if len(fake.calls) != 1 {
		t.Fatalf("失败路径同样必须先发出调用(只留日志不吞调用), got=%v", fake.calls)
	}
	if _, ok := s.cfg.Store.Get("th-del-fail"); ok {
		t.Fatal("本地会话应已删除")
	}
	if !s.cfg.Store.IsTombstoned("cx-del-fail") {
		t.Fatal("原生失败时墓碑更要落盘,否则下一次同步立刻复活")
	}
}

// 移除(local_only=true)是可恢复的隐藏,不是真删:绝不发 thread/delete。
func TestLocalOnlyRemoveSkipsThreadDelete(t *testing.T) {
	fake := &fakeCodexDeleter{}
	s := newDeleteSyncServer(t, fake)
	th := &store.Thread{ID: "th-del-soft", AgentID: "codex", CodexThreadID: "cx-del-soft", Title: "仅移除"}
	if err := s.cfg.Store.Create(th); err != nil {
		t.Fatal(err)
	}

	if _, err := s.DeleteThread(context.Background(), &pb.DeleteThreadRequest{
		ThreadId: "th-del-soft", LocalOnly: true,
	}); err != nil {
		t.Fatalf("移除应成功: %v", err)
	}
	if len(fake.calls) != 0 {
		t.Fatalf("local_only 移除不该发 thread/delete, got=%v", fake.calls)
	}
}
