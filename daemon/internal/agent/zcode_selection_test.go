package agent

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
	"github.com/pocketagent/daemon/internal/zcodeapp"
)

func TestZcodeConfirmedSelectionOnlyNativeCurrent(t *testing.T) {
	tests := []struct {
		name, raw, model string
		effort           *string
	}{
		{"absent", `{"settings":{"model":{"lastUsed":{"modelId":"requested-only"}}}}`, "", nil},
		{"nil-current", `{"settings":{"model":{"current":null}}}`, "", nil},
		{"actual", `{"settings":{"model":{"current":{"providerId":"p","modelId":"new-model","options":{"reasoningLevel":"new-level"}}}}}`, "zcode/new-model", strPtrZcode("new-level")},
		{"explicit-empty", `{"settings":{"model":{"current":{"modelId":"m","options":{"reasoningLevel":""}}}}}`, "zcode/m", strPtrZcode("")},
		{"no-effort", `{"settings":{"model":{"current":{"modelId":"m"}},"thoughtLevel":{"defaultLevel":"not-current"}}}`, "zcode/m", nil},
		{"duplicate", `{"settings":{"model":{"current":{"providerId":"p2","modelId":"m"},"available":[{"ref":{"providerId":"p1","modelId":"m"}},{"ref":{"providerId":"p2","modelId":"m"}}]}}}`, "zcode/p2/m", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, e := zcodeConfirmedSelection(json.RawMessage(tt.raw))
			if e != nil || s.ModelID != tt.model {
				t.Fatalf("selection=%+v error=%v", s, e)
			}
			if (s.Effort == nil) != (tt.effort == nil) || s.Effort != nil && *s.Effort != *tt.effort {
				t.Fatalf("effort=%v want=%v", s.Effort, tt.effort)
			}
		})
	}
}
func strPtrZcode(s string) *string { return &s }

func TestZcodeCurrentSelectionReadsAddressedNativeSession(t *testing.T) {
	p, logPath := newFakeZcodeProvider(t, `{"session":{"sessionId":"native"},"settings":{"model":{"current":{"modelId":"actual","options":{"reasoningLevel":"high"}}}}}`)
	thread := &store.Thread{ID: "local", AgentID: "zcode", ModelID: "zcode/requested", Effort: "low", ExternalSession: store.ExternalSessionRef{SessionID: "native"}}
	model, effort, err := p.CurrentSelection(context.Background(), thread)
	if err != nil || model != "zcode/actual" || effort == nil || *effort != "high" {
		t.Fatalf("model=%s effort=%v err=%v", model, effort, err)
	}
	// No replacement conversation is created just to answer a settings read.
	for _, req := range zcodeFakeRequests(t, logPath) {
		if req.Method == "session/create" || req.Method == "session/setModel" {
			t.Fatalf("unexpected mutation %s", req.Method)
		}
	}
}

func TestZcodeCloseSessionMissingIsIdempotentAndNeverRestarts(t *testing.T) {
	for _, code := range []string{"-32004", "-32010", "-32602"} {
		t.Run(code, func(t *testing.T) {
			p, logPath := newFakeZcodeProvider(t, "")
			t.Setenv("PA_FAKE_ZCODE_CLOSE_ERROR", code)
			err := p.CloseSession("native")
			if code == "-32004" {
				if err != nil {
					t.Fatal(err)
				}
			} else {
				var rpc *zcodeapp.RPCError
				if !errors.As(err, &rpc) {
					t.Fatalf("native error lost: %v", err)
				}
			}
			if p.gen.Load() != 1 {
				t.Fatalf("client restarted: gen=%d", p.gen.Load())
			}
			calls := 0
			for _, req := range zcodeFakeRequests(t, logPath) {
				if req.Method == "session/close" {
					calls++
				}
			}
			if calls != 1 {
				t.Fatalf("close retried %d times", calls)
			}
		})
	}
}

func TestZcodeStateUpdatedFiltersSessionAndPreservesActualEffort(t *testing.T) {
	raw := json.RawMessage(`{"sessionId":"mine","patch":{"model":{"current":{"providerId":"p","modelId":"dynamic","options":{"reasoningLevel":"ultra-new"}}}}}`)
	if got := zcodeStateUpdateSnapshot(raw, "other"); got != nil {
		t.Fatal("cross-session notification")
	}
	p := NewZcodeProvider()
	got := p.confirmedZcodeSelection(zcodeStateUpdateSnapshot(raw, "mine"))
	if got.ModelID != "zcode/dynamic" || got.Effort == nil || *got.Effort != "ultra-new" {
		t.Fatalf("%+v", got)
	}
}

func TestZcodeModeUsesNativeEffectiveValueAndRuntimeRevision(t *testing.T) {
	caps, err := parseZcodeModeSnapshot(json.RawMessage(`{"runtime":{"stateRevision":7},"settings":{"mode":{"current":"build"}}}`))
	if err != nil || caps.CurrentMode != "build" || caps.Revision == nil || *caps.Revision != 7 {
		t.Fatalf("%+v %v", caps, err)
	}
}

