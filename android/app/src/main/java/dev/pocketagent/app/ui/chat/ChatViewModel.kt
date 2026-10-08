package dev.pocketagent.app.ui.chat

import androidx.compose.runtime.snapshotFlow
import dev.pocketagent.app.AgentConnection
import dev.pocketagent.app.ui.ChatItem
import dev.pocketagent.app.ui.DELIVERY_QUEUED
import dev.pocketagent.app.ui.GoalDisplayLogic
import dev.pocketagent.app.ui.HistoryRoundsLogic
import dev.pocketagent.app.ui.SLASH_COMMANDS_DEADLINE_S
import dev.pocketagent.app.ui.SlashCommandCache
import dev.pocketagent.app.ui.SlashCommandInfo
import dev.pocketagent.app.ui.TurnSession
import dev.pocketagent.app.ui.historyChatItemsWithIds
import dev.pocketagent.app.ui.nextChatItemId
import dev.pocketagent.app.ui.isSlashPanelAgent
import dev.pocketagent.app.ui.component.PaNotifications
import dev.pocketagent.app.ui.component.ThreadSnapshot
import dev.pocketagent.app.ui.history.MessageSpan
import dev.pocketagent.app.ui.model.ModelSelectionRepository
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import pocketagent.v1.ApproveGuardianDeniedActionRequest
import pocketagent.v1.ClearQueuedInputsRequest
import pocketagent.v1.ClearQueuedInputsResponse
import pocketagent.v1.CodexSettings
import pocketagent.v1.CompactThreadRequest
import pocketagent.v1.ForkThreadRequest
import pocketagent.v1.GetCodexSettingsRequest
import pocketagent.v1.GetHostInfoRequest
import pocketagent.v1.GetThreadHistoryRequest
import pocketagent.v1.GetThreadHistoryResponse
import pocketagent.v1.GetThreadRequest
import pocketagent.v1.ListModelsRequest
import pocketagent.v1.ListModelsResponse
import pocketagent.v1.ListSlashCommandsRequest
import pocketagent.v1.ListThreadsRequest
import pocketagent.v1.ModelInfo
import pocketagent.v1.RollbackThreadRequest
import pocketagent.v1.SetCodexSettingsRequest
import pocketagent.v1.SetThreadGoalRequest
import pocketagent.v1.StartReviewRequest
import pocketagent.v1.ThreadMessage
import pocketagent.v1.ThreadStatus
import pocketagent.v1.Thread as PThread
import java.util.concurrent.TimeUnit

/**
 * ChatViewModel 的会话 RPC 网关(2026-10-09 效果层外移)。默认 [RealChatRpcGateway]
 * 直连 AgentConnection;JVM 单测注入 fake 子类(学 HistorySearchRepository 的
 * lambda 注入缝先例)钉 Result 包装与异常透传 —— 不 mock 网络、不碰真 channel。
 * 每个方法与迁出前 ChatScreen 效果层内的一段 withContext(Dispatchers.IO){…RPC…}
 * 逐字对应(请求构造/无 deadline 原样)。
 */
internal interface ChatRpcGateway {
    suspend fun getThread(threadId: String): PThread
    suspend fun threadHistory(threadId: String, rounds: Int): GetThreadHistoryResponse
    suspend fun forkThread(threadId: String): PThread
    suspend fun rollbackThread(threadId: String, turns: Int)
    suspend fun compactThread(threadId: String)
    suspend fun startReview(threadId: String, focus: String)
    suspend fun clearQueuedInputs(threadId: String): ClearQueuedInputsResponse
    suspend fun setCodexSettings(approvalPolicy: String, sandbox: String): CodexSettings
    suspend fun approveGuardianDeniedAction(threadId: String, reviewId: String, targetItemId: String)
    suspend fun setThreadGoal(threadId: String, goal: String): PThread
    suspend fun listModels(agentId: String, threadId: String): ListModelsResponse
}

/** 真网关:每段 = 迁出前效果层的 withContext(Dispatchers.IO) 原式。 */
internal class RealChatRpcGateway : ChatRpcGateway {
    override suspend fun getThread(threadId: String): PThread = withContext(Dispatchers.IO) {
        AgentConnection.threadService()
            .getThread(GetThreadRequest.newBuilder().setThreadId(threadId).build())
    }

    override suspend fun threadHistory(threadId: String, rounds: Int): GetThreadHistoryResponse =
        withContext(Dispatchers.IO) {
            AgentConnection.threadService()
                .getThreadHistory(GetThreadHistoryRequest.newBuilder().setThreadId(threadId).setRounds(rounds).build())
        }

    override suspend fun forkThread(threadId: String): PThread = withContext(Dispatchers.IO) {
        AgentConnection.threadService()
            .forkThread(ForkThreadRequest.newBuilder().setThreadId(threadId).build())
    }

    override suspend fun rollbackThread(threadId: String, turns: Int) {
        withContext(Dispatchers.IO) {
            AgentConnection.threadService().rollbackThread(
                RollbackThreadRequest.newBuilder()
                    .setThreadId(threadId)
                    .setNumTurns(turns)
                    .build(),
            )
        }
    }

