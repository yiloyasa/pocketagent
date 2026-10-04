// T-SUBVIS 探针:codex 0.155.1 collab spawnAgent 型子代理的**通道真值捕获**。
//
// 【要回答的三个问题】(任务书 §排查任务 1)
//  1. 子代理线程是否以 thread/started 通知出现?带不带 parentThreadId?
//  2. collabAgentToolCall item 的 agentThreadId/receiverThreadIds 是什么?
//  3. 子代理的对话内容从哪条通道来(thread/updated? item/*? 还是只能
//     thread/read 拉全量)?
//
// 【做法】独占 app-server,逐帧记录该轮全部通知(只计数 delta),轮次结束后
// thread/list 扫描 parentThreadId 非空的子线程,再对每个子线程 thread/read
// (includeTurns=true)拉全量,验证"真线程捷径"是否成立。原始帧全量落证据
// 文件,成败如实,不许美化。
//
// 【红线】不碰 ~/.codex/config.toml(线程级参数只走 thread/start 覆盖,S0 先例);
// 审批类反向请求一律 accept 是探针场景自担(S0 同款),不代表 daemon 会替用户决策。
package main

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
var frames []string
var methodCount = map[string]int{}

func ts() string { return time.Now().Format("15:04:05.000") }

func clipStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func addf(format string, a ...any) {
	mu.Lock()
	frames = append(frames, fmt.Sprintf(format, a...))
	mu.Unlock()
}

