//go:build pa_live

package agent

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

func TestZcodeNativeSelectionModesCommandsAndCodingPlan(t *testing.T) {
	if os.Getenv("PA_LIVE_ZCODE") != "1" {
		t.Skip("set PA_LIVE_ZCODE=1 for native verification")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	p := NewZcodeProvider()
	defer func() {
		if p.client != nil {
			p.client.Close()
		}
	}()
	modes, err := p.NativeModes(ctx)
	if err != nil || len(modes) == 0 {
		t.Fatalf("schema: %v modes=%v", err, modes)
	}
	workspace := t.TempDir()
	cmds, err := p.DiscoverCommands(ctx, workspace)
	if err != nil || len(cmds) == 0 {
		t.Fatalf("commands: %v count=%d", err, len(cmds))
	}
	t.Logf("native modes=%v commands=%d", modes, len(cmds))
	p.Models()
	modelID := ""
	p.mu.Lock()
	for id, ref := range p.refByModel {
		if strings.HasPrefix(ref.ProviderID, "account:") {
			modelID = "zcode/" + id
			break
		}
	}
	p.mu.Unlock()
	if modelID == "" {
		t.Fatal("native account model missing")
	}
	thread := &store.Thread{ID: "native-controls", AgentID: "zcode", ModelID: modelID}
	blocks, _, err := p.Run(ctx, TurnRequest{Thread: thread, Workspace: workspace, InputBlocks: zcodeTextBlocks("Reply exactly PONG. Do not use tools.")}, func(*pb.TurnServerMessage) {}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := p.CloseSession(thread.ExternalSession.SessionID); err != nil {
			t.Error(err)
		}
	}()
	if len(blocks) == 0 || !strings.Contains(blocks[0].GetText().GetText(), "PONG") {
		t.Fatal("native Coding Plan reply missing PONG")
	}
	actual, effort, err := p.CurrentSelection(ctx, thread)
	if err != nil || actual != modelID || effort == nil {
		t.Fatalf("current model=%s effort=%v err=%v", actual, effort, err)
	}
	t.Logf("Coding Plan native Run PONG model=%s effort=%s", actual, *effort)
	for _, mode := range modes {
		result, err := p.SetSessionMode(ctx, thread, mode)
		if err != nil {
			t.Fatalf("setMode %s: %v", mode, err)
		}
		read, err := p.SessionCapabilities(ctx, thread)
		if err != nil || read.CurrentMode != result.CurrentMode || read.Revision == nil {
			t.Fatalf("mode readback %+v err=%v", read, err)
		}
		t.Logf("requested=%s effective=%s revision=%d independentPlanState=%t", mode, read.CurrentMode, *read.Revision, read.IndependentPlanState)
	}
	generation := p.gen.Load()
	if err := p.CloseSession(thread.ExternalSession.SessionID); err != nil {
		t.Fatal(err)
	}
	if err := p.CloseSession(thread.ExternalSession.SessionID); err != nil {
		t.Fatal(err)
	}
	if p.gen.Load() != generation {
		t.Fatal("close restarted shared worker")
	}
	t.Log("native close twice succeeded without restart")
}
