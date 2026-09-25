// guard.go —— Pi 守护扩展:危险命令强制审批的"规则表 + 注入"。
//
// 【为什么必须在扩展里拦】Pi 没有内建审批(官方 security.md 明说),daemon 收到
// tool_execution_start 时工具**已经开始执行**了 —— 事件流里没有"执行前阻塞"点。
// 唯一的阻塞点是扩展的 tool_call 钩子:返回 {block:true} 工具就不会执行。
// 所以 daemon 侧维护规则表(正则),渲染成 TS 扩展文件,用 -e 注入每个子进程;
// 命中规则的调用弹 ctx.ui.confirm → extension_ui_request → 转手机审批卡,
// 手机拒绝 → block。命中规则 = 强制审批,绝不能"静默放行"。
package piapp

import (
	"fmt"
	"strings"
)

// GuardRule 一条危险命令规则:Pattern 是正则源码,new RegExp(s, "i") 在 TS 侧还原。
// 【转义链约定(2026-09-19 实测锚定)】raw string 里写单反斜杠 = 正则转义符
// (如 \s、\b);要匹配字面反斜杠(Windows 路径)写两个(如 C:\\Windows)。
// 链条:Pattern 值 --%q--> TS 源码字面量 --TS 解析--> 字符串值(= Pattern 值)
// --new RegExp--> 正则。绝不能用 (?i) 内联标志:那是 Go regexp 专有,JS 直接
// Invalid group 抛异常 → 扩展加载失败 → pi 子进程退出(真机 EOF 崩溃实锤)。
// 大小写不敏感统一由渲染端的 "i" 标志提供(对所有规则无副作用)。
type GuardRule struct {
	Pattern string
	Desc    string
}

// GuardRules 危险命令规则表(P1 最小集,简单正则即可 —— 调研 §10):
// rm -rf / git push --force / 写系统目录 / Windows 递归删除 / 提权 / 磁盘分区。
var GuardRules = []GuardRule{
	{Pattern: `rm\s+(-[a-zA-Z]*[rf][a-zA-Z]*\s+)+`, Desc: "递归/强制删除(rm -r/-f)"},
	{Pattern: `git\s+push\s+.*(--force|-f)\b`, Desc: "强制推送远端分支"},
	{Pattern: `(C:\\Windows|C:\\Program Files|/etc/|/usr/|/bin/|/sbin/|/boot/)`, Desc: "写系统目录"},
	{Pattern: `\b(format|diskpart|bcdedit|reg\s+add|mkfs)\b`, Desc: "系统级修改"},
	{Pattern: `\b(del|rd|rmdir)\s+/[sq]`, Desc: "Windows 递归删除"},
	{Pattern: `(^|\s)sudo\s`, Desc: "提权执行"},
}

// RenderGuardExtension 把规则表渲染成 TS 扩展源码。
// 规则以字符串数组进 TS,new RegExp 还原 —— 避免在 Go 里嵌 TS 字面量的转义地狱。
//
// 【2026-09-19 修 EOF 崩溃】desc 原来用 Go 的 %#v 渲染,生成 `[]string{...}` ——
// 那是 Go 专有语法,TS 解析直接 ParseError(pocketagent-pi-guard.ts:13:17
// "Missing semicolon"),pi 加载扩展失败即退出,生产端表现就是手机上的
// "获取 Pi 会话状态失败: get_state: pi 子进程已退出: EOF"(stderr 日志化后定位)。
// 改成与 rules 同款的 %q 逐行数组(纯 TS),并保持与 rules 同序。
func RenderGuardExtension(rules []GuardRule) string {
	var srcs, descs []string
	for _, r := range rules {
		srcs = append(srcs, fmt.Sprintf("  %q,", r.Pattern))
		descs = append(descs, fmt.Sprintf("  %q,", r.Desc))
	}
	return `// PocketAgent 守护扩展(daemon 自动注入,勿手改):
// 对命中危险规则的 tool_call 弹 confirm(无 timeout —— 超时由 daemon 兜底回 cancelled)。
// 用户拒绝 → {block:true},工具不执行。
export default function (pi: any) {
  const rules = [
` + strings.Join(srcs, "\n") + `
  ].map((s) => new RegExp(s, "i"));
  const desc = [
` + strings.Join(descs, "\n") + `
  ];
  pi.on("tool_call", async (event: any, ctx: any) => {
    let hay = "";
    try { hay = JSON.stringify(event.input ?? {}); } catch { return undefined; }
    for (let i = 0; i < rules.length; i++) {
      if (rules[i].test(hay)) {
        const label = (desc[i] || "危险操作") + ":请确认是否允许该操作";
        const ok = await ctx.ui.confirm("PocketAgent 审批", label);
        if (!ok) return { block: true, reason: "被用户拒绝(" + (desc[i] || "危险操作") + ")" };
      }
    }
    return undefined;
  });
}
`
}
