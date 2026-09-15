package dev.pocketagent.app.ui

import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
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
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.R
import dev.pocketagent.app.ui.anim.EmptyState
import dev.pocketagent.app.ui.anim.IconActionButton
import dev.pocketagent.app.ui.anim.edgeSwipeToOpenDrawer
import dev.pocketagent.app.ui.anim.pressable
import dev.pocketagent.app.ui.component.TaskCenter
import dev.pocketagent.app.ui.theme.Pa
import kotlinx.coroutines.delay
import java.text.SimpleDateFormat
import java.util.Date

/**
 * 任务中心(设计规范 §2.6)。
 *
 * 【无弹窗】未读只以角标告知,点进来直接看列表 —— 符合"不使用弹窗"的红线。
 * 【单一真相】这里只渲染 [TaskCenter.tasks] 的快照;轮次进度仍由 TurnSession 持有,
 * 所以不存在"任务中心说完成了、聊天页还在转"的双份状态。
 */
@Composable
fun TaskCenterScreen(
    currentTab: Int = 4,
    onOpenThread: (String) -> Unit,
    onOpenTerminal: () -> Unit,
    onOpenSettings: () -> Unit,
    onGoTab: (Int) -> Unit,
    modifier: Modifier = Modifier,
) {
    var kindTab by remember { mutableStateOf(TaskCenter.Kind.MESSAGE) }
    var showDrawer by remember { mutableStateOf(false) }

    val all by TaskCenter.tasks.collectAsState()
    val shown = all.filter { it.kind == kindTab }

    val running = all.count { it.status == TaskCenter.Status.RUNNING }
    val failed = all.count { it.status == TaskCenter.Status.FAILED || it.status == TaskCenter.Status.INTERRUPTED }

    // 进入页面即视为"看过这一屏" —— 清角标(设计规范:角标只负责把人引过来)。
    // 延迟 500ms 再清,避免刚进页面角标就闪没(设计规范 §8)。
    LaunchedEffect(Unit) {
        delay(500)
        TaskCenter.markAllRead()
    }
    // 停留期间新完成的任务直接算已读(设计规范 §8:"已看见不增加未读")
    DisposableEffect(Unit) {
        TaskCenter.setViewing(true)
        onDispose { TaskCenter.setViewing(false) }
    }

    Box(
        modifier
            .fillMaxSize()
            .background(Pa.Bg)
            // 左边缘右滑唤出侧边栏(问题 7)
            .edgeSwipeToOpenDrawer { showDrawer = true },
    ) {
        Column(Modifier.fillMaxSize().statusBarsPadding()) {
            // ---------- 顶栏:与其他页一致,App 图标 = 菜单入口 ----------
            Row(
                Modifier.fillMaxWidth().padding(start = 16.dp, end = 12.dp, top = 10.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Image(
                    bitmap = agentBitmap(R.mipmap.ic_launcher),
                    contentDescription = "菜单",
                    modifier = Modifier
                        .size(46.dp)
                        .pressable { showDrawer = true },
                    contentScale = ContentScale.Crop,
                )
                Text(
                    "任务中心",
                    Modifier.weight(1f).padding(start = 12.dp),
                    fontSize = 30.sp,
                    color = Pa.TextPrimary,
                )
                IconActionButton(R.drawable.ic_lucide_square_terminal, "终端", onOpenTerminal, bg = Pa.Card)
                Spacer(Modifier.width(8.dp))
                IconActionButton(R.drawable.ic_lucide_settings, "设置", onOpenSettings, bg = Pa.Card)
            }

            // ---------- 概览条 ----------
            Row(
                Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 16.dp, vertical = 12.dp)
                    .background(Pa.Card, RoundedCornerShape(18.dp))
                    .padding(horizontal = 14.dp, vertical = 12.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                StatCell("进行中", running.toString(), if (running > 0) Pa.Accent else Pa.TextTertiary, Modifier.weight(1f))
                Box(Modifier.width(1.dp).height(26.dp).background(Pa.Separator))
                StatCell("待处理", failed.toString(), if (failed > 0) Pa.Red else Pa.TextTertiary, Modifier.weight(1f))
                Box(Modifier.width(1.dp).height(26.dp).background(Pa.Separator))
                StatCell("全部", all.size.toString(), Pa.TextPrimary, Modifier.weight(1f))
            }

            // ---------- 分类切换 ----------
            Row(
                Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 16.dp)
                    .background(Pa.Card, RoundedCornerShape(50))
                    .padding(4.dp),
            ) {
                KindTab("消息处理", kindTab == TaskCenter.Kind.MESSAGE, Modifier.weight(1f)) {
                    kindTab = TaskCenter.Kind.MESSAGE
                }
                KindTab("文件下载", kindTab == TaskCenter.Kind.DOWNLOAD, Modifier.weight(1f)) {
                    kindTab = TaskCenter.Kind.DOWNLOAD
                }
            }

            Spacer(Modifier.height(6.dp))

            // ---------- 列表 ----------
            if (shown.isEmpty()) {
                EmptyState(
                    iconRes = R.drawable.ic_lucide_list_checks,
                    title = if (kindTab == TaskCenter.Kind.MESSAGE) "还没有处理中的任务" else "还没有下载任务",
                    hint = if (kindTab == TaskCenter.Kind.MESSAGE) {
                        "在会话里发一条消息,这里会显示它的进度"
                    } else {
                        "下载电脑上的文件后,进度会汇总到这里"
                    },
                )
            } else {
                LazyColumn(
                    Modifier.fillMaxSize(),
                    contentPadding = androidx.compose.foundation.layout.PaddingValues(
                        start = 16.dp, end = 16.dp, top = 4.dp,
                    ),
                    verticalArrangement = Arrangement.spacedBy(10.dp),
                ) {
                    items(shown, key = { it.id }) { task ->
                        TaskRow(
                            task = task,
                            onClick = {
                                TaskCenter.markRead(task.id)
                                if (task.kind == TaskCenter.Kind.MESSAGE && task.threadId.isNotEmpty()) {
                                    onOpenThread(task.threadId)
                                }
                            },
                            onRemove = { TaskCenter.remove(task.id) },
                            onRetry = {
                                // 中断/失败的消息类任务:回到会话里重发即可 —— 这里只做"引路"
                                if (task.threadId.isNotEmpty()) onOpenThread(task.threadId)
                            },
                        )
                    }
                    item { Spacer(Modifier.height(88.dp)) }
                }
            }
        }

        // ---------- 底部操作:清理已完成 ----------
        if (all.any { it.status != TaskCenter.Status.RUNNING }) {
            Box(
                Modifier
                    .align(Alignment.BottomCenter)
                    .fillMaxWidth()
                    .padding(horizontal = 16.dp, vertical = 20.dp),
            ) {
                Box(
                    Modifier
                        .fillMaxWidth()
                        .navigationBarsPadding()
                        .pressable(haptic = true) { TaskCenter.clearFinished() }
                        .background(Pa.CardHigh, RoundedCornerShape(16.dp))
                        .padding(vertical = 13.dp),
                    contentAlignment = Alignment.Center,
                ) {
                    Text("清理已完成的任务", fontSize = 14.sp, color = Pa.TextSecondary)
                }
            }
        }

        // ---------- 侧边栏(与其他 tab 一致) ----------
        if (showDrawer) {
            AppDrawer(
                currentTab = currentTab,
                hostTitle = "本机任务",
                onClose = { showDrawer = false },
                onGoTab = { onGoTab(it) },
                onOpenTerminal = onOpenTerminal,
                onOpenSettings = onOpenSettings,
                onOpenTaskCenter = {},
            )
        }
    }
}

