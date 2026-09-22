package agent

// zcode_lockwait_test.go —— zcode 生命周期锁"有界排队"的回归钉子。
//
// 【钉的是什么】用户真机症状:打开导入的 zcode 会话时**历史一片空白** ——
// daemon.log 实证「拉取导入历史失败(降级为已有内容): zcode 正忙」。根因是
// zcode 用纯 TryLock:低频用户路径(拉历史 / 拉斜杠命令 / 切模型)一旦撞上
// "初始化·探测"就立即失败,而不是等一等。本文件钉住新语义:先排队,拿到就
// 做完;窗口耗尽(持有方真卡死)才如实回忙。
// (忙哨兵的文案与 ErrBusy 分级仍由 zcode_busy_test.go 钉,两者互不重叠。)

import (
	"testing"
	"time"
)

// 排队成功:持锁方在窗口内释放 → 等的人应拿到锁。
func TestZcodeLockLifeWaitsThenAcquires(t *testing.T) {
	p := &ZcodeProvider{}
	p.lifeMu.Lock()
	go func() {
		time.Sleep(200 * time.Millisecond)
		p.lifeMu.Unlock()
	}()
	if !p.lockLife(3 * time.Second) {
		t.Fatal("持锁方在窗口内释放,排队应成功拿到锁")
	}
	p.lifeMu.Unlock() // 拿到的锁要还,否则后续用例受影响
}

// 窗口耗尽:持锁方不放手 → 如实返回 false(调用方回忙哨兵),且确实等过。
func TestZcodeLockLifeGivesUpAfterWindow(t *testing.T) {
	p := &ZcodeProvider{}
	p.lifeMu.Lock()
	defer p.lifeMu.Unlock()
	start := time.Now()
	if p.lockLife(150 * time.Millisecond) {
		t.Fatal("持锁方不放,排队不应成功")
	}
	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("应在窗口内等待过才放弃, 实际 %v", elapsed)
	}
}

// wait<=0 = 纯 TryLock 语义(既有忙用例靠它保住"立即返回",不白等一个窗口)。
func TestZcodeLockLifeZeroWaitIsImmediate(t *testing.T) {
	p := &ZcodeProvider{}
	p.lifeMu.Lock()
	defer p.lifeMu.Unlock()
	start := time.Now()
	if p.lockLife(0) {
		t.Fatal("锁被占时 wait=0 不应成功")
	}
	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Fatalf("wait=0 应立即返回, 实际 %v", elapsed)
	}
}

// 无争用时直接拿到(零开销,不引入行为变化)。
func TestZcodeLockLifeUncontendedAcquires(t *testing.T) {
	p := &ZcodeProvider{}
	if !p.lockLife(time.Second) {
		t.Fatal("无争用应直接拿到锁")
	}
	p.lifeMu.Unlock()
}
