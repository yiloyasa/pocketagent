package dev.pocketagent.app.ui

import dev.pocketagent.app.R
import dev.pocketagent.app.ui.theme.EditorialTone

/**
 * 协同页纯函数层:
 *  - 可参与协同的 agent 清单 + 参与者可用状态归并(动态,不写死);
 *  - agent 多选切换与失效参与者剪除;
 *  - 每 agent 的角色绑定;
 *  - 派发可行性判定(0 人禁用 + 禁用原因,防重复提交同源);
 *  - 单个 agent 派发相位 → 四词状态 / 进度卡视图(设计稿 10);
 *  - 空态/加载态人话文案。
 *
 * 【为什么独立成层】这些规则最容易被"顺手改个条件"悄悄反转且零报错,
 * 只能等用户看到错乱的选中态/角色绑定才暴露。集中成纯函数至少让它们**可被**断言,
 * 不再依赖真机/daemon(相位四词映射见 [collabPhaseStatus];参与者归并/派发判定/
 * 进度卡映射见 CollabDispatchFlowTest;此前本文件无任何测试,2026-09-30 编辑式升级补齐)。
 */

/**
 * 可参与协同的 agent 清单(纯函数):只取 READY 的参与者。
 * 【与 [collabParticipants] 的关系】归并规则只有这一份 —— 本函数是"能选的"投影,
 * 参与者芯片区展示的是含不可用原因的全集(结论 1);两处永远同源,不会各裁各的。
 * 绝不写死 listOf("pi","codex",...) —— 这是旧实现踩过的坑(用户明确禁止),
 * 新增一家 daemon 报上来就自动出现;未知 id 用通用 CPU 图标合成,不静默丢弃。
 */
fun collabCapableAgents(
    allAgents: List<AgentInfo>,
    collabIds: List<String>,
    disabled: Set<String>,
): List<AgentInfo> = collabParticipants(allAgents, collabIds, disabled)
    .filter { it.state == CollabParticipantState.READY }
    .map { it.agent }

/**
 * 参与者可用状态(结论 1:派发前"可用/不可用及原因"要一眼可辨)。
 *  - READY:能力支持 + 电脑端在线 + 手机端未停用,可勾选;
 *  - OFFLINE:daemon 报了协同 id 但该 agent 电脑端当前不可用(未安装/接入中);
 *  - LOCALLY_DISABLED:用户在手机端把它停用了 —— 原因在本地,不该装作不存在。
 */
enum class CollabParticipantState { READY, OFFLINE, LOCALLY_DISABLED }

/**
 * 协同参与者 = agent + 可用状态 + 不可用人话原因(READY 时为 null)。
 * 不可用的也**列出来**(灰显 + 原因),不静默消失 —— 结论 1:用户需要知道
 * "为什么不让我选它",只显示可选项等于把原因藏进了列表的裁剪规则里。
 */
data class CollabParticipant(
    val agent: AgentInfo,
    val state: CollabParticipantState,
    /** 不可用人话原因;READY = null。 */
    val unavailableReason: String?,
)

/**
 * 参与者归并(纯函数):daemon 报的协同 id(collab:<id>)是唯一名单来源,逐个
 * 归并出可用状态。次序 = 可用的在前(先看到的都是能选的),组内按显示名 ——
 * 同一份数据两次渲染同序,不跳动。
 *
 * 【名单纪律不变】绝不写死 listOf("pi","codex",...):daemon 报了就出现,
 * 没报就不出现;未知 id 用通用 CPU 图标合成,保证"报了就一定显示"。
 */
fun collabParticipants(
    allAgents: List<AgentInfo>,
    collabIds: List<String>,
    disabled: Set<String>,
): List<CollabParticipant> {
    val collabSet = collabIds.toSet()
    val byId = allAgents.associateBy { it.id }
    val out = collabIds.map { id ->
        val known = byId[id]
        when {
            // 本地停用:原因在我们手里,如实说,不装作"电脑端没这个 agent"。
            id in disabled -> {
                val agent = known ?: syntheticCollabAgent(id)
                CollabParticipant(agent, CollabParticipantState.LOCALLY_DISABLED, "${agent.name}:已在手机端停用,可在主页的 Agent 磁贴重新启用")
            }
            known == null -> CollabParticipant(syntheticCollabAgent(id), CollabParticipantState.READY, null)
            !known.available -> {
                // 不可用原因与首页 agent 网格同源(unavailableLabel):同一件事全 app 一个说法。
                CollabParticipant(known, CollabParticipantState.OFFLINE, "${known.name}:${unavailableLabel(known.id)}")
            }
            else -> CollabParticipant(known, CollabParticipantState.READY, null)
        }
    }
    return out.sortedWith(
        compareBy({ if (it.state == CollabParticipantState.READY) 0 else 1 }, { it.agent.name })
    )
}

/** daemon 报了协同 id 但没出现在已知 agent 表里:通用 CPU 图标合成一条,不静默丢弃。 */
private fun syntheticCollabAgent(id: String): AgentInfo = AgentInfo(
    id = id,
    name = selectedAgentLabel(id),
    method = "native",
    subtitle = "电脑端协同 agent",
    iconRes = R.drawable.ic_lucide_cpu,
    available = true,
)

