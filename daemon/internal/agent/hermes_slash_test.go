// hermes_slash_test.go —— hermes 斜杠命令账本与 SlashCommandsFor 的行为契约
// (2026-10-14 slash 位化;假客户端注入,不起真进程)。
//
// 【钉什么】
//  1. 解析:available_commands_update 的 availableCommands[].name/description/
//     input.hint → pb.SlashCommand 三字段;显式空数组 = 显式"没有命令"(空清单,
//     不是 nil);其它变体/坏帧不误判;
//  2. 写入点 1(会话建立窗口):session/new 应答期间到达的广告帧(缓冲路径)与
//     应答后、泵安装前到达的帧(直达路径;真实适配器 live 实测就是"应答后 ~1ms")
//     都必须入账 —— 旧接线这两条都会在 readLoop 判空处静默丢;
//  3. 写入点 2(轮次事件泵):轮内重广告照记账,且 KNOWN_PASSTHROUGH 台账留痕
//     并存(两者不互斥);
//  4. 隔离:带别家 sessionId 的广告不得挂到本线程账本;
//  5. 失败面:SlashCommandsFor 未记账回 nil(空面板,绝不冷启不硬编码);
//     会话建立失败闭窗(半挂载不留账本);dropThreadRef 连带清账本(ephemeral 不泄漏);
//  6. 重挂载(session/load)窗口同样接住广告,且历史回放计数不被命令帧污染。
package agent

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/hermesapp"
	"github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/store"
)

// ---- 解析层钉子 ----

// hermesSlashFrameJSON 按 live 实测形状拼一条 available_commands_update 帧
// (2026-10-14 探针抓帧:name 必有、description 必有、input.hint 可缺席)。
func hermesSlashFrameJSON(sessionID string, entries string) json.RawMessage {
	return json.RawMessage(`{"sessionId":"` + sessionID + `","update":{"sessionUpdate":"available_commands_update","availableCommands":[` + entries + `]}}`)
}

// TestParseHermesAvailableCommandsLiveShape 用 live 帧形状(help 无 input、
// model 带 input.hint)钉三字段映射与缺席语义。
func TestParseHermesAvailableCommandsLiveShape(t *testing.T) {
	raw := hermesSlashFrameJSON("sess-1",
		`{"description":"List available commands","name":"help"},`+
			`{"description":"Show current model and provider, or switch models","input":{"hint":"model name to switch to"},"name":"model"}`)
	sid, cmds, ok := parseHermesAvailableCommands(raw)
	if !ok {
		t.Fatalf("live 形状的 available_commands_update 必须被判为命令帧: %s", raw)
	}
	if sid != "sess-1" {
		t.Fatalf("会话 id 解析错: %q", sid)
	}
	if len(cmds) != 2 {
		t.Fatalf("应解析 2 条命令,得到 %d: %+v", len(cmds), cmds)
	}
	if cmds[0].Name != "help" || cmds[0].Description != "List available commands" || cmds[0].InputHint != "" {
		t.Fatalf("无 input 的条目应留空 inputHint: %+v", cmds[0])
	}
	if cmds[1].Name != "model" || cmds[1].InputHint != "model name to switch to" {
		t.Fatalf("input.hint → InputHint 映射错: %+v", cmds[1])
	}
}

// TestParseHermesAvailableCommandsRejectsNonCommandFrames 其它变体与坏帧必须
// 原样跳过(ok=false),不误记也不误计数。
func TestParseHermesAvailableCommandsRejectsNonCommandFrames(t *testing.T) {
	cases := map[string]json.RawMessage{
		"文本增量":  json.RawMessage(`{"sessionId":"s","update":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"hi"}}}`),
		"usage": json.RawMessage(`{"sessionId":"s","update":{"sessionUpdate":"usage_update","size":1,"used":2}}`),
		"坏帧":    json.RawMessage(`12345`),
		"空帧":    nil,
	}
	for name, raw := range cases {
		if _, _, ok := parseHermesAvailableCommands(raw); ok {
			t.Errorf("%s 不得被判为命令帧", name)
		}
	}
}

