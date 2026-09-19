package zcodeapp

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"
)

// GenerateTextSelected never substitutes another model after an explicit choice.
func (c *Client) GenerateTextSelected(ctx context.Context, model, effort, prompt string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, generateTextTimeout)
	defer cancel()
	raw, err := c.CallCtx(ctx, "session/create", map[string]any{"workspace": tempWorkspace(), "mode": "build"})
	if err != nil {
		return "", err
	}
	snap, err := ParseSessionSnapshot(raw)
	if err != nil {
		return "", err
	}
	defer func() {
		if snap.SessionID != "" {
			closeCtx, closeCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer closeCancel()
			_, _ = c.CallCtx(closeCtx, "session/close", map[string]string{"sessionId": snap.SessionID})
		}
	}()
	if model == "" || model == "default" {
		var current struct {
			Settings struct {
				Model struct {
					Current struct {
						ModelID string `json:"modelId"`
					} `json:"current"`
				} `json:"model"`
			} `json:"settings"`
		}
		if err := json.Unmarshal(raw, &current); err != nil {
			return "", err
		}
		model = current.Settings.Model.Current.ModelID
	}
	m, ok := snap.ModelByID(model)
	if !ok {
		return "", fmt.Errorf("ZCode 模型 %q 不在模型列表中", model)
	}
	if effort == "" {
		effort = m.DefaultReasoningLevel
	}
	if len(m.ReasoningLevels) > 0 && !slices.Contains(m.ReasoningLevels, effort) {
		return "", fmt.Errorf("ZCode 模型 %s 不支持思考档位 %s", model, effort)
	}
	selection := map[string]any{"providerId": m.ProviderID, "modelId": m.ModelID}
	if effort != "" {
		selection["options"] = map[string]string{"reasoningLevel": effort}
	}
	opID := fmt.Sprintf("pa-promptopt-%d", time.Now().UnixNano())
	result, err := c.attemptGenerate(ctx, selection, opID, withOutputDiscipline(prompt))
	if err != nil {
		cancelCtx, cancelCall := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelCall()
		_, _ = c.CallCtx(cancelCtx, "workspace/cancelGenerateText", map[string]string{"operationId": opID})
	}
	return result, err
}
