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
 *
 * 二版追加:按住拖动预览的纯决策 —— 手指位置↔刻度下标映射、手势状态机
 * (轻点=浮层/拖了=跳转/没拖=取消)、气泡 Y 钳制。这些错了的表现是"气泡串刻度/
 * 松手误跳/气泡出屏",同样不报错,全部单测钉死。
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

    // ==== 二版:按住拖动预览(手指↔下标映射 / 手势状态机 / 气泡定位)====

    // ---- 刻度几何(与 Canvas 绘制同一套数学的单源真值) ----

    @Test
    fun `刻度少时用自然步距并整体垂直居中`() {
        // tick=2, gap=7 → 自然步距 9;3 个刻度总高 2+9+9=20,容器 100 → 顶部 (100-20)/2=40
        val g = HistoryQuickNavLogic.tickGeometry(count = 3, heightPx = 100f, tickHeightPx = 2f, gapPx = 7f)
        assertEquals(9f, g.step, 1e-4f)
        assertEquals(40f, g.top, 1e-4f)
    }

    @Test
    fun `刻度多时整体压缩到满高`() {
        // 自然步距 9 放不下 20 个:压缩后首尾贴边,top=0,step=(100-2)/19
        val g = HistoryQuickNavLogic.tickGeometry(count = 20, heightPx = 100f, tickHeightPx = 2f, gapPx = 7f)
        assertEquals(0f, g.top, 1e-4f)
        assertEquals((100f - 2f) / 19f, g.step, 1e-4f)
        assertTrue(g.step < 9f)
    }

    @Test
    fun `单刻度与退化几何`() {
        // 单刻度:步距 0、垂直居中
        val g = HistoryQuickNavLogic.tickGeometry(1, 100f, 2f, 7f)
        assertEquals(0f, g.step, 1e-4f)
        assertEquals(49f, g.top, 1e-4f)
        // 容器比刻度还矮(极端小窗):不崩,顶部钳到 0
        val tiny = HistoryQuickNavLogic.tickGeometry(5, 1f, 2f, 7f)
        assertEquals(0f, tiny.top, 1e-4f)
    }

    // ---- 手指 Y ↔ 刻度下标映射 ----

    @Test
    fun `手指位置映射到最近刻度且越界钳制`() {
        val count = 5
        val h = 100f
        // 3 个刻度用不满 100 高:先拿几何算真实刻度中心再断言
        val g = HistoryQuickNavLogic.tickGeometry(count, h, 2f, 7f)
        fun centerY(i: Int) = g.top + i * g.step + 1f
        assertEquals(0, HistoryQuickNavLogic.indexAtY(centerY(0), count, h, 2f, 7f))
        assertEquals(1, HistoryQuickNavLogic.indexAtY(centerY(1), count, h, 2f, 7f))
        assertEquals(2, HistoryQuickNavLogic.indexAtY(centerY(2), count, h, 2f, 7f))
        // 两刻度中点:四舍五入到更近的那个(中点 .5 向上取整 → 高位)
        assertEquals(1, HistoryQuickNavLogic.indexAtY((centerY(0) + centerY(1)) / 2f, count, h, 2f, 7f))
        // 越界(拖出条外)钳到首尾 —— 拖到顶/底也要有确定的落点
        assertEquals(0, HistoryQuickNavLogic.indexAtY(-999f, count, h, 2f, 7f))
        assertEquals(count - 1, HistoryQuickNavLogic.indexAtY(999f, count, h, 2f, 7f))
    }

    @Test
    fun `单刻度恒 0 空刻度为 -1`() {
        assertEquals(0, HistoryQuickNavLogic.indexAtY(50f, 1, 100f, 2f, 7f))
        assertEquals(-1, HistoryQuickNavLogic.indexAtY(50f, 0, 100f, 2f, 7f))
        assertEquals(-1, HistoryQuickNavLogic.indexAtY(50f, -3, 100f, 2f, 7f))
    }

    // ---- 手势状态机:轻点=浮层 / 拖了=跳转 / 没拖=取消 ----

    private val geoHeight = 200f

    private fun released(
        g: HistoryQuickNavLogic.NavGesture,
        count: Int = 5,
        cancelSlopPx: Float = 20f,
    ): HistoryQuickNavLogic.NavRelease? = HistoryQuickNavLogic.released(
        g, count, geoHeight, tickHeightPx = 2f, gapPx = 7f, cancelSlopPx = cancelSlopPx,
    )

    @Test
    fun `预览出现前松手视为轻点_开浮层`() {
        val g = HistoryQuickNavLogic.pressed(100f)
        // 窗内微动(未超 slop)仍是 PRESSED,不进预览
        val nudged = HistoryQuickNavLogic.moved(g, 104f, dragSlopPx = 8f)
        assertEquals(HistoryQuickNavLogic.NavGesturePhase.PRESSED, nudged.phase)
        assertEquals(HistoryQuickNavLogic.NavRelease.OpenPanel, released(nudged))
    }

    @Test
    fun `按住超时进入预览_没拖就松手视为取消`() {
        var g = HistoryQuickNavLogic.pressed(100f)
        g = HistoryQuickNavLogic.held(g)
        assertEquals(HistoryQuickNavLogic.NavGesturePhase.PREVIEW, g.phase)
        assertEquals(HistoryQuickNavLogic.NavRelease.Cancel, released(g))
        // 拖走又拖回起点附近(位移 < 阈值)同样是取消 —— "回到起点位置附近"
        g = HistoryQuickNavLogic.moved(g, 300f, 8f)
        g = HistoryQuickNavLogic.moved(g, 112f, 8f)
        assertEquals(HistoryQuickNavLogic.NavRelease.Cancel, released(g))
    }

    @Test
    fun `拖动后松手跳到手指停住处的刻度`() {
        var g = HistoryQuickNavLogic.pressed(100f)
        // 立刻快扫(未等按住超时):超 slop 直接进预览 —— 拖动不等人
        g = HistoryQuickNavLogic.moved(g, 100f + 50f, dragSlopPx = 8f)
        assertEquals(HistoryQuickNavLogic.NavGesturePhase.PREVIEW, g.phase)
        g = HistoryQuickNavLogic.moved(g, 160f, 8f)
        val r = released(g, count = 10)
        assertTrue(r is HistoryQuickNavLogic.NavRelease.Jump)
        assertEquals(
            HistoryQuickNavLogic.indexAtY(160f, 10, geoHeight, 2f, 7f),
            (r as HistoryQuickNavLogic.NavRelease.Jump).entryIndex,
        )
    }

    @Test
    fun `预览中的位移恰好等于阈值不算没拖`() {
        // 边界:|dy| == cancelSlop 走跳转(条件是"< 阈值"才取消)
        var g = HistoryQuickNavLogic.pressed(100f)
        g = HistoryQuickNavLogic.held(g)
        g = HistoryQuickNavLogic.moved(g, 120f, 8f)
        val r = released(g, count = 10)
        assertTrue(r is HistoryQuickNavLogic.NavRelease.Jump)
    }

    @Test
    fun `空闲态松手无可裁定_作废回到空闲`() {
        val idle = HistoryQuickNavLogic.NavGesture()
        assertEquals(null, released(idle))
        // 作废(手势被打断/取消)后回到 IDLE,一切归零
        var g = HistoryQuickNavLogic.pressed(100f)
        g = HistoryQuickNavLogic.held(g)
        g = HistoryQuickNavLogic.canceled()
        assertEquals(HistoryQuickNavLogic.NavGesturePhase.IDLE, g.phase)
        // IDLE 上 moved/held 都是空转,不会"复活"预览
        assertEquals(HistoryQuickNavLogic.NavGesturePhase.IDLE, HistoryQuickNavLogic.moved(g, 50f, 8f).phase)
        assertEquals(HistoryQuickNavLogic.NavGesturePhase.IDLE, HistoryQuickNavLogic.held(g).phase)
    }

    // ---- 预览气泡 Y 定位(8dp 让位 + 容器内钳制) ----

    @Test
    fun `气泡顶随手指让位且钳制在容器内`() {
        val bubbleH = 120f
        val containerH = 800f
        val yield = 8f
        // 居中段:top = 手指 + 让位 - 半高
        assertEquals(400f + yield - bubbleH / 2f, HistoryQuickNavLogic.bubbleTopY(400f, bubbleH, containerH, yield), 1e-4f)
        // 顶到头:不出容器上缘
        assertEquals(0f, HistoryQuickNavLogic.bubbleTopY(0f, bubbleH, containerH, yield), 1e-4f)
        // 落到底:不出容器下缘
        assertEquals(containerH - bubbleH, HistoryQuickNavLogic.bubbleTopY(799f, bubbleH, containerH, yield), 1e-4f)
        // 容器比气泡矮(极端小窗):整体居中且不为负
        assertEquals(0f, HistoryQuickNavLogic.bubbleTopY(50f, bubbleH, 100f, yield), 1e-4f)
    }
}
