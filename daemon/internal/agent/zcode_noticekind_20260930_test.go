// zcode_noticekind_20260930_test.go —— 2026-09-30 zcode 复验收编批的分类钉子。
//
// 【钉什么】词表(knowledge_zcode.go)2026-09-30 已收 union 新面孔
// (message.*/part.*/streamRecovery.updated)与顶层遥测(v4/telemetry/event、
// computer-use/operation-event),但 zcodeNoticeKind 的收编批当时漏掉这九个键,
// 真出现仍灌 UNKNOWN。本文件钉:①九键 = PASSTHROUGH(只留痕不投影);
// ②拆壳接线(zcodeLedgerMethod)把复合键与分档一起带对;③台账落库 kind 与
// 分类一致(接线钉,防"分类函数对、写入侧仍回落 UNKNOWN");④红线反向钉:
// permission 四件套与反向请求 interaction/requestOfficialMcpAuthHeaders
// 维持 UNKNOWN(证不出就留 UNKNOWN,不猜)。
//
// 【证据口径】zcprobe 全子模式重跑 + bundle 静态 zod 枚举(见
// knowledge_zcode.go 2026-09-30 同日注释)。
package agent

import (
	"encoding/json"
	"testing"

	"github.com/pocketagent/daemon/internal/runtime"
)

func TestZcodeNoticeKind20260930Batch(t *testing.T) {
	cases := []struct {
		method string
		why    string
	}{
		{"message.upserted", "union 新面孔:桌面记录流镜像,daemon 只留痕"},
		{"message.removed", "union 新面孔:桌面记录流镜像,daemon 只留痕"},
		{"part.started", "union 新面孔:记录流分段粒度,daemon 只留痕"},
		{"part.delta", "union 新面孔:流式正文唯一来源仍是 model.streaming,daemon 只留痕"},
		{"part.upserted", "union 新面孔:记录流分段粒度,daemon 只留痕"},
		{"part.removed", "union 新面孔:记录流分段粒度,daemon 只留痕"},
		{"streamRecovery.updated", "union 新面孔:断流重连状态,daemon 只留痕"},
		{"v4/telemetry/event", "0.16.9 实测顶层遥测(一次委派轮 8 条),当时收编批漏掉"},
		{"computer-use/operation-event", "0.16.9 实测顶层操作镜像遥测,当时收编批漏掉"},
	}
	for _, c := range cases {
		if got := zcodeNoticeKind(c.method); got != runtime.KindKnownPassthrough {
			t.Errorf("zcodeNoticeKind(%q) = %q, want PASSTHROUGH(%s)", c.method, got, c.why)
		}
	}
}

// 拆壳接线:union 新面孔经 zcodeLedgerMethod 必须以复合键登记、按内部 type 分档;
// 顶层遥测原样透传分类。只测新键(旧键已由 zcode_sessionevent_ledger_test.go 钉住)。
func TestZcodeLedgerMethod20260930Keys(t *testing.T) {
	env := func(typ string) json.RawMessage {
		return json.RawMessage(`{"sessionId":"sess_x","type":"` + typ + `","payload":{}}`)
	}
	cases := []struct {
		method string
		params json.RawMessage
		wantM  string
		wantK  string
	}{
		{"session/event", env("part.delta"), "session/event:part.delta", runtime.KindKnownPassthrough},
		{"session/event", env("message.upserted"), "session/event:message.upserted", runtime.KindKnownPassthrough},
		{"session/event", env("streamRecovery.updated"), "session/event:streamRecovery.updated", runtime.KindKnownPassthrough},
		{"v4/telemetry/event", env(""), "v4/telemetry/event", runtime.KindKnownPassthrough},
		{"computer-use/operation-event", env(""), "computer-use/operation-event", runtime.KindKnownPassthrough},
	}
	for _, c := range cases {
		m, k := zcodeLedgerMethod(c.method, c.params)
		if m != c.wantM || k != c.wantK {
			t.Errorf("zcodeLedgerMethod(%q) = (%q,%q), want (%q,%q)", c.method, m, k, c.wantM, c.wantK)
		}
	}
}

// 台账落库钉:走与生产全局钩子同一对(zcodeLedgerMethod + recordProviderEvent),
// 断言九个新键在台账里的 normalized_kind 确为 PASSTHROUGH。
func TestZcode20260930BatchLandsInLedger(t *testing.T) {
	const th = "th-zcode-20260930"
	env := func(typ string) json.RawMessage {
		return json.RawMessage(`{"sessionId":"sess_x","type":"` + typ + `","payload":{}}`)
	}
	type in struct {
		method string
		params json.RawMessage
	}
	inputs := []in{
		{"session/event", env("message.upserted")},
		{"session/event", env("message.removed")},
		{"session/event", env("part.started")},
		{"session/event", env("part.delta")},
		{"session/event", env("part.upserted")},
		{"session/event", env("part.removed")},
		{"session/event", env("streamRecovery.updated")},
		{"v4/telemetry/event", json.RawMessage(`{}`)},
		{"computer-use/operation-event", json.RawMessage(`{}`)},
	}
	for _, it := range inputs {
		m, k := zcodeLedgerMethod(it.method, it.params)
		recordProviderEvent("zcode", m, th, k, it.params)
	}
	want := map[string]bool{}
	for _, it := range inputs {
		m, _ := zcodeLedgerMethod(it.method, it.params)
		want[m] = true
	}
	seen := 0
	for _, e := range sharedRuntimeBackend().UnknownEvents().List(runtime.ListFilter{ThreadID: th}, 0) {
		if !want[e.SourceMethod] {
			continue
		}
		seen++
		if e.NormalizedKind != runtime.KindKnownPassthrough {
			t.Errorf("%s 台账 normalized_kind = %q, want PASSTHROUGH", e.SourceMethod, e.NormalizedKind)
		}
	}
	if seen != len(want) {
		t.Fatalf("有新键没落台账: seen=%d want=%d", seen, len(want))
	}
}

// 红线反向钉(不猜):
//   - permission.requested/resolved、userInput.requested/resolved 已于
//     2026-10-07 预收编批翻案为 PASSTHROUGH(bundle 静态形状到手,见
//     notification_kinds_test.go 与 knowledge_zcode.go 新批),移出本清单;
//   - interaction/requestOfficialMcpAuthHeaders 是**反向请求**,生产路径走
//     reqHandler 兜底硬编码 UNKNOWN_REQUEST 登记(zcode.go,zcode 侧尚无 request
//     分类器),本分类器只管通知 —— 它出现在通知分类表里才是错的。
func TestZcodeNoticeKindUntouchedUnknowns(t *testing.T) {
	for _, m := range []string{
		"interaction/requestOfficialMcpAuthHeaders",
	} {
		if got := zcodeNoticeKind(m); got != runtime.KindUnknownNotification {
			t.Errorf("zcodeNoticeKind(%q) = %q, want UNKNOWN(证不出就留 UNKNOWN,不猜)", m, got)
		}
	}
	// 2026-10-07 收编对照:四件套如今必须 PASSTHROUGH(防回潮)。
	for _, m := range []string{
		"permission.requested", "permission.resolved",
		"userInput.requested", "userInput.resolved",
	} {
		if got := zcodeNoticeKind(m); got != runtime.KindKnownPassthrough {
			t.Errorf("zcodeNoticeKind(%q) = %q, want PASSTHROUGH(2026-10-07 预收编批)", m, got)
		}
	}
}
