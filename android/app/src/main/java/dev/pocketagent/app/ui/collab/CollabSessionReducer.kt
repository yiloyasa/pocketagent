package dev.pocketagent.app.ui.collab

import dev.pocketagent.app.ui.EditorialStatus
import pocketagent.v1.CollabMemberSnapshot
import pocketagent.v1.CollabSessionEvent
import pocketagent.v1.CollabSessionSnapshot

/**
 * 协同会话纯函数 reducer(P1 协同闭环 Task 6):事件帧/快照 → [CollabSessionState]。
 *
 * 【设计纪律】
 *  - **纯函数、无 IO、无 Android 类**:全部规则 JVM 单测可断言
 *    (CollabSessionReducerTest),改坏任何一条映射在测试里红,而不是在真机上被看见;
 *  - **seq 单调去重**:与 TurnSession.lastSequence 同一条纪律 —— 帧到达即推进
 *    断点,sequence=0(旧 daemon/无序号帧)不推进也不作废断点;
 *  - **缺口不猜**:sequence 跳水(慢消费者被 daemon 丢帧,对端约定)时不拿
 *    半截信息拼状态,标记 needsResync 让仓库层拉全量快照重建;
 *  - **未知 kind 兜底**:proto 注释钉死的协议 —— 客户端遇到不认识的 kind 必须
 *    按 snapshot 兜底刷新,不得报错(PROGRESS 就是词表外增量的先例);
 *  - **daemon 是唯一事实源**:快照整屏重建;成员帧之间会话级状态按 daemon
 *    deriveCollabSessionStatus 同一规则镜像派生,下一帧快照校正。
 */
object CollabSessionReducer {

    // ---- daemon kind 词表(collab_session_events.go 钉住的那套) ----

    /** 状态迁移事件的 kind 与成员/会话状态字符串同域。 */
    val MEMBER_KINDS = setOf(
        "ACCEPTED", "RUNNING", "WAITING_APPROVAL", "COMPLETED", "FAILED", "CANCELLED",
    )

    /** 整屏兜底帧(proto 注释点名)。 */
    const val KIND_SNAPSHOT = "SNAPSHOT"

    /**
     * 词表外的纯增量进度帧:客户端按协议以 snapshot 兜底,不报错。
     * PROGRESS 帧携带成员整卡(进度字段有值),所以可以直接应用该卡。
     */
    const val KIND_PROGRESS = "PROGRESS"

    // ---- 映射:daemon 字符串 → 客户端状态 ----

    /** daemon 状态串 → [CollabStatus];不认识的取值如实落 UNKNOWN(原文由调用方保留)。 */
    fun mapStatus(raw: String): CollabStatus = when (raw) {
        "ACCEPTED" -> CollabStatus.ACCEPTED
        "RUNNING" -> CollabStatus.RUNNING
        "WAITING_APPROVAL" -> CollabStatus.WAITING_APPROVAL
        "COMPLETED" -> CollabStatus.COMPLETED
        "FAILED" -> CollabStatus.FAILED
        "CANCELLED" -> CollabStatus.CANCELLED
        "" -> CollabStatus.UNKNOWN // daemon 未给状态:如实未知,不编
        else -> CollabStatus.UNKNOWN
    }

    /** proto 成员卡 → 客户端成员卡(字段对照只写这一遍)。 */
    fun mapMember(m: CollabMemberSnapshot): CollabMemberUi = CollabMemberUi(
        agentId = m.agentId,
        roleId = m.roleId,
        threadId = m.threadId,
        status = mapStatus(m.status),
        statusRaw = m.status,
        modelId = m.modelId,
        effort = m.effort,
        progress = m.progress,
        summary = m.summary,
        error = m.error,
        pendingApprovals = m.pendingApprovalsList.map { ref ->
            CollabApprovalUi(requestId = ref.requestId, label = ref.reason)
        },
        workspace = m.workspace,
    )

    /** 成员卡在状态树里的定位:有子线程按线程(取消/重试/审批同锚);受理失败的卡没有线程,按 agent 兜位。 */
    private fun CollabMemberUi.key(): String =
        if (threadId.isNotBlank()) "t:$threadId" else "a:$agentId"

    // ---- 快照重建 ----

    /**
     * 全量快照 → 整屏状态(整树替换,无残留)。开屏首拉/缺口重建/带快照帧共用。
     * [lastSequence] 由调用方给:事件帧路径传该帧 seq;GetCollabSession 路径
     * 传当前水位(快照本身不带 seq,不推进也不作废断点)。
     */
    fun fromSnapshot(snapshot: CollabSessionSnapshot, lastSequence: Long): CollabSessionState = CollabSessionState(
        sessionId = snapshot.sessionId.ifBlank { null },
        status = mapStatus(snapshot.status),
        statusRaw = snapshot.status,
        instruction = snapshot.instruction,
        sessionError = snapshot.error,
        workspace = snapshot.workspace,
        members = snapshot.membersList.map { mapMember(it) },
        lastSequence = lastSequence,
        loading = false,
        // 快照是 daemon 真值:会话级错误换新,传输层旧错误一并清掉
        error = null,
        needsResync = false,
    )

