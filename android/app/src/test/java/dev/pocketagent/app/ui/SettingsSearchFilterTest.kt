package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 设置搜索(审计 P1-3 子项)纯函数的单测。
 *
 * 口径:空查询=不进入搜索模式(返回空列表,页面照常渲染分组);
 * 非空查询对 title+desc 大小写不敏感匹配;命中区间给高亮用。
 */
class SettingsSearchFilterTest {

    private val index = settingsSearchIndex()

    @Test
    fun `索引非空且每条有组名`() {
        assertTrue(index.isNotEmpty())
        assertTrue(index.all { it.group.isNotBlank() && it.title.isNotBlank() })
    }

    @Test
    fun `空查询_返回空列表`() {
        assertTrue(filterSettingsEntries("", index).isEmpty())
        assertTrue(filterSettingsEntries("   ", index).isEmpty())
    }

    @Test
    fun `按标题命中`() {
        val hits = filterSettingsEntries("外观", index)
        assertTrue(hits.any { it.title == "外观与阅读" })
    }

    @Test
    fun `按描述命中_大小写不敏感`() {
        val hits = filterSettingsEntries("mcp", index)
        assertTrue(hits.any { it.title == "Agent 与模型" }) // desc 含 "MCP"
    }

    @Test
    fun `无命中_返回空列表`() {
        assertTrue(filterSettingsEntries("不存在的设置项xyz", index).isEmpty())
    }

    @Test
    fun `命中区间_标题命中给区间_未命中标题给null`() {
        assertEquals(0 until 2, matchRange("外观与阅读", "外观"))
        // "Daemon 桥接与输入":Da(0..1)emon=0..5,空格=6,桥接=7..8
        assertEquals(7 until 9, matchRange("Daemon 桥接与输入", "桥接"))
        assertNull(matchRange("外观与阅读", "模型"))
        assertNull(matchRange("外观与阅读", ""))
    }
}
