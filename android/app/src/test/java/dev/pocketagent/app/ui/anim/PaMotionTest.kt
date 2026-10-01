package dev.pocketagent.app.ui.anim

import androidx.compose.animation.core.Spring
import androidx.compose.animation.core.SpringSpec
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 全局动效令牌钉值:手感调优只许动 PaMotion,动了必须在这里留下痕迹。
 * 同时钉"同族组件同弹簧参数"的六层分层(M3 Expressive)与循环令牌时长基线。
 */
class PaMotionTest {

    private fun <T> params(spec: SpringSpec<T>): Pair<Float, Float> =
        spec.dampingRatio to spec.stiffness

    @Test
    fun `六层弹簧族_参数逐层钉死`() {
        assertEquals(1.0f to 420f, params(PaMotion.standard<Float>()))
        assertEquals(0.9f to 380f, params(PaMotion.emphasized<Float>()))
        assertEquals(0.8f to 380f, params(PaMotion.momentum<Float>()))
        assertEquals(0.85f to 300f, params(PaMotion.sheet<Float>()))
        assertEquals(Spring.DampingRatioNoBouncy to 520f, params(PaMotion.press<Float>()))
        // nav 的声明类型是 FiniteAnimationSpec(NavHost 约束),实现是 spring,
        // 断言其底层弹簧参数须显式收窄(实现若换成 tween 此处即断,这正是要钉的)。
        @Suppress("UNCHECKED_CAST")
        val navSpring = PaMotion.nav<Float>() as SpringSpec<Float>
        assertEquals(1.0f to 380f, params(navSpring))
    }

    @Test
    fun `分层纪律_微交互最快_标准临界_导航不慢于标准_弹层最慢`() {
        val press = PaMotion.press<Float>().stiffness
        val standard = PaMotion.standard<Float>().stiffness
        @Suppress("UNCHECKED_CAST")
        val nav = (PaMotion.nav<Float>() as SpringSpec<Float>).stiffness
        val sheet = PaMotion.sheet<Float>().stiffness
        assertTrue("press 应最快", press > standard)
        // nav 默认 380 比 standard(420)更柔:进场从容,退场由调用方传更大 stiffness
        assertTrue("nav 默认不应快于 standard", nav <= standard)
        assertTrue("sheet 应最慢(大面积位移的从容感)", sheet < standard)
        // 临界阻尼族(无过冲)与过冲族的边界
        assertEquals(1f, PaMotion.standard<Float>().dampingRatio, 0f)
        assertTrue(PaMotion.emphasized<Float>().dampingRatio < 1f)
        assertTrue(PaMotion.momentum<Float>().dampingRatio <= PaMotion.emphasized<Float>().dampingRatio)
    }

    @Test
    fun `循环令牌_时长基线钉值`() {
        assertEquals(1200, PaMotion.Loop.SHIMMER_MS)
        assertEquals(900, PaMotion.Loop.PULSE_MS)
        assertEquals(1500, PaMotion.Loop.PROGRESS_MS)
    }

    @Test
    fun `循环令牌_pulse与sweep可对任意类型实例化`() {
        // 循环令牌是 InfiniteRepeatableSpec 工厂;泛型实例化不炸即契约成立
        val p = PaMotion.pulse<Float>()
        val s = PaMotion.sweep<Float>()
        val custom = PaMotion.sweep<Float>(PaMotion.Loop.PROGRESS_MS)
        assertNotNull(p)
        assertNotNull(s)
        assertNotNull(custom)
    }

    @Test
    fun `减弱动效_instant存在且可实例化`() {
        assertNotNull(PaMotion.instant<Float>())
    }
}
