// Package codexapp 封装 `codex app-server` 子进程:JSON-RPC 2.0 over stdio,
// 每行一条消息。这是 OpenAI Codex 官方给 GUI 客户端(VS Code 扩展等)的协议。
//
// 【讲解】为什么桥接它而不是自己写 agent:codex app-server 自带 agent 循环、
// 沙箱、审批、MCP、ChatGPT 登录、会话管理——这些是 OpenAI 团队维护的,
// 我们只做"翻译官":手机 gRPC ↔ codex JSON-RPC。原作 Mobile Codex 就是这么干的。
package codexapp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/pocketagent/daemon/internal/hidewin"
	"github.com/pocketagent/daemon/internal/stderrdrain"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

type Client struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader

	mu      sync.Mutex
	writeMu sync.Mutex
	nextID  int64
	pending map[int64]chan rpcResp
	// abandoned 记录"调用方已按预算放弃"的探测请求(见 CallProbeCtx / giveUp):
	// dispatch 收到无人认领的应答时先按调用代号(id)查这里,命中即静默丢弃 ——
	// 不进 UnknownResponseHandler、不进未知事件台账。与 pending 同受 mu 保护。
	abandoned map[int64]abandonedProbe
	handlerMu sync.Mutex

	// waitOnce 保证 cmd.Wait 只执行一次(exec.Cmd 不允许二次 Wait):意外退出
	// (readLoop EOF,要拿退出码写死因日志)与 Close 主动收尸两条路径共用
	// reapWithLog(与 piapp/zcodeapp 同款,2026-09-20 补,P1-7)。
	waitOnce sync.Once

	// 通知(服务器 → 客户端,无需应答):item 更新、delta、turn 生命周期
	//
	// 【两个钩子的分工 —— 别合并】PocketAgent 是"手机遥控",一轮任务的事件需要
	// 实时转给手机,所以 NotifHandler 由 Run() 按轮次装上/卸下。但有一类通知
	// 跟"有没有在跑任务"无关:审查结论、会话归档/删除、队列变化、技能增删。
	// 它们以前全部落空 —— 因为不在轮次里就没人接(表现为手机上永远看不到
	// "审查者驳回了",以及电脑上删掉的会话在手机上还活着)。
	// 因此再加一个**常驻**的 GlobalHandler:子进程活着就一直挂着,
	// 负责把这类"后台事实"记下来。
	// Deprecated: 裸字段仅兼容独占客户端的旧探针。并发路径必须用 Setter；
	// 直接赋值不得与 CallCtx、SyncHandlers 或任何 Setter 并发。
	NotifHandler func(method string, params json.RawMessage)
	// GlobalHandler 常驻通知钩子(先于 NotifHandler 调用,只做记录,不做流式渲染)
	GlobalHandler func(method string, params json.RawMessage)
	// 反向请求(服务器 → 客户端,必须应答):审批等。id 用 RawMessage 原样透传
	// (数字 id 原值;字符串等非数字 id 原文),应答经 Respond/RespondError 原样
	// 回显 —— wire 合同不变,没有中间句柄(句柄桥已于 2026-10-04 删除)。
	ReqHandler             func(id json.RawMessage, method string, params json.RawMessage)
	UnknownResponseHandler func(id json.RawMessage, raw json.RawMessage)

	// Setter 写入与 CallCtx 快照读取均受 handlerMu 保护；dispatch 只读
	// 不可变快照且在锁外调用回调，回调可安全地发请求或卸载自身。
	hs atomic.Pointer[codexHandlers]
}

// codexHandlers 某一时刻的 handler 快照(发布后不可变)。
type codexHandlers struct {
	global          func(method string, params json.RawMessage)
	notif           func(method string, params json.RawMessage)
	req             func(id json.RawMessage, method string, params json.RawMessage)
	unknownResponse func(id json.RawMessage, raw json.RawMessage)
}

// syncHandlers 兼容旧探针的串行赋值后 CallCtx；生产写入统一用 Setter。
func (c *Client) syncHandlers() {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.publishHandlersLocked()
}

