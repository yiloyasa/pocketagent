//go:build windows

// cmd/capprobe —— dsh/hermes ACP 协议面能力探针(P1 取证,一次性;不碰生产适配器)。
//
// 取证问题(2026-10-14-agent-adapter-gap.md §三 P1 第 5/6 条):dsh/hermes 的
// compact / image / slash / text_generation / subagents 到底是「协议不支持」
// 还是「协议支持但适配器/daemon 未接」?本探针给出 live 实证,与静态证据
// (适配器源码 + vendored ACP schema)对表。
//
// 覆盖面:
//   dsh:
//     1. initialize 能力 dump(promptCapabilities.image 动态位 / sessionCapabilities);
//     2. session/new(无凭据也能建会话——key 校验在 prompt);
//     3. ACP 方法存在性逐个试探:session/set_mode|fork|delete|load(SDK v1 schema
//        有、dsh-acp 桥未注册 → 预期 -32601)、session/compact + 对照方法(协议
//        本身无此方法 → 预期 -32601);
//     4. slash 探测:session/prompt 发 "/help" —— 若被命令层分派返回命令文本,
//        若未分派则直达 LLM 报 no API key(本机路由实证过的错误形状)。
//   hermes:
//     1. initialize 能力 dump(image 位实测);
//     2. session/new;
//     3. slash 三连:"/help"(命令面清单)、"/compress"(空会话秒回 "Nothing to
//        compress",不调 LLM —— compact 经 slash 可达的实证)、"/context";
//     4. 方法存在性:session/compact(预期 -32601)+ session/set_model 对照
//        (meta.py 注册、server.py 已实现)+ 未知对照。
//
// 红线(AGENTS.md §4.2 + 任务书):
//   - DSH_HOME / HERMES_HOME 全部隔离到临时目录,探针结束删除 —— 绝不读写
//     用户 ~/.dsh / ~/.hermes / %LOCALAPPDATA%\hermes;
//   - hermes 家目录只做一次只读 config.yaml 复制(隔离目录内 session/new 需要
//     模型目录),不复制 .env/凭据,不写回任何文件;
//   - 全程不发真 LLM prompt(slash 命令与方法探测均不需要凭据);
//   - 全局预算内收工,超时杀子进程。
//
// 用法(在 daemon/ 下):
//
//	C:\Go\bin\go.exe run ./cmd/capprobe -agent all
//	C:\Go\bin\go.exe run ./cmd/capprobe -agent dsh
//	C:\Go\bin\go.exe run ./cmd/capprobe -agent hermes
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/pocketagent/daemon/internal/hostenv"
)

// dshBinRel 是 dsh 在 npm 包内的 ESM 入口相对路径(与 dshprobe/dshapp 同款派生)。
const dshBinRel = "@deepseek-ai/dsh/lib/bin.js"

func main() {
	which := flag.String("agent", "all", "dsh|hermes|all")
	budget := flag.Duration("budget", 105*time.Second, "全局时间预算(到点打印已得结果并杀进程)")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *budget)
	defer cancel()

	if *which == "dsh" || *which == "all" {
		runDsh(ctx)
	}
	if ctx.Err() != nil {
		fmt.Printf("!! 全局预算(%s)耗尽,hermes 步骤跳过\n", *budget)
		return
	}
	if *which == "hermes" || *which == "all" {
		runHermes(ctx)
	}
	if ctx.Err() != nil {
		fmt.Printf("!! 全局预算(%s)耗尽(已在超时前收集以上结果,子进程被杀)\n", *budget)
	}
	fmt.Println("== capprobe 完成 ==")
}

// ---- 通用 JSON-RPC over stdio 客户端(仿 dshprobe:手写、零外部依赖) ----

type rpcMsg struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   json.RawMessage `json:"error,omitempty"`
}

func (m rpcMsg) idInt() (int, bool) {
	var id int
	if err := json.Unmarshal(m.ID, &id); err != nil {
		return 0, false
	}
	return id, true
}

// clip 报告用原文限长。
func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "...[截断]"
}

type rpcClient struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	out    *bufio.Reader
stderr  []string

	mu      sync.Mutex
	nextID  int
	pending map[int]chan rpcMsg
	notes   []noteRec
	closed  bool
}

type noteRec struct {
	at  time.Time
	msg rpcMsg
	raw string // 非 JSON 行原文(诊断用)
}

