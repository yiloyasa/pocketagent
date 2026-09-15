@file:OptIn(ExperimentalMaterial3Api::class, ExperimentalFoundationApi::class)

package dev.pocketagent.app.ui

import android.widget.Toast
import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
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
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.relocation.BringIntoViewRequester
import androidx.compose.foundation.relocation.bringIntoViewRequester
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Button
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.Icon
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.TextRange
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.TextFieldValue
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.AgentConnection
import dev.pocketagent.app.R
import dev.pocketagent.app.ui.anim.EmptyState
import dev.pocketagent.app.ui.anim.IconActionButton
import dev.pocketagent.app.ui.anim.edgeSwipeToOpenDrawer
import dev.pocketagent.app.ui.anim.pressable
import dev.pocketagent.app.ui.anim.pressableLong
import dev.pocketagent.app.ui.component.PaSkeletonList
import dev.pocketagent.app.ui.component.SkeletonKind
import dev.pocketagent.app.ui.component.SwipeAction
import dev.pocketagent.app.ui.component.SwipeRevealRow
import dev.pocketagent.app.ui.component.ThreadPins
import dev.pocketagent.app.ui.component.UndoBarHost
import dev.pocketagent.app.ui.component.rememberUndoState
import dev.pocketagent.app.ui.theme.Pa
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import pocketagent.v1.CreateThreadRequest
import pocketagent.v1.CodexThreadInfo
import pocketagent.v1.GetHostInfoRequest
import pocketagent.v1.HostInfo
import pocketagent.v1.ImportCodexThreadRequest
import pocketagent.v1.ListCodexThreadsRequest
import pocketagent.v1.ListModelsRequest
import pocketagent.v1.ListThreadsRequest
import pocketagent.v1.ArchiveThreadRequest
import pocketagent.v1.DeleteThreadRequest
import pocketagent.v1.ModelInfo
import pocketagent.v1.RenameThreadRequest
import pocketagent.v1.ThreadStatus
import pocketagent.v1.Thread as PThread

/**
 * 会话列表页 —— 对标参考图 2(主页):
 * 大标题左上,右上圆角方形工具按钮;连接状态卡;会话卡片列表;圆形 FAB。
 */
