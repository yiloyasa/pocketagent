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
	sharedRuntimeBackend().UnknownEvents().Record(runtime.EventEnvelope{
		CorrelationID:    correlationID,
		SourceAgent:      agentID,
		SourceMethod:     "response",
		NormalizedKind:   runtime.KindUnknownResponse,
		SafetyClass:      runtime.SafetyClassUnknown,
		RawPayload:       redactNativePayload(raw),
		ProcessingStatus: runtime.StatusNew,
	})
}
