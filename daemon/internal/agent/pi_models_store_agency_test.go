package agent

// pi 模型目录 → agency 自适应编译管道的接线测试(2026-10-01"必须能选择模型"):
// StoreFileSource 读 models-store.json(含 provider 分桶+键即 id 形态)→
// Compile 产出全量候选。目录文件缺失/损坏 = 空列表(宁缺毋假)。

import (
	"strings"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pocketagent/daemon/internal/agency"
)

func TestPiModelsStoreAgencyPipeline(t *testing.T) {
	dir := t.TempDir()
	store := `{
  "deepseek": {"checkedAt": 1, "models": {
    "deepseek-flash": {"name": "DeepSeek V4.1 Flash", "reasoning": true, "input": ["text"], "contextWindow": 128000}
  }},
  "zai-coding-cn": {"checkedAt": 2, "models": {
    "glm-5.3": {"name": "GLM-5.3", "reasoning": true, "input": ["text"], "contextWindow": 200000},
    "glm-5.3-flash": {"name": "GLM-5.3-Flash", "reasoning": true, "input": ["text", "image"], "contextWindow": 128000}
  }}
}`
	p := filepath.Join(dir, ".pi", "agent", "models-store.json")
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(store), 0o644); err != nil {
		t.Fatal(err)
	}

	got := agency.Compile(context.Background(), 5*time.Second,
		agency.StoreFileSource{Path: p})
	if len(got) != 3 {
		t.Fatalf("want 3 candidates, got %d: %+v", len(got), got)
	}
	seen := map[string]bool{}
	for _, c := range got {
		seen[c.Provider+"/"+c.ID] = true
		if !strings.HasPrefix(c.From, "store:") {
			t.Fatalf("From 应标记来源(store: 前缀): %+v", c)
		}
	}
	for _, want := range []string{"deepseek/deepseek-flash", "zai-coding-cn/glm-5.3", "zai-coding-cn/glm-5.3-flash"} {
		if !seen[want] {
			t.Fatalf("缺 %s", want)
		}
	}
}

func TestPiModelsStoreAgencyMissingFile(t *testing.T) {
	got := agency.Compile(context.Background(), 2*time.Second,
		agency.StoreFileSource{Path: filepath.Join(t.TempDir(), "nope", "models-store.json")})
	if len(got) != 0 {
		t.Fatalf("缺失文件应返回空, got %d", len(got))
	}
}
