// T-SUBVIS 探针:pi(pi-subagents 扩展)子代理的**通道真值捕获**。
//
// 【要回答的问题】pi 会话诱导派子代理后,`pi --mode rpc` 的 JSONL 流里,
// 子代理活动以什么事件形态出现(tool_execution? 扩展自定义事件? 纯文本?)
// —— 回答"pi 子代理的对话内容从哪条通道来、可不可达"。
//
// 【隔离】独立 agent 目录(PI_CODING_AGENT_DIR 注入),绝不写用户 ~/.pi/agent:
//   - settings.json 探针自写(packages 仅 pi-subagents);
//   - npm/ 用 Windows 目录 junction 指向用户的 ~/.pi/agent/npm(只读复用,
//     免 242MB 拷贝;junction 失败退回整目录拷贝);
//   - auth.json / models-store.json 复制进探针目录(模型鉴权必需;仅本机
//     同用户目录内复制,不外传、不改用户原件)。
//   - sessions 落探针目录,不污染用户会话历史。
//
// 【诚实边界】extension_ui_request 的 dialog 类一律回 {"cancelled":true}
// 如实拒绝 —— 探针不替用户做任何 approve 决定(与 daemon 红线一致)。
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pocketagent/daemon/internal/hidewin"
	"github.com/pocketagent/daemon/internal/piapp"
)

var mu sync.Mutex
var frames []string
var typeCount = map[string]int{}
var stdinMu sync.Mutex

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

func writeLine(stdin io.WriteCloser, obj map[string]any) error {
	raw, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	stdinMu.Lock()
	defer stdinMu.Unlock()
	_, err = stdin.Write(append(raw, '\n'))
	return err
}

func main() {
	home, _ := os.UserHomeDir()
	agentDir := flag.String("agentdir", filepath.Join(home, ".pocketagentd", "pi-subvis-probe"), "探针专用 pi agent 目录(隔离)")
	ws := flag.String("ws", filepath.Join(os.TempDir(), "pa-pi-subvis-ws"), "探针工作区")
	out := flag.String("out", "", "证据文件输出路径(缺省写到系统临时目录)")
	wait := flag.Duration("wait", 6*time.Minute, "等待 agent_settled 的上限")
	postwait := flag.Duration("postwait", 25*time.Second, "settled 后继续收帧的窗口")
	prompt := flag.String("prompt",
		`用 scout 子代理调研当前目录:列出有哪些文件,并各用一句话说明每个文件的内容。子代理完成后,把它的结论完整告诉我。`,
		"诱导 pi-subagents 的 prompt")
	flag.Parse()

	// 证据落点:缺省写系统临时目录(通用派生,不写死任何个人专属绝对路径
	// —— 字符串常量会随探针二进制分发,规格 §2 明令禁止)。
	if *out == "" {
		*out = filepath.Join(os.TempDir(), "pi-subagent-capture.md")
	}

	log.SetFlags(log.Ltime)

	// ---- 1) 隔离 agent 目录 ----
	if err := setupAgentDir(*agentDir, home); err != nil {
		log.Fatalf("隔离 agent 目录准备失败: %v", err)
	}

	// ---- 2) 工作区(3 个小文件,scout 有东西可查) ----
	if err := setupWorkspace(*ws); err != nil {
		log.Fatalf("工作区准备失败: %v", err)
	}

	// ---- 3) 拉起 pi --mode rpc ----
	node, cli, err := piapp.LocatePi()
	if err != nil {
		log.Fatalf("定位 pi 失败: %v", err)
	}
	cmd := exec.Command(node, cli, "--mode", "rpc")
	cmd.Dir = *ws
	cmd.Env = append(os.Environ(), "PI_CODING_AGENT_DIR="+*agentDir)
	hidewin.Apply(cmd)
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		log.Fatalf("启动 pi rpc 失败: %v", err)
	}
	addf("[%s] pi --mode rpc 已启动 PID=%d agentDir=%s ws=%s", ts(), cmd.Process.Pid, *agentDir, *ws)
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()
	go func() {
		sc := bufio.NewScanner(stderr)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		for sc.Scan() {
			addf("[%s] STDERR %s", ts(), clipStr(sc.Text(), 300))
		}
	}()

	// ---- 4) 逐帧捕获 stdout JSONL ----
	lines := make(chan string, 256)
	go func() {
		defer close(lines)
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 1024*1024), 16*1024*1024)
		for sc.Scan() {
			lines <- sc.Text()
		}
	}()

	overall := time.After(*wait + *postwait + 2*time.Minute)
	settled := false
	promptSent := false
	var promptResp string
	// 读循环:settled 前 + postwait 窗口内一直收。
	deadline := time.Now().Add(*wait)
	go func() {
		// postwait 由主循环控制:主循环退出后进程被 kill,读协程随 stdout 关闭结束。
		for l := range lines {
			mu.Lock()
			cls, extra := classify(l)
			typeCount[cls]++
			frames = append(frames, fmt.Sprintf("[%s] %s %s", ts(), cls, clipStr(l, extra)))
			mu.Unlock()
		}
	}()

	for !settled && time.Now().Before(deadline) {
		select {
		case l, ok := <-lines:
			if !ok {
				addf("[%s] stdout 关闭(pi 退出)", ts())
				settled = true
				break
			}
			var m struct {
				Type string          `json:"type"`
				ID   json.RawMessage `json:"id"`
				OK   *bool           `json:"success"`
			}
			if json.Unmarshal([]byte(l), &m) != nil {
				continue
			}
			// dialog 类扩展请求:如实拒绝(cancelled),记录原文。
			if m.Type == "extension_ui_request" {
				var ui struct {
					ID     string `json:"id"`
					Method string `json:"method"`
				}
				_ = json.Unmarshal([]byte(l), &ui)
				switch ui.Method {
				case "select", "confirm", "input", "editor":
					addf("[%s] DIALOG(%s) -> cancelled(探针不代用户决定)", ts(), ui.Method)
					_ = writeLine(stdin, map[string]any{"type": "extension_ui_response", "id": ui.ID, "cancelled": true})
				}
			}
			if m.Type == "response" {
				var id string
				_ = json.Unmarshal(m.ID, &id)
				if id == "pa-prompt" {
					promptResp = clipStr(l, 400)
					if m.OK == nil || !*m.OK {
						addf("[%s] prompt 被拒(受理前失败),提前收场", ts())
						settled = true
					}
				}
			}
			if promptSent && m.Type == "agent_settled" {
				settled = true
			}
		case <-overall:
			addf("[%s] 总看门狗到点,强制收场", ts())
			settled = true
		case <-time.After(200 * time.Millisecond):
		}
		if !promptSent && !settled {
			// 首帧到达后(初始化完成)再发 prompt。
			mu.Lock()
			n := len(frames)
			mu.Unlock()
			if n > 0 {
				if err := writeLine(stdin, map[string]any{"type": "prompt", "id": "pa-prompt", "message": *prompt}); err != nil {
					addf("[%s] 写 prompt 失败: %v", ts(), err)
					break
				}
				promptSent = true
				addf("[%s] >>> prompt 已发出", ts())
			}
		}
	}
	// 主循环退出后,排空剩余行(postwait 窗口)。
	drain := time.After(*postwait)
