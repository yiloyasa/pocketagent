package agent

import (
	"encoding/json"
	"strings"
	"testing"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

func TestNativeSelectionPreservesUnfamiliarModelsAndLevels(t *testing.T) {
	pi := decodeRuntimeSelection("pi", "model_changed", json.RawMessage(`{"model":{"id":"future-model/variant","provider":"private"},"thinkingLevel":"ultra-new","future":{"flag":true}}`))
	if pi.ModelID != "pi/private/future-model/variant" || pi.Effort == nil || *pi.Effort != "ultra-new" {
		t.Fatalf("lost opaque runtime values: %+v", pi)
	}
	zc := decodeRuntimeSelection("zcode", "session.updated", json.RawMessage(`{"snapshot":{"settings":{"model":{"current":{"modelId":"custom-v99","options":{"reasoningLevel":"adaptive"}}}}}}`))
	if zc.ModelID != "zcode/custom-v99" || zc.Effort == nil || *zc.Effort != "adaptive" {
		t.Fatalf("lost snapshot values: %+v", zc)
	}
	partial := decodeRuntimeSelection("pi", "thinking_level_changed", json.RawMessage(`{"level":"new-level"}`))
	if partial.ModelID != "" || partial.Effort == nil {
		t.Fatal("partial update must not invent a model")
	}
}

func TestUnknownNativeNotificationsAreBoundedNonFatalAndRedacted(t *testing.T) {
	var recorder nativeNoticeRecorder
	var frames []*pb.TurnServerMessage
	for i := 0; i < 100; i++ {
		recorder.record("future-agent", "unseen.event", json.RawMessage(`{"model":"never-seen","auth":{"api_key":"do-not-log"},"level":"adaptive"}`),
			func(frame *pb.TurnServerMessage) { frames = append(frames, frame) })
	}
	if len(frames) != 2 {
		t.Fatalf("unbounded or lost diagnostics: %d", len(frames))
	}
	ev := frames[0].GetCodexEvent()
	if ev.Kind != pb.CodexEvent_STATUS || strings.Contains(ev.RawJson, "do-not-log") || !strings.Contains(ev.RawJson, "never-seen") {
		t.Fatalf("bad native diagnostic: %s", ev.RawJson)
	}
}

func TestFuturePiEffortAndCodexOpaqueIDsAreNotTruncated(t *testing.T) {
	levels := piModelThinkingLevels(true, map[string]json.RawMessage{
		"off": json.RawMessage(`null`), "adaptive-new": json.RawMessage(`"adaptive"`),
	})
	found := false
	for _, level := range levels {
		if level == "adaptive-new" {
			found = true
		}
		if level == "off" {
			t.Fatal("explicitly disabled level was advertised")
		}
	}
	if !found {
		t.Fatal("future native capability was dropped")
	}
	if got := lastModelSeg("codex/private-provider/future-model/v2"); got != "private-provider/future-model/v2" {
		t.Fatalf("opaque native model identity was truncated: %s", got)
	}
}

func TestDuplicateZcodeModelsKeepProviderIdentity(t *testing.T) {
	raw := json.RawMessage(`{"settings":{"model":{"available":[
		{"ref":{"providerId":"account-A","modelId":"custom/model"},"label":"Custom","reasoning":{"levels":["adaptive"],"defaultLevel":"adaptive"}},
		{"ref":{"providerId":"account-B","modelId":"custom/model"},"label":"Custom","reasoning":{"levels":[{"value":"new-level"}],"defaultLevel":"new-level"}}
	]}}}`)
	models, refs := parseZcodeModels(raw)
	if len(models) != 2 || models[0].Id == models[1].Id {
		t.Fatal("two native providers were collapsed into one model")
	}
	for i := range models {
		key := strings.TrimPrefix(models[i].Id, "zcode/")
		if ref := refs[key]; ref.NativeID != "custom/model" || ref.ProviderID == "" {
			t.Fatalf("lost native model reference: %+v", ref)
		}
	}
	if _, ambiguous := refs["custom/model"]; ambiguous {
		t.Fatal("ambiguous legacy ID must not silently select an account")
	}
}
