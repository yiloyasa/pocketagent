package dev.pocketagent.app.ui

import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.widget.Toast
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.MutableTransitionState
import androidx.compose.animation.fadeIn
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.horizontalScroll
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
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.LinkAnnotation
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.TextLinkStyles
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextDecoration
import androidx.compose.ui.text.withLink
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.ui.anim.PaMotion
import dev.pocketagent.app.ui.anim.pressable
import dev.pocketagent.app.ui.anim.rememberReduceMotion
import dev.pocketagent.app.ui.theme.PaEditorial
import dev.pocketagent.app.ui.theme.PaHacker
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

enum class MarkdownLineKind { PARAGRAPH, HEADING, BULLET, CODE, QUOTE, TABLE }

enum class MarkdownSpanKind { TEXT, BOLD, INLINE_CODE, LINK }

data class MarkdownSpan(
    val text: String,
    val kind: MarkdownSpanKind = MarkdownSpanKind.TEXT,
    /** kind=LINK 时的目标地址(裸 URL 与 [文本](URL) 都会填上)。 */
    val url: String = "",
)

data class MarkdownLine(
    val kind: MarkdownLineKind,
    val text: String,
    val spans: List<MarkdownSpan> = listOf(MarkdownSpan(text)),
    /** BULLET 的嵌套层级(前导空白每 2 字符一级,封顶 4);其余行恒 0。 */
    val indent: Int = 0,
    /** CODE 行所属围栏的语言标注(如 "python");裸围栏 / 非代码行为空串。 */
    val lang: String = "",
    /** TABLE 行的单元格(每格已解析为行内 spans);非表格行为空表。 */
    val cells: List<List<MarkdownSpan>> = emptyList(),
)

private val boldPattern = Regex("\\*\\*(.+?)\\*\\*|__(.+?)__")
private val inlineCodePattern = Regex("`([^`]+)`")

/** [文本](地址) 形式;地址里允许出现空格以外的常见字符。 */
private val mdLinkPattern = Regex("\\[([^]]+)]\\((\\S+?)\\)")

/** 裸 URL(http/https)。 */
private val bareUrlPattern = Regex("https?://[^\\s)）\\]】,，。;；]+")

/**
 * 电脑上的文件路径:会话里 agent 常直接给路径(如 盘符冒号反斜杠 + 目录 + index.html)。
 * 只认"盘符 + 反斜杠"或"Unix 绝对路径 + 带扩展名",避免把 a/b 这种普通文本当路径。
 */
private val pcPathPattern = Regex(
    """[A-Za-z]:\\[^\s"'<>|?*`]+\.[A-Za-z0-9]{1,6}""" +
        """|/(?:home|Users|opt|var|tmp|mnt|srv)/[^\s"'<>|?*`]+\.[A-Za-z0-9]{1,6}""",
)

/** 从正文里抽出电脑文件路径(去重,保持出现顺序)。 */
fun extractPcPaths(text: String): List<String> =
    pcPathPattern.findAll(text)
        .map { it.value.trimEnd('.', ',', '。', '，', ')', '）') }
        .distinct()
        .toList()

private fun inlineSpans(source: String): List<MarkdownSpan> {
    val linkRanges = mdLinkPattern.findAll(source).map { it.range.first to it.range.last }.toList()
    val bareRanges = bareUrlPattern.findAll(source)
        // 被 [文本](URL) 覆盖的裸 URL 不重复处理
        .filter { m -> linkRanges.none { r -> m.range.first >= r.first && m.range.last <= r.second } }
        .map { it.range.first to it.range.last }
        .toList()
    val pcRanges = pcPathPattern.findAll(source)
        .filter { m -> bareRanges.none { r -> m.range.first in r.first..r.second } }
        .map { it.range.first to it.range.last }
        .toList()
    val linkLike = linkRanges + bareRanges + pcRanges
    val markers = (boldPattern.findAll(source).map { it.range.first to it.range.last }
        .toList() + inlineCodePattern.findAll(source).map { it.range.first to it.range.last }.toList() + linkLike)
        .sortedBy { it.first }
    if (markers.isEmpty()) return listOf(MarkdownSpan(source))

    val spans = mutableListOf<MarkdownSpan>()
    var cursor = 0
    for ((start, end) in markers) {
        if (start < cursor) continue
        if (start > cursor) spans += MarkdownSpan(source.substring(cursor, start))
        val token = source.substring(start, end + 1)
        when {
            token.startsWith("**") || token.startsWith("__") -> {
                spans += MarkdownSpan(token.drop(2).dropLast(2), MarkdownSpanKind.BOLD)
            }
            token.startsWith("`") -> spans += MarkdownSpan(token.drop(1).dropLast(1), MarkdownSpanKind.INLINE_CODE)
            token.startsWith("[") -> {
                val m = mdLinkPattern.find(token)
                if (m != null) {
                    spans += MarkdownSpan(m.groupValues[1], MarkdownSpanKind.LINK, m.groupValues[2])
                } else {
                    spans += MarkdownSpan(token)
                }
            }
            token.startsWith("/") || Regex("^[A-Za-z]:").containsMatchIn(token) ->
                // 电脑文件路径:交给内置浏览器以 pc: 模式读取渲染
                spans += MarkdownSpan(token, MarkdownSpanKind.LINK, "pc:" + token)
            else -> spans += MarkdownSpan(token, MarkdownSpanKind.LINK, token)
        }
        cursor = end + 1
    }
    if (cursor < source.length) spans += MarkdownSpan(source.substring(cursor))
    return spans.ifEmpty { listOf(MarkdownSpan("")) }
}

