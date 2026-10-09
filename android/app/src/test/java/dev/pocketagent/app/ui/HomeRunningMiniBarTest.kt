package dev.pocketagent.app.ui

import dev.pocketagent.app.ui.component.paSessionBarPrimary
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.Thread
import pocketagent.v1.ThreadStatus

/**
 * 运行中会话 mini 条数据并集的单测(方向 C N1):daemon 快照 ∪ TurnSession 实时
 * 快照 → 展示模型。纯函数直跑,用 proto 默认实例造数据,不 mock 网络层。
 */
class HomeRunningMiniBarTest {

    private fun thread(
        id: String,
        title: String = "会话 $id",
        agentId: String = "codex",
        updatedAt: Long = 1_000L,
        turnStarted: Long? = null,
    ): Thread {
        val b = Thread.newBuilder()
            .setId(id)
            .setTitle(title)
            .setStatus(ThreadStatus.THREAD_STATUS_RUNNING)
            .setAgentId(agentId)
            .setUpdatedAtUnixMs(updatedAt)
        turnStarted?.let { b.setTurnStartedUnixMs(it) }
        return b.build()
    }

    @Test
    fun `daemon快照映射_标题归属与状态细语齐全`() {
        val sessions = buildPaRunningSessions(
            daemonRunning = listOf(thread("a", title = "修登录 Bug", agentId = "pi", updatedAt = 5_000L)),
            liveRunningIds = emptySet(),
            knownThreads = emptyList(),
            nowMs = 10_000_000L,
        )
        assertEquals(1, sessions.size)
        val s = sessions.single()
        assertEquals("a", s.threadId)
        assertEquals("修登录 Bug", s.title)
        assertEquals("Pi", s.agentLabel)
        // 起点缺席:细语回落"最近活动"(now-updated ≈ 2.77h → 「2 小时前」);排序键 = updatedAt
        assertEquals("最近活动 · 2 小时前", s.statusWhisper)
        assertEquals(5_000L, s.startedSortKey)
        // daemon 侧(电脑端在跑)没有手机端停止通道:不摆假停止
        assertFalse(s.stoppable)
    }

    @Test
    fun `live独有会话_从已知列表补全名片且可停`() {
        val sessions = buildPaRunningSessions(
            daemonRunning = emptyList(),
            liveRunningIds = setOf("live1"),
            knownThreads = listOf(thread("live1", title = "写周报", agentId = "zcode")),
            nowMs = 10_000_000L,
        )
        val s = sessions.single()
        assertEquals("live1", s.threadId)
        assertEquals("写周报", s.title)
        assertEquals("ZCode", s.agentLabel)
        // 手机端在跑(TurnSession 有状态):停止位亮出,接现有 cancelTurn 语义
        assertTrue(s.stoppable)
    }

    @Test
    fun `live独有且名单查无_未命名占位不编标题`() {
        val sessions = buildPaRunningSessions(
            daemonRunning = emptyList(),
            liveRunningIds = setOf("ghost"),
            knownThreads = emptyList(),
            nowMs = 42_000L,
        )
        val s = sessions.single()
        assertEquals("ghost", s.threadId)
        assertEquals("(未命名)", s.title)
        assertEquals("Codex", s.agentLabel) // agentId 缺省按 codex 归属
        assertEquals(42_000L, s.startedSortKey) // 无起点回退 updatedAt=now
        assertTrue(s.stoppable)
    }

    @Test
    fun `并集去重_同id以daemon快照为准但停止位随live`() {
        val sessions = buildPaRunningSessions(
            daemonRunning = listOf(thread("both", title = "daemon 视角标题", updatedAt = 9_000L)),
            liveRunningIds = setOf("both"),
            knownThreads = listOf(thread("both", title = "本地视角标题")),
            nowMs = 10_000_000L,
        )
        val s = sessions.single()
        assertEquals("both", s.threadId)
        assertEquals("daemon 视角标题", s.title) // 信息更全的一侧胜出
        assertTrue(s.stoppable) // 本机也在跑:停止位亮出
    }

    @Test
    fun `排序_启动键倒序_最近启动的做主会话`() {
        val sessions = buildPaRunningSessions(
            daemonRunning = listOf(
                thread("old", updatedAt = 1_000L),
                // 有真实起点:排序键取 turn_started,压过更晚的 updatedAt
                thread("started-early", updatedAt = 8_000L, turnStarted = 500L),
                thread("started-late", updatedAt = 2_000L, turnStarted = 9_500L),
            ),
            liveRunningIds = setOf("live-now"),
            knownThreads = listOf(thread("live-now", updatedAt = 3_000L, turnStarted = 9_000L)),
            nowMs = 10_000_000L,
        )
        // 启动键:started-late=9500 > live-now=9000 > old(updatedAt=1000) > started-early=500
        assertEquals(
            listOf("started-late", "live-now", "old", "started-early"),
            sessions.map { it.threadId },
        )
        assertEquals("started-late", paSessionBarPrimary(sessions)?.threadId)
    }

    @Test
    fun `空白id不入集_空输入得空表`() {
        assertTrue(buildPaRunningSessions(emptyList(), emptySet(), emptyList(), 0L).isEmpty())
        val sessions = buildPaRunningSessions(
            daemonRunning = listOf(thread("", title = "无 id 不显示")),
            liveRunningIds = setOf(""),
            knownThreads = emptyList(),
            nowMs = 1L,
        )
        assertTrue(sessions.isEmpty())
    }
}