func (c *Client) publishHandlersLocked() {
	c.hs.Store(&codexHandlers{
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

func (c *Client) SetUnknownResponseHandler(handler func(json.RawMessage, json.RawMessage)) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.UnknownResponseHandler = handler
	c.publishHandlersLocked()
}

// SetTurnHandlers 原子装卸当轮通知/审批钩子，不影响常驻钩子。
func (c *Client) SetTurnHandlers(notif func(string, json.RawMessage), req func(json.RawMessage, string, json.RawMessage)) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.NotifHandler, c.ReqHandler = notif, req
	c.publishHandlersLocked()
}

// LocateCodex 找 codex 可执行文件。
func LocateCodex() (string, error) {
	if p := os.Getenv("CODEX_BIN"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
		return "", fmt.Errorf("CODEX_BIN 指向的文件不存在: %s", p)
	}
	for _, name := range []string{"codex.exe", "codex", "codex.cmd"} {
		if p, err := exec.LookPath(name); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("未找到 codex(可用环境变量 CODEX_BIN 指定路径)")
}

// daemonScopeArgs 是 daemon 实例的 app-server 专属配置覆盖(弹窗治理专项,
// 2026-09-25 用户红线反馈"电脑一直在弹窗")。全部经 `-c key=value` /
// `--disable FEATURE` 传入 —— codex 只在**本进程内存**里覆写,绝不写用户的
// ~/.codex/config.toml(0.156.1 实测:--disable X 等价于 -c features.X=false,
// 覆写不落盘;帮助文本原文"Equivalent to `-c features.<name>=false`")。
//
//  1. `notify=[]` —— 关掉"回合结束执行外部程序"钩子。config.toml 的 notify 会让
//     codex 在每轮 turn 结束时 spawn 一个外部 exe(本机装过 Codex 桌面版,该值指向
//     桌面通知程序 codex-computer-use.exe —— 每轮结束往桌面弹通知的源头;桌面运行
//     时目录升级后现值已指向不存在的 exe,变成每轮空拉起一次报错)。daemon 的轮
//     次事件自有协议流推送手机,永远不需要桌面通知。
//  2. `--disable apps` —— 关掉 ChatGPT Apps/插件 broker(codex-rs 的
//     codex_apps_server_config,端点 https://chatgpt.com/backend-api/ps/mcp)。
//     本机网络到不了 chatgpt.com,该 broker 的 rmcp transport worker 会以 20-30s
//     周期永久重连、往 stderr 刷 "worker quit with fatal: Transport channel
//     closed"(daemon.log [codex-stderr] 每 30s 刷屏的实锤源头)。daemon 不使用
//     apps 能力,关掉零损失;0.156.1 实测:禁用后 initialize 与
//     account/rateLimits/read 照常应答,stderr 不再出现 rmcp 报错。
//
// 回归钉子:start_args_test.go 以 fake 子进程捕获真实 argv 逐项断言。
var daemonScopeArgs = []string{"-c", "notify=[]", "--disable", "apps"}

// CollabExtraArgs 上层注入的可选 argv 追加器(internal/agent 协同接线,
// agent/collab_inject.go):每次拉起 app-server **现算**(数据目录在 daemon
// 启动期才注入,不能在包初始化时定格),返回的参数逐项追加在 daemonScopeArgs
// 之后;nil/空 = 零行为变化。红线同 daemonScopeArgs:覆盖只进本子进程内存,
// 绝不写用户 ~/.codex/config.toml。
var CollabExtraArgs func() []string

// Start 拉起 codex app-server 子进程。
func Start() (*Client, error) {
	bin, err := LocateCodex()
	if err != nil {
		return nil, err
	}
	args := append([]string{"app-server"}, daemonScopeArgs...)
	if CollabExtraArgs != nil {
		args = append(args, CollabExtraArgs()...)
	}
	// .cmd/.bat 不能被 CreateProcess 直接执行,要经 cmd.exe 启动;含空格路径的
	// 引证形态必须走 scriptCmdLine 的规范式,不能依赖 exec.Command 的默认逐参
	// 拼装(踩坑证据见 scriptCmdLine 注释)。SysProcAttr.CmdLine 是"逐字透传"
	// 字段(hidewin.Apply 只补 HideWindow/CreationFlags,不覆盖它)。
	var cmd *exec.Cmd
	if len(bin) >= 4 && (bin[len(bin)-4:] == ".cmd" || bin[len(bin)-4:] == ".bat") {
		cmd = exec.Command("cmd.exe")
		cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: scriptCmdLine(bin, args)}
	} else {
		cmd = exec.Command(bin, args...)
	}
	cmd.Env = os.Environ()
	hidewin.Apply(cmd) // codex 是控制台程序:不隐藏会在用户桌面闪黑窗
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	// stderr 单独收进 daemon 日志(绝不混进 stdout —— 那里只允许 JSON-RPC 协议帧;
	// 也绝不透传 os.Stderr:生产 daemon 由 paguard/WMI 拉起(CREATE_NO_WINDOW,
	// 无控制台),os.Stderr 无人接收,codex 的死因输出全丢 —— piapp/zcodeapp
	// 当年同款病,2026-09-20 补齐 codexapp,范式与二者一致)。
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动 codex app-server 失败: %w", err)
	}
	c := &Client{
		cmd:       cmd,
		stdin:     stdin,
		stdout:    bufio.NewReaderSize(stdout, 4*1024*1024),
		pending:   map[int64]chan rpcResp{},
		abandoned: map[int64]abandonedProbe{},
	}
	c.syncHandlers() // readLoop 的读面就位(此刻 handler 均为 nil:上层 Start 之后才装)
	go c.readLoop()
	// stderr 逐行落 daemon.log([codex-stderr] 前缀,与 [pi-stderr]/[zcode-stderr]
	// 同款)。子进程退出 → 管道写端关闭 → drainStderr 读到 EOF 自然返回,goroutine
	// 不泄漏;收尸(cmd.Wait)由 reapWithLog 独占(waitOnce 保证只一次),这里
	// 绝不重复 Wait(exec.Cmd 不允许二次 Wait)。
	go drainStderr(stderr, func(l string) { log.Printf("[codex-stderr] %s", l) })
	return c, nil
}

