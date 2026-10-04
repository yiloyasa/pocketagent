// pi_models_store_bucket_test.go —— models-store.json 桶感知回退源的回归钉
// (2026-10-01"必须能选择模型"第二轮:0.99.1 真实形状)。
//
// 【钉什么】pi_models_store_agency_test.go 钉的是 agency 管线对「models 为
// 键即 id dict」旧形态的行为;本文件钉 0.99.1 的**真实形状**(models 是数组,
// 本机 ~/.pi/agent/models-store.json 实证,cmd/pimodelsprobe 本机实跑):
//   - 候选 Provider 必须来自顶层桶键(数组形状下 agency 继承不到,曾全空 →
//     ModelID 拼成 "/<id>",手机选中即"无效 Pi 模型"整轮失败);
//   - DisplayName/ContextWindow/SupportsThinking/SupportsImages 字段完整;
//   - 顶层元数据/容器键不得被当成 provider(防伪造 ModelID);
//   - 缺 Provider 的候选在回退出口被跳过(宁缺毋假)。
package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// pi0991StoreShape 是本机 0.99.1 models-store.json 的实录形状(截取两个桶:
// deepseek 带 thinkingLevelMap 与 image 模态;zai-coding-cn 同构)。
const pi0991StoreShape = `{
  "deepseek": {
    "models": [
      {"id": "deepseek-flash", "name": "DeepSeek V4.1 Flash", "provider": "deepseek",
       "reasoning": true, "input": ["text", "image"], "contextWindow": 1000000,
       "thinkingLevelMap": {"minimal": null, "low": "low", "medium": null, "high": "high", "max": "max"}},
      {"id": "deepseek-v4-pro", "name": "DeepSeek V4 Pro", "provider": "deepseek",
       "reasoning": true, "input": ["text"], "contextWindow": 1000000}
    ],
    "checkedAt": 1789278793309, "lastModified": 1789122473000, "etag": "W/\"x\""
  },
  "zai-coding-cn": {
    "models": [
      {"id": "glm-5.3", "name": "GLM-5.3", "provider": "zai-coding-cn",
       "reasoning": true, "input": ["text"], "contextWindow": 1000000}
    ],
    "checkedAt": 1
  }
}`

func writePiModelsStore(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "models-store.json")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPiModelsStoreProbeRealShape(t *testing.T) {
	got, err := piModelsStoreProbe(writePiModelsStore(t, pi0991StoreShape))
	if err != nil {
		t.Fatal(err)
	}
	type want struct {
		display  string
		reason   bool
		window   int64
		hasImage bool
	}
	wants := map[string]want{
		"deepseek/deepseek-flash":  {"DeepSeek V4.1 Flash", true, 1000000, true},
		"deepseek/deepseek-v4-pro": {"DeepSeek V4 Pro", true, 1000000, false},
		"zai-coding-cn/glm-5.3":    {"GLM-5.3", true, 1000000, false},
	}
	if len(got) != len(wants) {
		t.Fatalf("want %d 候选, got %d: %+v", len(wants), len(got), got)
	}
	for _, c := range got {
		w, ok := wants[c.Provider+"/"+c.ID]
		if !ok {
			t.Fatalf("多余/错 provider 候选: %+v", c)
		}
		if c.DisplayName != w.display || c.Reasoning != w.reason || c.ContextWindow != w.window {
			t.Fatalf("%s/%s 字段不完整: want %+v got %+v", c.Provider, c.ID, w, c)
		}
		hasImage := false
		for _, in := range c.Input {
			if in == "image" {
				hasImage = true
			}
		}
		if hasImage != w.hasImage {
			t.Fatalf("%s/%s 模态不符: want image=%v got %v", c.Provider, c.ID, w.hasImage, c.Input)
		}
	}
}

func TestPiModelsStoreProbeDictShape(t *testing.T) {
	// 旧形态(键即 id 的 dict):agency 在桶内会把 "models" 当 provider 名,
	// 桶感知源必须以桶键盖章纠正 —— 不随形状漂移。
	store := `{
  "deepseek": {"models": {"deepseek-flash": {"name": "DeepSeek V4.1 Flash", "reasoning": true, "contextWindow": 128000}}},
  "zai-coding-cn": {"models": {"glm-5.3": {"name": "GLM-5.3", "reasoning": true}}}
}`
	got, err := piModelsStoreProbe(writePiModelsStore(t, store))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]string{}
	for _, c := range got {
		seen[c.Provider+"/"+c.ID] = c.DisplayName
	}
	for k, wantName := range map[string]string{
		"deepseek/deepseek-flash": "DeepSeek V4.1 Flash",
		"zai-coding-cn/glm-5.3":   "GLM-5.3",
	} {
		if seen[k] != wantName {
			t.Fatalf("%s: want %q got %q(all=%v)", k, wantName, seen[k], seen)
		}
	}
	if _, bad := seen["models/deepseek-flash"]; bad {
		t.Fatalf("provider 被伪造成桶内容器名 models: %v", seen)
	}
}

