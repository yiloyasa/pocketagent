package agent

import (
	"encoding/json"

	"github.com/pocketagent/daemon/internal/runtime"
)

// recordUnknownResponse is the common protocol-boundary sink for a response
// that cannot be correlated with an outstanding request. It deliberately does
// not manufacture a business result: the caller continues its existing
// pending-call/error behavior while the durable runtime ledger keeps evidence.
func recordUnknownResponse(agentID, correlationID string, raw json.RawMessage) {
	recordUnknownResponseAs(runtime.KindUnknownResponse, agentID, "response", correlationID, raw)
}

// recordUnknownResponseAs 与 recordUnknownResponse 同一条路,额外接受分档结果与
// 登记方法名(规格 §4.6 的 normalized_kind;dsh/zcode 后缀登记的同款先例)。
//
// 【为什么留这个口子】"无法关联到在途请求"不等于"完全没见过":codex 能力探测
// 轮询超时放弃后,app-server 迟到补发的错误应答就是可实证识别的已知噪音
// (见 codex.go 的 codexLateResponseLedger)。认得出的按已知留痕,认不出的仍走
// UNKNOWN_RESPONSE —— 分类的意义是把这两类分开,而不是把台账清零。
func recordUnknownResponseAs(kind, agentID, method, correlationID string, raw json.RawMessage) {
	if kind == "" {
		kind = runtime.KindUnknownResponse
	}
	if method == "" {
		method = "response"
	}
	sharedRuntimeBackend().UnknownEvents().Record(runtime.EventEnvelope{
		CorrelationID:    correlationID,
		SourceAgent:      agentID,
		SourceMethod:     method,
		NormalizedKind:   kind,
		SafetyClass:      runtime.SafetyClassUnknown,
		RawPayload:       redactNativePayload(raw),
		ProcessingStatus: runtime.StatusNew,
	})
}
