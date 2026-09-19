//go:build windows
// +build windows

// ctrl.go:PTY 输入路径上的控制字节翻译与 Ctrl+C 真中断。
//
// ── 为什么手机键栏的 ^C(\x03)无法中断前台程序(2026-09-18 复现结论)──
//
// 探针 cmd/ptyctrlprobe 在本机(Windows 11 26200 + ConPTY + pwsh 7.6.6)
// 逐一实测了下表编码,pwsh 与 cmd.exe 结果完全一致:
//
//	编码                                          结果
//	--------------------------------------------  --------
//	裸 ETX "\x03"                                 无效
//	"\x1b[27;5;67~"(xterm modifyOtherKeys)      无效
//	"\x1b[27;5;67_"                               无效
//	"\x1b[67;5u"(kitty)                         无效
//	先 DECSET "\x1b[?9001h" 再上述序列            无效
//	win32-input-mode 5 参格式                     无效(格式错误)
//	win32-input-mode 6 参 VK 'C'+CTRL(各状态位)  无效
//	win32-input-mode 6 参 VK_CANCEL(=Ctrl+Break) 有效 → CTRL_BREAK_EVENT
//
// 源码依据(microsoft/terminal,src/host/input.cpp HandleGenericKeyEvent):
//
//	Ctrl+C : CTRL_PRESSED && !ALT && bKeyDown && VK=='C' && IsInProcessedInputMode()
//	Break  : 同前提但 VK==VK_CANCEL,无 processed 检查
//
// 即 ConPTY 的 VT 输入通道在这台机器上不产生 CTRL_C_EVENT,唯一能穿透的
// 硬控制台事件是 VK_CANCEL → CTRL_BREAK_EVENT(实测 ping 打印统计块与
// "Control-Break",但 ping.exe 对 Break 的特性是显示统计后继续跑)。
// 附:P/Invoke 实测控制台输入模式 CURMODE=503,含 ENABLE_PROCESSED_INPUT(0x1),
// 排除 processed 位缺失假设。
//
// ── 因此真正的 Ctrl+C 只能走 Win32 广播 ──
//
// daemon 侧 AttachConsole 到 shell 进程的控制台,再
// GenerateConsoleCtrlEvent(CTRL_C_EVENT, 0) 广播给该控制台全部进程
// —— 与真键盘 Ctrl+C 效果一致(ping 被中断出统计并回到提示符,实测
// "Control-C" 回显都相同)。广播失败(shell 已退出等)则回退到实验证实的
// VK_CANCEL 硬通道。
//
// ── 两个用血换来的陷阱(都有实测事故)──
//
//	1.【ignore 标志被子进程继承】SetConsoleCtrlHandler(NULL, TRUE)
//	  (忽略 Ctrl+C)是 per-process 属性,CreateProcess 默认继承 → shell
//	  及其子进程(ping)也会忽略 CTRL_C_EVENT,conhost 广播时跳过它们,
//	  API 返回成功但事件无人响应。实测症状:daemon 首次广播成功(彼时
//	  ignore 尚未设置),此后新开的会话全部失效。
//	  → 修复:daemon 常驻 ignore(见 init),仅在 spawn shell 的毫秒级
//	    窗口内临时解除,让子进程继承"响应 Ctrl+C"。
//
//	2.【迟到的 CTRL_C 投递】GenerateConsoleCtrlEvent 返回不代表事件已
//	  送达完毕;若广播后立即撤销 ignore 且尚未 FreeConsole,迟到的投递
//	  会命中 daemon 自己。实测症状:daemon 无声退出(WER 无崩溃记录、
//	  signal 优雅退出日志也没有)。
//	  → 修复:广播全程保持 ignore,先 FreeConsole 再谈别的;daemon 脱离
//	    控制台后就没有 CTRL_C 事件来源,绝对安全。
//
// ── 透明性 ──
//
// 翻译只针对孤立的 0x03 / 0x1c 控制字节(键盘无法输入这两个字节,粘贴
// 文本也不会含),其余字节原样透传:普通字符、粘贴、方向键等 VT 序列
// 均不受影响(见 ctrl_test.go 的分段用例)。
package term

import (
	"fmt"
	"log"

	"golang.org/x/sys/windows"
)

// win32KeySeq 编码 win32-input-mode 键事件:CSI Vk;Sc;Uc;Kd;Cs;Rc _
// 格式出自 microsoft/terminal InputStateMachineEngine::_GenerateWin32Key。
func win32KeySeq(vk, sc, uc, kd, cs, rc int) []byte {
	return []byte(fmt.Sprintf("\x1b[%d;%d;%d;%d;%d;%d_", vk, sc, uc, kd, cs, rc))
}

// ctrlBreakSeq = Ctrl+Break 键按下:VK_CANCEL(0x03)、扫描码 0x46(70)、
// 无字符、按下、LEFT_CTRL_PRESSED(0x0008)、重复 1 次。
var ctrlBreakSeq = win32KeySeq(3, 70, 0, 1, 8, 1)

// kernel32 里 x/sys/windows 未包装的控制台 API。
var (
	modKernel32               = windows.NewLazySystemDLL("kernel32.dll")
	procAttachConsole         = modKernel32.NewProc("AttachConsole")
	procFreeConsole           = modKernel32.NewProc("FreeConsole")
	procSetConsoleCtrlHandler = modKernel32.NewProc("SetConsoleCtrlHandler")
)

