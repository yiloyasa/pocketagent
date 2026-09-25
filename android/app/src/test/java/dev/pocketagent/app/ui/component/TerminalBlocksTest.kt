package dev.pocketagent.app.ui.component

import dev.pocketagent.app.ui.ChatItem
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 黑客终端模式主对话的纯函数钉子(Block 映射 / ❯ 前缀 / 脚注格式化 /
 * 工具状态符 / braille 帧序 / tmux 文案)。不触 Compose 运行时,JVM 可跑。
 */
class TerminalBlocksTest {

    // ───────── ChatItem → Block 映射 ─────────

    @Test
    fun block_mapping_covers_every_chat_item_role() {
        assertEquals(TerminalBlockKind.USER, terminalBlockKind(ChatItem(role = "user")))
        assertEquals(TerminalBlockKind.TOOL, terminalBlockKind(ChatItem(role = "tool")))
        assertEquals(TerminalBlockKind.OUTPUT, terminalBlockKind(ChatItem(role = "assistant")))
        // 无终端形态:保持既有渲染
        assertEquals(TerminalBlockKind.KEEP_EXISTING, terminalBlockKind(ChatItem(role = "thinking")))
        assertEquals(TerminalBlockKind.KEEP_EXISTING, terminalBlockKind(ChatItem(role = "codex_event")))
        // 不进时间线:plan 已上提为面板,usage 无正文
        assertEquals(TerminalBlockKind.SKIP, terminalBlockKind(ChatItem(role = "plan")))
        assertEquals(TerminalBlockKind.SKIP, terminalBlockKind(ChatItem(role = "usage")))
    }

    @Test
    fun unknown_role_falls_to_output_block_not_crash() {
        // 历史快照/未来新增的文本类 role 一律落输出 Block(枚举映射的兜底语义)
        assertEquals(TerminalBlockKind.OUTPUT, terminalBlockKind(ChatItem(role = "something_new")))
    }

    // ───────── ❯ 前缀构造 ─────────

    @Test
    fun prompt_prefix_appears_exactly_once_on_first_line() {
        val out = terminalUserPrompt("跑一下测试")
        assertTrue(out.startsWith("❯ "))
        assertEquals(1, out.split("❯").size - 1)
        assertEquals("❯ 跑一下测试", out)
    }

    @Test
    fun prompt_prefix_never_repeated_on_multiline_body() {
        val body = "第一行\n第二行\n第三行"
        val out = terminalUserPrompt(body)
        // ❯ 只在首行:第二/三行顶格,不做逐行提示符
        val lines = out.split("\n")
        assertEquals(3, lines.size)
        assertTrue(lines[0].startsWith("❯ "))
        assertFalse(lines[1].startsWith("❯"))
        assertFalse(lines[2].startsWith("❯"))
    }

    @Test
    fun composer_prompt_only_when_input_non_empty() {
        assertFalse(composerShowsPrompt(""))
        assertTrue(composerShowsPrompt("fix the bug"))
        // 空格也算非空输入(用户确实敲了东西)
        assertTrue(composerShowsPrompt(" "))
    }

    // ───────── 回合脚注格式化(有时长/无时长两态) ─────────

    @Test
    fun footnote_failure_is_bare_cross() {
        assertEquals("✗", formatTurnFootnote(success = false, elapsedMs = 0.0, outputTokens = 0))
        // 失败不给成功装饰,即便有时长
        assertEquals("✗", formatTurnFootnote(success = false, elapsedMs = 3200.0, outputTokens = 1200))
    }

    @Test
    fun footnote_with_duration_shows_seconds() {
        assertEquals("✓ 3.2s", formatTurnFootnote(true, elapsedMs = 3234.0, outputTokens = 0))
        assertEquals("✓ 0.5s", formatTurnFootnote(true, elapsedMs = 512.0, outputTokens = 0))
    }

    @Test
    fun footnote_without_duration_but_with_real_usage_shows_tokens() {
        assertEquals("✓ 1.2k tok", formatTurnFootnote(true, elapsedMs = 0.0, outputTokens = 1234))
    }

    @Test
    fun footnote_without_any_data_is_bare_check() {
        // 没有时长也没有用量:只显示 ✓,不编造数据
        assertEquals("✓", formatTurnFootnote(true, elapsedMs = 0.0, outputTokens = 0))
    }

    @Test
    fun footnote_duration_wins_over_tokens_when_both_present() {
        assertEquals("✓ 3.2s", formatTurnFootnote(true, elapsedMs = 3234.0, outputTokens = 1234))
    }

    @Test
    fun token_count_formatting() {
        assertEquals("0", formatTokenCount(0))
        assertEquals("999", formatTokenCount(999))
        assertEquals("1.0k", formatTokenCount(1000))
        assertEquals("1.2k", formatTokenCount(1234))
        assertEquals("1.2m", formatTokenCount(1_234_567))
    }

    // ───────── 工具 Block 状态符 ─────────

    @Test
    fun tool_running_status_is_empty_and_renders_spinner() {
        assertEquals("", terminalToolStatusMark("running", elapsedMs = 0.0))
        assertEquals("", terminalToolStatusMark("", elapsedMs = 0.0))
    }

    @Test
    fun tool_terminal_status_marks() {
        assertEquals("✓", terminalToolStatusMark("success", elapsedMs = 0.0))
        assertEquals("✓ 1.2s", terminalToolStatusMark("success", elapsedMs = 1234.0))
        // 没有退出码数据:失败只给 ✗,不编 "exit N"
        assertEquals("✗", terminalToolStatusMark("failure", elapsedMs = 0.0))
        assertEquals("✗ 0.5s", terminalToolStatusMark("failure", elapsedMs = 512.0))
    }

    @Test
    fun tool_args_summary_collapses_whitespace_and_truncates() {
        assertEquals("", terminalToolArgsSummary("   "))
        assertEquals("read a file", terminalToolArgsSummary("read\n   a\t file"))
        val long = "x".repeat(60)
        val out = terminalToolArgsSummary(long, maxLen = 48)
        assertEquals(49, out.length)
        assertTrue(out.endsWith("…"))
    }

    // ───────── braille 转轮帧序 ─────────

    @Test
    fun braille_frames_cycle_through_spec_sequence() {
        assertEquals('⠋', brailleFrameAt(0))
        assertEquals('⠏', brailleFrameAt(9))
        // 10 帧一循环:回绕后回到首帧
        assertEquals('⠋', brailleFrameAt(10))
        assertEquals(brailleFrameAt(0), brailleFrameAt(80))
        // 负数帧安全回绕
        assertEquals(brailleFrameAt(0), brailleFrameAt(-10))
    }

    // ───────── tmux 状态条文案 ─────────

    @Test
    fun tmux_conn_label_only_ok_reads_connected() {
        assertEquals("已连接", tmuxConnLabel("ok"))
        // checking 是巡检中间态、down 是已知断开:都读作重连中,与全 app 的
        // 「只有 down 算不在线」二值口径一致
        assertEquals("重连中", tmuxConnLabel("checking"))
        assertEquals("重连中", tmuxConnLabel("down"))
    }

    @Test
    fun tmux_clock_label_is_hhmm_and_ticks() {
        val now = 1_760_000_000_000L
        assertTrue(Regex("^\\d{2}:\\d{2}$").containsMatchIn(tmuxClockLabel(now)))
        // 30 分钟后的刻度不同(时钟在走,不是死值)
        assertNotEquals(tmuxClockLabel(now), tmuxClockLabel(now + 30 * 60 * 1000))
    }
}
