package agent

import "testing"

func TestIsReasoningDeltaMethodAcceptsCodexCamelCaseNotifications(t *testing.T) {
	tests := []struct {
		method string
		want   bool
	}{
		{"item/reasoning/summaryTextDelta", true},
		{"item/reasoning/textDelta", true},
		{"item/reasoning/delta", true},
		{"item/agentMessage/delta", false},
		{"item/reasoning/started", false},
		{"item/commandExecution/outputDelta", false},
	}
	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			if got := isReasoningDeltaMethod(tt.method); got != tt.want {
				t.Fatalf("isReasoningDeltaMethod(%q) = %v, want %v", tt.method, got, tt.want)
			}
		})
	}
}
