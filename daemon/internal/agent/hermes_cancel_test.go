// hermes_cancel_test.go —— H1 两件套的行为契约(假客户端注入,不起真进程):
//   - 轮次取消看门狗:ctx 取消 / 静默超时 / 单轮上限三闸都必须先向适配器发
//     session/cancel(假客户端记账钉死)再返回;ctx 取消还要交回已流出的正文;
//   - 审批等待豁免:审批在途时看门狗不得误杀(取消记账保持为空);
//   - turnMu 感知 ctx:前一轮挂死持锁时,排队轮次的"停止"必须即时生效
//     (停止键点不掉的回归钉子);
//   - Steerer:hermes 不支持运行中插话,一律 (false,"",nil) 让 server 退回排队;
//   - usage_update 投影:size→ModelContextWindow、used→Usage.InputTokens。
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/hermesapp"
	"github.com/pocketagent/daemon/internal/store"
)

// 编译期钉死:hermes 声明 Steerer(Steer 恒 false;取消走 Run 的看门狗)。
var _ Steerer = (*HermesProvider)(nil)

// CancelSession 记账进 cancelCalls(字段在本包 hermes_provider_test.go 的
// fakeHermesClient 上;形状与真实 *hermesapp.Client.CancelSession 同签名)。
func (f *fakeHermesClient) CancelSession(_ context.Context, sessionID string) error {
	f.mu.Lock()
	f.cancelCalls = append(f.cancelCalls, sessionID)
	f.mu.Unlock()
	return nil
}

// hermesSlowFake 是"prompt 挂起"的假客户端:Prompt 进入时报号(promptBegin),
// release 关闭才放行;ctx 取消则如实返回(与真实 callCtx 同语义,看门狗等
// prompt goroutine 退出才有意义)。通知/反向请求钩子沿用基础假客户端。
type hermesSlowFake struct {
	*fakeHermesClient
	promptBegin chan string
	release     chan struct{}
	stopped     chan struct{}
	firePerm    bool // Prompt 进入后先发一次 session/request_permission 反向请求
}

func newSlowHermesFake(base *fakeHermesClient) *hermesSlowFake {
	return &hermesSlowFake{
		fakeHermesClient: base,
		promptBegin:      make(chan string),
		release:          make(chan struct{}),
		stopped:          make(chan struct{}),
	}
}

func (f *hermesSlowFake) Prompt(ctx context.Context, sessionID, text string) (*hermesapp.PromptOutcome, error) {
	f.promptBegin <- sessionID
	if f.firePerm {
		f.mu.Lock()
		req := f.reqFn
		f.mu.Unlock()
		if req != nil {
			req(9001, "session/request_permission", json.RawMessage(fakePermParams))
		}
	}
	select {
	case <-f.release:
		return &hermesapp.PromptOutcome{StopReason: "end_turn"}, nil
	case <-ctx.Done():
		close(f.stopped)
		return nil, ctx.Err()
	}
}

// cancelCallsOf 读取假客户端的取消记账(并发安全拷贝)。
func cancelCallsOf(f *fakeHermesClient) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.cancelCalls...)
}

// runHermesCtx 带外部 ctx 的一轮(取消/看门狗测试用),Run 放 goroutine 里跑,
// 返回收场通道(测试侧带界等待 —— "点得掉"本身就是断言)。
func runHermesCtx(p *HermesProvider, ctx context.Context, text string) <-chan struct {
	blocks []pb.ContentBlock
	err    error
} {
	done := make(chan struct {
		blocks []pb.ContentBlock
		err    error
	}, 1)
	go func() {
		blocks, _, err := p.Run(ctx, TurnRequest{
			Thread:      &store.Thread{ID: "t1"},
			InputBlocks: []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: text}}}},
			Workspace:   `C:\ws`,
		}, func(*pb.TurnServerMessage) {}, nil)
		done <- struct {
			blocks []pb.ContentBlock
			err    error
		}{blocks, err}
	}()
	return done
}

