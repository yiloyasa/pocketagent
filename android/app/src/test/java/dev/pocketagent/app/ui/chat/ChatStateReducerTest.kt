package dev.pocketagent.app.ui.chat

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertSame
import org.junit.Assert.assertTrue
import org.junit.Test
import dev.pocketagent.app.ui.HistoryRoundsLogic
import dev.pocketagent.app.ui.history.HistoryIdentityIndex
import dev.pocketagent.app.ui.history.MessageSpan

/**
 * ChatScreen 页面级状态 reducer 纯函数钉子(2026-10-04 计划 Task 8,第一层分层)。
 *
 * 覆盖面(每条与迁出前 ChatScreen 的原地赋值语义一一对应):
 *  1. initial:设置档位经 clamp 收敛(脏值→默认档);
 *  2. 离线快照三通道:ShowOfflineSnapshot(在线路径/断线路径 resetIdentity)、
 *     BackOnline(成对清)、ClearSnapshotFlag(只清标志)、ClearSnapshot(成对清);
 *  3. 历史尾部回包 HistoryTailLoaded:spans 整表重建、knownTotal 保留、
 *     最旧 id 无条件覆盖(含旧 daemon 空串)、截断位与 fullLoaded 成对;
 *  4. 头部拼接 planPrependCount + PrependApplied:锚点命中/条数兜底/零新增/
 *     spans 前插与 itemStart 平移/空 id 不覆盖锚点;
 *  5. 轮数推进:RoundsAdvanced 无条件、RoundsAdvancedIfBeyond 只进不退;
 *  6. TruncationSettled:截断位与身份索引可信位成对落位;
 *  7. 加载更早在飞闸 Started/Finished;
 *  8. KnownTotalObserved:0 不覆盖(旧 daemon 未报);
 *  9. listOffset 派生:与迁出前 `if (historyTruncated && !offlineSnapshot) 1 else 0` 同式。
 */
class ChatStateReducerTest {

    // ---- 构造辅助 ----

    private fun state(
        offlineSnapshot: Boolean = false,
        offlineSnapshotAt: Long = 0L,
        roundsLoaded: Int = 20,
        truncated: Boolean = false,
        loadingEarlier: Boolean = false,
        identity: HistoryIdentityIndex = HistoryIdentityIndex(),
        oldestMessageId: String = "",
    ): ChatState = ChatState(
        offlineSnapshot = offlineSnapshot,
        offlineSnapshotAt = offlineSnapshotAt,
        roundsLoaded = roundsLoaded,
        truncated = truncated,
        loadingEarlier = loadingEarlier,
        identity = identity,
        oldestMessageId = oldestMessageId,
    )

    private fun span(
        messageId: String,
        itemStart: Int,
        itemCount: Int = 1,
        role: String = "user",
    ): MessageSpan = MessageSpan(
        messageId = messageId,
        turnId = "turn-$messageId",
        sequenceInResponse = 0,
        itemStart = itemStart,
        itemCount = itemCount,
        role = role,
    )

    // ---- 1. initial ----

    @Test
    fun `initial 使用设置档位并归零其余状态`() {
        val s = ChatStateReducer.initial(30)
        assertEquals(30, s.roundsLoaded)
        assertFalse(s.offlineSnapshot)
        assertEquals(0L, s.offlineSnapshotAt)
        assertFalse(s.truncated)
        assertFalse(s.loadingEarlier)
        assertTrue(s.identity.spans.isEmpty())
        assertEquals("", s.oldestMessageId)
    }

    @Test
    fun `initial 脏档位收敛回默认档`() {
        assertEquals(HistoryRoundsLogic.DEFAULT_ROUNDS, ChatStateReducer.initial(7).roundsLoaded)
        assertEquals(10, ChatStateReducer.initial(10).roundsLoaded)
    }

    // ---- 2. 离线快照 ----

    @Test
    fun `在线路径快照只置位与落盘时间 不动身份索引`() {
        val base = state(identity = HistoryIdentityIndex(spans = listOf(span("m1", 0))), oldestMessageId = "m1")
        val s = ChatStateReducer.reduce(base, ChatIntent.ShowOfflineSnapshot(1728000000000L))
        assertTrue(s.offlineSnapshot)
        assertEquals(1728000000000L, s.offlineSnapshotAt)
        assertEquals(listOf("m1"), s.identity.spans.map { it.messageId })
        assertEquals("m1", s.oldestMessageId)
    }

    @Test
    fun `断线快照 resetIdentity 清空索引与锚点 绝不让旧索引指进快照列表`() {
        val base = state(
            identity = HistoryIdentityIndex(spans = listOf(span("m1", 0)), knownTotal = 42),
            oldestMessageId = "m1",
        )
        val s = ChatStateReducer.reduce(base, ChatIntent.ShowOfflineSnapshot(7L, resetIdentity = true))
        assertTrue(s.offlineSnapshot)
        assertEquals(7L, s.offlineSnapshotAt)
        assertTrue(s.identity.spans.isEmpty())
        assertEquals(0, s.identity.knownTotal)
        assertEquals("", s.oldestMessageId)
    }

