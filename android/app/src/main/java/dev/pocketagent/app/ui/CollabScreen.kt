package dev.pocketagent.app.ui

import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.MutableTransitionState
import androidx.compose.animation.expandVertically
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.scaleIn
import androidx.compose.animation.shrinkVertically
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.platform.LocalView
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.AgentConnection
import dev.pocketagent.app.R
import dev.pocketagent.app.ui.anim.EmptyState
import dev.pocketagent.app.ui.anim.PaMotion
import dev.pocketagent.app.ui.anim.pressable
import dev.pocketagent.app.ui.component.PaHaptics
import dev.pocketagent.app.ui.component.PaNavIconButton
import dev.pocketagent.app.ui.component.PaScreenScaffold
import dev.pocketagent.app.ui.component.PaSheet
import dev.pocketagent.app.ui.component.PaStatusBadge
import dev.pocketagent.app.ui.theme.EditorialTone
import dev.pocketagent.app.ui.theme.PaEditorial
import dev.pocketagent.app.ui.theme.editorial
import dev.pocketagent.app.ui.theme.paPushBack

/**
 * 协同页(设计稿 10 · 编辑式):一条指令扇出给多个对等 agent 并行协作。
 *
 * 【为什么是独立页、且视觉与普通会话明显不同】
 * 普通会话是"手机 ↔ 单一 agent"的 1:1 对话流(ChatScreen);协同会话是
 * "一条指令 + N 个对等 agent 各自回报"的编排台。所以这里:
 *  1. 顶部有**石墨重点面**的说明卡(README:有明确主次关系的内容进面板)+ 分支图标;
 *  2. 下方是**按 agent 分行**的进度卡(每人一卡:身份 + 四词状态徽章 + 卡上入口),
 *     不是混在一起的对话流;
 *  3. 共享一条指令输入,结果按成员分别回报。
 *
 * 【并行对等,不是父子委派】这里没有任何"父 agent 派发子智能体"的概念;
 * 被选中的 agent 是平级协作者,各自收到同一条指令独立执行。参与者列出 daemon 报的
 * 全部协同 agent(CollabAware 门控):可用的可勾选,不可用的灰显并写明原因 ——
 * 只显示可选项,等于把"为什么不让我选它"藏进裁剪规则(结论 1)。一位失败不拖垮其余。
 *
 * [onOpenThread] 为 null 时不摆「查看会话」入口(没有导航通道就没有假入口)。
 */
