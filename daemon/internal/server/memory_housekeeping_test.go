// memory_housekeeping_test.go —— P1「轮末自动收敛 + 预算接线」的钉子
// （memory 分层设计稿 §4 P1：让 Compact 第一次在生产路径被调用、让预算第一次对 memory 说话）。
//
// 【要验什么】四件事，各自独立成钉，断哪根都不会被其余的绿掩盖：
//  1. 轮末钩子：跑完一轮后，超阈值的 conversation 槽真的被 Compact（Covered/Digest 非空、
//     摘要槽 kinds["summary"] 从 0 变非 0），且完整性判据（不静默丢 sequence）仍绿、
//     轮次照常返回（收敛失败只留日志的前提是它根本不该失败到影响返回）；
//  2. 软限联动：条数没到阈值、但 ResourceLedger 报软压时，轮末同样触发收敛；
//     且重复触发是幂等的（第二次零副作用——不重复收敛、不重复追加摘要行）；
//  3. 硬限拒可选写：hybrid 家的轮次索引是可选资源，超硬限后新索引被拒在 Append 之前
//     （不是"记完账再照写"），轮次本身不受影响；
//  4. 硬限不拒必需写：conversation 原文是必需资源，超硬限照常写入，越限只在 Usage 里
//     如实可见（复用 backend.go Acquire/Charge 的 Optional 语义，不新发明预算机制）。
//
// 【为什么预置条目走真实写链】预算钉子里的条目用 s.recordTurnMemory 直写（真实链含
// 记账），只有"阈值钉子"的垫床条目直接走 Memory().Append——它测的是水位判定本身，
// 与预算无关；混用记账路径反而让"触发到底来自阈值还是预算"变得不可分辨。
package server

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/store"
)

// memoryProseBlocks 造若干纯文本内容块（记忆原文钉子的输入）。
func memoryProseBlocks(texts ...string) []pb.ContentBlock {
	out := make([]pb.ContentBlock, 0, len(texts))
	for _, s := range texts {
		out = append(out, pb.ContentBlock{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: s}}})
	}
	return out
}

// memoryTestThreadID 预算钉子用的线程 id：确定性派生，让"先造策略、再建 server"
// 成为可能（预算策略按 thread id 覆盖，而策略必须在 New 之前给）。
func memoryTestThreadID(p agent.Provider) string { return "th-mem-" + p.ID() }

// budgetPolicyForThread 给指定线程的某类资源额度打 thread 级覆盖（其余字段走内置基线）。
func budgetPolicyForThread(threadID string, kind runtime.ResourceKind, quota int64) runtime.BudgetPolicy {
	lim := runtime.Limits{}
	if kind == runtime.ResConversation {
		lim.DiskConversationBytes = &quota
	} else {
		lim.DiskArtifactBytes = &quota
	}
	return runtime.BudgetPolicy{Threads: map[string]runtime.Limits{threadID: lim}}
}

// newTurnMemoryServerWithPolicy 同 newTurnMemoryServer，但可注入预算策略
// （预算联动的钉子需要 thread 级的小额度覆盖；零值策略 = 全部走内置基线）。
func newTurnMemoryServerWithPolicy(t *testing.T, p agent.Provider, policy runtime.BudgetPolicy) (*Server, *store.Thread) {
	t.Helper()
	dir := t.TempDir()
	fs, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("建 store 失败: %v", err)
	}
	srv := New(Config{
		Store:          fs,
		Providers:      map[string]agent.Provider{p.ID(): p},
		RuntimeBackend: runtime.NewFileBackend(dir),
		BudgetPolicy:   policy,
	})
	t.Cleanup(func() { _ = srv.events.Close(); srv.stopIdleMemorySweep() })
	thID := memoryTestThreadID(p)
	if err := fs.Create(&store.Thread{ID: thID, Title: "记忆收敛", Workspace: t.TempDir()}); err != nil {
		t.Fatalf("建线程失败: %v", err)
	}
	th, ok := fs.Get(thID)
	if !ok {
		t.Fatal("线程丢了")
	}
	return srv, th
}

