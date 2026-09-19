package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
	"github.com/pocketagent/daemon/internal/zcodeapp"
)

// withControlClient never retries a control operation or replaces a shared
// worker on a native error. Replacing it could interrupt another session.
func (p *ZcodeProvider) withControlClient(ctx context.Context, fn func(context.Context, *zcodeapp.Client) error) error {
	if !p.lifeMu.TryLock() {
		return ErrBusy
	}
	defer p.lifeMu.Unlock()
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, zcodeStatusBudget)
	defer cancel()
	if p.client == nil {
		c, err := zcodeapp.Start(zcodeapp.StartOptions{})
		if err != nil {
			return err
		}
		p.client, p.inited = c, true
		p.gen.Add(1)
	}
	return fn(ctx, p.client)
}

func (p *ZcodeProvider) selectionSessionID(t *store.Thread) string {
	if t == nil {
		return ""
	}
	if sid := p.loadedZcodeSessionID(t.ID); sid != "" {
		return sid
	}
	if t.AgentID == "" || t.AgentID == "zcode" {
		return t.ExternalSession.SessionID
	}
	return ""
}

// CurrentSelection returns only the addressed native session's confirmed state.
// An unmaterialized thread has no native selection; its requested model/default
// and another session's settings must not be reported as confirmation.
func (p *ZcodeProvider) CurrentSelection(ctx context.Context, t *store.Thread) (model string, effort *string, err error) {
	sid := p.selectionSessionID(t)
	if sid == "" {
		return "", nil, nil
	}
	err = p.withControlClient(ctx, func(ctx context.Context, c *zcodeapp.Client) error {
		raw, e := c.CallCtx(ctx, "session/resume", map[string]string{"sessionId": sid})
		if e != nil {
			return e
		}
		// The addressed session contains the latest registry revision. Refresh
		// catalog metadata from that native snapshot, not from a model-name table.
		models, refs := parseZcodeModels(raw)
		if len(models) > 0 {
			p.mu.Lock()
			p.models = append([]pb.ModelInfo{{Id: "default", Provider: "zcode", DisplayName: "ZCode（原生当前模型）", Available: true}}, models...)
			p.refByModel = refs
			p.modelsAt = time.Now()
			p.mu.Unlock()
		}
		selection, e := zcodeConfirmedSelection(raw)
		if e == nil {
			model, effort = selection.ModelID, selection.Effort
		}
		return e
	})
	return
}

func zcodeConfirmedSelection(raw json.RawMessage) (RuntimeSelection, error) {
	var w struct {
		Snapshot json.RawMessage `json:"snapshot"`
		Settings struct {
			ThoughtLevel struct {
				Current *string `json:"current"`
			} `json:"thoughtLevel"`
			Model struct {
				Current *struct {
					ProviderID string `json:"providerId"`
					ModelID    string `json:"modelId"`
					Options    struct {
						ReasoningLevel *string `json:"reasoningLevel"`
					} `json:"options"`
				} `json:"current"`
			} `json:"model"`
		} `json:"settings"`
	}
	if err := json.Unmarshal(raw, &w); err != nil {
		return RuntimeSelection{}, err
	}
	if len(w.Snapshot) > 0 {
		return zcodeConfirmedSelection(w.Snapshot)
	}
	current := w.Settings.Model.Current
	if current == nil || current.ModelID == "" {
		return RuntimeSelection{}, nil
	}
	key := current.ModelID
	// Same identity rule as parseZcodeModels, including duplicate native IDs.
	snapshot, err := zcodeapp.ParseSessionSnapshot(raw)
	if err != nil {
		return RuntimeSelection{}, err
	}
	count := 0
	for _, model := range snapshot.Models {
		if model.ModelID == key {
			count++
		}
	}
	if count > 1 {
		if current.ProviderID == "" {
			return RuntimeSelection{}, errors.New("ZCode 当前模型缺少 providerId，无法区分重复型号")
		}
		key = zcodeapp.ModelKey(current.ProviderID, key)
	}
	effort := current.Options.ReasoningLevel
	if effort == nil {
		effort = w.Settings.ThoughtLevel.Current
	}
	return RuntimeSelection{ModelID: "zcode/" + key, Effort: effort}, nil
}

// ZcodeSessionCapabilities exposes native semantics, not Codex sandbox aliases.
// Modes are protocol-accepted values; SetSessionMode returns the effective mode,
// which can differ (independentPlanState currently maps plan back to build).
type ZcodeSessionCapabilities struct {
	CurrentMode          string
	Modes                []string
	Revision             *int64
	IndependentPlanState bool
}

func (p *ZcodeProvider) NativeModes(ctx context.Context) (out []string, err error) {
	err = p.withControlClient(ctx, func(ctx context.Context, c *zcodeapp.Client) error {
		var e error
		out, e = c.ModeSchema(ctx)
		return e
	})
	return
}

func parseZcodeModeSnapshot(raw json.RawMessage) (ZcodeSessionCapabilities, error) {
	var w struct {
		Runtime struct {
			StateRevision *int64 `json:"stateRevision"`
		} `json:"runtime"`
		Settings struct {
			Mode struct {
				Current string `json:"current"`
			} `json:"mode"`
		} `json:"settings"`
	}
	err := json.Unmarshal(raw, &w)
	return ZcodeSessionCapabilities{CurrentMode: w.Settings.Mode.Current, Revision: w.Runtime.StateRevision}, err
}

