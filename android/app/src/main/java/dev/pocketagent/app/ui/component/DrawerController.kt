package dev.pocketagent.app.ui.component

import androidx.compose.animation.core.Animatable
import androidx.compose.runtime.Composable
import androidx.compose.runtime.Stable
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import dev.pocketagent.app.ui.anim.PaMotion
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.launch

/**
 * 侧边栏"跟手"控制器。
 *
 * 【为什么要它】上一版侧边栏是"手势触发 → 面板自己播入场动画":手指在动、面板不动,
 * 松手后才"啪"地滑出来 —— 这就是用户说的"不跟手"。
 * 真正跟手的做法是让 **手指位移直接驱动面板位移**,松手再按位移/方向吸附。
 * 所以把进度(0..1)提升成一个共享状态:手势写它、面板读它,天然同帧。
 *
 * 【松手速度交接(丝滑的第二半】拖动中每帧采样进度,松手时用最近的采样差算出
 * "手指此刻的速度",作为 initialVelocity 交给 [PaMotion.sheet] 的 spring ——
 * 手指甩多快面板就带多少动量落位,而不是清零后从静止重新加速。
 * 不交接速度的"跟手"只做对了一半:拖是跟了,松手那一下仍然"像被接管"。
 *
 * 用法:
 * ```
 * val drawer = rememberDrawerController()
 * Box(Modifier.edgeSwipeToOpenDrawer(
 *     onDragFraction = drawer::onDrag,
 *     onRelease = drawer::onRelease,
 * ))
 * if (drawer.visible) AppDrawer(progress = drawer.progress, onClose = drawer::close, ...)
 * ```
 */
@Stable
class DrawerController internal constructor(private val scope: CoroutineScope) {
    internal val anim = Animatable(0f)

    /** 0=完全关闭,1=完全展开。读取它会订阅重组。 */
    val progress: Float get() = anim.value

    /** 是否还需要渲染(留 0.1% 余量,保证退场动画能播完再卸载)。 */
    val visible: Boolean get() = anim.value > 0.001f

    // ---- 速度采样:指数平滑的单帧差分。窗口 120ms:更老的样本反映的是"之前"的
    // 速度,松手要交接的是"此刻"的速度;断流(>120ms)说明手指停过,速度清零。 ----
    private var lastSampleMs = 0L
    private var lastSampleFraction = 0f
    private var velocity = 0f

    private fun sample(fraction: Float) {
        val now = System.currentTimeMillis()
        val dtMs = now - lastSampleMs
        if (dtMs in 1..120) {
            val v = (fraction - lastSampleFraction) / (dtMs / 1000f)
            // 0.6/0.4 平滑:单帧差分噪声大,不平滑的话松手瞬间会抖一下
            velocity = if (velocity == 0f) v else velocity * 0.6f + v * 0.4f
        } else {
            velocity = 0f
        }
        lastSampleMs = now
        lastSampleFraction = fraction
    }

    /** 手势拖动中:直接 snap 到手指位置(不插值,才有"丝滑跟手"的手感)。 */
    fun onDrag(fraction: Float) {
        val clamped = fraction.coerceIn(0f, 1f)
        sample(clamped)
        scope.launch { anim.snapTo(clamped) }
    }

    /** 松手:按位移决定展开到底还是弹回,并把这最后一刻的速度交给 spring。 */
    fun onRelease(open: Boolean) {
        val v = velocity
        velocity = 0f
        scope.launch {
            anim.animateTo(if (open) 1f else 0f, PaMotion.sheet(), initialVelocity = v)
        }
    }

    fun open() = onRelease(true)
    fun close() = onRelease(false)

    /** 抽屉内左滑关闭:与边缘手势共用同一进度,方向相反。 */
    fun onDragClose(deltaFraction: Float) {
        val next = (anim.value + deltaFraction).coerceIn(0f, 1f)
        sample(next)
        scope.launch { anim.snapTo(next) }
    }
}

@Composable
fun rememberDrawerController(): DrawerController {
    val scope = rememberCoroutineScope()
    return remember(scope) { DrawerController(scope) }
}
