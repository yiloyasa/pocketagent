package dev.pocketagent.app.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.AgentConnection
import dev.pocketagent.app.R
import dev.pocketagent.app.ui.anim.IconActionButton
import dev.pocketagent.app.ui.anim.pressable
import dev.pocketagent.app.ui.component.PaSheet
import dev.pocketagent.app.ui.component.parentDirectoryOf
import dev.pocketagent.app.ui.theme.Pa
import dev.pocketagent.app.ui.theme.PaEditorial
import dev.pocketagent.app.ui.theme.EditorialTone
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import pocketagent.v1.DirectoryEntry
import pocketagent.v1.ListDirectoryRequest
import pocketagent.v1.ListRootsRequest

/**
 * 文件式目录选择器(与"文件"页同款浏览体验):
 * 此电脑盘符 → 逐级点进文件夹 → 底部"选择此文件夹"确认。
 * 供 新建会话选工作区 / 终端切换目录 等所有需要选文件夹的地方复用。
 *
 * 【PaSheet 统一浮层】玻璃底 + 82% 屏高上限 + 内建滚动(AUDIT sheet 溢出规则的
 * 结构化落地);目录列表自身是 LazyColumn,故 scrollable=false 防滚动嵌套打架。
 */
@Composable
fun FolderPickerSheet(onDismiss: () -> Unit, onPick: (String) -> Unit) {
    val scope = rememberCoroutineScope()
    var path by remember { mutableStateOf("") }
    var showRoots by remember { mutableStateOf(true) }
    var entries by remember { mutableStateOf(listOf<DirectoryEntry>()) }
    var loading by remember { mutableStateOf(false) }

    fun loadDir(p: String) {
        loading = true
        scope.launch {
            try {
                entries = withContext(Dispatchers.IO) {
                    AgentConnection.fileService()
                        .listDirectory(ListDirectoryRequest.newBuilder().setPath(p).build())
                        .entriesList
                }.filter { it.isDir }.sortedBy { it.name.lowercase() }
                path = p
                showRoots = false
            } catch (_: Exception) {
            } finally {
                loading = false
            }
        }
    }

    fun loadRoots() {
        loading = true
        scope.launch {
            try {
                entries = withContext(Dispatchers.IO) {
                    AgentConnection.fileService()
                        .listRoots(ListRootsRequest.newBuilder().build())
                        .rootsList
                }.filter { it.isDir }
                path = ""
                showRoots = true
            } catch (_: Exception) {
            } finally {
                loading = false
            }
        }
    }

    LaunchedEffect(Unit) { loadRoots() }

    /**
     * 【返回层级 · 第一级】层级选择器内部:先进了多层文件夹就逐级往上,
     * 已经在盘符列表(最顶层)才把返回交给下一级 —— 关掉整个选择器。
     * 和"文件"页共用同一套坐标规则。
     */
    fun goUpOneLevel(): Boolean {
        if (showRoots) return false
        // 与文件页共用同一份坐标规则(component/PathNav.kt)
        val up = parentDirectoryOf(path)
        if (up != null) loadDir(up) else loadRoots()
        return true
    }

    // ModalBottomSheet(PaSheet 底层)自身的返回处理是"直接关闭",会吞掉第一级层级。
    // 这里用优先级更高的 BackHandler 先吃掉,退到顶层后才放行给 onDismiss。
    androidx.activity.compose.BackHandler(enabled = !showRoots) { goUpOneLevel() }

    PaSheet(onDismissRequest = onDismiss, scrollable = false) {
        Column(Modifier.fillMaxWidth().padding(horizontal = 16.dp)) {
            Text(
                "选择文件夹", Modifier.fillMaxWidth().padding(horizontal = 8.dp),
                fontSize = 17.sp, lineHeight = 24.sp,
                fontWeight = FontWeight.SemiBold,
                color = PaEditorial.Ink,
            )
            // 路径栏:上级 + 当前位置
            Row(
                Modifier.fillMaxWidth().padding(horizontal = 8.dp, vertical = 8.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                if (!showRoots) {
                    IconActionButton(
                        R.drawable.ic_lucide_chevron_left, "上级",
                        { goUpOneLevel() },
                        bg = PaEditorial.Surface, size = 34.dp, iconSize = 16.dp,
                    )
                    Spacer(Modifier.width(10.dp))
                }
                Text(
                    if (showRoots) "此电脑" else path,
                    Modifier.weight(1f),
                    fontSize = 12.sp, color = PaEditorial.Muted,
                    maxLines = 1, overflow = TextOverflow.Ellipsis,
                )
                if (loading) {
                    Spacer(Modifier.width(8.dp))
                    CircularProgressIndicator(Modifier.size(14.dp), strokeWidth = 2.dp, color = PaEditorial.Primary)
                }
            }

            LazyColumn(Modifier.weight(1f, fill = false).height(420.dp)) {
                items(entries, key = { it.path.ifEmpty { it.name } }) { e ->
                    Row(
                        Modifier
                            .fillMaxWidth()
                            .pressable { if (e.isDir) loadDir(e.path.ifEmpty { joinPath(path, e.name) }) }
                            .background(PaEditorial.Surface, RoundedCornerShape(13.dp))
                            .border(1.dp, PaEditorial.Hairline, RoundedCornerShape(13.dp))
                            .padding(horizontal = 12.dp, vertical = 11.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Icon(
                            painterResource(R.drawable.ic_lucide_folder), null,
                            tint = PaEditorial.Running, modifier = Modifier.size(18.dp),
                        )
                        Spacer(Modifier.width(10.dp))
                        Text(e.name, fontSize = 14.sp, color = PaEditorial.Ink, maxLines = 1)
                    }
                    Spacer(Modifier.height(6.dp))
                }
            }

            // 底部:确认选择
            Box(
                Modifier
                    .fillMaxWidth()
                    .padding(top = 10.dp, bottom = 18.dp)
                    .pressable(enabled = !showRoots) {
                        if (!showRoots) {
                            AgentConnection.addRecentWorkspace(path)
                            onPick(path)
                        }
                    }
                    .background(if (showRoots) PaEditorial.Raised else PaEditorial.Graphite, RoundedCornerShape(14.dp))
                    .padding(vertical = 15.dp),
                contentAlignment = Alignment.Center,
            ) {
                Text(
                    if (showRoots) "先进入一个文件夹" else "选择此文件夹",
                    fontSize = 15.sp,
                    color = if (showRoots) PaEditorial.Muted else Color.White,
                    fontWeight = FontWeight.SemiBold,
                )
            }
        }
    }
}

/** 拼接目录路径(处理 \ 与 / 两种分隔符)。 */
private fun joinPath(parent: String, name: String): String {
    if (parent.isBlank()) return name // 盘符根(如 C:)直接拼名字
    val sep = if (parent.contains('\\')) "\\" else "/"
    return parent.trimEnd('\\', '/') + sep + name
}
