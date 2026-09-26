package dev.pocketagent.app.ui.component

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 语义触觉波形(纯函数)的单测(审计 P1-5)。
 *
 * 【为什么波形要抽纯函数】Android 的预置效果(EFFECT_CLICK 等)只在 API 29+ 有,
 * minSdk 26 的旧机要走 createWaveform 兜底 —— 两套路径必须长出**同一种语义**:
 * confirm=轻击一下、success=双击、warning=更重的一下。波形数据单测钉死,
 * 播放层(Android API)不测。
 */
class PaHapticsPatternTest {

    @Test
    fun `确认_轻击一下`() {
        val p = paHapticPattern(PaHapticKind.CONFIRM)
        assertEquals(2, p.size) // 前导 0 + 单脉冲
        assertEquals(0L, p.first())
        assertTrue(p[1] in 1..40) // 轻击:短脉冲
    }

    @Test
    fun `完成_双击模式`() {
        val p = paHapticPattern(PaHapticKind.SUCCESS)
        assertEquals(4, p.size) // 前导 0 + 脉冲 + 间隔 + 脉冲
        assertTrue(p[1] > 0)
        assertTrue(p[2] > 0) // 两击之间有间隔
        assertTrue(p[3] > 0)
    }

    @Test
    fun `警示_比确认更重的一下`() {
        val warning = paHapticPattern(PaHapticKind.WARNING)
        val confirm = paHapticPattern(PaHapticKind.CONFIRM)
        assertEquals(2, warning.size)
        assertTrue(warning[1] > confirm[1]) // warning 的脉冲时长 > confirm(更重)
    }
}
