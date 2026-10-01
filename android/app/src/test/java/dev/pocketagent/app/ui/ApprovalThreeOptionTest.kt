package dev.pocketagent.app.ui

import kotlinx.coroutines.channels.Channel
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.ApprovalOption
import pocketagent.v1.ApprovalRequest
import pocketagent.v1.TurnClientMessage

/**
 * 审批卡三按钮(C1:codex 0.159.2 三按钮审批增量)的纯逻辑单测。
 *
 * 钉死三条红线:
 *  1. 渲染映射:accept=主允许「本次允许」/ acceptForSession=次允许「本会话允许」/
 *     decline 与一切 deny 类=红「拒绝」;允许类在前、拒绝殿后;deny 特征词不漏网;
 *  2. 应答回填:点第三按钮 → ApprovalDecision.option 带原词 id,
 *     approved 布尔语义保持(acceptForSession 也是 approved=true+option);
 *  3. 旧 daemon 兼容:无 options → 空映射,走旧两按钮,旧口径回显一字不变。
 * cancel 也是合法应答,但**不作第三按钮广告** —— 不在映射表里,映射不到就不渲染。
 */
class ApprovalThreeOptionTest {

    // ---- 渲染映射:三按钮 ----

    @Test
    fun `三按钮渲染映射_顺序文案色档钉死`() {
        val buttons = approvalOptionButtonsOf(
            listOf(
                ApprovalOptionUi("accept"),
                ApprovalOptionUi("acceptForSession", "本次及本会话允许"),
                ApprovalOptionUi("decline"),
            ),
        )
        assertEquals(3, buttons.size)
        assertEquals(ApprovalOptionButton("accept", "本次允许", ApprovalOptionRole.PRIMARY_ALLOW), buttons[0])
        assertEquals(
            ApprovalOptionButton("acceptForSession", "本会话允许", ApprovalOptionRole.SECONDARY_ALLOW),
            buttons[1],
        )
        assertEquals(ApprovalOptionButton("decline", "拒绝", ApprovalOptionRole.DENY), buttons[2])
    }

    @Test
    fun `旧词枚举同样命中映射_approved系列`() {
        val buttons = approvalOptionButtonsOf(
            listOf(ApprovalOptionUi("approved"), ApprovalOptionUi("approved_for_session"), ApprovalOptionUi("denied")),
        )
        assertEquals(ApprovalOptionRole.PRIMARY_ALLOW, buttons[0].role)
        assertEquals("本次允许", buttons[0].label)
        assertEquals(ApprovalOptionRole.SECONDARY_ALLOW, buttons[1].role)
        assertEquals("本会话允许", buttons[1].label)
        assertEquals(ApprovalOptionRole.DENY, buttons[2].role)
        assertEquals("拒绝", buttons[2].label)
    }

    @Test
    fun `deny类特征词一律红色_未知id不猜成允许`() {
        val buttons = approvalOptionButtonsOf(
            listOf(ApprovalOptionUi("reject_all", "拒绝全部"), ApprovalOptionUi("DenyOnce"), ApprovalOptionUi("custom_weird", "奇怪选项")),
        )
        // 稳定分区:未知非 deny 选项排前,deny 类殿后且保持 daemon 相对顺序
        assertEquals(ApprovalOptionRole.SECONDARY_ALLOW, buttons[0].role) // 无特征词 → 不猜 deny
        assertEquals("奇怪选项", buttons[0].label) // 未知 id 用 daemon 给的 label
        assertEquals(ApprovalOptionRole.DENY, buttons[1].role) // reject 特征词
        assertEquals(ApprovalOptionRole.DENY, buttons[2].role) // 大小写不敏感的 deni
    }

    @Test
    fun `未知允许类id文案回退id_空label不摆空壳`() {
        val buttons = approvalOptionButtonsOf(listOf(ApprovalOptionUi("acceptReadOnly")))
        assertEquals("acceptReadOnly", buttons[0].label)
        assertEquals(ApprovalOptionRole.SECONDARY_ALLOW, buttons[0].role)
    }

    @Test
    fun `daemon乱序发送_输出仍允许在前拒绝殿后_同类保持原序`() {
        val buttons = approvalOptionButtonsOf(
            listOf(
                ApprovalOptionUi("decline"),
                ApprovalOptionUi("acceptForSession"),
                ApprovalOptionUi("accept"),
            ),
        )
        // 稳定分区:允许类保持 daemon 相对顺序在前,拒绝殿后
        assertEquals(listOf("acceptForSession", "accept", "decline"), buttons.map { it.id })
    }

    // ---- 旧 daemon 兼容:无 options ----

    @Test
    fun `无options返回空映射_走旧两按钮`() {
        assertTrue(approvalOptionButtonsOf(emptyList()).isEmpty())
        val legacy = pendingUi()
        assertTrue(legacy.options.isEmpty())
        assertNull(legacy.details)
        assertNull(legacy.submittedOption)
    }

    // ---- 提交中单按钮回显 ----

    @Test
    fun `提交中回显_旧口径落在主允许或拒绝_不两颗同转`() {
        val bAccept = ApprovalOptionButton("accept", "本次允许", ApprovalOptionRole.PRIMARY_ALLOW)
        val bSession = ApprovalOptionButton("acceptForSession", "本会话允许", ApprovalOptionRole.SECONDARY_ALLOW)
        val bDecline = ApprovalOptionButton("decline", "拒绝", ApprovalOptionRole.DENY)

        // 旧口径(submittedOption=null):允许类回显落主按钮,次允许不亮
        assertTrue(approvalOptionEcho(bAccept, ApprovalPhase.SUBMITTING, true, null))
        assertFalse(approvalOptionEcho(bSession, ApprovalPhase.SUBMITTING, true, null))
        assertFalse(approvalOptionEcho(bDecline, ApprovalPhase.SUBMITTING, true, null))
        // 拒绝回显只给 deny 按钮
        assertTrue(approvalOptionEcho(bDecline, ApprovalPhase.SUBMITTING, false, null))
        assertFalse(approvalOptionEcho(bAccept, ApprovalPhase.SUBMITTING, false, null))
        // 非 SUBMITTING 一律不回显
        assertFalse(approvalOptionEcho(bAccept, ApprovalPhase.PENDING, null, null))
        assertFalse(approvalOptionEcho(bDecline, ApprovalPhase.ALLOWED, true, null))
    }

