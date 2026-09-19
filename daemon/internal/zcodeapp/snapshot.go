package zcodeapp

// snapshot.go —— session/create / session/resume 应答快照的解析层(只读、纯函数)。
//
// 【为什么放独立文件】ZcodeProvider(agent/zcode.go)与 client.go/generate.go 归
// 审批接入智能体所有;本文件同包不同文件、只增不改 —— 谁的地盘谁施工。
//
// 【方法面来源】覆盖审计(docs/review/2026-09-19-provider-coverage-audit.md §7
// S-1"zcode 快照三连")指出三份现成数据躺在快照里没人消费:
//   1. slashCommands[]        —— 手机命令面板的数据源(现在导入命令面板即可用);
//   2. projection.totalTokenCount / contextWindow —— 上下文占用环;
//   3. settings.model.available[].properties.inputFormat.supportsImage —— 模型图片能力位。
// 本文件把快照一次性解析成结构体并给出 accessor + 单测;provider/server 层的
// 消费接线要改 zcode.go / server 文件(当前被并行智能体占用),等文件释放后再接,
// 本文件不反向依赖任何调用方。
//
// 【形状出处】全部来自实机探针证据 daemon/docs/verify/evidence/zcode-p0-probe.md
// §4.2 与 zcode-p0-probe-raw.log(2026-09-18 实测 0.16.5),不是猜的:
//   - projection:{"contextWindow":200000,"mode":"build","sessionId":"unknown","status":"idle","totalTokenCount":0,…}
//   - slashCommands 元素:{"description":"…","inputHint":"/goal …","name":"goal"}(raw.log 实测)
//   - inputFormat:{"supportsText":true,"supportsImage":true,"supportsVideo":…,"supportsAudio":false,"supportsPdf":false}
//   - thoughtLevel:{"available":[{"label":"disabled","value":"disabled"},…],"current":"enabled","enabled":true}
// 未实测到的可选字段零值放过:单个字段缺失不让整个快照解析失败(与
// agent/zcode_sessions.go 的 parseZcodeSessionList 同一条纪律)。

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// SessionSnapshot 是 session/create(以及 session/resume)应答快照中
// "手机端用得上的"字段的展平视图。只暴露已知形状的字段,其余留在原始 JSON 里。
type SessionSnapshot struct {
	SessionID string // session.sessionId("sess_<uuid>")
	Title     string // session.title(创建时通常为空,由 titleUpdated 补)

	Projection SessionProjection // projection 段:上下文占用的地面真值

	Models []SessionModel // settings.model.available[](展平 ref/properties)

	ThoughtLevels       []ThoughtLevel // settings.thoughtLevel.available[]
	ThoughtLevelCurrent string         // settings.thoughtLevel.current(档位 value)
	ThoughtLevelEnabled bool           // settings.thoughtLevel.enabled

	SlashCommands []SlashCommand // 顶层 slashCommands[](命令面板数据源)
}

// SessionProjection 快照 projection 段:总 token 占用与上下文窗口。
type SessionProjection struct {
	ContextWindow   int64  `json:"contextWindow"`
	TotalTokenCount int64  `json:"totalTokenCount"`
	Status          string `json:"status"`
	Mode            string `json:"mode"`
}

// InputFormat 模型输入模性(supportsImage 是审计 S 项要透出到 ModelInfo 的那一位)。
type InputFormat struct {
	SupportsText  bool `json:"supportsText"`
	SupportsImage bool `json:"supportsImage"`
	SupportsVideo bool `json:"supportsVideo"`
	SupportsAudio bool `json:"supportsAudio"`
	SupportsPdf   bool `json:"supportsPdf"`
}

// SessionModel 快照 settings.model.available[] 的一项(ref 与 reasoning 已展平)。
type SessionModel struct {
	ProviderID            string // ref.providerId
	ModelID               string // ref.modelId
	Label                 string // 展示名(缺省时调用方退回 ModelID)
	ProviderLabel         string // 供应方展示名
	ContextWindow         int64  // 该模型自己的窗口(可能与 projection 的不同)
	DefaultReasoningLevel string // reasoning.defaultLevel(GLM 系 setModel 必带)
	ReasoningLevels       []string
	InputFormat           InputFormat // properties.inputFormat
}

// ThoughtLevel settings.thoughtLevel.available[] 的一项(value 是协议值)。
type ThoughtLevel struct {
	Label string `json:"label"` // 展示名("disabled"/"enabled"/…)
	Value string `json:"value"` // 协议值(session/setThoughtLevel 的参数)
}

// SlashCommand 顶层 slashCommands[] 的一项(手机命令面板数据源)。
type SlashCommand struct {
	Name        string `json:"name"`        // 命令名(不带斜杠,实测 "goal"/"compact"/"init")
	Description string `json:"description"` // 一句话说明
	InputHint   string `json:"inputHint"`   // 参数提示(如 "/goal [pause|resume|…]",可为空)
}

