// ptyctrlprobe:最小复现——不经过 daemon/gRPC,纯 go-pty 直起 shell,
// 跑 ping -t 后依次尝试多种 Ctrl+C 键编码,找出 ConPTY 真正认可的中断序列。
//
// 背景与实验结论(2026-09-18,Windows 11 26200 + ConPTY + pwsh 7.6.6):
//
//	第一轮:a) 裸 \x03、b) \x1b[27;5;67~(xterm modifyOtherKeys)、
//	c) \x1b[27;5;67_、d) \x1b[67;5u(kitty)、e) 先 DECSET ?9001h 再 b)、
//	f/g) 5 参数 win32-input-mode —— 全部无法中断 ping。
//
//	查证 microsoft/terminal 源码:win32-input-mode 真实格式是【6 参数】
//	CSI Vk;Sc;Uc;Kd;Cs;Rc_(Cs=ControlKeyState,LEFT_CTRL=0x0008),
//	解析于 InputStateMachineEngine::_GenerateWin32Key,经
//	InteractDispatch::WriteCtrlKey → conhost HandleGenericKeyEvent
//	(src/host/input.cpp):
//	  Ctrl+C : CTRL_PRESSED && !ALT && bKeyDown && VK=='C' && IsInProcessedInputMode()
//	  Break  : CTRL_PRESSED && !ALT && bKeyDown && VK==VK_CANCEL(无 processed 检查)
//
//	第二轮:h/i(VK_C + LEFT/RIGHT/LR CTRL)、p(Uc='C')、u(CtrlDown+裸ETX)、
//	v(全按键序列)全部失败;m)(VK_CANCEL=3;70;0;1;8;1_)成功触发
//	CTRL_BREAK_EVENT —— ping 打印统计块 + "Control-Break"(但 ping.exe
//	对 Break 的特性是显示统计后继续跑,不退出)。
//
//	第四轮:P/Invoke 实测控制台输入模式 CURMODE=503,含 ENABLE_PROCESSED_INPUT
//	(0x1)——排除 processed 位缺失假设,VK_C+CTRL 依旧无效。
//
//	第五轮(本版):pwsh 与 cmd.exe 对照,判别是 PSReadLine 的问题还是
//	ConPTY 全局行为。
//
// 判据:发送候选后 3 秒内输出出现 ping 统计行("统计"/"statistics"/"Sent =" 等)
// 或 shell 提示符,视为中断成功。
package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	pty "github.com/aymanbagabas/go-pty"
)

// candidate 一种待试的 Ctrl+C 编码;pre 是可选前置序列(如 DECSET 9001h)。
type candidate struct {
	name string
	pre  []byte // 先写(可为空)
	seq  []byte // 后写,即被测序列
}

// 键事件编码器:6 参 win32-input-mode CSI Vk;Sc;Uc;Kd;Cs;Rc_
func win32Key(vk, sc, uc, kd, cs, rc int) []byte {
	return []byte(fmt.Sprintf("\x1b[%d;%d;%d;%d;%d;%d_", vk, sc, uc, kd, cs, rc))
}

// ctrlCSeq 用 win32-input-mode 编码 Ctrl+C 键按下(VK_C=0x43=67,扫描码 0x2E=46,
// 字符 0x03,按下,LEFT_CTRL_PRESSED=0x0008,1 次)
var ctrlCSeq = win32Key(67, 46, 3, 1, 8, 1)

// ctrlBreakSeq 编码 Ctrl+Break(VK_CANCEL=0x03=3,扫描码 0x46=70,无字符,按下,
// LEFT_CTRL_PRESSED,1 次)
var ctrlBreakSeq = win32Key(3, 70, 0, 1, 8, 1)

var candidates = []candidate{
	{name: "裸ETX \\x03", seq: []byte("\x03")},
	{name: "win32 CtrlC 67;46;3;1;8;1_", seq: ctrlCSeq},
	{name: "win32 CtrlBreak 3;70;0;1;8;1_", seq: ctrlBreakSeq},
}

// promptRe 匹配 PowerShell / cmd 提示符
var promptRe = regexp.MustCompile(`PS [A-Za-z]:\\|PS>|[A-Za-z]:\\>`)

