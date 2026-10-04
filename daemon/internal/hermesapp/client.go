package hermesapp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pocketagent/daemon/internal/hidewin"
	"github.com/pocketagent/daemon/internal/runtime"
)

// hermesHandlers 一组当轮钩子的原子快照:readLoop 每帧取一次快照,装卸
// (SetTurnHandlers)与分发并发安全(dshapp 同款机制;provider 的轮次串行化
// 保证同一时刻只有一轮在装钩子)。
type hermesHandlers struct {
	// Notif 每条 session/update 通知(流式投影的数据源;在 readLoop goroutine
	// 里同步调用,必须快进快出 —— 与 zcodeapp 通知钩子同一纪律)。
	Notif func(params json.RawMessage)
	// Req 服务器反向请求(session/request_permission 审批桥的数据面);nil 时
	// 该方法回落 -32601(不伪造 decision,与 dsh/codex 同红线)。
	Req func(id int64, method string, params json.RawMessage)
}

// Client 是一条 ACP stdio 连接(python -m acp_adapter)。
// 并发模型与 zcodeapp/dshapp 同族:请求用 pending map 关联应答,写入串行化,
// 通知(session/update 流)分发给钩子,收尸 waitOnce 保证只 Wait 一次。
type Client struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader

	mu        sync.Mutex
	writeMu   sync.Mutex
	nextID    int64
	pending   map[int64]chan rpcResp
	waitOnce  sync.Once
	closeOnce sync.Once

	handlerMu sync.Mutex
	hs        atomic.Pointer[hermesHandlers]

	exited chan struct{} // reap 完成后关闭(Alive 判定)
}

// SetTurnHandlers 原子装卸当轮通知/审批钩子(nil = 卸下),不影响 pending 应答。
func (c *Client) SetTurnHandlers(notif func(json.RawMessage), req func(id int64, method string, params json.RawMessage)) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.hs.Store(&hermesHandlers{Notif: notif, Req: req})
}

// loadHandlers 取当前钩子快照(无钩子返回 nil)。
func (c *Client) loadHandlers() *hermesHandlers { return c.hs.Load() }

// Respond 回服务器反向请求(审批桥的出口;result 形状见 PermissionOutcomeSelected)。
func (c *Client) Respond(id int64, result any) error {
	return c.writeFrame(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
}

// RespondError 以协议错误回反向请求(解析失败/审批通道缺失;绝不伪造 decision)。
func (c *Client) RespondError(id int64, code int, message string) error {
	return c.writeFrame(map[string]any{
		"jsonrpc": "2.0", "id": id,
		"error": map[string]any{"code": code, "message": message},
	})
}

type rpcResp struct {
	ID     int64
	Result json.RawMessage
	Err    *rpcError
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *rpcError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if s, ok := e.Data.(string); ok && s != "" {
		return fmt.Sprintf("hermes ACP %d: %s (%s)", e.Code, e.Message, truncateForLog(s, 300))
	}
	return fmt.Sprintf("hermes ACP %d: %s", e.Code, e.Message)
}

func truncateForLog(s string, n int) string {
	// 统一走 runtime.ClipRunesEllipsis,收敛见审查报告 util 节。
	return runtime.ClipRunesEllipsis(s, n)
}

// Start 拉起 ACP 适配器子进程(python -m acp_adapter,cwd=适配器目录)。
func Start(pythonExe, acpDir string) (*Client, error) {
	return startProcess(pythonExe, []string{"-m", "acp_adapter"}, acpDir)
}

// startProcess 是进程拉起内核(exe/args 已归一),独立出来供测试注入假子进程
// (os.Args[0] helper 模式,dshapp 同款)。
func startProcess(exe string, args []string, dir string) (*Client, error) {
	cmd := exec.Command(exe, args...)
	cmd.Dir = dir
	hidewin.Apply(cmd) // python 是控制台程序:不隐藏会闪黑窗
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
		return nil, fmt.Errorf("启动 Hermes ACP 适配器失败: %w", err)
	}
	c := &Client{
		cmd:     cmd,
		stdin:   stdin,
		stdout:  bufio.NewReaderSize(stdout, 8*1024*1024),
		pending: map[int64]chan rpcResp{},
		exited:  make(chan struct{}),
	}
	go c.readLoop()
	go c.drainStderr(stderr)
	return c, nil
}

