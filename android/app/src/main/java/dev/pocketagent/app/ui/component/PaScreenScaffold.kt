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
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.painter.Painter
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.R
import dev.pocketagent.app.ui.anim.pressable
import dev.pocketagent.app.ui.theme.PaEditorial

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
 * 【插槽稳定性契约(性能侦察 2026-10-09 的裁决落点)】各插槽参数在类型上都是
 * 稳定的(Compose 编译器对函数类型一律按 stable 推断),骨架本身可 skip;但
 * **实例**不稳定时(调用方每帧新建 lambda,引用不等),骨架的入参每次都被判
 * "已变化",整块跟着调用方逐帧重组。框架层无法在骨架内部独立收敛 ——
 * `remember(slot) { slot }` 对逐帧新引用是恒等空转(每帧 memo 必失效),
 * `@Stable` 也不能标在函数参数上。收敛点在调用方:热路径(如 ChatScreen)
 * 迁移时应把插槽 lambda `remember` 化或上提状态,把捕获收敛到稳定引用;骨架
 * 侧唯一能做的是把该契约写明,供迁移波次照做。
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
    content: @Composable ColumnScope.() -> Unit,
) {
    Column(
        modifier
            .fillMaxSize()
            .background(PaEditorial.Canvas)
            .statusBarsPadding(),
    ) {
        // 顶部导航:68dp 高,左导航钮 + 标题 + 唯一次操作
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
                    color = PaEditorial.Ink,
                    maxLines = 1,
                )
            }
            trailing?.invoke()
        }
        hostStatus?.let {
            Box(Modifier.padding(horizontal = PaEditorial.Space.PageMargin)) { it() }
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
 * 顶部导航圆钮(premium.css .icon-button):44dp 白底方格、13dp 圆角、细线描边;
 * [plain] = 无底无框的裸钮(返回箭头常用);[active] = 当前操作(主色浅底+主色图,
 * 主色只用于当前操作的纪律落点)。
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
) {
    val shape = RoundedCornerShape(PaEditorial.Radius.IconButton)
    val container = when {
        active -> PaEditorial.PrimaryContainer
        plain -> Color.Transparent
        else -> PaEditorial.Surface
    }
    val tint = if (active) PaEditorial.Primary else PaEditorial.Ink
    Box(
        modifier
            .size(44.dp)
            .background(container, shape)
            .then(
                if (plain) Modifier else Modifier.border(1.dp, PaEditorial.Hairline, shape),
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
) {
    PaNavIconButton(onClick, contentDescription, painterResource(iconRes), modifier, active, plain)
}
