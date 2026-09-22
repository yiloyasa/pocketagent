package main

// T-OPT 探针:定位「Default 模式下 codex 调 request_user_input 被拒」的拒绝方。
//
// 背景(真机截图 2026-09-21 16:41):codex 自述"选项卡组件在当前 Default 模式下
// 不可用,调用被直接拒了,只有切到 Plan 模式才能弹出真正的选项卡片"。
// daemon 侧 codex_requests.go 对 item/tool/requestUserInput 的 reqAskUser 分派
// 与应答形状({"answers":{qid:{"answers":[...]}}})均已存在,所以嫌疑落在
// codex 本体的模式门控上。二进制字符串已见三条硬证据:
//
//	"request_user_input is unavailable in .. mode"(Plan/Default 枚举相邻)
//	实验特性名单含 default_mode_request_user_input
//	ConfigToml [tools] 含 experimental_request_user_input 键
//
// 本探针用真帧裁决,三个场景按序跑(从无副作用到有副作用):
//
//	A 基线      :thread/start 正常参数(Default 模式)→ 诱导 prompt → 逐帧记录
//	C 线程级覆盖:thread/start config={"tools":{"experimental_request_user_input":true}}
//	              (与 daemon 既有的 ConfigOverrideFor 同通道,不碰用户 config.toml)
//	B 进程级开关:experimentalFeature/enablement/set {"default_mode_request_user_input":true}
//	              (schema: Process-wide runtime feature enablement,同样不落盘)
//
// B 放最后是因为它是进程级生效,开了会影响后续场景的归因。
// 红线遵守:不改用户 ~/.codex/config.toml;不伪造 decision(反向请求里的
// requestUserInput 用其第一个选项的真实文本作答——探针自问自答,如实记录;
// 其余未知反向请求回 JSON-RPC error,绝不回 decision)。
import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pocketagent/daemon/internal/codexapp"
)

var mu sync.Mutex
var frames []string // 每条 [时间] 场景 METHOD 原始参数
var methodCount = map[string]int{}

// turnResult 一轮诱导的结果。
type turnResult struct {
	completed     bool     // 是否等到 turn/completed
	askSeen       int      // 收到 item/tool/requestUserInput 的次数
	agentMessages []string // codex 的文本回复(轮末自述)
}

