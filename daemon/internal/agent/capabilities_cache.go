// capabilities_cache.go —— 只读查询的三类 TTL 缓存(用量/能力/MCP)。
//
// 【为什么要缓存 —— 根因与处方】"设置→技能与插件整页红字(暂时读不到能力列表)"
// 的根因:capabilities 要连打 skills/list + hooks/list + plugin/installed 三个
// 只读查询,codex 子进程繁忙时经常超过 10s 统一预算 → context deadline exceeded。
// 处方是两层:
//  1. TTL 缓存:命中直接返回(codex 一次都不打);三类查询的新鲜度要求不同,
//     各配各的 TTL(见下方三个常量);
//  2. 分级预算:重查询重拉时放宽到 heavyQueryBudget(45s)—— 缓存 miss 只发生在
//     冷启动/过期瞬间,慢一次可接受,之后 TTL 内秒回。
//
// 【过期兜底】重拉失败时,若手里还有旧值(**哪怕已过期**)就返回旧值并打日志,
// 比"整页红字"的体验好得多;只有完全无缓存才把错误透传给上层。
package agent

import (
	"log"
	"sync"
	"time"
)

// 三类查询各自的 TTL。差异化取值的理由:
//   - capabilities(skills/hooks/plugins):磁盘上的静态能力,30 分钟内的变化
//     (装/卸一个技能)对"为什么这个技能不生效"的回答几乎无影响;
//   - mcpServers:MCP 状态随子进程启动而稳定,5 分钟足够新;
//   - usage(用量/限流):用户在发任务前看"还有没有额度",必须新鲜,60 秒。
const (
	capabilitiesTTL = 30 * time.Minute
	mcpServersTTL   = 5 * time.Minute
	usageTTL        = 60 * time.Second
)

// heavyQueryBudget 是重查询(plugin/installed、mcpServerStatus/list 这类)在
// **缓存重拉**时的预算。正常路径仍走 withStatusClient 的 10s 默认预算。
const heavyQueryBudget = 45 * time.Second

// statusCache 是单槽 TTL 缓存,key 用于附加维度(如 capabilities 的 cwd ——
// 技能列表按目录算,cwd 变了必须重拉;usage/mcp 恒传空串)。
//
// 【并发纪律】mu 保护槽内状态;fetchMu 串行化重拉,防止并发 miss 时对 codex
// 打出双份请求(第二个读者会在拿到 fetchMu 后先复查新鲜度,通常直接命中)。
// fetch 在锁外执行 —— 重拉可能长达 45s,绝不能持 mu 等它。
type statusCache[T any] struct {
	ttl   time.Duration
	fetch func(key string) (T, error)
	// now 可注入时钟(单测用);nil = time.Now。
	now func() time.Time

	mu      sync.Mutex
	fetchMu sync.Mutex
	key     string
	val     T
	ok      bool
	at      time.Time
}

func (c *statusCache[T]) clock() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}

// fresh 返回未过期且 key 匹配的缓存值。
func (c *statusCache[T]) fresh(key string) (T, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ok && c.key == key && c.clock().Sub(c.at) < c.ttl {
		return c.val, true
	}
	return c.val, false
}

// stale 返回手里的旧值(**哪怕已过期**);没有任何缓存时 ok=false。
func (c *statusCache[T]) stale() (T, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.val, c.ok
}

// put 记录一次成功拉取。
func (c *statusCache[T]) put(key string, v T) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.key, c.val, c.ok, c.at = key, v, true, c.clock()
}

// read 是缓存的唯一读路径:命中直接返回;过期/未命中重拉(锁外);重拉失败
// 回退旧值(哪怕过期)并打日志;完全无缓存才透传错误(此时仍返回本次拉到的
// 部分结果 v,交由调用方决定如何降级 —— 例如 Usage 会把错误写进 .Error 字段)。
func (c *statusCache[T]) read(key, name string) (T, error) {
	if c.fetch == nil {
		// 零值缓存(如测试直接构造 &CodexProvider{}):无 fetch 可调,
		// 如实报错而不是 panic。
		var zero T
		return zero, errNoClient
	}
	if v, ok := c.fresh(key); ok {
		return v, nil
	}

	// 重拉:fetchMu 串行化同类查询;拿到锁后先复查(可能已被前一个读者刷新)。
	c.fetchMu.Lock()
	defer c.fetchMu.Unlock()
	if v, ok := c.fresh(key); ok {
		return v, nil
	}
	v, err := c.fetch(key)
	if err == nil {
		c.put(key, v)
		return v, nil
	}
	// 重拉失败:有旧值(哪怕过期)→ 返回旧值,绝不把"暂时读不到"变成整页红字。
	if old, ok := c.stale(); ok {
		log.Printf("[cache] %s 重拉失败,返回过期缓存(下次再试): %v", name, err)
		return old, nil
	}
	// 完全无缓存:透传错误 + 本次的部分结果(调用方按各自语义降级)。
	return v, err
}

// clear 清空缓存(下次 read 必重拉)。
func (c *statusCache[T]) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ok = false
	var zero T
	c.val = zero
}

// InvalidateAll 清空三类只读查询的缓存。
//
// 调用点清单(server 层,2026-09-30 起):
//   - SetCodexSettings 成功变更后(model.go):设置页立刻看到新值,而不是等 TTL;
//   - setAgentNativeConfigOption 写成功后(model.go):原生配置项是"本地下层事实",
//     写完即失效 —— 该写路径同时按 agent.ModelCatalogRefresher 清 pi/dsh 的模型目录缓存。
//
// 以后若有其它"本地下层事实变化"的入口(如技能装卸),也应调用它。
func (p *CodexProvider) InvalidateAll() {
	p.capCache.clear()
	p.mcpCache.clear()
	p.usageCache.clear()
}
