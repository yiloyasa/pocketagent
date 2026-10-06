package dev.pocketagent.app.ui.chat

import dev.pocketagent.app.ui.HistoryRoundsLogic
import dev.pocketagent.app.ui.SlashCommandInfo
import dev.pocketagent.app.ui.history.HistoryIdentityIndex
import dev.pocketagent.app.ui.history.MessageSpan
import pocketagent.v1.ThreadStatus

/**
 * ChatScreen 页面级长期状态(2026-10-04 会话选择与历史体验计划 Task 8,第一层分层)。
 *
 * 【它是什么】ChatScreen 里散落的七项页面级状态 —— 离线快照([offlineSnapshot]/
 * [offlineSnapshotAt])、历史按轮加载([roundsLoaded]/[truncated]/[loadingEarlier])、
 * 消息级身份索引([identity])与「已加载最旧 id」锚点([oldestMessageId]) —— 收拢成
 * 一棵可 reducer 化的状态树,照 CollabSessionState/ModelSelectionState 先例:
 * 纯数据 + 纯函数([ChatStateReducer]),无 Composable、无 gRPC、无 Android 类依赖,
 * JVM 单测钉死(ChatStateReducerTest)。
 *
 * 【第一层范围红线】只收本轮点名的页面级长期状态;以下仍留在 ChatScreen,归后续轮次:
 *  - 对话流(st.items/streamText/审批)—— TurnSession 单例资产,禁改;
 *  - 语音/提示词优化/模型选择/全文搜索浮层 —— 四个既有接线段的装配与 RPC 通道
 *    原样保留(本轮零改动,Task 8 的验证重点就是别破坏它们);
 *  - 浮层开合、滚动跟随等瞬态 UI 状态 —— 非长期状态,不在分层清单里。
 *
 * 【字段语义与原 ChatScreen 局部变量一一对应】迁出前写在 ChatScreen 函数体内
 * (2266-2293 行一带),注释原样搬移;写入通道全部走 [ChatIntent](见 reducer)。
 *
 * 【第二层增量】会话元数据四项([threadTitle]/[threadStatus]/[threadWorkspace]/
 * [threadAgentId],getThread 回填)、历史加载错误([loadError])与斜杠命令的
 * Compose 侧镜像([slashCommands],进程级 SlashCommandCache 仍留在 ChatScreen,
 * 这里只收面板渲染用的列表)照同一手法迁入:纯数据 + 纯 reducer 落位,
 * 四接线段(优化/搜索/语音/模型卡)引用符号零触碰。
 */
