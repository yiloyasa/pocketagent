// probe_fixtures.go —— P0 探针(dsh-p0-probe.md)实测原始帧的测试夹具。
// 供 dshapp 与 agent 两个包的单测共用:形状钉死 = 实测契约钉死
// (AUDIT §4 缺陷即测试;探针形状漂移时夹具随证据文件一起更新)。
package dshapp

import "encoding/json"

// ProbeHandshakeJSON 探针第 1 项 initialize 应答(逐字)。
const ProbeHandshakeJSON = `{
  "protocolVersion": 1,
  "agentInfo": {"name": "deepseek-harness-acp", "version": "0.0.1"},
  "agentCapabilities": {
    "mcpCapabilities": {"http": true},
    "promptCapabilities": {"image": false, "audio": false, "embeddedContext": false},
    "sessionCapabilities": {"close": {}, "list": {}, "resume": {}}
  },
  "authMethods": []
}`

// ProbeConfigOptionsJSON 探针第 2 项 session/new 应答的 configOptions(逐字)。
// model 的 value/currentValue 是双层 JSON 编码串(探针修正①的实测依据)。
const ProbeConfigOptionsJSON = `[
  {
    "id": "model", "name": "Model", "category": "model", "type": "select",
    "currentValue": "[\"deepseek-official\",\"deepseek-v4-flash\"]",
    "options": [
      {
        "group": "deepseek-official", "name": "DeepSeek",
        "options": [
          {"value": "[\"deepseek-official\",\"deepseek-flash\"]", "name": "DeepSeek-V41-Flash"},
          {"value": "[\"deepseek-official\",\"deepseek-v4-flash\"]", "name": "DeepSeek-V4-Flash",
           "description": "Fast, efficient, and economical; suited to focused, routine, or parallel tasks."},
          {"value": "[\"deepseek-official\",\"deepseek-v4-pro\"]", "name": "DeepSeek-V4-Pro",
           "description": "Stronger agentic coding, knowledge, and difficult reasoning; suited to complex or quality-critical tasks at higher cost."},
          {"value": "[\"deepseek-official\",\"deepseek-v4-flash-vision-exp\"]", "name": "DeepSeek-V4-Flash-Vision-Exp"}
        ]
      }
    ]
  },
  {
    "id": "reasoning_effort", "name": "Reasoning effort", "category": "thought_level", "type": "select",
    "currentValue": "high",
    "options": [
      {"value": "off", "name": "Off", "description": "Use for simple tasks that do not need reasoning."},
      {"value": "low", "name": "Low", "description": "Prefer for routine or latency-sensitive tasks."},
      {"value": "high", "name": "High", "description": "The default balance for most tasks."},
      {"value": "max", "name": "Max", "description": "Reserve for the hardest quality-first tasks."}
    ]
  }
]`

// ProbeInitializeResult 探针握手夹具的结构化形态(解析失败 panic:夹具是
// 编译进二进制的常量,坏了属于程序员错误)。
func ProbeInitializeResult() *InitializeResult {
	r := &InitializeResult{}
	if err := json.Unmarshal([]byte(ProbeHandshakeJSON), r); err != nil {
		panic(err)
	}
	return r
}

// ParseConfigOptions 解析 configOptions JSON(测试夹具/诊断用)。
func ParseConfigOptions(raw string) ([]ConfigOption, error) {
	var opts []ConfigOption
	err := json.Unmarshal([]byte(raw), &opts)
	return opts, err
}
