package dev.pocketagent.app.ui

import android.graphics.BitmapFactory
import android.content.Context
import android.content.Intent
import android.net.Uri
import android.provider.OpenableColumns
import android.widget.Toast
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.animation.expandVertically
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.shrinkVertically
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
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties
import com.google.protobuf.ByteString
import androidx.compose.runtime.collectAsState
import dev.pocketagent.app.AgentConnection
import dev.pocketagent.app.R as AppR
import dev.pocketagent.app.ui.anim.IconActionButton
import dev.pocketagent.app.ui.anim.pressable
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
import pocketagent.v1.WriteFileRequest

/** 一条聊天记录(用户/助手/工具过程)。 */
data class ChatItem(
    val role: String, // user / assistant / tool / codex_event
    val text: String = "",
    val toolName: String = "",
    val toolStatus: String = "", // running / success / failure
    val attachments: List<Attachment> = emptyList(),
    val codexEvent: CodexEvent? = null,
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
 * 顶栏 ☰ 打开侧边栏(会话列表/搜索/新建);输入栏对标图五:
 * 第一行输入,第二行 [+ 附件][审批策略][模型·强度][发送]。
 */
@Composable
fun ChatScreen(
    threadId: String,
    onBack: () -> Unit = {},
    onOpenThread: (String) -> Unit = {},
    onOpenSettings: () -> Unit = {},
) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    // 轮次状态在 TurnSession 单例:离开页面流不中断,回来接着看
    val st = remember(threadId) { TurnSession.state(threadId) }
    var input by remember { mutableStateOf("") }
    var showModelCard by remember { mutableStateOf(false) }
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

    Column(Modifier.fillMaxSize().background(Pa.Bg).statusBarsPadding()) {
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

        // ---- 💭 思考条(字号跟随"思考显示大小"设置)----
        AnimatedVisibility(
            visible = st.running && st.thinkingText.isNotEmpty(),
            enter = expandVertically() + fadeIn(),
            exit = shrinkVertically() + fadeOut(),
        ) {
            Text(
                "💭 " + st.thinkingText.takeLast(180),
                Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 16.dp, vertical = 4.dp)
                    .background(Pa.Card, RoundedCornerShape(14.dp))
                    .padding(10.dp),
                fontSize = (11 * thinkScale).sp,
                fontFamily = FontFamily.Monospace,
                color = Pa.TextSecondary,
                maxLines = 3,
            )
        }

        // ---- 消息列表 ----
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
                    "tool" -> {
                        val toolStatus = item.toolStatus.ifBlank {
                            if (item.text.isBlank()) "running" else "success"
                        }
                        Row(
                            Modifier
                                .fillMaxWidth()
                                .background(Pa.Card, RoundedCornerShape(12.dp))
                                .padding(horizontal = 12.dp, vertical = 8.dp),
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            Box(
                                Modifier
                                    .size(8.dp)
                                    .background(
                                        when (toolStatus) {
                                            "failure" -> Pa.Red
                                            "success" -> Pa.Green
                                            else -> Pa.Accent
                                        },
                                        CircleShape,
                                    ),
                            )
                            Spacer(Modifier.width(8.dp))
                            Text(item.toolName, fontSize = (13 * uiScale).sp, color = Pa.TextPrimary)
                            if (item.text.isNotEmpty() && AgentConnection.toolDetailsExpanded) {
                                Spacer(Modifier.width(8.dp))
                                Text(
                                    item.text.replace("\n", " ").take(60),
                                    Modifier.weight(1f),
                                    fontSize = (12 * uiScale).sp,
                                    color = Pa.TextSecondary,
                                    maxLines = 1,
                                )
                            }
                        }
                    }
                    // 后台 Codex/MCP 事件只影响右上角状态点，不再把诊断卡片
                    // 连续插入会话正文。
                    "codex_event" -> Unit
                    "usage" -> Unit
                    else -> MarkdownText(
                        item.text,
                        Modifier.fillMaxWidth(),
                        fontSize = 15f * uiScale,
                    )
                }
            }
            if (st.streaming) {
                item(key = "streaming-bubble") {
                    val blink by rememberInfiniteTransition(label = "caret").animateFloat(
                        initialValue = 1f,
                        targetValue = 0.15f,
                        animationSpec = infiniteRepeatable(tween(500), RepeatMode.Reverse),
                        label = "caretAlpha",
                    )
                    Column(Modifier.fillMaxWidth()) {
                        MarkdownText(
                            st.streamText.ifEmpty { "…" },
                            fontSize = 15f * uiScale,
                        )
                        Text(
                            "▍",
                            color = Pa.Accent.copy(alpha = blink),
                            fontSize = (15 * uiScale).sp,
                        )
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
                    Spacer(Modifier.weight(1f))
                    // [模型 · 强度] 直显当前模型(点击弹卡面切换)
                    Row(
                        Modifier
                            .pressable { showModelCard = true }
                            .padding(horizontal = 6.dp, vertical = 6.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        val selectedModelInfo = models.find { it.id == AgentConnection.selectedModel }
                        val contextLabel = selectedModelInfo?.contextWindow?.takeIf { it > 0 }?.let { "上下文 ${it / 1000}k" }
                        contextLabel?.let {
                            Text(it, fontSize = 10.5.sp, color = Pa.TextTertiary, maxLines = 1)
                            Spacer(Modifier.width(5.dp))
                        }
                        val modelName = selectedModelInfo?.displayName
                            ?: if (AgentConnection.selectedModel.isEmpty()) "跟随桌面" else AgentConnection.selectedModel.substringAfter('/')
                        val effortLabel = EFFORTS.find { it.id == AgentConnection.selectedEffort }?.label
                        Text(
                            modelName + if (effortLabel != null && effortLabel != "默认") " · $effortLabel" else "",
                            fontSize = 11.5.sp,
                            color = Pa.TextSecondary,
                            maxLines = 1,
                        )
                    }
                    Spacer(Modifier.width(4.dp))
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

    // ---- 侧边栏(图七):会话列表 + 搜索 + 新建 ----
    if (showDrawer) {
        ChatDrawer(
            currentThreadId = threadId,
            onClose = { showDrawer = false },
            onOpenThread = {
                showDrawer = false
                if (it != threadId) onOpenThread(it)
            },
            onOpenSettings = {
                showDrawer = false
                onOpenSettings()
            },
            onNewThread = {
                showDrawer = false
                showNewThread = true
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
        ModelEffortCard(onDismiss = { showModelCard = false })
    }
}

/**
 * 模型/思考强度卡面弹窗 —— "弹窗的卡面"而非长抽屉:
 * 居中卡片 + 限高可滚模型列表 + 思考强度胶囊 + 性能提示语。
 */
@Composable
fun ModelEffortCard(onDismiss: () -> Unit) {
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
            Column(
                Modifier
                    .fillMaxWidth()
                    .height(300.dp)
                    .verticalScroll(rememberScrollState()),
            ) {
                if (models.value.isEmpty()) {
                    Text("加载中…", fontSize = 13.sp, color = Pa.TextTertiary, modifier = Modifier.padding(8.dp))
                }
                models.value.forEach { m ->
                    val selected = m.id == AgentConnection.selectedModel ||
                        (AgentConnection.selectedModel.isEmpty() && m.id == "codex/default")
                    Row(
                        Modifier
                            .fillMaxWidth()
                            .pressable {
                                AgentConnection.selectedModel = m.id
                                AgentConnection.persistModel()
                            }
                            .padding(horizontal = 4.dp, vertical = 11.dp),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Text(
                            m.displayName,
                            Modifier.weight(1f),
                            fontSize = 15.sp,
                            color = if (selected) Pa.Accent else Pa.TextPrimary,
                            maxLines = 1,
                        )
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

/**
 * 聊天页侧边栏(对标图七):左上角 ☰ 弹出 ——
 * 顶部品牌行、新建对话、搜索框、历史会话列表、底部设置入口。
 */
@Composable
fun ChatDrawer(
    currentThreadId: String,
    onClose: () -> Unit,
    onOpenThread: (String) -> Unit,
    onOpenSettings: () -> Unit,
    onNewThread: () -> Unit,
) {
    var threads by remember { mutableStateOf(listOf<PThread>()) }
    var loading by remember { mutableStateOf(true) }
    var query by remember { mutableStateOf("") }

    LaunchedEffect(Unit) {
        try {
            withContext(Dispatchers.IO) {
                threads = AgentConnection.threadService()
                    .listThreads(ListThreadsRequest.newBuilder().build())
                    .threadsList
            }
        } catch (_: Exception) {}
        loading = false
    }

    // 入场:从左侧滑入(用 MutableTransitionState 使首帧即播动画)
    val drawerVisible = remember { androidx.compose.animation.core.MutableTransitionState(false) }
    drawerVisible.targetState = true

    Box(
        Modifier
            .fillMaxSize()
            .background(Color.Black.copy(alpha = 0.55f))
            .pressable { onClose() },
    ) {
        AnimatedVisibility(
            visible = drawerVisible.targetState,
            enter = slideInHorizontally(tween(260)) { it / 3 },
            exit = slideOutHorizontally(tween(200)) { it / 3 },
            modifier = Modifier
                .align(Alignment.CenterStart)
                .fillMaxHeight(),
        ) {
            Column(
                Modifier
                    .fillMaxWidth(0.82f)
                    .widthIn(max = 292.dp)
                    .fillMaxHeight()
                    .background(Pa.CardHigh, RoundedCornerShape(topEnd = 24.dp, bottomEnd = 24.dp))
                    .statusBarsPadding()
                    .navigationBarsPadding()
                    .padding(horizontal = 16.dp),
            ) {
                Spacer(Modifier.height(14.dp))
                // 品牌行
                Row(verticalAlignment = Alignment.CenterVertically) {
                    androidx.compose.foundation.Image(
                        bitmap = agentBitmap(AppR.mipmap.ic_launcher),
                        contentDescription = null,
                        modifier = Modifier.size(42.dp),
                        contentScale = ContentScale.Crop,
                    )
                    Spacer(Modifier.width(16.dp))
                    Column(Modifier.weight(1f)) {
                        Text("PocketAgent", fontSize = 16.sp, color = Pa.TextPrimary, fontWeight = FontWeight.SemiBold)
                        Text("你电脑上的 agent", fontSize = 10.5.sp, color = Pa.TextTertiary)
                    }
                    IconActionButton(
                        AppR.drawable.ic_lucide_x, "关闭", onClose,
                        bg = Color.Transparent, size = 36.dp, iconSize = 17.dp,
                    )
                }
                Spacer(Modifier.height(14.dp))
                // 新建对话
                Row(
                    Modifier
                        .fillMaxWidth()
                        .pressable { onNewThread() }
                        .background(Pa.AccentDim, RoundedCornerShape(14.dp))
                        .padding(horizontal = 14.dp, vertical = 12.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Icon(
                        painterResource(AppR.drawable.ic_lucide_plus), null,
                        tint = Pa.Accent, modifier = Modifier.size(18.dp),
                    )
                    Spacer(Modifier.width(10.dp))
                    Text("新建对话", fontSize = 14.5.sp, color = Pa.Accent, fontWeight = FontWeight.SemiBold)
                }
                Spacer(Modifier.height(12.dp))
                // 搜索
                Row(
                    Modifier
                        .fillMaxWidth()
                        .background(Pa.Card, RoundedCornerShape(14.dp))
                        .padding(horizontal = 12.dp, vertical = 10.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Icon(
                        painterResource(AppR.drawable.ic_lucide_search), null,
                        tint = Pa.TextTertiary, modifier = Modifier.size(16.dp),
                    )
                    Spacer(Modifier.width(8.dp))
                    BasicTextField(
                        value = query,
                        onValueChange = { query = it },
                        modifier = Modifier.weight(1f),
                        textStyle = TextStyle(fontSize = 13.5.sp, color = Pa.TextPrimary),
                        cursorBrush = SolidColor(Pa.Accent),
                        singleLine = true,
                        decorationBox = { inner ->
                            Box {
                                if (query.isEmpty()) Text("搜索会话", fontSize = 13.5.sp, color = Pa.TextTertiary)
                                inner()
                            }
                        },
                    )
                }
                Spacer(Modifier.height(14.dp))
                Text("历史会话", fontSize = 11.sp, color = Pa.TextTertiary)
                Spacer(Modifier.height(6.dp))

                val filtered = threads.filter {
                    query.isBlank() || it.title.contains(query, ignoreCase = true)
                }
                LazyColumn(Modifier.weight(1f)) {
                    if (loading) {
                        item {
                            Text(
                                "读取中…", Modifier.padding(8.dp),
                                fontSize = 12.sp, color = Pa.TextTertiary,
                            )
                        }
                    }
                    items(filtered, key = { it.id }) { t ->
                        Row(
                            Modifier
                                .fillMaxWidth()
                                .pressable { onOpenThread(t.id) }
                                .background(
                                    if (t.id == currentThreadId) Pa.Hover else Color.Transparent,
                                    RoundedCornerShape(12.dp),
                                )
                                .padding(horizontal = 10.dp, vertical = 11.dp),
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            Column(Modifier.weight(1f)) {
                                Text(
                                    t.title.ifEmpty { "(未命名)" },
                                    fontSize = 13.5.sp,
                                    color = if (t.id == currentThreadId) Pa.Accent else Pa.TextPrimary,
                                    maxLines = 1,
                                )
                                Text(
                                    "${t.messageCount} 条消息",
                                    fontSize = 10.sp, color = Pa.TextTertiary,
                                )
                            }
                        }
                    }
                }
                // 底部:设置入口
                Box(Modifier.fillMaxWidth().height(1.dp).background(Pa.Separator))
                Spacer(Modifier.height(6.dp))
                Row(
                    Modifier
                        .fillMaxWidth()
                        .pressable { onOpenSettings() }
                        .padding(horizontal = 10.dp, vertical = 12.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Icon(
                        painterResource(AppR.drawable.ic_lucide_settings), null,
                        tint = Pa.TextSecondary, modifier = Modifier.size(18.dp),
                    )
                    Spacer(Modifier.width(10.dp))
                    Text("设置", fontSize = 14.sp, color = Pa.TextSecondary)
                }
                Spacer(Modifier.height(4.dp))
            }
        }
    }
}

/**
 * 会话页右侧 Git 侧栏:该会话所在项目(daemon 工作区)的版本管理 ——
 * 分支/变更列表/diff 查看/一键提交。从输入栏的 [Git] 图标展开。
 */
@Composable
fun ThreadGitPanel(repoPath: String?, onClose: () -> Unit) {
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
                if ("${e.message}".contains("not a git repository")) notRepo = true else error = "${e.message}"
            } finally { busy = false }
        }
    }

    LaunchedEffect(repoPath) {
        repo = repoPath.orEmpty()
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
        AnimatedVisibility(
            visible = visibleState.targetState,
            enter = slideInHorizontally(tween(260)) { it / 2 },
            exit = slideOutHorizontally(tween(200)) { it / 2 },
            modifier = Modifier
                .align(Alignment.CenterEnd)
                .fillMaxHeight(),
        ) {
            Column(
                Modifier
                    .fillMaxWidth(0.86f)
                    .widthIn(max = 310.dp)
                    .fillMaxHeight()
                    .background(Pa.CardHigh, RoundedCornerShape(topStart = 24.dp, bottomStart = 24.dp))
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
                            Text("工作区干净 ✨", Modifier.padding(vertical = 20.dp), fontSize = 13.sp, color = Pa.TextSecondary)
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
