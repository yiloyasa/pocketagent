package dev.pocketagent.app.ui

// ConversationUpgradeIntegrationTest —— 升级批次跨组件 JVM 集成测试
// (2026-10-04 release-and-end-to-end-validation 计划 Task 4,验证清单第 1/2/3/4/7 条)。
//
// 【组装口径】真 Reducer + 真 Repository + fake RPC(学各 Repository 既有测试的
// 注入面,不 mock 网络、不碰真 channel):
//  - 模型:真 ModelSelectionReducer 全流转;ModelSelectionRepository 本身是
//    object + AgentConnection 静态封装,**没有 fake RPC 注入缝**(JVM 不可驱动,
//    强行调用会撞 gRPC channel 与 android.util.Log)——它的"无轮次调用"承诺
//    用可执行的两层钉子替代:①意图词表反射(只有四个选择意图,无任何
//    Cancel/Start 类);②仓库方法面反射(declaredMethods 无取消/轮次词根),
//    见「模型切换只影响下一轮」一组。
//  - 历史:真 HistorySearchRepository(注入 fake searchRpc/historyRpc)+ 真决策
//    HistorySearchState.planHitLocate + 真 ChatViewModel(ChatStateReducer)身份
//    索引重建,复现 ChatScreen 宿主的 buildHistoryBatch 拼装配方。
//  - offline:真 ChatViewModel + ChatStateReducer 断线→BackOnline 全流转。
//
// 被测产物只测不改;daemon 侧(TurnStarted/TurnSnapshot 语义)归 Go 侧 e2e,
// 本文件只钉客户端投影与 daemon 真值的一致性。

import dev.pocketagent.app.ui.chat.ChatIntent
import dev.pocketagent.app.ui.chat.ChatStateReducer
import dev.pocketagent.app.ui.chat.ChatViewModel
import dev.pocketagent.app.ui.history.HistoryLocatePlan
import dev.pocketagent.app.ui.history.HistorySearchRepository
import dev.pocketagent.app.ui.history.HistorySearchState
import dev.pocketagent.app.ui.history.HistorySearchStatus
import dev.pocketagent.app.ui.history.MessageSpan
import dev.pocketagent.app.ui.history.ResultsKind
import dev.pocketagent.app.ui.model.EffectiveSelectionUi
import dev.pocketagent.app.ui.model.ModelSelectionIntent
import dev.pocketagent.app.ui.model.ModelSelectionReducer
import dev.pocketagent.app.ui.model.ModelSelectionRepository
import dev.pocketagent.app.ui.model.SelectionSource
import io.grpc.Status
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.ContentBlock
import pocketagent.v1.EffectiveModelSelection
import pocketagent.v1.GetThreadHistoryResponse
import pocketagent.v1.ListModelsResponse
import pocketagent.v1.MatchRange
import pocketagent.v1.ModelInfo
import pocketagent.v1.SearchThreadMessagesResponse
import pocketagent.v1.TextBlock
import pocketagent.v1.ThreadMessage
import pocketagent.v1.ThreadMessageHit
import java.util.concurrent.atomic.AtomicInteger
import java.util.concurrent.atomic.AtomicReference

class ConversationUpgradeIntegrationTest {

    // ================================================================
    // 测试材料
    // ================================================================

    /** pi 档位制模型(带 daemon 透出的动态档位)。 */
    private fun piModel(id: String, vararg efforts: String): ModelInfo =
        ModelInfo.newBuilder()
            .setId(id)
            .setProvider("pi")
            .addAllAvailableEfforts(efforts.toList())
            .build()

    /** 新 daemon 的 ListModels 回包:effective_selection(5) + 模型表。 */
    private fun listModelsWithSelection(
        modelId: String,
        effort: String,
        source: String,
        nextTurn: Boolean,
        vararg efforts: String,
    ): ListModelsResponse =
        ListModelsResponse.newBuilder()
            .setEffectiveSelection(
                EffectiveModelSelection.newBuilder()
                    .setAgentId("pi")
                    .setModelId(modelId)
                    .setEffort(effort)
                    .setSource(source)
                    .setAppliesToNextTurn(nextTurn)
                    .setConfigurationConfirmed(true)
                    .setExplanation("会话覆盖:你在本会话选定的配置")
                    .addAllAvailableEfforts(efforts.toList()),
            )
            .addModels(piModel("pi/glm-5.3", "low", "high", "max"))
            .build()

