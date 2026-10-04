package dev.pocketagent.app.ui

import android.Manifest
import android.content.pm.PackageManager
import android.graphics.BitmapFactory
import android.content.Context
import android.content.Intent
import android.net.Uri
import android.provider.OpenableColumns
import android.speech.SpeechRecognizer
import android.util.Log
import android.widget.Toast
import androidx.activity.compose.BackHandler
import androidx.activity.ComponentActivity
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.animateContentSize
import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.animation.expandVertically
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.shrinkVertically
import androidx.compose.animation.slideInHorizontally
import androidx.compose.animation.slideInVertically
import androidx.compose.animation.slideOutHorizontally
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.border
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsFocusedAsState
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.requiredSize
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.runtime.remember
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.ime
import androidx.compose.foundation.layout.navigationBars
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.safeDrawing
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.sizeIn
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListState
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.Button
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.SideEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.runtime.snapshotFlow
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.graphics.lerp
import androidx.compose.ui.hapticfeedback.HapticFeedbackType
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalHapticFeedback
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties
import androidx.compose.ui.window.Popup
import androidx.compose.ui.window.PopupProperties
import com.google.protobuf.ByteString
import androidx.compose.runtime.collectAsState
import dev.pocketagent.app.AgentConnection
import dev.pocketagent.app.R as AppR
import dev.pocketagent.app.ui.anim.IconActionButton
import dev.pocketagent.app.ui.anim.PaMotion
import dev.pocketagent.app.ui.anim.edgeSwipeToOpenDrawer
import dev.pocketagent.app.ui.anim.pressable
import dev.pocketagent.app.ui.anim.pressableLong
import dev.pocketagent.app.ui.anim.rememberReduceMotion
import dev.pocketagent.app.ui.prompt.GrpcPromptOptimizeRepository
import dev.pocketagent.app.ui.prompt.PromptOptimizePreviewIntent
import dev.pocketagent.app.ui.prompt.PromptOptimizePreviewSheet
import dev.pocketagent.app.ui.prompt.PromptOptimizePreviewState
import dev.pocketagent.app.ui.prompt.isIdle
import dev.pocketagent.app.ui.prompt.promptOptimizeClickAllowed
import dev.pocketagent.app.ui.prompt.reducePromptOptimizePreview
// 【P3-T4】模型/强度选择拆分件与仓库(选中态唯一来源;页面只传 state/intent)
import dev.pocketagent.app.ui.model.ModelEffortCard
import dev.pocketagent.app.ui.model.ModelSelectionIntent
import dev.pocketagent.app.ui.model.ModelSelectionReducer
import dev.pocketagent.app.ui.model.ModelSelectionRepository
import androidx.core.content.ContextCompat
import dev.pocketagent.app.ui.component.PaNotifications
import dev.pocketagent.app.ui.component.PaDownloader
import dev.pocketagent.app.ui.component.PaSheet
import dev.pocketagent.app.ui.component.PaHaptics
import dev.pocketagent.app.ui.component.ConnectionBanner
import androidx.compose.ui.platform.LocalView
import dev.pocketagent.app.ui.component.DeliveryStatusBadge
import dev.pocketagent.app.ui.component.deliveryRetryable
import dev.pocketagent.app.ui.component.deliveryVisual
import dev.pocketagent.app.ui.component.TerminalFont
import dev.pocketagent.app.ui.component.TerminalBlockKind
import dev.pocketagent.app.ui.component.TerminalFootnote
import dev.pocketagent.app.ui.component.TerminalOutputBlock
import dev.pocketagent.app.ui.component.TerminalToolBlock
import dev.pocketagent.app.ui.component.TerminalUserBlock
import dev.pocketagent.app.ui.component.TmuxStatusLine
import dev.pocketagent.app.ui.component.composerShowsPrompt
import dev.pocketagent.app.ui.component.formatTurnFootnote
import dev.pocketagent.app.ui.component.terminalBlockKind
import dev.pocketagent.app.ui.component.terminalScanlines
import dev.pocketagent.app.ui.component.TypingDots
import dev.pocketagent.app.ui.component.UsageCache
import dev.pocketagent.app.ui.component.UsageSnapshot
import dev.pocketagent.app.ui.component.VOICE_ERROR_ENGINE
import dev.pocketagent.app.ui.component.VOICE_ERROR_ENGINE_BUSY
import dev.pocketagent.app.ui.component.VOICE_ERROR_MIC_BLOCKED
import dev.pocketagent.app.ui.component.UndoBarHost
import dev.pocketagent.app.ui.component.SenseVoiceModelManager
// 【P4 Task 4/5/6】语音域层六件套(状态机/控制器/引擎适配):ChatScreen 只做接线,不再自持状态机。
// VoiceErrorKind 取别名:本文件同包仍保留旧七类 VoiceErrorKind(SystemVoiceEngine 的单一真值源),
// 裸名指旧枚举,域层新枚举一律用 DomainVoiceErrorKind,避免同名歧义。
import dev.pocketagent.app.ui.voice.SenseVoiceEngine
import dev.pocketagent.app.ui.voice.SystemVoiceEngine
import dev.pocketagent.app.ui.voice.VoiceController
import dev.pocketagent.app.ui.voice.VoiceEngineKind
import dev.pocketagent.app.ui.voice.VoiceErrorKind as DomainVoiceErrorKind
import dev.pocketagent.app.ui.voice.VoiceEvent
import dev.pocketagent.app.ui.voice.VoicePhase
import dev.pocketagent.app.ui.component.keyboardAwareBottomPadding
import dev.pocketagent.app.ui.component.rememberDrawerController
import dev.pocketagent.app.ui.component.rememberUndoState
// pickVoiceEngine / voiceErrorText(新旧两套)/ joinVoiceText 等语音纯逻辑定义在本文件尾部
// (旧七类映射是 ui/voice/SystemVoiceEngine 的单一真值源,JVM 单测共用)
import dev.pocketagent.app.ui.theme.Pa
import dev.pocketagent.app.ui.theme.PaEditorial
import dev.pocketagent.app.ui.theme.PaHacker
import dev.pocketagent.app.ui.theme.paGlass
import dev.pocketagent.app.ui.theme.paPushBack
// 编辑式基座共享组件(第二波迁移;基座文件禁止修改,只消费)
import dev.pocketagent.app.ui.component.PaApprovalDialog
import dev.pocketagent.app.ui.component.PaHostChip
// 历史消息快速查看(2026-09-30,视频样板交互):左缘刻度条 + 左滑摘要卡浮层
import dev.pocketagent.app.ui.component.HistoryDragState
import dev.pocketagent.app.ui.component.HistoryQuickBubble
import dev.pocketagent.app.ui.component.HistoryQuickNavLogic
import dev.pocketagent.app.ui.component.HistoryQuickPanel
import dev.pocketagent.app.ui.component.HistoryTickBar
// 【P3 Task 8】页面级长期状态分层:ChatScreen 只 collect ChatViewModel 的投影,
// 写入经 ChatIntent 意图(状态树/纯 reducer 在 ui/chat/ChatState.kt)。
import dev.pocketagent.app.ui.chat.ChatIntent
import dev.pocketagent.app.ui.chat.ChatViewModel
import dev.pocketagent.app.ui.history.HistoryLocatePlan
import dev.pocketagent.app.ui.history.HistorySearchHit
import dev.pocketagent.app.ui.history.HistorySearchRepository
import dev.pocketagent.app.ui.history.HistorySearchState
import dev.pocketagent.app.ui.history.MessageSpan
import dev.pocketagent.app.ui.component.PaHostLink
import dev.pocketagent.app.ui.component.PaNavIconButton
import dev.pocketagent.app.ui.component.PaScreenScaffold
import dev.pocketagent.app.ui.component.PaSkeletonList
import dev.pocketagent.app.ui.component.SkeletonKind
import dev.pocketagent.app.ui.component.PaStatusBadge
import dev.pocketagent.app.ui.component.connectionBannerCopy
import dev.pocketagent.app.ui.component.ConnectionBannerState
import dev.pocketagent.app.ui.component.nextConnectionBannerState
// R2-A 消息长按上下文菜单(皮在 component/ContextMenu,只读消费):
// 长按任意用户/回复气泡锚定弹出,动作=复制全文/选择文本/引用回复/重试发送。
import dev.pocketagent.app.ui.component.ContextMenu
import dev.pocketagent.app.ui.component.ContextMenuItem
import dev.pocketagent.app.ui.component.longPressMenu
import dev.pocketagent.app.ui.component.rememberContextMenuState
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.LifecycleEventObserver
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import org.json.JSONArray
import org.json.JSONObject
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicBoolean
import pocketagent.v1.ApprovalRequest
import pocketagent.v1.CodexEvent
import pocketagent.v1.CreateThreadRequest
import pocketagent.v1.CodexSettings
import pocketagent.v1.GetCodexSettingsRequest
import pocketagent.v1.GetHostInfoRequest
import pocketagent.v1.GitStatusRequest
import pocketagent.v1.GitDiffRequest
import pocketagent.v1.GitCommitRequest
import pocketagent.v1.GetThreadHistoryRequest
import pocketagent.v1.SearchThreadMessagesRequest
import pocketagent.v1.GetThreadRequest
import pocketagent.v1.ForkThreadRequest
import pocketagent.v1.RollbackThreadRequest
import pocketagent.v1.CompactThreadRequest
import pocketagent.v1.ClearQueuedInputsRequest
import pocketagent.v1.StartReviewRequest
import pocketagent.v1.SetThreadGoalRequest
import pocketagent.v1.ExportThreadHtmlRequest
import pocketagent.v1.ListModelsRequest
import pocketagent.v1.ListSlashCommandsRequest
import pocketagent.v1.ListThreadsRequest
import pocketagent.v1.ModelInfo
import pocketagent.v1.SetCodexSettingsRequest
import pocketagent.v1.ApproveGuardianDeniedActionRequest
import pocketagent.v1.Thread as PThread
import pocketagent.v1.ThreadMessage
import pocketagent.v1.ThreadStatus
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
    /**
     * P1-4 运行中发消息的投递状态(仅 user 消息会带):
     * pending(发送中)/ steered(已插入)/ queued(排队中)/ overflow(队列满未送达)。
     * 空串 = 普通用户消息。状态机见 TurnDelivery.kt。
     */
    val deliverState: String = "",
    /** 排队位次(Queued.position,1 起;仅 deliverState=queued 时有值)。 */
    val queuePos: Int = 0,
    /**
     * 消息的墙钟时间(用户消息=发出时刻,assistant=创建时刻),毫秒 epoch;
     * **0 = 未知**(旧快照/上游未给)—— 消费方(历史导航气泡/面板按日分组)
     * 见 0 必须隐藏时间行,绝不编造。数据链:实时发送(System.currentTimeMillis)/
     * daemon 历史(ThreadMessage.created_at_unix_ms)/离线快照(Msg.savedAtMs)。
     */
    val sentAtMs: Long = 0,
)

/**
 * 【PF4 组合热点】时间线 item 的 LazyColumn 复用桶([androidx.compose.foundation.lazy.LazyColumn]
 * `contentType`):同桶 item 才能互相复用组合槽。桶必须等价于"渲染形态":
 *  - 常规主题:`item.role` 一对一映射 `when (item.role)` 分支(3778 起),同 role 同结构;
 *  - 黑客终端:渲染分支走 [terminalBlockKind],桶与其对齐(plan/usage 同为 SKIP 等)。
 * 不补桶时,滚动回收会让"用户气泡滚出、工具卡滚进"互相销毁重建整棵子树 —— 长会话
 * 滚动/流式追加期最大的组合浪费。纯函数:桶映射可 JVM 单测(ChatItemContentTypeTest)。
 */
internal fun chatItemContentType(hackerMode: Boolean, item: ChatItem): Any =
    if (hackerMode) terminalBlockKind(item) else item.role

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
private fun CodexEventCard(
    event: CodexEvent,
    uiScale: Float = 1f,
    guardianOverridden: Boolean = false,
    onApproveGuardian: ((CodexEvent) -> Unit)? = null,
    onOpenSettings: (() -> Unit)? = null,
) {
    // 【为什么按 kind 上色而不是"非 ERROR 一律 Accent"】这些卡片的用途完全不同:
    // 红色 = 出事了(审查驳回/任务错误);琥珀 = 需要留意但没坏(配置/沙箱/弃用);
    // 中性蓝 = 只是告知(技能变化、排队)。全都一个颜色等于没有优先级,
    // 用户会习惯性略过 —— 那么把这些卡片搬进时间线就白搬了。
    // 编辑式语义档:红=Denied、琥珀=Attention、蓝=Primary;卡面统一瓷白+细线
    // (黑客终端保持旧色盘,显示模式行为不变)。
    val (accent, bg) = when {
        event.kind == CodexEvent.Kind.ERROR -> PaEditorial.Denied to PaEditorial.Surface
        event.kind == CodexEvent.Kind.GUARDIAN_REVIEW && event.reviewStatus == "denied" -> PaEditorial.Denied to PaEditorial.Surface
        event.kind == CodexEvent.Kind.GUARDIAN_REVIEW &&
            (event.reviewStatus == "timedOut" || event.reviewStatus == "aborted") -> PaEditorial.Attention to PaEditorial.Surface
        event.kind == CodexEvent.Kind.GUARDIAN_REVIEW && event.reviewStatus == "approved" -> PaEditorial.Running to PaEditorial.Surface
        event.kind == CodexEvent.Kind.DEPRECATION ||
            event.kind == CodexEvent.Kind.CONFIG_WARNING ||
            event.kind == CodexEvent.Kind.SANDBOX_WARNING -> PaEditorial.Attention to PaEditorial.Surface
        else -> PaEditorial.Primary to PaEditorial.Surface
    }
    val statusLabel = guardianStatusLabel(event.reviewStatus)
    val riskLabel = riskLevelLabel(event.riskLevel)
    val overridable = guardianOverridable(event) && !guardianOverridden && onApproveGuardian != null
    val skillsBudgetWarning =
        event.message.contains("skill", ignoreCase = true) &&
            event.message.contains("budget", ignoreCase = true)
    var expanded by remember(event.title, event.rawJson) { mutableStateOf(false) }

    Column(
        Modifier
            .fillMaxWidth()
            .background(bg, RoundedCornerShape(13.dp))
            .border(
                1.dp,
                if (PaHacker.isHacker) accent.copy(alpha = 0.35f) else PaEditorial.Hairline,
                RoundedCornerShape(13.dp),
            )
            .padding(horizontal = 12.dp, vertical = 10.dp),
        verticalArrangement = Arrangement.spacedBy(5.dp),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Text(
                event.title.ifBlank { "Codex 状态" },
                Modifier.weight(1f),
                color = accent,
                fontSize = (13 * uiScale).sp,
                fontWeight = FontWeight.SemiBold,
            )
            if (statusLabel.isNotBlank()) {
                Text(
                    statusLabel,
                    Modifier
                        .background(accent.copy(alpha = 0.16f), RoundedCornerShape(6.dp))
                        .padding(horizontal = 6.dp, vertical = 1.dp),
                    color = accent,
                    fontSize = (10 * uiScale).sp,
                    fontWeight = FontWeight.Medium,
                )
                Spacer(Modifier.width(6.dp))
            }
            // 「详情」展开的是协议原始 JSON —— 对"内容已经完整渲染"的事件(回合 diff)
            // 它就是一段用户看不懂的分层结构(2026-09-18 反馈:展开后像坏了),这类事件不给入口。
            if (event.rawJson.isNotBlank() && event.kind != CodexEvent.Kind.TURN_DIFF) {
                TextButton(
                    onClick = { expanded = !expanded },
                    contentPadding = PaddingValues(horizontal = 4.dp, vertical = 0.dp),
                ) {
                    Text(
                        if (expanded) "收起" else "详情",
                        color = accent,
                        fontSize = (11 * uiScale).sp,
                    )
                }
            }
        }
        if (riskLabel.isNotBlank()) {
            Text(riskLabel, color = accent, fontSize = (11 * uiScale).sp, fontWeight = FontWeight.Medium)
        }
        if (event.message.isNotBlank()) {
            Text(
                if (skillsBudgetWarning) {
                    "技能说明超出上下文预算，Codex 仍能使用全部技能，但部分说明被缩短。"
                } else {
                    event.message
                },
                color = ChatInkColor,
                fontSize = (12 * uiScale).sp,
            )
        }
        // 回合 diff:等宽展示,横向可滚(桌面上的改动行往往很长,
        // 手机上换行会把 diff 的语义彻底破坏)。石墨代码箱(编辑式)/旧底色(黑客)。
        if (event.diff.isNotBlank()) {
            Text(
                event.diff,
                Modifier
                    .fillMaxWidth()
                    .heightIn(max = 260.dp)
                    .verticalScroll(rememberScrollState())
                    .horizontalScroll(rememberScrollState())
                    .background(
                        if (PaHacker.isHacker) PaEditorial.Canvas else PaEditorial.GraphiteCode,
                        RoundedCornerShape(8.dp),
                    )
                    .padding(8.dp),
                color = if (PaHacker.isHacker) PaEditorial.Muted else PaEditorial.CodeText,
                fontSize = (10 * uiScale).sp,
                fontFamily = FontFamily.Monospace,
            )
        }
        if (event.url.isNotBlank()) {
            Text(
                event.url,
                color = if (PaHacker.isHacker) PaEditorial.Subtle else PaEditorial.Muted,
                fontSize = (10 * uiScale).sp,
                fontFamily = FontFamily.Monospace,
            )
        }
        /*
         * 「仍然执行」:审查者(guardian / 自动审查)驳回之后的唯一出路。
         * 不给这个入口,用户看到的就是"任务莫名失败了",而且他并不知道
         * 自己其实有权推翻 —— 这才是这个按钮存在的全部理由。
         */
        when {
            overridable -> TextButton(
                onClick = { onApproveGuardian(event) },
                contentPadding = PaddingValues(horizontal = 8.dp, vertical = 0.dp),
            ) {
                Text("仍然执行", color = ChatDeniedColor, fontSize = (12 * uiScale).sp, fontWeight = FontWeight.SemiBold)
            }
            guardianOverridable(event) && guardianOverridden -> Text(
                "已推翻这次驳回,继续执行",
                color = ChatRunningColor,
                fontSize = (11 * uiScale).sp,
            )
        }
        if (skillsBudgetWarning && onOpenSettings != null) {
            TextButton(
                onClick = onOpenSettings,
                contentPadding = PaddingValues(horizontal = 8.dp, vertical = 0.dp),
            ) {
                Text("查看技能与插件", color = ChatAccentColor, fontSize = (12 * uiScale).sp)
            }
        }
        if (expanded) {
            Text(
                event.rawJson,
                Modifier
                    .fillMaxWidth()
                    .background(
                        if (PaHacker.isHacker) PaEditorial.Canvas else PaEditorial.GraphiteCode,
                        RoundedCornerShape(8.dp),
                    )
                    .padding(8.dp),
                color = if (PaHacker.isHacker) PaEditorial.Muted else PaEditorial.CodeText,
                fontSize = (10 * uiScale).sp,
                fontFamily = FontFamily.Monospace,
            )
        }
    }
}

/**
 * 消息进入动画:淡入 + 轻微上移(新消息"落进"时间线,而不是凭空出现)。
 *
 * 【为什么只动 graphicsLayer】性能铁律:动画只碰 transform/opacity(绘制层),
 * 不触发布局与重绘;绝不给逐条消息挂 blur/重动画。
 *
 * 【为什么是 spring(PaMotion.emphasized)】可打断是铁律:动画没播完时这条消息
 * 若还在流式更新,从当前值续走,不跳帧不重播。
 *
 * 【arm 语义】只在"新消息追加的那一帧"为 true(调用方用上一帧的条目数判定)。
 * LaunchedEffect(Unit) 只在进入组合时跑一次:历史加载/回滚/往回滚动重组
 * (arm=false 或复用已有实例)绝不重播动画 —— 空间连续性:元素从哪来回哪去。
 * 减弱动效时瞬时到位(不播位移,消息照常出现)。
 */
@Composable
private fun MessageEnterAnimation(
    arm: Boolean,
    modifier: Modifier = Modifier,
    content: @Composable () -> Unit,
) {
    val reduceMotion = rememberReduceMotion()
    val progress = remember { Animatable(1f) }
    LaunchedEffect(Unit) {
        if (arm && !reduceMotion) {
            progress.snapTo(0f)
            progress.animateTo(1f, PaMotion.emphasized())
        }
    }
    Box(
        modifier.graphicsLayer {
            alpha = progress.value
            translationY = (1f - progress.value) * 14.dp.toPx()
        },
    ) { content() }
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
            .background(PanelCardColor, RoundedCornerShape(13.dp))
            .border(1.dp, PanelHairlineColor, RoundedCornerShape(13.dp))
            .padding(horizontal = 12.dp, vertical = 10.dp),
    ) {
        Row(
            Modifier.fillMaxWidth().pressable(haptic = true) { expanded = !expanded },
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Icon(
                painterResource(AppR.drawable.ic_lucide_brain), null,
                tint = ChatAccentColor, modifier = Modifier.size(13.dp),
            )
            Spacer(Modifier.width(6.dp))
            Text(
                "思考过程" + if (streaming) " · 正在思考" else "",
                Modifier.weight(1f),
                fontSize = 11.5.sp,
                color = ChatAccentColor,
                fontWeight = FontWeight.SemiBold,
                maxLines = 1,
            )
            Text(if (expanded) "收起" else "展开", fontSize = 11.sp, color = ChatSubtleColor)
        }
        if (expanded) {
            Text(
                text,
                Modifier.fillMaxWidth().padding(top = 6.dp),
                fontSize = (11.5f * scale).sp,
                fontFamily = FontFamily.Monospace,
                color = ChatMutedColor,
            )
        }
    }
}

/**
 * 执行计划卡:codex turn/plan 的步骤列表。
 * 状态:completed=实心绿 / in_progress=蓝色(带呼吸)/ pending=空心灰;
 * 流式阶段还可附带"生成中"的草稿文本。
 */
/**
 * 执行计划面板(会话上方,可展开/收起)。
 *
 * 【为什么要做成面板而不是时间线卡片】
 * 计划卡原来跟着消息流走:跑完就滚出屏幕,想回头核对"还剩哪几步"要翻半天;
 * 而且它和"正在做什么"是同一件事 —— 应该是**常驻的**上下文,不是一次性输出。
 * 现在:未完成时它就是进度条(标题行直接显示 n/m),完成后自动收起成一行,
 * 点一下展开看全部步骤,再点收起。收起状态不占屏幕,展开状态不丢上下文。
 *
 * 【流式排版】草稿原文是一大段没有结构的文字,这里按空行/序号/短横线切段,
 * 每段单独一行、行距拉开,读到哪一步一眼能分清 —— 只改排版,不动整体结构。
 */
@Composable
private fun PlanPanel(
    steps: List<String>,
    draft: String,
    running: Boolean,
    expanded: Boolean,
    onToggle: () -> Unit,
    outerGutter: Dp = 0.dp,
) {
    // 解析步骤:同一条"steps 尚未生成、只有草稿"的阶段由 draft 兜底
    val parsed = remember(steps) {
        steps.mapNotNull { raw ->
            val status = raw.substringBefore('|')
            val text = raw.substringAfter('|', "").trim()
            if (text.isBlank()) null else status to text
        }
    }
    val done = parsed.count { it.first == "completed" }
    val total = parsed.size
    val current = parsed.firstOrNull { it.first == "in_progress" }?.second

    Column(Modifier.fillMaxWidth().padding(horizontal = outerGutter, vertical = 2.dp)) {
        Column(
            Modifier
                .fillMaxWidth()
                .background(PanelCardColor, RoundedCornerShape(13.dp))
                .border(1.dp, PanelHairlineColor, RoundedCornerShape(13.dp))
                .pressable { onToggle() }
                .padding(horizontal = 12.dp, vertical = 9.dp),
        ) {
            // ---- 标题行:总是可见(收起时就是全部) ----
            Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                Icon(
                    painterResource(AppR.drawable.ic_lucide_list_checks), null,
                    tint = if (running) ChatAccentColor else ChatRunningColor,
                    modifier = Modifier.size(13.dp),
                )
                Spacer(Modifier.width(6.dp))
                Text(
                    when {
                        running && total > 0 -> "执行计划 · $done/$total"
                        running -> "执行计划 · 制定中"
                        total > 0 && done == total -> "执行计划 · 已完成 $total 步"
                        total > 0 -> "执行计划 · 完成 $done/$total"
                        else -> "执行计划"
                    },
                    Modifier.weight(1f),
                    fontSize = 11.5.sp,
                    color = if (running) ChatAccentColor else ChatMutedColor,
                    fontWeight = FontWeight.SemiBold,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
                // 收起时把"当前这一步"直接显示在标题行 —— 不展开也知道在干什么
                if (!expanded && current != null) {
                    Text(
                        current,
                        Modifier.widthIn(max = 150.dp),
                        fontSize = 11.sp,
                        color = ChatSubtleColor,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                    Spacer(Modifier.width(6.dp))
                }
                Icon(
                    painterResource(
                        if (expanded) AppR.drawable.ic_lucide_chevron_up
                        else AppR.drawable.ic_lucide_chevron_down,
                    ),
                    null,
                    tint = ChatSubtleColor,
                    modifier = Modifier.size(14.dp),
                )
            }

            // ---- 进度条:有步骤时画一条细线,一眼看出大概还剩多少 ----
            if (total > 0) {
                Spacer(Modifier.height(7.dp))
                Box(
                    Modifier
                        .fillMaxWidth()
                        .height(3.dp)
                        .background(
                            if (PaHacker.isHacker) PaEditorial.Hover else PaEditorial.Raised,
                            RoundedCornerShape(2.dp),
                        ),
                ) {
                    Box(
                        Modifier
                            .fillMaxWidth(if (total == 0) 0f else done.toFloat() / total)
                            .height(3.dp)
                            .background(
                                if (running) ChatAccentColor else ChatRunningColor,
                                RoundedCornerShape(2.dp),
                            ),
                    )
                }
            }

            // ---- 展开区:完整步骤 + 生成中的草稿 ----
            if (expanded) {
                Spacer(Modifier.height(8.dp))
                if (parsed.isEmpty()) {
                    // 还没有结构化步骤:把草稿按可读段落排版
                    PlanDraftText(draft)
                } else {
                    parsed.forEach { (status, text) ->
                        Row(
                            Modifier.fillMaxWidth().padding(vertical = 3.dp),
                            verticalAlignment = Alignment.Top,
                        ) {
                            val dotColor = when (status) {
                                "completed" -> ChatRunningColor
                                "in_progress" -> ChatAccentColor
                                else -> ChatSubtleColor
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
                                Modifier.weight(1f),
                                fontSize = 12.sp,
                                color = if (status == "pending") ChatMutedColor else ChatInkColor,
                            )
                        }
                    }
                    // 步骤已出但仍在跑:把最新草稿附在下面(可能是下一步的说明)
                    if (running && draft.isNotBlank()) {
                        Spacer(Modifier.height(6.dp))
                        PlanDraftText(draft)
                    }
                }
            }
        }
    }
}

/**
 * 计划草稿的可读排版。
 *
 * 【问题】daemon 推来的草稿是模型原样吐出的自由文本,可能是一整段流水句,
 * 直接 Text() 出来就是"一大坨字",读不出层次。
 * 【做法】按空行切段;段内若以 `1.` `-` `*` `•` 或 `第n步` 开头,提成独立条目;
 * 条目之间用固定行距隔开,序号行稍亮、普通行稍暗。**不改内容,只改落版**。
 */
@Composable
private fun PlanDraftText(draft: String) {
    val lines = remember(draft) {
        draft.split("\n")
            .flatMap { it.split(Regex("(?<=[。;；])\\s*(?=\\d+[.、])")) }
            .map { it.trim() }
            .filter { it.isNotEmpty() }
    }
    Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(3.dp)) {
        lines.forEach { line ->
            val isItem = Regex("^(\\d+[.、)]|[-*•]|第\\s*\\d+\\s*步)").containsMatchIn(line)
            Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.Top) {
                if (isItem) {
                    Box(
                        Modifier
                            .padding(top = 5.dp)
                            .size(4.dp)
                            .background(ChatSubtleColor, CircleShape),
                    )
                    Spacer(Modifier.width(7.dp))
                }
                Text(
                    line,
                    Modifier.weight(1f),
                    fontSize = 11.5.sp,
                    lineHeight = 17.sp,
                    color = if (isItem) ChatMutedColor else ChatSubtleColor,
                )
            }
        }
    }
}

/**
 * 工具调用卡:一次调用一张卡(同一 call id 原地更新),
 * 展开可见调用的参数与输出,并显示耗时与成功/失败状态。
 */
/**
 * 判断一条工具调用的入参是否像 ZCode 子代理委派("Agent"/"Task" 工具)。
 *
 * 【为什么需要】ZCode 的子代理是普通工具调用,app-server 协议不发射任何子代理
 * 专用事件(见 daemon/internal/agent/zcode.go 的 Run 分发 switch),子代理内部
 * 对话完全不透出;手机端唯一能看到的就是这次 tool.updated 的入参(prompt/description)
 * 与结果。为了不让用户只看到一个"完成"的空卡,子代理工具卡默认展开,直接露出
 * "任务"(入参)与"产出"(结果)。这是项目铁律"识别不了≠出错、不伪造"的边界:
 * 此处只展示原生工具调用参数/结果,绝不编造子代理对话。
 *
 * 【识别依据,纯结构、不写死工具名】与 daemon 的 zcodeapp.ToolInputIsSubagent
 * 同源:ZCode Agent 工具入参稳定含 prompt(必填) + 一个子代理特征键
 * {description, agentType, subagent_type, outputFile, parentToolCallId}。用子串
 * 判定(入参是 daemon 已校验的合法 JSON),不引入 JSON 解析依赖;识别失败只返回
 * false(当普通工具,不自动展开),不报错、不降级。工具名(Agent/Task/subagent)
 * 怎么改都不影响识别,也不会把 Bash(command)/Write(file_path)误判。
 */
internal fun looksLikeSubagentDelegation(args: String): Boolean {
    if (args.isBlank()) return false
    if (!args.contains("\"prompt\"")) return false
    return args.contains("\"description\"") ||
        args.contains("\"agentType\"") ||
        args.contains("\"subagent_type\"") ||
        args.contains("\"outputFile\"") ||
        args.contains("\"parentToolCallId\"")
}

// ---- P0-4 常规模式工具卡折叠(纯函数,JVM 单测钉死,见 ToolCardCollapseTest)----

/** 文件类工具:折叠行摘要取目标文件名(路径太长,文件名一眼可辨)。 */
private val ToolCardFileTools = setOf("read", "write", "edit", "multiedit", "notebookedit")

/**
 * 工具卡默认展开判定(纯函数)。折叠一行是默认态;三类例外按优先级:
 * 红线卡(审批/反问类工具,折叠会藏住"正在等你拍板"的语义)> 全局「展开工具详情」
 * 设置(用户明说要看)> 子代理委派卡(委派内容必须可见,否则只剩一张"完成"空卡,
 * 识别依据见 [looksLikeSubagentDelegation])。
 */
internal fun toolCardDefaultExpanded(
    userPrefersExpanded: Boolean,
    isSubagent: Boolean,
    isRedLine: Boolean,
): Boolean = isRedLine || userPrefersExpanded || isSubagent

/**
 * 折叠行的工具名摘要(纯函数):Bash 类取命令首行,文件类取目标文件名,
 * 其余工具返回空串(折叠行只显示工具名)。JSON 解析失败一律空串回退 ——
 * 摘要只求一眼可辨、绝不编造;超长截断保留完整语义的首段。
 */
internal fun toolCallHeadline(toolName: String, toolArgs: String, maxLen: Int = 32): String {
    if (toolArgs.isBlank()) return ""
    val obj = runCatching { JSONObject(toolArgs) }.getOrNull() ?: return ""
    val lower = toolName.trim().lowercase()
    val raw = when {
        lower == "bash" || lower == "shell" || lower == "terminal" -> obj.optString("command")
        lower in ToolCardFileTools -> obj.optString("file_path")
        else -> return ""
    }
    val line = raw.lineSequence().firstOrNull { it.isNotBlank() }?.trim().orEmpty()
    if (line.isEmpty()) return ""
    val headline = if (lower in ToolCardFileTools) {
        line.substringAfterLast('/').substringAfterLast('\\')
    } else {
        line
    }
    return if (headline.length <= maxLen) headline else headline.take(maxLen) + "…"
}

// ---- R2-A 工具卡长输出渐进披露(纯函数,JVM 单测钉死,见 ChatToolOutputPreviewTest)----

/** 预览保留头部多少字:开头是命令回显/参数确认,信息密度最高。 */
internal const val TOOL_OUTPUT_PREVIEW_HEAD_CHARS = 2000

/** 预览保留尾部多少字:错误堆栈/最终结果多在输出末尾,尾部必须留。 */
internal const val TOOL_OUTPUT_PREVIEW_TAIL_CHARS = 400

/** 隐藏量下限:差这点字不值得多一次点击,原样全显(省一次交互,不玩阉割式预览)。 */
internal const val TOOL_OUTPUT_PREVIEW_MIN_OMITTED = 160

/** 头尾之间省略段的占位提示;独立成行,渲染在预览文本中间(字符数不计入「还有 N 字」)。 */
internal const val TOOL_OUTPUT_PREVIEW_OMITTED_MARK = "\n…(中间省略)…\n"

/** 工具输出的预览结果:visible 直接渲染;truncated=false 时 visible 就是全文。 */
data class ToolOutputPreview(
    val visible: String,
    val truncated: Boolean,
    /** 被省略的真实字符数(不造数,按钮上报它)。truncated=false 恒 0。 */
    val omittedChars: Int,
)

/**
 * 长输出 → 头尾预览(纯函数)。参数负数一律按 0 兜底;隐藏量不足
 * [TOOL_OUTPUT_PREVIEW_MIN_OMITTED] 时不截(可见 [ToolOutputPreview.truncated])。
 */
internal fun toolOutputPreview(
    text: String,
    headChars: Int = TOOL_OUTPUT_PREVIEW_HEAD_CHARS,
    tailChars: Int = TOOL_OUTPUT_PREVIEW_TAIL_CHARS,
    minOmitted: Int = TOOL_OUTPUT_PREVIEW_MIN_OMITTED,
): ToolOutputPreview {
    val head = headChars.coerceAtLeast(0)
    val tail = tailChars.coerceAtLeast(0)
    val omitted = text.length - head - tail
    if (omitted <= minOmitted.coerceAtLeast(0)) {
        return ToolOutputPreview(visible = text, truncated = false, omittedChars = 0)
    }
    return ToolOutputPreview(
        visible = text.take(head) + TOOL_OUTPUT_PREVIEW_OMITTED_MARK + text.takeLast(tail),
        truncated = true,
        omittedChars = omitted,
    )
}

@Composable
private fun ToolCallCard(item: ChatItem, scale: Float) {
    // P0-4:常规工具卡默认折叠一行(▸ 工具名+摘要 · 状态/耗时),点击展开全量参数/输出。
    // 默认展开判定走纯函数 toolCardDefaultExpanded(红线卡 > 全局设置 > 子代理委派,
    // 优先级理由见该函数注释);折叠状态 rememberSaveable 按消息 id(toolId)留档 ——
    // 滑出屏幕再滚回来不丢、不闪(普通 remember 会被 LazyColumn 回收重置)。
    val isSubagent = looksLikeSubagentDelegation(item.toolArgs)
    var expanded by rememberSaveable(item.toolId, AgentConnection.toolDetailsExpanded) {
        mutableStateOf(
            toolCardDefaultExpanded(
                userPrefersExpanded = AgentConnection.toolDetailsExpanded,
                isSubagent = isSubagent,
                // 红线语义:审批/反问类工具卡绝不折叠 —— 等用户拍板的东西不能藏
                isRedLine = askIsAskAction(item.toolName),
            ),
        )
    }
    // 折叠行的"一眼摘要"(Bash 命令首行/文件名):参数长时只在这里露一个抓手,
    // 全文展开再看(纯函数 toolCallHeadline,解析失败回退空串只显工具名)。
    val headline = remember(item.toolName, item.toolArgs) {
        toolCallHeadline(item.toolName, item.toolArgs)
    }
    // ▸/▾ 折叠指示:有箭头才有"可以点开"的暗示(折叠态横指,展开态转向下)
    val chevronRotation by animateFloatAsState(
        targetValue = if (expanded) 90f else 0f,
        label = "toolCardChevron",
    )
    // 状态语义:failure=未完成(珊瑚红)/ success=已完成(中性墨)/ running=正在执行(主色)。
    // 词走四词纪律(编辑式升级:状态必须写词,语义不单靠颜色);黑客终端保持旧词旧色。
    val statusColor = when (item.toolStatus) {
        "failure" -> ChatDeniedColor
        "success" -> PaEditorial.Running
        else -> ChatAccentColor
    }
    Column(
        Modifier
            .fillMaxWidth()
            .background(PanelCardColor, RoundedCornerShape(13.dp))
            .border(1.dp, PanelHairlineColor, RoundedCornerShape(13.dp))
            .padding(horizontal = 12.dp, vertical = 10.dp),
    ) {
        Row(
            Modifier
                .fillMaxWidth()
                // 折叠行整行可点(最小点击区 44dp 由行高+上下 padding 保证)
                .heightIn(min = 24.dp)
                .pressable(haptic = true) { expanded = !expanded },
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Icon(
                painterResource(AppR.drawable.ic_lucide_chevron_right),
                null,
                tint = ChatSubtleColor,
                modifier = Modifier
                    .size(12.dp)
                    .graphicsLayer { rotationZ = chevronRotation },
            )
            Spacer(Modifier.width(6.dp))
            Box(Modifier.size(8.dp).background(statusColor, CircleShape))
            Spacer(Modifier.width(8.dp))
            // 名字与摘要共享剩余宽度(各占一半封顶,超长省略号),
            // 耗时/状态恒贴右缘 —— 与 hacker 终端工具块同一排布(两路径一张皮)。
            Row(Modifier.weight(1f), verticalAlignment = Alignment.CenterVertically) {
                Text(
                    item.toolName.ifBlank { "工具" },
                    Modifier.weight(1f, fill = false),
                    fontSize = (13 * scale).sp,
                    color = ChatInkColor,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
                if (headline.isNotEmpty()) {
                    Spacer(Modifier.width(6.dp))
                    Text(
                        headline,
                        Modifier.weight(1f, fill = false),
                        fontSize = (11 * scale).sp,
                        color = ChatSubtleColor,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                }
            }
            if (item.toolElapsedMs > 0) {
                Text(
                    String.format(java.util.Locale.US, "%.1fs", item.toolElapsedMs / 1000.0),
                    fontSize = 10.5.sp,
                    color = ChatSubtleColor,
                )
                Spacer(Modifier.width(8.dp))
            }
            Text(
                when (item.toolStatus) {
                    "failure" -> "未完成"
                    "success" -> "已完成"
                    else -> "正在执行"
                },
                fontSize = 10.5.sp,
                color = statusColor,
            )
        }
        // 展开区出入场走 spring(PaMotion 令牌,可打断):披露动效让"多出来的内容有
        // 来处",折叠↔展开的状态变化需要连续性,不是装饰 —— 与折叠行 chevron 转向
        // 同一语义。hacker 终端工具块(TerminalToolBlock)不受影响。
        // 【全限定名】Column 作用域内同名扩展会抢走解析,要的是顶层函数(同"新内容"胶囊)。
        androidx.compose.animation.AnimatedVisibility(
            visible = expanded,
            enter = expandVertically(PaMotion.emphasized()) + fadeIn(PaMotion.emphasized()),
            exit = shrinkVertically(PaMotion.standard()) + fadeOut(PaMotion.standard()),
            label = "toolCardDetails",
        ) {
            Column {
                // 代码/参数区:石墨代码箱(#1A2430)+ 代码浅蓝字(README:代码区保留石墨深色);
                // 黑客终端保持旧底色(显示模式行为不变)。
                val codeBoxColor = if (PaHacker.isHacker) PaEditorial.Canvas else PaEditorial.GraphiteCode
                val codeTextColor = if (PaHacker.isHacker) PaEditorial.Ink else PaEditorial.CodeText
                if (item.toolArgs.isNotBlank()) {
                    // 标签行:段名 + 「复制」—— 代码箱自带复制口,长 JSON/命令不必靠
                    // 长按拖选(选区工具栏在代码箱里也难用)。轻触感由 pressable 自带。
                    Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                        Text("参数", Modifier.weight(1f), fontSize = 10.5.sp, color = ChatSubtleColor)
                        ToolCardCopyChip("复制", item.toolArgs)
                    }
                    Text(
                        item.toolArgs,
                        Modifier
                            .fillMaxWidth()
                            .padding(top = 2.dp)
                            .background(codeBoxColor, RoundedCornerShape(8.dp))
                            // 高度封顶 + 纵横双向滚动:超长参数/输出不再把卡撑成一屏,
                            // 长行保横向滚动不硬折(diff/JSON 的对齐语义,同回合 diff 口径)。
                            .heightIn(max = 260.dp)
                            .padding(8.dp)
                            .verticalScroll(rememberScrollState())
                            .horizontalScroll(rememberScrollState()),
                        fontSize = (10.5f * scale).sp,
                        fontFamily = FontFamily.Monospace,
                        // 不折行,交给横向滚动(与 MarkdownText 代码块同一取舍)
                        softWrap = false,
                        color = codeTextColor,
                    )
                }
                if (item.text.isNotBlank()) {
                    val preview = remember(item.text) { toolOutputPreview(item.text) }
                    // 长输出"展开全部"按消息留档(rememberSaveable 按 toolId):滑出屏幕
                    // 再滚回来不丢(与折叠态同一纪律)。流式期间文本增长不重置选择。
                    var showFullOutput by rememberSaveable(item.toolId) { mutableStateOf(false) }
                    Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                        Text(
                            if (item.toolStatus == "failure") "错误" else "输出",
                            Modifier.weight(1f),
                            fontSize = 10.5.sp,
                            color = ChatSubtleColor,
                        )
                        ToolCardCopyChip("复制", item.text)
                    }
                    Text(
                        if (showFullOutput) item.text else preview.visible,
                        Modifier
                            .fillMaxWidth()
                            .padding(top = 2.dp)
                            .background(codeBoxColor, RoundedCornerShape(8.dp))
                            .heightIn(max = 260.dp)
                            .padding(8.dp)
                            .verticalScroll(rememberScrollState())
                            .horizontalScroll(rememberScrollState()),
                        fontSize = (10.5f * scale).sp,
                        fontFamily = FontFamily.Monospace,
                        softWrap = false,
                        color = if (item.toolStatus == "failure") ChatDeniedColor
                        else if (PaHacker.isHacker) PaEditorial.Muted else PaEditorial.CodeText,
                    )
                    // 长输出渐进披露第二步:默认只渲染头部预览(纯函数 toolOutputPreview),
                    // 余量按真实字符数报在按钮上(不造数);展开后可收回。
                    if (preview.truncated) {
                        Text(
                            if (showFullOutput) "收起"
                            else "展开全部(还有 ${preview.omittedChars} 字)",
                            Modifier
                                .pressable { showFullOutput = !showFullOutput }
                                .padding(horizontal = 8.dp, vertical = 6.dp),
                            fontSize = 11.sp,
                            fontWeight = FontWeight.SemiBold,
                            color = ChatAccentColor,
                        )
                    }
                }
                if (item.toolArgs.isBlank() && item.text.isBlank()) {
                    Text("等待结果…", Modifier.padding(top = 8.dp), fontSize = 11.sp, color = ChatSubtleColor)
                }
            }
        }
    }
}

/**
 * 工具卡展开区的「复制」小口:标签行右侧的次级文字动作,复制整段参数/输出。
 * 低调(Subtle 11sp,与段名同档)但触感完整(pressable 自带);复制回执由
 * [copyMessage] 的 Toast 统一给,这里不再叠一层反馈。
 */
@Composable
private fun ToolCardCopyChip(label: String, payload: String) {
    val ctx = LocalContext.current
    Text(
        label,
        Modifier
            .pressable { copyMessage(ctx, payload) }
            .padding(horizontal = 8.dp, vertical = 6.dp),
        fontSize = 11.sp,
        color = ChatSubtleColor,
    )
}

/**
 * 当前 agent 的"默认模型" id。
 *
 * daemon 的 ListModels 实测 id 带 agent 前缀(`codex/default`、`echo/echo-1` …),
 * 用 `$agent/default` 表示"随桌面配置生效的模型"。抽成函数供两处共用,避免
 * "一处泛化、一处硬编码 codex" 再分叉。agent 为空时退回 codex。
 */
private fun agentDefaultModelId(agent: String): String =
    if (agent.isBlank()) "codex/default" else "$agent/default"

/**
 * 按会话归属 agent 过滤模型表(纯函数,单测钉死 —— 见 ChatModelRoutingTest)。
 *
 * 【为什么必须过滤】daemon 的 ListModels 是全局列表:每个 provider 的模型 id 都带
 * agent 前缀("codex/GLM-5.3-Flash"、"pi/zai-coding-cn/glm-5.3"、…),首段即归属
 * agent。真机实锤(图3):Pi 会话的模型弹窗把别家(codex/zcode)的模型表全列了
 * 出来 —— 根因就是拿全表直接渲染。按前缀过滤后,Pi 会话只见 Pi 自己的模型。
 *
 * agentId 为空(会话归属还没回填)时返回全表:与旧行为一致,回填后立即收窄。
 */
internal fun modelsForAgent(models: List<ModelInfo>, agentId: String): List<ModelInfo> {
    if (agentId.isBlank()) return models
    return models.filter { it.id.substringBefore('/') == agentId }
}

/**
 * 模型 id 的"裸名"显示:去掉首段 agent 前缀(纯函数)。
 * "pi/glm-5.3" → "glm-5.3";三段式("pi/zai-coding-cn/glm-5.3")只去首段,
 * 余下原样保留 —— 与 daemon 的 Thread.ModelID 形状一致,不猜第二段语义。
 * 无前缀的裸 id 原样返回(substringAfter 的缺省兜底必须给 id 本身,不能给空串)。
 */
internal fun modelBareName(id: String): String = id.substringAfter('/', id)

/**
 * 【选模型的共享落点】完整选择卡(ui/model.ModelEffortCard)与模型行快速切换面板
 * 共用同一份"选中之后做什么":标 selectionPending + 仓库 send(SelectModel)+
 * 记一次"最近使用"(ModelQuickSwitchLogic,进程内真实数据,驱动快速面板的 MRU 排序)。
 *
 * 【P3-T4 接线③】选择本体改走 ModelSelectionRepository:乐观更新、失效 effort
 * 清理(reducer cleanEffortFor,档位集合不含当前强度即清成"跟随默认")、会话级
 * 缓存与 daemon 真值回填全在仓库一条链上 —— 本函数不再直写
 * AgentConnection.selectedModel / setThreadModel / setThreadEffort
 * (旧的三路直写是"多个 mutable 全局互相覆盖"的病根,计划 Task 3/4 收敛点)。
 * 仓库按进入会话时 open(threadId, agentId) 登记的会话下发,这里不重复传路由。
 */
internal fun applyModelPick(
    threadId: String,
    agentId: String,
    id: String,
) {
    // 轮次标记:显式选择即将影响下一轮 —— 在下一轮 TurnInput 确认前,巡检的
    // 原生回填不再覆盖刚选的值(TurnSession.acceptRuntimeSelection 的闸)。
    TurnSession.state(threadId).selectionPending = true
    ModelSelectionRepository.send(ModelSelectionIntent.SelectModel(id))
    ModelQuickSwitchLogic.rememberPick(agentId, id)
}

/**
 * "压缩上下文"动作的后果文案,按会话归属 agent 动态(纯函数,单测钉死)。
 * 真机实锤(图9):Pi 会话里显示"让 Codex 压缩历史上下文" —— 文案硬编码 codex。
 * daemon 的 CompactThread RPC 已按会话 AgentID 路由(pi/zcode/codex 各走各的
 * 压缩),文案必须跟上;agent 未知(还没回填)时退中性说法,不点名。
 */
internal fun compactionConsequenceCopy(agentId: String): String =
    if (agentId.isBlank()) "压缩历史上下文以释放窗口空间,不会撤销文件改动。"
    else "让 ${selectedAgentLabel(agentId)} 压缩历史上下文以释放窗口空间,不会撤销文件改动。"

/**
 * 上下文窗口解析优先级(从高到低):
 *  1) live:daemon 随用量上报的真实窗口(唯一权威值)
 *  2) modelDeclared:当前生效模型自己声明的窗口
 *  3) agentDefaultDeclared:当前 agent 的默认模型声明的窗口
 * 只要拿到 >0 就算"窗口已知";全为 0 才返回 0(未知 → 环内画"—",绝不假装 0%)。
 *
 * 未知窗口必须保持未知。不能把历史默认值或 App 内置常量当成 Codex 的真实窗口,
 * 否则占用百分比会与 Codex 实际显示不一致。
 */
internal fun pickContextWindow(live: Long, modelDeclared: Int, agentDefaultDeclared: Int): Long {
    if (live > 0) return live
    if (modelDeclared > 0) return modelDeclared.toLong()
    if (agentDefaultDeclared > 0) return agentDefaultDeclared.toLong()
    return 0L
}

// ---- P0-1 流式滚动锚定(纯函数,JVM 单测钉死,见 ChatScrollAnchorTest)----
//
// 【为什么要有这套判定】旧实现是"新 token 一到就 scrollToItem(末尾)",用户上滑
// 阅读历史会被拽回底部(对标 ChatGPT/Claude 的标准策略:距底才跟随,离底出胶囊)。
// 判定全部抽成纯函数;状态(跟随/未读)留在 ChatScreen 的列表容器层 —— 常规气泡与
// hacker 终端块共用同一条 LazyColumn,锚定逻辑放这一层,两条渲染路径天然一致。

/** 跟随阈值(px):距底部 ≤100px 视为"贴底",恢复跟随;超过即"离底"。 */
internal const val SCROLL_FOLLOW_THRESHOLD_PX = 100

/** 流式正文按多少字符记 1 个"新内容块"(与跟随 effect 的重启节奏同源,可测可算)。 */
internal const val STREAM_BLOCK_CHARS = 24

/**
 * 流式正文喂给 InAppMarkdown 的节流间隔(ms)(审查 P1-22):delta 帧只更新
 * displayedStream 快照,间隔内的所有中间帧自动合并,长流式不再逐帧全文重解析。
 * 120ms 的流式视觉延迟无感。
 */
internal const val STREAM_RENDER_THROTTLE_MS = 120L

/**
 * 当前滚动位置距底部的像素(纯函数)。
 *
 * 最后一项不在可视区(用户翻到了中间)→ 距底"无限远"(Int.MAX_VALUE);
 * 最后一项完全可见 → 视口底缘到该项底缘的剩余距离(≤0 按 0,半屏长项贴底就是 0)。
 * 无内容/无可见项 → 0(空列表无所谓跟随,判定恒真,新消息来了就滚)。
 */
internal fun distanceFromBottomPx(
    totalItems: Int,
    lastVisibleIndex: Int,
    lastVisibleEnd: Int,
    viewportEndOffset: Int,
): Int {
    if (totalItems <= 0 || lastVisibleIndex < 0) return 0
    if (lastVisibleIndex < totalItems - 1) return Int.MAX_VALUE
    return (viewportEndOffset - lastVisibleEnd).coerceAtLeast(0)
}

/** 距底 ≤100px = 贴底(用户手动滚回这个范围内就恢复跟随)。 */
internal fun shouldFollowDistance(distanceFromBottomPx: Int): Boolean =
    distanceFromBottomPx <= SCROLL_FOLLOW_THRESHOLD_PX

/**
 * 新内容到达时是否自动回底:本来就在跟随(流式中持续钉底),**或**用户其实仍在
 * 距底 100px 内(没真正离开,只是内容短、列表没滚动)。真离底(上滑阅读)→ 不拽,
 * 交给「↓ N 条新内容」胶囊。
 */
internal fun shouldAutoScroll(distanceFromBottomPx: Int, wasFollowing: Boolean): Boolean =
    wasFollowing || shouldFollowDistance(distanceFromBottomPx)

/** 流式文本长度 → 块序号(每 STREAM_BLOCK_CHARS 个字符 1 块;负数防御按 0)。 */
internal fun streamBlockIndex(textLength: Int): Int =
    textLength.coerceAtLeast(0) / STREAM_BLOCK_CHARS

/**
 * 未读新块累计(纯函数):跟随中恒 0(用户看得见,没有"未读");离底期间每新增
 * 1 条完整消息 +1、流式正文每多 1 块 +1 —— 胶囊上的 N 就是它,不造数。
 */
internal fun accumulateUnreadBlocks(
    wasFollowing: Boolean,
    currentUnread: Int,
    newItemBlocks: Int,
    newStreamBlocks: Int,
): Int {
    if (wasFollowing) return 0
    return currentUnread + newItemBlocks.coerceAtLeast(0) + newStreamBlocks.coerceAtLeast(0)
}

/** LazyListState → 距底像素(组合层取数,判定交给纯函数 distanceFromBottomPx)。 */
private fun distanceFromBottomOfList(state: LazyListState): Int {
    val info = state.layoutInfo
    val last = info.visibleItemsInfo.lastOrNull()
    return distanceFromBottomPx(
        totalItems = info.totalItemsCount,
        lastVisibleIndex = last?.index ?: -1,
        lastVisibleEnd = (last?.offset ?: 0) + (last?.size ?: 0),
        viewportEndOffset = info.viewportEndOffset,
    )
}

/** 审批策略选项(与 codex approvalPolicy 对齐)。 */
val APPROVALS = listOf(
    Triple("untrusted", "仅信任", "白名单外的命令先批准"),
    Triple("on-request", "按需询问", "运行命令前征求批准"),
    Triple("never", "完全访问", "无需批准直接执行,注意风险"),
)

// ---- 离线快照集成(P1-8 阶段 D;纯函数集中在此,可 JVM 单测)----

/**
 * 断连时是否用本地快照渲染时间线:
 * 仅在「已知断网 + 时间线还没有任何消息」时,读 [dev.pocketagent.app.ui.component.ThreadSnapshot]
 * 渲染只读副本 —— 有连接时行为完全不变。
 *
 * 【为什么只认 "down" 不认 "checking"】checking 是连接巡检的中间态,不是"已知断网";
 * 按 != "ok" 判离线会在巡检瞬间闪出假离线(见 ChatScreen 顶部 connState 的长注释)。
 */
internal fun shouldRenderOfflineSnapshot(connState: String, itemsEmpty: Boolean): Boolean =
    connState == "down" && itemsEmpty

/**
 * 「离线快照 · HH:mm」横幅文案;无快照时间戳(<=0)返回 null(不显示横幅)。
 */
internal fun offlineSnapshotBanner(atMs: Long): String? {
    if (atMs <= 0) return null
    val hhmm = java.text.SimpleDateFormat("HH:mm", java.util.Locale.getDefault()).format(java.util.Date(atMs))
    return "离线快照 · $hhmm"
}

// ---- 时间线首载骨架 / 空会话空态(R1 深接残项收口;纯函数,单测钉死)----
// hermes 等会话首次进入时历史在途,旧实现整屏空白 —— 在途读作"卡死",空历史
// 没有交代。骨架/空态都只在数据真实处于那一态时出现,绝不盖住错误横幅或离线副本。

/**
 * 是否显示"历史还在加载"骨架:时间线为空、首次历史还没回来、没有报错、
 * 也没在离线副本上。任一否定条件成立就不摆 —— 有内容/已加载完/明确报错/
 * 看副本,都轮不到骨架说话。
 */
internal fun shouldShowTimelineSkeleton(
    itemsEmpty: Boolean,
    historyLoaded: Boolean,
    hasLoadError: Boolean,
    offlineSnapshot: Boolean,
): Boolean = itemsEmpty && !historyLoaded && !hasLoadError && !offlineSnapshot

/**
 * 是否显示"空会话"空态:历史确实加载完且一条都没有,且不在运行/恢复
 * (轮次帧马上会来,不该闪空态),也没有报错/离线副本。
 */
internal fun shouldShowTimelineEmptyState(
    itemsEmpty: Boolean,
    historyLoaded: Boolean,
    hasLoadError: Boolean,
    offlineSnapshot: Boolean,
    running: Boolean,
    recovering: Boolean,
): Boolean =
    itemsEmpty && historyLoaded && !hasLoadError && !offlineSnapshot && !running && !recovering

/**
 * 空会话空态文案:主行陈述事实,副行给下一步(发第一条)。agent 名字带上让
 * "在跟谁说话"可辨;归属未知退中性"agent",不冒名。无禁词(单测钉死)。
 */
internal fun timelineEmptyStateCopy(agentLabel: String): Pair<String, String> {
    val who = agentLabel.ifBlank { "agent" }
    return "还没有消息" to "发第一条消息，和 $who 开始这次任务"
}

// ---- 编辑式迁移(2026-09-29 UI 整体升级·第二波 2/4)纯函数区 ----
// 会话页迁到编辑式设计时新增的纯映射,集中在此便于 JVM 单测(ChatEditorialMigrationTest)。

/**
 * 投递徽标的编辑式文案(纯函数)。
 *
 * 【四态保持、措辞对齐设计】README §主流程异常分支:「发送状态从待发送→已送达→
 * 执行中;失败保留内容可重试」。daemon 的四级回执(见 TurnDelivery.kt)保持不变,
 * 只换嘴:
 *   pending(发送中…)        → 待发送
 *   steered(已插入运行中任务) → 已送达 · 执行中(插进当前轮 = daemon 已收到且在跑)
 *   queued(排队中 · 第N位)    → 已送达 · 排队中(已到 daemon,等前一轮结束)
 *   overflow(队列已满,未送达) → 未送达 · 内容已保留 · 点按重发(失败级,气泡可点重试)
 */
internal data class EditorialDeliveryCopy(val word: String, val detail: String, val failed: Boolean)

internal fun editorialDeliveryCopy(state: String, queuePos: Int): EditorialDeliveryCopy = when (state) {
    DELIVERY_PENDING -> EditorialDeliveryCopy("待发送", "", false)
    DELIVERY_STEERED -> EditorialDeliveryCopy("已送达", "执行中", false)
    DELIVERY_QUEUED -> EditorialDeliveryCopy(
        "已送达",
        if (queuePos > 1) "排队中 · 第${queuePos}位" else "排队中 · 下一条",
        false,
    )
    DELIVERY_OVERFLOW -> EditorialDeliveryCopy("未送达", "内容已保留 · 点按重发", true)
    else -> EditorialDeliveryCopy("", "", false)
}

/** 审批动作类型 → 弹窗「操作类型」行的人话词(与原审批卡标题同一套映射,不编新义)。 */
internal fun approvalActionKindLabel(name: String): String = when (name) {
    "shell" -> "执行命令"
    "file_change" -> "修改文件"
    "apply_patch" -> "应用补丁"
    "permissions" -> "提升权限"
    "write_file" -> "写入文件"
    // 【2026-10-04 裁决(pa_audit unknown_ui)】空白回退"未命名操作"(原"未知操作"):
    // 语义是"上游没给名字",非"系统不知道这是什么";改词不改行为,诚实回退保留
    // —— 不伪造工具名(AUDIT §3.3:禁显"未知/错误"字样)。
    else -> name.ifBlank { "未命名操作" }
}

/**
 * daemon 审批请求 → PaApprovalDialog 白名单文案(纯函数,单测钉住)。
 *
 * README §审批弹窗状态机白名单:Agent 与会话、操作目的、工作区、操作类型、原始命令。
 * [purpose]/[command] 与原审批卡抽屉( ApprovalSheet )的 subtitle/body 同一套映射
 * —— 只换容器不换信息;shell 的工作区取 args.cwd(shell 自报)优先,调用方再以
 * threadWorkspace 兜底。decision 是一次性的,actionKind 附「仅本次」与按钮
 * 「仅本次允许」同一口径,如实陈述授权范围。
 */
internal data class ApprovalUiCopy(val purpose: String, val actionKind: String, val command: String, val cwd: String)

internal fun approvalUiCopy(action: ApprovalRequest): ApprovalUiCopy {
    val name = action.action.name
    val args = runCatching { JSONObject(action.action.jsonArgs) }.getOrNull()
    val purpose = when (name) {
        "shell" -> "agent 请求执行以下命令"
        "file_change" -> "agent 请求改动以下文件"
        "apply_patch" -> "agent 请求改动以下文件"
        "permissions" -> "agent 请求额外的文件 / 网络权限"
        "request_user_input" -> args?.optString("message").orEmpty().ifBlank { "回答后它才会继续" }
        "mcp_elicitation" -> args?.optString("message").orEmpty().ifBlank { "请确认这次调用" }
        "write_file" -> "agent 请求写入以下文件"
        else -> "agent 请求执行以下操作"
    }
    // 理由/授权范围照原审批卡的正文拼装:能透传就透传,不改写不丢弃
    val reason = args?.optString("reason").orEmpty()
    val purposeFull = if (reason.isNotBlank()) "$purpose\n理由：$reason" else purpose
    val command = buildString {
        when (name) {
            "shell" -> append(args?.optString("command").orEmpty())
            "file_change", "apply_patch" -> {
                val files = args?.optJSONArray("files")
                if (files != null && files.length() > 0) {
                    for (i in 0 until files.length()) appendLine(files.optString(i))
                }
                args?.optInt("count", 0)?.takeIf { it > 0 }?.let { append("共 ").append(it).append(" 个文件") }
            }
            "permissions" -> {
                val perms = args?.optJSONObject("permissions")
                if (perms != null) append(perms.toString(2)) else append(action.action.jsonArgs)
            }
            else -> append(action.action.jsonArgs)
        }
        args?.optString("grantRoot").orEmpty().takeIf { it.isNotBlank() }?.let {
            append("\n会话内授权范围：").append(it)
        }
    }.trim()
    return ApprovalUiCopy(
        purpose = purposeFull,
        actionKind = approvalActionKindLabel(name) + " · 仅本次",
        command = command,
        cwd = args?.optString("cwd").orEmpty(),
    )
}

// ---- 会话页双模式取色小助手 ----
// 【为什么是 getter 而不是参数】PaHacker 显示模式在运行时可切;每个用色点就地取值,
// 重组时自然跟随,不必把模式层层传参。黑客终端保持旧深色盘(显示模式行为不变),
// 常规模式走编辑式令牌(瓷白卡面 + 细线 + 固定浅色盘)。

// 【2026-09-30 三色盘同构】双路径 getter 已坍缩:黑客/深色/浅色的差异全部由
// PaEditorial 三色盘供色(apply(dark,hacker) 就地换值),取色点不再分支 ——
// 显示逻辑同一套,只剩色盘差异。黑客模式的内容质感(等宽/❯/扫描线/tmux 条)
// 仍由 PaHacker 契约与 TerminalBlocks 承担。

/** 通用卡面:瓷白表面(黑客盘=近黑 #0C0D11)。 */
private val PanelCardColor: Color get() = PaEditorial.Surface
/** 通用细线。 */
private val PanelHairlineColor: Color get() = PaEditorial.Hairline
/** 琥珀提示面的描边/文字(离线横幅与待审批入口同族)。 */
private val OfflineBannerOutline: Color get() = PaEditorial.AttentionContainer
private val OfflineBannerText: Color get() = PaEditorial.AttentionText
/** 正文主色。 */
private val ChatInkColor: Color get() = PaEditorial.Ink
/** 次级文字。 */
private val ChatMutedColor: Color get() = PaEditorial.Muted
/** 三级文字。 */
private val ChatSubtleColor: Color get() = PaEditorial.Subtle
/** 语义强调(进行中/可点链接)。 */
private val ChatAccentColor: Color get() = PaEditorial.Primary
/** 语义绿(运行/成功)。 */
private val ChatRunningColor: Color get() = PaEditorial.Running
/** 语义红(未完成/拒绝)。 */
private val ChatDeniedColor: Color get() = PaEditorial.Denied

// =====================================================================================
// 编辑式会话页新增小组件(2026-09-29 UI 整体升级·第二波 2/4)
// 私有于会话页;共享组件(PaHostChip/PaStatusBadge/PaApprovalDialog 等)在 ui/component。
// =====================================================================================

/**
 * 投递徽标(编辑式):图标 + 状态词 + 可选详情,四态措辞见 [editorialDeliveryCopy]。
 * 色:未送达=珊瑚红;已送达=绿;等待=次级灰(进行中的中性,主色只给"当前操作")。
 */
@Composable
private fun EditorialDeliveryBadge(state: String, queuePos: Int) {
    val copy = editorialDeliveryCopy(state, queuePos)
    if (copy.word.isEmpty()) return
    val visual = deliveryVisual(state, queuePos)
    val tint = when {
        copy.failed -> ChatDeniedColor
        state == DELIVERY_STEERED -> ChatRunningColor
        else -> ChatMutedColor
    }
    Row(verticalAlignment = Alignment.CenterVertically) {
        val res = when (visual.icon) {
            dev.pocketagent.app.ui.component.DeliveryIconKind.CLOCK -> AppR.drawable.ic_lucide_clock
            dev.pocketagent.app.ui.component.DeliveryIconKind.CHECK_SINGLE -> AppR.drawable.ic_lucide_check
            dev.pocketagent.app.ui.component.DeliveryIconKind.EXCLAMATION -> AppR.drawable.ic_lucide_triangle_alert
        }
        Icon(painterResource(res), null, tint = tint, modifier = Modifier.size(12.dp))
        Spacer(Modifier.width(4.dp))
        Text(copy.word, fontSize = 10.sp, fontWeight = FontWeight(650), color = tint)
        if (copy.detail.isNotBlank()) {
            Spacer(Modifier.width(4.dp))
            Text(
                "· ${copy.detail}",
                fontSize = 10.sp,
                fontWeight = FontWeight(650),
                color = tint.copy(alpha = 0.85f),
                maxLines = 1,
            )
        }
    }
}

/**
 * 会话身份卡(05 HostStatus 槽):agent 图标 + 「agent · 工作区」+ 四词状态徽章。
 * 待审批时整卡可点 → 打开审批弹窗(卡上只做入口,不放批准/拒绝按钮)。
 */
@Composable
private fun ChatIdentityBadge(
    agentName: String,
    workspaceLabel: String?,
    statusView: EditorialStatusView,
    approvalWaiting: Boolean,
    onOpenApproval: () -> Unit,
    agentIconRes: Int?,
) {
    // 状态圆点(用户口径):蓝=运行中 / 绿=正常(已完成) / 红=故障(未完成) /
    // 琥珀=等待你处理;审批等待时整枚徽章可点,直达审批弹窗。
    val dotColor = when (statusView.status) {
        EditorialStatus.RUNNING -> PaEditorial.Primary
        EditorialStatus.COMPLETED -> PaEditorial.Running
        EditorialStatus.WAITING_FOR_YOU -> PaEditorial.Attention
        EditorialStatus.NOT_COMPLETED -> PaEditorial.Denied
    }
    Row(
        Modifier
            .then(
                if (approvalWaiting) {
                    Modifier
                        .pressable(haptic = true) { onOpenApproval() }
                        .semantics { contentDescription = "打开审批弹窗" }
                } else Modifier
            ),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        if (agentIconRes != null) {
            // 2026-09-30 品牌图标 18→24dp:标题行身份一眼可辨(用户令);原色
            // tint=Unspecified 不变,未知 agent 仍不画图标(不塞通用图标冒充身份)。
            Icon(
                painterResource(agentIconRes), null,
                tint = Color.Unspecified, modifier = Modifier.size(24.dp),
            )
            Spacer(Modifier.width(5.dp))
        }
        Text(
            if (workspaceLabel.isNullOrBlank()) agentName else "$agentName · $workspaceLabel",
            fontSize = 13.sp,
            fontWeight = FontWeight.SemiBold,
            color = PaEditorial.Muted,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
        Spacer(Modifier.width(7.dp))
        Box(
            Modifier
                .size(8.dp)
                .background(dotColor, CircleShape)
                .semantics { contentDescription = "状态:" + statusView.label },
        )
    }
}

/**
 * 断线横幅(12):断线原因/自动重连进度 + 草稿保留说明 + 手动重试。
 * 机制与既有 ConnectionBanner 一致(自动重连在 AgentConnection,重试走宿主 reloadToken),
 * 只换编辑式皮:琥珀浅底 + 细线描边 + 琥珀字(语义双通道:词在,色只是辅助)。
 */
@Composable
private fun ChatOfflineBanner(copy: String, onRetry: () -> Unit) {
    Row(
        Modifier
            .fillMaxWidth()
            .background(PaEditorial.AttentionContainer, RoundedCornerShape(13.dp))
            .border(1.dp, OfflineBannerOutline, RoundedCornerShape(13.dp))
            .padding(horizontal = 14.dp, vertical = 10.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Icon(
            painterResource(AppR.drawable.ic_lucide_refresh_cw), null,
            tint = PaEditorial.Attention, modifier = Modifier.size(16.dp),
        )
        Spacer(Modifier.width(10.dp))
        Column(Modifier.weight(1f)) {
            Text(copy, fontSize = 12.sp, lineHeight = 17.sp, color = OfflineBannerText)
            Text(
                "草稿已保存在此设备，恢复连接后由你决定是否发送",
                fontSize = 11.sp,
                lineHeight = 16.sp,
                color = OfflineBannerText.copy(alpha = 0.85f),
            )
        }
        Text(
            "重试",
            Modifier
                .pressable { onRetry() }
                .padding(horizontal = 8.dp, vertical = 8.dp),
            fontSize = 13.sp,
            fontWeight = FontWeight.SemiBold,
            color = PaEditorial.Primary,
        )
    }
}

/**
 * 待审批入口卡(05 时间线内):琥珀浅底 + 盾牌 + 「等待你确认命令执行」。
 * README 红线的界面落点:入口卡只负责"把你带进弹窗",批准/拒绝只在 PaApprovalDialog。
 */
@Composable
private fun ApprovalEntryCard(onOpen: () -> Unit) {
    Row(
        Modifier
            .fillMaxWidth()
            .pressable(haptic = true) { onOpen() }
            .semantics { contentDescription = "打开审批弹窗：等待你确认命令执行" }
            .background(PaEditorial.AttentionContainer, RoundedCornerShape(13.dp))
            .border(1.dp, OfflineBannerOutline, RoundedCornerShape(13.dp))
            .heightIn(min = 44.dp)
            .padding(horizontal = 14.dp, vertical = 12.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box(
            Modifier
                .size(36.dp)
                // 【三盘同构 2026-09-30】原硬编码浅盘值 #FFF0D7(图标盒)/#AC6A24(盾)/
                // #99621E(副文),深/黑客盘的琥珀底已反相为暗琥珀,浅奶油盒压暗底
                // 是漂移 —— 全部改走色盘:琥珀淡染/Attention/AttentionText。
                .background(PaEditorial.Attention.copy(alpha = 0.12f), RoundedCornerShape(13.dp)),
            contentAlignment = Alignment.Center,
        ) {
            Icon(
                painterResource(AppR.drawable.ic_lucide_shield), null,
                tint = PaEditorial.Attention, modifier = Modifier.size(18.dp),
            )
        }
        Spacer(Modifier.width(10.dp))
        Column(Modifier.weight(1f)) {
            Text(
                "等待你确认命令执行",
                fontSize = 14.sp,
                fontWeight = FontWeight.Bold,
                color = PaEditorial.Ink,
            )
            Text(
                "查看操作范围和命令 · 点击处理",
                fontSize = 11.sp,
                lineHeight = 16.sp,
                color = PaEditorial.AttentionText,
            )
        }
        Icon(
            painterResource(AppR.drawable.ic_lucide_chevron_right), null,
            tint = PaEditorial.Attention, modifier = Modifier.size(18.dp),
        )
    }
}

// ---- 提示条方法名 → 人话(L2 真机挑刺;纯函数,可 JVM 单测)----

/**
 * RPC 方法名 → 人话标题。横幅标题不露出内部方法名;名单外的方法返回 null,
 * 调用方把原文照抄(AUDIT §3.3:能透传就透传,不猜)。
 */
internal fun rpcMethodLabel(method: String): String? = when (method.trim()) {
    "session/resume" -> "恢复会话"
    "session/prompt" -> "发送消息"
    "session/new" -> "新建会话"
    else -> null
}

/**
 * 错误原文拆 (人话标题, 正文)(纯函数,单测钉死 ChatMethodErrorCopyTest)。
 *
 * daemon 报错常把内部方法名放最前:"session/resume\n<id> 失败:"或
 * "session/resume: 具体原因"。拆开后标题给人话,正文一字不动保持错误原文;
 * 首段不是已知方法名 → 返回 null,调用方整条原文照抄,不做任何改写。
 */
internal fun humanizeMethodError(raw: String): Pair<String, String>? {
    val text = raw.trim()
    if (text.isEmpty()) return null
    val nl = text.indexOf('\n')
    val head = (if (nl >= 0) text.substring(0, nl) else text).trim()
    val tail = if (nl >= 0) text.substring(nl + 1).trim() else ""
    // 首行形如 "session/resume" 或 "session/resume: 具体原因"(中英文冒号都认)
    val sep = head.indexOfFirst { it == ':' || it == '：' }
    val method = (if (sep >= 0) head.substring(0, sep) else head).trim()
    val inlineBody = if (sep >= 0) head.substring(sep + 1).trim() else ""
    val label = rpcMethodLabel(method) ?: return null
    val body = listOf(inlineBody, tail).filter { it.isNotBlank() }.joinToString("\n")
    return label to body
}

// ---- 撤回排队(pi 0.87.1 clear_queue 的手机端入口;纯函数集中在此,JVM 单测)----

/**
 * 「撤回排队」按钮的三重门控(纯函数,单测钉死 QueueClearGateTest):
 *
 * 1. agent==pi —— daemon 的 ClearQueuedInputs 只接 pi(非 pi 会话显式
 *    FailedPrecondition,不会假装成功),UI 先拦一层,免得点了必败;
 * 2. queuedCount>0 —— 没有排队就没有可撤回的东西,不给空按钮;
 * 3. daemon 宣告 `queue_clear:pi` 能力位 —— 旧 daemon 没有这个 RPC,
 *    能力位不在就绝不显示假入口(硬红线"无能力不显示假入口")。
 *    大小写/空白不敏感,与 RuntimeStateLogic.runtimeControlCapable 同口径。
 */
internal fun queueClearVisible(agentId: String, queuedCount: Int, capabilities: List<String>): Boolean =
    agentId.trim().equals("pi", ignoreCase = true) &&
        queuedCount > 0 &&
        capabilities.any { it.trim().equals("queue_clear:pi", ignoreCase = true) }

/**
 * 撤回成功的人话回执(纯函数)。条数口径与 pi 队列一致:
 * steered=插队(本轮还没跑完就会消费的那批)、follow_up=排队(本轮结束后依次执行)。
 * 两类全零如实说"本来就没有",不谎报撤回条数 —— pi 对空队列 clear 也返回成功。
 */
internal fun queueClearedNotice(steered: Int, followUp: Int): String {
    val s = steered.coerceAtLeast(0)
    val f = followUp.coerceAtLeast(0)
    return when {
        s + f <= 0 -> "队列里已经没有在排队的输入"
        s > 0 && f > 0 -> "已撤回 ${s + f} 条（插队 $s、排队 $f）"
        f > 0 -> "已撤回 $f 条排队输入"
        else -> "已撤回 $s 条插队输入"
    }
}

/**
 * 撤回失败的人话壳(纯函数):错误原文一字不动带出来(daemon 的业务失败原因
 * 本来就是中文人话,改写只会丢信息),只补一句"撤回排队失败"说明动作没成。
 */
internal fun queueClearFailureMessage(error: Throwable): String {
    val raw = error.message?.ifBlank { null } ?: error::class.java.simpleName
    return "撤回排队失败：$raw"
}

// ---- 会话级动作的能力位门控(daemon GetHostInfo.capabilities;纯函数,JVM 单测)----
// 【背景】压缩/分叉/回滚/HTML 导出在四家 agent 上的支持面**不对等**(daemon
// 2026-09-25 能力位对齐,基线 511269d:host.go 按 provider 接口逐 agent 宣告
// compact/fork/rollback/review/export_html:<id>):手机端这些入口此前无位可查,
// 只能按 agent 名单硬编码或全量显示 —— dsh 会话里"分叉/回滚"点了必败。本轮起
// 能力位是唯一门控数据源:位在才亮入口,缺位=隐藏,不摆点了必败的假入口
// (与上方 queueClearVisible 同口径;判定纯函数 agentCapableOf 见 SlashCommandsLogic.kt)。

/** 「压缩上下文」入口(更多面板 + /压缩 伪命令):compact:<agentId> 位。 */
internal fun compactActionOn(agentId: String?, capabilities: List<String>): Boolean =
    agentCapableOf("compact", agentId, capabilities)

/** 「分叉会话」入口:fork:<agentId> 位(现仅 codex 有 ForkThread 真实现)。 */
internal fun forkActionOn(agentId: String?, capabilities: List<String>): Boolean =
    agentCapableOf("fork", agentId, capabilities)

/** 「回滚…」入口:rollback:<agentId> 位(现仅 codex 有 RollbackThread 真实现)。 */
internal fun rollbackActionOn(agentId: String?, capabilities: List<String>): Boolean =
    agentCapableOf("rollback", agentId, capabilities)

/**
 * 附件/图片入口的隐藏门(能力位门控):daemon 逐 agent 宣告 `image_input:<id>`
 * (host.go:151,按 agent.ImageInputter 断言逐家如实声明——codex/pi/zcode 带位、
 * dsh 无位),缺位 = 该家发图必败(dsh 对图片块如实报"请改用文本输入"),
 * 附件按钮是手机端唯一的图片入口 —— 缺位会话里隐藏,免得用户选完图必吃失败壳;
 * 带位 agent 行为不变。判定与 compact/fork 同一口径(agentCapableOf,见
 * SlashCommandsLogic.kt:大小写/空白不敏感,agentId 空/空白一律视为缺位)。
 * 【切换记录】此前按"事实"硬编码 dsh(daemon 的 image_input 位尚未生成);
 * 位就绪后(2026-09-27 能力对齐)切为按位判定,与 compact/fork 同一节奏。
 */
internal fun attachmentEntryHiddenFor(agentId: String?, capabilities: List<String>): Boolean =
    !agentCapableOf("image_input", agentId, capabilities)

/**
 * 运行中发送的回执文案(纯函数,两态):`steer:<agentId>` 位在 = daemon 侧真插话
 * (codex 原生 turn/steer、pi 原生插队,本条会进当前轮次),报"已插入运行中任务";
 * 缺位 = daemon 按排队兜底(当前轮结束后自动续跑),报"已排队"。
 * 【为什么不报"将插入或排队"】能力位已经说清了发送语义,二选一如实报;
 * 最终落点仍以时间线的投递小标签为准(daemon 应答裁定),Toast 只报提交口径。
 */
internal fun steerSubmitNotice(steerCapable: Boolean): String =
    if (steerCapable) "已插入运行中任务" else "已排队"

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
 * 动态思考档位的中文标签(daemon 经 ModelInfo.available_efforts 透出的取值,
 * 主要是 pi 的档位全集 off/minimal/low/medium/high/xhigh/max)。
 * 未知档位直接显示原 id:daemon 以后新增取值时 UI 不至于显示空白。
 */
internal fun effortLabelOf(id: String): String = when (id) {
    "" -> "默认"
    "off" -> "关"
    "minimal" -> "极低"
    "low" -> "低"
    "medium" -> "中"
    "high" -> "高"
    "xhigh" -> "极高"
    "max" -> "最大"
    else -> id
}

/**
 * 思考强度档位选择(纯函数,单测钉死 —— 见 ModelEffortOptionsTest):
 *
 * 1. 选中的模型带动态档位(available_efforts 非空,daemon 只给 pi 填)→ 按它渲染;
 * 2. "跟随桌面配置"(模型为空)且会话归属的 agent 有带档位的模型(如 pi)→
 *    跟随该 agent 的档位(此时用户还没选具体模型,但档位语义已随 agent 确定);
 * 3. 其余(codex/zcode 等无动态档位)→ 现有内置默认档 [EFFORTS]。
 *
 * 【为什么首位恒补"默认"(空 id)】动态集合(pi)没有这个取值,不补的话用户一旦
 * 选了动态档位,就再也回不到"跟随桌面"这个初始态。
 */
internal fun effortsFor(
    models: List<ModelInfo>,
    selectedModelId: String,
    selectedAgent: String,
): List<EffortOption> {
    val modelId = selectedModelId.ifEmpty { "$selectedAgent/default" }
    val dynamic = models.firstOrNull { it.id == modelId && it.provider == selectedAgent }?.availableEffortsList.orEmpty()
    if (dynamic.isNotEmpty()) {
        return listOf(EffortOption("", "默认")) + dynamic.distinct().map { EffortOption(it, effortLabelOf(it)) }
    }
    // 【2026-10-02 修正】选中模型无动态档位时,分两种含义处理(注释与实现此前
    // 不符,强度选择对 codex/zcode/dsh/hermes 实际瘫痪——真机实证):
    //   a) 档位制 agent(pi,同 agent 有**任一**模型带档位)的模型没带档位:
    //      - 未知/私有模型 → 只显示"默认",不伪造档位(RuntimeStateRegressionTest);
    //      - "跟随桌面"(模型空串)→ 跟随该 agent 的档位(首个带档位模型的集合,
    //        此时用户还没选具体模型,档位语义已随 agent 确定)。
    //   b) 该 agent **从不**透出档位(codex/zcode/dsh/hermes)→ 回内置 [EFFORTS]
    //      (codex 的取值与 turn/start effort 一致)。
    val agentIsEffortBacked = models.any { it.provider == selectedAgent && it.availableEffortsCount > 0 }
    if (!agentIsEffortBacked) return EFFORTS
    if (selectedModelId.isNotBlank() && selectedModelId != "$selectedAgent/default") {
        return listOf(EffortOption("", "默认"))
    }
    val agentLevels = models.firstOrNull { it.provider == selectedAgent && it.availableEffortsCount > 0 }
        ?.availableEffortsList.orEmpty()
    return listOf(EffortOption("", "默认")) + agentLevels.distinct().map { EffortOption(it, effortLabelOf(it)) }
}

/**
 * 【已删除 MODEL_NAME_INLINE_MAX】
 * 旧逻辑:只在模型名超过 14 字符时才在输入框上方补一行全宽显示 ——
 * "刚好没超过、但已被工具行挤断"的名字依旧读不全(用户反馈"模型名仍未完整显示")。
 * 现在这一行**恒显且不限行数**,不再需要阈值常量。
 */

/**
 * 输入框内容超过该字符数时,给"展开编辑"图标轻微高亮(PaEditorial.Primary),
 * 提示用户可点开大窗编辑长文本(见输入栏展开图标)。
 */
private const val EXPAND_HINT_THRESHOLD = 80

/**
 * 附件 chip 里文件名的中间省略上限(见 [middleEllipsize])。
 * 取值远大于常见文件名,保证正常名字完整可读(配合最多 3 行换行);
 * 只有病态长名才退化为"保留扩展名的中间省略"。
 */
private const val ATTACH_CHIP_NAME_MAX = 80

/**
 * 【PA-REAL-0004 回归常量】输入区可点控件的最小触控箱边长(dp)。
 * Material 无障碍规范最小交互尺寸 = 48dp;440dpi(换算比 2.75)下折算 132px ——
 * uiautomator dump 里发送/语音/附件/"切换模型与思考强度"四节点的 bounds
 * 两条边都必须 ≥ 132px(复核清单见缺陷单回归结果)。
 * 视觉图标保持 18–20dp 不变,只放大不可见的触控/命中区域;布局上这四个
 * 控件都在 Column 的独立行里,箱子放大不挤压输入框。
 * 【T-D2 收尾(2026-09-20)】口径扩到输入区全部可点控件:Git 按钮、优化按钮、
 * 上下文环(展示型,环视觉 28dp 不变)、展开编辑、审批策略 chip、附件 chip 移除叉。
 * JVM 断言见 ChatInputTouchTargetTest(Compose 布局 JVM 测不了,常量是能钉的部分)。
 */
internal const val INPUT_TOUCH_TARGET_DP = 48

/**
 * 会话页。
 *
 * 需要 ExperimentalLayoutApi 的 opt-in,只为一处:
 * `WindowInsets.ime.union(WindowInsets.navigationBars)` —— 输入区要取"键盘与
 * 导航栏中较高的那个"而不是两者相加。自己手写等价逻辑反而更脆(要处理 insets
 * 消费语义),所以直接用官方 union。
 */
/**
 * 【F-D 历史按轮加载】打开会话路径的 GetThreadHistory 请求:带 rounds 让 daemon
 * 只回最近 N 轮(一轮 = 一条 user 起始;0 = 全量,本函数不会产出 0)。
 *
 * 【分工纪律】只有"打开会话/加载更早"走这里。导出(ChatScreen 导出面板)、
 * 断流恢复与 resync 回填(TurnSession)各自原地构建 limit=0 的全量请求,
 * 恢复判定依赖消息数增量,绝不能被档位截断。
 *
 * 【F-C 接入点】rounds 字段(proto #3,daemon 先按 limit 尾截再按 rounds 收窄)
 * 已就绪;若后续构建报 setRounds 未解析(字段被回退),过渡方案是改为
 * `.setLimit(rounds * 3)`(daemon 的 limit 按"消息条数"截断,一轮≈3 条)。
 */
private fun openThreadHistoryRequest(threadId: String, rounds: Int): GetThreadHistoryRequest =
    GetThreadHistoryRequest.newBuilder()
        .setThreadId(threadId)
        .setRounds(rounds)
        .build()

@OptIn(
    androidx.compose.foundation.layout.ExperimentalLayoutApi::class,
    ExperimentalMaterial3Api::class,
)
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
    /**
     * 【P3 Task 8 第一层分层】页面级长期状态(离线快照/历史加载/身份索引)的
     * 持有者,由入口包装 [dev.pocketagent.app.ui.chat.ChatRoute] 构造传入;
     * 直连调用(既有导航点)留空时在此兜底自建 —— 对外调用点零改动。
     */
    chat: ChatViewModel? = null,
) {
    val chatVm = chat ?: remember(threadId) { ChatViewModel() }
    // 页面级长期状态单点 collect(lifecycle 形态与既有 collectAsState 一致);
    // 下方以同名只读投影接回,全部读点零改动,写入一律走 chatVm.send 意图。
    val chatState by chatVm.state.collectAsState()
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    // 轮次状态在 TurnSession 单例:离开页面流不中断,回来接着看
    val st = remember(threadId) { TurnSession.state(threadId) }
    var input by st::draft
    // ---- 输入框修订号(P4 Task 2 前置小改)----
    // 内容每变一次 +1:优化预览的防覆盖闸拿它对比"请求发起时刻的修订号"
    // (PromptOptimizePreviewState.inputModifiedSince),等待期间改过输入就不许
    // 静默覆盖。所有写 input 的路径(语音并入/斜杠/引用/发送清空/大窗编辑/
    // 优化应用)统一走 setInput,防漏计;随 threadId 重置(草稿每会话一份,
    // 旧会话的修订号没有跨会话意义)。
    var inputRevision by remember(threadId) { mutableStateOf(0L) }
    fun setInput(value: String) {
        if (value != input) {
            input = value
            inputRevision++
        }
    }
    // 语义触觉(审计 P1-5)的宿主 view:PaHaptics 统一入口以 view 取 context 并兜底
    val hapticView = LocalView.current

    // ---- 语义触觉:任务完成(P1-5)----
    // 【为什么在视图层监听而不是改 TurnSession】终局收尾散在状态机多处(实时流/
    // 回放/恢复),视图层只认" running→false 且 turnTerminal 且无错误"这一个
    // 用户可感的终态,一处监听覆盖全部收尾路径,不碰状态机(禁改纪律)。
    // errorMessage==null = COMPLETED 帧(ok=true);FAILED 帧不响"完成"触觉。
    var wasTurnRunning by remember(threadId) { mutableStateOf(false) }
    LaunchedEffect(st.running, st.turnTerminal, st.errorMessage) {
        if (wasTurnRunning && !st.running && st.turnTerminal && st.errorMessage == null) {
            PaHaptics.success(hapticView)
        }
        wasTurnRunning = st.running
    }

    // ---- 语义触觉:审批到达(P1-5)----
    // pendingApproval 从无到有的瞬间是"任务停下来了,等你"—— warning 一下;
    // 从一张卡换到另一张(同卡更新)不重复响,只在"没有→有"边界触发。
    var hadPendingApproval by remember(threadId) { mutableStateOf(false) }
    LaunchedEffect(st.pendingApproval) {
        val now = st.pendingApproval != null
        if (now && !hadPendingApproval) PaHaptics.warning(hapticView)
        hadPendingApproval = now
    }

    // ---- 提示词优化(实验功能,开关在 设置-外观与阅读,默认关)----
    // 【P4 Task 2 升级】按钮 → 仓库(RPC)→ 预览弹层:结果不再直接替换输入框,
    // 而是进 PromptOptimizePreviewState 弹层,由用户确认(可编辑候选/防覆盖
    // 二选一/UndoBar 撤销)。按钮显隐仍走 T1 的纯逻辑状态机(PromptOptimizeLogic.kt);
    // 预览状态机(PreviewState + reducePromptOptimizePreview)管弹层。
    // 撤销复用 app 级 UndoBar:应用成功留 5 秒「已优化提示词[撤销]」,撤销只
    // 恢复本次应用前的文本。
    val promptOptUndo = rememberUndoState()
    var promptOptState by remember { mutableStateOf(PromptOptimizeState()) }
    val promptOptRepository = remember { GrpcPromptOptimizeRepository.fromAgentConnection() }
    // 在途请求句柄与代数:新请求顶掉旧请求(取消旧协程 + 代数核对丢迟到结果),
    // 快速连点绝不会生成多个并发替换结果。
    var promptOptJob by remember { mutableStateOf<Job?>(null) }
    var promptOptSeq by remember { mutableStateOf(0) }
    // 预览弹层状态(随会话重置:换会话不残留上一会话的候选/错误)
    var promptPreview by remember(threadId) { mutableStateOf(PromptOptimizePreviewState()) }
    // 最近一次选择的优化风格(会话内记忆,重进页面回默认)
    var promptStyle by remember { mutableStateOf(PromptOptimizeStyle.DEFAULT) }
    fun syncPromptOptState(text: String, offline: Boolean) {
        promptOptState = reducePromptOptimize(
            promptOptState,
            PromptOptimizeEvent.InputsChanged(text, AgentConnection.optimizePromptEnabled, offline),
        )
    }

    // ---- 【P4 Task 4/5/6 语音输入】VoiceController 接线(域层 ui/voice 六件套) ----
    // 旧页面胶水(voiceUi 状态机/墙钟看门狗/识别器装配/指令 Channel)整体摘除:状态机在
    // VoiceSessionReducer、8 秒死区看门狗(monotonic,不再依赖墙钟)在 VoiceController、
    // 引擎适配在 SystemVoiceEngine/SenseVoiceEngine —— ChatScreen 只做三件事:
    // 装配引擎、转发事件(Start/Stop/Permission/SessionSwitched)、消费展示态。
    // 生命周期不依赖 Compose 重组:页面隐藏走 onPageHidden,永久销毁走 release。
    val voiceController = remember { VoiceController(scope = scope, initialThreadId = threadId) }
    val voiceState by voiceController.state.collectAsState()

    // ---- 【语音交互动画】两个一次性触发器(纯 UI,不进状态机) ----
    // 【动画 ①】麦克风按下涟漪:每次点按 +1,组合层据此重放一圈"扩散 + 淡出"的圆环。
    var micRippleTick by remember { mutableStateOf(0) }
    // 【动画 ③】识别文本落框:Final 消费并入输入框时 +1,驱动输入框一次轻微的"落定"脉动,
    // 配合输入区的 animateContentSize 构成"文本飞入输入框"的过渡感。
    var voiceLandedTick by remember { mutableStateOf(0) }

    // ---- 【端侧兜底】SenseVoice 内置模型:进页幂等初始化(解压校验在 IO),状态驱动引擎选择 ----
    // 系统识别可用性只查一次(部分 ROM 预检就是 false,真机无识别服务 + 无 GMS 云识别)。
    LaunchedEffect(Unit) {
        SenseVoiceModelManager.init(ctx.applicationContext)
    }
    val senseModelState by SenseVoiceModelManager.state.collectAsState()
    val senseReady = senseModelState.ready
    val sysVoiceAvailable = remember {
        runCatching { SpeechRecognizer.isRecognitionAvailable(ctx) }.getOrDefault(false)
    }
    // 系统引擎弃用标记(旧 systemVoiceFailed 的领域化):置位后随下方键翻转改绑端侧。
    var systemVoiceRetired by remember { mutableStateOf(false) }
    // 当前绑定的系统引擎实例:弃用策略要读它的 roundFailed(本轮致命失败)标记。
    var activeSystemEngine by remember { mutableStateOf<SystemVoiceEngine?>(null) }

    // 引擎装配(决策表 pickVoiceEngine 钉住"系统优先 → SenseVoice 兜底"优先级):
    //  · 系统可用且未弃用 → SystemVoiceEngine(工厂预检 + on-device 优先;预检可用但
    //    创建失败时同样落回端侧,保持旧 createVoiceRecognizer 的回退语义);
    //  · 否则 SenseVoice 就绪 → SenseVoiceEngine(内置模型;senseReady 翻转 = 首次解压
    //    校验完成,识别器随键重绑,"初始化完自动可用");
    //  · 都不行 → null(点击麦克风给人话提示,模型就绪后自动重绑)。
    LaunchedEffect(sysVoiceAvailable, systemVoiceRetired, senseReady) {
        val engine = when (pickVoiceEngine(sysVoiceAvailable && !systemVoiceRetired, senseReady)) {
            VoiceEngineKind.SYSTEM_ON_DEVICE, VoiceEngineKind.SYSTEM_REMOTE -> {
                val sys = SystemVoiceEngine.create(ctx)
                activeSystemEngine = sys
                sys ?: SenseVoiceEngine.create()
            }
            VoiceEngineKind.SENSEVOICE -> {
                activeSystemEngine = null
                SenseVoiceEngine.create()
            }
            VoiceEngineKind.UNAVAILABLE -> {
                activeSystemEngine = null
                null
            }
        }
        Log.d(
            "PaVoice",
            "装配语音引擎 retired=$systemVoiceRetired senseReady=$senseReady kind=${engine?.kind}",
        )
        voiceController.bindEngine(engine)
    }
    // 引擎弃用策略:系统引擎本轮致命失败(roundFailed:弃用名单错误码/start 同步异常)
    // 或系统引擎死区(8 秒零回调)→ 置弃用标记,上方键翻转自动重绑为端侧,不再原地反复
    // 失败。端侧失败不弃用(重建要重新冷加载);绑定端侧时 activeSystemEngine=null 自然跳过。
    LaunchedEffect(voiceState.phase, voiceState.error, voiceState.sessionId) {
        val sys = activeSystemEngine ?: return@LaunchedEffect
        val deadZoneOnSystem = voiceState.error == DomainVoiceErrorKind.DEAD_ZONE
        if (voiceState.phase == VoicePhase.ERROR && !systemVoiceRetired &&
            (sys.roundFailed || deadZoneOnSystem)
        ) {
            Log.d(
                "PaVoice",
                "系统识别本轮失败,弃用改选端侧 fatal=${sys.roundFailed} deadZone=$deadZoneOnSystem",
            )
            systemVoiceRetired = true
        }
    }

    // 通知监控登记(P1-6):轮次只会从会话页发起,进页就把本会话交给通知监控,
    // 之后退出页面轮次照跑,完成/审批时才有 threadId 可发通知。
    LaunchedEffect(threadId) { PaNotifications.observeThread(threadId) }

    // 页面生命周期(Global Constraint:离开页面必须明确停止或静音):宿主 Activity ON_STOP
    // → controller.onPageHidden(),Stop 语义 = 收音中停收音并交付本会话最终稿、等权限中
    // 取消请求。永久销毁(旋转/离开组合)→ release():撤看门狗、销毁引擎、状态归零。
    val hostVoiceLifecycle = (ctx as? ComponentActivity)?.lifecycle
    DisposableEffect(voiceController) {
        val voiceLifecycleObserver = LifecycleEventObserver { _, event ->
            if (event == Lifecycle.Event.ON_STOP) voiceController.onPageHidden()
        }
        hostVoiceLifecycle?.addObserver(voiceLifecycleObserver)
        onDispose {
            hostVoiceLifecycle?.removeObserver(voiceLifecycleObserver)
            voiceController.release()
        }
    }

    // 切会话即停录:静音门 + 清草稿在 reducer(SessionSwitched),残余 Final 被静音门 +
    // sessionId 双门丢弃,绝不落新会话。首进页面不算"切换"(voiceHomeThreadId 钉住组合首见)。
    val voiceHomeThreadId = remember { threadId }
    LaunchedEffect(threadId) {
        if (threadId != voiceHomeThreadId) {
            Log.d("PaVoice", "切会话 $voiceHomeThreadId → $threadId,语音停录+静音")
            voiceController.send(VoiceEvent.SessionSwitched(threadId))
        }
    }

    // 初始化失败态(如首次解压被磁盘满/进程被杀打断)给一次重试机会:prepareBundledModel
    // 幂等(有 busy 挡板),本次点击弹原因,下次点击可能已就绪。
    // (声明在 micPermission 之前:Kotlin 局部函数引用不了其后声明的局部变量。)
    fun reportVoiceUnavailable() {
        if (!senseModelState.preparing && senseModelState.error != null) {
            SenseVoiceModelManager.prepareBundledModel()
        }
        val message = when {
            senseModelState.preparing -> "App 内置语音正在初始化，请稍候"
            senseModelState.error != null -> "端侧语音初始化失败：${senseModelState.error}（已自动重试，稍候再点一次）"
            // ready=false 又无 preparing/error:理论上的瞬态(初始化还没跑完),不再谎报
            // "设备不支持",改成引导重试
            else -> "语音引擎尚未就绪，请稍候再点一次；若持续出现请重启 App"
        }
        Toast.makeText(ctx, message, Toast.LENGTH_SHORT).show()
    }

    // 一次性权限申请:拒绝后 reducer 落 ERROR+INSUFFICIENT_PERMISSIONS(人话提示走下方
    // 错误观察器),下次点击可再次申请。授权但引擎没建起来(端侧初始化中/失败):撤掉等
    // 权限会话并如实报真实原因(reportVoiceUnavailable,可自愈重试),不报"设备不支持"
    // 的误导话 —— 与旧实现的"重复提示修复"口径一致。
    val micPermission = rememberLauncherForActivityResult(ActivityResultContracts.RequestPermission()) { granted ->
        Log.d("PaVoice", "权限回答 granted=$granted 引擎=${voiceController.current().engine}")
        if (granted && voiceController.current().engine == VoiceEngineKind.UNAVAILABLE) {
            voiceController.send(VoiceEvent.Cancel)
            reportVoiceUnavailable()
        } else {
            voiceController.send(VoiceEvent.Permission(granted))
        }
    }

    // ---- 语音输入交互(Task 6,最小接线) ----
    // 【裁定】识别完成默认【追加】到输入框(空格衔接,不覆盖已有内容);长按麦克风切换
    // 【替换】模式(开关式,图标着色 + 读屏描述 + Toast 同步反馈)。两种模式都不改写
    // 原文的大小写与标点。识别文本只并入输入框(走 setInput,修订号同车),绝不自动发送。
    var voiceReplaceMode by remember { mutableStateOf(false) }
    // 每轮 Final 只消费一次(以 sessionId 防重:finalText 在下一轮 Start 前一直驻留域状态)。
    var voiceConsumedFinalForSid by remember { mutableStateOf(0L) }
    LaunchedEffect(voiceState.phase, voiceState.sessionId) {
        val spoken = voiceState.finalText
        if (voiceState.phase == VoicePhase.IDLE && spoken.isNotEmpty() &&
            voiceConsumedFinalForSid != voiceState.sessionId
        ) {
            voiceConsumedFinalForSid = voiceState.sessionId
            setInput(voiceApplyToInput(input, spoken, replace = voiceReplaceMode))
            voiceLandedTick++ // 【动画 ③】文本落框触发输入框"落定"脉动(见输入区)
        }
    }
    // 错误人话提示:ERROR 相的错误分类 → 一次性 Toast。文案沿用旧口径(权限/MIUI「录音」
    // 开关指引等),经新枚举 voiceErrorText 重载映射;下一轮 Start 清错误后同类错误可再报。
    LaunchedEffect(voiceState.phase, voiceState.error) {
        if (voiceState.phase == VoicePhase.ERROR && voiceState.error != null) {
            Toast.makeText(ctx, voiceErrorText(voiceState.error!!), Toast.LENGTH_SHORT).show()
        }
    }
    // 录音时长(读屏反馈用):收音相每秒 +1;停止/转写保持最后一读,下一轮清零。
    var voiceElapsedSec by remember { mutableStateOf(0) }
    LaunchedEffect(voiceState.phase) {
        if (voiceState.phase == VoicePhase.STARTING || voiceState.phase == VoicePhase.LISTENING) {
            voiceElapsedSec = 0
            while (true) {
                delay(1_000)
                voiceElapsedSec++
            }
        }
    }
    // 展示态派生:会话进行中(波形/占位符/按钮语义)。STARTING 起就点亮(旧路径点击即亮),
    // PAUSED 也算进行中(再点一次=恢复),IDLE/ERROR 是真正的空闲;权限申请中不点亮。
    val voiceListening = when (voiceState.phase) {
        VoicePhase.STARTING, VoicePhase.LISTENING, VoicePhase.PAUSED,
        VoicePhase.STOPPING, VoicePhase.TRANSCRIBING,
        -> true
        else -> false
    }

    var showModelCard by remember { mutableStateOf(false) }
    // 【历史消息快速查看】左缘刻度条点开/关闭的浮层(视频样板交互)。
    // rememberSaveable:进程重建(切主题/配置变化)后面板开合态不丢。
    var showHistoryPanel by rememberSaveable { mutableStateOf(false) }
    // 【模型快速切换面板】模型行下拉箭头直接展开的轻面板(单点即切,
    // 不再强制进 ModelEffortCard 二级弹窗;"更多…"仍可进完整选择卡)。
    var showQuickModels by remember { mutableStateOf(false) }
    // 切换成功后的行内反馈 tick:驱动模型名一次轻量闪光(真实选择才有,不造假状态)
    var modelFlashTick by remember { mutableStateOf(0) }
    var showContextCard by remember { mutableStateOf(false) }
    var showDrawer by remember { mutableStateOf(false) }
    // 侧边栏跟手:手势与面板共用同一个进度(0..1),手指在哪面板就在哪
    val drawerCtrl = dev.pocketagent.app.ui.component.rememberDrawerController()
    var showNewThread by remember { mutableStateOf(false) }
    var showGitPanel by remember { mutableStateOf(false) }
    var showExport by remember { mutableStateOf(false) }
    // 子代理卡长按快览(T-SUB):非 null 时弹快览层(渲染在文件中部 sheet 区),
    // 收起不影响任何状态 —— 蓝本 §4.3"不打断主对话浏览的偷看"。
    var peekSub by remember(threadId) { mutableStateOf<SubAgentThreadMeta?>(null) }
    // 「更多」面板:分叉 / 回滚。这两件事都是**偶尔做一次**的会话级动作,
    // 单独给顶栏加两个常驻按钮会让它变挤,收进一个入口更稳。
    var showMore by remember { mutableStateOf(false) }
    // 【内置伪命令"/目标"的弹窗开关】斜杠卡面伪命令点选 = 执行动作:/目标
    // 打开下方的设置目标底栏(slashGoalSheet 块),/压缩 直接走 doCompact(),
    // /计划(review:<agentId> 位会话,现仅 codex)直接走 doStartReview()。
    var slashGoalSheet by remember { mutableStateOf(false) }
    // 回滚选择(二级面板):先选丢几个回合,再确认
    var rollbackOpen by remember { mutableStateOf(false) }
    var rollbackBusy by remember { mutableStateOf(false) }
    // 手动压缩是低频的会话级动作,与回滚一样放在“更多”里。
    var compactBusy by remember { mutableStateOf(false) }
    // 分叉(从最新状态 fork 新会话):防重入标记,与 rollbackBusy/compactBusy 同款 ——
    // RPC 在途时再点只会重复发 forkThread、弹重复 Toast,闸掉。
    var forkBusy by remember { mutableStateOf(false) }
    // "/计划"发起计划审查(review:<agentId> 位会话,现仅 codex):防重入标记,与 compactBusy 同款。
    var reviewBusy by remember { mutableStateOf(false) }
    // 【长文本大窗编辑】输入框最多 4 行,写长需求/贴代码时不够用。
    // 打开一个整屏可用高度的编辑面板,内容与输入框双向同一份 state。
    var expandedInputOpen by remember { mutableStateOf(false) }
    // 【执行计划面板】任务完成后计划卡不再散落在时间线里,而是收进会话上方
    // 一个可展开/收起的面板:默认收起(顶部只留一行进度摘要),点开看完整步骤。
    var planPanelOpen by remember { mutableStateOf(false) }
    // 最近一次计划的步骤(时间线里落定的 + 正在流式的,取最新的一份)
    val latestPlan = remember(st.items, st.planSteps) {
        st.items.lastOrNull { it.role == "plan" && it.planSteps.isNotEmpty() }?.planSteps
            ?: st.planSteps
    }
    // 【默认展开】计划一出现就展开 —— 对应"未完成时要保持在会话中显示"。
    // 键用 Boolean 而不是步骤列表:只在"从无到有"时触发一次,
    // 之后用户手动收起不会被反复弹开。
    LaunchedEffect(latestPlan.isNotEmpty()) {
        if (latestPlan.isNotEmpty()) planPanelOpen = true
    }
    // 侧边栏用的最近会话:和模型/设置一起预取,打开抽屉时直接就是数据,不必等网络
    var recentThreads by remember { mutableStateOf(listOf<PThread>()) }
    var recentLoading by remember { mutableStateOf(true) }

    // 【返回键】分层回退:面板/抽屉打开时先收面板,否则回会话列表(而不是直接退到桌面)。
    // 必须放在这些状态声明之后 —— 放在函数开头会读不到下面声明的 showDrawer 等。
    BackHandler(enabled = true) {
        when {
            drawerCtrl.visible -> drawerCtrl.close()
            showHistoryPanel -> showHistoryPanel = false
            rollbackOpen -> rollbackOpen = false
            showMore -> showMore = false
            showExport -> showExport = false
            showGitPanel -> showGitPanel = false
            showContextCard -> showContextCard = false
            showModelCard -> showModelCard = false
            // 计划面板展开时,返回先收起面板(它是最贴近"当前屏幕状态"的一层)
            planPanelOpen -> planPanelOpen = false
            else -> onBack()
        }
    }
    var attachments by remember { mutableStateOf(listOf<Attachment>()) }
    var selectedAttachment by remember { mutableStateOf<Attachment?>(null) }
    // 【PF4】按 threadId 记忆:导航 launchSingleTop 复用同一 ChatScreen 实例,
    // 会话切换必须重置本会话的 daemon 回填值,不能闪现上一会话的残值。
    var threadWorkspace by remember(threadId) { mutableStateOf<String?>(null) }
    var codexSettings by remember { mutableStateOf<CodexSettings?>(null) }
    var models by remember { mutableStateOf(listOf<ModelInfo>()) }
    var loadError by remember { mutableStateOf<String?>(null) }
    // 【P3 Task 8】离线快照标志已收进 ChatViewModel(ChatState)—— 此处为只读投影,
    // 写入走 chatVm.send(见离线横幅/错误条重试/getThread 效果各写点)。
    val offlineSnapshot = chatState.offlineSnapshot
    // 离线快照的落盘时间(横幅「离线快照 · HH:mm」用;0 = 无)
    val offlineSnapshotAt = chatState.offlineSnapshotAt
    var reloadToken by remember { mutableStateOf(0) }
    // 侧边栏品牌行的电脑名(与其他页面一致)
    var hostName by remember { mutableStateOf("") }
    val listState = rememberLazyListState()
    val connectionEpoch = AgentConnection.connectionEpoch.collectAsState().value
    // 【P3 Task 8】以下历史加载/身份索引状态已收进 ChatViewModel(ChatState),此处为
    // 只读投影 —— 写入一律走 chatVm.send(...) 意图(见各写点),读点零改动。
    val historyRoundsLoaded = chatState.roundsLoaded
    val historyTruncated = chatState.truncated
    val loadingEarlier = chatState.loadingEarlier
    val historyIdentity = chatState.identity
    val historyOldestMessageId = chatState.oldestMessageId

    /**
     * ThreadMessage 列表 → (ChatItem 序列, 消息级身份跨度)。历史映射一条
     * ThreadMessage ≥ 1 个 ChatItem(块拆分,见 HistoryMessageMapper),跨度
     * 记下每条消息映射出的连续段,搜索命中才能按 message_id 找回时间线下标。
     */
    fun buildHistoryBatch(messages: List<ThreadMessage>): Pair<List<ChatItem>, List<MessageSpan>> {
        val items = mutableListOf<ChatItem>()
        val spans = mutableListOf<MessageSpan>()
        for ((i, m) in messages.withIndex()) {
            val start = items.size
            items += historyToChatItems(listOf(m))
            spans += MessageSpan(
                messageId = m.messageId,
                turnId = m.turnId,
                sequenceInResponse = i,
                itemStart = start,
                itemCount = items.size - start,
                role = m.role,
            )
        }
        return items to spans
    }

    /**
     * 【Task 7 加载更早升级】历史片段落地:按「已加载最旧 message id」锚点只把
     * 回包头部的新消息**拼到列表头部**,不做整列表替换 —— daemon 幂等回尾部,
     * 回包 = 更早新增段 + 已加载部分;锚点之前的那段才是新增。已加载条目的引用
     * 不变,LazyColumn 不整表重组;回包没有新消息(重复拉取)时零列表变动。
     * 旧 daemon 回包 message_id 为空时退「回包条数 - 已加载条数」条数兜底。
     *
     * @return 实际新拼入的 ChatItem 条数(调用方的滚动锚点 delta 直接用)。
     */
    fun prependHistoryMessages(messages: List<ThreadMessage>): Int {
        if (messages.isEmpty()) return 0
        // 【P3 Task 8】拼接量裁定与身份索引推进收进 ChatViewModel(纯 reducer);
        // 这里只落地 st.items(对话流资产)并回传新条数(滚动锚点 delta 直接用)。
        val newCount = chatVm.planPrepend(messages.map { it.messageId })
        if (newCount <= 0) return 0
        val (newItems, newSpans) = buildHistoryBatch(messages.take(newCount))
        st.items.addAll(0, newItems)
        chatVm.send(ChatIntent.PrependApplied(newSpans, newItems.size, messages.first().messageId))
        return newItems.size
    }

    // 提示词优化:输入/开关/离线态任一变化都重算按钮可见性(单一事实来源是输入,
    // 状态机只记"请求在飞";纯逻辑见 PromptOptimizeLogic.kt,JVM 单测钉行为)。
    LaunchedEffect(input, offlineSnapshot, AgentConnection.optimizePromptEnabled) {
        syncPromptOptState(input, offlineSnapshot)
    }

    // ---- 斜杠命令面板(zcode/codex/pi 三源)----
    // threadAgentId 在下方 getThread 处赋值;命令列表按会话进程内缓存
    // (SlashCommandCache,见 SlashCommandsLogic.kt),进会话/重连/下拉刷新才重拉。
    // daemon 回空列表 / RPC 失败都安静降级,绝不 Toast 报错打断输入;在线时
    // hostCapabilities 已就位,内置伪命令(随 compact/review 位点亮,见
    // slashCandidatesFor)仍给三源会话兜出可用卡面。
    var threadAgentId by remember(threadId) { mutableStateOf("") } // 【PF4】换会话重置,理由同 threadWorkspace
    // 【能力位数据源】hostCapabilities 共享快照(与主页网格/设置页同一真源):
    // 会话级动作(压缩/分叉/回滚/计划审查/HTML 导出)与运行中发送回执(steer)
    // 的唯一门控数据源 —— 位在才亮入口,缺位不显示假入口(硬红线)。
    // 声明在函数级:send 的回执文案与本页各浮层都要读它。
    val hostCaps = AgentConnection.hostCapabilities.collectAsState().value
    // daemon 落库的 Thread.ModelID(getThread 回填):会话真实生效模型,
    // "pi/zai-coding-cn/glm-5.3" 这种 agent 前缀形式。模型芯片/弹窗的选中态以它
    // 为准(图3 实锤:过去 pi 会话没取到它,链路落到 codex 桌面默认
    // GLM-5.3-Flash 上,芯片显示成了 codex 的模型)。
    var threadModelId by remember(threadId) { mutableStateOf("") } // 【PF4】换会话重置,理由同 threadWorkspace
    var slashCommands by remember(threadId) { mutableStateOf<List<SlashCommandInfo>>(emptyList()) }
    LaunchedEffect(threadId, threadAgentId, connectionEpoch, reloadToken, slashTokenOf(input) != null, st.running) {
        if (!isSlashPanelAgent(threadAgentId)) {
            // 非面板会话(zcode/codex/pi 之外)不显示卡面(即使缓存里有别的会话留下的命令也不漏)
            slashCommands = emptyList()
            return@LaunchedEffect
        }
        val cached = SlashCommandCache.get(threadId)
        if (cached != null) slashCommands = cached
        val loaded = try {
            withContext(Dispatchers.IO) {
                AgentConnection.threadService()
                    .withDeadlineAfter(SLASH_COMMANDS_DEADLINE_S, TimeUnit.SECONDS)
                    .listSlashCommands(
                        ListSlashCommandsRequest.newBuilder().setThreadId(threadId).setRefresh(true).build(),
                    )
                    .commandsList
                    .map { SlashCommandInfo(it.name, it.description, it.inputHint) }
            }
        } catch (_: Exception) {
            emptyList() // 拉不到就没有面板,不是错误
        }
        SlashCommandCache.put(threadId, loaded)
        slashCommands = loaded
    }
    // 「优化」点击(P4 Task 2):走仓库发 RPC,结果进预览弹层等用户确认,不再
    // 直接替换输入框。daemon 侧整段预算 150s,手机端 deadline 再兜余量(仓库的
    // 独立 deadline);失败人话由仓库分类(分类器在 PromptOptimizeLogic),进弹层
    // 错误位。防覆盖契约:发起时记输入框修订号,应用时对比,改过就二选一。
    fun startPromptOptimize(original: String, style: PromptOptimizeStyle) {
        // 【busy 闸】照 doFork 先例:在途时再点直接忽略,不并发发请求、不叠弹层
        // (快速连点绝不生成多个并发替换结果;按钮本身也随 inFlight 隐藏)。
        if (!promptOptimizeClickAllowed(promptPreview, original)) return
        promptOptState = reducePromptOptimize(promptOptState, PromptOptimizeEvent.OptimizeStarted)
        promptPreview = reducePromptOptimizePreview(
            promptPreview,
            PromptOptimizePreviewIntent.Started(original, style, inputRevision),
            inputRevision,
        )
        promptOptJob?.cancel() // 新请求顶掉旧请求:旧协程取消,迟到结果再按代数丢弃
        val seq = ++promptOptSeq
        promptOptJob = scope.launch {
            val result = promptOptRepository.optimize(
                PromptOptimizeRequestModel(
                    original = original,
                    style = style,
                    agentId = AgentConnection.optimizeAgent,
                    modelId = AgentConnection.optimizeModel,
                    effort = "", // 无独立推理档位偏好,空 = daemon 默认档
                    requestVersion = inputRevision,
                ),
            )
            if (seq != promptOptSeq) return@launch // 迟到结果(已被新一轮顶掉):丢弃
            // 按钮状态机回到闲置,并按当前输入重算可见性(失败时输入未变,
            // LaunchedEffect(input) 不会自己重跑,必须手动 sync 一次)
            promptOptState = reducePromptOptimize(
                promptOptState,
                if (result.isSuccess) PromptOptimizeEvent.OptimizeSucceeded else PromptOptimizeEvent.OptimizeFailed,
            )
            syncPromptOptState(input, offlineSnapshot)
            result.fold(
                onSuccess = { candidate ->
                    promptPreview = reducePromptOptimizePreview(
                        promptPreview,
                        PromptOptimizePreviewIntent.Succeeded(candidate),
                        inputRevision,
                    )
                },
                onFailure = { e ->
                    promptPreview = reducePromptOptimizePreview(
                        promptPreview,
                        PromptOptimizePreviewIntent.Failed(e.message ?: "优化失败,原文未被改动"),
                        inputRevision,
                    )
                },
            )
        }
    }

    fun optimizeCurrentInput() = startPromptOptimize(input, promptStyle)

    /** 应用候选到输入框:撤销只恢复**本次**应用前的文本(UndoBar 5 秒窗口)。 */
    fun applyPromptCandidate(overwrite: Boolean) {
        val s = promptPreview
        val candidate = s.candidate?.trim() ?: return
        // 双保险:reducer 的 Apply 闸会拒绝"已改过输入却盲应用";这里同步短路,
        // 避免先清状态再发现不能应用的抖动。Overwrite 是用户在二选一里明选的覆盖。
        if (!overwrite && s.inputModifiedSince(inputRevision)) return
        val restored = input // 撤销恢复值 = 应用前输入框里的文本(覆盖时是新文本)
        promptPreview = reducePromptOptimizePreview(
            promptPreview,
            if (overwrite) PromptOptimizePreviewIntent.OverwriteModified else PromptOptimizePreviewIntent.Apply,
            inputRevision,
        )
        setInput(candidate)
        promptOptUndo.show("已优化提示词", onUndo = { setInput(restored) })
    }

    /**
     * 预览层意图分发:纯意图走 reducer,带副作用的(应用/重试/换风格/取消)落副作用。
     * 重试/换风格统一走 [startPromptOptimize] —— 由它发 Started(规范的加载态迁移),
     * 不先手动归约 Retry:否则 loading 先置真,会被 startPromptOptimize 自己的
     * busy 闸拦下,请求根本发不出去。
     */
    fun dispatchPromptPreview(intent: PromptOptimizePreviewIntent) {
        when (intent) {
            is PromptOptimizePreviewIntent.EditCandidate ->
                promptPreview = reducePromptOptimizePreview(promptPreview, intent, inputRevision)
            is PromptOptimizePreviewIntent.Restyle -> {
                if (!promptPreview.loading) { // 芯片在请求中已禁用,这里再闸一道
                    promptStyle = intent.style
                    startPromptOptimize(promptPreview.original.ifBlank { input }, intent.style)
                }
            }
            PromptOptimizePreviewIntent.Retry ->
                startPromptOptimize(promptPreview.original.ifBlank { input }, promptStyle)
            PromptOptimizePreviewIntent.Apply -> applyPromptCandidate(overwrite = false)
            PromptOptimizePreviewIntent.OverwriteModified -> applyPromptCandidate(overwrite = true)
            PromptOptimizePreviewIntent.Dismiss, PromptOptimizePreviewIntent.Undo -> {
                if (promptPreview.loading) promptOptJob?.cancel() // 关层即停:不再为看不见的请求花配额
                promptPreview = reducePromptOptimizePreview(promptPreview, intent, inputRevision)
                syncPromptOptState(input, offlineSnapshot)
            }
            // 生命周期意图(Started/Succeeded/Failed)正常只由 startPromptOptimize 的
            // 请求流程直接经 reducer 落状态;万一从分发口进来(编程失误),也按同一条
            // 状态机归约降级 —— 不静默吞事件、不留空白分支(Succeeded 带 loading 闸,
            // 已关层的迟到结果会被 reducer 拒收,不会误应用)。
            is PromptOptimizePreviewIntent.Started,
            is PromptOptimizePreviewIntent.Succeeded,
            is PromptOptimizePreviewIntent.Failed,
            ->
                promptPreview = reducePromptOptimizePreview(promptPreview, intent, inputRevision)
        }
    }

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
            // 不覆盖已有提示:离线快照的黄条比这里的红条更有信息量(真机验证发现
            // 两个并行加载共用 loadError,后完成者会把"离线:显示的是本地记录"顶掉)
            if (loadError == null) loadError = "无法加载模型设置：${e.message ?: "连接失败"}"
        }
        try {
            withContext(Dispatchers.IO) {
                models = AgentConnection.modelServiceSlow()
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
        try {
            withContext(Dispatchers.IO) {
                recentThreads = AgentConnection.threadService()
                    .listThreads(
                        ListThreadsRequest.newBuilder()
                            .setIncludeArchived(false)
                            .build())
                    .threadsList
            }
        } catch (_: Exception) {
            // 拉不到就显示空态,不影响当前会话
        }
        recentLoading = false
    }

    // ---- 子代理线程组(T-SUB):打开会话时顺带查一次 thread/list,本会话轮次结束后刷新 ----
    // 子组 = thread/list 按 parentThreadId 过滤(S1 修正案:子代理=真线程);
    // 拉取失败静默沿用上次快照 —— 子代理卡是"有则显示",不能打扰主对话。
    LaunchedEffect(threadId, connectionEpoch, reloadToken) {
        SubAgentsState.refresh()
    }
    // 轮次结束沿(false 沿)刷新:初始组合不再多发一次请求
    val subPrevRunning = remember(threadId) { AtomicBoolean(false) }
    LaunchedEffect(st.running) {
        if (subPrevRunning.getAndSet(st.running) && !st.running) SubAgentsState.refresh()
    }

    // 【P3-T4 接线①】进入会话:ModelSelectionRepository.open(threadId, agentId)
    // —— 先读本地「上次记录」(fromCache 标注)再用 daemon 真值整树替换(仓库恢复链)。
    // 【顺序红线】原"恢复该会话上次选定的模型/强度"的 selectedModel/selectedEffort
    // 重断言段已整体撤除(防双写):选择事实源收敛到仓库一棵状态树,本页不再往
    // AgentConnection.selectedModel/selectedEffort 写任何值 —— 若保留重断言,它会在
    // 仓库真值回填后把旧缓存值写回全局,两路口径互相覆盖(计划 Task 3 收敛点)。
    // 全局值的其它读者(侧边栏/主页/会话列表/新建会话)为波 4 待接线项。
    val modelSelectionState by ModelSelectionRepository.state.collectAsState()
    val modelSelection = ModelSelectionReducer.display(modelSelectionState)
    LaunchedEffect(threadId, threadAgentId) {
        ModelSelectionRepository.open(threadId, threadAgentId)
    }

    // 生效模型(全部按会话归属 agent 取,不再串别家的模型表/默认值):
    //   1) 手机端对该会话的显式选择(threadModel(threadId):null=从未选过 →
    //      退全局默认;""=显式选了"跟随桌面" → 继续往 daemon 真值走);
    //   2) daemon 落库的 Thread.ModelID —— 会话真实生效模型(getThread 回填,
    //      pi 会话是 "pi/<provider>/<model>" 前缀形式);
    //   3) codex 桌面默认 —— 【仅 codex 会话】读 codexSettings:它存的是 codex 的
    //      全局默认模型,pi 会话读它就是把 codex 模型硬编码进 pi 会话
    //      (图3 实锤:pi 会话芯片显示成 codex 的 GLM-5.3-Flash,根因在此);
    //   4) 本轮实际运行 → 会话 agent 的 default 条目 → 该 agent 模型表首个。
    // 模型查找只在 sessionModels(按 agent 过滤,见 modelsForAgent)里进行,
    // 别家的模型名再也不会漏进本会话的芯片/弹窗。
    val sessionAgent = threadAgentId.ifBlank { AgentConnection.selectedAgent }
    val sessionModels = remember(models, sessionAgent) { modelsForAgent(models, sessionAgent) }
    // 【P1-20 · 生命周期门控】可用面调查结论:build.gradle.kts 仅声明
    // lifecycle-viewmodel-compose(无 lifecycle-runtime-compose,repeatOnLifecycle
    // 不可用,且不许新增依赖);activity-compose 在用(BackHandler)→ activity 传递提供
    // lifecycle-runtime,Lifecycle/LifecycleEventObserver 保证可用;ProcessLifecycleOwner
    // 属 lifecycle-process,未声明不采用。故取宿主 Activity 的 Lifecycle 挂观察者:
    // 门控闸 = 一次性 CompletableDeferred,注册观察者后复查状态(消除 START 竞态),
    // finally 必摘观察者 —— 协程随组合销毁取消时闸被取消,观察者照样移除,零泄漏。
    val hostLifecycle = (LocalContext.current as? ComponentActivity)?.lifecycle
    LaunchedEffect(threadId,sessionAgent,connectionEpoch) {
        while(true) {
            // 前台门控:STOP 状态不跑 RPC;挂起等 ON_START,回前台立即补跑再进 10s 节奏
            if (hostLifecycle != null) {
                val gate = CompletableDeferred<Unit>()
                val gateObserver = LifecycleEventObserver { _, event ->
                    if (event == Lifecycle.Event.ON_START) gate.complete(Unit)
                }
                hostLifecycle.addObserver(gateObserver)
                try {
                    if (!hostLifecycle.currentState.isAtLeast(Lifecycle.State.STARTED)) gate.await()
                } finally { hostLifecycle.removeObserver(gateObserver) }
            }
            if(sessionAgent.isNotEmpty() && AgentConnection.connState.value=="ok") {
                try {
                    val native=withContext(Dispatchers.IO) {
                        AgentConnection.modelServiceSlow().listModels(ListModelsRequest.newBuilder()
                            .setAgentId(sessionAgent).setThreadId(threadId).build())
                    }
                    if(native.modelsCount>0) {
                        // 【P1-20 · 先比对再赋值】List equals 按内容逐元素比(proto ModelInfo
                        // 实现 equals):内容没变就不换实例,消除每 10s 一次的空转重组。
                        val merged=models.filter { it.provider!=sessionAgent }+native.modelsList
                        if(merged!=models) models=merged
                    }
                    TurnSession.acceptRuntimeSelection(threadId,native.currentModelId,
                        if(native.hasCurrentEffort()) native.currentEffort else null)
                    // 【P3-T4 接线②】巡检回包喂仓库:仓库没有外部写入通道(交底口径
                    // 只有 open/send/refresh 三入口),按二选一方案改调 repository.refresh()
                    // —— GetThread(24)+ListModels(5) 真值整树替换,选择状态只有仓库
                    // 一个写者,不再双路写。本巡检的既有职责(10s 节奏/前台门控/模型表
                    // 合并/原生选择回填 acceptRuntimeSelection)原样保留。
                    // 【竞态安全】refresh 自带在飞守卫(pending 非空时只更新模型表,
                    // 不覆盖乐观投影);失败只标仓库 error,不影响本巡检。
                    if (ModelSelectionRepository.currentThreadId() == threadId) {
                        ModelSelectionRepository.refresh()
                    }
                } catch(e: kotlinx.coroutines.CancellationException) { throw e }
                catch(_:Exception) { /* Keep last verified runtime state during network recovery. */ }
            }
            delay(10_000)
        }
    }
    val requestedModel=(AgentConnection.threadModel(threadId) ?: threadModelId).takeIf {
        it.isEmpty() || it.startsWith("$sessionAgent/")
    }.orEmpty()
    val followsNative=requestedModel.isBlank() || requestedModel==agentDefaultModelId(sessionAgent)
    val effectiveModelId=if(followsNative) st.runningModel.ifBlank { requestedModel } else requestedModel
    val effectiveModel=sessionModels.find { it.id==effectiveModelId }
        ?: if (effectiveModelId.isBlank() || effectiveModelId==agentDefaultModelId(sessionAgent))
            sessionModels.find { it.id==agentDefaultModelId(sessionAgent) } else null
    // agent 默认模型自报的窗口;没有上报就保持未知。
    val agentDefaultWindow = sessionModels.find { it.id == agentDefaultModelId(sessionAgent) }?.contextWindow ?: 0
    // 窗口解析优先级链:daemon 上报真实窗口 > 当前模型声明窗口 > agent 默认模型声明窗口。
    val reportedWindow = pickContextWindow(
        live = st.liveContextWindow,
        modelDeclared = effectiveModel?.contextWindow ?: 0,
        agentDefaultDeclared = agentDefaultWindow,
    )
    val contextWindow = reportedWindow
    val windowIsFallback = reportedWindow <= 0

    // 【生效模型名】在输入栏前统一计算,供"输入框上方全宽行"、tmux 状态条与
    // "工具行模型 chip"复用。
    // 修复 2b:displayName 可能为空串(daemon 返回的模型未带展示名),空串不能触发
    // `?:` 兜底,必须用 takeIf { isNotBlank() } 先判空,再回退到生效模型 id 去掉
    // agent 前缀的裸名(modelBareName:"pi/glm-5.3" → "glm-5.3")或"跟随桌面"。
    // 这样无论模型列表是否加载完,当前模型名都非空、完整可读。
    // 一律用 model/list 返回的 displayName,不硬编码任何模型名。
    // 【兜底键用 effectiveModelId 而非 selectedModel】生效链会落到 daemon
    // 落库的 Thread.ModelID(pi 会话真实模型),芯片显示跟真实跑的一致。
    val modelDisplayName = effectiveModel?.displayName?.takeIf { it.isNotBlank() }
    val modelName = modelDisplayName
        ?: if (effectiveModelId.isEmpty()) "跟随桌面"
        else modelBareName(effectiveModelId)
    // 内置档没有的取值(pi 动态档位 off/minimal/max 及未来新增)回
    // effortLabelOf 的中文标签,模型 chip 上不至于露出英文原 id。
    val effortLabel = effortLabelOf(AgentConnection.threadEffort(threadId) ?: st.runningEffort.orEmpty())

    // ---- 上下文窗口卡:断网缓存 + 自动刷新 ----
    // ① 每次收到用量更新(含从缓存回填的首帧)都写一份"最后一次已知用量"到本地:
    //    断网/自动断网后,卡片仍能照常显示这些数字,并标出是什么时候的。
    //    【R1 收口 · 落盘节流】hermes 的 usage_update 逐事件投影(密集时一秒数帧),
    //    缓存按 usagePersistCursorAccepts 节流写(3s 间隔;窗口变化/占用大跳除外,
    //    纯函数 UsagePersistThrottleTest 钉死)—— 输入栏实时环与卡面实时值不受
    //    节流影响,st.turnUsage 本身仍是逐帧真实值,这里只管磁盘写频率。
    var usagePersistCursor by remember(threadId) { mutableStateOf<UsagePersistCursor?>(null) }
    LaunchedEffect(threadId, st.turnUsage, st.totalUsage, st.liveContextWindow) {
        UsageCache.init(ctx)
        if (st.turnUsage != null || st.totalUsage != null) {
            val nowMs = System.currentTimeMillis()
            val nextTokens = usageTotalTokens(st.turnUsage)
            if (usagePersistCursorAccepts(usagePersistCursor, nowMs, nextTokens, contextWindow)) {
                UsageCache.save(
                    threadId,
                    UsageSnapshot(
                        contextWindow = contextWindow,
                        windowIsFallback = windowIsFallback,
                        turnUsage = st.turnUsage,
                        totalUsage = st.totalUsage,
                        updatedAt = nowMs,
                    ),
                )
                usagePersistCursor = UsagePersistCursor(nowMs, nextTokens, contextWindow)
            }
        }
    }
    // ② 卡片打开期间每 3s 刷新一次;卡片关闭即随 LaunchedEffect 作用域取消,不会漏取消。
    var usageRefreshTick by remember { mutableStateOf(0) }
    LaunchedEffect(showContextCard) {
        if (!showContextCard) return@LaunchedEffect
        while (true) {
            delay(3000)
            usageRefreshTick++
        }
    }
    // ③ 连接由"非 ok"回到"ok"时立即刷新一次(断网恢复后不必干等下一个 3s);
    //    断网时不做任何重试,失败静默 —— 卡片靠缓存继续显示,不做无谓刷屏。
    // connState 取值:checking / ok / down。**checking 是巡检中间态,不是"已知断网"**:
    // 若按 `!= "ok"` 判离线,连接巡检的瞬间会闪一条假的"离线"提示 —— 与本轮在修的
    // "UI 假装有状态"是同一类病。故:提示条只在 `down` 时出现;`checking` 不显示提示条,
    // 但 4 项数字照样从缓存兜底显示(不出现空态)。
    val connState = AgentConnection.connState.collectAsState().value
    val connDown = connState == "down"
    val connOk = connState == "ok"
    LaunchedEffect(connOk) {
        if (connOk) usageRefreshTick++
    }

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

    /**
     * 推翻审查者(guardian / 自动审查)的驳回结论,要求继续执行。
     *
     * 【为什么失败也要弹提示 —— 而不是静默重试】codex 侧对这条请求的校验很严:
     * 结论一旦过期(daemon 重启、动作已经重发过),它就不认这个 reviewId 了。
     * 这类失败重试没有意义,必须让用户看到"这条审查已经过期,请重新发起动作",
     * 否则他会以为按钮坏了,反复点。
     */
    fun approveGuardian(ev: CodexEvent) {
        scope.launch {
            try {
                withContext(Dispatchers.IO) {
                    AgentConnection.threadService().approveGuardianDeniedAction(
                        ApproveGuardianDeniedActionRequest.newBuilder()
                            .setThreadId(threadId)
                            .setReviewId(ev.reviewId)
                            .setTargetItemId(ev.targetItemId)
                            .build(),
                    )
                }
                // 成功:收起入口并留住"已推翻"的状态(离开页面再回来也不会又冒出按钮)
                TurnSession.markGuardianOverridden(threadId, ev.reviewId)
                Toast.makeText(ctx, "已推翻这次驳回,电脑会继续执行", Toast.LENGTH_SHORT).show()
            } catch (e: Exception) {
                Toast.makeText(ctx, "推翻失败:${e.message}", Toast.LENGTH_LONG).show()
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

    // ---- P0-1 流式滚动锚定(对标 ChatGPT/Claude:距底才跟随,离底出胶囊)----
    // 【为什么放列表容器层】常规气泡与 hacker 终端块共用同一条 LazyColumn,跟随/
    // 未读/胶囊只在容器层做一次,两条渲染路径天然一致;判定全走纯函数
    // (shouldAutoScroll / accumulateUnreadBlocks 等,JVM 单测钉死)。
    var followBottom by remember(threadId) { mutableStateOf(true) }
    // 离底期间累计的未读新块数(胶囊上的 N;回到底部即清零)
    var unreadBlocks by remember(threadId) { mutableStateOf(0) }
    // 程序滚动标记:不是 UI 状态(不参与重组),只用来让"距底观测"忽略我们自己
    // 发起的 scrollToItem —— 否则跟随滚动进行中的中间位置会被误判成"用户滚走了"。
    val programmaticScroll = remember(threadId) { AtomicBoolean(false) }
    // 上次"新内容到达"effect 记到的位置基线(差值 = 本次新增,喂给未读计数)
    var arriveItemCount by remember(threadId) { mutableStateOf(0) }
    var arriveStreamBlocks by remember(threadId) { mutableStateOf(0) }
    val haptic = LocalHapticFeedback.current
    val reduceMotion = rememberReduceMotion()

    // ---- 历史消息快速查看(视频样板):刻度条目 + 当前刻度 + 跳转 ----
    // 刻度 = 每条用户消息;LazyColumn 头部偏移:时间线顶部挂「加载更早」入口时
    // itemsIndexed 的下标整体 +1(与该入口的显隐条件严格一致)。
    // 【P3 Task 8】派生式上收为 ChatState.listOffset(语义原样),此处只投影。
    val historyListOffset = chatState.listOffset
    val historyEntries = remember(threadId, historyListOffset, st.items.size) {
        HistoryQuickNavLogic.entriesOf(st.items.toList(), historyListOffset)
    }
    // 当前刻度由 LazyListState 首个可见项驱动(规格:不额外造状态)
    val historyCurrentIndex = HistoryQuickNavLogic.currentIndexOf(
        historyEntries, listState.firstVisibleItemIndex,
    )
    // 按住拖动预览的实时手势状态(二版):刻度条写、预览气泡读;fingerY 逐帧更新
    // 但只在 draw/offset 延迟读里消费,拖动全程不重组列表。
    val historyDrag = remember(threadId) { HistoryDragState() }
    // 【R2 日期上下文】刻度日界标记(每天第一条画全长):随 entries 变化重算,
    // "now" 只影响跨日边界的归属,聊天页存续期内漂移可忽略。
    val historyDayStarts = remember(historyEntries) {
        HistoryQuickNavLogic.dayStartFlags(historyEntries, System.currentTimeMillis())
    }

    /** 浮层点卡片:跳到那条消息并关浮层(浮层自己不持列表状态)。 */
    fun jumpToHistoryEntry(listIndex: Int) {
        showHistoryPanel = false
        scope.launch {
            programmaticScroll.set(true)
            try {
                listState.scrollToItem(listIndex)
            } finally {
                programmaticScroll.set(false)
            }
            // 跳去历史 = 用户主动离开底部:解除跟随,新内容交给「↓ N 条新内容」胶囊
            followBottom = false
        }
    }

    /** 预览松手:跳到手指停住处的那条用户消息(与浮层点卡片同一条滚动路径)。 */
    fun jumpToHistoryTick(entryIndex: Int) {
        historyEntries.getOrNull(entryIndex)?.let { jumpToHistoryEntry(it.listIndex) }
    }

    /**
     * 【R1 实时刷选(Telegram date-bar 模式)】按住刻度条拖动,跨过刻度列表**立刻**
     * 滚到那条 —— "预览→松手才跳"两段合一为"拖到哪看到哪"。滚动用
     * requestScrollToItem(非挂起,可在手势中逐帧调用,不会与进行中的滚动冲突);
     * 刷选即离开底部:解除跟随,程序滚动标记压住"距底观测"(onScrubEnd 复位)。
     */
    fun scrubToHistoryTick(entryIndex: Int) {
        val entry = historyEntries.getOrNull(entryIndex) ?: return
        programmaticScroll.set(true)
        followBottom = false
        listState.requestScrollToItem(entry.listIndex)
    }

    /**
     * 【R3 面板快跳·最新】回底并**恢复跟随** —— 与跳历史相反的语义:用户明确
     * 要回到对话末端看新内容,此后流式输出继续拽底。路径与「↓ N 条新内容」
     * 胶囊一致(程序滚动标记挡住距底观测,完成后重申跟随)。
     */
    fun jumpToLatestFromHistory() {
        showHistoryPanel = false
        scope.launch {
            programmaticScroll.set(true)
            try {
                listState.scrollToItem(st.items.size - 1)
                followBottom = true
            } finally {
                programmaticScroll.set(false)
            }
        }
    }

    // ---- 【全文搜索·Task 7】daemon RPC 接线:message_id 主键定位 + 缺口自动拉取 ----
    // 防抖与五态呈现都在浮层内;这里注入两条 RPC 通道(搜索 + 历史片段拉取,
    // 均 IO 线程),并负责点命中后的自动定位流:message_id 命中身份索引 → 文本/
    // 时间对齐兜底 → 缺口自动请求目标历史片段(拉完只补头部,不做整列表替换)→
    // 滚动锚定;定位不到诚实降级(人话 Toast),绝不假跳。
    val historySearchRepository = remember(threadId) {
        HistorySearchRepository(
            threadId = threadId,
            searchRpc = { rpcThreadId, query, limit ->
                withContext(Dispatchers.IO) {
                    AgentConnection.threadService().searchThreadMessages(
                        SearchThreadMessagesRequest.newBuilder()
                            .setThreadId(rpcThreadId)
                            .setQuery(query)
                            .setLimit(limit)
                            .build(),
                    )
                }
            },
            historyRpc = { rounds ->
                withContext(Dispatchers.IO) {
                    AgentConnection.threadService().getThreadHistory(openThreadHistoryRequest(threadId, rounds))
                }
            },
        )
    }

    /**
     * 把一条全文命中解析成 LazyColumn 下标(缺口自动拉取,有界)。
     * 档位顺序:message_id 主键(身份索引,带角色校验)→ 文本/时间对齐兜底
     * (下标无关,回滚/分叉后仍准)→ 都不中按计划自动拉取后重试。
     * @return LazyColumn 下标;null = 有界努力后仍未定位(调用方给人话)。
     * @throws Exception 拉取 RPC 失败(调用方归一为人话)。
     */
    suspend fun resolveHitToListIndex(hit: HistorySearchHit): Int? {
        // 已加载档(主键 message_id,空 ID 退 sequence;坐标不可信不参与)
        HistorySearchState.locateLoadedSpan(hit, historyIdentity)?.let {
            return it.itemStart + historyListOffset
        }
        // 已加载兜底档(文本/时间对齐,身份索引过期也准)
        HistoryQuickNavLogic.locateHit(st.items.toList(), hit, historyListOffset)?.let { return it }
        // 缺口档:按裁定自动拉取(精确档一轮必中 / 旧 daemon 步进探索,皆有界)
        var roundsTried = 0
        var expansions = 0
        while (expansions < HistorySearchState.MAX_LOCATE_EXPANSIONS) {
            when (val plan = HistorySearchState.planHitLocate(
                hit, historyIdentity, historyRoundsLoaded, AgentConnection.historyRounds,
            )) {
                is HistoryLocatePlan.Jump -> return plan.itemIndex + historyListOffset
                is HistoryLocatePlan.Unresolvable -> return null
                is HistoryLocatePlan.FetchTarget -> {
                    if (plan.requestedRounds <= roundsTried) return null // 同档拉过仍未见:不重复拉
                    roundsTried = plan.requestedRounds
                    expansions++
                    Toast.makeText(ctx, "正在加载更早的历史以定位…", Toast.LENGTH_SHORT).show()
                    val msgs = historySearchRepository.fetchTargetMessages(plan.requestedRounds)
                    if (msgs.isEmpty()) return null
                    val grew = prependHistoryMessages(msgs) > 0
                    // 【P3 Task 8】状态写入经 ViewModel 意图(裁定逻辑原样)
                    chatVm.send(ChatIntent.RoundsAdvancedIfBeyond(plan.requestedRounds))
                    // 截断位与身份索引可信位跟随最新回包(sequence 坐标系随之校准)
                    chatVm.send(
                        ChatIntent.TruncationSettled(
                            HistoryRoundsLogic.canLoadEarlier(
                                firstRole = msgs.firstOrNull()?.role,
                                roundCount = msgs.count { it.role == "user" },
                                requestedRounds = plan.requestedRounds,
                            ),
                        ),
                    )
                    // 拉完重定位:先主键/兼容键,再文本对齐(新到消息可能直接对上)
                    HistorySearchState.locateLoadedSpan(hit, historyIdentity)?.let {
                        return it.itemStart + historyListOffset
                    }
                    HistoryQuickNavLogic.locateHit(st.items.toList(), hit, historyListOffset)?.let { return it }
                    if (!grew) return null // daemon 已到会话开头还没有:诚实失败
                }
            }
        }
        return null
    }

    /** 点全文命中:主键定位 → 缺口自动拉取 → 滚动锚定;定位不到给人话,不假跳。 */
    val onHistoryFullTextHitClick: (HistorySearchHit) -> Unit = { hit ->
        scope.launch {
            val listIndex = try {
                resolveHitToListIndex(hit)
            } catch (e: Exception) {
                if (e is kotlinx.coroutines.CancellationException) throw e
                Toast.makeText(ctx, "自动定位失败：${HistorySearchRepository.failureReason(e)}", Toast.LENGTH_SHORT).show()
                null
            }
            if (listIndex != null) {
                jumpToHistoryEntry(listIndex)
            } else {
                Toast.makeText(ctx, "未能定位到该消息：它可能在更早的历史里，请稍后再试", Toast.LENGTH_SHORT).show()
            }
        }
    }

    // 距底观测:跟随状态只由"当前列表位置"裁定 —— 用户上滑出 100px 即解除跟随;
    // 手动滚回距底 100px 内即恢复跟随并清未读。程序滚动(带标记)不参与裁定。
    LaunchedEffect(listState) {
        snapshotFlow {
            val info = listState.layoutInfo
            val last = info.visibleItemsInfo.lastOrNull()
            distanceFromBottomPx(
                totalItems = info.totalItemsCount,
                lastVisibleIndex = last?.index ?: -1,
                lastVisibleEnd = (last?.offset ?: 0) + (last?.size ?: 0),
                viewportEndOffset = info.viewportEndOffset,
            )
        }.collect { distance ->
            if (programmaticScroll.get()) return@collect
            val nowFollowing = shouldFollowDistance(distance)
            // 从"离底"被用户亲手滚回"贴底":一次轻触觉确认跟随已恢复(可选,跟减弱动效)
            if (nowFollowing && !followBottom && !reduceMotion) {
                haptic.performHapticFeedback(HapticFeedbackType.TextHandleMove)
            }
            followBottom = nowFollowing
            if (nowFollowing) unreadBlocks = 0
        }
    }

    // 新内容到达:仅当"本来在跟随"(或用户其实仍在距底 100px 内)才自动回底;
    // 真离底(上滑阅读)绝不拽动,只累加未读块数交给「↓ N 条新内容」胶囊。
    LaunchedEffect(st.items.size, st.streamText.length / STREAM_BLOCK_CHARS, st.streaming) {
        val wasFollowing = followBottom
        val itemBlocks = (st.items.size - arriveItemCount).coerceAtLeast(0)
        val streamBlocks = (streamBlockIndex(st.streamText.length) - arriveStreamBlocks).coerceAtLeast(0)
        arriveItemCount = st.items.size
        arriveStreamBlocks = streamBlockIndex(st.streamText.length)
        unreadBlocks = accumulateUnreadBlocks(wasFollowing, unreadBlocks, itemBlocks, streamBlocks)
        val total = st.items.size + if (st.streaming) 1 else 0
        // 用户手势进行中(drag/fling,且非我们的程序滚动)绝不抢滚动:同一帧里
        // "上滑"与"新内容到达"撞车时,以用户为准 —— 下一帧距底观测会解除跟随,
        // 之后的新块进未读计数,交给胶囊。
        val userScrolling = listState.isScrollInProgress && !programmaticScroll.get()
        if (total > 0 && !userScrolling &&
            shouldAutoScroll(distanceFromBottomOfList(listState), wasFollowing)
        ) {
            programmaticScroll.set(true)
            try {
                listState.scrollToItem(total - 1)
                // 重申跟随:程序回底不是"用户离底",距底观测被标记挡住看不到这帧
                followBottom = true
            } finally {
                programmaticScroll.set(false)
            }
        }
    }

    // 【PF4】按 threadId 记忆(getThread 用 t.title 回填且空标题不覆盖 —— 不带键时
    // 换到无标题会话会永远顶着上一会话的标题;带键换会话即回落「任务」)。
    var threadTitle by remember(threadId) { mutableStateOf("任务") }
    // daemon 上报的会话状态(getThread 回填;四词状态徽章的兜底数据源。
    // 运行中/恢复中由 st.running 实时覆盖,待审批由 st.pendingApproval 实时覆盖 ——
    // 徽章口径见下方 identityStatus 的装配注释)
    var threadStatus by remember(threadId) { mutableStateOf(ThreadStatus.THREAD_STATUS_UNSPECIFIED) }
    // 【桌面只读回显】会话目标顶栏文案(GoalDisplayLogic:goal_detail 电脑端现读优先,
    // goal 本地库兜底):有则显示一行,无则不占位。
    var threadGoalLine by remember(threadId) { mutableStateOf<String?>(null) } // 【PF4】换会话重置,理由同 threadWorkspace
    LaunchedEffect(threadId, connectionEpoch, reloadToken) {
        // 【P1-4 阶段 D · 离线快照】已知断网(down)进页且时间线还没内容:
        // 直接用本地只读副本渲染 + 顶部横幅 + 输入停用。checking 是巡检中间态,
        // 不算断网;有连接时行为完全不变。恢复连接会触发 connectionEpoch 变化
        // 重跑本效果,由在线分支把 offlineSnapshot 归位。
        if (st.items.isEmpty()) {
            val snap = withContext(Dispatchers.IO) { dev.pocketagent.app.ui.component.ThreadSnapshot.readThreadSnapshot(threadId) }
            if (snap != null) {
                st.items.clear()
                // 【深优 R2 修复】快照里的真实时间戳必须带上 —— 此前在这里被丢弃,
                // 且快照非空会跳过下面的在线加载,导致历史导航的日期分组全程失效
                st.items.addAll(snap.map {
                    ChatItem(role = it.role, text = it.text, sentAtMs = it.savedAtMs)
                })
                if (st.items.isNotEmpty()) listState.scrollToItem(st.items.size - 1)
                // 【P3 Task 8】离线快照标志经 ViewModel 意图落位(在线路径不动身份索引)
                chatVm.send(ChatIntent.ShowOfflineSnapshot(dev.pocketagent.app.ui.component.ThreadSnapshot.messagesAt(threadId)))
                loadError = null
            }
            // 无快照(null):不伪造内容,继续走在线加载(失败时按原有报错分支提示)
        }
        try {
            val t = withContext(Dispatchers.IO) {
                AgentConnection.threadService().getThread(GetThreadRequest.newBuilder().setThreadId(threadId).build())
            }
            if (t.title.isNotEmpty()) threadTitle = t.title
            threadStatus = t.status
            // 【P1-6】顺手把会话名交给通知模块:后台轮次完成时,通知标题用真实会话名
            PaNotifications.rememberTitle(threadId, t.title)
            threadWorkspace = t.workspace.ifBlank { null }
            // 斜杠命令面板按会话归属 agent 开关(只认 zcode;空/未知不显示)
            threadAgentId = t.agentId.ifBlank { "codex" }
            if (AgentConnection.threadModel(threadId) == null) AgentConnection.setThreadModel(threadId, t.modelId)
            if (AgentConnection.threadEffort(threadId) == null) AgentConnection.setThreadEffort(threadId, t.effort)
            // 会话真实生效模型(daemon 落库):模型芯片/弹窗选中态的数据源
            threadModelId = t.modelId
            if(st.replyCh==null) st.pendingApproval=t.pendingApprovalsList.firstOrNull()
            if (!st.running && !st.recovering) {
                if(t.status==ThreadStatus.THREAD_STATUS_RUNNING) TurnSession.retryRecovery(threadId)
                else if(t.lastTurnError.isNotBlank()) st.errorMessage=t.lastTurnError
            }
            // 目标回读(只增):daemon 在 GetThread 里现读 codex 的 thread/goal/get,
            // 这里只管把"目标:xxx"文案算出来;读不到/没设目标时为 null,不占位。
            threadGoalLine = GoalDisplayLogic.goalLine(t)
        } catch (e: Exception) {
            loadError = "无法加载会话：${e.message ?: "连接失败"}"
        }
        try {
            // 后台轮次进行中(或已有实时列表)时不覆盖
            if (!st.running && !st.recovering &&
                (st.items.isEmpty() || connectionEpoch > 0 || reloadToken > 0)
            ) {
                // 【F-D 历史按轮加载】只拉最近 historyRoundsLoaded 轮(设置档位起步,
                // 「加载更早」累加);重连/刷新沿用累计轮数,视图不缩水。导出、
                // 恢复/resync(TurnSession)与导出面板仍走全量,与这里互不影响。
                val resp = withContext(Dispatchers.IO) {
                    AgentConnection.threadService().getThreadHistory(
                        openThreadHistoryRequest(threadId, historyRoundsLoaded),
                    )
                }
                // 【F-D】是否被截断决定时间线顶部「加载更早」的显隐(纯函数在
                // HistoryRoundsLogic,JVM 单测钉住;空响应=还没有历史,不显示)
                val truncated = HistoryRoundsLogic.canLoadEarlier(
                    firstRole = resp.messagesList.firstOrNull()?.role,
                    roundCount = resp.messagesList.count { it.role == "user" },
                    requestedRounds = historyRoundsLoaded,
                )
                if (resp.messagesList.isNotEmpty()) {
                    // 【Task 7】落地同时重建消息级身份索引(message_id→时间线跨度,
                    // 搜索命中定位的数据底座)与「已加载最旧 id」锚点(加载更早
                    // 防重复拉取用)。
                    val (items, spans) = buildHistoryBatch(resp.messagesList)
                    st.items.clear()
                    st.items.addAll(items)
                    // 【P3 Task 8】身份索引/最旧锚点/截断位经 ViewModel 一次落位
                    // (knownTotal 保留在 reducer 内,普通拉取不洗掉搜索回包的全量基准)
                    chatVm.send(
                        ChatIntent.HistoryTailLoaded(
                            spans = spans,
                            oldestMessageId = resp.messagesList.first().messageId,
                            truncated = truncated,
                        ),
                    )
                    listState.scrollToItem(st.items.size - 1)
                }
                // 【P3 Task 8】截断位在 if 外落位(原语义:空回包也更新入口显隐;
                // 非空回包与 HistoryTailLoaded 落同值,幂等)
                chatVm.send(ChatIntent.TruncationSettled(truncated))
                // 【离线快照】成功后落一份本地只读副本(只留 role+text:
                // 图片/工具结果离线圈不了,宁可少显示也不能错显示)。
                // 【深优 R2 修复】时间戳随行保存 —— 此前整列剥掉,快照恢复后
                // 历史导航的日期分组全部退化为「早前」。
                dev.pocketagent.app.ui.component.ThreadSnapshot.saveMessages(
                    threadId,
                    st.items
                        .filter { it.role != "tool" && it.attachments.isEmpty() && it.text.isNotBlank() }
                        .map { dev.pocketagent.app.ui.component.ThreadSnapshot.Msg(it.role, it.text, it.sentAtMs) },
                )
            }
            st.historyLoaded = true
            // 【P3 Task 8】恢复在线撤掉残留的离线提示(flag + 落盘时间成对清)
            chatVm.send(ChatIntent.BackOnline)
            if (loadError?.startsWith("离线") == true) loadError = null
        } catch (e: Exception) {
            // 连不上:有快照就渲染快照 + 顶部横幅(输入同时停用),没有才报错(保持原行为)。
            val snap = withContext(Dispatchers.IO) { dev.pocketagent.app.ui.component.ThreadSnapshot.readThreadSnapshot(threadId) }
            if (!snap.isNullOrEmpty() && st.items.isEmpty()) {
                st.items.clear()
                st.items.addAll(snap.map { ChatItem(role = it.role, text = it.text) })
                listState.scrollToItem(st.items.size - 1)
                // 快照条目没有 daemon 身份:身份索引一并作废(离线下搜索本就不可达,
                // 重连重建 —— 绝不让旧索引把命中指进快照列表)
                // 【P3 Task 8】resetIdentity = true 对应迁出前的索引/锚点清空
                chatVm.send(
                    ChatIntent.ShowOfflineSnapshot(
                        dev.pocketagent.app.ui.component.ThreadSnapshot.messagesAt(threadId),
                        resetIdentity = true,
                    ),
                )
                loadError = null
            } else {
                loadError = "无法加载会话历史：${e.message ?: "连接失败"}"
                chatVm.send(ChatIntent.BackOnline)
            }
        }
    }

    /**
     * 【F-D 加载更早 / Task 7 升级】时间线顶部入口:累计轮数再加一个设置档位重拉。
     * daemon 幂等返回尾部,本次回包 = 更早新增段 + 已加载部分 —— 落地走
     * [prependHistoryMessages]:按「已加载最旧 message id」锚点只把头部新消息
     * 拼到列表前,**不做整列表替换**(已加载条目引用不变,不整表重组;回包没有
     * 新消息时零列表变动,重复拉取无副作用)。旧 daemon 无 id 时退条数兜底,
     * 行为等价旧的"整体替换但 delta 修正"。
     *
     * 滚动锚点(保留):拉取前记 LazyListState 的 firstVisibleItem(下标+偏移),
     * 头部拼接后旧条目整体下移 delta = 新拼入条目数,锚点下标 + delta 就是原来
     * 那条消息,页面不跳动;锚点在 0(「加载更早」按钮自己)说明用户已翻到顶,
     * 直接回列表顶部。
     */
    fun loadEarlier() {
        if (loadingEarlier || offlineSnapshot || st.running || st.recovering) return
        chatVm.send(ChatIntent.LoadEarlierStarted)
        val anchorIndex = listState.firstVisibleItemIndex
        val anchorOffset = listState.firstVisibleItemScrollOffset
        scope.launch {
            try {
                val next = HistoryRoundsLogic.nextRounds(historyRoundsLoaded, AgentConnection.historyRounds)
                val resp = withContext(Dispatchers.IO) {
                    AgentConnection.threadService().getThreadHistory(openThreadHistoryRequest(threadId, next))
                }
                if (resp.messagesList.isNotEmpty()) {
                    val delta = prependHistoryMessages(resp.messagesList)
                    chatVm.send(ChatIntent.RoundsAdvanced(next))
                    val target = if (anchorIndex <= 0) 0 else (anchorIndex + delta).coerceAtMost(st.items.size - 1)
                    listState.scrollToItem(target, if (anchorIndex <= 0) 0 else anchorOffset)
                }
                // 身份索引的"已到全量"可信位跟随最新截断位(sequence 兼容定位用)
                // 【P3 Task 8】截断位与可信位经 ViewModel 成对落位
                chatVm.send(
                    ChatIntent.TruncationSettled(
                        HistoryRoundsLogic.canLoadEarlier(
                            firstRole = resp.messagesList.firstOrNull()?.role,
                            roundCount = resp.messagesList.count { it.role == "user" },
                            requestedRounds = next,
                        ),
                    ),
                )
            } catch (e: Exception) {
                Toast.makeText(ctx, "加载更早失败：${e.message ?: "连接失败"}", Toast.LENGTH_SHORT).show()
            } finally {
                chatVm.send(ChatIntent.LoadEarlierFinished)
            }
        }
    }

    /**
     * 发送(空闲开新轮次 / 运行中插入或排队,见 TurnSession.send)。
     * @return false = 未接收(空文本 / 离线 / 流断恢复中):调用方不清空输入框。
     */
    fun send(text: String): Boolean {
        if (text.isBlank() || offlineSnapshot) return false
        val wasRunning = st.running // 先取:TurnSession.send 里可能刚把轮次置为 running
        val sent = attachments
        if (!TurnSession.send(threadId, text, sent)) return false
        attachments = emptyList()
        if (wasRunning) {
            // 运行中提交的回执:发送语义随 daemon 的 steer:<agentId> 位如实二选一
            // (有位=真插话进当前轮次,缺位=daemon 排队兜底);最终落点以时间线
            // 投递小标签为准(daemon 应答裁定),这里只报提交口径,不猜结果。
            Toast.makeText(ctx, steerSubmitNotice(agentCapableOf("steer", threadAgentId, hostCaps)), Toast.LENGTH_SHORT).show()
        }
        return true
    }

    /**
     * 「未送达」消息的整条重发(P0-2 失败级的重试入口,点的是整条消息气泡)。
     *
     * 复用既有发送语义 TurnSession.send(运行中=再走插话/排队,空闲=开新轮),
     * 不另造一条发送路径;成功后把失败气泡从时间线收掉 —— 同一句话在屏上只留
     * 一种形态(WhatsApp 口径:重试成功,红!气泡原位消失,新气泡带自己的
     * 投递状态),避免重发后出现两条一模一样的话。失败(仍离线等)什么都不动:
     * 气泡和红!标留着,下次再点。判定可重试走纯函数 deliveryRetryable。
     */
    fun retryDelivery(item: ChatItem) {
        if (!deliveryRetryable(item.deliverState) || offlineSnapshot) return
        if (!TurnSession.send(threadId, item.text, item.attachments)) return
        // 按内容+状态定位失败气泡(TurnSession.send 刚追加的新气泡是 pending,不会误中)
        val idx = st.items.indexOfFirst {
            it.deliverState == DELIVERY_OVERFLOW && it.text == item.text && it.attachments.size == item.attachments.size
        }
        if (idx >= 0) st.items.removeAt(idx)
    }

    /**
     * 分叉:从当前会话的最新状态复制出一条新会话,原会话不动。
     *
     * 【为什么不做"从某条消息分叉"】手机端时间线里没有 codex 的回合 id ——
     * 历史接口(ThreadMessage)不带 turnId,所以给不出"从这一回合之后分叉"。
     * 与其做一个选了也传不对参数的回合选择器,不如只提供"从最新状态分叉"这个
     * 真实成立的语义,并把这句话写在按钮下面。
     */
    fun doFork() {
        // 【防重入闸】与 doRollback/doCompact 同款:forkThread 在途时再点直接忽略,
        // 不重复发请求、不叠 Toast。
        if (forkBusy) return
        showMore = false
        if (st.running) {
            Toast.makeText(ctx, "任务正在执行,等它跑完再分叉", Toast.LENGTH_SHORT).show()
            return
        }
        forkBusy = true
        scope.launch {
            try {
                val nt = withContext(Dispatchers.IO) {
                    AgentConnection.threadService().forkThread(
                        ForkThreadRequest.newBuilder().setThreadId(threadId).build())
                }
                Toast.makeText(ctx, "已分叉出「${nt.title}」", Toast.LENGTH_SHORT).show()
                onOpenThread(nt.id)
            } catch (e: Exception) {
                Toast.makeText(ctx, e.message ?: "分叉失败", Toast.LENGTH_LONG).show()
            } finally {
                forkBusy = false
            }
        }
    }

    /** 回滚最近 turns 个回合:只改对话历史,不会撤销电脑上已经改过的文件。 */
    fun doRollback(turns: Int) {
        if (rollbackBusy) return
        rollbackBusy = true
        scope.launch {
            try {
                withContext(Dispatchers.IO) {
                    AgentConnection.threadService().rollbackThread(
                        RollbackThreadRequest.newBuilder()
                            .setThreadId(threadId)
                            .setNumTurns(turns)
                            .build())
                }
                rollbackOpen = false
                // 重新拉一次历史:daemon 会把本地历史裁掉,但手机端手上那份还是旧的,
                // 不重拉就会出现"回滚了但屏幕上那几条还在"。
                reloadToken++
                Toast.makeText(ctx, "已回滚最近 $turns 个回合", Toast.LENGTH_SHORT).show()
            } catch (e: Exception) {
                Toast.makeText(ctx, e.message ?: "回滚失败", Toast.LENGTH_LONG).show()
            } finally {
                rollbackBusy = false
            }
        }
    }

    /** 请求 Codex 压缩当前会话上下文;完成以后由 contextCompaction 事件刷新状态。 */
    fun doCompact() {
        if (compactBusy) return
        if (st.running) {
            Toast.makeText(ctx, "任务正在执行,等它完成后再压缩上下文", Toast.LENGTH_SHORT).show()
            return
        }
        compactBusy = true
        showMore = false
        scope.launch {
            try {
                withContext(Dispatchers.IO) {
                    AgentConnection.threadService().compactThread(
                        CompactThreadRequest.newBuilder().setThreadId(threadId).build(),
                    )
                }
                Toast.makeText(ctx, "已请求压缩上下文,完成后会显示压缩状态", Toast.LENGTH_SHORT).show()
            } catch (e: Exception) {
                Toast.makeText(ctx, e.message ?: "压缩上下文失败", Toast.LENGTH_LONG).show()
            } finally {
                compactBusy = false
            }
        }
    }

    /**
     * "/计划"(review:<agentId> 位会话,现仅 codex;可见性由斜杠面板按能力位
     * 裁决):发起计划审查。review/start 是"发起即回"
     * (daemon 侧实测 0.0s 返回,turn.status=inProgress),审查产出以
     * 事件(item/turn 系列)流回**本会话**,时间线直接可见,这里不需要任何后续轮询。
     * focus 是词元后已敲的审查重点(通常为空,daemon 有默认审查口径);
     * 失败走 slashReviewFailureMessage 的人话 Toast(daemon 业务错误透传,
     * 传输层错误翻成人话)。
     */
    fun doStartReview(focus: String) {
        if (reviewBusy) return
        if (st.running) {
            Toast.makeText(ctx, "任务正在执行,等它完成后再发起计划审查", Toast.LENGTH_SHORT).show()
            return
        }
        reviewBusy = true
        scope.launch {
            try {
                withContext(Dispatchers.IO) {
                    AgentConnection.threadService().startReview(
                        StartReviewRequest.newBuilder()
                            .setThreadId(threadId)
                            .setFocus(focus)
                            .build(),
                    )
                }
                Toast.makeText(ctx, "已发起计划审查,结果将以事件形式出现在会话中", Toast.LENGTH_LONG).show()
            } catch (e: Exception) {
                Toast.makeText(ctx, slashReviewFailureMessage(e), Toast.LENGTH_LONG).show()
            } finally {
                reviewBusy = false
            }
        }
    }

    /**
     * 撤回排队输入(pi 0.87.1 clear_queue 的手机端入口,等价 pi TUI 里按 Esc):
     * 清的是 pi 子进程内存里的原生排队(steer/followUp),不是 daemon 侧续跑队列。
     *
     * 【成功后两件事】
     *  1. 人话回执:daemon 回的 steered/followUp 条数经 queueClearedNotice 如实报;
     *  2. 刷新排队镜像:st.queuedCount 是手机侧对 daemon 排队的镜像(Queued 回执
     *     +1/QueueFlushed -1),队列被撤后镜像必须同步归零 —— 否则本轮终局帧到达时
     *     queuedCount>0 会让收尾逻辑一直等一条不会来的 QueueFlushed,轮次卡在"运行中";
     *     时间线上还挂着「排队中」的气泡一并收掉(变回普通用户消息)。
     * 【失败】错误原文透传(queueClearFailureMessage 只补人话壳),不猜不改写。
     */
    fun clearQueuedInputs() {
        scope.launch {
            try {
                val resp = withContext(Dispatchers.IO) {
                    AgentConnection.threadService().clearQueuedInputs(
                        ClearQueuedInputsRequest.newBuilder().setThreadId(threadId).build(),
                    )
                }
                st.queuedCount = 0
                clearDeliveryTags(st.items.toList(), queuedOnly = true)?.let { replaced ->
                    st.items.clear()
                    st.items.addAll(replaced)
                }
                Toast.makeText(ctx, queueClearedNotice(resp.steered, resp.followUp), Toast.LENGTH_SHORT).show()
            } catch (e: Exception) {
                Toast.makeText(ctx, queueClearFailureMessage(e), Toast.LENGTH_LONG).show()
            }
        }
    }

    // ---- R2-A 消息长按上下文菜单(单一实例,锚点=被长按的那条消息)----
    // 菜单本体在根层渲染(Popup,见 HistoryQuickPanel 旁);这里只持三份状态:
    //   messageMenu        锚点坐标(component/ContextMenuState)
    //   messageMenuTarget  被长按的消息(菜单摘要/动作都针对它)
    //   messageSelectArmed 「选择文本」的进入请求 —— 菜单点了它之后发给时间线里
    //                      对应 item 的局部 selectMode(LazyColumn 的 item 只能
    //                      改自己的局部态,跨层经这一棒交接)
    val messageMenu = rememberContextMenuState()
    // 【PF4】按 threadId 记忆:菜单目标/选择模式请求持有旧会话 ChatItem 引用,
    // 换会话必须作废(否则跨会话 copy/quote/retry 操作到上一会话的消息)。
    var messageMenuTarget by remember(threadId) { mutableStateOf<ChatItem?>(null) }
    var messageSelectArmed by remember(threadId) { mutableStateOf<ChatItem?>(null) }

    val connSt = AgentConnection.connState.collectAsState().value
    val hasFailure = loadError != null ||
        st.recovering ||
        st.recoveryError != null ||
        st.errorMessage != null ||
        connSt == "down"
    // 呈现层断连横幅状态:推进逻辑全在 component/nextConnectionBannerState(纯函数,
    // ConnectionBannerStateTest 钉死),本屏编辑式横幅(ChatOfflineBanner)只读结果 ——
    // 本地那份复制实现已删(它带两个已知缺陷:挂载即 down 虚计一轮;按 connDown 渲染,
    // checking 期间横幅闪没)。状态机口径:
    //  - 挂载即 down:可见但不虚计(prev 取挂载时现值);
    //  - checking:断过(attempts>0)保持可见 —— 探活回退链动辄数秒,撤了再放回
    //    就是用户看见的"横幅闪烁";没断过的首轮 checking 不出横幅;
    //  - ok:退场且轮数清零,不弹"已恢复"成功横幅。
    var chatBanner by remember {
        mutableStateOf(ConnectionBannerState(visible = false, attempts = 0))
    }
    var chatBannerPrev by remember { mutableStateOf(AgentConnection.connState.value) }
    LaunchedEffect(Unit) {
        AgentConnection.connState.collect { s ->
            chatBanner = nextConnectionBannerState(chatBannerPrev, s, chatBanner.attempts)
            chatBannerPrev = s
        }
    }

    // ---- 会话四词状态(轮次状态用 StatusBadge 四词;映射走 ui/EditorialStatus 单一事实源)----
    // 实时性:运行中/恢复中以 st.running·st.recovering 覆盖(daemon 状态滞后一拍);
    // 待审批优先于一切(同一事实在首页/任务中心/会话长出同一种状态);本轮已收尾且
    // 无错误按 IDLE 归「已完成」;其余落到 getThread 回填的 daemon 状态。
    val identityStatus = run {
        val pending = if (st.pendingApproval != null) 1 else 0
        val status = when {
            st.running || st.recovering -> ThreadStatus.THREAD_STATUS_RUNNING
            st.turnTerminal && st.errorMessage == null -> ThreadStatus.THREAD_STATUS_IDLE
            pending > 0 -> ThreadStatus.THREAD_STATUS_WAITING_APPROVAL
            else -> threadStatus
        }
        editorialThreadStatusView(status, pending, st.errorMessage ?: "")
    }

    // =====================================================================
    // 审批弹窗状态机(README §审批弹窗状态机;皮 = component/PaApprovalDialog,
    // 纯逻辑 = ui/EditorialApproval,数据流 = 现有 daemon ApprovalRequest 链)
    //
    // 请求(TurnSession.APPROVAL_REQUEST → st.pendingApproval)
    //   → 组装 ApprovalRequestUi(PENDING)并自动弹窗
    //   → onDecide → submitting(allow) → TurnSession.answerApproval(现有 SubmitApproval 链)
    //   → 请求从 st.pendingApproval 移除 = 服务端链路收下 → serverConfirmed → settled
    //     → ALLOWED/DENIED 终态词展示一拍 → 清弹窗、清待审批 UI
    //   → 提交失败(st.approvalError 非空且请求仍在)→ markUnavailable(人话原因)+ 重试
    //   → 请求在电脑端被先行处理(APPROVAL_RESOLVED/轮次终局清卡)→ markUnavailable 收场
    // 红线:dismiss(弹窗外点击/返回)= 暂不处理,请求保持待审批,绝不等于拒绝;
    //       不做乐观通过 —— 终态词只在提交链路收下请求后出现。
    // =====================================================================
    var approvalUi by remember(threadId) { mutableStateOf<ApprovalRequestUi?>(null) }
    var approvalDialogOpen by remember(threadId) { mutableStateOf(false) }
    LaunchedEffect(st.pendingApproval?.requestId) {
        val ap = st.pendingApproval
        if (ap == null) {
            val ui = approvalUi ?: return@LaunchedEffect
            approvalUi = when (ui.phase) {
                // 提交在途时请求消失 = answerApproval 已收下(通道/RPC 二选一成功)
                // → 服务端确认,落终态词(「移除待审批 UI」自此有了确认依据)
                ApprovalPhase.SUBMITTING -> ui.serverConfirmed(ui.submittedChoice == true).settled()
                // 未提交时请求消失 = 已在电脑端被处理(快照回填/竞速清理)→ 如实说,不给假决定
                ApprovalPhase.RECEIVED, ApprovalPhase.PENDING ->
                    ui.markUnavailable("这条审批已在电脑端被处理，无需再次决定")
                else -> ui
            }
        } else if (approvalUi?.requestId != ap.requestId) {
            // 新请求到达:组装白名单文案并自动弹窗(既有"请求即达"行为,容器换成弹窗)
            val copy = approvalUiCopy(ap)
            approvalUi = ApprovalRequestUi(
                requestId = ap.requestId,
                threadId = threadId,
                agentName = selectedAgentLabel(threadAgentId),
                sessionTitle = threadTitle.ifBlank { "会话" },
                purpose = copy.purpose,
                workspace = copy.cwd.ifBlank { threadWorkspace.orEmpty() },
                actionKind = copy.actionKind,
                command = copy.command,
                phase = ApprovalPhase.PENDING,
                // 【R2 三选项装配】daemon(R1-C1)透出的选项列表与详情:有则审批卡
                // 渲染三按钮(本次允许/本会话允许/拒绝);旧 daemon 无此字段 → 空
                // 列表走旧两按钮分支,双向兼容。
                options = ap.optionsList.toApprovalOptionUi(),
                details = ap.details,
            )
            approvalDialogOpen = true
        }
    }
    // 提交失败:TurnSession 置 approvalError 且请求仍在 → 暂无法处理(人话原因+重试)
    LaunchedEffect(st.approvalError) {
        val err = st.approvalError ?: return@LaunchedEffect
        val ui = approvalUi ?: return@LaunchedEffect
        if (st.pendingApproval?.requestId == ui.requestId) {
            approvalUi = ui.markUnavailable(err)
        }
    }
    // 相位收尾:CONFIRMED 一拍后落终态词;终态/请求已消失的失效态展示一拍后自动收弹窗。
    // 提交失败的 UNAVAILABLE(请求仍在)**不**自动关 —— 保留请求和决定界面等重试。
    LaunchedEffect(approvalUi?.phase) {
        val ui = approvalUi ?: return@LaunchedEffect
        when (ui.phase) {
            ApprovalPhase.CONFIRMED -> {
                delay(500)
                approvalUi = approvalUi?.settled()
            }
            ApprovalPhase.ALLOWED, ApprovalPhase.DENIED -> {
                delay(900)
                approvalDialogOpen = false
                approvalUi = null
            }
            ApprovalPhase.UNAVAILABLE -> if (st.pendingApproval == null) {
                delay(1400)
                approvalDialogOpen = false
                approvalUi = null
            }
            else -> {}
        }
    }

    // 反问类(request_user_input / mcp_elicitation)仍走既有问题卡:PaApprovalDialog
    // 的白名单没有"问题/选项/自由输入"的槽位(基座组件禁改),而反问必须把答案经
    // note 通道回给 daemon —— 两类语义在数据层同链,在界面层分容器。红线语义
    // (不可下滑关闭/拦截返回/不替用户做决定)由 PaSheetConvergenceTest 钉住不动。
    st.pendingApproval?.let { ap ->
        if (askIsAskAction(ap.action.name)) {
            ApprovalSheet(
                action = ap,
                error = st.approvalError,
                onDecision = { approved, note ->
                    // 语义触觉(P1-5):审批提交 = 轻击确认 —— 手指离开屏幕的瞬间就有
                    // "已交出去了"的物理回执,不等网络往返。
                    PaHaptics.confirm(hapticView)
                    TurnSession.answerApproval(threadId, ap.requestId, approved, note)
                },
            )
        }
    }

    // 审批弹窗本体:phase 驱动,决定按钮/重试/暂不处理三条通道(见上方状态机注释)
    approvalUi?.let { ui ->
        if (approvalDialogOpen) {
            val brandRes = agentBrandIcon(threadAgentId)
            val agentIconSlot: (@Composable () -> Unit)? = if (brandRes != null) {
                {
                    Icon(
                        painterResource(brandRes), null,
                        tint = Color.Unspecified, modifier = Modifier.size(28.dp),
                    )
                }
            } else {
                null
            }
            PaApprovalDialog(
                request = ui,
                onDecide = { allow ->
                    if (ui.decideEnabled) {
                        // 触感按决定语义分档(同动作全局同强度):允许 = confirm
                        // (已交出去);拒绝 = warning(打断电脑端的活,分量更重)。
                        if (allow) PaHaptics.confirm(hapticView) else PaHaptics.warning(hapticView)
                        approvalUi = ui.submitting(allow)
                        // 现有 SubmitApproval 链:通道直发,失败回退 submitApproval RPC;
                        // 请求是否被收下以 st.pendingApproval 的移除为准(见上方状态机)
                        TurnSession.answerApproval(threadId, ui.requestId, allow)
                    }
                },
                onDecideOption = { optionId ->
                    if (ui.decideEnabled) {
                        // 三按钮的第三按钮(acceptForSession 等):同为允许语义 →
                        // confirm 档;选项 id 原词走同一条 answerApproval 链
                        // (approved=true + option,daemon 按协议版本翻译)。
                        PaHaptics.confirm(hapticView)
                        approvalUi = ui.submitting(allow = true, option = optionId)
                        TurnSession.answerApproval(
                            threadId, ui.requestId, approved = true, option = optionId,
                        )
                    }
                },
                onDismiss = {
                    // 关闭 = 暂不处理(请求保持待审批,入口卡/身份行可再次打开)。
                    // 仅提交中/服务端确认中不关 —— 决定已在途,关掉会让确认没有落点。
                    if (ui.phase != ApprovalPhase.SUBMITTING && ui.phase != ApprovalPhase.CONFIRMED) {
                        approvalDialogOpen = false
                    }
                },
                onRetry = {
                    st.approvalError = null
                    approvalUi = approvalUi?.retry()
                },
                agentIcon = agentIconSlot,
            )
        }
    }

    // 根布局包一层 Box:给「已优化提示词[撤销]」的 UndoBar 提供顶部悬浮层
    // (不挤内容;FilesScreen/ThreadsScreen 同款用法)。Column 本体不动。
    Box(Modifier.fillMaxSize()) {
        // 【会话中段 lambda】目标/子代理/提示条/审批入口/计划面板/时间线/「新内容」胶囊
        // —— 黑客终端骨架(原 Column)与编辑式骨架(PaScreenScaffold)一字不差共用;
        // 首部的旧顶栏与断连横幅是黑客终端专属 chrome,块内按 PaHacker 条件渲染。
        val chatMiddle: @Composable ColumnScope.() -> Unit = {
        // 中段条块的水平留白:黑客骨架外层无边距,条块自带 14dp;编辑式骨架由
        // PaScreenScaffold 统一给 23dp 页边距,条块不再另加(8px 网格不叠两层)。
        val midGutter = if (PaHacker.isHacker) 14.dp else 0.dp
        if (PaHacker.isHacker) {
        // ---- 顶栏:☰ 侧边栏(会话列表/搜索)+ 标题 + 状态点 ----
        Row(
            Modifier
                .fillMaxWidth()
                .padding(horizontal = 14.dp, vertical = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            IconActionButton(AppR.drawable.ic_lucide_menu, "菜单", { drawerCtrl.open() }, bg = PaEditorial.Surface)
            Spacer(Modifier.width(14.dp))
            // 【会话页顶部:所用 agent 的图标 + 名称】此前顶栏只有标题,用户看不出
            // "这轮在跟哪个 agent 说话"——尤其四个 agent 能力差异明显(能否派发子
            // 智能体、能否看子代理对话),归属必须一眼可见。图标走 AgentBrand 唯一
            // 映射;未知 agent 不画图标只留名字(不塞通用图标冒充身份)。
            agentBrandIcon(threadAgentId)?.let { res ->
                Icon(
                    painterResource(res), null,
                    tint = PaEditorial.Primary, modifier = Modifier.size(15.dp),
                )
                Spacer(Modifier.width(5.dp))
            }
            Text(
                selectedAgentLabel(threadAgentId),
                fontSize = 11.5.sp, color = PaEditorial.Muted, maxLines = 1,
            )
            Spacer(Modifier.width(8.dp))
            // 【必须显式 Ellipsis】Modifier.weight(1f) 已把宽度约束死,默认
        // TextOverflow.Clip 会"干净地"切掉尾巴且不画省略号,看起来像文字坏了。
        Text(
            threadTitle, Modifier.weight(1f), fontSize = 16.sp, color = PaEditorial.Ink,
            maxLines = 1, overflow = TextOverflow.Ellipsis,
        )
            // 终止正在跑的轮次(用户反馈:任务跑飞了没法自己停)
            if (st.running) {
                IconActionButton(
                    AppR.drawable.ic_lucide_square, "终止任务",
                    {
                        // 终止 = warning 档(打断电脑端的活;与发送键原位终止、
                        // 「更多」面板终止同一档 —— 同动作全局同强度)
                        PaHaptics.warning(hapticView)
                        TurnSession.cancelTurn(threadId)
                        Toast.makeText(ctx, "已请求终止电脑端任务", Toast.LENGTH_SHORT).show()
                    },
                    bg = PaEditorial.Surface, size = 34.dp, iconSize = 16.dp, tint = PaEditorial.Denied,
                )
                Spacer(Modifier.width(8.dp))
            }
            IconActionButton(
                AppR.drawable.ic_lucide_download, "导出对话",
                { showExport = true },
                bg = PaEditorial.Surface, size = 34.dp, iconSize = 16.dp,
            )
            Spacer(Modifier.width(8.dp))
            IconActionButton(
                AppR.drawable.ic_lucide_ellipsis, "更多",
                { showMore = true },
                bg = PaEditorial.Surface, size = 34.dp, iconSize = 16.dp,
            )
            Spacer(Modifier.width(8.dp))
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
                    .background(if (hasFailure) PaEditorial.Denied else PaEditorial.Primary, CircleShape),
            )
            Spacer(Modifier.width(14.dp))
        }

        // ---- 断连全局横幅(P1-2):daemon 断连时与主页/会话列表同一条状态 ----
        // 只在 connState==down 时出现(见组件注释的 checking 纪律),重连成功自动消失;
        // 手动重试走本页既有 reloadToken 重载链(横幅自己不实现重连)。
        ConnectionBanner(onRetry = { reloadToken++ })
        } // PaHacker:旧顶栏 + 断连横幅(黑客终端专属 chrome 到此为止)

        // ---- 会话目标(桌面只读回显,只增):有则显示一行,无则不占位 ----
        // 数据来自 GetThread 的 goal_detail(电脑端 thread/goal/get 现读),
        // 文案由 GoalDisplayLogic 计算;与标题行同边距,单行省略不顶时间线。
        threadGoalLine?.let { goal ->
            Row(
                Modifier
                    .fillMaxWidth()
                    .padding(horizontal = midGutter),
            ) {
                Text(
                    goal,
                    fontSize = 11.5.sp,
                    color = PaEditorial.Muted,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
            }
        }

        // ---- 子对话来源行(T-SUB,蓝本 §5.1):当前线程是某主线程的子代理线程时, ----
        // 标题下补一行"来自主对话 · 父标题" + 兄弟 ◀ n/N ▶ 切换钮(降级钮方案,
        // 取舍见 SubAgentCard.kt 文件头)。切兄弟 = 导航 chat/{siblingId} 打开
        // 上一条/下一条子线程,转场走 NavHost 全局动效(与主对话↔子对话同款)。
        // ---- 子代理接入口(规范 §18.1/§18.2):固定在会话页顶部,不随消息滚动 ----
        // 以前它是消息列表里的一个 item:滚两下就找不到,语义上还像"一条消息"。
        // 现在上移到主对话区域**之上**的固定区;顺带落实 §18.2 —— 子代理消息不再
        // 混进主对话流(主对话只呈现自己的消息),子代理只能从这里的入口下钻。
        val subChildren = remember(SubAgentsState.threads, threadId) {
            SubAgentsLogic.childrenOf(SubAgentsState.threads, threadId)
        }
        val subRunIds = remember(st.running, SubAgentsState.lastRefreshedAt) {
            TurnSession.runningThreadIds() // TurnSession 无锁快照,不进 Compose 状态
        }
        // 【出入场】子代理卡区块出现/消失走 AnimatedContent(淡入+轻微上移 / 淡出,
        // 全 spring 可打断):subChildren 出现的瞬间卡片"浮入",清空时平滑让位,
        // 不硬跳占位。内容刷新(数量不变)不重播转场(同一 targetState)。
        AnimatedContent(
            targetState = subChildren.isNotEmpty(),
            transitionSpec = {
                (fadeIn(PaMotion.emphasized()) + slideInVertically(PaMotion.emphasized()) { it / 6 })
                    .togetherWith(fadeOut(PaMotion.standard()))
            },
            label = "subAgentEntry",
        ) { present ->
            if (present) {
            Box(Modifier.fillMaxWidth().padding(horizontal = midGutter, vertical = 2.dp)) {
                if (subChildren.size == 1) {
                    val only = subChildren[0]
                    SubAgentCard(
                        meta = only,
                        state = SubAgentsLogic.runStateOf(only, subRunIds),
                        onOpen = { onOpenThread(only.threadId) },
                        onPeek = { peekSub = only },
                    )
                } else {
                    SubAgentGroupCard(
                        children = subChildren,
                        liveRunIds = subRunIds,
                        onOpen = { meta -> onOpenThread(meta.threadId) },
                        onPeek = { meta -> peekSub = meta },
                    )
                }
            }
            }
        }

        // 【门控】父 id 明确已知才显示;关系未知(契约缺口)时不占位、不猜。
        val subChildGroup = remember(threadId, SubAgentsState.threads) {
            SubAgentsState.childGroupOf(threadId)
        }
        if (subChildGroup != null) {
            val (subPos, subTotal) = SubAgentsLogic.siblingPos(subChildGroup.siblings, threadId)
            // 父会话是否仍可达(在本库快照里):决定"返回主对话"是可用还是置灰。
            val parentReachable = SubAgentsState.threads.any {
                it.threadId == subChildGroup.parentThreadId
            }
            SubAgentSourceLine(
                parentTitle = subChildGroup.parentTitle,
                pos = subPos,
                total = subTotal,
                onPrev = {
                    SubAgentsLogic.siblingNav(subChildGroup.siblings, threadId, -1)?.let(onOpenThread)
                },
                onNext = {
                    SubAgentsLogic.siblingNav(subChildGroup.siblings, threadId, +1)?.let(onOpenThread)
                },
                // 规范 §18.3:父会话仍在本库里才给入口;不可达(已删除/未同步)时传 null
                // → 控件置灰,不给点了没反应的假入口。
                onBackToParent = if (parentReachable) {
                    { onOpenThread(subChildGroup.parentThreadId) }
                } else {
                    null
                },
            )
        }

        // ---- 连接 / 加载 / 恢复提示条 ----
        // 【统一出口】原先这里只渲染 loadError,于是 TurnSession 里那一整套
        // "断线后自动把电脑端任务结果捞回来"的状态(recovering / recoveryError /
        // errorMessage)**在界面上完全不可见**。真机表现就是用户说的:
        // "连接中断后仍显示红色、没有任何等待 UI" —— 他没法区分
        // "正在自动恢复"和"彻底失败了",只能干等。
        //
        // 现在按优先级合并成一条提示,并且等待态带进度指示:
        //   恢复失败 > 恢复中(等待) > 轮次错误 > 页面加载错误
        run {
            val waiting = st.recovering
            val noticeMsg = when {
                st.recoveryError != null -> st.recoveryError
                waiting -> st.errorMessage ?: "连接已断开，正在恢复电脑端任务结果"
                st.errorMessage != null -> st.errorMessage
                loadError != null -> loadError
                else -> null
            }
            if (noticeMsg != null) {
                // 等待态用琥珀(警示但非失败),彻底失败/加载失败用珊瑚红
                // (编辑式语义色档:ATTENTION / DENIED;黑客终端保持旧黄/红)
                val tone = if (PaHacker.isHacker) {
                    if (waiting) PaEditorial.Attention else PaEditorial.Denied
                } else {
                    if (waiting) PaEditorial.Attention else PaEditorial.Denied
                }
                // 【人话标题】daemon 报错常把内部方法名当首行("session/resume\n<id> 失败:"),
                // 直接上屏像天书(L2 真机挑刺);标题换人话(恢复会话/发送消息/新建会话),
                // 正文保持错误原文;没有方法名前缀时整条原文照抄(纯函数 humanizeMethodError)。
                val split = humanizeMethodError(noticeMsg)
                Row(
                    Modifier
                        .fillMaxWidth()
                        .padding(horizontal = midGutter, vertical = 2.dp)
                        .background(PaEditorial.Surface, RoundedCornerShape(13.dp))
                        .border(1.dp, PaEditorial.Hairline, RoundedCornerShape(13.dp))
                        .padding(horizontal = 12.dp, vertical = 7.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    // 等待态:转圈告诉用户"它在动",不是卡死了
                    if (waiting) {
                        CircularProgressIndicator(
                            Modifier.size(12.dp),
                            strokeWidth = 1.5.dp,
                            color = tone,
                        )
                        Spacer(Modifier.width(8.dp))
                    }
                    Column(Modifier.weight(1f)) {
                        Text(
                            split?.first ?: noticeMsg,
                            fontSize = 12.sp,
                            color = if (!waiting && offlineSnapshot) tone.copy(alpha = 0.85f) else tone,
                            // 拆不出方法名时整条原文照抄,保留原 2 行;拆出后标题一行
                            maxLines = if (split == null) 2 else 1,
                        )
                        split?.second?.takeIf { it.isNotBlank() }?.let {
                            Text(
                                it,
                                fontSize = 11.sp,
                                color = if (!waiting && offlineSnapshot) tone.copy(alpha = 0.85f) else tone,
                                maxLines = 2,
                            )
                        }
                    }
                    // 自动恢复进行中不给"重试"—— 重复点只会打断它自己的重试节奏。
                    // 只有"彻底失败"和"页面加载失败"才需要用户推一把。
                    if (!waiting) {
                        TextButton(onClick = {
                            if (st.recoveryError != null) {
                                TurnSession.retryRecovery(threadId)
                            } else {
                                loadError = null
                                chatVm.send(ChatIntent.ClearSnapshotFlag)
                                reloadToken++
                            }
                        }) {
                            Text(
                                "重试",
                                fontSize = 11.5.sp,
                                color = PaEditorial.Primary,
                            )
                        }
                    }
                }
            }
        }

        // ---- 离线快照横幅(P1-4 阶段 D:断连进页用本地只读副本渲染)----
        // 与错误提示条分开:它不是"失败",是"明确知道自己在看旧数据"。
        // 输入栏同时停用(见下方输入区),恢复连接后 connectionEpoch 变化自动回到在线。
        if (offlineSnapshot) {
            val banner = offlineSnapshotBanner(offlineSnapshotAt)
            if (banner != null) {
                Row(
                    Modifier
                        .fillMaxWidth()
                        .padding(horizontal = midGutter, vertical = 2.dp)
                        .background(PaEditorial.AttentionContainer, RoundedCornerShape(13.dp))
                        .border(1.dp, OfflineBannerOutline, RoundedCornerShape(13.dp))
                        .padding(horizontal = 12.dp, vertical = 7.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Icon(
                        painterResource(AppR.drawable.ic_lucide_info), null,
                        tint = PaEditorial.Attention, modifier = Modifier.size(13.dp),
                    )
                    Spacer(Modifier.width(8.dp))
                    Text(
                        "$banner · 显示的是电脑端会话的本地副本,输入已停用",
                        Modifier.weight(1f),
                        fontSize = 11.5.sp, color = OfflineBannerText, maxLines = 2,
                    )
                    TextButton(onClick = {
                        chatVm.send(ChatIntent.ClearSnapshot)
                        reloadToken++
                    }) {
                    Text(
                        "重试",
                        fontSize = 11.5.sp,
                        color = PaEditorial.Primary,
                    )
                    }
                }
            }
        }

        // ---- 待审批入口卡(05 会话页;README:审批只在弹窗里决策)----
        // 这张卡只是**入口**(点开 PaApprovalDialog),卡上不放任何批准/拒绝按钮;
        // 弹窗关闭(暂不处理)后从这里或顶部身份行再次进入。请求与弹窗相位的状态机
        // 见上方"审批弹窗状态机"块。反问类请求有自己的常驻问题卡,不进这张入口。
        val pendingApprovalEntry = st.pendingApproval
        if (pendingApprovalEntry != null && !askIsAskAction(pendingApprovalEntry.action.name)) {
            ApprovalEntryCard(onOpen = { approvalDialogOpen = true })
        }

        // ---- 执行计划面板 ----
        // 【用户诉求】计划不再随时间线滚走:未完成时在会话原位置仍能看到步骤,
        // 跑完之后那张卡不移除,而是把它**提到会话上方**收成一个可展开的面板,
        // 需要回顾时点开,不需要时收起只留一行进度摘要。
        //
        // 【本轮修正 · 真因】这里原先还挂着 `&& AgentConnection.showThinking`。
        // 那是「显示思考过程」的开关,跟交互计划毫无关系 —— 用户一旦把它关掉,
        // 计划面板就跟着一起消失,现象正是"桌面 agent 明明有这个能力,却不显示"。
        // 计划该不该出现只取决于**有没有计划数据**,不该受其它显示开关连坐。
        if (latestPlan.isNotEmpty()) {
            PlanPanel(
                steps = latestPlan,
                draft = if (st.running) st.planStream else "",
                running = st.running,
                expanded = planPanelOpen,
                onToggle = { planPanelOpen = !planPanelOpen },
                outerGutter = midGutter,
            )
        }

        // ---- 流式正文节流快照(P1-22)----
        // delta 帧高频直喂 InAppMarkdown,即便解析已异步化(MarkdownText 的 produceState),
        // 高频重组 + 解析排队仍会让长流式累计 O(n²) 掉帧。这里把直喂改为 ~120ms 合并快照:
        // 中间帧只写 displayedStream,间隔内到达的 delta 自然合并,流式视觉延迟无感。
        // 【最终帧一致性兜底】两条路径不经 delay 立即追平:
        //   1) 流结束(!streaming);
        //   2) 正文被整段转存进 items(TurnSession 在工具调用开始时把 streamText 清空 ——
        //      快照必须同步清空,否则该段会在气泡里残留成"既在历史又在气泡"的重复)。
        // 且每次 delay 醒来都重读 st.streamText 的最新值(不沿用 collect 到的旧值),
        // 任何时刻快照最终必等于 streamText,尾巴不丢。
        var displayedStream by remember(st) { mutableStateOf("") }
        LaunchedEffect(st) {
            snapshotFlow { st.streaming to st.streamText }
                .collect { (running, text) ->
                    when {
                        !running || text.isEmpty() -> displayedStream = text
                        text != displayedStream -> {
                            delay(STREAM_RENDER_THROTTLE_MS)
                            displayedStream = st.streamText
                        }
                    }
                }
        }

        // ---- 消息列表(思考过程 / 工具调用都在时间线原位置展示)----
        // 【进入动画的武装记录】记住"上一帧组合时的条目数":仅在"末尾追加了新消息"
        // 的那一帧允许进入动画(见 MessageEnterAnimation)。null = 本页首帧,绝不播
        // (打开会话加载历史时整套静默出现);SideEffect 在组合后登记当前数量,
        // 回滚缩容/加载更早前插都不满足"末尾追加",不会误触发。
        val prevItemCount = remember(threadId) { mutableStateOf<Int?>(null) }
        // 「撤回排队」入口(pi 0.87.1 clear_queue):
        // 【能力位数据源】读函数级的 hostCaps 共享快照(与主页网格/设置页同一
        // 真源),门控纯函数 queueClearVisible 锁三重条件(agent==pi + 确有排队 +
        // daemon 宣告 queue_clear:pi)—— 旧 daemon / 非 pi 会话 / 没有排队都不出
        // 按钮,不显示点了必败的假入口。
        val queueClearOn = queueClearVisible(threadAgentId, st.queuedCount, hostCaps)
        // 按钮只挂在最后一条排队消息上:同一条队列一张入口,不逐条重复
        val lastQueuedIndex = st.items.indexOfLast { it.deliverState == DELIVERY_QUEUED }
        // 消息列表容器(Box):列表本体 + P0-1 的「↓ N 条新内容」悬浮胶囊同层,
        // 常规气泡与 hacker 终端块共用(锚定状态与胶囊都只此一份)。
        Box(Modifier.weight(1f)) {
        LazyColumn(
            Modifier
                .fillMaxSize()
                // 黑客终端骨架自带 16dp;编辑式骨架由 PaScreenScaffold 统一 23dp 页边距
                .padding(horizontal = if (PaHacker.isHacker) 16.dp else 0.dp)
                // 黑客终端模式:CRT 扫描线(2dp 周期暗线,alpha 4%,只碰绘制层);
                // 叠在内容之上,常规主题零影响(见 TerminalBlocks.terminalScanlines)。
                .then(if (PaHacker.isHacker) Modifier.terminalScanlines() else Modifier),
            state = listState,
            // 黑客终端模式:块间留白 22dp(Warp 口径:留白>分割线);常规主题 14dp 不动
            verticalArrangement = Arrangement.spacedBy(if (PaHacker.isHacker) 22.dp else 14.dp),
            contentPadding = PaddingValues(vertical = 12.dp),
        ) {
            // 【F-D 加载更早】历史被截断时在时间线顶部给出的增量入口;离线快照
            // (只读旧数据)不显示。点击后重拉更大的轮数并保持滚动锚点,见 loadEarlier。
            if (historyTruncated && !offlineSnapshot) {
                item(key = "load-earlier") {
                    Box(
                        Modifier.fillMaxWidth().padding(vertical = 2.dp),
                        contentAlignment = Alignment.Center,
                    ) {
                        if (loadingEarlier) {
                            CircularProgressIndicator(modifier = Modifier.size(18.dp), strokeWidth = 2.dp)
                        } else {
                            TextButton(onClick = { loadEarlier() }) {
                                Text(
                                    "加载更早",
                                    fontSize = 12.5.sp,
                                    color = PaEditorial.Primary,
                                )
                            }
                        }
                    }
                }
            }
            // 【PF4】contentType 按渲染形态分桶(见 chatItemContentType):滚动回收只
            // 同形态复用,不同形态不再互毁重建。无稳定 id(改 ChatItem/TurnSession 越权)
            // 故不加 key,位置匹配语义原样保留 —— 纯复用提示,不碰投影。
            itemsIndexed(
                st.items,
                contentType = { _, item -> chatItemContentType(PaHacker.isHacker, item) },
            ) { index, item ->
                // 进入动画只武装"新消息追加"的那一帧(见上方 prevItemCount 注释)
                val prevCount = prevItemCount.value
                val enterArm = prevCount != null &&
                    index == st.items.lastIndex &&
                    st.items.size > prevCount
                // R2-A:本条消息的「选择文本」模式。长按默认开菜单(见用户/回复气泡的
                // longPressMenu),选一段文字改为从菜单进入 —— 手柄拖选能力不丢,但不再
                // 与菜单抢同一个长按手势。滑出屏幕即复位(remember 跟随 item 生命周期)。
                var selectMode by remember { mutableStateOf(false) }
                LaunchedEffect(messageSelectArmed) {
                    // 比引用不比值:内容完全相同的两条消息只命中被长按的那一条
                    if (messageSelectArmed === item) {
                        selectMode = true
                        messageSelectArmed = null
                    }
                }
                MessageEnterAnimation(arm = enterArm) {
                    if (PaHacker.isHacker) {
                    // 黑客终端模式:主对话读作"终端会话"——四类 Block(输入/输出/工具/脚注),
                    // 映射走纯函数 terminalBlockKind(JVM 单测钉住);无终端形态的 item
                    // (thinking/codex_event)保持既有渲染,plan/usage 照旧不进时间线。
                    when (terminalBlockKind(item)) {
                        TerminalBlockKind.USER -> TerminalUserBlock(
                            item, uiScale,
                            // 「撤回排队」入口与常规主题同一门控:只挂最后一条排队消息
                            queueClearOn = queueClearOn && index == lastQueuedIndex,
                            onClearQueued = { clearQueuedInputs() },
                            // P0-2:daemon 拒收(overflow)时整条 Block 可点击重发,
                            // 与常规气泡同一 retryDelivery(失败级才武装,见组件内判定)
                            onRetryDelivery = { retryDelivery(item) },
                            attachmentsContent = if (item.attachments.isEmpty()) null else { {
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
                            } },
                        )
                        TerminalBlockKind.TOOL -> TerminalToolBlock(item, uiScale)
                        TerminalBlockKind.OUTPUT -> TerminalOutputBlock(
                            item.text, uiScale,
                            onOpenUrl = onOpenBrowser,
                        )
                        TerminalBlockKind.KEEP_EXISTING -> when (item.role) {
                            "thinking" -> if (AgentConnection.showThinking) {
                                ThinkingCard(item.text, thinkScale, streaming = false)
                            } else Unit
                            "codex_event" -> item.codexEvent?.let { ev ->
                                CodexEventCard(
                                    event = ev,
                                    uiScale = uiScale,
                                    guardianOverridden = st.guardianOverridden.contains(ev.reviewId),
                                    onApproveGuardian = { approveGuardian(it) },
                                    onOpenSettings = onOpenSettings,
                                )
                            }
                            else -> Unit
                        }
                        TerminalBlockKind.SKIP -> Unit
                    }
                    } else when (item.role) {
                    "user" -> Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.End) {
                        // 编辑式(05):用户消息 = 石墨重点面气泡 + 白字,投递徽标挂在气泡
                        // 下方右对齐(待发送→已送达→执行中;失败保留内容可重试);
                        // 黑客终端保持旧气泡(AccentDim 终端块)与原徽标位(气泡内)。
                        Column(Modifier.widthIn(max = 320.dp), horizontalAlignment = Alignment.End) {
                        Column(
                            Modifier
                                .widthIn(max = 300.dp)
                                // R2-A:长按气泡 = 上下文菜单(复制全文/选择文本/引用回复,
                                // 失败级再加「重试发送」)。P0-2 失败级(队列满拒收)的
                                // "点按重发"保留为单击动作(longPressMenu 契约:挂了它就
                                // 不再叠加 clickable,单击经 onClick 传入);其余投递状态
                                // 点了没有可重试的东西,不武装点按。
                                .longPressMenu(
                                    messageMenu,
                                    onClick = if (deliveryRetryable(item.deliverState) && !offlineSnapshot) {
                                        { retryDelivery(item) }
                                    } else null,
                                )
                                    // 编辑式:石墨重点面(#1E2835)承载用户输入,与瓷白助手正文
                                    // 拉开层级;黑客终端模式:圆角收 6dp 读"终端块"(契约见 PaHacker)。
                                    .background(
                                        if (PaHacker.isHacker) PaEditorial.PrimaryContainer else PaEditorial.Graphite,
                                        if (PaHacker.isHacker) RoundedCornerShape(6.dp)
                                        else RoundedCornerShape(20.dp, 7.dp, 20.dp, 20.dp),
                                    )
                                    .padding(horizontal = 13.dp, vertical = 11.dp),
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
                                    // R2-A:长按默认开上下文菜单;要拖选一段从菜单进
                                    // 「选择文本」(与回复气泡同一交互,见 assistant 分支注释)
                                    if (selectMode) {
                                        SelectionContainer {
                                            Text(
                                                item.text,
                                                color = if (PaHacker.isHacker) PaEditorial.Ink else PaEditorial.OnGraphite,
                                                fontSize = (14 * uiScale).sp,
                                                lineHeight = (21 * uiScale).sp,
                                                // 黑客终端模式:等宽字体是风格的本体(契约见 PaHacker)。
                                                fontFamily = if (PaHacker.isHacker) PaHacker.mono else null,
                                            )
                                        }
                                    } else {
                                        Text(
                                            item.text,
                                            color = if (PaHacker.isHacker) PaEditorial.Ink else PaEditorial.OnGraphite,
                                            fontSize = (14 * uiScale).sp,
                                            lineHeight = (21 * uiScale).sp,
                                            // 黑客终端模式:等宽字体是风格的本体(契约见 PaHacker)。
                                            fontFamily = if (PaHacker.isHacker) PaHacker.mono else null,
                                        )
                                    }
                                    if (selectMode) {
                                        Text(
                                            "完成选择",
                                            Modifier
                                                .pressable { selectMode = false }
                                                .padding(top = 4.dp),
                                            fontSize = 10.5.sp,
                                            fontWeight = FontWeight.SemiBold,
                                            color = ChatAccentColor,
                                        )
                                    }
                                }
                                // 黑客终端模式:投递徽标留在气泡内(旧位置,行为不变);
                                // 编辑式徽标移到气泡下方(见外层 Column 尾部)。
                                if (PaHacker.isHacker && item.deliverState.isNotEmpty()) {
                                    val visual = deliveryVisual(item.deliverState, item.queuePos)
                                    if (visual.label.isNotEmpty()) {
                                        Spacer(Modifier.height(2.dp))
                                        Row(verticalAlignment = Alignment.CenterVertically) {
                                            DeliveryStatusBadge(item.deliverState, item.queuePos)
                                        }
                                    }
                                }
                            }
                            // 编辑式投递徽标 + 「撤回排队」(四态措辞对齐设计:待发送→已送达→
                            // 执行中;失败保留内容可重试)。QueueFlushed 后标签被 TurnSession
                            // 收掉,这条消息自动变回普通用户消息。
                            if (!PaHacker.isHacker && item.deliverState.isNotEmpty()) {
                                val copy = editorialDeliveryCopy(item.deliverState, item.queuePos)
                                if (copy.word.isNotEmpty()) {
                                    Spacer(Modifier.height(3.dp))
                                    Row(verticalAlignment = Alignment.CenterVertically) {
                                        EditorialDeliveryBadge(item.deliverState, item.queuePos)
                                        // 「撤回排队」:只挂在最后一条排队消息的回执行上,
                                        // 门控见 queueClearOn 的注释 —— 撤回是对
                                        // 还没执行的输入做反悔,与"终止任务"同级的
                                        // 干预动作,用红字让分量看得见。
                                        if (queueClearOn && index == lastQueuedIndex) {
                                            Text(
                                                "撤回排队",
                                                Modifier
                                                    .pressable(haptic = true) { clearQueuedInputs() }
                                                    .padding(horizontal = 8.dp, vertical = 2.dp),
                                                fontSize = 10.sp,
                                                color = ChatDeniedColor,
                                            )
                                        }
                                    }
                                }
                            }
                        }
                    }
                    // 思考过程:可折叠卡片(字号跟随"思考显示大小"设置);
                    // 受全局"显示思考过程"开关控制,关闭时只看正文与结果
                    "thinking" -> if (AgentConnection.showThinking) {
                        ThinkingCard(item.text, thinkScale, streaming = false)
                    } else Unit
                    // 执行计划:已提到会话上方的可折叠面板里(见 PlanPanel),
                    // 时间线里不再重复一张卡 —— 避免同一步骤在屏幕上有两份。
                    "plan" -> Unit
                    // 工具调用:一次调用一条卡,含参数/输出/耗时,可展开看详情
                    "tool" -> ToolCallCard(item, uiScale)
                    // 后台 Codex/MCP 事件只影响右上角状态点，不再把诊断卡片
                    // 连续插入会话正文。
                    // 【本轮修正】以前这里是一句 `-> Unit`,把所有 codex 事件
                    // 一律丢弃 —— 连"审查者驳回了动作"这种用户必须知情(而且可以
                    // 推翻)的事件也一起丢了。现在按 shouldShowCodexEventInTimeline
                    // 的分类渲染:诊断类在源头就不进 items,进 items 的都要看得见。
                    "codex_event" -> item.codexEvent?.let { ev ->
                        CodexEventCard(
                            event = ev,
                            uiScale = uiScale,
                            guardianOverridden = st.guardianOverridden.contains(ev.reviewId),
                            onApproveGuardian = { approveGuardian(it) },
                            onOpenSettings = onOpenSettings,
                        )
                    }
                    "usage" -> Unit
                    else -> Column(
                        Modifier
                            .fillMaxWidth()
                            // R2-A:长按回复 = 上下文菜单(复制全文/选择文本/引用回复)。
                            // 【与旧交互的衔接】此前正文套常驻 SelectionContainer,长按
                            // 直接进拖选 —— "长按复制整条"因此被移除过一次(手势互抢)。
                            // 现在长按优先开菜单,拖选改为菜单「选择文本」进入的显式模式
                            // (见本 item 顶部 selectMode):两个能力都在,不再互抢手势,
                            // 与 ChatGPT/Claude 官方 App 的消息菜单同构。
                            .longPressMenu(messageMenu) { messageMenuTarget = item },
                    ) {
                        // 选择模式:手柄拖选任意片段,系统工具栏给"复制/全选";
                        // 平时渲染普通正文,长按交给菜单。「完成选择」退出。
                        if (selectMode) {
                            SelectionContainer {
                                InAppMarkdown(
                                    text = item.text,
                                    modifier = Modifier.fillMaxWidth(),
                                    fontSize = (if (PaHacker.isHacker) 15f else 14f) * uiScale,
                                    onOpenUrl = onOpenBrowser,
                                )
                            }
                        } else {
                            InAppMarkdown(
                                text = item.text,
                                modifier = Modifier.fillMaxWidth(),
                                fontSize = (if (PaHacker.isHacker) 15f else 14f) * uiScale,
                                onOpenUrl = onOpenBrowser,
                            )
                        }
                        if (selectMode) {
                            Text(
                                "完成选择",
                                Modifier
                                    .pressable { selectMode = false }
                                    .padding(top = 4.dp),
                                fontSize = 10.5.sp,
                                fontWeight = FontWeight.SemiBold,
                                color = ChatAccentColor,
                            )
                        }
                        // 消息里如果给了电脑上的文件路径,唯一的动作是"用浏览器打开"
                        // —— 会话正文只负责展示输出;要存到手机,在浏览器顶栏点"下载到手机"。
                        val pcFiles = remember(item.text) { extractPcPaths(item.text) }
                        if (pcFiles.isNotEmpty()) {
                            pcFiles.take(3).forEach { path ->
                                val fileName = path.substringAfterLast('/').substringAfterLast('\\')
                                Row(
                                    Modifier.padding(top = 6.dp).fillMaxWidth(),
                                    verticalAlignment = Alignment.CenterVertically,
                                ) {
                                    Row(
                                        Modifier.pressable(haptic = true) {
                                            onOpenBrowser("pc:" + path)
                                        },
                                        verticalAlignment = Alignment.CenterVertically,
                                    ) {
                                        Icon(
                                            painterResource(AppR.drawable.ic_lucide_globe), null,
                                            tint = ChatAccentColor, modifier = Modifier.size(13.dp),
                                        )
                                        Spacer(Modifier.width(5.dp))
                                        Text("在浏览器打开 $fileName", fontSize = 10.5.sp, color = ChatAccentColor)
                                    }
                                }
                            }
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
            // 流式执行计划:已由上方 PlanPanel 承担(它同时兼做收起的进度条),
            // 这里不再重复渲染,否则同一步骤会出现两份。
            if (st.streaming) {
                item(key = "streaming-bubble") {
                    if (PaHacker.isHacker) {
                    // 黑客终端模式:流式输出块(块尾 braille 转轮,80ms/帧);
                    // 等首 token 时正文为空,TerminalOutputBlock 只渲染转轮本身。
                    TerminalOutputBlock(
                        st.streamText, uiScale,
                        onOpenUrl = onOpenBrowser,
                        streaming = true,
                    )
                    } else {
                    // 【流式两相切换】等首 token(三点) → 正文流式:AnimatedContent +
                    // spring 过渡(淡入+轻微上移,可打断)。AnimatedContent 自带尺寸
                    // 过渡,相位切换时高度不跳变;正文帧间更新不重播转场(同一 targetState)。
                    // 相位跟节流快照走(displayedStream 而非 streamText):首帧正文
                    // 就绪前保持"三点"相位,不会闪一帧空正文。
                    AnimatedContent(
                        targetState = displayedStream.isEmpty(),
                        transitionSpec = {
                            (fadeIn(PaMotion.emphasized()) +
                                slideInVertically(PaMotion.emphasized()) { it / 8 })
                                .togetherWith(fadeOut(PaMotion.standard()))
                        },
                        label = "streamPhase",
                    ) { waiting ->
                        if (waiting) {
                            // 等待首个 token:三点跳动"正在输入"指示(替代旧的光标闪烁)。
                            // 编辑式:瓷白卡面+细线(旧玻璃材质保留给黑客终端以外的主题切换
                            // 会打架,这里按模式取面);三点自身只动 graphicsLayer(见 TypingDots)。
                            Row(Modifier.fillMaxWidth()) {
                                Box(
                                    Modifier
                                        .then(
                                            if (PaHacker.isHacker) Modifier.paGlass(RoundedCornerShape(18.dp))
                                            else Modifier
                                                .background(PaEditorial.Surface, RoundedCornerShape(18.dp))
                                                .border(1.dp, PaEditorial.Hairline, RoundedCornerShape(18.dp)),
                                        )
                                        .padding(horizontal = 16.dp, vertical = 14.dp),
                                ) {
                                    TypingDots()
                                }
                            }
                        } else {
                            // 流式输出中:正文 + 行内小号三点跟随(仍在生成,比光标更自然)
                            // 【P1-22】喂节流快照 displayedStream,不再逐 delta 帧直喂全文
                            Column(Modifier.fillMaxWidth()) {
                                InAppMarkdown(
                                    text = displayedStream,
                                    modifier = Modifier.fillMaxWidth(),
                                    fontSize = (if (PaHacker.isHacker) 15f else 14f) * uiScale,
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
            }
            // 黑客终端模式:回合脚注(④)—— 轮次终局后挂在列表末尾右对齐。
            // 【不造数据】只在"有真实终局知识"时出现:运行中/恢复中绝不显示;
            // 有失败信息给 ✗,有真实 token 用量给 ✓ N tok,什么都没有只给 ✓。
            if (PaHacker.isHacker && !st.running && !st.recovering && st.recoveryError == null &&
                (st.turnUsage != null || st.errorMessage != null)
            ) {
                item(key = "turn-footnote") {
                    TerminalFootnote(
                        formatTurnFootnote(
                            success = st.errorMessage == null,
                            elapsedMs = 0.0, // 回合时长无现成数据,不编造
                            outputTokens = st.turnUsage?.outputTokens ?: 0,
                        ),
                    )
                }
            }
        }
        // 【R1 收口】hermes 等会话首次进入的两条"空白路"补齐(时间线 Box 的同层
        // 覆盖层,不进 LazyColumn —— 头部 item 会让既有的 historyListOffset 滚动
        // 锚定差一位;而骨架/空态只在 st.items 为空时出现,覆盖层无遮挡问题,
        // 列表与覆盖层在同一帧原子切换,不会闪叠):
        //  · 历史在途 → 消息气泡骨架(与真实气泡同轮廓,切内容不跳变;整块折叠为
        //    单条「正在加载」读屏播报);
        //  · 历史加载完且确实为空 → 空态(事实 + 下一步),不发愣。
        // 判定走纯函数(shouldShowTimelineSkeleton / shouldShowTimelineEmptyState,
        // TimelineSkeletonEmptyTest 钉死):报错横幅、离线副本、运行/恢复中都不摆。
        if (shouldShowTimelineSkeleton(
                itemsEmpty = st.items.isEmpty(),
                historyLoaded = st.historyLoaded,
                hasLoadError = loadError != null,
                offlineSnapshot = offlineSnapshot,
            )
        ) {
            // 气泡实际高度 ≈82dp,按它推条数,切到真实内容时高度不跳
            PaSkeletonList(SkeletonKind.MESSAGE_BUBBLE, perRowHeight = 82.dp)
        }
        if (shouldShowTimelineEmptyState(
                itemsEmpty = st.items.isEmpty(),
                historyLoaded = st.historyLoaded,
                hasLoadError = loadError != null,
                offlineSnapshot = offlineSnapshot,
                running = st.running,
                recovering = st.recovering,
            )
        ) {
            val (emptyHead, emptySub) = timelineEmptyStateCopy(selectedAgentLabel(threadAgentId))
            Column(
                Modifier.fillMaxWidth().padding(top = 96.dp),
                horizontalAlignment = Alignment.CenterHorizontally,
            ) {
                Icon(
                    painterResource(AppR.drawable.ic_lucide_message_square), null,
                    tint = PaEditorial.Subtle, modifier = Modifier.size(28.dp),
                )
                Spacer(Modifier.height(10.dp))
                Text(
                    emptyHead,
                    fontSize = 14.sp, lineHeight = 20.sp,
                    fontWeight = FontWeight.SemiBold,
                    color = PaEditorial.Muted,
                )
                Spacer(Modifier.height(4.dp))
                Text(
                    emptySub,
                    fontSize = 12.sp, lineHeight = 18.sp,
                    color = PaEditorial.Subtle,
                    textAlign = TextAlign.Center,
                )
            }
        }
            // 历史消息快速查看(视频样板二版+R1 实时刷选):左缘垂直刻度条 —— 每条
            // 用户消息一个刻度点(Canvas 一次绘制);轻点开完整列表浮层;按住拖动 =
            // 预览气泡跟手 + **列表实时滚动到手指所指**(拖到哪看到哪),松手精确锚定。
            HistoryTickBar(
                count = historyEntries.size,
                currentIndex = historyCurrentIndex,
                drag = historyDrag,
                onOpenPanel = { showHistoryPanel = true },
                onJump = { entryIndex -> jumpToHistoryTick(entryIndex) },
                onScrub = { entryIndex -> scrubToHistoryTick(entryIndex) },
                onScrubEnd = { programmaticScroll.set(false) },
                dayStarts = historyDayStarts,
                modifier = Modifier
                    .align(Alignment.CenterStart)
                    // 【真机修正 2026-10-01】本 Box 已被 PaScreenScaffold 收进 23dp
                    // 页边距内,直接 align 会把刻度条推进正文列(实测 bounds x=63px
                    // 起,正压在助手文本上)——负 offset 拉回屏幕真实左缘,视觉
                    // 10dp 全程悬在边距区,这才是不挡字的硬保证。
                    .offset(x = -PaEditorial.Space.PageMargin),
            )
            // 按住拖动的紧凑预览气泡:只在预览期间存在(scale+fade 出入场),
            // 松手即收 —— 平时零遮挡。align TopStart 让气泡 Y 与刻度条同坐标系;
            // 同样负 offset 出页边距,内部 16dp 间隙才是"距屏幕左缘 16dp"。
            HistoryQuickBubble(
                drag = historyDrag,
                entries = historyEntries,
                modifier = Modifier
                    .align(Alignment.TopStart)
                    .offset(x = -PaEditorial.Space.PageMargin),
            )
            // P0-1「↓ N 条新内容」胶囊:仅在用户离底且确有未读新块时浮现(PaEditorial.Primary 底)。
            // 点击平滑滚回底部、恢复跟随并清零未读(轻触觉由 pressable 自带)——
            // 流式输出从此不再把上滑阅读历史的用户拽回来。
            // 【全限定名】这里在 Column 作用域内,ColumnScope.AnimatedVisibility 扩展
            // 会抢走隐式接收者解析;盒子层要的是顶层函数,写全限定名绕开歧义。
            androidx.compose.animation.AnimatedVisibility(
                visible = unreadBlocks > 0 && !followBottom,
                modifier = Modifier
                    .align(Alignment.BottomEnd)
                    .padding(end = 20.dp, bottom = 12.dp),
                enter = fadeIn(PaMotion.emphasized()) +
                    slideInVertically(PaMotion.emphasized()) { it / 2 },
                exit = fadeOut(PaMotion.standard()),
                label = "unreadPill",
            ) {
                Row(
                    Modifier
                        .pressable(haptic = true) {
                            scope.launch {
                                followBottom = true
                                unreadBlocks = 0
                                val total = st.items.size + if (st.streaming) 1 else 0
                                if (total > 0) listState.animateScrollToItem(total - 1)
                            }
                        }
                        .background(ChatAccentColor, RoundedCornerShape(14.dp))
                        .padding(horizontal = 12.dp, vertical = 7.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Icon(
                        painterResource(AppR.drawable.ic_lucide_chevron_down), null,
                        tint = PaEditorial.OnPrimary, modifier = Modifier.size(12.dp),
                    )
                    Spacer(Modifier.width(4.dp))
                    Text(
                        "${unreadBlocks.coerceAtLeast(1)} 条新内容",
                        color = PaEditorial.OnPrimary,
                        fontSize = 11.5.sp,
                        fontWeight = FontWeight.Medium,
                    )
                }
            }
        }
        // 组合完成后登记本帧条目数(下一帧据此判定"是否末尾追加了新消息")
        SideEffect { prevItemCount.value = st.items.size }
        } // chatMiddle lambda 到此结束(两条骨架共用段的下边界)

        // ---- 编辑式主机状态槽(05/12):只承载离线件(主机胶囊+断线横幅) ----
        // 会话身份已上移到标题行右侧(ChatIdentityBadge,2026-09-30 布局修复:
        // "Agent·工作区+状态点"跟在标题旁,不再单独占一张大卡);本槽仅在断线时
        // 出现内容 —— 未连接时状态仍显示上次更新,绝不出现"正在执行"。
        val chatHostStatus: @Composable () -> Unit = {
            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                // 离线(12):主机胶囊 OFFLINE + 上次更新时间 —— 未连接时状态仍显示
                // 上次更新,绝不出现"正在执行"(hostChipCaption 的构造性保证);
                // 断线横幅保留既有机制(自动重连计数/排队条数/手动重试),草稿不丢。
                // 主机胶囊只认"已知断网"(down);横幅可见性走三态状态机 —— checking
                // 断过也保持可见,两者口径不同是有意的(见上方 chatBanner 注释)。
                if (connDown) {
                    PaHostChip(
                        hostName = hostName.ifBlank { "电脑" },
                        link = PaHostLink.OFFLINE,
                        lastUpdateText = offlineSnapshotAt.takeIf { it > 0 }?.let {
                            java.text.SimpleDateFormat("HH:mm", java.util.Locale.getDefault())
                                .format(java.util.Date(it))
                        },
                    )
                }
                if (chatBanner.visible) {
                    ChatOfflineBanner(
                        copy = connectionBannerCopy(chatBanner.attempts, TurnSession.totalQueuedInputs()),
                        onRetry = { reloadToken++ },
                    )
                }
            }
        }

        // ---- 底部输入区 lambda:两条骨架共用同一份内容,外壳按骨架取底安全/边距 ----
        val chatComposerBody: @Composable ColumnScope.() -> Unit = {
        // ---- 输入区(对标图五):附件行 + 双行胶囊 [输入 / + 审批 模型·强度 发送] ----
            // 已上传附件 chips:出现/收起走 AnimatedContent(展开+淡入 / 收缩+淡出,
            // 全 spring 可打断)—— 编辑条"带附件 ↔ 无附件"两个状态之间不再硬跳。
            AnimatedContent(
                targetState = attachments.isNotEmpty(),
                transitionSpec = {
                    (fadeIn(PaMotion.emphasized()) + expandVertically(PaMotion.emphasized()))
                        .togetherWith(fadeOut(PaMotion.standard()) + shrinkVertically(PaMotion.standard()))
                },
                label = "attachRow",
            ) { hasAttachments ->
                if (hasAttachments) {
                    Column {
                Row(
                    Modifier.fillMaxWidth().horizontalScroll(rememberScrollState()),
                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                ) {
                    attachments.forEach { at ->
                        Row(
                            Modifier
                                // 单击预览 / 长按复制完整文件名(复用消息长按复制的写法)
                                .pressableLong(
                                    onClick = { openAttachment(at) },
                                    onLongPress = { copyMessage(ctx, at.name) },
                                )
                                .background(PaEditorial.Raised, RoundedCornerShape(10.dp))
                                // PA-REAL-0004 收尾:chip 命中高度抬到 ≥48dp(原 ≈29dp)。
                                // 垂直 padding 7→0,由"移除叉"的 48dp 触控箱顶起;胶囊圆角/
                                // 配色/内容排布不变,仅整体增高到最小触控标准。
                                .heightIn(min = INPUT_TOUCH_TARGET_DP.dp)
                                .padding(horizontal = 10.dp),
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
                                tint = PaEditorial.Primary, modifier = Modifier.size(13.dp),
                            )
                            Spacer(Modifier.width(6.dp))
                            // 文件名最多 3 行、可换行;宽度封顶以强制换行(横向滚动行里若不封顶,
                            // 文本会一直延伸并被视口裁掉)。真需省略时保留扩展名(见 middleEllipsize)。
                            Text(
                                middleEllipsize(at.name, ATTACH_CHIP_NAME_MAX),
                                Modifier.widthIn(max = 220.dp),
                                fontSize = 11.sp, color = PaEditorial.Ink,
                                maxLines = 3, softWrap = true, overflow = TextOverflow.Ellipsis,
                            )
                            Spacer(Modifier.width(6.dp))
                            // PA-REAL-0004 收尾:移除叉命中区 13→48dp(原叉仅 13dp,chip 本体
                            // 点击是"预览"不能替代移除,故必须独立放大);叉视觉 13dp 居中不变。
                            // 前置 6dp Spacer 保留,触控箱自带居中留白即视觉间距。
                            Box(
                                Modifier
                                    .sizeIn(minWidth = INPUT_TOUCH_TARGET_DP.dp, minHeight = INPUT_TOUCH_TARGET_DP.dp)
                                    .pressable { attachments = attachments - at },
                                contentAlignment = Alignment.Center,
                            ) {
                                Icon(
                                    painterResource(AppR.drawable.ic_lucide_x), null,
                                    tint = PaEditorial.Subtle,
                                    modifier = Modifier.size(13.dp),
                                )
                            }
                        }
                    }
                    }
                        Spacer(Modifier.height(8.dp))
                    }
                }
            }

            // 【生效模型名】已上提到函数作用域(effectiveModel 声明处):
            // hacker 模式的 tmux 状态条在输入栏上方,与本栏共用同一个 modelName。

            var approvalMenu by remember { mutableStateOf(false) }
            // 【聚焦辉光】输入框聚焦/失焦经 spring 淡入淡出 1dp 主色描边:
            // "正在输入"有一圈安静的状态光,而不是生硬的有/无两态。
            val inputInteraction = remember { MutableInteractionSource() }
            val inputFocused by inputInteraction.collectIsFocusedAsState()
            val focusGlow by animateFloatAsState(
                targetValue = if (inputFocused) 1f else 0f,
                animationSpec = PaMotion.standard(),
                label = "inputFocusGlow",
            )
            // 编辑式:瓷白输入卡 + 细线(18dp 圆角);黑客终端保持玻璃材质(26dp)。
            val composerShape = if (PaHacker.isHacker) RoundedCornerShape(26.dp) else RoundedCornerShape(18.dp)
            Column(
                Modifier
                    .fillMaxWidth()
                    .then(
                        if (PaHacker.isHacker) Modifier.paGlass(RoundedCornerShape(26.dp), hairline = true)
                        else Modifier
                            .background(PaEditorial.Surface, composerShape)
                            .border(1.dp, PaEditorial.Hairline, composerShape),
                    )
                    .border(
                        1.dp,
                        PaEditorial.Primary.copy(alpha = 0.30f * focusGlow),
                        composerShape,
                    )
                    .animateContentSize(PaMotion.emphasized()),
            ) {
                // [斜杠命令卡面(纵向)] 会话是面板 agent(zcode/codex/pi 三源)且输入以 "/"
                // 开头时,列出该线程可用的命令 + 内置伪命令(zcode=会话快照 slashCommands[],
                // codex=skills 映射,pi=get_commands 真名映射,daemon 按 agent_id 路由同一
                // RPC;内置伪命令由 slashCandidatesFor 置顶合并,/压缩 认 compact:<id>、
                // /计划 认 review:<id> 能力位,缺位不进列表)。点选按动作分流:
                //   - FILL 普通命令 → applySlashPick 回填(zcode/pi→"/命令 " 保留后续文字,
                //     pi 真命令由 pi 原生展开;codex→自然语言触发技能);
                //   - GOAL/COMPACT/REVIEW 内置伪命令 → 执行动作(弹目标底栏 / doCompact /
                //     doStartReview),不填输入框,并把敲出的 "/词元" 清掉
                //     (stripSlashToken,避免把伪命令名当正文发出;REVIEW 的词元后
                //     残留文字作为审查重点 focus 下发)。
                // 视觉:纵向可滑卡面(最大 320dp,每行名称+说明完整不截断)—— 取代旧
                // "横滚 8 条"候选条,解决 codex/pi 长列表"看不全"的问题;
                // 可见性/过滤/合并/回填纯逻辑在 SlashCommandsLogic.kt。
                val slashCandidates = if (slashPanelVisible(threadAgentId, input, slashCommands, offlineSnapshot, hostCaps)) {
                    filterSlashCandidates(slashCandidatesFor(threadAgentId, slashCommands, hostCaps), input)
                } else {
                    emptyList()
                }
                if (slashCandidates.isNotEmpty()) {
                    Text(
                        "${slashCandidates.size} 个命令 · 上下滑动查看",
                        fontSize = 10.sp,
                        color = ChatSubtleColor,
                        modifier = Modifier.padding(start = 16.dp, top = 8.dp),
                    )
                    Column(
                        Modifier
                            .fillMaxWidth()
                            .heightIn(max = minOf(180, LocalConfiguration.current.screenHeightDp / 4).dp)
                            .verticalScroll(rememberScrollState())
                            .padding(start = 10.dp, end = 10.dp, top = 10.dp),
                        verticalArrangement = Arrangement.spacedBy(6.dp),
                    ) {
                        slashCandidates.forEach { c ->
                            // 每行一张卡:命令名 + 完整说明(不设 maxLines,描述长也看得全)。
                            Column(
                                Modifier
                                    .fillMaxWidth()
                                    .pressable {
                                        when (c.action) {
                                            SlashAction.GOAL -> {
                                                setInput(stripSlashToken(input))
                                                slashGoalSheet = true
                                            }
                                            SlashAction.COMPACT -> {
                                                setInput(stripSlashToken(input))
                                                doCompact()
                                            }
                                            SlashAction.REVIEW -> {
                                                // "/计划":词元后已敲的文字作为审查重点
                                                // (通常为空,daemon 有默认口径),清空输入框后发起。
                                                val reviewFocus = stripSlashToken(input)
                                                setInput("")
                                                doStartReview(reviewFocus)
                                            }
                                            SlashAction.FILL -> setInput(applySlashPick(input, c, threadAgentId))
                                        }
                                    }
                                    .background(
                                        PaEditorial.Raised,
                                        RoundedCornerShape(12.dp),
                                    )
                                    .padding(horizontal = 12.dp, vertical = 8.dp),
                            ) {
                                Text(
                                    "/" + c.name,
                                    fontSize = 12.sp,
                                    fontWeight = FontWeight.SemiBold,
                                    color = ChatAccentColor,
                                )
                                if (c.description.isNotBlank()) {
                                    Spacer(Modifier.height(4.dp))
                                    Text(
                                        c.description,
                                        fontSize = 11.sp,
                                        color = ChatMutedColor,
                                    )
                                }
                            }
                        }
                    }
                }
                // 【未知助手空态】会话归属 agent 已知但不在三源面板名单(zcode/codex/pi
                // 之外,如旧库残值/将来的第 4 个 agent)且敲出了 "/" → 人话空态,不静默
                // (AUDIT §3.3:遇未知值给人话,禁止"未知/错误"字样)。agentId 还没拉到
                // (加载中)与离线态都不提示,避免闪错/与离线横幅重复;纯逻辑见
                // slashUnknownAgentHintVisible(SlashCommandsLogic.kt)。
                if (slashUnknownAgentHintVisible(threadAgentId, input, offlineSnapshot)) {
                    Text(
                        SLASH_UNKNOWN_AGENT_HINT,
                        fontSize = 11.sp,
                        color = ChatSubtleColor,
                        modifier = Modifier.padding(start = 16.dp, top = 8.dp, bottom = 4.dp),
                    )
                }
                // 第一行:输入框 + 整个输入栏右上角的"展开编辑"小标识
                // 【为什么放右上角】它是低频辅助动作。放进工具行会和发送键 / 模型 chip
                // 抢注意力,用户明确要求"不要在右侧,也不要在发送键左边"。挪到整个胶囊的
                // 右上角后:不起眼(小号 + TextTertiary)、又始终可达,不占工具行的横向预算。
                // 【动画 ③·文本落框】识别文本并入时行数可能 0→1/1→2:外层 Box 挂
                // animateContentSize,高度变化平滑过渡(轻量,不引入额外布局);叠加
                // BasicTextField 上一次 alpha "落定"脉动,构成"文本飞入输入框"的过渡感。
                val voiceLandPulse = remember { Animatable(1f) }
                LaunchedEffect(voiceLandedTick) {
                    if (voiceLandedTick > 0) {
                        voiceLandPulse.snapTo(0.45f)
                        // 落定脉动走 spring 令牌:中途再收到语音片段,从当前透明度续走不跳变
                        voiceLandPulse.animateTo(1f, PaMotion.emphasized())
                    }
                }
                Box(Modifier.fillMaxWidth().animateContentSize()) {
                    // 【行数跟踪】展开编辑按钮只在文本占超过 2 行时出现 —— 输入区右上角
                    // 常驻 ↗ 与工具行"+"一上一下两个浮点,短文本时像两个入口互相打架
                    // (L2 真机挑刺);长文本才是它存在的时刻。行数取自真实排版
                    // (TextLayoutResult.lineCount,含自动换行),不按字符数猜。
                    var inputWrapLines by remember { mutableStateOf(1) }
                    BasicTextField(
                        value = input,
                        onValueChange = { setInput(it) },
                        onTextLayout = { inputWrapLines = it.lineCount },
                        modifier = Modifier
                            .fillMaxWidth()
                            // 【动画 ③】文本落定脉动:识别文本并入的一瞬整框 45%→100% 透明度
                            // (spring ~0.3s),只动绘制层(alpha),不触发重新布局,开销可忽略
                            .graphicsLayer { alpha = voiceLandPulse.value }
                            // 右侧预留 30dp:保证右上角图标不会压住光标 / 末尾文字
                            .padding(start = 18.dp, end = 30.dp, top = 12.dp, bottom = 12.dp)
                            .heightIn(min = 24.dp),
                        // 黑客终端模式:输入框字体等宽(JetBrains Mono,见 TerminalFont)
                        textStyle = TextStyle(
                            fontSize = 15.sp,
                            color = ChatInkColor,
                            fontFamily = if (PaHacker.isHacker) TerminalFont.chatFont else null,
                        ),
                        // 聚焦态喂给外层胶囊的辉光描边(见上方 inputFocused)
                        interactionSource = inputInteraction,
                        cursorBrush = SolidColor(ChatAccentColor),
                        maxLines = 4,
                        // 【P1-4】running 不再锁输入(运行中发消息=插入/排队);
                        // 只在离线快照态停用 —— 离线发不出去,别让用户白打一段话。
                        enabled = true,
                        decorationBox = { inner ->
                            // 黑客终端模式:输入非空时行首渲染 `❯ ` 提示符装饰(PaEditorial.Primary);
                            // 空输入显示 hint,不显示 ❯(见 composerShowsPrompt 纯函数)。
                            Row(verticalAlignment = Alignment.Top) {
                                if (PaHacker.isHacker && composerShowsPrompt(input)) {
                                    Text(
                                        "❯ ",
                                        color = ChatAccentColor,
                                        fontSize = 15.sp,
                                        fontWeight = FontWeight.Bold,
                                        fontFamily = TerminalFont.chatFont,
                                    )
                                }
                                Box {
                                    if (input.isEmpty()) {
                                        Text(
                                            when {
                                                offlineSnapshot -> "离线中，可继续编辑草稿；重连后发送…"
                                                // 【P4 Task 6】识别进行中:placeholder 优先展示引擎的
                                                // 部分转写(实时反馈);无字时按相给占位 ——
                                                // 停止后的 STOPPING/TRANSCRIBING(端侧转写窗口)
                                                // 显示"转写中…",出稿后回 IDLE 占位符切回常规文案。
                                                voiceListening ->
                                                    // 【P4 Task 6】识别中的实时反馈:优先展示引擎部分转写,
                                                    // 没有字时按相给占位(停止后 TRANSCRIBING=转写中)。
                                                    voiceState.partialText.ifBlank {
                                                        if (voiceState.phase == VoicePhase.TRANSCRIBING ||
                                                            voiceState.phase == VoicePhase.STOPPING
                                                        ) {
                                                            "转写中…"
                                                        } else {
                                                            "语音输入中…"
                                                        }
                                                    }
                                                // 黑客终端模式:hint 改为指令口吻(规格文案)
                                                PaHacker.isHacker -> "输入指令…"
                                                else -> "继续输入,或询问当前进度…"
                                            },
                                            fontSize = 15.sp,
                                            color = ChatSubtleColor,
                                            fontFamily = if (PaHacker.isHacker) TerminalFont.chatFont else null,
                                        )
                                    }
                                    inner()
                                }
                            }
                        },
                    )
                    // 展开编辑:内容超过 EXPAND_HINT_THRESHOLD 时轻微提亮到 PaEditorial.Primary,
                    // 保留原有"长文本可展开大窗"的提示语义;平时保持低调(TextTertiary)。
                    // 【条件露出】文本 ≤2 行时整块不渲染(见上方 inputWrapLines)——
                    // 只在真的需要大窗编辑时出现,输入区右上角不再常驻一个浮点。
                    if (inputWrapLines > 2) {
                    Box(
                        Modifier
                            .align(Alignment.TopEnd)
                            .padding(end = 10.dp, top = 6.dp)
                            // PA-REAL-0004 收尾:命中区 22→48dp;视觉图标(15dp)用内层
                            // padding 钉回原 22dp 盒中心(距盒右上角 11dp),位置零漂移。
                            .size(INPUT_TOUCH_TARGET_DP.dp)
                            .pressable { expandedInputOpen = true },
                        contentAlignment = Alignment.TopEnd,
                    ) {
                        Icon(
                            painterResource(AppR.drawable.ic_lucide_maximize_2), "展开编辑",
                            tint = if (input.length > EXPAND_HINT_THRESHOLD) ChatAccentColor else ChatSubtleColor,
                            modifier = Modifier
                                .padding(end = 3.5.dp, top = 3.5.dp)
                                .size(15.dp),
                        )
                    }
                    }
                }
                // [模型名全宽行] 当前生效的模型名**恒显且完整**:不截断、不限行数。
                // 【为什么改恒显】上一版只在名字超过 14 字符时才补这一行,于是"长度没超、
                // 但已被工具行省略号挤断"的名字依然读不全(用户反馈"模型名仍未完整显示")。
                // 现在无条件显示全文;整行同时内联"运行中模型确认",把原本独立的一行合并进来
                // (设计取向:能一行放下的不要分行),少一层堆叠且不丢信息。整行作模型/强度
                // 点击入口;所有控件触控箱仍走 INPUT_TOUCH_TARGET_DP(PA-REAL-0004)。
                //
                // 【2026-09-30 快速切换】点击整行(含下拉箭头)直接展开**快速切换面板**:
                // 最近用过的模型单点即切,不再强制进 ModelEffortCard 二级弹窗(两段式太重);
                // 面板尾部「更多…」仍进完整选择卡(跟随桌面/全部模型入口不丢)。模型名
                // 11sp→13sp、Muted→Ink:这是高频操作控件,字号与对比度按主控件档给。
                Box {
                Row(Modifier.fillMaxWidth().pressable { showQuickModels = true }
                    .semantics { contentDescription = "切换模型与思考强度" }
                    // PA-REAL-0004:整行高度 ≥48dp(实测 37.8dp)。heightIn 在 padding 外层,
                    // 内容(verticalAlignment=CenterVertically)在 48dp 内居中,只是上下多留白。
                    .heightIn(min = INPUT_TOUCH_TARGET_DP.dp)
                    // 【下移贴工具行】旧 padding(vertical=8)让模型名悬在输入区中段,与占位
                    // 文字读成一团(L2 真机挑刺);收成 top=4/bottom=0 —— 与占位文字拉开
                    // 4dp,视觉归入下方工具行一组(模型选择是"控件"不是提示文案)。
                    .padding(start = 16.dp, end = 16.dp, top = 4.dp), verticalAlignment=Alignment.CenterVertically) {
                    // 运行中模型确认:真机上"模型 · 强度 · 最近运行: 模型 · 强度"连读
                    // 出现"最大"重复三次(用户裁决:信息过载难读)。精简为把"最近运行"
                    // 收进快速切换面板头部(那里才是对比"当前 vs 正在跑"的语境)。
                    val runningHint = ""
                    val base = modelName + if (effortLabel != "默认") " · $effortLabel" else ""
                    // 切换成功行内反馈:modelFlashTick(快速面板/完整卡真实选中后 +1)驱动
                    // 模型名一次 Primary→Ink 的轻量闪光(纯绘制层动效,不造假状态;
                    // 模型名本体由生效链即时更新,闪的只是"刚变过"的提示)。
                    val nameFlash = remember { Animatable(0f) }
                    LaunchedEffect(modelFlashTick) {
                        if (modelFlashTick > 0) {
                            nameFlash.snapTo(1f)
                            nameFlash.animateTo(0f, tween(650))
                        }
                    }
                    val nameColor = lerp(ChatInkColor, ChatAccentColor, nameFlash.value)
                    Text(
                        buildAnnotatedString {
                            withStyle(SpanStyle(fontSize = 13.sp, color = nameColor, fontWeight = FontWeight.Medium)) {
                                append(base)
                            }
                            if (runningHint.isNotEmpty()) {
                                withStyle(SpanStyle(fontSize = 12.sp, color = ChatMutedColor)) {
                                    append(runningHint)
                                }
                            }
                        },
                        modifier=Modifier.weight(1f),
                        maxLines=2, overflow=TextOverflow.Ellipsis)
                    Icon(painterResource(AppR.drawable.ic_lucide_chevron_down), "展开快速切换模型", tint=ChatMutedColor,modifier=Modifier.requiredSize(18.dp))
                }
                // 快速切换面板:锚在模型行上方弹出;focusable 让面板外点击/返回键关闭。
                // 【2026-10-02 流畅度修复】面板从行内 Popup 挪到与 showModelCard 同层的
                // 稳定组合位(见文件底部 showModelCard 挂载点)—— 流式输出每帧重组会
                // 反复打断行内 Popup(真机:轮次运行中点模型行面板闪没/开不出,用户
                // 裁决"切换不流畅"的主因)。稳定层挂载不受输入区逐帧重组影响。
                }
                // 第二行:工具行
                Row(
                    Modifier
                        .fillMaxWidth()
                        .padding(start = 10.dp, end = 8.dp, bottom = 8.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Row(Modifier.weight(1f).horizontalScroll(rememberScrollState()),
                        verticalAlignment=Alignment.CenterVertically) {
                    // [+ 附件]
                    // 【image_input 位门控】daemon 逐 agent 宣告 image_input:<id>(dsh 无位,
                    // 发图必败),附件按钮是手机端唯一的图片入口 —— 缺位会话隐藏,免得
                    // 选完图必吃失败壳;带位 agent 不动(见 attachmentEntryHiddenFor)。
                    if (!attachmentEntryHiddenFor(threadAgentId, hostCaps)) {
                    Box(
                        Modifier
                            // PA-REAL-0004:触控箱 ≥48dp(原 32dp);图标仍 20dp 居中,视觉不变。
                            .sizeIn(minWidth = INPUT_TOUCH_TARGET_DP.dp, minHeight = INPUT_TOUCH_TARGET_DP.dp)
                            .pressable { pickFiles.launch("*/*") },
                        contentAlignment = Alignment.Center,
                    ) {
                        Icon(
                            painterResource(AppR.drawable.ic_lucide_plus), "添加附件",
                            tint = ChatMutedColor, modifier = Modifier.size(20.dp),
                        )
                    }
                    Spacer(Modifier.width(2.dp))
                    }
                    // [Git] 会话项目的版本管理侧栏
                    Box(
                        Modifier
                            // PA-REAL-0004 收尾:命中区 32→48dp;图标仍 18dp 居中,视觉不变。
                            .sizeIn(minWidth = INPUT_TOUCH_TARGET_DP.dp, minHeight = INPUT_TOUCH_TARGET_DP.dp)
                            .pressable { showGitPanel = true },
                        contentAlignment = Alignment.Center,
                    ) {
                        Icon(
                            painterResource(AppR.drawable.ic_lucide_git_branch), "项目 Git",
                            tint = ChatMutedColor, modifier = Modifier.size(18.dp),
                        )
                    }
                    Spacer(Modifier.width(2.dp))
                    // [优化](实验功能,开关在 设置-外观与阅读,默认关):把口语化输入
                    // 改写成给 AI 的清晰指令;可见性走 PromptOptimizeState 状态机
                    // (开关开 + 在线 + 输入非空 + 不在请求中)。请求中转圈,防重复点击。
                    if (promptOptState.buttonVisible || promptOptState.inFlight) {
                        Box(
                            Modifier
                                // PA-REAL-0004 收尾:命中区 32→48dp;转圈/星形图标绘制
                                // 尺寸不变,只放大不可见命中区。
                                .sizeIn(minWidth = INPUT_TOUCH_TARGET_DP.dp, minHeight = INPUT_TOUCH_TARGET_DP.dp)
                                .pressable { optimizeCurrentInput() },
                            contentAlignment = Alignment.Center,
                        ) {
                            if (promptOptState.inFlight) {
                                TypingDots(dotSize = 3.dp, gap = 2.dp)
                            } else {
                                Icon(
                                    painterResource(AppR.drawable.ic_lucide_sparkles), "优化提示词",
                                    tint = ChatAccentColor, modifier = Modifier.size(18.dp),
                                )
                            }
                        }
                        Spacer(Modifier.width(2.dp))
                    }
                    // [审批策略] 会话内无缝切换 —— 扁平可点区域(图标 + 文字 + chevron),
                    // 去掉 PaEditorial.Raised 胶囊底色回落到参考图的视觉层级(只借排版,颜色仍走 Pa)。
                    // DropdownMenu 的全部逻辑与 APPROVALS 取值映射保持不变。
                    if (sessionAgent == "codex") {
                    Box {
                        Row(
                            Modifier
                                .pressable { approvalMenu = true }
                                // PA-REAL-0004 收尾:整 chip 命中高度抬到 ≥48dp(原 ≈25dp)。
                                // heightIn 在 padding 外层(同"切换模型"整行口径),内容在
                                // 48dp 内垂直居中,视觉排布不变。
                                .heightIn(min = INPUT_TOUCH_TARGET_DP.dp)
                                .padding(horizontal = 4.dp, vertical = 6.dp),
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            val cur = codexSettings?.approvalPolicy ?: "on-request"
                            val danger = cur == "never"
                            Icon(
                                painterResource(AppR.drawable.ic_lucide_triangle_alert), null,
                                tint = if (danger) ChatDeniedColor else ChatAccentColor,
                                modifier = Modifier.size(13.dp),
                            )
                            Spacer(Modifier.width(5.dp))
                            Text(
                                APPROVALS.find { it.first == cur }?.second ?: "按需询问",
                                fontSize = 11.5.sp,
                                maxLines=1, overflow=TextOverflow.Ellipsis, modifier=Modifier.widthIn(max=72.dp),
                                color = if (danger) ChatDeniedColor else ChatInkColor,
                            )
                            Spacer(Modifier.width(2.dp))
                            Icon(
                                painterResource(AppR.drawable.ic_lucide_chevron_down), null,
                                tint = ChatSubtleColor, modifier = Modifier.size(12.dp),
                            )
                        }
                        DropdownMenu(
                            expanded = approvalMenu,
                            onDismissRequest = { approvalMenu = false },
                            containerColor = if (PaHacker.isHacker) PaEditorial.Raised else PaEditorial.Surface,
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
                                                    color = if (active) ChatAccentColor else ChatInkColor,
                                                )
                                                Text(desc, fontSize = 10.5.sp, color = ChatSubtleColor)
                                            }
                                            if (active) {
                                                Icon(
                                                    painterResource(AppR.drawable.ic_lucide_check), null,
                                                    tint = ChatAccentColor, modifier = Modifier.size(16.dp),
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
                    }
                    // [上下文窗口] 常显:圆形占用环 —— 外圈一个圆环,环内直接显示实际使用占比。
                    // 【本轮改版】旧版是「圆形图标 + 上下文 128k」的胶囊:横向占位大,而且
                    // 看不出窗口被占了多少。现在环内给占比(≥80% 黄、≥95% 红),窗口总量与
                    // 用量明细点开卡面看;窗口未知或还没上报用量时环内是"—",绝不假装 0%。
                    val ctxWindowKnown = !windowIsFallback && contextWindow > 0
                    val ctxUsage = st.turnUsage
                    val ctxRingKnown = ctxWindowKnown && ctxUsage != null
                    // 压缩同步:压缩后旧占用已失真,小环进入"已压"态而非继续显示旧占比。
                    val ctxCompacted = compactionPending(st.turnUsage, st.lastCompactedAt)
                    val ctxRatio =
                        if (ctxRingKnown) usageTotalTokens(ctxUsage).toFloat() / contextWindow.toFloat() else 0f
                    // 【空值不渲染】窗口未知或还没有用量时报"—"的空环,悬在工具行中间
                    // 像一个没有意义的"—"按钮(L2 真机挑刺);没有真值就整块不出,
                    // 拿到窗口+用量(或已压缩)才回场 —— 宁可缺席,不摆占位符。
                    if (ctxRingKnown || ctxCompacted) {
                    Box(
                        Modifier
                            // PA-REAL-0004 收尾:展示型控件的命中区 34→48dp;环视觉仍
                            // 28dp 居中不变(见下方 ContextRing 的 size),只放大命中区。
                            .sizeIn(minWidth = INPUT_TOUCH_TARGET_DP.dp, minHeight = INPUT_TOUCH_TARGET_DP.dp)
                            .semantics {
                                contentDescription = "上下文窗口 " +
                                    (if (ctxCompacted) "已压缩" else ringPercentLabel(ctxRatio, ctxRingKnown)) +
                                    if (ctxWindowKnown) " (窗口 ${contextWindow / 1000}k)" else ""
                            }
                            .pressable(haptic = true) { showContextCard = true },
                        contentAlignment = Alignment.Center,
                    ) {
                        ContextRing(
                            ratio = ctxRatio,
                            windowKnown = ctxRingKnown,
                            modifier = Modifier.size(28.dp),
                            compacted = ctxCompacted,
                        )
                    }
                    Spacer(Modifier.width(2.dp))
                    }
                    }
                    Spacer(Modifier.width(4.dp))
                    // 【P4 Task 6 语音输入】麦克风:无权限先申请(RECORD_AUDIO),有权限启动
                    // VoiceController 会话(系统识别优先,端侧 SenseVoice 兜底);识别中图标区
                    // 换成波形跳动柱;再点一次 = 停止并交付结果。识别文本只并入输入框
                    // (默认追加,长按切替换模式),绝不自动发送(保留编辑权)。
                    // 【动画 ②·识别中波形】4 根跳动柱替代旧的整体缩放脉动:柱间时长
                    // 错开(430/560/500/650ms)避免"同起同落"的机械感;空闲分支不挂
                    // InfiniteTransition,不空转。柱高用 scaleY 绘制层缩放,不改布局。
                    val micWave: List<Float> = if (voiceListening) {
                        val wave = rememberInfiniteTransition(label = "micWave")
                        listOf(
                            wave.animateFloat(
                                initialValue = 0.35f, targetValue = 1f,
                                animationSpec = infiniteRepeatable(tween(430), RepeatMode.Reverse),
                                label = "wave1",
                            ).value,
                            wave.animateFloat(
                                initialValue = 1f, targetValue = 0.3f,
                                animationSpec = infiniteRepeatable(tween(560), RepeatMode.Reverse),
                                label = "wave2",
                            ).value,
                            wave.animateFloat(
                                initialValue = 0.3f, targetValue = 1f,
                                animationSpec = infiniteRepeatable(tween(500), RepeatMode.Reverse),
                                label = "wave3",
                            ).value,
                            wave.animateFloat(
                                initialValue = 1f, targetValue = 0.4f,
                                animationSpec = infiniteRepeatable(tween(650), RepeatMode.Reverse),
                                label = "wave4",
                            ).value,
                        )
                    } else {
                        listOf(1f, 1f, 1f, 1f)
                    }
                    // 【动画 ①·按下涟漪】点按重放一圈"扩散 + 淡出"的 Accent 圆环:
                    // micRippleTick 每次点击 +1,Animatable 0→1 驱动半径与透明度。
                    val micRipple = remember { Animatable(0f) }
                    LaunchedEffect(micRippleTick) {
                        if (micRippleTick > 0) {
                            micRipple.snapTo(0f)
                            micRipple.animateTo(1f, tween(420))
                        }
                    }
                    Box(
                        Modifier
                            // PA-REAL-0004:触控箱 ≥48dp(原 40dp);涟漪/波形/图标的绘制尺寸
                            // 都不变,只是不可见的命中区放大。
                            .sizeIn(minWidth = INPUT_TOUCH_TARGET_DP.dp, minHeight = INPUT_TOUCH_TARGET_DP.dp)
                            // 【P4 Task 6】点按 = 开始/停止(按相裁决);长按 = 切换追加/替换模式。
                            .semantics {
                                // TalkBack 单一描述(图标本体不再各挂 contentDescription,避免读两遍):
                                // 空闲 = 模式;进行中 = 时长 + 引擎名 + 停止指引。
                                contentDescription = if (voiceListening) {
                                    "语音输入中,已录 $voiceElapsedSec 秒,${voiceEngineLabel(voiceState.engine)};" +
                                        "点按停止并出稿" + if (voiceReplaceMode) ",替换模式" else ""
                                } else if (voiceReplaceMode) {
                                    "语音输入(替换模式):点按开始,长按切回追加模式"
                                } else {
                                    "语音输入:点按开始,长按切换替换模式"
                                }
                            }
                            .pressableLong(
                                onClick = {
                                    micRippleTick++ // 【动画 ①】按下即起涟漪(不依赖识别结果)
                                    val granted = ContextCompat.checkSelfPermission(
                                        ctx, Manifest.permission.RECORD_AUDIO,
                                    ) == PackageManager.PERMISSION_GRANTED
                                    when (voiceState.phase) {
                                        // 空闲/失败:开新一轮(无权限则只申请,回答后经 Permission 事件续跑)
                                        VoicePhase.IDLE, VoicePhase.ERROR -> {
                                            if (granted && voiceController.current().engine == VoiceEngineKind.UNAVAILABLE) {
                                                // 引擎没建起来(端侧初始化中/失败):如实报因,可自愈重试
                                                reportVoiceUnavailable()
                                            } else {
                                                voiceController.send(VoiceEvent.Start)
                                            }
                                            if (!granted) micPermission.launch(Manifest.permission.RECORD_AUDIO)
                                        }
                                        // 收音中/启动中:停止并交付最终转写
                                        VoicePhase.STARTING, VoicePhase.LISTENING ->
                                            voiceController.send(VoiceEvent.Stop)
                                        // 暂停中:再点一次 = 恢复(同会话继续,文本累计不清)
                                        VoicePhase.PAUSED -> voiceController.send(VoiceEvent.Start)
                                        // 等权限弹窗/停止收尾/端侧转写中:防抖,忽略连点
                                        VoicePhase.REQUESTING_PERMISSION, VoicePhase.STOPPING,
                                        VoicePhase.TRANSCRIBING,
                                        -> {}
                                    }
                                },
                                onLongPress = {
                                    voiceReplaceMode = !voiceReplaceMode
                                    PaHaptics.light(hapticView)
                                    Toast.makeText(
                                        ctx,
                                        if (voiceReplaceMode) "语音输入已切到替换模式:识别结果将覆盖输入框" else "语音输入已切到追加模式:识别结果将接在输入框后",
                                        Toast.LENGTH_SHORT,
                                    ).show()
                                },
                            ),
                        contentAlignment = Alignment.Center,
                    ) {
                        // 涟漪圆环:仅动画进行中绘制(0..1),结束后自然隐藏
                        val ripple = micRipple.value
                        if (ripple > 0f && ripple < 1f) {
                            Box(
                                Modifier
                                    .size(32.dp)
                                    .graphicsLayer {
                                        scaleX = 0.5f + ripple
                                        scaleY = 0.5f + ripple
                                    }
                                    .background(ChatAccentColor.copy(alpha = 0.22f * (1f - ripple)), CircleShape),
                            )
                        }
                        if (voiceListening) {
                            // 识别中:波形柱接管图标位(32dp 盒内 4 柱,基线高 16dp)
                            Row(
                                horizontalArrangement = Arrangement.spacedBy(2.5.dp),
                                verticalAlignment = Alignment.CenterVertically,
                            ) {
                                micWave.forEach { v ->
                                    Box(
                                        Modifier
                                            .width(2.5.dp)
                                            .height(16.dp)
                                            .graphicsLayer { scaleY = v }
                                            .background(ChatAccentColor, RoundedCornerShape(2.dp)),
                                    )
                                }
                            }
                        } else {
                            // 替换模式点亮 Accent 色(视觉反馈);读屏描述统一挂在上方
                            // Box 的 semantics(避免读两遍),图标本体不再各挂描述。
                            Icon(
                                painterResource(AppR.drawable.ic_lucide_mic), null,
                                tint = if (voiceReplaceMode) ChatAccentColor else ChatMutedColor,
                                modifier = Modifier.size(18.dp),
                            )
                        }
                    }
                    Spacer(Modifier.width(4.dp))
                    // [发送 / 插入]
                    // 【P1-4】running 时输入可用,发送键变为「插入运行中任务」:
                    // 图标(列表+勾)与箭头区分,内容描述同步,让读屏与视觉都能分清
                    // "开新任务"与"插进正在跑的任务(或排队)"。离线快照态一律禁用。
                    val sendEnabled = !offlineSnapshot && input.isNotBlank() && !st.recovering
                    val inserting = st.running
                    // R2 停止可达性:运行中且输入为空 → 发送键原位变「终止」键(与
                    // ChatGPT/Codex 移动端同位语义:拇指不动就能停),不再是灰死的箭头。
                    // 运行中且输入非空 = P1-4 的「插入」,语义让位给输入;顶栏(黑客终端)
                    // 与「更多」(编辑式)的终止入口原样保留作兜底。
                    val canStopTurn = st.running && input.isBlank() && !offlineSnapshot
                    Box(
                        Modifier
                            // PA-REAL-0004:触控箱 ≥48dp(原 40dp)。Accent 圆底下沉一层保持
                            // 40dp 视觉不变(按下缩放仍作用于整体),只放大不可见命中区。
                            .sizeIn(minWidth = INPUT_TOUCH_TARGET_DP.dp, minHeight = INPUT_TOUCH_TARGET_DP.dp)
                            // 发送键 = 触感分级 confirm 档(发的动作值得重反馈,不再是
                            // 与普通点击同档的轻触);终止键 = warning 档(打断电脑端的
                            // 活,分量比发送更重)。反馈都在触发瞬间发生。
                            .pressable(scaleDown = 0.88f, enabled = sendEnabled || canStopTurn) {
                                if (canStopTurn) {
                                    PaHaptics.warning(hapticView)
                                    TurnSession.cancelTurn(threadId)
                                    Toast.makeText(ctx, "已请求终止电脑端任务", Toast.LENGTH_SHORT).show()
                                } else {
                                    PaHaptics.confirm(hapticView)
                                    if (send(input)) setInput("")
                                }
                            },
                        contentAlignment = Alignment.Center,
                    ) {
                        Box(
                            Modifier
                                .requiredSize(40.dp)
                                .background(
                                    when {
                                        canStopTurn -> ChatDeniedColor
                                        sendEnabled -> ChatAccentColor
                                        PaHacker.isHacker -> PaEditorial.Hover
                                        else -> PaEditorial.Raised
                                    },
                                    CircleShape,
                                ),
                            contentAlignment = Alignment.Center,
                        ) {
                            Icon(
                                painterResource(
                                    when {
                                        canStopTurn -> AppR.drawable.ic_lucide_square
                                        inserting -> AppR.drawable.ic_lucide_list_checks
                                        else -> AppR.drawable.ic_lucide_arrow_up
                                    }
                                ),
                                when {
                                    canStopTurn -> "终止任务"
                                    inserting -> "插入运行中任务"
                                    else -> "发送"
                                },
                                // 内容色:sendEnabled 腿坐 Primary 实底,读 OnPrimary 令牌
                                //(黑客盘电光青上白字 1.81:1);canStopTurn 腿坐 Denied
                                // 实底,读 OnDenied 令牌(深/黑客盘=暗红棕 #3A1514,
                                // 白字仅 2.51/2.77:1 已废)。
                                tint = when {
                                    canStopTurn -> PaEditorial.OnDenied
                                    sendEnabled -> PaEditorial.OnPrimary
                                    else -> ChatSubtleColor
                                },
                                modifier = Modifier.size(18.dp),
                            )
                        }
                    }
                }
            }
        } // chatComposerBody lambda 到此结束

        // —— 骨架分叉:黑客终端保持原 Column 骨架(显示模式行为不变);
        //       常规会话走编辑式 PaScreenScaffold(标题=会话名/返回/一个次操作/主机状态槽)。
        if (PaHacker.isHacker) {
            Column(
                Modifier
                    .fillMaxSize()
                    .background(PaEditorial.Canvas)
                    .statusBarsPadding()
                    // 【推背模糊】最近会话抽屉可见期间,被覆盖的会话内容整层后退
                    // (模糊+微缩+压暗)——抽屉是玻璃,透过玻璃看内容才是真磨砂。
                    .paPushBack(drawerCtrl.visible)
                    // 与其余页面统一:左边缘右滑也能唤出侧边栏
                    .edgeSwipeToOpenDrawer(
                        onDragFraction = drawerCtrl::onDrag,
                        onRelease = drawerCtrl::onRelease,
                    ),
            ) {
                // ---- tmux 式状态条(黑客终端模式):布局修复后移到界面最顶部 ----
                // (原在输入条上方;2026-09-30 用户令上移 —— 作为整个界面的第一行仪表,
                //  左段 [0:pa] + 会话标题,右段 连接状态 ● + 当前模型名 + HH:MM。)
                TmuxStatusLine(
                    title = threadTitle,
                    connState = connSt,
                    modelName = modelName,
                )
                chatMiddle()
                // 输入区外壳(黑客终端):原键盘避让(max(ime,nav))与边距原样保留
                Column(
                    Modifier
                        .fillMaxWidth()
                        .background(PaEditorial.Canvas)
                        .keyboardAwareBottomPadding()
                        .padding(start = 12.dp, end = 12.dp, top = 8.dp, bottom = 16.dp),
                ) {
                    chatComposerBody()
                }
            }
        } else {
            PaScreenScaffold(
                title = threadTitle.ifBlank { "会话" },
                titleContent = {
                    // 布局修复(2026-09-30):标题右侧跟 当前 Agent·工作区 + 状态点
                    // (绿=正常/蓝=运行中/红=故障/琥珀=待处理);原大身份卡取消。
                    Spacer(Modifier.width(10.dp))
                    ChatIdentityBadge(
                        agentName = selectedAgentLabel(threadAgentId),
                        workspaceLabel = threadWorkspace
                            ?.let { it.substringAfterLast('/').substringAfterLast('\\') }
                            ?.takeIf { it.isNotBlank() },
                        statusView = identityStatus,
                        approvalWaiting = st.pendingApproval != null &&
                            !askIsAskAction(st.pendingApproval!!.action.name),
                        onOpenApproval = { approvalDialogOpen = true },
                        agentIconRes = agentBrandIcon(threadAgentId),
                    )
                },
                modifier = Modifier
                    .fillMaxSize()
                    .paPushBack(drawerCtrl.visible)
                    .edgeSwipeToOpenDrawer(
                        onDragFraction = drawerCtrl::onDrag,
                        onRelease = drawerCtrl::onRelease,
                    ),
                leading = {
                    PaNavIconButton(
                        onClick = onBack,
                        contentDescription = "返回",
                        iconRes = AppR.drawable.ic_lucide_chevron_left,
                        plain = true,
                    )
                },
                // 唯一次操作(README 骨架纪律):「更多」收口终止/导出/最近会话等低频动作
                trailing = {
                    PaNavIconButton(
                        onClick = { showMore = true },
                        contentDescription = "更多",
                        iconRes = AppR.drawable.ic_lucide_ellipsis,
                    )
                },
                hostStatus = chatHostStatus,
                bottomBar = {
                    // 输入区外壳(编辑式):导航栏高度已由骨架 bottomBar 消费,
                    // 这里只避让键盘 —— 与旧 max(ime,nav) 语义等价(键盘弹出时导航栏在其后)。
                    Column(
                        Modifier
                            .fillMaxWidth()
                            .imePadding()
                            .padding(start = 16.dp, end = 16.dp, top = 8.dp, bottom = 12.dp),
                    ) {
                        chatComposerBody()
                    }
                },
            ) {
                chatMiddle()
            }
        }
        // 历史消息快速查看浮层(视频样板二版):从左缘滑出,宽 ~62%、scrim 0.25
        // (去遮挡:能隐约看到正文 —— 速查不是切页);黑客模式 GraphiteDrawer 底 +
        // 等宽字,常规模式 Surface 底 —— 全走 editorial 槽位。
        // 点卡片 → scrollToItem 跳转并关闭;Scrim 点击/返回键 → 关闭(BackHandler 见上)。
        HistoryQuickPanel(
            visible = showHistoryPanel,
            entries = historyEntries,
            currentIndex = historyCurrentIndex,
            onJump = { listIndex -> jumpToHistoryEntry(listIndex) },
            onDismiss = { showHistoryPanel = false },
            onJumpEarliest = historyEntries.firstOrNull()?.let { entry ->
                { jumpToHistoryEntry(entry.listIndex) }
            },
            onJumpLatest = { jumpToLatestFromHistory() },
            // 【全文搜索·Task 7】仓库注入(五态呈现/失败人话在浮层,防抖不变);
            // 命中点击走 message_id 主键自动定位(缺口自动拉取);searched_messages
            // 回传作 sequence 兼容定位的全量基准;会话标题进命中卡(spec)。
            searcher = historySearchRepository,
            onFullTextHitClick = onHistoryFullTextHitClick,
            threadTitle = threadTitle,
            onSearchSearched = { total ->
                // 最近一次搜索回包的实扫条数 = daemon 全量消息数(sequence 坐标系
                // 基准);身份索引随之刷新可信位。0(旧 daemon 未报)不覆盖已有值。
                // 【P3 Task 8】落位经 ViewModel(0 不覆盖的口径在 reducer 内)
                chatVm.send(ChatIntent.KnownTotalObserved(total))
            },
        )
        // R2-A 消息长按上下文菜单(单一实例):摘要行报"哪条消息",动作由
        // chatMessageMenuActions 按消息形态给(纯函数,ChatMessageMenuTest 钉死):
        //   复制全文(恒有,有正文才有菜单)/ 选择文本(显式进入拖选模式)/
        //   引用回复(该消息作为引文填进输入框,保留编辑权)/ 重试发送(仅失败级)。
        val menuTarget = messageMenuTarget
        if (menuTarget != null && messageMenu.visible) {
            ContextMenu(
                state = messageMenu,
                summary = chatMessageMenuSummary(menuTarget),
                items = chatMessageMenuActions(
                    retryable = deliveryRetryable(menuTarget.deliverState) && !offlineSnapshot,
                    hasText = menuTarget.text.isNotBlank(),
                ).map { id ->
                    when (id) {
                        "copy" -> ContextMenuItem("copy", "复制全文", AppR.drawable.ic_lucide_file_text)
                        "select" -> ContextMenuItem("select", "选择文本", AppR.drawable.ic_lucide_pencil)
                        "quote" -> ContextMenuItem("quote", "引用回复", AppR.drawable.ic_lucide_message_square)
                        else -> ContextMenuItem("retry", "重试发送", AppR.drawable.ic_lucide_refresh_cw)
                    }
                },
                onPick = { pick ->
                    messageMenu.hide()
                    when (pick.id) {
                        "copy" -> copyMessage(ctx, menuTarget.text)
                        "quote" -> {
                            // 引用只动草稿不发送:给用户留编辑权(与语音并入同一纪律)
                            setInput(quoteMessageIntoDraft(input, menuTarget.text))
                            Toast.makeText(ctx, "已引用到输入框,补充一句即可发送", Toast.LENGTH_SHORT).show()
                        }
                        "select" -> messageSelectArmed = menuTarget
                        "retry" -> retryDelivery(menuTarget)
                    }
                    messageMenuTarget = null
                },
                onDismiss = {
                    messageMenu.hide()
                    messageMenuTarget = null
                },
            )
        }
        // 「已优化提示词[撤销]」悬浮条:置顶居中,不挤内容(组件自带 5s 计时,
        // 撤销恢复原文;离开页面自动提交 —— 本条无提交动作,等价于自动消失)。
        UndoBarHost(promptOptUndo, Modifier.align(Alignment.TopCenter))
    }

    selectedAttachment?.let { attachment ->
        val preview = loadAttachmentPreview(ctx, attachment)
        Dialog(
            onDismissRequest = { selectedAttachment = null },
            properties = DialogProperties(usePlatformDefaultWidth = false),
        ) {
            Column(
                Modifier
                    .fillMaxSize()
                    .background(Color.Black.copy(alpha = 0.92f))
                    .clickable { selectedAttachment = null },
            ) {
                // 标题行:预览态显示**完整文件名**(不省略、可换行),长按复制完整名。
                // 这样"名字被裁"的老问题在点开预览后彻底消失(需求:预览标题行要完整可见)。
                Text(
                    attachment.name,
                    Modifier
                        .fillMaxWidth()
                        .pressableLong(
                            onClick = {},
                            onLongPress = { copyMessage(ctx, attachment.name) },
                        )
                        .padding(horizontal = 20.dp, vertical = 16.dp),
                    color = Color.White,
                    fontSize = 14.sp,
                    maxLines = 3,
                    softWrap = true,
                    overflow = TextOverflow.Ellipsis,
                )
                Box(
                    Modifier.fillMaxWidth().weight(1f),
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
    }

    // ---- 会话侧边栏:最近会话(搜索 + 最近使用时间)+ 底部全局入口 ----
    if (drawerCtrl.visible) {
        dev.pocketagent.app.ui.component.RecentThreadsDrawer(
            currentThreadId = threadId,
            threads = recentThreads,
            loading = recentLoading,
            dragProgress = drawerCtrl.progress,
            onDragClose = drawerCtrl::onDragClose,
            onReleaseClose = { drawerCtrl.close() },
            onClose = { drawerCtrl.close() },
            onOpenThread = { id ->
                drawerCtrl.close()
                if (id != threadId) onOpenThread(id)
            },
            // 会话页的抽屉里不再直接新建:要新建就去「全部会话」(那里有 + 与工作区选择)
            onOpenAllThreads = {
                drawerCtrl.close()
                onGoTab(1)
            },
            onOpenTerminal = {
                drawerCtrl.close()
                onOpenTerminal()
            },
            onOpenTaskCenter = {
                drawerCtrl.close()
                onOpenTaskCenter()
            },
            onOpenSettings = {
                drawerCtrl.close()
                onOpenSettings()
            },
        )
    }

    // ---- 更多:分叉 / 回滚 ----
    if (showMore) {
        // 【本轮换 PaSheet】统一玻璃浮层(滚动/高度上限/安全区内建)
        PaSheet(onDismissRequest = { showMore = false }) {
            Column(Modifier.fillMaxWidth().padding(bottom = 26.dp)) {
                Text(
                    "会话操作",
                    Modifier.fillMaxWidth().padding(horizontal = 24.dp),
                    fontSize = 17.sp, color = PaEditorial.Ink,
                )
                Spacer(Modifier.height(10.dp))
                if (threadAgentId=="zcode") {
                    Column(Modifier.fillMaxWidth().padding(horizontal=20.dp)) {
                        NativePermissionsSection("zcode",threadId)
                    }
                }
                // 【能力位门控】三个动作按 daemon 逐 agent 宣告的 compact/fork/
                // rollback:<agentId> 位点亮(判定见 ThreadActionGateTest 钉死的纯函数):
                // dsh 会话三项全无位 → 只剩标题与(zcode 的)权限区,不摆点了必败的
                // 假入口;分隔线只在两个可见区段之间出现,不出现在末尾。
                val compactOn = compactActionOn(threadAgentId, hostCaps)
                val forkOn = forkActionOn(threadAgentId, hostCaps)
                val rollbackOn = rollbackActionOn(threadAgentId, hostCaps)
                // 编辑式骨架(05):顶栏只留一个次操作(「更多」),低频动作收进本面板;
                // 黑客终端顶栏按钮原样保留,不重复给入口。
                if (!PaHacker.isHacker) {
                    if (st.running) {
                        SheetAction(AppR.drawable.ic_lucide_square, "终止当前任务", ChatDeniedColor) {
                            showMore = false
                            // 终止 = warning 档(与顶栏终止键、发送键原位终止同档)
                            PaHaptics.warning(hapticView)
                            TurnSession.cancelTurn(threadId)
                            Toast.makeText(ctx, "已请求终止电脑端任务", Toast.LENGTH_SHORT).show()
                        }
                        Spacer(Modifier.height(6.dp))
                    }
                    SheetAction(AppR.drawable.ic_lucide_menu, "最近会话") {
                        showMore = false
                        drawerCtrl.open()
                    }
                    SheetAction(AppR.drawable.ic_lucide_download, "导出对话") {
                        showMore = false
                        showExport = true
                    }
                    if (threadAgentId == "zcode" || compactOn || forkOn || rollbackOn) {
                        Spacer(Modifier.height(6.dp))
                        HorizontalDivider(Modifier.padding(horizontal = 24.dp), color = PaEditorial.Hairline)
                        Spacer(Modifier.height(6.dp))
                    }
                }
                if (compactOn) {
                    SheetAction(
                        AppR.drawable.ic_lucide_refresh_cw,
                        if (compactBusy) "压缩中…" else "压缩上下文",
                        PaEditorial.Primary,
                    ) { doCompact() }
                    Text(
                        // 文案按会话归属 agent 动态(daemon 的 CompactThread 已按 AgentID
                        // 路由):Pi 会话说"让 Pi 压缩",codex 会话说"让 Codex 压缩"。
                        // 真机实锤(图9):过去硬编码"让 Codex 压缩…",Pi 会话里张冠李戴。
                        compactionConsequenceCopy(threadAgentId),
                        Modifier.padding(start = 24.dp, end = 24.dp, bottom = 10.dp),
                        fontSize = 11.5.sp, color = PaEditorial.Subtle,
                    )
                    if (forkOn || rollbackOn) {
                        HorizontalDivider(Modifier.padding(horizontal = 24.dp), color = PaEditorial.Hairline)
                    }
                }
                if (forkOn) {
                    SheetAction(AppR.drawable.ic_lucide_git_branch, "分叉会话") { doFork() }
                    Text(
                        forkConsequence(),
                        Modifier.padding(start = 24.dp, end = 24.dp, bottom = 10.dp),
                        fontSize = 11.5.sp, color = PaEditorial.Subtle,
                    )
                    if (rollbackOn) {
                        HorizontalDivider(Modifier.padding(horizontal = 24.dp), color = PaEditorial.Hairline)
                    }
                }
                if (rollbackOn) {
                    SheetAction(
                        AppR.drawable.ic_lucide_undo_2, "回滚…", PaEditorial.Denied,
                    ) {
                        showMore = false
                        rollbackOpen = true
                    }
                    Text(
                        rollbackSummary(),
                        Modifier.padding(start = 24.dp, end = 24.dp),
                        fontSize = 11.5.sp, color = PaEditorial.Subtle,
                    )
                }
            }
        }
    }

    // ---- 斜杠面板伪命令"/目标":设置会话目标(SetThreadGoal)----
    // 由斜杠卡面的 /目标 伪命令打开(GOAL 动作,见上方候选段)。daemon 的
    // SetThreadGoal 按 agent 落库并下发(codex thread/goal/set、zcode session/goal,
    // pi 落库跟随),成功后用返回的 Thread 刷新顶栏目标文案,不整页重拉。
    if (slashGoalSheet) {
        var goalDraft by remember { mutableStateOf("") }
        var goalSaving by remember { mutableStateOf(false) }
        // 【本轮换 PaSheet】统一玻璃浮层
        PaSheet(onDismissRequest = { if (!goalSaving) slashGoalSheet = false }) {
            Column(Modifier.fillMaxWidth().padding(horizontal = 24.dp).padding(bottom = 26.dp)) {
                Text(
                    "设置会话目标",
                    Modifier.fillMaxWidth(),
                    fontSize = 17.sp, color = PaEditorial.Ink,
                )
                Text(
                    "目标会作为常驻上下文:agent 在这个会话里的每一轮都会看到它。",
                    Modifier.padding(top = 4.dp, bottom = 10.dp),
                    fontSize = 11.5.sp, color = PaEditorial.Subtle,
                )
                BasicTextField(
                    value = goalDraft,
                    onValueChange = { goalDraft = it },
                    modifier = Modifier
                        .fillMaxWidth()
                        // min 72 给短目标留形,max 200 封顶:目标再长输入框也不再增高,
                        // "保存目标"按钮始终在视口内(否则长目标把按钮推出 sheet 不可点,
                        // 同 SLA-21a / README §12 坑 6)。高度有界后 CoreTextField 内容
                        // 内部滚动并跟随光标,交互破坏最小(整列滚动会把按钮滚走)。
                        .heightIn(min = 72.dp, max = 200.dp)
                        .background(PaEditorial.Surface, RoundedCornerShape(12.dp))
                        .padding(12.dp),
                    textStyle = TextStyle(fontSize = 14.sp, color = PaEditorial.Ink),
                    cursorBrush = SolidColor(PaEditorial.Primary),
                    decorationBox = { inner ->
                        Box {
                            if (goalDraft.isEmpty()) {
                                Text(
                                    "例如:全程用 Kotlin,改动只落在 daemon 模块…",
                                    fontSize = 14.sp, color = PaEditorial.Subtle,
                                )
                            }
                            inner()
                        }
                    },
                )
                Spacer(Modifier.height(12.dp))
                Box(
                    Modifier
                        .fillMaxWidth()
                        .pressable(haptic = true) {
                            if (goalSaving) return@pressable
                            if (goalDraft.isBlank()) {
                                Toast.makeText(ctx, "先写点什么作为目标(或清空后关闭)", Toast.LENGTH_SHORT).show()
                                return@pressable
                            }
                            goalSaving = true
                            scope.launch {
                                try {
                                    val t = withContext(Dispatchers.IO) {
                                        AgentConnection.threadService().setThreadGoal(
                                            SetThreadGoalRequest.newBuilder()
                                                .setThreadId(threadId)
                                                .setGoal(goalDraft.trim())
                                                .build(),
                                        )
                                    }
                                    // 顶栏目标文案就地刷新(GoalDisplayLogic 与 GetThread 同一套口径)
                                    threadGoalLine = GoalDisplayLogic.goalLine(t)
                                    slashGoalSheet = false
                                    Toast.makeText(ctx, "已设置会话目标", Toast.LENGTH_SHORT).show()
                                } catch (e: Exception) {
                                    Toast.makeText(ctx, e.message ?: "设置目标失败", Toast.LENGTH_LONG).show()
                                } finally {
                                    goalSaving = false
                                }
                            }
                        }
                        .background(if (goalSaving) PaEditorial.Hover else PaEditorial.Primary, RoundedCornerShape(14.dp))
                        .padding(vertical = 13.dp),
                    contentAlignment = Alignment.Center,
                ) {
                    Text(
                        if (goalSaving) "正在保存…" else "保存目标",
                        // saving 腿坐 Hover 底保持白字;就绪腿坐 Primary 实底读 OnPrimary
                        fontSize = 14.5.sp,
                        color = if (goalSaving) Color.White else PaEditorial.OnPrimary,
                        fontWeight = FontWeight.Medium,
                    )
                }
            }
        }
    }

    // ---- 回滚:选丢几个回合,再确认 ----
    // 回合数从时间线里现数(每条用户消息 = 一个回合)。数不出回合(历史还没加载出来)
    // 就不给选项 —— 免得用户选完必然失败。
    if (rollbackOpen) {
        val turnTotal = st.items.count { it.role == "user" }
        val options = rollbackOptions(turnTotal)
        var picked by remember(turnTotal) { mutableStateOf(1) }
        // 【本轮换 PaSheet】统一玻璃浮层;滚动由封装内建(选项随回合数增长时
        // "取消/确认回滚"不会被推出视口,同 SLA-21a 语义),内容列不再自带 verticalScroll。
        PaSheet(onDismissRequest = { rollbackOpen = false }) {
            // 整列可滚(PaSheet 提供):选项随回合数增长(每条用户消息=一个回合),长会话时
            // "取消/确认回滚"保持可点(被滚动裁掉的节点不进无障碍树也不可点,README §12 坑 6)。
            Column(
                Modifier
                    .fillMaxWidth()
                    .padding(bottom = 26.dp),
            ) {
                Text(
                    "回滚会话",
                    Modifier.fillMaxWidth().padding(horizontal = 24.dp),
                    fontSize = 17.sp, color = PaEditorial.Ink,
                )
                if (options.isEmpty()) {
                    Text(
                        "这个会话还没有可回滚的回合。",
                        Modifier.padding(horizontal = 24.dp, vertical = 10.dp),
                        fontSize = 13.sp, color = PaEditorial.Subtle,
                    )
                } else {
                    Text(
                        rollbackConsequence(picked),
                        Modifier.fillMaxWidth().padding(horizontal = 24.dp, vertical = 6.dp),
                        fontSize = 12.sp, color = PaEditorial.Muted,
                    )
                    options.forEach { n ->
                        Row(
                            Modifier
                                .fillMaxWidth()
                                .pressable { picked = n }
                                .padding(horizontal = 24.dp, vertical = 12.dp),
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            Icon(
                                painterResource(
                                    if (picked == n) AppR.drawable.ic_lucide_circle_check
                                    else AppR.drawable.ic_lucide_undo_2,
                                ),
                                null,
                                tint = if (picked == n) PaEditorial.Denied else PaEditorial.Subtle,
                                modifier = Modifier.size(18.dp),
                            )
                            Spacer(Modifier.width(12.dp))
                            Text(
                                rollbackOptionLabel(n, turnTotal),
                                fontSize = 15.sp,
                                color = if (picked == n) PaEditorial.Ink else PaEditorial.Muted,
                            )
                        }
                    }
                    Row(
                        Modifier.fillMaxWidth().padding(horizontal = 20.dp, vertical = 10.dp),
                        horizontalArrangement = Arrangement.End,
                    ) {
                        TextButton(onClick = { rollbackOpen = false }, enabled = !rollbackBusy) {
                            Text("取消")
                        }
                        Spacer(Modifier.width(8.dp))
                        Button(
                            enabled = !rollbackBusy,
                            onClick = { doRollback(picked) },
                            colors = androidx.compose.material3.ButtonDefaults.buttonColors(
                                containerColor = PaEditorial.Denied,
                            ),
                            shape = RoundedCornerShape(14.dp),
                        ) { Text("确认回滚", color = PaEditorial.OnDenied) }
                    }
                }
            }
        }
    }

    // 导出对话(可多选)
    if (showExport) {
        ExportChatSheet(
            title = threadTitle,
            items = st.items.toList(),
            threadId = threadId,
            // 会话归属 agent(GetThread 里带回来):HTML 存档按 export_html:<agentId>
            // 能力位门控,位来自共享快照 hostCaps
            agentId = threadAgentId,
            capabilities = hostCaps,
            onDismiss = { showExport = false },
            onExport = { text, count ->
                showExport = false
                PaDownloader.saveTextAsFile(
                    "会话-%s-%s.md".format(
                        threadTitle.ifBlank { "未命名" }.take(20).replace(Regex("[\\\\/:*?\"<>|]"), "_"),
                        java.text.SimpleDateFormat("MMdd-HHmm", java.util.Locale.getDefault())
                            .format(java.util.Date()),
                    ),
                    text,
                    threadId,
                )
                Toast.makeText(ctx, "已导出 $count 条消息,可在任务中心查看", Toast.LENGTH_SHORT).show()
            },
        )
    }

    // 子代理快览(T-SUB,蓝本 §4.3):长按内联卡/组卡行弹出的"偷看"层,
    // 最近消息 + 「打开完整对话」;收起不影响任何状态。
    val peekMeta = peekSub
    if (peekMeta != null) {
        val peekRunIds = remember(peekMeta.threadId, SubAgentsState.lastRefreshedAt) {
            TurnSession.runningThreadIds()
        }
        SubAgentPeekSheet(
            meta = peekMeta,
            state = SubAgentsLogic.runStateOf(peekMeta, peekRunIds),
            onOpenFull = {
                peekSub = null
                onOpenThread(peekMeta.threadId)
            },
            onDismiss = { peekSub = null },
        )
    }

    // 长文本大窗编辑(需求:输入过长时可增大输入框窗口)
    if (expandedInputOpen) {
        ExpandedInputSheet(
            value = input,
            onValueChange = { setInput(it) },
            onDismiss = { expandedInputOpen = false },
            onSend = {
                val text = input
                expandedInputOpen = false
                if (send(text)) setInput("")
            },
            canSend = !offlineSnapshot && input.isNotBlank() && !st.recovering,
        )
    }

    // 优化预览弹层(P4 Task 2):原文/候选上下对比 + 候选可编辑 + 应用/撤销/
    // 防覆盖二选一;常驻「保留原意,不会自动执行」说明,候选大幅变长时提示复核
    // (纯函数 promptCandidateOverExpanded,阈值与裁定见其注释)。
    // 不空闲即挂载(请求中/有候选/有错误),空闲时彻底不进组合。
    if (!promptPreview.isIdle) {
        PromptOptimizePreviewSheet(
            state = promptPreview,
            currentRevision = inputRevision,
            onIntent = { dispatchPromptPreview(it) },
        )
    }

    // 新建对话(与所有入口同款:文件式选工作区)
    if (showNewThread) {
        NewThreadSheet(
            onDismiss = { showNewThread = false },
            onCreate = { ws ->
                showNewThread = false
                // 停用偏好放行闸(T-MW2,与主页"新建任务"同款):已停用 agent 不再创建
                // 新会话,给恢复路径提示(文案与主页同口径,不静默改路由);未停用/空
                // 偏好零行为。判定复用 newThreadAgentAllowed 纯函数,两入口口径一致。
                if (newThreadAgentAllowed(AgentConnection.selectedAgent, AgentConnection.disabledAgents)) {
                    scope.launch {
                        try {
                            val builder = CreateThreadRequest.newBuilder().setAgentId(AgentConnection.selectedAgent)
                            if (AgentConnection.selectedModel.isNotEmpty()) {
                                builder.setModelId(AgentConnection.modelForAgent(AgentConnection.selectedAgent))
                            }
                            // P1 修复:带上当前选中的 agent(设置页/主页切换到 Pi 后,新建会话
                            // 才真正走 pi 而不是仍默认 codex)。codex 或空保持不传 = 旧语义,
                            // 由 server 端默认路由;daemon 侧 agent 不可用仍允许建会话,
                            // 首条消息会得到 server 的人话错误。
                            val agentId = createThreadAgentId(AgentConnection.selectedAgent)
                            if (agentId.isNotEmpty()) {
                                builder.setAgentId(agentId)
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
                } else {
                    Toast.makeText(
                        ctx,
                        "「${selectedAgentLabel(AgentConnection.selectedAgent)}」已停用,请在主页 agent 卡片右上「管理」恢复后再新建",
                        Toast.LENGTH_SHORT,
                    ).show()
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
        // 【P3-T4 拆分】卡面整体搬至 ui/model.ModelEffortCard:页面只传仓库状态与
        // 意图出口,不再传 initialSelectedId(选中态唯一来源 =
        // ModelSelectionReducer.display(repository.state);daemon 真值 > 乐观投影 >
        // 「上次记录」)。运行中切换由卡面按 appliesToNextTurn 显示"将从下一轮生效"。
        ModelEffortCard(
            state = modelSelectionState,
            // 会话归属 agent:模型表/品牌图标/档位语义按它取,不用全局 selectedAgent
            // (否则在 pi 会话里开弹窗,弹的还是全局选中 agent 的模型表)。
            sessionAgent = sessionAgent,
            // 正在运行轮次的实际配置(只读对照行;卡面不写 TurnSession)
            runtimeModel = st.runningModel,
            runtimeEffort = st.runningEffort,
            // 意图出口:显式选择(模型/强度/重置)在下一轮确认前不让巡检原生回填覆盖
            onIntent = { intent ->
                if (intent != ModelSelectionIntent.Refresh) {
                    TurnSession.state(threadId).selectionPending = true
                }
                ModelSelectionRepository.send(intent)
            },
            onDismiss = { showModelCard = false },
            // 卡内真实选中 → 关卡后模型行给一次行内闪光(与快速面板同一反馈口径)
            onPicked = { modelFlashTick++ },
        )
    }

    // 【快速切换面板(2026-10-02 流畅度修复)】从输入区行内 Popup 挪到此处与
    // showModelCard 同层的稳定组合位 —— 输入区在流式输出时每帧重组,行内 Popup
    // 被反复打断(真机:轮次运行中点模型行,面板闪没/开不出,用户裁决"切换不
    // 流畅"的主因)。稳定层挂载后不受逐帧重组影响。
    if (showQuickModels) {
        val quickModels = remember(sessionModels, sessionAgent) {
            ModelQuickSwitchLogic.quickModels(sessionAgent, sessionModels)
        }
        Popup(
            alignment = Alignment.BottomStart,
            onDismissRequest = { showQuickModels = false },
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
                    val actualName = sessionModels.find { it.id == st.runningModel }?.displayName
                        ?: modelBareName(st.runningModel)
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
                        if (sessionModels.isEmpty()) "暂无可用模型，请进完整选择器刷新"
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
                                showQuickModels = false
                                applyModelPick(threadId, sessionAgent, qm.id)
                                modelFlashTick++ // 行内闪光:模型名刚真实切过一次
                            }
                            .heightIn(min = 44.dp)
                            .padding(horizontal = 14.dp, vertical = 8.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Column(Modifier.weight(1f)) {
                            Text(
                                qm.label,
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
                // 这里按会话 agent 的档位集(effortsFor 同源)渲染强度胶囊,单点即切
                // (下一轮生效语义与完整卡一致);【P3-T4 接线④】选中态来自仓库
                // display(ModelSelectionReducer.display),点击改发 SelectEffort 意图
                // —— 不再直读/直写 AgentConnection.selectedEffort 与 setThreadEffort。
                val quickEfforts = remember(sessionAgent, sessionModels) {
                    effortsFor(sessionModels, effectiveModelId, sessionAgent)
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
                            showQuickModels = false
                            showModelCard = true
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

    // 上下文窗口明细:点输入栏"上下文"胶囊打开(真实用量卡,替代旧估算版)。
    // 【断网可读】打开时先读本地缓存,再用实时数据覆盖;usageRefreshTick / 连接恢复
    // 都会让 cached 重新求值,从而"自动刷新"(读取 tick 作为 remember 键即登记了订阅)。
    if (showContextCard) {
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
            turnUsage = st.turnUsage ?: cached?.turnUsage,
            totalUsage = st.totalUsage ?: cached?.totalUsage,
            // 压缩同步:daemon 转发的"上下文已压缩"事件到达后,这里给出明确的压缩状态,
            // 而不是继续显示压缩前的旧占用(可能超出实际窗口)。
            compactionInProgress = st.compactionInProgress,
            lastCompactedAt = st.lastCompactedAt,
            // 压缩等待文案按会话归属 agent("等待 Pi/Codex 完成压缩"),不硬编码 codex。
            agentLabel = selectedAgentLabel(sessionAgent),
            // 只有"已知断网"(down)才显示离线提示条;checking 不显示(见上方 ③ 的注释)。
            online = !connDown,
            cachedAt = cached?.updatedAt,
            nowMillis = System.currentTimeMillis(),
            onDismiss = { showContextCard = false },
        )
    }
}

/**
 * 【P3-T4 拆分】原 ModelEffortCard(模型/思考强度卡面弹窗)已整体搬至
 * ui/model/ModelEffortCard.kt,并从"卡内自己拉 ListModels + currentPick 内部
 * 选中态 + 直写 AgentConnection"换血为计划 Task 4 的形态:
 *  - 页面只传 state(ModelSelectionRepository.state)与 onIntent(意图回调);
 *  - 选中态唯一来源 = ModelSelectionReducer.display(state)
 *    (daemon 真值 > 乐观投影 > 「上次记录」,fromCache 标注);
 *  - 选择动作走 ModelSelectionIntent(SelectModel/SelectEffort/ResetToDefault/
 *    Refresh),RPC/缓存/全局状态全部收敛在 ModelSelectionRepository;
 *  - 运行中切换按 appliesToNextTurn 显示"将从下一轮生效";
 *  - 单行选项件见 ui/model/ModelOptionRow.kt(含窄屏可读的能力副文案)。
 */

/** 一个可选项:label 主文案必填;description 副文案 / preview 代码块是 daemon 透传的原生字段,存在才渲染。 */
internal data class AskOption(
    val label: String,
    val description: String = "",
    val preview: String = "",
)

/**
 * 一条待用户回答的问题(来自 codex 的 item/tool/requestUserInput / MCP elicitation)。
 * 契约(F-A 终版):zcode/pi 卡 description/preview 空时键省略(omitempty)、multiSelect
 * 仅 true 时出现(缺省=单选);codex 卡 header/isSecret/isOther 恒出现(false 也透出)。
 * 这些都是 daemon 透传的问题级字段,存在才生效(旧形状全缺省,行为不变)。
 */
internal data class AskQuestion(
    val id: String,
    val text: String,
    val options: List<AskOption>,
    val multiSelect: Boolean = false,
    val isSecret: Boolean = false,
    val isOther: Boolean = false,
    val header: String = "", // codex 卡的问题分组小标题,存在才渲染
)

/** 问题 id 缺失时给个稳定的兜底 key,否则答案会挂不上(与旧行为一致)。 */
internal fun askQid(q: AskQuestion, idx: Int): String = q.id.ifBlank { "q$idx" }

/**
 * 解析审批卡 json_args 里的 questions(纯函数,JVM 可测)。
 * 【能透传就透传】选项取 label 主文案,description/preview 原样保留(渲染时如实展示、不改写);
 * 问题级 multiSelect/isSecret/isOther 存在才生效。不认识的字段/选项形状一律忽略不炸——
 * 旧 daemon(选项只有 label 的旧形状)解析结果与旧版逐字段等价,不回归。
 */
internal fun parseAskQuestions(args: JSONObject?): List<AskQuestion> {
    val arr = args?.optJSONArray("questions") ?: return emptyList()
    return buildList {
        for (i in 0 until arr.length()) {
            val q = arr.optJSONObject(i) ?: continue // 认不出的条目跳过,不炸
            val opts = q.optJSONArray("options")?.let { oa ->
                (0 until oa.length()).mapNotNull { k ->
                    when (val o = oa.opt(k)) {
                        is JSONObject -> AskOption(
                            label = o.optString("label"),
                            description = o.optString("description"),
                            preview = o.optString("preview"),
                        )
                        is String -> AskOption(label = o) // 宽容:裸字符串选项当 label
                        else -> null // 未知形状不硬造;选项全丢的题自然降级为自由输入兜底
                    }
                }
            } ?: emptyList()
            add(
                AskQuestion(
                    id = q.optString("id"),
                    text = q.optString("question"),
                    options = opts,
                    multiSelect = q.optBoolean("multiSelect", false),
                    isSecret = q.optBoolean("isSecret", false),
                    isOther = q.optBoolean("isOther", false),
                    header = q.optString("header"),
                ),
            )
        }
    }
}

/** 是否至少收集到一条答案(纯函数,JVM 可测):无问题列表的纯文字反问卡恒可提交(旧行为)。 */
internal fun askHasAnswers(
    questions: List<AskQuestion>,
    picks: Map<String, Set<String>>,
    texts: Map<String, String>,
): Boolean =
    questions.isEmpty() ||
        picks.values.any { it.isNotEmpty() } ||
        texts.values.any { it.isNotBlank() }

/**
 * 打包答案成 daemon/codex 期望的 {"<问题id>":{"answers":[...]}} 形状(纯函数,JVM 可测)。
 * 多选=勾选的 label 按选项渲染顺序依次入数组;自由输入(开放题/"其他"/密码题)非空则追加;
 * 单选/纯输入维持旧形状(单元素数组)。daemon 解析器同时容忍单值与数组,数组形状对多选天然成立。
 * 一题一条答案都没收集到就不放该键——不伪造已答。
 */
internal fun packAskAnswers(
    questions: List<AskQuestion>,
    picks: Map<String, Set<String>>,
    texts: Map<String, String>,
): JSONObject {
    val obj = JSONObject()
    questions.forEachIndexed { idx, q ->
        val key = askQid(q, idx)
        val chosen = picks[key].orEmpty()
        val vals: List<String> = buildList {
            q.options.forEach { o -> if (o.label in chosen && o.label !in this) add(o.label) }
            texts[key].orEmpty().trim().takeIf { it.isNotEmpty() && it !in this }?.let { add(it) }
        }
        if (vals.isNotEmpty()) {
            val arr = JSONArray()
            vals.forEach { arr.put(it) }
            obj.put(key, JSONObject().put("answers", arr))
        }
    }
    return obj
}

/** 需要"回答"而不是"批准"的动作类型。 */
private val ApprovalAskNames = setOf("request_user_input", "mcp_elicitation")

/**
 * 是否"反问类"动作(纯函数,JVM 可测):反问卡只认这两类 action.name,与具体 agent 无关。
 * 抽成函数是为了把"不对某个 agent 开小灶"这道契约锁进单测 —— 任何 agent 只要 daemon 上报
 * 这两类请求就走同一张卡;其他动作名一律走"拒绝/允许",不会误渲染成反问卡。
 */
internal fun askIsAskAction(name: String): Boolean = name in ApprovalAskNames

/**
 * 审批抽屉:按动作类型渲染 + 拒绝/允许(反问类则是"取消/提交")。
 *
 * 【为什么按类型分派】daemon 现在会送出多种动作类型(shell / file_change / apply_patch /
 * permissions / request_user_input / mcp_elicitation)。以前全部被画成"一条 shell 命令",
 * 于是"允许写入文件""允许放开网络访问"在手机上长得和"允许跑 rm -rf"一模一样 ——
 * 用户点"允许"时并不知道自己在允许什么,审批卡就失去了意义。
 * 这里按 name 给出各自的标题、正文与按钮文案,让每一次授权都能被看懂。
 *
 * 【反问卡与 agent 无关 · 运行时能力宣告】本抽屉是所有 agent 共用的同一张卡,不是 zcode 专属。
 * 渲染的唯一触发是 daemon 经 gRPC 送来的 `APPROVAL_REQUEST` 事件(TurnSession.kt:637 / 快照
 * 回填 :926 直接 `st.pendingApproval = ...`,全程没有任何 agent 身份判断),再看 action.name 是否
 * 落在反问类集合([askIsAskAction])决定画成"取消/提交"还是"拒绝/允许"。也就是说:
 * 哪个 agent 支持反问,完全由 daemon 在运行时上报请求来宣告,UI 不写死任何 agent 名单。
 * 这正是硬红线"无能力不显示假入口"的实现 —— 某 agent 的反问特性被关掉(比如 codex 的反问
 * 依赖一个会弹警告的实验特性)→ daemon 不再发 `request_user_input` → 抽屉根本不出现,绝不会
 * 渲染一张点了没反应的卡。推广反问体验到 codex/pi/dsh 不需要改 UI,只需各自 daemon 上报同类请求。
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun ApprovalSheet(
    action: ApprovalRequest,
    error: String?,
    onDecision: (Boolean, String) -> Unit,
) {
    var expanded by remember(action.requestId) { mutableStateOf(false) }
    val name = action.action.name
    val isAsk = askIsAskAction(name)
    // 参数是 JSON 字符串,解析失败时退回原样展示(不能让卡片变成空白)。
    val args = remember(action.action.jsonArgs) {
        runCatching { JSONObject(action.action.jsonArgs) }.getOrNull()
    }

    // 标题 / 副标题:让用户一眼知道"这次要发生的是什么事"。
    val title: String
    val subtitle: String
    when (name) {
        "shell" -> {
            title = "允许执行?"; subtitle = "agent 请求执行以下命令"
        }
        "file_change" -> {
            title = "允许修改文件?"; subtitle = "agent 请求改动以下文件"
        }
        "apply_patch" -> {
            title = "允许应用补丁?"; subtitle = "agent 请求改动以下文件"
        }
        "permissions" -> {
            title = "允许提权?"; subtitle = "agent 请求额外的文件 / 网络权限"
        }
        "request_user_input" -> {
            title = "agent 在问你"; subtitle = args?.optString("message").orEmpty().ifBlank { "回答后它才会继续" }
        }
        "mcp_elicitation" -> {
            title = "MCP 需要确认"
            val server = args?.optString("server").orEmpty()
            subtitle = (args?.optString("message").orEmpty().ifBlank { "请确认这次调用" }) +
                if (server.isNotBlank()) "（来自 $server）" else ""
        }
        "write_file" -> {
            // 旧行为保留:本地 provider 的写文件审批仍走这条路。
            title = "允许写入?"; subtitle = "agent 请求写入以下文件"
        }
        else -> {
            title = "需要你确认"; subtitle = "agent 请求执行以下操作"
        }
    }

    // 正文:按类型取该动作**特有**的字段,而不是把整坨 JSON 丢给用户。
    val body = remember(action.action.jsonArgs, name) {
        buildString {
            when (name) {
                "shell" -> {
                    append(args?.optString("command").orEmpty())
                    args?.optString("cwd").orEmpty().takeIf { it.isNotBlank() }?.let {
                        append("\n\n工作目录：").append(it)
                    }
                }
                "file_change", "apply_patch" -> {
                    val files = args?.optJSONArray("files")
                    if (files != null && files.length() > 0) {
                        for (i in 0 until files.length()) appendLine(files.optString(i))
                    }
                    args?.optInt("count", 0)?.takeIf { it > 0 }?.let { append("共 ").append(it).append(" 个文件") }
                }
                "permissions" -> {
                    val perms = args?.optJSONObject("permissions")
                    if (perms != null) append(perms.toString(2)) else append(action.action.jsonArgs)
                }
                else -> append(action.action.jsonArgs)
            }
            args?.optString("reason").orEmpty().takeIf { it.isNotBlank() }?.let {
                append("\n\n理由：").append(it)
            }
            args?.optString("grantRoot").orEmpty().takeIf { it.isNotBlank() }?.let {
                append("\n会话内授权范围：").append(it)
            }
        }.trim()
    }

    // 反问类:每个问题一行(有选项就渲染成可选按钮),答案按 codex 期望的
    // {"<问题id>":{"answers":[...]}} 形状回给 daemon。解析走纯函数 parseAskQuestions:
    // description/preview/multiSelect/isSecret/isOther 都是 daemon 透传字段,存在才生效;
    // 旧形状(选项只有 label)行为不变。
    val questions = remember(action.action.jsonArgs) { parseAskQuestions(args) }
    // 问题 id 缺失时给个稳定的兜底 key,否则答案会挂不上。
    var picks by remember(action.requestId) { mutableStateOf(emptyMap<String, Set<String>>()) } // 每题勾中的选项 label
    var texts by remember(action.requestId) { mutableStateOf(emptyMap<String, String>()) } // 每题自由输入
    val canSubmit = isAsk && askHasAnswers(questions, picks, texts)

    fun submit() {
        onDecision(true, packAskAnswers(questions, picks, texts).toString())
    }

    PaSheet(
        onDismissRequest = { },
        // 【红线语义·参数化后原样持有】三件事一字不动:
        //  1) 不可下滑关闭 —— swipeDismissEnabled = false,PaSheet 内部代为构造
        //     confirmValueChange 拒绝 Hidden 的 sheetState,下滑手势永远弹回;
        //  2) 拦截返回键 —— interceptBackPress = true,内部代为构造
        //     shouldDismissOnBackPress = false 的 properties;
        //  3) 不替用户做决定 —— onDismissRequest 为空,系统关闭请求(点遮罩等)
        //     一律忽略,卡片只能经"拒绝/允许/取消/提交"由用户亲手了结。
        // 两个开关都是稳定布尔:SheetState/ModalBottomSheetProperties 在
        // material3 1.3.0 是实验类,直接进公开签名会把 opt-in 扩散到全部
        // PaSheet 调用点(编译实证九处全红),等义能力由封装以稳定类型承载。
        // 这正是 PaSheet 参数化收敛的方向:语义由调用方显式声明(此前注释说要
        // "保持不换 PaSheet",如今这些开关有了显式入参,换封装不再放弃任何语义)。
        swipeDismissEnabled = false,
        interceptBackPress = true,
        // 【状态栏重叠的真因】material3 的 ModalBottomSheet 默认 insets
        // 只消费底部、不管顶部;命令一长抽屉被撑到接近全屏时,拖柄/标题/
        // 命令卡会直接画进状态栏。改传整块 safeDrawing(上下都管)让顶部
        // 自动让开 —— 这笔历史修复在 PaSheet 上经 contentWindowInsets 续命。
        // 注意:本版本 material3 的 contentWindowInsets 是 `@Composable () -> WindowInsets`
        // 而不是直接取值 —— 少一层 `{ }` 会连锁报"@Composable invocations can only
        // happen from the context of a @Composable function"。
        contentWindowInsets = { WindowInsets.safeDrawing },
        // 外层滚动保持 PaSheet 默认(开启):小屏/横屏上内容超限时"拒绝/允许"按钮
        // 永远可达 —— sheet_overflow 巡检的滚动保证不因迁移回退。
    ) {
        Row(
            Modifier.fillMaxWidth().padding(horizontal = 24.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(title, Modifier.weight(1f), fontSize = 19.sp, color = PaEditorial.Ink)
            // 反问类没有"可展开的原文",收起/展开这一对控件对它没有意义。
            if (!isAsk) {
                Text(
                    if (expanded) "收起" else "展开",
                    fontSize = 11.sp,
                    color = PaEditorial.Primary,
                )
                IconActionButton(
                    if (expanded) AppR.drawable.ic_lucide_chevron_left else AppR.drawable.ic_lucide_chevron_right,
                    if (expanded) "隐藏详情" else "展开详情",
                    { expanded = !expanded },
                    bg = Color.Transparent,
                    size = 36.dp,
                    iconSize = 18.dp,
                )
            }
        }
        Text(
            subtitle,
            Modifier
                .fillMaxWidth()
                .padding(horizontal = 24.dp, vertical = 4.dp),
            fontSize = 13.sp,
            color = PaEditorial.Muted,
        )
        error?.let {
            Text(
                it,
                Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 24.dp, vertical = 4.dp),
                fontSize = 12.sp,
                color = PaEditorial.Denied,
            )
        }
        if (isAsk) {
            // 【反问类】把问题原样摆出来:有选项就做成按钮(手机上点一下比打字快),
            // 也保留输入框用于开放式回答。答案经 note 通道回给 daemon。
            Column(
                Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 24.dp, vertical = 8.dp)
                    .heightIn(max = 320.dp)
                    .verticalScroll(rememberScrollState()),
            ) {
                if (questions.isEmpty()) {
                    Text(
                        args?.optString("message").orEmpty().ifBlank { action.action.jsonArgs },
                        fontSize = 13.sp, color = PaEditorial.Ink, lineHeight = 19.sp,
                    )
                }
                questions.forEachIndexed { idx, q ->
                    val key = askQid(q, idx)
                    val cur = texts[key].orEmpty()
                    val sel = picks[key] ?: emptySet()
                    val groupTop = if (idx == 0) 0.dp else 14.dp
                    // header:codex 卡的问题分组小标题,存在才渲染(cadence 与问题文本一体)
                    if (q.header.isNotBlank()) {
                        Text(
                            q.header,
                            Modifier.padding(top = groupTop, bottom = 2.dp),
                            fontSize = 11.sp,
                            color = PaEditorial.Subtle,
                        )
                    }
                    Text(
                        q.text.ifBlank { "请回答" },
                        Modifier.padding(top = if (q.header.isNotBlank()) 0.dp else groupTop, bottom = 8.dp),
                        fontSize = 13.5.sp, color = PaEditorial.Ink, lineHeight = 19.sp,
                    )
                    if (q.options.isNotEmpty()) {
                        Column {
                            q.options.forEach { opt ->
                                val active = opt.label in sel
                                Box(
                                    Modifier
                                        .fillMaxWidth()
                                        .padding(bottom = 6.dp)
                                        .pressable {
                                            if (q.multiSelect) {
                                                // 多选:切换勾选态,已勾的自由输入保留(作"其他"补充)
                                                val next = sel.toMutableSet()
                                                if (active) next.remove(opt.label) else next.add(opt.label)
                                                picks = picks + (key to next)
                                            } else {
                                                // 单选:点了就定,同时清自由输入(与旧"选择/输入互斥"语义一致)
                                                picks = picks + (key to setOf(opt.label))
                                                if (texts[key].orEmpty().isNotEmpty()) texts = texts - key
                                            }
                                        }
                                        .background(
                                            if (active) PaEditorial.PrimaryContainer else PaEditorial.Surface,
                                            RoundedCornerShape(12.dp),
                                        )
                                        .border(
                                            0.5.dp,
                                            if (active) PaEditorial.Primary else PaEditorial.Hairline,
                                            RoundedCornerShape(12.dp),
                                        )
                                        .padding(horizontal = 14.dp, vertical = 11.dp),
                                ) {
                                    Row(verticalAlignment = Alignment.Top) {
                                        if (q.multiSelect) {
                                            // 勾选态:选中=品牌蓝实底+白勾,未选中=空心方框
                                            Box(
                                                Modifier
                                                    .size(16.dp)
                                                    .clip(RoundedCornerShape(4.dp))
                                                    .background(if (active) PaEditorial.Primary else Color.Transparent)
                                                    .border(
                                                        0.5.dp,
                                                        if (active) PaEditorial.Primary else PaEditorial.Subtle,
                                                        RoundedCornerShape(4.dp),
                                                    ),
                                                contentAlignment = Alignment.Center,
                                            ) {
                                                if (active) {
                                                    Icon(
                                                        painterResource(AppR.drawable.ic_lucide_check),
                                                        null,
                                                        tint = PaEditorial.OnPrimary,
                                                        modifier = Modifier.size(12.dp),
                                                    )
                                                }
                                            }
                                            Spacer(Modifier.width(10.dp))
                                        }
                                        Column(Modifier.weight(1f)) {
                                            Text(
                                                opt.label,
                                                fontSize = 13.sp,
                                                color = if (active) PaEditorial.Primary else PaEditorial.Muted,
                                            )
                                            // 副文案:daemon 透传的 description,存在才显示
                                            if (opt.description.isNotBlank()) {
                                                Text(
                                                    opt.description,
                                                    Modifier.padding(top = 3.dp),
                                                    fontSize = 11.5.sp,
                                                    color = PaEditorial.Subtle,
                                                    lineHeight = 16.sp,
                                                )
                                            }
                                            // preview 如实展示:等宽字体+原样换行,不美化不改写
                                            if (opt.preview.isNotBlank()) {
                                                Text(
                                                    opt.preview,
                                                    Modifier
                                                        .fillMaxWidth()
                                                        .padding(top = 8.dp)
                                                        .clip(RoundedCornerShape(8.dp))
                                                        .background(PaEditorial.Canvas)
                                                        .border(0.5.dp, PaEditorial.Hairline, RoundedCornerShape(8.dp))
                                                        .padding(horizontal = 10.dp, vertical = 8.dp),
                                                    fontSize = 11.sp,
                                                    fontFamily = FontFamily.Monospace,
                                                    color = PaEditorial.Ink,
                                                    lineHeight = 16.sp,
                                                )
                                            }
                                        }
                                    }
                                }
                            }
                        }
                    }
                    Box(
                        Modifier
                            .fillMaxWidth()
                            .clip(RoundedCornerShape(12.dp))
                            .background(PaEditorial.Surface)
                            .border(0.5.dp, PaEditorial.Hairline, RoundedCornerShape(12.dp)),
                    ) {
                        BasicTextField(
                            value = cur,
                            onValueChange = { s ->
                                texts = texts + (key to s)
                                // 单选题一打字即清勾选(与旧"输入覆盖选择"一致);多选保留勾选,输入作补充
                                if (q.options.isNotEmpty() && !q.multiSelect && sel.isNotEmpty()) {
                                    picks = picks - key
                                }
                            },
                            modifier = Modifier.fillMaxWidth().padding(horizontal = 12.dp, vertical = 11.dp),
                            textStyle = TextStyle(fontSize = 13.sp, color = PaEditorial.Ink),
                            cursorBrush = SolidColor(PaEditorial.Primary),
                            // 密码题:输入以密码态展示,不回显明文
                            visualTransformation = if (q.isSecret) {
                                PasswordVisualTransformation()
                            } else {
                                VisualTransformation.None
                            },
                            decorationBox = { inner ->
                                Box {
                                    if (cur.isEmpty()) {
                                        Text(
                                            when {
                                                q.options.isEmpty() -> "输入你的回答…" // 无选项=自由输入兜底(含未知形状降级)
                                                q.isOther -> "或填写其他…"
                                                else -> "或直接输入你的回答…"
                                            },
                                            fontSize = 13.sp, color = PaEditorial.Subtle,
                                        )
                                    }
                                    inner()
                                }
                            },
                        )
                    }
                }
            }
        } else {
            // 【详情区出入场】展开/收起走 AnimatedContent(淡入+轻微上移 / 淡出,
            // spring 可打断):详情不再硬切出现/消失,收回时平滑让位。
            AnimatedContent(
                targetState = expanded || name == "permissions",
                transitionSpec = {
                    (fadeIn(PaMotion.emphasized()) + slideInVertically(PaMotion.emphasized()) { it / 10 })
                        .togetherWith(fadeOut(PaMotion.standard()))
                },
                label = "approvalDetails",
            ) { showDetails ->
            if (showDetails) {
            // 权限提升没有"展开才看"的道理:它要用户看清放开了什么,默认就展示。
            // 【详情区改造】三个问题一起解决:
            //  1) 原来是裸 Text,命令一长就把抽屉顶到全屏,既不滚动也没法收;
            //     → 加高度上限 + 纵向滚动,抽屉高度从此可控;
            //  2) 长命令的手机上核对/复制是刚需 → 套 SelectionContainer;
            //  3) 底色 PaEditorial.Canvas 铺在 PaEditorial.Raised 上是一块"纯黑凹陷" →
            //     换成 PaEditorial.Surface + 细描边,与 Markdown 代码块用同一套观感(全局一致)。
            val cmdScroll = rememberScrollState()
            Box(
                Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 24.dp, vertical = 12.dp)
                    .clip(RoundedCornerShape(14.dp))
                    .background(PaEditorial.Surface)
                    .border(0.5.dp, PaEditorial.Hairline, RoundedCornerShape(14.dp))
                    .heightIn(max = 240.dp)
                    .verticalScroll(cmdScroll),
            ) {
                SelectionContainer {
                    Text(
                        body,
                        Modifier.padding(14.dp),
                        fontSize = 12.sp,
                        fontFamily = FontFamily.Monospace,
                        color = PaEditorial.Ink,
                        // 命令按原样换行展示(不强制单行),配合纵向滚动即可读全
                        lineHeight = 17.sp,
                    )
                }
            }
            }
            }
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
                    .pressable(haptic = true) { onDecision(false, "") }
                    .background(PaEditorial.Surface, RoundedCornerShape(16.dp))
                    .padding(vertical = 14.dp),
                contentAlignment = Alignment.Center,
            ) {
                Text(if (isAsk) "取消" else "拒绝", fontSize = 16.sp, color = PaEditorial.Denied)
            }
            Spacer(Modifier.width(12.dp))
            Box(
                Modifier
                    .weight(1f)
                    .pressable(haptic = true, enabled = !isAsk || canSubmit) {
                        if (isAsk) submit() else onDecision(true, "")
                    }
                    .background(
                        if (isAsk && !canSubmit) PaEditorial.Hover else PaEditorial.Primary,
                        RoundedCornerShape(16.dp),
                    )
                    .padding(vertical = 14.dp),
                contentAlignment = Alignment.Center,
            ) {
                Text(
                    if (isAsk) "提交" else "允许",
                    fontSize = 16.sp,
                    color = if (isAsk && !canSubmit) PaEditorial.Subtle else PaEditorial.OnPrimary,
                )
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
                    .background(PaEditorial.Raised, RoundedCornerShape(topEnd = 22.dp, bottomEnd = 22.dp))
                    .statusBarsPadding()
                    .navigationBarsPadding()
                    .padding(horizontal = 16.dp),
            ) {
                Spacer(Modifier.height(14.dp))
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Icon(
                        painterResource(AppR.drawable.ic_lucide_git_branch), null,
                        tint = PaEditorial.Primary, modifier = Modifier.size(18.dp),
                    )
                    Spacer(Modifier.width(8.dp))
                    Column(Modifier.weight(1f)) {
                        Text("Git", fontSize = 16.sp, color = PaEditorial.Ink, fontWeight = FontWeight.SemiBold)
                        Text(
                            status?.branch?.takeIf { it.isNotEmpty() }?.let { "分支 $it · $repo" } ?: repo,
                            fontSize = 10.sp, color = PaEditorial.Subtle, maxLines = 1,
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
                            Text("当前会话未绑定工作区", fontSize = 14.sp, color = PaEditorial.Ink)
                            Text(
                                "创建会话时选择工作文件夹后，Git 会跟随该会话",
                                fontSize = 11.sp,
                                color = PaEditorial.Subtle,
                                modifier = Modifier.padding(top = 4.dp),
                            )
                        }
                    }
                    notRepo -> Box(
                        Modifier.weight(1f).fillMaxWidth(),
                        contentAlignment = Alignment.Center,
                    ) {
                        Column(horizontalAlignment = Alignment.CenterHorizontally) {
                            Text("该项目还不是 Git 仓库", fontSize = 14.sp, color = PaEditorial.Ink)
                            Text(
                                "在电脑上 git init 后,这里可以看状态、diff 和提交",
                                fontSize = 11.sp,
                                color = PaEditorial.Subtle,
                                modifier = Modifier.padding(top = 4.dp),
                            )
                        }
                    }
                    error != null -> Box(
                        Modifier.weight(1f).fillMaxWidth(),
                        contentAlignment = Alignment.Center,
                    ) {
                        Text(error!!, color = PaEditorial.Denied, fontSize = 12.sp)
                    }
                    status != null -> {
                        val st = status!!
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            Text("${st.changesList.size} 个变更", fontSize = 12.sp, color = PaEditorial.Muted)
                            Spacer(Modifier.weight(1f))
                            if (busy) {
                                CircularProgressIndicator(Modifier.size(14.dp), strokeWidth = 2.dp, color = PaEditorial.Primary)
                                Spacer(Modifier.width(8.dp))
                            }
                            IconActionButton(AppR.drawable.ic_lucide_refresh_cw, "刷新", { refresh() }, bg = PaEditorial.Surface, size = 34.dp, iconSize = 15.dp)
                        }
                        Spacer(Modifier.height(8.dp))
                        if (st.changesList.isEmpty()) {
                            Text("工作区干净", Modifier.padding(vertical = 20.dp), fontSize = 13.sp, color = PaEditorial.Muted)
                        }
                        LazyColumn(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(6.dp)) {
                            items(st.changesList, key = { it.path }) { c ->
                                Column(
                                    Modifier
                                        .fillMaxWidth()
                                        .background(PaEditorial.Surface, RoundedCornerShape(12.dp))
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
                                    // 与 GitScreen.ChangeRow 同一套读法:文件名永远完整可读(目录单独一行),
                                    // 状态字母翻成中文,未跟踪显示"新"而不是把 porcelain "??" 糊给用户。
                                    val untracked = isUntrackedChange(c.stagedStatus, c.worktreeStatus)
                                    val fileName = c.path.substringAfterLast('/')
                                    val dirPart = c.path.substringBeforeLast('/', "")
                                    Text(
                                        (if (untracked) "新 · " else "") + fileName,
                                        fontSize = 12.5.sp,
                                        fontFamily = FontFamily.Monospace,
                                        color = PaEditorial.Ink,
                                        maxLines = 1,
                                        overflow = TextOverflow.Ellipsis,
                                    )
                                    if (dirPart.isNotEmpty()) {
                                        Text(
                                            dirPart,
                                            fontSize = 10.sp,
                                            fontFamily = FontFamily.Monospace,
                                            color = PaEditorial.Subtle,
                                            maxLines = 1,
                                            overflow = TextOverflow.Ellipsis,
                                        )
                                    }
                                    Text(
                                        if (untracked) {
                                            "未跟踪 · 点击查看 diff"
                                        } else {
                                            "暂存:${gitStatusLabel(c.stagedStatus)} · 工作区:${gitStatusLabel(c.worktreeStatus)} · 点击查看 diff"
                                        },
                                        fontSize = 9.5.sp, color = PaEditorial.Subtle, maxLines = 1,
                                    )
                                }
                            }
                        }
                        if (st.changesList.isNotEmpty()) {
                            Box(
                                Modifier
                                    .fillMaxWidth()
                                    .pressable(enabled = !busy) { showCommit = true }
                                    .background(PaEditorial.Primary, RoundedCornerShape(14.dp))
                                    .padding(vertical = 12.dp),
                                contentAlignment = Alignment.Center,
                            ) {
                                Text(
                                    "提交全部变更",
                                    fontSize = 14.sp,
                                    color = PaEditorial.OnPrimary,
                                    fontWeight = FontWeight.SemiBold,
                                )
                            }
                        }
                    }
                    else -> Box(
                        Modifier.weight(1f).fillMaxWidth(),
                        contentAlignment = Alignment.Center,
                    ) {
                        Text("读取中…", fontSize = 12.sp, color = PaEditorial.Subtle)
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

/**
 * 导出对话面板:选消息 → 导出为 .txt(交给 PaDownloader 走后台下载 + 任务中心 + 用户数据管理)。
 *
 * 【为什么放这里】之前"保存到手机"散落在每条回复下面,用户根本分不清保存的是什么;
 * 汇总成一个入口 + 明确的条数回执,才符合"下载的东西要看得见、找得到"。
 */
/**
 * 长文本大窗编辑。
 *
 * 【为什么需要】底部小胶囊最多 4 行,写稍长的需求或贴一段代码时看不到全貌,
 * 改中间一句得来回滚。这里给一个占大半个屏幕的编辑区。
 *
 * 【为什么不会有内容不同步的问题】它和主输入框共用**同一份 state**
 * (`value` / `onValueChange` 直连 `input`),不存在两份副本需要回写,
 * 点"完成"只是收起面板,文字本来就已经在输入框里了。
 */
@OptIn(androidx.compose.material3.ExperimentalMaterial3Api::class)
@Composable
private fun ExpandedInputSheet(
    value: String,
    onValueChange: (String) -> Unit,
    onDismiss: () -> Unit,
    onSend: () -> Unit,
    canSend: Boolean,
) {
    PaSheet(
        onDismissRequest = onDismiss,
        // 【历史修复不回退】大窗编辑依赖整块 safeDrawing insets(顶部让开状态栏)。
        // 此前这正是"不换 PaSheet"的唯一理由;参数化后经 contentWindowInsets
        // 显式传入,修复语义原样保留。
        contentWindowInsets = { WindowInsets.safeDrawing },
        scrollable = false, // 编辑列自身固定高度、文本框内自滚;外层再包滚动只会打架
    ) {
        Column(
            Modifier
                .fillMaxWidth()
                // PaSheet 外层已给 82% 屏高上限(收敛前的 0.82f 直接顶满的是同一块
                // 可用高度):填满外层上限即可,编辑区大小与迁移前一致。
                .fillMaxHeight()
                .padding(horizontal = 16.dp),
        ) {
            Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                Text("编辑消息", Modifier.weight(1f), fontSize = 17.sp, color = PaEditorial.Ink)
                Text("${value.length} 字", fontSize = 11.sp, color = PaEditorial.Subtle)
                Spacer(Modifier.width(10.dp))
                // 完成 = 收起面板,内容原样留在底部输入框
                Box(
                    Modifier
                        .clip(RoundedCornerShape(10.dp))
                        .pressable { onDismiss() }
                        .background(PaEditorial.Surface)
                        .padding(horizontal = 12.dp, vertical = 7.dp),
                ) {
                    Text("完成", fontSize = 13.sp, color = PaEditorial.Ink)
                }
                Spacer(Modifier.width(8.dp))
                // 发送键触感分级:大窗里发送与底栏同档(confirm 级,见输入区发送键注释)
                val sendHapticView = LocalView.current
                Box(
                    Modifier
                        .clip(RoundedCornerShape(10.dp))
                        .pressable(scaleDown = 0.92f, enabled = canSend) {
                            PaHaptics.confirm(sendHapticView)
                            onSend()
                        }
                        .background(if (canSend) PaEditorial.Primary else PaEditorial.Hover)
                        .padding(horizontal = 12.dp, vertical = 7.dp),
                ) {
                    Text(
                        "发送", fontSize = 13.sp,
                        color = if (canSend) PaEditorial.OnPrimary else PaEditorial.Subtle,
                    )
                }
            }
            Spacer(Modifier.height(10.dp))
            val scroll = rememberScrollState()
            Box(
                Modifier
                    .fillMaxWidth()
                    .weight(1f)
                    .clip(RoundedCornerShape(16.dp))
                    .background(PaEditorial.Surface)
                    .border(0.5.dp, PaEditorial.Hairline, RoundedCornerShape(16.dp)),
            ) {
                BasicTextField(
                    value = value,
                    onValueChange = onValueChange,
                    modifier = Modifier.fillMaxSize().verticalScroll(scroll).padding(14.dp),
                    textStyle = TextStyle(fontSize = 15.sp, color = PaEditorial.Ink, lineHeight = 22.sp),
                    cursorBrush = SolidColor(PaEditorial.Primary),
                    decorationBox = { inner ->
                        Box {
                            if (value.isEmpty()) {
                                Text(
                                    "随心输入,发给电脑上的 agent…",
                                    fontSize = 15.sp, color = PaEditorial.Subtle,
                                )
                            }
                            inner()
                        }
                    },
                )
            }
            Spacer(Modifier.height(12.dp))
        }
    }
}

// ---- 导出 HTML 存档(daemon 侧 ExportThreadHtml)的门控与文案纯函数 ----

/** HTML 导出按钮的可用性(按 export_html:<agentId> 能力位判定,纯函数便于单测)。 */
internal enum class HtmlExportGate {
    /** daemon 宣告 export_html:<agentId> 位:直接发 RPC(现状仅 pi 有位)。 */
    SUPPORTED,
    /** 已确认无位(含旧 daemon 没发过位):本地人话拒绝,不发 RPC —— 无声就是 bug。 */
    UNSUPPORTED_AGENT,
    /** agentId 未知(空:GetThread 未回/旧数据):不拦截,交给 daemon 裁决。 */
    UNKNOWN,
}

/**
 * 【为什么空串放行】agent_id 缺失的旧数据里可能藏着真 pi 会话;若把"未知"误判成
 * "不支持",会被本地误杀(拦错比放错更伤)。放行的代价有限:就算真是 codex,
 * daemon 也会回错误,经失败 Toast 人话展示,永远有反馈。
 * 【门控数据源】`export_html:<agentId>` 能力位(daemon 2026-09-25 对齐):现状仍
 * 只有 pi 有位(行为等价),但不再按 agent 名单硬编码 —— 将来别家补上 ExportHTML
 * 实现并宣告位时,入口自动点亮,手机端零改动。
 */
internal fun htmlExportGate(agentId: String, capabilities: List<String>): HtmlExportGate = when {
    agentId.isBlank() -> HtmlExportGate.UNKNOWN
    agentCapableOf("export_html", agentId, capabilities) -> HtmlExportGate.SUPPORTED
    else -> HtmlExportGate.UNSUPPORTED_AGENT
}

/**
 * 导出失败的人话文案:daemon 自述的错误(业务错误)原样透传;传输层错误
 * (gRPC 状态码)翻译成可执行的人话,不把 "UNAVAILABLE: ..." 英文砸用户脸上。
 */
internal fun htmlExportFailureMessage(error: Throwable): String {
    val status = (error as? io.grpc.StatusRuntimeException)?.status
    if (status == null) {
        return error.message?.ifBlank { null } ?: "导出 HTML 失败"
    }
    return when (status.code) {
        io.grpc.Status.Code.UNIMPLEMENTED ->
            "电脑端 daemon 版本过旧,暂不支持 HTML 存档;请先升级电脑端"
        io.grpc.Status.Code.UNAVAILABLE ->
            "连不上电脑,请确认电脑端在线后重试"
        io.grpc.Status.Code.DEADLINE_EXCEEDED ->
            "电脑端渲染超时,请稍后重试"
        else -> error.message?.ifBlank { null } ?: "导出 HTML 失败"
    }
}

/**
 * Markdown 导出的正文纯函数(选中条目 → 对话块),供单测钉死"导出内容与选中
 * 一致"(P1-1:user → "## 我",其余 → "## Agent",正文原样,条目间空行分隔;
 * 标题/时间头在调用方拼,这里只管正文 —— 时间不进单测断言)。
 */
internal fun buildExportBody(chosen: List<ChatItem>): String {
    val sb = StringBuilder()
    chosen.forEach { m ->
        sb.append(if (m.role == "user") "## 我" else "## Agent").append("\n")
        sb.append(m.text).append("\n\n")
    }
    return sb.toString()
}

@Composable
private fun ExportChatSheet(
    title: String,
    items: List<ChatItem>,
    threadId: String,
    agentId: String,
    capabilities: List<String>,
    onDismiss: () -> Unit,
    onExport: (text: String, count: Int) -> Unit,
) {
    // 只导出有正文的对话消息(思考/工具另有开关,避免把日志塞进文档)
    val exportable = remember(items) {
        items.filter { (it.role == "user" || it.role == "assistant") && it.text.isNotBlank() }
    }
    // 【修复"只能导出 3 条"】原来的数据源是内存里的 st.items —— 它受
    // ChatScreen 那条 `!st.running && (st.items.isEmpty() || ...)` 门控,
    // 而且打开旧会话时可能只装了最近几轮。这里在打开面板时从 daemon
    // 拉一次**全量历史**(limit=0),用它做导出的真源。
    var full by remember(threadId) { mutableStateOf<List<ChatItem>?>(null) }
    LaunchedEffect(threadId) {
        if (threadId.isBlank()) return@LaunchedEffect
        full = runCatching {
            withContext(Dispatchers.IO) {
                AgentConnection.threadService()
                    .getThreadHistory(
                        GetThreadHistoryRequest.newBuilder().setThreadId(threadId).build(),
                    )
                    .messagesList
            }.let { historyToChatItems(it) }
                .filter { (it.role == "user" || it.role == "assistant") && it.text.isNotBlank() }
        }.getOrNull()
    }
    // 历史拉回来之前先用内存里的,拉到之后无缝换成全量(两边都以"有正文的对话消息"为准)
    val source = full ?: exportable
    var selected by remember(source) { mutableStateOf(source.indices.toSet()) }
    // ---- 导出 HTML 存档(daemon 侧 ExportThreadHtml RPC)----
    // 与上面的 Markdown 导出互补:Markdown 只导出手机上选中的正文;
    // HTML 是电脑端把整棵会话渲染成的离线存档(含思考/工具过程),文件落在
    // 电脑的会话工作区下,成功后经"文件页"的下载链路取回手机看。
    var exportingHtml by remember { mutableStateOf(false) }
    val htmlScope = rememberCoroutineScope()
    val htmlCtx = LocalContext.current
    // 可见性门控:已确认无 export_html:<agentId> 位的会话,按钮置灰降级(点击仍给
    // 人话 Toast,不许无声);agentId 未知(空)时保持原样,交给 daemon 裁决
    // (见 htmlExportGate 注释)。
    val htmlGateBlocked = htmlExportGate(agentId, capabilities) == HtmlExportGate.UNSUPPORTED_AGENT

    // 【本轮换 PaSheet】统一玻璃浮层;scrollable=false —— 内容主体是 LazyColumn
    // (自己会滚),避免两层滚动嵌套打架;82% 高度上限仍然生效。
    PaSheet(onDismissRequest = onDismiss, scrollable = false) {
        Column(
            Modifier
                .fillMaxWidth()
                .padding(horizontal = 20.dp)
                .padding(bottom = 16.dp),
        ) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Column(Modifier.weight(1f)) {
                    Text(
                        "导出对话",
                        fontSize = 19.sp,
                        color = PaEditorial.Ink,
                        fontWeight = FontWeight.SemiBold,
                    )
                    Spacer(Modifier.height(2.dp))
                    Text(
                        title.ifBlank { "未命名会话" } + " · 已选 " + selected.size + " / " + source.size + " 条" +
                            if (full == null) "(正在读取全部历史…)" else "",
                        fontSize = 11.5.sp,
                        color = PaEditorial.Subtle,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                }
                // 右上角:选择方式的拆分按钮。主按钮随当前选择**实时变化**
                // (全选后显示"已全选"、清空后显示"未选"、部分选中显示"已选 n"),
                // 主按钮点击 = 全选/清空来回切换(修复"反选后右上角没反应"的观感)。
                val allSelected = selected.size == source.size && selected.isNotEmpty()
                dev.pocketagent.app.ui.component.SplitButton(
                    label = "选择",
                    mainLabel = when {
                        selected.isEmpty() -> "未选"
                        allSelected -> "已全选"
                        else -> "已选 " + selected.size
                    },
                    compact = true,
                    onMainClick = {
                        selected = if (allSelected) emptySet() else source.indices.toSet()
                    },
                    options = listOf(
                        dev.pocketagent.app.ui.component.SplitOption(
                            "全选", "选中全部 " + source.size + " 条",
                            selected = allSelected,
                        ) { selected = source.indices.toSet() },
                        dev.pocketagent.app.ui.component.SplitOption(
                            "反选", "已选的反过来",
                        ) { selected = source.indices.toSet() - selected },
                        dev.pocketagent.app.ui.component.SplitOption(
                            "清空", "一条都不选",
                            selected = selected.isEmpty(),
                        ) { selected = emptySet() },
                    ),
                )
            }

            Spacer(Modifier.height(12.dp))
            androidx.compose.foundation.lazy.LazyColumn(
                Modifier
                    .fillMaxWidth()
                    .heightIn(max = 340.dp),
                verticalArrangement = Arrangement.spacedBy(6.dp),
            ) {
                items(source.size) { idx ->
                    val chatItem = source[idx]
                    val on = idx in selected
                    Row(
                        Modifier
                            .fillMaxWidth()
                            .pressable(haptic = true) {
                                selected = if (on) selected - idx else selected + idx
                            }
                            .background(
                                if (on) PaEditorial.Primary.copy(alpha = 0.20f) else PaEditorial.Surface,
                                RoundedCornerShape(12.dp),
                            )
                            .border(
                                if (on) 1.dp else 0.dp,
                                if (on) PaEditorial.Primary else androidx.compose.ui.graphics.Color.Transparent,
                                RoundedCornerShape(12.dp),
                            )
                            .padding(horizontal = 11.dp, vertical = 9.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        // 选中框(自绘,避免再引入一个图标资源)
                        Box(
                            Modifier
                                .size(16.dp)
                                .background(if (on) PaEditorial.Primary else Color.Transparent, RoundedCornerShape(5.dp))
                                .border(1.dp, if (on) PaEditorial.Primary else PaEditorial.Subtle, RoundedCornerShape(5.dp)),
                            contentAlignment = Alignment.Center,
                        ) {
                            if (on) {
                                Icon(
                                    painterResource(AppR.drawable.ic_lucide_check), null,
                                    tint = PaEditorial.OnPrimary, modifier = Modifier.size(11.dp),
                                )
                            }
                        }
                        Spacer(Modifier.width(10.dp))
                        Column(Modifier.weight(1f)) {
                            Text(
                                if (chatItem.role == "user") "你" else "Agent",
                                fontSize = 10.5.sp,
                                color = if (chatItem.role == "user") PaEditorial.AccentText else PaEditorial.Subtle,
                            )
                            Text(
                                chatItem.text.replace("\n", " ").take(80),
                                fontSize = 12.5.sp,
                                color = PaEditorial.Ink,
                                maxLines = 2,
                                overflow = TextOverflow.Ellipsis,
                            )
                        }
                    }
                }
            }

            Spacer(Modifier.height(14.dp))
            Box(
                Modifier
                    .fillMaxWidth()
                    .pressable(haptic = true) {
                        if (selected.isEmpty()) return@pressable
                        // 【P1-1 修复】导出数据源统一为 source:选中列表 UI、计数、
                        // 落盘必须用同一张表。full 历史加载完成后 selected 以 source
                        // 为键重建(remember(source)),索引语义已切到 source 上;若仍
                        // 按索引从内存子集 exportable 取,会把"已选 50/50 条"导成
                        // 内存里那几条(内容与计数错位)。条数用过滤后的实际值。
                        val chosen = source.filterIndexed { i, _ -> i in selected }
                        val sb = StringBuilder()
                        sb.append("# ").append(title.ifBlank { "未命名会话" }).append("\n")
                        sb.append("导出时间: ")
                            .append(java.text.SimpleDateFormat("yyyy-MM-dd HH:mm", java.util.Locale.getDefault())
                                .format(java.util.Date()))
                            .append("\n\n")
                        sb.append(buildExportBody(chosen))
                        onExport(sb.toString(), chosen.size)
                    }
                    .background(
                        if (selected.isEmpty()) PaEditorial.Hover else PaEditorial.Primary,
                        RoundedCornerShape(14.dp),
                    )
                    .padding(vertical = 13.dp),
                contentAlignment = Alignment.Center,
            ) {
                Text(
                    if (selected.isEmpty()) "请选择要导出的消息" else "导出 " + selected.size + " 条消息(Markdown)",
                    fontSize = 14.5.sp,
                    color = if (selected.isEmpty()) PaEditorial.Subtle else PaEditorial.OnPrimary,
                    fontWeight = FontWeight.Medium,
                )
            }

            Spacer(Modifier.height(10.dp))
            // 导出 HTML 存档:调 daemon 的 ExportThreadHtml(export_html:<agentId>
            // 位会话支持,现状仅 pi 宣告)。
            // 有位:发 RPC,成功 Toast 电脑上的绝对路径并收起面板,引导去文件页下载;
            // 失败人话 Toast(daemon 业务错误原样透传,传输层错误翻成人话)。
            // 无位(codex 等):点击直接本地人话拒绝,不发 RPC —— 无声就是 bug。
            Box(
                Modifier
                    .fillMaxWidth()
                    .pressable(haptic = true) {
                        // 防重入:一轮导出进行中再点忽略(按钮文案已是"正在电脑端渲染 HTML…")
                        if (exportingHtml) return@pressable
                        if (htmlGateBlocked) {
                            Toast.makeText(
                                htmlCtx,
                                // 能力位口径:该会话归属的助手没有 export_html 位(现状仅
                                // pi 宣告),不点名 agent,免得 daemon 将来扩位后文案说谎。
                                "该会话的助手暂不支持导出 HTML 存档",
                                Toast.LENGTH_LONG,
                            ).show()
                            return@pressable
                        }
                        exportingHtml = true
                        htmlScope.launch {
                            try {
                                val path = withContext(Dispatchers.IO) {
                                    AgentConnection.threadService()
                                        .exportThreadHtml(
                                            ExportThreadHtmlRequest.newBuilder()
                                                .setThreadId(threadId)
                                                .build(),
                                        )
                                        .path
                                }
                                Toast.makeText(
                                    htmlCtx,
                                    "已在电脑生成 HTML 存档:$path\n可在文件页下载查看",
                                    Toast.LENGTH_LONG,
                                ).show()
                                // 成功即收起面板;Toast 属应用层,面板关掉照样展示
                                onDismiss()
                            } catch (e: Exception) {
                                Toast.makeText(
                                    htmlCtx,
                                    htmlExportFailureMessage(e),
                                    Toast.LENGTH_LONG,
                                ).show()
                            } finally {
                                exportingHtml = false
                            }
                        }
                    }
                    .background(if (htmlGateBlocked) PaEditorial.Hover else PaEditorial.Surface, RoundedCornerShape(14.dp))
                    .padding(vertical = 13.dp),
                contentAlignment = Alignment.Center,
            ) {
                Text(
                    if (exportingHtml) "正在电脑端渲染 HTML…" else "导出 HTML 存档(电脑端)",
                    fontSize = 14.5.sp,
                    color = if (htmlGateBlocked) PaEditorial.Subtle else PaEditorial.Muted,
                    fontWeight = FontWeight.Medium,
                )
            }
        }
    }
}

/** 导出面板上的小胶囊按钮。 */
@Composable
private fun ExportChip(label: String, onClick: () -> Unit) {
    Box(
        Modifier
            .pressable(haptic = true) { onClick() }
            .background(PaEditorial.Surface, RoundedCornerShape(50))
            .padding(horizontal = 12.dp, vertical = 6.dp),
    ) {
        Text(label, fontSize = 12.sp, color = PaEditorial.Muted)
    }
}

/** 复制一条消息的文本到系统剪贴板(长按消息触发)。 */
private fun copyMessage(ctx: Context, text: String) {
    if (text.isBlank()) return
    val cm = ctx.getSystemService(Context.CLIPBOARD_SERVICE) as android.content.ClipboardManager
    cm.setPrimaryClip(android.content.ClipData.newPlainText("PocketAgent 消息", text))
    Toast.makeText(ctx, "已复制", Toast.LENGTH_SHORT).show()
}

// =====================================================================================
// R2-A 消息长按上下文菜单 —— 纯决策区(零 Android 依赖,ChatMessageMenuTest 钉死)
// 皮 = component/ContextMenu(锚点弹窗),接线在 ChatScreen(见根层渲染处)。
// =====================================================================================

/** 引用进输入框的单条引文最多带多少原文(超出截断,防长输出把草稿灌爆)。 */
internal const val QUOTE_MAX_CHARS = 160

/**
 * 长按菜单的动作 id 列表(顺序即展示顺序;ContextMenuLogic 只调 destructive 置底,
 * 本列表没有危险动作)。正文是菜单的前提:纯附件消息没有可复制/引用的东西,不出菜单。
 */
internal fun chatMessageMenuActions(retryable: Boolean, hasText: Boolean): List<String> {
    if (!hasText) return emptyList()
    return buildList {
        add("copy")
        add("select")
        add("quote")
        if (retryable) add("retry")
    }
}

/**
 * 菜单摘要行:角色词 + 换行压平后的正文开头 —— 让用户确认"这次操作针对哪条"。
 * 无正文只报角色词;超长截断加省略号(绝不渲染整条,菜单行只有一行高)。
 */
internal fun chatMessageMenuSummary(item: ChatItem): String {
    val label = if (item.role == "user") "用户消息" else "回复"
    val head = item.text.replace("\n", " ").trim()
    if (head.isEmpty()) return label
    return if (head.length <= 24) "$label · $head" else "$label · ${head.take(24)}…"
}

/**
 * 「引用回复」:把原文压成 Markdown 引文(`> ` 前缀)接进草稿,不自动发送。
 * 空草稿直接放引文;已有草稿先空一行再接(不和原句黏行)。引文超长截断 ——
 * 引用是"让 agent 知道我在说哪条",不是全文搬运(全文走「复制全文」)。
 */
internal fun quoteMessageIntoDraft(
    draft: String,
    quoted: String,
    maxQuoteChars: Int = QUOTE_MAX_CHARS,
): String {
    val body = quoted
        .lineSequence()
        .map { it.trim() }
        .filter { it.isNotEmpty() }
        .joinToString("\n") { "> $it" }
    if (body.isEmpty()) return draft
    val clipped = if (body.length <= maxQuoteChars) body else body.take(maxQuoteChars) + "…"
    return if (draft.isBlank()) "$clipped\n" else "${draft.trimEnd('\n')}\n\n$clipped\n"
}

// =====================================================================================
// P1-9/P4 语音输入 —— 纯逻辑区(遗留桥 + 接线纯函数)
//
// 【P4 Task 4/5 迁移说明】语音状态机/看门狗/识别器装配已整体迁往 ui/voice 六件套
// (VoiceSessionReducer + VoiceController + SystemVoiceEngine/SenseVoiceEngine),
// 旧页面胶水(voiceReduce/VoiceInputUi/VoiceEvent/PaVoiceRecognizer/
// createVoiceRecognizer/createSystemVoiceRecognizer/startVoiceWatchdog/死区墙钟
// 纯函数)已随接线切换整体删除。本区只保留:
//  1. 旧七类错误映射(VoiceErrorKind/voiceErrorKind/systemVoiceRetireNeeded)——
//     ui/voice/SystemVoiceEngine 以 import 复用其为单一真值源,不得删除或改名;
//  2. joinVoiceText / voiceApplyToInput —— Final 落框的追加/替换纯逻辑;
//  3. 新枚举文案映射 voiceErrorText(DomainVoiceErrorKind)重载 + voiceEngineLabel
//     + pickVoiceEngine(域层 VoiceEngineKind 决策表)。
// =====================================================================================

/**
 * 【遗留桥,勿删】识别失败的旧七类分类:与 SpeechRecognizer 错误码的映射在
 * [voiceErrorKind],ui/voice/SystemVoiceEngine 经 `systemVoiceErrorKind` 按名搬运到
 * 域层新枚举(ui.voice.VoiceErrorKind,新增死区/端侧三类)。本枚举不再被页面接线
 * 直接消费,仅作为错误码→分类的单一真值源存在。
 * 【ENGINE】端侧 SenseVoice 引擎专用(识别器/录音器异常),系统识别不会产生。
 * 【MIC_BLOCKED】端侧引擎报"运行时权限已给但仍采不了音"(错误码 102):MIUI 等国产
 *  ROM 在运行时权限之外还有独立的「录音」开关,或麦克风被其他应用占用 —— 人话提示
 *  直接给"去哪里开"的可行动指引(此前的静默失败点:102 漏映射落 OTHER,用户只看到
 *  千篇一律的"识别失败,请重试",永远不知道去安全中心开开关)。
 */
internal enum class VoiceErrorKind { NO_MATCH, BUSY, INSUFFICIENT_PERMISSIONS, CLIENT, OTHER, ENGINE, MIC_BLOCKED }

/** 识别文本并入输入框:空输入直接放;否则补一个空格再接(不覆盖已输入内容)。 */
internal fun joinVoiceText(existing: String, spoken: String): String = when {
    existing.isBlank() -> spoken
    existing.endsWith(" ") || existing.endsWith("\n") -> existing + spoken
    else -> "$existing $spoken"
}

// ---- 端侧兜底:引擎选择决策(纯函数,JVM 单测锁定;枚举为 ui.voice 域层类别) ----

/**
 * 引擎选择决策表(两级,语义与旧表一致;接线层照单装配,见 ChatScreen 的
 * bindEngine LaunchedEffect):
 *  · 系统可用            → SYSTEM_ON_DEVICE(系统侧;具体 on-device/远程由
 *    SystemVoiceEngine.create 工厂实测判定,决策表只表达"系统优先");
 *  · 系统不可用+模型就绪 → SENSEVOICE(小米等无识别服务 ROM 的落地);
 *  · 都不行              → UNAVAILABLE(点击麦克风给人话提示,内置模型初始化完成后
 *    随 senseReady 重绑为 SENSEVOICE)。
 */
internal fun pickVoiceEngine(sysAvailable: Boolean, senseReady: Boolean): VoiceEngineKind = when {
    sysAvailable -> VoiceEngineKind.SYSTEM_ON_DEVICE
    senseReady -> VoiceEngineKind.SENSEVOICE
    else -> VoiceEngineKind.UNAVAILABLE
}

/** 旧七类错误分类 → 人话提示(Toast 用;单一真值源,新枚举文案对其委托,见下方重载)。 */
internal fun voiceErrorText(kind: VoiceErrorKind): String = when (kind) {
    VoiceErrorKind.NO_MATCH -> "没听到,再试一次"
    VoiceErrorKind.BUSY -> "语音服务正忙,稍后再试"
    VoiceErrorKind.INSUFFICIENT_PERMISSIONS -> "系统语音服务权限受限,已自动切换端侧识别;若仍失败,请在系统设置里开启麦克风权限"
    VoiceErrorKind.CLIENT -> "语音识别不可用"
    VoiceErrorKind.OTHER -> "识别失败,请重试"
    VoiceErrorKind.ENGINE -> "端侧语音引擎异常,请重试"
    // 可行动指引:MIUI 的「录音」是运行时权限之外独立的一道开关,必须点名指路
    VoiceErrorKind.MIC_BLOCKED ->
        "麦克风被系统拦截:请在 系统设置→应用→PocketAgent→权限 中允许「麦克风」;小米/红米还需在 安全中心→权限管理 中允许「录音」,并确认麦克风未被通话等其他应用占用"
}

/** SpeechRecognizer 错误码(含端侧引擎自定义码)→ 旧七类分类(SystemVoiceEngine 单一真值源)。 */
internal fun voiceErrorKind(code: Int): VoiceErrorKind = when (code) {
    SpeechRecognizer.ERROR_NO_MATCH, SpeechRecognizer.ERROR_SPEECH_TIMEOUT -> VoiceErrorKind.NO_MATCH
    SpeechRecognizer.ERROR_RECOGNIZER_BUSY -> VoiceErrorKind.BUSY
    SpeechRecognizer.ERROR_INSUFFICIENT_PERMISSIONS -> VoiceErrorKind.INSUFFICIENT_PERMISSIONS
    SpeechRecognizer.ERROR_CLIENT -> VoiceErrorKind.CLIENT
    // 【端侧兜底】端侧引擎自定义错误码(100+):归"端侧引擎异常"
    // (旧遗留引擎的 101=模型加载失败已随 2026-09-20 摘除移除:该码不再由任何
    //  引擎产生,若出现落 else 兜底 OTHER,不误报引擎异常)
    VOICE_ERROR_ENGINE -> VoiceErrorKind.ENGINE
    // 【静默失败修复】端侧引擎的"麦克风被拦"(MIUI「录音」开关/被其他应用占用)与
    // "引擎忙"此前漏映射,落到 OTHER 只会说"识别失败,请重试"——MIUI 用户在安全
    // 中心关了「录音」开关时,永远得不到"去哪里开"的可行动指引(真机主诉根因之一)。
    VOICE_ERROR_MIC_BLOCKED -> VoiceErrorKind.MIC_BLOCKED
    VOICE_ERROR_ENGINE_BUSY -> VoiceErrorKind.BUSY
    else -> VoiceErrorKind.OTHER
}

/**
 * 系统识别错误码 → 是否应弃用系统引擎改选端侧(纯函数,JVM 单测锁定)。
 *
 * 【为什么存在】引擎选择"系统优先且粘滞":只要 isRecognitionAvailable=true 就恒选
 * SYSTEM,而小米等 ROM 上系统识别服务"名义可用实际不可用"的失败形态很多 —— 若这些
 * 失败只弹提示不弃用,用户会永远循环"点击→失败→再点→再失败",端侧 SenseVoice 明明
 * 内置就绪却永远轮不上(真机主诉"语音根本用不了"的主根因)。
 *
 * 覆盖的错误码:
 *  1 网络超时 / 2 网络 / 3 录音错误(MIUI「录音」开关关闭时系统识别的典型表现) /
 *  4 服务端 / 5 客户端 / 8 忙(常驻=服务半死) /
 *  9 识别服务自身无权限(HyperOS 真机实证:fuxi 上 App 的 RECORD_AUDIO 已授予,
 *    系统识别服务仍瞬间报 9,每次点击必现且永不可自愈 —— 旧假设"用户授权后可自愈"
 *    被证伪;若真是 App 权限被拒,弃用后端侧 SenseVoice 会报 102 并给出精确的
 *    「安全中心→录音」指引,两条路都诚实可行动) / 11 服务断连 / 12-13 语言不支持 ——
 *  在无 GMS 的国产 ROM 上这些几乎都修不好,弃用后识别器重建为端侧引擎。
 * 不覆盖:6/7(没听到)是正常业务结果;100+(端侧自定义码)到达时系统引擎本就未被选中,不能误伤。
 */
internal fun systemVoiceRetireNeeded(code: Int): Boolean =
    code in setOf(1, 2, 3, 4, 5, 8, 9, 11, 12, 13)

// =====================================================================================
// P4 Task 6 —— 语音接线纯函数(新枚举文案/引擎读屏名/落框追加与替换)
// =====================================================================================

/**
 * 域层错误分类(ui.voice.VoiceErrorKind)→ 人话提示(Toast 用)。
 * 共享七类沿用旧文案(委托 [voiceErrorText],真机文案零漂移);端侧四类为新增:
 *  · DEAD_ZONE     = 旧 voiceDeadZoneToast(false) 的"服务无响应";
 *  · MODEL_LOADING = 旧 voiceDeadZoneToast(true) 的"加载语音模型"(冷加载分流);
 *  · MODEL_CORRUPTED / STORAGE_FULL = 端侧模型损坏 / 磁盘空间不足(计划要求分别给文案)。
 */
internal fun voiceErrorText(kind: DomainVoiceErrorKind): String = when (kind) {
    DomainVoiceErrorKind.NO_MATCH -> voiceErrorText(VoiceErrorKind.NO_MATCH)
    DomainVoiceErrorKind.BUSY -> voiceErrorText(VoiceErrorKind.BUSY)
    DomainVoiceErrorKind.INSUFFICIENT_PERMISSIONS -> voiceErrorText(VoiceErrorKind.INSUFFICIENT_PERMISSIONS)
    DomainVoiceErrorKind.CLIENT -> voiceErrorText(VoiceErrorKind.CLIENT)
    DomainVoiceErrorKind.OTHER -> voiceErrorText(VoiceErrorKind.OTHER)
    DomainVoiceErrorKind.ENGINE -> voiceErrorText(VoiceErrorKind.ENGINE)
    DomainVoiceErrorKind.MIC_BLOCKED -> voiceErrorText(VoiceErrorKind.MIC_BLOCKED)
    DomainVoiceErrorKind.DEAD_ZONE -> "语音服务无响应,请重试或检查语音服务"
    DomainVoiceErrorKind.MODEL_LOADING -> "正在加载语音模型,请稍候再试"
    DomainVoiceErrorKind.MODEL_CORRUPTED -> "端侧语音模型损坏:请重新安装 App 后再试"
    DomainVoiceErrorKind.STORAGE_FULL -> "手机存储空间不足:清理存储后再试语音识别"
}

/** 引擎类别 → 读屏/日志用短名(进 contentDescription,不进正文)。 */
internal fun voiceEngineLabel(kind: VoiceEngineKind): String = when (kind) {
    VoiceEngineKind.SYSTEM_ON_DEVICE -> "系统识别(本机)"
    VoiceEngineKind.SYSTEM_REMOTE -> "系统识别(在线)"
    VoiceEngineKind.SENSEVOICE -> "端侧识别"
    VoiceEngineKind.UNAVAILABLE -> "语音引擎未就绪"
}

/**
 * 识别完成文本并入输入框(Task 6):replace=false 追加([joinVoiceText],空格衔接,
 * 不覆盖已有内容);replace=true 替换(覆盖输入框)。两种模式都不改写原文的大小写
 * 与标点 —— 对英文/路径/命令逐字保留(计划全局约束)。
 */
internal fun voiceApplyToInput(existing: String, spoken: String, replace: Boolean): String =
    if (replace) spoken else joinVoiceText(existing, spoken)

// 【P4 Task 5 删除说明】旧识别器装配胶水 createVoiceRecognizer / createSystemVoiceRecognizer
// (含 onResults/onPartialResults 的 transcript 全文日志、墙钟死区看门狗喂狗钩子)已整体删除:
// 装配改由 ui/voice/SystemVoiceEngine.create(预检 + on-device 优先 + startListening 同步异常
// 兜底)与 SenseVoiceEngine.create 接管,回调经 VoiceEngineEvent 流入 VoiceController,
// 生产日志只记引擎/耗时/字符数/错误分类,绝不落 transcript(Global Constraint)。
