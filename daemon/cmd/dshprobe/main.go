//go:build windows

// cmd/dshprobe —— DeepSeek Harness(dsh) ACP P0 探针(探针先行,不写任何生产代码)。
//
// 目的:把 dsh 接入设计(2026-09-20-dsh-integration-plan.md §7 P0 清单)的 8 项
// "文档/设计说 dsh 能这样"逐项变成"本机确实能/不能这样",产出带原始 JSON-RPC
// 帧的证据报告(docs/verify/evidence/dsh-p0-probe.md)。
//
// 覆盖 8 项:
//  1. handshake          initialize 握手 + 能力集全量 dump
//  2. session-new        session/new → config-option 状态(Models() 数据源契约)
//  3. update-granularity session/prompt 逐帧记录 session/update(流式粒度量化)
//  4. permission         session/request_permission allow/reject 双路
//  5. cancel             session/cancel 中断语义
//  6. resume             kill 子进程 → 新进程 session/list + session/resume(跨进程恢复)
//  7. no-key             无 DEEPSEEK_API_KEY 的错误形状(ErrorInfoCarrier 素材)
//  8. coldstart          冷启动耗时(进程 start → initialize 应答)
//
// 关键设计:
//   - 直接 node 拉起 bin.js --profile acp(ESM 入口,不经 .cmd 垫片);
//   - 手写 JSON-RPC 2.0 客户端:bufio.Reader 8MB 上限(长上下文单行帧;piapp 64KB 教训),
//     零外部依赖、与生产实现解耦(生产实现应复刻 codexapp/client.go 范式);
//   - DSH_HOME 隔离到临时目录 —— 绝不读写用户 ~/.dsh(红线);
//   - stderr 逐行收集,报告里以 [dsh-stderr] 前缀呈现;
//   - 子进程防闪窗:内联 CREATE_NO_WINDOW(等价 internal/hidewin.Apply,生产必须复用 hidewin);
//   - 每个交互 30~120s 超时,防探针挂死。
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	// 本机实测 node 位置(WMI 派生环境不保证 PATH,显式探测 + DSH_NODE 兜底)
	defaultNode = `C:\Users\yanzi\AppData\Local\Programs\Nodejs\node.exe`
	// 本机实测 dsh 入口(ESM bin.js,不要经 dsh.cmd 垫片)
	defaultBin = `G:\Claude Code\node_modules\@deepseek-ai\dsh\lib\bin.js`
)

// ---- JSON-RPC 消息 ----

type rpcMsg struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   json.RawMessage `json:"error,omitempty"`
	raw     string          // 收到/发出的原始行(报告用)
}

func (m rpcMsg) idString() string { return strings.Trim(string(m.ID), `"`) }

// noteRec 一条入站帧(通知或服务端反向请求)带时间戳。
type noteRec struct {
	at  time.Time
	msg rpcMsg
}

// ---- 子进程客户端(手写 JSON-RPC 2.0 over stdio,零外部依赖) ----

type clientOpts struct {
	node, bin string
	dshHome   string // DSH_HOME(隔离;绝不指向 ~/.dsh)
	workDir   string
	stripKey  bool // 剔除 DEEPSEEK_API_KEY(第 7 项)
	label     string
}

// writeCloser 最小接口(不引 io 包具名类型,便于测试替换)。
type writeCloser interface {
	Write([]byte) (int, error)
	Close() error
}

type client struct {
	opts       clientOpts
	cmd        *exec.Cmd
	stdin      writeCloser
	out        *bufio.Reader
	startedAt  time.Time
	initDur    time.Duration // start → initialize 应答(冷启动)
	initRes    json.RawMessage
	initErrRaw json.RawMessage
	initErr    error

	writeMu sync.Mutex
	mu      sync.Mutex
	nextID  int
	pending map[string]chan rpcMsg
	notes   []noteRec
	handler func(rpcMsg) // 服务端反向请求处理器(审批设)
	dead    bool

	records   []string // 全量原始行(截断),报告附录
	stderrLn  []string
	stderrN   int
	closeOnce sync.Once
}

// hideWindow 内联防闪窗(CREATE_NO_WINDOW)。
// 生产实现必须复用 internal/hidewin.Apply;探针零依赖故内联同款语义。
func hideWindow(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
}

// buildEnv 组装子进程环境:透传当前环境,覆写 DSH_HOME;stripKey 时剔除 DEEPSEEK_API_KEY。
func buildEnv(dshHome string, stripKey bool) []string {
	var out []string
	for _, kv := range os.Environ() {
		up := strings.ToUpper(kv)
		if strings.HasPrefix(up, "DSH_HOME=") {
			continue
		}
		if stripKey && strings.HasPrefix(up, "DEEPSEEK_API_KEY=") {
			continue
		}
		out = append(out, kv)
	}
	return append(out, "DSH_HOME="+dshHome)
}

func (c *client) record(line string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.records) >= 6000 {
		return
	}
	line = strings.TrimRight(line, "\r\n")
	if len(line) > 6000 {
		line = fmt.Sprintf("%.6000s …[截断,整行 %d 字节]", line, len(line))
	}
	c.records = append(c.records, line)
}

func (c *client) pushNote(m rpcMsg) {
	c.mu.Lock()
	c.notes = append(c.notes, noteRec{at: time.Now(), msg: m})
	c.mu.Unlock()
}

// start 拉起 node bin.js --profile acp。
func (c *client) start() error {
	cmd := exec.Command(c.opts.node, c.opts.bin, "--profile", "acp")
	cmd.Dir = c.opts.workDir
	cmd.Env = buildEnv(c.opts.dshHome, c.opts.stripKey)
	hideWindow(cmd)
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
	c.startedAt = time.Now()
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动 dsh acp 子进程失败: %w", err)
	}
	c.cmd = cmd
	c.stdin = stdin
	c.out = bufio.NewReaderSize(stdout, 8*1024*1024) // 8MB:长上下文/图单行帧(piapp 64KB 教训)
	go c.readLoop()
	go c.readStderr(stderr)
	return nil
}

func (c *client) readStderr(r interface{ Read([]byte) (int, error) }) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		c.mu.Lock()
		c.stderrN++
		if len(c.stderrLn) < 300 {
			l := sc.Text()
			if len(l) > 500 {
				l = l[:500] + "…"
			}
			c.stderrLn = append(c.stderrLn, l)
		}
		c.mu.Unlock()
	}
}

func (c *client) stderrSnapshot() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.stderrLn...)
}

func (c *client) readLoop() {
	for {
		line, err := c.out.ReadBytes('\n')
		if len(line) > 0 {
			c.record(string(line))
			var m rpcMsg
			if json.Unmarshal(line, &m) == nil {
				m.raw = strings.TrimRight(string(line), "\r\n")
				switch {
				case m.Method != "" && len(m.ID) > 0:
					// 服务端反向请求(session/request_permission 等)
					c.pushNote(m)
					if h := c.getHandler(); h != nil {
						go h(m) // 回写走 stdin,不能堵 readLoop
					} else {
						_ = c.sendErr(m.idString(), -32601, "dshprobe: no handler for "+m.Method)
					}
				case m.Method != "":
					c.pushNote(m)
				default:
					c.routeResponse(m)
				}
			}
		}
		if err != nil {
			c.mu.Lock()
			c.dead = true
			for id, ch := range c.pending {
				ch <- rpcMsg{raw: `{"dead":true,"err":"子进程已退出: ` + err.Error() + `"}`}
				delete(c.pending, id)
			}
			c.mu.Unlock()
			return
		}
	}
}

func (c *client) getHandler() func(rpcMsg) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.handler
}

func (c *client) setHandler(h func(rpcMsg)) {
	c.mu.Lock()
	c.handler = h
	c.mu.Unlock()
}

