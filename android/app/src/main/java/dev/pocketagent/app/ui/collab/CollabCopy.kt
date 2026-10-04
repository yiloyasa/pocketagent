package dev.pocketagent.app.ui.collab

/**
 * 协同会话的人话文案层(P1 协同闭环 Task 7)。
 *
 * 【它管什么】
 *  1. 七词状态词表(已提交/已受理/执行中/等待确认/已完成/未完成/已取消)——
 *     daemon 状态字符串 → 人话的**唯一转换点**;
 *  2. 汇总行"N/M 已完成 · X 个等待确认" —— 绝不显示笼统的"运行中";
 *  3. 每个禁用按钮旁的禁用原因(按钮灰 = 旁边一定有原因,与既有
 *     collabDispatchDecision 同纪律);
 *  4. 结果分区文案(成员原始结果 vs 汇总 Agent 结果)与"不自动合并 diff"的说明;
 *  5. 未知 kind / 未知状态的降级文案(原文照登 + 人话说明,不猜 —— 与 P2
 *     未知事件"认识的用中文名,不认识的原样显示,不丢弃"同一风格)。
 *
 * 【与 ui.CollabCopy 的关系】ui/CollabLogic.kt 里的旧 CollabCopy 管"派发前"
 * 的选人/派发按钮文案,本文件管"派发后"的会话/成员卡文案;两份各自成域,
 * 互不引用 —— CollabScreen 里以别名 import 区分(SessionCopy)。
 */
object CollabCopy {

    // ---- 七词状态词表(Task 7 统一口径;顺序即生命周期) ----

    const val WORD_SUBMITTED = "已提交"
    const val WORD_ACCEPTED = "已受理"
    const val WORD_RUNNING = "执行中"
    const val WORD_WAITING_APPROVAL = "等待确认"
    const val WORD_COMPLETED = "已完成"
    const val WORD_FAILED = "未完成"
    const val WORD_CANCELLED = "已取消"

    /**
     * 状态 → 人话词。UNKNOWN 不编词:返回 daemon 原文(认识的中文化,
     * 不认识的原样显示);原文也空时给一句如实的"没有给出状态"。
     * [raw] 由调用方从 CollabMemberUi.statusRaw / CollabSessionState.statusRaw 传入。
     */
    fun statusWord(status: CollabStatus, raw: String? = null): String = when (status) {
        CollabStatus.SUBMITTED -> WORD_SUBMITTED
        CollabStatus.ACCEPTED -> WORD_ACCEPTED
        CollabStatus.RUNNING -> WORD_RUNNING
        CollabStatus.WAITING_APPROVAL -> WORD_WAITING_APPROVAL
        CollabStatus.COMPLETED -> WORD_COMPLETED
        CollabStatus.FAILED -> WORD_FAILED
        CollabStatus.CANCELLED -> WORD_CANCELLED
        CollabStatus.UNKNOWN ->
            raw?.takeIf { it.isNotBlank() } ?: "(电脑端没有给出状态)"
    }

    // ---- 未知 kind / 未知状态降级(不猜,原文 + 人话说明) ----

    /** 未知状态说明(跟在原文后面;对齐 P2 未知事件的降级风格)。 */
    const val UNKNOWN_STATUS_NOTE =
        "电脑端返回了新版本的状态,当前版本还没有对应的展示方式;以上是它的原文"

    /** 未知事件 kind 的说明(客户端已按协议以快照兜底刷新,这里解释"为什么没变样")。 */
    const val UNKNOWN_EVENT_NOTE =
        "电脑端发来了新版本的事件,当前版本还没有对应的展示方式;已按最新快照显示"

    // ---- 断线与恢复(Task 6:保留最后一个真快照 + error 呈现) ----

    /** 断线呈现:快照保留,如实说"显示的是最后状态"。 */
    const val DISCONNECT_NOTICE = "连接中断,显示的是最后状态;恢复连接后会自动跟进"

    /** 进入页面/重开页面时首次恢复的等待文案。 */
    const val LOADING_RESTORE = "正在从电脑端恢复协同状态…"

    /** 缺口重建(GetCollabSession 兜底)失败的呈现:保留现状,不猜。 */
    const val RESYNC_FAILED =
        "暂时拉不到最新协同状态,显示的是最后已确认状态;稍后会自动再试"

