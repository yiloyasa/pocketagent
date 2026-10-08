package dev.pocketagent.app.ui.anim

import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.clickable
import androidx.compose.foundation.combinedClickable
import androidx.compose.foundation.gestures.awaitEachGesture
import androidx.compose.foundation.gestures.awaitFirstDown
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsPressedAsState
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.composed
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.Shape
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.hapticfeedback.HapticFeedbackType
import androidx.compose.ui.input.pointer.PointerEventPass
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.input.pointer.positionChange
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalHapticFeedback
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.unit.Dp
import dev.pocketagent.app.ui.theme.PaEditorial
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/**
 * 按压动效(全 app 通用):按下轻微缩小 + 松手弹回。
 *
 * 【讲解】为什么用 spring 而不是 tween:真实物理感——手指抬起时
 * 不是匀速恢复,而是带一点回弹地"弹"回来,这是"高级感"的核心来源。
 * dampingRatio 越小回弹越明显(0.5 明显 / 1.0 无回弹),
 * stiffness 越大反应越快(420 是"跟手但不僵硬"的甜点值)。
 */
fun Modifier.pressable(
    scaleDown: Float = 0.97f,
    haptic: Boolean = false,
    enabled: Boolean = true,
    onClick: () -> Unit,
): Modifier = composed {
    val interaction = remember { MutableInteractionSource() }
    val pressed by interaction.collectIsPressedAsState()
    val scale by animateFloatAsState(
        targetValue = if (pressed) scaleDown else 1f,
        animationSpec = PaMotion.press(),
        label = "press",
    )
    val hf = LocalHapticFeedback.current
    this
        .graphicsLayer { scaleX = scale; scaleY = scale }
        .clickable(interactionSource = interaction, indication = null, enabled = enabled) {
            if (haptic) hf.performHapticFeedback(HapticFeedbackType.TextHandleMove)
            onClick()
        }
}

/** 带长按的按压缩放(会话卡片:单击打开/长按管理)。 */
@OptIn(ExperimentalFoundationApi::class)
fun Modifier.pressableLong(
    onClick: () -> Unit,
    onLongPress: () -> Unit,
): Modifier = composed {
    val interaction = remember { MutableInteractionSource() }
    val pressed by interaction.collectIsPressedAsState()
    val scale by animateFloatAsState(
        targetValue = if (pressed) 0.97f else 1f,
        animationSpec = PaMotion.press(),
        label = "pressLong",
    )
    this
        .graphicsLayer { scaleX = scale; scaleY = scale }
        .combinedClickable(
            interactionSource = interaction,
            indication = null,
            onClick = onClick,
            onLongClick = onLongPress,
        )
}

/** 圆形/圆角方形图标按钮(顶部工具栏、发送键等),带按压缩放。 */
@Composable
fun IconActionButton(
    iconRes: Int,
    contentDescription: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    size: Dp = 42.dp,
    iconSize: Dp = 20.dp,
    bg: Color = MaterialTheme.colorScheme.surfaceVariant,
    // 传 null 表示沿用主题默认图标色;需要"激活态反色"的按钮(如搜索开关)才显式传色。
    tint: Color? = null,
    shape: Shape = RoundedCornerShape(PaEditorial.Radius.IconButton),
) {
    Box(
        modifier = modifier
            .size(size)
            .pressable { onClick() }
            .background(bg, shape),
        contentAlignment = Alignment.Center,
    ) {
        Icon(
            painterResource(iconRes), contentDescription,
            tint = tint ?: MaterialTheme.colorScheme.onBackground,
            modifier = Modifier.size(iconSize),
        )
    }
}

/** 分区标题行:左侧标题+副标题,右侧尾部组件(如"全部 →")。 */
@Composable
fun SectionHeader(
    title: String,
    subtitle: String? = null,
    modifier: Modifier = Modifier,
    trailing: @Composable () -> Unit = {},
) {
    Row(
        modifier.fillMaxWidth().padding(vertical = 4.dp),
        horizontalArrangement = androidx.compose.foundation.layout.Arrangement.SpaceBetween,
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Column {
            Text(title, fontSize = PaEditorial.Type.Heading.fontSize, color = MaterialTheme.colorScheme.onBackground)
            if (subtitle != null) {
                Text(subtitle, fontSize = PaEditorial.Type.Caption.fontSize, color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.padding(top = 2.dp))
            }
        }
        trailing()
    }
}

