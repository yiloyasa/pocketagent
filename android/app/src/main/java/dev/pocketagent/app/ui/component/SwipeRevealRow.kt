package dev.pocketagent.app.ui.component

import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.gestures.detectHorizontalDragGestures
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.MutableState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Shape
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.hapticfeedback.HapticFeedbackType
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.LocalHapticFeedback
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.semantics.CustomAccessibilityAction
import androidx.compose.ui.semantics.customActions
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.ui.anim.PaMotion
import dev.pocketagent.app.ui.anim.rememberReduceMotion
import dev.pocketagent.app.ui.theme.Pa
import kotlinx.coroutines.launch

/**
 * 侧滑动作项(设计规范 §2.1)。
 *
 * @param destructive 危险动作(删除),用 Pa.Red 底;非危险动作用 Pa.AccentDim 底。
 */
data class SwipeAction(
    val id: String,
    val label: String,
    val icon: Int,
    val tint: Color,
    val destructive: Boolean = false,
)

private val ACTION_WIDTH = 64.dp

/**
 * 互斥侧滑容器(设计规范 §2.1)。
 *
 * 【核心语义】同一时刻**只允许一行处于展开态**:[openKey] 是一个由列表持有的单值状态,
 * 它的值等于 [rowKey] 时本行展开,相当于别的行时本行自动回弹。
 * 这样用户能连续滑开不同行处理,而不会出现"多行同时张着嘴"的混乱。
 *
 * 【手势】8dp 方向阈值:垂直位移大于水平位移时**不拦截**事件,保证列表纵向滚动不受影响。
 * 滑动位移越过动作区宽度 40% 时触发一次触感反馈。
 *
 * 【跟手 + 动量落位】拖动中逐帧 snapTo(1:1 贴手指);松手把最后一刻的**手指速度**
 * 作为 initialVelocity 交给 [PaMotion.momentum] 的 spring —— 快扫带速度滑开、
 * 慢拖松手即停,落位是物理连续的,不是"播完为止"的固定动画。
 *
 * 【按压反馈】整行按压缩放挂在**外层容器**(内容层与动作层一起缩):
 * 只缩内容层会在边缘露出底下的动作层(红色删除钮从缝里渗出来)。反馈在按下瞬间,
 * 与全 app 的 pressable 同一手感。
 *
 * 【无障碍】侧滑手势不是唯一入口 —— 已在容器上注册 [CustomAccessibilityAction],
 * TalkBack 用户可直接从操作菜单里选择「重命名 / 删除」。
 *
 * 用法(列表外层持有 openKey):
 * ```
 * var openKey by remember { mutableStateOf<Any?>(null) }
 * // 滚动开始时收起,避免滑动中被误触
 * LaunchedEffect(listState.isScrollInProgress) { if (listState.isScrollInProgress) openKey = null }
 * LazyColumn {
 *   items(list, key = { it.id }) { row ->
 *     SwipeRevealRow(row.id, ::openKey, actions, onAction) { SessionRowContent(row) }
 *   }
 * }
 * ```
 *
 * @param rowBackground 内容层的**不透明**底色。必须能盖住动作层,否则会透出按钮。
 * @param onClick 点击内容。**注意:内容里不要再挂自己的 clickable/combinedClickable** ——
 *   内层 clickable 会在 Main 传递中先消费 down 事件,导致外层的横向拖拽检测器收到已消费事件而放弃,
 *   表现为"怎么滑都只触发点击"。容器独占手势是这里的关键约束。
 * @param onLongPress 长按内容(如打开管理面板)。
 */
