package dev.pocketagent.app.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
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
    Column(modifier) {
        parseMarkdownForDisplay(source).forEach { line ->
            val code = line.kind == MarkdownLineKind.CODE
            Text(
                text = line.annotated(),
                modifier = Modifier
                    .fillMaxWidth()
                    .then(if (code) Modifier.background(Pa.Card, RoundedCornerShape(10.dp)).padding(10.dp) else Modifier),
                color = if (code) Pa.TextSecondary else Pa.TextPrimary,
                fontSize = (if (line.kind == MarkdownLineKind.HEADING) fontSize + 2 else fontSize).sp,
                fontWeight = if (line.kind == MarkdownLineKind.HEADING) FontWeight.Bold else FontWeight.Normal,
                lineHeight = (fontSize * 1.55f).sp,
            )
        }
    }
}
