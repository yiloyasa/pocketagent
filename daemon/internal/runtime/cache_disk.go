package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// 本文件是 CacheManager 的**文件后端**（D7 收尾 A）：带 PersistToDisk 标志的条目
// 落 <dataDir>/cache/，重启后由同一 dataDir 的新实例读回；无标志条目保持纯内存
// 历史语义（NewBackend 的行为零分叉）。backend.go / cache.go 本轮禁改（另一条
// 工作流刚收口），所以接线方式是"装饰器 + 新方法"，不改既有入口的任何行为。
//
// 【写入纪律，逐条对应规格 §8.2 与 checkpoint.go】临时文件 → flush → fsync →
// 原子 rename。Windows 上不再对目录做 fsync 的取舍与 checkpoint.go 相同：
// 该平台没有"刷目录项"的对应语义，文件数据由 tmp.Sync() 保证，文件名落盘交给
// NTFS 日志与 MoveFileEx 的原子替换。
//
// 【与 checkpoint 相反：读取损坏时 fail-open（丢弃重建），不是 fail-closed】
// checkpoint 是权威状态：损坏必须显式报错，转 CORRUPTED 恢复分支，因为"看着成功、
// 实际残缺"等于静默丢用户数据。缓存是**可再生**数据：损坏的唯一代价是"下次多拉
// 一次上游"。为它报错、阻塞轮次或要求人工介入，是把"丢了会重来的东西"当成
// "丢了找不回的东西"对待 —— 方向性错误。所以这里解析失败就删文件、继续跑。

// cacheDir 是持久缓存条目的落盘目录（规格 §6.4 布局里的 cache 段）。
func cacheDir(dataDir string) string { return filepath.Join(dataDir, "cache") }

// diskCacheRegistry 是 *Backend -> *diskCache 的每后端单例注册表。
//
// 【为什么是包级注册表而不是 Backend 字段】backend.go 本轮禁改，不能加字段。
// 【为什么必须单例】fileMu 若随实例重建，两个实例的文件读写就不再互斥 ——
// Windows 上"读者开着目标文件"会让写者的 rename 拿到 ERROR_ACCESS_DENIED
// （checkpoint.go 里的实测记录），hydrate 扫描与 Put 落盘并发时会复现。
var diskCacheRegistry sync.Map

// DiskCache 返回带文件落盘的缓存入口。
//
// dataDir 为空时返回的就是纯内存实现（与 b.Cache() 同一行为）——持久化是
// 部署形态的选择，不改变任何调用方的可见语义；既有测试因此零分叉。
func (b *Backend) DiskCache() CacheManager {
	if b == nil || b.dataDir == "" {
		return b.Cache()
	}
	if v, ok := diskCacheRegistry.Load(b); ok {
		return v.(*diskCache)
	}
	dc, _ := diskCacheRegistry.LoadOrStore(b, &diskCache{b: b, dir: cacheDir(b.dataDir), mem: b.Cache()})
	return dc.(*diskCache)
}

// hydrateDiskCache 触发该后端持久条目的装载（幂等）。
//
// 【为什么 Observe 也要触发】持久条目在磁盘上、不在内存态里；若只在缓存自身
// 被调用时才装载，重启后诊断页（GetAgentRuntime → Observe）会显示 0 ——
// "接了文件后端反而显示 0"，比不显示更误导（observation.go 的同款教训）。
func hydrateDiskCache(b *Backend) {
	if b == nil || b.dataDir == "" {
		return
	}
	if dc, ok := b.DiskCache().(*diskCache); ok {
		dc.ensureHydrated()
	}
}

// diskCache 装饰 memCache：内存态仍是唯一读取路径（TTL/淘汰/统计语义全部不变），
// 磁盘只做两件事——重启时装载回来、写入时把带标志的条目投影出去。
type diskCache struct {
	b   *Backend
	dir string
	mem CacheManager
	// once 保证"每进程每后端只扫一次盘"。Invalidate 会同步删文件，所以
	// 内存态清空后重新扫描也只会看到空目录，不会复活被失效的条目。
	once sync.Once
	// fileMu 串行化全部落盘文件读写（原因见 diskCacheRegistry）。
	fileMu sync.Mutex
}

