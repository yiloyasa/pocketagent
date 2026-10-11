package dev.pocketagent.app.ui.component

import android.graphics.Paint
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.foundation.background
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.WindowInsetsSides
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.only
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawing
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.ModalBottomSheetProperties
import androidx.compose.material3.SheetValue
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.graphics.drawscope.drawIntoCanvas
import androidx.compose.ui.graphics.Outline
import androidx.compose.ui.graphics.Shape
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.graphics.nativeCanvas
import androidx.compose.ui.graphics.toArgb
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties
import dev.pocketagent.app.ui.anim.PaMotion
import dev.pocketagent.app.ui.anim.rememberReduceMotion
import dev.pocketagent.app.ui.theme.PaEditorial
import dev.pocketagent.app.ui.theme.PaGlassToken
import dev.pocketagent.app.ui.theme.paGlass

/**
 * 全 app 统一的玻璃浮层家族(ModalBottomSheet 皮肤 + 居中 dialog 形态,一个封装两种皮)。
 *
 * 【为什么收敛到一个封装】AUDIT 的 sheet 溢出巡检规则要求每个 ModalBottomSheet
 * 的内容块必须自带滚动或高度上限——这条规则源自真机实锤(小屏上长内容把
 * sheet 顶出屏幕)。此前每个调用点各自记得;收敛到封装之后"新 sheet 忘记
 * 加滚动"这类回归在结构上不可能再发生:默认就给 82% 屏高上限 + 竖向滚动。
 *
 * 【centered 变体(2026-10-09 精修波 D2)】居中 dialog 形态(模型卡/审批弹窗等
 * 原裸 Dialog 家族):同一块玻璃材质([PaGlassToken.Tint] + 顶缘高光)、同一档
 * 弹簧出场([PaMotion.sheet],删除裸 Dialog 的平台 fade 观感 —— 内容以 spring
 * 缩放浮现)、同一套高度上限/滚动纪律。"四张皮"(底部 sheet/裸 Dialog/自绘
 * 遮罩卡/M3 默认)从此一张皮。行为语义不变:返回键/点窗外仍走
 * [onDismissRequest](由 interceptBackPress / swipeDismissEnabled 承载,与底部
 * 形态同一对稳定布尔,实验类型仍不进签名)。
 *
 * 【材质】玻璃底色([PaGlassToken.Tint])+ 大圆角;被覆盖内容的推背模糊由
 * 调用方在内容根上挂 [dev.pocketagent.app.ui.theme.paPushBack] 负责(见
 * PaGlass.kt 的职责分离说明——模糊在内容层,不在浮层)。
 *
 * 【参数化收敛 · 为什么签名里只有稳定类型】本封装要接住 ChatScreen 两处遗留
 * 裸 ModalBottomSheet 的特殊语义(审批卡"不可下滑关闭+拦截返回键"、大窗编辑
 * "safeDrawing insets"),但 material3 1.3.0 给 [SheetState]、
 * [ModalBottomSheetProperties]、BottomSheetDefaults **整个类**都标了
 * @ExperimentalMaterial3Api——实验类型一旦出现在公开签名、或实验调用被写进
 * @Composable 默认参数(Compose 会把默认表达式下沉到调用点求值),opt-in 要求
 * 就传播到**每一个调用点**。编译实证:CollabScreen / HomeScreen /
 * RuntimeSection / SettingsScreen / SubAgentCard / FolderPicker /
 * ContextUsageCard 九处 PaSheet 调用全部报 "material API is experimental"。
 * 收敛的前提是不破坏任何现有调用点,所以实验对象一律在封装内部构造
 * (opt-in 留在这里),对外用两个稳定布尔开关承载等义能力:
 *  - [swipeDismissEnabled] = false:底部形态等价 sheetState 的
 *    confirmValueChange = { it != SheetValue.Hidden }(审批卡"不可下滑关闭");
 *    centered 形态等价"点窗外不关闭";
 *  - [interceptBackPress] = true:等价 properties 的
 *    shouldDismissOnBackPress = false(审批卡"拦截返回键")。
 * 两者默认都开/关与收敛前逐位一致,其余定制(拖柄、形状之外的 sheet 行为)
 * 真有第三种需求时再扩稳定参数,不为此放开实验类型进签名。
 *
 * @param scrollable 内容是否包滚动。内容本身已是 LazyColumn(自己会滚)时传
 *   false,避免滚动嵌套打架——但高度上限仍然生效。
 * @param centered true = 居中 dialog 形态(遮罩 + 居中玻璃卡 + spring 出场);
 *   false = 底部 sheet 形态(默认)。
 * @param swipeDismissEnabled false = 下滑手势再也收不起浮层(底部形态,内部代为
 *   构造 confirmValueChange 拒绝 Hidden 的 sheetState)/ 点窗外不关闭(centered
 *   形态)。审批卡传 false —— 这是它的红线语义,由调用点显式声明,封装不替任何
 *   调用点做决定。
 * @param interceptBackPress true = 返回键不再关闭浮层(内部代为构造
 *   shouldDismissOnBackPress = false 的 properties;centered 形态同义)。审批卡传 true。
 * @param contentWindowInsets 覆写内容 insets。默认与 material3 自己的默认值
 *   逐位一致(safeDrawing 只取底部——经反编译 BottomSheetDefaults.windowInsets
 *   字节码核实),即收敛前 PaSheet 不传这个参数时实际生效的值,零行为变化。
 *   大窗编辑等依赖 safeDrawing(顶部让开状态栏)历史修复的调用点从这里显式传入。
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun PaSheet(
    onDismissRequest: () -> Unit,
    modifier: Modifier = Modifier,
    // 顶部圆角对齐令牌(premium.css .modal 23dp);此前硬编码 28dp 是令牌化前的遗留值
    shape: Shape = RoundedCornerShape(topStart = PaEditorial.Radius.Sheet, topEnd = PaEditorial.Radius.Sheet),
    scrollable: Boolean = true,
    swipeDismissEnabled: Boolean = true,
    interceptBackPress: Boolean = false,
    centered: Boolean = false,
    // 满幅沉浸变体(仅 centered 语义下生效):去卡片宽度/高度帽与玻璃皮,内容
    // 铺满 Dialog 窗口(近黑遮罩仍由本封装提供)。为"看图"类全屏预览而设——
    // 同一关闭语义(返回/点窗外/内容自有 clickable)与弹簧出场,只是不装卡。
    fullBleed: Boolean = false,
    contentWindowInsets: @Composable () -> WindowInsets = {
        WindowInsets.safeDrawing.only(WindowInsetsSides.Bottom)
    },
    content: @Composable ColumnScope.() -> Unit,
) {
    if (centered) {
        PaSheetCentered(
            onDismissRequest = onDismissRequest,
            modifier = modifier,
            shape = shape,
            scrollable = scrollable,
            swipeDismissEnabled = swipeDismissEnabled,
            interceptBackPress = interceptBackPress,
            fullBleed = fullBleed,
            content = content,
        )
        return
    }
    // sheetState/properties 都在体内构造(而不是作为参数类型暴露):SheetState 与
    // ModalBottomSheetProperties 在 material3 1.3.0 都是实验类,进公开签名即把
    // opt-in 扩散到全部调用点(见类 KDoc 的编译实证)。默认分支与收敛前逐位一致:
    // skipPartiallyExpanded = true + 返回键可关。
    val effectiveSheetState = if (swipeDismissEnabled) {
        rememberModalBottomSheetState(skipPartiallyExpanded = true)
    } else {
        // 不可下滑关闭:Hidden 被拒绝,下滑手势永远弹回(审批卡红线语义)
        rememberModalBottomSheetState(
            skipPartiallyExpanded = true,
            confirmValueChange = { it != SheetValue.Hidden },
        )
    }
    val effectiveProperties = ModalBottomSheetProperties(shouldDismissOnBackPress = !interceptBackPress)
    val maxH = LocalConfiguration.current.screenHeightDp.dp * 0.82f
    ModalBottomSheet(
        onDismissRequest = onDismissRequest,
        sheetState = effectiveSheetState,
        properties = effectiveProperties,
        contentWindowInsets = contentWindowInsets,
        shape = shape,
        containerColor = PaGlassToken.Tint,
        tonalElevation = 0.dp, // 玻璃的层级感靠透与亮边,不靠 M3 的色调抬升
        // 浅盘海拔语言(D3):浮层 = Alert 档阴影;深色盘(含黑客)保持细线语言不加班驳阴影
        modifier = modifier.then(
            if (PaEditorial.IsDark) Modifier
            else Modifier.paEditorialShadow(PaEditorial.Shadow.Alert, shape),
        ),
    ) {
        Column(
            modifier = Modifier
                .heightIn(max = maxH)
                .then(if (scrollable) Modifier.verticalScroll(rememberScrollState()) else Modifier)
                .navigationBarsPadding(),
            content = content,
        )
    }
}

/**
 * 居中 dialog 形态(D2):全屏遮罩(可点关闭)+ 居中玻璃卡 + spring 出场。
 *
 * 【出场统一 PaMotion.sheet 档】0.96→1.0 缩放 + 淡入;裸 Dialog 的平台窗口
 * fade 无法经公开 API 关闭,但内容以 spring 主动浮现后,主导观感是弹簧而不是
 * fade。减弱动效时瞬时直达(instant,反馈仍在、位移消失)。
 *
 * 【点击语义】点遮罩 = onDismissRequest(与底部形态的下滑关闭同一"用户要求
 * 关闭"通道);点卡片本身吃掉事件不冒泡(swipeDismissEnabled=false 时遮罩
 * 也不关,审批类红线由调用方声明)。
 */
