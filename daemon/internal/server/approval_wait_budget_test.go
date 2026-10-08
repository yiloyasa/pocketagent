package server

// approval_wait_budget_test.go —— 审批等待预算(server 层)的**穿透式**回归钉子。
//
// 【背景(2026-10-09 实锤)】thread.go 的 approve 钩子曾硬编码 2 分钟超时,把
// agent/codex.go 的 codexApprovalWaitBudget(15 分钟)整体架空;当天的预算回归
// 测试在 provider 层直接注入挂起 approve、绕过 server 层,所以测试绿但防线无效。
// 本文件的两条用例都走**真 server 层**:concurrencyProvider 通过 StartTurn →
// runTurnOnce → approve 闭包真实拿到 server 注入的 ApprovalHook,审批卡经
// feedTurnStream 送到"手机",应答(或不应答)走 SubmitApproval / 预算超时。
//
// 【钉什么】
//  1. TestServerApprovalBudgetTruncatesHungApproval:缩时预算内无人应答 →
//     轮次按预算快速收尾,provider 的 ask 收到的是预算超时(context.DeadlineExceeded),
//     而不是任何比预算更长的硬编码截断 —— 若有人把 2 分钟字面量改回来,
//     本用例会在 awaitTurn 的 5 秒上限先炸(2 分钟 >> 5 秒)。
//  2. TestServerApprovalDeliveredWithinBudget:预算内真实应答 → 决定原样送达
//     provider(server 层没有过度截断,链路完好)。

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// shrinkApprovalBudget 收紧 server 层审批预算并注册恢复,返回原值。
func shrinkApprovalBudget(t *testing.T, d time.Duration) {
	t.Helper()
	old := approvalWaitBudget
	approvalWaitBudget = d
	t.Cleanup(func() { approvalWaitBudget = old })
}

// waitApprovalRequest 轮询流快照,等第一张审批卡出现(5 秒上限)。
func waitApprovalRequest(t *testing.T, f *feedTurnStream) *pb.ApprovalRequest {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		for _, m := range f.snapshot() {
			if r := m.GetApprovalRequest(); r != nil {
				return r
			}
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("审批卡从未出现")
	return nil
}

// TestServerApprovalBudgetTruncatesHungApproval 预算内无人应答:轮次按缩时预算
// 收尾,provider 收到预算超时错误。全程必须在 awaitTurn 的 5 秒上限内完成 ——
// 这一条同时排除"硬编码 2 分钟截断"回归(它跑不完 5 秒)。
func TestServerApprovalBudgetTruncatesHungApproval(t *testing.T) {
	shrinkApprovalBudget(t, 300*time.Millisecond)
	askErrCh := make(chan error, 1)
	p := &concurrencyProvider{run: func(ctx context.Context, _ agent.TurnRequest, _ agent.Sink, ask agent.ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
		_, _, err := ask(ctx, &pb.ToolCallBlock{Id: "native-tool", Name: "shell"})
		askErrCh <- err
		return nil, &pb.Usage{}, err
	}}
	s := concurrencyServer(t, p)
	f := concurrencyStream("budget-hang")
	done := make(chan error, 1)
	go func() { done <- s.StartTurn(f) }()
	defer closeIfOpenFeed(f)

	waitApprovalRequest(t, f) // 卡已到"手机",故意不点

	// 轮次必须在远小于旧 2 分钟硬编码的时间内自行收尾。
	select {
	case err := <-done:
		// provider 轮次失败被 runTurnOnce 包成 turnRunError,StartTurn 按
		// P1-4 语义继续排队续跑(队列空 → 返回 nil),所以这里 err == nil 是预期。
		_ = err
	case <-time.After(5 * time.Second):
		t.Fatal("轮次未按缩时预算收尾(疑似回到硬编码 2 分钟截断)")
	}
	// provider 的 ask 必须收到"预算超时",而不是 nil(假成功)或别的错误。
	select {
	case err := <-askErrCh:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("ask 应收到预算超时(context.DeadlineExceeded),得到 %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("provider 的 ask 从未返回")
	}
	// 审批卡必须有终局帧(Resolved),手机端不留点不动的死卡。
	var resolved bool
	for _, m := range f.snapshot() {
		if m.GetApprovalResolved() != nil {
			resolved = true
		}
	}
	if !resolved {
		t.Fatal("审批卡未收到 Resolved 终局帧")
	}
}

// TestServerApprovalDeliveredWithinBudget 预算内应答:决定原样送达 provider,
// 证明收口后的 server 链路(卡下发 + SubmitApproval 回填)完好、没有过度截断。
func TestServerApprovalDeliveredWithinBudget(t *testing.T) {
	// 缩到 2 秒:给测试调度留裕量,同时仍远小于旧 2 分钟(证明生效值是可注入
	// 的变量,不是任何硬编码)。
	shrinkApprovalBudget(t, 2*time.Second)
	decisionCh := make(chan bool, 1)
	p := &concurrencyProvider{run: func(ctx context.Context, _ agent.TurnRequest, _ agent.Sink, ask agent.ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
		ok, _, err := ask(ctx, &pb.ToolCallBlock{Id: "native-tool", Name: "shell"})
		decisionCh <- ok
		return nil, &pb.Usage{}, err
	}}
	s := concurrencyServer(t, p)
	f := concurrencyStream("budget-deliver")
	done := make(chan error, 1)
	go func() { done <- s.StartTurn(f) }()
	defer closeIfOpenFeed(f)

	req := waitApprovalRequest(t, f)
	if _, err := s.SubmitApproval(context.Background(), &pb.SubmitApprovalRequest{ThreadId: "t",
		Decision: &pb.ApprovalDecision{RequestId: req.RequestId, Approved: true}}); err != nil {
		t.Fatal(err)
	}
	awaitTurn(t, done)
	select {
	case ok := <-decisionCh:
		if !ok {
			t.Fatal("预算内应答被截断成拒绝")
		}
	case <-time.After(time.Second):
		t.Fatal("provider 的 ask 从未返回")
	}
}