// launch 拉起子进程并开读帧循环。防闪窗内联 CREATE_NO_WINDOW(dshprobe 同款;
// 生产路径必须用 internal/hidewin,探针为零依赖内联同语义)。
func launch(exe string, args []string, dir string, env []string) (*rpcClient, error) {
	cmd := exec.Command(exe, args...)
	cmd.Dir = dir
	if env != nil {
		cmd.Env = env
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	c := &rpcClient{cmd: cmd, stdin: stdin, out: bufio.NewReaderSize(stdout, 8<<20), pending: map[int]chan rpcMsg{}}
	go func() {
		sc := bufio.NewScanner(stderr)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		for sc.Scan() {
			c.mu.Lock()
			c.stderr = append(c.stderr, sc.Text())
			if len(c.stderr) > 40 {
				c.stderr = c.stderr[len(c.stderr)-40:]
			}
			c.mu.Unlock()
		}
	}()
	go c.readLoop()
	return c, nil
}

func (c *rpcClient) addNote(n noteRec) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.notes = append(c.notes, n)
}

// dumpNotes 打印全部留痕(含非 JSON 行),initialize 无应答时的诊断输出。
func (c *rpcClient) dumpNotes(label string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.notes) == 0 {
		fmt.Printf("  [%s 留痕] 空(子进程没发任何帧)\n", label)
		return
	}
	fmt.Printf("  [%s 留痕 %d 条]\n", label, len(c.notes))
	for _, n := range c.notes {
		if n.raw != "" {
			fmt.Println("    raw: " + clip(n.raw, 200))
			continue
		}
		fmt.Printf("    id=%s method=%s params=%s result=%s err=%s\n",
			string(n.msg.ID), n.msg.Method, clip(string(n.msg.Params), 120),
			clip(string(n.msg.Result), 120), clip(string(n.msg.Error), 120))
	}
}

func (c *rpcClient) readLoop() {
	for {
		line, err := c.out.ReadBytes('\n')
		if len(line) == 0 {
			if err != nil {
				return
			}
			continue
		}
		var m rpcMsg
		if json.Unmarshal(line, &m) != nil {
			// 非 JSON 行(横幅/日志)留痕,便于诊断。
			c.addNote(noteRec{raw: string(line)})
			continue
		}
		if m.Method != "" && m.ID != nil {
			// 服务端反向请求:探针路径不应出现,留痕即可。
			c.addNote(noteRec{at: time.Now(), msg: m})
			continue
		}
		if id, ok := m.idInt(); ok {
			c.mu.Lock()
			ch := c.pending[id]
			delete(c.pending, id)
			c.mu.Unlock()
			if ch != nil {
				ch <- m
			}
			continue
		}
		c.addNote(noteRec{at: time.Now(), msg: m})
	}
}

func (c *rpcClient) call(method string, params any, timeout time.Duration) (rpcMsg, error) {
	body, err := json.Marshal(params)
	if err != nil {
		return rpcMsg{}, err
	}
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	ch := make(chan rpcMsg, 1)
	c.pending[id] = ch
	c.mu.Unlock()

	// 注意:body 是 json.RawMessage(不是 []byte)——map 里的 []byte 会被
	// encoding/json 编码成 base64 字符串,RawMessage 才按原样内联。
	frame, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": json.RawMessage(body)})
	fmt.Printf("  → %s %s\n", method, clip(string(frame), 200))
	if _, err := c.stdin.Write(append(frame, '\n')); err != nil {
		return rpcMsg{}, fmt.Errorf("写入 %s 失败: %w", method, err)
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case m := <-ch:
		return m, nil
	case <-timer.C:
		return rpcMsg{}, fmt.Errorf("%s: 等待应答超时(%s)", method, timeout)
	}
}

// waitNote 等一条匹配的通知(轮询 notes;探针粒度足够)。
func (c *rpcClient) waitNote(pred func(rpcMsg) bool, timeout time.Duration) (rpcMsg, bool) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		c.mu.Lock()
		for i, n := range c.notes {
			if pred(n.msg) {
				c.notes = append(c.notes[:i], c.notes[i+1:]...)
				c.mu.Unlock()
				return n.msg, true
			}
		}
		c.mu.Unlock()
		time.Sleep(50 * time.Millisecond)
	}
	return rpcMsg{}, false
}

