// collab_delegate_parent_test.go —— T-S1Pa 回归钉子:委派链路的可选父链
// (DelegateOptions.ParentThreadID / DelegateSubTaskRequest.parent_thread_id=6)
// 在引擎侧的落库语义。
//
// 钉三件事:
//  1. 非空父 id → 子线程 ParentThreadID 落库(与 effort 覆盖同一次
//     Store.Update,互不挤掉);
//  2. 空父 id(缺省)→ 子线程父链为空、委派照常成功 —— v1 语义:pi 侧经
//     pi-mcp-adapter 的委派恒走此路径(诚实变暗,卡不亮不误报);
//  3. 与 codex 存根链(codex_subagent_sync.go)正交:委派子线程
//     CodexThreadID 为空,存根链的幂等闸不会改写本处落库的归属;存根链
//     登记全新 codex 子线程时与委派链并存互不干扰。
package server

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pocketagent/daemon/internal/agent"
	"github.com/pocketagent/daemon/internal/collabmcp"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// TestDelegateParentThreadIDPersisted 非空父 id:子线程 ParentThreadID 落库,
// 既有委派语义(成功轮、摘要返回)不受影响;effort 覆盖与父链同一次
// Update 共存。
func TestDelegateParentThreadIDPersisted(t *testing.T) {
	srv, eng := newCollabTestEngine(t, map[string]agent.Provider{"echo": agent.NewEcho()})
	// 父线程先入本地库:ParentThreadID 存的是父的 daemon 本地 id(现实形态;
	// 引擎不做父 id 存在性校验,这里用真实在库的父钉正常路径)。
	if err := srv.cfg.Store.Create(&store.Thread{ID: "th-parent-1", Title: "父会话"}); err != nil {
		t.Fatalf("建父线程失败: %v", err)
	}

	childID, summary, err := eng.Delegate(context.Background(), "echo", "帮我看日志", collabmcp.DelegateOptions{
		ParentThreadID: "th-parent-1",
		Effort:         "high",
	})
	if err != nil {
		t.Fatalf("委派失败: %v", err)
	}
	if summary == "" {
		t.Fatal("摘要为空")
	}
	got, ok := srv.cfg.Store.Get(childID)
	if !ok {
		t.Fatal("子线程没有落库")
	}
	if got.ParentThreadID != "th-parent-1" {
		t.Fatalf("子线程父链不符: got %q, want th-parent-1", got.ParentThreadID)
	}
	if got.Effort != "high" {
		t.Fatalf("父链与 effort 应同一次 Update 共存: effort=%q", got.Effort)
	}
}

// TestDelegateEmptyParentThreadIDLeavesChainEmpty 缺省空父 id:委派照常成功、
// 子线程父链为空(不炸、不补默认、不误报)。
func TestDelegateEmptyParentThreadIDLeavesChainEmpty(t *testing.T) {
	srv, eng := newCollabTestEngine(t, map[string]agent.Provider{"echo": agent.NewEcho()})
	childID, summary, err := eng.Delegate(context.Background(), "echo", "无父链任务", collabmcp.DelegateOptions{})
	if err != nil || summary == "" {
		t.Fatalf("空父链委派应照常成功: err=%v summary=%q", err, summary)
	}
	got, ok := srv.cfg.Store.Get(childID)
	if !ok {
		t.Fatal("子线程没有落库")
	}
	if got.ParentThreadID != "" {
		t.Fatalf("缺省时子线程父链应为空: %q", got.ParentThreadID)
	}
}

