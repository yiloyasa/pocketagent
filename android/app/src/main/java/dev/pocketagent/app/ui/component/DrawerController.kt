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
 * 抽屉"跟手"位移(纯函数,单测钉死 DrawerControllerLogicTest)。
 *
 * 【口径】进度 1 = 完全展开(偏移 0),进度 0 = 完全收起(偏移 = -面板宽)。
 * 面板宽必须用**真实 dp 换算出的像素** —— 之前用"屏宽 × 比例"估算,与实际面板
 * 宽(330dp/76dp)差出几十 px,收起态会在屏幕左缘留下一条永远合不拢的缝。
 * 入参超界按边界收敛:手势的累计误差不允许外溢到偏移上。
 */
internal fun drawerFollowOffsetPx(progress: Float, panelWidthPx: Float): Float =
    -panelWidthPx * (1f - progress.coerceIn(0f, 1f))

/**
 * 拖拽速度采样(纯逻辑,时钟可注入,单测钉死 DrawerControllerLogicTest)。
 *
 * 【为什么存在】松手吸附要用"手指此刻的速度"做 spring 的 initialVelocity ——
 * 手指甩多快面板就带多少动量落位,而不是清零后从静止重新加速。不交接速度的
 * "跟手"只做对了一半:拖是跟了,松手那一下仍然"像被接管"。
 *
 * 【采样口径】指数平滑的单帧差分。窗口 120ms:更老的样本反映的是"之前"的速度,
 * 松手要交接的是"此刻"的速度;断流(超过窗口)说明手指停过,速度清零。
 */
internal class DragVelocityTracker(
    /** 采样窗口:两次样本间隔超过它视为断流,速度归零。 */
    private val windowMs: Long = 120L,
) {
    private var lastSampleMs = 0L
    private var lastFraction = 0f
    private var velocity = 0f

    /** 当前平滑后的速度(进度/秒)。 */
    val current: Float get() = velocity

    /**
     * 记一帧样本。
     * @param fraction 当前进度 0..1([DrawerController.onDrag] 已 clamp,这里不再重复)
     * @param nowMs    单调时钟毫秒(生产传 System.currentTimeMillis,测试可注入)
     */
    fun sample(fraction: Float, nowMs: Long) {
        val dtMs = nowMs - lastSampleMs
        if (dtMs in 1..windowMs) {
            val v = (fraction - lastFraction) / (dtMs / 1000f)
            // 0.6/0.4 平滑:单帧差分噪声大,不平滑的话松手瞬间会抖一下。
            // 与 SwipeRevealRow 的松手采样同一套窗口与系数,全 app 手感一致。
            velocity = if (velocity == 0f) v else velocity * 0.6f + v * 0.4f
        } else {
            velocity = 0f
        }
        lastSampleMs = nowMs
        lastFraction = fraction
    }

    /** 清空采样(手势被取消时调用,避免旧速度漏进下一次松手)。 */
    fun reset() {
        velocity = 0f
        lastSampleMs = 0L
        lastFraction = 0f
    }
}

/**
 * 侧边栏"跟手"控制器。
 *
 * 【为什么要它】上一版侧边栏是"手势触发 → 面板自己播入场动画":手指在动、面板不动,
 * 松手后才"啪"地滑出来 —— 这就是用户说的"不跟手"。
 * 真正跟手的做法是让 **手指位移直接驱动面板位移**,松手再按位移/方向吸附。
 * 所以把进度(0..1)提升成一个共享状态:手势写它、面板读它,天然同帧。
 *
 * 【松手速度交接(丝滑的第二半】拖动中每帧采样进度([DragVelocityTracker]),
 * 松手时用最近的采样差算出"手指此刻的速度",作为 initialVelocity 交给
 * [PaMotion.sheet] 的 spring —— 手指甩多快面板就带多少动量落位,而不是清零后
 * 从静止重新加速。spring 本身轻微过冲(ζ=0.85),落位有"贴上去"的物理感。
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

    private val tracker = DragVelocityTracker()

    /** 0=完全关闭,1=完全展开。读取它会订阅重组。 */
    val progress: Float get() = anim.value

    /** 是否还需要渲染(留 0.1% 余量,保证退场动画能播完再卸载)。 */
    val visible: Boolean get() = anim.value > 0.001f

    /** 手势拖动中:直接 snap 到手指位置(不插值,才有"丝滑跟手"的手感)。 */
    fun onDrag(fraction: Float) {
        val clamped = fraction.coerceIn(0f, 1f)
        tracker.sample(clamped, System.currentTimeMillis())
        scope.launch { anim.snapTo(clamped) }
    }

    /** 松手:按位移决定展开到底还是弹回,并把这最后一刻的速度交给 spring。 */
    fun onRelease(open: Boolean) {
        val v = tracker.current
        tracker.reset()
        scope.launch {
            anim.animateTo(if (open) 1f else 0f, PaMotion.sheet(), initialVelocity = v)
        }
    }

    fun open() = onRelease(true)
    fun close() = onRelease(false)

    /** 抽屉内左滑关闭:与边缘手势共用同一进度,方向相反。 */
    fun onDragClose(deltaFraction: Float) {
        val next = (anim.value + deltaFraction).coerceIn(0f, 1f)
        tracker.sample(next, System.currentTimeMillis())
        scope.launch { anim.snapTo(next) }
    }
}

@Composable
fun rememberDrawerController(): DrawerController {
    val scope = rememberCoroutineScope()
    return remember(scope) { DrawerController(scope) }
}
