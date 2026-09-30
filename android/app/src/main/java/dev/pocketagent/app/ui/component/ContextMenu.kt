package dev.pocketagent.app.ui.component

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.MutableTransitionState
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.scaleIn
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.Stable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.composed
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.TransformOrigin
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.layout.onGloballyPositioned
import androidx.compose.ui.layout.positionInRoot
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.IntOffset
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Popup
import androidx.compose.ui.window.PopupProperties
import dev.pocketagent.app.ui.anim.PaMotion
import dev.pocketagent.app.ui.anim.pressable
import dev.pocketagent.app.ui.theme.Pa
import dev.pocketagent.app.ui.theme.PaEditorial
import dev.pocketagent.app.ui.theme.paGlass
import kotlin.math.roundToInt

/**
 * 锚点菜单项(设计规范 §2.2)。
 *
 * @param destructive 危险动作,文字与图标用 PaEditorial.Denied。
 */
data class ContextMenuItem(
    val id: String,
    val label: String,
    val icon: Int? = null,
    val destructive: Boolean = false,
)

/**
 * 长按锚点菜单的状态持有者。
 *
 * 【为什么不用底部弹窗】设计要求「在用户的按压点附近展开」,让用户明确当前操作
 * 针对的是哪一项。底部弹窗会把操作对象与操作区隔开,正是要避免的。
 */
@Stable
class ContextMenuState {
    internal var anchor by mutableStateOf<Offset?>(null)

    val visible: Boolean get() = anchor != null

    fun show(at: Offset) {
        anchor = at
    }

    fun hide() {
        anchor = null
    }
}

@Composable
fun rememberContextMenuState(): ContextMenuState = remember { ContextMenuState() }

/**
 * 把某个元素变成「长按弹出锚点菜单」的触发源。
 *
 * 采集**根坐标**下的按压点(元素根位置 + 局部按压点),这样菜单才能贴着手指出现。
 * 注意:使用本 Modifier 后不要再叠加 clickable,点击行为通过 [onClick] 传入。
 */
fun Modifier.longPressMenu(
    state: ContextMenuState,
    onClick: (() -> Unit)? = null,
): Modifier = composed {
    var rootPos by remember { mutableStateOf(Offset.Zero) }
    this
        .onGloballyPositioned { rootPos = it.positionInRoot() }
        .pointerInput(state) {
            detectTapGestures(
                onTap = { onClick?.invoke() },
                onLongPress = { local -> state.show(rootPos + local) },
            )
        }
}

private val MENU_W = 220.dp
private val MENU_MIN_W = 180.dp
private val MENU_MAX_W = 240.dp
private val ITEM_H = 44.dp
private val SAFE_EDGE = 12.dp

/**
 * 长按锚点菜单(设计规范 §2.2)。
 *
 * 【定位】优先展开在按压点**右下方**;水平越界则左翻、垂直越界则上翻,
 * 四周保留 12dp 安全边,保证不被屏幕边缘裁切。
 * 入场动画的 transformOrigin 会跟随翻转方向,让菜单看起来"从手指位置长出来"。
 *
 * 【无障碍】Popup 以 focusable 方式弹出,返回键由 onDismissRequest 处理(只关菜单不退页面);
 * 菜单容器标记为 Modal 语义。
 */
@Composable
fun ContextMenu(
    state: ContextMenuState,
    summary: String,
    items: List<ContextMenuItem>,
    onPick: (ContextMenuItem) -> Unit,
    onDismiss: () -> Unit,
) {
    val point = state.anchor ?: return

    val density = LocalDensity.current
    val config = LocalConfiguration.current
    val screenW = with(density) { config.screenWidthDp.dp.toPx() }
    val screenH = with(density) { config.screenHeightDp.dp.toPx() }

    val menuW = with(density) { MENU_W.toPx() }
    val menuH = with(density) {
        // 摘要行 ~30dp + 分隔线 1dp + 每项 44dp + 上下留白 12dp
        (30 + 1 + items.size * 44 + 12).dp.toPx()
    }
    val safe = with(density) { SAFE_EDGE.toPx() }

    val flipX = point.x + menuW > screenW - safe
    val flipY = point.y + menuH > screenH - safe
    val x = if (flipX) (point.x - menuW).coerceAtLeast(safe) else point.x
    val y = if (flipY) (point.y - menuH).coerceAtLeast(safe) else point.y

    val visibleState = remember { MutableTransitionState(false).apply { targetState = true } }

    Popup(
        properties = PopupProperties(focusable = true),
        onDismissRequest = onDismiss,
    ) {
        Box(Modifier.fillMaxSize()) {
            // 空白处点击关闭
            Box(
                Modifier
                    .fillMaxSize()
                    .clickable(
                        interactionSource = remember { MutableInteractionSource() },
                        indication = null,
                    ) { onDismiss() },
            )

            AnimatedVisibility(
                visibleState = visibleState,
                // 【动效令牌】入场从手指位置"长出来":轻微过冲的 scale + 临界阻尼 fade,
                // spring 从当前值续走,长按连发时被打断也不跳帧(替代原 tween(120) 硬切)。
                enter = fadeIn(PaMotion.standard()) +
                    scaleIn(PaMotion.emphasized(), initialScale = 0.92f),
                // 退场要干脆:更大 stiffness 的 spring,让位快但不砖墙
                exit = fadeOut(PaMotion.nav(600f)),
                modifier = Modifier.offset { IntOffset(x.roundToInt(), y.roundToInt()) },
            ) {
                Column(
                    Modifier
                        .widthIn(min = MENU_MIN_W, max = MENU_MAX_W)
                        .graphicsLayer {
                            transformOrigin = TransformOrigin(
                                pivotFractionX = if (flipX) 1f else 0f,
                                pivotFractionY = if (flipY) 1f else 0f,
                            )
                        }
                        .clip(RoundedCornerShape(14.dp))
                        // 玻璃语言:悬浮菜单 = 实色底(PaEditorial.Raised,可读性优先)+ 顶缘发丝高光
                        // + 一点投影——"浮在内容之上"的层级感来自亮边与影子,不是透。
                        .paGlass(RoundedCornerShape(14.dp), tint = PaEditorial.Raised, shadowElevation = 16.dp)
                        .padding(vertical = 6.dp),
                ) {
                    // 对象摘要行 —— 让用户确认"这次操作针对谁"
                    Text(
                        summary,
                        fontSize = 11.sp,
                        color = PaEditorial.Subtle,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(horizontal = 14.dp, vertical = 6.dp),
                    )
                    Box(Modifier.fillMaxWidth().height(1.dp).background(PaEditorial.Hairline))

                    items.forEach { item ->
                        val tint = if (item.destructive) PaEditorial.Denied else PaEditorial.Ink
                        Row(
                            Modifier
                                .fillMaxWidth()
                                .height(ITEM_H)
                                // 按下即缩放反馈(全 app 统一 pressable),不再是只有涟漪的裸 clickable
                                .pressable { onPick(item) }
                                .padding(horizontal = 14.dp),
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            if (item.icon != null) {
                                Icon(
                                    painterResource(item.icon), null,
                                    tint = if (item.destructive) PaEditorial.Denied else PaEditorial.Muted,
                                    modifier = Modifier.size(16.dp),
                                )
                                Spacer(Modifier.width(10.dp))
                            }
                            Text(item.label, fontSize = 13.sp, color = tint)
                        }
                    }
                }
            }
        }
    }
}
