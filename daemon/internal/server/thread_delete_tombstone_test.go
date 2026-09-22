package server

// thread_delete_tombstone_test.go —— 墓碑机制的回归钉子(用户真机实证:"删除后又回来")。
//
// 【钉住的根因】删除会话后,两条自动登记链会在用户没操作时把它重新建回来:
//   - registerPiSubagentStubs:每次列举会话都扫磁盘 artifacts 重新登记 pi 子代理存根;
//   - maybeRegisterCodexSubagentStub:codex 原生子线程的 thread/started 到达时登记存根。
// 它们按"磁盘/原生会话里有没有"判定,不知道"用户刚删过"。修复是墓碑:删除时记下
// 标识,同步链遇到就跳过。下面的用例钉死"删除后模拟同步链再跑一次,会话不能回来"。
//
// 另钉症状②③:原生事实来源(电脑上的记录/文件)已找不到时,GetThread/GetThreadHistory
// 必须如实报错,不能静默返回空内容假装进得去。

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// 删除 pi 子代理存根后,下一次"打开会话列表"触发的同步链不能再把它建回来。
func TestDeletePiSubagentStubNotResurrected(t *testing.T) {
	s := newTestServer(t)
	work := t.TempDir()
	now := time.Now().UnixMilli()
	parent := piParent(t, work, "p1", now-60_000, now)
	if err := s.cfg.Store.Create(parent); err != nil {
		t.Fatal(err)
	}
	writePiRun(t, work, "run-a", "researcher", now-30_000)

	// 前置:同步链先把子代理登记成存根。
	s.registerPiSubagentStubs()
	if _, ok := s.cfg.Store.Get("pi-sub-run-a"); !ok {
		t.Fatal("前置失败:子代理应先被登记为存根")
	}

	// 用户在手机端删除这条子代理存根。
	if _, err := s.DeleteThread(context.Background(), &pb.DeleteThreadRequest{ThreadId: "pi-sub-run-a"}); err != nil {
		t.Fatalf("删除应成功: %v", err)
	}
	if _, ok := s.cfg.Store.Get("pi-sub-run-a"); ok {
		t.Fatal("删除后存根应已从库里消失")
	}
	if !s.cfg.Store.IsTombstoned("pi-sub-run-a") {
		t.Fatal("删除应写入墓碑(否则同步链会重建它)")
	}

	// 模拟"打开会话列表"再次触发同步链。
	s.registerPiSubagentStubs()
	if _, ok := s.cfg.Store.Get("pi-sub-run-a"); ok {
		t.Fatal("墓碑失效:被删的 pi 子代理存根不应被同步链重建回来")
	}
}

// 删除 codex 子线程存根后,codex 再次发来 thread/started 也不能把它建回来。
func TestDeleteCodexSubagentStubNotResurrected(t *testing.T) {
	s := newTestServer(t)
	parent := &store.Thread{ID: "th-main", CodexThreadID: "cx-parent", Title: "主会话"}
	if err := s.cfg.Store.Create(parent); err != nil {
		t.Fatal(err)
	}

	// 前置:codex 子线程被登记为存根。
	s.SyncThreadEvent("cx-child", "thread/started",
		json.RawMessage(startedParams("cx-child", "cx-parent", "审查者")))
	if _, ok := s.cfg.Store.Get("cx-child"); !ok {
		t.Fatal("前置失败:codex 子线程应先被登记为存根")
	}

	// 用户在手机端删除。
	if _, err := s.DeleteThread(context.Background(), &pb.DeleteThreadRequest{ThreadId: "cx-child"}); err != nil {
		t.Fatalf("删除应成功: %v", err)
	}
	if _, ok := s.cfg.Store.Get("cx-child"); ok {
		t.Fatal("删除后存根应已从库里消失")
	}
	if !s.cfg.Store.IsTombstoned("cx-child") {
		t.Fatal("删除应写入墓碑")
	}

	// 模拟 codex 再次下发同一条 thread/started。
	s.SyncThreadEvent("cx-child", "thread/started",
		json.RawMessage(startedParams("cx-child", "cx-parent", "审查者")))
	if _, ok := s.cfg.Store.Get("cx-child"); ok {
		t.Fatal("墓碑失效:被删的 codex 子线程存根不应被重建")
	}
}

// 症状②③:会话存在但电脑上的记录已找不到时,历史接口必须如实报错(人话),
// 不能静默返回空内容假装进得去;同时把原因落到会话上让列表也能反映。
func TestGetThreadHistoryReportsMissingNativeSession(t *testing.T) {
	s := newTestServer(t)
	th := &store.Thread{
		ID:            "th-cxgone",
		AgentID:       "codex",
		Title:         "电脑上记录没了",
		CodexThreadID: "00000000-dead-beef-0000-000000000099",
		Status:        pb.ThreadStatus_THREAD_STATUS_IDLE,
	}
	if err := s.cfg.Store.Create(th); err != nil {
		t.Fatal(err)
	}

	_, err := s.GetThreadHistory(context.Background(), &pb.GetThreadHistoryRequest{ThreadId: "th-cxgone"})
	if err == nil {
		t.Fatal("应如实报错(内容读不出),而非静默返回空历史")
	}
	if !strings.Contains(err.Error(), "电脑上的记录已经找不到了") {
		t.Fatalf("报错应说明是文件找不到,而非笼统的'错误/未知', got: %v", err)
	}

	// 列表应如实反映失效原因。
	got, ok := s.cfg.Store.Get("th-cxgone")
	if !ok {
		t.Fatal("会话本身不应消失")
	}
	if got.LastTurnError == "" {
		t.Fatal("失效会话应在列表上标注原因(症状③)")
	}
	if !strings.Contains(got.LastTurnError, "电脑上的记录已经找不到了") {
		t.Fatalf("列表标注原因不符: %q", got.LastTurnError)
	}
}

// 症状②:GetThread(进会话的元数据入口)遇到悬空会话也要如实报错,不能假装进得去。
func TestGetThreadReportsMissingNativeSession(t *testing.T) {
	s := newTestServer(t)
	th := &store.Thread{
		ID:            "th-cxgone2",
		AgentID:       "codex",
		Title:         "电脑上记录没了2",
		CodexThreadID: "00000000-dead-beef-0000-000000000098",
		Status:        pb.ThreadStatus_THREAD_STATUS_IDLE,
	}
	if err := s.cfg.Store.Create(th); err != nil {
		t.Fatal(err)
	}
	_, err := s.GetThread(context.Background(), &pb.GetThreadRequest{ThreadId: "th-cxgone2"})
	if err == nil {
		t.Fatal("应如实报错,而非静默返回会话元数据")
	}
	if !strings.Contains(err.Error(), "电脑上的记录已经找不到了") {
		t.Fatalf("报错应说明文件找不到, got: %v", err)
	}
}
