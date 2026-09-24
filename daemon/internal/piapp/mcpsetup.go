// mcpsetup.go —— pi 协作 MCP 生产接线(T-L2pi-prod):daemon 自有 pi agent 目录、
// pi-mcp-adapter 一次性安装、mcp.json 写入器、子进程 env 注入。
//
// 【背景(T-L2pi 探针全链路实证)】pi-mcp-adapter 让 pi 通过 MCP 协议调用外部
// 服务器(collabmcp 落地后,pi 可借此把"委派/审批"接回 daemon)。adapter 由
// `pi install npm:pi-mcp-adapter --no-approve` 安装(非交互旗标已实证),装完写
// <agentdir>/settings.json(packages 列表)与 <agentdir>/npm/node_modules/;
// 插件运行时读 <agentdir>/mcp.json 的 {"mcpServers":{...}},服务器懒启动。
//
// 【隔离原则(与 dshprobe 的 DSH_HOME 同款)】daemon 绝不动用户默认 pi 目录
// (~/.pi 红线):协作路径使用 daemon 自有目录(默认 ~/.pocketagentd/pi-agent),
// 拉起子进程时以 PI_CODING_AGENT_DIR 注入。用户 env 已设 PI_CODING_AGENT_DIR 时
// 尊重之:目录解析用用户的,env 注入跳过(子进程经 os.Environ() 已继承,重复
// 注入会在 Windows env 块里产生同键重复,取值未定义)。
//
// 【总开关】本功能整体默认关闭:PA_PI_COLLAB ∈ {1,true,yes,on}(大小写不敏感)
// 开启,或上层经 SetCollabEnabled 显式设置(将来 --pi-collab flag 落地后的接线点,
// cmd/ 层本任务不可改)。默认关的原因:collabmcp 服务器尚未接线,开着会让 pi
// 凭空带一个不可用的 MCP 服务器。
package piapp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/pocketagent/daemon/internal/hidewin"
)

// ---- 常量 ----

const (
	// piAdapterPackage pi-mcp-adapter 的包标识(pi install 的实参与 settings.json
	// packages 列表里的落盘形态,探针实证两者一致)。
	piAdapterPackage = "npm:pi-mcp-adapter"
	// agentDirEnv pi 的 agent 目录隔离环境变量(pi 官方约定)。
	agentDirEnv = "PI_CODING_AGENT_DIR"
	// collabSwitchEnv 协作 MCP 总开关(env 形态的等价 --pi-collab flag,默认关)。
	collabSwitchEnv = "PA_PI_COLLAB"
	// mcpConfigFile pi-mcp-adapter 读取的 MCP 服务器注册表文件名(相对 agent 目录)。
	mcpConfigFile = "mcp.json"
	// piInstallTimeout pi install 的等待上界:npm 拉包+依赖树,冷缓存分钟级。
	piInstallTimeout = 5 * time.Minute
)

// CollabMcpServerName mcp.json 中 daemon 协作 MCP 服务器的注册名。
const CollabMcpServerName = "pocketagent-collab"

// ---- 开关 ----

// collabOverride/collabOverrideSet 显式开关(--pi-collab flag 落地后由 cmd 层经
// SetCollabEnabled 设置);未显式设置时回落 PA_PI_COLLAB env(默认关)。
var (
	collabOverride    atomic.Bool
	collabOverrideSet atomic.Bool
	configuredDataDir atomic.Pointer[string]
)

// SetDataDir wires the daemon-resolved data root into pi's isolated agent
// directory. A user-provided PI_CODING_AGENT_DIR still has precedence.
func SetDataDir(dir string) {
	dir = strings.TrimSpace(dir)
	configuredDataDir.Store(&dir)
}

// SetCollabEnabled 显式设置协作 MCP 总开关(优先于 env)。供将来 cmd/pocketagentd
// 的 --pi-collab flag 调用;当前生产路径无人调用,默认关。
func SetCollabEnabled(v bool) {
	collabOverrideSet.Store(true)
	collabOverride.Store(v)
}

// CollabEnabled 报告协作 MCP 总开关状态。默认关(未设 PA_PI_COLLAB 时 false)。
func CollabEnabled() bool {
	if collabOverrideSet.Load() {
		return collabOverride.Load()
	}
	return envTruthy(os.Getenv(collabSwitchEnv))
}

// envTruthy 开关真值判定:1/true/yes/on(大小写不敏感)为真,其余(含空)为假。
func envTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// ---- agent 目录解析 ----

