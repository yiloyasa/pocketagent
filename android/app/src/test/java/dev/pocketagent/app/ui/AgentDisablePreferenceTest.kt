package dev.pocketagent.app.ui

import dev.pocketagent.app.AgentConnection
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * agent 停用偏好(T-MW:管理 agent 抽屉开关接成真的)的单测。
 *
 * 覆盖四条线:
 *  1. 停用/恢复的持久化语义([AgentConnection.setAgentDisabled] 内存集合 + 幂等);
 *  2. 空偏好零行为(不停用任何 agent = 一切照旧);
 *  3. 与 capabilities 能力判定的叠加边界(偏好层只隐藏,不改写 available);
 *  4. 抽屉开关点按结果与新建任务放行(可用=真停用/恢复,不可用=不伪造)。
 */
class AgentDisablePreferenceTest {

    /** AgentConnection.disabledAgents 是进程级单例态:用例前后恢复现场。 */
    private fun <T> withDisabledAgents(set: Set<String>, block: () -> T): T {
        val old = AgentConnection.disabledAgents
        AgentConnection.setDisabledAgentsForTest(set)
        return try {
            block()
        } finally {
            AgentConnection.setDisabledAgentsForTest(old)
        }
    }

    // ---- 1. 停用持久化/恢复(setAgentDisabled → disabledAgents/isAgentDisabled)----

    @Test
    fun `停用写入集合_恢复移除_setter与查询一致`() {
        withDisabledAgents(emptySet()) {
            AgentConnection.setAgentDisabled("pi", true)
            assertTrue(AgentConnection.isAgentDisabled("pi"))
            assertEquals(setOf("pi"), AgentConnection.disabledAgents)
            // 恢复(抽屉开关拨回开)
            AgentConnection.setAgentDisabled("pi", false)
            assertFalse(AgentConnection.isAgentDisabled("pi"))
            assertEquals(emptySet<String>(), AgentConnection.disabledAgents)
        }
    }

    @Test
    fun `多agent停用互不干扰_恢复一个其余保留`() {
        withDisabledAgents(emptySet()) {
            AgentConnection.setAgentDisabled("pi", true)
            AgentConnection.setAgentDisabled("zcode", true)
            assertEquals(setOf("pi", "zcode"), AgentConnection.disabledAgents)
            AgentConnection.setAgentDisabled("pi", false)
            assertEquals(setOf("zcode"), AgentConnection.disabledAgents)
            assertTrue(AgentConnection.isAgentDisabled("zcode"))
        }
    }

    @Test
    fun `幂等同向重复写不产生重复项_空白id零行为`() {
        withDisabledAgents(emptySet()) {
            AgentConnection.setAgentDisabled("pi", true)
            AgentConnection.setAgentDisabled("pi", true)
            assertEquals(setOf("pi"), AgentConnection.disabledAgents)
            AgentConnection.setAgentDisabled("pi", false)
            AgentConnection.setAgentDisabled("pi", false) // 已是未停用,再写不反弹
            assertFalse(AgentConnection.isAgentDisabled("pi"))
            // 空白/纯空白 id:无处可停,集合不动
            AgentConnection.setAgentDisabled("", true)
            AgentConnection.setAgentDisabled("   ", true)
            assertEquals(emptySet<String>(), AgentConnection.disabledAgents)
        }
    }

    @Test
    fun `isAgentDisabled_查询前trim且空白恒false`() {
        withDisabledAgents(setOf("pi")) {
            assertTrue(AgentConnection.isAgentDisabled(" pi "))
            assertFalse(AgentConnection.isAgentDisabled(""))
            assertFalse(AgentConnection.isAgentDisabled("  "))
        }
    }

    // ---- 2. 空偏好零行为 ----

    @Test
    fun `空偏好零行为_全部显示_清单原样_新建全部放行`() {
        val agents = buildAgents(listOf("agent:pi:v0.85.1"))
        assertTrue(shouldShowAgent("codex", emptySet()))
        assertTrue(shouldShowAgent("anything", emptySet()))
        assertEquals(agents, filterAgents(agents, emptySet()))
        assertTrue(newThreadAgentAllowed("codex", emptySet()))
        assertTrue(newThreadAgentAllowed("pi", emptySet()))
        // 空偏好下 isAgentDisabled 恒 false
        withDisabledAgents(emptySet()) {
            assertFalse(AgentConnection.isAgentDisabled("codex"))
        }
    }

