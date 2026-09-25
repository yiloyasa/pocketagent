// thread_followup_queue_test.go —— pi followUp 原生投递的 server 层接线
// (排队投递点的三态:原生预投递优先 → daemon 内存队列兜底)。
//
// 【钉什么】(差距分析 P2「follow_up 原生投递」,证据 pi-followup-wire-probe.md)
//  1. 成功后摘除:provider FollowUp 投递成功 → 输入**不进** daemon 内存队列
//     (无续跑、无 QueueFlushed、队列 Len=0;pi 侧在当前轮 settled 周期内
//     自动续跑,Queued/QueueFlushed 回执由 queue_update 事件链补齐);
//  2. 失败回落:投递失败 → 与既有行为零差异(排队回执 + 终局后续跑);
//  3. 不支持回落:provider 未声明能力(假 provider 的投递钩子为 nil)→ 同兜底。
package server

import (
	"context"
	"testing"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// FollowUp 使 seqProvider 实现 agent.FollowUpper(钩子为 nil 时按"投递失败"
// 语义返回,server 应回落 daemon 队列 —— 与本文件 Steer 的打桩模式一致)。
func (p *seqProvider) FollowUp(ctx context.Context, threadID string, blocks []pb.ContentBlock) (bool, error) {
	p.mu.Lock()
	p.followUps++
	p.mu.Unlock()
	if p.followUp == nil {
		return false, context.DeadlineExceeded
	}
	return p.followUp(ctx, threadID, blocks)
}

// deliveries 记录 FollowUp 被调用的次数(打桩计数,定义紧随 FollowUp)。

// TestMidTurnInputFollowUpDeliveredSkipsDaemonQueue 场景 1:投递成功 → 不进
// daemon 队列、不续跑、无 QueueFlushed;server 自身也不自发造 Queued 回执
// (回执由 pi 的 queue_update 事件链映射,见 agent 层对账单测)。
func TestMidTurnInputFollowUpDeliveredSkipsDaemonQueue(t *testing.T) {
	p := &seqProvider{
		id: "pi", entered: make(chan struct{}, 16), release: make(chan struct{}, 16),
		followUp: func(ctx context.Context, threadID string, blocks []pb.ContentBlock) (bool, error) {
			return true, nil // pi 侧受理成功(进入 pi 原生 followUp 队列)
		},
	}
	srv, th := newSteerTestServer(t, "pi", p)
	stream := &feedTurnStream{ctx: context.Background(), in: make(chan *pb.TurnClientMessage, 8)}
	stream.push(t, textInput(th.ID, "m1"))
	done := make(chan error, 1)
	go func() { done <- srv.StartTurn(stream) }()
	<-p.entered

	// 轮次进行中再发一条 → 预投递为 followUp
	stream.push(t, textInput(th.ID, "忙完了再跑这条"))
	waitFor(t, "FollowUp 已被调用", func() bool { return p.deliveries() == 1 })

	// 放行首轮收尾:不允许出现续跑轮 / QueueFlushed
	p.release <- struct{}{}
	waitFor(t, "首轮结束", func() bool {
		return countFrame(stream.snapshot(), func(m *pb.TurnServerMessage) bool { return m.GetCompleted() != nil }) == 1
	})
	frames := stream.snapshot()
	if n := countFrame(frames, func(m *pb.TurnServerMessage) bool { return m.GetQueueFlushed() != nil }); n != 0 {
		t.Fatalf("followUp 预投递不应触发 daemon 续跑,QueueFlushed = %d(帧序: %s)", n, frameNames(frames))
	}
	if n := len(p.runsSoFar()); n != 1 {
		t.Fatalf("执行轮数 = %d, want 1(输入由 pi 原生队列接手)", n)
	}
	if n := srv.turnQueue.Len(th.ID); n != 0 {
		t.Fatalf("daemon 队列应保持为空(该条已摘除/从未入队),剩 %d", n)
	}
	// server 自身不发 Queued(位次真相在 pi 的 queue_update 事件里)
	if n := countFrame(frames, func(m *pb.TurnServerMessage) bool { return m.GetQueued() != nil }); n != 0 {
		t.Fatalf("投递成功路径 server 不自发 Queued(由 queue_update 链映射),得 %d", n)
	}
	close(stream.in)
	if err := <-done; err != nil {
		t.Fatalf("StartTurn: %v", err)
	}
}

// deliveries 记录 FollowUp 被调用的次数(打桩计数)。
func (p *seqProvider) deliveries() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.followUps
}

