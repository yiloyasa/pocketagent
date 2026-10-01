// dsh_probe_singleflight_test.go —— 探测互踩治理(2026-10-01)回归钉子。
//
// 背景(daemon.log 2026-10-01 00:49:11→00:49:42 实证):daemon 启动预热在
// lifeMu 内做 initialize 握手(冷启动 31s),窗口内到达的模型列表探测(取样
// 预算 10s → 排队窗口 ≤10s)窗口耗尽 → dshBusy → "模型列表探测失败(返回
// default 兜底)";并发 ListModels 之间 TTL 判空与写回无互斥,还会各自拉起
// 一次瞬态会话探测互踩(pi.go 弹窗治理专项同款问题)。本组用例钉死修复语义:
//   - 同类探测单飞:N 个并发调用共享同一次探测结果,一个探测周期最多一个瞬态会话;
//   - 握手礼让:撞上在飞握手先等落定(有界)再竞争,把"必然失败的互踩"变成
//     "等到即成功";窗口耗尽照旧 default 兜底(有界底线);
//   - 失败不毒化在飞槽:失败的探测照常放行后续重试。
package agent

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pocketagent/daemon/internal/dshapp"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// ---- 慢速 fake:模拟真实世界的慢握手 / 慢取样(嵌入 fakeDshClient 覆写单方法)----

// slowInitClient Initialize 入口先关 begun(≈beginHandshake 时刻),再睡眠模拟
// 冷启动握手(真机实测 31s,这里注入毫秒级)。
type slowInitClient struct {
	fakeDshClient
	initDelay time.Duration
	begun     chan struct{}
	begunOnce sync.Once
}

func (s *slowInitClient) Initialize(ctx context.Context) (*dshapp.InitializeResult, error) {
	s.begunOnce.Do(func() { close(s.begun) })
	time.Sleep(s.initDelay)
	return s.fakeDshClient.Initialize(ctx)
}

// slowNewClient 拉宽瞬态会话取样窗口,让并发调用稳定落在同一个探测周期内。
type slowNewClient struct {
	fakeDshClient
	newDelay time.Duration
}

func (s *slowNewClient) SessionNew(ctx context.Context, cwd string) (*dshapp.SessionState, error) {
	time.Sleep(s.newDelay)
	return s.fakeDshClient.SessionNew(ctx, cwd)
}

// probeSessionState 探针夹具目录装进 session/new 应答。
func probeSessionState(t *testing.T, sid string) *dshapp.SessionState {
	t.Helper()
	return &dshapp.SessionState{SessionID: sid, ConfigOptions: probeCatalogOpts(t)}
}

// ---- 单飞:注入 do 的直接单元测试(pi_models_singleflight_test.go 同款)----

// TestDshFetchModelsSingleFlightDedupes N 个并发调用共享同一次探测:do 只执行
// 1 次,所有调用者拿到同一结果(含错误)。
func TestDshFetchModelsSingleFlightDedupes(t *testing.T) {
	p := NewDshProvider()
	var calls atomic.Int32
	do := func() ([]pb.ModelInfo, dshapp.ModelCatalog, error) {
		calls.Add(1)
		time.Sleep(50 * time.Millisecond) // 模拟真实探测窗口(瞬态会话秒级)
		return []pb.ModelInfo{{Id: "dsh/x", Provider: "dsh"}}, dshapp.ModelCatalog{}, nil
	}

	const n = 8
	var wg sync.WaitGroup
	results := make([][]pb.ModelInfo, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			m, _, err := p.fetchModelsSingleFlight(do)
			if err != nil {
				t.Errorf("调用者 %d 不应有错误: %v", i, err)
				return
			}
			results[i] = m
		}(i)
	}
	wg.Wait()

	if got := calls.Load(); got != 1 {
		t.Fatalf("并发 %d 个调用,do 实际执行 %d 次(应为 1 —— 在飞期间必须共享)", n, got)
	}
	for i, r := range results {
		if len(r) != 1 || r[0].Id != "dsh/x" {
			t.Fatalf("调用者 %d 拿到的结果与探测不符: %v", i, r)
		}
	}
}

