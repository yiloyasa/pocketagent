// hermes_compact_test.go —— 「压缩上下文」接线的行为契约(假客户端注入,不起真进程):
//   - 成功路径:下发 /compress + 会话 id,回执正文在场 → 返回 nil,钩子随收场卸载;
//   - 闸②:没落地的会话拒绝,且不碰适配器、不建会话(不静默新开);
//   - 闸①:轮次在跑 / 另一操作持轮次锁 → 忙拒绝(不排队、不互拆钩子);
//   - 失败透传:传输错误 / refusal / 回执空(stopReason 空或正文空)一律如实报错,
//     绝不伪造成功;
//   - 预算耗尽:先发 session/cancel 再报错,不留悬空压缩轮;
//   - 审批失败安全:压缩轮的 request_permission 回 -32000(approve=nil),绝不代答。
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/hermesapp"
	"github.com/pocketagent/daemon/internal/store"
)

// hermesHandlersOf 并发安全读出假客户端当前装着的钩子(卸载断言用)。
func hermesHandlersOf(f *fakeHermesClient) (func(json.RawMessage), func(int64, string, json.RawMessage)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.notifFn, f.reqFn
}

// compactTestThread 造一个"已落地"的 hermes 线程(内存引用在场 = 本进程跑过)。
func compactTestThread(p *HermesProvider) *store.Thread {
	th := &store.Thread{ID: "t-compact", Workspace: `C:\ws`}
	p.setThreadRef(th.ID, hermesThreadRef{sessionID: "cached-sess"})
	return th
}

// TestHermesCompactThreadSendsSlashCommand 成功路径:下发 /compress 到已缓存
// 会话,回执正文在场 → nil;收场后钩子必须卸载(不给下一轮留残留)。
func TestHermesCompactThreadSendsSlashCommand(t *testing.T) {
	fake := newFakeHermesClient(true)
	p := newHermesTestProvider(t, fake)
	th := compactTestThread(p)

	if err := p.CompactThread(th); err != nil {
		t.Fatalf("压缩应成功: %v", err)
	}
	if fake.promptCalls != 1 || fake.promptTexts[0] != hermesCompressCommand {
		t.Fatalf("应下发一次 %q: calls=%d texts=%v", hermesCompressCommand, fake.promptCalls, fake.promptTexts)
	}
	if fake.promptSids[0] != "cached-sess" {
		t.Fatalf("应压到缓存会话,得到 %q", fake.promptSids[0])
	}
	if n, r := hermesHandlersOf(fake); n != nil || r != nil {
		t.Fatal("压缩收场后当轮钩子必须卸载(否则下一轮帧会串进压缩上下文)")
	}
	if fake.newCalls != 0 {
		t.Fatalf("缓存命中路径不该新建会话: newCalls=%d", fake.newCalls)
	}
}

// TestHermesCompactThreadRejectsNotLanded 闸②:没落地的会话拒绝,且不碰适配器、
// 不建会话 —— 没东西可压时绝不静默新开一个空会话。
func TestHermesCompactThreadRejectsNotLanded(t *testing.T) {
	fake := newFakeHermesClient(true)
	p := newHermesTestProvider(t, fake)
	th := &store.Thread{ID: "t-cold", Workspace: `C:\ws`}

	err := p.CompactThread(th)
	if err == nil || !strings.Contains(err.Error(), "还没有在电脑上落地") {
		t.Fatalf("未落地会话应拒绝并给可读文案: %v", err)
	}
	if fake.promptCalls != 0 || fake.newCalls != 0 {
		t.Fatalf("拒绝路径不该碰适配器: prompt=%d new=%d", fake.promptCalls, fake.newCalls)
	}
}

// TestHermesCompactThreadRejectsBusy 闸①:轮次在跑(turnActive)或另一操作持有
// 轮次互斥面(turnMu)都必须忙拒绝 —— /compress 是 mid-turn blocked 命令,且
// 钩子是 client 级的,并发会互拆对方的钩子。忙态走 ErrBusy 分级。
func TestHermesCompactThreadRejectsBusy(t *testing.T) {
	t.Run("轮次在跑", func(t *testing.T) {
		fake := newFakeHermesClient(true)
		p := newHermesTestProvider(t, fake)
		th := compactTestThread(p)
		p.mu.Lock()
		p.turnActive = true
		p.mu.Unlock()

		err := p.CompactThread(th)
		if !errors.Is(err, ErrBusy) {
			t.Fatalf("轮次在跑应回忙态(errors.Is ErrBusy): %v", err)
		}
		if fake.promptCalls != 0 {
			t.Fatalf("忙拒绝不该下发 prompt: %d", fake.promptCalls)
		}
	})
	t.Run("轮次锁被占", func(t *testing.T) {
		fake := newFakeHermesClient(true)
		p := newHermesTestProvider(t, fake)
		th := compactTestThread(p)
		if !p.turnMu.TryLock() {
			t.Fatal("测试前置:应能拿到轮次锁")
		}
		defer p.turnMu.Unlock()

		err := p.CompactThread(th)
		if !errors.Is(err, ErrBusy) {
			t.Fatalf("锁被占应回忙态(errors.Is ErrBusy): %v", err)
		}
		if fake.promptCalls != 0 {
			t.Fatalf("忙拒绝不该下发 prompt: %d", fake.promptCalls)
		}
	})
}

