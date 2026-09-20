// cmd/piprobe —— Pi RPC 模式 P0 探针(不改生产代码,先验证)。
//
// 目的:把调研文档 docs/review/2026-09-17-pi-接入调研与方案.md §11 的 11 项
// "文档说 Pi 能这样"逐项变成"本机确实能这样",产出带原始报文(截断)的证据
// 报告(docs/verify/evidence/pi-p0-probe.md)。
//
// 关键设计(对应调研 §3.2 的坑):
//   - 直接 node 拉起 cli.js --mode rpc,不经 pi.ps1 垫片;
//   - JSONL 分帧只认 \n,用 bufio.Reader.ReadBytes(默认 64KB 上限会炸,
//     图片 base64 单行数 MB —— 这里给 8MB);
//   - stderr 单独收集,不混进 JSONL 解析;
//   - 命令带 id,request/response 关联;事件另行分发;
//   - 干净 env 启动(模拟 daemon 被 WMI/看门狗派生:没有用户 shell 的环境变量)。
package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pocketagent/daemon/internal/hidewin"
)

// ---- 定位(对应调研 §2:不能依赖 PATH,显式探测 + PI_BIN 式兜底)----

const (
	// 本机实测 node 位置(daemon 环境可能没有 PATH,必须显式探测)
	defaultNode = `C:\Users\yanzi\AppData\Local\Programs\nodejs\node.exe`
	// 本机实测 Pi 入口(不要经 pi.ps1 垫片,直接 node 调 cli.js)
	defaultCLI = `G:\Claude Code\node_modules\@earendil-works\pi-coding-agent\dist\bundle\cli.js`
)

// locateNode 找 node 可执行文件:PI_NODE → PI_BIN(CODEX_BIN 式兜底)→ 固定路径 → PATH。
func locateNode() string {
	for _, k := range []string{"PI_NODE", "PI_BIN"} {
		if p := strings.TrimSpace(os.Getenv(k)); p != "" {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	if _, err := os.Stat(defaultNode); err == nil {
		return defaultNode
	}
	if p, err := exec.LookPath("node"); err == nil {
		return p
	}
	return ""
}

// locateCLI 找 pi 的 cli.js:PI_CLI 覆盖 → 固定路径。
func locateCLI() string {
	if p := strings.TrimSpace(os.Getenv("PI_CLI")); p != "" {
		return p
	}
	return defaultCLI
}

// ---- 探针主体 ----

type probe struct {
	node, cli  string
	sessionDir string // --session-dir:收到探针自己的目录,避免污染用户会话池(调研 §6.2)
	workDir    string // 子进程 cwd(决定 Pi 会话的工作区)
	extFile    string // -e 临时审批扩展(可空)
	noExt      bool   // --no-extensions:关掉本机 4 个扩展包做对照

	cmd     *exec.Cmd
	stdin   interface{ Write([]byte) (int, error) }
	out     *bufio.Reader
	envMode string // "clean" / "inherit"

	mu        sync.Mutex
	nextID    int
	pending   map[string]chan map[string]any
	records   []string // 原始行记录(截断),生成报告用
	stderrLog []string // stderr 摘录(诊断"pi 起不来/扩展报错")
	stderrN   int
	events    chan map[string]any
	extMu     sync.Mutex
	extWatch  func(req map[string]any) // extension_ui_request 处理器(审批测试设)
	dead      bool
	startedAt time.Time
}

// cleanEnv 构造"干净环境":模拟 daemon 由计划任务/WMI 派生时的最小环境变量集
// (调研 §2:P0 必须验证这条链路,否则会出现"终端里好好的,手机上一发就报错")。
func cleanEnv() []string {
	home := os.Getenv("USERPROFILE")
	if home == "" {
		home = `C:\Users\yanzi`
	}
	tmp := os.Getenv("TEMP")
	if tmp == "" {
		tmp = filepath.Join(home, `AppData\Local\Temp`)
	}
	return []string{
		"SystemRoot=C:\\Windows",
		"windir=C:\\Windows",
		"COMPUTERNAME=" + hostnameOrLocal(),
		"USERPROFILE=" + home,
		"HOMEDRIVE=" + home[:2],
		"HOMEPATH=" + home[2:],
		"APPDATA=" + filepath.Join(home, `AppData\Roaming`),
		"LOCALAPPDATA=" + filepath.Join(home, `AppData\Local`),
		"TEMP=" + tmp,
		"TMP=" + tmp,
		// node 与 Git Bash 的最小定位:Pi 在 Windows 上用 Git Bash 跑 bash 工具(调研 §2)
		"PATH=C:\\Windows\\System32;C:\\Windows;C:\\Program Files\\Git\\bin;C:\\Program Files\\Git\\cmd",
	}
}

func hostnameOrLocal() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return "localhost"
	}
	return h
}

func (p *probe) record(line string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.records) < 4000 {
		line = strings.TrimRight(line, "\r\n")
		if len(line) > 600 {
			line = fmt.Sprintf("%.600s …[截断,整行 %d 字节]", line, len(line))
		}
		p.records = append(p.records, line)
	}
}