// TestDshFetchModelsSingleFlightReleases 探测完成后在飞槽必须清空:下一次(串行)
// 调用重新执行 do,而不是永远吃第一次的旧结果(新鲜度由 Models() 的 TTL 缓存
// 负责,单飞只管"在飞期间去重")。
func TestDshFetchModelsSingleFlightReleases(t *testing.T) {
	p := NewDshProvider()
	var calls atomic.Int32
	do := func() ([]pb.ModelInfo, dshapp.ModelCatalog, error) {
		calls.Add(1)
		return nil, dshapp.ModelCatalog{}, nil
	}
	if _, _, err := p.fetchModelsSingleFlight(do); err != nil {
		t.Fatalf("不应有错误: %v", err)
	}
	if _, _, err := p.fetchModelsSingleFlight(do); err != nil {
		t.Fatalf("不应有错误: %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("串行两次调用 do 执行 %d 次(应为 2 —— 完成后必须释放在飞槽)", got)
	}
}

// TestDshFetchModelsSingleFlightFailureReleases do 失败也必须释放在飞槽且把错误
// 共享给等待方:失败路径照常放行后续重试,与 Models() 不缓存失败结果的行为一致。
func TestDshFetchModelsSingleFlightFailureReleases(t *testing.T) {
	p := NewDshProvider()
	var calls atomic.Int32
	boom := errors.New("boom")
	do := func() ([]pb.ModelInfo, dshapp.ModelCatalog, error) {
		calls.Add(1)
		if calls.Load() == 1 {
			return nil, dshapp.ModelCatalog{}, boom // 第一次探测失败
		}
		return []pb.ModelInfo{{Id: "dsh/x"}}, dshapp.ModelCatalog{}, nil
	}
	if _, _, err := p.fetchModelsSingleFlight(do); !errors.Is(err, boom) {
		t.Fatalf("失败结果应原样上浮: %v", err)
	}
	if _, _, err := p.fetchModelsSingleFlight(do); err != nil {
		t.Fatalf("失败后的重试应拿到新结果: %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("do 执行 %d 次(应为 2)", got)
	}
}

// ---- 端到端:并发 Models() 共享一次瞬态会话取样 ----

// TestDshModelsConcurrentProbeSingleFlight 并发 N 个 Models()(缓存全空)应共享
// 同一次探测:session/new 只发生 1 次,而非 N 个瞬态会话在 lifeMu 上互踩。
func TestDshModelsConcurrentProbeSingleFlight(t *testing.T) {
	sc := &slowNewClient{
		fakeDshClient: fakeDshClient{caps: probeCaps(), newState: probeSessionState(t, "s-cf")},
		newDelay:      150 * time.Millisecond,
	}
	p := NewDshProvider()
	p.newClient = func() (dshClient, error) { return sc, nil }

	const n = 8
	start := make(chan struct{})
	ready := make(chan struct{}, n)
	var wg sync.WaitGroup
	results := make([][]pb.ModelInfo, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ready <- struct{}{}
			<-start // 全员就位后同时放行,保证竞争落在同一个探测周期
			results[i] = p.Models()
		}(i)
	}
	for i := 0; i < n; i++ {
		<-ready
	}
	close(start)
	wg.Wait()

	snap := sc.snapshot()
	if got := len(snap.newCalls); got != 1 {
		t.Fatalf("并发 %d 个 Models() 应共享一次探测,session/new 实发 %d 次(互踩即 >1): %v", n, got, snap.newCalls)
	}
	for i, r := range results {
		if len(r) != 5 || r[0].Id != "default" {
			t.Fatalf("调用者 %d 应拿到 default + 4 模型: %d 条", i, len(r))
		}
	}
}

// ---- 端到端:探测礼让在飞握手(daemon.log 2026-10-01 回归)----

// TestDshModelsProbeWaitsForInFlightHandshake 核心回归:预热/懒启动的 initialize
// 握手在飞(持 lifeMu,冷启动 > 排队窗口)时,模型列表探测应先礼让等握手落定
// 再取样成功,而不是忙拒回 default 兜底 —— 真机时间线:握手 00:49:11→00:49:42,
// 探测 00:49:31 忙拒。修复后同一时序探测必然成功。
func TestDshModelsProbeWaitsForInFlightHandshake(t *testing.T) {
	oldLockWait, oldHSWait := dshLockWait, dshBusyHandshakeWait
	dshLockWait, dshBusyHandshakeWait = 30*time.Millisecond, 3*time.Second
	t.Cleanup(func() { dshLockWait, dshBusyHandshakeWait = oldLockWait, oldHSWait })

	slow := &slowInitClient{
		fakeDshClient: fakeDshClient{caps: probeCaps(), newState: probeSessionState(t, "s-hs")},
		initDelay:     300 * time.Millisecond, // 冷启动握手 > 探测排队窗口(30ms)
		begun:         make(chan struct{}),
	}
	p := NewDshProvider()
	p.newClient = func() (dshClient, error) { return slow, nil }
	p.client = slow // 预置客户端:探测路径直接走握手+取样,不走 startClient

	holderDone := make(chan error, 1)
	go func() {
		// 预热同款:withClientCtx 在锁内做 initialize 握手(慢)。
		holderDone <- p.withClientCtx(context.Background(), 0, func(context.Context, dshClient) error { return nil })
	}()
	<-slow.begun // 握手已在飞(持锁)

	ms := p.Models() // 应礼让握手落定后取样成功
	if len(ms) != 5 || ms[0].Id != "default" {
		t.Fatalf("撞上在飞握手应礼让等待后探测成功(default+4),实得 %d 条: %+v", len(ms), ms)
	}
	if err := <-holderDone; err != nil {
		t.Fatalf("持锁方应正常完成: %v", err)
	}
}

// TestDshProbeGivesUpWhenHandshakeWaitExpires 有界底线:礼让窗口耗尽(握手仍未
// 落定)必须照旧回 default 兜底,绝不无限等待拖住调用方("一个卡死的操作不拖住
// 全线"的纪律在礼让路径上依然成立)。
func TestDshProbeGivesUpWhenHandshakeWaitExpires(t *testing.T) {
	oldLockWait, oldHSWait := dshLockWait, dshBusyHandshakeWait
	dshLockWait, dshBusyHandshakeWait = 20*time.Millisecond, 40*time.Millisecond
	t.Cleanup(func() { dshLockWait, dshBusyHandshakeWait = oldLockWait, oldHSWait })

	slow := &slowInitClient{
		fakeDshClient: fakeDshClient{caps: probeCaps(), newState: probeSessionState(t, "s-to")},
		initDelay:     400 * time.Millisecond, // 握手远超礼让窗口(40ms)
		begun:         make(chan struct{}),
	}
	p := NewDshProvider()
	p.newClient = func() (dshClient, error) { return slow, nil }
	p.client = slow

	holderDone := make(chan error, 1)
	go func() {
		holderDone <- p.withClientCtx(context.Background(), 0, func(context.Context, dshClient) error { return nil })
	}()
	<-slow.begun

	ms := p.Models()
	if len(ms) != 1 || ms[0].Id != "default" {
		t.Fatalf("礼让窗口耗尽应回 default 兜底(有界),实得 %d 条: %+v", len(ms), ms)
	}
	if snap := slow.snapshot(); len(snap.newCalls) != 0 {
		t.Fatalf("礼让失败不得发起取样会话: %v", snap.newCalls)
	}
	if err := <-holderDone; err != nil {
		t.Fatalf("持锁方应正常完成: %v", err)
	}
}

// ---- 失败不毒化 + 目录取样单飞 ----

// TestDshModelsProbeFailureSharedThenRecovers 并发 Models() 共享同一次失败探测
// (只有一个探测周期、全部回 default 兜底);失败后在飞槽照常放行,下一次调用
// 重新探测成功。
//
// 【为何要拉宽取样窗口(flake 根治)】本用例曾不设 newDelay:失败探测在微秒级
// 完成,close(start) 放行的 4 个 goroutine 在全量并发下(其他包的测试二进制、
// 真实 dsh/codex 子进程在抢 CPU/握手窗)后到的调用者会错过在飞槽、各自再开
// 一个探测周期 —— session/new 计数断言随之偶发炸(串扰)。与上方
// TestDshModelsConcurrentProbeSingleFlight 同款治理:150ms 慢窗口让全部调用者
// 稳定落在同一个探测周期内,被测语义(单飞共享失败结果)不变。
func TestDshModelsProbeFailureSharedThenRecovers(t *testing.T) {
	sc := &slowNewClient{
		fakeDshClient: fakeDshClient{
			caps:     probeCaps(),
			newState: probeSessionState(t, "s-fail"),
			newErr:   &dshapp.RPCError{Method: "session/new", Code: -32603, Message: "boom"},
		},
		newDelay: 150 * time.Millisecond, // 拉宽失败探测窗口:4 个调用者必须共享同一周期
	}
	p := NewDshProvider()
	p.newClient = func() (dshClient, error) { return sc, nil }

	const n = 4
	start := make(chan struct{})
	ready := make(chan struct{}, n)
	var wg sync.WaitGroup
	results := make([][]pb.ModelInfo, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ready <- struct{}{}
			<-start
			results[i] = p.Models()
		}(i)
	}
	for i := 0; i < n; i++ {
		<-ready
	}
	close(start)
	wg.Wait()

	if got := len(sc.snapshot().newCalls); got != 1 {
		t.Fatalf("并发 %d 个调用应共享一次失败探测(周期内 session/new=1),实得 %d", n, got)
	}
	for i, r := range results {
		if len(r) != 1 || r[0].Id != "default" {
			t.Fatalf("调用者 %d 失败期应拿 default 兜底: %+v", i, r)
		}
	}

	// 恢复:失败不毒化在飞槽,下一次调用重新探测成功。
	sc.mu.Lock()
	sc.newErr = nil
	sc.mu.Unlock()
	if ms := p.Models(); len(ms) != 5 {
		t.Fatalf("恢复后应重新探测成功(default+4),实得 %d 条", len(ms))
	}
	if got := len(sc.snapshot().newCalls); got != 2 {
		t.Fatalf("恢复后应新增一个探测周期(累计 session/new=2),实得 %d", got)
	}
}

