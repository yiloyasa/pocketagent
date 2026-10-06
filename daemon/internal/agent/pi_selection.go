package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

type piSelectionCaller interface {
	CallCtx(context.Context, string, map[string]any, time.Duration) (json.RawMessage, error)
}

// Pi acknowledges unsupported thinking levels after clamping them to another
// level. Validate the selected model's levels and read back state before sending
// any prompt; success:false and a mismatching readback must never run silently.
func configurePiSelection(ctx context.Context, c piSelectionCaller, model, effort string) (actual, actualEffort, warn string, err error) {
	spec := strings.TrimPrefix(model, "pi/")
	if model != "" && spec == model {
		return "", "", "", fmt.Errorf("模型不属于 Pi: %s", model)
	}
	if spec != "" && spec != "default" {
		provider, id, ok := strings.Cut(spec, "/")
		if !ok || provider == "" || id == "" {
			return "", "", "", fmt.Errorf("无效 Pi 模型: %s", model)
		}
		if _, err := c.CallCtx(ctx, "set_model", map[string]any{"provider": provider, "modelId": id}, piCallBudget); err != nil {
			// 【降级+显示(2026-10-01)】pi update 更新模型目录后,会话遗留的旧模型
			// id 会被 pi 以 "Model not found" 拒绝 —— 只影响"用哪个模型",不该杀死
			// 整轮任务。回退 Pi 默认模型、回读**实际生效模型**并经 warn 上抛,
			// 由调用方发警告卡+Started 帧携带真实模型(手机模型行随之显示真值)。
			if strings.Contains(err.Error(), "Model not found") {
				log.Printf("[pi] 模型 %s 已不在 Pi 目录,沿用 Pi 当前默认模型继续本轮", model)
				warn = "会话原模型 " + model + " 已从 Pi 目录移除,已回退 Pi 默认模型;请在模型行重新选择。"
				// 【1.0.2 对齐(2026-10-07)】「回退默认」= 不动 pi 侧,跳过 set_model,
				// 直接 get_state 回读真值。bundle 实证 1.0.2 的 handler 只按
				// find(m => m.provider === command.provider && m.id === command.modelId)
				// 精确匹配,没有「跟随默认」分支 —— 旧候选 {"modelId":"default"} 与
				// {"provider":"","modelId":""} 双双必败(目录里没有 id 为 "default"
				// 或 provider/modelId 为空串的条目),每次降级都白打两轮必败 RPC。
				// 沿用当前模型即默认语义,跨版本一致:0.99.1 时代 daemon 生产路径
				// 本就只发 provider+id 二元组、default/无斜杠规格一律本地拒绝
				// (pi_selection 首版先例),不发这两种形状对它零差异。
				if raw, e3 := c.CallCtx(ctx, "get_state", nil, piCallBudget); e3 == nil {
					var st struct {
						Model struct {
							ID       string `json:"id"`
							Provider string `json:"provider"`
						} `json:"model"`
						ThinkingLevel string `json:"thinkingLevel"`
					}
					if json.Unmarshal(raw, &st) == nil && st.Model.ID != "" {
						return "pi/" + st.Model.Provider + "/" + st.Model.ID, st.ThinkingLevel, warn, nil
					}
				}
				return "", "", warn, nil
			}
			return "", "", "", fmt.Errorf("Pi 模型切换失败: %w", err)
		}
	}
	if effort != "" {
		raw, err := c.CallCtx(ctx, "get_available_thinking_levels", nil, piCallBudget)
		var levels struct {
			Levels []string `json:"levels"`
		}
		// Missing/new-shaped metadata means unknown, not unsupported. Let the
		// native setter decide, then verify its actual state. A known list remains
		// authoritative (important for Pi's automatic level clamping).
		known := err == nil && json.Unmarshal(raw, &levels) == nil && levels.Levels != nil
		if known && !slices.Contains(levels.Levels, effort) {
			return "", "", "", fmt.Errorf("当前 Pi 模型不支持思考强度 %q，可用档位: %s", effort, strings.Join(levels.Levels, ", "))
		}
		if !known {
			log.Printf("[pi] 思考能力资料暂不可用，交由原生设置与回读确认: %v", err)
		}
		if _, err := c.CallCtx(ctx, "set_thinking_level", map[string]any{"level": effort}, piCallBudget); err != nil {
			return "", "", "", fmt.Errorf("Pi 思考强度切换失败: %w", err)
		}
	}
	raw, err := c.CallCtx(ctx, "get_state", nil, piCallBudget)
	if err != nil {
		return "", "", "", fmt.Errorf("读取 Pi 生效配置失败: %w", err)
	}
	var state struct {
		Model struct {
			ID       string `json:"id"`
			Provider string `json:"provider"`
		} `json:"model"`
		ThinkingLevel string `json:"thinkingLevel"`
	}
	if err := json.Unmarshal(raw, &state); err != nil || state.Model.ID == "" || state.Model.Provider == "" {
		// The native setter succeeded but this version's optional telemetry has
		// a new shape. Do not invent an actual model, nor prevent an otherwise
		// accepted task from running. Retain bounded evidence for the adapter.
		log.Printf("[pi] 设置已受理，运行态格式暂无法识别: %s", redactNativePayload(raw))
		return "", "", "", nil
	}
	actual = "pi/" + state.Model.Provider + "/" + state.Model.ID
	if spec != "" && spec != "default" && actual != model {
		return "", "", "", fmt.Errorf("Pi 模型未生效: 请求 %s，实际 %s", model, actual)
	}
	if effort != "" && effort != state.ThinkingLevel {
		return "", "", "", fmt.Errorf("Pi 思考强度未生效: 请求 %s，实际 %s", effort, state.ThinkingLevel)
	}
	return actual, state.ThinkingLevel, "", nil
}

func (p *PiProvider) configureSelection(ctx context.Context, c piSelectionCaller, t *store.Thread, sink Sink) error {
	model, effort := p.desktopDefaults()
	if t.SettingsSnapshot != nil {
		model, effort = t.SettingsSnapshot.Model, t.SettingsSnapshot.Effort
		// Migrate old snapshots which accidentally captured the Codex defaults.
		if model != "" && !strings.HasPrefix(model, "pi/") {
			model, effort = "", ""
		}
	}
	if t.ModelID != "" && t.ModelID != "pi/default" {
		model = t.ModelID
	}
	if t.Effort != "" {
		effort = t.Effort
	}
	actual, actualEffort, warn, err := configurePiSelection(ctx, c, model, effort)
	if err != nil {
		return &piError{code: "PI_SETTINGS_REJECTED", msg: err.Error()}
	}
	if warn != "" {
		// 降级可见化(用户令"要显示模型不只是回退"):先发警告卡说明原模型失效
		// 与回退结果,下面的 Started 帧携带回读到的**真实生效模型** —— 手机
		// 会话/模型行随之显示真值,不再是 unknown/旧 id。
		sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_CodexEvent{CodexEvent: &pb.CodexEvent{
			Kind: pb.CodexEvent_STATUS, Code: "model_fallback",
			Title: "Pi 模型已回退", Message: warn,
		}}})
	}
	if actual == "" {
		sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_CodexEvent{CodexEvent: &pb.CodexEvent{
			Kind: pb.CodexEvent_STATUS, Code: "native_settings_unconfirmed",
			Title: "Pi 设置已受理", Message: "Agent 未报告可解码的生效状态，未将请求值冒充实际值。",
		}}})
		return nil
	}
	sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_Started{Started: &pb.TurnStarted{
		ModelId: actual, Effort: &actualEffort, ConfigurationConfirmed: true,
	}}})
	return nil
}