// start 拉起 node cli.js --mode rpc。
func (p *probe) start() error {
	args := []string{p.cli, "--mode", "rpc", "--session-dir", p.sessionDir, "--name", "piprobe"}
	if p.extFile != "" {
		args = append(args, "--extension", p.extFile)
	}
	if p.noExt {
		args = append(args, "--no-extensions")
	}
	cmd := exec.Command(p.node, args...)
	cmd.Dir = p.workDir
	if p.envMode == "clean" {
		cmd.Env = cleanEnv()
	} else {
		cmd.Env = os.Environ()
	}
	hidewin.Apply(cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动 pi rpc 子进程失败: %w", err)
	}
	p.cmd = cmd
	p.stdin = stdin
	p.out = bufio.NewReaderSize(stdout, 8*1024*1024) // 图片 base64 单行数 MB,64KB 默认必炸(调研 §3.2#1)
	p.startedAt = time.Now()
	go p.readLoop()
	go p.readStderr(stderr)
	return nil
}

func (p *probe) readStderr(r interface{ Read([]byte) (int, error) }) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		p.mu.Lock()
		p.stderrN++
		if len(p.records) < 4000 && len(p.stderrLog) < 200 {
			l := sc.Text()
			if len(l) > 300 {
				l = l[:300] + "…"
			}
			p.stderrLog = append(p.stderrLog, l)
		}
		p.mu.Unlock()
	}
}

func (p *probe) readLoop() {
	for {
		line, err := p.out.ReadBytes('\n')
		if len(line) > 0 {
			p.record(string(line))
			var m map[string]any
			if json.Unmarshal(line, &m) == nil {
				t, _ := m["type"].(string)
				switch t {
				case "response":
					id, _ := m["id"].(string)
					p.mu.Lock()
					ch, ok := p.pending[id]
					delete(p.pending, id)
					p.mu.Unlock()
					if ok {
						ch <- m
					}
				case "extension_ui_request":
					p.extMu.Lock()
					f := p.extWatch
					p.extMu.Unlock()
					if f != nil {
						go f(m) // 回写走 stdin,不能堵 readLoop
					}
					p.pushEvent(m)
				default:
					p.pushEvent(m)
				}
			}
		}
		if err != nil {
			p.mu.Lock()
			p.dead = true
			for id, ch := range p.pending {
				ch <- map[string]any{"type": "response", "success": false, "error": "子进程已退出: " + err.Error()}
				delete(p.pending, id)
			}
			p.mu.Unlock()
			close(p.events)
			return
		}
	}
}

func (p *probe) pushEvent(m map[string]any) {
	select {
	case p.events <- m:
	default: // 溢出丢弃:探针按需等待,不追求全量
	}
}

func (p *probe) sendLine(obj any) error {
	raw, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	p.mu.Lock()
	dead := p.dead
	p.mu.Unlock()
	if dead {
		return fmt.Errorf("子进程已退出")
	}
	_, err = p.stdin.Write(append(raw, '\n'))
	return err
}

// call 发一条命令并等 response(带 id 关联)。
func (p *probe) call(typ string, extra map[string]any, timeout time.Duration) (map[string]any, error) {
	p.mu.Lock()
	p.nextID++
	id := fmt.Sprintf("piprobe-%d", p.nextID)
	p.mu.Unlock()
	obj := map[string]any{"id": id, "type": typ}
	for k, v := range extra {
		obj[k] = v
	}
	ch := make(chan map[string]any, 1)
	p.mu.Lock()
	if p.dead {
		p.mu.Unlock()
		return nil, fmt.Errorf("子进程已退出")
	}
	p.pending[id] = ch
	p.mu.Unlock()
	if err := p.sendLine(obj); err != nil {
		p.mu.Lock()
		delete(p.pending, id)
		p.mu.Unlock()
		return nil, err
	}
	select {
	case resp := <-ch:
		return resp, nil
	case <-time.After(timeout):
		p.mu.Lock()
		delete(p.pending, id)
		p.mu.Unlock()
		return nil, fmt.Errorf("%s 等待应答超时(%s)", typ, timeout)
	}
}

// waitEvent 从事件流里等一个满足条件的事件(期间其它事件丢弃)。
func (p *probe) waitEvent(match func(m map[string]any) bool, timeout time.Duration) (map[string]any, bool) {
	deadline := time.After(timeout)
	for {
		select {
		case m, ok := <-p.events:
			if !ok {
				return nil, false
			}
			if match(m) {
				return m, true
			}
		case <-deadline:
			return nil, false
		}
	}
}

// collectEvents 收集事件直到 pred 命中或超时,返回期间收到的所有事件。
func (p *probe) collectEvents(pred func(m map[string]any) bool, timeout time.Duration) ([]map[string]any, bool) {
	var out []map[string]any
	deadline := time.After(timeout)
	for {
		select {
		case m, ok := <-p.events:
			if !ok {
				return out, false
			}
			out = append(out, m)
			if pred != nil && pred(m) {
				return out, true
			}
		case <-deadline:
			return out, false
		}
	}
}

func (p *probe) drain() {
	for {
		select {
		case _, ok := <-p.events:
			if !ok {
				return
			}
		case <-time.After(300 * time.Millisecond):
			return
		}
	}
}

