package dev.pocketagent.app.ui

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import dev.pocketagent.app.AgentConnection
import dev.pocketagent.app.ui.component.TaskCenter
import dev.pocketagent.app.ui.component.ThreadUsageStore
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.channels.awaitClose
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.callbackFlow
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import pocketagent.v1.ApprovalDecision
import pocketagent.v1.ApprovalRequest
import pocketagent.v1.CancelSignal
import pocketagent.v1.CancelTurnRequest
import pocketagent.v1.ContentBlock
import pocketagent.v1.CodexEvent
import pocketagent.v1.GetThreadRequest
import pocketagent.v1.GetThreadHistoryRequest
import pocketagent.v1.ImageBlock
import pocketagent.v1.TextBlock
import pocketagent.v1.ThreadStatus
import pocketagent.v1.ToolCallUpdate
import pocketagent.v1.TurnClientMessage
import pocketagent.v1.TurnInput
import pocketagent.v1.TurnServerMessage

/**
 * 轮次会话单例 —— 解决"退出对话界面,之前发的消息就不工作了":
 * 轮次的 gRPC 双向流跑在 app 级作用域,不随聊天页销毁。
 * 聊天页只是它的"视图":进入页面按 threadId 拿到同一份 State,
 * 流式气泡/工具卡/审批请求都会在后台继续更新,回来自动接着看。
 */
object TurnSession {
    class State {
        val items = mutableStateListOf<ChatItem>()
        var streamText by mutableStateOf("")
        var thinkingText by mutableStateOf("")
        /** 本轮 token 用量(上下文窗口交互卡用)。 */
        var turnUsage by mutableStateOf<pocketagent.v1.Usage?>(null)
        /** 本会话累计用量。 */
        var totalUsage by mutableStateOf<pocketagent.v1.Usage?>(null)
        /** daemon 随用量上报的真实上下文窗口大小(0=未知)。 */
        var liveContextWindow by mutableStateOf(0L)
        /** 当前执行计划(turn/plan/updated,"status|text" 列表);轮次结束落成计划卡。 */
        var planSteps by mutableStateOf(listOf<String>())
        /** 计划生成中的流式草稿(item/plan/delta)。 */
        var planStream by mutableStateOf("")
        /** 本轮实际生效的模型(codex 可能按桌面配置重定向)。 */
        var runningModel by mutableStateOf("")
        var running by mutableStateOf(false)
        var streaming by mutableStateOf(false)
        var recovering by mutableStateOf(false)
        var recoveryError by mutableStateOf<String?>(null)
        var errorMessage by mutableStateOf<String?>(null)
        var codexEvents by mutableStateOf(listOf<pocketagent.v1.CodexEvent>())
        /**
         * 已经被用户推翻过的审查 id。
         *
         * 【为什么要记在这里而不是改事件对象】CodexEvent 是不可变 proto,
         * 用户点"仍然执行"之后我们不能改那张卡上的事件;而"按钮要消失"
         * 又必须立刻反映到 UI 上。所以单独存一份集合,由卡片渲染时查询。
         * 放在 State 里 = 离开页面再回来仍然是"已推翻",不会重新给一个点了会报错的按钮。
         */
        var guardianOverridden by mutableStateOf(setOf<String>())
        var historyLoaded by mutableStateOf(false)
        var pendingApproval by mutableStateOf<ApprovalRequest?>(null)
        var approvalError by mutableStateOf<String?>(null)
        var replyCh: Channel<TurnClientMessage>? = null
    }

    private val states = java.util.concurrent.ConcurrentHashMap<String, State>()

