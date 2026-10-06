// codexmetaprobe 探针:codex 0.156.1 的**方法面(method surface)diff**。
//
// 【背景】vendored schema(codex-schema/codex_app_server_protocol.v2.schemas.json)
// 是按 0.155.1 二进制逐名验证过的(95 客户端请求 + 75 服务端通知)。本机 codex
// 已升到 0.156.1,release notes 只列产品面变更(模型目录//tui/语音//usage/
// worktree/主题),协议面是否有增删没有任何官方证据 —— 本探针补上这块证据。
//
// 【要回答的三个问题】
//  1. schema 里的 95+75 个方法名,在 0.156.1 二进制里是否全部还在(消失清单)?
//  2. 0.156.1 二进制里有没有 schema 之外的协议方法形状字符串(新增候选)?
//  3. daemon 的 codexapp 客户端与 0.156.1 能否完成轻量握手(initialize +
//     thread/list),错误通道形状是否仍是"未知方法 → JSON-RPC error"?
//
// 【做法与红线】三步,全部轻量、零副作用:
//  1. 二进制静态扫描(证据分级 B):整文件字节查找 schema 方法名;反向扫描
//     `seg/seg...` 形状的字符串找 schema 外候选。字符串命中是**必要条件**
//     证据,不是运行时证明 —— 分级如实标 B。
//  2. 活体轻量握手(证据分级 A):经 daemon 同款 codexapp.Start() 拉起
//     app-server,只调 initialize / thread/list / 一个故意不存在的校准方法。
//     **绝不跑真实 LLM 轮次**(不 thread/start、不 turn/start),**绝不碰
//     ~/.codex/config.toml**。
//  3. 反向扫描出的"新增候选"按命名语义给建议归类;证不出语义的如实标
//     UNKNOWN,不猜(证据分级 C)。
//
// 【用法】go run ./cmd/codexmetaprobe
// (-out 缺省写系统临时目录,可显式指定证据落点;-bin 直接指向原生 codex.exe,
// 默认从 codex.cmd 包装器自动解析)
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/pocketagent/daemon/internal/codexapp"
	"github.com/pocketagent/daemon/internal/hidewin"
)

// methodShape 是一条候选方法名:全小写段 + camelCase 段,`/` 分隔。
var methodShape = regexp.MustCompile(`[A-Za-z][A-Za-z0-9]*(?:/[A-Za-z0-9_]+)+`)

func main() {
	schemaPath := flag.String("schema",
		defaultSchemaPath(),
		"vendored 协议 schema 路径(方法面的 0.155.1 基线)")
	out := flag.String("out", "", "证据输出路径(缺省写到系统临时目录)")
	bin := flag.String("bin", "", "原生 codex.exe 路径(默认从 LocateCodex 的结果自动解析)")
	flag.Parse()

	// 证据落点:缺省写系统临时目录(通用派生,不写死任何个人专属绝对路径
	// —— 字符串常量会随探针二进制分发,规格 §2 明令禁止)。
	outPath := *out
	if outPath == "" {
		outPath = filepath.Join(os.TempDir(), "codex-0156-probe.log")
	}

	rep := &report{}
	run(*schemaPath, *bin, rep)

	body := []byte(rep.text.String())
	if err := os.WriteFile(outPath, body, 0o644); err != nil {
		log.Fatalf("写证据文件: %v", err)
	}
	fmt.Printf("证据已写入 %s(%d 字节)\n", outPath, len(body))
	fmt.Print(rep.summary.String())
}

// report 同时攒完整证据(落盘)与摘要(打印)。
type report struct {
	text    strings.Builder
	summary strings.Builder
}

func (r *report) sec(format string, a ...any) {
	s := fmt.Sprintf(format, a...)
	r.text.WriteString(s + "\n")
	fmt.Println(s)
}

