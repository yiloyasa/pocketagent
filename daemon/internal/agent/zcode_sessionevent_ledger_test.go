// zcode_sessionevent_ledger_test.go —— zcode session/event 台账治理回归钉(尾巴 4)。
//
// 【钉什么】attachGlobalHandler 先于 Run 的轮次钩子对每条通知都记一次账
// (zcodeapp 先 global 后 notif)。session/event 的 params 是
// {sessionId,type,payload,…} 外壳,分类表认的是**内部 type** —— 直接拿外层
// method="session/event" 记账,轮次期间 model.streaming/tool.updated 等每条
// 都灌 UNKNOWN。修复后:全局路径把 method 换成 "session/event:<type>"
// (dsh 的 "session/update:<kind>" 同款拼法,SourceMethod 同时保留原始方法名
// 与内部 type),分类器照常生效。
//
// 【两层钉】① zcodeLedgerMethod 纯函数拆壳/分类/兜底;② 端到端:fake zcode
// 跑一轮(fake 本来就发 model.streaming/turn.completed/turn.started),断言台账
// 里这些事件以拆壳后的名字落 PROJECTED/PASSTHROUGH,且不再有裸 "session/event"
// 的 UNKNOWN 条目。②依赖本机 node,没有则跳过(与图片链路测试同口径)。
package agent

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/store"
)

func TestZcodeLedgerMethodUnwrapsSessionEvent(t *testing.T) {
	env := func(typ string) json.RawMessage {
		return json.RawMessage(`{"sessionId":"sess_x","type":"` + typ + `","payload":{}}`)
	}
	// 内部 type 进分类表:method 换成 "session/event:<type>",分类用内部 type。
	if m, k := zcodeLedgerMethod("session/event", env("model.streaming")); m != "session/event:model.streaming" || k != runtime.KindKnownProjected {
		t.Fatalf("model.streaming 应拆壳为 session/event:model.streaming 且 PROJECTED,得到 (%q,%q)", m, k)
	}
	if m, k := zcodeLedgerMethod("session/event", env("turn.started")); m != "session/event:turn.started" || k != runtime.KindKnownPassthrough {
		t.Fatalf("turn.started 应拆壳且 PASSTHROUGH,得到 (%q,%q)", m, k)
	}
	// 顶层通知(非 session/event):原样透传,行为与修复前完全一致。
	if m, k := zcodeLedgerMethod("state.updated", env("")); m != "state.updated" || k != runtime.KindKnownProjected {
		t.Fatalf("顶层 state.updated 不应被改写,得到 (%q,%q)", m, k)
	}
	if m, k := zcodeLedgerMethod("brand.new.notice", nil); m != "brand.new.notice" || k != runtime.KindUnknownNotification {
		t.Fatalf("真未知顶层通知应保持 UNKNOWN,得到 (%q,%q)", m, k)
	}
	// 壳拆不开/type 为空:按原样 "session/event" 记 UNKNOWN —— 拆不开的壳
	// 最该让人看见,不替它编一个已知身份。
	if m, k := zcodeLedgerMethod("session/event", json.RawMessage(`{"sessionId":"sess_x"}`)); m != "session/event" || k != runtime.KindUnknownNotification {
		t.Fatalf("缺 type 应原样记 session/event 且 UNKNOWN,得到 (%q,%q)", m, k)
	}
	if m, k := zcodeLedgerMethod("session/event", json.RawMessage(`not-json`)); m != "session/event" || k != runtime.KindUnknownNotification {
		t.Fatalf("壳无法解析应原样记 session/event 且 UNKNOWN,得到 (%q,%q)", m, k)
	}
}

// 端到端:fake zcode 跑一轮(它会发 turn.started → model.streaming →
// turn.completed),断言全局路径记的台账按拆壳名分类,不再有裸 session/event 的
// UNKNOWN。fake 脚本由 zcode_image_input_test.go 提供(同包复用)。
func TestZcodeSessionEventLedgerUnwrappedEndToEnd(t *testing.T) {
	p, _ := newFakeZcodeProvider(t, "")
	ws := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_, _, err := p.Run(ctx, TurnRequest{
		Thread:      &store.Thread{ID: "th-zcode-ledger", AgentID: "zcode"},
		Workspace:   ws,
		InputBlocks: zcodeTextBlocks("只回复 PONG"),
	}, func(m *pb.TurnServerMessage) {}, nil)
	if err != nil {
		t.Fatalf("fake zcode 一轮应成功: %v", err)
	}
	entries := sharedRuntimeBackend().UnknownEvents().List(runtime.ListFilter{AgentID: "zcode"}, 0)

	byMethod := map[string]string{}
	for _, e := range entries {
		byMethod[e.SourceMethod] = e.NormalizedKind
		// 红线:全局路径不再产生裸 "session/event" 的 UNKNOWN(那正是本轮治理的病)。
		if e.SourceMethod == "session/event" && e.NormalizedKind == runtime.KindUnknownNotification {
			t.Fatalf("台账仍有裸 session/event 的 UNKNOWN: %+v", e)
		}
	}
	for m, want := range map[string]string{
		"session/event:model.streaming": runtime.KindKnownProjected,
		"session/event:turn.completed":  runtime.KindKnownProjected,
		"session/event:turn.started":    runtime.KindKnownPassthrough,
	} {
		if got, ok := byMethod[m]; !ok {
			t.Fatalf("台账缺 %s(fake 一轮必发,全局钩子应记账)", m)
		} else if got != want {
			t.Fatalf("%s 分类应为 %q,得到 %q", m, want, got)
		}
	}
}
