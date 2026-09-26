package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.ApprovalRequest
import pocketagent.v1.Thread
import pocketagent.v1.ThreadStatus

/**
 * 主页注意力三分的单测(P0-3 + P1-1):待审批组、运行中组、其余组。
 *
 * 归组判定的数据源即 daemon ListThreads 应答里的 status 与 pending_approvals,
 * 这里用 proto 默认实例造数据,不 mock 任何网络层。
 */
class HomeAttentionSectionsTest {

    private fun thread(
        id: String,
        status: ThreadStatus = ThreadStatus.THREAD_STATUS_IDLE,
        agentId: String = "codex",
        updatedAt: Long = 1_000L,
        pendingApprovals: Int = 0,
        parentThreadId: String = "",
    ): Thread {
        val b = Thread.newBuilder()
            .setId(id)
            .setTitle("会话 $id")
            .setStatus(status)
            .setAgentId(agentId)
            .setUpdatedAtUnixMs(updatedAt)
            .setParentThreadId(parentThreadId)
        repeat(pendingApprovals) { i -> b.addPendingApprovals(ApprovalRequest.newBuilder().setRequestId("req-$id-$i")) }
        return b.build()
    }

    // ---- 三分:待审批组 / 运行中组 / 其余组 ----

    @Test
    fun `待审批_运行中_其余_三分各归各位`() {
        val pending = thread("p1", ThreadStatus.THREAD_STATUS_WAITING_APPROVAL, pendingApprovals = 1)
        val waitingNoObject = thread("p2", ThreadStatus.THREAD_STATUS_WAITING_APPROVAL)
        val running = thread("r1", ThreadStatus.THREAD_STATUS_RUNNING)
        val idle = thread("i1", ThreadStatus.THREAD_STATUS_IDLE)
        val groups = partitionHomeAttention(listOf(pending, waitingNoObject, running, idle), emptySet())

        assertEquals(listOf("p1", "p2"), groups.pendingApproval.map { it.id })
        assertEquals(listOf("r1"), groups.running.map { it.id })
        assertEquals(listOf("i1"), groups.rest.map { it.id })
    }

    /**
     * 只有 pending_approvals、status 还停在 RUNNING(状态回写滞后的窗口期)
     * 也必须归待审批组 —— 审批对象在手就是"等你",不能掉进运行位。
     */
    @Test
    fun `带审批对象的RUNNING归待审批组_不重复占运行位`() {
        val both = thread("b1", ThreadStatus.THREAD_STATUS_RUNNING, pendingApprovals = 2)
        val groups = partitionHomeAttention(listOf(both), emptySet())

        assertEquals(listOf("b1"), groups.pendingApproval.map { it.id })
        assertTrue(groups.running.isEmpty())
    }

    // ---- 先减损再归组:归档/子代理存根/停用 agent 一律不进任何组 ----

    @Test
    fun `归档会话不进任何组`() {
        val archived = thread("a1", ThreadStatus.THREAD_STATUS_ARCHIVED, pendingApprovals = 1)
        val archivedRunning = thread("a2", ThreadStatus.THREAD_STATUS_ARCHIVED)
        val groups = partitionHomeAttention(listOf(archived, archivedRunning), emptySet())

        assertTrue(groups.pendingApproval.isEmpty())
        assertTrue(groups.running.isEmpty())
        assertTrue(groups.rest.isEmpty())
    }

    @Test
    fun `子代理存根线程不进任何组_与ThreadsScreen主列表同口径`() {
        val sub = thread("sub1", ThreadStatus.THREAD_STATUS_RUNNING, parentThreadId = "parent1")
        val subPending = thread("sub2", ThreadStatus.THREAD_STATUS_WAITING_APPROVAL, parentThreadId = "parent1")
        val groups = partitionHomeAttention(listOf(sub, subPending), emptySet())

        assertTrue(groups.pendingApproval.isEmpty())
        assertTrue(groups.running.isEmpty())
        assertTrue(groups.rest.isEmpty())
    }

    @Test
    fun `已停用agent的会话不进任何组_空agentId按codex归属`() {
        val piRunning = thread("pi1", ThreadStatus.THREAD_STATUS_RUNNING, agentId = "pi")
        val codexRunning = thread("c1", ThreadStatus.THREAD_STATUS_RUNNING, agentId = "")
        val groups = partitionHomeAttention(listOf(piRunning, codexRunning), disabledAgents = setOf("pi"))

        assertTrue(groups.running.map { it.id } == listOf("c1"))
        assertEquals(setOf("codex"), agentIdsOfRunning(groups.running))
    }

    // ---- 组内排序:updatedAtUnixMs 倒序(刚出事的排最前) ----

    @Test
    fun `组内按更新时刻倒序`() {
        val older = thread("older", ThreadStatus.THREAD_STATUS_RUNNING, updatedAt = 1_000L)
        val newer = thread("newer", ThreadStatus.THREAD_STATUS_RUNNING, updatedAt = 9_000L)
        val mid = thread("mid", ThreadStatus.THREAD_STATUS_RUNNING, updatedAt = 5_000L)
        val groups = partitionHomeAttention(listOf(older, newer, mid), emptySet())

        assertEquals(listOf("newer", "mid", "older"), groups.running.map { it.id })
    }

    // ---- agent 磁贴「运行中」徽章的数据源 ----

    @Test
    fun `运行徽章集合按归属agent归一_空为codex`() {
        val groups = partitionHomeAttention(
            listOf(
                thread("a", ThreadStatus.THREAD_STATUS_RUNNING, agentId = "pi"),
                thread("b", ThreadStatus.THREAD_STATUS_RUNNING, agentId = "zcode"),
                thread("c", ThreadStatus.THREAD_STATUS_RUNNING, agentId = ""),
                thread("d", ThreadStatus.THREAD_STATUS_IDLE, agentId = "pi"),
            ),
            emptySet(),
        )
        assertEquals(setOf("pi", "zcode", "codex"), agentIdsOfRunning(groups.running))
    }

    // ---- 元信息纯函数 ----

    @Test
    fun `待审批元信息_有审批对象报数_只有状态直说在等`() {
        assertEquals("2 个审批待处理", pendingApprovalMeta(2))
        assertEquals("1 个审批待处理", pendingApprovalMeta(1))
        assertEquals("等待你的处理", pendingApprovalMeta(0))
    }

    @Test
    fun `运行中元信息_有真实起点报运行时长_缺席回落最近活动`() {
        val now = 10_000_000L
        // 有起点(daemon 2026-09-26 起透出 turn_started_unix_ms):按时长报
        assertEquals("运行不到 1 分钟", runningMetaLabel(now - 30_000L, now, now))
        assertEquals("运行 5 分钟", runningMetaLabel(now - 5 * 60_000L, now, now))
        assertEquals("运行 1 小时 5 分", runningMetaLabel(now - 65 * 60_000L, now, now))
        // 起点在未来(时钟偏差/重启边界):钳到 0,不报负时长
        assertEquals("运行不到 1 分钟", runningMetaLabel(now + 30_000L, now, now))
        // 缺席(旧 daemon/未知):回退"最近活动"相对时间,数据全真
        assertEquals("最近活动 · 刚刚", runningMetaLabel(null, now - 30_000L, now))
        assertEquals("最近活动 · 5 分钟前", runningMetaLabel(null, now - 5 * 60_000L, now))
        assertEquals("运行中", runningMetaLabel(null, 0L, now))
    }
}
