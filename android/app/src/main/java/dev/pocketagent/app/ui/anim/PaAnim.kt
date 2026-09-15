package dev.pocketagent.app.ui.anim

import androidx.compose.animation.core.Spring
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.spring
import androidx.compose.foundation.background
import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.clickable
import androidx.compose.foundation.combinedClickable
import androidx.compose.foundation.gestures.detectHorizontalDragGestures
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsPressedAsState
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.composed
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Shape
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.hapticfeedback.HapticFeedbackType
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalHapticFeedback
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/**
 * 按压动效(全 app 通用):按下轻微缩小 + 松手弹回。
 *
 * 【讲解】为什么用 spring 而不是 tween:真实物理感——手指抬起时
 * 不是匀速恢复,而是带一点回弹地"弹"回来,这是"高级感"的核心来源。
 * dampingRatio 越小回弹越明显(0.5 明显 / 1.0 无回弹),
 * stiffness 越大反应越快(420 是"跟手但不僵硬"的甜点值)。
 */
fun Modifier.pressable(
    scaleDown: Float = 0.97f,
    haptic: Boolean = false,
    enabled: Boolean = true,
    onClick: () -> Unit,
): Modifier = composed {
    val interaction = remember { MutableInteractionSource() }
    val pressed by interaction.collectIsPressedAsState()
    val scale by animateFloatAsState(
        targetValue = if (pressed) scaleDown else 1f,
        animationSpec = spring(
            dampingRatio = Spring.DampingRatioNoBouncy,
            stiffness = 520f,
        ),
        label = "press",
    )
    val hf = LocalHapticFeedback.current
    this
        .graphicsLayer { scaleX = scale; scaleY = scale }
        .clickable(interactionSource = interaction, indication = null, enabled = enabled) {
            if (haptic) hf.performHapticFeedback(HapticFeedbackType.TextHandleMove)
            onClick()
        }
}

/** 带长按的按压缩放(会话卡片:单击打开/长按管理)。 */
@OptIn(ExperimentalFoundationApi::class)
fun Modifier.pressableLong(
    onClick: () -> Unit,
    onLongPress: () -> Unit,
): Modifier = composed {
    val interaction = remember { MutableInteractionSource() }
    val pressed by interaction.collectIsPressedAsState()
    val scale by animateFloatAsState(
        targetValue = if (pressed) 0.97f else 1f,
        animationSpec = spring(
            dampingRatio = Spring.DampingRatioNoBouncy,
            stiffness = 520f,
        ),
        label = "pressLong",
    )
    this
        .graphicsLayer { scaleX = scale; scaleY = scale }
        .combinedClickable(
            interactionSource = interaction,
            indication = null,
            onClick = onClick,
            onLongClick = onLongPress,
        )
}

/** 圆形/圆角方形图标按钮(顶部工具栏、发送键等),带按压缩放。 */
@Composable
fun IconActionButton(
    iconRes: Int,
    contentDescription: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    size: Dp = 42.dp,
    iconSize: Dp = 20.dp,
    bg: Color = MaterialTheme.colorScheme.surfaceVariant,
    tint: Color = MaterialTheme.colorScheme.onBackground,
    shape: Shape = RoundedCornerShape(14.dp),
) {
    Box(
        modifier = modifier
            .size(size)
            .pressable { onClick() }
            .background(bg, shape),
        contentAlignment = Alignment.Center,
    ) {
        Icon(painterResource(iconRes), contentDescription, tint = tint, modifier = Modifier.size(iconSize))
    }
}

/** 分区标题行:左侧标题+副标题,右侧尾部组件(如"全部 →")。 */
@Composable
fun SectionHeader(
    title: String,
    subtitle: String? = null,
    modifier: Modifier = Modifier,
    trailing: @Composable () -> Unit = {},
) {
    Row(
        modifier.fillMaxWidth().padding(vertical = 4.dp),
        horizontalArrangement = androidx.compose.foundation.layout.Arrangement.SpaceBetween,
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Column {
            Text(title, fontSize = 16.sp, color = MaterialTheme.colorScheme.onBackground)
            if (subtitle != null) {
                Text(subtitle, fontSize = 12.sp, color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.padding(top = 2.dp))
            }
        }
        trailing()
    }
}

