// client_test.go —— piapp 纯函数回归:扩展 UI 请求解析 + 守护扩展渲染。
package piapp

import (
	"bufio"
	"encoding/json"
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

// 定位器:环境变量优先,且不因 PATH 缺失而失败(daemon 由 WMI 派生时无用户 PATH)。
func TestLocatePiFallbacks(t *testing.T) {
	t.Setenv("PI_NODE", "")
	t.Setenv("PI_BIN", "")
	t.Setenv("PI_CLI", "")
	node, cli, err := LocatePi()
	if err != nil {
		t.Fatalf("本机固定路径应可定位: %v", err)
	}
	if node == "" || cli == "" {
		t.Fatalf("node/cli 都不能为空: %q %q", node, cli)
	}
	// PI_BIN 兜底(CODEX_BIN 式)
	t.Setenv("PI_BIN", node)
	if got := LocateNode(); got != node {
		t.Fatalf("PI_BIN 应优先生效: %q", got)
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
