package dev.pocketagent.app.ui.collab

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.ApprovalRequest
import pocketagent.v1.CollabMemberSnapshot
import pocketagent.v1.CollabSessionEvent
import pocketagent.v1.CollabSessionSnapshot

/**
 * 协同会话 reducer 纯函数钉子(P1 协同闭环 Task 6/7)。
 *
 * 覆盖面(与计划验收逐条对应):
 *  1. 快照 → 整屏状态(字段对照 + 成员卡映射 + 挂起审批映射);
 *  2. daemon 状态串 → 七词词表(已知映射 + 未知降级);
 *  3. 成员全轨迹 ACCEPTED→RUNNING→COMPLETED / FAILED / CANCELLED / WAITING_APPROVAL,
 *     会话级状态按 daemon 同款规则派生(部分终局:2 完成 1 等待 → 会话等待确认);
 *  4. seq 单调去重(TurnSession.lastSequence 同纪律;sequence=0 不推进水位);
 *  5. seq 缺口 → needsResync 标记;带快照的帧直接整屏重建;
 *  6. 未知 kind / 不带卡的状态帧 → 按协议以快照兜底(needsResync),不报错不猜;
 *  7. 成员卡动作可用性(重试/取消/查看会话/去确认)与禁用原因同源;
 *  8. 汇总行 "N/M 已完成 · X 个等待确认"。
 */
class CollabSessionReducerTest {

    // ---- 构造辅助(proto lite builder,JVM 直跑) ----

    private fun member(
        agentId: String = "pi",
        threadId: String = "t-$agentId",
        status: String = "RUNNING",
        progress: String = "",
        summary: String = "",
        error: String = "",
        approvals: List<Pair<String, String>> = emptyList(),
    ): CollabMemberSnapshot {
        val b = CollabMemberSnapshot.newBuilder()
            .setAgentId(agentId)
            .setRoleId("")
            .setThreadId(threadId)
            .setStatus(status)
            .setProgress(progress)
            .setSummary(summary)
            .setError(error)
        approvals.forEach { (rid, label) ->
            b.addPendingApprovals(ApprovalRequest.newBuilder().setRequestId(rid).setReason(label))
        }
        return b.build()
    }

    // memberActions 的入参是成员卡(CollabMemberUi);经 reducer 自带的 mapMember 走
    // 同一条 pb→UI 映射,保证测试与生产用同一份换算,不另造第二份映射。
    private fun memberUi(
        agentId: String = "pi",
        threadId: String = "t-$agentId",
        status: String = "RUNNING",
        progress: String = "",
        summary: String = "",
        error: String = "",
        approvals: List<Pair<String, String>> = emptyList(),
    ) = CollabSessionReducer.mapMember(member(agentId, threadId, status, progress, summary, error, approvals))

    private fun snapshot(
        sessionId: String = "collab-1",
        status: String = "RUNNING",
        instruction: String = "把回归测试跑一轮",
        error: String = "",
        members: List<CollabMemberSnapshot> = emptyList(),
    ): CollabSessionSnapshot {
        val b = CollabSessionSnapshot.newBuilder()
            .setSessionId(sessionId)
            .setInstruction(instruction)
            .setStatus(status)
            .setWorkspace("/repo")
            .setError(error)
        members.forEach { b.addMembers(it) }
        return b.build()
    }

    private fun event(
        sequence: Long,
        kind: String,
        sessionId: String = "collab-1",
        member: CollabMemberSnapshot? = null,
        snap: CollabSessionSnapshot? = null,
    ): CollabSessionEvent {
        val b = CollabSessionEvent.newBuilder()
            .setSequence(sequence)
            .setSessionId(sessionId)
            .setKind(kind)
        member?.let { b.setMember(it) }
        snap?.let { b.setSnapshot(it) }
        return b.build()
    }

