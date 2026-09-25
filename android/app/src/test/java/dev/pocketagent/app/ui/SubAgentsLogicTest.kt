package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.Thread as PThread
import java.util.concurrent.ConcurrentHashMap

/**
 * 【T-SUB 子代理线程组】纯决策钉子(SubAgentsLogic + SubAgentsState.toMeta):
 *
 * 钉四类不许反转的规则:
 * 1. 门控 —— 父 id 未知(空串)时 childrenOf 恒返空组,"无子线程=无卡不误报";
 * 2. 状态优先级 —— 本机实时流 > 子线程自有审批 > 快照运行(daemon
 *    Thread.Status=RUNNING,P-1) > 落库失败 > 完成;
 * 3. 组卡计数与折叠规则(蓝本 §4.2:运行中默认展开,全部终态默认折叠;
 *    P-3:用户手动表态优先,运行集变化不覆盖);
 * 4. 兄弟导航边界(越界/不在组里 = null,切换钮据此置灰)。
 */
class SubAgentsLogicTest {

    private fun meta(
        id: String,
        parent: String = "",
        title: String = "任务 $id",
        agentId: String = "codex",
        count: Int = 3,
        createdAtMs: Long = 0L,
        failed: Boolean = false,
        waiting: Boolean = false,
        statusRunning: Boolean = false,
    ) = SubAgentThreadMeta(
        threadId = id,
        parentThreadId = parent,
        title = title,
        agentId = agentId,
        messageCount = count,
        createdAtMs = createdAtMs,
        lastTurnFailed = failed,
        waitingApproval = waiting,
        statusRunning = statusRunning,
    )

    // ---- childrenOf:门控与排序 ----

    @Test
    fun `blank parent id returns empty group - the no-false-positive gate`() {
        // 门控核心:关系未知(daemon 未落 parent_thread_id,toMeta 如实透传为空串)= 无卡
        val all = listOf(meta("a", parent = "p"), meta("b", parent = ""))
        assertTrue(SubAgentsLogic.childrenOf(all, "").isEmpty())
        assertTrue(SubAgentsLogic.childrenOf(all, " ").isEmpty())
    }

    @Test
    fun `only threads whose parent matches are returned`() {
        val all = listOf(
            meta("main", parent = ""),
            meta("c1", parent = "main"),
            meta("c2", parent = "main"),
            meta("other", parent = "another"),
            meta("sub-of-c1", parent = "c1"), // 隔代不并入
        )
        val children = SubAgentsLogic.childrenOf(all, "main")
        assertEquals(listOf("c1", "c2"), children.map { it.threadId })
    }

    @Test
    fun `children are ordered by start time then id - no smart sorting`() {
        val all = listOf(
            meta("late", parent = "p", createdAtMs = 30),
            meta("first", parent = "p", createdAtMs = 10),
            meta("tie-b", parent = "p", createdAtMs = 20),
            meta("tie-a", parent = "p", createdAtMs = 20),
        )
        assertEquals(
            listOf("first", "tie-a", "tie-b", "late"),
            SubAgentsLogic.childrenOf(all, "p").map { it.threadId },
        )
    }

    // ---- runStateOf:优先级 ----

    @Test
    fun `live running stream wins over everything`() {
        val m = meta("c1", failed = true, waiting = true)
        assertEquals(
            SubAgentRunState.RUNNING,
            SubAgentsLogic.runStateOf(m, setOf("c1")),
        )
    }

    @Test
    fun `own approval beats stored failure`() {
        assertEquals(
            SubAgentRunState.WAITING_APPROVAL,
            SubAgentsLogic.runStateOf(meta("c1", failed = true, waiting = true), emptySet()),
        )
    }

    @Test
    fun `stored failure shows failed otherwise done`() {
        assertEquals(SubAgentRunState.FAILED, SubAgentsLogic.runStateOf(meta("c1", failed = true), emptySet()))
        assertEquals(SubAgentRunState.DONE, SubAgentsLogic.runStateOf(meta("c1"), emptySet()))
    }