@Composable
fun ThreadsScreen(
    currentTab: Int = 1,
    onOpenThread: (String) -> Unit,
    onOpenTerminal: () -> Unit,
    onOpenSettings: () -> Unit = {},
    onOpenTaskCenter: () -> Unit = {},
    onOpenBrowser: () -> Unit = {},
    onGoTab: (Int) -> Unit = {},
    modifier: Modifier = Modifier,
) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    var threads by remember { mutableStateOf(listOf<PThread>()) }
    var host by remember { mutableStateOf<HostInfo?>(null) }
    var showArchived by remember { mutableStateOf(false) }
    // 电脑上**尚未导入**的 codex 会话:导入过的会以本地会话身份出现,不会在这里重复一条。
    var pcThreads by remember { mutableStateOf(listOf<CodexThreadInfo>()) }
    var importingId by remember { mutableStateOf("") }
    // 置顶(本地偏好):置顶项排在最前
    var pinned by remember { mutableStateOf(ThreadPins.all()) }
    // 行内重命名:把该行原地换成输入框。
    // 【关键】值由页面持有(单一真相),组件内部**不留任何副本** ——
    // 之前用 String + 组件内 remember(TextFieldValue) 的写法,两边会各持一份、
    // 互不同步(表现为"在下方输入框打的字进不到重命名框")。这里整条链路都用 TextFieldValue 贯通。
    var renamingId by remember { mutableStateOf<String?>(null) }
    var renameField by remember { mutableStateOf(TextFieldValue("")) }
    var showDrawer by remember { mutableStateOf(false) }
    // 侧滑互斥:openKey 单值状态保证「任意时刻最多一行展开」
    val openKey = remember { mutableStateOf<Any?>(null) }
    val listState = rememberLazyListState()
    val undo = rememberUndoState()
    var showNewThread by remember { mutableStateOf(false) }
    var loadError by remember { mutableStateOf<String?>(null) }
    // 电脑会话单独记录错误:拉不到时要能看见,而不是显示成「电脑里什么都没有」
    var pcError by remember { mutableStateOf<String?>(null) }
    // 首次加载中:列表区显示骨架屏(设计规范 §2.3)。只在"列表还空着"时才出,
    // 切 tab 时旧数据仍在,不会因为刷新闪一下骨架。
    var loading by remember { mutableStateOf(true) }

    /** 置顶键:优先用 codex 线程 id,保证"电脑里的那条"导入前后是同一个键。 */
    fun pinKey(local: PThread?, codexId: String?): String =
        codexId?.takeIf { it.isNotEmpty() }
            ?: local?.codexThreadId?.takeIf { it.isNotEmpty() }
            ?: local?.id.orEmpty()

    /**
     * 拉取会话列表。
     * 【统一数据源】"活跃"与"电脑"本来就是同一批 codex 会话:
     * 本地列表里 codex_thread_id 非空的就是"电脑上的那条",所以电脑列表要
     * 剔除这些 id,两处不再各显示一条;同时**每次都刷新两个列表**,
     * 避免删掉一条后电脑列表还是旧缓存(点开报"无法打开")。
     */
    fun refresh() = scope.launch {
        try {
            withContext(Dispatchers.IO) {
                host = AgentConnection.connectWithFallback()
                    ?: AgentConnection.hostService()
                        .getHostInfo(GetHostInfoRequest.getDefaultInstance())
                // daemon 的 include_archived=true 是"活跃+已归档"的全量,这里自己分流
                val all = AgentConnection.threadService()
                    .listThreads(
                        ListThreadsRequest.newBuilder()
                            .setIncludeArchived(true)
                            .build())
                    .threadsList
                threads = all.filter { t ->
                    if (showArchived) t.status == ThreadStatus.THREAD_STATUS_ARCHIVED
                    else t.status != ThreadStatus.THREAD_STATUS_ARCHIVED
                }
                // 最近工作区跟随会话记录自动补全
                threads.map { it.workspace }.filter { it.isNotEmpty() }
                    .distinct().take(5).forEach { AgentConnection.addRecentWorkspace(it) }

                val importedCodexIds = all.map { it.codexThreadId }.filter { it.isNotEmpty() }.toSet()
                // 老版本 daemon 不带 codex_thread_id:退回「标题 + 工作目录」匹配,尽量不去重失败
                val localByTitle = all
                    .filter { it.workspace.isNotEmpty() }
                    .map { it.title.trim() to it.workspace.trim() }
                    .toSet()
                // 手机端新建的会话标题(如「新会话 09-16 01:32」)与电脑端摘要标题完全不同,
                // 只按标题去重会漏掉 —— 再放一条兜底:同目录且更新时间相差 2 分钟以内,
                // 判定为同一条会话(手机刚建完,电脑侧 rollout 的时间戳几乎相同)。
                val localByPlaceAndTime = all
                    .filter { it.workspace.isNotEmpty() && it.updatedAtUnixMs > 0 }
                    .map { Triple(it.workspace.trim().lowercase(), it.updatedAtUnixMs, it.title) }
                pcThreads = if (showArchived) {
                    emptyList()
                } else {
                    try {
                        val list = AgentConnection.threadService()
                            .listCodexThreads(ListCodexThreadsRequest.newBuilder().build())
                            .threadsList
                            .filter { c ->
                                if (c.id in importedCodexIds) return@filter false
                                if ((c.title.trim() to c.cwd.trim()) in localByTitle) return@filter false
                                localByPlaceAndTime.none { (ws, at, _) ->
                                    ws == c.cwd.trim().lowercase() &&
                                        kotlin.math.abs(at - c.updatedAtUnixMs) < 120_000
                                }
                            }
                        pcError = null
                        list
                    } catch (e: Exception) {
                        pcError = "读取电脑上的 codex 会话失败:" + (e.message ?: "连接失败")
                        emptyList()
                    }
                }
                loadError = null
            }
        } catch (e: Exception) {
            loadError = "会话列表加载失败：${e.message ?: "连接失败"}"
        }
        pinned = ThreadPins.all()
        loading = false
    }

    /**
     * 删除会话(模块 1 + 模块 8):乐观移除 + 顶部可撤销入口。
     * 5 秒内点「撤销」是**纯本地还原,必然成功**(不受断网影响);
     * 只有过了撤销窗口才真正提交到 daemon。
     */
    fun deleteThreadOptimistic(t: PThread) {
        val index = threads.indexOfFirst { it.id == t.id }
        if (index < 0) return
        openKey.value = null
        threads = threads.toMutableList().also { it.removeAt(index) }
        undo.show(
            message = "已删除「${t.title.ifEmpty { "未命名" }}」",
            onUndo = {
                val cur = threads.toMutableList()
                if (cur.none { it.id == t.id }) cur.add(index.coerceAtMost(cur.size), t)
                threads = cur
            },
            onCommit = {
                withContext(Dispatchers.IO) {
                    AgentConnection.threadService().deleteThread(
                        DeleteThreadRequest.newBuilder().setThreadId(t.id).build())
                }
                // 删除会连带清掉电脑上的 rollout:立刻刷新,电脑列表里那条也随之消失
                refresh()
            },
        )
    }

    /** 归档 / 取消归档:从当前视图移除 + 顶部可撤销(与删除共用同一套交互)。 */
    fun archiveOptimistic(t: PThread) {
        openKey.value = null
        val before = threads
        val target = t.status != ThreadStatus.THREAD_STATUS_ARCHIVED
        threads = threads.filterNot { it.id == t.id }
        undo.show(
            message = (if (target) "已归档「" else "已恢复「") + t.title.ifEmpty { "未命名" } + "」",
            onUndo = { threads = before },
            onCommit = {
                withContext(Dispatchers.IO) {
                    AgentConnection.threadService().archiveThread(
                        ArchiveThreadRequest.newBuilder()
                            .setThreadId(t.id).setArchived(target).build())
                }
                refresh()
            },
        )
    }

    /** 删除"电脑"上的 codex 会话:先导入再删,daemon 会连带清掉电脑上的 rollout。 */
    fun deletePcOptimistic(c: CodexThreadInfo) {
        openKey.value = null
        val before = pcThreads
        pcThreads = pcThreads.filterNot { it.id == c.id }
        undo.show(
            message = "已删除 codex 会话「" + c.title.ifEmpty { "未命名会话" } + "」",
            onUndo = { pcThreads = before },
            onCommit = {
                withContext(Dispatchers.IO) {
                    val tid = AgentConnection.threadService()
                        .importCodexThread(
                            ImportCodexThreadRequest.newBuilder()
                                .setCodexThreadId(c.id)
                                .build())
                        .threadId
                    AgentConnection.threadService().deleteThread(
                        DeleteThreadRequest.newBuilder().setThreadId(tid).build())
                }
                refresh()
            },
        )
    }

    /** 提交行内重命名:空串视为取消;失败回滚标题。 */
    fun commitRename(t: PThread) {
        val title = renameField.text.trim()
        renamingId = null
        if (title.isEmpty() || title == t.title) return
        val finalTitle = title.take(80)
        val before = threads
        threads = threads.map { if (it.id == t.id) it.toBuilder().setTitle(finalTitle).build() else it }
        scope.launch {
            try {
                withContext(Dispatchers.IO) {
                    AgentConnection.threadService().renameThread(
                        RenameThreadRequest.newBuilder()
                            .setThreadId(t.id).setTitle(finalTitle).build())
                }
            } catch (e: Exception) {
                threads = before
                Toast.makeText(ctx, "重命名失败：${e.message ?: "连接失败"}", Toast.LENGTH_SHORT).show()
            }
        }
    }

    val reconnectTick = AgentConnection.connectionEpoch.collectAsState().value
    LaunchedEffect(Unit, showArchived, reconnectTick) { refresh() }

    // 【不再"一滚动就收起"】旧实现会在拖拽结束的那一帧把刚滑开的行收回去,
    // 看起来像"滑开了又自动弹回"。改为:纵向真正滚动超过一屏距离才收起(见 listState 监听无需)。
    // 左边缘右滑也能唤出侧边栏(问题 7)。挂在页面根节点上,子节点仍在命中链里,
    // 只有判定为"从左边缘起手的右滑"才会 consume,不会吃掉列表的点击。
    Box(
        modifier
            .fillMaxSize()
            .background(Pa.Bg)
            // 有行处于展开态时让位给"右滑收起动作":两者不再抢同一段手势
            .edgeSwipeToOpenDrawer(enabled = openKey.value == null) { showDrawer = true },
    ) {
        Column(Modifier.fillMaxSize().statusBarsPadding()) {
            // ---- 大标题 + 工具按钮 ----
            Row(
                Modifier.fillMaxWidth().padding(start = 20.dp, end = 16.dp, top = 12.dp),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Row(verticalAlignment = Alignment.CenterVertically, modifier = Modifier.weight(1f)) {
                    IconActionButton(R.drawable.ic_lucide_menu, "菜单", { showDrawer = true }, bg = Pa.Card)
                    Spacer(Modifier.width(12.dp))
                    Text("会话", fontSize = 30.sp, color = Pa.TextPrimary)
                }
                Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                    IconActionButton(R.drawable.ic_lucide_square_terminal, "终端", onOpenTerminal)
                    IconActionButton(R.drawable.ic_lucide_settings, "设置", onOpenSettings)
                }
            }

            // ---- 连接状态卡 ----
            ConnectionCard(
                host = host,
                onRetry = { refresh() },
                modifier = Modifier.padding(horizontal = 16.dp, vertical = 14.dp),
            )
            loadError?.let { message ->
                Row(
                    Modifier
                        .fillMaxWidth()
                        .padding(horizontal = 16.dp, vertical = 2.dp)
                        .background(Pa.Card, RoundedCornerShape(14.dp))
                        .padding(horizontal = 12.dp, vertical = 8.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(message, Modifier.weight(1f), fontSize = 12.sp, color = Pa.Red, maxLines = 2)
                    TextButton(onClick = { refresh() }) {
                        Text("重试", color = Pa.Accent)
                    }
                }
            }

            // ---- 过滤 chips:全部 / 已归档(电脑上未导入的 codex 会话合并在"全部"里) ----
            Row(Modifier.padding(start = 16.dp, end = 16.dp, top = 2.dp)) {
                FilterChip2("全部", !showArchived) { showArchived = false }
                Spacer(Modifier.width(8.dp))
                FilterChip2("已归档", showArchived) { showArchived = true }
            }

            // ---- 会话列表:本地会话 + 电脑上未导入的 codex 会话,合成一条流 ----
            // "活跃"和"电脑"本来就是同一批 codex 会话,这里不再分两个标签页;
            // 置顶项恒排最前,其余按更新时间倒序。
            val pinnedKeys = pinned
            val rows: List<SessionRow> = buildList {
                threads.forEach { add(SessionRow.Local(it, pinKey(it, null))) }
                if (!showArchived) pcThreads.forEach { add(SessionRow.Pc(it)) }
            }.sortedWith(
                compareByDescending<SessionRow> { it.pinKey in pinnedKeys }
                    .thenByDescending { it.sortKey },
            )

            when {
                loading && rows.isEmpty() -> {
                    // 骨架屏:形状与真实会话行逐项对位(SESSION_ROW = 圆头像 + 两行文本条)
                    PaSkeletonList(
                        kind = SkeletonKind.SESSION_ROW,
                        modifier = Modifier.padding(horizontal = 16.dp),
                    )
                }
                rows.isEmpty() -> {
                    Column(Modifier.fillMaxSize()) {
                    pcError?.let { message ->
                        Text(
                            message,
                            Modifier
                                .fillMaxWidth()
                                .padding(horizontal = 16.dp, vertical = 8.dp)
                                .background(Pa.Red.copy(alpha = 0.12f), RoundedCornerShape(12.dp))
                                .padding(12.dp),
                            fontSize = 12.sp,
                            color = Pa.Red,
                        )
                    }
                    EmptyState(
                        iconRes = if (showArchived) R.drawable.ic_lucide_archive else R.drawable.ic_lucide_message_square,
                        title = if (showArchived) "没有归档的会话" else "还没有会话",
                        hint = if (showArchived) "左滑会话可恢复或删除" else "点右下角 + 开始你的第一个任务",
                    )
                    }
                }
                else -> {
                    LazyColumn(
                        Modifier.fillMaxSize(),
                        state = listState,
                        contentPadding = PaddingValues(horizontal = 16.dp, vertical = 4.dp),
                        verticalArrangement = Arrangement.spacedBy(10.dp),
                    ) {
                        items(
                            items = rows,
                            key = { r ->
                                when (r) {
                                    is SessionRow.Local -> "l-${r.t.id}"
                                    is SessionRow.Pc -> "p-${r.c.id}"
                                }
                            },
                        ) { row ->
                            when (row) {
                                is SessionRow.Local -> {
                                    val t = row.t
                                    val archived = t.status == ThreadStatus.THREAD_STATUS_ARCHIVED
                                    val isPinned = row.pinKey in pinnedKeys
                                    SwipeRevealRow(
                                        rowKey = t.id,
                                        openKey = openKey,
                                        actions = listOf(
                                            SwipeAction(
                                                "pin",
                                                if (isPinned) "取消置顶" else "置顶",
                                                R.drawable.ic_lucide_pin,
                                                Pa.AccentText,
                                            ),
                                            SwipeAction(
                                                "archive",
                                                if (archived) "恢复" else "归档",
                                                R.drawable.ic_lucide_archive,
                                                Pa.AccentText,
                                            ),
                                            SwipeAction(
                                                "rename", "重命名",
                                                R.drawable.ic_lucide_pencil, Pa.AccentText,
                                            ),
                                            SwipeAction(
                                                "delete", "删除", R.drawable.ic_lucide_trash_2,
                                                Color.White, destructive = true,
                                            ),
                                        ),
                                        onAction = { a ->
                                            when (a.id) {
                                                "pin" -> {
                                                    ThreadPins.toggle(row.pinKey)
                                                    pinned = ThreadPins.all()
                                                    openKey.value = null
                                                    Toast.makeText(
                                                        ctx,
                                                        if (isPinned) "已取消置顶" else "已置顶",
                                                        Toast.LENGTH_SHORT,
                                                    ).show()
                                                }
                                                "archive" -> archiveOptimistic(t)
                                                "rename" -> {
                                                    renameField = TextFieldValue(
                                                        t.title,
                                                        selection = TextRange(0, t.title.length),
                                                    )
                                                    renamingId = t.id
                                                }
                                                "delete" -> deleteThreadOptimistic(t)
                                            }
                                        },
                                        rowBackground = Pa.Card,
                                        // 已展开时点内容 = 收起,未展开才进会话
                                        onClick = {
                                            if (openKey.value == t.id) openKey.value = null else onOpenThread(t.id)
                                        },
                                    ) {
                                        ThreadCard(
                                            t = t,
                                            pinned = isPinned,
                                            renaming = renamingId == t.id,
                                            renameField = renameField,
                                            onRenameFieldChange = { renameField = it },
                                            onRenameCommit = { commitRename(t) },
                                        )
                                    }
                                }
                                is SessionRow.Pc -> {
                                    val c = row.c
                                    val isPinned = row.pinKey in pinnedKeys
                                    SwipeRevealRow(
                                        rowKey = c.id,
                                        openKey = openKey,
                                        actions = listOf(
                                            SwipeAction(
                                                "pin",
                                                if (isPinned) "取消置顶" else "置顶",
                                                R.drawable.ic_lucide_pin,
                                                Pa.AccentText,
                                            ),
                                            SwipeAction(
                                                "delete", "删除", R.drawable.ic_lucide_trash_2,
                                                Color.White, destructive = true,
                                            ),
                                        ),
                                        onAction = { a ->
                                            when (a.id) {
                                                "pin" -> {
                                                    ThreadPins.toggle(row.pinKey)
                                                    pinned = ThreadPins.all()
                                                    openKey.value = null
                                                }
                                                "delete" -> deletePcOptimistic(c)
                                            }
                                        },
                                        rowBackground = Pa.Card,
                                        onClick = {
                                            if (openKey.value == c.id) {
                                                openKey.value = null
                                            } else if (importingId.isEmpty()) {
                                                importingId = c.id
                                                scope.launch {
                                                    try {
                                                        val tid = withContext(Dispatchers.IO) {
                                                            AgentConnection.threadService()
                                                                .importCodexThread(
                                                                    ImportCodexThreadRequest.newBuilder()
                                                                        .setCodexThreadId(c.id)
                                                                        .build())
                                                                .threadId
                                                        }
                                                        onOpenThread(tid)
                                                    } catch (e: Exception) {
                                                        // 会话文件可能已被删掉:刷新一次列表,让它自动消失
                                                        Toast.makeText(
                                                            ctx,
                                                            "打开电脑会话失败：${e.message ?: "连接失败"}",
                                                            Toast.LENGTH_SHORT,
                                                        ).show()
                                                        refresh()
                                                    } finally {
                                                        importingId = ""
                                                    }
                                                }
                                            }
                                        },
                                    ) {
                                        PcSessionCard(info = c, importing = importingId == c.id)
                                    }
                                }
                            }
                        }
                        item { Spacer(Modifier.height(80.dp)) }
                    }
                }
            }
        }

        // ---- 撤销条(模块 8):删除会话后顶部浮出,5 秒内可一键还原 ----
        UndoBarHost(undo, Modifier.align(Alignment.TopCenter))

        // ---- FAB:圆形深灰卡 + 蓝色 + 号(弹出新建会话,可自选工作区) ----
        Box(
            Modifier
                .align(Alignment.BottomEnd)
                .padding(end = 24.dp, bottom = 28.dp)
                .size(60.dp)
                .pressable(scaleDown = 0.9f) { showNewThread = true }
                .background(Pa.CardHigh, CircleShape),
            contentAlignment = Alignment.Center,
        ) {
            Icon(
                painterResource(R.drawable.ic_lucide_plus), "新会话",
                tint = Pa.Accent, modifier = Modifier.size(26.dp),
            )
        }
    }

    // 新建会话抽屉:选工作区(在哪个文件夹里工作)
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
                        Toast.makeText(
                            ctx,
                            "创建失败：${e.message ?: "连接失败"}",
                            Toast.LENGTH_SHORT,
                        ).show()
                    }
                }
            },
        )
    }

    // 长按菜单已移除:重命名 / 归档 / 删除 全部收敛到左滑动作里,
    // 一条交互路径比"长按弹一层、再点一次"更连贯(也为后续接入长按上下文菜单留位)。

    // 重命名已改为「行内编辑」(见 ThreadCard 的 renaming 分支),不再使用对话框 —— 设计规范 §3.1

    if (showDrawer) {
        AppDrawer(
            currentTab = currentTab,
            hostTitle = host?.displayName ?: "未连接电脑",
            onClose = { showDrawer = false },
            onGoTab = { onGoTab(it) },
            onOpenTerminal = { onOpenTerminal() },
            onOpenSettings = { onOpenSettings() },
            onOpenTaskCenter = onOpenTaskCenter,
            onOpenBrowser = { onOpenBrowser() },
        )
    }
}

