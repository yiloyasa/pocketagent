package dev.pocketagent.app.ui.history

import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.MatchRange
import pocketagent.v1.SearchThreadMessagesResponse
import pocketagent.v1.ThreadMessageHit

/**
 * 历史全文搜索状态机/定位决策/高亮契约的 JVM 钉子(Task 7)。
 *
 * 反转后的表现全部静默:失败伪装成空结果、message_id 退化为下标算术假跳、
 * 高亮在中文/emoji 上错位、缺口自动拉取变成无底洞 —— 这里按 plan Review Focus
 * 逐条钉死。daemon 契约(只读)见 internal/server/thread_search.go:ranges 坐标 =
 * 消息全部文本块按 "\n" 拼接后的 rune 半开区间;from_loaded_history 恒 false;
 * message_id 空 = 退 sequence 兜底;has_more 是权威分页位。
 */
class HistorySearchStateTest {

    private fun hit(
        id: String = "",
        turnId: String = "",
        sequence: Int = 0,
        role: String = "user",
        snippet: String = "",
        atMs: Long = 0,
        ranges: List<HistoryMatchSpan> = emptyList(),
        fromLoaded: Boolean = false,
    ) = HistorySearchHit(
        messageId = id, turnId = turnId, sequence = sequence, role = role,
        snippet = snippet, createdAtMs = atMs, ranges = ranges, fromLoadedHistory = fromLoaded,
    )

    private fun span(
        id: String,
        sequenceInResponse: Int,
        itemStart: Int,
        role: String = "user",
    ) = MessageSpan(
        messageId = id, turnId = "t-$id", sequenceInResponse = sequenceInResponse,
        itemStart = itemStart, itemCount = 1, role = role,
    )

    // ---- 五态区分:resultsKindOf(scope, hits) ----

    @Test
    fun `五态_全量回包且未确认已加载_归全会话结果`() {
        val hits = listOf(hit(id = "m1", fromLoaded = false))
        assertEquals(
            ResultsKind.FULL_THREAD,
            HistorySearchState.resultsKindOf("full_thread", hits),
        )
    }

    @Test
    fun `五态_全部命中确认已加载_归本页结果`() {
        val hits = listOf(hit(id = "m1", fromLoaded = true), hit(id = "m2", fromLoaded = true))
        assertEquals(
            ResultsKind.PAGE_LOADED,
            HistorySearchState.resultsKindOf("full_thread", hits),
        )
    }

    @Test
    fun `五态_部分命中确认已加载_不归本页结果_保守口径`() {
        val hits = listOf(hit(id = "m1", fromLoaded = true), hit(id = "m2", fromLoaded = false))
        assertEquals(
            ResultsKind.FULL_THREAD,
            HistorySearchState.resultsKindOf("full_thread", hits),
        )
    }

    @Test
    fun `五态_scope为local_snapshot_归仅已下载_优先于from_loaded`() {
        assertEquals(
            ResultsKind.DOWNLOADED_ONLY,
            HistorySearchState.resultsKindOf("local_snapshot", listOf(hit(id = "m1", fromLoaded = true))),
        )
    }

    @Test
    fun `五态_scope为空_范围未知标注_且不臆断full_thread口径`() {
        assertTrue(!HistorySearchState.scopeKnown(""))
        assertTrue(HistorySearchState.scopeKnown("full_thread"))
        // 空 scope + fromLoaded=true 的命中仍可归本页结果(证据在命中位,不在 scope)
        assertEquals(
            ResultsKind.PAGE_LOADED,
            HistorySearchState.resultsKindOf("", listOf(hit(id = "m1", fromLoaded = true))),
        )
    }