// TestHermesRunUserCancelSendsSessionCancel 手机停止帧(ctx 取消)→ 先发
// session/cancel(适配器真停)→ Run 随 ctx 返回并交回已流出的正文。
func TestHermesRunUserCancelSendsSessionCancel(t *testing.T) {
	slow := newSlowHermesFake(newFakeHermesClient(true))
	p := NewHermesProvider()
	p.newClient = func() (hermesClient, error) { return slow, nil }

	ctx, cancel := context.WithCancel(context.Background())
	done := runHermesCtx(p, ctx, "先流正文再停")
	sid := <-slow.promptBegin // Prompt 已挂起,当轮钩子已装

	// 流一段正文(partial 素材;notif 在测试 goroutine 同步执行,睡一拍让它落定)
	slow.mu.Lock()
	notifFn := slow.notifFn
	slow.mu.Unlock()
	if notifFn == nil {
		t.Fatal("当轮通知钩子应在 Prompt 前装好")
	}
	notifFn(json.RawMessage(`{"sessionId":"` + sid + `","update":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"已流出"}}}`))
	time.Sleep(50 * time.Millisecond)

	cancel()
	select {
	case res := <-done:
		if !errors.Is(res.err, context.Canceled) {
			t.Fatalf("取消轮应回 ctx.Err: %v", res.err)
		}
		if len(res.blocks) == 0 || res.blocks[0].GetText().GetText() != "已流出" {
			t.Fatalf("已流出的正文必须交回(partial): %+v", res.blocks)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("取消后 Run 未返回 —— 停止键点不掉回归")
	}

	if cancels := cancelCallsOf(slow.fakeHermesClient); len(cancels) != 1 || cancels[0] != sid {
		t.Fatalf("应向适配器恰发一次 session/cancel(会话号对得上): %v", cancels)
	}
	select {
	case <-slow.stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("prompt goroutine 未随 ctx 退出")
	}
}

// TestHermesRunSilenceWatchdogCancels 静默超时闸:无任何事件超过看门狗窗口 →
// 判挂起,发 session/cancel 后按错误收场(轮次不再无限挂死)。
func TestHermesRunSilenceWatchdogCancels(t *testing.T) {
	oldCap, oldSil, oldTick := hermesTurnCap, hermesSilenceWatchdog, hermesWatchdogTick
	hermesSilenceWatchdog, hermesWatchdogTick, hermesTurnCap = 60*time.Millisecond, 20*time.Millisecond, time.Hour
	t.Cleanup(func() { hermesSilenceWatchdog, hermesWatchdogTick, hermesTurnCap = oldSil, oldTick, oldCap })

	slow := newSlowHermesFake(newFakeHermesClient(true))
	p := NewHermesProvider()
	p.newClient = func() (hermesClient, error) { return slow, nil }

	done := runHermesCtx(p, context.Background(), "挂住别动")
	sid := <-slow.promptBegin
	select {
	case res := <-done:
		if res.err == nil || !strings.Contains(res.err.Error(), "无任何事件") {
			t.Fatalf("静默超时应按看门狗错误收场: %v", res.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("看门狗未收场 —— 挂起轮次无法自愈回归")
	}
	if cancels := cancelCallsOf(slow.fakeHermesClient); len(cancels) != 1 || cancels[0] != sid {
		t.Fatalf("看门狗收场前必须发 session/cancel: %v", cancels)
	}
}

// TestHermesRunTurnCapCancels 单轮上限闸:超上限发取消收场(不陪跑算力)。
func TestHermesRunTurnCapCancels(t *testing.T) {
	oldCap, oldSil, oldTick := hermesTurnCap, hermesSilenceWatchdog, hermesWatchdogTick
	hermesTurnCap, hermesWatchdogTick, hermesSilenceWatchdog = 50*time.Millisecond, 20*time.Millisecond, time.Hour
	t.Cleanup(func() { hermesTurnCap, hermesWatchdogTick, hermesSilenceWatchdog = oldCap, oldTick, oldSil })

	slow := newSlowHermesFake(newFakeHermesClient(true))
	p := NewHermesProvider()
	p.newClient = func() (hermesClient, error) { return slow, nil }

	done := runHermesCtx(p, context.Background(), "跑太久")
	sid := <-slow.promptBegin
	select {
	case res := <-done:
		if res.err == nil || !strings.Contains(res.err.Error(), "轮次超时") {
			t.Fatalf("单轮上限应按超时错误收场: %v", res.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("上限闸未收场")
	}
	if cancels := cancelCallsOf(slow.fakeHermesClient); len(cancels) != 1 || cancels[0] != sid {
		t.Fatalf("上限收场前必须发 session/cancel: %v", cancels)
	}
}

// TestHermesApprovalWaitingExemptsWatchdog 审批在途 = 合法静默:等待时长远超
// 看门狗窗口时不得误杀(取消记账保持为空);审批结束后静默继续,看门狗照常
// 收场(且只发一次取消)。
func TestHermesApprovalWaitingExemptsWatchdog(t *testing.T) {
	oldSil, oldTick := hermesSilenceWatchdog, hermesWatchdogTick
	hermesSilenceWatchdog, hermesWatchdogTick = 40*time.Millisecond, 20*time.Millisecond
	t.Cleanup(func() { hermesSilenceWatchdog, hermesWatchdogTick = oldSil, oldTick })

	slow := newSlowHermesFake(newFakeHermesClient(true))
	slow.firePerm = true
	p := NewHermesProvider()
	p.newClient = func() (hermesClient, error) { return slow, nil }

	noCancelDuringApproval := true
	approve := func(ctx context.Context, _ *pb.ToolCallBlock) (bool, string, error) {
		time.Sleep(150 * time.Millisecond) // ≈4 个看门狗节拍:足够暴露误杀
		noCancelDuringApproval = len(cancelCallsOf(slow.fakeHermesClient)) == 0
		return false, "", nil
	}

	done := make(chan error, 1)
	go func() {
		_, _, err := p.Run(context.Background(), TurnRequest{
			Thread:      &store.Thread{ID: "t1"},
			InputBlocks: []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "need-approval 慢慢想"}}}},
			Workspace:   `C:\ws`,
		}, func(*pb.TurnServerMessage) {}, approve)
		done <- err
	}()
	sid := <-slow.promptBegin
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "无任何事件") {
			t.Fatalf("审批结束后的静默应仍由看门狗收场: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("看门狗未收场")
	}
	if !noCancelDuringApproval {
		t.Fatal("审批等待期间看门狗误杀(发了 session/cancel)")
	}
	if cancels := cancelCallsOf(slow.fakeHermesClient); len(cancels) != 1 || cancels[0] != sid {
		t.Fatalf("收场只应发一次 session/cancel: %v", cancels)
	}
}

// TestHermesRunCancelWhileQueuedOnTurnLock turnMu 感知 ctx:前一轮挂死持锁时,
// 排队轮次的"停止"必须即时生效(锁等待不得吞掉取消),且不触达适配器。
func TestHermesRunCancelWhileQueuedOnTurnLock(t *testing.T) {
	p := NewHermesProvider()
	fake := newFakeHermesClient(true)
	p.newClient = func() (hermesClient, error) { return fake, nil }

	p.turnMu.Lock() // 模拟前一轮挂死持锁(测试内永不主动释放;reaper 接锁即还)
	defer p.turnMu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	done := runHermesCtx(p, ctx, "排队中的第二轮")
	time.Sleep(100 * time.Millisecond) // 等 Run 卡进 turnMu 等待
	cancel()
	select {
	case res := <-done:
		if !errors.Is(res.err, context.Canceled) {
			t.Fatalf("排队轮的取消必须即时生效: %v", res.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("turnMu 等待不感知 ctx —— 停止键点不掉回归")
	}
	if fake.newCalls != 0 {
		t.Fatal("锁都没拿到就不该碰适配器")
	}
}

// TestHermesSteerNotSupported 插话面钉死:恒 (false,"",nil),server 层退回排队。
func TestHermesSteerNotSupported(t *testing.T) {
	p := NewHermesProvider()
	steered, turnID, err := p.Steer(context.Background(), "t1",
		[]pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "插一句"}}}})
	if steered || turnID != "" || err != nil {
		t.Fatalf("hermes 不支持运行中插话,应返回 (false,\"\",nil): %v %q %v", steered, turnID, err)
	}
}

