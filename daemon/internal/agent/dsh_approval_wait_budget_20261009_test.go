// dsh_approval_wait_budget_20261009_test.go —— 审批预算统一 15min(2026-10-09)
// 回归钉(照 codex_approval_wait_budget_20261009_test.go 先例):server 层
// approvalWaitBudget、codex 的 codexApprovalWaitBudget 与本家 dshApprovalTimeout
// 三处同预算后,provider 不得再比 server 先掐 —— 用户在 codex 面前能等到第 3 分钟,
// 在 dsh 面前也必须能等到(此前本家 2 分钟就把 server 的 15 分钟整体架空)。
//
// 【钉什么】
//  1. 预算值是产品参数:生产默认必须 15 分钟(三处互指口径,防单家漂移);
//  2. 缩时跑通超时分支,按 dsh 现有超时行为钉全部副作用:恰好一次成功应答、
//     选中 reject-once 侧(dsh 审批无 -32000 通道,超时走失败安全拒绝,绝不回
//     allow 伪装"同意",红线 1)、无任何 error 应答、waiting 复位(看门狗豁免
//     解除,不能留着"永远合法静默"的轮次)。
package agent

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

func TestDshApprovalWaitBudgetUnified15Min(t *testing.T) {
	// 1) 预算值是产品参数:默认 15min,与 server/codex 统一,不许单家漂移。
	if dshApprovalTimeout != 15*time.Minute {
		t.Fatalf("dshApprovalTimeout 生产默认应 15min(与 server approvalWaitBudget/codex codexApprovalWaitBudget 统一),得到 %s", dshApprovalTimeout)
	}

	// 2) 缩时单测:预算到期走超时分支,断言其全部副作用。
	old := dshApprovalTimeout
	dshApprovalTimeout = 40 * time.Millisecond
	t.Cleanup(func() { dshApprovalTimeout = old })

	fake := &fakeDshClient{}
	p := newTestDshProvider(t, fake)
	var waiting atomic.Bool
	// approve 挂起但 ctx 可取消:测试收尾放开 goroutine,不向进程泄漏常驻阻塞。
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		p.handleDshRequest(fake, 4242, "session/request_permission",
			json.RawMessage(permParams), ctx, func(*pb.TurnServerMessage) {},
			func(c context.Context, _ *pb.ToolCallBlock) (bool, string, error) {
				<-c.Done() // 手机在线但用户不点,只有预算到期能出
				return false, "", c.Err()
			}, &waiting, "th-budget")
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("缩时预算到期后 handleDshRequest 未返回(超时分支失灵)")
	}

	// waiting 复位:看门狗豁免解除(defer 复位必须在超时路径真实生效)。
	if waiting.Load() {
		t.Fatal("超预算后 waiting 必须复位,否则轮次被永久豁免看门狗")
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	// 超时是失败安全拒绝,不是协议故障:无任何 error 应答。
	if len(fake.responds) != 1 {
		t.Fatalf("超预算应恰好一次拒绝应答,得到 %d 次: %+v", len(fake.responds), fake.responds)
	}
	r := fake.responds[0]
	if r.isError {
		t.Fatalf("超预算走失败安全拒绝,不得回 error: %+v", r)
	}
	if r.id != 4242 {
		t.Fatalf("应答 id 应为请求原 id,得到 %d", r.id)
	}
	// 选中 reject-once 侧:绝不回 allow 伪装"同意"(红线 1)。
	b, _ := json.Marshal(r.result)
	if !strings.Contains(string(b), `"optionId":"reject-once"`) {
		t.Fatalf("超预算必须选中 reject-once 侧(失败安全,非用户决定): %s", b)
	}
}
