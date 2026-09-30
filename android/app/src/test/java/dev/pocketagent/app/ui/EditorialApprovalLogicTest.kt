package dev.pocketagent.app.ui

import dev.pocketagent.app.ui.theme.EditorialTone
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 审批弹窗状态机的纯逻辑单测(README §审批弹窗状态机)。
 *
 * 钉死四条红线:
 *  1. 流向唯一:收到请求→待决定→提交中→服务端确认→已允许/已拒绝;
 *  2. 提交中禁重复点击(第二次 submitting 是无操作);
 *  3. 失效/重复/超时/提交失败 → UNAVAILABLE:禁决定、给原因与重试,**绝不显示已批准**;
 *  4. 本层不存在"关闭=拒绝"的边(关闭走宿主 onDismiss,不经过这里)。
 */
class EditorialApprovalLogicTest {

    private fun pending() = ApprovalRequestUi(
        requestId = "req-1",
        threadId = "thread-1",
        agentName = "Codex",
        sessionTitle = "修复登录流程",
        purpose = "验证登录状态修复",
        workspace = "项目一",
        actionKind = "运行命令",
        command = "./gradlew :app:testDebugUnitTest",
    )

    // ---- decideEnabled / retryEnabled ----

    @Test
    fun `收到请求与待决定可决定_其余态一律禁用`() {
        assertTrue(pending().decideEnabled)
        assertTrue(pending().copy(phase = ApprovalPhase.RECEIVED).decideEnabled)
        assertFalse(pending().copy(phase = ApprovalPhase.SUBMITTING).decideEnabled)
        assertFalse(pending().copy(phase = ApprovalPhase.CONFIRMED).decideEnabled)
        assertFalse(pending().copy(phase = ApprovalPhase.ALLOWED).decideEnabled)
        assertFalse(pending().copy(phase = ApprovalPhase.DENIED).decideEnabled)
        assertFalse(pending().copy(phase = ApprovalPhase.UNAVAILABLE).decideEnabled)
    }

    @Test
    fun `只有暂无法处理态可重试`() {
        assertFalse(pending().retryEnabled)
        assertTrue(pending().copy(phase = ApprovalPhase.UNAVAILABLE).retryEnabled)
    }

    // ---- 流转:待决定 → 提交中 → 服务端确认 → 终态 ----

    @Test
    fun `提交中记录所点按钮_终态按服务端确认落词`() {
        val submitting = pending().submitting(allow = true)
        assertEquals(ApprovalPhase.SUBMITTING, submitting.phase)
        assertEquals(true, submitting.submittedChoice)

        val confirmed = submitting.serverConfirmed(allow = true)
        assertEquals(ApprovalPhase.CONFIRMED, confirmed.phase)
        val settled = confirmed.settled()
        assertEquals(ApprovalPhase.ALLOWED, settled.phase)

        val denied = pending().submitting(false).serverConfirmed(false).settled()
        assertEquals(ApprovalPhase.DENIED, denied.phase)
    }

    @Test
    fun `快路径_提交中直接拿服务端结果`() {
        assertEquals(ApprovalPhase.ALLOWED, pending().submitting(true).confirmed(true).phase)
        assertEquals(ApprovalPhase.DENIED, pending().submitting(false).confirmed(false).phase)
    }

    @Test
    fun `提交中禁重复点击_再次submitting是无操作`() {
        val once = pending().submitting(true)
        val twice = once.submitting(false) // 双击竞态:不允许翻转选择
        assertEquals(once, twice)
        assertEquals(true, twice.submittedChoice)
    }

    @Test
    fun `未提交的请求拿不到服务端确认_绝无乐观通过`() {
        // PENDING / UNAVAILABLE 上调用 confirmed/serverConfirmed 必须是无操作
        assertEquals(pending(), pending().serverConfirmed(true))
        assertEquals(pending(), pending().confirmed(true))
        val unavailable = pending().markUnavailable("请求已失效")
        assertEquals(unavailable, unavailable.confirmed(true))
    }

