package dev.pocketagent.app.ui

import dev.pocketagent.app.R
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 协同「派发全流程」纯函数单测(R3-B 多 Agent 状态一屏总览轮):
 *  1. 参与者状态归并(collabParticipants):可用/不可用及原因,名单仍由 daemon 驱动;
 *  2. 派发可行性判定(collabDispatchDecision):0 人禁用 + 原因,防重复提交同源;
 *  3. 进度卡状态映射(collabProgressView):四词徽章 + 补充说明 + 卡上入口(查看会话/重派);
 *  4. 选中数摘要与失效剪除(collabSelectionSummary / pruneCollabSelection)。
 *
 * 既有相位四词映射(collabPhaseStatus/collabPhaseTone)仍由 EditorialListWaveTest 钉死,不重复。
 */
class CollabDispatchFlowTest {

    private fun agent(
        id: String,
        name: String = id,
        available: Boolean = true,
    ) = AgentInfo(
        id = id,
        name = name,
        method = "stdio",
        subtitle = "测试",
        iconRes = R.drawable.ic_lucide_cpu,
        available = available,
    )

    // ---- collabParticipants:可用/不可用及原因(结论 1) ----

    @Test
    fun `参与者归并_可用项为READY且无原因`() {
        val out = collabParticipants(
            allAgents = listOf(agent("pi", "Pi")),
            collabIds = listOf("pi"),
            disabled = emptySet(),
        )
        assertEquals(1, out.size)
        assertEquals(CollabParticipantState.READY, out[0].state)
        assertNull(out[0].unavailableReason)
        assertEquals("Pi", out[0].agent.name)
    }

    @Test
    fun `参与者归并_本地停用给出停用原因而非假装不存在`() {
        val out = collabParticipants(
            allAgents = listOf(agent("pi", "Pi")),
            collabIds = listOf("pi"),
            disabled = setOf("pi"),
        )
        assertEquals(CollabParticipantState.LOCALLY_DISABLED, out[0].state)
        assertTrue(out[0].unavailableReason!!.contains("停用"))
        assertTrue(out[0].unavailableReason!!.contains("Pi"))
    }

    @Test
    fun `参与者归并_电脑端不可用原因与首页网格同源`() {
        // pi/zcode 不在占位目录 → "电脑端未安装";占位目录内 → "接入中"
        val out = collabParticipants(
            allAgents = listOf(agent("pi", "Pi", available = false), agent("opencode", "OpenCode", available = false)),
            collabIds = listOf("pi", "opencode"),
            disabled = emptySet(),
        )
        assertEquals(2, out.size)
        val pi = out.first { it.agent.id == "pi" }
        val oc = out.first { it.agent.id == "opencode" }
        assertEquals(CollabParticipantState.OFFLINE, pi.state)
        assertEquals("Pi:电脑端未安装", pi.unavailableReason)
        assertEquals(CollabParticipantState.OFFLINE, oc.state)
        assertEquals("OpenCode:接入中", oc.unavailableReason)
    }

    @Test
    fun `参与者归并_未知collab id合成条目_停用的以停用态呈现`() {
        val out = collabParticipants(
            allAgents = emptyList(),
            collabIds = listOf("newagent", "ghost"),
            disabled = setOf("ghost"),
        )
        // 结论 1:停用的参与者要带着原因出现在全集里(不静默消失),
        // 但 READY 的排前面;可勾选名单(collabCapableAgents)仍不含停用项。
        assertEquals(2, out.size)
        assertEquals("newagent", out[0].agent.id)
        assertEquals(CollabParticipantState.READY, out[0].state)
        assertEquals("ghost", out[1].agent.id)
        assertEquals(CollabParticipantState.LOCALLY_DISABLED, out[1].state)
        assertTrue(out[1].unavailableReason!!.contains("停用"))
    }

    @Test
    fun `参与者归并_可用的排前面_组内按显示名`() {
        val out = collabParticipants(
            allAgents = listOf(agent("zcode", "ZCode"), agent("pi", "Pi", available = false), agent("codex", "Codex")),
            collabIds = listOf("zcode", "pi", "codex"),
            disabled = emptySet(),
        )
        assertEquals(listOf("Codex", "ZCode", "Pi"), out.map { it.agent.name })
    }

    @Test
    fun `可协同清单是参与者READY投影_旧口径不变`() {
        val all = listOf(agent("pi", "Pi"), agent("zcode", "ZCode", available = false))
        val capable = collabCapableAgents(all, listOf("pi", "zcode"), disabled = setOf("pi"))
        // 停用与不可用的都不进可协同清单(与改造前口径一致)
        assertTrue(capable.isEmpty())
    }

    @Test
    fun `不可用原因汇总_全员可用为null_混合时逐个列出`() {
        val ready = CollabParticipant(agent("pi", "Pi"), CollabParticipantState.READY, null)
        val off = CollabParticipant(agent("zcode", "ZCode"), CollabParticipantState.OFFLINE, "ZCode:电脑端未安装")
        assertNull(collabUnavailableNote(listOf(ready)))
        assertEquals("ZCode:电脑端未安装", collabUnavailableNote(listOf(ready, off)))
    }

