package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pocketagent/daemon/internal/store"
)

// Backend 是一套默认实现的共享状态容器（规格 §13 P0.5「最低契约」的落地件）。
//
// 【为什么必须有默认实现】契约的设计意图是"没有原生能力的 agent 也不退化成
// 无记忆、无缓存、无恢复的临时执行器"。如果契约只给接口，那家 agent 的接入成本
// 就变成"先自己实现一套 memory+cache+checkpoint+resource"，结果它继续空白 ——
// 契约反而成了空转。Backend 让这类 agent 一行接线即达标；日后想换成原生实现，
// 只需替换 AgentRuntime 里的单个字段，契约不变。
//
// 【它不做什么】不生成语义摘要（那要有模型）、不落盘到文件（P1 的 journal 负责）、
// 不做删除（见 ResourceLedger 的注释）。本实现只保证**契约语义**成立。
//
// 【并发】单锁保护。按"单 daemon、少量 thread"的量级设计；分片是 P1 资源管理的事。
type Backend struct {
	mu sync.Mutex
	// now 可注入时钟（契约测试用）；nil = time.Now。
	now func() time.Time
	// dataDir 是文件版 checkpoint 的落盘根目录（实际写在 <dataDir>/checkpoints/）。
	//
	// 【为什么设成字段而不是给每个入口都加参数】更换 checkpoint 的持久化方式不该
	// 改变四个入口的签名 —— 那会让每个调用点都要选一次实现。空字符串 = 内存实现，
	// 即 NewBackend 的历史语义（进程内有效、零落盘），它保证了既有调用方与
	// 大量共享单后端实例的测试行为零分叉。需要跨重启时用 NewFileBackend。
	dataDir string
	// ckptMu 串行化文件 checkpoint 的读写（目录为空时无用）。
	//
	// 【为什么读也要参与互斥】实测证据：Windows 上 os.Rename 走
	// MoveFileEx(REPLACE_EXISTING)，而 Go 的 syscall.Open 只授
	// FILE_SHARE_READ|FILE_SHARE_WRITE（无 FILE_SHARE_DELETE）。于是**并发读者
	// 持有的句柄**会让写者替换目标时拿到 ERROR_ACCESS_DENIED —— 这是"读者阻塞
	// 写者"，不是写者互撞（把写者串成互斥后仍然复现，见测试）。用读写锁让写者
	// 等到没有读者在打开文件时再替换，读者之间仍可并行。POSIX 无此限制，但共用
	// 同一把锁不损失正确性。
	//
	// 【为什么单独一把锁，不是复用 mu】写 checkpoint 会 fsync（毫秒级），挂到 mu 上
	// 会让记忆/缓存/记账这些与磁盘无关的操作在 fsync 期间一起阻塞。
	ckptMu sync.RWMutex

	// ---- 语义记忆（RAM，重启即失；靠 Checkpoint 落盘）----
	seq     map[string]int64         // memory key -> sequence 高水位
	items   map[string][]MemoryItem  // memory key -> 未压缩的项
	covered map[string]SequenceRange // memory key -> 已压缩覆盖范围
	digest  map[string]string        // memory key -> 摘要正文
	version map[string]int64         // memory key -> 写入版本

	// ---- 落盘态（重启后仍在）----
	durable     map[string][]byte // checkpoint key -> 内容
	sessionRefs map[string]bool   // threadID -> 已建立的原生会话
	persistSeq  map[string]int64  // memory key -> 落盘的 sequence 高水位

	// ---- 可再生缓存 ----
	cache map[string]StoredCacheEntry // namespace+"/"+key -> 条目
	stats map[string]*CacheStats      // namespace -> 统计

	// ---- 资源记账 ----
	leases map[string]*ResourceLease
}

// StoredCacheEntry 是缓存条目的存储形态（导出以便将来的文件实现与外部测试复用）。
type StoredCacheEntry struct {
	Namespace    string     `json:"namespace"`
	Key          string     `json:"key"`
	Value        CacheValue `json:"value"`
	Expires      time.Time  `json:"expires"`
	StaleAllowed bool       `json:"stale_allowed"`
	// Persist 标记该条目已声明落盘（PersistToDisk=true），重启后保留。
	Persist bool `json:"persist"`
	// Inserted 写入序号（FIFO 排序）与 LastUsed（LRU 排序）。
	Inserted int64     `json:"inserted"`
	LastUsed time.Time `json:"last_used"`
}

// NewBackend 建一个空的默认后端（checkpoint 为内存实现，进程内有效）。
func NewBackend() *Backend {
	b := &Backend{}
	b.reset()
	return b
}

