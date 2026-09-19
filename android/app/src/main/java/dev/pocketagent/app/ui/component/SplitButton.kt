package dev.pocketagent.app.ui.component

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.tween
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.rotate
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Popup
import androidx.compose.ui.window.PopupProperties
import dev.pocketagent.app.R
import dev.pocketagent.app.ui.anim.pressable
import dev.pocketagent.app.ui.theme.Pa

/**
 * Split button 的一个下拉选项。
 */
class SplitOption(
    val title: String,
    val desc: String = "",
    val selected: Boolean = false,
    val onClick: () -> Unit,
)

/**
 * Split button(拆分按钮):左侧主按钮执行默认操作,右侧下拉区展开其他可选操作。
 *
 * 【风格】与全 app 一致的 Pa 令牌:Card 底、14dp 圆角、Separator 分隔线、
 * pressable 按压反馈;下拉面板 CardHigh 底 + 选中项 Accent 打勾。
 *
 * 典型用法:默认模型(主=恢复"跟随桌面配置",下拉=挑模型)、
 * 显示模式(主=循环切换,下拉=三选一)、连接(主=保存并测试,下拉=仅保存/重扫/重置)。
 */
@Composable
fun SplitButton(
    label: String,
    onMainClick: () -> Unit,
    options: List<SplitOption>,
    modifier: Modifier = Modifier,
    caption: String? = null,
    mainLabel: String? = null,
    /** 紧凑模式:高度 34dp、宽度自适应 —— 用于弹窗右上角等"按钮不该占一行"的场景。 */
    compact: Boolean = false,
) {
    var open by remember { mutableStateOf(false) }
    // chevron_right 旋转 90° 即指向下;展开时转到 270° 指向上
    val chevron by animateFloatAsState(if (open) 270f else 90f, tween(180), label = "chevron")
    val density = LocalDensity.current
    val buttonHeight = if (compact) 34.dp else 48.dp
    val corner = if (compact) RoundedCornerShape(10.dp) else RoundedCornerShape(14.dp)
    val menuTopPx = with(density) { (buttonHeight + 6.dp).roundToPx() }

    Box(modifier) {
        Row(
            Modifier
                .then(if (compact) Modifier else Modifier.fillMaxWidth())
                // 【heightIn 而不是 height】主按钮的 caption 要显示"当前生效值"(如模型名),
                // 固定高度会让两行 caption 被裁掉。给最小高度、允许按需长高。
                .heightIn(min = buttonHeight)
                .background(Pa.Card, corner),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            // ---- 主按钮区:执行默认操作 ----
            Row(
                Modifier
                    .then(if (compact) Modifier else Modifier.weight(1f))
                    .pressable(haptic = true) { onMainClick() }
                    .padding(
                        start = if (compact) 11.dp else 14.dp,
                        end = if (compact) 4.dp else 6.dp,
                    ),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Column(if (compact) Modifier else Modifier.weight(1f)) {
                    Text(
                        label,
                        fontSize = if (compact) 12.5.sp else 15.sp,
                        color = Pa.TextPrimary,
                        maxLines = 1,
                    )
                    if (caption != null && !compact) {
                        Spacer(Modifier.height(1.dp))
                        // 当前生效值(如模型名)允许折到第二行,不做单行硬截断。
                        Text(
                            caption,
                            fontSize = 13.5.sp,
                            color = Pa.TextSecondary,
                            maxLines = 2,
                            overflow = TextOverflow.Ellipsis,
                        )
                    }
                }
                if (mainLabel != null) {
                    Spacer(Modifier.width(if (compact) 6.dp else 8.dp))
                    Text(
                        mainLabel,
                        fontSize = if (compact) 10.5.sp else 13.sp,
                        color = Pa.Accent,
                        fontWeight = FontWeight.Medium,
                    )
                }
            }
            Box(
                Modifier
                    .width(1.dp)
                    .height(if (compact) 16.dp else 24.dp)
                    .background(Pa.Separator),
            )
            // ---- 下拉触发区 ----
            Box(
                Modifier
                    .size(width = if (compact) 30.dp else 44.dp, height = buttonHeight)
                    .pressable(haptic = true) { open = !open },
                contentAlignment = Alignment.Center,
            ) {
                Icon(
                    painterResource(R.drawable.ic_lucide_chevron_right),
                    contentDescription = "更多选项",
                    tint = Pa.TextSecondary,
                    // 入口箭头放大一档:13/16dp 在深色底上偏小(用户反馈"图标过小")。
                    modifier = Modifier.size(if (compact) 15.dp else 18.dp).rotate(chevron),
                )
            }
        }

        if (open) {
            Popup(
                // 贴在"下拉箭头"正下方、右对齐:是按钮的附属菜单,不再整宽铺满下方
                alignment = Alignment.TopEnd,
                offset = androidx.compose.ui.unit.IntOffset(0, menuTopPx),
                onDismissRequest = { open = false },
                properties = PopupProperties(focusable = true),
            ) {
                Column(
                    Modifier
                        .widthIn(min = if (compact) 140.dp else 176.dp, max = 300.dp)
                        .heightIn(max = 288.dp)
                        .verticalScroll(rememberScrollState())
                        .background(Pa.CardHigh, RoundedCornerShape(14.dp))
                        .border(1.dp, Pa.Separator, RoundedCornerShape(14.dp))
                        .padding(vertical = 5.dp),
                ) {
                    options.forEachIndexed { i, opt ->
                        if (i > 0) {
                            Box(
                                Modifier
                                    .fillMaxWidth()
                                    .padding(horizontal = 10.dp)
                                    .height(1.dp)
                                    .background(Pa.Separator),
                            )
                        }
                        Row(
                            Modifier
                                .fillMaxWidth()
                                .pressable {
                                    open = false
                                    opt.onClick()
                                }
                                .padding(horizontal = 12.dp, vertical = 9.dp),
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            Column(Modifier.weight(1f)) {
                                // 选项标题(模型名/档位名)允许折到第二行,不单行硬截断。
                                Text(
                                    opt.title,
                                    fontSize = 14.5.sp,
                                    color = if (opt.selected) Pa.Accent else Pa.TextPrimary,
                                    fontWeight = if (opt.selected) FontWeight.Medium else FontWeight.Normal,
                                    maxLines = 2,
                                    overflow = TextOverflow.Ellipsis,
                                )
                                if (opt.desc.isNotEmpty()) {
                                    Spacer(Modifier.height(1.dp))
                                    Text(
                                        opt.desc,
                                        fontSize = 12.5.sp,
                                        color = Pa.TextTertiary,
                                        maxLines = 2,
                                        overflow = TextOverflow.Ellipsis,
                                    )
                                }
                            }
                            if (opt.selected) {
                                Spacer(Modifier.width(8.dp))
                                Icon(
                                    painterResource(R.drawable.ic_lucide_check),
                                    contentDescription = null,
                                    tint = Pa.Accent,
                                    modifier = Modifier.size(18.dp),
                                )
                            }
                        }
                    }
                }
            }
        }
    }
}
