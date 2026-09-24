package dev.pocketagent.app.ui.component

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.ModalBottomSheet
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
 * @param scrollable 内容是否包滚动。内容本身已是 LazyColumn（自己会滚）时传
 *   false，避免滚动嵌套打架——但高度上限仍然生效。
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun PaSheet(
    onDismissRequest: () -> Unit,
    modifier: Modifier = Modifier,
    shape: Shape = RoundedCornerShape(topStart = 28.dp, topEnd = 28.dp),
    scrollable: Boolean = true,
    content: @Composable ColumnScope.() -> Unit,
) {
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    val maxH = LocalConfiguration.current.screenHeightDp.dp * 0.82f
    ModalBottomSheet(
        onDismissRequest = onDismissRequest,
        sheetState = sheetState,
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
