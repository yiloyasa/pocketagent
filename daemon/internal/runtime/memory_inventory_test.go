package runtime

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// memory_inventory_test.go —— 按会话记忆清单（ListThreadMemories）与清除
// （ClearThreadMemory）的回归用例。
//
// 验收点:
//
//	① 清单合并「已装载的内存态」与「盘上未装载的 checkpoint 载荷」，两路都不缺;
//	② 按 agent 隔离、按字节降序、seq/条数/字节/最后 checkpoint 时刻逐项如实;
//	③ 清除 = 内存态 + 持久层一起清，剩余合计正确，文件真的没了;
//	④ 清除后 hydrate 不复活：文件已删，同后端再 Load、新后端（模拟重启）再 Load
//	   都必须 fail-open 返回空 —— 「让这个会话的记忆重新开始」的完整语义;
//	⑤ 清一个不存在的会话：found=false，绝不能当成成功。
//
// 【为什么必须用 NewFileBackend】生产 daemon 用的是文件后端
// （internal/agent/runtime_descriptors.go: NewFileBackend(dir)）。清单的"盘上
// 未装载"半边只有文件后端才有;内存后端（测试共享单例）永远全装载,测不出②。

func inventoryKey(agentID, thread string) MemoryKey {
	return MemoryKey{AgentID: agentID, ThreadID: thread, Kind: MemoryConversation}
}

// seedInventory 往后端写两个会话的记忆并逐条落盘：
// th-a 两条（"aa"+"bb"，共 4 字节）；th-b 一条（"cccccccc"，8 字节）。
// th-b 字节更多 —— 清单按字节降序时应排在前面的那个。
func seedInventory(t *testing.T, b *Backend) {
	t.Helper()
	ctx := context.Background()
	for _, p := range []string{"aa", "bb"} {
		if _, err := b.Memory().Append(ctx, inventoryKey("echo", "th-a"), MemoryItem{Role: "user", Payload: []byte(p)}); err != nil {
			t.Fatalf("seed th-a Append(%s) 失败: %v", p, err)
		}
	}
	if err := b.Memory().Checkpoint(ctx, inventoryKey("echo", "th-a")); err != nil {
		t.Fatalf("seed th-a Checkpoint 失败: %v", err)
	}
	if _, err := b.Memory().Append(ctx, inventoryKey("echo", "th-b"), MemoryItem{Role: "user", Payload: []byte("cccccccc")}); err != nil {
		t.Fatalf("seed th-b Append 失败: %v", err)
	}
	if err := b.Memory().Checkpoint(ctx, inventoryKey("echo", "th-b")); err != nil {
		t.Fatalf("seed th-b Checkpoint 失败: %v", err)
	}
}

// TestListThreadMemoriesMergesMemoryAndDisk 覆盖 ①②。
func TestListThreadMemoriesMergesMemoryAndDisk(t *testing.T) {
	dir := t.TempDir()
	b := NewFileBackend(dir)
	seedInventory(t, b)

	// 别家 agent 的记忆绝不能算进来（按 agent 隔离是这类视图的底线）。
	ctx := context.Background()
	other := MemoryKey{AgentID: "other", ThreadID: "th-a", Kind: MemoryConversation}
	if _, err := b.Memory().Append(ctx, other, MemoryItem{Payload: []byte("intruder")}); err != nil {
		t.Fatalf("seed other 失败: %v", err)
	}
	if err := b.Memory().Checkpoint(ctx, other); err != nil {
		t.Fatalf("seed other checkpoint 失败: %v", err)
	}

	rows, totals := b.ListThreadMemories("echo")
	if len(rows) != 2 {
		t.Fatalf("echo 应有 2 个会话的记忆,实得 %d 行: %+v", len(rows), rows)
	}
	// 按字节降序:th-b(8B) 在前,th-a(4B) 在后。
	if rows[0].ThreadID != "th-b" || rows[1].ThreadID != "th-a" {
		t.Fatalf("应按字节降序(th-b,th-a),实得 %q,%q", rows[0].ThreadID, rows[1].ThreadID)
	}
	if rows[0].Entries != 1 || rows[0].Bytes != 8 || rows[0].Seq != 1 {
		t.Fatalf("th-b 行应 1 条/8B/seq=1,实得 %+v", rows[0])
	}
	if rows[1].Entries != 2 || rows[1].Bytes != 4 || rows[1].Seq != 2 {
		t.Fatalf("th-a 行应 2 条/4B/seq=2,实得 %+v", rows[1])
	}
	for _, r := range rows {
		if !r.Loaded {
			t.Fatalf("刚写过的会话应标记已装载: %+v", r)
		}
		if r.LastCheckpoint.IsZero() {
			t.Fatalf("落过盘的会话应带最后 checkpoint 时刻: %+v", r)
		}
		if age := time.Since(r.LastCheckpoint); age < 0 || age > time.Minute {
			t.Fatalf("最后 checkpoint 时刻应接近现在,实得 %v", r.LastCheckpoint)
		}
	}
	if totals.Threads != 2 || totals.Entries != 3 || totals.Bytes != 12 {
		t.Fatalf("合计应 2 会话/3 条/12B,实得 %+v", totals)
	}
}

