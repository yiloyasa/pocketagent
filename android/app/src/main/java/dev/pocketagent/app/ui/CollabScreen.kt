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
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
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
import dev.pocketagent.app.ui.anim.PaDuotoneMotif
import dev.pocketagent.app.ui.anim.PaMotion
import dev.pocketagent.app.ui.anim.pressable
import dev.pocketagent.app.ui.collab.CollabCopy as SessionCopy
import dev.pocketagent.app.ui.collab.CollabIntent
import dev.pocketagent.app.ui.collab.CollabMemberUi
import dev.pocketagent.app.ui.collab.CollabSessionReducer
import dev.pocketagent.app.ui.collab.CollabSessionRepository
import dev.pocketagent.app.ui.collab.CollabStatus
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
 *  2. 下方是**按成员分行**的进度卡(身份 + 七词状态徽章 + 卡上入口),
 *     不是混在一起的对话流;
 *  3. 共享一条指令输入,结果按成员分别回报(成员原始结果 ≠ 汇总 Agent 结果)。
 *
 * 【并行对等,不是父子委派】这里没有任何"父 agent 派发子智能体"的概念;
 * 被选中的 agent 是平级协作者,各自收到同一条指令独立执行。参与者列出 daemon 报的
 * 全部协同 agent(CollabAware 门控):可用的可勾选,不可用的灰显并写明原因。
 * 一位失败不拖垮其余。
 *
 * 【P1 协同闭环(Task 6/7)后本页怎么取数】派发前的选人/角色仍在 CollabState;
 * 派发后的会话/成员/终局全部来自 CollabSessionRepository 的单一状态树
 * (进入页面:先 GetCollabSession 快照恢复,再 WatchCollabSession 事件流跟随;
 * 断线保留最后真快照;seq 缺口全量重建)。取消/重试/审批入口以 CollabIntent
 * 从成员卡发出,UI 不直接碰 gRPC,也不读任何散落的全局进度 state。
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
    // 协同会话恢复(Task 6):进入页面 = 快照先行 + 事件流跟随;离开页面停流保快照。
    LaunchedEffect(Unit) { CollabSessionRepository.openLatest() }
    DisposableEffect(Unit) {
        onDispose { CollabSessionRepository.onPageHidden() }
    }

    // 协同会话的单一状态树(daemon 真值的投影;不再读散落的 phases/threadIds)
    val session = CollabSessionRepository.state.collectAsState().value

    val selected = CollabState.selected
    val roleBinding = CollabState.roleBinding
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
                                .background(PaEditorial.OnGraphiteHairline, RoundedCornerShape(PaEditorial.Radius.IconButton)),
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
                            fontSize = PaEditorial.Type.BodySmall.fontSize, lineHeight = 21.sp,
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
                            fontSize = PaEditorial.Type.Caption.fontSize, color = ed.Muted,
                        )
                    }
                    Text(
                        "电脑端接入协同的 agent 都在这里;不可用的标了原因",
                        fontSize = PaEditorial.Type.Caption.fontSize, lineHeight = 17.sp, color = ed.Subtle,
                    )
                    Spacer(Modifier.height(8.dp))
                    if (participants.isEmpty()) {
                        // 诚实空态:daemon 没报任何可协同 agent,不显示假入口
                        EmptyState(
                            R.drawable.ic_lucide_git_branch,
                            "暂无可协同的 agent",
                            "电脑端启用协作能力后,这里会列出可选项",
                            motif = PaDuotoneMotif.DISCOVER_RING, // 协同发现:雷达环 = 正在找电脑端可协同的 agent
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
                            fontSize = PaEditorial.Type.Caption.fontSize, lineHeight = 17.sp, color = ed.Subtle,
                            maxLines = 2, overflow = TextOverflow.Ellipsis,
                        )
                    }
                    if (selectedAgents.isEmpty() && participants.isNotEmpty()) {
                        Text(
                            CollabCopy.EMPTY_NO_SELECTION,
                            Modifier.padding(top = 8.dp),
                            fontSize = PaEditorial.Type.Caption.fontSize, lineHeight = 17.sp, color = ed.Subtle,
                        )
                    }
                }

                // ---- 分工与状态(Task 6/7):协同会话单一状态树驱动 ----
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
                            Text("配置角色", fontSize = PaEditorial.Type.BodySmall.fontSize, color = PaEditorial.Primary)
                            Spacer(Modifier.width(2.dp))
                            Icon(
                                painterResource(R.drawable.ic_lucide_chevron_right), null,
                                tint = PaEditorial.Primary, modifier = Modifier.size(14.dp),
                            )
                        }
                    }
                }

                if (session.sessionId != null) {
                    // ---- 会话横幅:指令回显 + "N/M 已完成 · X 个等待确认"汇总行 ----
                    item(key = "sessionBanner") {
                        CollabSessionBanner(
                            session = session,
                            onCancelSession = { CollabSessionRepository.send(CollabIntent.CancelSession) },
                        )
                        HorizontalHairlineEd()
                    }
                    if (session.members.isEmpty()) {
                        item {
                            Text(
                                "还没有成员受理回执;受理后这里会逐个出现成员卡",
                                Modifier.padding(vertical = 8.dp),
                                fontSize = PaEditorial.Type.Caption.fontSize, lineHeight = 17.sp, color = ed.Subtle,
                            )
                        }
                    } else {
                        // 进场 spring(结论 6):卡片从收起状态浮入,可打断;同卡内的
                        // 状态流转不重播进场(remember 记住入场态)。
                        itemsIndexed(
                            session.members,
                            key = { i, m -> "$i:" + m.threadId.ifBlank { m.agentId } },
                        ) { _, member ->
                            Column {
                                val enter = remember { MutableTransitionState(false).apply { targetState = true } }
                                AnimatedVisibility(
                                    visibleState = enter,
                                    enter = fadeIn(PaMotion.emphasized()) + expandVertically(PaMotion.emphasized()),
                                    exit = shrinkVertically(PaMotion.standard()) + fadeOut(PaMotion.standard()),
                                ) {
                                    Column {
                                        CollabMemberRow(
                                            member = member,
                                            roleName = member.roleId.takeIf { it.isNotBlank() }?.let { rid ->
                                                CollabRolesState.roles.firstOrNull { r -> r.id == rid }
                                            }?.let { collabRoleDisplayName(it) },
                                            agentName = participants.firstOrNull { it.agent.id == member.agentId }?.agent?.name
                                                ?: member.agentId,
                                            iconRes = participants.firstOrNull { it.agent.id == member.agentId }?.agent?.iconRes
                                                ?: R.drawable.ic_lucide_cpu,
                                            onOpenThread = onOpenThread,
                                            onRetry = {
                                                CollabSessionRepository.send(CollabIntent.RetryMember(member.threadId))
                                            },
                                            onCancelMember = {
                                                CollabSessionRepository.send(CollabIntent.CancelMember(member.threadId))
                                            },
                                            onOpenApproval = { requestId ->
                                                // 审批去成员会话页应答(SubmitApproval 既有通道);
                                                // 意图照发(仓库显式不处理,留审计语义)。
                                                CollabSessionRepository.send(
                                                    CollabIntent.OpenApproval(member.threadId, requestId)
                                                )
                                                onOpenThread?.invoke(member.threadId)
                                            },
                                        )
                                        HorizontalHairlineEd()
                                    }
                                }
                            }
                        }
                    }
                    // ---- 结果分区(Task 7):成员原始结果 ≠ 汇总 Agent 结果 ----
                    item(key = "sessionResults") {
                        CollabSessionResults(session = session, onOpenThread = onOpenThread)
                    }
                } else if (session.loading) {
                    item {
                        Text(
                            SessionCopy.LOADING_RESTORE,
                            Modifier.padding(vertical = 8.dp),
                            fontSize = PaEditorial.Type.Caption.fontSize, lineHeight = 17.sp, color = ed.Subtle,
                        )
                    }
                } else if (lastInstruction != null) {
                    // ---- 派发已发出、受理回执未达:七词之"已提交",如实呈现 ----
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
                                    fontSize = PaEditorial.Type.BodySmall.fontSize, lineHeight = 21.sp, color = ed.Ink,
                                )
                            }
                            Spacer(Modifier.height(4.dp))
                            Text(
                                if (dispatching) CollabCopy.BTN_DISPATCHING else SessionCopy.PROGRESS_SUBMITTED,
                                fontSize = PaEditorial.Type.Caption.fontSize, lineHeight = 16.sp, color = ed.Subtle,
                            )
                        }
                    }
                } else if (selectedAgents.isEmpty()) {
                    item {
                        Text(
                            "先在上面勾选这次一起干活的 agent,他们的执行进度会逐个出现在这里",
                            Modifier.padding(vertical = 8.dp),
                            fontSize = PaEditorial.Type.Caption.fontSize, lineHeight = 17.sp, color = ed.Subtle,
                        )
                    }
                }

                // ---- 错误呈现(断线说明 / 操作失败原因):如实给原文,不粉饰 ----
                session.error?.let { err ->
                    item {
                        Text(
                            err,
                            Modifier
                                .fillMaxWidth()
                                .padding(top = 12.dp)
                                .background(ed.DeniedContainer, RoundedCornerShape(PaEditorial.Radius.IconButton))
                                .padding(12.dp),
                            fontSize = PaEditorial.Type.Caption.fontSize, lineHeight = 17.sp,
                            color = EditorialTone.DENIED.text,
                        )
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
                    fontSize = PaEditorial.Type.Caption.fontSize, lineHeight = 16.sp, color = ed.Subtle,
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
                        .background(ed.Surface, RoundedCornerShape(PaEditorial.Radius.IconButton))
                        .border(1.dp, ed.Hairline, RoundedCornerShape(PaEditorial.Radius.IconButton))
                        .padding(horizontal = 12.dp, vertical = 12.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Box(Modifier.weight(1f)) {
                        if (cmd.isEmpty()) {
                            Text(
                                "描述需要协同完成的任务…",
                                fontSize = PaEditorial.Type.Body.fontSize, lineHeight = 21.sp, color = ed.Subtle,
                                maxLines = 1, overflow = TextOverflow.Ellipsis,
                            )
                        }
                        BasicTextField(
                            value = cmd,
                            onValueChange = { cmd = it },
                            modifier = Modifier.fillMaxWidth(),
                            textStyle = TextStyle(fontSize = PaEditorial.Type.Body.fontSize, lineHeight = 21.sp, color = ed.Ink),
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
                            RoundedCornerShape(PaEditorial.Radius.IconButton),
                        )
                        .padding(horizontal = 16.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(
                        if (dispatching) CollabCopy.BTN_DISPATCHING else CollabCopy.BTN_DISPATCH,
                        fontSize = PaEditorial.Type.Body.fontSize,
                        fontWeight = FontWeight.SemiBold,
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

/** 细分隔线(协同页逐成员行之间)。 */
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
 * 协同会话横幅:指令原文 + 汇总行"N/M 已完成 · X 个等待确认"(Task 7:绝不
 * 显示笼统的"运行中")+ 会话级动作(取消整个协同,禁用时给原因)。
 *
 * 【daemon 会话级错误的呈现】FAILED/CANCELLED 的会话带人话原因(applySessionError
 * 生成),原样展示;成员各自的真相在成员卡上,横幅只做如实汇总。
 */
@Composable
private fun CollabSessionBanner(
    session: dev.pocketagent.app.ui.collab.CollabSessionState,
    onCancelSession: () -> Unit,
) {
    val ed = MaterialTheme.editorial
    val actions = CollabSessionReducer.sessionActions(session)
    Column(
        Modifier
            .fillMaxWidth()
            .padding(top = 12.dp),
    ) {
        Row(
            Modifier.fillMaxWidth(),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Icon(
                painterResource(R.drawable.ic_lucide_git_branch), null,
                tint = PaEditorial.Primary, modifier = Modifier.size(16.dp),
            )
            Spacer(Modifier.width(8.dp))
            Text(
                session.instruction.ifBlank { "协同任务" },
                Modifier.weight(1f),
                fontSize = PaEditorial.Type.Body.fontSize, lineHeight = 21.sp,
                fontWeight = FontWeight.SemiBold, color = ed.Ink,
                maxLines = 2, overflow = TextOverflow.Ellipsis,
            )
            // 会话级七词徽章(色/形通道借四词体系,词用七词词表)
            CollabStatusBadge(status = session.status, raw = session.statusRaw)
        }
        // 汇总行:比例永远在(0/3 也如实说);等待确认/未完成分段追加
        val summary = SessionCopy.summaryLine(CollabSessionReducer.counts(session.members))
        if (summary != null) {
            Text(
                summary,
                Modifier.padding(top = 4.dp),
                fontSize = PaEditorial.Type.Caption.fontSize, lineHeight = 17.sp, color = ed.Muted,
            )
        }
        // 会话级人话原因(daemon 原文;FAILED/CANCELLED 时非空)
        if (session.sessionError.isNotBlank()) {
            Text(
                session.sessionError,
                Modifier
                    .fillMaxWidth()
                    .padding(top = 8.dp)
                    .background(ed.DeniedContainer, RoundedCornerShape(PaEditorial.Radius.IconButton))
                    .padding(12.dp),
                fontSize = PaEditorial.Type.Caption.fontSize, lineHeight = 17.sp,
                color = EditorialTone.DENIED.text,
            )
        }
        // 会话级动作:取消整个协同(禁用时旁边给原因,不占"能点"的形)
        Row(
            Modifier.fillMaxWidth().padding(top = 6.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            if (actions.canCancel) {
                Row(
                    Modifier
                        .pressable { onCancelSession() }
                        .padding(horizontal = 4.dp, vertical = 6.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Icon(
                        painterResource(R.drawable.ic_lucide_x), null,
                        tint = PaEditorial.Primary, modifier = Modifier.size(14.dp),
                    )
                    Spacer(Modifier.width(4.dp))
                    Text(SessionCopy.ACTION_CANCEL_SESSION, fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Primary)
                }
            } else if (session.members.isNotEmpty()) {
                Text(
                    "${SessionCopy.ACTION_CANCEL_SESSION} · ${actions.cancelReason}",
                    Modifier.padding(horizontal = 4.dp),
                    fontSize = PaEditorial.Type.Caption.fontSize, lineHeight = 16.sp, color = ed.Subtle,
                )
            }
        }
    }
}

/**
 * 协同会话状态徽章:七词词表(词)+ 四词体系的色档与形(通道复用)。
 * UNKNOWN 不进徽章 —— 返回 null,由调用方走"原文 + 降级说明"的诚实呈现。
 */
@Composable
private fun CollabStatusBadge(status: CollabStatus, raw: String?) {
    val badge = CollabSessionReducer.editorialBadgeOf(status)
    if (badge == null) {
        Column(horizontalAlignment = Alignment.End) {
            Text(
                SessionCopy.statusWord(status, raw),
                fontSize = PaEditorial.Type.Caption.fontSize, lineHeight = 16.sp, color = MaterialTheme.editorial.Muted,
            )
            Text(
                SessionCopy.UNKNOWN_STATUS_NOTE,
                fontSize = PaEditorial.Type.Micro.fontSize, lineHeight = 14.sp,
                color = MaterialTheme.editorial.Subtle,
                modifier = Modifier.padding(top = 2.dp),
            )
        }
    } else {
        PaStatusBadge(
            EditorialStatusView(status = badge, label = SessionCopy.statusWord(status, raw)),
        )
    }
}

/**
 * 单个成员卡(Task 6/7):身份图 + 名称 + 角色行 + 七词状态徽章 + 进度/审批/
 * 原因/动作。所有状态与入口都来自 daemon 成员卡真值([CollabMemberUi]);
 * 意图(取消/重试/审批/查看)从卡上发出,经 CollabSessionRepository 落 RPC。
 *
 * 【禁用按钮旁必有原因】重试/取消的可点态与禁用原因同源
 * (CollabSessionReducer.memberActions):能点的给主色入口,不能点的灰着
 * 并写明为什么 —— 不会出现"灰着但不知道为什么"。
 */
@Composable
private fun CollabMemberRow(
    member: CollabMemberUi,
    roleName: String?,
    agentName: String,
    iconRes: Int,
    onOpenThread: ((String) -> Unit)?,
    onRetry: () -> Unit,
    onCancelMember: () -> Unit,
    onOpenApproval: (String) -> Unit,
) {
    val ed = MaterialTheme.editorial
    val actions = CollabSessionReducer.memberActions(member)
    var reasonOpen by remember { mutableStateOf(false) }
    Column(
        Modifier
            .fillMaxWidth()
            .padding(vertical = 12.dp),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Image(
                bitmap = agentBitmap(iconRes),
                contentDescription = agentName,
                modifier = Modifier
                    .size(PaEditorial.Space.IconBox)
                    .clip(RoundedCornerShape(PaEditorial.Radius.IconButton)),
                contentScale = androidx.compose.ui.layout.ContentScale.Crop,
            )
            Spacer(Modifier.width(12.dp))
            Column(Modifier.weight(1f)) {
                Text(
                    agentName,
                    fontSize = PaEditorial.Type.Heading.fontSize, lineHeight = 21.sp,
                    fontWeight = FontWeight.SemiBold, color = ed.Ink,
                    maxLines = 1, overflow = TextOverflow.Ellipsis,
                )
                if (roleName != null) {
                    Text(
                        "角色 · $roleName",
                        Modifier.padding(top = 1.dp),
                        fontSize = PaEditorial.Type.Caption.fontSize, lineHeight = 17.sp, color = ed.Muted,
                        maxLines = 1, overflow = TextOverflow.Ellipsis,
                    )
                }
            }
            // 七词徽章的状态流转 spring(结论 6):状态一变,新徽章淡入+轻缩放浮入。
            AnimatedContent(
                targetState = member.status,
                transitionSpec = {
                    (fadeIn(PaMotion.standard()) + scaleIn(initialScale = 0.85f, animationSpec = PaMotion.standard()))
                        .togetherWith(fadeOut(PaMotion.standard()))
                },
                label = "collabMemberBadge",
            ) { st ->
                CollabStatusBadge(status = st, raw = member.statusRaw)
            }
        }
        // daemon 的一句话进度(有界摘要,不是原始日志);终局成员不叠进度行
        if (member.progress.isNotBlank() && !member.status.isFinished) {
            Text(
                member.progress,
                Modifier.padding(top = 4.dp),
                fontSize = PaEditorial.Type.Caption.fontSize, lineHeight = 16.sp, color = ed.Subtle,
                maxLines = 2, overflow = TextOverflow.Ellipsis,
            )
        }
        // 挂起审批(该成员等的是用户,不是系统在跑):逐笔列动作 + 去确认入口
        member.pendingApprovals.forEach { approval ->
            Row(
                Modifier
                    .fillMaxWidth()
                    .padding(top = 6.dp)
                    .background(ed.AttentionContainer, RoundedCornerShape(PaEditorial.Radius.IconButton))
                    .padding(horizontal = 12.dp, vertical = 8.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(
                    SessionCopy.APPROVAL_WAITING_PREFIX + approval.label.ifBlank { "(电脑端没有给出动作说明)" },
                    Modifier.weight(1f),
                    fontSize = PaEditorial.Type.Caption.fontSize, lineHeight = 17.sp,
                    color = EditorialTone.ATTENTION.text,
                    maxLines = 2, overflow = TextOverflow.Ellipsis,
                )
                if (onOpenThread != null && actions.canAnswerApproval) {
                    Text(
                        SessionCopy.ACTION_OPEN_APPROVAL,
                        Modifier
                            .pressable { onOpenApproval(approval.requestId) }
                            .padding(start = 8.dp),
                        fontSize = PaEditorial.Type.Caption.fontSize,
                        fontWeight = FontWeight.SemiBold,
                        color = EditorialTone.ATTENTION.text,
                    )
                }
            }
        }
        // 未完成成员的原因(daemon 人话原文,按成员各自展示;不改全局):
        // 收起时两行省略 + 「查看原因」展开全文 —— 截断不给真相,展开才完整。
        if (member.status.isFinished && !member.error.isNullOrBlank()) {
            Column(
                Modifier
                    .fillMaxWidth()
                    .padding(top = 8.dp)
                    .background(ed.DeniedContainer, RoundedCornerShape(PaEditorial.Radius.IconButton))
                    .padding(horizontal = 12.dp, vertical = 8.dp),
            ) {
                Text(
                    member.error,
                    Modifier.fillMaxWidth(),
                    fontSize = PaEditorial.Type.Caption.fontSize, lineHeight = 17.sp,
                    color = EditorialTone.DENIED.text,
                    maxLines = if (reasonOpen) Int.MAX_VALUE else 2,
                    overflow = TextOverflow.Ellipsis,
                )
                Text(
                    if (reasonOpen) SessionCopy.ACTION_HIDE_REASON else SessionCopy.ACTION_VIEW_REASON,
                    Modifier
                        .pressable { reasonOpen = !reasonOpen }
                        .padding(top = 4.dp, bottom = 2.dp),
                    fontSize = PaEditorial.Type.Caption.fontSize, lineHeight = 17.sp,
                    fontWeight = FontWeight.SemiBold,
                    color = EditorialTone.DENIED.text,
                )
            }
        }
        // 卡上动作行:重试 / 取消 / 查看会话(可点 = 主色;禁用 = 灰 + 原因)。
        Row(
            Modifier.fillMaxWidth().padding(top = 6.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(2.dp),
        ) {
            // 重试:只对已收尾成员开放(daemon 前置条件);禁用时说明为什么
            MemberActionText(
                label = SessionCopy.ACTION_RETRY,
                enabled = actions.canRetry,
                reason = actions.retryReason,
                iconRes = R.drawable.ic_lucide_refresh_cw,
                onClick = onRetry,
            )
            // 取消:只对未收尾成员开放(已收尾 = 无需取消,如实说)
            MemberActionText(
                label = SessionCopy.ACTION_CANCEL_MEMBER,
                enabled = actions.canCancel,
                reason = actions.cancelReason,
                iconRes = R.drawable.ic_lucide_x,
                onClick = onCancelMember,
            )
            // 查看会话:有子线程且有导航通道才摆(没有通道就没有假入口)
            if (onOpenThread != null && actions.canOpenSession) {
                Row(
                    Modifier
                        .pressable { onOpenThread(member.threadId) }
                        .padding(horizontal = 4.dp, vertical = 6.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Icon(
                        painterResource(R.drawable.ic_lucide_chevron_right), null,
                        tint = PaEditorial.Primary, modifier = Modifier.size(14.dp),
                    )
                    Spacer(Modifier.width(4.dp))
                    Text(SessionCopy.ACTION_OPEN_SESSION, fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Primary)
                }
            }
        }
    }
}

/**
 * 单个卡上动作:可点 = 主色入口;禁用 = 灰字 + 禁用原因(Task 7:每个禁用按钮
 * 旁边显示禁用原因,不出"灰着但不知道为什么")。禁用时不占触感、不吞点击。
 */
@Composable
private fun MemberActionText(
    label: String,
    enabled: Boolean,
    reason: String?,
    iconRes: Int,
    onClick: () -> Unit,
) {
    val ed = MaterialTheme.editorial
    if (enabled) {
        Row(
            Modifier
                .pressable { onClick() }
                .padding(horizontal = 4.dp, vertical = 6.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Icon(
                painterResource(iconRes), null,
                tint = PaEditorial.Primary, modifier = Modifier.size(14.dp),
            )
            Spacer(Modifier.width(4.dp))
            Text(label, fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Primary)
        }
    } else {
        Text(
            "$label · ${reason ?: "当前不可用"}",
            Modifier.padding(horizontal = 4.dp, vertical = 6.dp),
            fontSize = PaEditorial.Type.Caption.fontSize, lineHeight = 16.sp, color = ed.Subtle,
        )
    }
}

/**
 * 结果分区(Task 7):「成员原始结果」逐成员列终稿摘要/未完成原因,与
 * 「汇总 Agent 结果」分区**分开** —— 不把成员原话冒充汇总。
 *
 * 【diff 纪律】不自动合并:分区里固定给"不会自动合并"的说明。比较/采用入口
 * 的文案常量已备(COMPARE_RESULTS / CHOOSE_RESULT),P1 没有比较页与采用动作,
 * 不摆假入口 —— "查看"入口(打开成员会话)是唯一真实接线的通道。
 */
@Composable
private fun CollabSessionResults(
    session: dev.pocketagent.app.ui.collab.CollabSessionState,
    onOpenThread: ((String) -> Unit)?,
) {
    val ed = MaterialTheme.editorial
    val finished = session.members.filter { it.status.isFinished }
    Column(Modifier.fillMaxWidth().padding(top = 16.dp)) {
        Text(SessionCopy.SECTION_MEMBER_RESULTS, style = PaEditorial.Type.Heading, color = ed.Ink)
        Spacer(Modifier.height(8.dp))
        if (finished.isEmpty()) {
            Text(
                SessionCopy.MEMBER_RESULTS_EMPTY,
                fontSize = PaEditorial.Type.Caption.fontSize, lineHeight = 17.sp, color = ed.Subtle,
            )
        } else {
            finished.forEach { member ->
                Column(Modifier.fillMaxWidth().padding(vertical = 6.dp)) {
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Text(
                            member.agentId,
                            Modifier.weight(1f),
                            fontSize = PaEditorial.Type.BodySmall.fontSize, lineHeight = 19.sp,
                            fontWeight = FontWeight.SemiBold, color = ed.Ink,
                            maxLines = 1, overflow = TextOverflow.Ellipsis,
                        )
                        CollabStatusBadge(status = member.status, raw = member.statusRaw)
                    }
                    // 成员原始结果:成功 = 终稿摘要(daemon 有界截断的原文);
                    // 未完成/已取消 = daemon 给的人话原因。各自如实,不合并。
                    val body = member.summary.ifBlank { member.error }
                    if (body.isNotBlank()) {
                        Text(
                            body,
                            Modifier.padding(top = 4.dp),
                            fontSize = PaEditorial.Type.Caption.fontSize, lineHeight = 18.sp,
                            color = if (member.status == CollabStatus.COMPLETED) ed.Ink else EditorialTone.DENIED.text,
                        )
                    }
                    if (onOpenThread != null && member.threadId.isNotBlank()) {
                        Row(
                            Modifier
                                .pressable { onOpenThread(member.threadId) }
                                .padding(top = 2.dp),
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            Icon(
                                painterResource(R.drawable.ic_lucide_chevron_right), null,
                                tint = PaEditorial.Primary, modifier = Modifier.size(14.dp),
                            )
                            Spacer(Modifier.width(4.dp))
                            Text(SessionCopy.VIEW_RESULT, fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Primary)
                        }
                    }
                }
            }
        }
        Spacer(Modifier.height(16.dp))
        Text(SessionCopy.SECTION_SUMMARY_RESULT, style = PaEditorial.Type.Heading, color = ed.Ink)
        Spacer(Modifier.height(4.dp))
        Text(
            SessionCopy.SUMMARY_RESULT_NONE,
            fontSize = PaEditorial.Type.Caption.fontSize, lineHeight = 17.sp, color = ed.Subtle,
        )
        Spacer(Modifier.height(4.dp))
        Text(
            SessionCopy.DIFF_MERGE_NOTICE,
            fontSize = PaEditorial.Type.Caption.fontSize, lineHeight = 17.sp, color = ed.Subtle,
        )
    }
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
            fontSize = PaEditorial.Type.Caption.fontSize, lineHeight = 17.sp,
            color = if (on && ready) ed.Ink else ed.Muted,
            maxLines = 1,
        )
        if (stateWord != null) {
            Spacer(Modifier.height(2.dp))
            Text(
                stateWord,
                fontSize = PaEditorial.Type.Micro.fontSize, lineHeight = 14.sp,
                color = stateWordColor,
                maxLines = 1, overflow = TextOverflow.Ellipsis,
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
                Text("为 ${selectedAgentLabel(agentId)} 选角色", fontSize = PaEditorial.Type.Heading.fontSize, color = ed.Ink)
                Text(
                    "同一 agent 可绑定不同岗位设定",
                    fontSize = PaEditorial.Type.Caption.fontSize, lineHeight = 17.sp, color = ed.Muted,
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
                    fontSize = PaEditorial.Type.Caption.fontSize, lineHeight = 17.sp, color = ed.Subtle,
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
                Text("管理协同角色", fontSize = PaEditorial.Type.Body.fontSize, color = PaEditorial.Primary)
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
                fontSize = PaEditorial.Type.Body.fontSize, lineHeight = 20.sp,
                fontWeight = FontWeight.SemiBold, color = ed.Ink,
            )
            if (desc.isNotBlank()) {
                Text(
                    desc,
                    fontSize = PaEditorial.Type.Caption.fontSize, lineHeight = 17.sp, color = ed.Muted,
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
