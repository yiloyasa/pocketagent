// Package dshapp 封装 `dsh --profile acp` 子进程:Agent Client Protocol v1 over
// stdio,换行分隔 JSON-RPC 2.0(每行一条消息)。这是 DeepSeek Harness 给外部
// 客户端的机器接口(dsh 0.1.5-rc.2 实测,docs/verify/evidence/dsh-p0-probe.md)。
//
// 【讲解】为什么桥接它而不是自己写 agent:dsh 自带 agent 循环、沙箱工具、
// 会话持久化/恢复、审批反向请求——这些是 DeepSeek 团队维护的,我们只做
// "翻译官"。架构完全复制 codexapp(一进程多会话 + JSON-RPC 2.0 + 反向审批),
// 与 piapp 的"1 子进程=1 会话"模型不同:dsh 一个子进程承载任意多会话。
//
// 【本包明确不做的事(设计蓝本 §4.2,如实写进文档)】
//   - Steerer 不实现:ACP 一会话一 prompt,运行中插话由上层 turn_queue 排队续跑;
//   - 审批策略/沙箱不映射:dsh ACP 固定一次性审批(allow/reject,无"始终允许"),
//     没有策略面可映射;
//   - 流式粒度按"块级"预期实现:agent_message_chunk 是已提交消息的文本块,
//     非逐字 delta(上游 discussion #594 提议中;探针受缺 key 阻塞未量化);
//   - 图片输入能力协商 promptCapabilities.image=false 时如实报错,不静默丢弃;
//   - 不读写用户自己的 ~/.dsh:会话/凭据全部隔离在 DSH_HOME(默认
//     ~/.pocketagentd/dsh-home,daemon 数据目录内;用户显式设过 DSH_HOME 则尊重),
//     凭据只透传 daemon 进程 env 的 DEEPSEEK_API_KEY,绝不碰 ~/.dsh/.credentials.yaml。
package dshapp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

// 【位置为什么交给 hostenv 派生】本包曾把开发机的 node.exe 与 dsh 入口写成常量
// (C:\Users\<开发机用户名>\... 与 G:\Claude Code\...)：Go 的 -trimpath 只剥源码
// 路径、不剥字符串常量，它们会随 pocketagentd.exe 分发出去（规格 §2 明令禁止），
// 且换一台机器必然不命中。版本锁定哲学不变（升级仍是显式动作），变的是"默认位置"
// 的求法 —— 按用户环境派生。回归钉子见 devpath_assert_test.go。

// dshBinRel 是 dsh 在 npm 包内的入口相对路径。
const dshBinRel = "@deepseek-ai/dsh/lib/bin.js"

// defaultDshHome 返回桥接专用隔离 HOME 的默认值(daemon 数据目录内)。
// 用户显式给 daemon 设过 DSH_HOME 时尊重其选择(蓝本 §5.3 隔离原则)。
func defaultDshHome() string {
	if dir := resolvedConfiguredDataDir(); dir != "" {
		return filepath.Join(dir, "dsh-home")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(os.TempDir(), "pocketagentd", "dsh-home")
	}
	return filepath.Join(home, ".pocketagentd", "dsh-home")
}

// LocateDsh 找 (node 可执行文件, dsh 入口脚本)。
//
// 优先级:DSH_BIN env(可指 dsh.cmd 或 bin.js,与 CODEX_BIN 同款)+ DSH_NODE env
// → 固定路径(node.exe + @deepseek-ai/dsh/lib/bin.js)→ PATH 兜底(dsh.exe/dsh/
// dsh.cmd)。WMI 派生环境不保证 PATH(piapp 同款教训),所以固定路径在 PATH 之前。
func LocateDsh() (node, bin string, err error) {
	// 入口:DSH_BIN → 随包目录/npm 全局探测 → PATH
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
		return "", "", fmt.Errorf("未找到 dsh 入口(可用环境变量 DSH_BIN 指定 dsh.cmd 或 lib/bin.js)")
	}
	if _, statErr := os.Stat(bin); statErr != nil {
		return "", "", fmt.Errorf("DSH_BIN 指向的文件不存在: %s", bin)
	}
	// .cmd/.bat 入口走 cmd /c,不需要 node(见 launchSpec)。
	if isCmdScript(bin) {
		return "", bin, nil
	}
	// node:DSH_NODE → 用户环境派生 → PATH
	node = strings.TrimSpace(os.Getenv("DSH_NODE"))
	if node == "" {
		node = hostenv.FindNode()
	}
	if node == "" {
		return "", "", fmt.Errorf("未找到 node(可用环境变量 DSH_NODE 指定路径)")
	}
	if _, statErr := os.Stat(node); statErr != nil {
		return "", "", fmt.Errorf("DSH_NODE 指向的文件不存在: %s", node)
	}
	return node, bin, nil
}

