package dev.pocketagent.app.ui.component

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.Usage

/**
 * 断网可读的用量缓存(UsageCache 的纯逻辑部分)。
 *
 * 覆盖:往返序列化(含 Usage 全字段 / 空 Usage)、损坏载荷、陈旧判定、离线提示文案。
 * 这些不该只在真机上靠肉眼验 —— 断了网才发现"缓存读不出来"是最糟的时机。
 */
class WorkflowAUsageCacheTest {

    private fun usage(
        i: Long,
        o: Long,
        cr: Long,
        cw: Long,
        cost: Double,
        reason: Long,
    ): Usage = Usage.newBuilder()
        .setInputTokens(i)
        .setOutputTokens(o)
        .setCacheReadTokens(cr)
        .setCacheWriteTokens(cw)
        .setCostUsd(cost)
        .setReasoningTokens(reason)
        .build()

    @Test
    fun snapshot_round_trips_through_string() {
        val snap = UsageSnapshot(
            contextWindow = 272_000,
            windowIsFallback = false,
            turnUsage = usage(1_000, 200, 500, 30, 0.12, 40),
            totalUsage = usage(10_000, 2_000, 5_000, 300, 1.2, 400),
            updatedAt = 1_700_000_000_000L,
        )
        val decoded = decodeUsageSnapshot(encodeUsageSnapshot(snap))!!
        assertEquals(272_000L, decoded.contextWindow)
        assertFalse(decoded.windowIsFallback)
        assertEquals(1_700_000_000_000L, decoded.updatedAt)
        assertEquals(1_000L, decoded.turnUsage!!.inputTokens)
        assertEquals(500L, decoded.turnUsage!!.cacheReadTokens)
        assertEquals(0.12, decoded.turnUsage!!.costUsd, 1e-9)
        assertEquals(40L, decoded.turnUsage!!.reasoningTokens)
        assertEquals(10_000L, decoded.totalUsage!!.inputTokens)
    }

    @Test
    fun fallback_flag_and_missing_usage_are_preserved() {
        val snap = UsageSnapshot(
            contextWindow = 0,
            windowIsFallback = true,
            turnUsage = null,
            totalUsage = null,
            updatedAt = 42L,
        )
        val decoded = decodeUsageSnapshot(encodeUsageSnapshot(snap))!!
        assertTrue(decoded.windowIsFallback)
        assertNull(decoded.turnUsage)
        assertNull(decoded.totalUsage)
        assertEquals(42L, decoded.updatedAt)
    }

    @Test
    fun corrupt_or_empty_payload_decodes_to_null() {
        assertNull(decodeUsageSnapshot(null))
        assertNull(decodeUsageSnapshot(""))
        assertNull(decodeUsageSnapshot("{not json"))
    }

    @Test
    fun staleness_uses_a_named_threshold_and_ignores_missing_cache() {
        val now = 10_000_000_000L
        assertFalse(isUsageStale(now - 1_000, now))
        assertTrue(isUsageStale(now - (USAGE_STALE_AFTER_MS + 1), now))
        // 无缓存(updatedAt <= 0)不算陈旧 —— 它根本没有"多久之前"可言
        assertFalse(isUsageStale(0L, now))
        assertFalse(isUsageStale(-5L, now))
    }

    @Test
    fun offline_label_says_when_there_is_no_cache() {
        assertEquals("离线 · 暂无缓存", usageOfflineLabel(0L, 1_700_000_000_000L))
    }

    @Test
    fun offline_label_always_reads_as_offline_and_marks_the_time() {
        val now = System.currentTimeMillis()
        val label = usageOfflineLabel(now - 60_000, now)
        assertTrue(label.startsWith("离线"))
        assertTrue(label.contains("最后更新"))
    }
}
