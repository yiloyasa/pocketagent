//go:build !windows

package codexapp

// close_wait_other_test.go —— Close 收尸测试的平台辅助(非 Windows),
// 与 zcodeapp/close_wait_other_test.go 同款。
// 用 signal 0 探活:err != nil(ErrProcessDone/ESRCH)即进程已退出。

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

// waitProcessGone 轮询等待进程退出,超时返回错误。
func waitProcessGone(pid int, timeout time.Duration) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return nil // 进程已不存在
	}
	deadline := time.Now().Add(timeout)
	for {
		if err := p.Signal(syscall.Signal(0)); err != nil {
			return nil // 已退出
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("等待进程 %d 退出超时(%s),疑似只 Kill 未收尾", pid, timeout)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
