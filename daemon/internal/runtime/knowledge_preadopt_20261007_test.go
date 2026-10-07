// knowledge_preadopt_20261007_test.go —— 2026-10-07 预收编轮的词表钉子(zcode
// v4 三连 + permission/userInput 四件套、codex rawResponse 两连),走导出面
// TranslateEvent,风格照 knowledge_zcode_20260930_test。
//
// 【证据】docs/reviews/2026-10-07/field-coverage-zcode.md §A/§B(bundle 静态
// payload 形状)、field-coverage-codex.md §2/§3(0.160.0 二进制通知枚举表序 +
// codex-schema/v2/RawResponse*.json "Internal-only" 双证)。未运行任何程序。
package runtime

import (
	"strings"
	"testing"
)

// zcode 侧:7 个平键(v4 三连顶层通知 + permission/userInput 四件套轮内平键)
// 与 4 个复合键(全局钩子拆壳后的登记名,SourceMethod 原样查表)都必须出人话,
// 且 Detail 带 PASSTHROUGH 处置句。
func TestZcodeKnowledgePreAdopt20261007Entries(t *testing.T) {
	cases := []struct {
		method      string
		wantKeyword string
	}{
		{"v4/conversation/frame", "对话帧"},
		{"v4/telemetry/local-ttft", "首字延迟"},
		{"v4/cua/permission-observation", "权限"},
		{"permission.requested", "权限请求"},
		{"permission.resolved", "权限请求"},
		{"userInput.requested", "输入请求"},
		{"userInput.resolved", "输入请求"},
		// 复合键:全局钩子把 session/event 拆壳登记成 "session/event:<type>"。
		{"session/event:permission.requested", "权限请求"},
		{"session/event:permission.resolved", "权限请求"},
		{"session/event:userInput.requested", "输入请求"},
		{"session/event:userInput.resolved", "输入请求"},
	}
	for _, tc := range cases {
		tr := TranslateEvent("zcode", tc.method, KindKnownPassthrough, `{"sample":1}`)
		if tr.Summary == "" {
			t.Errorf("%s: summary 不得为空(词表缺条目)", tc.method)
			continue
		}
		if strings.Contains(tr.Summary, "未知") {
			t.Errorf("%s: 预收编条目不得译成未知: %s", tc.method, tr.Summary)
		}
		if !strings.Contains(tr.Summary, tc.wantKeyword) {
			t.Errorf("%s: summary 缺关键词 %q: %s", tc.method, tc.wantKeyword, tr.Summary)
		}
		if !strings.Contains(tr.Detail, "只原样透传留痕") {
			t.Errorf("%s: PASSTHROUGH 处置句缺失: %s", tc.method, tr.Detail)
		}
	}
}

// codex 侧:rawResponse 两连(0.160.0 内部通知)必须有词条,不得译成未知;
// 处置句如实写 PASSTHROUGH(只留痕,用量/条目可见面另有正式载体)。
func TestCodexKnowledgePreAdopt20261007Entries(t *testing.T) {
	for _, tc := range []struct{ method, keyword string }{
		{"rawResponse/completed", "原始用量"},
		{"rawResponseItem/completed", "条目"},
	} {
		tr := TranslateEvent("codex", tc.method, KindKnownPassthrough, `{"threadId":"t","usage":null}`)
		if tr.Summary == "" || strings.Contains(tr.Summary, "未知") {
			t.Errorf("%s: 词表缺条目或译成未知: %q", tc.method, tr.Summary)
			continue
		}
		if !strings.Contains(tr.Summary, tc.keyword) {
			t.Errorf("%s: summary 缺关键词 %q: %s", tc.method, tc.keyword, tr.Summary)
		}
		if !strings.Contains(tr.Detail, "只原样透传留痕") {
			t.Errorf("%s: PASSTHROUGH 处置句缺失: %s", tc.method, tr.Detail)
		}
	}
}

// 红线反向钉:预收编只限在册精确名(v4 三连、permission 四件套)与 codex 的
// rawResponse 前缀族;同形新名不得被词表误洗成已知。
func TestKnowledgePreAdopt20261007DoesNotWiden(t *testing.T) {
	for _, c := range []struct{ agent, method string }{
		{"zcode", "v4/telemetry/brandNewFact"},
		{"zcode", "permission.brandNew"},
		{"zcode", "userInput.brandNew"},
		{"codex", "brand/new/notice"},
	} {
		tr := TranslateEvent(c.agent, c.method, KindUnknownNotification, `{"sample":1}`)
		if !strings.Contains(tr.Summary, "未知") {
			t.Errorf("%s %s: 未收编名必须仍译为未知事件,得到: %s", c.agent, c.method, tr.Summary)
		}
	}
}
