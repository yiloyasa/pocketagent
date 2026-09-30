// pi_sessioninfo_kind_20261001_test.go —— 2026-10-01 通宵采样批的 pi 分类钉子。
//
// 【背景】活跃台账(unknown-events.json)实测 pi session_info_changed 落了一条
// UNKNOWN(seq 1291):词表 knowledge_pi.go 早有词条、GlobalHandler 也在真实消费它
// (MapSessionNameChange → TitleHandler → server 落库双端标题同步),但
// piEventNoticeKind / piNoticeKind 两张分类表都没档 —— "已知且已消费"被记成未知。
//
// 【钉什么】session_info_changed 补归 PROJECTED(标题同步是真实投影,与 zcode
// session.titleUpdated 同语义);邻位事件保持原档;真未知仍回落 UNKNOWN。
package agent

import (
	"testing"

	"github.com/pocketagent/daemon/internal/runtime"
)

func TestPiEventNoticeKindSessionInfoChanged20261001(t *testing.T) {
	if got := piEventNoticeKind("session_info_changed"); got != runtime.KindKnownProjected {
		t.Errorf("piEventNoticeKind(session_info_changed) = %q, want PROJECTED(标题同步是真实投影,通宵采样批补档)", got)
	}
	// 邻位回归:0.99.1 增补与老事件的原档不得被本批改动波及。
	for _, m := range []string{"turn_start", "message_end", "bash_execution_update", "thinking_level_changed"} {
		if got := piEventNoticeKind(m); got != runtime.KindKnownPassthrough {
			t.Errorf("piEventNoticeKind(%q) = %q, want PASSTHROUGH(原档回归)", m, got)
		}
	}
	if got := piEventNoticeKind("message_update"); got != runtime.KindKnownPassthrough {
		t.Errorf("piEventNoticeKind(message_update) = %q, want PASSTHROUGH(piNoticeKind 老表回归)", got)
	}
	// 真没见过的事件仍如实 UNKNOWN。
	if got := piEventNoticeKind("pi/probe/neverSeen"); got != runtime.KindUnknownNotification {
		t.Errorf("piEventNoticeKind(未见过) = %q, want UNKNOWN(不冒充已知)", got)
	}
}
