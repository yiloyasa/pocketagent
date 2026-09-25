// pi_followup_test.go —— pi followUp 原生投递的行为契约(fake 客户端,红灯先行)。
//
// 【三场景】(任务钉子,证据 docs/verify/evidence/pi-followup-wire-probe.md)
//  1. 投递带 streamingBehavior:FollowUp 走 Prompt 封装且 behavior=followUp;
//  2. (server 层钉摘除,见 thread_followup_queue_test.go);
//  3. 失败回落:上游拒绝/无运行轮次/settled 后 → delivered=false,调用方回落
//     daemon 队列(既有语义不变)。
//
// 另钉 queue_update 事件 → Queued/QueueFlushed 回执链的对账函数(入队 +1、
// 出队 −1,steering 变化不产生帧)。
package agent

import (
	"context"
	"errors"
	"testing"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// fakePromptRPC promptRPC 打桩:记录外发参数;err 非空时按错误应答。
type fakePromptRPC struct {
	gotMessage  string
	gotImages   []any
	gotBehavior string
	err         error
}

func (f *fakePromptRPC) Prompt(_ context.Context, message string, images []any, behavior string) error {
	f.gotMessage, f.gotImages, f.gotBehavior = message, images, behavior
	return f.err
}

func textBlocks(s string) []pb.ContentBlock {
	return []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: s}}}}
}

// markActive 把线程标成"有运行中轮次"(FollowUp 的守卫前置条件,与 Run 同款登记)。
func (p *PiProvider) markActive(threadID string) func() {
	p.steerMu.Lock()
	p.active[threadID] = true
	p.steerMu.Unlock()
	return func() {
		p.steerMu.Lock()
		delete(p.active, threadID)
		p.steerMu.Unlock()
	}
}

// TestPiFollowUpDeliversWithStreamingBehavior 场景 1:投递必须带 followUp 语义,
// 输入块转写(prompt 文本 + 图片)与既有 blocksToPrompt 同一实现。
func TestPiFollowUpDeliversWithStreamingBehavior(t *testing.T) {
	p := NewPiProvider()
	defer p.markActive("th-fu")()
	fake := &fakePromptRPC{}
	// 假客户端不经 aliveClient:直接打 followUpOn(窄接口),钉"外发形状"。
	delivered, err := p.followUpOn(context.Background(), fake, "th-fu", textBlocks("忙完了再跑这条"))
	if err != nil || !delivered {
		t.Fatalf("投递应成功: delivered=%v err=%v", delivered, err)
	}
	if fake.gotMessage != "忙完了再跑这条" {
		t.Fatalf("message 应来自输入块转写: %q", fake.gotMessage)
	}
	if fake.gotBehavior != "followUp" {
		t.Fatalf("必须投成 followUp(pi 流式期间缺 streamingBehavior 直接拒收): %q", fake.gotBehavior)
	}
}

// TestPiFollowUpFailureFallsBack 场景 3:上游拒绝 → delivered=false 原样上抛,
// 调用方据此回落 daemon 内存队列,输入不丢。
func TestPiFollowUpFailureFallsBack(t *testing.T) {
	p := NewPiProvider()
	fake := &fakePromptRPC{err: errors.New("pi 子进程已退出")}
	delivered, err := p.followUpOn(context.Background(), fake, "th-fu", textBlocks("x"))
	if delivered || err == nil {
		t.Fatalf("失败应 delivered=false 且带错误: delivered=%v err=%v", delivered, err)
	}
}

// TestPiFollowUpGuards 无运行轮次 / 本轮已 settled → 拒投(pi 空闲时 prompt 会
// 立即执行成新轮次,不是 followUp 语义 —— 证据 §1"空闲忽略 streamingBehavior")。
func TestPiFollowUpGuards(t *testing.T) {
	p := NewPiProvider()

	// 无运行轮次:直接拒。
	if delivered, _ := p.FollowUp(context.Background(), "th-g", textBlocks("x")); delivered {
		t.Fatal("无运行轮次应拒投")
	}
	// 有运行轮次但已见过 settled(终局收尾窗口):拒投,交 daemon 队列兜底。
	defer p.markActive("th-g")()
	p.steerMu.Lock()
	p.settled["th-g"] = true
	p.steerMu.Unlock()
	if delivered, _ := p.FollowUp(context.Background(), "th-g", textBlocks("x")); delivered {
		t.Fatal("settled 后应拒投(pi 已空闲,投递会立即执行)")
	}
}

