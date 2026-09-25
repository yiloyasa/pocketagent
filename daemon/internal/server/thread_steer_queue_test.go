// thread_steer_queue_test.go —— P1-4「运行中继续发消息」的 server 层状态机。
//
// 【测什么】从 gRPC 报文一路测到帧序与落库(与 turn_overrides_test.go 同思路:
// 只测纯函数挡不住"调用点被人删掉"):
//  1. echo 型 provider 运行中连发 3 条 → 排队回执(第 1/2 位)→ 终局后按序
//     自动续跑,帧序 Completed → QueueFlushed → Started,消息落库顺序正确;
//  2. 队列上限 10 条:第 11 条起拒收(Queued.position=0),已排队的照常跑完;
//  3. 用户取消 → 队列一起丢,不再续跑("终止任务"的按钮语义);
//  4. provider 支持 Steerer → Steered 回执、不排队;插不进 → 退回排队。
package server

import (
	"context"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// seqProvider 节奏可控的测试 provider:Run 记录输入、通知进入、等放行后返回
// 固定回复 —— 模拟"轮次正在跑,手机这时又发了一条"。
type seqProvider struct {
	id      string
	mu      sync.Mutex
	runs    []string
	entered chan struct{}
	release chan struct{}
	// steer 非空时 provider 实现 Steerer(模拟 codex)
	steer func(threadID string, blocks []pb.ContentBlock) (bool, string, error)
	// followUp 非空时 provider 的 FollowUp 投递成功(模拟 pi 原生 followUp 队列);
	// nil = 投递失败(返回 false),server 应回落 daemon 内存队列 —— 与 Steer
	// 字段同款"方法总在、语义由字段定"的打桩模式。
	followUp func(ctx context.Context, threadID string, blocks []pb.ContentBlock) (bool, error)
	// followUps 是 FollowUp 被调用次数的打桩计数(受 mu 保护,与 runs 同锁)。
	followUps int
}

func (p *seqProvider) ID() string             { return p.id }
func (p *seqProvider) DisplayName() string    { return "测试 provider" }
func (p *seqProvider) Models() []pb.ModelInfo { return nil }

func (p *seqProvider) Run(ctx context.Context, req agent.TurnRequest, sink agent.Sink, _ agent.ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
	text := firstUserText(req.InputBlocks)
	p.mu.Lock()
	p.runs = append(p.runs, text)
	p.mu.Unlock()
	p.entered <- struct{}{}
	select {
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	case <-p.release:
	}
	reply := "回复:" + text
	sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_TextDelta{TextDelta: &pb.TextDelta{Delta: reply}}})
	return []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: reply}}}}, &pb.Usage{}, nil
}

// Steer 使 seqProvider 实现 agent.Steerer(仅当 steer 非空时被 server 询问;
// 接口方法本身总是存在,返回"不支持"语义由 steer 字段决定)。
func (p *seqProvider) Steer(ctx context.Context, threadID string, blocks []pb.ContentBlock) (bool, string, error) {
	if p.steer == nil {
		return false, "", context.DeadlineExceeded
	}
	return p.steer(threadID, blocks)
}

func (p *seqProvider) runsSoFar() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.runs...)
}

// feedTurnStream 通道式 StartTurn 流:测试往 in 里推帧模拟手机边跑边发;
// close(in) = 手机结束发送(Recv 返回 io.EOF)。
type feedTurnStream struct {
	grpc.ServerStream
	ctx  context.Context
	in   chan *pb.TurnClientMessage
	mu   sync.Mutex
	sent []*pb.TurnServerMessage
	recv atomic.Int32 // 已被读手取走的帧数(同步"取消帧已被处理"用)
}

func (f *feedTurnStream) Recv() (*pb.TurnClientMessage, error) {
	m, ok := <-f.in
	f.recv.Add(1)
	if !ok {
		return nil, io.EOF
	}
	return m, nil
}

// received 读手已消费的帧数。Cancel 帧的处理在 Recv 返回后紧接执行
// (同一 goroutine),等计数到位即可确定 streamCancel 已发生。
func (f *feedTurnStream) received() int { return int(f.recv.Load()) }

func (f *feedTurnStream) Send(m *pb.TurnServerMessage) error {
	f.mu.Lock()
	f.sent = append(f.sent, m)
	f.mu.Unlock()
	return nil
}

func (f *feedTurnStream) Context() context.Context { return f.ctx }

func (f *feedTurnStream) snapshot() []*pb.TurnServerMessage {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]*pb.TurnServerMessage(nil), f.sent...)
}

func (f *feedTurnStream) push(t *testing.T, m *pb.TurnClientMessage) {
	t.Helper()
	select {
	case f.in <- m:
	case <-time.After(5 * time.Second):
		t.Fatal("推帧超时(读手没在收帧?)")
	}
}

