// knowledge_zcode_20260930_test.go —— zcode 2026-09-30 复验补齐词表条目的
// 回归钉子(走导出面 TranslateEvent,与词表实现解耦;风格照 knowledge_pi_0991_test)。
//
// 【钉什么】①台账复合键(session/event 拆壳后的登记名,SourceMethod 原样查表)
// 必须出人话 —— 此前只有平键,轮次事件在诊断页全部掉进"未知事件";
// ②顶层遥测(v4/telemetry/event、computer-use/operation-event)与已知拒绝
// (interaction/requestOfficialMcpAuthHeaders)的人话;③permission 四件套:
// 2026-09-30 时点刻意不收词表(证不出就留 UNKNOWN),2026-10-07 已按 bundle
// 静态 payload 形状翻案预收编 PASSTHROUGH(docs/reviews/2026-10-07/
// field-coverage-zcode.md §B)—— 本文件把旧断言同步翻转为"必须出人话",
// 谁把词条删了,这里就会红。
package runtime

import (
	"strings"
	"testing"
)

func TestZcodeKnowledge20260930Entries(t *testing.T) {
	cases := []struct {
		method      string
		kind        string
		wantKeyword string
	}{
		// 台账复合键(session/event:<type>,agent.zcodeLedgerMethod 拆壳后的登记名)。
		{"session/event", KindUnknownNotification, "拆不开"}, // 壳异常兜底条目也要有人话
		{"session/event:turn.started", KindKnownPassthrough, "这一轮"},
		{"session/event:turn.completed", KindKnownPassthrough, "这一轮"},
		{"session/event:turn.failed", KindKnownPassthrough, "失败"},
		{"session/event:turn.steerQueued", KindKnownPassthrough, "队列"},
		{"session/event:turn.steerDrained", KindKnownPassthrough, "插话"},
		{"session/event:model.streaming", KindKnownPassthrough, "流式"},
		{"session/event:tool.updated", KindKnownPassthrough, "工具"},
		{"session/event:session.updated", KindKnownPassthrough, "生命周期"},
		{"session/event:session.titleUpdated", KindKnownPassthrough, "标题"},
		{"session/event:session.created", KindKnownPassthrough, "会话"},
		{"session/event:session.resumed", KindKnownPassthrough, "恢复"},
		{"session/event:session.closed", KindKnownPassthrough, "关闭"},
		{"session/event:checkpoint.created", KindKnownPassthrough, "检查点"},
		{"session/event:rewind.triggered", KindKnownPassthrough, "回挡"},
		{"session/event:settings.updated", KindKnownPassthrough, "设置"},
		{"session/event:session.modelUpdated", KindKnownPassthrough, "模型"},
		{"session/event:message.upserted", KindKnownPassthrough, "消息记录"},
		{"session/event:message.removed", KindKnownPassthrough, "消息记录"},
		{"session/event:part.started", KindKnownPassthrough, "分段"},
		{"session/event:part.delta", KindKnownPassthrough, "分段"},
		{"session/event:part.upserted", KindKnownPassthrough, "分段"},
		{"session/event:part.removed", KindKnownPassthrough, "分段"},
		{"session/event:streamRecovery.updated", KindKnownPassthrough, "恢复"},
		// 顶层遥测补收。
		{"v4/telemetry/event", KindKnownPassthrough, "遥测"},
		{"computer-use/operation-event", KindKnownPassthrough, "操作镜像"},
		// 已知拒绝(反向请求,reqHandler 兜底按 UNKNOWN_REQUEST 登记)。
		{"interaction/requestOfficialMcpAuthHeaders", KindUnknownRequest, "鉴权头"},
	}
	for _, tc := range cases {
		tr := TranslateEvent("zcode", tc.method, tc.kind, `{"sample":1}`)
		if tr.Summary == "" {
			t.Errorf("%s: summary 不得为空(词表缺条目)", tc.method)
			continue
		}
		if strings.Contains(tr.Summary, "未知") {
			t.Errorf("%s: 已收编条目不得译成未知: %s", tc.method, tr.Summary)
		}
		if !strings.Contains(tr.Summary, tc.wantKeyword) {
			t.Errorf("%s: summary 缺关键词 %q: %s", tc.method, tc.wantKeyword, tr.Summary)
		}
	}
}

// 【2026-10-07 预收编批翻案】permission 四件套此前刻意不收词表(证不出就留
// UNKNOWN);上轮已取得完整 bundle 静态 payload 形状(field-coverage-zcode.md
// §B),主模型裁决收编 PASSTHROUGH —— 翻译必须出人话且带 PASSTHROUGH 处置句,
// 不得再走"未知事件"兜底。
func TestZcodePermissionQuadPassthrough20261007(t *testing.T) {
	for _, tc := range []struct{ m, keyword string }{
		{"permission.requested", "权限请求"},
		{"permission.resolved", "权限请求"},
		{"userInput.requested", "输入请求"},
		{"userInput.resolved", "输入请求"},
	} {
		tr := TranslateEvent("zcode", tc.m, KindKnownPassthrough, `{"sample":1}`)
		if strings.Contains(tr.Summary, "未知") || !strings.Contains(tr.Summary, tc.keyword) {
			t.Errorf("%s: 预收编后必须译出人话(含 %q),得到: %s", tc.m, tc.keyword, tr.Summary)
		}
		if !strings.Contains(tr.Detail, "只原样透传留痕") {
			t.Errorf("%s: PASSTHROUGH 处置句缺失: %s", tc.m, tr.Detail)
		}
	}
}
