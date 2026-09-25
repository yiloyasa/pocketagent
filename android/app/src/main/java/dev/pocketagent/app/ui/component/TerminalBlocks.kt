package dev.pocketagent.app.ui.component

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.IntrinsicSize
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawWithContent
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.platform.LocalUriHandler
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.Font
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.AgentConnection
import dev.pocketagent.app.R as AppR
import dev.pocketagent.app.ui.ChatItem
import dev.pocketagent.app.ui.DELIVERY_OVERFLOW
import dev.pocketagent.app.ui.MarkdownText
import dev.pocketagent.app.ui.anim.pressable
import dev.pocketagent.app.ui.anim.rememberReduceMotion
import dev.pocketagent.app.ui.deliveryLabel
import dev.pocketagent.app.ui.looksLikeSubagentDelegation
import dev.pocketagent.app.ui.theme.Pa
import dev.pocketagent.app.ui.theme.PaHacker
import kotlinx.coroutines.delay
import java.util.Locale
import java.text.SimpleDateFormat
import java.util.Date

/**
 * 黑客终端模式主对话的 Block 级组件 —— 「终端会话,不是聊天气泡」。
 *
 * 【设计公理(调研结论,来源 Warp Blocks / cool-retro-term / eDEX / starship / Tokyo Night)】
 *  - 黑客感=结构不是颜色:层级靠 Block 类型(输入/输出/工具/脚注)而非底色;
 *  - 代码感=语法层级:提示符/状态符/正文分三级颜色,不铺饱和色;
 *  - 纯饱和色铺纯黑是灾难:正文恒 Pa.TextPrimary,彩色只给提示符与状态符。
 *
 * 【四类 Block】① 用户输入 `❯ 正文`(无气泡无卡片);② Agent 输出(无底色,
 * 块间留白分隔);③ 工具调用 `▸ 名  参数`(左 1dp Separator 竖条,可折叠);
 * ④ 回合脚注 `✓ 3.2s` / `✗`(右对齐 11sp)。tmux 式状态条见 [TmuxStatusLine]。
 *
 * 【常规主题零分叉】本文件所有组件只在 `PaHacker.isHacker` 为 true 的分支里被
 * ChatScreen 调用;常规主题的渲染路径一行不碰。纯函数(映射/前缀/脚注格式化)
 * 集中在文件头部,JVM 单测钉住(见 TerminalBlocksTest)。
 *
 * 【不造数据红线】ChatItem 没有时间戳 → 时间戳 gutter 整条省略;回合没有
 * 时长数据 → 脚注只显示 ✓/✗ 或真实 token 数;工具失败没有退出码 → 只显示 ✗。
 */

// ─────────────────────────────────────────────────────────────────────────────
// 纯函数(JVM 可测,不触 Compose 运行时)
// ─────────────────────────────────────────────────────────────────────────────

/** 用户输入提示符:U+276F + 空格。多行输入时 ❯ 只出现在首行(见 terminalUserPrompt)。 */
internal const val TERM_PROMPT = "❯"

/** 用户输入 Block 的正文构造:提示符只落首行,多行后续行顶格(终端回显语义)。 */
internal fun terminalUserPrompt(text: String): String = "$TERM_PROMPT $text"

/** Composer 行首提示符:输入非空才渲染(hacker 模式,空输入显示 hint 不显示 ❯)。 */
internal fun composerShowsPrompt(text: String): Boolean = text.isNotEmpty()

/** braille 转轮帧序(调研规格:⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏,80ms/帧)。 */
internal const val BRAILLE_FRAMES = "⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏"

/** braille 转轮按帧号取字符(负数帧回绕,防御 LaunchedEffect 之外的调用)。 */
internal fun brailleFrameAt(tick: Int): Char =
    BRAILLE_FRAMES[Math.floorMod(tick, BRAILLE_FRAMES.length)]

/**
 * ChatItem → 终端 Block 映射(枚举全量,新增 role 落 OUTPUT 前必须过这里)。
 *
 * 【映射表】user→①;tool→③;assistant 与其它文本类(历史快照的各 role 文本)
 * →②;thinking / codex_event 无终端形态 → KEEP_EXISTING(沿用既有渲染);
 * plan(已上提为面板)/ usage(无正文)→ SKIP。
 */
internal enum class TerminalBlockKind { USER, OUTPUT, TOOL, KEEP_EXISTING, SKIP }