// TestListThreadMemoriesShowsDiskOnlyThreads：重启后（未触碰过的 key）也要如实列出
// —— 数据源就是盘上 checkpoint 的载荷,loaded=false 标注"本进程还没装载"。
func TestListThreadMemoriesShowsDiskOnlyThreads(t *testing.T) {
	dir := t.TempDir()
	seedInventory(t, NewFileBackend(dir))

	// 同 dataDir 的全新后端 = 模拟 daemon 重启:内存态全空,盘上文件还在。
	b2 := NewFileBackend(dir)
	rows, totals := b2.ListThreadMemories("echo")
	if len(rows) != 2 {
		t.Fatalf("重启后清单应仍列出 2 个会话(盘上),实得 %d 行: %+v", len(rows), rows)
	}
	for _, r := range rows {
		if r.Loaded {
			t.Fatalf("重启后未触碰的会话不该标记已装载: %+v", r)
		}
	}
	// 行值必须来自盘上载荷,不是编的:th-b 1 条/8B,th-a 2 条/4B,且 th-b 在前。
	if rows[0].ThreadID != "th-b" || rows[0].Entries != 1 || rows[0].Bytes != 8 {
		t.Fatalf("重启后 th-b 行应 1 条/8B,实得 %+v", rows[0])
	}
	if rows[1].ThreadID != "th-a" || rows[1].Entries != 2 || rows[1].Bytes != 4 {
		t.Fatalf("重启后 th-a 行应 2 条/4B,实得 %+v", rows[1])
	}
	if totals.Threads != 2 || totals.Entries != 3 || totals.Bytes != 12 {
		t.Fatalf("重启后合计应与重启前一致,实得 %+v", totals)
	}
}

// TestClearThreadMemoryRemovesMemoryAndDisk 覆盖 ③④。
func TestClearThreadMemoryRemovesMemoryAndDisk(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	b := NewFileBackend(dir)
	seedInventory(t, b)
	// 同一会话第二个 kind（artifact 槽）也要一起清:清除的单位是"会话",不是"某个槽"。
	artKey := MemoryKey{AgentID: "echo", ThreadID: "th-a", Kind: MemoryArtifact}
	if _, err := b.Memory().Append(ctx, artKey, MemoryItem{Payload: []byte("idx")}); err != nil {
		t.Fatalf("seed artifact 失败: %v", err)
	}
	if err := b.Memory().Checkpoint(ctx, artKey); err != nil {
		t.Fatalf("seed artifact checkpoint 失败: %v", err)
	}

	totals, found, err := b.ClearThreadMemory("echo", "th-a")
	if err != nil || !found {
		t.Fatalf("清除 th-a 应成功且 found=true,实得 err=%v found=%v", err, found)
	}
	// 剩余合计:只剩 th-b。
	if totals.Threads != 1 || totals.Entries != 1 || totals.Bytes != 8 {
		t.Fatalf("清除后合计应 1 会话/1 条/8B,实得 %+v", totals)
	}

	// 盘上文件真的没了:目录里不再有 th-a 名下的 checkpoint
	// （文件名含清洗后的 thread 组件,前缀匹配足以证明;哈希后缀由唯一性保证）。
	ents, rerr := os.ReadDir(filepath.Join(dir, "checkpoints"))
	if rerr != nil {
		t.Fatalf("读 checkpoint 目录失败: %v", rerr)
	}
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), "th-a.") {
			t.Fatalf("th-a 的 checkpoint 文件应已被删,目录里还有: %s", e.Name())
		}
	}

	// 清除后同后端再 Load:hydrated 标记已一并清掉,再探盘 → 文件没了 → fail-open 空。
	snap, err := b.Memory().Load(ctx, inventoryKey("echo", "th-a"))
	if err != nil {
		t.Fatalf("清除后 Load 不该报错: %v", err)
	}
	if len(snap.Items) != 0 {
		t.Fatalf("清除后 Load 必须为空(fail-open),实得 %d 条", len(snap.Items))
	}
	// 新后端（模拟重启）再 Load 同样不复活 —— 这是"不复活"的最终反证。
	b2 := NewFileBackend(dir)
	snap2, err := b2.Memory().Load(ctx, inventoryKey("echo", "th-a"))
	if err != nil {
		t.Fatalf("重启后 Load 不该报错: %v", err)
	}
	if len(snap2.Items) != 0 {
		t.Fatalf("盘上文件已删,重启后不得复活,实得 %d 条", len(snap2.Items))
	}
	// 清除后重新开始的会话,序号从 1 重排（"重新开始"语义的一部分）。
	seq, err := b.Memory().Append(ctx, inventoryKey("echo", "th-a"), MemoryItem{Payload: []byte("fresh")})
	if err != nil {
		t.Fatalf("清除后 Append 失败: %v", err)
	}
	if seq != 1 {
		t.Fatalf("清除后重新开始的会话应从 seq=1 起分配,实得 %d", seq)
	}
}

