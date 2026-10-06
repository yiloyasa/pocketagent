// Package piapp 封装 `pi --mode rpc` 子进程:JSONL over stdio,每行一条命令/事件。
// 这是 Pi coding agent 给外部 GUI 客户端的无头协议(docs/rpc.md,0.85.1 实测)。
//
// 【讲解】与 codexapp 同一个思路:daemon 只做"翻译官",手机 gRPC ↔ Pi JSONL。
// 与 codex 的关键差异:
//   - Pi 一个子进程 = 一个会话(codex app-server 一个进程承载所有线程),
//     所以 Client 由上层(agent.PiProvider)按 Thread 管理,1:1 映射;
//   - 协议不是 JSON-RPC:命令是 {"type":"prompt",...},应答是 {"type":"response",
//     "id":...,"success":bool};事件直接 {"type":"agent_settled",...} 流式到达;
//   - 分帧只认 \n(官方明说 Node readline 不合规);图片走 base64 进单行 JSON,
//     一张照片几 MB —— 读缓冲必须 8MB 起(64KB 默认必炸,见调研 §3.2#1);
//   - Pi 无内建审批:靠扩展在 tool_call 钩子里弹 extension_ui_request(confirm),
//     daemon 注入守护扩展(见 guard.go)+ 自带超时兜底,超时/断线一律 cancelled
//     (失败安全,调研 §4.6)。
//
// 【0.85.1 实测(P0 探针,docs/verify/evidence/pi-p0-probe.md)】
//   - 流式期间 message_update.usage 恒为 0,用量走 get_session_stats;
//   - agent_end 每轮至少一条,agent_settled 才是真终局(重试/压缩/排队后仍会续跑);
//   - tool_execution_update.partialResult 是累积值(直接替换,不追加);
//   - fire-and-forget 扩展 UI(notify/setStatus/setWidget/setTitle/set_editor_text)
//     不期待应答,绝不能回 extension_ui_response(会污染协议流)。
//
// 【0.87.1 复测(docs/verify/evidence/pi-087-usage-probe.log)】message_update
// 的流式增量帧 usage 仍恒 0,但每条 assistant 消息的**最后一个增量帧(实测
// text_end)带上该消息的真实 usage**(per-message 口径,非会话累计)—— 消费方
// 见 internal/agent/pi.go 的 piUsageTracker。
//
// 【1.0.2 复测(2026-10-05 活体探针,Temp/pi-p0-probe.md + dist/bundle 核对)】
// 方法注册表/参数/响应包络相对 0.99.1 零变化(33 命令逐一对照);唯一行为面
// 变化是**启动门控**:runRpcMode 在扩展加载(rebindSession)完成后才开始读
// stdin,spawn 后的首条命令可能数十秒才被消费 —— GetState/SwitchSession 据此
// 走就绪重试(callWithReadyRetry),0.99.1 及更早版本行为不变。
package piapp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/pocketagent/daemon/internal/dshapp"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pocketagent/daemon/internal/hidewin"
	"github.com/pocketagent/daemon/internal/hostenv"
	"github.com/pocketagent/daemon/internal/stderrdrain"
)

// ---- 定位(不能依赖 PATH:daemon 由计划任务经 WMI 派生,环境变量不保证齐全)----
//
// 【位置为什么交给 hostenv 派生】本包曾把开发机的 node.exe 与 pi 入口写成常量
// (C:\Users\<开发机用户名>\... 与 G:\Claude Code\...)。Go 的 -trimpath 只剥
// 源码路径、不剥字符串常量,这两条会被原样编进 pocketagentd.exe 随发布包分发
// (规格 §2 明令禁止),且换一台机器必然不命中。现在全部按用户环境派生,探测不到
// 就返回空串、由调用方报一句可操作的错误 —— 回归钉子见 devpath_assert_test.go。

// piCLIRel 是 pi 在 npm 包内的入口相对路径。
const piCLIRel = "@earendil-works/pi-coding-agent/dist/bundle/cli.js"

