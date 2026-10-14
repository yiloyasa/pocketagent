package selfguard

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testCfg(dir string) Config {
	return Config{Exe: "daemon.exe", Addr: "0.0.0.0:7246", DataDir: dir}
}

func TestProbeAddr(t *testing.T) {
	cases := map[string]string{
		"0.0.0.0:7246":   "127.0.0.1:7246",
		":7246":          "127.0.0.1:7246",
		"127.0.0.1:8080": "127.0.0.1:8080",
		"[::1]:9000":     "127.0.0.1:9000",
		"":               "127.0.0.1:7246",
		"noport":         "127.0.0.1:7246",
		"host:99999":     "127.0.0.1:7246",
	}
	for in, want := range cases {
		if got := ProbeAddr(in); got != want {
			t.Errorf("ProbeAddr(%q)=%q want %q", in, got, want)
		}
	}
}

func TestLockExclusion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, LockName)
	h1, err := tryLock(path)
	if err != nil || h1 == nil {
		t.Fatalf("首个锁必须拿到: %v", err)
	}
	defer h1.unlock()
	// 同进程第二个 tryLock 必须失败（独占语义；Windows/Unix 同断言）。
	h2, err := tryLock(path)
	if err == nil && h2 != nil {
		_ = h2.unlock()
		t.Fatal("锁被占时第二个 tryLock 必须失败")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("锁文件必须存在: %v", err)
	}
}

func TestLockReacquireAfterUnlock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, LockName)
	h1, _ := tryLock(path)
	if h1 == nil {
		t.Skip("首个锁没拿到，环境不支持文件锁")
	}
	_ = h1.unlock()
	h2, err := tryLock(path)
	if err != nil || h2 == nil {
		t.Fatalf("放锁后必须能重拿: %v", err)
	}
	_ = h2.unlock()
}

func TestDisabledMarker(t *testing.T) {
	dir := t.TempDir()
	if Disabled(dir) {
		t.Fatal("无标记时必须报告未停用")
	}
	if err := os.WriteFile(filepath.Join(dir, DisableName), []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	if !Disabled(dir) {
		t.Fatal("有标记时必须报告停用")
	}
}

func TestParentChildArgs(t *testing.T) {
	dir := t.TempDir()
	cfg := testCfg(dir)
	for _, a := range ParentArgs(cfg) {
		if a == "-guard-child" {
			t.Fatal("父进程参数禁止带 -guard-child（否则拉起的是第二个看门狗）")
		}
	}
	child := ChildArgs(cfg)
	found := false
	for _, a := range child {
		if a == "-guard-child" {
			found = true
		}
	}
	if !found {
		t.Fatal("子进程参数必须带 -guard-child")
	}
	cfg.Workspace = ""
	if len(ParentArgs(cfg)) != 4 {
		t.Fatalf("空 workspace 不传参，got %v", ParentArgs(cfg))
	}
}

func memProbes(dialErr error, launched *[][]string) Probes {
	return Probes{
		Dial: func(addr string, timeout time.Duration) error { return dialErr },
		Launch: func(exe string, args []string) error {
			*launched = append(*launched, append([]string{exe}, args...))
			return nil
		},
		Log:   func(format string, args ...any) {},
		Sleep: func(d time.Duration) {},
	}
}

func TestLoopLaunchesAfterMaxFail(t *testing.T) {
	dir := t.TempDir()
	var launched [][]string
	p := memProbes(errors.New("refused"), &launched)
	// 5 轮全失败，maxFail=3：第 3 轮拉一次，第 5 轮计数到 2 不拉。
	if err := Loop(testCfg(dir), p, time.Millisecond, 3, 5); err != nil {
		t.Fatal(err)
	}
	if len(launched) != 1 {
		t.Fatalf("期望恰拉起 1 次，got %d", len(launched))
	}
	if launched[0][0] != "daemon.exe" {
		t.Fatalf("拉起的必须是父进程自己，got %v", launched[0])
	}
}

func TestLoopResetsOnSuccess(t *testing.T) {
	dir := t.TempDir()
	var launched [][]string
	p := memProbes(nil, &launched)
	if err := Loop(testCfg(dir), p, time.Millisecond, 3, 10); err != nil {
		t.Fatal(err)
	}
	if len(launched) != 0 {
		t.Fatalf("全成功必须零拉起，got %d", len(launched))
	}
}

func TestLoopExitsOnDisableMarker(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, DisableName), []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	var launched [][]string
	p := memProbes(errors.New("refused"), &launched)
	// 无限轮 + 首轮即见标记：必须立刻返回而不是空转。
	if err := Loop(testCfg(dir), p, time.Hour, 1, 0); err != nil {
		t.Fatal(err)
	}
	if len(launched) != 0 {
		t.Fatal("停用标记下禁止拉起")
	}
}

func TestPortBusyText(t *testing.T) {
	s := PortBusyText("0.0.0.0:7246", errors.New("bind: address already in use"))
	for _, want := range []string{"7246", "netstat", "-addr"} {
		if !strings.Contains(s, want) {
			t.Errorf("端口被占人话缺 %q: %s", want, s)
		}
	}
}