// mustLoadMemory 取记忆快照，失败即 fatal（读不出来时后续断言全是噪音）。
func mustLoadMemory(t *testing.T, srv *Server, key runtime.MemoryKey) *runtime.MemorySnapshot {
	t.Helper()
	snap, err := srv.runtimeBackend.Memory().Load(context.Background(), key)
	if err != nil {
		t.Fatalf("读记忆 %s 失败: %v", key, err)
	}
	return snap
}

// assertTurnCompleted 断言最后一帧是 Completed——轮次返回不受收敛影响的可见凭据。
func assertTurnCompleted(t *testing.T, stream *fakeTurnStream) {
	t.Helper()
	if len(stream.sent) == 0 {
		t.Fatal("轮次没有发出任何帧")
	}
	if last := stream.sent[len(stream.sent)-1]; last.GetCompleted() == nil {
		t.Fatalf("最后一帧应为 Completed，实得 %+v", last.Msg)
	}
}

// seedConversation 直接向 conversation 槽垫 n 条 1 字节条目（只做阈值钉子的垫床）。
func seedConversation(t *testing.T, srv *Server, key runtime.MemoryKey, n int) {
	t.Helper()
	ctx := context.Background()
	for i := 0; i < n; i++ {
		if _, err := srv.runtimeBackend.Memory().Append(ctx, key, runtime.MemoryItem{
			Kind: runtime.MemoryConversation, Role: "user", Payload: []byte("x"),
		}); err != nil {
			t.Fatalf("垫床条目 %d 写入失败: %v", i, err)
		}
	}
}

// usageContains 报告 OverSoft/OverHard 名单是否含指定租约（手工循环：名单很短，
// 引 slices 只为一处 contains 得不偿失）。
func usageContains(list []string, leaseID string) bool {
	for _, id := range list {
		if id == leaseID {
			return true
		}
	}
	return false
}

// TestTurnEndCompactsConversationSlot：钉 1——轮末钩子。
// conversation 槽越过阈值（条数 > 200）后，一轮跑完必须真的被 Compact：
// Covered/Digest 非空、保留项收敛到 KeepRecentItems、摘要槽从 0 变非 0，
// 完整性判据不丢 sequence，轮次照常 Completed，且经既有 GetAgentRuntime 可见。
func TestTurnEndCompactsConversationSlot(t *testing.T) {
	p := &memShapeProvider{id: "hk1", model: runtime.MemoryDaemon, reply: "回答正文"}
	srv, th := newTurnMemoryServerWithPolicy(t, p, runtime.BudgetPolicy{})
	ctx := context.Background()
	key := runtime.MemoryKey{AgentID: p.ID(), ThreadID: th.ID, Kind: runtime.MemoryConversation}
	seedConversation(t, srv, key, 201) // 垫过阈值（>200）的床

	stream := runSimpleTurn(t, srv, th.ID)
	assertTurnCompleted(t, stream) // 收敛不得影响轮次交付

	snap := mustLoadMemory(t, srv, key)
	if len(snap.Items) != compactKeepRecentItems {
		t.Fatalf("收敛后应保留最近 %d 条，实得 %d（轮末 Compact 没跑？）", compactKeepRecentItems, len(snap.Items))
	}
	if snap.Covered.Empty() {
		t.Fatal("收敛后 Covered 必须非空——那是\"没静默丢数据\"的唯一凭据")
	}
	if snap.Digest == "" {
		t.Fatal("收敛后 Digest 必须非空（结构摘要，零模型调用）")
	}
	// 完整性判据：垫床 201 条 + 本轮 user/assistant 共 203 个 sequence，一个都不能凭空消失。
	seqs := make([]int64, 0, snap.Covered.To+2)
	for i := int64(1); i <= snap.Covered.To+int64(len(snap.Items)); i++ {
		seqs = append(seqs, i)
	}
	if err := runtime.CheckMemoryIntegrity(snap, seqs); err != nil {
		t.Fatalf("轮末收敛破坏了记忆完整性: %v", err)
	}
	// 摘要槽从 0 变非 0：结构摘要行登记进 summary 槽（kinds["summary"] 的唯一来源）。
	summarySnap := mustLoadMemory(t, srv, runtime.MemoryKey{AgentID: p.ID(), ThreadID: th.ID, Kind: runtime.MemorySummary})
	if len(summarySnap.Items) == 0 {
		t.Fatal("收敛应把结构摘要行登记进 summary 槽（否则 kinds[\"summary\"] 恒为 0，手机端零改动可见就是空话）")
	}
	for i, it := range summarySnap.Items {
		if it.Kind != runtime.MemorySummary || it.Meta[metaEntryKind] != entryCompactDigest {
			t.Fatalf("摘要槽第 %d 条必须自我描述 entry=compact_digest，实得 kind=%s meta=%v", i, it.Kind, it.Meta)
		}
		if it.Meta[metaCoveredFrom] == "" || it.Meta[metaCoveredTo] == "" || it.Meta[metaSourceKind] == "" {
			t.Fatalf("摘要槽第 %d 条应带覆盖范围与来源槽，实得 meta=%v", i, it.Meta)
		}
	}
	// 观测面（零协议变更）：效果经既有 GetAgentRuntime 的 memory 观测可见。
	resp, err := srv.GetAgentRuntime(ctx, &pb.GetAgentRuntimeRequest{AgentId: p.ID()})
	if err != nil {
		t.Fatalf("GetAgentRuntime 失败: %v", err)
	}
	if got := resp.GetMemory().GetKinds()["summary"]; got < 1 {
		t.Fatalf("GetAgentRuntime 应看到 kinds[summary]>=1，实得 %d（kinds=%v）", got, resp.GetMemory().GetKinds())
	}
}