func isCmdScript(bin string) bool {
	l := strings.ToLower(bin)
	return strings.HasSuffix(l, ".cmd") || strings.HasSuffix(l, ".bat")
}

// launchSpec 把 (node, bin) 归一成 (可执行文件, 参数)。.cmd/.bat 不能被
// CreateProcess 直接执行,要经 cmd /c(codexapp 同款)。
func launchSpec(node, bin string) (string, []string) {
	if isCmdScript(bin) {
		return "cmd.exe", []string{"/c", bin, "--profile", "acp"}
	}
	return node, []string{bin, "--profile", "acp"}
}

// Version 取 dsh 版本(--version);失败返回空串。带 5 分钟缓存:GetHostInfo
// 被手机高频探活,而 --version 要拉一次 node 子进程(cachedPiVersion 同款取舍);
// 探测只做 --version,绝不做 ACP initialize 握手(PA-REAL-0002/0003:启动期
// 一个 agent 的冷启动不许拖住别人)。
var (
	dshVerCache string
	dshVerAt    time.Time
	dshVerMu    sync.Mutex
)

func Version(node, bin string) string {
	dshVerMu.Lock()
	if time.Since(dshVerAt) < 5*time.Minute {
		cached := dshVerCache
		dshVerMu.Unlock()
		return cached
	}
	dshVerMu.Unlock()
	v := versionOnce(node, bin)
	dshVerMu.Lock()
	dshVerCache = v
	dshVerAt = time.Now()
	dshVerMu.Unlock()
	return v
}

// versionOnce 实际执行 --version(无缓存)。独立成函数以便测试注入清理。
func versionOnce(node, bin string) string {
	if bin == "" {
		return ""
	}
	exe, args := launchSpecVersion(node, bin)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, args...)
	hidewin.Apply(cmd) // 控制台程序:不隐藏会在用户桌面闪黑窗(piapp 同款)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func launchSpecVersion(node, bin string) (string, []string) {
	if isCmdScript(bin) {
		return "cmd.exe", []string{"/c", bin, "--version"}
	}
	return node, []string{bin, "--version"}
}

// ---- 客户端 ----

// StartOptions 启动参数。
type StartOptions struct {
	// Cwd 子进程工作目录(session/new 的默认 cwd 锚点);空 = 继承 daemon。
	Cwd string
	// ExtraEnv 额外注入子进程的环境变量(KEY=VALUE 形式),放在自算的 DSH_HOME
	// 之后追加(语义为可覆盖)。当前用途:daemon 主动补充的 dsh 凭据
	// (dsh-credentials.env 解析结果),详见 dshapp.LoadCredentialEnv。
	// 注意凭据键已按"进程 env 优先"在解析处剔除重复,这里不会用文件值压过进程 env。
	ExtraEnv []string
	// DataDir daemon 数据目录;空时 dshapp 按默认 ~/.pocketagentd 解析
	// (尊重显式数据目录,但本包不读全局 flag,保持自洽;见 DefaultDataDir 注释)。
	DataDir string
}

