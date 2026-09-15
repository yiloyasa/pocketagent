package dev.pocketagent.app.ui

import androidx.activity.compose.BackHandler
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.MutableTransitionState
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInHorizontally
import androidx.compose.animation.slideOutHorizontally
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
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.AgentConnection
import dev.pocketagent.app.R
import dev.pocketagent.app.ui.anim.IconActionButton
import dev.pocketagent.app.ui.anim.pressable
import dev.pocketagent.app.ui.anim.rememberReduceMotion
import dev.pocketagent.app.ui.component.PaBadge
import dev.pocketagent.app.ui.component.TaskCenter
import dev.pocketagent.app.ui.theme.Pa

/** 侧边栏主导航项。 */
private data class DrawerEntry(
    val icon: Int,
    val title: String,
    val desc: String,
    val tab: Int,
)

/**
 * 全局侧边栏 —— 底部导航栏移除后的唯一主导航。
 *
 * 【滑出方向】面板贴左边缘,必须从**左侧**滑入(初始偏移 `-it`,整块先移到屏幕外),
 * 之前写成 `it / 3`(正值)会让面板从右侧"倒着"挤进来 —— 这就是"从相反方向弹出"的根因。
 *
 * 【退场】调用方仍是 `if (showDrawer) AppDrawer(...)`,组件一旦被摘掉就来不及播退场动画。
 * 所以这里自己管一个 `closing` 标志:关闭请求先把 targetState 置 false 播完退场,
 * 等 currentState 也归位后才回调 `onClose()` 让调用方卸载 —— 无需改任何调用点。
 * 顺带把"点了导航项要跳的页"挂到退场结束之后再执行,视觉上就是"先收抽屉、再换页"。
 */
@Composable
fun AppDrawer(
    currentTab: Int,
    hostTitle: String,
    onClose: () -> Unit,
    onGoTab: (Int) -> Unit,
    onOpenTerminal: () -> Unit,
    onOpenSettings: () -> Unit,
    onOpenTaskCenter: () -> Unit = {},
) {
    val reduceMotion = rememberReduceMotion()

    val visibleState = remember { MutableTransitionState(false) }
    var closing by remember { mutableStateOf(false) }
    var pendingAction by remember { mutableStateOf<(() -> Unit)?>(null) }

    /** 请求关闭;[then] 会在抽屉完全收起后执行(用它承载"跳转到某个页面")。 */
    fun requestClose(then: (() -> Unit)? = null) {
        if (closing) return
        pendingAction = then
        closing = true
        visibleState.targetState = false
    }

    LaunchedEffect(Unit) { visibleState.targetState = true }
    // 退场跑完(currentState 也变 false)才真正通知调用方卸载,并执行挂起的跳转
    LaunchedEffect(visibleState.currentState, visibleState.targetState) {
        if (closing && !visibleState.targetState && !visibleState.currentState) {
            onClose()
            pendingAction?.invoke()
            pendingAction = null
        }
    }

    // 返回键优先关侧边栏(修复"按返回直接退回桌面")
    BackHandler(enabled = true) { requestClose() }

    val shown = visibleState.targetState || visibleState.currentState
    val scrimAlpha by animateFloatAsState(
        targetValue = if (shown) 1f else 0f,
        animationSpec = tween(220),
        label = "drawerScrim",
    )

    val unread by TaskCenter.unreadCount.collectAsState()

    // 遮罩:铺满整个屏幕(全屏点击区),淡入淡出
    Box(
        Modifier
            .fillMaxSize()
            .background(Color.Black.copy(alpha = 0.62f * scrimAlpha))
            // scaleDown=1f:遮罩本身不做缩放,只当点击区
            .pressable(scaleDown = 1f) { requestClose() },
    ) {
        // 动效:减弱动态效果时退化为纯淡入(不移位)
        val enter = if (reduceMotion) fadeIn(tween(120))
        else fadeIn(tween(180)) + slideInHorizontally(tween(280)) { -it }
        val exit = if (reduceMotion) fadeOut(tween(100))
        else fadeOut(tween(160)) + slideOutHorizontally(tween(220)) { -it }

        AnimatedVisibility(
            visibleState = visibleState,
            enter = enter,
            exit = exit,
            modifier = Modifier
                .align(Alignment.CenterStart)
                .fillMaxHeight(),
        ) {
            DrawerPanel(
                currentTab = currentTab,
                hostTitle = hostTitle,
                unread = unread,
                onGoTab = { requestClose { onGoTab(it) } },
                onOpenTerminal = { requestClose { onOpenTerminal() } },
                onOpenSettings = { requestClose { onOpenSettings() } },
                onOpenTaskCenter = { requestClose { onOpenTaskCenter() } },
                onClose = { requestClose() },
            )
        }
    }
}