    /** 三成员起点:pi RUNNING / codex RUNNING / zcode RUNNING,水位=5。 */
    private fun threeRunning(): CollabSessionState =
        CollabSessionReducer.fromSnapshot(
            snapshot(
                members = listOf(
                    member(agentId = "pi", status = "RUNNING", progress = "正在执行任务"),
                    member(agentId = "codex", status = "RUNNING"),
                    member(agentId = "zcode", status = "RUNNING"),
                ),
            ),
            lastSequence = 5L,
        )

    // ---- 1) 快照 → 整屏状态 ----

    @Test
    fun `快照整屏重建_字段对照只认daemon真值`() {
        val snap = snapshot(
            status = "WAITING_APPROVAL",
            members = listOf(
                member(agentId = "pi", status = "WAITING_APPROVAL", approvals = listOf("req-1" to "工具 shell")),
                member(agentId = "codex", status = "COMPLETED", summary = "结论:A"),
            ),
            error = "1 个成员未完成,第一例原因:X",
        )
        val st = CollabSessionReducer.fromSnapshot(snap, lastSequence = 7L)
        assertEquals("collab-1", st.sessionId)
        assertEquals(CollabStatus.WAITING_APPROVAL, st.status)
        assertEquals("把回归测试跑一轮", st.instruction)
        assertEquals("/repo", st.workspace)
        assertEquals("1 个成员未完成,第一例原因:X", st.sessionError)
        assertEquals(7L, st.lastSequence)
        assertEquals(2, st.members.size)
        assertFalse(st.loading)
        assertNull(st.error)
        assertFalse(st.needsResync)
        // 挂起审批映射到成员卡:真实 request_id 原样保留(审批红线)
        val pi = st.members.first { it.agentId == "pi" }
        assertEquals(1, pi.pendingApprovals.size)
        assertEquals("req-1", pi.pendingApprovals[0].requestId)
        assertEquals("工具 shell", pi.pendingApprovals[0].label)
        // 成功摘要/失败原因各归各位
        val codex = st.members.first { it.agentId == "codex" }
        assertEquals("结论:A", codex.summary)
        assertEquals("", codex.error)
    }

    // ---- 2) daemon 状态串 → 七词词表 ----

    @Test
    fun `daemon状态串映射_六个已知词加空串降级`() {
        assertEquals(CollabStatus.ACCEPTED, CollabSessionReducer.mapStatus("ACCEPTED"))
        assertEquals(CollabStatus.RUNNING, CollabSessionReducer.mapStatus("RUNNING"))
        assertEquals(CollabStatus.WAITING_APPROVAL, CollabSessionReducer.mapStatus("WAITING_APPROVAL"))
        assertEquals(CollabStatus.COMPLETED, CollabSessionReducer.mapStatus("COMPLETED"))
        assertEquals(CollabStatus.FAILED, CollabSessionReducer.mapStatus("FAILED"))
        assertEquals(CollabStatus.CANCELLED, CollabSessionReducer.mapStatus("CANCELLED"))
        // daemon 未给状态(旧版本):如实未知,不编
        assertEquals(CollabStatus.UNKNOWN, CollabSessionReducer.mapStatus(""))
    }

    @Test
    fun `未知状态串落UNKNOWN并保留原文降级`() {
        val st = CollabSessionReducer.fromSnapshot(snapshot(status = "SUPERSEDED_V2"), lastSequence = 1L)
        assertEquals(CollabStatus.UNKNOWN, st.status)
        assertEquals("SUPERSEDED_V2", st.statusRaw)
        // 词表降级:认识的用中文名,不认识的原样显示原文(不猜相近词)
        assertEquals("SUPERSEDED_V2", CollabCopy.statusWord(st.status, st.statusRaw))
    }

