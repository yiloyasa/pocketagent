// Package agency —— 四家 agent 的「目录型数据自适应编译架构」核心。
//
// 【它解决什么】各家 agent(codex/pi/zcode/dsh)的模型列表、命令、能力目录
// 等数据,来源会随上游版本漂移(RPC 改名/目录文件改结构/CLI 输出变化)。
// 本包提供与任何固定 schema 无关的**自适应提取 + 多来源编译**:
//   - Source:一个来源的一次探测(失败/为空不影响其他来源);
//   - ExtractCandidates:对任意形状 JSON 的宽容递归提取(别名表+启发式);
//   - Compile:并发探测全部来源 → 合并去重 → 字段级择优 → 稳定排序。
//
// 【适配力来自哪里】上游改字段名/换嵌套 → 提取器别名表与递归兜住;
// 上游加新来源(新 RPC/新文件)→ 实现 Source 追加进 Compile 参数即可,
// 编译器与消费端零改动(开放封闭)。
package agency

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"
)

// Candidate 一条模型候选。零值不可用语义由消费端保证(空 ID 不入列)。
type Candidate struct {
	Provider      string   // "pi" / "zai-coding-cn" / ""(未知,提取器尽力推断)
	ID            string   // provider 内模型 id(如 glm-5.3-flash)
	DisplayName   string   // 展示名(可空;空时消费端用 ID)
	Reasoning     bool     // 支持思考
	ContextWindow int64    // 上下文窗;0 = 未知
	Input         []string // 输入模态(text/image/...);nil = 未知
	From          string   // 来源名(诊断;多来源命中逗号并列)
}

// Source 一个模型候选来源的一次探测。
type Source interface {
	Name() string
	Probe(ctx context.Context) ([]Candidate, error)
}

// SourceFunc 便于用闭包实现 Source。
type SourceFunc struct {
	Name_   string
	ProbeFn func(ctx context.Context) ([]Candidate, error)
}

func (s SourceFunc) Name() string { return s.Name_ }
func (s SourceFunc) Probe(ctx context.Context) ([]Candidate, error) {
	return s.ProbeFn(ctx)
}

// Compile 并发探测全部来源(每源受 timeout 约束,失败/超时/为空的来源跳过,
// 绝不让单一来源拖垮整体),合并去重并按字段级择优排序。
//
// 合并键 = Provider + "/" + ID(大小写敏感,与线上 Thread.ModelID 同构)。
// 字段择优:非空者胜;ContextWindow 取最大;Reasoning 取或;Input 取更长序列;
// From 合并去重(逗号并列)。排序:"default" 语义(空 ID 或 ID=="default")置顶,
// 其余按 Provider、ID 字母序稳定输出。
func Compile(ctx context.Context, timeout time.Duration, sources ...Source) []Candidate {
	type slot struct {
		c     Candidate
		links map[string]bool
	}
	var (
		mu    sync.Mutex
		byKey = map[string]*slot{}
		order []string
	)
	add := func(cs []Candidate, from string) {
		mu.Lock()
		defer mu.Unlock()
		for _, c := range cs {
			if c.ID == "" {
				continue
			}
			key := c.Provider + "/" + c.ID
			cur, ok := byKey[key]
			if !ok {
				if c.From == "" {
					c.From = from
				}
				byKey[key] = &slot{c: c}
				order = append(order, key)
				continue
			}
			// 字段级择优:非空/更强者胜,来源并列。
			if cur.c.DisplayName == "" && c.DisplayName != "" {
				cur.c.DisplayName = c.DisplayName
			}
			if c.ContextWindow > cur.c.ContextWindow {
				cur.c.ContextWindow = c.ContextWindow
			}
			if c.Reasoning && !cur.c.Reasoning {
				cur.c.Reasoning = true
			}
			if len(c.Input) > len(cur.c.Input) {
				cur.c.Input = c.Input
			}
			if !strings.Contains(cur.c.From, from) {
				cur.c.From = strings.TrimSpace(cur.c.From + "," + from)
			}
		}
	}
	var wg sync.WaitGroup
	for _, src := range sources {
		if src == nil {
			continue
		}
		wg.Add(1)
		go func(s Source) {
			defer wg.Done()
			sctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			cs, err := s.Probe(sctx)
			if err != nil || len(cs) == 0 {
				return // 单来源失败隔离:不影响其他来源
			}
			add(cs, s.Name())
		}(src)
	}
	wg.Wait()

	out := make([]Candidate, 0, len(order))
	for _, key := range order {
		out = append(out, byKey[key].c)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		// default/跟随语义置顶
		ta, tb := isFollowDefault(a), isFollowDefault(b)
		if ta != tb {
			return ta
		}
		if a.Provider != b.Provider {
			return a.Provider < b.Provider
		}
		return a.ID < b.ID
	})
	return out
}

func isFollowDefault(c Candidate) bool {
	return c.ID == "" || c.ID == "default"
}
