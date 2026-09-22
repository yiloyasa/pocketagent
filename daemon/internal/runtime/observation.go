package runtime

import (
	"os"
	"path/filepath"
	"strings"
)

// 本文件是「按 agent 聚合的运行时状态观测」（规格 §9.5：每家都能显示
// memory / cache / resource / checkpoint / recovery 状态）。
//
// 【为什么需要它】四个入口（memory/cache/checkpoint/resource）此前只有 resource
// 有可观测视图（Usage(scope)），其余三项只存在于 Backend 的内存 map 里 ——
// 诊断页与 RPC 都问不出"codex 有多少记忆、几份 checkpoint"。而没有这个视图，
// 「其余家接入统一 Runtime」这件事验不了：接入前是 0、接入后还是 0，看不出差别。
//
// 【聚合口径一律复用既有的 key 构造函数】memory key 用 MemoryKey.String()、
// cache namespace 用 Namespace()、checkpoint 用 CheckpointKey + 同一套文件名
// 映射函数。各处自己拼字符串必然拼出第二套口径，然后"显示 0 条"却查不出原因。

// MemoryObservation 是一个 agent 名下的语义记忆视图。
type MemoryObservation struct {
	Threads int                `json:"threads"` // 涉及多少个 thread
	Items   int                `json:"items"`   // 条目总数
	Bytes   int64              `json:"bytes"`   // 载荷字节数
	Kinds   map[MemoryKind]int `json:"kinds,omitempty"`
}

// CacheObservation 是一个 agent 名下的可再生缓存视图（按命名空间前缀聚合）。
type CacheObservation struct {
	Namespaces int   `json:"namespaces"`
	Entries    int   `json:"entries"`
	Bytes      int64 `json:"bytes"`
	Hits       int64 `json:"hits"`
	Misses     int64 `json:"misses"`
	Evictions  int64 `json:"evictions"`
}

// CheckpointObservation 是一个 agent 名下的 checkpoint 视图。
type CheckpointObservation struct {
	Count int   `json:"count"`
	Bytes int64 `json:"bytes"`
}

// AgentObservation 是一个 agent 的运行时状态快照。
//
// 【Recovery 为什么在结构里】规格 §9.5 要求显示它；但目前**没有产出方**
// （RecoveryState 挂在 per-thread 的临时 AgentRuntime 上，而恢复是跨重启的事件）。
// 所以这里如实返回「未上报」，而不是留空让人以为"没这一项"。
type AgentObservation struct {
	AgentID    string                `json:"agent_id"`
	Memory     MemoryObservation     `json:"memory"`
	Cache      CacheObservation      `json:"cache"`
	Checkpoint CheckpointObservation `json:"checkpoint"`
	Recovery   RecoveryState         `json:"recovery"`
}

// Observe 聚合某个 agent 名下的运行时状态。
//
// 【为什么按 agent 隔离是底线】它把"codex 有多少记忆"摆给用户看；一旦口径串家，
// 用户会看到"codex 有 3 条记忆"而那 3 条其实是 echo 的 —— 比不显示更糟。
func (b *Backend) Observe(agentID string) AgentObservation {
	obs := AgentObservation{AgentID: agentID, Memory: MemoryObservation{Kinds: map[MemoryKind]int{}}}
	if b == nil || agentID == "" {
		return obs
	}
	b.mu.Lock()
	obs.Memory = b.observeMemoryLocked(agentID)
	obs.Cache = b.observeCacheLocked(agentID)
	if b.dataDir == "" {
		obs.Checkpoint = b.observeDurableCheckpointsLocked(agentID)
	}
	b.mu.Unlock()
	// 文件后端的 checkpoint 落在磁盘上、不在内存态里：必须走目录扫描，
	// 否则"接了文件后端之后诊断页反而显示 0"，比不显示更误导。
	// 扫描在锁外做：它只读目录，不碰 Backend 状态。
	if b.dataDir != "" {
		obs.Checkpoint = observeFileCheckpoints(b.dataDir, agentID)
	}
	// recovery：目前没有产出方 —— 如实报"未上报"，不凭空造一个"恢复成功"。
	obs.Recovery = RecoveryState{From: RecoveredNone}
	return obs
}

// observeMemoryLocked 按 MemoryKey.String() 的前缀聚合（key 形如 agent/thread/kind[#scope]）。
func (b *Backend) observeMemoryLocked(agentID string) MemoryObservation {
	out := MemoryObservation{Kinds: map[MemoryKind]int{}}
	prefix := agentID + "/"
	threads := map[string]bool{}
	for k, items := range b.items {
		if !strings.HasPrefix(k, prefix) {
			continue
		}
		parts := strings.SplitN(k[len(prefix):], "/", 2)
		if len(parts) != 2 {
			// 形状意外的 key：跳过而不是猜它属于谁 —— 猜错会把计数算到别的
			// thread/kind 上，而那是"看着有数字、其实是错的"。
			continue
		}
		threads[parts[0]] = true
		kind := parts[1]
		if i := strings.IndexByte(kind, '#'); i >= 0 {
			kind = kind[:i] // 去掉可选 Scope
		}
		out.Kinds[MemoryKind(kind)] += len(items)
		out.Items += len(items)
		for i := range items {
			out.Bytes += int64(len(items[i].Payload))
		}
	}
	out.Threads = len(threads)
	return out
}

// observeCacheLocked 按 Namespace(agentID, "") 前缀聚合（形如 agent/<id>/）。
func (b *Backend) observeCacheLocked(agentID string) CacheObservation {
	var out CacheObservation
	prefix := Namespace(agentID, "")
	for ns := range b.stats {
		if !strings.HasPrefix(ns, prefix) {
			continue
		}
		b.recomputeCacheLocked(ns)
		entries, bytes := b.cacheUsageLocked(ns)
		st := b.cacheStat(ns)
		out.Namespaces++
		out.Entries += entries
		out.Bytes += bytes
		out.Hits += st.Hits
		out.Misses += st.Misses
		out.Evictions += st.Evictions
	}
	return out
}

// observeDurableCheckpointsLocked 数内存后端的 checkpoint（key 形如 agent/thread/name）。
func (b *Backend) observeDurableCheckpointsLocked(agentID string) CheckpointObservation {
	var out CheckpointObservation
	prefix := agentID + "/"
	for k, payload := range b.durable {
		if !strings.HasPrefix(k, prefix) {
			continue
		}
		out.Count++
		out.Bytes += int64(len(payload))
	}
	return out
}

// observeFileCheckpoints 数文件后端的 checkpoint。
//
// 【口径必须与写侧一致】文件名由 checkpointFileComponent(agentID) 生成（截断 +
// 非法字符替换），所以这里的匹配也走同一个函数 —— 自己另写一套匹配规则会在
// agentID 含特殊字符或超长时静默数不到。
func observeFileCheckpoints(dataDir, agentID string) CheckpointObservation {
	var out CheckpointObservation
	entries, err := os.ReadDir(filepath.Join(dataDir, "checkpoints"))
	if err != nil {
		// 目录不存在 = 这个后端还没写过 checkpoint，不是错误。
		return out
	}
	prefix := checkpointFileComponent(agentID) + "."
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), prefix) || !strings.HasSuffix(e.Name(), ".ckpt") {
			continue
		}
		info, ierr := e.Info()
		if ierr != nil {
			continue
		}
		out.Count++
		out.Bytes += info.Size()
	}
	return out
}
