package dev.pocketagent.app.ui.component

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clipToBounds
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.graphics.painter.Painter
import androidx.compose.ui.layout.layout
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.R
import dev.pocketagent.app.ui.anim.pressable
import dev.pocketagent.app.ui.theme.PaEditorial
import kotlin.math.roundToInt

/**
 * 页面骨架(README「实现落地建议」四类共享组件之四)——
 * 状态栏 → 顶部导航(菜单/返回、标题、一个次操作)→ 主机状态 → 内容 → 底部主操作。
 *
 * 【为什么收口】README §视觉系统:「页面骨架一致」;此前各屏的顶部导航各写各的,
 * 高度/边距/返回语义慢慢分叉。第二波迁移时把页面内容装进 [content]、导航装进
 * 插槽即可,骨架本身不携带任何业务状态(抽屉开合、返回栈顺序仍是各页/DrawerController
 * 的职责 —— 本组件只保证"长得一致")。
 *
 * 【排版值】导航高 68dp、水平 22dp、标题 25/32 w720(premium.css .nav);内容区
 * 水平 23dp(README 外边距);画布按当前色盘 [PaEditorial.Canvas]。
 *
 * 【插槽稳定性契约(性能侦察 2026-10-09 的裁决落点;P2-9 已落地)】各插槽参数在
 * 类型上都是稳定的(Compose 编译器对函数类型一律按 stable 推断),骨架本身可
 * skip;但**实例**不稳定时(调用方每帧新建 lambda,引用不等),骨架的入参每次
 * 都被判"已变化",整块跟着调用方逐帧重组。框架层无法在骨架内部独立收敛 ——
 * `remember(slot) { slot }` 对逐帧新引用是恒等空转(每帧 memo 必失效),
 * `@Stable` 也不能标在函数参数上。收敛点在调用方。【落地实况(2026-10-09 修复轮)】
 * 热路径 ChatScreen 的全部插槽(titleContent/leading/trailing/hostStatus/bottomBar/
 * content 及其内容体 chatMiddle/chatComposerBody/chatHostStatus)已 remember 化:
 * 以各 lambda 捕获的**裸值集**为 key(照 chatMiddle 先例 —— remember 计算体
 * DisallowComposableCalls 只构造 lambda,session/pendingApproval 等状态委托的读取
 * 放执行时,归骨架内执行作用域订阅);本骨架对稳定插槽入参即可 skip。
 * 新调用方(整页迁移波次)照同款纪律接入。
 *
 * 【插槽纪律】[leading] 只放一个主导航钮(菜单或返回),[trailing] 只放**一个**
 * 次操作(README:一个次操作);主机状态槽 [hostStatus] 统一吃 [PaHostChip]。
 * [titleContent] 用于标题需要携带富信息时的替换(如会话页的"标题+Agent·工作区+
 * 状态点"行);给出后覆盖 [title] 文本,但骨架仍保证同一排版与对齐。
 * 导航钮用 [PaNavIconButton](44dp 最小点击区)。
 *
 * 【返回顺序不在骨架里】审批弹窗 → 抽屉 → 二级页 → 筛选状态的回退优先级是
 * 页面行为,由第二波在各自 BackHandler 里保证,骨架不替页面做决定。
 *
 * @param title 页面标题(25/32);[titleContent] 给出时作为读屏备用文本。
 * @param titleContent 富标题插槽(如会话身份行);null = 用 [title] 纯文本。
 * @param leading 导航槽起点(菜单/返回钮),null = 无。
 * @param trailing 导航槽终点(唯一一个次操作),null = 无。
 * @param hostStatus 主机状态槽(建议放 [PaHostChip]);null = 本页不展示。
 * @param bottomBar 底部输入/主操作区(自动避让导航条);null = 内容直落到底。
 * @param content 页面内容(已含 23dp 水平边距;滚动由页面自管)。
 * @param immersiveHeader 沉浸石墨头部(方向 C · N2,2026-10-09):true 时头部区
 *   (状态栏底+导航行+主机状态槽)整体用 [PaEditorial.Graphite] 打底,与内容区
 *   形成"台上/台下"两层世界 —— 暗面=Graphite 家族专属铁律的落点之一。状态栏
 *   底永远石墨(不随收放,常驻一条"台下"边);文字走 OnGraphite 档。默认 false
 *   = 既有画布语言,既有各屏零改动。
 * @param headerExpandFraction 头部收放系数 0..1(滚动方向驱动,调用方用 PaMotion
 *   插值后传入):1=全开,0=全收(导航行+主机状态槽卷收让位正文,状态栏石墨边
 *   常驻)。仅 [immersiveHeader]=true 时生效;传值需在 0..1(内部夹逼)。
 */
