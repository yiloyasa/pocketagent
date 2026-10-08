// hermes_pump_badframe_20261009_test.go —— B4「recordBadFrame 四家接线」的 hermes
// 回归钉(2026-10-09):hermes 没有常驻全局钩子,泵侧是坏帧的唯一留痕。
//
// 【钉什么】Run 通知生产者对 session/update 信封解不开的帧(ParseSessionUpdate
// 失败全零返回),此前与"异会话/异形帧"共用一条静默 return,轮内无任何痕迹。
// 收口后:JSON 本身解不开 = 坏帧 → 共享 recordBadFrame(台账 UNKNOWN + 限频
// STATUS 卡,hermes 裁量=出卡,理由见 native_notifications.go);解得开但没有
// sessionId 的异形帧维持静默(全家同款取舍)。
package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/pocketagent/daemon/internal/hermesapp"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/store"
)

// hermesBadframeFake Prompt 脚本:喂一条真坏帧 + 一条无 sessionId 的异形帧,
// 再正常收口(坏帧不得打断轮次)。
type hermesBadframeFake struct {
	*fakeHermesClient
}

func (f *hermesBadframeFake) Prompt(_ context.Context, sessionID, text string) (*hermesapp.PromptOutcome, error) {
	f.mu.Lock()
	notif := f.notifFn
	f.mu.Unlock()
	if notif != nil {
		// 先给一段正常正文(Run 终局要求有产出),再喂坏帧与异形帧。
		notif(json.RawMessage(`{"sessionId":"` + sessionID + `","update":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"正文片段"}}}`))
		notif(json.RawMessage(`12345`))         // 坏帧:非 JSON 对象,信封解不开
		notif(json.RawMessage(`{"update":{}}`)) // 异形帧:解得开但无 sessionId → 静默
	}
	return &hermesapp.PromptOutcome{StopReason: "end_turn"}, nil
}

func TestHermesPumpBadFrameCardAndLedger(t *testing.T) {
	fake := &hermesBadframeFake{fakeHermesClient: newFakeHermesClient(false)}
	p := NewHermesProvider()
	p.newClient = func() (hermesClient, error) { return fake, nil } // 包装体整体注入,覆盖 Prompt 才生效
	sink := &sinkCollect{}
	if _, _, err := p.Run(context.Background(), TurnRequest{
		Thread:      &store.Thread{ID: "th-hermes-badframe", AgentID: "hermes"},
		Workspace:   `C:\ws`,
		InputBlocks: []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "hi"}}}},
	}, sink.add, nil); err != nil {
		t.Fatalf("坏帧不得打断轮次: %v", err)
	}

	// 台账:恰好一条 <unparseable>(异形帧不得混入),归 UNKNOWN。
	found := 0
	for _, e := range sharedRuntimeBackend().UnknownEvents().List(runtime.ListFilter{AgentID: "hermes"}, 0) {
		if e.SourceMethod != "session/update:<unparseable>" {
			continue
		}
		if e.NormalizedKind != runtime.KindUnknownNotification {
			t.Fatalf("坏帧泵侧留痕应为 UNKNOWN: %+v", e)
		}
		found++
	}
	if found != 1 {
		t.Fatalf("坏帧应恰好落一条 <unparseable> 台账(异形帧静默), 实得 %d", found)
	}
	// 诊断卡:hermes 裁量=出卡(泵侧唯一留痕),卡上可见 <unparseable> 标记。
	card := false
	for _, ev := range sink.all() {
		if ce := ev.GetCodexEvent(); ce != nil && strings.Contains(ce.Title, "<unparseable>") {
			card = true
		}
	}
	if !card {
		t.Fatal("hermes 坏帧应出限频诊断卡(泵侧唯一留痕,裁量与 zcode/dsh 相反)")
	}
}
