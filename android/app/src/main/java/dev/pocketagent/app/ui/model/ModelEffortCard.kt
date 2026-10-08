package dev.pocketagent.app.ui.model

import androidx.compose.foundation.background
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalView
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.AgentConnection
import dev.pocketagent.app.R as AppR
import dev.pocketagent.app.ui.EffortOption
import dev.pocketagent.app.ui.anim.IconActionButton
import dev.pocketagent.app.ui.anim.pressable
import dev.pocketagent.app.ui.agentBrandIcon
import dev.pocketagent.app.ui.component.PaHaptics
import dev.pocketagent.app.ui.component.PaSheet
import dev.pocketagent.app.ui.effortLabelOf
import dev.pocketagent.app.ui.effortsFor
import dev.pocketagent.app.ui.modelBareName
import dev.pocketagent.app.ui.modelsForAgent
import dev.pocketagent.app.ui.theme.PaEditorial
import dev.pocketagent.app.ui.theme.PaHacker
import pocketagent.v1.ModelInfo

/**
 * 模型/思考强度卡(2026-10-04 会话选择与历史体验计划 Task 4 拆分件)。
 *
 * 【它是什么】原 ChatScreen.ModelEffortCard 整体搬出并换血:卡面不再自己发
 * RPC、不再持有 currentPick 内部选中态、不再读写 AgentConnection 全局 ——
 * 页面只传 [state](ModelSelectionRepository.state)与 [onIntent](意图回调),
 * 选中态唯一来源是 `ModelSelectionReducer.display(state)`
 * (daemon 真值 > 乐观投影 > 「上次记录」缓存,fromCache 会标注)。
 *
 * 【轮次红线】卡面只有选择意图(SelectModel/SelectEffort/ResetToDefault/Refresh),
 * 没有任何取消/改写运行中轮次的通道;运行中切换显示"将从下一轮生效"
 * (appliesToNextTurn,daemon 真值口径),正在运行的轮次不可达。
 *
 * 【失效 effort】切模型时的清理在数据层(ModelSelectionReducer.cleanEffortFor)
 * 已做;展示层再按 [cardSelectedEffort] 复核一遍 —— 档位集合不含当前强度时
 * 高亮自动落到"默认",绝不显示一个已失效的选中档(计划验收项)。
 *
 * 【未知 effort】daemon 未来新增取值:档位行不伪造,但把原 id 如实亮出
 * (effortLabelOf else -> id),并给「技术详情」入口(原文/来源/解释),
 * 不显示空白(计划验收项,与协同页未知状态同纪律)。
 *
 * 【提交中态(L2 真机发现 4)】点选到 daemon 写回之间乐观投影已上屏,但没有任何
 * 进行中指示,慢写回观感如回退 —— pending 在飞时([cardSubmission] 纯函数映射)
 * 卡面状态槽与被改的那一行亮「提交中…」;写回成功(pending 清)指示消失,
 * 失败回滚走既有错误链路(error 位)。
 */

// ---- 纯函数(JVM 单测钉死:ModelEffortOptionsTest / ChatModelRoutingTest)----

/** "跟随默认"判定:模型 id 为空(零值语义),或显式选了**本 agent**的 default
 *  条目;agent 未回填(空)时只有空 id 算跟随 —— 拼出来的 "/default" 是畸形
 *  id,不得误判成跟随(否则别家/畸形 default 会误高亮,ChatModelRoutingTest 钉死)。 */
internal fun cardIsFollowDefault(modelId: String, agentId: String): Boolean =
    modelId.isBlank() || (agentId.isNotBlank() && modelId == "$agentId/default")

/**
 * 选择来源的人话标签(信息行用)。UNKNOWN 不猜:daemon 原文有就展示原文,
 * 连原文都没有才落"未知来源"(词表外新增来源的降级口径,见 SelectionSource)。
 */
internal fun selectionSourceLabel(source: SelectionSource, sourceRaw: String): String =
    when (source) {
        SelectionSource.THREAD_OVERRIDE -> "本会话选择"
        SelectionSource.APP_DEFAULT -> "App 默认"
        SelectionSource.DESKTOP_DEFAULT -> "桌面默认"
        SelectionSource.NATIVE_CURRENT -> "Agent 当前值"
        SelectionSource.UNKNOWN -> sourceRaw.ifBlank { "未登记来源" }
    }

