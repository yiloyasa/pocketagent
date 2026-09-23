// turn_memory_test.go —— D7「所有 Agent 接入统一 Runtime」的接线回归。
//
// 【要验什么】不是"存在一个能写记忆的函数"，而是"跑完一轮之后，这家真的在统一
// Runtime 里留下了痕迹"。中间任何一环断掉都只有端到端断言能抓住：没接线（跑完
// 还是零）、分错了记忆形态（该写索引的写了全文）、checkpoint 没落盘（重启后归零）。
//
// 【为什么用形态替身当样本，而不是拿 echo】echo 在本轮之前是自己在 Run 里写记忆的
// （provider 内 recordTurn）——拿它当样本时，把 server 侧接线整个删掉断言仍然绿，
// 测不出"接线没了"。所以这里用**自己完全不碰统一 Runtime** 的替身，让"轮次之后
// 记忆里有没有东西"唯一地由 server 侧接线决定。真 echo 另有一条用例（文件末尾）。
package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/store"
)

// memShapeProvider 只申报记忆形态；Run 里**不碰统一 Runtime**（见文件头）。
type memShapeProvider struct {
	id    string
	model runtime.MemoryModel
	reply string
}

func (p *memShapeProvider) ID() string             { return p.id }
func (p *memShapeProvider) DisplayName() string    { return "记忆形态替身" }
func (p *memShapeProvider) Models() []pb.ModelInfo { return nil }

func (p *memShapeProvider) Run(_ context.Context, _ agent.TurnRequest, _ agent.Sink, _ agent.ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
	return []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: p.reply}}}}, &pb.Usage{}, nil
}

func (p *memShapeProvider) Descriptor() runtime.AgentDescriptor {
	return runtime.AgentDescriptor{ID: p.id, MemoryModel: p.model}
}

func (p *memShapeProvider) OpenRuntime(_ context.Context, _ *store.Thread) (*runtime.AgentRuntime, error) {
	return nil, nil // 本文件只验 server 侧接线，不交付运行时
}

// newTurnMemoryServer 造一个带**文件后端**的测试 server（checkpoint 是否落盘可核验），
// 并预先落一条线程。返回 (server, 线程 id, 数据目录)。
func newTurnMemoryServer(t *testing.T, provider agent.Provider) (*Server, string, string) {
	t.Helper()
	dir := t.TempDir()
	fs, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("建 store 失败: %v", err)
	}
	srv := New(Config{
		Store:          fs,
		Providers:      map[string]agent.Provider{provider.ID(): provider},
		RuntimeBackend: runtime.NewFileBackend(dir),
	})
	t.Cleanup(func() { _ = srv.events.Close() })
	thID := "th-mem-" + provider.ID()
	if err := fs.Create(&store.Thread{ID: thID, Title: "记忆接线", Workspace: t.TempDir()}); err != nil {
		t.Fatalf("建线程失败: %v", err)
	}
	return srv, thID, dir
}

// assertCheckpointFileExists 断言该 agent 名下至少有一份落盘的 memory checkpoint。
func assertCheckpointFileExists(t *testing.T, dir, agentID string) {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(dir, "checkpoints"))
	if err != nil {
		t.Fatalf("memory checkpoint 应落盘到 <dataDir>/checkpoints/，读目录失败: %v", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
		if strings.HasPrefix(e.Name(), agentID+".") && strings.HasSuffix(e.Name(), ".ckpt") {
			return
		}
	}
	t.Fatalf("%s 名下没有落盘的 checkpoint（每轮 checkpoint 没生效），目录内容=%v", agentID, names)
}

// assertNoCheckpointFile 断言该 agent 名下没有任何落盘的 checkpoint。
func assertNoCheckpointFile(t *testing.T, dir, agentID string) {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(dir, "checkpoints"))
	if err != nil {
		return // 目录都不存在 = 当然没有
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), agentID+".") && strings.HasSuffix(e.Name(), ".ckpt") {
			t.Fatalf("%s 不该有 checkpoint 落盘，实得 %s", agentID, e.Name())
		}
	}
}