drainLoop:
	for {
		select {
		case l, ok := <-lines:
			if !ok {
				break drainLoop
			}
			mu.Lock()
			cls, extra := classify(l)
			typeCount[cls]++
			frames = append(frames, fmt.Sprintf("[%s] %s %s", ts(), cls, clipStr(l, extra)))
			mu.Unlock()
		case <-drain:
			break drainLoop
		}
	}

	// ---- 5) 收尾:get_state + 探针目录 sessions 清单 ----
	_ = writeLine(stdin, map[string]any{"type": "get_state", "id": "pa-state"})
	time.Sleep(3 * time.Second)
	sess := listSessions(*agentDir)

	if cmd.Process != nil {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}

	// ---- 6) 汇总落盘 ----
	mu.Lock()
	localFrames := append([]string(nil), frames...)
	counts := map[string]int{}
	for t, n := range typeCount {
		counts[t] = n
	}
	mu.Unlock()

	var b strings.Builder
	b.WriteString("# pi(pi-subagents)子代理通道真值捕获(T-SUBVIS)\n\n")
	b.WriteString("- 时间: " + time.Now().Format("2006-01-02 15:04:05") + "\n")
	b.WriteString(fmt.Sprintf("- 隔离 agent 目录: %s(npm=junction→~/.pi/agent/npm,只读;auth/models=本机复制)\n", *agentDir))
	b.WriteString(fmt.Sprintf("- 工作区: %s\n", *ws))
	b.WriteString("- prompt: " + *prompt + "\n")
	b.WriteString("- prompt 受理应答: " + promptResp + "\n")
	b.WriteString("\n## 事件类型计数\n\n")
	keys := make([]string, 0, len(counts))
	for t := range counts {
		keys = append(keys, t)
	}
	sort.Strings(keys)
	for _, t := range keys {
		fmt.Fprintf(&b, "- `%s` x%d\n", t, counts[t])
	}
	b.WriteString("\n## 探针 agent 目录下的 sessions 文件(子会话落点证据)\n\n```\n" + sess + "```\n")
	b.WriteString("\n## 原始 JSONL 帧(按到达顺序)\n\n```json\n")
	for _, f := range localFrames {
		b.WriteString(f + "\n")
	}
	b.WriteString("```\n")
	if err := os.WriteFile(*out, []byte(b.String()), 0o644); err != nil {
		log.Fatalf("写证据文件 %s: %v", *out, err)
	}
	fmt.Printf("== 完成: 证据=%s ==\n", *out)
}

