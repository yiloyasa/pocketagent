package dev.pocketagent.app.ui.component

import androidx.activity.compose.BackHandler
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.scaleIn
import androidx.compose.animation.slideInHorizontally
import androidx.compose.animation.slideOutHorizontally
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.gestures.awaitEachGesture
import androidx.compose.foundation.gestures.awaitFirstDown
import androidx.compose.foundation.systemGestureExclusion
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.material3.TextField
import androidx.compose.material3.TextFieldDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.derivedStateOf
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.graphics.TransformOrigin
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.layout.onSizeChanged
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.LocalView
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.IntOffset
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.R as AppR
import dev.pocketagent.app.ui.anim.IconActionButton
import dev.pocketagent.app.ui.anim.PaMotion
import dev.pocketagent.app.ui.history.HistoryMatchSpan
import dev.pocketagent.app.ui.history.HistorySearchHit
import dev.pocketagent.app.ui.history.HistorySearchRepository
import dev.pocketagent.app.ui.history.HistorySearchState
import dev.pocketagent.app.ui.history.HistorySearchStatus
import dev.pocketagent.app.ui.history.ResultsKind
import dev.pocketagent.app.ui.theme.PaEditorial
import dev.pocketagent.app.ui.theme.PaGlassToken
import dev.pocketagent.app.ui.theme.PaHacker
import dev.pocketagent.app.ui.theme.paGlass
import kotlin.math.roundToInt
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

/**
 * 历史消息快速查看(2026-09-30 二版):左缘垂直刻度条 + **两段式**交互。
 *
 * 用户对一版的裁决:78% 宽浮层挡会话正文的字、交互不够丝滑。二版对齐视频样板
 * (ZCode 客户端左缘时间线)并做触屏翻译 —— 触屏没有 hover,"按住拖动"就是它的
 * 等价物:
 *
 *  - **A 按住拖动 = 实时刷选(R1,Telegram date-bar 模式)**:手指按住刻度条 →
 *    紧凑预览气泡贴左缘浮出(55% 屏宽、120dp 单卡、靠左,不遮正文主体);上下拖动
 *    气泡**进度驱动**跟手,**列表同时实时滚动到手指所指**(宿主 requestScrollToItem,
 *    非挂起逐帧安全)——"拖到哪看到哪",松手精确锚定;几乎没拖 = 取消只收气泡;
 *  - **B 轻点开浮层(去遮挡)**:预览出现前的快启轻点 → 完整列表,宽度
 *    78% → **62%**、scrim alpha 降到 **0.25**(隐约透出正文,强调"速查不是切页");
 *  - **C 不挡字硬保证**:刻度条视觉 10dp 全程在 23dp 页边距区内;气泡/浮层只在
 *    交互期间存在,松手即收;气泡竖直中心相对手指做 8dp 让位,不盖手指下的首行。
 *
 * 纯决策全在 [HistoryQuickNavLogic](JVM 单测钉住):下标映射、手势状态机、取消
 * 判定、气泡 Y 钳制。这里只做状态搬运与绘制。跳转由宿主(ChatScreen)回调实现。
 *
 * 【跟手 = 零帧卡顿】手指 Y 写进 [HistoryDragState.fingerY](mutableFloatStateOf):
 * 气泡位置走 `offset {}` 延迟读(逐帧只重排版不重组)、刻度高亮与气泡内容走
 * `derivedStateOf`(只在"指数变了"时重绘/重组)—— 整条拖动路径上没有任何
 * 逐帧重组。
 *
 * 【三色盘同构】常规模式 Surface/Raised 底;黑客模式 GraphiteDrawer 底 + 等宽字
 * (契约见 PaHacker)。全部取值走 editorial 槽位,无硬编码色。
 */

/** 刻度条视觉宽度(10dp 规格,半透明;贴在 23dp 页边距区内,不压正文)。 */
private val TickBarVisualWidth = 10.dp
/**
 * 刻度条触控宽度(40dp)。【真机 2026-10-01】视觉条贴屏幕左缘,而 MIUI/手势导航的
 * 返回手势带恰好也是边缘 ~24dp:x<24dp 的拖动会被系统**非确定性**吞掉(实测同一
 * 位置 2.5s 拖动可达、6s 拖动被拦),"按在边缘上有时灵有时不灵"。触控带加宽到
 * 40dp 后手指自然落在 15-30dp 一段 —— 稳定越过手势带;前 24dp 仍保留,能按到。
 * 超出 23dp 页边距的部分只盖正文列最左侧 17dp 的纯文本,无交互元素,可接受。
 */
private val TickBarTouchWidth = 40.dp
/** 单个刻度的高度与自然间距 —— 绘制与"手指↔下标映射"共用同一套几何。 */
private val TickHeight = 2.dp
private val TickGap = 7.dp

/** 按住多久浮出预览气泡。短到"即时"的体感,又给轻点留出判定窗。 */
private const val PreviewHoldMs = 120L
/** "几乎没拖"= 取消的位移阈值:预览中松手时位移小于它就不跳转。 */
private val CancelSlop = 20.dp

/** 预览气泡:宽 ~55% 屏宽,贴左缘,竖直让位 8dp。 */
private val BubbleWidthFraction = 0.55f
/**
 * 预览气泡定位钳制的名义高度(v3 气泡改自适应高度 64..200dp,定位钳制用
 * 名义值即可——贴近真实高度,边缘出入无感知)。
 */
private val BubbleHeight = 120.dp
private val BubbleYield = 8.dp
private val BubbleStartGap = 16.dp

/** 完整列表浮层:宽度 78% → 62%(一版用户裁决:挡字),scrim 降到 0.25。 */
private val PanelWidthFraction = 0.62f
private const val PanelScrimAlpha = 0.25f

/**
 * 按住拖动手势的实时 UI 状态(宿主持有一份,刻度条写、气泡读)。
 *
 * 拆成三个原子状态是刻意的:phase 变更稀少(驱动 AnimatedVisibility),
 * fingerY 逐帧变但只在 draw/offset 延迟读里被消费 —— 逐帧写它不触发任何重组。
 */
class HistoryDragState {
    var phase by mutableStateOf(HistoryQuickNavLogic.NavGesturePhase.IDLE)
        internal set
    var fingerY by mutableFloatStateOf(0f)
        internal set
    /** 刻度条(=列表容器)高度 px,气泡 Y 钳制用;onSizeChanged 登记。 */
    var heightPx by mutableFloatStateOf(0f)
        internal set

