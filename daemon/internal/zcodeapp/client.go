// Package zcodeapp 封装 ZCode 桌面端 agent 内核(`zcode.cjs app-server --stdio`)
// 的子进程与"ZCode Protocol"(JSON-RPC 风格 over stdio,NDJSON 每行一条)。
//
// 【讲解】为什么桥接它而不是自己写 agent:和 codex app-server 同款架构 ——
// ZCode 桌面端(Electron)的 agent 循环、工具执行、审批、MCP、会话管理都是
// ZCode 团队维护的现成内核,我们只做"翻译官":手机 gRPC ↔ ZCode Protocol。
//
// 【与 codex app-server 的差异(全部来自 cmd/zcprobe 实证,证据见
// docs/verify/evidence/zcode-p0-probe.md,不要凭"JSON-RPC 2.0 常识"改回去)】
//  1. 帧里【没有】"jsonrpc":"2.0" 字段 —— 服务器用 zod strict 校验,带了直接报
//     Unrecognized key。三种帧:请求 {id,method,params} / 通知 {method,params} /
//     应答 {id,result} 或 {id,error}。
//  2. 服务器反向请求的 id 是【字符串】("server-1"、"server-2"…),应答必须原样回显。
//  3. 客户端必须应答 startup/storagePath 与 session/requestRuntimePreferences,
//     否则 session/create 会以 -32022 "Client request timed out" 失败 —— 这两个
//     由本包自动应答(见 autoAnswer),不劳 provider 操心。
//  4. 单行上限 65536 字节,读缓冲给 8MB 保险(流式事件批冲刷下不怕长行)。
package zcodeapp

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
	"github.com/pocketagent/daemon/internal/stderrdrain"
)

// Client 是一条 app-server 子进程连接。并发安全:请求用 pending map 关联应答,
// 写入由 writeMu 串行化,通知/反向请求分发给钩子(通知钩子在 readLoop 里同步
// 调用必须快进快出;反向请求【P2-5】已改 goroutine 分发 —— 审批等待最长 120s,
// 同步执行会把 readLoop 堵死,通知泵与请求应答全停)。
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
	// reapWithLog(与 piapp 同款,2026-09-19 补)。
	waitOnce sync.Once

	// NotifHandler 当轮通知钩子(session/event 等),由 Run 按轮装上/卸下。
	// Deprecated: 裸字段仅兼容独占客户端的旧探针。并发路径必须用 Setter；
	// 直接赋值不得与 CallCtx、SyncHandlers 或任何 Setter 并发。
	NotifHandler func(method string, params json.RawMessage)
	// GlobalHandler 常驻通知钩子:与轮次无关的后台事实(启动状态、资源采样等)
	// 在这里做廉价记录;先于 NotifHandler 调用。
	GlobalHandler func(method string, params json.RawMessage)
	// ReqHandler 服务器反向请求钩子(审批等),返回 true 表示已应答。
	// 只处理业务类请求 —— startup/storagePath 与 session/requestRuntimePreferences
	// 由本包 autoAnswer 先行处理,轮不到它。
	// 【调用约定】由独立 goroutine 调用(不堵 readLoop),应答仍按原样 id 回写;
	// 可能并发多条反向请求,实现侧须自行保证并发安全。
	ReqHandler func(rawID json.RawMessage, method string, params json.RawMessage) bool

	// Setter 写入和 CallCtx 快照读取均受 handlerMu 保护。dispatch 只读
	// 不可变快照且在锁外调用回调，回调可重入请求或卸载自身。
	hs       atomic.Pointer[zcHandlers]
	accounts *desktopAccounts
}

// zcHandlers 某一时刻的 handler 快照。
type zcHandlers struct {
	global func(method string, params json.RawMessage)
	notif  func(method string, params json.RawMessage)
	req    func(rawID json.RawMessage, method string, params json.RawMessage) bool
}

// syncHandlers 兼容旧探针的串行赋值后 CallCtx；生产写入统一用 Setter。
func (c *Client) syncHandlers() {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.publishHandlersLocked()
}

func (c *Client) publishHandlersLocked() {
	c.hs.Store(&zcHandlers{global: c.GlobalHandler, notif: c.NotifHandler, req: c.ReqHandler})
}

func (c *Client) SetGlobalHandler(handler func(string, json.RawMessage)) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.GlobalHandler = handler
	c.publishHandlersLocked()
}

