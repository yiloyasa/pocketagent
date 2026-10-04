package runtime

// clip.go —— 按 rune 截断的共享工具(ClipRunes/ClipRunesEllipsis)。
//
// 【为什么放 runtime】daemon 里 agent/server 两个包各有 8 处同构的
// truncate/clip 小函数(A 类硬截/B 类加省略号),审查报告
// docs/reviews/2026-10-04-full-code-review.md 第七节 util 收敛项裁定:
// 收敛到 internal/runtime —— 因为它被 agent 与 server 双向依赖,而这两个包
// 都已依赖 runtime 且 runtime 不反向依赖任何 agent/server/leaf 适配器包
// (runtime 自身只依赖 store),放这里不产生新依赖边,三方共用零环。

// ClipRunes 按 rune 硬截到前 n 个字符,超出部分直接丢弃(不加省略号)。
// n<=0 返回空串。保留 rune 边界:中文不会被拦腰截出半个字符(乱码)。
// 适用场景:预览/标题等"长度本身是定长上界"的干净截断(如 turnJournal 预览)。
func ClipRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// ClipRunesEllipsis 按 rune 截断到前 n 个字符,超出时末尾追加省略号"…"
// (省略号不计入 n,截断结果为 n+1 个 rune);未超长则原样返回。
// n<=0 返回空串 —— 历史实现里 n<=0 有"补…"(echo truncate)与"视为不设限"
// (pi_events clipRunes)两种分歧,真实调用方均传大正数,此处统一取最保守的
// 空串语义;需要"n<=0 不设限"的调用方在封装层自持守卫(见 server 包 clipRunesOr)。
func ClipRunesEllipsis(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
