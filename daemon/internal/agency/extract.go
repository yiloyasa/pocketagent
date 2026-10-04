package agency

import (
	"strings"
)

// ExtractCandidates 对**任意形状**的 JSON 树(any:map[string]any / []any /
// 基础类型混合)做宽容递归提取,产出模型候选。不依赖任何固定 schema。
//
// 统一遍历语义(2026-10-01 定稿,替代早期双分支版本):
//   - 对每个 dict 的每个键值对 k→v:
//     · v 是 dict 且是「模型条目」→ 收编为候选(id = 值内 id 别名,**或键名 k**);
//     · 否则递归下钻(idHint=k,provider 继承)。
//   - 「模型条目」判定:id 非空(值内 id 别名或键名)且命中至少一个辅助键
//     (name/reasoning/contextWindow/input 等)。两者皆无 = 普通容器,下钻。
//   - provider 继承:祖先键名(zai-coding-cn: {...} 的子树内 provider=zai-coding-cn)。
//   - 防误报:登记元数据(checkedAt/etag)标量不产候选;无 id 无辅助不产候选。
//
// 返回按发现顺序排列;去重键 = provider/id。合并与排序由 Compile 负责。
func ExtractCandidates(root any) []Candidate {
	e := &extractor{seen: map[string]bool{}}
	e.walk(root, "", "", 0)
	return e.out
}

const maxExtractDepth = 12

type extractor struct {
	out  []Candidate
	seen map[string]bool
}

func (e *extractor) walk(node any, prov, idHint string, depth int) {
	if depth > maxExtractDepth {
		return
	}
	switch v := node.(type) {
	case map[string]any:
		if c, ok := modelEntry(v, idHint); ok {
			if c.Provider == "" {
				c.Provider = prov
			}
			key := c.Provider + "/" + c.ID
			if !e.seen[key] {
				e.seen[key] = true
				e.out = append(e.out, c)
			}
			return // 模型条目的子字段不是候选
		}
		for k, vv := range v {
			switch item := vv.(type) {
			case map[string]any:
				// provider 继承:键名是 provider 分桶(子树是模型集合)时,
				// 子树继承键名作为 provider 上下文 —— 首次进入时确立。
				childProv := prov
				if prov == "" && looksLikeModelCollection(item) {
					childProv = k
				}
				e.walk(item, childProv, k, depth+1)
			case []any:
				for _, it := range item {
					e.walk(it, prov, k, depth+1)
				}
			}
		}
	case []any:
		for _, it := range v {
			e.walk(it, prov, "", depth+1)
		}
	}
}

// modelEntry 判定 m 是否一条模型条目并提取。id 取值优先级:值内 id 别名 >
// 键名 idHint;辅助键判定防止把任意容器误认成模型。
func modelEntry(m map[string]any, idHint string) (Candidate, bool) {
	id, _ := strAny(m, idAliases...)
	if id == "" {
		id = idHint
	}
	if id == "" {
		return Candidate{}, false
	}
	aux := false
	for _, a := range auxKeys {
		if _, ok := m[a]; ok {
			aux = true
			break
		}
	}
	if !aux {
		return Candidate{}, false
	}
	c := Candidate{ID: id}
	if n, ok := strAny(m, "name", "displayName", "display_name", "label", "title"); ok {
		c.DisplayName = n
	}
	for _, a := range []string{"reasoning", "thinking", "supportsThinking", "supports_thinking"} {
		if v, ok := m[a].(bool); ok {
			c.Reasoning = v
			break
		}
	}
	for _, a := range []string{"contextWindow", "context_window", "ctxWindow", "context"} {
		if v, ok := m[a]; ok {
			switch n := v.(type) {
			case float64:
				c.ContextWindow = int64(n)
			case int64:
				c.ContextWindow = n
			case int:
				c.ContextWindow = int64(n)
			}
			break
		}
	}
	for _, a := range []string{"input", "modalities", "input_modalities"} {
		if arr, ok := m[a].([]any); ok {
			for _, it := range arr {
				if s, ok := it.(string); ok {
					c.Input = append(c.Input, s)
				}
			}
			break
		}
	}
	return c, true
}

// ---- 别名表与工具(自适应提取的词汇层) ----

// idAliases 提取「模型 id」的键别名(按优先序)。
var idAliases = []string{"id", "modelId", "model_id", "model", "slug"}

// auxKeys 辅助键:证明一个 dict 是「模型条目」而非普通容器
// (至少命中一个;全部未命中 = 普通容器,下钻不误报)。
var auxKeys = []string{"name", "displayName", "display_name", "label", "title",
	"reasoning", "thinking", "supportsThinking", "supports_thinking",
	"contextWindow", "context_window", "input", "modalities"}

// strAny 按别名序提取第一个非空 string。
func strAny(m map[string]any, aliases ...string) (string, bool) {
	for _, a := range aliases {
		if v, ok := m[a]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s, true
			}
		}
	}
	return "", false
}

// looksLikeModelCollection 判定 dict 是否「模型集合」:至少一半的值是 dict,
// 且其中至少一个含 id 形态键(provider 分桶继承的判定依据)。
func looksLikeModelCollection(m map[string]any) bool {
	return looksLikeModelCollectionDepth(m, 0)
}

func looksLikeModelCollectionDepth(m map[string]any, depth int) bool {
	if depth > 2 {
		return false
	}
	dicts, modelish := 0, 0
	for k, v := range m {
		if vm, ok := v.(map[string]any); ok {
			dicts++
			// 模型形态 = 值内 id 别名命中,或「键可作 id 且值含辅助键」
			// (models-store 的键即 id 形态:glm-5.3-flash: {label, ctx_window,…})
			if _, has := strAny(vm, idAliases...); has {
				modelish++
			} else if _, hasAux := strAny(vm, auxKeys...); hasAux {
				modelish++
			} else if isModelishKey(k) && len(vm) > 0 {
				modelish++
			} else if looksLikeModelCollectionDepth(vm, depth+1) {
				// provider 容器形态(models-store):值是「模型集合」的容器
				return true
			}
		}
	}
	return dicts > 0 && modelish > 0
}

// isModelishKey 键名本身像模型标识(非登记元数据、非明显容器词)。
func isModelishKey(k string) bool {
	for _, bad := range metaKeys {
		if strings.EqualFold(k, bad) {
			return false
		}
	}
	for _, bad := range []string{"models", "catalog", "providers", "items", "data", "list", "settings"} {
		if strings.EqualFold(k, bad) {
			return false
		}
	}
	return k != ""
}

// metaKeys 登记元数据键(目录容器的登记信息,不是模型,也不是 provider 名)。
var metaKeys = []string{"checkedAt", "checked_at", "lastModified", "last_modified", "etag", "revision"}
