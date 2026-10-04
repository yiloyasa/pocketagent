package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * S2 作用域徽章(AgentScopeBadge)的纯数据面单测 —— Composable 本体不进 JVM 测试,
 * 这里钉住词表与作用域清单这两个"改徽章必改"的数据源。
 *
 * 【要钉住的三件事】
 *  1. 徽章名与设置页其余位置同一份词表(agentDisplay:未知 id 回退原文,不猜,
 *     AUDIT §3.3);
 *  2. 「会话与对话」组的作用域 = 全部已接入 agent(codex/pi/zcode/hermes),
 *     与原 Daemon 页「全局对话显示」"全部 Agent"的口径一致,聚拢后不许缩水;
 *  3. 作用域名单与展示词表不漂移:SESSION_DISPLAY_SCOPES 每项都在 AGENT_DISPLAY
 *     有映射名(AUDIT 附录 A 挂牌的"新增 agent 须同步"义务的测试化 —— 漏同步时
 *     agentDisplay 回退原生 id,徽章露 "new-agent" 式原文,本组用例即红)。
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

    @Test
    fun `作用域名单每项都有AGENT_DISPLAY映射名_名单与词表不漂移`() {
        // 「新增 agent 须同步本行(并确认 AGENT_DISPLAY 有其显示名)」义务的测试化。
        // AGENT_DISPLAY 是 private,借 agentDisplay 的回退语义做等价断言:
        // agentDisplay = AGENT_DISPLAY[id] ?: id 原文 —— 对非空 id,
        // 「在 AGENT_DISPLAY 有映射名」⇔「译文 ≠ 原文 id」。
        // 若名单新增 agent 而词表漏同步,徽章将露出 "new-agent" 式原生 id,此处即红。
        SESSION_DISPLAY_SCOPES.forEach { id ->
            assertTrue(
                "SESSION_DISPLAY_SCOPES 含未登记显示名的 agent:$id(须在 AGENT_DISPLAY 补映射)",
                agentDisplay(id) != id,
            )
        }
    }
}