// TestClearThreadMemoryMissingThread 覆盖 ⑤。
func TestClearThreadMemoryMissingThread(t *testing.T) {
	dir := t.TempDir()
	b := NewFileBackend(dir)
	seedInventory(t, b)

	_, found, err := b.ClearThreadMemory("echo", "th-absent")
	if err != nil {
		t.Fatalf("清不存在的会话不该报传输错误(由上层翻 NotFound): %v", err)
	}
	if found {
		t.Fatal("不存在的会话绝不能报 found=true —— 那会把'没清任何东西'谎报成'清掉了'")
	}
	// 原有记忆不受误清影响。
	rows, totals := b.ListThreadMemories("echo")
	if len(rows) != 2 || totals.Entries != 3 {
		t.Fatalf("误清请求不得动到别的会话,实得 %+v / %+v", rows, totals)
	}
}

// TestListThreadMemoriesEmptyBackend：没有任何记忆时返回空清单与零合计,
// 不报错 —— "还没有记忆"是常态,不是故障。
func TestListThreadMemoriesEmptyBackend(t *testing.T) {
	b := NewFileBackend(t.TempDir())
	rows, totals := b.ListThreadMemories("echo")
	if len(rows) != 0 || totals.Threads != 0 || totals.Entries != 0 || totals.Bytes != 0 {
		t.Fatalf("空后端应返回空清单与零合计,实得 %+v / %+v", rows, totals)
	}
}

// TestClearThreadMemoryOnMemoryBackend：内存后端（checkpoint 存 b.durable map，
// 生产 daemon 的测试装配与精简部署走这条）的清除同样要清掉持久层 —— durable
// 里的旧账不删，下一次 hydrate 探"盘"就把刚清掉的记忆原样装回。
//
// 【为什么这条必须有】死锁回归也钉在这里：清除路径曾因"持 b.mu 调内部再取
// b.mu 的 durable 扫描"自死锁（RWMutex 不可重入），文件后端测试测不出来，
// 只有内存后端会走到那条分支。
func TestClearThreadMemoryOnMemoryBackend(t *testing.T) {
	ctx := context.Background()
	b := NewBackend() // dataDir 为空 = 内存后端
	seedInventory(t, b)

	rows, totals := b.ListThreadMemories("echo")
	if len(rows) != 2 || totals.Entries != 3 {
		t.Fatalf("内存后端清单应 2 会话/3 条,实得 %+v / %+v", rows, totals)
	}

	totals, found, err := b.ClearThreadMemory("echo", "th-a")
	if err != nil || !found {
		t.Fatalf("内存后端清除应成功,实得 err=%v found=%v", err, found)
	}
	if totals.Threads != 1 || totals.Entries != 1 || totals.Bytes != 8 {
		t.Fatalf("清除后剩余合计应 1/1/8B,实得 %+v", totals)
	}

	// hydrate 不复活：durable 已清,Load fail-open 空。
	snap, err := b.Memory().Load(ctx, inventoryKey("echo", "th-a"))
	if err != nil {
		t.Fatalf("清除后 Load 不该报错: %v", err)
	}
	if len(snap.Items) != 0 {
		t.Fatalf("durable 已清,Load 不得复活旧账,实得 %d 条", len(snap.Items))
	}
}