    @Test
    fun `blank thread id never counts as live running`() {
        assertEquals(
            SubAgentRunState.DONE,
            SubAgentsLogic.runStateOf(meta(""), setOf("")),
        )
    }

    @Test
    fun `snapshot running shows running instead of false done - P1`() {
        // P-1:thread/list 快照视角的后台运行中(THREAD_STATUS_RUNNING),
        // 不再因实时流集合看不到而被误标"已完成"
        assertEquals(
            SubAgentRunState.RUNNING,
            SubAgentsLogic.runStateOf(meta("c1", statusRunning = true), emptySet()),
        )
    }

    @Test
    fun `snapshot running sits between approval and stored failure - P1`() {
        // 优先级钉子:实时流 > 快照运行(与审批并存时审批优先)
        assertEquals(
            SubAgentRunState.WAITING_APPROVAL,
            SubAgentsLogic.runStateOf(meta("c1", waiting = true, statusRunning = true), emptySet()),
        )
        // 快照运行 > 落库失败:daemon 视角还在跑的线程如实显示运行中
        assertEquals(
            SubAgentRunState.RUNNING,
            SubAgentsLogic.runStateOf(meta("c1", failed = true, statusRunning = true), emptySet()),
        )
    }

    @Test
    fun `thread status running maps to snapshot running field - P1`() {
        val running = PThread.newBuilder().setId("t4")
            .setStatus(pocketagent.v1.ThreadStatus.THREAD_STATUS_RUNNING).build()
        assertTrue(SubAgentsLogic.toMeta(running).statusRunning)
        // 非运行态不误标
        assertFalse(
            SubAgentsLogic.toMeta(
                PThread.newBuilder().setId("t5")
                    .setStatus(pocketagent.v1.ThreadStatus.THREAD_STATUS_IDLE).build(),
            ).statusRunning,
        )
        assertFalse(
            SubAgentsLogic.toMeta(
                PThread.newBuilder().setId("t6")
                    .setStatus(pocketagent.v1.ThreadStatus.THREAD_STATUS_ARCHIVED).build(),
            ).statusRunning,
        )
    }

    // ---- 文案 ----

    @Test
    fun `status labels cover all six states`() {
        assertEquals("排队中", SubAgentsLogic.statusLabel(SubAgentRunState.QUEUED))
        assertEquals("运行中", SubAgentsLogic.statusLabel(SubAgentRunState.RUNNING))
        assertEquals("等待审批", SubAgentsLogic.statusLabel(SubAgentRunState.WAITING_APPROVAL))
        assertEquals("已完成", SubAgentsLogic.statusLabel(SubAgentRunState.DONE))
        assertEquals("失败", SubAgentsLogic.statusLabel(SubAgentRunState.FAILED))
        assertEquals("已取消", SubAgentsLogic.statusLabel(SubAgentRunState.CANCELLED))
    }

    @Test
    fun `preview flattens whitespace and truncates with ellipsis`() {
        assertEquals("", SubAgentsLogic.previewOf(null))
        assertEquals("", SubAgentsLogic.previewOf("  \n\t "))
        assertEquals("正在读 AuthService.kt", SubAgentsLogic.previewOf(" 正在读\nAuthService.kt\t"))
        val long = "字".repeat(130)
        val cut = SubAgentsLogic.previewOf(long)
        assertEquals(121, cut.length)
        assertTrue(cut.endsWith("…"))
        assertEquals("字".repeat(120) + "…", cut)
    }

    @Test
    fun `preview failure clears placeholder so retry can request again - P2`() {
        val cache = ConcurrentHashMap<String, String>()
        cache["c1"] = "" // 占位防重(与 requestPreview 发请求前同款)
        // 失败(text=null)→ 占位键移除:重试守卫(previews.containsKey)放行,
        // 卡片下次可见时可再拉 —— 占位键不被失败永久吞掉
        assertFalse(SubAgentsLogic.settlePreview(cache, "c1", null))
        assertFalse(cache.containsKey("c1"))
        // 重试成功 → 落缓存并让调用方驱动重组
        assertTrue(SubAgentsLogic.settlePreview(cache, "c1", "正在读 AuthService.kt"))
        assertEquals("正在读 AuthService.kt", cache["c1"])
        // 对照:成功但空白 = 该线程确无文本预览,占位保留防反复空拉(非失败)
        cache["c2"] = ""
        assertFalse(SubAgentsLogic.settlePreview(cache, "c2", "  \n "))
        assertEquals("", cache["c2"])
    }