// Alive 报告子进程是否仍在运行(收尸后为 false;零值 Client 未启动,直接 false
// —— 不再对 nil exited 落进 default 误报 true)。
func (c *Client) Alive() bool {
	if c.cmd == nil {
		return false
	}
	select {
	case <-c.exited:
		return false
	default:
		return true
	}
}

// reapWaitBudget 是 Close 收尸(Kill 后等 readLoop 的 reap)的等待上限:Kill 后
// cmd.Wait + pending 清场通常毫秒级;超时说明收尸路径异常卡死,记日志后继续 ——
// Close 常在调用方锁内,绝不能陪葬挂死。
const reapWaitBudget = 5 * time.Second

// Close 收尸(Kill + Wait 一次)。
func (c *Client) Close() {
	c.closeOnce.Do(func() {
		if c.cmd == nil {
			return // 零值 Client(未启动):无进程可收,直接返回而非挂在 nil exited 上
		}
		if c.cmd.Process != nil {
			_ = c.cmd.Process.Kill()
		}
		c.waitReap()
	})
}

func (c *Client) waitReap() {
	timer := time.NewTimer(reapWaitBudget)
	defer timer.Stop()
	select {
	case <-c.exited:
	case <-timer.C:
		log.Printf("[hermes-acp] Close 收尸等待超时(%s),放弃等待继续", reapWaitBudget)
	}
}

func (c *Client) reap() {
	c.waitOnce.Do(func() {
		if c.cmd != nil && c.cmd.Process != nil {
			_ = c.cmd.Wait()
		}
		c.mu.Lock()
		for _, ch := range c.pending {
			select {
			case ch <- rpcResp{Err: &rpcError{Code: -32000, Message: "hermes ACP connection closed"}}:
			default:
			}
		}
		c.pending = map[int64]chan rpcResp{}
		c.mu.Unlock()
		close(c.exited)
	})
}

func (c *Client) readLoop() {
	defer c.reap()
	for {
		line, err := c.stdout.ReadString('\n')
		if err != nil {
			return
		}
		var w wireMsg
		if jsonErr := json.Unmarshal([]byte(line), &w); jsonErr != nil {
			continue // 非协议帧(库噪音)静默丢
		}
		switch {
		case w.ID != nil && w.Method != "":
			// 服务器反向请求。审批桥只认 session/request_permission;已装钩子
			// 交给钩子应答,未装/不认识的方法一律回 method not found —— 绝不让
			// 适配器悬挂,也绝不伪造 decision(dsh/codex 同红线)。
			// 【P1】req 钩子等手机审批可达 120s,绝不能在 readLoop 里同步执行
			// (否则 session/update 流与全部应答投递冻结,agent/hermes.go 同款痛点):
			// 取 handler 快照后 go 派发(codexapp client.go 同款先例);钩子内的
			// 应答经 Respond/RespondError → writeFrame(带 5s 写预算)写回。
			// 【P1】id 解析失败的帧(字符串 id 等)不再静默丢:钩子面(int64 id)
			// 接不住,但按 JSON-RPC 规范回 -32601、id 原样回显 —— 静默丢会让
			// 适配器悬挂等应答。
			var id int64
			if jsonErr := json.Unmarshal(w.ID, &id); jsonErr != nil {
				c.writeFrame(map[string]any{
					"jsonrpc": "2.0", "id": w.ID,
					"error": map[string]any{"code": -32601, "message": "method not found: " + w.Method},
				})
				continue
			}
			h := c.loadHandlers()
			if w.Method == "session/request_permission" && h != nil && h.Req != nil {
				go h.Req(id, w.Method, w.Params) // 异步派发:readLoop 立即回到流上
				continue
			}
			c.writeFrame(map[string]any{
				"jsonrpc": "2.0", "id": w.ID,
				"error": map[string]any{"code": -32601, "message": "method not found: " + w.Method},
			})
		case w.ID != nil && w.Method == "":
			// 应答帧
			var id int64
			if jsonErr := json.Unmarshal(w.ID, &id); jsonErr != nil {
				continue
			}
			c.mu.Lock()
			ch, ok := c.pending[id]
			if ok {
				delete(c.pending, id)
			}
			c.mu.Unlock()
			if !ok {
				continue // 迟到应答静默丢(与 codex CallProbeCtx 同口径):CallCtx
				// 放弃路径已把 id 从 pending 除名,除名后迟到的帧落进本分支
				// (hermesapp 的未知应答处置口;ch 带缓冲且无人等待,不悬挂)。
			}
			ch <- rpcResp{ID: id, Result: w.Result, Err: w.Error}
		case w.Method == "session/update":
			if h := c.loadHandlers(); h != nil && h.Notif != nil {
				h.Notif(w.Params)
			}
		default:
			// 其他通知(非 session/update):daemon 侧无消费方,静默丢。
		}
	}
}

