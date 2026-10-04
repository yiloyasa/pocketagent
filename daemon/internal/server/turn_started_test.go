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
	"context"
	"sync"
	"testing"
	"time"

	"github.com/pocketagent/daemon/internal/agent"
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

// ---- 2026-10-04 会话选择计划 Task 2:TurnSnapshot 冻结与 TurnStarted 实际值 ----
//
// 【为什么必须从 StartTurn 测】快照的价值在"调用点存在且在正确时点捕获"
// (与 turn_overrides_test.go 同思路:只测纯函数挡不住调用点被人删掉)。
// 三根钉子:
//  1. Started 帧带实际生效的 model/effort(configuration_confirmed 随覆盖置位);
//  2. 红线:轮次运行中改写线程选择,进行中轮次的 provider 配置与 Started 帧
//     都不变,改写只落库等下一轮("从下一轮生效"的字面语义);
//  3. 排队续跑是新的轮次边界:每轮重取快照,各自的 Started 帧反映各自的配置。

// selectionTurnProbe 节奏可控的探针 provider:记录 provider.Run 实际收到的线程
// (本轮真实生效配置),进入后等放行 —— 模拟"轮次正在跑"。
type selectionTurnProbe struct {
	id      string
	models  []pb.ModelInfo
	mu      sync.Mutex
	thread  *store.Thread
	entered chan struct{}
	release chan struct{}
}

func (p *selectionTurnProbe) ID() string             { return p.id }
func (p *selectionTurnProbe) DisplayName() string    { return "轮次快照探针" }
func (p *selectionTurnProbe) Models() []pb.ModelInfo { return append([]pb.ModelInfo(nil), p.models...) }

func (p *selectionTurnProbe) Run(ctx context.Context, req agent.TurnRequest, sink agent.Sink, _ agent.ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
	p.mu.Lock()
	p.thread = req.Thread
	p.mu.Unlock()
	p.entered <- struct{}{}
	select {
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	case <-p.release:
	}
	reply := "完成"
	sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_TextDelta{TextDelta: &pb.TextDelta{Delta: reply}}})
	return []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: reply}}}}, &pb.Usage{}, nil
}

func (p *selectionTurnProbe) seenThread() *store.Thread {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.thread
}

// newSelectionTurnServer 带探针 provider 与已存在线程(初始选择 pi/m1 + high)的 server。
func newSelectionTurnServer(t *testing.T, p *selectionTurnProbe) (*Server, *store.Thread) {
	t.Helper()
	fs, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("建测试存储失败: %v", err)
	}
	srv := New(Config{Store: fs, Providers: map[string]agent.Provider{p.ID(): p}, Workspace: t.TempDir()})
	th := &store.Thread{
		ID: "th-selturn-" + p.ID(), Title: "快照测试", Workspace: t.TempDir(),
		AgentID: p.ID(), ModelID: p.ID() + "/m1", Effort: "high",
		Status: pb.ThreadStatus_THREAD_STATUS_IDLE,
	}
	if err := fs.Create(th); err != nil {
		t.Fatalf("建线程失败: %v", err)
	}
	return srv, th
}

// selectionInput 造一条带模型/强度覆盖的 TurnInput 帧(手机端会话内切换的形状)。
func selectionInput(threadID, text, modelID, effort string) *pb.TurnClientMessage {
	return &pb.TurnClientMessage{Msg: &pb.TurnClientMessage_Input{Input: &pb.TurnInput{
		ThreadId: threadID,
		Blocks:   []*pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: text}}}},
		ModelId:  modelID,
		Effort:   effort,
	}}}
}

// firstStartedFrame 取第一帧 Started(断言失败信息带帧序)。
func firstStartedFrame(frames []*pb.TurnServerMessage) *pb.TurnStarted {
	for _, m := range frames {
		if s := m.GetStarted(); s != nil {
			return s
		}
	}
	return nil
}

// TestTurnStartedCarriesEffectiveModelEffortAndConfirmation 钉子 1:Started 帧
// 必须带实际生效的 model/effort;会话显式覆盖时 configuration_confirmed=true。
// (source 暂无 proto 字段,steward 待补 —— 见 model_selection.go startedFrame 注释。)
func TestTurnStartedCarriesEffectiveModelEffortAndConfirmation(t *testing.T) {
	p := &selectionTurnProbe{
		id: "pi", entered: make(chan struct{}, 4), release: make(chan struct{}, 4),
		models: []pb.ModelInfo{{Id: "m1", AvailableEfforts: []string{"low", "high"}}, {Id: "m2"}},
	}
	srv, th := newSelectionTurnServer(t, p)
	stream := &feedTurnStream{ctx: context.Background(), in: make(chan *pb.TurnClientMessage, 4)}
	stream.push(t, selectionInput(th.ID, "跑", "pi/m1", "high"))
	done := make(chan error, 1)
	go func() { done <- srv.StartTurn(stream) }()
	<-p.entered
	p.release <- struct{}{}
	close(stream.in)
	if err := <-done; err != nil {
		t.Fatalf("StartTurn: %v", err)
	}
	started := firstStartedFrame(stream.snapshot())
	if started == nil {
		t.Fatalf("应有 Started 帧,帧序: %s", frameNames(stream.snapshot()))
	}
	if started.TurnId == "" || started.ModelId != "pi/m1" {
		t.Fatalf("Started 应带轮次 id 与实际生效模型: %+v", started)
	}
	if started.Effort == nil || started.GetEffort() != "high" {
		t.Fatalf("Started 必须显式在场实际生效的 effort(optional 非缺席): %+v", started)
	}
	if !started.ConfigurationConfirmed {
		t.Fatal("会话显式覆盖应置 configuration_confirmed=true")
	}
}

