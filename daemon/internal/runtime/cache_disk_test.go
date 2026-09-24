// cache_disk_test.go —— PersistToDisk 条目的**真跨进程**持久化（D7 收尾 A）。
//
// 【为什么必须用"新 Backend 实例 + 同 dataDir"】同进程内 Get 能命中说明不了
// 任何事（内存实现本来就命中）；只有新实例从 <dataDir>/cache/ 读回，才算
// "重启归零"这个缺陷被真正修掉。与 memory_persistence_test.go 的取证口径一致。
package runtime

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func diskCacheKey(agent, part, key string) CacheKey {
	return CacheKey{Namespace: Namespace(agent, part), Key: key}
}

// TestCachePersistToDiskSurvivesRestart：带标志的条目跨重启读回；
// 不带标志的条目保持历史语义（重启即失）。
func TestCachePersistToDiskSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	b1 := NewFileBackend(dir)
	c1 := b1.DiskCache()
	persistKey := diskCacheKey("echo", NsModels, "m1")
	if err := c1.Put(ctx, persistKey,
		CacheValue{Payload: []byte(`{"id":"m1"}`)},
		CachePolicy{TTL: time.Hour, PersistToDisk: true, StaleAllowed: true, MaxEntries: 10}); err != nil {
		t.Fatalf("Put(persist) 失败: %v", err)
	}
	volatileKey := diskCacheKey("echo", NsModels, "m2")
	if err := c1.Put(ctx, volatileKey,
		CacheValue{Payload: []byte(`{"id":"m2"}`)},
		CachePolicy{TTL: time.Hour, MaxEntries: 10}); err != nil {
		t.Fatalf("Put(内存) 失败: %v", err)
	}
	entries, err := filepath.Glob(filepath.Join(dir, "cache", "*.cache"))
	if err != nil {
		t.Fatalf("扫缓存目录失败: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("只应有 1 个带标志条目落盘，实得 %d（无标志条目不该落盘）", len(entries))
	}

	// —— 新 Backend 实例 + 同 dataDir（模拟 daemon 重启）——
	b2 := NewFileBackend(dir)
	c2 := b2.DiskCache()
	val, ok, err := c2.Get(ctx, persistKey)
	if err != nil || !ok {
		t.Fatalf("重启后带标志条目应可读回 (ok=%v err=%v)", ok, err)
	}
	if string(val.Payload) != `{"id":"m1"}` {
		t.Fatalf("重启后内容应原样，实得 %q", val.Payload)
	}
	pe, ok, err := c2.Peek(ctx, persistKey)
	if err != nil || !ok {
		t.Fatalf("重启后 Peek 应命中 (ok=%v err=%v)", ok, err)
	}
	if !pe.Fresh {
		t.Fatal("TTL 未到，重启后应仍新鲜")
	}
	if !pe.StaleAllowed {
		t.Fatal("StaleAllowed 策略位应随条目保留")
	}
	// 不带标志的条目：历史语义 = 重启即失
	if _, ok, _ := c2.Get(ctx, volatileKey); ok {
		t.Fatal("不带 PersistToDisk 的条目不应跨重启存活")
	}
}

// TestCachePersistTTLRespectedAfterRestart：重启不重置 TTL。
// 落盘时已过期的条目在重启后不得复活（Get 严格路径必须 miss）。
func TestCachePersistTTLRespectedAfterRestart(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	key := diskCacheKey("echo", NsAccount, "acct")

	b1 := NewFileBackend(dir)
	t0 := time.Unix(1_700_000_000, 0)
	b1.SetClock(func() time.Time { return t0 })
	c1 := b1.DiskCache()
	if err := c1.Put(ctx, key, CacheValue{Payload: []byte("x")},
		CachePolicy{TTL: time.Hour, PersistToDisk: true}); err != nil {
		t.Fatalf("Put 失败: %v", err)
	}

	// 新实例：时钟拨到过期之后。
	b2 := NewFileBackend(dir)
	b2.SetClock(func() time.Time { return t0.Add(2 * time.Hour) })
	c2 := b2.DiskCache()
	if _, ok, err := c2.Get(ctx, key); err != nil || ok {
		t.Fatalf("过期条目重启后不得复活 (ok=%v err=%v)", ok, err)
	}
	if _, ok, _ := c2.Peek(ctx, key); ok {
		t.Fatal("Peek 也不应报出已过期且被清理的条目")
	}
	// 过期条目的文件应被清理（fail-open 的一部分：不给下次重启留垃圾）。
	entries, _ := filepath.Glob(filepath.Join(dir, "cache", "*.cache"))
	if len(entries) != 0 {
		t.Fatalf("过期条目落盘文件应被删除，实得 %v", entries)
	}
}

