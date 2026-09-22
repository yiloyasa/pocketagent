package runtime

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ---- 命名空间 ----------------------------------------------------------------

// 缓存命名空间的末段名（规格 §6.4 的布局）。
// 完整命名空间形如 agent/<agent_id>/<part>。
const (
	NsModels          = "models"
	NsCapabilities    = "capabilities"
	NsAccount         = "account"
	NsMCP             = "mcp"
	NsSessionIndex    = "session-index"
	NsTranscriptIndex = "transcript-index"
	NsArtifactIndex   = "artifact-index"
	NsProjection      = "projection"
)

// Namespace 拼出规格 §6.4 的命名空间：agent/<agentID>/<part>。
//
// 【为什么强制命名空间】规格要求缓存按 agent / workspace / thread 隔离。
// 没有命名空间的缓存必然在"换 agent 后读到上一家的模型列表"这类问题上翻车，
// 而且失效时无法精确 Invalidate —— 只能全清，把无关 agent 的热缓存一起打掉。
func Namespace(agentID, part string) string {
	return "agent/" + agentID + "/" + part
}

var (
	// ErrCacheKeyIncomplete 表示 CacheKey 缺命名空间或键。
	ErrCacheKeyIncomplete = errors.New("runtime: cache key 不完整（需要 namespace 与 key）")
)

// CacheKey 定位一个可再生缓存条目。
type CacheKey struct {
	Namespace string `json:"namespace"`
	Key       string `json:"key"`
}

// Validate 检查 key 是否可定位。
func (k CacheKey) Validate() error {
	if k.Namespace == "" || k.Key == "" {
		return ErrCacheKeyIncomplete
	}
	return nil
}

// String 给日志与测试断言用。
func (k CacheKey) String() string { return k.Namespace + "/" + k.Key }

// CacheValue 一条缓存原文。本层不解释内容。
type CacheValue struct {
	Payload []byte    `json:"payload"`
	At      time.Time `json:"at"`
	// Expires 非零时优先于 CachePolicy.TTL（用于上游自带过期时间的场景）。
	Expires time.Time `json:"expires,omitempty"`
}

// CacheEntry 是 Peek 的返回：值 + 新鲜度 + 该条目的降级许可。
//
// 【为什么把 StaleAllowed 放在条目上】读取策略必须能只靠"拿到的东西"决策，
// 否则调用方还得再回查 policy，两处状态一漂移就会把降级路径写错。
type CacheEntry struct {
	Value        CacheValue `json:"value"`
	Fresh        bool       `json:"fresh"`
	StaleAllowed bool       `json:"stale_allowed"`
}

// EvictionPolicy 是容量到达上限时的淘汰策略。
type EvictionPolicy string

const (
	// EvictLRU 淘汰最久未使用（默认）。
	EvictLRU EvictionPolicy = "lru"
	// EvictFIFO 淘汰最早写入。
	EvictFIFO EvictionPolicy = "fifo"
	// EvictNone 不淘汰；到达上限后拒绝写入（宁可让上游多打一次，也不悄悄丢）。
	EvictNone EvictionPolicy = "none"
)

// CachePolicy 是缓存条目的留存与淘汰策略（规格 §6.4）。
type CachePolicy struct {
	TTL           time.Duration  `json:"ttl"`
	MaxEntries    int            `json:"max_entries"`
	MaxBytes      int64          `json:"max_bytes"`
	StaleAllowed  bool           `json:"stale_allowed"`
	PersistToDisk bool           `json:"persist_to_disk"`
	Eviction      EvictionPolicy `json:"eviction"`
}

// CacheStats 是一个命名空间的命中统计。
//
// 【为什么必须统计】规格 §6.4 要求缓存"有命中统计"。没有命中率就无法判断
// 某个 TTL 是拍得太长（数据陈旧）还是太短（等于没缓存）——初始 TTL 全是猜的，
// 只有统计能让它变成实测校准过的值。
type CacheStats struct {
	Namespace string `json:"namespace"`
	Entries   int    `json:"entries"`
	Bytes     int64  `json:"bytes"`
	Hits      int64  `json:"hits"`
	Misses    int64  `json:"misses"`
	Evictions int64  `json:"evictions"`
}

// ReadDecision 是"读缓存该往哪走"的判定结果（规格 §6.4 的四条读取规则）。
type ReadDecision string

const (
	// DecideFresh 新鲜命中 → 直接返回。
	DecideFresh ReadDecision = "fresh"
	// DecideStaleUsable 过期但允许降级 → 先返回旧值，后台刷新。
	DecideStaleUsable ReadDecision = "stale-usable"
	// DecideMiss 无缓存或不许降级 → 请求上游。
	DecideMiss ReadDecision = "miss"
)

// DecideRead 把规格 §6.4 的读取规则固化成纯函数。
//
// 【为什么抽出来】这是四条规则里最容易写错的岔路：把"过期但可用"误判成 miss，
// 会在上游抖动时把整页做成红字 —— 与 capabilities_cache.go 注释里记的那次线上
// 根因同源。抽成纯函数后可以穷举断言，provider 侧只需调用，不必各写一遍。
func DecideRead(entry CacheEntry, hit bool) ReadDecision {
	if !hit {
		return DecideMiss
	}
	if entry.Fresh {
		return DecideFresh
	}
	if entry.StaleAllowed {
		return DecideStaleUsable
	}
	return DecideMiss
}

// CacheManager 是公共缓存入口（规格 §6.2）。
//
// 【实现者须知】缓存是**可再生**的：淘汰掉只意味着下次要再拉一次。
// 因此本接口的任何实现都不得把语义记忆（MemoryManager 的内容）当缓存淘汰
// —— 两者是不同的资源类别，见 MemoryKind 的注释。
type CacheManager interface {
	// Get 命中原样返回 (value, true, nil)；未命中或已过期且不许降级时 (zero, false, nil)。
	Get(ctx context.Context, key CacheKey) (CacheValue, bool, error)
	// Peek 返回条目与新鲜度，不做任何刷新 —— 供"先返回旧值、后台刷新"路径使用。
	Peek(ctx context.Context, key CacheKey) (CacheEntry, bool, error)
	// Put 写入并应用策略（TTL / 容量 / 淘汰）。
	Put(ctx context.Context, key CacheKey, value CacheValue, policy CachePolicy) error
	// Invalidate 清空一个命名空间（前缀匹配）。
	Invalidate(ctx context.Context, namespace string) error
	// Stats 返回命名空间统计（namespace 为空 = 全部）。
	Stats(ctx context.Context, namespace string) CacheStats
}

// CacheKeyString 是给错误信息用的安全格式化（避免 key 为空时输出空串难定位）。
func CacheKeyString(k CacheKey) string {
	if k.Key == "" && k.Namespace == "" {
		return "<empty>"
	}
	return fmt.Sprintf("%s/%s", k.Namespace, k.Key)
}
