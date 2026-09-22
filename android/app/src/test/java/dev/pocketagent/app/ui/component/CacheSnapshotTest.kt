package dev.pocketagent.app.ui.component

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.File

/**
 * 会话快照「最近 N 轮副本」契约(需求②):
 *  - 覆盖而非合并:同一会话新一次拉取整体覆盖旧快照,不拼接、不残留;
 *  - 只含最近 N 轮:落盘的是调用方按 rounds 收窄后传入的那批,非全量。
 * 全程走临时目录,不碰真机/网络。
 */
class CacheSnapshotTest {

    @get:Rule
    val tmp = TemporaryFolder()

    private lateinit var dir: File

    @Before
    fun setUp() {
        dir = tmp.newFolder("snap")
        ThreadSnapshot.initForTest(dir)
    }

    @Test
    fun `new fetch overwrites the old snapshot for the same thread`() {
        // 先按较早的"轮数"落一份(旧的 3 条)
        ThreadSnapshot.saveMessages(
            "t1",
            listOf(
                ThreadSnapshot.Msg("user", "old-1"),
                ThreadSnapshot.Msg("assistant", "old-2"),
                ThreadSnapshot.Msg("user", "old-3"),
            ),
        )
        // 稍后按新的轮数重拉(新的 2 条)—— 应整体覆盖,不留旧尾巴
        ThreadSnapshot.saveMessages(
            "t1",
            listOf(
                ThreadSnapshot.Msg("user", "new-1"),
                ThreadSnapshot.Msg("assistant", "new-2"),
            ),
        )
        val loaded = ThreadSnapshot.loadMessages("t1")!!
        assertEquals(2, loaded.size)
        assertEquals("new-1", loaded[0].text)
        assertEquals("new-2", loaded[1].text)
    }

    @Test
    fun `snapshot reflects only the fetched recent N rounds, not the full history`() {
        // 模拟:全量历史 100 轮,但按设置只拉最近 10 轮 —— 落盘的只有这 10 条,非全量
        val full = (0 until 100).map { ThreadSnapshot.Msg("user", "h$it") }
        val recent10 = full.takeLast(10)
        ThreadSnapshot.saveMessages("t2", recent10)

        val loaded = ThreadSnapshot.loadMessages("t2")!!
        assertEquals(10, loaded.size)
        assertEquals("h90", loaded.first().text) // 保留的是最新的
        assertTrue("不应包含被 rounds 收窄掉的旧历史", loaded.none { it.text == "h0" })
    }
}