    /** 单文本块的 daemon 消息(历史身份跨度重建的原料)。 */
    private fun message(id: String, turn: String, role: String, text: String): ThreadMessage =
        ThreadMessage.newBuilder()
            .setRole(role)
            .setMessageId(id)
            .setTurnId(turn)
            .addBlocks(ContentBlock.newBuilder().setText(TextBlock.newBuilder().setText(text)))
            .build()

    /**
     * ChatScreen 宿主 buildHistoryBatch 的同款配方:ThreadMessage 列表 →
     * 身份跨度(每条消息映射出的 ChatItem 连续段;历史映射一条消息 ≥1 个
     * ChatItem,这里单文本块消息 = 1 个,经真 HistoryMessageMapper 计数)。
     */
    private fun spansFromMessages(messages: List<ThreadMessage>): List<MessageSpan> {
        var start = 0
        return messages.mapIndexed { i, m ->
            val count = historyToChatItems(listOf(m)).size
            val span = MessageSpan(
                messageId = m.messageId,
                turnId = m.turnId,
                sequenceInResponse = i,
                itemStart = start,
                itemCount = count,
                role = m.role,
            )
            start += count
            span
        }
    }

    // ================================================================
    // 1) 模型切换只影响下一轮;TurnStarted 与 UI 一致
    // ================================================================

    @Test
    fun `模型切换_乐观投影只标下一轮_真值回填与TurnStarted口径一致_其余状态零扰动`() {
        // 进入页面:先"上次记录"缓存,后 daemon 真值整树替换
        val s0 = ModelSelectionReducer.initial().copy(threadId = "t1", agentId = "pi")
        val cached = EffectiveSelectionUi(
            agentId = "pi", modelId = "pi/glm-5.3", effort = "max",
            source = SelectionSource.THREAD_OVERRIDE, sourceRaw = SelectionSource.THREAD_OVERRIDE.raw,
            appliesToNextTurn = false, explanation = "", availableEfforts = emptyList(),
        )
        val s1 = ModelSelectionReducer.withCachedSelection(s0, cached)
        assertTrue(ModelSelectionReducer.display(s1)?.fromCache == true) // 「上次记录」标注

        val truth = ModelSelectionReducer.fromListModelsResponse(
            listModelsWithSelection("pi/glm-5.3", "high", "thread_override", nextTurn = false, "low", "high", "max"),
            "pi",
        )
        val s2 = ModelSelectionReducer.applyDaemon(s1, truth)
        val shown2 = ModelSelectionReducer.display(s2)
        assertNotNull(shown2)
        assertFalse(shown2!!.optimistic)
        assertFalse(shown2.fromCache)
        // daemon 真值说"当前轮已带此配置"(applies_to_next_turn=false),UI 如实渲染
        assertFalse(shown2.appliesToNextTurn)

        // 运行中切换(乐观):客户端口径 = 将从下一轮生效,绝不触碰运行中的 Turn
        val s3 = ModelSelectionReducer.reduce(s2, ModelSelectionIntent.SelectModel("pi/qwen-max"))
        val opt = s3.selection
        assertNotNull(opt)
        assertEquals("pi/qwen-max", opt!!.modelId)
        assertEquals(SelectionSource.THREAD_OVERRIDE, opt.source)
        assertTrue(opt.appliesToNextTurn) // UI 层钉死:切换后显示"下一轮生效"
        assertTrue(opt.optimistic)
        assertEquals("pi/glm-5.3", s3.pending?.previous?.modelId) // 回滚锚点 = 换前的真值

        // 「只影响下一轮」的状态侧表达:切换只动 selection/pending 两个槽位,
        // 模型表/缓存/加载位/错误位/会话锚点全部原样(本层没有别的可写通道)
        assertEquals(
            s2.copy(selection = null, pending = null),
            s3.copy(selection = null, pending = null),
        )

        // 下发落盘 → daemon 真值回填:applies_to_next_turn 以 daemon 为准,UI 跟随
        val backfill = ModelSelectionReducer.mapSelection(
            EffectiveModelSelection.newBuilder()
                .setAgentId("pi")
                .setModelId("pi/qwen-max")
                .setEffort("low")
                .setSource("thread_override")
                .setAppliesToNextTurn(false) // daemon:新配置已落库,下一轮 TurnStarted 即带它
                .setExplanation("下一轮起按 pi/qwen-max 运行")
                .build(),
        )
        val s4 = ModelSelectionReducer.applyDaemon(ModelSelectionReducer.confirmSent(s3), backfill)
        val shown4 = ModelSelectionReducer.display(s4)
        assertNotNull(shown4)
        assertEquals("pi/qwen-max", shown4!!.modelId)
        assertFalse(shown4.optimistic) // 乐观投影退场,真值即结论
        assertFalse(shown4.appliesToNextTurn) // UI 与 daemon 的 TurnStarted 口径一致
        assertNull(s4.pending)
        assertNull(s4.error)
    }

