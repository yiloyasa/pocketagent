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
	"time"
)

type Client struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader

	mu      sync.Mutex
	nextID  int64
	pending map[int64]chan rpcResp

	// 通知(服务器 → 客户端,无需应答):item 更新、delta、turn 生命周期
	//
	// 【两个钩子的分工 —— 别合并】PocketAgent 是"手机遥控",一轮任务的事件需要
	// 实时转给手机,所以 NotifHandler 由 Run() 按轮次装上/卸下。但有一类通知
	// 跟"有没有在跑任务"无关:审查结论、会话归档/删除、队列变化、技能增删。
	// 它们以前全部落空 —— 因为不在轮次里就没人接(表现为手机上永远看不到
	// "审查者驳回了",以及电脑上删掉的会话在手机上还活着)。
	// 因此再加一个**常驻**的 GlobalHandler:子进程活着就一直挂着,
	// 负责把这类"后台事实"记下来。
	NotifHandler func(method string, params json.RawMessage)
	// GlobalHandler 常驻通知钩子(先于 NotifHandler 调用,只做记录,不做流式渲染)
	GlobalHandler func(method string, params json.RawMessage)
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
		pending: map[int64]chan rpcResp{},
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
func (c *Client) Call(method string, params interface{}) (json.RawMessage, error) {
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	ch := make(chan rpcResp, 1)
	c.pending[id] = ch
	c.mu.Unlock()

	body := map[string]interface{}{"jsonrpc": "2.0", "id": id, "method": method}
	if params != nil {
		body["params"] = params
	}
	raw, err := json.Marshal(body)
	if err != nil {
		c.forget(id)
		return nil, err
	}
	if _, err := c.stdin.Write(append(raw, '\n')); err != nil {
		c.forget(id)
		return nil, fmt.Errorf("写入 %s 失败: %w", method, err)
	}

	select {
	case resp := <-ch:
		if resp.err != nil {
			return nil, fmt.Errorf("%s: %w", method, resp.err)
		}
		return resp.raw, nil
	case <-time.After(callTimeout):
		c.forget(id)
		return nil, fmt.Errorf("%s: 等待 codex 应答超时(%s)", method, callTimeout)
	}
}

func (c *Client) forget(id int64) {
	c.mu.Lock()
	delete(c.pending, id)
	c.mu.Unlock()
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
	_, err = c.stdin.Write(append(raw, '\n'))
	return err
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
		// 先给常驻钩子(记录后台事实),再给当轮钩子(转流给手机)。
		// 顺序不能反:当轮钩子在 turn/completed 后会 finish(),之后的事件照样要落到
		// 常驻钩子里,否则"任务结束瞬间到达的归档/队列通知"又会丢。
		if c.GlobalHandler != nil {
			c.GlobalHandler(m.Method, m.Params)
		}
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

// Close 结束子进程。
func (c *Client) Close() {
	if c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
	_ = c.stdin.Close()
}
