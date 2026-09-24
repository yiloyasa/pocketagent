package agent

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/pocketagent/daemon/internal/runtime"
)

func TestRecordUnknownResponsePreservesCorrelationAndRedactsPayload(t *testing.T) {
	raw := json.RawMessage(`{"id":17,"result":{"token":"secret-value","answer":"kept"}}`)
	recordUnknownResponse("codex", "17", raw)

	entries := sharedRuntimeBackend().UnknownEvents().List(
		runtime.ListFilter{AgentID: "codex", Kind: runtime.KindUnknownResponse},
		0,
	)
	var found *runtime.EventEnvelope
	for i := range entries {
		if entries[i].CorrelationID == "17" {
			found = &entries[i]
			break
		}
	}
	if found == nil {
		t.Fatal("未关联到 pending 请求的 response 必须进入 UNKNOWN_RESPONSE 台账")
	}
	if found.RawPayload == "" || found.RawPayload == string(raw) {
		t.Fatalf("response 载荷必须经过有界脱敏，得到 %q", found.RawPayload)
	}
	if !strings.Contains(found.RawPayload, "[redacted]") {
		t.Fatalf("response 中的 token 必须脱敏，得到 %q", found.RawPayload)
	}
}