// TestParseHermesAvailableCommandsExplicitEmptyIsEmptyNotNil 显式空数组 =
// 适配器"没有命令"的显式事实(空清单 ≠ 从未广告);空名条目被过滤(点不动)。
func TestParseHermesAvailableCommandsExplicitEmptyIsEmptyNotNil(t *testing.T) {
	_, cmds, ok := parseHermesAvailableCommands(hermesSlashFrameJSON("s", ``))
	if !ok || cmds == nil || len(cmds) != 0 {
		t.Fatalf("显式空数组应得到非 nil 空清单, got ok=%v cmds=%#v", ok, cmds)
	}
	_, cmds, ok = parseHermesAvailableCommands(hermesSlashFrameJSON("s", `{"name":"","description":"空名"},{"name":"ok","description":"d"}`))
	if !ok || len(cmds) != 1 || cmds[0].Name != "ok" {
		t.Fatalf("空名条目应被过滤、其余保留: %+v", cmds)
	}
}

// ---- 写入点 1:会话建立窗口 ----

// hermesSlashAdvertiseFake 在会话建立 RPC 的指定时机发广告帧(对真实适配器
// "应答后 call_soon 立刻发"的两种到达次序建模):
//   - emitOnNew:session/new 应答期间发(应答处理完 → bind 之前,缓冲路径);
//   - emitOnSetModel:应答返回后、下一个 RPC(泵安装前)发(直达路径);
//   - emitOnLoad:session/load 期间发(remount 已绑定,直达路径)。
//
// 其余方法经嵌入转发。
type hermesSlashAdvertiseFake struct {
	*fakeHermesClient

	mu             sync.Mutex
	emitOnNew      []json.RawMessage // 用 SessionNew 的会话号改写后发出
	emitOnSetModel []json.RawMessage
	emitOnLoad     []json.RawMessage
}

func (f *hermesSlashAdvertiseFake) emit(frames []json.RawMessage, sid string) {
	if len(frames) == 0 {
		return
	}
	f.mu.Lock()
	notif := f.notifFn
	f.mu.Unlock()
	if notif == nil {
		return
	}
	for _, raw := range frames {
		notif(json.RawMessage(strings.ReplaceAll(string(raw), "$SID", sid)))
	}
}

func (f *hermesSlashAdvertiseFake) SessionNew(ctx context.Context, cwd string) (*hermesapp.SessionNewResult, error) {
	st, err := f.fakeHermesClient.SessionNew(ctx, cwd)
	if err != nil {
		return st, err
	}
	f.emit(f.emitOnNew, st.SessionID)
	return st, nil
}

func (f *hermesSlashAdvertiseFake) SetSessionModel(ctx context.Context, sessionID, modelID string) error {
	err := f.fakeHermesClient.SetSessionModel(ctx, sessionID, modelID)
	f.emit(f.emitOnSetModel, sessionID)
	return err
}

func (f *hermesSlashAdvertiseFake) LoadSession(ctx context.Context, cwd, sessionID string) (*hermesapp.SessionLoadResult, error) {
	st, err := f.fakeHermesClient.LoadSession(ctx, cwd, sessionID)
	f.emit(f.emitOnLoad, sessionID)
	return st, err
}

var _ hermesClient = (*hermesSlashAdvertiseFake)(nil)

// TestHermesSlashLedgerFilledFromSessionNewWindow 写入点 1:session/new 应答
// 窗口的广告(应答期间到达=缓冲路径)必须入账 —— 这正是 live 实测的到达时机
// (应答后 ~1ms,彼时轮次泵还没装)。旧接线此帧无人承接。
func TestHermesSlashLedgerFilledFromSessionNewWindow(t *testing.T) {
	base := newFakeHermesClient(true)
	fake := &hermesSlashAdvertiseFake{fakeHermesClient: base,
		emitOnNew: []json.RawMessage{hermesSlashFrameJSON("$SID",
			`{"description":"List available commands","name":"help"},`+
				`{"description":"Show current version","name":"version"}`)}}
	p := NewHermesProvider()
	p.newClient = func() (hermesClient, error) { return fake, nil }
	th := &store.Thread{ID: "t-slash-new"}

	if _, _, err := runHermes(t, p, th, nil); err != nil {
		t.Fatalf("轮次应成功: %v", err)
	}
	got := p.SlashCommandsFor(th.ID)
	if len(got) != 2 || got[0].Name != "help" || got[1].Name != "version" {
		t.Fatalf("session/new 窗口的广告必须入账: %+v", got)
	}
	if got[0].Description != "List available commands" {
		t.Fatalf("description 字段映射错: %+v", got[0])
	}
}