    @Test
    fun `七词词表_一一对齐且不出现禁用裸词`() {
        assertEquals("已提交", CollabCopy.statusWord(CollabStatus.SUBMITTED))
        assertEquals("已受理", CollabCopy.statusWord(CollabStatus.ACCEPTED))
        assertEquals("执行中", CollabCopy.statusWord(CollabStatus.RUNNING))
        assertEquals("等待确认", CollabCopy.statusWord(CollabStatus.WAITING_APPROVAL))
        assertEquals("已完成", CollabCopy.statusWord(CollabStatus.COMPLETED))
        assertEquals("未完成", CollabCopy.statusWord(CollabStatus.FAILED))
        assertEquals("已取消", CollabCopy.statusWord(CollabStatus.CANCELLED))
        // 四词纪律:失败/取消都不得显示「失败/错误」裸词
        assertFalse(CollabCopy.statusWord(CollabStatus.FAILED).contains("失败"))
        assertFalse(CollabCopy.statusWord(CollabStatus.FAILED).contains("错误"))
    }

    // ---- 3) 成员全轨迹与会话级派生 ----

    @Test
    fun `全轨迹_ACCEPTED到RUNNING到COMPLETED_会话随最后一个成员收敛`() {
        var st = CollabSessionReducer.fromSnapshot(
            snapshot(members = listOf(member(agentId = "pi", status = "ACCEPTED"))),
            lastSequence = 1L,
        )
        assertEquals(CollabStatus.ACCEPTED, st.members[0].status)
        assertEquals(CollabStatus.RUNNING, st.status) // daemon 派生:受理即有成员在跑
        st = CollabSessionReducer.reduce(
            st,
            event(2, "RUNNING", member = member(agentId = "pi", status = "RUNNING", progress = "正在执行任务")),
        )
        assertEquals(CollabStatus.RUNNING, st.members[0].status)
        assertEquals("正在执行任务", st.members[0].progress)
        assertEquals(2L, st.lastSequence)
        st = CollabSessionReducer.reduce(
            st,
            event(3, "COMPLETED", member = member(agentId = "pi", status = "COMPLETED", summary = "全部通过")),
        )
        assertEquals(CollabStatus.COMPLETED, st.members[0].status)
        assertTrue(st.members[0].status.isFinished)
        assertEquals(CollabStatus.COMPLETED, st.status)
        assertEquals("全部通过", st.members[0].summary)
    }

    @Test
    fun `全轨迹_FAILED与CANCELLED_会话聚合保持优先级`() {
        // 2 完成 + 1 失败 → 会话如实 FAILED(daemon 同口径:任一失败即整体 FAILED)
        var st = threeRunning()
        st = CollabSessionReducer.reduce(st, event(6, "COMPLETED", member = member(agentId = "pi", status = "COMPLETED", summary = "A")))
        st = CollabSessionReducer.reduce(st, event(7, "COMPLETED", member = member(agentId = "codex", status = "COMPLETED", summary = "B")))
        st = CollabSessionReducer.reduce(st, event(8, "FAILED", member = member(agentId = "zcode", status = "FAILED", error = "端口被占")))
        assertEquals(CollabStatus.FAILED, st.status)
        assertEquals("端口被占", st.members[2].error)
        assertEquals("", st.members[2].summary)
        // 2 完成 + 1 取消 → 会话 CANCELLED
        var st2 = threeRunning()
        st2 = CollabSessionReducer.reduce(st2, event(6, "COMPLETED", member = member(agentId = "pi", status = "COMPLETED", summary = "A")))
        st2 = CollabSessionReducer.reduce(st2, event(7, "COMPLETED", member = member(agentId = "codex", status = "COMPLETED", summary = "B")))
        st2 = CollabSessionReducer.reduce(st2, event(8, "CANCELLED", member = member(agentId = "zcode", status = "CANCELLED")))
        assertEquals(CollabStatus.CANCELLED, st2.status)
    }