// ─────────────────────────────────────────────────────────────────────────────
// 解析层纯函数(逐行/逐块映射,JVM 可单测 —— 渲染质量的地基钉在这里)
// ─────────────────────────────────────────────────────────────────────────────

private val headingPattern = Regex("^#{1,6}\\s+(.*)$")
private val bulletPattern = Regex("^(?:[-*+]\\s+|\\d+[.)]\\s+)(.*)$")

/** 分隔行单元格 `---` / `:---:` / `---:`。 */
private val tableSepCellPattern = Regex(":?-+:?")

/** 单元格内的转义竖线占位符(拆列前替换,拆完还原)。 */
private const val ESCAPED_PIPE = '\u0000'

/**
 * 围栏开口行的语言标注:` ```python ` → "python";裸 ` ``` ` → "";
 * ` ```kotlin title=Main.kt ` 这类带元信息的取第一个空格前的词。解析层不丢语言,
 * 渲染层据此在代码块右上角标注(调研结论 1)。
 */
fun fenceLanguage(raw: String): String =
    raw.trimStart().removePrefix("```").trim().substringBefore(' ').trim()

/**
 * 列表嵌套层级:前导空白每 2 字符一级,封顶 4(渲染端按层级缩进)。
 * 只看原始行的前导空白,标记符(`-`/`*`/`1.`)与内容层级无关。
 */
fun bulletIndentLevel(raw: String): Int =
    ((raw.length - raw.trimStart().length) / 2).coerceAtMost(4)

/**
 * 一行表格文本 → 单元格文本列表:剥首尾 `|` 后按 `|` 拆列,格子 trim;
 * `\|` 是转义竖线(内容里的竖线不拆列)。空行拆出单个空串。
 */
fun splitTableRow(line: String): List<String> {
    var t = line.replace("\\|", ESCAPED_PIPE.toString()).trim()
    if (t.startsWith("|")) t = t.substring(1)
    if (t.endsWith("|")) t = t.dropLast(1)
    return t.split('|').map { it.replace(ESCAPED_PIPE.toString(), "|").trim() }
}

/**
 * 表格分隔行判定(`| --- | :---: |`):拆出的每格都只由 `-`/`:` 组成且至少有一个 `-`。
 * 无 `|` 的水平线(`---`)不算 —— 那是分隔线,不是表格(防误吞段落)。
 */
fun isTableSeparatorRow(line: String): Boolean {
    val t = line.trim()
    if (!t.contains('-')) return false
    if (!t.contains('|')) return false
    val cells = splitTableRow(line)
    return cells.isNotEmpty() && cells.all { tableSepCellPattern.matches(it) }
}

/** 表格行 → 定长单元格 spans(不足补空、超出截断到 [cols]),格内继续走行内 span 解析。 */
fun tableRowCells(line: String, cols: Int): List<List<MarkdownSpan>> {
    val texts = splitTableRow(line)
    return List(cols) { i -> inlineSpans(texts.getOrElse(i) { "" }) }
}

/** 表格列宽自适应的字符数上下限(超限内容换行,防一列把表撑到天宽)。 */
internal const val TABLE_COL_MIN_CHARS = 4f
internal const val TABLE_COL_MAX_CHARS = 24f