/** 空状态:图标 + 主文案 + 副文案(参考稿"连接电脑后,任务会出现在这里")。 */
@Composable
fun EmptyState(iconRes: Int, title: String, hint: String, modifier: Modifier = Modifier) {
    Column(
        modifier.fillMaxWidth().padding(vertical = 48.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Icon(painterResource(iconRes), null, tint = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.size(44.dp))
        Text(title, Modifier.padding(top = 14.dp), fontSize = 14.sp, color = MaterialTheme.colorScheme.onBackground)
        Text(hint, Modifier.padding(top = 4.dp), fontSize = 12.sp, color = MaterialTheme.colorScheme.onSurfaceVariant)
    }
}

/**
 * 左边缘右滑唤出侧边栏(问题 7:交互更顺滑)。
 *
 * 【起手区间是实测出来的】
 *  - 最左侧 ~24dp 是**系统返回手势区**(HyperOS 实测 x<24dp 的横滑直接退到桌面),
 *    那一段我们永远拿不到,所以不指望它;
 *  - 起手点放宽到 [edgeWidth](64dp,约屏幕宽度的 6%),用户在"边缘往里一点"的位置
 *    右滑即可唤出 —— 这个区间系统不抢、我们也不和行内左滑打架
 *    (行内左滑从行中间起手,且只认"往左拉")。
 *
 * 【为什么不遮罩覆盖】把检测挂在页面根节点上,子节点仍在命中链里,点击不受影响;
 * 只有判定确实是从左侧起手的右滑后,才 consume 事件。
 */
fun Modifier.edgeSwipeToOpenDrawer(
    enabled: Boolean = true,
    edgeWidth: Dp = 64.dp,
    threshold: Dp = 64.dp,
    onOpen: () -> Unit,
): Modifier = composed {
    if (!enabled) return@composed this
    val density = androidx.compose.ui.platform.LocalDensity.current
    val edgePx = with(density) { edgeWidth.toPx() }
    val thresholdPx = with(density) { threshold.toPx() }
    var accepted by remember { mutableStateOf(false) }
    var travelled by remember { mutableStateOf(0f) }

    this.pointerInput(edgePx, thresholdPx) {
        // 注意:扩展函数不能写全限定名调用,必须靠上面的 import 直接引用
        detectHorizontalDragGestures(
            onDragStart = { offset ->
                accepted = offset.x <= edgePx
                travelled = 0f
            },
            onDragEnd = { accepted = false; travelled = 0f },
            onDragCancel = { accepted = false; travelled = 0f },
            onHorizontalDrag = { change, dragAmount ->
                if (!accepted) return@detectHorizontalDragGestures
                // 只认"往右拉"
                if (dragAmount <= 0f) {
                    accepted = false
                    return@detectHorizontalDragGestures
                }
                travelled += dragAmount
                change.consume()
                if (travelled >= thresholdPx) {
                    accepted = false
                    travelled = 0f
                    onOpen()
                }
            },
        )
    }
}

/**
 * 系统「减弱动态效果」是否开启(开发者选项里的"动画时长缩放"设为 0)。
 *
 * 【设计规范 §1.5】开启时:骨架屏取消平移、撤销条取消进度线动画、
 * 侧滑改为瞬时展开、抽屉取消滑入仅保留淡入。
 */
@Composable
fun rememberReduceMotion(): Boolean {
    val ctx = LocalContext.current
    return remember(ctx) {
        runCatching {
            android.provider.Settings.Global.getFloat(
                ctx.contentResolver,
                android.provider.Settings.Global.ANIMATOR_DURATION_SCALE,
                1f,
            ) == 0f
        }.getOrDefault(false)
    }
}