// LocateNode 找 node 可执行文件:PI_NODE → PI_BIN → 用户环境派生 → PATH。
func LocateNode() string {
	for _, k := range []string{"PI_NODE", "PI_BIN"} {
		if p := strings.TrimSpace(os.Getenv(k)); p != "" {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	return hostenv.FindNode()
}

// LocateCLI 找 pi 的 cli.js:PI_CLI 覆盖 → 随包目录/npm 全局探测;都没有返回空串。
//
// 【为什么不再无条件返回一个默认值】旧实现无脑返回写死的开发机路径,于是
// LocatePi 的报错会念出开发机的目录结构 —— 用户既不知道自己该做什么,也无从
// 判断是配置问题还是安装问题。返回空串让 LocatePi 报「未找到 pi 的 cli.js
// (可用 PI_CLI 指定路径)」。
func LocateCLI() string {
	if p := strings.TrimSpace(os.Getenv("PI_CLI")); p != "" {
		return p
	}
	return hostenv.FirstFile(hostenv.PackageCandidates(piCLIRel))
}

// LocatePi 报告 pi 是否可用(node + cli.js 都找得到)。
// DetectProviders 用它决定是否注册 PiProvider。
func LocatePi() (node, cli string, err error) {
	node, cli = LocateNode(), LocateCLI()
	if node == "" {
		return "", "", errors.New("未找到 node(可用 PI_NODE/PI_BIN 指定路径)")
	}
	if cli == "" {
		return "", "", errors.New("未找到 pi 的 cli.js(可用 PI_CLI 指定路径)")
	}
	if _, err := os.Stat(cli); err != nil {
		return "", "", fmt.Errorf("pi cli.js 不存在: %w", err)
	}
	return node, cli, nil
}

// Version 取 pi 版本(--version);失败返回空串。
// 用于启动握手前的版本漂移检查(调研 §11#11)。
//
// 【P1-3】exec 出的 cmd 必须先 hidewin.Apply:node 是控制台程序,daemon 后台
// 拉起时这条探测每执行一次就闪一个黑窗(GetHostInfo 每 5 分钟后台刷新 + 每次
// 启动探测)。与 Start 内的构造(下方 hidewin.Apply(cmd) 处)及 codexapp/
// zcodeapp 的启动路径同款写法;回归钉子见 hidewin_assert_test.go(AST 断言:
// 构造路径不得出现裸 exec 链式调用)。
func Version(node, cli string) string {
	if node == "" || cli == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, node, cli, "--version")
	hidewin.Apply(cmd) // 控制台程序:不隐藏会在用户桌面闪黑窗(与 Start 同款)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// ---- 事件与扩展 UI 请求 ----

// Event 是 Pi 发来的一行(命令应答之外的全部)。
type Event struct {
	Type string          // "agent_start" / "message_update" / "tool_execution_end" / ...
	Raw  json.RawMessage // 原始行(诊断与单测用)
}

// UIRequest 是 extension_ui_request(扩展向"有 UI 的客户端"要交互)。
//
// 两类(docs/rpc.md):dialog(select/confirm/input/editor)等应答;
// fire-and-forget(notify/setStatus/setWidget/setTitle/set_editor_text)不等应答。
type UIRequest struct {
	ID        string   // 回写 extension_ui_response 时的关联 id
	Method    string   // select / confirm / input / editor / notify / setStatus / ...
	Title     string   // dialog 标题
	Message   string   // confirm 的正文
	Options   []string // select 的选项
	TimeoutMs int64    // 扩展自带的超时;0 = 无(daemon 必须自己兜底,见 §4.6)
	// Placeholder/Prefill 是 input/editor 的提示与预填(rpc-types.d.ts:425/432,
	// pi 1.0.2 字段级全适配审计补收)——手机反问卡据此渲染输入提示,不丢语境。
	Placeholder string // input 的占位提示(可空)
	Prefill     string // editor 的预填文本(可空)
	Raw         json.RawMessage
}

// Dialog 报告这是不是需要应答的 dialog 类请求。
func (r *UIRequest) Dialog() bool {
	switch r.Method {
	case "select", "confirm", "input", "editor":
		return true
	}
	return false
}

// ---- 客户端 ----

// CallCtx 的两类"非答案"失败哨兵:callWithReadyRetry 据此区分"值得换 id 重试
// 的未应答"与"立即返回的定局"。errors.Is 可判。
var (
	// errPiGone 子进程已退出(readLoop EOF 置死,所有 pending 立即唤醒):重试无意义。
	errPiGone = errors.New("pi 子进程已退出")
	// errNoAnswer 应答帧在预算内未到达(子进程活着但不消费 stdin —— 典型:
	// pi 1.0.2 的启动门控,见 callWithReadyRetry)。
	errNoAnswer = errors.New("等待 pi 应答超时")
)

// Client 管理一个 pi --mode rpc 子进程(= 一个 Pi 会话)。
type Client struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader

	writeMu sync.Mutex

	mu        sync.Mutex
	nextID    int64
	pending   map[string]chan piResp
	dead      bool
	deathErr  error
	handlerMu sync.Mutex

	// GlobalHandler 常驻事件钩子(先调用;只做记录,不做流式渲染,绝不能阻塞 ——
	// 它跑在 readLoop 里,一堵整条事件管道都停)。分工与 codexapp.Client 相同。
	//
	// Deprecated: 公开裸字段仅兼容独占客户端的旧探针。并发路径必须用
	// Setter；直接赋值不得与 CallCtx、syncHandlers 或任何 Setter 并发。
	GlobalHandler func(ev Event)
	// EventHandler 当轮事件钩子(Run 装上/收尾卸下;同样不能阻塞,慢消费自行开 goroutine)。
	EventHandler func(ev Event)
	// UIRequestHandler 处理 dialog 类 extension_ui_request。返回后由 piapp 负责回写;
	// nil = 立即回 cancelled(失败安全:Pi 的 dialog 无 timeout 时会永久等待,
	// 默认拒绝比默认放行安全 —— 调研 §10/§4.6)。
	// 返回 respond=false 表示不回写(慎用:会让扩展永久挂起,仅 fire-and-forget 可)。
	UIRequestHandler       func(req *UIRequest) (payload map[string]any, respond bool)
	UnknownResponseHandler func(rawID json.RawMessage, raw json.RawMessage)

	// sess 是当前挂载会话的缓存(Start 后由 get_state 填,switch_session 后更新),
	// 以原子快照发布。【P2-4】Rename/ExportHTML 的挂载路径(ensureMounted→
	// SwitchSession)与 Run 的 mount 并发跑在不同 goroutine,读写/写写都可能相撞
	// —— 不再依赖"同 goroutine 程序顺序"的旧纪律,读写全走原子指针(与 hs 同款)。
	sess atomic.Pointer[sessState]
	// sessMu 串行化 updateSession 的读改写:两个挂载路径并发合并时避免丢更新。
	sessMu sync.Mutex

	// Setter 写入和 CallCtx 快照读取均受 handlerMu 保护。dispatch/emit
	// 只读不可变快照，回调在锁外执行，允许重入请求或卸载。
	hs atomic.Pointer[piHandlers]

	// reaped 在 Close 的收尸 Wait 完成后关闭(确认句柄已回收;reapOnce 防重复 Close)。
	reaped   chan struct{}
	reapOnce sync.Once

	// waitOnce 保证 cmd.Wait 只执行一次(exec.Cmd 不允许二次 Wait):意外退出
	// (readLoop EOF,要拿退出码写日志)与 Close 主动收尸两条路径共用 reapWithLog。
	waitOnce sync.Once

	deathOnce sync.Once
	deathCh   chan struct{} // readLoop 退出时关闭:上层据此把轮次判死(PI_CRASHED)
}

// piHandlers 某一时刻的 handler 快照。
type piHandlers struct {
	global          func(Event)
	ev              func(Event)
	ui              func(*UIRequest) (payload map[string]any, respond bool)
	unknownResponse func(rawID json.RawMessage, raw json.RawMessage)
}

// sessState 某一时刻的会话挂载缓存快照。
type sessState struct {
	file string // 当前挂载的会话 .jsonl 绝对路径
	id   string // 当前会话 id
}

// SessionFile 返回当前挂载会话文件的缓存(原子读;未挂载过返回空串)。
func (c *Client) SessionFile() string {
	if s := c.sess.Load(); s != nil {
		return s.file
	}
	return ""
}

// SessionID 返回当前会话 id 的缓存(原子读;未挂载过返回空串)。
func (c *Client) SessionID() string {
	if s := c.sess.Load(); s != nil {
		return s.id
	}
	return ""
}

// updateSession 把非空字段合并进会话缓存并原子发布。GetState(条件更新)与
// SwitchSession(校验通过后覆盖)共用:两条挂载路径可能并发(Rename/ExportHTML
// 的 ensureMounted ↔ Run 的 mount),读改写经 sessMu 串行化防丢更新。
func (c *Client) updateSession(file, id string) {
	c.sessMu.Lock()
	defer c.sessMu.Unlock()
	next := &sessState{}
	if cur := c.sess.Load(); cur != nil {
		*next = *cur
	}
	if file != "" {
		next.file = file
	}
	if id != "" {
		next.id = id
	}
	c.sess.Store(next)
}

// syncHandlers 兼容旧探针的串行赋值后 CallCtx；生产写入统一用 Setter。
func (c *Client) syncHandlers() {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.publishHandlersLocked()
}

func (c *Client) publishHandlersLocked() {
	c.hs.Store(&piHandlers{
		global: c.GlobalHandler, ev: c.EventHandler, ui: c.UIRequestHandler,
		unknownResponse: c.UnknownResponseHandler,
	})
}

func (c *Client) SetGlobalHandler(handler func(Event)) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.GlobalHandler = handler
	c.publishHandlersLocked()
}

