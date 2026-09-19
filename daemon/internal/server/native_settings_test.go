package server

import (
	"context"
	"testing"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

type modeAdapter struct{ dynamicAdapter }

func (*modeAdapter) NativeModes(context.Context) ([]string, error) {
	return []string{"build", "plan", "future-mode"}, nil
}
func (*modeAdapter) SetSessionMode(_ context.Context, _ *store.Thread, mode string) (agent.ZcodeSessionCapabilities, error) {
	if mode == "plan" {
		mode = "build"
	}
	return agent.ZcodeSessionCapabilities{CurrentMode: mode}, nil
}
func (*modeAdapter) SessionCapabilities(context.Context, *store.Thread) (agent.ZcodeSessionCapabilities, error) {
	return agent.ZcodeSessionCapabilities{CurrentMode: "build", Modes: []string{"build", "plan", "future-mode"}}, nil
}

func TestNativeModeUsesConfirmedValueAndDoesNotContaminateAgentDefault(t *testing.T) {
	provider := &modeAdapter{dynamicAdapter: dynamicAdapter{plainProvider: plainProvider{id: "zcode"}}}
	st := mustStore(t)
	if err := st.Create(&store.Thread{ID: "native-mode", AgentID: "zcode"}); err != nil {
		t.Fatal(err)
	}
	s := New(Config{Store: st, Providers: map[string]agent.Provider{"zcode": provider}})
	mode := "plan"
	out, err := s.SetCodexSettings(context.Background(), &pb.SetCodexSettingsRequest{
		AgentId: "zcode", ThreadId: "native-mode", NativeMode: &mode,
	})
	if err != nil || out.NativeMode != "build" {
		t.Fatalf("requested mode was presented as actual: %v %v", out, err)
	}
	thread, _ := st.Get("native-mode")
	if thread.SettingsSnapshot.NativeMode != "build" || st.AgentDefaults("zcode").NativeMode != "" {
		t.Fatal("native mode snapshot/default isolation broken")
	}
}

func TestNativeModeAcceptsFutureSchemaValueNotStaticEnum(t *testing.T) {
	provider := &modeAdapter{dynamicAdapter: dynamicAdapter{plainProvider: plainProvider{id: "zcode"}}}
	st := mustStore(t)
	s := New(Config{Store: st, Providers: map[string]agent.Provider{"zcode": provider}})
	mode := "future-mode"
	_, err := s.SetCodexSettings(context.Background(), &pb.SetCodexSettingsRequest{AgentId: "zcode", NativeMode: &mode})
	if err != nil || st.AgentDefaults("zcode").NativeMode != mode {
		t.Fatalf("native future mode rejected: %v", err)
	}
}
