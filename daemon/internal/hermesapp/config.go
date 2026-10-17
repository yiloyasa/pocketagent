package hermesapp

import (
	"os"
	"path/filepath"
	"strings"
)

// ===== config.yaml 的 agent.reasoning_effort 编辑(hermes 强度真实化)=====
//
// hermes 的思考强度没有运行时 RPC:唯一入口是 hermes 家目录 config.yaml 的
// `agent.reasoning_effort`(resolve_reasoning_config,所有面共用同一收口),
// 会话创建那一刻读取。daemon 在用户于手机上显式选择强度时写该键。
//
// 【读放宽 / 写收紧】daemon **只读不改**用户桌面配置(与 ~/.codex / ~/.dsh /
// ~/.pi 同一红线,AGENTS.md §4 红线 2;包注释 locate.go:11-15 同款声明):
//   - 读放宽:ConfigPath() 给 status/查询面**用户真实的**配置位置 ——
//     %HERMES_HOME% 未设时回落用户家 ~/.hermes。"一律只读"禁的是写,读用户
//     自己手写的配置是允许的。
//   - 写收紧:SetReasoningEffort() 只认 WritableConfigPath() —— %HERMES_HOME%
//     非空才给落点;未设/空白 → 返回错误 errNoHermesHome,**绝不回落用户家
//     目录**(绝不写 ~/.hermes)。%HERMES_HOME% 是 hermes 自己的家目录,由
//     hermes-npm 安装时指向 hermes 安装目录(status 实测),不是用户桌面
//     agent 配置面。
//   - 诚实缺席:写不进去时错误原样上抛,由调用方(agent/hermes.go)如实透给
//     手机端"未固定 + 原因",**不静默降级、不谎报已设为**(红线 5「不留假绿」)。
//
// 编辑策略(保守行级):已有 `reasoning_effort:` 键 → 原值替换;`agent:` 段内
// 追加;两者皆无 → 文件尾追加 agent: 段。不引入 YAML 库(依赖极简红线),
// 只处理本键的两级缩进形态;解析交给 hermes 自己的 loader。

// ConfigPath 返回 hermes 家目录 config.yaml 的绝对路径。
// **只读面专用**(status/查询看用户真实配置);写入面用 WritableConfigPath,
// 两者落点规则不同,不要混用。
// 家目录定位:HERMES_HOME 环境变量(适配器同款语义)→ 缺省用户家 .hermes
// 顺序;曾按个人盘固定安装位的实证路径硬编码,因不可移植已废弃;hermes-npm
// 安装时会把 HERMES_HOME 指到 hermes 安装目录(status 实测)。
func ConfigPath() string {
	if home := strings.TrimSpace(os.Getenv("HERMES_HOME")); home != "" {
		return filepath.Join(home, "config.yaml")
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".hermes", "config.yaml")
	}
	return ""
}

// WritableConfigPath 返回 daemon **允许写**的 config.yaml 绝对路径:
// 只认 %HERMES_HOME%(适配器同款语义)。未设/空白 → 空串,调用方必须当失败
// 处理,严禁改回落用户家 ~/.hermes(红线 2:daemon 只读不改用户桌面配置)。
func WritableConfigPath() string {
	if home := strings.TrimSpace(os.Getenv("HERMES_HOME")); home != "" {
		return filepath.Join(home, "config.yaml")
	}
	return ""
}

// SetReasoningEffort 把 agent.reasoning_effort 写为 effort(空串 = 删除该键,
// 回"跟随 hermes 默认")。写入是原子的(临时文件+rename)。
// **只写 %HERMES_HOME% 下的 config.yaml**;该环境变量未设/空白时返回
// errNoHermesHome 并**一个字节都不落**(绝不回落用户家目录 —— 见文件头
// 「读放宽 / 写收紧」)。错误不吞:调用方须如实透出,不许静默当成功。
func SetReasoningEffort(effort string) error {
	path := WritableConfigPath()
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

// hermesHomeError 是**写入面**的定位失败:HERMES_HOME 未设/空白,daemon 无
// 权可写的 config.yaml。文案必须说清"为什么不写"(红线 2,只读不改用户桌面
// 配置),让手机端能原样透出给用户,而不是一句含糊的"不可定位"。
type hermesHomeError struct{}

func (hermesHomeError) Error() string {
	return "HERMES_HOME 未设置,daemon 不改写用户家目录配置(~/.hermes),未固定强度(设 HERMES_HOME 指向 hermes 安装目录即可)"
}

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