data class ChatState(
    /**
     * 聊天区当前是否是"离线快照"(决定错误条用提示色而非报错色,并提示输入框不可用)。
     * 【无会话键的语义保真】原实现该标志就是无 remember 键(跨会话沿用,靠
     * getThread 效果重跑归位);本层随 ChatViewModel 换会话整体重建归 false ——
     * 效果重跑前不闪上一会话的横幅,方向更干净,行为兼容。
     */
    val offlineSnapshot: Boolean = false,

    /** 离线快照的落盘时间(横幅「离线快照 · HH:mm」用;0 = 无)。 */
    val offlineSnapshotAt: Long = 0L,

    /**
     * 【F-D 历史按轮加载】当前已拉取的累计轮数:打开会话=设置档位起步,
     * 「加载更早」按档位累加;重连/下拉刷新沿用累计值(视图不缩水)。
     * 只作用于打开会话路径 —— 导出/恢复/resync 保持全量,不读这个值。
     * 初始值由 ChatViewModel 构造时按设置档位给([ChatStateReducer.initial]),
     * 状态树自身不读 SharedPreferences(保持纯数据可测)。
     */
    val roundsLoaded: Int = HistoryRoundsLogic.DEFAULT_ROUNDS,

    /** daemon 返回的是不是"被截断的尾部"(决定时间线顶部「加载更早」的显隐)。 */
    val truncated: Boolean = false,

    /** 「加载更早」在飞(防重复点击;按钮同时换成小圈)。 */
    val loadingEarlier: Boolean = false,

    /**
     * 【Task 7 历史搜索/自动定位】已加载历史的消息级身份索引:每条已加载 daemon
     * 消息 → 时间线 ChatItem 连续段(message_id 主键定位的数据底座);每次历史
     * 拉取(打开会话/加载更早/自动定位)后由 reducer 重建或前插推进。
     */
    val identity: HistoryIdentityIndex = HistoryIdentityIndex(),

    /**
     * 已加载最旧消息的 daemon 稳定 id(「加载更早」防重复拉取的锚点);
     * 空 = 旧 daemon 未带 id,退「回包条数 - 已加载条数」兜底。
     */
    val oldestMessageId: String = "",

    /**
     * 会话标题(getThread 回填;顶栏/侧边栏/搜索命中卡/导出文件名共用)。
     * 【PF4 原注释】按 threadId 记忆 —— getThread 用 t.title 回填且空标题不覆盖,
     * 不带键时换到无标题会话会永远顶着上一会话的标题;本层随 ChatViewModel 换
     * 会话整体重建回落「任务」,与原 remember(threadId) 键控同语义。
     * 初始值「任务」= 迁出前 mutableStateOf 的初值。
     */
    val threadTitle: String = "任务",

    /**
     * daemon 上报的会话状态(getThread 回填;四词状态徽章的兜底数据源。
     * 运行中/恢复中由 st.running 实时覆盖,待审批由 st.pendingApproval 实时
     * 覆盖 —— 徽章口径仍在 ChatScreen 的 identityStatus 装配处)。
     */
    val threadStatus: ThreadStatus = ThreadStatus.THREAD_STATUS_UNSPECIFIED,

    /**
     * 会话工作区(getThread 回填;文件上传回填 PC 路径、抽屉/导出/Git 面板共用)。
     * null = 尚未回填或 daemon 报空([ChatIntent.ThreadHeaderLoaded] 内
     * `ifBlank { null }` 原式);【PF4】换会话随 ViewModel 重建归 null。
     */
    val threadWorkspace: String? = null,

    /**
     * 会话归属 agent(getThread 回填;斜杠面板开关/品牌图标/能力位文案共用)。
     * 空 = getThread 未回填(投影后由读点的 ifBlank 兜底,如 sessionAgent);
     * daemon 报空时 reducer 兜 "codex"(原式);【PF4】换会话随 ViewModel 重建归 ""。
     */
    val threadAgentId: String = "",

    /**
     * 页面级加载错误(统一提示条的「页面加载错误」档;模型设置/模型列表/会话/
     * 会话历史四路加载共用 —— 真机教训:两个并行加载共用一格,后完成者不得把
     * 先到的提示顶掉,见 [ChatIntent.LoadErrorSetIfAbsent])。
     * 【语义保真注】原实现是无键 remember(跨会话沿用,靠后续加载覆盖);本层随
     * ChatViewModel 换会话整体重建归 null —— 上一会话的失败横幅不再带进新会话
     * (getThread 效果重跑本就会重裁定),方向与第一层 offlineSnapshot 同款。
     */
    val loadError: String? = null,

    /**
     * 斜杠命令面板的候选列表(zcode/codex/pi 三源 daemon 回包的 Compose 侧镜像;
     * 面板显隐/候选合并仍在 ChatScreen 走 SlashCommandsLogic 纯逻辑)。
     * 进程级去重缓存 SlashCommandCache 原样留在 ChatScreen 的拉取效果内 ——
     * 本字段只是渲染镜像,与缓存对象无双向耦合(拉取 RPC 通道零改动)。
     */
    val slashCommands: List<SlashCommandInfo> = emptyList(),
) {
    /**
     * 时间线列表的头部偏移:时间线顶部挂「加载更早」入口时 itemsIndexed 的下标
     * 整体 +1(与该入口的显隐条件严格一致 —— 原式在 ChatScreen 原地派生,语义
     * 原样上收,消费方:历史刻度条定位/搜索命中跳转的下标换算)。
     */
    val listOffset: Int
        get() = if (truncated && !offlineSnapshot) 1 else 0
}

/**
 * 页面级长期状态的意图(Task 8 第一层的唯一写入通道)。
 * 与 ModelSelectionIntent 同一纪律:UI 只发意图,不直接改状态树;每条意图在
 * [ChatStateReducer.reduce] 里有且只有一个纯函数落点,语义与迁出前的原地赋值
 * 逐字对应(写点对照见各意图 KDoc)。
 */
sealed interface ChatIntent {

