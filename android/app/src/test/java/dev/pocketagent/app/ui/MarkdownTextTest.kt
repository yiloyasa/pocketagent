package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class MarkdownTextTest {
    @Test
    fun parsesCommonMarkdownWithoutDisplayingMarkers() {
        val lines = parseMarkdownForDisplay(
            """# Storage
            |
            |- **A:** 2.33 GB
            |  `G:\\试验区\\blobs`
            |```powershell
            |Get-ChildItem
            |```
            |""".trimMargin(),
        )

        assertEquals(MarkdownLineKind.HEADING, lines[0].kind)
        assertEquals("Storage", lines[0].text)
        assertEquals(MarkdownLineKind.BULLET, lines[2].kind)
        assertEquals("A: 2.33 GB", lines[2].text)
        val codeLines = lines.filter { it.kind == MarkdownLineKind.CODE }
        assertEquals(1, codeLines.size)
        assertEquals("Get-ChildItem", codeLines.single().text)
        assertFalse(lines.any { it.text.contains("**") || it.text.contains("`") })
    }

    @Test
    fun preservesPlainTextAndBlankLines() {
        val lines = parseMarkdownForDisplay("first\n\nsecond")

        assertEquals(listOf("first", "", "second"), lines.map { it.text })
        assertTrue(lines.all { it.kind == MarkdownLineKind.PARAGRAPH })
    }

    // ───────── 围栏语言标注(解析层不丢,渲染层右上角标注) ─────────

    @Test
    fun fence_language_extraction_variants() {
        assertEquals("", fenceLanguage("```"))
        assertEquals("", fenceLanguage("   ```   "))
        assertEquals("python", fenceLanguage("```python"))
        assertEquals("powershell", fenceLanguage("  ```powershell"))
        // 带元信息的取第一个空格前的词
        assertEquals("kotlin", fenceLanguage("```kotlin title=Main.kt"))
    }

    @Test
    fun code_fence_language_is_kept_and_reaches_blocks() {
        val src = "```python\nprint(1)\n```\nafter"
        val lines = parseMarkdownForDisplay(src)
        val codeLines = lines.filter { it.kind == MarkdownLineKind.CODE }
        assertEquals(listOf("print(1)"), codeLines.map { it.text })
        assertTrue("代码行应带语言标注", codeLines.all { it.lang == "python" })

        val block = groupMarkdownBlocks(lines).filterIsInstance<MdBlock.Code>().single()
        assertEquals("python", block.lang)
        assertEquals("print(1)", block.text)
        // 闭围栏后的普通段落不受影响
        assertEquals("after", lines.last().text)
    }

    @Test
    fun bare_fence_yields_empty_language_and_multiline_code_block() {
        val block = groupMarkdownBlocks(parseMarkdownForDisplay("```\nx\ny\n```"))
            .filterIsInstance<MdBlock.Code>().single()
        assertEquals("", block.lang)
        assertEquals("x\ny", block.text)
    }

    @Test
    fun fence_language_does_not_leak_between_blocks() {
        val src = "```python\na\n```\ntext\n```\nb\n```"
        val blocks = groupMarkdownBlocks(parseMarkdownForDisplay(src))
        val codeBlocks = blocks.filterIsInstance<MdBlock.Code>()
        assertEquals(2, codeBlocks.size)
        assertEquals("python", codeBlocks[0].lang)
        // 第二个裸围栏不得继承前一个块的语言
        assertEquals("", codeBlocks[1].lang)
    }

    // ───────── 表格(边界:分隔行判定 / 转义竖线 / 表后段落) ─────────

    @Test
    fun table_separator_row_boundaries() {
        assertTrue(isTableSeparatorRow("| --- | --- |"))
        assertTrue(isTableSeparatorRow("| :---: | ---: |"))
        assertTrue(isTableSeparatorRow("--- | ---"))
        assertTrue(isTableSeparatorRow("|---|"))
        // 无 '|' 的水平线不是表格分隔(那是分隔线,防误吞段落)
        assertFalse(isTableSeparatorRow("---"))
        // 普通两列文本不是分隔行
        assertFalse(isTableSeparatorRow("a | b"))
        assertFalse(isTableSeparatorRow(""))
    }

    @Test
    fun table_parses_header_body_and_escaped_pipe_cells() {
        val src = "| 名称 | 值 |\n| --- | --- |\n| a \\| b | 2 |\n| c | 3 |"
        val lines = parseMarkdownForDisplay(src)
        assertEquals(
            listOf(MarkdownLineKind.TABLE, MarkdownLineKind.TABLE, MarkdownLineKind.TABLE),
            lines.map { it.kind },
        )
        val table = groupMarkdownBlocks(lines).filterIsInstance<MdBlock.Table>().single()
        val cellText = { row: List<List<MarkdownSpan>> -> row.map { c -> c.joinToString("") { it.text } } }
        assertEquals(listOf("名称", "值"), cellText(table.header))
        assertEquals(2, table.rows.size)
        // 转义竖线是内容,不拆列
        assertEquals(listOf("a | b", "2"), cellText(table.rows[0]))
        assertEquals(listOf("c", "3"), cellText(table.rows[1]))
    }

    @Test
    fun table_needs_separator_row_and_stops_at_blank_line() {
        // 无分隔行:保持段落,不吞进表格
        val noSep = parseMarkdownForDisplay("a | b\nc | d")
        assertEquals(2, noSep.size)
        assertTrue(noSep.all { it.kind == MarkdownLineKind.PARAGRAPH })

        // 空行终止表体,后续段落保持原样
        val withBlank = parseMarkdownForDisplay("| h |\n| --- |\n| v |\n\nplain")
        assertEquals(MarkdownLineKind.TABLE, withBlank.first().kind)
        assertEquals(MarkdownLineKind.PARAGRAPH, withBlank.last().kind)
        assertEquals("plain", withBlank.last().text)
    }

    @Test
    fun short_table_cells_are_padded_to_column_count() {
        val lines = parseMarkdownForDisplay("| a | b | c |\n| --- | --- | --- |\n| only |")
        val table = groupMarkdownBlocks(lines).filterIsInstance<MdBlock.Table>().single()
        assertEquals(3, table.rows.single().size)
        assertEquals("", table.rows.single()[2].joinToString("") { it.text })
    }

    // ───────── 嵌套列表缩进层级 ─────────

    @Test
    fun nested_list_indent_levels_follow_leading_whitespace() {
        val lines = parseMarkdownForDisplay("- top\n  - second\n    - third\ntext")
        val bullets = lines.filter { it.kind == MarkdownLineKind.BULLET }
        assertEquals(listOf(0, 1, 2), bullets.map { it.indent })
        assertEquals(listOf("top", "second", "third"), bullets.map { it.text })
        // 有序与无序同一条缩进口径
        val ordered = parseMarkdownForDisplay("1. a\n  2. b")
        assertEquals(listOf(0, 1), ordered.map { it.indent })
        // 层级封顶 4,不随疯狂缩进无限加深
        assertEquals(4, bulletIndentLevel("                    - deep"))
    }

    // ───────── 引用块 ─────────

    @Test
    fun quote_lines_parse_and_merge_into_one_block() {
        val blocks = groupMarkdownBlocks(parseMarkdownForDisplay("> first\n> second\nafter"))
        val quote = blocks.filterIsInstance<MdBlock.Quote>().single()
        assertEquals(listOf("first", "second"), quote.lines.map { it.text })
        // 引用后的普通段落仍在引用块外
        assertTrue(blocks.last() is MdBlock.Line)
        assertEquals("after", (blocks.last() as MdBlock.Line).line.text)
    }

    // ───────── 表格列宽配比(纯函数) ─────────

    @Test
    fun table_column_weights_normalize_and_cap() {
        val w = tableColumnWeights(
            listOf(
                listOf("a", "very long column content that keeps going"),
                listOf("b", "x"),
            ),
        )
        assertEquals(2, w.size)
        assertEquals(1f, w.sum(), 0.001f)
        assertTrue(w[0] > 0f && w[1] > 0f)
        // 长列被封顶截住:两列权重比不超过 上限/下限(24/4)
        assertTrue(w[1] / w[0] <= TABLE_COL_MAX_CHARS / TABLE_COL_MIN_CHARS + 0.01f)

        // 退化输入不崩
        assertTrue(tableColumnWeights(emptyList()).isEmpty())
        assertEquals(listOf(1f), tableColumnWeights(listOf(listOf("only"))))
    }
}