// TestTurnEndCompactsOnSoftBudgetPressure：钉 2——预算软限触发收敛 + 幂等。
// 条数（62）远低于阈值（200），但租约用量越过软限：轮末必须由预算信号触发收敛；
// 收敛之后再触发一次（软压仍在——P1 租约记累计写入量），必须零副作用。
func TestTurnEndCompactsOnSoftBudgetPressure(t *testing.T) {
	p := &memShapeProvider{id: "hk2", model: runtime.MemoryDaemon, reply: "回答正文"}
	// 软限 = 额度/2 = 60，硬限 = 120。60 条 1 字节垫床 + 本轮 2+12 字节 = 74 ∈ [60,120)
	// ——软压成立、硬限不成立，触发源只能是软限信号。
	srv, th := newTurnMemoryServerWithPolicy(t, p,
		budgetPolicyForThread(memoryTestThreadID(p), runtime.ResConversation, 120))
	ctx := context.Background()
	key := runtime.MemoryKey{AgentID: p.ID(), ThreadID: th.ID, Kind: runtime.MemoryConversation}
	for i := 0; i < 60; i++ {
		// 走真实写链（含预算记账），每条 1 字节。
		srv.recordTurnMemory(p, th, "user", fmt.Sprintf("seed-%04d", i), memoryProseBlocks("x"))
	}
	leaseID := memoryLeaseID(p.ID(), th.ID, runtime.MemoryConversation)
	rep := srv.runtimeBackend.Resources().Usage(ctx, runtime.ScopeThread(th.ID))
	if !usageContains(rep.OverSoft, leaseID) {
		t.Fatalf("前置条件不成立：软压应已出现在 Usage.OverSoft（实得 OverSoft=%v Items=%+v）", rep.OverSoft, rep.Items)
	}

	stream := runSimpleTurn(t, srv, th.ID)
	assertTurnCompleted(t, stream)

	snap := mustLoadMemory(t, srv, key)
	if snap.Covered.Empty() || snap.Digest == "" {
		t.Fatalf("软限必须触发轮末收敛：实得 covered=%+v digest=%q items=%d", snap.Covered, snap.Digest, len(snap.Items))
	}
	if len(snap.Items) != compactKeepRecentItems {
		t.Fatalf("收敛后应保留最近 %d 条，实得 %d", compactKeepRecentItems, len(snap.Items))
	}
	seqs := make([]int64, 0, 64)
	for i := int64(1); i <= int64(len(snap.Items))+snap.Covered.To; i++ {
		seqs = append(seqs, i)
	}
	if err := runtime.CheckMemoryIntegrity(snap, seqs); err != nil {
		t.Fatalf("软限收敛破坏了记忆完整性: %v", err)
	}

	// 幂等钉子：软压仍在（租约记累计写入量），再触发一次必须零副作用——
	// 不重复收敛（Covered/Digest/Version 不动）、不重复追加摘要行。
	summaryKey := runtime.MemoryKey{AgentID: p.ID(), ThreadID: th.ID, Kind: runtime.MemorySummary}
	before, beforeSummary := mustLoadMemory(t, srv, key), mustLoadMemory(t, srv, summaryKey)
	srv.afterTurnMemoryHousekeeping(p, th)
	srv.afterTurnMemoryHousekeeping(p, th)
	after, afterSummary := mustLoadMemory(t, srv, key), mustLoadMemory(t, srv, summaryKey)
	if after.Version != before.Version || after.Covered != before.Covered || after.Digest != before.Digest || len(after.Items) != len(before.Items) {
		t.Fatalf("重复触发必须幂等：before(version=%d covered=%+v items=%d) after(version=%d covered=%+v items=%d)",
			before.Version, before.Covered, len(before.Items), after.Version, after.Covered, len(after.Items))
	}
	if len(afterSummary.Items) != len(beforeSummary.Items) {
		t.Fatalf("重复触发不得重复追加摘要行：before=%d after=%d", len(beforeSummary.Items), len(afterSummary.Items))
	}
}

