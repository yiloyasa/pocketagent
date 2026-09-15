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
}