// TestTurnSnapshotFrozenAgainstMidTurnSelectionWrite 钉子 2(全局约束红线):
// 轮次运行中改写线程选择 —— ① provider 消费的配置仍是启动那一刻的;
// ② Started 帧读快照,同样不变;③ 改写经 store 三方合并保留,等下一轮生效。
func TestTurnSnapshotFrozenAgainstMidTurnSelectionWrite(t *testing.T) {
	p := &selectionTurnProbe{
		id: "pi", entered: make(chan struct{}, 4), release: make(chan struct{}, 4),
		models: []pb.ModelInfo{{Id: "m1"}, {Id: "m2", AvailableEfforts: []string{"low"}}},
	}
	srv, th := newSelectionTurnServer(t, p)
	stream := &feedTurnStream{ctx: context.Background(), in: make(chan *pb.TurnClientMessage, 4)}
	stream.push(t, selectionInput(th.ID, "跑", "pi/m1", "high"))
	done := make(chan error, 1)
	go func() { done <- srv.StartTurn(stream) }()
	<-p.entered

	// 模拟"另一个选择写手"在轮次运行中改写线程选择(store.Get 拥有独立快照,
	// 三方合并允许并发写 —— 这正是运行中切换的唯一落库形态)
	cur, ok := srv.cfg.Store.Get(th.ID)
	if !ok {
		t.Fatal("线程不应丢失")
	}
	cur.ModelID = "pi/m2"
	cur.Effort = "low"
	if err := srv.cfg.Store.Update(cur); err != nil {
		t.Fatalf("运行中写入应被三方合并接受: %v", err)
	}

	p.release <- struct{}{}
	close(stream.in)
	if err := <-done; err != nil {
		t.Fatalf("StartTurn: %v", err)
	}

	// ① provider 消费的是冻结线程:模型/强度仍是启动那一刻的
	seen := p.seenThread()
	if seen == nil || seen.ModelID != "pi/m1" || seen.Effort != "high" {
		t.Fatalf("红线:进行中轮次配置被改写! provider 收到 model=%q effort=%q",
			seenThreadModel(seen), seenThreadEffort(seen))
	}
	// ② Started 帧读快照:与启动时刻一致,不受运行中写手影响
	started := firstStartedFrame(stream.snapshot())
	if started == nil || started.ModelId != "pi/m1" || started.GetEffort() != "high" {
		t.Fatalf("红线:Started 帧未反映冻结快照: %+v", started)
	}
	// ③ 运行中写入保留在库里(轮次自身没动模型/强度字段,三方合并不回滚写手)
	//    —— 这就是"切换只影响下一轮"的落库形态。
	got, ok := srv.cfg.Store.Get(th.ID)
	if !ok {
		t.Fatal("线程不应丢失")
	}
	if got.ModelID != "pi/m2" || got.Effort != "low" {
		t.Fatalf("运行中切换应落库等下一轮生效: model=%q effort=%q", got.ModelID, got.Effort)
	}
}

func seenThreadModel(t *store.Thread) string {
	if t == nil {
		return "<nil>"
	}
	return t.ModelID
}

func seenThreadEffort(t *store.Thread) string {
	if t == nil {
		return "<nil>"
	}
	return t.Effort
}

// TestQueuedTurnSnapshotRefreshesPerTurn 钉子 3:排队续跑 = 新的轮次边界,
// 各轮 Started 帧反映各自启动那一刻的配置(排队输入自带的覆盖在出队时生效)。
func TestQueuedTurnSnapshotRefreshesPerTurn(t *testing.T) {
	p := &selectionTurnProbe{
		id: "pi", entered: make(chan struct{}, 4), release: make(chan struct{}, 4),
		models: []pb.ModelInfo{{Id: "m1"}, {Id: "m2", AvailableEfforts: []string{"low"}}},
	}
	srv, th := newSelectionTurnServer(t, p)
	stream := &feedTurnStream{ctx: context.Background(), in: make(chan *pb.TurnClientMessage, 4)}
	stream.push(t, selectionInput(th.ID, "第一轮", "pi/m1", "high"))
	done := make(chan error, 1)
	go func() { done <- srv.StartTurn(stream) }()
	<-p.entered

	// 运行中带覆盖再发一条 → 排队(探针无 Steerer/FollowUpper,走 daemon 队列)
	stream.push(t, selectionInput(th.ID, "第二轮", "pi/m2", "low"))
	waitFor(t, "Queued 回执", func() bool {
		return countFrame(stream.snapshot(), func(m *pb.TurnServerMessage) bool { return m.GetQueued() != nil }) == 1
	})
	p.release <- struct{}{} // 第一轮终局
	<-p.entered             // 续跑轮进入
	p.release <- struct{}{} // 续跑轮终局
	close(stream.in)
	if err := <-done; err != nil {
		t.Fatalf("StartTurn: %v", err)
	}

	var starteds []*pb.TurnStarted
	for _, m := range stream.snapshot() {
		if s := m.GetStarted(); s != nil {
			starteds = append(starteds, s)
		}
	}
	if len(starteds) != 2 {
		t.Fatalf("应有两帧 Started,得到 %d(帧序: %s)", len(starteds), frameNames(stream.snapshot()))
	}
	if starteds[0].ModelId != "pi/m1" || starteds[0].GetEffort() != "high" {
		t.Fatalf("第一轮 Started 应反映 m1/high: %+v", starteds[0])
	}
	if starteds[1].ModelId != "pi/m2" || starteds[1].GetEffort() != "low" {
		t.Fatalf("第二轮 Started 应反映排队覆盖 m2/low: %+v", starteds[1])
	}
	if !starteds[1].ConfigurationConfirmed {
		t.Fatal("排队覆盖同样是会话显式选择,应置 configuration_confirmed=true")
	}
}
