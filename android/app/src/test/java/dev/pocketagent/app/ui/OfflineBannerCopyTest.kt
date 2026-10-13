package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 离线人话(快赢版)回归:相对时长横幅 + 输入门控。
 *
 * 时间戳只读 ThreadSnapshot 已有落盘值(messagesAt),此处用注入假钟(noWMs)
 * 钉相对文案,不编造。
 */
class OfflineBannerCopyTest {

    private val now = 1_700_000_000_000L

    // ---- 相对时长 ----

    @Test
    fun `无时间戳_返回空_不显示横幅`() {
        assertNull(offlineAgeCopy(0L, now))
        assertNull(offlineAgeCopy(-5L, now))
        assertNull(offlineSnapshotBanner(0L, now))
    }

    @Test
    fun `一分钟内_刚刚`() {
        assertEquals("刚刚", offlineAgeCopy(now - 30_000L, now))
    }

    @Test
    fun `分钟与小时桶_向下取整`() {
        assertEquals("5分钟前", offlineAgeCopy(now - 5L * 60_000L, now))
        assertEquals("3小时前", offlineAgeCopy(now - 3L * 3_600_000L, now))
    }

    @Test
    fun `跨天_天桶`() {
        assertEquals("2天前", offlineAgeCopy(now - 2L * 86_400_000L, now))
    }

    @Test
    fun `未来时间戳_按刚刚收敛_绝不出现负数`() {
        assertEquals("刚刚", offlineAgeCopy(now + 60_000L, now))
    }

    // ---- 横幅整句 ----

    @Test
    fun `横幅三段齐全_旧对话连上自动更新只能看不能发`() {
        val banner = offlineSnapshotBanner(now - 3L * 3_600_000L, now)
        assertEquals("这是3小时前的旧对话，连上自动更新，现在只能看不能发", banner)
    }

    @Test
    fun `横幅天桶同样成句`() {
        val banner = offlineSnapshotBanner(now - 2L * 86_400_000L, now)
        assertEquals("这是2天前的旧对话，连上自动更新，现在只能看不能发", banner)
    }

    // ---- 输入门控 ----

    @Test
    fun `离线快照态输入停用_在线可用`() {
        assertFalse(offlineInputEnabled(true))
        assertTrue(offlineInputEnabled(false))
    }

    // ---- 用户可见文案禁词纪律 ----

    @Test
    fun `离线新增文案_无禁词`() {
        val copies = listOfNotNull(
            offlineSnapshotBanner(now - 60_000L, now),
            offlineSnapshotBanner(now - 5L * 3_600_000L, now),
            offlineSnapshotBanner(now - 3L * 86_400_000L, now),
        )
        assertTrue(copies.size == 3)
        copies.forEach { copy ->
            assertFalse("不应出现「未知」:$copy", copy.contains("未知"))
            assertFalse("不应出现「错误」:$copy", copy.contains("错误"))
            assertFalse("用户文案禁英文技术词 token:$copy", copy.lowercase().contains("token"))
            assertFalse("用户文案禁英文技术词 daemon:$copy", copy.lowercase().contains("daemon"))
            assertFalse("用户文案禁英文技术词 adb:$copy", copy.lowercase().contains("adb"))
        }
    }
}