// NewFileBackend 建一个把 checkpoint 落到 dataDir 下的后端（规格 §8.2 的写入纪律）。
//
// 【为什么另起一个构造函数，而不是改 NewBackend 的签名】checkpoint 的持久化方式
// 是**部署形态**的选择，不是每个调用点都该操心的事：改了 NewBackend 签名会让
// 全部既有调用方被迫提供一个数据目录（它们中的多数是测试，共享一个进程内后端，
// 落盘反而会让用例互相污染）。这里沿用 identity.go 的约定 —— dataDir 为空退化为
// 内存实现，非空才落盘 —— 让"要不要持久化"变成一处显式决定。
//
// 【它只改 checkpoint】memory/cache/资源记账仍走内存实现：本包的 memory 目前靠
// Backend 内部的落盘态（ExportDurable/ImportDurable）表达，不在此次范围内。
func NewFileBackend(dataDir string) *Backend {
	b := &Backend{dataDir: dataDir}
	b.reset()
	return b
}

// SetClock 注入时钟（契约测试用：验证 TTL 到期与过期降级）。传 nil 恢复真实时钟。
func (b *Backend) SetClock(now func() time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.now = now
}

func (b *Backend) reset() {
	b.seq = map[string]int64{}
	b.items = map[string][]MemoryItem{}
	b.covered = map[string]SequenceRange{}
	b.digest = map[string]string{}
	b.version = map[string]int64{}
	b.durable = map[string][]byte{}
	b.sessionRefs = map[string]bool{}
	b.persistSeq = map[string]int64{}
	b.cache = map[string]StoredCacheEntry{}
	b.stats = map[string]*CacheStats{}
	b.leases = map[string]*ResourceLease{}
}

func (b *Backend) clock() time.Time {
	if b.now != nil {
		return b.now()
	}
	return time.Now()
}

// ---- 四个入口 ----------------------------------------------------------------

// Memory 返回默认语义记忆入口。
func (b *Backend) Memory() MemoryManager { return &memMemory{b: b} }

// Cache 返回默认缓存入口。
func (b *Backend) Cache() CacheManager { return &memCache{b: b} }

// Resources 返回默认资源记账入口。
func (b *Backend) Resources() ResourceLedger { return &memLedger{b: b} }

// Checkpoints 返回 checkpoint 入口。
//
// dataDir 为空时是内存实现（进程内有效）；非空时是文件实现（跨重启有效，
// 写入顺序见规格 §8.2 与 checkpoint.go）。两者的**语义与返回形状必须逐条一致**
// —— 换实现不该让调用方观察到行为差异，只该观察到"重启后还在不在"。
func (b *Backend) Checkpoints() CheckpointManager {
	if b.dataDir != "" {
		return &fileCheckpoints{dir: checkpointsDir(b.dataDir), mu: &b.ckptMu}
	}
	return &memCheckpoints{b: b}
}

// DaemonSession 返回"daemon 侧持有会话事实"的默认会话适配器（Source=DAEMON）。
// 无原生会话能力的 agent（Echo / OpenAI 兼容）用它即可满足契约。
func (b *Backend) DaemonSession() NativeSessionAdapter { return &memSession{b: b} }

// FullRuntime 组装一份完整的 AgentRuntime（四个入口 + 默认会话适配器全就位）。
//
// 【用法】provider 的 OpenRuntime 里一行：
//
//	rt := backend.FullRuntime(descriptor, thread)
//
// 需要替换其中某一项时（例如接了真的原生 session），改 rt.NativeSession 即可。
func (b *Backend) FullRuntime(desc AgentDescriptor, threadID string) *AgentRuntime {
	return &AgentRuntime{
		Descriptor:    desc,
		AgentID:       desc.ID,
		ThreadID:      threadID,
		Memory:        b.Memory(),
		Cache:         b.Cache(),
		Resources:     b.Resources(),
		Checkpoint:    b.Checkpoints(),
		NativeSession: b.DaemonSession(),
	}
}

// ---- 记忆 ----------------------------------------------------------------

type memMemory struct{ b *Backend }

var _ MemoryManager = (*memMemory)(nil)

func (m *memMemory) Load(_ context.Context, key MemoryKey) (*MemorySnapshot, error) {
	if err := key.Validate(); err != nil {
		return nil, err
	}
	m.b.mu.Lock()
	defer m.b.mu.Unlock()
	return m.b.snapshotLocked(key), nil
}

// snapshotLocked 必须在 b.mu 持有时调用。
func (b *Backend) snapshotLocked(key MemoryKey) *MemorySnapshot {
	ks := key.String()
	src := b.items[ks]
	out := &MemorySnapshot{
		Key:     key,
		Items:   append([]MemoryItem(nil), src...),
		Covered: b.covered[ks],
		Digest:  b.digest[ks],
		Version: b.version[ks],
		At:      b.clock(),
	}
	for i := range out.Items {
		out.Bytes += int64(len(out.Items[i].Payload))
	}
	return out
}