func (c *Client) SetUnknownResponseHandler(handler func(json.RawMessage, json.RawMessage)) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.UnknownResponseHandler = handler
	c.publishHandlersLocked()
}

func (c *Client) SetEventHandler(handler func(Event)) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.EventHandler = handler
	c.publishHandlersLocked()
}

func (c *Client) SetUIRequestHandler(handler func(*UIRequest) (map[string]any, bool)) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.UIRequestHandler = handler
	c.publishHandlersLocked()
}

// StartOptions 启动参数。
type StartOptions struct {
	Cwd            string   // 子进程工作目录(Pi 会话绑定创建时的工作区,之后不可换 —— 调研 §6.2)
	Name           string   // --name 会话显示名(手机端建的会话带 PocketAgent 标识,双端可见)
	ExtraArgs      []string // 额外 CLI 参数(如 --session-dir、--no-session)
	GuardExtension string   // -e 注入的守护扩展路径(危险命令审批);空 = 不注入
}

// buildRPCArgs 组装 `pi --mode rpc` 的启动参数(纯函数,单测钉死形状)。
//
// 固定形状:cli --mode rpc [--name N] [--extension E] [extra...]
//
// 【0.87.1 实测合法(2026-09-25 弹窗治理专项)】"cli --mode rpc --no-session" 在
// 0.87.1 下完整可用:get_available_models 应答 success=true 且返回全量模型表
// (探针留档:启动 30s+ 不退、stdin JSONL 协议正常)。此前 AUDIT 挂牌的"0.87.1
// rpc 模式拉起即退"是误读 —— daemon.log 里的"拉起→1~9s 后退出(退出码=1)"
// 全部是 fetchModels 一次性探测的正常 Close 收尸:Windows TerminateProcess 的
// 退出码恒为 1,与崩溃无关(处置见 agent/pi.go fetchModelsUncached 的完成日志
// 与并发去重)。
func buildRPCArgs(cli string, opts StartOptions) []string {
	args := []string{cli, "--mode", "rpc"}
	if opts.Name != "" {
		args = append(args, "--name", opts.Name)
	}
	if opts.GuardExtension != "" {
		args = append(args, "--extension", opts.GuardExtension)
	}
	args = append(args, opts.ExtraArgs...)
	return args
}

