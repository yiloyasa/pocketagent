package dev.pocketagent.app.ui.component

import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.spring
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
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
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
private val RevealSpring = spring<Float>(dampingRatio = 0.85f, stiffness = 500f)

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
    val revealPx = with(density) { (ACTION_WIDTH * actions.size).toPx() }
    // 【手感】旧值 = 动作区总宽的 40%(4 个动作时约 121dp),用户得"滑到底"才认账,
    // 松手还常常回弹。现在取 min(28%, 44dp):轻扫一小段就吸附全开,不再要求拖满。
    val thresholdPx = minOf(revealPx * 0.28f, with(density) { 44.dp.toPx() })
    val haptic = LocalHapticFeedback.current
    val scope = rememberCoroutineScope()
    val offset = remember { Animatable(0f) }
    val isOpen = openKey.value == rowKey

    // 互斥:本行不是当前展开项时回弹归位
    LaunchedEffect(isOpen) {
        if (!isOpen && offset.value != 0f) offset.animateTo(0f, RevealSpring)
        if (isOpen && offset.value != -revealPx) offset.animateTo(-revealPx, RevealSpring)
    }

    Box(
        modifier
            .fillMaxWidth()
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
        // ---- 动作层(背景):尺寸跟随父容器,靠右铺满高度 ----
        Box(Modifier.matchParentSize().clip(shape)) {
            Row(
                Modifier.align(Alignment.CenterEnd).fillMaxHeight(),
                horizontalArrangement = Arrangement.End,
            ) {
                actions.forEach { action ->
                    Box(
                        Modifier
                            .width(ACTION_WIDTH)
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
                            scope.launch { offset.animateTo(target, RevealSpring) }
                        },
                        onDragCancel = {
                            scope.launch {
                                offset.animateTo(if (isOpen) -revealPx else 0f, RevealSpring)
                            }
                        },
                        onHorizontalDrag = { change, dragAmount ->
                            change.consume()
                            val before = offset.value
                            val next = (before + dragAmount).coerceIn(-revealPx, 0f)
                            // 越过展开阈值时给一次触感(只在跨越的那一帧触发)
                            val crossed = (before > -thresholdPx) != (next > -thresholdPx)
                            if (crossed) haptic.performHapticFeedback(HapticFeedbackType.LongPress)
                            scope.launch { offset.snapTo(next) }
                        },
                    )
                },
        ) { content() }
    }
}