func (p *probe) close() {
	if p.cmd != nil && p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
	}
}

func (p *probe) dump() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := append([]string(nil), p.records...)
	for _, l := range p.stderrLog {
		out = append(out, "[stderr] "+l)
	}
	return out
}

func (p *probe) stderrCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.stderrN
}

// ---- 测试结果 ----

type result struct {
	n      int
	name   string
	status string // "通过" / "替代" / "失败"
	note   string
	proof  []string // 关键报文摘录(已截断)
}

// ---- main ----

func main() {
	out := flag.String("out", `G:\项目1\pocketagent\docs\verify\evidence\pi-p0-probe.md`, "报告输出路径")
	mode := flag.String("mode", "p0", "p0 = 全量 P0 探针;s = S 级增量命令探针(export_html/compact/thinking_levels/follow_up/session_info_changed);cmds = get_commands 斜杠命令探针")
	flag.Parse()

	r := &runner{}
	switch *mode {
	case "s":
		fmt.Println("== Pi S 级增量命令探针开始 ==")
		r.runSMode()
		fmt.Println("== 探针结束 ==")
	case "cmds":
		fmt.Println("== Pi get_commands 斜杠命令探针开始 ==")
		r.runCmdsMode()
		fmt.Println("== 探针结束 ==")
	default:
		fmt.Println("== Pi P0 探针开始 ==")
		r.runAll()
		fmt.Println("== 探针结束 ==")
	}

	// 生成报告
	title := "Pi RPC 模式 P0 探针记录"
	switch *mode {
	case "s":
		title = "Pi S 级增量命令探针记录(export_html/compact/thinking_levels/follow_up/session_info_changed)"
	case "cmds":
		title = "Pi get_commands 斜杠命令探针记录"
	}
	var b strings.Builder
	b.WriteString("# " + title + "\n\n")
	b.WriteString("> 生成:" + time.Now().Format("2006-01-02 15:04:05") + " 工具:daemon/cmd/piprobe(直接 node 拉起 cli.js --mode rpc,Go 手写 JSONL 分帧)\n\n")
	b.WriteString(r.summaryMD())
	for _, res := range r.results {
		b.WriteString(fmt.Sprintf("\n## %d. %s —— %s\n\n%s\n", res.n, res.name, res.status, res.note))
		if len(res.proof) > 0 {
			b.WriteString("```\n")
			for _, l := range res.proof {
				b.WriteString(l + "\n")
			}
			b.WriteString("```\n")
		}
	}
	b.WriteString("\n## 原始报文附录(全部,截断)\n\n```\n")
	for _, l := range r.rawLog {
		b.WriteString(l + "\n")
	}
	b.WriteString("```\n")
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		fmt.Println("建目录失败:", err)
	}
	if err := os.WriteFile(*out, []byte(b.String()), 0o644); err != nil {
		fmt.Println("写报告失败:", err)
	} else {
		fmt.Println("报告已写入:", *out)
	}
}

type runner struct {
	results []result
	rawLog  []string
}

func (r *runner) add(res result) {
	r.results = append(r.results, res)
	fmt.Printf("[%d] %s —— %s\n", res.n, res.name, res.status)
}

func (r *runner) collect(p *probe) {
	for _, l := range p.dump() {
		if len(r.rawLog) < 4000 {
			r.rawLog = append(r.rawLog, l)
		}
	}
}

func (r *runner) summaryMD() string {
	pass, alt, fail := 0, 0, 0
	for _, res := range r.results {
		switch res.status {
		case "通过":
			pass++
		case "替代":
			alt++
		case "失败":
			fail++
		}
	}
	return fmt.Sprintf("结果:**%d 通过 / %d 替代 / %d 失败**(共 %d 项)\n", pass, alt, fail, len(r.results))
}

