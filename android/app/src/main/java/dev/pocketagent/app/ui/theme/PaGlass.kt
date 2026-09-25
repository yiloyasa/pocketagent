package dev.pocketagent.app.ui.theme

import android.os.Build
import androidx.compose.animation.core.animateDpAsState
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.foundation.background
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.composed
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Outline
import androidx.compose.ui.graphics.Shape
import androidx.compose.ui.graphics.TransformOrigin
import androidx.compose.ui.graphics.asComposeRenderEffect
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import dev.pocketagent.app.ui.anim.PaMotion
import dev.pocketagent.app.ui.anim.rememberReduceMotion

/**
 * 磨砂玻璃材质系统（UI 改版 ·「高级感」的地基）。
 *
 * 【材质观】玻璃不是一个颜色，是三件事的叠加：
 *  1. **透** —— 半透明底色（[PaGlassToken.Tint]），让下层内容隐约透出来；
 *  2. **亮边** —— 顶缘一条 1px 高光发丝线（[PaGlassToken.Highlight]），
 *     模拟"光打在玻璃上边缘"的物理反馈，这是玻璃读作"玻璃"的关键一笔；
 *  3. **推背模糊** —— 浮层（抽屉/Sheet）出现时，被覆盖的内容自身播放
 *     模糊+轻微缩小（[Modifier.paPushBack]），透过玻璃看过去就是真磨砂。
 *
 * 【为什么模糊加在"被覆盖内容"上而不是浮层自己】Compose 的 Modifier.blur 模糊
 * 的是自己这一层的绘制内容，不是"背后已经画完的东西"；不引第三方库（依赖极简
 * 铁律）拿不到真正的 backdrop-blur，于是把方向反过来：浮层半透明，内容层模糊。
 * 视觉等价，且模糊半径可以随浮层开合**动画**（材质"浮现"而不是硬切）。
 *
 * 【性能纪律】模糊是 GPU 大户。纪律：
 *  - 只对**整屏内容层**在浮层开合期间模糊（一次一层），绝不逐气泡/逐卡片加 blur；
 *  - API < 31 RenderEffect 不存在，自动退化为纯缩放+压暗（降级是"少一层质感"，
 *    不是"换一种错的样子"）；
 *  - 系统减弱动效开启时不播模糊过渡，直接到位。
 */
object PaGlassToken {
    /** 玻璃底色：深色=近黑半透（读作"烟玻璃"），浅色=近白半透（读作"雾玻璃"）。 */
    var Tint by mutableStateOf(Color(0xE6151517)); internal set

    /** 顶缘发丝高光：白系；深色主题更亮（暗环境里光边更明显）。 */
    var Highlight by mutableStateOf(Color(0x2EFFFFFF)); internal set

    /** 浮层打开时给被覆盖内容的压暗色（配合模糊做出"退后一层"的深度）。 */
    var Scrim by mutableStateOf(Color(0x66000000)); internal set

    internal fun apply(dark: Boolean, hacker: Boolean = false) {
        when {
            // 【黑客终端】深绿玻璃:磷光绿世界的"烟玻璃"带绿相,不发灰;
            // 发丝高光用绿光(#33FF66)且可见度调低(0x24)——CRT 暗房里高光只该"隐约透边",
            // 沿用白系高光会在纯黑上刺眼;Scrim 加深(0x73)让浮层下的内容退得更远。
            hacker -> {
                Tint = Color(0xE60C160C); Highlight = Color(0x2433FF66); Scrim = Color(0x73000000)
            }
            dark -> {
                Tint = Color(0xE6151517); Highlight = Color(0x2EFFFFFF); Scrim = Color(0x66000000)
            }
            else -> {
                Tint = Color(0xF2FFFFFF); Highlight = Color(0x5CFFFFFF); Scrim = Color(0x33201E2C)
            }
        }
    }
}

/** 本机是否具备真模糊能力（RenderEffect，API 31+）。低版本静默降级为纯位移/缩放。 */
@Composable
fun rememberGlassBlurAvailable(): Boolean = remember {
    Build.VERSION.SDK_INT >= Build.VERSION_CODES.S
}

