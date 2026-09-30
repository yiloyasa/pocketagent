package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

type selectionFake struct {
	calls []string
	level string
	fail  string
}

func (f *selectionFake) CallCtx(_ context.Context, method string, _ map[string]any, _ time.Duration) (json.RawMessage, error) {
	f.calls = append(f.calls, method)
	if method == f.fail {
		return nil, errors.New("rejected")
	}
	if method == "get_available_thinking_levels" {
		return json.RawMessage(`{"levels":["low","high","max"]}`), nil
	}
	if method == "get_state" {
		return json.RawMessage(`{"model":{"id":"glm-5.3","provider":"zai"},"thinkingLevel":"` + f.level + `"}`), nil
	}
	return json.RawMessage(`{}`), nil
}

func TestPiSelectionRejectsClampedAndFailedSettings(t *testing.T) {
	for _, tc := range []struct{ effort, actual, fail string }{
		{"minimal", "high", ""},
		{"low", "high", ""},
		{"low", "low", "set_model"},
		{"low", "low", "set_thinking_level"},
	} {
		f := &selectionFake{level: tc.actual, fail: tc.fail}
		_, _, _, err := configurePiSelection(context.Background(), f, "pi/zai/glm-5.3", tc.effort)
		if err == nil {
			t.Fatalf("silently accepted %+v; calls=%v", tc, f.calls)
		}
		if tc.effort == "minimal" && strings.Contains(strings.Join(f.calls, ","), "set_thinking_level") {
			t.Fatal("must reject unsupported level before Pi clamps it")
		}
	}
}

func TestPiSelectionConfirmsRequestedLow(t *testing.T) {
	f := &selectionFake{level: "low"}
	model, effort, _, err := configurePiSelection(context.Background(), f, "pi/zai/glm-5.3", "low")
	if err != nil || model != "pi/zai/glm-5.3" || effort != "low" {
		t.Fatalf("model=%s effort=%s error=%v", model, effort, err)
	}
}
