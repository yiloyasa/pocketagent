package dev.pocketagent.app.ui

import dev.pocketagent.app.ui.chat.COMPOSER_IMMERSIVE_TRIGGER_PX
import dev.pocketagent.app.ui.chat.HEADER_COLLAPSE_TRIGGER_PX
import dev.pocketagent.app.ui.chat.immersiveScrollDecision
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

/**
 * 方向 C · N1 上滑沉浸判定(ui/chat/ChatImmersive.kt [immersiveScrollDecision])
 * 纯函数钉死:渐进两段式入场(头→胶囊)、一步退场、回顶/回底特判、小位移节流。
 */
class ChatImmersiveLogicTest {

    private fun decide(
        headerCollapsed: Boolean = false,
        composerImmersed: Boolean = false,
        acc: Int = 0,
        atTop: Boolean = false,
        atBottom: Boolean = false,
    ) = immersiveScrollDecision(headerCollapsed, composerImmersed, acc, atTop, atBottom)

    @Test
    fun thresholds_are_two_stages() {
        assertEquals(96, HEADER_COLLAPSE_TRIGGER_PX)
        assertEquals(192, COMPOSER_IMMERSIVE_TRIGGER_PX)
    }

    @Test
    fun small_jitter_keeps_current_state() {
        assertNull(decide(acc = 95))
        assertNull(decide(acc = -95))
        assertNull(decide(headerCollapsed = true, acc = 95))
        assertNull(decide(headerCollapsed = true, composerImmersed = true, acc = -95))
    }

    @Test
    fun scroll_up_collapses_header_first_then_immerses_composer() {
        // 第一段:越过头部阈值只收头部,输入栏维持
        assertEquals(Pair(true, false), decide(headerCollapsed = false, acc = 96))
        // 第二段:越过胶囊阈值进入完全沉浸(即使头部已收、累计重新从 0 算)
        assertEquals(Pair(true, true), decide(headerCollapsed = true, acc = 192))
        // 已完全沉浸再上滑:维持(null = 不落值)
        assertNull(decide(headerCollapsed = true, composerImmersed = true, acc = 300))
    }

    @Test
    fun scroll_down_restores_everything_in_one_step() {
        assertEquals(Pair(false, false), decide(headerCollapsed = true, acc = -96))
        assertEquals(
            Pair(false, false),
            decide(headerCollapsed = true, composerImmersed = true, acc = -96),
        )
        // 本来就没沉浸:不落值
        assertNull(decide(acc = -300))
    }

    @Test
    fun back_to_top_restores() {
        assertEquals(Pair(false, false), decide(headerCollapsed = true, atTop = true))
        assertEquals(
            Pair(false, false),
            decide(headerCollapsed = true, composerImmersed = true, atTop = true, acc = 500),
        )
        assertNull(decide(atTop = true))
    }

    @Test
    fun back_to_bottom_restores_only_when_immersed() {
        // 贴底且沉浸 → 复原("回到底部输入栏复原"的硬保证)
        assertEquals(
            Pair(false, false),
            decide(headerCollapsed = true, composerImmersed = true, atBottom = true),
        )
        // 贴底但只是头部收起:交给方向规则 —— 若贴底即复原,头部在列表底部
        // 附近(距底 100px 跟随阈值 > 头部阈值)将永远收不起来
        assertNull(decide(headerCollapsed = true, atBottom = true, acc = 50))
    }
}
