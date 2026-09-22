// Package hostenv 派生「当前用户机器上的工具位置」。
//
// 【为什么必须有这一层】daemon 与 paguard 都要拉起外部工具（node / pi / dsh /
// codex），而它们的安装位置随用户与机器而变。历史实现把开发机的绝对路径写成
// 常量（C:\Users\<某个用户名>\... 与 G:\Claude Code\...），带来两个后果：
//
//   - 换一台机器必然不命中，且错误消息里会把开发机路径原样吐给用户；
//   - 路径作为字符串常量被编译进 pocketagentd.exe / paguard.exe（Go 的
//     -trimpath 只剥源码路径，不剥字符串常量），随发布包分发出去 —— 规格 §2
//     明令发布包不得包含开发者电脑路径。
//
// 本包把「位置」改成按用户环境派生：随包分发目录 → 用户/机器级安装目录 → PATH。
// 全都探测不到时返回空串，由调用方报一句不含任何开发机路径的人话错误 ——
// 「找不到」是可解释的，「回落到一个只在我这台机器上存在的路径」不可解释。
//
// 【为什么不做成各包私有】piapp 与 dshapp 的探测链几乎一致（都要 node），
// 分开写迟早漂移成两套口径 —— 位置探测一旦漂移，现场看到的就是"pi 能找到、
// dsh 找不到"这类无法解释的现象。
package hostenv

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ExecutableRoots 返回本进程可执行文件所在目录及其相邻子目录。
//
// 用于探测「随包分发」的工具（当前分发包不带 node，保留探测面以便将来自带运行时）。
// 这是唯一不受用户环境影响、且属于"我们自己安装的东西"的位置，因此排在最前。
func ExecutableRoots() []string {
	exe, err := os.Executable()
	if err != nil {
		return nil
	}
	dir := filepath.Dir(exe)
	return []string{dir, filepath.Join(dir, "bin"), filepath.Join(dir, "runtime")}
}

// NodeCandidates 返回 node.exe 在「当前用户机器上」的候选位置，按可信度排序。
//
// 不含环境变量覆盖（PI_NODE / DSH_NODE 之类由调用方按各自变量名处理，因为
// 变量名是各 provider 的口径）。候选全部由环境变量派生，因此在任何用户机器上
// 都指向该用户自己的等价安装位置，而不是某台开发机的绝对路径。
func NodeCandidates() []string {
	var out []string
	// 随包分发的运行时优先：用户没装 node 时这是唯一出路。
	for _, root := range ExecutableRoots() {
		out = append(out, filepath.Join(root, "node.exe"))
	}
	// npm 官方 Windows 安装器的用户级默认落点（对应历史上的
	// C:\Users\<user>\AppData\Local\Programs\nodejs\node.exe）。
	if la := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); la != "" {
		out = append(out, filepath.Join(la, "Programs", "nodejs", "node.exe"))
	}
	if pf := strings.TrimSpace(os.Getenv("ProgramFiles")); pf != "" {
		out = append(out, filepath.Join(pf, "nodejs", "node.exe"))
	}
	if pf := strings.TrimSpace(os.Getenv("ProgramFiles(x86)")); pf != "" {
		out = append(out, filepath.Join(pf, "nodejs", "node.exe"))
	}
	return dedupe(out)
}

// NpmGlobalRoots 返回 npm 全局安装根（其下按 <scope>\<package> 组织）。
//
// 【为什么第一顺位不是默认位置】npm 的全局 prefix 是可配置的：用户在 .npmrc 里写
// `prefix=`，或用 NPM_CONFIG_PREFIX 环境变量。开发机上它就被配到了 G:\Claude Code
// —— pi / dsh 全在那个 prefix 下的 node_modules 里，只探 %APPDATA%\npm 会变成
// "找不到明明装着的东西"。这与硬编码开发机路径是同一类错误：假设位置，而不是读事实。
// 所以顺序是：用户配置的事实 → npm 默认位置。
func NpmGlobalRoots() []string {
	var out []string
	// 1. 用户实际配置的 prefix（最高优先）。
	for _, p := range npmConfiguredPrefixes() {
		out = append(out, filepath.Join(p, "node_modules"))
	}
	// 2. npm 默认位置：Windows 上默认全局前缀是 %APPDATA%\npm；部分安装方式落在
	// %LOCALAPPDATA%\npm 或 node 自带的 node_modules。
	if ap := strings.TrimSpace(os.Getenv("APPDATA")); ap != "" {
		out = append(out, filepath.Join(ap, "npm", "node_modules"))
	}
	if la := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); la != "" {
		out = append(out, filepath.Join(la, "npm", "node_modules"))
	}
	if pf := strings.TrimSpace(os.Getenv("ProgramFiles")); pf != "" {
		out = append(out, filepath.Join(pf, "nodejs", "node_modules"))
	}
	return dedupe(out)
}