func main() {
	out := flag.String("out", `G:\项目1\pocketagent\docs\verify\evidence\codex-askuser-t-opt.md`, "证据文件输出路径")
	wait := flag.Duration("wait", 120*time.Second, "单轮等待 turn/completed 的上限(探针超时保护)")
	cwd := flag.String("cwd", "", "thread/start 的工作区(默认临时目录,不污染项目)")
	flag.Parse()

	work := *cwd
	if work == "" {
		work = filepath.Join(os.TempDir(), "pa-askprobe-work")
		_ = os.MkdirAll(work, 0o755)
	}

	c, err := codexapp.Start()
	if err != nil {
		log.Fatal("spawn app-server:", err)
	}
	defer c.Close()

	c.SetGlobalHandler(func(method string, params json.RawMessage) { record("GLOBAL", method, params) })
	c.SetTurnHandlers(
		func(method string, params json.RawMessage) { record("NOTIF", method, params) },
		nil, // req 稍后按场景装
	)

	if _, err := c.Call("initialize", map[string]interface{}{"clientInfo": map[string]string{"name": "codexaskprobe", "version": "0.1"}}); err != nil {
		log.Fatal("initialize:", err)
	}

	var b strings.Builder
	b.WriteString("# T-OPT: Default 模式 request_user_input 被拒 —— 拒绝方定位探针\n\n")
	b.WriteString("- 时间: " + time.Now().Format("2006-01-02 15:04:05") + "\n")
	b.WriteString("- codex: 0.155.1(CODEX_BIN=G:\\Claude Code\\codex.cmd)\n")
	b.WriteString("- 工作区: " + work + "\n")
	b.WriteString("- 诱导 prompt: \"请调用 request_user_input 工具问我一个问题…(全文见帧)\"\n")
	b.WriteString("- 红线: 未改用户 ~/.codex/config.toml;探针对 requestUserInput 用第一选项的真实文本作答并如实记录,其余反向请求回 error,未伪造任何 decision\n\n")

	// ---- 第 0 步:实验特性清单(能力注册表真值) ----
	fmt.Println("== experimentalFeature/list ==")
	listFeatures := func(tag string) {
		if raw, err := c.Call("experimentalFeature/list", map[string]interface{}{}); err != nil {
			frames = append(frames, fmt.Sprintf("[%s] %s experimentalFeature/list 错误: %v", ts(), tag, err))
			fmt.Println("experimentalFeature/list 失败:", err)
			return
		} else {
			var resp struct {
				Data []struct {
					Name           string `json:"name"`
					DefaultEnabled bool   `json:"defaultEnabled"`
					Enabled        bool   `json:"enabled"`
					Stage          string `json:"stage"`
				} `json:"data"`
			}
			_ = json.Unmarshal(raw, &resp)
			for _, f := range resp.Data {
				if strings.Contains(f.Name, "user_input") {
					fmt.Fprintf(&b, "- [%s] 特性 `%s`: defaultEnabled=%v enabled=%v stage=%s\n", tag, f.Name, f.DefaultEnabled, f.Enabled, f.Stage)
					fmt.Printf("  [%s] 特性 %s: default=%v enabled=%v stage=%s\n", tag, f.Name, f.DefaultEnabled, f.Enabled, f.Stage)
				}
			}
		}
	}
	listFeatures("初始")

	induce := "请调用 request_user_input 工具向我提一个选择题:问题=\"PocketAgent 的主力原生 agent 是谁?\",选项 A=\"codex\"、B=\"pi\"。必须直接调用该工具,不要用普通文本提问。"

	// ---- 场景 A:基线(Default 模式,无任何解锁参数) ----
	resA := runScenario(c, "A-基线", map[string]interface{}{"cwd": work}, induce, *wait)

	// ---- 场景 B:进程级实验特性开关(default_mode_request_user_input) ----
	// schema: Process-wide runtime feature enablement。开了之后立刻复核注册表,
	// 再开新线程验证 —— 回避"开了但没生效"的假阴性归因。
	fmt.Println("== experimentalFeature/enablement/set default_mode_request_user_input=true ==")
	raw, err := c.Call("experimentalFeature/enablement/set", map[string]interface{}{
		"enablement": map[string]interface{}{"default_mode_request_user_input": true},
	})
	if err != nil {
		frames = append(frames, fmt.Sprintf("[%s] SCENARIO=B enablement/set 错误: %v raw=%s", ts(), err, clip(string(raw), 300)))
		fmt.Println("enablement/set 失败:", err)
	}
	listFeatures("set后")
	resB := turnResult{}
	if err == nil {
		resB = runScenario(c, "B-特性开关", map[string]interface{}{"cwd": work}, induce, *wait)
	}

	// ---- 场景 C1:线程级 config 覆盖,结构体形态 ----
	// 首轮试探(boolean)被 codex 拒:-32600 "expected struct ExperimentalRequestUserInput";
	// 遥测键 tools.experimental_request_user_input.enabled 指向字段名 enabled。
	resC := runScenario(c, "C-config覆盖", map[string]interface{}{
		"cwd":    work,
		"config": map[string]interface{}{"tools": map[string]interface{}{"experimental_request_user_input": map[string]interface{}{"enabled": true}}},
	}, induce, *wait)

	// ---- 场景 D:config 的 [features] 表线程级覆盖 ----
	resD := runScenario(c, "D-features覆盖", map[string]interface{}{
		"cwd":    work,
		"config": map[string]interface{}{"features": map[string]interface{}{"default_mode_request_user_input": true}},
	}, induce, *wait)

	// ---- 汇总 ----
	b.WriteString("## 场景结果汇总\n\n")
	b.WriteString("| 场景 | thread/start 参数 | 收到 requestUserInput | codex 轮末自述(截断) |\n|---|---|---|---|\n")
	writeRow := func(name, param string, r turnResult) {
		msg := "—"
		if len(r.agentMessages) > 0 {
			msg = clip(strings.Join(r.agentMessages, " / "), 200)
		}
		fmt.Fprintf(&b, "| %s | %s | %d 次 | %s |\n", name, param, r.askSeen, msg)
	}
	writeRow("A 基线(Default 模式)", "cwd", resA)
	if err == nil {
		writeRow("B 特性开关(enablement/set)", "set 后 cwd", resB)
	} else {
		b.WriteString("| B 特性开关(enablement/set) | 调用失败 | — | — |\n")
	}
	writeRow("C config 覆盖(结构体)", "cwd + config.tools.experimental_request_user_input={enabled:true}", resC)
	writeRow("D features 覆盖", "cwd + config.features.default_mode_request_user_input=true", resD)
	b.WriteString("\n**判定依据**: 场景 A 若 codex 全程未发出 item/tool/requestUserInput 且 daemon 侧无 -32601,则拒绝方=codex 本体(模式门控);任一解锁场景 codex 发出 requestUserInput 且收到探针应答后继续执行,即解锁路径成立。\n\n")

	b.WriteString("## 通知方法计数(全程)\n\n")
	keys := make([]string, 0, len(methodCount))
	for m := range methodCount {
		keys = append(keys, m)
	}
	sort.Strings(keys)
	for _, m := range keys {
		fmt.Fprintf(&b, "- `%s` x%d\n", m, methodCount[m])
	}
	b.WriteString("\n## 原始帧(按到达顺序)\n\n```json\n")
	mu.Lock()
	localFrames := append([]string(nil), frames...)
	mu.Unlock()
	for _, f := range localFrames {
		b.WriteString(f + "\n")
	}
	b.WriteString("```\n")
	if werr := os.WriteFile(*out, []byte(b.String()), 0o644); werr != nil {
		log.Fatalf("写证据文件 %s: %v", *out, werr)
	}
	fmt.Printf("== 完成: A(ask=%d) C(ask=%d) B(ask=%d) 证据=%s ==\n", resA.askSeen, resC.askSeen, resB.askSeen, *out)
}

