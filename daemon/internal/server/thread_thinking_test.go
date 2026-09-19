// thread_thinking_test.go —— "思考流落库"的钉子(设置页第 4 项的根因修复)。
//
// 【为什么必须从 StartTurn 测】这个 bug 的形态是"过路帧没人接":
// ThinkingDelta 在 sink 总线上推给手机后就丢了,assistant 消息落库时只剩
// 正文 —— 只测 prependThinkingBlock 纯函数挡不住"sink 漏接/落库点漏改"。
// 这里从 gRPC 报文一路测到 store:provider 在 Run 里推思考增量 + 返回正文,
// StartTurn 返回后,store 里的 assistant 消息必须带 ThinkingBlock(且在正文前),
// 重进会话的历史恢复(GetThreadHistory 读的就是这份 store)才有思考卡可渲染。
//
// 【与"隐藏推理过程"的关系】HideReasoningFor=true 时 provider 层(codex.go)
// 根本不发 ThinkingDelta —— 缓冲区收不到,落库自然没有思考块。这里用
// "不发思考增量的 provider"模拟该形态,钉住"隐藏时落库也没有思考块"。
package server

import (
	"context"
	"sync"
	"testing"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// thinkingEmitter 先推思考增量,再返回最终正文(模拟 codex 的真实事件时序)。
type thinkingEmitter struct {
	deltas   []string // 通过 sink 推的思考增量;nil = 整轮无思考(模拟"隐藏推理"生效)
	failText string   // 非空时 Run 返回错误(带部分正文),走失败落库分支
}

func (p *thinkingEmitter) ID() string             { return "codex" }
func (p *thinkingEmitter) DisplayName() string    { return "思考发射器(测试替身)" }
func (p *thinkingEmitter) Models() []pb.ModelInfo { return nil }

func (p *thinkingEmitter) Run(_ context.Context, _ agent.TurnRequest, sink agent.Sink, _ agent.ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
	for _, d := range p.deltas {
		sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_ThinkingDelta{ThinkingDelta: &pb.ThinkingDelta{Delta: d}}})
	}
	if p.failText != "" {
		return []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: p.failText}}}},
			&pb.Usage{}, context.DeadlineExceeded
	}
	return []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "结论"}}}}, &pb.Usage{}, nil
}

// newThinkingTestServer 落一条存量线程并接上测试 provider。
func newThinkingTestServer(t *testing.T, p agent.Provider) (*Server, *store.Thread) {
	t.Helper()
	fs, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("建 store 失败: %v", err)
	}
	srv := New(Config{Store: fs, Providers: map[string]agent.Provider{"codex": p}})
	th := &store.Thread{ID: "th-think", Title: "思考落库", Workspace: t.TempDir()}
	if err := fs.Create(th); err != nil {
		t.Fatalf("建线程失败: %v", err)
	}
	return srv, th
}

// runSimpleTurn 用 fakeTurnStream(turn_overrides_test.go)跑一轮。
func runSimpleTurn(t *testing.T, srv *Server, threadID string) *fakeTurnStream {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream := &fakeTurnStream{
		ctx: ctx,
		in: []*pb.TurnClientMessage{{Msg: &pb.TurnClientMessage_Input{Input: &pb.TurnInput{
			ThreadId: threadID,
			Blocks: []*pb.ContentBlock{
				{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "hi"}}},
			},
		}}}},
	}
	if err := srv.StartTurn(stream); err != nil {
		t.Fatalf("StartTurn: %v", err)
	}
	return stream
}

// lastAssistantBlocks 取线程最后一条 assistant 消息的块。
func lastAssistantBlocks(t *testing.T, srv *Server, threadID string) []pb.ContentBlock {
	t.Helper()
	th, ok := srv.cfg.Store.Get(threadID)
	if !ok {
		t.Fatal("线程丢了")
	}
	for i := len(th.Messages) - 1; i >= 0; i-- {
		if th.Messages[i].Role == "assistant" {
			return th.Messages[i].Blocks
		}
	}
	t.Fatal("没有 assistant 消息落库")
	return nil
}