    override suspend fun compactThread(threadId: String) {
        withContext(Dispatchers.IO) {
            AgentConnection.threadService()
                .compactThread(CompactThreadRequest.newBuilder().setThreadId(threadId).build())
        }
    }

    override suspend fun startReview(threadId: String, focus: String) {
        withContext(Dispatchers.IO) {
            AgentConnection.threadService().startReview(
                StartReviewRequest.newBuilder()
                    .setThreadId(threadId)
                    .setFocus(focus)
                    .build(),
            )
        }
    }

    override suspend fun clearQueuedInputs(threadId: String): ClearQueuedInputsResponse =
        withContext(Dispatchers.IO) {
            AgentConnection.threadService()
                .clearQueuedInputs(ClearQueuedInputsRequest.newBuilder().setThreadId(threadId).build())
        }

    override suspend fun setCodexSettings(approvalPolicy: String, sandbox: String): CodexSettings =
        withContext(Dispatchers.IO) {
            AgentConnection.modelService().setCodexSettings(
                SetCodexSettingsRequest.newBuilder()
                    .setApprovalPolicy(approvalPolicy)
                    .setSandbox(sandbox)
                    .build(),
            )
        }

    override suspend fun approveGuardianDeniedAction(threadId: String, reviewId: String, targetItemId: String) {
        withContext(Dispatchers.IO) {
            AgentConnection.threadService().approveGuardianDeniedAction(
                ApproveGuardianDeniedActionRequest.newBuilder()
                    .setThreadId(threadId)
                    .setReviewId(reviewId)
                    .setTargetItemId(targetItemId)
                    .build(),
            )
        }
    }

    override suspend fun setThreadGoal(threadId: String, goal: String): PThread = withContext(Dispatchers.IO) {
        AgentConnection.threadService().setThreadGoal(
            SetThreadGoalRequest.newBuilder()
                .setThreadId(threadId)
                .setGoal(goal)
                .build(),
        )
    }

    override suspend fun listModels(agentId: String, threadId: String): ListModelsResponse =
        withContext(Dispatchers.IO) {
            AgentConnection.modelServiceSlow().listModels(
                ListModelsRequest.newBuilder().setAgentId(agentId).setThreadId(threadId).build(),
            )
        }
}

/**
 * ChatScreen 页面级长期状态的持有者(2026-10-04 计划 Task 8,第一层分层;
 * 第二层增量:会话元数据四项/加载错误/斜杠命令镜像照同一形态收拢;
 * 第三层增量:[session] —— TurnSession 交互协议面的只读投影,见下)。
 *
 * 【形态】项目没有 viewModel 依赖(禁加),按计划口径用普通 class:构造即持有
 * 一棵 [ChatState] 的 StateFlow;ChatScreen 只 collect([ChatRoute] 构造并传入,
 * 直连入口在 ChatScreen 内兜底自建)。照 ModelSelectionRepository 的
 * "单一投影点 + 意图入口"先例,但更薄 —— 前两层迁出的全是**同步状态推进**
 * (RPC 仍留在 ChatScreen 的既有接线段里,零搬动)。第三层的 [session] 需要
 * 一个跟随 TurnSession 快照状态的常驻收集,所以本类在此引入了自己的
 * CoroutineScope(只挂投影收集这一件事;前两层的意图路径仍是无副作用纯
 * reducer,scope 不参与)。
 *
 * 【SharedPreferences】累计轮数的初始档位经 AgentConnection 既有访问器
 * ([AgentConnection.historyRounds])读取 —— 与迁出前 ChatScreen 的原地初始化
 * 同一条通道,本类不新增任何 prefs 直读。
 *
 * 【2026-10-09 效果层外移增量】RPC 直调从 ChatScreen 的 composable 效果层收进本类:
 *  - 打开会话加载三阶段(prefillOfflineSnapshot/loadThreadHead/loadHistoryTail)、
 *    「加载更早」(fetchEarlierHistory)+ 头部拼接落地(prependHistory);
 *  - 八个会话动作出 Result、UI 出 Toast(fork/rollback/compact/review/clearQueued/
 *    setApprovalPolicy/approveGuardianDenied/setThreadGoal);
 *  - 模型 10s 巡检循环(startModelInspection/stopModelInspection,scope 纪律与
 *    保活心跳同款;前台门由 UI 注入)。
 * RPC 统一经 [ChatRpcGateway](默认 [RealChatRpcGateway];JVM 单测换 fake 钉
 * Result 包装)。滚动锚定仍是 UI 副作用:VM 只回结果对象,UI 消费结果做
 * listState 操作;失败/toast 文案零变化。
 */
class ChatViewModel(stepRounds: Int = AgentConnection.historyRounds) {

    private val _state = MutableStateFlow(ChatStateReducer.initial(stepRounds))

    /** 页面级长期状态(离线快照/历史加载/身份索引/会话元数据/加载错误/斜杠命令镜像)。 */
    val state: StateFlow<ChatState> = _state.asStateFlow()

    /** 处理一条页面级意图(UI 只发意图;纯 reducer 落位,无副作用)。 */
    fun send(intent: ChatIntent) {
        _state.update { ChatStateReducer.reduce(it, intent) }
    }