func (c *Client) drainStderr(r io.Reader) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		_ = sc.Text() // hermes 的 stderr 是它自己的日志面;daemon 侧不透传(避免刷屏),
		// 死因经进程退出+RPC 错误数据(适配器把 provider 错误放 data.details)可见。
	}
}

type wireMsg struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// writeBudget 是单帧 stdin 写的预算:适配器挂死不读管道时 Write 会无限期阻塞,
// 无预算的写会把 writeMu 持有方与整条调用链拖死(readLoop 的 -32601 应答、审批
// 应答、取消帧收尾都可能陪葬)。
const writeBudget = 5 * time.Second

func (c *Client) writeFrame(v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.writeWithBudget(append(raw, '\n'), writeBudget)
}

// writeWithBudget 在 budget 内完成一次 stdin 写,超时即返回错误。
// 【超时语义】返回错误只代表"预算内没写完",底层 Write 可能仍在后台 goroutine 里
// 继续执行(管道写没有取消手段);超时必须按"连接可能已坏"对待 —— 后续写大概率
// 继续超时,由调用方 Close 收尸,绝不能当成"这帧没发出去"而重发(后台写完成后
// 补发会造成重复帧)。
// 【并发安全】超时放弃后 writeMu 随 writeFrame 返回而释放,迟到的后台 Write 不再
// 持有 writeMu;os.File 底层写自带写锁,与本方后续写按 syscall 串行,不会交错出帧
// (最坏是迟到的帧排在后面,连接已坏场景次序无关紧要)。
// 【波次口径】本波包内自包含私有 helper,不建跨包公共件(波 4 统一收敛)。
func (c *Client) writeWithBudget(data []byte, budget time.Duration) error {
	type writeOutcome struct {
		err error
	}
	done := make(chan writeOutcome, 1) // 带缓冲:放弃路径不阻塞后台写 goroutine
	go func() {
		_, err := c.stdin.Write(data)
		done <- writeOutcome{err: err}
	}()
	timer := time.NewTimer(budget)
	defer timer.Stop()
	select {
	case out := <-done:
		return out.err
	case <-timer.C:
		return fmt.Errorf("hermes ACP stdin 写超时(%s;写可能仍在后台完成)", budget)
	}
}

// callTimeout 是 CallCtx(短控制调用)的兜底应答预算(与 codex/zcode 的
// callTimeout 同口径 120s):ctx 之外的最后防线。只罩"问一句答一句"的短控制
// RPC;session/prompt 这类活到整轮结束的长调用走 CallTurnCtx,不受它约束
// (codex/zcode 的 turn 靠通知流驱动,120s 只罩它们的短控制调用;hermes 的
// prompt 是阻塞到整轮结束的长调用,长流式轮次轻松超 2 分钟,被兜底误杀即
// 全部假死超时)。
const callTimeout = 120 * time.Second

