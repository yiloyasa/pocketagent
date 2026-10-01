package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * R2-A 工具卡长输出渐进披露的纯函数(见 ChatScreen.kt「R2-A 工具卡长输出渐进披露」区):
 * 默认只渲染头尾预览(错误/结果多在末尾,尾部必须留),余量按真实字符数报在按钮上。
 */
class ChatToolOutputPreviewTest {

    @Test
    fun short_text_passes_through_untouched() {
        val p = toolOutputPreview("hello world")
        assertFalse(p.truncated)
        assertEquals(0, p.omittedChars)
        assertEquals("hello world", p.visible)
    }

    @Test
    fun empty_text_is_not_truncated() {
        val p = toolOutputPreview("")
        assertFalse(p.truncated)
        assertEquals("", p.visible)
    }

    @Test
    fun long_text_keeps_head_and_tail_with_mark_between() {
        val text = "H".repeat(3000) + "M".repeat(1000) + "T".repeat(500)
        val p = toolOutputPreview(text)
        assertTrue(p.truncated)
        assertEquals(4500 - 2000 - 400, p.omittedChars)
        assertEquals("H".repeat(2000) + TOOL_OUTPUT_PREVIEW_OMITTED_MARK + "T".repeat(400), p.visible)
    }

    @Test
    fun at_min_omitted_boundary_not_truncated() {
        // 隐藏量恰好等于下限:不值得多一次点击,原样全显
        val text = "x".repeat(2000 + 400 + 160)
        val p = toolOutputPreview(text)
        assertFalse(p.truncated)
        assertEquals(text, p.visible)
    }

    @Test
    fun one_char_over_min_omitted_truncates() {
        val text = "x".repeat(2000 + 400 + 161)
        val p = toolOutputPreview(text)
        assertTrue(p.truncated)
        assertEquals(161, p.omittedChars)
    }

    @Test
    fun custom_params_are_honored() {
        val text = "a".repeat(100)
        val p = toolOutputPreview(text, headChars = 10, tailChars = 10, minOmitted = 50)
        assertTrue(p.truncated)
        assertEquals(80, p.omittedChars)
        assertEquals("a".repeat(10) + TOOL_OUTPUT_PREVIEW_OMITTED_MARK + "a".repeat(10), p.visible)
    }

    @Test
    fun negative_params_are_coerced_to_zero() {
        val p = toolOutputPreview("abcdef", headChars = -3, tailChars = -1, minOmitted = 0)
        assertTrue(p.truncated)
        assertEquals(6, p.omittedChars)
        assertEquals(TOOL_OUTPUT_PREVIEW_OMITTED_MARK, p.visible)
    }

    @Test
    fun design_intent_head_retains_more_than_tail() {
        // 头部是命令回显/参数确认(密度高),尾部是结果/报错(必看但短):
        // 头保留量必须大于尾,否则长输出的开头全被吃掉
        assertTrue(TOOL_OUTPUT_PREVIEW_HEAD_CHARS > TOOL_OUTPUT_PREVIEW_TAIL_CHARS)
    }

    @Test
    fun omitted_mark_is_multiline_standalone() {
        // 等宽横滚代码箱里,占位提示独立成行,不与代码黏行
        assertTrue(TOOL_OUTPUT_PREVIEW_OMITTED_MARK.startsWith("\n"))
        assertTrue(TOOL_OUTPUT_PREVIEW_OMITTED_MARK.endsWith("\n"))
    }
}
