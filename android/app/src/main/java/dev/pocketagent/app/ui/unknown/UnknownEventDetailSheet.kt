package dev.pocketagent.app.ui.unknown

import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.widget.Toast
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.expandVertically
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.shrinkVertically
import androidx.compose.foundation.background
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
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.R
import dev.pocketagent.app.ui.anim.EmptyState
import dev.pocketagent.app.ui.RuntimeControlState
import dev.pocketagent.app.ui.UnknownEventActionModel
import dev.pocketagent.app.ui.UnknownEventCardModel
import dev.pocketagent.app.ui.UnknownEventDigestRow
import dev.pocketagent.app.ui.UnknownActionRoute
import dev.pocketagent.app.ui.UnknownEventResponseOf
import dev.pocketagent.app.ui.relativeTimeLabel
import dev.pocketagent.app.ui.unknownActionRequest
import dev.pocketagent.app.ui.unknownEventCardModel
import dev.pocketagent.app.ui.unknownLedgerNote
import dev.pocketagent.app.ui.unknownStatusLabel
import dev.pocketagent.app.ui.partitionUnknownEvents
import dev.pocketagent.app.ui.anim.PaMotion
import dev.pocketagent.app.ui.anim.pressable
import dev.pocketagent.app.ui.component.PaSheet
import dev.pocketagent.app.ui.theme.EditorialTone
import dev.pocketagent.app.ui.theme.PaEditorial
import kotlinx.coroutines.launch

private const val TAG = "UnknownEventDetailSheet"

/**
 * 「电脑端新消息」弹窗(2026-10-04 未知事件用户处理中心计划 Task 4)。
 *
 * 【为什么从「消息透析」改名】透析是技术台账视角;对普通用户,这里就是"电脑端发来
 * 了什么、我要不要处理"。列表分三组(需要你处理 / 已查看 / 仅供参考),折叠行只说
 * 人话(plain title + 影响 + 来源 + 时间 + 状态),原文/字段透析/correlation id
 * 全部折叠进「技术详情」—— 普通 UI 不直接显示 JSON(全局约束)。
 *
 * 【动作链路,三路由】
 *  - copy:手机本地剪贴板,**绝不发 daemon**(请求组装层 [unknownActionRequest] 已封死);
 *  - inspect:本端展开「技术详情」(内容走既有 GetUnknownEvent;daemon 侧 inspect
 *    复用同一份能力,不重复发动作 RPC);
 *  - acknowledge/retry_later/open_native(以及未来 daemon 新增白名单动作):经
 *    runtimeService().executeUnknownEventAction,带 event_id + 当前 revision(CAS)
 *    + 可选备注;**冲突是结构化结果**(ok=false+conflict=true),这里重拉详情并给人话
 *    「这条事件的状态已变化,已为你刷新」,不炸 UI、不显示技术异常。
 *
 * 【详情为什么按需拉且缓存】列表(summary)很轻,payload/透析表在 GetUnknownEvent
 * 里,首次展开才拉;缓存 keyed by eventId 放在 sheet 级而不是行级 remember ——
 * AnimatedVisibility 收起会把行内容移出组合,行级 remember 会被丢掉。冲突/处置后
 * 先清缓存再重拉,保证技术区看到的是 daemon 当前值。
 *
 * 【与设置页的关系】清单来自共享的 [RuntimeControlState.unknownEvents](打开即轻量
 * 刷新一次),不建第二数据源;入口行在 RuntimeSection(标题人话化见
 * [UnknownEventCopy.entryTitleOverride])。
 */
