package dev.pocketagent.app.ui

import dev.pocketagent.app.ui.theme.EditorialTone

/**
 * 审批弹窗状态机(README §审批弹窗状态机)的纯逻辑层 —— 不接 RPC、无 Android
 * 依赖,JVM 单测钉死(EditorialApprovalLogicTest);Compose 皮在
 * component/PaApprovalDialog,第二波把 daemon 的 ApprovalRequest 接到
 * [ApprovalRequestUi] 上即可。
 *
 * 【状态流】收到请求(RECEIVED)→ 待决定(PENDING)→ 提交中(SUBMITTING)→
 * 服务端确认(CONFIRMED)→ 已允许(ALLOWED)/已拒绝(DENIED)。
 * 另有 UNAVAILABLE:请求失效/重复/跨会话/超时/提交失败 —— 决定按钮禁用,
 * 展示原因与重试;**绝不显示已批准,不做乐观通过**。
 *
 * 【dismiss ≠ 拒绝】本层没有"关闭=拒绝"这条边:关闭弹窗是宿主的 onDismiss
 * 回调(暂不处理,请求留在待处理区),与 onDecide(false) 完全两条通道。
 */
enum class ApprovalPhase {
    /** 收到请求:事件已落地,展示层短暂经过,即可决定。 */
    RECEIVED,

    /** 待决定:按钮可用。 */
    PENDING,

    /** 提交中:禁重复点击,但仍可查看全部上下文。 */
    SUBMITTING,

    /** 服务端已确认:终态词(已允许/已拒绝)落地前的一拍。 */
    CONFIRMED,

    /** 已允许:服务端确认后唯一可以"移除待处理"的依据。 */
    ALLOWED,

    /** 已拒绝。 */
    DENIED,

    /** 暂无法处理:失效/重复/超时/提交失败,展示原因与重试。 */
    UNAVAILABLE,
}

/**
 * 审批请求的 UI 值对象(第二波由 daemon 事件装配)。
 *
 * 【只放必要信息】README 弹窗白名单:Agent 与会话、操作目的、工作区、操作类型、
 * 原始命令 —— 不放风险评分、不放 agent 品牌断言([agentName] 由调用方填,
 * 未知给中性占位词,组件只留图标槽)。
 *
 * @param submittedChoice SUBMITTING 中用户点的那颗按钮(true=允许/false=拒绝),
 *   用于在按钮上回显"提交中…"而非两颗都显示;提交失败进 UNAVAILABLE 时保留,
 *   供"上一次尝试没有成功"的措辞与重试。
 * @param unavailableReason UNAVAILABLE 时的人话原因(失效/重复/超时/提交失败),
 *   由调用方给具体文案,组件只负责展示。
 * @param queuePosition / queueTotal 多请求按到达顺序的数量(1 起始)。
 */
data class ApprovalRequestUi(
    val requestId: String,
    val threadId: String,
    val agentName: String,
    val sessionTitle: String,
    val purpose: String,
    val workspace: String,
    val actionKind: String,
    val command: String,
    val phase: ApprovalPhase = ApprovalPhase.PENDING,
    val submittedChoice: Boolean? = null,
    val unavailableReason: String? = null,
    val queuePosition: Int = 1,
    val queueTotal: Int = 1,
)

/** 决定按钮是否可用:只有收到请求/待决定两态可点(提交中禁重复,终态/失效禁用)。 */
val ApprovalRequestUi.decideEnabled: Boolean
    get() = phase == ApprovalPhase.RECEIVED || phase == ApprovalPhase.PENDING

/** 重试是否可用:只有 UNAVAILABLE 有"重试"可讲(其余态没有失败可重试)。 */
val ApprovalRequestUi.retryEnabled: Boolean
    get() = phase == ApprovalPhase.UNAVAILABLE

/**
 * 进入提交中:记录用户点的按钮用于单按钮回显;非可决定态调用是**无操作**
 * (纯函数不抛异常,防御调用方双击竞态 —— 第二次点击落在 SUBMITTING 时不改状态)。
 */
fun ApprovalRequestUi.submitting(allow: Boolean): ApprovalRequestUi =
    if (decideEnabled) copy(phase = ApprovalPhase.SUBMITTING, submittedChoice = allow) else this