// textInput 造一条 TurnInput 帧。
func textInput(threadID, text string) *pb.TurnClientMessage {
	return &pb.TurnClientMessage{Msg: &pb.TurnClientMessage_Input{Input: &pb.TurnInput{
		ThreadId: threadID,
		Blocks:   []*pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: text}}}},
	}}}
}

// waitFor 轮询等待条件成立(事件顺序全在 stream.sent 里,轮询最直白)。
func waitFor(t *testing.T, desc string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("等待超时: %s", desc)
}

// newSteerTestServer 造一个带测试 provider 与已存在线程的 server。
func newSteerTestServer(t *testing.T, id string, p *seqProvider) (*Server, *store.Thread) {
	t.Helper()
	fs, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("建测试存储失败: %v", err)
	}
	srv := New(Config{Store: fs, Providers: map[string]agent.Provider{id: p}})
	th := &store.Thread{
		ID: "th-q", Title: "排队测试", Workspace: t.TempDir(),
		ModelID: id + "/m", Status: pb.ThreadStatus_THREAD_STATUS_IDLE,
	}
	if err := fs.Create(th); err != nil {
		t.Fatalf("建线程失败: %v", err)
	}
	return srv, th
}

// countFrame 数某种终局/回执帧的个数。
func countFrame(frames []*pb.TurnServerMessage, f func(*pb.TurnServerMessage) bool) int {
	n := 0
	for _, m := range frames {
		if f(m) {
			n++
		}
	}
	return n
}

// TestMidTurnInputsQueueAndFlushInOrder 主路径:运行中连发 2 条 → 依次排队回执
// → 当前轮终局后按序自动续跑 → 三条消息按序落库。
func TestMidTurnInputsQueueAndFlushInOrder(t *testing.T) {
	p := &seqProvider{id: "echo", entered: make(chan struct{}, 16), release: make(chan struct{}, 16)}
	srv, th := newSteerTestServer(t, "echo", p)
	stream := &feedTurnStream{ctx: context.Background(), in: make(chan *pb.TurnClientMessage, 8)}
	stream.push(t, textInput(th.ID, "m1"))

	done := make(chan error, 1)
	go func() { done <- srv.StartTurn(stream) }()

	// 第一轮已进入 Run
	<-p.entered
	// 运行中再发两条
	stream.push(t, textInput(th.ID, "m2"))
	stream.push(t, textInput(th.ID, "m3"))
	waitFor(t, "两条 Queued 回执", func() bool {
		return countFrame(stream.snapshot(), func(m *pb.TurnServerMessage) bool {
			q := m.GetQueued()
			return q != nil
		}) == 2
	})
	frames := stream.snapshot()
	pos := make([]int32, 0, 2)
	for _, m := range frames {
		if q := m.GetQueued(); q != nil {
			pos = append(pos, q.Position)
		}
	}
	if len(pos) != 2 || pos[0] != 1 || pos[1] != 2 {
		t.Fatalf("排队回执位次 = %v, want [1 2]", pos)
	}

	// 依次放行三轮(m1 → 续跑 m2 → 续跑 m3)
	for i := 0; i < 3; i++ {
		p.release <- struct{}{}
		// 每放行一轮,若还有队列则等下一次进入 Run(第三轮没有队列,不等)
		if i < 2 {
			waitFor(t, "续跑轮进入 Run", func() bool { return len(p.runsSoFar()) > i+1 })
		}
	}
	close(stream.in)
	if err := <-done; err != nil {
		t.Fatalf("StartTurn: %v", err)
	}

	// 执行顺序:排队输入严格按发送顺序续跑
	runs := p.runsSoFar()
	if len(runs) != 3 || runs[0] != "m1" || runs[1] != "m2" || runs[2] != "m3" {
		t.Fatalf("执行顺序 = %v, want [m1 m2 m3]", runs)
	}
	// 帧序:3 个 Completed,2 个 QueueFlushed(每条排队输入一个),3 个 Started
	frames = stream.snapshot()
	if n := countFrame(frames, func(m *pb.TurnServerMessage) bool { return m.GetCompleted() != nil }); n != 3 {
		t.Fatalf("Completed 帧数 = %d, want 3", n)
	}
	if n := countFrame(frames, func(m *pb.TurnServerMessage) bool { return m.GetQueueFlushed() != nil }); n != 2 {
		t.Fatalf("QueueFlushed 帧数 = %d, want 2", n)
	}
	if n := countFrame(frames, func(m *pb.TurnServerMessage) bool { return m.GetStarted() != nil }); n != 3 {
		t.Fatalf("Started 帧数 = %d, want 3(含续跑轮)", n)
	}
	// QueueFlushed 紧跟在上一轮 Completed 之后、本轮 Started 之前
	for i, m := range frames {
		if m.GetQueueFlushed() != nil {
			if i == 0 || frames[i-1].GetCompleted() == nil {
				t.Fatalf("QueueFlushed 前一帧应是 Completed,帧序: %s", frameNames(frames))
			}
		}
	}
	// 落库:3 条 user + 3 条 assistant,顺序与执行一致
	got, _ := srv.cfg.Store.Get(th.ID)
	var roles []string
	for _, m := range got.Messages {
		roles = append(roles, m.Role)
	}
	if strings.Join(roles, ",") != "user,assistant,user,assistant,user,assistant" {
		t.Fatalf("落库消息序列 = %v", roles)
	}
	if firstUserText(got.Messages[2].Blocks) != "m2" || firstUserText(got.Messages[4].Blocks) != "m3" {
		t.Fatal("排队消息落库顺序与执行顺序不一致")
	}
}

