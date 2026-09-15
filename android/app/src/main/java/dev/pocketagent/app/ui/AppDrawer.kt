package dev.pocketagent.app.ui

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.MutableTransitionState
import androidx.compose.animation.core.tween
import androidx.compose.animation.slideInHorizontally
import androidx.compose.animation.slideOutHorizontally
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.R
import dev.pocketagent.app.ui.anim.IconActionButton
import dev.pocketagent.app.ui.anim.pressable
import dev.pocketagent.app.ui.theme.Pa

/** 侧边栏功能项。 */
data class DrawerEntry(val icon: Int, val title: String, val desc: String, val tab: Int)

/**
 * 全局侧边栏 —— 底部导航栏移除后的唯一主导航:
 * 左上角 App 图标(46dp,与主页顶栏一致)呼出,承载 终端/主页/会话/文件/Git/设置。
 */
@Composable
fun AppDrawer(
    currentTab: Int,
    hostTitle: String,
    onClose: () -> Unit,
    onGoTab: (Int) -> Unit,
    onOpenTerminal: () -> Unit,
    onOpenSettings: () -> Unit,
) {
    val visibleState = remember { MutableTransitionState(false) }
    visibleState.targetState = true

    val entries = listOf(
        DrawerEntry(R.drawable.ic_lucide_monitor, "主页", "agent 与最近任务", 0),
        DrawerEntry(R.drawable.ic_lucide_message_square, "会话", "全部任务会话", 1),
        DrawerEntry(R.drawable.ic_lucide_folder, "文件", "浏览电脑文件", 2),
        DrawerEntry(R.drawable.ic_lucide_git_branch, "Git", "工作区版本管理", 3),
    )

    Box(
        Modifier
            .fillMaxSize()
            .background(Color.Black.copy(alpha = 0.55f))
            .pressable { onClose() },
    ) {
        AnimatedVisibility(
            visible = visibleState.targetState,
            enter = slideInHorizontally(tween(260)) { it / 3 },
            exit = slideOutHorizontally(tween(200)) { it / 3 },
            modifier = Modifier
                .align(Alignment.CenterStart)
                .fillMaxHeight(),
        ) {
            Column(
            Modifier
                .fillMaxWidth(0.82f)
                .widthIn(max = 292.dp)
                    .fillMaxHeight()
                    .background(Pa.CardHigh, RoundedCornerShape(topEnd = 26.dp, bottomEnd = 26.dp))
                    .statusBarsPadding()
                    .navigationBarsPadding()
                    .padding(start = 20.dp, end = 16.dp),
            ) {
                Spacer(Modifier.height(18.dp))
                // 品牌行:App 图标与主页顶栏同尺寸(46dp)
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Image(
                        bitmap = agentBitmap(R.mipmap.ic_launcher),
                        contentDescription = null,
                        modifier = Modifier.size(46.dp),
                        contentScale = ContentScale.Crop,
                    )
                    Spacer(Modifier.width(16.dp))
                    Column(Modifier.weight(1f)) {
                        Text("PocketAgent", fontSize = 17.sp, color = Pa.TextPrimary, fontWeight = FontWeight.SemiBold)
                        Text(hostTitle, fontSize = 11.sp, color = Pa.TextTertiary)
                    }
                    IconActionButton(
                        R.drawable.ic_lucide_x, "关闭", onClose,
                        bg = Color.Transparent, size = 38.dp, iconSize = 18.dp,
                    )
                }
                Spacer(Modifier.height(18.dp))

                // 终端(全屏页,不走 tab)
                DrawerItem(
                    R.drawable.ic_lucide_square_terminal, "终端", "电脑上的真实 PTY",
                    selected = false,
                ) {
                    onClose(); onOpenTerminal()
                }
                Text(
                    "导航",
                    Modifier.padding(start = 8.dp, top = 14.dp, bottom = 4.dp),
                    fontSize = 10.5.sp, color = Pa.TextTertiary,
                )
                entries.forEach { e ->
                    DrawerItem(e.icon, e.title, e.desc, selected = e.tab == currentTab) {
                        onClose(); onGoTab(e.tab)
                    }
                }
                Spacer(Modifier.height(6.dp))
                Text(
                    "更多",
                    Modifier.padding(start = 8.dp, top = 10.dp, bottom = 4.dp),
                    fontSize = 10.5.sp, color = Pa.TextTertiary,
                )
                DrawerItem(
                    R.drawable.ic_lucide_settings, "设置", "连接 / 外观 / 权限",
                    selected = false,
                ) {
                    onClose(); onOpenSettings()
                }
            }
        }
    }
}

@Composable
private fun DrawerItem(icon: Int, title: String, desc: String, selected: Boolean, onClick: () -> Unit) {
    Row(
        Modifier
            .fillMaxWidth()
            .pressable { onClick() }
            .background(if (selected) Pa.Hover else Color.Transparent, RoundedCornerShape(14.dp))
            .padding(horizontal = 10.dp, vertical = 11.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box(
            Modifier
                .size(38.dp)
                .background(if (selected) Pa.AccentDim else Pa.Card, RoundedCornerShape(12.dp)),
            contentAlignment = Alignment.Center,
        ) {
            Icon(
                painterResource(icon), null,
                tint = if (selected) Pa.Accent else Pa.TextSecondary,
                modifier = Modifier.size(18.dp),
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
        if (selected) {
            Box(Modifier.size(6.dp).background(Pa.Accent, androidx.compose.foundation.shape.CircleShape))
        }
    }
}
