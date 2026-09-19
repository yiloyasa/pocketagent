//go:build pa_live

package zcodeapp

// live 探针(需 -tags pa_live):真实拉一次 app-server,走生产 GenerateText
// 路径(session/create 选模 + workspace/generateText)。用于提示词优化功能的
// 端到端验证;配额耗尽或未登录时如实报错。平时 go test 不跑(依赖真进程+配额)。

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestGenerateTextLive(t *testing.T) {
	start := time.Now()
	c, err := Start(StartOptions{Workspace: os.TempDir()})
	if err != nil {
		t.Fatalf("启动 app-server 失败(本机没装 ZCode?): %v", err)
	}
	defer c.Close()
	t.Logf("握手耗时: %v", time.Since(start))

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	got, err := c.GenerateText(ctx, "把这句话改写成给 AI 编程代理的清晰指令:登录页面有时候点按钮没反应")
	t.Logf("GenerateText => %q, err=%v, 总耗时=%v", got, err, time.Since(start))
	if err != nil {
		t.Fatalf("live 生成失败(原样记录;402=Token Plan 配额上限属环境问题): %v", err)
	}
	if got == "" {
		t.Fatal("live 生成返回空串")
	}
}

// ---- 输出纯净度 A/B 对比(输出纯净度迭代验证)----
//
// 同一句改写请求,B 组走生产路径 GenerateText(纪律收尾 + 出口净化),
// A 组走"迭代前"裸调用(同模型、同原句、无纪律收尾、无净化),各跑
// abRuns 次,统计前言/多方案/代码围栏出现率。只统计不裁决:输出原样打进
// t.Logf,供人工复核;模型行为有随机性,结论看趋势不看单次。
//
// 基准模型:与 GenerateText 首选一致 —— 有偏好记忆且仍在候选里就用它,
// 否则用桌面 current。A/B 不一致时(如 A 组模型挂死)在日志里如实标注。

const (
	abRuns   = 3 // 每组次数:3 次够看趋势,也省配额(每次都可能烧 token)
	abPrompt = "把这句话改写成给 AI 编程代理的清晰指令:登录页面有时候点按钮没反应"
)

// abViolationStat 一次输出的违规统计。
type abViolationStat struct {
	Out       string // 原样输出(净化后)
	Preamble  bool   // 前言/开场白
	MultiPlan bool   // 多方案/备选项
	Fence     bool   // Markdown 代码围栏
}

// abCheckOutput 用启发式正则统计违规标记。仅供 live 报告使用,不进生产代码:
// 生产路径绝不启发式截取,这里的"检出"也只是统计信号,不代表输出真的不可用。
// 形态依据 2026-09-19 live 实测(见本次 A/B 日志):
//   - 前言不只"好的/以下是"客套词,更常见是"为了让 AI……"导语句 + "这里提供
//     几个方案"引出语,因此按开头导语词 OR 引出语双重检测;
//   - 多方案标记常出现在中部("### 方案 1:""版本二:"),不锚定行首。
func abCheckOutput(out string) abViolationStat {
	st := abViolationStat{Out: out}
	// 前言:开头客套/导语词,或"提供多版本"的引出语
	st.Preamble = regexp.MustCompile(
		`^\s*((好的|当然|没问题|明白|收到|为了让|为了|考虑到|首先|以下是|这是|下面是|改写后|优化后|OK[,，]|Sure\b|Of course|Certainly|Here\s*\(?s|Here is))`).
		MatchString(out) ||
		regexp.MustCompile(`(这里提供|为你提供|提供几个|以下几个不同|不同详细程度|根据实际情况选择|改写方案|改写版本)`).
			MatchString(out)
	// 多方案:方案/选项/版本/Option/Version + 编号,不限位置、不带尾部断言
	// (live 实测形态"### 方案 1:直接且可执行""### 选项一:通用且结构化",
	// 尾部是正文汉字,加断言必漏;"选项"词形来自 2026-09-19 A/B 第 2 轮实测)。
	// 误报面可忽略:纯净改写句不含"方案 1/选项二/版本三"字样。
	st.MultiPlan = regexp.MustCompile(`(方案|选项|版本|[Oo]ption|[Vv]ersion)\s*[一二三四五六1-9AB]`).
		MatchString(out)
	st.Fence = strings.Contains(out, "```")
	return st
}