    @Test
    fun `全轨迹_WAITING_APPROVAL_一个成员等审批其他成员继续跑不被误报完成`() {
        // codex 完成、zcode 等审批:会话仍不是终局(daemon:等待不是终局)
        var st = threeRunning()
        st = CollabSessionReducer.reduce(st, event(6, "COMPLETED", member = member(agentId = "pi", status = "COMPLETED", summary = "A")))
        st = CollabSessionReducer.reduce(st, event(7, "COMPLETED", member = member(agentId = "codex", status = "COMPLETED", summary = "B")))
        assertEquals(CollabStatus.RUNNING, st.status) // zcode 还在跑
        st = CollabSessionReducer.reduce(
            st,
            event(
                8, "WAITING_APPROVAL",
                member = member(
                    agentId = "zcode", status = "WAITING_APPROVAL",
                    approvals = listOf("req-9" to "工具 shell"),
                ),
            ),
        )
        assertEquals(CollabStatus.WAITING_APPROVAL, st.status)
        assertEquals(1, st.members[2].pendingApprovals.size)
        // 多成员部分终局(2 完成 1 等待)的汇总行:绝不显示笼统"运行中"
        val line = CollabCopy.summaryLine(CollabSessionReducer.counts(st.members))
        assertEquals("2/3 已完成 · 1 个等待确认", line)
        // 审批应答后回 RUNNING(daemon 权威迁移的镜像)
        st = CollabSessionReducer.reduce(
            st,
            event(9, "RUNNING", member = member(agentId = "zcode", status = "RUNNING")),
        )
        assertEquals(CollabStatus.RUNNING, st.members[2].status)
        assertEquals(0, st.members[2].pendingApprovals.size)
    }

    @Test
    fun `会话派生_EMPTY名册回ACCEPTED_UNKNOWN成员不进任何桶`() {
        assertEquals(CollabStatus.ACCEPTED, CollabSessionReducer.deriveStatus(emptyList()))
        // 未知状态成员(新版本串):与 daemon 的 switch 落空一致,不改变派生结果(空桶 → COMPLETED)
        val allUnknown = listOf(
            CollabSessionReducer.mapMember(member(agentId = "pi", status = "SUPERSEDED_V2")),
        )
        assertEquals(CollabStatus.UNKNOWN, allUnknown[0].status)
        assertEquals(CollabStatus.COMPLETED, CollabSessionReducer.deriveStatus(allUnknown))
    }

    // ---- 4) seq 单调去重(TurnSession.lastSequence 同纪律) ----

    @Test
    fun `seq去重_旧帧与重复帧丢弃且水位不动`() {
        val st = threeRunning() // 水位 5
        val fresh = event(6, "COMPLETED", member = member(agentId = "pi", status = "COMPLETED", summary = "A"))
        val once = CollabSessionReducer.reduce(st, fresh)
        assertEquals(6L, once.lastSequence)
        // 重复帧(登记-回放窗口):丢弃,状态不回退
        val twice = CollabSessionReducer.reduce(once, fresh)
        assertEquals(once, twice)
        // 迟到帧(seq 更小):丢弃
        val late = CollabSessionReducer.reduce(once, event(3, "FAILED", member = member(agentId = "pi", status = "FAILED")))
        assertEquals(once, late)
        assertEquals(CollabStatus.COMPLETED, late.members.first { it.agentId == "pi" }.status)
    }

    @Test
    fun `seq为0的帧应用内容但不作废水位`() {
        val st = threeRunning() // 水位 5
        val applied = CollabSessionReducer.reduce(
            st,
            event(0, "PROGRESS", member = member(agentId = "pi", status = "RUNNING", progress = "新进度")),
        )
        assertEquals(5L, applied.lastSequence) // 旧 daemon 无序号帧不推进
        assertEquals("新进度", applied.members.first { it.agentId == "pi" }.progress)
    }

