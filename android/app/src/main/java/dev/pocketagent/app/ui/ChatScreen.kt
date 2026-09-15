package dev.pocketagent.app.ui

import android.graphics.BitmapFactory
import android.content.Context
import android.content.Intent
import android.net.Uri
import android.provider.OpenableColumns
import android.widget.Toast
import androidx.activity.compose.BackHandler
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.tween
import androidx.compose.animation.slideInHorizontally
import androidx.compose.animation.slideOutHorizontally
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.ModalBottomSheetProperties
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties
import com.google.protobuf.ByteString
import androidx.compose.runtime.collectAsState
import dev.pocketagent.app.AgentConnection
import dev.pocketagent.app.R as AppR
import dev.pocketagent.app.ui.anim.IconActionButton
import dev.pocketagent.app.ui.anim.edgeSwipeToOpenDrawer
import dev.pocketagent.app.ui.anim.pressable
import dev.pocketagent.app.ui.component.PaDownloader
import dev.pocketagent.app.ui.component.TypingDots
import dev.pocketagent.app.ui.theme.Pa
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import pocketagent.v1.ApprovalDecision
import pocketagent.v1.ApprovalRequest
import pocketagent.v1.ContentBlock
import pocketagent.v1.CodexEvent
import pocketagent.v1.CreateThreadRequest
import pocketagent.v1.CodexSettings
import pocketagent.v1.GetCodexSettingsRequest
import pocketagent.v1.GetHostInfoRequest
import pocketagent.v1.GitStatusRequest
import pocketagent.v1.GitDiffRequest
import pocketagent.v1.GitCommitRequest
import pocketagent.v1.GetThreadHistoryRequest
import pocketagent.v1.GetThreadRequest
import pocketagent.v1.ListModelsRequest
import pocketagent.v1.ListThreadsRequest
import pocketagent.v1.ModelInfo
import pocketagent.v1.SetCodexSettingsRequest
import pocketagent.v1.TextBlock
import pocketagent.v1.Thread as PThread
import pocketagent.v1.ToolCallUpdate
import pocketagent.v1.TurnClientMessage
import pocketagent.v1.TurnInput
import pocketagent.v1.TurnServerMessage
import pocketagent.v1.Usage
import pocketagent.v1.WriteFileRequest

/** 一条聊天记录(用户/助手/工具过程/思考过程/执行计划)。 */
data class ChatItem(
    val role: String, // user / assistant / tool / thinking / plan / codex_event
    val text: String = "",
    val toolName: String = "",
    val toolStatus: String = "", // running / success / failure
    val toolId: String = "",
    val toolArgs: String = "",
    val toolElapsedMs: Double = 0.0,
    val streaming: Boolean = false,
    val attachments: List<Attachment> = emptyList(),
    val codexEvent: CodexEvent? = null,
    /** 执行计划步骤(turn/plan),每项 "status|text"。 */
    val planSteps: List<String> = emptyList(),
)

/** 已上传到电脑的附件(手机 → daemon 工作区 .pocketagent-uploads)。 */
data class Attachment(
    val name: String,
    val pcPath: String,
    val size: Int,
    val mediaType: String = "",
    val localUri: String? = null,
    val previewData: ByteArray? = null,
)

private fun loadAttachmentPreview(context: Context, attachment: Attachment): ImageBitmap? {
    return runCatching {
        if (attachment.previewData != null) {
            BitmapFactory.decodeByteArray(
                attachment.previewData,
                0,
                attachment.previewData.size,
            )?.asImageBitmap()
        } else {
            val uri = attachment.localUri?.let(Uri::parse) ?: return@runCatching null
            context.contentResolver.openInputStream(uri)?.use { BitmapFactory.decodeStream(it)?.asImageBitmap() }
        }
    }.getOrNull()
}

@Composable
private fun CodexEventCard(event: CodexEvent) {
    var expanded by remember(event.rawJson) { mutableStateOf(AgentConnection.toolDetailsExpanded) }
    val color = if (event.kind == CodexEvent.Kind.ERROR) Pa.Red else Pa.Accent
    Column(
        Modifier
            .fillMaxWidth()
            .background(Pa.Card, RoundedCornerShape(14.dp))
            .padding(horizontal = 12.dp, vertical = 10.dp),
        verticalArrangement = Arrangement.spacedBy(5.dp),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Text(
                buildString {
                    append(event.title.ifBlank { "Codex 状态" })
                    if (event.httpStatus > 0) append(" · HTTP ${event.httpStatus}")
                    if (event.willRetry) append(" · 正在重试")
                },
                Modifier.weight(1f),
                color = color,
                fontSize = 13.sp,
                fontWeight = FontWeight.SemiBold,
            )
            if (event.rawJson.isNotBlank()) {
                TextButton(
                    onClick = { expanded = !expanded },
                    contentPadding = PaddingValues(horizontal = 4.dp, vertical = 0.dp),
                ) {
                    Text(if (expanded) "收起" else "详情", color = Pa.Accent, fontSize = 11.sp)
                }
            }
        }
        if (event.message.isNotBlank()) {
            Text(event.message, color = Pa.TextPrimary, fontSize = 12.sp)
        }
        if (event.url.isNotBlank()) {
            Text(
                event.url,
                color = Pa.TextTertiary,
                fontSize = 10.sp,
                fontFamily = FontFamily.Monospace,
            )
        }
        if (expanded) {
            Text(
                event.rawJson,
                Modifier
                    .fillMaxWidth()
                    .background(Pa.Bg, RoundedCornerShape(8.dp))
                    .padding(8.dp),
                color = Pa.TextSecondary,
                fontSize = 10.sp,
                fontFamily = FontFamily.Monospace,
            )
        }
    }
}

/**
 * 思考过程卡:可折叠。
 * 流式时默认展开(边想边看),落定后默认收起(避免长思考淹没正文)。
 */
@Composable
private fun ThinkingCard(text: String, scale: Float, streaming: Boolean) {
    var expanded by remember(streaming) { mutableStateOf(streaming) }
    Column(
        Modifier
            .fillMaxWidth()
            .background(Pa.Card, RoundedCornerShape(14.dp))
            .padding(horizontal = 12.dp, vertical = 10.dp),
    ) {
        Row(
            Modifier.fillMaxWidth().pressable(haptic = true) { expanded = !expanded },
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Icon(
                painterResource(AppR.drawable.ic_lucide_brain), null,
                tint = Pa.Accent, modifier = Modifier.size(13.dp),
            )
            Spacer(Modifier.width(6.dp))
            Text(
                "思考过程" + if (streaming) " · 正在思考" else "",
                Modifier.weight(1f),
                fontSize = 11.5.sp,
                color = Pa.Accent,
                fontWeight = FontWeight.SemiBold,
                maxLines = 1,
            )
            Text(if (expanded) "收起" else "展开", fontSize = 11.sp, color = Pa.TextTertiary)
        }
        if (expanded) {
            Text(
                text,
                Modifier.fillMaxWidth().padding(top = 6.dp),
                fontSize = (11.5f * scale).sp,
                fontFamily = FontFamily.Monospace,
                color = Pa.TextSecondary,
            )
        }
    }
}

/**
 * 执行计划卡:codex turn/plan 的步骤列表。
 * 状态:completed=实心绿 / in_progress=蓝色(带呼吸)/ pending=空心灰;
 * 流式阶段还可附带"生成中"的草稿文本。
 */
@Composable
private fun PlanCard(steps: List<String>, draft: String, streaming: Boolean) {
    Column(
        Modifier
            .fillMaxWidth()
            .background(Pa.Card, RoundedCornerShape(14.dp))
            .padding(horizontal = 12.dp, vertical = 10.dp),
    ) {
        Row(
            Modifier.fillMaxWidth(),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Icon(
                painterResource(AppR.drawable.ic_lucide_list_checks), null,
                tint = Pa.Accent, modifier = Modifier.size(13.dp),
            )
            Spacer(Modifier.width(6.dp))
            Text(
                "执行计划" + if (streaming) " · 制定中" else "",
                Modifier.weight(1f),
                fontSize = 11.5.sp,
                color = Pa.Accent,
                fontWeight = FontWeight.SemiBold,
                maxLines = 1,
            )
        }
        Spacer(Modifier.height(6.dp))
        steps.forEach { raw ->
            val status = raw.substringBefore('|')
            val text = raw.substringAfter('|', "")
            if (text.isNotBlank()) {
                Row(
                    Modifier.fillMaxWidth().padding(vertical = 3.dp),
                    verticalAlignment = Alignment.Top,
                ) {
                    val dotColor = when (status) {
                        "completed" -> Pa.Green
                        "in_progress" -> Pa.Accent
                        else -> Pa.TextTertiary
                    }
                    Box(
                        Modifier
                            .padding(top = 4.dp)
                            .size(7.dp)
                            .background(dotColor, CircleShape),
                    )
                    Spacer(Modifier.width(8.dp))
                    Text(
                        text,
                        fontSize = 12.sp,
                        color = if (status == "pending") Pa.TextSecondary else Pa.TextPrimary,
                    )
                }
            }
        }
        if (draft.isNotBlank() && steps.isEmpty()) {
            Text(
                draft,
                fontSize = 11.5.sp,
                fontFamily = FontFamily.Monospace,
                color = Pa.TextSecondary,
            )
        }
    }
}

/**
 * 工具调用卡:一次调用一张卡(同一 call id 原地更新),
 * 展开可见调用的参数与输出,并显示耗时与成功/失败状态。
 */
