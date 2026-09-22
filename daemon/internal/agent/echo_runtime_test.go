package agent

import (
	"context"
	"strings"
	"testing"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/store"
)

// TestEchoDeclarationIsCompliant：Echo 必须如实申报形态且四项必需入口都有交代。
func TestEchoDeclarationIsCompliant(t *testing.T) {
	rep := runtime.VerifyDeclaration("echo", NewEcho())
	if !rep.Declared {
		t.Fatal("Echo 必须实现 runtime.Participant（最低契约 provider 是后面所有 agent 的样板）")
	}
	if !rep.OK() {
		for _, v := range rep.Violations {
			t.Errorf("契约违约: %s", v)
		}
	}
	if got := rep.Descriptor.SessionModel; got != runtime.SourceDaemon {
		t.Fatalf("Echo 无原生会话，会话事实应在 daemon 侧（DAEMON），实得 %q", got)
	}
}

// TestEchoDescriptorDistinguishesNativeFromRuntimeEnvelope 钉住一个容易搞错的区分：
//
//	无原生 API  ≠  无记忆。
//
// Echo 必须在 runtime.memory/runtime.cache（daemon 侧公共入口）上声明**支持**，
// 同时在 memory.native/cache.native（原生自带能力）上声明**不支持**。
// 把两者混为一谈，就会得出"因为它没有 codex 那样的原生 API，所以它没有记忆"
// 的结论 —— 这正是规格 §6 开头那句禁令要拦的东西。
func TestEchoDescriptorDistinguishesNativeFromRuntimeEnvelope(t *testing.T) {
	d := echoRuntimeDescriptor()

	for _, c := range runtime.RequiredEntryPoints {
		if !d.Has(c) {
			t.Errorf("Echo 应声明支持 %q（它交付了默认实现）", c)
		}
		if _, limited := d.Limit(c); limited {
			t.Errorf("Echo 不该把 %q 列为不支持 —— 它确实交付了", c)
		}
	}
	for _, c := range []runtime.Capability{runtime.CapMemoryNative, runtime.CapCacheNative, runtime.CapMemoryCompaction} {
		if d.Has(c) {
			t.Errorf("Echo 没有原生能力，不该声明 %q", c)
		}
		if _, limited := d.Limit(c); !limited {
			t.Errorf("Echo 做不到 %q，必须写进 Limitations（沉默缺失即违约）", c)
		}
	}
}

// TestEchoRuntimeContract 是规格 §15 L1「所有 agent 通过同一套 runtime contract」
// 在 Echo 上的落地：跑与 runtime 包同一套用例集（非另写一份）。
func TestEchoRuntimeContract(t *testing.T) {
	p := NewEcho()
	th := &store.Thread{ID: "th-echo-contract"}
	rt, err := p.OpenRuntime(context.Background(), th)
	if err != nil {
		t.Fatalf("OpenRuntime 失败: %v", err)
	}
	runtime.ContractSuite{
		T:       t,
		AgentID: "echo",
		RT:      rt,
		Backend: p.EchoRuntimeBackend(),
		Thread:  th,
	}.RunAll()
}