/**
 * 品牌双描边(duotone)母题(2026-10-09 精修波 D6):Canvas/Path 自绘的 2 色
 * 线性图形 —— 主色 + 哑色双色描边,与 lucide 细线图标同一语言(细描边、圆头、
 * 不填色),不引入任何图片资产(资源极简铁律)。
 *
 * 【三个母题】发现环(Home/连接)、配对(Onboarding)、审批(EmptyState 按语境选)。
 * 色从 PaEditorial 语义槽位取(Primary/Subtle),三盘换肤即时跟随,不造新色值。
 */
enum class PaDuotoneMotif { DISCOVER_RING, PAIR, APPROVAL }

/**
 * 渲染一个 duotone 母题图形(正方形画布,默认 88dp;笔画 2dp 与 lucide 图标同量级)。
 * 纯装饰(contentDescription 由调用方语境给出),不动效(空态图形要"安静")。
 */
@Composable
fun PaDuotoneArt(
    motif: PaDuotoneMotif,
    modifier: Modifier = Modifier,
    artSize: Dp = 88.dp,
    strokeWidth: Dp = 2.dp,
) {
    val primary = PaEditorial.Primary // 主色:当前操作/品牌
    val muted = PaEditorial.Subtle // 哑色:次级结构
    Canvas(modifier.size(artSize)) {
        val k = size.minDimension / 100f // 归一化 100×100 坐标 → 画布
        val stroke = Stroke(width = strokeWidth.toPx(), cap = StrokeCap.Round, join = StrokeJoin.Round)
        when (motif) {
            PaDuotoneMotif.DISCOVER_RING -> {
                // 发现环:外环(哑)+ 内环(主色)+ 圆心点(主色)+ 右上两道广播弧
                // —— "正在找你的电脑"的雷达意象,与 Home 发现态同一母题
                val c = center
                drawCircle(color = muted, radius = 46f * k, center = c, style = stroke)
                drawCircle(color = primary, radius = 30f * k, center = c, style = stroke)
                drawCircle(color = primary, radius = 7f * k, center = c)
                drawArc(
                    color = primary,
                    startAngle = -78f, sweepAngle = 38f, useCenter = false,
                    topLeft = Offset(c.x - 38f * k, c.y - 38f * k),
                    size = Size(76f * k, 76f * k),
                    style = stroke,
                )
                drawArc(
                    color = primary,
                    startAngle = -30f, sweepAngle = 20f, useCenter = false,
                    topLeft = Offset(c.x - 38f * k, c.y - 38f * k),
                    size = Size(76f * k, 76f * k),
                    style = stroke,
                )
            }
            PaDuotoneMotif.PAIR -> {
                // 配对:左手机(哑色圆角矩形)+ 右显示器(主色)+ 连接线 + 中央配对点
                // —— 两台设备经一个"认亲"的点接起来
                drawRoundRect(
                    color = muted,
                    topLeft = Offset(6f * k, 28f * k),
                    size = Size(24f * k, 44f * k),
                    cornerRadius = CornerRadius(5f * k, 5f * k),
                    style = stroke,
                )
                drawRoundRect(
                    color = primary,
                    topLeft = Offset(60f * k, 24f * k),
                    size = Size(34f * k, 26f * k),
                    cornerRadius = CornerRadius(4f * k, 4f * k),
                    style = stroke,
                )
                // 显示器支脚
                drawLine(muted, Offset(77f * k, 50f * k), Offset(77f * k, 58f * k), stroke.width, StrokeCap.Round)
                drawLine(muted, Offset(68f * k, 58f * k), Offset(86f * k, 58f * k), stroke.width, StrokeCap.Round)
                // 连接线 + 中央配对点(主色实心,配对码的落点)
                drawLine(muted, Offset(30f * k, 50f * k), Offset(60f * k, 37f * k), stroke.width, StrokeCap.Round)
                drawCircle(color = primary, radius = 4.5f * k, center = Offset(45f * k, 43.5f * k))
            }
            PaDuotoneMotif.APPROVAL -> {
                // 审批:盾牌(主色描边)+ 盾内对勾(哑色)—— "执行前要过你这一关"
                val shield = Path().apply {
                    moveTo(50f * k, 16f * k)
                    lineTo(75f * k, 26f * k)
                    lineTo(75f * k, 50f * k)
                    lineTo(50f * k, 84f * k)
                    lineTo(25f * k, 50f * k)
                    lineTo(25f * k, 26f * k)
                    close()
                }
                drawPath(shield, color = primary, style = stroke)
                val check = Path().apply {
                    moveTo(37f * k, 50f * k)
                    lineTo(47f * k, 60f * k)
                    lineTo(65f * k, 40f * k)
                }
                drawPath(check, color = muted, style = stroke)
            }
        }
    }
}

