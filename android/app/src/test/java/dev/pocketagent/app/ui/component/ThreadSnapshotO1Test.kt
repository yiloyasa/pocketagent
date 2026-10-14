package dev.pocketagent.app.ui.component

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.File

/**
 * P1-8 离线会话快照(O1 档)补充测试:
 *  - 阶段 D 将依赖的四个稳定 API(saveThreadsSnapshot / readThreadsSnapshot /
 *    readThreadSnapshot / clearAll)的往返行为;
 *  - 双层限额之二:总量上限 5MB(条数上限 50 在 [ThreadSnapshotTest] 里验);
 *  - 损坏文件容错:读到坏 JSON 不崩、当空处理,且不污染后续写入;
 *  - 原子写:正本落盘后不留 .tmp 半截文件;
 *  - 列表级文件(threads.json / seen.json)不参与 per-thread 淘汰。
 */
class ThreadSnapshotO1Test {

    @get:Rule
    val tmp = TemporaryFolder()

    private lateinit var dir: File

    @Before
    fun setUp() {
        dir = tmp.newFolder("snap-o1")
        ThreadSnapshot.initForTest(dir)
    }

    // ---------------- 稳定 API(阶段 D 契约) ----------------

    @Test
    fun stableApi_roundTripsListRowsAndMessages() {
        val rows = listOf(
            ThreadSnapshot.Row("a", "标题 A", 1_700_000_000_000, "G:/ws", "ct-a", false, "sec1"),
            ThreadSnapshot.Row("b", "标题 B", 1_700_000_001_000, "", "", true, ""),
        )
        ThreadSnapshot.saveThreadsSnapshot(rows)
        assertEquals("列表快照应原样往返", rows, ThreadSnapshot.readThreadsSnapshot())

        val msgs = listOf(ThreadSnapshot.Msg("user", "你好"), ThreadSnapshot.Msg("assistant", "在"))
        ThreadSnapshot.saveMessages("a", msgs)
        // savedAtMs(2026-09-30)是写入端打的落盘时间:往返比较 role/text 原样,时间戳应为正
        val roundTrip = ThreadSnapshot.readThreadSnapshot("a")!!
        assertEquals("单线程快照应原样往返", msgs.map { it.role to it.text }, roundTrip.map { it.role to it.text })
        assertTrue("落盘时间戳应为正", roundTrip.all { it.savedAtMs > 0L })
        assertNull("无快照的线程返回 null", ThreadSnapshot.readThreadSnapshot("nope"))
    }

    @Test
    fun clearAll_freesSpaceAndEmptiesSnapshots() {
        ThreadSnapshot.saveThreadsSnapshot(listOf(ThreadSnapshot.Row("a", "t", 1, "", "", false)))
        ThreadSnapshot.saveMessages("a", listOf(ThreadSnapshot.Msg("user", "hi")))
        assertTrue(ThreadSnapshot.usage() > 0)

        val freed = ThreadSnapshot.clearAll()
        assertTrue("应报告释放的字节数", freed > 0)
        assertEquals(0L, ThreadSnapshot.usage())
        assertTrue(ThreadSnapshot.readThreadsSnapshot().isEmpty())
        assertNull(ThreadSnapshot.readThreadSnapshot("a"))
    }

    // ---------------- 总量上限 5MB ----------------

