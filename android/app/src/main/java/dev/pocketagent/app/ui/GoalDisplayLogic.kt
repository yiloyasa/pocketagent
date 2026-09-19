package dev.pocketagent.app.ui

import pocketagent.v1.Thread

/**
 * 会话目标展示的纯函数(daemon 的 Thread.goal_detail / Thread.goal → 聊天页顶栏文案)。
 *
 * 【两个字段的分工,与 proto 注释一致】Thread.goal 是 daemon 本库落库的目标
 * (手机设置后写的值);Thread.goal_detail 是打开会话时从电脑端 codex 现读的
 * 目标态,带状态机 —— 重连/换端后以它为准。这里定死取值优先级:
 * goal_detail.objective 非空 > goal;两者都空 → 不占位(返回 null)。
 *
 * 纯函数无 Composable 依赖,JVM 单测可直接钉住优先级与状态机文案。
 */
object GoalDisplayLogic {

    /** 目标文本:电脑端现读值优先,本地落库值兜底;都空返回空串。 */
    fun objectiveOf(thread: Thread?): String {
        thread ?: return ""
        val fromDesktop = thread.takeIf { it.hasGoalDetail() }?.goalDetail?.objective.orEmpty()
        return (fromDesktop.ifBlank { thread.goal }).trim()
    }

    /** codex 目标状态机原始值 → 中文;空/未知返回 null(不追加后缀,原样不误导)。 */
    fun statusLabel(status: String): String? = when (status) {
        "active" -> "进行中"
        "paused" -> "已暂停"
        "blocked" -> "受阻"
        "usageLimited" -> "额度受限"
        "budgetLimited" -> "预算受限"
        "complete" -> "已完成"
        else -> null
    }

    /** 顶栏整行文案"目标:xxx";无目标返回 null(不占位)。已知状态时追加" · 状态"。 */
    fun goalLine(thread: Thread?): String? {
        val objective = objectiveOf(thread)
        if (objective.isBlank()) return null
        val status = thread
            ?.takeIf { it.hasGoalDetail() }
            ?.goalDetail?.status
            ?.let { statusLabel(it) }
        return if (status != null) "目标:$objective · $status" else "目标:$objective"
    }
}
