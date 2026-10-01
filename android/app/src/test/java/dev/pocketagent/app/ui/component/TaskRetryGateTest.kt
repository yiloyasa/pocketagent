package dev.pocketagent.app.ui.component

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 任务中心「重试语义」纯逻辑的单测(R3-C):
 *  1. 门闩防重入 —— 冷却窗口内的第二发点击一律拒收,"重试中"窗口进出有界;
 *  2. 重新下载可行性分类 —— URL/电脑路径可重取,blob/文本另存不可重取
 *     (不可重取的条目**不摆**「重新下载」死按钮,分类是行尾动作的数据源)。
 */
class TaskRetryGateTest {

    // ---- RetryGate:防重入 ----

    @Test
    fun `首次点击放行_冷却窗口内第二发拒收`() {
        val g = RetryGate(cooldownMs = 1500)
        assertTrue(g.tryBegin(1000))
        assertFalse("冷却窗口内的双击必须被拒收", g.tryBegin(2000))
        assertTrue("窗口内应处于重试中", g.inFlight(2000))
    }

    @Test
    fun `冷却结束后再次放行且出窗`() {
        val g = RetryGate(cooldownMs = 1500)
        assertTrue(g.tryBegin(1000))
        // 第一窗已出窗(必须在第二发之前查验):2600-1000=1600 ≥ 1500
        assertFalse(g.inFlight(2600))
        assertTrue(g.tryBegin(2600)) // 冷却已过 → 放行
        // 二次放行即刻进入新窗口:inFlight 为真正是"重试中"视觉的数据源
        assertTrue(g.inFlight(2600))
    }

    @Test
    fun `边界_恰好等于冷却时长已出窗_可再次点击`() {
        val g = RetryGate(cooldownMs = 1500)
        assertTrue(g.tryBegin(0))
        assertTrue(g.tryBegin(1500)) // 1500 - 0 = 1500,不小于 1500 → 放行
    }

    @Test
    fun `新门闩未点击时不处于重试中_且首次必放行`() {
        val g = RetryGate(cooldownMs = 1500)
        assertFalse(g.inFlight(500_000))
        assertTrue("初值取负冷却:第一次点击恒放行", g.tryBegin(500_000))
    }

    @Test
    fun `连续快速点击只放行一发`() {
        val g = RetryGate(cooldownMs = 1500)
        val taps = listOf(0L, 100, 200, 300, 400)
        assertEquals(1, taps.count { g.tryBegin(it) })
    }

    // ---- redownloadPlanOf:重新下载可行性分类 ----

    @Test
    fun `URL来源归FROM_URL_电脑绝对路径归FROM_PC`() {
        assertEquals(RedownloadPlan.FROM_URL, redownloadPlanOf("https://example.com/a.apk"))
        assertEquals(RedownloadPlan.FROM_URL, redownloadPlanOf("http://example.com/a.zip"))
        assertEquals(RedownloadPlan.FROM_PC, redownloadPlanOf("/home/u/dist/app.apk"))
        assertEquals(RedownloadPlan.FROM_PC, redownloadPlanOf("C:\\build\\app.apk"))
        assertEquals(RedownloadPlan.FROM_PC, redownloadPlanOf("D:/out/pkg.msi"))
        assertEquals(RedownloadPlan.FROM_PC, redownloadPlanOf("\\\\nas\\share\\doc.pdf"))
    }

    @Test
    fun `blob与文本另存无来源可重放_归NONE`() {
        // blob 落盘与 `threadId/文件名` 形态的文本另存,手机侧没有可重放的来源:
        // 分类为 NONE 的条目**不摆**「重新下载」(宁可引导回会话,不给死按钮)
        assertEquals(RedownloadPlan.NONE, redownloadPlanOf("blob:1700000000:clip.zip"))
        assertEquals(RedownloadPlan.NONE, redownloadPlanOf("thr-9/report.md"))
        assertEquals(RedownloadPlan.NONE, redownloadPlanOf(""))
    }

    @Test
    fun `连接依赖映射_可空表示无来源`() {
        assertEquals(false, redownloadNeedsPc(RedownloadPlan.FROM_URL))
        assertEquals(true, redownloadNeedsPc(RedownloadPlan.FROM_PC))
        assertNull(redownloadNeedsPc(RedownloadPlan.NONE))
    }
}