// Start 拉起 pi 子进程。直接 node 调 cli.js,不经 pi.ps1 垫片(省一层 shell,
// 避免 PowerShell 执行策略干扰 —— 调研 §2)。
func Start(opts StartOptions) (*Client, error) {
	node, cli, err := LocatePi()
	if err != nil {
		return nil, err
	}
	args := buildRPCArgs(cli, opts)
	cmd := exec.Command(node, args...)
	if opts.Cwd != "" {
		cmd.Dir = opts.Cwd
	}
	cmd.Env = os.Environ()
	// 协作 MCP 接线(T-L2pi-prod):总开关开启且 daemon 自有 agent 目录就绪时
	// 追加 PI_CODING_AGENT_DIR;其余情况返回空串,行为与现状零差异。
	// 门控细节见 mcpsetup.go 的 CollabEnvInjection。
	if e := CollabEnvInjection(); e != "" {
		cmd.Env = append(cmd.Env, e)
	}
	// 【凭据注入(2026-10-01)】pi 的 BYOK provider(deepseek 等)从 daemon 数据目录
	// 的白名单凭据文件取 key —— 与 dsh 完全同一机制、同一白名单、同一份数据;
	// 文件缺失/无匹配键 = 静默零注入,行为与现状完全一致。
	cmd.Env = append(cmd.Env, dshapp.LoadCredentialEnv("")...)
	hidewin.Apply(cmd) // 控制台程序:不隐藏会在用户桌面闪黑窗(与 codexapp 同款)
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
		// 【死因日志】启动失败的原因必须落 daemon.log(生产环境 os.Stderr 无人接收)。
		log.Printf("[pi] 启动 pi rpc 子进程失败: node=%s args=%v err=%v", node, args, err)
		return nil, fmt.Errorf("启动 pi rpc 子进程失败: %w", err)
	}
	log.Printf("[pi] pi 子进程已拉起 PID=%d cwd=%q args=%v", cmd.Process.Pid, cmd.Dir, args)
	c := &Client{
		cmd:     cmd,
		stdin:   stdin,
		stdout:  bufio.NewReaderSize(stdout, 8*1024*1024), // 图片 base64 单行数 MB
		pending: map[string]chan piResp{},
		deathCh: make(chan struct{}),
		reaped:  make(chan struct{}),
	}
	c.syncHandlers() // readLoop 的读面就位(此刻 handler 均为 nil:上层 Start 之后才装)
	go c.readLoop()
	go c.readStderr(stderr)
	return c, nil
}

// readStderr 收 pi 的诊断输出(stderr 只放日志,不混协议 —— 调研 §3.2#2)。
//
// 【2026-09-19 改】不再依赖 PA_PI_DEBUG 门槛、不再写 os.Stderr:生产 daemon 由
// paguard/WMI 拉起(CREATE_NO_WINDOW,无控制台),os.Stderr 无人接收,pi 崩溃
// 的死因日志全丢("pi 子进程已退出: EOF" 无从诊断)。daemon 的标准 log 已被
// paguard 重定向进 daemon.log,这里一律走 log 包,pi 的 stderr 永远落文件。
func (c *Client) readStderr(r io.Reader) {
	// 【收口到 stderrdrain】裸扫 + 直传换成统一收口:补上**凭据脱敏**(api_key /
	// token / JWT / Bearer 不落盘)与**总量限流**(刷屏不撑爆 daemon.log),并与
	// codex / zcode / dsh 三家口径一致。前缀与"stderr 只落日志、不混协议"的
	// 纪律不变。
	stderrdrain.Drain(r, stderrdrain.Options{}, func(l string) {
		log.Printf("[pi-stderr] %s", l)
	})
}

func debug() bool { return os.Getenv("PA_PI_DEBUG") != "" }

// piResp 一次命令应答:成功带 data,失败带 error。用结构体让"失败"无法被
// 当成"空结果"混过去(与 codexapp.rpcResp 同纪律)。
type piResp struct {
	data  json.RawMessage
	err   error
	rawOK bool // success 字段的原始值(nil = 缺失)
}

