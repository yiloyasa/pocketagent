package dev.pocketagent.app.ui.component

import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.File

/** 预览内容缓存的失效语义与 LRU(审查 C2 / O3)。 */
class PreviewCacheTest {

    @get:Rule
    val tmp = TemporaryFolder()

    private lateinit var dir: File

    @Before
    fun setUp() {
        dir = tmp.newFolder("pv")
        PreviewCache.initForTest(dir)
    }

    /** 与 PreviewCache.key 同算法:md5(path\0size\0mtime) */
    private fun cacheFile(path: String, size: Long, mtime: Long): File {
        val md = java.security.MessageDigest.getInstance("MD5")
        val name = md.digest("$path\u0000$size\u0000$mtime".toByteArray(Charsets.UTF_8))
            .joinToString("") { "%02x".format(it.toInt() and 0xff) }
        return File(dir, name)
    }

    @Test
    fun hitsOnlyWhenPathSizeAndMtimeMatch() {
        val bytes = "hello".toByteArray()
        PreviewCache.put("/a/b.txt", 5, 1000, bytes)

        assertArrayEquals(bytes, PreviewCache.get("/a/b.txt", 5, 1000))
        // 电脑端文件被编辑 → mtime 变 → 必须 miss,否则会渲染出过期内容
        assertNull(PreviewCache.get("/a/b.txt", 5, 1001))
        assertNull(PreviewCache.get("/a/b.txt", 6, 1000))
        assertNull(PreviewCache.get("/a/c.txt", 5, 1000))
    }

    @Test
    fun keyIsPlainHexWithoutSignedByteArtifacts() {
        PreviewCache.put("/中文/文件.pdf", 12, 34, ByteArray(12))
        // 负 Byte 直接格式化会产生 ffffff80 之类的 8 位串,长度必然不是 32
        val f = cacheFile("/中文/文件.pdf", 12, 34)
        assertEquals(32, f.name.length)
        assertTrue(f.isFile)
    }

    @Test
    fun returnsNullWhenMissing() {
        assertNull(PreviewCache.get("/none", 1, 1))
    }

    @Test
    fun touchesOnHitSoHotEntriesSurviveLru() {
        PreviewCache.put("/a", 1, 1, ByteArray(10))
        val f = cacheFile("/a", 1, 1)
        assertTrue(f.setLastModified(System.currentTimeMillis() - 500_000))

        assertNotNull(PreviewCache.get("/a", 1, 1))
        assertTrue("命中后应 touch", f.lastModified() > System.currentTimeMillis() - 5_000)
    }

    @Test
    fun skipsEntriesAboveSingleFileLimit() {
        val origin = PreviewCache.maxEntryBytes
        PreviewCache.maxEntryBytes = 10
        try {
            PreviewCache.put("/big", 100, 1, ByteArray(100))
            assertEquals(0L, PreviewCache.usage())
            assertNull(PreviewCache.get("/big", 100, 1))
        } finally {
            PreviewCache.maxEntryBytes = origin
        }
    }

    @Test
    fun evictsOldestWhenOverBudget() {
        val origin = PreviewCache.maxTotalBytes
        PreviewCache.maxTotalBytes = 100
        try {
            val base = System.currentTimeMillis() - 600_000
            listOf("a", "b").forEachIndexed { i, n ->
                PreviewCache.put("/$n", 40, i.toLong(), ByteArray(40))
                val f = cacheFile("/$n", 40, i.toLong())
                assertTrue("缓存文件应存在: $n", f.isFile)
                assertTrue("应能设置 LRU 时间: $n", f.setLastModified(base + i * 60_000L))
            }
            PreviewCache.put("/c", 40, 2, ByteArray(40)) // 120 > 100 → 淘汰最旧

            val names = dir.listFiles() ?: emptyArray()
            val dump = if (names.isEmpty()) "(空)" else {
                names.joinToString { "${it.name.take(6)}=${it.length()}@${it.lastModified()}" }
            }
            assertNull("最旧的 a 应被淘汰 / $dump", PreviewCache.get("/a", 40, 0))
            assertNotNull("b 应保留 / $dump", PreviewCache.get("/b", 40, 1))
            assertNotNull("c 应保留 / $dump", PreviewCache.get("/c", 40, 2))
            assertTrue(PreviewCache.usage() <= 100)
        } finally {
            PreviewCache.maxTotalBytes = origin
        }
    }

    @Test
    fun clearReleasesSpace() {
        PreviewCache.put("/a", 30, 1, ByteArray(30))
        assertEquals(30L, PreviewCache.usage())

        assertEquals(30L, PreviewCache.clear())
        assertEquals(0L, PreviewCache.usage())
    }
}
