// dsh_title_badframe_20261007_test.go —— dsh 泵两处收口的回归钉(2026-10-07):
//
//  1. D-1(=Z-3 dsh 侧):泵入口 session/update 信封解包失败与异会话过滤此前共用
//     一条 continue;收口后坏帧补泵侧台账("session/update:<unparseable>",
//     UNKNOWN),异会话维持静默。裁量=只台账不出卡(全局钩子已有留痕,见
//     unknown-bypass-audit.md §二 D-1)。
//  2. session_info_update.title → TitleHandler(zcode session.titleUpdated 同款
//     标题链,字段覆盖审计建议反馈③):带 title 记 PROJECTED,server 层落库后
//     双端同步;桥 0.2.0-rc.2 不实发该变体,本钉防的是防御面接线回退。
package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/pocketagent/daemon/internal/dshapp"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/store"
)

// TestDshPumpBadFrameLedgerOnly 坏帧落台账、不出诊断卡、不打断轮次;
// 异会话帧(缺 sessionId)维持静默过滤。
func TestDshPumpBadFrameLedgerOnly(t *testing.T) {
	fake := &fakeDshClient{
		newState: &dshapp.SessionState{SessionID: "s-bad-1", ConfigOptions: probeCatalogOpts(t)},
		promptFn: func(_ context.Context, sid string, _ []map[string]interface{}, notify func(string, json.RawMessage)) (json.RawMessage, error) {
			notify("session/update", json.RawMessage(`12345`))         // 坏帧:信封解不开
			notify("session/update", json.RawMessage(`{"update":{}}`)) // 异会话(缺 sessionId):刻意静默
			return json.RawMessage(`{"stopReason":"end_turn"}`), nil
		},
	}
	p := newTestDshProvider(t, fake)
	sink := &sinkCollect{}
	if _, _, err := p.Run(context.Background(), TurnRequest{
		Thread:      &store.Thread{ID: "th-dsh-badframe", AgentID: "dsh"},
		Workspace:   `G:\ws`,
		InputBlocks: []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "hi"}}}},
	}, sink.add, nil); err != nil {
		t.Fatalf("坏帧不得打断轮次: %v", err)
	}
	found := false
	for _, e := range sharedRuntimeBackend().UnknownEvents().List(runtime.ListFilter{AgentID: "dsh"}, 0) {
		if e.SourceMethod != "session/update:<unparseable>" {
			continue
		}
		if e.NormalizedKind != runtime.KindUnknownNotification {
			t.Fatalf("坏帧泵侧留痕应为 UNKNOWN: %+v", e)
		}
		found = true
	}
	if !found {
		t.Fatal("泵入口坏帧未落台账(D-1 收口失效)")
	}
	for _, ev := range sink.all() {
		if ce := ev.GetCodexEvent(); ce != nil && strings.Contains(ce.Title, "<unparseable>") {
			t.Fatalf("坏帧不应出诊断卡(裁量:全局钩子已有留痕,泵侧只台账): %+v", ce)
		}
	}
}

// TestDshSessionInfoTitleReachesTitleHandler session_info_update 带 title 必须
// 触发 TitleHandler(线程 ID + 标题原文透传),台账记 PROJECTED,轮次照常完成。
func TestDshSessionInfoTitleReachesTitleHandler(t *testing.T) {
	fake := &fakeDshClient{
		newState: &dshapp.SessionState{SessionID: "s-title-1", ConfigOptions: probeCatalogOpts(t)},
		promptFn: func(_ context.Context, sid string, _ []map[string]interface{}, notify func(string, json.RawMessage)) (json.RawMessage, error) {
			env, _ := json.Marshal(map[string]interface{}{
				"sessionId": sid,
				"update": map[string]interface{}{
					// ACP union 形状(types.gen.d.ts:3896):{sessionUpdate,
					// title?, updatedAt?};只给真键。
					"sessionUpdate": "session_info_update",
					"title":         "桌面起的标题",
				},
			})
			notify("session/update", env)
			return json.RawMessage(`{"stopReason":"end_turn"}`), nil
		},
	}
	p := newTestDshProvider(t, fake)
	titleCh := make(chan string, 4)
	p.TitleHandler = func(threadID, title string) {
		if title != "" {
			titleCh <- threadID + "\x00" + title
		}
	}
	sink := &sinkCollect{}
	if _, _, err := p.Run(context.Background(), TurnRequest{
		Thread:      &store.Thread{ID: "th-dsh-title", AgentID: "dsh"},
		Workspace:   `G:\ws`,
		InputBlocks: []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "hi"}}}},
	}, sink.add, nil); err != nil {
		t.Fatalf("Run: %v", err)
	}
	select {
	case got := <-titleCh:
		if want := "th-dsh-title\x00桌面起的标题"; got != want {
			t.Fatalf("TitleHandler 收到 %q, want %q(带我们的线程 ID,标题原文透传)", got, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("TitleHandler 未被触发(dsh 会话命名→手机同步链路断了)")
	}
	// 台账分类:带 title 的 session_info_update 走标题投影路径 → PROJECTED
	// (修复前落 default 变 UNKNOWN)。断言"存在 PROJECTED 条目"而非"无其他档":
	// 共享台账(进程内)会累积其它用例同方法名的条目,存在性断言才抗串扰。
	projected := false
	for _, e := range sharedRuntimeBackend().UnknownEvents().List(runtime.ListFilter{AgentID: "dsh"}, 0) {
		if e.SourceMethod == "session/update:session_info_update" && e.NormalizedKind == runtime.KindKnownProjected {
			projected = true
		}
	}
	if !projected {
		t.Fatal("带 title 的 session_info_update 未记 PROJECTED(修复前落 UNKNOWN)")
	}
}

// TestDshSessionInfoWithoutTitleNoProjection 仅 updatedAt/空 title 的已知变体:
// 不触发 TitleHandler,台账记 PASSTHROUGH(已知、无投影),诊断卡保持既有行为。
func TestDshSessionInfoWithoutTitleNoProjection(t *testing.T) {
	p := NewDshProvider()
	called := false
	p.TitleHandler = func(string, string) { called = true }
	sink := &sinkCollect{}
	notices := &nativeNoticeRecorder{}
	raw := json.RawMessage(`{"sessionUpdate":"session_info_update","updatedAt":1791000000000}`)
	p.handleSessionUpdate(raw, sink.add, &dshTurnState{}, notices)
	if called {
		t.Fatal("无 title 不得触发 TitleHandler")
	}
	// 存在性断言(共享台账抗串扰,见上一测试):无 title 的已知变体应记
	// PASSTHROUGH(已知、无投影),不得冒充未知。
	passthrough := false
	for _, e := range sharedRuntimeBackend().UnknownEvents().List(runtime.ListFilter{AgentID: "dsh"}, 0) {
		if e.SourceMethod == "session/update:session_info_update" && e.NormalizedKind == runtime.KindKnownPassthrough {
			passthrough = true
		}
	}
	if !passthrough {
		t.Fatal("无 title 的 session_info_update 未记 PASSTHROUGH(已知变体不得落 UNKNOWN)")
	}
}
