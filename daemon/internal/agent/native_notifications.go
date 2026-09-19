package agent

import (
	"encoding/json"
	"strings"
	"sync"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// RuntimeSelection is a partial update. Missing is not the same as explicitly
// choosing an empty/default effort; opaque IDs are never guessed from labels.
type RuntimeSelection struct {
	ModelID string
	Effort  *string
}

func decodeRuntimeSelection(agentID, method string, raw json.RawMessage) RuntimeSelection {
	var value map[string]json.RawMessage
	if json.Unmarshal(raw, &value) != nil {
		return RuntimeSelection{}
	}
	// Both standalone snapshots and wrapped notification snapshots are used by
	// supported native runtimes. Unknown siblings remain untouched.
	if inner := value["snapshot"]; len(inner) > 0 {
		return decodeRuntimeSelection(agentID, method, inner)
	}
	text := func(raw json.RawMessage) string {
		var s string
		_ = json.Unmarshal(raw, &s)
		return s
	}
	var result RuntimeSelection
	if agentID == "pi" {
		var model map[string]json.RawMessage
		_ = json.Unmarshal(value["model"], &model)
		if id, provider := text(model["id"]), text(model["provider"]); id != "" && provider != "" {
			result.ModelID = "pi/" + provider + "/" + id
		}
		level := value["thinkingLevel"]
		if method == "thinking_level_changed" {
			level = value["level"]
		}
		if len(level) > 0 {
			var s string
			if json.Unmarshal(level, &s) == nil {
				result.Effort = &s
			}
		}
	}
	if agentID == "zcode" {
		var settings struct {
			Model struct {
				Current struct {
					ModelID string `json:"modelId"`
					Options struct {
						ReasoningLevel *string `json:"reasoningLevel"`
					} `json:"options"`
				} `json:"current"`
			} `json:"model"`
		}
		if json.Unmarshal(value["settings"], &settings) == nil {
			if settings.Model.Current.ModelID != "" {
				result.ModelID = "zcode/" + settings.Model.Current.ModelID
			}
			result.Effort = settings.Model.Current.Options.ReasoningLevel
		}
	}
	return result
}

func emitRuntimeSelection(selection RuntimeSelection, sink Sink) {
	if selection.ModelID == "" && selection.Effort == nil {
		return
	}
	sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_Started{Started: &pb.TurnStarted{
		ModelId: selection.ModelID, Effort: selection.Effort, ConfigurationConfirmed: true,
	}}})
}

// Unknown notifications are not protocol failures. Preserve bounded, redacted
// evidence in the existing durable event stream without noisy warning UI.
type nativeNoticeRecorder struct {
	mu    sync.Mutex
	count map[string]int
}

func (r *nativeNoticeRecorder) record(agentID, method string, raw json.RawMessage, sink Sink) {
	r.mu.Lock()
	if r.count == nil {
		r.count = map[string]int{}
	}
	if r.count[method] >= 2 || len(r.count) >= 64 {
		r.mu.Unlock()
		return
	}
	r.count[method]++
	r.mu.Unlock()
	sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_CodexEvent{CodexEvent: &pb.CodexEvent{
		Kind: pb.CodexEvent_STATUS, Code: "native_notification", Title: agentID + " · " + method,
		Message: "原生状态通知", RawJson: redactNativePayload(raw),
	}}})
}

func redactNativePayload(raw json.RawMessage) string {
	if len(raw) > 32*1024 {
		return `{"diagnostic":"native payload exceeded 32KiB; omitted"}`
	}
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return `{"diagnostic":"invalid native JSON; omitted"}`
	}
	var redact func(any)
	redact = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, value := range x {
				key := strings.ToLower(strings.ReplaceAll(k, "_", ""))
				switch key {
				case "apikey", "authorization", "password", "secret", "accesstoken", "refreshtoken", "token", "cookie":
					x[k] = "[redacted]"
				default:
					redact(value)
				}
			}
		case []any:
			for _, item := range x {
				redact(item)
			}
		}
	}
	redact(v)
	out, _ := json.Marshal(v)
	if len(out) > 8000 {
		return `{"diagnostic":"redacted native payload exceeded 8KiB; omitted"}`
	}
	return string(out)
}
