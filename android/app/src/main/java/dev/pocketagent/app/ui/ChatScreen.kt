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
import androidx.compose.runtime.produceState
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
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
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
import dev.pocketagent.app.ui.model.cardEffortLevels
import dev.pocketagent.app.ui.model.modelDisplayLabel
import androidx.core.content.ContextCompat
import dev.pocketagent.app.ui.component.PaNotifications
import dev.pocketagent.app.ui.component.PaDownloader
import dev.pocketagent.app.ui.component.PaSheet
import dev.pocketagent.app.ui.component.PaHaptics
import dev.pocketagent.app.ui.component.PaStatusGlyphSlot
import dev.pocketagent.app.ui.component.ConnectionBanner
import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.scaleIn
import androidx.compose.ui.platform.LocalView
import dev.pocketagent.app.ui.component.DeliveryStatusBadge
import dev.pocketagent.app.ui.component.deliveryRetryable
import dev.pocketagent.app.ui.component.deliveryVisual
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
import dev.pocketagent.app.ui.chat.ChatApprovalFlow
import dev.pocketagent.app.ui.chat.ChatComposerArea
import dev.pocketagent.app.ui.chat.ChatAttachmentViewer
import dev.pocketagent.app.ui.chat.ChatContextUsagePanel
import dev.pocketagent.app.ui.chat.ChatGoalSheet
import dev.pocketagent.app.ui.chat.ChatMoreSheet
import dev.pocketagent.app.ui.chat.ChatQuickModelsPanel
import dev.pocketagent.app.ui.chat.ChatRollbackSheet
import dev.pocketagent.app.ui.chat.ChatHostStatusArea
import dev.pocketagent.app.ui.chat.ChatIntent
import dev.pocketagent.app.ui.chat.ChatTimelineArea
import dev.pocketagent.app.ui.chat.ChatViewModel
import dev.pocketagent.app.ui.history.HistoryLocatePlan
import dev.pocketagent.app.ui.history.HistorySearchHit
import dev.pocketagent.app.ui.history.HistorySearchRepository
import dev.pocketagent.app.ui.history.HistorySearchState
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
import java.util.concurrent.atomic.AtomicBoolean
import pocketagent.v1.ApprovalRequest
import pocketagent.v1.CodexEvent
import pocketagent.v1.CreateThreadRequest
import pocketagent.v1.CodexSettings
import pocketagent.v1.GetHostInfoRequest
import pocketagent.v1.GitStatusRequest
import pocketagent.v1.GitDiffRequest
import pocketagent.v1.GitCommitRequest
import pocketagent.v1.GetThreadHistoryRequest
import pocketagent.v1.SearchThreadMessagesRequest
import pocketagent.v1.GetThreadRequest
import pocketagent.v1.ExportThreadHtmlRequest
import pocketagent.v1.ModelInfo
import pocketagent.v1.Thread as PThread
import pocketagent.v1.ThreadStatus
import pocketagent.v1.WriteFileRequest

/**
 * ChatItem 稳定 id 的进程级序号:每条进入时间线的 ChatItem 都在**构造处**领一个
 * 全进程唯一的 id(见 [nextChatItemId]),历史回填则优先用 daemon 的 message_id
 * 合成(见 [historyChatItemId])—— LazyColumn 的 itemsIndexed 拿它做 key,
 * 历史前插(addAll(0,…))时既有条目 key 不位移,可见区 item 不再整体重执行。
 */
private val chatItemIdSeq = java.util.concurrent.atomic.AtomicLong(0)

/** 生成一个新的 ChatItem 稳定 id(实时帧/落卡/快照等一次性构造统一走这里)。 */
internal fun nextChatItemId(role: String): String = "ci-$role-${chatItemIdSeq.incrementAndGet()}"

/**
 * 历史回填条目的稳定 id:daemon 带 message_id 时用 "hm-<id>#<块序号>"
 * (同一条消息重拉得到同一 key,整表回填后 LazyColumn 仍能按 key 复用节点);
 * 旧 daemon 无 id 时退进程级序号(每次构造不同,行为等同旧的按位置匹配)。
 */
internal fun historyChatItemId(messageId: String, subIndex: Int): String =
    if (messageId.isNotBlank()) "hm-$messageId#$subIndex" else nextChatItemId("hist")