// abBaselineSelection 决定 A/B 共用的基准模型:优先级为
//  1. 环境变量 PA_AB_MODEL(格式 "providerId/modelId",基准模型挂死时手动
//     指定一个能响应的候选,否则 A/B 两边全是超时,测不出纯净度);
//  2. 偏好记忆(与 GenerateText 的置顶规则一致);
//  3. 桌面 current(默认)。
//
// 返回 selection 形状。
func abBaselineSelection(t *testing.T, c *Client) map[string]interface{} {
	t.Helper()
	candidates, err := c.generateCandidates(context.Background())
	if err != nil || len(candidates) == 0 {
		t.Fatalf("拿候选模型失败: %v", err)
	}
	if override := os.Getenv("PA_AB_MODEL"); override != "" {
		i := strings.IndexByte(override, '/')
		if i <= 0 || i == len(override)-1 {
			t.Fatalf("PA_AB_MODEL 格式应为 providerId/modelId, got %q", override)
		}
		sel := map[string]interface{}{
			"providerId": override[:i],
			"modelId":    override[i+1:],
		}
		// 有 defaultLevel 就带上(有的模型强制要求 reasoningLevel,live 踩过)
		for _, cand := range candidates {
			p, _ := cand["providerId"].(string)
			m, _ := cand["modelId"].(string)
			if p == sel["providerId"] && m == sel["modelId"] {
				if opts, ok := cand["options"].(map[string]string); ok {
					sel["options"] = opts
				}
				break
			}
		}
		t.Logf("PA_AB_MODEL 指定基准: %v", sel)
		return sel
	}
	if pref := loadModelPref(); pref != nil {
		return promotePreferredModel(candidates, pref.ProviderID, pref.ModelID)[0]
	}
	return candidates[0]
}

// abRunRawBare 迭代前路径:同模型、裸 prompt(无纪律收尾、无净化)直接调
// workspace/generateText,应答走与生产相同的 parse(但跳过 sanitize,呈现
// 模型原始输出)。
func abRunRawBare(t *testing.T, c *Client, sel map[string]interface{}) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), generateTextAttemptBudget)
	defer cancel()
	raw, err := c.CallCtx(ctx, "workspace/generateText", map[string]interface{}{
		"workspace":       tempWorkspace(),
		"selection":       sel,
		"querySource":     generateTextQuerySource + "-ab-before",
		"prompt":          abPrompt,
		"maxOutputTokens": generateTextMaxOutputTokens,
		"operationId":     "pa-ab-before",
	})
	if err != nil {
		return "", err
	}
	return parseGenerateTextResult(raw)
}

func TestGenerateTextPurityABCompare(t *testing.T) {
	start := time.Now()
	c, err := Start(StartOptions{Workspace: os.TempDir()})
	if err != nil {
		t.Fatalf("启动 app-server 失败(本机没装 ZCode?): %v", err)
	}
	defer c.Close()

	sel := abBaselineSelection(t, c)
	selJSON, _ := json.Marshal(sel)
	t.Logf("基准模型: %s", selJSON)

	run := func(label string, fn func() (string, error)) []abViolationStat {
		var stats []abViolationStat
		for i := 1; i <= abRuns; i++ {
			out, err := fn()
			if err != nil {
				t.Logf("%s 第 %d/%d 次: 失败 err=%v", label, i, abRuns, err)
				// 402/1302 类错误原样上抛:配额阻塞时后续组也没必要跑
				if strings.Contains(err.Error(), "402") || strings.Contains(err.Error(), "1302") {
					t.Skipf("%s 撞上 402/1302(配额/限流),中止对比如实标注: %v", label, err)
				}
				continue
			}
			st := abCheckOutput(out)
			stats = append(stats, st)
			flags := []string{}
			if st.Preamble {
				flags = append(flags, "前言")
			}
			if st.MultiPlan {
				flags = append(flags, "多方案")
			}
			if st.Fence {
				flags = append(flags, "围栏")
			}
			if len(flags) == 0 {
				flags = append(flags, "纯净")
			}
			t.Logf("%s 第 %d/%d 次 [%s]: %q", label, i, abRuns, strings.Join(flags, ","), st.Out)
		}
		return stats
	}

	// A 组:迭代前(裸 prompt,无纪律、无净化)
	before := run("迭代前", func() (string, error) { return abRunRawBare(t, c, sel) })
	// B 组:迭代后(生产 GenerateText:纪律收尾 + sanitize)
	after := run("迭代后", func() (string, error) { return c.GenerateText(context.Background(), abPrompt) })

	rate := func(stats []abViolationStat, f func(abViolationStat) bool) string {
		if len(stats) == 0 {
			return "n/a"
		}
		n := 0
		for _, st := range stats {
			if f(st) {
				n++
			}
		}
		return fmt.Sprintf("%d/%d", n, len(stats))
	}
	t.Logf("==== 纯净度对比(基准模型 %s,总耗时 %v)====", selJSON, time.Since(start))
	t.Logf("前言出现率:   迭代前 %s → 迭代后 %s",
		rate(before, func(s abViolationStat) bool { return s.Preamble }),
		rate(after, func(s abViolationStat) bool { return s.Preamble }))
	t.Logf("多方案出现率: 迭代前 %s → 迭代后 %s",
		rate(before, func(s abViolationStat) bool { return s.MultiPlan }),
		rate(after, func(s abViolationStat) bool { return s.MultiPlan }))
	t.Logf("围栏出现率:   迭代前 %s → 迭代后 %s",
		rate(before, func(s abViolationStat) bool { return s.Fence }),
		rate(after, func(s abViolationStat) bool { return s.Fence }))
}