// TestDshCatalogSampleConcurrentSingleFlight 原生目录取样同样单飞:设置页并发
// 打开/写入校验时共享同一次瞬态会话取样(config 读取在治理范围内)。
func TestDshCatalogSampleConcurrentSingleFlight(t *testing.T) {
	sc := &slowNewClient{
		fakeDshClient: fakeDshClient{caps: probeCaps(), newState: probeSessionState(t, "s-cat")},
		newDelay:      150 * time.Millisecond,
	}
	p := NewDshProvider()
	p.newClient = func() (dshClient, error) { return sc, nil }

	const n = 6
	start := make(chan struct{})
	ready := make(chan struct{}, n)
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ready <- struct{}{}
			<-start
			_, errs[i] = p.ConfigOptionsCatalog(context.Background(), nil)
		}(i)
	}
	for i := 0; i < n; i++ {
		<-ready
	}
	close(start)
	wg.Wait()

	if got := len(sc.snapshot().newCalls); got != 1 {
		t.Fatalf("并发 %d 个目录取样应共享一次探测,session/new 实发 %d 次: %v", n, got, sc.snapshot().newCalls)
	}
	for i, err := range errs {
		if err != nil {
			t.Fatalf("调用者 %d 不应有错误: %v", i, err)
		}
	}
}
