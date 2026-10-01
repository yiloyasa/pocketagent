package agent

// collab_inject.go —— codex / zcode / hermes 的协同 MCP 真注入 + dsh 判据修正
// (X2 协同面收口,2026-09-30)。
//
// 【CollabAware 的判据是"接线事实"】provider.go 的合同:CollabReady 必须报告
// "本 provider 那一侧的模型看得见 agent_delegate"。pi/dsh 之外的三家此前无
// CollabAware 实现(手机端协同入口只亮 pi),本轮按各家的真实注入通道补齐 ——
// 每条通道都以"对端协议/适配器的实证形状"为据,不是配置开关:
//
//   - codex:`codex app-server` argv 以 `-c mcp_servers.pocketagent-collab.*`
//     内存态覆盖(codexapp.daemonScopeArgs 同款纪律:只进子进程内存,绝不写用户
//     ~/.codex/config.toml;本机 codex-cli 0.159.2 实测 `-c` 接受该键,非法键
//     会当场报 "Error parsing -c overrides",合法键静默通过)。
//   - zcode:session/create 与 session/resume 的**会话参数** mcpServers
//     (zcode.cjs app-server 协议 schema 实证:两方法的 zod schema 均含
//     `mcpServers: array({name,command,args,env:[{name,value}]}, strict)`)。
//   - hermes:ACP session/new·load·resume·fork 的 mcpServers(适配器
//     acp_adapter/server.py:_register_session_mcp_servers 实证:非空即
//     register_mcp_servers 注册进会话工具面 —— 与 dsh 的"非空拒绝"相反,
//     这是真注册)。
//
// 三家的注入要素与 pi/dsh 完全同构:命令 = 本 daemon 可执行文件,-collab-mcp
// 子模式 + 共享数据目录;回环鉴权靠薄代理的 pair-code.txt 回退链(cmd 层
// collabProxyToken,与 pi/dsh 的 env 注入取同一个 legacy token),故这里不再
// 注入 POCKETAGENT_TOKEN —— agent 包够不到 server 的 token 派生(依赖方向),
// 而回退链本就是 pi 会话路径(env 被裸参数重写)的既定兜底。
//
// 【深度锁与既有两家同构】注入发生在 provider 的会话建立路径,该层无法区分
// "用户主动接入"与"委派子线程",子线程因此同样看得见 agent_delegate —— 与
// pi(全局 env 注入)/dsh(部署级补丁)完全同构;深度锁由引擎第二道闸
// (server/collab_delegate.go 的 childThreads 入口判定)兜底,本轮不改其语义。
//
// 【注入点与文件所有权】codex 的 argv 拼装在 codexapp.Start(经 CollabExtraArgs
// 钩子)、hermes 的请求拼装在 hermesapp 四个会话方法(经 SessionMCPServersSource
// 钩子)、zcode 的会话参数在 agent/zcode_session.go 两个调用点(withCollabMCP)。
// 钩子全部**惰性现算**:数据目录在 daemon 启动期才注入(main.go →
// SetRuntimeDataDir),包级 init 只挂钩子、不定格参数。

import (
	"fmt"
	"os"
	"strings"

	"github.com/pocketagent/daemon/internal/codexapp"
	"github.com/pocketagent/daemon/internal/collabmcp"
	"github.com/pocketagent/daemon/internal/dshapp"
	"github.com/pocketagent/daemon/internal/hermesapp"
)

func init() {
	// codex:argv 追加钩子 —— Start 每次拉起 app-server 时现算。
	codexapp.CollabExtraArgs = collabCodexExtraArgs
	// hermes:四个会话建立/重挂请求的 mcpServers 来源。
	hermesapp.SessionMCPServersSource = func() []map[string]any {
		if e := CollabSessionMCPEntry(); e != nil {
			return []map[string]any{e}
		}
		return nil
	}
}

// RuntimeDataDir 返回 daemon 数据目录(SetRuntimeDataDir 的读端)。
// 注入要素(-data)在 daemon 启动期才可知,调用方必须现取、不许缓存。
func RuntimeDataDir() string {
	runtimeDataDirMu.Lock()
	defer runtimeDataDirMu.Unlock()
	return runtimeDataDir
}

// collabProxyLaunch 返回把本 daemon 注册为 pocketagent-collab 本地 MCP 服务器
// 的启动要素:命令 = os.Executable()(daemon 自己),参数 = -collab-mcp 子模式 +
// 共享数据目录(pi/dsh 接线同款)。要素不齐(exe 定位失败/数据目录未注入)返回
// ok=false —— 调用方不注入、CollabReady 如实报不具备,绝不摆"开关开着就算
// 接线"的假可用。
func collabProxyLaunch() (command string, args []string, ok bool) {
	exe, err := os.Executable()
	if err != nil {
		return "", nil, false
	}
	if strings.TrimSpace(RuntimeDataDir()) == "" {
		return "", nil, false
	}
	return exe, []string{"-collab-mcp", "-data", RuntimeDataDir()}, true
}