// TestGenerateTextLiveListModels 打印真实快照的 current + available 模型清单,
// 供挑选 A/B 对比基准(PA_AB_MODEL)用,不发生成请求、不烧配额。
func TestGenerateTextLiveListModels(t *testing.T) {
	c, err := Start(StartOptions{Workspace: os.TempDir()})
	if err != nil {
		t.Fatalf("启动 app-server 失败: %v", err)
	}
	defer c.Close()
	candidates, err := c.generateCandidates(context.Background())
	if err != nil {
		t.Fatalf("拿候选失败: %v", err)
	}
	for i, sel := range candidates {
		t.Logf("候选 %d: %s", i, func() string {
			b, _ := json.Marshal(sel)
			return string(b)
		}())
	}
}

// TestAbCheckOutputShapes 离线钉检测正则(零配额,样本取自 2026-09-19 真实
// A/B 日志的形态):迭代前的前言导语/多方案各词形必须全检出,迭代后的纯净
// 输出必须零误报。正则再改动时先过这里,不必烧 live 配额回归。
func TestAbCheckOutputShapes(t *testing.T) {
	// 迭代前形态(截取自真实违规输出,保留结构)
	dirty := []struct {
		name, out string
		want      abViolationStat
	}{
		{"导语+版本罗列", "为了让 AI 编程代理能够高效理解,这里提供三个不同详细程度的版本:\n\n### 版本一:简明直接版\n> “排查并修复”\n\n### 版本二:详细排查版\n> **任务目标**:排查",
			abViolationStat{Preamble: true, MultiPlan: true}},
		{"导语+选项罗列", "为了让 AI 编程代理高效地理解和修复这个 Bug,指令需要转化。\n\n这里提供几个不同详细程度的改写版本:\n\n### 选项一:通用且结构化(推荐)\n> “请帮我排查”\n\n### 选项二:特定技术栈",
			abViolationStat{Preamble: true, MultiPlan: true}},
		{"导语+方案数字编号", "为了让 AI 编程代理能高效定位并解决问题。\n\n### 方案 1:标准清晰版\n> “修复登录页面的 Bug”\n\n### 方案 2:详细结构化版",
			abViolationStat{Preamble: true, MultiPlan: true}},
		{"客套词开头", "好的,以下是改写结果:排查并修复登录问题",
			abViolationStat{Preamble: true}},
		{"整段围栏", "```markdown\n排查并修复登录页面的按钮 Bug。\n```",
			abViolationStat{Fence: true}},
	}
	for _, c := range dirty {
		if got := abCheckOutput(c.out); got.Preamble != c.want.Preamble ||
			got.MultiPlan != c.want.MultiPlan || got.Fence != c.want.Fence {
			t.Errorf("%s: got %+v want %+v", c.name, got, c.want)
		}
	}
	// 迭代后形态(真实纯净输出)+ 易误报样本,必须全部零违规
	clean := []string{
		"解决登录页面偶尔点击按钮无响应的问题。",
		"排查并修复登录页面按钮偶发点击无响应的问题。",
		"修复登录页面偶发点击按钮无响应的问题。",
		"把登录按钮改成加载两秒后提交,并给出验证方式", // 正文含"方案"不说,防将来误伤
		"给出修复方案,并验证登录按钮的点击响应",    // 正文单独出现"方案"无编号:不算多方案标记
	}
	for _, out := range clean {
		if got := abCheckOutput(out); got.Preamble || got.MultiPlan || got.Fence {
			t.Errorf("纯净样本被误报: %q => %+v", out, got)
		}
	}
}