// scriptCmdLine 构造 cmd.exe 的**逐字命令行**(经 SysProcAttr.CmdLine 透传,
// 绕开 exec.Command 的逐参拼装):
//
//		cmd.exe /d /s /c ""<script>" <arg1> <arg2>…"
//
//	  - /d:跳过注册表 AutoRun 脚本,spawn 形态确定;
//	  - /s + 最外层一对引号:cmd 对 /c 余文的引号剥离规则就此**恒定** —— 永远
//	    只剥最外层一对,得到的执行文是 `"<script>" args...`,脚本路径的引号
//	    无论如何不丢。脚本路径段 = `"` + bin + `"`(在最外层引号内自带一对
//	    引号,与外层包裹叠加后线上呈现为 `""<script>"`);
//	  - 各参数仍逐个 syscall.EscapeArg 转义(含空格的参数在子进程侧按 CRT 规则
//	    正确还原),与最外层剥离互不干扰。
//
// 【为什么不能用旧形态 exec.Command("cmd.exe", "/c", bin, args...)】旧形态靠
// Go 逐参转义得到 `cmd.exe /c "G:\Claude Code\codex.cmd" app-server …`,cmd 对
// /c 余文有两条互斥规则:引号**恰好两个**且中间无特殊字符时原样保留;否则落
// "剥首字符 + 剥最后一个引号"的旧规则。只要行内任何一个参数自带引号,引号数
// 一超过 2 就落旧规则 —— 脚本路径的引号被剥掉,cmd 把第一个空格前的
// `G:\Claude` 当命令名去执行。活体证据(本机 daemon.log,系统级 setx 遗留的
// CODEX_BIN=G:\Claude Code\codex.cmd,路径含空格):
//
//	'G:\Claude' 不是内部或外部命令…
//
// codex provider 因此在本机整体不可用。协同注入(collab_inject 的
// `-c mcp_servers.*.command="…"` 值自带引号)会让该失效从"偶发"变"必现"。
// 本函数的规范引证形态对"其余参数带不带引号"免疫,是对症的修法。
func scriptCmdLine(bin string, args []string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, `"`+bin+`"`) // 脚本路径在最外层引号内自带一对引号
	for _, a := range args {
		parts = append(parts, syscall.EscapeArg(a))
	}
	return "cmd.exe /d /s /c " + `"` + strings.Join(parts, " ") + `"`
}

