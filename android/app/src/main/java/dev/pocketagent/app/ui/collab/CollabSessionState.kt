package dev.pocketagent.app.ui.collab

/**
 * 协同会话仓库层的状态与意图(P1 协同闭环 Task 6)。
 *
 * 【它是什么】协同页从"派发后撒手 + 一组散落的全局 mutable state"升级为
 * "单一 [CollabSessionState] 状态树":daemon 是协同状态的唯一事实来源,
 * 本文件只是它的**客户端投影** —— 快照(Get/List)恢复整屏,事件流(Watch)
 * 增量跟随,投影永不编造 daemon 没给的事实(受理不写成完成,等待不是终局)。
 *
 * 【为什么状态在这里、配置在 CollabState】选人/绑角色是"派发前的用户意图",
 * 留在 ui.CollabState;会话一旦派发,进度/成员卡/终局全部由仓库层的
 * [CollabSessionState] 驱动 —— CollabScreen 不再读散落的 phases/threadIds。
 */

/**
 * 协同状态(七词词表,Task 7 统一口径;人话词在 CollabCopy.statusWord):
 *
 *  已提交(SUBMITTED,客户端事实:指令已发出、daemon 真值未达)→
 *  已受理(ACCEPTED)→ 执行中(RUNNING)→ 等待确认(WAITING_APPROVAL)→
 *  已完成(COMPLETED)/ 未完成(FAILED)/ 已取消(CANCELLED)。
 *
 * 【为什么保留 daemon 原文】daemon 侧状态是 string 口径(proto 注释:状态集
 * 稳定后再冻结枚举),新版本可能给出本客户端不认识的取值 —— 映射为
 * [UNKNOWN] 并把原文留在 [CollabMemberUi.statusRaw] / 展示层,如实降级,
 * 不猜、不编一个相近的已知状态。
 */
enum class CollabStatus(val raw: String) {
    /** 客户端事实:DispatchCollab 已发出,快照/受理回执还没回来。 */
    SUBMITTED(""),

    ACCEPTED("ACCEPTED"),
    RUNNING("RUNNING"),
    WAITING_APPROVAL("WAITING_APPROVAL"),
    COMPLETED("COMPLETED"),
    FAILED("FAILED"),
    CANCELLED("CANCELLED"),

    /** daemon 给了本客户端不认识的状态串(新版本新增);原文见 statusRaw。 */
    UNKNOWN("");

    /** 终局判定(与 daemon store.CollabStatusIsFinished 同口径):终局不复活。 */
    val isFinished: Boolean
        get() = this == COMPLETED || this == FAILED || this == CANCELLED
}

/** 成员卡上的一笔挂起审批(映射 proto ApprovalRequest 的最小可展示集)。 */
data class CollabApprovalUi(
    /** 真实 request_id:审批必须对应真实 thread_id/request_id(全局红线)。 */
    val requestId: String,
    /** 展示用短句(daemon 侧把工具名/理由放进来,原样透传,不加工)。 */
    val label: String,
)

/**
 * 协同成员卡的客户端投影(整卡真值,非 diff;对应 proto CollabMemberSnapshot)。
 */
data class CollabMemberUi(
    val agentId: String,
    val roleId: String,
    /** 该成员的子线程 id:取消/重试/审批/查看会话都按它定位;受理失败时为空。 */
    val threadId: String,
    val status: CollabStatus,
    /** daemon 状态原文([CollabStatus.UNKNOWN] 降级展示用;已知状态时与 raw 同)。 */
    val statusRaw: String,
    val modelId: String,
    val effort: String,
    /** daemon 侧有界的一句话进度(不是原始日志;空 = 无)。 */
    val progress: String,
    /** 成功时的终稿摘要;失败/取消时为空,不伪造。 */
    val summary: String,
    /** 失败/取消的人话原因(daemon 原文;空 = 无)。 */
    val error: String,
    val pendingApprovals: List<CollabApprovalUi>,
    /** 该成员实际工作区(隔离 worktree 时与会话级不同;空 = 未报)。 */
    val workspace: String,
)

/** 成员计数快照(汇总行的原料;"N/M 已完成 · X 个等待确认"由 CollabCopy 拼装)。 */
data class CollabSessionCounts(
    val total: Int,
    val completed: Int,
    val failed: Int,
    val cancelled: Int,
    /** 等审批(等待确认)成员数。 */
    val waiting: Int,
    /** 仍在受理/执行中的成员数。 */
    val active: Int,
)

/**
 * 协同会话状态树(计划 Task 6 接口的落地版)。
 *
 * 【needsResync 为什么存在】daemon 对慢消费者丢帧不补发,客户端凭 sequence
 * 缺口发现"中间发生了我不知道的事" —— 纯 reducer 不能发 RPC,只能把"需要全量
 * 重建"标记在状态上,由仓库层看到标记后拉 GetCollabSession 兜底(计划:
 * seq 缺口时全量拉快照重建)。这是对计划接口清单的一个**增量字段**。
 */
data class CollabSessionState(
    /** 当前协同会话 id;null = 本次没有会话(还没派发,或 daemon 没报进行中会话)。 */
    val sessionId: String? = null,
    val status: CollabStatus = CollabStatus.SUBMITTED,
    /** daemon 状态原文(UNKNOWN 降级展示用)。 */
    val statusRaw: String = "",
    /** 全体成员共用的那条指令原文。 */
    val instruction: String = "",
    /** 会话级人话原因(daemon 给的;FAILED/CANCELLED 时非空)。 */
    val sessionError: String = "",
    /** 协同工作区(未隔离时各成员共享;空 = 未报)。 */
    val workspace: String = "",
    val members: List<CollabMemberUi> = emptyList(),
    /**
     * 事件断点(已按序应用的最大 sequence;TurnSession.lastSequence 同纪律)。
     * sequence=0 的帧(旧 daemon)不推进、也不作废现有断点;0 = 没有可信断点。
     */
    val lastSequence: Long = 0L,
    /** 正在做首次恢复/重建(GetCollabSession 在飞)。 */
    val loading: Boolean = false,
    /** 传输层/操作层错误的人话呈现(断线提示、RPC 失败原因;daemon 会话级错误在 sessionError)。 */
    val error: String? = null,
    /** true = 事件流出现缺口/不认识的帧,需要拉全量快照重建(仓库层消费)。 */
    val needsResync: Boolean = false,
)

/**
 * 用户对协同会话的意图(计划 Task 6 接口,原样落地)。
 * 成员卡上的取消/重试/审批入口都以 [CollabIntent] 发出,经
 * CollabSessionRepository.send 落到对应 RPC —— UI 不直接碰 gRPC。
 */
sealed interface CollabIntent {
    /** 派发一条共同指令(成员来自 CollabState 的选人与角色绑定)。 */
    data class Dispatch(val instruction: String) : CollabIntent

    /** 取消一个成员(对应其子线程;daemon 对已终局成员幂等)。 */
    data class CancelMember(val threadId: String) : CollabIntent

    /** 重试一个成员(daemon 以新代际重建轮次,不覆盖旧结果)。 */
    data class RetryMember(val threadId: String) : CollabIntent

    /** 取消整个协同(逐成员发起取消,daemon 等状态收敛后回快照)。 */
    data object CancelSession : CollabIntent

    /**
     * 打开一笔挂起审批:导航到该成员的会话页(审批卡与 SubmitApproval 都在
     * 会话页,既有通道复用,不新增审批界面)—— 由 UI 层消费(导航),仓库层不处理。
     */
    data class OpenApproval(val threadId: String, val requestId: String) : CollabIntent
}