    /**
     * 会话 RPC 网关(默认真连接;JVM 单测注入 fake 钉 Result 包装/异常透传)。
     * internal var 只为本模块测试开放,生产代码不换绑。
     */
    internal var gateway: ChatRpcGateway = RealChatRpcGateway()

    /**
     * 「头部拼接」前的增量裁定(纯查询):本次历史回包有多少条是真正的新消息。
     * 与 [ChatIntent.PrependApplied] 配对使用 —— 先按本裁定取新增段落地
     * st.items(ChatScreen 侧,对话流资产),再回执推进身份索引。
     */
    fun planPrepend(incomingMessageIds: List<String>): Int =
        ChatStateReducer.planPrependCount(_state.value, incomingMessageIds)

    // ---- 【P3 Task 8 第三层】TurnSession 交互协议面:只读投影 ----
    //
    // TurnSession.State 是 Compose 快照状态(mutableStateOf/mutableStateList)的
    // remember 单例,不是 StateFlow。照任务口径用 snapshotFlow 手写桥:它订阅块内
    // 读到的全部快照状态,任一变化且投影结果(data class equals)有变时发射一次,
    // 收敛进 StateFlow 单点分发。ChatScreen/子组件只 collect 本投影,不再各自
    // 直读 TurnSession 单例(读面收敛;写面与命令面仍归 ChatScreen 直持,
    // 迁留清单见 [TurnSessionUiState] KDoc)。

    private val sessionScope = CoroutineScope(SupervisorJob() + Dispatchers.Main.immediate)

    private val _session = MutableStateFlow(TurnSessionUiState())

    /** TurnSession 轮次状态的只读投影(生命周期/审批/错误/用量/模型/计划/派生标量)。 */
    val session: StateFlow<TurnSessionUiState> = _session.asStateFlow()

    private var sessionJob: Job? = null

    /**
     * 绑定一个会话的只读投影。**同步播种**当前值后再挂跟随收集 —— 若只靠收集,
     * 首帧会先渲染默认态再跳真值(重进正在运行的会话会闪一帧"空闲");播种保证
     * 首帧即真实值。幂等:重复调用换绑到新 threadId(旧收集取消)。
     * 由 ChatScreen 按 threadId 键控调用(构造期 remember 内),离开页面
     * [unbindSession] 解绑 —— scope 随 ViewModel 存续,解绑后无在飞任务,不留泄漏。
     */
    fun bindSession(threadId: String) {
        sessionJob?.cancel()
        _session.value = turnSessionUiStateOf(TurnSession.state(threadId))
        sessionJob = sessionScope.launch {
            // C3 保活心跳:页面开着 = 该会话"正被访问",TurnSession 空闲回收
            // 不收走正在被查看的会话(只保活这一个;通知轮询不算访问)。随
            // sessionJob 取消(unbind/换绑)一并停。
            launch {
                while (true) {
                    TurnSession.touch(threadId)
                    delay(SESSION_KEEPALIVE_MS)
                }
            }
            snapshotFlow { turnSessionUiStateOf(TurnSession.state(threadId)) }
                .collect { _session.value = it }
        }
    }

    /** 解绑只读投影的跟随收集(离开页面;投影值保留,下次 bind 重新播种)。 */
    fun unbindSession() {
        sessionJob?.cancel()
        sessionJob = null
    }

    // ---- 【C1 第三步】RPC 直调收进意图层:composable 只调 vm 方法 ----

    /**
     * 斜杠命令拉取(zcode/codex/pi 三源;迁自 ChatScreen 的 LaunchedEffect,
     * RPC/缓存/意图落位三段语义逐字保留):非面板会话清空镜像(缓存残留不漏);
     * 缓存命中先渲染;RPC 失败安静降级为空表照写,绝不 Toast 打断输入。
     */
    suspend fun refreshSlashCommands(threadId: String, agentId: String) {
        if (!isSlashPanelAgent(agentId)) {
            // 非面板会话(zcode/codex/pi 之外)不显示卡面(即使缓存里有别的会话留下的命令也不漏)
            send(ChatIntent.SlashCommandsSettled(emptyList()))
            return
        }
        val cached = SlashCommandCache.get(threadId)
        if (cached != null) send(ChatIntent.SlashCommandsSettled(cached))
        val loaded = try {
            withContext(Dispatchers.IO) {
                AgentConnection.threadService()
                    .withDeadlineAfter(SLASH_COMMANDS_DEADLINE_S, TimeUnit.SECONDS)
                    .listSlashCommands(
                        ListSlashCommandsRequest.newBuilder().setThreadId(threadId).setRefresh(true).build(),
                    )
                    .commandsList
                    .map { SlashCommandInfo(it.name, it.description, it.inputHint) }
            }
        } catch (_: Exception) {
            emptyList() // 拉不到就没有面板,不是错误
        }
        SlashCommandCache.put(threadId, loaded)
        send(ChatIntent.SlashCommandsSettled(loaded))
    }

