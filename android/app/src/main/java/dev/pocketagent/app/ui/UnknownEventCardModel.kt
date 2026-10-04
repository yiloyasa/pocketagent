package dev.pocketagent.app.ui

import pocketagent.v1.UnknownEventActionRequest
import pocketagent.v1.UnknownEventSummary
import pocketagent.v1.UserActionDescriptor

/**
 * 未知事件用户处理中心(2026-10-04 计划 Task 4)的卡片模型层:**纯函数,不碰 Compose**。
 *
 * 【它解决什么】旧「消息透析」弹窗直接拿 proto 类型渲染;新「电脑端新消息」入口要的
 * 是 daemon 的人话字段(UnknownEventSummary 12~17)、旧 daemon 降级口径、动作按钮的
 * 路由分类与分组,这些判据单测跑不了 Compose —— 全部抽到这里钉住。
 *
 * 【它不做什么】不发明 daemon 语义:人话文案一律 daemon 给(客户端不翻译不改写);
 * daemon 没给的(旧版本)按 [UnknownEventCopy 的降级说明] 如实陈述,不编造
 * (AUDIT §3.3 / 规格评审焦点「旧 daemon 没有新字段时,客户端仍能显示方法名和明确的降级说明」)。
 */

// ---- 动作 ID 与 daemon 白名单同键(全小写下划线,客户端原样回传,不翻译) ----
// daemon 侧封口在 server/unknown_actions.go;这里只是客户端路由用的常量镜像。
const val UNKNOWN_ACTION_COPY = "copy"
const val UNKNOWN_ACTION_INSPECT = "inspect"
const val UNKNOWN_ACTION_ACKNOWLEDGE = "acknowledge"
const val UNKNOWN_ACTION_RETRY_LATER = "retry_later"
const val UNKNOWN_ACTION_OPEN_NATIVE = "open_native"

/**
 * 客户端对一条动作的路由(纯函数):
 *  - [UnknownActionRoute.LOCAL_COPY]:copy 是**手机本地行为**(剪贴板在手机上),
 *    绝不发给 daemon(daemon 执行器同样拒绝,客户端先挡一道省一次往返);
 *  - [UnknownActionRoute.EXPAND_DETAIL]:inspect = 在本端展开技术详情(详情内容走
 *    既有 GetUnknownEvent,daemon 侧 inspect 也是复用同一份能力,不重复发动作);
 *  - [UnknownActionRoute.REMOTE]:其余动作(acknowledge/retry_later/open_native 以及
 *    未来 daemon 新增的白名单动作)一律带 event_id + expected_revision 原样回传,
 *    白名单校验归 daemon —— 客户端不维护第二份白名单,免得两边漂移。
 */
fun unknownActionRoute(actionId: String): UnknownActionRoute = when (actionId) {
    UNKNOWN_ACTION_COPY -> UnknownActionRoute.LOCAL_COPY
    UNKNOWN_ACTION_INSPECT -> UnknownActionRoute.EXPAND_DETAIL
    else -> UnknownActionRoute.REMOTE
}

enum class UnknownActionRoute {
    /** 手机本地剪贴板动作,不发给 daemon。 */
    LOCAL_COPY,
    /** 本端展开技术详情(内容经既有 GetUnknownEvent,不发动作 RPC)。 */
    EXPAND_DETAIL,
    /** 发往 daemon 执行(带 expected_revision 做 CAS;白名单由 daemon 裁决)。 */
    REMOTE,
}

/** 一条动作按钮(daemon UserActionDescriptor 一比一搬运,字段不改写不过滤)。 */
data class UnknownEventActionModel(
    val actionId: String,
    /** 按钮文案:daemon 给人话,客户端照抄不翻译。 */
    val label: String,
    /** 一句话说明按下之后会发生什么;空 = 无。 */
    val description: String,
    /** 只有 safe=true 才渲染成可点按钮(红线:客户端不自行发明"允许"类动作)。 */
    val safe: Boolean,
    val requiresConfirmation: Boolean,
    val destructive: Boolean,
    val route: UnknownActionRoute,
)

fun unknownEventActionModel(d: UserActionDescriptor): UnknownEventActionModel = UnknownEventActionModel(
    actionId = d.actionId,
    label = d.label,
    description = d.description,
    safe = d.safe,
    requiresConfirmation = d.requiresConfirmation,
    destructive = d.destructive,
    route = unknownActionRoute(d.actionId),
)

/**
 * 一条未知事件的卡片模型(列表行与详情共用)。
 *
 * 【降级字段怎么读】[degraded]=true 表示 daemon 没给人话字段(旧版本 daemon 或旧
 * 台账条目):此时 [title] 是方法名原文(不猜)、[explanation]/[impact] 为空串、
 * [actions] 为空(旧 daemon 没有动作表,客户端绝不造按钮)、[technicalAvailable]
 * 为 false —— 界面据此显示固定降级说明,不占位编内容。
 */
data class UnknownEventCardModel(
    val eventId: String,
    /** 一行标题:daemon 的 plain_title;降级时 = 方法名原文 → 归类文案 → eventId。 */
    val title: String,
    /** daemon 的两三句话人话解释;降级/未给 = 空串(UI 整块缺席)。 */
    val explanation: String,
    /** daemon 给的「对当前任务的影响」;降级/未给 = 空串。 */
    val impact: String,
    val requiresUser: Boolean,
    val actions: List<UnknownEventActionModel>,
    /** true = daemon 提供了用户处理中心的人话字段(12~17);false = 旧 daemon,降级口径。 */
    val technicalAvailable: Boolean,
    /** 处置状态原文(NEW/INSPECTED/…),人话翻译由 [unknownStatusLabel] 在渲染层做。 */
    val status: String,
    val sourceAgent: String,
    val sourceMethod: String,
    val receivedAtUnixMs: Long,
    /** 当前 revision:动作请求的 expected_revision 唯一来源(CAS 纪律)。 */
    val revision: Long,
    /** 关联会话;空 = 无归属(只进全局中心,不虚构跳转目标)。 */
    val threadId: String,
    /** true = daemon 未提供人话字段,按降级口径呈现。 */
    val degraded: Boolean,
)

