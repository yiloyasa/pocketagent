// recovery_events_test.go —— per-agent recovery 的产出方（D7 收尾 C）。
//
// 【最小诚实语义】只报告"本进程内发生过、且有证据"的恢复动作：
//   - 重启后第一次触碰 memory，hydrate 从磁盘装载 checkpoint → 这是一次真实的
//     跨进程恢复，可报；
//   - 没有从磁盘装载过任何东西 → 恒报 RecoveredNone（"未上报"），绝不编造。
package runtime

import (
	"context"
	"testing"
)

// TestRecoveryReportedAfterRestartHydration：重启 → 触碰 memory → recovery 报真实信息。
func TestRecoveryReportedAfterRestartHydration(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	key := memKeyFor("th-recovery")

	b1 := NewFileBackend(dir)
	if _, err := b1.Memory().Append(ctx, key, MemoryItem{Role: "user", Payload: []byte("old")}); err != nil {
		t.Fatalf("Append 失败: %v", err)
	}
	if err := b1.Memory().Checkpoint(ctx, key); err != nil {
		t.Fatalf("Checkpoint 失败: %v", err)
	}

	// —— 新实例（模拟 daemon 重启）——
	b2 := NewFileBackend(dir)

	// 触碰前：什么都没发生，必须如实报"未上报"。
	obs := b2.Observe("echo")
	if obs.Recovery.Attempted || obs.Recovery.From != RecoveredNone {
		t.Fatalf("尚未发生任何恢复动作时不得上报 (attempted=%v from=%s)",
			obs.Recovery.Attempted, obs.Recovery.From)
	}

	// 生产轮次路径的顺序是 Append（不先 Load）——它内部触发 hydrate 装载。
	if _, err := b2.Memory().Append(ctx, key, MemoryItem{Role: "user", Payload: []byte("new")}); err != nil {
		t.Fatalf("重启后 Append 失败: %v", err)
	}

	obs = b2.Observe("echo")
	if !obs.Recovery.Attempted {
		t.Fatal("从磁盘 hydrate 装载后 recovery 应上报 attempted")
	}
	if !obs.Recovery.Succeeded {
		t.Fatal("装载成功即恢复成功，succeeded 应为真")
	}
	if obs.Recovery.From != RecoveredCheckpoint {
		t.Fatalf("恢复来源应为 checkpoint，实得 %q", obs.Recovery.From)
	}

	// 事件源要能给出 thread 级明细（诊断页/导出用）。
	events := b2.RecoveryEvents("echo")
	if len(events) != 1 {
		t.Fatalf("应有 1 条恢复事件，实得 %d", len(events))
	}
	ev := events[0]
	if ev.Kind != KindRecoveryHydrate {
		t.Fatalf("事件类别应为 %q，实得 %q", KindRecoveryHydrate, ev.Kind)
	}
	if ev.AgentID != "echo" || ev.ThreadID != "th-recovery" {
		t.Fatalf("事件应带 agent/thread 定位，实得 %+v", ev)
	}
}

// TestRecoveryNotClaimedWithoutDiskCheckpoint：盘上根本没有 checkpoint 时，
// hydrate 查无此物 —— 没有恢复发生，不得上报（不硬造）。
func TestRecoveryNotClaimedWithoutDiskCheckpoint(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	key := memKeyFor("th-nodisk")

	b1 := NewFileBackend(dir)
	// 只 Append 不 Checkpoint：磁盘上没有任何 checkpoint。
	if _, err := b1.Memory().Append(ctx, key, MemoryItem{Payload: []byte("x")}); err != nil {
		t.Fatalf("Append 失败: %v", err)
	}

	b2 := NewFileBackend(dir)
	if _, err := b2.Memory().Append(ctx, key, MemoryItem{Payload: []byte("y")}); err != nil {
		t.Fatalf("重启后 Append 失败: %v", err)
	}
	obs := b2.Observe("echo")
	if obs.Recovery.Attempted || obs.Recovery.From != RecoveredNone {
		t.Fatalf("盘上无 checkpoint 时不得上报恢复 (attempted=%v from=%s)",
			obs.Recovery.Attempted, obs.Recovery.From)
	}
	if len(b2.RecoveryEvents("echo")) != 0 {
		t.Fatal("盘上无 checkpoint 时不得产生恢复事件")
	}
}

// TestRecoveryIsolatedPerAgent：恢复事实按 agent 隔离 —— 只触碰 echo 的记忆，
// codex 的 recovery 不得被点亮（观测口径串家比不显示更糟）。
func TestRecoveryIsolatedPerAgent(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	b1 := NewFileBackend(dir)
	echoKey := memKeyFor("th-e")
	codexKey := MemoryKey{AgentID: "codex", ThreadID: "th-c", Kind: MemoryConversation}
	for _, k := range []MemoryKey{echoKey, codexKey} {
		if _, err := b1.Memory().Append(ctx, k, MemoryItem{Payload: []byte("x")}); err != nil {
			t.Fatalf("Append(%s) 失败: %v", k.AgentID, err)
		}
		if err := b1.Memory().Checkpoint(ctx, k); err != nil {
			t.Fatalf("Checkpoint(%s) 失败: %v", k.AgentID, err)
		}
	}

	b2 := NewFileBackend(dir)
	// 只触碰 echo。
	if _, err := b2.Memory().Append(ctx, echoKey, MemoryItem{Payload: []byte("n")}); err != nil {
		t.Fatalf("重启后 Append 失败: %v", err)
	}
	if got := b2.Observe("codex").Recovery; got.Attempted || got.From != RecoveredNone {
		t.Fatalf("未触碰的 agent 不得上报恢复 (attempted=%v from=%s)", got.Attempted, got.From)
	}
	if got := b2.Observe("echo").Recovery; !got.Attempted || got.From != RecoveredCheckpoint {
		t.Fatalf("已触碰的 agent 应上报真实恢复，实得 %+v", got)
	}
}

// TestRecoveryOnMemoryBackendStaysUnreported：内存后端没有跨进程载体，
// 同进程内的 hydrated 标记不代表"重启恢复"，必须保持恒不报
// （与 observation_test.go 的 TestObserveRecoveryIsHonestWhenUnreported 同一铁律）。
func TestRecoveryOnMemoryBackendStaysUnreported(t *testing.T) {
	ctx := context.Background()
	b := NewBackend()
	key := memKeyFor("th-mem")
	// 先写 checkpoint 再触碰：内存实现里 hydrate 也能"找到"数据，但那是
	// 同一进程内的数据，不构成恢复事件。
	if _, err := b.Memory().Append(ctx, key, MemoryItem{Payload: []byte("x")}); err != nil {
		t.Fatalf("Append 失败: %v", err)
	}
	if err := b.Memory().Checkpoint(ctx, key); err != nil {
		t.Fatalf("Checkpoint 失败: %v", err)
	}
	if _, err := b.Memory().Load(ctx, key); err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if got := b.Observe("echo").Recovery; got.Attempted || got.From != RecoveredNone {
		t.Fatalf("内存后端不得上报恢复 (attempted=%v from=%s)", got.Attempted, got.From)
	}
	if events := b.RecoveryEvents("echo"); len(events) != 0 {
		t.Fatalf("内存后端不得产生恢复事件，实得 %d", len(events))
	}
}
