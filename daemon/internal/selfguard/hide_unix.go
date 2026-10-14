//go:build unix

package selfguard

import (
	"os/exec"
)

// applyHidden 非 Windows 无控制台窗口概念，直调 Start 即可。
func applyHidden(cmd *exec.Cmd) {
}