// CallCtx 发命令并等应答。
//
// 纪律(与 codexapp.Client.CallCtx 相同,都是被线上问题逼出来的):
//  1. success:false 必须变 Go error,不能把 {"success":false} 塞进返回值;
//  2. 必须有超时:子进程不回应答时绝不能无限阻塞(prompt 受理后的失败走事件流,
//     不算这里 —— 这里只等 {"type":"response"} 帧)。
func (c *Client) CallCtx(ctx context.Context, typ string, params map[string]any, timeout time.Duration) (json.RawMessage, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	c.mu.Lock()
	if c.dead {
		err := c.deathErr
		c.mu.Unlock()
		return nil, fmt.Errorf("%w: %v", errPiGone, err)
	}
	c.nextID++
	id := fmt.Sprintf("pa-%d", c.nextID)
	ch := make(chan piResp, 1)
	c.pending[id] = ch
	c.mu.Unlock()

	// 同步点:发命令前重新发布 handler 快照,保证「装卸 handler → CallCtx →
	// 事件到达」时序下 readLoop 一定能看到最新 handler(并发纪律见 Client 字段注释)。
	c.syncHandlers()

	obj := make(map[string]any, len(params)+2)
	for k, v := range params {
		obj[k] = v
	}
	obj["type"] = typ
	obj["id"] = id
	raw, err := json.Marshal(obj)
	if err != nil {
		c.forget(id)
		return nil, err
	}
	if err := c.writeLine(raw); err != nil {
		c.forget(id)
		return nil, fmt.Errorf("写入 %s 失败: %w", typ, err)
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case resp := <-ch:
		if resp.err != nil {
			return nil, fmt.Errorf("%s: %w", typ, resp.err)
		}
		return resp.data, nil
	case <-ctx.Done():
		c.forget(id)
		return nil, fmt.Errorf("%s: %w", typ, ctx.Err())
	case <-timer.C:
		c.forget(id)
		return nil, fmt.Errorf("%s: %w(%s)", typ, errNoAnswer, timeout)
	}
}

// Call 无 ctx 便捷版(默认 120s 兜底)。
func (c *Client) Call(typ string, params map[string]any, timeout time.Duration) (json.RawMessage, error) {
	return c.CallCtx(context.Background(), typ, params, timeout)
}

func (c *Client) forget(id string) {
	c.mu.Lock()
	delete(c.pending, id)
	c.mu.Unlock()
}

// writeBudget 单帧写入 stdin 的等待预算。pi 的单行可带数 MB base64 图片
// (读缓冲 8MB 的另一半),子进程卡死/不读管道时裸 Write 会无限阻塞 —— 持
// writeMu 期间堵死会把所有后续写(乃至 Close)一起冻住。
const writeBudget = 5 * time.Second

// writeWithBudget 在 budget 时间内尝试完成一次 stdin 写。写序语义不变:
// Write 仍受 writeMu 串行化(预算只限制"等 Write 返回"的时长,不绕过互斥)。
//
// 【已知语义】超时返回 error 后,执行 Write 的 goroutine 无法取消,**写可能
// 仍在后台完成**(帧最终可能落进管道)。调用方按失败处理即可:协议层要么
// 放弃本轮、要么随子进程一起被判死;os.File 允许多 goroutine 并发调用,
// 超时后残留的 Write 与后续 Write/Close 并发不会 panic。
func (c *Client) writeWithBudget(data []byte, budget time.Duration) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	errCh := make(chan error, 1)
	go func() {
		_, err := c.stdin.Write(data)
		errCh <- err
	}()
	timer := time.NewTimer(budget)
	defer timer.Stop()
	select {
	case err := <-errCh:
		return err
	case <-timer.C:
		return fmt.Errorf("写入 pi 子进程 stdin 超时(>%s),写可能仍在后台完成", budget)
	}
}

func (c *Client) writeLine(raw []byte) error {
	return c.writeWithBudget(append(raw, '\n'), writeBudget)
}

// RespondUI 回写 extension_ui_response(dialog 应答)。
// payload 的形状跟 method 走:select/input/editor 用 {"value":...},
// confirm 用 {"confirmed":bool};任何 dialog 都可用 {"cancelled":true} 兜底拒绝。
func (c *Client) RespondUI(id string, payload map[string]any) error {
	obj := make(map[string]any, len(payload)+2)
	for k, v := range payload {
		obj[k] = v
	}
	obj["type"] = "extension_ui_response"
	obj["id"] = id
	raw, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	return c.writeLine(raw)
}