func (m *memMemory) Append(_ context.Context, key MemoryKey, item MemoryItem) (int64, error) {
	if err := key.Validate(); err != nil {
		return 0, err
	}
	b := m.b
	b.mu.Lock()
	defer b.mu.Unlock()
	ks := key.String()
	b.seq[ks]++
	seq := b.seq[ks]
	item.Sequence = seq
	if item.Kind == "" {
		item.Kind = key.Kind
	}
	if item.At.IsZero() {
		item.At = b.clock()
	}
	b.items[ks] = append(b.items[ks], item)
	b.version[ks]++
	return seq, nil
}

func (m *memMemory) ReadWindow(_ context.Context, key MemoryKey, limit MemoryLimit) ([]MemoryItem, error) {
	if err := key.Validate(); err != nil {
		return nil, err
	}
	m.b.mu.Lock()
	defer m.b.mu.Unlock()

	var candidates []MemoryItem
	for _, it := range m.b.items[key.String()] {
		if limit.FromSequence > 0 && it.Sequence <= limit.FromSequence {
			continue
		}
		candidates = append(candidates, it)
	}

	// 窗口语义 = **最近**：从最新往回取满条数/字节上限，再按由旧到新返回。
	// （上下文构建的主流用法是"给模型看最近 N 条"，从头截会在长会话里
	//   把用户的上一句话截掉。）单条自身超预算时仍返回该条 —— 返回空集合
	// 会让调用方误判为"这个会话没有内容"。
	var chosen []MemoryItem
	var bytes int64
	for i := len(candidates) - 1; i >= 0; i-- {
		it := candidates[i]
		if limit.MaxItems > 0 && len(chosen) >= limit.MaxItems {
			break
		}
		sz := int64(len(it.Payload))
		if limit.MaxBytes > 0 && len(chosen) > 0 && bytes+sz > limit.MaxBytes {
			break
		}
		chosen = append(chosen, it)
		bytes += sz
	}
	for i, j := 0, len(chosen)-1; i < j; i, j = i+1, j-1 {
		chosen[i], chosen[j] = chosen[j], chosen[i]
	}
	return chosen, nil
}

func (m *memMemory) Compact(_ context.Context, key MemoryKey, policy CompactPolicy) (*MemorySnapshot, error) {
	if err := key.Validate(); err != nil {
		return nil, err
	}
	b := m.b
	b.mu.Lock()
	defer b.mu.Unlock()
	ks := key.String()
	all := b.items[ks]
	if len(all) == 0 {
		return b.snapshotLocked(key), nil
	}

	cut := len(all) - policy.KeepRecentItems
	if cut < 0 {
		cut = 0
	}
	if cut > len(all) {
		cut = len(all)
	}
	// TargetBytes：在不违反 KeepRecentItems 的前提下继续往前扩大压缩范围。
	if policy.TargetBytes > 0 && cut > 0 {
		var keptBytes int64
		for _, it := range all[cut:] {
			keptBytes += int64(len(it.Payload))
		}
		for cut > 0 && keptBytes > policy.TargetBytes {
			cut--
			keptBytes += int64(len(all[cut].Payload))
		}
	}
	if cut <= 0 {
		return b.snapshotLocked(key), nil
	}

	compacted := all[:cut]
	// 关键：被压缩掉的 sequence 全部记进 Covered，摘要正文记进 Digest。
	// 这两件一起构成"替换关系"，是"不静默丢数据"的唯一凭据。
	rng := b.covered[ks]
	for _, it := range compacted {
		rng = rng.Extend(it.Sequence)
	}
	b.covered[ks] = rng
	if d := structuralDigest(compacted); d != "" {
		if prev := b.digest[ks]; prev != "" {
			b.digest[ks] = prev + "\n" + d
		} else {
			b.digest[ks] = d
		}
	}
	b.items[ks] = append([]MemoryItem(nil), all[cut:]...)
	b.version[ks]++
	return b.snapshotLocked(key), nil
}

