// commands.go —— piapp.Client 的 get_commands 封装(pi 斜杠命令清单)。
//
// 【实机证据】0.85.1,cmd/piprobe -mode cmds(2026-09-19,docs/verify/evidence/
// pi-cmds-probe.md):get_commands 应答 data 形如
//
//	{"commands":[{"name":"websearch","description":"Open web search curator",
//	              "source":"extension","sourceInfo":{...}}, ...]}
//
// source ∈ extension(扩展命令)/ prompt(prompt 模板)/ skill(技能,名字带
// "skill:" 前缀)。三源都是"prompt 以 /<name> 开头即可触发"的真命令:extension
// 命令由 session.prompt 直接执行,skill:/模板由 agent-session 内建展开
// (_expandSkillCommand / expandPromptTemplate),所以名字**原样透传**就是手机端
// 点选后要回填的 "/命令 " 真名。
package piapp

import (
	"context"
	"encoding/json"
	"time"
)

// SlashCommand pi get_commands 里的一条命令(展平 sourceInfo,只留面板要用的字段)。
type SlashCommand struct {
	Name        string `json:"name"`        // 不带斜杠的真名(skill 形如 "skill:mmx-cli")
	Description string `json:"description"` // 一句话说明(可空)
	Source      string `json:"source"`      // extension / prompt / skill
}

// GetCommands 拉当前会话可用的斜杠命令清单(本地注册面,毫秒级应答)。
// 命令清单是静态注册面,10s 上界纯兜底(ctx 先到也算失败);失败原样返回错误。
func (c *Client) GetCommands(ctx context.Context) ([]SlashCommand, error) {
	raw, err := c.CallCtx(ctx, "get_commands", nil, 10*time.Second)
	if err != nil {
		return nil, err
	}
	return ParseSlashCommands(raw), nil
}

// ParseSlashCommands 解析 get_commands 应答(抽成纯函数单测钉形状):
// 缺 commands 字段 / 应答不是 JSON 对象时返回空切片,不让脏报文炸调用方。
func ParseSlashCommands(raw json.RawMessage) []SlashCommand {
	var res struct {
		Commands []SlashCommand `json:"commands"`
	}
	if json.Unmarshal(raw, &res) != nil {
		return nil
	}
	out := make([]SlashCommand, 0, len(res.Commands))
	for _, sc := range res.Commands {
		if sc.Name == "" {
			continue // 无名命令点不动,防御性跳过
		}
		out = append(out, sc)
	}
	return out
}