func main() {
	cwd := flag.String("cwd", "", "thread/start 的工作区(缺省当前工作目录)")
	out := flag.String("out", "", "证据文件输出路径(缺省写到系统临时目录)")
	wait := flag.Duration("wait", 7*time.Minute, "单轮等待 turn/completed 的上限")
	postwait := flag.Duration("postwait", 40*time.Second, "turn/completed 后继续收尾帧的窗口(子代理收尾通知可能滞后)")
	prompt := flag.String("prompt",
		`请用 collab 工具的 spawnAgent 派发一个名为 worker 的子代理。任务给它:列出 daemon/internal 目录下的子目录名,并各用一句话说明每个目录的用途(只读调研,不要修改任何文件)。派发后先告诉我"已派发";等子代理完成后,把它的结论完整汇报给我。`,
		"诱导 spawnAgent 的 prompt")
	flag.Parse()

	// 缺省派生:cwd 缺省取当前工作目录(工作区语义),out 缺省写系统临时目录
	// (通用派生,不写死任何个人专属绝对路径 —— 字符串常量会随探针二进制
	// 分发,规格 §2 明令禁止)。取不到 cwd 是人话错误,由 -cwd 显式解困。
	if *cwd == "" {
		d, err := os.Getwd()
		if err != nil {
			log.Fatalf("获取当前工作目录失败(可用 -cwd 显式指定): %v", err)
		}
		*cwd = d
	}
	if *out == "" {
		*out = filepath.Join(os.TempDir(), "codex-subagent-spawn-capture.md")
	}

	deadline := time.Now().Add(*wait + *postwait + 3*time.Minute)

	c, err := codexapp.Start()
	if err != nil {
		log.Fatal("spawn app-server:", err)
	}
	defer c.Close()

	c.SetTurnHandlers(func(method string, params json.RawMessage) {
		mu.Lock()
		methodCount[method]++
		p := string(params)
		interesting := method == "thread/started" || strings.HasPrefix(method, "item/") ||
			method == "turn/started" || method == "turn/completed" ||
			strings.Contains(p, `"collabAgentToolCall"`) || strings.Contains(p, `"subAgentActivity"`) ||
			strings.Contains(p, "parentThreadId") && !strings.Contains(p, `"parentThreadId":null`)
		if strings.Contains(method, "delta") && !interesting {
			mu.Unlock()
			return // delta 只计数
		}
		if interesting {
			frames = append(frames, fmt.Sprintf("[%s] **%s** %s", ts(), method, clipStr(p, 9000)))
		} else {
			frames = append(frames, fmt.Sprintf("[%s] %s %s", ts(), method, clipStr(p, 300)))
		}
		mu.Unlock()
	}, func(id int64, method string, params json.RawMessage) {
		addf("[%s] REQ(id=%d) %s %s", ts(), id, method, clipStr(string(params), 600))
		// 审批类反向请求一律 accept:探针场景自担(S0 同款惯例),让轮次能走完。
		_ = c.Respond(id, map[string]string{"decision": "accept"})
	})

	if _, err := c.Call("initialize", map[string]interface{}{"clientInfo": map[string]string{"name": "codexspawnprobe", "version": "0.1"}}); err != nil {
		log.Fatal("initialize:", err)
	}

	// 只读调研任务:read-only 沙箱足够;approvalPolicy=never 避免审批往返
	// (线程级覆盖,不碰 config.toml)。
	raw, err := c.Call("thread/start", map[string]interface{}{
		"cwd":            *cwd,
		"sandbox":        "read-only",
		"approvalPolicy": "never",
	})
	if err != nil {
		log.Fatalf("thread/start: %v\n原始应答: %s", err, clipStr(string(raw), 400))
	}
	var tr struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	_ = json.Unmarshal(raw, &tr)
	tid := tr.Thread.ID
	addf("[%s] thread/start -> %s (sandbox=read-only, approvalPolicy=never)", ts(), tid)
	fmt.Println("== thread/start ->", tid)

	turnDone := false
	turnStart := time.Now()
	if _, err := c.Call("turn/start", map[string]interface{}{
		"threadId": tid,
		"input":    []map[string]string{{"type": "text", "text": *prompt}},
	}); err != nil {
		addf("[%s] turn/start 错误: %v", ts(), err)
	} else {
		for time.Now().Before(deadline) {
			time.Sleep(500 * time.Millisecond)
			mu.Lock()
			for i := len(frames) - 1; i >= 0 && i >= len(frames)-5; i-- {
				if strings.Contains(frames[i], "] turn/completed") {
					turnDone = true
				}
			}
			mu.Unlock()
			if turnDone {
				break
			}
		}
	}
	elapsed := time.Since(turnStart).Round(time.Second)
	fmt.Printf("== turn/completed=%v 耗时=%s,postwait=%s 收尾帧 ==\n", turnDone, elapsed, *postwait)
	time.Sleep(*postwait)

	// ---- thread/list:扫描 parentThreadId 非空的子线程 ----
	listRaw, lerr := c.Call("thread/list", map[string]interface{}{"archived": false, "limit": 200})
	var listOut strings.Builder
	childIDs := []string{}
	if lerr != nil {
		listOut.WriteString("thread/list 失败: " + lerr.Error() + "\n")
		addf("[%s] thread/list 失败: %v", ts(), lerr)
	} else {
		var res struct {
			Data []struct {
				ID             string  `json:"id"`
				Name           *string `json:"name"`
				Preview        string  `json:"preview"`
				ParentThreadId *string `json:"parentThreadId"`
				AgentNickname  *string `json:"agentNickname"`
				AgentRole      *string `json:"agentRole"`
				ThreadSource   *string `json:"threadSource"`
				Source         any     `json:"source"`
				Status         struct {
					Type string `json:"type"`
				} `json:"status"`
				Cwd       string `json:"cwd"`
				Path      string `json:"path"`
				UpdatedAt int64  `json:"updatedAt"`
			} `json:"data"`
		}
		if json.Unmarshal(listRaw, &res) != nil {
			listOut.WriteString("thread/list 应答无法解析: " + clipStr(string(listRaw), 600) + "\n")
		} else {
			fmt.Fprintf(&listOut, "共 %d 条线程:\n", len(res.Data))
			for _, it := range res.Data {
				mark := ""
				if it.ParentThreadId != nil && *it.ParentThreadId != "" {
					mark = " **子线程**"
					childIDs = append(childIDs, it.ID)
				}
				fmt.Fprintf(&listOut, "- id=%s parent=%v nickname=%v role=%v threadSource=%v status=%s name=%v preview=%q%s\n",
					it.ID, deref(it.ParentThreadId), deref(it.AgentNickname), deref(it.AgentRole),
					deref(it.ThreadSource), it.Status.Type, deref(it.Name), clipStr(it.Preview, 80), mark)
			}
		}
	}

	// ---- 对每个子线程 thread/read 拉全量 ----
	readOut := strings.Builder{}
	for _, cid := range childIDs {
		fmt.Println("== thread/read ->", cid)
		rr, rerr := c.Call("thread/read", map[string]interface{}{"threadId": cid, "includeTurns": true})
		readOut.WriteString(fmt.Sprintf("\n### 子线程 %s\n\n", cid))
		if rerr != nil {
			readOut.WriteString("thread/read 失败: " + rerr.Error() + "\n")
			continue
		}
		readOut.WriteString(summarizeRead(rr))
	}

	// ---- 汇总落盘 ----
	mu.Lock()
	localFrames := append([]string(nil), frames...)
	counts := map[string]int{}
	for m, n := range methodCount {
		counts[m] = n
	}
	mu.Unlock()

	var b strings.Builder
	b.WriteString("# codex collab spawnAgent 子代理通道真值捕获(T-SUBVIS)\n\n")
	b.WriteString("- 时间: " + time.Now().Format("2006-01-02 15:04:05") + "\n")
	b.WriteString(fmt.Sprintf("- 参数: thread/start { cwd: %s, sandbox: \"read-only\", approvalPolicy: \"never\" }\n", *cwd))
	b.WriteString("- prompt: " + *prompt + "\n")
	b.WriteString(fmt.Sprintf("- 结果: turn/completed %v(耗时 %s);thread/list 子线程 %d 个\n", turnDone, elapsed, len(childIDs)))
	b.WriteString("\n## 通知方法计数(delta 只计数不落帧)\n\n")
	keys := make([]string, 0, len(counts))
	for m := range counts {
		keys = append(keys, m)
	}
	sort.Strings(keys)
	for _, m := range keys {
		fmt.Fprintf(&b, "- `%s` x%d\n", m, counts[m])
	}
	b.WriteString("\n## thread/list 快照(节选,含 parentThreadId)\n\n```\n" + listOut.String() + "```\n")
	b.WriteString("\n## 子线程 thread/read 全量摘要\n" + readOut.String())
	b.WriteString("\n## 原始帧(按到达顺序)\n\n```json\n")
	for _, f := range localFrames {
		b.WriteString(f + "\n")
	}
	b.WriteString("```\n")
	if err := os.WriteFile(*out, []byte(b.String()), 0o644); err != nil {
		log.Fatalf("写证据文件 %s: %v", *out, err)
	}
	fmt.Printf("== 完成: 子线程=%d 证据=%s ==\n", len(childIDs), *out)
}