    @Test
    fun `续传超窗兜底帧_等序快照帧应用重建且水位不越权`() {
        // daemon Watch 的 gap 补帧记 sequence = after(正好等于客户端水位,
        // collabCoordinator.Watch 的约定):不能按"重复帧"丢掉 —— 整屏重建幂等。
        val st = threeRunning() // 水位 5,客户端以 after=5 续传,daemon 补一帧 seq=5 的快照
        val filler = CollabSessionReducer.reduce(
            st,
            event(5, "SNAPSHOT", snap = snapshot(status = "CANCELLED", members = listOf(
                member(agentId = "pi", status = "COMPLETED", summary = "A"),
                member(agentId = "codex", status = "COMPLETED", summary = "B"),
                member(agentId = "zcode", status = "CANCELLED"),
            ))),
        )
        assertFalse(filler.needsResync)
        assertEquals(5L, filler.lastSequence) // 水位不越权(快照帧自带 seq=after)
        assertEquals(CollabStatus.CANCELLED, filler.status)
        // 等序但**不带**快照的帧没有新内容:丢弃
        val stray = CollabSessionReducer.reduce(
            filler,
            event(5, "COMPLETED", member = member(agentId = "pi", status = "COMPLETED")),
        )
        assertEquals(filler, stray)
    }

    @Test
    fun `串错会话的帧不收`() {
        val st = threeRunning()
        val stranger = CollabSessionReducer.reduce(
            st,
            event(9, "COMPLETED", sessionId = "collab-other", member = member(agentId = "pi", status = "COMPLETED")),
        )
        assertEquals(st, stranger)
    }

    // ---- 5) seq 缺口 → 全量重建 ----

    @Test
    fun `seq缺口不带快照_标记needsResync专名册外的卡不擅自追加`() {
        val st = threeRunning() // 水位 5
        // 缺口(5 → 9)且带的是名册内成员:应用该卡 + 标记重建
        val gapped = CollabSessionReducer.reduce(
            st,
            event(9, "FAILED", member = member(agentId = "zcode", status = "FAILED", error = "超时")),
        )
        assertTrue(gapped.needsResync)
        assertEquals(9L, gapped.lastSequence)
        assertEquals(CollabStatus.FAILED, gapped.members.first { it.agentId == "zcode" }.status)
        // 名册外的成员(半张名单):不追加、不猜,等全量重建
        val strangerCard = CollabSessionReducer.reduce(
            threeRunning(),
            event(6, "COMPLETED", member = member(agentId = "newcomer", status = "COMPLETED", summary = "X")),
        )
        assertTrue(strangerCard.needsResync)
        assertEquals(3, strangerCard.members.size) // 名册没被污染
    }

    @Test
    fun `seq缺口带快照_直接整屏重建不再标记`() {
        val st = threeRunning() // 水位 5
        val rebuilt = CollabSessionReducer.reduce(
            st,
            event(9, "SNAPSHOT", snap = snapshot(status = "COMPLETED", members = listOf(member(agentId = "pi", status = "COMPLETED", summary = "全绿")))),
        )
        assertFalse(rebuilt.needsResync)
        assertEquals(9L, rebuilt.lastSequence)
        assertEquals(1, rebuilt.members.size)
        assertEquals(CollabStatus.COMPLETED, rebuilt.status)
    }

    @Test
    fun `resync_拉回全量快照_水位保留且标记清空`() {
        val gapped = CollabSessionReducer.reduce(
            threeRunning(),
            event(9, "FAILED", member = member(agentId = "zcode", status = "FAILED", error = "超时")),
        )
        assertTrue(gapped.needsResync)
        val rebuilt = CollabSessionReducer.resync(
            gapped,
            snapshot(
                status = "FAILED",
                error = "1 个成员未完成,第一例原因:超时",
                members = listOf(
                    member(agentId = "pi", status = "COMPLETED", summary = "A"),
                    member(agentId = "codex", status = "COMPLETED", summary = "B"),
                    member(agentId = "zcode", status = "FAILED", error = "超时"),
                ),
            ),
        )
        assertFalse(rebuilt.needsResync)
        assertEquals(9L, rebuilt.lastSequence) // 快照不带 seq:水位不动,流继续续传
        assertEquals(3, rebuilt.members.size)
        assertEquals(CollabStatus.FAILED, rebuilt.status)
        assertTrue(rebuilt.sessionError.contains("超时"))
    }