    /** app 级作用域:导航离开聊天页也不会取消协程。 */
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Default)

    fun state(threadId: String): State = states.getOrPut(threadId) {
        State().also { fresh ->
            // 会话用量像缓存一样回显:daemon 的历史接口不带用量,只靠内存的话
            // 每次重开会话上下文窗口都会归零(用户反馈的核心问题)。
            ThreadUsageStore.load(threadId)?.let { snap ->
                fresh.turnUsage = snap.turn
                fresh.totalUsage = snap.total
                fresh.liveContextWindow = snap.contextWindow
            }
        }
    }

    /** 全部轮次都结束(没有任何线程在跑)才停保活服务。 */
    private fun maybeStopKeepAlive() {
        if (states.values.none { it.running || it.recovering }) {
            AgentConnection.stopTurnService()
        }
    }

    /**
     * 把累积中的思考文本落成时间线上的一张可折叠卡片。
     *
     * 思考总是发生在正文/工具调用之前,所以在"第一段正文出现""第一个工具调用"
     * "本轮结束"三个时点各尝试提交一次即可;提交后 thinkingText 清空,
     * 重复调用自然变成空操作。这样退出页面再回来也能看到完整的思考过程。
     */
    private fun commitThinking(st: State) {
        val t = st.thinkingText
        if (t.isBlank()) return
        st.items.add(ChatItem("thinking", t.trim()))
        st.thinkingText = ""
    }

    /**
     * 把本轮流式更新的执行计划落成时间线上的一张步骤卡。
     * 与思考同理:计划只发生在正文之前,轮次结束提交一次即可。
     */
    private fun commitPlan(st: State) {
        val steps = st.planSteps
        if (steps.isEmpty()) return
        // 同一轮内已落过卡则原地更新(轮次只会提交一次,防御性写法)
        val idx = st.items.indexOfLast { it.role == "plan" }
        val card = ChatItem("plan", planSteps = steps)
        if (idx >= 0) st.items[idx] = card else st.items.add(card)
        st.planSteps = emptyList()
        st.planStream = ""
    }

    /**
     * 把一条 codex 事件放进时间线;同一次审查/同一轮的 diff 原地更新,不重复插卡。
     *
     * 与工具卡同一套思路(见 TOOL_CALL 分支):codex 对同一件事会发多条通知
     * (审查 started → completed;diff 反复更新),逐条追加会让时间线里
     * 同一件事出现好几张卡 —— 用户会以为发生了好几次。
     */
    private fun upsertCodexEventItem(st: State, event: pocketagent.v1.CodexEvent) {
        val card = codexEventToChatItem(event)
        val key = codexEventTimelineKey(event)
        if (key.isNotEmpty()) {
            val idx = st.items.indexOfLast {
                it.role == "codex_event" && it.codexEvent?.let { e -> codexEventTimelineKey(e) } == key
            }
            if (idx >= 0) {
                st.items[idx] = card
                return
            }
        }
        // 连续重复的同类提示(例如同一句 hooks 警告重复到达)只留一条,
        // 否则时间线会被同一句话挤满。
        val last = st.items.lastOrNull()
        if (last?.role == "codex_event" &&
            last.codexEvent?.kind == event.kind &&
            last.codexEvent?.message == event.message
        ) {
            return
        }
        st.items.add(card)
    }

    /** 用户推翻了某次审查的驳回结论:收起入口并记住状态。 */
    fun markGuardianOverridden(threadId: String, reviewId: String) {
        if (reviewId.isBlank()) return
        val st = state(threadId)
        st.guardianOverridden = st.guardianOverridden + reviewId
    }

    /** 发送一条消息并启动/接续后台轮次。 */
    fun send(threadId: String, text: String, attachments: List<Attachment> = emptyList()) {
        val st = state(threadId)
        if (text.isBlank() || st.running) return
        st.running = true
        st.recovering = false
        st.recoveryError = null
        st.errorMessage = null
        st.codexEvents = emptyList()
        st.approvalError = null
        st.pendingApproval = null
        st.thinkingText = ""
        st.streamText = ""
        // 注意:这里**不重置** turnUsage —— 新一轮上报到达前,上下文窗口继续显示
        // 上一次的用量(否则发完消息面板立刻变成 0%,看起来像数据丢了)。
        st.planSteps = emptyList()
        st.planStream = ""
        // 前台保活:锁屏/切后台不被系统杀,任务流才能活着跑完
        AgentConnection.startTurnService()
        // 【任务中心】登记这一轮消息处理 —— 之后退出聊天页也由 TaskCenter 持有引用,
        // 侧边栏角标与任务中心列表由此得到数据来源(此前无人调用,角标恒 0)。
        TaskCenter.trackMessage(threadId, text.replace('\n', ' ').take(24))
        // 附件已上传到电脑,把 PC 路径一并发给 agent,它会自己去读
        val attachNote = if (attachments.isEmpty()) "" else
            attachments.joinToString("\n") { "[附件已上传到电脑: ${it.pcPath}]" } + "\n\n"
        st.items.add(ChatItem("user", text, attachments = attachments))

        scope.launch {
            try {
                val stub = AgentConnection.threadServiceAsync()
                val requestStream = Channel<TurnClientMessage>(8)
                st.replyCh = requestStream

                val inputBuilder = TurnInput.newBuilder()
                    .setThreadId(threadId)
                    .addBlocks(
                        ContentBlock.newBuilder()
                            .setText(TextBlock.newBuilder().setText(attachNote + text)),
                    )
                // 图片同时以 ImageBlock 发给支持视觉的 provider；路径说明仍保留在文本块中，
                // 这样 echo/codex 等只读文本的 provider 也能找到电脑上的原文件。
                attachments.filter {
                    attachmentKind(it.name, it.mediaType) == AttachmentKind.IMAGE &&
                        it.previewData != null
                }.forEach { attachment ->
                    val data = attachment.previewData ?: return@forEach
                    inputBuilder.addBlocks(
                        ContentBlock.newBuilder()
                            .setImage(
                                ImageBlock.newBuilder()
                                    .setData(com.google.protobuf.ByteString.copyFrom(data))
                                    .setMediaType(
                                        attachment.mediaType.ifBlank { "image/jpeg" },
                                    ),
                            ),
                    )
                }
                // 会话内无缝切换:模型/思考强度随本轮真实发给 daemon→codex
                if (AgentConnection.selectedModel.isNotEmpty()) {
                    inputBuilder.setModelId(AgentConnection.selectedModel)
                }
                if (AgentConnection.selectedEffort.isNotEmpty()) {
                    inputBuilder.setEffort(AgentConnection.selectedEffort)
                }
                val initMsg = TurnClientMessage.newBuilder().setInput(inputBuilder).build()

                val responseFlow = callbackFlow<TurnServerMessage> {
                    val observer = object : io.grpc.stub.StreamObserver<TurnServerMessage> {
                        override fun onNext(value: TurnServerMessage) { trySend(value) }
                        override fun onError(t: Throwable) { close(t) }
                        override fun onCompleted() { close() }
                    }
                    val requestObserver = stub.startTurn(observer)
                    launch {
                        requestObserver.onNext(initMsg)
                        for (msg in requestStream) requestObserver.onNext(msg)
                        requestObserver.onCompleted()
                    }
                    awaitClose { }
                }

                var sawEnd = false
                responseFlow.collect { ev ->
                    when (ev.msgCase) {
                        TurnServerMessage.MsgCase.STARTED -> {
                            st.streaming = true
                            if (ev.started.modelId.isNotEmpty()) st.runningModel = ev.started.modelId
                        }
                        TurnServerMessage.MsgCase.TEXT_DELTA -> {
                            // 正文开始 = 思考阶段结束,先把思考落成一张卡片
                            commitThinking(st)
                            st.streamText += ev.textDelta.delta
                        }
                        TurnServerMessage.MsgCase.THINKING_DELTA -> st.thinkingText += ev.thinkingDelta.delta
                        TurnServerMessage.MsgCase.USAGE -> {
                            // 用量进上下文窗口交互卡，不再挤占聊天时间线。
                            st.turnUsage = ev.usage.usage
                            st.totalUsage = ev.usage.totalUsage
                            // codex 上报的真实窗口大小:模型列表没有各模型的窗口,
                            // 切模型后上下文窗口靠这个继续显示。
                            if (ev.usage.modelContextWindow > 0) {
                                st.liveContextWindow = ev.usage.modelContextWindow
                            }
                            // 落盘缓存:重开该会话时先回显,不再从 0% 开始
                            ThreadUsageStore.save(
                                threadId,
                                ev.usage.usage,
                                ev.usage.totalUsage,
                                st.liveContextWindow,
                            )
                        }
                        TurnServerMessage.MsgCase.CODEX_EVENT -> {
                            st.codexEvents = (st.codexEvents + ev.codexEvent).takeLast(20)
                            val cev = ev.codexEvent
                            when {
                                cev.title == "执行计划更新" && cev.planStepsCount > 0 -> {
                                    // 结构化步骤:实时替换当前计划,轮次结束落成卡片
                                    st.planSteps = cev.planStepsList
                                    st.planStream = ""
                                }
                                cev.title == "执行计划生成中" && cev.message.isNotBlank() -> {
                                    st.planStream += cev.message
                                }
                            }
                            if (cev.kind == CodexEvent.Kind.ERROR) {
                                st.errorMessage = cev.message
                            }
                            // 【本轮修正】以前这些事件只进 st.codexEvents,而那个列表
                            // **从未被渲染**(ChatScreen 里 "codex_event" -> Unit) ——
                            // 于是审查驳回、配置警告、回合 diff、技能变化全部静默消失,
                            // 用户看到的只有"任务莫名失败"。现在按"是否需要用户知情"
                            // 分类入时间线,诊断类仍旧只做状态与错误提示(见
                            // shouldShowCodexEventInTimeline 的长注释)。
                            if (shouldShowCodexEventInTimeline(cev)) {
                                upsertCodexEventItem(st, cev)
                            }
                        }
                        TurnServerMessage.MsgCase.TOOL_CALL -> {
                            val tc = ev.toolCall
                            if (tc.phase == ToolCallUpdate.Phase.STARTED) {
                                if (st.streamText.isNotEmpty()) {
                                    st.items.add(ChatItem("assistant", st.streamText))
                                    st.streamText = ""
                                }
                                commitThinking(st)
                                st.items.add(
                                    ChatItem(
                                        "tool",
                                        toolName = tc.call.name.ifBlank { "工具" },
                                        toolId = tc.call.id,
                                        toolArgs = tc.call.jsonArgs,
                                        toolStatus = "running",
                                    ),
                                )
                            } else {
                                val res = if (tc.hasResult()) tc.result else null
                                // 结果回填到同一次调用的那张卡上(同一 id 原地更新),
                                // 而不是再插一条新记录 —— 这样工具卡才是"一次调用一条"。
                                val idx = st.items.indexOfLast { item ->
                                    item.role == "tool" &&
                                        (
                                            (tc.call.id.isNotEmpty() && item.toolId == tc.call.id) ||
                                                (tc.call.id.isEmpty() && item.toolStatus == "running")
                                            )
                                }
                                val prev = if (idx >= 0) st.items[idx] else null
                                val updated = ChatItem(
                                    "tool",
                                    toolName = tc.call.name.ifBlank { prev?.toolName ?: "工具" },
                                    toolId = tc.call.id.ifBlank { prev?.toolId ?: "" },
                                    toolArgs = tc.call.jsonArgs.ifBlank { prev?.toolArgs ?: "" },
                                    toolElapsedMs = res?.elapsedMs ?: 0.0,
                                    text = (res?.output ?: "").take(4000),
                                    toolStatus = if (res?.ok == false) "failure" else "success",
                                )
                                if (idx >= 0) st.items[idx] = updated else st.items.add(updated)
                            }
                        }
                        TurnServerMessage.MsgCase.APPROVAL_REQUEST -> st.pendingApproval = ev.approvalRequest
                        TurnServerMessage.MsgCase.COMPLETED -> {
                            sawEnd = true
                            commitThinking(st)
                            commitPlan(st)
                            if (st.streamText.isNotEmpty()) {
                                st.items.add(ChatItem("assistant", st.streamText))
                            }
                            st.streamText = ""
                            st.streaming = false
                            st.running = false
                            st.recovering = false
                            if (st.codexEvents.none { it.kind == CodexEvent.Kind.ERROR }) {
                                st.errorMessage = null
                            }
                            st.approvalError = null
                            st.pendingApproval = null
                            st.replyCh = null
                            // 任务中心:轮次正常结束 → 标记完成(有 ERROR 事件仍算失败)
                            TaskCenter.finishMessage(threadId, ok = st.errorMessage == null)
                        }
                        TurnServerMessage.MsgCase.FAILED -> {
                            sawEnd = true
                            st.errorMessage = ev.failed.error.message.ifBlank { "任务失败" }
                            commitThinking(st)
                            commitPlan(st)
                            if (st.streamText.isNotEmpty()) {
                                st.items.add(ChatItem("assistant", st.streamText))
                            }
                            st.streamText = ""
                            st.streaming = false
                            st.running = false
                            st.recovering = false
                            st.pendingApproval = null
                            st.replyCh = null
                            // 任务中心:轮次失败 → 标记失败并带上原因
                            TaskCenter.finishMessage(threadId, ok = false, detail = st.errorMessage ?: "")
                        }
                        else -> {}
                    }
                }
                st.running = false
                if (!sawEnd) {
                    // 流被静默关闭(USB/网络闪断走的是 onCompleted 而非 onError):
                    // 没有结束帧 = 电脑端结果未知,按断流处理并启动自动恢复
                    st.streaming = false
                    st.replyCh = null
                    st.recovering = true
                    st.errorMessage = "连接已断开，正在恢复电脑端任务结果"
                    if (st.streamText.isNotEmpty()) {
                        st.items.add(ChatItem("assistant", st.streamText))
                        st.streamText = ""
                    }
                    recoverOrphanTurn(threadId, st)
                }
            } catch (e: Exception) {
                st.streaming = false
                st.running = false
                st.replyCh = null
                st.recovering = true
                st.errorMessage = "连接已断开，正在恢复电脑端任务结果"
                if (st.streamText.isNotEmpty()) {
                    st.items.add(ChatItem("assistant", st.streamText))
                    st.streamText = ""
                }
                recoverOrphanTurn(threadId, st)
            }
            maybeStopKeepAlive()
        }
    }

    /**
     * 断流恢复:USB/WiFi 闪断会掐掉 gRPC 流,但 daemon 电脑上的轮次还在继续跑。
     * 后台轮询:自动重连(多候选回退)→ 查会话历史,一旦电脑端落库了新回复,
     * 就用真实历史整屏回填,用户无感接上。
     */
    private fun recoverOrphanTurn(threadId: String, st: State) {
        scope.launch {
            var baseline = try {
                AgentConnection.threadService().getThreadHistory(
                    GetThreadHistoryRequest.newBuilder().setThreadId(threadId).build(),
                ).messagesList.size
            } catch (_: Exception) { -1 }
            var recovered = false
            for (i in 0 until 180) { // 最长约 30 分钟(10s × 180),覆盖长任务
                if (recovered) break
                delay(if (i == 0) 800 else 5000)
                val snapshot = try {
                    if (AgentConnection.connectWithFallback(freshChannel = true) == null) continue
                    val history = AgentConnection.threadService().getThreadHistory(
                        GetThreadHistoryRequest.newBuilder().setThreadId(threadId).build(),
                    ).messagesList
                    val thread = AgentConnection.threadService().getThread(
                        GetThreadRequest.newBuilder().setThreadId(threadId).build(),
                    )
                    history to thread.status
                } catch (_: Exception) {
                    continue // 这一轮没连上,下个周期再试
                }
                val history = snapshot.first
                val status = snapshot.second
                if (baseline < 0) {
                    baseline = history.size
                }
                val completedAfterReconnect =
                    status == ThreadStatus.THREAD_STATUS_IDLE &&
                        history.lastOrNull()?.role == "assistant"
                if (history.size > baseline || completedAfterReconnect) {
                    recovered = true
                    st.items.clear()
                    st.items.addAll(historyToChatItems(history))
                    st.recovering = false
                    st.recoveryError = null
                    st.errorMessage = null
                    // 任务中心:断流后从电脑端恢复成功 = 这一轮实际跑完了
                    TaskCenter.finishMessage(threadId, ok = true, detail = "已从电脑端恢复结果")
                    maybeStopKeepAlive()
                }
            }
            if (!recovered) {
                st.recovering = false
                st.recoveryError = "自动恢复超时，请手动刷新会话历史"
                TaskCenter.finishMessage(threadId, ok = false, detail = "自动恢复超时")
                maybeStopKeepAlive()
            }
        }
    }

    fun retryRecovery(threadId: String) {
        val st = state(threadId)
        if (st.recovering) return
        st.recoveryError = null
        st.errorMessage = "正在重新获取电脑端任务结果"
        st.recovering = true
        recoverOrphanTurn(threadId, st)
    }

    /**
     * 终止正在跑的轮次(用户在会话页点"停止")。
     *
     * 【双保险】优先走轮次流里的取消帧(daemon 在流内直接 cancel);
     * 流已断开时退回 CancelTurn 一元 RPC —— daemon 会从 activeTurns
     * 注册表里找到该线程在跑的轮次并取消。本地状态由随后到达的
     * FAILED/COMPLETED 帧收尾;若流本来就断了,取消后走一次历史同步兜底。
     */
    fun cancelTurn(threadId: String) {
        val st = state(threadId)
        if (!st.running) return
        val sent = st.replyCh?.trySend(
            TurnClientMessage.newBuilder()
                .setCancel(CancelSignal.newBuilder().setReason("用户在手机端终止"))
                .build(),
        )
        if (sent?.isSuccess != true) {
            // 流不可用(断线/恢复中):直接打 RPC,daemon 端取消电脑上的执行
            scope.launch {
                runCatching {
                    withContext(Dispatchers.IO) {
                        AgentConnection.threadService().cancelTurn(
                            CancelTurnRequest.newBuilder().setThreadId(threadId).build(),
                        )
                    }
                }.onSuccess {
                    // 电脑端已取消:同步一次历史,把已产出的部分内容拿回来
                    resyncFromDaemon(threadId, silent = true)
                }.onFailure {
                    st.recoveryError = "终止失败:${it.message ?: "无法连接电脑"}"
                }
            }
        }
    }

    /**
     * 手动同步:从 daemon 拉取最新历史并整屏回填。
     *
     * 任务中心「重试」/会话页手动刷新/取消后的兜底都走这里 ——
     * 关键在**找到对应消息并正常输出到手机端**:电脑端只要落库了
     * 新回复,这里就把它带回时间线并结束任务中心里的对应条目。
     */
    fun resyncFromDaemon(threadId: String, silent: Boolean = false) {
        val st = state(threadId)
        if (st.running) return
        if (!silent) {
            st.recovering = true
            st.recoveryError = null
            st.errorMessage = "正在从电脑端同步任务结果"
        }
        scope.launch {
            try {
                val history = withContext(Dispatchers.IO) {
                    AgentConnection.threadService().getThreadHistory(
                        GetThreadHistoryRequest.newBuilder().setThreadId(threadId).build(),
                    ).messagesList
                }
                val current = st.items.count { it.role == "user" || it.role == "assistant" }
                st.items.clear()
                st.items.addAll(historyToChatItems(history))
                st.recovering = false
                st.recoveryError = null
                st.errorMessage = null
                TaskCenter.finishMessage(
                    threadId,
                    ok = true,
                    detail = if (history.size >= current) "已从电脑端同步结果" else "",
                )
            } catch (e: Exception) {
                st.recovering = false
                st.recoveryError = "同步失败:${e.message ?: "无法连接电脑"}"
                TaskCenter.finishMessage(threadId, ok = false, detail = "同步失败")
            }
            maybeStopKeepAlive()
        }
    }

    /**
     * 回复审批请求(允许/拒绝),经后台流的 request 通道发给 daemon。
     *
     * note 是自由文本通道,只有"反问类"请求(request_user_input / MCP elicitation)才用:
     * 里面装的是 `{"<问题id>":{"answers":["..."]}}`,daemon 原样转成 codex 期望的答案映射。
     * 审批类固定传空串 —— 别把两类语义混在一个字段里,否则 agent 会收到一段
     * 和它的问题无关的"理由"。
     */
    fun answerApproval(threadId: String, requestId: String, approved: Boolean, note: String = "") {
        val st = state(threadId)
        val ap = st.pendingApproval ?: return
        if (ap.requestId != requestId) {
            st.approvalError = "审批请求已更新，请重新确认"
            return
        }
        val decision = ApprovalDecision.newBuilder()
            .setRequestId(requestId)
            .setApproved(approved)
        if (note.isNotEmpty()) decision.setNote(note)
        val result = st.replyCh?.trySend(
            TurnClientMessage.newBuilder()
                .setApproval(decision.build())
                .build(),
        )
        if (result?.isSuccess == true) {
            st.pendingApproval = null
            st.approvalError = null
        } else {
            st.approvalError = "审批发送失败，请检查连接后重试"
        }
    }
}