    /**
     * 侧栏/模型芯片静态预取的四件套回包(迁自 ChatScreen 的 LaunchedEffect):
     * RPC 与"现值为空才写"的加载错误裁定在 VM;**失败项回 null,UI 侧赋值保留
     * 旧值**(与迁出前"catch 不覆盖已有值"同一语义)。四个加载互相独立,
     * 后两个(电脑名/最近会话)失败静默 —— 不影响当前会话。
     */
    data class StaticPrefetch(
        val codexSettings: CodexSettings? = null,
        val models: List<ModelInfo>? = null,
        val hostName: String? = null,
        val recentThreads: List<PThread>? = null,
    )

    suspend fun prefetchStatic(): StaticPrefetch {
        val settings = try {
            withContext(Dispatchers.IO) {
                AgentConnection.modelService()
                    .getCodexSettings(GetCodexSettingsRequest.newBuilder().build())
            }
        } catch (e: Exception) {
            // 不覆盖已有提示:离线快照的黄条比这里的红条更有信息量(真机验证发现
            // 两个并行加载共用 loadError,后完成者会把"离线:显示的是本地记录"顶掉)
            send(ChatIntent.LoadErrorSetIfAbsent("无法加载模型设置：${e.message ?: "连接失败"}"))
            null
        }
        val models = try {
            withContext(Dispatchers.IO) {
                AgentConnection.modelServiceSlow()
                    .listModels(ListModelsRequest.newBuilder().build()).modelsList
            }
        } catch (e: Exception) {
            send(ChatIntent.LoadErrorSetIfAbsent("无法加载模型列表：${e.message ?: "连接失败"}"))
            null
        }
        val hostName = try {
            withContext(Dispatchers.IO) {
                AgentConnection.hostService()
                    .getHostInfo(GetHostInfoRequest.getDefaultInstance()).displayName
            }
        } catch (_: Exception) {
            null // 电脑名取不到不影响聊天,侧边栏回退到通用文案
        }
        val recent = try {
            withContext(Dispatchers.IO) {
                AgentConnection.threadService()
                    .listThreads(
                        ListThreadsRequest.newBuilder()
                            .setIncludeArchived(false)
                            .build(),
                    )
                    .threadsList
            }
        } catch (_: Exception) {
            null // 拉不到就显示空态,不影响当前会话
        }
        return StaticPrefetch(settings, models, hostName, recent)
    }

    // ---- 【2026-10-09 效果层外移 · 任务1】打开会话加载:getThread + 历史回填 ----
    //
    // 迁自 ChatScreen 的 LaunchedEffect(threadId, connectionEpoch, reloadToken)
    // (~140 行)。RPC/TurnSession 写入/意图落位全部在 VM;滚动锚定是 UI 副作用,
    // 拆成三个阶段方法按原顺序调用,每个返回"是否需要滚到底"的结果对象 —— UI 层
    // 只消费结果做 listState.scrollToItem(st.items.size - 1)。失败路径语义逐字
    // 保留:loadError 横幅文案经既有意图原样落位,UI 不新增也不改写。

    /** 打开会话 getThread 成功后的 UI 本地投影回填值(threadModelId/threadGoalLine)。 */
    class ThreadHeadResult(
        /** daemon 落库的 Thread.ModelID(会话真实生效模型)。 */
        val modelId: String,
        /** 顶栏目标文案(GoalDisplayLogic 与 GetThread 同一套口径;null = 未设目标)。 */
        val goalLine: String?,
    )

    /** 历史尾部回填的结果:各 true 位对应迁出前 effect 的一次 scrollToItem(到底)。 */
    class HistoryTailResult(
        /** 在线回包已整表落地(迁出前 3312 行的滚动)。 */
        val backfilled: Boolean,
        /** 加载失败走了离线快照兜底(迁出前 3343 行的滚动)。 */
        val snapshotFallback: Boolean,
    )

    /**
     * 阶段①:进页离线快照预渲染(迁自效果开头 `if (st.items.isEmpty())` 块)。
     * 已知断网时先渲染本地只读副本(时间戳随行),黄条比红条有信息量(清加载错误)。
     *
     * @return true = 已渲染快照(UI 层按迁出前原式滚到底);false = 无快照,
     *   不伪造内容,继续在线加载(st.items 由调用方闸空,VM 不重复判)。
     */
    suspend fun prefillOfflineSnapshot(threadId: String): Boolean {
        val snap = withContext(Dispatchers.IO) { ThreadSnapshot.readThreadSnapshot(threadId) }
            ?: return false
        val st = TurnSession.state(threadId)
        st.items.clear()
        // 【深优 R2 修复】快照里的真实时间戳必须带上(否则历史导航日期分组失效)
        st.items.addAll(
            snap.map {
                ChatItem(role = it.role, text = it.text, sentAtMs = it.savedAtMs, id = nextChatItemId(it.role))
            },
        )
        TurnSession.enforceTimelineCap(st)
        // 离线快照标志经 ViewModel 意图落位(在线路径不动身份索引)
        send(ChatIntent.ShowOfflineSnapshot(ThreadSnapshot.messagesAt(threadId)))
        // 快照黄条比红条有信息量:清掉加载错误(原 loadError = null)
        send(ChatIntent.LoadErrorCleared)
        return true
    }

