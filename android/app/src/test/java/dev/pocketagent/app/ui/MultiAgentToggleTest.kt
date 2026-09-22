package dev.pocketagent.app.ui

// MultiAgentToggleTest.kt —— 「多 agent 协同」真实状态的回归钉子。
//
// 【背景】旧实现是个"可拨但功能未实现"的假开关（用户实锤：开关可拨 = 假可用），
// 由 multiAgentToggleResult 这个恒关闸门兜着。现在的协同状态**完全由 daemon 的
// 接线事实驱动**（capabilities 里的 collab:<id>），因此本文件钉的是提取与文案：
//  1. 只认 collab: 前缀，其余能力（agent:/text_generation:/terminal…）一律不算；
//  2. 去重且稳定排序（capabilities 是集合，展示必须可复现）；
//  3. 空态如实说明"尚未接入"并给出下一步，**不得再出现"开发中"口径**；
//  4. 有接入时把人话说全（谁接入了 + 能派子智能体），而不是只亮一个灯。

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class MultiAgentToggleTest {

    @Test
    fun 只认collab前缀() {
        val caps = listOf(
            "agent:pi", "agent:codex", "collab:pi", "collab:dsh",
            "text_generation:pi", "terminal", "fs", "git",
        )
        assertEquals(listOf("dsh", "pi"), resolveCollabAgentIds(caps))
    }

    @Test
    fun 无agent接入时为空() {
        assertEquals(
            emptyList<String>(),
            resolveCollabAgentIds(listOf("agent:pi", "agent:dsh", "terminal")),
        )
    }

    @Test
    fun 去重并按字典序稳定输出() {
        assertEquals(
            listOf("dsh", "pi"),
            resolveCollabAgentIds(listOf("collab:pi", "collab:dsh", "collab:pi")),
        )
    }

    @Test
    fun 有接入时点名并说明能力() {
        val s = collabSubtitle(listOf("dsh", "pi"))
        assertTrue("应点名已接入的 agent, got=$s", s.contains("dsh") && s.contains("pi"))
        assertTrue("应说明能派子智能体, got=$s", s.contains("子智能体"))
    }

    @Test
    fun 空态说明原因而非开发中() {
        val s = collabSubtitle(emptyList())
        assertTrue("空态应说明为什么没有, got=$s", s.contains("尚未接入"))
        assertFalse("不得再出现'开发中'口径(假可用的旧口径), got=$s", s.contains("开发中"))
    }
}
