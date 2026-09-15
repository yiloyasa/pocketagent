package dev.pocketagent.app.ui.component

import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.StartOffset
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.height
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
 * 「对方正在输入」三点跳动(three-dot bouncing)。
 *
 * 【动效】每点沿抛物线 4x(1-x) 起落(0→1→0),三点半径 150ms 相位差,
 * 周期 420ms —— 比"光标闪烁"更接近真人打字的节奏感,也更安静。
 * 【无障碍】减弱动态效果时退化为静态三点(呼吸透明度都不做)。
 *
 * @param dotSize 圆点直径;消息气泡内默认 6dp,行内跟随用 4dp
 */
@Composable
fun TypingDots(
    modifier: Modifier = Modifier,
    dotSize: Dp = 6.dp,
    color: Color = Pa.TextTertiary,
) {
    if (rememberReduceMotion()) {
        Row(modifier, verticalAlignment = Alignment.CenterVertically) {
            repeat(3) { i ->
                if (i > 0) Spacer(Modifier.width(dotSize))
                androidx.compose.foundation.layout.Box(
                    Modifier.size(dotSize).background(color.copy(alpha = 0.45f), CircleShape),
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
                animation = tween(durationMillis = 420, easing = LinearEasing),
                repeatMode = RepeatMode.Restart,
                initialStartOffset = StartOffset(i * 150),
            ),
            label = "dot$i",
        )
    }

    Row(modifier, verticalAlignment = Alignment.Bottom) {
        phases.forEachIndexed { i, o ->
            if (i > 0) Spacer(Modifier.width(dotSize * 0.9f))
            // 抛物线 4x(1-x):0 起步、中点最高、回落到 0,比 sin 更自然的"跳"
            val lift = 4f * o.value * (1f - o.value)
            val alpha = 0.55f + 0.45f * lift
            androidx.compose.foundation.layout.Box(
                Modifier
                    .height(dotSize)
                    .graphicsLayer {
                        translationY = -lift * dotSize.toPx() * 1.6f
                        this.alpha = alpha
                    }
                    .size(dotSize)
                    .background(color, CircleShape),
            )
        }
    }
}