// TestReconcileNativeQueue 队列对账:queue_update 的 followUp 数组是**完整当前
// 队列**(证据 §2)—— 入队(变长)发 Queued(+1),出队(变短)发 QueueFlushed(−1);
// steering 变化不产生帧;位次/深度取 followUp 队列自身长度。
func TestReconcileNativeQueue(t *testing.T) {
	p := NewPiProvider()
	const th = "th-q"
	frames := p.reconcileNativeQueue(th, []byte(`{"steering":[],"followUp":["a"]}`))
	if len(frames) != 1 {
		t.Fatalf("0→1 应发 1 帧 Queued 回执,得 %d", len(frames))
	}
	if q := frames[0].GetQueued(); q == nil || q.Position != 1 || q.Depth != 1 {
		t.Fatalf("入队回执 = %+v, want Position=1 Depth=1", frames[0].GetQueued())
	}

	// steering 入队/出队:followUp 不变 → 零帧(不污染手机排队计数)。
	if frames := p.reconcileNativeQueue(th, []byte(`{"steering":["s"],"followUp":["a"]}`)); len(frames) != 0 {
		t.Fatalf("steering 变化不应产生帧,得 %d", len(frames))
	}

	// 再入队一条:0 增量帧?不 —— 高水位 1→2,发 1 帧 Queued(第 2 位)。
	frames = p.reconcileNativeQueue(th, []byte(`{"steering":[],"followUp":["a","b"]}`))
	if len(frames) != 1 {
		t.Fatalf("1→2 应发 1 帧,得 %d", len(frames))
	}
	if q := frames[0].GetQueued(); q == nil || q.Position != 2 || q.Depth != 2 {
		t.Fatalf("第二条入队回执 = %+v, want Position=2 Depth=2", frames[0].GetQueued())
	}

	// 出队一条(pi 取走队首续跑):发 QueueFlushed,Remaining=1。
	frames = p.reconcileNativeQueue(th, []byte(`{"steering":[],"followUp":["b"]}`))
	if len(frames) != 1 {
		t.Fatalf("2→1 应发 1 帧 QueueFlushed,得 %d", len(frames))
	}
	if qf := frames[0].GetQueueFlushed(); qf == nil || qf.Remaining != 1 {
		t.Fatalf("出队回执 = %+v, want Remaining=1", frames[0].GetQueueFlushed())
	}

	// 全部出队:1 帧 QueueFlushed(Remaining=0)。
	frames = p.reconcileNativeQueue(th, []byte(`{"steering":[],"followUp":[]}`))
	if len(frames) != 1 || frames[0].GetQueueFlushed() == nil || frames[0].GetQueueFlushed().Remaining != 0 {
		t.Fatalf("清空应发 1 帧 QueueFlushed(0),得 %v", frames)
	}

	// 脏报文:不炸、不产生帧(宁可少收账,不伪造)。
	if frames := p.reconcileNativeQueue(th, []byte(`not-json`)); len(frames) != 0 {
		t.Fatalf("脏报文应零帧,得 %d", len(frames))
	}
	// 清空后的重复空事件:零帧(水位不动,不重复回执)。
	if frames := p.reconcileNativeQueue(th, []byte(`{"steering":[],"followUp":[]}`)); len(frames) != 0 {
		t.Fatalf("空队列重复事件应零帧,得 %d", len(frames))
	}
}

// TestFollowUpperCompilePin 编译期钉死:PiProvider 声明 FollowUpper 能力
// (与 QueueClearer 的 var _ 同款;server 层靠类型断言探测,声明漂移在此报警)。
func TestFollowUpperCompilePin(t *testing.T) {
	var _ FollowUpper = (*PiProvider)(nil)
}
