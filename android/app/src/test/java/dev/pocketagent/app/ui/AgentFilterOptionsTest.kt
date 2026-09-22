package dev.pocketagent.app.ui

// AgentFilterOptionsTest.kt —— 会话列表「agent 筛选维度」的回归钉子。
//
// 【背景】用户实测：会话列表的筛选条只有 Codex / Pi / ZCode —— **新增的 DSH 根本
// 不出现**。根因是那四个 chip 是**硬编码**的。改成从 daemon 报的 capabilities
// （`agent:<id>`）动态派生后，本文件钉住派生规则：
//  1. 只认 agent: 前缀；带版本段（`agent:pi:v0.85.1`）也能取到 id；
//  2. **不出现 echo**（它是冒烟用的假 agent，不该成为用户的筛选维度）；
//  3. 去重 + 稳定排序（capabilities 是集合，chip 顺序必须可复现）；
//  4. 标签走 selectedAgentLabel（与主页/会话页同一套显示名，不各写一份）。

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class AgentFilterOptionsTest {

    @Test
    fun 四家装齐时四个维度都在() {
        val caps = listOf(
            "agent:codex", "agent:pi", "agent:zcode", "agent:dsh",
            "collab:pi", "collab:dsh", "terminal", "fs",
        )
        val ids = agentFilterOptions(caps).map { it.first }
        assertEquals(listOf("codex", "dsh", "pi", "zcode"), ids)
    }

    @Test
    fun 带版本段也能取到id() {
        val ids = agentFilterOptions(listOf("agent:pi:v0.85.1", "agent:codex")).map { it.first }
        assertEquals(listOf("codex", "pi"), ids)
    }

    @Test
    fun 不把echo当筛选维度() {
        val ids = agentFilterOptions(listOf("agent:echo", "agent:pi")).map { it.first }
        assertEquals(listOf("pi"), ids)
        assertFalse("echo 是冒烟假 agent,不该出现在筛选条", ids.contains("echo"))
    }

    @Test
    fun 去重且排序稳定() {
        val ids = agentFilterOptions(listOf("agent:pi", "agent:dsh", "agent:pi", "agent:dsh")).map { it.first }
        assertEquals(listOf("dsh", "pi"), ids)
    }

    @Test
    fun 标签走统一显示名() {
        val opts = agentFilterOptions(listOf("agent:dsh"))
        assertEquals(1, opts.size)
        assertEquals("dsh", opts[0].first)
        assertTrue("标签应非空且用的是统一显示名", opts[0].second.isNotBlank())
    }

    @Test
    fun 无任何agent时不产生维度() {
        assertEquals(emptyList<Pair<String, String>>(), agentFilterOptions(listOf("terminal", "fs", "git")))
    }
}