/**
 * 面板本体 —— 与遮罩解耦,方便单独调样式。
 *
 * 布局自上而下:品牌头(Logo + 名称 + 连接态胶囊) → 双快捷磁贴(终端 / 任务中心) →
 * 导航分组 → 更多分组 → 底部元信息(模型 + 版本)。
 */
@Composable
private fun DrawerPanel(
    currentTab: Int,
    hostTitle: String,
    unread: Int,
    onGoTab: (Int) -> Unit,
    onOpenTerminal: () -> Unit,
    onOpenSettings: () -> Unit,
    onOpenTaskCenter: () -> Unit,
    onClose: () -> Unit,
) {
    val connState = AgentConnection.connState.value
    val transport = AgentConnection.connTransport.value
    val connected = connState == "ok"

    val entries = listOf(
        DrawerEntry(R.drawable.ic_lucide_monitor, "主页", "agent 与最近任务", 0),
        DrawerEntry(R.drawable.ic_lucide_message_square, "会话", "全部任务会话", 1),
        DrawerEntry(R.drawable.ic_lucide_folder, "文件", "浏览电脑文件", 2),
        DrawerEntry(R.drawable.ic_lucide_git_branch, "Git", "工作区版本管理", 3),
    )

    Column(
        Modifier
            .fillMaxWidth(0.86f)
            .widthIn(max = 330.dp)
            .fillMaxHeight()
            .clip(RoundedCornerShape(topEnd = 30.dp, bottomEnd = 30.dp))
            .background(Pa.CardHigh)
            .statusBarsPadding()
            .navigationBarsPadding(),
    ) {
        Column(
            Modifier
                .fillMaxWidth()
                .weight(1f)
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 18.dp),
        ) {
            Spacer(Modifier.height(16.dp))

            // ---------- 品牌头 ----------
            Row(verticalAlignment = Alignment.CenterVertically) {
                // Logo 外圈带一圈 AccentDim 光环,提升"品牌位"的体量感
                Box(
                    Modifier
                        .size(54.dp)
                        .background(Pa.AccentDim, RoundedCornerShape(17.dp)),
                    contentAlignment = Alignment.Center,
                ) {
                    Image(
                        bitmap = agentBitmap(R.mipmap.ic_launcher),
                        contentDescription = null,
                        modifier = Modifier.size(44.dp).clip(RoundedCornerShape(13.dp)),
                        contentScale = ContentScale.Crop,
                    )
                }
                Spacer(Modifier.width(13.dp))
                Column(Modifier.weight(1f)) {
                    Text(
                        "PocketAgent",
                        fontSize = 18.sp,
                        color = Pa.TextPrimary,
                        fontWeight = FontWeight.SemiBold,
                    )
                    Spacer(Modifier.height(3.dp))
                    ConnectionPill(connected = connected, label = hostTitle, transport = transport)
                }
                IconActionButton(
                    R.drawable.ic_lucide_x, "关闭", onClose,
                    bg = Color.Transparent, size = 36.dp, iconSize = 18.dp,
                )
            }

            Spacer(Modifier.height(18.dp))

            // ---------- 快捷磁贴:终端 / 任务中心(带未读角标) ----------
            Row(
                Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(10.dp),
            ) {
                QuickTile(
                    icon = R.drawable.ic_lucide_square_terminal,
                    title = "终端",
                    hint = "真实 PTY",
                    modifier = Modifier.weight(1f),
                    onClick = onOpenTerminal,
                )
                QuickTile(
                    icon = R.drawable.ic_lucide_list_checks,
                    title = "任务中心",
                    hint = if (unread > 0) "$unread 条未处理" else "运行与下载",
                    badge = unread,
                    modifier = Modifier.weight(1f),
                    onClick = onOpenTaskCenter,
                )
            }

            // ---------- 导航 ----------
            DrawerGroupLabel("导航")
            entries.forEach { e ->
                DrawerItem(
                    icon = e.icon,
                    title = e.title,
                    desc = e.desc,
                    selected = e.tab == currentTab,
                    onClick = { onGoTab(e.tab) },
                )
            }

            // ---------- 更多 ----------
            DrawerGroupLabel("更多")
            DrawerItem(
                icon = R.drawable.ic_lucide_settings,
                title = "设置",
                desc = "连接 / 外观 / 权限",
                selected = false,
                onClick = onOpenSettings,
            )

            Spacer(Modifier.height(14.dp))
        }

        // ---------- 底部元信息 ----------
        Box(Modifier.fillMaxWidth().height(1.dp).background(Pa.Separator))
        Row(
            Modifier
                .fillMaxWidth()
                .padding(horizontal = 20.dp, vertical = 13.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            val model = AgentConnection.selectedModel
            Text(
                model.takeIf { it.isNotEmpty() } ?: "跟随桌面配置",
                Modifier.weight(1f),
                fontSize = 10.5.sp,
                color = Pa.TextTertiary,
                maxLines = 1,
            )
            Text("v0.1.0", fontSize = 10.5.sp, color = Pa.TextTertiary)
        }
    }
}

