// start_args_test.go —— 弹窗治理专项(2026-09-25)回归钉子:daemon 拉起的
// app-server 必须带上 daemonScopeArgs 覆盖(notify=[] + --disable apps)。
//
// 【为什么用 fake 子进程捕获真实 argv】Start 的参数拼装发生在生产函数内部,
// 单测若只看 daemonScopeArgs 变量,未来有人在 Start 里另拼一份参数就绕过了钉子。
// 这里走真 Start:CODEX_BIN 指向测试二进制自己,TestMain 顶部按 PA 环境变量分流
// —— 子进程把自己的 os.Args[1:] 原样落盘后立即退出,父进程读回逐项断言。
// (与 internal/agent/codex_effect_test.go 的 fake app-server 同惯用法。)
package codexapp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// waitFile 轮询等 fake 子进程落盘(Start 返回 ≠ 子进程已跑完;测试二进制冷启动
// 需要几百毫秒)。
func waitFile(t *testing.T, path string, d time.Duration) ([]byte, error) {
	t.Helper()
	deadline := time.Now().Add(d)
	for {
		if raw, err := os.ReadFile(path); err == nil {
			return raw, nil
		}
		if time.Now().After(deadline) {
			return nil, os.ErrNotExist
		}
		time.Sleep(25 * time.Millisecond)
	}
}

// TestMain 兼任 argv 捕获子进程入口:被 Start 以 CODEX_BIN 拉起时,
// os.Args[1:] == ["app-server", "-c", "notify=[]", "--disable", "apps", ...]。
func TestMain(m *testing.M) {
	if path := os.Getenv("PA_DAEMON_SCOPE_ARGV_FILE"); path != "" {
		_ = os.WriteFile(path, []byte(strings.Join(os.Args[1:], "\n")), 0o644)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestStartPassesDaemonScopeArgs(t *testing.T) {
	argvFile := filepath.Join(t.TempDir(), "argv.txt")
	t.Setenv("PA_DAEMON_SCOPE_ARGV_FILE", argvFile)
	t.Setenv("CODEX_BIN", os.Args[0]) // LocateCodex 认 CODEX_BIN → 指回测试二进制

	c, err := Start()
	if err != nil {
		t.Fatalf("Start(fake CODEX_BIN): %v", err)
	}
	// fake 子进程落盘 argv 后自行退出;先等文件再收尸(Close 的 Kill 会打断
	// 还没来得及写的子进程)。
	raw, err := waitFile(t, argvFile, 5*time.Second)
	c.Close()
	if err != nil {
		t.Fatalf("fake 子进程未写出 argv(Start 没按 CODEX_BIN 拉起?): %v", err)
	}
	got := strings.Split(strings.TrimSpace(string(raw)), "\n")
	want := append([]string{"app-server"}, daemonScopeArgs...)
	if len(got) != len(want) {
		t.Fatalf("app-server argv = %v, 期望 %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("argv[%d] = %q, 期望 %q(全量: got=%v want=%v)", i, got[i], want[i], got, want)
		}
	}
}

// daemonScopeArgs 的静态形状钉子:notify 清空与 apps 关闭必须同时在场,
// 且只允许这两个语义(新增覆盖必须显式更新本断言,防止悄悄扩大影响面)。
func TestDaemonScopeArgsPinned(t *testing.T) {
	joined := strings.Join(daemonScopeArgs, "\x00")
	if !strings.Contains(joined, "notify=[]") {
		t.Fatal("必须覆盖 notify=[](回合结束桌面通知/外部程序是弹窗源头)")
	}
	if !strings.Contains(joined, "--disable") || !strings.Contains(joined, "apps") {
		t.Fatal("必须 --disable apps(封 ps/mcp rmcp 重连刷屏)")
	}
	if len(daemonScopeArgs) != 4 {
		t.Fatalf("daemonScopeArgs 形状变化: %v —— 请同步更新 start_args_test.go 的两处断言", daemonScopeArgs)
	}
	// 覆盖必须紧跟 "app-server" 之后的参数位置(Start 内 append 语义),这里
	// 守住"daemonScopeArgs 本身不含 app-server"以防有人把重复前缀拼进列表。
	for _, a := range daemonScopeArgs {
		if a == "app-server" {
			t.Fatal("daemonScopeArgs 不得包含 app-server 前缀(Start 已自带)")
		}
	}
}

// Start 在 fake 子进程秒退时的收尾路径不阻塞、不泄漏:Close 幂等可重复调用。
func TestStartFakeChildCloseIdempotent(t *testing.T) {
	argvFile := filepath.Join(t.TempDir(), "argv2.txt")
	t.Setenv("PA_DAEMON_SCOPE_ARGV_FILE", argvFile)
	t.Setenv("CODEX_BIN", os.Args[0])
	c, err := Start()
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	done := make(chan struct{})
	go func() { c.Close(); c.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Close 幂等调用 5s 未返回")
	}
}

// TestStartAppendsCollabExtraArgs 协同 MCP 注入钩子(X2 收口)的 argv 落点:
// 钩子非 nil 时,返回的参数逐项追加在 daemonScopeArgs 之后 —— daemon 据此把
// pocketagent-collab 以 `-c mcp_servers.*` 内存态覆盖挂进 app-server(见
// internal/agent/collab_inject.go);nil = 与改造前逐字一致(上一条测试即钉此)。
func TestStartAppendsCollabExtraArgs(t *testing.T) {
	argvFile := filepath.Join(t.TempDir(), "argv.txt")
	t.Setenv("PA_DAEMON_SCOPE_ARGV_FILE", argvFile)
	t.Setenv("CODEX_BIN", os.Args[0])
	prev := CollabExtraArgs
	CollabExtraArgs = func() []string {
		return []string{"-c", `mcp_servers.pocketagent-collab.command="C:\pa\daemon.exe"`}
	}
	t.Cleanup(func() { CollabExtraArgs = prev })

	c, err := Start()
	if err != nil {
		t.Fatalf("Start(fake CODEX_BIN): %v", err)
	}
	raw, err := waitFile(t, argvFile, 5*time.Second)
	c.Close()
	if err != nil {
		t.Fatalf("fake 子进程未写出 argv: %v", err)
	}
	got := strings.Split(strings.TrimSpace(string(raw)), "\n")
	want := append([]string{"app-server"}, daemonScopeArgs...)
	want = append(want, "-c", `mcp_servers.pocketagent-collab.command="C:\pa\daemon.exe"`)
	if len(got) != len(want) {
		t.Fatalf("app-server argv = %v, 期望 %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("argv[%d] = %q, 期望 %q(协同覆盖应整体追加在 daemonScopeArgs 之后)", i, got[i], want[i])
		}
	}
}