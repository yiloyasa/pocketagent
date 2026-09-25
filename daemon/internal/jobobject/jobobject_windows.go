//go:build windows

// Package jobobject 把 daemon 及其全部子进程绑进一个 Job Object,设
// KILL_ON_JOB_CLOSE + UI 限制 —— daemon 无论怎么死(优雅退出/taskkill /F/
// 崩溃/断电),内核保证把 codex/dsh/pi 子进程一起带走,后台不再残留孤儿;
// 且 Job 内进程弹 UI 被内核直接拒绝。
//
// 【为什么必须做这一层】优雅退出只覆盖"收到信号"一条路:部署脚本 taskkill /F、
// 崩溃、断电都绕过它,而 Windows 进程父子关系不传递死亡——父死后 agent 子进程
// 继续活着(真机实锤:每次热部署都残留 node/codex.exe)。Job Object 是内核级
// 父子绑定。优雅退出(收尸日志、mDNS GOODBYE)与 Job 兜底互补:前者管"体面",
// 后者管"必然"。
//
// 【为什么必须在启动早期、拉起任何子进程之前】Job 绑定只影响"之后启动"的
// 子进程(CreateProcess 时进程加入调用方所属 Job);先绑后启,全部子进程天然
// 在 Job 里,无需逐个再挂。
//
// 【实现】用 golang.org/x/sys/windows 的类型安全封装,不手写内存布局。
package jobobject

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Assign 把当前进程绑进新的 Kill-on-Close Job 并叠加 UI 限制。
// 返回 Job 句柄交调用方持有:句柄活着 Job 就活着;daemon 退出句柄随进程关闭,
// 内核终止全部成员。绑定失败返回错误(调用方记日志但**不阻塞启动**——守护
// 链路降级为"有孤儿可杀",不能因为兜底层失败起不来)。
func Assign() (windows.Handle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, fmt.Errorf("CreateJobObject: %w", err)
	}
	// KILL_ON_JOB_CLOSE:daemon 死 = 全家死。
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		windows.CloseHandle(job)
		return 0, fmt.Errorf("SetInformationJobObject(kill-on-close): %w", err)
	}
	// UI 限制:禁止 Job 内进程读剪贴板/全局原子/改显示设置/关机 —— agent 内核
	// 是后台服务,弹 UI 在内核层被拒。这是 hidewin(进程级)之上的最后一道闸:
	// hidewin 管"不闪黑窗",UI 限制管"想弹也弹不出"。
	uil := windows.JOBOBJECT_BASIC_UI_RESTRICTIONS{
		UIRestrictionsClass: windows.JOB_OBJECT_UILIMIT_READCLIPBOARD |
			windows.JOB_OBJECT_UILIMIT_WRITECLIPBOARD |
			windows.JOB_OBJECT_UILIMIT_GLOBALATOMS |
			windows.JOB_OBJECT_UILIMIT_DISPLAYSETTINGS |
			windows.JOB_OBJECT_UILIMIT_EXITWINDOWS |
			windows.JOB_OBJECT_UILIMIT_HANDLES,
	}
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectBasicUIRestrictions,
		uintptr(unsafe.Pointer(&uil)), uint32(unsafe.Sizeof(uil))); err != nil {
		windows.CloseHandle(job)
		return 0, fmt.Errorf("SetInformationJobObject(ui): %w", err)
	}
	// 把自己绑进去:之后的全部子进程自动入 Job。
	if err := windows.AssignProcessToJobObject(job, windows.CurrentProcess()); err != nil {
		windows.CloseHandle(job)
		return 0, fmt.Errorf("AssignProcessToJobObject: %w", err)
	}
	return job, nil
}
