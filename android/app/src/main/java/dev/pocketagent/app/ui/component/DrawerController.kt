package dev.pocketagent.app.ui.component

import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.spring
import androidx.compose.runtime.Composable
import androidx.compose.runtime.Stable
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
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

    private val settle = spring<Float>(dampingRatio = 0.9f, stiffness = 900f)

    /** 手势拖动中:直接 snap 到手指位置(不插值,才有"丝滑跟手"的手感)。 */
    fun onDrag(fraction: Float) {
        scope.launch { anim.snapTo(fraction.coerceIn(0f, 1f)) }
    }

    /** 松手:按位移决定展开到底还是弹回。 */
    fun onRelease(open: Boolean) {
        scope.launch { anim.animateTo(if (open) 1f else 0f, settle) }
    }

    fun open() = onRelease(true)
    fun close() = onRelease(false)

    /** 抽屉内左滑关闭:与边缘手势共用同一进度,方向相反。 */
    fun onDragClose(deltaFraction: Float) {
        scope.launch { anim.snapTo((anim.value + deltaFraction).coerceIn(0f, 1f)) }
    }
}

@Composable
fun rememberDrawerController(): DrawerController {
    val scope = rememberCoroutineScope()
    return remember(scope) { DrawerController(scope) }
}
