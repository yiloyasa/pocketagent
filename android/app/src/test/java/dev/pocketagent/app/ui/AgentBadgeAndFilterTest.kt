package dev.pocketagent.app.ui

import dev.pocketagent.app.R
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 会话卡 agent 徽标选图与 agent 筛选判定的单测。
 * 兼容规则与 daemon 侧 threadAgentID 一致:agent_id 缺省(旧库)= codex。
 */
class AgentBadgeAndFilterTest {

    // ---- threadAgentIdOf ----

    @Test
    fun `agent_id缺省视为codex`() {
        assertEquals("codex", threadAgentIdOf(""))
        assertEquals("codex", threadAgentIdOf("codex"))
        assertEquals("pi", threadAgentIdOf("pi"))
        assertEquals("zcode", threadAgentIdOf("zcode"))
    }

    // ---- agentBadgeRes(徽标选图)----

    @Test
    fun `徽标按归属选图_空为codex`() {
        assertEquals(R.drawable.agent_codex, agentBadgeRes(""))
        assertEquals(R.drawable.agent_codex, agentBadgeRes("codex"))
        assertEquals(R.drawable.agent_pi, agentBadgeRes("pi"))
        assertEquals(R.drawable.agent_zcode, agentBadgeRes("zcode"))
    }

    @Test
    fun `未知agent徽标兜底codex`() {
        assertEquals(R.drawable.agent_codex, agentBadgeRes("hermes"))
    }

    // ---- matchesAgentFilter(筛选判定)----

    @Test
    fun `全部放行`() {
        assertTrue(matchesAgentFilter("pi", AGENT_FILTER_ALL))
        assertTrue(matchesAgentFilter("", AGENT_FILTER_ALL))
        assertTrue(matchesAgentFilter("zcode", AGENT_FILTER_ALL))
    }

    @Test
    fun `按归属精确匹配`() {
        assertTrue(matchesAgentFilter("zcode", AGENT_FILTER_ZCODE))
        assertTrue(matchesAgentFilter("", AGENT_FILTER_CODEX)) // 缺省按 codex 归属
        assertFalse(matchesAgentFilter("pi", AGENT_FILTER_ZCODE))
        assertFalse(matchesAgentFilter("zcode", AGENT_FILTER_PI))
    }
}