@Composable
private fun PaSheetCentered(
    onDismissRequest: () -> Unit,
    modifier: Modifier,
    shape: Shape,
    scrollable: Boolean,
    swipeDismissEnabled: Boolean,
    interceptBackPress: Boolean,
    fullBleed: Boolean,
    content: @Composable ColumnScope.() -> Unit,
) {
    Dialog(
        onDismissRequest = onDismissRequest,
        properties = DialogProperties(
            usePlatformDefaultWidth = false,
            dismissOnBackPress = !interceptBackPress,
            dismissOnClickOutside = swipeDismissEnabled,
            decorFitsSystemWindows = false,
        ),
    ) {
        val reduceMotion = rememberReduceMotion()
        var entered by remember { mutableStateOf(false) }
        LaunchedEffect(Unit) { entered = true }
        val enter by animateFloatAsState(
            targetValue = if (entered) 1f else 0f,
            animationSpec = if (reduceMotion) PaMotion.instant() else PaMotion.sheet(),
            label = "paSheetCenteredEnter",
        )
        val currentOnDismiss by rememberUpdatedState(onDismissRequest)
        BoxWithConstraints(
            modifier
                .fillMaxSize()
                .background(PaEditorial.Scrim)
                // 点遮罩 = 关闭(与底部形态下滑关闭同语义);关不掉时(swipeDismiss
                // =false)手势也不透传给下层内容
                .then(
                    if (swipeDismissEnabled) {
                        Modifier.pointerInput(Unit) {
                            detectTapGestures { currentOnDismiss() }
                        }
                    } else {
                        Modifier
                    },
                )
                .then(
                    if (fullBleed) Modifier else Modifier
                        .windowInsetsPadding(WindowInsets.safeDrawing)
                        .imePadding(),
                ),
            contentAlignment = Alignment.Center,
        ) {
            val maxH = maxHeight * 0.82f
            // 满幅档:无卡可装——去宽度/高度帽与玻璃/海拔,内容自己铺;【N4】内容根
            // 在封装内强制 fillMaxSize(),不靠调用方自觉(旧实现给裸 Modifier,漏挂
            // fillMaxSize 的内容会塌成包裹尺寸,满幅档名不副实)。
            // 弹簧出场与关闭语义保持同族。
            val cardMods = if (fullBleed) Modifier.fillMaxSize() else Modifier
                .padding(horizontal = 23.dp)
                .widthIn(max = 400.dp)
                .heightIn(max = maxH)
            Column(
                Modifier
                    .graphicsLayer {
                        alpha = enter
                        val scale = 0.96f + 0.04f * enter
                        scaleX = scale
                        scaleY = scale
                    }
                    .then(cardMods)
                    // 浅盘海拔语言(D3):居中弹窗 = Modal 档(三档最高);深色盘不加班驳阴影
                    // (满幅档无卡,不驳阴影不披玻璃)
                    .then(
                        if (fullBleed || PaEditorial.IsDark) Modifier
                        else Modifier.paEditorialShadow(PaEditorial.Shadow.Modal, shape),
                    )
                    // 玻璃材质:透 + 顶缘高光,与底部 sheet 同一块皮
                    .then(if (fullBleed) Modifier else Modifier.paGlass(shape))
                    .then(if (scrollable) Modifier.verticalScroll(rememberScrollState()) else Modifier)
                    // 卡片吃掉点击,不让冒泡到遮罩(点卡片不等于点窗外)
                    .pointerInput(Unit) { detectTapGestures(onTap = {}) },
                content = content,
            )
        }
    }
}