// TestHermesSlashLedgerFilledAfterResponseBeforePump 写入点 1(直达路径):
// 应答返回之后、泵安装之前(set_model 往返窗口)到达的广告同样入账 —— 真实
// 适配器把广告排在应答之后,本窗口是它的常规落点。
func TestHermesSlashLedgerFilledAfterResponseBeforePump(t *testing.T) {
	base := newFakeHermesClient(true)
	fake := &hermesSlashAdvertiseFake{fakeHermesClient: base,
		emitOnSetModel: []json.RawMessage{hermesSlashFrameJSON("$SID",
			`{"description":"Clear conversation history","name":"reset"}`)}}
	p := NewHermesProvider()
	p.newClient = func() (hermesClient, error) { return fake, nil }
	// 显式模型 → Run 会发 set_model(本帧的载体窗口);默认档不发则此路径不经过。
	th := &store.Thread{ID: "t-slash-bind", ModelID: "hermes/deepseek-flash"}

	if _, _, err := runHermes(t, p, th, nil); err != nil {
		t.Fatalf("轮次应成功: %v", err)
	}
	got := p.SlashCommandsFor(th.ID)
	if len(got) != 1 || got[0].Name != "reset" {
		t.Fatalf("绑定后到达的广告必须入账: %+v", got)
	}
}

// TestHermesSlashLedgerIgnoresOtherSessions 共享适配器上别家会话的广告不得
// 挂到本线程(缓冲路径按绑定 sid 对表丢弃;直达路径按 sid 比较丢弃)。
func TestHermesSlashLedgerIgnoresOtherSessions(t *testing.T) {
	base := newFakeHermesClient(true)
	fake := &hermesSlashAdvertiseFake{fakeHermesClient: base,
		emitOnNew: []json.RawMessage{hermesSlashFrameJSON("other-sess", `{"description":"别家","name":"foreign"}`)},
		emitOnSetModel: []json.RawMessage{
			hermesSlashFrameJSON("other-sess", `{"description":"别家直达","name":"foreign2"}`),
			hermesSlashFrameJSON("$SID", `{"description":"自家的","name":"mine"}`)}}
	p := NewHermesProvider()
	p.newClient = func() (hermesClient, error) { return fake, nil }
	th := &store.Thread{ID: "t-slash-iso", ModelID: "hermes/deepseek-flash"}

	if _, _, err := runHermes(t, p, th, nil); err != nil {
		t.Fatalf("轮次应成功: %v", err)
	}
	got := p.SlashCommandsFor(th.ID)
	if len(got) != 1 || got[0].Name != "mine" {
		t.Fatalf("只应入账本线程会话的广告: %+v", got)
	}
}

// TestHermesSlashLedgerSessionNewFailureClosesWindow 会话建立失败:闭窗,
// 迟到的广告不得留下"半挂载账本";SlashCommandsFor 保持 nil,绝不冷启补拉。
func TestHermesSlashLedgerSessionNewFailureClosesWindow(t *testing.T) {
	base := newFakeHermesClient(true)
	base.newErr = context.DeadlineExceeded
	fake := &hermesSlashAdvertiseFake{fakeHermesClient: base}
	// 失败后由测试侧补喂一帧本会话广告(模拟迟到帧):闭窗后必须被拒。
	p := NewHermesProvider()
	p.newClient = func() (hermesClient, error) { return fake, nil }
	th := &store.Thread{ID: "t-slash-fail"}

	_, _, err := runHermes(t, p, th, nil)
	if err == nil {
		t.Fatal("session/new 失败应如实报错")
	}
	fake.mu.Lock()
	notif := fake.notifFn
	fake.mu.Unlock()
	if notif != nil {
		// 失败路径应当已卸载钩子;即便仍在,闭窗也必须拦下。
		notif(hermesSlashFrameJSON("late-sess", `{"description":"迟到","name":"late"}`))
	}
	if got := p.SlashCommandsFor(th.ID); got != nil {
		t.Fatalf("建立失败不得留账本(闭窗失效): %+v", got)
	}
}