// TestHermesUsageUpdateProjection usage_update 投影形状:窗口=size、占用=used,
// 不伪造输出 token(zcode/dsh 同一条 TurnServerMessage_Usage 通道)。
func TestHermesUsageUpdateProjection(t *testing.T) {
	p := NewHermesProvider()
	var events []*pb.TurnServerMessage
	sink := func(ev *pb.TurnServerMessage) { events = append(events, ev) }
	raw := json.RawMessage(`{"sessionId":"s1","update":{"sessionUpdate":"usage_update","size":128000,"used":4200}}`)
	p.projectSessionUpdate(hermesapp.ParseSessionUpdate(raw), sink, &strings.Builder{}, map[string]int{}, &sync.Mutex{}, &nativeNoticeRecorder{})

	if len(events) != 1 {
		t.Fatalf("usage_update 应投影为一帧, got %d", len(events))
	}
	u := events[0].GetUsage()
	if u == nil {
		t.Fatalf("应为 TurnServerMessage_Usage: %+v", events[0].Msg)
	}
	if u.ModelContextWindow != 128000 {
		t.Fatalf("ModelContextWindow 应取 size: %+v", u)
	}
	usage := u.GetUsage()
	if usage == nil || usage.GetInputTokens() != 4200 {
		t.Fatalf("Usage.InputTokens 应取 used: %+v", usage)
	}
	if usage.GetOutputTokens() != 0 {
		t.Fatalf("hermes 的 usage_update 没有 output 面,不得伪造: %+v", usage)
	}
}
