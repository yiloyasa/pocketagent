package zcodeapp

// client_test.go —— 传输层单测:帧形状、id 回显、autoAnswer、应答关联。
// 全部形状来自 cmd/zcprobe 实证(docs/verify/evidence/zcode-p0-probe.md),
// 目的是钉住"无 jsonrpc 字段""字符串 id 原样回显""Jb 严格形状"这三条地面真值。

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pocketagent/daemon/internal/testkit"
)

// newTestClient 造一个手动的 Client:stdin 用内存缓冲代替管道,
// 不拉子进程(readLoop 由测试直接驱动 dispatch)。
func newTestClient() (*Client, *syncBuffer) {
	buf := &syncBuffer{}
	c := &Client{
		stdin:   buf,
		pending: map[int64]chan rpcResp{},
	}
	return c, buf
}

// syncBuffer 并发安全的内存 writer(代替 stdin 管道)。
type syncBuffer struct {
	mu     sync.Mutex
	buf    []byte
	closed bool
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = append(b.buf, p...)
	return len(p), nil
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.buf)
}

func (b *syncBuffer) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	return nil
}

// wasClosed 是否已被 Close(内部锁保证与 Close 调用方有 happens-before,
// 测试读它不会与实现侧产生数据竞争)。
func (b *syncBuffer) wasClosed() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.closed
}

