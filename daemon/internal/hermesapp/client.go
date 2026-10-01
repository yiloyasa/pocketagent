package hermesapp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"

	"github.com/pocketagent/daemon/internal/hidewin"
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
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
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

// Alive 报告子进程是否仍在运行(收尸后为 false)。
func (c *Client) Alive() bool {
	select {
	case <-c.exited:
		return false
	default:
		return true
	}
}

// Close 收尸(Kill + Wait 一次)。
func (c *Client) Close() {
	c.closeOnce.Do(func() {
		if c.cmd != nil && c.cmd.Process != nil {
			_ = c.cmd.Process.Kill()
		}
		c.waitReap()
	})
}

func (c *Client) waitReap() {
	<-c.exited
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
			var id int64
			if jsonErr := json.Unmarshal(w.ID, &id); jsonErr != nil {
				continue
			}
			h := c.loadHandlers()
			if w.Method == "session/request_permission" && h != nil && h.Req != nil {
				h.Req(id, w.Method, w.Params)
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
				continue // 迟到应答静默丢(与 codex CallProbeCtx 同口径)
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

func (c *Client) writeFrame(v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_, err = c.stdin.Write(append(raw, '\n'))
	return err
}

// Call 发一个请求并等应答(不带超时;调用方用 ctx 控制)。
func (c *Client) Call(method string, params any) (json.RawMessage, error) {
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	ch := make(chan rpcResp, 1)
	c.pending[id] = ch
	c.mu.Unlock()

	if err := c.writeFrame(map[string]any{
		"jsonrpc": "2.0", "id": id, "method": method, "params": params,
	}); err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, fmt.Errorf("hermes ACP write %s: %w", method, err)
	}
	resp := <-ch
	if resp.Err != nil {
		return nil, resp.Err
	}
	return resp.Result, nil
}

// killSegvGuard Windows 下 Kill 已够( TerminateProcess);保留下列常量防误删说明。
