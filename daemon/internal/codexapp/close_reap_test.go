package codexapp

// close_reap_test.go —— P1-7 回归钉子:Close 必须 Kill 后收尸(cmd.Wait),
// 退出码必须进日志。修复前 Close 只 Kill 不 Wait,进程/管道句柄泄漏且崩溃
// 无从诊断;收尸范式与 piapp/zcodeapp 的 reapWithLog 完全一致(waitOnce 保证
// cmd.Wait 只执行一次)。

import (
	"bytes"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pocketagent/daemon/internal/testkit"
)

// closeRecorder 记录 stdin 是否被 Close(测试观测用,代替真实管道)。
type closeRecorder struct {
	closed atomic.Bool
}

func (w *closeRecorder) Write(p []byte) (int, error) { return len(p), nil }
func (w *closeRecorder) Close() error                { w.closed.Store(true); return nil }
func (w *closeRecorder) wasClosed() bool             { return w.closed.Load() }

// TestCodexAppCloseHelperProcess Close 收尸测试的子进程本体:长睡等待被父进程
// Kill(标准 exec.Cmd helper 范式;正常跑测试套件时无环境变量,直接返回)。
func TestCodexAppCloseHelperProcess(t *testing.T) {
	if os.Getenv("CODEXAPP_CLOSE_HELPER") != "1" {
		return
	}
	time.Sleep(60 * time.Second) // 永远不该自然醒来,只等 Kill
	os.Exit(0)
}

// TestCloseKillsProcessAndReaps 钉 Close 的行为契约:子进程被杀、stdin 被关、
// Close 不阻塞、进程最终被收尸(cmd.Wait 在 Close 内部 goroutine 完成)。
// 收尸观察走 OS 句柄(与 zcodeapp 同款):测试读 cmd.ProcessState 与收尸
// goroutine 的 Wait 构成数据竞争,-race 必报。
func TestCloseKillsProcessAndReaps(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestCodexAppCloseHelperProcess$")
	cmd.Env = append(os.Environ(), "CODEXAPP_CLOSE_HELPER=1")
	stdin := &closeRecorder{}
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
		t.Fatalf("Close 后子进程应被 Kill 并收尸: %v", err)
	}
	// 重复 Close 不许 panic(waitOnce 兜底:第二次不会再 Wait)
	c.Close()
}

// TestReapWithLogWritesExitCode 钉死因日志:收尸必须把退出码与 why 标记写进
// 标准 log(daemon.log)。用真子进程 exit 7;同步调用 reapWithLog,捕获输出断言。
func TestReapWithLogWritesExitCode(t *testing.T) {
	cmd := exec.Command("cmd", "/c", "exit", "7")
	c := &Client{cmd: cmd, stdin: &closeRecorder{}, pending: map[int64]chan rpcResp{}}
	if err := cmd.Start(); err != nil {
		t.Skipf("无法拉起测试子进程: %v", err)
	}

	var buf bytes.Buffer
	old := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(old)

	c.reapWithLog("测试收尸") // 同步:返回即 Wait 已完成,此后读日志无竞争
	out := buf.String()
	if !strings.Contains(out, "退出码=7") {
		t.Fatalf("收尸日志必须带退出码(P1-7:EOF 只有症状,退出码才是死因): %q", out)
	}
	if !strings.Contains(out, "测试收尸") {
		t.Fatalf("收尸日志应带 why 标记区分退出原因: %q", out)
	}

	// waitOnce:第二次收尸不重复 Wait(exec.Cmd 不允许二次 Wait,重复只会
	// 记到第一次的结果),也不得 panic。
	c.reapWithLog("第二次收尸")
}

// TestReapWithLogNilCmdSafe readLoop 路径的 Client 可能没有 cmd(如单测直接
// 构造):reapWithLog 必须安全返回,不得 panic。
func TestReapWithLogNilCmdSafe(t *testing.T) {
	c := &Client{pending: map[int64]chan rpcResp{}}
	c.reapWithLog("无 cmd")
}