/** 不可用参与者的原因汇总行(芯片下方的小字);全员可用 = null(不占位)。 */
fun collabUnavailableNote(participants: List<CollabParticipant>): String? {
    val notes = participants
        .filter { it.state != CollabParticipantState.READY }
        .mapNotNull { it.unavailableReason }
    return notes.takeIf { it.isNotEmpty() }?.joinToString(" · ")
}

/** agent 多选:把 id 加进/移出已选集合(纯函数)。 */
fun toggleCollabSelection(selected: Set<String>, id: String): Set<String> =
    if (id in selected) selected - id else selected + id

/**
 * 剪除已失效的选中项(纯函数):agent 掉线/被停用后,它不再能参与本次协同,
 * 留在已选集合里会让"已选 N 位"与实际能派发的对不上。派发判定与按钮都按
 * 剪除后的集合算 —— 看到的数字就是能派发的人数。
 */
fun pruneCollabSelection(selected: Set<String>, readyIds: Set<String>): Set<String> =
    selected intersect readyIds

/**
 * 每 agent 的角色绑定(纯函数):binding = Map<agentId, roleId>。
 * roleId 为 null/空 = 解绑(回到"未配置角色")。
 */
fun bindCollabRole(binding: Map<String, String>, agentId: String, roleId: String?): Map<String, String> {
    val next = binding.toMutableMap()
    if (roleId.isNullOrBlank()) next.remove(agentId) else next[agentId] = roleId
    return next
}

// ---- 派发可行性(结论 1/3):按钮的禁用态与禁用原因同源,不出"灰着但不知道为什么" ----

/**
 * 派发判定结果:[canDispatch] 决定按钮可点,disabledReason 决定按钮旁边那句
 * "为什么现在点不了"。两件事由同一个纯函数给出 —— 永远不会出现"按钮灰了
 * 但界面上找不到原因"或反过来。
 */
data class CollabDispatchDecision(
    val canDispatch: Boolean,
    /** 不可派发的人话原因;canDispatch = true 时为 null。 */
    val disabledReason: String?,
)

/**
 * 可派发判定(纯函数):至少一位参与者 + 指令非空 + 没有正在飞的一单。
 * 原因优先级 = 参与者 > 指令 > 提交中(用户按"从上往下补条件"的顺序补齐)。
 * [dispatching] 是防重复提交的 UI 位 —— 状态层 [CollabState.dispatching] 还有
 * 一道硬闸,两层各挡各的(UI 挡误触,状态层挡编程路径)。
 */
fun collabDispatchDecision(
    selectedCount: Int,
    instructionReady: Boolean,
    dispatching: Boolean,
): CollabDispatchDecision = when {
    selectedCount <= 0 -> CollabDispatchDecision(false, CollabCopy.REASON_NO_PARTICIPANT)
    !instructionReady -> CollabDispatchDecision(false, CollabCopy.REASON_NO_INSTRUCTION)
    dispatching -> CollabDispatchDecision(false, CollabCopy.REASON_DISPATCH_IN_FLIGHT)
    else -> CollabDispatchDecision(true, null)
}

/** 参与区的选中数摘要(trailing 小字);0 人也如实说,不藏数字。 */
fun collabSelectionSummary(selectedCount: Int): String = when {
    selectedCount <= 0 -> "未选参与者"
    else -> "已选 $selectedCount 位"
}

// ---- 进度卡状态映射(结论 2/4):相位 → 徽章四词 + 补充说明行 + 卡上入口 ----

/**
 * 逐 Agent 进度卡的派生视图(纯函数):徽章四词、徽章之外的一句补充说明、
 * 以及卡上该摆哪些入口。UI 不再各自 if/else,改坏映射会在单测里红。
 */
data class CollabProgressView(
    /** 四词徽章;null = 未派发(无事实不摆空壳徽章,与 collabPhaseStatus 同纪律)。 */
    val status: EditorialStatus?,
    /**
     * 徽章之外的一句补充(ACK/RUNNING 才有:四词"正在执行"分不出"到哪一步了";
     * DONE/ERROR 的徽章与原因块已说明,不再叠字)。
     */
    val detailLine: String?,
    /** 已有该成员的子线程 id → 可单独查看其会话(结论 2)。 */
    val canOpenSession: Boolean,
    /** 受理失败 → 卡上摆「重派该 Agent」入口(结论 2:失败不拖垮其余,也能单独重来)。 */
    val canRedispatch: Boolean,
)