// CollabSessionMCPEntry 返回 pocketagent-collab 的 stdio MCP 服务器条目。
// 形状对齐 zcode session 参数与 ACP mcpServers 的公共子集:
// {name, command, args: string[], env: [{name, value}]}(zcode 侧 zod schema
// strict,多余键会被整条请求拒掉 —— 只发双方都认识的四个键)。
// 未接线返回 nil。
func CollabSessionMCPEntry() map[string]any {
	command, args, ok := collabProxyLaunch()
	if !ok {
		return nil
	}
	// env 留空数组:薄代理有 pair-code.txt 回退链兜鉴权(见文件头),schema
	// 要求 env 在场(zcode strict),给空数组而不是省键。
	return map[string]any{
		"name":    collabmcp.ServerName,
		"command": command,
		"args":    args,
		"env":     []any{},
	}
}

// withCollabMCP 协同 MCP 会话参数注入(zcode 侧调用):接线成立时给会话请求
// 参数带上 pocketagent-collab 的 stdio 条目;未接线时不带该键 —— 旧版 zcode
// 的 strict schema 不认未知键,不带 = 与改造前零差异。
func withCollabMCP(params map[string]interface{}) map[string]interface{} {
	if e := CollabSessionMCPEntry(); e != nil {
		params["mcpServers"] = []any{e}
	}
	return params
}

// ---- codex:argv 覆盖 -----------------------------------------------------------

// collabCodexExtraArgs 把 pocketagent-collab 以 `-c mcp_servers.*` 覆盖挂进
// app-server argv。值按 TOML 字面量编码(codex 的 -c 覆盖按 TOML 解析:字符串
// 必须带引号,Windows 路径的反斜杠必须转义);键里的连字符是 TOML bare key
// 的合法字符,无需加引号。未接线返回 nil(零追加)。
func collabCodexExtraArgs() []string {
	command, args, ok := collabProxyLaunch()
	if !ok {
		return nil
	}
	base := "mcp_servers." + collabmcp.ServerName
	return []string{
		"-c", base + ".command=" + tomlString(command),
		"-c", base + ".args=" + tomlStringArray(args),
	}
}

// tomlString 把字符串编码成 TOML basic string(转义 \ " 与控制字符;非 ASCII
// 原样保留 —— TOML 文件约定 UTF-8,codex 按 UTF-8 读)。
func tomlString(s string) string {
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
				b.WriteString(fmt.Sprintf(`\u%04X`, r))
				continue
			}
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// tomlStringArray 把字符串切片编码成 TOML 数组(元素逐个 tomlString)。
func tomlStringArray(xs []string) string {
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = tomlString(x)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// ---- dsh:部署级补丁判据 ---------------------------------------------------------

// dshCollabPatchWired 读部署级 profile 补丁层文件,判定 pocketagent-collab 是否
// 已注册(文件不可读/不含条目 = 未接线)。路径参数化以便单测注入临时文件。
func dshCollabPatchWired(path string) bool {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	// 补丁条目的 id 是 "pocketagent-collab-mcp"(dshapp 内部常量),含共享
	// 服务器名 pocketagent-collab(collabmcp.ServerName)为子串;文件由
	// dshapp.WriteCollabMcpPatch 生成,内容形状受 collabpatch_test.go 钉死。
	return strings.Contains(string(raw), collabmcp.ServerName)
}

// dshCollabWired 生产入口:默认隔离 HOME 下的补丁层。
func dshCollabWired() bool { return dshCollabPatchWired(dshapp.CollabPatchPath("")) }

// ---- CollabReady 三家落点 -------------------------------------------------------

// CollabReady 见 CollabAware。判据 = 下一次 app-server 拉起的 argv 将真实带上
// pocketagent-collab 的 mcp_servers 覆盖(注入要素齐备),不是配置开关的意愿。
// 覆盖随子进程拉起生效:长驻 app-server 要等下一次重启才看得到工具 —— 与
// pi/dsh 的"下次会话生效"同口径,如实边界,不算虚报。
func (p *CodexProvider) CollabReady() bool { return len(collabCodexExtraArgs()) > 0 }

// CollabReady 见 CollabAware。判据 = 新建/重挂会话的请求将真实带上
// pocketagent-collab 条目(zcode 会话参数通道)。
func (p *ZcodeProvider) CollabReady() bool { return CollabSessionMCPEntry() != nil }

// CollabReady 见 CollabAware。判据 = 会话请求将真实带上 pocketagent-collab
// 条目(hermes ACP 会话参数通道,适配器非空即真注册)。
func (p *HermesProvider) CollabReady() bool { return CollabSessionMCPEntry() != nil }