// TestHermesCompactThreadErrorPassthrough 传输失败原样上抛(带 /compress 语境)。
func TestHermesCompactThreadErrorPassthrough(t *testing.T) {
	fake := newFakeHermesClient(true)
	fake.promptErr = errors.New("管道断了")
	p := newHermesTestProvider(t, fake)
	th := compactTestThread(p)

	err := p.CompactThread(th)
	if !errors.Is(err, fake.promptErr) {
		t.Fatalf("prompt 错误应可穿透(errors.Is): %v", err)
	}
	if !strings.Contains(err.Error(), hermesCompressCommand) {
		t.Fatalf("错误应带 /compress 语境: %v", err)
	}
}

// TestHermesCompactThreadFailureStopReasonReported 被拒/被取消的终局如实报错。
func TestHermesCompactThreadFailureStopReasonReported(t *testing.T) {
	fake := newFakeHermesClient(true)
	fake.promptOutcome = &hermesapp.PromptOutcome{StopReason: "refusal"}
	p := newHermesTestProvider(t, fake)
	th := compactTestThread(p)

	err := p.CompactThread(th)
	if err == nil || !strings.Contains(err.Error(), "refusal") {
		t.Fatalf("refusal 应如实报错(不伪造成功): %v", err)
	}
}

// TestHermesCompactThreadEmptyReplyReported 回执空的两种形态都必须报错:
// stopReason 空(应答体缺失)与正文空(命令没被识别/没产出)。
func TestHermesCompactThreadEmptyReplyReported(t *testing.T) {
	t.Run("stopReason 空", func(t *testing.T) {
		fake := newFakeHermesClient(true)
		fake.promptOutcome = &hermesapp.PromptOutcome{} // 零值应答
		p := newHermesTestProvider(t, fake)
		th := compactTestThread(p)

		err := p.CompactThread(th)
		if err == nil || !strings.Contains(err.Error(), "没有回执") {
			t.Fatalf("空 stopReason 应报错: %v", err)
		}
	})
	t.Run("正文空", func(t *testing.T) {
		fake := newFakeHermesClient(true)
		fake.promptQuiet = true // 收不到任何 agent_message_chunk
		p := newHermesTestProvider(t, fake)
		th := compactTestThread(p)

		err := p.CompactThread(th)
		if err == nil || !strings.Contains(err.Error(), "没有回执正文") {
			t.Fatalf("空正文应报错(end_turn 不等于压缩成功): %v", err)
		}
	})
}

// TestHermesCompactThreadBudgetCancels 预算耗尽:先发 session/cancel(适配器真停)
// 再报超时错,不留悬空压缩轮。
func TestHermesCompactThreadBudgetCancels(t *testing.T) {
	old := hermesCompactBudget
	hermesCompactBudget = 300 * time.Millisecond
	defer func() { hermesCompactBudget = old }()

	slow := newSlowHermesFake(newFakeHermesClient(true))
	p := NewHermesProvider()
	p.newClient = func() (hermesClient, error) { return slow, nil }
	th := compactTestThread(p)

	done := make(chan error, 1)
	go func() { done <- p.CompactThread(th) }()
	<-slow.promptBegin // 压缩轮已进入,持轮次锁等待

	select {
	case err := <-done:
		t.Fatalf("预算未到就返回了: %v", err)
	case <-time.After(80 * time.Millisecond):
	}
	if cancels := cancelCallsOf(slow.fakeHermesClient); len(cancels) != 0 {
		t.Fatalf("预算未耗尽不该先取消: %v", cancels)
	}
	// 让 ctx 超时自然到期 → 代码路径自行收场。
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "压缩超时") {
			t.Fatalf("预算耗尽应报超时并说明已发取消: %v", err)
		}
		if cancels := cancelCallsOf(slow.fakeHermesClient); len(cancels) != 1 || cancels[0] != "cached-sess" {
			t.Fatalf("超时必须补发 session/cancel: %v", cancels)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("压缩轮在预算耗尽后没有收场")
	}
}

