package dev.pocketagent.app.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
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
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.runtime.collectAsState
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.TextRange
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.TextFieldValue
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.AgentConnection
import dev.pocketagent.app.R
import dev.pocketagent.app.ui.theme.Pa
import kotlinx.coroutines.launch
import pocketagent.v1.CollabPolicy
import pocketagent.v1.RuntimePolicy

/**
 * 设置页的「运行时」区段(W11b)。
 *
 * 【为什么单独成文件,而不是塞进 SettingsScreen】SettingsScreen.kt 已 178KB,
 * 再加这一块(五组内容:恢复横幅 / 本机总览与形态卡 / 预算 / 未知事件 / 协同策略)
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
    val unknownLines = RuntimeControlState.unknownEvents.map { unknownEventLine(it) }
    val unknownNote = unknownLedgerNote(RuntimeControlState.unknownTotal, RuntimeControlState.unknownDropped)

    var expanded by remember { mutableStateOf(false) }
    var editingBudget by remember { mutableStateOf<BudgetRow?>(null) }
    var expandedAgent by remember { mutableStateOf<String?>(null) }
    var openEvent by remember { mutableStateOf<UnknownEventLine?>(null) }
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
                    divider = cards.isNotEmpty() || budgets.isNotEmpty() || collabs.isNotEmpty() || unknownLines.isNotEmpty(),
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

            // ---- 未知事件 ----
            SubHeader("原生发来的新事件")
            if (unknownLines.isEmpty()) {
                SectionNote(
                    text = "最近没有原生发来的新事件",
                    color = Pa.TextSecondary,
                )
            } else {
                unknownLines.forEach { line ->
                    SettingRow(
                        title = line.title,
                        desc = line.subtitle,
                        value = line.statusLabel,
                        chevron = true,
                        divider = true,
                        onClick = { openEvent = line },
                    )
                }
                if (unknownNote != null) SectionNote(text = unknownNote, color = Pa.TextTertiary)
            }

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

    openEvent?.let { line ->
        UnknownEventDialog(line = line, onDismiss = { openEvent = null })
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
 * 未知事件详情 + 确认。
 *
 * 【为什么原文照给不加工】这是 README §7 主线 A 的落地点:原生 agent 升级后发来的
 * 东西我们看不懂,但"看不懂"正是用户要判断(要不要等上游支持、要不要关掉某个实验
 * 特性)的依据。把原文藏起来等于把判断权也拿走。密钥类字段由 daemon 侧脱敏。
 */
@Composable
private fun UnknownEventDialog(line: UnknownEventLine, onDismiss: () -> Unit) {
    var raw by remember(line.eventId) { mutableStateOf<String?>(null) }
    var note by remember(line.eventId) { mutableStateOf<String?>(null) }
    var error by remember(line.eventId) { mutableStateOf<String?>(null) }
    var pending by remember(line.eventId) { mutableStateOf(false) }
    val scope = rememberCoroutineScope()

    LaunchedEffect(line.eventId) {
        RuntimeControlState.unknownEventDetail(line.eventId)
            .onSuccess { d ->
                raw = listOfNotNull(
                    d.rawJson.takeIf { it.isNotBlank() },
                    d.decodedFieldsJson.takeIf { it.isNotBlank() }?.let { "解出的字段:$it" },
                    d.note.takeIf { it.isNotBlank() }?.let { "备注:$it" },
                ).joinToString("\n\n").ifBlank { "(载荷为空)" }
            }
            .onFailure { error = it.message ?: it::class.java.simpleName }
    }

    AlertDialog(
        onDismissRequest = onDismiss,
        containerColor = Pa.CardHigh,
        title = { Text(line.title, fontSize = 15.sp, color = Pa.TextPrimary) },
        text = {
            Column(Modifier.heightIn(max = 380.dp).verticalScroll(rememberScrollState())) {
                Text(line.subtitle, fontSize = 12.sp, color = Pa.TextSecondary)
                Spacer(Modifier.height(10.dp))
                Text(
                    raw ?: error ?: "读取中…",
                    fontSize = 12.sp,
                    color = if (raw != null) Pa.TextPrimary else Pa.TextTertiary,
                )
                if (note != null) {
                    Spacer(Modifier.height(8.dp))
                    Text(note!!, fontSize = 12.sp, color = Pa.Yellow)
                }
            }
        },
        confirmButton = {
            Row {
                AckButton("已查看", "INSPECTED", pending) { action ->
                    pending = true
                    scope.launch {
                        val err = RuntimeControlState.acknowledge(line.eventId, action, "")
                        pending = false
                        if (err != null) note = err else onDismiss()
                    }
                }
                AckButton("待上游支持", "NEEDS_NATIVE_SUPPORT", pending) { action ->
                    pending = true
                    scope.launch {
                        val err = RuntimeControlState.acknowledge(line.eventId, action, "")
                        pending = false
                        if (err != null) note = err else onDismiss()
                    }
                }
            }
        },
        dismissButton = {
            Row {
                AckButton("稍后再看", "RETRY_LATER", pending) { action ->
                    pending = true
                    scope.launch {
                        val err = RuntimeControlState.acknowledge(line.eventId, action, "")
                        pending = false
                        if (err != null) note = err else onDismiss()
                    }
                }
                TextButton(onClick = onDismiss) { Text("关闭", color = Pa.TextSecondary) }
            }
        },
    )
}

@Composable
private fun AckButton(label: String, action: String, disabled: Boolean, onClick: (String) -> Unit) {
    TextButton(onClick = { onClick(action) }, enabled = !disabled) {
        Text(label, fontSize = 13.sp, color = Pa.AccentText)
    }
}
