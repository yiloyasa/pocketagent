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
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.rotate
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Popup
import androidx.compose.ui.window.PopupProperties
import dev.pocketagent.app.R
import dev.pocketagent.app.ui.anim.PaMotion
import dev.pocketagent.app.ui.anim.pressable
import dev.pocketagent.app.ui.theme.Pa
import dev.pocketagent.app.ui.theme.PaEditorial

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
    // chevron_right 旋转 90° 即指向下;展开时转到 270° 指向上。
    // 状态动画走 PaMotion 令牌(spring):中途反向点击可从当前角度续走,不回跳;
    // 减弱动效时瞬时到位(系统动画缩放=0 退 snap)。
    val reduceMotion = dev.pocketagent.app.ui.anim.rememberReduceMotion()
    val chevron by animateFloatAsState(
        if (open) 270f else 90f,
        if (reduceMotion) PaMotion.instant() else PaMotion.standard(),
        label = "chevron",
    )
    val density = LocalDensity.current
    val buttonHeight = if (compact) 34.dp else 48.dp
    val corner = if (compact) RoundedCornerShape(PaEditorial.Radius.Chip) else RoundedCornerShape(PaEditorial.Radius.IconButton)
    val menuTopPx = with(density) { (buttonHeight + 6.dp).roundToPx() }

    Box(modifier) {
        Row(
            Modifier
                .then(if (compact) Modifier else Modifier.fillMaxWidth())
                // 【heightIn 而不是 height】主按钮的 caption 要显示"当前生效值"(如模型名),
                // 固定高度会让两行 caption 被裁掉。给最小高度、允许按需长高。
                .heightIn(min = buttonHeight)
                .background(PaEditorial.Surface, corner)
                .border(1.dp, PaEditorial.Hairline, corner),
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
                        // 【尺寸豁免】非 Type 令牌:标签字号与按钮几何联动(compact
                        // 34dp 高→12.5sp;常规 48dp 高→15sp),取值介于 Body(14)与
                        // Heading(16)之间;Type 档日后增设按钮标签档时可收编,
                        // 当前按组件尺寸语义豁免。
                        fontSize = if (compact) 12.5.sp else 15.sp,
                        color = PaEditorial.Ink,
                        maxLines = 1,
                    )
                    if (caption != null && !compact) {
                        Spacer(Modifier.height(1.dp))
                        // 当前生效值(如模型名)允许折到第二行,不做单行硬截断。
                        Text(
                            caption,
                            fontSize = PaEditorial.Type.Body.fontSize,
                            color = PaEditorial.Muted,
                            maxLines = 2,
                            overflow = TextOverflow.Ellipsis,
                        )
                    }
                }
                if (mainLabel != null) {
                    Spacer(Modifier.width(if (compact) 6.dp else 8.dp))
                    Text(
                        mainLabel,
                        // 【尺寸豁免】仅 compact 档非 Type 令牌:10.5sp 与 34dp 紧凑按钮
                        // 几何联动,低于 Micro(11)档(当前生效值短词,紧凑弹窗场景);
                        // 常规档已收编 BodySmall(13/21)。(常规 15sp 标签档的豁免注释
                        // 见上方主标签 Text —— 同一裁决,几何联动语义一致。)
                        fontSize = if (compact) 10.5.sp else PaEditorial.Type.BodySmall.fontSize,
                        color = PaEditorial.Primary,
                        fontWeight = FontWeight.Medium,
                    )
                }
            }
            Box(
                Modifier
                    .width(1.dp)
                    .height(if (compact) 16.dp else 24.dp)
                    .background(PaEditorial.Hairline),
            )
            // ---- 下拉触发区 ----
            // 【主副动作分区 ≥44dp】主区与下拉区之间给足 44dp 宽的独立触控带,
            // 手指不会一按跨两个动作(紧凑模式原 30dp 偏窄,统一抬到 44)。
            Box(
                Modifier
                    .size(width = 44.dp, height = buttonHeight)
                    .pressable(haptic = true) { open = !open },
                contentAlignment = Alignment.Center,
            ) {
                Icon(
                    painterResource(R.drawable.ic_lucide_chevron_right),
                    contentDescription = "更多选项",
                    tint = PaEditorial.Muted,
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
                        .background(PaEditorial.Surface, RoundedCornerShape(PaEditorial.Radius.IconButton))
                        .border(1.dp, PaEditorial.Hairline, RoundedCornerShape(PaEditorial.Radius.IconButton))
                        .padding(vertical = 5.dp),
                ) {
                    options.forEachIndexed { i, opt ->
                        if (i > 0) {
                            Box(
                                Modifier
                                    .fillMaxWidth()
                                    .padding(horizontal = 10.dp)
                                    .height(1.dp)
                                    .background(PaEditorial.Hairline),
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
                                    fontSize = PaEditorial.Type.Body.fontSize,
                                    color = if (opt.selected) PaEditorial.Primary else PaEditorial.Ink,
                                    fontWeight = if (opt.selected) FontWeight.Medium else FontWeight.Normal,
                                    maxLines = 2,
                                    overflow = TextOverflow.Ellipsis,
                                )
                                if (opt.desc.isNotEmpty()) {
                                    Spacer(Modifier.height(1.dp))
                                    Text(
                                        opt.desc,
                                        fontSize = PaEditorial.Type.BodySmall.fontSize,
                                        color = PaEditorial.Muted,
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
                                    tint = PaEditorial.Primary,
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
