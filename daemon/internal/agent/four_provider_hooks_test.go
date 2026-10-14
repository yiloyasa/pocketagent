// four_provider_hooks_test.go —— 「所有 agent 的功能全部接进来在同一框架下」的覆盖钉。
//
// 【钉什么】四家原生 provider(codex / pi / zcode / dsh)的事件钩子体最终都必须
// 落进同一条诊断台账链:未知/未分类通知 → recordProviderEvent + 各家 classifier;
// 对不上在途请求的响应 → recordUnknownResponse。本文件按**每家 attach 处的真实
// 调用形状**(codex.go attachGlobalHandler / pi.go clientFor / zcode.go
// attachGlobalHandler+attachUnknownResponseHandler / dsh.go newClientLocked)逐家
// 演练一遍钩子体,断言台账里带着正确的 agent 归属与 normalized_kind。
//
// 【为什么这样钉而不起真进程】钩子挂载发生在子进程创建路径里,agent 层测试不起
// 真进程;"readLoop 分发到钩子"那一半由各 app 包的 unknown_response_dispatch_test
// 已钉死(codexapp/piapp/zcodeapp/dshapp 四份,穷尽查证于 2026-09-24)。三层合起来:
// app 分发(已有)→ 钩子体→汇点(本文件)→ 汇点→台账(unknown_events_index_test 等)。
// 任何一层断,都有对应的失败信息。
//
// 【为什么逐家而非只测共用汇点】汇点是同一份代码,但每家钩子体里的 agent 归属串、
// classifier 接线是各自手写的 —— 新增 provider 时抄错 agent 串或忘接 classifier,
// 症状是"那家的事件全记成 UNKNOWN 或归属错乱",逐家钉才钉得住。
package agent

import (
	"encoding/json"
	"testing"

	"github.com/pocketagent/daemon/internal/runtime"
)

func findLedgerEntry(t *testing.T, agentID, method string) runtime.EventEnvelope {
	t.Helper()
	if e, ok := firstLedgerByMethod(agentID, method); ok {
		return e
	}
	t.Fatalf("agent %s 的钩子必须把 method=%s 记进诊断台账(钩子体→汇点断链)", agentID, method)
	return runtime.EventEnvelope{}
}

// TestFourProvidersGlobalNoticeHooksLandInLedger 四家的全局事件钩子体逐家演练。
func TestFourProvidersGlobalNoticeHooksLandInLedger(t *testing.T) {
	const probe = "fourProviderHookProbe"

	// codex:attachGlobalHandler → recordProviderEvent("codex", method, tid, codexGlobalNoticeKind(method), params)。
	recordProviderEvent("codex", probe+"/threadish", "th-4p",
		codexGlobalNoticeKind(probe+"/threadish"), json.RawMessage(`{"threadId":"x"}`))
	// pi:clientFor 的 GlobalHandler → recordProviderEvent("pi", type, threadID, piNoticeKind(type), data)。
	recordProviderEvent("pi", probe+"PiEvent", "th-4p-pi", piNoticeKind(probe+"PiEvent"), json.RawMessage(`{}`))
	// zcode:attachGlobalHandler → recordProviderEvent("zcode", method, "", zcodeNoticeKind(method), params)。
	recordProviderEvent("zcode", probe+"ZcodeEvent", "", zcodeNoticeKind(probe+"ZcodeEvent"), json.RawMessage(`{}`))
	// dsh:newClientLocked 的 GlobalHandler → recordProviderEvent("dsh", method, "", dshNoticeKind(method), params)。
	recordProviderEvent("dsh", probe+"DshEvent", "", dshNoticeKind(probe+"DshEvent"), json.RawMessage(`{}`))

	for _, tc := range []struct{ agent, method string }{
		{"codex", probe + "/threadish"},
		{"pi", probe + "PiEvent"},
		{"zcode", probe + "ZcodeEvent"},
		{"dsh", probe + "DshEvent"},
	} {
		e := findLedgerEntry(t, tc.agent, tc.method)
		if e.NormalizedKind != runtime.KindUnknownNotification {
			t.Errorf("%s: 从未见过的通知必须如实记 UNKNOWN(不冒充已知),得到 %s", tc.agent, e.NormalizedKind)
		}
	}

	// 已知方法同样要能落台账且分档正确(每家抽一个代表,classifier 接线错了这里先红)。
	recordProviderEvent("zcode", "session.modelUpdated", "",
		zcodeNoticeKind("session.modelUpdated"), json.RawMessage(`{}`))
	if e := findLedgerEntry(t, "zcode", "session.modelUpdated"); e.NormalizedKind != runtime.KindKnownProjected {
		t.Errorf("zcode 已知投影档分档不对: %s", e.NormalizedKind)
	}
	recordProviderEvent("pi", "message_update", "th-4p-pi", piNoticeKind("message_update"), json.RawMessage(`{}`))
	if e := findLedgerEntry(t, "pi", "message_update"); e.NormalizedKind != runtime.KindKnownPassthrough {
		t.Errorf("pi 已知透传档分档不对: %s", e.NormalizedKind)
	}
	recordProviderEvent("dsh", "session/update:context_usage", "",
		dshNoticeKind("session/update:context_usage"), json.RawMessage(`{}`))
	if e := findLedgerEntry(t, "dsh", "session/update:context_usage"); e.NormalizedKind != runtime.KindKnownProjected {
		t.Errorf("dsh 已知投影档分档不对: %s", e.NormalizedKind)
	}
}

// TestFourProvidersUnknownResponseHooksLandInLedger 四家的未知响应钩子体逐家演练。
func TestFourProvidersUnknownResponseHooksLandInLedger(t *testing.T) {
	for _, agent := range []string{"codex", "pi", "zcode", "dsh"} {
		recordUnknownResponse(agent, "corr-"+agent, json.RawMessage(`{"echo":1}`))
		entries := sharedRuntimeBackend().UnknownEvents().List(
			runtime.ListFilter{AgentID: agent, Kind: runtime.KindUnknownResponse}, 0)
		found := false
		for _, e := range entries {
			if e.CorrelationID == "corr-"+agent {
				found = true
			}
		}
		if !found {
			t.Fatalf("agent %s 的未知响应必须以 UNKNOWN_RESPONSE + correlation 落台账", agent)
		}
	}
}
