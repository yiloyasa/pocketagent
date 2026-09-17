package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 上下文窗口解析优先级(pickContextWindow)+ 它如何决定占用环是否显示真实占比。
 *
 * 【为什么必须钉】真机上"选中模型声明窗口=0、codex 又不在用量帧里带上报窗口",
 * 于是窗口被解析成 0 → `windowIsFallback=true` → 环内恒显示破折号(用户要的是实际占比)。
 * 本组用例锁死"用 agent 默认模型的窗口兜底",保证环内出现数字。
 */
class ContextWindowResolveTest {

    /** 与调用点一致:窗口解析不出(<=0)即视为兜底 / 未知。 */
    private fun fallback(window: Long): Boolean = window <= 0

    @Test
    fun live_window_wins() {
        val w = pickContextWindow(live = 272_000L, modelDeclared = 0, agentDefaultDeclared = 0)
        assertEquals(272_000L, w)
        assertFalse(fallback(w))
    }

    @Test
    fun model_declared_beats_agent_default() {
        val w = pickContextWindow(live = 0L, modelDeclared = 128_000, agentDefaultDeclared = 272_000)
        assertEquals(128_000L, w)
        assertFalse(fallback(w))
    }

    /** 本轮回归守门:真机场景 —— 选中模型与上报窗口都没给值,靠 agent 默认窗口兜底。 */
    @Test
    fun real_device_scenario_falls_back_to_agent_default() {
        val w = pickContextWindow(live = 0L, modelDeclared = 0, agentDefaultDeclared = 272_000)
        assertEquals(272_000L, w)
        assertFalse("窗口必须算作已知,否则环内不会显示数字", fallback(w))
    }

    @Test
    fun all_zero_means_unknown() {
        val w = pickContextWindow(live = 0L, modelDeclared = 0, agentDefaultDeclared = 0)
        assertEquals(0L, w)
        assertTrue(fallback(w))
    }

    @Test
    fun negatives_and_zeros_never_misjudge() {
        assertEquals(0L, pickContextWindow(live = -1L, modelDeclared = 0, agentDefaultDeclared = 0))
        assertEquals(0L, pickContextWindow(live = 0L, modelDeclared = -5, agentDefaultDeclared = 0))
        assertEquals(0L, pickContextWindow(live = 0L, modelDeclared = 0, agentDefaultDeclared = -9))
        assertTrue(fallback(pickContextWindow(live = -1L, modelDeclared = -1, agentDefaultDeclared = -1)))
    }

    /**
     * 集成式:真机那条链(0, 0, 272000)解析出的窗口喂给 ringPercentLabel,
     * 环内应得到 "3%"(7520 / 272000),证明环内会出现真实数字而不是破折号。
     */
    @Test
    fun resolved_window_makes_the_ring_show_a_real_percent() {
        val window = pickContextWindow(live = 0L, modelDeclared = 0, agentDefaultDeclared = 272_000)
        assertFalse(fallback(window))
        val ratio = 7_520f / window.toFloat()
        assertEquals("3%", ringPercentLabel(ratio, windowKnown = true))
    }
}