// structuralDigest 生成**结构摘要**（条数 + seq 范围 + 角色分布）。
//
// 【这不是语义摘要，刻意如此】语义摘要要模型生成，本包不调模型。默认后端给出
// 结构摘要，保证"压缩有据可查"；真正的语义摘要由 agent 侧写进 MemorySummary 槽。
// 把两者混为一谈会导致契约测试依赖模型输出，进而不可复现。
func structuralDigest(items []MemoryItem) string {
	if len(items) == 0 {
		return ""
	}
	roles := map[string]int{}
	for _, it := range items {
		r := it.Role
		if r == "" {
			r = "(无角色)"
		}
		roles[r]++
	}
	keys := make([]string, 0, len(roles))
	for k := range roles {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	fmt.Fprintf(&sb, "已压缩 %d 条（seq %d..%d）", len(items), items[0].Sequence, items[len(items)-1].Sequence)
	for _, k := range keys {
		fmt.Fprintf(&sb, "；%s=%d", k, roles[k])
	}
	return sb.String()
}

func (m *memMemory) Checkpoint(_ context.Context, key MemoryKey) error {
	if err := key.Validate(); err != nil {
		return err
	}
	b := m.b
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.checkpointMemoryLocked(key)
}

func (b *Backend) checkpointMemoryLocked(key MemoryKey) error {
	ks := key.String()
	snap := b.snapshotLocked(key)
	payload, err := json.Marshal(snap)
	if err != nil {
		return fmt.Errorf("runtime: 序列化记忆失败: %w", err)
	}
	ck := CheckpointKey{AgentID: key.AgentID, ThreadID: key.ThreadID, Name: memoryCheckpointName(key)}
	b.durable[ck.String()] = payload
	b.persistSeq[ks] = b.seq[ks]
	return nil
}

// memoryCheckpointName 把 MemoryKey 映射到 checkpoint 名（kind + scope）。
func memoryCheckpointName(key MemoryKey) string {
	if key.Scope == "" {
		return "memory:" + string(key.Kind)
	}
	return "memory:" + string(key.Kind) + ":" + key.Scope
}

func (m *memMemory) Restore(_ context.Context, key MemoryKey) error {
	if err := key.Validate(); err != nil {
		return err
	}
	b := m.b
	b.mu.Lock()
	defer b.mu.Unlock()
	ks := key.String()
	ck := CheckpointKey{AgentID: key.AgentID, ThreadID: key.ThreadID, Name: memoryCheckpointName(key)}
	raw, ok := b.durable[ck.String()]
	if !ok {
		// 没有 checkpoint 时**不动内存态**：把"没落盘过"当成"清空记忆"会
		// 在恢复路径上静默丢数据。调用方要靠 found 语义判断，而不是靠副作用。
		return nil
	}
	var snap MemorySnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return fmt.Errorf("runtime: 反序列化 checkpoint 失败(%s): %w", ck, err)
	}
	b.items[ks] = append([]MemoryItem(nil), snap.Items...)
	b.covered[ks] = snap.Covered
	b.digest[ks] = snap.Digest
	b.version[ks] = snap.Version
	// sequence 高水位只能升不能降：否则 Append 会复用已发出过的 sequence，
	// 而手机端是按 sequence 增量补帧的 —— 复用会让它对不上账。
	if b.seq[ks] < b.persistSeq[ks] {
		b.seq[ks] = b.persistSeq[ks]
	}
	return nil
}

// ---- 缓存 ----------------------------------------------------------------

type memCache struct{ b *Backend }

var _ CacheManager = (*memCache)(nil)

var (
	// ErrCacheFull 在 Eviction=EvictNone 且已满时返回（宁可让上游多打一次，也不悄悄丢）。
	ErrCacheFull = errors.New("runtime: 缓存已达配置上限且策略为不接受淘汰")
)

func cacheStorageKey(k CacheKey) string { return k.Namespace + "/" + k.Key }

func (c *memCache) Get(_ context.Context, key CacheKey) (CacheValue, bool, error) {
	if err := key.Validate(); err != nil {
		return CacheValue{}, false, err
	}
	b := c.b
	b.mu.Lock()
	defer b.mu.Unlock()
	entry, ok := b.cache[cacheStorageKey(key)]
	now := b.clock()
	if !ok {
		b.cacheStat(key.Namespace).Misses++
		return CacheValue{}, false, nil
	}
	entry.LastUsed = now
	b.cache[cacheStorageKey(key)] = entry
	if now.After(entry.Expires) {
		// Get 是**严格**路径：过期即未命中。需要"过期但可用"的降级走 Peek+DecideRead。
		// 理由：Get 的调用方通常不会再看新鲜度，宽松默认会让陈旧值冒充新值。
		b.cacheStat(key.Namespace).Misses++
		return CacheValue{}, false, nil
	}
	b.cacheStat(key.Namespace).Hits++
	return entry.Value, true, nil
}

func (c *memCache) Peek(_ context.Context, key CacheKey) (CacheEntry, bool, error) {
	if err := key.Validate(); err != nil {
		return CacheEntry{}, false, err
	}
	b := c.b
	b.mu.Lock()
	defer b.mu.Unlock()
	entry, ok := b.cache[cacheStorageKey(key)]
	if !ok {
		return CacheEntry{}, false, nil
	}
	b.cache[cacheStorageKey(key)] = withLastUsed(entry, b.clock())
	return CacheEntry{
		Value:        entry.Value,
		Fresh:        !b.clock().After(entry.Expires),
		StaleAllowed: entry.StaleAllowed,
	}, true, nil
}

