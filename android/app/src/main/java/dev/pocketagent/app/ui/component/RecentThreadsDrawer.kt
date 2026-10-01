package dev.pocketagent.app.ui.component

import androidx.compose.animation.core.Animatable
import androidx.compose.foundation.background
import androidx.compose.foundation.Image
import androidx.compose.foundation.clickable
import androidx.compose.foundation.gestures.detectHorizontalDragGestures
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.R
import dev.pocketagent.app.ui.agentBadgeRes
import dev.pocketagent.app.ui.agentBitmap
import dev.pocketagent.app.ui.anim.IconActionButton
import dev.pocketagent.app.ui.anim.PaMotion
import dev.pocketagent.app.ui.anim.pressable
import dev.pocketagent.app.ui.selectedAgentLabel
import dev.pocketagent.app.ui.theme.Pa
import dev.pocketagent.app.ui.theme.PaEditorial
import dev.pocketagent.app.ui.theme.paGlass
import dev.pocketagent.app.ui.threadAgentIdOf
import pocketagent.v1.Thread
import pocketagent.v1.ThreadStatus

/**
 * 「最近会话」侧边栏 —— 会话页左上角菜单与左边缘右滑唤出的就是这个。
 *
 * 【为什么不是全局导航抽屉】在会话里点菜单,用户的第一诉求是"换一个会话接着聊",
 * 而不是看到"主页/文件/Git"这层全局导航;所以这里给的是最近会话(带搜索与最近使用时间),
 * 底部保留一排全局入口,等于把两件事放在同一个侧边栏里,不用来回跳。
 *
 * 【卡顿修复】上一版抽屉在展开时才发网络请求,期间一直是空列表、看起来"卡住";
 * 现在数据在打开前就由调用方预热(见 ChatScreen 的 recentThreads),
 * 动画本身改成 Animatable + spring,位移与遮罩同帧推进,不再出现"先卡后滑"。
 */