// runScenario 起一个新线程跑一轮诱导,记录逐帧与结果。startParams 直接透传 thread/start。
func runScenario(c *codexapp.Client, name string, startParams map[string]interface{}, induce string, wait time.Duration) turnResult {
	fmt.Printf("== 场景 %s: thread/start %v ==\n", name, startParams)
	res := turnResult{}
	var askMu sync.Mutex

	c.SetTurnHandlers(
		func(method string, params json.RawMessage) {
			record(name, method, params)
			if method == "item/completed" && strings.Contains(string(params), `"agentMessage"`) {
				var it struct {
					Item struct {
						Text string `json:"text"`
					} `json:"item"`
				}
				_ = json.Unmarshal(params, &it)
				if strings.TrimSpace(it.Item.Text) != "" {
					askMu.Lock()
					res.agentMessages = append(res.agentMessages, it.Item.Text)
					askMu.Unlock()
				}
			}
		},
		func(id int64, method string, params json.RawMessage) {
			record(name, "REQ/"+method, params)
			if method == "item/tool/requestUserInput" {
				var a struct {
					Questions []struct {
						Id      string `json:"id"`
						Header  string `json:"header"`
						Options []struct {
							Label string `json:"label"`
						} `json:"options"`
					} `json:"questions"`
				}
				_ = json.Unmarshal(params, &a)
				ans := map[string]interface{}{}
				for _, q := range a.Questions {
					v := "A" // 无选项时的兜底文本(如实作答,非 decision)
					if len(q.Options) > 0 {
						v = q.Options[0].Label // 取第一选项的真实文本
					}
					ans[q.Id] = map[string]interface{}{"answers": []string{v}}
				}
				payload := map[string]interface{}{"answers": ans}
				record(name, "RESP/"+method, mustJSON(payload))
				_ = c.Respond(id, payload)
				askMu.Lock()
				res.askSeen++
				askMu.Unlock()
				return
			}
			// 其余反向请求(审批等,本探针场景不应出现):如实回 error,绝不伪造 decision。
			record(name, "RESP-ERR/"+method, json.RawMessage(`"探针无此能力,回 error"`))
			_ = c.RespondError(id, -32601, "codexaskprobe 不支持该反向请求")
		},
	)
	defer c.SetTurnHandlers(nil, nil)

	raw, err := c.Call("thread/start", startParams)
	if err != nil {
		frames = append(frames, fmt.Sprintf("[%s] SCENARIO=%s thread/start 错误: %v raw=%s", ts(), name, err, clip(string(raw), 400)))
		fmt.Println("thread/start 失败:", err)
		return res
	}
	var tr struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	_ = json.Unmarshal(raw, &tr)
	frames = append(frames, fmt.Sprintf("[%s] SCENARIO=%s thread/start -> %s", ts(), name, tr.Thread.ID))

	if _, err := c.Call("turn/start", map[string]interface{}{
		"threadId": tr.Thread.ID,
		"input":    []map[string]string{{"type": "text", "text": induce}},
	}); err != nil {
		frames = append(frames, fmt.Sprintf("[%s] SCENARIO=%s turn/start 错误: %v", ts(), name, err))
		return res
	}

	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)
		askMu.Lock()
		done := res.completed
		askMu.Unlock()
		if done {
			break
		}
		mu.Lock()
		for i := len(frames) - 1; i >= 0 && i >= len(frames)-40; i-- {
			if strings.Contains(frames[i], "] "+name+" turn/completed") {
				askMu.Lock()
				res.completed = true
				askMu.Unlock()
				break
			}
		}
		mu.Unlock()
		if res.completed {
			break
		}
	}
	frames = append(frames, fmt.Sprintf("[%s] SCENARIO=%s 结果: completed=%v askSeen=%d", ts(), name, res.completed, res.askSeen))
	return res
}

// record 落一帧。delta 类只计数;requestUserInput/REQ/turn 生命周期全量落盘,其余截断。
func record(scene, method string, params json.RawMessage) {
	mu.Lock()
	defer mu.Unlock()
	methodCount[method]++
	s := string(params)
	if strings.Contains(method, "delta") {
		return
	}
	interesting := method == "turn/completed" || method == "turn/started" || method == "error" ||
		strings.HasPrefix(method, "REQ") || strings.HasPrefix(method, "RESP") ||
		strings.Contains(s, `"requestUserInput"`) || strings.Contains(s, `"userInput"`) ||
		method == "thread/started"
	if interesting {
		frames = append(frames, fmt.Sprintf("[%s] %s %s %s", ts(), scene, method, clip(s, 4000)))
	} else {
		frames = append(frames, fmt.Sprintf("[%s] %s %s %s", ts(), scene, method, clip(s, 200)))
	}
}

func mustJSON(v interface{}) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func ts() string { return time.Now().Format("15:04:05.000") }

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