    @Test
    fun `五态_仓库映射_零命中回NoResults_远端失败回RemoteError_绝不伪装空结果`() {
        val empty = SearchThreadMessagesResponse.newBuilder()
            .setScope("full_thread").setHasMore(false).setSearchedMessages(7).build()
        val noResults = HistorySearchRepository.mapResponse(empty, "查询词")
        assertTrue(noResults is HistorySearchStatus.NoResults)
        noResults as HistorySearchStatus.NoResults
        assertEquals("查询词", noResults.query)
        assertEquals(ResultsKind.FULL_THREAD, noResults.kind)
        assertTrue(noResults.scopeKnown)

        val failed = runBlocking {
            HistorySearchRepository(
                threadId = "t",
                searchRpc = { _, _, _ -> throw java.net.ConnectException("offline") },
            ).search("查询词")
        }
        assertTrue(failed is HistorySearchStatus.RemoteError)
        failed as HistorySearchStatus.RemoteError
        assertTrue(failed.reason.isNotBlank()) // 人话,不是空串/裸异常名
        assertTrue(!failed.reason.contains("ConnectException"))
    }

    @Test
    fun `五态_仓库映射_has_more权威透传_messageId_turnId_ranges透传_异常范围丢弃`() {
        val resp = SearchThreadMessagesResponse.newBuilder()
            .setScope("full_thread")
            .setHasMore(true)
            .setSearchedMessages(42)
            .addHits(
                ThreadMessageHit.newBuilder()
                    .setSequence(3).setRole("assistant").setSnippet("…命中摘录…")
                    .setCreatedAtUnixMs(111).setMessageId("m3").setTurnId("turn-9")
                    .setFromLoadedHistory(false)
                    .addRanges(MatchRange.newBuilder().setStart(2).setEnd(5))
                    .addRanges(MatchRange.newBuilder().setStart(-1).setEnd(0)) // 异常:丢弃
                    .addRanges(MatchRange.newBuilder().setStart(7).setEnd(7)) // 空:丢弃
                    .addRanges(MatchRange.newBuilder().setStart(-2).setEnd(4)), // 负起点:钳到 0
            )
            .build()
        val status = HistorySearchRepository.mapResponse(resp, "命中")
        assertTrue(status is HistorySearchStatus.Results)
        status as HistorySearchStatus.Results
        assertTrue(status.hasMore) // 权威分页位原样透传
        assertEquals(42, status.searchedMessages)
        assertEquals(1, status.hits.size)
        val h = status.hits[0]
        assertEquals("m3", h.messageId)
        assertEquals("turn-9", h.turnId)
        assertEquals(listOf(0 to 4, 2 to 5), h.ranges.map { it.start to it.end }.sortedBy { it.first })
    }

    // ---- 命中去重:message_id 主键,空 ID 退 sequence ----

    @Test
    fun `去重_同messageId保留首条_空id按sequence去重_两通道互不污染`() {
        val hits = listOf(
            hit(id = "A", sequence = 1),
            hit(id = "A", sequence = 2), // 同 id:去
            hit(sequence = 3), // 空 id,seq 3:留
            hit(sequence = 3), // 空 id,同 seq:去
            hit(id = "B", sequence = 3), // 有 id,seq 撞了空 id 通道:留
        )
        val kept = HistorySearchState.dedupeHits(hits)
        assertEquals(listOf("A", "", "B"), kept.map { it.messageId })
        assertEquals(listOf(1, 3, 3), kept.map { it.sequence })
    }

    // ---- 定位键:message_id 为主,sequence 仅兼容 ----

    @Test
    fun `定位键_有messageId走ById_空退sequence`() {
        assertEquals(
            HistoryLocateKey.ById("m1"),
            HistorySearchState.locateKeyOf(hit(id = "m1", sequence = 9)),
        )
        assertEquals(
            HistoryLocateKey.BySequence(9),
            HistorySearchState.locateKeyOf(hit(id = "", sequence = 9)),
        )
    }

    @Test
    fun `message_id定位_命中身份索引即回跨度_角色不符回null不假跳`() {
        val identity = HistoryIdentityIndex(
            spans = listOf(span("m3", 0, itemStart = 4), span("m4", 1, itemStart = 7, role = "assistant")),
            knownTotal = 5,
        )
        val found = HistorySearchState.locateLoadedSpan(hit(id = "m3"), identity)
        assertEquals(4, found?.itemStart)
        // 主键命中文本但角色不符 = 索引过期(回滚/分叉换过列表):回 null 走拉取
        assertNull(HistorySearchState.locateLoadedSpan(hit(id = "m3", role = "assistant"), identity))
        assertNull(HistorySearchState.locateLoadedSpan(hit(id = "不存在"), identity))
    }

