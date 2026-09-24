package dev.pocketagent.app.ui

import androidx.compose.animation.animateContentSize
import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.Spring
import androidx.compose.animation.core.spring
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.IntrinsicSize
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.hapticfeedback.HapticFeedbackType
import androidx.compose.ui.platform.LocalHapticFeedback
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.AgentConnection
import dev.pocketagent.app.R as AppR
import dev.pocketagent.app.ui.anim.pressable
import dev.pocketagent.app.ui.anim.pressableLong
import dev.pocketagent.app.ui.component.PaSheet
import dev.pocketagent.app.ui.theme.Pa
import dev.pocketagent.app.ui.theme.PaGlassToken
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import pocketagent.v1.GetThreadHistoryRequest

/**
 * 子智能体内联卡 + 子对话页增强件(T-SUB)。
 *
 * 视觉/交互规格来源:蓝本 §4(单卡/组卡/长按快览)、§5.2(兄弟切换)、§8(动效);
 * 数据源:S1 修正案"子代理=真线程"—— 卡片数据=thread/list 过滤出的子线程组,
 * 点击进子对话=直接复用 chat/{childThreadId} 路由打开真线程(流式/输入/审批
 * 全功能),不在本文件造第二条渲染管线。
 *
 * 【兄弟切换的取舍:降级钮方案】蓝本 §5.2 的 HorizontalPager 需要把整个子对话
 * 页包进 Pager,而子对话页就是既有 ChatScreen(自带状态机/输入区/审批条,
 * 5000+ 行)——套 Pager 意味着重造页面或嵌套整个屏,与"复用整条线程管线"的
 * S1 方向冲突。故采用任务书预案的降级方案:头部来源行右侧 ◀ n/N ▶ 切换钮,
 * 点击导航到上/下一个子线程(chat/{siblingId}),转场走 NavHost 全局动效
 * (fadeIn 220ms + slide,与主对话↔子对话下钻同款方向感)。
 */

/** 单卡/组卡通用的 agent 品牌图标;未知 agent id 用通用占位(沿用既有约定)。 */
@Composable
private fun SubAgentBrandIcon(agentId: String, size: Int) {
    val res = agentBrandIcon(agentId) ?: AppR.drawable.ic_lucide_cpu
    Icon(
        painterResource(res), null,
        tint = Pa.Accent, modifier = Modifier.size(size.dp),
    )
}

/** 运行态 → 状态色(蓝本 §4.1:运行=品牌蓝 / 等审批·失败=红 / 完成=绿 / 排队·取消=灰)。 */
private fun subAgentStateColor(state: SubAgentRunState): Color = when (state) {
    SubAgentRunState.RUNNING -> Pa.Accent
    SubAgentRunState.WAITING_APPROVAL, SubAgentRunState.FAILED -> Pa.Red
    SubAgentRunState.DONE -> Pa.Green
    SubAgentRunState.QUEUED, SubAgentRunState.CANCELLED -> Pa.TextTertiary
}

/**
 * 状态点:运行中做 ~1.2s 周期的呼吸(alpha 0.4↔1.0,蓝本 §8),
 * 其余状态常亮 —— 与工具卡三色语义同源(见 ChatScreen ToolCallCard)。
 * 呼吸驱动用 Animatable + 临界阻尼低刚度 spring 往返(与 TypingDots 同族):
 * 速度两端自然归零,没有 tween 定时器到点掐断的生硬感;协程随组合取消,不泄漏。
 */
@Composable
private fun SubAgentStatusDot(state: SubAgentRunState, size: Int = 8) {
    val color = subAgentStateColor(state)
    if (state == SubAgentRunState.RUNNING) {
        val breath = remember { Animatable(0.4f) }
        LaunchedEffect(Unit) {
            while (true) {
                breath.animateTo(1f, spring(dampingRatio = 1f, stiffness = 55f))
                breath.animateTo(0.4f, spring(dampingRatio = 1f, stiffness = 55f))
            }
        }
        Box(Modifier.size(size.dp).alpha(breath.value).background(color, CircleShape))
    } else {
        Box(Modifier.size(size.dp).background(color, CircleShape))
    }
}

