package dev.pocketagent.app.ui

import dev.pocketagent.app.AgentConnection
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 建会话入口停用放行闸(T-MW2:ChatScreen / ThreadsScreen 两处 createThread 前
 * 复用 [newThreadAgentAllowed] 纯函数)的单测。
 *
 * 用例按两个入口的真实调用形状取判定
 * (`newThreadAgentAllowed(AgentConnection.selectedAgent, AgentConnection.disabledAgents)`),
 * 覆盖:
 *  1. 拦截生效:当前选中 agent 已停用 → 判定不放行(两个入口同判);
 *  2. 未停用零行为:偏好为空 / 目标未停用 → 判定放行(一切照旧);
 *  3. 两入口口径一致:同一 (selectedAgent, disabledAgents) 下判定必然相同 ——
 *     抽屉恢复(偏好移除)后放行,与主页"新建任务"入口同一条链。
 */
class NewThreadGateEntriesTest {

    /** AgentConnection 单例态(selectedAgent + disabledAgents)用例前后恢复现场。 */
    private fun <T> withAgentState(selected: String, disabled: Set<String>, block: () -> T): T {
        val oldSelected = AgentConnection.selectedAgent
        val oldDisabled = AgentConnection.disabledAgents
        AgentConnection.selectedAgent = selected
        AgentConnection.setDisabledAgentsForTest(disabled)
        return try {
            block()
        } finally {
            AgentConnection.selectedAgent = oldSelected
            AgentConnection.setDisabledAgentsForTest(oldDisabled)
        }
    }

    // ---- 1. 拦截生效:已停用 agent 在两入口都被挡下 ----

    @Test
    fun `已停用选中agent_ChatScreen入口判定不放行`() {
        withAgentState(selected = "pi", disabled = setOf("pi")) {
            // ChatScreen.kt createThread 前的判定(T-MW2 接线后的真实调用形状)
            val allowed = newThreadAgentAllowed(AgentConnection.selectedAgent, AgentConnection.disabledAgents)
            assertFalse("已停用的 pi 不得从 ChatScreen 入口新建会话", allowed)
        }
    }

    @Test
    fun `已停用选中agent_ThreadsScreen入口判定不放行`() {
        withAgentState(selected = "zcode", disabled = setOf("zcode")) {
            // ThreadsScreen.kt createThread 前的判定(与 ChatScreen 同款调用形状)
            val allowed = newThreadAgentAllowed(AgentConnection.selectedAgent, AgentConnection.disabledAgents)
            assertFalse("已停用的 zcode 不得从 ThreadsScreen 入口新建会话", allowed)
        }
    }

    @Test
    fun `停用codex挡住缺省新建_空白selectedAgent按codex归属`() {
        // 空白 selectedAgent 落到 codex(threadAgentIdOf 缺省口径):停用 codex 即挡住
        // 两入口的缺省新建;停用别的 agent 不波及缺省入口
        withAgentState(selected = "codex", disabled = setOf("codex")) {
            assertFalse(newThreadAgentAllowed(AgentConnection.selectedAgent, AgentConnection.disabledAgents))
        }
        withAgentState(selected = "", disabled = setOf("codex")) {
            assertFalse(newThreadAgentAllowed(AgentConnection.selectedAgent, AgentConnection.disabledAgents))
        }
        withAgentState(selected = "", disabled = setOf("pi")) {
            assertTrue(newThreadAgentAllowed(AgentConnection.selectedAgent, AgentConnection.disabledAgents))
        }
    }

    // ---- 2. 未停用零行为:空偏好/未停用目标一律放行 ----

    @Test
    fun `空偏好零行为_任意选中agent两入口均放行`() {
        withAgentState(selected = "pi", disabled = emptySet()) {
            assertTrue(newThreadAgentAllowed(AgentConnection.selectedAgent, AgentConnection.disabledAgents))
        }
        withAgentState(selected = "zcode", disabled = emptySet()) {
            assertTrue(newThreadAgentAllowed(AgentConnection.selectedAgent, AgentConnection.disabledAgents))
        }
        withAgentState(selected = "codex", disabled = emptySet()) {
            assertTrue(newThreadAgentAllowed(AgentConnection.selectedAgent, AgentConnection.disabledAgents))
        }
    }

    @Test
    fun `停用别人不波及_未停用选中agent照常放行`() {
        withAgentState(selected = "zcode", disabled = setOf("pi")) {
            assertTrue(newThreadAgentAllowed(AgentConnection.selectedAgent, AgentConnection.disabledAgents))
        }
    }

    // ---- 3. 恢复链:抽屉恢复(偏好移除)后两入口重新放行,判定与主页一致 ----

    @Test
    fun `恢复后重新放行_与主页入口判定一致`() {
        withAgentState(selected = "pi", disabled = setOf("pi")) {
            assertFalse("停用期间拦截", newThreadAgentAllowed(AgentConnection.selectedAgent, AgentConnection.disabledAgents))
            // 抽屉开关拨回"显示"= 偏好集合移除(AgentConnection.setAgentDisabled(false) 的落点)
            AgentConnection.setAgentDisabled("pi", false)
            assertTrue("恢复后放行", newThreadAgentAllowed(AgentConnection.selectedAgent, AgentConnection.disabledAgents))
            // 与主页"新建任务"入口同判(同一纯函数,入参同源 → 必然一致)
            assertEquals(
                newThreadAgentAllowed(AgentConnection.selectedAgent, AgentConnection.disabledAgents),
                newThreadAgentAllowed(AgentConnection.selectedAgent, AgentConnection.disabledAgents),
            )
        }
    }
}
