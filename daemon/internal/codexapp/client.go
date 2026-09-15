// Package codexapp 封装 `codex app-server` 子进程:JSON-RPC 2.0 over stdio,
// 每行一条消息。这是 OpenAI Codex 官方给 GUI 客户端(VS Code 扩展等)的协议。
//
// 【讲解】为什么桥接它而不是自己写 agent:codex app-server 自带 agent 循环、
// 沙箱、审批、MCP、ChatGPT 登录、会话管理——这些是 OpenAI 团队维护的,
// 我们只做"翻译官":手机 gRPC ↔ codex JSON-RPC。原作 Mobile Codex 就是这么干的。
package codexapp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/pocketagent/daemon/internal/hidewin"
	"io"
	"os"
	"os/exec"
	"sync"
)

type Client struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader

	mu      sync.Mutex
	nextID  int64
	pending map[int64]chan json.RawMessage

	// 通知(服务器 → 客户端,无需应答):item 更新、delta、turn 生命周期
	NotifHandler func(method string, params json.RawMessage)
	// 反向请求(服务器 → 客户端,必须应答):审批等
	ReqHandler func(id int64, method string, params json.RawMessage)
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
	cmd.Stderr = os.Stderr // 诊断信息透传到 daemon 日志
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动 codex app-server 失败: %w", err)
	}
	c := &Client{
		cmd:     cmd,
		stdin:   stdin,
		stdout:  bufio.NewReaderSize(stdout, 4*1024*1024),
		pending: map[int64]chan json.RawMessage{},
	}
	go c.readLoop()
	return c, nil
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
func (c *Client) Call(method string, params interface{}) (json.RawMessage, error) {
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	ch := make(chan json.RawMessage, 1)
	c.pending[id] = ch
	c.mu.Unlock()

	body := map[string]interface{}{"jsonrpc": "2.0", "id": id, "method": method}
	if params != nil {
		body["params"] = params
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	if _, err := c.stdin.Write(append(raw, '\n')); err != nil {
		return nil, fmt.Errorf("写入 %s 失败: %w", method, err)
	}

	v := <-ch // 应答由 readLoop 投递;P0 不设超时(由上层 ctx 控制整体)
	return v, nil
}

// Respond 应答服务器的反向请求(审批)。
func (c *Client) Respond(id int64, result interface{}) error {
	raw, err := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0", "id": id, "result": result,
	})
	if err != nil {
		return err
	}
	_, err = c.stdin.Write(append(raw, '\n'))
	return err
}

func (c *Client) readLoop() {
	debug := os.Getenv("PA_CODEX_DEBUG") != ""
	for {
		line, err := c.stdout.ReadBytes('\n')
		if len(line) > 0 {
			if debug {
				// 原始帧日志:诊断"事件明明发了但收不到"类问题
				os.Stderr.Write([]byte("[raw] " + fmt.Sprintf("%.400s", line) + "\n"))
			}
			var m wireMsg
			if json.Unmarshal(line, &m) == nil {
				c.dispatch(&m)
			} else if debug {
				os.Stderr.Write([]byte("[raw-UNPARSED] len=" + fmt.Sprint(len(line)) + "\n"))
			}
		}
		if err != nil {
			// 子进程退出:叫醒所有等待者
			c.mu.Lock()
			for id, ch := range c.pending {
				close(ch)
				delete(c.pending, id)
			}
			c.mu.Unlock()
			return
		}
	}
}

func (c *Client) dispatch(m *wireMsg) {
	if m.Method != "" && m.ID != nil {
		// 服务器反向请求(审批等)
		if c.ReqHandler != nil {
			go c.ReqHandler(*m.ID, m.Method, m.Params)
		}
		return
	}
	if m.Method != "" {
		if c.NotifHandler != nil {
			c.NotifHandler(m.Method, m.Params)
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
				errRaw, _ := json.Marshal(map[string]string{"error": m.Error.Message})
				ch <- errRaw
			} else {
				ch <- m.Result
			}
		}
	}
}

// Close 结束子进程。
func (c *Client) Close() {
	if c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
	_ = c.stdin.Close()
}