/**
 * 卡上档位表:daemon 真值带 available_efforts([daemonEfforts] 非空)就按它渲染
 * (首位恒补"默认"—— 不补的话选了动态档位就再也回不到跟随默认);
 * 没带就退既有 effortsFor 纯函数(内置默认档/pi agent 档位全家桶,规则已被
 * ModelEffortOptionsTest 钉死)。
 */
internal fun cardEffortLevels(
    models: List<ModelInfo>,
    agentId: String,
    modelId: String,
    daemonEfforts: List<String>,
): List<EffortOption> {
    val dynamic = daemonEfforts.filter { it.isNotBlank() }.distinct()
    if (dynamic.isNotEmpty()) {
        return listOf(EffortOption("", "默认")) + dynamic.map { EffortOption(it, effortLabelOf(it)) }
    }
    return effortsFor(models, modelId, agentId)
}

/**
 * 卡上该高亮的强度(展示层失效清理:计划验收项"先清理失效值,再显示可选档位")。
 *  - daemon 真值给了档位集合:不在集合里即失效 → 清成""(跟随默认高亮);
 *  - 集合为空且"跟随默认":档位由 agent 当前模型决定,客户端无法判定 → 保留原值不猜;
 *  - 其余按 ModelSelectionReducer.cleanEffortFor(无法判定也保留,等真值回填校正)。
 * 只影响显示,不反向写状态 —— 写侧清理归 reducer 的 SelectModel,避免"显示层
 * 回写状态"造成环(daemon 真值到达后自然校正)。
 */
internal fun cardSelectedEffort(
    models: List<ModelInfo>,
    agentId: String,
    modelId: String,
    effort: String,
    daemonEfforts: List<String>,
): String {
    if (effort.isBlank()) return ""
    val dynamic = daemonEfforts.filter { it.isNotBlank() }.distinct()
    if (dynamic.isNotEmpty()) return if (effort in dynamic) effort else ""
    if (cardIsFollowDefault(modelId, agentId)) return effort
    return ModelSelectionReducer.cleanEffortFor(models, agentId, effort, modelId)
}

// ---- 提交中态(L2 真机发现 4:点选到 daemon 写回之间无进行中指示,慢写回观感如回退)----

/** 「提交中」状态词(四词状态风格,与协同直控开关的「提交中」同款)。 */
private const val SUBMITTING_LABEL = "提交中…"

/**
 * 卡面「提交中」的展示映射(pending → 展示态;纯函数,JVM 单测钉死 —— 见
 * ModelEffortOptionsTest)。
 *
 * 【pending 是什么】ModelSelectionReducer 的在飞锚点:用户刚点选、乐观投影已上屏、
 * daemon 真值未回填(pending != null 即在飞;下发成功 confirmSent 清、真值到达
 * applyDaemon 清、失败 rollback 退回上一份并走既有错误链路)。
 *
 * 【指示钉在被改的行】与下发前的上一份选择([PendingSelection.previous])逐维
 * 比较,差在哪个维度哪一行亮「提交中…」;previous 缺席(此前从无任何已知选择)
 * 无法逐维归因 → 模型/强度两行都亮(保守多提示,不漏钉)。
 */
internal data class CardSubmissionUi(
    val inFlight: Boolean,
    val modelPending: Boolean,
    val effortPending: Boolean,
    /** 状态词;不在飞为空串。 */
    val label: String,
)

internal fun cardSubmission(state: ModelSelectionState): CardSubmissionUi {
    val pending = state.pending ?: return CardSubmissionUi(false, false, false, "")
    val prev = pending.previous
        ?: return CardSubmissionUi(true, modelPending = true, effortPending = true, label = SUBMITTING_LABEL)
    val display = ModelSelectionReducer.display(state)
    return CardSubmissionUi(
        inFlight = true,
        modelPending = display?.modelId.orEmpty() != prev.modelId,
        effortPending = display?.effort.orEmpty() != prev.effort,
        label = SUBMITTING_LABEL,
    )
}

