// Command paguard 是 PocketAgent daemon 的无窗口看门狗,取代旧的
// watchdog.bat(cmd /c + netstat + findstart 链)。
//
// 【为什么改】旧链路:计划任务 → wscript → WMI 拉起 "cmd /c watchdog.bat"。
// WMI 在交互会话里创建的控制台进程会分配一个可见控制台,于是桌面每分钟
// 闪一次 cmd 黑窗。paguard 编译为 windowsgui 子系统(-H=windowsgui),
// 物理上不可能创建控制台窗口;端口探测用进程内 TCP 拨号,不再 exec
// netstat/findstr。启动 daemon 复用 internal/hidewin(CREATE_NO_WINDOW)。
package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/pocketagent/daemon/internal/hidewin"
)

func main() {
	var (
		probeAddr  = flag.String("probe", "127.0.0.1:7246", "探测端口是否有人在听")
		daemonAddr = flag.String("daemon-addr", "0.0.0.0:7246", "daemon 监听地址")
		pairCode   = flag.String("pair-code", "", "配对码;转发给 daemon 作鉴权口令(与手机端 token 一致)")
		checkOnly  = flag.Bool("check-only", false, "只探测并记日志,不启动 daemon(自测用)")
		killPid    = flag.Int("kill", 0, "TerminateProcess 指定 PID(测试守护链路用,GUI 子系统无窗口)")
	)
	flag.Parse()

	exe, err := os.Executable()
	if err != nil {
		exitLog("paguard: os.Executable: %v", err)
	}
	dir := filepath.Dir(exe)

	if *killPid > 0 {
		if err := killProcess(*killPid); err != nil {
			exitLog("paguard: kill %d: %v", *killPid, err)
		}
		log(dir, "killed pid=%d", *killPid)
		return
	}

	if portUp(*probeAddr) {
		log(dir, "port %s up", *probeAddr)
		return
	}
	log(dir, "port %s down", *probeAddr)
	if *checkOnly {
		return
	}

	if err := startDaemon(dir, *daemonAddr, *pairCode); err != nil {
		exitLog("paguard: start daemon: %v", err)
	}
	// 等daemon 完成绑定;成功即退出,daemon 独立存活(paguard 由 WMI
	// 创建、不在计划任务的 job 里,它的子进程同样不受 job 拆除影响)。
	for i := 0; i < 20; i++ {
		time.Sleep(250 * time.Millisecond)
		if portUp(*probeAddr) {
			log(dir, "daemon back up on %s", *probeAddr)
			return
		}
	}
	log(dir, "WARNING: daemon still down after 5s")
}

func portUp(addr string) bool {
	c, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}

func killProcess(pid int) error {
	k32 := syscall.NewLazyDLL("kernel32.dll")
	open := k32.NewProc("OpenProcess")
	term := k32.NewProc("TerminateProcess")
	h, _, e := open.Call(0x0001 /*PROCESS_TERMINATE*/, 0 /*bInheritHandle*/, uintptr(pid))
	if h == 0 {
		return fmt.Errorf("OpenProcess: %v", e)
	}
	defer syscall.CloseHandle(syscall.Handle(h))
	r, _, e := term.Call(h, 1)
	if r == 0 {
		return fmt.Errorf("TerminateProcess: %v", e)
	}
	return nil
}

func startDaemon(dir, addr, pairCode string) error {
	out, err := os.OpenFile(filepath.Join(dir, "daemon.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()

	// 与旧 watchdog.bat 完全一致的启动参数:cwd=daemon 目录,workspace=仓库根。
	args := []string{"-addr", addr, "-workspace", filepath.Clean(filepath.Join(dir, ".."))}
	if pairCode != "" {
		args = append(args, "-pair-code", pairCode)
	}
	cmd := exec.Command(filepath.Join(dir, "pocketagentd.exe"), args...)
	cmd.Dir = dir
	// 【为什么不再注入 CODEX_BIN 默认值】旧实现在环境未设 CODEX_BIN 时注入一条
	// 写死的开发机路径（G:\Claude Code\codex.cmd）：它作为字符串常量进 paguard.exe
	// 随发布包分发（规格 §2 明令禁止），且在用户机器上必然不存在 —— 后果不只是
	// 泄漏，而是**覆盖**掉 daemon 自己的定位链，把"能找到 codex"变成"找不到"。
	// 未设即不注入：cmd.Env 保持 nil 时 exec 继承父进程环境，用户自己的 CODEX_BIN
	// （或 daemon 侧的探测结果）照常生效。
	cmd.Stdout, cmd.Stderr = out, out
	hidewin.Apply(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	log(dir, "watchdog started pocketagentd (paguard, pid=%d)", cmd.Process.Pid)
	return nil
}

func log(dir, format string, args ...any) {
	f, err := os.OpenFile(filepath.Join(dir, "watchdog.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	ts := time.Now().Format("2006/01/02 15:04:05")
	fmt.Fprintf(f, "[%s] %s\n", ts, fmt.Sprintf(format, args...))
}

func exitLog(format string, args ...any) {
	fmt.Fprintln(os.Stderr, fmt.Sprintf(format, args...))
	os.Exit(1)
}