    @Test
    fun `终态上的一切推进都是无操作_不允许翻案`() {
        val allowed = pending().submitting(true).confirmed(true)
        assertEquals(allowed, allowed.submitting(false))
        assertEquals(allowed, allowed.serverConfirmed(false))
        assertEquals(allowed, allowed.settled())
        assertEquals(allowed, allowed.retry())
        // 终态也不接受 markUnavailable:已确认的结果不回退
        assertEquals(allowed, allowed.markUnavailable("随便什么原因"))
    }

    // ---- 暂无法处理:失效/重复/超时/提交失败 ----

    @Test
    fun `待决定态失效_禁决定_给原因_可重试回待决定`() {
        val stale = pending().markUnavailable("该请求已失效，请重新发起")
        assertEquals(ApprovalPhase.UNAVAILABLE, stale.phase)
        assertFalse(stale.decideEnabled)
        assertEquals("该请求已失效，请重新发起", stale.unavailableReason)

        val retried = stale.retry()
        assertEquals(ApprovalPhase.PENDING, retried.phase)
        assertTrue(retried.decideEnabled)
        assertNull(retried.unavailableReason)
    }

    @Test
    fun `提交失败进暂无法处理_保留提交痕迹但不显示已批准`() {
        val failed = pending().submitting(true).markUnavailable("提交失败，网络中断")
        assertEquals(ApprovalPhase.UNAVAILABLE, failed.phase)
        assertEquals(true, failed.submittedChoice) // 讲得清"上一次尝试没有成功"
        assertFalse(failed.decideEnabled)
        // 红线:任何字符串口径都不出现"已允许"
        assertFalse(failed.phaseWord.contains("已允许"))
    }

    @Test
    fun `重试后重新走完整提交链`() {
        val again = pending()
            .markUnavailable("请求超时")
            .retry()
            .submitting(true)
            .serverConfirmed(true)
            .settled()
        assertEquals(ApprovalPhase.ALLOWED, again.phase)
    }

    // ---- 状态词与色档 ----

    @Test
    fun `阶段状态词钉死_措辞不出现失败与错误`() {
        assertEquals("收到审批请求", pending().copy(phase = ApprovalPhase.RECEIVED).phaseWord)
        assertEquals("等待你处理", pending().phaseWord)
        assertEquals("提交中…", pending().copy(phase = ApprovalPhase.SUBMITTING).phaseWord)
        assertEquals("服务端已确认", pending().copy(phase = ApprovalPhase.CONFIRMED).phaseWord)
        assertEquals("已允许", pending().copy(phase = ApprovalPhase.ALLOWED).phaseWord)
        assertEquals("已拒绝", pending().copy(phase = ApprovalPhase.DENIED).phaseWord)
        assertEquals("暂无法处理", pending().copy(phase = ApprovalPhase.UNAVAILABLE).phaseWord)
        ApprovalPhase.values().forEach { phase ->
            assertFalse(pending().copy(phase = phase).phaseWord.contains("失败"))
            assertFalse(pending().copy(phase = phase).phaseWord.contains("错误"))
        }
    }

    @Test
    fun `阶段色档_待决定琥珀_允许绿_拒绝珊瑚_阻断琥珀`() {
        assertEquals(EditorialTone.ATTENTION, pending().phaseTone)
        assertEquals(EditorialTone.RUNNING, pending().copy(phase = ApprovalPhase.SUBMITTING).phaseTone)
        assertEquals(EditorialTone.RUNNING, pending().copy(phase = ApprovalPhase.ALLOWED).phaseTone)
        assertEquals(EditorialTone.DENIED, pending().copy(phase = ApprovalPhase.DENIED).phaseTone)
        assertEquals(EditorialTone.ATTENTION, pending().copy(phase = ApprovalPhase.UNAVAILABLE).phaseTone)
    }

    // ---- 多请求排队 ----

    @Test
    fun `排队数量词_单请求不摆空壳_越界收敛`() {
        assertEquals("", approvalQueueLabel(1, 1))
        assertEquals("", approvalQueueLabel(1, 0))
        assertEquals("共 3 项待审批 · 当前第 2 项", approvalQueueLabel(2, 3))
        assertEquals("共 3 项待审批 · 当前第 1 项", approvalQueueLabel(0, 3))
        assertEquals("共 3 项待审批 · 当前第 3 项", approvalQueueLabel(5, 3))
    }
}
