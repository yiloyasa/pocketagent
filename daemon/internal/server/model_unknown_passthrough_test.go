// model_unknown_passthrough_test.go —— AUDIT §3.3「mock 新模型 ID」回归资产
// (README §7:server/model.go ListModels **无模型白名单**,原生返回什么透传什么)。
//
// 【钉什么】上游 codex/pi/zcode 升级后冒出任何未登记 model id(带新能力位)时,
// appendProviderModels 必须原文透传:只加 provider 前缀,DisplayName/上下文窗口/
// 能力位逐字段复制;不因"名单里没有"过滤掉,也不替它编造显示名或能力。镜像原则
// 的模型列表半边 —— 用户在手机上看到的就是原生报来的原文。
//
// 【为什么值得钉】这条路径没有别的测试看内容:并行化测试(model_list_parallel)钉
// 时序、efforts 测试钉缓存只读,谁都不拦"有人在这里加一个白名单/默认值"的回归。
package server

import (
	"strings"
	"testing"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

func TestListModelsUnknownModelIDPassthroughVerbatim(t *testing.T) {
	// 原生冒出的"从没登记过"的模型:新 id、原生显示名、新思考档位、全能力位。
	// 【为什么用内联字面量而不是先造变量再放进切片】pb.ModelInfo 值拷贝会连
	// protoimpl.MessageState 里的锁一起复制(go vet copylocks 红线);期望字段
	// 在断言处用同值字面量对照,不共享同一个 proto 值。
	srv := New(Config{Store: mustStore(t), Providers: map[string]agent.Provider{
		// provider id 也故意取"未来"的名字:动态接入不认名字,只认注册表。
		"future-agent": &listableStub{plainProvider: plainProvider{id: "future-agent"},
			models: []pb.ModelInfo{{
				Id:               "brand-new-model-9",
				DisplayName:      "原生原文显示名(勿改写)",
				ContextWindow:    1234567,
				SupportsThinking: true,
				SupportsImages:   true,
				Available:        true,
				AvailableEfforts: []string{"turbo-new", "ultra"},
			}}},
	}})

	res, err := srv.ListModels(ctxBG(), &pb.ListModelsRequest{AgentId: "future-agent"})
	if err != nil {
		t.Fatalf("ListModels 失败: %v", err)
	}
	if len(res.Models) != 1 {
		t.Fatalf("未登记模型不得被名单过滤,应原样出现 1 条,得到 %d 条: %+v", len(res.Models), res.Models)
	}
	m := res.Models[0]
	if m.Id != "future-agent/brand-new-model-9" {
		t.Fatalf("应只加 provider 前缀,得到 %q", m.Id)
	}
	if m.DisplayName != "原生原文显示名(勿改写)" {
		t.Fatalf("显示名必须保留原生原文(不翻译不编造),得到 %q", m.DisplayName)
	}
	if m.ContextWindow != 1234567 || !m.SupportsThinking || !m.SupportsImages || !m.Available {
		t.Fatalf("能力位必须逐字段透传: %+v", m)
	}
	if strings.Join(m.AvailableEfforts, ",") != "turbo-new,ultra" {
		t.Fatalf("未登记的思考档位必须原样透传,得到 %v", m.AvailableEfforts)
	}
}
