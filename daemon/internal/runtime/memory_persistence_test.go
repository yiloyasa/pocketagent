// memory_persistence_test.go —— memory checkpoint 的**真跨进程**持久化。
//
// 【为什么必须用"新 Backend 实例 + 同 dataDir"】契约套件里的
// MemorySurvivesRestartOnlyViaCheckpoint 用 ExportDurable/ImportDurable 在**同一
// 进程内**模拟重启 —— 它证明的是"内存态的导出/导入无损"，不是"跨进程落盘"。
// 文件后端的 checkpoint 只有写到磁盘、再由新实例读回，才算真的跨重启存活。
// 同进程 export/import 全绿说明不了跨进程持久化（本项目踩过的坑）。
package runtime

import (
	"context"
	"testing"
)

func memKeyFor(thread string) MemoryKey {
	return MemoryKey{AgentID: "echo", ThreadID: thread, Kind: MemoryConversation}
}

// TestMemoryCheckpointSurvivesRealRestart：真跨进程恢复 + 观测可见 + 序号延续。
func TestMemoryCheckpointSurvivesRealRestart(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	key := memKeyFor("th-restart")

	// —— 进程 1：写两条，落盘 ——
	b1 := NewFileBackend(dir)
	for _, p := range []string{"hello", "world"} {
		if _, err := b1.Memory().Append(ctx, key, MemoryItem{Role: "user", Payload: []byte(p)}); err != nil {
			t.Fatalf("Append(%s) 失败: %v", p, err)
		}
	}
	if err := b1.Memory().Checkpoint(ctx, key); err != nil {
		t.Fatalf("Checkpoint 失败: %v", err)
	}

	// —— 进程 2：同 dataDir 的全新后端（模拟 daemon 重启） ——
	b2 := NewFileBackend(dir)
	snap, err := b2.Memory().Load(ctx, key)
	if err != nil {
		t.Fatalf("重启后 Load 失败: %v", err)
	}
	if len(snap.Items) != 2 {
		t.Fatalf("重启后应读回 2 项，实得 %d —— memory checkpoint 没落盘或没装载", len(snap.Items))
	}
	if string(snap.Items[0].Payload) != "hello" || string(snap.Items[1].Payload) != "world" {
		t.Fatalf("重启后内容应原样，实得 %q,%q", snap.Items[0].Payload, snap.Items[1].Payload)
	}

	// 观测必须能看见：重启后诊断页显示 0 比不显示更误导
	obs := b2.Observe("echo")
	if obs.Memory.Items != 2 {
		t.Fatalf("重启后（Load 之后）观测 memory 应为 2 条，实得 %+v", obs.Memory)
	}
	if obs.Checkpoint.Count == 0 {
		t.Fatalf("重启后观测应数出 checkpoint，实得 %+v", obs.Checkpoint)
	}

	// 序号高水位必须跨进程延续：复用序号会与已落盘的 sequence 撞车
	// （手机端按 sequence 增量补帧，撞车即对不上账）。
	seq, err := b2.Memory().Append(ctx, key, MemoryItem{Payload: []byte("again")})
	if err != nil {
		t.Fatalf("重启后 Append 失败: %v", err)
	}
	if seq != 3 {
		t.Fatalf("重启后 sequence 应接着 3，实得 %d（复用序号会让摘要覆盖范围失去意义）", seq)
	}
}

