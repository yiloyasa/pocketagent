package agent

// codex_watchdog_consts_test.go —— P1-2「量化谎言注释 + 裸字面量」的防线钉子。
//
// 【本文件防的是哪一类再犯】阈值**值**与它的**注释/文案**各写各的。
// 实到:codex Run 里静默阈值是裸字面量 150_000(毫秒),而同段注释写"90 秒收不到
// 任何通知就判失败" —— 注释是假话。更糟的是 dsh/hermes/pi/zcode 四家的阈值注释
// 都写着"对齐 codex":基准源说 90 秒、对齐源全是 150 秒,基准与对齐互相矛盾,
// 下一个人按注释调 codex,四家立刻跟着漂。
//
// 【三道钉子,各防一面】
//  1. TestCodexWatchdogConstDefaults:三个常量(以及毫秒口径)不漂 —— 生产默认值
//     与提取前的裸字面量逐位等价(150 秒→150000 毫秒、60 分钟、10 秒),保证这次
//     提取是零行为变更(实值不许动)。
//  2. TestWatchdogThresholdFamilyStaysAligned:五家阈值同值。"对齐 codex"这句
//     注释从此有一个**可核对的真源**(对比 pi_watchdog_test.go 的同款先例:它钉了
//     pi 一家,这里钉全家)。看门狗**巡检节奏**不在对齐口径内(pi 更短,各家自定),
//     本文件显式写死,免得下一个人把 tick 也"对齐"掉。
//  3. TestCodexWatchdogCommentMatchesConst:注释与**用户可见文案**里的秒/分钟必须
//     与常量一致(锚点式取舍见文件末说明)。

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// ---- 1) 默认值与旧字面量逐位等价 ----

func TestCodexWatchdogConstDefaults(t *testing.T) {
	// 静默看门狗 150 秒;毫秒口径单独钉 —— 判定式是"单调毫秒相减",若有人把 150s
	// 误写成 150ms(或把 .Milliseconds() 误改成 .Seconds()),这条会红。
	if codexSilenceWatchdog != 150*time.Second {
		t.Fatalf("codexSilenceWatchdog 默认应 150 秒,得到 %s", codexSilenceWatchdog)
	}
	if got := codexSilenceWatchdog.Milliseconds(); got != 150_000 {
		t.Fatalf("codexSilenceWatchdog 的毫秒口径应等于旧字面量 150_000,得到 %d", got)
	}
	if codexTurnCap != 60*time.Minute {
		t.Fatalf("codexTurnCap 默认应 60 分钟,得到 %s", codexTurnCap)
	}
	if codexWatchdogTick != 10*time.Second {
		t.Fatalf("codexWatchdogTick 默认应 10 秒,得到 %s", codexWatchdogTick)
	}
}

// ---- 2) 五家"对齐 codex"的值一致性(基准源真源化) ----

