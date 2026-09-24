package dev.pocketagent.app.ui

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.expandVertically
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.shrinkVertically
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
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
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Icon
import androidx.compose.material3.OutlinedTextField
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
import kotlinx.coroutines.launch
import pocketagent.v1.CollabPolicy
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
    val overview = runtimeOverview(RuntimeControlState.overview)
    val cards = agentContractCards(RuntimeControlState.overview)
    val banner = recoveryBanner(RuntimeControlState.recovery)
    val budgets = budgetRows(RuntimeControlState.budget?.effectiveGlobal)
    val collabs = collabPolicyRows(RuntimeControlState.collab?.effective)
    val unknownCount = RuntimeControlState.unknownEvents.size
    // NEW = daemon 侧还没被处置过的条数;入口行的计数徽标只数这一档。
    val unknownPending = RuntimeControlState.unknownEvents.count { it.processingStatus == "NEW" }
    val unknownNote = unknownLedgerNote(RuntimeControlState.unknownTotal, RuntimeControlState.unknownDropped)

    var expanded by remember { mutableStateOf(false) }
    var editingBudget by remember { mutableStateOf<BudgetRow?>(null) }
    var expandedAgent by remember { mutableStateOf<String?>(null) }
    var showDialysisSheet by remember { mutableStateOf(false) }
    var busy by remember { mutableStateOf<String?>(null) }
    val scope = rememberCoroutineScope()

    // 首次进入拉一次。之后靠手动刷新 —— 用户改预算/策略后本地已有 daemon 回传的真值,
    // 不必每次滚动重组都打一轮 RPC。
    LaunchedEffect(Unit) {
        if (RuntimeControlState.overview == null) RuntimeControlState.load()
    }

    val summary = when {
        loadState == RuntimeLoadState.LOADING && overview == null -> "读取中…"
        overview == null -> sectionErrors[RuntimeControlState.SECTION_STATE] ?: "没读到电脑端的状态"
        else -> buildString {
            append(overview.stateLabel)
            if (RuntimeControlState.unknownTotal > 0) append(" · ${RuntimeControlState.unknownTotal} 条新事件")
        }
    }

    SectionBlock(icon = R.drawable.ic_lucide_sliders, title = "运行时") {
        // ---- 启动恢复横幅:降级时始终可见(警告不折叠) ----
        if (banner != null && banner.degraded) {
            RecoveryBannerBlock(banner)
        }

        SettingRow(
            title = "运行时控制面",
            value = summary,
            chevron = true,
            divider = expanded,
            onClick = { expanded = !expanded },
        )

        if (expanded) {
            // ---- 分块失败:逐块说清哪一块没读到,不整页清空 ----
            if (sectionErrors.isNotEmpty()) {
                SectionNote(
                    text = sectionErrors.entries.joinToString(";") { "${sectionLabel(it.key)}: ${it.value}" },
                    color = Pa.Yellow,
                )
            }

            // ---- 本机总览 ----
            if (overview != null) {
                SettingRow(
                    title = "本机",
                    desc = overview.dataDir,
                    value = "v${overview.version}",
                    divider = true,
                )
                SettingRow(
                    title = "策略代数",
                    value = "预算 ${overview.budgetRevision} · 协同 ${overview.collabRevision}",
                    // 消息透析入口行常驻(0 条也显示弱化徽标),所以下面永远还有一行。
                    divider = true,
                )
            }

            // ---- 每 agent 形态卡 ----
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
                    AgentRuntimeDetail(card)
                }
            }

            // ---- 预算(可改) ----
            if (budgets.isNotEmpty()) {
                SubHeader("预算(点任意一项可改)")
                budgets.forEachIndexed { i, row ->
                    SettingRow(
                        title = row.label,
                        value = row.value,
                        chevron = true,
                        divider = i < budgets.lastIndex,
                        onClick = { editingBudget = row },
                    )
                }
                Spacer(Modifier.height(6.dp))
            }

            // ---- 消息透析(入口行;列表与详情收进弹窗,不再摊在设置页里) ----
            SettingRow(
                title = "消息透析",
                desc = when {
                    unknownCount == 0 -> "最近没有需要透析的消息"
                    else -> unknownNote ?: "共 $unknownCount 条"
                },
                chevron = true,
                divider = collabs.isNotEmpty(),
                control = {
                    if (unknownPending > 0) {
                        Text(
                            "$unknownPending",
                            Modifier
                                .background(Pa.Red, RoundedCornerShape(50))
                                .padding(horizontal = 8.dp, vertical = 2.dp),
                            fontSize = 11.sp,
                            color = androidx.compose.ui.graphics.Color.White,
                            fontWeight = FontWeight.SemiBold,
                        )
                    } else {
                        // 0 条时弱化:只给一个灰色数字,不抢视觉。
                        Text("0", fontSize = 12.sp, color = Pa.TextTertiary)
                    }
                },
                onClick = { showDialysisSheet = true },
            )

            // ---- 协同策略(可改) ----
            if (collabs.isNotEmpty()) {
                SubHeader("协同策略")
                collabs.forEachIndexed { i, row ->
                    if (row.key == "max_parallel_members") {
                        SettingRow(
                            title = row.label,
                            value = row.value,
                            chevron = true,
                            divider = i < collabs.lastIndex,
                            onClick = { editingBudget = BudgetRow(row.key, row.label, row.value) },
                        )
                    } else {
                        SettingRow(
                            title = row.label,
                            divider = i < collabs.lastIndex,
                            control = {
                                Switch(
                                    checked = row.value == "开启",
                                    enabled = busy == null,
                                    onCheckedChange = { on ->
                                        busy = row.key
                                        scope.launch {
                                            RuntimeControlState.saveCollabPolicy { b ->
                                                when (row.key) {
                                                    "collaboration_enabled" -> b.setCollaborationEnabled(on)
                                                    "native_delegate_enabled" -> b.setNativeDelegateEnabled(on)
                                                    "fanout_enabled" -> b.setFanoutEnabled(on)
                                                }
                                            }?.let { err ->
                                                sectionErrorPing(RuntimeControlState.SECTION_COLLAB, err)
                                            }
                                            busy = null
                                        }
                                    },
                                )
                            },
                        )
                    }
                }
            }

            // ---- 手动刷新 ----
            Spacer(Modifier.height(4.dp))
            Row(
                Modifier.fillMaxWidth().padding(vertical = 6.dp),
                horizontalArrangement = Arrangement.End,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                TextButton(onClick = { RuntimeControlState.load() }) {
                    Text("重新读取", fontSize = 13.sp, color = Pa.AccentText)
                }
            }
        }
    }

    editingBudget?.let { row ->
        BudgetEditDialog(
            row = row,
            onDismiss = { editingBudget = null },
            onResult = { err ->
                editingBudget = null
                if (err != null) sectionErrorPing(RuntimeControlState.SECTION_BUDGET, err)
            },
        )
    }

    if (showDialysisSheet) {
        UnknownEventSheet(onDismiss = { showDialysisSheet = false })
    }
}

