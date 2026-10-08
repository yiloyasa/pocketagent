package dev.pocketagent.app.ui.component

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.defaultMinSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.R
import dev.pocketagent.app.ui.EditorialStatus
import dev.pocketagent.app.ui.EditorialStatusView
import dev.pocketagent.app.ui.PaStatusGlyph
import dev.pocketagent.app.ui.statusGlyphOf
import dev.pocketagent.app.ui.theme.PaEditorial

/**
 * 形通道的统一渲染槽(徽章族共用:PaStatusBadge / PaHostChip 都从这里取形状)。
 *
 * 【三通道之一】四词状态除了色档与状态词,再给一个互不相同的形状 ——
 * 空心环(等用户)/ 实心点(进行中)/ 对勾(已完成)/ 叹号(未完成),
 * 红绿色弱也能扫一眼分清。所有形状统一居中在 10dp 槽位里,徽章行高不因形状跳动。
 */
@Composable
internal fun PaStatusGlyphSlot(
    glyph: PaStatusGlyph,
    tint: Color,
    modifier: Modifier = Modifier,
) {
    Box(modifier.size(10.dp), contentAlignment = Alignment.Center) {
        when (glyph) {
            PaStatusGlyph.RING -> Box(
                Modifier
                    .size(8.dp)
                    .border(1.5.dp, tint, CircleShape),
            )
            PaStatusGlyph.DOT -> Box(
                Modifier
                    .size(6.dp)
                    .background(tint, CircleShape),
            )
            PaStatusGlyph.CHECK -> Icon(
                painterResource(R.drawable.ic_lucide_check), null,
                tint = tint, modifier = Modifier.size(10.dp),
            )
            PaStatusGlyph.EXCLAMATION -> Icon(
                painterResource(R.drawable.ic_lucide_triangle_alert), null,
                tint = tint, modifier = Modifier.size(10.dp),
            )
        }
    }
}

/**
 * 状态徽章(README「实现落地建议」四类共享组件之二)—— 四词状态机的唯一皮。
 *
 * 【口径】词只能来自 [EditorialStatusView] 的四词(等待你处理/正在执行/已完成/
 * 未完成),组件不接受裸字符串当状态词,结构上杜绝「运行中/处理中/失败」各页
 * 各说各话回潮。[suffix] 是可选的时间/来源后缀(README:并显示时间或来源),
 * 以「·」拼接,不是状态词的一部分。
 *
 * 【语义三通道】色档浅底([EditorialStatusView.status] 的 tone.container)+
 * 加深文字(tone.text,10sp 对浅底 ≥4.5:1)+ 形通道([statusGlyphOf] 的环/点/
 * 勾/叹号,见 [PaStatusGlyphSlot]),读屏另给「状态:」前缀的 contentDescription
 * —— 词、色、形三线同义,色弱与读屏用户都不依赖颜色。
 *
 * 【族内一致性】药丸圆角 100dp、minHeight 22dp、水平 8dp/垂直 4dp 内边距、
 * 10sp/650 字重(PaEditorial.Type.State)—— 徽章族的统一度量;ThreadStatusBadge
 * (裸文本旧形态)与 DeliveryBadge(行内回执)各按其版面保持自身节奏,迁移时
 * 由本组件接管。
 *
 * 【为何静止不脉冲】编辑式稿的 .state 药丸全部是静态底色块;运行感由页内进度
 * 元素表达,徽章保持安静的层级(与 ThreadStatusBadge 的脉冲徽章职责不同,
 * 后者在本轮保持原行为,第二波迁移时由本组件接管)。
 */
@Composable
fun PaStatusBadge(
    view: EditorialStatusView,
    modifier: Modifier = Modifier,
    suffix: String? = null,
) {
    val tone = view.status.tone
    val text = buildString {
        append("状态：")
        append(view.label)
        if (!suffix.isNullOrBlank()) append(" · ").append(suffix)
    }
    Row(
        modifier
            .defaultMinSize(minHeight = 22.dp)
            .background(tone.container, RoundedCornerShape(PaEditorial.Radius.Pill))
            .padding(horizontal = 8.dp, vertical = 4.dp)
            .semantics { contentDescription = text },
        verticalAlignment = Alignment.CenterVertically,
    ) {
        PaStatusGlyphSlot(statusGlyphOf(view.status), tone.accent)
        Spacer(Modifier.width(5.dp))
        Text(
            view.label,
            fontSize = PaEditorial.Type.Micro.fontSize, lineHeight = 14.sp,
            fontWeight = FontWeight.SemiBold,
            color = tone.text,
            maxLines = 1,
        )
        if (!suffix.isNullOrBlank()) {
            Spacer(Modifier.width(4.dp))
            Text(
                "· $suffix",
                fontSize = PaEditorial.Type.Micro.fontSize, lineHeight = 14.sp,
                fontWeight = FontWeight.SemiBold,
                color = tone.text.copy(alpha = 0.85f),
                maxLines = 1,
            )
        }
    }
}

/** 便捷重载:已有 [EditorialStatus] 且不需要 detail 时直取。 */
@Composable
fun PaStatusBadge(
    status: EditorialStatus,
    modifier: Modifier = Modifier,
    suffix: String? = null,
) {
    PaStatusBadge(EditorialStatusView(status), modifier, suffix)
}