@Composable
fun UnknownEventDetailSheet(onDismiss: () -> Unit) {
    // 打开即轻量刷新一次(单路 RPC,不背运行时区段其余四块)。
    LaunchedEffect(Unit) { RuntimeControlState.refreshUnknownEvents() }

    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    // 卡片模型映射(唯一映射点);分组/排序都是纯函数,同份数据每次渲染同序。
    val groups = partitionUnknownEvents(
        RuntimeControlState.unknownEvents.map { unknownEventCardModel(it) },
    )
    val ledger = unknownLedgerNote(RuntimeControlState.unknownTotal, RuntimeControlState.unknownDropped)

    var expandedId by remember { mutableStateOf<String?>(null) }
    var technicalOpenFor by remember { mutableStateOf<String?>(null) }
    // 详情缓存(键 = eventId);detailInFlight 去重"快速收起再展开"时的并发重复拉取。
    val details = remember { mutableStateMapOf<String, SheetDetailState>() }
    val detailInFlight = remember { mutableSetOf<String>() }
    // 备注输入(可选):状态型动作随请求透传 —— 有输入就带上,不做"始终传空串"。
    val notes = remember { mutableStateMapOf<String, String>() }
    // 动作进行中(eventId to actionId);同一条事件在途时禁用该行按钮防连点/重复提交。
    var busyAction by remember { mutableStateOf<Pair<String, String>?>(null) }
    // 每行的人话结果(成功=daemon detail 原文;冲突=已刷新;失败=统一人话)。
    val messages = remember { mutableStateMapOf<String, EventResultMessage>() }
    // requires_confirmation 的动作先过一次确认框(daemon 当前动作表全为 false,
    // 但该字段是契约的一部分,客户端必须接住 —— 未来 daemon 打开它时 UI 不用改)。
    var pendingConfirm by remember { mutableStateOf<Pair<UnknownEventCardModel, UnknownEventActionModel>?>(null) }

    fun ensureDetail(eventId: String) {
        if (!details.containsKey(eventId) && detailInFlight.add(eventId)) {
            scope.launch {
                val r = RuntimeControlState.unknownEventDetail(eventId)
                details[eventId] = r.fold(
                    onSuccess = { SheetDetailState(detail = it) },
                    onFailure = {
                        // 技术异常只进日志;界面给固定人话(评审焦点红线:详情拉取失败
                        // 不能显示技术异常堆栈)。
                        android.util.Log.w(TAG, "未识别事件详情加载失败 event=$eventId", it)
                        SheetDetailState(loadFailed = true)
                    },
                )
                detailInFlight.remove(eventId)
            }
        }
    }

    /** 冲突/处置后的刷新:清缓存重拉,技术区与 revision 跟着 daemon 真值走。 */
    fun refreshDetail(eventId: String) {
        details.remove(eventId)
        ensureDetail(eventId)
    }

    fun runAction(card: UnknownEventCardModel, action: UnknownEventActionModel, note: String) {
        if (busyAction != null) return
        // 组装层拒绝 copy(双保险):copy 永不出网。
        val req = unknownActionRequest(card.eventId, card.revision, action, note) ?: return
        check(req.actionId == action.actionId && req.eventId == card.eventId) { "动作请求组装错位" }
        busyAction = card.eventId to action.actionId
        scope.launch {
            val result = RuntimeControlState.executeUnknownEventAction(card.eventId, card.revision, action.actionId, note)
            busyAction = null
            result.fold(
                onSuccess = { r ->
                    if (r.conflict) {
                        // 结构化冲突:daemon 已把当前 summary/new_revision 随结果带回并
                        // 就地更新列表;这里再重拉详情刷新技术区。人话只说"已刷新"。
                        refreshDetail(card.eventId)
                        messages[card.eventId] = EventResultMessage(UnknownEventCopy.CONFLICT_REFRESHED, attention = true)
                    } else {
                        // 成功:照抄 daemon 的人话 detail(实际做了什么);缺席才用本地兜底。
                        messages[card.eventId] = EventResultMessage(
                            r.detail.ifBlank { UnknownEventCopy.ACTION_DONE },
                            attention = false,
                        )
                    }
                },
                onFailure = { e ->
                    android.util.Log.w(TAG, "未识别事件动作失败 event=${card.eventId} action=${action.actionId}", e)
                    messages[card.eventId] = EventResultMessage(UnknownEventCopy.ACTION_FAILED, attention = true)
                },
            )
        }
    }

    /** copy:手机本地剪贴板(与 MarkdownText 同款 setPrimaryClip + Toast 回执)。 */
    fun copyTechnical(detail: UnknownEventResponseOf?) {
        val raw = detail?.let {
            listOfNotNull(
                it.rawJson.takeIf(String::isNotBlank),
                it.decodedFieldsJson.takeIf(String::isNotBlank),
            ).joinToString("\n\n")
        }.orEmpty()
        if (raw.isBlank()) {
            Toast.makeText(ctx, UnknownEventCopy.COPY_EMPTY, Toast.LENGTH_SHORT).show()
            return
        }
        val cm = ctx.getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager
        cm.setPrimaryClip(ClipData.newPlainText("技术原文", raw))
        Toast.makeText(ctx, UnknownEventCopy.COPY_SUCCESS, Toast.LENGTH_SHORT).show()
    }

    PaSheet(onDismissRequest = onDismiss) {
        Column(Modifier.fillMaxWidth().padding(horizontal = 24.dp)) {
            Text(
                UnknownEventCopy.SHEET_TITLE,
                fontSize = PaEditorial.Type.Heading.fontSize,
                color = PaEditorial.Ink,
                fontWeight = FontWeight.SemiBold,
            )
            ledger?.let {
                Text(it, Modifier.padding(top = 3.dp), fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Muted)
            }
            val isEmpty = groups.needsUser.isEmpty() && groups.handled.isEmpty() && groups.reference.isEmpty()
            if (isEmpty) {
                EmptyState(
                    iconRes = R.drawable.ic_lucide_message_square,
                    title = UnknownEventCopy.EMPTY_TITLE,
                    hint = UnknownEventCopy.EMPTY_HINT,
                )
            } else {
                Spacer(Modifier.height(4.dp))
                val groupBlock: @Composable (String, List<UnknownEventCardModel>) -> Unit = { label, items ->
                    if (items.isNotEmpty()) {
                        Text(
                            "$label · ${items.size}",
                            Modifier.padding(top = 10.dp, bottom = 2.dp),
                            fontSize = PaEditorial.Type.Caption.fontSize,
                            color = PaEditorial.Muted,
                            fontWeight = FontWeight.Medium,
                        )
                        items.forEachIndexed { i, card ->
                            UnknownEventCardRow(
                                card = card,
                                expanded = expandedId == card.eventId,
                                technicalOpen = technicalOpenFor == card.eventId,
                                detail = details[card.eventId],
                                note = notes[card.eventId] ?: "",
                                busy = busyAction?.first == card.eventId,
                                message = messages[card.eventId],
                                onToggle = {
                                    expandedId = if (expandedId == card.eventId) null else card.eventId
                                    technicalOpenFor = null
                                },
                                onToggleTechnical = {
                                    technicalOpenFor = if (technicalOpenFor == card.eventId) null else card.eventId
                                },
                                onEnsureDetail = { ensureDetail(card.eventId) },
                                onNoteChange = { notes[card.eventId] = it },
                                onAction = { action ->
                                    when (action.route) {
                                        UnknownActionRoute.EXPAND_DETAIL -> {
                                            // inspect = 展开详情(内容走既有 GetUnknownEvent)。
                                            technicalOpenFor = card.eventId
                                            ensureDetail(card.eventId)
                                        }
                                        UnknownActionRoute.LOCAL_COPY -> copyTechnical(details[card.eventId]?.detail)
                                        UnknownActionRoute.REMOTE -> {
                                            if (action.requiresConfirmation) {
                                                pendingConfirm = card to action
                                            } else {
                                                runAction(card, action, notes[card.eventId] ?: "")
                                            }
                                        }
                                    }
                                },
                            )
                            if (i < items.lastIndex) {
                                Box(Modifier.fillMaxWidth().height(1.dp).background(PaEditorial.Hairline))
                            }
                        }
                    }
                }
                groupBlock(UnknownEventCopy.GROUP_NEEDS_USER, groups.needsUser)
                groupBlock(UnknownEventCopy.GROUP_HANDLED, groups.handled)
                groupBlock(UnknownEventCopy.GROUP_REFERENCE, groups.reference)
            }
            Spacer(Modifier.height(8.dp))
        }
    }

    pendingConfirm?.let { (card, action) ->
        AlertDialog(
            onDismissRequest = { pendingConfirm = null },
            containerColor = PaEditorial.Surface,
            title = { Text("确认执行「${action.label}」?", fontSize = PaEditorial.Type.Heading.fontSize, color = PaEditorial.Ink) },
            text = {
                Text(
                    action.description.ifBlank { UnknownEventCopy.ACTION_DONE },
                    fontSize = PaEditorial.Type.BodySmall.fontSize,
                    color = PaEditorial.Muted,
                )
            },
            confirmButton = {
                TextButton(onClick = {
                    pendingConfirm = null
                    runAction(card, action, notes[card.eventId] ?: "")
                }) { Text("确认", color = PaEditorial.Primary) }
            },
            dismissButton = {
                TextButton(onClick = { pendingConfirm = null }) { Text("取消", color = PaEditorial.Muted) }
            },
        )
    }
}

