package server

// thread_delete_test.go —— 删除会话的回归钉子(用户真机实证:"任何会话都删不掉")。
//
// 【钉的两件事】
//  1. **僵尸 RUNNING 不得成为"永远删不掉"的理由**:落盘状态是 RUNNING、但内存里
//     没有任何在飞租约时,必须允许删除并把状态复位 —— 真正的运行判据只有租约。
//     (僵尸从哪来:轮次异常收尾时来不及复位状态;以前只有 daemon 重启才会被清。)
//  2. **原生侧删除失败不阻塞本地删除**:电脑上的 rollout 找不到、或该 agent 的 CLI
//     当前起不来,不该把"用户想让这条会话从手机上消失"一起否掉 —— 电脑侧残留可以
//     在"未导入的电脑会话"列表里重新露面,由用户再处置。

import (
	"context"
	"errors"
	"testing"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// 僵尸态:Status=RUNNING 但没有任何在飞租约 → 可删。
func TestDeleteThreadZombieRunningIsDeletable(t *testing.T) {
	s := newTestServer(t)
	th := &store.Thread{ID: "th-zombie", Title: "卡住的会话", Status: pb.ThreadStatus_THREAD_STATUS_RUNNING}
	if err := s.cfg.Store.Create(th); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeleteThread(context.Background(), &pb.DeleteThreadRequest{ThreadId: "th-zombie"}); err != nil {
		t.Fatalf("僵尸 RUNNING(无在飞租约)应可删除, got %v", err)
	}
	if _, ok := s.cfg.Store.Get("th-zombie"); ok {
		t.Fatal("会话应已从库里删除")
	}
}

// 真在跑:持有在飞租约 → 必须拒绝(即使落盘状态是 IDLE)。
func TestDeleteThreadRejectsRealtimeRunning(t *testing.T) {
	s := newTestServer(t)
	s.activeTurns = newActiveTurns()
	th := &store.Thread{ID: "th-live", Title: "真在跑", Status: pb.ThreadStatus_THREAD_STATUS_IDLE}
	if err := s.cfg.Store.Create(th); err != nil {
		t.Fatal(err)
	}
	if _, err := s.activeTurns.acquire(context.Background(), "th-live", func() {}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeleteThread(context.Background(), &pb.DeleteThreadRequest{ThreadId: "th-live"}); err == nil {
		t.Fatal("有在飞租约时必须拒绝删除")
	}
	if _, ok := s.cfg.Store.Get("th-live"); !ok {
		t.Fatal("被拒绝的会话不应被删掉")
	}
}

// 原生删除失败(电脑上没有这条 codex 会话的 rollout)→ 本地仍应删掉。
func TestDeleteThreadSurvivesNativeDeleteFailure(t *testing.T) {
	s := newTestServer(t)
	th := &store.Thread{
		ID: "th-cx-missing", AgentID: "codex", Title: "电脑上没有的会话",
		CodexThreadID: "00000000-dead-beef-0000-000000000000",
		Status:        pb.ThreadStatus_THREAD_STATUS_IDLE,
	}
	if err := s.cfg.Store.Create(th); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeleteThread(context.Background(), &pb.DeleteThreadRequest{ThreadId: "th-cx-missing"}); err != nil {
		t.Fatalf("原生删除失败不应阻塞本地删除, got %v", err)
	}
	if _, ok := s.cfg.Store.Get("th-cx-missing"); ok {
		t.Fatal("本地记录应已删除(电脑侧残留由'未导入的电脑会话'列表兜底)")
	}
}

// 未导入的电脑会话"点删除"的路径:同样不该因 rollout 缺失而失败。
func TestDeleteThreadLocalOnlyHidesSession(t *testing.T) {
	s := newTestServer(t)
	th := &store.Thread{
		ID: "th-local", AgentID: "codex", Title: "仅本地移除",
		CodexThreadID: "00000000-dead-beef-0000-000000000001",
		Status:        pb.ThreadStatus_THREAD_STATUS_IDLE,
	}
	if err := s.cfg.Store.Create(th); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeleteThread(context.Background(), &pb.DeleteThreadRequest{ThreadId: "th-local", LocalOnly: true}); err != nil {
		t.Fatalf("LocalOnly 删除应成功, got %v", err)
	}
	if _, ok := s.cfg.Store.Get("th-local"); ok {
		t.Fatal("LocalOnly 应把会话从列表里移除")
	}
}

// 哨兵错误:调用方要能把"文件已经没有了"与"真故障"区分开。
func TestFindCodexSessionMissingUsesSentinel(t *testing.T) {
	_, err := agent.FindCodexSession("00000000-dead-beef-0000-000000000002")
	if err == nil {
		t.Fatal("不存在的会话应返回错误")
	}
	if !errors.Is(err, agent.ErrCodexSessionNotFound) {
		t.Fatalf("应是 ErrCodexSessionNotFound 哨兵, got %v", err)
	}
}
