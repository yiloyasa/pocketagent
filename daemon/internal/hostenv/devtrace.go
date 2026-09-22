package hostenv

import (
	"bytes"
	"strings"
)

// developerTraceNeedles 是「开发机专有痕迹」的关键字表。
//
// 每一项都是**我这台开发机专有**、在任何用户机器上都不该出现的东西：
//   - "yanzi"            —— 开发机的 Windows 用户名（出现在 C:\Users\yanzi\...）
//   - `G:\项目1`          —— 本仓库在开发机上的绝对路径
//   - "Claude Code"      —— 开发机上第三方工具的自选安装目录名
//   - "100.92.193.109"   —— 开发机的 Tailscale 地址
//
// 【为什么这张表要有唯一实现】发布闸门（dist/windows/build.ps1 的字节级扫描）、
// Go 侧的源码回归钉子、以及将来可能加进 pa_audit 的规则，用的必须是同一份关键字。
// 各写各的迟早漂移成"一处拦住、一处放过" —— 而漏放的那一处就是把路径送进
// 发布包的那一处。
//
// 注意 "yanzi" 是小写匹配；调用方按需 ToLower 后再比。
func developerTraceNeedles() []string {
	return []string{"yanzi", `G:\项目1`, "Claude Code", "100.92.193.109"}
}

// DeveloperTracesIn 报告一段字节里命中的开发机痕迹（去重、按表内顺序）。
//
// 【为什么做双解码】要扫的对象既可能是 Go 源码（UTF-8），也可能是编译产物
// （exe 里字符串常量按原始字节存放，中文以 UTF-8 存储但整段可能不是合法 UTF-8）。
// Latin1 逐字节映射 + UTF-8 各解一遍取并集，才不会因为一处非法字节序列漏掉整条
// 命中 —— 漏报在这里的代价是"带开发机路径的包被发出去"。
func DeveloperTracesIn(data []byte) []string {
	lower := bytes.ToLower(data)
	var hits []string
	seen := make(map[string]bool, len(developerTraceNeedles()))
	for _, needle := range developerTraceNeedles() {
		if seen[needle] {
			continue
		}
		if bytes.Contains(lower, []byte(strings.ToLower(needle))) {
			seen[needle] = true
			hits = append(hits, needle)
		}
	}
	return hits
}
