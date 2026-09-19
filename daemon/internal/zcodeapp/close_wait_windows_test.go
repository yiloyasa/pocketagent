//go:build windows

package zcodeapp

// close_wait_windows_test.go —— Close 收尸测试的平台辅助(Windows)。
//
// 【为什么不用 cmd.ProcessState 判断进程退出】cmd.Wait 在 Close 的收尸
// goroutine 里进行,exec.Cmd 的 ProcessState 无锁,测试侧任何读取都会与
// 它构成数据竞争(-race 必报)。这里改用 OS 进程句柄等待:与收尸 goroutine
// 零共享内存,只回答"进程死没死"这一个行为问题。

import (
	"fmt"
	"syscall"
	"time"
)

// waitProcessGone 轮询等待进程退出,超时返回错误。
// OpenProcess 失败按"进程已不存在(已退出)"处理 —— 自己拉起的子进程
// 不存在权限问题,打不开句柄只有已退出一种解释。
func waitProcessGone(pid int, timeout time.Duration) error {
	h, err := syscall.OpenProcess(syscall.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return nil // 进程已消失,正是期望
	}
	defer syscall.CloseHandle(h)
	deadline := time.Now().Add(timeout)
	for {
		event, err := syscall.WaitForSingleObject(h, 100)
		if err != nil {
			return fmt.Errorf("等待进程 %d 退出失败: %w", pid, err)
		}
		if event == syscall.WAIT_OBJECT_0 {
			return nil // 进程已退出
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("等待进程 %d 退出超时(%s),疑似只 Kill 未收尾", pid, timeout)
		}
	}
}
