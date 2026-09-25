package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Test

/**
 * 最近任务卡元信息行(纯函数)的单测。
 * 口径:「N 条消息 · 相对时间」;时间戳缺席(<=0)只报条数,不编时间。
 */
class ThreadRecentMetaTest {

    private val now = 1_800_000_000_000L

    @Test
    fun `条数加相对时间`() {
        // 30 分钟前
        assertEquals(
            "5 条消息 · 30 分钟前",
            threadRecentMeta(5, now - 30L * 60_000L, now),
        )
    }

    @Test
    fun `刚刚更新_报刚刚`() {
        assertEquals("1 条消息 · 刚刚", threadRecentMeta(1, now - 10_000L, now))
    }

    @Test
    fun `小时与天档位`() {
        assertEquals("0 条消息 · 3 小时前", threadRecentMeta(0, now - 3L * 3_600_000L, now))
        assertEquals("12 条消息 · 2 天前", threadRecentMeta(12, now - 2L * 86_400_000L, now))
    }

    @Test
    fun `时间戳缺席_只报条数`() {
        assertEquals("5 条消息", threadRecentMeta(5, 0L, now))
        assertEquals("5 条消息", threadRecentMeta(5, -1L, now))
    }

    @Test
    fun `超过一周_回落绝对日期不空段`() {
        // 相对时间回落到绝对日期(非空),拼出来仍是「条数 · 时间」两段
        val meta = threadRecentMeta(7, now - 30L * 86_400_000L, now)
        assertEquals(true, meta.startsWith("7 条消息 · "))
        assertEquals(false, meta.endsWith("· "))
    }
}