// TestHardBudgetRejectsOptionalIndex：钉 3——硬限拒掉 hybrid 家的可选轮次索引。
// 拒绝必须发生在 Append 之前（槽里看不到新条目），且轮次照常完成（拒绝不拦轮次）。
func TestHardBudgetRejectsOptionalIndex(t *testing.T) {
	p := &memShapeProvider{id: "hk3", model: runtime.MemoryHybrid, reply: "回答正文"}
	payload, _, ok := turnIndexEntry("seed-1", "user", memoryProseBlocks("x"))
	if !ok || len(payload) <= 1 {
		t.Fatalf("前置条件不成立：索引条目应可构造且非平凡（ok=%v size=%d)", ok, len(payload))
	}
	// 硬限 = 2×条目-1：第一条进得来（used < hard），第二条记不上去（next >= hard）。
	quota := int64(2*len(payload) - 1)
	srv, th := newTurnMemoryServerWithPolicy(t, p, budgetPolicyForThread(memoryTestThreadID(p), runtime.ResArtifactDisk, quota))
	srv.recordTurnMemory(p, th, "user", "seed-1", memoryProseBlocks("x"))
	key := runtime.MemoryKey{AgentID: p.ID(), ThreadID: th.ID, Kind: runtime.MemoryArtifact}
	if got := mustLoadMemory(t, srv, key); len(got.Items) != 1 {
		t.Fatalf("前置条件不成立：第一条索引应写入成功，实得 %d 条", len(got.Items))
	}
	// 第二条：租约已存在 → Charge 撞硬限 → 这条可选索引必须被拒在写入之前。
	srv.recordTurnMemory(p, th, "assistant", "seed-2", memoryProseBlocks("y"))
	snap := mustLoadMemory(t, srv, key)
	if len(snap.Items) != 1 || snap.Items[0].Meta[metaTurnID] != "seed-1" {
		t.Fatalf("超硬限后可选索引应被拒（仍应只有 seed-1 一条），实得 %d 条（首条 turn_id=%s）",
			len(snap.Items), snap.Items[0].Meta[metaTurnID])
	}
	// 真实轮次在硬限之上照常完成（其索引写入同样被拒，不拦轮次）。
	stream := runSimpleTurn(t, srv, th.ID)
	assertTurnCompleted(t, stream)
	snap = mustLoadMemory(t, srv, key)
	if len(snap.Items) != 1 {
		t.Fatalf("真实轮次的索引写入同样应被硬限拒绝，实得 %d 条", len(snap.Items))
	}
}