func run(schemaPath, binFlag string, rep *report) {
	// ---- 第 0 步:解析 vendored schema,拿到 0.155.1 方法面基线 ----
	reqs, notifs, err := schemaMethods(schemaPath)
	if err != nil {
		log.Fatalf("解析 schema: %v", err)
	}
	rep.sec("# codex-0156-probe —— 0.156.1 方法面 diff 证据")
	rep.sec("# 证据分级: A=运行时实测(live JSON-RPC 应答) / B=二进制静态字节命中(必要条件,非运行时证明) / C=启发式反向扫描(需人工判读)")
	rep.sec("# 时间: %s", time.Now().Format(time.RFC3339))
	rep.sec("# schema 基线: %s(客户端请求 %d + 服务端通知 %d)", schemaPath, len(reqs), len(notifs))

	// ---- 第 1 步:定位原生 codex.exe,记版本 ----
	bin, err := resolveNativeExe(binFlag)
	if err != nil {
		log.Fatalf("定位原生 codex.exe: %v", err)
	}
	version := "未知"
	cmd := exec.Command(bin, "--version")
	hidewin.Apply(cmd)
	if out, err := cmd.CombinedOutput(); err == nil {
		version = strings.TrimSpace(string(out))
	} else {
		version = fmt.Sprintf("未知(--version 失败: %v)", err)
	}
	rep.sec("# 原生二进制: %s", bin)
	rep.sec("# 版本: %s(证据分级 A:进程实跑输出)", version)

	raw, err := os.ReadFile(bin)
	if err != nil {
		log.Fatalf("读二进制: %v", err)
	}
	rep.sec("# 二进制大小: %d 字节", len(raw))

	// ---- 第 2 步(B 级证据):schema 方法逐名查找 → 消失清单 ----
	var missingReqs, missingNotifs []string
	rep.sec("")
	rep.sec("## 一、schema 方法在 0.156.1 二进制中的存在性(证据分级 B)")
	rep.sec("### 客户端请求(%d 个)", len(reqs))
	for _, m := range reqs {
		if !bytes.Contains(raw, []byte(m)) {
			missingReqs = append(missingReqs, m)
		}
	}
	rep.sec("命中 %d / %d", len(reqs)-len(missingReqs), len(reqs))
	if len(missingReqs) == 0 {
		rep.sec("消失请求: 无 —— schema 全部 95 个请求方法名在 0.156.1 二进制中仍可找到")
	} else {
		rep.sec("消失请求(嫌疑,需 live 校准复核): %s", strings.Join(missingReqs, ", "))
	}
	rep.sec("### 服务端通知(%d 个)", len(notifs))
	for _, m := range notifs {
		if !bytes.Contains(raw, []byte(m)) {
			missingNotifs = append(missingNotifs, m)
		}
	}
	rep.sec("命中 %d / %d", len(notifs)-len(missingNotifs), len(notifs))
	if len(missingNotifs) == 0 {
		rep.sec("消失通知: 无 —— schema 全部 75 个通知方法名在 0.156.1 二进制中仍可找到")
	} else {
		rep.sec("消失通知(嫌疑): %s", strings.Join(missingNotifs, ", "))
	}

	// ---- 第 2.5 步(C 级证据):反向扫描 schema 外的方法形状候选 ----
	known := make(map[string]bool, len(reqs)+len(notifs))
	firstSegs := map[string]bool{}
	for _, m := range append(append([]string{}, reqs...), notifs...) {
		known[m] = true
		firstSegs[strings.SplitN(m, "/", 2)[0]] = true
	}
	// 协议外的疑似新命名空间也纳入首段观察集,避免新方法整个被贪心匹配吞掉。
	for _, seg := range []string{"hook", "process", "voice", "usage", "worktree", "session", "environment", "realtime", "collab", "login", "tui", "memory", "project", "rollout", "remoteControl", "userVerification", "collaborationMode", "mock", "getConversationSummary", "gitDiffToRemote", "getAuthStatus"} {
		firstSegs[seg] = true
	}
	candNew, candUnknown := reverseScan(raw, known, firstSegs)
	rep.sec("")
	rep.sec("## 二、schema 之外的方法形状候选(证据分级 C:启发式,需人工判读)")
	rep.sec("### 2a. 首段已知、全名不在 schema 中(%d 个;与第四节 A 级 diff 相互印证)", len(candNew))
	for _, c := range candNew {
		mark := ""
		if strings.HasPrefix(c, "thread/") || strings.HasPrefix(c, "item/") ||
			strings.HasPrefix(c, "turn/") || strings.HasPrefix(c, "account/") ||
			strings.HasPrefix(c, "hook/") || strings.HasPrefix(c, "process/") ||
			strings.HasPrefix(c, "mcpServer/") {
			mark = "  <-- 协议命名空间,重点判读"
		}
		rep.sec("  %s%s", c, mark)
	}
	if len(candNew) == 0 {
		rep.sec("  (无)")
	}
	rep.sec("### 2b. 其余形状相似串(噪声嫌疑,仅存档;%d 个,最多列 120)", len(candUnknown))
	for i, c := range candUnknown {
		if i >= 120 {
			rep.sec("  ...(其余 %d 个略)", len(candUnknown)-120)
			break
		}
		rep.sec("  %s", c)
	}

	// ---- 第 3 步(A 级证据):活体轻量握手 ----
	// 先握手:未知方法校准探测会逼 0.156.1 自报**完整请求面枚举**,拿它对 schema
	// 做 A 级 diff(第 3.5 步),比二进制字节扫描(B 级)强一个证据等级。
	rep.sec("")
	rep.sec("## 三、活体轻量握手(证据分级 A:真实 JSON-RPC 应答;零 LLM 轮次、零配置写入)")
	runtimeReqs := liveHandshake(rep)

	// ---- 第 3.5 步(A 级 diff):运行时自报请求面 vs vendored schema ----
	rep.sec("")
	rep.sec("## 四、运行时请求面 vs vendored schema(A 级 diff)")
	if len(runtimeReqs) == 0 {
		rep.sec("未能取得运行时请求面枚举(校准探测失败),本节跳过")
	} else {
		rep.sec("运行时自报请求数: %d(vendored schema: %d)", len(runtimeReqs), len(reqs))
		schemaSet := map[string]bool{}
		for _, m := range reqs {
			schemaSet[m] = true
		}
		runtimeSet := map[string]bool{}
		var vanished, added []string
		for _, m := range runtimeReqs {
			runtimeSet[m] = true
			if !schemaSet[m] {
				added = append(added, m)
			}
		}
		for _, m := range reqs {
			if !runtimeSet[m] {
				vanished = append(vanished, m)
			}
		}
		sort.Strings(added)
		// 【证据边界】schema 曾对 0.155.1 二进制"全命中"只证明 schema ⊆ 0.155.1,
		// 证明不了 0.155.1 方法面 == schema。所以"新增"是"相对 vendored schema 新增",
		// 不等于"0.156 相对 0.155 新增";"消失"则两边证据齐全(A+B),可下结论。
		rep.sec("### 4a. 消失(schema 有、0.156.1 运行时拒绝;A+B 双证): %d 个", len(vanished))
		for _, m := range vanished {
			rep.sec("  - %s", m)
		}
		if len(vanished) == 0 {
			rep.sec("  (无)")
		}
		rep.sec("### 4b. 相对 schema 新增(运行时接受、schema 没有;A 级): %d 个", len(added))
		for _, m := range added {
			rep.sec("  - %s", m)
		}
		if len(added) == 0 {
			rep.sec("  (无)")
		}
		rep.summary.WriteString(fmt.Sprintf("A 级 diff: 消失 %d;相对 schema 新增 %d\n", len(vanished), len(added)))
	}

	// ---- 摘要 ----
	rep.summary.WriteString("\n—— 摘要 ——\n")
	rep.summary.WriteString(fmt.Sprintf("版本: %s\n", version))
	rep.summary.WriteString(fmt.Sprintf("消失请求: %d;消失通知: %d\n", len(missingReqs), len(missingNotifs)))
	rep.summary.WriteString(fmt.Sprintf("schema 外候选(协议命名空间): %d;其余候选: %d\n", len(candNew), len(candUnknown)))
}

