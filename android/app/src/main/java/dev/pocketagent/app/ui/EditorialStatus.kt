package dev.pocketagent.app.ui

import dev.pocketagent.app.ui.component.TaskCenter
import dev.pocketagent.app.ui.component.lastTurnErrorSummary
import dev.pocketagent.app.ui.theme.EditorialTone
import pocketagent.v1.OutputState
import pocketagent.v1.ThreadStatus

/**
 * 四词状态口径(README §状态)的枚举:等待你处理 / 正在执行 / 已完成 / 未完成。
 *
 * 【为什么收口到这里】状态词从前散在各页(「待你处理」「运行中」「中断」各说各话),
 * 编辑式升级要求**全 app 只有这四个词 + 一一对应的语义色档**,且语义绝不单靠颜色。
 * 第二波各页(首页/会话/任务中心/协同)一律经 [EditorialStatusView] 取词取色,
 * 同一事实在不同页面长出同一种状态。
 *
 * 【与旧徽章的关系】component/ThreadStatusBadge 的「待你处理/运行中/中断」三态
 * 仍然独立存在、行为不变(本轮不迁移页面);本枚举是升级后的**目标口径**,
 * 第二波迁移时替换过去。优先级判定与其同构:待审批 > 运行 > 失败。
 */
enum class EditorialStatus(val word: String, val tone: EditorialTone) {
    /** 琥珀:轮次/任务等的是用户(审批、输入),不是系统在跑。 */
    WAITING_FOR_YOU("等待你处理", EditorialTone.ATTENTION),

    /** 绿:daemon/Agent 正在执行。主机离线时绝不落到这一档(见 PaHostChip)。 */
    RUNNING("正在执行", EditorialTone.RUNNING),

    /** 中性墨:终局且成功(含"确实没有正文"的事实陈述,OutputState.EMPTY)。 */
    COMPLETED("已完成", EditorialTone.NEUTRAL),

    /** 珊瑚红:终局但没成(失败/取消/中断/适配器丢正文)。措辞禁「失败」「错误」。 */
    NOT_COMPLETED("未完成", EditorialTone.DENIED),
}

/** 四词状态的展示值:[label] 默认即四词本身,[detail] 是可选的时间/来源/原因后缀。 */
data class EditorialStatusView(
    val status: EditorialStatus,
    val label: String = status.word,
    val detail: String? = null,
)

/**
 * 四词状态的**形通道**(色 + 词之外的第三条语义通道,色弱也可辨)。
 *
 * 【为什么要有形】红绿琥珀对红绿色弱是同一团暖色,只靠色档分不开四态 ——
 * 每态一个互不相同的形状:空心环(等用户来填)/ 实心点(进行中)/ 对勾(收尾)/
 * 叹号(要回头看)。词已经保证了可辨,形状是让"扫一眼"也分得清的第二保险。
 */
enum class PaStatusGlyph { RING, DOT, CHECK, EXCLAMATION }

/** 四词状态 → 形通道映射(纯函数,单测钉死 StatusGlyphMappingTest)。 */
fun statusGlyphOf(status: EditorialStatus): PaStatusGlyph = when (status) {
    EditorialStatus.WAITING_FOR_YOU -> PaStatusGlyph.RING
    EditorialStatus.RUNNING -> PaStatusGlyph.DOT
    EditorialStatus.COMPLETED -> PaStatusGlyph.CHECK
    EditorialStatus.NOT_COMPLETED -> PaStatusGlyph.EXCLAMATION
}

// ---------------------------------------------------------------- 会话(ThreadStatus)

/**
 * 会话四词映射(总函数):挂审批 > 运行中 > 失败(未完成)> 干净收尾(已完成)。
 *
 * 与 component/ThreadStatusBadge.threadStatusBadge 同一套优先级(审计 P1-1 单一
 * 事实源精神):待审批的会话等的是用户,即使状态滞后还在 RUNNING 也必须显示
 * 「等待你处理」。IDLE+lastTurnError 的失败判定与 TurnSession 恢复路径一致。
 * UNSPECIFIED(daemon 未上报)= 从未记录到完成 → 保守归「未完成」。
 */