func (p *ZcodeProvider) nativeModeFor(t *store.Thread) string {
	// An existing snapshot (even empty) must not drift with later global defaults.
	if t != nil && t.SettingsSnapshot != nil {
		return t.SettingsSnapshot.NativeMode
	}
	p.prefsMu.Lock()
	defer p.prefsMu.Unlock()
	return p.prefs.NativeMode
}

func applyZcodeNativeMode(ctx context.Context, c *zcodeapp.Client, sid, mode string) (string, error) {
	if mode == "" {
		return "", nil
	}
	raw, err := c.CallCtx(ctx, "session/setMode", map[string]string{"sessionId": sid, "mode": mode})
	if err != nil {
		return "", err
	}
	caps, err := parseZcodeModeSnapshot(raw)
	if err == nil && caps.CurrentMode == "" {
		err = errors.New("ZCode 未确认实际 mode")
	}
	return caps.CurrentMode, err
}

func (p *ZcodeProvider) SessionCapabilities(ctx context.Context, t *store.Thread) (out ZcodeSessionCapabilities, err error) {
	sid := p.selectionSessionID(t)
	if sid == "" {
		return out, errors.New("ZCode 会话尚未建立")
	}
	err = p.withControlClient(ctx, func(ctx context.Context, c *zcodeapp.Client) error {
		raw, e := c.CallCtx(ctx, "session/resume", map[string]string{"sessionId": sid})
		if e != nil {
			return e
		}
		out, e = parseZcodeModeSnapshot(raw)
		if e != nil {
			return e
		}
		out.Modes, e = c.ModeSchema(ctx)
		if e != nil {
			return e
		}
		raw, e = c.CallCtx(ctx, "runtime/capabilities", map[string]any{})
		if e != nil {
			return e
		}
		var caps struct {
			IndependentPlanState bool `json:"independentPlanState"`
		}
		if e = json.Unmarshal(raw, &caps); e != nil {
			return e
		}
		out.IndependentPlanState = caps.IndependentPlanState
		return nil
	})
	return
}

func (p *ZcodeProvider) SetSessionMode(ctx context.Context, t *store.Thread, mode string) (out ZcodeSessionCapabilities, err error) {
	sid := p.selectionSessionID(t)
	if sid == "" {
		return out, errors.New("ZCode 会话尚未建立")
	}
	err = p.withControlClient(ctx, func(ctx context.Context, c *zcodeapp.Client) error {
		raw, e := c.CallCtx(ctx, "session/setMode", map[string]string{"sessionId": sid, "mode": mode})
		if e != nil {
			return e
		}
		out, e = parseZcodeModeSnapshot(raw)
		if e == nil && out.CurrentMode == "" {
			return fmt.Errorf("ZCode 未确认实际 mode")
		}
		return e
	})
	return
}

// DiscoverCommands creates no prompt and never reuses another workspace's list.
func (p *ZcodeProvider) DiscoverCommands(ctx context.Context, workspace string) (out []*pb.SlashCommand, err error) {
	if workspace == "" {
		return nil, errors.New("ZCode 命令发现需要工作区")
	}
	workspace, err = filepath.Abs(workspace)
	if err != nil {
		return nil, err
	}
	err = p.withControlClient(ctx, func(ctx context.Context, c *zcodeapp.Client) (err error) {
		raw, err := c.CallCtx(ctx, "session/create", map[string]any{
			"workspace": zcodeWorkspace(workspace), "mode": "build",
		})
		if err != nil {
			return err
		}
		snapshot, err := zcodeapp.ParseSessionSnapshot(raw)
		if err != nil {
			return err
		}
		if snapshot.SessionID == "" {
			return errors.New("ZCode 命令快照没有 sessionId")
		}
		defer func() {
			closeCtx, cancel := context.WithTimeout(context.Background(), zcodeInitBudget)
			defer cancel()
			err = errors.Join(err, c.CloseSession(closeCtx, snapshot.SessionID))
		}()
		out = zcodeSlashCommands(snapshot.SlashCommands)
		return nil
	})
	return
}

// state.updated.patch is settings, not a full session/event payload.
func zcodeStateUpdateSnapshot(raw json.RawMessage, sessionID string) json.RawMessage {
	var msg struct {
		SessionID string          `json:"sessionId"`
		Patch     json.RawMessage `json:"patch"`
	}
	if json.Unmarshal(raw, &msg) != nil || msg.SessionID != sessionID || len(msg.Patch) == 0 {
		return nil
	}
	snapshot, _ := json.Marshal(map[string]json.RawMessage{"settings": msg.Patch})
	return snapshot
}

func (p *ZcodeProvider) confirmedZcodeSelection(raw json.RawMessage) RuntimeSelection {
	out, err := zcodeConfirmedSelection(raw)
	if err != nil {
		return RuntimeSelection{}
	}
	var w struct {
		Settings struct {
			Model struct {
				Current struct {
					ProviderID string `json:"providerId"`
					ModelID    string `json:"modelId"`
				} `json:"current"`
			} `json:"model"`
		} `json:"settings"`
	}
	if json.Unmarshal(raw, &w) != nil {
		return out
	}
	current := w.Settings.Model.Current
	qualified := zcodeapp.ModelKey(current.ProviderID, current.ModelID)
	p.mu.Lock()
	_, ok := p.refByModel[qualified]
	p.mu.Unlock()
	if ok {
		out.ModelID = "zcode/" + qualified
	}
	return out
}