    internal fun apply(g: HistoryQuickNavLogic.NavGesture) {
        phase = g.phase
        fingerY = g.y
    }
}

/**
 * 刻度条手势:按下 → 预览(跟手) → 松手裁定。决策全在 [HistoryQuickNavLogic]。
 *
 * 回调经 rememberUpdatedState 转接、计时器挂 rememberCoroutineScope:
 * pointerInput 只按 count 重启 —— 否则宿主每次重组(流式输出时很频繁)换新
 * lambda 实例会掐断进行中的拖动手势。计时器不能放 awaitEachGesture 块内
 * (那是 AwaitPointerEventScope 限制作用域,无 CoroutineScope/无可挂起延迟),
 * 外挂 scope 与指针事件同在主线程,对捕获变量的先后读写无数据竞争。
 */
@Composable
private fun rememberHistoryDragGesture(
    drag: HistoryDragState,
    count: Int,
    onOpenPanel: () -> Unit,
    onJump: (Int) -> Unit,
): Modifier {
    val openPanel by rememberUpdatedState(onOpenPanel)
    val jump by rememberUpdatedState(onJump)
    val scope = rememberCoroutineScope()
    // 【触感分级】松手跳转=确认级动作(导航已发生),走中档 PaHaptics.confirm,
    // 与撤销/审批提交同强度;拖动途中的跨档仍是轻档 light(见 HistoryTickBar)。
    val hapticView = LocalView.current
    return Modifier.pointerInput(count) {
        // pointerInput 块的 receiver 是 Density:刻度几何常数在这里换算成 px
        val tickHPx = TickHeight.toPx()
        val gapPx = TickGap.toPx()
        val cancelSlopPx = CancelSlop.toPx()
        awaitEachGesture {
            val down = awaitFirstDown(requireUnconsumed = false)
            var g = HistoryQuickNavLogic.pressed(down.position.y)
            drag.apply(g)
            var settled = false
            // 按住判定窗:超时未松 → 预览浮出。所有收场路径(正常松手/手势被打断)
            // 都必须 cancel 它,防"幽灵气泡"。
            val holdJob = scope.launch {
                delay(PreviewHoldMs)
                g = HistoryQuickNavLogic.held(g)
                drag.apply(g)
            }
            try {
                while (true) {
                    val event = awaitPointerEvent()
                    val change = event.changes.firstOrNull { it.id == down.id }
                    if (change == null || !change.pressed) {
                        // 松手:按状态机裁定(轻点=开浮层 / 拖了=跳转 / 没拖=取消)
                        holdJob.cancel()
                        when (val r = HistoryQuickNavLogic.released(
                            g, count, drag.heightPx, tickHPx, gapPx, cancelSlopPx,
                        )) {
                            HistoryQuickNavLogic.NavRelease.OpenPanel -> openPanel()
                            is HistoryQuickNavLogic.NavRelease.Jump -> {
                                PaHaptics.confirm(hapticView)
                                jump(r.entryIndex)
                            }
                            else -> {}
                        }
                        drag.apply(HistoryQuickNavLogic.canceled())
                        settled = true
                        break
                    }
                    if (change.position.y != g.y) {
                        g = HistoryQuickNavLogic.moved(g, change.position.y, viewConfiguration.touchSlop)
                        drag.apply(g)
                    }
                    change.consume()
                }
            } finally {
                holdJob.cancel()
                // 手势被外层打断(pointer cancel 等):只收场不裁定 —— 等价"取消"。
                if (!settled) drag.apply(HistoryQuickNavLogic.canceled())
            }
        }
    }
}

/**
 * 左缘垂直刻度条。满高透明触控条 + Canvas 画刻度点(不做逐条 composable,
 * 消息很多时也只有一次绘制,规格红线)。
 *
 * 交互:轻点 → [onOpenPanel] 开完整列表;按住 → 预览气泡浮出、拖动跟手、
 * 松手 → [onJump](手指停住处的刻度下标);几乎没拖 = 取消(组件内部收场,
 * 不打扰宿主)。
 *
 * @param count 刻度数(=用户消息数);0 时不渲染(没有可导航的内容)。
 * @param currentIndex 当前可见位置(视口顶部最近滚过的用户消息,-1=无)。
 * @param drag 宿主持有的手势状态(与 [HistoryQuickBubble] 共享同一份)。
 */
