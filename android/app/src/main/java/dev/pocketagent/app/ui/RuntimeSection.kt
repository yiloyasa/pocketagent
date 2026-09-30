package dev.pocketagent.app.ui

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.expandVertically
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.shrinkVertically
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Icon
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.RadioButton
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.runtime.collectAsState
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.TextRange
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.TextFieldValue
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.AgentConnection
import dev.pocketagent.app.R
import dev.pocketagent.app.ui.anim.EmptyState
import dev.pocketagent.app.ui.anim.PaMotion
import dev.pocketagent.app.ui.anim.pressable
import dev.pocketagent.app.ui.component.PaSheet
import dev.pocketagent.app.ui.theme.Pa
import dev.pocketagent.app.ui.theme.PaEditorial
import dev.pocketagent.app.ui.theme.EditorialTone
import kotlinx.coroutines.async
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.launch
import pocketagent.v1.CollabPolicy
import pocketagent.v1.ControlPlane
import pocketagent.v1.RuntimePolicy
import pocketagent.v1.UnknownEventSummary

/**
 * 设置页的「运行时」区段(W11b)。
 *
 * 【为什么单独成文件,而不是塞进 SettingsScreen】SettingsScreen.kt 已 178KB,
 * 再加这一块(五组内容:恢复横幅 / 本机总览与形态卡 / 预算 / 消息透析弹窗 / 协同策略)
 * 会让它彻底读不动。这里只做**排版**,所有规则(取值映射、缺席判定)都在
 * [RuntimeStateLogic] 的纯函数里被单测钉住 —— 本文件里不出现任何 if 判据。
 *
 * 【为什么旧 daemon 整块不显示】daemon 的 capabilities 里没有 `runtime.v1` 时,
 * 这些 RPC 一律 UNIMPLEMENTED。摆一排点了必然没反应的开关,比不摆更糟
 * (规格 §21.3:无能力不显示假入口)。
 *
 * 【为什么不做成"点了进二级页"】设置页的骨架是 LazyColumn + item(key=...),
 * 二级页脚手架 [SubPageScaffold] 是 file-private 的,别的文件用不了。本区段自带
 * 折叠:降级横幅**始终露出**(那是警告,不该藏在折叠里),细节展开才拉。
 */
@Composable
fun RuntimeSection() {
    val caps by AgentConnection.hostCapabilities.collectAsState()
    if (runtimeControlCapable(caps)) {
        RuntimeSectionBody()
    }
}

@Composable
private fun RuntimeSectionBody() {
    val loadState = RuntimeControlState.loadState
    val sectionErrors = RuntimeControlState.sectionErrors
    val cards = agentContractCards(RuntimeControlState.overview)
    val banner = recoveryBanner(RuntimeControlState.recovery)
    val budgets = budgetRows(RuntimeControlState.budget?.effectiveGlobal)
    val collabs = collabPolicyRows(RuntimeControlState.collab?.effective)
    val unknownCount = RuntimeControlState.unknownEvents.size
    // NEW = daemon 侧还没被处置过的条数;入口行的计数徽标只数这一档。
    val unknownPending = RuntimeControlState.unknownEvents.count { it.processingStatus == "NEW" }
    val unknownNote = unknownLedgerNote(RuntimeControlState.unknownTotal, RuntimeControlState.unknownDropped)

    // 内存详情/管理弹窗挂在哪张形态卡上(agent id)。提升到区段级:AgentRuntimeDetail
    // 的展开体在卡片折叠时会被移出组合,行级 remember 存不住跨折叠的弹窗状态。
    var memorySheetAgent by remember { mutableStateOf<String?>(null) }

    // 首次进入拉一次。之后靠手动刷新 —— 用户改预算/策略后本地已有 daemon 回传的真值,
    // 不必每次滚动重组都打一轮 RPC。
    LaunchedEffect(Unit) {
        if (RuntimeControlState.overview == null) RuntimeControlState.load()
    }

    // ---- 控制中心(顶部区段):一页看全 daemon 所有可控域,写动作走既有 RPC ----
    // 排在「运行时」区段之上:它是"总览+就地切换",运行时区段是"逐块深查"。
    ControlCenterSection(
        onOpenMemory = { memorySheetAgent = it },
        banner = banner,
        cards = cards,
        sectionErrors = sectionErrors.mapKeys { sectionLabel(it.key) },
    )

    if (RuntimeControlState.dialysisSheetOpen) {
        UnknownEventSheet(onDismiss = { RuntimeControlState.dialysisSheetOpen = false })
    }

    memorySheetAgent?.let { agentId ->
        ThreadMemorySheet(agentId = agentId, onDismiss = { memorySheetAgent = null })
    }
}

/**
 * 控制中心身份行 = 既有事实串(主机名 · 版本 · 运行时长)+ 系统标签。
 * 【为什么系统并入这里】L2 真机挑刺:"windows · 数据目录:LOCALAPPDATA" 孤悬成行
 * 且数据目录对用户没有决策价值;系统标签归入身份行(尾段),路径行整行删除。
 */
private fun controlPlaneIdentityLine(plane: ControlPlane, nowMs: Long): String =
    listOfNotNull(
        controlPlaneEntryDesc(plane, nowMs).takeIf { it.isNotBlank() },
        plane.os.takeIf { it.isNotBlank() },
    ).joinToString(" · ")