func withLastUsed(e StoredCacheEntry, at time.Time) StoredCacheEntry {
	e.LastUsed = at
	return e
}

func (c *memCache) Put(_ context.Context, key CacheKey, value CacheValue, policy CachePolicy) error {
	if err := key.Validate(); err != nil {
		return err
	}
	b := c.b
	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.clock()
	if value.At.IsZero() {
		value.At = now
	}
	expires := value.Expires
	if expires.IsZero() {
		expires = now.Add(policy.TTL)
	}
	sk := cacheStorageKey(key)
	inserted := now.UnixNano()
	if prev, ok := b.cache[sk]; ok {
		inserted = prev.Inserted // 覆盖写不改变 FIFO 位次
	}
	b.cache[sk] = StoredCacheEntry{
		Namespace:    key.Namespace,
		Key:          key.Key,
		Value:        value,
		Expires:      expires,
		StaleAllowed: policy.StaleAllowed,
		Persist:      policy.PersistToDisk,
		Inserted:     inserted,
		LastUsed:     now,
	}
	b.cacheStat(key.Namespace).Entries = 0 // 由 recompute 统一重算

	if err := b.enforceCachePolicyLocked(key.Namespace, policy); err != nil {
		delete(b.cache, sk) // 回滚：拒绝写入时不留下半个条目
		b.recomputeCacheLocked(key.Namespace)
		return err
	}
	b.recomputeCacheLocked(key.Namespace)
	return nil
}

// enforceCachePolicyLocked 按策略淘汰到容量以内。EvictNone 时不淘汰，直接报满。
func (b *Backend) enforceCachePolicyLocked(namespace string, policy CachePolicy) error {
	for {
		entries, bytes := b.cacheUsageLocked(namespace)
		overEntries := policy.MaxEntries > 0 && entries > policy.MaxEntries
		overBytes := policy.MaxBytes > 0 && bytes > policy.MaxBytes
		if !overEntries && !overBytes {
			return nil
		}
		if policy.Eviction == EvictNone || policy.Eviction == "" {
			return ErrCacheFull
		}
		victim, ok := b.cacheVictimLocked(namespace, policy.Eviction)
		if !ok {
			return ErrCacheFull
		}
		delete(b.cache, victim)
		b.cacheStat(namespace).Evictions++
	}
}

func (b *Backend) cacheUsageLocked(namespace string) (int, int64) {
	n, varBytes := 0, int64(0)
	for sk, e := range b.cache {
		if !namespaceMatches(e.Namespace, namespace) {
			continue
		}
		_ = sk
		n++
		varBytes += int64(len(e.Value.Payload))
	}
	return n, varBytes
}

// namespaceMatches 报告条目命名空间是否落在待统计/待失效的范围内。
//
// Invalidate("agent/echo") 必须能清掉 "agent/echo/models" 下的全部条目 ——
// 精确相等匹配会让"整家 agent 失效"变成一个静默无效的操作。
//
// 容忍尾斜杠：调用方可能写成 "agent/echo" 也可能写成 "agent/echo/"（Namespace(id, "")
// 就产出后者）。不做这个归一化，同一语义的两种写法会一个生效一个不生效 ——
// 而"不生效"是静默的。
func namespaceMatches(entryNS, query string) bool {
	if query == "" {
		return true
	}
	q := strings.Trim(query, "/")
	if q == "" {
		return true
	}
	return entryNS == q || strings.HasPrefix(entryNS, q+"/")
}

func (b *Backend) cacheVictimLocked(namespace string, policy EvictionPolicy) (string, bool) {
	keys := make([]string, 0, len(b.cache))
	for sk, e := range b.cache {
		if namespaceMatches(e.Namespace, namespace) {
			keys = append(keys, sk)
		}
	}
	if len(keys) == 0 {
		return "", false
	}
	sort.Strings(keys) // 决定论：同分时按 key 排序，避免 map 遍历顺序造成的不稳定淘汰
	victim := keys[0]
	for _, sk := range keys[1:] {
		cur, next := b.cache[victim], b.cache[sk]
		switch policy {
		case EvictFIFO:
			if next.Inserted < cur.Inserted {
				victim = sk
			}
		default: // EvictLRU
			if next.LastUsed.Before(cur.LastUsed) {
				victim = sk
			}
		}
	}
	return victim, true
}

func (b *Backend) recomputeCacheLocked(namespace string) {
	st := b.cacheStat(namespace)
	n, bytes := b.cacheUsageLocked(namespace)
	st.Entries, st.Bytes, st.Namespace = n, bytes, namespace
}

func (b *Backend) cacheStat(namespace string) *CacheStats {
	st, ok := b.stats[namespace]
	if !ok {
		st = &CacheStats{Namespace: namespace}
		b.stats[namespace] = st
	}
	return st
}

