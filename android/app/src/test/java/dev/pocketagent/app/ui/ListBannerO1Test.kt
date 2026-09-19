package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * P1-8 断连横幅状态机(O1 档)三态:
 * ① 断连 + 有快照 → 黄色提示「离线快照 · HH:mm」;
 * ② 断连 + 无快照 → 仍走离线语义,但不渲染时间(没有快照就没有"多新"可言);
 * ③ 已连接(只是列表拉取失败,如 codex 上游无响应)→ 中性,绝不出现"离线"二字。
 *
 * 时间格式化是注入的,这里用固定 "14:30" 让断言与日期无关。
 */
class ListBannerO1Test {

    private val fmt: (Long) -> String = { "14:30" }

    @Test
    fun disconnected_withSnapshot_showsOfflineSnapshotAtTime() {
        val banner = buildListBanner(connectionOk = false, snapshotAtMs = 1_700_000_000_000L, formatTime = fmt)
        assertTrue(banner.offline)
        assertEquals("离线快照 · 14:30,点击重试", banner.message)
    }

    @Test
    fun disconnected_withoutSnapshot_hidesTimeButStaysOffline() {
        val banner = buildListBanner(connectionOk = false, snapshotAtMs = 0L, formatTime = fmt)
        assertTrue(banner.offline)
        assertEquals("snapshotAtMs=0 不得把 epoch 渲染成时刻", "离线快照,点击重试", banner.message)
        assertFalse("无时间时不应出现孤立的分隔点", banner.message.contains("·"))
    }

    @Test
    fun connected_neverSaysOffline_evenWhenListFetchFails() {
        val banner = buildListBanner(connectionOk = true, snapshotAtMs = 1_700_000_000_000L, formatTime = fmt)
        assertFalse(banner.offline)
        assertTrue(banner.message.contains("14:30"))
        assertFalse("连接正常时禁止出现「离线」字样", banner.message.contains("离线"))
    }
}
