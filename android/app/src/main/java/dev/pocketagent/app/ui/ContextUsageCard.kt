package dev.pocketagent.app.ui

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.ui.theme.Pa
import pocketagent.v1.Usage

/**
 * 上下文用量卡(真实数据版)。
 *
 * 替换旧 ContextWindowCard:旧版用"内容字符量 / 4 估算"占用,本版所有数字都来自
 * codex 经 daemon 上报的真实 token 用量(UsageUpdate.usage / total_usage),
 * **完全不做字符估算**。协议不提供"按来源(子代理/MCP/消息/技能)的 token 明细",
 * 因此"条目构成"部分只给真实**条目次数**(从已收消息 blocks 统计),并明确注明来源。
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ContextUsageCard(
    contextWindow: Long,
    windowIsFallback: Boolean,
    turnUsage: Usage?,
    totalUsage: Usage?,
    items: List<ChatItem>,
    onDismiss: () -> Unit,
) {
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    // Tab 只切换"Token 明细/派生指标"的口径:0 = 本 turn,1 = 本会话累计(费用视角)。
    // 【圆环不受 Tab 影响】圆环恒用 turnUsage:每轮输入都重发全部上下文,
    // 本 turn 用量就是"当前窗口被占了多少",与桌面端 Agent 的 context% 同口径。
    var scopeTab by remember { mutableStateOf(0) }
    val usage = if (scopeTab == 0) turnUsage else totalUsage

    // 窗口已知 = 有真实窗口大小且非兜底默认
    val windowKnown = contextWindow > 0 && !windowIsFallback
    // 圆环占用恒用本 turn 用量(当前上下文占用),与本会话累计 Tab 无关(见上方注释)。
    val occupied = usageTotalTokens(turnUsage)
    val ratio = if (windowKnown) occupied.toFloat() / contextWindow.toFloat() else 0f

    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = sheetState,
        containerColor = Pa.CardHigh,
        dragHandle = { Box(Modifier.padding(vertical = 10.dp).size(36.dp, 4.dp).background(Pa.Separator, RoundedCornerShape(50))) },
    ) {
        Column(
            Modifier
                .fillMaxWidth()
                .padding(20.dp, 16.dp, 20.dp, 28.dp)
                .verticalScroll(rememberScrollState()),
        ) {
            // 标题
            Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                Text("上下文窗口", fontSize = 20.sp, color = Pa.TextPrimary, fontWeight = FontWeight.SemiBold)
                Spacer(Modifier.weight(1f))
                Box(
                    Modifier
                        .size(28.dp)
                        .clip(CircleShape)
                        .background(Pa.Bg)
                        .clickable { onDismiss() },
                    contentAlignment = Alignment.Center,
                ) {
                    Text("✕", fontSize = 14.sp, color = Pa.TextSecondary)
                }
            }
            Spacer(Modifier.height(16.dp))

            // 圆环主视觉
            Box(Modifier.fillMaxWidth(), contentAlignment = Alignment.Center) {
                val ringSize = 150.dp
                Canvas(Modifier.size(ringSize)) {
                    val stroke = 13.dp.toPx()
                    // 底环
                    drawArc(
                        color = Pa.Bg,
                        startAngle = -90f,
                        sweepAngle = 360f,
                        useCenter = false,
                        style = Stroke(stroke),
                    )
                    if (windowKnown) {
                        drawArc(
                            color = ringColor(ratio),
                            startAngle = -90f,
                            sweepAngle = 360f * ratio.coerceIn(0f, 1f),
                            useCenter = false,
                            style = Stroke(stroke, cap = StrokeCap.Round),
                        )
                    }
                }
                Column(horizontalAlignment = Alignment.CenterHorizontally) {
                    if (windowKnown) {
                        Text(
                            String.format(java.util.Locale.US, "%.0f%%", ratio * 100f),
                            fontSize = 30.sp,
                            fontWeight = FontWeight.SemiBold,
                            color = ringColor(ratio),
                        )
                        Text(
                            "${formatTokens(occupied)} / ${formatTokens(contextWindow)}",
                            fontSize = 11.sp,
                            color = Pa.TextSecondary,
                        )
                    } else {
                        Text("窗口未知", fontSize = 20.sp, fontWeight = FontWeight.SemiBold, color = Pa.TextSecondary)
                        Text("daemon 尚未上报窗口大小", fontSize = 11.sp, color = Pa.TextTertiary)
                    }
                }
            }
            Spacer(Modifier.height(10.dp))

            // 本 turn / 本会话 切换
            Row(
                Modifier
                    .fillMaxWidth()
                    .clip(RoundedCornerShape(10.dp))
                    .background(Pa.Bg)
                    .padding(4.dp),
                horizontalArrangement = Arrangement.SpaceEvenly,
            ) {
                ScopeTab("本 turn", scopeTab == 0) { scopeTab = 0 }
                ScopeTab("本会话累计", scopeTab == 1) { scopeTab = 1 }
            }
            Spacer(Modifier.height(16.dp))

            // 真实 token 明细
            Text("Token 明细(真实上报)", fontSize = 13.sp, color = Pa.TextPrimary, fontWeight = FontWeight.SemiBold)
            Spacer(Modifier.height(8.dp))
            TokenLine("输入", usage?.inputTokens ?: 0, Pa.Accent)
            TokenLine("输出", usage?.outputTokens ?: 0, Pa.Green)
            TokenLine("缓存读", usage?.cacheReadTokens ?: 0, Pa.Accent.copy(alpha = 0.7f))
            TokenLine("缓存写", usage?.cacheWriteTokens ?: 0, Pa.TextSecondary)
            TokenLine("思考(reasoning)", usage?.reasoningTokens ?: 0, Pa.AccentText)
            if (usage == null) {
                Spacer(Modifier.height(6.dp))
                Text("daemon 尚未上报用量,等待下一帧 tokenUsage/updated。", fontSize = 11.sp, color = Pa.TextTertiary)
            }
            Spacer(Modifier.height(14.dp))

            // 派生指标
            Text("派生指标", fontSize = 13.sp, color = Pa.TextPrimary, fontWeight = FontWeight.SemiBold)
            Spacer(Modifier.height(8.dp))
            DerivedLine("缓存命中率", cacheHitRate(usage), "缓存读 ÷ 输入")
            DerivedLine("输入输出比", ioRatio(usage), "输出 ÷ 输入")
            if (windowKnown) {
                // 剩余余量恒按"当前上下文占用"(本 turn)计算,不随 Tab 切换。
                val remain = (contextWindow - occupied).coerceAtLeast(0)
                DerivedLine("剩余余量", formatTokens(remain), "窗口 − 已用(当前占用)")
            }
            Spacer(Modifier.height(14.dp))

            HorizontalDivider(color = Pa.Separator)
            Spacer(Modifier.height(12.dp))

            // 条目构成(真实次数统计)
            Text("条目构成(真实次数统计)", fontSize = 13.sp, color = Pa.TextPrimary, fontWeight = FontWeight.SemiBold)
            Spacer(Modifier.height(8.dp))
            val stats = computeItemStats(items)
            CountLine("正文消息数", stats.bodyMessages)
            CountLine("命令执行数(shell)", stats.commandExecs)
            CountLine("MCP 工具调用数", stats.mcpCalls)
            CountLine("文件变更数", stats.fileChanges)
            CountLine("其他工具数", stats.otherTools)
            Spacer(Modifier.height(8.dp))
            Text(
                "codex 协议不提供按来源 token 明细,以上为真实条目次数统计(从已收到的消息 blocks 计数)。",
                fontSize = 9.5.sp,
                color = Pa.TextTertiary,
                lineHeight = 14.sp,
            )
        }
    }
}

@Composable
private fun ScopeTab(label: String, active: Boolean, onClick: () -> Unit) {
    Box(
        Modifier
            .clip(RoundedCornerShape(8.dp))
            .background(if (active) Pa.Card else Pa.Bg)
            .clickable { onClick() }
            .padding(horizontal = 16.dp, vertical = 7.dp),
        contentAlignment = Alignment.Center,
    ) {
        Text(label, fontSize = 12.sp, color = if (active) Pa.TextPrimary else Pa.TextSecondary)
    }
}

@Composable
private fun TokenLine(label: String, value: Long, color: Color) {
    Row(Modifier.fillMaxWidth().padding(vertical = 3.5.dp), verticalAlignment = Alignment.CenterVertically) {
        Text(label, Modifier.width(120.dp), fontSize = 11.5.sp, color = Pa.TextSecondary)
        Box(Modifier.size(8.dp).background(color, RoundedCornerShape(2.dp)))
        Spacer(Modifier.width(8.dp))
        Text(formatTokens(value), Modifier.weight(1f), fontSize = 12.sp, color = Pa.TextPrimary, textAlign = TextAlign.End)
    }
}

@Composable
private fun DerivedLine(label: String, value: String, formula: String) {
    Row(Modifier.fillMaxWidth().padding(vertical = 3.dp), verticalAlignment = Alignment.CenterVertically) {
        Text(label, Modifier.width(92.dp), fontSize = 11.5.sp, color = Pa.TextSecondary)
        Text(value, Modifier.width(70.dp), fontSize = 12.sp, color = Pa.TextPrimary, textAlign = TextAlign.End)
        Spacer(Modifier.width(8.dp))
        Text("= $formula", Modifier.weight(1f), fontSize = 10.sp, color = Pa.TextTertiary)
    }
}

@Composable
private fun CountLine(label: String, count: Int) {
    Row(Modifier.fillMaxWidth().padding(vertical = 3.dp), verticalAlignment = Alignment.CenterVertically) {
        Text(label, Modifier.weight(1f), fontSize = 11.5.sp, color = Pa.TextSecondary)
        Text("$count", Modifier.width(56.dp), fontSize = 12.sp, color = Pa.TextPrimary, textAlign = TextAlign.End)
    }
}

/** 占用率环色:命名常量阈值,<80% 主色、≥80% 黄、≥95% 红。 */
private fun ringColor(ratio: Float): Color {
    return when {
        ratio >= RING_DANGER_THRESHOLD -> Pa.Red
        ratio >= RING_WARN_THRESHOLD -> Pa.Yellow
        else -> Pa.Accent
    }
}