// TestMemoryAppendAfterRestartContinuesMemory：重启后往同一 key 追加必须**接着**
// 盘上的旧记忆走，而不是从空态重新开始。
//
// 【为什么这条比"Load 能读回"更关键】生产轮次路径（echo 的 recordTurn）是
// Append → Checkpoint，全程不调 Load。若装载只挂在 Load 上，重启后第一轮就会：
// 在空态上从 seq=1 重新分配 → Checkpoint 用"只有新内容"的快照覆盖盘上旧记忆。
// 那等于"重启即丢历史"，而且丢得无声。所以装载挂在每个入口（首次触碰即装载）。
func TestMemoryAppendAfterRestartContinuesMemory(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	key := memKeyFor("th-continue")

	b1 := NewFileBackend(dir)
	if _, err := b1.Memory().Append(ctx, key, MemoryItem{Role: "user", Payload: []byte("old")}); err != nil {
		t.Fatalf("Append 失败: %v", err)
	}
	if err := b1.Memory().Checkpoint(ctx, key); err != nil {
		t.Fatalf("Checkpoint 失败: %v", err)
	}

	// 新进程：不先 Load，直接 Append（复刻生产 recordTurn 的顺序）。
	b2 := NewFileBackend(dir)
	seq, err := b2.Memory().Append(ctx, key, MemoryItem{Role: "user", Payload: []byte("new")})
	if err != nil {
		t.Fatalf("重启后 Append 失败: %v", err)
	}
	if seq != 2 {
		t.Fatalf("重启后 Append 应接着旧高水位（2），实得 %d —— 从 1 重来会与盘上序号撞车", seq)
	}
	snap, err := b2.Memory().Load(ctx, key)
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if len(snap.Items) != 2 {
		t.Fatalf("重启后应保留旧记忆并在其上追加（2 条），实得 %d —— 旧记忆在 Append 前没被装载", len(snap.Items))
	}
	if string(snap.Items[0].Payload) != "old" || string(snap.Items[1].Payload) != "new" {
		t.Fatalf("顺序应为 old→new，实得 %q,%q", snap.Items[0].Payload, snap.Items[1].Payload)
	}
}

// TestMemoryCheckpointAfterRestartDoesNotWipe：重启后未追加任何内容就 Checkpoint，
// 不得用空内存态把盘上的旧记忆覆盖掉（那是比"不落盘"更隐蔽的丢数据）。
func TestMemoryCheckpointAfterRestartDoesNotWipe(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	key := memKeyFor("th-wipe")

	b1 := NewFileBackend(dir)
	if _, err := b1.Memory().Append(ctx, key, MemoryItem{Payload: []byte("keep")}); err != nil {
		t.Fatalf("Append 失败: %v", err)
	}
	if err := b1.Memory().Checkpoint(ctx, key); err != nil {
		t.Fatalf("Checkpoint 失败: %v", err)
	}

	// 新进程直接 Checkpoint（不 Append、不 Load）—— 必须先把盘上的旧记忆装载回来。
	b2 := NewFileBackend(dir)
	if err := b2.Memory().Checkpoint(ctx, key); err != nil {
		t.Fatalf("Checkpoint 失败: %v", err)
	}

	// 第三个实例读盘，确认内容没被清成空。
	b3 := NewFileBackend(dir)
	snap, err := b3.Memory().Load(ctx, key)
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if len(snap.Items) != 1 || string(snap.Items[0].Payload) != "keep" {
		t.Fatalf("重启后空态 Checkpoint 不得清掉盘上记忆，实得 %d 项", len(snap.Items))
	}
}

// TestMemoryRestoreOnFileBackend：公共 Restore 从文件读回，且保持
// "回到 checkpoint 那一刻"的覆盖语义（显式恢复与"首次触碰装载"不同）。
func TestMemoryRestoreOnFileBackend(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	key := memKeyFor("th-restore")

	b1 := NewFileBackend(dir)
	for i := 1; i <= 3; i++ {
		if _, err := b1.Memory().Append(ctx, key, MemoryItem{Payload: []byte("x")}); err != nil {
			t.Fatalf("Append 失败: %v", err)
		}
	}
	if err := b1.Memory().Checkpoint(ctx, key); err != nil {
		t.Fatalf("Checkpoint 失败: %v", err)
	}

	// 新进程先追加 2 条（追加前会装载旧记忆），再 Restore —— 必须回到 checkpoint 时的 3 条。
	b2 := NewFileBackend(dir)
	for i := 0; i < 2; i++ {
		if _, err := b2.Memory().Append(ctx, key, MemoryItem{Payload: []byte("new")}); err != nil {
			t.Fatalf("Append 失败: %v", err)
		}
	}
	if err := b2.Memory().Restore(ctx, key); err != nil {
		t.Fatalf("文件后端 Restore 失败: %v", err)
	}
	snap, err := b2.Memory().Load(ctx, key)
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if len(snap.Items) != 3 {
		t.Fatalf("文件后端 Restore 应回到 checkpoint 时的 3 项，实得 %d", len(snap.Items))
	}
	// Restore 不得把序号高水位降回去（降了就会复用已发出过的序号）。
	seq, err := b2.Memory().Append(ctx, key, MemoryItem{Payload: []byte("y")})
	if err != nil {
		t.Fatalf("Append 失败: %v", err)
	}
	if seq <= 3 {
		t.Fatalf("Restore 后 Append 必须接着旧高水位（>3），实得 %d", seq)
	}
}