// Client 管理一个 dsh --profile acp 子进程(一进程多会话,与 codexapp 同构)。
type Client struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader

	mu        sync.Mutex
	writeMu   sync.Mutex
	nextID    int64
	pending   map[int64]chan rpcResp
	dead      bool // readLoop EOF / Close 置位(与 pending 同锁);Alive 读它
	handlerMu sync.Mutex

	// waitOnce 保证 cmd.Wait 只执行一次(exec.Cmd 不允许二次 Wait):意外退出
	// (readLoop EOF,要拿退出码写死因日志)与 Close 主动收尸两条路径共用
	// reapWithLog(codexapp/piapp/zcodeapp 三方一致范式)。
	waitOnce sync.Once

	// NotifHandler 当轮通知钩子(Run 按轮装卸):session/update 流。
	// Deprecated: 裸字段仅兼容独占客户端的旧探针。并发路径必须用 Setter;
	// 直接赋值不得与 CallCtx、SyncHandlers 或任何 Setter 并发。
	NotifHandler func(method string, params json.RawMessage)
	// GlobalHandler 常驻通知钩子(先于 NotifHandler 调用,只做记录,不做流式渲染)。
	GlobalHandler func(method string, params json.RawMessage)
	// ReqHandler 反向请求(服务器 → 客户端,必须应答):session/request_permission 等。
	ReqHandler             func(id int64, method string, params json.RawMessage)
	UnknownResponseHandler func(id int64, raw json.RawMessage)

	// Setter 写入与 CallCtx 快照读取均受 handlerMu 保护;dispatch 只读不可变
	// 快照且在锁外调用回调,回调可安全地发请求或卸载自身(codexapp 同款)。
	hs atomic.Pointer[dshHandlers]
}

// dshHandlers 某一时刻的 handler 快照(发布后不可变)。
type dshHandlers struct {
	global          func(method string, params json.RawMessage)
	notif           func(method string, params json.RawMessage)
	req             func(id int64, method string, params json.RawMessage)
	unknownResponse func(id int64, raw json.RawMessage)
}

// SyncHandlers 是 syncHandlers 的导出版:provider 在轮次收尾卸下 handler 后
// 同步快照,readLoop 才能立刻看到卸载。
func (c *Client) SyncHandlers() { c.syncHandlers() }

func (c *Client) syncHandlers() {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.publishHandlersLocked()
}

func (c *Client) publishHandlersLocked() {
	c.hs.Store(&dshHandlers{
		global: c.GlobalHandler, notif: c.NotifHandler, req: c.ReqHandler,
		unknownResponse: c.UnknownResponseHandler,
	})
}

func (c *Client) SetGlobalHandler(handler func(string, json.RawMessage)) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.GlobalHandler = handler
	c.publishHandlersLocked()
}

func (c *Client) SetUnknownResponseHandler(handler func(int64, json.RawMessage)) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.UnknownResponseHandler = handler
	c.publishHandlersLocked()
}

// SetTurnHandlers 原子装卸当轮通知/审批钩子,不影响常驻钩子。
func (c *Client) SetTurnHandlers(notif func(string, json.RawMessage), req func(int64, string, json.RawMessage)) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.NotifHandler, c.ReqHandler = notif, req
	c.publishHandlersLocked()
}

// Start 拉起 dsh ACP 子进程。
//
// 【隔离】DSH_HOME 默认指向 ~/.pocketagentd/dsh-home(daemon 数据目录内):
// 会话/凭据与用户自己的 ~/.dsh 零共享。用户显式给 daemon 设过 DSH_HOME 时
// 尊重其选择(蓝本 §5.3 的进阶逃生门)。DEEPSEEK_API_KEY 随 os.Environ()
// 原样透传 —— 我们不添加、不存储、不读取任何凭据文件。
func Start(opts StartOptions) (*Client, error) {
	node, bin, err := LocateDsh()
	if err != nil {
		return nil, err
	}
	exe, args := launchSpec(node, bin)
	return startProcess(exe, args, opts)
}

