// client_test.go —— piapp 纯函数回归:扩展 UI 请求解析 + 守护扩展渲染。
package piapp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

// dialog 类要等应答,fire-and-forget 绝不能回写(实测 pi-lens 的 setStatus
// 一次会话发 26 条,误回会污染协议流)。
func TestParseUIRequestDialogKinds(t *testing.T) {
	dialog := `{"type":"extension_ui_request","id":"uuid-1","method":"confirm","title":"Dangerous!","message":"Allow rm -rf?"}`
	req := parseUIRequest(json.RawMessage(dialog))
	if req == nil || !req.Dialog() {
		t.Fatalf("confirm 应为 dialog: %+v", req)
	}
	if req.ID != "uuid-1" || req.Title != "Dangerous!" || req.Message != "Allow rm -rf?" {
		t.Fatalf("字段解析错误: %+v", req)
	}
	sel := `{"type":"extension_ui_request","id":"uuid-2","method":"select","options":["Allow","Block"],"timeout":10000}`
	req2 := parseUIRequest(json.RawMessage(sel))
	if !req2.Dialog() || len(req2.Options) != 2 || req2.TimeoutMs != 10000 {
		t.Fatalf("select 解析错误: %+v", req2)
	}
	for _, m := range []string{"notify", "setStatus", "setWidget", "setTitle", "set_editor_text"} {
		raw, _ := json.Marshal(map[string]any{"type": "extension_ui_request", "id": "x", "method": m})
		if r := parseUIRequest(raw); r == nil || r.Dialog() {
			t.Fatalf("%s 不应判定为 dialog: %+v", m, r)
		}
	}
	// 缺 id:拒绝(无法回写)
	if r := parseUIRequest(json.RawMessage(`{"type":"extension_ui_request","method":"confirm"}`)); r != nil {
		t.Fatalf("缺 id 应返回 nil: %+v", r)
	}
}

// 守护扩展:规则表必须完整进 TS,命中行为由扩展 confirm(无 timeout,daemon 兜底)。
func TestRenderGuardExtension(t *testing.T) {
	src := RenderGuardExtension(GuardRules)
	for _, rule := range GuardRules {
		// 正则源码以 %q 形式嵌入(字符串字面量),还原后等价于规则本身
		quoted := "\"" + strings.ReplaceAll(strings.ReplaceAll(rule.Pattern, "\\", "\\\\"), "\"", "\\\"") + "\""
		if !strings.Contains(src, quoted) && !strings.Contains(src, rule.Pattern) {
			t.Fatalf("规则缺失: %s", rule.Pattern)
		}
		if !strings.Contains(src, rule.Desc) {
			t.Fatalf("规则描述缺失: %s", rule.Desc)
		}
	}
	if !strings.Contains(src, "pi.on(\"tool_call\"") || !strings.Contains(src, "ctx.ui.confirm") {
		t.Fatal("扩展必须挂 tool_call 钩子并弹 confirm(审批转手机)")
	}
	if !strings.Contains(src, "block: true") {
		t.Fatal("拒绝必须 block(否则强制审批形同虚设)")
	}
	// 回归钉死(2026-09-19 EOF 崩溃):desc 曾用 Go 的 %#v 渲染出 `[]string{...}`,
	// TS ParseError → pi 加载扩展失败即退出(真机表现 get_state: EOF)。
	if strings.Contains(src, "[]string{") {
		t.Fatal("渲染产物含 Go 专有语法 []string{,TS 解析必炸(pi 加载扩展即退出)")
	}
	// 回归钉死(同一次崩溃的第二层):(?i) 是 Go regexp 专有内联标志,JS RegExp
	// 抛 Invalid group → 扩展加载失败 → pi 退出。大小写不敏感只能用 "i" 标志。
	if strings.Contains(src, "(?i)") {
		t.Fatal("渲染产物含 (?i) 内联标志,JS RegExp Invalid group(pi 加载扩展即退出)")
	}
	if !strings.Contains(src, `new RegExp(s, "i")`) {
		t.Fatal("正则必须带 i 标志还原大小写不敏感(替代被禁用的 (?i))")
	}
}