@Composable
fun HistoryTickBar(
    count: Int,
    currentIndex: Int,
    onOpenPanel: () -> Unit,
    onJump: (Int) -> Unit,
    modifier: Modifier = Modifier,
    drag: HistoryDragState = remember { HistoryDragState() },
    /** 【R1 实时刷选】预览中跨到新刻度(含回扫);宿主把列表立刻滚到该条。 */
    onScrub: (Int) -> Unit = {},
    /** 【R1 实时刷选】刷选收场(松手/取消/手势打断,指数回 -1):宿主复位程序滚动标记。 */
    onScrubEnd: () -> Unit = {},
    /** 【R2 日期上下文】日界标记(每天第一条=true):长会话里日期分段一眼可辨。 */
    dayStarts: List<Boolean> = emptyList(),
) {
    if (count <= 0) return
    val hacker = PaHacker.isHacker
    val idleColor = if (hacker) {
        PaEditorial.OnGraphiteSubtle.copy(alpha = 0.55f)
    } else {
        PaEditorial.Muted.copy(alpha = 0.45f)
    }
    val activeColor = PaEditorial.Primary
    val tickHPx = with(LocalDensity.current) { TickHeight.toPx() }
    val gapPx = with(LocalDensity.current) { TickGap.toPx() }
    // 拖动预览中的高亮刻度:只在"指数变了"时触发重绘(derivedStateOf 去抖,
    // Canvas 里读 → 重绘级失效,零重组)
    val scrubIndex = remember(count, tickHPx, gapPx) {
        derivedStateOf {
            if (drag.phase == HistoryQuickNavLogic.NavGesturePhase.PREVIEW) {
                HistoryQuickNavLogic.indexAtY(drag.fingerY, count, drag.heightPx, tickHPx, gapPx)
            } else -1
        }
    }
    // 预览中跨过刻度给一次轻触感(时间刷选的标准体感,对齐系统时钟滚轮/相册裁剪);
    // 只在指数变化时重启一次 effect,手指在两条刻度之间滑动不会连发。
    // 【触感分级】轻档统一走 PaHaptics.light(CLOCK_TICK):与 pressable(haptic)
    // 同强度,且同样跟随系统「触感反馈」总开关 —— 关了全局安静,不各自为政。
    val hapticView = LocalView.current
    // 【实时刷选(Telegram date-bar 模式,2026-10-01 三轮深优 R1)】预览中跨过
    // 刻度 → 宿主**立刻滚动列表**到那条 —— "预览→松手才跳"两段合一为"拖到哪
    // 看到哪",与全局刷选手感对齐(宿主经 requestScrollToItem 逐帧安全滚动)。
    // 【首指只记档】预览刚浮现时 scrubIndex=离按下点最近的刻度,这一跳不是用户
    // 拖出来的 —— 直接滚会变成"按住 120ms 列表自己跳一格"的失控感;首个指数
    // 只记档,之后每个**变化**才刷选(快扫时 slop 仅 ~21px,几乎无感损耗)。
    // 回调经 rememberUpdatedState 转接,防重组换 lambda 掐断进行中的手势。
    val hapticScrub by rememberUpdatedState(onScrub)
    val hapticScrubEnd by rememberUpdatedState(onScrubEnd)
    var firedScrub by remember { mutableIntStateOf(Int.MIN_VALUE) }
    LaunchedEffect(scrubIndex.value) {
        val v = scrubIndex.value
        if (v < 0) {
            if (firedScrub != Int.MIN_VALUE) hapticScrubEnd()
            firedScrub = Int.MIN_VALUE
            return@LaunchedEffect
        }
        PaHaptics.light(hapticView)
        if (firedScrub == Int.MIN_VALUE) {
            firedScrub = v
        } else if (v != firedScrub) {
            hapticScrub(v)
            firedScrub = v
        }
    }
    Box(
        modifier
            .width(TickBarTouchWidth)
            .fillMaxHeight()
            // 【真机 2026-10-01】向系统申请返回手势豁免(尽力而为:系统每边最多
            // 认 200dp,整条未必全认)——配合 40dp 触控带,把"真手按在视觉刻度上
            // 拖动被 MIUI 返回手势掐死、气泡闪现即消失"的体感压到最低。
            .systemGestureExclusion()
            .onSizeChanged { drag.heightPx = it.height.toFloat() }
            .then(rememberHistoryDragGesture(drag, count, onOpenPanel, onJump))
            .semantics {
                contentDescription =
                    "历史消息时间线,共 $count 条用户消息;轻点打开列表,按住拖动快速浏览"
            },
    ) {
        Canvas(
            // 视觉刻度钉在触控盒最左缘(0-10dp):触控带加宽后不能跟着居中漂移
            Modifier
                .align(Alignment.CenterStart)
                .width(TickBarVisualWidth)
                .fillMaxHeight()
        ) {
            // 几何单源:与手指↔下标映射共用 HistoryQuickNavLogic.tickGeometry,
            // 画出来在哪、映射就算到哪,永不各说各话。
            val geo = HistoryQuickNavLogic.tickGeometry(count, size.height, tickHPx, gapPx)
            // 预览中指到的刻度临时接管高亮(视频样板:悬停刻度与其余拉开);
            // 读 scrubIndex 是绘制期状态读 → 只重绘不重组
            val highlight = scrubIndex.value.takeIf { it >= 0 } ?: currentIndex
            for (i in 0 until count) {
                val active = i == highlight
                // 日界刻度画全长:非激活态也比普通刻度长,日期分段一眼可辨(R2)
                val w = when {
                    active -> size.width
                    dayStarts.getOrNull(i) == true -> size.width
                    else -> size.width * 0.7f
                }
                drawRoundRect(
                    color = if (active) activeColor else idleColor,
                    topLeft = Offset(0f, geo.top + i * geo.step),
                    size = Size(w, tickHPx),
                    cornerRadius = CornerRadius(tickHPx / 2f),
                )
            }
        }
    }
}

/**
 * 按住拖动的紧凑预览气泡(视频样板的 hover 摘要卡的触屏翻译)。
 *
 * 只在 drag.phase == PREVIEW 时存在(scale+fade 出入场,PaMotion spring);
 * 位置进度驱动跟手(offset 延迟读手指 Y + 8dp 让位钳制),内容走 derivedStateOf
 * ——只在切到另一条消息时重组。平时零遮挡:不预览时根本不在组合树里。
 *
 * 必须挂在刻度条同一个 Box 里(align TopStart),Y 才是同一坐标系。
 */