@Composable
fun SwipeRevealRow(
    rowKey: Any,
    openKey: MutableState<Any?>,
    actions: List<SwipeAction>,
    onAction: (SwipeAction) -> Unit,
    modifier: Modifier = Modifier,
    rowBackground: Color = Pa.Card,
    shape: Shape = RoundedCornerShape(18.dp),
    onClick: (() -> Unit)? = null,
    onLongPress: (() -> Unit)? = null,
    content: @Composable () -> Unit,
) {
    if (actions.isEmpty()) {
        Box(modifier) { content() }
        return
    }

    val density = LocalDensity.current
    // 动作区最多占屏宽 72%:4 个动作 × 64dp 在窄屏上会顶到边界,露出没铺背景的黑块
    val maxReveal = with(density) { 0.72f * android.content.res.Resources.getSystem().displayMetrics.widthPixels }
    val revealPx = minOf(with(density) { (ACTION_WIDTH * actions.size).toPx() }, maxReveal)
    // 【手感】旧值 = 动作区总宽的 40%(4 个动作时约 121dp),用户得"滑到底"才认账,
    // 松手还常常回弹。现在取 min(28%, 44dp):轻扫一小段就吸附全开,不再要求拖满。
    val thresholdPx = minOf(revealPx * 0.28f, with(density) { 44.dp.toPx() })
    val haptic = LocalHapticFeedback.current
    val scope = rememberCoroutineScope()
    val reduceMotion = rememberReduceMotion()
    val offset = remember { Animatable(0f) }
    val isOpen = openKey.value == rowKey

    // ---- 按压缩放(整行,含动作层一起缩,见类注释) ----
    var pressed by remember { mutableStateOf(false) }
    val pressScale by animateFloatAsState(
        targetValue = if (pressed) 0.98f else 1f,
        animationSpec = if (reduceMotion) PaMotion.instant() else PaMotion.press(),
        label = "swipeRowPress",
    )

    // 互斥:本行不是当前展开项时回弹归位(这是状态修正,不是动量落位,用标准 spring)
    LaunchedEffect(isOpen) {
        if (!isOpen && offset.value != 0f) offset.animateTo(0f, PaMotion.standard())
        if (isOpen && offset.value != -revealPx) offset.animateTo(-revealPx, PaMotion.momentum())
    }

    // ---- 松手速度采样:与 DrawerController 同一套 120ms 窗口 + 0.6/0.4 平滑 ----
    var lastDragMs by remember { mutableStateOf(0L) }
    var dragVelocityPx by remember { mutableStateOf(0f) }

    Box(
        modifier
            .fillMaxWidth()
            .graphicsLayer { scaleX = pressScale; scaleY = pressScale }
            .semantics {
                customActions = actions.map { a ->
                    CustomAccessibilityAction(a.label) {
                        openKey.value = null
                        onAction(a)
                        true
                    }
                }
            },
    ) {
        // ---- 动作层(背景):整块铺底(含左侧空白区),避免左移后露出"未衔接的黑块" ----
        Box(
            Modifier
                .matchParentSize()
                .clip(shape)
                .background(Pa.Card),
        ) {
            Row(
                Modifier
                    .align(Alignment.CenterEnd)
                    .fillMaxHeight()
                    .width(with(density) { revealPx.toDp() }),
                horizontalArrangement = Arrangement.End,
            ) {
                actions.forEach { action ->
                    Box(
                        Modifier
                            .weight(1f, fill = true)
                            .fillMaxHeight()
                            .background(if (action.destructive) Pa.Red else Pa.AccentDim)
                            .clickable {
                                openKey.value = null
                                onAction(action)
                            },
                        contentAlignment = Alignment.Center,
                    ) {
                        Column(horizontalAlignment = Alignment.CenterHorizontally) {
                            Icon(
                                painterResource(action.icon), null,
                                tint = action.tint,
                                modifier = Modifier.size(16.dp),
                            )
                            Text(
                                action.label,
                                fontSize = 10.sp,
                                color = action.tint,
                                modifier = Modifier.padding(top = 4.dp),
                            )
                        }
                    }
                }
            }
        }

        // ---- 内容层(前景):决定整体高度,不透明以盖住动作层 ----
        // 【圆角修复】以前是裸 .background(rowBackground) —— 矩形底把卡片自身的
        // 圆角盖掉了,列表看上去是一个个直角黑块。这里统一裁到同一 shape。
        Box(
            Modifier
                .fillMaxWidth()
                .heightIn(min = 64.dp)
                .graphicsLayer { translationX = offset.value }
                .clip(shape)
                .background(rowBackground)
                // 手势顺序很重要:先声明 tap,再声明 drag —— 后者在内层,
                // PointerEventPass.Main 是「由内向外」派发,所以 drag 先拿到事件;
                // 一旦拖动越过阈值它就 consume,内层的 tap 检测器随即自动取消。
                .pointerInput(rowKey) {
                    detectTapGestures(
                        onPress = {
                            // 按下即反馈(pressed 状态驱动缩放);取消/抬起都要复位
                            pressed = true
                            try {
                                awaitRelease()
                            } finally {
                                pressed = false
                            }
                        },
                        onTap = {
                            // 已展开时点内容 = 收起;未展开才真正触发 onClick
                            if (openKey.value == rowKey) openKey.value = null else onClick?.invoke()
                        },
                        onLongPress = { onLongPress?.invoke() },
                    )
                }
                .pointerInput(rowKey, actions) {
                    detectHorizontalDragGestures(
                        onDragEnd = {
                            val target = if (-offset.value > thresholdPx) -revealPx else 0f
                            openKey.value = if (target != 0f) rowKey else null
                            val v = dragVelocityPx
                            dragVelocityPx = 0f
                            scope.launch {
                                offset.animateTo(target, PaMotion.momentum(), initialVelocity = v)
                            }
                        },
                        onDragCancel = {
                            dragVelocityPx = 0f
                            scope.launch {
                                offset.animateTo(if (isOpen) -revealPx else 0f, PaMotion.standard())
                            }
                        },
                        onHorizontalDrag = { change, dragAmount ->
                            change.consume()
                            val before = offset.value
                            val next = (before + dragAmount).coerceIn(-revealPx, 0f)
                            // 【速度采样】dt 窗口 120ms,断流(手指停顿)即清零;
                            // 0.6/0.4 指数平滑去掉单帧噪声,松手交接才不抖。
                            val dtMs = change.uptimeMillis - lastDragMs
                            if (dtMs in 1..120) {
                                val v = dragAmount / (dtMs / 1000f)
                                dragVelocityPx =
                                    if (dragVelocityPx == 0f) v else dragVelocityPx * 0.6f + v * 0.4f
                            } else {
                                dragVelocityPx = 0f
                            }
                            lastDragMs = change.uptimeMillis
                            // 越过展开阈值时给一次轻触感(只在跨越的那一帧触发;
                            // TextHandleMove 比 LongPress 轻,是"档位"感不是"警告"感)
                            val crossed = (before > -thresholdPx) != (next > -thresholdPx)
                            if (crossed) haptic.performHapticFeedback(HapticFeedbackType.TextHandleMove)
                            scope.launch { offset.snapTo(next) }
                        },
                    )
                },
        ) { content() }
    }
}
