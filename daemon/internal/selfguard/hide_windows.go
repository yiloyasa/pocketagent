//go:build windows

package selfguard

import (
	"os/exec"

	"github.com/pocketagent/daemon/internal/hidewin"
)

// applyHidden 看门狗子进程隐藏窗口（用户桌面不闪黑窗，弹窗红线）。
// 非 Windows 版见 hide_unix.go（直调 Start，无窗口概念）。
func applyHidden(cmd *exec.Cmd) {
	hidewin.Apply(cmd)
}