/**
 * 会话列表的一行:本地会话 或 电脑上未导入的 codex 会话。
 * 合成一条流之后,置顶/排序才有统一的依据。
 */
private sealed class SessionRow {
    abstract val sortKey: Long
    abstract val pinKey: String

    class Local(val t: PThread, override val pinKey: String) : SessionRow() {
        override val sortKey: Long get() = t.updatedAtUnixMs
    }

    class Pc(val c: CodexThreadInfo) : SessionRow() {
        override val sortKey: Long get() = c.updatedAtUnixMs
        override val pinKey: String get() = c.id
    }
}

/** 连接状态卡:未连接=中性;已连接=主机名+规格+外网直连行。 */
@Composable
private fun ConnectionCard(host: HostInfo?, onRetry: () -> Unit, modifier: Modifier = Modifier) {
    val connState = AgentConnection.connState.collectAsState().value
    val transport = AgentConnection.connTransport.collectAsState().value
    val addr = AgentConnection.connAddress.collectAsState().value
    val diagnostic = AgentConnection.connError.collectAsState().value
    val ok = connState == "ok" && host != null
    Row(
        modifier
            .fillMaxWidth()
            .pressable { onRetry() }
            .background(Pa.Card, RoundedCornerShape(20.dp))
            .padding(16.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box(
            Modifier.size(44.dp).background(Pa.AccentDim, RoundedCornerShape(13.dp)),
            contentAlignment = Alignment.Center,
        ) {
            Icon(
                painterResource(R.drawable.ic_lucide_monitor), null,
                tint = Pa.Accent, modifier = Modifier.size(22.dp),
            )
        }
        Spacer(Modifier.width(12.dp))
        Column(Modifier.weight(1f)) {
            if (ok) {
                Text(host!!.displayName, fontSize = 16.sp, color = Pa.TextPrimary)
                Text(
                    "${host.os}/${host.arch} · 工作区 ${host.workspacesList.firstOrNull() ?: "-"}",
                    fontSize = 12.sp,
                    color = Pa.TextSecondary,
                    modifier = Modifier.padding(top = 2.dp),
                )
                Text(
                    "已连接 · $transport · $addr",
                    fontSize = 11.sp,
                    color = Pa.Accent,
                    modifier = Modifier.padding(top = 3.dp),
                )
            } else {
                Text("未连接", fontSize = 16.sp, color = Pa.TextPrimary)
                Text(
                    if (connState == "down") {
                        diagnostic?.message ?: "自动重连中(每 5 秒)· USB → 局域网 → Tailscale 回退"
                    } else {
                        "正在检查连接…"
                    },
                    fontSize = 12.sp,
                    color = if (diagnostic != null) Pa.Red else Pa.TextSecondary,
                    modifier = Modifier.padding(top = 2.dp),
                )
                Text("点按立即重试", fontSize = 11.sp, color = Pa.TextTertiary, modifier = Modifier.padding(top = 3.dp))
            }
        }
        Box(
            Modifier.size(8.dp).background(
                when {
                    ok -> Pa.Green
                    connState == "down" -> Pa.Red
                    else -> Pa.TextTertiary
                },
                CircleShape,
            ),
        )
    }
}

/** 会话卡片:标题 + 摘要行。重命名时该行**原地**变成输入框(设计规范 §3.1,全程无对话框)。 */
@Composable
private fun ThreadCard(
    t: PThread,
    pinned: Boolean = false,
    renaming: Boolean,
    renameField: TextFieldValue,
    onRenameFieldChange: (TextFieldValue) -> Unit,
    onRenameCommit: () -> Unit,
) {
    Column(
        Modifier
            .fillMaxWidth()
            // 不要在这里挂 clickable:手势由外层 SwipeRevealRow 独占,否则侧滑会被内层消费掉
            .background(Pa.Card, RoundedCornerShape(18.dp))
            .padding(horizontal = 16.dp, vertical = 14.dp),
    ) {
        if (renaming) {
            val focus = remember { FocusRequester() }
            val bringIntoView = remember { BringIntoViewRequester() }
            LaunchedEffect(Unit) {
                focus.requestFocus()
                // 软键盘是异步弹出的:等它占好高度后再把这一行滚进可视区,
                // 否则位于列表底部的条目会被输入法盖住(真机实测踩到过)。
                kotlinx.coroutines.delay(320)
                runCatching { bringIntoView.bringIntoView() }
            }
            OutlinedTextField(
                // 受控组件:值完全由页面提供,内部不留副本 —— 保证与外部双向实时同步
                value = renameField,
                onValueChange = onRenameFieldChange,
                singleLine = true,
                textStyle = androidx.compose.ui.text.TextStyle(fontSize = 15.sp),
                shape = RoundedCornerShape(12.dp),
                modifier = Modifier
                    .fillMaxWidth()
                    .focusRequester(focus)
                    .bringIntoViewRequester(bringIntoView),
                keyboardOptions = KeyboardOptions(imeAction = ImeAction.Done),
                keyboardActions = KeyboardActions(onDone = { onRenameCommit() }),
            )
        } else {
            Row(verticalAlignment = Alignment.CenterVertically) {
                if (pinned) {
                    Icon(
                        painterResource(R.drawable.ic_lucide_pin), "已置顶",
                        tint = Pa.AccentText, modifier = Modifier.size(12.dp),
                    )
                    Spacer(Modifier.width(5.dp))
                }
                Text(
                    t.title.ifEmpty { "(未命名)" },
                    fontSize = 15.sp,
                    color = Pa.TextPrimary,
                    maxLines = 1,
                )
            }
        }
        Row(verticalAlignment = Alignment.CenterVertically, modifier = Modifier.padding(top = 4.dp)) {
            if (t.status == ThreadStatus.THREAD_STATUS_ARCHIVED) {
                Text("已归档", fontSize = 10.sp, color = Pa.Accent)
                Text("  ·  ", fontSize = 10.sp, color = Pa.TextTertiary)
            }
            Text(
                "${t.messageCount} 条消息 · " +
                    java.text.SimpleDateFormat("MM-dd HH:mm").format(java.util.Date(t.updatedAtUnixMs)),
                fontSize = 12.sp,
                color = Pa.TextSecondary,
            )
            if (t.workspace.isNotEmpty()) {
                Text(
                    "  ·  " + t.workspace.substringAfterLast('/').substringAfterLast('\\'),
                    fontSize = 12.sp,
                    color = Pa.TextTertiary,
                    maxLines = 1,
                )
            }
        }
    }
}

/** "电脑"标签里的 codex 历史会话卡:点一下导入并打开,左滑可删除(含电脑上的 rollout)。 */
@Composable
private fun PcSessionCard(
    info: CodexThreadInfo,
    importing: Boolean,
) {
    Column(
        Modifier
            .fillMaxWidth()
            // 手势由外层 SwipeRevealRow 独占,这里不再挂 clickable
            .background(Pa.Card, RoundedCornerShape(18.dp))
            .padding(horizontal = 16.dp, vertical = 14.dp),
    ) {
        Text(
            info.title.ifEmpty { "(未命名会话)" },
            fontSize = 15.sp,
            color = Pa.TextPrimary,
            maxLines = 1,
        )
        Row(verticalAlignment = Alignment.CenterVertically, modifier = Modifier.padding(top = 4.dp)) {
            Text(
                if (importing) "正在打开…" else "codex 会话",
                fontSize = 10.sp,
                color = if (importing) Pa.Accent else Pa.Accent,
            )
            Text("  ·  ", fontSize = 10.sp, color = Pa.TextTertiary)
            Text(
                java.text.SimpleDateFormat("MM-dd HH:mm")
                    .format(java.util.Date(info.updatedAtUnixMs)),
                fontSize = 12.sp,
                color = Pa.TextSecondary,
            )
            if (info.cwd.isNotEmpty()) {
                Text(
                    "  ·  " + info.cwd.substringAfterLast('/').substringAfterLast('\\'),
                    fontSize = 12.sp,
                    color = Pa.TextTertiary,
                    maxLines = 1,
                )
            }
        }
    }
}

/**
 * 新建会话抽屉:先选"在哪个文件夹里工作"。
 * 工作区通过文件式目录选择器浏览电脑选定,真实传给 codex(rollout 的 cwd),
 * 桌面端在同项目下可见本会话。所有新建对话入口共用。
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun NewThreadSheet(onDismiss: () -> Unit, onCreate: (workspace: String) -> Unit) {
    val state = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    val recents = AgentConnection.recentWorkspaces.collectAsState().value
    var picked by remember { mutableStateOf("") } // 空 = 默认工作区
    var showBrowser by remember { mutableStateOf(false) }
    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = state,
        containerColor = Pa.CardHigh,
        shape = RoundedCornerShape(topStart = 26.dp, topEnd = 26.dp),
    ) {
        Text(
            "新建会话", Modifier.fillMaxWidth().padding(horizontal = 24.dp),
            fontSize = 17.sp, color = Pa.TextPrimary,
        )
        Text(
            "选择这个会话在电脑上的工作文件夹(agent 只在该文件夹里干活)",
            Modifier.fillMaxWidth().padding(horizontal = 24.dp, vertical = 4.dp),
            fontSize = 12.sp, color = Pa.TextSecondary,
        )
        Spacer(Modifier.height(10.dp))
        Column(Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 4.dp)) {
            recents.map { it to it }.forEach { (ws, label) ->
                Row(
                    Modifier
                        .fillMaxWidth()
                        .pressable { picked = ws }
                        .background(if (picked == ws) Pa.Hover else Color.Transparent, RoundedCornerShape(12.dp))
                        .padding(horizontal = 12.dp, vertical = 10.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Icon(
                        painterResource(R.drawable.ic_lucide_folder), null,
                        tint = if (picked == ws) Pa.Accent else Pa.TextSecondary,
                        modifier = Modifier.size(16.dp),
                    )
                    Spacer(Modifier.width(10.dp))
                    Text(
                        label, fontSize = 13.sp,
                        color = if (picked == ws) Pa.Accent else Pa.TextPrimary,
                        maxLines = 1,
                    )
                }
            }
            if (recents.isEmpty()) {
                Text(
                    "请从“浏览电脑文件夹”选择本次会话的工作区",
                    fontSize = 12.sp,
                    color = Pa.TextTertiary,
                    modifier = Modifier.padding(horizontal = 4.dp, vertical = 6.dp),
                )
            }
            // 文件式浏览电脑选文件夹
            Row(
                Modifier
                    .fillMaxWidth()
                    .pressable { showBrowser = true }
                    .background(Pa.Card, RoundedCornerShape(12.dp))
                    .padding(horizontal = 12.dp, vertical = 12.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Icon(
                    painterResource(R.drawable.ic_lucide_monitor), null,
                    tint = Pa.Accent, modifier = Modifier.size(16.dp),
                )
                Spacer(Modifier.width(10.dp))
                Text("浏览电脑文件夹…", fontSize = 13.sp, color = Pa.Accent)
            }
            if (picked.isNotBlank()) {
                Text(
                    "已选:$picked",
                    fontSize = 11.sp, color = Pa.TextTertiary,
                    modifier = Modifier.padding(horizontal = 4.dp, vertical = 6.dp),
                    maxLines = 1,
                )
            }
            Spacer(Modifier.height(12.dp))
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.End) {
                TextButton(onClick = onDismiss) { Text("取消") }
                Spacer(Modifier.width(8.dp))
                Button(
                    enabled = picked.isNotBlank(),
                    onClick = { onCreate(picked) },
                    colors = androidx.compose.material3.ButtonDefaults.buttonColors(containerColor = Pa.Accent),
                    shape = RoundedCornerShape(14.dp),
                ) {
                    Text(
                        if (picked.isBlank()) "先选工作区" else "创建",
                        color = if (picked.isBlank()) Pa.TextTertiary else Color(0xFF0B0B0D),
                    )
                }
            }
            Spacer(Modifier.height(18.dp))
        }
    }

    if (showBrowser) {
        FolderPickerSheet(
            onDismiss = { showBrowser = false },
            onPick = { picked = it; showBrowser = false },
        )
    }
}


/** 小过滤 chip(活跃/已归档)。 */
@Composable
fun FilterChip2(label: String, selected: Boolean, onClick: () -> Unit) {
    Box(
        Modifier
            .pressable { onClick() }
            .background(
                if (selected) Pa.AccentDim else Pa.Card,
                RoundedCornerShape(50),
            )
            .padding(horizontal = 16.dp, vertical = 7.dp),
    ) {
        Text(label, fontSize = 13.sp, color = if (selected) Pa.Accent else Pa.TextSecondary)
    }
}

/** 抽屉操作行:图标 + 文案(重命名/归档/删除)。 */
@Composable
fun SheetAction(iconRes: Int, label: String, tint: Color = Pa.TextPrimary, onClick: () -> Unit) {
    Row(
        Modifier
            .fillMaxWidth()
            .pressable { onClick() }
            .padding(horizontal = 24.dp, vertical = 14.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Icon(painterResource(iconRes), null, tint = tint, modifier = Modifier.size(20.dp))
        Spacer(Modifier.width(14.dp))
        Text(label, fontSize = 15.sp, color = tint)
    }
}