func (c *Client) readLoop() {
	for {
		line, err := c.stdout.ReadBytes('\n')
		if len(line) > 0 {
			// [pi-raw] 是 stdout 原始行,正常对话高频:保留 PA_PI_DEBUG 门槛防刷爆
			// daemon.log,但输出通道同样改走 log 包(生产 stderr 无人接收)。
			if debug() {
				log.Printf("[pi-raw] %.400s", line)
			}
			c.dispatch(line)
		}
		if err != nil {
			// 子进程退出:置死、叫醒所有等待者(上层 Run 会转 TurnFailed,
			// 不让手机继续转圈 —— 调研 §8.4)。
			c.mu.Lock()
			c.dead = true
			c.deathErr = err
			for id, ch := range c.pending {
				ch <- piResp{err: fmt.Errorf("%w: %w", errPiGone, err)}
				delete(c.pending, id)
			}
			c.mu.Unlock()
			c.deathOnce.Do(func() { close(c.deathCh) })
			// 【死因日志】退出码必须落 daemon.log:EOF 只有症状,退出码+stderr
			// 才是死因。与 Close 收尸共用 reapWithLog(Once 保证只 Wait 一次)。
			go c.reapWithLog("意外退出(stdout 关闭)")
			return
		}
	}
}

// PID 返回子进程 PID(日志定位用);未知返回 0。
func (c *Client) PID() int {
	if c.cmd != nil && c.cmd.Process != nil {
		return c.cmd.Process.Pid
	}
	return 0
}

// reapWithLog 收尸(cmd.Wait)并把退出码写进标准 log(daemon.log)。
// 意外退出(readLoop EOF)与 Close 主动关闭两条路径共用;waitOnce 保证
// cmd.Wait 只执行一次(exec.Cmd 不允许二次 Wait)。正常 Kill 关闭时退出码
// 非 0 属预期,日志里带 why 字段区分,不会被误读成崩溃。
func (c *Client) reapWithLog(why string) {
	if c.cmd == nil {
		return
	}
	c.waitOnce.Do(func() {
		err := c.cmd.Wait()
		code := -1
		var ee *exec.ExitError
		if errors.As(err, &ee) && ee.Exited() {
			code = ee.ExitCode()
		}
		log.Printf("[pi] pi 子进程退出(%s) PID=%d 退出码=%d err=%v", why, c.PID(), code, err)
		if c.reaped != nil {
			c.reapOnce.Do(func() { close(c.reaped) })
		}
	})
}

func (c *Client) dispatch(line []byte) {
	var m struct {
		Type string          `json:"type"`
		ID   json.RawMessage `json:"id"`
		Data json.RawMessage `json:"data"`
		OK   *bool           `json:"success"`
		Err  string          `json:"error"`
	}
	if json.Unmarshal(line, &m) != nil {
		return // 非 JSON 行:忽略(诊断可开 PA_PI_DEBUG)
	}
	switch m.Type {
	case "response":
		// pi 的应答 id 官方支持任意 JSON 值;我们只发 string。
		var id string
		if json.Unmarshal(m.ID, &id) != nil {
			return
		}
		c.mu.Lock()
		ch, ok := c.pending[id]
		delete(c.pending, id)
		c.mu.Unlock()
		if !ok {
			if hs := c.hs.Load(); hs != nil && hs.unknownResponse != nil {
				go hs.unknownResponse(append(json.RawMessage(nil), m.ID...), append(json.RawMessage(nil), line...))
			}
			return
		}
		if m.OK == nil || !*m.OK {
			ch <- piResp{err: errors.New(m.Err)}
			return
		}
		ch <- piResp{data: m.Data}
	case "extension_ui_request":
		req := parseUIRequest(line)
		if req == nil {
			return
		}
		if req.Dialog() {
			var handler func(*UIRequest) (map[string]any, bool)
			if hs := c.hs.Load(); hs != nil {
				handler = hs.ui
			}
			if handler == nil {
				// 没人接:立即 cancelled,防"无 timeout 永久挂起"(§4.6)。
				_ = c.RespondUI(req.ID, map[string]any{"cancelled": true})
				return
			}
			// 回写可能等手机/超时,绝不能堵 readLoop。
			go func(req *UIRequest) {
				payload, respond := handler(req)
				if respond {
					_ = c.RespondUI(req.ID, payload)
				}
			}(req)
		}
		// dialog 与 fire-and-forget 都作为事件分发(fire-and-forget 只记录/展示)。
		c.emit(Event{Type: m.Type, Raw: append([]byte(nil), line...)})
	default:
		c.emit(Event{Type: m.Type, Raw: append([]byte(nil), line...)})
	}
}

func (c *Client) emit(ev Event) {
	// 只读快照:与 provider 协程装卸裸字段解耦(并发纪律见 Client 字段注释)。
	hs := c.hs.Load()
	if hs == nil {
		return
	}
	if hs.global != nil {
		hs.global(ev)
	}
	if hs.ev != nil {
		hs.ev(ev)
	}
}