// ParseSessionSnapshot 解析 session/create / session/resume 的应答快照。
// JSON 非法时返回错误;字段缺失/形状不符时按零值放过,不报错。
func ParseSessionSnapshot(raw json.RawMessage) (*SessionSnapshot, error) {
	var wire struct {
		Session struct {
			SessionID string `json:"sessionId"`
			Title     string `json:"title"`
		} `json:"session"`
		Projection SessionProjection `json:"projection"`
		Settings   struct {
			Model struct {
				Available []struct {
					Ref struct {
						ProviderID string `json:"providerId"`
						ModelID    string `json:"modelId"`
					} `json:"ref"`
					Label         string `json:"label"`
					ProviderLabel string `json:"providerLabel"`
					ContextWindow int64  `json:"contextWindow"`
					Reasoning     struct {
						DefaultLevel string            `json:"defaultLevel"`
						Levels       []json.RawMessage `json:"levels"`
					} `json:"reasoning"`
					Properties struct {
						InputFormat InputFormat `json:"inputFormat"`
					} `json:"properties"`
				} `json:"available"`
			} `json:"model"`
			ThoughtLevel struct {
				Available []ThoughtLevel `json:"available"`
				Current   string         `json:"current"`
				Enabled   bool           `json:"enabled"`
			} `json:"thoughtLevel"`
		} `json:"settings"`
		SlashCommands []SlashCommand `json:"slashCommands"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return nil, fmt.Errorf("解析 ZCode 会话快照失败: %w", err)
	}
	out := &SessionSnapshot{
		SessionID:           wire.Session.SessionID,
		Title:               wire.Session.Title,
		Projection:          wire.Projection,
		ThoughtLevels:       wire.Settings.ThoughtLevel.Available,
		ThoughtLevelCurrent: wire.Settings.ThoughtLevel.Current,
		ThoughtLevelEnabled: wire.Settings.ThoughtLevel.Enabled,
		SlashCommands:       wire.SlashCommands,
	}
	for _, m := range wire.Settings.Model.Available {
		out.Models = append(out.Models, SessionModel{
			ProviderID:            m.Ref.ProviderID,
			ModelID:               m.Ref.ModelID,
			Label:                 m.Label,
			ProviderLabel:         m.ProviderLabel,
			ContextWindow:         m.ContextWindow,
			DefaultReasoningLevel: m.Reasoning.DefaultLevel,
			ReasoningLevels:       parseReasoningLevels(m.Reasoning.Levels),
			InputFormat:           m.Properties.InputFormat,
		})
	}
	return out, nil
}

// ContextUsagePercent 上下文占用百分比(projection.totalTokenCount / contextWindow)。
// 窗口未知(<=0)或还没有用量时返回 0;超出窗口时封顶 100 —— 这个值是给手机端
// "占用环"用的,进度语义下宁可封顶也不显示 >100。
func (s *SessionSnapshot) ContextUsagePercent() float64 {
	if s == nil || s.Projection.ContextWindow <= 0 || s.Projection.TotalTokenCount <= 0 {
		return 0
	}
	pct := float64(s.Projection.TotalTokenCount) / float64(s.Projection.ContextWindow) * 100
	if pct > 100 {
		return 100
	}
	return pct
}

// ModelByID 按 modelId 查快照模型表(ref.modelId,如 "glm-5.2";不含 "zcode/" 前缀)。
// ModelKey preserves the provider/model pair when two providers use the same model ID.
func ModelKey(providerID, modelID string) string {
	return url.PathEscape(providerID) + "/" + url.PathEscape(modelID)
}
func (s *SessionSnapshot) ModelByID(modelID string) (SessionModel, bool) {
	if s == nil {
		return SessionModel{}, false
	}
	for _, m := range s.Models {
		if ModelKey(m.ProviderID, m.ModelID) == modelID {
			return m, true
		}
	}
	var found *SessionModel
	for i := range s.Models {
		if s.Models[i].ModelID == modelID {
			if found != nil {
				return SessionModel{}, false
			} // Ambiguous legacy ID: never choose the first account.
			found = &s.Models[i]
		}
	}
	if found != nil {
		return *found, true
	}
	return SessionModel{}, false
}

// SlashCommandByName 按名字查斜杠命令;name 带 "/" 前缀也认,大小写不敏感。
// (手机命令面板点击后要回查元数据拼提示用。)
func (s *SessionSnapshot) SlashCommandByName(name string) (SlashCommand, bool) {
	if s == nil {
		return SlashCommand{}, false
	}
	name = trimSlashPrefix(name)
	for _, c := range s.SlashCommands {
		if strings.EqualFold(trimSlashPrefix(c.Name), name) {
			return c, true
		}
	}
	return SlashCommand{}, false
}

// trimSlashPrefix 去掉开头的一个 "/"(没有则原样)。
func trimSlashPrefix(s string) string {
	if len(s) > 0 && s[0] == '/' {
		return s[1:]
	}
	return s
}

func parseReasoningLevels(raw []json.RawMessage) []string {
	var out []string
	for _, r := range raw {
		var value string
		if json.Unmarshal(r, &value) != nil {
			var option struct {
				Value string `json:"value"`
			}
			if json.Unmarshal(r, &option) != nil {
				continue
			}
			value = option.Value
		}
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}