func attachConsole(pid uint32) error {
	r1, _, e1 := procAttachConsole.Call(uintptr(pid))
	if r1 == 0 {
		return fmt.Errorf("AttachConsole(%d): %v", pid, e1)
	}
	return nil
}

func freeConsole() error {
	r1, _, e1 := procFreeConsole.Call()
	if r1 == 0 {
		return fmt.Errorf("FreeConsole: %v", e1)
	}
	return nil
}

// setIgnore 开/关 daemon 对 CTRL_C_EVENT 的忽略。
func setIgnore(on bool) error {
	add := uintptr(0)
	if on {
		add = 1
	}
	r1, _, e1 := procSetConsoleCtrlHandler.Call(0, add)
	if r1 == 0 {
		return fmt.Errorf("SetConsoleCtrlHandler(NULL,%v): %v", on, e1)
	}
	return nil
}

func init() {
	// daemon 常驻忽略 CTRL_C_EVENT:它要反复 AttachConsole 广播给别人,
	// 自己绝不能被顺带投递命中(它由 paguard 以 TerminateProcess 管理
	// 生命周期,不需要 Ctrl+C)。shell 子进程会继承此标志,所以每次
	// spawn shell 前后用 spawnShellIgnoreWindow 临时解除/恢复。
	if err := setIgnore(true); err != nil {
		log.Printf("[ctrlc] 常驻 ignore 设置失败(后续广播可能误伤 daemon): %v", err)
	}
}

// consoleMu 串行化控制台切换:一个进程同一时刻只能 Attach 一个控制台,
// 并发会话的 Ctrl+C 必须排队,否则广播会发到别的会话的控制台。
// spawn shell 的 ignore 窗口也纳入同一把锁,防止与广播交错。
var consoleMu = make(chan struct{}, 1)

// broadcastCtrlC 向 pid 所在控制台的【全部进程】广播真 CTRL_C_EVENT,
// 等价于用户在该终端按下 Ctrl+C。
func broadcastCtrlC(pid uint32) error {
	consoleMu <- struct{}{}
	defer func() { <-consoleMu }()

	// daemon 平时无控制台;脱离是幂等防御(首次广播前 daemon 带着由
	// CREATE_NO_WINDOW 得来的隐藏控制台,必须先脱离)。
	_ = freeConsole()
	if err := attachConsole(pid); err != nil {
		_ = freeConsole()
		return err
	}
	// ignore 常驻开启,广播全程不撤销;发完立即脱离控制台。
	err := windows.GenerateConsoleCtrlEvent(windows.CTRL_C_EVENT, 0)
	if ferr := freeConsole(); ferr != nil {
		log.Printf("[ctrlc] target pid=%d 广播后 FreeConsole 失败: %v", pid, ferr)
	}
	return err
}

// spawnShellIgnoreWindow 在 spawn shell 子进程前后调用:
//   - begin:临时解除 ignore(拿住 consoleMu,防与广播交错),让即将
//     CreateProcess 的子进程继承"响应 Ctrl+C";
//   - 返回的 restore:子进程启动完成后调用,恢复 ignore 并放锁。
//
// 窗口内 daemon 无控制台(启动早期为隐藏控制台,无 CTRL_C 来源;此后
// 均为无控制台状态),不存在被误伤的窗口风险。
func spawnShellIgnoreWindow() (restore func()) {
	consoleMu <- struct{}{}
	if err := setIgnore(false); err != nil {
		log.Printf("[ctrlc] spawn 窗口解除 ignore 失败(子进程可能继承忽略): %v", err)
	}
	return func() {
		if err := setIgnore(true); err != nil {
			log.Printf("[ctrlc] spawn 窗口恢复 ignore 失败: %v", err)
		}
		<-consoleMu
	}
}

// writeInput 是 Session.Write 的实现:对 0x03 / 0x1c 做翻译,其余透传。
func (s *Session) writeInput(data []byte) error {
	actions := translateInput(data)
	for _, a := range actions {
		if len(a.data) > 0 {
			if _, err := s.pty.Write(a.data); err != nil {
				return err
			}
		}
		if a.ctrl {
			if err := s.sendInterrupt(a.isBreak); err != nil {
				return err
			}
		}
	}
	return nil
}

// sendInterrupt 执行一次中断:
//   - Ctrl+C(0x03):优先 Win32 广播真 CTRL_C_EVENT;失败回退
//     win32-input-mode 的 VK_CANCEL(CTRL_BREAK_EVENT,实验证实的唯一
//     VT 硬通道;对多数程序等效终止,对 ping 显示统计)。
//   - Ctrl+Break(0x1c):直接走 VK_CANCEL 序列。
func (s *Session) sendInterrupt(isBreak bool) error {
	if !isBreak {
		if s.cmd != nil && s.cmd.Process != nil {
			if err := broadcastCtrlC(uint32(s.cmd.Process.Pid)); err == nil {
				log.Printf("[ctrlc] 广播成功(session=%s shell pid=%d)", s.ID, s.cmd.Process.Pid)
				return nil
			}
		}
	}
	_, err := s.pty.Write(ctrlBreakSeq)
	return err
}
