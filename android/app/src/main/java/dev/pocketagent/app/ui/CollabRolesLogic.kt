package dev.pocketagent.app.ui

import pocketagent.v1.CollabRole

/**
 * 协同角色(Collab Role)的纯函数层:id 校验 / 显示名回落 / 摘要行拼接。
 *
 * 【为什么独立成层】这些规则最容易被"顺手改个条件"悄悄反转,而反转后没有任何报错,
 * 只能等用户派发失败、看到"未定义的协同角色 xx"才暴露([CollabRolesState] 的删除也同理)。
 * 集中到这里用单测钉死,改坏会在 `test` 阶段直接红。
 *
 * 【校验口径与 daemon 同源但职责不同】这里做的是**客户端前置校验**:让保存按钮在 id 非法时
 * 直接禁用并给一句人话,用户不必点了保存、跨进程 round-trip 一圈才被告知。daemon 端若仍
 * 返回 InvalidArgument(例如并发边界下的 trim 差异),其原文会经 [CollabRolesState] 原样透传,
 * 前端禁用只是"第一道闸门",不是"吞掉后端错误"的借口。
 */

/** role.id 长度上限,与 daemon 契约一致(超过返回人话错误,不截断、不静默)。 */
const val COLLAB_ROLE_ID_MAX: Int = 64

/**
 * 校验 role.id(纯函数,单测钉死)。
 * @return null = 合法;否则返回**人话**原因(与 daemon 文案同口径,UI 直接展示这串文字)。
 *
 * 【空格判定用 isWhitespace 而非只判 ' '】slug 型 id 里无论空格还是 tab/换行都该拒,
 * 统一拦下;文案沿用 daemon 的"不能含空格"(用户视角的空格即任意空白)。
 */
fun validateCollabRoleId(id: String): String? {
    val t = id.trim()
    return when {
        t.isEmpty() -> "协同角色 id 不能为空"
        t.any { it.isWhitespace() } -> "协同角色 id 不能含空格"
        t.length > COLLAB_ROLE_ID_MAX -> "协同角色 id 过长(上限 $COLLAB_ROLE_ID_MAX)"
        else -> null
    }
}

/**
 * 角色显示名回落(纯函数):name 非空用 name,否则回落 id。
 * 列表/卡片靠它"一眼看到对应关系";两者都空(理论不发生,daemon 要求 id 必填)回落 id 原文。
 */
fun collabRoleDisplayName(role: CollabRole): String =
    role.name.takeIf { it.isNotBlank() } ?: role.id

/**
 * 角色摘要行(纯函数):列表项第二行要的"agent + model/effort"对应关系。
 * - agent 名优先走既有 [selectedAgentLabel](与全 App 命名一致),未知原样显示;
 * - model/effort 任一非空就补进摘要,中间用"·"分隔;都为空则只留 agent;
 * - 全空(既没 agent 也没 id)返回空串,调用方据此不占位、不伪造。
 */
fun collabRoleSummaryLine(role: CollabRole): String {
    val agent = role.agentId.takeIf { it.isNotBlank() } ?: role.id.takeIf { it.isNotBlank() } ?: return ""
    val parts = mutableListOf(selectedAgentLabel(agent))
    role.model.takeIf { it.isNotBlank() }?.let { parts += it }
    role.effort.takeIf { it.isNotBlank() }?.let { parts += it }
    return parts.joinToString(" · ")
}
