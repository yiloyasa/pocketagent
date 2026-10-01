// knowledge_codex_20261002_test.go —— 2026-10-02 深接入差距修复批的 codex 词表钉子
// (走导出面 TranslateEvent,与词表实现解耦;风格照 knowledge_codex_20261001_test)。
//
// 【背景】vendored 0.159.2 ServerNotification 逐名 diff 对照 daemon 事件翻译层:
// 14 个方法早有投影分支(codex_events.go / codex_events_ext.go / Run 的 changeFiles),
// 但分类表漏登记、词表没词条 —— 真到达时诊断页只能给"未知事件"兜底。同批已在
// codex.go 的 codexGlobalNoticeKinds 补登记(全 PROJECTED),此处逐条钉"必须出
// 人话、不得译成未知、关键词在位"。活体证据:
// docs/verify/evidence/codex-01592-liveturn-20261002.log。
package runtime

import (
	"strings"
	"testing"
)

func TestCodexDeepIntegrationKnowledge20261002Entries(t *testing.T) {
	cases := []struct {
		method      string
		wantKeyword string
	}{
		{"item/autoApprovalReview/started", "审查者"},
		{"item/autoApprovalReview/completed", "审查者"},
		{"autoApprovalReview/strictReviewRequired", "送审"},
		{"item/plan/delta", "计划"},
		{"command/exec/outputDelta", "输出"},
		{"process/outputDelta", "输出"},
		{"item/fileChange/outputDelta", "输出"},
		{"item/fileChange/patchUpdated", "文件"},
		{"item/mcpToolCall/progress", "工具"},
		{"item/reasoning/textDelta", "思考"},
		{"model/rerouted", "模型"},
		{"model/safetyBuffering/updated", "缓冲"},
		{"thread/compacted", "压缩"},
		{"windows/worldWritableWarning", "目录"},
	}
	for _, tc := range cases {
		tr := TranslateEvent("codex", tc.method, KindKnownProjected, `{"threadId":"th-cx-1002"}`)
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

// 红线反向钉:本批扩容不得波及"明确不收清单"——桌面 Projects 面
// (project/changed 与 0.159.2 新增的 thread/project/updated)仍走未知口径;
// 幻影 thread/plan/updated 同样不得被误收。
func TestCodexUnclaimedNoticesStayUnknown20261002(t *testing.T) {
	for _, m := range []string{"project/changed", "thread/project/updated", "thread/plan/updated"} {
		tr := TranslateEvent("codex", m, KindUnknownNotification, `{"x":1}`)
		if !strings.Contains(tr.Summary, "未知") {
			t.Errorf("%s: 刻意不收/幻影方法必须仍译为未知事件,得到: %s", m, tr.Summary)
		}
	}
}