func (c *memCache) Invalidate(_ context.Context, namespace string) error {
	b := c.b
	b.mu.Lock()
	defer b.mu.Unlock()
	for sk, e := range b.cache {
		if namespaceMatches(e.Namespace, namespace) {
			delete(b.cache, sk)
		}
	}
	if namespace != "" {
		b.recomputeCacheLocked(namespace)
	}
	return nil
}

func (c *memCache) Stats(_ context.Context, namespace string) CacheStats {
	b := c.b
	b.mu.Lock()
	defer b.mu.Unlock()
	if namespace != "" {
		st := *b.cacheStat(namespace)
		b.recomputeCacheLocked(namespace)
		st.Entries, st.Bytes = b.cacheUsageLocked(namespace)
		return st
	}
	out := CacheStats{}
	for ns := range b.stats {
		s := b.cacheStat(ns)
		n, bytes := b.cacheUsageLocked(ns)
		s.Entries, s.Bytes, s.Namespace = n, bytes, ns
		out.Entries += n
		out.Bytes += bytes
		out.Hits += s.Hits
		out.Misses += s.Misses
		out.Evictions += s.Evictions
	}
	return out
}

// ---- 资源记账 ----------------------------------------------------------------

type memLedger struct{ b *Backend }

var _ ResourceLedger = (*memLedger)(nil)

var (
	// ErrLeaseExists 表示租约 id 已被占用（防止两个 owner 共用一个 id 互相记账）。
	ErrLeaseExists = errors.New("runtime: 资源租约 id 已存在")
)

func (l *memLedger) Acquire(_ context.Context, req LeaseRequest) (*ResourceLease, error) {
	if req.LeaseID == "" {
		return nil, fmt.Errorf("runtime: 租约必须有 lease_id")
	}
	b := l.b
	b.mu.Lock()
	defer b.mu.Unlock()
	if cur, ok := b.leases[req.LeaseID]; ok && cur.ReleasedAt.IsZero() {
		return nil, fmt.Errorf("%w: %s", ErrLeaseExists, req.LeaseID)
	}
	// 硬上限只拒绝**可选**资源；必需资源（保存用户消息等）允许越限记账，
	// 由 Usage 如实反映压力 —— 拒掉必需资源等于换个名字丢用户数据。
	if req.Optional {
		used := b.usedLocked(req.OwnerScope, req.Kind)
		if ClassifyPressure(used+req.ReservedBytes, req.SoftLimit, req.HardLimit) == PressureHard {
			return nil, fmt.Errorf("%w: scope=%s kind=%s used=%d reserve=%d hard=%d",
				ErrOverHardLimit, req.OwnerScope, req.Kind, used, req.ReservedBytes, req.HardLimit)
		}
	}
	lease := &ResourceLease{
		LeaseID:       req.LeaseID,
		OwnerScope:    req.OwnerScope,
		Kind:          req.Kind,
		ReservedBytes: req.ReservedBytes,
		UsedBytes:     req.ReservedBytes,
		SoftLimit:     req.SoftLimit,
		HardLimit:     req.HardLimit,
		SpillPolicy:   req.SpillPolicy,
		Optional:      req.Optional,
		CreatedAt:     b.clock(),
	}
	b.leases[req.LeaseID] = lease
	return leaseCopy(lease), nil
}

func (l *memLedger) Charge(_ context.Context, leaseID string, bytes int64) (*ResourceLease, error) {
	b := l.b
	b.mu.Lock()
	defer b.mu.Unlock()
	lease, ok := b.leases[leaseID]
	if !ok || !lease.ReleasedAt.IsZero() {
		return nil, fmt.Errorf("%w: %s", ErrLeaseNotFound, leaseID)
	}
	next := lease.UsedBytes + bytes
	if lease.Optional && lease.HardLimit > 0 && next >= lease.HardLimit {
		return nil, fmt.Errorf("%w: lease=%s used=%d charge=%d hard=%d",
			ErrOverHardLimit, leaseID, lease.UsedBytes, bytes, lease.HardLimit)
	}
	lease.UsedBytes = next
	return leaseCopy(lease), nil
}

func (l *memLedger) Release(_ context.Context, leaseID string) error {
	b := l.b
	b.mu.Lock()
	defer b.mu.Unlock()
	lease, ok := b.leases[leaseID]
	if !ok {
		return fmt.Errorf("%w: %s", ErrLeaseNotFound, leaseID)
	}
	if !lease.ReleasedAt.IsZero() {
		return nil // 幂等：Release 常在 defer 里，重复释放不该报错
	}
	lease.ReleasedAt = b.clock()
	lease.UsedBytes = 0
	return nil
}

func leaseCopy(l *ResourceLease) *ResourceLease {
	cp := *l
	return &cp
}

