package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.AgentThreadInfo

/**
 * 电脑上 Pi/ZCode 会话行(ListAgentThreads 分源)的纯函数单测:
 * 混合列表按 agent 筛选不串源;稳定键跨 agent 不撞。
 */
class AgentPcSessionsTest {

    private fun info(agentId: String, sessionId: String) =
        AgentThreadInfo.newBuilder().setAgentId(agentId).setSessionId(sessionId).build()

    // ---- pcAgentVisible(筛选判定)----

    @Test
    fun `全部筛选放行两族会话`() {
        assertTrue(pcAgentVisible("pi", AGENT_FILTER_ALL))
        assertTrue(pcAgentVisible("zcode", AGENT_FILTER_ALL))
    }

    @Test
    fun `单一agent筛选不放行另一家`() {
        assertTrue(pcAgentVisible("pi", AGENT_FILTER_PI))
        assertFalse(pcAgentVisible("zcode", AGENT_FILTER_PI))
        assertTrue(pcAgentVisible("zcode", AGENT_FILTER_ZCODE))
        assertFalse(pcAgentVisible("pi", AGENT_FILTER_ZCODE))
        // codex 筛选下 Pi/ZCode 电脑会话整组隐藏(codex 走自己的 pcThreads)
        assertFalse(pcAgentVisible("pi", AGENT_FILTER_CODEX))
        assertFalse(pcAgentVisible("zcode", AGENT_FILTER_CODEX))
    }

    // ---- agentPcKey(稳定键)----

    @Test
    fun `键由agent与sessionId组成且互不撞`() {
        assertEquals("pi:sess-1", agentPcKey(info("pi", "sess-1")))
        assertEquals("zcode:sess-1", agentPcKey(info("zcode", "sess-1")))
        // 同一 sessionId 出现在两个 agent 下(理论不可能,但键必须可区分)
        assertTrue(agentPcKey(info("pi", "sess-1")) != agentPcKey(info("zcode", "sess-1")))
    }
}