    // ---- 3. 与 capabilities 能力判定的叠加(覆盖不破坏)----

    @Test
    fun `停用仅隐藏不改写能力判定_恢复后磁贴原样回来`() {
        val caps = listOf("agent:pi:v0.85.1", "agent:zcode")
        val agents = buildAgents(caps)
        val pi = agents.first { it.id == "pi" }
        assertTrue("daemon 报了就该点亮(能力判定本身)", pi.available)
        withDisabledAgents(setOf("pi")) {
            // 偏好层从展示清单隐藏,但不改写 available 字段
            val shown = filterAgents(agents, AgentConnection.disabledAgents)
            assertFalse("已停用的 pi 不得出现在网格清单", shown.any { it.id == "pi" })
            assertEquals(true, agents.first { it.id == "pi" }.available)
            assertEquals(true, agents.first { it.id == "zcode" }.available)
            assertTrue("未停用的 zcode 照常显示", shown.any { it.id == "zcode" })
        }
        // 恢复(停用集合移除)后磁贴原样回来,能力判定照旧 —— 偏好层没留下任何污染
        val restored = filterAgents(agents, emptySet())
        assertEquals(pi, restored.first { it.id == "pi" })
        assertEquals(agents.size, restored.size)
    }

    @Test
    fun `停用与灰显正交_停用一个不可用agent也不误点亮谁`() {
        // 停用列表与 capabilities 完全独立:daemon 没报的 agent 依旧灰显,
        // 停用别的 agent 不改变它的可用性
        val agents = buildAgents(emptyList())
        val shown = filterAgents(agents, setOf("codex"))
        assertFalse(shown.any { it.id == "codex" })
        val hermes = shown.first { it.id == "hermes" }
        assertFalse(hermes.available)
    }

    // ---- 4. 抽屉开关点按结果(可用=真停用/恢复;不可用=不伪造)----

    @Test
    fun `可用agent开关_关=真实停用_开=真实恢复_文案给恢复路径`() {
        val off = agentManageToggleResult(requestOn = false, available = true, disabled = false, agentId = "pi", name = "Pi")
        assertTrue("关闭请求应产生停用态", off.first)
        assertTrue("提示应说明已停用", off.second!!.contains("已停用"))
        val on = agentManageToggleResult(requestOn = true, available = true, disabled = true, agentId = "pi", name = "Pi")
        assertFalse("打开请求应恢复(未停用)", on.first)
        assertTrue("提示应说明已恢复", on.second!!.contains("已恢复"))
    }

    @Test
    fun `不可用agent开关_不伪造停用_状态原地不动_给真实原因`() {
        // daemon 未报的 agent:拨开关只弹真实原因,停用态不变
        val unreported = agentManageToggleResult(requestOn = false, available = false, disabled = false, agentId = "opencode", name = "OpenCode")
        assertFalse("不可用 agent 不得被写成已停用", unreported.first)
        assertTrue("提示应是真实原因(接入中)", unreported.second!!.contains("接入中"))
        val notInstalled = agentManageToggleResult(requestOn = true, available = false, disabled = false, agentId = "gemini", name = "gemini")
        assertFalse(notInstalled.first)
        assertTrue("能力驱动型灰显应报未安装", notInstalled.second!!.contains("电脑端未安装"))
        // 曾停用后 daemon 掉线的 agent:点按不得把它误恢复(偏好保留,等 daemon 回来仍隐藏)
        val keepDisabled = agentManageToggleResult(requestOn = true, available = false, disabled = true, agentId = "pi", name = "Pi")
        assertTrue(keepDisabled.first)
    }

    // ---- 5. 新建任务入口放行(停用 agent 不再创建新会话)----

    @Test
    fun `新建任务_已停用agent拒绝_其余放行`() {
        val disabled = setOf("pi")
        assertFalse(newThreadAgentAllowed("pi", disabled))
        assertTrue(newThreadAgentAllowed("zcode", disabled))
        // 空串按 codex 归属判定(与 threadAgentIdOf 同口径):停用 codex 即挡住缺省新建
        assertFalse(newThreadAgentAllowed("", setOf("codex")))
        assertTrue(newThreadAgentAllowed("", disabled))
        // trim 后仍比对(" pi " 视为 pi)
        assertFalse(newThreadAgentAllowed(" pi ", disabled))
    }
}