// startProcess 是 Start 的进程拉起内核(exe/args 已归一),独立出来供测试注入
// 假子进程(os.Args[0] helper 模式,piapp/codexapp 同款)。
func startProcess(exe string, args []string, opts StartOptions) (*Client, error) {
	cmd := exec.Command(exe, args...)
	if opts.Cwd != "" {
		cmd.Dir = opts.Cwd
	}
	// DSH_HOME 缺省注入(用户已设则尊重);其余 env(含 DEEPSEEK_API_KEY)原样透传。
	env := os.Environ()
	if os.Getenv("DSH_HOME") == "" {
		home := defaultDshHome()
		if opts.DataDir != "" {
			home = filepath.Join(opts.DataDir, "dsh-home")
		}
		env = append(env, "DSH_HOME="+home)
	}
	// ExtraEnv 在 DSH_HOME 之后追加:允许上层用显式值覆盖前面已设项(如补 dsh 凭据);
	// 凭据键的"进程 env 优先"已在 LoadCredentialEnv 解析时保证,这里不重复判。
	if len(opts.ExtraEnv) > 0 {
		env = append(env, opts.ExtraEnv...)
	}
	cmd.Env = env
	hidewin.Apply(cmd) // 控制台程序:不隐藏会在用户桌面闪黑窗(与 codexapp 同款)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	// stderr 单独收进 daemon 日志(绝不混进 stdout —— 那里只允许 JSON-RPC 协议帧;
	// 也绝不透传 os.Stderr:生产 daemon 由 paguard/WMI 拉起,无控制台,透传即丢)。
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		log.Printf("[dsh] 启动 dsh acp 子进程失败: exe=%s args=%v err=%v", exe, args, err)
		return nil, fmt.Errorf("启动 dsh acp 子进程失败: %w", err)
	}
	log.Printf("[dsh] dsh 子进程已拉起 PID=%d args=%v DSH_HOME=%q", cmd.Process.Pid, args, os.Getenv("DSH_HOME"))
	c := &Client{
		cmd:     cmd,
		stdin:   stdin,
		stdout:  bufio.NewReaderSize(stdout, 8*1024*1024), // 单行帧要扛长上下文/图(piapp 64KB 必炸的教训,蓝本 §4.2 给足 8MB)
		pending: map[int64]chan rpcResp{},
	}
	c.syncHandlers() // readLoop 的读面就位(此刻 handler 均为 nil:上层 Start 之后才装)
	go c.readLoop()
	// stderr 逐行落 daemon.log([dsh-stderr] 前缀)。子进程退出 → 管道写端关闭 →
	// drainStderr 读到 EOF 自然返回,goroutine 不泄漏;收尸(cmd.Wait)由
	// reapWithLog 独占(waitOnce 保证只一次),这里绝不重复 Wait。
	go drainStderr(stderr, func(l string) { log.Printf("[dsh-stderr] %s", l) })
	return c, nil
}

// drainStderr 逐行收子进程 stderr,每行交给 line 回调(生产注入 log.Printf 落
// daemon.log,测试注入 bytes.Buffer 收集器)。核心抽成自由函数以便单测
// (codexapp 同款,缺陷即测试)。
func drainStderr(r io.Reader, line func(string)) {
	// 【收口到 stderrdrain】与 codexapp 同款改造:原实现裸扫 + 直传,凭据会原样
	// 落进 daemon.log,且没有总量上限。dsh 尤其敏感 —— 它的 stderr 可能吐出
	// 上游凭据链相关文本(provider route / credentials service),不脱敏等于把
	// 凭据写进磁盘日志。签名不变,调用方与单测零改动。
	stderrdrain.Drain(r, stderrdrain.Options{}, line)
}

type wireMsg struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data,omitempty"`
	} `json:"error,omitempty"`
}

// RPCError 是 dsh 返回的 JSON-RPC error(保真携带 code/message/data)。
//
// 【为什么用结构体而不是格式化字符串】探针实测(修正③):缺 key 错误 =
// code=-32603、无 data、message 含 "no API key"。上层要按 code+message 归一化成
// MISSING_CREDENTIAL(ErrorInfoCarrier),字符串格式化会把 code 弄丢。
type RPCError struct {
	Method  string
	Code    int
	Message string
	Data    json.RawMessage
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("%s: dsh 返回错误 %d: %s", e.Method, e.Code, e.Message)
}

// Call 发请求并等待应答(callTimeout=120s 兜底)。
//
// 两条纪律与 codexapp.CallCtx 完全同款(都是被线上问题逼出来的):
//  1. JSON-RPC 错误必须变成 Go error(*RPCError 保真携带 code/message);
//  2. 必须有超时:应答靠 readLoop 投递,子进程不回就永远阻塞。
func (c *Client) Call(method string, params interface{}) (json.RawMessage, error) {
	return c.CallCtx(context.Background(), method, params)
}

// CallCtx 是带 ctx 的版本:ctx 截止/取消即时返回,不留下持锁僵尸 goroutine。
func (c *Client) CallCtx(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	return c.callTimeout(ctx, method, params, callTimeout)
}

// PromptCtx 发起 session/prompt 并等终局。
//
// 【为什么不受 callTimeout=120s 管】prompt 的应答在轮次终局(end_turn/cancelled)
// 才回来,coding 轮次轻松超过 2 分钟(参考 codex 单轮上限 60 分钟)。这里只受
// 调用方 ctx 管(daemon 侧另有 150s 无事件看门狗与手机断连取消兜底,见
// agent/dsh.go),传输层不设兜底超时 —— 宁可信任看门狗,也不在轮次中途误杀。
func (c *Client) PromptCtx(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	return c.callTimeout(ctx, method, params, 0)
}