/**
 * 模型与思考强度卡(弹窗卡面)。
 *
 * @param state 仓库状态(ModelSelectionRepository.state);选中态/模型表/加载与
 *   错误位全部来自它,卡面不自持任何选择状态。
 * @param sessionAgent 会话归属 agent(页面按 getThread 回填算好;空时退全局
 *   选中 agent 兜底,与旧卡一致)。
 * @param runtimeModel/runtimeEffort 正在运行轮次的模型/强度(TurnSession 只读,
 *   "Agent 当前"对照行用;卡面不写它)。
 * @param onIntent 意图出口:页面接 ModelSelectionRepository.send;卡面不碰
 *   RPC/缓存/全局状态。
 * @param onDismiss 关卡回调。@param onPicked 一次真实选择后的行内反馈钩子
 *   (宿主给模型行闪光用)。
 */
@Composable
internal fun ModelEffortCard(
    state: ModelSelectionState,
    sessionAgent: String,
    runtimeModel: String,
    runtimeEffort: String?,
    onIntent: (ModelSelectionIntent) -> Unit,
    onDismiss: () -> Unit,
    onPicked: () -> Unit = {},
) {
    // 选中态唯一来源:daemon 真值 > 乐观投影 > 「上次记录」(null = 都还没有)
    val display = ModelSelectionReducer.display(state)
    // 会话归属 agent:调用方没算出来(空)时退全局选中 agent,行为与旧卡一致
    val agent = sessionAgent.ifBlank { AgentConnection.selectedAgent }
    // 模型表:仓库全表按会话 agent 过滤(与输入栏芯片/快速面板同一条 modelsForAgent
    // 口径 —— pi 会话绝不串进 codex/zcode 的模型表,图3 实锤修复的延续)
    val models = remember(state.models, agent) { modelsForAgent(state.models, agent) }
    val selectedModelId = display?.modelId.orEmpty()
    // 档位判定的基准模型:"跟随默认"时用 agent 原生当前模型(旧卡同口径;
    // 空则 effortsFor 自己落到 $agent/default)
    val effortLevelModelId =
        if (cardIsFollowDefault(selectedModelId, agent)) runtimeModel else selectedModelId
    val daemonEfforts = display?.availableEfforts.orEmpty()
    val efforts = remember(state, runtimeModel, agent) {
        cardEffortLevels(models, agent, effortLevelModelId, daemonEfforts)
    }
    val selectedEffortShown = remember(state, models, agent) {
        cardSelectedEffort(models, agent, selectedModelId, display?.effort.orEmpty(), daemonEfforts)
    }
    val followDefault = cardIsFollowDefault(selectedModelId, agent)
    // 未知 effort / 未知来源 → 「技术详情」入口(原 id+原文+解释,绝不显示空白)
    val unknownEffort =
        selectedEffortShown.isNotBlank() && effortLabelOf(selectedEffortShown) == selectedEffortShown
    val unknownSource = display != null && display.source == SelectionSource.UNKNOWN
    var showTechDetails by remember { mutableStateOf(false) }
    // 【提交中态(L2 发现 4)】pending != null = 有一笔选择在飞:状态词占住卡面状态槽,
    // 被改的那一行亮「提交中…」;写回成功(pending 清)即消失,失败走既有错误链路。
    val submission = cardSubmission(state)
    // 模型维度在飞且目标是"跟随默认"(空 id,零值合法):指示亮在跟随默认行上。
    val modelPendingOnFollow = submission.modelPending && selectedModelId.isBlank()

    // 强度胶囊的触感落点(独立卡面,自取 LocalView;light 档见下方 onClick)
    val effortHapticView = LocalView.current
    val mono = if (PaHacker.isHacker) PaEditorial.MonoFont else null
    // 【D2 模态家族统一】裸 Dialog → PaSheet centered 变体:同一块玻璃材质、同一档
    // spring 出场(PaMotion.sheet)、同一个 Radius.Sheet 圆角令牌(原硬编码 22dp)。
    // 行为语义零变化:点窗外/返回键仍走 onDismiss,内容与交互一字不动。
    PaSheet(
        onDismissRequest = onDismiss,
        centered = true,
    ) {
        Column(
            Modifier
                .fillMaxWidth()
                .padding(20.dp),
        ) {
            Row(
                Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text("模型与思考强度", fontSize = PaEditorial.Type.Heading.fontSize, color = PaEditorial.Ink)
                IconActionButton(
                    AppR.drawable.ic_lucide_x, "关闭", onDismiss,
                    bg = androidx.compose.ui.graphics.Color.Transparent, size = 36.dp, iconSize = 18.dp,
                )
            }
            Spacer(Modifier.height(10.dp))

            // ---- 信息行(计划验收项⑤:来源/原文/生效时机/解释/上次记录标注)----
            if (display != null) {
                Text(
                    buildString {
                        append("来源：")
                        append(selectionSourceLabel(display.source, display.sourceRaw))
                        // 运行中选择:daemon 口径的"从下一轮生效",如实渲染
                        if (display.appliesToNextTurn) append(" · 将从下一轮生效")
                        // 「上次记录」:本地缓存尚未被 daemon 真值替换,必须标注
                        if (display.fromCache) append(" · 上次记录")
                    },
                    fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Muted, fontFamily = mono,
                )
                if (display.explanation.isNotBlank()) {
                    Text(
                        display.explanation,
                        fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Subtle,
                        modifier = Modifier.padding(top = 2.dp),
                    )
                }
            }
            // "Agent 当前"对照行(正在运行轮次的实际配置;TurnSession 只读投影)
            if (runtimeModel.isNotBlank()) {
                Text(
                    "Agent 当前：${
                        models.find { it.id == runtimeModel }?.displayName?.takeIf { it.isNotBlank() }
                            ?: modelBareName(runtimeModel)
                    }" + (runtimeEffort?.let { " · ${effortLabelOf(it)}" } ?: ""),
                    fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Muted,
                    modifier = Modifier.padding(top = 2.dp),
                )
            }
            Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                val syncLine = when {
                    // 提交中优先:这是用户刚点下的那笔选择的"进行中"真相(四词状态风格)
                    submission.inFlight -> submission.label
                    state.loading -> "正在同步电脑端真值…"
                    state.error != null -> state.error
                    display?.fromCache == true -> "显示上次记录;电脑端真值未达,可点刷新"
                    else -> "${models.count { !it.id.endsWith("/default") }} 个模型"
                }
                Text(
                    syncLine,
                    Modifier.weight(1f), fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Muted,
                )
                TextButton(onClick = { onIntent(ModelSelectionIntent.Refresh) }) { Text("刷新") }
            }

            // ---- 模型列表(限高可滚;"跟随默认"入口 + 全部模型)----
            Column(
                Modifier
                    .fillMaxWidth()
                    .height(300.dp)
                    .verticalScroll(rememberScrollState()),
            ) {
                // 「跟随默认」= 清会话覆盖(model/effort 双空,零值合法)—— 这是
                // 唯一能回到跟随默认的路(ResetToDefault,计划接线⑥)。
                ModelOptionRow(
                    title = "跟随默认",
                    // 【提交中钉行】清覆盖在飞:目标行就是这里(空 id = 跟随默认,零值合法)
                    subtitle = (if (agent == "codex") "使用电脑上 codex 当前的默认模型"
                        else "使用默认解析(桌面下发/Agent 原生当前值)") +
                        if (modelPendingOnFollow) " · ${submission.label}" else "",
                    selected = followDefault,
                    brandIconRes = agentBrandIcon(agent),
                    onClick = {
                        onPicked()
                        onIntent(ModelSelectionIntent.ResetToDefault)
                    },
                )
                if (models.isEmpty()) {
                    Text(
                        if (state.loading) "加载中…" else "Agent 暂未报告模型，请刷新",
                        fontSize = PaEditorial.Type.BodySmall.fontSize, color = PaEditorial.Subtle,
                        modifier = Modifier.padding(8.dp),
                    )
                }
                models.forEach { m ->
                    ModelOptionRow(
                        title = m.displayName.ifBlank { m.id },
                        detail = modelRowDetail(
                            contextWindow = m.contextWindow,
                            supportsThinking = m.supportsThinking,
                            supportsImages = m.supportsImages,
                            available = m.available,
                        ),
                        selected = m.id == selectedModelId,
                        // 【提交中钉行】模型切换在飞:指示亮在显示中的乐观目标行上
                        // (行组件无状态,文案由卡面算好传入,不新增行参数)
                        subtitle = if (submission.modelPending && m.id == selectedModelId) {
                            submission.label
                        } else "",
                        brandIconRes = agentBrandIcon(agent),
                        onClick = {
                            onPicked()
                            onIntent(ModelSelectionIntent.SelectModel(m.id))
                        },
                    )
                }
            }

            Spacer(Modifier.height(12.dp))
            Box(Modifier.fillMaxWidth().height(1.dp).background(PaEditorial.Hairline))
            Spacer(Modifier.height(12.dp))
            Text("思考强度", fontSize = PaEditorial.Type.Body.fontSize, color = PaEditorial.Ink)
            Spacer(Modifier.height(10.dp))
            // 【横向可滚】pi 动态全集 + "默认"最多 8 枚胶囊,窄屏一排放不下,
            // 溢出的档位以前会被直接裁掉(点不到也看不见)。选中态来自
            // cardSelectedEffort(展示层失效清理),不是全局可变状态。
            Row(
                horizontalArrangement = Arrangement.spacedBy(8.dp),
                modifier = Modifier.fillMaxWidth().horizontalScroll(rememberScrollState()),
            ) {
                efforts.forEach { e ->
                    val selected = selectedEffortShown == e.id
                    Box(
                        Modifier
                            .pressable {
                                // 强度切换 = light 档(高频低强度的刻度级反馈;模型/档位
                                // 选择卡内不惊扰)。LocalView 由卡面自取。
                                PaHaptics.light(effortHapticView)
                                onIntent(ModelSelectionIntent.SelectEffort(e.id))
                            }
                            .background(
                                if (selected) PaEditorial.Primary else PaEditorial.Surface,
                                RoundedCornerShape(50),
                            )
                            .padding(horizontal = 14.dp, vertical = 8.dp),
                    ) {
                        Text(
                            e.label,
                            fontSize = PaEditorial.Type.BodySmall.fontSize,
                            color = if (selected) PaEditorial.OnPrimary else PaEditorial.Muted,
                        )
                    }
                }
            }
            // 【提交中钉行】强度切换在飞:胶囊行下补一行状态小字(四词状态风格,不靠色);
            // 写回成功(pending 清)整行消失,失败回滚走既有错误链路(error 位)。
            if (submission.effortPending) {
                Text(
                    "思考强度${submission.label}",
                    fontSize = PaEditorial.Type.Caption.fontSize,
                    color = PaEditorial.Muted,
                    modifier = Modifier.padding(top = 6.dp),
                )
            }
            // 【未知 effort 不空白】档位行没有当前强度(不伪造)时,把原 id 如实亮出
            if (selectedEffortShown.isNotBlank() && efforts.none { it.id == selectedEffortShown }) {
                Text(
                    "当前强度：${effortLabelOf(selectedEffortShown)}（非本模型常规档位）",
                    fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Attention,
                    modifier = Modifier.padding(top = 6.dp),
                )
            }
            // 【技术详情入口】未知 effort / 未认知来源:给原文与解释,不给空白
            if (unknownEffort || unknownSource) {
                TextButton(onClick = { showTechDetails = !showTechDetails }) {
                    Text(if (showTechDetails) "收起技术详情" else "技术详情", fontSize = PaEditorial.Type.Caption.fontSize)
                }
                if (showTechDetails && display != null) {
                    Text(
                        buildString {
                            append("模型 ")
                            append(display.modelId.ifBlank { "(跟随默认)" })
                            append("\n强度 ")
                            append(display.effort.ifBlank { "(跟随默认)" })
                            append("\n来源 ")
                            append(display.sourceRaw.ifBlank { "(未提供)" })
                            if (display.explanation.isNotBlank()) {
                                append("\n")
                                append(display.explanation)
                            }
                        },
                        fontSize = PaEditorial.Type.Micro.fontSize, color = PaEditorial.Subtle, fontFamily = mono,
                        modifier = Modifier.padding(top = 4.dp),
                    )
                }
            }
            Spacer(Modifier.height(12.dp))
            Text(
                when {
                    // 运行中/乐观切换:明确"下一轮才生效",运行中的轮次不被改写
                    display?.appliesToNextTurn == true ->
                        "本次修改将从下一轮生效；正在运行的轮次按开始时的配置跑完。"
                    followDefault ->
                        "跟随默认解析（桌面下发/Agent 原生当前值）；选择具体模型后可设置它的思考档位。"
                    else ->
                        "修改将发送给电脑端确认，回填后显示为真值（含来源与生效时机）。"
                },
                fontSize = PaEditorial.Type.Caption.fontSize,
                color = PaEditorial.Subtle,
            )
        }
    }
}
