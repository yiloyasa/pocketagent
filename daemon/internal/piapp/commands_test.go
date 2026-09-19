// commands_test.go —— get_commands 封装的纯函数单测:应答形状解析钉死
// (探针实机证据形状见 commands.go 头注释),脏报文/无名条目防御。
package piapp

import (
	"encoding/json"
	"testing"
)

// 探针实拍形状(节选自 pi-cmds-probe.md 原始报文):三源混合 + sourceInfo。
func TestParseSlashCommandsProbeShape(t *testing.T) {
	raw := json.RawMessage(`{"commands":[
		{"name":"websearch","description":"Open web search curator","source":"extension","sourceInfo":{"scope":"user"}},
		{"name":"council","description":"Run a bounded council","source":"prompt"},
		{"name":"skill:mmx-cli","description":"Use mmx to generate text","source":"skill"}
	]}`)
	got := ParseSlashCommands(raw)
	if len(got) != 3 {
		t.Fatalf("应解析出 3 条,实得 %d:%v", len(got), got)
	}
	if got[0].Name != "websearch" || got[0].Source != "extension" {
		t.Errorf("第 1 条解析错: %+v", got[0])
	}
	if got[1].Source != "prompt" {
		t.Errorf("prompt 模板 source 应为 prompt: %+v", got[1])
	}
	if got[2].Name != "skill:mmx-cli" {
		// skill 的真名带 "skill:" 前缀,必须原样保留(手机回填 "/skill:mmx-cli " 才能触发)
		t.Errorf("skill 命令名应保留 skill: 前缀: %+v", got[2])
	}
	if got[0].Description != "Open web search curator" {
		t.Errorf("description 应原样透传: %+v", got[0])
	}
}

func TestParseSlashCommandsDefendsDirtyPayload(t *testing.T) {
	// 非 JSON / 缺 commands 字段 / 无名条目:一律空切片或过滤掉,不炸调用方
	if got := ParseSlashCommands(json.RawMessage(`not-json`)); len(got) != 0 {
		t.Errorf("脏报文应返回空切片,实得 %v", got)
	}
	if got := ParseSlashCommands(json.RawMessage(`{}`)); len(got) != 0 {
		t.Errorf("缺 commands 应返回空切片,实得 %v", got)
	}
	got := ParseSlashCommands(json.RawMessage(`{"commands":[{"name":"","description":"无名"},{"name":"ok"}]}`))
	if len(got) != 1 || got[0].Name != "ok" {
		t.Errorf("无名条目应被过滤: %v", got)
	}
}