@Composable
fun HistoryQuickBubble(
    drag: HistoryDragState,
    entries: List<HistoryQuickNavLogic.Entry>,
    modifier: Modifier = Modifier,
) {
    if (entries.isEmpty()) return
    val hacker = PaHacker.isHacker
    val density = LocalDensity.current
    // 【减弱动效】系统动画缩放=0 时出入场退化瞬时(snap 同义 PaMotion.instant,
    // 其静态类型 AnimationSpec 传不进 fadeIn,同 AppDrawer 的既有处理)。
    // 跟手不受影响:气泡位置走 offset{} placement 层,与转场规格无关。
    val reduceMotion = dev.pocketagent.app.ui.anim.rememberReduceMotion()
    val bubbleW = LocalConfiguration.current.screenWidthDp.dp * BubbleWidthFraction
    val bubbleHPx = with(density) { BubbleHeight.toPx() }
    val yieldPx = with(density) { BubbleYield.toPx() }
    val startGapPx = with(density) { BubbleStartGap.toPx() }
    val tickHPx = with(density) { TickHeight.toPx() }
    val gapPx = with(density) { TickGap.toPx() }
    // 预览指向的刻度下标:只在跨过相邻刻度时变一次(内容切档),
    // 手指逐帧移动只重算这一个 Int,不重组任何布局。
    val scrubIndex = remember(entries.size, tickHPx, gapPx) {
        derivedStateOf {
            if (drag.phase == HistoryQuickNavLogic.NavGesturePhase.PREVIEW) {
                HistoryQuickNavLogic.indexAtY(drag.fingerY, entries.size, drag.heightPx, tickHPx, gapPx)
            } else -1
        }
    }
    // 退场定格:松手后 phase 立即回 IDLE(指数变 -1),退场动画还在播 ——
    // 内容维持在最后指到的那条,不能闪回第 0 条。
    val lastScrub = remember { mutableStateOf(0) }
    LaunchedEffect(scrubIndex.value) {
        if (scrubIndex.value >= 0) lastScrub.value = scrubIndex.value
    }
    Box(modifier) {
        AnimatedVisibility(
            visible = drag.phase == HistoryQuickNavLogic.NavGesturePhase.PREVIEW,
            // 入场从刻度条方向"长出来"(轻微过冲 scale + fade);退场干脆
            // (nav 大 stiffness 让位)。全是 spring,拖动中被打断不跳帧。
            enter = if (reduceMotion) {
                fadeIn(androidx.compose.animation.core.snap())
            } else {
                fadeIn(PaMotion.standard()) +
                    scaleIn(PaMotion.emphasized(), initialScale = 0.92f)
            },
            exit = if (reduceMotion) {
                fadeOut(androidx.compose.animation.core.snap())
            } else {
                fadeOut(PaMotion.nav(600f))
            },
            modifier = Modifier.offset {
                // 【真机 2026-10-01 · 跟手回归 placement 层】上一轮为省一档排版
                // 换成 graphicsLayer{translationY} 挂在 AnimatedVisibility 上,
                // 实机结果气泡定格在入场位完全不跟随 —— AV 的进出场图层接管了
                // 该节点的绘制管线,外层 layer 的状态失效被吞。跟手必须走
                // offset{}(placement):每帧只重排版这个 55%×120dp 小节点,
                // 零重组、零列表重排,1:1 无延迟。
                IntOffset(
                    startGapPx.roundToInt(),
                    HistoryQuickNavLogic.bubbleTopY(
                        drag.fingerY, bubbleHPx, drag.heightPx, yieldPx,
                    ).roundToInt(),
                )
            },
        ) {
            val index = (scrubIndex.value.takeIf { it >= 0 } ?: lastScrub.value)
                .coerceIn(0, entries.size - 1)
            val entry = entries[index]
            // 【v3 重做】缩略图只在"指到带图消息"时解码一次,换条消息才重算 ——
            // 拖动逐帧不解码(remember 键 = 字节数组引用 + 下标)
            val thumb = remember(entry.thumbBytes, index) {
                entry.thumbBytes?.let {
                    runCatching { android.graphics.BitmapFactory.decodeByteArray(it, 0, it.size) }.getOrNull()
                }
            }
            Column(
                Modifier
                    .width(bubbleW)
                    // 【v3】自适应高度(此前固定 120dp:内容一两行时下半截是空黑框,
                    // 真机观感"做的很差"正是它)—— 上限 200dp 防长文撑爆
                    .heightIn(min = 64.dp, max = 200.dp)
                    .graphicsLayer {
                        // 以左缘中点为轴缩放:气泡从刻度条方向长出来,不是从卡片中心鼓包
                        transformOrigin = TransformOrigin(0f, 0.5f)
                    }
                    .clip(RoundedCornerShape(PaEditorial.Radius.IconButton))
                    // 玻璃语言(与 ContextMenu 同族):实色底 + 顶缘发丝高光 + 投影;
                    // 黑客模式走 GraphiteDrawer 底(契约同浮层面板)。
                    .paGlass(
                        RoundedCornerShape(PaEditorial.Radius.IconButton),
                        tint = if (hacker) PaEditorial.GraphiteDrawer else PaEditorial.Raised,
                        shadowElevation = 16.dp,
                    )
                    .padding(horizontal = 14.dp, vertical = 10.dp),
            ) {
                // 标题(两行):消息首行,粗体
                Text(
                    entry.title,
                    fontSize = PaEditorial.Type.Body.fontSize,
                    lineHeight = 18.sp,
                    fontWeight = FontWeight.Bold,
                    color = PaEditorial.Ink,
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis,
                    fontFamily = if (hacker) PaEditorial.MonoFont else null,
                )
                Spacer(Modifier.height(5.dp))
                // 元数据行:N/M · 日期 · 图片数 —— 单行紧凑,信息密度高
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text(
                        HistoryQuickNavLogic.positionLabel(index, entries.size),
                        fontSize = PaEditorial.Type.Micro.fontSize,
                        color = PaEditorial.Primary,
                        fontWeight = FontWeight.SemiBold,
                        fontFamily = PaEditorial.MonoFont,
                    )
                    val stamp = HistoryQuickNavLogic.historyStampOf(
                        entry.sentAtMs, System.currentTimeMillis(),
                    )
                    if (stamp.isNotEmpty()) {
                        Spacer(Modifier.width(6.dp))
                        Text("·", fontSize = PaEditorial.Type.Micro.fontSize, color = PaEditorial.Subtle)
                        Spacer(Modifier.width(6.dp))
                        Text(
                            stamp,
                            fontSize = PaEditorial.Type.Micro.fontSize,
                            color = PaEditorial.Subtle,
                            fontFamily = PaEditorial.MonoFont,
                        )
                    }
                    if (entry.imageCount > 0) {
                        Spacer(Modifier.width(6.dp))
                        Text("·", fontSize = PaEditorial.Type.Micro.fontSize, color = PaEditorial.Subtle)
                        Spacer(Modifier.width(6.dp))
                        Text(
                            "${entry.imageCount} 图",
                            fontSize = PaEditorial.Type.Micro.fontSize,
                            color = PaEditorial.Subtle,
                            fontFamily = PaEditorial.MonoFont,
                        )
                    }
                }
                // 缩略图行(带图消息):首图 44dp 圆角实图 + 计数
                if (thumb != null) {
                    Spacer(Modifier.height(6.dp))
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Image(
                            bitmap = thumb.asImageBitmap(),
                            contentDescription = "消息图片预览",
                            contentScale = ContentScale.Crop,
                            modifier = Modifier
                                .size(44.dp)
                                .clip(RoundedCornerShape(PaEditorial.Radius.Chip)),
                        )
                        if (entry.imageCount > 1) {
                            Spacer(Modifier.width(6.dp))
                            Text(
                                "+${entry.imageCount - 1}",
                                fontSize = PaEditorial.Type.Caption.fontSize,
                                color = PaEditorial.Subtle,
                                fontFamily = PaEditorial.MonoFont,
                            )
                        }
                    }
                } else if (entry.summary.isNotEmpty()) {
                    Spacer(Modifier.height(4.dp))
                    Text(
                        entry.summary,
                        fontSize = PaEditorial.Type.Caption.fontSize,
                        lineHeight = 17.sp,
                        color = if (hacker) PaEditorial.OnGraphiteMuted else PaEditorial.Muted,
                        maxLines = 2,
                        overflow = TextOverflow.Ellipsis,
                        fontFamily = if (hacker) PaEditorial.MonoFont else null,
                    )
                }
                Spacer(Modifier.height(7.dp))
                // 【v3】刷选进度条:当前条在整场对话中的位置,一跟手就看得见远近
                Box(
                    Modifier
                        .fillMaxWidth()
                        .height(2.dp)
                        .clip(RoundedCornerShape(PaEditorial.Radius.Terminal))
                        .background(if (hacker) PaEditorial.OnGraphiteSubtle else PaEditorial.Hairline),
                ) {
                    Box(
                        Modifier
                            .fillMaxWidth((index + 1f) / entries.size)
                            .fillMaxHeight()
                            .background(PaEditorial.Primary),
                    )
                }
            }
        }
    }
}