@Composable
fun CollabScreen(
    onBack: () -> Unit,
    onOpenCollabRoles: () -> Unit,
    onOpenThread: ((String) -> Unit)? = null,
) {
    val ed = MaterialTheme.editorial
    val caps = AgentConnection.hostCapabilities.collectAsState().value
    val agents = remember(caps) { buildAgents(caps) }
    val collabIds = remember(caps) { resolveCollabAgentIds(caps) }
    val disabled = AgentConnection.disabledAgents
    // 参与者全集(可用 + 不可用带原因);能勾选的只是其中 READY 的投影(结论 1)。
    val participants = remember(agents, collabIds, disabled) {
        collabParticipants(agents, collabIds, disabled)
    }
    val readyIds = remember(participants) {
        participants.filter { it.state == CollabParticipantState.READY }.map { it.agent.id }.toSet()
    }
    val unavailableNote = remember(participants) { collabUnavailableNote(participants) }

    // 进入即拉一次角色列表(给"为 agent 配角色"用)
    LaunchedEffect(collabIds) { if (collabIds.isNotEmpty()) CollabRolesState.load() }
    // agent 掉线/被停用后剪除失效选中项:派发判定按剪除后的集合,"已选 N 位"才与
    // 实际能派发的人数一致(归并纯函数 pruneCollabSelection,单测钉死)。
    LaunchedEffect(readyIds) { CollabState.retainSelection(readyIds) }

    val selected = CollabState.selected
    val roleBinding = CollabState.roleBinding
    val phases = CollabState.phases
    val memberErrors = CollabState.memberErrors
    val dispatchError = CollabState.dispatchError
    val dispatching = CollabState.dispatching
    val lastInstruction = CollabState.lastInstruction

    var cmd by remember { mutableStateOf("") }
    var roleFor by remember { mutableStateOf<String?>(null) }

    // 已选中的 agent(从可用清单里挑,保持顺序稳定)
    val selectedAgents = participants
        .filter { it.state == CollabParticipantState.READY && it.agent.id in selected }
        .map { it.agent }
    // 派发可行性与禁用原因同源(结论 1/3):按钮灰 = 旁边一定有原因。
    val decision = collabDispatchDecision(
        selectedCount = selectedAgents.size,
        instructionReady = cmd.isNotBlank(),
        dispatching = dispatching,
    )

    Box(
        Modifier
            .fillMaxSize()
            .background(ed.Canvas)
            .paPushBack(roleFor != null),
    ) {
        PaScreenScaffold(
            title = "多 Agent 协同",
            leading = {
                PaNavIconButton(
                    onClick = onBack,
                    contentDescription = "返回",
                    iconRes = R.drawable.ic_lucide_chevron_left,
                    plain = true,
                )
            },
            // 唯一次操作:角色管理(设计稿 10 右上齿轮;弹层里的"管理协同角色"入口保留)
            trailing = {
                PaNavIconButton(
                    onClick = onOpenCollabRoles,
                    contentDescription = "协同角色设置",
                    iconRes = R.drawable.ic_lucide_settings,
                )
            },
        ) {
            LazyColumn(
                Modifier.weight(1f).fillMaxWidth(),
                contentPadding = androidx.compose.foundation.layout.PaddingValues(top = 4.dp, bottom = 16.dp),
            ) {
                // ---- 说明卡(石墨重点面):协同身份,一眼可辨 ----
                item {
                    Column(
                        Modifier
                            .fillMaxWidth()
                            .background(ed.Graphite, RoundedCornerShape(PaEditorial.Radius.Card))
                            .padding(16.dp),
                    ) {
                        Box(
                            Modifier
                                .size(PaEditorial.Space.IconBox)
                                .background(PaEditorial.OnGraphiteHairline, RoundedCornerShape(13.dp)),
                            contentAlignment = Alignment.Center,
                        ) {
                            Icon(
                                painterResource(R.drawable.ic_lucide_git_branch), null,
                                tint = PaEditorial.OnGraphite, modifier = Modifier.size(20.dp),
                            )
                        }
                        Spacer(Modifier.height(12.dp))
                        Text(
                            "一条任务,多个视角。",
                            style = PaEditorial.Type.Heading,
                            color = PaEditorial.OnGraphite,
                        )
                        Spacer(Modifier.height(4.dp))
                        Text(
                            "把同一指令并行交给选中的 Agent;每位参与者分别回报,不混为一条对话。",
                            fontSize = 13.sp, lineHeight = 21.sp,
                            color = PaEditorial.OnGraphiteMuted,
                        )
                    }
                }

                // ---- 参与协同(多选;可用可勾,不可用灰显带原因 —— 结论 1) ----
                item {
                    Row(
                        Modifier.fillMaxWidth().padding(top = 16.dp, bottom = 8.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Text("参与协同", style = PaEditorial.Type.Heading, color = ed.Ink)
                        Spacer(Modifier.weight(1f))
                        // 选中数明确(结论 1):0 人也如实说,不藏数字。
                        Text(
                            collabSelectionSummary(selectedAgents.size),
                            fontSize = 12.sp, color = ed.Muted,
                        )
                    }
                    Text(
                        "电脑端接入协同的 agent 都在这里;不可用的标了原因",
                        fontSize = 12.sp, lineHeight = 17.sp, color = ed.Subtle,
                    )
                    Spacer(Modifier.height(8.dp))
                    if (participants.isEmpty()) {
                        // 诚实空态:daemon 没报任何可协同 agent,不显示假入口
                        EmptyState(
                            R.drawable.ic_lucide_git_branch,
                            "暂无可协同的 agent",
                            "电脑端启用协作能力后,这里会列出可选项",
                        )
                    } else {
                        Row(
                            Modifier.horizontalScroll(rememberScrollState()),
                            horizontalArrangement = Arrangement.spacedBy(8.dp),
                        ) {
                            participants.forEach { p ->
                                CollabAgentChip(
                                    a = p.agent,
                                    state = p.state,
                                    on = p.agent.id in selected,
                                    onClick = { CollabState.toggle(p.agent.id) },
                                )
                            }
                        }
                    }
                    // 不可用参与者的原因汇总(结论 1:"不可用及原因");全员可用不占位。
                    unavailableNote?.let {
                        Text(
                            it,
                            Modifier.padding(top = 8.dp),
                            fontSize = 12.sp, lineHeight = 17.sp, color = ed.Subtle,
                            maxLines = 2, overflow = TextOverflow.Ellipsis,
                        )
                    }
                    if (selectedAgents.isEmpty() && participants.isNotEmpty()) {
                        Text(
                            CollabCopy.EMPTY_NO_SELECTION,
                            Modifier.padding(top = 8.dp),
                            fontSize = 12.sp, lineHeight = 17.sp, color = ed.Subtle,
                        )
                    }
                }

                // ---- 分工与状态:逐 agent 一张进度卡(身份 + 四词徽章 + 卡上入口) ----
                item {
                    Row(
                        Modifier.fillMaxWidth().padding(top = 16.dp, bottom = 4.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Text("分工与状态", style = PaEditorial.Type.Heading, color = ed.Ink)
                        Spacer(Modifier.weight(1f))
                        Row(
                            Modifier
                                .pressable { onOpenCollabRoles() }
                                .padding(horizontal = 4.dp, vertical = 10.dp),
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            Text("配置角色", fontSize = 13.sp, color = PaEditorial.Primary)
                            Spacer(Modifier.width(2.dp))
                            Icon(
                                painterResource(R.drawable.ic_lucide_chevron_right), null,
                                tint = PaEditorial.Primary, modifier = Modifier.size(14.dp),
                            )
                        }
                    }
                }

                if (selectedAgents.isEmpty()) {
                    item {
                        Text(
                            "先在上面勾选这次一起干活的 agent,他们的执行进度会逐个出现在这里",
                            Modifier.padding(vertical = 8.dp),
                            fontSize = 12.sp, lineHeight = 17.sp, color = ed.Subtle,
                        )
                    }
                } else {
                    items(selectedAgents, key = { it.id }) { agent ->
                        // 进度卡进场 spring(结论 6):勾选瞬间卡片从收起状态浮入,
                        // 可打断;同卡内的相位流转不重播进场(remember 记住入场态)。
                        val enter = remember { MutableTransitionState(false).apply { targetState = true } }
                        AnimatedVisibility(
                            visibleState = enter,
                            enter = fadeIn(PaMotion.emphasized()) + expandVertically(PaMotion.emphasized()),
                            exit = shrinkVertically(PaMotion.standard()) + fadeOut(PaMotion.standard()),
                        ) {
                            Column {
                                val roleId = roleBinding[agent.id]
                                val roleName = roleId?.let { rid ->
                                    CollabRolesState.roles.firstOrNull { r -> r.id == rid }?.let { collabRoleDisplayName(it) }
                                }
                                val phase = phases[agent.id] ?: CollabAgentPhase.IDLE
                                val threadId = CollabState.threadIds[agent.id].orEmpty()
                                CollabAgentRow(
                                    agent = agent,
                                    roleName = roleName,
                                    phase = phase,
                                    memberError = memberErrors[agent.id],
                                    threadId = threadId,
                                    onOpenThread = onOpenThread,
                                    onConfigRole = { roleFor = agent.id },
                                    onRedispatch = { CollabState.redispatchMember(agent.id) },
                                )
                                HorizontalHairlineEd()
                            }
                        }
                    }
                }

                // ---- 整体派发失败(连 RPC 都没通):如实给原因,不粉饰 ----
                dispatchError?.let { err ->
                    item {
                        Text(
                            err,
                            Modifier
                                .fillMaxWidth()
                                .padding(top = 12.dp)
                                .background(ed.DeniedContainer, RoundedCornerShape(13.dp))
                                .padding(12.dp),
                            fontSize = 12.sp, lineHeight = 17.sp,
                            color = EditorialTone.DENIED.text,
                        )
                    }
                }

                // ---- 已发指令回显 + 诚实等待说明(扁平行,不做底色块) ----
                if (lastInstruction != null) {
                    item {
                        Column(Modifier.padding(top = 12.dp)) {
                            Row(
                                Modifier.fillMaxWidth().padding(vertical = 4.dp),
                                verticalAlignment = Alignment.CenterVertically,
                            ) {
                                Icon(
                                    painterResource(R.drawable.ic_lucide_git_branch), null,
                                    tint = PaEditorial.Primary, modifier = Modifier.size(16.dp),
                                )
                                Spacer(Modifier.width(8.dp))
                                Text(
                                    lastInstruction!!,
                                    fontSize = 13.sp, lineHeight = 21.sp, color = ed.Ink,
                                )
                            }
                            Spacer(Modifier.height(4.dp))
                            Text(
                                CollabCopy.WAITING_DAEMON,
                                fontSize = 11.sp, lineHeight = 16.sp, color = ed.Subtle,
                            )
                        }
                    }
                }
            }

            // ---- 底部:共同指令输入 + 派发(主操作按钮走石墨重点面白字,
            //      与审批弹窗"仅本次允许"同一形态;主色只给"当前操作"类元素) ----
            // 派发按钮的禁用原因(结论 1/3):0 人/指令空白/提交中,灰着必有原因,
            // 原因与按钮同源(collabDispatchDecision)。可派发时不占一行。
            decision.disabledReason?.let {
                Text(
                    it,
                    Modifier.padding(start = 4.dp, bottom = 4.dp),
                    fontSize = 11.sp, lineHeight = 16.sp, color = ed.Subtle,
                )
            }
            val hapticView = LocalView.current
            Row(
                Modifier
                    .fillMaxWidth()
                    .padding(bottom = 8.dp),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
                verticalAlignment = Alignment.Bottom,
            ) {
                // 共同指令输入区(结论 3):多行生长,上限五行后内部滚动,
                // 长任务描述不必挤在一行里横着跑。
                Row(
                    Modifier
                        .weight(1f)
                        .heightIn(min = PaEditorial.Space.TouchMin, max = 120.dp)
                        .background(ed.Surface, RoundedCornerShape(13.dp))
                        .border(1.dp, ed.Hairline, RoundedCornerShape(13.dp))
                        .padding(horizontal = 12.dp, vertical = 12.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Box(Modifier.weight(1f)) {
                        if (cmd.isEmpty()) {
                            Text(
                                "描述需要协同完成的任务…",
                                fontSize = 14.sp, lineHeight = 21.sp, color = ed.Subtle,
                                maxLines = 1, overflow = TextOverflow.Ellipsis,
                            )
                        }
                        BasicTextField(
                            value = cmd,
                            onValueChange = { cmd = it },
                            modifier = Modifier.fillMaxWidth(),
                            textStyle = TextStyle(fontSize = 14.sp, lineHeight = 21.sp, color = ed.Ink),
                            cursorBrush = SolidColor(PaEditorial.Primary),
                        )
                    }
                }
                // 派发按钮(当前操作):可用 = 石墨底白字;不可用 = 抬升面灰字,不可点。
                // 触感 = confirm(结论 6:动作确认档);防重复提交双保险 ——
                // 按钮禁用(第一道)+ CollabState.dispatching 硬闸(第二道,挡 IME/连点)。
                Row(
                    Modifier
                        .heightIn(min = PaEditorial.Space.TouchMin)
                        .pressable(enabled = decision.canDispatch) {
                            PaHaptics.confirm(hapticView)
                            CollabState.dispatch(cmd)
                            cmd = ""
                        }
                        .background(
                            if (decision.canDispatch) ed.Graphite else ed.Raised,
                            RoundedCornerShape(13.dp),
                        )
                        .padding(horizontal = 16.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(
                        if (dispatching) CollabCopy.BTN_DISPATCHING else CollabCopy.BTN_DISPATCH,
                        fontSize = 14.sp,
                        fontWeight = FontWeight(650),
                        color = if (decision.canDispatch) PaEditorial.OnGraphite else ed.Subtle,
                    )
                }
            }
        }
    }

    // ---- 角色选择底部弹层(复用既有 CollabRolesState 的角色数据)----
    roleFor?.let { agentId ->
        CollabRolePickerSheet(
            agentId = agentId,
            currentRoleId = roleBinding[agentId],
            onPick = { rid -> CollabState.bindRole(agentId, rid); roleFor = null },
            onManage = { roleFor = null; onOpenCollabRoles() },
            onDismiss = { roleFor = null },
        )
    }
}

/** 细分隔线(协同页逐 agent 行之间)。 */
@Composable
private fun HorizontalHairlineEd() {
    Box(
        Modifier
            .fillMaxWidth()
            .padding(vertical = 2.dp)
            .height(1.dp)
            .background(MaterialTheme.editorial.Hairline),
    )
}

/**
 * 单个 agent 多选芯片(设计稿 10):选中 = 薄荷浅底 + 绿描边(运行语义色,不是
 * 装饰 —— "已加入这次协同"就是这台手机的当前操作对象);未选中 = 瓷白 + 细线;
 * 不可用 = 图标灰阶 + 状态词(结论 1:可用/不可用一眼可辨,原因汇总在芯片下方)。
 */
@Composable
private fun CollabAgentChip(
    a: AgentInfo,
    state: CollabParticipantState,
    on: Boolean,
    onClick: () -> Unit,
) {
    val ed = MaterialTheme.editorial
    val ready = state == CollabParticipantState.READY
    val borderColor = if (on && ready) EditorialTone.RUNNING.accent else ed.Hairline
    // 状态词:已选 > 不可用词(OFFLINE 用首页 agent 网格同源的"电脑端未安装/接入中",
    // 本地停用说"已停用") > 不摆。READY 的未选中芯片不摆词,保持安静。
    val stateWord = when {
        on && ready -> "已选"
        state == CollabParticipantState.LOCALLY_DISABLED -> "已停用"
        state == CollabParticipantState.OFFLINE -> unavailableLabel(a.id)
        else -> null
    }
    val stateWordColor = when {
        on && ready -> EditorialTone.RUNNING.text
        else -> ed.Muted
    }
    Column(
        Modifier
            .pressable(enabled = ready) { onClick() }
            .width(84.dp)
            .background(if (on && ready) ed.RunningContainer else ed.Surface, RoundedCornerShape(PaEditorial.Radius.Card))
            .border(1.dp, borderColor, RoundedCornerShape(PaEditorial.Radius.Card))
            .padding(12.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Image(
            bitmap = agentBitmap(a.iconRes),
            contentDescription = a.name,
            modifier = Modifier.size(40.dp),
            contentScale = androidx.compose.ui.layout.ContentScale.Crop,
            // 色通道之外的降饱和:不可用磁贴与首页 agent 网格同一处理(grayscale)。
            colorFilter = if (ready) null else grayscale(),
        )
        Spacer(Modifier.height(6.dp))
        Text(
            a.name,
            fontSize = 12.sp, lineHeight = 17.sp,
            color = if (on && ready) ed.Ink else ed.Muted,
            maxLines = 1,
        )
        if (stateWord != null) {
            Spacer(Modifier.height(2.dp))
            Text(
                stateWord,
                fontSize = 10.sp, lineHeight = 14.sp,
                color = stateWordColor,
                maxLines = 1, overflow = TextOverflow.Ellipsis,
            )
        }
    }
}

/**
 * 逐 agent 进度卡(设计稿 10「分工与状态」):身份图 + 名称 + 角色行 + 四词状态徽章;
 * 一位失败不拖垮其余 —— 相位与失败原因都按成员独立展示。
 *
 * 【状态三通道(结论 2/4)】徽章 = 词(四词体系同源 PaStatusBadge)+ 色档 + 形
 * (环/点/勾/叹号,PaStatusGlyph);徽章位置的状态流转走 spring 淡入淡出(结论 6),
 * 相位变化时徽章从 0.85 缩放浮入,不硬跳。
 *
 * 【卡上入口(结论 2)】
 *  - 受理失败:「查看原因」展开 daemon 原文全文 + 「重派该 Agent」单发这一家;
 *  - 已拿到子线程:「查看会话」单独打开该成员的会话(onOpenThread 缺席时不摆 ——
 *    没有导航通道就没有假入口)。
 */
@Composable
private fun CollabAgentRow(
    agent: AgentInfo,
    roleName: String?,
    phase: CollabAgentPhase,
    memberError: String?,
    threadId: String,
    onOpenThread: ((String) -> Unit)?,
    onConfigRole: () -> Unit,
    onRedispatch: () -> Unit,
) {
    val ed = MaterialTheme.editorial
    // 相位 → 徽章/说明/入口的唯一映射(纯函数 collabProgressView,单测钉死)。
    val progress = collabProgressView(phase, hasThreadId = threadId.isNotBlank())
    var reasonOpen by remember { mutableStateOf(false) }
    Column(
        Modifier
            .fillMaxWidth()
            .padding(vertical = 12.dp),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Image(
                bitmap = agentBitmap(agent.iconRes),
                contentDescription = agent.name,
                modifier = Modifier
                    .size(PaEditorial.Space.IconBox)
                    .clip(RoundedCornerShape(13.dp)),
                contentScale = androidx.compose.ui.layout.ContentScale.Crop,
            )
            Spacer(Modifier.width(12.dp))
            Column(Modifier.weight(1f)) {
                Text(
                    agent.name,
                    fontSize = 15.sp, lineHeight = 21.sp,
                    fontWeight = FontWeight(650), color = ed.Ink,
                    maxLines = 1, overflow = TextOverflow.Ellipsis,
                )
                if (roleName != null) {
                    Text(
                        "角色 · $roleName",
                        Modifier.padding(top = 1.dp),
                        fontSize = 12.sp, lineHeight = 17.sp, color = ed.Muted,
                        maxLines = 1, overflow = TextOverflow.Ellipsis,
                    )
                }
            }
            // 四词徽章的状态流转 spring(结论 6):相位一变,新徽章淡入+轻缩放浮入,
            // 旧徽章淡出让位;spring 可打断,连点勾选/派发不会跳帧。
            AnimatedContent(
                targetState = phase,
                transitionSpec = {
                    (fadeIn(PaMotion.standard()) + scaleIn(initialScale = 0.85f, animationSpec = PaMotion.standard()))
                        .togetherWith(fadeOut(PaMotion.standard()))
                },
                label = "collabPhaseBadge",
            ) { ph ->
                val status = collabPhaseStatus(ph)
                if (status != null) {
                    PaStatusBadge(status)
                } else {
                    Text(
                        collabPhaseLabel(ph),
                        fontSize = 11.sp, lineHeight = 16.sp, color = ed.Subtle,
                    )
                }
            }
        }
        // 徽章四词之外的一句补充(ACK/RUNNING 专有):"到哪一步了"由纯函数给,
        // IDLE/DONE/ERROR 不叠字(IDLE 有右侧文案,DONE 徽章够,ERROR 有原因块)。
        progress.detailLine?.let {
            Text(
                it,
                Modifier.padding(top = 4.dp),
                fontSize = 11.sp, lineHeight = 16.sp, color = ed.Subtle,
            )
        }
        // 未完成成员的原因(daemon 人话原文,按成员各自展示;不改全局):
        // 收起时两行省略 + 「查看原因」展开全文 —— 截断不给真相,展开才完整。
        if (phase == CollabAgentPhase.ERROR && !memberError.isNullOrBlank()) {
            Column(
                Modifier
                    .fillMaxWidth()
                    .padding(top = 8.dp)
                    .background(ed.DeniedContainer, RoundedCornerShape(13.dp))
                    .padding(horizontal = 12.dp, vertical = 8.dp),
            ) {
                Text(
                    memberError,
                    Modifier.fillMaxWidth(),
                    fontSize = 12.sp, lineHeight = 17.sp,
                    color = EditorialTone.DENIED.text,
                    maxLines = if (reasonOpen) Int.MAX_VALUE else 2,
                    overflow = TextOverflow.Ellipsis,
                )
                Text(
                    if (reasonOpen) CollabCopy.HIDE_REASON else CollabCopy.VIEW_REASON,
                    Modifier
                        .pressable { reasonOpen = !reasonOpen }
                        .padding(top = 4.dp, bottom = 2.dp),
                    fontSize = 12.sp, lineHeight = 17.sp,
                    fontWeight = FontWeight(650),
                    color = EditorialTone.DENIED.text,
                )
            }
        }
        // 卡上动作行(扁平行,不套容器;结论 2 的失败入口 + 成员会话入口)。
        val showActions = progress.canRedispatch || (progress.canOpenSession && onOpenThread != null)
        if (showActions) {
            Row(
                Modifier.fillMaxWidth().padding(top = 6.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                if (progress.canRedispatch) {
                    Row(
                        Modifier
                            .pressable { onRedispatch() }
                            .padding(horizontal = 4.dp, vertical = 6.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Icon(
                            painterResource(R.drawable.ic_lucide_refresh_cw), null,
                            tint = PaEditorial.Primary, modifier = Modifier.size(14.dp),
                        )
                        Spacer(Modifier.width(4.dp))
                        Text(CollabCopy.REDISPATCH_MEMBER, fontSize = 12.sp, color = PaEditorial.Primary)
                    }
                }
                if (progress.canOpenSession && onOpenThread != null) {
                    Row(
                        Modifier
                            .pressable { onOpenThread(threadId) }
                            .padding(horizontal = 4.dp, vertical = 6.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Icon(
                            painterResource(R.drawable.ic_lucide_chevron_right), null,
                            tint = PaEditorial.Primary, modifier = Modifier.size(14.dp),
                        )
                        Spacer(Modifier.width(4.dp))
                        Text(CollabCopy.OPEN_SESSION, fontSize = 12.sp, color = PaEditorial.Primary)
                    }
                }
            }
        }
        // 角色配置行(可点,复用既有角色数据);扁平行,不做底色子容器
        Row(
            Modifier
                .fillMaxWidth()
                .pressable { onConfigRole() }
                .padding(vertical = 10.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Icon(
                painterResource(R.drawable.ic_lucide_sliders), null,
                tint = PaEditorial.Primary, modifier = Modifier.size(16.dp),
            )
            Spacer(Modifier.width(8.dp))
            Text(
                if (roleName != null) "角色:$roleName" else CollabCopy.ROLE_NONE,
                fontSize = 12.sp, lineHeight = 17.sp, color = ed.Muted,
            )
            Spacer(Modifier.weight(1f))
            Icon(
                painterResource(R.drawable.ic_lucide_chevron_right), null,
                tint = ed.Subtle, modifier = Modifier.size(16.dp),
            )
        }
    }
}

/**
 * 角色选择底部弹层:列出该 agent 的协同角色(复用 CollabRolesState 既有数据),
 * 可解绑(未配置角色),也可跳去「协同角色」页做完整管理(复用 CollabRolesScreen)。
 *
 * 【PaSheet 统一浮层】玻璃底 + 82% 屏高上限 + 内建滚动;被覆盖内容的推背模糊由
 * CollabScreen 根上的 paPushBack 负责。选项行用瓷白卡 + 细线(编辑式弹层内容)。
 */
@Composable
private fun CollabRolePickerSheet(
    agentId: String,
    currentRoleId: String?,
    onPick: (String?) -> Unit,
    onManage: () -> Unit,
    onDismiss: () -> Unit,
) {
    val ed = MaterialTheme.editorial
    val roles = CollabRolesState.roles.filter { it.agentId == agentId }
    PaSheet(onDismissRequest = onDismiss) {
        Row(Modifier.fillMaxWidth().padding(horizontal = 20.dp)) {
            Column(Modifier.weight(1f)) {
                Text("为 ${selectedAgentLabel(agentId)} 选角色", fontSize = 17.sp, color = ed.Ink)
                Text(
                    "同一 agent 可绑定不同岗位设定",
                    fontSize = 12.sp, lineHeight = 17.sp, color = ed.Muted,
                    modifier = Modifier.padding(top = 2.dp),
                )
            }
            PaNavIconButton(
                onClick = onDismiss,
                contentDescription = "关闭",
                iconRes = R.drawable.ic_lucide_x,
                plain = true,
            )
        }
        Spacer(Modifier.height(12.dp))
        Column(
            Modifier.fillMaxWidth().padding(horizontal = 16.dp),
            verticalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            // 未配置角色(解绑)
            RolePickRow("未配置角色", "按 agent 默认行为执行", selected = currentRoleId == null, onClick = { onPick(null) })
            roles.forEach { role ->
                RolePickRow(
                    title = collabRoleDisplayName(role),
                    desc = collabRoleSummaryLine(role),
                    selected = currentRoleId == role.id,
                    onClick = { onPick(role.id) },
                )
            }
            if (roles.isEmpty()) {
                Text(
                    "该 agent 还没有专属角色,去「协同角色」里新建",
                    fontSize = 12.sp, lineHeight = 17.sp, color = ed.Subtle,
                    modifier = Modifier.padding(vertical = 6.dp),
                )
            }
            // 管理角色入口(复用现有 CollabRolesScreen)
            Row(
                Modifier
                    .fillMaxWidth()
                    .pressable { onManage() }
                    .background(ed.Surface, RoundedCornerShape(PaEditorial.Radius.Card))
                    .border(1.dp, ed.Hairline, RoundedCornerShape(PaEditorial.Radius.Card))
                    .padding(16.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Icon(
                    painterResource(R.drawable.ic_lucide_sliders), null,
                    tint = PaEditorial.Primary, modifier = Modifier.size(18.dp),
                )
                Spacer(Modifier.width(10.dp))
                Text("管理协同角色", fontSize = 14.sp, color = PaEditorial.Primary)
            }
            Spacer(Modifier.height(30.dp))
        }
    }
}

/** 角色选择项:瓷白卡 + 细线;选中 = 主色对勾(当前选择)。 */
@Composable
private fun RolePickRow(title: String, desc: String, selected: Boolean, onClick: () -> Unit) {
    val ed = MaterialTheme.editorial
    Row(
        Modifier
            .fillMaxWidth()
            .pressable { onClick() }
            .background(ed.Surface, RoundedCornerShape(PaEditorial.Radius.Card))
            .border(
                1.dp,
                if (selected) PaEditorial.Primary else ed.Hairline,
                RoundedCornerShape(PaEditorial.Radius.Card),
            )
            .padding(16.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Column(Modifier.weight(1f)) {
            Text(
                title,
                fontSize = 14.sp, lineHeight = 20.sp,
                fontWeight = FontWeight(650), color = ed.Ink,
            )
            if (desc.isNotBlank()) {
                Text(
                    desc,
                    fontSize = 12.sp, lineHeight = 17.sp, color = ed.Muted,
                    modifier = Modifier.padding(top = 2.dp),
                    maxLines = 1, overflow = TextOverflow.Ellipsis,
                )
            }
        }
        if (selected) {
            Icon(
                painterResource(R.drawable.ic_lucide_check), null,
                tint = PaEditorial.Primary, modifier = Modifier.size(18.dp),
            )
        }
    }
}
