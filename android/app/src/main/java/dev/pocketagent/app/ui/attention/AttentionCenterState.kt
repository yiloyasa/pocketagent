package dev.pocketagent.app.ui.attention

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import dev.pocketagent.app.ui.UnknownEventCardModel
import dev.pocketagent.app.ui.recoveryBanner
import dev.pocketagent.app.ui.relativeTimeLabel
import dev.pocketagent.app.ui.selectedAgentLabel
import dev.pocketagent.app.ui.threadAgentIdOf
import pocketagent.v1.HostRecoveryReport
import pocketagent.v1.Thread as PbThread

/**
 * Attention Center 的状态与纯函数(2026-10-04 未知事件用户处理中心计划 Task 5)。
 *
 * 【分档/筛选/优先级全是纯函数】归档判据、最高优先级、离线时间戳标注都要在单测里
 * 钉死(照 RuntimeStateLogicTest 风格);Compose 只消费结果,不在排版层写判据。
 *
 * 【OfflineStore 同款纪律】离线时中心显示本地缓存时间戳,不把旧数据伪装成实时:
 * [attentionFreshnessLabel] 在线返回 null(不摆任何"缓存"字样),离线才标注。
 */

// ---- 四档筛选(计划 Task 5:需要我处理 / 运行中 / 已完成 / 已忽略)-----------

enum class AttentionFilter(val label: String) {
    NEEDS_ME("需要我处理"),
    RUNNING("运行中"),
    DONE("已完成"),
    IGNORED("已忽略"),
}

/** 归档桶(与筛选一一对应;null = 不进中心四档,见 [attentionBucketOf])。 */
enum class AttentionBucket { NEEDS_ME, RUNNING, DONE, IGNORED }

/**
 * 中心状态(纯数据;items 由调用方从既有数据源组装,本文件不建第二数据源)。
 * [cachedAtUnixMs] 是最近一次加载成功的本地时刻,离线标注的唯一依据。
 */
data class AttentionCenterState(
    val items: List<AttentionItem> = emptyList(),
    val filter: AttentionFilter = AttentionFilter.NEEDS_ME,
    /** true = 当前不在线:items 是本地缓存快照,必须标注缓存时刻。 */
    val offline: Boolean = false,
    /** 最近一次成功加载时刻(unix ms);0 = 从未成功加载。 */
    val cachedAtUnixMs: Long = 0,
)

/**
 * 一条注意项的归档(纯函数)。
 *
 * 【未知事件的分档,逐态说清】
 *  - INSPECTED / ESCALATED / NEEDS_NATIVE_SUPPORT(与未来 daemon 新增的处置态):
 *    用户或另一台设备已处理 → **已完成**;
 *  - IGNORED(用户明确不处理)与 RETRY_LATER(用户选择稍后再看,不催办)→ **已忽略**:
 *    两者的共同语义是"用户说了暂不处理",回「需要我处理」会催办用户已拒绝的事;
 *  - NEW + requires_user → **需要我处理**(daemon 侧已判定要人介入);
 *  - NEW + !requires_user(新通知,仅供参考)→ **null,不进四档**:它不催办也不
 *    属于运行/完成,硬塞任何一档都是错误信号 —— 完整清单仍在「电脑端新消息」弹窗。
 */
fun attentionBucketOf(item: AttentionItem): AttentionBucket? = when (item) {
    is AttentionItem.UnknownEvent -> when {
        item.processingStatus == "IGNORED" || item.processingStatus == "RETRY_LATER" -> AttentionBucket.IGNORED
        item.processingStatus != "NEW" -> AttentionBucket.DONE
        item.requiresUser -> AttentionBucket.NEEDS_ME
        else -> null
    }
    is AttentionItem.Approval -> AttentionBucket.NEEDS_ME
    is AttentionItem.Recovery -> AttentionBucket.NEEDS_ME
    is AttentionItem.Collab -> if (item.needsAction) AttentionBucket.NEEDS_ME else AttentionBucket.RUNNING
}

/**
 * 按当前筛选取条目。排序:同档内按优先级升序(审批等人 > 恢复异常 > 未知事件)、
 * 同优先级按时间倒序(刚出事的排最前;无时间源的条目自然沉底)。
 */
fun filterAttention(state: AttentionCenterState): List<AttentionItem> {
    val bucket = when (state.filter) {
        AttentionFilter.NEEDS_ME -> AttentionBucket.NEEDS_ME
        AttentionFilter.RUNNING -> AttentionBucket.RUNNING
        AttentionFilter.DONE -> AttentionBucket.DONE
        AttentionFilter.IGNORED -> AttentionBucket.IGNORED
    }
    return state.items
        .filter { attentionBucketOf(it) == bucket }
        .sortedWith(
            compareBy<AttentionItem> { attentionPriority(it) }
                .thenByDescending { attentionTimeOf(it) }
        )
}

