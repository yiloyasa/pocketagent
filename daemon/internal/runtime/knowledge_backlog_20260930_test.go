// knowledge_backlog_20260930_test.go —— 2026-09-30 未知台账积压收编批的词表钉子
// (走导出面 TranslateEvent,与词表实现解耦;风格照 knowledge_zcode_20260930_test)。
//
// 【背景】活跃台账(200 条滚动窗)实测 UNKNOWN 积压三组:codex 迟到错误应答
// 24 条(收编,见 codexEventSummaries 同日批注)、dsh 裸 session/update 13 条
// (agent 侧拆壳补档,词表本就有)、remoteControl/status/changed 2 条(明确
// 不收清单内的 remoteControl/*,维持 UNKNOWN)。
//
// 【钉什么】①收编的两条 "response:<请求方法>" 复合键必须出人话,且不得译成
// "未知";②红线反向钉:裸 "response"(识别不出的迟到应答)必须仍是"未知事件"
// 口径 —— 谁把整族 response 都编成人话,这里就会红;③remoteControl/status/
// changed 维持未知口径(明确不收清单的既有决策)。
package runtime

import (
	"strings"
	"testing"
)

func TestCodexLateResponseKnowledge20260930Entries(t *testing.T) {
	cases := []struct {
		method      string
		kind        string
		wantKeyword string
	}{
		{"response:account/rateLimits/read", KindKnownPassthrough, "限额"},
		{"response:account/usage/read", KindKnownPassthrough, "用量"},
	}
	for _, tc := range cases {
		tr := TranslateEvent("codex", tc.method, tc.kind,
			`{"error":{"code":-32603,"message":"token usage profile fetch timed out"},"id":319,"jsonrpc":""}`)
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

// 红线反向钉:识别不出家族的迟到应答仍以裸 "response" + UNKNOWN_RESPONSE 入账,
// 翻译必须走"未知事件"兜底,不得被上面的复合键误伤。
func TestCodexBareResponseStaysUnknown(t *testing.T) {
	tr := TranslateEvent("codex", "response", KindUnknownResponse,
		`{"error":{"code":-32603,"message":"something we never saw"},"id":42,"jsonrpc":""}`)
	if !strings.Contains(tr.Summary, "未知") {
		t.Errorf("裸 response 必须仍译为未知事件,得到: %s", tr.Summary)
	}
	if !strings.Contains(tr.Detail, "没能对应到任何在途请求") {
		t.Errorf("UNKNOWN_RESPONSE 的处置句缺失: %s", tr.Detail)
	}
}

// remoteControl/* 在 knowledge_codex.go 文件头"明确不收清单"里(桌面远程控制,
// 手机端无消费方)—— 分类维持 UNKNOWN,翻译维持未知口径。哪天有人想收编,
// 先推翻文件头的决策并把本用例一起改,别只加词表。
func TestCodexRemoteControlStatusStaysUnknown(t *testing.T) {
	tr := TranslateEvent("codex", "remoteControl/status/changed", KindUnknownNotification,
		`{"environmentId":null,"installationId":"x","serverName":"s","status":"disabled"}`)
	if !strings.Contains(tr.Summary, "未知") {
		t.Errorf("remoteControl/status/changed 刻意不收,必须仍译为未知事件,得到: %s", tr.Summary)
	}
}
