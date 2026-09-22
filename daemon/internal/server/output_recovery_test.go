package server

// output_recovery_test.go —— 空正文时的原生历史补偿(方案 §4.4 图二 / §8-D3)。
//
// 【钉住的场景】用户报的"任务完成却什么都没有"里,有一部分是**正文确实存在、只是没经
// 增量到达**(上下文压缩边界、事件漏帧)。不查原生历史就只能报"没有正文",而用户
// 看着电脑上明明有 —— 那正是图二。本组用例把四档出口逐一钉死,免得日后有人把
// "读不出来"与"确实没有"又合成一个含糊的状态。

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// recoveringProvider 一轮不给正文块,但实现了 NativeTurnReader —— 模拟 codex
// (它既有增量流、也有可查的原生 rollout)。
type recoveringProvider struct {
	blocks []pb.ContentBlock
	text   string
	err    error
}

func (recoveringProvider) ID() string             { return "empty" }
func (recoveringProvider) DisplayName() string    { return "空输出+原生历史测试" }
func (recoveringProvider) Models() []pb.ModelInfo { return nil }

func (p recoveringProvider) Run(_ context.Context, _ agent.TurnRequest, _ agent.Sink, _ agent.ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
	return p.blocks, &pb.Usage{}, nil
}

func (p recoveringProvider) ReadNativeTurn(_ context.Context, _ *store.Thread) (*agent.NativeTurnResult, error) {
	if p.err != nil {
		return nil, p.err
	}
	if p.text == "" {
		return nil, nil // 原生也没有终局记录
	}
	return &agent.NativeTurnResult{Text: p.text}, nil
}

// runTurnWithReader 与 runTurnWith 同构,只是 provider 类型要能带原生历史读取。
func runTurnWithReader(t *testing.T, srv *Server, thID string, p recoveringProvider) []*pb.TurnServerMessage {
	t.Helper()
	srv.cfg.Providers["empty"] = p
	stream := &feedTurnStream{ctx: context.Background(), in: make(chan *pb.TurnClientMessage, 8)}
	stream.push(t, textInput(thID, "跑一下"))
	done := make(chan error, 1)
	go func() { done <- srv.StartTurn(stream) }()
	close(stream.in)
	if err := <-done; err != nil {
		t.Fatalf("StartTurn: %v", err)
	}
	return stream.snapshot()
}

func textOfCompleted(t *testing.T, c *pb.TurnCompleted) string {
	t.Helper()
	var sb strings.Builder
	for _, b := range c.GetFinalBlocks() {
		sb.WriteString(b.GetText().GetText())
	}
	return sb.String()
}

// 原生历史里有正文 → 补回来,并如实标 RECOVERED + 写明来源。
// 【为什么"来源"也要钉】手机端与事后排障都靠它区分"这轮本来就正常"与"是补回来的"。
func TestEmptyTurnRecoversTextFromNativeHistory(t *testing.T) {
	s := newEmptyOutputServer(t)
	frames := runTurnWithReader(t, s, "th-empty", recoveringProvider{text: "原生历史里的最终答复"})

	got := completedFrameOf(t, frames)
	if got.GetOutputState() != pb.OutputState_OUTPUT_STATE_RECOVERED_FROM_NATIVE_HISTORY {
		t.Fatalf("读到正文就该标 RECOVERED, got %v", got.GetOutputState())
	}
	if got.GetOutputSource() != "native_history" {
		t.Fatalf("output_source 应指明来源, got %q", got.GetOutputSource())
	}
	if txt := textOfCompleted(t, got); !strings.Contains(txt, "原生历史里的最终答复") {
		t.Fatalf("补偿到的正文必须进 final_blocks, got %q", txt)
	}
}

// 原生记录确认没有正文 → EMPTY。这是**事实陈述**,不是错误。
func TestEmptyTurnStaysEmptyWhenNativeHasNoText(t *testing.T) {
	s := newEmptyOutputServer(t)
	frames := runTurnWithReader(t, s, "th-empty", recoveringProvider{})

	got := completedFrameOf(t, frames)
	if got.GetOutputState() != pb.OutputState_OUTPUT_STATE_EMPTY {
		t.Fatalf("原生也没有正文时应为 EMPTY, got %v", got.GetOutputState())
	}
	if got.GetOutputSource() != "" {
		t.Fatalf("没补偿到就不该编一个来源, got %q", got.GetOutputSource())
	}
}

// 读取失败 → ADAPTER_LOST_OUTPUT:**不能确认**是"真没有"还是"我们弄丢了"。
// 【为什么不能归 EMPTY】那等于替用户断言"这台机器就是没产出" —— 而他可能在电脑上
// 明明看到了。如实标"不确定",用户才知道该重试。
func TestEmptyTurnReportsAdapterLostOnReadFailure(t *testing.T) {
	s := newEmptyOutputServer(t)
	frames := runTurnWithReader(t, s, "th-empty", recoveringProvider{err: errors.New("rollout 读取失败")})

	got := completedFrameOf(t, frames)
	if got.GetOutputState() != pb.OutputState_OUTPUT_STATE_ADAPTER_LOST_OUTPUT {
		t.Fatalf("读不出来时应标 ADAPTER_LOST_OUTPUT(提示可重试), got %v", got.GetOutputState())
	}
}

// 该 provider 没有原生历史可查 → EMPTY:我们没有理由怀疑它丢了正文。
func TestEmptyTurnStaysEmptyForProviderWithoutNativeHistory(t *testing.T) {
	s := newEmptyOutputServer(t)
	frames := runTurnWith(t, s, "th-empty", emptyProvider{})

	got := completedFrameOf(t, frames)
	if got.GetOutputState() != pb.OutputState_OUTPUT_STATE_EMPTY {
		t.Fatalf("没有原生历史可查时应为 EMPTY, got %v", got.GetOutputState())
	}
}
