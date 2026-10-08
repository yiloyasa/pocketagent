// codex_approval_wait_budget_20261009_test.go —— B3「审批等待超时预算」回归钉
// (2026-10-09):codex 反向请求的审批等待此前无上界 —— 手机在线、用户就是不点时,
// 反向请求永远占着未决槽、手机端挂着一张点不动的死卡。收口后给 15 分钟预算
// (codexApprovalWaitBudget),超预算按"通道失败"处置。
//
// 【钉什么】fake approve 挂起(流还在、人不点),缩时预算到期后断言超时分支
// 的**全部**副作用:
//   1. 回恰好一个 -32000 error(照 codex.go"通道失败不是用户拒绝"口径),
//      绝不回 {"decision":"denied"} 伪装用户意志(红线 1);
//   2. waiting 复位 —— 看门狗豁免解除,不能留着"永远合法静默"的轮次;
//   3. 未决请求注销 —— pendingResolved 不泄漏;
//   4. 无任何 decision 应答。
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

func TestCodexApprovalWaitBudgetTimeout(t *testing.T) {
	old := codexApprovalWaitBudget
	codexApprovalWaitBudget = 30 * time.Millisecond
	defer func() { codexApprovalWaitBudget = old }()

	p := NewCodexProvider()
	rsp := &fakeCodexResponder{}
	var waiting atomic.Bool
	id := json.RawMessage("77")
	// approve 挂起但 ctx 可取消:测试收尾放开 goroutine,不向进程泄漏常驻阻塞。
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.handleCodexApproval(rsp, id, reqApprovalCommand, "execCommandApproval",
		json.RawMessage(`{"command":["ls"]}`), nil,
		ctx, func(*pb.TurnServerMessage) {}, func(ctx context.Context, call *pb.ToolCallBlock) (bool, string, error) {
			<-ctx.Done() // 挂起:手机在线但用户不点,只有预算到期能出这个 select
			return false, "", ctx.Err()
		}, &waiting, "th-budget")

	// 1) 通道故障口径:恰好一个 -32000,文案说清"未取得用户决定",不是 denied。
	if rsp.errCount() != 1 {
		t.Fatalf("超预算应回恰好一个 error,得到 %d 个", rsp.errCount())
	}
	if code := rsp.errCodes[0]; code != -32000 {
		t.Fatalf("超预算应回 -32000(通道失败口径),得到 %d", code)
	}
	if msg := rsp.errMsgs[0]; !strings.Contains(msg, "审批等待超时") || !strings.Contains(msg, "未取得用户决定") {
		t.Fatalf("超预算文案应说清是等待超时而非用户决定,得到 %q", msg)
	}
	// 2) 绝不伪装用户拒绝:没有任何 decision 应答。
	if n := rsp.resultCount(); n != 0 {
		t.Fatalf("超预算绝不应回 decision(那是伪造用户意志),记录到 %d 条应答", n)
	}
	// 3) waiting 复位:看门狗豁免解除。
	if waiting.Load() {
		t.Fatal("超预算后 waiting 必须复位,否则轮次被永久豁免看门狗")
	}
	// 4) 未决请求注销:pendingResolved 不泄漏(白盒断言,同包口径)。
	p.pendingReqMu.Lock()
	_, still := p.pendingResolved[codexRequestIDKey(id)]
	p.pendingReqMu.Unlock()
	if still {
		t.Fatal("超预算分支必须注销未决请求,否则 pendingResolved 泄漏")
	}
}
