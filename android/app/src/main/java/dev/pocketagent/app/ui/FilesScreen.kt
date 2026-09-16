package dev.pocketagent.app.ui

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Card
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.ListItem
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.AgentConnection
import dev.pocketagent.app.R as AppR
import dev.pocketagent.app.ui.anim.IconActionButton
import dev.pocketagent.app.ui.component.FileKind
import dev.pocketagent.app.ui.component.PaDownloader
import dev.pocketagent.app.ui.component.PreviewMode
import dev.pocketagent.app.ui.component.fileKindOf
import androidx.compose.ui.graphics.Color
import dev.pocketagent.app.ui.anim.edgeSwipeToOpenDrawer
import dev.pocketagent.app.ui.anim.pressable
import dev.pocketagent.app.ui.theme.Pa
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import pocketagent.v1.CreateDirectoryRequest
import pocketagent.v1.DirectoryEntry
import pocketagent.v1.WriteFileRequest
import pocketagent.v1.GetHostInfoRequest
import pocketagent.v1.ListDirectoryRequest
import pocketagent.v1.ListRootsRequest
import pocketagent.v1.ReadFileRequest

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
    var showDrawer by remember { mutableStateOf(false) }
    // 侧边栏跟手:与主页/会话页同一套 controller
    val drawerCtrl = dev.pocketagent.app.ui.component.rememberDrawerController()
    // 返回层级:抽屉开着先关抽屉,否则回到主页并展开侧边栏(而不是直接退回桌面)
    BackHandler(enabled = true) {
        if (drawerCtrl.visible) drawerCtrl.close() else onBackToHome()
    }
    val scope = rememberCoroutineScope()
    var path by remember { mutableStateOf(initialPath ?: "") }
    var showRoots by remember { mutableStateOf(initialPath.isNullOrEmpty()) }
    var entries by remember { mutableStateOf(listOf<DirectoryEntry>()) }
    var loading by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }
    var showCreate by remember { mutableStateOf(false) } // 新建文件/文件夹
    // 不能内置预览的文件(旧版 Office / 压缩包 / 音视频):点它时给一个"下载到手机"的引导
    var askDownload by remember { mutableStateOf<DirectoryEntry?>(null) }
    val ctx = androidx.compose.ui.platform.LocalContext.current

    fun loadDir(p: String) {
        loading = true
        scope.launch {
            try {
                entries = withContext(Dispatchers.IO) {
                    AgentConnection.fileService()
                        .listDirectory(ListDirectoryRequest.newBuilder().setPath(p).build())
                        .entriesList
                }.sortedWith(
                    compareByDescending<DirectoryEntry> { it.isDir }.thenBy { it.name.lowercase() },
                )
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

    fun loadRoots(preserveError: String? = null) {
        loading = true
        scope.launch {
            try {
                entries = withContext(Dispatchers.IO) {
                    AgentConnection.fileService()
                        .listRoots(ListRootsRequest.newBuilder().build())
                        .rootsList
                }
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

    Scaffold(
        containerColor = Pa.Bg,
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
                    .padding(horizontal = 12.dp, vertical = 8.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                IconActionButton(AppR.drawable.ic_lucide_menu, "菜单", { drawerCtrl.open() }, bg = Pa.Card)
                if (!showRoots) {
                IconActionButton(
                    AppR.drawable.ic_lucide_chevron_left, "上级",
                    {
                        // C:\dev\x -> C:\dev;C:\dev -> C:\(盘根仍是目录);C:\ -> 此电脑
                        val norm = path.replace('/', '\\').trimEnd('\\')
                        val idx = norm.lastIndexOf('\\')
                        when {
                            idx > 2 -> loadDir(norm.substring(0, idx) + "\\")
                            idx == 2 -> loadDir(norm + "\\")
                            else -> loadRoots()
                        }
                    },
                    bg = Pa.Card,
                )
                    Spacer(Modifier.width(10.dp))
                }
                Text(
                    when {
                        showRoots -> "此电脑"
                        else -> path.substringAfterLast('\\').substringAfterLast('/').ifEmpty { path }
                    },
                    Modifier.weight(1f),
                    fontSize = 17.sp,
                    color = Pa.TextPrimary,
                    maxLines = 1,
                )
                if (loading) {
                    CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp, color = Pa.Accent)
                    Spacer(Modifier.width(12.dp))
                }
                IconActionButton(
                    AppR.drawable.ic_lucide_monitor, "此电脑",
                    { loadRoots() },
                    bg = Pa.Card,
                )
                Spacer(Modifier.width(8.dp))
                if (!showRoots) {
                    IconActionButton(
                        AppR.drawable.ic_lucide_plus, "新建",
                        { showCreate = true },
                        bg = Pa.Card,
                    )
                    Spacer(Modifier.width(8.dp))
                }
                IconActionButton(
                    AppR.drawable.ic_lucide_refresh_cw, "刷新",
                    { if (showRoots) loadRoots() else loadDir(path) },
                    bg = Pa.Card,
                )
            }
        },
    ) { pad ->
        Column(Modifier.padding(pad).fillMaxSize()) {
            if (path.isNotEmpty()) {
                Text(
                    path,
                    Modifier.padding(horizontal = 16.dp, vertical = 4.dp),
                    fontSize = 11.sp,
                    fontFamily = FontFamily.Monospace,
                    color = Pa.TextTertiary,
                    maxLines = 1,
                )
            }
            error?.let {
                Card(Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 6.dp)) {
                    Text(it, Modifier.padding(10.dp), color = Pa.Red, fontSize = 13.sp)
                }
            }
            // 【修复】weight(1f) 让 LazyColumn 占满剩余高度并启用懒加载滚动;
            // 之前没加 weight 导致列表不滚动、看不到全部
            LazyColumn(
                Modifier.weight(1f).padding(horizontal = 16.dp),
                verticalArrangement = androidx.compose.foundation.layout.Arrangement.spacedBy(8.dp),
            ) {
                items(entries, key = { it.path }) { e ->
                    val kind = if (e.isDir) null else fileKindOf(e.name)
                    Row(
                        Modifier
                            .fillMaxWidth()
                            .background(Pa.Card, RoundedCornerShape(14.dp))
                            .clickable {
                                when {
                                    e.isDir -> loadDir(e.path)
                                    // 【能力矩阵分派】能内置渲染的一律交给内置浏览器 ——
                                    // Office / PDF / Markdown / 图片 / 网页 都能排版展示,
                                    // 比在这个弹层里塞纯文本好得多。
                                    kind?.preview == PreviewMode.IN_APP -> onOpenBrowser(
                                        // 【带上目录条目的 size/mtime】内置浏览器据此生成预览缓存键 ——
                                        // 文件在电脑上被改过 → mtime 变 → 键变 → 自动重拉,不会看到旧内容。
                                        "pc:" + e.path + "?_pas=" + e.sizeBytes + "&_pam=" + e.modifiedUnixMs,
                                    )
                                    // 内置不了(旧版 Office / 压缩包 / 音视频)才引导下载
                                    else -> askDownload = e
                                }
                            }
                            .padding(start = 14.dp, end = 6.dp, top = 12.dp, bottom = 12.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Icon(
                            painterResource(
                                if (e.isDir) AppR.drawable.ic_lucide_folder else AppR.drawable.ic_lucide_file_text,
                            ),
                            null,
                            tint = if (e.isDir) Pa.Accent else (kind?.color ?: Pa.TextSecondary),
                            modifier = Modifier.size(20.dp),
                        )
                        Spacer(Modifier.width(12.dp))
                        Column(Modifier.weight(1f)) {
                            Text(
                                e.name, fontSize = 14.sp, color = Pa.TextPrimary,
                                maxLines = 1, overflow = androidx.compose.ui.text.style.TextOverflow.Ellipsis,
                            )
                            if (!e.isDir) {
                                Text(
                                    listOfNotNull(
                                        formatSize(e.sizeBytes),
                                        kind?.takeIf { it != FileKind.OTHER }?.label,
                                        if (kind?.preview == PreviewMode.IN_APP) "可预览" else "需下载打开",
                                    ).joinToString("  ·  "),
                                    fontSize = 10.5.sp, color = Pa.TextTertiary, maxLines = 1,
                                )
                            }
                        }
                        if (!e.isDir) {
                            // 每一行都能单独下载到手机(不必先进浏览器)
                            IconActionButton(
                                AppR.drawable.ic_lucide_download, "下载到手机",
                                {
                                    PaDownloader.downloadFromPc(e.path)
                                    android.widget.Toast.makeText(
                                        ctx, "已开始下载 " + e.name, android.widget.Toast.LENGTH_SHORT,
                                    ).show()
                                },
                                bg = Color.Transparent, size = 34.dp, iconSize = 16.dp,
                                tint = Pa.TextSecondary,
                            )
                        }
                    }
                }
            }
        }
    }

    // 不能内置预览时的引导:说清楚为什么,并给一步到位的下载动作
    askDownload?.let { e ->
        val k = fileKindOf(e.name)
        androidx.compose.material3.AlertDialog(
            onDismissRequest = { askDownload = null },
            title = { Text(e.name, fontSize = 15.sp, maxLines = 1) },
            text = {
                Text(
                    "${k.label} 这个格式没法在 App 内排版展示。\n" +
                        "下载到手机后,可以用系统里对应的应用打开。",
                    fontSize = 12.5.sp,
                    color = Pa.TextSecondary,
                )
            },
            confirmButton = {
                TextButton(onClick = {
                    PaDownloader.downloadFromPc(e.path)
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
                    .background(Pa.CardHigh, RoundedCornerShape(20.dp))
                    .padding(20.dp)
                    .fillMaxWidth(),
            ) {
                Text("新建", fontSize = 17.sp, color = Pa.TextPrimary)
                Text(
                    "位置:${path.ifEmpty { "(未进入目录)" }}",
                    fontSize = 11.sp, color = Pa.TextTertiary,
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
                        colors = androidx.compose.material3.ButtonDefaults.buttonColors(containerColor = Pa.Accent),
                        shape = RoundedCornerShape(14.dp),
                    ) { Text("创建", color = androidx.compose.ui.graphics.Color(0xFF0B0B0D)) }
                }
            }
        }
    }
}

private fun formatSize(n: Long): String = when {
    n < 1024 -> "$n B"
    n < 1024 * 1024 -> "%.1f KB".format(n / 1024.0)
    else -> "%.1f MB".format(n / 1024.0 / 1024.0)
}