@Composable
fun RecentThreadsDrawer(
    currentThreadId: String,
    threads: List<Thread>,
    loading: Boolean,
    onClose: () -> Unit,
    onOpenThread: (String) -> Unit,
    onOpenAllThreads: () -> Unit,
    onOpenTerminal: () -> Unit,
    onOpenTaskCenter: () -> Unit,
    onOpenSettings: () -> Unit,
    /** 跟手进度(0..1)。由页面手势驱动;传了就不播内部入场动画。 */
    dragProgress: Float? = null,
    /** 面板内水平拖动:正数=向左拖(关闭),交给页面按进度吸附。 */
    onDragClose: ((Float) -> Unit)? = null,
    onReleaseClose: ((Boolean) -> Unit)? = null,
) {
    val config = LocalConfiguration.current
    val panelWidth = (config.screenWidthDp * 0.84f).coerceAtMost(340f).dp
    val panelWidthPx = with(LocalDensity.current) { panelWidth.toPx() }

    // 跟手时进度由页面给;没有外部进度时自己播入场动画(保持组件可独立使用)。
    // 【令牌化】入场用 PaMotion.sheet(大面积位移的从容 spring,可打断)。
    // 【draw 层读取】外部进度经 rememberUpdatedState 进 graphicsLayer 块:透明度与
    // 位移都在 draw 阶段拿最新值,不依赖"每帧重组"(动画中段丢一帧重组也不跳变)。
    val extProgress = rememberUpdatedState(dragProgress)
    val inner = remember { Animatable(0f) }
    LaunchedEffect(Unit) {
        if (dragProgress == null) inner.animateTo(1f, PaMotion.sheet())
    }

    var query by remember { mutableStateOf("") }
    val filtered = remember(threads, query) {
        val q = query.trim()
        val base = threads.filter { it.status != ThreadStatus.THREAD_STATUS_ARCHIVED }
        if (q.isEmpty()) base else base.filter {
            it.title.contains(q, ignoreCase = true) || it.workspace.contains(q, ignoreCase = true)
        }
    }
    // 任务角标:与全局抽屉同一事实来源(TaskCenter 未读数 = 待处理任务),
    // 不自造数字 —— 角标口径全 app 唯一(README:任务角标完全相同)。
    val unread by TaskCenter.unreadCount.collectAsState()

    Box(Modifier.fillMaxSize()) {
        // 遮罩:点一下关闭。压暗色统一走 PaEditorial.Scrim —— 与全局抽屉 AppDrawer
        // 真正同口径(此前注释声称同口径、实际用的是玻璃令牌的黑,冷色层次不一致)。
        Box(
            Modifier
                .fillMaxSize()
                .graphicsLayer { alpha = extProgress.value ?: inner.value }
                .background(PaEditorial.Scrim)
                .clickable { onClose() },
        )

        Column(
            Modifier
                .width(panelWidth)
                .fillMaxHeight()
                .graphicsLayer {
                    val p = extProgress.value ?: inner.value
                    translationX = -panelWidthPx * (1f - p)
                }
                // 【玻璃面板】与全局抽屉 AppDrawer 同一材质语言:半透明烟玻璃 + 右缘投影;
                // 发丝线关(上下缘贴屏幕边),被覆盖内容的推背模糊由调用方(ChatScreen)负责
                .paGlass(
                    RoundedCornerShape(topEnd = 24.dp, bottomEnd = 24.dp),
                    hairline = false,
                    shadowElevation = 10.dp,
                )
                .clip(RoundedCornerShape(topEnd = 24.dp, bottomEnd = 24.dp))
                // 面板打开后在面板上向左拖也能跟手关闭(与边缘手势共用同一进度)
                .pointerInput(Unit) {
                    detectHorizontalDragGestures(
                        onDragEnd = { onReleaseClose?.invoke(false) },
                        onHorizontalDrag = { change, dragAmount ->
                            change.consume()
                            onDragClose?.invoke(dragAmount / panelWidthPx)
                        },
                    )
                }
                .statusBarsPadding()
                .navigationBarsPadding(),
        ) {
            // ---------- 头部 ----------
            Row(
                Modifier.fillMaxWidth().padding(start = 16.dp, end = 8.dp, top = 12.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text("最近会话", Modifier.weight(1f), fontSize = 17.sp, color = PaEditorial.Ink, fontWeight = FontWeight.SemiBold)
                Text(
                    "全部会话",
                    Modifier
                        .pressable(haptic = true) { onOpenAllThreads() }
                        .padding(horizontal = 8.dp, vertical = 6.dp),
                    fontSize = 12.sp,
                    color = PaEditorial.AccentText,
                )
                IconActionButton(
                    R.drawable.ic_lucide_x, "关闭", onClose,
                    bg = Color.Transparent, size = 34.dp, iconSize = 17.dp,
                )
            }

            // ---------- 搜索 ----------
            Row(
                Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 16.dp, vertical = 8.dp)
                    .background(PaEditorial.Canvas, RoundedCornerShape(12.dp))
                    .padding(horizontal = 12.dp, vertical = 8.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Icon(
                    painterResource(R.drawable.ic_lucide_search), null,
                    tint = PaEditorial.Subtle, modifier = Modifier.size(14.dp),
                )
                Spacer(Modifier.width(8.dp))
                Box(Modifier.weight(1f)) {
                    if (query.isEmpty()) {
                        Text("搜索会话标题或工作区", fontSize = 12.5.sp, color = PaEditorial.Subtle)
                    }
                    BasicTextField(
                        value = query,
                        onValueChange = { query = it },
                        singleLine = true,
                        textStyle = TextStyle(color = PaEditorial.Ink, fontSize = 12.5.sp),
                        cursorBrush = SolidColor(PaEditorial.Primary),
                        modifier = Modifier.fillMaxWidth(),
                    )
                }
                if (query.isNotEmpty()) {
                    Icon(
                        painterResource(R.drawable.ic_lucide_x), "清空",
                        tint = PaEditorial.Subtle,
                        modifier = Modifier
                            .size(14.dp)
                            .clickable { query = "" },
                    )
                }
            }

            // ---------- 列表(行规格与「全部会话」主列表同语言:agent 身份图 + 标题 + 元信息行) ----------
            when {
                loading && threads.isEmpty() -> PaSkeletonList(
                    kind = SkeletonKind.SESSION_ROW,
                    modifier = Modifier.padding(horizontal = 16.dp),
                )
                filtered.isEmpty() -> Row(
                    Modifier
                        .padding(horizontal = 16.dp, vertical = 16.dp)
                        .heightIn(min = 44.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Icon(
                        painterResource(R.drawable.ic_lucide_message_square), null,
                        tint = PaEditorial.Subtle, modifier = Modifier.size(20.dp),
                    )
                    Spacer(Modifier.width(12.dp))
                    Column {
                        Text(
                            if (query.isEmpty()) "还没有会话" else "没有匹配的会话",
                            fontSize = 13.sp,
                            color = PaEditorial.Muted,
                        )
                        if (query.isEmpty()) {
                            Spacer(Modifier.height(2.dp))
                            Text(
                                "到「全部会话」新建一个,这里就会跟着更新",
                                fontSize = 11.sp,
                                color = PaEditorial.Subtle,
                            )
                        }
                    }
                }
                else -> LazyColumn(
                    Modifier.weight(1f).fillMaxWidth(),
                    verticalArrangement = Arrangement.spacedBy(8.dp),
                    contentPadding = androidx.compose.foundation.layout.PaddingValues(
                        start = 16.dp, end = 16.dp, bottom = 8.dp,
                    ),
                ) {
                    items(filtered, key = { it.id }) { th ->
                        val active = th.id == currentThreadId
                        Row(
                            Modifier
                                .fillMaxWidth()
                                .heightIn(min = 44.dp)
                                .pressable(haptic = true) { onOpenThread(th.id) }
                                .background(
                                    if (active) PaEditorial.PrimaryContainer else Color.Transparent,
                                    RoundedCornerShape(12.dp),
                                )
                                .padding(horizontal = 12.dp, vertical = 12.dp),
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            // agent 身份图:与主列表(ThreadCard)同规格 —— 42dp 方格、13dp 圆角、
                            // 按 agent_id 选图(缺省 codex)。进会话前就能认出"这是谁干的"。
                            Image(
                                bitmap = agentBitmap(agentBadgeRes(threadAgentIdOf(th.agentId))),
                                contentDescription = null,
                                modifier = Modifier
                                    .size(PaEditorial.Space.IconBox)
                                    .clip(RoundedCornerShape(13.dp)),
                                contentScale = ContentScale.Crop,
                            )
                            Spacer(Modifier.width(12.dp))
                            Column(Modifier.weight(1f)) {
                                Text(
                                    th.title.ifEmpty { "(未命名)" },
                                    fontSize = 15.sp, lineHeight = 21.sp,
                                    fontWeight = FontWeight(650),
                                    color = if (active) PaEditorial.AccentText else PaEditorial.Ink,
                                    maxLines = 1,
                                    overflow = TextOverflow.Ellipsis,
                                )
                                Spacer(Modifier.height(2.dp))
                                // 元信息行:归属 agent · 工作区 · 最近使用(与主列表同口径;
                                // 时间用相对时间——抽屉的诉求是"接着聊最近的",今天/昨天一眼可辨)
                                Text(
                                    buildList {
                                        add(selectedAgentLabel(threadAgentIdOf(th.agentId)))
                                        if (th.workspace.isNotEmpty()) add(shortWorkspace(th.workspace))
                                        add(relativeTime(th.updatedAtUnixMs))
                                    }.filter { it.isNotEmpty() }.joinToString("  ·  "),
                                    fontSize = 11.sp, lineHeight = 16.sp,
                                    color = PaEditorial.Subtle,
                                    maxLines = 1,
                                    overflow = TextOverflow.Ellipsis,
                                )
                            }
                        }
                    }
                }
            }

            // ---------- 底部全局入口(任务角标 = 真实待处理数,与全局抽屉同源) ----------
            Box(Modifier.fillMaxWidth().height(1.dp).background(PaEditorial.Hairline))
            Row(
                Modifier.fillMaxWidth().padding(horizontal = 12.dp, vertical = 8.dp),
                horizontalArrangement = Arrangement.SpaceEvenly,
            ) {
                DrawerFooterAction(R.drawable.ic_lucide_monitor, "主页", onOpenAllThreads)
                DrawerFooterAction(R.drawable.ic_lucide_square_terminal, "终端", onOpenTerminal)
                DrawerFooterAction(R.drawable.ic_lucide_list_checks, "任务", onOpenTaskCenter, badge = unread)
                DrawerFooterAction(R.drawable.ic_lucide_settings, "设置", onOpenSettings)
            }
        }
    }
}

/**
 * 底部图标钮:图标 + 词,双通道;[badge] > 0 时图标右上叠任务角标 —— 数字来自
 * TaskCenter,规格(琥珀底/近白字/9sp)与全局抽屉的 DrawerBadge 同一档。
 */
@Composable
private fun DrawerFooterAction(icon: Int, label: String, onClick: () -> Unit, badge: Int = 0) {
    Column(
        Modifier
            .pressable(haptic = true) { onClick() }
            .padding(horizontal = 10.dp, vertical = 4.dp)
            .heightIn(min = 44.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Box(contentAlignment = Alignment.Center) {
            Icon(
                painterResource(icon), label,
                tint = PaEditorial.Muted,
                modifier = Modifier.size(18.dp),
            )
            if (badge > 0) {
                Box(
                    Modifier
                        .align(Alignment.TopEnd)
                        .background(PaEditorial.Attention, RoundedCornerShape(7.dp))
                        .padding(horizontal = 4.dp, vertical = 1.dp),
                ) {
                    Text(
                        if (badge > 99) "99+" else badge.toString(),
                        fontSize = 9.sp, lineHeight = 11.sp,
                        color = PaEditorial.OnGraphite, fontWeight = FontWeight(650),
                    )
                }
            }
        }
        Spacer(Modifier.height(4.dp))
        Text(label, fontSize = 10.sp, color = PaEditorial.Subtle)
    }
}

/** 工作区只显示末尾一段(带盘符),避免长路径挤掉标题。 */
fun shortWorkspace(path: String): String {
    val norm = path.trimEnd('/', '\\')
    val sep = maxOf(norm.lastIndexOf('/'), norm.lastIndexOf('\\'))
    if (sep <= 0) return norm
    val last = norm.substring(sep + 1)
    val head = norm.substring(0, sep)
    val headSep = maxOf(head.lastIndexOf('/'), head.lastIndexOf('\\'))
    return if (headSep >= 0) head.substring(headSep + 1) + "\\" + last else head + "\\" + last
}

/** 相对时间:1 分钟内=刚刚 / 1 小时内=x 分钟前 / 今天=HH:mm / 昨天 / 更早=MM-dd。 */
fun relativeTime(unixMs: Long): String {
    if (unixMs <= 0) return ""
    val now = System.currentTimeMillis()
    val diff = now - unixMs
    val cal = java.util.Calendar.getInstance()
    val target = java.util.Calendar.getInstance().apply { timeInMillis = unixMs }
    val sameDay = cal.get(java.util.Calendar.YEAR) == target.get(java.util.Calendar.YEAR) &&
        cal.get(java.util.Calendar.DAY_OF_YEAR) == target.get(java.util.Calendar.DAY_OF_YEAR)
    val fmt = java.text.SimpleDateFormat("HH:mm", java.util.Locale.getDefault())
    val fmtDate = java.text.SimpleDateFormat("MM-dd", java.util.Locale.getDefault())
    return when {
        diff < 60_000 -> "刚刚"
        diff < 3_600_000 -> "${diff / 60_000} 分钟前"
        sameDay -> "今天 ${fmt.format(java.util.Date(unixMs))}"
        diff < 48 * 3_600_000L -> "昨天 ${fmt.format(java.util.Date(unixMs))}"
        else -> fmtDate.format(java.util.Date(unixMs))
    }
}