/** 一条聊天记录(用户/助手/工具过程/思考过程/执行计划)。 */
data class ChatItem(
    val role: String, // user / assistant / tool / thinking / plan / codex_event / items_trimmed
    val text: String = "",
    val toolName: String = "",
    val toolStatus: String = "", // running / success / failure
    val toolId: String = "",
    val toolArgs: String = "",
    val toolElapsedMs: Double = 0.0,
    val streaming: Boolean = false,
    /**
     * 构造时捕获的附件快照(C2 稳定化审计):写入点只有 composer 的整体重赋值
     * (`attachments + uploaded` / `- at` / `= emptyList()`,每次都产新列表)与
     * 历史映射的 `listOf(...)` 一次性构造 —— **构造后绝不再 mutate**,持有旧引用
     * 的 ChatItem 永远看到同一份不可变内容。
     */
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
    /**
     * 时间线 key(C2):构造处领的全进程唯一 id,itemsIndexed 拿它做 key。
     * 空串 = 旧构造路径漏领(历史遗留/usage 等不进时间线的角色),key 回退
     * 按位置生成,绝不因重复空串崩 LazyColumn。原地更新(工具卡/审查卡/投递
     * 标签)必须沿用被替换实例的 id(copy 天然保留),否则节点被拆掉重建。
     */
    val id: String = "",
    /**
     * role == "items_trimmed" 时有效:C3 内存上限截断后,中间被折叠的**早期
     * 原始消息**条数(含此前一轮截断已折叠的累计)。渲染为「⋯ 已折叠 N 条早期
     * 消息」细行;不进 checkpoint / 导出 / 历史导航(那些路径按 role 过滤)。
     */
    val trimmedCount: Int = 0,
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
    // C3 折叠标记行(items_trimmed)两种主题同一渲染形态(细行),桶固定为专属
    // 字符串:不随 hackerMode 在 OUTPUT/role 桶间漂移,滚动回收不与正文互毁。
    if (item.role == "items_trimmed") "items_trimmed"
    else if (hackerMode) terminalBlockKind(item) else item.role

/** 已上传到电脑的附件(手机 → daemon 工作区 .pocketagent-uploads)。 */
data class Attachment(
    val name: String,
    val pcPath: String,
    val size: Int,
    val mediaType: String = "",
    val localUri: String? = null,
    val previewData: ByteArray? = null,
)

internal fun loadAttachmentPreview(context: Context, attachment: Attachment): ImageBitmap? {
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

/**
 * 附件缩略图解码目标(px):时间线附件卡按 64dp 展示(AttachmentPreview.Image),
 * 按 2x 密度取 ~128px 解析度 —— 全尺寸原图(1-5MB)按 64dp 显示是纯浪费的解码与内存。
 */
internal const val ATTACHMENT_THUMBNAIL_TARGET_PX = 128

/**
 * 两段解码的抽样档(纯函数,可 JVM 单测):返回把宽高都压到 [targetPx] 附近的
 * 最小 2^n inSampleSize;原尺寸已不超过目标(或尺寸未知)按 1(原样解码)。
 */
internal fun attachmentThumbnailSampleSize(
    width: Int,
    height: Int,
    targetPx: Int = ATTACHMENT_THUMBNAIL_TARGET_PX,
): Int {
    if (width <= 0 || height <= 0) return 1
    var sample = 1
    var w = width
    var h = height
    while (w / 2 >= targetPx && h / 2 >= targetPx) {
        w /= 2
        h /= 2
        sample *= 2
    }
    return sample
}

/**
 * 附件缩略图加载(P1-8):两段解码(inJustDecodeBounds 量尺寸 → inSampleSize 抽样),
 * 只解出 ~128px 的小图。旧路径 [loadAttachmentPreview] 直接 decodeByteArray 全图,
 * 卡片列表滚动时每张附件都按原尺寸(可达数 MB 像素)解码,解码结果还只显示 64dp。
 * 全尺寸解码保留给附件大图预览(ChatAttachmentViewer 的满幅检视),互不影响。
 */
internal fun loadAttachmentThumbnail(context: Context, attachment: Attachment): ImageBitmap? {
    return runCatching {
        val data = attachment.previewData
        if (data != null) {
            val bounds = BitmapFactory.Options().apply { inJustDecodeBounds = true }
            BitmapFactory.decodeByteArray(data, 0, data.size, bounds)
            val opts = BitmapFactory.Options().apply {
                inSampleSize = attachmentThumbnailSampleSize(bounds.outWidth, bounds.outHeight)
            }
            return@runCatching BitmapFactory.decodeByteArray(data, 0, data.size, opts)?.asImageBitmap()
        }
        val uri = attachment.localUri?.let(Uri::parse) ?: return@runCatching null
        // 流不可重读:量尺寸与抽样解码各开一次流
        val bounds = BitmapFactory.Options().apply { inJustDecodeBounds = true }
        context.contentResolver.openInputStream(uri)?.use { BitmapFactory.decodeStream(it, null, bounds) }
        val opts = BitmapFactory.Options().apply {
            inSampleSize = attachmentThumbnailSampleSize(bounds.outWidth, bounds.outHeight)
        }
        context.contentResolver.openInputStream(uri)?.use {
            BitmapFactory.decodeStream(it, null, opts)?.asImageBitmap()
        }
    }.getOrNull()
}

/**
 * 缩略图的组合期加载(P1-8):produceState 桥,解码在 Dispatchers.IO 上跑,
 * 不再阻塞合成帧;key 用 previewData 引用(与 P0-2 的 remember key 同一口径,
 * ByteArray equals = 引用比较,不再对整图做 contentHashCode)。解码完成前返回
 * null(卡片渲染既有兜底图标)。
 */
@Composable
internal fun rememberAttachmentThumbnail(context: Context, attachment: Attachment): ImageBitmap? =
    produceState<ImageBitmap?>(
        initialValue = null,
        attachment.previewData,
        attachment.localUri,
    ) {
        if (attachment.previewData == null && attachment.localUri == null) return@produceState
        value = kotlinx.coroutines.withContext(kotlinx.coroutines.Dispatchers.IO) {
            loadAttachmentThumbnail(context, attachment)
        }
    }.value

@Composable
internal fun CodexEventCard(
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
            .background(bg, RoundedCornerShape(PaEditorial.Radius.IconButton))
            .border(
                1.dp,
                if (PaHacker.isHacker) accent.copy(alpha = 0.35f) else PaEditorial.Hairline,
                RoundedCornerShape(PaEditorial.Radius.IconButton),
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
                        .background(accent.copy(alpha = 0.16f), RoundedCornerShape(PaEditorial.Radius.Terminal))
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
        // 【D1 diff 着色 2026-10-09】按行首标记上色:+ → Running 系绿、- → Denied 系红、
        // @@ → 哑色(OnGraphiteSubtle),其余行保持代码箱本色 —— 取色与
        // PromptOptimizeDiff 的红删高亮同一套编辑式语义档(纯函数 codeBoxSpans,JVM 单测钉死)。
        if (event.diff.isNotBlank()) {
            val diffColors = codeBoxColors(if (PaHacker.isHacker) PaEditorial.Muted else PaEditorial.CodeText)
            val diffAnnotated = remember(event.diff, diffColors) {
                codeBoxAnnotated(event.diff, diffColors, CodeBoxKind.DIFF)
            }
            Text(
                diffAnnotated,
                Modifier
                    .fillMaxWidth()
                    .heightIn(max = 260.dp)
                    .verticalScroll(rememberScrollState())
                    .horizontalScroll(rememberScrollState())
                    .background(
                        if (PaHacker.isHacker) PaEditorial.Canvas else PaEditorial.GraphiteCode,
                        RoundedCornerShape(PaEditorial.Radius.Chip),
                    )
                    .padding(8.dp),
                color = if (PaHacker.isHacker) PaEditorial.Muted else PaEditorial.CodeText,
                fontSize = (10 * uiScale).sp,
                fontFamily = PaEditorial.MonoFont,
            )
        }
        if (event.url.isNotBlank()) {
            Text(
                event.url,
                color = if (PaHacker.isHacker) PaEditorial.Subtle else PaEditorial.Muted,
                fontSize = (10 * uiScale).sp,
                fontFamily = PaEditorial.MonoFont,
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
                        RoundedCornerShape(PaEditorial.Radius.Chip),
                    )
                    .padding(8.dp),
                color = if (PaHacker.isHacker) PaEditorial.Muted else PaEditorial.CodeText,
                fontSize = (10 * uiScale).sp,
                fontFamily = PaEditorial.MonoFont,
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
internal fun MessageEnterAnimation(
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
internal fun ThinkingCard(text: String, scale: Float, streaming: Boolean) {
    var expanded by remember(streaming) { mutableStateOf(streaming) }
    Column(
        Modifier
            .fillMaxWidth()
            .background(PanelCardColor, RoundedCornerShape(PaEditorial.Radius.IconButton))
            .border(1.dp, PanelHairlineColor, RoundedCornerShape(PaEditorial.Radius.IconButton))
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
                fontSize = PaEditorial.Type.Caption.fontSize,
                color = ChatAccentColor,
                fontWeight = FontWeight.SemiBold,
                maxLines = 1,
            )
            Text(if (expanded) "收起" else "展开", fontSize = PaEditorial.Type.Caption.fontSize, color = ChatSubtleColor)
        }
        if (expanded) {
            Text(
                text,
                Modifier.fillMaxWidth().padding(top = 6.dp),
                fontSize = (11.5f * scale).sp,
                fontFamily = PaEditorial.MonoFont,
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
internal fun PlanPanel(
    steps: List<String>,
    /**
     * 流式草稿的 provider(P2-13):在面板内部读取 —— 只有展开区真正消费草稿,
     * planStream 的 delta 订阅就收在 PlanPanel(展开时)里,不再让调用方
     * (整个时间线区)随每个 delta 重组。
     */
    draft: () -> String,
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
                .background(PanelCardColor, RoundedCornerShape(PaEditorial.Radius.IconButton))
                .border(1.dp, PanelHairlineColor, RoundedCornerShape(PaEditorial.Radius.IconButton))
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
                    fontSize = PaEditorial.Type.Caption.fontSize,
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
                        fontSize = PaEditorial.Type.Caption.fontSize,
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
                            RoundedCornerShape(PaEditorial.Radius.Terminal),
                        ),
                ) {
                    Box(
                        Modifier
                            .fillMaxWidth(if (total == 0) 0f else done.toFloat() / total)
                            .height(3.dp)
                            .background(
                                if (running) ChatAccentColor else ChatRunningColor,
                                RoundedCornerShape(PaEditorial.Radius.Terminal),
                            ),
                    )
                }
            }

            // ---- 展开区:完整步骤 + 生成中的草稿 ----
            // 【P2-13】draft() 的读取(即 planStream 订阅)只发生在展开分支内:
            // 面板收起时不读草稿,流式 delta 零重组;展开时也只有 PlanPanel 子树重组。
            if (expanded) {
                Spacer(Modifier.height(8.dp))
                if (parsed.isEmpty()) {
                    // 还没有结构化步骤:把草稿按可读段落排版
                    PlanDraftText(draft())
                } else {
                    parsed.forEach { (status, text) ->
                        Row(
                            Modifier.fillMaxWidth().padding(vertical = 3.dp),
                            verticalAlignment = Alignment.Top,
                        ) {
                            // D4-2 步骤点升级为"点/勾"形槽:pending→completed 的瞬间
                            // 颜色 spring 过渡 + 勾形 spring 长出 + light 触觉(见 PlanStepDot)。
                            // 槽位固定 10dp,点→勾切换时行高不跳。
                            Box(
                                Modifier
                                    .padding(top = 4.dp)
                                    .size(10.dp),
                                contentAlignment = Alignment.Center,
                            ) {
                                PlanStepDot(status)
                            }
                            Spacer(Modifier.width(8.dp))
                            Text(
                                text,
                                Modifier.weight(1f),
                                fontSize = PaEditorial.Type.Caption.fontSize,
                                color = if (status == "pending") ChatMutedColor else ChatInkColor,
                            )
                        }
                    }
                    // 步骤已出但仍在跑:把最新草稿附在下面(可能是下一步的说明)
                    val draftNow = draft()
                    if (running && draftNow.isNotBlank()) {
                        Spacer(Modifier.height(6.dp))
                        PlanDraftText(draftNow)
                    }
                }
            }
        }
    }
}

/**
 * 计划步骤的状态点(D4-2 精修):pending 灰点 / in_progress 主色点 / completed
 * 绿色勾形 —— 勾形与 PaStatusBadge 的四词形通道(CHECK)同族,"完成"不再只靠
 * 颜色辨(色弱可辨)。
 *
 * 【动效】pending→completed 瞬间:颜色 animateColorAsState(PaMotion.press 档,
 * 无过冲不抢戏)+ 勾形经 AnimatedContent spring 缩放长出 —— 状态变化有了
 * "落定感",不是硬切。减弱动效时 animateColorAsState 仍在(纯颜色无位移,
 * 不属减弱范围),勾形直接切换(AnimatedContent 用 instant 过渡)。
 *
 * 【触觉】light 档(PaHaptics.light,刻度级):只在"观察到未完成→完成"的转变
 * 帧打一次 —— 历史加载时整列已完成的步骤**不**齐震(用三态 remember 区分
 * "首次看到"与"真的转变",首帧只记录不触发)。
 */
@Composable
private fun PlanStepDot(status: String) {
    val view = LocalView.current
    val reduceMotion = rememberReduceMotion()
    val dotColor by animateColorAsState(
        targetValue = when (status) {
            "completed" -> ChatRunningColor
            "in_progress" -> ChatAccentColor
            else -> ChatSubtleColor
        },
        animationSpec = PaMotion.press(),
        label = "planStepColor",
    )
    // 触觉的门闩:null=本组合还没观察过(首帧只记录);false→true 的转变才打
    var seenCompleted by remember { mutableStateOf<Boolean?>(null) }
    LaunchedEffect(status) {
        val completed = status == "completed"
        if (completed && seenCompleted == false) PaHaptics.light(view)
        seenCompleted = completed
    }
    AnimatedContent(
        targetState = status == "completed",
        transitionSpec = {
            if (reduceMotion) {
                fadeIn(PaMotion.instant()) togetherWith fadeOut(PaMotion.instant())
            } else {
                (fadeIn(PaMotion.press()) + scaleIn(PaMotion.press(), initialScale = 0.4f))
                    .togetherWith(fadeOut(PaMotion.standard()))
            }
        },
        label = "planStepGlyph",
    ) { completed ->
        if (completed) {
            Icon(
                painterResource(AppR.drawable.ic_lucide_check), null,
                tint = dotColor, modifier = Modifier.size(10.dp),
            )
        } else {
            Box(Modifier.size(6.dp).background(dotColor, CircleShape))
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
                    fontSize = PaEditorial.Type.Caption.fontSize,
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
internal fun ToolCallCard(item: ChatItem, scale: Float) {
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
            .background(PanelCardColor, RoundedCornerShape(PaEditorial.Radius.IconButton))
            .border(1.dp, PanelHairlineColor, RoundedCornerShape(PaEditorial.Radius.IconButton))
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
            // D5-1 状态点接形通道:裸 8dp 色点 → PaStatusGlyphSlot(与 PaStatusBadge
            // 同族的环/点/勾/叹号槽)—— 词(右侧状态词)+ 色 + 形三通道,
            // 失败=叹号/成功=勾/执行中=点,色弱也能扫一眼分清。
            PaStatusGlyphSlot(
                glyph = when (item.toolStatus) {
                    "failure" -> PaStatusGlyph.EXCLAMATION
                    "success" -> PaStatusGlyph.CHECK
                    else -> PaStatusGlyph.DOT
                },
                tint = statusColor,
            )
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
                    fontSize = PaEditorial.Type.Micro.fontSize,
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
                fontSize = PaEditorial.Type.Micro.fontSize,
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
                // 【D5-2 双色代码箱】参数是 JSON:行内 key 一色(Mcp 紫)、value 保持
                // 代码箱本色(正则级识别,不做完整 parser);输出区先按 diff 检测
                // (比例>60% 启用 +/绿 -/红 @@/哑色),再退 JSON,再退单色 —— 全部
                // 纯函数 codeBoxAnnotated,JVM 单测钉死。
                val codeBoxColor = if (PaHacker.isHacker) PaEditorial.Canvas else PaEditorial.GraphiteCode
                val codeTextColor = if (PaHacker.isHacker) PaEditorial.Ink else PaEditorial.CodeText
                val boxColors = codeBoxColors(codeTextColor)
                if (item.toolArgs.isNotBlank()) {
                    // 标签行:段名 + 「复制」—— 代码箱自带复制口,长 JSON/命令不必靠
                    // 长按拖选(选区工具栏在代码箱里也难用)。轻触感由 pressable 自带。
                    Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                        Text("参数", Modifier.weight(1f), fontSize = PaEditorial.Type.Micro.fontSize, color = ChatSubtleColor)
                        ToolCardCopyChip("复制", item.toolArgs)
                    }
                    val argsAnnotated = remember(item.toolArgs, boxColors) {
                        codeBoxAnnotated(item.toolArgs, boxColors)
                    }
                    Text(
                        argsAnnotated,
                        Modifier
                            .fillMaxWidth()
                            .padding(top = 2.dp)
                            .background(codeBoxColor, RoundedCornerShape(PaEditorial.Radius.Chip))
                            // 高度封顶 + 纵横双向滚动:超长参数/输出不再把卡撑成一屏,
                            // 长行保横向滚动不硬折(diff/JSON 的对齐语义,同回合 diff 口径)。
                            .heightIn(max = 260.dp)
                            .padding(8.dp)
                            .verticalScroll(rememberScrollState())
                            .horizontalScroll(rememberScrollState()),
                        fontSize = (10.5f * scale).sp,
                        fontFamily = PaEditorial.MonoFont,
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
                            fontSize = PaEditorial.Type.Micro.fontSize,
                            color = ChatSubtleColor,
                        )
                        ToolCardCopyChip("复制", item.text)
                    }
                    val outputShown = if (showFullOutput) item.text else preview.visible
                    // 失败输出的整红是状态语义(未完成),优先于装饰性着色,不做 diff/JSON 着色
                    val outputText =
                        if (item.toolStatus == "failure") AnnotatedString(outputShown)
                        else remember(outputShown, boxColors) { codeBoxAnnotated(outputShown, boxColors) }
                    Text(
                        outputText,
                        Modifier
                            .fillMaxWidth()
                            .padding(top = 2.dp)
                            .background(codeBoxColor, RoundedCornerShape(PaEditorial.Radius.Chip))
                            .heightIn(max = 260.dp)
                            .padding(8.dp)
                            .verticalScroll(rememberScrollState())
                            .horizontalScroll(rememberScrollState()),
                        fontSize = (10.5f * scale).sp,
                        fontFamily = PaEditorial.MonoFont,
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
                            fontSize = PaEditorial.Type.Caption.fontSize,
                            fontWeight = FontWeight.SemiBold,
                            color = ChatAccentColor,
                        )
                    }
                }
                if (item.toolArgs.isBlank() && item.text.isBlank()) {
                    Text("等待结果…", Modifier.padding(top = 8.dp), fontSize = PaEditorial.Type.Caption.fontSize, color = ChatSubtleColor)
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
        fontSize = PaEditorial.Type.Caption.fontSize,
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
internal val PanelCardColor: Color get() = PaEditorial.Surface
/** 通用细线。 */
internal val PanelHairlineColor: Color get() = PaEditorial.Hairline
/** 琥珀提示面的描边/文字(离线横幅与待审批入口同族)。 */
internal val OfflineBannerOutline: Color get() = PaEditorial.AttentionContainer
internal val OfflineBannerText: Color get() = PaEditorial.AttentionText
/** 正文主色。 */
internal val ChatInkColor: Color get() = PaEditorial.Ink
/** 次级文字。 */
internal val ChatMutedColor: Color get() = PaEditorial.Muted
/** 三级文字。 */
internal val ChatSubtleColor: Color get() = PaEditorial.Subtle
/** 语义强调(进行中/可点链接)。 */
internal val ChatAccentColor: Color get() = PaEditorial.Primary
/** 语义绿(运行/成功)。 */
internal val ChatRunningColor: Color get() = PaEditorial.Running
/** 语义红(未完成/拒绝)。 */
internal val ChatDeniedColor: Color get() = PaEditorial.Denied

// =====================================================================================
// 编辑式会话页新增小组件(2026-09-29 UI 整体升级·第二波 2/4)
// 私有于会话页;共享组件(PaHostChip/PaStatusBadge/PaApprovalDialog 等)在 ui/component。
// =====================================================================================

/**
 * 投递徽标(编辑式):图标 + 状态词 + 可选详情,四态措辞见 [editorialDeliveryCopy]。
 * 色:未送达=珊瑚红;已送达=绿;等待=次级灰(进行中的中性,主色只给"当前操作")。
 */
@Composable
internal fun EditorialDeliveryBadge(state: String, queuePos: Int) {
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
        Text(copy.word, fontSize = PaEditorial.Type.Micro.fontSize, fontWeight = FontWeight.SemiBold, color = tint)
        if (copy.detail.isNotBlank()) {
            Spacer(Modifier.width(4.dp))
            Text(
                "· ${copy.detail}",
                fontSize = PaEditorial.Type.Micro.fontSize,
                fontWeight = FontWeight.SemiBold,
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
            fontSize = PaEditorial.Type.BodySmall.fontSize,
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
internal fun ChatOfflineBanner(copy: String, onRetry: () -> Unit) {
    Row(
        Modifier
            .fillMaxWidth()
            .background(PaEditorial.AttentionContainer, RoundedCornerShape(PaEditorial.Radius.IconButton))
            .border(1.dp, OfflineBannerOutline, RoundedCornerShape(PaEditorial.Radius.IconButton))
            .padding(horizontal = 14.dp, vertical = 10.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Icon(
            painterResource(AppR.drawable.ic_lucide_refresh_cw), null,
            tint = PaEditorial.Attention, modifier = Modifier.size(16.dp),
        )
        Spacer(Modifier.width(10.dp))
        Column(Modifier.weight(1f)) {
            Text(copy, fontSize = PaEditorial.Type.Caption.fontSize, lineHeight = 17.sp, color = OfflineBannerText)
            Text(
                "草稿已保存在此设备，恢复连接后由你决定是否发送",
                fontSize = PaEditorial.Type.Caption.fontSize,
                lineHeight = 16.sp,
                color = OfflineBannerText.copy(alpha = 0.85f),
            )
        }
        Text(
            "重试",
            Modifier
                .pressable { onRetry() }
                .padding(horizontal = 8.dp, vertical = 8.dp),
            fontSize = PaEditorial.Type.BodySmall.fontSize,
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
internal fun ApprovalEntryCard(onOpen: () -> Unit) {
    Row(
        Modifier
            .fillMaxWidth()
            .pressable(haptic = true) { onOpen() }
            .semantics { contentDescription = "打开审批弹窗：等待你确认命令执行" }
            .background(PaEditorial.AttentionContainer, RoundedCornerShape(PaEditorial.Radius.IconButton))
            .border(1.dp, OfflineBannerOutline, RoundedCornerShape(PaEditorial.Radius.IconButton))
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
                .background(PaEditorial.Attention.copy(alpha = 0.12f), RoundedCornerShape(PaEditorial.Radius.IconButton)),
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
                fontSize = PaEditorial.Type.Body.fontSize,
                fontWeight = FontWeight.Bold,
                color = PaEditorial.Ink,
            )
            Text(
                "查看操作范围和命令 · 点击处理",
                fontSize = PaEditorial.Type.Caption.fontSize,
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
internal const val EXPAND_HINT_THRESHOLD = 80

/**
 * 附件 chip 里文件名的中间省略上限(见 [middleEllipsize])。
 * 取值远大于常见文件名,保证正常名字完整可读(配合最多 3 行换行);
 * 只有病态长名才退化为"保留扩展名的中间省略"。
 */
internal const val ATTACH_CHIP_NAME_MAX = 80

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
    // ---- 【P3 Task 8 第三层】TurnSession 交互协议面:只读投影单点 collect ----
    // 「只读展示 + 条件判断」的状态读取收进 ChatViewModel.session(TurnSessionUiState,
    // snapshotFlow 桥),下方读点改读投影;写入通道(st.items/pendingApproval/
    // errorMessage/approvalError/queuedCount/historyLoaded/草稿)与 TurnSession 命令
    // 通道(send/cancelTurn/retryRecovery/answerApproval/...)仍由本页直接持有。
    // bindSession/unbindSession 成对挂在 DisposableEffect(见下方效果块):绑定体
    // 同步播种投影(首帧即真实值,不留默认态闪帧),随会话换绑,离开页面解绑。
    // bindSession 的副作用挂 DisposableEffect(C4):绑定/解绑对称成对,随会话
    // 换绑(onDispose 旧会话 → effect 体绑新会话),不再在 remember 初始化器里
    // 做副作用绑定。效果体在组合成功后、首帧绘制前运行,投影播种值同帧生效,
    // 首帧不闪默认态。
    val session by chatVm.session.collectAsState()
    DisposableEffect(chatVm, threadId) {
        chatVm.bindSession(threadId)
        onDispose {
            chatVm.unbindSession()
            // 模型 10s 巡检循环与投影绑定同一生命周期(2026-10-09 效果层外移):
            // 离开页面即停,不留会发 RPC 的在飞循环 —— 与 bindSession/unbindSession 成对。
            chatVm.stopModelInspection()
        }
    }
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
    LaunchedEffect(session.running, session.turnTerminal, session.errorMessage) {
        if (wasTurnRunning && !session.running && session.turnTerminal && session.errorMessage == null) {
            PaHaptics.success(hapticView)
        }
        wasTurnRunning = session.running
    }

    // ---- 语义触觉:审批到达(P1-5)----
    // pendingApproval 从无到有的瞬间是"任务停下来了,等你"—— warning 一下;
    // 从一张卡换到另一张(同卡更新)不重复响,只在"没有→有"边界触发。
    var hadPendingApproval by remember(threadId) { mutableStateOf(false) }
    LaunchedEffect(session.pendingApproval) {
        val now = session.pendingApproval != null
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

    // ---- 【语音交互动画】一次性触发器(纯 UI,不进状态机) ----
    // 【动画 ①】麦克风按下涟漪触发器已随输入区迁至 ChatComposerArea(仅输入区消费)。
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
    val voiceReplaceMode = remember { mutableStateOf(false) }
    // 每轮 Final 只消费一次(以 sessionId 防重:finalText 在下一轮 Start 前一直驻留域状态)。
    var voiceConsumedFinalForSid by remember { mutableStateOf(0L) }
    LaunchedEffect(voiceState.phase, voiceState.sessionId) {
        val spoken = voiceState.finalText
        if (voiceState.phase == VoicePhase.IDLE && spoken.isNotEmpty() &&
            voiceConsumedFinalForSid != voiceState.sessionId
        ) {
            voiceConsumedFinalForSid = voiceState.sessionId
            setInput(voiceApplyToInput(input, spoken, replace = voiceReplaceMode.value))
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

    val showModelCard = remember { mutableStateOf(false) }
    // 【历史消息快速查看】左缘刻度条点开/关闭的浮层(视频样板交互)。
    // rememberSaveable:进程重建(切主题/配置变化)后面板开合态不丢。
    val showHistoryPanel = rememberSaveable { mutableStateOf(false) }
    // 【模型快速切换面板】模型行下拉箭头直接展开的轻面板(单点即切,
    // 不再强制进 ModelEffortCard 二级弹窗;"更多…"仍可进完整选择卡)。
    val showQuickModels = remember { mutableStateOf(false) }
    // 切换成功后的行内反馈 tick:驱动模型名一次轻量闪光(真实选择才有,不造假状态)
    var modelFlashTick by remember { mutableStateOf(0) }
    val showContextCard = remember { mutableStateOf(false) }
    var showDrawer by remember { mutableStateOf(false) }
    // 侧边栏跟手:手势与面板共用同一个进度(0..1),手指在哪面板就在哪
    val drawerCtrl = dev.pocketagent.app.ui.component.rememberDrawerController()
    var showNewThread by remember { mutableStateOf(false) }
    val showGitPanel = remember { mutableStateOf(false) }
    val showExport = remember { mutableStateOf(false) }
    // 子代理卡长按快览(T-SUB):非 null 时弹快览层(渲染在文件中部 sheet 区),
    // 收起不影响任何状态 —— 蓝本 §4.3"不打断主对话浏览的偷看"。
    val peekSub = remember(threadId) { mutableStateOf<SubAgentThreadMeta?>(null) }
    // 「更多」面板:分叉 / 回滚。这两件事都是**偶尔做一次**的会话级动作,
    // 单独给顶栏加两个常驻按钮会让它变挤,收进一个入口更稳。
    val showMore = remember { mutableStateOf(false) }
    // 【内置伪命令"/目标"的弹窗开关】斜杠卡面伪命令点选 = 执行动作:/目标
    // 打开下方的设置目标底栏(slashGoalSheet 块),/压缩 直接走 doCompact(),
    // /计划(review:<agentId> 位会话,现仅 codex)直接走 doStartReview()。
    val slashGoalSheet = remember { mutableStateOf(false) }
    // 回滚选择(二级面板):先选丢几个回合,再确认
    val rollbackOpen = remember { mutableStateOf(false) }
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
    val expandedInputOpen = remember { mutableStateOf(false) }
    // 【执行计划面板】任务完成后计划卡不再散落在时间线里,而是收进会话上方
    // 一个可展开/收起的面板:默认收起(顶部只留一行进度摘要),点开看完整步骤。
    val planPanelOpen = remember { mutableStateOf(false) }
    // 最近一次计划的步骤(时间线里落定的 + 正在流式的,取最新的一份)
    // 【P3 Task 8 第三层】派生已上收 TurnSessionUiState.latestPlan(投影内同式:
    // 时间线 plan 卡取最新,流式草稿兜底),此处只投影。
    val latestPlan = session.latestPlan
    // 【默认展开】计划一出现就展开 —— 对应"未完成时要保持在会话中显示"。
    // 键用 Boolean 而不是步骤列表:只在"从无到有"时触发一次,
    // 之后用户手动收起不会被反复弹开。
    LaunchedEffect(latestPlan.isNotEmpty()) {
        if (latestPlan.isNotEmpty()) planPanelOpen.value = true
    }
    // 侧边栏用的最近会话:和模型/设置一起预取,打开抽屉时直接就是数据,不必等网络
    var recentThreads by remember { mutableStateOf(listOf<PThread>()) }
    var recentLoading by remember { mutableStateOf(true) }

    // 【返回键】分层回退:面板/抽屉打开时先收面板,否则回会话列表(而不是直接退到桌面)。
    // 必须放在这些状态声明之后 —— 放在函数开头会读不到下面声明的 showDrawer 等。
    BackHandler(enabled = true) {
        when {
            drawerCtrl.visible -> drawerCtrl.close()
            showHistoryPanel.value -> showHistoryPanel.value = false
            rollbackOpen.value -> rollbackOpen.value = false
            showMore.value -> showMore.value = false
            showExport.value -> showExport.value = false
            showGitPanel.value -> showGitPanel.value = false
            showContextCard.value -> showContextCard.value = false
            showModelCard.value -> showModelCard.value = false
            // 计划面板展开时,返回先收起面板(它是最贴近"当前屏幕状态"的一层)
            planPanelOpen.value -> planPanelOpen.value = false
            else -> onBack()
        }
    }
    val attachments = remember { mutableStateOf(listOf<Attachment>()) }
    val selectedAttachment = remember { mutableStateOf<Attachment?>(null) }
    // 【PF4】按 threadId 记忆:导航 launchSingleTop 复用同一 ChatScreen 实例,
    // 会话切换必须重置本会话的 daemon 回填值,不能闪现上一会话的残值。
    // 【P3 Task 8 第二层】threadWorkspace/loadError 已收进 ChatViewModel(ChatState)
    // —— 此处为只读投影;写入走 chatVm.send(ThreadHeaderLoaded/WorkspaceResolved/
    // LoadError* 意图,见 getThread 效果/文件上传/模型设置预取/各重试点)。
    // loadError 原为无键 remember(跨会话沿用,靠后续加载覆盖);随 ViewModel 换
    // 会话重建归 null —— 上一会话的失败不再带进新会话,方向与第一层同款更干净。
    val threadWorkspace = chatState.threadWorkspace
    var codexSettings by remember { mutableStateOf<CodexSettings?>(null) }
    // 【2026-10-09 效果层外移】模型大表已收进 ChatViewModel(ChatState)—— 只读投影;
    // 写点三处全在 VM/意图侧:巡检合并(ModelListMerged)、静态预取替换(ModelListSet)、
    // 重试清空(ModelListReset)。读点零改动。
    val models = chatState.models
    val loadError = chatState.loadError
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
     * 【2026-10-09 效果层外移】ThreadMessage → ChatItem/身份跨度的拼装配方与
     * 「头部拼接」落地(st.items.addAll(0,…) + 封顶 + PrependApplied)已收进
     * ChatViewModel([ChatViewModel.prependHistory] / 私有 buildHistoryBatch):
     * 本页与全文搜索定位共用同一入口,滚动锚定(delta)仍由调用方持有。
     */

    // 提示词优化:输入/开关/离线态任一变化都重算按钮可见性(单一事实来源是输入,
    // 状态机只记"请求在飞";纯逻辑见 PromptOptimizeLogic.kt,JVM 单测钉行为)。
    // 【P2-10】input 从 effect key 挪进 snapshotFlow:key 在组合期求值会让整页
    // 订阅每次击键;snapshotFlow 块内读取不进组合作用域。发射语义与旧"按键重启
    // effect"一致:初始值 + 每个 distinct 变化各跑一次 sync(同步语义逐字保留)。
    LaunchedEffect(offlineSnapshot, AgentConnection.optimizePromptEnabled) {
        snapshotFlow { input }.collect { syncPromptOptState(it, offlineSnapshot) }
    }

    // ---- 斜杠命令面板(zcode/codex/pi 三源)----
    // threadAgentId 在下方 getThread 处赋值;命令列表按会话进程内缓存
    // (SlashCommandCache,见 SlashCommandsLogic.kt),进会话/重连/下拉刷新才重拉。
    // daemon 回空列表 / RPC 失败都安静降级,绝不 Toast 报错打断输入;在线时
    // hostCapabilities 已就位,内置伪命令(随 compact/review 位点亮,见
    // slashCandidatesFor)仍给三源会话兜出可用卡面。
    // 【P3 Task 8 第二层】会话归属 agent 已收进 ChatViewModel(ChatState)—— 只读
    // 投影;写点唯一(getThread 回填,ThreadHeaderLoaded),读点零改动。
    // 【PF4】换会话重置随 ViewModel 键控,理由同 threadWorkspace。
    val threadAgentId = chatState.threadAgentId
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
    // 【P3 Task 8 第二层】斜杠命令的 Compose 侧镜像已收进 ChatViewModel(ChatState)
    // —— 只读投影;写点三处都在下方效果内(SlashCommandsSettled)。进程级
    // SlashCommandCache 与拉取 RPC 原样留在这里(缓存去重通道零改动)。
    val slashCommands = chatState.slashCommands
    // 【P2-10】旧 key `slashTokenOf(input) != null` 是组合期直读:每次击键都让整页
    // 重组合只为了重算这一个布尔。改 snapshotFlow 订阅(不进组合作用域):面板
    // 出现时机(布尔翻转沿 + 初值)与刷新语义逐字保留 —— collect 每个发射(初值/
    // 翻转)各调一次 refreshSlashCommands,与旧"键变化重启 effect"同拍。
    LaunchedEffect(threadId, threadAgentId, connectionEpoch, reloadToken, session.running) {
        snapshotFlow { slashTokenOf(input) != null }.collect {
            // 【C1 RPC 入 VM】拉取/缓存/镜像落位三段整体收进 ChatViewModel.refreshSlashCommands
            // (语义逐字保留:非面板清空 → 缓存先渲染 → RPC 回包/失败降级空表)。
            chatVm.refreshSlashCommands(threadId, threadAgentId)
        }
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
    // 【C1 RPC 入 VM】四路 RPC 收进 ChatViewModel.prefetchStatic;失败项回 null,
    // 这里只在非空时覆盖(与迁出前"catch 不覆盖已有值"同一语义),加载错误的
    // "现值为空才写"裁定在 VM(LoadErrorSetIfAbsent)。
    LaunchedEffect(connectionEpoch, reloadToken) {
        val prefetched = chatVm.prefetchStatic()
        prefetched.codexSettings?.let { codexSettings = it }
        prefetched.models?.let { chatVm.send(ChatIntent.ModelListSet(it)) }
        prefetched.hostName?.let { hostName = it }
        prefetched.recentThreads?.let { recentThreads = it }
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
    LaunchedEffect(session.running) {
        if (subPrevRunning.getAndSet(session.running) && !session.running) SubAgentsState.refresh()
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
    // 【2026-10-09 效果层外移 · 任务3】10s 巡检循环收进 ChatViewModel.startModelInspection:
    // 循环体(RPC/模型表合并/原生选择回填/仓库刷新)与 scope 纪律(挂 sessionScope,
    // 离开页面随上方 DisposableEffect 的 onDispose 停表 —— 与保活心跳同款)都在 VM;
    // 本效果只做两件迁出前就属于 UI 的事:
    //  ① 键控重启:threadId/sessionAgent/connectionEpoch 任一变化即重入 start
    //     (内部先停旧循环,等价原 LaunchedEffect 的取消重建);
    //  ② 前台门注入:Activity Lifecycle 是宿主资产不进 VM(【P1-20】口径:无
    //     lifecycle-runtime-compose 依赖,用宿主 Lifecycle + 一次性 CompletableDeferred
    //     消除 START 竞态,finally 必摘观察者 —— 代码原样搬自原效果体开头)。
    suspend fun foregroundGate() {
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
    }
    LaunchedEffect(threadId, sessionAgent, connectionEpoch) {
        chatVm.startModelInspection(threadId, sessionAgent, ::foregroundGate)
    }
    val requestedModel=(AgentConnection.threadModel(threadId) ?: threadModelId).takeIf {
        it.isEmpty() || it.startsWith("$sessionAgent/")
    }.orEmpty()
    val followsNative=requestedModel.isBlank() || requestedModel==agentDefaultModelId(sessionAgent)
    val effectiveModelId=if(followsNative) session.runningModel.ifBlank { requestedModel } else requestedModel
    val effectiveModel=sessionModels.find { it.id==effectiveModelId }
        ?: if (effectiveModelId.isBlank() || effectiveModelId==agentDefaultModelId(sessionAgent))
            sessionModels.find { it.id==agentDefaultModelId(sessionAgent) } else null
    // agent 默认模型自报的窗口;没有上报就保持未知。
    val agentDefaultWindow = sessionModels.find { it.id == agentDefaultModelId(sessionAgent) }?.contextWindow ?: 0
    // 窗口解析优先级链:daemon 上报真实窗口 > 当前模型声明窗口 > agent 默认模型声明窗口。
    val reportedWindow = pickContextWindow(
        live = session.liveContextWindow,
        modelDeclared = effectiveModel?.contextWindow ?: 0,
        agentDefaultDeclared = agentDefaultWindow,
    )
    val contextWindow = reportedWindow
    val windowIsFallback = reportedWindow <= 0

    // 【生效模型名】在输入栏前统一计算,供"输入框上方全宽行"、tmux 状态条与
    // "工具行模型 chip"复用。
    // 【统一翻译层】与快速面板/完整卡同一份 modelDisplayLabel:申报视图
    // (sessionModels)displayName → 大表(models)displayName → 裸名。
    // L2 真机发现⑤后半:daemon 未带展示名时旧链直接裸 id 上屏(如
    // "zai-coding-cn/glm-5.3-flash"),现在大表补位翻出人话名;
    // 生效模型名非空不变量由翻译层兜底保证(绝不空白、不造假名)。
    // 一律用 model/list 返回的 displayName,不硬编码任何模型名。
    // 【兜底键用 effectiveModelId 而非 selectedModel】生效链会落到 daemon
    // 落库的 Thread.ModelID(pi 会话真实模型),芯片显示跟真实跑的一致。
    val modelName = if (effectiveModelId.isEmpty()) "跟随桌面"
        else modelDisplayLabel(effectiveModelId, sessionModels, models)
    // 内置档没有的取值(pi 动态档位 off/minimal/max 及未来新增)回
    // effortLabelOf 的中文标签,模型 chip 上不至于露出英文原 id。
    val effortLabel = effortLabelOf(AgentConnection.threadEffort(threadId) ?: session.runningEffort.orEmpty())

    // ---- 上下文窗口卡:断网缓存 + 自动刷新 ----
    // ① 每次收到用量更新(含从缓存回填的首帧)都写一份"最后一次已知用量"到本地:
    //    断网/自动断网后,卡片仍能照常显示这些数字,并标出是什么时候的。
    //    【R1 收口 · 落盘节流】hermes 的 usage_update 逐事件投影(密集时一秒数帧),
    //    缓存按 usagePersistCursorAccepts 节流写(3s 间隔;窗口变化/占用大跳除外,
    //    纯函数 UsagePersistThrottleTest 钉死)—— 输入栏实时环与卡面实时值不受
    //    节流影响,st.turnUsage 本身仍是逐帧真实值,这里只管磁盘写频率。
    var usagePersistCursor by remember(threadId) { mutableStateOf<UsagePersistCursor?>(null) }
    LaunchedEffect(threadId, session.turnUsage, session.totalUsage, session.liveContextWindow) {
        UsageCache.init(ctx)
        if (session.turnUsage != null || session.totalUsage != null) {
            val nowMs = System.currentTimeMillis()
            val nextTokens = usageTotalTokens(session.turnUsage)
            if (usagePersistCursorAccepts(usagePersistCursor, nowMs, nextTokens, contextWindow)) {
                UsageCache.save(
                    threadId,
                    UsageSnapshot(
                        contextWindow = contextWindow,
                        windowIsFallback = windowIsFallback,
                        turnUsage = session.turnUsage,
                        totalUsage = session.totalUsage,
                        updatedAt = nowMs,
                    ),
                )
                usagePersistCursor = UsagePersistCursor(nowMs, nextTokens, contextWindow)
            }
        }
    }
    // ② 卡片打开期间每 3s 刷新一次;卡片关闭即随 LaunchedEffect 作用域取消,不会漏取消。
    var usageRefreshTick by remember { mutableStateOf(0) }
    LaunchedEffect(showContextCard.value) {
        if (!showContextCard.value) return@LaunchedEffect
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
                // 【2026-10-09 效果层外移】RPC 段在 ChatViewModel.setApprovalPolicy
                // (sandbox 取乐观赋值后的现值,与迁出前同值 —— 乐观赋值只改 approvalPolicy);
                // getOrThrow 让失败从原 RPC 抛出点浮出,catch 文案零变化。
                codexSettings = chatVm.setApprovalPolicy(
                    sandbox = codexSettings?.sandbox ?: "",
                    approvalPolicy = id,
                ).getOrThrow()
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
                // 【2026-10-09 效果层外移】RPC 段在 ChatViewModel.approveGuardianDenied
                chatVm.approveGuardianDenied(threadId, ev.reviewId, ev.targetItemId).getOrThrow()
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
            selectedAttachment.value = attachment
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
                // 【P3 Task 8 第二层】工作区回填经 ViewModel(WorkspaceResolved)
                chatVm.send(ChatIntent.WorkspaceResolved(ws))
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
                attachments.value = attachments.value + uploaded
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
    val followBottom = remember(threadId) { mutableStateOf(true) }
    // 离底期间累计的未读新块数(胶囊上的 N;回到底部即清零)
    val unreadBlocks = remember(threadId) { mutableStateOf(0) }
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
        showHistoryPanel.value = false
        scope.launch {
            programmaticScroll.set(true)
            try {
                listState.scrollToItem(listIndex)
            } finally {
                programmaticScroll.set(false)
            }
            // 跳去历史 = 用户主动离开底部:解除跟随,新内容交给「↓ N 条新内容」胶囊
            followBottom.value = false
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
        followBottom.value = false
        listState.requestScrollToItem(entry.listIndex)
    }

    /**
     * 【R3 面板快跳·最新】回底并**恢复跟随** —— 与跳历史相反的语义:用户明确
     * 要回到对话末端看新内容,此后流式输出继续拽底。路径与「↓ N 条新内容」
     * 胶囊一致(程序滚动标记挡住距底观测,完成后重申跟随)。
     */
    fun jumpToLatestFromHistory() {
        showHistoryPanel.value = false
        scope.launch {
            programmaticScroll.set(true)
            try {
                listState.scrollToItem(st.items.size - 1)
                followBottom.value = true
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
                    // 【2026-10-09 效果层外移】头部拼接落地经 ChatViewModel.prependHistory
                    // (与「加载更早」共用同一条落地通道;delta 语义原样)
                    val grew = chatVm.prependHistory(threadId, msgs) > 0
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
            if (nowFollowing && !followBottom.value && !reduceMotion) {
                haptic.performHapticFeedback(HapticFeedbackType.TextHandleMove)
            }
            followBottom.value = nowFollowing
            if (nowFollowing) unreadBlocks.value = 0
        }
    }

    // 新内容到达:仅当"本来在跟随"(或用户其实仍在距底 100px 内)才自动回底;
    // 真离底(上滑阅读)绝不拽动,只累加未读块数交给「↓ N 条新内容」胶囊。
    // 【P0-1】旧形态把 (items.size, streamText.length/24, streaming) 写进
    // LaunchedEffect 的 key:键在**组合期**求值,等于让整页组合作用域订阅
    // streamText —— 流式期间每个 delta 触发一次全页重组,只为决定"要不要回底"。
    // 改 LaunchedEffect(Unit) + snapshotFlow:snapshotFlow 块内的读取只订阅该
    // 流(不进组合作用域),收敛出同样的重启语义(键值三元组 distinct 变化才跑),
    // 未读块数的计算口径逐字不变。
    LaunchedEffect(Unit) {
        snapshotFlow {
            Triple(st.items.size, streamBlockIndex(st.streamText.length), st.streaming)
        }.collect { (itemCount, streamBlocksNow, streamingNow) ->
            val wasFollowing = followBottom.value
            val itemBlocks = (itemCount - arriveItemCount).coerceAtLeast(0)
            val streamBlocks = (streamBlocksNow - arriveStreamBlocks).coerceAtLeast(0)
            arriveItemCount = itemCount
            arriveStreamBlocks = streamBlocksNow
            unreadBlocks.value = accumulateUnreadBlocks(wasFollowing, unreadBlocks.value, itemBlocks, streamBlocks)
            val total = itemCount + if (streamingNow) 1 else 0
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
                    followBottom.value = true
                } finally {
                    programmaticScroll.set(false)
                }
            }
        }
    }

    // 【PF4】按 threadId 记忆(getThread 用 t.title 回填且空标题不覆盖 —— 不带键时
    // 换到无标题会话会永远顶着上一会话的标题;带键换会话即回落「任务」)。
    // 【P3 Task 8 第二层】threadTitle/threadStatus 已收进 ChatViewModel(ChatState)
    // —— 只读投影;写点唯一(getThread 回填,ThreadHeaderLoaded),读点零改动。
    val threadTitle = chatState.threadTitle
    // daemon 上报的会话状态(getThread 回填;四词状态徽章的兜底数据源。
    // 运行中/恢复中由 st.running 实时覆盖,待审批由 st.pendingApproval 实时覆盖 ——
    // 徽章口径见下方 identityStatus 的装配注释)
    val threadStatus = chatState.threadStatus
    // 【桌面只读回显】会话目标顶栏文案(GoalDisplayLogic:goal_detail 电脑端现读优先,
    // goal 本地库兜底):有则显示一行,无则不占位。
    val threadGoalLine = remember(threadId) { mutableStateOf<String?>(null) } // 【PF4】换会话重置,理由同 threadWorkspace
    LaunchedEffect(threadId, connectionEpoch, reloadToken) {
        // 【2026-10-09 效果层外移 · 任务1】本效果的 RPC/getThread/历史回填/快照兜底
        // 整体收进 ChatViewModel 三阶段方法(行为零变化);滚动锚定是 UI 副作用 ——
        // VM 返回结果对象,这里只按结果位做 listState.scrollToItem(到底,原式)。
        //
        // 【P1-4 阶段 D · 离线快照】已知断网(down)进页且时间线还没内容:
        // 直接用本地只读副本渲染 + 顶部横幅 + 输入停用。checking 是巡检中间态,
        // 不算断网;有连接时行为完全不变。恢复连接会触发 connectionEpoch 变化
        // 重跑本效果,由在线分支把 offlineSnapshot 归位。
        if (st.items.isEmpty()) {
            if (chatVm.prefillOfflineSnapshot(threadId) && st.items.isNotEmpty()) {
                listState.scrollToItem(st.items.size - 1)
            }
            // 无快照:不伪造内容,继续走在线加载(失败时按原有报错分支提示)
        }
        // 阶段②getThread:失败已按原文案落 loadError 横幅(返回 null);成功回填
        // 会话真实生效模型(daemon 落库,模型芯片/弹窗选中态的数据源)与目标顶栏文案
        chatVm.loadThreadHead(threadId)?.let { head ->
            threadModelId = head.modelId
            threadGoalLine.value = head.goalLine
        }
        // 阶段③历史尾部回填(含失败快照兜底;后台轮次进行中/已有实时列表时门控在 VM 内原式)
        val tail = chatVm.loadHistoryTail(threadId, historyRoundsLoaded, connectionEpoch, reloadToken)
        if (tail.backfilled) listState.scrollToItem(st.items.size - 1)
        if (tail.snapshotFallback) listState.scrollToItem(st.items.size - 1)
    }

    /**
     * 【F-D 加载更早 / Task 7 升级】时间线顶部入口:累计轮数再加一个设置档位重拉。
     * daemon 幂等返回尾部,本次回包 = 更早新增段 + 已加载部分 —— 落地走
     * [ChatViewModel.fetchEarlierHistory]→[ChatViewModel.prependHistory]:
     * 按「已加载最旧 message id」锚点只把头部新消息
     * 拼到列表前,**不做整列表替换**(已加载条目引用不变,不整表重组;回包没有
     * 新消息时零列表变动,重复拉取无副作用)。旧 daemon 无 id 时退条数兜底,
     * 行为等价旧的"整体替换但 delta 修正"。
     *
     * 滚动锚点(保留):拉取前记 LazyListState 的 firstVisibleItem(下标+偏移),
     * 头部拼接后旧条目整体下移 delta = 新拼入条数,锚点下标 + delta 就是原来
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
                // 【2026-10-09 效果层外移】RPC+头部拼接+轮数/截断位落位在
                // ChatViewModel.fetchEarlierHistory;这里只按结果做滚动锚定:
                // 头部拼接后旧条目整体下移 delta = 新拼入条数,锚点下标 + delta 就是
                // 原来那条消息,页面不跳动;锚点在 0(「加载更早」按钮自己)说明用户
                // 已翻到顶,直接回列表顶部。回包为空(receivedCount==0)不滚动(原式)。
                val r = chatVm.fetchEarlierHistory(threadId, next)
                if (r.receivedCount > 0) {
                    val target = if (anchorIndex <= 0) 0 else (anchorIndex + r.appendedCount).coerceAtMost(st.items.size - 1)
                    listState.scrollToItem(target, if (anchorIndex <= 0) 0 else anchorOffset)
                }
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
        val sent = attachments.value
        if (!TurnSession.send(threadId, text, sent)) return false
        attachments.value = emptyList()
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
        showMore.value = false
        if (st.running) {
            Toast.makeText(ctx, "任务正在执行,等它跑完再分叉", Toast.LENGTH_SHORT).show()
            return
        }
        forkBusy = true
        scope.launch {
            try {
                // 【2026-10-09 效果层外移】RPC 在 ChatViewModel.forkThread(Result 形态);
                // getOrThrow 让失败从原 RPC 抛出点浮出,catch/finally 文案与语义零变化。
                val nt = chatVm.forkThread(threadId).getOrThrow()
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
                // RPC 在 ChatViewModel.rollbackThread(Result 形态)
                chatVm.rollbackThread(threadId, turns).getOrThrow()
                rollbackOpen.value = false
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
        showMore.value = false
        scope.launch {
            try {
                // RPC 在 ChatViewModel.compactThread(Result 形态)
                chatVm.compactThread(threadId).getOrThrow()
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
                // RPC 在 ChatViewModel.startReview(Result 形态);review/start 是
                // "发起即回",审查产出以事件流回本会话,这里不需要任何后续轮询
                chatVm.startReview(threadId, focus).getOrThrow()
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
                // 【2026-10-09 效果层外移】RPC 在 ChatViewModel.clearQueuedInputs
                // (Result 形态,回执条数原样);镜像归零与排队气泡回收照旧在本页落地
                val counts = chatVm.clearQueuedInputs(threadId).getOrThrow()
                st.queuedCount = 0
                clearDeliveryTags(st.items.toList(), queuedOnly = true)?.let { replaced ->
                    st.items.clear()
                    st.items.addAll(replaced)
                }
                Toast.makeText(ctx, queueClearedNotice(counts.steered, counts.followUp), Toast.LENGTH_SHORT).show()
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
    val messageSelectArmed = remember(threadId) { mutableStateOf<ChatItem?>(null) }

    val connSt = AgentConnection.connState.collectAsState().value
    val hasFailure = loadError != null ||
        session.recovering ||
        session.recoveryError != null ||
        session.errorMessage != null ||
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
        val pending = if (session.pendingApproval != null) 1 else 0
        val status = when {
            session.running || session.recovering -> ThreadStatus.THREAD_STATUS_RUNNING
            session.turnTerminal && session.errorMessage == null -> ThreadStatus.THREAD_STATUS_IDLE
            pending > 0 -> ThreadStatus.THREAD_STATUS_WAITING_APPROVAL
            else -> threadStatus
        }
        editorialThreadStatusView(status, pending, session.errorMessage ?: "")
    }

    // 【C1 拆分】审批弹窗状态机整体迁至 ui/chat/ChatApproval.kt;弹窗开合旗标
    // approvalDialogOpen 留在本页(时间线入口卡/身份徽章也开它),随参数直传。
    val approvalDialogOpen = remember(threadId) { mutableStateOf(false) }
    ChatApprovalFlow(
        threadId = threadId,
        threadAgentId = threadAgentId,
        threadTitle = threadTitle,
        threadWorkspace = threadWorkspace,
        session = session,
        st = st,
        hapticView = hapticView,
        approvalDialogOpen = approvalDialogOpen,
    )

    // 根布局包一层 Box:给「已优化提示词[撤销]」的 UndoBar 提供顶部悬浮层
    // (不挤内容;FilesScreen/ThreadsScreen 同款用法)。Column 本体不动。
    Box(Modifier.fillMaxSize()) {
        // 【会话中段 lambda】目标/子代理/提示条/审批入口/计划面板/时间线/「新内容」胶囊
        // —— 黑客终端骨架(原 Column)与编辑式骨架(PaScreenScaffold)一字不差共用;
        // 首部的旧顶栏与断连横幅是黑客终端专属 chrome,块内按 PaHacker 条件渲染。
        // 【C1 拆分】中段子树(时间线区)整体迁至 ui/chat/ChatTimeline.kt 的
        // ChatTimelineArea;此处只保留薄委托,两条骨架(黑客 Column / PaScreenScaffold)
        // 共用段与迁移前一字不差。重试链等闭包捕获以回调下放,语义原样。
        // 【P2-9】lambda remember 化:实例身份稳定,不再每次重组新分配一个捕获 30+
        // 变量的闭包,也让骨架侧拿到可比较的内容 lambda。key = lambda 与其捕获链
        // (loadEarlier/send/retryDelivery 等本页局部函数)触及的**全部裸值**;
        // session/threadGoalLine/codexSettings/reloadToken 等是状态委托,读取发生在
        // lambda 执行时(归执行作用域订阅),不进 key。捕获的局部函数只触键控值与
        // 状态委托,不依赖键外易变态。
        val chatMiddle: @Composable ColumnScope.() -> Unit = remember(
            threadId, threadTitle, threadAgentId, st, ctx, hapticView, drawerCtrl,
            hasFailure, threadGoalLine, loadError, offlineSnapshot, offlineSnapshotAt,
            historyTruncated, loadingEarlier, historyRoundsLoaded, hostCaps,
            uiScale, thinkScale, listState, messageMenu, messageSelectArmed, peekSub,
            approvalDialogOpen, planPanelOpen, showExport, showMore, showHistoryPanel,
            unreadBlocks, followBottom, historyEntries, historyCurrentIndex, historyDrag,
            historyDayStarts, scope,
        ) {
            // remember 计算体不可调用 composable(DisallowComposableCalls):
            // 这里只**构造**并返回 composable lambda,执行仍发生在骨架内容位。
            val content: @Composable ColumnScope.() -> Unit = {
            ChatTimelineArea(
                threadId = threadId,
                threadTitle = threadTitle,
                threadAgentId = threadAgentId,
                session = session,
                st = st,
                ctx = ctx,
                hapticView = hapticView,
                drawerCtrl = drawerCtrl,
                hasFailure = hasFailure,
                threadGoalLine = threadGoalLine.value,
                loadError = loadError,
                offlineSnapshot = offlineSnapshot,
                offlineSnapshotAt = offlineSnapshotAt,
                historyTruncated = historyTruncated,
                loadingEarlier = loadingEarlier,
                queueClearOn = queueClearVisible(threadAgentId, session.queuedCount, hostCaps),
                uiScale = uiScale,
                thinkScale = thinkScale,
                listState = listState,
                messageMenu = messageMenu,
                messageSelectArmed = messageSelectArmed,
                peekSub = peekSub,
                approvalDialogOpen = approvalDialogOpen,
                planPanelOpen = planPanelOpen,
                showExport = showExport,
                showMore = showMore,
                showHistoryPanel = showHistoryPanel,
                unreadBlocks = unreadBlocks,
                followBottom = followBottom,
                historyEntries = historyEntries,
                historyCurrentIndex = historyCurrentIndex,
                historyDrag = historyDrag,
                historyDayStarts = historyDayStarts,
                onOpenThread = onOpenThread,
                onOpenBrowser = onOpenBrowser,
                onOpenSettings = onOpenSettings,
                onApproveGuardian = { approveGuardian(it) },
                onRetryDelivery = { retryDelivery(it) },
                onOpenAttachment = { openAttachment(it) },
                onClearQueued = { clearQueuedInputs() },
                onLoadEarlier = { loadEarlier() },
                onReload = { reloadToken++ },
                onReloadAfterFailure = {
                    if (st.recoveryError != null || st.recovering) {
                        TurnSession.retryRecovery(threadId)
                    } else {
                        // 【P3 Task 8 第二层】重试清错误经 ViewModel(原 loadError = null)
                        chatVm.send(ChatIntent.LoadErrorCleared)
                        codexSettings = null
                        chatVm.send(ChatIntent.ModelListReset)
                        reloadToken++
                    }
                },
                onRetryNotice = {
                    if (st.recoveryError != null) {
                        TurnSession.retryRecovery(threadId)
                    } else {
                        chatVm.send(ChatIntent.LoadErrorCleared)
                        chatVm.send(ChatIntent.ClearSnapshotFlag)
                        reloadToken++
                    }
                },
                onReloadSnapshot = {
                    chatVm.send(ChatIntent.ClearSnapshot)
                    reloadToken++
                },
                onJumpTick = { jumpToHistoryTick(it) },
                onScrubTick = { scrubToHistoryTick(it) },
                onScrubEnd = { programmaticScroll.set(false) },
                onMenuTarget = { messageMenuTarget = it },
                scope = scope,
            )
            }
            content
        } // chatMiddle 薄委托到此结束
        // ---- 编辑式主机状态槽(05/12):只承载离线件(主机胶囊+断线横幅) ----
        // 会话身份已上移到标题行右侧(ChatIdentityBadge,2026-09-30 布局修复:
        // "Agent·工作区+状态点"跟在标题旁,不再单独占一张大卡);本槽仅在断线时
        // 出现内容 —— 未连接时状态仍显示上次更新,绝不出现"正在执行"。
        // 【C1 拆分】主机状态槽迁至 ui/chat/ChatComposer.kt(ChatHostStatusArea)
        val chatHostStatus: @Composable () -> Unit = {
            ChatHostStatusArea(
                connDown = connDown,
                hostName = hostName,
                offlineSnapshotAt = offlineSnapshotAt,
                chatBanner = chatBanner,
                onReload = { reloadToken++ },
            )
        }
        // 【C1 拆分】底部输入区整体迁至 ui/chat/ChatComposer.kt(ChatComposerArea);
        // 此处只保留薄委托,两条骨架的输入区外壳(黑客终端/编辑式)不变。
        // 【P2-10】草稿以 provider 形态传入:参数在 ChatComposerArea 内部读取,
        // 击键只重组输入区子树,不再把整页订阅到 st.draft。
        val chatComposerBody: @Composable ColumnScope.() -> Unit = {
            ChatComposerArea(
                threadId = threadId,
                session = session,
                ctx = ctx,
                hapticView = hapticView,
                input = { input },
                setInput = ::setInput,
                attachments = attachments,
                threadAgentId = threadAgentId,
                slashCommands = slashCommands,
                offlineSnapshot = offlineSnapshot,
                hostCaps = hostCaps,
                slashGoalSheet = slashGoalSheet,
                onCompact = ::doCompact,
                onStartReview = ::doStartReview,
                onOpenAttachment = ::openAttachment,
                voiceLandedTick = voiceLandedTick,
                voiceListening = voiceListening,
                voiceState = voiceState,
                voiceReplaceMode = voiceReplaceMode,
                voiceElapsedSec = voiceElapsedSec,
                micPermission = micPermission,
                voiceController = voiceController,
                reportVoiceUnavailable = ::reportVoiceUnavailable,
                expandedInputOpen = expandedInputOpen,
                modelName = modelName,
                effortLabel = effortLabel,
                showQuickModels = showQuickModels,
                modelFlashTick = modelFlashTick,
                sessionAgent = sessionAgent,
                codexSettings = codexSettings,
                onSetApproval = ::setApproval,
                windowIsFallback = windowIsFallback,
                contextWindow = contextWindow,
                showContextCard = showContextCard,
                showGitPanel = showGitPanel,
                pickFiles = pickFiles,
                promptOptState = promptOptState,
                onOptimizeInput = ::optimizeCurrentInput,
                onSend = ::send,
            )
        } // chatComposerBody 薄委托到此结束

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
                        approvalWaiting = session.pendingApproval != null &&
                            !askIsAskAction(session.pendingApproval!!.action.name),
                        onOpenApproval = { approvalDialogOpen.value = true },
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
                        onClick = { showMore.value = true },
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
            visible = showHistoryPanel.value,
            entries = historyEntries,
            currentIndex = historyCurrentIndex,
            onJump = { listIndex -> jumpToHistoryEntry(listIndex) },
            onDismiss = { showHistoryPanel.value = false },
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
                        "select" -> messageSelectArmed.value = menuTarget
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

    // 【C1 拆分】附件预览大图迁至 ui/chat/ChatSheets.kt
    if (selectedAttachment.value != null) {
        ChatAttachmentViewer(selectedAttachment = selectedAttachment, ctx = ctx)
    }

    // 【C1 拆分】「更多」面板迁至 ui/chat/ChatSheets.kt
    if (showMore.value) {
        ChatMoreSheet(
            showMore = showMore,
            showExport = showExport,
            rollbackOpen = rollbackOpen,
            drawerCtrl = drawerCtrl,
            threadId = threadId,
            threadAgentId = threadAgentId,
            hostCaps = hostCaps,
            running = session.running,
            compactBusy = compactBusy,
            ctx = ctx,
            hapticView = hapticView,
            onCompact = ::doCompact,
            onFork = ::doFork,
        )
    }

    // 【C1 拆分】"/目标"底栏迁至 ui/chat/ChatSheets.kt
    if (slashGoalSheet.value) {
        ChatGoalSheet(
            slashGoalSheet = slashGoalSheet,
            threadGoalLine = threadGoalLine,
            threadId = threadId,
            ctx = ctx,
            scope = scope,
            // 【2026-10-09 效果层外移】SetThreadGoal RPC 收进 ChatViewModel.setThreadGoal
            // (Result 形态);面板内 Toast/关闭/目标文案刷新语义零变化
            onSetGoal = { chatVm.setThreadGoal(threadId, it) },
        )
    }

    // 【C1 拆分】回滚面板迁至 ui/chat/ChatSheets.kt
    if (rollbackOpen.value) {
        ChatRollbackSheet(
            rollbackOpen = rollbackOpen,
            userTurnCount = session.userTurnCount,
            rollbackBusy = rollbackBusy,
            onRollback = ::doRollback,
        )
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




    // 导出对话(可多选)
    if (showExport.value) {
        ExportChatSheet(
            title = threadTitle,
            // C3:折叠标记行不是消息,不进导出选择
            items = st.items.toList().filter { it.role != "items_trimmed" },
            threadId = threadId,
            // 会话归属 agent(GetThread 里带回来):HTML 存档按 export_html:<agentId>
            // 能力位门控,位来自共享快照 hostCaps
            agentId = threadAgentId,
            capabilities = hostCaps,
            onDismiss = { showExport.value = false },
            onExport = { text, count ->
                showExport.value = false
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
    val peekMeta = peekSub.value
    if (peekMeta != null) {
        val peekRunIds = remember(peekMeta.threadId, SubAgentsState.lastRefreshedAt) {
            TurnSession.runningThreadIds()
        }
        SubAgentPeekSheet(
            meta = peekMeta,
            state = SubAgentsLogic.runStateOf(peekMeta, peekRunIds),
            onOpenFull = {
                peekSub.value = null
                onOpenThread(peekMeta.threadId)
            },
            onDismiss = { peekSub.value = null },
        )
    }

    // 长文本大窗编辑(需求:输入过长时可增大输入框窗口)
    // 【P2-10】value/canSend 以 provider 形态传入:大窗里的击键只重组大窗子树。
    if (expandedInputOpen.value) {
        ExpandedInputSheet(
            value = { input },
            onValueChange = { setInput(it) },
            onDismiss = { expandedInputOpen.value = false },
            onSend = {
                val text = input
                expandedInputOpen.value = false
                if (send(text)) setInput("")
            },
            canSend = { !offlineSnapshot && input.isNotBlank() && !session.recovering },
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
    if (showGitPanel.value) {
        ThreadGitPanel(
            repoPath = threadWorkspace,
            onClose = { showGitPanel.value = false },
        )
    }

    if (showModelCard.value) {
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
            onDismiss = { showModelCard.value = false },
            // 卡内真实选中 → 关卡后模型行给一次行内闪光(与快速面板同一反馈口径)
            onPicked = { modelFlashTick++ },
        )
    }

    // 【C1 拆分】快速切换面板迁至 ui/chat/ChatQuickPanels.kt
    if (showQuickModels.value) {
        ChatQuickModelsPanel(
            showQuickModels = showQuickModels,
            showModelCard = showModelCard,
            modelSelectionState = modelSelectionState,
            modelSelection = modelSelection,
            sessionAgent = sessionAgent,
            sessionModels = sessionModels,
            effectiveModelId = effectiveModelId,
            st = st,
            threadId = threadId,
            onModelPicked = { modelFlashTick++ },
        )
    }

    // 【C1 拆分】上下文明细卡迁至 ui/chat/ChatQuickPanels.kt
    if (showContextCard.value) {
        ChatContextUsagePanel(
            showContextCard = showContextCard,
            ctx = ctx,
            usageRefreshTick = usageRefreshTick,
            connOk = connOk,
            connDown = connDown,
            threadId = threadId,
            sessionAgent = sessionAgent,
            windowIsFallback = windowIsFallback,
            contextWindow = contextWindow,
            session = session,
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
 * 【G4 接线】placeholder=输入框提示文案、prefill=初始文本(pi 的 input/editor 反问表单,
 * daemon 非空才进卡,piapp/client.go / pi_events.go buildAskUserCall);键缺失=空串=
 * 与老卡行为逐字节一致。
 */
internal data class AskQuestion(
    val id: String,
    val text: String,
    val options: List<AskOption>,
    val multiSelect: Boolean = false,
    val isSecret: Boolean = false,
    val isOther: Boolean = false,
    val header: String = "", // codex 卡的问题分组小标题,存在才渲染
    val placeholder: String = "", // pi input 卡的输入提示,存在才替换默认提示文案
    val prefill: String = "", // pi editor 卡的预填文本,非空才进初始输入
)

/** 问题 id 缺失时给个稳定的兜底 key,否则答案会挂不上(与旧行为一致)。 */
internal fun askQid(q: AskQuestion, idx: Int): String = q.id.ifBlank { "q$idx" }

/**
 * 解析审批卡 json_args 里的 questions(纯函数,JVM 可测)。
 * 【能透传就透传】选项取 label 主文案,description/preview 原样保留(渲染时如实展示、不改写);
 * 问题级 multiSelect/isSecret/isOther 存在才生效;placeholder/prefill(pi input/editor 的
 * 输入提示/预填)非空才携带。不认识的字段/选项形状一律忽略不炸——
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
                    placeholder = q.optString("placeholder"),
                    prefill = q.optString("prefill"),
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
 * 每题自由输入的初始 map(纯函数,JVM 可测):pi editor 的 prefill(G4)非空才作
 * 初始文本(key 走 askQid 兜底,与提交打包同源);无 prefill 的老卡返回空 map,
 * 与旧版 `emptyMap()` 初始态逐字段一致。
 */
internal fun askInitialTexts(questions: List<AskQuestion>): Map<String, String> =
    questions.mapIndexedNotNull { idx, q ->
        q.prefill.takeIf { it.isNotBlank() }?.let { askQid(q, idx) to it }
    }.toMap()

/**
 * 自由输入框的提示文案(纯函数,JVM 可测):pi input 的 placeholder(G4)非空优先;
 * 键缺失(空串)回落旧文案 —— 无选项=自由输入兜底、isOther=「或填写其他」、
 * 其余=「或直接输入你的回答」,与老卡逐字节一致。
 */
internal fun askInputHint(q: AskQuestion): String =
    q.placeholder.ifBlank {
        when {
            q.options.isEmpty() -> "输入你的回答…" // 无选项=自由输入兜底(含未知形状降级)
            q.isOther -> "或填写其他…"
            else -> "或直接输入你的回答…"
        }
    }

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
internal fun ApprovalSheet(
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
    // 每题自由输入;pi editor 的 prefill(G4)经 askInitialTexts 非空才作初始文本,
    // 键缺失=空 map=老卡初始态。
    var texts by remember(action.requestId) { mutableStateOf(askInitialTexts(questions)) }
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
            Text(title, Modifier.weight(1f), fontSize = PaEditorial.Type.Heading.fontSize, color = PaEditorial.Ink)
            // 反问类没有"可展开的原文",收起/展开这一对控件对它没有意义。
            if (!isAsk) {
                Text(
                    if (expanded) "收起" else "展开",
                    fontSize = PaEditorial.Type.Caption.fontSize,
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
            fontSize = PaEditorial.Type.BodySmall.fontSize,
            color = PaEditorial.Muted,
        )
        error?.let {
            Text(
                it,
                Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 24.dp, vertical = 4.dp),
                fontSize = PaEditorial.Type.Caption.fontSize,
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
                        fontSize = PaEditorial.Type.BodySmall.fontSize, color = PaEditorial.Ink, lineHeight = 19.sp,
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
                            fontSize = PaEditorial.Type.Caption.fontSize,
                            color = PaEditorial.Subtle,
                        )
                    }
                    Text(
                        q.text.ifBlank { "请回答" },
                        Modifier.padding(top = if (q.header.isNotBlank()) 0.dp else groupTop, bottom = 8.dp),
                        fontSize = PaEditorial.Type.Body.fontSize, color = PaEditorial.Ink, lineHeight = 19.sp,
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
                                            RoundedCornerShape(PaEditorial.Radius.IconButton),
                                        )
                                        .border(
                                            0.5.dp,
                                            if (active) PaEditorial.Primary else PaEditorial.Hairline,
                                            RoundedCornerShape(PaEditorial.Radius.IconButton),
                                        )
                                        .padding(horizontal = 14.dp, vertical = 11.dp),
                                ) {
                                    Row(verticalAlignment = Alignment.Top) {
                                        if (q.multiSelect) {
                                            // 勾选态:选中=品牌蓝实底+白勾,未选中=空心方框
                                            Box(
                                                Modifier
                                                    .size(16.dp)
                                                    .clip(RoundedCornerShape(PaEditorial.Radius.Terminal))
                                                    .background(if (active) PaEditorial.Primary else Color.Transparent)
                                                    .border(
                                                        0.5.dp,
                                                        if (active) PaEditorial.Primary else PaEditorial.Subtle,
                                                        RoundedCornerShape(PaEditorial.Radius.Terminal),
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
                                                fontSize = PaEditorial.Type.BodySmall.fontSize,
                                                color = if (active) PaEditorial.Primary else PaEditorial.Muted,
                                            )
                                            // 副文案:daemon 透传的 description,存在才显示
                                            if (opt.description.isNotBlank()) {
                                                Text(
                                                    opt.description,
                                                    Modifier.padding(top = 3.dp),
                                                    fontSize = PaEditorial.Type.Caption.fontSize,
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
                                                        .clip(RoundedCornerShape(PaEditorial.Radius.Chip))
                                                        .background(PaEditorial.Canvas)
                                                        .border(0.5.dp, PaEditorial.Hairline, RoundedCornerShape(PaEditorial.Radius.Chip))
                                                        .padding(horizontal = 10.dp, vertical = 8.dp),
                                                    fontSize = PaEditorial.Type.Caption.fontSize,
                                                    fontFamily = PaEditorial.MonoFont,
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
                            .clip(RoundedCornerShape(PaEditorial.Radius.IconButton))
                            .background(PaEditorial.Surface)
                            .border(0.5.dp, PaEditorial.Hairline, RoundedCornerShape(PaEditorial.Radius.IconButton)),
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
                            textStyle = TextStyle(fontSize = PaEditorial.Type.BodySmall.fontSize, color = PaEditorial.Ink),
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
                                        // pi input 的 placeholder(G4)经 askInputHint 非空才替换,键缺失回落旧文案
                                        Text(
                                            askInputHint(q),
                                            fontSize = PaEditorial.Type.BodySmall.fontSize, color = PaEditorial.Subtle,
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
                    .clip(RoundedCornerShape(PaEditorial.Radius.IconButton))
                    .background(PaEditorial.Surface)
                    .border(0.5.dp, PaEditorial.Hairline, RoundedCornerShape(PaEditorial.Radius.IconButton))
                    .heightIn(max = 240.dp)
                    .verticalScroll(cmdScroll),
            ) {
                SelectionContainer {
                    Text(
                        body,
                        Modifier.padding(14.dp),
                        fontSize = PaEditorial.Type.Caption.fontSize,
                        fontFamily = PaEditorial.MonoFont,
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
                    .background(PaEditorial.Surface, RoundedCornerShape(PaEditorial.Radius.Card))
                    .padding(vertical = 14.dp),
                contentAlignment = Alignment.Center,
            ) {
                Text(if (isAsk) "取消" else "拒绝", fontSize = PaEditorial.Type.Heading.fontSize, color = PaEditorial.Denied)
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
                        RoundedCornerShape(PaEditorial.Radius.Card),
                    )
                    .padding(vertical = 14.dp),
                contentAlignment = Alignment.Center,
            ) {
                Text(
                    if (isAsk) "提交" else "允许",
                    fontSize = PaEditorial.Type.Heading.fontSize,
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
    // 组合期不得写状态(Compose 快照红线:组合中写 State 的时序不受快照系统保护,
    // 审查 2026-10-04 :6802 项):首帧以 target=false 组合,LaunchedEffect 在首帧后
    // 推进 targetState → AnimatedVisibility 的 visible 经历 false→true 翻转,声明的
    // slideInHorizontally 入场照播(旧写法组合期即置 true,首帧 visible 恒 true,
    // 这条入场动画此前从未真正播过)。
    LaunchedEffect(Unit) { visibleState.targetState = true }

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
                    .background(
                        PaEditorial.Raised,
                        // 方向性圆角逐处归档(2026-10-09):左滑入侧板属弹层族 → Sheet 档(23dp)
                        RoundedCornerShape(topEnd = PaEditorial.Radius.Sheet, bottomEnd = PaEditorial.Radius.Sheet),
                    )
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
                        Text("Git", fontSize = PaEditorial.Type.Heading.fontSize, color = PaEditorial.Ink, fontWeight = FontWeight.SemiBold)
                        Text(
                            status?.branch?.takeIf { it.isNotEmpty() }?.let { "分支 $it · $repo" } ?: repo,
                            fontSize = PaEditorial.Type.Micro.fontSize, color = PaEditorial.Subtle, maxLines = 1,
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
                            Text("当前会话未绑定工作区", fontSize = PaEditorial.Type.Body.fontSize, color = PaEditorial.Ink)
                            Text(
                                "创建会话时选择工作文件夹后，Git 会跟随该会话",
                                fontSize = PaEditorial.Type.Caption.fontSize,
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
                            Text("该项目还不是 Git 仓库", fontSize = PaEditorial.Type.Body.fontSize, color = PaEditorial.Ink)
                            Text(
                                "在电脑上 git init 后,这里可以看状态、diff 和提交",
                                fontSize = PaEditorial.Type.Caption.fontSize,
                                color = PaEditorial.Subtle,
                                modifier = Modifier.padding(top = 4.dp),
                            )
                        }
                    }
                    error != null -> Box(
                        Modifier.weight(1f).fillMaxWidth(),
                        contentAlignment = Alignment.Center,
                    ) {
                        Text(error!!, color = PaEditorial.Denied, fontSize = PaEditorial.Type.Caption.fontSize)
                    }
                    status != null -> {
                        val st = status!!
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            Text("${st.changesList.size} 个变更", fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Muted)
                            Spacer(Modifier.weight(1f))
                            if (busy) {
                                CircularProgressIndicator(Modifier.size(14.dp), strokeWidth = 2.dp, color = PaEditorial.Primary)
                                Spacer(Modifier.width(8.dp))
                            }
                            IconActionButton(AppR.drawable.ic_lucide_refresh_cw, "刷新", { refresh() }, bg = PaEditorial.Surface, size = 34.dp, iconSize = 15.dp)
                        }
                        Spacer(Modifier.height(8.dp))
                        if (st.changesList.isEmpty()) {
                            Text("工作区干净", Modifier.padding(vertical = 20.dp), fontSize = PaEditorial.Type.BodySmall.fontSize, color = PaEditorial.Muted)
                        }
                        LazyColumn(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(6.dp)) {
                            items(st.changesList, key = { it.path }) { c ->
                                Column(
                                    Modifier
                                        .fillMaxWidth()
                                        .background(PaEditorial.Surface, RoundedCornerShape(PaEditorial.Radius.IconButton))
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
                                        fontSize = PaEditorial.Type.BodySmall.fontSize,
                                        fontFamily = PaEditorial.MonoFont,
                                        color = PaEditorial.Ink,
                                        maxLines = 1,
                                        overflow = TextOverflow.Ellipsis,
                                    )
                                    if (dirPart.isNotEmpty()) {
                                        Text(
                                            dirPart,
                                            fontSize = PaEditorial.Type.Micro.fontSize,
                                            fontFamily = PaEditorial.MonoFont,
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
                                        fontSize = PaEditorial.Type.State.fontSize, color = PaEditorial.Subtle, maxLines = 1,
                                    )
                                }
                            }
                        }
                        if (st.changesList.isNotEmpty()) {
                            Box(
                                Modifier
                                    .fillMaxWidth()
                                    .pressable(enabled = !busy) { showCommit = true }
                                    .background(PaEditorial.Primary, RoundedCornerShape(PaEditorial.Radius.IconButton))
                                    .padding(vertical = 12.dp),
                                contentAlignment = Alignment.Center,
                            ) {
                                Text(
                                    "提交全部变更",
                                    fontSize = PaEditorial.Type.Body.fontSize,
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
                        Text("读取中…", fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Subtle)
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
            title = { Text("变更 (git diff)", fontSize = PaEditorial.Type.Body.fontSize) },
            text = {
                Text(
                    d.take(30000).ifEmpty { "(无未暂存变更)" },
                    fontSize = PaEditorial.Type.Micro.fontSize, fontFamily = PaEditorial.MonoFont,
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
internal fun InAppMarkdown(
    text: String,
    modifier: Modifier = Modifier,
    fontSize: Float = 15f,
    onOpenUrl: (String) -> Unit,
    /** 流式路径传 true:新长出来的块播 fade-in(D4-1,见 MarkdownText)。 */
    streaming: Boolean = false,
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
        MarkdownText(text, modifier, fontSize, streaming)
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
 * (`value` / `onValueChange` 直连 `input`;P2-10 起 value/canSend 以 provider
 * 形态传入,击键只重组本面板子树),不存在两份副本需要回写,
 * 点"完成"只是收起面板,文字本来就已经在输入框里了。
 */
@OptIn(androidx.compose.material3.ExperimentalMaterial3Api::class)
@Composable
private fun ExpandedInputSheet(
    value: () -> String,
    onValueChange: (String) -> Unit,
    onDismiss: () -> Unit,
    onSend: () -> Unit,
    canSend: () -> Boolean,
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
                Text("编辑消息", Modifier.weight(1f), fontSize = PaEditorial.Type.Heading.fontSize, color = PaEditorial.Ink)
                Text("${value().length} 字", fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Subtle)
                Spacer(Modifier.width(10.dp))
                // 完成 = 收起面板,内容原样留在底部输入框
                Box(
                    Modifier
                        .clip(RoundedCornerShape(PaEditorial.Radius.Chip))
                        .pressable { onDismiss() }
                        .background(PaEditorial.Surface)
                        .padding(horizontal = 12.dp, vertical = 7.dp),
                ) {
                    Text("完成", fontSize = PaEditorial.Type.BodySmall.fontSize, color = PaEditorial.Ink)
                }
                Spacer(Modifier.width(8.dp))
                // 发送键触感分级:大窗里发送与底栏同档(confirm 级,见输入区发送键注释)
                val sendHapticView = LocalView.current
                val sendable = canSend()
                Box(
                    Modifier
                        .clip(RoundedCornerShape(PaEditorial.Radius.Chip))
                        .pressable(scaleDown = 0.92f, enabled = sendable) {
                            PaHaptics.confirm(sendHapticView)
                            onSend()
                        }
                        .background(if (sendable) PaEditorial.Primary else PaEditorial.Hover)
                        .padding(horizontal = 12.dp, vertical = 7.dp),
                ) {
                    Text(
                        "发送", fontSize = PaEditorial.Type.BodySmall.fontSize,
                        color = if (sendable) PaEditorial.OnPrimary else PaEditorial.Subtle,
                    )
                }
            }
            Spacer(Modifier.height(10.dp))
            val scroll = rememberScrollState()
            Box(
                Modifier
                    .fillMaxWidth()
                    .weight(1f)
                    .clip(RoundedCornerShape(PaEditorial.Radius.Card))
                    .background(PaEditorial.Surface)
                    .border(0.5.dp, PaEditorial.Hairline, RoundedCornerShape(PaEditorial.Radius.Card)),
            ) {
                BasicTextField(
                    value = value(),
                    onValueChange = onValueChange,
                    modifier = Modifier.fillMaxSize().verticalScroll(scroll).padding(14.dp),
                    textStyle = TextStyle(fontSize = PaEditorial.Type.Heading.fontSize, color = PaEditorial.Ink, lineHeight = 22.sp),
                    cursorBrush = SolidColor(PaEditorial.Primary),
                    decorationBox = { inner ->
                        Box {
                            if (value().isEmpty()) {
                                Text(
                                    "随心输入,发给电脑上的 agent…",
                                    fontSize = PaEditorial.Type.Heading.fontSize, color = PaEditorial.Subtle,
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
                        fontSize = PaEditorial.Type.Heading.fontSize,
                        color = PaEditorial.Ink,
                        fontWeight = FontWeight.SemiBold,
                    )
                    Spacer(Modifier.height(2.dp))
                    Text(
                        title.ifBlank { "未命名会话" } + " · 已选 " + selected.size + " / " + source.size + " 条" +
                            if (full == null) "(正在读取全部历史…)" else "",
                        fontSize = PaEditorial.Type.Caption.fontSize,
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
                                RoundedCornerShape(PaEditorial.Radius.IconButton),
                            )
                            .border(
                                if (on) 1.dp else 0.dp,
                                if (on) PaEditorial.Primary else androidx.compose.ui.graphics.Color.Transparent,
                                RoundedCornerShape(PaEditorial.Radius.IconButton),
                            )
                            .padding(horizontal = 11.dp, vertical = 9.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        // 选中框(自绘,避免再引入一个图标资源)
                        Box(
                            Modifier
                                .size(16.dp)
                                .background(if (on) PaEditorial.Primary else Color.Transparent, RoundedCornerShape(PaEditorial.Radius.Terminal))
                                .border(1.dp, if (on) PaEditorial.Primary else PaEditorial.Subtle, RoundedCornerShape(PaEditorial.Radius.Terminal)),
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
                                fontSize = PaEditorial.Type.Micro.fontSize,
                                color = if (chatItem.role == "user") PaEditorial.AccentText else PaEditorial.Subtle,
                            )
                            Text(
                                chatItem.text.replace("\n", " ").take(80),
                                fontSize = PaEditorial.Type.BodySmall.fontSize,
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
                        RoundedCornerShape(PaEditorial.Radius.IconButton),
                    )
                    .padding(vertical = 13.dp),
                contentAlignment = Alignment.Center,
            ) {
                Text(
                    if (selected.isEmpty()) "请选择要导出的消息" else "导出 " + selected.size + " 条消息(Markdown)",
                    fontSize = PaEditorial.Type.Body.fontSize,
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
                    .background(if (htmlGateBlocked) PaEditorial.Hover else PaEditorial.Surface, RoundedCornerShape(PaEditorial.Radius.IconButton))
                    .padding(vertical = 13.dp),
                contentAlignment = Alignment.Center,
            ) {
                Text(
                    if (exportingHtml) "正在电脑端渲染 HTML…" else "导出 HTML 存档(电脑端)",
                    fontSize = PaEditorial.Type.Body.fontSize,
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
        Text(label, fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Muted)
    }
}

/** 复制一条消息的文本到系统剪贴板(长按消息触发)。 */
internal fun copyMessage(ctx: Context, text: String) {
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