// schemaMethods 从 vendored schema 抽 ClientRequest / ServerNotification 的方法名。
func schemaMethods(path string) (reqs, notifs []string, err error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	var doc struct {
		Definitions map[string]struct {
			OneOf []struct {
				Properties map[string]json.RawMessage `json:"properties"`
			} `json:"oneOf"`
		} `json:"definitions"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, nil, err
	}
	extract := func(def string) ([]string, error) {
		d, ok := doc.Definitions[def]
		if !ok {
			return nil, fmt.Errorf("schema 缺 definitions.%s", def)
		}
		var out []string
		for _, v := range d.OneOf {
			mraw := v.Properties["method"]
			if len(mraw) == 0 {
				continue
			}
			var mv struct {
				Const string   `json:"const"`
				Enum  []string `json:"enum"`
			}
			if err := json.Unmarshal(mraw, &mv); err != nil {
				return nil, err
			}
			name := mv.Const
			if name == "" && len(mv.Enum) > 0 {
				name = mv.Enum[0]
			}
			if name != "" {
				out = append(out, name)
			}
		}
		sort.Strings(out)
		return out, nil
	}
	if reqs, err = extract("ClientRequest"); err != nil {
		return nil, nil, err
	}
	if notifs, err = extract("ServerNotification"); err != nil {
		return nil, nil, err
	}
	return reqs, notifs, nil
}

// reverseScan 反向扫描:找 schema 之外的 `seg/seg...` 形状候选。
//
// 【为什么不能一把 regex 抓完】Rust 二进制的字符串常量是**连续拼接、无分隔符**的,
// 贪心匹配会把 "thread/listaccount/read" 这样的相邻方法粘成一个超串。对策:
// 除了整段 findall,再从段内每个 '/' 之后重试锚定,把被吞的下半截也捞出来;
// 已知首段单独逐处锚定扫描。产出仍是启发式的,分级 C,人工判读兜底。
func reverseScan(raw []byte, known map[string]bool, firstSegs map[string]bool) (newInNS, unknown []string) {
	seen := map[string]bool{}
	add := func(tok string) {
		tok = strings.Trim(tok, "/")
		if tok == "" || seen[tok] || known[tok] {
			return
		}
		// 形状过滤:至少两段,首段全小写字母且长度 ≥3,每段非空。
		segs := strings.Split(tok, "/")
		if len(segs) < 2 || len(tok) > 64 {
			return
		}
		first := segs[0]
		if len(first) < 3 || first != strings.ToLower(first) || !isWord(first) {
			return
		}
		for _, s := range segs[1:] {
			if s == "" || !isWordLoose(s) {
				return
			}
		}
		seen[tok] = true
		if firstSegs[first] {
			newInNS = append(newInNS, tok)
		} else {
			unknown = append(unknown, tok)
		}
	}
	word := func(b byte) bool {
		return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b == '_' || b == '/' || b == '-'
	}
	// 逐偏移提取方法形状 token(等价 findall,但可控制边界)。
	for i := 0; i < len(raw); {
		if !isAlpha(raw[i]) {
			i++
			continue
		}
		j := i
		for j < len(raw) && word(raw[j]) {
			j++
		}
		if run := raw[i:j]; bytes.ContainsRune(run, '/') {
			for _, m := range methodShape.FindAllString(string(run), -1) {
				add(m)
				// 贪心吞并的补救:从每个内部 '/' 之后重新锚定。
				for k := 0; k < len(m); k++ {
					if m[k] == '/' {
						add(m[k+1:])
					}
				}
			}
		}
		i = j
	}
	sort.Strings(newInNS)
	sort.Strings(unknown)
	return newInNS, unknown
}

func isAlpha(b byte) bool { return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' }
func isWord(s string) bool {
	for i := 0; i < len(s); i++ {
		b := s[i]
		if !isAlpha(b) && !(b >= '0' && b <= '9') && b != '_' {
			return false
		}
	}
	return true
}
func isWordLoose(s string) bool {
	for i := 0; i < len(s); i++ {
		b := s[i]
		if !isAlpha(b) && !(b >= '0' && b <= '9') && b != '_' {
			return false
		}
	}
	return true
}

// runtimeEnumRe 从"未知方法"错误的文案里抓 0.156.1 自报的完整请求面枚举。
var runtimeEnumRe = regexp.MustCompile("expected one of (.+)$")

// liveHandshake 用 daemon 同款客户端做 initialize + thread/list + 未知方法校准。
// 返回运行时自报的完整请求面(从校准探测的错误文案里解析;解析不出返回 nil)。
func liveHandshake(rep *report) []string {
	c, err := codexapp.Start()
	if err != nil {
		rep.sec("拉起 app-server 失败: %v", err)
		return nil
	}
	defer c.Close()

	// 顺手记录握手窗口内到达的任何服务端通知(0.156.1 可能在连接早期就推新通知)。
	var notifMu = make(chan string, 64)
	c.SetGlobalHandler(func(method string, params json.RawMessage) {
		select {
		case notifMu <- fmt.Sprintf("%s %s", method, clipStr(string(params), 200)):
		default:
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if _, err := c.CallCtx(ctx, "initialize", map[string]interface{}{
		"clientInfo": map[string]string{"name": "codexmetaprobe", "version": "0.1"},
	}); err != nil {
		rep.sec("initialize 失败: %v", err)
		return nil
	}
	rep.sec("initialize: 成功(0.156.1 接受 daemon 同款客户端握手)")

	raw, err := c.CallCtx(ctx, "thread/list", map[string]interface{}{"limit": 1, "archived": false})
	if err != nil {
		rep.sec("thread/list 失败: %v", err)
	} else {
		rep.sec("thread/list: 成功,应答头 240 字节: %s", clipStr(string(raw), 240))
	}

	// 校准探测:一个**故意不存在**的方法。据此确认"方法不存在"在 0.156.1 的
	// 错误通道形状(错误码/文案),为二进制静态扫描的结论提供运行时对照;
	// 错误文案里的 "expected one of `...`" 是 0.156.1 **自报的完整请求面**,
	// 是 A 级证据里含金量最高的一份。
	runtimeReqs := []string{}
	_, nerr := c.CallCtx(ctx, "thread/__pa_probe_nonexistent", map[string]interface{}{})
	rep.sec("校准探测 thread/__pa_probe_nonexistent → error: %v(证据分级 A:未知方法的运行时形状)", nerr)
	if nerr != nil {
		if m := runtimeEnumRe.FindStringSubmatch(nerr.Error()); m != nil {
			for _, tok := range strings.Split(m[1], ", ") {
				tok = strings.Trim(strings.TrimSpace(tok), "`")
				if tok != "" {
					runtimeReqs = append(runtimeReqs, tok)
				}
			}
			rep.sec("从错误文案解析出运行时自报请求面: %d 个方法(全表见第四节 diff)", len(runtimeReqs))
		} else {
			rep.sec("错误文案里没解析出 expected-one-of 枚举,请求面 diff 退化为仅 B 级")
		}
	}

	// thread/revert 参数形状探测(零副作用):空参数必然被参数校验拒绝,
	// 拒绝文案会**自报缺哪些字段** —— 0.156.1 用它顶替了消失的 thread/rollback,
	// daemon 侧后续迁移(RollbackThread)需要知道它的参数形状。
	if rvRaw, rverr := c.CallCtx(ctx, "thread/revert", map[string]interface{}{}); rverr != nil {
		rep.sec("thread/revert 空参数探测 → error: %v(证据分级 A:参数校验自报字段形状;零副作用)", rverr)
	} else {
		rep.sec("thread/revert 空参数探测 → 竟然成功: %s(若如此,记录形状供判读)",
			clipStr(string(rvRaw), 200))
	}
	// 第二跳:补上 threadId(全零 uuid,必不存在的线程)再探,让校验报出下一个
	// 必填字段 —— 为 RollbackThread 的迁移方案/thread-revert 语义提供形状证据。
	// 线程不存在只会得到 not found 类错误,无副作用。
	if _, rverr2 := c.CallCtx(ctx, "thread/revert", map[string]interface{}{
		"threadId": "00000000-0000-0000-0000-000000000000",
	}); rverr2 != nil {
		rep.sec("thread/revert {threadId:全零uuid} 探测 → error: %v(证据分级 A:下一层字段形状)", rverr2)
	} else {
		rep.sec("thread/revert {threadId:全零uuid} 探测 → 竟然成功(异常,需人工判读)")
	}

	close(notifMu)
	var got []string
	for n := range notifMu {
		got = append(got, n)
	}
	if len(got) == 0 {
		rep.sec("握手窗口内未收到任何服务端通知")
	} else {
		rep.sec("握手窗口内收到的服务端通知:")
		for _, n := range got {
			rep.sec("  <noti> %s", n)
		}
	}
	return runtimeReqs
}