/** 恢复横幅:降级摘要 + **逐条**"哪句没跑到"(只报条数等于没说)。 */
@Composable
private fun RecoveryBannerBlock(banner: RecoveryBanner) {
    Column(
        Modifier
            .fillMaxWidth()
            .padding(top = 8.dp, bottom = 2.dp)
            .background(Pa.Red.copy(alpha = 0.12f), RoundedCornerShape(12.dp))
            .padding(12.dp),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Icon(
                painterResource(R.drawable.ic_lucide_triangle_alert),
                contentDescription = null,
                tint = Pa.RedText,
                modifier = Modifier.size(15.dp),
            )
            Spacer(Modifier.width(8.dp))
            Text(banner.headline, fontSize = 13.sp, color = Pa.RedText)
        }
        if (banner.lostLines.isNotEmpty()) {
            Spacer(Modifier.height(8.dp))
            Text(
                "上次退出时没轮到的输入(共 ${banner.lostLines.size} 句,涉及 ${banner.threadCount} 个会话)",
                fontSize = 12.sp,
                color = Pa.TextSecondary,
            )
            banner.lostLines.forEach { line ->
                Text(
                    line,
                    Modifier.padding(top = 4.dp),
                    fontSize = 12.5.sp,
                    color = Pa.TextPrimary,
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
 */
@Composable
private fun AgentRuntimeDetail(card: AgentContractCard) {
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
            .background(Pa.CardHigh, RoundedCornerShape(12.dp))
            .padding(horizontal = 12.dp, vertical = 8.dp),
    ) {
        card.rows.forEach { r ->
            DetailLine(r.label, r.value)
        }
        card.issues.forEach { DetailLine("未交代", it, Pa.RedText) }
        card.limitations.forEach { DetailLine("明确不支持", it, Pa.TextSecondary) }
        when {
            resourceNote != null -> DetailLine("资源", resourceNote!!, Pa.TextTertiary)
            resources?.isEmpty() == true -> DetailLine("资源", "当前没有活跃的资源租约")
            resources != null -> resources!!.forEach { DetailLine("资源", it) }
        }
        usage.forEach { DetailLine(it.label, it.value) }
    }
}

@Composable
private fun DetailLine(label: String, value: String, color: androidx.compose.ui.graphics.Color = Pa.TextPrimary) {
    Row(Modifier.fillMaxWidth().padding(vertical = 2.dp)) {
        Text(label, fontSize = 12.sp, color = Pa.TextTertiary, modifier = Modifier.width(72.dp))
        Text(value, fontSize = 12.5.sp, color = color, modifier = Modifier.weight(1f))
    }
}

@Composable
private fun SubHeader(text: String) {
    Text(
        text,
        Modifier.padding(start = 2.dp, top = 12.dp, bottom = 2.dp),
        fontSize = 12.sp,
        color = Pa.TextTertiary,
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
        containerColor = Pa.CardHigh,
        title = { Text(row.label, fontSize = 16.sp, color = Pa.TextPrimary) },
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
                    Text(error!!, fontSize = 12.sp, color = Pa.RedText)
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
                    val err = saveRuntimeEdit(row.key, n)
                    if (err != null) error = err else onResult(null)
                }
            }) { Text("保存", color = Pa.AccentText) }
        },
        dismissButton = {
            Row {
                TextButton(onClick = {
                    scope.launch {
                        val err = saveRuntimeEdit(row.key, null)
                        if (err != null) error = err else onResult(null)
                    }
                }) { Text("恢复默认", color = Pa.TextSecondary) }
                TextButton(onClick = onDismiss) { Text("取消", color = Pa.TextSecondary) }
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
                color = Pa.TextPrimary,
                fontWeight = FontWeight.SemiBold,
            )
            ledger?.let {
                Text(it, Modifier.padding(top = 3.dp), fontSize = 11.sp, color = Pa.TextTertiary)
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
                        Box(Modifier.fillMaxWidth().height(1.dp).background(Pa.Separator))
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
                    Text(line.relativeTime, fontSize = 11.sp, color = Pa.TextTertiary)
                    Spacer(Modifier.width(8.dp))
                }
                if (line.sourceAgent.isNotBlank()) {
                    Text(
                        line.sourceAgent,
                        fontSize = 11.sp,
                        color = Pa.TextSecondary,
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
                    color = Pa.TextPrimary,
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis,
                )
                Spacer(Modifier.width(6.dp))
                Icon(
                    painterResource(
                        if (open) R.drawable.ic_lucide_chevron_up else R.drawable.ic_lucide_chevron_down
                    ),
                    contentDescription = null,
                    tint = Pa.TextTertiary,
                    modifier = Modifier.size(14.dp),
                )
            }
            line.degradedNote?.let {
                Text(it, Modifier.padding(top = 3.dp), fontSize = 11.sp, color = Pa.TextTertiary)
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
                    detail == null -> Text("读取中…", fontSize = 12.sp, color = Pa.TextTertiary)
                    detail.error != null -> Text(
                        detail.error!!,
                        fontSize = 12.sp,
                        color = Pa.Yellow,
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
                                color = Pa.TextSecondary,
                            )
                        }
                        if (d.digest.isNotEmpty()) {
                            Text(
                                "逐字段",
                                Modifier.padding(top = 10.dp, bottom = 2.dp),
                                fontSize = 11.sp,
                                color = Pa.TextTertiary,
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
                                color = Pa.AccentText,
                            )
                            Icon(
                                painterResource(
                                    if (rawOpen) R.drawable.ic_lucide_chevron_up
                                    else R.drawable.ic_lucide_chevron_down
                                ),
                                contentDescription = null,
                                tint = Pa.AccentText,
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
                                    .background(Pa.Bg, RoundedCornerShape(12.dp))
                                    .padding(10.dp),
                                fontSize = 11.sp,
                                fontFamily = FontFamily.Monospace,
                                color = Pa.TextPrimary,
                            )
                        }
                        Row(Modifier.fillMaxWidth()) {
                            AckButton("已查看", "INSPECTED", pendingAck, onAck)
                            AckButton("待上游支持", "NEEDS_NATIVE_SUPPORT", pendingAck, onAck)
                            AckButton("稍后再看", "RETRY_LATER", pendingAck, onAck)
                        }
                        ackFailure?.let {
                            Text(it, Modifier.padding(top = 2.dp), fontSize = 12.sp, color = Pa.Yellow)
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
            color = Pa.TextPrimary,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
        if (row.meaning != null) {
            Text(
                row.meaning!!,
                Modifier.weight(1f).padding(horizontal = 6.dp),
                fontSize = 11.sp,
                color = Pa.TextSecondary,
            )
        }
        Text(
            row.preview,
            Modifier.weight(1.1f),
            fontSize = 11.sp,
            color = Pa.TextTertiary,
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
        Text(label, fontSize = 13.sp, color = Pa.AccentText)
    }
}