fun collabProgressView(
    phase: CollabAgentPhase,
    hasThreadId: Boolean,
): CollabProgressView = when (phase) {
    CollabAgentPhase.IDLE ->
        CollabProgressView(status = null, detailLine = null, canOpenSession = false, canRedispatch = false)
    CollabAgentPhase.ACK ->
        CollabProgressView(
            status = EditorialStatus.RUNNING,
            detailLine = "已收到指令,等待电脑端受理",
            canOpenSession = false, // 还没拿到子线程,没有会话可看 —— 不摆假入口
            canRedispatch = false,
        )
    CollabAgentPhase.RUNNING ->
        CollabProgressView(
            status = EditorialStatus.RUNNING,
            detailLine = "执行中 · 等待电脑端回报",
            canOpenSession = hasThreadId,
            canRedispatch = false,
        )
    CollabAgentPhase.DONE ->
        CollabProgressView(
            status = EditorialStatus.COMPLETED,
            detailLine = null,
            canOpenSession = hasThreadId,
            canRedispatch = false,
        )
    CollabAgentPhase.ERROR ->
        CollabProgressView(
            status = EditorialStatus.NOT_COMPLETED,
            detailLine = null, // 原因块承接解释,不叠一行半截话
            canOpenSession = hasThreadId,
            canRedispatch = true,
        )
}

/** 单个 agent 在协同页中的派发相位(客户端真实可知的态,绝不编造 agent 回复)。 */
enum class CollabAgentPhase { IDLE, ACK, RUNNING, DONE, ERROR }

/**
 * 派发相位 → 四词状态(纯函数,单测钉死 CollabPhaseStatusTest;设计稿 10 的口径):
 *  - ACK(已收到指令)/ RUNNING(daemon 已受理、执行中)→ 「正在执行」
 *    (README:接受/运行中都表述为正在执行;受理即起飞,结果另说);
 *  - DONE → 「已完成」;ERROR → 「未完成」(措辞禁「失败」「错误」,四词纪律);
 *  - IDLE(还没派发)→ null:无事实不摆空壳徽章(与 editorialStatusOrNull 同纪律)。
 *
 * 一位成员失败不拖垮其余 —— 相位按成员独立判定,本函数绝不读全局状态。
 */
fun collabPhaseStatus(phase: CollabAgentPhase): EditorialStatus? = when (phase) {
    CollabAgentPhase.IDLE -> null
    CollabAgentPhase.ACK, CollabAgentPhase.RUNNING -> EditorialStatus.RUNNING
    CollabAgentPhase.DONE -> EditorialStatus.COMPLETED
    CollabAgentPhase.ERROR -> EditorialStatus.NOT_COMPLETED
}

/** 派发相位的语义色档(徽章/进度行共用;IDLE = 中性灰,不是"运行中"的绿)。 */
fun collabPhaseTone(phase: CollabAgentPhase): EditorialTone =
    collabPhaseStatus(phase)?.tone ?: EditorialTone.GREY

/**
 * 卡片相位文案(纯函数):只描述客户端真实态与"等待 daemon"的诚实态,
 * 不写死任何 agent 的回复内容(AUDIT §3.3:无能力/未接入时如实说人话,禁"未知/错误")。
 * 【四词纪律】ERROR 档的词只能落在「未完成」(collabPhaseStatus 措辞纪律),
 * 这里的 ERROR 文案只是"正常路径到不了"的兜底,同样不用「失败/出错」裸词。
 */
fun collabPhaseLabel(phase: CollabAgentPhase): String = when (phase) {
    CollabAgentPhase.IDLE -> "等待指令"
    CollabAgentPhase.ACK -> "已收到指令"
    CollabAgentPhase.RUNNING -> "执行中 · 等待电脑端回报"
    CollabAgentPhase.DONE -> "已完成"
    CollabAgentPhase.ERROR -> "未完成"
}

/** 协同页空态/加载态人话文案(集中常量,便于统一口径,禁止"未知/错误"裸词)。 */
object CollabCopy {
    const val EMPTY_NO_CAPABILITY = "当前连接的电脑尚未接入协同能力,无法发起多 agent 协同"
    const val EMPTY_NO_SELECTION = "先勾选这次要一起干活的 agent"
    const val LOADING_AGENTS = "正在读取电脑端可协同的 agent…"
    const val WAITING_DAEMON = "指令已派发给各 agent,正在等待电脑端执行反馈(协同扇出接口接入中)"
    const val ROLE_NONE = "未配置角色"

    // ---- 派发按钮的禁用原因(collabDispatchDecision 同源,按钮灰 = 旁边有原因) ----
    const val REASON_NO_PARTICIPANT = "先勾选至少一位参与者,才能派发"
    const val REASON_NO_INSTRUCTION = "写下要协同完成的任务,即可派发"
    const val REASON_DISPATCH_IN_FLIGHT = "正在派发,请稍候"
    const val BTN_DISPATCH = "派发"
    const val BTN_DISPATCHING = "派发中…"

    // ---- 进度卡入口(结论 2:失败卡要有"查看原因"与"重派该 Agent") ----
    // 【P1 协同闭环(Task 6/7)后】派发后的成员卡改由 CollabSessionRepository
    // 的单一状态树驱动,入口/禁用原因/七词状态词表移至 ui.collab.CollabCopy
    // (会话域文案);本文件的派发前文案(选人/派发按钮)与下面的相位映射
    // (CollabAgentPhase → 四词,EditorialListWaveTest 钉死)保持原样不动。
    const val VIEW_REASON = "查看原因"
    const val HIDE_REASON = "收起原因"
    const val REDISPATCH_MEMBER = "重派该 Agent"
    const val OPEN_SESSION = "查看会话"
}
