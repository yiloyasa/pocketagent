package dev.pocketagent.app.ui.component

import dev.pocketagent.app.ui.Attachment
import dev.pocketagent.app.ui.ChatItem
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 历史消息快速查看(2026-09-30,视频样板交互)纯逻辑钉子:
 * 刻度=每条用户消息;标题=首行截断;摘要=其余内容压行截断;当前高亮=
 * 视口顶部最近滚过的用户消息。反转后的表现是"刻度错位/浮层串行",不报错,
 * 故先钉死(与 HistoryRoundsLogic 同一测试口径)。
 */
class HistoryQuickNavLogicTest {

    private fun user(text: String) = ChatItem(role = "user", text = text)
    private fun assistant(text: String) = ChatItem(role = "assistant", text = text)

    // ---- 标题/摘要截断 ----

    @Test
    fun `标题取首行并去空白`() {
        assertEquals("停掉的 deamon 进程", HistoryQuickNavLogic.titleOf("  停掉的 deamon 进程\n来停掉 daemon 进程"))
        assertEquals("单行原样", HistoryQuickNavLogic.titleOf("单行原样"))
    }

    @Test
    fun `标题超长以省略号收尾`() {
        val long = "很".repeat(60)
        val got = HistoryQuickNavLogic.titleOf(long)
        assertEquals(HistoryQuickNavLogic.TITLE_MAX_CHARS + 1, got.length) // 40 字 + …
        assertTrue(got.endsWith("…"))
    }

    @Test
    fun `摘要取首行之后的剩余内容并压成一行`() {
        val got = HistoryQuickNavLogic.summaryOf("标题行\n第二行内容\n第三行内容")
        assertEquals("第二行内容 第三行内容", got)
    }

    @Test
    fun `单行消息摘要为空`() {
        assertEquals("", HistoryQuickNavLogic.summaryOf("只有一行"))
    }

    @Test
    fun `摘要超长截断且带省略号`() {
        val got = HistoryQuickNavLogic.summaryOf("t\n" + "很".repeat(80))
        assertEquals(HistoryQuickNavLogic.SUMMARY_MAX_CHARS + 1, got.length)
        assertTrue(got.endsWith("…"))
    }

    @Test
    fun `truncate 边界`() {
        assertEquals("", HistoryQuickNavLogic.truncate("abc", 0))
        assertEquals("abc", HistoryQuickNavLogic.truncate("  abc  ", 10))
        assertEquals("ab…", HistoryQuickNavLogic.truncate("abc", 2))
    }

    // ---- 刻度条目提取 ----

    @Test
    fun `只有 user 消息进刻度且 listIndex 计入头部偏移`() {
        val items = listOf(
            user("第一条指令"),
            assistant("回复一"),
            assistant("回复二"),
            user("第二条指令\n补充说明"),
        )
        val entries = HistoryQuickNavLogic.entriesOf(items, listOffset = 1)
        assertEquals(2, entries.size)
        assertEquals(0, entries[0].itemIndex)
        assertEquals(1, entries[0].listIndex) // 0 + 头部「加载更早」1 位
        assertEquals("第一条指令", entries[0].title)
        assertEquals(3, entries[1].itemIndex)
        assertEquals(4, entries[1].listIndex)
        assertEquals("第二条指令", entries[1].title)
        assertEquals("补充说明", entries[1].summary)
    }

    @Test
    fun `偏移为 0 时 listIndex 等于 itemIndex`() {
        val items = listOf(user("a"), user("b"))
        val entries = HistoryQuickNavLogic.entriesOf(items, listOffset = 0)
        assertEquals(listOf(0, 1), entries.map { it.listIndex })
    }

    @Test
    fun `纯附件消息标题取首个附件名`() {
        val items = listOf(
            ChatItem(
                role = "user", text = "",
                attachments = listOf(
                    Attachment(name = "log.txt", pcPath = "/tmp/log.txt", size = 128),
                    Attachment(name = "shot.png", pcPath = "/tmp/shot.png", size = 64),
                ),
            ),
        )
        val entries = HistoryQuickNavLogic.entriesOf(items, listOffset = 0)
        assertEquals(1, entries.size)
        assertEquals("log.txt", entries[0].title)
        assertEquals("共 2 个附件", entries[0].summary)
    }

    @Test
    fun `既无文本也无附件的占位消息不进刻度`() {
        val items = listOf(ChatItem(role = "user", text = ""), user("真消息"))
        val entries = HistoryQuickNavLogic.entriesOf(items, listOffset = 0)
        assertEquals(1, entries.size)
        assertEquals("真消息", entries[0].title)
    }

    @Test
    fun `空时间线无刻度`() {
        assertTrue(HistoryQuickNavLogic.entriesOf(emptyList(), 0).isEmpty())
    }

    // ---- 当前可见位置高亮 ----

    @Test
    fun `当前刻度取视口顶部最近滚过的用户消息`() {
        val items = listOf(user("a"), assistant("x"), user("b"), user("c"))
        val entries = HistoryQuickNavLogic.entriesOf(items, listOffset = 0)
        // listIndex: a=0, b=2, c=3
        assertEquals(-1, HistoryQuickNavLogic.currentIndexOf(entries, firstVisibleListIndex = 0 - 1))
        assertEquals(0, HistoryQuickNavLogic.currentIndexOf(entries, firstVisibleListIndex = 0))
        assertEquals(0, HistoryQuickNavLogic.currentIndexOf(entries, firstVisibleListIndex = 1))
        assertEquals(1, HistoryQuickNavLogic.currentIndexOf(entries, firstVisibleListIndex = 2))
        assertEquals(2, HistoryQuickNavLogic.currentIndexOf(entries, firstVisibleListIndex = 3))
        assertEquals(2, HistoryQuickNavLogic.currentIndexOf(entries, firstVisibleListIndex = 99))
    }

    @Test
    fun `无刻度时当前下标为 -1`() {
        assertEquals(-1, HistoryQuickNavLogic.currentIndexOf(emptyList(), 5))
    }
}