// noteVariants 快照当前已收通知的 sessionUpdate 变体计数。
func (c *rpcClient) noteVariants() map[string]int {
	out := map[string]int{}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, n := range c.notes {
		var env struct {
			Update struct {
				Variant string `json:"sessionUpdate"`
			} `json:"update"`
		}
		if json.Unmarshal(n.msg.Params, &env) == nil && env.Update.Variant != "" {
			out[env.Update.Variant]++
		}
	}
	return out
}

func (c *rpcClient) printVariants() {
	v := c.noteVariants()
	if len(v) == 0 {
		fmt.Println("  [通知] 无 session/update 通知收到")
		return
	}
	keys := make([]string, 0, len(v))
	for k := range v {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s×%d", k, v[k]))
	}
	fmt.Printf("  [通知变体] %s\n", strings.Join(parts, ", "))
}

func (c *rpcClient) close() {
	if c == nil {
		return
	}
	_ = c.stdin.Close()
	if c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
	_ = c.cmd.Wait()
}

func printStderrTail(c *rpcClient, label string, n int) {
	if len(c.stderr) == 0 {
		return
	}
	tail := c.stderr
	if len(tail) > n {
		tail = tail[len(tail)-n:]
	}
	fmt.Printf("  [%s stderr 尾部]\n", label)
	for _, l := range tail {
		fmt.Println("    " + clip(l, 220))
	}
}

func printResult(m rpcMsg, label string) {
	if len(m.Error) > 0 {
		fmt.Printf("  ← %s 错误: %s\n", label, clip(string(m.Error), 300))
		return
	}
	fmt.Printf("  ← %s 应答: %s\n", label, clip(string(m.Result), 400))
}

// ---- dsh 侧 ----

func runDsh(ctx context.Context) {
	fmt.Println("==== dsh ACP 协议面探针 ====")
	t0 := time.Now()

	node, bin, err := locateDsh()
	if err != nil {
		fmt.Println("[dsh] binary-not-found:", err)
		return
	}
	fmt.Printf("[dsh] locate node=%s bin=%s\n", node, bin)

	// DSH_HOME 隔离(红线:绝不碰 ~/.dsh)。
	home, err := os.MkdirTemp("", "capprobe-dsh-home-*")
	if err != nil {
		fmt.Println("[dsh] 临时目录失败:", err)
		return
	}
	defer os.RemoveAll(home)
	ws, err := os.MkdirTemp("", "capprobe-dsh-ws-*")
	if err != nil {
		fmt.Println("[dsh] 工作区失败:", err)
		return
	}
	defer os.RemoveAll(ws)

	exe, args := "node", []string{bin, "--profile", "acp"}
	if isCmdScript(bin) {
		exe, args = "cmd.exe", append([]string{"/c", bin}, "--profile", "acp")
	}
	env := filteredEnv("DEEPSEEK_API_KEY", "DSH_HOME="+home)

	c, err := launch(exe, args, "", env)
	if err != nil {
		fmt.Println("[dsh] 启动失败:", err)
		return
	}
	defer c.close()

	// 1. initialize 能力 dump。
	m, err := c.call("initialize", map[string]any{
		"protocolVersion": 1,
		"clientCapabilities": map[string]any{
			"fs": map[string]bool{"readTextFile": false, "writeTextFile": false},
		},
	}, 30*time.Second)
	if err != nil {
		fmt.Println("[dsh] initialize 失败:", err)
		c.dumpNotes("dsh")
		printStderrTail(c, "dsh", 12)
		return
	}
	printResult(m, "initialize")
	fmt.Printf("  (耗时 %s)\n", time.Since(t0).Round(time.Millisecond))

	// 2. session/new。
	m, err = c.call("session/new", map[string]any{"cwd": ws, "mcpServers": []any{}}, 15*time.Second)
	if err != nil {
		fmt.Println("[dsh] session/new 失败:", err)
		printStderrTail(c, "dsh", 12)
		return
	}
	var st struct {
		SessionID string `json:"sessionId"`
	}
	_ = json.Unmarshal(m.Result, &st)
	fmt.Printf("  ← session/new sessionId=%s\n", st.SessionID)
	if st.SessionID == "" {
		printResult(m, "session/new(raw)")
		printStderrTail(c, "dsh", 12)
		return
	}

	// 3. 方法存在性试探。
	//    判读:-32601 Method not found = 桥/协议未注册该方法;
	//         其他(-32602/-32603/成功)= 方法已注册(存在性成立)。
	methodProbes := []struct {
		method string
		params map[string]any
	}{
		{"session/set_mode", map[string]any{"sessionId": "probe-no-such", "modeId": "x"}}, // SDK v1 schema 有
		{"session/fork", map[string]any{"sessionId": "probe-no-such", "cwd": ws}},         // SDK v1 schema 有
		{"session/delete", map[string]any{"sessionId": "probe-no-such"}},                  // SDK v1 schema 有
		{"session/load", map[string]any{"sessionId": "probe-no-such", "cwd": ws}},         // SDK v1 schema 有
		{"session/compact", map[string]any{"sessionId": st.SessionID}},                    // 协议无此方法
		{"session/nonexistent_control", map[string]any{"sessionId": st.SessionID}},        // 对照:必然 -32601
	}
	fmt.Println("[dsh] 方法存在性试探(预期未注册=-32601;已注册=参数校验错或成功):")
	for _, p := range methodProbes {
		m, err := c.call(p.method, p.params, 8*time.Second)
		if err != nil {
			fmt.Printf("  %s: %v\n", p.method, err)
			continue
		}
		printResult(m, p.method)
	}

	// 4. slash 探测:"/help" 文本。dsh-acp 桥静态未见 slash 分派 → 预期直达 LLM,
	//    无凭据时报 no API key(-32603)。若返回命令文本 = 有分派。
	fmt.Println(`[dsh] slash 探测:session/prompt "/help"(无凭据):`)
	m, err = c.call("session/prompt", map[string]any{
		"sessionId": st.SessionID,
		"prompt":    []map[string]any{{"type": "text", "text": "/help"}},
	}, 20*time.Second)
	if err != nil {
		fmt.Println("[dsh] /help:", err)
	} else {
		printResult(m, "/help")
	}
	c.printVariants()
	printStderrTail(c, "dsh", 8)
	fmt.Printf("[dsh] 步骤完成,总耗时 %s\n\n", time.Since(t0).Round(time.Millisecond))
}