/** 详情拉取状态:detail 在手 / 仍在拉(两者皆空)/ 失败([loadFailed])。 */
private data class SheetDetailState(
    val detail: UnknownEventResponseOf? = null,
    val loadFailed: Boolean = false,
)

/** 一行的人话结果(成功=false 色调照抄 daemon detail;冲突/失败=注意色调)。 */
private data class EventResultMessage(val text: String, val attention: Boolean)

/** 处置状态徽标的用色:需要你处理=琥珀,新消息=次级,已处置=中性(词+色双通道)。 */
private fun statusTone(card: UnknownEventCardModel): Color = when {
    card.status == "NEW" && card.requiresUser -> PaEditorial.AttentionText
    card.status == "NEW" -> PaEditorial.Muted
    else -> PaEditorial.NeutralText
}

/**
 * 弹窗里的一条事件:折叠态一行(标题 + 影响 + 来源/时间 + 状态),整行可点展开;
 * 展开体 = 人话解释 + 建议下一步 + 可选备注 + 动作按钮(按 daemon actions 表)+
 * 折叠的「技术详情」(原文/字段透析/翻译/支持包提示)。
 */
@Composable
private fun UnknownEventCardRow(
    card: UnknownEventCardModel,
    expanded: Boolean,
    technicalOpen: Boolean,
    detail: SheetDetailState?,
    note: String,
    busy: Boolean,
    message: EventResultMessage?,
    onToggle: () -> Unit,
    onToggleTechnical: () -> Unit,
    onEnsureDetail: () -> Unit,
    onNoteChange: (String) -> Unit,
    onAction: (UnknownEventActionModel) -> Unit,
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
                Column(Modifier.weight(1f)) {
                    Text(
                        card.title,
                        fontSize = PaEditorial.Type.Body.fontSize,
                        lineHeight = 19.sp,
                        color = PaEditorial.Ink,
                        fontWeight = FontWeight.SemiBold,
                        maxLines = 2,
                        overflow = TextOverflow.Ellipsis,
                    )
                    if (card.impact.isNotBlank()) {
                        Text(
                            card.impact,
                            Modifier.padding(top = 2.dp),
                            fontSize = PaEditorial.Type.Caption.fontSize,
                            lineHeight = 16.sp,
                            color = PaEditorial.Muted,
                            maxLines = 2,
                            overflow = TextOverflow.Ellipsis,
                        )
                    }
                    val meta = listOf(card.sourceAgent, relativeTimeLabel(card.receivedAtUnixMs))
                        .filter { it.isNotBlank() }
                        .joinToString(" · ")
                    if (meta.isNotEmpty()) {
                        Text(
                            meta,
                            Modifier.padding(top = 2.dp),
                            fontSize = PaEditorial.Type.Caption.fontSize,
                            color = PaEditorial.Subtle,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                        )
                    }
                }
                Spacer(Modifier.width(8.dp))
                Text(
                    unknownStatusLabel(card.status),
                    fontSize = PaEditorial.Type.Micro.fontSize,
                    lineHeight = 14.sp,
                    color = statusTone(card),
                    fontWeight = FontWeight.SemiBold,
                )
                Spacer(Modifier.width(6.dp))
                Icon(
                    painterResource(
                        if (expanded) R.drawable.ic_lucide_chevron_up else R.drawable.ic_lucide_chevron_down
                    ),
                    contentDescription = null,
                    tint = PaEditorial.Subtle,
                    modifier = Modifier.size(14.dp),
                )
            }
            if (card.degraded) {
                Text(
                    UnknownEventCopy.DEGRADED_NOTE,
                    Modifier.padding(top = 3.dp),
                    fontSize = PaEditorial.Type.Caption.fontSize,
                    color = PaEditorial.Muted,
                )
            }
        }

        // ---- 展开态:spring 驱动、可打断(连点展开/收起从当前高度续走) ----
        AnimatedVisibility(
            visible = expanded,
            enter = expandVertically(PaMotion.sheet()) + fadeIn(PaMotion.sheet()),
            exit = shrinkVertically(PaMotion.standard()) + fadeOut(PaMotion.standard()),
        ) {
            LaunchedEffect(card.eventId) {
                // 内容进组合 = 首次展开(或再次展开);缓存命中时 onEnsureDetail 直接短路。
                if (detail == null) onEnsureDetail()
            }
            Column(Modifier.fillMaxWidth().padding(bottom = 12.dp)) {
                if (card.explanation.isNotBlank()) {
                    Text(
                        card.explanation,
                        Modifier.padding(top = 4.dp),
                        fontSize = PaEditorial.Type.BodySmall.fontSize,
                        lineHeight = 18.sp,
                        color = PaEditorial.Muted,
                    )
                }
                // daemon 建议的下一步(人话);详情未在手时先不占位,技术区加载完即出现。
                detail?.detail?.recommendedNextStep?.let {
                    Text(
                        it,
                        Modifier.padding(top = 6.dp),
                        fontSize = PaEditorial.Type.Caption.fontSize,
                        lineHeight = 17.sp,
                        color = PaEditorial.Ink,
                    )
                }

                // ---- 动作区:按 daemon actions 表渲染(safe=true 才是可点按钮)----
                val usable = card.actions.filter { it.safe }
                if (usable.isNotEmpty()) {
                    OutlinedTextField(
                        value = note,
                        onValueChange = onNoteChange,
                        Modifier
                            .fillMaxWidth()
                            .padding(top = 6.dp),
                        singleLine = true,
                        label = { Text(UnknownEventCopy.NOTE_LABEL, fontSize = PaEditorial.Type.Caption.fontSize) },
                        placeholder = { Text(UnknownEventCopy.NOTE_PLACEHOLDER, fontSize = PaEditorial.Type.Caption.fontSize) },
                    )
                    // 一行塞不下全部按钮(窄屏会挤到换行截断),三个一排。
                    usable.chunked(3).forEach { rowActions ->
                        Row(Modifier.fillMaxWidth()) {
                            rowActions.forEach { a -> ActionButton(a, busy, onAction) }
                        }
                    }
                }
                message?.let {
                    Text(
                        it.text,
                        Modifier.padding(top = 2.dp),
                        fontSize = PaEditorial.Type.Caption.fontSize,
                        lineHeight = 17.sp,
                        color = if (it.attention) EditorialTone.ATTENTION.accent else PaEditorial.RunningText,
                    )
                }

                // ---- 技术详情(折叠):原文 / 字段透析 / correlation id 全在这里 ----
                Row(
                    Modifier
                        .fillMaxWidth()
                        .pressable { onToggleTechnical() }
                        .padding(vertical = 8.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(
                        if (technicalOpen) UnknownEventCopy.TECHNICAL_HIDE else UnknownEventCopy.TECHNICAL_SHOW,
                        fontSize = PaEditorial.Type.Caption.fontSize,
                        color = PaEditorial.Primary,
                    )
                    Icon(
                        painterResource(
                            if (technicalOpen) R.drawable.ic_lucide_chevron_up
                            else R.drawable.ic_lucide_chevron_down
                        ),
                        contentDescription = null,
                        tint = PaEditorial.Primary,
                        modifier = Modifier.size(13.dp),
                    )
                }
                AnimatedVisibility(
                    visible = technicalOpen,
                    enter = expandVertically(PaMotion.sheet()) + fadeIn(PaMotion.sheet()),
                    exit = shrinkVertically(PaMotion.standard()) + fadeOut(PaMotion.standard()),
                ) {
                    Column(Modifier.fillMaxWidth().padding(bottom = 4.dp)) {
                        Text(
                            UnknownEventCopy.TECHNICAL_SECTION_TITLE,
                            fontSize = PaEditorial.Type.Caption.fontSize,
                            color = PaEditorial.Muted,
                            fontWeight = FontWeight.Medium,
                        )
                        when {
                            detail == null || (!detail.loadFailed && detail.detail == null) -> Text(
                                UnknownEventCopy.DETAIL_LOADING,
                                Modifier.padding(top = 4.dp),
                                fontSize = PaEditorial.Type.Caption.fontSize,
                                color = PaEditorial.Muted,
                            )
                            detail.loadFailed -> Text(
                                UnknownEventCopy.DETAIL_LOAD_FAILED,
                                Modifier.padding(top = 4.dp),
                                fontSize = PaEditorial.Type.Caption.fontSize,
                                lineHeight = 17.sp,
                                color = EditorialTone.ATTENTION.accent,
                            )
                            else -> TechnicalDetailBody(detail.detail!!)
                        }
                    }
                }
            }
        }
    }
}

