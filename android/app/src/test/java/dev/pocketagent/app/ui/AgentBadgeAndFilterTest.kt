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

    /**
     * 回归:dsh 等"when 没列出来"的 agent 以前被 else 兜底成 codex 的脸。
     * 现在统一收口到 AGENT_BRAND_ICON,映射表里有的都走自己的图。
     */
    @Test
    fun `已知brand直接走映射表_dsh不再兜底成codex`() {
        assertEquals(R.drawable.agent_dsh, agentBadgeRes("dsh"))
        assertEquals(R.drawable.agent_hermes, agentBadgeRes("hermes"))
        assertEquals(R.drawable.agent_opencode, agentBadgeRes("opencode"))
    }

    /**
     * 映射表里没有的 id(daemon 将来动态上报的新 harness)用中性 cpu 图标兜底,
     * 而不是偷偷显示成 codex 的脸。
     */
    @Test
    fun `未知agent徽标中性兜底不冒充他人`() {
        assertEquals(R.drawable.ic_lucide_cpu, agentBadgeRes("some-future-agent"))
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
