// collabpatch.go —— 把本地 collab MCP 注册进 dsh 的**部署级** Cordis 配置。
//
// 【为什么必须走部署级,而不是 session/new 的 mcpServers】
// 官方 handbook 实测:DSH 的 MCP 实例是**部署级 Cordis 插件**,ACP Session 是在
// 那套固定 Host 组合**之后**才创建的 —— "ACP 客户端声称支持 MCP"并不等于它的
// 服务器能被 DSH 使用;另一份 dsh-acp 文档也写明 session/new 的 mcpServers
// **空值接受、非空拒绝**。所以会话级注入是**假成功**(可能不报错但被忽略),
// 唯一可靠路径是在 profile 的 patch 层注册 @deepseek-ai/dsh-mcp-client
// (字段形状取自该包 lib/index.js 的 createTransport + serverName)。
//
// 【落在哪】dsh 跑在**隔离 HOME**(~/.pocketagentd/dsh-home,daemon 数据目录内),
// profile 补丁层是 profiles/acp/cordis.patch.yml —— **归我们所有,不碰用户的 ~/.dsh**。
//
// 【为什么不引入 YAML 库】daemon 的依赖原则是"极简"(无 ORM/无框架);
// profile 补丁层的形状是**固定的顶层数组**,自己按 YAML 双引号标量安全转义即可,
// 不值得为它拉一个解析器进来。
//
// 【为什么只在"空层"时写】cordis.patch.yml 自称 "Your patch layer",用户可能已经
// 放了别的东西。要往里**追加**就得解析 YAML(又回到引入库的问题),所以这里取保守解:
//
//	· 空层(空文件 / `[]`)        → 写入我们的条目
//	· 已有我们的 id               → 视为已接线(幂等,不动)
//	· 非空且无我们的 id           → **不覆盖**,只回报 false 让调用方留痕提示
//
// 宁可"这次没接上、但用户内容完好",也不整表覆盖把别人的 patch 抹掉。
package dshapp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// collabPatchID patch 条目的 id:按它判定"已接线",也是幂等锚点。
const collabPatchID = "pocketagent-collab-mcp"

// CollabPatchPath 返回隔离 HOME 下 profile 补丁层的路径。
func CollabPatchPath(dshHome string) string {
	if strings.TrimSpace(dshHome) == "" {
		dshHome = defaultDshHome()
	}
	return filepath.Join(dshHome, "profiles", "acp", "cordis.patch.yml")
}

// WriteCollabMcpPatch 把 collab MCP 写进 profile 补丁层。
//
// 返回 (已接线, 出错)。"已接线"为 true 时文件确实含我们的条目(本次写入或本来就有)。
// 非空且无我们条目时返回 (false, nil) —— 这不是错误,是"刻意不覆盖"的降级。
func WriteCollabMcpPatch(dshHome, command string, args []string, env map[string]string) (bool, error) {
	if strings.TrimSpace(command) == "" {
		return false, nil // 没有可执行文件路径 = 无法接线,静默跳过(调用方已记录)
	}
	path := CollabPatchPath(dshHome)

	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	body := strings.TrimSpace(string(existing))
	switch {
	case strings.Contains(body, collabPatchID):
		// 已有我们的条目:幂等返回,不动文件(用户可能手动改过路径,尊重他)。
		return true, nil
	case body != "" && body != "[]" && !isEmptyCommentOnly(body):
		// 非空且是别人的 patch:不覆盖,交给调用方提示用户手动合并。
		return false, nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	if err := os.WriteFile(path, []byte(renderCollabPatch(command, args, env)), 0o600); err != nil {
		return false, err
	}
	return true, nil
}

// isEmptyCommentOnly 判断"层里只有注释、没有实际条目"(初始文件就是这种形态)。
func isEmptyCommentOnly(body string) bool {
	for _, line := range strings.Split(body, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		if t == "[]" {
			continue
		}
		return false // 出现任何真实内容就不是"空注释层"
	}
	return true
}

// renderCollabPatch 生成补丁层内容:一个只含我们这条的顶层数组。
//
// 全部标量都用 YAML 双引号形式并转义,所以 Windows 路径里的 `\`、含 `:` 的 URL、
// 带空格的目录都不会把 YAML 结构带坏。
func renderCollabPatch(command string, args []string, env map[string]string) string {
	var b strings.Builder
	b.WriteString("# PocketAgent 写入:把本地 collab MCP 注册为 dsh 的部署级插件。\n")
	b.WriteString("# 由 daemon 自动维护(见 internal/dshapp/collabpatch.go);如已自行配置请手动合并。\n")
	b.WriteString("- id: " + yamlScalar(collabPatchID) + "\n")
	b.WriteString("  name: " + yamlScalar("@deepseek-ai/dsh-mcp-client") + "\n")
	b.WriteString("  config:\n")
	b.WriteString("    serverName: " + yamlScalar("collab") + "\n") // 工具名 → mcp__collab__<tool>
	b.WriteString("    transport: " + yamlScalar("stdio") + "\n")
	b.WriteString("    command: " + yamlScalar(command) + "\n")
	b.WriteString("    args:\n")
	for _, a := range args {
		b.WriteString("      - " + yamlScalar(a) + "\n")
	}
	b.WriteString("    env:\n")
	for _, k := range sortedKeys(env) {
		b.WriteString("      " + k + ": " + yamlScalar(env[k]) + "\n")
	}
	return b.String()
}

// sortedKeys 让 env 输出稳定(否则 map 遍历顺序会让每次写出的文件不同)。
func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	for i := 1; i < len(out); i++ { // 插入排序:键极少,不值得引 sort
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// yamlScalar 把任意字符串安全写成 YAML 双引号标量。
// 双引号形式是 YAML 里唯一能完整表达任意内容的标量形式:转义 \ " 与控制字符即可,
// 其余字符(含中文)原样保留。
func yamlScalar(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				b.WriteString(fmt.Sprintf(`\x%02x`, r))
				continue
			}
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}