/**
 * 表格列宽配比:按各列最大文本长度归一到和为 1 的权重,单列占比封顶在
 * [TABLE_COL_MIN_CHARS]..[TABLE_COL_MAX_CHARS] 字符档(渲染端 Row weight 用)。
 * 纯函数:渲染层把表头+数据行拍平成字符串传入,测试直接钉归一与封顶。
 */
fun tableColumnWeights(rows: List<List<String>>): List<Float> {
    if (rows.isEmpty()) return emptyList()
    val n = rows.maxOf { it.size }
    if (n == 0) return emptyList()
    val raw = IntArray(n)
    for (row in rows) for (i in row.indices) {
        val len = row[i].length.coerceAtLeast(1)
        if (len > raw[i]) raw[i] = len
    }
    val capped = raw.map { it.toFloat().coerceIn(TABLE_COL_MIN_CHARS, TABLE_COL_MAX_CHARS) }
    val sum = capped.sum()
    if (sum <= 0f) return List(n) { 1f / n }
    return capped.map { it / sum }
}

fun parseMarkdownForDisplay(source: String): List<MarkdownLine> {
    val result = mutableListOf<MarkdownLine>()
    val lines = source.replace("\r\n", "\n").split('\n')
    var inFence = false
    var fenceLang = ""
    var i = 0
    while (i < lines.size) {
        val raw = lines[i]
        if (raw.trimStart().startsWith("```")) {
            // 开围栏记语言,闭围栏清掉:语言只属于这一个代码块
            fenceLang = if (inFence) "" else fenceLanguage(raw)
            inFence = !inFence
            i++
            continue
        }
        if (inFence) {
            result += MarkdownLine(
                MarkdownLineKind.CODE, raw,
                listOf(MarkdownSpan(raw, MarkdownSpanKind.INLINE_CODE)),
                lang = fenceLang,
            )
            i++
            continue
        }
        val trimmed = raw.trimStart()
        val heading = headingPattern.find(trimmed)
        when {
            heading != null -> {
                val text = stripInlineMarkdown(heading.groupValues[1])
                result += MarkdownLine(MarkdownLineKind.HEADING, text, inlineSpans(heading.groupValues[1]))
            }
            // 表格:当前行含 '|' 且下一行是分隔行才成立(无分隔行的竖线是普通段落);
            // 随后连续含 '|' 的非空行都归入表体,空行终止(GFM 语义)。
            trimmed.contains('|') && i + 1 < lines.size && isTableSeparatorRow(lines[i + 1]) -> {
                val cols = splitTableRow(lines[i + 1]).size
                result += MarkdownLine(MarkdownLineKind.TABLE, "", cells = tableRowCells(trimmed, cols))
                i += 2
                while (i < lines.size) {
                    val t = lines[i].trim()
                    if (t.isEmpty() || !t.contains('|') || t.startsWith("```")) break
                    result += MarkdownLine(MarkdownLineKind.TABLE, "", cells = tableRowCells(lines[i], cols))
                    i++
                }
                continue
            }
            bulletPattern.find(trimmed) != null -> {
                val content = bulletPattern.find(trimmed)!!.groupValues[1]
                val text = stripInlineMarkdown(content)
                result += MarkdownLine(
                    MarkdownLineKind.BULLET, text, inlineSpans(content),
                    indent = bulletIndentLevel(raw),
                )
            }
            trimmed.startsWith(">") -> {
                // 引用块:剥一层 '>' 与其后空白,嵌套 '> >' 逐层剥到内容
                val content = trimmed.dropWhile { it == '>' }.trimStart()
                val text = stripInlineMarkdown(content)
                result += MarkdownLine(MarkdownLineKind.QUOTE, text, inlineSpans(content))
            }
            else -> result += MarkdownLine(MarkdownLineKind.PARAGRAPH, stripInlineMarkdown(raw), inlineSpans(raw))
        }
        i++
    }
    return result
}

fun stripInlineMarkdown(source: String): String = source
    .replace(boldPattern) { it.groupValues[1].ifEmpty { it.groupValues[2] } }
    .replace(inlineCodePattern) { it.groupValues[1] }
    .replace(Regex("\\[([^]]+)]\\([^)]*\\)"), "$1")

