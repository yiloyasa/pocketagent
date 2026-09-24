package dev.pocketagent.app.ui.component

import androidx.compose.animation.core.animateIntAsState
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.ime
import androidx.compose.foundation.layout.navigationBars
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalDensity
import dev.pocketagent.app.ui.anim.PaMotion
import dev.pocketagent.app.ui.anim.rememberReduceMotion

/**
 * 【全局统一的"键盘/导航栏底部安全高度"】
 *
 * 需要给底部输入区留出安全高度的页面,一律用这个,不要各写各的 ——
 * 之前会话页只写 `imePadding()`、别的页什么都不写,才出现"键盘弹起压住输入框"
 * 和"键盘收起压在手势条上"两种症状交替出现。
 *
 * 语义是**取两者较大的那一个,而不是相加**:
 * - 键盘弹起:底部要留的是键盘高度(导航栏被键盘盖住了,不需要再叠一份);
 * - 键盘收起:底部要留的是导航栏/手势条高度。
 * 直接 `imePadding().navigationBarsPadding()` 会把两者加起来,在键盘弹起时
 * 多出一条导航栏高度的空白 —— 那正是"输入框离键盘有一道缝"的来源。
 *
 * 【为什么手算而不是用 WindowInsets.union】本项目的 Compose 版本里
 * `WindowInsets.union` 不存在(已实测编译不过),官方也没有直接等价物,
 * 所以用 `getBottom(density)` 取像素值后手动取 max。
 *
 * 【本轮改版:spring 跟随,不再裸贴 insets】个别 ROM 的 IME insets 是阶跃上报的
 * (0 → 满高一步到位),直接消费就有"输入条瞬移"的跳变。这里对取 max 后的像素值
 * 过一层 **PaMotion.press 档的临界阻尼 spring**:stiffness 520 让补偿值贴着键盘
 * 帧走(不慢半拍地"飘"),任何阶跃都被拉成连续曲线。减弱动效时 snap 直达
 * (反馈仍在、位移不播)。
 */
@Composable
fun Modifier.keyboardAwareBottomPadding(): Modifier {
    val density = LocalDensity.current
    val imeBottom = WindowInsets.ime.getBottom(density)
    val navBottom = WindowInsets.navigationBars.getBottom(density)
    val reduceMotion = rememberReduceMotion()
    val bottomPx by animateIntAsState(
        targetValue = maxOf(imeBottom, navBottom),
        animationSpec = if (reduceMotion) PaMotion.instant() else PaMotion.press(),
        label = "keyboardAwareBottomPadding",
    )
    return windowInsetsPadding(WindowInsets(0, 0, 0, bottomPx))
}
