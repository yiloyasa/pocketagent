package dev.pocketagent.app.ui

import dev.pocketagent.app.ui.component.TaskCenter
import dev.pocketagent.app.ui.theme.EditorialTone
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.OutputState
import pocketagent.v1.ThreadStatus

/**
 * 四词状态映射器的单测(README §状态:等待你处理/正在执行/已完成/未完成)。
 *
 * 钉三件事:
 *  1. 优先级 —— 待审批 > 运行(状态滞后的窗口期也不能把"等你"说成"在跑");
 *  2. 四词原文与色档一一对应(语义不单靠颜色);
 *  3. 不编状态 —— 无事实(UNSPECIFIED)在 ofOrNull 里返回 null,不摆空壳。
 */
class EditorialStatusTest {

    // ---- ThreadStatus 映射 ----

    @Test
    fun `运行中无审批_正在执行_绿色档`() {
        val s = editorialStatusOf(ThreadStatus.THREAD_STATUS_RUNNING, 0, "")
        assertEquals(EditorialStatus.RUNNING, s)
        assertEquals("正在执行", s.word)
        assertEquals(EditorialTone.RUNNING, s.tone)
    }

    @Test
    fun `挂着审批时即使状态滞后仍是等待你处理`() {
        assertEquals(
            EditorialStatus.WAITING_FOR_YOU,
            editorialStatusOf(ThreadStatus.THREAD_STATUS_RUNNING, 2, ""),
        )
        assertEquals(
            EditorialStatus.WAITING_FOR_YOU,
            editorialStatusOf(ThreadStatus.THREAD_STATUS_WAITING_APPROVAL, 0, ""),
        )
    }

    @Test
    fun `IDLE带最后错误_未完成_珊瑚档_附首行摘要`() {
        val view = editorialThreadStatusView(
            ThreadStatus.THREAD_STATUS_IDLE, 0, "上游超时\nsession/resume 失败",
        )
        assertEquals(EditorialStatus.NOT_COMPLETED, view.status)
        assertEquals("未完成", view.status.word)
        assertEquals(EditorialTone.DENIED, view.status.tone)
        assertTrue(view.detail?.contains("上游超时") == true)
        // 摘要只取首行,不把方法名一起摊出来
        assertTrue(view.detail?.contains("session/resume") != true)
    }

    @Test
    fun `IDLE干净与归档_已完成_中性档`() {
        assertEquals(
            EditorialStatus.COMPLETED,
            editorialStatusOf(ThreadStatus.THREAD_STATUS_IDLE, 0, ""),
        )
        assertEquals(
            EditorialStatus.COMPLETED,
            editorialStatusOf(ThreadStatus.THREAD_STATUS_ARCHIVED, 0, "旧错误也不影响归档口径" ),
        )
    }

    @Test
    fun `归档是用户终局_不因历史错误翻案`() {
        // ARCHIVED 不走"IDLE+错误→未完成"分支:显式收起的会话按已完成收口
        assertEquals(
            EditorialStatus.COMPLETED,
            editorialStatusOf(ThreadStatus.THREAD_STATUS_ARCHIVED, 0, "旧错误"),
        )
    }

    @Test
    fun `UNSPECIFIED_总函数保守归未完成_可空函数返回null`() {
        assertEquals(
            EditorialStatus.NOT_COMPLETED,
            editorialStatusOf(ThreadStatus.THREAD_STATUS_UNSPECIFIED, 0, ""),
        )
        assertNull(editorialStatusOrNull(ThreadStatus.THREAD_STATUS_UNSPECIFIED, 0, ""))
    }

    @Test
    fun `UNSPECIFIED但挂着审批或错误_不是null_按事实归档`() {
        assertEquals(
            EditorialStatus.WAITING_FOR_YOU,
            editorialStatusOrNull(ThreadStatus.THREAD_STATUS_UNSPECIFIED, 1, ""),
        )
        assertEquals(
            EditorialStatus.NOT_COMPLETED,
            editorialStatusOrNull(ThreadStatus.THREAD_STATUS_UNSPECIFIED, 0, "超时"),
        )
    }

    // ---- TaskCenter.Status 映射 ----

    @Test
    fun `任务中心状态四词映射`() {
        assertEquals(
            EditorialStatus.RUNNING,
            editorialStatusOf(TaskCenter.Status.RUNNING),
        )
        assertEquals(
            EditorialStatus.COMPLETED,
            editorialStatusOf(TaskCenter.Status.DONE),
        )
        assertEquals(
            EditorialStatus.NOT_COMPLETED,
            editorialStatusOf(TaskCenter.Status.FAILED),
        )
        assertEquals(
            EditorialStatus.NOT_COMPLETED,
            editorialStatusOf(TaskCenter.Status.CANCELED),
        )
        assertEquals(
            EditorialStatus.NOT_COMPLETED,
            editorialStatusOf(TaskCenter.Status.INTERRUPTED),
        )
    }

    // ---- OutputState 映射 ----

    @Test
    fun `轮次正常收尾含空正文_已完成`() {
        assertEquals(EditorialStatus.COMPLETED, editorialStatusOrNull(OutputState.OUTPUT_STATE_TEXT))
        // EMPTY 是事实陈述("确实没话说"),不伪装也不算未完成
        assertEquals(EditorialStatus.COMPLETED, editorialStatusOrNull(OutputState.OUTPUT_STATE_EMPTY))
        assertEquals(
            EditorialStatus.COMPLETED,
            editorialStatusOrNull(OutputState.OUTPUT_STATE_ARTIFACT_ONLY),
        )
        assertEquals(
            EditorialStatus.COMPLETED,
            editorialStatusOrNull(OutputState.OUTPUT_STATE_RECOVERED_FROM_NATIVE_HISTORY),
        )
    }

    @Test
    fun `轮次失败与丢正文_未完成_UNSPECIFIED为null`() {
        assertEquals(EditorialStatus.NOT_COMPLETED, editorialStatusOrNull(OutputState.OUTPUT_STATE_ERROR))
        assertEquals(
            EditorialStatus.NOT_COMPLETED,
            editorialStatusOrNull(OutputState.OUTPUT_STATE_ADAPTER_LOST_OUTPUT),
        )
        assertNull(editorialStatusOrNull(OutputState.OUTPUT_STATE_UNSPECIFIED))
    }

    // ---- 四词契约 ----

    @Test
    fun `四词原文与色档是全 app 契约_改名即断链`() {
        assertEquals("等待你处理", EditorialStatus.WAITING_FOR_YOU.word)
        assertEquals("正在执行", EditorialStatus.RUNNING.word)
        assertEquals("已完成", EditorialStatus.COMPLETED.word)
        assertEquals("未完成", EditorialStatus.NOT_COMPLETED.word)
        assertEquals(EditorialTone.ATTENTION, EditorialStatus.WAITING_FOR_YOU.tone)
        assertEquals(EditorialTone.RUNNING, EditorialStatus.RUNNING.tone)
        assertEquals(EditorialTone.NEUTRAL, EditorialStatus.COMPLETED.tone)
        assertEquals(EditorialTone.DENIED, EditorialStatus.NOT_COMPLETED.tone)
        // 四档色互不相同:语义不单靠颜色,但颜色也不能撞车
        assertEquals(
            4,
            setOf(
                EditorialStatus.WAITING_FOR_YOU.tone.accent,
                EditorialStatus.RUNNING.tone.accent,
                EditorialStatus.COMPLETED.tone.accent,
                EditorialStatus.NOT_COMPLETED.tone.accent,
            ).size,
        )
    }
}