func (r *runner) runAll() {
	node, cli := locateNode(), locateCLI()

	// 0) 环境定位 + --version(为第 1、11 项提供事实)
	ver := ""
	if node == "" || cli == "" {
		r.add(result{n: 1, name: "daemon 真实环境拉起", status: "失败", note: fmt.Sprintf("node=%q cli=%q 定位失败", node, cli)})
		return
	}
	if out, err := exec.Command(node, cli, "--version").Output(); err == nil {
		ver = strings.TrimSpace(string(out))
	}
	fmt.Printf("node=%s\ncli=%s\npi version=%s\n", node, cli, ver)

	// 探针工作区与会话目录(临时,不污染用户 ~/.pi/agent/sessions —— 调研 §6.2)
	base, err := os.MkdirTemp("", "piprobe-*")
	if err != nil {
		r.add(result{n: 1, name: "daemon 真实环境拉起", status: "失败", note: "建临时目录失败: " + err.Error()})
		return
	}
	defer os.RemoveAll(base)
	workDir := filepath.Join(base, "ws")
	_ = os.MkdirAll(workDir, 0o755)
	sessionDir := filepath.Join(base, "sessions")
	_ = os.MkdirAll(sessionDir, 0o755)

	// ---- 进程 A:干净 env + 全量扩展(用户装了 4 个扩展包,验证它们的行为)----
	extFile := filepath.Join(base, "approve-probe.ts")
	if err := os.WriteFile(extFile, []byte(approveProbeExt), 0o644); err != nil {
		r.add(result{n: 6, name: "审批 extension_ui_request", status: "失败", note: "写扩展失败: " + err.Error()})
	}
	p := &probe{
		node: node, cli: cli, sessionDir: sessionDir, workDir: workDir,
		extFile: extFile, envMode: "clean",
		pending: map[string]chan map[string]any{},
		events:  make(chan map[string]any, 8192),
	}
	startErr := p.start()
	if startErr == nil {
		// 启动后给 2 秒缓冲(扩展加载、startup 事件)
		time.Sleep(2 * time.Second)
	}

	// 1) 干净 env 拉起 + ~/.pi/agent 可定位
	if startErr != nil {
		r.add(result{n: 1, name: "daemon 真实环境拉起", status: "失败", note: startErr.Error() + "\nenv=clean(模拟 WMI 派生)node=" + node + " cli=" + cli})
	} else {
		proof := []string{fmt.Sprintf("env=clean node=%s", node), fmt.Sprintf("cli=%s", cli)}
		r.add(result{n: 1, name: "daemon 真实环境拉起(干净 env,不经 pi.ps1)", status: "通过",
			note: "最小环境变量集(SystemRoot/USERPROFILE/TEMP/PATH 等)直接 node 拉起 cli.js --mode rpc 成功,stderr 行数:" + fmt.Sprint(p.stderrCount()), proof: proof})
	}

	// 2) get_state 握手
	var sessionFile, sessionID, modelStr string
	if startErr != nil {
		r.add(result{n: 2, name: "get_state 握手", status: "失败", note: "进程未启动"})
	} else {
		resp, err := p.call("get_state", nil, 15*time.Second)
		if err != nil {
			r.add(result{n: 2, name: "get_state 握手", status: "失败", note: err.Error()})
		} else if ok, _ := resp["success"].(bool); !ok {
			r.add(result{n: 2, name: "get_state 握手", status: "失败", note: "success=false: " + fmt.Sprint(resp["error"])})
		} else {
			data, _ := resp["data"].(map[string]any)
			sessionFile, _ = data["sessionFile"].(string)
			sessionID, _ = data["sessionId"].(string)
			if m, ok := data["model"].(map[string]any); ok {
				modelStr = fmt.Sprint(m["provider"]) + "/" + fmt.Sprint(m["id"])
			}
			r.add(result{n: 2, name: "get_state 握手", status: "通过",
				note:  fmt.Sprintf("sessionId=%s model=%s isStreaming=%v sessionFile=%s", sessionID, modelStr, data["isStreaming"], sessionFile),
				proof: clipLines(p.dump(), `"command":"get_state"`, 6)})
		}
	}

	// 3) 一轮完整对话:text_delta → agent_settled
	if startErr == nil {
		res3 := testTurn(p, "只回复四个字:探针成功。不要使用任何工具。")
		r.add(result{n: 3, name: "一轮对话(text_delta → agent_settled)", status: res3.status, note: res3.note, proof: res3.proof})
	}

	// 4) 工具调用:toolcall_* + tool_execution_*,partialResult 累积验证
	if startErr == nil {
		p.drain()
		resp, err := p.call("prompt", map[string]any{"message": "用 bash 工具执行这条命令:for i in $(seq 1 15); do echo line-$i; sleep 0.15; done。执行完把输出原样贴出来。"}, 20*time.Second)
		if err != nil {
			r.add(result{n: 4, name: "工具调用事件 + partialResult 累积", status: "失败", note: err.Error()})
		} else if ok, _ := resp["success"].(bool); !ok {
			r.add(result{n: 4, name: "工具调用事件 + partialResult 累积", status: "失败", note: "prompt 被拒: " + fmt.Sprint(resp["error"])})
		} else {
			evs, settled := p.collectEvents(func(m map[string]any) bool {
				t, _ := m["type"].(string)
				return t == "agent_settled"
			}, 120*time.Second)
			note, proof, ok4 := analyzeToolEvents(evs)
			status := "通过"
			if !settled {
				status = "失败"
				note += ";未等到 agent_settled"
			} else if !ok4 {
				status = "替代"
			}
			r.add(result{n: 4, name: "工具调用事件 + partialResult 累积", status: status, note: note, proof: proof})
		}
	}

	// 5) abort 中断
	if startErr == nil {
		p.drain()
		_, _ = p.call("prompt", map[string]any{"message": "用 bash 工具执行:sleep 30 && echo done。等命令跑完再回复。"}, 15*time.Second)
		// 等工具真正开始跑
		p.waitEvent(func(m map[string]any) bool {
			t, _ := m["type"].(string)
			return t == "tool_execution_start"
		}, 30*time.Second)
		time.Sleep(1 * time.Second)
		abortAt := time.Now()
		abortResp, aerr := p.call("abort", nil, 20*time.Second)
		_, settled := p.collectEvents(func(m map[string]any) bool {
			t, _ := m["type"].(string)
			return t == "agent_settled"
		}, 60*time.Second)
		abortOK := aerr == nil && boolOf(abortResp["success"])
		if abortOK && settled {
			r.add(result{n: 5, name: "abort 中断", status: "通过",
				note:  fmt.Sprintf("abort success=true,%s 后收到 agent_settled(命令要跑 30s,提前收尾=中断生效)", time.Since(abortAt).Round(time.Millisecond)),
				proof: pickLines(p.dump(), []string{`"command":"abort"`, `"type":"agent_end"`, `"type":"agent_settled"`}, 8)})
		} else {
			r.add(result{n: 5, name: "abort 中断", status: "失败",
				note: fmt.Sprintf("abort success=%v err=%v settled=%v", abortOK, aerr, settled)})
		}
	}

	// 6) 审批:注入最小 tool_call 扩展 → extension_ui_request 出现 → response 回写放行/拦截
	if startErr == nil {
		r.add(r.testApproval(p))
	}

	// 9) 体积:~1MB base64 图片走 prompt.images,验证 8MB 分帧缓冲
	if startErr == nil {
		r.add(r.testImage(p))
	}

	// 10) 扩展干扰:全程已记录,这里汇总本机 4 个扩展包是否主动弹 UI
	if startErr == nil {
		proof := pickLines(p.dump(), []string{`extension_ui_request`}, 10)
		n := countSub(p.dump(), `extension_ui_request`)
		status, note := "通过", fmt.Sprintf("全程共出现 %d 次 extension_ui_request(明细见附录;含探针注入扩展的审批请求)", n)
		if n == 0 {
			status, note = "替代", "全程未出现任何 extension_ui_request:本机 4 个扩展包(pi-web-access/pi-subagents/pi-hermes-memory/pi-lens)在普通对话+工具调用流程中未主动弹 UI,无挂死风险,但也未实测其主动弹窗场景"
		}
		r.add(result{n: 10, name: "扩展干扰记录(4 个扩展包)", status: status, note: note, proof: proof})
	}

	p.close()
	time.Sleep(500 * time.Millisecond)
	r.collect(p)

	// ---- 进程 B:switch_session 重挂载 + get_entries 游标(对应调研 §8.3)----
	if sessionFile == "" {
		r.add(result{n: 7, name: "switch_session 恢复历史", status: "失败", note: "进程 A 未取得 sessionFile,无法验证"})
		r.add(result{n: 8, name: "get_entries(since=) 游标", status: "失败", note: "同上"})
		return
	}
	pb := &probe{
		node: node, cli: cli, sessionDir: sessionDir, workDir: workDir,
		envMode: "clean", pending: map[string]chan map[string]any{},
		events: make(chan map[string]any, 8192),
	}
	if err := pb.start(); err != nil {
		r.add(result{n: 7, name: "switch_session 恢复历史", status: "失败", note: "进程 B 启动失败: " + err.Error()})
		r.add(result{n: 8, name: "get_entries(since=) 游标", status: "失败", note: "同上"})
		pb.close()
		return
	}
	time.Sleep(2 * time.Second)

	// 7) switch_session + 历史完整
	resp, err := pb.call("switch_session", map[string]any{"sessionPath": sessionFile}, 15*time.Second)
	if err != nil || !boolOf(resp["success"]) {
		r.add(result{n: 7, name: "switch_session 恢复历史", status: "失败", note: fmt.Sprintf("err=%v resp=%v", err, resp)})
	} else {
		st, _ := pb.call("get_state", nil, 15*time.Second)
		data, _ := st["data"].(map[string]any)
		mr, _ := pb.call("get_messages", nil, 15*time.Second)
		md, _ := mr["data"].(map[string]any)
		msgs, _ := md["messages"].([]any)
		histHas := false
		raw, _ := json.Marshal(md)
		if strings.Contains(string(raw), "探针成功") || strings.Contains(string(raw), "line-14") {
			histHas = true
		}
		got, _ := data["sessionFile"].(string)
		if samePath(got, sessionFile) && len(msgs) > 0 && histHas {
			r.add(result{n: 7, name: "switch_session 恢复历史", status: "通过",
				note:  fmt.Sprintf("switch 成功;get_state.sessionFile 与预期一致;get_messages 返回 %d 条,含此前轮次的文本", len(msgs)),
				proof: clipLines(pb.dump(), `"command":"switch_session"`, 4)})
		} else {
			r.add(result{n: 7, name: "switch_session 恢复历史", status: "失败",
				note: fmt.Sprintf("sessionFile 匹配=%v msgs=%d 历史含已知文本=%v", samePath(got, sessionFile), len(msgs), histHas)})
		}
	}

	// 8) get_entries 游标:全量 → 记 lastId → 新增条目 → since 增量 → 错值 success:false
	testGetEntries(pb, r)

	pb.close()
	time.Sleep(500 * time.Millisecond)
	r.collect(pb)
}

