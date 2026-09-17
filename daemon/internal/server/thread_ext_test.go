package server

// 同步降级相关单测:
//   1) runBounded:codex 调用必须在预算内结束,挂起要超时降级,不能无限阻塞;
//   2) mergeCodexEntries:同步结果写回本地库(标题/分组/归档)且只动匹配到的会话。

import (
	"testing"
	"time"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

func TestRunBoundedReturnsFastValue(t *testing.T) {
	v, err := runBounded(50*time.Millisecond, func() ([]int, error) {
		return []int{1, 2, 3}, nil
	})
	if err != nil {
		t.Fatalf("fast fn 不应报错, got %v", err)
	}
	if len(v) != 3 {
		t.Fatalf("fast fn 应原样返回值, got %v", v)
	}
}

func TestRunBoundedTimesOutInsteadOfBlocking(t *testing.T) {
	start := time.Now()
	_, err := runBounded(30*time.Millisecond, func() ([]int, error) {
		time.Sleep(2 * time.Second) // 模拟 codex 子进程卡死
		return nil, nil
	})
	if err == nil {
		t.Fatal("挂起的 fn 应超时返回 error,而不是一直阻塞")
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("超时应在预算附近返回,实际用了 %v(会拖累会话列表)", elapsed)
	}
}

func newTestServer(t *testing.T) *Server {
	t.Helper()
	fs, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("建测试存储失败: %v", err)
	}
	return &Server{cfg: Config{Store: fs}}
}

func TestMergeCodexEntriesAppliesChanges(t *testing.T) {
	s := newTestServer(t)
	th := &store.Thread{
		ID:            "th1",
		CodexThreadID: "cx1",
		Title:         "旧标题",
		Status:        pb.ThreadStatus_THREAD_STATUS_IDLE,
	}
	if err := s.cfg.Store.Create(th); err != nil {
		t.Fatal(err)
	}

	s.mergeCodexEntries([]agent.CodexThreadEntry{
		{ID: "cx1", Title: "新标题", SectionID: "secA", Archived: true},
	})

	got, ok := s.cfg.Store.Get("th1")
	if !ok {
		t.Fatal("线程不应丢失")
	}
	if got.Title != "新标题" {
		t.Fatalf("标题未更新: %q", got.Title)
	}
	if got.SectionID != "secA" {
		t.Fatalf("分组未更新: %q", got.SectionID)
	}
	if got.Status != pb.ThreadStatus_THREAD_STATUS_ARCHIVED {
		t.Fatalf("归档状态未更新: %v", got.Status)
	}
}

func TestMergeCodexEntriesSkipsUnknownCodexID(t *testing.T) {
	s := newTestServer(t)
	th := &store.Thread{ID: "th1", CodexThreadID: "cx1", Title: "原标题"}
	if err := s.cfg.Store.Create(th); err != nil {
		t.Fatal(err)
	}

	s.mergeCodexEntries([]agent.CodexThreadEntry{{ID: "other", Title: "X"}})

	got, _ := s.cfg.Store.Get("th1")
	if got.Title != "原标题" {
		t.Fatalf("不应改动 codex id 不匹配的线程, got %q", got.Title)
	}
}

func TestMergeCodexEntriesLeavesRunningStatusUntouched(t *testing.T) {
	s := newTestServer(t)
	th := &store.Thread{
		ID:            "th1",
		CodexThreadID: "cx1",
		Title:         "旧",
		Status:        pb.ThreadStatus_THREAD_STATUS_RUNNING,
	}
	if err := s.cfg.Store.Create(th); err != nil {
		t.Fatal(err)
	}

	// codex 侧说它已归档,但本地正在跑 —— 不能把进行中的会话改成归档。
	s.mergeCodexEntries([]agent.CodexThreadEntry{{ID: "cx1", Title: "新", Archived: true}})

	got, _ := s.cfg.Store.Get("th1")
	if got.Status != pb.ThreadStatus_THREAD_STATUS_RUNNING {
		t.Fatalf("进行中的会话状态不应被覆盖, got %v", got.Status)
	}
	if got.Title != "新" {
		t.Fatalf("标题仍应更新, got %q", got.Title)
	}
}
