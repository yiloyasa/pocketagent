// start_args_test.go —— 弹窗治理专项(2026-09-25)回归钉子:pi rpc 子进程的
// 启动参数形状必须与 0.87.1 实测合法的形状逐字一致。
//
// 【背景】AUDIT 挂过"pi 0.87.1 rpc 模式拉起即退待专项"。本专项实证(直接
// node 拉起 0.87.1 cli.js):
//   - `--mode rpc --no-session` 完全合法:子进程存活 30s+ 不退,
//     get_available_models 应答 success=true 且返回全量模型表;
//   - daemon.log 的"拉起→1~9s 后退出码=1"全部是 fetchModels 一次性探测的
//     正常 Close 收尸(Windows TerminateProcess 退出码恒 1),不是崩溃。
// 所以本文件不"修参数",而是**钉死参数**:今后任何改动 rpc 启动形状的提交,
// 若未重新探针实证,在这里先失败。
//
// 【怎么钉】两层:
//  1. buildRPCArgs 纯函数断言(三种生产形状:一次性探测/守护会话/textOnly);
//  2. Start 真实 spawn 的 argv 捕获 —— PI_NODE 指向测试二进制自己,TestMain
//     顶部按环境变量分流(先于 flag 解析,故 `--mode rpc` 不会炸测试二进制),
//     子进程把 os.Args[1:] 落盘后退出,父进程逐项断言(与 codexapp 的
//     start_args_test.go 同惯用法)。
package piapp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// waitFile 轮询等 fake 子进程落盘:Start 返回只代表 CreateProcess 成功,子进程
// (测试二进制冷启动)真正写出 argv 还要几百毫秒,父进程绝不能读完就断言。
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

// TestMain 兼任 argv 捕获子进程入口:Start 以 PI_NODE=<测试二进制> 拉起时,
// 本分流先于 m.Run() 执行,把 node 视角的 argv(os.Args[1:])原样落盘。
func TestMain(m *testing.M) {
	if path := os.Getenv("PA_PI_START_ARGV_FILE"); path != "" {
		_ = os.WriteFile(path, []byte(strings.Join(os.Args[1:], "\n")), 0o644)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// TestBuildRPCArgsPinned 钉死三种生产调用形状(fetchModels 一次性探测、
// clientFor 守护会话、textOnly 降级 —— 与 agent/pi.go 的调用点一一对应)。
func TestBuildRPCArgsPinned(t *testing.T) {
	cases := []struct {
		name string
		opts StartOptions
		want []string
	}{
		{
			name: "一次性模型探测(fetchModels:ExtraArgs 只带 --no-session)",
			opts: StartOptions{ExtraArgs: []string{"--no-session"}},
			want: []string{"cli.js", "--mode", "rpc", "--no-session"},
		},
		{
			name: "守护会话(clientFor:Name+GuardExtension)",
			opts: StartOptions{Name: "我的会话", GuardExtension: `C:\tmp\guard.ts`},
			want: []string{"cli.js", "--mode", "rpc", "--name", "我的会话", "--extension", `C:\tmp\guard.ts`},
		},
		{
			name: "textOnly 降级(clientFor:五连关 + 无守护扩展)",
			opts: StartOptions{ExtraArgs: []string{"--no-session", "--no-tools", "--no-extensions", "--no-skills", "--no-prompt-templates"}},
			want: []string{"cli.js", "--mode", "rpc", "--no-session", "--no-tools", "--no-extensions", "--no-skills", "--no-prompt-templates"},
		},
	}
	for _, tc := range cases {
		if got := buildRPCArgs("cli.js", tc.opts); !equalStrings(got, tc.want) {
			t.Fatalf("%s: buildRPCArgs = %v, 期望 %v", tc.name, got, tc.want)
		}
	}
	// 0.87.1 实测合法的最小形状不得被破坏:"--mode" 与 "rpc" 必须是相邻的一对,
	// 且紧跟 cli 之后(历史上拆散成 "--mode rpc" 两个独立旗标的写法一旦回归,
	// pi 会把 "rpc" 当作未知参数直接退出)。
	base := buildRPCArgs("cli.js", StartOptions{})
	if len(base) != 3 || base[1] != "--mode" || base[2] != "rpc" {
		t.Fatalf("rpc 最小形状被改动: %v", base)
	}
}

// TestStartPassesRPCArgs 走真 Start:捕获运行时 argv,断言与 buildRPCArgs 一致
// (防未来有人在 Start 里绕过 buildRPCArgs 另拼一份)。
func TestStartPassesRPCArgs(t *testing.T) {
	argvFile := filepath.Join(t.TempDir(), "argv.txt")
	t.Setenv("PA_PI_START_ARGV_FILE", argvFile)
	// LocateNode 认 PI_NODE、LocateCLI 认 PI_CLI:都指向测试二进制自己(必然存在)。
	t.Setenv("PI_NODE", os.Args[0])
	t.Setenv("PI_CLI", os.Args[0])

	c, err := Start(StartOptions{ExtraArgs: []string{"--no-session"}})
	if err != nil {
		t.Fatalf("Start(fake PI_NODE/PI_CLI): %v", err)
	}
	// fake 子进程落盘 argv 后自行退出(os.Exit(0));先等文件再收尸 —— Close 的
	// Kill 会打断"还没来得及写"的子进程(测试二进制冷启动要几百毫秒)。
	raw, err := waitFile(t, argvFile, 5*time.Second)
	c.Close()
	if err != nil {
		t.Fatalf("fake 子进程未写出 argv(Start 没按 PI_NODE 拉起?): %v", err)
	}
	got := strings.Split(strings.TrimSpace(string(raw)), "\n")
	want := buildRPCArgs(os.Args[0], StartOptions{ExtraArgs: []string{"--no-session"}})
	if !equalStrings(got, want) {
		t.Fatalf("运行时 argv = %v, 期望 %v", got, want)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