// TestEchoTurnWritesRuntimeMemory 是不 mock 中间层的端到端证据：
// 真实跑一轮 Echo → 记忆真的被写进公共运行时 → 模拟 daemon 重启后仍能恢复。
//
// 【为什么这条比契约套件更值钱】契约套件测的是"接口语义成立"；这条测的是
// "轮次路径真的走了契约"。两者缺一：只有前者 -> 契约可能是一块没人用的装饰；
// 只有后者 -> 单项语义可能不符合契约。
func TestEchoTurnWritesRuntimeMemory(t *testing.T) {
	p := NewEcho()
	backend := p.EchoRuntimeBackend()
	thread := &store.Thread{ID: "th-echo-turn-memory"}
	ctx := context.Background()

	var events int
	sink := func(_ *pb.TurnServerMessage) { events++ }
	blocks, _, err := p.Run(ctx, TurnRequest{
		Thread:      thread,
		Workspace:   t.TempDir(),
		InputBlocks: []pb.ContentBlock{blockText("你好，回声")},
	}, sink, nil)
	if err != nil {
		t.Fatalf("Echo 轮次失败: %v", err)
	}
	if events == 0 {
		t.Fatal("Echo 未向 sink 发出任何事件（前置条件不满足，后续断言无意义）")
	}
	if len(blocks) == 0 {
		t.Fatal("Echo 未返回内容块（前置条件不满足）")
	}

	key := runtime.MemoryKey{AgentID: echoAgentID, ThreadID: thread.ID, Kind: runtime.MemoryConversation}
	snap, err := p.mustRuntime(t, thread).Memory.Load(ctx, key)
	if err != nil {
		t.Fatalf("读记忆失败: %v", err)
	}
	if len(snap.Items) != 2 {
		t.Fatalf("一轮过后应记下 user + assistant 两条，实得 %d", len(snap.Items))
	}
	if snap.Items[0].Role != "user" || snap.Items[1].Role != "assistant" {
		t.Fatalf("记忆角色顺序应为 user→assistant，实得 %q→%q", snap.Items[0].Role, snap.Items[1].Role)
	}
	if !strings.Contains(string(snap.Items[0].Payload), "你好，回声") {
		t.Fatalf("用户输入原文应进入记忆，实得 %q", snap.Items[0].Payload)
	}

	// ---- 端到端恢复：模拟 daemon 重启 ----
	durable, err := backend.ExportDurable()
	if err != nil {
		t.Fatalf("ExportDurable 失败: %v", err)
	}
	if err := backend.ImportDurable(durable); err != nil {
		t.Fatalf("ImportDurable（模拟重启）失败: %v", err)
	}

	rt2 := p.mustRuntime(t, thread)
	// 重启后先验证"不 Restore 就看不到" —— 否则说明重启模拟是假的，恢复断言不成立。
	if lost, _ := rt2.Memory.Load(ctx, key); len(lost.Items) != 0 {
		t.Fatalf("模拟重启后内存态应已清空（说明重启是假的），实得 %d 条", len(lost.Items))
	}
	if err := rt2.Memory.Restore(ctx, key); err != nil {
		t.Fatalf("重启后 Restore 失败: %v", err)
	}
	recovered, err := rt2.Memory.Load(ctx, key)
	if err != nil {
		t.Fatalf("重启后 Load 失败: %v", err)
	}
	if len(recovered.Items) != 2 {
		t.Fatalf("重启后应恢复 2 条会话记忆，实得 %d", len(recovered.Items))
	}
	if !strings.Contains(string(recovered.Items[0].Payload), "你好，回声") {
		t.Fatalf("恢复后的用户输入原文应完好，实得 %q", recovered.Items[0].Payload)
	}
}

// mustRuntime 取运行时，失败即终止（测试辅助）。
func (e *EchoProvider) mustRuntime(t *testing.T, thread *store.Thread) *runtime.AgentRuntime {
	t.Helper()
	rt, err := e.OpenRuntime(context.Background(), thread)
	if err != nil {
		t.Fatalf("OpenRuntime 失败: %v", err)
	}
	if rt == nil {
		t.Fatal("OpenRuntime 返回 nil 运行时")
	}
	return rt
}

// TestEchoOpenRuntimeRejectsNilThread：缺 thread 必须 fail closed，
// 而不是交付一个"无主运行时"让调用方往不存在的会话上写记忆。
func TestEchoOpenRuntimeRejectsNilThread(t *testing.T) {
	rt, err := NewEcho().OpenRuntime(context.Background(), nil)
	if err == nil {
		t.Fatalf("缺 thread 应报错，实得 rt=%+v", rt)
	}
	if rt != nil {
		t.Fatal("报错时不得同时返回运行时（调用方会以为交付成功了）")
	}
}

// TestEchoTurnWithoutThreadIsHarmless：无 thread 的轮次（如某些探测路径）
// 不得因为记记忆而崩 —— 记忆记账是加法，不该让主流程挂掉。
func TestEchoTurnWithoutThreadIsHarmless(t *testing.T) {
	p := NewEcho()
	var events int
	sink := func(_ *pb.TurnServerMessage) { events++ }
	if _, _, err := p.Run(context.Background(), TurnRequest{
		Workspace:   t.TempDir(),
		InputBlocks: []pb.ContentBlock{blockText("no thread")},
	}, sink, nil); err != nil {
		t.Fatalf("无 thread 的轮次不该失败: %v", err)
	}
	if events == 0 {
		t.Fatal("无 thread 时轮次本身仍应正常吐事件")
	}
}
