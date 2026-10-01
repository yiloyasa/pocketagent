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

    @Test
    fun `全部波形形态统一_前导零与正时长_三级强度单调`() {
        // 形态红线:createWaveform 的 timings 首位必须是前导 0,其余全为正时长
        // —— 谁把前导 0 丢了或写出 0 时长脉冲,旧机兜底路径会直接抛异常
        val all = PaHapticKind.entries
        assertEquals(3, all.size) // 轻/中/重三级齐全,误删一级这里先红
        for (kind in all) {
            val p = paHapticPattern(kind)
            assertEquals("前导 0", 0L, p.first())
            assertTrue("脉冲数为奇数(前导0+成对脉冲/间隔)", p.size % 2 == 0)
            assertTrue(p.drop(1).all { it > 0 })
            assertTrue("单档波形总时长不得超过 200ms(触感是提示不是按摩)", p.sum() <= 200L)
        }
        // 三级强度单调:轻(CLOCK_TICK 同档的 confirm) < 重(warning)
        val light = paHapticPattern(PaHapticKind.CONFIRM)[1]
        val heavy = paHapticPattern(PaHapticKind.WARNING)[1]
        assertTrue(light < heavy)
    }
}