/**
 * UnknownEventSummary → 卡片模型(唯一映射点)。
 *
 * 【降级判据 = plain_title 是否缺席】新 daemon 对三个 UNKNOWN_* kind 都会给
 * plain_title(daemon 侧固定文案表);旧 daemon / 旧条目读出来是空串。拿一个字段当
 * 12~17 是否齐备的探针,与 daemon 的"整组一起写"实现对齐,不做逐字段猜测。
 */
fun unknownEventCardModel(s: UnknownEventSummary): UnknownEventCardModel {
    val plainTitle = s.plainTitle
    val degraded = plainTitle.isBlank()
    val kindLabel = unknownKindLabel(s.normalizedKind)
    return UnknownEventCardModel(
        eventId = s.eventId,
        title = if (!degraded) {
            plainTitle
        } else {
            // 降级链:方法名原文 → 归类人话 → eventId;空白标题会让列表看起来像坏了。
            s.sourceMethod.ifBlank { kindLabel }.ifBlank { s.eventId }
        },
        explanation = if (degraded) "" else s.plainExplanation,
        impact = if (degraded) "" else s.impact,
        // 降级时不催办:字段都读不全的事件,不该把用户拉进"需要处理"的队列。
        requiresUser = s.requiresUser && !degraded,
        actions = if (degraded) emptyList() else s.actionsList.map { unknownEventActionModel(it) },
        technicalAvailable = !degraded,
        status = s.processingStatus,
        sourceAgent = s.sourceAgent,
        sourceMethod = s.sourceMethod,
        receivedAtUnixMs = s.receivedAtUnixMs,
        revision = s.revision,
        threadId = s.threadId,
        degraded = degraded,
    )
}

// ---- 列表分组(Task 4:需要你处理 / 已查看 / 仅供参考)--------------------------

enum class UnknownEventGroup { NEEDS_USER, HANDLED, REFERENCE }

/**
 * 一条事件的分组(纯函数)。
 *
 * 【处置态优先于 requires_user】事件一旦有了处置状态(INSPECTED/IGNORED/RETRY_LATER/
 * ESCALATED/NEEDS_NATIVE_SUPPORT 与未来 daemon 新增的处置态),说明用户(或另一台设备)
 * 已经处理过它 —— 无论 requires_user 是什么都归「已查看」,不能让旧请求一直催办。
 * 【未来新状态不催办】daemon 出现我们没见过的处置态时按「已查看」处理:把它误报成
 * 待办会催办根本不存在的问题;漏显示在「已查看」组里代价小得多(失败安全方向)。
 */
fun unknownEventGroupOf(status: String, requiresUser: Boolean): UnknownEventGroup = when {
    status != "NEW" -> UnknownEventGroup.HANDLED
    requiresUser -> UnknownEventGroup.NEEDS_USER
    else -> UnknownEventGroup.REFERENCE
}

/** 三组清单(组内按收到时间倒序:刚来的排最前)。 */
data class UnknownEventGroups(
    val needsUser: List<UnknownEventCardModel>,
    val handled: List<UnknownEventCardModel>,
    val reference: List<UnknownEventCardModel>,
)

fun partitionUnknownEvents(cards: List<UnknownEventCardModel>): UnknownEventGroups {
    val byRecency = compareByDescending<UnknownEventCardModel> { it.receivedAtUnixMs }
    val grouped = cards.groupBy { unknownEventGroupOf(it.status, it.requiresUser) }
    return UnknownEventGroups(
        needsUser = grouped[UnknownEventGroup.NEEDS_USER].orEmpty().sortedWith(byRecency),
        handled = grouped[UnknownEventGroup.HANDLED].orEmpty().sortedWith(byRecency),
        reference = grouped[UnknownEventGroup.REFERENCE].orEmpty().sortedWith(byRecency),
    )
}

// ---- 动作执行请求组装(CAS 纪律的落点)------------------------------------------

/**
 * 组一个动作执行请求;**copy 一律拒绝**(返回 null)。
 *
 * 【为什么在组装层就封死】copy 是手机本地剪贴板动作,发往电脑端没有意义;daemon
 * 执行器对它也会回 InvalidArgument —— 客户端在组装层先挡一道,省一次注定失败的
 * 往返,也让"copy 绝不发给 daemon"这条红线有单测钉着。
 *
 * 【CAS 必带】expected_revision = 客户端最后一次看到的 UnknownEventSummary.revision;
 * 冲突不是错误,是结构化结果(ok=false + conflict=true),调用方据此重拉详情刷新。
 */
fun unknownActionRequest(
    eventId: String,
    expectedRevision: Long,
    action: UnknownEventActionModel,
    note: String,
): UnknownEventActionRequest? {
    if (action.route == UnknownActionRoute.LOCAL_COPY) return null
    return UnknownEventActionRequest.newBuilder()
        .setEventId(eventId)
        .setExpectedRevision(expectedRevision)
        .setActionId(action.actionId)
        .setNote(note)
        .build()
}