func locateDsh() (node, bin string, err error) {
	bin = strings.TrimSpace(os.Getenv("DSH_BIN"))
	if bin == "" {
		bin = hostenv.FirstFile(hostenv.PackageCandidates(dshBinRel))
	}
	if bin == "" {
		for _, name := range []string{"dsh.exe", "dsh", "dsh.cmd"} {
			if p, lookErr := exec.LookPath(name); lookErr == nil {
				bin = p
				break
			}
		}
	}
	if bin == "" {
		return "", "", errors.New("未找到 dsh 入口(可用 DSH_BIN 指定 dsh.cmd 或 lib/bin.js)")
	}
	node = strings.TrimSpace(os.Getenv("DSH_NODE"))
	if node == "" {
		node = hostenv.FindNode()
	}
	if node == "" {
		return "", "", errors.New("未找到 node(可用 DSH_NODE 指定路径)")
	}
	return node, bin, nil
}

func isCmdScript(p string) bool {
	l := strings.ToLower(p)
	return strings.HasSuffix(l, ".cmd") || strings.HasSuffix(l, ".bat")
}

// filteredEnv 继承当前环境,剔除指定 key,追加额外赋值。
func filteredEnv(dropKeys ...string) []string {
	drop := map[string]bool{}
	var extra []string
	for _, k := range dropKeys {
		if i := strings.IndexByte(k, '='); i >= 0 {
			drop[strings.ToUpper(k[:i])] = true
			extra = append(extra, k)
		} else {
			drop[strings.ToUpper(k)] = true
		}
	}
	var out []string
	for _, e := range os.Environ() {
		if i := strings.IndexByte(e, '='); i >= 0 && drop[strings.ToUpper(e[:i])] {
			continue
		}
		out = append(out, e)
	}
	return append(out, extra...)
}

// ---- hermes 侧 ----

func runHermes(ctx context.Context) {
	fmt.Println("==== hermes ACP 协议面探针 ====")

	py, acpDir, err := locateHermes()
	if err != nil {
		fmt.Println("[hermes] binary-not-found:", err)
		return
	}
	fmt.Printf("[hermes] locate python=%s acp=%s\n", py, acpDir)

	// HERMES_HOME 隔离(红线:绝不写用户家)。两处只读取材到隔离目录/子进程 env:
	//   - config.yaml:一次性只读复制(session/new 的模型目录需要);
	//   - .env 的 DEEPSEEK_API_KEY:只读解析后经子进程 env 注入(与 daemon 生产
	//     行为同款——凭据只透传 env,不落盘、不写回、不打印)。
	home, err := os.MkdirTemp("", "capprobe-hermes-home-*")
	if err != nil {
		fmt.Println("[hermes] 临时目录失败:", err)
		return
	}
	defer os.RemoveAll(home)
	copyHermesConfig(home)
	ws, err := os.MkdirTemp("", "capprobe-hermes-ws-*")
	if err != nil {
		fmt.Println("[hermes] 工作区失败:", err)
		return
	}
	defer os.RemoveAll(ws)

	hermesLaunch(py, acpDir, home, ws)
}