    /**
     * 阶段②:getThread 回包落地(迁自效果第一个 try/catch,语义逐字保留)。
     * 成功:会话元数据四项经 ThreadHeaderLoaded 落位、通知模块记会话名、
     * thread model/effort 空位回填、审批/恢复/错误三条原生写入、目标文案计算。
     * 失败:LoadErrorReported("无法加载会话：…")原文案落位并返回 null。
     *
     * @return null = 失败(UI 无需回填);非空 = UI 按原式回填 threadModelId/goalLine。
     */
    suspend fun loadThreadHead(threadId: String): ThreadHeadResult? {
        return try {
            val t = gateway.getThread(threadId)
            // 会话元数据四项(title/status/workspace/agentId)一次落位;三条原式
            // (空标题不覆盖/空工作区→null/空 agentId→"codex")收在 reducer 内。
            send(
                ChatIntent.ThreadHeaderLoaded(
                    title = t.title,
                    status = t.status,
                    workspace = t.workspace,
                    agentId = t.agentId,
                ),
            )
            // 【P1-6】顺手把会话名交给通知模块:后台轮次完成时,通知标题用真实会话名
            PaNotifications.rememberTitle(threadId, t.title)
            if (AgentConnection.threadModel(threadId) == null) AgentConnection.setThreadModel(threadId, t.modelId)
            if (AgentConnection.threadEffort(threadId) == null) AgentConnection.setThreadEffort(threadId, t.effort)
            val st = TurnSession.state(threadId)
            if (st.replyCh == null) st.pendingApproval = t.pendingApprovalsList.firstOrNull()
            if (!st.running && !st.recovering) {
                if (t.status == ThreadStatus.THREAD_STATUS_RUNNING) TurnSession.retryRecovery(threadId)
                else if (t.lastTurnError.isNotBlank()) st.errorMessage = t.lastTurnError
            }
            // 目标回读(只增):daemon 在 GetThread 里现读 codex 的 thread/goal/get
            ThreadHeadResult(modelId = t.modelId, goalLine = GoalDisplayLogic.goalLine(t))
        } catch (e: Exception) {
            send(ChatIntent.LoadErrorReported("无法加载会话：${e.message ?: "连接失败"}"))
            null
        }
    }

    /**
     * 阶段③:历史尾部按轮回填(迁自效果第二个 try/catch,语义逐字保留)。
     * 在线路径:回包整表落地(身份索引/锚点/截断位经意图一次落位)+ 本地只读
     * 副本落盘(时间戳随行);失败路径:有快照先渲染快照(身份索引一并作废),
     * 没有才报错 —— 与迁出前逐字一致。
     *
     * 注意与迁出前同款的字节级语义:`catch (e: Exception)` 连 CancellationException
     * 一并按失败处理(原实现如此,取消路径经兜底分支里的 withContext 再取消抛出),
     * 不"顺手修正"。
     *
     * @param connectionEpoch/reloadToken 原效果的键参数(门控三项之二,原式传回)。
     */
    suspend fun loadHistoryTail(
        threadId: String,
        rounds: Int,
        connectionEpoch: Int,
        reloadToken: Int,
    ): HistoryTailResult {
        val st = TurnSession.state(threadId)
        return try {
            var backfilled = false
            // 后台轮次进行中(或已有实时列表)时不覆盖(门控原式)
            if (!st.running && !st.recovering &&
                (st.items.isEmpty() || connectionEpoch > 0 || reloadToken > 0)
            ) {
                val resp = gateway.threadHistory(threadId, rounds)
                // 【F-D】是否被截断决定时间线顶部「加载更早」的显隐(纯函数在
                // HistoryRoundsLogic;空响应=还没有历史,不显示)
                val truncated = HistoryRoundsLogic.canLoadEarlier(
                    firstRole = resp.messagesList.firstOrNull()?.role,
                    roundCount = resp.messagesList.count { it.role == "user" },
                    requestedRounds = rounds,
                )
                if (resp.messagesList.isNotEmpty()) {
                    // 身份索引(message_id→时间线跨度)与「已加载最旧 id」锚点一次落位
                    val (items, spans) = buildHistoryBatch(resp.messagesList)
                    st.items.clear()
                    st.items.addAll(items)
                    // C3:整表回填后统一封顶(超 400 中间截断)
                    TurnSession.enforceTimelineCap(st)
                    // knownTotal 保留在 reducer 内(普通拉取不洗掉搜索回包的全量基准)
                    send(
                        ChatIntent.HistoryTailLoaded(
                            spans = spans,
                            oldestMessageId = resp.messagesList.first().messageId,
                            truncated = truncated,
                        ),
                    )
                    backfilled = true // UI 层在此结果位做 scrollToItem(原 3312 行)
                }
                // 截断位在 if 外落位(原语义:空回包也更新入口显隐,幂等)
                send(ChatIntent.TruncationSettled(truncated))
                // 【离线快照】成功后落一份本地只读副本(只留 role+text+时间戳)
                ThreadSnapshot.saveMessages(
                    threadId,
                    st.items
                        .filter { it.role != "tool" && it.attachments.isEmpty() && it.text.isNotBlank() }
                        .map { ThreadSnapshot.Msg(it.role, it.text, it.sentAtMs) },
                )
            }
            st.historyLoaded = true
            // 恢复在线撤掉残留的离线提示(flag + 落盘时间成对清)
            send(ChatIntent.BackOnline)
            // 只清「离线」前缀残留的条件收在 reducer(其它错误不在此处清)
            send(ChatIntent.LoadErrorClearedIfOffline)
            HistoryTailResult(backfilled = backfilled, snapshotFallback = false)
        } catch (e: Exception) {
            // 连不上:有快照就渲染快照(输入由 offlineSnapshot 停用),没有才报错
            val snap = withContext(Dispatchers.IO) { ThreadSnapshot.readThreadSnapshot(threadId) }
            if (!snap.isNullOrEmpty() && st.items.isEmpty()) {
                st.items.clear()
                st.items.addAll(
                    snap.map {
                        ChatItem(role = it.role, text = it.text, id = nextChatItemId(it.role))
                    },
                )
                TurnSession.enforceTimelineCap(st)
                // 快照条目没有 daemon 身份:身份索引一并作废(resetIdentity = true,
                // 绝不让旧索引把命中指进快照列表),快照兜底不报错(清加载错误)
                send(
                    ChatIntent.ShowOfflineSnapshot(
                        ThreadSnapshot.messagesAt(threadId),
                        resetIdentity = true,
                    ),
                )
                send(ChatIntent.LoadErrorCleared)
                HistoryTailResult(backfilled = false, snapshotFallback = true) // UI 滚到底(原 3343 行)
            } else {
                send(ChatIntent.LoadErrorReported("无法加载会话历史：${e.message ?: "连接失败"}"))
                send(ChatIntent.BackOnline)
                HistoryTailResult(backfilled = false, snapshotFallback = false)
            }
        }
    }