    @Test
    fun `提交中回显_带选项id严格按id落位`() {
        val bAccept = ApprovalOptionButton("accept", "本次允许", ApprovalOptionRole.PRIMARY_ALLOW)
        val bSession = ApprovalOptionButton("acceptForSession", "本会话允许", ApprovalOptionRole.SECONDARY_ALLOW)
        assertTrue(approvalOptionEcho(bSession, ApprovalPhase.SUBMITTING, true, "acceptForSession"))
        assertFalse(approvalOptionEcho(bAccept, ApprovalPhase.SUBMITTING, true, "acceptForSession"))
    }

    // ---- 状态机:第三按钮的提交链(approved 布尔语义保持) ----

    @Test
    fun `第三按钮提交记录选项id_终态落已允许`() {
        val submitting = pendingUi(options = listOf(ApprovalOptionUi("acceptForSession")))
            .submitting(allow = true, option = "acceptForSession")
        assertEquals(ApprovalPhase.SUBMITTING, submitting.phase)
        assertEquals(true, submitting.submittedChoice)
        assertEquals("acceptForSession", submitting.submittedOption)

        // acceptForSession 也是允许:服务端确认后落 ALLOWED,与旧两按钮同一终态词
        assertEquals(ApprovalPhase.ALLOWED, submitting.confirmed(true).phase)
    }

    @Test
    fun `提交中双击竞态_选项id不被翻转`() {
        val once = pendingUi().submitting(true, "acceptForSession")
        assertEquals(once, once.submitting(false, "decline"))
        assertEquals("acceptForSession", once.submittedOption)
    }

    @Test
    fun `旧两按钮提交_不带选项id_旧口径不变`() {
        val submitting = pendingUi().submitting(true)
        assertEquals(true, submitting.submittedChoice)
        assertNull(submitting.submittedOption)
        assertEquals(ApprovalPhase.ALLOWED, submitting.confirmed(true).phase)
    }

    // ---- proto 读取面:daemon ApprovalRequest options/details → UI ----

    @Test
    fun `proto选项与详情装配进UI值对象`() {
        val proto = ApprovalRequest.newBuilder()
            .setRequestId("req-c1")
            .addOptions(
                ApprovalOption.newBuilder()
                    .setId("acceptForSession")
                    .setLabel("本次及本会话允许")
                    .setDescription("本会话内同类请求不再询问"),
            )
            .addOptions(ApprovalOption.newBuilder().setId("decline").setLabel("拒绝"))
            .setDetails("将把 git push 加入本会话执行白名单")
            .build()
        assertEquals(2, proto.optionsCount)
        assertEquals("将把 git push 加入本会话执行白名单", proto.details)

        val ui = proto.optionsList.toApprovalOptionUi()
        assertEquals("acceptForSession", ui[0].id)
        assertEquals("本次及本会话允许", ui[0].label)
        assertEquals("本会话内同类请求不再询问", ui[0].description)
        assertEquals("decline", ui[1].id)
    }

    // ---- 应答 option 回填:answerApproval 真实通道(replyCh) ----

    @Test
    fun `点第三按钮_decision回填option且approved保持true`() {
        val threadId = "approval-option-c1"
        val st = TurnSession.state(threadId)
        st.pendingApproval = ApprovalRequest.newBuilder().setRequestId("req-c1").build()
        val ch = Channel<TurnClientMessage>(1)
        st.replyCh = ch
        try {
            TurnSession.answerApproval(threadId, "req-c1", approved = true, option = "acceptForSession")
            val msg = ch.tryReceive().getOrNull()
            assertEquals("req-c1", msg?.approval?.requestId)
            assertTrue(msg!!.approval.approved) // 布尔语义保持:第三按钮也是允许
            assertEquals("acceptForSession", msg.approval.option)
        } finally {
            st.replyCh = null
            st.pendingApproval = null
        }
    }

    @Test
    fun `旧两按钮应答_不填option字段_线上旧链路零变化`() {
        val threadId = "approval-option-legacy"
        val st = TurnSession.state(threadId)
        st.pendingApproval = ApprovalRequest.newBuilder().setRequestId("req-old").build()
        val ch = Channel<TurnClientMessage>(1)
        st.replyCh = ch
        try {
            TurnSession.answerApproval(threadId, "req-old", approved = false)
            val msg = ch.tryReceive().getOrNull()
            assertEquals(false, msg?.approval?.approved)
            assertEquals("", msg!!.approval.option) // 空串 = 不设字段 = 旧语义
            assertEquals("", msg.approval.note)
        } finally {
            st.replyCh = null
            st.pendingApproval = null
        }
    }

    private fun pendingUi(
        options: List<ApprovalOptionUi> = emptyList(),
        details: String? = null,
    ) = ApprovalRequestUi(
        requestId = "req-c1",
        threadId = "thread-c1",
        agentName = "Codex",
        sessionTitle = "修复登录流程",
        purpose = "验证登录状态修复",
        workspace = "项目一",
        actionKind = "运行命令",
        command = "git push origin main",
        options = options,
        details = details,
    )
}
