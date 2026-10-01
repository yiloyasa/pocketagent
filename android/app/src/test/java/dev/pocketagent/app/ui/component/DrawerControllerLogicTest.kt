package dev.pocketagent.app.ui.component

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 抽屉跟手纯逻辑的单测:位移口径 + 松手速度采样。
 *
 * 【为什么存在】"跟手"的两半 —— 手指在哪面板在哪(位移),松手带多少动量落位
 * (速度交接)—— 都是数学,不碰 Compose 就能钉死;UI 层只消费这两件事实。
 */
class DrawerControllerLogicTest {

    // ---- drawerFollowOffsetPx:进度 → placement 偏移 ----

    @Test
    fun `进度0_完全收起_偏移等于负面板宽`() {
        assertEquals(-300f, drawerFollowOffsetPx(0f, 300f), 0.001f)
    }

    @Test
    fun `进度1_完全展开_偏移为零`() {
        assertEquals(0f, drawerFollowOffsetPx(1f, 300f), 0.001f)
    }

    @Test
    fun `进度一半_偏移一半面板宽`() {
        assertEquals(-150f, drawerFollowOffsetPx(0.5f, 300f), 0.001f)
    }

    @Test
    fun `超界进度按边界收敛_手势累计误差不外溢`() {
        assertEquals(0f, drawerFollowOffsetPx(1.7f, 300f), 0.001f)
        assertEquals(-300f, drawerFollowOffsetPx(-0.4f, 300f), 0.001f)
    }

    // ---- DragVelocityTracker:松手速度采样 ----

    @Test
    fun `首帧采样不产生速度_没有差分就没有速度`() {
        val t = DragVelocityTracker()
        t.sample(0.2f, nowMs = 1_000)
        assertEquals(0f, t.current, 0.0001f)
    }

    @Test
    fun `匀速右拖_速度为正且平滑窗口内稳定`() {
        val t = DragVelocityTracker()
        t.sample(0.0f, nowMs = 0)
        t.sample(0.1f, nowMs = 100) // 0.1 / 0.1s = 1.0 进度每秒
        assertEquals(1.0f, t.current, 0.0001f)
        t.sample(0.2f, nowMs = 200) // 差分仍是 1.0,平滑后不变
        assertEquals(1.0f, t.current, 0.0001f)
    }

    @Test
    fun `反向拖动_速度翻负_松手应弹回而非展开`() {
        val t = DragVelocityTracker()
        t.sample(0.2f, nowMs = 0)
        t.sample(0.1f, nowMs = 100) // -1.0 进度每秒
        assertTrue(t.current < 0f)
    }

    @Test
    fun `采样断流_速度清零_手指停过就不带旧动量`() {
        val t = DragVelocityTracker()
        t.sample(0.0f, nowMs = 0)
        t.sample(0.1f, nowMs = 100)
        assertEquals(1.0f, t.current, 0.0001f)
        t.sample(0.5f, nowMs = 1_000) // 距上一帧 900ms > 120ms 窗口
        assertEquals(0f, t.current, 0.0001f)
    }

    @Test
    fun `窗口边界内仍在采样_120ms 整恰好不断流`() {
        val t = DragVelocityTracker()
        t.sample(0.0f, nowMs = 0)
        t.sample(0.12f, nowMs = 120) // dt = 120ms,恰在窗口内
        assertEquals(1.0f, t.current, 0.0001f)
    }

    @Test
    fun `reset_清空速度与基线_取消手势不漏旧动量`() {
        val t = DragVelocityTracker()
        t.sample(0.0f, nowMs = 0)
        t.sample(0.1f, nowMs = 50)
        t.reset()
        assertEquals(0f, t.current, 0.0001f)
        // reset 后当作全新手势:下一帧重新建立基线,不与旧样本做差分
        t.sample(0.9f, nowMs = 10_000)
        assertEquals(0f, t.current, 0.0001f)
    }
}