/**
 * 左滑浮层:历史消息摘要卡列表(Drawer 式 Panel)。
 *
 * 二版去遮挡:宽度 78% → 62%,scrim alpha 0.25(隐约透出正文 —— 速查不是切页);
 * 滑入滑出动画保持(spring,同族可打断)。
 * 【R3 深优】搜索框(标题/摘要过滤,关闭即清)+「最早/最新」快跳;列表按日分组。
 *
 * @param visible 开关(宿主持有,返回键/Scrim 点击都关它)。
 * @param entries [HistoryQuickNavLogic.entriesOf] 的产物。
 * @param currentIndex 当前可见位置(高亮用,-1=无)。
 * @param onJump(listIndex) 点卡片:宿主把 LazyColumn 滚到该下标并关闭浮层。
 * @param onDismiss 关闭(Scrim 点击)。
 * @param onJumpEarliest 「最早」快跳:滚到第一条用户消息(null = 不摆按钮)。
 * @param onJumpLatest 「最新」快跳:回底并恢复跟随(null = 不摆按钮)。
 * @param searcher 【全文搜索·Task 7 升级】宿主注入的 HistorySearchRepository
 *   (RPC 在仓库内归一为五态);null = 不接全文搜索(搜索框只做本地过滤)。
 *   面板只管防抖与状态呈现,不碰 gRPC;远端失败如实呈现人话,不伪装空结果。
 * @param onFullTextHitClick 点全文命中:宿主按 message_id 主键定位(缺口自动
 *   请求目标历史片段),定位不到诚实降级(Toast),面板不猜坐标系。
 * @param threadTitle 会话标题(命中卡展示,spec:高亮片段+角色+日期+轮次+会话标题)。
 * @param onSearchSearched 搜索回包的 searched_messages 上报(宿主记作"daemon
 *   全量消息总数",sequence 兼容定位的坐标系基准;不关心可传空)。
 */