    @Test
    fun `group summary lists only non-zero segments`() {
        assertEquals("", SubAgentsLogic.groupSummaryLine(emptyList()))
        assertEquals("1 完成", SubAgentsLogic.groupSummaryLine(listOf(SubAgentRunState.DONE)))
        assertEquals(
            "1 运行 1 等审批 1 失败 2 完成",
            SubAgentsLogic.groupSummaryLine(
                listOf(
                    SubAgentRunState.RUNNING,
                    SubAgentRunState.WAITING_APPROVAL,
                    SubAgentRunState.FAILED,
                    SubAgentRunState.DONE,
                    SubAgentRunState.CANCELLED,
                ),
            ),
        )
    }

    @Test
    fun `group is active while any child runs waits or queues`() {
        assertTrue(SubAgentsLogic.groupIsActive(listOf(SubAgentRunState.DONE, SubAgentRunState.RUNNING)))
        assertTrue(SubAgentsLogic.groupIsActive(listOf(SubAgentRunState.QUEUED)))
        assertTrue(SubAgentsLogic.groupIsActive(listOf(SubAgentRunState.WAITING_APPROVAL)))
        // 全部终态(完成/失败/取消)= 默认折叠
        assertFalse(SubAgentsLogic.groupIsActive(listOf(SubAgentRunState.DONE, SubAgentRunState.FAILED)))
        assertFalse(SubAgentsLogic.groupIsActive(emptyList()))
    }

    @Test
    fun `user collapse survives run set change - no auto expand - P3`() {
        val kids = listOf(meta("a", parent = "p"), meta("b", parent = "p"))
        val active = kids.map { SubAgentsLogic.runStateOf(it, setOf("a")) }
        val settled = kids.map { SubAgentsLogic.runStateOf(it, emptySet()) }
        // 未表态:运行中默认展开,父轮结束后全部终态默认折叠(默认规则不变)
        assertTrue(SubAgentsLogic.expandedOf(null, active))
        assertFalse(SubAgentsLogic.expandedOf(null, settled))
        // P-3 核心:用户手动收起 → 父轮结束(运行集清空)→ 意图优先,不自动弹开
        assertFalse(SubAgentsLogic.expandedOf(false, settled))
        // 对称保护:用户手动展开后全部终态,也保持展开不被折叠
        assertTrue(SubAgentsLogic.expandedOf(true, settled))
    }

    // ---- 兄弟导航 ----

    @Test
    fun `sibling position is one-based and hides for unknown threads`() {
        val group = listOf(meta("a", parent = "p"), meta("b", parent = "p"), meta("c", parent = "p"))
        assertEquals(1 to 3, SubAgentsLogic.siblingPos(group, "a"))
        assertEquals(2 to 3, SubAgentsLogic.siblingPos(group, "b"))
        assertEquals(3 to 3, SubAgentsLogic.siblingPos(group, "c"))
        assertEquals(0 to 0, SubAgentsLogic.siblingPos(group, "nope"))
        assertEquals(0 to 0, SubAgentsLogic.siblingPos(emptyList(), "a"))
    }

    @Test
    fun `sibling nav returns null at the edges or off-group`() {
        val group = listOf(meta("a", parent = "p"), meta("b", parent = "p"))
        assertEquals("b", SubAgentsLogic.siblingNav(group, "a", +1))
        assertEquals("a", SubAgentsLogic.siblingNav(group, "b", -1))
        assertNull(SubAgentsLogic.siblingNav(group, "a", -1)) // 已是第一个
        assertNull(SubAgentsLogic.siblingNav(group, "b", +1)) // 已是最后一个
        assertNull(SubAgentsLogic.siblingNav(group, "nope", +1))
    }

    // ---- toMeta:pb → UI 快照(父关系透传 + 门控 + 运行状态映射) ----