/** 技术详情正文:详细解读 + 字段透析表(含 correlation id 行)+ 脱敏原文(可滚动代码箱)。 */
@Composable
private fun TechnicalDetailBody(d: UnknownEventResponseOf) {
    d.translation?.let {
        Text(
            it,
            Modifier.padding(top = 6.dp),
            fontSize = PaEditorial.Type.Caption.fontSize,
            lineHeight = 17.sp,
            color = PaEditorial.Muted,
        )
    }
    if (d.digest.isNotEmpty()) {
        Text(
            UnknownEventCopy.FIELD_DIGEST_TITLE,
            Modifier.padding(top = 10.dp, bottom = 2.dp),
            fontSize = PaEditorial.Type.Caption.fontSize,
            color = PaEditorial.Muted,
            fontWeight = FontWeight.Medium,
        )
        d.digest.forEach { DigestRow(it) }
    }
    Text(
        technicalRawText(d),
        Modifier
            .fillMaxWidth()
            .padding(top = 8.dp)
            .heightIn(max = 220.dp)
            .verticalScroll(rememberScrollState())
            .background(PaEditorial.GraphiteCode, RoundedCornerShape(PaEditorial.Radius.IconButton))
            .padding(10.dp),
        fontSize = PaEditorial.Type.Caption.fontSize,
        fontFamily = PaEditorial.MonoFont,
        color = PaEditorial.CodeText,
    )
    d.supportBundleHint?.let {
        Text(
            it,
            Modifier.padding(top = 4.dp),
            fontSize = PaEditorial.Type.Caption.fontSize,
            lineHeight = 16.sp,
            color = PaEditorial.Subtle,
        )
    }
}