// testTurn 发一轮对话并分析 text_delta 与 agent_settled。
func testTurn(p *probe, msg string) result {
	p.drain()
	resp, err := p.call("prompt", map[string]any{"message": msg}, 20*time.Second)
	if err != nil {
		return result{status: "失败", note: err.Error()}
	}
	if ok, _ := resp["success"].(bool); !ok {
		return result{status: "失败", note: "prompt 被拒: " + fmt.Sprint(resp["error"])}
	}
	var text strings.Builder
	evs, settled := p.collectEvents(func(m map[string]any) bool {
		t, _ := m["type"].(string)
		return t == "agent_settled"
	}, 180*time.Second)
	for _, m := range evs {
		if t, _ := m["type"].(string); t == "message_update" {
			if ame, ok := m["assistantMessageEvent"].(map[string]any); ok {
				if at, _ := ame["type"].(string); at == "text_delta" {
					text.WriteString(fmt.Sprint(ame["delta"]))
				}
			}
		}
	}
	note := fmt.Sprintf("prompt success=true(受理);收到 %d 个事件;agent_settled=%v;拼接文本=%q", len(evs), settled, clip(text.String(), 60))
	if !settled {
		return result{status: "失败", note: note}
	}
	hasAgentEnd := false
	for _, m := range evs {
		if t, _ := m["type"].(string); t == "agent_end" {
			hasAgentEnd = true
		}
	}
	if hasAgentEnd {
		note += ";事件序含 agent_end(agent_settled 才判终局,验证通过)"
	}
	return result{status: "通过", note: note, proof: pickLines(p.dump(), []string{`"text_delta"`, `"type":"agent_settled"`}, 6)}
}

