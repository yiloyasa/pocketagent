package dev.pocketagent.app.ui.component

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 断连横幅三态状态机的单测(断线 / 重连中 / 已恢复)。
 *
 * 【口径】checking 是重连探测的中间态:断过就保持横幅可见(探活回退链动辄数秒,
 * 撤了再放回来就是用户看见的"横幅闪烁");恢复(ok)直接退场,不弹成功横幅;
 * 轮数只在"从非 down 落回 down"时 +1,挂载时已在 down 不虚计。
 */
class ConnectionBannerStateTest {

    // ---- 启动期:没断过不出横幅 ----

    @Test
    fun `启动首轮checking_不出横幅`() {
        val s = nextConnectionBannerState(prev = null, next = "checking", attempts = 0)
        assertFalse(s.visible)
        assertEquals(0, s.attempts)
    }

    // ---- 断线 ----

    @Test
    fun `落回down_可见且记一轮`() {
        val s = nextConnectionBannerState(prev = "checking", next = "down", attempts = 0)
        assertTrue(s.visible)
        assertEquals(1, s.attempts)
    }

    @Test
    fun `down持续_不重复计轮`() {
        val s = nextConnectionBannerState(prev = "down", next = "down", attempts = 2)
        assertTrue(s.visible)
        assertEquals(2, s.attempts)
    }

    @Test
    fun `挂载时已在down_不虚计一轮但可见`() {
        val s = nextConnectionBannerState(prev = "down", next = "down", attempts = 0)
        assertTrue(s.visible)
        assertEquals(0, s.attempts)
    }

    // ---- 重连中:sticky,不闪 ----

    @Test
    fun `断过后进入checking_横幅保持可见_不闪没`() {
        val s = nextConnectionBannerState(prev = "down", next = "checking", attempts = 1)
        assertTrue(s.visible)
        assertEquals(1, s.attempts)
    }

    @Test
    fun `重连探测失败再落down_计下一轮`() {
        val s = nextConnectionBannerState(prev = "checking", next = "down", attempts = 1)
        assertTrue(s.visible)
        assertEquals(2, s.attempts)
    }

    // ---- 已恢复:退场即无声,不闪成功横幅 ----

    @Test
    fun `恢复ok_横幅退场且轮数清零`() {
        val s = nextConnectionBannerState(prev = "down", next = "ok", attempts = 3)
        assertFalse(s.visible)
        assertEquals(0, s.attempts)
    }

    @Test
    fun `恢复后再断_从第一轮重新计`() {
        val after = nextConnectionBannerState(prev = "down", next = "ok", attempts = 3)
        val again = nextConnectionBannerState(prev = "ok", next = "down", attempts = after.attempts)
        assertTrue(again.visible)
        assertEquals(1, again.attempts)
    }
}