func (c *Client) callTimeout(ctx context.Context, method string, params interface{}, timeout time.Duration) (json.RawMessage, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	ch := make(chan rpcResp, 1)
	c.pending[id] = ch
	c.mu.Unlock()

	// 同步点:发命令前重新发布 handler 快照,保证「装卸 handler → CallCtx →
	// 事件到达」时序下 readLoop 一定能看到最新 handler(codexapp 同款)。
	c.syncHandlers()

	body := map[string]interface{}{"jsonrpc": "2.0", "id": id, "method": method}
	if params != nil {
		body["params"] = params
	}
	raw, err := json.Marshal(body)
	if err != nil {
		c.forget(id)
		return nil, err
	}
	if err := c.writeLine(raw); err != nil {
		c.forget(id)
		return nil, fmt.Errorf("写入 %s 失败: %w", method, err)
	}

	var timerC <-chan time.Time
	if timeout > 0 {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		timerC = timer.C
	}
	select {
	case resp, ok := <-ch:
		if !ok {
			return nil, fmt.Errorf("%s: %w", method, errClientClosed)
		}
		if resp.err != nil {
			return nil, withMethod(method, resp.err) // *RPCError 原样上浮,补齐 method 上下文
		}
		return resp.raw, nil
	case <-ctx.Done():
		c.forget(id)
		return nil, fmt.Errorf("%s: %w", method, ctx.Err())
	case <-timerC:
		c.forget(id)
		return nil, fmt.Errorf("%s: 等待 dsh 应答超时(%s)", method, timeout)
	}
}

// withMethod 给错误补齐发起方 method 上下文。*RPCError 是本次调用专属
// (pending channel 一对一),就地补字段安全;其他错误(如 errClientClosed 共享
// 哨兵)用 %w 包装,errors.Is/As 链不受影响。
func withMethod(method string, err error) error {
	var re *RPCError
	if errors.As(err, &re) {
		if re.Method == "" {
			re.Method = method
		}
		return re
	}
	return fmt.Errorf("%s: %w", method, err)
}

func (c *Client) forget(id int64) {
	c.mu.Lock()
	delete(c.pending, id)
	c.mu.Unlock()
}

var errClientClosed = errors.New("dsh 子进程连接已关闭")

// callTimeout 单条请求的等待上限(宽松兜底,与 codexapp 同值;prompt 不走它)。
const callTimeout = 120 * time.Second

// Notify 发通知(无需应答):session/cancel 用。
func (c *Client) Notify(method string, params interface{}) error {
	body := map[string]interface{}{"jsonrpc": "2.0", "method": method}
	if params != nil {
		body["params"] = params
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	return c.writeLine(raw)
}

// writeBudget 单帧写入 stdin 的等待预算。单行帧要扛长上下文/图(读缓冲 8MB
// 的另一半),子进程卡死/不读管道时裸 Write 会无限阻塞 —— 持 writeMu 期间
// 堵死会把所有后续写(乃至 Close)一起冻住。
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
		return fmt.Errorf("写入 dsh 子进程 stdin 超时(>%s),写可能仍在后台完成", budget)
	}
}

func (c *Client) writeLine(raw []byte) error {
	return c.writeWithBudget(append(raw, '\n'), writeBudget)
}

// Respond 应答服务器的反向请求(审批)。
func (c *Client) Respond(id int64, result interface{}) error {
	raw, err := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0", "id": id, "result": result,
	})
	if err != nil {
		return err
	}
	return c.writeLine(raw)
}

// RespondError 用 JSON-RPC error 应答反向请求。
//
// 【为什么必须区分 Respond 和 RespondError】与 codex_requests.go 同款纪律:
// 审批"拒绝"是**成功的应答**(内容=用户不同意,走 Respond,形状=ACP outcome);
// 我们能力不足(不认识的反向请求)**不能**伪造一个 decision —— 正确做法是回
// JSON-RPC error,让它按"客户端不支持"处理。绝不伪造"同意"(红线 1)。
func (c *Client) RespondError(id int64, code int, message string) error {
	raw, err := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"error":   map[string]interface{}{"code": code, "message": message},
	})
	if err != nil {
		return err
	}
	return c.writeLine(raw)
}