// frameNames 调试辅助:把帧序列打成人话(失败信息里用)。
func frameNames(frames []*pb.TurnServerMessage) string {
	names := make([]string, 0, len(frames))
	for _, m := range frames {
		switch {
		case m.GetStarted() != nil:
			names = append(names, "Started")
		case m.GetTextDelta() != nil:
			names = append(names, "TextDelta")
		case m.GetCompleted() != nil:
			names = append(names, "Completed")
		case m.GetFailed() != nil:
			names = append(names, "Failed")
		case m.GetQueued() != nil:
			names = append(names, "Queued")
		case m.GetQueueFlushed() != nil:
			names = append(names, "QueueFlushed")
		case m.GetSteered() != nil:
			names = append(names, "Steered")
		default:
			names = append(names, "?")
		}
	}
	return strings.Join(names, "→")
}

// TestMidTurnInputQueueLimit 队列上限:前 10 条给位次,第 11 条起拒收;
// 已排队的照常逐条跑完。
func TestMidTurnInputQueueLimit(t *testing.T) {
	p := &seqProvider{id: "echo", entered: make(chan struct{}, 16), release: make(chan struct{}, 16)}
	srv, th := newSteerTestServer(t, "echo", p)
	stream := &feedTurnStream{ctx: context.Background(), in: make(chan *pb.TurnClientMessage, 16)}
	stream.push(t, textInput(th.ID, "m0"))
	done := make(chan error, 1)
	go func() { done <- srv.StartTurn(stream) }()

	<-p.entered
	const extra = 12
	for i := 0; i < extra; i++ {
		stream.push(t, textInput(th.ID, "mx"+string(rune('a'+i))))
	}
	waitFor(t, "12 条回执", func() bool {
		return countFrame(stream.snapshot(), func(m *pb.TurnServerMessage) bool { return m.GetQueued() != nil }) == extra
	})
	var accepted, rejected int
	var lastDepth int32
	for _, m := range stream.snapshot() {
		if q := m.GetQueued(); q != nil {
			if q.Position == 0 {
				rejected++
			} else {
				accepted++
				lastDepth = q.Depth
			}
		}
	}
	if accepted != 10 || rejected != 2 {
		t.Fatalf("接受 %d / 拒收 %d, want 10 / 2(上限 %d)", accepted, rejected, turnQueueLimit)
	}
	if lastDepth != 10 {
		t.Fatalf("最后一条排队回执的 depth = %d, want 10", lastDepth)
	}
	// 放行第一轮 + 10 条排队轮
	for i := 0; i < 11; i++ {
		p.release <- struct{}{}
		if i < 10 {
			waitFor(t, "续跑轮进入", func() bool { return len(p.runsSoFar()) > i+1 })
		}
	}
	close(stream.in)
	if err := <-done; err != nil {
		t.Fatalf("StartTurn: %v", err)
	}
	if n := len(p.runsSoFar()); n != 11 {
		t.Fatalf("执行轮数 = %d, want 11(1 + 排队 10)", n)
	}
	if n := srv.turnQueue.Len(th.ID); n != 0 {
		t.Fatalf("队列应已清空,剩 %d", n)
	}
}

