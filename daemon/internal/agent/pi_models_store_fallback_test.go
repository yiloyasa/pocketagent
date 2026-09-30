package agent

// piModelsStoreFallback 的合成夹具单测(2026-10-01):目录文件是"无法切换模型"
// 修复的数据源,解析器坏掉 = 全部模型消失。夹具为合成数据,非真实用户目录。

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPiModelsStoreFallback(t *testing.T) {
	dir := t.TempDir()
	store := `{
  "deepseek": {"models": {
    "deepseek-flash": {"name": "DeepSeek V4.1 Flash", "reasoning": true, "input": ["text"], "contextWindow": 128000}
  }},
  "zai-coding-cn": {"models": {
    "glm-5.3": {"name": "GLM-5.3", "reasoning": true, "input": ["text"], "contextWindow": 200000},
    "glm-5.3-flash": {"name": "GLM-5.3-Flash", "reasoning": true, "input": ["text", "image"], "contextWindow": 128000}
  }}
}`
	// 函数按 ~/.pi/agent/models-store.json 解析,夹具写到同构子路径
	p := filepath.Join(dir, ".pi", "agent", "models-store.json")
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(store), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)

	got := piModelsStoreFallback()
	if len(got) != 3 {
		t.Fatalf("want 3 models, got %d", len(got))
	}
	seen := map[string]bool{}
	for _, m := range got {
		seen[m.Id] = true
		if m.Provider != "pi" || !m.Available {
			t.Fatalf("bad model: %+v", m)
		}
		if m.Id == "zai-coding-cn/glm-5.3-flash" && !m.SupportsImages {
			t.Fatal("glm-5.3-flash 应按目录 input 含 image 而支持图片")
		}
	}
	for _, want := range []string{"deepseek/deepseek-flash", "zai-coding-cn/glm-5.3", "zai-coding-cn/glm-5.3-flash"} {
		if !seen[want] {
			t.Fatalf("缺模型 %s", want)
		}
	}
}

func TestPiModelsStoreFallbackMissingOrBroken(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	if got := piModelsStoreFallback(); len(got) != 0 {
		t.Fatalf("缺失文件应返回空, got %d", len(got))
	}
	p := filepath.Join(dir, ".pi", "agent", "models-store.json")
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	_ = os.WriteFile(p, []byte("not-json"), 0o644)
	if got := piModelsStoreFallback(); len(got) != 0 {
		t.Fatalf("坏文件应返回空, got %d", len(got))
	}
}
