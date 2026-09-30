package dev.pocketagent.app.ui

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
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
import dev.pocketagent.app.ui.component.PaSheet
import dev.pocketagent.app.ui.component.usageOfflineLabel
import dev.pocketagent.app.ui.theme.Pa
import dev.pocketagent.app.ui.theme.PaEditorial
import pocketagent.v1.Usage

/**
 * 上下文用量卡(精简版 · 断网可读)。
 *
 * 【本轮取舍】用户反馈"信息过多",只保留 4 项:输入 / 输出 / 缓存 / 缓存命中率。
 * 之前还堆了"缓存写 / 思考 / 派生指标(输入输出比、剩余余量) / 条目构成次数",
 * 与"我想知道窗口用了多少"这件正事无关,已全部删除。
 *
 * 【断网可读】数字来自实时上报,但断网时退回本地缓存([cachedAt] 非空即表示这是
 * 最后一次已知值),并显示"离线 · 最后更新 HH:mm"提示条,而不是空态 —— 窗口真未知
 * 时也如实显示"窗口 — / daemon 尚未上报窗口大小",绝不假装有数。
 *
 * 【自动刷新】卡片打开期间由调用方每 3s 触发一次重组(见 ChatScreen 的 LaunchedEffect)。
 */
@Composable
fun ContextUsageCard(
    contextWindow: Long,
    windowIsFallback: Boolean,
    turnUsage: Usage?,
    totalUsage: Usage?,
    /** 连接是否可用;false 时显示离线提示条。 */
    online: Boolean,
    /** 最后一次已知用量的时间戳(来自本地缓存);null = 无缓存。 */
    cachedAt: Long?,
    /** 当前时间(展示"最后更新"的新鲜度用)。 */
    nowMillis: Long,
    /** Codex 正在执行 contextCompaction item。 */
    compactionInProgress: Boolean = false,
    /** 最近一次上下文压缩时间(0 = 未压缩过);驱动"已压缩"状态显示。 */
    lastCompactedAt: Long = 0L,
    /**
     * 会话归属 agent 的展示名("Pi"/"Codex"/…),压缩等待文案用它点名。
     * 真机实锤(图9 同源问题):过去"等待 Codex 完成压缩"硬编码 codex,Pi 会话
     * 里张冠李戴;空串退中性"agent",绝不冒名。
     */
    agentLabel: String = "",
    onDismiss: () -> Unit,
) {
    // Tab 只切换下面 4 项的口径:0 = 本 turn,1 = 本会话累计。
    // 【圆环不受 Tab 影响】圆环恒用 turnUsage:每轮输入都重发全部上下文,
    // 本 turn 用量就是"当前窗口被占了多少",与桌面端 Agent 的 context% 同口径。
    var scopeTab by remember { mutableStateOf(0) }
    val usage = if (scopeTab == 0) turnUsage else totalUsage

    // 窗口已知 = 有真实窗口大小且非兜底默认
    val windowKnown = contextWindow > 0 && !windowIsFallback
    // 【压缩同步】压缩后、新一轮用量上报前:旧占用已失真(可能超出实际窗口),
    // 绝不继续显示 —— 环与占比进入"已压缩"态,等真实数据回来再恢复。
    val compactedPending = !compactionInProgress && compactionPending(turnUsage, lastCompactedAt)
    val occupied = usageTotalTokens(turnUsage)
    val ratio = if (windowKnown && !compactedPending) occupied.toFloat() / contextWindow.toFloat() else 0f

    // 【本轮换 PaSheet】统一玻璃浮层:82% 屏高上限 + 竖向滚动由封装内建
    // (AUDIT sheet 溢出规则的结构化落地),底色走玻璃 Tint 而不是 CardHigh。
    PaSheet(onDismissRequest = onDismiss) {
        Column(
            Modifier
                .fillMaxWidth()
                .padding(20.dp, 16.dp, 20.dp, 28.dp),
        ) {
            // 标题
            Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                Text("上下文窗口", fontSize = 20.sp, color = PaEditorial.Ink, fontWeight = FontWeight.SemiBold)
                Spacer(Modifier.weight(1f))
                Box(
                    Modifier
                        .size(28.dp)
                        .clip(CircleShape)
                        .background(PaEditorial.Canvas)
                        .clickable { onDismiss() },
                    contentAlignment = Alignment.Center,
                ) {
                    Text("✕", fontSize = 14.sp, color = PaEditorial.Muted)
                }
            }
            Spacer(Modifier.height(12.dp))

            // 离线提示条:只在断网时出现,如实标出数据新鲜度(不设空态、不假装实时)。
            if (!online) {
                Row(
                    Modifier
                        .fillMaxWidth()
                        .background(PaEditorial.Attention.copy(alpha = 0.12f), RoundedCornerShape(10.dp))
                        .padding(horizontal = 12.dp, vertical = 7.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(
                        usageOfflineLabel(cachedAt ?: 0L, nowMillis),
                        fontSize = 11.sp, color = PaEditorial.Attention, maxLines = 1,
                    )
                }
                Spacer(Modifier.height(12.dp))
            }

            // 压缩提示条:压缩后、新一轮用量上报前出现,如实说明当前占比为何留白。
            if (compactedPending) {
                Row(
                    Modifier
                        .fillMaxWidth()
                        .background(PaEditorial.Primary.copy(alpha = 0.12f), RoundedCornerShape(10.dp))
                        .padding(horizontal = 12.dp, vertical = 7.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(
                        compactionNoticeLabel(lastCompactedAt, nowMillis),
                        fontSize = 11.sp, color = PaEditorial.Primary, maxLines = 1,
                    )
                }
                Spacer(Modifier.height(12.dp))
            }
            if (compactionInProgress) {
                Row(
                    Modifier
                        .fillMaxWidth()
                        .background(PaEditorial.Primary.copy(alpha = 0.12f), RoundedCornerShape(10.dp))
                        .padding(horizontal = 12.dp, vertical = 7.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(
                        "上下文压缩中 · 旧占用暂不显示",
                        fontSize = 11.sp,
                        color = PaEditorial.Primary,
                        maxLines = 1,
                    )
                }
                Spacer(Modifier.height(12.dp))
            }

            // 圆环主视觉(保留)
            Box(Modifier.fillMaxWidth(), contentAlignment = Alignment.Center) {
                val ringSize = 150.dp
                Canvas(Modifier.size(ringSize)) {
                    val stroke = 13.dp.toPx()
                    // 底环
                    drawArc(
                        color = PaEditorial.Canvas,
                        startAngle = -90f,
                        sweepAngle = 360f,
                        useCenter = false,
                        style = Stroke(stroke),
                    )
                    if (windowKnown && !compactedPending && !compactionInProgress) {
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
                    if (compactionInProgress) {
                        Text("压缩中", fontSize = 20.sp, fontWeight = FontWeight.SemiBold, color = PaEditorial.Primary)
                        // 按 agent 点名(agentLabel 空串退中性"agent"),不再硬编码 Codex。
                        Text(
                            "等待 ${agentLabel.ifBlank { "agent" }} 完成压缩",
                            fontSize = 11.sp,
                            color = PaEditorial.Subtle,
                        )
                    } else if (compactedPending) {
                        // 压缩态:不显示任何百分比 —— 压缩前的旧占比已失真,
                        // 新占比要等下一轮用量上报,此处绝不造数。
                        Text("已压缩", fontSize = 20.sp, fontWeight = FontWeight.SemiBold, color = PaEditorial.Primary)
                        Text("等待新一轮用量上报", fontSize = 11.sp, color = PaEditorial.Subtle)
                    } else if (windowKnown) {
                        Text(
                            String.format(java.util.Locale.US, "%.0f%%", ratio * 100f),
                            fontSize = 30.sp,
                            fontWeight = FontWeight.SemiBold,
                            color = ringColor(ratio),
                        )
                        Text(
                            "${formatTokens(occupied)} / ${formatTokens(contextWindow)}",
                            fontSize = 11.sp,
                            color = PaEditorial.Muted,
                        )
                    } else {
                        // 窗口真未知(schema 无该字段/daemon 未上报):如实显"—"不造数
                        // (README §11 既定口径,与 [ContextRing] 的 ringPercentLabel 同一形态),
                        // 不写「未知」字样(AUDIT §3.3 禁令)。
                        Text("窗口 —", fontSize = 20.sp, fontWeight = FontWeight.SemiBold, color = PaEditorial.Muted)
                        Text("daemon 尚未上报窗口大小", fontSize = 11.sp, color = PaEditorial.Subtle)
                    }
                }
            }
            Spacer(Modifier.height(14.dp))

            // 本 turn / 本会话 切换(仅切换下面 4 项的口径;圆环恒为本 turn 占用)
            Row(
                Modifier
                    .fillMaxWidth()
                    .clip(RoundedCornerShape(10.dp))
                    .background(PaEditorial.Canvas)
                    .padding(4.dp),
                horizontalArrangement = Arrangement.SpaceEvenly,
            ) {
                ScopeTab("本 turn", scopeTab == 0) { scopeTab = 0 }
                ScopeTab("本会话累计", scopeTab == 1) { scopeTab = 1 }
            }
            Spacer(Modifier.height(16.dp))

            // 指标表:4 项(输入 / 输出 / 缓存 / 缓存命中率)。
            // 【为什么是表】以后要加一项只需在 usageMetrics() 里加一行,不必再复制一段 UI。
            Text("用量明细", fontSize = 13.sp, color = PaEditorial.Ink, fontWeight = FontWeight.SemiBold)
            Spacer(Modifier.height(8.dp))
            usageMetrics().forEach { m ->
                MetricLine(m.label, m.value(usage), toneColor(m.tone))
            }
        }
    }
}

/** 4 项指标的色调(与主题令牌解耦,保证指标表可在 JVM 单测里被取值验证)。 */
internal enum class UsageMetricTone { ACCENT, GREEN, ACCENT_DIM, NEUTRAL }

/** 一项用量指标:名称 + 色调 + 取值。以后加一项只加一行。 */
internal class UsageMetric(
    val label: String,
    val tone: UsageMetricTone,
    val value: (Usage?) -> String,
)

/**
 * 上下文窗口卡只展示的 4 项指标(纯函数,便于单测)。
 * "缓存"取 cacheReadTokens —— 与"缓存命中率 = 缓存读 ÷ 输入"同口径,不另造合计口径。
 */
internal fun usageMetrics(): List<UsageMetric> = listOf(
    UsageMetric("输入", UsageMetricTone.ACCENT) { formatTokens(it?.inputTokens ?: 0L) },
    UsageMetric("输出", UsageMetricTone.GREEN) { formatTokens(it?.outputTokens ?: 0L) },
    UsageMetric("缓存", UsageMetricTone.ACCENT_DIM) { formatTokens(it?.cacheReadTokens ?: 0L) },
    UsageMetric("缓存命中率", UsageMetricTone.NEUTRAL) { cacheHitRate(it) },
)

@Composable
private fun toneColor(tone: UsageMetricTone): Color = when (tone) {
    UsageMetricTone.ACCENT -> PaEditorial.Primary
    UsageMetricTone.GREEN -> PaEditorial.Running
    UsageMetricTone.ACCENT_DIM -> PaEditorial.Primary.copy(alpha = 0.7f)
    UsageMetricTone.NEUTRAL -> PaEditorial.Muted
}

@Composable
private fun ScopeTab(label: String, active: Boolean, onClick: () -> Unit) {
    Box(
        Modifier
            .clip(RoundedCornerShape(8.dp))
            .background(if (active) PaEditorial.Surface else PaEditorial.Canvas)
            .clickable { onClick() }
            .padding(horizontal = 16.dp, vertical = 7.dp),
        contentAlignment = Alignment.Center,
    ) {
        Text(label, fontSize = 12.sp, color = if (active) PaEditorial.Ink else PaEditorial.Muted)
    }
}

@Composable
private fun MetricLine(label: String, value: String, color: Color) {
    Row(Modifier.fillMaxWidth().padding(vertical = 4.dp), verticalAlignment = Alignment.CenterVertically) {
        Text(label, Modifier.weight(1f), fontSize = 12.sp, color = PaEditorial.Muted)
        Box(Modifier.size(8.dp).background(color, RoundedCornerShape(2.dp)))
        Spacer(Modifier.width(8.dp))
        Text(value, fontSize = 13.sp, color = PaEditorial.Ink, textAlign = TextAlign.End)
    }
}

/** 占用率环色:命名常量阈值,<80% 主色、≥80% 黄、≥95% 红。 */
internal fun ringColor(ratio: Float): Color {
    return when {
        ratio >= RING_DANGER_THRESHOLD -> PaEditorial.Denied
        ratio >= RING_WARN_THRESHOLD -> PaEditorial.Attention
        else -> PaEditorial.Primary
    }
}

private val RING_WARN_THRESHOLD = 0.80f
private val RING_DANGER_THRESHOLD = 0.95f

/** 输入栏小环的线宽。 */
private val RING_STROKE = 3.dp

/**
 * 上下文占用环:外圈一个圆环,环内直接显示实际使用占比。
 *
 * 【为什么是环不是胶囊】旧版是「圆形线性图标 + 上下文 128k」的胶囊:横向占位大,
 * 而且**看不出窗口被占了多少**。圆环一眼给占比,窗口总量与用量明细点开卡面看。
 *
 * 【环内文案必须自适应字号】"100%"比"9%"多两个字符。固定字号会让百分号贴到环上、
 * 甚至被裁掉 —— 那就又变成"看起来有、实际读不到"。这里按文案长度取字号,
 * 保证任何占比都完整可见(纯函数,单测覆盖)。
 *
 * [windowKnown] = false 时画一圈空环 + "—":**绝不假装 0%**(那会被读成"窗口是空的")。
 */
@Composable
fun ContextRing(
    ratio: Float,
    windowKnown: Boolean,
    modifier: Modifier = Modifier.size(28.dp),
    /** 压缩后等新用量:环内显示"已压",不显示任何百分比(旧占比已失真)。 */
    compacted: Boolean = false,
) {
    val label = if (compacted) "已压" else ringPercentLabel(ratio, windowKnown)
    val color = when {
        compacted -> PaEditorial.Primary
        windowKnown -> ringColor(ratio)
        else -> PaEditorial.Subtle
    }
    Box(modifier, contentAlignment = Alignment.Center) {
        Canvas(Modifier.fillMaxSize()) {
            val stroke = RING_STROKE.toPx()
            // 底环:先铺满一圈,占比弧叠在上面
            drawArc(
                color = PaEditorial.Canvas,
                startAngle = -90f,
                sweepAngle = 360f,
                useCenter = false,
                style = Stroke(stroke),
            )
            if (windowKnown && !compacted) {
                drawArc(
                    color = color,
                    startAngle = -90f,
                    sweepAngle = 360f * ratio.coerceIn(0f, 1f),
                    useCenter = false,
                    style = Stroke(stroke, cap = StrokeCap.Round),
                )
            }
        }
        Text(
            label,
            fontSize = ringPercentFontSize(label).sp,
            fontWeight = FontWeight.SemiBold,
            color = color,
            maxLines = 1,
        )
    }
}

/** 环内文案:窗口/用量未知时给"—",不造数。 */
internal fun ringPercentLabel(ratio: Float, windowKnown: Boolean): String =
    if (!windowKnown) "—" else String.format(java.util.Locale.US, "%.0f%%", ratio * 100f)

/**
 * 环内文案字号(sp)。按字符数自适应:
 * "9%" / "42%" 用 9.5sp;"100%" / "—" 用 7.5sp —— 保证最长文案也不出圈、不被裁。
 */
internal fun ringPercentFontSize(label: String): Float = when {
    label.length <= 3 -> 9.5f
    else -> 7.5f
}

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

/** 千分位整数(如 "1,234,567")。 */
fun formatTokens(v: Long): String = String.format(java.util.Locale.US, "%,d", v)

// ---- 上下文压缩同步(纯函数,可 JVM 单测) ----

/**
 * 是否处于"已压缩、等新用量"态:压缩发生过、且压缩前的旧占用已作废(turnUsage=null)。
 * 新一轮用量上报后 turnUsage 恢复非空,自动退出该态 —— 不需要额外的清除时机。
 */
internal fun compactionPending(turnUsage: Usage?, lastCompactedAt: Long): Boolean =
    lastCompactedAt > 0L && turnUsage == null

/**
 * 压缩提示条文案。纯函数。
 * 同一天只给时刻("已于 14:03 压缩");跨天补日期,与离线提示条同一套时间语义。
 */
internal fun compactionNoticeLabel(compactedAt: Long, now: Long): String {
    if (compactedAt <= 0L) return "上下文已压缩 · 等待新一轮用量上报"
    val pattern = if (dev.pocketagent.app.ui.component.sameCalendarDay(compactedAt, now)) {
        "'已于' HH:mm '压缩 · 等待新一轮用量上报'"
    } else {
        "'已于' M月d日 HH:mm '压缩 · 等待新一轮用量上报'"
    }
    return java.text.SimpleDateFormat(pattern, java.util.Locale.getDefault())
        .format(java.util.Date(compactedAt))
}