    /**
     * 进入离线快照展示。对应迁出前 getThread 效果里的两处:
     *  - 在线加载前先渲染本地快照(3110 行一带):只置位,身份索引不动;
     *  - 断线兜底分支(3205 行一带):快照条目没有 daemon 身份,身份索引与
     *    最旧 id 锚点**一并作废**(离线下搜索本就不可达,重连重建 —— 绝不让
     *    旧索引把命中指进快照列表),由 [resetIdentity] 表达。
     *
     * @param atMs 快照落盘时间(横幅「离线快照 · HH:mm」);0 = 未知(横幅退化为不带时间)。
     * @param resetIdentity true = 同时清空身份索引与最旧 id(断线快照分支)。
     */
    data class ShowOfflineSnapshot(val atMs: Long, val resetIdentity: Boolean = false) : ChatIntent

    /**
     * 恢复在线/加载失败无快照:撤掉快照态(flag + 落盘时间都清)。
     * 对应迁出前的 `offlineSnapshot = false; offlineSnapshotAt = 0` 成对赋值
     * (在线恢复分支与断线无快照的报错分支,两处同款)。
     */
    data object BackOnline : ChatIntent

    /**
     * 只清快照标志、不清落盘时间。对应错误条「重试」按钮的迁出前语义
     * (`offlineSnapshot = false`,At 保留 —— reloadToken 驱动的重载会重新裁定)。
     */
    data object ClearSnapshotFlag : ChatIntent

    /**
     * 清快照标志 + 落盘时间。对应离线横幅上「重新加载」按钮的迁出前语义
     * (`offlineSnapshot = false; offlineSnapshotAt = 0`)。
     */
    data object ClearSnapshot : ChatIntent

    /**
     * 打开会话的历史尾部回包落地(reducer 内一次性推进,替代迁出前 3169-3177
     * 行的四连赋值):身份索引按本次 spans 整表重建(保留 knownTotal —— 全量
     * 基准来自搜索回包,不因普通拉取丢失)、最旧 id **无条件**覆盖(旧 daemon
     * 空 id 也照写,与原行为一致)、截断位落位(fullLoaded 跟随)。
     */
    data class HistoryTailLoaded(
        val spans: List<MessageSpan>,
        val oldestMessageId: String,
        val truncated: Boolean,
    ) : ChatIntent

    /**
     * 「头部拼接」落地完成(replace 不做整列表替换的那条路径):身份索引把新
     * spans 拼到头部、既有 spans 的 itemStart 整体平移;最旧 id 只在回包带了
     * 非空 id 时覆盖(空 = 旧 daemon,保留旧锚点)。对应迁出前
     * prependHistoryMessages 内的身份索引/锚点两段赋值(items 落地仍在
     * ChatScreen —— 那是对话流资产)。
     */
    data class PrependApplied(
        val newSpans: List<MessageSpan>,
        val newItemCount: Int,
        val incomingOldestId: String,
    ) : ChatIntent

    /** 累计轮数直接推进(「加载更早」的档位累加,无条件覆盖)。 */
    data class RoundsAdvanced(val rounds: Int) : ChatIntent

    /**
     * 累计轮数只在更大时推进(搜索自动定位的缺口拉取:定位可能重复请求同一
     * 档位,不回退 —— 对应迁出前 `if (plan.requestedRounds > historyRoundsLoaded)`
     * 的条件赋值)。
     */
    data class RoundsAdvancedIfBeyond(val rounds: Int) : ChatIntent

    /**
     * 截断位落位,身份索引的「已到全量」可信位跟随(sequence 兼容定位用)。
     * 对应迁出前三处 `historyTruncated = …; historyIdentity = copy(fullLoaded = !…)`
     * 的成对赋值(打开会话/加载更早/定位拉取同款)。
     */
    data class TruncationSettled(val truncated: Boolean) : ChatIntent

    /** 「加载更早」开始(防重入闸落下;对应 `loadingEarlier = true`)。 */
    data object LoadEarlierStarted : ChatIntent

    /** 「加载更早」结束(finally 语义,成败都放闸;对应 `loadingEarlier = false`)。 */
    data object LoadEarlierFinished : ChatIntent

    /**
     * 全文搜索回包报来的 daemon 全量实扫条数(sequence 坐标系基准);0(旧
     * daemon 未报)**不覆盖**已有值。对应迁出前 onSearchSearched 的条件赋值。
     */
    data class KnownTotalObserved(val total: Int) : ChatIntent

    // ---- 第二层:会话元数据 / 加载错误 / 斜杠命令镜像 ----