// TestMidTurnInputFollowUpFailureFallsBack 场景 2:投递失败 → 回落 daemon 队列,
// 回执与续跑和既有语义逐帧一致(Queued → Completed → QueueFlushed → Started)。
func TestMidTurnInputFollowUpFailureFallsBack(t *testing.T) {
	p := &seqProvider{
		id: "pi", entered: make(chan struct{}, 16), release: make(chan struct{}, 16),
		followUp: func(ctx context.Context, threadID string, blocks []pb.ContentBlock) (bool, error) {
			return false, context.DeadlineExceeded // 轮次刚好终局/pi 拒收
		},
	}
	srv, th := newSteerTestServer(t, "pi", p)
	stream := &feedTurnStream{ctx: context.Background(), in: make(chan *pb.TurnClientMessage, 8)}
	stream.push(t, textInput(th.ID, "m1"))
	done := make(chan error, 1)
	go func() { done <- srv.StartTurn(stream) }()
	<-p.entered

	stream.push(t, textInput(th.ID, "排我"))
	waitFor(t, "Queued 回执(回落排队)", func() bool {
		return countFrame(stream.snapshot(), func(m *pb.TurnServerMessage) bool { return m.GetQueued() != nil }) == 1
	})
	// 放行首轮 + 续跑轮
	p.release <- struct{}{}
	waitFor(t, "续跑轮进入", func() bool { return len(p.runsSoFar()) > 1 })
	p.release <- struct{}{}
	close(stream.in)
	if err := <-done; err != nil {
		t.Fatalf("StartTurn: %v", err)
	}
	runs := p.runsSoFar()
	if len(runs) != 2 || runs[0] != "m1" || runs[1] != "排我" {
		t.Fatalf("回落应按既有语义续跑: %v, want [m1 排我]", runs)
	}
	frames := stream.snapshot()
	if n := countFrame(frames, func(m *pb.TurnServerMessage) bool { return m.GetQueueFlushed() != nil }); n != 1 {
		t.Fatalf("回落路径 QueueFlushed = %d, want 1", n)
	}
	if n := srv.turnQueue.Len(th.ID); n != 0 {
		t.Fatalf("回落条目续跑后队列应清空,剩 %d", n)
	}
}

// TestMidTurnInputFollowUpUnsupportedFallsBack 场景 3:provider 未声明该能力
// (投递钩子 nil = 投递失败)→ 与今天完全一致地排队续跑。
func TestMidTurnInputFollowUpUnsupportedFallsBack(t *testing.T) {
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
	p.release <- struct{}{}
	waitFor(t, "续跑轮进入", func() bool { return len(p.runsSoFar()) > 1 })
	p.release <- struct{}{}
	close(stream.in)
	if err := <-done; err != nil {
		t.Fatalf("StartTurn: %v", err)
	}
	if n := len(p.runsSoFar()); n != 2 {
		t.Fatalf("不支持 followUp 的 provider 应保持既有排队续跑: %v", p.runsSoFar())
	}
	if n := countFrame(stream.snapshot(), func(m *pb.TurnServerMessage) bool { return m.GetQueueFlushed() != nil }); n != 1 {
		t.Fatal("既有续跑帧序不得被本特性改变")
	}
}

// 编译期钉:seqProvider(带 FollowUp 方法)满足 server 探测的扩展能力接口。
var _ agent.FollowUpper = (*seqProvider)(nil)
