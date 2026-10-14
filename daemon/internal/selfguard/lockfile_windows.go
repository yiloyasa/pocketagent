//go:build windows

package selfguard

import (
	"os"

	"golang.org/x/sys/windows"
)

// lockHandle 持有锁的文件句柄（关文件即放锁，进程死即内核回收）。
type lockHandle struct {
	f *os.File
}

// tryLock 非阻塞拿独占锁：拿得到返回 handle（调用方持有直到子进程退出），
// 拿不到返回 nil（已有看门狗在岗）。文件不存在即创建。
//
// 【为什么用 LockFileEx 而不用“pid 文件比对”】pid 会复用，比对 pid+启动时间
// 要走 ToolHelp 快照；文件锁由内核在进程退出时强制回收，无复用、无竞态。
func tryLock(path string) (*lockHandle, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	// 低 32 位全 1 = 锁整文件；FAIL_IMMEDIATELY = 被占立即回，不等。
	err = windows.LockFileEx(
		windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0, 1, 0,
		&windows.Overlapped{},
	)
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	return &lockHandle{f: f}, nil
}

// unlock 放锁并关闭（顺序不能反：先解后关）。
func (h *lockHandle) unlock() error {
	if h == nil || h.f == nil {
		return nil
	}
	_ = windows.UnlockFileEx(
		windows.Handle(h.f.Fd()),
		0, 1, 0,
		&windows.Overlapped{},
	)
	return h.f.Close()
}
