// fake_agent_test.go —— deterministic fake agent 自身的回放语义钉子。
//
// 【测什么】七类必须覆盖场景(2026-10-04 发布验证计划 Task 1)在 fake 层面的
// 语义:剧情逐字回放、审批等待、迟到补发被拒、进程退出、长静默、取消与重试。
// server 集成(Task 2/3 的 collab/unknown E2E)由后续任务消费本包,不在本文件。
//
// 【时序纪律】全部等待用毫秒级真实 timer;断言只做下界(事件确实等了)与
// 宽上界(3s 内必须收场),不做贴线断言 —— Windows 计时器粒度与 CI 抖动吃不掉。
package testagent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/runtime"
)

// ---- 辅助 ----

// newFake 建一个台账隔离的 fake(独立 runtime 后端,不碰生产共享账)。
func newFake(t *testing.T, id string, sc Scenario) *FakeAgent {
	t.Helper()
	f := &Factory{Ledger: runtime.NewBackend()}
	return f.Provider(id, sc).(*FakeAgent)
}

// frameKind 取帧的人读类型名(断言帧序用)。
func frameKind(m *pb.TurnServerMessage) string {
	switch m.Msg.(type) {
	case *pb.TurnServerMessage_TextDelta:
		return "text"
	case *pb.TurnServerMessage_ThinkingDelta:
		return "thinking"
	case *pb.TurnServerMessage_ToolCall:
		return "tool"
	case *pb.TurnServerMessage_Usage:
		return "usage"
	case *pb.TurnServerMessage_CodexEvent:
		return "codex_event"
	default:
		return fmt.Sprintf("%T", m.Msg)
	}
}

// kindsOf 把帧序压成类型名序列。
func kindsOf(frames []*pb.TurnServerMessage) []string {
	out := make([]string, 0, len(frames))
	for _, m := range frames {
		out = append(out, frameKind(m))
	}
	return out
}

// joined 逗号拼接(断言失败信息用)。
func joined(ss []string) string { return strings.Join(ss, ",") }

// ledgerOf 取台账全部条目(List 倒序,统一转回正序便于对照)。
func ledgerOf(t *testing.T, a *FakeAgent) []runtime.EventEnvelope {
	t.Helper()
	l := a.ledger()
	if l == nil {
		t.Fatal("台账后端不应为 nil")
	}
	list := l.UnknownEvents().List(runtime.ListFilter{}, 100)
	for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
		list[i], list[j] = list[j], list[i]
	}
	return list
}

// countKind 统计台账里某分档的条数。
func countKind(envs []runtime.EventEnvelope, kind string) int {
	n := 0
	for _, e := range envs {
		if e.NormalizedKind == kind {
			n++
		}
	}
	return n
}