func TestWatchdogThresholdFamilyStaysAligned(t *testing.T) {
	// 静默看门狗:codex / dsh / hermes / pi / zcode 五家同值。
	// (pi/hermes 是包级 var,仅为让测试可收紧阈值;本包无 t.Parallel,既有测试都用
	// defer/t.Cleanup 还原,此处读到的就是生产默认值。)
	for _, fam := range []struct {
		who string
		got time.Duration
	}{
		{"dsh", dshSilenceWatchdog},
		{"hermes", hermesSilenceWatchdog},
		{"pi", piSilenceWatchdog},
		{"zcode", zcodeWatchdogSilence},
	} {
		if fam.got != codexSilenceWatchdog {
			t.Fatalf("%s 静默看门狗应与 codex 对齐(%s),实得 %s", fam.who, codexSilenceWatchdog, fam.got)
		}
	}
	// 单轮上限:codex / dsh / hermes / pi 四家同值。
	// 【zcode 不在此列】zcode 的 60 分钟仍是 Run 内的裸字面量(zcode.go:2170),
	// 不在本次改动面;登记见 docs/reviews/2026-10-17/fix-agent-small-debts.md。
	for _, fam := range []struct {
		who string
		got time.Duration
	}{
		{"dsh", dshTurnCap},
		{"hermes", hermesTurnCap},
		{"pi", piTurnCap},
	} {
		if fam.got != codexTurnCap {
			t.Fatalf("%s 单轮上限应与 codex 对齐(%s),实得 %s", fam.who, codexTurnCap, fam.got)
		}
	}
	// 巡检节奏**不在**对齐口径内:pi 的 piWatchdogTick 比本家更短(见 pi.go 注释)。
	// 这里钉 hermes 与 codex 同档,并把"pi 不参与"写死,免得下一个人把 tick 也当成
	// 必须对齐的项(那会把 pi 的口径无声改掉)。
	if hermesWatchdogTick != codexWatchdogTick {
		t.Fatalf("hermes 看门狗节奏应与 codex 同为 %s,实得 %s", codexWatchdogTick, hermesWatchdogTick)
	}
	if piWatchdogTick == codexWatchdogTick {
		t.Fatalf("pi 的巡检节奏是各家自定,若真要与 codex 同值需连注释一起改;实得 %s", piWatchdogTick)
	}
}

// ---- 3) 注释 / 用户可见文案 与 常量一致(同类再犯防线) ----
//
// 【为什么锚点式而不是全文扫描】全文扫"N 秒 / N 分钟"必然误伤:同文件里 120 秒的
// callTimeout、15 分钟的审批预算、8 秒的握手预算都是别的阈值,还有"P1-2 之前这里
// 错写成 90 秒"这类**历史陈述**。所以本用例只校验与本主题直接相关的锚点:
//  ① 三个常量各自的文档注释;② 两条会发到手机上的静默/超限错误文案 + 一条日志;
// ③ 等待循环那段说明注释(P1-2 的原案发位置)。
// 任何一处出现**错的**秒/分钟数、或该出现的数缺失,都会红;措辞怎么改都不会误伤。

var (
	secondsLiteral = regexp.MustCompile(`(\d+)\s*秒`)
	minutesLiteral = regexp.MustCompile(`(\d+)\s*分钟`)
)

// parseAgentSource 解析本包的一个源文件(带注释),连同位置信息用的 FileSet。
//
// 【为什么直接按文件名读】go test 的工作目录恒为本包目录,"codex.go" 即被测源码;
// 写死路径比 runtime.Caller 少一层不确定性,且报错信息直接指向文件与行号。
func parseAgentSource(t *testing.T, name string) (*ast.File, *token.FileSet) {
	t.Helper()
	src, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("读 %s 失败(工作目录应为本包目录): %v", name, err)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, src, parser.ParseComments)
	if err != nil {
		t.Fatalf("解析 %s 失败: %v", name, err)
	}
	return file, fset
}

// codexConstDoc 取 codex.go 里某个常量的文档注释文本。
func codexConstDoc(t *testing.T, name string) string {
	t.Helper()
	file, _ := parseAgentSource(t, "codex.go")
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for _, ident := range vs.Names {
				if ident.Name == name && vs.Doc != nil {
					return vs.Doc.Text()
				}
			}
		}
	}
	t.Fatalf("codex.go 里找不到常量 %s 或它的文档注释(改名/删注释会让这条钉子失效,请同步更新)", name)
	return ""
}

// codexStringLiterals 取 codex.go 里全部字符串字面量的文本(含格式串)。
func codexStringLiterals(t *testing.T) []string {
	t.Helper()
	file, _ := parseAgentSource(t, "codex.go")
	var out []string
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		if text, err := strconv.Unquote(lit.Value); err == nil {
			out = append(out, text)
		}
		return true
	})
	return out
}

