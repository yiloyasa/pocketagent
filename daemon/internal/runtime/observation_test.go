// observation_test.go —— 按 agent 聚合的运行时状态观测（规格 §9.5「每家都能显示
// memory/cache/resource/checkpoint/recovery 状态」的数据源）。
//
// 【为什么先钉"隔离"这一条】观测是给诊断页用的：它把"codex 有多少记忆/缓存/
// checkpoint"摆给用户看。一旦聚合口径串了家，用户看到的是"codex 有 3 条记忆"，
// 而那 3 条其实是 echo 的 —— 比不显示更糟：它会把人引向完全错误的结论。
package runtime

import (
	"context"
	"testing"
	"time"
)

func TestObserveAggregatesPerAgent(t *testing.T) {
	b := NewBackend()
	ctx := context.Background()

	// echo 的两条记忆（conversation 层）
	for _, role := range []string{"user", "assistant"} {
		if _, err := b.Memory().Append(ctx,
			MemoryKey{AgentID: "echo", ThreadID: "th-1", Kind: MemoryConversation},
			MemoryItem{Role: role, Payload: []byte("hello-world")}); err != nil {
			t.Fatalf("Append 失败: %v", err)
		}
	}
	// 另一家的记忆：不得混入 echo 的观测
	if _, err := b.Memory().Append(ctx,
		MemoryKey{AgentID: "codex", ThreadID: "th-2", Kind: MemoryConversation},
		MemoryItem{Role: "user", Payload: []byte("other")}); err != nil {
		t.Fatalf("Append 失败: %v", err)
	}
	// echo / pi 各一条缓存（命名空间 agent/<id>/models）
	for _, id := range []string{"echo", "pi"} {
		if err := b.Cache().Put(ctx,
			CacheKey{Namespace: Namespace(id, NsModels), Key: "m1"},
			CacheValue{Payload: []byte("{}")},
			CachePolicy{TTL: time.Minute, MaxEntries: 10}); err != nil {
			t.Fatalf("Put 失败: %v", err)
		}
	}
	// echo 的 checkpoint
	if err := b.Checkpoints().Save(ctx,
		CheckpointKey{AgentID: "echo", ThreadID: "th-1", Name: "conversation"},
		[]byte("cp-payload")); err != nil {
		t.Fatalf("Save 失败: %v", err)
	}

	obs := b.Observe("echo")
	if obs.AgentID != "echo" {
		t.Fatalf("观测应回显 agent_id，实得 %q", obs.AgentID)
	}
	if obs.Memory.Threads != 1 || obs.Memory.Items != 2 {
		t.Fatalf("echo 记忆应为 1 线程 2 条目，实得 %+v", obs.Memory)
	}
	if obs.Memory.Kinds[MemoryConversation] != 2 {
		t.Fatalf("分层计数应含 conversation=2，实得 %+v", obs.Memory.Kinds)
	}
	if obs.Memory.Bytes <= 0 {
		t.Fatalf("应统计记忆字节数，实得 %d", obs.Memory.Bytes)
	}
	if obs.Cache.Namespaces != 1 || obs.Cache.Entries != 1 {
		t.Fatalf("echo 缓存应为 1 命名空间 1 条目，实得 %+v", obs.Cache)
	}
	if obs.Checkpoint.Count != 1 || obs.Checkpoint.Bytes <= 0 {
		t.Fatalf("echo checkpoint 应为 1 份且带字节数，实得 %+v", obs.Checkpoint)
	}

	// 隔离：别家不该看到 echo 的东西
	other := b.Observe("codex")
	if other.Memory.Items != 1 {
		t.Fatalf("codex 记忆应为 1 条，实得 %d（聚合串家了）", other.Memory.Items)
	}
	if other.Cache.Entries != 0 {
		t.Fatalf("codex 不该有缓存条目，实得 %d", other.Cache.Entries)
	}
	if other.Checkpoint.Count != 0 {
		t.Fatalf("codex 不该有 checkpoint，实得 %d", other.Checkpoint.Count)
	}

	// 完全没数据的 agent：全零、不报错（诊断页要能显示"这家还没用上"）
	empty := b.Observe("nobody")
	if empty.Memory.Items != 0 || empty.Cache.Entries != 0 || empty.Checkpoint.Count != 0 {
		t.Fatalf("无数据的 agent 应全零，实得 %+v", empty)
	}
}

// TestObserveCheckpointOnFileBackend：文件后端的 checkpoint 不在内存 map 里，
// 而是在 <dataDir>/checkpoints/ 目录里 —— 观测必须也能数出来，否则"接入了文件
// 后端之后诊断页反而显示 0"，比不显示更误导。
func TestObserveCheckpointOnFileBackend(t *testing.T) {
	dir := t.TempDir()
	b := NewFileBackend(dir)
	ctx := context.Background()
	for _, id := range []string{"echo", "pi"} {
		if err := b.Checkpoints().Save(ctx,
			CheckpointKey{AgentID: id, ThreadID: "th", Name: "conversation"},
			[]byte("payload-"+id)); err != nil {
			t.Fatalf("Save 失败: %v", err)
		}
	}
	echo := b.Observe("echo")
	if echo.Checkpoint.Count != 1 {
		t.Fatalf("文件后端 echo checkpoint 应数出 1 份，实得 %d", echo.Checkpoint.Count)
	}
	if echo.Checkpoint.Bytes <= 0 {
		t.Fatalf("应统计文件大小，实得 %d", echo.Checkpoint.Bytes)
	}
	if pi := b.Observe("pi"); pi.Checkpoint.Count != 1 {
		t.Fatalf("pi checkpoint 应数出 1 份，实得 %d", pi.Checkpoint.Count)
	}
}

// TestObserveRecoveryIsHonestWhenUnreported：recovery 状态目前没有产出方，
// 观测必须如实报"未上报"，不得凭空造一个"恢复成功"。
func TestObserveRecoveryIsHonestWhenUnreported(t *testing.T) {
	b := NewBackend()
	obs := b.Observe("echo")
	if obs.Recovery.Attempted {
		t.Fatal("没有产出方时不得声称尝试过恢复（那是编造）")
	}
	if obs.Recovery.From != RecoveredNone {
		t.Fatalf("未上报时来源应为 none，实得 %q", obs.Recovery.From)
	}
}
