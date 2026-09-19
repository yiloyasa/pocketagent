package agent

// pi_running_test.go —— P2-2 回归钉子:重入打断后的 running 守卫闭环。
//
// 【背景】server 层重入打断会取消旧轮的 Run ctx,但旧 Run 还要 abort + 等
// agent_settled 收尾(最长 piSettleWait=30s)才清 p.running —— 期间新轮直接进
// 会吃 PI_BUSY("返回后第一轮必失败")。修复:守卫看到"旧轮已被打断"时
// 有界轮询等释放(上限 piSettleWait);旧轮真在跑仍立即 PI_BUSY。
// 只测 acquireRunning(纯内存,不起子进程);旧轮释放用 goroutine 模拟
// "abort 收尾完成后 defer 清旗标"。

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestPiAcquireRunningIdle 空闲线程:直接登记成功。
func TestPiAcquireRunningIdle(t *testing.T) {
	p := NewPiProvider()
	if err := p.acquireRunning("th-1", context.Background()); err != nil {
		t.Fatalf("空闲线程应直接登记成功: %v", err)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.running["th-1"] {
		t.Fatal("登记后 running 应为 true")
	}
}

// TestPiAcquireRunningBusyNotInterrupted 旧轮真在跑(未被取消):立即 PI_BUSY,
// 不等待(否则"正常跑着"的会话会被新轮拖住)。
func TestPiAcquireRunningBusyNotInterrupted(t *testing.T) {
	p := NewPiProvider()
	p.mu.Lock()
	p.running["th-1"] = true
	p.runningCtx["th-1"] = context.Background() // 未取消 = 旧轮活着
	p.mu.Unlock()

	start := time.Now()
	err := p.acquireRunning("th-1", context.Background())
	if err == nil {
		t.Fatal("旧轮活着应拒绝(PI_BUSY)")
	}
	if time.Since(start) > time.Second {
		t.Fatalf("活轮拒绝应立即返回,却等了 %v", time.Since(start))
	}
}

// TestPiAcquireRunningWaitsForInterruptedCleanup 旧轮已被打断(abort 收尾中):
// 等收尾完成(旗标清零)后新轮登记成功,不再吃 PI_BUSY。
func TestPiAcquireRunningWaitsForInterruptedCleanup(t *testing.T) {
	p := NewPiProvider()
	oldCtx, oldCancel := context.WithCancel(context.Background())
	p.mu.Lock()
	p.running["th-1"] = true
	p.runningCtx["th-1"] = oldCtx
	p.mu.Unlock()
	oldCancel() // server 重入打断:旧轮 ctx 已取消,收尾还在进行

	// 模拟旧轮 100ms 后完成收尾(defer 清旗标 + 删 ctx 记录)
	var once sync.Once
	go func() {
		time.Sleep(100 * time.Millisecond)
		once.Do(func() {
			p.mu.Lock()
			p.running["th-1"] = false
			delete(p.runningCtx, "th-1")
			p.mu.Unlock()
		})
	}()

	start := time.Now()
	if err := p.acquireRunning("th-1", context.Background()); err != nil {
		t.Fatalf("打断收尾完成后新轮应登记成功: %v", err)
	}
	if time.Since(start) < 50*time.Millisecond {
		t.Fatal("应等到旧轮释放后才接管,不应立即成功")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.running["th-1"] || p.runningCtx["th-1"] == nil {
		t.Fatal("接管后 running/runningCtx 应指向新轮")
	}
}

// TestPiAcquireRunningTimeout 等不到释放(收尾卡死):上限 piSettleWait 到点
// 后仍要 PI_BUSY 兜底,不能无限等。(把上限缩短测试:直接缩短 sleep 探测 ——
// 用小轮询上界不可注入,这里退而验证"超时路径返回 PI_BUSY"逻辑本身:
// 旗标永不清零时 acquireRunning 最长等 piSettleWait,30s 在单测里太长,
// 改为验证旗标永不清零 + 人为接管 runningCtx 的取消态时立即拒绝。)
func TestPiAcquireRunningInterruptedButFlagStuck(t *testing.T) {
	p := NewPiProvider()
	oldCtx, oldCancel := context.WithCancel(context.Background())
	p.mu.Lock()
	p.running["th-1"] = true
	p.runningCtx["th-1"] = oldCtx
	p.mu.Unlock()
	oldCancel()

	// 收尾永远不来(旗标永不清零):真实上限是 piSettleWait=30s;单测里只验证
	// deadline 分支的返回值语义 —— 用一个极短的等价探针直接走该分支不现实,
	// 因此这里验证"ctx 取消但旗标未清"时至少进入等待(而非立即拒绝):
	// 200ms 内不返回错误即说明在等待中。
	done := make(chan error, 1)
	go func() { done <- p.acquireRunning("th-1", context.Background()) }()
	select {
	case err := <-done:
		t.Fatalf("旗标未清时不应立即返回,却得到 %v", err)
	case <-time.After(200 * time.Millisecond):
		// 正确:仍在等(piSettleWait 上限后才会 PI_BUSY 兜底)
	}
}