// 定位器：环境变量优先；未设时按用户环境派生，绝不回落到某台开发机的绝对路径
// （daemon 由 WMI 派生时无用户 PATH，所以派生链必须自己探目录，不能只靠 PATH）。
//
// 【语义变更 2026-09-22】原断言「本机固定路径应可定位」依赖写在源码里的开发机
// 绝对路径 —— 那条路径经 -trimpath 之后仍留在二进制字符串表里，随发布包分发，
// 属发布边界缺陷（规格 §2：发布包不得包含开发者电脑路径）。改为断言「要么定位
// 成功且路径真实存在，要么明确失败且报错不泄漏开发机路径」：这是消除硬编码
// 之后仍然成立、且能继续防回归的表述。
func TestLocatePiFallbacks(t *testing.T) {
	t.Setenv("PI_NODE", "")
	t.Setenv("PI_BIN", "")
	t.Setenv("PI_CLI", "")
	node, cli, err := LocatePi()
	if err != nil {
		if strings.Contains(err.Error(), "Claude Code") || strings.Contains(err.Error(), "yanzi") {
			t.Fatalf("定位失败的报错泄漏开发机路径: %v", err)
		}
	} else {
		if node == "" || cli == "" {
			t.Fatalf("定位成功却返回空路径: %q %q", node, cli)
		}
		if _, statErr := os.Stat(node); statErr != nil {
			t.Fatalf("返回的 node 本机不存在: %q", node)
		}
		if _, statErr := os.Stat(cli); statErr != nil {
			t.Fatalf("返回的 cli 本机不存在: %q", cli)
		}
	}
	// PI_NODE 兜底（CODEX_BIN 式）：显式指定必须优先生效
	self := os.Args[0] // 测试二进制自身必然存在
	t.Setenv("PI_NODE", self)
	if got := LocateNode(); got != self {
		t.Fatalf("PI_NODE 应优先生效: %q", got)
	}
}

// ---- 2026-09-19 质量巡逻 P1 回归 ----

// P1-2:Close 必须 Kill 后 cmd.Wait() 收尸,否则进程句柄/管道泄漏(长跑 daemon
// 每次重挂载都 Close 一次)。范式同 term/manager.go:Kill 后异步 Wait。
func TestCloseReapsProcess(t *testing.T) {
	cmd := exec.Command("cmd", "/c", "ping", "-n", "30", "127.0.0.1", ">nul")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("StdinPipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Skipf("无法拉起测试子进程: %v", err)
	}
	c := &Client{cmd: cmd, stdin: stdin, deathCh: make(chan struct{}), reaped: make(chan struct{})}
	c.Close()
	select {
	case <-c.reaped: // Wait 已返回;此后读 ProcessState 与 Wait 之间有 HB,-race 干净
		if cmd.ProcessState == nil {
			t.Fatal("reaped 已关闭但 ProcessState 为 nil")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Close 后 5s 内 cmd.Wait 未收尸(句柄泄漏)")
	}
	// 重复 Close 不许 panic(reapOnce 兜底)
	c.Close()
}

// P1-1:readLoop 的读面是原子快照 —— 装上则收到事件,卸下则不再收到。
func TestHandlerSnapshotFunnel(t *testing.T) {
	c := &Client{pending: map[string]chan piResp{}, deathCh: make(chan struct{})}

	var got int
	c.EventHandler = func(ev Event) { got++ }
	c.syncHandlers()
	c.emit(Event{Type: "message_update"})
	if got != 1 {
		t.Fatalf("装上 handler 后应收到 1 条事件,实得 %d", got)
	}
	c.EventHandler = nil
	c.syncHandlers()
	c.emit(Event{Type: "message_update"})
	if got != 1 {
		t.Fatalf("卸下 handler 后不应再收到事件,实得 %d", got)
	}
}

// P1-1:模拟 pi.go 的 Run 时序 —— 一个 goroutine 连续派发事件(读快照),
// 本 goroutine 反复装卸裸字段并经同步点发布,跑 -race 验证无 data race。
func TestHandlerSnapshotRaceFree(t *testing.T) {
	c := &Client{pending: map[string]chan piResp{}, deathCh: make(chan struct{})}
	c.syncHandlers()

	stop := make(chan struct{})
	done := make(chan struct{})
	go func() { // 充当 readLoop:持续 emit(读快照)
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
			}
			c.emit(Event{Type: "message_update"})
		}
	}()

	for i := 0; i < 2000; i++ { // 充当 provider Run:装卸裸字段 + CallCtx 同步点
		c.EventHandler = func(ev Event) {}
		c.syncHandlers()
		c.EventHandler = nil
		c.syncHandlers()
	}
	close(stop)
	<-done
}