// parseUIRequest 解析 extension_ui_request(只取用得到的字段)。
func parseUIRequest(line []byte) *UIRequest {
	var raw struct {
		ID          string `json:"id"`
		Method      string `json:"method"`
		Title       string `json:"title"`
		Message     string `json:"message"`
		Options     []any  `json:"options"`
		Timeout     any    `json:"timeout"`
		Placeholder string `json:"placeholder"`
		Prefill     string `json:"prefill"`
	}
	if json.Unmarshal(line, &raw) != nil || raw.ID == "" {
		return nil
	}
	req := &UIRequest{ID: raw.ID, Method: raw.Method, Title: raw.Title, Message: raw.Message,
		Placeholder: raw.Placeholder, Prefill: raw.Prefill,
		Raw: append([]byte(nil), line...)}
	for _, o := range raw.Options {
		if s, ok := o.(string); ok {
			req.Options = append(req.Options, s)
		}
	}
	switch t := raw.Timeout.(type) {
	case float64:
		req.TimeoutMs = int64(t)
	case string:
		// 容错:有的版本可能给字符串数字
		var n int64
		if _, err := fmt.Sscanf(t, "%d", &n); err == nil {
			req.TimeoutMs = n
		}
	}
	return req
}

// Alive 报告子进程是否还活着。
func (c *Client) Alive() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return !c.dead
}

// DeathCh 返回死亡通知 channel(子进程退出时关闭)。
func (c *Client) DeathCh() <-chan struct{} { return c.deathCh }

// Close 杀掉子进程(轮次收尾/daemon 退出用)。
//
// 【必须收尸】exec.Cmd 明确要求 Wait 回收子进程,只 Kill 不 Wait 会泄漏进程
// 句柄/管道 —— 长跑 daemon 每次重挂载(旧 client Close)都泄一个。范式抄
// term/manager.go:Kill 后放独立 goroutine Wait,Close 本身不等(Kill 后
// Wait 通常毫秒级返回;重复 Close 只是多收一次 "already called" 错误,无害)。
func (c *Client) Close() {
	cmd := c.cmd
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	c.writeMu.Lock()
	_ = c.stdin.Close()
	c.writeMu.Unlock()
	if cmd != nil {
		go c.reapWithLog("Close 主动关闭")
	}
}

// ---- 会话模型(set_model)----

// piModelBudget set_model 的等待上界:目录查找 + 会话级状态切换,本地毫秒级;
// 30s 对齐 agent 侧 piCallBudget 的既有量级(此前调用点传的就是它)。
const piModelBudget = 30 * time.Second

// splitPiModelSpec 拆解 daemon 侧 pi 模型规格 "<provider>/<modelId>"
// (Thread.ModelID 去掉 "pi/" 前缀后的形态,如 "zai-coding-cn/glm-5.3")。
// 首段作 provider、其余整体作 modelId(模型 id 自身可含 "/",如 openrouter 的
// "meta-llama/…",不得误伤);拆解先例 = agent/pi_selection.go 的 strings.Cut。
func splitPiModelSpec(spec string) (provider, modelID string, ok bool) {
	provider, modelID, ok = strings.Cut(spec, "/")
	// 拒绝路径两段一律回空(全有或全无,拒绝结果不携带半截结论)
	if !ok {
		// 无 "/" = 无 provider 段:不得把整段塞回 provider(旧形状祸根),整体拒绝
		return "", "", false
	}
	if provider == "" || modelID == "" {
		return "", "", false
	}
	return provider, modelID, true
}

// SetModel 切换 Pi 会话当前模型。
//
// 【为什么必须拆出 provider】pi 的 set_model handler 按二元组定位模型:
//
//	find(m => m.provider === command.provider && m.id === command.modelId)
//
// provider 缺省(undefined)永不匹配 → "Model not found: undefined/…" → pi 层
// 沿用当前模型 —— 1.0.2 真机实证(15d/15e):选中的模型只生效一轮,随后跨轮次
// 静默回退。发送 {provider, modelId} 对 0.99.1 无害:JSON 帧多余字段被忽略,
// 且 0.99.1 起模型身份就是 provider+id 二元组(get_state.model 与
// models-store 均带 provider;daemon 0.99.1 生产路径 pi_selection.go 本就发送
// 此形状)。
// modelSpec 无 "/" 或段为空 = 无效规格:本地拒绝不发(旧形状发必败的
// {"modelId": spec},失败语义同样是"沿用当前模型",这里只是把失败提前且可判)。
func (c *Client) SetModel(ctx context.Context, modelSpec string) error {
	provider, modelID, ok := splitPiModelSpec(modelSpec)
	if !ok {
		return fmt.Errorf("无效 Pi 模型规格(需 provider/modelId): %q", modelSpec)
	}
	_, err := c.CallCtx(ctx, "set_model", map[string]any{"provider": provider, "modelId": modelID}, piModelBudget)
	return err
}

// ---- 会话挂载(daemon 重启后重挂载的握手,调研 §8.3:这一步不能省)----

// 就绪等待预算。【1.0.2 启动门控】pi 1.0.2 的 runRpcMode 在 `await
// rebindSession()`(扩展加载)完成**之后**才 attachJsonlLineReader 开始读
// stdin —— spawn 后立刻发送的命令会在管道里躺到扩展绑定结束(本机 4 个扩展包
// 含 LSP/会话回填实测 >15s,探针留档 Temp/pi-p0-probe.md 第 2/7/8 项:首轮
// get_state 应答实际到达但晚于 15s 预算)。单次预算因此只做"一次尝试"的上界,
// 拿不到应答就换 id 重试,直到 pi 就绪/定局。
const (
	// piReadyAttempt 单次尝试的等待上界。太长会把重试节奏拖慢(30s ctx 下只试
	// 两次),太短会在就绪前后空转:12s 在"秒级就绪"(0.99.1)与"几十秒就绪"
	// (1.0.2+重扩展)之间取衡。
	piReadyAttempt = 12 * time.Second
	// piReadyTotal 调用方 ctx 无 deadline 时的总上界(绝不能无限等)。
	piReadyTotal = 90 * time.Second
)

