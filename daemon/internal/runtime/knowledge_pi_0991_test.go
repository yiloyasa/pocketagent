// knowledge_pi_0991_test.go —— pi 0.99.1 词表增补条目的回归钉子(走导出面
// TranslateEvent,与 runtime 词表实现解耦)。
//
// 【证据】2026-09-30 探针:pi-0991-m99.md A7 项活体抓到
// {"type":"bash_execution_update","id":"piprobe-9","delta":"PB99-PROBE-OK\n"};
// thinking_level_changed 为 0.87.1 起既有事件、0.99.1 全集复核补收编
// (pi-0991-p0 附录 agent 事件序列)。
package runtime

import (
	"strings"
	"testing"
)

func TestPiKnowledge0991Entries(t *testing.T) {
	cases := []struct {
		method      string
		wantKeyword string
	}{
		{"bash_execution_update", "bash"},
		{"thinking_level_changed", "思考"},
	}
	for _, tc := range cases {
		tr := TranslateEvent("pi", tc.method, KindKnownPassthrough, `{"sample":1}`)
		if tr.Summary == "" {
			t.Errorf("%s: summary 不得为空(词表缺条目)", tc.method)
			continue
		}
		if strings.Contains(tr.Summary, "未知") {
			t.Errorf("%s: 已收编事件不得译成未知: %s", tc.method, tr.Summary)
		}
		if !strings.Contains(tr.Summary, tc.wantKeyword) {
			t.Errorf("%s: summary 缺关键词 %q: %s", tc.method, tc.wantKeyword, tr.Summary)
		}
	}
}