func (c *diskCache) ensureHydrated() { c.once.Do(c.loadPersisted) }

func (c *diskCache) Get(ctx context.Context, key CacheKey) (CacheValue, bool, error) {
	if err := key.Validate(); err != nil {
		return CacheValue{}, false, err
	}
	c.ensureHydrated()
	return c.mem.Get(ctx, key)
}

func (c *diskCache) Peek(ctx context.Context, key CacheKey) (CacheEntry, bool, error) {
	if err := key.Validate(); err != nil {
		return CacheEntry{}, false, err
	}
	c.ensureHydrated()
	return c.mem.Peek(ctx, key)
}

// Put 先走内存（容量/淘汰/TTL 全部沿用既有语义），再按标志投影到磁盘。
func (c *diskCache) Put(ctx context.Context, key CacheKey, value CacheValue, policy CachePolicy) error {
	if err := key.Validate(); err != nil {
		return err
	}
	c.ensureHydrated()
	if err := c.mem.Put(ctx, key, value, policy); err != nil {
		return err
	}
	if !policy.PersistToDisk {
		// 覆盖为无标志 = 撤回落盘：不同步删文件的话，旧值会在重启后压过内存里的新值。
		c.unpersistEntry(key)
		return nil
	}
	entry, ok := c.storedEntry(key)
	if !ok {
		// 写入后立刻被淘汰/回滚：磁盘上不该留它。
		c.unpersistEntry(key)
		return nil
	}
	// 落盘失败只告警不报错：内存已写成功（调用方语义不受影响），而缓存落盘
	// 是"尽力而为"的投影 —— 报错会把可再生数据的 IO 抖动升级成调用方可见的失败。
	if err := c.persistEntry(entry); err != nil {
		log.Printf("[cache] WARNING: 缓存条目落盘失败（仅影响重启后可用性）key=%s: %v", CacheKeyString(key), err)
	}
	return nil
}

func (c *diskCache) Invalidate(ctx context.Context, namespace string) error {
	c.ensureHydrated()
	if err := c.mem.Invalidate(ctx, namespace); err != nil {
		return err
	}
	// 失效必须连落盘文件一起清：否则"失效"只对本进程有效，重启后被旧文件复活。
	c.removePersistedByNamespace(namespace)
	return nil
}

func (c *diskCache) Stats(ctx context.Context, namespace string) CacheStats {
	c.ensureHydrated()
	return c.mem.Stats(ctx, namespace)
}

// storedEntry 取该 key 在内存里的**存储形态**（含已计算的 Expires 等），
// 落盘必须存这一份而不是调用方传入的原始值 —— 否则两处"怎么算过期时间"
// 会各自演化，重启后 TTL 就分叉了。
func (c *diskCache) storedEntry(key CacheKey) (StoredCacheEntry, bool) {
	b := c.b
	b.mu.Lock()
	defer b.mu.Unlock()
	e, ok := b.cache[cacheStorageKey(key)]
	return e, ok
}

// persistEntry 原子落盘一个条目（临时文件 → fsync → rename，纪律同 checkpoint.go）。
func (c *diskCache) persistEntry(e StoredCacheEntry) error {
	c.fileMu.Lock()
	defer c.fileMu.Unlock()
	if err := os.MkdirAll(c.dir, 0o700); err != nil {
		return fmt.Errorf("runtime: 建缓存目录失败(%s): %w", c.dir, err)
	}
	raw, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("runtime: 序列化缓存条目失败: %w", err)
	}
	// 临时文件与目标同目录：rename 的原子性以同一文件系统为前提（checkpoint.go 同款）。
	tmp, err := os.CreateTemp(c.dir, "cache-*.tmp")
	if err != nil {
		return fmt.Errorf("runtime: 建临时缓存文件失败: %w", err)
	}
	tmpName := tmp.Name()
	fail := func(err error) error {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if _, err := tmp.Write(raw); err != nil {
		return fail(fmt.Errorf("runtime: 写临时缓存文件失败: %w", err))
	}
	if err := tmp.Sync(); err != nil {
		return fail(fmt.Errorf("runtime: fsync 临时缓存文件失败: %w", err))
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("runtime: 关闭临时缓存文件失败: %w", err)
	}
	if err := os.Rename(tmpName, c.pathFor(CacheKey{Namespace: e.Namespace, Key: e.Key})); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("runtime: 原子替换缓存文件失败(namespace=%s key=%s): %w", e.Namespace, e.Key, err)
	}
	return nil
}

