package dev.pocketagent.app.ui.prompt

import dev.pocketagent.app.ui.PromptOptimizeStyle

/**
 * 优化预览状态(P4 Task 1 落类型,Task 2 落 UI/reducer):原文与候选文本
 * **始终分离**的两个槽位 —— 优化失败、超时、空返回或取消都只写 error/loading,
 * 绝不触碰原文槽位;候选要进输入框必须经用户确认(Task 2 的 Apply 意图)。
 *
 * 【防覆盖契约】[sourceRevision] 是发起请求那一刻输入框的修订号;请求返回时
 * 预览层用 [inputModifiedSince] 对比当前修订号,已前进即说明用户在等待期间改过
 * 输入,必须让用户选择"覆盖/保留新文本",不得静默替换。
 *
 * @param original 发起优化时的输入框原文(撤销只恢复到它)
 * @param candidate daemon 返回的改写文本(已 trim;null = 没有可确认的候选)
 * @param style 本次请求选择的风格(展示口径:让用户知道按哪种风格改的)
 * @loading 是否有请求在飞(进行中禁用再次发起,防并发替换)
 * @param error 最近一次失败的人话文案(可清空重试;null = 无错误)
 * @param sourceRevision 发起请求时输入框的修订号(防覆盖,见上)
 */
internal data class PromptOptimizePreviewState(
    val original: String = "",
    val candidate: String? = null,
    val style: PromptOptimizeStyle = PromptOptimizeStyle.DEFAULT,
    val loading: Boolean = false,
    val error: String? = null,
    val sourceRevision: Long = 0L,
) {
    /** 是否有可确认的候选文本(空串/纯空白不算 —— 双保险,仓库层已拒收空串)。 */
    val hasCandidate: Boolean
        get() = !candidate.isNullOrBlank()

    /**
     * 请求期间用户是否改过输入框(currentRevision = 当前输入框修订号)。
     * true → 应用候选前必须让用户二选一(覆盖 / 保留新文本),不静默替换。
     */
    fun inputModifiedSince(currentRevision: Long): Boolean = currentRevision != sourceRevision
}
