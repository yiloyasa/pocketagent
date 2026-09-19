package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNativeTerminalErrorWithoutAssistantIsRecovered(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	raw := `{"type":"event_msg","timestamp":"2026-09-19T08:55:24Z","payload":{"type":"task_started","turn_id":"native-turn"}}
{"type":"event_msg","timestamp":"2026-09-19T09:01:32Z","payload":{"type":"task_complete","turn_id":"native-turn","last_agent_message":null,"error":{"message":"upstream unavailable","code":"upstream"}}}
`
	if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := readCodexNativeTurn(context.Background(), path, 0)
	if err != nil || got == nil || got.Error == nil || got.Error.Message != "upstream unavailable" || got.Text != "" {
		t.Fatalf("native failure lost: %+v err=%v", got, err)
	}
	raw += `{"type":"event_msg","timestamp":"2026-09-19T09:02:00Z","payload":{"type":"task_started","turn_id":"later"}}` + "\n"
	_ = os.WriteFile(path, []byte(raw), 0600)
	if got, err := readCodexNativeTurn(context.Background(), path, 0); err != nil || got != nil {
		t.Fatalf("old terminal must not end a newer turn: %+v %v", got, err)
	}
}
