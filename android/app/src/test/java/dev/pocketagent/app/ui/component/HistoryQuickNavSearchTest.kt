package dev.pocketagent.app.ui.component

import dev.pocketagent.app.ui.ChatItem
import dev.pocketagent.app.ui.history.HistorySearchHit
import dev.pocketagent.app.ui.history.HistorySearchRepository
import dev.pocketagent.app.ui.history.HistorySearchStatus
import dev.pocketagent.app.ui.history.ResultsKind
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.SearchThreadMessagesResponse
import pocketagent.v1.ThreadMessageHit

/**
 * 历史浮层全文搜索(Task 7 升级后)的接线面钉子:防抖归一化/去重、snippet→本地
 * 条目的"已加载兜底"跳转映射(时间/文本对齐,绝不拿 daemon sequence 与本地
 * listIndex 做算术换算)、仓库搜索的 proto→纯模型映射与失败人话归一。
 *
 * 与旧版的关键差异:RPC 失败不再静默降级为空列表(那会把"没搜成"伪装成"没有"),
 * 而是归一为 RemoteError 如实呈现;命中模型升级为 message_id 主键 + ranges
 * (HistorySearchHit),五态归一在 HistorySearchState/Repository(JVM 钉子在
 * HistorySearchStateTest)。
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
    ) = HistorySearchHit(
        messageId = "", turnId = "", sequence = sequence, role = role,
        snippet = snippet, createdAtMs = atMs,
    )

    // ---- 防抖纯函数(浮层搜索框,升级前后不变) ----

    @Test
    fun `防抖窗口400毫秒_命中上限30`() {
        assertEquals(400L, HistoryQuickNavLogic.SEARCH_DEBOUNCE_MS)
        assertEquals(30, HistoryQuickNavLogic.SEARCH_LIMIT)
        assertEquals(30, HistorySearchRepository.DEFAULT_LIMIT)
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

    // ---- snippet 核提取(已加载兜底定位的文本证据) ----

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

    // ---- 命中→本地条目的"已加载兜底"映射(locateHit)----

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
    fun `全不中回null_诚实降级_上游转自动拉取`() {
        val items = listOf(user("已加载的消息", atMs = 1_000))
        // 时间未知 + 核不在任何已加载正文里 → 在更早的历史里:
        // Task 7 起这里不再是终点,宿主会按裁定自动请求目标历史片段后重定位
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

    // ---- 仓库搜索:proto→纯模型映射 + 失败人话(替代旧 runSearch 静默降级) ----

    private fun rpcResp(vararg hits: ThreadMessageHit, hasMore: Boolean = false, searched: Int = 2) =
        SearchThreadMessagesResponse.newBuilder()
            .setScope("full_thread")
            .setHasMore(hasMore)
            .setSearchedMessages(searched)
            .addAllHits(hits.toList())
            .build()

    @Test
    fun `search_命中经假RPC原样映射_messageId与权威has_more不丢`() {
        val resp = rpcResp(
            ThreadMessageHit.newBuilder()
                .setSequence(3).setRole("user").setSnippet("…第一条…")
                .setCreatedAtUnixMs(111).setMessageId("m3").setTurnId("t3")
                .build(),
            ThreadMessageHit.newBuilder()
                .setSequence(9).setRole("assistant").setSnippet("第二条")
                .setCreatedAtUnixMs(222)
                .build(),
            hasMore = true,
        )
        val status = runBlocking {
            HistorySearchRepository(
                threadId = "t",
                searchRpc = { _, _, _ -> resp },
            ).search("query")
        }
        assertTrue(status is HistorySearchStatus.Results)
        status as HistorySearchStatus.Results
        assertEquals(ResultsKind.FULL_THREAD, status.kind)
        assertTrue(status.hasMore) // 权威分页位,原样透传
        assertEquals(2, status.hits.size)
        assertEquals("m3", status.hits[0].messageId)
        assertEquals("t3", status.hits[0].turnId)
        assertEquals("assistant", status.hits[1].role)
        assertEquals(222L, status.hits[1].createdAtMs)
    }

    @Test
    fun `search_RPC失败归一RemoteError_不伪装空结果_人话可读`() {
        val status = runBlocking {
            HistorySearchRepository(
                threadId = "t",
                searchRpc = { _, _, _ -> throw java.net.ConnectException("offline") },
            ).search("query")
        }
        assertTrue(status is HistorySearchStatus.RemoteError)
        status as HistorySearchStatus.RemoteError
        assertTrue(status.reason.isNotBlank())
        assertTrue(!status.reason.contains("ConnectException"))
    }

    @Test
    fun `fetchTargetMessages_经历史通道按轮拉取_自动定位用`() {
        val repo = HistorySearchRepository(
            threadId = "t",
            searchRpc = { _, _, _ -> rpcResp() },
            historyRpc = { rounds ->
                assertEquals(21, rounds)
                pocketagent.v1.GetThreadHistoryResponse.newBuilder().build()
            },
        )
        val msgs = runBlocking { repo.fetchTargetMessages(21) }
        assertTrue(msgs.isEmpty())
    }
}
