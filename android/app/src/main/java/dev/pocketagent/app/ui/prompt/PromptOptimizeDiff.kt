package dev.pocketagent.app.ui.prompt

// PromptOptimizeDiff.kt —— 优化预览的轻量行级差异(P4 Task 2,纯函数,无
// Android 依赖,可 JVM 单测)。
//
// 【为什么不引第三方库】diff 工具库会拉进新依赖;预览场景只需要"哪些行变了"
// 的高亮,前缀/后缀剥离 + 小规模 LCS 足够。LCS 的 O(n×m) DP 被 [PROMPT_DIFF_DP_CAP]
// 钳死(400×400 ≈ 16 万格,微秒级),调用方再以 remember(original, candidate)
// 缓存,组合内不阻塞主线程。

/** 差异行类型:原/新文本按行对齐后的归类(供预览高亮)。 */
internal enum class PromptDiffKind { SAME, ADDED, REMOVED }

/** 一行差异:[text] 不含行尾换行符。 */
internal data class PromptDiffLine(val kind: PromptDiffKind, val text: String)

/**
 * 单侧进入 LCS 的行数上限:中段超限(极端长文本)直接退化为"整段删 + 整段加",
 * 不做逐行对齐 —— 正确性不丢(删除/新增并排呈现),只牺牲"行内对齐"的精细度。
 */
internal const val PROMPT_DIFF_DP_CAP = 400

/**
 * 逐行差异(old → new):先剥公共前缀/后缀,中段用 LCS 对齐,按原顺序输出
 * SAME/REMOVED/ADDED 行。
 *
 * 【边界口径】
 *  - 两参一致 → 全 SAME;
 *  - **任一侧为空串 = 纯新增/纯删除**(整体成行,不经前缀剥离与 LCS);
 *    双空串 = 无差异(空列表)。注意 Kotlin 的 `"".split('\n')` 返回 `[""]`
 *    (一个空行),若把空串直接拆行参与对齐,会凭空多出一条幻影行 —— 所以
 *    空串边界必须在拆行前短路(真机门禁实锤过的坑);
 *  - "\r\n" 与 "\n" 都按行分隔,行内容不带 "\r";
 *  - LCS 平局(删/加等价)时先输出 REMOVED 再输出 ADDED,输出顺序确定可钉。
 */
internal fun promptLineDiff(oldText: String, newText: String): List<PromptDiffLine> {
    // 空串边界短路:不走 split(空串会被拆成一个幻影空行)、不走 LCS
    if (oldText.isEmpty() && newText.isEmpty()) return emptyList()
    if (oldText.isEmpty()) {
        return newText.split('\n').map { PromptDiffLine(PromptDiffKind.ADDED, it.removeSuffix("\r")) }
    }
    if (newText.isEmpty()) {
        return oldText.split('\n').map { PromptDiffLine(PromptDiffKind.REMOVED, it.removeSuffix("\r")) }
    }
    val old = oldText.split('\n').map { it.removeSuffix("\r") }
    val new = newText.split('\n').map { it.removeSuffix("\r") }
    var start = 0
    while (start < old.size && start < new.size && old[start] == new[start]) start++
    var endOld = old.size
    var endNew = new.size
    while (endOld > start && endNew > start && old[endOld - 1] == new[endNew - 1]) {
        endOld--
        endNew--
    }
    val midOld = old.subList(start, endOld)
    val midNew = new.subList(start, endNew)
    val out = mutableListOf<PromptDiffLine>()
    repeat(start) { out += PromptDiffLine(PromptDiffKind.SAME, old[it]) }
    when {
        midOld.isEmpty() && midNew.isEmpty() -> Unit // 完全一致
        midOld.isEmpty() -> midNew.forEach { out += PromptDiffLine(PromptDiffKind.ADDED, it) }
        midNew.isEmpty() -> midOld.forEach { out += PromptDiffLine(PromptDiffKind.REMOVED, it) }
        midOld.size > PROMPT_DIFF_DP_CAP || midNew.size > PROMPT_DIFF_DP_CAP -> {
            // 超限退化:不跑 DP,整段删 + 整段加
            midOld.forEach { out += PromptDiffLine(PromptDiffKind.REMOVED, it) }
            midNew.forEach { out += PromptDiffLine(PromptDiffKind.ADDED, it) }
        }
        else -> {
            // dp[i][j] = midOld[i..] 与 midNew[j..] 的最长公共子序列长度
            val dp = Array(midOld.size + 1) { IntArray(midNew.size + 1) }
            for (i in midOld.indices.reversed()) {
                for (j in midNew.indices.reversed()) {
                    dp[i][j] = if (midOld[i] == midNew[j]) {
                        dp[i + 1][j + 1] + 1
                    } else {
                        maxOf(dp[i + 1][j], dp[i][j + 1])
                    }
                }
            }
            var i = 0
            var j = 0
            while (i < midOld.size && j < midNew.size) {
                when {
                    midOld[i] == midNew[j] -> {
                        out += PromptDiffLine(PromptDiffKind.SAME, midOld[i]); i++; j++
                    }
                    dp[i + 1][j] >= dp[i][j + 1] -> {
                        out += PromptDiffLine(PromptDiffKind.REMOVED, midOld[i]); i++
                    }
                    else -> {
                        out += PromptDiffLine(PromptDiffKind.ADDED, midNew[j]); j++
                    }
                }
            }
            while (i < midOld.size) { out += PromptDiffLine(PromptDiffKind.REMOVED, midOld[i]); i++ }
            while (j < midNew.size) { out += PromptDiffLine(PromptDiffKind.ADDED, midNew[j]); j++ }
        }
    }
    repeat(old.size - endOld) { out += PromptDiffLine(PromptDiffKind.SAME, old[endOld + it]) }
    return out
}

/**
 * 扩写启发式阈值(P4 Task 3):候选 trim 后长度超过原文 **2 倍**,或净增超过
 * **500 字**,任一命中即提示"请检查是否加入了原文没有的内容"。
 *
 * 【为什么是"或"】2 倍抓"短文本被翻倍"(默认/精简风格最不该发生);500 字
 * 绝对增量抓"长文本被塞段"(长原文下倍数不敏感,绝对量更接近用户对"变得
 * 太长"的感知)。
 *
 * 【裁定:扩写提示放客户端】daemon 的 OptimizePromptResponse 只有 optimized
 * 一个字段且本轮禁止改 proto;在 text 里内嵌标记行会污染"返回只含改写文本"
 * 的既有契约(文本要直接回填输入框)。故由客户端用本函数对比 original/
 * candidate 长度实现,daemon 侧只在日志记双方字节数(见 prompt_opt.go)。
 */
internal const val PROMPT_EXPANSION_RATIO = 2.0
internal const val PROMPT_EXPANSION_ABS_CHARS = 500

/** 候选相对原文是否"大幅变长"(阈值与理由见 [PROMPT_EXPANSION_RATIO] 注)。 */
internal fun promptCandidateOverExpanded(original: String, candidate: String): Boolean {
    val o = original.trim().length
    val c = candidate.trim().length
    if (o == 0) return false
    return c > o * PROMPT_EXPANSION_RATIO || c - o > PROMPT_EXPANSION_ABS_CHARS
}