/** 快览弹层/迷你行的角色短标签。 */
internal fun subAgentRoleLabel(role: String): String = when (role) {
    "user" -> "我"
    "assistant" -> "AI"
    "thinking" -> "思考"
    "tool" -> "工具"
    "plan" -> "计划"
    else -> role
}

/**
 * 单卡(该主线程只有 1 个子代理线程,蓝本 §4.1):
 * 蓝底卡 + 左缘品牌竖条 + agent 图标 + 标题 + 状态点 + 最后消息预览(≤2 行渐隐)
 * + 计数徽章;整卡可点进子对话、长按弹层快览。
 */
@Composable
fun SubAgentCard(
    meta: SubAgentThreadMeta,
    state: SubAgentRunState,
    onOpen: () -> Unit,
    onPeek: () -> Unit,
    modifier: Modifier = Modifier,
) {
    // 可见时按需拉预览(一次 RPC/线程,结果缓存;拉不到就不显示)
    LaunchedEffect(meta.threadId) { SubAgentsState.requestPreview(meta.threadId) }
    // 预览缓存命中后 previewTick 自增触发重组;预览直接在组件内读缓存,
    // 不经调用方传参(item 内容 lambda 不会因缓存变化重组,传参会拿旧值)
    val tick = SubAgentsState.previewTick
    val currentPreview = SubAgentsState.cachedPreview(meta.threadId)
    val hf = LocalHapticFeedback.current
    Column(
        modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(12.dp))
            .background(Pa.AccentDim)
            // 玻璃材质语言:实色底(AccentDim) + 0.5dp 发丝高光边(同玻璃家族的亮边令牌)
            .border(0.5.dp, PaGlassToken.Highlight, RoundedCornerShape(12.dp))
            .pressableLong(
                onClick = {
                    hf.performHapticFeedback(HapticFeedbackType.TextHandleMove)
                    onOpen()
                },
                onLongPress = {
                    hf.performHapticFeedback(HapticFeedbackType.TextHandleMove)
                    onPeek()
                },
            ),
    ) {
        Row(
            Modifier
                .fillMaxWidth()
                .height(IntrinsicSize.Min),
        ) {
            // 左缘品牌竖条(与工具卡的"动作"区分:这是"一个在干活的分身",身份感优先)
            Box(Modifier.width(3.dp).fillMaxHeight().background(Pa.Accent))
            Column(
                Modifier
                    .weight(1f)
                    .padding(start = 10.dp, end = 12.dp, top = 10.dp, bottom = 10.dp),
            ) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    SubAgentBrandIcon(meta.agentId, 16)
                    Spacer(Modifier.width(8.dp))
                    Text(
                        meta.title,
                        Modifier.weight(1f),
                        fontSize = 13.sp,
                        color = Pa.TextPrimary,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                    Spacer(Modifier.width(8.dp))
                    SubAgentStatusDot(state)
                    Spacer(Modifier.width(5.dp))
                    Text(
                        SubAgentsLogic.statusLabel(state),
                        fontSize = 10.5.sp,
                        color = subAgentStateColor(state),
                    )
                }
                // 最后一条消息预览:≤2 行,超出部分渐隐进卡底(蓝本 §4.1 渐隐遮罩)。
                // 只在真溢出时叠加遮罩,短预览保持干净。
                if (!currentPreview.isNullOrBlank()) {
                    var overflowed by remember(currentPreview) { mutableStateOf(false) }
                    Box(Modifier.padding(top = 6.dp)) {
                        Text(
                            SubAgentsLogic.previewOf(currentPreview),
                            Modifier.fillMaxWidth(),
                            fontSize = 11.5.sp,
                            lineHeight = 15.sp,
                            color = Pa.TextSecondary,
                            maxLines = 2,
                            overflow = TextOverflow.Clip,
                            onTextLayout = { overflowed = it.hasVisualOverflow },
                        )
                        if (overflowed) {
                            Box(
                                Modifier
                                    .align(Alignment.BottomCenter)
                                    .fillMaxWidth()
                                    .height(12.dp)
                                    .background(
                                        Brush.verticalGradient(
                                            listOf(Pa.AccentDim.copy(alpha = 0f), Pa.AccentDim),
                                        ),
                                    ),
                            )
                        }
                    }
                }
                Spacer(Modifier.height(6.dp))
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text(
                        "${meta.messageCount} 条消息",
                        fontSize = 10.5.sp,
                        color = Pa.AccentText,
                    )
                    Spacer(Modifier.weight(1f))
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Text("查看对话", fontSize = 10.5.sp, color = Pa.AccentText)
                        Icon(
                            painterResource(AppR.drawable.ic_lucide_chevron_right), null,
                            tint = Pa.AccentText, modifier = Modifier.size(12.dp),
                        )
                    }
                }
            }
        }
    }
}