// resolveNativeExe 把 codex 可执行定位结果解析成**原生** exe(字节扫描对象)。
// npm 包装器(codex.cmd)不是原生二进制,真正的协议方法串在
// node_modules/@openai/codex-*/vendor/*/bin/codex.exe 里。
func resolveNativeExe(flagBin string) (string, error) {
	if flagBin != "" {
		if _, err := os.Stat(flagBin); err != nil {
			return "", err
		}
		return flagBin, nil
	}
	base, err := codexapp.LocateCodex()
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(filepath.Ext(base), ".cmd") && !strings.EqualFold(filepath.Ext(base), ".bat") {
		return base, nil // 已是原生 exe
	}
	// 从包装器目录按 npm 布局解析 vendor 下的原生 exe。
	dir := filepath.Dir(base)
	patterns := []string{
		filepath.Join(dir, "node_modules", "@openai", "codex", "node_modules", "@openai", "codex-win32-*", "vendor", "*", "bin", "codex.exe"),
		filepath.Join(dir, "node_modules", "@openai", "codex-win32-*", "vendor", "*", "bin", "codex.exe"),
	}
	for _, p := range patterns {
		hits, _ := filepath.Glob(p)
		if len(hits) > 0 {
			return hits[0], nil
		}
	}
	return "", fmt.Errorf("无法从包装器 %s 解析原生 codex.exe(可用 -bin 直接指定)", base)
}

func clipStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func defaultSchemaPath() string {
	// 通用派生:从当前目录逐级向上找仓库相对的 vendored schema(在任何 cwd
	// 跑都能定位到所属仓库);找不到回落仓库相对路径,由 -schema 显式覆盖。
	// 不写死任何个人专属绝对路径。
	rel := filepath.Join("codex-schema", "codex_app_server_protocol.v2.schemas.json")
	dir, err := os.Getwd()
	if err != nil {
		return rel
	}
	for {
		p := filepath.Join(dir, rel)
		if _, err := os.Stat(p); err == nil {
			return p
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return rel
}
