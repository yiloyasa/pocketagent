package dev.pocketagent.app.ui.chat

import dev.pocketagent.app.AgentConnection
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update

/**
 * ChatScreen 页面级长期状态的持有者(2026-10-04 计划 Task 8,第一层分层;
 * 第二层增量:会话元数据四项/加载错误/斜杠命令镜像照同一形态收拢)。
 *
 * 【形态】项目没有 viewModel 依赖(禁加),按计划口径用普通 class:构造即持有
 * 一棵 [ChatState] 的 StateFlow;ChatScreen 只 collect([ChatRoute] 构造并传入,
 * 直连入口在 ChatScreen 内兜底自建)。照 ModelSelectionRepository 的
 * "单一投影点 + 意图入口"先例,但更薄 —— 两层迁出的全是**同步状态推进**
 * (RPC 仍留在 ChatScreen 的既有接线段里,零搬动),所以**不需要 CoroutineScope**:
 * 每条意图都是一次 [ChatStateReducer.reduce] 纯函数调用,经 StateFlow.update
 * 原子落位(全部调用点在主线程:组合回调/LaunchedEffect)。后续轮次接管
 * 历史拉取 RPC 时再引入 scope,本轮不备而不用(不加死代码)。
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
}