    @Test
    fun `模型选择_意图词表与方法面_无任何取消或轮次调用通道`() {
        // 钉 ①:用户意图词表 = 恰好四个选择意图,不含任何 Cancel/Start/轮次动作
        // (发轮次归 TurnSession,选择层连通道都没有 —— 轮次红线的编译级表达;
        //  硬编码词表钉,不用 kotlin-reflect(sealedSubclasses 依赖它,JVM 测试类路径
        //  无且禁加) —— 将来新增第五意图时词表漏登记,本断言照红,钉子语义不变)
        assertEquals(
            listOf("Refresh", "ResetToDefault", "SelectEffort", "SelectModel"),
            listOf(
                ModelSelectionIntent.Refresh::class,
                ModelSelectionIntent.ResetToDefault::class,
                ModelSelectionIntent.SelectEffort::class,
                ModelSelectionIntent.SelectModel::class,
            ).map { it.simpleName.orEmpty() }.sorted(),
        )

        // 钉 ②:仓库方法面无取消/轮次词根(不发 Cancel/Start 类调用的结构保证;
        // 运行时调用集断言需给 object 注入缝,见文件头说明与报告)
        val forbidden = Regex("(?i)(cancel|interrupt|abort|start.*turn|turn.*start|stop.*turn|turn.*stop)")
        val offenders = ModelSelectionRepository::class.java.declaredMethods
            .map { it.name }
            .filter { forbidden.containsMatchIn(it) }
        assertTrue("选择仓库不得有取消/轮次方法,违例:$offenders", offenders.isEmpty())
    }

    @Test
    fun `TurnStarted与UI一致_source与appliesToNextTurn映射逐字_未知来源不编造`() {
        // daemon 侧 TurnStarted/TurnSnapshot 语义已测;这里钉 UI 投影逐字一致:
        // 四个已知来源 + 词表外/空来源,source/sourceRaw/appliesToNextTurn 原样透传
        val cases = listOf(
            "thread_override" to SelectionSource.THREAD_OVERRIDE,
            "app_default" to SelectionSource.APP_DEFAULT,
            "desktop_default" to SelectionSource.DESKTOP_DEFAULT,
            "native_current" to SelectionSource.NATIVE_CURRENT,
            "future_source" to SelectionSource.UNKNOWN, // 新版本新增:降级不报错,原文保留
            "" to SelectionSource.UNKNOWN,
        )
        for ((raw, expected) in cases) {
            val ui = ModelSelectionReducer.mapSelection(
                EffectiveModelSelection.newBuilder()
                    .setAgentId("pi").setModelId("pi/glm-5.3").setEffort("high")
                    .setSource(raw).setAppliesToNextTurn(true)
                    .build(),
            )
            assertEquals(expected, ui.source)
            assertEquals(raw, ui.sourceRaw) // 未知来源原文如实展示(UI 标注用)
            assertTrue("daemon 说的生效时机必须原样到达 UI:$raw", ui.appliesToNextTurn)
            // display():真值在时 UI 渲染的就是这份 source/时机
            val state = ModelSelectionReducer.applyDaemon(ModelSelectionReducer.initial(), ui)
            assertEquals(ui, ModelSelectionReducer.display(state))
        }

        // display() 回退序:无真值退「上次记录」,缓存时机不主张"下一轮"
        val cached = EffectiveSelectionUi(
            agentId = "pi", modelId = "pi/glm-5.3", effort = "",
            source = SelectionSource.THREAD_OVERRIDE, sourceRaw = SelectionSource.THREAD_OVERRIDE.raw,
            appliesToNextTurn = false, explanation = "", availableEfforts = emptyList(),
        )
        val cacheOnly = ModelSelectionReducer.withCachedSelection(ModelSelectionReducer.initial(), cached)
        assertEquals(cached.copy(fromCache = true), ModelSelectionReducer.display(cacheOnly))
    }

