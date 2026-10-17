package server

// model_selection_cache_test.go —— lastTurnSelections 的淘汰纪律与
// selectionChangedHook 的并发纪律(clean-daemon-server P1-1b / P1-2)。
//
// 【钉什么】
//  1. 过期快照被淘汰:上一轮久远到超出 TTL → 按"首次观察"处理(不补发迟到的
//     变化通知),且那条过期条目从表里消失。
//  2. 全表老化:没被本线程碰到、但已到期的条目,在别的线程写入时被扫掉。
//  3. 线程删除后条目被清理:DeleteThread 连带丢掉它的选择快照。
//  4. 钩子并发:并发读(每轮流式 goroutine) + 并发写(接线)不得报竞态 ——
//     这条判决只在 `go test -race` 下有意义。
//
// 钩子的语义一个字没动:首次观察不广播、相等不广播、真变化广播一次。并发用例
// 里钩子体只用原子计数,不在测试代码里自造竞态。

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// agedSelectionEntry 造一条"写在 TTL 之外"的快照(直接写表,绕开 noteSelectionChanged)。
func agedSelectionEntry(snap *TurnSnapshot) *turnSelectionEntry {
	return &turnSelectionEntry{snap: snap, at: time.Now().Add(-lastTurnSelectionTTL - time.Hour)}
}

func storedSelectionAt(t *testing.T, threadID string) (time.Time, bool) {
	t.Helper()
	v, ok := lastTurnSelections.Load(threadID)
	if !ok {
		return time.Time{}, false
	}
	e, ok := v.(*turnSelectionEntry)
	if !ok {
		t.Fatalf("表里存的应是 *turnSelectionEntry,得到 %T", v)
	}
	return e.at, true
}

// 1. 上一轮超出 TTL:不再当"上一次"用(不补发变化通知),并且那条过期快照被换新。
func TestNoteSelectionChangedIgnoresAndEvictsExpiredSnapshot(t *testing.T) {
	resetSelectionTracking()
	defer resetSelectionTracking()
	var calls atomic.Int64
	restore := setSelectionChangedHook(func(string, *pb.EffectiveModelSelection, *pb.EffectiveModelSelection) {
		calls.Add(1)
	})
	defer restore()

	const tid = "th-sel-ttl"
	lastTurnSelections.Store(tid, agedSelectionEntry(&TurnSnapshot{ThreadID: tid, AgentID: "pi",
		ModelID: "pi/m1", Source: SelectionSourceAppDefault}))

	noteSelectionChanged(tid, &TurnSnapshot{ThreadID: tid, AgentID: "pi",
		ModelID: "pi/m2", Source: SelectionSourceThreadOverride})

	if n := calls.Load(); n != 0 {
		t.Fatalf("过期快照不得当成\"上一次\"补发变化通知,广播了 %d 次", n)
	}
	at, ok := storedSelectionAt(t, tid)
	if !ok {
		t.Fatal("本轮快照应已写入")
	}
	if time.Since(at) > lastTurnSelectionTTL {
		t.Fatalf("过期快照应被本轮的新快照顶掉,留下的落库时刻却仍是 %v 前", time.Since(at))
	}
}

// 2. 全表老化:别的线程写入时,到期且不会再被碰的条目被扫掉。
func TestNoteSelectionChangedSweepsExpiredEntries(t *testing.T) {
	resetSelectionTracking()
	defer resetSelectionTracking()
	reset := setSelectionChangedHook(nil)
	defer reset()

	lastTurnSelections.Store("th-sel-dormant", agedSelectionEntry(&TurnSnapshot{ThreadID: "th-sel-dormant"}))
	lastTurnSelections.Store("th-sel-live", &turnSelectionEntry{
		snap: &TurnSnapshot{ThreadID: "th-sel-live"},
		at:   time.Now(),
	})

	noteSelectionChanged("th-sel-other", &TurnSnapshot{ThreadID: "th-sel-other"})

	if _, ok := lastTurnSelections.Load("th-sel-dormant"); ok {
		t.Fatal("到期且无人再读的条目必须在写入侧被扫掉(否则按线程数无界增长)")
	}
	if _, ok := lastTurnSelections.Load("th-sel-live"); !ok {
		t.Fatal("未过期条目不得被误删")
	}
	if _, ok := lastTurnSelections.Load("th-sel-other"); !ok {
		t.Fatal("本轮快照应已写入")
	}
}

