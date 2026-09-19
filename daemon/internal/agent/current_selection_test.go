package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSelectionJournalReadsNativeOpaqueIDsNotAssistantClaims(t *testing.T) {
	for _, tc := range []struct{ agentID, raw, model, effort string }{
		{"pi", "{\"type\":\"model_change\",\"provider\":\"new-vendor\",\"modelId\":\"model/2027\"}\n" +
			"{\"type\":\"thinking_level_change\",\"thinkingLevel\":\"adaptive-v2\"}\n" +
			"{\"type\":\"message\",\"message\":{\"role\":\"assistant\",\"content\":\"I am some other model\"}}\n",
			"pi/new-vendor/model/2027", "adaptive-v2"},
		{"codex", "{\"type\":\"turn_context\",\"payload\":{\"model\":\"private/model/v2\",\"effort\":\"ultra-new\"}}\n" +
			"{\"type\":\"event_msg\",\"payload\":{\"type\":\"agent_message\",\"message\":\"model is fake\"}}\n",
			"codex/private/model/v2", "ultra-new"},
	} {
		t.Run(tc.agentID, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "native.jsonl")
			if err := os.WriteFile(path, []byte(tc.raw), 0600); err != nil {
				t.Fatal(err)
			}
			file, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			model, effort, err := readSelectionJournal(context.Background(), file, tc.agentID)
			if err != nil || model != tc.model || effort == nil || *effort != tc.effort {
				t.Fatalf("model=%q effort=%v err=%v", model, effort, err)
			}
		})
	}
}