// DefaultCollabAgentDir daemon 自有 pi agent 目录默认值:~/.pocketagentd/pi-agent。
// 取不到用户主目录返回 ""(调用方按未就绪处理,绝不退化到用户 ~/.pi)。
func DefaultCollabAgentDir() string {
	if p := configuredDataDir.Load(); p != nil && strings.TrimSpace(*p) != "" {
		return filepath.Join(strings.TrimSpace(*p), "pi-agent")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".pocketagentd", "pi-agent")
}

// ResolveCollabAgentDir 解析协作路径实际使用的 agent 目录:
// 用户 env 已设 PI_CODING_AGENT_DIR → 尊重(隔离原则:不覆写用户显式配置,
// 初始化与注入都落在用户的目录上);否则用 daemon 自有默认目录。
func ResolveCollabAgentDir() string {
	if p := strings.TrimSpace(os.Getenv(agentDirEnv)); p != "" {
		return p
	}
	return DefaultCollabAgentDir()
}

// ---- env 注入(Start 的拉起路径调用)----

// CollabEnvInjection 返回需要追加给 pi 子进程的 "PI_CODING_AGENT_DIR=<目录>" 行;
// 不满足门控返回 ""(不注入,pi 走默认用户目录,行为与现状零差异)。门控四条:
//
//  1. 总开关关闭(默认)→ 不注入;
//  2. 用户 env 已设 PI_CODING_AGENT_DIR → 不注入(子进程经 os.Environ() 已继承;
//     再追加会造成 Windows env 块同键重复,取值未定义);
//  3. 目录为空/不存在 → 不注入(SetupCollabAgentDir 未跑过:注入一个空目录会让
//     pi 丢凭据、丢扩展,主功能直接坏 —— 目录存在是注入的硬前提);
//  4. 其余 → 注入 daemon 自有目录。
func CollabEnvInjection() string {
	return collabEnvInjection(DefaultCollabAgentDir())
}

func collabEnvInjection(dir string) string {
	if !CollabEnabled() {
		return ""
	}
	if os.Getenv(agentDirEnv) != "" {
		return ""
	}
	if dir == "" {
		return ""
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return ""
	}
	return agentDirEnv + "=" + dir
}

// ---- 一次性初始化(幂等)----

// AdapterInstalled 报告 pi-mcp-adapter 是否已装进 agentDir。两条证据任一即可,
// 与 pi install 的落盘行为一一对应(T-L2pi 探针实证):settings.json 的 packages
// 列表含 npm:pi-mcp-adapter,或 <agentdir>/npm/node_modules/pi-mcp-adapter 存在。
func AdapterInstalled(agentDir string) bool {
	if raw, err := os.ReadFile(filepath.Join(agentDir, "settings.json")); err == nil {
		var st struct {
			Packages []string `json:"packages"`
		}
		if json.Unmarshal(raw, &st) == nil {
			for _, p := range st.Packages {
				if p == piAdapterPackage {
					return true
				}
			}
		}
	}
	st, err := os.Stat(filepath.Join(agentDir, "npm", "node_modules", "pi-mcp-adapter"))
	return err == nil && st.IsDir()
}