// TestTurnWritesDaemonConversationMemory：memory.daemon 家 → 原文进 daemon 侧会话记忆。
func TestTurnWritesDaemonConversationMemory(t *testing.T) {
	p := &memShapeProvider{id: "dm", model: runtime.MemoryDaemon, reply: "回答正文"}
	srv, thID, dir := newTurnMemoryServer(t, p)
	runSimpleTurn(t, srv, thID)

	key := runtime.MemoryKey{AgentID: p.ID(), ThreadID: thID, Kind: runtime.MemoryConversation}
	snap, err := srv.runtimeBackend.Memory().Load(context.Background(), key)
	if err != nil {
		t.Fatalf("读记忆失败: %v", err)
	}
	if len(snap.Items) != 2 {
		t.Fatalf("一轮过后应记下 user + assistant 两条，实得 %d（server 侧接线没生效？）", len(snap.Items))
	}
	if snap.Items[0].Role != "user" || !strings.Contains(string(snap.Items[0].Payload), "hi") {
		t.Fatalf("user 原文应进记忆，实得 role=%q payload=%q", snap.Items[0].Role, snap.Items[0].Payload)
	}
	if snap.Items[1].Role != "assistant" || !strings.Contains(string(snap.Items[1].Payload), "回答正文") {
		t.Fatalf("assistant 原文应进记忆，实得 role=%q payload=%q", snap.Items[1].Role, snap.Items[1].Payload)
	}
	assertCheckpointFileExists(t, dir, p.ID())
}

// TestTurnWritesHybridIndexWithoutProse：memory.hybrid 家 → 只写**索引**，
// 正文不进 daemon 侧（descriptor.go:154「原生存正文，daemon 存摘要与索引」）。
func TestTurnWritesHybridIndexWithoutProse(t *testing.T) {
	const prose = "这段正文不该出现在-daemon-侧记忆里"
	p := &memShapeProvider{id: "hy", model: runtime.MemoryHybrid, reply: prose}
	srv, thID, dir := newTurnMemoryServer(t, p)
	runSimpleTurn(t, srv, thID)

	key := runtime.MemoryKey{AgentID: p.ID(), ThreadID: thID, Kind: runtime.MemoryArtifact}
	snap, err := srv.runtimeBackend.Memory().Load(context.Background(), key)
	if err != nil {
		t.Fatalf("读记忆失败: %v", err)
	}
	if len(snap.Items) == 0 {
		t.Fatal("hybrid 家每轮应留下索引条目，否则诊断页看不出它用上了统一 Runtime")
	}
	for i, it := range snap.Items {
		if strings.Contains(string(it.Payload), prose) {
			t.Fatalf("第 %d 条索引里出现了正文（hybrid 的原生侧才存正文），实得 %q", i, it.Payload)
		}
		if it.Meta["entry"] != "turn_index" {
			t.Fatalf("第 %d 条索引必须自我描述 meta[entry]=turn_index，实得 meta=%v", i, it.Meta)
		}
		if it.Meta["turn_id"] == "" || it.Role == "" || it.Meta["blocks"] == "" || it.Meta["bytes"] == "" {
			t.Fatalf("第 %d 条索引应含 turn_id/role/blocks/bytes，实得 meta=%v role=%q", i, it.Meta, it.Role)
		}
	}
	assertCheckpointFileExists(t, dir, p.ID())

	// 诊断页口径：Observe 必须看得见（D7 的验收方式就是"接入前是 0、接入后不是 0"；
	// 若只断到 Memory().Load，接线掉了但别处替它写了仍是绿的）。
	if obs := srv.runtimeBackend.Observe(p.ID()); obs.Memory.Items == 0 || obs.Checkpoint.Count == 0 {
		t.Fatalf("诊断页应能看到 hybrid 家的记忆与 checkpoint，实得 %+v", obs)
	}
}

