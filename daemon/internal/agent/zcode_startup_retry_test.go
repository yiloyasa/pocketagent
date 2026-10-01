// zcode_startup_retry_test.go —— 钉住 78c5f9f「启动闪退即重试一次」的三个边界
// (agent/zcode.go withClientCtx 的 errNoZcodeClient 路径):
//
//	① 首拉即闪退 → 换新进程重试一次,协议路径全通(重试触发);
//	② 重试仍闪退 → 原样上抛,绝不第三次拉起(重试仍死上抛);
//	③ 首拉进程活着、fn 自行失败 → 不触发闪退重试(进程活着不重试);
//	④ 控制路径(withControlClient)自起子进程同样挂上常驻双钩子
//	  (2026-09-30 盘点缺口修复的钉子,见 zcode_selection.go)。
//
// 【fake 子进程怎么来】ZCODE_NODE / ZCODE_CJS 是 LocateZCode 的官方测试注入口
// (zcodeapp/client.go),这里把 ZCODE_NODE 指回**测试二进制自己**:zcodeLaunchSpec
// 拉起 `<node> <cjs> app-server --stdio --surface desktop`,子进程的
// os.Args[2] == "app-server" 且带 PA_FAKE_ZCODE_MODE 时,下方 init() 门直接进入
// fake 协议循环(与本包 codex_effect_test.go 的 TestMain 门同款先例;init 先于
// TestMain 与全部测试执行,普通 `go test` 的 os.Args 全是 -test.* 旗标,永不触发)。
// 三种模式:crash(启动即 exit 1,零 stderr)/ serve(NDJSON 应答循环)/
// crash-then-serve(第一次 boot 造标记后 exit 1,之后正常 serve)。
// 纯 Go 实现,不依赖真 node / zcode.cjs,任何机器可复跑。
package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pocketagent/daemon/internal/zcodeapp"
)

// init —— fake zcode app-server 的进程门(见文件头)。必须同时满足
// 「以 app-server 参数被拉起」与「带着 fake 模式环境变量」两个条件,普通测试
// 进程(os.Args 全是 -test.* 旗标、无该环境变量)零影响。
func init() {
	if len(os.Args) > 2 && os.Args[2] == "app-server" && os.Getenv("PA_FAKE_ZCODE_MODE") != "" {
		os.Exit(runFakeZcodeAppServer())
	}
}

// runFakeZcodeAppServer 是 fake app-server 主循环。stdout 是协议管道,只写
// NDJSON 应答,任何日志都进文件(boot 标记)或 stderr,绝不混进 stdout。
func runFakeZcodeAppServer() int {
	mode := os.Getenv("PA_FAKE_ZCODE_MODE")
	zcodeFakeAppendBoot()
	switch mode {
	case "crash":
		// 复刻真机偶发症状:拉起即 exit 1,零 stderr(78c5f9f 的背景死法)。
		return 1
	case "crash-then-serve":
		marker := os.Getenv("PA_FAKE_ZCODE_MARKER")
		if marker == "" {
			return 2
		}
		if _, err := os.Stat(marker); err == nil {
			return zcodeFakeServe() // 已闪退过一次:这次正常服务
		}
		if err := os.WriteFile(marker, []byte("crashed"), 0o644); err != nil {
			return 2
		}
		return 1
	case "serve":
		return zcodeFakeServe()
	}
	return 2
}

// zcodeFakeServe NDJSON 应答循环:对每个带 id 的请求回 {"id":…,"result":{"ok":true}}。
// provider 测试路径只发 runtime/capabilities 之类的普通请求,无需实现反向请求。
func zcodeFakeServe() int {
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 64*1024), 8*1024*1024)
	out := json.NewEncoder(os.Stdout)
	for in.Scan() {
		var m struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		if json.Unmarshal(in.Bytes(), &m) != nil || len(m.ID) == 0 {
			continue // 通知/坏帧:吞掉不回
		}
		if err := out.Encode(map[string]any{"id": m.ID, "result": map[string]any{"ok": true}}); err != nil {
			return 3
		}
	}
	return 0
}

// zcodeFakeAppendBoot 往 PA_FAKE_ZCODE_LOG 追加一行 boot 标记:父进程据此数
// 「实际拉起了几个子进程」(重试次数的地面真值)。写在一切行为之前:闪退
// (exit 1)与首次应答都必然以它为前奏,读数不需要额外同步等待。
func zcodeFakeAppendBoot() {
	logPath := os.Getenv("PA_FAKE_ZCODE_LOG")
	if logPath == "" {
		return
	}
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString("start\n")
}

