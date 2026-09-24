package dev.pocketagent.app.ui

import androidx.compose.foundation.Image
import androidx.compose.animation.animateColorAsState
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
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.material3.Icon
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
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.AgentConnection
import dev.pocketagent.app.R
import dev.pocketagent.app.ui.anim.EmptyState
import dev.pocketagent.app.ui.anim.IconActionButton
import dev.pocketagent.app.ui.anim.PaMotion
import dev.pocketagent.app.ui.anim.pressable
import dev.pocketagent.app.ui.component.PaSheet
import dev.pocketagent.app.ui.theme.Pa
import dev.pocketagent.app.ui.theme.paPushBack

/**
 * 协同页(专属):一条指令扇出给多个对等 agent 并行协作。
 *
 * 【为什么是独立页、且视觉与普通会话明显不同】
 * 普通会话是"手机 ↔ 单一 agent"的 1:1 对话流(ChatScreen);协同会话是
 * "一条指令 + N 个对等 agent 各自回报"的编排台。两者形态本质不同,所以这里:
 *  1. 顶部有**绿色调专属横幅** + 分支图标 + "协同模式"徽标(普通会话没有这层身份);
 *  2. 下方是**按 agent 分列的状态卡**(不是混在一起的对话流);
 *  3. 共享一条指令输入,但回报区是每个 agent 独立的卡片。
 * 这三点让"这是协同会话,不是普通聊天"一眼可辨。
 *
 * 【并行对等,不是父子委派】这里没有任何"父 agent 派发子智能体"的概念;
 * 被选中的 agent 是平级协作者,各自收到同一条指令独立执行。daemon 侧
 * 的扇出能力由主代理并行实现,本页只负责编排与展示(数据来源走既有 RPC + 本地状态)。
 */
