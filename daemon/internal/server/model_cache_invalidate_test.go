// model_cache_invalidate_test.go —— 原生配置写通道的失效纪律(2026-09-30):
// setAgentNativeConfigOption 写成功后必须当场失效"本地下层事实"缓存
// (InvalidateAll + ModelCatalogRefresher.InvalidateModels),与 SetCodexSettings
// 成功后的 InvalidateAll 同款,不等 TTL —— 否则手机端"随后重拉目录"拿到的
// 还是 TTL 内的旧 currentValue。
//
// 【放哪个包】写入口是 server 包的未导出方法,单测只能在 server 包内驱动
// (agent 包无法触达);按 native_config_options_rpc_test.go 的既有 mock 风格
// (嵌 seqProvider 的可编程假 provider)打桩,是最小成本的桩测。
package server

import (
	"context"
	"sync"
	"testing"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// fakeInvalidatingProvider 假 provider:实现 NativeConfigOptions 写通道 +
// 两个失效接口,用计数器钉住"写成功 → 失效被调"这条链。
type fakeInvalidatingProvider struct {
	seqProvider
	setErr error

	mu        sync.Mutex
	invAll    int // InvalidateAll 被调次数
	invModels int // InvalidateModels 被调次数
}

func (p *fakeInvalidatingProvider) ConfigOptionsCatalog(context.Context, *store.Thread) ([]agent.NativeConfigOption, error) {
	return nil, nil
}

func (p *fakeInvalidatingProvider) SetNativeConfigOption(context.Context, *store.Thread, string, string) error {
	return p.setErr
}

func (p *fakeInvalidatingProvider) InvalidateAll() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.invAll++
}

// InvalidateModels 使 fake 实现 agent.ModelCatalogRefresher(ListModels refresh=true 同款)。
func (p *fakeInvalidatingProvider) InvalidateModels() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.invModels++
}

func (p *fakeInvalidatingProvider) counters() (int, int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.invAll, p.invModels
}

// TestNativeConfigWriteInvalidatesCaches 写成功 → 两个失效接口各被调一次;
// 写失败(provider 拒绝)→ 不失效(失败没有改变任何下层事实)。
func TestNativeConfigWriteInvalidatesCaches(t *testing.T) {
	prov := &fakeInvalidatingProvider{}
	prov.id = "dsh"
	srv := New(Config{
		Store:     mustStore(t),
		Providers: map[string]agent.Provider{"dsh": prov},
	})
	req := &pb.SetCodexSettingsRequest{
		AgentId:              "dsh",
		NativeConfigOptionId: &[]string{"reasoning_effort"}[0],
		NativeConfigValue:    &[]string{"low"}[0],
	}

	// 基线:写成功路径。
	if _, err := srv.SetCodexSettings(context.Background(), req); err != nil {
		t.Fatalf("原生配置写入应成功: %v", err)
	}
	invAll, invModels := prov.counters()
	if invAll != 1 || invModels != 1 {
		t.Fatalf("写成功后应各失效一次: InvalidateAll=%d InvalidateModels=%d", invAll, invModels)
	}

	// 对照:provider 拒绝的写不上抛成功,也不清缓存(保持原值语义)。
	prov.setErr = context.DeadlineExceeded
	if _, err := srv.SetCodexSettings(context.Background(), req); err == nil {
		t.Fatal("provider 拒绝的写应上抛")
	}
	invAll, invModels = prov.counters()
	if invAll != 1 || invModels != 1 {
		t.Fatalf("写失败不应失效缓存: InvalidateAll=%d InvalidateModels=%d", invAll, invModels)
	}
}
