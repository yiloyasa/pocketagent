package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * P0-1 流式滚动锚定的判定纯函数(见 ChatScreen.kt「P0-1 流式滚动锚定」区):
 * 距底 100px 内才跟随;离底只计未读、绝不拽动;手动滚回即恢复。
 */
class ChatScrollAnchorTest {

    @Test
    fun follow_threshold_is_100px() {
        assertEquals(100, SCROLL_FOLLOW_THRESHOLD_PX)
    }

    @Test
    fun at_or_within_threshold_counts_as_following() {
        assertTrue(shouldFollowDistance(0))
        assertTrue(shouldFollowDistance(99))
        assertTrue(shouldFollowDistance(100))
        assertFalse(shouldFollowDistance(101))
        assertFalse(shouldFollowDistance(Int.MAX_VALUE))
    }

    @Test
    fun auto_scroll_follows_when_already_following_even_if_snapshot_is_stale() {
        // 流式中持续钉底:内容刚追加、列表还没滚到位(距底读数被拉大),
        // 只要上一帧在跟随就继续回底 —— 否则第一个新 token 就会把跟随打断
        assertTrue(shouldAutoScroll(distanceFromBottomPx = 40_000, wasFollowing = true))
    }

    @Test
    fun auto_scroll_resumes_when_user_is_still_within_threshold() {
        // 没在跟随,但用户其实仍贴底(内容短/刚回底)→ 视为没离开,恢复跟随
        assertTrue(shouldAutoScroll(distanceFromBottomPx = 40, wasFollowing = false))
    }

    @Test
    fun auto_scroll_refuses_when_user_scrolled_away() {
        // 上滑阅读历史:既不在跟随、也远超阈值 → 新 token 绝不拽动
        assertFalse(shouldAutoScroll(distanceFromBottomPx = Int.MAX_VALUE, wasFollowing = false))
    }

    // ---- e2e-paired-20261009 问题1:胶囊态抑制跟底(判定矩阵) ----

    @Test
    fun immersed_capsule_never_auto_scrolls_on_new_content() {
        // 胶囊态 + 新内容到达(流式 chunk/新消息)→ 保持胶囊不跟底:
        // 未读交给「↓ N 条新内容」胶囊,点它才回底
        assertFalse(
            shouldAutoScroll(
                distanceFromBottomPx = Int.MAX_VALUE, wasFollowing = false, composerImmersed = true,
            ),
        )
        // 上一帧还在跟随(流式中进胶囊)也一样不拽 —— 跟随位可能滞后于用户手势
        assertFalse(
            shouldAutoScroll(
                distanceFromBottomPx = 0, wasFollowing = true, composerImmersed = true,
            ),
        )
        // 仍在距底阈值内(长尾项读数按 0 计的边角)同样不拽
        assertFalse(
            shouldAutoScroll(
                distanceFromBottomPx = 0, wasFollowing = false, composerImmersed = true,
            ),
        )
    }

    @Test
    fun non_immersed_auto_scroll_behavior_unchanged() {
        // 非胶囊态行为逐字不变(默认参数 = 旧签名调用点/旧用例口径)
        assertTrue(
            shouldAutoScroll(distanceFromBottomPx = 40_000, wasFollowing = true, composerImmersed = false),
        )
        assertTrue(
            shouldAutoScroll(distanceFromBottomPx = 40, wasFollowing = false, composerImmersed = false),
        )
        assertFalse(
            shouldAutoScroll(distanceFromBottomPx = Int.MAX_VALUE, wasFollowing = false, composerImmersed = false),
        )
    }

    @Test
    fun distance_is_zero_for_empty_or_unmeasured_lists() {
        assertEquals(0, distanceFromBottomPx(totalItems = 0, lastVisibleIndex = -1, lastVisibleEnd = 0, viewportEndOffset = 0))
        assertEquals(0, distanceFromBottomPx(totalItems = 5, lastVisibleIndex = -1, lastVisibleEnd = 0, viewportEndOffset = 900))
    }

    @Test
    fun distance_is_far_when_last_item_not_visible() {
        // 用户翻到中间:最后一项不在可视区 = 距底"无限远"
        assertEquals(
            Int.MAX_VALUE,
            distanceFromBottomPx(totalItems = 30, lastVisibleIndex = 7, lastVisibleEnd = 1200, viewportEndOffset = 1600),
        )
    }

    @Test
    fun distance_is_remaining_space_when_last_item_fully_visible() {
        // 视口底 1600,最后一项底缘 1560 → 距底 40;底缘超出视口(长项)按 0
        assertEquals(
            40,
            distanceFromBottomPx(totalItems = 30, lastVisibleIndex = 29, lastVisibleEnd = 1560, viewportEndOffset = 1600),
        )
        assertEquals(
            0,
            distanceFromBottomPx(totalItems = 30, lastVisibleIndex = 29, lastVisibleEnd = 1700, viewportEndOffset = 1600),
        )
    }

    @Test
    fun stream_block_index_grows_every_24_chars_and_never_negative() {
        assertEquals(0, streamBlockIndex(0))
        assertEquals(0, streamBlockIndex(23))
        assertEquals(1, streamBlockIndex(24))
        assertEquals(3, streamBlockIndex(72))
        assertEquals(0, streamBlockIndex(-5))
        assertEquals(STREAM_BLOCK_CHARS, 24)
    }

    @Test
    fun unread_stays_zero_while_following() {
        assertEquals(0, accumulateUnreadBlocks(wasFollowing = true, currentUnread = 0, newItemBlocks = 4, newStreamBlocks = 9))
        // 已经背着未读时回到贴底:清零(调用方以返回值覆盖)
        assertEquals(0, accumulateUnreadBlocks(wasFollowing = true, currentUnread = 7, newItemBlocks = 0, newStreamBlocks = 0))
    }

    @Test
    fun unread_accumulates_items_and_stream_blocks_while_away() {
        assertEquals(
            5,
            accumulateUnreadBlocks(wasFollowing = false, currentUnread = 0, newItemBlocks = 2, newStreamBlocks = 3),
        )
        assertEquals(
            12,
            accumulateUnreadBlocks(wasFollowing = false, currentUnread = 7, newItemBlocks = 2, newStreamBlocks = 3),
        )
    }

    @Test
    fun unread_accumulation_never_decreases_and_never_negative() {
        assertEquals(
            7,
            accumulateUnreadBlocks(wasFollowing = false, currentUnread = 7, newItemBlocks = -3, newStreamBlocks = -1),
        )
    }
}