internal fun terminalBlockKind(item: ChatItem): TerminalBlockKind = when (item.role) {
    "user" -> TerminalBlockKind.USER
    "tool" -> TerminalBlockKind.TOOL
    "thinking", "codex_event" -> TerminalBlockKind.KEEP_EXISTING
    "plan", "usage" -> TerminalBlockKind.SKIP
    else -> TerminalBlockKind.OUTPUT // assistant 及历史快照文本类
}

/**
 * 工具参数摘要:压平空白后截断(单行放不下就省略号),空参数返回空串
 * (调用方据此隐藏参数段)。不解析 JSON 语义 —— 摘要只求一眼可辨,展开看全文。
 */
internal fun terminalToolArgsSummary(args: String, maxLen: Int = 48): String {
    val flat = args.replace(Regex("\\s+"), " ").trim()
    if (flat.isEmpty()) return ""
    return if (flat.length <= maxLen) flat else flat.take(maxLen) + "…"
}

/**
 * 工具 Block 右侧状态符:运行中返回空串(调用方渲染转轮);
 * 成功 `✓`;失败 `✗`(没有退出码数据,不编 `exit N`);真实耗时>0 时附在状态符后。
 */
internal fun terminalToolStatusMark(status: String, elapsedMs: Double): String {
    val mark = when (status) {
        "success" -> "✓"
        "failure" -> "✗"
        else -> return ""
    }
    if (elapsedMs <= 0.0) return mark
    return "$mark ${String.format(Locale.US, "%.1fs", elapsedMs / 1000.0)}"
}

/** token 数紧凑格式:1234→"1.2k",1234567→"1.2m"(脚注用,与 starship 同风格)。 */
internal fun formatTokenCount(tokens: Long): String = when {
    tokens >= 1_000_000 -> String.format(Locale.US, "%.1fm", tokens / 1_000_000.0)
    tokens >= 1_000 -> String.format(Locale.US, "%.1fk", tokens / 1_000.0)
    else -> tokens.toString()
}

/**
 * 回合脚注文案:失败 `✗`;成功时有时长给 `✓ 3.2s`,没有时长但有真实 token
 * 给 `✓ 1.2k tok`,都没有就只给 `✓` —— 没有的数据一律不编。
 */
internal fun formatTurnFootnote(success: Boolean, elapsedMs: Double, outputTokens: Long): String {
    if (!success) return "✗"
    val tail = when {
        elapsedMs > 0.0 -> String.format(Locale.US, "%.1fs", elapsedMs / 1000.0)
        outputTokens > 0 -> "${formatTokenCount(outputTokens)} tok"
        else -> ""
    }
    return if (tail.isEmpty()) "✓" else "✓ $tail"
}

/** tmux 状态条连接段文案:ok=已连接;其余(checking 巡检态/down)=重连中。 */
internal fun tmuxConnLabel(connState: String): String =
    if (connState == "ok") "已连接" else "重连中"

/** tmux 状态条时钟段 `HH:MM`(24 小时,真实设备时钟)。 */
internal fun tmuxClockLabel(epochMs: Long): String =
    SimpleDateFormat("HH:mm", Locale.US).format(Date(epochMs))

// ─────────────────────────────────────────────────────────────────────────────
// 字体:JetBrains Mono(Regular+Bold,OFL,已随 res/font 落库;许可见
// res/raw/jetbrains_mono_ofl.txt)。回退方案:若字体资源不可用,消费方
// 在 hacker 分支里退回 PaHacker.mono(系统等宽)—— 本对象不做运行时探测。
// ─────────────────────────────────────────────────────────────────────────────

/** hacker 模式对话字体。theme/PaHacker.kt 被并行线冻结,故 chatFont 落在新文件。 */
object TerminalFont {
    val chatFont: FontFamily = FontFamily(
        Font(AppR.font.jetbrains_mono_regular, FontWeight.Normal),
        Font(AppR.font.jetbrains_mono_bold, FontWeight.Bold),
    )
}

// ─────────────────────────────────────────────────────────────────────────────
// Block 组件
// ─────────────────────────────────────────────────────────────────────────────

/** 等宽正文的统一排版网格:14sp / 行高 1.45(调研规格)。 */
internal fun terminalBodyFontSize(uiScale: Float) = 14f * uiScale
internal fun terminalBodyLineHeight(uiScale: Float) = terminalBodyFontSize(uiScale) * 1.45f

/**
 * ① 用户输入 Block:`❯ 正文` —— ❯ bold Pa.Accent,正文 bold Pa.TextPrimary;
 * 无气泡无卡片背景(与普通聊天的根本区别);多行时 ❯ 只在首行。
 * 附件网格与投递状态标签沿用常规主题的既有语义(渲染在正文上方/下方)。
 */
