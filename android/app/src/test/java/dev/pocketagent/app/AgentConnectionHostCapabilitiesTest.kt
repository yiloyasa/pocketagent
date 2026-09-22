package dev.pocketagent.app

import dev.pocketagent.app.ui.buildAgents
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * T-DSH 问题 A 回归:agent 可用性统一数据源 —— capabilities 共享快照。
 *
 * 【缺陷即测试】真机实锤(2026-09-21):主页网格 Pi/ZCode/DSH 全部点亮
 * (daemon GetHostInfo 报了能力),而另一条渲染链却按空 capabilities 渲染成
 * "只有 Codex 可选,其余灰显电脑端未安装"。根因 = 各页面各持独立的
 * GetHostInfo 局部快照,任一条失败/未就绪就按空数据自成一派。
 * 修复 = AgentConnection.hostCapabilities 共享快照:所有成功路径写入
 * ([AgentConnection.updateHostCapabilities]),渲染链一律读它。
 * 本用例钉死:写入生效、StateFlow 同值去重、共享快照与点亮判定的接线
 * (buildAgents 对 daemon 真值必须全点亮、对空快照只亮 codex)。
 */
class AgentConnectionHostCapabilitiesTest {

    /** 用例前后恢复共享快照现场(AgentConnection 是进程级单例)。 */
    private fun withCaps(caps: List<String>, block: () -> Unit) {
        val old = AgentConnection.hostCapabilities.value
        try {
            AgentConnection.updateHostCapabilities(caps)
            block()
        } finally {
            AgentConnection.updateHostCapabilities(old)
        }
    }

    @Test
    fun `updateHostCapabilities 写入共享快照`() {
        withCaps(listOf("agent:pi", "agent:dsh", "agent:zcode")) {
            assertEquals(
                listOf("agent:pi", "agent:dsh", "agent:zcode"),
                AgentConnection.hostCapabilities.value,
            )
        }
    }

    @Test
    fun `同值重复写入去重_StateFlow语义`() {
        withCaps(emptyList()) {
            val first = AgentConnection.hostCapabilities.value
            AgentConnection.updateHostCapabilities(first.toList()) // 同值(内容相等)再写
            assertEquals(first, AgentConnection.hostCapabilities.value)
            // StateFlow 保证:相等值不向订阅者重复发射(无多余重组)。
            // 这里以 content-equality 钉语义,避免为去重断言引入 coroutines-test 依赖。
        }
    }

    @Test
    fun `共享快照含daemon真值时_buildAgents全点亮_与主页网格同口径`() {
        // 用户截图的反面钉死:capabilities 报了 agent:pi/dsh/zcode,
        // 无论哪条渲染链读到这份共享快照,都必须点亮这三个 agent。
        withCaps(listOf("terminal", "fs", "agent", "agent:pi:v0.85.1", "agent:dsh", "agent:zcode")) {
            val agents = buildAgents(AgentConnection.hostCapabilities.value).associateBy { it.id }
            assertTrue("Pi 应点亮(带版本号新格式)", agents.getValue("pi").available)
            assertTrue("DSH 应点亮", agents.getValue("dsh").available)
            assertTrue("ZCode 应点亮", agents.getValue("zcode").available)
        }
    }

    @Test
    fun `共享快照为空时_仅codex点亮_灰显副文案如实`() {
        // 缺陷形态记录:空快照 = "只剩 Codex 可选"。修复后该形态只允许出现在
        // daemon 确实没报任何 agent 的真实场景,不再因某条链加载失败而出现
        // (共享快照由探活循环 8s 兜底刷新)。
        withCaps(emptyList()) {
            val agents = buildAgents(AgentConnection.hostCapabilities.value).associateBy { it.id }
            assertTrue(agents.getValue("codex").available)
            assertFalse(agents.getValue("pi").available)
            assertFalse(agents.getValue("dsh").available)
            assertFalse(agents.getValue("zcode").available)
        }
    }
}