    /** 派发已受理回执未达时(七词之"已提交")的一句话补充。 */
    const val PROGRESS_SUBMITTED = "指令已提交给电脑端,等待受理回执"

    // ---- 汇总行(绝不显示笼统的"运行中") ----

    /**
     * "N/M 已完成 · X 个等待确认"汇总行。
     *  - 比例段永远在(0/3 也如实说);
     *  - 有等待确认的成员才追加"等待确认"段;
     *  - 有未完成(失败/取消)的成员才追加"未完成"段;
     *  - 没有成员(null 会话)返回 null,不占位。
     */
    fun summaryLine(counts: CollabSessionCounts?): String? {
        if (counts == null || counts.total <= 0) return null
        val parts = mutableListOf("${counts.completed}/${counts.total} $WORD_COMPLETED")
        if (counts.waiting > 0) parts.add("${counts.waiting} 个$WORD_WAITING_APPROVAL")
        val notCompleted = counts.failed + counts.cancelled
        if (notCompleted > 0) parts.add("$notCompleted 个$WORD_FAILED")
        return parts.joinToString(" · ")
    }

    // ---- 结果分区(Task 7:成员原始结果 vs 汇总 Agent 结果) ----

    const val SECTION_MEMBER_RESULTS = "成员原始结果"
    const val SECTION_SUMMARY_RESULT = "汇总 Agent 结果"

    /** 结果区空态:还没有成员收尾(诚实等待,不伪造结果)。 */
    const val MEMBER_RESULTS_EMPTY = "还没有成员收尾;每位成员完成后,它的原始结果会列在这里"

    /** 诚实空态:P1 协同没有汇总 Agent,不伪造一份"汇总"。 */
    const val SUMMARY_RESULT_NONE =
        "本次协同没有安排汇总 Agent;各成员的结果在上方分别列出,由你自行比较"

    /** diff 纪律:不自动合并,先比较/查看,再由用户选择。 */
    const val DIFF_MERGE_NOTICE =
        "各成员对文件的改动不会自动合并;先逐个查看、比较,再决定采用哪一份"

    /** 成员结果行的"查看"入口(打开该成员会话,真实导航)。 */
    const val VIEW_RESULT = "查看结果"

    /**
     * 比较 / 选择入口的文案(计划 Task 7 要求备齐)。P1 没有比较页与采用动作,
     * **不摆假入口** —— 只放文案常量,等 P2 的比较视图落地后接线。
     */
    const val COMPARE_RESULTS = "比较结果"
    const val CHOOSE_RESULT = "选择采用"

    // ---- 成员卡与会话级动作 ----

    const val ACTION_RETRY = "重试该成员"
    const val ACTION_CANCEL_MEMBER = "取消该成员"
    const val ACTION_CANCEL_SESSION = "取消整个协同"
    const val ACTION_OPEN_APPROVAL = "去确认"
    const val ACTION_VIEW_REASON = "查看原因"
    const val ACTION_HIDE_REASON = "收起原因"
    const val ACTION_OPEN_SESSION = "查看会话"
    const val ACTION_OPEN_THREAD = "打开会话"

    /** 挂起审批行(成员卡上):前缀 + daemon 的动作短句。 */
    const val APPROVAL_WAITING_PREFIX = "等待你确认:"

    // ---- 禁用原因(每个禁用按钮旁显示;与 daemon 侧前置条件同口径) ----

    /** 重试:daemon 只允许对终局成员重试(RetryCollabMember 前置条件)。 */
    const val REASON_RETRY_ACTIVE = "该成员还没收尾,不能重试;要重来请先取消它"

    /** 重试/取消/查看会话共同前置:成员得有自己的子线程(受理失败的卡没有)。 */
    const val REASON_NO_THREAD = "该成员没有自己的会话线程"

    /** 取消:daemon 对已终局成员幂等返回原卡,客户端如实说"无需取消"。 */
    const val REASON_CANCEL_FINISHED = "该成员已收尾,无需取消"

    /** 审批入口:该成员当前没有挂起审批。 */
    const val REASON_APPROVAL_NONE = "该成员当前没有等待你确认的操作"

    /** 取消整个协同:所有成员都已终局。 */
    const val REASON_SESSION_ALL_FINISHED = "所有成员都已收尾,没有可取消的成员"

    /** 查看会话:还没有 thread_id。 */
    const val REASON_OPEN_NO_THREAD = "该成员还没有自己的会话,受理后才能查看"
}
