package dev.pocketagent.app.ui

import dev.pocketagent.app.ui.chat.COMPOSER_IMMERSIVE_TRIGGER_PX
import dev.pocketagent.app.ui.chat.HEADER_COLLAPSE_TRIGGER_PX
import dev.pocketagent.app.ui.chat.followBottomDecision
import dev.pocketagent.app.ui.chat.immersiveResetOnGestureReachBottom
import dev.pocketagent.app.ui.chat.immersiveResetOnSend
import dev.pocketagent.app.ui.chat.immersiveScrollDecision
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 方向 C · N1 上滑沉浸判定(ui/chat/ChatImmersive.kt [immersiveScrollDecision])
 * 纯函数钉死:渐进两段式入场(头→胶囊)、一步退场、回顶/回底特判、小位移节流。
 * 【e2e-paired-20261009 问题1】胶囊态抑制跟底(followBottomDecision)与
 * 发消息复原(immersiveResetOnSend)的判定矩阵同文件钉死。
 * 【F8 遗留反馈 2】用户手势直达贴底的第四复原动作
 * (immersiveResetOnGestureReachBottom)判定矩阵同文件钉死。
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

    // ---- e2e-paired-20261009 问题1:胶囊态抑制 followBottom 翻真(判定矩阵) ----

    @Test
    fun follow_decision_never_flips_true_while_immersed() {
        // 胶囊态 + 流式长尾项把距底读数算成"贴底" → 绝不翻真(胶囊态形同虚设的主冲突点)
        assertFalse(followBottomDecision(atFollowDistance = true, composerImmersed = true))
        // 胶囊态 + 远离底部 → 解除跟随(维持 false)
        assertFalse(followBottomDecision(atFollowDistance = false, composerImmersed = true))
    }

    @Test
    fun follow_decision_unchanged_when_not_immersed() {
        // 非胶囊态行为不变:贴底恢复跟随、离底解除跟随
        assertTrue(followBottomDecision(atFollowDistance = true, composerImmersed = false))
        assertFalse(followBottomDecision(atFollowDistance = false, composerImmersed = false))
    }

    // ---- e2e-paired-20261009 问题1:用户主动发消息 → 沉浸态复原(判定矩阵) ----

    @Test
    fun send_resets_immersive_from_any_immersive_piece() {
        // 胶囊态 + 用户发送 → 展开复原
        assertTrue(immersiveResetOnSend(headerCollapsed = false, composerImmersed = true, composerCapsuleTapped = false))
        // 只收了石墨头(第一阶段)+ 用户发送 → 复原
        assertTrue(immersiveResetOnSend(headerCollapsed = true, composerImmersed = false, composerCapsuleTapped = false))
        // 点胶囊手动展开态 + 用户发送 → 复原
        assertTrue(immersiveResetOnSend(headerCollapsed = false, composerImmersed = true, composerCapsuleTapped = true))
    }

    @Test
    fun send_is_noop_when_not_immersed() {
        // 非胶囊态行为不变:false = 调用方整段跳过,不动 followBottom/未读计数
        assertFalse(immersiveResetOnSend(headerCollapsed = false, composerImmersed = false, composerCapsuleTapped = false))
    }

    // ---- F8 遗留反馈 2:用户手势直达贴底 → 沉浸态复原(第四复原动作,判定矩阵) ----

    @Test
    fun gesture_reach_bottom_resets_immersive_from_any_immersive_piece() {
        // 快 fling 直落底(同 item 差分被跳过的到达):胶囊态 + 双闸贴底(距底阈值
        // 内 + canScrollForward 翻假)→ 复原
        assertTrue(
            immersiveResetOnGestureReachBottom(
                atFollowDistance = true, canScrollForward = false, programmaticScroll = false,
                headerCollapsed = false, composerImmersed = true, composerCapsuleTapped = false,
            )
        )
        // 只收了石墨头(第一段沉浸)同样复原
        assertTrue(
            immersiveResetOnGestureReachBottom(
                atFollowDistance = true, canScrollForward = false, programmaticScroll = false,
                headerCollapsed = true, composerImmersed = false, composerCapsuleTapped = false,
            )
        )
        // 胶囊点开态(手动展开旗标挂着)同样复原
        assertTrue(
            immersiveResetOnGestureReachBottom(
                atFollowDistance = true, canScrollForward = false, programmaticScroll = false,
                headerCollapsed = false, composerImmersed = true, composerCapsuleTapped = true,
            )
        )
    }

    @Test
    fun gesture_reach_bottom_never_fires_for_programmatic_or_fake_bottom() {
        // 程序滚动(跟底/流式跟底/历史锚定)标记期间的到达 = 程序 → 绝不复原
        assertFalse(
            immersiveResetOnGestureReachBottom(
                atFollowDistance = true, canScrollForward = false, programmaticScroll = true,
                headerCollapsed = false, composerImmersed = true, composerCapsuleTapped = false,
            )
        )
        // 程序滚动压过一切沉浸件:即使三件全挂着也不复原
        assertFalse(
            immersiveResetOnGestureReachBottom(
                atFollowDistance = true, canScrollForward = false, programmaticScroll = true,
                headerCollapsed = true, composerImmersed = true, composerCapsuleTapped = true,
            )
        )
        // 伪装贴底:距底读数被长尾项 clamp 在阈值内,但 canScrollForward 仍真
        // (列表还能继续下滚 = 流式长尾项里阅读)→ 绝不复原
        assertFalse(
            immersiveResetOnGestureReachBottom(
                atFollowDistance = true, canScrollForward = true, programmaticScroll = false,
                headerCollapsed = false, composerImmersed = true, composerCapsuleTapped = false,
            )
        )
        // 未达贴底距离(还在半空)→ 不复原
        assertFalse(
            immersiveResetOnGestureReachBottom(
                atFollowDistance = false, canScrollForward = false, programmaticScroll = false,
                headerCollapsed = false, composerImmersed = true, composerCapsuleTapped = false,
            )
        )
        // 非沉浸态到达 → false(调用方整段跳过,非胶囊态行为不变)
        assertFalse(
            immersiveResetOnGestureReachBottom(
                atFollowDistance = true, canScrollForward = false, programmaticScroll = false,
                headerCollapsed = false, composerImmersed = false, composerCapsuleTapped = false,
            )
        )
    }

    @Test
    fun gesture_reach_bottom_restore_is_independent_of_follow_decision() {
        // 复原 ≠ 翻 followBottom:胶囊态下距底观测的 follow 裁定仍恒 false(F8 抑制
        // 语义原样)—— 复原帧两者同帧发生时,followBottom 维持 false,未读胶囊留着
        // (直到点它或发消息才重申跟随),两个纯函数互不越界。
        assertTrue(
            immersiveResetOnGestureReachBottom(
                atFollowDistance = true, canScrollForward = false, programmaticScroll = false,
                headerCollapsed = false, composerImmersed = true, composerCapsuleTapped = false,
            )
        )
        assertFalse(followBottomDecision(atFollowDistance = true, composerImmersed = true))
    }
}
