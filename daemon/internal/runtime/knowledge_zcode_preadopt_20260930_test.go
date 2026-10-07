// knowledge_zcode_preadopt_20260930_test.go —— zcode 2026-09-30 预收编批词表
// 条目的回归钉子(走导出面 TranslateEvent,风格照 knowledge_zcode_20260930_test)。
//
// 【钉什么】process/toolExecResource、plugins/operationProgress、
// interaction/providerRuntimeHeadersCancelled 三名(bundle 顶层通知注册表静态
// 在案、无实测帧)已按 process/* 遥测家族口径预收编:①平键词表必须出人话,
// 不得再译成"未知事件";②红线反向钉:未收编的同族新名(process/brandNewSample)
// 必须仍是"未知事件"口径 —— 预收编只限注册表在案的三名,不许顺手把兜底放宽
// (permission 四件套 2026-10-07 已按新证据另行预收编为 PASSTHROUGH,移出本
// 钉,见 knowledge_preadopt_20261007_test.go 与 knowledge_zcode.go 同日批)。
package runtime

import (
	"strings"
	"testing"
)

func TestZcodeKnowledgePreAdopt20260930Entries(t *testing.T) {
	cases := []struct {
		method      string
		kind        string
		wantKeyword string
	}{
		{"process/toolExecResource", KindKnownPassthrough, "工具执行资源"},
		{"plugins/operationProgress", KindKnownPassthrough, "插件操作进度"},
		{"interaction/providerRuntimeHeadersCancelled", KindKnownPassthrough, "鉴权头"},
	}
	for _, tc := range cases {
		tr := TranslateEvent("zcode", tc.method, tc.kind, `{"sample":1}`)
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

// 红线反向钉:预收编只限注册表在案的三名 —— 同族未在案新名必须仍走"未知
// 事件"兜底(permission 四件套 2026-10-07 已另行预收编为 PASSTHROUGH,移出
// 本清单;近似名不因收编而放宽的红线由 knowledge_preadopt_20261007_test 续钉)。
func TestZcodeKnowledgePreAdoptDoesNotWidenFallback(t *testing.T) {
	for _, m := range []string{
		"process/brandNewSample", "plugins/brandNewProgress",
		"interaction/brandNewCancelled",
	} {
		tr := TranslateEvent("zcode", m, KindUnknownNotification, `{"sample":1}`)
		if !strings.Contains(tr.Summary, "未知") {
			t.Errorf("%s: 未收编名必须仍译为未知事件,得到: %s", m, tr.Summary)
		}
	}
}