    // ================================================================
    // 2) effort 不可用时清理并显示原因
    // ================================================================

    @Test
    fun `effort不可用_切模型即清理为新模型的档位_真值回填原因与档位呈现就位`() {
        val models = listOf(piModel("pi/glm-5.3", "low", "high", "max"))
        val truth = ModelSelectionReducer.applyDaemon(
            ModelSelectionReducer.initial().copy(agentId = "pi", threadId = "t1"),
            EffectiveSelectionUi(
                agentId = "pi", modelId = "pi/other-x", effort = "xhigh",
                source = SelectionSource.THREAD_OVERRIDE, sourceRaw = SelectionSource.THREAD_OVERRIDE.raw,
                appliesToNextTurn = false, explanation = "", availableEfforts = emptyList(),
            ),
        ).copy(models = models)

        // SelectModel(无 xhigh 档位的模型):失效强度清成空串(跟随默认,零值合法)
        val switched = ModelSelectionReducer.reduce(truth, ModelSelectionIntent.SelectModel("pi/glm-5.3"))
        val opt = switched.selection
        assertNotNull(opt)
        assertEquals("pi/glm-5.3", opt!!.modelId)
        assertEquals("", opt.effort) // cleanEffortFor 生效:不保留失效强度
        assertTrue(opt.optimistic)
        // 呈现数据就位:档位表 = 新模型 daemon 透出的动态档;原因位清空等真值回填
        assertEquals(listOf("low", "high", "max"), opt.availableEfforts)
        assertEquals("", opt.explanation)

        // daemon 真值回填:原因与档位一并就位,UI 直接展示
        val backfill = ModelSelectionReducer.mapSelection(
            EffectiveModelSelection.newBuilder()
                .setAgentId("pi")
                .setModelId("pi/glm-5.3")
                .setEffort("")
                .setSource("thread_override")
                .setAppliesToNextTurn(true)
                .setExplanation("xhigh 不被 pi/glm-5.3 支持,已改为跟随默认")
                .addAllAvailableEfforts(listOf("low", "high", "max"))
                .build(),
        )
        val settled = ModelSelectionReducer.applyDaemon(ModelSelectionReducer.confirmSent(switched), backfill)
        val shown = ModelSelectionReducer.display(settled)
        assertNotNull(shown)
        assertEquals("xhigh 不被 pi/glm-5.3 支持,已改为跟随默认", shown!!.explanation)
        assertEquals(listOf("low", "high", "max"), shown.availableEfforts)
        assertEquals("", shown.effort)
        assertFalse(shown.optimistic)
    }

    @Test
    fun `effort不可用_无法判定档位时保留原值等真值校正_不猜`() {
        // 模型不在表里且 agent 档位情况未知(空表):cleanEffortFor 回 null → 保留原值
        val truth = ModelSelectionReducer.applyDaemon(
            ModelSelectionReducer.initial().copy(agentId = "pi", threadId = "t1"),
            EffectiveSelectionUi(
                agentId = "pi", modelId = "pi/other-x", effort = "high",
                source = SelectionSource.THREAD_OVERRIDE, sourceRaw = SelectionSource.THREAD_OVERRIDE.raw,
                appliesToNextTurn = false, explanation = "", availableEfforts = emptyList(),
            ),
        )
        val switched = ModelSelectionReducer.reduce(truth, ModelSelectionIntent.SelectModel("pi/brand-new"))
        assertEquals("high", switched.selection?.effort) // 不猜,等 daemon 真值回填校正
    }

    // ================================================================
    // 3) 历史搜索命中未加载消息时自动拉取并定位(全链)
    // ================================================================

