package agent

import (
	"sync"
	"time"
)

// lifeLockRetryStep 是 lockLifeMu 有界排队的重试间隔。三家(dsh/hermes/zcode)
// 的 lockLife 原实现逐字节相同,收敛到本文件只留一份;25ms 这个值与原三处
// 一致,不改时序。
const lifeLockRetryStep = 25 * time.Millisecond

// lockLifeMu 有界排队获取生命周期锁:先 TryLock(无争用零开销),失败后在 wait
// 窗口内短间隔重试 —— 排队而非立即拒绝(pi/zcode 排队语义,不 busy-reject),
// 把"模型取样/握手刚好人挤人"的瞬时争用吸收成先后等待;窗口耗尽仍拿不到
// (持有方真卡死)才返回 false → 调用方回忙哨兵,保留 TryLock 模式"一个卡死的
// 操作不拖住全线"的底线。
//
// wait <= 0 时退化为纯 TryLock 语义 —— 既有单测靠它保住"立即忙"的断言,
// 不必为了一个生产优化去改测试的语义。
//
// 三家的 lockLife 方法保留为一行薄委托(zcode_lockwait_test.go 等直接调
// p.lockLife),注释与忙哨兵文案仍在各家文件里、不窜用。
func lockLifeMu(lifeMu *sync.Mutex, wait time.Duration) bool {
	if lifeMu.TryLock() {
		return true
	}
	if wait <= 0 {
		return false
	}
	deadline := time.Now().Add(wait)
	for {
		time.Sleep(lifeLockRetryStep)
		if lifeMu.TryLock() {
			return true
		}
		if !time.Now().Before(deadline) {
			return false
		}
	}
}