    /** 缺口重建:保留现有事件水位(快照可能落后于流),只换整屏内容。 */
    fun resync(state: CollabSessionState, snapshot: CollabSessionSnapshot): CollabSessionState =
        fromSnapshot(snapshot, state.lastSequence)

    // ---- 成员帧应用 ----

    /**
     * 单成员整卡上树(按 [memberKey] 替换,顺序保持受理序)。
     *  - 键不在名册(名册过期/只拿到半张名单)→ needsResync,不擅自追加:
     *    成员数错会连累"N/M"汇总,宁可多拉一次快照;
     *  - 会话级状态按 daemon 同款规则镜像派生(见 [deriveStatus]),
     *    让"N/M 已完成"在两次快照之间也即时成立。
     */
    fun applyMember(state: CollabSessionState, card: CollabMemberSnapshot): CollabSessionState {
        val incoming = mapMember(card)
        val key = incoming.key()
        val idx = state.members.indexOfFirst { it.key() == key }
        val members = if (idx >= 0) {
            state.members.toMutableList().also { it[idx] = incoming }
        } else {
            null
        }
        if (members == null) {
            // 名册里没有这张卡:不追加、不猜,等全量快照重建名册
            return state.copy(needsResync = true)
        }
        return state.copy(
            members = members,
            status = deriveStatus(members),
            statusRaw = "",
            needsResync = state.needsResync,
        )
    }

    // ---- 事件帧 reduce ----

    /**
     * 应用一帧 [CollabSessionEvent]。返回新状态(不可变;不满足应用条件原样返回)。
     */
    fun reduce(state: CollabSessionState, ev: CollabSessionEvent): CollabSessionState {
        val seq = ev.sequence
        // 1) 单调去重:严格更旧的帧直接丢弃,断点不动。
        //    【等序帧的特殊待遇】daemon 的续传超窗兜底帧记 sequence = after(正好
        //    等于客户端请求水位,见 collabCoordinator.Watch 的 gap 补帧)—— 帧内容
        //    是整屏快照,重建幂等,不能按"重复"丢掉;等序但**不带**快照的帧没有
        //    新内容(> 水位的实时帧不会等序到达,daemon 转发器已按序过滤),丢弃。
        if (seq > 0) {
            if (seq < state.lastSequence) return state
            if (seq == state.lastSequence && !ev.hasSnapshot()) return state
        }
        // 2) 串错会话的帧不收(防御;Watch 只订一个会话,正常到不了这)
        if (ev.sessionId.isNotEmpty() && state.sessionId != null && ev.sessionId != state.sessionId) {
            return state
        }
        val watermark = if (seq > 0) seq else state.lastSequence
        // 3) 带全量快照的帧(含 SNAPSHOT kind、续传超窗兜底帧、未知 kind 带快照):整屏重建
        if (ev.hasSnapshot()) {
            return fromSnapshot(ev.snapshot, watermark)
        }
        val isGap = seq > 0 && state.lastSequence > 0 && seq > state.lastSequence + 1
        // 4) 不带快照:按 kind 分派;任何"说明不了自己"的帧都走全量兜底
        return when {
            ev.kind == KIND_SNAPSHOT ->
                // 宣称是快照却没带快照:没有内容可应用,拉全量
                state.copy(lastSequence = watermark, needsResync = true)

            ev.kind == KIND_PROGRESS && ev.hasMember() ->
                // 词表外的增量进度帧:带成员整卡,按协议以卡内容兜底刷新
                applyMember(state, ev.member).copy(lastSequence = watermark)

            ev.kind in MEMBER_KINDS && ev.hasMember() ->
                // 状态迁移帧:整卡真值上树;缺口时同时标记重建
                applyMember(state, ev.member).let {
                    it.copy(lastSequence = watermark, needsResync = it.needsResync || isGap)
                }

            ev.kind in MEMBER_KINDS ->
                // 状态迁移帧没带卡:不知道发生了什么,拉全量
                state.copy(lastSequence = watermark, needsResync = true)

            else ->
                // 未知 kind(词表外新增)且不带快照:协议兜底 = 拉全量快照刷新,
                // 不报错、不猜、不丢水位
                state.copy(lastSequence = watermark, needsResync = true)
        }
    }

    // ---- 派生(与 daemon deriveCollabSessionStatus 同一规则,镜像不发明) ----