// drainStderr 逐行收子进程 stderr,每行交给 line 回调(生产注入 log.Printf 落
// daemon.log,测试注入 bytes.Buffer 收集器)。核心抽成自由函数以便单测 ——
// 管道与进程生命周期缠在一起时无法直接测(AUDIT §4:缺陷即测试)。
//
// 终止条件:子进程退出 → 管道写端关闭 → Scanner 读到 EOF → 本函数返回,
// 调用方 goroutine 随之结束,不泄漏。收尸(cmd.Wait)归 reapWithLog(独占),
// 这里绝不 Wait。非 EOF 的读取错误(如收尸方先一步关了管道)记一行回调即返回,
// 不 panic、不重试 —— 空行原样透传、无换行的截断行(进程死于半行输出)照收,
// 死因诊断一行都不能丢。
func drainStderr(r io.Reader, line func(string)) {
	// 【收口到 stderrdrain】原实现是裸扫 + 直传,缺两样:①不看内容,**凭据
	// (api_key / token / JWT / Bearer)会原样落进 daemon.log**;②没有总量上限,
	// 子进程疯狂刷屏时能把日志撑爆、把真正的死因淹掉。stderrdrain 补齐这两点,
	// 并把 pi / zcode / dsh 三家的同款逻辑收成一份(此前各抄一遍,口径已漂移)。
	// 签名与"逐行回调"语义保持不变,既有调用方与单测零改动。
	stderrdrain.Drain(r, stderrdrain.Options{}, line)
}

type wireMsg struct {
	JSONRPC string `json:"jsonrpc"`
	// ID 用 RawMessage:JSON-RPC 2.0 的 id 允许 number|string(还有 null=无 id)。
	// 自家请求恒发数字 id(见 call),但服务器侧的 id 形状不受我们控制 ——
	// zcode 桥实测其反向请求 id 就是字符串,codex 未来版本同样可能切换。
	// 旧实现锁死 *int64:一行 "id":"req-1" 会让整帧 json.Unmarshal 失败,
	// readLoop 只剩一行 debug 日志就丢帧 —— 审批请求、事件、应答全部静默
	// 消失且无台账。改 RawMessage 后按形状分发,见 dispatch。
	ID     json.RawMessage `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data,omitempty"`
	} `json:"error,omitempty"`
}

// Call 发请求并等待应答。
//
// 【两条不容商量的纪律 —— 都是被线上问题逼出来的】
//
//  1. **JSON-RPC 错误必须变成 Go error**。以前这里把 codex 的错误原样塞进返回的
//     JSON(`{"error":"..."}`)、err 却返回 nil,于是每个调用点的 `if err != nil`
//     全是死分支:codex 说"这个会话没找到",我们当成"成功了,拿到空结果"。
//     后果很具体 —— turn/start 失败被当成成功后,手机端只会一直转圈直到
//     150 秒看门狗才报错;thread/resume 失败被当成成功,后续 turn/start 打到
//     一个子进程不认识的线程上,codex 静默吞掉、什么都不发生。
//     现在错误在传输层就转成 error,调用点写不写判断都不会再被误导。
//
//  2. **必须有超时**。应答靠 readLoop 投递,子进程一旦不回(协议不支持、
//     内部死锁、被沙箱拦住),这里会**永远**阻塞。而调用方大多是在 withClient
//     持锁的情况下调它 —— 一个请求挂住,后面所有 codex 操作全部排队等待。
//     宁可口齿清楚地报超时,也不要一个不响的死锁。
//
// 【为什么还要 CallCtx】callTimeout(120s)是"最后的兜底",对"探测 codex 是否
// 可达"这类调用太宽松:调用方只想等几秒。没有 ctx 出入口时,调用方要么陪着
// 等满 120s,要么用 goroutine 包一层 —— 后者会留下一个还持着锁的僵尸 goroutine
// (本机实测过的坑)。CallCtx 让超时由 context 负责:到期即返回,不留后台 goroutine,
// 也不占着调用方的锁。
func (c *Client) CallCtx(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	return c.call(ctx, method, params, false)
}