    // ---- 6) 未知 kind / 词表外帧的协议兜底 ----

    @Test
    fun `未知kind不带快照_拉全量兜底不报错不猜`() {
        val st = threeRunning()
        val next = CollabSessionReducer.reduce(st, event(6, "SOME_NEW_KIND_V2"))
        assertEquals(6L, next.lastSequence)
        assertTrue(next.needsResync)
        assertEquals(st.members, next.members) // 没有可应用的内容:状态一字不改
    }

    @Test
    fun `PROGRESS帧_词表外增量kind_按协议以成员卡兜底刷新`() {
        val st = threeRunning()
        val next = CollabSessionReducer.reduce(
            st,
            event(6, "PROGRESS", member = member(agentId = "codex", status = "RUNNING", progress = "正在使用工具 shell")),
        )
        assertFalse(next.needsResync) // PROGRESS 带整卡真值,不需要全量
        assertEquals("正在使用工具 shell", next.members.first { it.agentId == "codex" }.progress)
        assertEquals(CollabStatus.RUNNING, next.status)
    }

    @Test
    fun `状态帧没带成员卡_拉全量兜底`() {
        val next = CollabSessionReducer.reduce(threeRunning(), event(6, "COMPLETED"))
        assertTrue(next.needsResync)
        assertEquals(6L, next.lastSequence)
    }

    // ---- 7) 动作可用性 + 禁用原因同源 ----

    @Test
    fun `成员动作_执行中_可取消不可重试且各有原因`() {
        val a = CollabSessionReducer.memberActions(memberUi(agentId = "pi", status = "RUNNING"))
        assertTrue(a.canCancel)
        assertNull(a.cancelReason)
        assertFalse(a.canRetry)
        assertEquals(CollabCopy.REASON_RETRY_ACTIVE, a.retryReason)
        assertTrue(a.canOpenSession)
        assertNull(a.openSessionReason)
        assertFalse(a.canAnswerApproval)
        assertEquals(CollabCopy.REASON_APPROVAL_NONE, a.approvalReason)
    }

    @Test
    fun `成员动作_已终局_可重试无需取消`() {
        val a = CollabSessionReducer.memberActions(memberUi(agentId = "pi", status = "COMPLETED"))
        assertTrue(a.canRetry)
        assertNull(a.retryReason)
        assertFalse(a.canCancel)
        assertEquals(CollabCopy.REASON_CANCEL_FINISHED, a.cancelReason)
        // 取消成员的幂等语义(3 种终局一致)
        assertEquals(CollabCopy.REASON_CANCEL_FINISHED, CollabSessionReducer.memberActions(memberUi(status = "FAILED")).cancelReason)
        assertEquals(CollabCopy.REASON_CANCEL_FINISHED, CollabSessionReducer.memberActions(memberUi(status = "CANCELLED")).cancelReason)
    }

    @Test
    fun `成员动作_受理失败的卡没有线程_全部入口给原因`() {
        val a = CollabSessionReducer.memberActions(memberUi(agentId = "pi", threadId = "", status = "FAILED", error = "角色不存在"))
        assertFalse(a.canRetry)
        assertEquals(CollabCopy.REASON_NO_THREAD, a.retryReason)
        assertFalse(a.canCancel)
        assertEquals(CollabCopy.REASON_NO_THREAD, a.cancelReason)
        assertFalse(a.canOpenSession)
        assertEquals(CollabCopy.REASON_OPEN_NO_THREAD, a.openSessionReason)
    }

    @Test
    fun `成员动作_有挂起审批_给去确认入口`() {
        val a = CollabSessionReducer.memberActions(
            memberUi(agentId = "pi", status = "WAITING_APPROVAL", approvals = listOf("req-1" to "工具 shell")),
        )
        assertTrue(a.canAnswerApproval)
        assertNull(a.approvalReason)
        assertFalse(a.canRetry)
        assertTrue(a.canCancel)
    }