/**
 * 渲染前清洗:把少量在目标设备字体里取不到字形(显示为豆腐块 / 错误字形)的字符,
 * 替换成等义、一定能渲染的安全形式。
 *
 * 【为什么需要】MarkdownText 正文走 Material 默认字体(Roboto / 设备默认),绝大多数字符
 * 都能正常渲染,且缺失字形会自动回退到系统字体;但 §(U+00A7 SECTION SIGN)等极少数符号在
 * 个别机型 / 字重下仍可能掉成豆腐块。这里**不粗暴 strip 全盘特殊字符**(那会把
 * `|§9 连接可靠性|` 这类表格行的语义拆坏),只做"异常字符 → 友好替代"的定点替换,
 * 作为字体回退之外的兜底。
 *
 * 【如何扩展】往 [DISPLAY_TEXT_REPLACEMENTS] 加一行即可(键支持多字符序列,值是替代文本),
 * 无需改任何调用点。§N 这类带序号的走下方正则规则,要换别的友好形式只改一处。
 */
private val DISPLAY_TEXT_REPLACEMENTS: Map<String, String> = mapOf(
    // 默认不替换任何字符:§ 等一般能靠平台字体回退正常渲染。
    // 若某机型实测把 § 渲染成豆腐块,在此加一行覆盖即可,例如:
    //   "§" to "§",            // 仍保留原字符(确认回退生效)
    //   "§9" to "第 9 节",      // 或按语义替换。新增条目无需改调用点。
)

/** `§数字`(可带小数点,如 §9 / §7.2 / §10)→ `第 数字 节`:保留语义且任何字体都能渲染。 */
private val SECTION_SIGN_RE = Regex("§\\s*(\\d[\\d.]*)")

/**
 * 见 [DISPLAY_TEXT_REPLACEMENTS]。返回清洗后的文本;原文本无异常字符时原样返回。
 * 字体回退优先(不改动内容),清洗表仅作兜底,绝不整段 strip 特殊字符。
 */
fun sanitizeDisplayText(text: String): String {
    var out = text
    for ((bad, good) in DISPLAY_TEXT_REPLACEMENTS) {
        if (bad in out) out = out.replace(bad, good)
    }
    // §数字 → 第 数字 节:§ 在个别机型掉字形时的兜底(原字符能渲染时等同友好改写)。
    out = SECTION_SIGN_RE.replace(out) { "第 ${it.groupValues[1]} 节" }
    return out
}

// ─────────────────────────────────────────────────────────────────────────────
// 渲染层(spans → AnnotatedString 与各类块)
// ─────────────────────────────────────────────────────────────────────────────

/** 行内元素 → AnnotatedString;[bullet] 为 true 时行首补列表符。 */
fun spansToAnnotated(spans: List<MarkdownSpan>, bullet: Boolean = false): AnnotatedString = buildAnnotatedString {
    if (bullet) append("• ")
    spans.forEach { span ->
        when (span.kind) {
            MarkdownSpanKind.BOLD -> withStyle(SpanStyle(fontWeight = FontWeight.Bold)) { append(span.text) }
            // 行内代码:等宽 + 底色芯片(调研结论 3:底色区分,不只靠颜色)
            MarkdownSpanKind.INLINE_CODE -> withStyle(
                SpanStyle(
                    fontFamily = PaEditorial.MonoFont,
                    color = PaEditorial.Primary,
                    background = PaEditorial.Raised,
                ),
            ) { append(span.text) }
            // 链接走 LinkAnnotation:点击由 LocalUriHandler 处理 —— 聊天页会把它接到内置浏览器上
            MarkdownSpanKind.LINK -> withLink(
                LinkAnnotation.Url(
                    span.url,
                    TextLinkStyles(style = SpanStyle(color = PaEditorial.Primary, textDecoration = TextDecoration.Underline)),
                ),
            ) { append(span.text) }
            MarkdownSpanKind.TEXT -> append(span.text)
        }
    }
}

private fun MarkdownLine.annotated(): AnnotatedString =
    spansToAnnotated(spans, bullet = kind == MarkdownLineKind.BULLET)

/**
 * 代码块复制:与消息长按复制(ChatScreen.copyMessage)同一套剪贴板语义 ——
 * ClipboardManager.setPrimaryClip + Toast「已复制」回执,成功才提示。
 */
internal fun copyCodeToClipboard(ctx: Context, code: String) {
    if (code.isBlank()) return
    val cm = ctx.getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager
    cm.setPrimaryClip(ClipData.newPlainText("PocketAgent 代码", code))
    Toast.makeText(ctx, "已复制", Toast.LENGTH_SHORT).show()
}