// CallProbeCtx 是**只读探测轮询**专用的 CallCtx 变体(account/rateLimits/read、
// account/usage/read 这类"问一句答一句"的状态查询,2026-10-01 起 fetchUsageUncached
// 的两个用量探测走这里)。
//
// 【与 CallCtx 唯一的差别】探测按预算放弃后,app-server 迟到补发的应答**静默丢弃**,
// 不再进 UnknownResponseHandler / 未知事件台账:
//   - 背景:app-server 对这两个方法要先等自己的上游(chatgpt.com backend),实测
//     慢路径远超 daemon 的 10s 探测预算;daemon 超时降级后,上游失败才姗姗补发
//     error 帧(daemon.log + 台账 200 条滚动窗实证:每个探测窗积 24 条;上一轮已
//     把它们收编为 KNOWN_PASSTHROUGH,但只是分类,源头仍在)。
//   - 机制:放弃时把请求 id 记入 abandoned(见 giveUp);dispatch 对无人认领的
//     应答先查 abandoned,命中即丢弃(见 dispatch / takeAbandoned)。一次应答只丢
//     一次:同 id 再来的帧视为真异常,回退走兜底留痕。
//   - 边界:仅探测调用方(CallProbeCtx)选择加入;CallCtx 的迟到应答语义一字不变,
//     普通孤儿帧仍走 UnknownResponseHandler 留痕(不碰其他探测,红线)。
func (c *Client) CallProbeCtx(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	return c.call(ctx, method, params, true)
}

// call 是 CallCtx / CallProbeCtx 的公共实现;probe=true 时,"放弃等待"除名 pending
// 之外还会把 id 记入 abandoned(迟到应答静默丢弃),其余行为逐字节一致。
func (c *Client) call(ctx context.Context, method string, params interface{}, probe bool) (json.RawMessage, error) {
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
	// 事件到达」时序下 readLoop 一定能看到最新 handler(并发纪律见 hs 字段注释;
	// 与 piapp.Client.CallCtx 的同款同步点)。
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

	select {
	case resp, ok := <-ch:
		if !ok {
			return nil, fmt.Errorf("%s: %w", method, errClientClosed)
		}
		if resp.err != nil {
			return nil, fmt.Errorf("%s: %w", method, resp.err)
		}
		return resp.raw, nil
	case <-ctx.Done():
		c.giveUp(id, method, probe)
		return nil, fmt.Errorf("%s: %w", method, ctx.Err())
	case <-time.After(callTimeout):
		c.giveUp(id, method, probe)
		return nil, fmt.Errorf("%s: 等待 codex 应答超时(%s)", method, callTimeout)
	}
}

// Call 是无 ctx 的便捷版:用 background,仍受 callTimeout 上界保护。
func (c *Client) Call(method string, params interface{}) (json.RawMessage, error) {
	return c.CallCtx(context.Background(), method, params)
}

func (c *Client) forget(id int64) {
	c.mu.Lock()
	delete(c.pending, id)
	c.mu.Unlock()
}

// abandonedProbe 一次"调用方已按预算放弃"的探测请求的记账:dispatch 用 id 认出
// 它的迟到应答并静默丢弃;method 仅用于 PA_CODEX_DEBUG 下的丢弃日志。
type abandonedProbe struct {
	method string
	at     time.Time
}

// abandonedProbeTTL 是已放弃探测 id 的记账保留期。迟到应答通常在 app-server 自己
// 的上游超时内就会到达、在此之前被丢弃;超过保留期还没来的(上游彻底失联)按过期
// 清理,防止 abandoned 无界增长。此后若真有应答到达,回退走 UnknownResponseHandler
// 留痕 —— 极端陈旧的孤儿帧仍可诊断,不该永久隐身。
const abandonedProbeTTL = 10 * time.Minute