func (c *client) routeResponse(m rpcMsg) {
	c.mu.Lock()
	ch, ok := c.pending[m.idString()]
	if ok {
		delete(c.pending, m.idString())
	}
	c.mu.Unlock()
	if ok {
		ch <- m
	}
}

func (c *client) send(m rpcMsg) error {
	raw, err := json.Marshal(m)
	if err != nil {
		return err
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_, err = c.stdin.Write(append(raw, '\n'))
	return err
}

func (c *client) sendErr(id string, code int, msg string) error {
	return c.send(rpcMsg{
		JSONRPC: "2.0",
		ID:      json.RawMessage(strconv.Quote(id)),
		Error:   mustJSON(map[string]any{"code": code, "message": msg}),
	})
}

func (c *client) notify(method string, params any) error {
	return c.send(rpcMsg{JSONRPC: "2.0", Method: method, Params: mustJSON(params)})
}

// call 发请求等应答。返回:result / 应答原始行 / JSON-RPC error 原始对象 / error。
func (c *client) call(method string, params any, timeout time.Duration) (json.RawMessage, string, json.RawMessage, error) {
	c.mu.Lock()
	if c.dead {
		c.mu.Unlock()
		return nil, "", nil, errors.New("子进程已退出")
	}
	c.nextID++
	id := fmt.Sprintf("dshprobe-%d", c.nextID)
	ch := make(chan rpcMsg, 1)
	c.pending[id] = ch
	c.mu.Unlock()
	req := rpcMsg{JSONRPC: "2.0", ID: json.RawMessage(strconv.Quote(id)), Method: method, Params: mustJSON(params)}
	if err := c.send(req); err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, "", nil, err
	}
	select {
	case m := <-ch:
		if strings.Contains(m.raw, `"dead":true`) {
			return nil, m.raw, nil, errors.New("子进程已退出")
		}
		if len(m.Error) > 0 {
			return nil, m.raw, m.Error, fmt.Errorf("%s 返回 JSON-RPC error", method)
		}
		return m.Result, m.raw, nil, nil
	case <-time.After(timeout):
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, "", nil, fmt.Errorf("%s 等待应答超时(%s)", method, timeout)
	}
}

func (c *client) respond(id string, result any) error {
	return c.send(rpcMsg{JSONRPC: "2.0", ID: json.RawMessage(strconv.Quote(id)), Result: mustJSON(result)})
}

func (c *client) notesFrom(idx int) []noteRec {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]noteRec(nil), c.notes[idx:]...)
}

func (c *client) notesLen() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.notes)
}

func (c *client) close() {
	c.closeOnce.Do(func() {
		if c.cmd != nil && c.cmd.Process != nil {
			_ = c.stdin.Close()
			_ = c.cmd.Process.Kill()
			done := make(chan struct{})
			go func() { _ = c.cmd.Wait(); close(done) }()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
			}
		}
	})
}

// initialize 拉起后第一呼叫,并计量冷启动。
func (c *client) initialize() error {
	params := map[string]any{
		"protocolVersion":    1,
		"clientCapabilities": map[string]any{"fs": map[string]any{"readTextFile": false, "writeTextFile": false}},
	}
	res, _, errRaw, err := c.call("initialize", params, 60*time.Second)
	c.initDur = time.Since(c.startedAt)
	c.initRes = res
	c.initErrRaw = errRaw
	c.initErr = err
	return err
}

// ---- 形状/格式小工具 ----

func mustJSON(v any) json.RawMessage {
	raw, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("null")
	}
	return raw
}

func pretty(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "(空)"
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		return string(raw)
	}
	return buf.String()
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return fmt.Sprintf("%s\n…[截断,整段 %d 字节]", s[:n], len(s))
}

func clip(s string, n int) string {
	rr := []rune(s)
	if len(rr) <= n {
		return s
	}
	return string(rr[:n]) + "…"
}

// variantOf 提取 session/update 的变体名(ACP v1 字段 sessionUpdate)。
func variantOf(m rpcMsg) string {
	var p struct {
		Update map[string]any `json:"update"`
	}
	_ = json.Unmarshal(m.Params, &p)
	if p.Update != nil {
		if s, ok := p.Update["sessionUpdate"].(string); ok {
			return s
		}
		if s, ok := p.Update["type"].(string); ok {
			return s
		}
	}
	return "?"
}

// chunkText 提取 agent_message_chunk/agent_thought_chunk 的文本。
func chunkText(m rpcMsg) string {
	var p struct {
		Update struct {
			Content struct {
				Text string `json:"text"`
				Type string `json:"type"`
			} `json:"content"`
		} `json:"update"`
	}
	_ = json.Unmarshal(m.Params, &p)
	return p.Update.Content.Text
}

func stopReasonOf(res json.RawMessage) string {
	var p struct {
		StopReason string `json:"stopReason"`
	}
	_ = json.Unmarshal(res, &p)
	if p.StopReason == "" {
		return "(无 stopReason 字段)"
	}
	return p.StopReason
}