/**
 * 编辑式阴影规格的绘制落点(D3 浅盘海拔语言):把 [PaEditorial.ShadowSpec]
 * (blur/dy/color)画在节点背后。
 *
 * 【为什么要自绘】Compose 的 graphicsLayer.shadowElevation 只吃"海拔高度",
 * 出来的影子颜色/偏移由系统决定,表达不了 premium.css 那种"低饱和蓝黑 + 定
 * 制 dy"的编辑式阴影;用 nativeCanvas 的 Paint.setShadowLayer 才能把规格
 * 逐位画出来(透明填充 + 阴影层 = 只画影子不画面)。
 *
 * 【调用纪律】只在**浅色盘**用(调用方以 PaEditorial.IsDark 门控):深色盘按
 * M3 暗色规范走"色调面替代阴影"的既有细线语言,不加班驳阴影。圆角取 shape
 * 轮廓的左上角近似(底部 sheet 的下缘圆角在屏幕外,不参与视觉)。
 */
internal fun Modifier.paEditorialShadow(spec: PaEditorial.ShadowSpec, shape: Shape): Modifier =
    drawBehind {
        val outline = shape.createOutline(size, layoutDirection, this)
        val radius = when (outline) {
            is Outline.Rounded -> outline.roundRect.topLeftCornerRadius.x
            else -> 0f
        }
        // 直接用框架 Paint(Compose 的 Paint() 在 Android 上本就包着同一个对象):
        // setShadowLayer 画在透明填充上 = 只画影子不画面。
        val paint = Paint()
        paint.color = android.graphics.Color.TRANSPARENT
        paint.setShadowLayer(
            spec.blur.toPx(),
            0f,
            spec.dy.toPx(),
            spec.color.toArgb(),
        )
        drawIntoCanvas { canvas ->
            canvas.nativeCanvas.drawRoundRect(
                0f,
                0f,
                size.width,
                size.height,
                radius,
                radius,
                paint,
            )
        }
    }