// 无 handler 接 dialog:立即 cancelled(失败安全,§4.6);回写帧经真实管道落盘。
func TestDispatchDialogCancelledFallback(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer r.Close()
	c := &Client{stdin: w, pending: map[string]chan piResp{}, deathCh: make(chan struct{})}
	c.dispatch([]byte(`{"type":"extension_ui_request","id":"u1","method":"confirm","message":"x"}`))
	w.Close()

	_ = r.SetReadDeadline(time.Now().Add(2 * time.Second))
	line, err := bufio.NewReader(r).ReadString('\n')
	if err != nil {
		t.Fatalf("未收到 cancelled 回写: %v", err)
	}
	if !strings.Contains(line, `"cancelled":true`) || !strings.Contains(line, `"id":"u1"`) {
		t.Fatalf("回写帧不符: %s", line)
	}
}

// 有 handler 接 dialog:异步执行并回写 confirm 帧(绝不能堵 readLoop)。
func TestDispatchDialogHandlerResponds(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer r.Close()
	c := &Client{stdin: w, pending: map[string]chan piResp{}, deathCh: make(chan struct{})}
	called := make(chan struct{})
	c.UIRequestHandler = func(req *UIRequest) (map[string]any, bool) {
		close(called)
		return map[string]any{"confirmed": true}, true
	}
	c.syncHandlers()
	c.dispatch([]byte(`{"type":"extension_ui_request","id":"u2","method":"confirm","message":"y"}`))

	select {
	case <-called:
	case <-time.After(2 * time.Second):
		t.Fatal("handler 未被调用")
	}
	_ = r.SetReadDeadline(time.Now().Add(2 * time.Second))
	line, err := bufio.NewReader(r).ReadString('\n')
	if err != nil {
		t.Fatalf("未收到 confirm 回写: %v", err)
	}
	if !strings.Contains(line, `"confirmed":true`) || !strings.Contains(line, `"id":"u2"`) {
		t.Fatalf("回写帧不符: %s", line)
	}
}

// errWriter 模拟已断的 stdin:writeLine 必败,但失败发生在同步点之后。
type errWriter struct{}

func (errWriter) Write(p []byte) (int, error) { return 0, fmt.Errorf("管道已断") }
func (errWriter) Close() error                { return nil }