    /**
     * getThread 回包的会话元数据四项一次落位。对应迁出前 getThread 成功分支的
     * 三条原式(reducer 内逐字保留,调用方传 daemon 原值):
     *  - `if (t.title.isNotEmpty()) threadTitle = t.title`(空标题不覆盖,否则
     *    换到无标题会话会顶着上一会话的标题);
     *  - `threadWorkspace = t.workspace.ifBlank { null }`;
     *  - `threadAgentId = t.agentId.ifBlank { "codex"}`。
     * status 无条件覆盖(`threadStatus = t.status` 原式)。
     */
    data class ThreadHeaderLoaded(
        val title: String,
        val status: ThreadStatus,
        val workspace: String,
        val agentId: String,
    ) : ChatIntent

    /**
     * 会话工作区直接落位。对应迁出前文件上传(pickFiles)回填的
     * `threadWorkspace = ws` 原式 —— ws 是本地已解析的非空路径
     * (投影值兜底 → RPC getThread → 双 elvis),无条件覆盖。
     */
    data class WorkspaceResolved(val workspace: String) : ChatIntent

    /**
     * 页面加载错误无条件落位。对应迁出前 getThread 失败与会话历史加载失败两处
     * `loadError = "无法加载会话[历史]：…"`(本会话的最新失败就是当前事实)。
     */
    data class LoadErrorReported(val message: String) : ChatIntent

    /**
     * 页面加载错误只在现值为空时落位。对应迁出前模型设置/模型列表两个并行加载
     * 的 `if (loadError == null) loadError = "…"` 原式 —— 后完成者不顶掉先到的
     * 提示(真机教训)。条件收在 reducer 内对权威现值判定,不读投影(投影存在
     * 一帧滞后,原地判会把先到的错误顶掉 —— 与 RoundsAdvancedIfBeyond 同理)。
     */
    data class LoadErrorSetIfAbsent(val message: String) : ChatIntent

    /**
     * 页面加载错误清空。对应迁出前四处 `loadError = null`:离线快照分支两处
     * (getThread 快照命中/断线快照兜底,快照黄条比红条有信息量)与两处用户
     * 重试(顶栏状态点/统一提示条「重试」按钮)。
     */
    data object LoadErrorCleared : ChatIntent

    /**
     * 只清「离线」前缀的残留错误。对应迁出前历史加载成功分支的
     * `if (loadError?.startsWith("离线") == true) loadError = null` 原式 ——
     * 前缀字面量是错误文案的约定(离线提示统一以「离线」开头),收在 reducer
     * 纯函数内可被 JVM 单测钉死;其它错误(真失败)不在此处清。
     */
    data object LoadErrorClearedIfOffline : ChatIntent

    /**
     * 斜杠命令镜像无条件替换。对应迁出前拉取效果内三处同款赋值:
     *  - 非面板会话清空(`slashCommands = emptyList()`,即使缓存里有别的会话
     *    残留也不漏);
     *  - 缓存命中先渲染(`if (cached != null) slashCommands = cached`,调用点
     *    保留 null 闸 —— 那是对本地变量的新鲜判定,无投影滞后问题);
     *  - RPC 回包落位(`slashCommands = loaded`,失败安静降级为空表照写)。
     */
    data class SlashCommandsSettled(val commands: List<SlashCommandInfo>) : ChatIntent
}

/**
 * ChatState 的纯函数 reducer(Task 8 第一层)。全部规则 JVM 可断言,改坏任何一条
 * 迁移语义在 ChatStateReducerTest 里红,而不是在真机上被看见。
 */
object ChatStateReducer {

    /**
     * 初始态:累计轮数从设置档位收敛([HistoryRoundsLogic.clampRounds],prefs
     * 脏值兜底回默认档)。轮数的读取在 ChatViewModel 构造时经既有访问器完成,
     * 本函数只接数值 —— 状态树保持无环境依赖。
     */
    fun initial(stepRounds: Int): ChatState = ChatState(
        offlineSnapshot = false,
        offlineSnapshotAt = 0L,
        roundsLoaded = HistoryRoundsLogic.clampRounds(stepRounds),
        truncated = false,
        loadingEarlier = false,
        identity = HistoryIdentityIndex(),
        oldestMessageId = "",
        // 第二层缺省与迁出前各 remember 初值逐字对应(换会话随 ViewModel 重建回落)
        threadTitle = "任务",
        threadStatus = ThreadStatus.THREAD_STATUS_UNSPECIFIED,
        threadWorkspace = null,
        threadAgentId = "",
        loadError = null,
        slashCommands = emptyList(),
    )

