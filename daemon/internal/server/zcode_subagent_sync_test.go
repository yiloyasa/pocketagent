package server

// zcode_subagent_sync_test.go —— zcode 原生子代理清单存根登记的回归钉子。
//
// 【钉四件事】(协议依据 zc-subagents-probe.log + bundle schema,见
// zcode_subagent_sync.go 文件头)
//  1. running/ended 条目按 childSessionId 幂等落成存根,父子直挂(清单按父会话
//     取,无需候选消歧);
//  2. 状态映射:running/blocked→RUNNING、waiting→WAITING_APPROVAL、终态→IDLE,
//     failed 落 last_turn_error 且成功后清除;
//  3. 幂等:同一清单重复登记不重写(UpdatedAt 不变);
//  4. 墓碑:用户删过的存根,清单再来不再建回。

import (
	"testing"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
	"github.com/pocketagent/daemon/internal/zcodeapp"
)

// zcParent 造一个 zcode 父线程(已落库)。
func zcParent(t *testing.T, s *Server, id string) *store.Thread {
	t.Helper()
	parent := &store.Thread{
		ID:        id,
		Title:     "zcode 父会话",
		AgentID:   "zcode",
		Workspace: "C:\\work\\ws",
		Status:    pb.ThreadStatus_THREAD_STATUS_IDLE,
		CreatedAt: 1790330000000,
		UpdatedAt: 1790330001000,
	}
	if err := s.cfg.Store.Create(parent); err != nil {
		t.Fatal(err)
	}
	return parent
}

func zcResult() zcodeapp.SubagentsResult {
	return zcodeapp.SubagentsResult{
		Revision:        5,
		ChildSessionIDs: []string{"child-1", "child-2"},
		Running: []zcodeapp.SubagentEntry{{
			ChildSessionID: "child-1",
			SubagentType:   "explore",
			Title:          "检索证据",
			Status:         "running",
			StartedAt:      1790330002000,
		}},
		Ended: zcodeapp.SubagentsEndedPage{
			Total: 1,
			Items: []zcodeapp.SubagentEntry{{
				ChildSessionID: "child-2",
				SubagentType:   "general",
				Title:          "算数",
				Summary:        "137*942=129054",
				Status:         "success",
				StartedAt:      1790330000000,
				EndedAt:        1790330001000,
			}},
		},
	}
}

func TestZcodeSubagentStubsRegisteredAndLinked(t *testing.T) {
	s := newTestServer(t)
	parent := zcParent(t, s, "th-zc")

	s.registerZcodeSubagentStubs(parent.ID, zcResult())

	run, ok := s.cfg.Store.Get("zc-sub-child-1")
	if !ok {
		t.Fatal("running 条目应以存根形态登记(id = zc-sub-<childSessionId>)")
	}
	if run.ParentThreadID != parent.ID || run.AgentID != "zcode" {
		t.Fatalf("running 存根归属/agent 不对: parent=%q agent=%q", run.ParentThreadID, run.AgentID)
	}
	if run.Status != pb.ThreadStatus_THREAD_STATUS_RUNNING {
		t.Fatalf("running 条目应映射 RUNNING, got %v", run.Status)
	}
	if run.Title != "检索证据" {
		t.Fatalf("原生 title 应原样落库, got %q", run.Title)
	}

	end, ok := s.cfg.Store.Get("zc-sub-child-2")
	if !ok {
		t.Fatal("ended 条目应以存根形态登记")
	}
	if end.Status != pb.ThreadStatus_THREAD_STATUS_IDLE {
		t.Fatalf("success 终态应映射 IDLE, got %v", end.Status)
	}
	if end.LastTurnError != "" {
		t.Fatalf("成功终态不得带失败标记, got %q", end.LastTurnError)
	}
	if len(end.Messages) != 1 || storedMessageText(end.Messages[0]) != "[摘要] 137*942=129054" {
		t.Fatalf("ended 条目应带一条摘要消息, got %+v", end.Messages)
	}
	if end.UpdatedAt == 0 {
		t.Fatal("ended 存根 UpdatedAt 应由 store 落值(非零)")
	}
}

