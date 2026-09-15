package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertSame
import org.junit.Test
import pocketagent.v1.CodexEvent

class ChatTimelineTest {
    @Test
    fun codex_event_is_kept_as_a_chat_timeline_item() {
        val event = CodexEvent.newBuilder()
            .setKind(CodexEvent.Kind.ERROR)
            .setTitle("上游服务暂时不可用")
            .setMessage("请稍后重试")
            .setHttpStatus(503)
            .build()

        val item = codexEventToChatItem(event)

        assertEquals("codex_event", item.role)
        assertEquals("请稍后重试", item.text)
        assertSame(event, item.codexEvent)
    }
}
