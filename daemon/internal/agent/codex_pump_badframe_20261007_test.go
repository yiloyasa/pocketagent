// codex_pump_badframe_20261007_test.go —— C-1 泵侧坏帧旁路的回归钉
// (unknown-bypass-audit.md §二 C-1,2026-10-07 收口轮)。
//
// 【钉什么】Run 事件泵 item/turn 各 case 的参数解包失败,此前与异线程过滤共用
// 一条静默分支,坏形状与"别的会话的帧"不可区分。收口后:解包失败 →
// recordBadFrame = 台账 UNKNOWN("…:<unparseable>",与 zcode/dsh 泵侧同款命名)
// + 限频 STATUS 卡。裁量与 zcode/dsh 的"只台账"不同:codex 的常驻全局钩子对
// 这几个方法按 codexGlobalNoticeKinds 记 KNOWN_PROJECTED/PASSTHROUGH,"这一帧
// 坏了"在台账里只存在于泵侧 —— 坏帧是唯一留痕,按 recordAs 的 nil-sink 语义
// 传真 sink 出卡;异线程过滤维持静默(全家同款刻意取舍)。
//
// 【怎么喂】fake codex app-server(codex_effect_test.go,TestHelperProcess 惯用法)
// 的 PA_FAKE_CODEX_BADFRAME_NOTIFY:=1 发四条 params 为 JSON 字符串的坏帧
// (结构体解包必失败);="terminal" 发坏终局帧(钉"留痕不改收口语义");
// ="foreign" 发异线程的好帧作静默对照。不依赖本机 codex。
package agent

import (
	"strings"
	"testing"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/store"
)

func TestCodexPumpBadFrameLedgerAndCard(t *testing.T) {
	p, _ := newFakeCodexProvider(t, false)
	t.Setenv("PA_FAKE_CODEX_BADFRAME_NOTIFY", "1")
	ws := t.TempDir()
	// 坏帧不得打断轮次:fake 随后照常发好 turn/completed,Run 正常收口。
	msgs := runOneTurn(t, p, &store.Thread{ID: "th-codex-badframe"}, ws)

	// 1) 台账:四类坏帧各一条 <unparseable> UNKNOWN(item/started|completed、
	//    turn/started、item/agentMessage/delta),thread_id 关联到我们自己的
	//    会话 id(pump 侧 notices 的归属)。
	want := map[string]bool{
		"item/started:<unparseable>":            false,
		"item/completed:<unparseable>":          false,
		"turn/started:<unparseable>":            false,
		"item/agentMessage/delta:<unparseable>": false,
	}
	for _, e := range sharedRuntimeBackend().UnknownEvents().List(
		runtime.ListFilter{AgentID: "codex", ThreadID: "th-codex-badframe"}, 0) {
		if _, ok := want[e.SourceMethod]; !ok {
			continue
		}
		if e.NormalizedKind != runtime.KindUnknownNotification {
			t.Fatalf("坏帧泵侧留痕应为 UNKNOWN: %+v", e)
		}
		want[e.SourceMethod] = true
	}
	for m, seen := range want {
		if !seen {
			t.Fatalf("坏帧 %s 未落台账(C-1 收口失效)", m)
		}
	}

	// 2) 出卡(裁量钉):recordAs 的限频 STATUS 卡,标题带 <unparseable>。
	cards := 0
	for _, m := range msgs {
		if ev := m.GetCodexEvent(); ev != nil && strings.Contains(ev.Title, "<unparseable>") {
			if ev.Kind != pb.CodexEvent_STATUS || ev.Code != "native_notification" {
				t.Fatalf("坏帧卡应为 recordAs 的限频 STATUS 卡, got=%+v", ev)
			}
			cards++
		}
	}
	if cards == 0 {
		t.Fatalf("坏帧未出诊断卡(裁量:泵侧是 codex 坏帧的唯一留痕,应传真 sink), msgs=%d 条", len(msgs))
	}
}

// TestCodexPumpBadTerminalFrameLedgerNotFinish 钉终局帧坏形状:
// 解包失败必须留痕,但收口语义不变 —— 不因坏帧提前 finish,由 fake 随后的
// 好 turn/completed 正常收口(runOneTurn 的 err==nil 即钉住"轮次不被坏帧打断、
// 收口条件未被本修复改变")。
func TestCodexPumpBadTerminalFrameLedgerNotFinish(t *testing.T) {
	p, _ := newFakeCodexProvider(t, false)
	t.Setenv("PA_FAKE_CODEX_BADFRAME_NOTIFY", "terminal")
	ws := t.TempDir()
	msgs := runOneTurn(t, p, &store.Thread{ID: "th-codex-badframe3"}, ws)

	found := false
	for _, e := range sharedRuntimeBackend().UnknownEvents().List(
		runtime.ListFilter{AgentID: "codex", ThreadID: "th-codex-badframe3"}, 0) {
		if e.SourceMethod == "turn/completed:<unparseable>" {
			if e.NormalizedKind != runtime.KindUnknownNotification {
				t.Fatalf("终局坏帧留痕应为 UNKNOWN: %+v", e)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("turn/completed 坏帧未落台账(C-1 终局档收口失效)")
	}
	// 裁量再现:终局坏帧也出限频 STATUS 卡(泵侧是唯一"坏了"留痕)。
	card := false
	for _, m := range msgs {
		if ev := m.GetCodexEvent(); ev != nil && strings.Contains(ev.Title, "<unparseable>") {
			card = true
		}
	}
	if !card {
		t.Fatalf("终局坏帧未出诊断卡, msgs=%d 条", len(msgs))
	}
}

func TestCodexPumpForeignThreadFrameStaysSilent(t *testing.T) {
	p, _ := newFakeCodexProvider(t, false)
	// "foreign":好帧但 threadId 指向别的会话 —— 异线程过滤维持静默
	// (不得出现 <unparseable> 台账条目或诊断卡;它们不是坏帧)。
	t.Setenv("PA_FAKE_CODEX_BADFRAME_NOTIFY", "foreign")
	ws := t.TempDir()
	msgs := runOneTurn(t, p, &store.Thread{ID: "th-codex-badframe2"}, ws)

	for _, m := range msgs {
		if ev := m.GetCodexEvent(); ev != nil && strings.Contains(ev.Title, "<unparseable>") {
			t.Fatalf("异线程好帧不应出 <unparseable> 卡: %+v", ev)
		}
	}
	for _, e := range sharedRuntimeBackend().UnknownEvents().List(
		runtime.ListFilter{AgentID: "codex", ThreadID: "th-codex-badframe2"}, 0) {
		if strings.Contains(e.SourceMethod, "<unparseable>") {
			t.Fatalf("异线程好帧不应落 <unparseable> 台账: %+v", e)
		}
	}
}