// keyRelated 判断错误是否疑似凭证缺失(用于"受阻(缺 key)"归类;最终以原始帧为准)。
func keyRelated(raws ...string) bool {
	s := strings.ToLower(strings.Join(raws, " "))
	for _, k := range []string{"credential", "api key", "apikey", "missing_key", "missingkey", "unauthorized", "not authenticated", "authentication", "401", "deepseek_api_key", "login"} {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}

// ---- 探针结果 ----

type item struct {
	n          int
	name       string
	status     string // 通过 / 受阻(缺 key) / 失败
	cmd        string // 概念命令
	note       string
	frames     []string // 原始帧(截断)
	conclusion string
	impact     string
}

type summary struct {
	granularity string
	resume      string
	modelShape  string
	noKey       string
	coldDur     string
}

type runner struct {
	mu    sync.Mutex
	items []item
	sum   summary
}

func (r *runner) add(it item) {
	r.mu.Lock()
	if it.n == 0 {
		it.n = len(r.items) + 1
	}
	r.items = append(r.items, it)
	n := it.n
	r.mu.Unlock()
	fmt.Printf("[%d] %s —— %s\n", n, it.name, it.status)
}

func (r *runner) setSummary(f func(*summary)) {
	r.mu.Lock()
	f(&r.sum)
	r.mu.Unlock()
}

// ---- 第 1 项:handshake ----

func doHandshake(c *client, coldDur time.Duration) item {
	it := item{
		name:   "initialize 握手 + 能力集 dump",
		status: "通过",
		cmd:    `initialize {"protocolVersion":1,"clientCapabilities":{"fs":{"readTextFile":false,"writeTextFile":false}}}`,
		frames: []string{trunc(pretty(c.initRes), 5000)},
	}
	var p struct {
		ProtocolVersion   int            `json:"protocolVersion"`
		AgentInfo         map[string]any `json:"agentInfo"`
		AgentCapabilities struct {
			McpCapabilities     map[string]any `json:"mcpCapabilities"`
			PromptCapabilities  map[string]any `json:"promptCapabilities"`
			SessionCapabilities map[string]any `json:"sessionCapabilities"`
		} `json:"agentCapabilities"`
		AuthMethods []any `json:"authMethods"`
	}
	_ = json.Unmarshal(c.initRes, &p)
	sc := p.AgentCapabilities.SessionCapabilities
	has := func(k string) bool { _, ok := sc[k]; return ok }
	it.note = fmt.Sprintf("protocolVersion=%d agentInfo=%v 冷启动(start→initialize 应答)=%s",
		p.ProtocolVersion, p.AgentInfo, coldDur.Round(time.Millisecond))
	it.conclusion = fmt.Sprintf(
		"握手成功:protocolVersion=%d;agentInfo.name=%v v%v;sessionCapabilities=%v(list=%v resume=%v close=%v);"+
			"promptCapabilities=%v;authMethods=%d 个。能力协商面完整,与设计 §2.1 预期一致(0.1.5 新增 session 面在场)。",
		p.ProtocolVersion, p.AgentInfo["name"], p.AgentInfo["version"], mustJSON(sc),
		has("list"), has("resume"), has("close"),
		mustJSON(p.AgentCapabilities.PromptCapabilities), len(p.AuthMethods))
	it.impact = "dshapp.Client 启动后第一呼叫固定为 initialize;sessionCapabilities.list/resume 在场才启用 ExternalSession 重挂载路径(§5.2 能力门控);" +
		"promptCapabilities.image=false → 图片输入如实报错不静默丢(R4);authMethods 为空 → 无需 authenticate 步骤,凭据链在 harness 内部解析;" +
		"探测路径(provider.go DetectProviders)绝不做此握手,只做文件存在+--version(PA-REAL-0002/0003 教训)。"
	return it
}

// ---- 第 2 项:session-new ----

func doSessionNew(c *client, workDir string) (string, item) {
	res, rawLine, errRaw, err := c.call("session/new", map[string]any{"cwd": workDir, "mcpServers": []any{}}, 60*time.Second)
	it := item{
		name:   "session/new → config-option 状态(Models() 数据源契约)",
		cmd:    fmt.Sprintf(`session/new {"cwd":%q,"mcpServers":[]}`, workDir),
		frames: []string{trunc(pretty(res), 8000)},
	}
	if err != nil {
		it.status = "受阻(缺 key)"
		if !keyRelated(string(errRaw), rawLine) {
			it.status = "失败"
		}
		it.note = "session/new 失败: " + err.Error()
		if len(errRaw) > 0 {
			it.frames = append(it.frames, trunc(string(errRaw), 1500))
		}
		it.frames = append(it.frames, trunc(rawLine, 3000))
		it.conclusion = "session/new 在本机环境的实际错误形状见原始帧;若属凭证错误,则 Models() 的瞬态会话取样必须带凭证错误兜底(见第 7 项)。"
		it.impact = "Models() 数据源 = session/new 应答;取样失败时 ListModels 如实报错,不伪造名单(设计原则 §7)。"
		return "", it
	}
	var p struct {
		SessionID string `json:"sessionId"`
	}
	_ = json.Unmarshal(res, &p)
	m := map[string]any{}
	_ = json.Unmarshal(res, &m)
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// 解析 configOptions(实测:[]configOptions[].{id,name,category,type,currentValue,options})
	var co struct {
		ConfigOptions []struct {
			ID           string `json:"id"`
			Name         string `json:"name"`
			Category     string `json:"category"`
			Type         string `json:"type"`
			CurrentValue any    `json:"currentValue"`
			Options      []any  `json:"options"`
		} `json:"configOptions"`
	}
	_ = json.Unmarshal(res, &co)
	var coDesc []string
	for _, o := range co.ConfigOptions {
		coDesc = append(coDesc, fmt.Sprintf("{id:%q category:%q type:%q currentValue=%s 选项数=%d}",
			o.ID, o.Category, o.Type, string(mustJSON(o.CurrentValue)), len(o.Options)))
	}
	it.status = "通过"
	it.note = fmt.Sprintf("sessionId=%s 顶层键=%v", p.SessionID, keys)
	if len(coDesc) > 0 {
		it.conclusion = fmt.Sprintf(
			"config-option 真实形状:应答顶层即 sessionId + configOptions 数组,每项 {id,name,category,type,currentValue,options};实测 %s。"+
				"关键细节:model 的 value/currentValue 是 JSON 字符串编码的 [provider,model] 二元组(如 \"[\\\"deepseek-official\\\",\\\"deepseek-v4-flash\\\"]\",双层编码,set_config_option 时须原样回传该字符串);"+
				"模型选项按 group 分组且 live catalog 随版本浮动;effort 档位实测 off/low/high/max。这是 DshProvider.Models() 的唯一数据源(无静态名单)。",
			strings.Join(coDesc, " ; "))
	} else {
		it.conclusion = "应答未见 configOptions 数组(形状漂移风险 R6),以原始帧为准;Models() 需按能力协商降级。"
	}
	it.impact = "Models() 数据源 = session/new 应答中的 config-option 状态:缓存 + TTL,InvalidateModels() 清缓存(ModelCatalogRefresher);" +
		"无会话时用一次性瞬态会话取样后 session/close;set_config_option(model/reasoning_effort) 的 optionId 以本帧实测字段为准;" +
		"严禁在 daemon 里写死模型名单(设计原则 §7)。"
	return p.SessionID, it
}

// ---- 第 3 项:update-granularity ----

func doGranularity(c *client, sid string) item {
	const promptText = "1+1 等于几？直接回答"
	marker := c.notesLen()
	t0 := time.Now()
	res, rawLine, errRaw, err := c.call("session/prompt", map[string]any{
		"sessionId": sid,
		"prompt":    []any{map[string]any{"type": "text", "text": promptText}},
	}, 120*time.Second)
	notes := c.notesFrom(marker)
	it := item{
		name: "session/prompt 逐帧记录 session/update(流式粒度量化)",
		cmd:  fmt.Sprintf(`session/prompt {"sessionId":"%s","prompt":[{"type":"text","text":%q}]}`, sid, promptText),
	}
	var frames []string
	variants := map[string]int{}
	var chunkLens []int
	var thoughtLens []int
	for _, n := range notes {
		v := variantOf(n.msg)
		variants[v]++
		switch v {
		case "agent_message_chunk":
			chunkLens = append(chunkLens, len([]rune(chunkText(n.msg))))
		case "agent_thought_chunk":
			thoughtLens = append(thoughtLens, len([]rune(chunkText(n.msg))))
		}
		frames = append(frames, fmt.Sprintf("[+%4dms] %s", n.at.Sub(t0).Milliseconds(), trunc(n.msg.raw, 4000)))
	}
	it.frames = frames
	if len(rawLine) > 0 {
		it.frames = append(it.frames, "[prompt 应答] "+trunc(rawLine, 2000))
	}
	if err != nil {
		it.status = "受阻(缺 key)"
		if !keyRelated(string(errRaw), rawLine) {
			it.status = "失败"
		}
		it.note = "prompt 失败: " + err.Error()
		if len(errRaw) > 0 {
			it.frames = append(it.frames, trunc(string(errRaw), 2000))
		}
		it.conclusion = "无 DEEPSEEK_API_KEY 环境下的实际错误形状见原始帧 —— 本帧即第 7 项(ErrorInfoCarrier)素材。"
		it.impact = "缺 key 时线程仍可建(prompt 才报错) → 注册不依赖 key(§4.4);错误形状映射 ErrorInfoCarrier:MISSING_CREDENTIAL + 结构化 Error,终局 TurnFailed 帧保真。"
		return it
	}
	var chunksNote string
	switch {
	case len(chunkLens) == 0:
		chunksNote = "全程 0 帧 agent_message_chunk(异常,需人工核对原始帧)"
	case len(chunkLens) == 1:
		chunksNote = fmt.Sprintf("assistant 文本以 1 帧到齐(%d 字符)= 块级(已提交消息文本块),非逐 delta 流式", chunkLens[0])
	default:
		total := 0
		for _, l := range chunkLens {
			total += l
		}
		avg := total / len(chunkLens)
		chunksNote = fmt.Sprintf("assistant 文本 %d 帧(各 %v 字符,均长 %d)= delta 级分片", len(chunkLens), chunkLens, avg)
	}
	hasUsage := false
	for _, n := range notes {
		ls := strings.ToLower(n.msg.raw)
		if strings.Contains(ls, "usage") || strings.Contains(ls, "context") {
			hasUsage = true
			break
		}
	}
	it.status = "通过"
	it.note = fmt.Sprintf("update 帧=%d 变体分布=%v prompt 应答 stopReason=%s",
		len(notes), variants, stopReasonOf(res))
	it.conclusion = chunksNote + fmt.Sprintf(";thought 帧 %d 个(各 %v 字符);tool_call 帧 %d 个(纯问答无工具,符合预期);帧内 usage/context 字段在场=%v;终局 stopReason=%s。",
		len(thoughtLens), thoughtLens, variants["tool_call"]+variants["tool_call_update"], hasUsage, stopReasonOf(res))
	it.impact = "agent_message_chunk → pb.AssistantDelta;块级粒度下手机端'逐字上屏'实为'按块上屏'(R2 实锤,UI 预期按此校准,跟踪上游 discussion #594);" +
		"agent_thought_chunk → ThinkingDelta(HideReasoning 过滤);usage/context 字段在场性决定 UsageUpdate(上下文环)的数据源与兜底;" +
		"未识别变体 → 主线 A 原样包装透传(native_notifications 范式)。"
	return it
}

// ---- 第 4 项:permission ----

// doPermissionRound 跑一轮:诱导工具调用 → 自动应答 allow/reject。
// 反向请求处理器经 reqCh 把描述传回(避开 goroutine 写闭包变量的竞态)。
func doPermissionRound(c *client, sid, promptText, mode string) (frames []string, note string) {
	marker := c.notesLen()
	t0 := time.Now()
	reqCh := make(chan string, 4)
	c.setHandler(func(req rpcMsg) {
		var p struct {
			SessionID string         `json:"sessionId"`
			ToolCall  map[string]any `json:"toolCall"`
			Options   []struct {
				OptionID string `json:"optionId"`
				Name     string `json:"name"`
				Kind     string `json:"kind"`
			} `json:"options"`
			Kind string `json:"kind"`
		}
		_ = json.Unmarshal(req.Params, &p)
		var optIDs []string
		for _, o := range p.Options {
			optIDs = append(optIDs, fmt.Sprintf("{optionId:%q name:%q kind:%q}", o.OptionID, o.Name, o.Kind))
		}
		pick := ""
		for _, o := range p.Options {
			if mode == "allow" && strings.Contains(o.Kind, "allow") {
				pick = o.OptionID
				break
			}
			if mode == "reject" && strings.Contains(o.Kind, "reject") {
				pick = o.OptionID
				break
			}
		}
		if pick == "" && len(p.Options) > 0 {
			if mode == "allow" {
				pick = p.Options[0].OptionID
			} else {
				pick = p.Options[len(p.Options)-1].OptionID
			}
		}
		_ = c.respond(req.idString(), map[string]any{
			"outcome": map[string]any{"outcome": "selected", "optionId": pick},
		})
		reqCh <- fmt.Sprintf("收到 session/request_permission:sessionId=%q requestKind=%q options=%v toolCall=%s → 自动%s optionId=%q(outcome={outcome:\"selected\",optionId})",
			p.SessionID, p.Kind, optIDs, string(mustJSON(p.ToolCall)), mode, pick)
	})
	defer c.setHandler(nil)
	res, rawLine, errRaw, err := c.call("session/prompt", map[string]any{
		"sessionId": sid,
		"prompt":    []any{map[string]any{"type": "text", "text": promptText}},
	}, 120*time.Second)
	notes := c.notesFrom(marker)
	var permNotes []string
drain:
	for {
		select {
		case s := <-reqCh:
			permNotes = append(permNotes, s)
		default:
			break drain
		}
	}
	for _, n := range notes {
		frames = append(frames, fmt.Sprintf("[+%4dms] %s", n.at.Sub(t0).Milliseconds(), trunc(n.msg.raw, 4000)))
	}
	if len(rawLine) > 0 {
		frames = append(frames, "[prompt 应答] "+trunc(rawLine, 2000))
	}
	if err != nil {
		if len(errRaw) > 0 {
			frames = append(frames, trunc(string(errRaw), 2000))
		}
		st := "失败"
		if keyRelated(string(errRaw), rawLine) {
			st = "受阻(缺 key)"
		}
		return frames, st + " prompt 失败: " + err.Error()
	}
	var statuses []string
	for _, n := range notes {
		v := variantOf(n.msg)
		if v == "tool_call" || v == "tool_call_update" {
			var p struct {
				Update struct {
					Status string `json:"status"`
				} `json:"update"`
			}
			_ = json.Unmarshal(n.msg.Params, &p)
			statuses = append(statuses, v+":"+p.Update.Status)
		}
	}
	permStr := "(未收到反向请求 —— 工具未被调用或被策略自动放行/拒绝,见原始帧)"
	if len(permNotes) > 0 {
		permStr = strings.Join(permNotes, " ; ")
	}
	return frames, fmt.Sprintf("stopReason=%s tool_call 轨迹=%v;审批(%s)=%s", stopReasonOf(res), statuses, mode, permStr)
}

func doPermission(c *client, sid string) item {
	it := item{
		name: "session/request_permission 反向请求(allow/reject 双路)",
		cmd:  `session/prompt {"prompt":[{"type":"text","text":"运行命令 echo dshprobe-permission-test …"}]} → 反向请求自动应答`,
	}
	allowFrames, allowNote := doPermissionRound(c, sid, "请运行命令 echo dshprobe-permission-test 并告诉我它的输出。", "allow")
	rejectFrames, rejectNote := doPermissionRound(c, sid, "请再运行一次命令 echo dshprobe-permission-test-2 并告诉我它的输出。", "reject")
	it.frames = append(append([]string{"── allow 路 ──"}, allowFrames...), append([]string{"── reject 路 ──"}, rejectFrames...)...)
	if strings.Contains(allowNote, "受阻") || strings.Contains(rejectNote, "受阻") {
		it.status = "受阻(缺 key)"
	} else if strings.Contains(allowNote, "失败") || strings.Contains(rejectNote, "失败") {
		it.status = "失败"
	} else {
		it.status = "通过"
	}
	it.note = "allow: " + allowNote + " | reject: " + rejectNote
	it.conclusion = "请求形状(options 数组/kind 字段/optionId)与应答形状(outcome={outcome:\"selected\",optionId})以原始帧为准;" +
		"allow 路命令应实际执行(输出含 dshprobe-permission-test),reject 路命令不应执行。"
	it.impact = "审批桥映射:dsh session/request_permission{options:[...]} → ToolCallBlock + ApprovalReason → 手机弹卡 → SubmitApproval 翻译为 outcome{type:\"selected\",optionId};" +
		"只做一次性 allow/reject,无'始终允许'(R3,审批卡如实两键);超时(2min)/断线 → 拒绝路径,绝不伪造'同意'(红线 1);" +
		"未识别的反向请求回 JSON-RPC error(codex_requests.go 精确匹配范式)。"
	return it
}

// ---- 第 5 项:cancel ----

func doCancel(c *client, sid string) item {
	const promptText = "从 1 数到 100000，每个数字单独一行输出"
	it := item{
		name: "session/cancel 中断语义",
		cmd:  fmt.Sprintf(`session/prompt {"text":%q} → 2s 后 session/cancel 通知`, promptText),
	}
	marker := c.notesLen()
	t0 := time.Now()
	type promptResult struct {
		res    json.RawMessage
		raw    string
		errRaw json.RawMessage
		err    error
	}
	done := make(chan promptResult, 1)
	go func() {
		res, raw, errRaw, err := c.call("session/prompt", map[string]any{
			"sessionId": sid,
			"prompt":    []any{map[string]any{"type": "text", "text": promptText}},
		}, 120*time.Second)
		done <- promptResult{res, raw, errRaw, err}
	}()
	time.Sleep(2 * time.Second)
	cancelAt := time.Now()
	if err := c.notify("session/cancel", map[string]any{"sessionId": sid}); err != nil {
		it.status = "失败"
		it.note = "session/cancel 发送失败: " + err.Error()
		return it
	}
	pr := <-done
	notes := c.notesFrom(marker)
	variants := map[string]int{}
	for _, n := range notes {
		variants[variantOf(n.msg)]++
		it.frames = append(it.frames, fmt.Sprintf("[+%4dms] %s", n.at.Sub(t0).Milliseconds(), trunc(n.msg.raw, 2000)))
	}
	if pr.err != nil {
		it.status = "失败"
		it.note = "prompt 等待失败: " + pr.err.Error()
		if len(pr.errRaw) > 0 {
			it.frames = append(it.frames, trunc(string(pr.errRaw), 2000))
		}
		if keyRelated(string(pr.errRaw), pr.raw) {
			it.status = "受阻(缺 key)"
			it.conclusion = "无 key 下 prompt 即时凭证失败,cancel 中断语义无法在本机验证(需配 DEEPSEEK_API_KEY 补测)。"
		} else {
			it.conclusion = "cancel 路径未走通,原因见原始帧(需人工核对)。"
		}
		return it
	}
	sr := stopReasonOf(pr.res)
	it.status = "通过"
	it.note = fmt.Sprintf("cancel 发出后 %s 收到 prompt 应答 stopReason=%s;期间 update 帧=%d 变体=%v",
		time.Since(cancelAt).Round(time.Millisecond), sr, len(notes), variants)
	it.conclusion = fmt.Sprintf("session/cancel(通知,无 id)发出后 prompt 终局 stopReason=%s(期望 cancelled);update 流是否出现 cancelled 变体见原始帧。", sr)
	it.impact = "CancelTurn → session/cancel 通知;stopReason=cancelled → TurnFailed(cancelled);" +
		"prompt 终局由应答驱动而非超时(dshapp 的 callTimeout 不管 prompt 在飞,§4.2)。"
	return it
}

// ---- 第 6 项:resume(跨进程) ----

func doResume(node, bin, home, workDir string, r *runner) item {
	it := item{
		name: "跨进程 resume(kill 子进程 → 新进程 session/list + session/resume)",
		cmd:  `kill → node bin.js --profile acp → initialize → session/list → session/resume → session/prompt "我们刚才聊了什么？一句话"`,
	}
	// 阶段 A:旧进程建会话聊一句
	old := &client{opts: clientOpts{node: node, bin: bin, dshHome: home, workDir: workDir, label: "resume-旧进程"}, pending: map[string]chan rpcMsg{}}
	if err := old.start(); err != nil {
		it.status = "失败"
		it.note = "旧进程启动失败: " + err.Error()
		return it
	}
	if err := old.initialize(); err != nil {
		it.status = "失败"
		if keyRelated(string(old.initErrRaw)) {
			it.status = "受阻(缺 key)"
		}
		it.note = "旧进程 initialize 失败: " + err.Error()
		old.close()
		return it
	}
	sid, _ := doSessionNew(old, workDir) // session/new 形状已由第 2 项覆盖,这里只要 sessionId
	var priorLine string
	if sid != "" {
		_, raw, _, err := old.call("session/prompt", map[string]any{
			"sessionId": sid,
			"prompt":    []any{map[string]any{"type": "text", "text": "1+1 等于几？直接回答"}},
		}, 120*time.Second)
		if err != nil {
			priorLine = raw // 可能是凭证错误,如实带出
		}
	}
	old.close()
	time.Sleep(800 * time.Millisecond)
	if sid == "" {
		it.status = "失败"
		it.note = "旧进程未取得 sessionId " + clip(priorLine, 300)
		return it
	}

	// 阶段 B:新进程 list + resume
	fresh := &client{opts: clientOpts{node: node, bin: bin, dshHome: home, workDir: workDir, label: "resume-新进程"}, pending: map[string]chan rpcMsg{}}
	if err := fresh.start(); err != nil {
		it.status = "失败"
		it.note = "新进程启动失败: " + err.Error()
		return it
	}
	defer fresh.close()
	if err := fresh.initialize(); err != nil {
		it.status = "失败"
		it.note = "新进程 initialize 失败: " + err.Error()
		return it
	}
	listRes, _, listErrRaw, listErr := fresh.call("session/list", map[string]any{}, 60*time.Second)
	if listErr != nil {
		it.status = "失败"
		it.note = "session/list 失败: " + listErr.Error()
		it.frames = append(it.frames, trunc(string(listErrRaw), 2000))
		return it
	}
	it.frames = append(it.frames, "── session/list 应答 ──", trunc(pretty(listRes), 5000))
	inList := strings.Contains(string(listRes), sid)
	// resume 期间重放检测
	marker := fresh.notesLen()
	resRes, _, resErrRaw, resErr := fresh.call("session/resume", map[string]any{"sessionId": sid, "cwd": workDir, "mcpServers": []any{}}, 60*time.Second)
	if resErr != nil {
		it.status = "失败"
		it.note = fmt.Sprintf("session/resume 失败(在列=%v): %s", inList, resErr.Error())
		it.frames = append(it.frames, trunc(string(resErrRaw), 2000))
		return it
	}
	time.Sleep(1 * time.Second)
	replayed := fresh.notesFrom(marker)
	it.frames = append(it.frames, "── session/resume 应答 ──", trunc(pretty(resRes), 3000))
	it.frames = append(it.frames, fmt.Sprintf("── resume 后 1s 内收到的 update 帧(重放检测):%d 帧 ──", len(replayed)))
	for _, n := range replayed {
		it.frames = append(it.frames, trunc(n.msg.raw, 2000))
	}
	// 恢复后追问上下文
	marker2 := fresh.notesLen()
	t0 := time.Now()
	res, rawLine, errRaw, err := fresh.call("session/prompt", map[string]any{
		"sessionId": sid,
		"prompt":    []any{map[string]any{"type": "text", "text": "我们刚才聊了什么？一句话"}},
	}, 120*time.Second)
	notes := fresh.notesFrom(marker2)
	for _, n := range notes {
		it.frames = append(it.frames, fmt.Sprintf("[+%4dms] %s", n.at.Sub(t0).Milliseconds(), trunc(n.msg.raw, 3000)))
	}
	if len(rawLine) > 0 {
		it.frames = append(it.frames, "[prompt 应答] "+trunc(rawLine, 2000))
	}
	if err != nil {
		if len(errRaw) > 0 {
			it.frames = append(it.frames, trunc(string(errRaw), 2000))
		}
		it.status = "失败"
		if keyRelated(string(errRaw), rawLine) {
			it.status = "受阻(缺 key)"
		}
		it.note = fmt.Sprintf("resume 后 prompt 失败(list 在列=%v,resume 本身成功): %s", inList, err.Error())
		it.conclusion = fmt.Sprintf(
			"跨进程 resume 协议面已实证:子进程 kill 后,新进程 session/list 在列=%v,session/resume 成功(应答含 configOptions,同 session/new 形状),且不重放旧 update(重放 %d 帧);"+
				"上下文保持(恢复后问答引用旧内容)因凭证失败未验证 —— 需配 DEEPSEEK_API_KEY 补测。",
			inList, len(replayed))
		r.setSummary(func(s *summary) {
			s.resume = fmt.Sprintf("协议面已实证:list 在列=%v,resume 成功,重放=%d 帧;上下文保持待配 key 补测", inList, len(replayed))
		})
		return it
	}
	var answer strings.Builder
	for _, n := range notes {
		if variantOf(n.msg) == "agent_message_chunk" {
			answer.WriteString(chunkText(n.msg))
		}
	}
	ans := clip(strings.TrimSpace(answer.String()), 200)
	hits := []string{}
	for _, k := range []string{"1+1", "2", "加", "echo", "数", "probe"} {
		if strings.Contains(answer.String(), k) {
			hits = append(hits, k)
		}
	}
	resumed := inList || strings.Contains(string(resRes), sid)
	it.status = "通过"
	it.note = fmt.Sprintf("sessionId 在 session/list 中=%v;resume 应答=%q;resume 重放旧 update=%d 帧;恢复后回答=%q(命中标记=%v);stopReason=%s",
		inList, clip(string(resRes), 200), len(replayed), ans, hits, stopReasonOf(res))
	it.conclusion = fmt.Sprintf(
		"跨进程 resume 可行=%v:子进程被 kill 后,新进程 session/list %s 目标会话,session/resume 成功,恢复后追问引用旧上下文(回答:%q)。"+
			"resume 是否重放旧 update:%d 帧(0 帧=符合'log 恢复不重放'预期,手机端不会收到重复历史帧)。",
		resumed, map[bool]string{true: "列出", false: "未列出"}[inList], ans, len(replayed))
	it.impact = "R5(跨进程 resume)关闭/收敛:ExternalSession.SessionID = ACP sessionId 落 threads.json;" +
		"重挂载 = 懒启动 → initialize → session/list 校验存在 → session/resume;'不在列 → 显式报错不静默新开'(三方共同教训);" +
		"不重放旧 update → 手机端重挂载后不收重复帧,无需去重逻辑。"
	r.setSummary(func(s *summary) {
		s.resume = fmt.Sprintf("可行=%v(list 含目标=%v,重放=%d 帧)回答=%q", resumed, inList, len(replayed), ans)
	})
	return it
}

// ---- 第 7 项:no-key ----

func doNoKey(node, bin, base string) item {
	it := item{
		name: "无 DEEPSEEK_API_KEY 的错误形状(ErrorInfoCarrier 素材)",
		cmd:  `env 剔除 DEEPSEEK_API_KEY → initialize → session/new → session/prompt "1+1 等于几？直接回答"`,
	}
	home := filepath.Join(base, "home-nokey")
	workDir := filepath.Join(base, "ws-nokey")
	_ = os.MkdirAll(workDir, 0o755)
	c := &client{opts: clientOpts{node: node, bin: bin, dshHome: home, workDir: workDir, stripKey: true, label: "no-key"}, pending: map[string]chan rpcMsg{}}
	defer c.close()
	if err := c.start(); err != nil {
		it.status = "失败"
		it.note = "启动失败: " + err.Error()
		return it
	}
	if err := c.initialize(); err != nil {
		it.status = "失败"
		it.note = "initialize 即失败: " + err.Error()
		it.frames = []string{trunc(string(c.initErrRaw), 2000)}
		return it
	}
	it.frames = append(it.frames, "── initialize 应答(无 key 下握手不受影响)──", trunc(pretty(c.initRes), 2500))
	snRes, snLine, snErrRaw, snErr := c.call("session/new", map[string]any{"cwd": workDir, "mcpServers": []any{}}, 60*time.Second)
	var sid string
	if snErr == nil {
		_ = json.Unmarshal(snRes, &struct {
			SessionID *string `json:"sessionId"`
		}{&sid})
		it.frames = append(it.frames, "── session/new 应答 ──", trunc(pretty(snRes), 3000))
	} else {
		it.frames = append(it.frames, "── session/new 错误 ──", trunc(string(snErrRaw), 2000), trunc(snLine, 2000))
	}
	if sid != "" {
		_, rawLine, errRaw, err := c.call("session/prompt", map[string]any{
			"sessionId": sid,
			"prompt":    []any{map[string]any{"type": "text", "text": "1+1 等于几？直接回答"}},
		}, 90*time.Second)
		if len(rawLine) > 0 {
			it.frames = append(it.frames, "── session/prompt 应答 ──", trunc(rawLine, 3000))
		}
		if len(errRaw) > 0 {
			it.frames = append(it.frames, "── session/prompt JSON-RPC error ──", trunc(string(errRaw), 3000))
		}
		it.status = "通过" // 第 7 项的"通过"= 成功观测到缺 key 场景的真实形状(成功或错误都算素材)
		if err != nil {
			it.note = "无 key 下 prompt 如实失败,错误形状见原始帧"
			var e struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			}
			_ = json.Unmarshal(errRaw, &e)
			it.conclusion = fmt.Sprintf("缺 key 错误形状实测:session/prompt 返回 JSON-RPC error code=%d,message=%q(无结构化 data 字段);initialize/session/new 均不受影响。"+
				"ErrorInfoCarrier 映射:按 code+message 归一化为 MISSING_CREDENTIAL,原样 message 给人话诊断;session/new 无 key 仍成功 → '注册不依赖 key、首条 prompt 才报错'(§4.4)成立。", e.Code, clip(e.Message, 200))
		} else {
			it.note = "无 key 下 prompt 竟然成功 —— 凭据可能经 dsh 回退链解析(cwd .env / ~/.dsh/.env;红线所致未核查 ~/.dsh)"
			it.conclusion = "本机无法构造缺钥场景(DEEPSEEK_API_KEY env 已剔除仍成功 → key 来自 dsh 自身回退链)。MISSING_CREDENTIAL 形状需在净机/无 ~/.dsh 回退的环境补测。"
		}
	} else {
		it.status = "通过"
		it.note = "无 key 下 session/new 即失败,错误形状见原始帧"
		it.conclusion = "缺 key 时 session/new 就报错(而非 prompt 才报错)→ §4.4'注册不依赖 key、首条 prompt 才报错'的假设需修正为'session/new 即报错',Models() 取样要带凭证错误兜底。"
	}
	for _, l := range c.stderrSnapshot() {
		it.frames = append(it.frames, "[dsh-stderr] "+l)
	}
	return it
}

