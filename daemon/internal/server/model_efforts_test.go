// model_efforts_test.go —— ListModels 填充 ModelInfo.available_efforts 的回归。
//
// 【为什么必须钉死】思考档位的动态透出是 ModelEffortCard 的数据源:pi 填动态
// 档位、无能力 provider 留空(= 手机端默认档);同时 ListModels 不得再改写
// provider 返回的缓存切片(双前缀/数据竞争,见 dynamicEffortsFor 与复制注释)。
package server

import (
	"context"
	"testing"
	"time"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// effortsStub 带 Models + 动态档位的 provider 桩(生产对应 *agent.PiProvider)。
type effortsStub struct {
	PiProviderStub
	models     []pb.ModelInfo
	levels     []string
	seenThread *store.Thread // 记录 ThinkingLevels 被调时传入的探针线程
}

func (s *effortsStub) Models() []pb.ModelInfo { return s.models }

func (s *effortsStub) ThinkingLevels(t *store.Thread) []string {
	s.seenThread = t
	return s.levels
}

func TestListModelsFillsAvailableEfforts(t *testing.T) {
	st := mustStore(t)
	th := piExportThread("th-eff-1")
	if err := st.Create(th); err != nil {
		t.Fatalf("建线程失败: %v", err)
	}
	pi := &effortsStub{
		models: []pb.ModelInfo{{Id: "zai-coding-cn/glm-5.3", Provider: "pi", DisplayName: "GLM-5.3", AvailableEfforts: []string{"low", "high", "max"}}},
		levels: []string{"low", "high", "max"},
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
	if pi.seenThread != nil {
		t.Fatal("model metadata must not depend on an unrelated live thread")
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

// 探针选"最近更新的该 agent 会话"(有活客户端才拿得到真实动态档位)。
func TestDynamicEffortsProbePicksNewestPiThread(t *testing.T) {
	st := mustStore(t)
	old := piExportThread("th-eff-old")
	neu := piExportThread("th-eff-new")
	for _, th := range []*store.Thread{old, neu} {
		if err := st.Create(th); err != nil {
			t.Fatalf("建线程失败: %v", err)
		}
	}
	// Get 返回独立快照。跨过旧线程的毫秒时间戳后真实提交，避免依赖
	// 未提交字段修改或时间戳并列时 map 的遍历顺序。
	cur, ok := st.Get("th-eff-new")
	if !ok {
		t.Fatal("新线程不存在")
	}
	time.Sleep(time.Until(time.UnixMilli(old.UpdatedAt + 2)))
	if err := st.Update(cur); err != nil {
		t.Fatalf("更新新线程失败: %v", err)
	}
	if cur.UpdatedAt <= old.UpdatedAt {
		t.Fatalf("更新时间未区分: old=%d new=%d", old.UpdatedAt, cur.UpdatedAt)
	}
	pi := &effortsStub{levels: []string{"off"}}
	srv := New(Config{Store: st, Providers: map[string]agent.Provider{"pi": pi}})
	if got := srv.dynamicEffortsFor(pi); len(got) != 1 || got[0] != "off" {
		t.Fatalf("应返回注入档位,实际 %v", got)
	}
	if pi.seenThread == nil || pi.seenThread.ID != "th-eff-new" {
		t.Fatalf("探针应是最近更新的 pi 线程,实际 %+v", pi.seenThread)
	}
}

// 没有任何该 agent 会话时也要能出兜底档位(探针用匿名线程,不 panic)。
func TestDynamicEffortsProbeWithoutThreads(t *testing.T) {
	st := mustStore(t)
	pi := &effortsStub{levels: []string{"low", "max"}}
	srv := New(Config{Store: st, Providers: map[string]agent.Provider{"pi": pi}})
	got := srv.dynamicEffortsFor(pi)
	if len(got) != 2 {
		t.Fatalf("应返回注入档位,实际 %v", got)
	}
}

// 【关键回归】ListModels 不得改写 provider 返回的缓存切片:
// 两次调用都不能把 id 叠成 "pi/pi/…",也不得把档位写回 provider 的模型缓存。
func TestListModelsDoesNotMutateProviderCache(t *testing.T) {
	st := mustStore(t)
	pi := &effortsStub{
		models: []pb.ModelInfo{{Id: "default", Provider: "pi"}},
		levels: []string{"high"},
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