func (c *Client) SetTurnHandlers(notif func(string, json.RawMessage), req func(json.RawMessage, string, json.RawMessage) bool) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.NotifHandler, c.ReqHandler = notif, req
	c.publishHandlersLocked()
}

// SyncHandlers 是 syncHandlers 的导出版:provider 在轮次收尾卸下 handler
// (defer 置 nil,此后不再有 CallCtx)时同步快照,readLoop 才能立刻看到卸载。
func (c *Client) SyncHandlers() { c.syncHandlers() }

// LocateZCode 找 node.exe + zcode.cjs(ZCode 桌面端 agent 内核)。
//
// 优先环境变量 ZCODE_NODE / ZCODE_CJS(便于多机部署/测试注入),缺省用
// Windows 下的标准安装位置(本机实证存在)。两样都齐才算可用。
func LocateZCode() (node, cjs string, err error) {
	node = os.Getenv("ZCODE_NODE")
	if node == "" {
		node = filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "nodejs", "node.exe")
	}
	cjs = os.Getenv("ZCODE_CJS")
	if cjs == "" {
		cjs = filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "ZCode", "resources", "glm", "zcode.cjs")
	}
	if _, err := os.Stat(node); err != nil {
		return "", "", fmt.Errorf("未找到 node(%s 可指定): %w", "ZCODE_NODE", err)
	}
	if _, err := os.Stat(cjs); err != nil {
		return "", "", fmt.Errorf("未找到 ZCode agent 内核 zcode.cjs(%s 可指定): %w", "ZCODE_CJS", err)
	}
	return node, cjs, nil
}

// StartOptions 拉起参数。Workspace 是会话的工作区(app-server 的 cwd)。
type StartOptions struct {
	Workspace string
}

// zcodeLaunchSpec runs the CLI on Node; launching Electron without its parent
// runtime environment starts the desktop application instead of the stdio server.
func zcodeLaunchSpec(node, cjs string, _ StartOptions) (string, []string) {
	return node, []string{cjs, "app-server", "--stdio", "--surface", "desktop"}
}

// ---- 子进程环境修补(2026-09-19 根因修复)----
//
// 【死因实录】生产 daemon 由 计划任务→watchdog_hidden.vbs(WMI)→paguard 链拉起,
// 进程环境里【没有】ZCode 桌面端给子进程注入的这对变量:
//
//	ZCODE_BUILTIN_PROVIDER_CONFIG_FILE(内置 provider 配置,路径含版本号与
//	  endpoint 哈希:`~/.zcode/v2/runtime/provider/windows-x86_64/<版本>/endpoint-*/…`)
//	ZCODE_PERSONAL_PROVIDER_CONFIG_FILE(~/.zcode/v2/provider_config.json)
//
// zcode.cjs 的启动序列(prepareCliProviderRuntimeEnv)规定:两个变量【都在】→
// 直接采用、跳过磁盘查找;否则退回查找 <cjs目录>/provider/zcode-builtin.json
// 与 <cjs目录>/../../../../../config/provider/zcode-builtin.json —— 当前安装
// 布局两处都不存在 → throw → 顶层 catch 向 stderr 打印"无法定位 CLI ZCode
// Built-in Provider Config:…"后 exit(1)。daemon 侧看到的就是"进程刚拉起就死":
// 写 session/create 报 `write |1: The pipe is being closed`,或"连接已关闭"。
//
// 【为什么探针一直正常】zcprobe 从交互终端跑,终端由 ZCode 桌面端拉起、完整
// 继承了这对变量 —— 典型"生产差异"。隔离实验(2026-09-19):
//   - 干净环境去掉两变量直跑 zcode.cjs app-server → 立即 exit(1),错误逐字一致;
//   - 同一干净环境只补回两变量 → 正常跑到 startup/storageState ready。
//
// 【补法】变量不全时从桌面端数据目录发现并注入(文件名实证存在):
//
//	personal: ~/.zcode/v2/provider_config.json
//	builtin : 当前安装包，或与安装版本/平台/endpoint 匹配的更高 revision 缓存。
//	          不按 mtime 跨版本、跨 endpoint 猜测。
//
// 【刻意不做】不另设 ZCODE_STORAGE_DIR:子进程必须与桌面端共用同一存储,
// 导入会话的 session/resume(HistoryFor)才找得到桌面端会话;共享存储与
// 运行中的桌面端实测无锁冲突(直跑实验 storageState 正常 ready)。
const (
	zcodeEnvBuiltin  = "ZCODE_BUILTIN_PROVIDER_CONFIG_FILE"
	zcodeEnvPersonal = "ZCODE_PERSONAL_PROVIDER_CONFIG_FILE"
)