@Composable
fun HistoryQuickPanel(
    visible: Boolean,
    entries: List<HistoryQuickNavLogic.Entry>,
    currentIndex: Int,
    onJump: (Int) -> Unit,
    onDismiss: () -> Unit,
    onJumpEarliest: (() -> Unit)? = null,
    onJumpLatest: (() -> Unit)? = null,
    searcher: HistorySearchRepository? = null,
    onFullTextHitClick: (HistorySearchHit) -> Unit = {},
    threadTitle: String = "",
    onSearchSearched: (Int) -> Unit = {},
) {
    val hacker = PaHacker.isHacker
    // 搜索是面板的临时视图状态:关闭即清,下次打开不带旧查询(线性返回语义)
    var query by remember { mutableStateOf("") }
    // ---- 【全文搜索·Task 7 五态】命中状态从"列表+在途位"升级为 HistorySearchStatus:
    // 本页/全会话/仅已下载/远端失败/无结果 可区分;失败如实呈现人话,不再静默降级为空。----
    var searchStatus by remember(searcher) {
        mutableStateOf<HistorySearchStatus>(HistorySearchStatus.Idle)
    }
    var searching by remember(searcher) { mutableStateOf(false) }
    // 上次已发起的查询(防抖去重)与在途任务(新查询到达即取消旧 RPC)
    var lastFiredQuery by remember(searcher) { mutableStateOf<String?>(null) }
    var searchJob by remember(searcher) { mutableStateOf<Job?>(null) }
    val searchScope = rememberCoroutineScope()
    LaunchedEffect(searcher) {
        // 搜索器换人(换会话等):旧状态立即作废,不留给新会话串台
        lastFiredQuery = null
        searchStatus = HistorySearchStatus.Idle
        searching = false
    }
    LaunchedEffect(searcher, query) {
        if (searcher == null) return@LaunchedEffect
        val q = HistoryQuickNavLogic.searchableQuery(query)
        if (q == null) {
            // 清空/空白:取消在途、立刻清状态(无需防抖)
            searchJob?.cancel()
            lastFiredQuery = null
            searching = false
            searchStatus = HistorySearchStatus.Idle
            return@LaunchedEffect
        }
        if (HistoryQuickNavLogic.shouldSkipFire(q, lastFiredQuery)) return@LaunchedEffect
        lastFiredQuery = q
        searchJob?.cancel()
        searching = true
        searchJob = searchScope.launch {
            // search 从不把失败伪装成空结果(失败在仓库内归一为 RemoteError):
            // 五态如实到达 UI;命中数上报给宿主(sequence 兼容定位的全量基准)
            val status = searcher.search(q, HistoryQuickNavLogic.SEARCH_LIMIT)
            searchStatus = status
            searching = false
            if (status is HistorySearchStatus.Results) onSearchSearched(status.searchedMessages)
        }
    }
    LaunchedEffect(visible) {
        if (!visible) query = ""
    }
    // 【减弱动效】scrim/面板的出入场在系统动画缩放=0 时退化瞬时(淡入保留"变了"的事实,位移不播)
    val reduceMotion = dev.pocketagent.app.ui.anim.rememberReduceMotion()
    // 返回键关闭(与抽屉/面板同层语义);visible=false 时不武装,不抢别的层。
    BackHandler(enabled = visible) { onDismiss() }
    // 【R3】过滤后的列表:搜索时只显示命中的;分组/计数都基于它
    val filtered = HistoryQuickNavLogic.filterEntries(entries, query)
    AnimatedVisibility(
        visible = visible,
        // scrim 与面板同节奏淡入淡出;alpha 0.25 —— 能隐约看到正文,不是全黑罩
        enter = if (reduceMotion) fadeIn(androidx.compose.animation.core.snap())
        else fadeIn(PaMotion.standard()),
        exit = if (reduceMotion) fadeOut(androidx.compose.animation.core.snap())
        else fadeOut(PaMotion.nav(600f)),
        label = "historyPanelScrim",
    ) {
        Box(
            Modifier
                .fillMaxSize()
                // PaGlassToken.Scrim 是浮层压暗的系统槽位(随主题换色),
                // 这里只要它的"色"不要它的"浓度":浓度由 PanelScrimAlpha 管。
                .background(PaGlassToken.Scrim.copy(alpha = PanelScrimAlpha))
                .clickable(onClick = onDismiss),
        )
    }
    AnimatedVisibility(
        visible = visible,
        enter = if (reduceMotion) slideInHorizontally(androidx.compose.animation.core.snap()) { -it }
        else slideInHorizontally(PaMotion.emphasized()) { -it },
        // 退场与气泡/ContextMenu/UndoBar 同节奏:nav(600f) 大 stiffness,干脆让位
        // 不砖墙(进场从容、退场干脆是全 app 浮层族的统一 spring 语义)。
        exit = if (reduceMotion) slideOutHorizontally(androidx.compose.animation.core.snap()) { -it }
        else slideOutHorizontally(PaMotion.nav(600f)) { -it },
    ) {
        val panelWidth = LocalConfiguration.current.screenWidthDp.dp * PanelWidthFraction
        Column(
            Modifier
                .fillMaxHeight()
                .width(panelWidth)
                .background(if (hacker) PaEditorial.GraphiteDrawer else PaEditorial.Surface)
                .then(if (hacker) Modifier else Modifier.border(1.dp, PaEditorial.Hairline))
                .statusBarsPadding(),
        ) {
            // ---- 头部:标题 + 条数 + 关闭 ----
            Row(
                Modifier
                    .fillMaxWidth()
                    .padding(start = 16.dp, end = 6.dp, top = 10.dp, bottom = 6.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(
                    "历史消息",
                    fontSize = PaEditorial.Type.Heading.fontSize,
                    fontWeight = FontWeight.SemiBold,
                    color = PaEditorial.Ink,
                    fontFamily = if (hacker) PaEditorial.MonoFont else null,
                    modifier = Modifier.weight(1f),
                )
                Text(
                    if (query.isBlank()) "${entries.size} 条" else "${filtered.size}/${entries.size} 条",
                    fontSize = PaEditorial.Type.Caption.fontSize,
                    color = PaEditorial.Subtle,
                    fontFamily = if (hacker) PaEditorial.MonoFont else null,
                )
                Spacer(Modifier.width(4.dp))
                IconActionButton(
                    AppR.drawable.ic_lucide_x, "关闭历史消息面板", onDismiss,
                    bg = Color.Transparent, size = 36.dp, iconSize = 16.dp,
                    tint = PaEditorial.Muted,
                )
            }
            Spacer(Modifier.height(4.dp))
            // ---- 【R3】搜索框:标题/摘要过滤,临时态(关闭即清) ----
            TextField(
                value = query,
                onValueChange = { query = it },
                singleLine = true,
                placeholder = {
                    Text(
                        "搜索这一场对话…",
                        fontSize = PaEditorial.Type.BodySmall.fontSize,
                        color = PaEditorial.Subtle,
                        fontFamily = if (hacker) PaEditorial.MonoFont else null,
                    )
                },
                leadingIcon = {
                    IconActionButton(
                        AppR.drawable.ic_lucide_search, "搜索历史消息", {},
                        bg = Color.Transparent, size = 30.dp, iconSize = 15.dp,
                        tint = PaEditorial.Muted,
                    )
                },
                trailingIcon = {
                    if (query.isNotEmpty()) {
                        IconActionButton(
                            AppR.drawable.ic_lucide_x, "清空搜索", { query = "" },
                            bg = Color.Transparent, size = 30.dp, iconSize = 13.dp,
                            tint = PaEditorial.Muted,
                        )
                    }
                },
                textStyle = androidx.compose.ui.text.TextStyle(
                    fontSize = PaEditorial.Type.BodySmall.fontSize,
                    color = PaEditorial.Ink,
                    fontFamily = if (hacker) PaEditorial.MonoFont else null,
                ),
                colors = TextFieldDefaults.colors(
                    focusedContainerColor = if (hacker) PaEditorial.GraphiteCode else PaEditorial.Canvas,
                    unfocusedContainerColor = if (hacker) PaEditorial.GraphiteCode else PaEditorial.Canvas,
                    focusedIndicatorColor = Color.Transparent,
                    unfocusedIndicatorColor = Color.Transparent,
                    cursorColor = PaEditorial.Primary,
                ),
                shape = RoundedCornerShape(PaEditorial.Radius.Chip),
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 12.dp)
                    .heightIn(min = 40.dp),
            )
            // ---- 【R3】最早/最新快跳 ----
            Row(Modifier.padding(horizontal = 16.dp, vertical = 6.dp)) {
                if (onJumpEarliest != null) {
                    Text(
                        "最早",
                        Modifier
                            .clip(RoundedCornerShape(50))
                            .clickable(onClick = onJumpEarliest)
                            .padding(horizontal = 12.dp, vertical = 6.dp),
                        fontSize = PaEditorial.Type.Caption.fontSize,
                        color = PaEditorial.Primary,
                        fontWeight = FontWeight.SemiBold,
                        fontFamily = if (hacker) PaEditorial.MonoFont else null,
                    )
                }
                if (onJumpLatest != null) {
                    Text(
                        "最新",
                        Modifier
                            .clip(RoundedCornerShape(50))
                            .clickable(onClick = onJumpLatest)
                            .padding(horizontal = 12.dp, vertical = 6.dp),
                        fontSize = PaEditorial.Type.Caption.fontSize,
                        color = PaEditorial.Primary,
                        fontWeight = FontWeight.SemiBold,
                        fontFamily = if (hacker) PaEditorial.MonoFont else null,
                    )
                }
            }
            if (entries.isEmpty()) {
                Text(
                    "还没有可跳转的用户消息",
                    Modifier.padding(horizontal = 16.dp, vertical = 12.dp),
                    fontSize = PaEditorial.Type.BodySmall.fontSize,
                    color = PaEditorial.Subtle,
                    fontFamily = if (hacker) PaEditorial.MonoFont else null,
                )
            } else {
                // 【R2 日期上下文】按自然日分组:组头「今天/昨天/M月d日/早前」,
                // 组内行序号沿用全量序号(group.firstIndex + i)——跳转映射不变
                val groups = remember(filtered) {
                    HistoryQuickNavLogic.groupByDay(filtered, System.currentTimeMillis())
                }
                // 【全文搜索】搜索态分两个区块:「本页命中」(本地过滤,即时)+
                // 「全文命中」(daemon RPC,snippet 作次行);非搜索态保持原貌不加头。
                // (queryFiltering 只管本地过滤区块的显隐;全文在途位是上面的
                // searching 状态,由仓库五态驱动 —— 二者各管一段,不混用。)
                val queryFiltering = query.isNotBlank()
                LazyColumn(
                    Modifier.fillMaxWidth(),
                    contentPadding = androidx.compose.foundation.layout.PaddingValues(bottom = 16.dp),
                ) {
                    if (queryFiltering) {
                        item(key = "sec-local") {
                            HistorySectionLabel("本页命中 · ${filtered.size}")
                        }
                        if (filtered.isEmpty()) {
                            item(key = "local-none") {
                                Text(
                                    "本页没有匹配「${query.trim()}」的消息",
                                    Modifier.padding(horizontal = 16.dp, vertical = 12.dp),
                                    fontSize = PaEditorial.Type.BodySmall.fontSize,
                                    color = PaEditorial.Subtle,
                                    fontFamily = if (hacker) PaEditorial.MonoFont else null,
                                )
                            }
                        }
                    }
                    groups.forEach { group ->
                        item(key = "h-${group.firstIndex}") {
                            Text(
                                group.label,
                                Modifier.padding(start = 16.dp, end = 16.dp, top = 10.dp, bottom = 2.dp),
                                fontSize = PaEditorial.Type.Caption.fontSize,
                                color = PaEditorial.Subtle,
                                fontWeight = FontWeight.SemiBold,
                                fontFamily = PaEditorial.MonoFont,
                            )
                        }
                        itemsIndexed(group.entries, key = { _, e -> e.listIndex }) { i, entry ->
                            // 高亮/序号都以「全量 entries」为准:搜索过滤后组内序≠全量序,
                            // 用 listIndex 反查全量下标(查不到=搜索结果不在当前视口序列,不高亮)
                            val entryIndex = entries.indexOfFirst { it.listIndex == entry.listIndex }
                            val active = entryIndex >= 0 && entryIndex == currentIndex
                            Row(
                                Modifier
                                    .fillMaxWidth()
                                    // 高亮 = 当前可见位置:主色细条 + 主色标题;点卡片跳转
                                    .then(
                                        if (active) Modifier.background(PaEditorial.PrimaryContainer)
                                        else Modifier
                                    )
                                    .clickable { onJump(entry.listIndex) }
                                    .heightIn(min = 56.dp)
                                    .padding(horizontal = 16.dp, vertical = 10.dp),
                                verticalAlignment = Alignment.Top,
                            ) {
                                // 序号:第几条用户消息(1 起),等宽对齐不跳动
                                Text(
                                    "${entryIndex + 1}",
                                    Modifier.width(24.dp),
                                    fontSize = PaEditorial.Type.Caption.fontSize,
                                    color = if (active) PaEditorial.Primary else PaEditorial.Subtle,
                                    fontWeight = FontWeight.SemiBold,
                                    fontFamily = PaEditorial.MonoFont,
                            )
                            Column(Modifier.weight(1f)) {
                                Text(
                                    entry.title,
                                    fontSize = PaEditorial.Type.Body.fontSize,
                                    lineHeight = 19.sp,
                                    fontWeight = FontWeight.Bold,
                                    color = if (active) PaEditorial.Primary else PaEditorial.Ink,
                                    maxLines = 1,
                                    overflow = TextOverflow.Ellipsis,
                                    fontFamily = if (hacker) PaEditorial.MonoFont else null,
                                )
                                if (entry.summary.isNotEmpty()) {
                                    Spacer(Modifier.height(3.dp))
                                    Text(
                                        entry.summary,
                                        fontSize = PaEditorial.Type.Caption.fontSize,
                                        lineHeight = 17.sp,
                                        color = if (hacker) PaEditorial.OnGraphiteMuted else PaEditorial.Muted,
                                        maxLines = 2,
                                        overflow = TextOverflow.Ellipsis,
                                        fontFamily = if (hacker) PaEditorial.MonoFont else null,
                                    )
                                }
                            }
                            if (active) {
                                Spacer(Modifier.width(8.dp))
                                Box(
                                    Modifier
                                        .size(7.dp)
                                        .background(PaEditorial.Primary, CircleShape)
                                        .align(Alignment.CenterVertically),
                                )
                            }
                        }
                    } // itemsIndexed(组内行)
                } // groups.forEach
                    // ---- 区块二:全文命中(Task 7 五态呈现)----
                    // 本页/全会话/仅已下载命中按口径分区标注;远端失败如实呈现人话,
                    // 不伪装空结果(Review Focus 红线);无结果与"搜索中"各占一行。
                    // 本地过滤区块(上)始终可用,不受远端成败影响。
                    if (searching && searchStatus == HistorySearchStatus.Idle) {
                        item(key = "ft-searching") {
                            Text(
                                "全文搜索中…",
                                Modifier.padding(horizontal = 16.dp, vertical = 12.dp),
                                fontSize = PaEditorial.Type.BodySmall.fontSize,
                                color = PaEditorial.Subtle,
                                fontFamily = if (hacker) PaEditorial.MonoFont else null,
                            )
                        }
                    }
                    when (val s = searchStatus) {
                        is HistorySearchStatus.Results -> {
                            item(key = "sec-ft") {
                                HistorySectionLabel(
                                    s.kind.label + " · ${s.hits.size}" +
                                        (if (s.hasMore) " · 还有更多" else "") +
                                        (if (!s.scopeKnown) " · 范围未确认" else ""),
                                )
                            }
                            itemsIndexed(
                                s.hits,
                                key = { _, h ->
                                    "ft-" + h.messageId.ifBlank { "s${h.sequence}" } +
                                        "-${h.snippet.hashCode()}"
                                },
                            ) { _, hit ->
                                HistorySearchHitCard(
                                    hit = hit,
                                    query = s.query,
                                    threadTitle = threadTitle,
                                    hacker = hacker,
                                ) { onFullTextHitClick(hit) }
                            }
                        }
                        is HistorySearchStatus.NoResults -> {
                            item(key = "ft-none") {
                                Text(
                                    when {
                                        s.kind == ResultsKind.DOWNLOADED_ONLY ->
                                            "已下载内容里没有匹配「${s.query}」的消息"
                                        s.scopeKnown -> "全会话里没有匹配「${s.query}」的消息"
                                        else -> "没有匹配「${s.query}」的消息(电脑端未报告搜索范围)"
                                    },
                                    Modifier.padding(horizontal = 16.dp, vertical = 12.dp),
                                    fontSize = PaEditorial.Type.BodySmall.fontSize,
                                    color = PaEditorial.Subtle,
                                    fontFamily = if (hacker) PaEditorial.MonoFont else null,
                                )
                            }
                        }
                        is HistorySearchStatus.RemoteError -> {
                            item(key = "ft-error") {
                                RemoteSearchErrorRow(s.reason, hacker)
                            }
                        }
                        HistorySearchStatus.Idle -> {}
                    }
                } // LazyColumn
            }
        }
    }
}