@Composable
fun PaScreenScaffold(
    title: String,
    modifier: Modifier = Modifier,
    titleContent: (@Composable RowScope.() -> Unit)? = null,
    leading: (@Composable () -> Unit)? = null,
    trailing: (@Composable () -> Unit)? = null,
    hostStatus: (@Composable () -> Unit)? = null,
    bottomBar: (@Composable () -> Unit)? = null,
    // 新参必须在 [content] 之前:既有各屏全部以尾随 lambda 传 content,
    // 尾随 lambda 永远映射最后一个参数,追加在后面会全线破调用点。
    immersiveHeader: Boolean = false,
    headerExpandFraction: Float = 1f,
    content: @Composable ColumnScope.() -> Unit,
) {
    Column(
        modifier
            .fillMaxSize()
            .background(PaEditorial.Canvas),
    ) {
        // 顶部导航:68dp 高,左导航钮 + 标题 + 唯一次操作。
        // 【沉浸头部】immersiveHeader=true:石墨打底在 statusBarsPadding 外层
        // (状态栏区一并石墨);常规路径与此改版前逐像素等价 —— 状态栏 inset 由
        // 本头部块消费(等价于旧的根级 statusBarsPadding,布局顺序不变)。
        val headerHost: Modifier = if (immersiveHeader) {
            Modifier
                .background(PaEditorial.Graphite)
                .statusBarsPadding()
        } else {
            Modifier.statusBarsPadding()
        }
        val headerCollapseModifier: Modifier = if (immersiveHeader) {
            Modifier.headerCollapse(headerExpandFraction)
        } else {
            Modifier
        }
        Column(
            Modifier
                .fillMaxWidth()
                .then(headerHost),
        ) {
            // 收放壳:仅沉浸头部生效(常规头部直通,零开销);内容按全高测量、
            // 布局高度取比例、超出裁剪 + 内容淡出 —— 收起读作"卷上去"不是挤压重排。
            Column(headerCollapseModifier) {
                Row(
                    Modifier
                        .heightIn(min = PaEditorial.Space.NavHeight)
                        .padding(horizontal = PaEditorial.Space.NavGutter),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    leading?.let {
                        it()
                        Spacer(Modifier.width(12.dp))
                    }
                    if (titleContent != null) {
                        Row(
                            Modifier
                                .weight(1f)
                                .semantics { this.contentDescription = title },
                            verticalAlignment = Alignment.CenterVertically,
                            content = titleContent,
                        )
                    } else {
                        Text(
                            title,
                            Modifier.weight(1f),
                            style = PaEditorial.Type.Title,
                            color = if (immersiveHeader) PaEditorial.OnGraphite else PaEditorial.Ink,
                            maxLines = 1,
                        )
                    }
                    trailing?.invoke()
                }
                hostStatus?.let {
                    Box(Modifier.padding(horizontal = PaEditorial.Space.PageMargin)) { it() }
                }
            }
        }
        Column(
            Modifier
                .weight(1f)
                .padding(horizontal = PaEditorial.Space.PageMargin),
            content = content,
        )
        bottomBar?.let {
            Box(Modifier.navigationBarsPadding()) { it() }
        }
    }
}

