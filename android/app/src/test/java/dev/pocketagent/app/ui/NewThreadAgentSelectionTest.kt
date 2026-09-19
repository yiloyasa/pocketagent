package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Test
import pocketagent.v1.CreateThreadRequest

/**
 * 新建会话 agent 接线的单测(P1 修复回归):
 * 修复前 onCreate 只 setModelId/setWorkspace,设置/主页切到 Pi 后新建会话仍走 codex;
 * 修复后由 [createThreadAgentId] 决定是否 setAgentId —— 判定是纯函数,这里直接实测
 * "选中值 → CreateThreadRequest.agent_id 落到线上" 的完整行为。
 *
 * 语义与 daemon 侧 providerForThread 对齐:agent_id 缺省(空)= 旧默认 codex,
 * 显式传 "pi"/"zcode" = 路由到对应后端。
 */
class NewThreadAgentSelectionTest {

    // ---- createThreadAgentId:选中值 → 应写入请求的 agent_id ----

    @Test
    fun `选中pi时新建会话应携带agentId_pi`() {
        assertEquals("pi", createThreadAgentId("pi"))
    }

    @Test
    fun `选中zcode时新建会话应携带agentId_zcode`() {
        assertEquals("zcode", createThreadAgentId("zcode"))
    }

    @Test
    fun `选中codex时新建会话agentId为空_保持旧语义`() {
        assertEquals("", createThreadAgentId("codex"))
    }

    @Test
    fun `默认(空串)时新建会话agentId为空_走server默认路由`() {
        assertEquals("", createThreadAgentId(""))
    }

    @Test
    fun `选中值带空白时去除空白后再判定`() {
        // 与 AgentConnection.switchAgent 的 trim 对齐:写入侧已去空白,这里兜底同样处理
        assertEquals("pi", createThreadAgentId(" pi "))
        assertEquals("", createThreadAgentId("  "))
    }

    // ---- CreateThreadRequest 组装:与 onCreate 内两处入口同款写法 ----

    /** 复现 onCreate 的组装序列,断言最终请求里的 agent_id(空 = proto 缺省不设)。 */
    private fun buildRequest(selectedAgent: String): CreateThreadRequest {
        val builder = CreateThreadRequest.newBuilder()
        val agentId = createThreadAgentId(selectedAgent)
        if (agentId.isNotEmpty()) {
            builder.setAgentId(agentId)
        }
        return builder.build()
    }

    @Test
    fun `组装请求_选中pi则请求携带agentId等于pi`() {
        val request = buildRequest("pi")
        assertEquals("pi", request.agentId)
    }

    @Test
    fun `组装请求_选中codex则请求agentId为空`() {
        val request = buildRequest("codex")
        assertEquals("", request.agentId)
    }

    @Test
    fun `组装请求_默认则请求agentId为空`() {
        val request = buildRequest("")
        assertEquals("", request.agentId)
    }

    // ---- selectedAgentLabel:标签展示名与选中源一致 ----

    @Test
    fun `标签展示名_与设置页切换Agent命名一致`() {
        assertEquals("Codex", selectedAgentLabel("codex"))
        assertEquals("Codex", selectedAgentLabel("")) // 默认(未切换过)= Codex
        assertEquals("Pi", selectedAgentLabel("pi"))
        assertEquals("ZCode", selectedAgentLabel("zcode"))
        assertEquals("OpenCode", selectedAgentLabel("opencode"))
        assertEquals("Claude", selectedAgentLabel("claude"))
        assertEquals("Hermes", selectedAgentLabel("hermes"))
        assertEquals("unknown-agent", selectedAgentLabel("unknown-agent")) // 未知 id 原样显示
    }
}