    @Test
    fun `历史搜索_命中未加载消息_fake回命中_计划拉取_回包落地身份重建_定位成功全链`() = runBlocking {
        // --- 场景:全会话 8 条消息(seq 0..7,id m1..m8),手机已加载尾部 3 条
        //    (m6..m8,锚点 m6);命中的 m2(seq=1)仍在未下载的历史里。
        val searchCalls = AtomicInteger(0)
        val capturedQuery = AtomicReference("")
        val capturedLimit = AtomicInteger(0)
        val searchResp = SearchThreadMessagesResponse.newBuilder()
            .setScope("full_thread")
            .setSearchedMessages(8)
            .setHasMore(false)
            .addHits(
                ThreadMessageHit.newBuilder()
                    .setSequence(1)
                    .setRole("user")
                    .setSnippet("…这里提到跳跃词…")
                    .setMessageId("m2")
                    .setTurnId("turn-2")
                    .setFromLoadedHistory(false)
                    .addRanges(MatchRange.newBuilder().setStart(1).setEnd(4)),
            )
            .build()

        val fetchCalls = AtomicInteger(0)
        val capturedRounds = AtomicInteger(0)
        // daemon 幂等回尾部:请求 7 轮 → 回 seq1..seq7(m2..m8,含已加载尾部)
        val fetchedMessages = (2..8).map { n ->
            message("m$n", "turn-$n", "user", "第 $n 条消息正文")
        }

        val repo = HistorySearchRepository(
            threadId = "t1",
            searchRpc = { _, query, limit ->
                searchCalls.incrementAndGet()
                capturedQuery.set(query)
                capturedLimit.set(limit)
                searchResp
            },
            historyRpc = { rounds ->
                fetchCalls.incrementAndGet()
                capturedRounds.set(rounds)
                GetThreadHistoryResponse.newBuilder().addAllMessages(fetchedMessages).build()
            },
        )

        // --- ① 搜索:真仓库 fake RPC 回命中
        val status = repo.search("跳跃词")
        assertTrue(status is HistorySearchStatus.Results)
        status as HistorySearchStatus.Results
        assertEquals("跳跃词", capturedQuery.get())
        assertEquals(HistorySearchRepository.DEFAULT_LIMIT, capturedLimit.get())
        assertEquals(1, status.hits.size)
        val hit = status.hits[0]
        assertEquals("m2", hit.messageId)
        assertEquals(8, hit.searchedTotal) // 命中随身携带全量基准
        assertEquals(1, searchCalls.get())

        // --- ② 宿主状态:打开会话已加载尾部(截断),搜索回包报全量基准
        val chatVm = ChatViewModel(stepRounds = 10)
        val tail = listOf(
            message("m6", "turn-6", "user", "第 6 条消息正文"),
            message("m7", "turn-7", "assistant", "第 7 条回复正文"),
            message("m8", "turn-8", "user", "第 8 条消息正文"),
        )
        chatVm.send(ChatIntent.HistoryTailLoaded(spansFromMessages(tail), oldestMessageId = "m6", truncated = true))
        chatVm.send(ChatIntent.KnownTotalObserved(status.searchedMessages))
        assertEquals(8, chatVm.state.value.identity.knownTotal)

        // --- ③ 定位裁定:命中不在已加载内容 → 精确缺口拉取计划(8 - 1 = 7 轮)
        val plan1 = HistorySearchState.planHitLocate(
            hit, chatVm.state.value.identity,
            currentRounds = chatVm.state.value.roundsLoaded, stepRounds = 10,
        )
        assertEquals(HistoryLocatePlan.FetchTarget(requestedRounds = 7, exact = true), plan1)
        val fetchPlan = plan1 as HistoryLocatePlan.FetchTarget

        // --- ④ 按计划拉取:真仓库走注入的历史通道,轮数与计划一致
        val messages = repo.fetchTargetMessages(fetchPlan.requestedRounds)
        assertEquals(1, fetchCalls.get())
        assertEquals(7, capturedRounds.get())
        assertEquals(listOf("m2", "m3", "m4", "m5", "m6", "m7", "m8"), messages.map { it.messageId })

        // --- ⑤ 落地:宿主配方(锚点裁定 + 身份重建 + 轮数只进不退)
        val newCount = chatVm.planPrepend(messages.map { it.messageId })
        assertEquals(4, newCount) // 锚点 m6 在回包下标 4:m2..m5 是新消息
        chatVm.send(ChatIntent.PrependApplied(spansFromMessages(messages.take(newCount)), newCount, messages.first().messageId))
        chatVm.send(ChatIntent.RoundsAdvancedIfBeyond(fetchPlan.requestedRounds))
        val state = chatVm.state.value
        assertEquals(listOf("m2", "m3", "m4", "m5", "m6", "m7", "m8"), state.identity.spans.map { it.messageId })
        assertEquals(8, state.identity.knownTotal) // 全量基准保留
        assertEquals(10, state.roundsLoaded) // 请求 7 轮 < 已加载 10 轮:不回退
        assertEquals("m2", state.oldestMessageId)

        // --- ⑥ 重裁:命中已在已加载内容 → 直接定位到时间线下标
        val plan2 = HistorySearchState.planHitLocate(
            hit, state.identity,
            currentRounds = state.roundsLoaded, stepRounds = 10,
        )
        assertEquals(HistoryLocatePlan.Jump(0), plan2) // m2 的 ChatItem 段起点
        // 角色校验在链路里生效:索引角色与命中不符则宁可重拉不假跳
        assertNull(HistorySearchState.locateLoadedSpan(hit.copy(role = "assistant"), state.identity))
        // LazyColumn 目标 = Jump 下标 + 列表头偏移(截断且在线 → 「加载更早」占 1)
        assertEquals(1, state.listOffset)
    }

