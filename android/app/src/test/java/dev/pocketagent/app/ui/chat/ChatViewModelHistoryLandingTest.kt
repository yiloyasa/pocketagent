package dev.pocketagent.app.ui.chat

// ChatViewModel 历史落地通道的 JVM 钉子(2026-10-09 效果层外移 · 任务1)。
//
// 【被测物】prependHistory —— 迁自 ChatScreen 的局部函数 prependHistoryMessages
// (「加载更早」与全文搜索缺口拉取共用的头部拼接落地):按「已加载最旧 message id」
// 锚点只把回包头部新消息拼到 st.items 头部,不做整列表替换;身份索引前插推进经
// PrependApplied 意图(reducer 语义已有 ChatStateReducerTest 钉死,这里钉拼装配方
// 与落地组合:条数裁定 × st.items 落位 × 身份索引推进的一致性)。
// fetchEarlierHistory 的 RPC 腿走 gateway(JVM 可注 fake),其滚动锚定(received/
// appended 两数)一并在 fake 回包上钉住。

import dev.pocketagent.app.ui.ChatItem
import dev.pocketagent.app.ui.TurnSession
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.ContentBlock
import pocketagent.v1.GetThreadHistoryResponse
import pocketagent.v1.TextBlock
import pocketagent.v1.ThreadMessage
import pocketagent.v1.Thread as PThread

class ChatViewModelHistoryLandingTest {

    private class OneShotGateway(val response: GetThreadHistoryResponse) : ChatViewModelActionsTestGateway() {
        override suspend fun threadHistory(threadId: String, rounds: Int): GetThreadHistoryResponse = response
    }

    @Test
    fun `prependHistory 锚点命中只拼头部新消息_返回新增条数`() {
        val vm = ChatViewModel(stepRounds = 20)
        val tid = "t-prepend-anchor"
        // 已加载尾部两条(m2/m3):身份索引 + 最旧锚点 m2
        vm.send(
            ChatIntent.HistoryTailLoaded(
                spans = listOf(
                    dev.pocketagent.app.ui.history.MessageSpan("m2", "turn-m2", 0, 0, 1, "user"),
                    dev.pocketagent.app.ui.history.MessageSpan("m3", "turn-m3", 1, 1, 1, "assistant"),
                ),
                oldestMessageId = "m2",
                truncated = true,
            ),
        )
        val st = TurnSession.state(tid)
        st.items.add(ChatItem(role = "user", text = "旧-m2", id = "hm-m2#0"))
        st.items.add(ChatItem(role = "assistant", text = "旧-m3", id = "hm-m3#0"))

        // daemon 幂等回尾部:回包 = 更早段(m0/m1)+ 已加载部分(m2/m3)
        val appended = vm.prependHistory(tid, listOf(msg("m0"), msg("m1", "assistant"), msg("m2"), msg("m3", "assistant")))

        assertEquals(2, appended)
        assertEquals(4, st.items.size)
        assertTrue(st.items[0].id.startsWith("hm-m0#"))
        assertTrue(st.items[1].id.startsWith("hm-m1#"))
        assertEquals("旧-m2", st.items[2].text) // 既有条目原位不动(引用稳定)
        // 身份索引前插推进:新 spans 在头部,既有 spans 的 itemStart 整体平移
        val spans = vm.state.value.identity.spans
        assertEquals(4, spans.size)
        assertEquals("m0", spans[0].messageId)
        assertEquals(0, spans[0].itemStart)
        assertEquals("m3", spans[3].messageId)
        assertEquals(3, spans[3].itemStart)
        // 最旧锚点被回包头部非空 id 覆盖
        assertEquals("m0", vm.state.value.oldestMessageId)
    }

    @Test
    fun `prependHistory 旧daemon无id退条数兜底`() {
        val vm = ChatViewModel(stepRounds = 20)
        val tid = "t-prepend-fallback"
        val st = TurnSession.state(tid)
        st.items.add(ChatItem(role = "user", text = "已加载", id = "hm-x#0"))

        // 兜底公式是「回包条数 - identity.spans 数」(P3 Task 8 既有语义,spans≈
        // 已加载消息数;直塞 items 不产生 spans 属人工态)→ 5 - 0 = 5
        val appended = vm.prependHistory(tid, (1..5).map { msg("n$it") })
        assertEquals(5, appended)
        assertEquals(6, st.items.size)
    }

    @Test
    fun `prependHistory 重复拉取零列表变动`() {
        val vm = ChatViewModel(stepRounds = 20)
        val tid = "t-prepend-dup"
        vm.send(
            ChatIntent.HistoryTailLoaded(
                spans = listOf(dev.pocketagent.app.ui.history.MessageSpan("m2", "turn-m2", 0, 0, 1, "user")),
                oldestMessageId = "m2",
                truncated = true,
            ),
        )
        val st = TurnSession.state(tid)
        st.items.add(ChatItem(role = "user", text = "旧-m2", id = "hm-m2#0"))

        val appended = vm.prependHistory(tid, listOf(msg("m2"), msg("m3", "assistant")))
        assertEquals(0, appended)
        assertEquals(1, st.items.size) // 零列表变动(重复拉取无副作用)
    }

