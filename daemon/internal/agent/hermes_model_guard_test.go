// hermes_model_guard_test.go —— 模型守卫契约(守卫口径对齐 codex:按 "/default"
// 后缀拦默认档,L4 实害钉子):
//   - 默认档不发 session/set_model:手机选"hermes/default(默认模型)"落库的是
//     带前缀完整 ID,旧守卫只比 "default" 拦不住 → 曾对默认档发出注定多余的
//     set 并误报"切模型失败";裸 "default"(历史哨兵形)也一并拦;
//   - 显式模型照发,并钉死 acpModel 换算:静态列表形 "hermes/deepseek-flash"
//     补 "deepseek:" 前缀;会话内显式选择的冒号形 "hermes/deepseek:deepseek-flash"
//     (R4-H4 实证 set_model 的 modelId 就是 provider:model 形)剥前缀后原样
//     透传,不得二次加前缀。
package agent

import (
	"testing"

	"github.com/pocketagent/daemon/internal/store"
)

// TestHermesModelGuardDefaultsSkipSet 默认档两形态都不得发 session/set_model。
func TestHermesModelGuardDefaultsSkipSet(t *testing.T) {
	cases := []struct {
		name    string
		modelID string
	}{
		{"带前缀完整ID(手机默认模型档)", "hermes/default"},
		{"裸default历史哨兵", "default"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := newFakeHermesClient(true)
			p := newHermesTestProvider(t, fake)
			if _, _, err := runHermes(t, p, &store.Thread{ID: "t-guard", ModelID: tc.modelID}, nil); err != nil {
				t.Fatalf("轮次应成功: %v", err)
			}
			fake.mu.Lock()
			defer fake.mu.Unlock()
			if fake.modelCalls != 0 {
				t.Fatalf("默认档 %q 不得发 session/set_model,实发 %d 次(实参 %+v)",
					tc.modelID, fake.modelCalls, fake.modelArgs)
			}
		})
	}
}

// TestHermesModelGuardExplicitSendsSet 显式模型仍发 set_model,且发在本轮会话上;
// acpModel 换算按形分钉死(无冒号补前缀 / 有冒号原样透传)。
func TestHermesModelGuardExplicitSendsSet(t *testing.T) {
	cases := []struct {
		name         string
		modelID      string
		wantACPModel string
	}{
		{"静态列表形补deepseek前缀", "hermes/deepseek-flash", "deepseek:deepseek-flash"},
		{"会话内冒号形原样透传(R4-H4)", "hermes/deepseek:deepseek-flash", "deepseek:deepseek-flash"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := newFakeHermesClient(true)
			p := newHermesTestProvider(t, fake)
			if _, _, err := runHermes(t, p, &store.Thread{ID: "t-guard", ModelID: tc.modelID}, nil); err != nil {
				t.Fatalf("轮次应成功: %v", err)
			}
			fake.mu.Lock()
			defer fake.mu.Unlock()
			if fake.modelCalls != 1 {
				t.Fatalf("显式模型 %q 应恰发一次 session/set_model,实发 %d 次", tc.modelID, fake.modelCalls)
			}
			call := fake.modelArgs[0]
			if call.modelID != tc.wantACPModel {
				t.Fatalf("acpModel 换算不符:modelID=%q got %q want %q",
					tc.modelID, call.modelID, tc.wantACPModel)
			}
			if call.sessionID != "fresh-sess" {
				t.Fatalf("set_model 应发在本轮会话上,got %q", call.sessionID)
			}
		})
	}
}
