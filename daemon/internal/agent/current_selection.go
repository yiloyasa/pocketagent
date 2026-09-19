package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/pocketagent/daemon/internal/store"
)

// CurrentSelectionReader reports only observed native state, never an app-side
// default or the user's requested value. A nil effort means it was not reported.
type CurrentSelectionReader interface {
	CurrentSelection(context.Context, *store.Thread) (string, *string, error)
}

func (p *PiProvider) CurrentSelection(ctx context.Context, t *store.Thread) (string, *string, error) {
	if c := p.aliveClient(t.ID); c != nil {
		raw, err := c.CallCtx(ctx, "get_state", nil, piCallBudget)
		if err != nil {
			return "", nil, err
		}
		selection := decodeRuntimeSelection("pi", "get_state", raw)
		return selection.ModelID, selection.Effort, nil
	}
	// Desktop Pi may be a different process. Its native session journal still
	// records model_change/thinking_level_change with provider IDs intact.
	if t.ExternalSession.SessionFile == "" {
		return "", nil, nil
	}
	file, err := os.Open(t.ExternalSession.SessionFile)
	if err != nil {
		return "", nil, err
	}
	defer file.Close()
	model, effort, err := readSelectionJournal(ctx, file, "pi")
	return model, effort, err
}

func (p *CodexProvider) CurrentSelection(ctx context.Context, t *store.Thread) (string, *string, error) {
	if t.CodexThreadID == "" {
		return "", nil, nil
	}
	path, err := FindCodexSession(t.CodexThreadID)
	if err != nil {
		return "", nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return "", nil, err
	}
	defer file.Close()
	return readSelectionJournal(ctx, file, "codex")
}

func readSelectionJournal(ctx context.Context, file *os.File, agentID string) (string, *string, error) {
	// Cap each foreground poll's work. Never use a stale guessed value when the
	// latest state is outside this retained tail.
	if stat, err := file.Stat(); err == nil && stat.Size() > 16<<20 {
		if _, err := file.Seek(-(16 << 20), io.SeekEnd); err != nil {
			return "", nil, err
		}
	}
	scan := bufio.NewScanner(file)
	scan.Buffer(make([]byte, 64*1024), 8<<20)
	var model string
	var effort *string
	for scan.Scan() {
		if err := ctx.Err(); err != nil {
			return "", nil, err
		}
		var entry struct {
			Type          string  `json:"type"`
			Provider      string  `json:"provider"`
			ModelID       string  `json:"modelId"`
			ThinkingLevel *string `json:"thinkingLevel"`
			Payload       struct {
				Model           string  `json:"model"`
				Effort          *string `json:"effort"`
				ReasoningEffort *string `json:"reasoning_effort"`
			} `json:"payload"`
		}
		if json.Unmarshal(scan.Bytes(), &entry) != nil {
			continue
		}
		switch {
		case agentID == "pi" && entry.Type == "model_change":
			if entry.Provider != "" && entry.ModelID != "" {
				model = "pi/" + entry.Provider + "/" + entry.ModelID
			}
		case agentID == "pi" && entry.Type == "thinking_level_change":
			effort = entry.ThinkingLevel
		case agentID == "codex" && entry.Type == "turn_context":
			if entry.Payload.Model != "" {
				model = "codex/" + strings.TrimPrefix(entry.Payload.Model, "codex/")
			}
			effort = entry.Payload.Effort
			if effort == nil {
				effort = entry.Payload.ReasoningEffort
			}
		}
	}
	if err := scan.Err(); err != nil && !errors.Is(err, io.EOF) {
		return "", nil, err
	}
	return model, effort, nil
}