// ---- 写入点 2:轮次事件泵 ----

// TestHermesSlashLedgerFilledFromTurnPump 轮内重广告(未来适配器改时机/共享
// 会话路径)由泵消费侧写入账本;KNOWN_PASSTHROUGH 台账留痕必须并存(账本写入
// 与留痕不互斥)。
func TestHermesSlashLedgerFilledFromTurnPump(t *testing.T) {
	fake := newFakeHermesClient(true)
	p := newHermesTestProvider(t, fake)
	th := &store.Thread{ID: "t-slash-pump"}

	if _, _, err := runHermesText(t, p, th, nil, "commands-round 再来一轮"); err != nil {
		t.Fatalf("轮次应成功: %v", err)
	}
	got := p.SlashCommandsFor(th.ID)
	if len(got) != 2 || got[0].Name != "help" || got[1].Name != "model" {
		t.Fatalf("轮内广告必须入账: %+v", got)
	}
	if got[1].InputHint != "model name to switch to" {
		t.Fatalf("轮内广告的 inputHint 映射错: %+v", got[1])
	}
	// 台账留痕照旧(处置事实不因新增账本消费方而消失)。
	if e := findHermesLedgerEntry(t, "session/update:available_commands_update"); e.NormalizedKind != runtime.KindKnownPassthrough {
		t.Fatalf("命令广告仍应 KNOWN_PASSTHROUGH 留痕: %s", e.NormalizedKind)
	}
}

// ---- 失败面与收尸 ----

// TestHermesSlashCommandsForNeverColdStarts 未记账(daemon 刚重启/会话未建立)
// 回 nil:面板安静空着,绝不为拉清单冷启适配器(ACP 无清单 RPC,连可拉的口子
// 都没有)。
func TestHermesSlashCommandsForNeverColdStarts(t *testing.T) {
	p := NewHermesProvider()
	newCalls := 0
	p.newClient = func() (hermesClient, error) { newCalls++; return newFakeHermesClient(true), nil }
	if got := p.SlashCommandsFor("never-seen"); got != nil {
		t.Fatalf("未记账线程应回 nil: %+v", got)
	}
	if newCalls != 0 {
		t.Fatalf("SlashCommandsFor 不得冷启适配器(起了 %d 次)", newCalls)
	}
}

// TestHermesSlashLedgerDroppedWithEphemeralRef dropThreadRef(一次性优化轮收尸)
// 连带清账本:ephemeral 线程的命令面不许逐次泄漏。
func TestHermesSlashLedgerDroppedWithEphemeralRef(t *testing.T) {
	p := NewHermesProvider()
	p.setThreadRef("t-eph", hermesThreadRef{sessionID: "s-eph"})
	p.setSlashCommands("t-eph", []*pb.SlashCommand{{Name: "help"}})
	if p.SlashCommandsFor("t-eph") == nil {
		t.Fatal("测试前置:账本应已记账")
	}
	p.dropThreadRef("t-eph")
	if got := p.SlashCommandsFor("t-eph"); got != nil {
		t.Fatalf("收尸后账本必须清掉: %+v", got)
	}
}

// ---- 重挂载(session/load)窗口 ----

