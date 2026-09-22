// output_state_test.go —— 「任务完成但正文为空」不再被伪装成占位文本。
//
// 【钉住的根因】codex / dsh / zcode 三个适配器在最终正文为空时各自塞一句
// "(完成,无文本输出)" 当**正文**发出来。后果:手机端分不清三种完全不同的情况
// ——真的没话说、只产出了非文本产物、适配器把正文弄丢了;用户看到的都是
// "任务完成 + 一句占位",无从判断该不该重试(方案 §4.4 的图二)。
//
// 【修复】适配器如实返回空块,由 server 层按 final_blocks 派 OutputState。
// 本文件钉两件事:判定规则本身,以及状态**真的进了 Completed 帧**(接线没断)。
package server

import (
	"context"
	"testing"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

func textBlock(s string) *pb.ContentBlock {
	return &pb.ContentBlock{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: s}}}
}

// 判定规则:有文本 → TEXT;只有产物 → ARTIFACT_ONLY;其余 → EMPTY。
func TestOutputStateOfClassifiesFinalBlocks(t *testing.T) {
	cases := []struct {
		name   string
		blocks []*pb.ContentBlock
		want   pb.OutputState
	}{
		{"有正文", []*pb.ContentBlock{textBlock("结论:可以合并")}, pb.OutputState_OUTPUT_STATE_TEXT},
		{"无块", nil, pb.OutputState_OUTPUT_STATE_EMPTY},
		{"空块切片", []*pb.ContentBlock{}, pb.OutputState_OUTPUT_STATE_EMPTY},
		{"空文本块", []*pb.ContentBlock{textBlock("")}, pb.OutputState_OUTPUT_STATE_EMPTY},
		{"只有空白", []*pb.ContentBlock{textBlock("  \n ")}, pb.OutputState_OUTPUT_STATE_EMPTY},
		{
			"只有思考",
			[]*pb.ContentBlock{{Block: &pb.ContentBlock_Thinking{Thinking: &pb.ThinkingBlock{Text: "想了很久"}}}},
			pb.OutputState_OUTPUT_STATE_EMPTY,
		},
		{
			"思考 + 正文",
			[]*pb.ContentBlock{
				{Block: &pb.ContentBlock_Thinking{Thinking: &pb.ThinkingBlock{Text: "想"}}},
				textBlock("正文"),
			},
			pb.OutputState_OUTPUT_STATE_TEXT,
		},
		{
			"只有图片",
			[]*pb.ContentBlock{{Block: &pb.ContentBlock_Image{Image: &pb.ImageBlock{}}}},
			pb.OutputState_OUTPUT_STATE_ARTIFACT_ONLY,
		},
		{
			"只有工具结果",
			[]*pb.ContentBlock{{Block: &pb.ContentBlock_ToolResult{ToolResult: &pb.ToolResultBlock{}}}},
			pb.OutputState_OUTPUT_STATE_ARTIFACT_ONLY,
		},
		{
			"思考 + 产物(仍算产物)",
			[]*pb.ContentBlock{
				{Block: &pb.ContentBlock_Thinking{Thinking: &pb.ThinkingBlock{Text: "想"}}},
				{Block: &pb.ContentBlock_Image{Image: &pb.ImageBlock{}}},
			},
			pb.OutputState_OUTPUT_STATE_ARTIFACT_ONLY,
		},
		{"含 nil 元素不 panic", []*pb.ContentBlock{nil}, pb.OutputState_OUTPUT_STATE_EMPTY},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := outputStateOf(tc.blocks); got != tc.want {
				t.Fatalf("outputStateOf = %v, want %v", got, tc.want)
			}
		})
	}
}

// emptyProvider 一轮结束**不给任何正文块** —— 就是三个适配器修好后的行为。
type emptyProvider struct{ blocks []pb.ContentBlock }

func (emptyProvider) ID() string             { return "empty" }
func (emptyProvider) DisplayName() string    { return "空输出测试" }
func (emptyProvider) Models() []pb.ModelInfo { return nil }

func (p emptyProvider) Run(_ context.Context, _ agent.TurnRequest, _ agent.Sink, _ agent.ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
	return p.blocks, &pb.Usage{}, nil
}

// newEmptyOutputServer 造一个用 "empty" provider 的测试 server。
// 【为什么不用 newEventTestServer】那个 helper 只接受 *seqProvider,而本文件要的
// 是"如实返回空块"的 provider;往 Providers 里塞 typed-nil 迟早踩空。
func newEmptyOutputServer(t *testing.T) *Server {
	t.Helper()
	fs, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("建测试存储失败: %v", err)
	}
	srv := New(Config{Store: fs, Providers: map[string]agent.Provider{}, DataDir: t.TempDir()})
	t.Cleanup(func() { _ = srv.events.Close() })
	th := &store.Thread{
		ID: "th-empty", Title: "空输出", Workspace: t.TempDir(),
		ModelID: "empty/m", Status: pb.ThreadStatus_THREAD_STATUS_IDLE,
	}
	if err := fs.Create(th); err != nil {
		t.Fatalf("建线程失败: %v", err)
	}
	return srv
}

// runTurnWith 用给定 provider 跑完一轮,返回手机收到的帧。
func runTurnWith(t *testing.T, srv *Server, thID string, p emptyProvider) []*pb.TurnServerMessage {
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

func completedFrameOf(t *testing.T, frames []*pb.TurnServerMessage) *pb.TurnCompleted {
	t.Helper()
	for _, f := range frames {
		if c := f.GetCompleted(); c != nil {
			return c
		}
	}
	t.Fatal("这轮没有 Completed 帧")
	return nil
}

// 【接线钉】空正文这一轮,Completed 帧必须如实上报 EMPTY,且**不带**任何占位文本。
func TestEmptyTurnReportsEmptyOutputState(t *testing.T) {
	s := newEmptyOutputServer(t)
	frames := runTurnWith(t, s, "th-empty", emptyProvider{})
	got := completedFrameOf(t, frames)
	if got.GetOutputState() != pb.OutputState_OUTPUT_STATE_EMPTY {
		t.Fatalf("空正文应上报 EMPTY, got %v", got.GetOutputState())
	}
	for _, b := range got.GetFinalBlocks() {
		if txt := b.GetText().GetText(); txt != "" {
			t.Fatalf("不该再出现占位正文, got %q", txt)
		}
	}
}

// 【接线钉】只有产物的一轮必须与"真的没话说"区分开。
func TestArtifactOnlyTurnReportsArtifactOnly(t *testing.T) {
	s := newEmptyOutputServer(t)
	p := emptyProvider{blocks: []pb.ContentBlock{
		{Block: &pb.ContentBlock_Image{Image: &pb.ImageBlock{}}},
	}}
	got := completedFrameOf(t, runTurnWith(t, s, "th-empty", p))
	if got.GetOutputState() != pb.OutputState_OUTPUT_STATE_ARTIFACT_ONLY {
		t.Fatalf("只有产物应上报 ARTIFACT_ONLY, got %v", got.GetOutputState())
	}
}
