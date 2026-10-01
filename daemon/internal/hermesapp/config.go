package hermesapp

import (
	"os"
	"path/filepath"
	"strings"
)

// ===== config.yaml 的 agent.reasoning_effort 编辑(hermes 强度真实化)=====
//
// hermes 的思考强度没有运行时 RPC:唯一入口是家目录 config.yaml 的
// `agent.reasoning_effort`(resolve_reasoning_config,所有面共用同一收口),
// 会话创建那一刻读取。daemon 在用户于手机上显式选择强度时写该键 —— 与用户
// 手动编辑 config.yaml 等价(hermes 官方配置面,G:\Hermes 是 hermes 自己的家,
// 与 ~/.codex 等只读红线无关)。
//
// 编辑策略(保守行级):已有 `reasoning_effort:` 键 → 原值替换;`agent:` 段内
// 追加;两者皆无 → 文件尾追加 agent: 段。不引入 YAML 库(依赖极简红线),
// 只处理本键的两级缩进形态;解析交给 hermes 自己的 loader。

// ConfigPath 返回 hermes 家目录 config.yaml 的绝对路径。
// 家目录定位:HERMES_HOME 环境变量(适配器同款语义)→ 默认 <盘根>\Hermes 的
// 实证位不可移植,改为按 HERMES_HOME/缺省用户家 .hermes 顺序;本机 hermes-npm
// 把 HERMES_HOME 指到 G:\Hermes(status 实测)。
func ConfigPath() string {
	if home := strings.TrimSpace(os.Getenv("HERMES_HOME")); home != "" {
		return filepath.Join(home, "config.yaml")
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".hermes", "config.yaml")
	}
	return ""
}

// SetReasoningEffort 把 agent.reasoning_effort 写为 effort(空串 = 删除该键,
// 回"跟随 hermes 默认")。写入是原子的(临时文件+rename)。
func SetReasoningEffort(effort string) error {
	path := ConfigPath()
	if path == "" {
		return errNoHermesHome
	}
	raw, _ := os.ReadFile(path) // 不存在 = 空内容,走追加分支
	lines := strings.Split(string(raw), "\n")
	agentIdx := -1
	keyIdx := -1
	for i, ln := range lines {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "agent:") && !strings.HasPrefix(t, "agent_") {
			agentIdx = i
		}
		if agentIdx >= 0 && strings.HasPrefix(t, "reasoning_effort:") {
			keyIdx = i
			break
		}
	}
	trimmed := strings.TrimSpace(effort)
	switch {
	case keyIdx >= 0 && trimmed != "":
		lines[keyIdx] = "  reasoning_effort: " + trimmed
	case keyIdx >= 0 && trimmed == "":
		lines = append(lines[:keyIdx], lines[keyIdx+1:]...)
	case agentIdx >= 0 && trimmed != "":
		lines = append(lines[:agentIdx+1], append([]string{"  reasoning_effort: " + trimmed}, lines[agentIdx+1:]...)...)
	default: // 无 agent 段:文件尾追加(空 effort 且无段 = 无事可做)
		if trimmed == "" {
			return nil
		}
		if len(lines) > 0 && lines[len(lines)-1] != "" {
			lines = append(lines, "")
		}
		lines = append(lines, "agent:", "  reasoning_effort: "+trimmed)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

type hermesHomeError struct{}

func (hermesHomeError) Error() string { return "hermes 家目录不可定位(HERMES_HOME 未设且无用户家)" }

var errNoHermesHome = hermesHomeError{}

// validReasoningEfforts 与 hermes 的 VALID_REASONING_EFFORTS 对齐
// (hermes_constants.py:1300)。daemon 在写入前校验,非法档宁可不写。
var validReasoningEfforts = map[string]bool{
	"minimal": true, "low": true, "medium": true,
	"high": true, "xhigh": true, "max": true, "ultra": true,
}

// IsValidReasoningEffort 报告 effort 是否是 hermes 认的档位(空串不算——
// 它是"跟随默认",走删键路径,由调用方直接处理)。
func IsValidReasoningEffort(effort string) bool {
	return validReasoningEfforts[strings.ToLower(strings.TrimSpace(effort))]
}
