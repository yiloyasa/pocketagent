package agency

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type Ctx = context.Context

var errBoom = errors.New("boom")

// 未知形状适应力夹具:字段全改名/嵌套变化/混入无关节点 —— 提取器仍应工作。
func mustJSON(t *testing.T, s string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestExtractUnknownShapeRenamedFields(t *testing.T) {
	// 字段全改名(contextWindow→ctx_window / name→label / 嵌套 wrapper)仍提取
	root := mustJSON(t, `{
	  "zai-coding-cn": {"checkedAt": 1, "etag": "x", "models": {
	    "glm-5.3-flash": {"label": "GLM Flash", "ctx_window": 128000,
	                      "modalities": ["text"], "wrapper": {"nested": true}}
	  }},
	  "deepseek": {"models": {
	    "v4-pro": {"title": "V4 Pro", "reasoning": true}
	  }}
	}`)
	got := ExtractCandidates(root)
	ids := map[string]bool{}
	for _, c := range got {
		ids[c.Provider+"/"+c.ID] = true
	}
	for _, want := range []string{"zai-coding-cn/glm-5.3-flash", "deepseek/v4-pro"} {
		if !ids[want] {
			t.Fatalf("缺 %s; got %v", want, ids)
		}
	}
}

func TestExtractSkipsNonModels(t *testing.T) {
	// 无 id 无辅助键的 dict 不是模型;登记容器(checkedAt 等)不产候选
	root := mustJSON(t, `{
	  "meta": {"checkedAt": 1758000000, "etag": "abc"},
	  "models": {
	    "glm-5.3": {"name": "GLM-5.3", "reasoning": true},
	    "empty": {"desc": "只有描述没有 id 形态键"}
	  }
	}`)
	got := ExtractCandidates(root)
	if len(got) != 1 || got[0].ID != "glm-5.3" {
		t.Fatalf("want 仅 glm-5.3, got %+v", got)
	}
}

func TestCompileSkipsFailedSourceAndDedupes(t *testing.T) {
	a := SourceFunc{Name_: "a", ProbeFn: func(ctx Ctx) ([]Candidate, error) {
		return []Candidate{{Provider: "zai-coding-cn", ID: "glm-5.3-flash", DisplayName: "GLM Flash", ContextWindow: 128000}}, nil
	}}
	b := SourceFunc{Name_: "b", ProbeFn: func(ctx Ctx) ([]Candidate, error) {
		return []Candidate{{Provider: "zai-coding-cn", ID: "glm-5.3-flash", ContextWindow: 200000}}, nil
	}}
	got := Compile(Ctx(context.Background()), 2, a, b)
	if len(got) != 1 {
		t.Fatalf("want 1 merged, got %d", len(got))
	}
	c := got[0]
	if c.ContextWindow != 200000 || c.DisplayName != "GLM Flash" || c.From == "" {
		t.Fatalf("字段择优/来源合并不符: %+v", c)
	}
}