func TestPiModelsStoreProbeSkipsNonProviderKeys(t *testing.T) {
	// 顶层混入元数据与容器键:都不得产出带该 provider 名的候选。
	store := `{
  "checkedAt": 1,
  "etag": "W/\"x\"",
  "models": [{"id": "rogue", "name": "Rogue", "reasoning": true}],
  "settings": {"a": 1},
  "deepseek": {"models": [{"id": "deepseek-flash", "name": "DeepSeek V4.1 Flash", "reasoning": true}]}
}`
	got, err := piModelsStoreProbe(writePiModelsStore(t, store))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("只应有 deepseek 桶 1 条候选, got %d: %+v", len(got), got)
	}
	if got[0].Provider != "deepseek" || got[0].ID != "deepseek-flash" {
		t.Fatalf("候选异常: %+v", got[0])
	}
}

func TestIsProviderBucketKey(t *testing.T) {
	for _, ok := range []string{"deepseek", "zai-coding-cn", "openai", "x-ai"} {
		if !isProviderBucketKey(ok) {
			t.Errorf("%q 应判为 provider 桶", ok)
		}
	}
	for _, bad := range []string{"models", "Models", "checkedAt", "last_modified", "etag", "revision", "settings", ""} {
		if isProviderBucketKey(bad) {
			t.Errorf("%q 不得判为 provider 桶", bad)
		}
	}
}

func TestPiModelsStoreFallbackFrom(t *testing.T) {
	out := piModelsStoreFallbackFrom(writePiModelsStore(t, pi0991StoreShape))
	if len(out) != 3 {
		t.Fatalf("want 3 个 ModelInfo, got %d: %+v", len(out), out)
	}
	byID := map[string]pbModelInfoShape{}
	for i := range out {
		mi := &out[i]
		byID[mi.Id] = pbModelInfoShape{
			display: mi.DisplayName, window: mi.ContextWindow,
			thinking: mi.SupportsThinking, images: mi.SupportsImages,
			provider: mi.Provider, available: mi.Available,
		}
		if mi.Provider != "pi" || !mi.Available {
			t.Fatalf("%s: Provider/Available 异常: %+v", mi.Id, byID[mi.Id])
		}
	}
	v, ok := byID["deepseek/deepseek-flash"]
	if !ok {
		t.Fatalf("缺 deepseek/deepseek-flash: %v", byID)
	}
	if v.display != "DeepSeek V4.1 Flash" || v.window != 1000000 || !v.thinking || !v.images {
		t.Fatalf("deepseek-flash 字段不完整: %+v", v)
	}
	v, ok = byID["zai-coding-cn/glm-5.3"]
	if !ok {
		t.Fatalf("缺 zai-coding-cn/glm-5.3: %v", byID)
	}
	if v.images {
		t.Fatalf("glm-5.3 无 image 模态却报 SupportsImages: %+v", v)
	}
}

// pbModelInfoShape 把断言字段收拢成一个可比较形状(避免直接比较 pb 消息)。
type pbModelInfoShape struct {
	display   string
	window    int32
	thinking  bool
	images    bool
	provider  string
	available bool
}

func TestPiModelsStoreFallbackFromSkipsProviderless(t *testing.T) {
	// 防御形状:顶层无 provider 桶(整体 models 数组)→ 提取不出 provider,
	// 回退出口必须跳过(拼不出合法 "pi/<provider>/<id>"),绝不发坏候选。
	store := `{"models": [{"id": "rogue", "name": "Rogue", "reasoning": true}]}`
	if out := piModelsStoreFallbackFrom(writePiModelsStore(t, store)); len(out) != 0 {
		t.Fatalf("无 provider 候选应被跳过, got %+v", out)
	}
	// 文件缺失/坏 JSON:空列表(宁缺毋假),与旧行为一致。
	if out := piModelsStoreFallbackFrom(filepath.Join(t.TempDir(), "nope.json")); len(out) != 0 {
		t.Fatalf("缺失文件应返回空, got %+v", out)
	}
	if out := piModelsStoreFallbackFrom(writePiModelsStore(t, "{oops")); len(out) != 0 {
		t.Fatalf("坏 JSON 应返回空, got %+v", out)
	}
}

// TestPiModelsStoreSourceName 源名沿用 "store:" 前缀(Compile 的 From 诊断口径
// 不变;From 的盖章由 Compile 完成,已有 pi_models_store_agency_test.go 钉住)。
func TestPiModelsStoreSourceName(t *testing.T) {
	src := piModelsStoreSource(writePiModelsStore(t, pi0991StoreShape))
	if got := src.Name(); len(got) < 6 || got[:6] != "store:" {
		t.Fatalf("源名应带 store: 前缀, got %q", got)
	}
	cs, err := src.Probe(context.Background())
	if err != nil || len(cs) != 3 {
		t.Fatalf("Probe 应产 3 候选, got %d err=%v", len(cs), err)
	}
	for _, c := range cs {
		if c.Provider == "" {
			t.Fatalf("Probe 直接产出(未经 Compile)也必须带桶键 provider: %+v", c)
		}
	}
}
