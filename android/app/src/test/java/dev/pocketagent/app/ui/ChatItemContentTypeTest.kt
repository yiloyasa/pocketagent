package dev.pocketagent.app.ui

import dev.pocketagent.app.ui.component.TerminalBlockKind
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotEquals
import org.junit.Test

/**
 * 【PF4 组合热点】时间线 LazyColumn 的 contentType 分桶纪律(ChatScreen.chatItemContentType):
 * 桶 = 渲染形态。常规主题按 role 一对一;黑客终端按 terminalBlockKind 聚合。
 * 这里钉死映射,防止后续新增 role 时把不同形态归进同一回收桶(跨形态复用 = 组合互毁)。
 */
class ChatItemContentTypeTest {

    private fun item(role: String) = ChatItem(role = role, text = "x")

    @Test
    fun 常规模式_同role同桶() {
        assertEquals(
            chatItemContentType(false, item("assistant")),
            chatItemContentType(false, item("assistant")),
        )
    }

    @Test
    fun 常规模式_不同role不同桶() {
        val roles = listOf("user", "assistant", "tool", "thinking", "plan", "codex_event", "usage")
        val buckets = roles.map { chatItemContentType(false, item(it)) }
        // 两两互异:role 一对一映射 when(role) 渲染分支
        assertEquals(buckets.size, buckets.distinct().size)
    }

    @Test
    fun 黑客模式_桶与terminalBlockKind对齐() {
        assertEquals(TerminalBlockKind.USER, chatItemContentType(true, item("user")))
        assertEquals(TerminalBlockKind.TOOL, chatItemContentType(true, item("tool")))
        assertEquals(TerminalBlockKind.OUTPUT, chatItemContentType(true, item("assistant")))
        assertEquals(TerminalBlockKind.KEEP_EXISTING, chatItemContentType(true, item("thinking")))
        assertEquals(TerminalBlockKind.KEEP_EXISTING, chatItemContentType(true, item("codex_event")))
        assertEquals(TerminalBlockKind.SKIP, chatItemContentType(true, item("plan")))
        assertEquals(TerminalBlockKind.SKIP, chatItemContentType(true, item("usage")))
    }

    @Test
    fun 黑客模式_不同形态不同桶() {
        assertNotEquals(
            chatItemContentType(true, item("user")),
            chatItemContentType(true, item("assistant")),
        )
        assertNotEquals(
            chatItemContentType(true, item("tool")),
            chatItemContentType(true, item("thinking")),
        )
    }

    @Test
    fun 两种模式_桶空间互不串扰() {
        // 常规桶是 role 字符串、黑客桶是枚举,类型不同天然不相等(双主题不共用回收池)
        assertNotEquals(
            chatItemContentType(false, item("user")),
            chatItemContentType(true, item("user")),
        )
    }
}