private val RING_WARN_THRESHOLD = 0.80f
private val RING_DANGER_THRESHOLD = 0.95f

/** 一个用量对象的真实上下文占用 token(桌面端 codex 同口径:total = input + output;
 *  OpenAI 语义下 cached ⊆ input、reasoning ⊆ output,绝不重复计入,否则占用虚高)。 */
fun usageTotalTokens(u: Usage?): Long {
    if (u == null) return 0L
    return u.inputTokens + u.outputTokens
}

/** 缓存命中率 = 缓存读 ÷ 输入(输入为 0 时记 0,不造数)。 */
fun cacheHitRate(u: Usage?): String {
    if (u == null || u.inputTokens <= 0) return "—"
    val r = u.cacheReadTokens.toFloat() / u.inputTokens.toFloat()
    return String.format(java.util.Locale.US, "%.1f%%", r * 100f)
}

/** 输入输出比 = 输出 ÷ 输入(输入为 0 时记 0)。 */
fun ioRatio(u: Usage?): String {
    if (u == null || u.inputTokens <= 0) return "—"
    val r = u.outputTokens.toFloat() / u.inputTokens.toFloat()
    return String.format(java.util.Locale.US, "%.2f", r)
}

/** 条目构成统计:纯函数,便于单测。次数来自真实消息 blocks,非 token 估算。 */
data class ItemStats(
    val bodyMessages: Int,
    val commandExecs: Int,
    val mcpCalls: Int,
    val fileChanges: Int,
    val otherTools: Int,
)

fun computeItemStats(items: List<ChatItem>): ItemStats {
    var body = 0
    var cmd = 0
    var mcp = 0
    var file = 0
    var other = 0
    for (it in items) {
        when (it.role) {
            "user", "assistant" -> body++
            "tool" -> {
                val n = it.toolName.lowercase()
                when {
                    n == "shell" -> cmd++
                    n == "file_change" || n == "apply_patch" -> file++
                    n.startsWith("mcp") || n.contains("__") -> mcp++
                    else -> other++
                }
            }
        }
    }
    return ItemStats(body, cmd, mcp, file, other)
}

/** 千分位整数(如 "1,234,567")。 */
fun formatTokens(v: Long): String = String.format(java.util.Locale.US, "%,d", v)
