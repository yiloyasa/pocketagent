package dev.pocketagent.app.ui.component

import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.ui.anim.rememberReduceMotion
import dev.pocketagent.app.ui.theme.Pa
import dev.pocketagent.app.ui.theme.PaEditorial
import pocketagent.v1.ThreadStatus

/** 会话徽章的三种语义(审计 P1-1:Running 脉冲 / Waiting 红点 / Failed 红!)。 */
enum class PaThreadBadgeKind { RUNNING, WAITING, FAILED }

/**
 * 会话状态徽章的语义值:[label] 是短标签(列表行右侧放得下),
 * [detail] 仅 FAILED 有值 = 最后错误摘要(首行),由宿主行自行决定是否展示。
 */
data class PaThreadBadge(
    val kind: PaThreadBadgeKind,
    val label: String,
    val detail: String? = null,
)

/**
 * 最后错误的摘要(纯函数):daemon 的 last_turn_error 常带方法名前缀与多行
 * ("session/resume\n<id> 失败:..."),列表行只放得下人话首行;超长截到 40 字
 * 加省略号。空输入(全是空白)返回空串。
 */
fun lastTurnErrorSummary(raw: String, maxLen: Int = 40): String {
    val firstLine = raw.lines().firstOrNull { it.isNotBlank() }?.trim().orEmpty()
    if (firstLine.isEmpty()) return ""
    return if (firstLine.length <= maxLen) firstLine else firstLine.take(maxLen) + "…"
}

/**
 * 会话三态徽章语义(纯函数,单测钉死 ThreadStatusBadgeTest;审计 P1-1 单一事实源)。
 *
 * ThreadsScreen 会话卡与 HomeScreen 最近任务卡都从这里取徽章,同一会话在两处
 * 必须长出同一种状态(以前主页有一套、列表一套,口径迟早分叉)。
 *
 * 【优先级与主页三分同口径】待审批(对象在手,或 daemon 已置 WAITING_APPROVAL)
 * > 运行中 > 失败(IDLE + lastTurnError,与 TurnSession 恢复路径的失败判定一致)。
 * 挂着审批的会话等的是用户,不该被显示成"运行中"。
 *
 * Done/Idle 无错/归档 → null:无特殊就不渲染,不摆"一切正常"的徽章空壳。
 * 文案禁「未知」「错误」(AUDIT §3.3):失败态用「中断」+错误摘要原文。
 */
fun threadStatusBadge(
    status: ThreadStatus,
    pendingApprovals: Int,
    lastTurnError: String,
): PaThreadBadge? = when {
    pendingApprovals > 0 || status == ThreadStatus.THREAD_STATUS_WAITING_APPROVAL ->
        PaThreadBadge(PaThreadBadgeKind.WAITING, "待你处理")
    status == ThreadStatus.THREAD_STATUS_RUNNING ->
        PaThreadBadge(PaThreadBadgeKind.RUNNING, "运行中")
    status == ThreadStatus.THREAD_STATUS_IDLE && lastTurnError.isNotBlank() ->
        PaThreadBadge(PaThreadBadgeKind.FAILED, "中断", lastTurnErrorSummary(lastTurnError))
    else -> null
}

/**
 * 会话状态徽章(P1-1):小面积语义色,前缀点/叹号 + 着色短标签,统一 10.5sp。
 *  - RUNNING:绿色脉冲点(系统减弱动效开启时退化为静态点,与全 app 动效纪律一致);
 *  - WAITING:红色静态点(静止的红点是"等你"的语义,与主页「待你处理」区同色);
 *  - FAILED:红色粗体「!」(摘要由宿主行按版面决定展示,徽章本身保持单行紧凑)。
 */
@Composable
fun ThreadStatusBadge(badge: PaThreadBadge, modifier: Modifier = Modifier) {
    val (dotColor, labelColor) = when (badge.kind) {
        PaThreadBadgeKind.RUNNING -> PaEditorial.Running to PaEditorial.RunningText
        PaThreadBadgeKind.WAITING -> PaEditorial.Denied to PaEditorial.DeniedText
        PaThreadBadgeKind.FAILED -> PaEditorial.Denied to PaEditorial.DeniedText
    }
    Row(modifier, verticalAlignment = Alignment.CenterVertically) {
        when (badge.kind) {
            PaThreadBadgeKind.FAILED -> Text(
                "!",
                fontSize = 11.sp, color = dotColor, fontWeight = FontWeight.Bold,
            )
            else -> PulseDot8(dotColor, pulse = badge.kind == PaThreadBadgeKind.RUNNING)
        }
        Spacer(Modifier.width(4.dp))
        Text(badge.label, fontSize = 10.5.sp, color = labelColor, maxLines = 1)
    }
}

/** 8dp 语义状态点:RUNNING 脉冲、其余静态(减弱动效时不脉冲)。 */
@Composable
private fun PulseDot8(color: Color, pulse: Boolean) {
    val reduceMotion = rememberReduceMotion()
    if (pulse && !reduceMotion) {
        val transition = rememberInfiniteTransition(label = "threadBadgePulse")
        val alpha by transition.animateFloat(
            initialValue = 1f,
            targetValue = 0.35f,
            animationSpec = infiniteRepeatable(tween(900, easing = LinearEasing), RepeatMode.Reverse),
            label = "threadBadgePulseAlpha",
        )
        androidx.compose.foundation.layout.Box(
            Modifier.size(8.dp).graphicsLayer { this.alpha = alpha }.background(color, CircleShape),
        )
    } else {
        androidx.compose.foundation.layout.Box(
            Modifier.size(8.dp).background(color, CircleShape),
        )
    }
}
