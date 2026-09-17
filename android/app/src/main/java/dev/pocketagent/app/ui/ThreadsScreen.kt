@file:OptIn(ExperimentalMaterial3Api::class, ExperimentalFoundationApi::class)

package dev.pocketagent.app.ui

import android.widget.Toast
import androidx.activity.compose.BackHandler
import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.horizontalScroll
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
import androidx.compose.foundation.rememberScrollState
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
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.TextRange
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.TextFieldValue
import androidx.compose.ui.text.style.TextOverflow
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
import dev.pocketagent.app.ui.component.shortWorkspace
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
import pocketagent.v1.ListThreadSectionsRequest
import pocketagent.v1.ArchiveThreadRequest
import pocketagent.v1.CreateThreadSectionRequest
import pocketagent.v1.DeleteThreadRequest
import pocketagent.v1.DeleteThreadSectionRequest
import pocketagent.v1.ForkThreadRequest
import pocketagent.v1.ModelInfo
import pocketagent.v1.MoveThreadToSectionRequest
import pocketagent.v1.RenameThreadRequest
import pocketagent.v1.RenameThreadSectionRequest
import pocketagent.v1.ThreadStatus
import pocketagent.v1.Thread as PThread
import pocketagent.v1.ThreadSection as PSection

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
    /** 返回层级:二级页面按返回 → 回主页并展开侧边栏。 */
    onBackToHome: () -> Unit = {},
    onGoTab: (Int) -> Unit = {},
    modifier: Modifier = Modifier,
) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    var threads by remember { mutableStateOf(listOf<PThread>()) }
    var host by remember { mutableStateOf<HostInfo?>(null) }
    // 【筛选维度】全部 / 已归档 / 未分组 / 某个分组 —— 单值互斥(见 ThreadOrganize.kt)。
    // 以前这里是 showArchived 一个布尔量,加上分组后布尔量表达不了第三、第四态。
    var filter by remember { mutableStateOf(THREAD_FILTER_ALL) }
    // codex 侧的分组(事实来源在电脑上:在电脑上分的组,手机上立刻能看到)。
    var sections by remember { mutableStateOf(listOf<PSection>()) }
    // 分组管理面板:新建 / 重命名 / 删除
    var sectionSheet by remember { mutableStateOf<SectionSheet?>(null) }
    var sectionField by remember { mutableStateOf("") }
    var sectionBusy by remember { mutableStateOf(false) }
    // 长按会话卡后的"组织"面板(移动到分组 / 分叉)
    var organizeTarget by remember { mutableStateOf<PThread?>(null) }
    // 筛选 chip 行的滚动位置。分组数量不确定,行内横向滚动;新建完要把它滚出来。
    val chipScroll = rememberScrollState()
    var scrollChipsToEnd by remember { mutableStateOf(false) }
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
    // 侧边栏跟手:与主页/会话页同一套 controller
    val drawerCtrl = dev.pocketagent.app.ui.component.rememberDrawerController()
    // 新建会话面板(返回键要用,声明必须早于 BackHandler)
    var showNewThread by remember { mutableStateOf(false) }
    // 归档筛选的旧口径,由 filter 派生 —— 保留它是因为列表分流、电脑会话过滤、
    // 空态文案都按"当前是不是在看归档"分支,三处语义完全一致,再各自判断 filter
    // 只会让它们有机会不一致。
    val showArchived = filter == THREAD_FILTER_ARCHIVED
    // 【返回层级 · 分级回退】抽屉 → 正在重命名 → 新建面板 → 组织/分组面板 → 筛选 → 主板块。
    // 之前只有"抽屉/回主页"两级:在"已归档"里按返回会直接跳回主页,
    // 重命名框开着也会被判为"该回主板块"。
    BackHandler(enabled = true) {
        when {
            drawerCtrl.visible -> drawerCtrl.close()
            renamingId != null -> renamingId = null
            showNewThread -> showNewThread = false
            organizeTarget != null -> organizeTarget = null
            sectionSheet != null -> sectionSheet = null
            filter != THREAD_FILTER_ALL -> filter = THREAD_FILTER_ALL
            else -> onBackToHome()
        }
    }
    // 侧滑互斥:openKey 单值状态保证「任意时刻最多一行展开」
    val openKey = remember { mutableStateOf<Any?>(null) }
    val listState = rememberLazyListState()
    val undo = rememberUndoState()
    var loadError by remember { mutableStateOf<String?>(null) }
    // 列表是否是"离线快照"渲染的(决定错误条的颜色:离线用提示色而非报错色)
    var offline by remember { mutableStateOf(false) }
    // 重试进行中:点「重试」先自愈半死通道再拉列表,期间按钮显示"进行中"防抖,
    // 避免连续点击重复触发(也避免重试仍打在已坏的通道上)。
    var retrying by remember { mutableStateOf(false) }
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
    // doRefresh:真正的拉取逻辑(可挂起、可等待)。refresh() 只是把它丢进协程跑,
    // 这样重试可以在"回收半死通道"之后再等待本次拉取结束(按钮显示进行中防抖)。
    suspend fun doRefresh() {
        // 快照要落"未过滤的全量",而 all 是 withContext 块里的局部量 ——
        // 用外层变量把它带出来(同一协程顺序执行,不存在竞态)。
        var allForSnapshot: List<PThread> = emptyList()
        // gRPC 是否可达:getHostInfo 成功 = daemon 活着(顶部连接卡据此显示"已连接")。
        // 区分"连不上 daemon"(真离线)与"连得上但列表拉取失败"(codex 上游无响应),
        // 两者横幅文案与配色不同,不能都写成"离线"。
        var hostReached = false
        try {
            withContext(Dispatchers.IO) {
                // 先探测 gRPC:成功才认为连接健康;失败则 hostReached=false,
                // 进入 catch 后按"离线"分支展示快照。
                host = runCatching {
                    AgentConnection.connectWithFallback()
                        ?: AgentConnection.hostService()
                            .getHostInfo(GetHostInfoRequest.getDefaultInstance())
                }.onSuccess { hostReached = true }.getOrThrow()
                // daemon 的 include_archived=true 是"活跃+已归档"的全量,这里自己分流
                val all = AgentConnection.threadService()
                    .listThreads(
                        ListThreadsRequest.newBuilder()
                            .setIncludeArchived(true)
                            .build())
                    .threadsList
                // 【存全量,渲染时再筛】threads 一律保存 daemon 给的原始列表(含归档、
                // 带 section_id),筛选只在渲染那一步做。理由有两条:
                //   1) 切分组/切归档是纯本地动作,不再重新拉一次(拉一次要等 daemon 把
                //      codex 侧的分组状态同步进本地库,冷启动时 1~2 秒);
                //   2) 列表不再被"当前视图"裁剪,离线快照因此天然是全量的。
                threads = all
                // 最近工作区跟随会话记录自动补全
                threads.map { it.workspace }.filter { it.isNotEmpty() }
                    .distinct().take(5).forEach { AgentConnection.addRecentWorkspace(it) }

                val importedCodexIds = all.map { it.codexThreadId }.filter { it.isNotEmpty() }.toSet()

                // ---- 分组:事实来源在 codex 侧,每次刷新跟着列表一起拉 ----
                // 【失败不清空】分组拉不到(电脑上 codex 没起来/网络抖动)时保留上一次的,
                // 让用户继续按老分组看;清空会让他以为"分组全没了"。
                runCatching {
                    sections = AgentConnection.threadService()
                        .listThreadSections(ListThreadSectionsRequest.getDefaultInstance())
                        .sectionsList
                }

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
                // 电脑上未导入的 codex 会话只在"全部"里露面:它们没有分组信息
                // (CodexThreadInfo 不带 section),放进分组视图会让用户以为
                // "这个分组里有几条没归组的"。
                pcThreads = if (filter != THREAD_FILTER_ALL) {
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
                offline = false

                // 【失效键回收】会话在电脑端被删后,手机端的 threadModel/usage 键会永久残留。
                // 这里在 IO 线程内用"全量列表"(all 含归档)比对,延迟 7 天清理。
                runCatching {
                    val live: Set<String> = all.map { it.id }.filter { it.isNotEmpty() }.toSet()
                    dev.pocketagent.app.ui.component.ThreadSnapshot.pruneStalePrefs(live)
                }
            }
            // 【离线快照】拉取成功就落一份本地副本:断网时列表还能照常显示
            // (遥控器形态下,"上次连上时有哪些会话"本身就是有用信息)。
            // threads 现在是未裁剪的全量(见上面的注释),所以快照天然是全量的。
            dev.pocketagent.app.ui.component.ThreadSnapshot.saveThreads(
                threads.map {
                    dev.pocketagent.app.ui.component.ThreadSnapshot.Row(
                        id = it.id,
                        title = it.title,
                        updatedAtUnixMs = it.updatedAtUnixMs,
                        workspace = it.workspace,
                        codexThreadId = it.codexThreadId,
                        archived = it.status == ThreadStatus.THREAD_STATUS_ARCHIVED,
                        sectionId = it.sectionId,
                    )
                },
            )
        } catch (e: Exception) {
            // 连不上就退回快照:有快照 → 显示并标明是离线副本;没有 → 保持原报错。
            val snap = dev.pocketagent.app.ui.component.ThreadSnapshot.loadThreads()
                .filter {
                    matchesThreadFilter(
                        sectionId = it.sectionId,
                        archived = it.archived,
                        filter = filter,
                    )
                }
            if (snap.isNotEmpty()) {
                threads = snap.map {
                    PThread.newBuilder()
                        .setId(it.id)
                        .setTitle(it.title)
                        .setUpdatedAtUnixMs(it.updatedAtUnixMs)
                        .setWorkspace(it.workspace)
                        .setCodexThreadId(it.codexThreadId)
                        .build()
                }
                pcThreads = emptyList()
                pcError = null
                val at = dev.pocketagent.app.ui.component.ThreadSnapshot.threadsSnapshotAt()
                // 纯函数构造横幅:区分"连不上 daemon"(真离线)与"连得上但 codex 无响应"
                // (中性,不写"离线"),避免和上方的"已连接"连接卡自相矛盾。
                val banner = buildListBanner(
                    connectionOk = hostReached,
                    snapshotAtMs = at,
                    agoText = { ms -> dev.pocketagent.app.ui.component.agoText(ms) },
                )
                loadError = banner.message
                // 配色跟随横幅语义:真离线(黄·快照)与 codex 无响应(中性)都不按硬错误红;
                // 只有"连快照都没有"的真·加载失败才用红(见下面的 else 分支)。
                offline = banner.offline
            } else {
                loadError = "会话列表加载失败：${e.message ?: "连接失败"}"
                offline = false
            }
        }
        pinned = ThreadPins.all()
        loading = false
    }

    /** 暴露给 UI 的非挂起入口:把 doRefresh 丢进协程执行。 */
    fun refresh() = scope.launch { doRefresh() }

    /**
     * 重试:先自愈半死通道(OkHttp 传输 UNAVAILABLE 半死态),再拉列表。
     * 否则重试仍打在同一条已坏通道上 —— 表现就是"点重试毫无效果"。
     * 期间按钮显示"进行中"防抖,本次拉取真正结束才解除。
     */
    fun retry() {
        if (retrying) return
        retrying = true
        scope.launch {
            try {
                AgentConnection.recycleChannel("会话列表重试:自愈半死通道")
                doRefresh()
            } finally {
                retrying = false
            }
        }
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

    // ---- 会话组织:分组增删改移 + 分叉 ----
    // 【为什么全部走 refresh() 收尾】分组是 codex 侧的事实(在电脑上分的组手机上要
    // 立刻能看到),本地自己拼一份状态迟早两端不一致。这几个动作都低频,多拉一次
    // 列表换"看到的就是真的",值。

    /** 把会话移进分组;sectionId 为空 = 移出分组。 */
    fun moveThreadToSection(t: PThread, sectionId: String) {
        organizeTarget = null
        scope.launch {
            try {
                withContext(Dispatchers.IO) {
                    AgentConnection.threadService().moveThreadToSection(
                        MoveThreadToSectionRequest.newBuilder()
                            .setThreadId(t.id)
                            .setSectionId(sectionId)
                            .build())
                }
                if (sectionId.isEmpty()) {
                    Toast.makeText(ctx, "已移出分组", Toast.LENGTH_SHORT).show()
                }
                refresh()
            } catch (e: Exception) {
                // daemon 对"还没在电脑上落地"的会话回 FAILED_PRECONDITION,那句话
                // 本来就是写给用户看的("先发一条消息"),直接透出比翻译成"系统错误"有用。
                Toast.makeText(ctx, e.message ?: "移动分组失败", Toast.LENGTH_LONG).show()
            }
        }
    }

    /** 分叉会话:复制出一条新会话并直接打开,原会话保持不动。 */
    fun forkThread(t: PThread) {
        organizeTarget = null
        scope.launch {
            try {
                val nt = withContext(Dispatchers.IO) {
                    AgentConnection.threadService().forkThread(
                        ForkThreadRequest.newBuilder()
                            .setThreadId(t.id)
                            // last_turn_id 留空 = 从最新状态分叉。手机端时间线里没有
                            // codex 的回合 id(历史接口不返回 turnId),给不出"从某个回合分叉",
                            // 所以不做一个选了也没用的回合选择器。
                            .build())
                }
                refresh()
                onOpenThread(nt.id)
            } catch (e: Exception) {
                Toast.makeText(ctx, e.message ?: "分叉失败", Toast.LENGTH_LONG).show()
            }
        }
    }

    /** 新建分组;into 非空时把这条会话一并移进去(用户点的是"新建并移入")。 */
    fun createSection(name: String, into: PThread?) {
        if (sectionBusy) return
        sectionBusy = true
        scope.launch {
            try {
                val sec = withContext(Dispatchers.IO) {
                    AgentConnection.threadService().createThreadSection(
                        CreateThreadSectionRequest.newBuilder().setName(name.trim()).build())
                }
                sections = sections + sec
                sectionSheet = null
                sectionField = ""
                // 新分组的 chip 排在最后一个,不滚过去就是"创建成功但看不见"
                scrollChipsToEnd = true
                if (into != null) moveThreadToSection(into, sec.id)
            } catch (e: Exception) {
                Toast.makeText(ctx, e.message ?: "新建分组失败", Toast.LENGTH_LONG).show()
            } finally {
                sectionBusy = false
            }
        }
    }

    fun renameSection(id: String, name: String) {
        if (sectionBusy) return
        sectionBusy = true
        scope.launch {
            try {
                val sec = withContext(Dispatchers.IO) {
                    AgentConnection.threadService().renameThreadSection(
                        RenameThreadSectionRequest.newBuilder()
                            .setSectionId(id).setName(name.trim()).build())
                }
                // 【只换名字,别把计数一起换掉】RenameThreadSection 的应答里没有
                // thread_count(分组计数是 daemon 用本地库现算的),直接替换会让
                // chip 上的数字变成 0,改个名字看着像"这个组空了"。
                sections = sections.map {
                    if (it.id == id) sec.toBuilder().setThreadCount(it.threadCount).build()
                    else it
                }
                sectionSheet = null
                sectionField = ""
            } catch (e: Exception) {
                Toast.makeText(ctx, e.message ?: "重命名分组失败", Toast.LENGTH_LONG).show()
            } finally {
                sectionBusy = false
            }
        }
    }

    /** 删除分组。**只删分组本身**:组里的会话会变成未分组,不会被删掉。 */
    fun deleteSection(id: String) {
        if (sectionBusy) return
        sectionBusy = true
        scope.launch {
            try {
                withContext(Dispatchers.IO) {
                    AgentConnection.threadService().deleteThreadSection(
                        DeleteThreadSectionRequest.newBuilder().setSectionId(id).build())
                }
                sections = sections.filterNot { it.id == id }
                // 正在看的那个分组被删了就退回"全部",否则停在一个不存在的筛选上是空列表
                if (filter == id) filter = THREAD_FILTER_ALL
                sectionSheet = null
                sectionField = ""
                Toast.makeText(ctx, "已删除分组,组里的会话变成未分组", Toast.LENGTH_SHORT).show()
                refresh()
            } catch (e: Exception) {
                Toast.makeText(ctx, e.message ?: "删除分组失败", Toast.LENGTH_LONG).show()
            } finally {
                sectionBusy = false
            }
        }
    }

    val reconnectTick = AgentConnection.connectionEpoch.collectAsState().value
    // 【不把 filter 放进 key】threads 拿的是全量,筛选在渲染时做,所以切筛选是纯本地
    // 动作:不重新拉列表,也不会因为一次网络抖动让"刚点进去的分组"变空。
    LaunchedEffect(Unit, reconnectTick) { refresh() }

    // 【不再"一滚动就收起"】旧实现会在拖拽结束的那一帧把刚滑开的行收回去,
    // 看起来像"滑开了又自动弹回"。改为:纵向真正滚动超过一屏距离才收起(见 listState 监听无需)。
    // 左边缘右滑也能唤出侧边栏(问题 7)。挂在页面根节点上,子节点仍在命中链里,
    // 只有判定为"从左边缘起手的右滑"才会 consume,不会吃掉列表的点击。
    Box(
        modifier
            .fillMaxSize()
            .background(Pa.Bg)
            // 有行处于展开态时让位给"右滑收起动作":两者不再抢同一段手势
            .edgeSwipeToOpenDrawer(
            enabled = openKey.value == null,
            onDragFraction = drawerCtrl::onDrag,
            onRelease = drawerCtrl::onRelease,
        ),
    ) {
        Column(Modifier.fillMaxSize().statusBarsPadding()) {
            // ---- 大标题 + 工具按钮 ----
            Row(
                Modifier.fillMaxWidth().padding(start = 20.dp, end = 16.dp, top = 12.dp),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Row(verticalAlignment = Alignment.CenterVertically, modifier = Modifier.weight(1f)) {
                    IconActionButton(R.drawable.ic_lucide_menu, "菜单", { drawerCtrl.open() }, bg = Pa.Card)
                    Spacer(Modifier.width(12.dp))
                    Text("会话", fontSize = 30.sp, color = Pa.TextPrimary)
                }
                Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                    IconActionButton(R.drawable.ic_lucide_refresh_cw, "刷新", { refresh() }, bg = Pa.Card)
                    IconActionButton(R.drawable.ic_lucide_square_terminal, "终端", onOpenTerminal)
                    IconActionButton(R.drawable.ic_lucide_settings, "设置", onOpenSettings)
                }
            }

            // ---- 连接状态卡 ----
            ConnectionCard(
                host = host,
                onRetry = { retry() },
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
                    Text(
                        message, Modifier.weight(1f), fontSize = 12.sp,
                        color = if (offline) Pa.Yellow else Pa.Red, maxLines = 2,
                    )
                    TextButton(onClick = { retry() }, enabled = !retrying) {
                        Text(if (retrying) "进行中…" else "重试", color = Pa.Accent)
                    }
                }
            }

            // ---- 筛选 chips:全部 / 已归档 / 未分组 / 各个分组 ----
            // 【为什么并成一排】分组和归档一样,回答的都是"我要看哪一批会话" ——
            // 分两排会让人以为要先选档位再选分组。分组数量未知,所以这一排横向滚动。
            Row(
                Modifier.fillMaxWidth().padding(start = 16.dp, end = 16.dp, top = 2.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Row(
                    Modifier.weight(1f).horizontalScroll(chipScroll),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    FilterChip2("全部", filter == THREAD_FILTER_ALL) { filter = THREAD_FILTER_ALL }
                    Spacer(Modifier.width(8.dp))
                    FilterChip2("已归档", showArchived) { filter = THREAD_FILTER_ARCHIVED }
                    Spacer(Modifier.width(8.dp))
                    FilterChip2("未分组", filter == THREAD_FILTER_UNGROUPED) {
                        filter = THREAD_FILTER_UNGROUPED
                    }
                    sections.forEach { sec ->
                        Spacer(Modifier.width(8.dp))
                        SectionChip(
                            label = sec.name.ifBlank { "未命名分组" },
                            selected = filter == sec.id,
                            count = sec.threadCount,
                            // 点 = 看这个分组;长按 = 管这个分组(改名 / 删除)
                            onClick = { filter = sec.id },
                            onLongPress = {
                                // 预填当前名字:改名是"改"不是"重打",空输入框会逼用户
                                // 把整个名字重新敲一遍(还会因为记不全而改坏)。
                                sectionField = sec.name
                                sectionSheet = SectionSheet.Manage(sec.id, sec.name)
                            },
                        )
                    }
                    Spacer(Modifier.width(4.dp))
                }
                Spacer(Modifier.width(8.dp))
                // 【固定在右端,不参与横向滚动】分组一多,"新建分组"就会被滚出屏幕 ——
                // 而它恰恰是"分组多到需要管理"时更要用的那个入口。筛选 chips 自己滚,
                // 这个入口留在原地。
                NewSectionChip { sectionField = ""; sectionSheet = SectionSheet.Create() }
            }

            // 新建分组后滚到末尾:chip 加进来时 maxValue 还是旧的,所以 key 里带上
            // sections.size 让它在下一次测量之后再滚。
            LaunchedEffect(scrollChipsToEnd, sections.size) {
                if (scrollChipsToEnd) {
                    chipScroll.animateScrollTo(chipScroll.maxValue)
                    scrollChipsToEnd = false
                }
            }

            // 当前筛选的说明(分组名可能被横向滚出屏幕,这里给一句"你在看什么")
            val activeSectionName = sections.firstOrNull { it.id == filter }?.name
            // 【筛选指向了一个我们列不出来的分组】分两种情形,都不该让用户对着一个
            // 被筛过的列表发呆:①离线,分组列表整个读不到;②这个分组刚在电脑上被删。
            // 这时给出说明,并提示"点全部可退出"(点全部永远可用,不依赖网络)。
            val activeSectionMissing = filter.isNotEmpty() && activeSectionName == null &&
                filter != THREAD_FILTER_ARCHIVED && filter != THREAD_FILTER_UNGROUPED
            if (activeSectionName != null || filter == THREAD_FILTER_UNGROUPED ||
                activeSectionMissing
            ) {
                Text(
                    when {
                        activeSectionName != null ->
                            "只看分组「${activeSectionName.ifBlank { "未命名分组" }}」"
                        filter == THREAD_FILTER_UNGROUPED -> "只看未分组的会话"
                        else -> "只看某个分组(分组列表暂时读不到,点「全部」可退出)"
                    },
                    Modifier.padding(start = 18.dp, top = 6.dp),
                    fontSize = 11.5.sp, color = Pa.TextTertiary,
                )
            }

            // ---- 会话列表:本地会话 + 电脑上未导入的 codex 会话,合成一条流 ----
            // "活跃"和"电脑"本来就是同一批 codex 会话,这里不再分两个标签页;
            // 置顶项恒排最前,其余按更新时间倒序。
            val pinnedKeys = pinned
            val rows: List<SessionRow> = buildList {
                // 【筛选只在渲染这一步做】threads 是全量(见 refresh 的注释),
                // 归档/分组的判定全走同一套规则(matchesThreadFilter),
                // 不在别处再写一遍 if —— 两处判断迟早会不一致。
                threads
                    .filter {
                        matchesThreadFilter(
                            sectionId = it.sectionId,
                            archived = it.status == ThreadStatus.THREAD_STATUS_ARCHIVED,
                            filter = filter,
                        )
                    }
                    .forEach { add(SessionRow.Local(it, pinKey(it, null))) }
                if (filter == THREAD_FILTER_ALL) pcThreads.forEach { add(SessionRow.Pc(it)) }
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
                        iconRes = when {
                            showArchived -> R.drawable.ic_lucide_archive
                            activeSectionName != null -> R.drawable.ic_lucide_folder
                            else -> R.drawable.ic_lucide_message_square
                        },
                        title = when {
                            showArchived -> "没有归档的会话"
                            activeSectionName != null ->
                                "「${activeSectionName.ifBlank { "未命名分组" }}」还没有会话"
                            filter == THREAD_FILTER_UNGROUPED -> "没有未分组的会话"
                            else -> "还没有会话"
                        },
                        hint = when {
                            showArchived -> "左滑会话可恢复或删除"
                            activeSectionName != null || filter == THREAD_FILTER_UNGROUPED ->
                                "长按会话可把它移进分组"
                            else -> "点右下角 + 开始你的第一个任务"
                        },
                    )
                    }
                }
                else -> {
                    LazyColumn(
                        Modifier
                            .fillMaxSize()
                            // 点空白处收起已展开的行(列表条目之间的空隙与底部留白都算)
                            .pointerInput(Unit) {
                                detectTapGestures { if (openKey.value != null) openKey.value = null }
                            },
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
                                        // 长按 = 会话组织(移动到分组 / 分叉)。
                                        // 【为什么是长按而不是第 5 个左滑动作】动作区是 64dp × 动作数,
                                        // 5 个就 320dp,超过 SwipeRevealRow 的 72% 上限(窄屏会被裁掉),
                                        // 而且左滑本身已经承载了最高频的四个动作。长按这个位置
                                        // 本来就是为"上下文菜单"留的(见文件下方注释)。
                                        onLongPress = { organizeTarget = t },
                                        // 已展开时点内容 = 收起,未展开才进会话
                                        onClick = {
                                            // 任何一行处于展开态时,点击(包括点别的行)先收起 —— iOS 式行为,
                                            // 避免"想收起却误进会话"
                                            if (openKey.value != null) openKey.value = null
                                            else onOpenThread(t.id)
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
                                            if (openKey.value != null) {
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

    // 长按菜单:重命名 / 归档 / 删除 仍然全部收敛在左滑动作里(一条交互路径比
    // "长按弹一层、再点一次"更连贯)。长按只承载**左滑放不下**的动作 —— 分组有
    // 任意多个、"移动到哪个分组"天然需要一个列表,塞进左滑动作区会超过 72% 宽度上限。
    organizeTarget?.let { t ->
        ModalBottomSheet(
            onDismissRequest = { organizeTarget = null },
            containerColor = Pa.CardHigh,
            shape = RoundedCornerShape(topStart = 26.dp, topEnd = 26.dp),
        ) {
            Column(Modifier.fillMaxWidth().padding(bottom = 26.dp)) {
                Text(
                    t.title.ifBlank { "未命名会话" },
                    Modifier.fillMaxWidth().padding(horizontal = 24.dp),
                    fontSize = 17.sp, color = Pa.TextPrimary,
                    maxLines = 1, overflow = TextOverflow.Ellipsis,
                )
                Text(
                    "移动到分组",
                    Modifier.padding(start = 24.dp, top = 16.dp, bottom = 2.dp),
                    fontSize = 11.5.sp, color = Pa.TextTertiary,
                )
                // 当前所在分组打上「当前」并在文案里标明 —— 用户点开就能确认"这条现在在哪"
                SheetAction(
                    R.drawable.ic_lucide_folder,
                    if (t.sectionId.isEmpty()) "未分组（当前）" else "未分组",
                    tint = if (t.sectionId.isEmpty()) Pa.Accent else Pa.TextPrimary,
                ) { moveThreadToSection(t, "") }
                sections.forEach { sec ->
                    val here = sec.id == t.sectionId
                    SheetAction(
                        R.drawable.ic_lucide_folder,
                        sec.name.ifBlank { "未命名分组" } + if (here) "（当前）" else "",
                        tint = if (here) Pa.Accent else Pa.TextPrimary,
                    ) { moveThreadToSection(t, sec.id) }
                }
                SheetAction(R.drawable.ic_lucide_folder_plus, "新建分组并移入…", Pa.Accent) {
                    sectionField = ""
                    sectionSheet = SectionSheet.Create(moveInto = t)
                }
                HorizontalDivider(Modifier.padding(vertical = 8.dp), color = Pa.Separator)
                Text(
                    "分叉",
                    Modifier.padding(start = 24.dp, bottom = 2.dp),
                    fontSize = 11.5.sp, color = Pa.TextTertiary,
                )
                SheetAction(R.drawable.ic_lucide_git_branch, "分叉会话") { forkThread(t) }
                Text(
                    forkConsequence(),
                    Modifier.padding(start = 24.dp, end = 24.dp, top = 2.dp),
                    fontSize = 11.5.sp, color = Pa.TextTertiary,
                )
            }
        }
    }

    // ---- 分组管理面板(顶部「新建分组」chip / 长按分组 chip) ----
    sectionSheet?.let { sheet ->
        SectionManagerSheet(
            sheet = sheet,
            field = sectionField,
            onFieldChange = { sectionField = it },
            busy = sectionBusy,
            onDismiss = { sectionSheet = null; sectionField = "" },
            onSubmit = { name ->
                when (sheet) {
                    is SectionSheet.Create -> createSection(name, sheet.moveInto)
                    is SectionSheet.Manage -> renameSection(sheet.id, name)
                }
            },
            onDelete = { (sheet as? SectionSheet.Manage)?.let { deleteSection(it.id) } },
        )
    }

    // 重命名已改为「行内编辑」(见 ThreadCard 的 renaming 分支),不再使用对话框 —— 设计规范 §3.1

    if (drawerCtrl.visible) {
        AppDrawer(
            currentTab = currentTab,
            hostTitle = host?.displayName ?: "未连接电脑",
            progress = drawerCtrl.progress,
            onClose = { drawerCtrl.close() },
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
                    Modifier.weight(1f),
                    fontSize = 15.sp,
                    color = Pa.TextPrimary,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
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
                    // 带上一级目录(盘符),否则 G:\试验区 只显示"试验区"看不出在哪个盘
                    "  ·  " + shortWorkspace(t.workspace),
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
            Modifier.fillMaxWidth(),
            fontSize = 15.sp,
            color = Pa.TextPrimary,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
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
                    "  ·  " + shortWorkspace(info.cwd),
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

/**
 * 分组管理面板的形态。
 *
 * @property Create.moveInto 非空表示"从某个会话上发起的建组" —— 建完要把这条会话
 *   一并移进去。用对象本身携带这个意图,而不是再加一个"当前操作对象"的旁路状态:
 *   面板关了意图就没了,不会在下一次建组时冒出来。
 */
private sealed interface SectionSheet {
    data class Create(val moveInto: PThread? = null) : SectionSheet
    data class Manage(val id: String, val name: String) : SectionSheet
}

/** 分组 chip:点筛选、长按管理。带一个会话数,让"这个组是不是空的"一眼可见。 */
@Composable
private fun SectionChip(
    label: String,
    selected: Boolean,
    count: Int,
    onClick: () -> Unit,
    onLongPress: () -> Unit,
) {
    Row(
        Modifier
            // 长按与点击共存用 pressableLong(内部是 combinedClickable)——
            // 手写两个 pointerInput 叠在一起时,内层会把 down 消费掉,外层就再也收不到点击。
            .pressableLong(onClick = onClick, onLongPress = onLongPress)
            .background(if (selected) Pa.AccentDim else Pa.Card, RoundedCornerShape(50))
            .padding(horizontal = 13.dp, vertical = 7.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Icon(
            painterResource(R.drawable.ic_lucide_folder), null,
            tint = if (selected) Pa.Accent else Pa.TextSecondary,
            modifier = Modifier.size(13.dp),
        )
        Spacer(Modifier.width(6.dp))
        Text(
            label, fontSize = 13.sp,
            color = if (selected) Pa.Accent else Pa.TextSecondary,
            maxLines = 1,
        )
        if (count > 0) {
            Spacer(Modifier.width(6.dp))
            Text(
                "$count", fontSize = 11.sp,
                color = if (selected) Pa.AccentText else Pa.TextTertiary,
            )
        }
    }
}

/** 「新建分组」chip:与筛选 chip 同一排、同一形状,避免多一个常驻工具按钮。 */
@Composable
private fun NewSectionChip(onClick: () -> Unit) {
    Row(
        Modifier
            .pressable { onClick() }
            .background(Pa.Card, RoundedCornerShape(50))
            .padding(horizontal = 13.dp, vertical = 7.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Icon(
            painterResource(R.drawable.ic_lucide_folder_plus), null,
            tint = Pa.TextSecondary, modifier = Modifier.size(13.dp),
        )
        Spacer(Modifier.width(6.dp))
        Text("新建分组", fontSize = 13.sp, color = Pa.TextSecondary)
    }
}

/**
 * 新建 / 改名 / 删除分组。
 *
 * 【删除为什么要二次确认且必须解释】codex 删分组**只删分组**,组里的会话会变成
 * 未分组。用户凭直觉会以为"删组 = 删里面的会话",不解释清楚就没人敢点;
 * 只说"确认删除?"又等于什么都没说。所以确认页直接把这句后果写在按钮上方。
 */
@Composable
private fun SectionManagerSheet(
    sheet: SectionSheet,
    field: String,
    onFieldChange: (String) -> Unit,
    busy: Boolean,
    onDismiss: () -> Unit,
    onSubmit: (String) -> Unit,
    onDelete: () -> Unit,
) {
    // 换一个面板(改名→新建)时确认态要复位,否则会带着上一份的"确认删除"进来
    var confirmDelete by remember(sheet) { mutableStateOf(false) }
    val manage = sheet as? SectionSheet.Manage

    ModalBottomSheet(
        onDismissRequest = onDismiss,
        containerColor = Pa.CardHigh,
        shape = RoundedCornerShape(topStart = 26.dp, topEnd = 26.dp),
    ) {
        Column(Modifier.fillMaxWidth().padding(horizontal = 20.dp).padding(bottom = 26.dp)) {
            Text(
                if (manage == null) "新建分组" else "分组设置",
                fontSize = 17.sp, color = Pa.TextPrimary,
            )
            if (manage != null) {
                Text(
                    manage.name.ifBlank { "未命名分组" },
                    Modifier.padding(top = 2.dp),
                    fontSize = 12.sp, color = Pa.TextSecondary,
                    maxLines = 1, overflow = TextOverflow.Ellipsis,
                )
            }

            if (confirmDelete) {
                Text(
                    "删除分组「${manage?.name?.ifBlank { "未命名分组" }.orEmpty()}」？\n" +
                        "组里的会话不会被删除,只会变成未分组。",
                    Modifier.padding(top = 14.dp),
                    fontSize = 14.sp, color = Pa.TextPrimary,
                )
                Row(
                    Modifier.fillMaxWidth().padding(top = 16.dp),
                    horizontalArrangement = Arrangement.End,
                ) {
                    TextButton(onClick = { confirmDelete = false }) { Text("取消") }
                    Spacer(Modifier.width(8.dp))
                    Button(
                        onClick = { confirmDelete = false; onDelete() },
                        colors = androidx.compose.material3.ButtonDefaults.buttonColors(
                            containerColor = Pa.Red,
                        ),
                        shape = RoundedCornerShape(14.dp),
                    ) { Text("确认删除", color = Color.White) }
                }
            } else {
                OutlinedTextField(
                    value = field,
                    onValueChange = onFieldChange,
                    modifier = Modifier.fillMaxWidth().padding(top = 14.dp),
                    singleLine = true,
                    placeholder = { Text("分组名", color = Pa.TextTertiary) },
                    keyboardOptions = KeyboardOptions(imeAction = ImeAction.Done),
                    keyboardActions = KeyboardActions(
                        onDone = { if (field.isNotBlank() && !busy) onSubmit(field) },
                    ),
                )
                Row(
                    Modifier.fillMaxWidth().padding(top = 14.dp),
                    horizontalArrangement = Arrangement.End,
                ) {
                    TextButton(onClick = onDismiss, enabled = !busy) { Text("取消") }
                    Spacer(Modifier.width(8.dp))
                    Button(
                        enabled = field.isNotBlank() && !busy,
                        onClick = { onSubmit(field) },
                        colors = androidx.compose.material3.ButtonDefaults.buttonColors(
                            containerColor = Pa.Accent,
                        ),
                        shape = RoundedCornerShape(14.dp),
                    ) {
                        Text(
                            if (manage == null) "创建" else "保存",
                            color = if (field.isNotBlank() && !busy) Color(0xFF0B0B0D)
                            else Pa.TextTertiary,
                        )
                    }
                }
                if (manage != null) {
                    Spacer(Modifier.height(8.dp))
                    HorizontalDivider(color = Pa.Separator)
                    SheetAction(
                        R.drawable.ic_lucide_trash_2, "删除分组", Pa.Red,
                    ) { confirmDelete = true }
                }
            }
        }
    }
}