// waitFrames 有界等待第 run 轮发出至少 n 帧(消除"回放刚起飞"的读竞态)。
func waitFrames(t *testing.T, a *FakeAgent, run, n int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		if len(a.RunFrames(run)) >= n {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("等第 %d 轮发出 %d 帧超时,现有 %d 帧", run, n, len(a.RunFrames(run)))
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// blockingHook 永不应答的审批 hook:等 ctx 到点按其 Err 返回,与 server 真
// approve 钩子的超时形态一致(thread.go 的 2min 预算同款形状)。
func blockingHook(ctx context.Context, _ *pb.ToolCallBlock) (bool, string, error) {
	<-ctx.Done()
	return false, "", ctx.Err()
}

// agentTurnRequest 造最小 TurnRequest(fake 不读内容,仅满足签名)。
func agentTurnRequest() agent.TurnRequest { return agent.TurnRequest{} }

// ---- 场景 1:正常文本和工具调用 ----

// 剧本逐字回放:文本/工具开始→结果/用量帧按序发出,终稿与用量作为 Run 返回值;
// 工具结果帧复用同 callID 的开始卡(echo 同构帧序)。
func TestFakeNormalTextAndToolCall(t *testing.T) {
	a := newFake(t, "echo", Scenario{
		Name: "正常文本+工具调用",
		Events: []AgentEvent{
			Text("我先看看目录……"),
			ToolCall("call-1", "list_dir", `{"path":"."}`),
			ToolResult("call-1", true, "a.txt\nb.txt"),
			UsageOf(30, 120),
			Completed("完成:目录里有 2 个文件"),
		},
	})
	final, usage, err := a.Run(context.Background(), agentTurnRequest(), nil, nil)
	if err != nil {
		t.Fatalf("正常剧本不应失败: %v", err)
	}
	if got, want := joined(kindsOf(a.RunFrames(0))), "text,tool,tool,usage"; got != want {
		t.Fatalf("帧序不符: got %s want %s", got, want)
	}
	frames := a.RunFrames(0)
	fin := frames[2].GetToolCall()
	if fin.GetPhase() != pb.ToolCallUpdate_FINISHED || !fin.GetResult().GetOk() {
		t.Fatalf("第 3 帧应为 FINISHED 且 ok: %+v", fin)
	}
	if fin.GetCall().GetName() != "list_dir" || fin.GetCall().GetId() != "call-1" {
		t.Fatalf("结果帧应复用开始卡(id/name): %+v", fin.GetCall())
	}
	if usage == nil || usage.InputTokens != 30 || usage.OutputTokens != 120 {
		t.Fatalf("Run 应返回剧本用量: %+v", usage)
	}
	if len(final) != 1 || final[0].GetText().GetText() != "完成:目录里有 2 个文件" {
		t.Fatalf("终稿块不符: %+v", final)
	}
	if n := len(a.Approvals()); n != 0 {
		t.Fatalf("无审批剧本不应发审批卡: %d", n)
	}
}

// ---- 场景 2:未知通知/请求/响应 ----

// 三类未知事件全部走台账路:不产生协议帧、不结束轮次;未知反向请求按结局分流
// 台账(允许=KNOWN_PASSTHROUGH,拒绝/超时=UNKNOWN_REQUEST),绝不伪造批准。
func TestFakeUnknownEventTriplet(t *testing.T) {
	t.Run("允许路", func(t *testing.T) {
		a := newFake(t, "pi", Scenario{
			Name: "未知三类-允许",
			Events: []AgentEvent{
				UnknownNotification("progress/update", `{"progress":50,"total":100}`),
				UnknownRequest("novel/method", `{"x":1}`),
				UnknownResponse("resp-42", `{"err":"late"}`),
				Completed("ok"),
			},
		})
		allow := func(_ context.Context, call *pb.ToolCallBlock) (bool, string, error) {
			if call.GetName() != "unknown_request" {
				t.Errorf("未知请求必须走通用确认卡,实际动作名: %q", call.GetName())
			}
			if !strings.Contains(call.GetJsonArgs(), "novel/method") {
				t.Errorf("确认卡应带脱敏 method 原文: %s", call.GetJsonArgs())
			}
			return true, "允许", nil
		}
		final, _, err := a.Run(context.Background(), agentTurnRequest(), nil, allow)
		if err != nil || len(final) != 1 {
			t.Fatalf("未知事件不得结束轮次: final=%v err=%v", final, err)
		}
		if frames := a.RunFrames(0); len(frames) != 0 {
			t.Fatalf("未知事件不产生协议帧(诊断只落台账): %s", joined(kindsOf(frames)))
		}
		envs := ledgerOf(t, a)
		if got := countKind(envs, runtime.KindUnknownNotification); got != 1 {
			t.Errorf("未知通知应入账 1 条,得到 %d: %+v", got, envs)
		}
		if got := countKind(envs, runtime.KindKnownPassthrough); got != 1 {
			t.Errorf("允许的未知请求应记 KNOWN_PASSTHROUGH 1 条,得到 %d", got)
		}
		if got := countKind(envs, runtime.KindUnknownResponse); got != 1 {
			t.Errorf("未知响应应入账 1 条(带 correlation),得到 %d", got)
		}
		for _, e := range envs {
			if e.SourceAgent != "pi" || e.ProcessingStatus != runtime.StatusNew {
				t.Errorf("台账条目应带来源与 NEW 状态: %+v", e)
			}
		}
	})
	t.Run("拒绝路", func(t *testing.T) {
		a := newFake(t, "zcode", Scenario{Events: []AgentEvent{
			UnknownRequest("novel/a", `{}`), Completed("ok"),
		}})
		deny := func(context.Context, *pb.ToolCallBlock) (bool, string, error) { return false, "拒绝", nil }
		if _, _, err := a.Run(context.Background(), agentTurnRequest(), nil, deny); err != nil {
			t.Fatalf("拒绝不该让轮次崩: %v", err)
		}
		envs := ledgerOf(t, a)
		if got := countKind(envs, runtime.KindUnknownRequest); got != 1 {
			t.Fatalf("拒绝的未知请求应记 UNKNOWN_REQUEST,得到 %d: %+v", got, envs)
		}
		if !strings.Contains(envs[0].Note, "拒绝") {
			t.Errorf("台账备注应写明安全拒绝: %q", envs[0].Note)
		}
	})
	t.Run("超时路", func(t *testing.T) {
		// Approval 只作 Budget 载体:场景级 ask 与未知请求各走一次超时收场。
		a := newFake(t, "zcode", Scenario{
			Approval: &ApprovalScenario{Budget: 40 * time.Millisecond},
			Events:   []AgentEvent{UnknownRequest("novel/b", `{}`), Completed("ok")},
		})
		start := time.Now()
		if _, _, err := a.Run(context.Background(), agentTurnRequest(), nil, blockingHook); err != nil {
			t.Fatalf("超时路不该让轮次崩: %v", err)
		}
		if el := time.Since(start); el > 3*time.Second {
			t.Fatalf("短预算超时应在毫秒级收场,实际 %v", el)
		}
		obs := a.Approvals()
		if len(obs) != 2 {
			t.Fatalf("场景级审批 + 未知请求应各留一次观察: %+v", obs)
		}
		for i, o := range obs {
			if !errors.Is(o.Err, context.DeadlineExceeded) {
				t.Errorf("观察 %d 应为超时收场: %+v", i, o)
			}
		}
		if got := countKind(ledgerOf(t, a), runtime.KindUnknownRequest); got != 1 {
			t.Fatalf("超时的未知请求应落 UNKNOWN_REQUEST(失败安全),得到 %d", got)
		}
	})
}

// ---- 场景 3:等待审批 ----

// fake 发卡→挂起等待→决定来自注入的 hook(批准/拒绝/超时三路),观察面留下
// 完整记录;server 侧发 ApprovalRequest/Resolved 帧是集成任务的事。
func TestFakeApprovalPending(t *testing.T) {
	t.Run("批准", func(t *testing.T) {
		a := newFake(t, "codex", Scenario{
			Name: "等待审批-批准",
			Approval: &ApprovalScenario{
				CallName:    "shell",
				JSONArgs:    `{"command":"rm -rf build"}`,
				DecideAfter: 30 * time.Millisecond, // 挂起:其他成员可继续的时序锚点
			},
			Events: []AgentEvent{Completed("已执行")},
		})
		hook := func(ctx context.Context, call *pb.ToolCallBlock) (bool, string, error) {
			if call.GetName() != "shell" || !strings.Contains(call.GetJsonArgs(), "rm -rf build") {
				t.Errorf("卡面应按脚本构造: %+v", call)
			}
			<-time.After(30 * time.Millisecond) // 模拟用户思考
			return true, "好的", nil
		}
		start := time.Now()
		final, _, err := a.Run(context.Background(), agentTurnRequest(), nil, hook)
		if err != nil || len(final) != 1 {
			t.Fatalf("批准后应走完终局: %v err=%v", final, err)
		}
		if el := time.Since(start); el < 55*time.Millisecond {
			t.Fatalf("挂起+等待时长应可观测(≥DecideAfter+决定时长),实际 %v", el)
		}
		obs := a.Approvals()
		if len(obs) != 1 || !obs[0].Allow || obs[0].Note != "好的" {
			t.Fatalf("审批观察不符: %+v", obs)
		}
		if !obs[0].RequestedAt.Before(obs[0].ResolvedAt) {
			t.Errorf("请求/应答打点应有序(可注入时钟打点): %+v", obs[0])
		}
	})
	t.Run("拒绝后仍走完剧本", func(t *testing.T) {
		a := newFake(t, "codex", Scenario{
			Approval: &ApprovalScenario{CallName: "shell"},
			Events:   []AgentEvent{Completed("未执行,已取消该步骤")},
		})
		deny := func(context.Context, *pb.ToolCallBlock) (bool, string, error) { return false, "太危险", nil }
		final, _, err := a.Run(context.Background(), agentTurnRequest(), nil, deny)
		if err != nil || final[0].GetText().GetText() != "未执行,已取消该步骤" {
			t.Fatalf("拒绝是决定不是故障,剧本应继续: %v err=%v", final, err)
		}
		if obs := a.Approvals(); len(obs) != 1 || obs[0].Allow {
			t.Fatalf("拒绝观察不符: %+v", a.Approvals())
		}
	})
	t.Run("超时按拒绝收场", func(t *testing.T) {
		a := newFake(t, "codex", Scenario{
			Approval: &ApprovalScenario{Budget: 40 * time.Millisecond}, // 压缩 server 2min 预算
			Events:   []AgentEvent{Completed("超时未批准")},
		})
		if _, _, err := a.Run(context.Background(), agentTurnRequest(), nil, blockingHook); err != nil {
			t.Fatalf("审批超时是终局可收场的: %v", err)
		}
		obs := a.Approvals()
		if len(obs) != 1 || !errors.Is(obs[0].Err, context.DeadlineExceeded) {
			t.Fatalf("超时观察不符: %+v", obs)
		}
	})
	t.Run("审批通道缺失不伪装决定", func(t *testing.T) {
		a := newFake(t, "codex", Scenario{
			Approval: &ApprovalScenario{}, // 发卡,但不给 hook
			Events:   []AgentEvent{Completed("done")},
		})
		if _, _, err := a.Run(context.Background(), agentTurnRequest(), nil, nil); err != nil {
			t.Fatalf("通道缺失按拒绝继续,不该崩: %v", err)
		}
		if obs := a.Approvals(); len(obs) != 1 || obs[0].Err == nil {
			t.Fatalf("通道缺失必须留痕(不伪装用户意志): %+v", obs)
		}
	})
}

// ---- 场景 4:迟到响应(Delay 后补发被拒)----

// 终局早已落定,迟到的补发在到点瞬间被拒:不发出、计数 +1、goroutine 有界退出。
func TestFakeLateResponseRejectedAfterTerminal(t *testing.T) {
	a := newFake(t, "dsh", Scenario{
		Name: "迟到响应被拒",
		Events: []AgentEvent{
			Text("唯一一帧"),
			Late(200*time.Millisecond, Text("迟到补发")), // 先登记,再在终局处闭闸
			Completed("终稿"),
		},
	})
	start := time.Now()
	final, _, err := a.Run(context.Background(), agentTurnRequest(), nil, nil)
	if err != nil || final[0].GetText().GetText() != "终稿" {
		t.Fatalf("终局不该受迟到项影响: %v err=%v", final, err)
	}
	if el := time.Since(start); el > 100*time.Millisecond {
		t.Fatalf("Run 应在迟到计时器到点前返回,实际 %v", el)
	}
	if frames := a.RunFrames(0); len(frames) != 1 {
		t.Fatalf("终局后不得再发任何帧: %s", joined(kindsOf(frames)))
	}
	// 等迟到计时器到点结算:必须判拒,且后台 goroutine 退出(无泄漏)。
	if !a.WaitIdle(3 * time.Second) {
		t.Fatal("迟到补发 goroutine 未按终局闸退出(泄漏)")
	}
	if n := a.LateDropped(); n != 1 {
		t.Fatalf("迟到补发应被拒恰好 1 次,得到 %d", n)
	}
	if frames := a.Frames(); len(frames) != 1 {
		t.Fatalf("结算后仍不得出现迟到帧: 现有 %d 帧", len(frames))
	}
}

// 迟到补发赶上终局前:照常发出(判定以终局闸为界)。
func TestFakeLateResponseInTimeIsDelivered(t *testing.T) {
	a := newFake(t, "dsh", Scenario{
		Events: []AgentEvent{
			Late(30*time.Millisecond, Text("赶上了")),
			Pause(120 * time.Millisecond), // 留出迟到窗口
			Completed("终稿"),
		},
	})
	if _, _, err := a.Run(context.Background(), agentTurnRequest(), nil, nil); err != nil {
		t.Fatalf("不该失败: %v", err)
	}
	if n := a.LateDropped(); n != 0 {
		t.Fatalf("未迟到不得计拒: %d", n)
	}
	if got, want := joined(kindsOf(a.RunFrames(0))), "text"; got != want {
		t.Fatalf("迟到的赶趟帧应照常发出: got %s want %s", got, want)
	}
}

// ---- 场景 5:子进程退出 ----

// ExitAfter 在静默中途到点:回放以 provider 错误(EOF/exit status 语义)收场,
// 错误携带结构化 ErrorInfo(server 据此保留进 TurnFailed 帧)。
func TestFakeProcessExitMidTurn(t *testing.T) {
	a := newFake(t, "codex", Scenario{
		Name:      "轮中进程退出",
		ExitAfter: 40 * time.Millisecond,
		Events: []AgentEvent{
			Text("启动"),
			Pause(300 * time.Millisecond), // 长静默中途崩溃
			Completed("永远到不了"),
		},
	})
	start := time.Now()
	final, _, err := a.Run(context.Background(), agentTurnRequest(), nil, nil)
	el := time.Since(start)
	var exit *ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("应以 ExitError 收场: %v", err)
	}
	if exit.Code != 1 || !strings.Contains(err.Error(), "exit status 1") || !strings.Contains(err.Error(), "EOF") {
		t.Fatalf("退出错误应带 EOF/exit status 语义: %v", err)
	}
	info := exit.ErrorInfo()
	if info.GetCode() != "AGENT_EXITED" || !info.GetRetryable() {
		t.Fatalf("结构化诊断应可进 TurnFailed 帧: %+v", info)
	}
	if final != nil {
		t.Fatalf("退出轮不得有终稿: %+v", final)
	}
	if el > 250*time.Millisecond {
		t.Fatalf("退出应打断长静默,实际 %v", el)
	}
	if frames := a.RunFrames(0); joined(kindsOf(frames)) != "text" {
		t.Fatalf("退出前只应有启动帧: %s", joined(kindsOf(frames)))
	}
}

// ---- 场景 6:150 秒静默看门狗 ----

// 看门狗场景的注入手段:长时间零帧静默。本用例钉 fake 侧语义(静默期间无任何
// 帧、静默长度可控);上层熔断属 server/集成路径,由 Task 2 的 E2E 消费
// (届时以协同看门狗预算驱动,fake 静默时长按需放大)。
func TestFakeSilenceWindowForWatchdog(t *testing.T) {
	a := newFake(t, "hermes", Scenario{
		Name: "长静默(看门狗注入)",
		Events: []AgentEvent{
			Text("开始"),
			Pause(300 * time.Millisecond), // 零帧静默窗
			Completed("静默后恢复"),
		},
	})
	done := make(chan error, 1)
	go func() {
		_, _, err := a.Run(context.Background(), agentTurnRequest(), nil, nil)
		done <- err
	}()
	waitFrames(t, a, 0, 1)
	time.Sleep(120 * time.Millisecond) // 落在静默窗中段采样
	if frames := a.RunFrames(0); len(frames) != 1 {
		t.Fatalf("静默窗内不得发出任何帧: %s", joined(kindsOf(frames)))
	}
	if err := <-done; err != nil {
		t.Fatalf("静默后应恢复走完: %v", err)
	}
	if kinds := joined(kindsOf(a.RunFrames(0))); kinds != "text" {
		t.Fatalf("静默窗本身不产帧,全轮只应有一帧: %s", kinds)
	}
}

// ---- 场景 7:取消和重试 ----

// 取消:回放可被 ctx 中断,已发帧保留为部分现场;重试:同一 fake 再次 Run,
// 回放逐字一致(确定性),观察按轮次累积、互不串账。
func TestFakeCancelAndRetry(t *testing.T) {
	a := newFake(t, "pi", Scenario{
		Name: "取消与重试",
		Events: []AgentEvent{
			Text("第一段"),
			Pause(200 * time.Millisecond),
			Text("第二段"),
			Completed("终稿"),
		},
	})
	// 第一次:中途取消。
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, _, err := a.Run(ctx, agentTurnRequest(), nil, nil)
		done <- err
	}()
	<-a.EnterCh() // 回放已起飞
	waitFrames(t, a, 0, 1)
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("取消应以 ctx.Err 收场: %v", err)
	}
	if frames := a.RunFrames(0); joined(kindsOf(frames)) != "text" {
		t.Fatalf("取消现场应保留已发部分帧: %s", joined(kindsOf(frames)))
	}
	if !a.WaitIdle(2 * time.Second) {
		t.Fatal("取消后后台 goroutine 应退出(无泄漏)")
	}
	// 第二、三次:重试 —— 回放逐字一致。
	run := func() string {
		final, usage, err := a.Run(context.Background(), agentTurnRequest(), nil, nil)
		if err != nil || final[0].GetText().GetText() != "终稿" {
			t.Fatalf("重试轮应完整走完: %v err=%v", final, err)
		}
		if usage != nil {
			t.Fatalf("本剧本无用量帧,不应返回用量: %+v", usage)
		}
		return joined(kindsOf(a.RunFrames(a.Runs() - 1)))
	}
	first := run()
	second := run()
	if first != "text,text" || first != second {
		t.Fatalf("重试回放必须逐字一致: %q vs %q", first, second)
	}
	if n := a.Runs(); n != 3 {
		t.Fatalf("Run 计数应累积: %d", n)
	}
	if frames := a.RunFrames(0); len(frames) != 1 {
		t.Errorf("取消轮现场不应被重试覆盖: %s", joined(kindsOf(frames)))
	}
}

