// Package selfguard 让 daemon 单文件运行时自带看门狗。
//
// 【为什么做在进程内】标准分发只有 daemon.exe 一个文件（见 dist/release.ps1），
// 放不下 paguard.exe + install.bat 全家桶。看门狗只能是自己：父进程启动时
// 顺手拉起一个同文件的 -guard-child 子进程，子进程每轮探活回环端口，连续
// 失败即按原参数把父进程拉起来。父子靠 <数据目录>/guard.lock 的独占文件锁
// 互斥：锁得住 = 暂无看门狗在岗，可以拉起；锁不住 = 已有子进程持有，不重复。
//
// 【停机语义】看门狗分不清"用户主动杀"和"崩溃"——要停全套：先建 <数据目录>/noguard
// 空文件（或本次传 -no-guard），再结束两个进程。marker 存在时父不拉、子自退。
package selfguard

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// 文件名全部收敛在这里，调用方只传数据目录。
const (
	// LockName 看门狗在岗锁（子进程持有独占锁直到退出）。
	LockName = "guard.lock"
	// DisableName 停用标记（存在即不拉看门狗，已在跑的子进程自行退出）。
	DisableName = "noguard"
	// LogName 看门狗自己的日志（与 daemon.log 分开，省得互相抢）。
	LogName = "guard.log"
)

// Config 看门狗子进程的全部输入（纯数据，可单测构造）。
type Config struct {
	// Exe 被看护的父进程可执行文件（os.Executable() 原样透传）。
	Exe string
	// Addr 父进程监听地址（形如 0.0.0.0:7246 / :7246 / 127.0.0.1:7246）。
	Addr string
	// DataDir 数据目录（锁/标记/日志落点）。
	DataDir string
	// Workspace 透传给拉起命令的 -workspace（空 = 不传，走 daemon 缺省）。
	Workspace string
}

// ProbeAddr 把监听地址折成子进程探活的回环地址（只取端口）。
// 解析失败回落 127.0.0.1:7246，绝不让看门狗因地址死掉。
func ProbeAddr(addr string) string {
	port := 7246
	if _, portStr, err := net.SplitHostPort(addr); err == nil {
		if p, err := strconv.Atoi(portStr); err == nil && p > 0 && p < 65536 {
			port = p
		}
	}
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
}

// Disabled 停用标记是否存在（存在 = 用户要停全套）。
func Disabled(dataDir string) bool {
	_, err := os.Stat(filepath.Join(dataDir, DisableName))
	return err == nil
}

// ParentArgs 组装拉起父进程的命令行（去掉 -guard-child，带上三件套）。
func ParentArgs(cfg Config) []string {
	args := []string{"-addr", cfg.Addr, "-data", cfg.DataDir}
	if strings.TrimSpace(cfg.Workspace) != "" {
		args = append(args, "-workspace", cfg.Workspace)
	}
	return args
}

// ChildArgs 组装父进程拉起看门狗子进程的命令行。
func ChildArgs(cfg Config) []string {
	args := []string{"-guard-child", "-addr", cfg.Addr, "-data", cfg.DataDir}
	if strings.TrimSpace(cfg.Workspace) != "" {
		args = append(args, "-workspace", cfg.Workspace)
	}
	return args
}

// PortBusyText 端口被占时的人话（net.Listen 失败包一层这个再 Fatal）。
// 不探测占端口的进程（那要拉 netstat 子进程，弹窗红线），给精确自查命令。
func PortBusyText(listenAddr string, listenErr error) string {
	_, portStr, err := net.SplitHostPort(listenAddr)
	port := strings.TrimSpace(portStr)
	if err != nil || port == "" {
		port = listenAddr
	}
	return fmt.Sprintf("监听 %s 失败（端口大概率被别的程序占了）: %v\n"+
		"自查（电脑上执行）: netstat -ano | findstr :%s  看最后一列 PID，再 tasklist /fi \"pid eq 那个PID\" 找程序\n"+
		"出路：关掉占端口的程序，或换个端口启动（-addr 0.0.0.0:PORT，手机 App 里填的端口也要跟着改）",
		listenAddr, listenErr, port)
}

