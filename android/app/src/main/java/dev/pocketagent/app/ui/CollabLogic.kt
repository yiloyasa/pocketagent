package dev.pocketagent.app.ui

import dev.pocketagent.app.R
import pocketagent.v1.CollabRole

/**
 * 协同页纯函数层:
 *  - 可参与协同的 agent 清单(动态,不写死);
 *  - agent 多选切换;
 *  - 每 agent 的角色绑定;
 *  - 单个 agent 派发相位 → 卡片文案;
 *  - 空态/加载态人话文案。
 *
 * 【为什么独立成层】这些规则最容易被"顺手改个条件"悄悄反转且零报错,
 * 只能等用户看到错乱的选中态/角色绑定才暴露。集中成纯函数至少让它们**可被**断言,
 * 不再依赖真机/daemon。
 *
 * ⚠ 本文件当前**没有**对应单测:原注释指向的 CollabScreenLogicTest 并不存在
 * (2026-09-23 W11c 核验:test 目录下没有任何用例引用这里的函数)。留这句话是为了
 * 不让下一个人再去找一个不存在的文件 —— 要么补测试,要么别再声称"单测钉死"。
 */

/**
 * 可参与协同的 agent 清单(纯函数):
 * 只取 daemon 动态上报、且确实接入协同(capabilities 的 collab:<id>)的 agent,
 * 并排除本地已停用;其余一律不显示。绝不写死 listOf("pi","codex",...) ——
 * 这是旧实现踩过的坑(用户明确禁止),新增一家 daemon 报上来就自动出现。
 *
 * 【与 daemon 同真源】collab:<id> 由 daemon 在 GetHostInfo.capabilities 报出,
 * 这里只消费、不发明名单。若某 collab id 没出现在已知 agent 表里(罕见),
 * 也用通用 CPU 图标合成一条,保证"daemon 报了就一定显示",不静默丢弃。
 */
fun collabCapableAgents(
    allAgents: List<AgentInfo>,
    collabIds: List<String>,
    disabled: Set<String>,
): List<AgentInfo> {
    val collabSet = collabIds.toSet()
    val known = allAgents
        .filter { it.id in collabSet && it.available && it.id !in disabled }
    val knownIds = known.map { it.id }.toSet()
    // 罕见兜底:daemon 报了协同但没出现在已知表里,也要让用户能选到,不静默消失。
    val synthetic = collabIds
        .filter { it !in knownIds && it !in disabled }
        .map {
            AgentInfo(
                id = it,
                name = selectedAgentLabel(it),
                method = "native",
                subtitle = "电脑端协同 agent",
                iconRes = R.drawable.ic_lucide_cpu,
                available = true,
            )
        }
    return (known + synthetic).sortedBy { it.name }
}

/** agent 多选:把 id 加进/移出已选集合(纯函数)。 */
fun toggleCollabSelection(selected: Set<String>, id: String): Set<String> =
    if (id in selected) selected - id else selected + id

/**
 * 每 agent 的角色绑定(纯函数):binding = Map<agentId, roleId>。
 * roleId 为 null/空 = 解绑(回到"未配置角色")。
 */
fun bindCollabRole(binding: Map<String, String>, agentId: String, roleId: String?): Map<String, String> {
    val next = binding.toMutableMap()
    if (roleId.isNullOrBlank()) next.remove(agentId) else next[agentId] = roleId
    return next
}

/** 单个 agent 在协同页中的派发相位(客户端真实可知的态,绝不编造 agent 回复)。 */
enum class CollabAgentPhase { IDLE, ACK, RUNNING, DONE, ERROR }

/**
 * 卡片相位文案(纯函数):只描述客户端真实态与"等待 daemon"的诚实态,
 * 不写死任何 agent 的回复内容(AUDIT §3.3:无能力/未接入时如实说人话,禁"未知/错误")。
 */
fun collabPhaseLabel(phase: CollabAgentPhase): String = when (phase) {
    CollabAgentPhase.IDLE -> "等待指令"
    CollabAgentPhase.ACK -> "已收到指令"
    CollabAgentPhase.RUNNING -> "执行中 · 等待电脑端回报"
    CollabAgentPhase.DONE -> "已完成"
    CollabAgentPhase.ERROR -> "执行出错"
}

/** 协同页空态/加载态人话文案(集中常量,便于统一口径,禁止"未知/错误"裸词)。 */
object CollabCopy {
    const val EMPTY_NO_CAPABILITY = "当前连接的电脑尚未接入协同能力,无法发起多 agent 协同"
    const val EMPTY_NO_SELECTION = "先勾选这次要一起干活的 agent"
    const val LOADING_AGENTS = "正在读取电脑端可协同的 agent…"
    const val WAITING_DAEMON = "指令已派发给各 agent,正在等待电脑端执行反馈(协同扇出接口接入中)"
    const val ROLE_NONE = "未配置角色"
}
