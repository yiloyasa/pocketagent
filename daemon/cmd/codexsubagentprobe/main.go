package main

// S0 探针:验证 codex 0.149.1 审批升级审查子代理的**实际发射行为**——
// thread/start 带 approvalsReviewer:"auto_review" 后,触发一次需要审批升级的
// 命令执行,监听 item/started|item/completed 里的 subAgentActivity 与
// collabAgentToolCall 两类 ThreadItem(出现次数、形状、节奏),原始帧落证据文件。
// 这是 0.149.1 真值的首次采集:成败都如实记录,不许美化。
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
var frames []string // 每条 [时间] METHOD 原始参数
var methodCount = map[string]int{}

func main() {
	cwd := flag.String("cwd", "", "thread/start 的工作区(缺省 = 当前工作目录)")
	out := flag.String("out", "", "证据文件输出路径(缺省写到系统临时目录)")
	wait := flag.Duration("wait", 120*time.Second, "单轮等待 turn/completed 的上限")
	prompt := flag.String("prompt", "", "会触发审批升级的 prompt(写工作区外 + 联网);缺省写系统临时目录下的探针文件")
	flag.Parse()

	// 落点全部运行时派生(规格 §2:字符串常量会随探针二进制分发,不写死任何
	// 个人专属绝对路径;开发机自测用 -cwd/-out/-prompt 覆盖)。
	work := *cwd
	if work == "" {
		if w, err := os.Getwd(); err == nil && w != "" {
			work = w
		} else {
			work, _ = os.MkdirTemp("", "codexsubagentprobe-ws")
		}
	}
	reportPath := *out
	if reportPath == "" {
		reportPath = filepath.Join(os.TempDir(), "codex-subagent-s0.md")
	}
	s0Prompt := *prompt
	if s0Prompt == "" {
		// 语义保持:写工作区之外的文件 + 联网,才会触发审批升级(auto review)。
		// 落点用系统临时目录,不含任何个人专属盘符/目录。
		s0Prompt = fmt.Sprintf(
			"运行命令:curl.exe -s -o %s https://example.com 然后告诉我退出码。该操作需要经过审查(auto review)",
			filepath.Join(os.TempDir(), "pa-s0-probe-out.txt"))
	}

	c, err := codexapp.Start()
	if err != nil {
		log.Fatal("spawn app-server:", err)
	}
	defer c.Close()

	subagentItems := 0 // subAgentActivity + collabAgentToolCall 计数
	c.NotifHandler = func(method string, params json.RawMessage) {
		mu.Lock()
		methodCount[method]++
		isDelta := strings.Contains(method, "delta")
		if isDelta {
			mu.Unlock()
			return // delta 太吵,只计数不落帧
		}
		s := string(params)
		// 关键帧全量落盘:子代理 item、审批审查(autoApprovalReview/guardian)、
		// 反向请求、线程信息(含 parentThreadId,L2 伏笔);其余帧截断。
		interesting := method == "item/started" || method == "item/completed" ||
			strings.HasPrefix(method, "item/autoApprovalReview") || strings.HasPrefix(method, "item/guardian") ||
			method == "guardianWarning" || method == "thread/started" || strings.HasPrefix(method, "REQ")
		if strings.Contains(s, `"subAgentActivity"`) || strings.Contains(s, `"collabAgentToolCall"`) {
			subagentItems++
			frames = append(frames, fmt.Sprintf("[%s] **%s**(子代理item x%d) %s", ts(), method, subagentItems, s))
		} else if interesting {
			frames = append(frames, fmt.Sprintf("[%s] %s %s", ts(), method, clipStr(s, 4000)))
		} else {
			frames = append(frames, fmt.Sprintf("[%s] %s %s", ts(), method, clipStr(s, 240)))
		}
		mu.Unlock()
	}
	c.ReqHandler = func(id json.RawMessage, method string, params json.RawMessage) {
		mu.Lock()
		frames = append(frames, fmt.Sprintf("[%s] REQ(id=%s) %s %s", ts(), string(id), method, clipStr(string(params), 300)))
		mu.Unlock()
		// 审批类反向请求一律 accept,让轮次能走完(我们不替用户决策,探针场景自担)。
		_ = c.Respond(id, map[string]string{"decision": "accept"})
	}

	if _, err := c.Call("initialize", map[string]interface{}{"clientInfo": map[string]string{"name": "codexsubagentprobe", "version": "0.1"}}); err != nil {
		log.Fatal("initialize:", err)
	}

	// 关键参数:approvalsReviewer:"auto_review" —— 不碰用户 config,只在
	// thread/start 上带。schema 说明审查者在"sandbox 逃逸/被拦的联网/MCP 审批"
	// 等审批请求上接管路由;默认沙箱放行的命令不会走审批,因此再把线程压成
	// read-only + on-request,保证升级动作真的发生。
	raw, err := c.Call("thread/start", map[string]interface{}{
		"cwd":               work,
		"approvalsReviewer": "auto_review",
		"approvalPolicy":    "on-request",
		"sandbox":           "read-only",
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
	mu.Lock()
	frames = append(frames, fmt.Sprintf("[%s] thread/start -> %s (approvalsReviewer=auto_review)", ts(), tid))
	mu.Unlock()
	fmt.Println("== thread/start ->", tid)

	started := time.Now()
	ok := runTurn(c, tid, s0Prompt, *wait)
	elapsed := time.Since(started).Round(time.Second)

	mu.Lock()
	nItems := subagentItems
	localFrames := append([]string(nil), frames...)
	counts := map[string]int{}
	for m, n := range methodCount {
		counts[m] = n
	}
	mu.Unlock()

	// 汇总 + 写证据文件(原始帧全量落盘,成败如实)。
	var b strings.Builder
	b.WriteString("# codex 子代理 S0 探针证据(T-L1)\n\n")
	b.WriteString("- 时间: " + time.Now().Format("2006-01-02 15:04:05") + "\n")
	b.WriteString(fmt.Sprintf("- 参数: thread/start { cwd: %s, approvalsReviewer: \"auto_review\", approvalPolicy: \"on-request\", sandbox: \"read-only\" }\n", work))
	b.WriteString("- prompt: " + s0Prompt + "\n")
	b.WriteString(fmt.Sprintf("- 结果: turn/completed %v(耗时 %s);subAgentActivity+collabAgentToolCall item 共 %d 条\n", ok, elapsed, nItems))
	b.WriteString("\n## 通知方法计数\n\n")
	keys := make([]string, 0, len(counts))
	for m := range counts {
		keys = append(keys, m)
	}
	sort.Strings(keys)
	for _, m := range keys {
		fmt.Fprintf(&b, "- `%s` x%d\n", m, counts[m])
	}
	b.WriteString("\n## 原始帧(按到达顺序)\n\n```json\n")
	for _, f := range localFrames {
		b.WriteString(f + "\n")
	}
	b.WriteString("```\n")
	if err := os.WriteFile(reportPath, []byte(b.String()), 0o644); err != nil {
		log.Fatalf("写证据文件 %s: %v", reportPath, err)
	}
	fmt.Printf("== 完成: turn/completed=%v 耗时=%s 子代理item=%d 证据=%s ==\n", ok, elapsed, nItems, reportPath)
}

func runTurn(c *codexapp.Client, tid, prompt string, d time.Duration) bool {
	mu.Lock()
	base := len(frames)
	mu.Unlock()
	if _, err := c.Call("turn/start", map[string]interface{}{
		"threadId": tid,
		"input":    []map[string]string{{"type": "text", "text": prompt}},
	}); err != nil {
		mu.Lock()
		frames = append(frames, fmt.Sprintf("[%s] turn/start 错误: %v", ts(), err))
		mu.Unlock()
		return false
	}
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)
		mu.Lock()
		for i := base; i < len(frames); i++ {
			if strings.Contains(frames[i], "] turn/completed") {
				mu.Unlock()
				return true
			}
		}
		mu.Unlock()
	}
	return false
}

func ts() string { return time.Now().Format("15:04:05.000") }

func clipStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