// TestTurnSkipsNativeMemory：memory.native 家 → 正文全在原生侧，daemon 不写。
func TestTurnSkipsNativeMemory(t *testing.T) {
	p := &memShapeProvider{id: "nv", model: runtime.MemoryNative, reply: "x"}
	srv, thID, dir := newTurnMemoryServer(t, p)
	runSimpleTurn(t, srv, thID)

	for _, kind := range []runtime.MemoryKind{runtime.MemoryConversation, runtime.MemoryArtifact} {
		snap, err := srv.runtimeBackend.Memory().Load(context.Background(),
			runtime.MemoryKey{AgentID: p.ID(), ThreadID: thID, Kind: kind})
		if err != nil {
			t.Fatalf("读记忆失败: %v", err)
		}
		if len(snap.Items) != 0 {
			t.Fatalf("memory.native 家不该往 daemon 侧写 %s，实得 %d 条", kind, len(snap.Items))
		}
	}
	assertNoCheckpointFile(t, dir, p.ID())
}

// TestTurnSkipsProviderWithoutDeclaration：没申报记忆形态的 provider（未实现
// runtime.Participant）→ 一条都不写，且不得因此崩掉轮次。
//
// 【为什么必须有这条】沉默缺失不等于"它有记忆"。猜着写会把别家的数据记到它名下，
// 而且这种错在下游看起来完全正常（诊断页有数字，只是归属错了）。
func TestTurnSkipsProviderWithoutDeclaration(t *testing.T) {
	p := &captureProvider{} // 未实现 runtime.Participant（见 turn_overrides_test.go）
	srv, thID, dir := newTurnMemoryServer(t, p)
	runSimpleTurn(t, srv, thID)

	obs := srv.runtimeBackend.Observe(p.ID())
	if obs.Memory.Items != 0 || obs.Checkpoint.Count != 0 {
		t.Fatalf("未申报记忆形态的 provider 不该产生任何写入，实得 %+v", obs)
	}
	assertNoCheckpointFile(t, dir, p.ID())
}

// TestEchoTurnWritesConversationThroughServer：真 echo provider 经 server 接线落
// 会话记忆，且**换一个同数据目录的新后端**仍读得回（真跨进程恢复，不是同进程模拟）。
//
// 【为什么这条从 agent 包搬到这里】echo 原先在自己的 Run 里写记忆；D7 把接线收归
// server 之后，"echo 的记忆能被读到"的验证点随之移动 —— 留在原处会变成对一件已经
// 不成立的事的断言（agent 包内的 Run 不再写任何记忆）。
func TestEchoTurnWritesConversationThroughServer(t *testing.T) {
	p := agent.NewEcho()
	srv, thID, dir := newTurnMemoryServer(t, p)
	runSimpleTurn(t, srv, thID)

	key := runtime.MemoryKey{AgentID: p.ID(), ThreadID: thID, Kind: runtime.MemoryConversation}
	snap, err := srv.runtimeBackend.Memory().Load(context.Background(), key)
	if err != nil {
		t.Fatalf("读记忆失败: %v", err)
	}
	if len(snap.Items) != 2 {
		t.Fatalf("echo 一轮过后应记下 2 条会话记忆，实得 %d", len(snap.Items))
	}

	// 真跨进程：同 dataDir 的**新**后端（模拟 daemon 重启）也要读得回。
	fresh := runtime.NewFileBackend(dir)
	snap2, err := fresh.Memory().Load(context.Background(), key)
	if err != nil {
		t.Fatalf("重启后读记忆失败: %v", err)
	}
	if len(snap2.Items) != 2 {
		t.Fatalf("echo 的会话记忆必须跨重启存活，实得 %d 条", len(snap2.Items))
	}
}