// ================================================================================
// 控制中心(GetControlPlane):一页看全 daemon 全部可控域,就地切换。
//
// 【为什么是"域行 + 就地展开"而不是二级页】设置页骨架是 LazyColumn + item,二级页
// 脚手架是别处 file-private 的;域的量级(8 个)在折叠列表里一屏放得下,展开才看项,
// 与「运行时」区段的折叠语言一致。
//
// 【降级】老 daemon(GetControlPlane 回 UNIMPLEMENTED)整个区段不渲染 —— 摆一个
// 点了必然空着的"控制中心",比不摆更糟;状态层的 [ControlPlaneState.unsupported]
// 记住这份版本事实。单域 status_code=2 时文案就是 daemon 的 summary 原文,红点只是
// 小面积档位标记 —— 客户端不自行把 2 渲染成"故障"。
//
// 【写通道】toggle(collab 域)走既有 SetCollabPolicy、number(budget/collab 域)走
// 既有 SetRuntimePolicy/SetCollabPolicy、agents 桌面偏好 select 走既有 SetCodexSettings,
// 路由与参数由 ControlPlaneLogic 的纯函数钉死;其余项只读呈现 —— 没接通的写通道
// 不摆入口。写成功后回读 GetControlPlane,不做乐观改写 —— "看到的就是生效的"由
// daemon 保证。
// ================================================================================
@Composable
private fun ControlCenterSection(
    onOpenMemory: (String) -> Unit,
    banner: RecoveryBanner?,
    cards: List<AgentContractCard>,
    sectionErrors: Map<String, String>,
) {
    val plane = ControlPlaneState.plane
    val unsupported = ControlPlaneState.unsupported
    val refreshError = ControlPlaneState.refreshError
    val loadState = ControlPlaneState.loadState
    val rows = controlDomainRows(plane)
    var expanded by remember { mutableStateOf(false) }
    var expandedDomain by remember { mutableStateOf<String?>(null) }
    var expandedAgent by remember { mutableStateOf<String?>(null) }
    var editingNumber by remember { mutableStateOf<ControlItemRow?>(null) }
    var editingSelect by remember { mutableStateOf<ControlItemRow?>(null) }
    // 写动作失败原因(daemon 原文)。落在本区段内呈现,与刷新失败分开,用户才分得清
    // "是没改成"还是"没读到"。
    var writeError by remember { mutableStateOf<String?>(null) }
    val scope = rememberCoroutineScope()

    // 首次进入拉一次;之后靠手动刷新与"写后回读"(与运行时区段同款纪律,不随滚动
    // 重组反复打 RPC)。老 daemon 的 UNIMPLEMENTED 由状态层记成事实,这里据此隐身。
    LaunchedEffect(Unit) {
        if (ControlPlaneState.plane == null && !ControlPlaneState.unsupported) {
            ControlPlaneState.refresh()
        }
    }

    // 老 daemon:连入口都不摆(能力缺席的确定信号,不是失败)。
    if (unsupported) return

    // 【区块头自带「重新读取」】刷新入口原先孤悬在展开体底部(L2 真机挑刺:收起时
    // 根本够不到,看着也像一行没归属的浮字),挪到区块标题行尾 —— 与主页「全部 →」
    // 「管理」同款语言,收起/展开都可达。
    Column(Modifier.fillMaxWidth()) {
        Row(
            Modifier.padding(start = 2.dp, top = 16.dp, bottom = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Icon(
                painterResource(R.drawable.ic_lucide_server), null,
                tint = PaEditorial.Muted,
                modifier = Modifier.size(14.dp),
            )
            Spacer(Modifier.width(7.dp))
            Text(
                "控制中心",
                Modifier.weight(1f),
                fontSize = 11.sp,
                lineHeight = 16.sp,
                letterSpacing = 1.2.sp,
                fontWeight = FontWeight(650),
                color = PaEditorial.Muted,
            )
            Text(
                "重新读取",
                Modifier
                    .pressable {
                        writeError = null
                        ControlPlaneState.refresh()
                    }
                    .padding(horizontal = 6.dp, vertical = 2.dp),
                fontSize = 13.sp,
                color = PaEditorial.Primary,
            )
        }
        // 统一卡型(编辑式):18dp 圆角 + 瓷白实色底 + 1dp 细线描边
        // (SettingsScreen.paSettingsCard 是别文件 private,这里内联同款材质)。
        Column(
            Modifier
                .fillMaxWidth()
                .background(PaEditorial.Surface, RoundedCornerShape(18.dp))
                .border(1.dp, PaEditorial.Hairline, RoundedCornerShape(18.dp))
                .padding(horizontal = 16.dp, vertical = 2.dp),
        ) {
        // ---- 入口行:人话标题 + 身份事实(版本/主机名/运行时长/系统) ----
        SettingRow(
            title = "电脑控制台",
            desc = when {
                plane != null -> controlPlaneIdentityLine(plane, System.currentTimeMillis())
                loadState == RuntimeLoadState.LOADING -> "读取中…"
                else -> refreshError ?: "没读到电脑端的控制面"
            },
            chevron = true,
            divider = expanded,
            onClick = { expanded = !expanded },
        )

        AnimatedVisibility(
            visible = expanded,
            enter = expandVertically(PaMotion.sheet()) + fadeIn(PaMotion.sheet()),
            exit = shrinkVertically(PaMotion.standard()) + fadeOut(PaMotion.standard()),
        ) {
            Column(Modifier.padding(top = 8.dp, bottom = 4.dp)) {
                // 【数据目录行删除】"windows · 数据目录:LOCALAPPDATA" 孤悬一行且对用户
                // 无决策价值(L2 真机挑刺);系统标签并入上方身份行,路径类事实不再单独占行。
                // ---- 启动恢复横幅:降级时始终可见(警告不折叠) ----
                if (banner != null && banner.degraded) {
                    RecoveryBannerBlock(banner)
                }
                // ---- 分块读取失败:逐块说清,不整页清空 ----
                sectionErrors.forEach { (k, v) ->
                    SectionNote("$k: $v", EditorialTone.ATTENTION.accent)
                }
                // ---- 每 agent 形态卡(运行时合同核验结果,总览域行只给计数) ----
                cards.forEach { card ->
                    val open = expandedAgent == card.agentId
                    SettingRow(
                        title = card.title,
                        desc = card.versionLine,
                        value = card.note ?: if (card.ok) "契约达标" else "${card.issues.size} 项未交代",
                        chevron = true,
                        divider = true,
                        onClick = { expandedAgent = if (open) null else card.agentId },
                    )
                    if (open) {
                        AgentRuntimeDetail(card) { onOpenMemory(card.agentId) }
                    }
                }
                rows.forEachIndexed { i, domain ->
                    ControlDomainBlock(
                        domain = domain,
                        expanded = expandedDomain == domain.id,
                        divider = i < rows.lastIndex,
                        onToggleExpand = {
                            expandedDomain = if (expandedDomain == domain.id) null else domain.id
                        },
                        onOpenDialysis = { RuntimeControlState.dialysisSheetOpen = true },
                        onOpenMemory = onOpenMemory,
                        onEditNumber = { editingNumber = it },
                        onEditSelect = { editingSelect = it },
                        onToggle = { item, on ->
                            scope.launch {
                                ControlPlaneState.applyToggle(item.domainId, item.id, on)
                                    .onSuccess { writeError = null }
                                    .onFailure { writeError = it.message }
                            }
                        },
                        // 透析域头部 B 开关的写失败与展开体内的开关共用同一个失败位:
                        // 用户分得清"没改成"还是"没读到",原因也只有一个落点。
                        onToggleUnknown = { err -> writeError = err },
                    )
                }
                writeError?.let {
                    SectionNote(it, EditorialTone.ATTENTION.accent)
                }
            }
        }
        // 【收尾顺序】内层展开列 → AnimatedVisibility → 统一卡列 → 区块根列;
        // 底部孤悬的「重新读取」行已删(挪到区块头,见上)。
        }
    }

    editingNumber?.let { item ->
        ControlNumberEditDialog(
            item = item,
            onDismiss = { editingNumber = null },
            onResult = {
                editingNumber = null
            },
        )
    }

    editingSelect?.let { item ->
        ControlSelectEditDialog(
            item = item,
            onDismiss = { editingSelect = null },
        )
    }
}

/**
 * 域行的状态圆点:0 灰 / 1 黄 / 2 红 —— 语义色只用在 8dp 小圆点上(小面积允许)。
 */
@Composable
private fun ControlStatusDot(kind: ControlDotKind) {
    val color = when (kind) {
        ControlDotKind.NORMAL -> PaEditorial.Subtle
        ControlDotKind.ATTENTION -> EditorialTone.ATTENTION.accent
        ControlDotKind.LIMITED -> PaEditorial.Denied
    }
    Box(
        Modifier
            .padding(start = 8.dp)
            .size(8.dp)
            .background(color, CircleShape),
    )
}

/**
 * 控制中心域行(专用排版):title 15sp + summary 12sp + 行尾状态点/箭头。
 *
 * 【与通用 SettingRow 的差异】summary 在 weight(1f) 文字列内独占剩余宽,行尾
 * 状态点是固定小圆点不参与挤压;行距收紧 2dp;字阶降一档 —— 三件事都来自
 * L2 真机对长 summary 域行(资源预算)的挑刺。结构上仍守统一行型:无边无影,
 * 分隔线 0.5dp Separator,点击整行 pressable(与设置页行同手感)。
 *
 * 【trailing 插槽(直控开关位)】部分域(协同/未知事件)在行尾、状态点之前摆一个
 * 直控 Switch —— 用户不进展开体就能碰总闸。插槽消费自己的点击,整行的 pressable
 * (展开/弹窗)不受影响;null = 该域没有直控开关,版面与改造前完全一致。
 */
@Composable
private fun ControlDomainHeaderRow(
    title: String,
    summary: String,
    dotKind: ControlDotKind,
    divider: Boolean,
    chevron: Boolean,
    onClick: (() -> Unit)?,
    trailing: (@Composable () -> Unit)? = null,
) {
    Column {
        Row(
            Modifier
                .fillMaxWidth()
                .then(if (onClick != null) Modifier.pressable(haptic = true) { onClick() } else Modifier)
                .padding(vertical = 11.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Column(Modifier.weight(1f)) {
                Text(title, fontSize = 15.sp, color = PaEditorial.Ink, maxLines = 1)
                // 行距收紧 2dp(通用行 top=2 → 0):summary 紧贴 title 成一块
                Text(
                    summary,
                    Modifier.fillMaxWidth(),
                    fontSize = 12.sp,
                    color = PaEditorial.Muted,
                    maxLines = 2,
                )
            }
            trailing?.invoke()
            ControlStatusDot(dotKind)
            if (chevron) {
                Spacer(Modifier.width(6.dp))
                Icon(
                    painterResource(R.drawable.ic_lucide_chevron_right), null,
                    tint = PaEditorial.Subtle,
                    modifier = Modifier.size(15.dp),
                )
            }
        }
        if (divider) {
            Box(Modifier.fillMaxWidth().height(1.dp).background(PaEditorial.Hairline))
        }
    }
}

/**
 * 一个功能域:行 = title + summary 原文 + 状态圆点;域内有接通写通道的项就地展开 items。
 *
 * 【两类整行直达的特例】dialysis 与 memory 的纵深内容(逐条处置/按会话清除)已经
 * 在既有弹窗里,总览页整行点开弹窗即可,不重复摊一层 —— dialysis 弹窗状态在
 * [RuntimeControlState] 共享,memory 弹窗需要具体 agentId(取第一个按 agent 的
 * 记忆项),还没有 Agent 接入时禁用整行,不盲开一个空弹窗。
 *
 * 【展开判定按域内可写项动态化】daemon 的 domain.writable 是域级声明("有通道"),
 * 客户端只对**接通的**通道摆入口([controlDomainExpandable]):全只读的域
 * (纯事实项,或通道尚未接通)不摆展开箭头 —— 展开体里一行可写项都没有的展开,
 * 是个假编辑区;这类域的事实已由 summary 或上方形态卡带出,不丢信息。
 */
@Composable
private fun ControlDomainBlock(
    domain: ControlDomainRow,
    expanded: Boolean,
    divider: Boolean,
    onToggleExpand: () -> Unit,
    onOpenDialysis: () -> Unit,
    onOpenMemory: (String) -> Unit,
    onEditNumber: (ControlItemRow) -> Unit,
    onEditSelect: (ControlItemRow) -> Unit,
    onToggle: (ControlItemRow, Boolean) -> Unit,
    onToggleUnknown: (String?) -> Unit,
) {
    val memoryAgent = if (domain.id == CONTROL_DOMAIN_MEMORY) memoryAgentTargetOf(domain.items) else null
    val onClick: (() -> Unit)? = when (domain.id) {
        CONTROL_DOMAIN_DIALYSIS -> onOpenDialysis
        CONTROL_DOMAIN_MEMORY -> memoryAgent?.let { { onOpenMemory(it) } }
        else -> if (controlDomainExpandable(domain.items)) onToggleExpand else null
    }
    // 【域行头部直控开关(2026-09-30 双端直控)】协同域头部摆协同总开关、未知事件
    // 域头部摆未知请求透传 B 开关 —— 不进展开体/二级编辑就能碰总闸。数据取值与
    // 可用性由 ControlDirectSwitchLogic 的纯函数钉死(现值认不出/快照缺席/提交中
    // 一律禁用);开关本体见 [CollabMasterSwitch] / [DialysisUnknownSwitch]。
    val headerTrailing: (@Composable () -> Unit)? = when (domain.id) {
        CONTROL_DOMAIN_COLLAB -> {
            val item = collabMasterSwitchItem(domain)
            // @Composable lambda 字面量直接搁在 if/when 表达式位 K2 推断不出类型,
            // 显式类型局部值承转(fa9ec7b 曾因此整包编译不过)。
            if (item == null) null
            else {
                val block: @Composable () -> Unit = { CollabMasterSwitch(item = item, onToggle = onToggle) }
                block
            }
        }
        CONTROL_DOMAIN_DIALYSIS -> {
            val block: @Composable () -> Unit = { DialysisUnknownSwitch(onWriteError = onToggleUnknown) }
            block
        }
        else -> null
    }
    // 【域行专用排版,不走通用 SettingRow】L2 真机挑刺(06-control-plane-full):
    // summary 长文案被行尾状态点/箭头挤压、title 与 summary 行距偏松、字阶偏大。
    // 收成专用行 —— title/summary 独占 weight(1f) 剩余宽(状态点是行尾固定小点,
    // 不进文字流),行距收紧 2dp(通用行 top=2 → 0),字阶 title 15sp / summary 12sp,
    // 与设置页通用行(16sp/13sp)拉开一档,控制中心内部层级更清楚。
    ControlDomainHeaderRow(
        title = domain.title,
        summary = domain.summary,
        dotKind = domain.dotKind,
        divider = divider,
        chevron = onClick != null,
        onClick = onClick,
        trailing = headerTrailing,
    )
    // memory 行点不动 = 只有合计项(还没有 Agent 接入);补一句事实说明为什么不跳。
    if (domain.id == CONTROL_DOMAIN_MEMORY && memoryAgent == null && domain.items.isNotEmpty()) {
        Text(
            "还没有已接入的 Agent,合计显示在上面的摘要里",
            Modifier.padding(start = 2.dp, top = 2.dp),
            fontSize = 11.sp,
            color = PaEditorial.Muted,
        )
    }
    AnimatedVisibility(
        visible = expanded,
        enter = expandVertically(PaMotion.sheet()) + fadeIn(PaMotion.sheet()),
        exit = shrinkVertically(PaMotion.standard()) + fadeOut(PaMotion.standard()),
    ) {
        Column(Modifier.padding(bottom = 4.dp)) {
            domain.items.forEachIndexed { i, item ->
                ControlItemLine(
                    item = item,
                    enabled = ControlPlaneState.busyItemId == null,
                    onEditNumber = onEditNumber,
                    onEditSelect = onEditSelect,
                    onToggle = onToggle,
                )
                if (i < domain.items.lastIndex) {
                    Box(Modifier.fillMaxWidth().height(1.dp).background(PaEditorial.Hairline))
                }
            }
        }
    }
}

/**
 * 直控 Switch 的统一色板(与展开体内 ControlItemLine 的开关同款):
 * 选中轨道 = 主色,未选 = 抬升面 + 细线描边。三处开关一个色,控制中心内部不分叉。
 */
@Composable
private fun controlSwitchColors() = androidx.compose.material3.SwitchDefaults.colors(
    checkedTrackColor = PaEditorial.Primary,
    uncheckedTrackColor = PaEditorial.Raised,
    uncheckedBorderColor = PaEditorial.Hairline,
)

/**
 * 协同域头部的协同总开关直控(2026-09-30 双端直控波次)。
 *
 * 【写通道】与展开体内的同类开关同一条:[ControlPlaneState.applyToggle] → 既有
 * SetCollabPolicy(先 Get 再完整 Set + CAS),键 → 字段映射在 applyControlToggle
 * (纯函数,单测钉死);成功后回读 GetControlPlane,开关位置由 daemon 下一份快照
 * 决定 —— 不做乐观改写。
 *
 * 【状态失真时如实】现值认不出 / 提交中 → 禁用([collabMasterSwitchState] 钉死);
 * daemon 快照里没有该项 → 头部根本不摆开关(装配点判空)。
 */
@Composable
private fun CollabMasterSwitch(
    item: ControlItemRow,
    onToggle: (ControlItemRow, Boolean) -> Unit,
) {
    val state = collabMasterSwitchState(item, ControlPlaneState.busyItemId != null)
    Switch(
        checked = state.checked,
        enabled = state.enabled,
        onCheckedChange = { on -> onToggle(item, on) },
        colors = controlSwitchColors(),
        modifier = Modifier
            .padding(start = 6.dp)
            .semantics { contentDescription = "协同总开关" },
    )
}

/**
 * 未知事件域头部的未知请求透传 B 开关(同语义直控,2026-09-30 双端直控波次)。
 *
 * 【为什么在透析域头部】该开关决定"未识别的反向请求要不要先来手机上确认",
 * 归宿就是消息透析;设置页实验区已有同款入口(SplitButton),这里是同数据的
 * 快捷位 —— 数据源与写通道完全同一条:读 RuntimeControlState.budget.policy 的
 * unknown_request_passthrough,写走 [applyUnknownPassthroughToggle](既有
 * SetRuntimePolicy 完整策略 + CAS,成功后回读控制中心)。
 *
 * 【加载中禁用】预算快照缺席(还没读到/上次读取失败)时禁用 —— 把"没读到"
 * 画成"是关的"且可点,用户一按就可能写反;等快照在手再启用,如实。
 * busy 状态本地持有(本开关的提交不经过 busyItemId 通道),提交期间禁用防连点。
 */
@Composable
private fun DialysisUnknownSwitch(onWriteError: (String?) -> Unit) {
    val scope = rememberCoroutineScope()
    var busy by remember { mutableStateOf(false) }
    val state = unknownPassthroughSwitchState(
        budget = RuntimeControlState.budget,
        busy = busy || ControlPlaneState.busyItemId != null,
    )
    Row(verticalAlignment = Alignment.CenterVertically) {
        Text("透传", fontSize = 11.sp, color = PaEditorial.Muted)
        Switch(
            checked = state.checked,
            enabled = state.enabled,
            onCheckedChange = { on ->
                busy = true
                scope.launch {
                    onWriteError(applyUnknownPassthroughToggle(on))
                    busy = false
                }
            },
            colors = controlSwitchColors(),
            modifier = Modifier
                .padding(start = 2.dp)
                .semantics { contentDescription = "未知请求透传开关(未识别操作确认)" },
        )
    }
}

/**
 * 域内一项:toggle → Switch(现值认不出时走只读分支,显示原文不猜布尔);
 * budget/collab number → 点行弹小输入框;agents 桌面偏好 select → 点行弹候选弹窗
 * (候选只列 daemon 给的 options);其余(info / 未接写通道)只读 label + 现值原文。
 * hint 一律 12sp TextSecondary 垫在项下 —— daemon 解释"改动影响什么"的话,照抄。
 */
@Composable
private fun ControlItemLine(
    item: ControlItemRow,
    enabled: Boolean,
    onEditNumber: (ControlItemRow) -> Unit,
    onEditSelect: (ControlItemRow) -> Unit,
    onToggle: (ControlItemRow, Boolean) -> Unit,
) {
    Column(Modifier.fillMaxWidth().padding(vertical = 4.dp)) {
        when {
            item.action == ControlItemAction.COLLAB_TOGGLE && controlToggleOn(item.current) != null -> {
                Row(
                    Modifier.fillMaxWidth(),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(item.label, Modifier.weight(1f), fontSize = 14.sp, color = PaEditorial.Ink)
                    Switch(
                        checked = controlToggleOn(item.current) == true,
                        enabled = enabled,
                        onCheckedChange = { on -> onToggle(item, on) },
                        colors = controlSwitchColors(),
                    )
                }
            }
            item.action == ControlItemAction.BUDGET_NUMBER || item.action == ControlItemAction.COLLAB_NUMBER -> {
                Row(
                    Modifier
                        .fillMaxWidth()
                        .pressable(haptic = true) { onEditNumber(item) },
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(item.label, Modifier.weight(1f), fontSize = 14.sp, color = PaEditorial.Ink)
                    Text(item.current, fontSize = 13.sp, color = PaEditorial.Muted)
                    Spacer(Modifier.width(4.dp))
                    Icon(
                        painterResource(R.drawable.ic_lucide_chevron_right),
                        contentDescription = null,
                        tint = PaEditorial.Subtle,
                        modifier = Modifier.size(14.dp),
                    )
                }
            }
            item.action == ControlItemAction.AGENTS_SELECT -> {
                Row(
                    Modifier
                        .fillMaxWidth()
                        .pressable(haptic = true) { onEditSelect(item) },
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(item.label, Modifier.weight(1f), fontSize = 14.sp, color = PaEditorial.Ink)
                    Text(item.current, fontSize = 13.sp, color = PaEditorial.Muted)
                    Spacer(Modifier.width(4.dp))
                    Icon(
                        painterResource(R.drawable.ic_lucide_chevron_right),
                        contentDescription = null,
                        tint = PaEditorial.Subtle,
                        modifier = Modifier.size(14.dp),
                    )
                }
            }
            else -> {
                Row(
                    Modifier.fillMaxWidth(),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(item.label, Modifier.weight(1f), fontSize = 14.sp, color = PaEditorial.Ink)
                    Text(
                        item.current,
                        fontSize = 13.sp,
                        color = PaEditorial.Muted,
                        maxLines = 2,
                        overflow = TextOverflow.Ellipsis,
                    )
                }
            }
        }
        if (item.hint.isNotBlank()) {
            Text(
                item.hint,
                Modifier.padding(top = 2.dp),
                fontSize = 12.sp,
                color = PaEditorial.Muted,
            )
        }
    }
}

/**
 * 控制中心数值编辑(budget/collab 两域共用;输入口径 MB/计数,单位由
 * [controlNumberUnit] 按键给)。
 *
 * 【为什么自己写而不直接复用 BudgetEditDialog】那一个把键入值**原样**写进字段
 * (MB 标签、字节存储),单位差 2^20 倍;控制中心这里在写路由里换算
 * ([applyControlBudgetNumber] 字节键 ×1MB;collab 成员上限是计数,不过它)。
 * 预填沿用 [budgetInputValue] 的光标纪律(在末尾,防"预填 3 敲 4 得 43");现值在
 * 框上方照原文显示 —— 预填不上(KB/B/非整数 MB)时用户有原文可对。
 * 提交按 [ControlItemRow.domainId] 路由(item 自带所属域,调用方不再猜域)。
 */
@Composable
private fun ControlNumberEditDialog(
    item: ControlItemRow,
    onDismiss: () -> Unit,
    onResult: () -> Unit,
) {
    val unit = controlNumberUnit(item.id)
    var text by remember { mutableStateOf(budgetInputValue(controlNumberPrefill(item.current))) }
    var error by remember { mutableStateOf<String?>(null) }
    val scope = rememberCoroutineScope()

    AlertDialog(
        onDismissRequest = onDismiss,
        containerColor = PaEditorial.Surface,
        title = { Text(item.label, fontSize = 16.sp, color = PaEditorial.Ink) },
        text = {
            Column {
                Text("当前:${item.current}", fontSize = 12.5.sp, color = PaEditorial.Muted)
                Spacer(Modifier.height(8.dp))
                OutlinedTextField(
                    value = text,
                    // 只保留数字;光标位置由 TextFieldValue 自带(见 [budgetInputValue])。
                    onValueChange = { v -> text = v.copy(text = v.text.filter { c -> c.isDigit() }) },
                    singleLine = true,
                    label = { Text("新上限($unit)", fontSize = 13.sp) },
                )
                if (error != null) {
                    Spacer(Modifier.height(8.dp))
                    Text(error!!, fontSize = 12.sp, color = EditorialTone.DENIED.text)
                }
            }
        },
        confirmButton = {
            TextButton(onClick = {
                val n = text.text.toLongOrNull()
                if (n == null) {
                    error = "请填一个非负整数"
                    return@TextButton
                }
                scope.launch {
                    ControlPlaneState.applyNumber(item.domainId, item.id, n)
                        .onSuccess { onResult() }
                        .onFailure { error = it.message }
                }
            }) { Text("保存", color = PaEditorial.Primary) }
        },
        dismissButton = {
            TextButton(onClick = onDismiss) { Text("取消", color = PaEditorial.Muted) }
        },
    )
}

/**
 * agents 桌面偏好的选择弹窗(SetCodexSettings 的 select 写通道)。
 *
 * 【候选为什么只列 daemon 给的 options】值域是 daemon 的契约,客户端抄一份本地词表,
 * daemon 升级换词表后两处就会漂移 —— 候选展示与写入校验([controlAgentsOptionInvalid])
 * 共用同一份 options,一个不猜。
 *
 * 【预选纪律】现值原文恰好在候选里(字符串相等)才预选;对不上就不预选、保存禁用,
 * 让用户明选 —— 现值显示与候选词表对不上时,猜一个"最像的"等于替用户做了一次写。
 * 提交按 [ControlItemRow.domainId] 路由;失败原因落在本弹窗内呈现(与刷新失败分开)。
 */
@Composable
private fun ControlSelectEditDialog(
    item: ControlItemRow,
    onDismiss: () -> Unit,
) {
    var selected by remember { mutableStateOf(item.current.takeIf { it in item.options }) }
    var error by remember { mutableStateOf<String?>(null) }
    val scope = rememberCoroutineScope()

    AlertDialog(
        onDismissRequest = onDismiss,
        containerColor = PaEditorial.Surface,
        title = { Text(item.label, fontSize = 16.sp, color = PaEditorial.Ink) },
        text = {
            Column {
                Text("当前:${item.current}", fontSize = 12.5.sp, color = PaEditorial.Muted)
                Spacer(Modifier.height(8.dp))
                item.options.forEach { opt ->
                    Row(
                        Modifier
                            .fillMaxWidth()
                            .pressable { selected = opt },
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        RadioButton(
                            selected = selected == opt,
                            onClick = { selected = opt },
                        )
                        Text(opt, fontSize = 14.sp, color = PaEditorial.Ink)
                    }
                }
                if (item.hint.isNotBlank()) {
                    Spacer(Modifier.height(4.dp))
                    Text(item.hint, fontSize = 12.sp, color = PaEditorial.Muted)
                }
                if (error != null) {
                    Spacer(Modifier.height(8.dp))
                    Text(error!!, fontSize = 12.sp, color = EditorialTone.DENIED.text)
                }
            }
        },
        confirmButton = {
            TextButton(
                enabled = selected != null,
                onClick = {
                    val v = selected ?: return@TextButton
                    scope.launch {
                        ControlPlaneState.applySelect(item.domainId, item.id, v, item.options)
                            .onSuccess { onDismiss() }
                            .onFailure { error = it.message }
                    }
                },
            ) { Text("保存", color = PaEditorial.Primary) }
        },
        dismissButton = {
            TextButton(onClick = onDismiss) { Text("取消", color = PaEditorial.Muted) }
        },
    )
}

/** 恢复横幅:降级摘要 + **逐条**"哪句没跑到"(只报条数等于没说)。 */
@Composable
private fun RecoveryBannerBlock(banner: RecoveryBanner) {
    Column(
        Modifier
            .fillMaxWidth()
            .padding(top = 8.dp, bottom = 2.dp)
            .background(PaEditorial.DeniedContainer, RoundedCornerShape(13.dp))
            .padding(12.dp),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Icon(
                painterResource(R.drawable.ic_lucide_triangle_alert),
                contentDescription = null,
                tint = EditorialTone.DENIED.text,
                modifier = Modifier.size(15.dp),
            )
            Spacer(Modifier.width(8.dp))
            Text(banner.headline, fontSize = 13.sp, color = EditorialTone.DENIED.text)
        }
        if (banner.lostLines.isNotEmpty()) {
            Spacer(Modifier.height(8.dp))
            Text(
                "上次退出时没轮到的输入(共 ${banner.lostLines.size} 句,涉及 ${banner.threadCount} 个会话)",
                fontSize = 12.sp,
                color = PaEditorial.Muted,
            )
            banner.lostLines.forEach { line ->
                Text(
                    line,
                    Modifier.padding(top = 4.dp),
                    fontSize = 12.5.sp,
                    color = PaEditorial.Ink,
                )
            }
        }
    }
}

/**
 * 形态卡的展开体:六项形态 + 违约 + 显式不支持 + 该 agent 当前资源租约
 * + §9.5 后四块用量(记忆 / 缓存 / 检查点 / 恢复)。
 *
 * 【四块用量为什么排在资源租约之后】§9.5 的原文顺序是 memory/cache/resource/
 * checkpoint/recovery,但资源租约的成败(网络失败时显示 note)同时是四块能否取到
 * 的前置信号 —— 读不到时四块整段缺席,排在前面的失败原因正好解释这件事。
 *
 * 【记忆行为什么是入口】记忆用量有下钻内容(按会话清单 + 清除动作,见
 * [ThreadMemorySheet]),其余三块只是数值。入口只绑在 key=="memory" 的行上 ——
 * 那是数据层钉过的锚点:daemon 真给了 memory 块才有 key,缺席时没有假入口。
 */
@Composable
private fun AgentRuntimeDetail(card: AgentContractCard, onOpenMemory: () -> Unit) {
    var resources by remember(card.agentId) { mutableStateOf<List<String>?>(null) }
    var resourceNote by remember(card.agentId) { mutableStateOf<String?>(null) }
    var usage by remember(card.agentId) { mutableStateOf<List<RuntimeUsageRow>>(emptyList()) }

    LaunchedEffect(card.agentId) {
        RuntimeControlState.agentRuntime(card.agentId)
            .onSuccess { r ->
                resources = r.resourcesList.map { m ->
                    val used = formatBytesLabel(m.usedBytes)
                    val hard = formatBytesLabel(m.hardLimitBytes)
                    "${m.kind} · $used / $hard${if (m.optional) "(可选)" else ""}"
                }
                resourceNote = null
                usage = agentRuntimeUsageRows(r)
            }
            .onFailure { resourceNote = it.message ?: it::class.java.simpleName }
    }

    Column(
        Modifier
            .fillMaxWidth()
            .padding(start = 12.dp, bottom = 8.dp)
            .background(PaEditorial.Raised, RoundedCornerShape(12.dp))
            .padding(horizontal = 12.dp, vertical = 8.dp),
    ) {
        card.rows.forEach { r ->
            DetailLine(r.label, r.value)
        }
        card.issues.forEach { DetailLine("未交代", it, EditorialTone.DENIED.text) }
        card.limitations.forEach { DetailLine("明确不支持", it, PaEditorial.Muted) }
        when {
            resourceNote != null -> DetailLine("资源", resourceNote!!, PaEditorial.Muted)
            resources?.isEmpty() == true -> DetailLine("资源", "当前没有活跃的资源租约")
            resources != null -> resources!!.forEach { DetailLine("资源", it) }
        }
        usage.forEach { row ->
            if (row.key == "memory") {
                MemoryUsageEntryRow(row = row, onOpen = onOpenMemory)
            } else {
                DetailLine(row.label, row.value)
            }
        }
    }
}

/**
 * 「记忆用量」入口行:可点进内存详情/管理弹窗。
 *
 * 【降级是事实陈述】旧 daemon 没有内存管理两条 RPC(listThreadMemories 会回
 * UNIMPLEMENTED,数据层已把它记成 [RuntimeControlState.memoriesUnsupported]):
 * 此时入口灰显(不可点、无 chevron),补一句"此 daemon 版本未提供内存管理" ——
 * 用量数值照常显示(那条 RPC 属于更早的版本,仍然可用)。不写"未知/不支持错误"
 * 这类猜测词,只陈述版本事实。
 */
@Composable
private fun MemoryUsageEntryRow(row: RuntimeUsageRow, onOpen: () -> Unit) {
    val unsupported = RuntimeControlState.memoriesUnsupported
    if (unsupported) {
        DetailLine(row.label, row.value)
        DetailLine("内存管理", "此 daemon 版本未提供内存管理", PaEditorial.Muted)
        return
    }
    Row(
        Modifier
            .fillMaxWidth()
            .pressable(haptic = true) { onOpen() }
            .padding(vertical = 2.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text("记忆用量", fontSize = 12.sp, color = PaEditorial.Muted, modifier = Modifier.width(72.dp))
        Text(
            row.value,
            fontSize = 12.5.sp,
            color = PaEditorial.Ink,
            modifier = Modifier.weight(1f),
        )
        Icon(
            painterResource(R.drawable.ic_lucide_chevron_right),
            contentDescription = null,
            tint = PaEditorial.Subtle,
            modifier = Modifier.size(13.dp),
        )
    }
}

@Composable
private fun DetailLine(label: String, value: String, color: androidx.compose.ui.graphics.Color = PaEditorial.Ink) {
    Row(Modifier.fillMaxWidth().padding(vertical = 2.dp)) {
        Text(label, fontSize = 12.sp, color = PaEditorial.Muted, modifier = Modifier.width(72.dp))
        Text(value, fontSize = 12.5.sp, color = color, modifier = Modifier.weight(1f))
    }
}

@Composable
private fun SubHeader(text: String) {
    Text(
        text,
        Modifier.padding(start = 2.dp, top = 12.dp, bottom = 2.dp),
        fontSize = 12.sp,
        color = PaEditorial.Muted,
        fontWeight = FontWeight.Medium,
    )
}

@Composable
private fun SectionNote(text: String, color: androidx.compose.ui.graphics.Color) {
    Text(
        text,
        Modifier.fillMaxWidth().padding(vertical = 6.dp, horizontal = 2.dp),
        fontSize = 12.sp,
        color = color,
    )
}

/** 分块失败原因补记(写回数据层,与 load 的分块错误同一处呈现)。 */
private fun sectionErrorPing(section: String, message: String) {
    RuntimeControlState.noteSectionError(section, message)
}

internal fun sectionLabel(key: String): String = when (key) {
    RuntimeControlState.SECTION_HOST -> "启动恢复"
    RuntimeControlState.SECTION_STATE -> "本机状态"
    RuntimeControlState.SECTION_BUDGET -> "预算"
    RuntimeControlState.SECTION_COLLAB -> "协同策略"
    RuntimeControlState.SECTION_UNKNOWN -> "新事件"
    else -> key
}

/**
 * 预算项编辑。
 *
 * 【为什么走"Get → 改 → Set"】这不是 UI 的选择,而是契约的硬约束:Set 提交的是
 * **完整策略**,缺席项会被重置回默认(实测踩到过:只关扇出把成员上限从 2 变回"不限")。
 * 所以本对话框只改一项,其余项原样带回 —— 由 [RuntimeControlState.saveBudget] 保证
 * 提交的是从 daemon 读回的那份完整对象,并带 revision 做 CAS。
 */
@Composable
private fun BudgetEditDialog(
    row: BudgetRow,
    onDismiss: () -> Unit,
    onResult: (String?) -> Unit,
) {
    val isTurns = row.key == "max_parallel_turns"
    val isMembers = row.key == "max_parallel_members"
    val unit = if (isTurns) "路" else if (isMembers) "家" else "MB"
    var text by remember { mutableStateOf(budgetInputValue(row.value)) }
    var error by remember { mutableStateOf<String?>(null) }
    val scope = rememberCoroutineScope()

    AlertDialog(
        onDismissRequest = onDismiss,
        containerColor = PaEditorial.Surface,
        title = { Text(row.label, fontSize = 16.sp, color = PaEditorial.Ink) },
        text = {
            Column {
                OutlinedTextField(
                    value = text,
                    // 只保留数字;光标位置由 TextFieldValue 自带(见 [budgetInputValue])。
                    onValueChange = { v -> text = v.copy(text = v.text.filter { c -> c.isDigit() }) },
                    singleLine = true,
                    label = { Text("新上限($unit)", fontSize = 13.sp) },
                )
                if (error != null) {
                    Spacer(Modifier.height(8.dp))
                    Text(error!!, fontSize = 12.sp, color = EditorialTone.DENIED.text)
                }
            }
        },
        confirmButton = {
            TextButton(onClick = {
                val n = text.text.toLongOrNull()
                if (n == null) {
                    error = "请填一个非负整数"
                    return@TextButton
                }
                scope.launch {
                    // 字节类键的输入单位是 MB,必须换算成字节再写(与控制中心
                    // applyControlBudgetNumber 同一口径)。修复存量 bug:旧实现
                    // 把「8 MB」的 8 原样写成 8 字节 —— 与窗口显示的现值差一百万
                    // 倍,而字节字段小值=把该类资源预算砍到几乎为零。
                    val scaled = if (unit == "MB") n shl 20 else n
                    val err = saveRuntimeEdit(row.key, scaled)
                    if (err != null) error = err else onResult(null)
                }
            }) { Text("保存", color = PaEditorial.Primary) }
        },
        dismissButton = {
            Row {
                TextButton(onClick = {
                    scope.launch {
                        val err = saveRuntimeEdit(row.key, null)
                        if (err != null) error = err else onResult(null)
                    }
                }) { Text("恢复默认", color = PaEditorial.Muted) }
                TextButton(onClick = onDismiss) { Text("取消", color = PaEditorial.Muted) }
            }
        },
    )
}

/** 只取现值的数字部分("8 MB" → "8"),避免把单位当输入内容回填。 */
internal fun currentNumeric(display: String): String =
    display.takeWhile { it.isDigit() }

/**
 * 编辑框的初始内容:**预填当前值,且光标放在末尾**。
 *
 * 【为什么必须显式给 selection】用 String 版重载时初始光标落在位置 0 —— 真机实测踩到:
 * 预填 "3" 时直接输入 "4" 得到 **"43"**(插到了旧值前面)。用户以为设了 4、实际设了 43,
 * 而预算被改错既没有编译期错误也没有运行期提示,只有下一轮跑出异常行为才显形。
 */
internal fun budgetInputValue(display: String): TextFieldValue {
    val digits = currentNumeric(display)
    return TextFieldValue(digits, TextRange(digits.length))
}

/**
 * 预算单项写入 / 清除 presence —— `null` 表示"回到未设置(沿用上一级)",与显式 0 不同。
 *
 * 【为什么先取出来改、再整体写回】proto lite 的生成代码里没有"就地取子 builder"
 * (`globalBuilder`)这种访问器;先 `toBuilder()` 出来改、再整体 `setGlobal`,是等价的
 * 写法,而且**每个 key 只动它自己那一项**,其余项原样带回 —— 这正是"提交完整策略"
 * 这条纪律要的效果。
 */
internal fun applyBudgetEdit(b: RuntimePolicy.Builder, key: String, value: Long?) {
    val g = b.global.toBuilder()
    when (key) {
        "ram_working_bytes" -> if (value == null) g.clearRamWorkingBytes() else g.setRamWorkingBytes(value)
        "ram_event_buffer_bytes" -> if (value == null) g.clearRamEventBufferBytes() else g.setRamEventBufferBytes(value)
        "ram_session_index_bytes" -> if (value == null) g.clearRamSessionIndexBytes() else g.setRamSessionIndexBytes(value)
        "disk_conversation_bytes" -> if (value == null) g.clearDiskConversationBytes() else g.setDiskConversationBytes(value)
        "disk_cache_bytes" -> if (value == null) g.clearDiskCacheBytes() else g.setDiskCacheBytes(value)
        "disk_artifact_bytes" -> if (value == null) g.clearDiskArtifactBytes() else g.setDiskArtifactBytes(value)
        "max_parallel_turns" -> if (value == null) g.clearMaxParallelTurns() else g.setMaxParallelTurns(value.toInt())
    }
    b.global = g.build()
}

/**
 * 协同策略单项写入。
 *
 * 【为什么不能和预算共用一个函数】成员上限属于**协同策略**(CollabPolicy),不在预算
 * 树上 —— 两者的落盘文件、revision 与 CAS 各一套。写错对象的表现是"改了但没生效",
 * 而没有任何编译期错误会提醒(除非像现在这样把 key 送进错误的 Builder)。
 */
internal fun applyCollabEdit(b: CollabPolicy.Builder, key: String, value: Long?) {
    if (key == "max_parallel_members") {
        if (value == null) b.clearMaxParallelMembers() else b.setMaxParallelMembers(value.toInt())
    }
}

/** 按 key 归属把编辑提交到正确的策略对象上(预算 / 协同策略是两套独立的 CAS)。 */
private suspend fun saveRuntimeEdit(key: String, value: Long?): String? =
    if (key == "max_parallel_members") {
        RuntimeControlState.saveCollabPolicy { b -> applyCollabEdit(b, key, value) }
    } else {
        RuntimeControlState.saveBudget { b -> applyBudgetEdit(b, key, value) }
    }

/**
 * 消息透析弹窗(PaSheet 统一玻璃浮层):折叠行给"一句话人话",点开一行才展开详情。
 *
 * 【为什么从设置页内嵌行改成弹窗】未知事件的详情是"原文 + 透析表 + 处置动作"的
 * 纵深内容,摊在设置页里既占地方又逼用户滚动;折叠进弹窗后,设置页只剩一个入口行,
 * 手机上一屏就能看完。
 *
 * 【详情为什么按需拉且缓存】列表(summary)很轻,payload/透析表在 GetUnknownEvent
 * 里,首次展开才拉;缓存 keyed by eventId 放在 **sheet 级**而不是行级 remember ——
 * AnimatedVisibility 收起会把行内容移出组合,行级 remember 会被丢掉,变成每次
 * 展开都重拉。sheet 级缓存覆盖"收起再展开"的场景;关掉弹窗即释放,下次打开重拉,
 * 保证看到的是 daemon 当前值。
 *
 * 【paPushBack 说明】推背模糊要挂在被覆盖的内容根上;本区段的宿主内容根在
 * SettingsScreen(其 paPushBack 只认它自己的 showApprovalPreview 状态),入口行
 * 在本文件里够不到那个根 —— 不跨文件改宿主,弹窗本体用 PaSheet 玻璃材质照常渲染。
 */
@Composable
private fun UnknownEventSheet(onDismiss: () -> Unit) {
    // 最新在前:台账顺序 daemon 不承诺,排序规则由 UI 定死,同一份数据每次渲染同序。
    val events = RuntimeControlState.unknownEvents.sortedByDescending { it.receivedAtUnixMs }
    val ledger = unknownLedgerNote(RuntimeControlState.unknownTotal, RuntimeControlState.unknownDropped)
    // 详情缓存(键 = eventId);detailInFlight 去重"快速收起再展开"时的并发重复拉取。
    val details = remember { mutableStateMapOf<String, SheetDetailState>() }
    val detailInFlight = remember { mutableSetOf<String>() }
    var expandedId by remember { mutableStateOf<String?>(null) }
    var rawOpenFor by remember { mutableStateOf<String?>(null) }
    var pendingAck by remember { mutableStateOf<String?>(null) }
    var ackFailure by remember { mutableStateOf<Pair<String, String>?>(null) }
    val scope = rememberCoroutineScope()

    PaSheet(onDismissRequest = onDismiss) {
        Column(Modifier.fillMaxWidth().padding(horizontal = 24.dp)) {
            Text(
                "消息透析",
                fontSize = 19.sp,
                color = PaEditorial.Ink,
                fontWeight = FontWeight.SemiBold,
            )
            ledger?.let {
                Text(it, Modifier.padding(top = 3.dp), fontSize = 11.sp, color = PaEditorial.Muted)
            }
            if (events.isEmpty()) {
                EmptyState(
                    iconRes = R.drawable.ic_lucide_message_square,
                    title = "最近没有需要透析的消息",
                    hint = "电脑端发来新事件后会登记在这里",
                )
            } else {
                Spacer(Modifier.height(4.dp))
                events.forEachIndexed { i, s ->
                    val line = unknownEventLine(s)
                    val open = expandedId == s.eventId
                    UnknownEventRow(
                        summary = s,
                        line = line,
                        open = open,
                        detail = details[s.eventId],
                        rawOpen = rawOpenFor == s.eventId,
                        pendingAck = pendingAck == s.eventId,
                        ackFailure = ackFailure?.takeIf { it.first == s.eventId }?.second,
                        onToggle = {
                            expandedId = if (open) null else s.eventId
                            rawOpenFor = null
                            ackFailure = null
                        },
                        onToggleRaw = { rawOpenFor = if (rawOpenFor == s.eventId) null else s.eventId },
                        onEnsureDetail = {
                            if (!details.containsKey(s.eventId) && detailInFlight.add(s.eventId)) {
                                scope.launch {
                                    val r = RuntimeControlState.unknownEventDetail(s.eventId)
                                    details[s.eventId] = r.fold(
                                        onSuccess = { SheetDetailState(detail = it) },
                                        onFailure = {
                                            SheetDetailState(error = it.message ?: it::class.java.simpleName)
                                        },
                                    )
                                    detailInFlight.remove(s.eventId)
                                }
                            }
                        },
                        onAck = { action ->
                            pendingAck = s.eventId
                            scope.launch {
                                val err = RuntimeControlState.acknowledge(s.eventId, action, "")
                                pendingAck = null
                                // 成功时 unknownEvents 已被 daemon 回传真值就地替换,
                                // 折叠行的状态徽标随重组更新;失败原因落在本行下方。
                                ackFailure = if (err != null) s.eventId to err else null
                            }
                        },
                    )
                    if (i < events.lastIndex) {
                        Box(Modifier.fillMaxWidth().height(1.dp).background(PaEditorial.Hairline))
                    }
                }
            }
            Spacer(Modifier.height(8.dp))
        }
    }
}

/** 详情拉取状态:成功带 [detail],失败带 [error];都为空 = 还在拉。 */
private data class SheetDetailState(
    val detail: UnknownEventResponseOf? = null,
    val error: String? = null,
)

/**
 * 弹窗里的一条事件:折叠态一行(相对时间 + agent 名 + summary 人话),
 * 整行可点展开;展开体 = 分类/状态 + 详细解读 + 字段透析表 + 原文 + 处置动作。
 */
@Composable
private fun UnknownEventRow(
    summary: UnknownEventSummary,
    line: UnknownEventLine,
    open: Boolean,
    detail: SheetDetailState?,
    rawOpen: Boolean,
    pendingAck: Boolean,
    ackFailure: String?,
    onToggle: () -> Unit,
    onToggleRaw: () -> Unit,
    onEnsureDetail: () -> Unit,
    onAck: (String) -> Unit,
) {
    Column(Modifier.fillMaxWidth()) {
        // ---- 折叠态:一行人话 ----
        Column(
            Modifier
                .fillMaxWidth()
                .pressable(onClick = onToggle)
                .padding(vertical = 10.dp),
        ) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                if (line.relativeTime.isNotEmpty()) {
                    Text(line.relativeTime, fontSize = 11.sp, color = PaEditorial.Muted)
                    Spacer(Modifier.width(8.dp))
                }
                if (line.sourceAgent.isNotBlank()) {
                    Text(
                        line.sourceAgent,
                        fontSize = 11.sp,
                        color = PaEditorial.Muted,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                    Spacer(Modifier.width(8.dp))
                }
                Text(
                    // summary 有 = daemon 的人话;无 = 回落方法名原文(降级说明在下一行)。
                    line.summary ?: line.title,
                    Modifier.weight(1f),
                    fontSize = 13.sp,
                    color = PaEditorial.Ink,
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis,
                )
                Spacer(Modifier.width(6.dp))
                Icon(
                    painterResource(
                        if (open) R.drawable.ic_lucide_chevron_up else R.drawable.ic_lucide_chevron_down
                    ),
                    contentDescription = null,
                    tint = PaEditorial.Subtle,
                    modifier = Modifier.size(14.dp),
                )
            }
            line.degradedNote?.let {
                Text(it, Modifier.padding(top = 3.dp), fontSize = 11.sp, color = PaEditorial.Muted)
            }
        }

        // ---- 展开态:spring 驱动、可打断(连点展开/收起从当前高度续走) ----
        AnimatedVisibility(
            visible = open,
            enter = expandVertically(PaMotion.sheet()) + fadeIn(PaMotion.sheet()),
            exit = shrinkVertically(PaMotion.standard()) + fadeOut(PaMotion.standard()),
        ) {
            LaunchedEffect(summary.eventId) {
                // 内容进组合 = 首次展开(或再次展开);缓存命中时 onEnsureDetail 直接短路。
                if (detail == null) onEnsureDetail()
            }
            Column(Modifier.fillMaxWidth().padding(bottom = 12.dp)) {
                when {
                    detail == null -> Text("读取中…", fontSize = 12.sp, color = PaEditorial.Muted)
                    detail.error != null -> Text(
                        detail.error!!,
                        fontSize = 12.sp,
                        color = EditorialTone.ATTENTION.accent,
                    )
                    else -> {
                        val d = detail.detail!!
                        DetailLine("归类", unknownKindLabel(summary.normalizedKind))
                        DetailLine("状态", line.statusLabel)
                        // 详细解读:daemon 没给(旧版本)就整块缺席,不占位。
                        d.translation?.let {
                            Text(
                                it,
                                Modifier.padding(top = 8.dp),
                                fontSize = 12.5.sp,
                                color = PaEditorial.Muted,
                            )
                        }
                        if (d.digest.isNotEmpty()) {
                            Text(
                                "逐字段",
                                Modifier.padding(top = 10.dp, bottom = 2.dp),
                                fontSize = 11.sp,
                                color = PaEditorial.Muted,
                                fontWeight = FontWeight.Medium,
                            )
                            d.digest.forEach { DigestRow(it) }
                        }
                        Row(
                            Modifier
                                .fillMaxWidth()
                                .pressable { onToggleRaw() }
                                .padding(vertical = 8.dp),
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            Text(
                                if (rawOpen) "收起原文" else "查看原文",
                                fontSize = 12.sp,
                                color = PaEditorial.Primary,
                            )
                            Icon(
                                painterResource(
                                    if (rawOpen) R.drawable.ic_lucide_chevron_up
                                    else R.drawable.ic_lucide_chevron_down
                                ),
                                contentDescription = null,
                                tint = PaEditorial.Primary,
                                modifier = Modifier.size(13.dp),
                            )
                        }
                        if (rawOpen) {
                            Text(
                                rawBlockText(d),
                                Modifier
                                    .fillMaxWidth()
                                    .heightIn(max = 220.dp)
                                    .verticalScroll(rememberScrollState())
                                    .background(PaEditorial.GraphiteCode, RoundedCornerShape(13.dp))
                                    .padding(10.dp),
                                fontSize = 11.sp,
                                fontFamily = FontFamily.Monospace,
                                color = PaEditorial.CodeText,
                            )
                        }
                        // 五键分两行:一行塞五个在窄屏会挤到换行截断。
                        // 忽略(IGNORED)=明确不处理;转人工(ESCALATED)=需要人/上游接手
                        // —— daemon 协议两值早已支持,此前只是没上屏。
                        Row(Modifier.fillMaxWidth()) {
                            AckButton("已查看", "INSPECTED", pendingAck, onAck)
                            AckButton("稍后再看", "RETRY_LATER", pendingAck, onAck)
                            AckButton("忽略", "IGNORED", pendingAck, onAck)
                        }
                        Row(Modifier.fillMaxWidth()) {
                            AckButton("转人工", "ESCALATED", pendingAck, onAck)
                            AckButton("待上游支持", "NEEDS_NATIVE_SUPPORT", pendingAck, onAck)
                        }
                        ackFailure?.let {
                            Text(it, Modifier.padding(top = 2.dp), fontSize = 12.sp, color = EditorialTone.ATTENTION.accent)
                        }
                    }
                }
            }
        }
    }
}

/** 字段级透析表的一行:key / 含义 / 预览 三列紧凑排;daemon 没给出含义就只排两列。 */
@Composable
private fun DigestRow(row: UnknownEventDigestRow) {
    Row(Modifier.fillMaxWidth().padding(vertical = 3.dp)) {
        Text(
            row.key,
            Modifier.weight(0.9f),
            fontSize = 11.sp,
            fontFamily = FontFamily.Monospace,
            color = PaEditorial.Ink,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
        if (row.meaning != null) {
            Text(
                row.meaning!!,
                Modifier.weight(1f).padding(horizontal = 6.dp),
                fontSize = 11.sp,
                color = PaEditorial.Muted,
            )
        }
        Text(
            row.preview,
            Modifier.weight(1.1f),
            fontSize = 11.sp,
            color = PaEditorial.Muted,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
    }
}

/** 原文区内容:脱敏原文在前,"解出的字段"随后(沿用旧详情弹窗的两段式)。 */
private fun rawBlockText(d: UnknownEventResponseOf): String =
    listOfNotNull(
        d.rawJson.takeIf { it.isNotBlank() },
        d.decodedFieldsJson.takeIf { it.isNotBlank() }?.let { "解出的字段:$it" },
    ).joinToString("\n\n").ifBlank { "(载荷为空)" }

@Composable
private fun AckButton(label: String, action: String, disabled: Boolean, onClick: (String) -> Unit) {
    TextButton(onClick = { onClick(action) }, enabled = !disabled) {
        Text(label, fontSize = 13.sp, color = PaEditorial.Primary)
    }
}

/**
 * 内存详情/管理弹窗(PaSheet 统一玻璃浮层)。
 *
 * 【内容三段】① 记忆形态人话(这台 agent 的记忆记在哪、清除动的是什么);② 该
 * agent 的记忆合计与分层计数(working/conversation/summary/artifact);③ 按会话
 * 的清单(ListThreadMemories,daemon 已按字节降序排好),每行一个「清除」动作。
 *
 * 【清除必须过确认框】确认文案写清「只清 PocketAgent 侧记忆,不动电脑端原生会话」
 * —— 这个动作对用户不可逆,而这句正是他与"删会话"区分开的唯一凭据。
 *
 * 【乐观更新 + 失败回滚】点确认后本行先消失、合计同步少(界面不出现"行没了合计
 * 还在"的自相矛盾);daemon 真值(剩余合计)回来后接管;失败则换回点击那一刻的行
 * 快照,并给一句"已还原"。行快照在确认时冻结 —— 只有那一刻的数据才能忠实还原。
 *
 * 【降级】listThreadMemories 回 UNIMPLEMENTED(旧 daemon)时整弹窗换成一句事实
 * 陈述「此 daemon 版本未提供内存管理」,不摆必然失败的空清单。
 *
 * 【paPushBack 说明】与 UnknownEventSheet 同款取舍:宿主内容根在 SettingsScreen,
 * 本文件够不到那个根,弹窗本体用 PaSheet 玻璃材质照常渲染。
 */
@Composable
private fun ThreadMemorySheet(agentId: String, onDismiss: () -> Unit) {
    // rows 为 null = 清单还在拉;非 null = daemon 的次序(字节降序)原样上屏。
    var rows by remember { mutableStateOf<List<ThreadMemoryRowUi>?>(null) }
    var totals by remember { mutableStateOf<ThreadMemoryTotalsUi?>(null) }
    var kinds by remember { mutableStateOf<List<MemoryKindCount>>(emptyList()) }
    var kindsNote by remember { mutableStateOf<String?>(null) }
    var loadError by remember { mutableStateOf<String?>(null) }
    // 管理动作状态:待确认的行 / 是否在清除 / 失败回滚后的提示。
    var pendingClear by remember { mutableStateOf<ThreadMemoryRowUi?>(null) }
    var clearing by remember { mutableStateOf(false) }
    var rollbackNote by remember { mutableStateOf<String?>(null) }
    val scope = rememberCoroutineScope()

    // 旧 daemon 的确定信号在数据层记忆;打开弹窗时若已是事实,直接给陈述页。
    val unsupported = RuntimeControlState.memoriesUnsupported
    val modelRaw = memoryModelOf(RuntimeControlState.overview, agentId)

    LaunchedEffect(agentId) {
        if (unsupported) return@LaunchedEffect
        // 两路并发:清单(按会话)与用量(合计/分层的权威来源)。清单失败且不是
        // "版本没这功能"才给错误行 —— UNIMPLEMENTED 已经换陈述页,不再叠加报错。
        coroutineScope {
            val listJob = async {
                RuntimeControlState.listThreadMemories(agentId)
                    .onSuccess { resp ->
                        rows = mapThreadMemories(resp)
                        totals = ThreadMemoryTotalsUi(resp.totalThreads, resp.totalEntries, resp.totalBytes)
                    }
                    .onFailure { e ->
                        if (!RuntimeControlState.memoriesUnsupported) {
                            loadError = e.message ?: e::class.java.simpleName
                        }
                    }
            }
            val usageJob = async {
                RuntimeControlState.agentRuntime(agentId)
                    .onSuccess { r ->
                        val m = if (r.hasMemory()) r.memory else null
                        kinds = memoryKindsList(m?.kindsMap ?: emptyMap())
                        kindsNote = null
                    }
                    .onFailure { e -> kindsNote = e.message ?: e::class.java.simpleName }
            }
            listJob.await()
            usageJob.await()
        }
    }

    PaSheet(onDismissRequest = onDismiss) {
        Column(Modifier.fillMaxWidth().padding(horizontal = 24.dp)) {
            Text(
                "内存管理",
                fontSize = 19.sp,
                color = PaEditorial.Ink,
                fontWeight = FontWeight.SemiBold,
            )
            when {
                // ---- 降级:事实陈述,不摆空清单也不报错 ----
                unsupported -> {
                    Spacer(Modifier.height(6.dp))
                    Text(
                        "此 daemon 版本未提供内存管理",
                        fontSize = 13.sp,
                        color = PaEditorial.Muted,
                    )
                    Text(
                        "电脑端的记忆用量仍显示在形态卡里;升级 daemon 后这里可以按会话查看与清除。",
                        Modifier.padding(top = 4.dp),
                        fontSize = 12.sp,
                        color = PaEditorial.Muted,
                    )
                }
                else -> {
                    // ---- ① 形态人话 ----
                    if (modelRaw != null) {
                        Text(
                            memoryModelExplanation(modelRaw),
                            Modifier.padding(top = 4.dp),
                            fontSize = 12.5.sp,
                            color = PaEditorial.Muted,
                        )
                    }
                    // ---- ② 合计 + 分层 ----
                    totals?.let {
                        Text(
                            threadTotalsLine(it),
                            Modifier.padding(top = 8.dp),
                            fontSize = 13.sp,
                            color = PaEditorial.Ink,
                            fontWeight = FontWeight.Medium,
                        )
                    }
                    if (kinds.isNotEmpty()) {
                        Text(
                            kinds.joinToString(" · ") { "${memoryKindLabel(it.kind)} ${it.count}" },
                            Modifier.padding(top = 3.dp),
                            fontSize = 12.sp,
                            color = PaEditorial.Muted,
                        )
                    }
                    kindsNote?.let {
                        Text(it, Modifier.padding(top = 3.dp), fontSize = 11.sp, color = PaEditorial.Muted)
                    }
                    rollbackNote?.let {
                        Text(it, Modifier.padding(top = 4.dp), fontSize = 12.sp, color = EditorialTone.ATTENTION.accent)
                    }

                    // ---- ③ 按会话清单 ----
                    when {
                        rows == null && loadError == null && !unsupported -> {
                            Text("读取中…", Modifier.padding(top = 12.dp), fontSize = 12.sp, color = PaEditorial.Muted)
                        }
                        loadError != null -> {
                            Text(loadError!!, Modifier.padding(top = 12.dp), fontSize = 12.sp, color = EditorialTone.ATTENTION.accent)
                        }
                        rows?.isEmpty() == true -> {
                            EmptyState(
                                iconRes = R.drawable.ic_lucide_brain,
                                title = "这个 agent 还没有会话记忆",
                                hint = "对话跑起来之后,记忆会按会话出现在这里",
                            )
                        }
                        rows != null -> {
                            Text(
                                "按会话(占得多的在前)",
                                Modifier.padding(top = 12.dp, bottom = 2.dp),
                                fontSize = 11.sp,
                                color = PaEditorial.Muted,
                                fontWeight = FontWeight.Medium,
                            )
                            rows!!.forEachIndexed { i, r ->
                                MemoryThreadRow(
                                    row = r,
                                    clearing = clearing,
                                    onClear = { pendingClear = r },
                                )
                                if (i < rows!!.lastIndex) {
                                    Box(Modifier.fillMaxWidth().height(1.dp).background(PaEditorial.Hairline))
                                }
                            }
                        }
                    }
                }
            }
            Spacer(Modifier.height(8.dp))
        }
    }

    // ---- 清除确认:独立弹窗,文案把"动什么/不动什么"说死 ----
    pendingClear?.let { target ->
        AlertDialog(
            onDismissRequest = { if (!clearing) pendingClear = null },
            containerColor = PaEditorial.Surface,
            title = {
                Text("清除这个会话的记忆?", fontSize = 16.sp, color = PaEditorial.Ink)
            },
            text = {
                Column {
                    Text(
                        target.threadId,
                        fontSize = 12.sp,
                        fontFamily = FontFamily.Monospace,
                        color = PaEditorial.Muted,
                        maxLines = 2,
                        overflow = TextOverflow.Ellipsis,
                    )
                    Spacer(Modifier.height(8.dp))
                    Text(
                        "只清除 PocketAgent 在电脑端记的这份会话记忆" +
                            "(${target.entries} 条 · ${formatBytesLabel(target.bytes)})，" +
                            "不动电脑端原生程序里的会话内容。清除后这个会话的记忆从零开始。",
                        fontSize = 13.sp,
                        color = PaEditorial.Muted,
                    )
                }
            },
            confirmButton = {
                TextButton(
                    enabled = !clearing,
                    onClick = {
                        clearing = true
                        rollbackNote = null
                        // 冻结快照:失败回滚还原的是"点确认那一刻"的界面。
                        val snapshot = rows ?: emptyList()
                        val snapshotTotals = totals
                        // 乐观更新:行先消失,合计同步少(算式唯一,见 threadTotalsFrom)。
                        val optimistic = threadRowsWithout(snapshot, target.threadId)
                        rows = optimistic
                        totals = threadTotalsFrom(optimistic)
                        scope.launch {
                            val result = RuntimeControlState.clearThreadMemory(agentId, target.threadId)
                            result
                                .onSuccess { resp ->
                                    // daemon 真值接管乐观合计。
                                    totals = ThreadMemoryTotalsUi(resp.totalThreads, resp.totalEntries, resp.totalBytes)
                                }
                                .onFailure { e ->
                                    rows = snapshot
                                    totals = snapshotTotals
                                    rollbackNote = "清除失败:" +
                                        (e.message ?: e::class.java.simpleName) + ",已还原"
                                }
                            clearing = false
                            pendingClear = null
                        }
                    },
                ) { Text("清除", color = EditorialTone.DENIED.text) }
            },
            dismissButton = {
                TextButton(
                    enabled = !clearing,
                    onClick = { pendingClear = null },
                ) { Text("取消", color = PaEditorial.Muted) }
            },
        )
    }
}

/** 清单里的一个会话行:thread id + 人话合计,行尾「清除」动作(裸图标按钮)。 */
@Composable
private fun MemoryThreadRow(row: ThreadMemoryRowUi, clearing: Boolean, onClear: () -> Unit) {
    Row(
        Modifier
            .fillMaxWidth()
            .padding(vertical = 8.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Column(Modifier.weight(1f)) {
            Text(
                row.threadId,
                fontSize = 12.5.sp,
                fontFamily = FontFamily.Monospace,
                color = PaEditorial.Ink,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            Text(
                threadMemoryLine(row),
                Modifier.padding(top = 2.dp),
                fontSize = 11.5.sp,
                color = PaEditorial.Muted,
            )
        }
        TextButton(onClick = onClear, enabled = !clearing) {
            Text("清除", fontSize = 13.sp, color = EditorialTone.DENIED.text)
        }
    }
}
