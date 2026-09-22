package dev.pocketagent.app.ui

import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 子代理工具卡识别(情况 B:ZCode 不透出子代理内部对话,只透出 tool.updated
 * 的入参/结果)。识别依据与 daemon zcodeapp.ToolInputIsSubagent 同源:纯结构、
 * 不写死工具名,普通工具不误判。
 */
class SubagentToolCardTest {

    @Test
    fun agent_completed_input_is_subagent() {
        val args = """{"prompt":"撰写调研方案","description":"多 Agent 协同","agentType":"general-purpose","model":"glm-5.3"}"""
        assertTrue(looksLikeSubagentDelegation(args))
    }

    @Test
    fun agent_async_input_is_subagent() {
        val args = """{"prompt":"后台跑调研","description":"task","outputFile":"C:/a/task.output","subagent_type":"general-purpose"}"""
        assertTrue(looksLikeSubagentDelegation(args))
    }

    @Test
    fun nested_subagent_has_parent_tool_call_id() {
        val args = """{"prompt":"细分任务","parentToolCallId":"call_parent"}"""
        assertTrue(looksLikeSubagentDelegation(args))
    }

    @Test
    fun bash_tool_is_not_subagent() {
        val args = """{"command":"ls -la","description":"列出目录"}"""
        assertFalse(looksLikeSubagentDelegation(args))
    }

    @Test
    fun prompt_without_marker_is_not_subagent() {
        val args = """{"prompt":"只问一句话"}"""
        assertFalse(looksLikeSubagentDelegation(args))
    }

    @Test
    fun empty_args_is_not_subagent() {
        assertFalse(looksLikeSubagentDelegation(""))
        assertFalse(looksLikeSubagentDelegation("{}"))
    }
}
