package selfguard

import (
	"os/exec"
	"path/filepath"
	"time"
)

// GuardInterval 探活间隔（崩溃后 1 分钟内拉起，与旧 paguard 计划任务同档）。
const GuardInterval = 30 * time.Second

// GuardMaxFail 连续失败几次认定父进程已死。
const GuardMaxFail = 3

// Ensure 父进程启动时调用：停用标记/已有在岗则跳过，否则拉起子进程。
// 返回 (spawned, reason)：spawned=true 已拉起；false tob reason 说明为什么没拉。
// 锁拿不到（目录不可写等）一律 fail open，只给 reason，绝不阻塞 daemon 启动。
func Ensure(cfg Config) (bool, string) {
	if Disabled(cfg.DataDir) {
		return false, "停用标记 noguard 存在，不拉看门狗（删掉该文件即恢复）"
	}
	h, err := tryLock(filepath.Join(cfg.DataDir, LockName))
	if err != nil {
		return false, "看门狗锁拿不到（已有在岗或目录不可写），跳过拉起"
	}
	// 锁拿到了 = 暂无在岗：先放锁再拉（锁是子进程的，父进程不长期持有）。
	_ = h.unlock()
	if err := StartChild(cfg); err != nil {
		return false, "看门狗拉起失败（daemon 照常跑，下次启动重试）"
	}
	return true, "看门狗已拉起（单文件自守护，停用见 noguard 标记）"
}

// RunChild 看门狗子进程主函数（-guard-child 命中即进这里，永不返回到 serve）。
// 先非阻塞拿锁：拿不到 = 已有同行在岗，直接静默退出（双父竞态的收敛）。
func RunChild(cfg Config) {
	h, err := tryLock(filepath.Join(cfg.DataDir, LockName))
	if err != nil || h == nil {
		// 拿不到锁：要么已有在岗，要么目录不可写——前者静默退，后者记一行。
		if err != nil {
			appendLog(cfg.DataDir, "拿锁失败，子进程退出（目录不可写？）")
		}
		return
	}
	defer h.unlock()
	appendLog(cfg.DataDir, "看门狗上岗（看护 "+cfg.Exe+" "+ProbeAddr(cfg.Addr)+"）")
	probes := NetProbes(cfg.DataDir, func(exe string, args []string) error {
		// 重拉的父进程要可见控制台（崩溃恢复用户看得见；隐藏拉起等于“假死”）。
		cmd := exec.Command(exe, args...)
		if err := cmd.Start(); err != nil {
			return err
		}
		return cmd.Process.Release()
	})
	_ = Loop(cfg, probes, GuardInterval, GuardMaxFail, 0)
}
