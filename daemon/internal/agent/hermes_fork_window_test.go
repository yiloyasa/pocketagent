// hermes_fork_window_test.go —— fork 会话命令广告窗口的行为契约
// (2026-10-15 D4:补 hermes fork 会话的命令账本捕获窗口)。
//
// 【调用链核实结论(走实施第 2 条:无调用链可挂窗口,不造调用链)】
//   - daemon 侧没有任何代码调用 hermesapp.Client.ForkSession:agent 包内
//     grep ForkSession 零命中;HermesProvider 无 ForkThread/ForkSession 方法
//     (本文件 TestHermesForkHasNoDedicatedCaptureEntry 用反射钉死);
//   - server 侧 ForkThread(thread_ext.go:277) 只分流 pi 与 codex,无 hermes
//     分支;hermes 分叉语义在 daemon 侧不存在"重挂载入口"(D1 报告 §六.3
//     「将来的第四家形状」)。
//   - 因此不新增 fork 功能、不在“不存在的调用链”上挂窗口捕获器(时序语义
//     不许动,见 hermes_slash.go 文件头)。fork 会话的第一轮广告走已有
//     第二写入点:轮次泵消费侧 noteCommandsFromEvent —— 本文件钉死该路径。
//
// 【钉什么】
//  1. fork 子会话跑第一轮时,轮内到达的 available_commands_update 经泵路径
//     入账(端到端 runHermesText + commands-round 脚本);父线程账本不受污染;
//     消费侧 sid 二道防线对 fork sid 同样生效;
//  2. fork 路径上的显式空广告同样覆盖旧账本(空清单 ≠ 从未广告);
//  3. agent 层 fork 入口缺席的反射守卫(将来真加 fork 方法时本用例变红,
//     提醒同步加与 create/remount 同款的窗口捕获器)。
package agent

import (
	"reflect"
	"strings"
	"testing"

	"github.com/pocketagent/daemon/internal/hermesapp"
	"github.com/pocketagent/daemon/internal/store"
)

// TestHermesForkAdvertiseFillsViaTurnPump fork 子会话的第一轮广告走轮次泵
// 路径入账:子线程(ExternalSession = fork 出的新会话)跑带轮内广告的一轮,
// 账本命中;父线程账本保持 nil(无泄漏);外来 sid 的帧被消费侧二道防线拦下。
func TestHermesForkAdvertiseFillsViaTurnPump(t *testing.T) {
	fake := newFakeHermesClient(true)
	p := newHermesTestProvider(t, fake)

	// 父线程先跑一轮无广告轮:前置条件是账本为空(排除假阳性)。
	parent := &store.Thread{ID: "t-fork-parent"}
	if _, _, err := runHermes(t, p, parent, nil); err != nil {
		t.Fatalf("父轮次应成功: %v", err)
	}
	if got := p.SlashCommandsFor(parent.ID); got != nil {
		t.Fatalf("前置:无广告轮不得留账本: %+v", got)
	}

	// fork 子会话的第一轮:ExternalSession 指向 fork 出的新会话号,轮内
	// commands-round 脚本模拟适配器在 fork 应答后补发的广告(走泵路径)。
	child := &store.Thread{ID: "t-fork-child",
		ExternalSession: store.ExternalSessionRef{SessionID: "fork-sess-new", Cwd: `C:\ws`}}
	if _, _, err := runHermesText(t, p, child, nil, "commands-round fork后第一轮"); err != nil {
		t.Fatalf("fork 子会话首轮应成功: %v", err)
	}
	fake.mu.Lock()
	lastSID := ""
	if len(fake.promptSids) > 0 {
		lastSID = fake.promptSids[len(fake.promptSids)-1]
	}
	fake.mu.Unlock()
	if lastSID != "fork-sess-new" {
		t.Fatalf("子会话首轮应跑在 fork 出的新会话上,实际 sid=%q", lastSID)
	}
	got := p.SlashCommandsFor(child.ID)
	if len(got) != 2 || got[0].Name != "help" || got[1].Name != "model" {
		t.Fatalf("fork 首轮的轮内广告必须经泵路径入账: %+v", got)
	}
	if got[1].InputHint != "model name to switch to" {
		t.Fatalf("fork 广告的 inputHint 映射错: %+v", got[1])
	}
	// 父线程账本不得被子会话广告污染(共享适配器隔离)。
	if again := p.SlashCommandsFor(parent.ID); again != nil {
		t.Fatalf("fork 子会话广告不得污染父线程账本: %+v", again)
	}

	// 消费侧二道防线:fork sid 下别家会话的帧同样拒收,已有账本不被覆盖。
	p.noteCommandsFromEvent(child.ID, "fork-sess-new",
		hermesapp.ParseSessionUpdate(hermesSlashFrameJSON("alien-sess", `{"name":"x","description":"y"}`)))
	if got2 := p.SlashCommandsFor(child.ID); len(got2) != 2 || got2[0].Name != "help" {
		t.Fatalf("外来 sid 帧不得覆盖 fork 子会话账本: %+v", got2)
	}
}

