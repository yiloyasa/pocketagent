package server

// pi_subagent_sync_test.go —— T-SUBA-PI 回归钉子:pi 子代理运行的存根登记。
//
// 【钉三件事】
//  1. 幂等:同一 runId 反复登记只产出一条存根,消息只增不减;
//  2. **归属只在候选唯一时挂**:同工作区 + 活动期内候选 = 1 → 挂父链;
//     候选 ≥2 或时间对不上 → **不挂**(诚实变暗,绝不猜父子);
//  3. 没有 subagent-artifacts 目录 = 没派过子代理 → 零副作用。

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/pocketagent/daemon/internal/store"
)

// piParent 造一个 pi 父会话(有真实 session_file 指向临时工作区)。
func piParent(t *testing.T, work, name string, created, updated int64) *store.Thread {
	t.Helper()
	sf := filepath.Join(work, name+".jsonl")
	if err := os.WriteFile(sf, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return &store.Thread{
		ID:        "th-" + name,
		AgentID:   "pi",
		Title:     "父会话 " + name,
		CreatedAt: created,
		UpdatedAt: updated,
		ExternalSession: store.ExternalSessionRef{
			SessionID:   "sess-" + name,
			SessionFile: sf,
		},
	}
}

// writePiRun 在 work 的 subagent-artifacts 下造一次子代理运行。
func writePiRun(t *testing.T, work, runID, agentName string, tsMillis int64) {
	t.Helper()
	dir := filepath.Join(work, "subagent-artifacts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	meta := `{"runId":"` + runID + `","agent":"` + agentName + `","model":"m","timestamp":` +
		strconv.FormatInt(tsMillis, 10) + `,"cwd":` + strconv.Quote(work) + `}`
	for name, body := range map[string]string{
		runID + "_" + agentName + "_meta.json":        meta,
		runID + "_" + agentName + "_output.md":        "产出正文",
		runID + "_" + agentName + "_transcript.jsonl": `{"role":"assistant","text":"子代理说了一句"}` + "\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// 唯一候选 → 挂父链,且带上对话与产出。
func TestPiSubagentStubLinkedWhenSoleCandidate(t *testing.T) {
	s := newTestServer(t)
	work := t.TempDir()
	now := time.Now().UnixMilli()
	parent := piParent(t, work, "p1", now-60_000, now)
	if err := s.cfg.Store.Create(parent); err != nil {
		t.Fatal(err)
	}
	writePiRun(t, work, "run-a", "researcher", now-30_000)

	s.registerPiSubagentStubs()

	got, ok := s.cfg.Store.Get("pi-sub-run-a")
	if !ok {
		t.Fatal("子代理应以存根形态登记(id = pi-sub-<runId>)")
	}
	if got.ParentThreadID != parent.ID {
		t.Fatalf("唯一候选应挂上父链, want=%s got=%q", parent.ID, got.ParentThreadID)
	}
	if got.AgentID != "pi" {
		t.Fatalf("存根 agent 应为 pi, got %q", got.AgentID)
	}
	if len(got.Messages) < 2 {
		t.Fatalf("应带上[产出]+对话, got %d 条", len(got.Messages))
	}
}

// 候选 ≥2(同工作区两个会话都在活动期内)→ **不挂**父链。
func TestPiSubagentStubNotLinkedWhenAmbiguous(t *testing.T) {
	s := newTestServer(t)
	work := t.TempDir()
	now := time.Now().UnixMilli()
	for _, n := range []string{"a", "b"} {
		if err := s.cfg.Store.Create(piParent(t, work, n, now-60_000, now)); err != nil {
			t.Fatal(err)
		}
	}
	writePiRun(t, work, "run-amb", "researcher", now-30_000)

	s.registerPiSubagentStubs()

	got, ok := s.cfg.Store.Get("pi-sub-run-amb")
	if !ok {
		t.Fatal("即使归属判不出,子代理本身也应登记(只是不挂父链)")
	}
	if got.ParentThreadID != "" {
		t.Fatalf("多候选时不得猜父子, got parent=%q", got.ParentThreadID)
	}
}

// 时间对不上(子代理运行早于会话很久)→ 不挂。
func TestPiSubagentStubNotLinkedOutsideWindow(t *testing.T) {
	s := newTestServer(t)
	work := t.TempDir()
	now := time.Now().UnixMilli()
	if err := s.cfg.Store.Create(piParent(t, work, "p1", now-60_000, now)); err != nil {
		t.Fatal(err)
	}
	writePiRun(t, work, "run-old", "researcher", now-3_600_000) // 一小时前

	s.registerPiSubagentStubs()

	got, _ := s.cfg.Store.Get("pi-sub-run-old")
	if got != nil && got.ParentThreadID != "" {
		t.Fatalf("活动期外的运行不得挂父链, got %q", got.ParentThreadID)
	}
}

// 幂等:跑两遍不重复创建,消息不被削掉。
func TestPiSubagentStubIdempotent(t *testing.T) {
	s := newTestServer(t)
	work := t.TempDir()
	now := time.Now().UnixMilli()
	if err := s.cfg.Store.Create(piParent(t, work, "p1", now-60_000, now)); err != nil {
		t.Fatal(err)
	}
	writePiRun(t, work, "run-idem", "worker", now-30_000)

	s.registerPiSubagentStubs()
	first, _ := s.cfg.Store.Get("pi-sub-run-idem")
	if first == nil {
		t.Fatal("首轮应登记")
	}
	s.registerPiSubagentStubs()
	second, _ := s.cfg.Store.Get("pi-sub-run-idem")
	if second == nil || len(second.Messages) < len(first.Messages) {
		t.Fatalf("重复扫描不得削掉已有消息: first=%d second=%d",
			len(first.Messages), len(second.Messages))
	}
	// 全库只应有一条该 run 的存根。
	n := 0
	for _, th := range s.cfg.Store.List(true) {
		if th.ID == "pi-sub-run-idem" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("存根必须唯一, got %d 条", n)
	}
}

// 没派过子代理(无 artifacts 目录)→ 零副作用、不报错。
func TestPiSubagentStubsNoopWithoutArtifacts(t *testing.T) {
	s := newTestServer(t)
	work := t.TempDir()
	now := time.Now().UnixMilli()
	if err := s.cfg.Store.Create(piParent(t, work, "p1", now-60_000, now)); err != nil {
		t.Fatal(err)
	}
	before := len(s.cfg.Store.List(true))
	s.registerPiSubagentStubs()
	if after := len(s.cfg.Store.List(true)); after != before {
		t.Fatalf("无 artifacts 不应新增任何线程: before=%d after=%d", before, after)
	}
}
