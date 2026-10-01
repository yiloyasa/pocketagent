package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * usage 落盘节流纯函数的单测(R1 残项收口:hermes usage_update 逐事件投影后,
 * 本地缓存不再逐帧写 —— 节流 + 窗口变化/占用大跳两条例外,缓存永不失真)。
 */
class UsagePersistThrottleTest {

    private fun cursor(atMs: Long, tokens: Long = 1_000L, window: Long = 128_000L) =
        UsagePersistCursor(atMs, tokens, window)

    @Test
    fun `没落过盘_首帧必写`() {
        assertTrue(usagePersistCursorAccepts(null, nowMs = 1_000L, nextTokens = 10L, nextContextWindow = 0L))
    }

    @Test
    fun `间隔内小变化_节流跳过`() {
        assertFalse(
            usagePersistCursorAccepts(
                cursor(atMs = 0L, tokens = 1_000L),
                nowMs = USAGE_SAVE_MIN_INTERVAL_MS - 1,
                nextTokens = 1_400L,
                nextContextWindow = 128_000L,
            ),
        )
    }

    @Test
    fun `间隔到点_放行`() {
        assertTrue(
            usagePersistCursorAccepts(
                cursor(atMs = 0L),
                nowMs = USAGE_SAVE_MIN_INTERVAL_MS,
                nextTokens = 1_100L,
                nextContextWindow = 128_000L,
            ),
        )
    }

    @Test
    fun `间隔未到但窗口变化_立即放行_切模型首报不失真`() {
        assertTrue(
            usagePersistCursorAccepts(
                cursor(atMs = 0L, window = 128_000L),
                nowMs = 100L,
                nextTokens = 1_050L,
                nextContextWindow = 200_000L,
            ),
        )
    }

    @Test
    fun `间隔未到但占用大跳_立即放行`() {
        assertTrue(
            usagePersistCursorAccepts(
                cursor(atMs = 0L, tokens = 1_000L),
                nowMs = 100L,
                nextTokens = 1_000L + USAGE_SAVE_TOKEN_STEP,
                nextContextWindow = 128_000L,
            ),
        )
    }

    @Test
    fun `窗口未知零值不算变化_不触发额外落盘`() {
        assertFalse(
            usagePersistCursorAccepts(
                cursor(atMs = 0L, window = 0L),
                nowMs = 100L,
                nextTokens = 1_000L,
                nextContextWindow = 0L,
            ),
        )
    }

    @Test
    fun `占用下降不足一档_同样节流`() {
        assertFalse(
            usagePersistCursorAccepts(
                cursor(atMs = 0L, tokens = 5_000L),
                nowMs = 100L,
                nextTokens = 4_500L,
                nextContextWindow = 128_000L,
            ),
        )
    }

    @Test
    fun `游标字段就是落盘时的三要素`() {
        val c = UsagePersistCursor(atMs = 42L, tokens = 7L, contextWindow = 9L)
        assertEquals(42L, c.atMs)
        assertEquals(7L, c.tokens)
        assertEquals(9L, c.contextWindow)
    }
}