    @Test
    fun `会话动作_全员终局禁用取消并给原因`() {
        val active = CollabSessionReducer.sessionActions(threeRunning())
        assertTrue(active.canCancel)
        assertNull(active.cancelReason)
        val allDone = CollabSessionReducer.sessionActions(
            CollabSessionReducer.fromSnapshot(
                snapshot(members = listOf(
                    member(agentId = "pi", status = "COMPLETED"),
                    member(agentId = "codex", status = "CANCELLED"),
                )),
                lastSequence = 1L,
            ),
        )
        assertFalse(allDone.canCancel)
        assertEquals(CollabCopy.REASON_SESSION_ALL_FINISHED, allDone.cancelReason)
    }

    // ---- 8) 汇总行与徽章桥接 ----

    @Test
    fun `汇总行_比例永远在_分段按真相追加`() {
        assertNull(CollabCopy.summaryLine(null))
        assertNull(CollabCopy.summaryLine(CollabSessionCounts(0, 0, 0, 0, 0, 0))) // 空会话不占位
        assertEquals("0/3 已完成", CollabCopy.summaryLine(CollabSessionReducer.counts(threeRunning().members)))
        assertEquals("3/3 已完成", CollabCopy.summaryLine(
            CollabSessionReducer.counts(listOf(
                member(agentId = "pi", status = "COMPLETED"),
                member(agentId = "codex", status = "COMPLETED"),
                member(agentId = "zcode", status = "COMPLETED"),
            ).map { CollabSessionReducer.mapMember(it) })
        ))
        // 失败也算"未完成"段(不是"失败"裸词)
        assertEquals("1/3 已完成 · 2 个未完成", CollabCopy.summaryLine(
            CollabSessionReducer.counts(listOf(
                member(agentId = "pi", status = "COMPLETED"),
                member(agentId = "codex", status = "FAILED"),
                member(agentId = "zcode", status = "CANCELLED"),
            ).map { CollabSessionReducer.mapMember(it) })
        ))
    }

    @Test
    fun `四词徽章桥接_七词借色形通道_未知不进徽章`() {
        assertEquals(dev.pocketagent.app.ui.EditorialStatus.RUNNING, CollabSessionReducer.editorialBadgeOf(CollabStatus.RUNNING))
        assertEquals(dev.pocketagent.app.ui.EditorialStatus.RUNNING, CollabSessionReducer.editorialBadgeOf(CollabStatus.ACCEPTED))
        assertEquals(dev.pocketagent.app.ui.EditorialStatus.WAITING_FOR_YOU, CollabSessionReducer.editorialBadgeOf(CollabStatus.WAITING_APPROVAL))
        assertEquals(dev.pocketagent.app.ui.EditorialStatus.COMPLETED, CollabSessionReducer.editorialBadgeOf(CollabStatus.COMPLETED))
        assertEquals(dev.pocketagent.app.ui.EditorialStatus.NOT_COMPLETED, CollabSessionReducer.editorialBadgeOf(CollabStatus.FAILED))
        assertEquals(dev.pocketagent.app.ui.EditorialStatus.NOT_COMPLETED, CollabSessionReducer.editorialBadgeOf(CollabStatus.CANCELLED))
        assertNull(CollabSessionReducer.editorialBadgeOf(CollabStatus.UNKNOWN))
    }

    @Test
    fun `初始状态_SUBMITTED含loading为假_会话为空`() {
        val st = CollabSessionReducer.initial()
        assertNull(st.sessionId)
        assertEquals(CollabStatus.SUBMITTED, st.status)
        assertEquals(0L, st.lastSequence)
        assertFalse(st.loading)
        assertFalse(st.needsResync)
        assertTrue(st.members.isEmpty())
    }
}
