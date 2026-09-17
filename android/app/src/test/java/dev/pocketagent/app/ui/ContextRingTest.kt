package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Test
import pocketagent.v1.Usage

/**
 * 上下文占用环(输入栏那个小圆环)的两件纯逻辑:
 *   ① 环内文案怎么生成;② 文案字号怎么选。
 *
 * 【为什么这两条值得钉】环内只有 28dp —— 文案一旦偏长就会贴到环上甚至被裁掉,
 * 用户看到的是"这里好像有个数,但读不出来"。而"100%"与"9%"相差两个字符,
 * 不是靠眼睛能保证的,必须由纯函数 + 单测锁死。
 */
class ContextRingTest {

    @Test
    fun label_renders_integer_percent() {
        assertEquals("0%", ringPercentLabel(0f, true))
        assertEquals("42%", ringPercentLabel(0.423f, true))
        assertEquals("100%", ringPercentLabel(1.0f, true))
    }

    @Test
    fun label_is_never_faked_when_window_or_usage_unknown() {
        // 窗口未知 / 用量还没上报时给"—"。绝不能显示 0%(会被读成"窗口是空的")。
        assertEquals("—", ringPercentLabel(0f, false))
        assertEquals("—", ringPercentLabel(0.9f, false))
    }

    @Test
    fun label_keeps_over_budget_honest() {
        // 超出窗口(上下文被超出)时如实给出 >100%,不 clamp 成 100% 假装刚好满。
        assertEquals("150%", ringPercentLabel(1.5f, true))
    }

    @Test
    fun font_size_shrinks_for_the_longest_label() {
        // 3 字符及以内(9% / 42%)用常规字号;4 字符(100%)降一档,保证不出圈。
        assertEquals(9.5f, ringPercentFontSize("9%"), 0.001f)
        assertEquals(9.5f, ringPercentFontSize("42%"), 0.001f)
        assertEquals(7.5f, ringPercentFontSize("100%"), 0.001f)
        assertEquals(7.5f, ringPercentFontSize("999%"), 0.001f)
    }

    @Test
    fun ring_label_matches_the_usage_semantics_used_by_the_card() {
        // 环用 turn 用量、口径与卡面圆环一致:total = input + output
        // (cached ⊆ input、reasoning ⊆ output,不重复计入)。
        val turn = Usage.newBuilder()
            .setInputTokens(90_000)
            .setOutputTokens(30_000)
            .setCacheReadTokens(50_000)
            .build()
        val window = 128_000L
        val ratio = usageTotalTokens(turn).toFloat() / window.toFloat()
        assertEquals(120_000L, usageTotalTokens(turn))
        assertEquals("94%", ringPercentLabel(ratio, true))
        assertEquals(9.5f, ringPercentFontSize(ringPercentLabel(ratio, true)), 0.001f)
    }

    @Test
    fun null_usage_reads_as_zero_not_a_crash() {
        assertEquals(0L, usageTotalTokens(null))
        assertEquals("0%", ringPercentLabel(usageTotalTokens(null).toFloat() / 100f, true))
    }
}