func TestThinkingDeltaPersistedAcrossHistory(t *testing.T) {
	p := &thinkingEmitter{deltas: []string{"先看目录", "再读主函数"}}
	srv, th := newThinkingTestServer(t, p)
	stream := runSimpleTurn(t, srv, th.ID)

	blocks := lastAssistantBlocks(t, srv, th.ID)
	if len(blocks) != 2 {
		t.Fatalf("落库块数 = %d, want 2(思考+正文): %+v", len(blocks), blocks)
	}
	// 思考块在前(与流式时序一致),文本完整 = 增量按序拼接
	thb, ok := blocks[0].Block.(*pb.ContentBlock_Thinking)
	if !ok {
		t.Fatalf("第一块应是思考块, got %T", blocks[0].Block)
	}
	if want := "先看目录再读主函数"; thb.Thinking.Text != want {
		t.Fatalf("思考文本 = %q, want %q", thb.Thinking.Text, want)
	}
	if tb, ok := blocks[1].Block.(*pb.ContentBlock_Text); !ok || tb.Text.Text != "结论" {
		t.Fatalf("第二块应是正文「结论」, got %T", blocks[1].Block)
	}

	// 终局帧 final_blocks 与落库一致(离线快照/事件回放同源);
	// 手机端终局只提取文本,多出来的思考块不会被重复渲染。
	var completed *pb.TurnCompleted
	for _, m := range stream.sent {
		if c := m.GetCompleted(); c != nil {
			completed = c
		}
	}
	if completed == nil || len(completed.FinalBlocks) != 2 {
		t.Fatalf("Completed.final_blocks 应含 2 块, got %+v", completed)
	}

	// GetThreadHistory 读的就是这份 store:历史恢复路径从此有思考块可渲染
	resp, err := srv.GetThreadHistory(context.Background(), &pb.GetThreadHistoryRequest{ThreadId: th.ID})
	if err != nil {
		t.Fatalf("GetThreadHistory: %v", err)
	}
	var last *pb.ThreadMessage
	for _, m := range resp.Messages {
		if m.Role == "assistant" {
			last = m
		}
	}
	if last == nil || len(last.Blocks) != 2 || last.Blocks[0].GetThinking() == nil {
		t.Fatalf("历史恢复读不到思考块: %+v", last)
	}
}

func TestNoThinkingMeansNoThinkingBlock(t *testing.T) {
	// 不发思考增量(等价于"隐藏推理过程"在 provider 层过滤掉了全部思考事件):
	// 落库只有正文,不制造空思考块。
	p := &thinkingEmitter{}
	srv, th := newThinkingTestServer(t, p)
	runSimpleTurn(t, srv, th.ID)

	blocks := lastAssistantBlocks(t, srv, th.ID)
	if len(blocks) != 1 {
		t.Fatalf("无思考时落库块数 = %d, want 1: %+v", len(blocks), blocks)
	}
	if blocks[0].GetThinking() != nil {
		t.Fatal("不应有空思考块")
	}
}

func TestFailedTurnStillPersistsThinking(t *testing.T) {
	// 失败轮也要留住思考与部分正文:断流恢复靠轮询历史,落库缺失就永远等不到。
	p := &thinkingEmitter{deltas: []string{"推理到一半"}, failText: "部分正文"}
	srv, th := newThinkingTestServer(t, p)
	runSimpleTurn(t, srv, th.ID)

	blocks := lastAssistantBlocks(t, srv, th.ID)
	if len(blocks) != 2 || blocks[0].GetThinking() == nil || blocks[0].GetThinking().GetText() != "推理到一半" {
		t.Fatalf("失败轮的思考未落库: %+v", blocks)
	}
}

func TestTurnThinkingBufConcurrentAdd(t *testing.T) {
	// sink 的调用方可能来自多个 goroutine:逐字累积必须线程安全。
	var buf turnThinkingBuf
	var wg sync.WaitGroup
	const workers, per = 8, 100
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < per; j++ {
				buf.add("x")
			}
		}()
	}
	wg.Wait()
	if got := buf.take(); len(got) != workers*per {
		t.Fatalf("并发累积丢字: %d != %d", len(got), workers*per)
	}
	if got := buf.take(); got != "" {
		t.Fatalf("take 后未清空: %q", got)
	}
}

func TestPrependThinkingBlockEmptyIsNoop(t *testing.T) {
	blocks := []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "a"}}}}
	got := prependThinkingBlock(blocks, "  \n")
	if len(got) != 1 || got[0].GetThinking() != nil {
		t.Fatal("空白思考不应制造思考块")
	}
	// 原样返回同一片底层:空白思考时不做无谓拷贝
	if &got[0] != &blocks[0] {
		t.Fatal("空白思考时应原样返回入参切片")
	}
}
