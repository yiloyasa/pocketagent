package term

// manager_spawn_test.go —— Open 锁重排(spawn 移出 m.mu)的回归钉子(2026-10-09)。
//
// 钉三件事:
//  1. 并发同 id Open:占位防重入,恰一 goroutine 真正 spawn,两调用者拿到
//     同一 Session(旧版靠整段持锁串行化,新版靠 creating 占位票);
//  2. spawn 失败必须回滚占位 —— 占位泄漏会让下一次同 id Open 永远等票
//     (本用例若回归会直接 hang,go test 超时即信号);
//  3. spawn 窗口内到达的 Close 不丢:登记在票上,由 Open 收尾补杀,
//     对齐旧版"Close 在 m.mu 排队、必关掉正在 Open 的会话"的语义。
//
// 时间/调度注入:spawnHook 白盒钩子把第一次 spawn 卡在 pty 创建之前,
// 用 waitCond 等占位建立,不靠裸 sleep 猜调度(个别 sleep 只用于给
// "不该发生第二次 spawn"留出暴露窗口)。

import (
	"sync"
	"testing"
	"time"
)

// TestConcurrentOpenSameSessionSingleSpawn 两 goroutine 同开一个 id:
// 恰一次 spawn,双方拿到同一会话指针。
func TestConcurrentOpenSameSessionSingleSpawn(t *testing.T) {
	m := NewManager()
	dir := t.TempDir()
	shell := DefaultShell()

	var mu sync.Mutex
	entered := 0
	release := make(chan struct{})
	m.spawnHook = func() {
		mu.Lock()
		entered++
		first := entered == 1
		mu.Unlock()
		if first {
			<-release // 只卡第一个真正进入 spawn 的;若出现第二个(回归)则放行
		}
	}

	type result struct {
		s   *Session
		err error
	}
	res1 := make(chan result, 1)
	go func() {
		s, err := m.Open("race-1", dir, shell, 80, 24, nil)
		res1 <- result{s, err}
	}()
	// 等 g1 建立占位并进入 spawn(卡在钩子里)
	if !waitCond(func() bool {
		m.mu.Lock()
		defer m.mu.Unlock()
		_, ok := m.creating["race-1"]
		return ok
	}, 5*time.Second) {
		t.Fatal("第一个 Open 未建立占位")
	}
	if !waitCond(func() bool {
		mu.Lock()
		defer mu.Unlock()
		return entered == 1
	}, 5*time.Second) {
		t.Fatal("第一个 Open 未进入 spawn")
	}

	res2 := make(chan result, 1)
	go func() {
		s, err := m.Open("race-1", dir, shell, 80, 24, nil)
		res2 <- result{s, err}
	}()
	// 给 g2 留出走到等票点的窗口:占位在,它绝不该触发第二次 spawn
	time.Sleep(150 * time.Millisecond)
	mu.Lock()
	if entered != 1 {
		mu.Unlock()
		t.Fatalf("并发同 id Open 触发了第二次 spawn, entered=%d(占位防重入失效)", entered)
	}
	mu.Unlock()

	close(release)
	var r1, r2 result
	select {
	case r1 = <-res1:
	case <-time.After(15 * time.Second):
		t.Fatal("第一个 Open 未返回")
	}
	select {
	case r2 = <-res2:
	case <-time.After(15 * time.Second):
		t.Fatal("等票的第二个 Open 未返回")
	}
	if r1.err != nil || r2.err != nil {
		t.Fatalf("Open 失败: %v / %v", r1.err, r2.err)
	}
	if r1.s != r2.s {
		t.Fatal("两调用者应拿到同一 Session(恰一 spawn,复用成果)")
	}
	mu.Lock()
	defer mu.Unlock()
	if entered != 1 {
		t.Fatalf("全程应恰一次 spawn, entered=%d", entered)
	}
	m.Close("race-1")
}

// TestOpenFailureRollsBackPlaceholder spawn 失败回滚占位:失败后同 id 重开
// 必须能成功(占位若泄漏,这里会永远等票 —— hang 即回归信号)。
func TestOpenFailureRollsBackPlaceholder(t *testing.T) {
	m := NewManager()
	dir := t.TempDir()
	shell := DefaultShell()

	if _, err := m.Open("rollback-1", dir, "definitely-not-a-shell-xyz", 80, 24, nil); err == nil {
		t.Fatal("坏 shell 路径应 spawn 失败")
	}
	m.mu.Lock()
	_, leaked := m.creating["rollback-1"]
	m.mu.Unlock()
	if leaked {
		t.Fatal("spawn 失败后占位未回滚")
	}

	s, err := m.Open("rollback-1", dir, shell, 80, 24, nil)
	if err != nil {
		t.Fatalf("失败回滚后重 Open: %v", err)
	}
	if !s.alive() {
		t.Fatal("重开的会话应存活")
	}
	m.Close("rollback-1")
}

// TestCloseDuringSpawnKillsOnFinish spawn 窗口内到达的 Close 不丢:
// Open 收尾时按票上的 closeRequested 补杀,会话不出现在表里。
func TestCloseDuringSpawnKillsOnFinish(t *testing.T) {
	m := NewManager()
	dir := t.TempDir()
	shell := DefaultShell()

	release := make(chan struct{})
	m.spawnHook = func() { <-release }

	type outcome struct {
		s   *Session
		err error
	}
	done := make(chan outcome, 1)
	go func() {
		s, err := m.Open("closewin-1", dir, shell, 80, 24, nil)
		done <- outcome{s, err}
	}()
	if !waitCond(func() bool {
		m.mu.Lock()
		defer m.mu.Unlock()
		_, ok := m.creating["closewin-1"]
		return ok
	}, 5*time.Second) {
		t.Fatal("占位未建立")
	}

	m.Close("closewin-1") // 窗口内:会话尚未落表,Close 应登记意图而非漏掉
	close(release)

	oc := <-done
	if oc.err != nil {
		t.Fatalf("Open: %v", oc.err)
	}
	if oc.s.alive() {
		t.Fatal("spawn 窗口内的 Close 应在收尾时补杀会话")
	}
	if _, ok := m.Get("closewin-1"); ok {
		t.Fatal("补杀后会话应已出表")
	}
}
