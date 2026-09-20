// model_efforts_test.go —— ListModels 填充 ModelInfo.available_efforts 的回归。
//
// 【为什么必须钉死】思考档位的动态透出是 ModelEffortCard 的数据源:provider
// Models() 报什么档位就透什么、没报就留空(= 手机端默认档);同时 ListModels
// 不得再改写 provider 返回的缓存切片(双前缀/数据竞争,见 appendProviderModels
// 复制注释),也不得为模型元数据去依赖任何活线程。
// 【T-24 清理】dynamicEffortsFor(T-14 重写 ListModels 后零调用方)已删除;
// 原本只测它的两个探针用例(TestDynamicEffortsProbePicksNewestPiThread /
// TestDynamicEffortsProbeWithoutThreads)随之删除 —— 探针选择行为在生产中
// 已不存在。
// 【T-26/T-28 清理】server 侧 ThinkingLevels 接口与 agent.PiProvider.ThinkingLevels
// 也已删除(pi 档位由 provider 在 Models() 里自填 available_efforts,运行期校验
// 走 agent/pi_selection.go 直发 get_available_thinking_levels RPC);effortsStub
// 上死掉的 ThinkingLevels 桩方法与只为它存在的 levels/seenThread 字段随之删除
// ——"ListModels 不得依赖活线程"改由编译期保证:Provider 接口上已无任何
// 收线程的方法。
package server

import (
	"context"
	"testing"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// effortsStub 带 Models 的 provider 桩(生产对应 *agent.PiProvider)。
type effortsStub struct {
	PiProviderStub
	models []pb.ModelInfo
}

func (s *effortsStub) Models() []pb.ModelInfo { return s.models }

func TestListModelsFillsAvailableEfforts(t *testing.T) {
	st := mustStore(t)
	th := piExportThread("th-eff-1")
	if err := st.Create(th); err != nil {
		t.Fatalf("建线程失败: %v", err)
	}
	pi := &effortsStub{
		models: []pb.ModelInfo{{Id: "zai-coding-cn/glm-5.3", Provider: "pi", DisplayName: "GLM-5.3", AvailableEfforts: []string{"low", "high", "max"}}},
	}
	srv := New(Config{Store: st, Providers: map[string]agent.Provider{"pi": pi}})
	res, err := srv.ListModels(context.Background(), &pb.ListModelsRequest{})
	if err != nil {
		t.Fatalf("ListModels 失败: %v", err)
	}
	if len(res.Models) != 1 {
		t.Fatalf("应返回 1 个模型,实际 %d", len(res.Models))
	}
	m := res.Models[0]
	if m.Id != "pi/zai-coding-cn/glm-5.3" {
		t.Fatalf("模型 id 应带 provider 前缀,实际 %q", m.Id)
	}
	if len(m.AvailableEfforts) != 3 || m.AvailableEfforts[0] != "low" || m.AvailableEfforts[2] != "max" {
		t.Fatalf("应透出 pi 的动态档位,实际 %v", m.AvailableEfforts)
	}
}

// 无动态档位能力的 provider(codex/zcode/…)留空:手机端据此回内置默认档。
func TestListModelsEmptyEffortsWithoutCapability(t *testing.T) {
	st := mustStore(t)
	srv := New(Config{Store: st, Providers: map[string]agent.Provider{
		"pi": &PiProviderStub{}, // 无 ThinkingLevels 能力
	}})
	res, err := srv.ListModels(context.Background(), &pb.ListModelsRequest{})
	if err != nil {
		t.Fatalf("ListModels 失败: %v", err)
	}
	for _, m := range res.Models {
		if len(m.AvailableEfforts) != 0 {
			t.Fatalf("无能力 provider 的模型不应带档位: %s -> %v", m.Id, m.AvailableEfforts)
		}
	}
}

// 【关键回归】ListModels 不得改写 provider 返回的缓存切片:
// 两次调用都不能把 id 叠成 "pi/pi/…",也不得把档位写回 provider 的模型缓存。
func TestListModelsDoesNotMutateProviderCache(t *testing.T) {
	st := mustStore(t)
	pi := &effortsStub{
		models: []pb.ModelInfo{{Id: "default", Provider: "pi"}},
	}
	srv := New(Config{Store: st, Providers: map[string]agent.Provider{"pi": pi}})
	for round := 0; round < 2; round++ {
		res, err := srv.ListModels(context.Background(), &pb.ListModelsRequest{})
		if err != nil {
			t.Fatalf("第 %d 次 ListModels 失败: %v", round+1, err)
		}
		if res.Models[0].Id != "pi/default" {
			t.Fatalf("第 %d 次应得到单前缀 id,实际 %q", round+1, res.Models[0].Id)
		}
	}
	// provider 侧的缓存模型保持原样
	if pi.models[0].Id != "default" {
		t.Fatalf("provider 缓存被改写: %q", pi.models[0].Id)
	}
	if len(pi.models[0].AvailableEfforts) != 0 {
		t.Fatalf("档位不应写回 provider 缓存: %v", pi.models[0].AvailableEfforts)
	}
}
