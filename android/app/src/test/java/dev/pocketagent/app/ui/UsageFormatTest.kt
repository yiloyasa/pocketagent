package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Test
import pocketagent.v1.RateLimitWindow

/**
 * 用量/限流格式化规则。
 *
 * 这些函数出错的表现是"文案别扭"而不是"报错",所以只能靠单测钉住 ——
 * 尤其是过期时间被算成负数、窗口长度未知时不回退这两类。
 */
class UsageFormatTest {

    @Test
    fun window_label_prefers_real_duration_and_falls_back_when_unknown() {
        assertEquals("5 小时", rateLimitWindowLabel(300, "5 小时"))
        assertEquals("1 天", rateLimitWindowLabel(1440, "每周"))
        assertEquals("7 天", rateLimitWindowLabel(10080, "每周"))
        assertEquals("45 分钟", rateLimitWindowLabel(45, "5 小时"))
        // codex 没给窗口长度:回退到档位名,而不是显示 "0 分钟"
        assertEquals("每周", rateLimitWindowLabel(0, "每周"))
        assertEquals("5 小时", rateLimitWindowLabel(-1, "5 小时"))
    }

    @Test
    fun reset_countdown_is_human_readable_across_scales() {
        val now = 1_700_000_000_000L
        fun at(secondsFromNow: Long) = resetCountdownLabel(now / 1000 + secondsFromNow, now)

        assertEquals("30 分钟后重置", at(30 * 60))
        assertEquals("2 小时后重置", at(2 * 3600))
        assertEquals("3 天后重置", at(3 * 86400))
    }

    /**
     * 关键回归:重置时间已经过期时必须归一成"即将重置"。
     * 直接相减会得到 "-3 小时后重置" —— 一眼像 bug,而且会让人怀疑整个数据是假的。
     */
    @Test
    fun already_passed_reset_time_does_not_render_a_negative_countdown() {
        val now = 1_700_000_000_000L
        assertEquals("即将重置", resetCountdownLabel(now / 1000 - 3600, now))
    }

    @Test
    fun missing_reset_time_renders_nothing_so_the_row_can_hide_it() {
        assertEquals("", resetCountdownLabel(0, 1_700_000_000_000L))
        assertEquals("", resetCountdownLabel(-5, 1_700_000_000_000L))
    }

    @Test
    fun used_percent_is_clamped_so_the_bar_never_draws_outside_itself() {
        assertEquals(0, usedPercentClamped(RateLimitWindow.newBuilder().setUsedPercent(-20).build()))
        assertEquals(42, usedPercentClamped(RateLimitWindow.newBuilder().setUsedPercent(42).build()))
        assertEquals(100, usedPercentClamped(RateLimitWindow.newBuilder().setUsedPercent(180).build()))
    }

    @Test
    fun usage_level_thresholds_are_pinned() {
        assertEquals(0, usageLevelColorPercent(0))
        assertEquals(0, usageLevelColorPercent(79))
        assertEquals(1, usageLevelColorPercent(80))
        assertEquals(1, usageLevelColorPercent(94))
        assertEquals(2, usageLevelColorPercent(95))
        assertEquals(2, usageLevelColorPercent(100))
    }
}