@Composable
fun MarkdownText(
    source: String,
    modifier: Modifier = Modifier,
    fontSize: Float = 15f,
    /** 流式渲染(2026-10-09 精修波 D4-1):新长出来的块 140ms 级 fade-in;静止正文不受影响。 */
    streaming: Boolean = false,
) {
    // 【为什么按"块"而不是按"行"渲染】原来这里是 parseMarkdownForDisplay(...).forEach { Text(...) },
    // 每一行代码各自画一个带圆角的背景。后果有三个,真机上都看得到:
    //   1) 相邻行的圆角互相咬合,左右边缘漏出页底纯黑 → 视觉上是"横竖缝隙";
    //   2) 每行上下各 10dp padding,代码块行距被翻倍,ASCII 图(那些 │ ─ ┌ 组成的
    //      架构图)纵向对不齐,看起来像"设计坏了";
    //   3) 没有横向滚动 + 默认软换行 → 长行被硬折,图直接散架。
    // 现在把连续代码行聚合成**一个**整块,统一背景/圆角/内边距,并允许横向滚动。
    // 渲染前清洗:把可能在部分机型掉字形的少数符号(如 §)替换为友好形式,
    // 作为字体回退之外的兜底(见 sanitizeDisplayText)。
    //
    // 【性能纪律】parse 与 compose 分离,且解析异步化(审查 P1-22):
    //   1) 解析+聚合是重 CPU 纯函数,放 produceState 里在 Dispatchers.Default 上跑,
    //      主线程只消费结果列表 —— 不再阻塞合成帧;
    //   2) key=source:流式追加时 source 每帧变化,produceState 会取消上一帧的解析
    //      协程、只为最新内容解析一次(withContext 在协程被取消时不再把过期结果
    //      写回 value),天然合并中间帧,消除"每个 delta 帧全文重解析"的 O(n²);
    //   3) 解析函数本身仍是纯函数(可 JVM 单测),此处只挪执行线程,不改语义。
    val blocks by produceState(emptyList<MdBlock>(), source) {
        value = withContext(Dispatchers.Default) {
            groupMarkdownBlocks(parseMarkdownForDisplay(sanitizeDisplayText(source)))
        }
    }
    // D4-1 流式新行 fade-in:只在流式路径 + 未开"减弱动效"时启用。
    // 方案选型见 MarkdownStreamingFade 的注释 —— 不拆 source、不加第二份解析,
    // 120ms 节流管线(displayedStream)原样复用。
    val lineFade = streaming && !rememberReduceMotion()
    Column(modifier) {
        blocks.forEach { block ->
            val isBlankSpacer = block is MdBlock.Line &&
                block.line.kind == MarkdownLineKind.PARAGRAPH &&
                block.line.text.isBlank()
            if (lineFade && !isBlankSpacer) {
                MarkdownStreamingFade { RenderMdBlock(block, fontSize) }
            } else {
                RenderMdBlock(block, fontSize)
            }
        }
    }
}

/**
 * 流式块级 fade-in(D4-1):新长出来的块在首组合时播一次 ~140ms 级淡入。
 *
 * 【为什么用 MutableTransitionState 而不是 visible=true】AnimatedVisibility 以
 * visible=true 首组合会直接呈现(不播进场);MutableTransitionState(false→true)
 * 强制从不可见起播。配合 forEach 的**位置记忆** remember:同位置复用的旧块状态
 * 已是 true 不重播,只有"列表末尾新长出来的块"拿到全新状态 —— 正是
 * "新文本行 fade-in"的语义,已存在的内容纹丝不动。
 *
 * 【为什么不 per-chunk 拆两个 MarkdownText】拆"旧段+新增段"两份解析会把
 * P1-22 合并的解析量翻倍,且代码围栏/表格跨段时边界渲染会闪错;单份解析 +
 * 块级淡入零额外解析,节流管线零改动。
 *
 * 【时长口径】任务口径 140ms:走 PaMotion.press() 弹簧(stiffness 520 临界阻尼,
 * settle≈175ms,无过冲)—— 全 app 动效令牌纪律禁裸 tween,140ms 量级里 press
 * 档是最接近的令牌,差别在不可感知区间。
 */
@Composable
private fun MarkdownStreamingFade(content: @Composable () -> Unit) {
    val appear = remember { MutableTransitionState(false).apply { targetState = true } }
    AnimatedVisibility(
        visibleState = appear,
        // 只淡入不位移:纯 fade 不触发布局跳变(下方跟随的三点位置稳定)
        enter = fadeIn(PaMotion.press()),
    ) { content() }
}

