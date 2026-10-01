package dev.pocketagent.app.ui.component

import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.spring
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import dev.pocketagent.app.ui.anim.rememberReduceMotion
import dev.pocketagent.app.ui.theme.Pa
import dev.pocketagent.app.ui.theme.PaEditorial
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlin.math.PI
import kotlin.math.sin

/**
 * 「电脑端正在思考」三点指示器。
 *
 * 【这一版的动效取向:呼吸式 + 真物理 spring】
 *  - 位移只留 2.5dp(点径的 0.5 倍),远看几乎不动,近看有轻微起伏;
 *  - 主导变化是透明度 0.30 → 1.00 与缩放 0.82 → 1.00,像"脉冲"而不是"蹦跳";
 *  - 周期驱动用 **Animatable + spring 临界阻尼往返**(而非 tween 定时循环):
 *    spring 的速度曲线两端自然归零,没有定时器到点"掐断"的生硬感;
 *    本版 Compose 的 infiniteRepeatable 只收 DurationBasedAnimationSpec
 *    (spring 无固定时长进不去),所以用协程 while 循环驱动 —— 这也是"真 spring"
 *    的唯一通路,组件离开组合时协程随 LaunchedEffect 一并取消,不泄漏。
 *  - 三点相位差 160ms;单点呼吸周期 ~0.82s(stiffness 95 临界阻尼,见下),
 *    三点整体构成 600-900ms 体感区间的连续波浪;只动 graphicsLayer
 *    (scale/alpha/translationY),不触发布局,流式期间逐帧文本更新也不会
 *    放大它的开销。
 *
 * 【无障碍】减弱动态效果时退化为静态三点(只保留透明度差)。
 */
@Composable
fun TypingDots(
    modifier: Modifier = Modifier,
    dotSize: Dp = 6.dp,
    color: Color = PaEditorial.Primary,
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

    // 每点一条 0..1 相位的 spring 往返协程;相位差靠错峰启动(160ms/点)
    val lifts = List(3) { remember { Animatable(0f) } }
    LaunchedEffect(Unit) {
        lifts.forEachIndexed { i, anim ->
            launch {
                delay(i * 160L)
                while (true) {
                    // 临界阻尼 spring stiffness=95:ω≈9.75,半程 settle≈0.41s,
                    // 整周期 ≈0.82s —— 落在"呼吸自然"的 600-900ms 体感区间
                    // (旧值 55 周期 ~1.1s,肉眼可辨的"慢半拍")。
                    anim.animateTo(1f, spring(dampingRatio = 1f, stiffness = 95f))
                    anim.animateTo(0f, spring(dampingRatio = 1f, stiffness = 95f))
                }
            }
        }
    }

    Row(modifier, verticalAlignment = Alignment.CenterVertically) {
        lifts.forEachIndexed { i, anim ->
            if (i > 0) Spacer(Modifier.width(gap))
            // sin 曲线:0 → 1 → 0,比抛物线更平滑(两端导数为 0,没有生硬的起停)
            val lift = sin(PI * anim.value).toFloat()
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
