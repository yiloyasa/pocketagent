// pi_models_probe_test.go —— probePiModels 的三态折叠语义(2026-10-07,pi 1.0.2
// 对齐专项):真表命中 / 超时回退 / 解析失败不回退。传输层(就绪重试)语义归
// piapp 的 client_test(newDelayedFakePi 端到端),本文件用窄接口桩钉 agent 侧
// 折叠与字段映射 —— 不拉真子进程(与 pi_selection_test 的 selectionFake 同模式)。
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// modelsProbeStub 是 piModelsProbeCaller 的记录型桩:返回预置应答/错误,并记下
// 调用形状(方法名/单次预算)供断言。
type modelsProbeStub struct {
	raw    json.RawMessage
	err    error
	calls  int
	typ    string
	budget time.Duration
}

func (s *modelsProbeStub) CallWithReadyRetry(_ context.Context, typ string, _ map[string]any, attemptBudget time.Duration) (json.RawMessage, error) {
	s.calls++
	s.typ, s.budget = typ, attemptBudget
	if s.err != nil {
		return nil, s.err
	}
	return s.raw, nil
}

// piProbeRealTable 对齐 1.0.2 get_available_models 实证形状(bundle:
// getAvailableSnapshot() → {models:[…]}),字段名与 probePiModels 的解析结构
// 逐一对应;第二条特意空 name,钉 DisplayName 的 id 兜底。
const piProbeRealTable = `{"models":[
	{"id":"glm-5.3","name":"GLM-5.3","provider":"zai-coding-cn","reasoning":true,
	 "input":["text"],"contextWindow":1000000,
	 "thinkingLevelMap":{"off":null,"minimal":null,"medium":null,"low":"low","high":"high","max":"max"}},
	{"id":"deepseek-flash","name":"","provider":"deepseek","reasoning":false,
	 "input":["text","image"],"contextWindow":128000}
]}`

// 真表命中:字段映射逐项钉死(Id=provider/id 无 pi/ 前缀、Provider 恒 "pi"、
// 空名兜底 id、思考档位过滤、图片模态),且必须发的是 get_available_models、
// 带分相单次预算(piModelsProbeAttempt)。
func TestProbePiModelsParsesRealTable(t *testing.T) {
	stub := &modelsProbeStub{raw: json.RawMessage(piProbeRealTable)}
	out := probePiModels(stub, func() []pb.ModelInfo {
		t.Fatal("真表命中不得走 models-store 回退")
		return nil
	})
	if stub.calls != 1 || stub.typ != "get_available_models" || stub.budget != piModelsProbeAttempt {
		t.Fatalf("调用形状不符: calls=%d typ=%s budget=%s", stub.calls, stub.typ, stub.budget)
	}
	if len(out) != 2 {
		t.Fatalf("应解析出 2 个模型, 实得 %d", len(out))
	}
	m0 := &out[0]
	if m0.Id != "zai-coding-cn/glm-5.3" || m0.Provider != "pi" || m0.DisplayName != "GLM-5.3" ||
		m0.ContextWindow != 1000000 || !m0.SupportsThinking || m0.SupportsImages || !m0.Available {
		t.Fatalf("glm-5.3 字段映射不符: %+v", m0)
	}
	// 思考档位按模型表过滤(实测 glm-5.3 只有 low/high/max;off/minimal 为 null 关)
	want := []string{"low", "high", "max"}
	if len(m0.AvailableEfforts) != len(want) {
		t.Fatalf("档位不符: %v", m0.AvailableEfforts)
	}
	for i, w := range want {
		if m0.AvailableEfforts[i] != w {
			t.Fatalf("档位[%d]=%s, 期望 %s", i, m0.AvailableEfforts[i], w)
		}
	}
	m1 := &out[1]
	if m1.Id != "deepseek/deepseek-flash" || m1.DisplayName != "deepseek-flash" ||
		m1.SupportsThinking || !m1.SupportsImages {
		t.Fatalf("deepseek-flash 字段映射不符(空名兜底 id/无思考/带图片): %+v", m1)
	}
	if got := m1.AvailableEfforts; len(got) != 1 || got[0] != "off" {
		t.Fatalf("非 reasoning 模型档位应为 [off]: %v", got)
	}
}

// 超时回退:重试预算耗尽仍未就绪(1.0.2 门控下最坏情形)必须落 models-store
// 回退(不劣化),候选 Provider 由桶键盖章 → ModelID 合法(宁缺毋假出口已过滤)。
func TestProbePiModelsFallsBackAfterTimeout(t *testing.T) {
	stub := &modelsProbeStub{err: errors.New("get_available_models: 等待 pi 应答超时(12s)")}
	fb := piModelsStoreFallbackFrom(writePiModelsStore(t, pi0991StoreShape))
	out := probePiModels(stub, func() []pb.ModelInfo { return fb })
	if len(out) == 0 {
		t.Fatal("超时必须落 models-store 回退,不能空手而归")
	}
	for i := range out {
		if out[i].Provider != "pi" || !strings.Contains(out[i].Id, "/") {
			t.Fatalf("回退候选 ModelID 非法: provider=%q id=%q", out[i].Provider, out[i].Id)
		}
	}
}

// 解析失败不回退:success 应答但形状漂移 → nil 且不走 models-store(既有分支
// 语义,宁缺毋假 —— 防止形状漂移时静默换源造成的口径混乱)。
func TestProbePiModelsBadPayloadNoFallback(t *testing.T) {
	stub := &modelsProbeStub{raw: json.RawMessage(`{"models":"shape-drift"}`)}
	out := probePiModels(stub, func() []pb.ModelInfo {
		t.Fatal("解析失败不得走 models-store 回退(既有分支语义)")
		return nil
	})
	if out != nil {
		t.Fatalf("解析失败应返回 nil, 实得 %+v", out)
	}
}