// npmConfiguredPrefixes 返回 npm 全局 prefix 的**配置值**（不是默认位置）。
// 取值顺序与 npm 自身一致：环境变量先于用户 .npmrc。
func npmConfiguredPrefixes() []string {
	var out []string
	if p := strings.TrimSpace(os.Getenv("NPM_CONFIG_PREFIX")); p != "" {
		out = append(out, p)
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		out = append(out, parseNpmrcPrefix(filepath.Join(home, ".npmrc"))...)
	}
	return out
}

// parseNpmrcPrefix 从 npmrc 文本里取出 prefix 配置值。
//
// 【为什么自己解析而不调 `npm config get prefix`】daemon 由计划任务经 WMI 派生，
// 环境变量与 PATH 都不保证齐全（paguard 的既有教训），再拉一个 npm 子进程既慢、
// 又多一个失败点；而 .npmrc 只是个几行的文本文件，读它与读配置本身等价。
// 缺文件、无该键都安静返回空 —— 没有 .npmrc 是绝大多数机器的正常状态。
func parseNpmrcPrefix(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []string
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
		// 跳过空行/注释/段头：段内的同名键不属于全局配置。
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "[") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok || !strings.EqualFold(strings.TrimSpace(key), "prefix") {
			continue
		}
		if val = strings.Trim(strings.TrimSpace(val), `"'`); val != "" {
			out = append(out, val)
		}
	}
	return out
}

// NpmGlobalPackage 把 npm 全局包内的相对路径展开成候选位置。
// rel 用正斜杠书写（如 "@scope/name/dist/cli.js"），内部按段拼接以适配 Windows。
func NpmGlobalPackage(rel string) []string {
	parts := strings.Split(strings.Trim(rel, "/"), "/")
	var out []string
	for _, root := range NpmGlobalRoots() {
		out = append(out, filepath.Join(append([]string{root}, parts...)...))
	}
	return out
}

// PackageCandidates 把一个 npm 包内入口的相对路径（如
// "@scope/name/dist/cli.js"）展开成候选位置：随包分发目录内的 node_modules → npm 全局根。
//
// 供 pi / dsh 这类「本体是 npm 包里的一个 js 入口」的工具使用 —— 它们没有
// 独立的 exe，位置只能靠包的落点推算。
func PackageCandidates(rel string) []string {
	parts := strings.Split(strings.Trim(rel, "/"), "/")
	var out []string
	for _, root := range ExecutableRoots() {
		out = append(out, filepath.Join(append(append([]string{root}, "node_modules"), parts...)...))
	}
	out = append(out, NpmGlobalPackage(rel)...)
	return dedupe(out)
}

// FirstFile 返回候选列表中第一个「存在的文件」；都不存在返回空串。
//
// 目录不算命中：把目录当可执行文件交给 CreateProcess 得到的是费解的系统错误，
// 不如让调用方报"未找到"，现场一眼能懂。
func FirstFile(cands []string) string {
	for _, p := range cands {
		if p == "" {
			continue
		}
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

// FindNode 按「派生候选 → PATH」返回可用的 node.exe；找不到返回空串。
//
// 【为什么不回落任何绝对路径】回落写死的路径在别的机器上必然失效，而且会把
// 那条路径带进错误消息。返回空串让调用方报「未找到 node（可用 XXX 指定路径）」——
// 用户看到的是自己该做什么，不是开发机的目录结构。
func FindNode() string {
	if p := FirstFile(NodeCandidates()); p != "" {
		return p
	}
	if p, err := exec.LookPath("node"); err == nil {
		return p
	}
	return ""
}

// dedupe 去重并丢弃空串，保持原有顺序（顺序本身是可信度排序的一部分）。
func dedupe(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, p := range in {
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}
