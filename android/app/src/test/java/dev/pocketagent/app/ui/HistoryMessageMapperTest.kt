package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.ContentBlock
import pocketagent.v1.TextBlock
import pocketagent.v1.ThreadMessage
import pocketagent.v1.ToolCallBlock
import pocketagent.v1.ToolResultBlock
import pocketagent.v1.Usage
import pocketagent.v1.UsageUpdate

class HistoryMessageMapperTest {
    @Test
    fun keeps_text_and_tool_result_as_separate_chat_items() {
        val message = ThreadMessage.newBuilder()
            .setRole("assistant")
            .addBlocks(
                ContentBlock.newBuilder()
                    .setText(TextBlock.newBuilder().setText("先检查项目")),
            )
            .addBlocks(
                ContentBlock.newBuilder()
                    .setToolCall(
                        ToolCallBlock.newBuilder()
                            .setName("list_dir")
                            .setId("call-1"),
                    ),
            )
            .addBlocks(
                ContentBlock.newBuilder()
                    .setToolResult(
                        ToolResultBlock.newBuilder()
                            .setCallId("call-1")
                            .setOk(true)
                            .setOutput("main.go"),
                    ),
            )
            .build()

        val items = historyMessageToChatItems(message)

        assertEquals(3, items.size)
        assertEquals("assistant", items[0].role)
        assertEquals("先检查项目", items[0].text)
        assertEquals("list_dir", items[1].toolName)
        assertTrue(items[1].text.isEmpty())
        assertEquals("main.go", items[2].text)
    }

    @Test
    fun maps_usage_updates_to_a_visible_timeline_item() {
        val item = usageUpdateToChatItem(
            UsageUpdate.newBuilder()
                .setUsage(
                    Usage.newBuilder()
                        .setInputTokens(120)
                        .setOutputTokens(45)
                        .build(),
                )
                .setTotalUsage(
                    Usage.newBuilder()
                        .setInputTokens(1200)
                        .setOutputTokens(450)
                        .build(),
                )
                .build(),
        )

        assertEquals("usage", item.role)
        assertTrue(item.text.contains("输入 120"))
        assertTrue(item.text.contains("输出 45"))
        assertTrue(item.text.contains("累计输入 1200"))
    }

    @Test
    fun omits_total_section_when_total_usage_unset() {
        // 历史回放 hermes 的 usage_update 从不设置 total_usage:proto 消息字段未设置
        // 时 getter 返回默认实例而非 null,`?:` 防不住 —— 必须像 R4-T4 一样用
        // hasTotalUsage() 判"没设置",未设置就不写"累计"段,而不是把"没报累计"
        // 渲染成"累计输入 0 · 输出 0"。
        val item = usageUpdateToChatItem(
            UsageUpdate.newBuilder()
                .setUsage(
                    Usage.newBuilder()
                        .setInputTokens(30)
                        .setOutputTokens(12)
                        .build(),
                )
                .build(),
        )

        assertEquals("usage", item.role)
        assertTrue(item.text.contains("输入 30"))
        assertTrue(item.text.contains("输出 12"))
        assertFalse(item.text.contains("累计"))
    }
}