@Composable
internal fun TerminalUserBlock(
    item: ChatItem,
    uiScale: Float,
    queueClearOn: Boolean,
    onClearQueued: () -> Unit,
    attachmentsContent: (@Composable () -> Unit)? = null,
) {
    Column(Modifier.fillMaxWidth()) {
        if (attachmentsContent != null) {
            attachmentsContent()
            Spacer(Modifier.height(6.dp))
        }
        if (item.text.isNotBlank()) {
            SelectionContainer {
                Text(
                    buildAnnotatedString {
                        withStyle(SpanStyle(color = Pa.Accent, fontWeight = FontWeight.Bold)) {
                            append("$TERM_PROMPT ")
                        }
                        withStyle(SpanStyle(color = Pa.TextPrimary, fontWeight = FontWeight.Bold)) {
                            append(item.text)
                        }
                    },
                    fontSize = terminalBodyFontSize(uiScale).sp,
                    lineHeight = terminalBodyLineHeight(uiScale).sp,
                    fontFamily = TerminalFont.chatFont,
                )
            }
        }
        // P1-4 投递状态标签:与常规主题同一套语义(pending/steered/queued/overflow),
        // 复用同一纯函数 deliveryLabel(TurnDelivery.kt)防两处分叉;仅外观归入终端块。
        if (item.deliverState.isNotEmpty()) {
            val label = deliveryLabel(item.deliverState, item.queuePos)
            if (label.isNotEmpty()) {
                Spacer(Modifier.height(2.dp))
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text(
                        "·",
                        color = if (item.deliverState == DELIVERY_OVERFLOW) Pa.Red else Pa.AccentText,
                        fontSize = 10.sp,
                        fontFamily = TerminalFont.chatFont,
                    )
                    Spacer(Modifier.width(6.dp))
                    Text(
                        label,
                        fontSize = 10.sp,
                        color = if (item.deliverState == DELIVERY_OVERFLOW) Pa.Red else Pa.AccentText,
                        fontFamily = TerminalFont.chatFont,
                    )
                    if (queueClearOn) {
                        Text(
                            "撤回排队",
                            Modifier
                                .pressable(haptic = true) { onClearQueued() }
                                .padding(horizontal = 8.dp, vertical = 2.dp),
                            fontSize = 10.sp,
                            color = Pa.Red,
                            fontFamily = TerminalFont.chatFont,
                        )
                    }
                }
            }
        }
    }
}

/**
 * ② Agent 输出 Block:无底色差(层级靠留白与语法色),正文走 MarkdownText
 * (hacker 下自动等宽);streaming=true 时块尾放 braille 转轮(80ms/帧 Pa.Accent)。
 */
@Composable
internal fun TerminalOutputBlock(
    text: String,
    uiScale: Float,
    onOpenUrl: (String) -> Unit,
    streaming: Boolean = false,
) {
    Column(Modifier.fillMaxWidth()) {
        if (text.isNotBlank()) {
            SelectionContainer {
                val handler = remember(onOpenUrl) {
                    object : androidx.compose.ui.platform.UriHandler {
                        override fun openUri(uri: String) {
                            onOpenUrl(uri)
                        }
                    }
                }
                androidx.compose.runtime.CompositionLocalProvider(
                    LocalUriHandler provides handler,
                ) {
                    MarkdownText(
                        text,
                        Modifier.fillMaxWidth(),
                        terminalBodyFontSize(uiScale),
                    )
                }
            }
        }
        if (streaming) {
            Row(Modifier.padding(start = 1.dp, top = if (text.isBlank()) 0.dp else 4.dp)) {
                BrailleSpinner()
            }
        }
    }
}

/** braille 转轮:⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏,80ms/帧;减弱动效时静止首帧(不闪)。 */
@Composable
internal fun BrailleSpinner(color: Color = Pa.Accent) {
    val reduceMotion = rememberReduceMotion()
    var tick by remember { mutableStateOf(0) }
    if (!reduceMotion) {
        LaunchedEffect(Unit) {
            while (true) {
                delay(80)
                tick++
            }
        }
    }
    Text(
        brailleFrameAt(tick).toString(),
        color = color,
        fontSize = 13.sp,
        fontFamily = TerminalFont.chatFont,
    )
}