// zcodeChildEnv 构造子进程环境:继承 daemon 当前环境,并保证 provider 配置
// 两个变量在场(缺失则从 ~/.zcode/v2 自动发现)。发现失败不注入、按原环境
// 启动 —— 错误会经 [zcode-stderr] 可见,绝不静默伪造。
func zcodeChildEnv(cjs string) []string {
	env := os.Environ()
	// ELECTRON_RUN_AS_NODE 是 Electron 家族的著名脚枪:一旦在场,任何 Electron
	// 二进制都会被降级成纯 node。这里直跑 node.exe 本不受影响,但 zcode 内部
	// 若再拉桌面二进制会行为漂移;剥掉只赚不亏(任务书候选死因之一,防御性排除)。
	env = dropEnv(env, "ELECTRON_RUN_AS_NODE")
	if envHasNonEmpty(env, zcodeEnvBuiltin) && envHasNonEmpty(env, zcodeEnvPersonal) {
		return env
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		log.Printf("[zcode] 环境修补失败:拿不到用户主目录(%v),按原环境启动(若仍崩可见 [zcode-stderr])", err)
		return env
	}
	v2 := filepath.Join(home, ".zcode", "v2")
	if !envHasNonEmpty(env, zcodeEnvPersonal) {
		if p := filepath.Join(v2, "provider_config.json"); fileExists(p) {
			env = append(env, zcodeEnvPersonal+"="+p)
		} else {
			log.Printf("[zcode] 环境修补:未找到个人 provider 配置 %s", filepath.Join(v2, "provider_config.json"))
		}
	}
	if !envHasNonEmpty(env, zcodeEnvBuiltin) {
		if p := builtinProviderConfigForCJS(cjs, v2); p != "" {
			env = append(env, zcodeEnvBuiltin+"="+p)
			log.Printf("[zcode] 环境修补:builtin provider 配置 = %s", p)
		} else {
			log.Printf("[zcode] 环境修补:未在 %s 下发现 builtin provider 配置(runtime/provider/*/*/endpoint-*/zcode-builtin.json)", v2)
		}
	}
	return env
}

// builtinProviderConfigForCJS 优先使用当前 zcode.cjs 同一安装包携带的配置。
// runtime 缓存只接受同平台/安装版本/endpoint 的更高 revision，
// 不再按全局 mtime 猜测别的环境。
func builtinProviderConfigForCJS(cjs, v2 string) string {
	return scopedBuiltinProviderConfig(cjs, v2)
}

// newestBuiltinProviderConfig 在 <v2>/runtime/provider 下找 mtime 最新的
// zcode-builtin.json(平台/版本/endpoint 目录名不写死,跟随桌面端演进)。
// 找不到返回空串。
func newestBuiltinProviderConfig(v2 string) string {
	root := filepath.Join(v2, "runtime", "provider")
	var best string
	var bestAt time.Time
	platforms, err := os.ReadDir(root)
	if err != nil {
		return ""
	}
	for _, plat := range platforms {
		if !plat.IsDir() {
			continue
		}
		versions, err := os.ReadDir(filepath.Join(root, plat.Name()))
		if err != nil {
			continue
		}
		for _, ver := range versions {
			if !ver.IsDir() {
				continue
			}
			endpoints, err := os.ReadDir(filepath.Join(root, plat.Name(), ver.Name()))
			if err != nil {
				continue
			}
			for _, ep := range endpoints {
				if !ep.IsDir() {
					continue
				}
				p := filepath.Join(root, plat.Name(), ver.Name(), ep.Name(), "zcode-builtin.json")
				fi, err := os.Stat(p)
				if err != nil {
					continue
				}
				if best == "" || fi.ModTime().After(bestAt) {
					best, bestAt = p, fi.ModTime()
				}
			}
		}
	}
	return best
}

// envHasNonEmpty 环境切片里某变量是否存在且非空(Windows 下变量名不区分大小写,
// 但 os.Environ 由我们自己的进程产出,名字与常量一致,直接精确比对即可)。
func envHasNonEmpty(env []string, key string) bool {
	for _, kv := range env {
		if k, v, ok := strings.Cut(kv, "="); ok && k == key && strings.TrimSpace(v) != "" {
			return true
		}
	}
	return false
}