// hermesLaunch 拉起 hermes 适配器并跑完整探测链(独立出来便于注入 env)。
func hermesLaunch(py, acpDir, home, ws string) {
	t0 := time.Now()
	env := filteredEnv("HERMES_HOME="+home)
	if realHome := realHermesHome(); realHome != "" {
		if key := readEnvFile(filepath.Join(realHome, ".env"), "DEEPSEEK_API_KEY"); key != "" {
			// 只透传 env,不落盘不打印(dshapp 凭据纪律同款)。
			env = append(env, "DEEPSEEK_API_KEY="+key)
			fmt.Println("[hermes] 已从真实家目录 .env 只读提取 DEEPSEEK_API_KEY 经 env 注入(值不落盘不打印)")
		}
	}
	c, err := launch(py, []string{"-m", "acp_adapter"}, acpDir, env)
	if err != nil {
		fmt.Println("[hermes] 启动失败:", err)
		return
	}
	defer c.close()

	// 1. initialize。
	m, err := c.call("initialize", map[string]any{
		"protocolVersion": 1,
		"clientCapabilities": map[string]any{
			"fs": map[string]bool{"readTextFile": false, "writeTextFile": false},
		},
	}, 40*time.Second)
	if err != nil {
		fmt.Println("[hermes] initialize 失败:", err)
		printStderrTail(c, "hermes", 15)
		return
	}
	printResult(m, "initialize")
	fmt.Printf("  (冷启+握手耗时 %s)\n", time.Since(t0).Round(time.Millisecond))

	// 2. session/new。
	m, err = c.call("session/new", map[string]any{"cwd": ws, "mcpServers": []any{}}, 40*time.Second)
	if err != nil {
		fmt.Println("[hermes] session/new 失败:", err)
		printStderrTail(c, "hermes", 15)
		return
	}
	var st struct {
		SessionID string `json:"sessionId"`
	}
	_ = json.Unmarshal(m.Result, &st)
	fmt.Printf("  ← session/new sessionId=%s\n", st.SessionID)
	if st.SessionID == "" {
		printResult(m, "session/new(raw)")
		printStderrTail(c, "hermes", 15)
		return
	}

	// 3. slash 三连(全部不需要 LLM):
	//    /help    → 命令面清单(适配器 slash 分派存在的实证);
	//    /compress→ 空会话秒回 "Nothing to compress"(compact 经 slash 可达;
	//               LLM 只在有历史时才被调用);
	//    /context → 上下文信息。
	for _, cmdText := range []string{"/help", "/compress", "/context"} {
		fmt.Printf("[hermes] slash 探测:%s\n", cmdText)
		before := len(c.notes)
		m, err := c.call("session/prompt", map[string]any{
			"sessionId": st.SessionID,
			"prompt":    []map[string]any{{"type": "text", "text": cmdText}},
		}, 30*time.Second)
		if err != nil {
			fmt.Println("  err:", err)
			continue
		}
		printResult(m, cmdText)
		// 摘要该命令收到的 agent_message_chunk 正文。
		if txt := collectMessageChunks(c, before); txt != "" {
			fmt.Printf("  [agent_message_chunk] %s\n", clip(strings.TrimSpace(txt), 500))
		}
		c.dropNotes(before)
	}

	// 4. 方法存在性试探。
	fmt.Println("[hermes] 方法存在性试探:")
	methodProbes := []struct {
		method string
		params map[string]any
	}{
		{"session/compact", map[string]any{"sessionId": st.SessionID}},             // 协议无此方法
		{"session/set_model", map[string]any{"sessionId": "probe-no-such", "modelId": "x"}}, // 已注册对照
		{"session/nonexistent_control", map[string]any{"sessionId": st.SessionID}},          // 对照:必然 -32601
	}
	for _, p := range methodProbes {
		m, err := c.call(p.method, p.params, 15*time.Second)
		if err != nil {
			fmt.Printf("  %s: %v\n", p.method, err)
			continue
		}
		printResult(m, p.method)
	}
	c.printVariants()
	printStderrTail(c, "hermes", 10)
	fmt.Printf("[hermes] 步骤完成,总耗时 %s\n", time.Since(t0).Round(time.Millisecond))
}