/**
 * ③ 工具调用 Block:`▸ 工具名  参数摘要` + 右对齐状态(运行中=转轮 /
 * 成功=✓ / 失败=✗);左侧 1dp Pa.Separator 竖条横贯整块;单击折叠
 * (折叠只留一行)。参数/输出等宽小字,不再铺卡片底色。
 */
@Composable
internal fun TerminalToolBlock(
    item: ChatItem,
    uiScale: Float,
) {
    // 折叠语义与常规 ToolCallCard 一致:全局「展开工具详情」开关优先;
    // 运行中默认展开(看得到正在干什么);子代理委派默认展开(委派内容必须可见)。
    var expanded by remember(item.toolId, AgentConnection.toolDetailsExpanded) {
        mutableStateOf(
            AgentConnection.toolDetailsExpanded ||
                looksLikeSubagentDelegation(item.toolArgs) ||
                item.toolStatus == "running",
        )
    }
    val statusMark = terminalToolStatusMark(item.toolStatus, item.toolElapsedMs)
    val running = statusMark.isEmpty()
    val statusColor = when (item.toolStatus) {
        "failure" -> Pa.Red
        "success" -> Pa.Green
        else -> Pa.Accent
    }
    Row(
        Modifier
            .fillMaxWidth()
            .height(IntrinsicSize.Min)
            .pressable(haptic = true) { expanded = !expanded },
        verticalAlignment = Alignment.CenterVertically,
    ) {
        // 左 1dp 竖条横贯整块 —— 工具过程的"装订线",层级靠结构不靠底色
        Box(
            Modifier
                .width(1.dp)
                .fillMaxHeight()
                .background(Pa.Separator),
        )
        Spacer(Modifier.width(8.dp))
        // 名字与参数摘要共享剩余宽度(各占一半封顶,超长省略号),
        // 状态符恒在右缘不被长参数挤出视野。
        Row(Modifier.weight(1f), verticalAlignment = Alignment.CenterVertically) {
            Text(
                "▸ ",
                color = Pa.TextTertiary,
                fontSize = (13 * uiScale).sp,
                fontFamily = TerminalFont.chatFont,
            )
            Text(
                item.toolName.ifBlank { "工具" },
                Modifier.weight(1f, fill = false),
                color = Pa.TextPrimary,
                fontSize = (13 * uiScale).sp,
                fontFamily = TerminalFont.chatFont,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            val argsSummary = terminalToolArgsSummary(item.toolArgs)
            if (argsSummary.isNotEmpty()) {
                Spacer(Modifier.width(6.dp))
                Text(
                    argsSummary,
                    Modifier.weight(1f, fill = false),
                    color = Pa.TextTertiary,
                    fontSize = (11.5f * uiScale).sp,
                    fontFamily = TerminalFont.chatFont,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
            }
        }
        Spacer(Modifier.width(8.dp))
        if (running) {
            BrailleSpinner(color = Pa.Accent)
        } else {
            Text(
                statusMark,
                color = statusColor,
                fontSize = (11 * uiScale).sp,
                fontFamily = TerminalFont.chatFont,
            )
        }
    }
    if (expanded) {
        Column(
            Modifier
                .fillMaxWidth()
                .padding(start = 9.dp, top = 6.dp),
        ) {
            if (item.toolArgs.isNotBlank()) {
                Text(
                    item.toolArgs,
                    color = Pa.TextSecondary,
                    fontSize = (10.5f * uiScale).sp,
                    lineHeight = (10.5f * uiScale * 1.4f).sp,
                    fontFamily = TerminalFont.chatFont,
                )
            }
            if (item.text.isNotBlank()) {
                Text(
                    item.text,
                    Modifier.padding(top = 4.dp),
                    color = if (item.toolStatus == "failure") Pa.Red else Pa.TextSecondary,
                    fontSize = (10.5f * uiScale).sp,
                    lineHeight = (10.5f * uiScale * 1.4f).sp,
                    fontFamily = TerminalFont.chatFont,
                )
            }
            if (item.toolArgs.isBlank() && item.text.isBlank()) {
                Text(
                    "等待结果…",
                    color = Pa.TextTertiary,
                    fontSize = (11 * uiScale).sp,
                    fontFamily = TerminalFont.chatFont,
                )
            }
        }
    }
}

/**
 * ④ 回合脚注:输出块末尾右对齐 `✓ 3.2s` / `✗`(comment 色 11sp)。
 * 文案由 [formatTurnFootnote] 构造,没有的数据不显示。
 */
@Composable
internal fun TerminalFootnote(
    label: String,
    modifier: Modifier = Modifier,
) {
    Text(
        label,
        modifier = modifier.fillMaxWidth(),
        textAlign = TextAlign.End,
        color = Pa.TextTertiary,
        fontSize = 11.sp,
        fontFamily = TerminalFont.chatFont,
    )
}

// ─────────────────────────────────────────────────────────────────────────────
// tmux 式状态条
// ─────────────────────────────────────────────────────────────────────────────

/**
 * tmux 式状态条:聊天页底部、输入条上方,28dp 高。
 * 左段 `[0:pa]`(Pa.Accent 底 + Bg 字反色)+ 会话标题;右段 连接状态 `● 已连接` /
 * `● 重连中` + 当前模型名 + `HH:MM`,段间 `│` Pa.Separator。
 * 数据全部来自现成状态(connState/线程标题/生效模型/设备时钟),不造。
 */
@Composable
internal fun TmuxStatusLine(
    title: String,
    connState: String,
    modelName: String,
    modifier: Modifier = Modifier,
) {
    // 设备时钟 30s 一跳:状态条分钟粒度,足够;只动一个 state,不牵连重组列表
    var clockMs by remember { mutableStateOf(System.currentTimeMillis()) }
    LaunchedEffect(Unit) {
        while (true) {
            delay(30_000)
            clockMs = System.currentTimeMillis()
        }
    }
    Row(
        modifier
            .fillMaxWidth()
            .height(28.dp)
            // 顶缘一条发丝分隔:状态条是"贴底仪表",与上方会话流断开一层
            .drawWithContent {
                drawRect(Pa.Separator, topLeft = Offset.Zero, size = Size(size.width, 1.dp.toPx()))
                drawContent()
            }
            .padding(horizontal = 16.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box(
            Modifier
                .background(Pa.Accent, RoundedCornerShape(3.dp))
                .padding(horizontal = 6.dp, vertical = 1.dp),
        ) {
            Text(
                "[0:pa]",
                color = Pa.Bg,
                fontSize = 10.sp,
                fontWeight = FontWeight.SemiBold,
                fontFamily = TerminalFont.chatFont,
            )
        }
        Spacer(Modifier.width(8.dp))
        Text(
            title.ifBlank { "任务" },
            Modifier.weight(1f),
            color = Pa.TextTertiary,
            fontSize = 10.5.sp,
            fontFamily = TerminalFont.chatFont,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
        Text(
            "● ${tmuxConnLabel(connState)}",
            color = if (connState == "ok") Pa.Green else Pa.Yellow,
            fontSize = 10.5.sp,
            fontFamily = TerminalFont.chatFont,
        )
        if (modelName.isNotBlank()) {
            TmuxSegmentDivider()
            Text(
                modelName,
                color = Pa.TextSecondary,
                fontSize = 10.5.sp,
                fontFamily = TerminalFont.chatFont,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
        }
        TmuxSegmentDivider()
        Text(
            tmuxClockLabel(clockMs),
            color = Pa.TextTertiary,
            fontSize = 10.5.sp,
            fontFamily = TerminalFont.chatFont,
        )
    }
}

/** 段间 `│` 分隔(Pa.Separator,与竖条同源,层级靠结构)。 */
@Composable
private fun TmuxSegmentDivider() {
    Text(
        "│",
        Modifier.padding(horizontal = 6.dp),
        color = Pa.Separator,
        fontSize = 10.5.sp,
        fontFamily = TerminalFont.chatFont,
    )
}

// ─────────────────────────────────────────────────────────────────────────────
// 扫描线(克制:2dp 周期 1dp 暗线,alpha 4%,随 hacker 模式开关)
// ─────────────────────────────────────────────────────────────────────────────

/** 扫描线 alpha 上限(调研规格 ≤4%)。 */
internal const val SCANLINE_ALPHA = 0.04f

/**
 * 会话内容层的 CRT 扫描线:先画内容再叠 2dp 周期的 1dp 暗线(只碰绘制层,
 * 不触发布局)。仅在 hacker 分支由 ChatScreen 挂到消息列表,常规主题零影响。
 */
internal fun Modifier.terminalScanlines(): Modifier = drawWithContent {
    drawContent()
    val step = 2.dp.toPx()
    val line = 1.dp.toPx()
    val paint = Color.Black.copy(alpha = SCANLINE_ALPHA)
    var y = 0f
    while (y < size.height) {
        drawRect(paint, topLeft = Offset(0f, y), size = Size(size.width, line))
        y += step
    }
}