/** 单个 Markdown 渲染块的分发(MarkdownText 主体与流式淡入共用一份)。 */
@Composable
private fun RenderMdBlock(block: MdBlock, fontSize: Float) {
    when (block) {
        is MdBlock.Code -> MarkdownCodeBlock(block.text, block.lang, fontSize)
        is MdBlock.Table -> MarkdownTableBlock(block.header, block.rows, fontSize)
        is MdBlock.Quote -> MarkdownQuoteBlock(block.lines, fontSize)
        is MdBlock.Line ->
            // 空行 = 段落间的呼吸:固定 6dp 节奏(旧实现画一行空文字,高度吃满整行行距)
            if (block.line.kind == MarkdownLineKind.PARAGRAPH && block.line.text.isBlank()) {
                Spacer(Modifier.height(6.dp))
            } else {
                MarkdownTextLine(block.line, fontSize)
            }
    }
}

@Composable
private fun MarkdownTextLine(line: MarkdownLine, fontSize: Float, color: Color = PaEditorial.Ink) {
    // 【P1-6】AnnotatedString 按 line 缓存:旧实现在每次重组都 line.annotated() 逐行
    // 重建(spans→AnnotatedString 的 buildAnnotatedString 全量重跑)。MarkdownLine 是
    // data class,equals 为结构比较(spans 列表逐元素比较),内容不变即命中缓存。
    // 颜色等盘面状态参与 key:三盘换肤(IsDark/黑客盘切换)时强制重建,
    // 与 MarkdownCodeBlock「remember 键含 colors」的换肤即时重映色纪律一致。
    val annotated = remember(line, PaEditorial.IsDark, PaHacker.isHacker) { line.annotated() }
    Text(
        text = annotated,
        // 黑客终端模式:正文行整体等宽(代码块本来就是等宽,普通行随之统一)。
        // 字体走 theme 层令牌 PaEditorial.MonoFont(JetBrains Mono,下载落库 res/font;
        // 等宽统一 2026-10-09)。
        fontFamily = if (PaHacker.isHacker) PaEditorial.MonoFont else null,
        // 嵌套列表:按解析层给出的层级缩进(每级 14dp),层级一眼可辨
        modifier = Modifier
            .fillMaxWidth()
            .padding(start = (line.indent * 14).dp),
        color = color,
        fontSize = (if (line.kind == MarkdownLineKind.HEADING) fontSize + 2 else fontSize).sp,
        fontWeight = if (line.kind == MarkdownLineKind.HEADING) FontWeight.Bold else FontWeight.Normal,
        lineHeight = (fontSize * 1.55f).sp,
    )
}

