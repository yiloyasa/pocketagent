package agent

import (
	"context"
	"os"
	"testing"

	"github.com/pocketagent/daemon/internal/runtime"
)

// TestBackendForDataDirPersistsCheckpoints 钉住 W6 的接线：
// 给了数据目录时，checkpoint 必须真的落到磁盘，而不是仍活在进程内存 map 里。
//
// 【为什么单测这一层】包级共享后端由 sync.Once 定型、进程内无法重来，所以
// 「接线是否真的生效」不能在单例上直接验。把选择逻辑抽成 backendForDataDir 后，
// 这里就能直接验「选出来的后端」的持久化行为 —— 而"文件后端写好了却没接上"
// 正是本波最可能留下的半成品形态（工程铁律：不留半接系统）。
func TestBackendForDataDirPersistsCheckpoints(t *testing.T) {
	dir := t.TempDir()
	b := backendForDataDir(dir)
	if b == nil {
		t.Fatal("后端不应为空")
	}
	ctx := context.Background()
	key := runtime.CheckpointKey{AgentID: "echo", ThreadID: "th-1", Name: "conversation"}
	payload := []byte(`{"hello":"world"}`)
	if err := b.Checkpoints().Save(ctx, key, payload); err != nil {
		t.Fatalf("Save 失败: %v", err)
	}

	// 落盘证据：数据目录下必须出现条目（文件后端与内存后端的分界就在这里）。
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读数据目录失败: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("给了数据目录却没落盘 —— 文件后端没接上，仍是内存形态")
	}

	// 换一个后端实例读回（模拟 daemon 重启）。
	b2 := backendForDataDir(dir)
	got, found, err := b2.Checkpoints().Load(ctx, key)
	if err != nil || !found {
		t.Fatalf("重启后读不回: found=%v err=%v", found, err)
	}
	if string(got) != string(payload) {
		t.Fatalf("读回内容不符: %q ≠ %q", got, payload)
	}
}

// TestBackendForDataDirEmptyStaysInMemory 钉住无目录时的内存语义：
// 行为零分叉（既有测试与内存模式都依赖它）。
func TestBackendForDataDirEmptyStaysInMemory(t *testing.T) {
	ctx := context.Background()
	key := runtime.CheckpointKey{AgentID: "echo", ThreadID: "th-2", Name: "conversation"}
	b := backendForDataDir("")
	if b == nil {
		t.Fatal("后端不应为空")
	}
	if err := b.Checkpoints().Save(ctx, key, []byte("x")); err != nil {
		t.Fatalf("内存后端 Save 失败: %v", err)
	}
	// 新实例读不到 = 内存语义的判据（重启即失）。
	if _, found, _ := backendForDataDir("").Checkpoints().Load(ctx, key); found {
		t.Fatal("内存后端在新实例里不该还有 checkpoint")
	}
}
