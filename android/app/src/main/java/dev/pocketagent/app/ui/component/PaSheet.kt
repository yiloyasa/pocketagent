package dev.pocketagent.app.ui.component

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.WindowInsetsSides
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.only
import androidx.compose.foundation.layout.safeDrawing
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.ModalBottomSheetProperties
import androidx.compose.material3.SheetValue
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Shape
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.unit.dp
import dev.pocketagent.app.ui.theme.PaGlassToken

/**
 * 全 app 统一的玻璃底部浮层（ModalBottomSheet 的皮肤 + 安全封装）。
 *
 * 【为什么收敛到一个封装】AUDIT 的 sheet 溢出巡检规则要求每个 ModalBottomSheet
 * 的内容块必须自带滚动或高度上限——这条规则源自真机实锤（小屏上长内容把
 * sheet 顶出屏幕）。此前每个调用点各自记得；收敛到封装之后"新 sheet 忘记
 * 加滚动"这类回归在结构上不可能再发生：默认就给 82% 屏高上限 + 竖向滚动。
 *
 * 【材质】玻璃底色（[PaGlassToken.Tint]）+ 大圆角；被覆盖内容的推背模糊由
 * 调用方在内容根上挂 [dev.pocketagent.app.ui.theme.paPushBack] 负责（见
 * PaGlass.kt 的职责分离说明——模糊在内容层，不在浮层）。
 *
 * 【参数化收敛 · 为什么签名里只有稳定类型】本封装要接住 ChatScreen 两处遗留
 * 裸 ModalBottomSheet 的特殊语义（审批卡"不可下滑关闭+拦截返回键"、大窗编辑
 * "safeDrawing insets"），但 material3 1.3.0 给 [SheetState]、
 * [ModalBottomSheetProperties]、BottomSheetDefaults **整个类**都标了
 * @ExperimentalMaterial3Api——实验类型一旦出现在公开签名、或实验调用被写进
 * @Composable 默认参数（Compose 会把默认表达式下沉到调用点求值），opt-in 要求
 * 就传播到**每一个调用点**。编译实证：CollabScreen / HomeScreen /
 * RuntimeSection / SettingsScreen / SubAgentCard / FolderPicker /
 * ContextUsageCard 九处 PaSheet 调用全部报 "material API is experimental"。
 * 收敛的前提是不破坏任何现有调用点，所以实验对象一律在封装内部构造
 * （opt-in 留在这里），对外用两个稳定布尔开关承载等义能力：
 *  - [swipeDismissEnabled] = false：等价 sheetState 的
 *    confirmValueChange = { it != SheetValue.Hidden }（审批卡"不可下滑关闭"）；
 *  - [interceptBackPress] = true：等价 properties 的
 *    shouldDismissOnBackPress = false（审批卡"拦截返回键"）。
 * 两者默认都开/关与收敛前逐位一致，其余定制（拖柄、形状之外的 sheet 行为）
 * 真有第三种需求时再扩稳定参数，不为此放开实验类型进签名。
 *
 * @param scrollable 内容是否包滚动。内容本身已是 LazyColumn（自己会滚）时传
 *   false，避免滚动嵌套打架——但高度上限仍然生效。
 * @param swipeDismissEnabled false = 下滑手势再也收不起浮层（内部代为构造
 *   confirmValueChange 拒绝 Hidden 的 sheetState）。审批卡传 false —— 这是
 *   它的红线语义，由调用点显式声明，封装不替任何调用点做决定。
 * @param interceptBackPress true = 返回键不再关闭浮层（内部代为构造
 *   shouldDismissOnBackPress = false 的 properties）。审批卡传 true。
 * @param contentWindowInsets 覆写内容 insets。默认与 material3 自己的默认值
 *   逐位一致（safeDrawing 只取底部——经反编译 BottomSheetDefaults.windowInsets
 *   字节码核实），即收敛前 PaSheet 不传这个参数时实际生效的值，零行为变化。
 *   大窗编辑等依赖 safeDrawing（顶部让开状态栏）历史修复的调用点从这里显式传入。
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun PaSheet(
    onDismissRequest: () -> Unit,
    modifier: Modifier = Modifier,
    shape: Shape = RoundedCornerShape(topStart = 28.dp, topEnd = 28.dp),
    scrollable: Boolean = true,
    swipeDismissEnabled: Boolean = true,
    interceptBackPress: Boolean = false,
    contentWindowInsets: @Composable () -> WindowInsets = {
        WindowInsets.safeDrawing.only(WindowInsetsSides.Bottom)
    },
    content: @Composable ColumnScope.() -> Unit,
) {
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
        tonalElevation = 0.dp, // 玻璃的层级感靠透与亮边，不靠 M3 的色调抬升
        modifier = modifier,
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
