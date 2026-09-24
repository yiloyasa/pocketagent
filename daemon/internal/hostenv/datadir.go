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
// 保留它的用途是**沿用 + 迁移源**：老用户的数据在那里，默认路径由 migration.go
// 的 ResolveDataDir 把它安全搬进 UserDataDir（校验+原子 rename+失败回滚）；
// 显式 -data 路径则完全绕开迁移逻辑。此处只回答"老目录在哪"。
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
