package server

// turn_started_test.go —— Thread.turn_started_unix_ms 透出的回归钉子(2026-09-26,
// EA3 轮留下的"真实时长缺口":手机端运行中条此前只能显示"最近活动·相对时间",
// 因为协议没有轮次开始时刻)。本文件钉住四件事:
//  1. threadToPB 把 store 值透传进 pb(optional:0 → 显式缺席,不冒充);
//  2. store JSON 落盘往返不丢该字段;
//  3. zcode 存根首次观察到 RUNNING 时记存根的轮次起点;
//  4. 启动恢复回路对重启后仍 RUNNING 的会话**无条件**回填重启时刻
//     (真实开始不可追溯,见 backfillRestartedRunningStamp 注释),恢复出
//     终局/IDLE 的会话不动。

import (
	"testing"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
	"github.com/pocketagent/daemon/internal/zcodeapp"
)

func TestThreadToPBTurnStartedPassthrough(t *testing.T) {
	s := newTestServer(t)
	th := &store.Thread{
		ID:                "th-run",
		Status:            pb.ThreadStatus_THREAD_STATUS_RUNNING,
		TurnStartedUnixMs: 1727300000000,
	}
	out := s.threadToPB(th)
	if out.TurnStartedUnixMs == nil {
		t.Fatal("非零 TurnStartedUnixMs 应透出(optional 显式在场)")
	}
	if v := out.GetTurnStartedUnixMs(); v != 1727300000000 {
		t.Fatalf("透传值不符: got %d", v)
	}
	// 0 → 缺席:旧 daemon/尚未开轮的会话不把 0 冒充成真实时刻
	out2 := s.threadToPB(&store.Thread{ID: "th-fresh"})
	if out2.TurnStartedUnixMs != nil {
		t.Fatalf("0 值应缺席, got %d", out2.GetTurnStartedUnixMs())
	}
}

func TestTurnStartedUnixMsSurvivesStoreRoundTrip(t *testing.T) {
	s := newTestServer(t)
	th := &store.Thread{
		ID:                "th-round",
		Status:            pb.ThreadStatus_THREAD_STATUS_RUNNING,
		TurnStartedUnixMs: 1727300000001,
	}
	if err := s.cfg.Store.Create(th); err != nil {
		t.Fatal(err)
	}
	got, ok := s.cfg.Store.Get("th-round")
	if !ok {
		t.Fatal("线程不应丢失")
	}
	if got.TurnStartedUnixMs != 1727300000001 {
		t.Fatalf("JSON 落盘往返应保留轮次起点, got %d", got.TurnStartedUnixMs)
	}
}

func TestZcodeSubagentStubRunningStamp(t *testing.T) {
	s := newTestServer(t)
	parent := &store.Thread{ID: "th-main", AgentID: "zcode", Title: "父"}
	if err := s.cfg.Store.Create(parent); err != nil {
		t.Fatal(err)
	}
	before := time.Now().UnixMilli()
	s.registerZcodeSubagentStubs("th-main", zcodeapp.SubagentsResult{
		Running: []zcodeapp.SubagentEntry{{ChildSessionID: "cs-1", Status: "running"}},
	})
	got, ok := s.cfg.Store.Get("zc-sub-cs-1")
	if !ok {
		t.Fatal("存根应登记")
	}
	if got.Status != pb.ThreadStatus_THREAD_STATUS_RUNNING {
		t.Fatalf("存根应为 RUNNING, got %v", got.Status)
	}
	if got.TurnStartedUnixMs < before {
		t.Fatalf("首次观察到 RUNNING 应记轮次起点, got %d", got.TurnStartedUnixMs)
	}
}

func TestBackfillRestartedRunningStamp(t *testing.T) {
	s := newTestServer(t)
	legacy := &store.Thread{ID: "th-legacy", Status: pb.ThreadStatus_THREAD_STATUS_RUNNING}
	stale := &store.Thread{
		ID:                "th-stale",
		Status:            pb.ThreadStatus_THREAD_STATUS_RUNNING,
		TurnStartedUnixMs: 1000, // 上一进程留下的旧值:对当前展示无意义,应被覆盖
	}
	idle := &store.Thread{ID: "th-idle", Status: pb.ThreadStatus_THREAD_STATUS_IDLE}
	for _, th := range []*store.Thread{legacy, stale, idle} {
		if err := s.cfg.Store.Create(th); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now()
	s.backfillRestartedRunningStamp(now)

	for _, id := range []string{"th-legacy", "th-stale"} {
		got, ok := s.cfg.Store.Get(id)
		if !ok {
			t.Fatalf("%s 不应丢失", id)
		}
		if got.Status != pb.ThreadStatus_THREAD_STATUS_RUNNING {
			t.Fatalf("%s 应保持 RUNNING(测试库无 provider,不会恢复出终局), got %v", id, got.Status)
		}
		if got.TurnStartedUnixMs < now.UnixMilli() {
			t.Fatalf("%s 应回填重启时刻(无条件覆盖旧值), got %d", id, got.TurnStartedUnixMs)
		}
	}
	if got, _ := s.cfg.Store.Get("th-idle"); got.TurnStartedUnixMs != 0 {
		t.Fatalf("IDLE 会话不该被盖, got %d", got.TurnStartedUnixMs)
	}
}
