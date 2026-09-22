package dev.pocketagent.app.ui

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
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Dialog
import android.widget.Toast
import dev.pocketagent.app.R
import kotlinx.coroutines.launch
import dev.pocketagent.app.ui.anim.IconActionButton
import dev.pocketagent.app.ui.anim.pressable
import dev.pocketagent.app.ui.theme.Pa
import pocketagent.v1.CollabRole

/**
 * 协同角色列表页:看到 / 新增 / 编辑入口(编辑走 [onOpenEditor],删除走二次确认)。
 *
 * 【宁缺勿假】加载失败与空态分开处理(加载中给指示;失败给原因+重试;空态给人话
 * "还没有配置协同角色"+新建,绝不说"未知/错误",见 AUDIT §3.3)。数据来自 [CollabRolesState],
 * 进页面即拉一次;保存/删除后状态对象自行刷新,从编辑器回退到此页即见最新。
 */
@Composable
fun CollabRolesScreen(
    onBack: () -> Unit,
    onOpenEditor: (String) -> Unit,
) {
    LaunchedEffect(Unit) { CollabRolesState.load() }
    val loadState = CollabRolesState.loadState
    val roles = CollabRolesState.roles
    val loadError = CollabRolesState.loadError

    Column(Modifier.fillMaxSize().background(Pa.Bg).statusBarsPadding()) {
        // ---- 顶部栏:返回 / 标题 / 新建(空态与有数据都一样可达) ----
        Row(
            Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 10.dp),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            IconActionButton(R.drawable.ic_lucide_chevron_left, "返回", onBack, bg = Pa.Card)
            Text("协同角色", fontSize = 17.sp, color = Pa.TextPrimary)
            IconActionButton(R.drawable.ic_lucide_plus, "新建协同角色", { onOpenEditor("") }, bg = Pa.Card)
        }

        when (loadState) {
            CollabRolesLoadState.LOADING -> {
                Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                    CircularProgressIndicator(color = Pa.Accent, strokeWidth = 3.dp, modifier = Modifier.size(28.dp))
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
                        contentPadding = PaddingValues(horizontal = 16.dp, vertical = 8.dp),
                        verticalArrangement = Arrangement.spacedBy(10.dp),
                    ) {
                        items(roles, key = { it.id }) { role ->
                            CollabRoleRow(role = role, onClick = { onOpenEditor(role.id) })
                        }
                        item { Spacer(Modifier.height(8.dp)) }
                    }
                }
            }
        }
    }
}

