package dev.pocketagent.app.ui.component

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.ThreadStatus

/**
 * 会话三态徽章语义(纯函数)的单测(审计 P1-1)。
 *
 * 口径与主页注意力三分(partitionHomeAttention)一致:
 *  - 待审批(pendingApprovals>0 或 WAITING_APPROVAL 状态)优先于运行 ——
 *    挂着审批的会话,等的是用户,不该显示成"运行中";
 *  - 失败 = IDLE 且带 lastTurnError(daemon 终局回写的口径,见 TurnSession
 *    恢复路径的同款判定);
 *  - Done/Idle 无错/归档 → null(无特殊,不摆"一切正常"的徽章空壳)。
 */
class ThreadStatusBadgeTest {

    @Test
    fun `运行中_绿色运行徽章`() {
        val badge = threadStatusBadge(ThreadStatus.THREAD_STATUS_RUNNING, 0, "")
        assertEquals(PaThreadBadgeKind.RUNNING, badge?.kind)
        assertEquals("运行中", badge?.label)
    }

    @Test
    fun `待审批优先于运行_状态滞后窗口期也不误报`() {
        val badge = threadStatusBadge(ThreadStatus.THREAD_STATUS_RUNNING, 2, "")
        assertEquals(PaThreadBadgeKind.WAITING, badge?.kind)
        assertEquals("待你处理", badge?.label)
    }

    @Test
    fun `WAITING状态无审批对象也是待你处理`() {
        val badge = threadStatusBadge(ThreadStatus.THREAD_STATUS_WAITING_APPROVAL, 0, "")
        assertEquals(PaThreadBadgeKind.WAITING, badge?.kind)
    }

    @Test
    fun `IDLE带最后错误_中断徽章带摘要`() {
        val badge = threadStatusBadge(ThreadStatus.THREAD_STATUS_IDLE, 0, "上游超时\nsession/resume 失败")
        assertEquals(PaThreadBadgeKind.FAILED, badge?.kind)
        assertEquals("中断", badge?.label)
        assertTrue(badge?.detail?.contains("上游超时") == true)
        // 摘要只取首行,不把第二行的方法名一起摊出来
        assertTrue(badge?.detail?.contains("session/resume") != true)
    }

    @Test
    fun `完成与空闲与归档_无徽章`() {
        assertNull(threadStatusBadge(ThreadStatus.THREAD_STATUS_IDLE, 0, ""))
        assertNull(threadStatusBadge(ThreadStatus.THREAD_STATUS_ARCHIVED, 0, "旧错误"))
        assertNull(threadStatusBadge(ThreadStatus.THREAD_STATUS_UNSPECIFIED, 0, ""))
    }

    @Test
    fun `错误摘要截断_超长首行收进40字`() {
        val long = "很长的错误".repeat(20)
        val summary = lastTurnErrorSummary(long)
        assertTrue(summary.length <= 41) // 40 字 + 省略号
        assertTrue(summary.endsWith("…"))
    }

    @Test
    fun `错误摘要空白输入_返回空串`() {
        assertEquals("", lastTurnErrorSummary("  \n "))
    }
}
