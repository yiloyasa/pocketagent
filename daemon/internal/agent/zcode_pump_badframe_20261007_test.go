// zcode_pump_badframe_20261007_test.go —— Z-3 泵入口坏帧旁路的回归钉
// (unknown-bypass-audit.md §二 Z-3,2026-10-07 收口)。
//
// 【钉什么】Run 事件泵入口对 session/event 的解包失败与异会话过滤此前共用一条
// continue,坏帧在轮内无任何泵侧痕迹。收口后:解包失败补一条泵侧台账
// ("session/event:<unparseable>",UNKNOWN),异会话过滤维持静默(全家同款
// 刻意取舍)。裁量=坏帧只台账、不出限频 STATUS 卡:常驻全局钩子对同一帧已先落
// UNKNOWN 台账,泵侧不是唯一留痕;传输级异常帧无用户处置路径。
//
// 【怎么喂坏帧】fake zcode 脚本(zcode_image_input_test.go,同包复用)的
// PA_FAKE_ZCODE_EVENTS 支持 {rawParams}:params 原样写成 JSON 字符串,
// zcodeEventEnvelope 解包必失败。依赖本机 node,没有则跳过(与图片链路同口径)。
package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/store"
)

func TestZcodePumpBadFrameLedgerOnly(t *testing.T) {
	p, _ := newFakeZcodeProvider(t, "")
	events := []map[string]any{
		// 坏帧:params 是 JSON 字符串,信封解包必失败(全局钩子会记裸
		// "session/event" UNKNOWN,泵侧应记 <unparseable> 条目,两条共存)。
		{"rawParams": `"not-an-envelope"`},
	}
	raw, _ := json.Marshal(events)
	t.Setenv("PA_FAKE_ZCODE_EVENTS", string(raw))

	sink := &sinkCollect{}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if _, _, err := p.Run(ctx, TurnRequest{
		Thread:      &store.Thread{ID: "th-zcode-badframe", AgentID: "zcode"},
		Workspace:   t.TempDir(),
		InputBlocks: zcodeTextBlocks("只回复 PONG"),
	}, sink.add, nil); err != nil {
		t.Fatalf("坏帧不得打断轮次: %v", err)
	}

	found := false
	for _, e := range sharedRuntimeBackend().UnknownEvents().List(runtime.ListFilter{AgentID: "zcode"}, 0) {
		if e.SourceMethod != "session/event:<unparseable>" {
			continue
		}
		if e.NormalizedKind != runtime.KindUnknownNotification {
			t.Fatalf("坏帧泵侧留痕应为 UNKNOWN: %+v", e)
		}
		found = true
	}
	if !found {
		t.Fatal("泵入口坏帧未落台账(Z-3 收口失效)")
	}
	// 裁量钉:坏帧只台账,不出诊断卡(recordAs 才推卡,recordLedgerOnly 不推)。
	for _, ev := range sink.all() {
		if ce := ev.GetCodexEvent(); ce != nil && strings.Contains(ce.Title, "<unparseable>") {
			t.Fatalf("坏帧不应出诊断卡(裁量:全局钩子已有留痕,泵侧只台账): %+v", ce)
		}
	}
}
