package agent

import (
	"context"
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

// 【"echo 的记忆能被读到"这条断言的去向】原先这里有个
// TestEchoTurnWritesRuntimeMemory：跑一轮 echo，断言记忆被写进公共运行时、且
// 模拟重启后能恢复。D7 把记忆写入从 provider 的 Run 收归 server 的公共轮次路径
// （server/turn_memory.go）之后，这条断言的前提（provider 自己写）已经不成立 ——
// 留在原处会变成对一件已经不成立的事的检查，且仍绿会掩盖"接线根本没生效"。
// 等价且更强的那条（真 echo 经 server 落库 + 换同数据目录的新后端仍读得回）现在在
// server 包：见 turn_memory_test.go::TestEchoTurnWritesConversationThroughServer。

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
