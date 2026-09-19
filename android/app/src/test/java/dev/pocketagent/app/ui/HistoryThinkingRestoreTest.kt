package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.ContentBlock
import pocketagent.v1.TextBlock
import pocketagent.v1.ThreadMessage
import pocketagent.v1.ThinkingBlock

/**
 * "隐藏推理过程持久化"的历史恢复侧钉子(设置页第 4 项)。
 *
 * 【钉住什么】daemon 修复后,assistant 消息的块序列 = [思考块, 正文块] 一起落库;
 * 重进会话 / 重启 app 后 GetThreadHistory 返回的就是它。思考块必须映射成
 * role="thinking" 的 ChatItem(ChatScreen 对该角色渲染 ThinkingCard),
 * 且排在正文之前 —— 时间线顺序与当初直播时一致。
 *
 * 【为什么钉它】映射分支(block.hasThinking())一旦被删/被改,现象不是报错而是
 * "思考卡静默消失" —— 与本项要修的用户反馈一模一样,回归时极难定位。
 * daemon 侧对应的落库钉子见 daemon internal/server/thread_thinking_test.go。
 */
class HistoryThinkingRestoreTest {

    @Test
    fun `历史恢复把思考块渲染成思考卡且在正文之前`() {
        val message = ThreadMessage.newBuilder()
            .setRole("assistant")
            .addBlocks(
                ContentBlock.newBuilder()
                    .setThinking(ThinkingBlock.newBuilder().setText("先看目录结构,再读主函数")),
            )
            .addBlocks(
                ContentBlock.newBuilder()
                    .setText(TextBlock.newBuilder().setText("结论:入口在 App.kt")),
            )
            .build()

        val items = historyMessageToChatItems(message)

        assertEquals(2, items.size)
        assertEquals("thinking", items[0].role)
        assertEquals("先看目录结构,再读主函数", items[0].text)
        assertEquals("assistant", items[1].role)
        assertEquals("结论:入口在 App.kt", items[1].text)
    }

    @Test
    fun `无思考块的旧消息不产生思考卡`() {
        // 修复前落库的历史消息只有正文:必须原样渲染,不得凭空造思考卡。
        val message = ThreadMessage.newBuilder()
            .setRole("assistant")
            .addBlocks(
                ContentBlock.newBuilder()
                    .setText(TextBlock.newBuilder().setText("旧的纯文本回复")),
            )
            .build()

        val items = historyMessageToChatItems(message)

        assertEquals(1, items.size)
        assertEquals("assistant", items[0].role)
        assertTrue(items.none { it.role == "thinking" })
    }
}