    /**
     * 「加载更早」的 RPC 与落地(迁自 ChatScreen.loadEarlier 的 scope.launch 体)。
     * daemon 幂等返回尾部,回包 = 更早新增段 + 已加载部分;头部拼接走
     * [prependHistory](只拼头部新消息,不做整列表替换)。滚动锚定在 UI:调用方
     * 在发起前已记 LazyListState 锚点(下标+偏移),按返回的 appendedCount 平移。
     *
     * @return receivedCount = daemon 回包条数(>0 才推进轮数与滚动,原式);
     *   appendedCount = 实际新拼入条数(滚动锚点 delta,回包无新消息时为 0)。
     */
    suspend fun fetchEarlierHistory(threadId: String, rounds: Int): EarlierHistoryResult {
        val resp = gateway.threadHistory(threadId, rounds)
        var appended = 0
        if (resp.messagesList.isNotEmpty()) {
            appended = prependHistory(threadId, resp.messagesList)
            send(ChatIntent.RoundsAdvanced(rounds))
        }
        // 身份索引的"已到全量"可信位跟随最新截断位(在 if 外,原式;与
        // HistoryTailLoaded 空回包路径同款幂等)
        send(
            ChatIntent.TruncationSettled(
                HistoryRoundsLogic.canLoadEarlier(
                    firstRole = resp.messagesList.firstOrNull()?.role,
                    roundCount = resp.messagesList.count { it.role == "user" },
                    requestedRounds = rounds,
                ),
            ),
        )
        return EarlierHistoryResult(receivedCount = resp.messagesList.size, appendedCount = appended)
    }

    /** [fetchEarlierHistory] 的结果对象(UI 只取两数做滚动锚定)。 */
    class EarlierHistoryResult(
        val receivedCount: Int,
        val appendedCount: Int,
    )

    /**
     * 【Task 7 加载更早升级】历史片段落地:按「已加载最旧 message id」锚点只把
     * 回包头部的新消息**拼到列表头部**,不做整列表替换(已加载条目引用不变,
     * LazyColumn 不整表重组;回包没有新消息时零列表变动)。旧 daemon 回包
     * message_id 为空时退「回包条数 - 已加载条数」条数兜底。
     * 全文搜索自动定位的缺口拉取(ChatScreen.resolveHitToListIndex)共用本方法。
     *
     * @return 实际新拼入的 ChatItem 条数(调用方的滚动锚点 delta 直接用)。
     */
    fun prependHistory(threadId: String, messages: List<ThreadMessage>): Int {
        if (messages.isEmpty()) return 0
        // 拼接量裁定收在 ChatStateReducer.planPrependCount(纯函数)
        val newCount = planPrepend(messages.map { it.messageId })
        if (newCount <= 0) return 0
        val (newItems, newSpans) = buildHistoryBatch(messages.take(newCount))
        val st = TurnSession.state(threadId)
        st.items.addAll(0, newItems)
        // C3:前插后统一封顶(超 400 中间截断;已加载条目引用稳定不变)
        TurnSession.enforceTimelineCap(st)
        // 身份索引把新 spans 拼到头部、既有 spans 的 itemStart 整体平移;
        // 最旧 id 只在回包带了非空 id 时覆盖(裁定在 reducer)
        send(ChatIntent.PrependApplied(newSpans, newItems.size, messages.first().messageId))
        return newItems.size
    }

