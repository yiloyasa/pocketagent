package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/pocketagent/daemon/internal/codexapp"
	"github.com/pocketagent/daemon/internal/piapp"
)

// Explicit opt-in: ordinary unit tests never spend provider credits or launch
// user-configured Agents. This probe does not write desktop configuration.
func TestNativePiModelSelection(t *testing.T) {
	if os.Getenv("PA_LIVE_ADAPTER_VERIFY") != "1" {
		t.Skip("set PA_LIVE_ADAPTER_VERIFY=1 for local native protocol verification")
	}
	p := NewPiProvider()
	models := p.Models()
	var selected string
	for i := range models {
		m := &models[i]
		t.Logf("Pi model=%s window=%d levels=%v", m.Id, m.ContextWindow, m.AvailableEfforts)
		if strings.HasSuffix(m.Id, "/glm-5.2") {
			selected = "pi/" + m.Id
		}
	}
	if selected == "" {
		t.Fatal("configured Pi does not report glm-5.2; probe will not substitute another model")
	}
	c, err := piapp.Start(piapp.StartOptions{Cwd: t.TempDir(),
		ExtraArgs: []string{"--no-session", "--no-tools", "--no-extensions", "--no-skills", "--no-prompt-templates"}})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	model, effort, _, err := configurePiSelection(ctx, c, selected, "high")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("NATIVE_CONFIRMED model=%s effort=%s", model, effort)
	if _, _, _, err := configurePiSelection(ctx, c, selected, "low"); err == nil {
		t.Fatal("GLM-5.2 low must be rejected")
	} else {
		t.Logf("UNSUPPORTED_REJECTED: %v", err)
	}
	next := strings.TrimSuffix(selected, "glm-5.2") + "glm-5.3"
	model, effort, _, err = configurePiSelection(ctx, c, next, "low")
	if err != nil || model != next || effort != "low" {
		t.Fatalf("glm-5.3 low: model=%s effort=%s err=%v", model, effort, err)
	}
	t.Logf("NATIVE_CONFIRMED model=%s effort=%s", model, effort)

}

func TestNativeSelectedTextGenerator(t *testing.T) {
	id := os.Getenv("PA_LIVE_GENERATE_AGENT")
	if id == "" {
		t.Skip("set PA_LIVE_GENERATE_AGENT and PA_LIVE_GENERATE_MODEL to verify one real generation")
	}
	model := os.Getenv("PA_LIVE_GENERATE_MODEL")
	if !strings.HasPrefix(model, id+"/") {
		t.Fatal("explicit Agent-prefixed model required")
	}
	var generator TextGenerator
	switch id {
	case "pi":
		generator = NewPiProvider()
	case "zcode":
		generator = NewZcodeProvider()
	case "codex":
		generator = NewCodexProvider()
	default:
		t.Fatal("unsupported validation Agent")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	text, err := generator.GenerateText(ctx, model, os.Getenv("PA_LIVE_GENERATE_EFFORT"),
		"这是纯文本连通验证，不得调用工具、读取或修改文件。请只回复 PA_NATIVE_OK。")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "PA_NATIVE_OK") {
		t.Fatalf("unexpected validation output: %.120s", text)
	}
	t.Logf("NATIVE_GENERATION_OK agent=%s model=%s reply=%q", id, model, text)
}

func TestNativeAgentCatalogs(t *testing.T) {
	if os.Getenv("PA_LIVE_ADAPTER_VERIFY") != "1" {
		t.Skip("native catalog opt-in")
	}
	codex := NewCodexProvider()
	zc := NewZcodeProvider()
	defer func() {
		if codex.client != nil {
			codex.client.Close()
		}
		if zc.client != nil {
			zc.client.Close()
		}
	}()
	for _, p := range []Provider{codex, zc} {
		models := p.Models()
		if len(models) <= 1 {
			t.Errorf("%s returned only a fallback; native catalog was not verified", p.ID())
		}
		for i := range models {
			m := &models[i]
			if strings.Contains(strings.ToLower(m.DisplayName), "glm-5.2") ||
				strings.Contains(strings.ToLower(m.DisplayName), "sol") {
				t.Logf("NATIVE_MODEL agent=%s id=%s name=%s levels=%v", p.ID(), m.Id, m.DisplayName, m.AvailableEfforts)
			}
		}
	}
}

func TestNativeCodexGoalProtocol(t *testing.T) {
	if os.Getenv("PA_LIVE_ADAPTER_VERIFY") != "1" {
		t.Skip("native goal protocol opt-in")
	}
	p := NewCodexProvider()
	defer func() {
		if p.client != nil {
			p.client.Close()
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	var id string
	err := p.withClientCtx(ctx, 35*time.Second, func(ctx context.Context, c *codexapp.Client) error {
		raw, err := c.CallCtx(ctx, "thread/start", map[string]any{"cwd": t.TempDir()})
		if err != nil {
			return err
		}
		var start struct {
			Thread struct {
				ID string `json:"id"`
			} `json:"thread"`
		}
		if err := json.Unmarshal(raw, &start); err != nil {
			return err
		}
		id = start.Thread.ID
		if id == "" {
			return fmt.Errorf("missing native thread ID")
		}
		defer func() {
			cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_, _ = c.CallCtx(cleanup, "thread/archive", map[string]string{"threadId": id})
			if _, err := c.CallCtx(cleanup, "thread/delete", map[string]string{"threadId": id}); err != nil {
				t.Logf("native probe cleanup: %v", err)
			}
		}()
		if err := pushGoalViaContext(ctx, c, id, "PA_NATIVE_GOAL_ROUNDTRIP"); err != nil {
			return err
		}
		got, err := codexGoalGetVia(ctx, c, id)
		if err != nil {
			return err
		}
		if got == nil || got.Objective != "PA_NATIVE_GOAL_ROUNDTRIP" {
			return fmt.Errorf("native goal did not round-trip: %+v", got)
		}
		return pushGoalViaContext(ctx, c, id, "")
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("NATIVE_GOAL_SET_READ_CLEAR_OK thread=%s disposable=true", id)
}