// zcodeRetryFakeEnv 把 provider 的 LocateZCode 指到测试二进制自己(每个测试独立
// 的临时 log/marker,t.Setenv 自动恢复,互不串扰)。
func zcodeRetryFakeEnv(t *testing.T, mode string) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("拿测试二进制路径: %v", err)
	}
	t.Setenv("ZCODE_NODE", exe)
	cjs := filepath.Join(t.TempDir(), "zcode.cjs")
	if err := os.WriteFile(cjs, []byte("// fake zcode app-server stub\n"), 0o644); err != nil {
		t.Fatalf("造假 cjs: %v", err)
	}
	t.Setenv("ZCODE_CJS", cjs)
	t.Setenv("PA_FAKE_ZCODE_MODE", mode)
	logPath := filepath.Join(t.TempDir(), "boot.log")
	t.Setenv("PA_FAKE_ZCODE_LOG", logPath)
	t.Setenv("PA_FAKE_ZCODE_MARKER", filepath.Join(t.TempDir(), "crashed.marker"))
	return logPath
}

func zcodeBootCount(t *testing.T, logPath string) int {
	t.Helper()
	raw, err := os.ReadFile(logPath)
	if err != nil {
		return 0
	}
	return strings.Count(string(raw), "start\n")
}

// zcodeWaitBootCount 轮询等待 boot 计数到 want(子进程写文件与父进程读有毫秒级
// 错拍);超时按实际计数报错。
func zcodeWaitBootCount(t *testing.T, logPath string, want int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if zcodeBootCount(t, logPath) >= want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("fake app-server boot 次数未达 %d(实际 %d)", want, zcodeBootCount(t, logPath))
}

// zcodeWaitDeath 轮询等 reap 收尸完成(Alive 翻 false)。78c5f9f 的重试判定读的
// 就是这一位:EOF → reapWithLog 是异步的,fn 里等它落定再返回错误,才能让
// 「!Alive() → 重试」的分支被确定性命中,而不是碰运气。
func zcodeWaitDeath(t *testing.T, c *zcodeapp.Client) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !c.Alive() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("fake 子进程 5s 内未被收尸(Alive 仍为真),测试前提不成立")
}

// ① 重试触发:首次拉起即闪退 → 换新进程重试一次 → 协议应答全通、恰好两次拉起、
// client 换了进程。这就是 78c5f9f 要救的主症状("偶发闪退把 provider 整个判死")。
func TestZcodeStartCrashRetrySucceedsOnSecondStart(t *testing.T) {
	logPath := zcodeRetryFakeEnv(t, "crash-then-serve")
	p := NewZcodeProvider()
	t.Cleanup(func() {
		if p.client != nil {
			p.client.Close()
		}
	})

	calls := 0
	pidFirst := 0
	fn := func(ctx context.Context, c *zcodeapp.Client) error {
		calls++
		if calls == 1 {
			pidFirst = c.PID()
			if _, err := c.CallCtx(ctx, "runtime/capabilities", map[string]any{}); err == nil {
				t.Error("首个进程应已闪退,CallCtx 却成功了")
				return nil
			}
			zcodeWaitDeath(t, c)
			return errors.New("boom: 首个进程已闪退")
		}
		// 重试后的新进程:协议路径必须全通(fake 正常应答)。
		if _, err := c.CallCtx(ctx, "runtime/capabilities", map[string]any{}); err != nil {
			return fmt.Errorf("重试后的新进程应答失败: %w", err)
		}
		return nil
	}
	if err := p.withClientCtx(context.Background(), zcodeStatusBudget, fn); err != nil {
		t.Fatalf("闪退重试后应当成功,却得到: %v", err)
	}
	if calls != 2 {
		t.Errorf("fn 应被调用 2 次(原试+重试),实际 %d", calls)
	}
	if p.client == nil || p.client.PID() == 0 || p.client.PID() == pidFirst {
		t.Errorf("重试后应持有一个新进程,实际 client=%v pid=%d(旧 pid=%d)",
			p.client != nil, p.client.PID(), pidFirst)
	}
	zcodeWaitBootCount(t, logPath, 2)
	// withClientCtx 返回后不可能再拉起:此刻计数必须恰好为 2。
	if n := zcodeBootCount(t, logPath); n != 2 {
		t.Errorf("子进程应恰好拉起 2 次,实际 %d", n)
	}
}