func TestZcodeNativeModesCreatesNoSession(t *testing.T) {
	p, logPath := newFakeZcodeProvider(t, "")
	modes, err := p.NativeModes(context.Background())
	if err != nil || len(modes) != 5 {
		t.Fatalf("modes=%v err=%v", modes, err)
	}
	for _, req := range zcodeFakeRequests(t, logPath) {
		if req.Method != "session/setMode" {
			t.Fatalf("unexpected method %s", req.Method)
		}
	}
}

func TestZcodeDiscoverCommandsUsesRequestedWorkspaceAndCloses(t *testing.T) {
	p, logPath := newFakeZcodeProvider(t, `{"session":{"sessionId":"native"},"slashCommands":[{"name":"project-command","description":"from this workspace"}]}`)
	workspace := t.TempDir()
	cmds, err := p.DiscoverCommands(context.Background(), workspace)
	if err != nil || len(cmds) != 1 || cmds[0].Name != "project-command" {
		t.Fatalf("commands=%v err=%v", cmds, err)
	}
	closed, created := false, false
	for _, req := range zcodeFakeRequests(t, logPath) {
		switch req.Method {
		case "session/create":
			var params struct {
				Workspace struct {
					Path string `json:"workspacePath"`
				} `json:"workspace"`
			}
			_ = json.Unmarshal(req.Params, &params)
			if filepath.Clean(params.Workspace.Path) != filepath.Clean(workspace) {
				t.Fatalf("wrong workspace: %s", params.Workspace.Path)
			}
			created = true
		case "session/close":
			closed = true
		case "session/send", "workspace/generateText":
			t.Fatalf("unexpected prompt: %s", req.Method)
		}
	}
	if !created || !closed {
		t.Fatalf("created=%t closed=%t", created, closed)
	}
}

func TestZcodeRunForwardsNativeStateAndUnknownNotifications(t *testing.T) {
	p, _ := newFakeZcodeProvider(t, "")
	t.Setenv("PA_FAKE_ZCODE_STATE_UPDATE", "1")
	thread := &store.Thread{ID: "turn", ModelID: "zcode/default"}
	msgs := runZcodeImageTurn(t, p, zcodeImageTurnRequest(thread, t.TempDir()))
	confirmed, unknown := false, false
	for _, msg := range msgs {
		if s := msg.GetStarted(); s != nil && s.ConfigurationConfirmed {
			confirmed = s.ModelId == "zcode/native-switched" && s.Effort != nil && *s.Effort == "future-level"
		}
		if e := msg.GetCodexEvent(); e != nil && strings.Contains(e.Title, "future.unknown.notification") {
			unknown = true
		}
	}
	if !confirmed || !unknown {
		t.Fatalf("confirmation=%t unknown=%t", confirmed, unknown)
	}
}

func TestZcodeRunNativeModeIsExplicitAndSnapshotScoped(t *testing.T) {
	for _, tt := range []struct {
		name, global, snapshot string
		hasSnapshot            bool
		want                   string
	}{
		{"empty", "", "", false, ""},
		{"default", "edit", "", false, "edit"},
		{"existing-empty", "yolo", "", true, ""},
		{"snapshot-wins", "yolo", "plan", true, "plan"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p, logPath := newFakeZcodeProvider(t, "")
			p.ApplyPrefs(DesktopPrefs{NativeMode: tt.global})
			thread := &store.Thread{ID: "mode", ModelID: "zcode/default"}
			if tt.hasSnapshot {
				thread.SettingsSnapshot = &store.TurnSettings{NativeMode: tt.snapshot}
			}
			var messages []*pb.TurnServerMessage
			_, _, err := p.Run(context.Background(), TurnRequest{Thread: thread, Workspace: t.TempDir(), InputBlocks: zcodeTextBlocks("hello")}, func(m *pb.TurnServerMessage) { messages = append(messages, m) }, nil)
			if err != nil {
				t.Fatal(err)
			}
			var modes []string
			for _, req := range zcodeFakeRequests(t, logPath) {
				if req.Method == "session/setMode" {
					var r struct {
						Mode string `json:"mode"`
					}
					_ = json.Unmarshal(req.Params, &r)
					modes = append(modes, r.Mode)
				}
			}
			if tt.want == "" {
				if len(modes) != 0 {
					t.Fatalf("empty mode overwrote native: %v", modes)
				}
			} else if len(modes) != 1 || modes[0] != tt.want {
				t.Fatalf("modes=%v", modes)
			}
			if tt.want == "plan" {
				for _, msg := range messages {
					if e := msg.GetCodexEvent(); e != nil && e.Title == "zcode · mode" && e.Message != "原生权限模式: build" {
						t.Fatalf("claimed requested mode: %s", e.Message)
					}
				}
			}
		})
	}
}