// giveUp 在调用方放弃等待时收尾:普通调用从 pending 除名(迟到应答此后照旧走
// UnknownResponseHandler 留痕);探测调用改为把 id 记入 abandoned(dispatch 据此
// 静默丢弃它的迟到应答)。同一把锁内完成除名与登记,分派侧不会看到中间态。
func (c *Client) giveUp(id int64, method string, probe bool) {
	c.mu.Lock()
	delete(c.pending, id)
	if probe {
		now := time.Now()
		for aid, ap := range c.abandoned {
			if now.Sub(ap.at) > abandonedProbeTTL {
				delete(c.abandoned, aid)
			}
		}
		if c.abandoned == nil {
			c.abandoned = map[int64]abandonedProbe{} // 直构 Client 的测试路径
		}
		c.abandoned[id] = abandonedProbe{method: method, at: now}
	}
	c.mu.Unlock()
}

// takeAbandoned 认领一条已放弃探测的迟到应答:命中即除名(一次应答只丢一次,
// 同 id 的重复帧回退走兜底,不当成无限重复的静默丢弃)。
func (c *Client) takeAbandoned(id int64) (abandonedProbe, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	ap, ok := c.abandoned[id]
	if ok {
		delete(c.abandoned, id)
	}
	return ap, ok
}

var errClientClosed = errors.New("codex app-server 连接已关闭")

// writeBudget 单次写 stdin 的预算。协议帧都极小(几 KB 内),管道缓冲即吞;
// 超过它还没写完,几乎只有一种解释 —— 子进程停止读 stdin(内部卡死/僵死),
// 再等下去只会让调用方 goroutine 超越自身 deadline(旧实现的实测病:writeLine
// 无期限阻塞,CallCtx 的超时形同虚设)。
const writeBudget = 5 * time.Second

func (c *Client) writeLine(raw []byte) error {
	return c.writeWithBudget(append(raw, '\n'), writeBudget)
}

// writeWithBudget 在 budget 内把 data(含结尾换行)写进子进程 stdin,超时返回
// 错误。本包私有(波 4 才考虑收编跨包公共件):piapp/zcodeapp 各有同款病灶,
// 本波不动它们。
//
// 【为什么锁在 goroutine 里拿】写必须互斥(writeMu),否则两帧字节交错、协议流
// 帧边界被毁。若沿用旧法"调用方先 Lock 再写",超时返回时锁就交出去了,而后台
// 那笔写可能仍在进行 —— 下一帧会与残尾巴帧交错。因此把「拿锁 → 写 → 放锁」
// 整体放进 worker:超时返回后 worker 仍持锁直到写真正结束,期间后续写(worker
// 卡在等锁)会同样超时失败,而不是写出一笔交错的坏帧。卡住的写随管道断开而
// 解除(Close 先 Kill,子进程一死管道读端关闭,Write 立刻带错返回)并放锁,
// 后续写自愈 —— 不需要"一旦超时就封死通道"的闩。
//
// 【超时语义】超时返回时,那笔写可能仍在后台完成(甚至最终成功)—— 一次已在
// 管道上的写无法撤销。调用方应把错误当"发送未确认"对待:上层(CallCtx 等)
// 自有超时/放弃语义接管,不假设字节写没写进去。
//
// 【代价】卡住期间每次写入会留一个阻塞在 Write/等锁上的 goroutine,持续到管道
// 断开(Close)为止 —— 调用方各有超时预算,堆积有界;远好于旧实现的"调用方
// goroutine 永久挂死"。
func (c *Client) writeWithBudget(data []byte, budget time.Duration) error {
	type writeOutcome struct{ err error }
	done := make(chan writeOutcome, 1) // 带缓冲:没人等结果也要送出并放锁
	go func() {
		c.writeMu.Lock()
		defer c.writeMu.Unlock()
		_, err := c.stdin.Write(data)
		done <- writeOutcome{err: err}
	}()
	timer := time.NewTimer(budget)
	defer timer.Stop()
	select {
	case o := <-done:
		return o.err
	case <-timer.C:
		return fmt.Errorf("写入 codex stdin 超过预算 %s:子进程可能已停止读取 stdin;本次写可能仍在后台完成(结果未知),期间后续写入将同样超时失败", budget)
	}
}