// TestHermesSlashLedgerRemountWindow 重挂载窗口:load 应答期间到达的广告入账;
// 历史回放帧照旧只计数(命令帧不得污染回放计数口径 —— 计数只喂日志)。
func TestHermesSlashLedgerRemountWindow(t *testing.T) {
	base := newFakeHermesClient(true)
	fake := &hermesSlashAdvertiseFake{fakeHermesClient: base,
		emitOnLoad: []json.RawMessage{hermesSlashFrameJSON("old-sess",
			`{"description":"Compress conversation context","name":"compress"}`)}}
	p := NewHermesProvider()
	p.newClient = func() (hermesClient, error) { return fake, nil }
	th := &store.Thread{ID: "t-slash-remount", ExternalSession: store.ExternalSessionRef{SessionID: "old-sess", Cwd: `C:\ws`}}

	if _, _, err := runHermes(t, p, th, nil); err != nil {
		t.Fatalf("重挂载轮应成功: %v", err)
	}
	got := p.SlashCommandsFor(th.ID)
	if len(got) != 1 || got[0].Name != "compress" {
		t.Fatalf("重挂载窗口的广告必须入账: %+v", got)
	}
}

// TestHermesSlashLedgerRemountGhostKeepsNil 幽灵会话(load 查无)硬拒绝:
// 失败路径闭窗,账本保持 nil,不静默新开也不留半挂载命令面。
func TestHermesSlashLedgerRemountGhostKeepsNil(t *testing.T) {
	base := newFakeHermesClient(true)
	base.loadErr = hermesapp.ErrSessionNotFound
	fake := &hermesSlashAdvertiseFake{fakeHermesClient: base,
		emitOnLoad: []json.RawMessage{hermesSlashFrameJSON("ghost", `{"description":"幽灵","name":"ghost"}`)}}
	p := NewHermesProvider()
	p.newClient = func() (hermesClient, error) { return fake, nil }
	th := &store.Thread{ID: "t-slash-ghost", ExternalSession: store.ExternalSessionRef{SessionID: "ghost"}}

	_, _, err := runHermes(t, p, th, nil)
	if err == nil || !strings.Contains(err.Error(), "不静默新开会话") {
		t.Fatalf("幽灵会话应显式拒绝: %v", err)
	}
	if got := p.SlashCommandsFor(th.ID); got != nil {
		t.Fatalf("失败路径不得留账本: %+v", got)
	}
}

// TestHermesSlashLedgerEmptyAdvertiseOverwrites 显式空广告覆盖旧账本:适配器
// 说"现在没有命令"(空数组)是显式事实,面板照实清空(不是"从未广告")。
func TestHermesSlashLedgerEmptyAdvertiseOverwrites(t *testing.T) {
	p := NewHermesProvider()
	p.setSlashCommands("t-overwrite", []*pb.SlashCommand{{Name: "old"}})
	_, cmds, ok := parseHermesAvailableCommands(hermesSlashFrameJSON("s", ``))
	if !ok || cmds == nil {
		t.Fatal("测试前置:空数组应解析为非 nil 空清单")
	}
	p.setSlashCommands("t-overwrite", cmds)
	got := p.SlashCommandsFor("t-overwrite")
	if got == nil || len(got) != 0 {
		t.Fatalf("显式空广告应清空面板(非 nil 空清单): %#v", got)
	}
}

// TestHermesSlashCaptureConcurrent 并发纪律:捕获器钩子(readLoop goroutine)
// 与绑定/读取并发调用下 -race 不报(锁边界:capture.mu → threadMu,单向)。
func TestHermesSlashCaptureConcurrent(t *testing.T) {
	p := NewHermesProvider()
	cap := p.beginSlashCapture(&store.Thread{ID: "t-race"})
	if cap == nil {
		t.Fatal("测试前置:带归属线程应开窗")
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				cap.handler(hermesSlashFrameJSON("s-race", `{"name":"c","description":"d"}`))
			} else {
				_ = p.SlashCommandsFor("t-race")
			}
		}(i)
	}
	wg.Wait()
	cap.bind("s-race")
	cap.commit()
	if got := p.SlashCommandsFor("t-race"); len(got) != 1 || got[0].Name != "c" {
		t.Fatalf("并发写入后账本不符: %+v", got)
	}
}
