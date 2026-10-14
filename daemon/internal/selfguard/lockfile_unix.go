//go:build unix

package selfguard

import (
	"os"

	"golang.org/x/sys/unix"
)

// lockHandle 持有锁的文件句柄（关文件即放锁，进程死即内核回收）。
type lockHandle struct {
	f *os.File
}

// tryLock 非阻塞拿独占锁：拿得到返回 handle，拿不到返回 nil。
// 与 Windows 版同语义（见 lockfile_windows.go 注释），供非 Windows 构建
// 与单测交叉验证；生产主力是 Windows。
func tryLock(path string) (*lockHandle, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = f.Close()
		return nil, err
	}
	return &lockHandle{f: f}, nil
}

// unlock 放锁并关闭。
func (h *lockHandle) unlock() error {
	if h == nil || h.f == nil {
		return nil
	}
	_ = unix.Flock(int(h.f.Fd()), unix.LOCK_UN)
	return h.f.Close()
}
