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
    fun `同 URL 二次存档只留一份_读到的是最新内容与时间戳`() {
        // 【N2】putUrl 写新档后必须清掉同 URL 的旧快照文件(旧实现同 key 堆多份);
        // getUrl 按"文件名 capturedAt 段最大"挑新档 —— mtime 是"最近查看",旧档被
        // touch 过就会假压新档,按内容自带时间戳选才对。
        val key = OfflineStore.cacheKey("https://x.com/p")
        OfflineStore.putUrl("https://x.com/p", "旧页面 v1".toByteArray(), "text/html", "p.html", capturedAtMs = 1_000L)
        OfflineStore.putUrl("https://x.com/p", "新页面 v2".toByteArray(), "text/html", "p2.html", capturedAtMs = 2_000L)

        // 只留一份:目录里同 key 前缀的文件恰一个
        val files = OfflineStore.dirFor("url")!!.listFiles { f -> f.isFile && f.name.startsWith("$key.") }!!
        assertEquals("同 URL 二次存档后应只剩 1 个文件", 1, files.size)
        assertTrue("留下的是带新时间戳(2000ms → hex 7d0)的新档", files[0].name.contains("7d0"))
        // 读到最新内容与文件名内嵌的时间戳
        val snap = OfflineStore.getUrl("https://x.com/p")!!
        assertArrayEquals("新页面 v2".toByteArray(), snap.bytes)
        assertEquals("p2.html", snap.name)
        assertEquals("capturedAt 取自文件名段(不随 touch 漂移)", 2_000L, snap.capturedAt)
    }

    @Test
    fun `旧格式与新档混存_按capturedAt选新档_mtime仅旧格式间回退`() {
        // 存量目录兜底口径:手工造一个旧格式(两段,无时间戳段)文件 + 一个新档。
        // 旧档 mtime 拨到"未来"(被 touch 过的极端情况),仍必须输给带有效时间戳的新档;
        // 两个旧格式之间才按 mtime 回退。
        val key = OfflineStore.cacheKey("https://x.com/old")
        val dir = OfflineStore.dirFor("url")!!
        // 旧格式:mime hex + name hex(两段)
        val oldA = java.io.File(dir, "$key.746578742f68746d6c.612e68746d6c") // "text/html"."a.html"
        oldA.writeBytes("旧A".toByteArray())
        val oldB = java.io.File(dir, "$key.746578742f68746d6c.622e68746d6c") // "text/html"."b.html"
        oldB.writeBytes("旧B".toByteArray())
        assertTrue(oldA.setLastModified(9_000L))
        assertTrue(oldB.setLastModified(8_000L))
        // 旧格式之间 mtime 回退:旧A(mtime 更大)应命中
        val hitOld = OfflineStore.getUrl("https://x.com/old")!!
        assertArrayEquals("旧A".toByteArray(), hitOld.bytes)
        // 新档 capturedAt=5_000(墙上远早于旧档 mtime 9_000)也必须胜出
        OfflineStore.putUrl("https://x.com/old", "新档".toByteArray(), "text/html", "c.html", capturedAtMs = 5_000L)
        val hitNew = OfflineStore.getUrl("https://x.com/old")!!
        assertArrayEquals("capturedAt 段优先于 mtime,新档必须命中", "新档".toByteArray(), hitNew.bytes)
        assertEquals(5_000L, hitNew.capturedAt)
        assertEquals(1, dir.listFiles { f -> f.isFile && f.name.startsWith("$key.") }!!.size)
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