func (c *Client) readLoop() {
	debug := os.Getenv("PA_DSH_DEBUG") != ""
	for {
		line, err := c.stdout.ReadBytes('\n')
		if len(line) > 0 {
			if debug {
				// 原始帧日志:诊断"事件明明发了但收不到"类问题。走 log 落
				// daemon.log(与 [dsh-stderr] 同款)—— 后台 daemon 无控制台,
				// os.Stderr 直写无人接收(codexapp T-24 同款回归钉)。
				log.Printf("[dsh-debug] [raw] %.400s", line)
			}
			var m wireMsg
			if json.Unmarshal(line, &m) == nil {
				c.dispatch(&m)
			} else if debug {
				log.Printf("[dsh-debug] [raw-UNPARSED] len=%d", len(line))
			}
		}
		if err != nil {
			// 子进程退出:置死、叫醒所有等待者
			c.mu.Lock()
			c.dead = true
			for id, ch := range c.pending {
				ch <- rpcResp{err: errClientClosed}
				delete(c.pending, id)
			}
			c.mu.Unlock()
			// 【死因日志】EOF 只是症状,退出码才是死因:收尸并把退出码落
			// daemon.log(与 Close 收尸共用 reapWithLog,Once 保证只 Wait 一次)。
			go c.reapWithLog("意外退出(stdout 关闭)")
			return
		}
	}
}

func (c *Client) dispatch(m *wireMsg) {
	if m.Method != "" && m.ID != nil {
		// 服务器反向请求(session/request_permission 等)。只读快照:与 Run
		// 协程装卸裸字段解耦;回写可能等手机审批,绝不能堵 readLoop。
		if hs := c.hs.Load(); hs != nil && hs.req != nil {
			go hs.req(*m.ID, m.Method, m.Params)
		}
		return
	}
	if m.Method != "" {
		// 先给常驻钩子(记录后台事实),再给当轮钩子(转流给手机),顺序不能反。
		if hs := c.hs.Load(); hs != nil {
			if hs.global != nil {
				hs.global(m.Method, m.Params)
			}
			if hs.notif != nil {
				hs.notif(m.Method, m.Params)
			}
		}
		return
	}
	if m.ID != nil {
		c.mu.Lock()
		ch, ok := c.pending[*m.ID]
		delete(c.pending, *m.ID)
		c.mu.Unlock()
		if ok {
			if m.Error != nil {
				ch <- rpcResp{err: &RPCError{Code: m.Error.Code, Message: m.Error.Message, Data: m.Error.Data}}
			} else {
				ch <- rpcResp{raw: m.Result}
			}
		} else if hs := c.hs.Load(); hs != nil && hs.unknownResponse != nil {
			raw, _ := json.Marshal(m)
			go hs.unknownResponse(*m.ID, raw)
		}
	}
}

// rpcResp 一次应答:要么是结果,要么是错误。用结构体而不是裸 RawMessage,
// 就是为了让"错误"无法被当成"空结果"混过去(codexapp 同纪律)。
type rpcResp struct {
	raw json.RawMessage
	err error
}

// PID 返回子进程 PID(日志定位用);未知返回 0。
func (c *Client) PID() int {
	if c.cmd != nil && c.cmd.Process != nil {
		return c.cmd.Process.Pid
	}
	return 0
}

// Alive 报告子进程是否还活着(readLoop 已见 EOF 即死)。
// 注意:零值 Client(单测直构,未 Start)也按活处理 —— 上层以 Call 失败为准。
func (c *Client) Alive() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return !c.dead
}

// reapWithLog 收尸(cmd.Wait)并把退出码写进标准 log(daemon.log)。
// 意外退出(readLoop EOF)与 Close 主动关闭两条路径共用;waitOnce 保证
// cmd.Wait 只执行一次(exec.Cmd 不允许二次 Wait)。
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
		log.Printf("[dsh] dsh acp 子进程退出(%s) PID=%d 退出码=%d err=%v", why, c.PID(), code, err)
	})
}

// Close 结束子进程并收割(Kill 后必须 Wait 收尸,防句柄/管道泄漏 —— codexapp
// P1-7 同款;收尸放专门 goroutine,不拖住 Close 调用方)。
func (c *Client) Close() {
	c.mu.Lock()
	c.dead = true
	c.mu.Unlock()
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