/**
 * 空状态:图标 + 主文案 + 副文案(参考稿"连接电脑后,任务会出现在这里")。
 *
 * 【D6 图形槽位】[motif] 给了就渲染 duotone 品牌母题(替代单色 lucide 图标,
 * 主色+哑色双描边,见 [PaDuotoneArt]);null(默认)保持原单色图标 —— 向后
 * 兼容,调用点按语境逐个配图形,不配的不动。
 */
@Composable
fun EmptyState(
    iconRes: Int,
    title: String,
    hint: String,
    modifier: Modifier = Modifier,
    motif: PaDuotoneMotif? = null,
) {
    Column(
        modifier.fillMaxWidth().padding(vertical = 48.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        if (motif != null) {
            PaDuotoneArt(motif, Modifier.padding(bottom = 16.dp))
        } else {
            Icon(painterResource(iconRes), null, tint = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.size(44.dp))
        }
        Text(title, Modifier.padding(top = 14.dp), fontSize = PaEditorial.Type.Body.fontSize, color = MaterialTheme.colorScheme.onBackground)
        Text(hint, Modifier.padding(top = 4.dp), fontSize = PaEditorial.Type.Caption.fontSize, color = MaterialTheme.colorScheme.onSurfaceVariant)
    }
}

/**
 * 左边缘右滑唤出侧边栏。
 *
 * 【起手区间是实测出来的】
 *  - 最左侧 ~24dp 是**系统返回手势区**(HyperOS 实测 x<24dp 的横滑直接退到桌面),
 *    那一段我们永远拿不到,所以不指望它;
 *  - 起手点放宽到 [edgeWidth](80dp,约屏宽 8%),"边缘往里一点"右滑即可唤出。
 *
 * 【为什么不能更宽(与行内左滑的冲突)】96dp 时几乎盖住列表条目的左半边:
 * 用户想左滑某行却在边缘起手 → 先被判成"右滑开抽屉"。现在收到 80dp,
 * 并由调用方在**有行展开时**传 enabled=false,两种手势彻底分开。
 *
 * 【灵敏度调优(修复"还是不够灵敏")】三处放宽,针对实测的三种失败场景:
 *  1. 起手点偏内(>64dp)直接判定失败 → 命中带从 64dp 放宽到 80dp
 *     (系统吃掉 24dp 后,可用带从 ~40dp 增至 ~56dp);
 *  2. 手指轻微上下抖,纵向位移累计超过 28dp 就永久 break →
 *     改为**按比例判定**(纵向 > 横向 1.6 倍且 > 32dp 才让给滚动),
 *     单纯增大的纵向绝对值不再误杀;
 *  3. 松手必须横拖满 33% 屏宽(约 120dp)才吸附展开,轻扫必回弹 →
 *     阈值降到 18% 屏宽,并叠加"速度够快也算数"(快扫是小位移的常见形态)。
 *
 * 【为什么要用 Initial 传球(修复"偶尔滑不动")】
 * 之前用 detectHorizontalDragGestures 挂在根节点,事件先给子节点(列表滚动、行内左滑),
 * 子节点一旦 consume,根节点的识别器就被取消 —— 表现就是"滑了没反应"。
 * 现在在 Initial 阶段监听:判定为右滑并 consume 之后,子节点再也收不到这段位移。
 *
 * 【误触保护】纵向位移明显压过横向 → 判定为滚动列表,立即放弃;
 * 起手后往左拉 → 判定为行内左滑,立即放弃。两者都不 consume。
 */
fun Modifier.edgeSwipeToOpenDrawer(
    enabled: Boolean = true,
    edgeWidth: Dp = 80.dp,
    threshold: Dp = 28.dp,
    /** 旧式用法:滑够阈值就立即打开(面板自己播动画)。 */
    onOpen: (() -> Unit)? = null,
    /** 跟手用法:拖动中实时上报 0..1 进度,由调用方驱动面板位移。 */
    onDragFraction: ((Float) -> Unit)? = null,
    /** 跟手用法:松手时告知是否应展开到底。 */
    onRelease: ((Boolean) -> Unit)? = null,
): Modifier = composed {
    if (!enabled) return@composed this
    val density = androidx.compose.ui.platform.LocalDensity.current
    val edgePx = with(density) { edgeWidth.toPx() }
    val thresholdPx = with(density) { threshold.toPx() }
    // 纵向否决:要求"纵向 > 横向 * 1.6 且纵向 > 32dp"才让给列表滚动。
    // 原来只看纵向绝对值(>28dp),手指轻微抖动累计到 28dp 就永久放弃 —— 这是
    // "滑不动"的头号原因。改成比例判定后,只要整体是横向意图就继续跟手。
    val verticalGiveUpPx = with(density) { 32.dp.toPx() }
    val verticalRatio = 1.6f
    // 进入"跟手"的门槛:8dp。比放弃阈值(28dp)小得多 ——
    // 手指一动面板就跟,同时给纵向滚动/行内左滑留出判定的余地。
    val startPx = with(density) { 8.dp.toPx() }
    // 松手吸附:位移过 18% 屏宽就展开;不足但有明显右扫速度(v>700px/s)也展开,
    // 照顾"快速轻扫"——这种手势位移天然小,原来卡在 33% 门槛上必然回弹。
    val settleFraction = 0.18f
    val flingVelocityPx = with(density) { 700.dp.toPx() }

    this.pointerInput(edgePx, thresholdPx) {
        awaitEachGesture {
            // Initial 传球:在子节点(列表滚动/行内左滑)之前拿到事件,起点稍偏也不会被吃掉
            val down = awaitFirstDown(requireUnconsumed = false, pass = PointerEventPass.Initial)
            if (down.position.x > edgePx) return@awaitEachGesture
            var horizontal = 0f
            var vertical = 0f
            var tracking = false
            var lastVelocityX = 0f
            while (true) {
                val event = awaitPointerEvent(PointerEventPass.Initial)
                val change = event.changes.firstOrNull { it.id == down.id } ?: break
                if (!change.pressed) break
                horizontal += change.positionChange().x
                vertical += kotlin.math.abs(change.positionChange().y)
                // 【速度兜底】positionChange 只看位移,拿不到速度;用位移/事件间隔自己算。
                // 快扫的位移天然小,只靠位移阈值会把"快速轻扫"判成回弹。
                val dtMs = (change.uptimeMillis - down.uptimeMillis).coerceAtLeast(1L)
                lastVelocityX = horizontal / (dtMs / 1000f)
                if (!tracking) {
                    // 明显往左:是行内左滑,不是开抽屉
                    if (horizontal < -thresholdPx) break
                    // 纵向明显压过横向:交给列表去滚动(按比例,不看纵向绝对值)
                    if (vertical > verticalGiveUpPx &&
                        vertical > kotlin.math.abs(horizontal) * verticalRatio
                    ) {
                        break
                    }
                    if (horizontal >= startPx) tracking = true
                }
                if (tracking) {
                    change.consume()
                    val frac = (horizontal / size.width.toFloat()).coerceIn(0f, 1f)
                    if (onDragFraction != null) {
                        onDragFraction(frac)
                    } else if (horizontal >= thresholdPx) {
                        onOpen?.invoke()
                        break
                    }
                }
            }
            if (tracking) {
                val frac = (horizontal / size.width.toFloat()).coerceIn(0f, 1f)
                onRelease?.invoke(frac > settleFraction || lastVelocityX > flingVelocityPx)
            }
        }
    }
}

/**
 * 系统「减弱动态效果」是否开启(开发者选项里的"动画时长缩放"设为 0)。
 *
 * 【设计规范 §1.5】开启时:骨架屏取消平移、撤销条取消进度线动画、
 * 侧滑改为瞬时展开、抽屉取消滑入仅保留淡入。
 */
@Composable
fun rememberReduceMotion(): Boolean {
    val ctx = LocalContext.current
    return remember(ctx) {
        runCatching {
            android.provider.Settings.Global.getFloat(
                ctx.contentResolver,
                android.provider.Settings.Global.ANIMATOR_DURATION_SCALE,
                1f,
            ) == 0f
        }.getOrDefault(false)
    }
}
