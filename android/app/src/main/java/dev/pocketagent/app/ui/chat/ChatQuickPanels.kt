package dev.pocketagent.app.ui.chat

import android.content.Context
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.MutableState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.LocalView
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Popup
import androidx.compose.ui.window.PopupProperties
import dev.pocketagent.app.R as AppR
import dev.pocketagent.app.ui.ContextRing
import dev.pocketagent.app.ui.TurnSession
import dev.pocketagent.app.ui.applyModelPick
import dev.pocketagent.app.ui.effortLabelOf
import dev.pocketagent.app.ui.modelsForAgent
import dev.pocketagent.app.ui.selectedAgentLabel
import dev.pocketagent.app.ui.ContextUsageCard
import dev.pocketagent.app.ui.component.PaHaptics
import dev.pocketagent.app.ui.component.UsageCache
import dev.pocketagent.app.ui.component.UsageSnapshot
import dev.pocketagent.app.ui.model.EffectiveSelectionUi
import dev.pocketagent.app.ui.ModelQuickSwitchLogic
import dev.pocketagent.app.ui.model.ModelSelectionIntent
import dev.pocketagent.app.ui.model.ModelSelectionRepository
import dev.pocketagent.app.ui.model.ModelSelectionState
import dev.pocketagent.app.ui.model.cardEffortLevels
import dev.pocketagent.app.ui.anim.pressable
import dev.pocketagent.app.ui.model.modelDisplayLabel
import dev.pocketagent.app.ui.theme.PaEditorial
import dev.pocketagent.app.ui.theme.PaHacker
import pocketagent.v1.ModelInfo

/**
 * 模型快速切换面板 + 上下文窗口明细卡(C1 拆分,2026-10-09):两者都从
 * ChatScreen 巨函数整体迁出,清单两卡同源/断网缓存兜底等语义一字不差。
 */