/** 服务端确认(带结果):仅提交中可进;允许/拒绝从此以服务端确认为唯一依据。 */
fun ApprovalRequestUi.serverConfirmed(allow: Boolean): ApprovalRequestUi =
    if (phase == ApprovalPhase.SUBMITTING) {
        copy(phase = ApprovalPhase.CONFIRMED, submittedChoice = allow)
    } else this

/** 确认落地为终态词:CONFIRMED → 已允许/已拒绝;其余态无操作。 */
fun ApprovalRequestUi.settled(): ApprovalRequestUi =
    if (phase == ApprovalPhase.CONFIRMED) {
        copy(phase = if (submittedChoice == true) ApprovalPhase.ALLOWED else ApprovalPhase.DENIED)
    } else this

/**
 * 快路径:提交中直接拿到服务端结果(跳过 CONFIRMED 一拍)。
 * 仅提交中可进 —— PENDING/UNAVAILABLE 上调用是无操作,杜绝对未提交请求的乐观通过。
 */
fun ApprovalRequestUi.confirmed(allow: Boolean): ApprovalRequestUi =
    if (phase == ApprovalPhase.SUBMITTING) {
        copy(phase = if (allow) ApprovalPhase.ALLOWED else ApprovalPhase.DENIED)
    } else this

/**
 * 标记暂无法处理(请求失效/重复/跨会话/超时/提交失败)。可决定态与提交中都可进;
 * 提交中进入时保留 [ApprovalRequestUi.submittedChoice] —— 界面要讲清
 * "上一次尝试没有成功",而不是装作没点过。**任何路径都不会产生 ALLOWED**。
 */
fun ApprovalRequestUi.markUnavailable(reason: String): ApprovalRequestUi =
    if (phase in arrayOf(
            ApprovalPhase.RECEIVED, ApprovalPhase.PENDING,
            ApprovalPhase.SUBMITTING, ApprovalPhase.UNAVAILABLE,
        )
    ) {
        copy(phase = ApprovalPhase.UNAVAILABLE, unavailableReason = reason)
    } else this

/** 重试:UNAVAILABLE → 回到待决定重新校验来源状态;其余态无操作。 */
fun ApprovalRequestUi.retry(): ApprovalRequestUi =
    if (phase == ApprovalPhase.UNAVAILABLE) {
        copy(phase = ApprovalPhase.PENDING, unavailableReason = null)
    } else this

/** 阶段的状态词(组件状态徽章用;措辞对齐四词纪律,不用「错误」「失败」字样)。 */
val ApprovalRequestUi.phaseWord: String
    get() = when (phase) {
        ApprovalPhase.RECEIVED -> "收到审批请求"
        ApprovalPhase.PENDING -> "等待你处理"
        ApprovalPhase.SUBMITTING -> "提交中…"
        ApprovalPhase.CONFIRMED -> "服务端已确认"
        ApprovalPhase.ALLOWED -> "已允许"
        ApprovalPhase.DENIED -> "已拒绝"
        ApprovalPhase.UNAVAILABLE -> "暂无法处理"
    }

/** 阶段的语义色档:待决定=琥珀;提交中/已允许=绿;已拒绝=珊瑚红;暂无法处理=琥珀(可重试的阻断)。 */
val ApprovalRequestUi.phaseTone: EditorialTone
    get() = when (phase) {
        ApprovalPhase.RECEIVED, ApprovalPhase.PENDING -> EditorialTone.ATTENTION
        ApprovalPhase.SUBMITTING, ApprovalPhase.CONFIRMED, ApprovalPhase.ALLOWED -> EditorialTone.RUNNING
        ApprovalPhase.DENIED -> EditorialTone.DENIED
        ApprovalPhase.UNAVAILABLE -> EditorialTone.ATTENTION
    }

/**
 * 多请求到达顺序的数量词:单请求返回空串(不摆"共 1 项"的空壳);
 * [position] 越界按边界收敛(0 → 第 1 项,超总数 → 第 total 项)。
 */
fun approvalQueueLabel(position: Int, total: Int): String {
    if (total <= 1) return ""
    val pos = position.coerceIn(1, total)
    return "共 $total 项待审批 · 当前第 $pos 项"
}
