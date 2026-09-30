package dev.pocketagent.app.ui.component

import androidx.activity.compose.BackHandler
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInHorizontally
import androidx.compose.animation.slideOutHorizontally
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.gestures.detectTapGestures
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
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.R as AppR
import dev.pocketagent.app.ui.anim.IconActionButton
import dev.pocketagent.app.ui.anim.PaMotion
import dev.pocketagent.app.ui.theme.PaEditorial
import dev.pocketagent.app.ui.theme.PaHacker

/**
 * 历史消息快速查看(2026-09-30):左缘垂直刻度条 + 左滑浮层摘要卡列表。
 * 交互样板来自用户提供的演示视频(ZCode 客户端):
 *  - 刻度条:每条**用户消息**一个刻度,贴会话内容区左缘,半透明不遮内容;
 *  - 点刻度条 → 从左缘滑出 Drawer 式浮层(宽 ~78%):全部用户消息的
 *    序号 + 标题(首行粗体)+ 摘要(次行截断两行),当前可见位置高亮;
 *  - 点卡片 → 跳到那条消息并关浮层;浮层外点击 / 返回键 = 关闭。
 *
 * 纯 UI 层:条目与高亮判定走 [HistoryQuickNavLogic](JVM 单测钉住),这里不重复
 * 决策;跳转动作由宿主(ChatScreen)回调实现 —— 浮层不持有列表状态。
 *
 * 【三色盘同构】常规模式 Surface 底;黑客模式 GraphiteDrawer 底 + 等宽字
 * (契约见 PaHacker)。全部取值走 editorial 槽位,无硬编码色。
 */

/** 刻度条视觉宽度(8-10dp 规格,半透明;触控条更宽一点但不遮正文)。 */
private val TickBarVisualWidth = 10.dp
/** 刻度条触控宽度:视觉 10dp + 两侧各 7dp 命中余量(≈24dp,横向再宽会盖正文)。 */
private val TickBarTouchWidth = 24.dp

/**
 * 左缘垂直刻度条。满高透明触控条 + Canvas 画刻度点(不做逐条 composable,
 * 消息很多时也只有一次绘制,规格红线)。
 *
 * @param count 刻度数(=用户消息数);0 时不渲染(没有可导航的内容)。
 * @param currentIndex 当前可见位置(视口顶部最近滚过的用户消息,-1=无)。
 * @param onOpen 点按刻度条 → 宿主打开浮层。
 */
@Composable
fun HistoryTickBar(
    count: Int,
    currentIndex: Int,
    onOpen: () -> Unit,
    modifier: Modifier = Modifier,
) {
    if (count <= 0) return
    val hacker = PaHacker.isHacker
    val idleColor = if (hacker) {
        PaEditorial.OnGraphiteSubtle.copy(alpha = 0.55f)
    } else {
        PaEditorial.Muted.copy(alpha = 0.45f)
    }
    val activeColor = PaEditorial.Primary
    Box(
        modifier
            .width(TickBarTouchWidth)
            .fillMaxHeight()
            // 命中区透传给浮层手势前先消费点按;不画底色,只靠刻度点示意可点
            .pointerInput(onOpen) { detectTapGestures { onOpen() } }
            .semantics {
                contentDescription = "历史消息快速查看,共 $count 条用户消息"
            },
        contentAlignment = Alignment.CenterStart,
    ) {
        Canvas(Modifier.width(TickBarVisualWidth).fillMaxHeight()) {
            val tickH = 2.dp.toPx()
            val gap = 7.dp.toPx()
            // 刻度均匀分布:自然间距放不下就整体压缩(极端小窗兜底),组内垂直居中
            val naturalStep = tickH + gap
            val step = if (count > 1) minOf(naturalStep, (size.height - tickH) / (count - 1)) else 0f
            val totalH = tickH + step * (count - 1)
            var y = (size.height - totalH) / 2f
            for (i in 0 until count) {
                val active = i == currentIndex
                // 当前可见位置的刻度更长更亮(视频样板:当前刻度与其余拉开)
                val w = if (active) size.width else size.width * 0.7f
                drawRoundRect(
                    color = if (active) activeColor else idleColor,
                    topLeft = Offset(0f, y),
                    size = Size(w, tickH),
                    cornerRadius = CornerRadius(tickH / 2f),
                )
                y += step
            }
        }
    }
}

/**
 * 左滑浮层:历史消息摘要卡列表(Drawer 式 Panel)。
 *
 * @param visible 开关(宿主持有,返回键/Scrim 点击都关它)。
 * @param entries [HistoryQuickNavLogic.entriesOf] 的产物。
 * @param currentIndex 当前可见位置(高亮用,-1=无)。
 * @param onJump(listIndex) 点卡片:宿主把 LazyColumn 滚到该下标并关闭浮层。
 * @param onDismiss 关闭(Scrim 点击)。
 */