    @Test
    fun `BackOnline 成对清标志与落盘时间 不动索引`() {
        val base = state(
            offlineSnapshot = true,
            offlineSnapshotAt = 7L,
            identity = HistoryIdentityIndex(spans = listOf(span("m1", 0))),
        )
        val s = ChatStateReducer.reduce(base, ChatIntent.BackOnline)
        assertFalse(s.offlineSnapshot)
        assertEquals(0L, s.offlineSnapshotAt)
        assertEquals(1, s.identity.spans.size)
    }

    @Test
    fun `错误条重试只清标志 保留落盘时间`() {
        val base = state(offlineSnapshot = true, offlineSnapshotAt = 123456L)
        val s = ChatStateReducer.reduce(base, ChatIntent.ClearSnapshotFlag)
        assertFalse(s.offlineSnapshot)
        assertEquals(123456L, s.offlineSnapshotAt)
    }

    @Test
    fun `横幅重载成对清标志与落盘时间`() {
        val base = state(offlineSnapshot = true, offlineSnapshotAt = 123456L)
        val s = ChatStateReducer.reduce(base, ChatIntent.ClearSnapshot)
        assertFalse(s.offlineSnapshot)
        assertEquals(0L, s.offlineSnapshotAt)
    }

    // ---- 3. 历史尾部回包 ----

    @Test
    fun `HistoryTailLoaded spans 重建 knownTotal 保留 截断位与可信位成对`() {
        val base = state(
            identity = HistoryIdentityIndex(spans = listOf(span("old", 0)), knownTotal = 99),
            oldestMessageId = "old",
        )
        val newSpans = listOf(span("a", 0), span("b", 1, role = "assistant"))
        val s = ChatStateReducer.reduce(
            base,
            ChatIntent.HistoryTailLoaded(spans = newSpans, oldestMessageId = "a", truncated = true),
        )
        assertEquals(listOf("a", "b"), s.identity.spans.map { it.messageId })
        assertEquals(99, s.identity.knownTotal)
        assertTrue(s.truncated)
        assertFalse(s.identity.fullLoaded)
        assertEquals("a", s.oldestMessageId)
    }

    @Test
    fun `HistoryTailLoaded 无截断时 fullLoaded 置位`() {
        val s = ChatStateReducer.reduce(
            state(),
            ChatIntent.HistoryTailLoaded(spans = listOf(span("a", 0)), oldestMessageId = "a", truncated = false),
        )
        assertFalse(s.truncated)
        assertTrue(s.identity.fullLoaded)
    }

    @Test
    fun `HistoryTailLoaded 最旧 id 无条件覆盖 旧 daemon 空串也照写`() {
        val base = state(oldestMessageId = "keep")
        val s = ChatStateReducer.reduce(
            base,
            ChatIntent.HistoryTailLoaded(spans = listOf(span("a", 0)), oldestMessageId = "", truncated = false),
        )
        assertEquals("", s.oldestMessageId)
    }

    // ---- 4. 头部拼接 ----

    @Test
    fun `planPrepend 锚点命中 只取锚点之前的新消息`() {
        val base = state(oldestMessageId = "b")
        // daemon 幂等回尾部:回包 = [新a, 已加载b, 已加载c],b 是锚点 → 只有 a 是新的
        val count = ChatStateReducer.planPrependCount(base, listOf("a", "b", "c"))
        assertEquals(1, count)
    }

    @Test
    fun `planPrepend 锚点缺失退条数兜底 负值钳零`() {
        // 旧 daemon 无 id(oldest 空):回包 5 条、已加载 3 条(identity 三枚 span)→ 新 2 条。
        // 空索引=已加载条数未知,planPrependCount 会把 5 条全记新(5-0)——两种口径都成立,
        // 本用例钉的是「条数兜底=回包−已加载」的算术,故显式带 3 枚 span。
        assertEquals(2, ChatStateReducer.planPrependCount(
            state(identity = HistoryIdentityIndex(spans = listOf(span("a", 0), span("b", 1), span("c", 2)))),
            listOf("x1", "x2", "x3", "x4", "x5")))
        // 回包比已加载还少(重复拉取/锚点全命中):零新增
        assertEquals(
            0,
            ChatStateReducer.planPrependCount(
                state(identity = HistoryIdentityIndex(spans = listOf(span("a", 0), span("b", 1), span("c", 2)))),
                listOf("a", "b"),
            ),
        )
    }