// 同步点不变式:CallCtx 在发命令前重新发布快照 —— 模拟 pi.go「Run 装完
// handler(不手动 sync)直接 CallCtx」的时序,Call 返回后 readLoop 必须能
// 看到新 handler(哪怕命令本身发送失败)。
func TestCallCtxRefreshesHandlerSnapshot(t *testing.T) {
	c := &Client{stdin: errWriter{}, pending: map[string]chan piResp{}, deathCh: make(chan struct{})}

	eventCh := make(chan Event, 1)
	c.EventHandler = func(ev Event) { eventCh <- ev }
	// 不手动 syncHandlers:Call 内部走「同步点 → writeLine(败)」
	if _, err := c.Call("probe", nil, 2*time.Second); err == nil {
		t.Fatal("errWriter 应使命令发送失败")
	}
	// 同步点已把 handler 发布成快照:此刻 emit 必须送达
	c.emit(Event{Type: "agent_start"})
	select {
	case ev := <-eventCh:
		if ev.Type != "agent_start" {
			t.Fatalf("事件类型不符: %+v", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("CallCtx 同步点未发布快照:emit 未送达新 handler")
	}
}

// ---- 2026-09-19 质量巡逻 P2 回归 ----

// P2-4 行为:会话缓存合并语义 —— 非空字段覆盖、空字段保留原值
// (GetState 的条件写与 SwitchSession 的覆盖写共用 updateSession)。
func TestSessionSnapshotMergeSemantics(t *testing.T) {
	c := &Client{pending: map[string]chan piResp{}, deathCh: make(chan struct{})}
	if c.SessionFile() != "" || c.SessionID() != "" {
		t.Fatal("未挂载过缓存应为空串")
	}
	c.updateSession(`C:\s\a.jsonl`, "sid-a")
	if c.SessionFile() != `C:\s\a.jsonl` || c.SessionID() != "sid-a" {
		t.Fatalf("首次写入应全部生效: %q %q", c.SessionFile(), c.SessionID())
	}
	// GetState 条件写(某字段缺失不抹掉另一字段)
	c.updateSession("", "sid-b")
	if c.SessionFile() != `C:\s\a.jsonl` || c.SessionID() != "sid-b" {
		t.Fatalf("空字段不应覆盖旧值: %q %q", c.SessionFile(), c.SessionID())
	}
	// SwitchSession 覆盖写(校验通过后文件更新,id 不动)
	c.updateSession(`C:\s\b.jsonl`, "")
	if c.SessionFile() != `C:\s\b.jsonl` || c.SessionID() != "sid-b" {
		t.Fatalf("覆盖写应只动非空字段: %q %q", c.SessionFile(), c.SessionID())
	}
}

// P2-4 并发:Rename/ExportHTML 的 ensureMounted→SwitchSession(写)与 Run 的
// mount/checkMountedSession(读)跑在不同 goroutine —— 多写多读混合,-race 下
// 必须干净(修复前裸字段读写必报 data race)。
func TestSessionSnapshotRaceFree(t *testing.T) {
	c := &Client{pending: map[string]chan piResp{}, deathCh: make(chan struct{})}
	var wg sync.WaitGroup
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(w int) { // 充当挂载路径:并发合并写
			defer wg.Done()
			for i := 0; i < 500; i++ {
				c.updateSession(fmt.Sprintf(`C:\s\%d-%d.jsonl`, w, i), fmt.Sprintf("sid-%d-%d", w, i))
			}
		}(w)
	}
	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func() { // 充当 Run/checkMountedSession:并发读
			defer wg.Done()
			for i := 0; i < 500; i++ {
				_ = c.SessionFile()
				_ = c.SessionID()
			}
		}()
	}
	wg.Wait()
}

// ---- 1.0.2 启动门控:就绪重试回归(2026-10-05 探针第 2/7/8 项修复)----

// newDelayedFakePi 拉起"扩展绑定中"的假 pi:PA_FAKE_PI_DELAY 秒后才开始读
// stdin(先发的命令在管道里排队)—— 与 pi 1.0.2 runRpcMode 的
// `await rebindSession()` 完成后才 attach stdin 读循环同构。
func newDelayedFakePi(t *testing.T, delaySecs int) *Client {
	t.Helper()
	return newFakePiChildEnv(t, delaySecs)
}

