// capabilities_cache_test.go —— 三类只读查询 TTL 缓存的回归测试。
//
// 【要钉死的四件事】(对应"暂时读不到能力列表"的修复)
//  1. 缓存命中:同 key 二次调用只打一次 codex(fetch 计数 = 1);
//  2. TTL 过期:过期后重拉,拿到新值;
//  3. 重拉失败回退:手里有旧值(哪怕过期)时返回旧值而不是报错;
//     完全无缓存时错误如实透传;
//  4. InvalidateAll:清空后下一次读取必然重拉。
//
// 另加 provider 接线测试:CapabilitiesDetailed / Usage / McpStatusDetailed
// 确实走各自的缓存(防止以后有人绕过缓存直连 codex)。
package agent

import (
	"errors"
	"testing"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// fakeClock 是可手动推进的时钟(TTL 语义测试用)。
type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time { return c.t }
func (c *fakeClock) advance(d time.Duration) {
	if c.t.IsZero() {
		c.t = time.Now()
	}
	c.t = c.t.Add(d)
}

// TestStatusCacheHitCountsFetch 场景 1:TTL 内同 key 二次读取只 fetch 一次。
func TestStatusCacheHitCountsFetch(t *testing.T) {
	calls := 0
	ck := &fakeClock{}
	c := statusCache[string]{
		ttl: capabilitiesTTL,
		fetch: func(string) (string, error) {
			calls++
			return "v1", nil
		},
		now: ck.now,
	}
	v1, err := c.read("cwd-a", "capabilities")
	if err != nil || v1 != "v1" {
		t.Fatalf("首次读取: v=%q err=%v", v1, err)
	}
	v2, err := c.read("cwd-a", "capabilities")
	if err != nil || v2 != "v1" {
		t.Fatalf("二次读取: v=%q err=%v", v2, err)
	}
	if calls != 1 {
		t.Fatalf("缓存命中但 fetch 被调了 %d 次(应为 1)", calls)
	}
	// key 变化(cwd 不同)必须重拉:技能列表按目录算。
	if _, err := c.read("cwd-b", "capabilities"); err != nil {
		t.Fatalf("换 cwd 读取失败: %v", err)
	}
	if calls != 2 {
		t.Fatalf("cwd 变化后应重拉: calls=%d", calls)
	}
}

// TestStatusCacheTTLExpiryRefetch 场景 2:过期后重拉并更新缓存。
func TestStatusCacheTTLExpiryRefetch(t *testing.T) {
	calls := 0
	ck := &fakeClock{}
	ck.advance(0)
	c := statusCache[string]{
		ttl: usageTTL,
		fetch: func(string) (string, error) {
			calls++
			return "v" + string(rune('0'+calls)), nil
		},
		now: ck.now,
	}
	if v, _ := c.read("", "usage"); v != "v1" {
		t.Fatalf("首次读取应得 v1, got=%q", v)
	}
	// TTL 内:仍命中。
	if v, _ := c.read("", "usage"); v != "v1" {
		t.Fatalf("TTL 内不应重拉, got=%q", v)
	}
	ck.advance(usageTTL + time.Second)
	if v, _ := c.read("", "usage"); v != "v2" {
		t.Fatalf("过期后应重拉得 v2, got=%q", v)
	}
	if calls != 2 {
		t.Fatalf("过期重拉 fetch 应被调 2 次, got=%d", calls)
	}
}

// TestStatusCacheStaleFallback 场景 3:重拉失败回退旧缓存;无缓存透传错误。
func TestStatusCacheStaleFallback(t *testing.T) {
	ck := &fakeClock{}
	ck.advance(0)
	fail := errors.New("context deadline exceeded")
	wantFail := true
	c := statusCache[string]{
		ttl: mcpServersTTL,
		fetch: func(string) (string, error) {
			if wantFail {
				return "", fail
			}
			return "fresh", nil
		},
		now: ck.now,
	}
	// 3a. 完全无缓存时失败:错误如实透传(设置页仍会红字,但这只发生在
	// 冷启动 + codex 不可达的组合,而不是每次进页面)。
	if _, err := c.read("", "mcpServers"); !errors.Is(err, fail) {
		t.Fatalf("无缓存时错误应透传, got=%v", err)
	}
	// 3b. 先成功一次,再把时钟推过 TTL 并让重拉失败:必须返回旧值且不报错。
	wantFail = false
	if _, err := c.read("", "mcpServers"); err != nil {
		t.Fatalf("预置成功值失败: %v", err)
	}
	ck.advance(mcpServersTTL + time.Second)
	wantFail = true
	v, err := c.read("", "mcpServers")
	if err != nil {
		t.Fatalf("重拉失败时应回退旧缓存而不是报错: %v", err)
	}
	if v != "fresh" {
		t.Fatalf("回退应返回旧值 fresh, got=%q", v)
	}
	// 3c. 回退不覆盖缓存:恢复拉取能力后拿到的是新值。
	wantFail = false
	if v, _ := c.read("", "mcpServers"); v != "fresh" {
		t.Fatalf("恢复后应重拉成功, got=%q", v)
	}
}

// TestStatusCacheInvalidateAll 场景 4:清空后下一次读取必然重拉。
func TestStatusCacheInvalidateAll(t *testing.T) {
	calls := 0
	ck := &fakeClock{}
	c := statusCache[string]{
		ttl: capabilitiesTTL,
		fetch: func(string) (string, error) {
			calls++
			return "ok", nil
		},
		now: ck.now,
	}
	_, _ = c.read("", "capabilities")
	c.clear()
	_, _ = c.read("", "capabilities")
	if calls != 2 {
		t.Fatalf("InvalidateAll 后应重拉: calls=%d", calls)
	}
}

// TestProviderCacheWiring 钉住三个对外方法确实走各自的缓存与 InvalidateAll。
func TestProviderCacheWiring(t *testing.T) {
	p := NewCodexProvider()

	// capabilities:两次调用一次 fetch;InvalidateAll 后再拉。
	capCalls := 0
	p.capCache.fetch = func(string) ([]*pb.CodexCapability, error) {
		capCalls++
		return []*pb.CodexCapability{{Kind: "skill", Name: "demo"}}, nil
	}
	if _, err := p.CapabilitiesDetailed("/w"); err != nil {
		t.Fatalf("CapabilitiesDetailed: %v", err)
	}
	if _, err := p.CapabilitiesDetailed("/w"); err != nil {
		t.Fatalf("CapabilitiesDetailed 二次: %v", err)
	}
	if capCalls != 1 {
		t.Fatalf("capabilities 未命中缓存: calls=%d", capCalls)
	}
	// usage / mcp 同理。
	usageCalls, mcpCalls := 0, 0
	p.usageCache.fetch = func(string) (*pb.CodexUsage, error) {
		usageCalls++
		return &pb.CodexUsage{PlanType: "pro"}, nil
	}
	p.mcpCache.fetch = func(string) ([]*pb.McpServerStatus, error) {
		mcpCalls++
		return []*pb.McpServerStatus{{Name: "s"}}, nil
	}
	if u := p.Usage(); u.GetPlanType() != "pro" {
		t.Fatalf("Usage 未走缓存返回值: %+v", u)
	}
	if u := p.Usage(); u.GetPlanType() != "pro" {
		t.Fatalf("Usage 二次: %+v", u)
	}
	if _, err := p.McpStatusDetailed(); err != nil {
		t.Fatalf("McpStatusDetailed: %v", err)
	}
	if _, err := p.McpStatusDetailed(); err != nil {
		t.Fatalf("McpStatusDetailed 二次: %v", err)
	}
	if usageCalls != 1 || mcpCalls != 1 {
		t.Fatalf("usage/mcp 未命中缓存: usage=%d mcp=%d", usageCalls, mcpCalls)
	}
	// InvalidateAll 清三类缓存:下一次读取各自重拉。
	p.InvalidateAll()
	_, _ = p.CapabilitiesDetailed("/w")
	_ = p.Usage()
	if capCalls != 2 || usageCalls != 2 {
		t.Fatalf("InvalidateAll 未生效: cap=%d usage=%d", capCalls, usageCalls)
	}
	if _, err := p.McpStatusDetailed(); err != nil {
		t.Fatalf("InvalidateAll 后 McpStatusDetailed: %v", err)
	}
	if mcpCalls != 2 {
		t.Fatalf("InvalidateAll 未清 mcp 缓存: calls=%d", mcpCalls)
	}
}

// TestProviderUsageStaleFallbackOnRefetchFailure 用量重拉失败时回退旧值,
// 且返回的旧值不带 error(否则手机会把过期值渲染成"暂时读不到")。
func TestProviderUsageStaleFallbackOnRefetchFailure(t *testing.T) {
	p := NewCodexProvider()
	good := &pb.CodexUsage{PlanType: "pro"}
	first := true
	p.usageCache.fetch = func(string) (*pb.CodexUsage, error) {
		if first {
			first = false
			return good, nil
		}
		// 模拟 codex 繁忙:fetchUsageUncached 会把错误写进 .Error 并走 error 通道。
		return &pb.CodexUsage{Error: "context deadline exceeded"}, errors.New("context deadline exceeded")
	}
	if u := p.Usage(); u != good {
		t.Fatalf("首次应返回成功值")
	}
	if u := p.Usage(); u != good || u.GetError() != "" {
		t.Fatalf("重拉失败应回退旧值且不带 error: %+v", u)
	}
}