// TestHardBudgetNeverRejectsConversationProse：钉 4——必需写照常放行。
// conversation 原文是必需资源（Optional=false）：越过硬限后本轮 user/assistant 原文
// 仍全部入槽，越限只体现在 Usage.OverHard 里（压力如实可见，不静默丢弃）。
func TestHardBudgetNeverRejectsConversationProse(t *testing.T) {
	p := &memShapeProvider{id: "hk4", model: runtime.MemoryDaemon, reply: "回答正文"}
	// 额度 8（软 4/硬 8）：两条垫床 4 字节 + 本轮 2+12 字节 = 18 ≥ 硬限，越限成立。
	srv, th := newTurnMemoryServerWithPolicy(t, p, budgetPolicyForThread(memoryTestThreadID(p), runtime.ResConversation, 8))
	key := runtime.MemoryKey{AgentID: p.ID(), ThreadID: th.ID, Kind: runtime.MemoryConversation}
	srv.recordTurnMemory(p, th, "user", "seed-1", memoryProseBlocks("hi"))
	srv.recordTurnMemory(p, th, "user", "seed-2", memoryProseBlocks("hi"))

	stream := runSimpleTurn(t, srv, th.ID)
	assertTurnCompleted(t, stream)

	snap := mustLoadMemory(t, srv, key)
	if len(snap.Items) != 4 {
		t.Fatalf("必需写不得被硬限拒绝（2 条垫床 + 本轮 2 条），实得 %d 条", len(snap.Items))
	}
	last := snap.Items[len(snap.Items)-1]
	if last.Role != "assistant" || !strings.Contains(string(last.Payload), "回答正文") {
		t.Fatalf("本轮 assistant 原文必须照常入槽，实得 role=%q payload=%q", last.Role, last.Payload)
	}
	rep := srv.runtimeBackend.Resources().Usage(context.Background(), runtime.ScopeThread(th.ID))
	if !usageContains(rep.OverHard, memoryLeaseID(p.ID(), th.ID, runtime.MemoryConversation)) {
		t.Fatalf("越限必需写应在 Usage.OverHard 如实可见（OverHard=%v Items=%+v）", rep.OverHard, rep.Items)
	}
}

// TestClearThreadMemoryReleasesBudgetLeases：清空会话记忆后，预算租约一并退出——
// 否则手机端会看到"记忆已清零、压力还满格"的自相矛盾（预算联动的账要跟得上事实）。
func TestClearThreadMemoryReleasesBudgetLeases(t *testing.T) {
	p := &memShapeProvider{id: "hk5", model: runtime.MemoryDaemon, reply: "回答正文"}
	srv, th := newTurnMemoryServerWithPolicy(t, p, runtime.BudgetPolicy{})
	runSimpleTurn(t, srv, th.ID) // 产生 conversation 记忆 + 租约
	ctx := context.Background()
	if _, err := srv.ClearThreadMemory(ctx, &pb.ClearThreadMemoryRequest{AgentId: p.ID(), ThreadId: th.ID}); err != nil {
		t.Fatalf("清空会话记忆失败: %v", err)
	}
	rep := srv.runtimeBackend.Resources().Usage(ctx, runtime.ScopeThread(th.ID))
	for _, it := range rep.Items {
		if it.Kind == runtime.ResConversation || it.Kind == runtime.ResArtifactDisk {
			t.Fatalf("清空记忆后不该残留记忆类租约: %+v", rep.Items)
		}
	}
}

// ---------- P3：闲时整理 ----------