    @Test
    fun `空ID退sequence_坐标由knownTotal撑起_未加载头部对不上回null`() {
        // 已加载的是全量尾部 2 条(全量 5 条):daemon 下标 3..4
        val identity = HistoryIdentityIndex(
            spans = listOf(span("", 0, itemStart = 0), span("", 1, itemStart = 3)),
            knownTotal = 5,
        )
        assertEquals(3, identity.firstDaemonIndex)
        val found = HistorySearchState.locateLoadedSpan(hit(sequence = 4, snippet = "x"), identity)
        assertEquals(3, found?.itemStart)
        // sequence=1 在未下载的头部:超出已加载段,回 null(走缺口拉取,不猜)
        assertNull(HistorySearchState.locateLoadedSpan(hit(sequence = 1, snippet = "x"), identity))
    }

    @Test
    fun `空ID退sequence_坐标系不可信时不参与_宁可拉取不假跳`() {
        // 没搜过(knownTotal=0)且历史被截断:尾部下标 ≠ 全量下标,sequence 兜底禁用
        val identity = HistoryIdentityIndex(
            spans = listOf(span("", 0, itemStart = 0)),
            knownTotal = 0,
            fullLoaded = false,
        )
        assertTrue(!identity.sequenceTrusted)
        assertNull(HistorySearchState.locateLoadedSpan(hit(sequence = 0, snippet = "x"), identity))
        // 已加载即全量(fullLoaded):0 基下标可信
        val full = identity.copy(fullLoaded = true)
        assertEquals(0, HistorySearchState.locateLoadedSpan(hit(sequence = 0, snippet = "x"), full)?.itemStart)
    }

    // ---- 缺口自动拉取决策 ----

    @Test
    fun `自动定位_已加载命中直接Jump_不做算术拉取`() {
        val identity = HistoryIdentityIndex(
            spans = listOf(span("m1", 0, itemStart = 6)), knownTotal = 10,
        )
        val plan = HistorySearchState.planHitLocate(
            hit(id = "m1"), identity, currentRounds = 20, stepRounds = 10,
        )
        assertEquals(HistoryLocatePlan.Jump(6), plan)
    }

    @Test
    fun `自动定位_有总数_精确轮数一轮必中_如seq9共30条则请求21轮`() {
        val plan = HistorySearchState.planHitLocate(
            hit(id = "不在", sequence = 9), null, currentRounds = 20, stepRounds = 10,
        )
        assertEquals(HistoryLocatePlan.FetchTarget(21, exact = true), plan)
    }

    @Test
    fun `自动定位_精确轮数钳到上限_防超大线程整段拉爆`() {
        val plan = HistorySearchState.planHitLocate(
            hit(id = "不在", sequence = 1), HistoryIdentityIndex(knownTotal = 10_000),
            currentRounds = 20, stepRounds = 10,
        )
        assertEquals(HistoryLocatePlan.FetchTarget(200, exact = true), plan)
        assertEquals(200, HistorySearchState.roundsToCoverSequence(1, 10_000))
    }

    @Test
    fun `自动定位_无总数_旧daemon按档位步进探索_到顶给人话`() {
        val step = HistorySearchState.planHitLocate(
            hit(id = "", sequence = 3), null, currentRounds = 20, stepRounds = 10,
        )
        assertEquals(HistoryLocatePlan.FetchTarget(30, exact = false), step)
        val capped = HistorySearchState.planHitLocate(
            hit(id = "", sequence = 3), null, currentRounds = 195, stepRounds = 10,
        )
        assertTrue(capped is HistoryLocatePlan.Unresolvable)
    }

    // ---- ranges 高亮:拼接空间契约(中文/emoji/跨块/装饰) ----

    @Test
    fun `拼接契约_多文本块按换行相连_与daemon同一坐标系`() {
        assertEquals("前半匹配\n词后半", HistorySearchState.concatenatedSearchText(listOf("前半匹配", "词后半")))
        assertEquals("", HistorySearchState.concatenatedSearchText(emptyList()))
    }