/**
 * 组卡(≥2 个子代理线程,蓝本 §4.2):头部计数行 + 迷你行列表,可折叠
 * (animateContentSize 弹性;运行中默认展开、全部终态默认折叠)。
 * 每行点击直接进该子对话,长按弹该行的快览。
 */
@Composable
fun SubAgentGroupCard(
    children: List<SubAgentThreadMeta>,
    liveRunIds: Set<String>,
    onOpen: (SubAgentThreadMeta) -> Unit,
    onPeek: (SubAgentThreadMeta) -> Unit,
    modifier: Modifier = Modifier,
) {
    val states = remember(children, liveRunIds) {
        children.map { SubAgentsLogic.runStateOf(it, liveRunIds) }
    }
    // 用户意图单独存,键只跟 children(P-3):运行集(liveRunIds)随父轮开始/结束
    // 频繁变化,不得重置用户手动折叠/展开;null = 用户未表态,落 groupIsActive
    // 默认值(运行中默认展开、全部终态默认折叠)。
    var userExpanded by remember(children) { mutableStateOf<Boolean?>(null) }
    val expanded = SubAgentsLogic.expandedOf(userExpanded, states)
    val hf = LocalHapticFeedback.current
    Column(
        modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(12.dp))
            .background(Pa.AccentDim)
            // 同单卡:实色底 + 发丝高光边(材质家族统一)
            .border(0.5.dp, PaGlassToken.Highlight, RoundedCornerShape(12.dp))
            .animateContentSize(
                spring(dampingRatio = 0.8f, stiffness = Spring.StiffnessMediumLow),
            ),
    ) {
        // 头部:整卡可点 = 折叠/展开切换(写用户意图,展开态由 expandedOf 推导)
        Row(
            Modifier
                .fillMaxWidth()
                .pressable(haptic = true) { userExpanded = !expanded }
                .padding(horizontal = 12.dp, vertical = 10.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Box(Modifier.size(3.dp, 14.dp).background(Pa.Accent, RoundedCornerShape(2.dp)))
            Spacer(Modifier.width(8.dp))
            Text(
                "${children.size} 个子智能体在协同",
                Modifier.weight(1f),
                fontSize = 13.sp,
                color = Pa.TextPrimary,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            Text(
                SubAgentsLogic.groupSummaryLine(states),
                fontSize = 10.5.sp,
                color = Pa.TextSecondary,
            )
            Spacer(Modifier.width(6.dp))
            Icon(
                painterResource(
                    if (expanded) AppR.drawable.ic_lucide_chevron_up
                    else AppR.drawable.ic_lucide_chevron_down,
                ),
                null, tint = Pa.TextSecondary, modifier = Modifier.size(14.dp),
            )
        }
        if (expanded) {
            children.forEachIndexed { idx, meta ->
                val st = states[idx]
                Row(
                    Modifier
                        .fillMaxWidth()
                        .pressableLong(
                            onClick = {
                                hf.performHapticFeedback(HapticFeedbackType.TextHandleMove)
                                onOpen(meta)
                            },
                            onLongPress = {
                                hf.performHapticFeedback(HapticFeedbackType.TextHandleMove)
                                onPeek(meta)
                            },
                        )
                        .padding(horizontal = 12.dp, vertical = 7.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    SubAgentBrandIcon(meta.agentId, 13)
                    Spacer(Modifier.width(8.dp))
                    Text(
                        meta.title,
                        Modifier.weight(1f),
                        fontSize = 12.sp,
                        color = Pa.TextPrimary,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                    SubAgentStatusDot(st, size = 6)
                    Spacer(Modifier.width(5.dp))
                    Text(
                        SubAgentsLogic.statusLabel(st),
                        fontSize = 10.sp,
                        color = subAgentStateColor(st),
                    )
                }
            }
            Spacer(Modifier.height(4.dp))
        }
    }
}

/**
 * 子对话页头部来源行("来自主对话 · 来源" + 兄弟计数/切换钮,蓝本 §5.1/§5.2)。
 * 兄弟切到 1 个或不足时对应钮禁用;pos==0(当前不在组里,理论不可达)不显示计数。
 */
@Composable
fun SubAgentSourceLine(
    parentTitle: String,
    pos: Int,
    total: Int,
    onPrev: () -> Unit,
    onNext: () -> Unit,
    /**
     * 返回主对话（规范 §18.3）。**null = 父会话不可达**（已删除 / 关系未知），
     * 此时控件置灰 —— 不给点了没反应的假入口。
     */
    onBackToParent: (() -> Unit)? = null,
    modifier: Modifier = Modifier,
) {
    Row(
        modifier.fillMaxWidth().padding(horizontal = 14.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        // 【显式返回主对话】以前只能靠系统返回箭头:子对话页一深（或从通知直接
        // 进来）就找不到回去的路。给一个带文案的显式入口,而不是只有一个小图标。
        Row(
            Modifier
                .clip(RoundedCornerShape(8.dp))
                .pressable(enabled = onBackToParent != null) { onBackToParent?.invoke() }
                .padding(horizontal = 5.dp, vertical = 3.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Icon(
                painterResource(AppR.drawable.ic_lucide_chevron_left), "返回主对话",
                tint = if (onBackToParent != null) Pa.AccentText else Pa.TextTertiary,
                modifier = Modifier.size(13.dp),
            )
            Text(
                "主对话",
                fontSize = 11.5.sp,
                color = if (onBackToParent != null) Pa.AccentText else Pa.TextTertiary,
            )
        }
        Spacer(Modifier.width(6.dp))
        Icon(
            painterResource(AppR.drawable.ic_lucide_message_square), null,
            tint = Pa.TextSecondary, modifier = Modifier.size(11.dp),
        )
        Spacer(Modifier.width(5.dp))
        Text(
            if (parentTitle.isBlank()) "来自主对话" else "来自主对话 · $parentTitle",
            Modifier.weight(1f),
            fontSize = 11.5.sp,
            color = Pa.TextSecondary,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
        if (pos > 0 && total > 1) {
            SubAgentSiblingSwitch(
                pos = pos,
                total = total,
                onPrev = onPrev,
                onNext = onNext,
            )
        }
    }
}

/** "◀ n/N ▶" 兄弟切换钮(蓝本 §5.2 头部计数 chip 的降级钮实现)。 */
@Composable
fun SubAgentSiblingSwitch(
    pos: Int,
    total: Int,
    onPrev: () -> Unit,
    onNext: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Row(
        modifier
            .clip(RoundedCornerShape(10.dp))
            .background(Pa.Card)
            .padding(horizontal = 2.dp, vertical = 2.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Icon(
            painterResource(AppR.drawable.ic_lucide_chevron_left), "上一个子对话",
            tint = if (pos > 1) Pa.AccentText else Pa.TextTertiary,
            modifier = Modifier
                .size(20.dp)
                .pressable(enabled = pos > 1) { onPrev() },
        )
        Text(
            "$pos/$total",
            fontSize = 10.5.sp,
            color = Pa.TextSecondary,
            textAlign = TextAlign.Center,
            modifier = Modifier.width(26.dp),
        )
        Icon(
            painterResource(AppR.drawable.ic_lucide_chevron_right), "下一个子对话",
            tint = if (pos < total) Pa.AccentText else Pa.TextTertiary,
            modifier = Modifier
                .size(20.dp)
                .pressable(enabled = pos < total) { onNext() },
        )
    }
}

/**
 * 长按快览弹层(蓝本 §4.3):该子线程最近 5 条消息快照 + "打开完整对话"。
 * 定位是"不打断主对话浏览的偷看":sheet 收起不影响任何状态;消息拉取失败
 * 显示空态(不伪造对话)。
 */
@Composable
fun SubAgentPeekSheet(
    meta: SubAgentThreadMeta,
    state: SubAgentRunState,
    onOpenFull: () -> Unit,
    onDismiss: () -> Unit,
) {
    var messages by remember(meta.threadId) { mutableStateOf<List<ChatItem>>(emptyList()) }
    var loading by remember(meta.threadId) { mutableStateOf(true) }
    LaunchedEffect(meta.threadId) {
        try {
            val resp = withContext(Dispatchers.IO) {
                AgentConnection.threadService().getThreadHistory(
                    GetThreadHistoryRequest.newBuilder()
                        .setThreadId(meta.threadId)
                        // daemon 语义:limit 尾截 —— 只要最近若干条
                        .setLimit(SubAgentsState.PEEK_HISTORY_LIMIT)
                        .build(),
                )
            }
            messages = historyToChatItems(resp.messagesList)
                .filter { it.text.isNotBlank() }
                .takeLast(5)
        } catch (_: Exception) {
            // 拉不到 = 空态,不伪造
        }
        loading = false
    }
    // 【本轮换 PaSheet】统一玻璃浮层:82% 高度上限 + 滚动 + 导航栏安全区由封装内建,
    // 原先手写的 verticalScroll / navigationBarsPadding / imePadding 全部交给封装。
    PaSheet(onDismissRequest = onDismiss) {
        Column(
            Modifier
                .fillMaxWidth()
                .padding(horizontal = 20.dp),
        ) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                SubAgentBrandIcon(meta.agentId, 16)
                Spacer(Modifier.width(8.dp))
                Text(
                    meta.title,
                    Modifier.weight(1f),
                    fontSize = 15.sp,
                    color = Pa.TextPrimary,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
                SubAgentStatusDot(state)
                Spacer(Modifier.width(5.dp))
                Text(
                    SubAgentsLogic.statusLabel(state),
                    fontSize = 10.5.sp,
                    color = subAgentStateColor(state),
                )
            }
            Text(
                "最近消息",
                Modifier.padding(top = 10.dp, bottom = 4.dp),
                fontSize = 10.5.sp,
                color = Pa.TextTertiary,
            )
            when {
                loading -> {
                    CircularProgressIndicator(
                        Modifier.padding(vertical = 18.dp).size(16.dp),
                        strokeWidth = 2.dp,
                    )
                }
                messages.isEmpty() -> {
                    Text(
                        "暂无消息记录",
                        Modifier.padding(vertical = 18.dp),
                        fontSize = 12.sp,
                        color = Pa.TextTertiary,
                    )
                }
                else -> {
                    messages.forEach { item ->
                        Row(Modifier.padding(vertical = 5.dp)) {
                            Text(
                                subAgentRoleLabel(item.role),
                                fontSize = 10.sp,
                                color = Pa.AccentText,
                                modifier = Modifier.width(34.dp),
                            )
                            Text(
                                item.text,
                                Modifier.weight(1f),
                                fontSize = 12.sp,
                                color = Pa.TextSecondary,
                                maxLines = 3,
                                overflow = TextOverflow.Ellipsis,
                            )
                        }
                    }
                }
            }
            Button(
                onClick = onOpenFull,
                Modifier
                    .fillMaxWidth()
                    .padding(top = 14.dp, bottom = 18.dp),
                colors = ButtonDefaults.buttonColors(containerColor = Pa.Accent),
                shape = RoundedCornerShape(14.dp),
            ) {
                Text("打开完整对话", color = Color.White, fontSize = 13.sp)
            }
        }
    }
}