    @Test
    fun `planPrepend 全部重复 锚点在首位即零新增`() {
        val base = state(oldestMessageId = "a")
        assertEquals(0, ChatStateReducer.planPrependCount(base, listOf("a", "b", "c")))
    }

    @Test
    fun `PrependApplied spans 前插 既有 itemStart 整体平移`() {
        val base = state(
            identity = HistoryIdentityIndex(spans = listOf(span("b", 0), span("c", 1, itemCount = 2))),
            oldestMessageId = "b",
        )
        val s = ChatStateReducer.reduce(
            base,
            ChatIntent.PrependApplied(newSpans = listOf(span("a", 0)), newItemCount = 1, incomingOldestId = "a"),
        )
        val spans = s.identity.spans
        assertEquals(listOf("a", "b", "c"), spans.map { it.messageId })
        assertEquals(0, spans[0].itemStart)
        assertEquals(1, spans[1].itemStart)
        assertEquals(2, spans[2].itemStart)
        assertEquals("a", s.oldestMessageId)
    }

    @Test
    fun `PrependApplied 空 id 不覆盖旧锚点 旧 daemon 语义`() {
        val base = state(oldestMessageId = "keep")
        val s = ChatStateReducer.reduce(
            base,
            ChatIntent.PrependApplied(newSpans = listOf(span("a", 0)), newItemCount = 1, incomingOldestId = ""),
        )
        assertEquals("keep", s.oldestMessageId)
    }

    // ---- 5. 轮数推进 ----

    @Test
    fun `RoundsAdvanced 无条件覆盖 加载更早的档位累加`() {
        assertEquals(40, ChatStateReducer.reduce(state(roundsLoaded = 20), ChatIntent.RoundsAdvanced(40)).roundsLoaded)
        // 窄化也照写(与迁出前的直接赋值一致:调用方负责给 next)
        assertEquals(10, ChatStateReducer.reduce(state(roundsLoaded = 20), ChatIntent.RoundsAdvanced(10)).roundsLoaded)
    }

    @Test
    fun `RoundsAdvancedIfBeyond 只进不退 定位可能重复请求同一档位`() {
        val s = ChatStateReducer.reduce(state(roundsLoaded = 30), ChatIntent.RoundsAdvancedIfBeyond(50))
        assertEquals(50, s.roundsLoaded)
        val untouched = ChatStateReducer.reduce(state(roundsLoaded = 50), ChatIntent.RoundsAdvancedIfBeyond(30))
        assertEquals(50, untouched.roundsLoaded)
        // 未变化时原样返回(不触发无谓的 StateFlow 发射)
        val base = state(roundsLoaded = 50)
        assertSame(base, ChatStateReducer.reduce(base, ChatIntent.RoundsAdvancedIfBeyond(49)))
    }

    // ---- 6. 截断位 ----

    @Test
    fun `TruncationSettled 截断位与身份索引可信位成对`() {
        val base = state(
            truncated = true,
            identity = HistoryIdentityIndex(spans = listOf(span("a", 0)), fullLoaded = false),
        )
        val settled = ChatStateReducer.reduce(base, ChatIntent.TruncationSettled(false))
        assertFalse(settled.truncated)
        assertTrue(settled.identity.fullLoaded)

        val back = ChatStateReducer.reduce(settled, ChatIntent.TruncationSettled(true))
        assertTrue(back.truncated)
        assertFalse(back.identity.fullLoaded)
    }

    // ---- 7. 加载更早在飞闸 ----

    @Test
    fun `加载更早 Started 落闸 Finished 放闸`() {
        assertTrue(ChatStateReducer.reduce(state(), ChatIntent.LoadEarlierStarted).loadingEarlier)
        assertFalse(
            ChatStateReducer.reduce(state(loadingEarlier = true), ChatIntent.LoadEarlierFinished).loadingEarlier,
        )
    }

    // ---- 8. knownTotal ----

    @Test
    fun `KnownTotalObserved 正值覆盖 零值保留 旧 daemon 未报不洗基准`() {
        val base = state(identity = HistoryIdentityIndex(spans = listOf(span("a", 0)), knownTotal = 42))
        assertEquals(77, ChatStateReducer.reduce(base, ChatIntent.KnownTotalObserved(77)).identity.knownTotal)
        assertEquals(42, ChatStateReducer.reduce(base, ChatIntent.KnownTotalObserved(0)).identity.knownTotal)
    }

    // ---- 9. listOffset 派生 ----

    @Test
    fun `listOffset 与迁出前表达式同式 只有截断且在线才偏移 1`() {
        assertEquals(1, state(truncated = true, offlineSnapshot = false).listOffset)
        assertEquals(0, state(truncated = true, offlineSnapshot = true).listOffset)
        assertEquals(0, state(truncated = false, offlineSnapshot = false).listOffset)
        assertEquals(0, state(truncated = false, offlineSnapshot = true).listOffset)
    }
}