    @Test
    fun `thread proto maps to ui snapshot`() {
        val pb = PThread.newBuilder()
            .setId("t1")
            .setTitle("重构登录")
            .setAgentId("pi")
            .setMessageCount(12)
            .setCreatedAtUnixMs(111)
            .setUpdatedAtUnixMs(222)
            .setLastTurnError("boom")
            .setStatus(pocketagent.v1.ThreadStatus.THREAD_STATUS_WAITING_APPROVAL)
            .build()
        val m = SubAgentsLogic.toMeta(pb)
        assertEquals("t1", m.threadId)
        assertEquals("重构登录", m.title)
        assertEquals("pi", m.agentId)
        assertEquals(12, m.messageCount)
        assertEquals(111L, m.createdAtMs)
        assertTrue(m.lastTurnFailed)
        assertTrue(m.waitingApproval)
    }

    @Test
    fun `blank title falls back and empty error is not failure`() {
        val pb = PThread.newBuilder().setId("t2").build()
        val m = SubAgentsLogic.toMeta(pb)
        // 不伪造:标题缺省给中性占位,错误空 = 非失败
        assertEquals("子任务", m.title)
        assertFalse(m.lastTurnFailed)
        assertFalse(m.waitingApproval)
    }

    /**
     * 【父关系透传钉子,P-4 改正向】T-S1P 已补契约(thread.proto 字段 22
     * parent_thread_id)并接线(daemon 两路填充:codex 原生子线程经 thread/started
     * 存根登记;collab 委派建线时落库)。toMeta 如实透传,不丢不造:非空 = 内联卡
     * /来源行的门控依据;空 = 关系未知,按普通会话处理("无卡不误报"的源头,
     * 见上方 blank parent id 用例)。
     */
    @Test
    fun `parent linkage passes through from proto to ui snapshot`() {
        val pb = PThread.newBuilder().setId("t3").setParentThreadId("main").build()
        assertEquals("main", SubAgentsLogic.toMeta(pb).parentThreadId)
    }

    /**
     * 【T-S1P 正向端到端,P-4 补课】pb 带真实 parent_thread_id → toMeta 透传
     * → childrenOf 命中出卡:内联卡数据源全链路,断言"接通后的正道"。
     */
    @Test
    fun `parent linkage end to end - proto feeds childrenOf to render cards`() {
        val parent = PThread.newBuilder().setId("main").setTitle("主任务").build()
        val c1 = PThread.newBuilder().setId("c1").setParentThreadId("main")
            .setTitle("读 AuthService").setCreatedAtUnixMs(10).build()
        val c2 = PThread.newBuilder().setId("c2").setParentThreadId("main")
            .setTitle("改签名").setCreatedAtUnixMs(20).build()
        val top = PThread.newBuilder().setId("top").setTitle("顶层会话").build()
        val all = listOf(parent, c1, c2, top).map { SubAgentsLogic.toMeta(it) }

        // pb → toMeta:父关系透传不丢,顶层会话如实落空串
        assertEquals("main", all[1].parentThreadId)
        assertEquals("main", all[2].parentThreadId)
        assertEquals("", all[3].parentThreadId)

        // childrenOf 命中:该主线程出两张卡(启动顺序);门控关闭处不出组不误报
        assertEquals(listOf("c1", "c2"), SubAgentsLogic.childrenOf(all, "main").map { it.threadId })
        assertTrue(SubAgentsLogic.childrenOf(all, "").isEmpty())
        assertTrue(SubAgentsLogic.childrenOf(all, "top").isEmpty())
    }

    // ---- 快览弹层角色短标签 ----

    @Test
    fun `peek sheet role labels are short and stable`() {
        assertEquals("我", subAgentRoleLabel("user"))
        assertEquals("AI", subAgentRoleLabel("assistant"))
        assertEquals("思考", subAgentRoleLabel("thinking"))
        assertEquals("工具", subAgentRoleLabel("tool"))
        // 未知角色如实回显原文,不编造(设计原则 §7:识别不了≠出错)
        assertEquals("codex_event", subAgentRoleLabel("codex_event"))
    }
}