// TestCacheCorruptFileFailsOpen：缓存是**可再生**数据，读取损坏时必须
// 丢弃重建（fail-open），而不是像 checkpoint 那样 fail-closed 报错。
func TestCacheCorruptFileFailsOpen(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	key := diskCacheKey("echo", NsCapabilities, "caps")

	b1 := NewFileBackend(dir)
	if err := b1.DiskCache().Put(ctx, key, CacheValue{Payload: []byte("v1")},
		CachePolicy{TTL: time.Hour, PersistToDisk: true}); err != nil {
		t.Fatalf("Put 失败: %v", err)
	}
	// 模拟损坏：截断 + 写垃圾。
	files, _ := filepath.Glob(filepath.Join(dir, "cache", "*.cache"))
	if len(files) != 1 {
		t.Fatalf("应恰有 1 个缓存文件，实得 %v", files)
	}
	if err := os.WriteFile(files[0], []byte("PKGCACH1-corrupted-garbage"), 0o600); err != nil {
		t.Fatalf("制造损坏文件失败: %v", err)
	}

	// 新实例：装载遇到损坏文件 → 丢弃、不报错、继续可用。
	b2 := NewFileBackend(dir)
	c2 := b2.DiskCache()
	if _, ok, err := c2.Get(ctx, key); err != nil || ok {
		t.Fatalf("损坏条目应被丢弃 (ok=%v err=%v)——fail-open 不是 fail-closed", ok, err)
	}
	// 丢弃之后可重建：Put → 跨实例读回。
	if err := c2.Put(ctx, key, CacheValue{Payload: []byte("v2")},
		CachePolicy{TTL: time.Hour, PersistToDisk: true}); err != nil {
		t.Fatalf("损坏后重建 Put 失败: %v", err)
	}
	b3 := NewFileBackend(dir)
	val, ok, err := b3.DiskCache().Get(ctx, key)
	if err != nil || !ok || string(val.Payload) != "v2" {
		t.Fatalf("重建后应可跨实例读回 (ok=%v err=%v val=%q)", ok, err, val.Payload)
	}
}

// TestCacheInvalidateRemovesPersistedFiles：Invalidate 必须连落盘文件一起清，
// 否则"失效"只对本进程有效，重启后失效条目被旧文件复活。
func TestCacheInvalidateRemovesPersistedFiles(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	ns := Namespace("echo", NsModels)

	b1 := NewFileBackend(dir)
	c1 := b1.DiskCache()
	for _, k := range []string{"a", "b"} {
		if err := c1.Put(ctx, CacheKey{Namespace: ns, Key: k}, CacheValue{Payload: []byte(k)},
			CachePolicy{TTL: time.Hour, PersistToDisk: true}); err != nil {
			t.Fatalf("Put(%s) 失败: %v", k, err)
		}
	}
	// 前缀隔离：别家 agent 的持久条目不得被误伤。
	if err := c1.Put(ctx, diskCacheKey("pi", NsModels, "a"), CacheValue{Payload: []byte("pi")},
		CachePolicy{TTL: time.Hour, PersistToDisk: true}); err != nil {
		t.Fatalf("Put(pi) 失败: %v", err)
	}
	if err := c1.Invalidate(ctx, ns); err != nil {
		t.Fatalf("Invalidate 失败: %v", err)
	}

	b2 := NewFileBackend(dir)
	c2 := b2.DiskCache()
	for _, k := range []string{"a", "b"} {
		if _, ok, _ := c2.Get(ctx, CacheKey{Namespace: ns, Key: k}); ok {
			t.Fatalf("Invalidate 后条目 %s 不得跨重启复活", k)
		}
	}
	if _, ok, _ := c2.Get(ctx, diskCacheKey("pi", NsModels, "a")); !ok {
		t.Fatal("Invalidate(echo) 不得波及 pi 的持久条目")
	}
}

// TestDiskCacheOverwriteWithoutFlagUnpersists：同一 key 改用无标志策略覆盖写，
// 落盘文件必须同步移除 —— 否则旧值在重启后压过内存里的新值。
func TestDiskCacheOverwriteWithoutFlagUnpersists(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	key := diskCacheKey("echo", NsProjection, "proj")

	b1 := NewFileBackend(dir)
	c1 := b1.DiskCache()
	if err := c1.Put(ctx, key, CacheValue{Payload: []byte("disk-v")},
		CachePolicy{TTL: time.Hour, PersistToDisk: true}); err != nil {
		t.Fatalf("Put(persist) 失败: %v", err)
	}
	if err := c1.Put(ctx, key, CacheValue{Payload: []byte("mem-v")},
		CachePolicy{TTL: time.Hour}); err != nil {
		t.Fatalf("Put(内存) 失败: %v", err)
	}

	b2 := NewFileBackend(dir)
	if val, ok, _ := b2.DiskCache().Get(ctx, key); ok {
		t.Fatalf("覆盖为无标志后旧值不得复活，实得 %q", val.Payload)
	}
}

// TestDiskCacheOnMemoryBackendIsPureMemory：dataDir 为空 = 纯内存（历史语义）。
// 这一条钉住"既有调用方与测试零分叉"：内存后端绝不创建文件。
func TestDiskCacheOnMemoryBackendIsPureMemory(t *testing.T) {
	ctx := context.Background()
	b := NewBackend()
	c := b.DiskCache()
	key := diskCacheKey("echo", NsModels, "m1")
	if err := c.Put(ctx, key, CacheValue{Payload: []byte("x")},
		CachePolicy{TTL: time.Hour, PersistToDisk: true}); err != nil {
		t.Fatalf("内存后端 Put 失败: %v", err)
	}
	if _, ok, _ := c.Get(ctx, key); !ok {
		t.Fatal("内存后端条目应在本进程内可读（语义与 b.Cache() 一致）")
	}
}