// TestMidTurnCancelDropsQueue 取消 = 连排队的消息一起停。
func TestMidTurnCancelDropsQueue(t *testing.T) {
	p := &seqProvider{id: "echo", entered: make(chan struct{}, 16), release: make(chan struct{}, 16)}
	srv, th := newSteerTestServer(t, "echo", p)
	stream := &feedTurnStream{ctx: context.Background(), in: make(chan *pb.TurnClientMessage, 8)}
	stream.push(t, textInput(th.ID, "m1"))
	done := make(chan error, 1)
	go func() { done <- srv.StartTurn(stream) }()

	<-p.entered
	stream.push(t, textInput(th.ID, "m2"))
	waitFor(t, "Queued 回执", func() bool {
		return countFrame(stream.snapshot(), func(m *pb.TurnServerMessage) bool { return m.GetQueued() != nil }) == 1
	})
	// 用户点"终止任务":等读手真正消费了取消帧(共 3 帧)再放行,
	// 否则"轮次先完成、取消后到"的合法竞态会让队列多跑一轮。
	stream.push(t, &pb.TurnClientMessage{Msg: &pb.TurnClientMessage_Cancel{Cancel: &pb.CancelSignal{Reason: "测试取消"}}})
	waitFor(t, "取消帧被读手消费", func() bool { return stream.received() >= 3 })
	p.release <- struct{}{} // 让第一轮退出(ctx 已取消,provider 返回 ctx.Err)
	close(stream.in)
	if err := <-done; err != nil {
		t.Fatalf("StartTurn: %v", err)
	}
	frames := stream.snapshot()
	if n := countFrame(frames, func(m *pb.TurnServerMessage) bool { return m.GetQueueFlushed() != nil }); n != 0 {
		t.Fatalf("取消后不应续跑队列,QueueFlushed = %d", n)
	}
	if n := len(p.runsSoFar()); n != 1 {
		t.Fatalf("执行轮数 = %d, want 1(排队的 m2 必须不跑)", n)
	}
	if n := srv.turnQueue.Len(th.ID); n != 0 {
		t.Fatalf("取消后队列应清空,剩 %d", n)
	}
	// 收尾帧两种都合法:provider 恰好先拿到放行 → Completed;先感知取消 → Failed。
	// 关键断言在上面:队列没续跑、执行只有一轮。
	nEnd := countFrame(frames, func(m *pb.TurnServerMessage) bool {
		return m.GetFailed() != nil || m.GetCompleted() != nil
	})
	if nEnd != 1 {
		t.Fatalf("应有且只有一个终局帧,得到 %d(帧序: %s)", nEnd, frameNames(frames))
	}
}

// TestMidTurnInputSteeredThenFallsBack codex 型 provider:能插 → Steered 回执、
// 不排队;插不进(轮次未开始等)→ 退回排队并在终局后执行。
func TestMidTurnInputSteeredThenFallsBack(t *testing.T) {
	p := &seqProvider{
		id: "codex", entered: make(chan struct{}, 16), release: make(chan struct{}, 16),
		steer: func(threadID string, blocks []pb.ContentBlock) (bool, string, error) {
			if firstUserText(blocks) == "插入我" {
				return true, "codex-turn-77", nil
			}
			// codex 拒绝(模拟 expectedTurnId 不匹配 / 轮次未开始)
			return false, "", context.DeadlineExceeded
		},
	}
	srv, th := newSteerTestServer(t, "codex", p)
	stream := &feedTurnStream{ctx: context.Background(), in: make(chan *pb.TurnClientMessage, 8)}
	stream.push(t, textInput(th.ID, "m1"))
	done := make(chan error, 1)
	go func() { done <- srv.StartTurn(stream) }()

	<-p.entered
	// 插入成功:Steered 回执
	stream.push(t, textInput(th.ID, "插入我"))
	waitFor(t, "Steered 回执", func() bool {
		return countFrame(stream.snapshot(), func(m *pb.TurnServerMessage) bool { return m.GetSteered() != nil }) == 1
	})
	for _, m := range stream.snapshot() {
		if s := m.GetSteered(); s != nil {
			if s.CodexTurnId != "codex-turn-77" || s.TurnId == "" {
				t.Fatalf("Steered 帧 = %+v, want codex turn id 与当前轮次 id", s)
			}
		}
	}
	// 插不进:退回排队
	stream.push(t, textInput(th.ID, "排我"))
	waitFor(t, "Queued 回执", func() bool {
		return countFrame(stream.snapshot(), func(m *pb.TurnServerMessage) bool { return m.GetQueued() != nil }) == 1
	})
	// 放行:第一轮 + 1 条排队
	p.release <- struct{}{}
	waitFor(t, "续跑轮进入", func() bool { return len(p.runsSoFar()) > 1 })
	p.release <- struct{}{}
	close(stream.in)
	if err := <-done; err != nil {
		t.Fatalf("StartTurn: %v", err)
	}
	runs := p.runsSoFar()
	if len(runs) != 2 || runs[0] != "m1" || runs[1] != "排我" {
		t.Fatalf("执行序列 = %v, want [m1 排我](插入的不新开轮次)", runs)
	}
	frames := stream.snapshot()
	if n := countFrame(frames, func(m *pb.TurnServerMessage) bool { return m.GetQueueFlushed() != nil }); n != 1 {
		t.Fatalf("QueueFlushed = %d, want 1(仅排队的那条)", n)
	}
	if n := countFrame(frames, func(m *pb.TurnServerMessage) bool { return m.GetSteered() != nil }); n != 1 {
		t.Fatalf("Steered = %d, want 1", n)
	}
}
