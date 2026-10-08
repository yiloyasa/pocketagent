package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertSame
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 【C3】TurnSession State 空闲回收 + 时间线内存上限的纯函数裁定。
 *
 * 回收语义(TurnSession.evictableThreadIds):running/recovering 永不回收;
 * 最近访问的 6 个会话豁免(用户刚切走不能立刻丢);其余 30 分钟无访问回收。
 * 通知轮询(runningThreadIds)不调用 state(),天然不算访问 —— 这里钉死判定端。
 *
 * 截断语义(TurnSession.trimmedTimelineItems):超 400 条时中间截断,
 * 保头部 50 + 尾部 300,折叠段以 items_trimmed 标记行呈现;已折叠计数跨轮累计。
 */
class TurnSessionRecycleAndCapTest {

    private val now = 1_000_000L
    private val idle = TurnSession.SESSION_IDLE_RECYCLE_MS

    private fun info(
        tid: String,
        lastAccess: Long,
        active: Boolean = false,
    ) = TurnSession.SessionIdleInfo(tid, lastAccess, active)

    // ------------------------------------------------------------ 空闲回收

    @Test
    fun `空闲超时且不在最近保留名单内_回收`() {
        val evict = TurnSession.evictableThreadIds(
            // 7 个都空闲超时:最近 6 个豁免,第 7 个(最老)回收
            listOf(
                info("t1", now - idle - 1),
                info("t2", now - idle - 2),
                info("t3", now - idle - 3),
                info("t4", now - idle - 4),
                info("t5", now - idle - 5),
                info("t6", now - idle - 6),
                info("t7", now - idle - 7),
            ),
            nowMs = now,
        )
        assertEquals(listOf("t7"), evict)
    }

    @Test
    fun `最近 N 个访问的会话即使空闲超时也不回收`() {
        val sessions = (1..8).map { info("t$it", now - idle - it.toLong() * 60_000) }
        val evict = TurnSession.evictableThreadIds(sessions, nowMs = now, keepRecent = 6)
        // 豁免最近 6 个(t1..t6),只有最老的 t7/t8 超时回收
        assertEquals(setOf("t7", "t8"), evict.toSet())
    }

    @Test
    fun `running 或 recovering 的会话永不回收`() {
        val evict = TurnSession.evictableThreadIds(
            listOf(
                info("running", now - idle * 10, active = true),
                info("recovering", now - idle * 10, active = true),
                info("idle-old", now - idle * 10, active = false),
            ),
            nowMs = now,
        )
        // idle-old 是唯一非豁免会话,必然在"最近 6"里 —— 手动把 keepRecent 收到 0 隔离判定
        val isolated = TurnSession.evictableThreadIds(
            listOf(
                info("running", now - idle * 10, active = true),
                info("idle-old", now - idle * 10, active = false),
            ),
            nowMs = now,
            keepRecent = 0,
        )
        assertEquals(listOf("idle-old"), isolated)
        assertTrue("running/recovering 不得出现在任何回收名单里", "running" !in evict && "recovering" !in evict)
    }

    @Test
    fun `空闲未超时的会话不回收`() {
        val evict = TurnSession.evictableThreadIds(
            listOf(info("fresh", now - 1000)),
            nowMs = now,
            keepRecent = 0,
        )
        assertNull("刚访问过的会话被回收了", evict.firstOrNull())
    }

    // ------------------------------------------------------------ 时间线上限

    private fun bulk(count: Int, role: String = "assistant") =
        (0 until count).map { ChatItem(role, text = "m$it", id = "id$it") }

    @Test
    fun `未超上限返回原引用_零变化`() {
        val items = bulk(TurnSession.TIMELINE_MAX_ITEMS)
        assertSame("上限内的列表不应被复制替换", items, TurnSession.trimmedTimelineItems(items))
    }

    @Test
    fun `超上限中间截断_保头50尾300插折叠标记`() {
        val items = bulk(500)
        val trimmed = TurnSession.trimmedTimelineItems(items)
        // 结构:头 50 + 标记 1 + 尾 300 = 351
        assertEquals(351, trimmed.size)
        val marker = trimmed[TurnSession.TIMELINE_KEEP_HEAD]
        assertEquals("items_trimmed", marker.role)
        assertEquals(500 - 350, marker.trimmedCount)
        assertTrue(marker.id.isNotEmpty())
        // 头尾保持原序与原 id(已加载条目引用不变 → LazyColumn key 稳定)
        assertEquals("id0", trimmed.first().id)
        assertEquals("id499", trimmed.last().id)
        assertEquals(items.take(50), trimmed.take(50))
        assertEquals(items.takeLast(300), trimmed.takeLast(300))
    }