/** 字段级透析表的一行:key / 含义 / 预览 三列紧凑排;daemon 没给出含义就只排两列。 */
@Composable
private fun DigestRow(row: UnknownEventDigestRow) {
    Row(Modifier.fillMaxWidth().padding(vertical = 3.dp)) {
        Text(
            row.key,
            Modifier.weight(0.9f),
            fontSize = PaEditorial.Type.Caption.fontSize,
            fontFamily = PaEditorial.MonoFont,
            color = PaEditorial.Ink,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
        if (row.meaning != null) {
            Text(
                row.meaning!!,
                Modifier.weight(1f).padding(horizontal = 6.dp),
                fontSize = PaEditorial.Type.Caption.fontSize,
                color = PaEditorial.Muted,
            )
        }
        Text(
            row.preview,
            Modifier.weight(1.1f),
            fontSize = PaEditorial.Type.Caption.fontSize,
            color = PaEditorial.Muted,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
    }
}

/** 原文区内容:脱敏原文在前,"解出的字段"随后(沿用旧详情弹窗的两段式)。 */
private fun technicalRawText(d: UnknownEventResponseOf): String =
    listOfNotNull(
        d.rawJson.takeIf { it.isNotBlank() },
        d.decodedFieldsJson.takeIf { it.isNotBlank() }?.let { "解出的字段:$it" },
    ).joinToString("\n\n").ifBlank { "(载荷为空)" }

/** 动作按钮:文案照抄 daemon 的 label(客户端不翻译);destructive 按危险样式。 */
@Composable
private fun ActionButton(
    action: UnknownEventActionModel,
    busy: Boolean,
    onAction: (UnknownEventActionModel) -> Unit,
) {
    TextButton(onClick = { onAction(action) }, enabled = !busy) {
        Text(
            action.label,
            fontSize = PaEditorial.Type.BodySmall.fontSize,
            color = if (action.destructive) EditorialTone.DENIED.text else PaEditorial.Primary,
        )
    }
}
