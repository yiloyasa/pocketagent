// hermes_approval_wait_budget_20261009_test.go —— 审批预算统一 15min(2026-10-09)
// 回归钉(照 codex_approval_wait_budget_20261009_test.go 先例):server 层
// approvalWaitBudget、codex 的 codexApprovalWaitBudget 与本家 hermesApprovalTimeout
// 三处同预算后,provider 不得再比 server 先掐 —— 用户在 codex 面前能等到第 3 分钟,
// 在 hermes 面前也必须能等到(此前本家 2 分钟就把 server 的 15 分钟整体架空)。
//
// 【钉什么】
//  1. 预算值是产品参数:生产默认必须 15 分钟(三处互指口径,防单家漂移);
//  2. 缩时跑通超时分支,按 hermes 现有超时行为钉全部副作用:恰好一次成功应答、
//     选中 deny 侧(hermes 审批无 -32000 通道,超时走失败安全拒绝,绝不回
//     allow 伪装"同意",红线 1)、无任何 error 应答、应答 id 回到原请求。
package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

func TestHermesApprovalWaitBudgetUnified15Min(t *testing.T) {
	// 1) 预算值是产品参数:默认 15min,与 server/codex 统一,不许单家漂移。
	if hermesApprovalTimeout != 15*time.Minute {
		t.Fatalf("hermesApprovalTimeout 生产默认应 15min(与 server approvalWaitBudget/codex codexApprovalWaitBudget 统一),得到 %s", hermesApprovalTimeout)
	}

	// 2) 缩时单测:预算到期走超时分支,断言其全部副作用。
	old := hermesApprovalTimeout
	hermesApprovalTimeout = 40 * time.Millisecond
	t.Cleanup(func() { hermesApprovalTimeout = old })

	fake := newFakeHermesClient(true)
	p := newHermesTestProvider(t, fake)
	// approve 挂起但 ctx 可取消:测试收尾放开 goroutine,不向进程泄漏常驻阻塞。
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		p.handleRequestPermission(fake, 4242, "session/request_permission",
			json.RawMessage(fakePermParams), ctx, func(*pb.TurnServerMessage) {},
			func(c context.Context, _ *pb.ToolCallBlock) (bool, string, error) {
				<-c.Done() // 手机在线但用户不点,只有预算到期能出
				return false, "", c.Err()
			}, "th-budget")
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("缩时预算到期后 handleRequestPermission 未返回(超时分支失灵)")
	}

	fake.mu.Lock()
	defer fake.mu.Unlock()
	// 超时是失败安全拒绝,不是协议故障:无任何 error 应答。
	if len(fake.respondErr) != 0 {
		t.Fatalf("超预算走失败安全拒绝,不得回 error: %+v", fake.respondErr)
	}
	if len(fake.responds) != 1 {
		t.Fatalf("超预算应恰好一次拒绝应答,得到 %d 次", len(fake.responds))
	}
	if fake.responds[0].ID != 4242 {
		t.Fatalf("应答 id 应为请求原 id,得到 %d", fake.responds[0].ID)
	}
	// 选中 deny 侧:绝不回 allow 伪装"同意"(红线 1)。
	out, _ := json.Marshal(fake.responds[0].Result)
	if !strings.Contains(string(out), `"optionId":"deny"`) {
		t.Fatalf("超预算必须选中 deny 侧(失败安全,非用户决定): %s", out)
	}
}
