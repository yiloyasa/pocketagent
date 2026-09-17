package dev.pocketagent.app.ui

import pocketagent.v1.RateLimitWindow

/**
 * 用量/限流的纯格式化函数。
 *
 * 【为什么单独放一个文件】这些是把 codex 的原始数值变成人话的地方,规则不多但
 * 极易写错(过期时间算成负数、分钟数写成 "300 分钟"、窗口长度未知时不回退),
 * 而且错了之后在真机上只是一句别扭的文案,不会报错。放成纯函数就能在单测里钉住。
 */

/**
 * 限流窗口长度的中文短标签。
 *
 * [fallback] 用于 codex 没给窗口长度时 —— 此时宁可显示我们约定俗成的档位名,
 * 也不要显示 "0 分钟"(那会让人以为窗口是零长度)。
 */
fun rateLimitWindowLabel(minutes: Long, fallback: String): String = when {
    minutes <= 0 -> fallback
    minutes % 1440L == 0L -> "${minutes / 1440} 天"
    minutes % 60L == 0L -> "${minutes / 60} 小时"
    else -> "$minutes 分钟"
}

/**
 * 距离限流窗口重置还有多久。
 *
 * 【为什么要把负数夹住】codex 给的重置时间可能已经过期(我们读到的是缓存值,
 * 或者读的那一刻刚好跨过重置点),直接相减会得到 "-3 小时后重置" 这种一眼像
 * bug 的文案。归一成"即将重置"既真实又不吓人。
 *
 * 返回空串表示这项数据缺失,调用方应当**整段不显示**,而不是显示一个空壳。
 */
fun resetCountdownLabel(resetsAtUnixS: Long, nowMs: Long): String {
    if (resetsAtUnixS <= 0L) return ""
    val remainMinutes = (resetsAtUnixS * 1000L - nowMs) / 60_000L
    return when {
        remainMinutes <= 0L -> "即将重置"
        remainMinutes < 60L -> "$remainMinutes 分钟后重置"
        remainMinutes < 1440L -> "${remainMinutes / 60} 小时后重置"
        else -> "${remainMinutes / 1440} 天后重置"
    }
}

/** 已用百分比:codex 给的越界值(负数 / >100)一律夹到 0..100,否则进度条会画出格。 */
fun usedPercentClamped(window: RateLimitWindow): Int = window.usedPercent.coerceIn(0, 100)

/**
 * 用量条的颜色档位。
 *
 * 【为什么不用单一颜色 + 百分比文字】额度是"能不能干活"的前置条件,扫一眼颜色
 * 就能判断要不要现在发任务,比读数字快得多。阈值取 80/95:codex 侧接近上限时
 * 请求会被降级或拒绝,80% 开始提醒是留出反应时间。
 */
fun usageLevelColorPercent(percent: Int): Int = when {
    percent >= 95 -> 2 // 危险
    percent >= 80 -> 1 // 注意
    else -> 0 // 正常
}