// usedLocked 汇总某 (scope, kind) 下**活跃**租约的已用量。
func (b *Backend) usedLocked(scope string, kind ResourceKind) int64 {
	var total int64
	for _, l := range b.leases {
		if !l.ReleasedAt.IsZero() {
			continue
		}
		if scope != "" && l.OwnerScope != scope {
			continue
		}
		if kind != "" && l.Kind != kind {
			continue
		}
		total += l.UsedBytes
	}
	return total
}

func (l *memLedger) Usage(_ context.Context, scope string) UsageReport {
	b := l.b
	b.mu.Lock()
	defer b.mu.Unlock()
	rep := UsageReport{Scope: scope, ByKind: map[ResourceKind]int64{}}
	for _, lease := range b.leases {
		if !lease.ReleasedAt.IsZero() {
			continue
		}
		if scope != "" && lease.OwnerScope != scope {
			continue
		}
		rep.Leases++
		rep.ByKind[lease.Kind] += lease.UsedBytes
		rep.Items = append(rep.Items, *leaseCopy(lease))
		switch lease.Pressure() {
		case PressureHard:
			rep.OverHard = append(rep.OverHard, lease.LeaseID)
		case PressureSoft:
			rep.OverSoft = append(rep.OverSoft, lease.LeaseID)
		}
	}
	// 租约存在 map 里，遍历顺序随机 —— 不排序会让"同一状态两次查询"给出不同次序，
	// 手机端做前后对比时就会看到假的抖动。
	sort.Slice(rep.Items, func(i, j int) bool { return rep.Items[i].LeaseID < rep.Items[j].LeaseID })
	sort.Strings(rep.OverHard)
	sort.Strings(rep.OverSoft)
	return rep
}

// ---- checkpoint --------------------------------------------------------------

type memCheckpoints struct{ b *Backend }

var _ CheckpointManager = (*memCheckpoints)(nil)

func (c *memCheckpoints) Save(_ context.Context, key CheckpointKey, payload []byte) error {
	if err := key.Validate(); err != nil {
		return err
	}
	b := c.b
	b.mu.Lock()
	defer b.mu.Unlock()
	// 内存实现天然原子。文件实现必须按规格 §8.2 的
	// 「临时文件 → flush → fsync → atomic rename」来做，否则崩溃时会读到半截内容。
	b.durable[key.String()] = append([]byte(nil), payload...)
	return nil
}

func (c *memCheckpoints) Load(_ context.Context, key CheckpointKey) ([]byte, bool, error) {
	if err := key.Validate(); err != nil {
		return nil, false, err
	}
	b := c.b
	b.mu.Lock()
	defer b.mu.Unlock()
	raw, ok := b.durable[key.String()]
	if !ok {
		return nil, false, nil
	}
	return append([]byte(nil), raw...), true, nil
}

func (c *memCheckpoints) Drop(_ context.Context, key CheckpointKey) error {
	if err := key.Validate(); err != nil {
		return err
	}
	b := c.b
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.durable, key.String())
	return nil
}

// ---- 会话（Source=DAEMON）----------------------------------------------------

type memSession struct{ b *Backend }

var _ NativeSessionAdapter = (*memSession)(nil)

var (
	// ErrNoNativeSession 表示没有可恢复的会话引用。
	ErrNoNativeSession = errors.New("runtime: 没有可恢复的会话引用")
)

func (s *memSession) Source() SessionSource { return SourceDaemon }

func (s *memSession) Open(_ context.Context, thread *store.Thread) error {
	if thread == nil {
		return fmt.Errorf("runtime: thread 为空")
	}
	b := s.b
	b.mu.Lock()
	defer b.mu.Unlock()
	b.sessionRefs[thread.ID] = true
	// 引用写回 thread，由调用方（server 层）负责持久化 —— 与 pi/zcode 的
	// ExternalSession 落库同一约定。
	thread.ExternalSession.SessionID = "daemon:" + thread.ID
	return nil
}

func (s *memSession) Resume(_ context.Context, thread *store.Thread) error {
	if thread == nil {
		return fmt.Errorf("runtime: thread 为空")
	}
	s.b.mu.Lock()
	defer s.b.mu.Unlock()
	if thread.ExternalSession.SessionID == "" || !s.b.sessionRefs[thread.ID] {
		return fmt.Errorf("%w: thread=%s", ErrNoNativeSession, thread.ID)
	}
	return nil
}

func (s *memSession) Checkpoint(_ context.Context, _ *store.Thread) error {
	// 引用已经写在 thread 上，由调用方持久化；这里没有额外状态要落。
	return nil
}

