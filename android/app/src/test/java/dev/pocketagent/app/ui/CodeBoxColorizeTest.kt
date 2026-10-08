package dev.pocketagent.app.ui

import androidx.compose.ui.graphics.Color
import dev.pocketagent.app.ui.theme.AgentTheme
import dev.pocketagent.app.ui.theme.PaEditorial
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * D1/D5 代码箱着色纯函数的 JVM 钉子(回合 diff 卡 + 工具卡参数/输出代码箱):
 *  1. 判型([classifyCodeBox]):diff 形状(+/−/@@ 行占比 >60% 且 ≥3 非空行)
 *     优先于 JSON,再退单色 —— 两三行巧合的 +/- 不构成 diff 语境;
 *  2. 拼接守恒([codeBoxSpans]):着色片段按序拼接恒等于原文,一个字符不能丢;
 *  3. 取色([codeBoxColors]):+ → Running 系绿、− → Denied 系红、@@ → 哑色、
 *     JSON 键 → Mcp 紫,全部 PaEditorial 语义槽位;黑客盘自动降饱和(*Text 档)。
 */
class CodeBoxColorizeTest {

    @After
    fun restorePalette() {
        AgentTheme.mode = "dark"
        PaEditorial.apply(dark = false)
    }

    // ---- classifyCodeBox:判型 ----

    @Test
    fun `diff shape over threshold classifies DIFF`() {
        assertEquals(
            CodeBoxKind.DIFF,
            classifyCodeBox("+added\n-removed\n@@ -1,2 +1,2 @@\n context"),
        )
    }

    @Test
    fun `short or unmarked output stays PLAIN`() {
        // 2 行以 +/- 开头:不到 3 行下限,巧合不构成 diff
        assertEquals(CodeBoxKind.PLAIN, classifyCodeBox("++ 完成\n-- done"))
        // 0% 标记行
        assertEquals(CodeBoxKind.PLAIN, classifyCodeBox("ok\nfine\nsure"))
        // 空白
        assertEquals(CodeBoxKind.PLAIN, classifyCodeBox("   \n"))
    }

    @Test
    fun `bracketed text classifies JSON`() {
        assertEquals(CodeBoxKind.JSON, classifyCodeBox("""{"a":1,"b":[2,3]}"""))
        assertEquals(CodeBoxKind.JSON, classifyCodeBox("[1,2]\n"))
    }

    @Test
    fun `diff wins over json`() {
        // 既包在 {} 里又是 diff 形状:diff 优先(任务口径)
        assertEquals(CodeBoxKind.DIFF, classifyCodeBox("{\n-a\n+b\n-c\n+d\n+e\n}"))
    }

    // ---- codeBoxSpans:分片与拼接守恒 ----

    @Test
    fun `diff spans conserve text and mark three kinds`() {
        val text = "+added\n-removed\n@@ -1 +1 @@\n context\n"
        val spans = codeBoxSpans(text, CodeBoxKind.DIFF)
        assertEquals("拼接守恒:片段按序拼接恒等于原文", text, spans.joinToString("") { it.text })
        assertEquals(
            listOf(
                CodeBoxSpanKind.DIFF_ADD,
                CodeBoxSpanKind.DIFF_DEL,
                CodeBoxSpanKind.DIFF_HUNK,
                CodeBoxSpanKind.PLAIN,
            ),
            spans.map { it.kind }.distinct(),
        )
    }

    @Test
    fun `json spans mark keys only`() {
        val text = "{\n  \"name\": \"pa\",\n  \"n\": 3\n}"
        val spans = codeBoxSpans(text, CodeBoxKind.JSON)
        assertEquals("拼接守恒", text, spans.joinToString("") { it.text })
        assertEquals(
            listOf("\"name\":", "\"n\":"),
            spans.filter { it.kind == CodeBoxSpanKind.JSON_KEY }.map { it.text.trim() },
        )
        // 值不在键片段里(键只到冒号,值保持代码箱本色)
        assertTrue(spans.none { it.kind == CodeBoxSpanKind.JSON_KEY && "\"pa\"" in it.text })
    }

    @Test
    fun `plain and empty stay single span`() {
        assertEquals(
            listOf(CodeBoxSpan("hello", CodeBoxSpanKind.PLAIN)),
            codeBoxSpans("hello", CodeBoxKind.PLAIN),
        )
        assertEquals(
            listOf(CodeBoxSpan("", CodeBoxSpanKind.PLAIN)),
            codeBoxSpans("", CodeBoxKind.JSON),
        )
    }

    // ---- codeBoxColors:取色槽位 ----

    @Test
    fun `light palette uses accent slots`() {
        AgentTheme.mode = "dark"
        PaEditorial.apply(dark = false)
        val c = codeBoxColors(PaEditorial.CodeText)
        assertEquals(PaEditorial.Running, c.add)
        assertEquals(PaEditorial.Denied, c.del)
        assertEquals(PaEditorial.OnGraphiteSubtle, c.hunk)
        assertEquals(PaEditorial.Mcp, c.key)
        assertEquals(PaEditorial.CodeText, c.plain)
    }

    @Test
    fun `hacker palette desaturates to text slots`() {
        AgentTheme.mode = "hacker"
        val c = codeBoxColors(PaEditorial.Muted)
        assertEquals(PaEditorial.RunningText, c.add)
        assertEquals(PaEditorial.DeniedText, c.del)
        assertEquals("黑客盘 plain 由调用方给(Canvas 底配 Muted)", PaEditorial.Muted, c.plain)
        // 降饱和档必须真的不同于常规档
        assertNotEquals(PaEditorial.Running, c.add)
    }

    // ---- codeBoxAnnotated:上色接线 ----

    @Test
    fun `annotated diff carries add and del colors`() {
        AgentTheme.mode = "dark"
        PaEditorial.apply(dark = false)
        val colors = codeBoxColors(PaEditorial.CodeText)
        val annotated = codeBoxAnnotated("+a\n-b", colors, CodeBoxKind.DIFF)
        assertEquals("+a\n-b", annotated.text)
        assertEquals(colors.add, annotated.spanStyles.first { it.start == 0 }.item.color)
        assertEquals(colors.del, annotated.spanStyles.first { it.start == 3 }.item.color)
    }

    @Test
    fun `annotated default classifies diff first then json`() {
        AgentTheme.mode = "dark"
        PaEditorial.apply(dark = false)
        val colors = codeBoxColors(Color(0xFFD5E6F8))
        // 默认判型:diff 形状文本出现加色
        assertTrue(
            codeBoxAnnotated("+x\n-y\n@@ z", colors).spanStyles.any { it.item.color == colors.add },
        )
        // 默认判型:JSON 键上 Mcp 紫(行首键口径:键必须在一行行首,压缩单行 JSON 不着色)
        assertTrue(
            codeBoxAnnotated("{\n  \"k\": 1\n}", colors).spanStyles.any { it.item.color == PaEditorial.Mcp },
        )
        // 纯文本零着色
        assertTrue(codeBoxAnnotated("just text", colors).spanStyles.isEmpty())
    }
}
