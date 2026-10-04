package dev.pocketagent.app.ui.chat

import dev.pocketagent.app.ui.HistoryRoundsLogic
import dev.pocketagent.app.ui.history.HistoryIdentityIndex
import dev.pocketagent.app.ui.history.MessageSpan

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
    }
}
