// pi_models_singleflight_test.go —— 弹窗治理专项(2026-09-25)回归钉子:
// fetchModels 的单飞去重。背景:手机端连发 ListModels 时,Models() 的 TTL 缓存
// 判空与写回之间无互斥,daemon.log 18:17:52-53 实测一秒拉起 3 个一次性 pi
// 子进程(每次一个 node 启动 + 一次 Kill 收尸)。单飞后一个探测周期最多一个。
package agent

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// 并发 N 个调用共享同一次探测:do 只执行 1 次,所有调用者拿到同一结果。
func TestFetchModelsSingleFlightDedupes(t *testing.T) {
	p := &PiProvider{}
	var calls atomic.Int32
	do := func() []pb.ModelInfo {
		calls.Add(1)
		time.Sleep(50 * time.Millisecond) // 模拟真实探测窗口(子进程冷启动秒级)
		return []pb.ModelInfo{{Id: "default", Provider: "pi"}}
	}

	const n = 8
	var wg sync.WaitGroup
	results := make([][]pb.ModelInfo, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i] = p.fetchModelsSingleFlight(do)
		}(i)
	}
	wg.Wait()

	if got := calls.Load(); got != 1 {
		t.Fatalf("并发 %d 个调用,do 实际执行 %d 次(应为 1 —— 在飞期间必须共享)", n, got)
	}
	for i, r := range results {
		if len(r) != 1 || r[0].Id != "default" {
			t.Fatalf("调用者 %d 拿到的结果与探测不符: %v", i, r)
		}
	}
}

// 探测完成后在飞槽必须清空:下一次(串行)调用重新执行 do,而不是永远吃第一次
// 的旧结果(新鲜度由 Models() 的 TTL 缓存负责,单飞只管"在飞期间去重")。
func TestFetchModelsSingleFlightReleases(t *testing.T) {
	p := &PiProvider{}
	var calls atomic.Int32
	do := func() []pb.ModelInfo {
		calls.Add(1)
		return nil
	}
	p.fetchModelsSingleFlight(do)
	p.fetchModelsSingleFlight(do)
	if got := calls.Load(); got != 2 {
		t.Fatalf("串行两次调用 do 执行 %d 次(应为 2 —— 完成后必须释放在飞槽)", got)
	}
}

// do 失败(返回 nil)也必须释放在飞槽且不 panic:失败路径照常放行后续重试,
// 与 Models() 不缓存空结果的行为保持一致。
func TestFetchModelsSingleFlightFailureReleases(t *testing.T) {
	p := &PiProvider{}
	var calls atomic.Int32
	do := func() []pb.ModelInfo {
		calls.Add(1)
		if calls.Load() == 1 {
			return nil // 第一次探测失败
		}
		return []pb.ModelInfo{{Id: "default", Provider: "pi"}}
	}
	if r := p.fetchModelsSingleFlight(do); r != nil {
		t.Fatalf("失败结果应为 nil: %v", r)
	}
	if r := p.fetchModelsSingleFlight(do); len(r) != 1 {
		t.Fatalf("失败后的重试应拿到新结果: %v", r)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("do 执行 %d 次(应为 2)", got)
	}
}