// newFakePiChildEnv 同上,但可给假 pi 子进程追加 env(如 PA_FAKE_PI_EXPECT_MODEL)。
func newFakePiChildEnv(t *testing.T, delaySecs int, extraEnv ...string) *Client {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestFakePiProcess$")
	cmd.Env = append(append(os.Environ(), "PA_FAKE_PI_CHILD=1", fmt.Sprintf("PA_FAKE_PI_DELAY=%d", delaySecs)), extraEnv...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("StdinPipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("拉起假 pi 失败: %v", err)
	}
	c := &Client{
		cmd:     cmd,
		stdin:   stdin,
		stdout:  bufio.NewReaderSize(stdout, 64*1024),
		pending: map[string]chan piResp{},
		deathCh: make(chan struct{}),
		reaped:  make(chan struct{}),
	}
	c.syncHandlers()
	go c.readLoop()
	t.Cleanup(c.Close)
	return c
}

// 就绪前发送的首条命令:单次预算(1s)内拿不到应答 → 换 id 重试 → 子进程 3s 后
// 开始消费管道,重试帧必须拿到应答(探针第 2 项:get_state 应答实际到达但晚于
// 旧版 15s 单次预算的直接回归)。旧 id 的迟到应答落 unknown 被静默丢弃,无害。
func TestCallReadyRetryUntilReady(t *testing.T) {
	c := newDelayedFakePi(t, 3)
	start := time.Now()
	raw, err := c.callWithReadyRetry(context.Background(), "get_state", nil, 1*time.Second)
	if err != nil {
		t.Fatalf("就绪后重试应成功: %v", err)
	}
	if el := time.Since(start); el < 3*time.Second {
		t.Fatalf("应等到子进程就绪(>=3s),实耗 %s", el)
	}
	var data struct {
		SessionID string `json:"sessionId"`
	}
	if json.Unmarshal(raw, &data) != nil || data.SessionID != "fake-session-id" {
		t.Fatalf("get_state 应答形状不符: %s", raw)
	}
}

// 定局失败绝不能重试:success:false(pi 已作答)单次即返,不标可重试哨兵。
func TestCallReadyRetryStopsOnDefinitiveError(t *testing.T) {
	c := newDelayedFakePi(t, 0)
	start := time.Now()
	_, err := c.callWithReadyRetry(context.Background(), "get_entries", map[string]any{"since": "bad-cursor"}, 1*time.Second)
	if err == nil {
		t.Fatal("success:false 必须报错")
	}
	if errors.Is(err, errNoAnswer) || errors.Is(err, errPiGone) {
		t.Fatalf("定局失败不得标成可重试哨兵: %v", err)
	}
	if el := time.Since(start); el > 2*time.Second {
		t.Fatalf("success:false 应单次即返(实耗 %s,疑似重试)", el)
	}
}

// 子进程死亡是定局:立即返回 errPiGone,不重试(不把失败探测拖满总预算)。
func TestCallReadyRetryStopsOnDeadProcess(t *testing.T) {
	c := &Client{pending: map[string]chan piResp{}, deathCh: make(chan struct{})}
	c.mu.Lock()
	c.dead = true
	c.deathErr = errors.New("simulated exit")
	c.mu.Unlock()
	_, err := c.callWithReadyRetry(context.Background(), "get_state", nil, 1*time.Second)
	if !errors.Is(err, errPiGone) {
		t.Fatalf("死子进程应报 errPiGone, 实得 %v", err)
	}
}

// GetState 全链路(真实 readLoop/dispatch/解析管线)在"延迟就绪"的假 pi 上成功
// 且会话缓存回填 —— 探针第 2 项(get_state 握手)的验收形状。
func TestGetStateAgainstDelayedFakePi(t *testing.T) {
	c := newDelayedFakePi(t, 2)
	st, err := c.GetState(context.Background())
	if err != nil {
		t.Fatalf("延迟就绪下 GetState 应成功: %v", err)
	}
	if c.SessionID() != "fake-session-id" || c.SessionFile() != `C:\fake\pi-session.jsonl` {
		t.Fatalf("会话缓存未回填: file=%q id=%q", c.SessionFile(), c.SessionID())
	}
	if st["sessionId"] != "fake-session-id" {
		t.Fatalf("get_state data 形状不符: %v", st)
	}
}

// SwitchSession 全链路:switch + get_state 校验 + 缓存覆盖 —— 探针第 7 项
// (switch_session 恢复历史)的传输层验收形状(历史内容归 get_messages/
// get_entries,agent 层职责)。
func TestSwitchSessionAgainstFakePi(t *testing.T) {
	c := newDelayedFakePi(t, 0)
	// 假 pi 的 get_state 恒回 C:\fake\pi-session.jsonl:目标必须一致才过校验
	if err := c.SwitchSession(context.Background(), `C:\fake\pi-session.jsonl`); err != nil {
		t.Fatalf("SwitchSession 应成功: %v", err)
	}
	if c.SessionFile() != `C:\fake\pi-session.jsonl` {
		t.Fatalf("会话缓存未覆盖: %q", c.SessionFile())
	}
}

// ---- set_model 的 provider 维度(1.0.2 find 谓词;真机 15d/15e 回退根因)----

// 拆解纯函数:首段 provider、其余整体 modelId(openrouter 类含 "/" 的模型 id
// 不误伤);缺 "/" 或段空一律拒绝。
func TestSplitPiModelSpec(t *testing.T) {
	check := func(spec, wantProvider, wantModelID string, wantOK bool) {
		t.Helper()
		provider, modelID, ok := splitPiModelSpec(spec)
		if ok != wantOK || provider != wantProvider || modelID != wantModelID {
			t.Fatalf("splitPiModelSpec(%q) = (%q,%q,%v), 期望 (%q,%q,%v)",
				spec, provider, modelID, ok, wantProvider, wantModelID, wantOK)
		}
	}
	check("zai-coding-cn/glm-5.3", "zai-coding-cn", "glm-5.3", true)
	check("openrouter/meta-llama/llama-3", "openrouter", "meta-llama/llama-3", true)
	check("glm-5.3", "", "", false)        // 无 provider 段(旧形状祸根)
	check("zai-coding-cn/", "", "", false) // 空 modelId
	check("/glm-5.3", "", "", false)       // 空 provider
	check("", "", "", false)               // 空规格
}

// 无 provider 段的规格必须本地拒绝(不发 provider 缺省的 set_model —— 那正是
// 1.0.2 恒 "Model not found: undefined/…" → 跨轮次静默回退的旧形状)。
func TestSetModelRejectsProviderLessSpec(t *testing.T) {
	c := newDelayedFakePi(t, 0)
	err := c.SetModel(context.Background(), "deepseek-flash")
	if err == nil || !strings.Contains(err.Error(), "无效 Pi 模型规格") {
		t.Fatalf("无 provider 段的规格应本地拒绝: %v", err)
	}
}

// 线上形状断言:期望二元组经 PA_FAKE_PI_EXPECT_MODEL 注入假 pi,只有线上帧的
// provider/modelId 恰为该值才应答成功 —— SetModel("zai-coding-cn/glm-5.3")
// 成功即证明拆解后字段在场且原样上送;值不符的规格被 fake 以 success:false
// 拒绝(防"字段在场但值错"的退化)。
func TestSetModelWireShape(t *testing.T) {
	c := newFakePiChildEnv(t, 0, "PA_FAKE_PI_EXPECT_MODEL=zai-coding-cn/glm-5.3")
	if err := c.SetModel(context.Background(), "zai-coding-cn/glm-5.3"); err != nil {
		t.Fatalf("provider/id 拆解后应原样上送(fake 按二元组校验): %v", err)
	}
	if err := c.SetModel(context.Background(), "deepseek/deepseek-chat"); err == nil {
		t.Fatal("值不符的二元组应被 fake 拒绝(success:false 必须变成 error)")
	}
}
