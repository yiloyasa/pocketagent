package dev.pocketagent.app.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.LinkAnnotation
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.TextLinkStyles
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextDecoration
import androidx.compose.ui.text.withLink
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.R
import dev.pocketagent.app.ui.theme.Pa

enum class MarkdownLineKind { PARAGRAPH, HEADING, BULLET, CODE }

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

fun parseMarkdownForDisplay(source: String): List<MarkdownLine> {
    val result = mutableListOf<MarkdownLine>()
    var inFence = false
    source.replace("\r\n", "\n").split('\n').forEach { raw ->
        if (raw.trimStart().startsWith("```")) {
            inFence = !inFence
            return@forEach
        }
        if (inFence) {
            result += MarkdownLine(MarkdownLineKind.CODE, raw, listOf(MarkdownSpan(raw, MarkdownSpanKind.INLINE_CODE)))
            return@forEach
        }
        val trimmed = raw.trimStart()
        val heading = Regex("^#{1,6}\\s+(.*)$").find(trimmed)
        val bullet = Regex("^(?:[-*+]\\s+|\\d+[.)]\\s+)(.*)$").find(trimmed)
        when {
            heading != null -> {
                val text = stripInlineMarkdown(heading.groupValues[1])
                result += MarkdownLine(MarkdownLineKind.HEADING, text, inlineSpans(heading.groupValues[1]))
            }
            bullet != null -> {
                val text = stripInlineMarkdown(bullet.groupValues[1])
                result += MarkdownLine(MarkdownLineKind.BULLET, text, inlineSpans(bullet.groupValues[1]))
            }
            else -> result += MarkdownLine(MarkdownLineKind.PARAGRAPH, stripInlineMarkdown(raw), inlineSpans(raw))
        }
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

private fun MarkdownLine.annotated(): AnnotatedString = buildAnnotatedString {
    if (kind == MarkdownLineKind.BULLET) append("• ")
    spans.forEach { span ->
        when (span.kind) {
            MarkdownSpanKind.BOLD -> withStyle(SpanStyle(fontWeight = FontWeight.Bold)) { append(span.text) }
            MarkdownSpanKind.INLINE_CODE -> withStyle(SpanStyle(fontFamily = FontFamily.Monospace, color = Pa.Accent)) { append(span.text) }
            // 链接走 LinkAnnotation:点击由 LocalUriHandler 处理 —— 聊天页会把它接到内置浏览器上
            MarkdownSpanKind.LINK -> withLink(
                LinkAnnotation.Url(
                    span.url,
                    TextLinkStyles(style = SpanStyle(color = Pa.Accent, textDecoration = TextDecoration.Underline)),
                ),
            ) { append(span.text) }
            MarkdownSpanKind.TEXT -> append(span.text)
        }
    }
}

@Composable
fun MarkdownText(source: String, modifier: Modifier = Modifier, fontSize: Float = 15f) {
    // 【为什么按"块"而不是按"行"渲染】原来这里是 parseMarkdownForDisplay(...).forEach { Text(...) },
    // 每一行代码各自画一个带圆角的背景。后果有三个,真机上都看得到:
    //   1) 相邻行的圆角互相咬合,左右边缘漏出页底纯黑 → 视觉上是"横竖缝隙";
    //   2) 每行上下各 10dp padding,代码块行距被翻倍,ASCII 图(那些 │ ─ ┌ 组成的
    //      架构图)纵向对不齐,看起来像"设计坏了";
    //   3) 没有横向滚动 + 默认软换行 → 长行被硬折,图直接散架。
    // 现在把连续代码行聚合成**一个**整块,统一背景/圆角/内边距,并允许横向滚动。
    // 渲染前清洗:把可能在部分机型掉字形的少数符号(如 §)替换为友好形式,
    // 作为字体回退之外的兜底(见 sanitizeDisplayText)。
    val blocks = remember(source) { groupMarkdownBlocks(parseMarkdownForDisplay(sanitizeDisplayText(source))) }
    Column(modifier) {
        blocks.forEach { block ->
            when (block) {
                is MdBlock.Code -> MarkdownCodeBlock(block.text, fontSize)
                is MdBlock.Line -> MarkdownTextLine(block.line, fontSize)
            }
        }
    }
}

@Composable
private fun MarkdownTextLine(line: MarkdownLine, fontSize: Float) {
    Text(
        text = line.annotated(),
        modifier = Modifier.fillMaxWidth(),
        color = Pa.TextPrimary,
        fontSize = (if (line.kind == MarkdownLineKind.HEADING) fontSize + 2 else fontSize).sp,
        fontWeight = if (line.kind == MarkdownLineKind.HEADING) FontWeight.Bold else FontWeight.Normal,
        lineHeight = (fontSize * 1.55f).sp,
    )
}

@Composable
private fun MarkdownCodeBlock(code: String, fontSize: Float) {
    // 等宽 + 比页底抬高一档的背景 + 细描边,读起来是"一块代码面板"而不是"凹进去的黑洞"。
    // 代码块内容不走 inline span 着色 —— 那会把整段代码染成强调蓝(旧实现的问题之一)。
    val scroll = rememberScrollState()
    Box(
        Modifier
            .fillMaxWidth()
            .padding(vertical = 4.dp)
            .clip(RoundedCornerShape(12.dp))
            .background(Pa.Card)
            .border(0.5.dp, Pa.Separator, RoundedCornerShape(12.dp))
            // 先裁形状再横滚,滚动内容不会溢出圆角
            .horizontalScroll(scroll),
    ) {
        Text(
            text = code,
            modifier = Modifier.padding(horizontal = 12.dp, vertical = 10.dp),
            fontFamily = FontFamily.Monospace,
            // 代码比正文小半号更接近"终端感",同时保证不低于 9sp 以免不可读
            fontSize = (fontSize - 1.5f).coerceAtLeast(9f).sp,
            color = Pa.TextSecondary,
            lineHeight = (fontSize * 1.4f).sp,
            // 关键:不折行,交给横向滚动 —— ASCII 图与长命令行靠这个保住对齐
            softWrap = false,
        )
    }
}

/** Markdown 渲染块:要么是一个整块代码,要么是普通一行。 */
sealed interface MdBlock {
    /** 连续的代码行已合并,text 内部用 '\n' 分隔。 */
    data class Code(val text: String) : MdBlock
    data class Line(val line: MarkdownLine) : MdBlock
}

/**
 * 把逐行解析结果聚合成渲染块 —— 连续 [MarkdownLineKind.CODE] 合并成单个 [MdBlock.Code]。
 *
 * 抽成纯函数是为了能单测:块边界(代码块开头/结尾/多段代码块/空代码块)是这类
 * 渲染最容易出错的边界,不应该只能靠肉眼在真机上看。
 */
fun groupMarkdownBlocks(lines: List<MarkdownLine>): List<MdBlock> {
    val out = mutableListOf<MdBlock>()
    var i = 0
    while (i < lines.size) {
        if (lines[i].kind != MarkdownLineKind.CODE) {
            out += MdBlock.Line(lines[i])
            i++
            continue
        }
        val buf = StringBuilder()
        while (i < lines.size && lines[i].kind == MarkdownLineKind.CODE) {
            if (buf.isNotEmpty()) buf.append('\n')
            buf.append(lines[i].text)
            i++
        }
        out += MdBlock.Code(buf.toString())
    }
    return out
}