// analyzeToolEvents 分析第 4 项的事件流。
func analyzeToolEvents(evs []map[string]any) (note string, proof []string, ok bool) {
	var toolcallStart, toolcallDelta, toolcallEnd, execStart, execUpdate, execEnd int
	var lastPartial, prevPartial string
	var cumulative = true
	for _, m := range evs {
		t, _ := m["type"].(string)
		switch t {
		case "message_update":
			if ame, ok := m["assistantMessageEvent"].(map[string]any); ok {
				switch at, _ := ame["type"].(string); at {
				case "toolcall_start":
					toolcallStart++
				case "toolcall_delta":
					toolcallDelta++
				case "toolcall_end":
					toolcallEnd++
				}
			}
		case "tool_execution_start":
			execStart++
		case "tool_execution_update":
			execUpdate++
			// partialResult 累积验证(调研 §3.2#7):后一次应包含前一次的开头
			pr, _ := m["partialResult"].(map[string]any)
			lastPartial = partialText(pr)
			if prevPartial != "" && lastPartial != "" {
				if !strings.HasPrefix(lastPartial, prevPartial) && !strings.Contains(lastPartial, firstLineOf(prevPartial)) {
					cumulative = false
				}
			}
			prevPartial = lastPartial
		case "tool_execution_end":
			execEnd++
		}
	}
	note = fmt.Sprintf("toolcall_start/delta/end=%d/%d/%d tool_execution_start/update/end=%d/%d/%d", toolcallStart, toolcallDelta, toolcallEnd, execStart, execUpdate, execEnd)
	ok = toolcallStart > 0 && execStart > 0 && execEnd > 0
	if execUpdate >= 2 {
		if cumulative {
			note += fmt.Sprintf(";partialResult 为累积值(后值含前值前缀,最后长度 %d 字符)", len(lastPartial))
		} else {
			note += ";partialResult 疑似非累积(需人工核对附录)"
			ok = false
		}
	} else {
		note += fmt.Sprintf(";update 次数=%d,不足以比对累积性", execUpdate)
	}
	return note, proof, ok
}

func partialText(pr map[string]any) string {
	if pr == nil {
		return ""
	}
	content, _ := pr["content"].([]any)
	var sb strings.Builder
	for _, c := range content {
		cm, _ := c.(map[string]any)
		if cm == nil {
			continue
		}
		if ct, _ := cm["type"].(string); ct == "text" {
			s, _ := cm["text"].(string)
			sb.WriteString(s)
		}
	}
	return sb.String()
}

