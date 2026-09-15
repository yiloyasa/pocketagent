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
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.R
import dev.pocketagent.app.ui.theme.Pa

enum class MarkdownLineKind { PARAGRAPH, HEADING, BULLET, CODE }

enum class MarkdownSpanKind { TEXT, BOLD, INLINE_CODE }

data class MarkdownSpan(val text: String, val kind: MarkdownSpanKind = MarkdownSpanKind.TEXT)

data class MarkdownLine(
    val kind: MarkdownLineKind,
    val text: String,
    val spans: List<MarkdownSpan> = listOf(MarkdownSpan(text)),
)

private val boldPattern = Regex("\\*\\*(.+?)\\*\\*|__(.+?)__")
private val inlineCodePattern = Regex("`([^`]+)`")

private fun inlineSpans(source: String): List<MarkdownSpan> {
    val markers = (boldPattern.findAll(source).map { it.range.first to it.range.last }
        .toList() + inlineCodePattern.findAll(source).map { it.range.first to it.range.last }.toList())
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
