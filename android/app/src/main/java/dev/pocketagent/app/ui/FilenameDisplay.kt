// FilenameDisplay.kt —— 文件名的中间省略规则(纯逻辑,可单测)。
//
// 【为什么单独抽出来】文件名在窄屏被裁是反复出现的老问题,而"省略时必须保住扩展名"
// 是一条能被钉死的**规则**:`报告-2026-09-17-final-v3.docx` 绝不能显示成
// `报告-2026-09-17-fin…`(用户靠扩展名判断这是什么文件)。规则一旦写进 Composable
// 就只能靠肉眼在各种窄屏上验,所以抽成纯函数 + 单测。
package dev.pocketagent.app.ui

/**
 * 中间省略:头部保留 + `…` + 尾部扩展名。
 *
 * 当 [name] 长度不超过 [max] 时原样返回;否则尽量"保头留尾"(保留扩展名)。
 * 若 [max] 太小、连扩展名都放不下,则退化为"头部 + `…`"。
 *
 * @param max 目标显示的最大字符数(含省略号与扩展名)。
 */
internal fun middleEllipsize(name: String, max: Int): String {
    if (max <= 0) return ""
    if (name.length <= max) return name
    if (max == 1) return "…"

    val suffix = extensionSuffix(name)
    val headBudget = max - 1 - suffix.length // 1 = 省略号占位
    if (suffix.isNotEmpty() && headBudget >= 1) {
        return name.substring(0, headBudget) + "…" + suffix
    }
    // 无扩展名,或 max 太小放不下"头 + … + 扩展名":退化为"头部 + …"
    return name.substring(0, max - 1) + "…"
}

/**
 * 取出应保留的"扩展名尾巴"(含点),支持 `tar.gz` 这类复合扩展名。
 *
 * 规则保守:最后一段是 1..8 位字母数字才认作扩展名;若它前面那段也是 1..4 位
 * 字母数字(如 `tar`/`vim`),一并保留。这样对 `a.txt` / `x.tar.gz` 都能给出
 * 合适的尾巴,而对 `v1.2.3` 这类"带点的普通词"也不会误判得太离谱。
 */
private fun extensionSuffix(name: String): String {
    val dot = name.lastIndexOf('.')
    if (dot <= 0 || dot == name.length - 1) return ""
    val last = name.substring(dot + 1)
    if (last.length > 8 || !last.all { it.isLetterOrDigit() }) return ""
    val prevDot = name.lastIndexOf('.', dot - 1)
    if (prevDot > 0) {
        val mid = name.substring(prevDot + 1, dot)
        if (mid.length in 1..4 && mid.all { it.isLetterOrDigit() }) {
            return name.substring(prevDot)
        }
    }
    return name.substring(dot)
}
