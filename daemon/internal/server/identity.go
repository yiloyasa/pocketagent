package server

import (
	"log"

	"github.com/pocketagent/daemon/internal/runtime"
)

// hostIDFromInstallation 取本机持久化的 host id（W5；规格 §3 待改造项、§4.1）。
//
// 【为什么失败不致命但必须出声】身份文件读不出来时若直接 Fatalf，用户会遇到
// 「daemon 起不来」而不知道删掉一个文件就能恢复；若静默换个新 id，手机会认为
// 换了台电脑、配对信任被悄悄打断而用户毫不知情。所以取中道：降级为进程内随机
// （服务保持可用），但在日志里点名原因与后果 —— 用户据此决定修文件还是重置身份。
func hostIDFromInstallation(dataDir string) string {
	inst, err := runtime.LoadOrCreateInstallation(dataDir)
	if err != nil {
		log.Printf("[identity] 安装身份不可用，本次运行使用临时 host id（重启后会变，手机会认为换了机器）: %v", err)
		return randomID("host")
	}
	return inst.HostID
}