    @Test
    fun `再次截断_旧标记剔除且折叠数累计`() {
        // 先造一份已折叠 150 条的列表(头 50 + 标记 + 尾 300),头尾再各长出
        // 一批新内容,超限重截:旧标记不能重复出现,折叠数必须累计而不是清零。
        val first = TurnSession.trimmedTimelineItems(bulk(500))
        val grown = first.take(20) + bulk(200, "user") + first.drop(20)
        val retrimmed = TurnSession.trimmedTimelineItems(grown)
        val markers = retrimmed.filter { it.role == "items_trimmed" }
        assertEquals("折叠标记只能有一条", 1, markers.size)
        // 折叠数 = 本轮新折叠的原始条数 + 旧标记已折叠数(跨轮累计,不能清零)
        val newlyFolded = grown.count { it.role != "items_trimmed" } -
            retrimmed.count { it.role != "items_trimmed" }
        val carried = first.first { it.role == "items_trimmed" }.trimmedCount
        assertEquals(newlyFolded + carried, markers.single().trimmedCount)
    }

    // ------------------------------------------------------------ 封顶入口(P1-3)

    @Test
    fun `未超上限 enforceTimelineCap 零变化_不触发整表替换`() {
        // P1-3:封顶入口在未超限时应 O(1) 早退(旧实现每帧 toList() 整表复制后
        // 又原样换回)。行为级钉法:上限内条目逐条原样保留(同引用、同序)。
        val st = TurnSession.State()
        val items = bulk(10)
        st.items.addAll(items)
        TurnSession.enforceTimelineCap(st)
        assertEquals(10, st.items.size)
        assertEquals(items, st.items.toList())
    }

    @Test
    fun `超上限 enforceTimelineCap 中间截断`() {
        val st = TurnSession.State()
        st.items.addAll(bulk(TurnSession.TIMELINE_MAX_ITEMS + 100))
        TurnSession.enforceTimelineCap(st)
        // 头 50 + 折叠标记 1 + 尾 300 = 351
        assertEquals(351, st.items.size)
        assertEquals("items_trimmed", st.items[TurnSession.TIMELINE_KEEP_HEAD].role)
    }

    // ------------------------------------------------------------ 只读访问器(P1-4)

    @Test
    fun `peek 不存在的会话返回 null 且不创建 State`() {
        // P1-4:通知轮询改走 peek —— 不存在的 threadId 不许经 getOrPut 凭空拉起
        // State(旧实现走 state(),轮询把"登记过"的 id 全部实体化,states 无界增长)。
        val tid = "peek-never-exists-${System.nanoTime()}"
        assertNull(TurnSession.peek(tid))
    }

    @Test
    fun `peek 返回既有 State 且不刷新访问时刻`() {
        val tid = "peek-existing-${System.nanoTime()}"
        val st = TurnSession.state(tid) // state():创建并刷 lastAccessAt(既有语义)
        val accessBefore = st.lastAccessAt
        Thread.sleep(2) // 墙钟推进一点点,证明 peek 不写访问时刻
        assertSame(st, TurnSession.peek(tid))
        assertEquals("peek 不得刷新 lastAccessAt(通知轮询不算访问)", accessBefore, st.lastAccessAt)
    }

    // ------------------------------------------------------------ 稳定 id(C2)

    @Test
    fun `nextChatItemId 进程内唯一且带角色前缀`() {
        val a = nextChatItemId("user")
        val b = nextChatItemId("user")
        assertNotEquals(a, b)
        assertTrue(a.startsWith("ci-user-"))
    }

    @Test
    fun `历史条目 id 按 message_id 合成_重拉同消息同 id`() {
        assertEquals(historyChatItemId("msg-9", 0), historyChatItemId("msg-9", 0))
        assertEquals("hm-msg-9#1", historyChatItemId("msg-9", 1))
        // 旧 daemon 无 id:退进程级序号,每次构造不同(位置匹配语义)
        assertNotEquals(historyChatItemId("", 0), historyChatItemId("", 0))
    }

    @Test
    fun `历史回填助手为每条消息的每个块落稳定 id`() {
        val msg = pocketagent.v1.ThreadMessage.newBuilder()
            .setMessageId("m1")
            .setRole("assistant")
            .addBlocks(
                pocketagent.v1.ContentBlock.newBuilder()
                    .setText(pocketagent.v1.TextBlock.newBuilder().setText("a")),
            )
            .addBlocks(
                pocketagent.v1.ContentBlock.newBuilder()
                    .setText(pocketagent.v1.TextBlock.newBuilder().setText("b")),
            )
            .build()
        val items = historyChatItemsWithIds(listOf(msg))
        assertEquals(listOf("hm-m1#0", "hm-m1#1"), items.map { it.id })
    }
}
