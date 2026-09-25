//go:build !windows

// Package jobobject 非 Windows 平台无 Job Object:Unix 的进程组/会话语义
// 由各平台原生死亡传递(SIGHUP 传播)处理,daemon 退出收尾走既有优雅退出。
package jobobject

// Assign 非 Windows 无操作,返回 0 表示"无 Job 句柄"(不报错 —— 兜底层
// 在不支持的平台上静默缺席,不阻塞启动)。
func Assign() (uintptr, error) { return 0, nil }