// ② 重试仍死上抛:两次拉起都闪退 → 错误原样返回,绝不第三次拉起
// ("真配置错误不该被重试掩盖")。
func TestZcodeStartCrashRetryStillDeadPropagates(t *testing.T) {
	logPath := zcodeRetryFakeEnv(t, "crash")
	p := NewZcodeProvider()
	t.Cleanup(func() {
		if p.client != nil {
			p.client.Close()
		}
	})

	calls := 0
	fn := func(ctx context.Context, c *zcodeapp.Client) error {
		calls++
		if _, err := c.CallCtx(ctx, "runtime/capabilities", map[string]any{}); err == nil {
			t.Error("crash 模式下 CallCtx 不可能成功")
		}
		zcodeWaitDeath(t, c)
		return errors.New("boom: 进程仍闪退")
	}
	err := p.withClientCtx(context.Background(), zcodeStatusBudget, fn)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("重试仍死必须原样上抛(含 boom),实际: %v", err)
	}
	if calls != 2 {
		t.Errorf("fn 应被调用恰好 2 次(原试+重试,无第三次),实际 %d", calls)
	}
	zcodeWaitBootCount(t, logPath, 2)
	if n := zcodeBootCount(t, logPath); n != 2 {
		t.Errorf("重试仍死时子进程应恰好拉起 2 次(不重试第三次),实际 %d", n)
	}
}

// ③ 进程活着不重试:首个进程活着且能正常应答,fn 自己失败 → 闪退重试分支不得
// 触发(活着就说明不是启动闪退,不扔进程),错误原样返回、恰好一次拉起。
func TestZcodeStartAliveDoesNotCrashRetry(t *testing.T) {
	logPath := zcodeRetryFakeEnv(t, "serve")
	p := NewZcodeProvider()
	t.Cleanup(func() {
		if p.client != nil {
			p.client.Close()
		}
	})

	calls := 0
	fn := func(ctx context.Context, c *zcodeapp.Client) error {
		calls++
		// 先证明进程活着且协议可达,再模拟一次业务失败(非 RPC、非闪退)。
		if _, err := c.CallCtx(ctx, "runtime/capabilities", map[string]any{}); err != nil {
			return fmt.Errorf("serve 模式下应答失败: %w", err)
		}
		return errors.New("boom: 业务失败但进程活着")
	}
	err := p.withClientCtx(context.Background(), zcodeStatusBudget, fn)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("fn 的错误应原样返回(不触发闪退重试),实际: %v", err)
	}
	if calls != 1 {
		t.Errorf("进程活着时 fn 应只被调用 1 次,实际 %d", calls)
	}
	zcodeWaitBootCount(t, logPath, 1)
	if n := zcodeBootCount(t, logPath); n != 1 {
		t.Errorf("进程活着不得换新进程,应恰好拉起 1 次,实际 %d", n)
	}
	if p.client == nil || !p.client.Alive() {
		t.Error("进程活着不重试:client 应原样保留且仍活着")
	}
}

// ④ 控制路径补挂钩子的钉子(withControlClient 自起子进程时):全局通知钩子与
// 未知应答钩子必须与 withClientCtx 的三个 Start 点位同款在场 —— 否则控制类操作
// 先于 Run 到来时,这条 client 整个生命周期诊断台账失明。
func TestZcodeControlClientStartAttachesHandlers(t *testing.T) {
	logPath := zcodeRetryFakeEnv(t, "serve")
	p := NewZcodeProvider()
	t.Cleanup(func() {
		if p.client != nil {
			p.client.Close()
		}
	})

	if err := p.withControlClient(context.Background(), func(context.Context, *zcodeapp.Client) error {
		return nil
	}); err != nil {
		t.Fatalf("withControlClient 应成功: %v", err)
	}
	if p.client == nil {
		t.Fatal("withControlClient 后 client 应就位")
	}
	if p.client.GlobalHandler == nil {
		t.Error("控制路径自起的子进程缺 SetGlobalHandler(轮次外通知将静默丢弃)")
	}
	if p.client.UnknownResponseHandler == nil {
		t.Error("控制路径自起的子进程缺 SetUnknownResponseHandler(未知应答将无留痕)")
	}
	zcodeWaitBootCount(t, logPath, 1)
}
