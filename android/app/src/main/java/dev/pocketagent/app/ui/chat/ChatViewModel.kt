package dev.pocketagent.app.ui.chat

import androidx.compose.runtime.snapshotFlow
import dev.pocketagent.app.AgentConnection
import dev.pocketagent.app.ui.DELIVERY_QUEUED
import dev.pocketagent.app.ui.SLASH_COMMANDS_DEADLINE_S
import dev.pocketagent.app.ui.SlashCommandCache
import dev.pocketagent.app.ui.SlashCommandInfo
import dev.pocketagent.app.ui.TurnSession
import dev.pocketagent.app.ui.isSlashPanelAgent
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
import pocketagent.v1.CodexSettings
import pocketagent.v1.GetCodexSettingsRequest
import pocketagent.v1.GetHostInfoRequest
import pocketagent.v1.ListModelsRequest
import pocketagent.v1.ListSlashCommandsRequest
import pocketagent.v1.ListThreadsRequest
import pocketagent.v1.ModelInfo
import pocketagent.v1.Thread as PThread
import java.util.concurrent.TimeUnit

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