// TestDelegateParentChainOrthogonalToCodexStubChain 与 codex 存根链正交:
// 委派子线程(CodexThreadID 空)不会被 thread/started 存根链改写父链归属;
// 存根链登记全新 codex 子线程照常工作,与委派链并存互不干扰。
func TestDelegateParentChainOrthogonalToCodexStubChain(t *testing.T) {
	srv, eng := newCollabTestEngine(t, map[string]agent.Provider{"echo": agent.NewEcho()})
	// 父会话是接入过的 codex 线程(存根链找父按 CodexThreadID)。
	if err := srv.cfg.Store.Create(&store.Thread{
		ID: "th-parent-1", Title: "父会话", AgentID: "codex", CodexThreadID: "cx-parent",
	}); err != nil {
		t.Fatalf("建父线程失败: %v", err)
	}

	childID, _, err := eng.Delegate(context.Background(), "echo", "带父链任务", collabmcp.DelegateOptions{
		ParentThreadID: "th-parent-1",
	})
	if err != nil {
		t.Fatalf("委派失败: %v", err)
	}

	// 结构正交:委派子线程建线程时不带 codex 会话身份,存根链按
	// CodexThreadID 匹配天然碰不到它。
	child, ok := srv.cfg.Store.Get(childID)
	if !ok {
		t.Fatal("子线程没有落库")
	}
	if child.CodexThreadID != "" {
		t.Fatalf("委派子线程不应有 CodexThreadID: %q", child.CodexThreadID)
	}

	// 攻击面 1:thread/started 通知冒出与委派子线程同 id 的线程 —— 幂等闸
	// ("已在库一律不动")必须挡住,委派子线程已落库的父链归属不被改写。
	srv.maybeRegisterCodexSubagentStub(childID, []byte(
		`{"thread":{"id":"`+childID+`","parentThreadId":"cx-parent","agentNickname":"冒名"}}`))
	got, _ := srv.cfg.Store.Get(childID)
	if got.ParentThreadID != "th-parent-1" {
		t.Fatalf("存根链改写了委派子线程的父链: %q", got.ParentThreadID)
	}

	// 攻击面 2(并存证明):正常存根登记(全新 codex 子线程)照常落库,
	// 委派子线程不受波及。
	srv.maybeRegisterCodexSubagentStub("cx-child-new", []byte(
		`{"thread":{"id":"cx-child-new","parentThreadId":"cx-parent","agentNickname":"审查员"}}`))
	stub, ok := srv.cfg.Store.Get("cx-child-new")
	if !ok || stub.ParentThreadID != "th-parent-1" || stub.CodexThreadID != "cx-child-new" {
		t.Fatalf("存根链应照常登记全新 codex 子线程: ok=%v stub=%+v", ok, stub)
	}
	got, _ = srv.cfg.Store.Get(childID)
	if got.ParentThreadID != "th-parent-1" || got.Title != "带父链任务" {
		t.Fatalf("委派子线程被存根链波及: parent=%q title=%q", got.ParentThreadID, got.Title)
	}
}

// ---- T-S1Pb 回归钉子:有界并发(取代旧版 depth>0 单飞)+ 父链宿主自动推断 ----

// blockingDelegateProvider 测试专用阻塞 provider:进入 Run 先发一个 entered 信号
// (测试据此确认已真正占用一个并发名额、进入在飞态),再阻塞到 release 关闭。
// 用它可确定性地构造"多个委派同时在飞"的场景,验证有界并发而非旧版单飞。
type blockingDelegateProvider struct {
	release chan struct{}
	entered chan struct{} // 缓冲够用即可,每次进入 Run 发一个信号
}

func newBlockingDelegateProvider() *blockingDelegateProvider {
	return &blockingDelegateProvider{release: make(chan struct{}), entered: make(chan struct{}, 16)}
}

func (p *blockingDelegateProvider) ID() string          { return "block" }
func (p *blockingDelegateProvider) DisplayName() string { return "阻塞(测试)" }
func (p *blockingDelegateProvider) Models() []pb.ModelInfo {
	return []pb.ModelInfo{{Id: "default", Provider: "block", Available: true}}
}
func (p *blockingDelegateProvider) Run(_ context.Context, _ agent.TurnRequest, _ agent.Sink, _ agent.ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
	p.entered <- struct{}{}
	<-p.release
	return []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "完成"}}}}, &pb.Usage{}, nil
}

