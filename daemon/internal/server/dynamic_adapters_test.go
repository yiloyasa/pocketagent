package server

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

type dynamicAdapter struct {
	plainProvider
	refreshes int
	generated int
	model     string
	failure   error
}

func (p *dynamicAdapter) Models() []pb.ModelInfo {
	return []pb.ModelInfo{{Id: "private/model-v99", Provider: p.id, Available: true,
		AvailableEfforts: []string{"adaptive", "future-level"}}}
}
func (p *dynamicAdapter) InvalidateModels() { p.refreshes++ }
func (p *dynamicAdapter) GenerateText(_ context.Context, model, effort, prompt string) (string, error) {
	p.generated++
	p.model = model
	if p.failure != nil {
		return "", p.failure
	}
	return "明确的任务指令", nil
}

func TestNewAdapterAndOpaqueModelsNeedNoServerWhitelist(t *testing.T) {
	p := &dynamicAdapter{plainProvider: plainProvider{id: "new-agent"}}
	s := New(Config{Store: mustStore(t), Providers: map[string]agent.Provider{"new-agent": p}})
	out, err := s.ListModels(context.Background(), &pb.ListModelsRequest{AgentId: "new-agent", Refresh: true})
	if err != nil || len(out.Models) != 1 || p.refreshes != 1 {
		t.Fatalf("catalog=%v refreshes=%d err=%v", out, p.refreshes, err)
	}
	m := out.Models[0]
	if m.Id != "new-agent/private/model-v99" || strings.Join(m.AvailableEfforts, ",") != "adaptive,future-level" {
		t.Fatalf("opaque fields were changed: %v", m)
	}
}

func TestOptimizerUsesSelectedAdapterWithoutFallback(t *testing.T) {
	p := &dynamicAdapter{plainProvider: plainProvider{id: "new-agent"}}
	other := &dynamicAdapter{plainProvider: plainProvider{id: "zcode"}}
	s := New(Config{Store: mustStore(t), Providers: map[string]agent.Provider{"new-agent": p, "zcode": other}})
	req := &pb.OptimizePromptRequest{Text: "整理任务", AgentId: "new-agent", ModelId: "new-agent/private/model-v99", Effort: "future-level"}
	out, err := s.OptimizePrompt(context.Background(), req)
	if err != nil || out.Optimized == "" || p.model != req.ModelId || other.generated != 0 {
		t.Fatalf("route failed: out=%v err=%v otherCalls=%d", out, err, other.generated)
	}
	p.failure = errors.New("native model service offline")
	if _, err := s.OptimizePrompt(context.Background(), req); err == nil || other.generated != 0 {
		t.Fatal("native failure silently fell back to another Agent")
	}
}

func TestAgentDefaultsSurviveRestartAndRemainIsolated(t *testing.T) {
	dir := t.TempDir()
	st, _ := store.NewFileStore(dir)
	_ = st.SetDefaultModelID("codex/private/native-model")
	_ = st.SetDefaultEffort("high")
	pi := &dynamicAdapter{plainProvider: plainProvider{id: "pi"}}
	s := New(Config{Store: st, Providers: map[string]agent.Provider{"pi": pi}})
	model, effort := "pi/private/model-v99", "adaptive"
	if _, err := s.SetCodexSettings(context.Background(), &pb.SetCodexSettingsRequest{
		AgentId: "pi", Model: &model, Effort: &effort,
	}); err != nil {
		t.Fatal(err)
	}
	reopened, _ := store.NewFileStore(dir)
	if reopened.DefaultModelID() != "codex/private/native-model" || reopened.DefaultEffort() != "high" {
		t.Fatal("Pi settings contaminated Codex")
	}
	d := reopened.AgentDefaults("pi")
	if d.Model != model || d.Effort != effort || reopened.AgentDefaults("zcode").Model != "" {
		t.Fatalf("agent defaults not isolated/persistent: %+v", d)
	}
}

func TestLocalOnlyRemovalPreservesNativeSessionReference(t *testing.T) {
	s := newAgentSessionsServer(t)
	th := &store.Thread{ID: "remove-only", AgentID: "pi",
		ExternalSession: store.ExternalSessionRef{SessionID: "original-session", SessionFile: "not-a-real-file"}}
	if err := s.cfg.Store.Create(th); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeleteThread(context.Background(), &pb.DeleteThreadRequest{ThreadId: th.ID, LocalOnly: true}); err != nil {
		t.Fatal(err)
	}
	if !s.cfg.Store.SessionHidden("pi", "original-session") {
		t.Fatal("removed native session would immediately reappear")
	}
	if _, ok := s.cfg.Store.Get(th.ID); ok {
		t.Fatal("local thread not removed")
	}
}
