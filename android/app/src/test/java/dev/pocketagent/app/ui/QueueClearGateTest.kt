package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 「撤回排队」按钮的门控与文案纯函数单测。
 *
 * 【口径】daemon 的 ClearQueuedInputs RPC 是 pi 专用(非 pi 会话显式
 * FailedPrecondition,见 daemon/internal/server/thread.go),能力位
 * `queue_clear:pi` 由 daemon 在 GetHostInfo.capabilities 按需宣告 —— 手机端
 * 三重门控(agent==pi 且 queuedCount>0 且能力位在)缺一不可:
 * 无能力不显示假入口(硬红线),有能力才给入口。
 */
class QueueClearGateTest {

    // ---- queueClearVisible:三重门控 ----

    @Test
    fun `pi会话_有排队_能力位在_显示`() {
        assertTrue(queueClearVisible("pi", 2, listOf("runtime.v1", "queue_clear:pi")))
    }

    @Test
    fun `非pi会话_一律不显示`() {
        assertFalse(queueClearVisible("codex", 1, listOf("queue_clear:pi")))
        assertFalse(queueClearVisible("zcode", 1, listOf("queue_clear:pi")))
        assertFalse(queueClearVisible("dsh", 1, listOf("queue_clear:pi")))
    }

    @Test
    fun `没有排队_不显示`() {
        assertFalse(queueClearVisible("pi", 0, listOf("queue_clear:pi")))
        assertFalse(queueClearVisible("pi", -1, listOf("queue_clear:pi")))
    }

    @Test
    fun `daemon未宣告queue_clear位_不显示假入口`() {
        assertFalse(queueClearVisible("pi", 3, emptyList()))
        assertFalse(queueClearVisible("pi", 3, listOf("runtime.v1", "collab:codex")))
    }

    @Test
    fun `agent与能力位大小写空白不敏感`() {
        assertTrue(queueClearVisible("Pi", 1, listOf(" QUEUE_CLEAR:PI ")))
        assertTrue(queueClearVisible(" pi ", 1, listOf("queue_clear:pi")))
    }

    @Test
    fun `别的agent的queue_clear位不给pi开入口`() {
        // daemon 按 agent 逐个宣告位;pi 只认自己的那一位
        assertFalse(queueClearVisible("pi", 1, listOf("queue_clear:codex")))
    }

    // ---- queueClearedNotice:成功回执人话(条数如实报) ----

    @Test
    fun `只有排队条数_报排队口径`() {
        assertEquals("已撤回 3 条排队输入", queueClearedNotice(0, 3))
    }

    @Test
    fun `只有插队条数_报插队口径`() {
        assertEquals("已撤回 1 条插队输入", queueClearedNotice(1, 0))
    }

    @Test
    fun `两类都有_总数加分类明细`() {
        assertEquals("已撤回 4 条（插队 1、排队 3）", queueClearedNotice(1, 3))
    }

    @Test
    fun `两类全零_如实说队列本来就空_不谎报撤回条数`() {
        assertEquals("队列里已经没有在排队的输入", queueClearedNotice(0, 0))
    }

    @Test
    fun `负数防御_按零处理`() {
        assertEquals("队列里已经没有在排队的输入", queueClearedNotice(-2, -3))
    }

    // ---- queueClearFailureMessage:失败壳 + 错误原文一字不动 ----

    @Test
    fun `daemon业务错误原文透传`() {
        val msg = queueClearFailureMessage(RuntimeException("该会话的 agent 没有可撤回的原生排队输入(仅 pi 会话支持)"))
        assertTrue(msg.contains("该会话的 agent 没有可撤回的原生排队输入(仅 pi 会话支持)"))
    }

    @Test
    fun `空message_退到异常类名_不出空白提示`() {
        val msg = queueClearFailureMessage(RuntimeException(""))
        assertEquals("撤回排队失败：RuntimeException", msg)
    }

    @Test
    fun `nullmessage_退到异常类名`() {
        val msg = queueClearFailureMessage(RuntimeException())
        assertEquals("撤回排队失败：RuntimeException", msg)
    }

    @Test
    fun `文案不带禁词`() {
        val copy = queueClearedNotice(2, 1) + queueClearFailureMessage(RuntimeException("x"))
        assertFalse(copy.contains("未知"))
        assertFalse(copy.contains("错误"))
    }
}