/**
 * 玻璃面：半透明底 + 顶缘发丝高光。
 *
 * 这个 Modifier 只负责"材质长得像玻璃"（透+亮边），不负责模糊——模糊由
 * [Modifier.paPushBack] 在内容层做。职责分离：材质静态、模糊动态，两层可以
 * 独立开关、独立测试。
 *
 * @param tint 玻璃底色；默认 [PaGlassToken.Tint]。需要更"透"或更"实"的部位
 *   显式传：导航栏实一点（alpha 0.92）、卡片透一点（0.8）——同一材质家族里的
 *   浓淡就是层级。**玻璃上永远不再叠第二层玻璃**（透上透读不清，材质层级铁律）。
 * @param hairline 顶缘 1px 高光线。顶缘悬空的（顶栏、卡片）必须开——亮边朝光；
 *   底缘贴边的容器关掉它（贴边玻璃的边界靠屏幕边，不靠亮边）。
 * @param shadowElevation 悬浮玻璃（浮层卡）给一点投影；贴边的导航栏/编辑条给 0。
 */
fun Modifier.paGlass(
    shape: Shape = RoundedCornerShape(0.dp),
    tint: Color? = null,
    hairline: Boolean = true,
    shadowElevation: Dp = 0.dp,
): Modifier = composed {
    val glassTint = tint ?: PaGlassToken.Tint
    val highlight = PaGlassToken.Highlight
    val density = LocalDensity.current
    val hairlinePx = with(density) { 1.dp.toPx() }
    this
        .background(glassTint, shape)
        .drawBehind {
            if (!hairline || hairlinePx <= 0f) return@drawBehind
            val outline = shape.createOutline(size, layoutDirection, this)
            val radiusPx = when (outline) {
                is Outline.Rounded -> outline.roundRect.topLeftCornerRadius.x
                else -> 0f
            }
            // 发丝线只描顶缘：垂直渐变从高光到透明、只画 1px 高的描边
            val brush = Brush.verticalGradient(
                colors = listOf(highlight, Color.Transparent),
                startY = 0f,
                endY = hairlinePx,
            )
            drawRoundRect(
                brush = brush,
                topLeft = Offset(hairlinePx / 2, hairlinePx / 2),
                size = Size(size.width - hairlinePx, size.height - hairlinePx),
                cornerRadius = CornerRadius(radiusPx, radiusPx),
                style = Stroke(width = hairlinePx),
            )
        }
        .then(if (shadowElevation > 0.dp) Modifier.graphicsLayer {
            // 局部改名:graphicsLayer 自己有个同名 Float 属性,直接赋值会自赋
            val shadowPx = shadowElevation.toPx()
            this.shape = shape
            this.shadowElevation = shadowPx
        } else Modifier)
}

/**
 * 推背模糊：浮层激活时，内容层模糊 + 轻微缩小 + 压暗（材质在"浮现"，不是 fade）。
 *
 * 【用法】挂在**被覆盖的内容根**上（不是浮层上）：
 * `Content(Modifier.paPushBack(drawerOpen || sheetOpen))`。
 *
 * 【降级链】API < 31 → 无模糊，保留缩放+压暗；减弱动效 → 瞬时到位不播动画。
 */
fun Modifier.paPushBack(
    active: Boolean,
    blurRadius: Dp = 22.dp,
    scaleDown: Float = 0.965f,
): Modifier = composed {
    val canBlur = rememberGlassBlurAvailable()
    val reduceMotion = rememberReduceMotion()
    val radius by animateDpAsState(
        targetValue = if (active && canBlur) blurRadius else 0.dp,
        animationSpec = PaMotion.standard(),
        label = "paPushBackBlur",
    )
    val scale by animateFloatAsState(
        targetValue = if (active) scaleDown else 1f,
        animationSpec = if (reduceMotion) PaMotion.instant() else PaMotion.emphasized(),
        label = "paPushBackScale",
    )
    val radiusPx = with(LocalDensity.current) { radius.toPx() }
    val effect = remember(radiusPx) {
        if (radiusPx > 0.5f && Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
            android.graphics.RenderEffect.createBlurEffect(
                radiusPx, radiusPx, android.graphics.Shader.TileMode.CLAMP,
            ).asComposeRenderEffect()
        } else {
            null
        }
    }
    this.graphicsLayer {
        scaleX = scale
        scaleY = scale
        transformOrigin = TransformOrigin(0.5f, 0.35f) // 从内容重心后退，不是屏幕中心
        alpha = if (active) 0.94f else 1f
        renderEffect = effect
    }
}