// unpersistEntry 移除该 key 的落盘文件（不存在视为成功：多数条目本就没落过盘）。
func (c *diskCache) unpersistEntry(key CacheKey) {
	c.fileMu.Lock()
	defer c.fileMu.Unlock()
	if err := os.Remove(c.pathFor(key)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		log.Printf("[cache] WARNING: 移除缓存落盘文件失败（仅影响重启后一致性）key=%s: %v", CacheKeyString(key), err)
	}
}

// removePersistedByNamespace 清掉命名空间（前缀匹配）内的全部落盘文件。
// 文件名是哈希、反查不了命名空间，所以逐个解开内容比对 —— 目录里只有带标志
// 条目，数量级是个位数到几十，扫描代价可忽略。
func (c *diskCache) removePersistedByNamespace(namespace string) {
	c.fileMu.Lock()
	defer c.fileMu.Unlock()
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return // 目录不存在 = 从没落过盘
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".cache" {
			continue
		}
		path := filepath.Join(c.dir, e.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var stored StoredCacheEntry
		if err := json.Unmarshal(raw, &stored); err != nil {
			continue // 损坏文件留给装载路径清理，这里不抢
		}
		if namespaceMatches(stored.Namespace, namespace) {
			if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
				log.Printf("[cache] WARNING: Invalidate 删落盘文件失败: %v", err)
			}
		}
	}
}

// loadPersisted 把 <dataDir>/cache/ 里的条目装回内存态（fail-open）。
//
// 【为什么直接插入 b.cache 而不走 Put】Put 会应用容量策略：装载阶段触顶被拒
// （EvictNone 报 ErrCacheFull）等于"重启把能恢复的数据拒之门外" —— 与 fail-open
// 相反。直接插入沿用 ImportDurable（backend.go）的同款先例：装载不做策略裁决。
func (c *diskCache) loadPersisted() {
	c.fileMu.Lock()
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		c.fileMu.Unlock()
		return // 目录不存在 = 从没落过盘，不是错误
	}
	now := c.b.clock()
	var toLoad []StoredCacheEntry
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".cache" {
			continue // 残留的 .tmp 等垃圾文件不认识、也不动它
		}
		path := filepath.Join(c.dir, e.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var stored StoredCacheEntry
		if err := json.Unmarshal(raw, &stored); err != nil || stored.Namespace == "" || stored.Key == "" {
			// fail-open 的"丢弃"：删掉损坏文件，只留一行告警。缓存可重生，
			// 保留损坏文件没有 checkpoint 那种"人工修复"的价值。
			_ = os.Remove(path)
			log.Printf("[cache] WARNING: 缓存落盘文件损坏，已丢弃重建（%s）: %v", e.Name(), err)
			continue
		}
		if now.After(stored.Expires) {
			// 过期条目直接清理：TTL 语义不因重启而重置，也不给下次扫描留垃圾。
			_ = os.Remove(path)
			continue
		}
		stored.Persist = true
		toLoad = append(toLoad, stored)
	}
	c.fileMu.Unlock()

	b := c.b
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, stored := range toLoad {
		b.cache[stored.Namespace+"/"+stored.Key] = stored
		b.recomputeCacheLocked(stored.Namespace)
	}
}

// pathFor 把 key 映射到落盘文件名。与 checkpoint.go 的 pathFor 同一条防线：
// id 可能含路径分隔符或 ".."，哈希定名让路径穿越在结构上不可能发生；
// 可读前缀只为排障，唯一性由哈希后缀负责。
func (c *diskCache) pathFor(key CacheKey) string {
	sk := cacheStorageKey(key)
	sum := sha256.Sum256([]byte(sk))
	return filepath.Join(c.dir, fmt.Sprintf("%s-%s.cache", checkpointFileComponent(sk), hex.EncodeToString(sum[:8])))
}

// 编译期锚：diskCache 必须始终满足 CacheManager（换实现时先在这里断）。
var _ CacheManager = (*diskCache)(nil)
