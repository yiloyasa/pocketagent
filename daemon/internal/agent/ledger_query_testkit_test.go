// ledger_query_testkit_test.go —— 台账查询的共享测试内核。
//
// 【它解决什么】"按 agent+method 找台账条目"这段 List+匹配循环曾有三份拷贝:
// unknown_passthrough_test 的 ledgerEntries、four_provider_hooks_test 的
// findLedgerEntry、hermes_field_coverage 的 findHermesLedgerEntry。三者的差别
// 只在"要全部还是要第一条"与"找不到时的红字措辞",循环体与过滤条件完全一致。
// 收敛成 ledgerByMethod(全部)/ firstLedgerByMethod(第一条)两层后:
//   - 三家各自的函数名与签名保留(调用点不用改),
//   - 三家各自的 Fatalf 文案**逐字保留**(它们是给各自用例看的诊断语)。
package agent

import (
	"github.com/pocketagent/daemon/internal/runtime"
)

// ledgerByMethod 按 agentID + method 过滤台账条目(agentID 空 = 不按 agent 过滤,
// 与原三份拷贝一致),顺序即 List 给出的顺序。
func ledgerByMethod(agentID, method string) []runtime.EventEnvelope {
	var out []runtime.EventEnvelope
	for _, e := range sharedRuntimeBackend().UnknownEvents().List(runtime.ListFilter{AgentID: agentID}, 0) {
		if e.SourceMethod == method {
			out = append(out, e)
		}
	}
	return out
}

// firstLedgerByMethod 取第一条匹配;没有则 ok=false —— "没找到"的红字留在调用方。
func firstLedgerByMethod(agentID, method string) (runtime.EventEnvelope, bool) {
	if es := ledgerByMethod(agentID, method); len(es) > 0 {
		return es[0], true
	}
	return runtime.EventEnvelope{}, false
}