/**
 * 连接态胶囊:小圆点 + 文案。放在品牌头下方,让"我在连哪台电脑"一眼可见。
 * 已连接用绿点 + transport,未连接用灰点,错误用红点。
 */
@Composable
private fun ConnectionPill(connected: Boolean, label: String, transport: String) {
    val state = AgentConnection.connState.value
    val dot = when {
        state == "ok" -> Pa.Green
        state == "down" -> Pa.Red
        else -> Pa.TextTertiary
    }
    val text = when {
        connected && transport.isNotEmpty() -> "已连接 · $transport"
        connected -> "已连接 · $label"
        state == "down" -> "未连接 · 自动重连中"
        else -> label
    }
    Row(
        Modifier
            .clip(RoundedCornerShape(50))
            .background(Pa.Card)
            .padding(horizontal = 8.dp, vertical = 3.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box(Modifier.size(6.dp).background(dot, CircleShape))
        Spacer(Modifier.width(6.dp))
        Text(
            text,
            fontSize = 10.sp,
            color = Pa.TextSecondary,
            maxLines = 1,
        )
    }
}

/** 快捷磁贴:图标 + 标题 + 小字,右上可挂角标。 */
@Composable
private fun QuickTile(
    icon: Int,
    title: String,
    hint: String,
    modifier: Modifier = Modifier,
    badge: Int = 0,
    onClick: () -> Unit,
) {
    Column(
        modifier
            .pressable(haptic = true) { onClick() }
            .background(Pa.Card, RoundedCornerShape(16.dp))
            .padding(horizontal = 12.dp, vertical = 11.dp),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            if (badge > 0) {
                PaBadge(count = badge) {
                    Icon(
                        painterResource(icon), null,
                        tint = Pa.Accent, modifier = Modifier.size(17.dp),
                    )
                }
            } else {
                Icon(
                    painterResource(icon), null,
                    tint = Pa.Accent, modifier = Modifier.size(17.dp),
                )
            }
        }
        Spacer(Modifier.height(9.dp))
        Text(title, fontSize = 13.sp, color = Pa.TextPrimary, fontWeight = FontWeight.Medium)
        Text(hint, fontSize = 10.sp, color = Pa.TextTertiary, maxLines = 1)
    }
}

/** 分组小标题。 */
@Composable
private fun DrawerGroupLabel(text: String) {
    Text(
        text,
        Modifier.padding(start = 10.dp, top = 18.dp, bottom = 6.dp),
        fontSize = 10.5.sp,
        color = Pa.TextTertiary,
        fontWeight = FontWeight.Medium,
    )
}

/**
 * 导航行:左侧图标块 + 标题/描述 + 右侧选中圆点。
 * 选中态:左侧一条 3dp 的高亮竖条(位置锚定感强于纯色块)+ 蓝底图标 + 蓝字。
 */
@Composable
private fun DrawerItem(
    icon: Int,
    title: String,
    desc: String,
    selected: Boolean,
    onClick: () -> Unit,
) {
    val highlight by animateFloatAsState(
        targetValue = if (selected) 1f else 0f,
        animationSpec = tween(220),
        label = "drawerSelected",
    )
    Box(Modifier.fillMaxWidth().padding(vertical = 1.dp)) {
        // 选中竖条:贴着面板左边缘,随选中态渐显
        Box(
            Modifier
                .align(Alignment.CenterStart)
                .width(3.dp)
                .height(20.dp)
                .alpha(highlight)
                .background(Pa.Accent, RoundedCornerShape(50)),
        )
        Row(
            Modifier
                .fillMaxWidth()
                .pressable(haptic = true) { onClick() }
                .background(
                    if (selected) Pa.Hover else Color.Transparent,
                    RoundedCornerShape(14.dp),
                )
                .padding(start = 13.dp, end = 10.dp, top = 10.dp, bottom = 10.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Box(
                Modifier
                    .size(36.dp)
                    .background(if (selected) Pa.AccentDim else Pa.Card, RoundedCornerShape(11.dp)),
                contentAlignment = Alignment.Center,
            ) {
                Icon(
                    painterResource(icon), null,
                    tint = if (selected) Pa.Accent else Pa.TextSecondary,
                    modifier = Modifier.size(17.dp),
                )
            }
            Spacer(Modifier.width(12.dp))
            Column(Modifier.weight(1f)) {
                Text(
                    title, fontSize = 14.5.sp,
                    color = if (selected) Pa.Accent else Pa.TextPrimary,
                    fontWeight = if (selected) FontWeight.SemiBold else FontWeight.Normal,
                )
                Text(desc, fontSize = 10.5.sp, color = Pa.TextTertiary)
            }
            Box(
                Modifier
                    .size(6.dp)
                    .alpha(highlight)
                    .background(Pa.Accent, CircleShape),
            )
        }
    }
}
