package dev.pocketagent.app.ui.component

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.ui.theme.Pa

/**
 * 数字角标(设计规范 §2.5)。
 *
 * 【位置】挂在 [content] 的右上角外沿,偏移 `x +5dp / y -5dp`,像系统角标一样
 * "咬"在图标角上而不是盖住图标本体。
 *
 * 【样式】红底白字的胶囊:最小 15dp 高、9sp 半粗字、1.5dp 与所在表面同色的描边 ——
 * 无论垫在浅色还是深色上,角标都有一圈清晰的"切口"轮廓(修复:此前 12dp/8sp
 * 在 17dp 的小图标上糊成一团,看不出数字)。
 *
 * 【为什么不用弹窗】设计要求「不使用弹窗,直接进入页面即可看到」——
 * 角标是最小侵入的告知方式,不打断用户当前操作。
 *
 * 【边界】[count] > 99 显示 `99+`;[count] <= 0 时**完全不渲染**(不显示灰色 0)。
 *
 * 【无障碍】以 `contentDescription` 提供语义,读屏用户能听到未读数量。
 */
@Composable
fun PaBadge(
    count: Int,
    modifier: Modifier = Modifier,
    content: @Composable () -> Unit,
) {
    Box(modifier) {
        content()
        if (count > 0) {
            val label = if (count > 99) "99+" else count.toString()
            // 描边色与所处表面一致,让角标"咬合"进图标而不是浮在空中
            val outline = if (Pa.IsDark) Pa.CardHigh else Pa.Card
            Box(
                Modifier
                    .align(Alignment.TopEnd)
                    .offset(x = 5.dp, y = (-5).dp)
                    .heightIn(min = 15.dp)
                    .widthIn(min = 15.dp)
                    .background(Pa.Red, CircleShape)
                    .border(1.5.dp, outline, CircleShape)
                    .padding(horizontal = 4.dp, vertical = 1.dp)
                    .semantics { contentDescription = "任务中心，$count 条未处理" },
                contentAlignment = Alignment.Center,
            ) {
                Text(
                    label,
                    fontSize = 9.sp,
                    color = Color.White,
                    fontWeight = FontWeight.SemiBold,
                )
            }
        }
    }
}