// 3. 线程删除 → 它的选择快照一并消失(线程没了,没有下一次可比的"上一次")。
func TestDeleteThreadClearsLastTurnSelections(t *testing.T) {
	resetSelectionTracking()
	defer resetSelectionTracking()

	s := newTestServer(t)
	const tid = "th-del-sel"
	if err := s.cfg.Store.Create(&store.Thread{ID: tid, Title: "带选择的会话"}); err != nil {
		t.Fatal(err)
	}
	noteSelectionChanged(tid, &TurnSnapshot{ThreadID: tid, AgentID: "pi", ModelID: "pi/m1"})
	if _, ok := lastTurnSelections.Load(tid); !ok {
		t.Fatal("前置条件:跑过一轮的线程应有选择快照")
	}

	if _, err := s.DeleteThread(context.Background(), &pb.DeleteThreadRequest{ThreadId: tid}); err != nil {
		t.Fatalf("DeleteThread: %v", err)
	}
	if _, ok := lastTurnSelections.Load(tid); ok {
		t.Fatal("会话删除后,它的选择快照必须一并清掉(删得越多留得越多)")
	}
}

// 4. 钩子并发:读侧(每轮流式 goroutine)与写侧(接线/替换)同时进行。
// 只在 -race 下才是判决;非 race 运行时它至少钉住"不 panic、不卡死"。
func TestSelectionChangedHookConcurrentReadWrite(t *testing.T) {
	resetSelectionTracking()
	defer resetSelectionTracking()
	defer setSelectionChangedHook(nil)()

	const (
		readers = 8
		writers = 4
		rounds  = 200
	)
	var fired atomic.Int64
	var wg sync.WaitGroup
	// 读侧:模拟 N 个线程轮流跑 StartTurn(noteSelectionChanged 的真实调用方)。
	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func(r int) {
			defer wg.Done()
			for i := 0; i < rounds; i++ {
				tid := "th-race-" + string(rune('a'+r))
				noteSelectionChanged(tid, &TurnSnapshot{ThreadID: tid, AgentID: "pi", ModelID: "pi/m1"})
				noteSelectionChanged(tid, &TurnSnapshot{ThreadID: tid, AgentID: "pi", ModelID: "pi/m2"})
			}
		}(r)
	}
	// 写侧:未来的真实广播接线就是这种"启动期写一次"的形状(这里反复写以撞上读)。
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < rounds; i++ {
				restore := setSelectionChangedHook(func(string, *pb.EffectiveModelSelection, *pb.EffectiveModelSelection) {
					fired.Add(1)
				})
				restore()
			}
		}()
	}
	wg.Wait()

	// 收尾:并发写结束后,钩子必须还能被正常换掉(证明没留下撕裂的状态)。
	var tail atomic.Int64
	restore := setSelectionChangedHook(func(string, *pb.EffectiveModelSelection, *pb.EffectiveModelSelection) {
		tail.Add(1)
	})
	defer restore()
	noteSelectionChanged("th-race-tail", &TurnSnapshot{ThreadID: "th-race-tail", ModelID: "pi/m1"})
	noteSelectionChanged("th-race-tail", &TurnSnapshot{ThreadID: "th-race-tail", ModelID: "pi/m2"})
	if tail.Load() != 1 {
		t.Fatalf("并发写结束后钩子应照常工作,真变化应广播 1 次,得到 %d", tail.Load())
	}
}