// compactPermFake 压缩轮审批钉子:Prompt 进入 → 报号 → 发一次审批反向请求 →
// 等 release → 回一段回执正文(与真适配器"压缩完回结果文本"同形)→ end_turn。
// 只动自己,不碰共享假客户端的既有脚本(审批用例会因多出的帧改变断言)。
type compactPermFake struct {
	*fakeHermesClient
	promptBegin chan struct{} // 缓冲 1:报号即走,不阻塞压缩轮
	release     chan struct{}
}

func newCompactPermFake() *compactPermFake {
	return &compactPermFake{
		fakeHermesClient: newFakeHermesClient(true),
		promptBegin:      make(chan struct{}, 1),
		release:          make(chan struct{}),
	}
}

func (f *compactPermFake) Prompt(ctx context.Context, sessionID, text string) (*hermesapp.PromptOutcome, error) {
	f.mu.Lock()
	f.promptCalls++
	f.promptSids = append(f.promptSids, sessionID)
	f.promptTexts = append(f.promptTexts, text)
	req, notif := f.reqFn, f.notifFn
	f.mu.Unlock()
	select {
	case f.promptBegin <- struct{}{}:
	default:
	}
	if req != nil {
		req(9001, "session/request_permission", json.RawMessage(fakePermParams))
	}
	select {
	case <-f.release:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if notif != nil {
		notif(json.RawMessage(`{"sessionId":"` + sessionID + `","update":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"已压缩:上下文 120k → 30k"}}}`))
	}
	return &hermesapp.PromptOutcome{StopReason: "end_turn"}, nil
}

// TestHermesCompactThreadApprovalFailsSafe 压缩轮里的审批请求按失败安全回
// -32000(approve=nil):手机此刻没有审批卡可弹,绝不代用户答应(红线 1);
// 审批被拒后压缩轮照常走到终局,不悬挂。
func TestHermesCompactThreadApprovalFailsSafe(t *testing.T) {
	fake := newCompactPermFake()
	p := NewHermesProvider()
	p.newClient = func() (hermesClient, error) { return fake, nil }
	th := compactTestThread(p)

	done := make(chan error, 1)
	go func() { done <- p.CompactThread(th) }()
	<-fake.promptBegin // 此刻钩子已装(Prompt 里才读得到 reqFn)

	deadline := time.Now().Add(2 * time.Second)
	for {
		fake.mu.Lock()
		n := len(fake.respondErr)
		var code int
		if n > 0 {
			code = fake.respondErr[0].Code
		}
		fake.mu.Unlock()
		if n > 0 {
			if code != -32000 {
				t.Fatalf("无审批通道应回 -32000,得到 %d(%+v)", code, fake.respondErr)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("压缩轮的审批请求没有被应答(适配器会一直挂着)")
		}
		time.Sleep(10 * time.Millisecond)
	}
	close(fake.release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("审批被拒后压缩轮应正常收场: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("压缩轮没有收场")
	}
	if n, _ := hermesHandlersOf(fake.fakeHermesClient); n != nil {
		t.Fatal("压缩收场后当轮钩子必须卸载")
	}
}

// TestHermesCompactThreadLocksOutRun 压缩在途时 Run 必须被轮次互斥面挡住 ——
// 双方共用 client 级钩子(SetTurnHandlers 是整面替换),并发会互拆。
func TestHermesCompactThreadLocksOutRun(t *testing.T) {
	fake := newCompactPermFake()
	p := NewHermesProvider()
	p.newClient = func() (hermesClient, error) { return fake, nil }
	th := compactTestThread(p)

	compactDone := make(chan error, 1)
	go func() { compactDone <- p.CompactThread(th) }()
	<-fake.promptBegin // 压缩在途,持 turnMu

	runDone := make(chan struct{})
	go func() {
		defer close(runDone)
		_, _, _ = p.Run(context.Background(), TurnRequest{
			Thread:      th,
			InputBlocks: []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "你好"}}}},
			Workspace:   `C:\ws`,
		}, func(*pb.TurnServerMessage) {}, nil)
	}()
	select {
	case <-runDone:
		t.Fatal("压缩在途时 Run 不该并发进入(钩子会被互拆)")
	case <-time.After(120 * time.Millisecond):
	}

	close(fake.release)
	select {
	case err := <-compactDone:
		if err != nil {
			t.Fatalf("压缩应成功: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("压缩轮没有收场")
	}
	// 释放后 Run 才进得去(它的 prompt 也要报到)。
	select {
	case <-fake.promptBegin:
	case <-time.After(3 * time.Second):
		t.Fatal("压缩释放后 Run 仍被挡在轮次锁外")
	}
	select {
	case <-runDone:
	case <-time.After(3 * time.Second):
		t.Fatal("Run 没有收场")
	}
}
