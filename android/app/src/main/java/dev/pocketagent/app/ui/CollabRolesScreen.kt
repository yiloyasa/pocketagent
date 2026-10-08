package dev.pocketagent.app.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Dialog
import android.widget.Toast
import dev.pocketagent.app.R
import kotlinx.coroutines.launch
import dev.pocketagent.app.ui.anim.pressable
import dev.pocketagent.app.ui.component.PaNavIconButton
import dev.pocketagent.app.ui.component.PaScreenScaffold
import dev.pocketagent.app.ui.theme.EditorialTone
import dev.pocketagent.app.ui.theme.PaEditorial
import dev.pocketagent.app.ui.theme.editorial
import pocketagent.v1.CollabRole

/**
 * 协同角色列表页(编辑式):看到 / 新增 / 编辑入口(编辑走 [onOpenEditor],删除走二次确认)。
 *
 * 【宁缺勿假】加载失败与空态分开处理(加载中给指示;失败给原因+重试;空态给人话
 * "还没有配置协同角色"+新建,绝不说"未知/错误",见 AUDIT §3.3)。数据来自 [CollabRolesState],
 * 进页面即拉一次;保存/删除后状态对象自行刷新,从编辑器回退到此页即见最新。
 * 【视觉】列表行 = 瓷白卡 + 细线(README:角色管理这类"有主次"的配置内容用瓷白卡),
 * 页面骨架走 [PaScreenScaffold](标题 25/32,右上唯一一次操作 = 新建)。
 */
@Composable
fun CollabRolesScreen(
    onBack: () -> Unit,
    onOpenEditor: (String) -> Unit,
) {
    val ed = MaterialTheme.editorial
    LaunchedEffect(Unit) { CollabRolesState.load() }
    val loadState = CollabRolesState.loadState
    val roles = CollabRolesState.roles
    val loadError = CollabRolesState.loadError

    PaScreenScaffold(
        title = "协同角色",
        leading = {
            PaNavIconButton(
                onClick = onBack,
                contentDescription = "返回",
                iconRes = R.drawable.ic_lucide_chevron_left,
                plain = true,
            )
        },
        // 唯一次操作:新建(空态与有数据都一样可达)
        trailing = {
            PaNavIconButton(
                onClick = { onOpenEditor("") },
                contentDescription = "新建协同角色",
                iconRes = R.drawable.ic_lucide_plus,
            )
        },
    ) {
        when (loadState) {
            CollabRolesLoadState.LOADING -> {
                Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                    CircularProgressIndicator(
                        color = PaEditorial.Primary, strokeWidth = 3.dp, modifier = Modifier.size(28.dp),
                    )
                }
            }
            CollabRolesLoadState.ERROR -> {
                CollabErrorState(error = loadError ?: "加载失败", onRetry = { CollabRolesState.load() })
            }
            CollabRolesLoadState.READY -> {
                if (roles.isEmpty()) {
                    CollabEmptyState(onNew = { onOpenEditor("") })
                } else {
                    LazyColumn(
                        contentPadding = PaddingValues(top = 8.dp, bottom = 24.dp),
                    ) {
                        items(roles, key = { it.id }) { role ->
                            CollabRoleRow(role = role, onClick = { onOpenEditor(role.id) })
                        }
                    }
                }
            }
        }
    }
}

