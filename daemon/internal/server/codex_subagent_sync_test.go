package server

// codex_subagent_sync_test.go —— T-S1P 回归钉子:codex 原生子代理线程的存根
// 登记(maybeRegisterCodexSubagentStub)+ thread/list 带出 parent_thread_id。
//
// 【真值依据】S0 实测帧(docs/verify/evidence/codex-subagent-s0.md):
// thread/started = {"thread":{id, parentThreadId(子代理非空/主线程 null),
// agentNickname, cwd, ...}},顶层无 threadId(由 agent.codexThreadIDOf 回退解析)。

import (
	"encoding/json"
	"testing"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// startedParams 拼一条 thread/started 通知参数(S0 实测形状的测试最小集)。
func startedParams(id, parentID, nickname string) string {
	raw := map[string]interface{}{"id": id, "parentThreadId": parentID, "agentNickname": nickname, "cwd": `C:\w`}
	if parentID == "" {
		delete(raw, "parentThreadId")
	}
	if nickname == "" {
		delete(raw, "agentNickname")
	}
	b, _ := json.Marshal(map[string]interface{}{"thread": raw})
	return string(b)
}

func TestSubagentStubRegisteredWithParentLink(t *testing.T) {
	s := newTestServer(t)
	parent := &store.Thread{ID: "th-main", CodexThreadID: "cx-parent", Title: "主会话"}
	if err := s.cfg.Store.Create(parent); err != nil {
		t.Fatal(err)
	}

	s.SyncThreadEvent("cx-child", "thread/started",
		json.RawMessage(startedParams("cx-child", "cx-parent", "审查者")))

	got, ok := s.cfg.Store.Get("cx-child")
	if !ok {
		t.Fatal("子代理线程应以存根形态登记(id = codex 子线程 id)")
	}
	if got.ParentThreadID != "th-main" {
		t.Fatalf("ParentThreadID 应是父的本地 id, got %q", got.ParentThreadID)
	}
	if got.CodexThreadID != "cx-child" {
		t.Fatalf("CodexThreadID 应回填 codex 子线程 id, got %q", got.CodexThreadID)
	}
	if got.Title != "审查者" {
		t.Fatalf("标题应取 agentNickname, got %q", got.Title)
	}
	if got.Status != pb.ThreadStatus_THREAD_STATUS_IDLE {
		t.Fatalf("存根状态应标记 IDLE, got %v", got.Status)
	}
	if len(got.Messages) != 0 {
		t.Fatalf("存根只登记元数据,不造内容, got %d 条消息", len(got.Messages))
	}
}

func TestSubagentStubNicknameFallbackTitle(t *testing.T) {
	s := newTestServer(t)
	parent := &store.Thread{ID: "th-main", CodexThreadID: "cx-parent", Title: "主会话"}
	if err := s.cfg.Store.Create(parent); err != nil {
		t.Fatal(err)
	}

	s.SyncThreadEvent("cx-child", "thread/started",
		json.RawMessage(startedParams("cx-child", "cx-parent", "")))

	got, _ := s.cfg.Store.Get("cx-child")
	if got == nil || got.Title != "子代理" {
		t.Fatalf("昵称缺省时标题应兜底为\"子代理\", got %+v", got)
	}
}

func TestSubagentStubSkippedForMainThreadStarted(t *testing.T) {
	// S0 实测:主线程的 thread/started parentThreadId 为空(字段缺省)——
	// 普通会话不登记任何东西,后续行为与从前完全一致。
	s := newTestServer(t)
	before := len(s.cfg.Store.List(true))

	s.SyncThreadEvent("cx-main", "thread/started",
		json.RawMessage(startedParams("cx-main", "", "")))

	if after := len(s.cfg.Store.List(true)); after != before {
		t.Fatalf("无父 id 的 thread/started 不得登记线程, before=%d after=%d", before, after)
	}
}

func TestSubagentStubSkippedWhenParentNotInStore(t *testing.T) {
	// 父会话从没被 App 接入:子对话无处归属,如实跳过,不造关系。
	s := newTestServer(t)

	s.SyncThreadEvent("cx-child", "thread/started",
		json.RawMessage(startedParams("cx-child", "cx-unknown", "审查者")))

	if _, ok := s.cfg.Store.Get("cx-child"); ok {
		t.Fatal("父不在本地库时不得登记存根")
	}
}

func TestSubagentStubRegistrationIdempotent(t *testing.T) {
	s := newTestServer(t)
	parent := &store.Thread{ID: "th-main", CodexThreadID: "cx-parent", Title: "主会话"}
	if err := s.cfg.Store.Create(parent); err != nil {
		t.Fatal(err)
	}
	params := json.RawMessage(startedParams("cx-child", "cx-parent", "审查者"))

	s.SyncThreadEvent("cx-child", "thread/started", params)
	s.SyncThreadEvent("cx-child", "thread/started", params) // 通知重放/并发重入

	if got, _ := s.cfg.Store.Get("cx-child"); got == nil || got.Title != "审查者" {
		t.Fatalf("重复登记必须幂等, got %+v", got)
	}
	if n := len(s.cfg.Store.List(true)); n != 2 {
		t.Fatalf("重复通知不得产生第二条记录, 共 %d 条", n)
	}
}

func TestSubagentStubLifecycleViaExistingSync(t *testing.T) {
	// 存根登记后,子线程的会话级事实(thread/status/changed、thread/deleted)
	// 经既有 SyncThreadEvent 路径自动命中存根 —— 生命周期零额外代码。
	s := newTestServer(t)
	parent := &store.Thread{ID: "th-main", CodexThreadID: "cx-parent", Title: "主会话"}
	if err := s.cfg.Store.Create(parent); err != nil {
		t.Fatal(err)
	}
	s.SyncThreadEvent("cx-child", "thread/started",
		json.RawMessage(startedParams("cx-child", "cx-parent", "审查者")))

	s.SyncThreadEvent("cx-child", "thread/status/changed",
		json.RawMessage(`{"threadId":"cx-child","status":{"type":"active"}}`))
	got, _ := s.cfg.Store.Get("cx-child")
	if got.Status != pb.ThreadStatus_THREAD_STATUS_RUNNING {
		t.Fatalf("状态同步应命中存根, got %v", got.Status)
	}

	s.SyncThreadEvent("cx-child", "thread/deleted", json.RawMessage(`{"threadId":"cx-child"}`))
	if _, ok := s.cfg.Store.Get("cx-child"); ok {
		t.Fatal("删除同步应命中存根并清理")
	}
}

func TestSubagentStubThreadSourceFallbackShape(t *testing.T) {
	// P0 报告的 SessionSource 变体(subAgent.thread_spawn):主字段缺位时从容错
	// 内层取父 id —— 真实流量若证明内层不存在,主字段路径已覆盖,本用例钉住
	// 的是"容错解析不误伤主字段"。
	s := newTestServer(t)
	parent := &store.Thread{ID: "th-main", CodexThreadID: "cx-parent", Title: "主会话"}
	if err := s.cfg.Store.Create(parent); err != nil {
		t.Fatal(err)
	}
	params := `{"thread":{"id":"cx-child","threadSource":{"type":"subAgent","subAgent":{"thread_spawn":{"depth":1,"parent_thread_id":"cx-parent","agent_nickname":"孵化器"}}}}}`

	s.SyncThreadEvent("cx-child", "thread/started", json.RawMessage(params))

	got, ok := s.cfg.Store.Get("cx-child")
	if !ok {
		t.Fatal("threadSource 兜底形状应登记存根")
	}
	if got.ParentThreadID != "th-main" || got.Title != "孵化器" {
		t.Fatalf("兜底解析不符: parent=%q title=%q", got.ParentThreadID, got.Title)
	}
}

func TestListThreadsCarriesParentThreadID(t *testing.T) {
	// 下游点亮钉子:thread/list 输出的 Thread 带 parent_thread_id(手机端
	// SubAgentsLogic.childrenOf 按它过滤出子代理组)。
	s := newTestServer(t)
	parent := &store.Thread{ID: "th-main", CodexThreadID: "cx-parent", Title: "主会话"}
	if err := s.cfg.Store.Create(parent); err != nil {
		t.Fatal(err)
	}
	stub := &store.Thread{
		ID: "cx-child", Title: "审查者", AgentID: "codex",
		CodexThreadID: "cx-child", ParentThreadID: "th-main",
		Status: pb.ThreadStatus_THREAD_STATUS_IDLE,
	}
	if err := s.cfg.Store.Create(stub); err != nil {
		t.Fatal(err)
	}

	resp, err := s.ListThreads(nil, &pb.ListThreadsRequest{})
	if err != nil {
		t.Fatalf("ListThreads: %v", err)
	}
	byID := map[string]*pb.Thread{}
	for _, th := range resp.Threads {
		byID[th.Id] = th
	}
	if got := byID["cx-child"]; got == nil || got.ParentThreadId != "th-main" {
		t.Fatalf("thread/list 应带出子线程的 parent_thread_id=th-main, got %+v", byID["cx-child"])
	}
	if got := byID["th-main"]; got == nil || got.ParentThreadId != "" {
		t.Fatalf("顶层会话 parent_thread_id 应为空, got %+v", got)
	}
}