func (s *memSession) Close(_ context.Context, thread *store.Thread) error {
	if thread == nil {
		return fmt.Errorf("runtime: thread 为空")
	}
	s.b.mu.Lock()
	defer s.b.mu.Unlock()
	// 只释放句柄，**不删会话内容** —— 关会话不等于删会话。
	delete(s.b.sessionRefs, thread.ID)
	return nil
}

// ---- 持久态导出 / 导入（模拟 daemon 重启）------------------------------------

// durableState 是**只有落盘部分**的状态快照。
//
// 【为什么不导出内存态】真实的 daemon 重启会丢掉全部 RAM。如果导出里带上
// 内存记忆，契约测试就会"重启后记忆还在"地全绿，而生产环境并非如此 ——
// 那是一条假绿。这里刻意只带落盘部分，让"没 checkpoint 的记忆会丢"这件事
// 在测试里显式成立。
type durableState struct {
	Checkpoints map[string][]byte           `json:"checkpoints"`
	PersistSeq  map[string]int64            `json:"persist_seq"`
	SessionRefs map[string]bool             `json:"session_refs"`
	Persist     map[string]StoredCacheEntry `json:"persist_cache"`
}

// ExportDurable 导出落盘态（模拟"关闭 daemon 前磁盘上有什么"）。
func (b *Backend) ExportDurable() ([]byte, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	st := durableState{
		Checkpoints: b.durable,
		PersistSeq:  b.persistSeq,
		SessionRefs: b.sessionRefs,
		Persist:     map[string]StoredCacheEntry{},
	}
	for sk, e := range b.cache {
		if e.Persist {
			st.Persist[sk] = e
		}
	}
	return json.Marshal(st)
}

// ImportDurable 用落盘态重置后端，**清空全部内存态**（模拟 daemon 重启）。
//
// 注意 sequence 高水位随落盘态一起回来：不这么做，重启后 Append 会复用已发出过的
// sequence，而手机端是按 sequence 增量补帧的 —— 复用即对不上账。
func (b *Backend) ImportDurable(data []byte) error {
	var st durableState
	if err := json.Unmarshal(data, &st); err != nil {
		return fmt.Errorf("runtime: 恢复落盘态失败: %w", err)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.reset()
	if st.Checkpoints != nil {
		b.durable = st.Checkpoints
	}
	if st.PersistSeq != nil {
		b.persistSeq = st.PersistSeq
	}
	if st.SessionRefs != nil {
		b.sessionRefs = st.SessionRefs
	}
	for sk, e := range st.Persist {
		e.Persist = true
		b.cache[sk] = e
		b.recomputeCacheLocked(e.Namespace)
	}
	for ks, v := range b.persistSeq {
		b.seq[ks] = v
	}
	return nil
}

// ---- 契约不变量 --------------------------------------------------------------

// MemoryIntegrityError 描述一处记忆完整性问题。
type MemoryIntegrityError struct {
	Key      MemoryKey
	Sequence int64
	Reason   string
}

func (e MemoryIntegrityError) Error() string {
	return fmt.Sprintf("runtime: 记忆完整性破坏 key=%s seq=%d: %s", e.Key, e.Sequence, e.Reason)
}

// CheckMemoryIntegrity 校验"没有任何 sequence 既不在摘要覆盖范围内、也不在保留项里"。
//
// 【为什么这是可执行判据】规格 §6.1 要求"会话历史是用户数据或可恢复过程数据，
// 不得被当作普通缓存静默删除"。"静默"的精确含义就是：这个 sequence 没人能再解释
// 它去哪了。因此判据可以写成一道集合题 —— everAppended 里的每个 sequence，
// 必须 либо落在 snapshot.Covered 内，要么出现在 snapshot.Items 里。
//
// 契约测试用它证明压缩没丢东西；任何人改了 Compact 都会在这里被拦下。
func CheckMemoryIntegrity(s *MemorySnapshot, everAppended []int64) error {
	if s == nil {
		return fmt.Errorf("runtime: 快照为空")
	}
	retained := make(map[int64]bool, len(s.Items))
	for _, it := range s.Items {
		retained[it.Sequence] = true
	}
	for _, seq := range everAppended {
		if s.Covered.Covers(seq) || retained[seq] {
			continue
		}
		return MemoryIntegrityError{Key: s.Key, Sequence: seq, Reason: "既未被摘要覆盖、也不在保留项中（静默丢失）"}
	}
	// 保留项必须是 Covered 的**紧邻后继**：中间出现空档说明有项在转换中被吞掉。
	if len(s.Items) > 0 && !s.Covered.Empty() && s.Items[0].Sequence != s.Covered.To+1 {
		return MemoryIntegrityError{
			Key:      s.Key,
			Sequence: s.Items[0].Sequence,
			Reason:   fmt.Sprintf("保留项首条 seq=%d 与摘要覆盖上界 %d 之间有空档", s.Items[0].Sequence, s.Covered.To),
		}
	}
	return nil
}