/** 空态:人话说明 + 新建入口(无数据时绝不显示"未知/错误",AUDIT §3.3)。 */
@Composable
private fun CollabEmptyState(onNew: () -> Unit) {
    Column(
        Modifier.fillMaxSize().padding(horizontal = 32.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.Center,
    ) {
        Text("还没有配置协同角色", fontSize = 16.sp, color = Pa.TextPrimary)
        Spacer(Modifier.height(6.dp))
        Text(
            "配置后,委派子任务时直接引用角色 id 即可复用同一套岗位设定",
            fontSize = 13.sp, color = Pa.TextSecondary,
        )
        Spacer(Modifier.height(16.dp))
        Row(
            Modifier
                .fillMaxWidth()
                .pressable { onNew() }
                .background(Pa.AccentDim, RoundedCornerShape(14.dp))
                .padding(14.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Icon(painterResource(R.drawable.ic_lucide_plus), null, tint = Pa.Accent, modifier = Modifier.size(18.dp))
            Spacer(Modifier.width(10.dp))
            Text("新建协同角色", fontSize = 15.sp, color = Pa.Accent)
        }
    }
}

/** 加载失败态:原因 + 重试(原因原样透传,不泛化)。 */
@Composable
private fun CollabErrorState(error: String, onRetry: () -> Unit) {
    Column(
        Modifier.fillMaxSize().padding(horizontal = 32.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.Center,
    ) {
        Text("协同角色加载失败", fontSize = 16.sp, color = Pa.TextPrimary)
        Spacer(Modifier.height(6.dp))
        Text(error, fontSize = 13.sp, color = Pa.RedText, maxLines = 3, overflow = TextOverflow.Ellipsis)
        Spacer(Modifier.height(16.dp))
        Row(
            Modifier
                .fillMaxWidth()
                .pressable { onRetry() }
                .background(Pa.Card, RoundedCornerShape(14.dp))
                .padding(14.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Icon(painterResource(R.drawable.ic_lucide_chevron_right), null, tint = Pa.Accent, modifier = Modifier.size(18.dp))
            Spacer(Modifier.width(10.dp))
            Text("重试", fontSize = 15.sp, color = Pa.Accent)
        }
    }
}

/** 单个角色行:显示名 + agent/model/effort 摘要 + 职责预览;点击进编辑;删除走二次确认。 */
@Composable
private fun CollabRoleRow(role: CollabRole, onClick: () -> Unit) {
    var showDelete by remember { mutableStateOf(false) }
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    Column(
        Modifier
            .fillMaxWidth()
            .pressable { onClick() }
            .background(Pa.Card, RoundedCornerShape(16.dp))
            .padding(14.dp),
    ) {
        Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
            Column(Modifier.weight(1f)) {
                Text(
                    collabRoleDisplayName(role), fontSize = 15.sp, color = Pa.TextPrimary,
                    maxLines = 1, overflow = TextOverflow.Ellipsis,
                )
                val summary = collabRoleSummaryLine(role)
                if (summary.isNotEmpty()) {
                    Spacer(Modifier.height(3.dp))
                    Text(summary, fontSize = 12.sp, color = Pa.TextSecondary, maxLines = 1, overflow = TextOverflow.Ellipsis)
                }
                if (role.description.isNotBlank()) {
                    Spacer(Modifier.height(3.dp))
                    Text(role.description, fontSize = 11.5.sp, color = Pa.TextTertiary, maxLines = 1, overflow = TextOverflow.Ellipsis)
                }
            }
            IconActionButton(
                R.drawable.ic_lucide_trash_2, "删除", { showDelete = true },
                bg = Color.Transparent, size = 36.dp, iconSize = 18.dp, tint = Pa.TextSecondary,
            )
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

/** 删除二次确认:明确写出后果(引用它的委派会报"未定义的协同角色")。 */
@Composable
private fun CollabDeleteConfirm(
    roleName: String,
    roleId: String,
    onDismiss: () -> Unit,
    onConfirm: () -> Unit,
) {
    Dialog(onDismissRequest = onDismiss) {
        Column(
            Modifier
                .fillMaxWidth()
                .background(Pa.CardHigh, RoundedCornerShape(22.dp))
                .padding(20.dp),
        ) {
            Text("删除协同角色", fontSize = 17.sp, color = Pa.TextPrimary)
            Spacer(Modifier.height(10.dp))
            Text(
                "确定删除「$roleName」($roleId)?\n\n删除后,引用它的委派会报「未定义的协同角色 $roleId」,正在进行的协作可能中断。此操作不可撤销。",
                fontSize = 13.sp, color = Pa.TextSecondary,
            )
            Spacer(Modifier.height(18.dp))
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.End) {
                Text(
                    "取消", fontSize = 14.sp, color = Pa.TextSecondary,
                    modifier = Modifier.pressable { onDismiss() }.padding(horizontal = 14.dp, vertical = 8.dp),
                )
                Spacer(Modifier.width(8.dp))
                Text(
                    "删除", fontSize = 14.sp, color = Pa.Red, fontWeight = androidx.compose.ui.text.font.FontWeight.Medium,
                    modifier = Modifier.pressable { onConfirm() }.padding(horizontal = 14.dp, vertical = 8.dp),
                )
            }
        }
    }
}