    /**
     * ThreadMessage 列表 → (ChatItem 序列, 消息级身份跨度)(迁自 ChatScreen 的
     * 局部函数 buildHistoryBatch,原式)。历史映射一条 ThreadMessage ≥ 1 个
     * ChatItem(块拆分,见 HistoryMessageMapper),跨度记下每条消息映射出的
     * 连续段,搜索命中才能按 message_id 找回时间线下标。
     */
    private fun buildHistoryBatch(messages: List<ThreadMessage>): Pair<List<ChatItem>, List<MessageSpan>> {
        val items = mutableListOf<ChatItem>()
        val spans = mutableListOf<MessageSpan>()
        for ((i, m) in messages.withIndex()) {
            val start = items.size
            // C2:历史条目领稳定 id(daemon message_id 合成,旧 daemon 退序号)
            items += historyChatItemsWithIds(listOf(m))
            spans += MessageSpan(
                messageId = m.messageId,
                turnId = m.turnId,
                sequenceInResponse = i,
                itemStart = start,
                itemCount = items.size - start,
                role = m.role,
            )
        }
        return items to spans
    }

    // ---- 【2026-10-09 效果层外移 · 任务2】会话动作:VM 出 Result、UI 出 Toast ----
    //
    // 统一形态:suspend fun x(args): Result<X>(失败 = 原异常装进 Result.failure,
    // 文案零变化 —— UI 层在原 try/catch 位置 getOrThrow,异常从原 RPC 抛出点浮出,
    // catch/finally 的语义与迁出前逐位一致)。
    // 【取消语义逐字保留】迁出前的 catch(e: Exception) 连 CancellationException
    // 也按失败处理(取消路径同样走 toast/finally),rpcResult 不做特判修正。

    /** [clearQueuedInputs] 的回执条数(撤回人话回执的口径,原 resp.steered/followUp)。 */
    class QueueCounts(val steered: Int, val followUp: Int)

    private suspend fun <T> rpcResult(block: suspend () -> T): Result<T> = try {
        Result.success(block())
    } catch (e: Exception) {
        Result.failure(e)
    }

    /** 分叉:从当前会话的最新状态复制出一条新会话(forkThread,原 doFork 的 RPC 段)。 */
    suspend fun forkThread(threadId: String): Result<PThread> = rpcResult { gateway.forkThread(threadId) }

    /** 回滚最近 turns 个回合(rollbackThread,原 doRollback 的 RPC 段)。 */
    suspend fun rollbackThread(threadId: String, turns: Int): Result<Unit> =
        rpcResult { gateway.rollbackThread(threadId, turns) }

    /** 请求压缩当前会话上下文(compactThread,原 doCompact 的 RPC 段)。 */
    suspend fun compactThread(threadId: String): Result<Unit> =
        rpcResult { gateway.compactThread(threadId) }

    /** 发起计划审查(startReview"发起即回",原 doStartReview 的 RPC 段)。 */
    suspend fun startReview(threadId: String, focus: String): Result<Unit> =
        rpcResult { gateway.startReview(threadId, focus) }

    /** 撤回排队输入(clearQueuedInputs,原 clearQueuedInputs 的 RPC 段)。 */
    suspend fun clearQueuedInputs(threadId: String): Result<QueueCounts> = rpcResult {
        val resp = gateway.clearQueuedInputs(threadId)
        QueueCounts(steered = resp.steered, followUp = resp.followUp)
    }

    /**
     * 审批策略会话内无缝切换(setCodexSettings,原 setApproval 的 RPC 段)。
     * @param sandbox 当前设置里的 sandbox 原值(迁出前从乐观赋值后的本地变量读,
     *   两者 sandbox 相同 —— 乐观赋值只改 approvalPolicy);
     * @param approvalPolicy 目标策略 id。
     * @return daemon 回包的权威设置(成功后 UI 原式整包覆盖本地 codexSettings)。
     */
    suspend fun setApprovalPolicy(sandbox: String, approvalPolicy: String): Result<CodexSettings> =
        rpcResult { gateway.setCodexSettings(approvalPolicy = approvalPolicy, sandbox = sandbox) }

    /** 推翻审查者(guardian)的驳回结论(approveGuardianDeniedAction,原 approveGuardian 的 RPC 段)。 */
    suspend fun approveGuardianDenied(
        threadId: String,
        reviewId: String,
        targetItemId: String,
    ): Result<Unit> = rpcResult { gateway.approveGuardianDeniedAction(threadId, reviewId, targetItemId) }

    /** 设置会话目标(setThreadGoal,原 ChatGoalSheet 的 RPC 段;目标原文照传)。 */
    suspend fun setThreadGoal(threadId: String, goal: String): Result<PThread> =
        rpcResult { gateway.setThreadGoal(threadId, goal) }

