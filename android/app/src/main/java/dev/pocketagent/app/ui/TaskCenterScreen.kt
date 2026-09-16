package dev.pocketagent.app.ui

import androidx.activity.compose.BackHandler
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
import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.pager.HorizontalPager
import androidx.compose.foundation.pager.rememberPagerState
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
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.platform.LocalDensity
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
import dev.pocketagent.app.ui.component.PaDownloader
import dev.pocketagent.app.ui.component.TaskCenter
import dev.pocketagent.app.ui.component.fileKindOf
import dev.pocketagent.app.ui.component.humanSize
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
    onOpenBrowser: (String) -> Unit = {},
    /** 下载类任务点卡片本体 → 跳「用户数据管理」(文件的具体管理都在那)。 */
    onOpenUserData: () -> Unit = {},
    /** 返回层级:二级页面按返回 → 回主页并展开侧边栏。 */
    onBackToHome: () -> Unit = {},
    modifier: Modifier = Modifier,
) {
    var kindTab by remember { mutableStateOf(TaskCenter.Kind.MESSAGE) }
    var showDrawer by remember { mutableStateOf(false) }
    // 侧边栏跟手:与主页/会话页同一套 controller
    val drawerCtrl = dev.pocketagent.app.ui.component.rememberDrawerController()
    BackHandler(enabled = true) {
        if (drawerCtrl.visible) drawerCtrl.close() else onBackToHome()
    }

    val all by TaskCenter.tasks.collectAsState()
    val ctx = androidx.compose.ui.platform.LocalContext.current
    // 旧版本下载条目无 localUri:进任务中心时按文件名找回本地路径,
    // DONE 行的「打开文件」才能正常走 openLocalFile(列表元素原地改+persist 重发射,
    // List.equals 按元素比较 → LaunchedEffect 不会因此重启,legacyTried 双保险防重查)。
    val legacyTried = remember { mutableSetOf<String>() }
    LaunchedEffect(all) {
        for (t in all) {
            if (t.kind != TaskCenter.Kind.DOWNLOAD || t.status != TaskCenter.Status.DONE) continue
            if (t.localUri.isNotEmpty() || !legacyTried.add(t.id)) continue
            val uri = kotlinx.coroutines.withContext(kotlinx.coroutines.Dispatchers.IO) {
                dev.pocketagent.app.ui.component.PaDownloader.resolveLocalUri(ctx, t.title)
            }
            if (uri.isNotEmpty()) TaskCenter.attachLocalUriIfEmpty(t.id, uri)
        }
    }

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
            .edgeSwipeToOpenDrawer(
                        onDragFraction = drawerCtrl::onDrag,
                        onRelease = drawerCtrl::onRelease,
                    ),
    ) {
        Column(Modifier.fillMaxSize().statusBarsPadding()) {
            // ---------- 顶栏:与其他页一致,App 图标 = 菜单入口 ----------
            Row(
                Modifier.fillMaxWidth().padding(start = 16.dp, end = 12.dp, top = 10.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                IconActionButton(R.drawable.ic_lucide_menu, "菜单", { drawerCtrl.open() }, bg = Pa.Card)
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

            // ---------- 两个窗口:左右滑动切换(与顶部 chip 双向联动) ----------
            val pager = rememberPagerState(pageCount = { 2 })
            LaunchedEffect(kindTab) {
                val target = if (kindTab == TaskCenter.Kind.MESSAGE) 0 else 1
                if (pager.currentPage != target) pager.animateScrollToPage(target)
            }
            LaunchedEffect(pager.currentPage) {
                val k = if (pager.currentPage == 0) TaskCenter.Kind.MESSAGE else TaskCenter.Kind.DOWNLOAD
                if (kindTab != k) kindTab = k
            }

            HorizontalPager(
                state = pager,
                modifier = Modifier.fillMaxSize(),
            ) { page ->
                val pageKind = if (page == 0) TaskCenter.Kind.MESSAGE else TaskCenter.Kind.DOWNLOAD
                val rows = all.filter { it.kind == pageKind }
                if (rows.isEmpty()) {
                    EmptyState(
                        iconRes = R.drawable.ic_lucide_list_checks,
                        title = if (pageKind == TaskCenter.Kind.MESSAGE) "还没有处理中的任务" else "还没有下载任务",
                        hint = if (pageKind == TaskCenter.Kind.MESSAGE) {
                            "在会话里发一条消息,这里会显示它的进度;左右滑动可切到文件下载"
                        } else {
                            "在会话里点文件旁的下载,进度会汇总到这里;左右滑动可切回消息处理"
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
                        items(rows, key = { it.id }) { task ->
                            TaskRow(
                                task = task,
                                onClick = {
                                    TaskCenter.markRead(task.id)
                                    when {
                                        // 消息处理 → 回到那个会话
                                        task.kind == TaskCenter.Kind.MESSAGE && task.threadId.isNotEmpty() ->
                                            onOpenThread(task.threadId)
                                        // 下载项 → 点整行进「用户数据管理」(查看/分享/删除都在那),
                                        // 右上角的地球图标才是"直接打开文件"
                                        task.kind == TaskCenter.Kind.DOWNLOAD -> onOpenUserData()
                                    }
                                },
                                onRemove = { TaskCenter.remove(task.id) },
                                onRetry = {
                                    // 中断/失败的消息类任务:不只是"引路"—— 先从电脑端把
                                    // 已完成的结果同步回来(找到对应消息并输出到手机端),再进会话
                                    if (task.kind == TaskCenter.Kind.MESSAGE && task.threadId.isNotEmpty()) {
                                        TurnSession.resyncFromDaemon(task.threadId)
                                        onOpenThread(task.threadId)
                                    }
                                },
                                onOpenFile = {
                                    // 网页/文本/图片 → 内置浏览器;APK → 系统安装器
                                    PaDownloader.openLocalFile(
                                        ctx = ctx,
                                        uri = task.localUri,
                                        title = task.title,
                                        detail = task.detail,
                                        onOpenInBrowser = onOpenBrowser,
                                    )
                                },
                            )
                        }
                        item { Spacer(Modifier.height(88.dp)) }
                    }
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
        if (drawerCtrl.visible) {
            AppDrawer(
                currentTab = currentTab,
                hostTitle = "本机任务",
                progress = drawerCtrl.progress,
            onClose = { drawerCtrl.close() },
                onGoTab = { onGoTab(it) },
                onOpenTerminal = onOpenTerminal,
                onOpenSettings = onOpenSettings,
                onOpenTaskCenter = {},
                onOpenBrowser = { drawerCtrl.close(); onOpenBrowser("") },
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
    onOpenFile: (() -> Unit)? = null,
) {
    val accent = when (task.status) {
        TaskCenter.Status.RUNNING -> Pa.Accent
        TaskCenter.Status.DONE -> Pa.Green
        TaskCenter.Status.FAILED -> Pa.Red
        TaskCenter.Status.CANCELED -> Pa.TextTertiary
        TaskCenter.Status.INTERRUPTED -> Pa.Yellow
    }
    // 下载项按文件类型着色并标出类型(用户反馈:只显示"2.0MB"看不出这是什么文件)
    val kind = if (task.kind == TaskCenter.Kind.DOWNLOAD) fileKindOf(task.title) else null
    val icon = when (task.kind) {
        TaskCenter.Kind.MESSAGE -> R.drawable.ic_lucide_message_square
        // 【按文件类型出图】下载项不再一律用"下载"图标 —— 用户一眼要认出这是表格/网页/安装包
        TaskCenter.Kind.DOWNLOAD -> kind?.iconRes ?: R.drawable.ic_lucide_download
    }
    val iconTint = kind?.color ?: accent
    val iconBg = kind?.color?.copy(alpha = 0.16f) ?: Pa.CardHigh
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
                Modifier.size(36.dp).background(iconBg, RoundedCornerShape(11.dp)),
                contentAlignment = Alignment.Center,
            ) {
                Icon(painterResource(icon), null, tint = iconTint, modifier = Modifier.size(17.dp))
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
                    if (kind != null) {
                        Spacer(Modifier.width(6.dp))
                        Text(
                            kind.label,
                            Modifier
                                .background(kind.color.copy(alpha = 0.18f), RoundedCornerShape(50))
                                .padding(horizontal = 6.dp, vertical = 1.dp),
                            fontSize = 9.5.sp,
                            color = kind.color,
                        )
                    }
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
                    if (task.sourceTitle.isNotEmpty()) {
                        Text(
                            "  ·  来自会话「" + task.sourceTitle + "」",
                            fontSize = 10.5.sp,
                            color = Pa.TextTertiary,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                        )
                    } else if (task.detail.isNotEmpty()) {
                        Text(
                            "  ·  " + task.detail,
                            fontSize = 10.5.sp,
                            color = Pa.TextTertiary,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                        )
                    }
                }
                if (task.sizeText.isNotEmpty() || task.detail.isNotEmpty() || task.localUri.isNotEmpty()) {
                    Spacer(Modifier.height(2.dp))
                    Text(
                        listOf(
                            task.sizeText.ifEmpty { task.sizeText },
                            // detail 形如「已保存到「下载/PocketAgent/xxx」」,比 content:// 友好
                            task.detail.ifEmpty { task.localUri },
                        ).filter { it.isNotEmpty() }
                            .joinToString("  ·  "),
                        fontSize = 9.5.sp,
                        color = Pa.TextTertiary,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                }
            }
            Spacer(Modifier.width(8.dp))
            when {
                task.status == TaskCenter.Status.DONE &&
                    task.kind == TaskCenter.Kind.DOWNLOAD && onOpenFile != null ->
                    IconActionButton(
                        R.drawable.ic_lucide_globe, "打开文件", onOpenFile,
                        bg = Color.Transparent, size = 34.dp, iconSize = 17.dp, tint = Pa.Accent,
                    )
                task.status == TaskCenter.Status.FAILED ||
                    task.status == TaskCenter.Status.INTERRUPTED ->
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
            BoxWithConstraints(
                Modifier
                    .fillMaxWidth()
                    .height(4.dp)
                    // 【修复光线外溢】扫描高亮用 translationX 移动,默认不被父容器裁剪,
                    // 高亮会画出进度条、压到卡片文字。clip 之后光带只在 4dp 轨道内扫。
                    .clip(RoundedCornerShape(50))
                    .background(Pa.Bg, RoundedCornerShape(50)),
            ) {
                if (task.progress > 0f) {
                    // 真实进度:任务层保证单调不减,不会出现"前进又倒退"
                    Box(
                        Modifier
                            .fillMaxWidth(task.progress.coerceIn(0.02f, 1f))
                            .fillMaxHeight()
                            .background(accent, RoundedCornerShape(50)),
                    )
                } else {
                    // 不确定进度:一道 40% 宽的高光匀速扫过(左→右循环,两端羽化),
                    // 上一版"实心条来回弹"太像加载失败,这一版更接近系统级进度语言
                    val trackPx = with(LocalDensity.current) { maxWidth.toPx() }
                    val transition = rememberInfiniteTransition(label = "indeterminate")
                    val sweep by transition.animateFloat(
                        initialValue = 0f,
                        targetValue = 1f,
                        animationSpec = infiniteRepeatable(
                            animation = tween(1500, easing = LinearEasing),
                            repeatMode = RepeatMode.Restart,
                        ),
                        label = "sweep",
                    )
                    Box(
                        Modifier
                            .fillMaxWidth(0.40f)
                            .fillMaxHeight()
                            .graphicsLayer {
                                translationX = trackPx * 1.4f * sweep - trackPx * 0.40f
                            }
                            .background(
                                androidx.compose.ui.graphics.Brush.horizontalGradient(
                                    listOf(
                                        accent.copy(alpha = 0f),
                                        accent.copy(alpha = 0.9f),
                                        accent.copy(alpha = 0f),
                                    ),
                                ),
                                RoundedCornerShape(50),
                            ),
                    )
                }
            }
        }
    }
}
