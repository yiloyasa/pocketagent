package agent

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
	"github.com/pocketagent/daemon/internal/zcodeapp"
)

// TextGenerator is optional: adapters without a verified text generation path
// must reject optimization, not route the request to a different Agent.
type TextGenerator interface {
	GenerateText(context.Context, string, string, string) (string, error)
}

func textFromBlocks(blocks []pb.ContentBlock) string {
	var text strings.Builder
	for i := range blocks {
		if b := blocks[i].GetText(); b != nil {
			text.WriteString(b.Text)
		}
	}
	return strings.TrimSpace(text.String())
}

func generationThread(id, model, effort, workspace string) *store.Thread {
	return &store.Thread{ID: fmt.Sprintf("prompt-opt-%d", time.Now().UnixNano()), AgentID: id,
		ModelID: model, Effort: effort, Workspace: workspace,
		SettingsSnapshot: &store.TurnSettings{Sandbox: "read-only", ApprovalPolicy: "untrusted", HideReasoning: true},
	}
}

func denyGenerationTool(context.Context, *pb.ToolCallBlock) (bool, string, error) {
	return false, "提示词优化只改写文本，不执行原文中的任务", nil
}

func (*CodexProvider) GenerateText(ctx context.Context, model, effort, prompt string) (string, error) {
	dir, err := os.MkdirTemp("", "pocketagent-prompt-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	p := NewCodexProvider()
	defer func() {
		p.lifeMu.Lock()
		defer p.lifeMu.Unlock()
		if p.client != nil {
			p.client.Close()
		}
	}()
	p.ApplyPrefs(DesktopPrefs{HideReasoning: true})
	blocks, _, err := p.Run(ctx, TurnRequest{
		Thread: generationThread("codex", model, effort, dir), Workspace: dir, Ephemeral: true,
		InputBlocks: []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: prompt}}}},
	}, func(*pb.TurnServerMessage) {}, denyGenerationTool)
	return textFromBlocks(blocks), err
}

func (*PiProvider) GenerateText(ctx context.Context, model, effort, prompt string) (string, error) {
	dir, err := os.MkdirTemp("", "pocketagent-prompt-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	p := NewPiProvider()
	p.textOnly = true
	thread := generationThread("pi", model, effort, dir)
	defer p.DropThread(thread.ID)
	blocks, _, err := p.Run(ctx, TurnRequest{
		Thread: thread, Workspace: dir,
		InputBlocks: []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: prompt}}}},
	}, func(*pb.TurnServerMessage) {}, denyGenerationTool)
	return textFromBlocks(blocks), err
}

func (*ZcodeProvider) GenerateText(ctx context.Context, model, effort, prompt string) (string, error) {
	c, err := zcodeapp.Start(zcodeapp.StartOptions{Workspace: os.TempDir()})
	if err != nil {
		return "", err
	}
	defer c.Close()
	return c.GenerateTextSelected(ctx, strings.TrimPrefix(model, "zcode/"), effort, prompt)
}