private fun attentionTimeOf(item: AttentionItem): Long = when (item) {
    is AttentionItem.UnknownEvent -> item.receivedAtUnixMs
    else -> 0L
}

/**
 * 最高优先级摘要(首页一行的数据源;null = 没有可摘要的"活"事项)。
 * 只在 NEEDS_ME / RUNNING 里挑 —— 已完成/已忽略是档案,永远不该当"最高优先级"。
 */
fun topAttentionSummary(items: List<AttentionItem>): AttentionItem? =
    items.filter { attentionBucketOf(it).let { b -> b == AttentionBucket.NEEDS_ME || b == AttentionBucket.RUNNING } }
        .minByOrNull { attentionPriority(it) }

/**
 * 优先级(数值越小越急):会话停摆等人 > 电脑端启动异常 > 未知事件等你 >
 * 运行中事项。已完成/已忽略不参与排序(见 [topAttentionSummary])。
 */
fun attentionPriority(item: AttentionItem): Int = when (attentionBucketOf(item)) {
    AttentionBucket.NEEDS_ME -> when (item) {
        is AttentionItem.Approval -> 0
        is AttentionItem.Recovery -> 1
        else -> 2
    }
    AttentionBucket.RUNNING -> 3
    AttentionBucket.DONE -> 4
    AttentionBucket.IGNORED -> 5
    null -> 6
}

/** 「需要我处理」计数(首页/抽屉角标的数据源)。 */
fun attentionNeedsMeCount(items: List<AttentionItem>): Int =
    items.count { attentionBucketOf(it) == AttentionBucket.NEEDS_ME }

/**
 * 离线时间戳标注(OfflineStore 纪律:不伪装实时)。
 * 在线 → null(实时,不摆字);离线有加载时刻 → 「本地缓存 · N 分钟前」;
 * 离线从未加载成功 → 「本地缓存」(有"这是本地的"这个事实就够,不编时刻)。
 */
fun attentionFreshnessLabel(offline: Boolean, cachedAtUnixMs: Long, nowMs: Long): String? {
    if (!offline) return null
    val rel = relativeTimeLabel(cachedAtUnixMs, nowMs)
    return if (rel.isBlank()) "本地缓存" else "本地缓存 · $rel"
}

// ---- 组装(从既有数据源到注意项;纯映射,不发 RPC 不造值)----------------------

/** Task 4 的卡片模型 → 注意项(逐字段搬运,detail 用 impact 人话)。 */
fun unknownEventAttentionItems(cards: List<UnknownEventCardModel>): List<AttentionItem.UnknownEvent> =
    cards.map { c ->
        AttentionItem.UnknownEvent(
            id = c.eventId,
            title = c.title,
            detail = c.impact,
            threadId = c.threadId,
            requiresUser = c.requiresUser,
            processingStatus = c.status,
            receivedAtUnixMs = c.receivedAtUnixMs,
        )
    }

/** 首页既有三分的待审批组 → 注意项(同源同口径,不重算归组)。 */
fun approvalAttentionItems(pendingApproval: List<PbThread>): List<AttentionItem.Approval> =
    pendingApproval.map { t ->
        AttentionItem.Approval(
            id = t.id,
            title = t.title.ifEmpty { "(未命名)" },
            detail = "${selectedAgentLabel(threadAgentIdOf(t.agentId))} · " +
                "${t.pendingApprovalsCount} 项等待你处理",
            actionLabel = "去处理",
            pendingCount = t.pendingApprovalsCount,
        )
    }

/**
 * 启动恢复报告 → 注意项;**null = 无事可报**(报告缺席,或 status=="ok" 一切正常)。
 * 归组/降级判据复用既有 [recoveryBanner](同一份 daemon 真值,不做第二份推导)。
 */
fun recoveryAttentionItem(report: HostRecoveryReport?): AttentionItem.Recovery? {
    val banner = recoveryBanner(report) ?: return null
    if (!banner.degraded) return null
    return AttentionItem.Recovery(
        id = "recovery",
        title = banner.headline,
        detail = if (banner.lostLines.isEmpty()) {
            "启动状态:${banner.status}"
        } else {
            "上次退出有 ${banner.lostLines.size} 句输入没跑到,涉及 ${banner.threadCount} 个会话"
        },
        actionLabel = "去查看",
    )
}

/**
 * 中心的会话级 UI 状态:筛选档记住上次选择(重进中心不回弹到第一档)。
 * 【与 [AttentionCenterState] 的命名区分】数据类是纯状态快照(单测用);本对象只
 * 持久化"用户当前停在哪一档"这一项 UI 偏好。条目清单不在此持有 —— 由首页从既有
 * 数据源组装后传入,避免第二份事实。
 */
object AttentionCenterUiState {
    var filter by mutableStateOf(AttentionFilter.NEEDS_ME)
}