    @Test
    fun `prependHistory 空回包直接返回零`() {
        val vm = ChatViewModel(stepRounds = 20)
        assertEquals(0, vm.prependHistory("t-prepend-empty", emptyList()))
    }

    @Test
    fun `fetchEarlierHistory 回包为空不推进轮数_非空推进并回报两数`() = kotlinx.coroutines.runBlocking {
        val tid = "t-fetch-earlier"
        // 空回包:received=0,TruncationSettled 仍落位(原式:入口显隐跟随)
        val vm1 = ChatViewModel(stepRounds = 20).also { it.gateway = OneShotGateway(GetThreadHistoryResponse.getDefaultInstance()) }
        val r1 = vm1.fetchEarlierHistory(tid, 40)
        assertEquals(0, r1.receivedCount)
        assertEquals(0, r1.appendedCount)
        assertEquals(20, vm1.state.value.roundsLoaded) // RoundsAdvanced 未发(原式在 isNotEmpty 内)
        assertTrue(!vm1.state.value.truncated)

        // 非空回包(3 条全新消息,无锚点退条数兜底 3):received=3, appended=3, 轮数推进
        val resp = GetThreadHistoryResponse.newBuilder()
            .addMessages(msg("a1"))
            .addMessages(msg("a2", "assistant"))
            .addMessages(msg("a3"))
            .build()
        val vm2 = ChatViewModel(stepRounds = 20).also { it.gateway = OneShotGateway(resp) }
        val r2 = vm2.fetchEarlierHistory("t-fetch-earlier-2", 60)
        assertEquals(3, r2.receivedCount)
        assertEquals(3, r2.appendedCount)
        assertEquals(60, vm2.state.value.roundsLoaded)
        assertEquals(3, TurnSession.state("t-fetch-earlier-2").items.size)
    }

    // ---------------------------------------------------------- 【N1】离线兜底路径

    @Test
    fun `离线快照兜底路径时间戳不丢`() = kotlinx.coroutines.runBlocking {
        // 【N1】loadHistoryTail 失败分支的 ChatItem 构造此前漏带 sentAtMs —— 快照里
        // 的真实时间戳整列剥掉,恢复后历史导航的日期分组退化「早前」。主路径
        // (prefillOfflineSnapshot)带时间戳,兜底路径必须同语义。
        val snapDir = org.junit.rules.TemporaryFolder().apply { create() }.newFolder("pa_snap")
        dev.pocketagent.app.ui.component.ThreadSnapshot.initForTest(snapDir)
        val tid = "t-offline-sentat"
        dev.pocketagent.app.ui.component.ThreadSnapshot.saveMessages(
            tid,
            listOf(
                dev.pocketagent.app.ui.component.ThreadSnapshot.Msg("user", "早的用户消息", 1_700_000_000_000L),
                dev.pocketagent.app.ui.component.ThreadSnapshot.Msg("assistant", "早的助手消息", 1_700_000_060_000L),
            ),
        )
        val vm = ChatViewModel(stepRounds = 20).also { it.gateway = ThrowingGateway() }
        val r = vm.loadHistoryTail(tid, rounds = 20, connectionEpoch = 0, reloadToken = 0)
        assertTrue("应走快照兜底路径", r.snapshotFallback)
        val st = TurnSession.state(tid)
        assertEquals(2, st.items.size)
        assertEquals("兜底路径的时间戳必须随行", 1_700_000_000_000L, st.items[0].sentAtMs)
        assertEquals(1_700_000_060_000L, st.items[1].sentAtMs)
    }

    /** threadHistory 必抛:制造"连不上"以触发离线快照兜底分支。 */
    private class ThrowingGateway : ChatViewModelActionsTestGateway() {
        override suspend fun threadHistory(threadId: String, rounds: Int): GetThreadHistoryResponse =
            throw java.io.IOException("连接失败(测试注入)")
    }
}

/** 只为复用 FakeGateway 的骨架:除被覆写方法外其余调用即失败(测试不应触达)。 */
private open class ChatViewModelActionsTestGateway : ChatRpcGateway {
    override suspend fun getThread(threadId: String): PThread = error("unexpected getThread")
    override suspend fun threadHistory(threadId: String, rounds: Int): GetThreadHistoryResponse = error("unexpected threadHistory")
    override suspend fun forkThread(threadId: String): PThread = error("unexpected forkThread")
    override suspend fun rollbackThread(threadId: String, turns: Int) = error("unexpected rollbackThread")
    override suspend fun compactThread(threadId: String) = error("unexpected compactThread")
    override suspend fun startReview(threadId: String, focus: String) = error("unexpected startReview")
    override suspend fun clearQueuedInputs(threadId: String): pocketagent.v1.ClearQueuedInputsResponse = error("unexpected clearQueuedInputs")
    override suspend fun setCodexSettings(approvalPolicy: String, sandbox: String): pocketagent.v1.CodexSettings = error("unexpected setCodexSettings")
    override suspend fun approveGuardianDeniedAction(threadId: String, reviewId: String, targetItemId: String) = error("unexpected approveGuardianDeniedAction")
    override suspend fun setThreadGoal(threadId: String, goal: String): PThread = error("unexpected setThreadGoal")
    override suspend fun listModels(agentId: String, threadId: String): pocketagent.v1.ListModelsResponse = error("unexpected listModels")
}