/** 面板区块头(「本页命中 / 全文命中」),样式与日分组组头同族(等宽小字)。 */
@Composable
private fun HistorySectionLabel(text: String) {
    Text(
        text,
        Modifier.padding(start = 16.dp, end = 16.dp, top = 10.dp, bottom = 2.dp),
        fontSize = PaEditorial.Type.Caption.fontSize,
        color = PaEditorial.Subtle,
        fontWeight = FontWeight.SemiBold,
        fontFamily = PaEditorial.MonoFont,
    )
}

/**
 * 远端搜索失败行(五态之一):如实呈现人话原因,并说明本地过滤不受影响 ——
 * 绝不把失败静默画成"无结果"(Review Focus:失败不伪装空结果)。
 */
@Composable
private fun RemoteSearchErrorRow(reason: String, hacker: Boolean) {
    Column(
        Modifier
            .fillMaxWidth()
            .padding(horizontal = 16.dp, vertical = 10.dp),
    ) {
        Text(
            "全文搜索失败",
            fontSize = PaEditorial.Type.Micro.fontSize,
            color = PaEditorial.Denied,
            fontWeight = FontWeight.SemiBold,
            fontFamily = PaEditorial.MonoFont,
        )
        Spacer(Modifier.height(3.dp))
        Text(
            reason,
            fontSize = PaEditorial.Type.BodySmall.fontSize,
            lineHeight = 17.sp,
            color = if (hacker) PaEditorial.OnGraphiteMuted else PaEditorial.Muted,
        )
        Spacer(Modifier.height(2.dp))
        Text(
            "本页过滤不受影响,可稍后重试。",
            fontSize = PaEditorial.Type.Caption.fontSize,
            color = PaEditorial.Subtle,
            fontFamily = if (hacker) PaEditorial.MonoFont else null,
        )
    }
}

