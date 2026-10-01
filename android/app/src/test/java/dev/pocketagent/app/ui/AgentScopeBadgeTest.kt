package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Test

/**
 * S2 作用域徽章(AgentScopeBadge)的纯数据面单测 —— Composable 本体不进 JVM 测试,
 * 这里钉住词表与作用域清单这两个"改徽章必改"的数据源。
 *
 * 【要钉住的两件事】
 *  1. 徽章名与设置页其余位置同一份词表(agentDisplay:未知 id 回退原文,不猜,
 *     AUDIT §3.3);
 *  2. 「会话与对话」组的作用域 = 全部已接入 agent(codex/pi/zcode/hermes),
 *     与原 Daemon 页「全局对话显示」"全部 Agent"的口径一致,聚拢后不许缩水。
 */
class AgentScopeBadgeTest {

    @Test
    fun `徽章名复用agentDisplay词表_已知id译人话`() {
        assertEquals("Codex", agentDisplay("codex"))
        assertEquals("Pi", agentDisplay("pi"))
        assertEquals("ZCode", agentDisplay("zcode"))
        assertEquals("Hermes", agentDisplay("hermes"))
    }

    @Test
    fun `未知id回退原文_空id回退Agent`() {
        assertEquals("new-agent", agentDisplay("new-agent"))
        assertEquals("Agent", agentDisplay(""))
    }

    @Test
    fun `会话与对话作用域覆盖全部已接入agent`() {
        assertEquals(listOf("codex", "pi", "zcode", "hermes"), SESSION_DISPLAY_SCOPES)
    }

    @Test
    fun `作用域内每个agent都有品牌图标_徽章不留空洞`() {
        // 徽章渲染:有图标才画图标;已接入的四个 agent 必须都有品牌图,否则标签缺半边。
        SESSION_DISPLAY_SCOPES.forEach { id ->
            assertNotNull("缺少品牌图标:$id", agentBrandIcon(id))
        }
    }
}