// SetupCollabAgentDir 一次性初始化 agent 目录(幂等,可重复调用):
//
//  1. 建目录(MkdirAll,已存在无害);
//  2. adapter 已装(AdapterInstalled)→ 跳过安装直接返回 nil(不重跑 pi install,
//     不碰网络);
//  3. 未装 → 以现有 node+cli 拉起方式执行 `pi install npm:pi-mcp-adapter
//     --no-approve`(T-L2pi 探针实证的非交互旗标),目标目录经 PI_CODING_AGENT_DIR
//     隔离,耗时上界 piInstallTimeout;
//  4. 安装需网络:失败打 WARNING 并返回 error。调用方(pi 主启动路径)必须容忍
//     失败继续跑 —— 协作目录初始化失败绝不阻塞 pi 主功能,下次启动重试。
//
// 凭据(auth.json/models-store.json)【不在本函数自动复制】:涉及凭据去向,
// 必须走"用户引导确认后显式调用",见 CopyUserCredentials。
func SetupCollabAgentDir(agentDir string) error {
	if strings.TrimSpace(agentDir) == "" {
		return errors.New("agent 目录为空,无法初始化 pi 协作目录")
	}
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		return fmt.Errorf("创建 pi agent 目录失败: %w", err)
	}
	if AdapterInstalled(agentDir) {
		log.Printf("[pi-collab] pi-mcp-adapter 已装,跳过安装(幂等): %s", agentDir)
		return nil
	}
	node, cli, err := LocatePi()
	if err != nil {
		log.Printf("[pi-collab] WARNING: 无法定位 pi,跳过 adapter 安装(pi 主功能不受影响,下次重试): %v", err)
		return fmt.Errorf("定位 pi 失败: %w", err)
	}
	args := []string{cli, "install", piAdapterPackage, "--no-approve"}
	ctx, cancel := context.WithTimeout(context.Background(), piInstallTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, node, args...)
	cmd.Dir = agentDir
	// env 隔离:透传当前环境,PI_CODING_AGENT_DIR 去重后覆盖(Windows env 块对
	// 同键重复取值未定义,必须先剔后追,dshprobe buildEnv 同款)。
	cmd.Env = envWithKey(agentDirEnv, agentDir)
	hidewin.Apply(cmd) // 控制台程序:不隐藏会在用户桌面闪黑窗(与 Start/Version 同款)
	log.Printf("[pi-collab] 安装 pi-mcp-adapter: node=%s args=%v dir=%s", node, args, agentDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		tail := string(out)
		if len(tail) > 800 {
			tail = tail[len(tail)-800:] // 只留尾段:npm 的报错在最后
		}
		log.Printf("[pi-collab] WARNING: pi install 失败(pi 主功能不受影响,下次重试): %v; 输出尾段: %s", err, tail)
		return fmt.Errorf("pi install %s 失败: %w", piAdapterPackage, err)
	}
	if !AdapterInstalled(agentDir) {
		err := fmt.Errorf("pi install 退出码 0 但未检出安装痕迹(settings.json/packages 或 npm/node_modules): %s", agentDir)
		log.Printf("[pi-collab] WARNING: %v(下次重试)", err)
		return err
	}
	log.Printf("[pi-collab] pi-mcp-adapter 安装完成: %s", agentDir)
	return nil
}

// envWithKey 组装子进程环境:透传当前环境,剔除同名键(大小写不敏感,Windows
// env 名不分大小写)后追加覆盖。
func envWithKey(key, value string) []string {
	out := make([]string, 0, len(os.Environ())+1)
	for _, e := range os.Environ() {
		if k, _, ok := strings.Cut(e, "="); ok && strings.EqualFold(k, key) {
			continue
		}
		out = append(out, e)
	}
	return append(out, key+"="+value)
}

// ---- 凭据复制(留接口,不自动)----

// CredentialWhitelist 允许复制进 agent 目录的凭据文件名(白名单,防任意文件写)。
var CredentialWhitelist = []string{"auth.json", "models-store.json"}

// CopyUserCredentials 把用户确认后的 pi 凭据文件复制进 agentDir(逐一覆盖写,0600)。
//
// 【故意不自动】auth.json/models-store.json 是登录凭据,复制 = 把用户身份落进
// daemon 自有目录 —— 涉及凭据去向,必须由上层在"用户引导确认后"显式调用;
// daemon 生产路径(含 SetupCollabAgentDir)当前【不调用】本函数,仅留接口。
//
// sources:目标文件名(必须在 CredentialWhitelist 内)→ 源文件绝对路径。
func CopyUserCredentials(agentDir string, sources map[string]string) error {
	if strings.TrimSpace(agentDir) == "" {
		return errors.New("agent 目录为空,无法复制凭据")
	}
	allowed := make(map[string]bool, len(CredentialWhitelist))
	for _, n := range CredentialWhitelist {
		allowed[n] = true
	}
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		return fmt.Errorf("创建 pi agent 目录失败: %w", err)
	}
	var errs []error
	for name, src := range sources {
		if !allowed[name] {
			errs = append(errs, fmt.Errorf("凭据文件名 %q 不在白名单 %v 内,拒绝复制", name, CredentialWhitelist))
			continue
		}
		data, err := os.ReadFile(src)
		if err != nil {
			errs = append(errs, fmt.Errorf("读取凭据源 %s 失败: %w", src, err))
			continue
		}
		dst := filepath.Join(agentDir, name)
		if err := os.WriteFile(dst, data, 0o600); err != nil {
			errs = append(errs, fmt.Errorf("写凭据 %s 失败: %w", dst, err))
			continue
		}
		log.Printf("[pi-collab] 凭据已复制(用户确认后): %s -> %s", src, dst)
	}
	return errors.Join(errs...)
}

