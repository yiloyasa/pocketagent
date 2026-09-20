package zcodeapp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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
		// 【T-02 容错,替代原"档位不在 levels 即报错"的硬校验】用户没显式选时,
		// 档位来自模型自带 defaultLevel。快照可能自相矛盾:deepseek-v4-flash 实测
		// default_level="max" 但自声明 levels=[low,medium,high,xhigh,none],上游
		// 也确实拒收 max —— 旧硬校验让手机端"提示词优化"在该模型上整次失败。
		// 与 internal/agent/zcode.go applyModel 同语义,不新增任何写死枚举:
		// defaultLevel ∉ 自声明 levels → 置空省略下方 selection 的 options(该
		// 分支本就支持无 options 的形状),沿用桌面端对当前模型的默认档位,记
		// 日志后照常;一致元数据(GLM 系,defaultLevel 在 levels 内)照旧显式下发。
		effort = m.DefaultReasoningLevel
		if len(m.ReasoningLevels) > 0 && !slices.Contains(m.ReasoningLevels, effort) {
			log.Printf("[zcode] 模型 %s 自带 defaultLevel %q 不在其声明档位 %v 内,省略 reasoningLevel(沿用桌面端当前档位)", model, effort, m.ReasoningLevels)
			effort = ""
		}
	}
	// 用户显式 effort 不进上面的分支:原样透传进 options,不做本地校验 ——
	// provider 是唯一裁决者,拒收时由 generateText 失败分支把错误报给用户。
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
