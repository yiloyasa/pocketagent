package dev.pocketagent.app.ui

import dev.pocketagent.app.ui.chat.userGroupNeighbors
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

    // ---- 【P0 崩溃修复 2026-10-09】userGroupNeighbors 安全读(竞态越界降级) ----

    @Test
    fun user_group_neighbors_normal_frame_matches_old_bare_index_semantics() {
        val roles = listOf("user", "user", "assistant", "user")
        // 中部:前邻用户成组、后邻非用户不成组,与旧式 `index>0 && roles[index-1]=="user"` 逐位同值
        assertEquals(true to false, userGroupNeighbors(roles, 1))
        // 非用户邻居不计入分组
        assertEquals(true to true, userGroupNeighbors(roles, 2))
        assertEquals(false to false, userGroupNeighbors(roles, 3))
        // 首条:无前邻居(getOrNull(-1)=null != "user")
        assertEquals(false to true, userGroupNeighbors(roles, 0))
    }

    @Test
    fun user_group_neighbors_race_frame_stale_index_into_empty_or_shrunk_list_is_false() {
        // 真机实崩形态:陈旧 index(46)打到已清空的快照(length 0)——降级"无邻居",绝不抛
        assertEquals(false to false, userGroupNeighbors(emptyList(), 46))
        assertEquals(false to false, userGroupNeighbors(emptyList(), 0))
        // 列表缩水(快照 60 条 → daemon 47 条)后的陈旧 index:越界一侧降级,
        // 界内一侧照常判定(shrunk = user/assistant/user,界 0..2)
        val shrunk = listOf("user", "assistant", "user")
        assertEquals(false to false, userGroupNeighbors(shrunk, 4))
        assertEquals(true to false, userGroupNeighbors(shrunk, 3))
        assertEquals(false to false, userGroupNeighbors(shrunk, 2))
    }
}
