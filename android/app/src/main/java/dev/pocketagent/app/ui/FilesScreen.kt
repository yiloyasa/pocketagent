package dev.pocketagent.app.ui

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
    onGoTab: (Int) -> Unit = {},
) {
    var showDrawer by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()
    var path by remember { mutableStateOf(initialPath ?: "") }
    var showRoots by remember { mutableStateOf(initialPath.isNullOrEmpty()) }
    var entries by remember { mutableStateOf(listOf<DirectoryEntry>()) }
    var loading by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }
    var preview by remember { mutableStateOf<Pair<String, String>?>(null) }
    var showCreate by remember { mutableStateOf(false) } // 新建文件/文件夹

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
        modifier = Modifier.edgeSwipeToOpenDrawer { showDrawer = true },
        topBar = {
            Row(
                Modifier
                    .fillMaxWidth()
                    .statusBarsPadding()
                    .padding(horizontal = 12.dp, vertical = 8.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Image(
                    bitmap = agentBitmap(AppR.mipmap.ic_launcher),
                    contentDescription = "菜单",
                    modifier = Modifier
                        .size(40.dp)
                        .pressable { showDrawer = true },
                    contentScale = ContentScale.Crop,
                )
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
                    Row(
                        Modifier
                            .fillMaxWidth()
                            .background(Pa.Card, RoundedCornerShape(14.dp))
                            .clickable {
                                if (e.isDir) loadDir(e.path) else {
                                    scope.launch {
                                        try {
                                            val sb = StringBuilder()
                                            var isText = true
                                            withContext(Dispatchers.IO) {
                                                val it2 = AgentConnection.fileService().readFile(
                                                    ReadFileRequest.newBuilder()
                                                        .setPath(e.path)
                                                        .setMaxBytes(256 * 1024)
                                                        .build())
                                                while (it2.hasNext()) {
                                                    val c = it2.next()
                                                    if (c.totalSize > 0) isText = c.text
                                                    if (c.text) sb.append(String(c.data.toByteArray(), Charsets.UTF_8))
                                                }
                                            }
                                            preview = e.name to (if (isText) sb.toString() else "(二进制文件,不支持预览)")
                                        } catch (ex: Exception) {
                                            preview = e.name to "读取失败:${ex.message}"
                                        }
                                    }
                                }
                            }
                            .padding(horizontal = 14.dp, vertical = 12.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Icon(
                            painterResource(
                                if (e.isDir) AppR.drawable.ic_lucide_folder else AppR.drawable.ic_lucide_file_text,
                            ),
                            null,
                            tint = if (e.isDir) Pa.Accent else Pa.TextSecondary,
                            modifier = Modifier.size(20.dp),
                        )
                        Spacer(Modifier.width(12.dp))
                        Text(e.name, Modifier.weight(1f), fontSize = 14.sp, color = Pa.TextPrimary, maxLines = 1)
                        if (!e.isDir) {
                            Text(formatSize(e.sizeBytes), fontSize = 11.sp, color = Pa.TextTertiary)
                        }
                    }
                }
            }
        }
    }

    preview?.let { (name, content) ->
        androidx.compose.material3.AlertDialog(
            onDismissRequest = { preview = null },
            title = { Text(name, fontSize = 15.sp, maxLines = 1) },
            text = {
                // 【修复】预览可滚动
                Text(
                    content.take(30000),
                    Modifier
                        .height(420.dp)
                        .verticalScroll(rememberScrollState()),
                    fontSize = 11.sp,
                    fontFamily = FontFamily.Monospace,
                    color = Pa.TextPrimary,
                )
            },
            confirmButton = { TextButton(onClick = { preview = null }) { Text("关闭") } },
        )
    }

    if (showDrawer) {
        AppDrawer(
            currentTab = currentTab,
            hostTitle = "文件",
            onClose = { showDrawer = false },
            onGoTab = { onGoTab(it) },
            onOpenTerminal = { onOpenTerminal() },
            onOpenSettings = { onOpenSettings() },
            onOpenTaskCenter = onOpenTaskCenter,
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