// ---- mcp.json 写入器 ----

// WriteCollabMcpConfig 写 <agentdir>/mcp.json 的
// mcpServers["pocketagent-collab"] 条目,格式与探针实证一致:
//
//	{"mcpServers":{"pocketagent-collab":{"command":"<绝对路径>","args":[...]}}}
//
// 语义:
//   - 只增改自己的一个条目,其余内容原样保留(整文档按 RawMessage 读写,未知
//     顶层键、他人条目的未知字段如 env/cwd 一律不丢 —— 能透传就透传);
//   - 原子写:同目录临时文件 + os.Rename(Windows 下 Go 的 rename 覆盖已存在
//     目标;最坏情况留下 .tmp 残片,mcp.json 本体要么旧要么新,不会半截);
//   - 已有 mcp.json 解析失败 → 拒绝覆盖返回 error(宁可让调用方处理,不毁数据);
//   - serverCommand 转绝对路径后落盘;serverArgs 由调用方注入(collabmcp 服务器
//     落地后传 "<daemon exe> mcp --stdio" 之类,当前无生产调用方)。
func WriteCollabMcpConfig(agentDir, serverCommand string, serverArgs []string) error {
	return WriteCollabMcpConfigWithEnv(agentDir, serverCommand, serverArgs, nil)
}

// WriteCollabMcpConfigWithEnv 是 WriteCollabMcpConfig 的 env 扩展版(T-L2fix,
// 只增不改:既有函数行为与落盘格式零变化 —— env 为空时输出逐字节同旧)。
// env 非空时写入条目的 "env" 对象(MCP stdio 服务器配置的标准字段,由
// pi-mcp-adapter 在拉起服务器子进程时注入其环境),供 -collab-mcp 代理进程
// 读取 POCKETAGENT_TOKEN 回环鉴权:
//
//	{"mcpServers":{"pocketagent-collab":{"args":[…],"command":"…",
//	                 "env":{"POCKETAGENT_TOKEN":"<legacyTokenHash>"}}}}
//
// 其余语义(条目替换粒度、未知字段透传、原子写、解析失败拒绝覆盖)与
// WriteCollabMcpConfig 完全一致。
func WriteCollabMcpConfigWithEnv(agentDir, serverCommand string, serverArgs []string, env map[string]string) error {
	if strings.TrimSpace(agentDir) == "" {
		return errors.New("agent 目录为空,无法写 mcp.json")
	}
	if strings.TrimSpace(serverCommand) == "" {
		return errors.New("serverCommand 为空,拒绝写入占位 MCP 服务器")
	}
	abs, err := filepath.Abs(serverCommand)
	if err != nil {
		return fmt.Errorf("serverCommand 转绝对路径失败: %w", err)
	}
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		return fmt.Errorf("创建 agent 目录失败: %w", err)
	}
	path := filepath.Join(agentDir, mcpConfigFile)

	doc := map[string]json.RawMessage{}
	if raw, err := os.ReadFile(path); err == nil {
		if len(bytes.TrimSpace(raw)) > 0 {
			if err := json.Unmarshal(raw, &doc); err != nil {
				return fmt.Errorf("已有 %s 无法解析,拒绝覆盖(请人工处理): %w", path, err)
			}
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("读取 %s 失败: %w", path, err)
	}
	servers := map[string]json.RawMessage{}
	if raw, ok := doc["mcpServers"]; ok {
		if err := json.Unmarshal(raw, &servers); err != nil {
			return fmt.Errorf("%s 的 mcpServers 不是对象,拒绝覆盖: %w", path, err)
		}
	}
	entryBody := map[string]any{"command": abs, "args": serverArgs}
	if len(env) > 0 {
		entryBody["env"] = env
	}
	entry, err := json.Marshal(entryBody)
	if err != nil {
		return fmt.Errorf("序列化 %s 条目失败: %w", CollabMcpServerName, err)
	}
	servers[CollabMcpServerName] = entry
	if doc["mcpServers"], err = json.Marshal(servers); err != nil {
		return fmt.Errorf("序列化 mcpServers 失败: %w", err)
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 mcp.json 失败: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(agentDir, mcpConfigFile+"-*.tmp")
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("写临时文件失败: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("关闭临时文件失败: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("原子替换 %s 失败: %w", path, err)
	}
	return nil
}