@Composable
fun HistoryQuickPanel(
    visible: Boolean,
    entries: List<HistoryQuickNavLogic.Entry>,
    currentIndex: Int,
    onJump: (Int) -> Unit,
    onDismiss: () -> Unit,
) {
    val hacker = PaHacker.isHacker
    // 返回键关闭(与抽屉/面板同层语义);visible=false 时不武装,不抢别的层。
    BackHandler(enabled = visible) { onDismiss() }
    AnimatedVisibility(
        visible = visible,
        enter = fadeIn(PaMotion.emphasized()) +
            slideInHorizontally(PaMotion.emphasized()) { -it / 4 },
        exit = fadeOut(PaMotion.standard()) +
            slideOutHorizontally(PaMotion.standard()) { -it / 4 },
    ) {
        // Scrim:浮层外点击 = 关闭(全屏,拦截下层交互)
        Box(
            Modifier
                .fillMaxSize()
                .background(PaEditorial.Scrim)
                .clickable(onClick = onDismiss),
        )
    }
    AnimatedVisibility(
        visible = visible,
        enter = slideInHorizontally(PaMotion.emphasized()) { -it },
        exit = slideOutHorizontally(PaMotion.standard()) { -it },
    ) {
        val panelWidth = LocalConfiguration.current.screenWidthDp.dp * 0.78f
        Column(
            Modifier
                .fillMaxHeight()
                .width(panelWidth)
                .background(if (hacker) PaEditorial.GraphiteDrawer else PaEditorial.Surface)
                .then(if (hacker) Modifier else Modifier.border(1.dp, PaEditorial.Hairline))
                .statusBarsPadding(),
        ) {
            // ---- 头部:标题 + 条数 + 关闭 ----
            Row(
                Modifier
                    .fillMaxWidth()
                    .padding(start = 16.dp, end = 6.dp, top = 10.dp, bottom = 6.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(
                    "历史消息",
                    fontSize = 16.sp,
                    fontWeight = FontWeight.SemiBold,
                    color = PaEditorial.Ink,
                    fontFamily = if (hacker) PaHacker.mono else null,
                    modifier = Modifier.weight(1f),
                )
                Text(
                    "${entries.size} 条",
                    fontSize = 11.5.sp,
                    color = PaEditorial.Subtle,
                    fontFamily = if (hacker) PaHacker.mono else null,
                )
                Spacer(Modifier.width(4.dp))
                IconActionButton(
                    AppR.drawable.ic_lucide_x, "关闭历史消息面板", onDismiss,
                    bg = Color.Transparent, size = 36.dp, iconSize = 16.dp,
                    tint = PaEditorial.Muted,
                )
            }
            Spacer(Modifier.height(4.dp))
            if (entries.isEmpty()) {
                Text(
                    "还没有可跳转的用户消息",
                    Modifier.padding(horizontal = 16.dp, vertical = 12.dp),
                    fontSize = 12.5.sp,
                    color = PaEditorial.Subtle,
                    fontFamily = if (hacker) PaHacker.mono else null,
                )
            } else {
                LazyColumn(
                    Modifier.fillMaxWidth(),
                    contentPadding = androidx.compose.foundation.layout.PaddingValues(bottom = 16.dp),
                ) {
                    itemsIndexed(entries, key = { _, e -> e.listIndex }) { i, entry ->
                        val active = i == currentIndex
                        Row(
                            Modifier
                                .fillMaxWidth()
                                // 高亮 = 当前可见位置:主色细条 + 主色标题;点卡片跳转
                                .then(
                                    if (active) Modifier.background(PaEditorial.PrimaryContainer)
                                    else Modifier
                                )
                                .clickable { onJump(entry.listIndex) }
                                .heightIn(min = 56.dp)
                                .padding(horizontal = 16.dp, vertical = 10.dp),
                            verticalAlignment = Alignment.Top,
                        ) {
                            // 序号:第几条用户消息(1 起),等宽对齐不跳动
                            Text(
                                "${i + 1}",
                                Modifier.width(24.dp),
                                fontSize = 11.5.sp,
                                color = if (active) PaEditorial.Primary else PaEditorial.Subtle,
                                fontWeight = FontWeight.SemiBold,
                                fontFamily = PaHacker.mono,
                            )
                            Column(Modifier.weight(1f)) {
                                Text(
                                    entry.title,
                                    fontSize = 14.sp,
                                    lineHeight = 19.sp,
                                    fontWeight = FontWeight.Bold,
                                    color = if (active) PaEditorial.Primary else PaEditorial.Ink,
                                    maxLines = 1,
                                    overflow = TextOverflow.Ellipsis,
                                    fontFamily = if (hacker) PaHacker.mono else null,
                                )
                                if (entry.summary.isNotEmpty()) {
                                    Spacer(Modifier.height(3.dp))
                                    Text(
                                        entry.summary,
                                        fontSize = 12.sp,
                                        lineHeight = 17.sp,
                                        color = if (hacker) PaEditorial.OnGraphiteMuted else PaEditorial.Muted,
                                        maxLines = 2,
                                        overflow = TextOverflow.Ellipsis,
                                        fontFamily = if (hacker) PaHacker.mono else null,
                                    )
                                }
                            }
                            if (active) {
                                Spacer(Modifier.width(8.dp))
                                Box(
                                    Modifier
                                        .size(7.dp)
                                        .background(PaEditorial.Primary, CircleShape)
                                        .align(Alignment.CenterVertically),
                                )
                            }
                        }
                    }
                }
            }
        }
    }
}