func TestCodexWatchdogCommentMatchesConst(t *testing.T) {
	// ① 三个常量各自的文档注释:该出现的数必须在,且不许出现别的数。
	for _, anchor := range []struct {
		constName string
		unit      *regexp.Regexp
		want      int
	}{
		{"codexSilenceWatchdog", secondsLiteral, 150},
		{"codexTurnCap", minutesLiteral, 60},
		{"codexWatchdogTick", secondsLiteral, 10},
	} {
		doc := codexConstDoc(t, anchor.constName)
		matches := anchor.unit.FindAllStringSubmatch(doc, -1)
		if len(matches) == 0 {
			t.Fatalf("%s 的文档注释里没有写出阈值(注释与常量必须互相可核对)", anchor.constName)
		}
		for _, m := range matches {
			got, err := strconv.Atoi(m[1])
			if err != nil {
				t.Fatalf("%s 注释里的数字 %q 解析失败: %v", anchor.constName, m[1], err)
			}
			if got != anchor.want {
				t.Fatalf("%s 的注释写 %s,与常量实值 %d 不符(注释不许再当第二份真值)",
					anchor.constName, m[0], anchor.want)
			}
		}
	}

	// ② 用户可见文案:静默超限的两条错误文案必须写 150 秒。
	silenceLits := 0
	for _, lit := range codexStringLiterals(t) {
		if !strings.Contains(lit, "无任何事件") {
			continue
		}
		silenceLits++
		if !strings.Contains(lit, "150 秒") {
			t.Fatalf("静默超限的错误文案必须与常量一致(150 秒),得到: %q", lit)
		}
		for _, m := range secondsLiteral.FindAllStringSubmatch(lit, -1) {
			if m[1] != "150" {
				t.Fatalf("静默超限的错误文案里出现 %s,与常量 150 秒不符: %q", m[0], lit)
			}
		}
	}
	if silenceLits == 0 {
		t.Fatal("没找到静默超限的错误文案(文案被改名会让这条钉子失效,请同步更新)")
	}

	// 单轮超限的两条错误文案 + 一条日志必须写 60 分钟。
	// 【锚句要窄】"审批等待超时(%s)" 也含"超时(",但那是审批预算(15 分钟)、
	// 与单轮上限无关,不能被本条扫进来;只认 codex 自己的两种表述。
	timeoutLits := 0
	for _, lit := range codexStringLiterals(t) {
		if !strings.Contains(lit, "codex 超时(") && !strings.Contains(lit, "轮次超时(") {
			continue
		}
		timeoutLits++
		if !strings.Contains(lit, "60 分钟") {
			t.Fatalf("单轮超限的错误文案必须与常量一致(60 分钟),得到: %q", lit)
		}
		for _, m := range minutesLiteral.FindAllStringSubmatch(lit, -1) {
			if m[1] != "60" {
				t.Fatalf("单轮超限的错误文案里出现 %s,与常量 60 分钟不符: %q", m[0], lit)
			}
		}
	}
	if timeoutLits == 0 {
		t.Fatal("没找到单轮超限的错误文案(文案被改名会让这条钉子失效,请同步更新)")
	}

	// ③ 等待循环那段说明注释("…就判失败"是它的锚句)必须写 150 秒 —— 这正是
	// P1-2 的原案发位置,此前这里是"90 秒",与 150 秒实值互相矛盾。
	waitDoc := ""
	file, _ := parseAgentSource(t, "codex.go")
	for _, group := range file.Comments {
		if text := group.Text(); strings.Contains(text, "收不到任何通知") {
			waitDoc = text
			break
		}
	}
	if waitDoc == "" {
		t.Fatal("没找到等待循环的说明注释(注释被改名会让这条钉子失效,请同步更新)")
	}
	if !strings.Contains(waitDoc, "150 秒") {
		t.Fatalf("等待循环注释必须与常量一致(150 秒),注释里却没有: %q", waitDoc)
	}
}