@Composable
fun CollabScreen(
    onBack: () -> Unit,
    onOpenCollabRoles: () -> Unit,
) {
    val ctx = LocalContext.current
    val caps = AgentConnection.hostCapabilities.collectAsState().value
    val agents = remember(caps) { buildAgents(caps) }
    val collabIds = remember(caps) { resolveCollabAgentIds(caps) }
    val disabled = AgentConnection.disabledAgents
    val capable = remember(agents, collabIds, disabled) {
        collabCapableAgents(agents, collabIds, disabled)
    }

    // 进入即拉一次角色列表(给"为 agent 配角色"用)
    LaunchedEffect(collabIds) { if (collabIds.isNotEmpty()) CollabRolesState.load() }

    val selected = CollabState.selected
    val roleBinding = CollabState.roleBinding
    val phases = CollabState.phases
    val lastInstruction = CollabState.lastInstruction

    var cmd by remember { mutableStateOf("") }
    var roleFor by remember { mutableStateOf<String?>(null) }

    // 已选中的 agent(从可协同清单里挑,保持顺序稳定,不重复过滤)
    val selectedAgents = capable.filter { it.id in selected }
    val canSend = selected.isNotEmpty() && cmd.isNotBlank()

    // 【推背模糊】角色选择浮层(PaSheet,玻璃材质)打开时,本页内容模糊+缩小+压暗,
    // 透过玻璃看过去才是真磨砂(PaGlass 的职责分离:模糊在内容层,不在浮层上)。
    Column(Modifier.fillMaxSize().background(Pa.Bg).statusBarsPadding().paPushBack(roleFor != null)) {
        // ---- 顶部横幅:协同身份(一眼可辨)----
        CollabHeader(onBack = onBack, agentCount = selected.size)

        LazyColumn(
            Modifier.weight(1f).fillMaxWidth(),
            contentPadding = androidx.compose.foundation.layout.PaddingValues(horizontal = 16.dp, vertical = 12.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            // ---- 选 agent(多选)----
            item { AgentSelector(capable = capable, selected = selected, onToggle = CollabState::toggle) }

            if (selectedAgents.isEmpty()) {
                item {
                    Text(CollabCopy.EMPTY_NO_SELECTION, fontSize = 12.5.sp, color = Pa.TextTertiary)
                }
            }

            // ---- 每个已选 agent:品牌图标 + 角色配置 + 派发相位卡 ----
            items(selectedAgents, key = { it.id }) { agent ->
                val roleId = roleBinding[agent.id]
                val roleName = roleId?.let { rid ->
                    CollabRolesState.roles.firstOrNull { r -> r.id == rid }?.let { collabRoleDisplayName(it) }
                }
                CollabAgentCard(
                    agent = agent,
                    roleName = roleName,
                    phase = phases[agent.id] ?: CollabAgentPhase.IDLE,
                    onConfigRole = { roleFor = agent.id },
                )
            }

            // ---- 已发指令回显 + 诚实等待说明(扁平行,不做底色块) ----
            if (lastInstruction != null) {
                item {
                    Column {
                        Row(
                            Modifier.fillMaxWidth().padding(vertical = 4.dp),
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            Icon(painterResource(R.drawable.ic_lucide_git_branch), null, tint = Pa.Accent, modifier = Modifier.size(16.dp))
                            Spacer(Modifier.width(8.dp))
                            Text(lastInstruction!!, fontSize = 13.sp, color = Pa.TextPrimary)
                        }
                        Spacer(Modifier.height(4.dp))
                        Text(CollabCopy.WAITING_DAEMON, fontSize = 11.5.sp, color = Pa.TextTertiary)
                    }
                }
            }
        }

        // ---- 底部指令输入(共享一条指令)----
        Row(Modifier.fillMaxWidth().padding(horizontal = 16.dp).padding(bottom = 12.dp)) {
            CommandInput(
                cmd = cmd,
                onCmdChange = { cmd = it },
                canSend = canSend,
                onSend = { if (canSend) { CollabState.dispatch(cmd); cmd = "" } },
            )
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

/** 顶部横幅:分支图标 + 协同徽标,铺在 Bg 上(页面头部不是卡,统一卡语言下去底)。 */
@Composable
private fun CollabHeader(onBack: () -> Unit, agentCount: Int) {
    Column(
        Modifier.fillMaxWidth()
            .padding(horizontal = 16.dp, vertical = 12.dp),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            IconActionButton(R.drawable.ic_lucide_chevron_left, "返回", onBack, bg = Pa.Card)
            Spacer(Modifier.width(10.dp))
            Icon(painterResource(R.drawable.ic_lucide_git_branch), null, tint = Pa.Green, modifier = Modifier.size(20.dp))
            Spacer(Modifier.width(8.dp))
            Text("多 Agent 协同", fontSize = 18.sp, color = Pa.TextPrimary, fontWeight = FontWeight.Medium)
            Spacer(Modifier.weight(1f))
            // 协同中徽标:纯色文字(统一卡语言:去底色 pill)
            Text("协同模式", fontSize = 11.sp, color = Pa.GreenText)
        }
        Spacer(Modifier.height(6.dp))
        Text(
            if (agentCount == 0) "勾选 agent,一条指令同时派给它们并行协作"
            else "已选 $agentCount 个 agent · 一条指令扇出并行执行",
            fontSize = 12.sp, color = Pa.TextSecondary,
        )
    }
}

/** 选 agent 区:多选芯片,只显示 daemon 动态上报的可协同 agent。 */
@Composable
private fun AgentSelector(
    capable: List<AgentInfo>,
    selected: Set<String>,
    onToggle: (String) -> Unit,
) {
    Column(Modifier.fillMaxWidth()) {
        Text("参与这次协作的 agent（多选）", fontSize = 15.sp, color = Pa.TextPrimary, fontWeight = FontWeight.Medium)
        Spacer(Modifier.height(4.dp))
        Text("只显示电脑端当前接入协同的 agent", fontSize = 11.5.sp, color = Pa.TextSecondary)
        Spacer(Modifier.height(10.dp))
        if (capable.isEmpty()) {
            // 诚实空态:daemon 没报任何可协同 agent,不显示假入口
            EmptyState(
                R.drawable.ic_lucide_git_branch,
                "暂无可协同的 agent",
                "电脑端启用协作能力后,这里会列出可选项",
            )
        } else {
            Row(
                Modifier.horizontalScroll(rememberScrollState()),
                horizontalArrangement = Arrangement.spacedBy(10.dp),
            ) {
                capable.forEach { a ->
                    CollabAgentChip(a = a, on = a.id in selected, onClick = { onToggle(a.id) })
                }
            }
        }
    }
}

/**
 * 单个 agent 多选芯片:品牌图标 + 名称。
 * 【统一卡语言】底色一律 Pa.Card(唯一卡型);选中态只用 1.5dp Accent 描边 +
 * 文字颜色表达,不再换底色(信息层级靠颜色,不靠第二种底)。
 */
@Composable
private fun CollabAgentChip(a: AgentInfo, on: Boolean, onClick: () -> Unit) {
    // 选中描边色 spring 过渡:点选瞬间描边"流"成品牌色,与按压缩放叠出跟手感
    val borderColor by animateColorAsState(
        targetValue = if (on) Pa.Accent else Pa.Separator,
        animationSpec = PaMotion.standard(),
        label = "collabChipBorder",
    )
    Column(
        Modifier.pressable { onClick() }.width(84.dp)
            .background(Pa.Card, RoundedCornerShape(16.dp))
            .border(if (on) 1.5.dp else 0.5.dp, borderColor, RoundedCornerShape(16.dp))
            .padding(12.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Image(
            bitmap = agentBitmap(a.iconRes),
            contentDescription = a.name,
            modifier = Modifier.size(40.dp),
            contentScale = androidx.compose.ui.layout.ContentScale.Crop,
        )
        Spacer(Modifier.height(6.dp))
        Text(a.name, fontSize = 12.sp, color = if (on) Pa.TextPrimary else Pa.TextSecondary, maxLines = 1)
        if (on) {
            Spacer(Modifier.height(2.dp))
            Text("已选", fontSize = 10.sp, color = Pa.Accent)
        }
    }
}

/** 单个 agent 的状态卡:品牌图标 + 名称 + 角色 + 派发相位(收到/执行中)。 */
@Composable
private fun CollabAgentCard(
    agent: AgentInfo,
    roleName: String?,
    phase: CollabAgentPhase,
    onConfigRole: () -> Unit,
) {
    val phaseTint = phaseColor(phase)
    Column(
        Modifier.fillMaxWidth()
            // 唯一卡型:16dp/Pa.Card/无边框无阴影
            .background(Pa.Card, RoundedCornerShape(16.dp))
            .padding(16.dp),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Image(
                bitmap = agentBitmap(agent.iconRes),
                contentDescription = agent.name,
                modifier = Modifier.size(34.dp).clip(CircleShape),
                contentScale = androidx.compose.ui.layout.ContentScale.Crop,
            )
            Spacer(Modifier.width(10.dp))
            Column(Modifier.weight(1f)) {
                Text(agent.name, fontSize = 15.sp, color = Pa.TextPrimary, fontWeight = FontWeight.Medium)
                if (roleName != null) {
                    Spacer(Modifier.height(2.dp))
                    Text("角色 · $roleName", fontSize = 11.5.sp, color = Pa.AccentText, maxLines = 1, overflow = TextOverflow.Ellipsis)
                }
            }
            // 相位徽标:纯语义色文字(去底色 pill)
            Text(collabPhaseLabel(phase), fontSize = 11.sp, color = phaseTint)
        }
        Spacer(Modifier.height(10.dp))
        // 角色配置行(可点,复用既有角色数据);卡内扁平行,不做底色子容器
        Row(
            Modifier.fillMaxWidth().pressable { onConfigRole() }
                .padding(horizontal = 2.dp, vertical = 10.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Icon(painterResource(R.drawable.ic_lucide_sliders), null, tint = Pa.Accent, modifier = Modifier.size(16.dp))
            Spacer(Modifier.width(8.dp))
            Text(
                if (roleName != null) "角色：$roleName" else CollabCopy.ROLE_NONE,
                fontSize = 12.5.sp, color = Pa.TextSecondary,
            )
            Spacer(Modifier.weight(1f))
            Icon(painterResource(R.drawable.ic_lucide_chevron_right), null, tint = Pa.TextTertiary, modifier = Modifier.size(16.dp))
        }
    }
}

/** 相位 → 颜色(收到=蓝 / 执行中=黄 / 完成=绿 / 出错=红 / 空闲=灰)。 */
private fun phaseColor(p: CollabAgentPhase): Color = when (p) {
    CollabAgentPhase.IDLE -> Pa.TextTertiary
    CollabAgentPhase.ACK -> Pa.Accent
    CollabAgentPhase.RUNNING -> Pa.Yellow
    CollabAgentPhase.DONE -> Pa.Green
    CollabAgentPhase.ERROR -> Pa.Red
}

/** 底部指令输入框:共享一条指令,发送键在高亮/置灰间切换。 */
@Composable
private fun CommandInput(
    cmd: String,
    onCmdChange: (String) -> Unit,
    canSend: Boolean,
    onSend: () -> Unit,
) {
    Row(
        Modifier.fillMaxWidth().background(Pa.Card, RoundedCornerShape(16.dp))
            .padding(horizontal = 12.dp, vertical = 8.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        BasicTextField(
            value = cmd,
            onValueChange = onCmdChange,
            modifier = Modifier.weight(1f).padding(horizontal = 8.dp, vertical = 10.dp),
            textStyle = TextStyle(color = Pa.TextPrimary, fontSize = 14.sp),
            decorationBox = { inner ->
                if (cmd.isEmpty()) Text("输入要协同执行的指令…", color = Pa.TextTertiary, fontSize = 14.sp)
                inner()
            },
        )
        IconActionButton(
            R.drawable.ic_lucide_arrow_up, "发送",
            onSend,
            bg = if (canSend) Pa.Accent else Pa.Hover,
        )
    }
}

/**
 * 角色选择底部弹层:列出该 agent 的协同角色(复用 CollabRolesState 既有数据),
 * 可解绑(未配置角色),也可跳去「协同角色」页做完整管理(复用 CollabRolesScreen)。
 *
 * 【PaSheet 统一浮层】玻璃底 + 82% 屏高上限 + 内建滚动(AUDIT sheet 溢出规则的
 * 结构化落地);被覆盖内容的推背模糊由 CollabScreen 根上的 paPushBack 负责。
 */
@Composable
private fun CollabRolePickerSheet(
    agentId: String,
    currentRoleId: String?,
    onPick: (String?) -> Unit,
    onManage: () -> Unit,
    onDismiss: () -> Unit,
) {
    val roles = CollabRolesState.roles.filter { it.agentId == agentId }
    PaSheet(onDismissRequest = onDismiss) {
        Row(Modifier.fillMaxWidth().padding(horizontal = 20.dp)) {
            Column(Modifier.weight(1f)) {
                Text("为 ${selectedAgentLabel(agentId)} 选角色", fontSize = 18.sp, color = Pa.TextPrimary)
                Text("同一 agent 可绑定不同岗位设定", fontSize = 12.sp, color = Pa.TextSecondary, modifier = Modifier.padding(top = 2.dp))
            }
            IconActionButton(R.drawable.ic_lucide_x, "关闭", onDismiss, bg = Color.Transparent, size = 38.dp, iconSize = 18.dp)
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
                Text("该 agent 还没有专属角色,去「协同角色」里新建", fontSize = 12.sp, color = Pa.TextTertiary, modifier = Modifier.padding(vertical = 6.dp))
            }
            // 管理角色入口(复用现有 CollabRolesScreen;唯一卡型)
            Row(
                Modifier.fillMaxWidth().pressable { onManage() }
                    .background(Pa.Card, RoundedCornerShape(16.dp))
                    .padding(16.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Icon(painterResource(R.drawable.ic_lucide_sliders), null, tint = Pa.Accent, modifier = Modifier.size(18.dp))
                Spacer(Modifier.width(10.dp))
                Text("管理协同角色", fontSize = 14.sp, color = Pa.Accent)
            }
            Spacer(Modifier.height(30.dp))
        }
    }
}

/** 角色选择项(唯一卡型:16dp/Pa.Card/无边)。 */
@Composable
private fun RolePickRow(title: String, desc: String, selected: Boolean, onClick: () -> Unit) {
    Row(
        Modifier.fillMaxWidth().pressable { onClick() }
            .background(Pa.Card, RoundedCornerShape(16.dp))
            .padding(16.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Column(Modifier.weight(1f)) {
            Text(title, fontSize = 14.5.sp, color = Pa.TextPrimary)
            if (desc.isNotBlank()) {
                Text(desc, fontSize = 12.sp, color = Pa.TextSecondary, modifier = Modifier.padding(top = 2.dp), maxLines = 1, overflow = TextOverflow.Ellipsis)
            }
        }
        if (selected) Icon(painterResource(R.drawable.ic_lucide_check), null, tint = Pa.Accent, modifier = Modifier.size(18.dp))
    }
}