    // ---- 【2026-10-09 效果层外移 · 任务3】模型 10s 巡检循环 ----
    //
    // 迁自 ChatScreen 的 LaunchedEffect(threadId, sessionAgent, connectionEpoch)
    // (~45 行循环体):前台门控注入(UI 持 Activity Lifecycle,经 [foregroundGate]
    // 原式挂观察者)、循环体(RPC/模型表合并/原生选择回填/仓库刷新)在 VM。
    // scope 纪律与保活心跳同款:挂 sessionScope,离开页面随 DisposableEffect
    // 的 onDispose 走 [stopModelInspection];换键重启 = UI 的 LaunchedEffect 重入
    // [startModelInspection](先停旧循环,等价原效果的取消重建)。

    private var inspectionJob: Job? = null

    /**
     * 启动(或换键重启)模型巡检循环:10s 节奏、前台门控、连接 ok 才发 RPC;
     * ListModels 回包按 agent 前缀合并进模型大表(ModelListMerged,"先比对再赋值"
     * 在 reducer)、原生选择回填 acceptRuntimeSelection、仓库 refresh(仅当仓库
     * 仍停在本会话,竞态安全:refresh 自带在飞守卫)。
     *
     * @param foregroundGate 前台门(迁自原效果体开头):不在 STARTED 时挂起等待
     *   ON_START,回前台立即补跑再进 10s 节奏。由 UI 从宿主 Activity 的 Lifecycle
     *   构造(Activity 资产不进 VM),每次调用注册新观察者、finally 必摘。
     */
    fun startModelInspection(
        threadId: String,
        agentId: String,
        foregroundGate: suspend () -> Unit,
    ) {
        stopModelInspection()
        inspectionJob = sessionScope.launch {
            while (true) {
                // 前台门控:STOP 状态不跑 RPC;挂起等 ON_START
                foregroundGate()
                if (agentId.isNotEmpty() && AgentConnection.connState.value == "ok") {
                    try {
                        val native = gateway.listModels(agentId, threadId)
                        if (native.modelsCount > 0) {
                            // 先比对再赋值在 reducer(内容没变不换实例,消除每 10s
                            // 一次的空转重组);按 id 首段过滤(与 modelsForAgent 同口径)
                            send(ChatIntent.ModelListMerged(agentId, native.modelsList))
                        }
                        TurnSession.acceptRuntimeSelection(
                            threadId,
                            native.currentModelId,
                            if (native.hasCurrentEffort()) native.currentEffort else null,
                        )
                        // 巡检回包喂仓库:GetThread(24)+ListModels(5) 真值整树替换,
                        // 选择状态只有仓库一个写者;失败只标仓库 error,不影响本巡检
                        if (ModelSelectionRepository.currentThreadId() == threadId) {
                            ModelSelectionRepository.refresh()
                        }
                    } catch (e: kotlinx.coroutines.CancellationException) {
                        throw e
                    } catch (_: Exception) {
                        // Keep last verified runtime state during network recovery.
                    }
                }
                delay(10_000)
            }
        }
    }

    /** 停止巡检循环(离开页面/换键重启前;与 unbindSession 同一 onDispose 成对)。 */
    fun stopModelInspection() {
        inspectionJob?.cancel()
        inspectionJob = null
    }

    private companion object {
        /** 页面保活心跳节奏(远小于 TurnSession 的 30 分钟空闲阈值)。 */
        const val SESSION_KEEPALIVE_MS = 60_000L
    }
}

/**
 * TurnSession 单例状态 → [TurnSessionUiState] 只读投影的适配边(纯映射,不写
 * 任何状态)。items/streamText/thinkingText/planStream/draft 等高频流与写入面
 * 不进投影(理由见 [TurnSessionUiState] KDoc「留了什么」);派生标量与迁出前
 * ChatScreen 的原地派生表达式逐字同式。
 */
internal fun turnSessionUiStateOf(st: TurnSession.State): TurnSessionUiState = TurnSessionUiState(
    running = st.running,
    streaming = st.streaming,
    recovering = st.recovering,
    turnTerminal = st.turnTerminal,
    pendingApproval = st.pendingApproval,
    approvalError = st.approvalError,
    errorMessage = st.errorMessage,
    recoveryError = st.recoveryError,
    runningModel = st.runningModel,
    runningEffort = st.runningEffort,
    queuedCount = st.queuedCount,
    historyLoaded = st.historyLoaded,
    turnUsage = st.turnUsage,
    totalUsage = st.totalUsage,
    liveContextWindow = st.liveContextWindow,
    compactionInProgress = st.compactionInProgress,
    lastCompactedAt = st.lastCompactedAt,
    guardianOverridden = st.guardianOverridden,
    // 派生标量:原 ChatScreen 原地派生同式上收(时间线 plan 卡取最新,流式兜底)
    itemsEmpty = st.items.isEmpty(),
    itemCount = st.items.size,
    lastQueuedIndex = st.items.indexOfLast { it.deliverState == DELIVERY_QUEUED },
    userTurnCount = st.items.count { it.role == "user" },
    latestPlan = st.items.lastOrNull { it.role == "plan" && it.planSteps.isNotEmpty() }?.planSteps
        ?: st.planSteps,
)
