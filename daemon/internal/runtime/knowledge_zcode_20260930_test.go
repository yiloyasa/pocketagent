// knowledge_zcode_20260930_test.go —— zcode 2026-09-30 复验补齐词表条目的
// 回归钉子(走导出面 TranslateEvent,与词表实现解耦;风格照 knowledge_pi_0991_test)。
//
// 【钉什么】①台账复合键(session/event 拆壳后的登记名,SourceMethod 原样查表)
// 必须出人话 —— 此前只有平键,轮次事件在诊断页全部掉进"未知事件";
// ②顶层遥测(v4/telemetry/event、computer-use/operation-event)与已知拒绝
// (interaction/requestOfficialMcpAuthHeaders)的人话;③红线反向钉:
// permission 四件套刻意不收词表(证不出就留 UNKNOWN,不猜),翻译必须仍是
// "未知事件"口径 —— 谁哪天给它们编了人话,这里就会红。
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

// 红线反向钉:permission 四件套刻意不收词表(union 在,投影形状需带审批的
// 真实轮次实证)—— 翻译必须仍走"未知事件"兜底,Detail 不冒充已知处置。
func TestZcodePermissionQuadStaysUnknown(t *testing.T) {
	for _, m := range []string{"permission.requested", "permission.resolved", "userInput.requested", "userInput.resolved"} {
		tr := TranslateEvent("zcode", m, KindUnknownNotification, `{"sample":1}`)
		if !strings.Contains(tr.Summary, "未知") {
			t.Errorf("%s: 未实证四件套必须仍译为未知事件,得到: %s", m, tr.Summary)
		}
	}
}
