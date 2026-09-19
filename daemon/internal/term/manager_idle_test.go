package term

import (
	"bytes"
	"context"
	"log"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// P1 会话生命周期回归:空闲回收 / 有 reader 不回收 / 重 Attach 已回收会话。
//
// 回收路径的时间参数全部注入(毫秒级 idle + reapOnce 手动驱动),
// 不真 sleep 10 分钟,也不依赖后台 ticker 的调度时机。
// 白盒构造的 Session 不带真实 pty/cmd(close() 对 nil 有防御)。

// waitCond 每 10ms 轮询 cond 直到成立或超时。
func waitCond(cond func() bool, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return cond()
}

// captureLog 截获标准 log 输出,返回读取函数与恢复函数。
func captureLog() (read func() string, restore func()) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	return buf.String, func() { log.SetOutput(os.Stderr) }
}

// 回收触发:无 reader 且闲置超过 idle → 会话被关闭、移出表、打回收日志。
func TestIdleReclaimWithoutReaders(t *testing.T) {
	readLog, restore := captureLog()
	defer restore()

	m := NewManager()
	m.SetIdleTimeout(80 * time.Millisecond)
	s := &Session{ID: "idle-1", exitCh: make(chan struct{}), idleSince: time.Now()}
	m.mu.Lock()
	m.sessions["idle-1"] = s
	m.mu.Unlock()

	if !waitCond(func() bool {
		m.reapOnce(80 * time.Millisecond)
		_, ok := m.Get("idle-1")
		return !ok
	}, 3*time.Second) {
		t.Fatal("无 reader 闲置会话未被回收出表")
	}
	if !s.closed || s.exitCode != -1 {
		t.Fatalf("回收应关闭会话且 exitCode=-1: closed=%v exitCode=%d", s.closed, s.exitCode)
	}
	if !strings.Contains(readLog(), "[term] 会话空闲回收 id=idle-1") {
		t.Fatalf("缺少空闲回收日志: %q", readLog())
	}
}

// 有 reader 绝不回收;reader 反注册后重新开始计时并可回收。
func TestNoReclaimWhileReaderAttached(t *testing.T) {
	readLog, restore := captureLog()
	defer restore()

	m := NewManager()
	m.SetIdleTimeout(60 * time.Millisecond)
	s := &Session{ID: "busy-1", exitCh: make(chan struct{}), idleSince: time.Now()}
	m.mu.Lock()
	m.sessions["busy-1"] = s
	m.mu.Unlock()

	// 模拟 Attach:订阅输出流
	ch, err := s.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		m.reapOnce(60 * time.Millisecond)
		time.Sleep(20 * time.Millisecond)
	}
	if _, ok := m.Get("busy-1"); !ok {
		t.Fatal("有 reader 的会话被误回收")
	}
	if !s.alive() || s.ReaderCount() != 1 {
		t.Fatalf("会话应仍存活且有 1 个 reader: alive=%v readers=%d", s.alive(), s.ReaderCount())
	}

	// 模拟 Attach 断开:反注册 → 空闲计时重启 → 超时回收
	s.Unsubscribe(ch)
	if s.ReaderCount() != 0 {
		t.Fatalf("反注册后 reader 数应为 0, got %d", s.ReaderCount())
	}
	if !waitCond(func() bool {
		m.reapOnce(60 * time.Millisecond)
		_, ok := m.Get("busy-1")
		return !ok
	}, 3*time.Second) {
		t.Fatal("reader 反注册后超时应回收")
	}
	if !strings.Contains(readLog(), "[term] 会话空闲回收 id=busy-1") {
		t.Fatalf("缺少回收日志: %q", readLog())
	}
}

// 重 Attach 已回收的会话 = 当新会话开(新 Session 指针,旧的已关闭)。
func TestReattachReclaimedSessionGetsNewSession(t *testing.T) {
	m := NewManager()
	m.SetIdleTimeout(80 * time.Millisecond)
	dir := t.TempDir()
	shell, err := exec.LookPath("cmd.exe")
	if err != nil {
		t.Fatalf("LookPath cmd.exe: %v", err)
	}

	s1, err := m.Open("reattach-1", dir, shell, 80, 24, nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	// 不订阅(reader=0),手动驱动回收
	if !waitCond(func() bool {
		m.reapOnce(80 * time.Millisecond)
		_, ok := m.Get("reattach-1")
		return !ok
	}, 5*time.Second) {
		t.Fatal("会话未被空闲回收")
	}
	if s1.alive() {
		t.Fatal("被回收的旧会话应已关闭")
	}

	// 同 session_id 重新 Attach:当新会话开
	s2, err := m.Open("reattach-1", dir, shell, 80, 24, nil)
	if err != nil {
		t.Fatalf("回收后重 Open: %v", err)
	}
	if s2 == s1 {
		t.Fatal("回收后重 Attach 复用了旧 Session 指针,应为新会话")
	}
	if !s2.alive() {
		t.Fatal("新会话应存活")
	}
	m.Close("reattach-1")
}

// 后台回收 goroutine 随 ctx 取消退出,不留常驻 goroutine。
func TestReaperLoopStopsOnContextCancel(t *testing.T) {
	m := NewManager()
	m.SetIdleTimeout(time.Hour) // 保证 ticker 分支不会误触发回收
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 预先取消:reaperLoop 应立即返回

	done := make(chan struct{})
	go func() {
		m.reaperLoop(ctx, time.Hour)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("reaperLoop 未随 ctx 取消退出(goroutine 泄漏)")
	}
}

// shell 已自行退出的死会话由回收器清出表,防 map 积压。
func TestDeadSessionSweptFromTable(t *testing.T) {
	readLog, restore := captureLog()
	defer restore()

	m := NewManager()
	m.SetIdleTimeout(time.Minute) // 不触发空闲回收分支
	s := &Session{ID: "dead-1", exitCh: make(chan struct{})}
	close(s.exitCh) // 模拟 shell 已退出
	m.mu.Lock()
	m.sessions["dead-1"] = s
	m.mu.Unlock()

	m.reapOnce(time.Minute)
	if _, ok := m.Get("dead-1"); ok {
		t.Fatal("已退出会话未被清出表")
	}
	if !strings.Contains(readLog(), "清理已退出会话 id=dead-1") {
		t.Fatalf("缺少清理日志: %q", readLog())
	}
}