// TestHermesForkEmptyAdvertiseClearsViaPump fork 路径上的显式空广告同样是
// "没有命令"的显式事实:经泵路径覆盖旧账本(非 nil 空清单);非命令变体与
// 外来 sid 不得误清。
func TestHermesForkEmptyAdvertiseClearsViaPump(t *testing.T) {
	p := NewHermesProvider()
	const tid, sid = "t-fork-child-empty", "fork-sess-e"

	_, cmds, ok := parseHermesAvailableCommands(hermesSlashFrameJSON(sid, `{"name":"help","description":"d"}`))
	if !ok {
		t.Fatal("测试前置:命令帧应解析成功")
	}
	p.setSlashCommands(tid, cmds)
	if got := p.SlashCommandsFor(tid); len(got) != 1 {
		t.Fatalf("测试前置:账本应有 1 条: %+v", got)
	}

	// 显式空广告覆盖(泵路径,与 TestHermesSlashLedgerEmptyAdvertiseOverwrites
	// 的直接覆盖不同,这里走与 fork 首轮相同的消费侧入口)。
	p.noteCommandsFromEvent(tid, sid,
		hermesapp.ParseSessionUpdate(hermesSlashFrameJSON(sid, ``)))
	got := p.SlashCommandsFor(tid)
	if got == nil || len(got) != 0 {
		t.Fatalf("fork 路径显式空广告应清空面板(非 nil 空清单): %#v", got)
	}

	// 非命令变体不得误清(空文本 chunk 经泵入口同样无操作)。
	p.setSlashCommands(tid, cmds)
	p.noteCommandsFromEvent(tid, sid,
		hermesapp.ParseSessionUpdate([]byte(`{"sessionId":"`+sid+`","update":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"hi"}}}`)))
	if got := p.SlashCommandsFor(tid); len(got) != 1 {
		t.Fatalf("非命令变体不得误清账本: %+v", got)
	}

	// 外来 sid 的显式空广告不得清掉本线程账本。
	p.noteCommandsFromEvent(tid, sid,
		hermesapp.ParseSessionUpdate(hermesSlashFrameJSON("alien-sess", ``)))
	if got := p.SlashCommandsFor(tid); len(got) != 1 {
		t.Fatalf("外来 sid 的空广告不得清账本: %+v", got)
	}
}

// TestHermesForkHasNoDedicatedCaptureEntry agent 层 fork 入口缺席的反射守卫:
// HermesProvider 与 hermesClient 依赖面上不得出现含 Fork 的方法(ForkSession
// 只活在 *hermesapp.Client 上,provider 从未调用,故无窗口可挂)。
//
// 将来若为 hermes 接通分叉而新增 provider 侧 fork 方法,本用例会变红 —— 那
// 是有意的:新增调用链必须同步加与 create/remount 同款的窗口捕获器(复用
// beginSlashCapture helper,不另写时序),届时同步更新本守卫。
func TestHermesForkHasNoDedicatedCaptureEntry(t *testing.T) {
	pt := reflect.TypeOf((*HermesProvider)(nil))
	for i := 0; i < pt.NumMethod(); i++ {
		if name := pt.Method(i).Name; strings.Contains(strings.ToLower(name), "fork") {
			t.Fatalf("HermesProvider 新增了 fork 入口 %q:必须同步加窗口捕获器并更新本守卫(见文件头)", name)
		}
	}
	it := reflect.TypeOf((*hermesClient)(nil)).Elem()
	for i := 0; i < it.NumMethod(); i++ {
		if name := it.Method(i).Name; strings.Contains(strings.ToLower(name), "fork") {
			t.Fatalf("hermesClient 依赖面新增了 fork 方法 %q:必须同步加窗口捕获器并更新本守卫(见文件头)", name)
		}
	}
}
