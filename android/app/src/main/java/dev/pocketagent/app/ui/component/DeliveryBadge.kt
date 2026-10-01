package dev.pocketagent.app.ui.component

import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.R as AppR
import dev.pocketagent.app.ui.DELIVERY_OVERFLOW
import dev.pocketagent.app.ui.DELIVERY_PENDING
import dev.pocketagent.app.ui.DELIVERY_QUEUED
import dev.pocketagent.app.ui.DELIVERY_STEERED
import dev.pocketagent.app.ui.deliveryLabel
import dev.pocketagent.app.ui.theme.Pa
import dev.pocketagent.app.ui.theme.PaEditorial

/**
 * P0-2 发送状态徽标 —— WhatsApp 语义的「图标 + 短文案」,常规气泡与 hacker
 * 终端块两条渲染路径共用这一个组件(状态→图标/文案的映射只此一份,防两处分叉)。
 *
 * 【状态分级(对齐 daemon 的 Steered/Queued/Queued(position<=0) 回执帧,TurnDelivery.kt)】
 *   时钟   = 还没到 daemon:pending(发送中)/ queued(排队中 · 第N位,既有文案保留)
 *   单勾   = 已提交 daemon(steered)——文案按审计 P0-2 口径收敛为「已提交」
 *   红!    = daemon 拒收(overflow)——整条消息可点击重发(重试语义在 ChatScreen 挂接)
 *   双勾   = daemon 开始执行:协议当前**没有**这一级的回执帧(QueueFlushed 直接收掉
 *            标签、消息回到普通样式),故本组件只出三级;daemon 若补此帧,
 *            在 [deliveryVisual] 加一个分支即可,UI 无需另改。
 */

/** 投递状态对应的图标语义(纯数据,JVM 单测钉死;drawable 映射只在组合层)。 */
internal enum class DeliveryIconKind { CLOCK, CHECK_SINGLE, EXCLAMATION }

/** 状态 → (图标, 文案, 是否失败级) 的映射结果。label 为空 = 不显示徽标。 */
internal data class DeliveryVisual(
    val icon: DeliveryIconKind,
    val label: String,
    val failure: Boolean,
)

/**
 * 状态 → 图标/文案映射(纯函数,JVM 单测钉死,见 DeliveryBadgeTest)。
 * 文案尽可能复用 [deliveryLabel] 的既有口径(排队位次句式等),只有 steered
 * 按 P0-2 要求换成「已提交」;映射外的状态一律空串(不显示,绝不猜)。
 */
internal fun deliveryVisual(state: String, queuePos: Int): DeliveryVisual = when (state) {
    DELIVERY_PENDING -> DeliveryVisual(DeliveryIconKind.CLOCK, deliveryLabel(state, queuePos), false)
    DELIVERY_QUEUED -> DeliveryVisual(DeliveryIconKind.CLOCK, deliveryLabel(state, queuePos), false)
    DELIVERY_STEERED -> DeliveryVisual(DeliveryIconKind.CHECK_SINGLE, "已提交", false)
    DELIVERY_OVERFLOW -> DeliveryVisual(DeliveryIconKind.EXCLAMATION, deliveryLabel(state, queuePos), true)
    else -> DeliveryVisual(DeliveryIconKind.CLOCK, "", false)
}

/** 只有失败级(队列满被拒收)才可整条点击重发;其余状态点击没有可重试的东西。 */
internal fun deliveryRetryable(state: String): Boolean = state == DELIVERY_OVERFLOW

private fun DeliveryIconKind.drawableRes(): Int = when (this) {
    DeliveryIconKind.CLOCK -> AppR.drawable.ic_lucide_clock
    DeliveryIconKind.CHECK_SINGLE -> AppR.drawable.ic_lucide_check
    DeliveryIconKind.EXCLAMATION -> AppR.drawable.ic_lucide_triangle_alert
}

/**
 * 投递徽标:12dp 图标 + 10sp 短文案,语义色跟随状态(等待=Accent/提交=Green/
 * 拒收=Red);失败级附「点按重发」动作提示(点击本体挂在外层整条消息上)。
 *
 * 【徽章族一致性】字阶对齐 PaEditorial.Type.State(10sp/行高 14sp,状态词字重
 * 650 —— 与 PaStatusBadge 同一档小字度量);颜色取各语义的 *Text 小字档
 * (10sp 对浅底对比度 ≥4.5:1 的纪律,不用图形档硬撑小字)。形通道(时钟/勾/叹号)
 * 让红绿色弱不靠颜色也能分档。
 *
 * @param mono hacker 终端模式用等宽字体(契约见 PaHacker),常规主题用系统字体。
 */
@Composable
internal fun DeliveryStatusBadge(
    state: String,
    queuePos: Int,
    modifier: Modifier = Modifier,
    mono: Boolean = false,
) {
    val visual = deliveryVisual(state, queuePos)
    if (visual.label.isEmpty()) return
    // 【语义色】失败=红(必须先处理);已提交=绿(到达 daemon 的确认);
    // 等待类=Accent(进行中的中性强调)。红色永远只表示"要用户出手"。
    // 10sp 小字一律取 *Text 档(对比度纪律),图形档只给 12dp 图标。
    val tint = when {
        visual.failure -> PaEditorial.DeniedText
        state == DELIVERY_STEERED -> PaEditorial.RunningText
        else -> PaEditorial.AccentText
    }
    val font = if (mono) TerminalFont.chatFont else null
    Row(modifier, verticalAlignment = Alignment.CenterVertically) {
        Icon(
            painterResource(visual.icon.drawableRes()),
            null,
            tint = tint,
            modifier = Modifier.size(12.dp),
        )
        Spacer(Modifier.width(4.dp))
        Text(
            visual.label,
            fontSize = 10.sp,
            lineHeight = 14.sp,
            fontWeight = FontWeight(650),
            color = tint,
            fontFamily = font,
        )
        if (visual.failure) {
            Spacer(Modifier.width(6.dp))
            Text(
                "点按重发",
                fontSize = 10.sp,
                lineHeight = 14.sp,
                color = PaEditorial.DeniedText,
                fontWeight = FontWeight.Medium,
                fontFamily = font,
            )
        }
    }
}