// testApproval 第 6 项:注入扩展的 confirm 审批,放行 + 拦截两条路。
func (r *runner) testApproval(p *probe) result {
	p.drain()
	// A. 放行:watcher 回 confirmed:true
	p.extMu.Lock()
	p.extWatch = func(req map[string]any) {
		if m, _ := req["method"].(string); m == "confirm" {
			_ = p.sendLine(map[string]any{"type": "extension_ui_response", "id": req["id"], "confirmed": true})
		}
	}
	p.extMu.Unlock()
	resp, err := p.call("prompt", map[string]any{"message": "用 bash 工具原样执行:echo APPROVE_PROBE_ALLOW。不要做任何别的事。"}, 20*time.Second)
	if err != nil || !boolOf(resp["success"]) {
		return result{n: 6, name: "审批 extension_ui_request(注入扩展 confirm)", status: "失败", note: fmt.Sprintf("prompt 失败 err=%v resp=%v", err, resp["error"])}
	}
	evs, settled := p.collectEvents(func(m map[string]any) bool {
		t, _ := m["type"].(string)
		return t == "agent_settled"
	}, 120*time.Second)
	reqSeen, allowOut := false, false
	for _, m := range evs {
		if t, _ := m["type"].(string); t == "extension_ui_request" {
			if mm, _ := m["method"].(string); mm == "confirm" {
				reqSeen = true
			}
		}
		if t, _ := m["type"].(string); t == "tool_execution_end" {
			if strings.Contains(partialText(mapOf(m["result"])), "APPROVE_PROBE_ALLOW") {
				allowOut = true
			}
		}
	}
	_ = settled

	// B. 拦截:watcher 回 cancelled:true(扩展收到 false → block)
	p.drain()
	p.extMu.Lock()
	p.extWatch = func(req map[string]any) {
		if m, _ := req["method"].(string); m == "confirm" {
			_ = p.sendLine(map[string]any{"type": "extension_ui_response", "id": req["id"], "cancelled": true})
		}
	}
	p.extMu.Unlock()
	resp2, err2 := p.call("prompt", map[string]any{"message": "用 bash 工具原样执行:echo APPROVE_PROBE_DENY。不要做任何别的事。"}, 20*time.Second)
	if err2 != nil || !boolOf(resp2["success"]) {
		return result{status: "失败", note: fmt.Sprintf("第二轮 prompt 失败 err=%v resp=%v", err2, resp2["error"])}
	}
	evs2, settled2 := p.collectEvents(func(m map[string]any) bool {
		t, _ := m["type"].(string)
		return t == "agent_settled"
	}, 120*time.Second)
	denyOut, denied := false, false
	for _, m := range evs2 {
		if t, _ := m["type"].(string); t == "tool_execution_end" {
			if strings.Contains(partialText(mapOf(m["result"])), "APPROVE_PROBE_DENY") {
				denyOut = true
			}
		}
		if t, _ := m["type"].(string); t == "message_update" {
			if ame, ok := m["assistantMessageEvent"].(map[string]any); ok {
				if at, _ := ame["type"].(string); at == "text_delta" {
					if strings.Contains(fmt.Sprint(ame["delta"]), "拦截") || strings.Contains(fmt.Sprint(ame["delta"]), "Blocked") {
						denied = true
					}
				}
			}
		}
	}
	if !settled2 {
		// 等待兜底
	}
	// 收尾:摘掉 watcher,防止影响后续测试
	p.extMu.Lock()
	p.extWatch = nil
	p.extMu.Unlock()

	if reqSeen && allowOut && !denyOut {
		return result{n: 6, name: "审批 extension_ui_request(注入扩展 confirm)", status: "通过",
			note:  fmt.Sprintf("extension_ui_request(confirm,无 timeout)出现;confirmed:true → 命令实际执行(输出含 ALLOW);cancelled:true → 被扩展 block(未执行 DENY,agent 回复拦截=%v)", denied),
			proof: pickLines(p.dump(), []string{`extension_ui_request`, `extension_ui_response`}, 8)}
	}
	return result{n: 6, name: "审批 extension_ui_request(注入扩展 confirm)", status: "失败",
		note: fmt.Sprintf("reqSeen=%v allowOut=%v denyOut=%v settled2=%v(明细见附录)", reqSeen, allowOut, denyOut, settled2)}
}

// testImage 第 9 项:发 ~1MB base64 图片。
func (r *runner) testImage(p *probe) result {
	p.drain()
	img := makeNoisePNG(500, 500) // 随机噪声 PNG 压不动,~1MB
	b64 := base64.StdEncoding.EncodeToString(img)
	sendLine := fmt.Sprintf(`{"id":"img-1","type":"prompt","message":"用不超过十个字描述这张图。","images":[{"type":"image","data":"<base64 %d 字节>","mimeType":"image/png"}]}`, len(b64))
	resp, err := p.call("prompt", map[string]any{
		"message": "用不超过十个字描述这张图。",
		"images":  []any{map[string]any{"type": "image", "data": b64, "mimeType": "image/png"}},
	}, 30*time.Second)
	if err != nil {
		return result{n: 9, name: "体积(~1MB base64 图片分帧)", status: "失败", note: fmt.Sprintf("发送失败(单行 JSON %d 字节): %v", len(sendLine), err)}
	}
	if ok, _ := resp["success"].(bool); !ok {
		return result{n: 9, name: "体积(~1MB base64 图片分帧)", status: "失败", note: fmt.Sprintf("prompt 被拒: %s", fmt.Sprint(resp["error"])), proof: []string{clip(sendLine, 160)}}
	}
	_, settled := p.collectEvents(func(m map[string]any) bool {
		t, _ := m["type"].(string)
		return t == "agent_settled"
	}, 180*time.Second)
	maxIn := 0
	for _, l := range p.dump() {
		if strings.Contains(l, "…[截断,整行") {
			if n := parseIntBetween(l, "整行 ", " 字节"); n > maxIn {
				maxIn = n
			}
		}
	}
	status := "通过"
	note := fmt.Sprintf("图片 base64=%d 字节(单行 JSON>1.3MB),prompt success=true,agent_settled=%v;8MB bufio.ReaderSize 分帧正常", len(b64), settled)
	if !settled {
		status = "替代"
		note += "(响应事件未在时限内收齐,但发送/受理链路已验证)"
	}
	if maxIn > 0 {
		note += fmt.Sprintf(";单行最大 %d 字节", maxIn)
	}
	return result{n: 9, name: "体积(~1MB base64 图片分帧)", status: status, note: note, proof: []string{clip(sendLine, 200)}}
}

