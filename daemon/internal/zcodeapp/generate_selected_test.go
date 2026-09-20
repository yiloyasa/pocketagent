package zcodeapp

// generate_selected_test.go —— GenerateTextSelected(手机"提示词优化"指定模型
// 路径)的 T-02 回归钉子:档位校验按来源分级,绝不因快照自相矛盾整次失败。
//
// 病灶模型:deepseek-v4-flash 实测 default_level="max" 不在自声明 levels
// [low,medium,high,xhigh,none] 里,上游也拒收 max —— 旧代码硬校验直接报
// "不支持思考档位",优化功能在该模型上 100% 失败。语义与
// internal/agent/zcode.go applyModel 的 T-02 容错一致,不新增任何写死枚举。

import (
	"context"
	"strings"
	"testing"
)

// snapshotDeepseekShape T-02 病灶形状:defaultLevel ∉ 自声明 levels。
const snapshotDeepseekShape = `{"settings":{"model":{"available":[
	{"ref":{"providerId":"p-ds","modelId":"deepseek-v4-flash"},
	 "reasoning":{"defaultLevel":"max","levels":["low","medium","high","xhigh","none"]}}]}}}`

// snapshotConsistentLevels 一致元数据:defaultLevel 在自声明 levels 内(GLM 系形状)。
const snapshotConsistentLevels = `{"settings":{"model":{"available":[
	{"ref":{"providerId":"p-glm","modelId":"glm-x"},
	 "reasoning":{"defaultLevel":"high","levels":["low","medium","high"]}}]}}}`

// TestSelectedDeepseekShapeOmitsOptions 场景 1(修复对象):用户未选档位、模型
// 自带 defaultLevel ∉ 自声明 levels → 不得报错,selection 省略 options(协议
// 语义"不覆盖",沿用桌面端当前档位),生成照常完成。
func TestSelectedDeepseekShapeOmitsOptions(t *testing.T) {
	c, buf := newTestClient()
	type out struct {
		text string
		err  error
	}
	ch := make(chan out, 1)
	go func() {
		s, err := c.GenerateTextSelected(context.Background(), "deepseek-v4-flash", "", "x")
		ch <- out{s, err}
	}()

	create := waitFrame(t, buf, "session/create")
	replyOK(c, create, snapshotDeepseekShape)

	gen := waitFrame(t, buf, "workspace/generateText")
	sel, _ := gen["params"].(map[string]interface{})["selection"].(map[string]interface{})
	if sel["providerId"] != "p-ds" || sel["modelId"] != "deepseek-v4-flash" {
		t.Fatalf("应选中病灶模型本身, got %v", sel)
	}
	if _, has := sel["options"]; has {
		t.Fatalf("defaultLevel=max 不在自声明 levels 里必须省略 options(沿用桌面端当前档位), got %v", sel)
	}
	replyOK(c, gen, `{"text":"ok"}`)

	r := <-ch
	if r.err != nil {
		t.Fatalf("快照自相矛盾不得整次失败(T-02 病灶): %v", r.err)
	}
	if r.text != "ok" {
		t.Fatalf("生成应照常完成, got %q", r.text)
	}
	if n := strings.Count(buf.String(), "cancelGenerateText"); n != 0 {
		t.Fatalf("成功路径不应调 cancelGenerateText: %s", buf.String())
	}
}

// TestSelectedExplicitEffortPassesThrough 场景 2:用户显式 effort(哪怕不在
// 自声明 levels 里)→ 原样透传进 options,provider 是唯一裁决者。
func TestSelectedExplicitEffortPassesThrough(t *testing.T) {
	c, buf := newTestClient()
	type out struct {
		text string
		err  error
	}
	ch := make(chan out, 1)
	go func() {
		// adaptive-future 不在 snapshotDeepseekShape 的 levels 里:
		// 显式值照样透传,本地校验已删。
		s, err := c.GenerateTextSelected(context.Background(), "deepseek-v4-flash", "adaptive-future", "x")
		ch <- out{s, err}
	}()

	create := waitFrame(t, buf, "session/create")
	replyOK(c, create, snapshotDeepseekShape)

	gen := waitFrame(t, buf, "workspace/generateText")
	sel, _ := gen["params"].(map[string]interface{})["selection"].(map[string]interface{})
	opts, _ := sel["options"].(map[string]interface{})
	if opts["reasoningLevel"] != "adaptive-future" {
		t.Fatalf("用户显式 effort 必须原样透传进 options, got %v", sel)
	}
	replyOK(c, gen, `{"text":"ok"}`)

	r := <-ch
	if r.err != nil {
		t.Fatalf("显式 effort 不该被本地校验拒绝: %v", r.err)
	}
	if r.text != "ok" {
		t.Fatalf("got %q", r.text)
	}
}

// TestSelectedConsistentLevelsKeptExplicit 场景 3:一致元数据(defaultLevel 在
// 自声明 levels 内,GLM 系形状)→ defaultLevel 照旧显式下发进 options。
func TestSelectedConsistentLevelsKeptExplicit(t *testing.T) {
	c, buf := newTestClient()
	type out struct {
		text string
		err  error
	}
	ch := make(chan out, 1)
	go func() {
		s, err := c.GenerateTextSelected(context.Background(), "glm-x", "", "x")
		ch <- out{s, err}
	}()

	create := waitFrame(t, buf, "session/create")
	replyOK(c, create, snapshotConsistentLevels)

	gen := waitFrame(t, buf, "workspace/generateText")
	sel, _ := gen["params"].(map[string]interface{})["selection"].(map[string]interface{})
	opts, _ := sel["options"].(map[string]interface{})
	if opts["reasoningLevel"] != "high" {
		t.Fatalf("一致元数据的 defaultLevel 应照旧显式下发, got %v", sel)
	}
	replyOK(c, gen, `{"text":"ok"}`)

	r := <-ch
	if r.err != nil {
		t.Fatalf("不该报错: %v", r.err)
	}
	if r.text != "ok" {
		t.Fatalf("got %q", r.text)
	}
}
