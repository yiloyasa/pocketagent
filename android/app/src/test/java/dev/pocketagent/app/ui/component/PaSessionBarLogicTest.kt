package dev.pocketagent.app.ui.component

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 运行中会话 mini 条(方向 C N1)的判定层单测:可见性 / 主会话选择 / 角标计数 /
 * 状态细语兜底。全部纯函数,JVM 直跑,不 mock 任何组件。
 */
class PaSessionBarLogicTest {

    private fun session(
        id: String,
        sortKey: Long,
        title: String = "会话 $id",
        whisper: String = "运行 5 分钟",
        stoppable: Boolean = false,
    ) = PaRunningSessionUi(
        threadId = id,
        title = title,
        agentLabel = "Codex",
        statusWhisper = whisper,
        iconRes = 1,
        startedSortKey = sortKey,
        stoppable = stoppable,
    )

    // ---- 可见性:≥1 个运行中会话才常驻 ----

    @Test
    fun `可见性_无运行中会话不显示_有一个即显示`() {
        assertFalse(paSessionBarVisible(emptyList()))
        assertTrue(paSessionBarVisible(listOf(session("a", 1L))))
        assertTrue(paSessionBarVisible(listOf(session("a", 2L), session("b", 1L))))
    }

    // ---- 主会话:多会话并行取「最近启动」(sortKey 最大)----

    @Test
    fun `主会话_空列表为null`() {
        assertNull(paSessionBarPrimary(emptyList()))
    }

    @Test
    fun `主会话_单个即其本身`() {
        val only = session("only", 7L)
        assertEquals("only", paSessionBarPrimary(listOf(only))?.threadId)
    }

    @Test
    fun `主会话_多会话取启动键最大者`() {
        val primary = paSessionBarPrimary(
            listOf(session("old", 100L), session("new", 900L), session("mid", 500L)),
        )
        assertEquals("new", primary?.threadId)
    }

    @Test
    fun `主会话_启动键并列取列表先者_与调用方新到旧排序口径一致`() {
        val primary = paSessionBarPrimary(
            listOf(session("first", 500L), session("second", 500L)),
        )
        assertEquals("first", primary?.threadId)
    }

    // ---- 角标:总数口径,1 个不摆,>99 封顶 ----

    @Test
    fun `角标_零或一个不摆角标`() {
        assertNull(paSessionBarBadgeText(0))
        assertNull(paSessionBarBadgeText(1))
    }

    @Test
    fun `角标_两到九十九报数`() {
        assertEquals("2", paSessionBarBadgeText(2))
        assertEquals("10", paSessionBarBadgeText(10))
        assertEquals("99", paSessionBarBadgeText(99))
    }

    @Test
    fun `角标_超过九十九封顶99加`() {
        assertEquals("99+", paSessionBarBadgeText(100))
        assertEquals("99+", paSessionBarBadgeText(5000))
    }

    // ---- 状态细语:时间源缺席时兜底「正在执行…」----

    @Test
    fun `细语_空白兜底为正在执行_非空白原样透传`() {
        assertEquals("正在执行…", paSessionBarWhisper(""))
        assertEquals("正在执行…", paSessionBarWhisper("   "))
        assertEquals("运行 5 分钟", paSessionBarWhisper("运行 5 分钟"))
        assertEquals("最近活动 · 3 分钟前", paSessionBarWhisper("最近活动 · 3 分钟前"))
    }

    // ---- 展示模型默认值:stoppable 缺省为假(不摆假停止位)----

    @Test
    fun `展示模型_stoppable缺省为假`() {
        val s = PaRunningSessionUi("t", "标题", "Codex", "运行 1 分钟", 1, 1L)
        assertFalse(s.stoppable)
    }
}