func TestZcodeSubagentStubStatusMapping(t *testing.T) {
	s := newTestServer(t)
	parent := zcParent(t, s, "th-zc")

	res := zcodeapp.SubagentsResult{
		Running: []zcodeapp.SubagentEntry{
			{ChildSessionID: "b1", Status: "blocked", Title: "x"},
			{ChildSessionID: "b2", Status: "waiting", Title: "y"},
		},
		Ended: zcodeapp.SubagentsEndedPage{Total: 1, Items: []zcodeapp.SubagentEntry{
			{ChildSessionID: "b3", Status: "failed", Title: "z"},
		}},
	}
	s.registerZcodeSubagentStubs(parent.ID, res)

	if got, _ := s.cfg.Store.Get("zc-sub-b1"); got.Status != pb.ThreadStatus_THREAD_STATUS_RUNNING {
		t.Fatalf("blocked 是仍在进行的后台运行,应映射 RUNNING, got %v", got.Status)
	}
	if got, _ := s.cfg.Store.Get("zc-sub-b2"); got.Status != pb.ThreadStatus_THREAD_STATUS_WAITING_APPROVAL {
		t.Fatalf("waiting(zcode 语义=等用户交互)应映射 WAITING_APPROVAL, got %v", got.Status)
	}
	failed, _ := s.cfg.Store.Get("zc-sub-b3")
	if failed.Status != pb.ThreadStatus_THREAD_STATUS_IDLE || failed.LastTurnError == "" {
		t.Fatalf("failed 应 IDLE+失败标记, got status=%v err=%q", failed.Status, failed.LastTurnError)
	}

	// 下一轮清单里它成功了:IDLE 保持、失败标记清除、摘要更新。
	res2 := zcodeapp.SubagentsResult{
		Ended: zcodeapp.SubagentsEndedPage{Total: 1, Items: []zcodeapp.SubagentEntry{
			{ChildSessionID: "b3", Status: "success", Title: "z", Summary: "完成了"},
		}},
	}
	s.registerZcodeSubagentStubs(parent.ID, res2)
	fixed, _ := s.cfg.Store.Get("zc-sub-b3")
	if fixed.LastTurnError != "" {
		t.Fatalf("成功后失败标记应清除, got %q", fixed.LastTurnError)
	}
	if len(fixed.Messages) != 1 || storedMessageText(fixed.Messages[0]) != "[摘要] 完成了" {
		t.Fatalf("新摘要应替换旧摘要, got %+v", fixed.Messages)
	}
}

func TestZcodeSubagentStubIdempotent(t *testing.T) {
	s := newTestServer(t)
	parent := zcParent(t, s, "th-zc")

	s.registerZcodeSubagentStubs(parent.ID, zcResult())
	first, _ := s.cfg.Store.Get("zc-sub-child-2")
	// 完全相同的清单再来一遍(模拟下一轮终局的重复事实):不得重写。
	s.registerZcodeSubagentStubs(parent.ID, zcResult())
	again, _ := s.cfg.Store.Get("zc-sub-child-2")

	if again.UpdatedAt != first.UpdatedAt || again.Status != first.Status || len(again.Messages) != len(first.Messages) {
		// 【vet】按字段打印:pb 内嵌 MessageState,整个值进 Fatalf 会被记拷锁。
		t.Fatalf("重复登记应零写入: updatedAt=%d→%d status=%v→%v msgs=%d→%d",
			first.UpdatedAt, again.UpdatedAt, first.Status, again.Status,
			len(first.Messages), len(again.Messages))
	}
}

func TestZcodeSubagentStubTombstoned(t *testing.T) {
	s := newTestServer(t)
	parent := zcParent(t, s, "th-zc")

	s.registerZcodeSubagentStubs(parent.ID, zcResult())
	if err := s.cfg.Store.AddTombstone("zc-sub-child-1"); err != nil {
		t.Fatal(err)
	}
	_ = s.cfg.Store.Delete("zc-sub-child-1")

	// 清单里它还在(用户只是删了卡,上游事实未变):不得建回。
	s.registerZcodeSubagentStubs(parent.ID, zcResult())
	if _, ok := s.cfg.Store.Get("zc-sub-child-1"); ok {
		t.Fatal("墓碑命中的存根不得重新登记")
	}
}

func TestZcodeSubagentStubSkipsWithoutParentOrID(t *testing.T) {
	s := newTestServer(t)
	// 父线程不在库:整批跳过,不造孤儿存根。
	s.registerZcodeSubagentStubs("th-missing", zcResult())
	if _, ok := s.cfg.Store.Get("zc-sub-child-1"); ok {
		t.Fatal("父不在本地库时不得登记存根")
	}
	// 条目缺 childSessionId:单条跳过(没有幂等键就不登记),其余照常。
	zcParent(t, s, "th-zc")
	res := zcodeapp.SubagentsResult{
		Running: []zcodeapp.SubagentEntry{{Status: "running", Title: "no-id"}},
	}
	s.registerZcodeSubagentStubs("th-zc", res)
	if _, ok := s.cfg.Store.Get("zc-sub-"); ok {
		t.Fatal("缺 childSessionId 的条目不得登记")
	}
}
