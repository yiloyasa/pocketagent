package main

import (
	"reflect"
	"testing"
)

func TestAgentIDsFromCapabilities(t *testing.T) {
	got := agentIDsFromCapabilities([]string{
		"terminal",
		"agent:pi:v0.85.1",
		"agent:codex",
		"agent:pi",
		"collab:pi",
		"agent:",
		"agent:  ",
	})
	want := []string{"codex", "pi"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("agent ids = %#v, want %#v", got, want)
	}
}

func TestAgentIDsFromCapabilitiesIsStable(t *testing.T) {
	got := agentIDsFromCapabilities([]string{
		"agent:zcode",
		"agent:echo",
		"agent:codex:v1",
	})
	want := []string{"codex", "echo", "zcode"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("agent ids = %#v, want %#v", got, want)
	}
}