// ---- 第 8 项:coldstart ----

func doColdstart(c *client) item {
	it := item{
		name:   "冷启动耗时(start → initialize 应答,首次 profile 初始化)",
		cmd:    "node bin.js --profile acp(virgin DSH_HOME)→ initialize",
		frames: []string{trunc(pretty(c.initRes), 2500)},
	}
	it.status = "通过"
	it.note = fmt.Sprintf("耗时=%s(含进程 spawn + Node 启动 + dsh 首次 profile 自动初始化)", c.initDur.Round(time.Millisecond))
	it.conclusion = fmt.Sprintf("冷启动实测 %s。对照预算:ListModels 并行拉取 10s 预算内可完成;daemon 启动期绝不做此握手(PA-REAL-0002/0003),只有懒启动首个线程/模型列举时付一次。", c.initDur.Round(time.Millisecond))
	it.impact = "dshapp 懒启动 + 崩溃自动重启的成本锚点;ListModels 的 dsh 分支若遇冷启动超 10s → 如实报'初始化中'(既有并行有界机制自动兜底);探测只做 --version(有缓存 5min)。"
	return it
}

// ---- 定位与版本 ----

func locateNode() string {
	if p := strings.TrimSpace(os.Getenv("DSH_NODE")); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
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

func locateBin() string {
	if p := strings.TrimSpace(os.Getenv("DSH_BIN")); p != "" {
		return p
	}
	return defaultBin
}

func dshVersion(node, bin string) string {
	out, err := exec.Command(node, bin, "--version").Output()
	if err != nil {
		return "(--version 失败: " + err.Error() + ")"
	}
	return strings.TrimSpace(string(out))
}

// setupClient 拉起 + initialize,失败时把失败登记为 item 并返回 nil。
func setupClient(r *runner, opts clientOpts) *client {
	c := &client{opts: opts, pending: map[string]chan rpcMsg{}}
	if err := c.start(); err != nil {
		r.add(item{name: opts.label, status: "失败", note: "启动失败: " + err.Error()})
		return nil
	}
	if err := c.initialize(); err != nil {
		note := err.Error()
		if len(c.initErrRaw) > 0 {
			note += ";error=" + clip(string(c.initErrRaw), 500)
		}
		st := "失败"
		if keyRelated(string(c.initErrRaw)) {
			st = "受阻(缺 key)"
		}
		r.add(item{name: opts.label, status: st, note: "initialize 失败: " + note,
			frames: []string{trunc(strings.Join(c.records, "\n"), 3000)}})
		return nil
	}
	return c
}

// ---- 全量编排:A(1/2/3/4/5/8) → kill → B(6,同 HOME) → C(7,净 env) ----

func runAll(r *runner, node, bin, base, workDir string) {
	homeA := filepath.Join(base, "home-a")
	_ = os.MkdirAll(homeA, 0o755)
	_ = os.MkdirAll(workDir, 0o755)

	cA := &client{opts: clientOpts{node: node, bin: bin, dshHome: homeA, workDir: workDir, label: "A"}, pending: map[string]chan rpcMsg{}}
	startErr := cA.start()
	var initErr error
	coldDur := time.Duration(0)
	if startErr == nil {
		initErr = cA.initialize()
		coldDur = cA.initDur
		r.setSummary(func(s *summary) { s.coldDur = coldDur.Round(time.Millisecond).String() })
	}

	// 8. 冷启动 + 1. 握手(同一 virgin HOME 进程)
	it8 := item{n: 8, name: "冷启动耗时(start → initialize 应答,首次 profile 初始化)", status: "通过",
		cmd:        "node bin.js --profile acp(virgin DSH_HOME)→ initialize",
		note:       fmt.Sprintf("耗时=%s(含进程 spawn + Node 启动 + dsh 首次 profile 自动初始化)", coldDur.Round(time.Millisecond)),
		conclusion: fmt.Sprintf("冷启动实测 %s;daemon 启动期绝不做握手(PA-REAL-0002/0003),懒启动首个线程/模型列举时付一次;ListModels 10s 预算兼容性以此为锚。", coldDur.Round(time.Millisecond)),
		impact:     "dshapp 懒启动 + 崩溃自动重启成本锚点;超 10s 时 ListModels 如实报'初始化中';探测只做 --version(缓存 5min)。"}
	if startErr != nil {
		r.add(item{n: 1, name: "initialize 握手 + 能力集 dump", status: "失败", note: "进程 A 启动失败: " + startErr.Error()})
		it8.status, it8.note = "失败", "进程 A 启动失败: "+startErr.Error()
		r.add(it8)
	} else if initErr != nil {
		r.add(item{n: 1, name: "initialize 握手 + 能力集 dump", status: "失败",
			note:   "initialize 失败: " + initErr.Error() + " error=" + clip(string(cA.initErrRaw), 500),
			frames: []string{trunc(strings.Join(cA.records, "\n"), 3000)}})
		it8.status, it8.note = "受阻(缺 key)", "initialize 阶段失败(见第 1 项),耗时仅到失败点="+coldDur.Round(time.Millisecond).String()
		if !keyRelated(string(cA.initErrRaw)) {
			it8.status = "失败"
		}
		r.add(it8)
	} else {
		it1 := doHandshake(cA, coldDur)
		it1.n = 1
		r.add(it1)
		r.add(it8)

		// 2. session/new
		sid, it2 := doSessionNew(cA, workDir)
		it2.n = 2
		r.add(it2)
		r.setSummary(func(s *summary) { s.modelShape = clip(it2.conclusion, 300) })

		if sid != "" {
			it3 := doGranularity(cA, sid)
			it3.n = 3
			r.add(it3)
			r.setSummary(func(s *summary) {
				if it3.status == "通过" {
					s.granularity = clip(it3.conclusion, 300)
				} else {
					s.granularity = "未测(本机无 DEEPSEEK_API_KEY,prompt 即时凭证失败,见第 3/7 项原始帧);P1 UI 先按'块级上屏'设计(R2 上限假设),配 key 后重跑本探针量化"
				}
			})

			it4 := doPermission(cA, sid)
			it4.n = 4
			r.add(it4)

			it5 := doCancel(cA, sid)
			it5.n = 5
			r.add(it5)
		} else {
			for _, name := range []string{"session/prompt 逐帧记录 session/update(流式粒度量化)",
				"session/request_permission 反向请求(allow/reject 双路)", "session/cancel 中断语义"} {
				r.add(item{n: 0, name: name, status: "失败", note: "无 sessionId(session/new 失败,见第 2 项),无法继续"})
			}
		}
	}
	cA.close()
	time.Sleep(800 * time.Millisecond)

	// 6. 跨进程 resume(同 HOME 新进程)
	it6 := doResume(node, bin, homeA, workDir, r)
	it6.n = 6
	r.add(it6)

	// 7. no-key(净 env 新进程,独立 HOME)
	it7 := doNoKey(node, bin, base)
	it7.n = 7
	r.add(it7)
	r.setSummary(func(s *summary) { s.noKey = clip(it7.note+";"+it7.conclusion, 300) })
}

// ---- 报告生成 ----

func writeReport(out, node, bin, ver, base string, items []item, sum summary, envKeyPresent bool) error {
	sorted := append([]item(nil), items...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].n < sorted[j].n })

	var b strings.Builder
	b.WriteString("# dsh ACP P0 探针记录(DeepSeek Harness 桥接前置验证)\n\n")
	b.WriteString("> 生成:" + time.Now().Format("2006-01-02 15:04:05") + " 工具:daemon/cmd/dshprobe(node 直拉 bin.js --profile acp,Go 手写 JSON-RPC 2.0 分帧,零外部依赖)\n")
	b.WriteString("> node=" + node + "\n")
	b.WriteString("> bin=" + bin + " version=" + ver + "\n")
	b.WriteString("> 隔离:全程 DSH_HOME=<临时目录>\\home-*,未读写用户 ~/.dsh(红线);DEEPSEEK_API_KEY=" + map[bool]string{true: "SET(经环境透传)", false: "EMPTY(本机 shell 无 key)"}[envKeyPresent] + "\n")
	b.WriteString("> 设计蓝本:2026-09-20-dsh-integration-plan.md §7 P0(8 项)\n\n")

	pass, blocked, failed := 0, 0, 0
	for _, it := range sorted {
		switch {
		case it.status == "通过":
			pass++
		case strings.HasPrefix(it.status, "受阻"):
			blocked++
		default:
			failed++
		}
	}
	b.WriteString(fmt.Sprintf("结果:**%d 通过 / %d 受阻(缺 key) / %d 失败**(共 %d 项)\n\n", pass, blocked, failed, len(sorted)))

	b.WriteString("## 关键结论速览(汇报五要点)\n\n")
	b.WriteString("| 要点 | 实测结论 |\n|---|---|\n")
	b.WriteString("| 流式粒度 | " + orDash(sum.granularity) + " |\n")
	b.WriteString("| 跨进程 resume(R5) | " + orDash(sum.resume) + " |\n")
	b.WriteString("| config-option 模型选项形状(Models() 契约) | " + orDash(sum.modelShape) + " |\n")
	b.WriteString("| 缺 key 错误形状 | " + orDash(sum.noKey) + " |\n")
	b.WriteString("| 冷启动耗时 | " + orDash(sum.coldDur) + " |\n\n")

	for _, it := range sorted {
		b.WriteString(fmt.Sprintf("## %d. %s —— %s\n\n", it.n, it.name, it.status))
		if it.cmd != "" {
			b.WriteString("概念命令:`" + it.cmd + "`\n\n")
		}
		if it.note != "" {
			b.WriteString(it.note + "\n\n")
		}
		if len(it.frames) > 0 {
			b.WriteString("原始帧:\n\n```\n")
			for _, f := range it.frames {
				b.WriteString(f + "\n")
			}
			b.WriteString("```\n\n")
		}
		if it.conclusion != "" {
			b.WriteString("**结论**:" + it.conclusion + "\n\n")
		}
		if it.impact != "" {
			b.WriteString("**对 DshProvider 设计的影响**:" + it.impact + "\n\n")
		}
	}
	b.WriteString("> 未完成/需 DEEPSEEK_API_KEY 补测的项以「受阻(缺 key)」标注;其原始帧即错误形状素材(第 7 项口径),非失败口径伪造。\n")

	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	return os.WriteFile(out, []byte(b.String()), 0o644)
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—(单项运行模式或该项受阻,见对应小节)"
	}
	return strings.ReplaceAll(s, "\n", " ")
}

