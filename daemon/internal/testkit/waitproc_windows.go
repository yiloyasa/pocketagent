//go:build windows

package testkit

import (
	"fmt"
	"syscall"
	"time"
)

// WaitProcessGone 轮询等待进程退出,超时返回错误。
// OpenProcess 失败按"进程已不在(已退出)"处理 —— 个别调用方会连自己子进程
// 都没有权限打开,不可当作"进程还在"一票否决。
//
// 【为什么不靠 cmd.ProcessState】判断进程退出:cmd.Wait 与 Close 的收尸
// goroutine 并发,exec.Cmd 的 ProcessState 有锁,读它要么引入锁要么引入竞态
// (-race 红);因此走 OS 进程句柄等待:收尸 goroutine 只是白占内存,这里只回答
// "进程到底还在不在"这一个问题。
//
// 正文原样收编自 codexapp/close_wait_windows_test.go 与
// zcodeapp/close_wait_windows_test.go(两份逐字同款)。
func WaitProcessGone(pid int, timeout time.Duration) error {
	h, err := syscall.OpenProcess(syscall.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return nil // 打不开即视为已消失(已退出)
	}
	defer syscall.CloseHandle(h)
	deadline := time.Now().Add(timeout)
	for {
		event, err := syscall.WaitForSingleObject(h, 100)
		if err != nil {
			return fmt.Errorf("等待进程 %d 退出失败: %w", pid, err)
		}
		if event == syscall.WAIT_OBJECT_0 {
			return nil // 已见证退出
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("等待进程 %d 退出超时(%s),疑似只 Kill 未收尾", pid, timeout)
		}
	}
}