// TestDelegateParallelSucceeds 连续并行发起 3 个委派,断言全部成功(不再被旧版
// "depth>0 单飞"闸拒绝),且各自经 inferred 父链挂到唯一父轮次 th-parent-1。
// 父轮次持 activeTurns 租约(模拟协调器正在跑的轮次),空 parent_thread_id 入参
// 由引擎经 soleParentTurn 推断出父;并断言 3 个委派确实在飞(在飞计数=3),
// 证明确实是并行而非退化成串行。
func TestDelegateParallelSucceeds(t *testing.T) {
	blk := newBlockingDelegateProvider()
	srv, eng := newCollabTestEngine(t, map[string]agent.Provider{"block": blk})
	ce := eng.(*collabEngine)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pLease, err := srv.activeTurns.acquire(ctx, "th-parent-1", cancel)
	if err != nil {
		t.Fatalf("父租约获取失败: %v", err)
	}
	defer srv.activeTurns.Clear("th-parent-1", pLease)

	const n = 3
	var wg sync.WaitGroup
	var mu sync.Mutex
	var fails []string
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// 空入参 → 引擎应推断出父 th-parent-1(soleParentTurn 排除子线程后唯一候选)
			pid := ce.resolveParentThreadID("")
			if pid != "th-parent-1" {
				mu.Lock()
				fails = append(fails, fmt.Sprintf("iter %d: pid=%q", i, pid))
				mu.Unlock()
				return
			}
			childID, _, derr := eng.Delegate(context.Background(), "block",
				fmt.Sprintf("并行子任务 %d", i), collabmcp.DelegateOptions{ParentThreadID: pid})
			if derr != nil {
				mu.Lock()
				fails = append(fails, fmt.Sprintf("iter %d: err=%v", i, derr))
				mu.Unlock()
				return
			}
			got, ok := srv.cfg.Store.Get(childID)
			if !ok || got.ParentThreadID != "th-parent-1" {
				mu.Lock()
				fails = append(fails, fmt.Sprintf("iter %d: 父链未挂接(parent=%q ok=%v)", i, parentOf(got, ok), ok))
				mu.Unlock()
			}
		}(i)
	}
	// 等到 n 个都真正进入 Run(各占一个并发名额)→ 证明是并行而非旧版单飞
	for i := 0; i < n; i++ {
		select {
		case <-blk.entered:
		case <-time.After(5 * time.Second):
			t.Fatalf("超时:只有 %d 个委派进入运行,未能并行", i)
		}
	}
	// 此刻在飞数应等于 n(有界并发允许同时 n 个,旧版只能 1 个)
	ce.mu.Lock()
	inflightNow := ce.inflight
	ce.mu.Unlock()
	if inflightNow != n {
		t.Fatalf("并行在飞数应为 %d, got %d(旧版单飞只会是 1)", n, inflightNow)
	}
	close(blk.release) // 放行
	wg.Wait()
	if len(fails) > 0 {
		t.Fatalf("并行委派失败: %v", fails)
	}
}

// TestDelegateParallelLimitRejectsWhenExceeded 把上限临时调到 1,并行发 2 个,
// 断言恰好 1 成功、1 被"并行上限"错误拒绝(超限如实报错,绝不伪造成功/排队)。
func TestDelegateParallelLimitRejectsWhenExceeded(t *testing.T) {
	blk := newBlockingDelegateProvider()
	_, eng := newCollabTestEngine(t, map[string]agent.Provider{"block": blk})
	// 保存并临时把上限调到 1(默认 3);验证"超限即拒绝、不排队"
	oldG, oldD := maxCollabParallel, maxCollabParallelPerDevice
	maxCollabParallel, maxCollabParallelPerDevice = 1, 1
	defer func() { maxCollabParallel, maxCollabParallelPerDevice = oldG, oldD }()

	var wg sync.WaitGroup
	var mu sync.Mutex
	var success, rejected int
	var rejErr error
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, derr := eng.Delegate(context.Background(), "block", "任务", collabmcp.DelegateOptions{})
			mu.Lock()
			if derr != nil {
				rejected++
				rejErr = derr
			} else {
				success++
			}
			mu.Unlock()
		}()
	}
	// 等到一个真正进入运行(占满上限 1)
	select {
	case <-blk.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("超时:没有任何委派进入运行")
	}
	close(blk.release)
	wg.Wait()
	if success != 1 || rejected != 1 {
		t.Fatalf("上限=1 时应恰好 1 成功 1 拒绝: success=%d rejected=%d", success, rejected)
	}
	if rejErr == nil || !strings.Contains(rejErr.Error(), "并行上限") {
		t.Fatalf("超限应返回并行上限错误, got %v", rejErr)
	}
}

// TestDelegateDepthLockRejectsChildSpawningChild 深度锁 1(子不生子):先做一次
// 普通委派,其创建的子线程 id 会登记进 childThreads;再用该子线程 id 充当
// ParentThreadID 发起委派 → 引擎判定"子在生子" → 以 errDelegateDepthRejected 拒绝。
func TestDelegateDepthLockRejectsChildSpawningChild(t *testing.T) {
	srv, eng := newCollabTestEngine(t, map[string]agent.Provider{"echo": agent.NewEcho()})
	// 首委派:子线程父链应为空(诚实变暗)
	childID, _, err := eng.Delegate(context.Background(), "echo", "父委派", collabmcp.DelegateOptions{})
	if err != nil {
		t.Fatalf("首委派应成功: %v", err)
	}
	got, ok := srv.cfg.Store.Get(childID)
	if !ok {
		t.Fatal("首委派子线程未落库")
	}
	if got.ParentThreadID != "" {
		t.Fatalf("首委派子线程父链应空: %q", got.ParentThreadID)
	}
	// 用首委派产生的子线程 id 充当"父链"再委派 → 子在生子 → 深度锁拒绝
	_, _, err2 := eng.Delegate(context.Background(), "echo", "嵌套委派", collabmcp.DelegateOptions{ParentThreadID: childID})
	if !errors.Is(err2, errDelegateDepthRejected) {
		t.Fatalf("子生子应被深度锁拒绝: got err=%v", err2)
	}
}

