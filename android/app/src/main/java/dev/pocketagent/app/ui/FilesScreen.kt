package dev.pocketagent.app.ui

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.material3.Card
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.runtime.snapshotFlow
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clipToBounds
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.platform.LocalView
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.AgentConnection
import dev.pocketagent.app.R as AppR
import dev.pocketagent.app.ui.anim.EmptyState
import dev.pocketagent.app.ui.anim.IconActionButton
import dev.pocketagent.app.ui.anim.edgeSwipeToOpenDrawer
import dev.pocketagent.app.ui.component.PaDownloader
import dev.pocketagent.app.ui.component.PaHaptics
import dev.pocketagent.app.ui.component.PaNavIconButton
import dev.pocketagent.app.ui.component.PaSkeletonList
import dev.pocketagent.app.ui.component.PreviewMode
import dev.pocketagent.app.ui.component.SkeletonKind
import dev.pocketagent.app.ui.component.SwipeAction
import dev.pocketagent.app.ui.component.SwipeRevealRow
import dev.pocketagent.app.ui.component.UndoBarHost
import dev.pocketagent.app.ui.component.parentDirectoryOf
import dev.pocketagent.app.ui.component.pathBreadcrumbs
import dev.pocketagent.app.ui.component.rememberUndoState
import dev.pocketagent.app.ui.theme.Pa
import dev.pocketagent.app.ui.theme.PaEditorial
import dev.pocketagent.app.ui.theme.EditorialTone
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import pocketagent.v1.CreateDirectoryRequest
import pocketagent.v1.DeleteEntryRequest
import pocketagent.v1.MoveEntryRequest
import pocketagent.v1.WriteFileRequest
import pocketagent.v1.ListDirectoryRequest
import pocketagent.v1.ListRootsRequest