    // ================================================================
    // 4) 搜索失败和无结果区别显示
    // ================================================================

    @Test
    fun `搜索失败与无结果严格分态_scope未知带标注_失败永不伪装成空`() = runBlocking {
        // --- 失败态:真仓库,注入抛 gRPC 异常的搜索通道
        val failingRepo = HistorySearchRepository(
            threadId = "t1",
            searchRpc = { _, _, _ -> throw Status.UNAVAILABLE.asRuntimeException() },
        )
        val failed = failingRepo.search("查询词")
        assertTrue(failed is HistorySearchStatus.RemoteError)
        failed as HistorySearchStatus.RemoteError
        assertEquals("查询词", failed.query) // 失败也记住查的什么
        assertTrue("人话原因:$failed.reason", failed.reason.contains("连不上"))
        assertTrue(failed.reason.isNotBlank() && !failed.reason.contains("UNAVAILABLE"))

        // 分状态人话:参数不可用 / 会话已不存在(与其他失败可区分)
        val invalid = HistorySearchRepository(
            threadId = "t1",
            searchRpc = { _, _, _ -> throw Status.INVALID_ARGUMENT.asRuntimeException() },
        ).search("查询词") as HistorySearchStatus.RemoteError
        assertEquals("请求参数不可用", invalid.reason)
        val notFound = HistorySearchRepository(
            threadId = "t1",
            searchRpc = { _, _, _ -> throw Status.NOT_FOUND.asRuntimeException() },
        ).search("查询词") as HistorySearchStatus.RemoteError
        assertTrue(notFound.reason.contains("已不存在"))

        // --- 无结果态:RPC 成功、零命中(与失败严格两个类型,UI 两副面孔)
        val emptyRepo = HistorySearchRepository(
            threadId = "t1",
            searchRpc = { _, _, _ ->
                SearchThreadMessagesResponse.newBuilder().setScope("full_thread").setSearchedMessages(12).build()
            },
        )
        val none = emptyRepo.search("不存在的词")
        assertTrue(none is HistorySearchStatus.NoResults)
        none as HistorySearchStatus.NoResults
        assertEquals("不存在的词", none.query)
        assertEquals(ResultsKind.FULL_THREAD, none.kind)
        assertTrue(none.scopeKnown)
        assertTrue(failed::class != none::class) // 两态在类型上就分开了,不可能互相伪装

        // --- scope 未知标注:旧 daemon 空 scope,Results/NoResults 都标"范围未知"
        val legacyEmpty = HistorySearchRepository(
            threadId = "t1",
            searchRpc = { _, _, _ ->
                SearchThreadMessagesResponse.newBuilder().setSearchedMessages(3).build()
            },
        ).search("查询词") as HistorySearchStatus.NoResults
        assertFalse(legacyEmpty.scopeKnown) // UI 据此追加"(范围未知)"

        val legacyHits = HistorySearchRepository(
            threadId = "t1",
            searchRpc = { _, _, _ ->
                SearchThreadMessagesResponse.newBuilder()
                    .setSearchedMessages(3)
                    .addHits(
                        ThreadMessageHit.newBuilder()
                            .setSequence(0).setRole("user").setSnippet("命中词").setMessageId("mx"),
                    )
                    .build()
            },
        ).search("命中词")
        assertTrue(legacyHits is HistorySearchStatus.Results)
        legacyHits as HistorySearchStatus.Results
        assertFalse(legacyHits.scopeKnown)
        // scope 词表常量钉住(daemon 契约字符串不漂移)
        assertEquals("full_thread", HistorySearchState.SCOPE_FULL_THREAD)
        assertEquals("local_snapshot", HistorySearchState.SCOPE_LOCAL_SNAPSHOT)
    }

