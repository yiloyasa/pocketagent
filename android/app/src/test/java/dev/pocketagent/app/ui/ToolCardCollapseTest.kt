package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * P0-4 常规模式工具卡折叠的判定纯函数(见 ChatScreen.kt「P0-4 常规模式工具卡折叠」区):
 * 默认折叠一行;红线卡(审批/反问)> 全局设置 > 子代理委派 三类例外;摘要只求一眼可辨。
 */
class ToolCardCollapseTest {

    @Test
    fun default_is_collapsed() {
        assertFalse(toolCardDefaultExpanded(userPrefersExpanded = false, isSubagent = false, isRedLine = false))
    }

    @Test
    fun red_line_card_never_collapses_regardless_of_other_inputs() {
        // 审批/反问类:等用户拍板的东西不能藏,连全局"折叠"设置也压不过它
        assertTrue(toolCardDefaultExpanded(userPrefersExpanded = false, isSubagent = false, isRedLine = true))
        assertTrue(toolCardDefaultExpanded(userPrefersExpanded = false, isSubagent = true, isRedLine = true))
    }

    @Test
    fun user_setting_and_subagent_expand() {
        assertTrue(toolCardDefaultExpanded(userPrefersExpanded = true, isSubagent = false, isRedLine = false))
        assertTrue(toolCardDefaultExpanded(userPrefersExpanded = false, isSubagent = true, isRedLine = false))
    }

    @Test
    fun bash_headline_takes_first_non_blank_command_line() {
        assertEquals(
            "git status",
            toolCallHeadline("Bash", """{"command":"git status","timeout":1000}"""),
        )
        assertEquals(
            "echo hi",
            toolCallHeadline("Bash", """{"command":"\n\necho hi\nls -la"}"""),
        )
    }

    @Test
    fun file_tool_headline_takes_bare_file_name() {
        assertEquals(
            "build.gradle.kts",
            toolCallHeadline("Read", """{"file_path":"/home/u/proj/app/build.gradle.kts"}"""),
        )
        assertEquals(
            "MainActivity.kt",
            toolCallHeadline("Edit", """{"file_path":"G:\\proj\\app\\MainActivity.kt","old_string":"a"}"""),
        )
    }

    @Test
    fun unknown_tool_yields_empty_headline() {
        // 摘要只对有把握的工具给;其余工具折叠行只显示工具名,绝不猜参数含义
        assertEquals("", toolCallHeadline("WebSearch", """{"query":"kotlin"}"""))
    }

    @Test
    fun malformed_or_blank_args_yield_empty_headline() {
        assertEquals("", toolCallHeadline("Bash", ""))
        assertEquals("", toolCallHeadline("Bash", "not-json"))
        assertEquals("", toolCallHeadline("Bash", """{"command":"   "}"""))
    }

    @Test
    fun long_headline_is_truncated_with_ellipsis() {
        val longCommand = "echo " + "a".repeat(60)
        val out = toolCallHeadline("Bash", """{"command":"$longCommand"}""")
        assertEquals(32 + 1, out.length)
        assertTrue(out.endsWith("…"))
    }

    @Test
    fun custom_max_len_is_honored() {
        val out = toolCallHeadline("Bash", """{"command":"echo abcdef"}""", maxLen = 5)
        assertEquals("echo …", out)
    }
}
