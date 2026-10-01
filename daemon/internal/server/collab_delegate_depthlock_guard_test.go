// collab_delegate_depthlock_guard_test.go —— X2 深度锁推理漏洞的回归钉子
// (2026-09-30)。漏洞形状:委派子线程(其代理进程)发起再委派时,
// soleParentTurn 把**仍阻塞中的父轮次**误判为唯一父 —— 父轮次此刻同步阻塞在
// 它自己的 agent_delegate 工具调用上(租约未放),真正的发起方是仍在运行的
// 子线程;旧逻辑据此把子线程的再委派挂到非子父链上,第二道深度闸
// (opts.ParentThreadID ∈ childThreads)被完整绕过,"子不生子"失效。
//
// 修复:DelegateSubTask 的父链解析改走 resolveParentThreadIDGuarded —— 推断出
// 唯一候选、且此刻仍有委派子轮次在飞时,不再信任推断,以 errDelegateDepthRejected
// 如实拒绝(同步阻塞语义保证候选父不可能是发起方)。
//
// 钉三件事:
//  1. 攻击面(核心):父轮次持租约 + 子轮次在飞 + 空 parent_thread_id 的委派
//     → 深度拒绝,不建孙线程(RPC 级端到端,过真拦截器);
//  2. 正常面不被误伤:无子在飞时推断照常成立(首个委派的父链仍点亮);
//  3. 诚实变暗面:候选不唯一/无候选时不拒绝也不伪造,父链留空(与既有
//     soleParentTurn 语义一致,深度防线只加在"唯一候选 + 子在飞"上)。
package server

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// TestResolveParentThreadIDGuardedBlocksWhileChildInFlight 攻击面(引擎级):
// 唯一非子候选(阻塞中的父)+ 委派子轮次在飞 → 防线拒绝。修复前该状态会
// 推断出阻塞父并把再委派放行(漏洞本体),修复后必须返回 errDelegateDepthRejected。
func TestResolveParentThreadIDGuardedBlocksWhileChildInFlight(t *testing.T) {
	srv, eng := newCollabTestEngine(t, map[string]agent.Provider{"echo": agent.NewEcho()})
	ce := eng.(*collabEngine)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 父轮次持租约(协调器 A 正在跑轮)。
	pLease, err := srv.activeTurns.acquire(ctx, "th-parent-1", cancel)
	if err != nil {
		t.Fatalf("父租约获取失败: %v", err)
	}
	defer srv.activeTurns.Clear("th-parent-1", pLease)

	// 委派子线程 B:登记进 childThreads + 轮次活跃(模拟"子的代理进程正在跑")。
	cLease, err := srv.activeTurns.acquire(ctx, "th-child-1", cancel)
	if err != nil {
		t.Fatalf("子租约获取失败: %v", err)
	}
	defer srv.activeTurns.Clear("th-child-1", cLease)
	ce.mu.Lock()
	ce.childThreads["th-child-1"] = true
	ce.mu.Unlock()

	// 空 parent_thread_id(子线程的代理进程不会填)→ 防线必须拒绝。
	pid, gerr := ce.resolveParentThreadIDGuarded("")
	if !errors.Is(gerr, errDelegateDepthRejected) {
		t.Fatalf("子线程再委派必须被深度闸拦截: pid=%q err=%v", pid, gerr)
	}
	if pid != "" {
		t.Fatalf("被拒时父链必须留空: %q", pid)
	}

	// 对照:同一状态下显式父链仍原样透传(它由引擎入口闸按 childThreads 精确
	// 判定,不经本防线 —— 语义与既有钉子一致)。
	if pid, gerr := ce.resolveParentThreadIDGuarded("th-explicit"); gerr != nil || pid != "th-explicit" {
		t.Fatalf("显式父链应原样透传: pid=%q err=%v", pid, gerr)
	}
}