// interrupted 判断这段新增输出是否意味着 ping 被中断。
func interrupted(newOut string) bool {
	for _, kw := range []string{"统计", "statistics", "Sent =", "已发送", "Lost =", "已丢失", "^C", "Control-Break", "Minimum"} {
		if strings.Contains(newOut, kw) {
			return true
		}
	}
	return promptRe.MatchString(newOut)
}

func main() {
	for _, shell := range []string{findShell("pwsh.exe"), findShell("cmd.exe")} {
		fmt.Printf("======== shell: %s ========\n", shell)
		testShell(shell)
	}
}

func testShell(shell string) {
	p, err := pty.New()
	if err != nil {
		fmt.Printf("[fatal] 创建 PTY: %v\n", err)
		os.Exit(1)
	}
	defer p.Close()
	_ = p.Resize(100, 40)

	cmd := p.Command(shell)
	cmd.Env = os.Environ()
	cmd.Dir = "C:\\"
	if err := cmd.Start(); err != nil {
		fmt.Printf("[fatal] 启动 shell: %v\n", err)
		os.Exit(1)
	}

	// 输出累积缓冲(带锁),各阶段用偏移量做快照切分
	var mu sync.Mutex
	var buf []byte
	go func() {
		b := make([]byte, 32*1024)
		for {
			n, err := p.Read(b)
			if n > 0 {
				mu.Lock()
				buf = append(buf, b[:n]...)
				mu.Unlock()
			}
			if err != nil {
				return
			}
		}
	}()

	snap := func() int {
		mu.Lock()
		defer mu.Unlock()
		return len(buf)
	}
	outSince := func(mark int) string {
		mu.Lock()
		defer mu.Unlock()
		if mark >= len(buf) {
			return ""
		}
		return string(buf[mark:])
	}

	// 等 shell 就绪,顺便观察 ConPTY 是否主动发 DECSET ?9001h 请求
	time.Sleep(4 * time.Second)
	warm := outSince(0)
	fmt.Printf("[warm] %d 字节,含 ?9001h 请求=%v(末尾 %q)\n",
		len(warm), strings.Contains(warm, "\x1b[?9001h"), tail(warm, 40))

	pingRunning := false
	for _, c := range candidates {
		if !pingRunning {
			writePty(p, []byte("ping -t 127.0.0.1\r\n"))
			time.Sleep(3 * time.Second)
			pingRunning = true
		}
		mark := snap()
		if len(c.pre) > 0 {
			writePty(p, c.pre)
			time.Sleep(200 * time.Millisecond)
		}
		writePty(p, c.seq)
		time.Sleep(3 * time.Second)
		newOut := outSince(mark)
		ok := interrupted(newOut)
		fmt.Printf("[%s] 中断=%v 新增=%d字节 末尾=%q\n", c.name, ok, len(newOut), tail(newOut, 60))
		if ok {
			// 命中时打印完整新增输出,人工确认是"统计块+提示符"而非误报
			fmt.Printf("[%s] 完整输出: %q\n", c.name, clean(newOut, 600))
		}
		pingRunning = !ok
		if ok {
			// 回提示符,让下一候选能重新启动 ping
			writePty(p, []byte("\r\n"))
			time.Sleep(500 * time.Millisecond)
		}
	}

	// 收尾:退出 shell + 兜底杀进程
	writePty(p, []byte("exit\r\n"))
	time.Sleep(1 * time.Second)
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

func writePty(p pty.Pty, b []byte) {
	if _, err := p.Write(b); err != nil {
		fmt.Printf("[warn] write: %v\n", err)
	}
}

func findShell(name string) string {
	if name == "cmd.exe" {
		if cs := os.Getenv("ComSpec"); cs != "" {
			return cs
		}
		return "cmd.exe"
	}
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	if cs := os.Getenv("ComSpec"); cs != "" {
		return cs
	}
	return "cmd.exe"
}

func tail(s string, n int) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\x00", "")
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

// clean 清理输出并截断到 n 字节,便于打印完整证据段。
func clean(s string, n int) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\x00", "")
	if len(s) > n {
		return s[:n] + "...(截断)"
	}
	return s
}

// runtime 引用占位:保持 import 与平台相关代码一致(Windows 上无额外用途则删)
var _ = runtime.GOOS
