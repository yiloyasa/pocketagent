package dev.pocketagent.app.ui.component

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.File

/** 离线快照的落盘/回读/限额/淘汰逻辑(审查 C3 / O1 / O2)。 */
class ThreadSnapshotTest {

    @get:Rule
    val tmp = TemporaryFolder()

    private lateinit var dir: File

    @Before
    fun setUp() {
        dir = tmp.newFolder("snap")
        ThreadSnapshot.initForTest(dir)
    }

    /** 与 ThreadSnapshot.msgFileName 同算法:md5(threadId) + .json */
    private fun msgFile(threadId: String): File {
        val md = java.security.MessageDigest.getInstance("MD5")
        val name = md.digest(threadId.toByteArray(Charsets.UTF_8))
            .joinToString("") { "%02x".format(it.toInt() and 0xff) } + ".json"
        return File(dir, name)
    }

    @Test
    fun agoTextFormatsRelativeTime() {
        val now = System.currentTimeMillis()
        assertEquals("刚刚", agoText(now))
        assertEquals("刚刚", agoText(now - 59_000))
        assertEquals("1 分钟前", agoText(now - 60_000))
        assertEquals("59 分钟前", agoText(now - 59 * 60_000))
        assertEquals("1 小时前", agoText(now - 3_600_000))
        assertEquals("1 天前", agoText(now - 86_400_000))
        // 时钟回拨/未来时间戳不能显示成负数
        assertEquals("刚刚", agoText(now + 60_000))
    }

    @Test
    fun keepsOnlyMostRecentMessages() {
        val msgs = (0 until 100).map { ThreadSnapshot.Msg("user", "m$it") }
        ThreadSnapshot.saveMessages("t1", msgs)

        val loaded = ThreadSnapshot.loadMessages("t1")!!
        assertEquals(60, loaded.size)
        assertEquals("m40", loaded.first().text)
        assertEquals("m99", loaded.last().text)
    }

    @Test
    fun trimsMessagesByByteBudget() {
        val fat = "x".repeat(5_000) // 单条成本约 10KB
        val msgs = (0 until 40).map { ThreadSnapshot.Msg("user", "$it:$fat") }
        ThreadSnapshot.saveMessages("t1", msgs)

        val loaded = ThreadSnapshot.loadMessages("t1")!!
        assertEquals(20, loaded.size) // 200KB / 10KB
        assertTrue("应保留最新的", loaded.last().text.startsWith("39:"))
    }

    @Test
    fun returnsNullWhenNoSnapshot() {
        // null = 无快照;空 list = 有快照但为空 —— 两者语义不同,不能混
        assertNull(ThreadSnapshot.loadMessages("nope"))
    }

    @Test
    fun roundTripsThreadRows() {
        val rows = listOf(
            ThreadSnapshot.Row("a", "标题", 1_700_000_000_000, "G:/ws", "ct-1", false),
            ThreadSnapshot.Row("b", "", 0, "", "", true),
        )
        ThreadSnapshot.saveThreads(rows)

        assertEquals(rows, ThreadSnapshot.loadThreads())
        assertTrue(ThreadSnapshot.threadsSnapshotAt() > 0)
        assertNull(ThreadSnapshot.loadMessages("a"))
    }

    @Test
    fun evictsOldestThreadSnapshots() {
        val base = System.currentTimeMillis() - 1_000_000
        repeat(35) { i ->
            ThreadSnapshot.saveMessages("t$i", listOf(ThreadSnapshot.Msg("user", "hi")))
            msgFile("t$i").setLastModified(base + i * 1_000L)
        }
        ThreadSnapshot.saveMessages("t35", listOf(ThreadSnapshot.Msg("user", "hi")))

        val kept = (0..35).filter { msgFile("t$it").isFile }
        assertEquals(30, kept.size)
        assertEquals((6..35).toList(), kept) // 最旧的 t0..t5 被淘汰
    }

    @Test
    fun doesNotPruneWhenLiveListIsEmpty() {
        // 空列表多半是"没拉到",此时绝不能删用户的模型选择
        ThreadSnapshot.pruneStalePrefs(emptySet())
        assertFalse(File(dir, "seen.json").isFile)
    }
}