fun editorialStatusOf(
    status: ThreadStatus,
    pendingApprovals: Int,
    lastTurnError: String,
): EditorialStatus = when {
    pendingApprovals > 0 || status == ThreadStatus.THREAD_STATUS_WAITING_APPROVAL ->
        EditorialStatus.WAITING_FOR_YOU
    status == ThreadStatus.THREAD_STATUS_RUNNING -> EditorialStatus.RUNNING
    status == ThreadStatus.THREAD_STATUS_IDLE && lastTurnError.isNotBlank() ->
        EditorialStatus.NOT_COMPLETED
    status == ThreadStatus.THREAD_STATUS_UNSPECIFIED -> EditorialStatus.NOT_COMPLETED
    else -> EditorialStatus.COMPLETED
}

/**
 * 会话四词映射(可空):UNSPECIFIED → null,沿用「无事实不摆空壳徽章」的既有纪律
 * (旧 daemon 未上报状态的会话不渲染状态徽章,而不是编一个状态词)。
 */
fun editorialStatusOrNull(
    status: ThreadStatus,
    pendingApprovals: Int,
    lastTurnError: String,
): EditorialStatus? = when {
    status == ThreadStatus.THREAD_STATUS_UNSPECIFIED &&
        pendingApprovals <= 0 &&
        lastTurnError.isBlank() -> null
    else -> editorialStatusOf(status, pendingApprovals, lastTurnError)
}

/**
 * 会话四词展示值:未完成时附带最后错误的首行人话摘要(40 字截断,复用
 * lastTurnErrorSummary 单一事实源),供第二波列表行/卡片决定是否展示。
 */
fun editorialThreadStatusView(
    status: ThreadStatus,
    pendingApprovals: Int,
    lastTurnError: String,
): EditorialStatusView {
    // ofOrNull 只对「UNSPECIFIED 且无审批无错误」返回 null;展示值走总函数口径归「未完成」
    val s = editorialStatusOrNull(status, pendingApprovals, lastTurnError)
        ?: editorialStatusOf(status, pendingApprovals, lastTurnError)
    val detail = if (s == EditorialStatus.NOT_COMPLETED && lastTurnError.isNotBlank()) {
        lastTurnErrorSummary(lastTurnError)
    } else null
    return EditorialStatusView(s, detail = detail)
}

// ---------------------------------------------------------------- 任务中心(TaskCenter.Status)

/**
 * 任务中心四词映射:RUNNING → 正在执行;DONE → 已完成;
 * FAILED / CANCELED / INTERRUPTED → 未完成(取消也是"没做完",不是"失败")。
 */
fun editorialStatusOf(status: TaskCenter.Status): EditorialStatus = when (status) {
    TaskCenter.Status.RUNNING -> EditorialStatus.RUNNING
    TaskCenter.Status.DONE -> EditorialStatus.COMPLETED
    TaskCenter.Status.FAILED,
    TaskCenter.Status.CANCELED,
    TaskCenter.Status.INTERRUPTED,
    -> EditorialStatus.NOT_COMPLETED
}

// ---------------------------------------------------------------- 轮次终态(OutputState)

/**
 * 轮次终态四词映射(可空):TEXT/EMPTY/ARTIFACT_ONLY/RECOVERED_* 都是"这一轮
 * 正常收尾" → 已完成(EMPTY 是事实陈述,不伪装成有正文);ERROR / ADAPTER_LOST_OUTPUT
 * → 未完成;UNSPECIFIED(旧 daemon)→ null(不编状态)。
 */
fun editorialStatusOrNull(state: OutputState): EditorialStatus? = when (state) {
    OutputState.OUTPUT_STATE_TEXT,
    OutputState.OUTPUT_STATE_EMPTY,
    OutputState.OUTPUT_STATE_ARTIFACT_ONLY,
    OutputState.OUTPUT_STATE_RECOVERED_FROM_NATIVE_HISTORY,
    -> EditorialStatus.COMPLETED
    OutputState.OUTPUT_STATE_ERROR,
    OutputState.OUTPUT_STATE_ADAPTER_LOST_OUTPUT,
    -> EditorialStatus.NOT_COMPLETED
    else -> null
}
