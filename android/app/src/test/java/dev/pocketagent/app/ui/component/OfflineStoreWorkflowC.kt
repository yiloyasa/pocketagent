package dev.pocketagent.app.ui.component

import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.File

/** 项6(缓存与内存):离线存储的纯逻辑(LRU 顺序 / 淘汰集合 / 字节统计 / 陈旧判定 / 快照读写)。 */
class OfflineStoreWorkflowC {

    @get:Rule
    val tmp = TemporaryFolder()

    private lateinit var dir: File

    @Before
    fun setUp() {
        dir = tmp.newFolder("offline")
        OfflineStore.initForTest(dir)
    }

    @Test
    fun lruOrderIsOldestFirstAndStableByName() {
        val e = listOf(
            OfflineStore.CacheEntry("b", 1, 100),
            OfflineStore.CacheEntry("a", 1, 100),
            OfflineStore.CacheEntry("c", 1, 50),
        )
        assertEquals(listOf("c", "a", "b"), OfflineStore.lruOrder(e).map { it.name })
    }

    @Test
    fun selectForEvictionRemovesOldestUntilUnderBudget() {
        val e = listOf(
            OfflineStore.CacheEntry("a", 40, 1),
            OfflineStore.CacheEntry("b", 40, 2),
            OfflineStore.CacheEntry("c", 40, 3),
        )
        assertEquals(emptyList<String>(), OfflineStore.selectForEviction(e, 120).map { it.name })
        assertEquals(listOf("a"), OfflineStore.selectForEviction(e, 100).map { it.name })
        assertEquals(listOf("a", "b"), OfflineStore.selectForEviction(e, 50).map { it.name })
    }

    @Test
    fun bytesUsedSumsSizes() {
        assertEquals(
            120L,
            OfflineStore.bytesUsed(
                listOf(OfflineStore.CacheEntry("a", 40, 1), OfflineStore.CacheEntry("b", 80, 2)),
            ),
        )
        assertEquals(0L, OfflineStore.bytesUsed(emptyList()))
    }

    @Test
    fun isStaleHonorsTtl() {
        assertTrue(OfflineStore.isStale(0, 1000, 500))
        assertFalse("ttl<=0 表示永不过期", OfflineStore.isStale(0, 1000, 0))
        assertFalse(OfflineStore.isStale(600, 1000, 500))
    }

    @Test
    fun cacheKeyIsDeterministicHex() {
        val k = OfflineStore.cacheKey("https://a.b/c?d=1")
        assertEquals("同输入恒同键", k, OfflineStore.cacheKey("https://a.b/c?d=1"))
        assertEquals(32, k.length)
        assertTrue("必须是无符号十六进制", k.all { it in "0123456789abcdef" })
    }

    @Test
    fun urlSnapshotRoundTrips() {
        val bytes = "页面内容".toByteArray()
        OfflineStore.putUrl("https://x.com/p", bytes, "text/html", "p.html")

        val snap = OfflineStore.getUrl("https://x.com/p")
        assertTrue("同 URL 必须命中", snap != null)
        assertArrayEquals(bytes, snap!!.bytes)
        assertEquals("text/html", snap.mime)
        assertEquals("p.html", snap.name)
        assertNull("不同 URL 不得命中", OfflineStore.getUrl("https://x.com/other"))
        assertTrue(OfflineStore.dirFor("url")!!.isDirectory)
    }

    @Test
    fun clearReleasesSpaceAndReturnsReleasedBytes() {
        OfflineStore.putUrl("https://x.com/p", ByteArray(10) { 1 }, "text/html", "p.html")
        val before = OfflineStore.usage()
        assertTrue(before > 0)
        val released = OfflineStore.clear("url")
        assertEquals("返回值应等于清前字节数", before, released)
        assertEquals(0L, OfflineStore.usage())
    }

    @Test
    fun clearUnknownPartitionIsNoOpAndZero() {
        OfflineStore.putUrl("https://x.com/p", ByteArray(10) { 1 }, "text/html", "p.html")
        val before = OfflineStore.usage()
        assertEquals(0L, OfflineStore.clear("nope"))
        assertEquals("未知 id 不得动任何文件", before, OfflineStore.usage())
    }

    @Test
    fun clearSkipsNonClearablePartition() {
        // 注入一个 clearable=false 的分区(代表"用户显式下载内容"),清理必须跳过且返回 0
        val root = tmp.newFolder("offline2")
        OfflineStore.initForTest(
            root,
            listOf(OfflineStore.Partition("dl", "下载内容", "pa_dl", 1_000_000, false)),
        )
        val f = File(root, "pa_dl/keep.bin")
        f.parentFile?.mkdirs()
        f.writeBytes(ByteArray(20) { 1 })

        assertEquals("clearable=false 必须返回 0", 0L, OfflineStore.clear("dl"))
        assertTrue("已下载内容不得被清理", f.isFile)
    }

    @Test
    fun sweepStaleRemovesOnlyEntriesPastTtl() {
        // 注入一个 ttl=500ms 的分区:陈旧条目被清扫,新鲜的保留
        val root = tmp.newFolder("offline3")
        OfflineStore.initForTest(
            root,
            listOf(OfflineStore.Partition("t", "测试", "pa_t", 1_000_000, true, 500)),
        )
        val fresh = File(root, "pa_t/fresh.bin")
        val stale = File(root, "pa_t/stale.bin")
        fresh.writeBytes(ByteArray(4) { 1 })
        stale.writeBytes(ByteArray(4) { 2 })
        val now = 10_000L
        assertTrue(fresh.setLastModified(now - 100))
        assertTrue(stale.setLastModified(now - 600))

        assertEquals(1, OfflineStore.sweepStale("t", now))
        assertTrue("新鲜条目必须保留", fresh.isFile)
        assertFalse("陈旧条目必须删除", stale.exists())
    }

    @Test
    fun sweepStaleSkipsPartitionWithoutTtl() {
        val root = tmp.newFolder("offline4")
        OfflineStore.initForTest(
            root,
            listOf(OfflineStore.Partition("keep", "不过期", "pa_keep", 1_000_000, true, 0)),
        )
        val f = File(root, "pa_keep/old.bin")
        f.writeBytes(ByteArray(4) { 1 })
        assertTrue(f.setLastModified(1L)) // 1970 年,但分区声明永不过期

        assertEquals("ttl<=0 = 永不过期,不得清扫", 0, OfflineStore.sweepStale("keep"))
        assertTrue(f.isFile)
    }
}