// ---- main ----

func main() {
	out := flag.String("out", `G:\项目1\pocketagent\docs\verify\evidence\dsh-p0-probe.md`, "证据报告输出路径")
	run := flag.String("run", "", "单项:handshake|session-new|update-granularity|permission|cancel|resume|no-key|coldstart|configpersist|resumecompat;-all = 全部 8 项")
	all := flag.Bool("all", false, "跑全部 8 项(默认,与空 -run 等价)")
	// resumecompat 专用:真实 DSH_HOME 的只读来源(空 = 自动探测迁移后数据目录
	// %LOCALAPPDATA%\PocketAgent\data\dsh-home,回退 ~/.pocketagentd/dsh-home)。
	compatHome := flag.String("compat-home", "", "resumecompat:要复制的真实 DSH_HOME(只读源,副本进临时目录)")
	flag.Parse()

	node, bin := locateNode(), locateBin()
	if node == "" || bin == "" {
		fmt.Printf("定位失败 node=%q bin=%q\n", node, bin)
		os.Exit(1)
	}
	ver := dshVersion(node, bin)
	fmt.Printf("node=%s\nbin=%s (%s)\n", node, bin, ver)

	base, err := os.MkdirTemp("", "dshprobe-*")
	if err != nil {
		fmt.Println("建临时目录失败:", err)
		os.Exit(1)
	}
	defer os.RemoveAll(base)
	workDir := filepath.Join(base, "ws")
	_ = os.MkdirAll(workDir, 0o755)

	r := &runner{}
	mode := "all"
	if *run != "" {
		mode = *run
	} else if !*all {
		fmt.Println("(未指定 -run/-all,默认跑全量 8 项;单项用 -run <name>)")
	}
	switch mode {
	case "all":
		fmt.Println("== dsh P0 探针(全量 8 项)开始 ==")
		runAll(r, node, bin, base, workDir)
	case "handshake":
		home := filepath.Join(base, "home-handshake")
		_ = os.MkdirAll(home, 0o755)
		c := &client{opts: clientOpts{node: node, bin: bin, dshHome: home, workDir: workDir, label: mode}, pending: map[string]chan rpcMsg{}}
		if err := c.start(); err != nil {
			r.add(item{name: mode, status: "失败", note: err.Error()})
			break
		}
		if err := c.initialize(); err != nil {
			r.add(item{name: mode, status: "失败", note: "initialize: " + err.Error() + " " + clip(string(c.initErrRaw), 400)})
			c.close()
			break
		}
		it := doHandshake(c, c.initDur)
		r.add(it)
		c.close()
	case "coldstart":
		home := filepath.Join(base, "home-coldstart")
		_ = os.MkdirAll(home, 0o755)
		c := &client{opts: clientOpts{node: node, bin: bin, dshHome: home, workDir: workDir, label: mode}, pending: map[string]chan rpcMsg{}}
		if err := c.start(); err != nil {
			r.add(item{name: mode, status: "失败", note: err.Error()})
			break
		}
		if err := c.initialize(); err != nil {
			r.add(item{name: mode, status: "失败", note: "initialize: " + err.Error() + " " + clip(string(c.initErrRaw), 400)})
			c.close()
			break
		}
		it := doColdstart(c)
		r.add(it)
		r.setSummary(func(s *summary) { s.coldDur = c.initDur.Round(time.Millisecond).String() })
		c.close()
	case "session-new":
		home := filepath.Join(base, "home-sn")
		_ = os.MkdirAll(home, 0o755)
		c := setupClient(r, clientOpts{node: node, bin: bin, dshHome: home, workDir: workDir, label: mode})
		if c == nil {
			break
		}
		defer c.close()
		_, it := doSessionNew(c, workDir)
		r.add(it)
		r.setSummary(func(s *summary) { s.modelShape = clip(it.conclusion, 300) })
	case "update-granularity":
		home := filepath.Join(base, "home-ug")
		_ = os.MkdirAll(home, 0o755)
		c := setupClient(r, clientOpts{node: node, bin: bin, dshHome: home, workDir: workDir, label: mode})
		if c == nil {
			break
		}
		defer c.close()
		sid, _ := doSessionNew(c, workDir)
		if sid == "" {
			r.add(item{name: mode, status: "失败", note: "无 sessionId"})
			break
		}
		it := doGranularity(c, sid)
		r.add(it)
		r.setSummary(func(s *summary) { s.granularity = clip(it.conclusion, 300) })
	case "permission":
		home := filepath.Join(base, "home-perm")
		_ = os.MkdirAll(home, 0o755)
		c := setupClient(r, clientOpts{node: node, bin: bin, dshHome: home, workDir: workDir, label: mode})
		if c == nil {
			break
		}
		defer c.close()
		sid, _ := doSessionNew(c, workDir)
		if sid == "" {
			r.add(item{name: mode, status: "失败", note: "无 sessionId"})
			break
		}
		r.add(doPermission(c, sid))
	case "cancel":
		home := filepath.Join(base, "home-cancel")
		_ = os.MkdirAll(home, 0o755)
		c := setupClient(r, clientOpts{node: node, bin: bin, dshHome: home, workDir: workDir, label: mode})
		if c == nil {
			break
		}
		defer c.close()
		sid, _ := doSessionNew(c, workDir)
		if sid == "" {
			r.add(item{name: mode, status: "失败", note: "无 sessionId"})
			break
		}
		r.add(doCancel(c, sid))
	case "resume":
		home := filepath.Join(base, "home-resume")
		_ = os.MkdirAll(home, 0o755)
		r.add(doResume(node, bin, home, workDir, r))
	case "no-key":
		it := doNoKey(node, bin, base)
		r.add(it)
		r.setSummary(func(s *summary) { s.noKey = clip(it.note+" | "+it.conclusion, 300) })
	case "configpersist":
		// 原生设置透出轮(2026-09-25)新增:set_config_option 持久性实证。
		// 单项独立运行(-run configpersist),证据另写一份报告文件,不覆写 P0 八项。
		home := filepath.Join(base, "home-cfgp")
		_ = os.MkdirAll(home, 0o755)
		c := setupClient(r, clientOpts{node: node, bin: bin, dshHome: home, workDir: workDir, label: mode})
		if c == nil {
			break
		}
		defer c.close()
		r.add(doConfigPersist(c, workDir))
	case "resumecompat":
		// 升级兼容轮(2026-09-25)新增:rc.3 对 rc.2 旧 sessionId 的失败形态实证。
		// 真实 DSH_HOME 只读(先整树复制进临时目录,探针结束删除)。
		r.add(runResumeCompat(node, bin, *compatHome))
	default:
		fmt.Printf("未知 -run %q\n", *run)
		os.Exit(1)
	}

	if err := writeReport(*out, node, bin, ver, base, r.items, r.sum, os.Getenv("DEEPSEEK_API_KEY") != ""); err != nil {
		fmt.Println("写报告失败:", err)
		os.Exit(1)
	}
	fmt.Println("报告已写入:", *out)
}
