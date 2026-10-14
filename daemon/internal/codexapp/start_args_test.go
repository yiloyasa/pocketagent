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

	"github.com/pocketagent/daemon/internal/testkit"
)

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
	raw, err := testkit.WaitFile(t, argvFile, 5*time.Second)
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
	raw, err := testkit.WaitFile(t, argvFile, 5*time.Second)
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

// TestScriptCmdLineQuotesSpacedPath 引证形态钉子(2026-10-05,CODEX_BIN 含空格
// 修复):.cmd/.bat 经 cmd.exe 启动时,命令行必须是 `cmd.exe /d /s /c ""脚本"
// 参数…"` 的规范引证 —— 最外层一对引号 + /s 保证 cmd 恒定只剥最外层,脚本路径
// 引号永不丢失;其余参数逐个 syscall.EscapeArg。活体踩坑:本机 CODEX_BIN=
// G:\Claude Code\codex.cmd(含空格),旧形态 `cmd /c "G:\Claude Code\codex.cmd"
// args…` 在行内任一参数自带引号(协同注入的 -c 值)时引号数 >2,cmd 落入
// "剥首字符+最后一个引号"旧规则,报 `'G:\Claude' 不是内部或外部命令`,
// codex provider 整体不可用(daemon.log 实证)。
func TestScriptCmdLineQuotesSpacedPath(t *testing.T) {
	got := scriptCmdLine(`G:\Claude Code\codex.cmd`,
		[]string{"app-server", "-c", "notify=[]", "--disable", "apps"})
	// 脚本路径必须整体在引号里,且位于最外层引号对之内(cmd /s 剥掉最外层后,
	// 执行文以 "G:\Claude Code\codex.cmd" 开头)。
	if !strings.HasPrefix(got, `cmd.exe /d /s /c ""G:\Claude Code\codex.cmd" `) {
		t.Fatalf("脚本路径必须被引住且整体包裹在最外层引号对内: %s", got)
	}
	if !strings.HasSuffix(got, `"`) {
		t.Fatalf("最外层引号必须闭合(/s 剥离后执行文才完整): %s", got)
	}
	// 参数逐个在场且 daemonScopeArgs 的引号敏感值(notify=[])原样透传。
	for _, want := range []string{"app-server", "-c", "notify=[]", "--disable", "apps"} {
		if !strings.Contains(got, want) {
			t.Fatalf("参数 %q 必须在场: %s", want, got)
		}
	}
	// 不含空格的脚本路径同样成立(形态统一,不给两套规则留分叉)。
	simple := scriptCmdLine(`C:\bin\codex.cmd`, []string{"app-server"})
	if !strings.HasPrefix(simple, `cmd.exe /d /s /c ""C:\bin\codex.cmd" app-server"`) {
		t.Fatalf("无空格路径应走同一规范形态: %s", simple)
	}
}