func deref(s *string) string {
	if s == nil {
		return "<null>"
	}
	if *s == "" {
		return "<空串>"
	}
	return *s
}

// summarizeRead 把 thread/read 应答翻成逐 turn 摘要:每个 item 取类型+关键
// 字段,文本类带正文(clip),协作/活动类带全量关键字段。原始正文证据以
// 原始帧区为准,这里保证"子对话内容可不可达"一眼可见。
func summarizeRead(raw json.RawMessage) string {
	var res struct {
		Thread struct {
			ID             string  `json:"id"`
			ParentThreadId *string `json:"parentThreadId"`
			AgentNickname  *string `json:"agentNickname"`
			Preview        string  `json:"preview"`
			Path           string  `json:"path"`
			Turns          []struct {
				ID     string `json:"id"`
				Status string `json:"status"`
				Items  []json.RawMessage
			} `json:"turns"`
		} `json:"thread"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return "thread/read 应答无法解析: " + clipStr(string(raw), 600) + "\n"
	}
	t := res.Thread
	var b strings.Builder
	fmt.Fprintf(&b, "- parentThreadId=%v agentNickname=%v preview=%q\n", deref(t.ParentThreadId), deref(t.AgentNickname), clipStr(t.Preview, 120))
	fmt.Fprintf(&b, "- rollout 落盘: %s\n", t.Path)
	fmt.Fprintf(&b, "- turns: %d 个\n", len(t.Turns))
	for _, turn := range t.Turns {
		fmt.Fprintf(&b, "  - turn %s(status=%s) items=%d\n", clipStr(turn.ID, 18), turn.Status, len(turn.Items))
		for _, it := range turn.Items {
			b.WriteString("    - " + summarizeItem(it) + "\n")
		}
	}
	return b.String()
}

func summarizeItem(it json.RawMessage) string {
	var head struct {
		Type string `json:"type"`
	}
	_ = json.Unmarshal(it, &head)
	switch head.Type {
	case "agentMessage":
		var s struct {
			Text string `json:"text"`
		}
		_ = json.Unmarshal(it, &s)
		return fmt.Sprintf("agentMessage: %q", clipStr(s.Text, 500))
	case "userMessage":
		var s struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		}
		_ = json.Unmarshal(it, &s)
		txts := make([]string, 0, len(s.Content))
		for _, c := range s.Content {
			txts = append(txts, c.Text)
		}
		return fmt.Sprintf("userMessage: %q", clipStr(strings.Join(txts, "\\n"), 300))
	case "collabAgentToolCall":
		return "collabAgentToolCall: " + clipStr(string(it), 1200)
	case "subAgentActivity":
		return "subAgentActivity: " + clipStr(string(it), 400)
	case "reasoning":
		return "reasoning(摘要省略)"
	case "commandExecution":
		var s struct {
			Command          string `json:"command"`
			Status           string `json:"status"`
			ExitCode         *int   `json:"exitCode"`
			AggregatedOutput string `json:"aggregatedOutput"`
		}
		_ = json.Unmarshal(it, &s)
		return fmt.Sprintf("commandExecution: %q status=%s exit=%v out=%q", clipStr(s.Command, 160), s.Status, derefI(s.ExitCode), clipStr(s.AggregatedOutput, 200))
	default:
		return head.Type + ": " + clipStr(string(it), 200)
	}
}

func derefI(p *int) string {
	if p == nil {
		return "<null>"
	}
	return fmt.Sprintf("%d", *p)
}