// newIdleSweepServer 造一个带两个 daemon 家 provider 的测试 server，各自有一条
// 归属明确的线程（AgentID 必填：闲时扫描按 threadAgentID→Providers 反查，不像
// runSimpleTurn 走旧库兼容路径）。返回 (server, 两个线程, 置旧闲时钟的快捷方式)。
func newIdleSweepServer(t *testing.T) (*Server, *store.Thread, *store.Thread) {
	t.Helper()
	fs, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("建 store 失败: %v", err)
	}
	p1 := &memShapeProvider{id: "idle1", model: runtime.MemoryDaemon, reply: "回答正文"}
	p2 := &memShapeProvider{id: "idle2", model: runtime.MemoryDaemon, reply: "回答正文"}
	srv := New(Config{
		Store:          fs,
		Providers:      map[string]agent.Provider{p1.ID(): p1, p2.ID(): p2},
		RuntimeBackend: runtime.NewFileBackend(t.TempDir()),
	})
	t.Cleanup(func() { _ = srv.events.Close() })
	mk := func(id, agentID string) *store.Thread {
		th := &store.Thread{ID: id, AgentID: agentID, Title: "闲时整理", Workspace: t.TempDir()}
		if err := fs.Create(th); err != nil {
			t.Fatalf("建线程失败: %v", err)
		}
		got, ok := fs.Get(id)
		if !ok {
			t.Fatal("线程丢了")
		}
		return got
	}
	th1, th2 := mk("th-idle-1", p1.ID()), mk("th-idle-2", p2.ID())
	return srv, th1, th2
}

// makeIdle 让该 server 看起来已空闲超过阈值（把闲时钟拨到阈值之前）。
func makeIdle(srv *Server) {
	srv.lastTurnSettledAt.Store(time.Now().Add(-idleMemoryIdleAfter() - time.Minute).UnixNano())
}

// TestIdleSweepCompactsLargestOverThresholdThread：闲时触发钉——
// 空闲超阈值后，一次 sweep 只收「超阈幅度最大」的那一个会话，且走的是 P1 同一钩子
// （同参数、同幂等闸门、同摘要行登记）。
func TestIdleSweepCompactsLargestOverThresholdThread(t *testing.T) {
	srv, th1, th2 := newIdleSweepServer(t)
	p1 := srv.cfg.Providers[threadAgentID(th1)]
	p2 := srv.cfg.Providers[threadAgentID(th2)]
	// th1 超阈 1 条，th2 超阈 60 条：sweep 必须选 th2。
	key1 := runtime.MemoryKey{AgentID: p1.ID(), ThreadID: th1.ID, Kind: runtime.MemoryConversation}
	key2 := runtime.MemoryKey{AgentID: p2.ID(), ThreadID: th2.ID, Kind: runtime.MemoryConversation}
	seedConversation(t, srv, key1, 201)
	seedConversation(t, srv, key2, 260)
	makeIdle(srv)

	srv.idleMemorySweep()

	after1 := mustLoadMemory(t, srv, key1)
	if len(after1.Items) != 201 || !after1.Covered.Empty() {
		t.Fatalf("超阈较小者不该被这次 sweep 收敛（一次只会话），实得 items=%d covered=%+v",
			len(after1.Items), after1.Covered)
	}
	after2 := mustLoadMemory(t, srv, key2)
	if len(after2.Items) != compactKeepRecentItems || after2.Covered.Empty() || after2.Digest == "" {
		t.Fatalf("超阈最大者应被收敛到轮末同参数（保留 %d 条、Covered/Digest 非空），实得 items=%d covered=%+v",
			compactKeepRecentItems, len(after2.Items), after2.Covered)
	}
	// 复用 P1 钩子的凭据：摘要行登记进了 summary 槽（同一登记链路）。
	summary2 := mustLoadMemory(t, srv, runtime.MemoryKey{AgentID: p2.ID(), ThreadID: th2.ID, Kind: runtime.MemorySummary})
	if len(summary2.Items) == 0 {
		t.Fatal("闲时收敛应与轮末一样把摘要行登记进 summary 槽（复用 P1 的证据）")
	}
}