    @Test
    fun `高亮_中文rune坐标_半开区间切段`() {
        // "晴天匹配词雨天":匹配词 = rune [2,5)
        val spans = HistorySearchState.snippetHighlightSpans(
            "晴天匹配词雨天",
            listOf(HistoryMatchSpan(2, 5)),
            "匹配词",
        )
        assertEquals(listOf(IntRange(2, 4)), spans)
    }

    @Test
    fun `高亮_emoji等代理对_rune坐标换算UTF16不错位`() {
        // "😀匹配词尾":😀 占 2 个 UTF-16 码元、1 个 rune;匹配词 = rune [1,4) → UTF-16 [2,5)
        val spans = HistorySearchState.snippetHighlightSpans(
            "😀匹配词尾",
            listOf(HistoryMatchSpan(1, 4)),
            "匹配词",
        )
        assertEquals(listOf(IntRange(2, 4)), spans)
        val s = "😀匹配词尾"
        assertEquals("匹配词", s.substring(spans[0].first, spans[0].last + 1))
    }

    @Test
    fun `高亮_跨块命中_在拼接空间切段_分隔符留在段内`() {
        val concat = HistorySearchState.concatenatedSearchText(listOf("前半匹配", "词后半"))
        // daemon 语义:"匹配\n词" = 拼接串 rune [2,6) —— 分隔符 \n 占 rune 4,
        // 跨块词被顶到 rune 5(前半匹配=0..3,\n=4,词后半=5..7)
        val spans = HistorySearchState.snippetHighlightSpans(
            concat, listOf(HistoryMatchSpan(2, 6)), "匹配词",
        )
        assertEquals(listOf(IntRange(2, 5)), spans)
        assertEquals("匹配\n词", concat.substring(spans[0].first, spans[0].last + 1))
    }

    @Test
    fun `高亮_摘录带截断装饰_剥壳映射回原样坐标`() {
        // daemon:摘录 = 全文窗口,截断处补单个 …;匹配词在全文 rune [2,5)
        val spans = HistorySearchState.snippetHighlightSpans(
            "…前文匹配词后文…",
            listOf(HistoryMatchSpan(2, 5)),
            "匹配词",
        )
        assertEquals(listOf(IntRange(3, 5)), spans)
        val s = "…前文匹配词后文…"
        assertEquals("匹配词", s.substring(spans[0].first, spans[0].last + 1))
    }

    @Test
    fun `高亮_大小写折叠与daemon同口径_摘录窗外范围丢弃_无范围不画`() {
        // 折叠:daemon EqualFold 找到的 "abc",这里同样命中
        assertEquals(
            listOf(IntRange(0, 2)),
            HistorySearchState.snippetHighlightSpans(
                "abc DEF", listOf(HistoryMatchSpan(0, 3)), "ABC",
            ),
        )
        // 第 2 处在摘录窗外:丢弃,只画窗内的
        assertEquals(
            listOf(IntRange(0, 2)),
            HistorySearchState.snippetHighlightSpans(
                "匹配词", listOf(HistoryMatchSpan(0, 3), HistoryMatchSpan(10, 13)), "匹配词",
            ),
        )
        // 老 daemon 无 ranges / 空 query:不高亮,绝不乱画
        assertTrue(HistorySearchState.snippetHighlightSpans("匹配词", emptyList(), "匹配词").isEmpty())
        assertTrue(
            HistorySearchState.snippetHighlightSpans(
                "匹配词", listOf(HistoryMatchSpan(0, 3)), "",
            ).isEmpty(),
        )
    }

    // ---- 失败人话分类 ----

    @Test
    fun `失败人话_grpc状态与IO异常分类_不裸抛异常名`() {
        val notFound = io.grpc.Status.NOT_FOUND.withDescription("会话不存在")
            .asRuntimeException()
        assertTrue(HistorySearchRepository.failureReason(notFound).contains("不存在"))
        assertTrue(
            HistorySearchRepository.failureReason(java.net.ConnectException("x")).contains("连接"),
        )
        val unavailable = io.grpc.Status.UNAVAILABLE.asRuntimeException()
        assertTrue(HistorySearchRepository.failureReason(unavailable).contains("连不上"))
    }
}