// dropEnv 从环境切片里剥掉某变量(忽略大小写:Windows 环境变量名不区分)。
func dropEnv(env []string, key string) []string {
	out := env[:0:0]
	for _, kv := range env {
		if k, _, ok := strings.Cut(kv, "="); ok && strings.EqualFold(k, key) {
			continue
		}
		out = append(out, kv)
	}
	return out
}

// fileExists 路径存在且是普通文件。
func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}

// Start 拉起 app-server 子进程并进入读循环。
func Start(opts StartOptions) (*Client, error) {
	node, cjs, err := LocateZCode()
	if err != nil {
		return nil, err
	}
	runtimeExe, args := zcodeLaunchSpec(node, cjs, opts)
	cmd := exec.Command(runtimeExe, args...)
	if opts.Workspace != "" {
		cmd.Dir = opts.Workspace
	}
	// 环境修补(见 zcodeChildEnv 注释):生产链路缺 provider 配置变量时
	// zcode.cjs 启动即退,这是 2026-09-19 "ZCode 会话完全发不出消息"的根因。
	cmd.Env = zcodeChildEnv(cjs)
	hidewin.Apply(cmd) // node 是控制台程序:不隐藏会在用户桌面闪黑窗
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	// stderr 单独收进 daemon 日志(绝不混进 stdout —— 那里只允许 NDJSON 协议帧)。
	// 【2026-09-19 改】不再透传 os.Stderr:生产 daemon 由 paguard/WMI 拉起
	// (CREATE_NO_WINDOW,无控制台),os.Stderr 无人接收,zcode 的死因输出全丢。
	// 改用 StderrPipe + readStderr goroutine,一律走 log 包([zcode-stderr] 前缀,
	// 与 piapp 同款范式),生产环境永远落 daemon.log。
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		// 【死因日志】启动失败的原因必须落 daemon.log(生产环境 os.Stderr 无人接收)。
		log.Printf("[zcode] 启动 ZCode app-server 失败: runtime=%s cjs=%s err=%v", runtimeExe, cjs, err)
		return nil, fmt.Errorf("启动 ZCode app-server 失败: %w", err)
	}
	log.Printf("[zcode] ZCode 子进程已拉起 PID=%d cwd=%q runtime=%s", cmd.Process.Pid, cmd.Dir, runtimeExe)
	c := &Client{
		cmd:      cmd,
		stdin:    stdin,
		stdout:   bufio.NewReaderSize(stdout, 8*1024*1024), // 单行上限 65536B,给 8MB 保险
		pending:  map[int64]chan rpcResp{},
		waitOnce: sync.Once{},
		accounts: newDesktopAccounts(cmd.Env),
	}
	c.syncHandlers() // readLoop 的读面就位(此刻 handler 均为 nil:上层 Start 之后才装)
	go c.readLoop()
	go c.readStderr(stderr)
	return c, nil
}

// readStderr 收 ZCode 的诊断输出(stderr 只放日志,不混协议)。
// 死因定位靠它:zcode.cjs 启动崩溃的错误原文(如 provider 配置缺失)只出现在
// 这里,退出码(reapWithLog)只有症状,stderr 才是死因。
func (c *Client) readStderr(r io.Reader) {
	// 【收口到 stderrdrain】裸扫 + 直传换成统一收口:补上**凭据脱敏**(zcode 的
	// stderr 可能带账户 key / provider 凭据)与**总量限流**(刷屏不撑爆 daemon.log),
	// 并与 codex / pi / dsh 口径一致。前缀与"stderr 只落日志、不混协议"不变。
	stderrdrain.Drain(r, stderrdrain.Options{}, func(l string) {
		log.Printf("[zcode-stderr] %s", l)
	})
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
		log.Printf("[zcode] ZCode 子进程退出(%s) PID=%d 退出码=%d err=%v", why, c.PID(), code, err)
	})
}

