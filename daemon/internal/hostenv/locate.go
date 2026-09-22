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
// npm 在 Windows 上的默认全局前缀就是 %APPDATA%\npm；部分安装方式落在
// %LOCALAPPDATA%\npm 或 node 自带的 node_modules。三种都探，因为"用户是怎么装的"
// 我们无从预知 —— 但每一种都是该用户机器上的真实位置，而非某台开发机的目录。
func NpmGlobalRoots() []string {
	var out []string
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