/** 空态:人话说明 + 新建入口(无数据时绝不显示"未知/错误",AUDIT §3.3)。 */
@Composable
private fun CollabEmptyState(onNew: () -> Unit) {
    val ed = MaterialTheme.editorial
    Column(
        Modifier.fillMaxSize().padding(horizontal = 8.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.Center,
    ) {
        Text(
            "还没有配置协同角色",
            style = PaEditorial.Type.Heading, color = ed.Ink,
        )
        Spacer(Modifier.height(6.dp))
        Text(
            "配置后,委派子任务时直接引用角色 id 即可复用同一套岗位设定",
            fontSize = PaEditorial.Type.BodySmall.fontSize, lineHeight = 21.sp, color = ed.Muted,
        )
        Spacer(Modifier.height(16.dp))
        Row(
            Modifier
                .fillMaxWidth()
                .heightIn(min = PaEditorial.Space.TouchMin)
                .pressable { onNew() }
                .background(ed.PrimaryContainer, RoundedCornerShape(PaEditorial.Radius.Card))
                .padding(14.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Icon(painterResource(R.drawable.ic_lucide_plus), null, tint = PaEditorial.Primary, modifier = Modifier.size(18.dp))
            Spacer(Modifier.width(10.dp))
            Text("新建协同角色", fontSize = PaEditorial.Type.Heading.fontSize, color = PaEditorial.Primary)
        }
    }
}

/** 加载失败态:原因 + 重试(原因原样透传,不泛化)。 */
@Composable
private fun CollabErrorState(error: String, onRetry: () -> Unit) {
    val ed = MaterialTheme.editorial
    Column(
        Modifier.fillMaxSize().padding(horizontal = 8.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.Center,
    ) {
        Text(
            "协同角色加载失败",
            style = PaEditorial.Type.Heading, color = ed.Ink,
        )
        Spacer(Modifier.height(6.dp))
        Text(
            error,
            fontSize = PaEditorial.Type.BodySmall.fontSize, lineHeight = 21.sp,
            color = EditorialTone.DENIED.text,
            maxLines = 3, overflow = TextOverflow.Ellipsis,
        )
        Spacer(Modifier.height(16.dp))
        Row(
            Modifier
                .fillMaxWidth()
                .heightIn(min = PaEditorial.Space.TouchMin)
                .pressable { onRetry() }
                .background(ed.Surface, RoundedCornerShape(PaEditorial.Radius.Card))
                .border(1.dp, ed.Hairline, RoundedCornerShape(PaEditorial.Radius.Card))
                .padding(14.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Icon(painterResource(R.drawable.ic_lucide_refresh_cw), null, tint = PaEditorial.Primary, modifier = Modifier.size(18.dp))
            Spacer(Modifier.width(10.dp))
            Text("重试", fontSize = PaEditorial.Type.Heading.fontSize, color = PaEditorial.Primary)
        }
    }
}

/** 单个角色行(瓷白卡 + 细线):显示名 + agent/model/effort 摘要 + 职责预览;点进编辑;删除走二次确认。 */
@Composable
private fun CollabRoleRow(role: CollabRole, onClick: () -> Unit) {
    val ed = MaterialTheme.editorial
    var showDelete by remember { mutableStateOf(false) }
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    Column(
        Modifier
            .fillMaxWidth()
            .padding(vertical = 6.dp)
            .pressable { onClick() }
            // 瓷白卡 + 细线(编辑式:配置类内容用瓷白卡,普通列表靠留白+细线)
            .background(ed.Surface, RoundedCornerShape(PaEditorial.Radius.Card))
            .border(1.dp, ed.Hairline, RoundedCornerShape(PaEditorial.Radius.Card))
            .padding(16.dp),
    ) {
        Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
            Column(Modifier.weight(1f)) {
                Text(
                    collabRoleDisplayName(role),
                    fontSize = PaEditorial.Type.Heading.fontSize, lineHeight = 21.sp,
                    fontWeight = FontWeight.SemiBold, color = ed.Ink,
                    maxLines = 1, overflow = TextOverflow.Ellipsis,
                )
                val summary = collabRoleSummaryLine(role)
                if (summary.isNotEmpty()) {
                    Spacer(Modifier.height(3.dp))
                    Text(
                        summary, fontSize = PaEditorial.Type.Caption.fontSize, lineHeight = 17.sp, color = ed.Muted,
                        maxLines = 1, overflow = TextOverflow.Ellipsis,
                    )
                }
                if (role.description.isNotBlank()) {
                    Spacer(Modifier.height(3.dp))
                    Text(
                        role.description, fontSize = PaEditorial.Type.Caption.fontSize, lineHeight = 16.sp, color = ed.Subtle,
                        maxLines = 1, overflow = TextOverflow.Ellipsis,
                    )
                }
            }
            androidx.compose.foundation.layout.Box(
                Modifier
                    .size(PaEditorial.Space.TouchMin)
                    .pressable { showDelete = true },
                contentAlignment = Alignment.Center,
            ) {
                Icon(
                    painterResource(R.drawable.ic_lucide_trash_2), "删除",
                    tint = ed.Muted, modifier = Modifier.size(18.dp),
                )
            }
        }
    }
    if (showDelete) {
        CollabDeleteConfirm(
            roleName = collabRoleDisplayName(role),
            roleId = role.id,
            onDismiss = { showDelete = false },
            onConfirm = {
                showDelete = false
                // 失败用 Toast 把 daemon 原文如实透传(不泛化);成功时状态对象已自行移除该行。
                scope.launch {
                    val err = CollabRolesState.delete(role.id)
                    if (err != null) Toast.makeText(ctx, err, Toast.LENGTH_LONG).show()
                }
            },
        )
    }
}

/** 删除二次确认:明确写出后果(引用它的委派会报"未定义的协同角色")。编辑式瓷白弹窗。 */
@Composable
private fun CollabDeleteConfirm(
    roleName: String,
    roleId: String,
    onDismiss: () -> Unit,
    onConfirm: () -> Unit,
) {
    val ed = MaterialTheme.editorial
    Dialog(onDismissRequest = onDismiss) {
        Column(
            Modifier
                .fillMaxWidth()
                .background(Color.Transparent)
                .padding(horizontal = PaEditorial.Space.PageMargin),
        ) {
            Column(
                Modifier
                    .fillMaxWidth()
                    .background(ed.Surface, RoundedCornerShape(PaEditorial.Radius.Sheet))
                    .border(1.dp, ed.Hairline, RoundedCornerShape(PaEditorial.Radius.Sheet))
                    .padding(20.dp),
            ) {
                Text(
                    "删除协同角色",
                    style = PaEditorial.Type.Heading, color = ed.Ink,
                )
                Spacer(Modifier.height(10.dp))
                Text(
                    "确定删除「$roleName」($roleId)?\n\n删除后,引用它的委派会报「未定义的协同角色 $roleId」,正在进行的协作可能中断。此操作不可撤销。",
                    fontSize = PaEditorial.Type.BodySmall.fontSize, lineHeight = 21.sp, color = ed.Muted,
                )
                Spacer(Modifier.height(18.dp))
                Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.End) {
                    Text(
                        "取消", fontSize = PaEditorial.Type.Body.fontSize, color = ed.Muted,
                        modifier = Modifier
                            .pressable { onDismiss() }
                            .padding(horizontal = 14.dp, vertical = 12.dp),
                    )
                    Spacer(Modifier.width(8.dp))
                    Text(
                        "删除", fontSize = PaEditorial.Type.Body.fontSize,
                        fontWeight = FontWeight.SemiBold,
                        color = EditorialTone.DENIED.text,
                        modifier = Modifier
                            .pressable { onConfirm() }
                            .padding(horizontal = 14.dp, vertical = 12.dp),
                    )
                }
            }
        }
    }
}