/**
 * 文件浏览器 —— 可访问整台电脑:先"此电脑"盘符列表,再逐级进入。
 * 目录在前排序;文件点开为可滚动文本预览(≤256KB)。
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun FilesScreen(
    initialPath: String?,
    currentTab: Int = 2,
    onOpenTerminal: () -> Unit = {},
    onOpenSettings: () -> Unit = {},
    onOpenTaskCenter: () -> Unit = {},
    onOpenBrowser: (String) -> Unit = {},
    /** 返回层级:二级页面按返回 → 回主页并展开侧边栏。 */
    onBackToHome: () -> Unit = {},
    onGoTab: (Int) -> Unit = {},
) {
    // 编辑式浅色 M3 色板只包本页子树(M3 组件自动落编辑式色,深色显示模式不漏深)
    androidx.compose.material3.MaterialTheme(colorScheme = editorialM3Scheme()) {
        EditorialSystemBars()
        FilesScreenBody(
            initialPath, currentTab, onOpenTerminal, onOpenSettings,
            onOpenTaskCenter, onOpenBrowser, onBackToHome, onGoTab,
        )
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun FilesScreenBody(
    initialPath: String?,
    currentTab: Int = 2,
    onOpenTerminal: () -> Unit = {},
    onOpenSettings: () -> Unit = {},
    onOpenTaskCenter: () -> Unit = {},
    onOpenBrowser: (String) -> Unit = {},
    onBackToHome: () -> Unit = {},
    onGoTab: (Int) -> Unit = {},
) {
    var showDrawer by remember { mutableStateOf(false) }
    // 侧边栏跟手:与主页/会话页同一套 controller
    val drawerCtrl = dev.pocketagent.app.ui.component.rememberDrawerController()
    val scope = rememberCoroutineScope()
    val view = LocalView.current
    var path by remember { mutableStateOf(initialPath ?: "") }
    var showRoots by remember { mutableStateOf(initialPath.isNullOrEmpty()) }
    var entries by remember { mutableStateOf(listOf<FileRowModel>()) }
    // 【文件搜索】只对"当前目录已加载的条目"按名字做不区分大小写的过滤。
    // 跨目录递归搜索需要 daemon 侧的接口(现在的 SearchContent 是**内容**搜索,
    // 不是文件名搜索),所以界面上会明确写出搜索范围 —— 不做"看起来能搜全部、
    // 其实只搜一层"的假承诺。
    var searchOpen by remember { mutableStateOf(false) }
    var searchQuery by remember { mutableStateOf("") }
    var loading by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }
    var showCreate by remember { mutableStateOf(false) } // 新建文件/文件夹
    // 不能内置预览的文件(旧版 Office / 压缩包 / 音视频):点它时给一个"下载到手机"的引导
    var askDownload by remember { mutableStateOf<FileRowModel?>(null) }
    // 【文件管理】沿用会话列表的左滑范式:同一时刻最多一行展开
    val openKey = remember { mutableStateOf<Any?>(null) }
    val listState = rememberLazyListState()
    val undo = rememberUndoState()
    var renaming by remember { mutableStateOf<FileRowModel?>(null) }
    var renameText by remember { mutableStateOf("") }
    var pendingDelete by remember { mutableStateOf<FileRowModel?>(null) }
    val ctx = androidx.compose.ui.platform.LocalContext.current
    // 面包屑:分段规则在 component/PathNav.kt(与文件夹选择器同一份坐标纪律)
    val crumbs = remember(path) { pathBreadcrumbs(path) }
    val crumbScroll = rememberScrollState()
    // 路径越深面包屑越长:每次换目录把末端(当前层)滚进视野
    LaunchedEffect(path) {
        snapshotFlow { crumbScroll.maxValue }.collect { max ->
            if (max > 0) crumbScroll.animateScrollTo(max)
        }
    }

    fun loadDir(p: String) {
        loading = true
        scope.launch {
            try {
                entries = withContext(Dispatchers.IO) {
                    AgentConnection.fileService()
                        .listDirectory(ListDirectoryRequest.newBuilder().setPath(p).build())
                        .entriesList
                }.map { it.toRowModel() }.let { sortFileRows(it) }
                path = p
                showRoots = false
                error = null
            } catch (e: Exception) {
                error = "${e.message}"
            } finally {
                loading = false
            }
        }
    }

    /** 删除(目录递归删)。撤销时按原名重建 —— 目录只能重建空壳,
     *  撤销条文案里如实说明,不做假的"已完整恢复"。 */
    fun deleteEntry(e: FileRowModel) {
        val parent = path
        scope.launch {
            try {
                withContext(Dispatchers.IO) {
                    AgentConnection.fileService().deleteEntry(
                        DeleteEntryRequest.newBuilder()
                            .setPath(e.path)
                            .setRecursive(e.isDir)
                            .build(),
                    )
                }
                openKey.value = null
                PaHaptics.warning(view)
                loadDir(parent)
                undo.show(
                    message = if (e.isDir) {
                        "已删除电脑上的文件夹 ${middleEllipsize(e.name, 20)}(撤销只重建空目录)"
                    } else {
                        "已删除电脑上的 ${middleEllipsize(e.name, 20)}(撤销恢复为空文件)"
                    },
                    onUndo = {
                        withContext(Dispatchers.IO) {
                            if (e.isDir) {
                                AgentConnection.fileService().createDirectory(
                                    CreateDirectoryRequest.newBuilder().setPath(e.path).build())
                            } else {
                                AgentConnection.fileService().writeFile(
                                    WriteFileRequest.newBuilder()
                                        .setPath(e.path)
                                        .setData(com.google.protobuf.ByteString.EMPTY)
                                        .setCreate(true).setTruncate(true).build())
                            }
                        }
                        loadDir(parent)
                    },
                )
            } catch (ex: Exception) {
                error = "删除失败:${ex.message}"
            }
        }
    }

    /** 重命名:daemon 没有独立 Rename,用 MoveEntry(同目录 old→new)实现。 */
    fun renameEntry(e: FileRowModel, newName: String) {
        val sep = if (e.path.contains('/') && !e.path.contains('\\')) "/" else "\\"
        val parent = e.path.substringBeforeLast(sep, "")
        val target = if (parent.isEmpty()) newName else parent + sep + newName
        scope.launch {
            try {
                withContext(Dispatchers.IO) {
                    AgentConnection.fileService().moveEntry(
                        MoveEntryRequest.newBuilder()
                            .setFromPath(e.path).setToPath(target).build())
                }
                openKey.value = null
                PaHaptics.confirm(view)
                loadDir(path)
            } catch (ex: Exception) {
                error = "重命名失败:${ex.message}"
            }
        }
    }

    fun loadRoots(preserveError: String? = null) {
        loading = true
        scope.launch {
            try {
                entries = withContext(Dispatchers.IO) {
                    AgentConnection.fileService()
                        .listRoots(ListRootsRequest.newBuilder().build())
                        .rootsList
                }.map { it.toRowModel() }
                path = ""
                showRoots = true
                error = preserveError
            } catch (e: Exception) {
                error = "${e.message}"
            } finally {
                loading = false
            }
        }
    }

    /**
     * 【返回层级 · 第一级】目录树里往上退一层。
     *  C:\dev\x -> C:\dev;C:\dev -> C:\(盘根仍是目录);C:\ -> 此电脑(盘符列表)。
     * 顶栏的"上级"按钮与系统返回键共用这一份逻辑,避免两处规则漂移。
     *  @return true = 本次返回已被消费(停在本页);false = 已经在最顶层(此电脑),
     *          交给下一级(回到主板块)处理。
     */
    fun goUpOneLevel(): Boolean {
        if (showRoots) return false
        // 坐标规则收敛到 component/PathNav.kt,与文件夹选择器共用同一份,
        // 避免两处实现再次漂移(历史上两处都踩过"原地重载"的坑)。
        val up = parentDirectoryOf(path)
        if (up != null) loadDir(up) else loadRoots()
        return true
    }

    // 【返回层级 · 分级回退】抽屉 → 当前目录的上级 → 主板块(主页并展开侧边栏)。
    // 之前只有两级(抽屉/主页),进了多层目录按返回会直接跳回主页,把浏览位置丢掉。
    BackHandler(enabled = true) {
        when {
            drawerCtrl.visible -> drawerCtrl.close()
            goUpOneLevel() -> Unit
            else -> onBackToHome()
        }
    }

    /** 在当前目录新建:文件夹走 CreateDirectory,空文件走 WriteFile。 */
    fun createEntry(name: String, isDir: Boolean) {
        val sep = if (path.contains('\\')) "\\" else "/"
        val target = path.trimEnd('\\', '/') + sep + name
        scope.launch {
            try {
                withContext(Dispatchers.IO) {
                    if (isDir) {
                        AgentConnection.fileService().createDirectory(
                            CreateDirectoryRequest.newBuilder().setPath(target).build())
                    } else {
                        AgentConnection.fileService().writeFile(
                            WriteFileRequest.newBuilder()
                                .setPath(target).setData(com.google.protobuf.ByteString.EMPTY)
                                .setCreate(true).setTruncate(true).build())
                    }
                }
                PaHaptics.confirm(view)
                loadDir(path)
            } catch (e: Exception) {
                error = "新建失败:${e.message}"
            }
        }
    }

    LaunchedEffect(Unit) {
        if (initialPath.isNullOrEmpty()) {
            // 全局文件入口从“此电脑”开始；会话 workspace 只作为显式快捷入口。
            loadRoots()
        } else {
            loadDir(initialPath)
        }
    }

    // 【搜索结果】空查询时原样返回,不做任何额外拷贝。
    val visibleEntries = remember(entries, searchQuery) {
        val q = searchQuery.trim()
        if (q.isEmpty()) entries
        else entries.filter { it.name.contains(q, ignoreCase = true) }
    }

    Scaffold(
        containerColor = PaEditorial.Canvas,
        // 左边缘右滑也能唤出侧边栏(问题 7)
        modifier = Modifier.edgeSwipeToOpenDrawer(
                        onDragFraction = drawerCtrl::onDrag,
                        onRelease = drawerCtrl::onRelease,
                    ),
        topBar = {
            Row(
                Modifier
                    .fillMaxWidth()
                    .statusBarsPadding()
                    .padding(start = PaEditorial.Space.NavGutter, end = 14.dp, top = 8.dp, bottom = 8.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                PaNavIconButton(
                    onClick = { drawerCtrl.open() },
                    contentDescription = "菜单",
                    iconRes = AppR.drawable.ic_lucide_menu,
                )
                if (!showRoots) {
                    PaNavIconButton(
                        onClick = { goUpOneLevel() },
                        contentDescription = "上级",
                        iconRes = AppR.drawable.ic_lucide_chevron_left,
                        plain = true,
                    )
                }
                Text(
                    when {
                        showRoots -> "此电脑"
                        else -> path.substringAfterLast('\\').substringAfterLast('/').ifEmpty { path }
                    },
                    Modifier.weight(1f),
                    style = PaEditorial.Type.Title,
                    color = PaEditorial.Ink,
                    maxLines = 1,
                )
                if (loading) {
                    CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp, color = PaEditorial.Primary)
                    Spacer(Modifier.width(12.dp))
                }
                PaNavIconButton(
                    onClick = { loadRoots() },
                    contentDescription = "此电脑",
                    iconRes = AppR.drawable.ic_lucide_monitor,
                )
                Spacer(Modifier.width(8.dp))
                if (!showRoots) {
                    // 【文件搜索】按名字过滤当前目录内容。放在这里而不是塞进抽屉,
                    // 是因为"在眼前这堆文件里找某个"才是高频场景。
                    PaNavIconButton(
                        onClick = {
                            searchOpen = !searchOpen
                            if (!searchOpen) searchQuery = ""
                        },
                        contentDescription = if (searchOpen) "关闭搜索" else "搜索",
                        iconRes = if (searchOpen) AppR.drawable.ic_lucide_x else AppR.drawable.ic_lucide_search,
                        active = searchOpen,
                    )
                    Spacer(Modifier.width(8.dp))
                    PaNavIconButton(
                        onClick = { showCreate = true },
                        contentDescription = "新建",
                        iconRes = AppR.drawable.ic_lucide_plus,
                    )
                    Spacer(Modifier.width(8.dp))
                }
                PaNavIconButton(
                    onClick = { if (showRoots) loadRoots() else loadDir(path) },
                    contentDescription = "刷新",
                    iconRes = AppR.drawable.ic_lucide_refresh_cw,
                )
            }
        },
    ) { pad ->
        // 【为什么要 clipToBounds】撤销条入场动画是从内容区上方(-自身高度)滑下来的。
        // 宿主 Box 若不加裁剪,动画头几帧会画到顶栏之上 —— 真机上就是"标题栏上闪过
        // 一层残影"。裁掉越界部分,动画只在内容区内可见。
        Box(Modifier.padding(pad).fillMaxSize().clipToBounds()) {
        Column(Modifier.fillMaxSize()) {
            if (!showRoots) {
                // 【可点级面包屑】首段"此电脑"回盘符列表,中间任一段直达那一层;
                // 末段(当前层)不可点。横向可滚,换目录自动滚到末端。
                Row(
                    Modifier
                        .fillMaxWidth()
                        .horizontalScroll(crumbScroll)
                        .padding(horizontal = PaEditorial.Space.PageMargin, vertical = 2.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    crumbs.forEachIndexed { i, crumb ->
                        if (i > 0) {
                            Icon(
                                painterResource(AppR.drawable.ic_lucide_chevron_right), null,
                                tint = PaEditorial.Subtle, modifier = Modifier.size(12.dp),
                            )
                        }
                        val isCurrent = i == crumbs.lastIndex
                        Text(
                            middleEllipsize(crumb.label, 20),
                            Modifier
                                .then(
                                    if (!isCurrent) {
                                        Modifier.clickable {
                                            when {
                                                crumb.target.isEmpty() -> loadRoots()
                                                else -> loadDir(crumb.target)
                                            }
                                        }
                                    } else {
                                        Modifier
                                    },
                                )
                                .padding(horizontal = 4.dp, vertical = 14.dp),
                            fontSize = 11.sp,
                            lineHeight = 16.sp,
                            fontFamily = FontFamily.Monospace,
                            fontWeight = if (isCurrent) FontWeight.SemiBold else FontWeight.Normal,
                            color = if (isCurrent) PaEditorial.Ink else PaEditorial.Primary,
                            maxLines = 1,
                        )
                    }
                }
            }
            error?.let {
                Row(
                    Modifier
                        .fillMaxWidth()
                        .padding(horizontal = PaEditorial.Space.PageMargin, vertical = 6.dp)
                        .background(PaEditorial.DeniedContainer, RoundedCornerShape(13.dp))
                        .padding(horizontal = 12.dp, vertical = 4.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(
                        it,
                        Modifier.weight(1f),
                        color = EditorialTone.DENIED.text, fontSize = 13.sp, lineHeight = 18.sp,
                    )
                    // 断线/目录打不开时给下一步:原地重试(顶层重拉盘符,目录层重进)
                    TextButton(onClick = { if (showRoots) loadRoots() else loadDir(path) }) {
                        Text("重试", fontSize = 12.sp, color = EditorialTone.DENIED.text)
                    }
                }
            }
            // ---- 搜索框(仅非"此电脑"层显示:盘符列表没有搜索的意义) ----
            if (searchOpen && !showRoots) {
                Row(
                    Modifier
                        .fillMaxWidth()
                        .padding(horizontal = PaEditorial.Space.PageMargin, vertical = 2.dp)
                        .background(PaEditorial.Surface, RoundedCornerShape(13.dp))
                        .border(1.dp, PaEditorial.Hairline, RoundedCornerShape(13.dp))
                        .padding(horizontal = 12.dp, vertical = 2.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Icon(
                        painterResource(AppR.drawable.ic_lucide_search), null,
                        tint = PaEditorial.Subtle, modifier = Modifier.size(14.dp),
                    )
                    Spacer(Modifier.width(8.dp))
                    BasicTextField(
                        value = searchQuery,
                        onValueChange = { searchQuery = it },
                        modifier = Modifier.weight(1f).padding(vertical = 10.dp),
                        singleLine = true,
                        textStyle = TextStyle(
                            fontSize = 13.5.sp, color = PaEditorial.Ink,
                        ),
                        cursorBrush = SolidColor(PaEditorial.Primary),
                        decorationBox = { inner ->
                            Box {
                                if (searchQuery.isEmpty()) {
                                    Text(
                                        "在当前目录中搜索文件名…",
                                        fontSize = 13.5.sp, color = PaEditorial.Muted,
                                    )
                                }
                                inner()
                            }
                        },
                    )
                    if (searchQuery.isNotEmpty()) {
                        Text(
                            "${visibleEntries.size} 项",
                            fontSize = 10.5.sp, color = PaEditorial.Muted,
                        )
                    }
                }
                // 说明搜索范围:避免用户以为"搜不到 = 文件不存在"
                Text(
                    "范围：当前目录（不含子目录），按文件名匹配，不区分大小写",
                    Modifier.padding(start = 18.dp, top = 4.dp),
                    fontSize = 10.sp,
                    color = PaEditorial.Muted,
                )
            }
            // 【修复】weight(1f) 让 LazyColumn 占满剩余高度并启用懒加载滚动;
            // 之前没加 weight 导致列表不滚动、看不到全部
            // 分组标题(09:「工作区文件 · N 项」):条目数右对齐
            Row(
                Modifier
                    .fillMaxWidth()
                    .padding(start = PaEditorial.Space.PageMargin, end = PaEditorial.Space.PageMargin, top = 10.dp, bottom = 2.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(
                    if (showRoots) "此电脑" else "工作区文件",
                    style = PaEditorial.Type.Heading,
                    color = PaEditorial.Ink,
                )
                Spacer(Modifier.weight(1f))
                Text(
                    visibleEntries.size.toString() + " 项",
                    fontSize = 12.sp, lineHeight = 17.sp,
                    color = PaEditorial.Muted,
                )
            }
            if (loading && visibleEntries.isEmpty()) {
                // 首次加载:骨架占位,不白屏(轮廓与真实行对位:42dp 图标格 + 文本条)
                PaSkeletonList(
                    SkeletonKind.FILE_ROW,
                    Modifier.weight(1f).padding(horizontal = PaEditorial.Space.PageMargin, vertical = 10.dp),
                    count = 7,
                )
            } else {
            LazyColumn(
                Modifier.weight(1f).padding(horizontal = PaEditorial.Space.PageMargin),
                state = listState,
                verticalArrangement = androidx.compose.foundation.layout.Arrangement.spacedBy(8.dp),
            ) {
                if (visibleEntries.isEmpty() && searchQuery.isNotBlank()) {
                    item(key = "__no_match__") {
                        Text(
                            "当前目录没有匹配「$searchQuery」的文件",
                            Modifier.fillMaxWidth().padding(vertical = 24.dp),
                            fontSize = 12.5.sp,
                            color = PaEditorial.Muted,
                        )
                    }
                }
                if (visibleEntries.isEmpty() && searchQuery.isBlank() && !loading) {
                    // 空态给真实说明 + 下一步,不白屏也不装死
                    item(key = "__empty__") {
                        when {
                            showRoots -> EmptyState(
                                AppR.drawable.ic_lucide_monitor,
                                "没有读到任何盘符",
                                "确认电脑端 daemon 正在运行,再点右上角「刷新」",
                            )
                            else -> EmptyState(
                                AppR.drawable.ic_lucide_folder_plus,
                                "这个文件夹是空的",
                                "点右上角「+」新建文件或文件夹;若电脑上应有内容,点「刷新」",
                            )
                        }
                    }
                }
                items(visibleEntries, key = { it.path }) { e ->
                    // 【文件管理】左滑出「重命名 / 删除」,与会话列表同一套交互。
                    // 行内容里不能挂 clickable(容器独占手势),所以点击走 onClick 参数。
                    SwipeRevealRow(
                        rowKey = e.path,
                        openKey = openKey,
                        shape = RoundedCornerShape(18.dp),
                        rowBackground = PaEditorial.Surface,
                        actions = listOf(
                            SwipeAction(
                                "rename", "重命名", AppR.drawable.ic_lucide_pencil,
                                PaEditorial.Ink,
                            ),
                            SwipeAction(
                                "delete", "删除", AppR.drawable.ic_lucide_trash_2,
                                androidx.compose.ui.graphics.Color.White, destructive = true,
                            ),
                        ),
                        onAction = { a ->
                            when (a.id) {
                                "rename" -> {
                                    renameText = e.name
                                    renaming = e
                                }
                                "delete" -> pendingDelete = e
                            }
                        },
                        onClick = {
                            when {
                                e.isDir -> loadDir(e.path)
                                // 【能力矩阵分派】能内置渲染的一律交给内置浏览器 ——
                                // Office / PDF / Markdown / 图片 / 网页 都能排版展示,
                                // 比在这个弹层里塞纯文本好得多。
                                e.kind.preview == PreviewMode.IN_APP -> onOpenBrowser(
                                    // 【带上目录条目的 size/mtime】内置浏览器据此生成预览缓存键 ——
                                    // 文件在电脑上被改过 → mtime 变 → 键变 → 自动重拉,不会看到旧内容。
                                    "pc:" + e.path + "?_pas=" + e.sizeBytes + "&_pam=" + e.modifiedUnixMs,
                                )
                                // 内置不了(旧版 Office / 压缩包 / 音视频)才引导下载
                                else -> askDownload = e
                            }
                        },
                    ) {
                    Row(
                        Modifier
                            .fillMaxWidth()
                            .padding(start = 12.dp, end = 4.dp, top = 12.dp, bottom = 12.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        // 行首 42dp 图标方格(09:文件夹绿格 / 文件蓝格);
                        // 图标按 FileKind 分型(代码/表格/图片/压缩包/音视频…一眼可分)
                        Box(
                            Modifier
                                .size(42.dp)
                                .background(
                                    if (e.isDir) PaEditorial.RunningContainer else PaEditorial.PrimaryContainer,
                                    RoundedCornerShape(13.dp),
                                ),
                            contentAlignment = Alignment.Center,
                        ) {
                            Icon(
                                painterResource(fileRowIcon(e.isDir, e.kind)),
                                null,
                                tint = if (e.isDir) PaEditorial.Running else PaEditorial.Primary,
                                modifier = Modifier.size(20.dp),
                            )
                        }
                        Spacer(Modifier.width(12.dp))
                        Column(Modifier.weight(1f)) {
                            Text(
                                // 长文件名中间省略,保住扩展名(FilenameDisplay 规则)
                                middleEllipsize(e.name, 26),
                                fontSize = 15.sp, lineHeight = 21.sp,
                                fontWeight = FontWeight.SemiBold,
                                color = PaEditorial.Ink,
                                maxLines = 1, overflow = androidx.compose.ui.text.style.TextOverflow.Ellipsis,
                            )
                            val meta = entryMetaLine(e)
                            if (meta.isNotEmpty()) {
                                Text(
                                    meta,
                                    fontSize = 11.sp, lineHeight = 16.sp, color = PaEditorial.Muted, maxLines = 1,
                                )
                            }
                        }
                        if (!e.isDir) {
                            // 每一行都能单独下载到手机(不必先进浏览器);触达区 ≥44dp
                            IconActionButton(
                                AppR.drawable.ic_lucide_download, "下载到手机",
                                {
                                    PaDownloader.downloadFromPc(e.path)
                                    PaHaptics.light(view)
                                    android.widget.Toast.makeText(
                                        ctx, "已开始下载 " + e.name, android.widget.Toast.LENGTH_SHORT,
                                    ).show()
                                },
                                bg = Color.Transparent, size = 44.dp, iconSize = 18.dp,
                                tint = PaEditorial.Subtle,
                            )
                        }
                        Icon(
                            painterResource(AppR.drawable.ic_lucide_chevron_right), null,
                            tint = PaEditorial.Subtle, modifier = Modifier.size(16.dp),
                        )
                    }
                    }
                }
            }
            }
        }
            // 撤销条浮在内容之上(删除后给一次反悔机会)
            // applyStatusBarsPadding = false:外层 Scaffold 的 topBar 已经消费过状态栏,
            // 内容区又整体下移了一个顶栏高度。组件若再加一次,提示条会掉到内容首行上。
            UndoBarHost(
                undo,
                Modifier.align(Alignment.TopCenter),
                applyStatusBarsPadding = false,
            )
        }
    }

    // 滚动中收起已展开的行,避免滑动过程中误触
    LaunchedEffect(listState.isScrollInProgress) {
        if (listState.isScrollInProgress) openKey.value = null
    }

    // 重命名:daemon 无独立 Rename,用 MoveEntry 同目录改名
    renaming?.let { e ->
        androidx.compose.ui.window.Dialog(onDismissRequest = { renaming = null }) {
            Column(
                Modifier
                    .background(PaEditorial.Surface, RoundedCornerShape(23.dp))
                    .padding(20.dp)
                    .fillMaxWidth(),
            ) {
                Text("重命名", fontSize = 17.sp, color = PaEditorial.Ink)
                Text(
                    (if (e.isDir) "文件夹 · " else "文件 · ") + middleEllipsize(e.name, 30),
                    fontSize = 11.sp, color = PaEditorial.Muted,
                    modifier = Modifier.padding(top = 2.dp, bottom = 10.dp), maxLines = 1,
                )
                androidx.compose.material3.OutlinedTextField(
                    value = renameText,
                    onValueChange = { renameText = it },
                    singleLine = true,
                    label = { Text("新名称", fontSize = 12.sp) },
                    modifier = Modifier.fillMaxWidth(),
                )
                Row(
                    Modifier.fillMaxWidth().padding(top = 14.dp),
                    horizontalArrangement = androidx.compose.foundation.layout.Arrangement.End,
                ) {
                    androidx.compose.material3.TextButton(onClick = { renaming = null }) { Text("取消") }
                    Spacer(Modifier.width(8.dp))
                    androidx.compose.material3.Button(
                        onClick = {
                            val n = renameText.trim()
                            if (n.isNotEmpty() && n != e.name) renameEntry(e, n)
                            renaming = null
                        },
                        colors = androidx.compose.material3.ButtonDefaults.buttonColors(containerColor = PaEditorial.Graphite),
                        shape = RoundedCornerShape(14.dp),
                    ) { Text("确定", color = androidx.compose.ui.graphics.Color.White, fontWeight = FontWeight.SemiBold) }
                }
            }
        }
    }

    // 删除确认:动作发生在**电脑上**(不是手机本地),目录是递归删除,后果不可逆。
    // 语义必须写清:删的是电脑上的文件;Undo 也只能有限恢复(空目录/空文件)。
    pendingDelete?.let { e ->
        androidx.compose.material3.AlertDialog(
            onDismissRequest = { pendingDelete = null },
            title = {
                Text(
                    if (e.isDir) "删除电脑上的文件夹?" else "删除电脑上的文件?",
                    fontSize = 15.sp,
                )
            },
            text = {
                Text(
                    if (e.isDir) {
                        "「${middleEllipsize(e.name, 30)}」及其全部内容将从电脑上永久删除,不进回收站。\n\n" +
                            "5 秒内可点「撤销」,但只能在原位置重建同名空文件夹,里面的内容找不回。"
                    } else {
                        "「${middleEllipsize(e.name, 30)}」将从电脑上永久删除,不进回收站。\n\n" +
                            "5 秒内可点「撤销」,但只能恢复为同名空文件,内容找不回。"
                    },
                    fontSize = 12.5.sp, color = PaEditorial.Muted,
                )
            },
            confirmButton = {
                TextButton(onClick = {
                    pendingDelete = null
                    deleteEntry(e)
                }) { Text("删除", color = EditorialTone.DENIED.text, fontWeight = FontWeight.SemiBold) }
            },
            dismissButton = { TextButton(onClick = { pendingDelete = null }) { Text("取消") } },
        )
    }

    // 不能内置预览时的引导:说清楚为什么,并给一步到位的下载动作
    askDownload?.let { e ->
        androidx.compose.material3.AlertDialog(
            onDismissRequest = { askDownload = null },
            title = { Text(middleEllipsize(e.name, 30), fontSize = 15.sp, maxLines = 1) },
            text = {
                Text(
                    "${e.kind.label} 这个格式没法在 App 内排版展示。\n" +
                        "下载到手机后,可以用系统里对应的应用打开。",
                    fontSize = 12.5.sp,
                    color = PaEditorial.Muted,
                )
            },
            confirmButton = {
                TextButton(onClick = {
                    PaDownloader.downloadFromPc(e.path)
                    PaHaptics.light(view)
                    android.widget.Toast.makeText(
                        ctx, "已开始下载 " + e.name, android.widget.Toast.LENGTH_SHORT,
                    ).show()
                    askDownload = null
                }) { Text("下载到手机") }
            },
            dismissButton = { TextButton(onClick = { askDownload = null }) { Text("取消") } },
        )
    }

    if (drawerCtrl.visible) {
        AppDrawer(
            currentTab = currentTab,
            hostTitle = "文件",
            progress = drawerCtrl.progress,
            onClose = { drawerCtrl.close() },
            onGoTab = { onGoTab(it) },
            onOpenTerminal = { onOpenTerminal() },
            onOpenSettings = { onOpenSettings() },
            onOpenTaskCenter = onOpenTaskCenter,
            onOpenBrowser = { onOpenBrowser("") },
        )
    }

    // 新建文件/文件夹对话框(直接在电脑上创建,桌面端同步可见)
    if (showCreate) {
        var name by remember { mutableStateOf("") }
        var isDir by remember { mutableStateOf(false) }
        androidx.compose.ui.window.Dialog(onDismissRequest = { showCreate = false }) {
            Column(
                Modifier
                    .background(PaEditorial.Surface, RoundedCornerShape(23.dp))
                    .padding(20.dp)
                    .fillMaxWidth(),
            ) {
                Text("新建", fontSize = 17.sp, color = PaEditorial.Ink)
                Text(
                    "位置:${path.ifEmpty { "(未进入目录)" }}",
                    fontSize = 11.sp, color = PaEditorial.Muted,
                    modifier = Modifier.padding(top = 2.dp, bottom = 10.dp), maxLines = 1,
                )
                androidx.compose.material3.OutlinedTextField(
                    value = name,
                    onValueChange = { name = it },
                    singleLine = true,
                    label = { Text(if (isDir) "文件夹名称" else "文件名称", fontSize = 12.sp) },
                    modifier = Modifier.fillMaxWidth(),
                )
                Row(Modifier.padding(top = 10.dp)) {
                    dev.pocketagent.app.ui.FilterChip2("文件", !isDir) { isDir = false }
                    Spacer(Modifier.width(8.dp))
                    dev.pocketagent.app.ui.FilterChip2("文件夹", isDir) { isDir = true }
                }
                Row(
                    Modifier.fillMaxWidth().padding(top = 14.dp),
                    horizontalArrangement = androidx.compose.foundation.layout.Arrangement.End,
                ) {
                    androidx.compose.material3.TextButton(onClick = { showCreate = false }) { Text("取消") }
                    Spacer(Modifier.width(8.dp))
                    androidx.compose.material3.Button(
                        onClick = {
                            if (name.isNotBlank()) {
                                createEntry(name.trim(), isDir)
                                showCreate = false
                            }
                        },
                        colors = androidx.compose.material3.ButtonDefaults.buttonColors(containerColor = PaEditorial.Graphite),
                        shape = RoundedCornerShape(14.dp),
                    ) { Text("创建", color = androidx.compose.ui.graphics.Color.White, fontWeight = FontWeight.SemiBold) }
                }
            }
        }
    }
}
