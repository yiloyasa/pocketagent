// turn_cancel_conduction_test.go —— CancelTurn RPC → provider Run ctx 的全链传导断言。
//
// 【钉什么】hermes 类"prompt 阻塞到轮次终局"的 provider,停止键要真正点得掉,
// 依赖一条看不见的传导链:CancelTurn RPC → activeTurns.Cancel(取当前轮的
// streamCancel)→ streamRunCtx → runTurnOnce 的 runCtx → provider.Run 的 ctx。
// agent 包的 hermes_cancel_test.go 已钉住"ctx 取消 → session/cancel + partial
// 交回"的后半段;本文件补前半段 —— **server 层取消必须真的到达 provider**,
// 中途任何一环被拆(比如有人把 acquire 登记的 cancel 换成 no-op),这里先红。
//
// 【为什么从 StartTurn 测】传导链的每一环都长在 StartTurn/runTurnOnce 的装配
// 代码里,单测任何一层纯函数都挡不住"装配点被人改掉";fakeTurnStream 模式
// (turn_overrides_test.go)从 gRPC 报文一路喂到 provider,是既有最小面。
package server

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// cancelBlockingProvider 模拟 hermes 的 Run 形态:进入后先流一段正文,再挂住等
// 终局;ctx 取消则交回已产出正文 + ctx.Err()(与 hermes 看门狗的 ctx 闸同语义)。
// 它记住观察到的 ctx.Err() —— 传导断言的取证点。
type cancelBlockingProvider struct {
	entered   chan struct{}
	returned  chan struct{}
	sawCtxErr error
}

func (p *cancelBlockingProvider) ID() string             { return "hermes" }
func (p *cancelBlockingProvider) DisplayName() string    { return "Hermes(取消传导测试替身)" }
func (p *cancelBlockingProvider) Models() []pb.ModelInfo { return nil }

func (p *cancelBlockingProvider) Run(ctx context.Context, _ agent.TurnRequest, sink agent.Sink, _ agent.ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
	close(p.entered)
	sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_TextDelta{TextDelta: &pb.TextDelta{Delta: "已产出"}}})
	<-ctx.Done()
	p.sawCtxErr = ctx.Err()
	close(p.returned)
	return []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "已产出"}}}}, nil, ctx.Err()
}

// TestCancelTurnConductsToProviderRunCtx 手机"终止任务"(CancelTurn RPC)必须
// 传导为 provider.Run 的 ctx 取消,轮次按 Failed(带 partial)收场、流正常结束;
// 收尾后再发一次 CancelTurn 是安全 no-op。
func TestCancelTurnConductsToProviderRunCtx(t *testing.T) {
	st := mustStore(t)
	th := &store.Thread{
		ID: "th-cancel-1", AgentID: "hermes", Workspace: t.TempDir(),
		Status: pb.ThreadStatus_THREAD_STATUS_IDLE,
	}
	if err := st.Create(th); err != nil {
		t.Fatalf("建线程失败: %v", err)
	}
	p := &cancelBlockingProvider{entered: make(chan struct{}), returned: make(chan struct{})}
	srv := New(Config{Store: st, Providers: map[string]agent.Provider{"hermes": p}})

	stream := &fakeTurnStream{
		ctx: context.Background(),
		in: []*pb.TurnClientMessage{{Msg: &pb.TurnClientMessage_Input{Input: &pb.TurnInput{
			ThreadId: th.ID,
			Blocks:   []*pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "长任务,慢慢跑"}}}},
		}}}},
	}
	streamDone := make(chan error, 1)
	go func() { streamDone <- srv.StartTurn(stream) }()

	select {
	case <-p.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("轮次未启动,测不了取消")
	}

	// 手机端"终止任务"按钮 → CancelTurn RPC。
	if _, err := srv.CancelTurn(context.Background(), &pb.CancelTurnRequest{ThreadId: th.ID}); err != nil {
		t.Fatalf("CancelTurn 应成功: %v", err)
	}

	// 传导断言:provider.Run 观察到的 ctx 必须已取消。
	select {
	case <-p.returned:
		if !errors.Is(p.sawCtxErr, context.Canceled) {
			t.Fatalf("CancelTurn 必须传导为 provider ctx 取消,实际观察到 %v", p.sawCtxErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("CancelTurn 后 provider.Run 未返回 —— 停止键点不掉回归")
	}

	// 用户主动停 = 清空队列并正常结束流(不报错、不自动续跑排队输入)。
	select {
	case err := <-streamDone:
		if err != nil {
			t.Fatalf("取消收场应正常结束流: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("StartTurn 未收场")
	}

	// 终局帧:Failed(AGENT_ERROR + 已产出的 partial)——手机端据此结束转圈
	// 并保留已流出的内容。
	var failed *pb.TurnFailed
	for _, m := range stream.sent {
		if f := m.GetFailed(); f != nil {
			failed = f
		}
	}
	if failed == nil {
		t.Fatalf("取消后流上应有 Failed 帧,实际帧数 %d", len(stream.sent))
	}
	if failed.PartialText != "已产出" {
		t.Fatalf("已产出的正文必须随 Failed 帧交回(partial),实际 %q", failed.PartialText)
	}
	if failed.Error == nil || failed.Error.Code != "AGENT_ERROR" {
		t.Fatalf("取消收场应按 AGENT_ERROR 报告: %+v", failed.Error)
	}

	// 轮次已收尾:注册表里没有活跃轮次,再发一次 CancelTurn 必须安全 no-op。
	if _, err := srv.CancelTurn(context.Background(), &pb.CancelTurnRequest{ThreadId: th.ID}); err != nil {
		t.Fatalf("无在跑轮次时 CancelTurn 应为安全 no-op: %v", err)
	}
}
