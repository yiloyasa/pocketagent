// Package codexapp 封装 `codex app-server` 子进程:JSON-RPC 2.0 over stdio,
// 每行一条消息。这是 OpenAI Codex 官方给 GUI 客户端(VS Code 扩展等)的协议。
//
// 【讲解】为什么桥接它而不是自己写 agent:codex app-server 自带 agent 循环、
// 沙箱、审批、MCP、ChatGPT 登录、会话管理——这些是 OpenAI 团队维护的,
// 我们只做"翻译官":手机 gRPC ↔ codex JSON-RPC。原作 Mobile Codex 就是这么干的。
package codexapp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/pocketagent/daemon/internal/hidewin"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

type Client struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader

	mu        sync.Mutex
	writeMu   sync.Mutex
	nextID    int64
	pending   map[int64]chan rpcResp
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
	// 反向请求(服务器 → 客户端,必须应答):审批等
	ReqHandler func(id int64, method string, params json.RawMessage)

	// Setter 写入与 CallCtx 快照读取均受 handlerMu 保护；dispatch 只读
	// 不可变快照且在锁外调用回调，回调可安全地发请求或卸载自身。
	hs atomic.Pointer[codexHandlers]
}

// codexHandlers 某一时刻的 handler 快照(发布后不可变)。
type codexHandlers struct {
	global func(method string, params json.RawMessage)
	notif  func(method string, params json.RawMessage)
	req    func(id int64, method string, params json.RawMessage)
}

// syncHandlers 兼容旧探针的串行赋值后 CallCtx；生产写入统一用 Setter。
func (c *Client) syncHandlers() {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.publishHandlersLocked()
}

func (c *Client) publishHandlersLocked() {
	c.hs.Store(&codexHandlers{global: c.GlobalHandler, notif: c.NotifHandler, req: c.ReqHandler})
}

func (c *Client) SetGlobalHandler(handler func(string, json.RawMessage)) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.GlobalHandler = handler
	c.publishHandlersLocked()
}

// SetTurnHandlers 原子装卸当轮通知/审批钩子，不影响常驻钩子。
func (c *Client) SetTurnHandlers(notif func(string, json.RawMessage), req func(int64, string, json.RawMessage)) {
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

// Start 拉起 codex app-server 子进程。
func Start() (*Client, error) {
	bin, err := LocateCodex()
	if err != nil {
		return nil, err
	}
	args := []string{"app-server"}
	// .cmd/.bat 不能被 CreateProcess 直接执行,要经 cmd /c
	if len(bin) >= 4 && (bin[len(bin)-4:] == ".cmd" || bin[len(bin)-4:] == ".bat") {
		args = append([]string{"/c", bin}, args...)
		bin = "cmd.exe"
	}
	cmd := exec.Command(bin, args...)
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
		cmd:     cmd,
		stdin:   stdin,
		stdout:  bufio.NewReaderSize(stdout, 4*1024*1024),
		pending: map[int64]chan rpcResp{},
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
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024) // 单行上限 1MB(stderr 是诊断文本,不是协议帧,给足余量)
	for sc.Scan() {
		line(sc.Text())
	}
	if err := sc.Err(); err != nil {
		line("读取异常: " + err.Error())
	}
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
		c.forget(id)
		return nil, fmt.Errorf("%s: %w", method, ctx.Err())
	case <-time.After(callTimeout):
		c.forget(id)
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

var errClientClosed = errors.New("codex app-server 连接已关闭")

func (c *Client) writeLine(raw []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_, err := c.stdin.Write(append(raw, '\n'))
	return err
}

// callTimeout 单条请求的等待上限。取值宽松(有些请求确实慢:插件安装、历史压缩),
// 但只要它有界,就不会出现"一个请求挂住、所有 codex 操作排队等它"。
const callTimeout = 120 * time.Second

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
// 【为什么必须区分 Respond 和 RespondError】反向请求的"拒绝"有两种完全不同的含义:
//   - 审批类(审批卡上点"拒绝"):这是一个**成功的应答**,内容是"用户不同意"。
//     必须走 Respond,形状是该请求规定的 decision 字段。
//   - 我们能力不足(不管理 ChatGPT 凭证、没有实现动态工具、不生成 attestation):
//     这时**不能**伪造一个 decision —— 对方按自己 schema 解析会失败或误判。
//     正确做法是回 JSON-RPC error,让它按"客户端不支持"处理。
//
// 早期实现把这两者混成一个 {"decision":"decline"},于是 token 刷新被当成
// "用户拒绝了刷新",请求语义被彻底改写。
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

func (c *Client) dispatch(m *wireMsg) {
	if m.Method != "" && m.ID != nil {
		// 服务器反向请求(审批等)。只读快照:与 Run 协程装卸裸字段解耦。
		if hs := c.hs.Load(); hs != nil && hs.req != nil {
			go hs.req(*m.ID, m.Method, m.Params)
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
	if m.ID != nil {
		c.mu.Lock()
		ch, ok := c.pending[*m.ID]
		delete(c.pending, *m.ID)
		c.mu.Unlock()
		if ok {
			if m.Error != nil {
				// 错误在传输层就转成 Go error(见 Call 的长注释:以前这里塞进
				// 返回的 JSON,调用点的 err 判断全是死分支)。
				ch <- rpcResp{err: fmt.Errorf("codex 返回错误 %d: %s", m.Error.Code, m.Error.Message)}
			} else {
				ch <- rpcResp{raw: m.Result}
			}
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