// wireMsg 一行协议帧。ID 用 RawMessage:我们发出的请求 id 是数字,而服务器反向
// 请求的 id 是字符串("server-1"),统一存原文、按需解析。
type wireMsg struct {
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

// rpcResp 一次应答:要么是结果,要么是错误。用结构体而不是裸 RawMessage,
// 就是为了让"错误"无法被当成"空结果"混过去(与 codexapp 同款教训)。
type rpcResp struct {
	raw json.RawMessage
	err error
}

var errClientClosed = errors.New("ZCode app-server 连接已关闭")

// callTimeout 单条请求的等待上限。取值宽松(会话建立要等存储初始化),
// 但只要有界,就不会出现"一个请求挂住、所有 ZCode 操作排队等它"。
const callTimeout = 120 * time.Second

// CallCtx 发请求并等待应答,受 ctx 与 callTimeout 双重上界保护。
//
// 【两条纪律,与 codexapp.CallCtx 相同且都被线上问题逼出来过】
//  1. JSON-RPC 错误在传输层就转成 Go error,调用点的 err 判断永不落空;
//  2. 必须有超时:子进程不回时宁可口齿清楚地报超时,也不留一个永不返回的调用。
func (c *Client) CallCtx(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if c.accounts != nil && (method == "session/create" || method == "session/resume" || method == "session/setModel" || method == "session/send" || method == "workspace/generateText") {
		if err := c.accounts.sync(ctx, c); err != nil {
			return nil, &AccountSyncError{Err: err}
		}
	}
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	ch := make(chan rpcResp, 1)
	c.pending[id] = ch
	c.mu.Unlock()

	body := map[string]interface{}{"id": id, "method": method}
	if params != nil {
		body["params"] = params
	}
	// 同步点:发请求前重新发布 handler 快照,保证「装卸 handler → CallCtx →
	// 反向请求到达」时序下 readLoop 一定能看到最新 handler(与 piapp.CallCtx 同款)。
	c.syncHandlers()
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
		return nil, fmt.Errorf("%s: 等待 ZCode 应答超时(%s)", method, callTimeout)
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

func (c *Client) writeLine(raw []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_, err := c.stdin.Write(append(raw, '\n'))
	return err
}

// Respond 应答服务器反向请求(rawID 原样回显 —— 服务器的 id 是字符串)。
func (c *Client) Respond(rawID json.RawMessage, result interface{}) error {
	body, err := json.Marshal(map[string]interface{}{"id": rawID, "result": result})
	if err != nil {
		return err
	}
	return c.writeLine(body)
}

// RespondError 用 error 帧应答反向请求。
//
// 【为什么必须区分 Respond 和 RespondError】与 codexapp 同款:审批的"拒绝"是
// 成功的应答(decision:"deny");而我们能力不足的请求(elicitation、官方 MCP
// 的 auth headers)不能伪造 decision,要回 error 让对方按"客户端不支持"处理。
func (c *Client) RespondError(rawID json.RawMessage, code int, message string) error {
	body, err := json.Marshal(map[string]interface{}{
		"id":    rawID,
		"error": map[string]interface{}{"code": code, "message": message},
	})
	if err != nil {
		return err
	}
	return c.writeLine(body)
}

// readLoop 唯一的读 goroutine:解析 NDJSON 并分发。任何一行解析失败只记日志,
// 不中断整条管道(和 codexapp 一致:单行坏帧不该杀死整条会话)。
func (c *Client) readLoop() {
	for {
		line, err := c.stdout.ReadBytes('\n')
		if len(line) > 0 {
			var m wireMsg
			if json.Unmarshal(line, &m) == nil {
				c.dispatch(&m)
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
			// 【死因日志】EOF 只是症状,退出码+stderr 才是死因:收尸并把退出码
			// 落 daemon.log(与 Close 收尸共用 reapWithLog,Once 保证只 Wait 一次;
			// 与 piapp 同款,2026-09-19 补)。
			go c.reapWithLog("意外退出(stdout 关闭)")
			return
		}
	}
}

func (c *Client) dispatch(m *wireMsg) {
	switch {
	case m.Method != "" && len(m.ID) > 0:
		// 服务器反向请求(必须应答,否则服务器会一直等 → 调用超时)
		if c.autoAnswer(m) {
			return
		}
		// 【P2-5】业务反向请求走 goroutine 分发(对齐 piapp 的 dialog 模式):
		// 审批/反问要等手机决定(最长 120s),同步执行会把 readLoop 堵死 ——
		// 等待期间 session/event 通知与所有请求应答全停,看门狗还会误杀轮次。
		// handler 从快照取(readLoop 唯一读面,免与装卸方 data race);返回
		// false 由这里统一回 -32601 —— 应答仍按原样 id 回写,不伪造结果。
		if hs := c.hs.Load(); hs != nil && hs.req != nil {
			handler := hs.req
			id, method, params := m.ID, m.Method, m.Params
			go func() {
				if !handler(id, method, params) {
					_ = c.RespondError(id, -32601, "PocketAgent 未实现该反向请求: "+method)
				}
			}()
			return
		}
		_ = c.RespondError(m.ID, -32601, "PocketAgent 未实现该反向请求: "+m.Method)
	case m.Method != "":
		// 先给常驻钩子(后台事实),再给当轮钩子(转流给手机),顺序与 codexapp 相同:
		// 当轮钩子在轮次结束后会被卸下,收尾瞬间到达的事件必须还有人接。
		// 只读快照:与 provider 协程装卸裸字段解耦(见 hs 字段注释)。
		if hs := c.hs.Load(); hs != nil {
			if hs.global != nil {
				hs.global(m.Method, m.Params)
			}
			if hs.notif != nil {
				hs.notif(m.Method, m.Params)
			}
		}
	case len(m.ID) > 0:
		// 我们发出的请求的应答(id 是数字)
		var id int64
		if json.Unmarshal(m.ID, &id) != nil {
			return
		}
		c.mu.Lock()
		ch, ok := c.pending[id]
		delete(c.pending, id)
		c.mu.Unlock()
		if ok {
			if m.Error != nil {
				ch <- rpcResp{err: &RPCError{Code: m.Error.Code, Message: m.Error.Message, Data: m.Error.Data}}
			} else {
				ch <- rpcResp{raw: m.Result}
			}
		}
	}
}

// autoAnswer 客户端必须应答的协议级反向请求。
//
//   - startup/storagePath:应答 result={"method":"startup/storagePathReady","reuse":true}
//     (形状来自 zcode.cjs 内嵌 zod schema;实测本机虽未发过该请求,桌面端其它
//     配置下会发,留着防超时)
//   - session/requestRuntimePreferences:回"全默认"偏好。schema(IKe)里
//     nativeSearchEnhancementsEnabled 无默认值必填,其余有默认;不给会话建立
//     就会 -32022 超时(实证)。
func (c *Client) autoAnswer(m *wireMsg) bool {
	switch m.Method {
	case "interaction/requestProviderRuntimeHeaders":
		if c.accounts == nil {
			return false
		}
		// Disk reads/decryption must not stall the protocol read pump.
		go func() {
			var req struct {
				ProviderID    string `json:"providerId"`
				AccountAccess struct {
					Family string `json:"accountType"`
					Mode   string `json:"mode"`
				} `json:"accountAccess"`
			}
			if json.Unmarshal(m.Params, &req) != nil {
				_ = c.RespondError(m.ID, -32602, "ZCode account request 无效")
				return
			}
			key, err := c.accounts.requestKey(req.ProviderID, req.AccountAccess.Family, req.AccountAccess.Mode)
			if err != nil {
				_ = c.Respond(m.ID, map[string]any{"headersApplied": false, "errorMessage": err.Error()})
				return
			}
			_ = c.Respond(m.ID, map[string]any{"headersApplied": true, "requestAuth": map[string]string{"apiKey": key}})
		}()
		return true
	case "startup/storagePath":
		_ = c.Respond(m.ID, map[string]interface{}{"method": "startup/storagePathReady", "reuse": true})
		return true
	case "session/requestRuntimePreferences":
		_ = c.Respond(m.ID, map[string]interface{}{
			"nativeSearchEnhancementsEnabled":      false,
			"memoryEnabled":                        false,
			"askUserQuestionAutoResolutionEnabled": false,
			"modelContextBudgetStrategy":           "preflight-v1",
		})
		return true
	}
	return false
}

// Close 结束子进程并收割。
//
// 【为什么 Kill 后必须 Wait】exec.Cmd 明确要求调用方 Wait 收割子进程:只 Kill
// 不 Wait 会泄漏进程句柄与管道 —— 长跑 daemon 每次重挂载都 Close 一次
// (agent/zcode.go 的"杀掉重来"路径),泄漏会持续累积。收尸走 reapWithLog
// (退出码落 daemon.log),放专门 goroutine,不拖住 Close 调用方
// (与 piapp 同款,范式来自 term/manager.go:Kill 后 goroutine Wait)。
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
