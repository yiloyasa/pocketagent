package agent

import (
	"encoding/json"
	"testing"
)

func TestAgentMessageTextFromItemV2(t *testing.T) {
	raw := json.RawMessage(`{"type":"agentMessage","id":"m1","text":"最终答案"}`)
	if got := agentMessageTextFromItem(raw); got != "最终答案" {
		t.Fatalf("got %q", got)
	}
}

func TestAgentMessageTextFromItemLegacyContent(t *testing.T) {
	raw := json.RawMessage(`{"type":"agentMessage","id":"m1","content":[{"type":"text","text":"旧版答案"}]}`)
	if got := agentMessageTextFromItem(raw); got != "旧版答案" {
		t.Fatalf("got %q", got)
	}
}

func TestAgentMessageTextFromNotification(t *testing.T) {
	raw := json.RawMessage(`{"threadId":"t1","item":{"type":"agentMessage","id":"m1","text":"item终局"}}`)
	if got := agentMessageTextFromNotification(raw); got != "item终局" {
		t.Fatalf("got %q", got)
	}
}

func TestAgentMessageTextsFromTurn(t *testing.T) {
	raw := json.RawMessage(`{"threadId":"t1","turn":{"items":[{"type":"commandExecution","id":"c1"},{"type":"agentMessage","id":"m1","text":"第一段"},{"type":"agentMessage","id":"m2","text":"第二段"}]}}`)
	if got := agentMessageTextsFromTurn(raw); got != "第一段第二段" {
		t.Fatalf("got %q", got)
	}
}

func TestAgentMessageTextRejectsNonAgentItem(t *testing.T) {
	raw := json.RawMessage(`{"type":"plan","id":"p1","text":"不是回复"}`)
	if got := agentMessageTextFromItem(raw); got != "" {
		t.Fatalf("got %q", got)
	}
}
