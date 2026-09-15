package dev.pocketagent.app.ui.component

import androidx.compose.animation.core.FastOutSlowInEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.StartOffset
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import dev.pocketagent.app.ui.anim.rememberReduceMotion
import dev.pocketagent.app.ui.theme.Pa
import kotlin.math.PI
import kotlin.math.sin

/**
 * 「电脑端正在思考」三点指示器。
 *
 * 【这一版的动效取向:克制】上一版是"抛跳"(位移 1.6×点径,线性 easing),
 * 三点各自独立跳,视觉上显得歪斜、振幅过大,和 App 的静态卡片语言不搭。
 * 现在改为**呼吸式**:
 *  - 位移只留 2.5dp(点径的 0.5 倍),远看几乎不动,近看有轻微起伏;
 *  - 主导变化是透明度 0.30 → 1.00 与缩放 0.82 → 1.00,像"脉冲"而不是"蹦跳";
 *  - 周期拉到 1100ms、相位差 130ms,节奏舒缓,配 FastOutSlowIn 缓动;
 *  - 对齐方式改成居中(Bottom 对齐会因位移把点推得高低不齐)。
 *
 * 【无障碍】减弱动态效果时退化为静态三点(只保留透明度差)。
 */
@Composable
fun TypingDots(
    modifier: Modifier = Modifier,
    dotSize: Dp = 6.dp,
    color: Color = Pa.Accent,
    gap: Dp = 4.dp,
) {
    if (rememberReduceMotion()) {
        Row(modifier, verticalAlignment = Alignment.CenterVertically) {
            repeat(3) { i ->
                if (i > 0) Spacer(Modifier.width(gap))
                Box(
                    Modifier
                        .size(dotSize)
                        .background(color.copy(alpha = if (i == 1) 0.9f else 0.45f), CircleShape),
                )
            }
        }
        return
    }

    val transition = rememberInfiniteTransition(label = "typingDots")
    val phases = List(3) { i ->
        transition.animateFloat(
            initialValue = 0f,
            targetValue = 1f,
            animationSpec = infiniteRepeatable(
                animation = tween(
                    durationMillis = 1100,
                    easing = FastOutSlowInEasing,
                ),
                repeatMode = RepeatMode.Restart,
                initialStartOffset = StartOffset(i * 130),
            ),
            label = "dot$i",
        )
    }

    Row(modifier, verticalAlignment = Alignment.CenterVertically) {
        phases.forEachIndexed { i, p ->
            if (i > 0) Spacer(Modifier.width(gap))
            // sin 曲线:0 → 1 → 0,比抛物线更平滑(两端导数为 0,没有生硬的起停)
            val lift = sin(PI * p.value).toFloat()
            Box(
                Modifier
                    .size(dotSize)
                    .graphicsLayer {
                        translationY = -lift * dotSize.toPx() * 0.5f
                        scaleX = 0.82f + 0.18f * lift
                        scaleY = 0.82f + 0.18f * lift
                        alpha = 0.30f + 0.70f * lift
                    }
                    .background(color, CircleShape),
            )
        }
    }
}