    @Test
    fun totalBudget_evictsOldestUntilWithinFiveMB() {
        val fat = "x".repeat(200_000) // 单条 20 万字符,文件约 200KB,线程内不触发 400KB 上限
        val base = System.currentTimeMillis() - 1_000_000
        // 25 个 × ~200KB ≈ 5.0MB:低于总量上限,一个都不该被淘汰
        repeat(25) { i ->
            ThreadSnapshot.saveMessages("t$i", listOf(ThreadSnapshot.Msg("user", fat)))
            msgFile(dir, "t$i").setLastModified(base + i * 1_000L)
        }
        val beforeTotal = ThreadSnapshot.usage()
        assertEquals("低于上限时不得淘汰", 25, (0..24).count { msgFile(dir, "t$it").isFile })
        assertTrue("前置条件:应接近 5MB", beforeTotal in 4_900_000L..5_242_880L)

        // 再加 3 个 → 总量超 5MB → 按 LRU 淘汰最旧的,直到压回上限内
        repeat(3) { j ->
            ThreadSnapshot.saveMessages("t${25 + j}", listOf(ThreadSnapshot.Msg("user", fat)))
        }

        assertFalse("最旧的 t0 应被总量淘汰", msgFile(dir, "t0").isFile)
        assertFalse("次旧的 t1 应被总量淘汰", msgFile(dir, "t1").isFile)
        assertTrue("较新的 t2 应保留", msgFile(dir, "t2").isFile)
        val total = dir.listFiles()?.filter { it.name.endsWith(".json") }?.sumOf { it.length() } ?: 0L
        assertTrue("淘汰后总量必须回到 5MB 内:$total", total <= 5L * 1024 * 1024)
    }

    // ---------------- 损坏文件容错 ----------------

    @Test
    fun corruptJson_isTreatedAsEmpty_andDoesNotCrashLaterWrites() {
        // 列表文件坏 → 空列表;消息文件坏 → null(与"有快照但为空"区分)
        File(dir, "threads.json").writeText("{ this is not json")
        assertTrue(ThreadSnapshot.readThreadsSnapshot().isEmpty())

        ThreadSnapshot.saveMessages("t1", listOf(ThreadSnapshot.Msg("user", "hi")))
        val f = msgFile(dir, "t1")
        f.writeText("]]] broken")
        assertNull("坏消息文件应视为无快照", ThreadSnapshot.readThreadSnapshot("t1"))

        // 损坏不能污染后续写:重新保存后应能正常读回
        ThreadSnapshot.saveThreadsSnapshot(listOf(ThreadSnapshot.Row("a", "t", 1, "", "", false)))
        assertEquals(1, ThreadSnapshot.readThreadsSnapshot().size)
        ThreadSnapshot.saveMessages("t1", listOf(ThreadSnapshot.Msg("user", "恢复")))
        assertEquals("恢复", ThreadSnapshot.readThreadSnapshot("t1")?.single()?.text)
    }

    // ---------------- 原子写 ----------------

    @Test
    fun atomicWrite_leavesNoTmpFilesBehind() {
        ThreadSnapshot.saveThreadsSnapshot(listOf(ThreadSnapshot.Row("a", "t", 1, "", "", false)))
        ThreadSnapshot.saveMessages("a", listOf(ThreadSnapshot.Msg("user", "hi")))
        ThreadSnapshot.saveMessages("b", listOf(ThreadSnapshot.Msg("user", "hi")))

        val leftovers = dir.listFiles()?.filter { it.name.endsWith(".tmp") } ?: emptyList()
        assertTrue("正本写完不应残留 .tmp:$leftovers", leftovers.isEmpty())
        assertEquals(1, ThreadSnapshot.readThreadsSnapshot().size)
        assertNotNull(ThreadSnapshot.readThreadSnapshot("a"))
    }

    // ---------------- 列表级文件不参与 per-thread 淘汰 ----------------

    @Test
    fun listAndSeenFiles_survivePerThreadEviction() {
        // 造出会触发条数淘汰的场景(50 上限),列表与 seen 文件必须安然无恙
        val base = System.currentTimeMillis() - 1_000_000
        ThreadSnapshot.saveThreadsSnapshot(
            (0 until 10).map { ThreadSnapshot.Row("r$it", "t$it", 1L, "", "", false) },
        )
        File(dir, "seen.json").writeText("{}")
        repeat(52) { i ->
            ThreadSnapshot.saveMessages("t$i", listOf(ThreadSnapshot.Msg("user", "hi")))
            msgFile(dir, "t$i").setLastModified(base + i * 1_000L)
        }
        assertTrue("threads.json 不得被淘汰", File(dir, "threads.json").isFile)
        assertTrue("seen.json 不得被当快照淘汰", File(dir, "seen.json").isFile)
        assertEquals("per-thread 快照仍应守 50 上限", 50, (0..51).count { msgFile(dir, "t$it").isFile })
    }
}