// TestIdleSweepSkipsWhenBusyOrRecent：闲时判定的两道闸——有在飞轮次、或距上次轮次
// 收尾还没到阈值，超阈也不动（闲时整理绝不与轮次抢时机）。
func TestIdleSweepSkipsWhenBusyOrRecent(t *testing.T) {
	srv, _, th2 := newIdleSweepServer(t)
	p2 := srv.cfg.Providers[threadAgentID(th2)]
	key2 := runtime.MemoryKey{AgentID: p2.ID(), ThreadID: th2.ID, Kind: runtime.MemoryConversation}
	seedConversation(t, srv, key2, 201)

	// 闸 1：有在飞轮次（哪怕闲时钟已旧）。
	makeIdle(srv)
	lease, err := srv.activeTurns.acquire(context.Background(), th2.ID, func() {})
	if err != nil {
		t.Fatalf("占轮次租约失败: %v", err)
	}
	srv.idleMemorySweep()
	srv.activeTurns.Clear(th2.ID, lease)
	if got := mustLoadMemory(t, srv, key2); len(got.Items) != 201 || !got.Covered.Empty() {
		t.Fatalf("有在飞轮次时 sweep 必须不动: items=%d covered=%+v", len(got.Items), got.Covered)
	}

	// 闸 2：距上次轮次收尾未到阈值（刚忙完）。
	srv.lastTurnSettledAt.Store(time.Now().UnixNano())
	srv.idleMemorySweep()
	if got := mustLoadMemory(t, srv, key2); len(got.Items) != 201 || !got.Covered.Empty() {
		t.Fatalf("未到空闲阈值时 sweep 必须不动: items=%d covered=%+v", len(got.Items), got.Covered)
	}
}

