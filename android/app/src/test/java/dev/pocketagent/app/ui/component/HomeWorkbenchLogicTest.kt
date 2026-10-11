package dev.pocketagent.app.ui.component

import dev.pocketagent.app.ui.AgentInfo
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 首页工作台纯逻辑的单测(首屏效率轮):骨架门控、三类空态文案、磁贴三态。
 *
 * 【口径红线】
 *  - 骨架只在"已连接 + 首次加载未落定 + 无错误"时出现 —— 失败/断连时骨架就是撒谎;
 *  - 空态必须"真实状态说明 + 去向按钮"成对出现,且不出现与当前状态相悖的文案
 *    (旧实现已连接时还写"连接电脑后…",这里钉死不再回潮);
 *  - 磁贴三态(可用/接入中/未安装)语义由状态词承担,不单靠灰度。
 */
class HomeWorkbenchLogicTest {

    @Test fun `ordinary phone preserves four agent columns`() {
        assertEquals(4, homeAgentGridColumns(360f, 1f))
    }

    @Test fun `narrow phone reduces columns instead of squeezing labels`() {
        assertEquals(3, homeAgentGridColumns(288f, 1f))
    }

    @Test fun `large fonts get wider tiles`() {
        assertEquals(2, homeAgentGridColumns(328f, 1.5f))
        assertEquals(1, homeAgentGridColumns(288f, 2f))
    }

    @Test fun `wide layouts preserve visual grid and pathological inputs stay safe`() {
        assertEquals(4, homeAgentGridColumns(1000f, 1f))
        assertEquals(1, homeAgentGridColumns(0f, 1f))
        assertEquals(1, homeAgentGridColumns(Float.NaN, 1f))
        assertEquals(4, homeAgentGridColumns(360f, Float.NaN))
    }

    // ---- homeShowsRecentSkeleton:骨架门控 ----

    @Test
    fun `已连接且首次加载未回且无错误_显示骨架`() {
        assertTrue(homeShowsRecentSkeleton(connected = true, firstLoadDone = false, loadError = null))
    }

    @Test
    fun `首次加载已落定_不再出骨架_刷新不闪骨架`() {
        assertEquals(false, homeShowsRecentSkeleton(connected = true, firstLoadDone = true, loadError = null))
    }

    @Test
    fun `加载失败_骨架退场改走错误横幅`() {
        assertEquals(
            false,
            homeShowsRecentSkeleton(connected = true, firstLoadDone = false, loadError = "首页数据加载失败"),
        )
    }

    @Test
    fun `未连接_绝不出加载骨架`() {
        assertEquals(false, homeShowsRecentSkeleton(connected = false, firstLoadDone = false, loadError = null))
    }

    // ---- homeRecentEmptyCopy:最近会话空态区分两种事实 ----

    @Test
    fun `本机没有会话_空态引导开第一单`() {
        val copy = homeRecentEmptyCopy(hasAnyRecent = false, agentLabel = "Codex")
        assertEquals("还没有会话", copy.title)
        assertEquals("开始新任务", copy.actionLabel)
    }

    @Test
    fun `有会话但被当前Agent过滤没了_说明过滤口径_不冒充没有会话`() {
        val copy = homeRecentEmptyCopy(hasAnyRecent = true, agentLabel = "Pi")
        assertEquals("「Pi」还没有会话", copy.title)
        // 副文讲清"只显示当前 Agent 的会话",与旧实现"连接电脑后…"相悖文案划清界限
        assertTrue(copy.hint.contains("当前 Agent"))
        assertEquals("开始新任务", copy.actionLabel)
    }

    // ---- homeAgentGridNote:无可用 agent 的空态注记 ----

    @Test
    fun `至少一个可用_不摆空态注记`() {
        assertNull(homeAgentGridNote(availableCount = 1))
        assertNull(homeAgentGridNote(availableCount = 7))
    }

    @Test
    fun `零可用_给真实原因与去向`() {
        val note = homeAgentGridNote(availableCount = 0)
        assertTrue(note != null)
        note!!
        assertTrue(note.title.contains("暂无可用 Agent"))
        assertTrue(note.hint.contains("重启 daemon"))
        assertEquals("查看管理", note.actionLabel)
    }

    // ---- homeDiscoveryEmptyCopy:发现主机列表 ----

    @Test
    fun `搜索中不是空态_交给骨架`() {
        assertNull(homeDiscoveryEmptyCopy(searching = true))
    }

    @Test
    fun `搜完没有_给真实说明与手动添加去向`() {
        val copy = homeDiscoveryEmptyCopy(searching = false)!!
        assertEquals("暂无发现的主机", copy.title)
        assertEquals("手动输入地址", copy.actionLabel)
    }

    // ---- homeAgentTileState / homeAgentTileLabel:磁贴三态 ----

    private fun agent(id: String, available: Boolean) =
        AgentInfo(id, id, "stdio", "副文", iconRes = 0, available = available)

    @Test
    fun `可用磁贴_全彩态讲连接方式`() {
        val a = agent("codex", available = true)
        assertEquals(HomeAgentTileState.AVAILABLE, homeAgentTileState(a))
        assertEquals("stdio", homeAgentTileLabel(HomeAgentTileState.AVAILABLE, a.method))
    }

    @Test
    fun `展示目录占位_接入中态`() {
        val a = agent("opencode", available = false)
        assertEquals(HomeAgentTileState.INTEGRATING, homeAgentTileState(a))
        assertEquals("接入中", homeAgentTileLabel(HomeAgentTileState.INTEGRATING, a.method))
    }

    @Test
    fun `能力驱动型不可用_电脑端未安装态`() {
        val a = agent("pi", available = false)
        assertEquals(HomeAgentTileState.NOT_INSTALLED, homeAgentTileState(a))
        assertEquals("电脑端未安装", homeAgentTileLabel(HomeAgentTileState.NOT_INSTALLED, a.method))
    }

    @Test
    fun `daemon新报的未知agent_可用则全彩_不可用按未安装口径`() {
        assertEquals(
            HomeAgentTileState.AVAILABLE,
            homeAgentTileState(agent("newagent", available = true)),
        )
        assertEquals(
            HomeAgentTileState.NOT_INSTALLED,
            homeAgentTileState(agent("newagent", available = false)),
        )
    }
}
