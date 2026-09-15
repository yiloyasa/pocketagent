// Package hidewin 统一给 daemon 拉起的子进程隐藏控制台窗口。
//
// 【为什么需要】daemon 由计划任务在后台拉起,没有可见控制台;此时再
// exec 一个控制台程序(cmd/ipconfig/git),Windows 会为它新建一个前台
// 控制台窗口 —— 用户桌面上就"闪现黑窗"(每个 GetHostInfo 探活都弹一次,
// 而 app 每 8 秒探活一次)。CREATE_NO_WINDOW 让子进程拿到一个不可见的
// 控制台,输出照常通过管道返回。
package hidewin

import (
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

// Apply 给 cmd 打上隐藏窗口标记(幂等,重复调用无害)。
func Apply(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= createNoWindow
}
