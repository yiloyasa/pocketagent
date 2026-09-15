package dev.pocketagent.app.ui.component

import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.drawWithCache
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Shape
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import dev.pocketagent.app.ui.anim.rememberReduceMotion
import dev.pocketagent.app.ui.theme.Pa

/** 骨架屏形状类型 —— 必须与真实内容逐项对位,否则切换时会跳动(设计规范 §2.3)。 */
enum class SkeletonKind { SESSION_ROW, MESSAGE_BUBBLE, FILE_ROW, CARD_BLOCK }

/**
 * 骨架屏高光块。
 *
 * 【实现】底色 [Pa.Hover] + 一条 [Pa.Card] 高光带自左向右循环平移 1200ms。
 * 高光用 [drawWithCache] 一次性算好 brush,靠 graphicsLayer 级别的工作量,
 * 不触发任何重组,也没有任何 RPC —— 符合设计规范"骨架屏零开销"的红线。
 */
@Composable
private fun ShimmerBox(
    modifier: Modifier,
    shape: Shape = RoundedCornerShape(8.dp),
    reduceMotion: Boolean = rememberReduceMotion(),
) {
    if (reduceMotion) {
        // 减弱动态效果:退化为静态色块,不保留平移
        Box(modifier.clip(shape).background(Pa.Hover.copy(alpha = 0.6f)))
        return
    }
    val transition = rememberInfiniteTransition(label = "skeleton")
    val phase by transition.animateFloat(
        initialValue = 0f,
        targetValue = 1f,
        animationSpec = infiniteRepeatable(tween(1200, easing = LinearEasing)),
        label = "skeletonPhase",
    )
    Box(
        modifier
            .clip(shape)
            .drawWithCache {
                val start = phase * (size.width + 240f) - 240f
                val brush = Brush.linearGradient(
                    colors = listOf(Pa.Hover, Pa.Card, Pa.Hover),
                    start = Offset(start, 0f),
                    end = Offset(start + 240f, 0f),
                )
                onDrawBehind { drawRect(brush) }
            },
    )
}

/** 会话行骨架:圆形头像 38dp + 两行文本条。 */
@Composable
private fun SkeletonSessionRow(reduceMotion: Boolean) {
    Row(
        Modifier.fillMaxWidth().padding(vertical = 14.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        ShimmerBox(Modifier.size(38.dp), CircleShape, reduceMotion)
        Spacer(Modifier.width(12.dp))
        Column(Modifier.weight(1f)) {
            ShimmerBox(Modifier.fillMaxWidth(0.70f).height(13.dp), reduceMotion = reduceMotion)
            Spacer(Modifier.height(8.dp))
            ShimmerBox(Modifier.fillMaxWidth(0.45f).height(10.dp), reduceMotion = reduceMotion)
        }
    }
}

/** 消息气泡骨架:左右交替。 */
@Composable
private fun SkeletonBubble(alignEnd: Boolean, reduceMotion: Boolean) {
    Row(
        Modifier.fillMaxWidth().padding(vertical = 8.dp),
        horizontalArrangement = if (alignEnd) Arrangement.End else Arrangement.Start,
    ) {
        Column(
            Modifier.fillMaxWidth(if (alignEnd) 0.62f else 0.48f),
            horizontalAlignment = if (alignEnd) Alignment.End else Alignment.Start,
        ) {
            ShimmerBox(Modifier.fillMaxWidth().height(38.dp), RoundedCornerShape(14.dp), reduceMotion)
            Spacer(Modifier.height(6.dp))
            ShimmerBox(Modifier.fillMaxWidth(0.66f).height(38.dp), RoundedCornerShape(14.dp), reduceMotion)
        }
    }
}

/** 文件行骨架:图标方块 32dp + 名称条 + 右侧大小条。 */
@Composable
private fun SkeletonFileRow(reduceMotion: Boolean) {
    Row(
        Modifier.fillMaxWidth().padding(vertical = 12.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        ShimmerBox(Modifier.size(32.dp), RoundedCornerShape(8.dp), reduceMotion)
        Spacer(Modifier.width(12.dp))
        ShimmerBox(Modifier.fillMaxWidth(0.55f).height(12.dp), reduceMotion = reduceMotion)
        Spacer(Modifier.weight(1f))
        ShimmerBox(Modifier.width(40.dp).height(10.dp), reduceMotion = reduceMotion)
    }
}

/** 卡片骨架:卡片外形 + 两条文本条。 */
@Composable
private fun SkeletonCardBlock(reduceMotion: Boolean) {
    Column(
        Modifier
            .fillMaxWidth()
            .padding(vertical = 8.dp)
            .clip(RoundedCornerShape(14.dp))
            .background(Pa.Card)
            .padding(14.dp),
    ) {
        ShimmerBox(Modifier.fillMaxWidth(0.40f).height(13.dp), reduceMotion = reduceMotion)
        Spacer(Modifier.height(10.dp))
        ShimmerBox(Modifier.fillMaxWidth(0.85f).height(11.dp), reduceMotion = reduceMotion)
        Spacer(Modifier.height(6.dp))
        ShimmerBox(Modifier.fillMaxWidth(0.62f).height(11.dp), reduceMotion = reduceMotion)
    }
}

/**
 * 骨架屏列表(设计规范 §2.3)。
 *
 * 【数量】[count] 传 0 时按视口高度自动推算 —— 避免出现"骨架 6 条、真实内容 2 条"
 * 的高度跳变。视口高度不可知(如被放进纵向滚动容器)时回退到 6 条。
 *
 * 【无障碍】整块以 [clearAndSetSemantics] 折叠为单条「正在加载」播报,
 * 不让读屏逐个念出没有意义的占位条。
 */
@Composable
fun PaSkeletonList(
    kind: SkeletonKind,
    modifier: Modifier = Modifier,
    count: Int = 0,
    perRowHeight: Dp = 66.dp,
) {
    val reduceMotion = rememberReduceMotion()
    val container = modifier
        .fillMaxWidth()
        .clearAndSetSemantics { contentDescription = "正在加载" }

    BoxWithConstraints(container) {
        val auto = when {
            count > 0 -> count
            maxHeight.value == Float.POSITIVE_INFINITY -> 6
            else -> (maxHeight / perRowHeight).toInt().coerceIn(3, 10)
        }
        Column(Modifier.fillMaxWidth()) {
            repeat(auto) { index ->
                when (kind) {
                    SkeletonKind.SESSION_ROW -> SkeletonSessionRow(reduceMotion)
                    SkeletonKind.MESSAGE_BUBBLE -> SkeletonBubble(alignEnd = index % 2 == 1, reduceMotion)
                    SkeletonKind.FILE_ROW -> SkeletonFileRow(reduceMotion)
                    SkeletonKind.CARD_BLOCK -> SkeletonCardBlock(reduceMotion)
                }
            }
        }
    }
}
