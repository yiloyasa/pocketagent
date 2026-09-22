package zcodeapp

import (
	"encoding/json"
	"testing"
)

// TestToolInputIsSubagent 把"ZCode 子代理委派"的识别依据钉死:纯结构判定,
// 不依赖工具名(Agent/Task/subagent),也不把普通工具误判。
func TestToolInputIsSubagent(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  bool
	}{
		{
			// ZCode Agent 工具 completed 态入参(prompt + agentType + description)
			name:  "子代理委派_completed",
			input: `{"prompt":"撰写调研方案","description":"多 Agent 协同调研","agentType":"general-purpose","model":"glm-5.3"}`,
			want:  true,
		},
		{
			// ZCode Agent 工具 async_launched 态入参(prompt + outputFile)
			name:  "子代理委派_async",
			input: `{"prompt":"后台跑调研","description":"task","outputFile":"C:/a/task.output","subagent_type":"general-purpose"}`,
			want:  true,
		},
		{
			// 带 parentToolCallId 的嵌套委派(子代理再派子代理)
			name:  "子代理委派_嵌套",
			input: `{"prompt":"细分任务","parentToolCallId":"call_parent"}`,
			want:  true,
		},
		{
			// 普通 Bash 工具:入参是 command,没有 prompt/子代理特征键
			name:  "普通工具_Bash",
			input: `{"command":"ls -la","description":"列出目录"}`,
			want:  false,
		},
		{
			// 有 prompt 但没有任一子代理特征键:不误判为子代理
			name:  "仅prompt无特征键",
			input: `{"prompt":"只问一句话"}`,
			want:  false,
		},
		{
			// 空入参:当普通工具
			name:  "空入参",
			input: `{}`,
			want:  false,
		},
		{
			// 非法 JSON:识别失败不报错,返回 false
			name:  "非法JSON",
			input: `not json`,
			want:  false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ToolInputIsSubagent(json.RawMessage(c.input))
			if got != c.want {
				t.Fatalf("ToolInputIsSubagent(%s) = %v, want %v", c.name, got, c.want)
			}
		})
	}
}

// TestSubagentInputMarkerKeys 文档守卫:子代理特征键集合非空(防止 subagent.go
// 的注释与代码漂移;本测试只断言"有键",键的语义由 TestToolInputIsSubagent 覆盖)。
func TestSubagentInputMarkerKeys(t *testing.T) {
	if len(subagentInputMarkerKeys()) == 0 {
		t.Fatal("子代理特征键不应为空")
	}
}
