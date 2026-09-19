package dev.pocketagent.app.ui

import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 分组覆盖范围说明文案的单测(用户实锤:要明确"分组功能覆盖哪些 Agent")。
 * 锁定两点:说明必须点明是 PocketAgent 的本地整理功能;必须覆盖三家 agent。
 */
class SectionCoverageNoteTest {

    @Test
    fun `说明点明是PocketAgent本地整理功能`() {
        assertTrue(SECTION_COVERAGE_NOTE.contains("PocketAgent"))
        assertTrue(SECTION_COVERAGE_NOTE.contains("本地"))
    }

    @Test
    fun `说明覆盖三家agent`() {
        assertTrue(SECTION_COVERAGE_NOTE.contains("Codex"))
        assertTrue(SECTION_COVERAGE_NOTE.contains("Pi"))
        assertTrue(SECTION_COVERAGE_NOTE.contains("ZCode"))
    }
}