// TestSoleParentTurn 父链推断(soleParentTurn)四态单测:
//   - 0 个活跃 → false
//   - 1 个活跃(非子线程)→ 命中该父
//   - 1 个活跃但它是 childThreads 成员 → false
//   - 2 个活跃(1 父 1 子)→ 命中父(排除子后唯一候选)
func TestSoleParentTurn(t *testing.T) {
	srv, eng := newCollabTestEngine(t, map[string]agent.Provider{"echo": agent.NewEcho()})
	ce := eng.(*collabEngine)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 0 个活跃 → 推断不出
	if id, ok := ce.soleParentTurn(); ok {
		t.Fatalf("0 活跃应推断不出父: got %q", id)
	}

	// 1 个活跃(非子线程)→ 命中该父
	pLease, err := srv.activeTurns.acquire(ctx, "th-parent-1", cancel)
	if err != nil {
		t.Fatalf("父租约获取失败: %v", err)
	}
	if id, ok := ce.soleParentTurn(); !ok || id != "th-parent-1" {
		t.Fatalf("1 个非子活跃应命中 th-parent-1: ok=%v id=%q", ok, id)
	}

	// 2 个活跃(1 父 1 子):子线程占一个活跃租约且登记在 childThreads → 排除后命中父
	cLease, err := srv.activeTurns.acquire(ctx, "th-child-1", cancel)
	if err != nil {
		t.Fatalf("子租约获取失败: %v", err)
	}
	ce.mu.Lock()
	ce.childThreads["th-child-1"] = true
	ce.mu.Unlock()
	if id, ok := ce.soleParentTurn(); !ok || id != "th-parent-1" {
		t.Fatalf("1父1子活跃应命中父: ok=%v id=%q", ok, id)
	}
	srv.activeTurns.Clear("th-child-1", cLease)
	ce.mu.Lock()
	delete(ce.childThreads, "th-child-1")
	ce.mu.Unlock()

	// 收尾先清父租约:下面要构造"只有子线程在飞"的孤立场景。
	srv.activeTurns.Clear("th-parent-1", pLease)

	// 1 个活跃但它本身就是 childThreads 成员 → 推断不出(false)
	cLease2, err := srv.activeTurns.acquire(ctx, "th-child-2", cancel)
	if err != nil {
		t.Fatalf("子租约2获取失败: %v", err)
	}
	ce.mu.Lock()
	ce.childThreads["th-child-2"] = true
	ce.mu.Unlock()
	if id, ok := ce.soleParentTurn(); ok {
		t.Fatalf("活跃者若为子线程应推断不出: got %q", id)
	}
	srv.activeTurns.Clear("th-child-2", cLease2)
	ce.mu.Lock()
	delete(ce.childThreads, "th-child-2")
	ce.mu.Unlock()
}

// TestResolveParentThreadIDInferredWhenEmpty 验证 agent_delegate 入参
// parent_thread_id 为空时,引擎用推断值(resolveParentThreadID 组合 soleParentTurn
// + 既有 DelegateOptions 链路):无活跃轮次→留空(诚实不伪造);父轮次持租约→
// 推断出父;显式给了→原样透传(不另造路径)。
func TestResolveParentThreadIDInferredWhenEmpty(t *testing.T) {
	srv, eng := newCollabTestEngine(t, map[string]agent.Provider{"echo": agent.NewEcho()})
	ce := eng.(*collabEngine)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 无活跃 → 空入参返回空(诚实留空,不猜)
	if got := ce.resolveParentThreadID(""); got != "" {
		t.Fatalf("无活跃时空入参应留空: got %q", got)
	}
	// 显式给了 → 原样返回(走既有 DelegateOptions 链路)
	if got := ce.resolveParentThreadID("th-explicit"); got != "th-explicit" {
		t.Fatalf("显式父链应原样返回: got %q", got)
	}
	// 父轮次持租约 → 空入参推断出父
	pLease, err := srv.activeTurns.acquire(ctx, "th-parent-1", cancel)
	if err != nil {
		t.Fatalf("父租约获取失败: %v", err)
	}
	defer srv.activeTurns.Clear("th-parent-1", pLease)
	if got := ce.resolveParentThreadID(""); got != "th-parent-1" {
		t.Fatalf("空入参应推断出 th-parent-1: got %q", got)
	}
}

// parentOf 测试辅助:安全取线程父链,ok=false 时给占位避免 nil 解引用。
func parentOf(th *store.Thread, ok bool) string {
	if !ok {
		return "<未落库>"
	}
	return th.ParentThreadID
}