// classify 给一行 JSONL 分类并决定截取长度(信息量大的帧给长预算)。
func classify(line string) (cls string, extra int) {
	var m struct {
		Type          string `json:"type"`
		MethodName    string `json:"method"`
		ToolName      string `json:"toolName"`
		CustomType    string `json:"customType"`
		ExtensionName string `json:"extensionName"`
	}
	_ = json.Unmarshal([]byte(line), &m)
	switch m.Type {
	case "tool_execution_start", "tool_execution_end":
		return m.Type + ":" + m.ToolName, 8000
	case "tool_execution_update":
		return "tool_execution_update:" + m.ToolName, 2500
	case "message_start", "message_end":
		return m.Type, 2500
	case "message_update":
		return "message_update", 300
	case "agent_start", "agent_end", "agent_settled":
		return m.Type, 1500
	case "extension_ui_request":
		return "extension_ui_request:" + m.MethodName, 2000
	case "response":
		return "response", 400
	default:
		// 扩展自定义事件(subagent_* 之类)可能带任意字段名:原类型名透传。
		name := m.Type
		if m.CustomType != "" {
			name = m.Type + ":" + m.CustomType
		}
		if strings.Contains(strings.ToLower(line), "subagent") || strings.Contains(strings.ToLower(line), "sub-agent") {
			return "SUBAGENT-HIT " + name, 8000
		}
		return name, 500
	}
}

func setupAgentDir(dir, home string) error {
	if err := os.MkdirAll(filepath.Join(dir, "sessions"), 0o755); err != nil {
		return err
	}
	settings := map[string]any{
		"lastChangelogVersion": "0.85.1",
		"packages":             []string{"npm:pi-subagents"},
	}
	raw, _ := json.MarshalIndent(settings, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), raw, 0o644); err != nil {
		return err
	}
	// npm junction(只读复用用户的 node_modules;失败退回整目录拷贝)。
	link := filepath.Join(dir, "npm")
	target := filepath.Join(home, ".pi", "agent", "npm")
	if _, err := os.Stat(link); err != nil {
		cmd := exec.Command("cmd", "/c", "mklink", "/J", link, target)
		hidewin.Apply(cmd)
		out, jerr := cmd.CombinedOutput()
		if jerr != nil {
			addf("[%s] junction 失败(%s),退回整目录拷贝(可能耗时)", ts(), clipStr(string(out), 200))
			if cerr := copyDir(target, link); cerr != nil {
				return fmt.Errorf("junction 与拷贝均失败: %w", cerr)
			}
		}
	}
	// 模型鉴权文件:本机复制(不外传、不改用户原件)。
	for _, f := range []string{"auth.json", "models-store.json"} {
		src := filepath.Join(home, ".pi", "agent", f)
		dst := filepath.Join(dir, f)
		if data, err := os.ReadFile(src); err == nil {
			_ = os.WriteFile(dst, data, 0o600)
		}
	}
	return nil
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		out := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(out, 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(out, data, 0o644)
	})
}

func setupWorkspace(ws string) error {
	if err := os.MkdirAll(ws, 0o755); err != nil {
		return err
	}
	files := map[string]string{
		"apple.txt": "apple 是一种水果,也是这家公司文化的代名词。\n本文件只用于探针验证,无其他内容。",
		"banana.md": "# banana\n\n黄色、弯的、甜。探针工作区的说明文件。",
		"cherry.go": "package main\n\n// cherry 只是个探针样例文件。\nfunc main() {}\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(ws, name), []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// listSessions 列出探针 agent 目录 sessions 子树(相对路径 + 字节数),
// 证明"子会话文件落在哪"。
func listSessions(agentDir string) string {
	root := filepath.Join(agentDir, "sessions")
	var b strings.Builder
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		info, _ := d.Info()
		n := int64(-1)
		if info != nil {
			n = info.Size()
		}
		rel, _ := filepath.Rel(agentDir, p)
		fmt.Fprintf(&b, "%s (%d bytes)\n", rel, n)
		return nil
	})
	return b.String()
}