// callWithReadyRetry 发命令并等应答;应答未到(且子进程未死)就换 id 重试,
// 直到拿到答案 / success:false 等定局失败 / 子进程死亡 / 调用方 ctx 到期 /
// 总预算耗尽。
//
// 【为什么必须重试】见上方"1.0.2 启动门控":首条命令很可能在 pi 开始读 stdin
// 之前就已入管道。对 0.99.1 及更早(就绪前即消费)零差异:首次尝试毫秒级拿到
// 应答,循环当即返回 —— 无需版本探测,两代协议同一段代码。
//
// 【不重试什么】success:false(pi 已作答,如 switch_session 被扩展取消)、
// 子进程死亡(errPiGone)、调用方 ctx 取消/到期 —— 都是定局。超时重试时旧 id
// 已 forget,pi 就绪后补发的旧应答在 dispatch 落 unknown(生产无 handler,
// 静默丢弃,无害;帧很小,管道无压力)。
func (c *Client) callWithReadyRetry(ctx context.Context, typ string, params map[string]any, attemptBudget time.Duration) (json.RawMessage, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	total := piReadyTotal
	if dl, ok := ctx.Deadline(); ok {
		total = time.Until(dl)
		if total <= 0 {
			return nil, fmt.Errorf("%s: %w", typ, ctx.Err())
		}
	}
	deadline := time.Now().Add(total)
	for {
		raw, err := c.CallCtx(ctx, typ, params, attemptBudget)
		if err == nil || !errors.Is(err, errNoAnswer) {
			return raw, err
		}
		if !time.Now().Before(deadline) {
			return nil, err // 没时间再试:把最后一次未应答超时报出去
		}
	}
}

// CallWithReadyRetry 是 callWithReadyRetry 的导出面:agent 层模型探测的一次性
// 子进程(fetchModelsUncached → probePiModels)与常驻会话路径(GetState/
// SwitchSession)共用同一套「1.0.2 启动门控」就绪重试语义,不另写第二实现
// (注释互指:agent/pi.go 的 piModelsProbeAttempt 与本文件 piReadyAttempt 同值同义)。
func (c *Client) CallWithReadyRetry(ctx context.Context, typ string, params map[string]any, attemptBudget time.Duration) (json.RawMessage, error) {
	return c.callWithReadyRetry(ctx, typ, params, attemptBudget)
}

// GetState 拉当前会话状态并刷新 SessionFile/SessionID 缓存。
// 走就绪重试:它常是 spawn 后的第一条命令(新会话挂载路径),在 pi 1.0.2 的
// 启动门控下要等到扩展绑定完成才有应答。
func (c *Client) GetState(ctx context.Context) (map[string]any, error) {
	raw, err := c.callWithReadyRetry(ctx, "get_state", nil, piReadyAttempt)
	if err != nil {
		return nil, err
	}
	var data map[string]any
	if json.Unmarshal(raw, &data) != nil {
		return nil, fmt.Errorf("get_state 应答无法解析")
	}
	if sf, _ := data["sessionFile"].(string); sf != "" {
		c.updateSession(sf, "")
	}
	if sid, _ := data["sessionId"].(string); sid != "" {
		c.updateSession("", sid)
	}
	return data, nil
}

// SwitchSession 切到已有会话文件并校验(重挂载:先 switch,再 get_state 确认
// sessionFile 与预期一致 —— codex 那边"不 resume 就静默吞请求"的同款教训)。
// switch 走就绪重试:重挂载常发生在新 spawn 的子进程上(1.0.2 启动门控,见
// callWithReadyRetry);校验用的 GetState 自带同样的重试。
func (c *Client) SwitchSession(ctx context.Context, sessionFile string) error {
	raw, err := c.callWithReadyRetry(ctx, "switch_session", map[string]any{"sessionPath": sessionFile}, piReadyAttempt)
	if err != nil {
		return err
	}
	var data struct {
		Cancelled bool `json:"cancelled"`
	}
	if json.Unmarshal(raw, &data) == nil && data.Cancelled {
		return errors.New("switch_session 被扩展取消")
	}
	st, err := c.GetState(ctx)
	if err != nil {
		return fmt.Errorf("switch 后 get_state 校验失败: %w", err)
	}
	got, _ := st["sessionFile"].(string)
	if got != "" && !strings.EqualFold(filepath.Clean(got), filepath.Clean(sessionFile)) {
		return fmt.Errorf("switch_session 后会话不符: 期望 %s 实得 %s", sessionFile, got)
	}
	c.updateSession(sessionFile, "")
	return nil
}