// ---- 看门狗主循环（可注入，单测不碰真实进程/端口）----

// Probes 探活与拉起的两个可注入点。
type Probes struct {
	// Dial 超时拨号，成功返回 nil。
	Dial func(addr string, timeout time.Duration) error
	// Launch 拉起父进程（传完整命令行），返回即认为已交付。
	Launch func(exe string, args []string) error
	// Log 行日志（子进程落 guard.log，单测收内存）。
	Log func(format string, args ...any)
	// Sleep 可中断的等待（单测加速/限轮）。
	Sleep func(d time.Duration)
}

// Loop 跑看门狗：每 interval 探一次 probeAddr，连续 maxFail 次失败即拉起
// 父进程并重置计数；每轮先看停用标记，有则退出（返回 nil 即"我自退了"）。
// maxRounds <= 0 = 无限跑（生产）；>0 只跑 N 轮（单测）。
func Loop(cfg Config, p Probes, interval time.Duration, maxFail, maxRounds int) error {
	if p.Dial == nil || p.Launch == nil || p.Log == nil || p.Sleep == nil {
		return fmt.Errorf("selfguard: 探针未注全，拒绝空转")
	}
	if maxFail <= 0 {
		maxFail = 3
	}
	probe := ProbeAddr(cfg.Addr)
	fail := 0
	for round := 0; maxRounds <= 0 || round < maxRounds; round++ {
		if Disabled(cfg.DataDir) {
			p.Log("停用标记 %s 存在，看门狗自行退出", DisableName)
			return nil
		}
		if err := p.Dial(probe, 5*time.Second); err != nil {
			fail++
			p.Log("探活 %s 第 %d 次失败: %v", probe, fail, err)
			if fail >= maxFail {
				args := ParentArgs(cfg)
				if err := p.Launch(cfg.Exe, args); err != nil {
					p.Log("拉起父进程失败: %v（下一轮重试）", err)
				} else {
					p.Log("已拉起父进程 %s %v", cfg.Exe, args)
				}
				fail = 0
			}
		} else {
			fail = 0
		}
		p.Sleep(interval)
	}
	return nil
}

// NetProbes 生产用探针（真实拨号/真实拉起/文件日志/真实等待）。
func NetProbes(dataDir string, launch func(exe string, args []string) error) Probes {
	return Probes{
		Dial: func(addr string, timeout time.Duration) error {
			c, err := net.DialTimeout("tcp", addr, timeout)
			if err != nil {
				return err
			}
			return c.Close()
		},
		Launch: launch,
		Log: func(format string, args ...any) {
			appendLog(dataDir, fmt.Sprintf(format, args...))
		},
		Sleep: time.Sleep,
	}
}

// appendLog 看门狗日志追加写（超 1MB 整文件轮转一代，写失败就丢，不炸）。
func appendLog(dataDir, line string) {
	path := filepath.Join(dataDir, LogName)
	if fi, err := os.Stat(path); err == nil && fi.Size() > 1<<20 {
		_ = os.Rename(path, path+".1")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = fmt.Fprintf(f, "%s %s\n", time.Now().Format("2006-01-02 15:04:05"), line)
}

// StartChild 拉起看门狗子进程（隐藏窗口，输出丢弃，启动即脱离）。
// 返回 nil 即已交付（子进程死活由它自己的锁保证，不在这里等）。
func StartChild(cfg Config) error {
	if strings.TrimSpace(cfg.Exe) == "" {
		return fmt.Errorf("selfguard: 可执行文件路径为空，无法拉起看门狗")
	}
	cmd := exec.Command(cfg.Exe, ChildArgs(cfg)...)
	applyHidden(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	// 脱离：父进程退出不带走子进程（看门狗的意义就在这里）。
	return cmd.Process.Release()
}