/** 快速切换面板:与完整选择卡同一数据源,稳定组合位挂载(不受输入区逐帧重组影响)。 */
@Composable
internal fun ChatQuickModelsPanel(
    showQuickModels: MutableState<Boolean>,
    showModelCard: MutableState<Boolean>,
    modelSelectionState: ModelSelectionState,
    modelSelection: EffectiveSelectionUi?,
    sessionAgent: String,
    sessionModels: List<ModelInfo>,
    effectiveModelId: String,
    st: TurnSession.State,
    threadId: String,
    onModelPicked: () -> Unit,
) {
    // 【快速切换面板(2026-10-02 流畅度修复)】从输入区行内 Popup 挪到此处与
    // showModelCard 同层的稳定组合位 —— 输入区在流式输出时每帧重组,行内 Popup
    // 被反复打断(真机:轮次运行中点模型行,面板闪没/开不出,用户裁决"切换不
    // 流畅"的主因)。稳定层挂载后不受逐帧重组影响。
    if (showQuickModels.value) {
        // 【L2 真机发现 5 · 两卡同源】模型清单改用与完整卡(ui/model.ModelEffortCard)
        // 完全同一份数据:ModelSelectionRepository.state.models(daemon 会话级申报,
        // listModelsWithSelection 按 agent/thread 收窄)经同一条 modelsForAgent 过滤
        // —— 与卡面第 203 行逐字同式。
        // 【根因】旧实现读本页全局大表 models(不带 agent 过滤的全 agent ListModels
        // + 10s 巡检合并),而巡检合并的过滤键是 provider 字段:大表里 pi 系模型的
        // provider 是上游供应商(如 zai-coding-cn)而非 "pi",永远滤不掉,于是 pi
        // 会话的快速面板摆出 6 个模型、完整卡按申报只有 2 个 —— 同一入口两级清单。
        // 【以申报为准】申报表就是 daemon 会为该会话接受的 set_model 全集:表外模型
        // 选了也会被真值回填校正(L2 发现 1 的静默回退正是表外选择),且 reducer 的
        // 失效 effort 清理(cleanEffortFor)只认申报表。清单统一后,选中态
        // (qm.id == effectiveModelId)、MRU 失效淘汰、提交流(applyModelPick →
        // 仓库 SelectModel)全部原样不动。
        val cardModels = remember(modelSelectionState.models, sessionAgent) {
            modelsForAgent(modelSelectionState.models, sessionAgent)
        }
        val quickModels = remember(cardModels, sessionAgent) {
            ModelQuickSwitchLogic.quickModels(sessionAgent, cardModels)
        }
        Popup(
            alignment = Alignment.BottomStart,
            onDismissRequest = { showQuickModels.value = false },
            properties = PopupProperties(focusable = true),
        ) {
            Column(
                Modifier
                    .widthIn(max = 330.dp)
                    // 黑客模式 GraphiteDrawer 底 + 等宽字;常规 Surface 底(同浮层口径)
                    .background(
                        if (PaHacker.isHacker) PaEditorial.GraphiteDrawer else PaEditorial.Surface,
                        RoundedCornerShape(14.dp),
                    )
                    .border(1.dp, PaEditorial.Hairline, RoundedCornerShape(14.dp))
                    .padding(vertical = 6.dp),
            ) {
                val mono = if (PaHacker.isHacker) PaHacker.mono else null
                val effortHapticView = LocalView.current
                // 【面板头部:当前 vs 正在跑】模型行精简后,"正在运行的模型"信息
                // 收进这里 —— 对比语境(当前选择 vs 实际在跑)在面板里才读得通。
                if (st.runningEffort != null && st.runningModel.isNotBlank()) {
                    // 【统一翻译层】申报表优先、全 agent 大表补位、裸名/原 id 兜底
                    // (modelDisplayLabel,L2 发现 5:正在跑的模型不在申报表里时
                    // 不再把 "hermes/deepseek-flash" 这类裸 id 直接亮出来)
                    val actualName = modelDisplayLabel(st.runningModel, cardModels, sessionModels)
                    Text(
                        "正在运行：$actualName · ${effortLabelOf(st.runningEffort.orEmpty())}",
                        Modifier
                            .fillMaxWidth()
                            .padding(horizontal = 14.dp, vertical = 6.dp),
                        fontSize = 11.sp, color = PaEditorial.Subtle, fontFamily = mono,
                    )
                }
                if (quickModels.isEmpty()) {
                    Text(
                        // 【两卡同源】空态判定跟着申报表走:申报表空 = 完整卡此刻
                        // 也只显示"Agent 暂未报告模型,请刷新",两级入口说同一句话
                        if (cardModels.isEmpty()) "暂无可用模型，请进完整选择器刷新"
                        else "暂无可快速切换的模型",
                        Modifier.padding(horizontal = 14.dp, vertical = 12.dp),
                        fontSize = 12.5.sp, color = PaEditorial.Subtle, fontFamily = mono,
                    )
                }
                quickModels.forEach { qm ->
                    val current = qm.id == effectiveModelId
                    Row(
                        Modifier
                            .fillMaxWidth()
                            .pressable {
                                showQuickModels.value = false
                                applyModelPick(threadId, sessionAgent, qm.id)
                                onModelPicked() // 行内闪光:模型名刚真实切过一次
                            }
                            .heightIn(min = 44.dp)
                            .padding(horizontal = 14.dp, vertical = 8.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Column(Modifier.weight(1f)) {
                            Text(
                                // 【统一翻译层】与面板头部同一条解析链:申报表 displayName
                                // → 大表 displayName → 裸名/原 id(行数据自带的 qm.label
                                // 只查申报表,这里补上大表补位,避免申报表缺展示名时露裸 id)
                                modelDisplayLabel(qm.id, cardModels, sessionModels),
                                fontSize = 13.5.sp,
                                color = if (current) PaEditorial.Primary else PaEditorial.Ink,
                                fontWeight = if (current) FontWeight.SemiBold else FontWeight.Normal,
                                maxLines = 1, overflow = TextOverflow.Ellipsis,
                                fontFamily = mono,
                            )
                            if (qm.detail.isNotEmpty()) {
                                Text(
                                    qm.detail, fontSize = 10.5.sp,
                                    color = PaEditorial.Subtle, fontFamily = mono,
                                )
                            }
                        }
                        if (current) {
                            Spacer(Modifier.width(8.dp))
                            Icon(
                                painterResource(AppR.drawable.ic_lucide_check), "当前模型",
                                tint = PaEditorial.Primary, modifier = Modifier.size(16.dp),
                            )
                        }
                    }
                }
                // 【思考强度行(2026-10-02 流畅度补全)】强度切换此前藏在"更多…"二级
                // 完整卡里,一级面板只有模型 —— 用户裁决"切换不流畅"的第二主因。
                // 这里按会话 agent 的档位集渲染强度胶囊,单点即切
                // (下一轮生效语义与完整卡一致);【P3-T4 接线④】选中态来自仓库
                // display(ModelSelectionReducer.display),点击改发 SelectEffort 意图
                // —— 不再直读/直写 AgentConnection.selectedEffort 与 setThreadEffort。
                // 【两卡同源(L2 发现 5)】强度档位改走完整卡同一条 cardEffortLevels:
                // daemon 真值 available_efforts(仓库 display 回传)优先,退申报表
                // effortsFor。旧实现读全局大表,其 provider 字段是上游供应商而非
                // agent id,effortsFor 的 agent 匹配永远落空 —— pi 会话在快速面板
                // 只看得到内置五档(默认/低/中/高/极高),完整卡却是 pi 动态全集
                // (默认/关/低/高/最大),同一发现的两半。选中态
                // (modelSelection?.effort)与单点即切的下发链原样不动。
                val quickEfforts = remember(cardModels, effectiveModelId, sessionAgent, modelSelection?.availableEfforts) {
                    cardEffortLevels(
                        models = cardModels,
                        agentId = sessionAgent,
                        modelId = effectiveModelId,
                        daemonEfforts = modelSelection?.availableEfforts.orEmpty(),
                    )
                }
                Row(
                    Modifier
                        .fillMaxWidth()
                        .horizontalScroll(rememberScrollState())
                        .padding(horizontal = 8.dp, vertical = 2.dp),
                    horizontalArrangement = Arrangement.spacedBy(6.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(
                        "强度",
                        Modifier.padding(start = 6.dp),
                        fontSize = 11.sp, color = PaEditorial.Subtle, fontFamily = mono,
                    )
                    quickEfforts.forEach { e ->
                        val selected = modelSelection?.effort == e.id
                        Text(
                            e.label,
                            Modifier
                                .clip(RoundedCornerShape(50))
                                .pressable {
                                    PaHaptics.light(effortHapticView)
                                    ModelSelectionRepository.send(ModelSelectionIntent.SelectEffort(e.id))
                                }
                                .padding(horizontal = 10.dp, vertical = 5.dp),
                            fontSize = 12.sp,
                            color = if (selected) PaEditorial.Primary else PaEditorial.Muted,
                            fontWeight = if (selected) FontWeight.SemiBold else FontWeight.Normal,
                            fontFamily = mono,
                        )
                    }
                }
                // 面板尾:「更多…」进完整选择卡(跟随桌面/全部模型/强度档位)
                Box(
                    Modifier
                        .fillMaxWidth()
                        .padding(horizontal = 8.dp, vertical = 2.dp)
                        .height(1.dp)
                        .background(PaEditorial.Hairline)
                )
                Row(
                    Modifier
                        .fillMaxWidth()
                        .pressable {
                            showQuickModels.value = false
                            showModelCard.value = true
                        }
                        .heightIn(min = 44.dp)
                        .padding(horizontal = 14.dp, vertical = 8.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(
                        "更多…",
                        Modifier.weight(1f),
                        fontSize = 13.sp, color = PaEditorial.Muted, fontFamily = mono,
                    )
                    Icon(
                        painterResource(AppR.drawable.ic_lucide_chevron_right), "打开完整模型选择器",
                        tint = PaEditorial.Subtle, modifier = Modifier.size(15.dp),
                    )
                }
            }
        }
    }
}

/** 上下文窗口明细卡:实时值优先,断网走本地缓存兜底(3s 刷新 + 恢复连接即刷)。 */
@Composable
internal fun ChatContextUsagePanel(
    showContextCard: MutableState<Boolean>,
    ctx: Context,
    usageRefreshTick: Int,
    connOk: Boolean,
    connDown: Boolean,
    threadId: String,
    sessionAgent: String,
    windowIsFallback: Boolean,
    contextWindow: Long,
    session: TurnSessionUiState,
) {
    // 上下文窗口明细:点输入栏"上下文"胶囊打开(真实用量卡,替代旧估算版)。
    // 【断网可读】打开时先读本地缓存,再用实时数据覆盖;usageRefreshTick / 连接恢复
    // 都会让 cached 重新求值,从而"自动刷新"(读取 tick 作为 remember 键即登记了订阅)。
    if (showContextCard.value) {
        UsageCache.init(ctx)
        val cached = remember(usageRefreshTick, connOk, threadId) { UsageCache.load(threadId) }
        // 实时窗口已知就用实时值;否则退回缓存里的最后一次真实窗口。
        val liveWindowKnown = !windowIsFallback && contextWindow > 0
        val cw: Long
        val fb: Boolean
        if (liveWindowKnown) {
            cw = contextWindow
            fb = false
        } else if (cached != null && cached.contextWindow > 0) {
            cw = cached.contextWindow
            fb = cached.windowIsFallback
        } else {
            cw = contextWindow
            fb = windowIsFallback
        }
        ContextUsageCard(
            contextWindow = cw,
            windowIsFallback = fb,
            turnUsage = session.turnUsage ?: cached?.turnUsage,
            totalUsage = session.totalUsage ?: cached?.totalUsage,
            // 压缩同步:daemon 转发的"上下文已压缩"事件到达后,这里给出明确的压缩状态,
            // 而不是继续显示压缩前的旧占用(可能超出实际窗口)。
            compactionInProgress = session.compactionInProgress,
            lastCompactedAt = session.lastCompactedAt,
            // 压缩等待文案按会话归属 agent("等待 Pi/Codex 完成压缩"),不硬编码 codex。
            agentLabel = selectedAgentLabel(sessionAgent),
            // 只有"已知断网"(down)才显示离线提示条;checking 不显示(见上方 ③ 的注释)。
            online = !connDown,
            cachedAt = cached?.updatedAt,
            nowMillis = System.currentTimeMillis(),
            onDismiss = { showContextCard.value = false },
        )
    }
}
