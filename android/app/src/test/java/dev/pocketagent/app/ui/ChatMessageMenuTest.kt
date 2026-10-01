package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * R2-A 消息长按上下文菜单的纯决策(见 ChatScreen.kt「R2-A 消息长按上下文菜单」区):
 * 动作按消息形态给(正文是前提;重试仅失败级)、摘要行报"哪条消息"、引用压成
 * Markdown 引文进草稿不发送。
 */
class ChatMessageMenuTest {

    // ---- chatMessageMenuActions ----

    @Test
    fun no_text_no_menu_even_if_retryable() {
        // 纯附件消息没有可复制/引用的东西;重试虽然与正文无关,但菜单整体不出现
        assertTrue(chatMessageMenuActions(retryable = true, hasText = false).isEmpty())
    }

    @Test
    fun plain_message_gets_copy_select_quote_in_order() {
        assertEquals(
            listOf("copy", "select", "quote"),
            chatMessageMenuActions(retryable = false, hasText = true),
        )
    }

    @Test
    fun retryable_appends_retry_last() {
        assertEquals(
            listOf("copy", "select", "quote", "retry"),
            chatMessageMenuActions(retryable = true, hasText = true),
        )
    }

    // ---- chatMessageMenuSummary ----

    @Test
    fun user_and_assistant_get_different_role_words() {
        assertTrue(chatMessageMenuSummary(ChatItem(role = "user", text = "帮我跑下测试")).startsWith("用户消息"))
        assertTrue(chatMessageMenuSummary(ChatItem(role = "assistant", text = "好的")).startsWith("回复"))
    }

    @Test
    fun empty_text_summary_is_role_word_only() {
        assertEquals("用户消息", chatMessageMenuSummary(ChatItem(role = "user", text = "")))
        assertEquals("回复", chatMessageMenuSummary(ChatItem(role = "assistant", text = "   ")))
    }

    @Test
    fun newlines_are_flattened_in_summary() {
        val s = chatMessageMenuSummary(ChatItem(role = "assistant", text = "第一行\n第二行"))
        assertEquals("回复 · 第一行 第二行", s)
    }

    @Test
    fun long_text_summary_is_clamped_with_ellipsis() {
        val s = chatMessageMenuSummary(ChatItem(role = "user", text = "字".repeat(40)))
        assertEquals("用户消息 · " + "字".repeat(24) + "…", s)
    }

    // ---- quoteMessageIntoDraft ----

    @Test
    fun empty_draft_becomes_quoted_block() {
        assertEquals("> 帮我看看这个\n", quoteMessageIntoDraft("", "帮我看看这个"))
    }

    @Test
    fun multiline_quote_prefixes_every_line() {
        assertEquals("> a\n> b\n", quoteMessageIntoDraft("", "  a  \nb"))
    }

    @Test
    fun existing_draft_is_separated_by_blank_line() {
        assertEquals(
            "帮我看看这个\n\n> 上一条回复\n",
            quoteMessageIntoDraft("帮我看看这个", "上一条回复"),
        )
    }

    @Test
    fun existing_draft_trailing_newlines_are_collapsed() {
        assertEquals(
            "草稿\n\n> 引文\n",
            quoteMessageIntoDraft("草稿\n\n\n", "引文"),
        )
    }

    @Test
    fun overlong_quote_is_clipped_with_ellipsis() {
        val out = quoteMessageIntoDraft("", "字".repeat(200))
        assertEquals("> " + "字".repeat(158) + "…\n", out)
    }

    @Test
    fun custom_max_quote_chars_is_honored() {
        val out = quoteMessageIntoDraft("", "abcdef", maxQuoteChars = 5)
        assertEquals("> abc…\n", out)
    }

    @Test
    fun blank_quoted_text_leaves_draft_untouched() {
        assertEquals("原草稿", quoteMessageIntoDraft("原草稿", "   \n  "))
    }
}
