package hostenv

import (
	"os"
	"path/filepath"
	"strings"
)

// 本文件负责「daemon 的运行数据放在哪」——与 locate.go 同属一类职责：
// 位置必须由用户环境派生，而不是写死。

const (
	// legacyDirName 是历史数据目录名（家目录下的点目录）。
	legacyDirName = ".pocketagentd"
	// appDirName 是当前应用在用户数据根下的目录名。
	appDirName = "PocketAgent"
)

// LegacyDataDir 返回历史数据目录（<家目录>/.pocketagentd）。
//
// 保留它的用途是**沿用**，不是兼容读取：老用户的数据就在那里，daemon 继续用它，
// 不搬、不删、不双写。
func LegacyDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return legacyDirName
	}
	return filepath.Join(home, legacyDirName)
}

// UserDataDir 返回当前用户的应用数据目录（<LOCALAPPDATA>/PocketAgent/data）。
//
// 【为什么放 LOCALAPPDATA】它是 Windows 上「当前用户的应用数据」的标准位置：
// 备份、清理、漫游策略都认它，而家目录下的点目录默认隐藏、用户不易找到。
// 非 Windows 语义（无 LOCALAPPDATA）退到 <家目录>/.local/share。
func UserDataDir() string {
	base := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
	if base == "" {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			base = filepath.Join(home, ".local", "share")
		}
	}
	if base == "" {
		return filepath.Join(appDirName, "data")
	}
	return filepath.Join(base, appDirName, "data")
}

// DaemonDataDir 决策 daemon 该用哪个数据目录，并报告是否沿用了历史位置。
//
// 优先级：新位置已存在 → 老位置已存在 → 新位置（全新安装）。
//
// 【为什么不自动搬移老数据】数据目录里有配对的设备 token、自签证书与配对码
// （本机实测 99MB，其中 85MB 是 pi-agent）。自动搬移会同时引入三件危险事：
// daemon 运行中搬、搬一半失败、两边都有数据时以谁为准。项目既有红线是
// 「绝不自动删用户数据」，此处把同一纪律延伸为「绝不自动搬用户数据」——
// 新装用户走新位置（规格 §2），老用户原地不动（零风险），搬迁交给用户显式决定
// （用 -data 指定，或手动移动目录）。
//
// 返回的 legacy 供调用方如实打印当前用的是哪个位置：数据在哪，用户就该知道。
func DaemonDataDir() (dir string, legacy bool) {
	newDir := UserDataDir()
	if isDir(newDir) {
		return newDir, false
	}
	oldDir := LegacyDataDir()
	if isDir(oldDir) {
		return oldDir, true
	}
	return newDir, false
}

func isDir(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}