func TestAutoAnswerRuntimePreferencesShape(t *testing.T) {
	c, buf := newTestClient()
	// 服务器反向请求:字符串 id("server-1")+ 运行时偏好询问
	c.dispatch(&wireMsg{
		ID:     json.RawMessage(`"server-1"`),
		Method: "session/requestRuntimePreferences",
		Params: json.RawMessage(`{"sessionId":"sess_x","scope":"runtime-materialization"}`),
	})
	// 应答已改 goroutine 异步回写(见 client.go autoAnswer,对齐 P2-5 派发模式):
	// 照抄 TestReqHandlerFalseFallsBackTo32601 的轮询等帧,保证确定性。
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(buf.String(), `"preflight-v1"`) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	out := buf.String()
	var resp struct {
		ID     json.RawMessage `json:"id"`
		Result struct {
			NativeSearchEnhancementsEnabled bool   `json:"nativeSearchEnhancementsEnabled"`
			MemoryEnabled                   bool   `json:"memoryEnabled"`
			AskUserQuestionAutoResolution   bool   `json:"askUserQuestionAutoResolutionEnabled"`
			ModelContextBudgetStrategy      string `json:"modelContextBudgetStrategy"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("应答不是合法 JSON: %v\n%s", err, out)
	}
	if string(resp.ID) != `"server-1"` {
		t.Fatalf("id 必须原样回显为字符串,得到 %s", string(resp.ID))
	}
	if strings.Contains(out, "jsonrpc") {
		t.Fatalf("帧里绝不允许出现 jsonrpc 字段(实证会被 strict 校验拒绝): %s", out)
	}
	if resp.Result.ModelContextBudgetStrategy != "preflight-v1" {
		t.Fatalf("modelContextBudgetStrategy 应为 preflight-v1,得到 %q", resp.Result.ModelContextBudgetStrategy)
	}
}

func TestAutoAnswerStoragePathReady(t *testing.T) {
	c, buf := newTestClient()
	c.dispatch(&wireMsg{
		ID:     json.RawMessage(`"server-9"`),
		Method: "startup/storagePath",
		Params: json.RawMessage(`{"path":"C:\\x"}`),
	})
	// 应答已改 goroutine 异步回写:同款轮询等帧(见 TestReqHandlerFalseFallsBackTo32601)。
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(buf.String(), `"startup/storagePathReady"`) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	out := buf.String()
	if !strings.Contains(out, `"method":"startup/storagePathReady"`) {
		t.Fatalf("storagePath 应答应带 method 标记: %s", out)
	}
	if !strings.Contains(out, `"reuse":true`) {
		t.Fatalf("storagePath 应答应带 reuse:true: %s", out)
	}
}

func TestUnknownServerRequestGetsError(t *testing.T) {
	c, buf := newTestClient()
	c.dispatch(&wireMsg{
		ID:     json.RawMessage(`"server-3"`),
		Method: "elicitation/create",
		Params: json.RawMessage(`{}`),
	})
	// 无 handler 的兜底 -32601 已改 goroutine 异步回写(见 client.go dispatch):
	// 同款轮询等帧(见 TestReqHandlerFalseFallsBackTo32601)。
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(buf.String(), `-32601`) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	out := buf.String()
	if !strings.Contains(out, `-32601`) {
		t.Fatalf("未实现的反向请求应回 -32601: %s", out)
	}
	if strings.Contains(out, `"result"`) {
		t.Fatalf("不支持的请求绝不能伪造 result: %s", out)
	}
}

func TestReqHandlerDelegation(t *testing.T) {
	c, buf := newTestClient()
	// P2-5:ReqHandler 已改 goroutine 分发 —— 测试经 channel 与 handler 同步,
	// 不再用共享变量跨 goroutine 读写(-race 必报)。
	gotMethod := make(chan string, 1)
	c.ReqHandler = func(rawID json.RawMessage, method string, params json.RawMessage) bool {
		_ = c.Respond(rawID, map[string]string{"decision": "allow"})
		gotMethod <- method // 先落盘应答再通知:主 goroutine 收到时帧已写出
		return true
	}
	c.SyncHandlers() // dispatch 只读快照,装卸后必须发布
	c.dispatch(&wireMsg{
		ID:     json.RawMessage(`"server-5"`),
		Method: "interaction/requestPermission",
		Params: json.RawMessage(`{"toolName":"Write"}`),
	})
	select {
	case m := <-gotMethod:
		if m != "interaction/requestPermission" {
			t.Fatalf("审批请求应交给 ReqHandler,得到 %q", m)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ReqHandler 未被调用")
	}
	if !strings.Contains(buf.String(), `"decision":"allow"`) {
		t.Fatalf("ReqHandler 的应答应写出: %s", buf.String())
	}
}

func TestResponseCorrelationAndError(t *testing.T) {
	c, _ := newTestClient()
	ch := make(chan rpcResp, 1)
	c.mu.Lock()
	c.pending[7] = ch
	c.mu.Unlock()
	// 正常应答(数字 id)
	c.dispatch(&wireMsg{ID: json.RawMessage(`7`), Result: json.RawMessage(`{"ok":true}`)})
	r := <-ch
	if r.err != nil || string(r.raw) != `{"ok":true}` {
		t.Fatalf("应答关联失败: raw=%s err=%v", r.raw, r.err)
	}
	// 错误应答必须变 Go error,不能混成空结果(经 JSON 解析,保证与真实帧同形状)
	ch2 := make(chan rpcResp, 1)
	c.mu.Lock()
	c.pending[8] = ch2
	c.mu.Unlock()
	var errFrame wireMsg
	if err := json.Unmarshal([]byte(`{"id":8,"error":{"code":-32022,"message":"Client request timed out"}}`), &errFrame); err != nil {
		t.Fatal(err)
	}
	c.dispatch(&errFrame)
	r2 := <-ch2
	if r2.err == nil || !strings.Contains(r2.err.Error(), "-32022") {
		t.Fatalf("错误应答应转成带码的 error: %v", r2.err)
	}
}

func TestNotificationBothHandlers(t *testing.T) {
	c, _ := newTestClient()
	order := []string{}
	c.GlobalHandler = func(method string, params json.RawMessage) { order = append(order, "global:"+method) }
	c.NotifHandler = func(method string, params json.RawMessage) { order = append(order, "notif:"+method) }
	c.SyncHandlers() // P2-5:dispatch 只读快照,装卸后必须发布
	c.dispatch(&wireMsg{Method: "session/event", Params: json.RawMessage(`{}`)})
	if len(order) != 2 || order[0] != "global:session/event" || order[1] != "notif:session/event" {
		t.Fatalf("通知应先常驻钩子再当轮钩子: %v", order)
	}
}

// ---- Close 收尸(Kill 后必须 Wait,否则进程句柄/管道泄漏)----

// TestZCodeAppCloseHelperProcess Close 测试的子进程本体:长睡等待被父进程
// Kill(标准 exec.Cmd helper 范式;正常跑测试套件时无环境变量,直接返回)。
func TestZCodeAppCloseHelperProcess(t *testing.T) {
	if os.Getenv("ZCODEAPP_CLOSE_HELPER") != "1" {
		return
	}
	time.Sleep(60 * time.Second) // 永远不该自然醒来,只等 Kill
	os.Exit(0)
}

// TestCloseKillsProcessAndClosesStdin 钉 Close 的行为契约:子进程被杀、
// stdin 被关、Close 不阻塞。收尸(cmd.Wait)由 Close 内部 goroutine 完成,
// 测试只经 OS 句柄观察进程退出,绝不碰 exec.Cmd 内部状态(-race 下 Wait
// 并发访问 ProcessState 会报竞争)。
func TestCloseKillsProcessAndClosesStdin(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestZCodeAppCloseHelperProcess$")
	cmd.Env = append(os.Environ(), "ZCODEAPP_CLOSE_HELPER=1")
	stdin := &syncBuffer{}
	c := &Client{cmd: cmd, stdin: stdin, pending: map[int64]chan rpcResp{}}
	if err := cmd.Start(); err != nil {
		t.Fatalf("拉起 helper 子进程失败: %v", err)
	}

	closed := make(chan struct{})
	go func() { defer close(closed); c.Close() }()
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("Close 不应阻塞(Kill/关管道都是同步快路径)")
	}

	if !stdin.wasClosed() {
		t.Fatal("Close 必须关闭 stdin 管道")
	}
	if err := testkit.WaitProcessGone(cmd.Process.Pid, 10*time.Second); err != nil {
		t.Fatalf("Close 后子进程应被 Kill 退出: %v", err)
	}
}

// ---- 2026-09-19 质量巡逻 P2 回归 ----

// P2-5:慢 ReqHandler 绝不能堵 readLoop —— 审批等待期间到达的通知必须照常
// 分发(修复前 ReqHandler 在 readLoop 里同步执行,审批等待期通知泵全停)。
func TestSlowReqHandlerDoesNotBlockNotifs(t *testing.T) {
	c, _ := newTestClient()
	handlerEntered := make(chan struct{})
	release := make(chan struct{})
	handlerDone := make(chan struct{})
	c.ReqHandler = func(rawID json.RawMessage, method string, params json.RawMessage) bool {
		close(handlerEntered)
		<-release // 模拟等手机审批(现实中最长 120s)
		close(handlerDone)
		return true
	}
	notifGot := make(chan string, 1)
	c.NotifHandler = func(method string, params json.RawMessage) { notifGot <- method }
	c.SyncHandlers()

	c.dispatch(&wireMsg{ID: json.RawMessage(`"server-1"`), Method: "interaction/requestPermission", Params: json.RawMessage(`{}`)})
	select {
	case <-handlerEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("handler 未被调用")
	}
	// handler 仍挂着:通知必须能立即送达(goroutine 分发的意义所在)
	c.dispatch(&wireMsg{Method: "session/event", Params: json.RawMessage(`{}`)})
	select {
	case m := <-notifGot:
		if m != "session/event" {
			t.Fatalf("通知类型不符: %s", m)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("慢 ReqHandler 堵住了通知分发(readLoop 被同步执行卡死)")
	}
	close(release)
	select {
	case <-handlerDone:
	case <-time.After(2 * time.Second):
		t.Fatal("handler 未在释放后完成")
	}
}

// P2-5:返回 false 的业务反向请求由客户端层统一回 -32601(不伪造 result)。
func TestReqHandlerFalseFallsBackTo32601(t *testing.T) {
	c, buf := newTestClient()
	c.ReqHandler = func(rawID json.RawMessage, method string, params json.RawMessage) bool { return false }
	c.SyncHandlers()
	c.dispatch(&wireMsg{ID: json.RawMessage(`"server-7"`), Method: "some/unknown", Params: json.RawMessage(`{}`)})
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) { // goroutine 异步回写,轮询等帧落盘
		if strings.Contains(buf.String(), "-32601") {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("返回 false 应回 -32601,实得: %s", buf.String())
}

// P2-5 + P1-1 同款:一个 goroutine 持续 dispatch(读快照),本 goroutine 反复
// 装卸裸字段并发布快照,跑 -race 验证无 data race。
func TestZcodeHandlerSnapshotRaceFree(t *testing.T) {
	c, _ := newTestClient()
	c.syncHandlers()

	stop := make(chan struct{})
	done := make(chan struct{})
	go func() { // 充当 readLoop:持续派发(读快照)
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
			}
			c.dispatch(&wireMsg{Method: "session/event", Params: json.RawMessage(`{}`)})
		}
	}()

	for i := 0; i < 2000; i++ { // 充当 provider Run:装卸裸字段 + 发布快照
		c.NotifHandler = func(method string, params json.RawMessage) {}
		c.ReqHandler = func(rawID json.RawMessage, method string, params json.RawMessage) bool { return true }
		c.SyncHandlers()
		c.NotifHandler = nil
		c.ReqHandler = nil
		c.SyncHandlers()
	}
	close(stop)
	<-done
}