// TestResolveParentThreadIDGuardedHonestStates 诚实变暗面(不拒绝也不伪造):
//   - 唯一候选但无子在飞 → 正常推断(正常面,首个委派的父链仍要点亮);
//   - 有子在飞但候选不唯一(另一协调器也在跑)→ 留空不拒绝;
//   - 有子在飞但无非子候选(子的父轮次已收尾的窗口)→ 留空不拒绝。
func TestResolveParentThreadIDGuardedHonestStates(t *testing.T) {
	srv, eng := newCollabTestEngine(t, map[string]agent.Provider{"echo": agent.NewEcho()})
	ce := eng.(*collabEngine)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 唯一候选、无子在飞 → 推断照常成立(正常面)。
	pLease, err := srv.activeTurns.acquire(ctx, "th-parent-1", cancel)
	if err != nil {
		t.Fatalf("父租约获取失败: %v", err)
	}
	pid, gerr := ce.resolveParentThreadIDGuarded("")
	if gerr != nil || pid != "th-parent-1" {
		t.Fatalf("无子在飞时应照常推断出父: pid=%q err=%v", pid, gerr)
	}

	// 唯一候选 + 另一非子候选(第二个协调器)同时活跃 + 子在飞 → 候选不唯一,
	// 不拒绝、留空(诚实变暗;拒绝只属于"唯一候选 + 子在飞")。
	cLease, err := srv.activeTurns.acquire(ctx, "th-child-1", cancel)
	if err != nil {
		t.Fatalf("子租约获取失败: %v", err)
	}
	defer srv.activeTurns.Clear("th-child-1", cLease)
	ce.mu.Lock()
	ce.childThreads["th-child-1"] = true
	ce.mu.Unlock()
	p2Lease, err := srv.activeTurns.acquire(ctx, "th-parent-2", cancel)
	if err != nil {
		t.Fatalf("第二父租约获取失败: %v", err)
	}
	pid, gerr = ce.resolveParentThreadIDGuarded("")
	if gerr != nil || pid != "" {
		t.Fatalf("候选不唯一时不应拒绝也不应伪造: pid=%q err=%v", pid, gerr)
	}

	// 无非子候选(两个父轮次都已收尾、只剩子在飞的窗口)→ 留空不拒绝。
	srv.activeTurns.Clear("th-parent-1", pLease)
	srv.activeTurns.Clear("th-parent-2", p2Lease)
	pid, gerr = ce.resolveParentThreadIDGuarded("")
	if gerr != nil || pid != "" {
		t.Fatalf("无候选时应留空不拒绝: pid=%q err=%v", pid, gerr)
	}
}

// TestDelegateSubTaskBlocksChildThreadRedelegation 攻击面(RPC 级端到端,过真
// 拦截器):协调器 A 的轮次持租约,其委派子线程 B 的轮次正在飞;此时 B 的代理
// 进程发来**空 parent_thread_id** 的再委派 → 修复前:soleParentTurn 推断出阻塞
// 中的 A,深度闸被绕过,孙线程被建出;修复后:防线拦截,error 字段如实说明
// 深度拒绝,孙线程不存在。首个委派本身不受影响、照常完成(正常面同场验证)。
func TestDelegateSubTaskBlocksChildThreadRedelegation(t *testing.T) {
	gate := &gateProvider{entered: make(chan struct{}), release: make(chan struct{})}
	client, srv, cleanup := newCollabRPCServer(t, map[string]agent.Provider{"gate": gate, "echo": agent.NewEcho()})
	defer cleanup()
	tok := LegacyTokenFor(collabRPCPairCode)
	ce := srv.collabEngineFor()

	// 协调器 A 的手机轮次持租约(agent_delegate 由它同步发起)。
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pLease, err := srv.activeTurns.acquire(ctx, "th-parent-1", cancel)
	if err != nil {
		t.Fatalf("父租约获取失败: %v", err)
	}
	defer srv.activeTurns.Clear("th-parent-1", pLease)

	// 第一个委派(A 发起):子线程 B 进入 Run 后阻塞 —— B 已登记进 childThreads。
	var firstResp *pb.DelegateSubTaskResponse
	done := make(chan struct{})
	go func() {
		defer close(done)
		firstResp, _ = client.DelegateSubTask(clientCtxWithBearer(tok),
			&pb.DelegateSubTaskRequest{AgentId: "gate", Task: "外层委派"})
	}()
	select {
	case <-gate.entered: // B 已真正跑起来(登记先于 Run 进入,此刻必已在册)
	case <-time.After(5 * time.Second):
		t.Fatal("超时:首个委派未进入运行")
	}

	// 子线程 B 的代理进程发起再委派(空 parent_thread_id,与生产链路一致):
	// 深度锁必须拦截,不建孙线程。
	rej, err := client.DelegateSubTask(clientCtxWithBearer(tok),
		&pb.DelegateSubTaskRequest{AgentId: "echo", Task: "子线程的再委派"})
	if err != nil {
		t.Fatalf("深度拒绝应走 error 字段而非 gRPC status: %v", err)
	}
	if !strings.Contains(rej.GetError(), "不能再委派") {
		t.Fatalf("再委派必须被深度闸拦截: %q", rej.GetError())
	}
	if rej.GetChildThreadId() != "" {
		t.Fatalf("被拒委派不得建孙线程: %q", rej.GetChildThreadId())
	}
	if threads := srv.cfg.Store.List(true); len(threads) != 1 {
		t.Fatalf("孙线程不应存在(库里应只有子线程 B): %d 条", len(threads))
	}

	// 正常面:首个委派不被误伤,放行后照常完成;其父链仍点亮(A 的推断成立,
	// 当时无子在飞)。
	close(gate.release)
	<-done
	if firstResp.GetError() != "" {
		t.Fatalf("首个委派不应被误伤: %q", firstResp.GetError())
	}
	got, ok := srv.cfg.Store.Get(firstResp.GetChildThreadId())
	if !ok || got.ParentThreadID != "th-parent-1" {
		t.Fatalf("首个委派的父链应照常推断落库: ok=%v parent=%q", ok, parentOf(got, ok))
	}
	ce.mu.Lock()
	marked := ce.childThreads[firstResp.GetChildThreadId()]
	ce.mu.Unlock()
	if !marked {
		t.Fatalf("首个委派的子线程 %s 没有打来源标记", firstResp.GetChildThreadId())
	}
}