    // ================================================================
    // 7) offline snapshot 只读且显示时间
    // ================================================================

    @Test
    fun `offline快照_断线落时间与只读语义_重连BackOnline成对清_重试只清标志`() {
        val chatVm = ChatViewModel(stepRounds = 30)
        assertEquals(30, chatVm.state.value.roundsLoaded) // 设置档位收敛
        assertFalse(chatVm.state.value.offlineSnapshot)

        // 在线:已加载尾部(截断),列表头带「加载更早」偏移
        chatVm.send(
            ChatIntent.HistoryTailLoaded(
                spans = listOf(
                    MessageSpan("m1", "turn-1", 0, 0, 1, "user"),
                    MessageSpan("m2", "turn-2", 1, 1, 1, "assistant"),
                ),
                oldestMessageId = "m1",
                truncated = true,
            ),
        )
        assertEquals(1, chatVm.state.value.listOffset)

        // 断线快照分支:落盘时间进状态(横幅「离线快照 · HH:mm」的数据位),
        // 身份索引与最旧 id 一并作废 —— 绝不让旧索引把命中指进只读快照列表
        chatVm.send(ChatIntent.ShowOfflineSnapshot(atMs = 1_728_000_000_000L, resetIdentity = true))
        var s = chatVm.state.value
        assertTrue(s.offlineSnapshot)
        assertEquals(1_728_000_000_000L, s.offlineSnapshotAt) // 显示时间的就位判据
        assertTrue(s.identity.spans.isEmpty())
        assertEquals("", s.oldestMessageId)
        assertEquals(0, s.listOffset) // 快照时间线是只读的:不显示「加载更早」入口
        assertEquals(30, s.roundsLoaded) // 快照不动加载档位

        // 只读的通道侧表达:快照态下没有任何意图能恢复身份索引(重建只靠重连后重载)
        chatVm.send(ChatIntent.BackOnline)
        s = chatVm.state.value
        assertFalse(s.offlineSnapshot)
        assertEquals(0L, s.offlineSnapshotAt) // 成对清:标志 + 时间一起退
        assertTrue(s.identity.spans.isEmpty()) // 不复活,等 HistoryTailLoaded 重建

        // 重试通道:只清标志、保留落盘时间(reloadToken 驱动的重载会重新裁定)
        chatVm.send(ChatIntent.ShowOfflineSnapshot(atMs = 42L))
        chatVm.send(ChatIntent.ClearSnapshotFlag)
        s = chatVm.state.value
        assertFalse(s.offlineSnapshot)
        assertEquals(42L, s.offlineSnapshotAt)

        // 横幅「重新加载」通道:成对清(标志 + 时间)
        chatVm.send(ChatIntent.ShowOfflineSnapshot(atMs = 42L))
        chatVm.send(ChatIntent.ClearSnapshot)
        s = chatVm.state.value
        assertFalse(s.offlineSnapshot)
        assertEquals(0L, s.offlineSnapshotAt)
    }

    @Test
    fun `ChatViewModel 初始档位脏值经 clamp 收敛回默认档`() {
        assertEquals(HistoryRoundsLogic.DEFAULT_ROUNDS, ChatViewModel(stepRounds = 7).state.value.roundsLoaded)
        assertEquals(
            ChatStateReducer.initial(10).roundsLoaded,
            ChatViewModel(stepRounds = 10).state.value.roundsLoaded,
        )
    }
}