/**
 * 摘录高亮:把 daemon MatchRange(消息拼接空间 rune 半开区间)切进摘录文本
 * (HistorySearchState.snippetHighlightSpans 负责坐标映射,对不齐则不高亮,
 * 绝不画错位)。纯函数,无组合状态。
 */
private fun highlightedSnippet(
    snippet: String,
    ranges: List<HistoryMatchSpan>,
    query: String,
): AnnotatedString {
    val spans = HistorySearchState.snippetHighlightSpans(snippet, ranges, query)
    if (spans.isEmpty()) return AnnotatedString(snippet)
    return buildAnnotatedString {
        var cursor = 0
        for (r in spans.sortedBy { it.first }) {
            if (r.first < cursor) continue // 区间不重叠(daemon 保证),防御性跳过
            append(snippet.substring(cursor, r.first))
            withStyle(
                SpanStyle(
                    background = PaEditorial.PrimaryContainer,
                    color = PaEditorial.Primary,
                    fontWeight = FontWeight.SemiBold,
                ),
            ) {
                append(snippet.substring(r.first, r.last + 1))
            }
            cursor = r.last + 1
        }
        if (cursor < snippet.length) append(snippet.substring(cursor))
    }
}

/**
 * 一条全文命中卡(Task 7 规格:高亮片段 + 角色 + 日期 + 所属轮次 + 会话标题)。
 * 首行 = 角色标签 · 时间戳 · 轮次短标(空则不占位);次行 = 命中摘录(按
 * MatchRange 高亮);末行 = 会话标题(同会话恒定,规格要求进卡)。点击 → 宿主
 * 按 message_id 主键自动定位(缺口自动拉取),定位不到由宿主诚实降级。
 */
@Composable
private fun HistorySearchHitCard(
    hit: HistorySearchHit,
    query: String,
    threadTitle: String,
    hacker: Boolean,
    onClick: () -> Unit,
) {
    Column(
        Modifier
            .fillMaxWidth()
            .clickable(onClick = onClick)
            .heightIn(min = 48.dp)
            .padding(horizontal = 16.dp, vertical = 10.dp),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Text(
                if (hit.role == "user") "用户" else "助手",
                fontSize = PaEditorial.Type.Micro.fontSize,
                color = PaEditorial.Primary,
                fontWeight = FontWeight.SemiBold,
                fontFamily = PaEditorial.MonoFont,
            )
            val stamp = HistoryQuickNavLogic.historyStampOf(
                hit.createdAtMs, System.currentTimeMillis(),
            )
            if (stamp.isNotEmpty()) {
                Spacer(Modifier.width(6.dp))
                Text("·", fontSize = PaEditorial.Type.Micro.fontSize, color = PaEditorial.Subtle)
                Spacer(Modifier.width(6.dp))
                Text(
                    stamp,
                    fontSize = PaEditorial.Type.Micro.fontSize,
                    color = PaEditorial.Subtle,
                    fontFamily = PaEditorial.MonoFont,
                )
            }
            if (hit.turnId.isNotBlank()) {
                Spacer(Modifier.width(6.dp))
                Text("·", fontSize = PaEditorial.Type.Micro.fontSize, color = PaEditorial.Subtle)
                Spacer(Modifier.width(6.dp))
                Text(
                    "轮次 " + hit.turnId.take(8) + if (hit.turnId.length > 8) "…" else "",
                    fontSize = PaEditorial.Type.Micro.fontSize,
                    color = PaEditorial.Subtle,
                    fontFamily = PaEditorial.MonoFont,
                )
            }
        }
        Spacer(Modifier.height(3.dp))
        Text(
            highlightedSnippet(hit.snippet, hit.ranges, query),
            fontSize = PaEditorial.Type.BodySmall.fontSize,
            lineHeight = 17.sp,
            color = PaEditorial.Ink,
            maxLines = 2,
            overflow = TextOverflow.Ellipsis,
            fontFamily = if (hacker) PaEditorial.MonoFont else null,
        )
        if (threadTitle.isNotBlank()) {
            Spacer(Modifier.height(2.dp))
            Text(
                "会话 · $threadTitle",
                fontSize = PaEditorial.Type.Micro.fontSize,
                color = PaEditorial.Subtle,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
                fontFamily = if (hacker) PaEditorial.MonoFont else null,
            )
        }
    }
}