// ---- 附加:IgnoreStdin(僵死进程吞 stdin)----

// IgnoreStdin=true:Steer 照常 ack 但输入被吞(无帧、无效果)—— 上层只能靠
// 看门狗/取消收场;false:输入被消费,立即回可断言的文本增量。
func TestFakeIgnoreStdinSwallowSteer(t *testing.T) {
	swallow := newFake(t, "codex", Scenario{
		IgnoreStdin: true,
		Events:      []AgentEvent{Pause(150 * time.Millisecond), Completed("僵死中")},
	})
	finished := make(chan struct{}, 1)
	go func() {
		_, _, _ = swallow.Run(context.Background(), agentTurnRequest(), nil, nil)
		finished <- struct{}{}
	}()
	<-swallow.EnterCh()
	ok, turnID, err := swallow.Steer(context.Background(), "th", []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "插话"}}}})
	if !ok || err != nil || turnID == "" {
		t.Fatalf("僵死进程也照常 ack: ok=%v turn=%q err=%v", ok, turnID, err)
	}
	inputs := swallow.SteeredInputs()
	if len(inputs) != 1 || inputs[0][0].GetText().GetText() != "插话" {
		t.Fatalf("被吞的输入也要留观察: %+v", inputs)
	}
	<-finished
	if frames := swallow.RunFrames(0); len(frames) != 0 {
		t.Fatalf("IgnoreStdin 不得产生任何效果帧: %s", joined(kindsOf(frames)))
	}

	normal := newFake(t, "pi", Scenario{Events: []AgentEvent{Pause(150 * time.Millisecond), Completed("正常")}})
	normalDone := make(chan struct{}, 1)
	go func() {
		_, _, _ = normal.Run(context.Background(), agentTurnRequest(), nil, nil)
		normalDone <- struct{}{}
	}()
	<-normal.EnterCh()
	ok, _, err = normal.Steer(context.Background(), "th", nil)
	if !ok || err != nil {
		t.Fatalf("正常 Steer 应成功: %v", err)
	}
	if frames := normal.RunFrames(0); len(frames) != 1 || !strings.Contains(frames[0].GetTextDelta().GetDelta(), "[steered]") {
		t.Fatalf("正常路径应回可断言的消费帧: %s", joined(kindsOf(frames)))
	}
	<-normalDone
}