// TestIdleSweepTickerFires：ticker 接线钉——首个轮次惰性启动的整理循环，在
// 空闲超阈值后会真的触发 sweep（轮询等待，避免裸 sleep 的时序脆断）。
func TestIdleSweepTickerFires(t *testing.T) {
	srv, _, th2 := newIdleSweepServer(t)
	p2 := srv.cfg.Providers[threadAgentID(th2)]
	key2 := runtime.MemoryKey{AgentID: p2.ID(), ThreadID: th2.ID, Kind: runtime.MemoryConversation}
	seedConversation(t, srv, key2, 201)
	makeIdle(srv)

	// 收紧 ticker 周期（包级原子配置=任务说的"选简单的不加 proto 字段"的开关位；
	// 用完即还原，否则后面所有测试的整理循环都按 10ms 空转）。
	oldTick := idleMemorySweepTickNanos.Load()
	idleMemorySweepTickNanos.Store(int64(10 * time.Millisecond))
	t.Cleanup(func() {
		idleMemorySweepTickNanos.Store(oldTick)
		srv.stopIdleMemorySweep()
		// 先停未来的 tick,再等整理循环 goroutine 真正退出(含在飞那次 sweep 的
		// Compact/Checkpoint 落盘)——否则异步 checkpoint 撞上 t.TempDir() 的
		// RemoveAll(批次 2 实测:"checkpoints: The directory is not empty")。
		srv.waitIdleMemorySweepStopped()
	})

	srv.ensureIdleMemorySweep()
	deadline := time.Now().Add(2 * time.Second)
	for {
		snap := mustLoadMemory(t, srv, key2)
		if !snap.Covered.Empty() {
			break // 收敛已发生
		}
		if time.Now().After(deadline) {
			t.Fatal("2s 内 ticker 没有触发闲时收敛（整理循环没接线？）")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// ---------- P3：artifact 独立登记 ----------

// TestTurnRegistersLargeToolResultArtifact：artifact 登记钉——
// hybrid 家的消息里出现超过阈值的大块 tool_result 时，除既有的轮次索引行外，
// 每块产物独立登记一行（Meta[entry]=artifact，spec §6.3 在 daemon 侧可得的范围：
// id / 来源轮次 / 字节 / 形状指纹；正文仍只在原生侧，daemon 不留第二份）。
func TestTurnRegistersLargeToolResultArtifact(t *testing.T) {
	p := &memShapeProvider{id: "art1", model: runtime.MemoryHybrid, reply: "回答正文"}
	srv, th := newTurnMemoryServerWithPolicy(t, p, runtime.BudgetPolicy{})
	big := strings.Repeat("X", artifactEntryMinBytes+100)
	blocks := append(memoryProseBlocks("问题正文"), pb.ContentBlock{
		Block: &pb.ContentBlock_ToolResult{ToolResult: &pb.ToolResultBlock{
			CallId: "call-9", Ok: true, Output: big,
		}},
	})

	srv.recordTurnMemory(p, th, "assistant", "turn-77", blocks)

	key := runtime.MemoryKey{AgentID: p.ID(), ThreadID: th.ID, Kind: runtime.MemoryArtifact}
	snap := mustLoadMemory(t, srv, key)
	if len(snap.Items) != 2 {
		t.Fatalf("应有索引行 + 产物行两条，实得 %d 条: %+v", len(snap.Items), snap.Items)
	}
	var art *runtime.MemoryItem
	for i := range snap.Items {
		if snap.Items[i].Meta[metaEntryKind] == entryArtifact {
			art = &snap.Items[i]
		}
	}
	if art == nil {
		t.Fatalf("找不到 entry=artifact 的独立产物行: %+v", snap.Items)
	}
	if art.Meta[metaTurnID] != "turn-77" {
		t.Fatalf("产物行应带来源轮次，实得 meta=%v", art.Meta)
	}
	if got := art.Meta[metaBytes]; got != itoaLen(len(big)) {
		t.Fatalf("产物行应带字节（实得 %s，want %d）", got, len(big))
	}
	if art.Meta[metaArtifactID] == "" || art.Meta[metaSHA256] == "" {
		t.Fatalf("产物行应带 artifact_id 与形状指纹，实得 meta=%v", art.Meta)
	}
	// 确定论：同一产物两次登记得到同一 id（内容指纹派生，重复产物可对账）。
	_, meta2, ok := artifactEntryRow("turn-78", &pb.ContentBlock{
		Block: &pb.ContentBlock_ToolResult{ToolResult: &pb.ToolResultBlock{CallId: "call-x", Output: big}},
	})
	if !ok || meta2[metaArtifactID] != art.Meta[metaArtifactID] {
		t.Fatalf("同一产物应派生同一 artifact_id: %s vs %s", meta2[metaArtifactID], art.Meta[metaArtifactID])
	}
	// 正文红线：产物行不得携带工具输出正文（daemon 不留第二份正文）。
	if strings.Contains(string(art.Payload), strings.Repeat("X", 64)) {
		t.Fatalf("产物行不得携带正文（hybrid 正文只在原生侧）: %q", art.Payload)
	}
}

// TestSmallToolResultSkipsArtifactRow：小输出不值得独立成行（索引行已有形状指纹）。
func TestSmallToolResultSkipsArtifactRow(t *testing.T) {
	p := &memShapeProvider{id: "art2", model: runtime.MemoryHybrid, reply: "回答正文"}
	srv, th := newTurnMemoryServerWithPolicy(t, p, runtime.BudgetPolicy{})
	blocks := append(memoryProseBlocks("问题正文"), pb.ContentBlock{
		Block: &pb.ContentBlock_ToolResult{ToolResult: &pb.ToolResultBlock{
			CallId: "call-1", Ok: true, Output: "tiny",
		}},
	})

	srv.recordTurnMemory(p, th, "assistant", "turn-1", blocks)

	snap := mustLoadMemory(t, srv, runtime.MemoryKey{AgentID: p.ID(), ThreadID: th.ID, Kind: runtime.MemoryArtifact})
	for _, it := range snap.Items {
		if it.Meta[metaEntryKind] == entryArtifact {
			t.Fatalf("小输出不该独立登记产物行: meta=%v", it.Meta)
		}
	}
}

// itoaLen 测试专用十进制（避免为断言引 strconv 到处跑）。
func itoaLen(n int) string { return fmt.Sprintf("%d", n) }