    /** 头部拼接前的增量裁定(纯查询,不改状态):本次回包实际新增多少条。 */
    fun planPrependCount(state: ChatState, incomingMessageIds: List<String>): Int {
        if (incomingMessageIds.isEmpty()) return 0
        // daemon 幂等回尾部:回包里能找到「已加载最旧 id」时,它之前的都是新消息。
        // (原语义:锚点按 id 文本比对;id 均非空的正常路径。)
        val anchorPos = if (state.oldestMessageId.isNotBlank()) {
            incomingMessageIds.indexOfFirst { it == state.oldestMessageId }
        } else -1
        // 旧 daemon 无 id:退「回包条数 - 已加载条数」条数兜底(负值钳 0)。
        return if (anchorPos >= 0) {
            anchorPos
        } else {
            (incomingMessageIds.size - state.identity.spans.size).coerceAtLeast(0)
        }
    }

    /** 一条意图 → 新状态(不可变;语义见各意图 KDoc)。 */
    fun reduce(state: ChatState, intent: ChatIntent): ChatState = when (intent) {
        is ChatIntent.ShowOfflineSnapshot -> if (intent.resetIdentity) {
            state.copy(
                offlineSnapshot = true,
                offlineSnapshotAt = intent.atMs,
                identity = HistoryIdentityIndex(),
                oldestMessageId = "",
            )
        } else {
            state.copy(offlineSnapshot = true, offlineSnapshotAt = intent.atMs)
        }

        ChatIntent.BackOnline -> state.copy(offlineSnapshot = false, offlineSnapshotAt = 0L)

        ChatIntent.ClearSnapshotFlag -> state.copy(offlineSnapshot = false)

        ChatIntent.ClearSnapshot -> state.copy(offlineSnapshot = false, offlineSnapshotAt = 0L)

        is ChatIntent.HistoryTailLoaded -> state.copy(
            identity = HistoryIdentityIndex(
                spans = intent.spans,
                knownTotal = state.identity.knownTotal,
                fullLoaded = !intent.truncated,
            ),
            oldestMessageId = intent.oldestMessageId,
            truncated = intent.truncated,
        )

        is ChatIntent.PrependApplied -> state.copy(
            identity = state.identity.copy(
                spans = intent.newSpans + state.identity.spans.map {
                    it.copy(itemStart = it.itemStart + intent.newItemCount)
                },
            ),
            oldestMessageId = if (intent.incomingOldestId.isNotBlank()) {
                intent.incomingOldestId
            } else {
                state.oldestMessageId
            },
        )

        is ChatIntent.RoundsAdvanced -> state.copy(roundsLoaded = intent.rounds)

        is ChatIntent.RoundsAdvancedIfBeyond ->
            if (intent.rounds > state.roundsLoaded) state.copy(roundsLoaded = intent.rounds) else state

        is ChatIntent.TruncationSettled -> state.copy(
            truncated = intent.truncated,
            identity = state.identity.copy(fullLoaded = !intent.truncated),
        )

        ChatIntent.LoadEarlierStarted -> state.copy(loadingEarlier = true)

        ChatIntent.LoadEarlierFinished -> state.copy(loadingEarlier = false)

        is ChatIntent.KnownTotalObserved ->
            if (intent.total > 0) {
                state.copy(identity = state.identity.copy(knownTotal = intent.total))
            } else {
                state
            }

        // ---- 第二层:会话元数据 / 加载错误 / 斜杠命令镜像 ----

        is ChatIntent.ThreadHeaderLoaded -> state.copy(
            threadTitle = if (intent.title.isNotEmpty()) intent.title else state.threadTitle,
            threadStatus = intent.status,
            threadWorkspace = intent.workspace.ifBlank { null },
            threadAgentId = intent.agentId.ifBlank { "codex" },
        )

        is ChatIntent.WorkspaceResolved -> state.copy(threadWorkspace = intent.workspace)

        is ChatIntent.LoadErrorReported -> state.copy(loadError = intent.message)

        is ChatIntent.LoadErrorSetIfAbsent ->
            if (state.loadError == null) state.copy(loadError = intent.message) else state

        ChatIntent.LoadErrorCleared -> state.copy(loadError = null)

        ChatIntent.LoadErrorClearedIfOffline ->
            if (state.loadError?.startsWith("离线") == true) state.copy(loadError = null) else state

        is ChatIntent.SlashCommandsSettled -> state.copy(slashCommands = intent.commands)
    }
}
