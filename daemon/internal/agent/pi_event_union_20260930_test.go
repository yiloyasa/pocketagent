// pi_event_union_20260930_test.go —— pi 0.99.1 事件全集 × 分类两表的对表钉子。
//
// 【为什么有这个测试】2026-09-30 留观收尾批把两张分类表(piEventNoticeKind /
// piNoticeKind)与 pi 0.99.1 的**权威事件全集**逐一对表:
//   - 静态全集:pi 随包 docs/json.md(agent/turn 5 + message 3 + tool 3 +
//     queue/state 4 + compaction 2 + retry 5 六节 + RPC-only 2 类),共 24 类;
//   - 活体全集:piprobe -mode=m99 一轮(2026-09-30,14/14 通过)实际到达的顶层
//     事件全部命中下表;其余嵌套形状(assistantMessageEvent 的 text*/thinking*/
//     toolcall*、get_tree 条目的 model_change/thinking_level_change)不是顶层
//     通知,永远不会进分类器,故不入表。
//
// 此后 pi 新增事件(如 0.99.2+)时,本测试的表就是"必须补档"的清单 —— 谁漏了
// 谁的 UNKNOWN 会先在这里红,而不是灌进诊断页才被发现。
package agent

import (
	"testing"

	"github.com/pocketagent/daemon/internal/runtime"
)

// piEventUnion0991 pi 0.99.1 权威事件全集(docs/json.md 六节 + RPC-only),
// 注释标每类的分档落点,便于逐条对表。
var piEventUnion0991 = map[string]string{
	// agent/turn 事件(piNoticeKind)
	"agent_start":   runtime.KindKnownPassthrough,
	"agent_end":     runtime.KindKnownPassthrough,
	"agent_settled": runtime.KindKnownPassthrough,
	"turn_start":    runtime.KindKnownPassthrough, // piEventNoticeKind
	"turn_end":      runtime.KindKnownPassthrough, // piEventNoticeKind
	// message 事件
	"message_start":  runtime.KindKnownPassthrough, // piEventNoticeKind
	"message_end":    runtime.KindKnownPassthrough, // piEventNoticeKind
	"message_update": runtime.KindKnownPassthrough, // piNoticeKind(Run 主投影,台账侧留痕)
	// tool 事件(piNoticeKind)
	"tool_execution_start":  runtime.KindKnownPassthrough,
	"tool_execution_update": runtime.KindKnownPassthrough,
	"tool_execution_end":    runtime.KindKnownPassthrough,
	// queue/state 事件
	"queue_update":           runtime.KindKnownPassthrough, // piNoticeKind
	"entry_appended":         runtime.KindKnownPassthrough, // piEventNoticeKind
	"session_info_changed":   runtime.KindKnownProjected,   // 标题同步是真实投影(pi.go 补档批)
	"thinking_level_changed": runtime.KindKnownPassthrough, // piEventNoticeKind(0.99.1 补收编)
	// compaction 事件(piNoticeKind;Run 路径另有 MapStatusEvent 投影,双路不冲突)
	"compaction_start": runtime.KindKnownPassthrough,
	"compaction_end":   runtime.KindKnownPassthrough,
	// retry 事件
	"auto_retry_start":                  runtime.KindKnownPassthrough, // piNoticeKind
	"auto_retry_end":                    runtime.KindKnownPassthrough, // piNoticeKind
	"summarization_retry_scheduled":     runtime.KindKnownPassthrough, // piEventNoticeKind
	"summarization_retry_attempt_start": runtime.KindKnownPassthrough, // piEventNoticeKind
	"summarization_retry_finished":      runtime.KindKnownPassthrough, // piEventNoticeKind
	// RPC-only 事件
	"bash_execution_update": runtime.KindKnownPassthrough, // piEventNoticeKind(留观收编:daemon 不发 bash RPC)
	"extension_error":       runtime.KindKnownPassthrough, // 两表都有档(Run 另投影 ERROR 卡)
}

// TestPiEventUnion0991Classified 权威全集 24 类逐一过 piEventNoticeKind(生产所有
// pi 记录路径的统一入口),任何一类落 UNKNOWN 即失守。
func TestPiEventUnion0991Classified(t *testing.T) {
	for et, want := range piEventUnion0991 {
		if got := piEventNoticeKind(et); got != want {
			t.Errorf("piEventNoticeKind(%q) = %q, want %q", et, got, want)
		}
	}
	if len(piEventUnion0991) != 24 {
		t.Errorf("0.99.1 权威全集应 24 类(docs/json.md 对表),现在 %d 类 —— 若是升级后的增补,请同步更新注释与 knowledge_pi.go 词表", len(piEventUnion0991))
	}
}

// TestPiLegacyEventsStillClassified 兜底防回滚:abort 在 0.99.1 全集已消失
// (piprobe 实证),词条保留防旧版 pi 回连;extension_ui_request 是 RPC 扩展
// UI 子协议的载体帧;两者与"没见过的事件"必须分得开。
func TestPiLegacyEventsStillClassified(t *testing.T) {
	for _, et := range []string{"abort", "extension_ui_request"} {
		if got := piEventNoticeKind(et); got != runtime.KindKnownPassthrough {
			t.Errorf("piEventNoticeKind(%q) = %q, want PASSTHROUGH(兜底词条不得丢)", et, got)
		}
	}
	if got := piEventNoticeKind("brand_new_0992_event"); got != runtime.KindUnknownNotification {
		t.Errorf("全集外事件应落 UNKNOWN,得到 %q(否则新事件漏档不会被发现)", got)
	}
}