// callTimeout 单条请求的等待上限。取值宽松(有些请求确实慢:插件安装、历史压缩),
// 但只要它有界,就不会出现"一个请求挂住、所有 codex 操作排队等它"。
const callTimeout = 120 * time.Second

// Respond 应答服务器的反向请求(审批)。id 原样回显:数字 id 回数字、字符串等
// 非数字 id 原文透传 —— dispatch 交付给 ReqHandler 的就是线上原样 RawMessage,
// 这里不加任何变换(wire 合同不变)。曾经的中转负数句柄桥(登记/查表/TTL 全套)
// 已随签名的 RawMessage 化(2026-10-04)整体删除。
func (c *Client) Respond(id json.RawMessage, result interface{}) error {
	if !idPresent(id) {
		return fmt.Errorf("反向请求缺少可回应的 id,拒绝回写")
	}
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
// 【为什么必须区分 Respond 和 RespondError】反向请求的"拒绝"有两种完全不同的含义:
//   - 审批类(审批卡上点"拒绝"):这是一个**成功的应答**,内容是"用户不同意"。
//     必须走 Respond,形状是该请求规定的 decision 字段。
//   - 我们能力不足(不管理 ChatGPT 凭证、没有实现动态工具、不生成 attestation):
//     这时**不能**伪造一个 decision —— 对方按自己 schema 解析会失败或误判。
//     正确做法是回 JSON-RPC error,让它按"客户端不支持"处理。
//
// 早期实现把这两者混成一个 {"decision":"decline"},于是 token 刷新被当成
// "用户拒绝了刷新",请求语义被彻底改写。
func (c *Client) RespondError(id json.RawMessage, code int, message string) error {
	if !idPresent(id) {
		return fmt.Errorf("反向请求缺少可回应的 id,拒绝回写")
	}
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
	debug := os.Getenv("PA_CODEX_DEBUG") != ""
	for {
		line, err := c.stdout.ReadBytes('\n')
		if len(line) > 0 {
			if debug {
				// 原始帧日志:诊断"事件明明发了但收不到"类问题。走 log 落
				// daemon.log(与 [codex-stderr] 同款)—— 后台 daemon 无控制台,
				// os.Stderr 直写无人接收,调试帧全丢(T-24 回归:
				// client_debug_ast_test.go 钉死 readLoop 内不得再直写 os.Stderr)。
				log.Printf("[codex-debug] [raw] %.400s", line)
			}
			var m wireMsg
			if json.Unmarshal(line, &m) == nil {
				c.dispatch(&m)
			} else if debug {
				log.Printf("[codex-debug] [raw-UNPARSED] len=%d", len(line))
			}
		}
		if err != nil {
			// 子进程退出:叫醒所有等待者
			c.mu.Lock()
			for id, ch := range c.pending {
				ch <- rpcResp{err: errClientClosed}
				delete(c.pending, id)
			}
			c.mu.Unlock()
			// 【死因日志】EOF 只是症状,退出码才是死因:收尸并把退出码落
			// daemon.log(与 Close 收尸共用 reapWithLog,Once 保证只 Wait 一次;
			// 与 piapp/zcodeapp 同款,2026-09-20 补)。
			go c.reapWithLog("意外退出(stdout 关闭)")
			return
		}
	}
}

// idPresent 判断帧是否携带"可回应"的 id。JSON-RPC 2.0 的 id 允许 number|string,
// 而 id:null 按 spec 等价于"无 id"(请求无法被对应应答)。旧 *int64 解码会把
// null 变成 nil 指针、同样按无 id 处理 —— 本判断保持该语义(不做特判的话,
// json.RawMessage("null") 长度非零,会被误判成带 id)。
func idPresent(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	return !bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

func (c *Client) dispatch(m *wireMsg) {
	// 服务器反向请求(审批等,带 id,必须应答)。id 双形态全部**原样直通**:
	// 数字(现行 codex app-server)与非数字(字符串等,zcode 桥实测其服务器
	// 反向请求就是字符串)都按线上原文交给 handler,应答经 Respond/RespondError
	// 原样回显 —— 中间无句柄(负数句柄桥已随签名 RawMessage 化删除)。
	// 只读快照:与 Run 协程装卸裸字段解耦。
	if m.Method != "" && idPresent(m.ID) {
		if hs := c.hs.Load(); hs != nil && hs.req != nil {
			go hs.req(m.ID, m.Method, m.Params)
		}
		return
	}
	if m.Method != "" {
		// 先给常驻钩子(记录后台事实),再给当轮钩子(转流给手机)。
		// 顺序不能反:当轮钩子在 turn/completed 后会 finish(),之后的事件照样要落到
		// 常驻钩子里,否则"任务结束瞬间到达的归档/队列通知"又会丢。
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
	if idPresent(m.ID) {
		// 应答帧。自家请求 id 恒为数字(call 里 nextID 从 1 递增),只有数字
		// 才可能命中 pending/abandoned;字符串 id 的应答不可能是我们的 ——
		// 但绝不静默丢,照旧进 unknownResponse 台账留痕,id 参数原样透传
		// (RawMessage 签名后不再需要 0 占位:数字与非数字都交真实 id)。
		var id int64
		if err := json.Unmarshal(m.ID, &id); err == nil {
			c.mu.Lock()
			ch, ok := c.pending[id]
			delete(c.pending, id)
			c.mu.Unlock()
			if ok {
				if m.Error != nil {
					// 错误在传输层就转成 Go error(见 Call 的长注释:以前这里塞进
					// 返回的 JSON,调用点的 err 判断全是死分支)。
					ch <- rpcResp{err: fmt.Errorf("codex 返回错误 %d: %s", m.Error.Code, m.Error.Message)}
				} else {
					ch <- rpcResp{raw: m.Result}
				}
				return
			}
			if ap, abandoned := c.takeAbandoned(id); abandoned {
				// 迟到的探测应答:调用方早已按预算放弃并降级(用量面板如实显示
				// "读不到"),app-server 补发的上游错误是纯冗余证据 —— 静默丢弃,
				// 不进 UnknownResponseHandler、不进未知事件台账(源头治理,2026-10-01;
				// 先前的台账收编见 provider 侧 codexLateResponseLedger,仍作兜底)。
				// PA_CODEX_DEBUG 下留一行,与 readLoop 的原始帧日志同款口径。
				if os.Getenv("PA_CODEX_DEBUG") != "" {
					log.Printf("[codex-debug] 丢弃已放弃探测 %s(id=%d)的迟到应答", ap.method, id)
				}
				return
			}
		}
		if hs := c.hs.Load(); hs != nil && hs.unknownResponse != nil {
			raw, _ := json.Marshal(m)
			go hs.unknownResponse(m.ID, raw)
		}
	}
}

// rpcResp 一次应答:要么是结果,要么是错误。用结构体而不是裸 RawMessage,
// 就是为了让"错误"无法被当成"空结果"混过去。
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
		log.Printf("[codex] codex app-server 子进程退出(%s) PID=%d 退出码=%d err=%v", why, c.PID(), code, err)
	})
}

// Close 结束子进程并收割。
//
// 【为什么 Kill 后必须 Wait】exec.Cmd 明确要求调用方 Wait 收割子进程:只 Kill
// 不 Wait 会泄漏进程句柄与管道 —— 长跑 daemon 每次重启 codex 子进程都 Close
// 一次,泄漏会持续累积,且退出码无人记录、崩溃无从诊断(P1-7)。收尸走
// reapWithLog(退出码落 daemon.log),放专门 goroutine,不拖住 Close 调用方
// (piapp/zcodeapp 同款,范式来自 term/manager.go:Kill 后 goroutine Wait)。
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