/**
 * 头部收放壳(方向 C · N2 沉浸石墨头部):按 [expand](0..1)收放高度 —— 内容按
 * 全高测量,布局高度取比例值,超出部分裁剪,内容随收起淡出(0.35 保底,不是
 * 完全消失,卷收过程仍读得出"那是头部")。expand=1 时高度零裁剪、alpha 恒 1,
 * 行为直通。纯布局层:不读任何组合状态,供骨架按帧插值调用。
 */
private fun Modifier.headerCollapse(expand: Float): Modifier {
    val fraction = expand.coerceIn(0f, 1f)
    val fade = 0.35f + 0.65f * fraction
    return this
        .clipToBounds()
        .layout { measurable, constraints ->
            val placeable = measurable.measure(constraints)
            val height = (placeable.height * fraction).roundToInt()
            layout(placeable.width, height) {
                if (height > 0) {
                    // 高度裁到比例值,内容原位摆放;淡出走下方内层 graphicsLayer
                    // (placeRelative 的 alpha/layerBlock 位参在本工程 compose-ui
                    // 1.7 已移除,见其候选签名只有 zIndex)。
                    placeable.placeRelative(0, 0)
                }
            }
        }
        // 淡出:graphicsLayer 在 layout 内层,包裹"被摆放的头部内容"的绘制,
        // 纯绘制层 alpha,不触发重新布局。
        .graphicsLayer { alpha = fade }
}

/**
 * 顶部导航圆钮(premium.css .icon-button):44dp 白底方格、13dp 圆角、细线描边;
 * [plain] = 无底无框的裸钮(返回箭头常用);[active] = 当前操作(主色浅底+主色图,
 * 主色只用于当前操作的纪律落点);[onGraphite] = 坐石墨沉浸头部的形态(方向 C ·
 * N2):透明底 + OnGraphite 图 + OnGraphiteHairline 细边 —— 头部收放/换盘时
 * 同一套几何,只换内容色,不另造第二种导航钮。
 *
 * @param contentDescription 读屏名(必填 —— 图标钮没有文字,无障碍名不能省)。
 */
@Composable
fun PaNavIconButton(
    onClick: () -> Unit,
    contentDescription: String,
    icon: Painter,
    modifier: Modifier = Modifier,
    active: Boolean = false,
    plain: Boolean = false,
    onGraphite: Boolean = false,
) {
    val shape = RoundedCornerShape(PaEditorial.Radius.IconButton)
    val container = when {
        active -> PaEditorial.PrimaryContainer
        plain || onGraphite -> Color.Transparent
        else -> PaEditorial.Surface
    }
    val tint = when {
        active -> PaEditorial.Primary
        onGraphite -> PaEditorial.OnGraphite
        else -> PaEditorial.Ink
    }
    val borderColor = if (onGraphite) PaEditorial.OnGraphiteHairline else PaEditorial.Hairline
    Box(
        modifier
            .size(44.dp)
            .background(container, shape)
            .then(
                if (plain) Modifier else Modifier.border(1.dp, borderColor, shape),
            )
            .pressable(onClick = onClick)
            .semantics { this.contentDescription = contentDescription },
        contentAlignment = Alignment.Center,
    ) {
        Icon(icon, null, tint = tint, modifier = Modifier.size(20.dp))
    }
}

/** 便捷重载:直接用项目内置 lucide 资源。 */
@Composable
fun PaNavIconButton(
    onClick: () -> Unit,
    contentDescription: String,
    iconRes: Int,
    modifier: Modifier = Modifier,
    active: Boolean = false,
    plain: Boolean = false,
    onGraphite: Boolean = false,
) {
    PaNavIconButton(onClick, contentDescription, painterResource(iconRes), modifier, active, plain, onGraphite)
}
