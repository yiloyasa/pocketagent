package dev.pocketagent.app.ui.component

import dev.pocketagent.app.ui.ChatItem
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 历史浮层全文搜索(daemon SearchThreadMessages RPC 接入,R4-SR4)纯逻辑钉子:
 * 防抖归一化/去重、snippet→本地条目的跳转映射(时间/文本对齐,绝不拿 daemon
 * sequence 与本地 listIndex 做算术换算)、RPC 失败静默降级。
 *
 * 反转后的表现是"点全文命中跳错条/假跳到错误消息/打字每键一发 RPC",都不报错,
 * 故先钉死。RPC 一律经 ThreadSearcher 接口注入假实现,面板/逻辑层不碰 gRPC。
 */
class HistoryQuickNavSearchTest {

    private fun user(text: String, atMs: Long = 0) =
        ChatItem(role = "user", text = text, sentAtMs = atMs)

    private fun assistant(text: String, atMs: Long = 0) =
        ChatItem(role = "assistant", text = text, sentAtMs = atMs)

    private fun hit(
        sequence: Int = 0,
        role: String = "user",
        snippet: String,
        atMs: Long = 0,
    ) = HistoryQuickNavLogic.SearchHit(
        sequence = sequence, role = role, snippet = snippet, createdAtMs = atMs,
    )

    // ---- 防抖纯函数 ----

    @Test
    fun `防抖窗口400毫秒_命中上限30`() {
        assertEquals(400L, HistoryQuickNavLogic.SEARCH_DEBOUNCE_MS)
        assertEquals(30, HistoryQuickNavLogic.SEARCH_LIMIT)
    }

    @Test
    fun `searchableQuery_空白回null_其余去首尾空白`() {
        assertNull(HistoryQuickNavLogic.searchableQuery(""))
        assertNull(HistoryQuickNavLogic.searchableQuery("   \n "))
        assertEquals("daemon", HistoryQuickNavLogic.searchableQuery("  daemon "))
        assertEquals("搜索词", HistoryQuickNavLogic.searchableQuery("搜索词"))
    }

    @Test
    fun `shouldSkipFire_相同查询跳过_不同或首查不跳`() {
        assertTrue(HistoryQuickNavLogic.shouldSkipFire("abc", "abc"))
        assertEquals(false, HistoryQuickNavLogic.shouldSkipFire("abc", "abcd"))
        assertEquals(false, HistoryQuickNavLogic.shouldSkipFire("abc", null))
    }

    // ---- snippet 核提取 ----

    @Test
    fun `snippetCore_剥首尾省略号与空白_中间文本不动`() {
        assertEquals("匹配词前后文", HistoryQuickNavLogic.snippetCore("…匹配词前后文…"))
        assertEquals("匹配词前…后文", HistoryQuickNavLogic.snippetCore(" 匹配词前…后文 "))
        // ASCII 点同剥(daemon 若用三点风格也不至于匹配失败)
        assertEquals("core", HistoryQuickNavLogic.snippetCore("...core…"))
    }

    @Test
    fun `snippetCore_剥光后回空`() {
        assertEquals("", HistoryQuickNavLogic.snippetCore("…"))
        assertEquals("", HistoryQuickNavLogic.snippetCore(" ... "))
        assertEquals("", HistoryQuickNavLogic.snippetCore(""))
    }

    // ---- 命中→本地条目的跳转映射(locateHit)----

    @Test
    fun `created_at与文本双证_同文本多条取时间对的那条`() {
        val items = listOf(
            user("请帮我部署这个服务", atMs = 1_000),
            user("请帮我部署这个服务", atMs = 2_000),
            assistant("好的,开始部署", atMs = 2_100),
        )
        val idx = HistoryQuickNavLogic.locateHit(
            items, hit(snippet = "…部署这个服务成功了吗", atMs = 2_000), listOffset = 0,
        )
        assertEquals(1, idx)
    }

    @Test
    fun `仅文本证_本地时间未知或对不上也能定位`() {
        val items = listOf(
            user("第一条", atMs = 0),
            assistant("包含关键词 postgres 调优的回复", atMs = 5_000),
        )
        // hit 带时间但本地条目 sentAtMs=0:时间证失败,文本证兜底
        val idx = HistoryQuickNavLogic.locateHit(
            items, hit(role = "assistant", snippet = "…postgres 调优…", atMs = 9_999), 0,
        )
        assertEquals(1, idx)
    }

    @Test
    fun `文本前缀场景_命中在消息开头也能定位`() {
        // daemon 摘录从消息开头开始(无前导截断):核是正文前缀,包含匹配覆盖之
        val items = listOf(user("磁盘满了怎么清理", atMs = 7_000))
        val idx = HistoryQuickNavLogic.locateHit(
            items, hit(snippet = "磁盘满了怎么清理…", atMs = 0), 0,
        )
        assertEquals(0, idx)
    }

    @Test
    fun `仅时间证_核不在本地文本时兜底且跳过无正文占位项`() {
        val items = listOf(
            ChatItem(role = "user", text = "", sentAtMs = 3_000), // 附件占位项:同时间戳无正文
            user("正文在本地被截断了,不含核", atMs = 3_000),
        )
        val idx = HistoryQuickNavLogic.locateHit(
            items, hit(snippet = "…本地根本没这段文字…", atMs = 3_000), 0,
        )
        assertEquals(1, idx)
    }

    @Test
    fun `role不同不匹配_助手命中不落用户条目`() {
        val items = listOf(user("同样的文字", atMs = 1_000))
        val idx = HistoryQuickNavLogic.locateHit(
            items, hit(role = "assistant", snippet = "同样的文字", atMs = 1_000), 0,
        )
        assertNull(idx)
    }

    @Test
    fun `全不中回null_诚实降级不假跳`() {
        val items = listOf(user("已加载的消息", atMs = 1_000))
        // 时间未知 + 核不在任何已加载正文里 → 在更早的历史里
        val idx = HistoryQuickNavLogic.locateHit(
            items, hit(snippet = "…只有更早历史才有的话…", atMs = 0), 0,
        )
        assertNull(idx)
    }

    @Test
    fun `listOffset计入返回下标_头部偏移一位`() {
        val items = listOf(user("带关键词 beta 的指令", atMs = 1_000))
        val idx = HistoryQuickNavLogic.locateHit(
            items, hit(snippet = "关键词 beta"), listOffset = 1,
        )
        assertEquals(1, idx)
    }

    // ---- RPC 经接口注入(mock 用 ThreadSearcher,不碰 gRPC)----

    @Test
    fun `runSearch_命中经假实现原样返回`() {
        val fake = HistoryQuickNavLogic.ThreadSearcher { _, _ ->
            listOf(
                hit(sequence = 3, snippet = "第一条", atMs = 111),
                hit(sequence = 9, role = "assistant", snippet = "第二条", atMs = 222),
            )
        }
        val hits = kotlinx.coroutines.runBlocking {
            HistoryQuickNavLogic.runSearch(fake, "query", limit = 30)
        }
        assertEquals(2, hits.size)
        assertEquals(3, hits[0].sequence)
        assertEquals("assistant", hits[1].role)
        assertEquals(222L, hits[1].createdAtMs)
    }

    @Test
    fun `runSearch_RPC失败静默降级为空列表_不抛错`() {
        val broken = HistoryQuickNavLogic.ThreadSearcher { _, _ -> throw java.net.ConnectException("offline") }
        val hits = kotlinx.coroutines.runBlocking {
            HistoryQuickNavLogic.runSearch(broken, "query", limit = 30)
        }
        assertTrue(hits.isEmpty())
    }
}