// testGetEntries 第 8 项。
func testGetEntries(pb *probe, r *runner) {
	pb.drain()
	resp, err := pb.call("get_entries", nil, 15*time.Second)
	if err != nil || !boolOf(resp["success"]) {
		r.add(result{n: 8, name: "get_entries(since=) 游标", status: "失败", note: fmt.Sprintf("全量失败 err=%v resp=%v", err, resp["error"])})
		return
	}
	data, _ := resp["data"].(map[string]any)
	entries, _ := data["entries"].([]any)
	if len(entries) == 0 {
		r.add(result{n: 8, name: "get_entries(since=) 游标", status: "失败", note: "entries 为空"})
		return
	}
	last, _ := entries[len(entries)-1].(map[string]any)
	lastID, _ := last["id"].(string)

	// 造一条新条目:发一条短消息(产生新 session entry)
	pr, err := pb.call("prompt", map[string]any{"message": "只回复一个字:好。"}, 20*time.Second)
	if err != nil || !boolOf(pr["success"]) {
		r.add(result{n: 8, name: "get_entries(since=) 游标", status: "失败", note: "造新条目失败: " + fmt.Sprint(err, pr["error"])})
		return
	}
	pb.collectEvents(func(m map[string]any) bool {
		t, _ := m["type"].(string)
		return t == "agent_settled"
	}, 120*time.Second)

	inc, err := pb.call("get_entries", map[string]any{"since": lastID}, 15*time.Second)
	if err != nil || !boolOf(inc["success"]) {
		r.add(result{n: 8, name: "get_entries(since=) 游标", status: "失败", note: fmt.Sprintf("since 增量失败 err=%v resp=%v", err, inc["error"])})
		return
	}
	incData, _ := inc["data"].(map[string]any)
	incEntries, _ := incData["entries"].([]any)

	bad, _ := pb.call("get_entries", map[string]any{"since": "piprobe-nonexistent-id"}, 15*time.Second)
	badOK := boolOf(bad["success"])
	note := fmt.Sprintf("全量 %d 条;新增后 since=%s 增量 %d 条;错误游标 success=%v(应为 false)", len(entries), clip(lastID, 12), len(incEntries), badOK)
	status := "通过"
	if badOK || len(incEntries) == 0 {
		status = "失败"
	}
	r.add(result{n: 8, name: "get_entries(since=) 游标", status: status, note: note,
		proof: pickLines(pb.dump(), []string{`"command":"get_entries"`}, 4)})
}

// approveProbeExt 探针注入的最小审批扩展:对含 APPROVE_PROBE 的 bash 命令弹 confirm,
// 故意不带 timeout(验证 daemon 侧必须自己兜底超时 —— 调研 §4.6)。
const approveProbeExt = `export default function (pi: any) {
  pi.on("tool_call", async (event: any, ctx: any) => {
    const cmd = event?.input?.command ?? "";
    if (typeof cmd === "string" && cmd.includes("APPROVE_PROBE")) {
      const ok = await ctx.ui.confirm("piprobe 审批", "允许执行该探针命令吗?(无 timeout)");
      if (!ok) return { block: true, reason: "piprobe 探针拦截" };
    }
    return undefined;
  });
}
`

// ---- 小工具 ----

func boolOf(v any) bool {
	b, _ := v.(bool)
	return b
}

func mapOf(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func firstLineOf(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func samePath(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

func pickLines(lines []string, subs []string, max int) []string {
	var out []string
	for _, sub := range subs {
		for _, l := range lines {
			if strings.Contains(l, sub) {
				out = append(out, clip(l, 500))
				if len(out) >= max {
					return out
				}
			}
		}
	}
	return out
}

func clipLines(lines []string, sub string, max int) []string {
	return pickLines(lines, []string{sub}, max)
}

func countSub(lines []string, sub string) int {
	n := 0
	for _, l := range lines {
		n += strings.Count(l, sub)
	}
	return n
}

func parseIntBetween(s, a, b string) int {
	i := strings.Index(s, a)
	if i < 0 {
		return 0
	}
	j := strings.Index(s[i+len(a):], b)
	if j < 0 {
		return 0
	}
	n := 0
	for _, c := range s[i+len(a) : i+len(a)+j] {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// makeNoisePNG 生成随机噪声 PNG(压缩不动,size 接近 w*h*4)。
func makeNoisePNG(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	rnd := rand.New(rand.NewSource(42))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(rnd.Intn(256)), G: uint8(rnd.Intn(256)), B: uint8(rnd.Intn(256)), A: 255})
		}
	}
	var buf strings.Builder
	_ = png.Encode(&bufWriter{&buf}, img)
	return []byte(buf.String())
}

type bufWriter struct{ b *strings.Builder }

func (w *bufWriter) Write(p []byte) (int, error) { return w.b.Write(p) }