@Composable
private fun ToolCallCard(item: ChatItem, scale: Float) {
    // 运行中默认展开(看得到正在干什么),结束后默认收起
    var expanded by remember(item.toolId) { mutableStateOf(item.toolStatus == "running") }
    val statusColor = when (item.toolStatus) {
        "failure" -> Pa.Red
        "success" -> Pa.Green
        else -> Pa.Accent
    }
    Column(
        Modifier
            .fillMaxWidth()
            .background(Pa.Card, RoundedCornerShape(12.dp))
            .padding(horizontal = 12.dp, vertical = 10.dp),
    ) {
        Row(
            Modifier.fillMaxWidth().pressable(haptic = true) { expanded = !expanded },
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Box(Modifier.size(8.dp).background(statusColor, CircleShape))
            Spacer(Modifier.width(8.dp))
            Text(
                item.toolName.ifBlank { "工具" },
                Modifier.weight(1f),
                fontSize = (13 * scale).sp,
                color = Pa.TextPrimary,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            if (item.toolElapsedMs > 0) {
                Text(
                    String.format(java.util.Locale.US, "%.1fs", item.toolElapsedMs / 1000.0),
                    fontSize = 10.5.sp,
                    color = Pa.TextTertiary,
                )
                Spacer(Modifier.width(8.dp))
            }
            Text(
                when (item.toolStatus) {
                    "failure" -> "失败"
                    "success" -> "完成"
                    else -> "运行中"
                },
                fontSize = 10.5.sp,
                color = statusColor,
            )
        }
        if (expanded) {
            if (item.toolArgs.isNotBlank()) {
                Text("参数", Modifier.padding(top = 8.dp), fontSize = 10.5.sp, color = Pa.TextTertiary)
                Text(
                    item.toolArgs,
                    Modifier
                        .fillMaxWidth()
                        .padding(top = 2.dp)
                        .background(Pa.Bg, RoundedCornerShape(8.dp))
                        .padding(8.dp),
                    fontSize = (10.5f * scale).sp,
                    fontFamily = FontFamily.Monospace,
                    color = Pa.TextPrimary,
                )
            }
            if (item.text.isNotBlank()) {
                Text(
                    if (item.toolStatus == "failure") "错误" else "输出",
                    Modifier.padding(top = 8.dp),
                    fontSize = 10.5.sp,
                    color = Pa.TextTertiary,
                )
                Text(
                    item.text,
                    Modifier
                        .fillMaxWidth()
                        .padding(top = 2.dp)
                        .background(Pa.Bg, RoundedCornerShape(8.dp))
                        .padding(8.dp),
                    fontSize = (10.5f * scale).sp,
                    fontFamily = FontFamily.Monospace,
                    color = if (item.toolStatus == "failure") Pa.Red else Pa.TextSecondary,
                )
            }
            if (item.toolArgs.isBlank() && item.text.isBlank()) {
                Text("等待结果…", Modifier.padding(top = 8.dp), fontSize = 11.sp, color = Pa.TextTertiary)
            }
        }
    }
}

/**
 * 上下文窗口明细 —— 点输入栏的"上下文"胶囊弹出(问题 11:替换旧样式)。
 *
 * 【为什么改】旧版是居中小弹窗 + 五条各自独立的细进度条:
 * 内容本身就多(本轮用量 / 累计用量 / 构成估算),弹窗里要一直往下滚,
 * 而且五条独立条形看不出"上下文整体被占了多满"。
 *
 * 【新结构】
 *   1. 底部大卡片(ModalBottomSheet)—— 内容多时就该用竖向空间,而不是挤一个框。
 *   2. 顶部「主视觉」把总量画成一条**容量条**:已用段(输入/输出)+ 剩余空间同框,
 *      "还能装多少"一眼可见,比五个百分比数字直观得多。
 *   3. 本轮 / 累计改成**分段切换**,不再上下堆两遍,少滚一半。
 *
 * 【口径】"合计"只算 输入 + 输出。缓存读/缓存写是输入的**子集**、思考是输出的子集,
 * 若一并相加会把同一批 token 数两遍 —— 明细里照列(便于排查),但绝不参与合计。
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun ContextWindowCard(
    model: ModelInfo?,
    contextWindow: Long,
    turnUsage: Usage?,
    totalUsage: Usage?,
    items: List<ChatItem>,
    onDismiss: () -> Unit,
) {
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    var scopeTab by remember { mutableStateOf(0) } // 0 = 本轮, 1 = 本会话累计
    val usage = if (scopeTab == 0) turnUsage else totalUsage
    // daemon 还没上报过用量时的兜底:按会话内容字符量估一个占用,
    // 面板不会再显示"0%"或"暂无数据"这种看起来像坏了的状态。
    val estimated = remember(items) { estimateTokens(items) }
    val showEstimate = usage == null && estimated > 0

    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = sheetState,
        containerColor = Pa.CardHigh,
        shape = RoundedCornerShape(topStart = 26.dp, topEnd = 26.dp),
    ) {
        Column(
            Modifier
                .fillMaxWidth()
                .padding(horizontal = 20.dp)
                .navigationBarsPadding()
                .padding(bottom = 18.dp),
        ) {
            // ---------- 标题行 ----------
            Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                Column(Modifier.weight(1f)) {
                    Text("上下文窗口", fontSize = 20.sp, color = Pa.TextPrimary, fontWeight = FontWeight.SemiBold)
                    Spacer(Modifier.height(2.dp))
                    Text(
                        model?.displayName ?: "跟随桌面配置",
                        fontSize = 11.5.sp,
                        color = Pa.TextTertiary,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                }
                IconActionButton(
                    AppR.drawable.ic_lucide_x, "关闭", onDismiss,
                    bg = Color.Transparent, size = 36.dp, iconSize = 18.dp,
                )
            }

            Spacer(Modifier.height(14.dp))

            // ---------- 主视觉:容量条 ----------
            ContextHero(
                usage = usage,
                contextWindow = contextWindow,
                estimatedTokens = if (showEstimate) estimated else 0L,
            )

            Spacer(Modifier.height(16.dp))

            // ---------- 本轮 / 累计 分段切换 ----------
            Row(
                Modifier
                    .fillMaxWidth()
                    .background(Pa.Bg, RoundedCornerShape(50))
                    .padding(4.dp),
            ) {
                ScopeTab("本轮", scopeTab == 0, Modifier.weight(1f)) { scopeTab = 0 }
                ScopeTab("本会话累计", scopeTab == 1, Modifier.weight(1f)) { scopeTab = 1 }
            }

            Spacer(Modifier.height(14.dp))

            // ---------- 明细 ----------
            if (usage == null && showEstimate) {
                Column(
                    Modifier
                        .fillMaxWidth()
                        .background(Pa.Card, RoundedCornerShape(18.dp))
                        .padding(horizontal = 14.dp, vertical = 12.dp),
                ) {
                    TokenRow("估算输入", estimated, contextWindow, Pa.Accent)
                    Spacer(Modifier.height(4.dp))
                    Box(Modifier.fillMaxWidth().height(1.dp).background(Pa.Separator))
                    Spacer(Modifier.height(10.dp))
                    Text(
                        "合计 ~${String.format(java.util.Locale.US, "%,d", estimated)} tokens",
                        fontSize = 11.5.sp,
                        color = Pa.TextPrimary,
                    )
                    Text(
                        "由会话内容字符量估算(≈4 字符/token);daemon 下次上报后自动切换为真实值。",
                        Modifier.padding(top = 3.dp),
                        fontSize = 10.sp,
                        color = Pa.TextTertiary,
                    )
                }
            } else if (usage == null) {
                Text(
                    "暂无数据(尚未收到用量上报)",
                    Modifier
                        .fillMaxWidth()
                        .background(Pa.Card, RoundedCornerShape(14.dp))
                        .padding(14.dp),
                    fontSize = 12.sp,
                    color = Pa.TextTertiary,
                )
            } else {
                Column(
                    Modifier
                        .fillMaxWidth()
                        .background(Pa.Card, RoundedCornerShape(18.dp))
                        .padding(horizontal = 14.dp, vertical = 12.dp),
                ) {
                    TokenRow("输入", usage.inputTokens, contextWindow, Pa.Accent)
                    TokenRow("输出", usage.outputTokens, contextWindow, Pa.Green)
                    TokenRow("思考", usage.reasoningTokens, contextWindow, Pa.AccentText)
                    TokenRow("缓存读", usage.cacheReadTokens, contextWindow, Pa.Accent.copy(alpha = 0.55f))
                    TokenRow("缓存写", usage.cacheWriteTokens, contextWindow, Pa.TextTertiary)

                    Spacer(Modifier.height(4.dp))
                    Box(Modifier.fillMaxWidth().height(1.dp).background(Pa.Separator))
                    Spacer(Modifier.height(10.dp))

                    val total = usage.inputTokens + usage.outputTokens
                    val hitRate = if (usage.inputTokens > 0) {
                        usage.cacheReadTokens.toFloat() / usage.inputTokens * 100f
                    } else -1f
                    Text(
                        buildString {
                            append("合计 ")
                            append(String.format(java.util.Locale.US, "%,d", total))
                            append(" tokens")
                            if (hitRate >= 0) {
                                append(" · 缓存命中 ")
                                append(String.format(java.util.Locale.US, "%.1f%%", hitRate))
                            }
                            if (usage.costUsd > 0) {
                                append(" · 成本 $")
                                append(String.format(java.util.Locale.US, "%.4f", usage.costUsd))
                            }
                        },
                        fontSize = 11.5.sp,
                        color = Pa.TextPrimary,
                    )
                    Text(
                        "缓存读/写包含在「输入」内、思考包含在「输出」内,故不计入合计。",
                        Modifier.padding(top = 3.dp),
                        fontSize = 10.sp,
                        color = Pa.TextTertiary,
                    )
                }
            }

            Spacer(Modifier.height(18.dp))

            EstimateSection(items)
        }
    }
}

/**
 * 主视觉:一条"容量条"。
 *
 * 底槽 = 整个上下文窗口;填充 = 输入(蓝) + 输出(绿);剩下的深色部分就是**还能装多少**。
 * 右侧百分比 = 占用 / 窗口。窗口未知时退化成"只用不占比",并明确提示去发一轮消息。
 */
/**
 * 会话内容的 token 估算(≈4 字符/token)。
 * 仅用于 daemon 尚未上报用量时的兜底展示,不作计费依据。
 */
private fun estimateTokens(items: List<ChatItem>): Long {
    var chars = 0L
    items.forEach { chars += it.text.length + it.toolArgs.length }
    return chars / 4
}

@Composable
private fun ContextHero(usage: Usage?, contextWindow: Long, estimatedTokens: Long = 0) {
    val isEstimate = usage == null && estimatedTokens > 0
    val input = if (isEstimate) estimatedTokens else (usage?.inputTokens ?: 0)
    val output = if (isEstimate) 0L else (usage?.outputTokens ?: 0)
    val used = input + output
    val known = contextWindow > 0
    val ratio = if (known) (used.toFloat() / contextWindow).coerceIn(0f, 1f) else 0f
    // 占用色:正常蓝,接近/超过窗口时转红,提示"该清上下文了"
    val barColor = when {
        !known -> Pa.Accent
        ratio >= 0.9f -> Pa.Red
        ratio >= 0.7f -> Pa.Yellow
        else -> Pa.Accent
    }

    Column(
        Modifier
            .fillMaxWidth()
            .background(Pa.Card, RoundedCornerShape(18.dp))
            .padding(horizontal = 16.dp, vertical = 14.dp),
    ) {
        Row(verticalAlignment = Alignment.Bottom) {
            Text(
                if (usage == null && !isEstimate) "—" else String.format(java.util.Locale.US, "%,d", used),
                fontSize = 26.sp,
                color = Pa.TextPrimary,
                fontWeight = FontWeight.SemiBold,
            )
            if (isEstimate) {
                Spacer(Modifier.width(6.dp))
                Text(
                    "估算",
                    Modifier
                        .padding(bottom = 4.dp)
                        .background(Pa.Hover, RoundedCornerShape(50))
                        .padding(horizontal = 7.dp, vertical = 2.dp),
                    fontSize = 9.5.sp,
                    color = Pa.TextSecondary,
                )
            }
            Spacer(Modifier.width(6.dp))
            Text(
                if (known) " / ${contextWindow / 1000}K tokens" else " tokens 已用",
                Modifier.padding(bottom = 4.dp),
                fontSize = 12.sp,
                color = Pa.TextTertiary,
            )
            Spacer(Modifier.weight(1f))
            Text(
                if (known) String.format(java.util.Locale.US, "%.1f%%", ratio * 100f) else "窗口未知",
                Modifier
                    .background(
                        if (known) barColor.copy(alpha = 0.16f) else Pa.Hover,
                        RoundedCornerShape(50),
                    )
                    .padding(horizontal = 9.dp, vertical = 3.dp),
                fontSize = 11.sp,
                color = if (known) barColor else Pa.TextTertiary,
                fontWeight = FontWeight.Medium,
            )
        }

        Spacer(Modifier.height(12.dp))

        // 容量条:已用段 + 剩余段同框
        Row(
            Modifier
                .fillMaxWidth()
                .height(10.dp)
                .background(Pa.Bg, RoundedCornerShape(50)),
        ) {
            if (known && used > 0) {
                Row(
                    Modifier
                        .weight(ratio.coerceAtLeast(0.012f), fill = false)
                        .fillMaxHeight(),
                ) {
                    if (input > 0) {
                        Box(
                            Modifier
                                .weight(input.toFloat() / used, fill = false)
                                .fillMaxHeight()
                                .background(Pa.Accent),
                        )
                    }
                    if (output > 0) {
                        Box(
                            Modifier
                                .weight(output.toFloat() / used, fill = false)
                                .fillMaxHeight()
                                .background(Pa.Green),
                        )
                    }
                }
            }
            Spacer(Modifier.weight((1f - ratio).coerceAtLeast(0.001f)))
        }

        Spacer(Modifier.height(10.dp))

        // 图例:把"已用 / 剩余"翻译成人话
        Row(verticalAlignment = Alignment.CenterVertically) {
            LegendDot(Pa.Accent)
            Text(
                "输入 ${String.format(java.util.Locale.US, "%,d", input)}",
                fontSize = 10.5.sp, color = Pa.TextSecondary,
            )
            Spacer(Modifier.width(12.dp))
            LegendDot(Pa.Green)
            Text(
                "输出 ${String.format(java.util.Locale.US, "%,d", output)}",
                fontSize = 10.5.sp, color = Pa.TextSecondary,
            )
            Spacer(Modifier.weight(1f))
            Text(
                if (known) "剩余 ${(contextWindow - used).coerceAtLeast(0) / 1000}K" else "",
                fontSize = 10.5.sp, color = Pa.TextTertiary,
            )
        }

        if (!known) {
            Text(
                "发一轮消息后,电脑端会随用量上报真实窗口大小。",
                Modifier.padding(top = 8.dp),
                fontSize = 10.5.sp,
                color = Pa.TextTertiary,
            )
        }
    }
}

@Composable
private fun LegendDot(color: Color) {
    Box(Modifier.size(7.dp).background(color, CircleShape))
    Spacer(Modifier.width(5.dp))
}

/** 分段切换按钮(选中=蓝底白字)。 */
@Composable
private fun ScopeTab(label: String, selected: Boolean, modifier: Modifier = Modifier, onClick: () -> Unit) {
    Box(
        modifier
            .pressable(haptic = true) { onClick() }
            .background(if (selected) Pa.Accent else Color.Transparent, RoundedCornerShape(50))
            .padding(vertical = 9.dp),
        contentAlignment = Alignment.Center,
    ) {
        Text(
            label,
            fontSize = 12.5.sp,
            color = if (selected) Color.White else Pa.TextSecondary,
            fontWeight = if (selected) FontWeight.Medium else FontWeight.Normal,
        )
    }
}

/** 一行 token 维度:名称 · 数值 · 占比条 · 百分比。 */
@Composable
private fun TokenRow(label: String, value: Long, contextWindow: Long, color: Color) {
    val pct = if (contextWindow > 0) value.toFloat() / contextWindow * 100f else 0f
    val over = pct > 100f
    Row(
        Modifier.fillMaxWidth().padding(vertical = 5.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(label, Modifier.width(46.dp), fontSize = 11.5.sp, color = Pa.TextSecondary)
        Text(
            String.format(java.util.Locale.US, "%,d", value),
            Modifier.width(68.dp),
            fontSize = 11.5.sp,
            color = Pa.TextPrimary,
            textAlign = TextAlign.End,
        )
        Spacer(Modifier.width(10.dp))
        Box(
            Modifier
                .weight(1f)
                .height(6.dp)
                .background(Pa.Bg, RoundedCornerShape(50)),
        ) {
            Box(
                Modifier
                    .fillMaxWidth((pct / 100f).coerceIn(0f, 1f))
                    .fillMaxHeight()
                    .background(if (over) Pa.Red else color, RoundedCornerShape(50)),
            )
        }
        Spacer(Modifier.width(8.dp))
        Text(
            if (over) "超窗" else String.format(java.util.Locale.US, "%.1f%%", pct),
            Modifier.width(44.dp),
            fontSize = 10.5.sp,
            color = if (over) Pa.Red else Pa.TextTertiary,
            textAlign = TextAlign.End,
        )
    }
}

/**
 * 构成估算:按当前时间线各部分的文字量粗估 token(≈4 字符/token),
 * 回答"上下文都被谁占了"。协议未提供逐项精确值,这里只是参考。
 */
@Composable
private fun EstimateSection(items: List<ChatItem>) {
    Text("构成估算", fontSize = 13.sp, color = Pa.TextPrimary, fontWeight = FontWeight.SemiBold)
    Spacer(Modifier.height(2.dp))
    Text("按会话内容字符量估算(≈4 字符/token),仅供参考", fontSize = 10.sp, color = Pa.TextTertiary)
    Spacer(Modifier.height(10.dp))
    if (items.isEmpty()) {
        Text("暂无会话内容", fontSize = 11.sp, color = Pa.TextTertiary)
        return
    }
    var dialog = 0L      // 对话消息(user/assistant)
    var thinking = 0L    // 思考过程
    var tools = 0L       // 普通工具调用(含子 agent/技能,按工具名区分不出的归这里)
    var mcp = 0L         // MCP 工具(名称以 mcp 开头或含 __ 分隔)
    var other = 0L       // 其他事件
    items.forEach { it ->
        val textLen = (it.text.length + it.toolArgs.length).toLong()
        when {
            it.role == "user" || it.role == "assistant" -> dialog += it.text.length.toLong()
            it.role == "thinking" -> thinking += it.text.length.toLong()
            it.role == "tool" -> {
                val isMcp = it.toolName.startsWith("mcp") || it.toolName.contains("__")
                if (isMcp) mcp += textLen else tools += textLen
            }
            else -> other += textLen
        }
    }
    val charTotal = (dialog + thinking + tools + mcp + other).coerceAtLeast(1L)
    data class Part(val label: String, val chars: Long, val color: Color)
    val parts = listOf(
        Part("对话消息", dialog, Pa.Accent),
        Part("思考", thinking, Pa.AccentText),
        Part("工具/子agent/技能", tools, Pa.Green),
        Part("MCP", mcp, Pa.Mcp),
        Part("其他", other, Pa.TextTertiary),
    )

    // 堆叠条:段与段之间留 2dp 缝隙,避免同色相邻时糊成一块
    Row(
        Modifier
            .fillMaxWidth()
            .height(10.dp)
            .background(Pa.Bg, RoundedCornerShape(50)),
    ) {
        parts.forEach { p ->
            if (p.chars > 0) {
                Box(
                    Modifier
                        .weight(p.chars.toFloat() / charTotal, fill = false)
                        .fillMaxHeight()
                        .background(p.color, RoundedCornerShape(50)),
                )
                Spacer(Modifier.width(2.dp))
            }
        }
    }
    Spacer(Modifier.height(12.dp))
    Column(
        Modifier
            .fillMaxWidth()
            .background(Pa.Card, RoundedCornerShape(16.dp))
            .padding(horizontal = 14.dp, vertical = 10.dp),
    ) {
        parts.forEach { p ->
            val pct = p.chars.toFloat() / charTotal * 100f
            Row(
                Modifier.fillMaxWidth().padding(vertical = 3.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Box(Modifier.size(8.dp).background(p.color, RoundedCornerShape(2.dp)))
                Spacer(Modifier.width(9.dp))
                Text(p.label, Modifier.weight(1f), fontSize = 11.5.sp, color = Pa.TextSecondary)
                Text(
                    String.format(java.util.Locale.US, "%.1f%%", pct),
                    fontSize = 11.sp,
                    color = Pa.TextTertiary,
                )
            }
        }
    }
}

/** 审批策略选项(与 codex approvalPolicy 对齐)。 */
val APPROVALS = listOf(
    Triple("untrusted", "仅信任", "白名单外的命令先批准"),
    Triple("on-request", "按需询问", "运行命令前征求批准"),
    Triple("never", "完全访问", "无需批准直接执行,注意风险"),
)

/** 思考强度选项(id 与 codex turn/start 的 effort 取值一致)。 */
data class EffortOption(val id: String, val label: String)

val EFFORTS = listOf(
    EffortOption("", "默认"),
    EffortOption("low", "低"),
    EffortOption("medium", "中"),
    EffortOption("high", "高"),
    EffortOption("xhigh", "极高"),
)

/**
 * 聊天页(全屏):流式事件实时上屏。
 * 顶栏 ☰ 打开**全局侧边栏**(与主页/会话/文件/Git 完全同款,左边缘右滑亦可唤出)。
 * 输入栏对标图五:第一行输入,第二行 [+ 附件][审批策略][模型·强度][发送]。
 */
@Composable
fun ChatScreen(
    threadId: String,
    onBack: () -> Unit = {},
    onOpenThread: (String) -> Unit = {},
    onOpenSettings: () -> Unit = {},
    onOpenTerminal: () -> Unit = {},
    onOpenTaskCenter: () -> Unit = {},
    onGoTab: (Int) -> Unit = {},
    /** 打开内置浏览器(带地址);传空串表示只打开浏览器首页。 */
    onOpenBrowser: (String) -> Unit = {},
) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    // 【返回键】APP 内返回上一层(会话 → 主页),而不是直接退回桌面
    BackHandler(enabled = true) { onBack() }
    // 轮次状态在 TurnSession 单例:离开页面流不中断,回来接着看
    val st = remember(threadId) { TurnSession.state(threadId) }
    var input by remember { mutableStateOf("") }
    var showModelCard by remember { mutableStateOf(false) }
    var showContextCard by remember { mutableStateOf(false) }
    var showDrawer by remember { mutableStateOf(false) }
    var showNewThread by remember { mutableStateOf(false) }
    var showGitPanel by remember { mutableStateOf(false) }
    var attachments by remember { mutableStateOf(listOf<Attachment>()) }
    var selectedAttachment by remember { mutableStateOf<Attachment?>(null) }
    var threadWorkspace by remember { mutableStateOf<String?>(null) }
    var codexSettings by remember { mutableStateOf<CodexSettings?>(null) }
    var models by remember { mutableStateOf(listOf<ModelInfo>()) }
    var loadError by remember { mutableStateOf<String?>(null) }
    var reloadToken by remember { mutableStateOf(0) }
    // 侧边栏品牌行的电脑名(与其他页面一致)
    var hostName by remember { mutableStateOf("") }
    val listState = rememberLazyListState()
    val connectionEpoch = AgentConnection.connectionEpoch.collectAsState().value

    val uiScale = AgentConnection.uiTextScale
    val thinkScale = AgentConnection.thinkTextScale

    // codex 桌面端设置(审批策略芯片用)+ 模型列表(芯片显示模型名用)
    LaunchedEffect(connectionEpoch, reloadToken) {
        try {
            withContext(Dispatchers.IO) {
                codexSettings = AgentConnection.modelService()
                    .getCodexSettings(GetCodexSettingsRequest.newBuilder().build())
            }
        } catch (e: Exception) {
            loadError = "无法加载模型设置：${e.message ?: "连接失败"}"
        }
        try {
            withContext(Dispatchers.IO) {
                models = AgentConnection.modelService()
                    .listModels(ListModelsRequest.newBuilder().build()).modelsList
            }
        } catch (e: Exception) {
            if (loadError == null) loadError = "无法加载模型列表：${e.message ?: "连接失败"}"
        }
        try {
            withContext(Dispatchers.IO) {
                hostName = AgentConnection.hostService()
                    .getHostInfo(GetHostInfoRequest.getDefaultInstance()).displayName
            }
        } catch (_: Exception) {
            // 电脑名取不到不影响聊天,侧边栏回退到通用文案
        }
    }

    // 【会话级模型】进入会话时恢复该会话上次选定的模型;该会话没选过则用全局默认。
    // 修复:selectedModel 过去是全局单值,在 A 会话切模型会污染 B 会话,
    // 退出再进又被重置成"跟随桌面配置";现在按 threadId 单独记住。
    LaunchedEffect(threadId) {
        AgentConnection.selectedModel =
            AgentConnection.threadModel(threadId) ?: AgentConnection.defaultModel
    }

    // 生效模型:会话选择 → codex 桌面默认 → 本轮实际运行 → 列表首个。
    // 统一后,无论"跟随桌面配置"还是指定模型,上下文窗口都可查看、可点开明细。
    val effectiveModelId = AgentConnection.selectedModel
        .ifEmpty { codexSettings?.model.orEmpty() }
        .ifEmpty { st.runningModel }
    val effectiveModel = models.find { it.id == effectiveModelId }
        ?: models.find { it.id == st.runningModel }
        ?: models.find { it.id == "codex/default" }
        ?: models.firstOrNull()
    // 窗口大小:模型列表值 与 daemon 随用量上报的真实值 取大;
    // 列表里大多数 codex 模型没有窗口数据,靠上报值兜底(切模型后仍可显示)。
    val contextWindow = maxOf((effectiveModel?.contextWindow ?: 0).toLong(), st.liveContextWindow)

    /** 会话内无缝切换审批策略(codex 桌面端同步生效)。 */
    fun setApproval(id: String) {
        val b = (codexSettings ?: CodexSettings.getDefaultInstance()).toBuilder().setApprovalPolicy(id)
        codexSettings = b.build()
        scope.launch {
            try {
                withContext(Dispatchers.IO) {
                    codexSettings = AgentConnection.modelService().setCodexSettings(
                        SetCodexSettingsRequest.newBuilder()
                            .setApprovalPolicy(id)
                            .setSandbox(codexSettings?.sandbox ?: "")
                            .build())
                }
                Toast.makeText(ctx, "审批策略 → ${APPROVALS.find { it.first == id }?.second}", Toast.LENGTH_SHORT).show()
            } catch (e: Exception) {
                Toast.makeText(ctx, "切换失败:${e.message}", Toast.LENGTH_SHORT).show()
            }
        }
    }

    fun openAttachment(attachment: Attachment) {
        if (attachmentKind(attachment.name, attachment.mediaType) == AttachmentKind.IMAGE) {
            selectedAttachment = attachment
            return
        }
        val uri = attachment.localUri?.let(Uri::parse)
        if (uri == null) {
            Toast.makeText(ctx, "文件已上传到电脑：${attachment.pcPath}", Toast.LENGTH_SHORT).show()
            return
        }
        runCatching {
            ctx.startActivity(
                Intent.createChooser(
                    Intent(Intent.ACTION_VIEW)
                        .setDataAndType(uri, attachment.mediaType.ifBlank { "*/*" })
                        .addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION),
                    "打开附件",
                ),
            )
        }.onFailure {
            Toast.makeText(ctx, "没有可打开此文件的应用", Toast.LENGTH_SHORT).show()
        }
    }

    /** 手机选中的文件上传到电脑工作区,聊天里引用 PC 路径。 */
    val pickFiles = rememberLauncherForActivityResult(ActivityResultContracts.GetMultipleContents()) { uris ->
        if (uris.isNotEmpty()) scope.launch {
            try {
                val ws = withContext(Dispatchers.IO) {
                    threadWorkspace
                        ?: AgentConnection.threadService()
                            .getThread(GetThreadRequest.newBuilder().setThreadId(threadId).build())
                            .workspace
                            .ifBlank { null }
                        ?: error("当前会话没有工作区，请先为会话选择工作区")
                }
                threadWorkspace = ws
                val uploaded = withContext(Dispatchers.IO) {
                    uris.mapIndexed { index, uri ->
                        val (name, bytes) = ctx.contentResolver.query(uri, null, null, null, null)?.use { c ->
                            val idx = c.getColumnIndex(OpenableColumns.DISPLAY_NAME)
                            val n = if (c.moveToFirst() && idx >= 0) c.getString(idx) else null
                            val data = ctx.contentResolver.openInputStream(uri)?.use { it.readBytes() }
                                ?: error("读取失败")
                            n.orEmpty().ifBlank { "file" } to data
                        } ?: error("无法读取附件")
                        val safe = name.replace(Regex("[\\\\/:*?\"<>|]"), "_")
                        val pcPath = ws.trimEnd('/', '\\') + "\\.pocketagent-uploads\\" +
                            System.currentTimeMillis() + "_" + index + "_" + safe
                        AgentConnection.fileService().writeFile(
                            WriteFileRequest.newBuilder()
                                .setPath(pcPath)
                                .setData(ByteString.copyFrom(bytes))
                                .setCreate(true).setTruncate(true)
                                .build())
                        attachmentFromBytes(
                            name = safe,
                            pcPath = pcPath,
                            bytes = bytes,
                            mediaType = ctx.contentResolver.getType(uri).orEmpty(),
                            localUri = uri.toString(),
                        )
                    }
                }
                attachments = attachments + uploaded
                Toast.makeText(ctx, "已上传 ${uploaded.size} 个附件", Toast.LENGTH_SHORT).show()
            } catch (e: Exception) {
                Toast.makeText(ctx, "附件上传失败:${e.message}", Toast.LENGTH_SHORT).show()
            }
        }
    }

    LaunchedEffect(st.items.size, st.streamText.length / 24) {
        val total = st.items.size + if (st.streaming) 1 else 0
        if (total > 0) listState.scrollToItem(total - 1)
    }

    var threadTitle by remember { mutableStateOf("任务") }
    LaunchedEffect(threadId, connectionEpoch, reloadToken) {
        try {
            val t = AgentConnection.threadService().getThread(
                GetThreadRequest.newBuilder().setThreadId(threadId).build())
            if (t.title.isNotEmpty()) threadTitle = t.title
            threadWorkspace = t.workspace.ifBlank { null }
        } catch (e: Exception) {
            loadError = "无法加载会话：${e.message ?: "连接失败"}"
        }
        try {
            // 后台轮次进行中(或已有实时列表)时不覆盖
            if (!st.running && !st.recovering &&
                (st.items.isEmpty() || connectionEpoch > 0 || reloadToken > 0)
            ) {
                val resp = AgentConnection.threadService().getThreadHistory(
                    GetThreadHistoryRequest.newBuilder().setThreadId(threadId).build(),
                )
                if (resp.messagesList.isNotEmpty()) {
                    st.items.clear()
                    st.items.addAll(historyToChatItems(resp.messagesList))
                    listState.scrollToItem(st.items.size - 1)
                }
            }
            st.historyLoaded = true
        } catch (e: Exception) {
            loadError = "无法加载会话历史：${e.message ?: "连接失败"}"
        }
    }

    fun send(text: String) {
        if (text.isBlank() || st.running) return
        val sent = attachments
        attachments = emptyList()
        TurnSession.send(threadId, text, sent)
    }

    st.pendingApproval?.let { ap ->
        ApprovalSheet(
            action = ap,
            error = st.approvalError,
            onDecision = { approved ->
                TurnSession.answerApproval(threadId, ap.requestId, approved)
            },
        )
    }

    val connSt = AgentConnection.connState.collectAsState().value
    val hasFailure = loadError != null ||
        st.recovering ||
        st.recoveryError != null ||
        st.errorMessage != null ||
        connSt == "down"

    Column(
        Modifier
            .fillMaxSize()
            .background(Pa.Bg)
            .statusBarsPadding()
            // 与其余页面统一:左边缘右滑也能唤出侧边栏
            .edgeSwipeToOpenDrawer { showDrawer = true },
    ) {
        // ---- 顶栏:☰ 侧边栏(会话列表/搜索)+ 标题 + 状态点 ----
        Row(
            Modifier
                .fillMaxWidth()
                .padding(horizontal = 14.dp, vertical = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            IconActionButton(AppR.drawable.ic_lucide_menu, "菜单", { showDrawer = true }, bg = Pa.Card)
            Spacer(Modifier.width(14.dp))
            Text(threadTitle, Modifier.weight(1f), fontSize = 16.sp, color = Pa.TextPrimary, maxLines = 1)
            Box(
                Modifier
                    .size(14.dp)
                    .pressable(
                        scaleDown = 0.86f,
                        enabled = hasFailure,
                    ) {
                        if (st.recoveryError != null || st.recovering) {
                            TurnSession.retryRecovery(threadId)
                        } else {
                            loadError = null
                            codexSettings = null
                            models = emptyList()
                            reloadToken++
                        }
                    }
                    .padding(3.dp)
                    .background(if (hasFailure) Pa.Red else Pa.Accent, CircleShape),
            )
            Spacer(Modifier.width(14.dp))
        }

        // ---- 消息列表(思考过程 / 工具调用都在时间线原位置展示)----
        LazyColumn(
            Modifier.weight(1f).padding(horizontal = 16.dp),
            state = listState,
            verticalArrangement = Arrangement.spacedBy(14.dp),
            contentPadding = PaddingValues(vertical = 12.dp),
        ) {
            items(st.items) { item ->
                when (item.role) {
                    "user" -> Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.End) {
                        Column(
                            Modifier
                                .widthIn(max = 300.dp)
                                .background(Pa.AccentDim, RoundedCornerShape(18.dp, 6.dp, 18.dp, 18.dp))
                                .padding(horizontal = 12.dp, vertical = 10.dp),
                            verticalArrangement = Arrangement.spacedBy(8.dp),
                        ) {
                            item.attachments.chunked(2).forEach { rowAttachments ->
                                Row(
                                    Modifier.fillMaxWidth(),
                                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                                ) {
                                    rowAttachments.forEach { attachment ->
                                        AttachmentCard(
                                            attachment = attachment,
                                            preview = remember(
                                                attachment.name,
                                                attachment.previewData?.contentHashCode(),
                                            ) {
                                                loadAttachmentPreview(ctx, attachment)
                                            },
                                            modifier = Modifier.weight(1f),
                                            onClick = { openAttachment(attachment) },
                                        )
                                    }
                                    if (rowAttachments.size == 1) {
                                        Spacer(Modifier.weight(1f))
                                    }
                                }
                            }
                            if (item.text.isNotBlank()) {
                                Text(
                                    item.text,
                                    color = Pa.TextPrimary,
                                    fontSize = (15 * uiScale).sp,
                                )
                            }
                        }
                    }
                    // 思考过程:可折叠卡片(字号跟随"思考显示大小"设置);
                    // 受全局"显示思考过程"开关控制,关闭时只看正文与结果
                    "thinking" -> if (AgentConnection.showThinking) {
                        ThinkingCard(item.text, thinkScale, streaming = false)
                    } else Unit
                    // 执行计划:步骤卡(同样受思考过程开关控制)
                    "plan" -> if (AgentConnection.showThinking) {
                        PlanCard(item.planSteps, draft = "", streaming = false)
                    } else Unit
                    // 工具调用:一次调用一条卡,含参数/输出/耗时,可展开看详情
                    "tool" -> ToolCallCard(item, uiScale)
                    // 后台 Codex/MCP 事件只影响右上角状态点，不再把诊断卡片
                    // 连续插入会话正文。
                    "codex_event" -> Unit
                    "usage" -> Unit
                    else -> Column(Modifier.fillMaxWidth()) {
                        InAppMarkdown(
                            text = item.text,
                            modifier = Modifier.fillMaxWidth(),
                            fontSize = 15f * uiScale,
                            onOpenUrl = onOpenBrowser,
                        )
                        // 会话输出可保存到手机(后台下载,任务中心可见)
                        if (item.text.isNotBlank()) {
                            Row(
                                Modifier
                                    .padding(top = 4.dp)
                                    .pressable(haptic = true) {
                                        PaDownloader.saveTextAsFile(
                                            "会话输出-%s-%d.md".format(
                                                threadId.take(6),
                                                System.currentTimeMillis() % 100000,
                                            ),
                                            item.text,
                                            threadId,
                                        )
                                        Toast.makeText(ctx, "已开始保存到手机", Toast.LENGTH_SHORT).show()
                                    },
                                verticalAlignment = Alignment.CenterVertically,
                            ) {
                                Icon(
                                    painterResource(AppR.drawable.ic_lucide_download), null,
                                    tint = Pa.TextTertiary, modifier = Modifier.size(13.dp),
                                )
                                Spacer(Modifier.width(5.dp))
                                Text("保存到手机", fontSize = 10.sp, color = Pa.TextTertiary)
                            }
                        }
                    }
                }
            }
            // 流式思考:边想边展示,和落定的思考卡同款样式
            if (st.running && AgentConnection.showThinking && st.thinkingText.isNotEmpty()) {
                item(key = "streaming-thinking") {
                    ThinkingCard(st.thinkingText, thinkScale, streaming = true)
                }
            }
            // 流式执行计划:结构化步骤或生成中的草稿,边出边看
            if (st.running && AgentConnection.showThinking &&
                (st.planSteps.isNotEmpty() || st.planStream.isNotEmpty())
            ) {
                item(key = "streaming-plan") {
                    PlanCard(st.planSteps, st.planStream, streaming = true)
                }
            }
            if (st.streaming) {
                item(key = "streaming-bubble") {
                    if (st.streamText.isEmpty()) {
                        // 等待首个 token:三点跳动"正在输入"指示(替代旧的光标闪烁)
                        Row(Modifier.fillMaxWidth()) {
                            Box(
                                Modifier
                                    .background(Pa.Card, RoundedCornerShape(18.dp))
                                    .padding(horizontal = 16.dp, vertical = 14.dp),
                            ) {
                                TypingDots()
                            }
                        }
                    } else {
                        // 流式输出中:正文 + 行内小号三点跟随(仍在生成,比光标更自然)
                        Column(Modifier.fillMaxWidth()) {
                            InAppMarkdown(
                                text = st.streamText,
                                modifier = Modifier.fillMaxWidth(),
                                fontSize = 15f * uiScale,
                                onOpenUrl = onOpenBrowser,
                            )
                            TypingDots(
                                Modifier.padding(start = 2.dp, top = 6.dp),
                                dotSize = 4.dp,
                            )
                        }
                    }
                }
            }
        }

        // ---- 输入区(对标图五):附件行 + 双行胶囊 [输入 / + 审批 模型·强度 发送] ----
        Column(
            Modifier
                .fillMaxWidth()
                .background(Pa.Bg)
                .imePadding()
                .padding(start = 12.dp, end = 12.dp, top = 8.dp, bottom = 16.dp),
        ) {
            // 已上传附件 chips
            if (attachments.isNotEmpty()) {
                Row(
                    Modifier.fillMaxWidth().horizontalScroll(rememberScrollState()),
                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                ) {
                    attachments.forEach { at ->
                        Row(
                            Modifier
                                .pressable { openAttachment(at) }
                                .background(Pa.CardHigh, RoundedCornerShape(10.dp))
                                .padding(horizontal = 10.dp, vertical = 7.dp),
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            Icon(
                                painterResource(
                                    if (attachmentKind(at.name, at.mediaType) == AttachmentKind.IMAGE) {
                                        AppR.drawable.ic_lucide_image
                                    } else {
                                        AppR.drawable.ic_lucide_file_text
                                    },
                                ),
                                null,
                                tint = Pa.Accent, modifier = Modifier.size(13.dp),
                            )
                            Spacer(Modifier.width(6.dp))
                            Text(at.name, fontSize = 11.sp, color = Pa.TextPrimary, maxLines = 1)
                            Spacer(Modifier.width(6.dp))
                            Icon(
                                painterResource(AppR.drawable.ic_lucide_x), null,
                                tint = Pa.TextTertiary,
                                modifier = Modifier
                                    .size(13.dp)
                                    .pressable { attachments = attachments - at },
                            )
                        }
                    }
                }
                Spacer(Modifier.height(8.dp))
            }

            var approvalMenu by remember { mutableStateOf(false) }
            Column(
                Modifier
                    .fillMaxWidth()
                    .background(Pa.Card, RoundedCornerShape(26.dp)),
            ) {
                // 第一行:输入框
                BasicTextField(
                    value = input,
                    onValueChange = { input = it },
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(horizontal = 18.dp, vertical = 12.dp)
                        .heightIn(min = 24.dp),
                    textStyle = TextStyle(fontSize = 15.sp, color = Pa.TextPrimary),
                    cursorBrush = SolidColor(Pa.Accent),
                    maxLines = 4,
                    enabled = !st.running,
                    decorationBox = { inner ->
                        Box {
                            if (input.isEmpty()) {
                                Text("随心输入,发给电脑上的 agent…", fontSize = 15.sp, color = Pa.TextTertiary)
                            }
                            inner()
                        }
                    },
                )
                // 第二行:工具行
                Row(
                    Modifier
                        .fillMaxWidth()
                        .padding(start = 10.dp, end = 8.dp, bottom = 8.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    // [+ 附件]
                    Box(
                        Modifier
                            .size(32.dp)
                            .pressable { pickFiles.launch("*/*") },
                        contentAlignment = Alignment.Center,
                    ) {
                        Icon(
                            painterResource(AppR.drawable.ic_lucide_plus), "添加附件",
                            tint = Pa.TextSecondary, modifier = Modifier.size(20.dp),
                        )
                    }
                    Spacer(Modifier.width(2.dp))
                    // [Git] 会话项目的版本管理侧栏
                    Box(
                        Modifier
                            .size(32.dp)
                            .pressable { showGitPanel = true },
                        contentAlignment = Alignment.Center,
                    ) {
                        Icon(
                            painterResource(AppR.drawable.ic_lucide_git_branch), "项目 Git",
                            tint = Pa.TextSecondary, modifier = Modifier.size(18.dp),
                        )
                    }
                    Spacer(Modifier.width(2.dp))
                    // [审批策略] 会话内无缝切换
                    Box {
                        Row(
                            Modifier
                                .pressable { approvalMenu = true }
                                .background(Pa.CardHigh, RoundedCornerShape(50))
                                .padding(horizontal = 10.dp, vertical = 6.dp),
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            val cur = codexSettings?.approvalPolicy ?: "on-request"
                            val danger = cur == "never"
                            Icon(
                                painterResource(AppR.drawable.ic_lucide_triangle_alert), null,
                                tint = if (danger) Pa.Red else Pa.Accent,
                                modifier = Modifier.size(13.dp),
                            )
                            Spacer(Modifier.width(5.dp))
                            Text(
                                APPROVALS.find { it.first == cur }?.second ?: "按需询问",
                                fontSize = 11.5.sp,
                                color = if (danger) Pa.Red else Pa.TextPrimary,
                            )
                        }
                        DropdownMenu(
                            expanded = approvalMenu,
                            onDismissRequest = { approvalMenu = false },
                            containerColor = Pa.CardHigh,
                            shape = RoundedCornerShape(16.dp),
                        ) {
                            APPROVALS.forEach { (id, label, desc) ->
                                val active = (codexSettings?.approvalPolicy ?: "on-request") == id
                                DropdownMenuItem(
                                    text = {
                                        Row(verticalAlignment = Alignment.CenterVertically) {
                                            Column(Modifier.weight(1f)) {
                                                Text(
                                                    label, fontSize = 14.sp,
                                                    color = if (active) Pa.Accent else Pa.TextPrimary,
                                                )
                                                Text(desc, fontSize = 10.5.sp, color = Pa.TextTertiary)
                                            }
                                            if (active) {
                                                Icon(
                                                    painterResource(AppR.drawable.ic_lucide_check), null,
                                                    tint = Pa.Accent, modifier = Modifier.size(16.dp),
                                                )
                                            }
                                        }
                                    },
                                    onClick = {
                                        approvalMenu = false
                                        setApproval(id)
                                    },
                                )
                            }
                        }
                    }
                    // [上下文窗口] 常显(窗口未知也显示,点开可看用量);
                    // 窗口大小 = 模型列表值与 daemon 上报的真实值取大。
                    Row(
                        Modifier
                            .pressable(haptic = true) { showContextCard = true }
                            .background(Pa.CardHigh, RoundedCornerShape(50))
                            .padding(horizontal = 9.dp, vertical = 6.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Icon(
                            painterResource(AppR.drawable.ic_lucide_cpu), null,
                            tint = Pa.Accent, modifier = Modifier.size(12.dp),
                        )
                        Spacer(Modifier.width(4.dp))
                        Text(
                            if (contextWindow > 0) "上下文 ${contextWindow / 1000}k" else "上下文",
                            fontSize = 10.5.sp, color = Pa.TextSecondary, maxLines = 1,
                        )
                    }
                    Spacer(Modifier.width(6.dp))
                    // [模型 · 强度] 直显当前模型(点击弹卡面切换)。
                    // weight(1f) + 省略号:模型名再长也只吃剩余空间,绝不挤压发送键。
                    val modelName = effectiveModel?.displayName
                        ?: if (AgentConnection.selectedModel.isEmpty()) "跟随桌面"
                        else AgentConnection.selectedModel.substringAfter('/')
                    val effortLabel = EFFORTS.find { it.id == AgentConnection.selectedEffort }?.label
                    Row(
                        Modifier
                            .weight(1f)
                            .pressable { showModelCard = true }
                            .padding(horizontal = 4.dp, vertical = 6.dp),
                        horizontalArrangement = Arrangement.End,
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Text(
                            modelName + if (!effortLabel.isNullOrBlank() && effortLabel != "默认") " · $effortLabel" else "",
                            Modifier.weight(1f, fill = false),
                            fontSize = 11.5.sp,
                            color = Pa.TextSecondary,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                        )
                        Spacer(Modifier.width(3.dp))
                        Icon(
                            painterResource(AppR.drawable.ic_lucide_chevron_right), null,
                            tint = Pa.TextTertiary, modifier = Modifier.size(12.dp),
                        )
                    }
                    Spacer(Modifier.width(6.dp))
                    // [发送]
                    val canSend = !st.running && input.isNotBlank()
                    Box(
                        Modifier
                            .size(36.dp)
                            .pressable(scaleDown = 0.88f, enabled = canSend) {
                                send(input)
                                input = ""
                            }
                            .background(if (canSend) Pa.Accent else Pa.Hover, CircleShape),
                        contentAlignment = Alignment.Center,
                    ) {
                        Icon(
                            painterResource(AppR.drawable.ic_lucide_arrow_up), "发送",
                            tint = if (canSend) Color.White else Pa.TextTertiary,
                            modifier = Modifier.size(18.dp),
                        )
                    }
                }
            }
        }
    }

    selectedAttachment?.let { attachment ->
        val preview = loadAttachmentPreview(ctx, attachment)
        Dialog(
            onDismissRequest = { selectedAttachment = null },
            properties = DialogProperties(usePlatformDefaultWidth = false),
        ) {
            Box(
                Modifier
                    .fillMaxSize()
                    .background(Color.Black.copy(alpha = 0.92f))
                    .clickable { selectedAttachment = null },
                contentAlignment = Alignment.Center,
            ) {
                if (preview != null) {
                    androidx.compose.foundation.Image(
                        preview,
                        contentDescription = attachment.name,
                        contentScale = ContentScale.Fit,
                        modifier = Modifier
                            .fillMaxSize()
                            .padding(18.dp),
                    )
                } else {
                    Text("无法预览：${attachment.name}", color = Color.White)
                }
            }
        }
    }

    // ---- 侧边栏:与主页/会话/文件/Git 完全同款(全局导航),不再维护第二套设计 ----
    if (showDrawer) {
        AppDrawer(
            currentTab = 1,
            hostTitle = hostName.ifBlank { "你电脑上的 agent" },
            onClose = { showDrawer = false },
            onGoTab = {
                showDrawer = false
                onGoTab(it)
            },
            onOpenTerminal = {
                showDrawer = false
                onOpenTerminal()
            },
            onOpenSettings = {
                showDrawer = false
                onOpenSettings()
            },
            onOpenTaskCenter = {
                showDrawer = false
                onOpenTaskCenter()
            },
            onNewThread = {
                showDrawer = false
                showNewThread = true
            },
            onOpenBrowser = {
                showDrawer = false
                onOpenBrowser("")
            },
        )
    }

    // 新建对话(与所有入口同款:文件式选工作区)
    if (showNewThread) {
        NewThreadSheet(
            onDismiss = { showNewThread = false },
            onCreate = { ws ->
                showNewThread = false
                scope.launch {
                    try {
                        val builder = CreateThreadRequest.newBuilder()
                        if (AgentConnection.selectedModel.isNotEmpty()) {
                            builder.setModelId(AgentConnection.selectedModel)
                        }
                        if (ws.isNotBlank()) {
                            builder.setWorkspace(ws)
                        }
                        val t = withContext(Dispatchers.IO) {
                            AgentConnection.threadService().createThread(builder.build())
                        }
                        onOpenThread(t.id)
                    } catch (e: Exception) {
                        Toast.makeText(ctx, "创建失败:${e.message}", Toast.LENGTH_SHORT).show()
                    }
                }
            },
        )
    }

    // ---- 右侧 Git 侧栏:该会话所在项目的版本管理 ----
    if (showGitPanel) {
        ThreadGitPanel(
            repoPath = threadWorkspace,
            onClose = { showGitPanel = false },
        )
    }

    if (showModelCard) {
        ModelEffortCard(threadId = threadId, onDismiss = { showModelCard = false })
    }

    // 上下文窗口明细:点输入栏"上下文"胶囊打开
    if (showContextCard) {
        ContextWindowCard(
            model = effectiveModel,
            contextWindow = contextWindow,
            turnUsage = st.turnUsage,
            totalUsage = st.totalUsage,
            items = st.items.toList(),
            onDismiss = { showContextCard = false },
        )
    }
}

/**
 * 模型/思考强度卡面弹窗 —— "弹窗的卡面"而非长抽屉:
 * 居中卡片 + 限高可滚模型列表 + 思考强度胶囊 + 性能提示语。
 */
@Composable
fun ModelEffortCard(threadId: String = "", onDismiss: () -> Unit) {
    Dialog(onDismissRequest = onDismiss) {
        Column(
            Modifier
                .fillMaxWidth()
                .background(Pa.CardHigh, RoundedCornerShape(22.dp))
                .padding(20.dp),
        ) {
            Row(
                Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text("模型与思考强度", fontSize = 17.sp, color = Pa.TextPrimary)
                IconActionButton(
                    AppR.drawable.ic_lucide_x, "关闭", onDismiss,
                    bg = Color.Transparent, size = 36.dp, iconSize = 18.dp,
                )
            }
            Spacer(Modifier.height(10.dp))

            val models = remember { mutableStateOf(listOf<ModelInfo>()) }
            LaunchedEffect(Unit) {
                models.value = try {
                    AgentConnection.modelService()
                        .listModels(ListModelsRequest.newBuilder().build())
                        .modelsList
                } catch (e: Exception) {
                    emptyList()
                }
            }

            /**
             * 选定模型:会话内选择只改"本会话"(空串="跟随桌面配置",也是有效选择),
             * 不污染全局默认,也不影响其他会话。
             */
            fun pick(id: String) {
                AgentConnection.selectedModel = id
                if (threadId.isNotBlank()) {
                    AgentConnection.setThreadModel(threadId, id)
                } else {
                    AgentConnection.setGlobalModel(id)
                }
            }

            Column(
                Modifier
                    .fillMaxWidth()
                    .height(300.dp)
                    .verticalScroll(rememberScrollState()),
            ) {
                // 显式"跟随桌面配置"选项(空模型 id)
                val followSelected = AgentConnection.selectedModel.isEmpty()
                Row(
                    Modifier
                        .fillMaxWidth()
                        .pressable { pick("") }
                        .padding(horizontal = 4.dp, vertical = 11.dp),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Column(Modifier.weight(1f)) {
                        Text(
                            "跟随桌面配置",
                            fontSize = 15.sp,
                            color = if (followSelected) Pa.Accent else Pa.TextPrimary,
                            maxLines = 1,
                        )
                        Text("使用电脑上 codex 当前的默认模型", fontSize = 10.5.sp, color = Pa.TextTertiary)
                    }
                    if (followSelected) {
                        Icon(
                            painterResource(AppR.drawable.ic_lucide_check), null,
                            tint = Pa.Accent, modifier = Modifier.size(18.dp),
                        )
                    }
                }
                if (models.value.isEmpty()) {
                    Text("加载中…", fontSize = 13.sp, color = Pa.TextTertiary, modifier = Modifier.padding(8.dp))
                }
                models.value.forEach { m ->
                    val selected = m.id == AgentConnection.selectedModel
                    Row(
                        Modifier
                            .fillMaxWidth()
                            .pressable { pick(m.id) }
                            .padding(horizontal = 4.dp, vertical = 11.dp),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Column(Modifier.weight(1f)) {
                            Text(
                                m.displayName,
                                fontSize = 15.sp,
                                color = if (selected) Pa.Accent else Pa.TextPrimary,
                                maxLines = 1,
                                overflow = TextOverflow.Ellipsis,
                            )
                            if (m.contextWindow > 0) {
                                Text(
                                    "上下文 ${m.contextWindow / 1000}k" +
                                        if (m.supportsThinking) " · 支持思考" else "",
                                    fontSize = 10.5.sp,
                                    color = Pa.TextTertiary,
                                )
                            }
                        }
                        if (selected) {
                            Icon(
                                painterResource(AppR.drawable.ic_lucide_check), null,
                                tint = Pa.Accent, modifier = Modifier.size(18.dp),
                            )
                        }
                    }
                }
            }

            Spacer(Modifier.height(12.dp))
            Box(Modifier.fillMaxWidth().height(1.dp).background(Pa.Separator))
            Spacer(Modifier.height(12.dp))
            Text("思考强度", fontSize = 14.sp, color = Pa.TextPrimary)
            Spacer(Modifier.height(10.dp))
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                EFFORTS.forEach { e ->
                    val selected = AgentConnection.selectedEffort == e.id
                    Box(
                        Modifier
                            .pressable {
                                AgentConnection.selectedEffort = e.id
                                AgentConnection.persistEffort()
                            }
                            .background(
                                if (selected) Pa.Accent else Pa.Card,
                                RoundedCornerShape(50),
                            )
                            .padding(horizontal = 14.dp, vertical = 8.dp),
                    ) {
                        Text(
                            e.label,
                            fontSize = 13.sp,
                            color = if (selected) Color.White else Pa.TextSecondary,
                        )
                    }
                }
            }
            Spacer(Modifier.height(12.dp))
            Text(
                "切换模型或思考强度可能影响会话的后续性能",
                fontSize = 11.sp,
                color = Pa.TextTertiary,
            )
        }
    }
}

/** 审批抽屉:命令卡片 + 拒绝(红)/允许(蓝)双按钮。 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun ApprovalSheet(
    action: ApprovalRequest,
    error: String?,
    onDecision: (Boolean) -> Unit,
) {
    val state = rememberModalBottomSheetState(
        skipPartiallyExpanded = true,
        confirmValueChange = { it != androidx.compose.material3.SheetValue.Hidden },
    )
    var expanded by remember(action.requestId) { mutableStateOf(false) }
    ModalBottomSheet(
        onDismissRequest = { },
        sheetState = state,
        properties = ModalBottomSheetProperties(
            shouldDismissOnBackPress = false,
        ),
        containerColor = Pa.CardHigh,
        shape = RoundedCornerShape(topStart = 26.dp, topEnd = 26.dp),
        dragHandle = {
            Box(
                Modifier.padding(top = 10.dp).width(40.dp).height(4.dp)
                    .background(Pa.Hover, CircleShape),
            )
        },
    ) {
        Row(
            Modifier.fillMaxWidth().padding(horizontal = 24.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text("允许执行?", Modifier.weight(1f), fontSize = 19.sp, color = Pa.TextPrimary)
            Text(
                if (expanded) "收起" else "展开",
                fontSize = 11.sp,
                color = Pa.Accent,
            )
            IconActionButton(
                if (expanded) AppR.drawable.ic_lucide_chevron_left else AppR.drawable.ic_lucide_chevron_right,
                if (expanded) "隐藏命令" else "展开命令",
                { expanded = !expanded },
                bg = Color.Transparent,
                size = 36.dp,
                iconSize = 18.dp,
            )
        }
        Text(
            "agent 请求执行以下" + if (action.action.name == "write_file") "写入" else "命令",
            Modifier
                .fillMaxWidth()
                .padding(horizontal = 24.dp, vertical = 4.dp),
            fontSize = 13.sp,
            color = Pa.TextSecondary,
        )
        error?.let {
            Text(
                it,
                Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 24.dp, vertical = 4.dp),
                fontSize = 12.sp,
                color = Pa.Red,
            )
        }
        if (expanded) {
            Text(
                action.action.jsonArgs,
                Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 24.dp, vertical = 12.dp)
                    .background(Pa.Bg, RoundedCornerShape(14.dp))
                    .padding(14.dp),
                fontSize = 12.sp,
                fontFamily = FontFamily.Monospace,
                color = Pa.TextPrimary,
            )
        }
        Row(
            Modifier
                .fillMaxWidth()
                .padding(horizontal = 24.dp)
                .padding(bottom = 30.dp),
        ) {
            Box(
                Modifier
                    .weight(1f)
                    .pressable { onDecision(false) }
                    .background(Pa.Card, RoundedCornerShape(16.dp))
                    .padding(vertical = 14.dp),
                contentAlignment = Alignment.Center,
            ) {
                Text("拒绝", fontSize = 16.sp, color = Pa.Red)
            }
            Spacer(Modifier.width(12.dp))
            Box(
                Modifier
                    .weight(1f)
                    .pressable { onDecision(true) }
                    .background(Pa.Accent, RoundedCornerShape(16.dp))
                    .padding(vertical = 14.dp),
                contentAlignment = Alignment.Center,
            ) {
                Text("允许", fontSize = 16.sp, color = Color.White)
            }
        }
    }
}

/** 相对时间:几分钟前/几小时前/昨天/M月d日。 */
internal fun relativeTime(unixMs: Long): String {
    if (unixMs <= 0) return ""
    val diff = System.currentTimeMillis() - unixMs
    val min = diff / 60000
    return when {
        min < 1 -> "刚刚"
        min < 60 -> "${min}分钟前"
        min < 60 * 24 -> "${min / 60}小时前"
        min < 60 * 48 -> "昨天"
        else -> {
            val d = java.text.SimpleDateFormat("M月d日", java.util.Locale.CHINA)
            d.format(java.util.Date(unixMs))
        }
    }
}


/**
 * 会话页右侧 Git 侧栏:该会话所在项目(daemon 工作区)的版本管理 ——
 * 分支/变更列表/diff 查看/一键提交。从输入栏的 [Git] 图标展开。
 */
@Composable
fun ThreadGitPanel(repoPath: String?, onClose: () -> Unit) {
    // 返回键先关 Git 侧栏
    BackHandler(enabled = true) { onClose() }
    val scope = rememberCoroutineScope()
    var repo by remember { mutableStateOf(repoPath.orEmpty()) }
    var status by remember { mutableStateOf<pocketagent.v1.GitStatus?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    var notRepo by remember { mutableStateOf(false) }
    var diff by remember { mutableStateOf<String?>(null) }
    var showCommit by remember { mutableStateOf(false) }
    var commitMsg by remember { mutableStateOf("") }
    var busy by remember { mutableStateOf(false) }

    fun refresh() {
        if (repo.isEmpty()) return
        busy = true
        scope.launch {
            try {
                status = withContext(Dispatchers.IO) {
                    AgentConnection.gitService()
                        .status(GitStatusRequest.newBuilder().setRepo(repo).build())
                }
                error = null; notRepo = false
            } catch (e: Exception) {
                val lower = "${e.message}".lowercase()
                if (lower.contains("not a git repository") ||
                    lower.contains("not a git repo") ||
                    lower.contains("不是 git")
                ) {
                    notRepo = true
                } else {
                    error = "${e.message}"
                }
            } finally { busy = false }
        }
    }

    LaunchedEffect(repoPath) {
        repo = repoPath.orEmpty()
        if (repo.isBlank()) {
            // 会话没绑工作区时回退到 daemon 的默认工作区,尽量让 Git 栏有内容
            repo = withContext(Dispatchers.IO) {
                runCatching {
                    AgentConnection.connectWithFallback()
                        ?.workspacesList?.firstOrNull()
                        ?: AgentConnection.hostService()
                            .getHostInfo(GetHostInfoRequest.getDefaultInstance())
                            .workspacesList.firstOrNull()
                }.getOrNull().orEmpty()
            }
        }
        if (repo.isNotBlank()) {
            refresh()
        }
    }

    val visibleState = remember { androidx.compose.animation.core.MutableTransitionState(false) }
    visibleState.targetState = true

    Box(
        Modifier
            .fillMaxSize()
            .background(Color.Black.copy(alpha = 0.45f))
            .pressable { onClose() },
    ) {
        // 从左侧滑入:方向跟随触发按钮(输入栏的 Git 图标)所在的左侧,
        // 与会话侧边栏(☰,同样在左)保持一致的出入场方向。
        AnimatedVisibility(
            visible = visibleState.targetState,
            enter = slideInHorizontally(tween(260)) { -it / 2 },
            exit = slideOutHorizontally(tween(200)) { -it / 2 },
            modifier = Modifier
                .align(Alignment.CenterStart)
                .fillMaxHeight(),
        ) {
            Column(
                Modifier
                    .fillMaxWidth(0.86f)
                    .widthIn(max = 310.dp)
                    .fillMaxHeight()
                    .background(Pa.CardHigh, RoundedCornerShape(topEnd = 22.dp, bottomEnd = 22.dp))
                    .statusBarsPadding()
                    .navigationBarsPadding()
                    .padding(horizontal = 16.dp),
            ) {
                Spacer(Modifier.height(14.dp))
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Icon(
                        painterResource(AppR.drawable.ic_lucide_git_branch), null,
                        tint = Pa.Accent, modifier = Modifier.size(18.dp),
                    )
                    Spacer(Modifier.width(8.dp))
                    Column(Modifier.weight(1f)) {
                        Text("Git", fontSize = 16.sp, color = Pa.TextPrimary, fontWeight = FontWeight.SemiBold)
                        Text(
                            status?.branch?.takeIf { it.isNotEmpty() }?.let { "分支 $it · $repo" } ?: repo,
                            fontSize = 10.sp, color = Pa.TextTertiary, maxLines = 1,
                        )
                    }
                    IconActionButton(
                        AppR.drawable.ic_lucide_x, "关闭", onClose,
                        bg = Color.Transparent, size = 36.dp, iconSize = 17.dp,
                    )
                }
                Spacer(Modifier.height(10.dp))

                when {
                    repo.isBlank() -> Box(
                        Modifier.weight(1f).fillMaxWidth(),
                        contentAlignment = Alignment.Center,
                    ) {
                        Column(horizontalAlignment = Alignment.CenterHorizontally) {
                            Text("当前会话未绑定工作区", fontSize = 14.sp, color = Pa.TextPrimary)
                            Text(
                                "创建会话时选择工作文件夹后，Git 会跟随该会话",
                                fontSize = 11.sp,
                                color = Pa.TextTertiary,
                                modifier = Modifier.padding(top = 4.dp),
                            )
                        }
                    }
                    notRepo -> Box(
                        Modifier.weight(1f).fillMaxWidth(),
                        contentAlignment = Alignment.Center,
                    ) {
                        Column(horizontalAlignment = Alignment.CenterHorizontally) {
                            Text("该项目还不是 Git 仓库", fontSize = 14.sp, color = Pa.TextPrimary)
                            Text(
                                "在电脑上 git init 后,这里可以看状态、diff 和提交",
                                fontSize = 11.sp,
                                color = Pa.TextTertiary,
                                modifier = Modifier.padding(top = 4.dp),
                            )
                        }
                    }
                    error != null -> Box(
                        Modifier.weight(1f).fillMaxWidth(),
                        contentAlignment = Alignment.Center,
                    ) {
                        Text(error!!, color = Pa.Red, fontSize = 12.sp)
                    }
                    status != null -> {
                        val st = status!!
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            Text("${st.changesList.size} 个变更", fontSize = 12.sp, color = Pa.TextSecondary)
                            Spacer(Modifier.weight(1f))
                            if (busy) {
                                CircularProgressIndicator(Modifier.size(14.dp), strokeWidth = 2.dp, color = Pa.Accent)
                                Spacer(Modifier.width(8.dp))
                            }
                            IconActionButton(AppR.drawable.ic_lucide_refresh_cw, "刷新", { refresh() }, bg = Pa.Card, size = 34.dp, iconSize = 15.dp)
                        }
                        Spacer(Modifier.height(8.dp))
                        if (st.changesList.isEmpty()) {
                            Text("工作区干净", Modifier.padding(vertical = 20.dp), fontSize = 13.sp, color = Pa.TextSecondary)
                        }
                        LazyColumn(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(6.dp)) {
                            items(st.changesList, key = { it.path }) { c ->
                                Column(
                                    Modifier
                                        .fillMaxWidth()
                                        .background(Pa.Card, RoundedCornerShape(12.dp))
                                        .pressable {
                                            scope.launch {
                                                try {
                                                    diff = withContext(Dispatchers.IO) {
                                                        AgentConnection.gitService()
                                                            .diff(GitDiffRequest.newBuilder().setRepo(repo).build())
                                                            .unifiedDiff
                                                    }
                                                } catch (e: Exception) { error = "diff 失败:${e.message}" }
                                            }
                                        }
                                        .padding(horizontal = 12.dp, vertical = 10.dp),
                                ) {
                                    Text(c.path, fontSize = 12.sp, fontFamily = FontFamily.Monospace, color = Pa.TextPrimary, maxLines = 1)
                                    Text(
                                        "暂存:${c.stagedStatus.ifEmpty { "-" }} · 工作区:${c.worktreeStatus.ifEmpty { "-" }}",
                                        fontSize = 9.5.sp, color = Pa.TextTertiary,
                                    )
                                }
                            }
                        }
                        if (st.changesList.isNotEmpty()) {
                            Box(
                                Modifier
                                    .fillMaxWidth()
                                    .pressable(enabled = !busy) { showCommit = true }
                                    .background(Pa.Accent, RoundedCornerShape(14.dp))
                                    .padding(vertical = 12.dp),
                                contentAlignment = Alignment.Center,
                            ) {
                                Text("提交全部变更", fontSize = 14.sp, color = Color.White, fontWeight = FontWeight.SemiBold)
                            }
                        }
                    }
                    else -> Box(
                        Modifier.weight(1f).fillMaxWidth(),
                        contentAlignment = Alignment.Center,
                    ) {
                        Text("读取中…", fontSize = 12.sp, color = Pa.TextTertiary)
                    }
                }
                Spacer(Modifier.height(10.dp))
            }
        }
    }

    // diff 弹窗
    diff?.let { d ->
        androidx.compose.material3.AlertDialog(
            onDismissRequest = { diff = null },
            title = { Text("变更 (git diff)", fontSize = 14.sp) },
            text = {
                Text(
                    d.take(30000).ifEmpty { "(无未暂存变更)" },
                    fontSize = 10.sp, fontFamily = FontFamily.Monospace,
                    modifier = Modifier.height(420.dp),
                )
            },
            confirmButton = { androidx.compose.material3.TextButton(onClick = { diff = null }) { Text("关闭") } },
        )
    }

    // 提交弹窗
    if (showCommit) {
        androidx.compose.material3.AlertDialog(
            onDismissRequest = { showCommit = false },
            title = { Text("提交全部变更") },
            text = {
                androidx.compose.material3.OutlinedTextField(
                    commitMsg, { commitMsg = it },
                    label = { Text("提交信息") },
                    modifier = Modifier.fillMaxWidth(), minLines = 2,
                )
            },
            confirmButton = {
                androidx.compose.material3.TextButton(
                    enabled = commitMsg.isNotBlank() && !busy,
                    onClick = {
                        busy = true
                        scope.launch {
                            try {
                                withContext(Dispatchers.IO) {
                                    AgentConnection.gitService().commit(
                                        GitCommitRequest.newBuilder()
                                            .setRepo(repo).setMessage(commitMsg.trim()).build())
                                }
                                showCommit = false; commitMsg = ""
                                refresh()
                            } catch (e: Exception) { error = "提交失败:${e.message}" } finally { busy = false }
                        }
                    },
                ) { Text("提交") }
            },
            dismissButton = { androidx.compose.material3.TextButton(onClick = { showCommit = false }) { Text("取消") } },
        )
    }
}

/**
 * MarkdownText 的内置浏览器版:把 Compose 的 UriHandler 换成"打开内置浏览器"。
 * Compose 1.7 的 LinkAnnotation 默认走 LocalUriHandler(系统浏览器),这里接管掉。
 */
@Composable
private fun InAppMarkdown(
    text: String,
    modifier: Modifier = Modifier,
    fontSize: Float = 15f,
    onOpenUrl: (String) -> Unit,
) {
    val handler = remember(onOpenUrl) {
        object : androidx.compose.ui.platform.UriHandler {
            override fun openUri(uri: String) {
                onOpenUrl(uri)
            }
        }
    }
    androidx.compose.runtime.CompositionLocalProvider(
        androidx.compose.ui.platform.LocalUriHandler provides handler,
    ) {
        MarkdownText(text, modifier, fontSize)
    }
}
