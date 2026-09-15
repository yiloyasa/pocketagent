package dev.pocketagent.app.ui

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import dev.pocketagent.app.AgentConnection
import dev.pocketagent.app.ui.component.TaskCenter
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
        var historyLoaded by mutableStateOf(false)
        var pendingApproval by mutableStateOf<ApprovalRequest?>(null)
        var approvalError by mutableStateOf<String?>(null)
        var replyCh: Channel<TurnClientMessage>? = null
    }

    private val states = java.util.concurrent.ConcurrentHashMap<String, State>()

    /** app 级作用域:导航离开聊天页也不会取消协程。 */
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Default)

    fun state(threadId: String): State = states.getOrPut(threadId) { State() }

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
        st.turnUsage = null
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
                    if (AgentConnection.connectWithFallback() == null) continue
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

    /** 回复审批请求(允许/拒绝),经后台流的 request 通道发给 daemon。 */
    fun answerApproval(threadId: String, requestId: String, approved: Boolean) {
        val st = state(threadId)
        val ap = st.pendingApproval ?: return
        if (ap.requestId != requestId) {
            st.approvalError = "审批请求已更新，请重新确认"
            return
        }
        val result = st.replyCh?.trySend(
            TurnClientMessage.newBuilder()
                .setApproval(
                    ApprovalDecision.newBuilder()
                        .setRequestId(requestId)
                        .setApproved(approved),
                )
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
