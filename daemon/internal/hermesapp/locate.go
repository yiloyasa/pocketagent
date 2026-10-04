// Package hermesapp 桥接 Hermes Agent(Nous Research,经 npm bridge 的 ACP
// stdio 适配器)。与 dshapp 同族:stdio JSON-RPC,一轮一会话或续会话。
//
// 【运行时形态(2026-10-02 本机实证)】
// hermes-agent npm 包(runtime/hermes-agent/)内含 git checkout + uv 托管 venv,
// ACP 适配器入口 = venv python -m acp_adapter(入口注册名 hermes-acp)。
// 协议 = Agent Client Protocol 0.9.0(Zed 系):initialize → session/new →
// session/prompt → session/update 流(agent_message_chunk 正文 /
// agent_thought_chunk 思考 / tool_call 工具)→ prompt 应答(stopReason)。
//
// 【凭据/模型】hermes 家目录(%HERMES_HOME%,.env + config.yaml +
// auth.json)。模型由 config.yaml 的 model.default 决定;key 走 .env 的
// DEEPSEEK_API_KEY 等(env-seeded)。daemon **只读不改**用户家配置(与
// ~/.codex 同一红线);未配置时 ACP initialize 的 authMethods 如实上报,
// 首轮 prompt 以明确错误回给手机端。
package hermesapp

import (
	"os"
	"path/filepath"
	"strings"
)

// LocateHermes 定位 ACP 适配器的启动要素:(pythonExe, acpAdapterDir)。
// 探测顺序:
//  1. HERMES_NPM_ROOT 环境变量(显式指定 hermes-agent npm 包根,高级用户);
//  2. 各 npm 全局/项目常见根下的 hermes-agent 包(runtime checkout 已就绪);
//  3. 未命中返回错误(不猜、不装、不改用户环境)。
//
// venv python 必须真实存在(runtimeReady);只找到包根而 venv 缺 = npm 包
// 装了没初始化,报错信息带修复命令(hermes-npm status / 重跑安装)。
func LocateHermes() (string, string, error) {
	candidates := []string{}
	if r := strings.TrimSpace(os.Getenv("HERMES_NPM_ROOT")); r != "" {
		candidates = append(candidates, r)
	}
	// 开发机自测:用 HERMES_NPM_ROOT 环境变量指向本机 hermes-agent 安装位
	// (勿在代码硬编码个人盘符路径 —— 会被编进二进制,触发打包痕迹闸门)。
	if npmGlobal := npmGlobalRoot(); npmGlobal != "" {
		candidates = append(candidates, filepath.Join(npmGlobal, "hermes-agent"))
	}
	for _, root := range candidates {
		py := filepath.Join(root, "runtime", "hermes-agent", "venv", "Scripts", "python.exe")
		if st, err := os.Stat(py); err == nil && !st.IsDir() {
			acpDir := filepath.Join(root, "runtime", "hermes-agent")
			return py, acpDir, nil
		}
	}
	return "", "", errHermesNotFound
}

func npmGlobalRoot() string {
	// npm prefix -g 常见落位(Windows):%APPDATA%\npm
	appdata, err := os.UserConfigDir()
	if err != nil || appdata == "" {
		return ""
	}
	p := filepath.Join(appdata, "npm", "node_modules")
	if st, err := os.Stat(p); err == nil && st.IsDir() {
		return p
	}
	return ""
}

var errHermesNotFound = &HermesNotFoundError{}

// HermesNotFoundError 定位失败的固定文案(含修复指引,面向手机端透出)。
type HermesNotFoundError struct{}

func (e *HermesNotFoundError) Error() string {
	return "未找到 Hermes Agent 运行时(npm 包装了但没初始化,先跑 npm 包的 postinstall;" +
		"或用 HERMES_NPM_ROOT 指向 hermes-agent 包根)"
}