// Call 发一个请求并等应答(无 ctx 便捷版:Background + callTimeout 兜底,短
// 控制调用口径;要随轮取消的调用方用 CallCtx/CallTurnCtx)。
func (c *Client) Call(method string, params any) (json.RawMessage, error) {
	return c.CallCtx(context.Background(), method, params)
}

// CallCtx 发一个**短控制**请求并等应答,三路 select(ch / ctx.Done / callTimeout
// 兜底):initialize、session/new·load·resume·fork·list、set_model 等"问一句答
// 一句"的调用全走这里,Background ctx 也有 120s 上界。session/prompt 禁走本
// 入口(长轮次会被兜底误杀),见 CallTurnCtx。
func (c *Client) CallCtx(ctx context.Context, method string, params any) (json.RawMessage, error) {
	return c.call(ctx, method, params, true)
}

// CallTurnCtx 发一个**活到整轮结束**的请求并等应答,两路 select(ch / ctx.Done),
// 无兜底 timer —— 专供 session/prompt:长流式轮次轻松超 2 分钟,Run 的 ctx 又无
// deadline,120s 兜底会把长轮次误杀成假死超时。其存活由调用方的 hermes 看门狗
// 三闸(150s 静默 + 60min 上限 + 审批豁免)与 ctx 取消治理;放弃路径与 CallCtx
// 同款 forget 除名,迟到应答落 readLoop 未知应答处置口。
func (c *Client) CallTurnCtx(ctx context.Context, method string, params any) (json.RawMessage, error) {
	return c.call(ctx, method, params, false)
}

// call 是 CallCtx / CallTurnCtx 的公共实现(codexapp call(ctx,…,probe bool) 同款
// 结构);budget=true 时多一路 callTimeout 兜底。注册 pending → 写帧 → 三路/两路
// select:放弃路径 forget 除名,本方法返回后不留任何等 <-ch 的后台 goroutine
// (acp.go 旧 goroutine 包装的僵尸坑已根治,见 CallCtx 系列注释)。
func (c *Client) call(ctx context.Context, method string, params any, budget bool) (json.RawMessage, error) {
	if ctx == nil {
		ctx = context.Background() // nil ctx 合法:短控制调用退化为 callTimeout 兜底,长调用无兜底
	}
	if c.cmd == nil {
		return nil, fmt.Errorf("hermes ACP %s: client 未启动(零值 Client)", method)
	}
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	ch := make(chan rpcResp, 1)
	c.pending[id] = ch
	c.mu.Unlock()

	if err := c.writeFrame(map[string]any{
		"jsonrpc": "2.0", "id": id, "method": method, "params": params,
	}); err != nil {
		c.forget(id)
		return nil, fmt.Errorf("hermes ACP write %s: %w", method, err)
	}
	var timer *time.Timer
	if budget {
		timer = time.NewTimer(callTimeout)
		defer timer.Stop()
	}
	var timeout <-chan time.Time // budget=false 时保持 nil:select 该臂永不就绪(等价无第三路)
	if timer != nil {
		timeout = timer.C
	}
	select {
	case resp := <-ch:
		if resp.Err != nil {
			return nil, resp.Err
		}
		return resp.Result, nil
	case <-ctx.Done():
		c.forget(id) // 除名:迟到应答落入 readLoop 未知应答处置口,不再悬挂等待者
		return nil, fmt.Errorf("hermes ACP %s: %w", method, ctx.Err())
	case <-timeout:
		c.forget(id) // 同上:callTimeout 兜底放弃路径与 ctx 放弃完全同待遇
		return nil, fmt.Errorf("hermes ACP %s: 等待应答超时(%s)", method, callTimeout)
	}
}

// forget 从 pending 表除名一个在途 id(mu 内删除,幂等;写失败与放弃路径共用)。
func (c *Client) forget(id int64) {
	c.mu.Lock()
	delete(c.pending, id)
	c.mu.Unlock()
}

// killSegvGuard Windows 下 Kill 已够( TerminateProcess);保留下列常量防误删说明。