@Composable
private fun MarkdownCodeBlock(code: String, lang: String, fontSize: Float) {
    // 等宽 + 比页底抬高一档的背景 + 细描边,读起来是"一块代码面板"而不是"凹进去的黑洞"。
    // 代码块内容不走 inline span 着色 —— 那会把整段代码染成强调蓝(旧实现的问题之一)。
    // 【本轮对齐玻璃层级】底色 PaEditorial.Surface → PaEditorial.Raised(海拨再抬一档):
    // 助手正文直接铺在页底上,代码块是与正文并列的"面板",材质上要比正文
    // 高一级才读得出块边界;代码文字用 TextPrimary:深色实色底上二级灰读代码费眼。
    // 【本轮新增头部条】右上角语言标注(解析层不再丢弃围栏语言)+ 复制按钮
    // (与消息长按复制同一剪贴板语义,调研结论 1)。
    // 【D1 语法着色 2026-10-09】代码正文从 Ink 单色升级为 keyword/string/comment/number
    // 四类轻量着色(CodeSyntax 纯函数,JVM 单测钉死);未知语言退单色,行为与旧版一致。
    // 着色走与 P1-22 同一条异步纪律:tokenize 是 O(n) 扫描,放 Default 线程解析,
    // 流式期间随 120ms 节流快照逐块重跑,不阻塞合成帧;色板在组合期读当前盘,
    // 三盘换肤即时重映色(remember 键含 colors)。
    val scroll = rememberScrollState()
    val ctx = LocalContext.current
    // 等宽统一(2026-10-09):代码块在全部色盘下都是同一 JetBrains Mono(令牌
    // PaEditorial.MonoFont),扫描线/等宽网格逐列对齐不再是 hacker 专属
    val mono = PaEditorial.MonoFont
    val tokens by produceState<List<CodeToken>?>(null, code, lang) {
        value = withContext(Dispatchers.Default) { tokenizeCode(code, lang) }
    }
    val colors = syntaxColorsForCurrentPalette()
    val annotated = remember(tokens, colors) {
        tokens?.let { syntaxTokensToAnnotated(it, colors) } ?: AnnotatedString(code)
    }
    Column(
        Modifier
            .fillMaxWidth()
            .padding(vertical = 4.dp)
            .clip(RoundedCornerShape(PaEditorial.Radius.IconButton))
            .background(PaEditorial.Raised)
            .border(0.5.dp, PaEditorial.Hairline, RoundedCornerShape(PaEditorial.Radius.IconButton)),
    ) {
        Row(
            Modifier
                .fillMaxWidth()
                .padding(start = 12.dp, end = 6.dp, top = 4.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Spacer(Modifier.weight(1f))
            if (lang.isNotBlank()) {
                Text(
                    lang,
                    fontSize = PaEditorial.Type.Micro.fontSize,
                    color = PaEditorial.Muted,
                    fontFamily = mono,
                )
                Spacer(Modifier.width(10.dp))
            }
            Text(
                "复制",
                Modifier
                    .pressable(haptic = true) { copyCodeToClipboard(ctx, code) }
                    // 10dp 竖向 padding:按钮热区约 36dp 高,紧凑但可点(触达)
                    .padding(horizontal = 10.dp, vertical = 10.dp),
                fontSize = PaEditorial.Type.Caption.fontSize,
                color = PaEditorial.Muted,
                fontFamily = mono,
            )
        }
        Box(
            Modifier
                // 先裁形状再横滚,滚动内容不会溢出圆角
                .fillMaxWidth()
                .horizontalScroll(scroll),
        ) {
            Text(
                text = annotated,
                modifier = Modifier.padding(start = 12.dp, end = 12.dp, bottom = 10.dp, top = 2.dp),
                fontFamily = mono,
                // 代码比正文小半号更接近"终端感",同时保证不低于 9sp 以免不可读
                fontSize = (fontSize - 1.5f).coerceAtLeast(9f).sp,
                color = PaEditorial.Ink,
                lineHeight = (fontSize * 1.4f).sp,
                // 关键:不折行,交给横向滚动 —— ASCII 图与长命令行靠这个保住对齐
                softWrap = false,
            )
        }
    }
}

@Composable
private fun MarkdownTableBlock(
    header: List<List<MarkdownSpan>>,
    rows: List<List<List<MarkdownSpan>>>,
    fontSize: Float,
) {
    // 表格(调研结论 2):列宽按内容自适应(IntrinsicSize.Max 取最宽行自然宽),
    // 超出屏宽交给横向滚动;表头加粗 + 抬一档底色区分;行间 0.5dp 发丝分隔线。
    // 权重由纯函数 tableColumnWeights 计算(单列封顶,长内容换行不撑爆)。
    val scroll = rememberScrollState()
    val weights = remember(header, rows) {
        tableColumnWeights((listOf(header) + rows).map { row -> row.map { cell -> cell.joinToString("") { it.text } } })
    }
    Box(
        Modifier
            .fillMaxWidth()
            .padding(vertical = 4.dp)
            .horizontalScroll(scroll),
    ) {
        Column(
            Modifier
                .width(IntrinsicSize.Max)
                .clip(RoundedCornerShape(PaEditorial.Radius.IconButton))
                .background(PaEditorial.Surface)
                .border(0.5.dp, PaEditorial.Hairline, RoundedCornerShape(PaEditorial.Radius.IconButton)),
        ) {
            MarkdownTableRow(header, weights, fontSize, header = true)
            rows.forEach { row ->
                Box(Modifier.fillMaxWidth().height(0.5.dp).background(PaEditorial.Hairline))
                MarkdownTableRow(row, weights, fontSize, header = false)
            }
        }
    }
}

@Composable
private fun MarkdownTableRow(
    cells: List<List<MarkdownSpan>>,
    weights: List<Float>,
    fontSize: Float,
    header: Boolean,
) {
    Row(
        Modifier
            .fillMaxWidth()
            .then(if (header) Modifier.background(PaEditorial.Raised) else Modifier),
    ) {
        cells.forEachIndexed { i, cell ->
            Box(Modifier.weight(if (i < weights.size) weights[i] else 1f)) {
                Text(
                    text = spansToAnnotated(cell),
                    modifier = Modifier.padding(horizontal = 10.dp, vertical = 7.dp),
                    color = PaEditorial.Ink,
                    fontSize = (fontSize - 1f).coerceAtLeast(9f).sp,
                    fontWeight = if (header) FontWeight.SemiBold else FontWeight.Normal,
                    lineHeight = (fontSize * 1.4f).sp,
                )
            }
        }
    }
}

@Composable
private fun MarkdownQuoteBlock(quotes: List<MarkdownLine>, fontSize: Float) {
    // 引用块(调研结论 4):左侧 3dp 竖线 + 正文降一档灰,连续 '>' 行并成一个块,
    // 竖线贯通整块(不是每行各画一截)。
    Row(
        Modifier
            .fillMaxWidth()
            .padding(vertical = 4.dp)
            .height(IntrinsicSize.Min),
    ) {
        Box(
            Modifier
                .width(3.dp)
                .fillMaxHeight()
                .background(PaEditorial.Muted.copy(alpha = 0.6f), RoundedCornerShape(PaEditorial.Radius.Terminal)),
        )
        Spacer(Modifier.width(10.dp))
        Column {
            quotes.forEach { MarkdownTextLine(it, fontSize, color = PaEditorial.Muted) }
        }
    }
}

/** Markdown 渲染块:整块代码 / 表格 / 引用 / 普通行。 */
sealed interface MdBlock {
    /** 连续的代码行已合并,text 内部用 '\n' 分隔;lang 为围栏语言标注(可为空)。 */
    data class Code(val text: String, val lang: String = "") : MdBlock

    /** 一个完整表格:header 为首行(表头),rows 为其后数据行;每行是单元格列表,单元格为行内 spans。 */
    data class Table(val header: List<List<MarkdownSpan>>, val rows: List<List<List<MarkdownSpan>>>) : MdBlock

    /** 连续引用行(已剥 '>'),并成一个左竖线块。 */
    data class Quote(val lines: List<MarkdownLine>) : MdBlock

    data class Line(val line: MarkdownLine) : MdBlock
}

/**
 * 把逐行解析结果聚合成渲染块 —— 连续 [MarkdownLineKind.CODE] 合并成单个 [MdBlock.Code],
 * 连续 [MarkdownLineKind.TABLE] 合并成单个 [MdBlock.Table](首行=表头),
 * 连续 [MarkdownLineKind.QUOTE] 合并成单个 [MdBlock.Quote]。
 *
 * 抽成纯函数是为了能单测:块边界(代码块开头/结尾/多段代码块/空代码块/表格起止/
 * 引用块边界)是这类渲染最容易出错的边界,不应该只能靠肉眼在真机上看。
 */
fun groupMarkdownBlocks(lines: List<MarkdownLine>): List<MdBlock> {
    val out = mutableListOf<MdBlock>()
    var i = 0
    while (i < lines.size) {
        val kind = lines[i].kind
        if (kind != MarkdownLineKind.CODE && kind != MarkdownLineKind.TABLE && kind != MarkdownLineKind.QUOTE) {
            out += MdBlock.Line(lines[i])
            i++
            continue
        }
        if (kind == MarkdownLineKind.CODE) {
            // 语言取自本块首行(开围栏时记下,闭围栏即清,不会串块)
            val lang = lines[i].lang
            val buf = StringBuilder()
            while (i < lines.size && lines[i].kind == MarkdownLineKind.CODE) {
                if (buf.isNotEmpty()) buf.append('\n')
                buf.append(lines[i].text)
                i++
            }
            out += MdBlock.Code(buf.toString(), lang)
            continue
        }
        if (kind == MarkdownLineKind.TABLE) {
            val header = lines[i].cells
            i++
            val body = mutableListOf<List<List<MarkdownSpan>>>()
            while (i < lines.size && lines[i].kind == MarkdownLineKind.TABLE) {
                body += lines[i].cells
                i++
            }
            out += MdBlock.Table(header, body)
            continue
        }
        val quotes = mutableListOf<MarkdownLine>()
        while (i < lines.size && lines[i].kind == MarkdownLineKind.QUOTE) {
            quotes += lines[i]
            i++
        }
        out += MdBlock.Quote(quotes)
    }
    return out
}