    /**
     * 会话级状态由成员卡派生(不独立记账):有人在跑 → RUNNING;有人等审批 →
     * WAITING_APPROVAL;全员终局按 FAILED > CANCELLED > COMPLETED 聚合;
     * 空名册 → ACCEPTED。UNKNOWN 成员不计入任何桶(与 daemon 的 switch 落空一致)。
     */
    fun deriveStatus(members: List<CollabMemberUi>): CollabStatus {
        if (members.isEmpty()) return CollabStatus.ACCEPTED
        var running = false
        var waiting = false
        var failed = false
        var cancelled = false
        for (m in members) {
            when (m.status) {
                CollabStatus.ACCEPTED, CollabStatus.RUNNING -> running = true
                CollabStatus.WAITING_APPROVAL -> waiting = true
                CollabStatus.FAILED -> failed = true
                CollabStatus.CANCELLED -> cancelled = true
                CollabStatus.COMPLETED, CollabStatus.SUBMITTED, CollabStatus.UNKNOWN -> Unit
            }
        }
        return when {
            running -> CollabStatus.RUNNING
            waiting -> CollabStatus.WAITING_APPROVAL
            failed -> CollabStatus.FAILED
            cancelled -> CollabStatus.CANCELLED
            else -> CollabStatus.COMPLETED
        }
    }

    /** 成员计数(汇总行的原料)。 */
    fun counts(members: List<CollabMemberUi>): CollabSessionCounts {
        var completed = 0
        var failed = 0
        var cancelled = 0
        var waiting = 0
        var active = 0
        for (m in members) {
            when (m.status) {
                CollabStatus.COMPLETED -> completed++
                CollabStatus.FAILED -> failed++
                CollabStatus.CANCELLED -> cancelled++
                CollabStatus.WAITING_APPROVAL -> waiting++
                CollabStatus.ACCEPTED, CollabStatus.RUNNING, CollabStatus.SUBMITTED -> active++
                CollabStatus.UNKNOWN -> Unit
            }
        }
        return CollabSessionCounts(
            total = members.size,
            completed = completed,
            failed = failed,
            cancelled = cancelled,
            waiting = waiting,
            active = active,
        )
    }

    // ---- 动作可用性(按钮可点 + 禁用原因同源,不出"灰着但不知道为什么") ----

    /** 单个成员卡的动作可用性:每个入口的可点态与禁用原因由同一个纯函数给出。 */
    data class CollabMemberActions(
        val canOpenSession: Boolean,
        val openSessionReason: String?,
        val canRetry: Boolean,
        val retryReason: String?,
        val canCancel: Boolean,
        val cancelReason: String?,
        /** 有挂起审批 → 打开成员会话去确认(真实 thread_id/request_id)。 */
        val canAnswerApproval: Boolean,
        val approvalReason: String?,
    )

    fun memberActions(m: CollabMemberUi): CollabMemberActions {
        val hasThread = m.threadId.isNotBlank()
        val canRetry = m.status.isFinished && hasThread
        val canCancel = !m.status.isFinished && hasThread
        val canApproval = m.pendingApprovals.isNotEmpty() && hasThread
        return CollabMemberActions(
            canOpenSession = hasThread,
            openSessionReason = if (hasThread) null else CollabCopy.REASON_OPEN_NO_THREAD,
            canRetry = canRetry,
            retryReason = when {
                canRetry -> null
                !hasThread -> CollabCopy.REASON_NO_THREAD
                else -> CollabCopy.REASON_RETRY_ACTIVE
            },
            canCancel = canCancel,
            cancelReason = when {
                canCancel -> null
                !hasThread -> CollabCopy.REASON_NO_THREAD
                else -> CollabCopy.REASON_CANCEL_FINISHED
            },
            canAnswerApproval = canApproval,
            approvalReason = if (canApproval) null else CollabCopy.REASON_APPROVAL_NONE,
        )
    }

    /** 会话级动作(取消整个协同)。 */
    data class CollabSessionActions(
        val canCancel: Boolean,
        val cancelReason: String?,
    )

    fun sessionActions(state: CollabSessionState): CollabSessionActions {
        val canCancel = state.members.isNotEmpty() && state.members.any { !it.status.isFinished }
        return CollabSessionActions(
            canCancel = canCancel,
            cancelReason = if (canCancel) null else CollabCopy.REASON_SESSION_ALL_FINISHED,
        )
    }

    // ---- 四词徽章桥接(七词词表 → 既有 PaStatusBadge 的形/色通道) ----

    /**
     * 七词状态 → 四词徽章(只借 EditorialStatus 的色档与形通道,**词**仍用
     * CollabCopy.statusWord 的七词)。UNKNOWN 返回 null:未知状态不进任何已知
     * 徽章,原样显示 + 降级说明。
     */
    fun editorialBadgeOf(status: CollabStatus): EditorialStatus? = when (status) {
        CollabStatus.SUBMITTED, CollabStatus.ACCEPTED, CollabStatus.RUNNING -> EditorialStatus.RUNNING
        CollabStatus.WAITING_APPROVAL -> EditorialStatus.WAITING_FOR_YOU
        CollabStatus.COMPLETED -> EditorialStatus.COMPLETED
        CollabStatus.FAILED, CollabStatus.CANCELLED -> EditorialStatus.NOT_COMPLETED
        CollabStatus.UNKNOWN -> null
    }

    /** 空状态(进入页面、尚未恢复/尚未派发)。 */
    fun initial(): CollabSessionState = CollabSessionState()
}
