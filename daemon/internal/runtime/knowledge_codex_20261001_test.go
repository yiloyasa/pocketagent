// knowledge_codex_20261001_test.go —— 2026-10-01 通宵采样批的 codex 词表钉子
// (走导出面 TranslateEvent,与词表实现解耦;风格照 knowledge_backlog_20260930_test)。
//
// 【背景】codexliveturnprobe 活体单轮(0.159.2)通知序列 46 条逐一对表,轮次骨架
// 族此前无词条,真到达时诊断页只能给"未知事件"兜底。本批词表补十条(分类批见
// codex.go 的 codexGlobalNoticeKind,与本表一一对应),逐条钉"必须出人话、不得
// 译成未知、关键词在位";kind 用分类器同款值,保证诊断页分类/翻译两张皮不脱节。
package runtime

import (
	"strings"
	"testing"
)

func TestCodexTurnSkeletonKnowledge20261001Entries(t *testing.T) {
	cases := []struct {
		method      string
		kind        string
		wantKeyword string
	}{
		{"turn/started", KindKnownPassthrough, "开始"},
		{"turn/completed", KindKnownProjected, "完成"},
		{"item/started", KindKnownProjected, "条目"},
		{"item/completed", KindKnownProjected, "条目"},
		{"item/agentMessage/delta", KindKnownProjected, "流式"},
		{"item/reasoning/summaryTextDelta", KindKnownProjected, "思考"},
		{"item/commandExecution/outputDelta", KindKnownProjected, "输出"},
		{"thread/tokenUsage/updated", KindKnownProjected, "token"},
		{"account/rateLimits/updated", KindKnownProjected, "限制"},
		{"mcpServer/startupStatus/updated", KindKnownProjected, "mcp"},
	}
	for _, tc := range cases {
		tr := TranslateEvent("codex", tc.method, tc.kind, `{"threadId":"th-probe"}`)
		if tr.Summary == "" {
			t.Errorf("%s: summary 不得为空(词表缺条目)", tc.method)
			continue
		}
		if strings.Contains(tr.Summary, "未知") {
			t.Errorf("%s: 已收编条目不得译成未知: %s", tc.method, tr.Summary)
		}
		if !strings.Contains(strings.ToLower(tr.Summary), tc.wantKeyword) {
			t.Errorf("%s: summary 缺关键词 %q: %s", tc.method, tc.wantKeyword, tr.Summary)
		}
	}
}

// 红线反向钉:本批扩容不得波及"明确不收清单"——桌面 MCP 流
// (mcpServer/event/stream)与幻影方法仍走未知口径。
func TestCodexUnclaimedNoticesStayUnknown20261001(t *testing.T) {
	for _, m := range []string{"mcpServer/event/stream", "thread/plan/updated"} {
		tr := TranslateEvent("codex", m, KindUnknownNotification, `{"x":1}`)
		if !strings.Contains(tr.Summary, "未知") {
			t.Errorf("%s: 刻意不收/幻影方法必须仍译为未知事件,得到: %s", m, tr.Summary)
		}
	}
}