// realHermesHome 返回真实 hermes 家目录候选中的第一个含 config.yaml 的目录
// (只读探查):HERMES_HOME env → %LOCALAPPDATA%\hermes → ~/.hermes。
// copyHermesConfig 与凭据提取共用同一事实来源。
func realHermesHome() string {
	var candidates []string
	if h := strings.TrimSpace(os.Getenv("HERMES_HOME")); h != "" {
		candidates = append(candidates, h)
	}
	if la := os.Getenv("LOCALAPPDATA"); la != "" {
		candidates = append(candidates, filepath.Join(la, "hermes"))
	}
	if uh, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(uh, ".hermes"))
	}
	for _, dir := range candidates {
		if st, err := os.Stat(filepath.Join(dir, "config.yaml")); err == nil && !st.IsDir() {
			return dir
		}
	}
	return ""
}

// readEnvKey 从 .env 文本只读解析一个 KEY=VALUE(值去引号;不打印)。
func readEnvFile(path, key string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		i := strings.IndexByte(line, '=')
		if i < 0 || strings.TrimSpace(line[:i]) != key {
			continue
		}
		return strings.Trim(strings.TrimSpace(line[i+1:]), `"'`)
	}
	return ""
}

// copyHermesConfig 从真实 hermes 家目录只读复制 config.yaml 到隔离目录。
// 找不到就跳过(隔离目录内 session/new 可能失败,如实记录)。
func copyHermesConfig(dstHome string) {
	dir := realHermesHome()
	if dir == "" {
		fmt.Println("[hermes] 未找到真实家目录的 config.yaml(隔离目录从零开始,session/new 可能失败——如实记录)")
		return
	}
	src := filepath.Join(dir, "config.yaml")
	data, err := os.ReadFile(src) // 只读
	if err != nil {
		fmt.Println("[hermes] config.yaml 读取失败:", err)
		return
	}
	if err := os.WriteFile(filepath.Join(dstHome, "config.yaml"), data, 0o600); err != nil {
		fmt.Println("[hermes] config.yaml 复制失败:", err)
		return
	}
	fmt.Printf("[hermes] 已只读复制 config.yaml(%d 字节,来源 %s)到隔离 HERMES_HOME\n", len(data), src)
}

// locateHermes 与 internal/hermesapp.LocateHermes 同款派生(探针零依赖内联):
// HERMES_NPM_ROOT → npm 全局根下的 hermes-agent 包,venv python 必须真实在场。
func locateHermes() (py, acpDir string, err error) {
	var roots []string
	if r := strings.TrimSpace(os.Getenv("HERMES_NPM_ROOT")); r != "" {
		roots = append(roots, r)
	}
	roots = append(roots, hostenv.NpmGlobalRoots()...)
	for _, root := range roots {
		pkg := filepath.Join(root, "hermes-agent")
		p := filepath.Join(pkg, "runtime", "hermes-agent", "venv", "Scripts", "python.exe")
		if st, statErr := os.Stat(p); statErr == nil && !st.IsDir() {
			return p, filepath.Join(pkg, "runtime", "hermes-agent"), nil
		}
	}
	return "", "", errors.New("未找到 Hermes Agent 运行时(python -m acp_adapter 的 venv python;可用 HERMES_NPM_ROOT 指定包根)")
}

// collectMessageChunks 汇总 [from:] 之后通知里的 agent_message_chunk 正文。
func collectMessageChunks(c *rpcClient, from int) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var b strings.Builder
	if from > len(c.notes) {
		return ""
	}
	for _, n := range c.notes[from:] {
		var env struct {
			Update struct {
				Variant string `json:"sessionUpdate"`
				Content struct {
					Text string `json:"text"`
				} `json:"content"`
			} `json:"update"`
		}
		if json.Unmarshal(n.msg.Params, &env) == nil && env.Update.Variant == "agent_message_chunk" {
			b.WriteString(env.Update.Content.Text)
		}
	}
	return b.String()
}

// dropNotes 丢弃 [from:] 之后的通知(每步 slash 后清场,变体计数不跨步膨胀)。
func (c *rpcClient) dropNotes(from int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if from > len(c.notes) {
		return
	}
	c.notes = c.notes[:from]
}