    // ---- collabDispatchDecision:0 人禁用 + 原因,防重复提交(结论 1/3) ----

    @Test
    fun `派发判定_零参与者禁用且给原因`() {
        val d = collabDispatchDecision(selectedCount = 0, instructionReady = true, dispatching = false)
        assertFalse(d.canDispatch)
        assertEquals(CollabCopy.REASON_NO_PARTICIPANT, d.disabledReason)
    }

    @Test
    fun `派发判定_指令空白禁用且给原因`() {
        val d = collabDispatchDecision(selectedCount = 2, instructionReady = false, dispatching = false)
        assertFalse(d.canDispatch)
        assertEquals(CollabCopy.REASON_NO_INSTRUCTION, d.disabledReason)
    }

    @Test
    fun `派发判定_提交中禁用且给原因`() {
        val d = collabDispatchDecision(selectedCount = 2, instructionReady = true, dispatching = true)
        assertFalse(d.canDispatch)
        assertEquals(CollabCopy.REASON_DISPATCH_IN_FLIGHT, d.disabledReason)
    }

    @Test
    fun `派发判定_就绪可派发且原因为空`() {
        val d = collabDispatchDecision(selectedCount = 1, instructionReady = true, dispatching = false)
        assertTrue(d.canDispatch)
        assertNull(d.disabledReason)
    }

    @Test
    fun `派发判定_原因优先级_零参与者压过指令空白`() {
        val d = collabDispatchDecision(selectedCount = 0, instructionReady = false, dispatching = false)
        assertEquals(CollabCopy.REASON_NO_PARTICIPANT, d.disabledReason)
    }

    // ---- collabProgressView:进度卡状态映射(结论 2/4) ----

    @Test
    fun `进度卡_未派发不摆四词徽章`() {
        val v = collabProgressView(CollabAgentPhase.IDLE, hasThreadId = false)
        assertNull(v.status)
        assertNull(v.detailLine)
        assertFalse(v.canOpenSession)
        assertFalse(v.canRedispatch)
    }

    @Test
    fun `进度卡_受理与执行都归正在执行且给到哪一步的说明`() {
        val ack = collabProgressView(CollabAgentPhase.ACK, hasThreadId = false)
        val run = collabProgressView(CollabAgentPhase.RUNNING, hasThreadId = true)
        assertEquals(EditorialStatus.RUNNING, ack.status)
        assertEquals("正在执行", ack.status!!.word)
        assertEquals("已收到指令,等待电脑端受理", ack.detailLine)
        assertFalse(ack.canOpenSession) // 还没拿到子线程,没有会话可看
        assertEquals(EditorialStatus.RUNNING, run.status)
        assertTrue(run.canOpenSession)
    }

    @Test
    fun `进度卡_完成归已完成_线程在手可看会话`() {
        val v = collabProgressView(CollabAgentPhase.DONE, hasThreadId = true)
        assertEquals(EditorialStatus.COMPLETED, v.status)
        assertEquals("已完成", v.status!!.word)
        assertNull(v.detailLine)
        assertTrue(v.canOpenSession)
        assertFalse(v.canRedispatch)
    }

    @Test
    fun `进度卡_未完成给四词与重派入口_不落到失败措辞`() {
        val v = collabProgressView(CollabAgentPhase.ERROR, hasThreadId = false)
        assertEquals(EditorialStatus.NOT_COMPLETED, v.status)
        assertEquals("未完成", v.status!!.word)
        assertTrue(v.canRedispatch)
        assertFalse(v.canOpenSession)
        // 四词纪律:相位兜底文案也不用「失败/出错」裸词
        assertFalse(collabPhaseLabel(CollabAgentPhase.ERROR).contains("失败"))
        assertFalse(collabPhaseLabel(CollabAgentPhase.ERROR).contains("出错"))
    }

    // ---- 选中数摘要与失效剪除(结论 1) ----

    @Test
    fun `选中数摘要_零人也如实说`() {
        assertEquals("未选参与者", collabSelectionSummary(0))
        assertEquals("已选 2 位", collabSelectionSummary(2))
    }

    @Test
    fun `失效剪除_只留交集_掉线者出局`() {
        val pruned = pruneCollabSelection(
            selected = setOf("pi", "zcode", "codex"),
            readyIds = setOf("pi", "codex"),
        )
        assertEquals(setOf("pi", "codex"), pruned)
    }

    @Test
    fun `多选切换_加与减对称`() {
        var s: Set<String> = emptySet()
        s = toggleCollabSelection(s, "pi")
        assertEquals(setOf("pi"), s)
        s = toggleCollabSelection(s, "pi")
        assertTrue(s.isEmpty())
    }
}