@Composable
private fun StatCell(label: String, value: String, valueColor: Color, modifier: Modifier = Modifier) {
    Column(modifier, horizontalAlignment = Alignment.CenterHorizontally) {
        Text(value, fontSize = 19.sp, color = valueColor, fontWeight = FontWeight.SemiBold)
        Text(label, fontSize = 10.5.sp, color = Pa.TextTertiary)
    }
}

@Composable
private fun KindTab(label: String, selected: Boolean, modifier: Modifier = Modifier, onClick: () -> Unit) {
    Box(
        modifier
            .pressable(haptic = true) { onClick() }
            .background(if (selected) Pa.Accent else Color.Transparent, RoundedCornerShape(50))
            .padding(vertical = 9.dp),
        contentAlignment = Alignment.Center,
    ) {
        Text(
            label,
            fontSize = 13.sp,
            color = if (selected) Color.White else Pa.TextSecondary,
            fontWeight = if (selected) FontWeight.Medium else FontWeight.Normal,
        )
    }
}

/**
 * 单条任务行。
 * 进行中:高亮进度条 + 可点"打开会话";失败/中断:红色/黄色标记 + 重试入口。
 */
@Composable
private fun TaskRow(
    task: TaskCenter.Task,
    onClick: () -> Unit,
    onRemove: () -> Unit,
    onRetry: () -> Unit,
) {
    val accent = when (task.status) {
        TaskCenter.Status.RUNNING -> Pa.Accent
        TaskCenter.Status.DONE -> Pa.Green
        TaskCenter.Status.FAILED -> Pa.Red
        TaskCenter.Status.CANCELED -> Pa.TextTertiary
        TaskCenter.Status.INTERRUPTED -> Pa.Yellow
    }
    val icon = when (task.kind) {
        TaskCenter.Kind.MESSAGE -> R.drawable.ic_lucide_message_square
        TaskCenter.Kind.DOWNLOAD -> R.drawable.ic_lucide_arrow_up
    }
    val statusText = when (task.status) {
        TaskCenter.Status.RUNNING -> "进行中"
        TaskCenter.Status.DONE -> "已完成"
        TaskCenter.Status.FAILED -> "失败"
        TaskCenter.Status.CANCELED -> "已取消"
        TaskCenter.Status.INTERRUPTED -> "已中断"
    }

    Column(
        Modifier
            .fillMaxWidth()
            .pressable(haptic = true) { onClick() }
            .background(Pa.Card, RoundedCornerShape(18.dp))
            .padding(horizontal = 14.dp, vertical = 13.dp),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Box(
                Modifier.size(36.dp).background(Pa.CardHigh, RoundedCornerShape(11.dp)),
                contentAlignment = Alignment.Center,
            ) {
                Icon(painterResource(icon), null, tint = accent, modifier = Modifier.size(17.dp))
            }
            Spacer(Modifier.width(11.dp))
            Column(Modifier.weight(1f)) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text(
                        task.title.ifEmpty { "(未命名任务)" },
                        Modifier.weight(1f, fill = false),
                        fontSize = 14.5.sp,
                        color = Pa.TextPrimary,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                    if (task.rounds > 1) {
                        Spacer(Modifier.width(6.dp))
                        Text(
                            "${task.rounds} 轮",
                            Modifier
                                .background(Pa.AccentDim, RoundedCornerShape(50))
                                .padding(horizontal = 6.dp, vertical = 1.dp),
                            fontSize = 9.5.sp,
                            color = Pa.AccentText,
                        )
                    }
                }
                Spacer(Modifier.height(3.dp))
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Box(Modifier.size(5.dp).background(accent, CircleShape))
                    Spacer(Modifier.width(6.dp))
                    Text(statusText, fontSize = 10.5.sp, color = accent)
                    Text(
                        "  ·  " + SimpleDateFormat("MM-dd HH:mm").format(Date(task.startedAt)),
                        fontSize = 10.5.sp,
                        color = Pa.TextTertiary,
                    )
                    if (task.detail.isNotEmpty()) {
                        Text(
                            "  ·  " + task.detail,
                            fontSize = 10.5.sp,
                            color = Pa.TextTertiary,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                        )
                    }
                }
            }
            Spacer(Modifier.width(8.dp))
            when (task.status) {
                TaskCenter.Status.FAILED, TaskCenter.Status.INTERRUPTED ->
                    IconActionButton(
                        R.drawable.ic_lucide_refresh_cw, "重试", onRetry,
                        bg = Color.Transparent, size = 34.dp, iconSize = 17.dp, tint = Pa.Accent,
                    )
                else ->
                    IconActionButton(
                        R.drawable.ic_lucide_x, "移除", onRemove,
                        bg = Color.Transparent, size = 34.dp, iconSize = 15.dp, tint = Pa.TextTertiary,
                    )
            }
        }
        if (task.status == TaskCenter.Status.RUNNING) {
            Spacer(Modifier.height(10.dp))
            Box(
                Modifier
                    .fillMaxWidth()
                    .height(4.dp)
                    .background(Pa.Bg, RoundedCornerShape(50)),
            ) {
                // 进度未知时用一条 30% 的"呼吸条",避免 0% 看起来像卡死
                val frac = if (task.progress > 0f) task.progress.coerceIn(0.02f, 1f) else 0.3f
                Box(
                    Modifier
                        .fillMaxWidth(frac)
                        .fillMaxHeight()
                        .background(accent, RoundedCornerShape(50)),
                )
            }
        }
    }
}
