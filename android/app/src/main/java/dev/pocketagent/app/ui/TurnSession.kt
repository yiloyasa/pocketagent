package dev.pocketagent.app.ui

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import dev.pocketagent.app.AgentConnection
import dev.pocketagent.app.ui.component.TaskCenter
import dev.pocketagent.app.ui.component.ThreadUsageStore
import dev.pocketagent.app.ui.component.TurnCheckpointStore
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.channels.awaitClose
import kotlinx.coroutines.flow.buffer
import kotlinx.coroutines.flow.callbackFlow
import kotlinx.coroutines.flow.collect
import kotlinx.coroutines.launch
import kotlinx.coroutines.withTimeoutOrNull
import kotlinx.coroutines.withTimeout
import kotlinx.coroutines.withContext
import pocketagent.v1.OutputState
import pocketagent.v1.SubmitApprovalRequest
import pocketagent.v1.ApprovalDecision
import pocketagent.v1.ApprovalRequest
import pocketagent.v1.CancelSignal
import pocketagent.v1.CancelTurnRequest
import pocketagent.v1.ContentBlock
import pocketagent.v1.CodexEvent
import pocketagent.v1.GetThreadEventsSinceRequest
import pocketagent.v1.GetThreadRequest
import pocketagent.v1.GetThreadHistoryRequest
import pocketagent.v1.HostInfo
import pocketagent.v1.ImageBlock
import pocketagent.v1.TextBlock
import pocketagent.v1.ThreadStatus
import pocketagent.v1.ToolCallUpdate
import pocketagent.v1.TurnClientMessage
import pocketagent.v1.TurnInput
import pocketagent.v1.TurnServerMessage
import pocketagent.v1.HostEvent

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
        /** Codex 正在执行 contextCompaction item。 */
        var compactionInProgress by mutableStateOf(false)
        /**
         * 最近一次"上下文已压缩"的时间(0 = 本进程内未发生过)。
         *
         * 【为什么必须单独记】压缩后 codex 不一定立刻补发新的用量,而压缩前最后一次
         * 上报的占用可能已贴近甚至超出窗口 —— 继续显示它就是"超出实际窗口的假数据"。
         * 事件到达时清掉 turnUsage(占用回到未知),由这个时间去驱动卡片上的
         * "已压缩,等待新一轮用量"状态;新一轮用量上报后自动恢复真实占比。
         */
        var lastCompactedAt by mutableStateOf(0L)
        /** 当前执行计划(turn/plan/updated,"status|text" 列表);轮次结束落成计划卡。 */
        var planSteps by mutableStateOf(listOf<String>())
        /** 计划生成中的流式草稿(item/plan/delta)。 */
        var planStream by mutableStateOf("")
        /** 本轮实际生效的模型(codex 可能按桌面配置重定向)。 */
        var runningModel by mutableStateOf("")
        var runningEffort by mutableStateOf<String?>(null)
        var selectionPending by mutableStateOf(false)
        var running by mutableStateOf(false)
        var streaming by mutableStateOf(false)
        var recovering by mutableStateOf(false)
        var recoveryError by mutableStateOf<String?>(null)
        var errorMessage by mutableStateOf<String?>(null)
        var turnTerminal by mutableStateOf(false)
        var generation = 0L
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
        /** WatchEvents 只作为恢复唤醒信号，不承载正文或终局事实。 */
        val recoverySignal = Channel<Unit>(Channel.CONFLATED)
        /**
         * 已排队等待执行的输入条数(P1-4)。
         * Queued 回执(position>0)时 +1,QueueFlushed 时 -1;为 0 表示没有排队。
         * 终局帧(COMPLETED/FAILED)看到它 >0 时不收尾 —— daemon 马上会发
         * QueueFlushed + Started 继续跑下一条,流与"运行中"状态必须保持。
         */
        var queuedCount by mutableStateOf(0)
        /**
         * 已应用的事件断点(P1-5):本会话收到的最大 TurnServerMessage.sequence。
         *
         * 【语义】它只有在"断点之后的每一帧都经本机按序应用过"时才可信 ——
         * 实时流帧与 GetThreadEventsSince 回放帧都会推进它;一旦走过一次
         * GetThreadHistory 全量回填(内容来源不是逐帧),它必须归零作废,
         * 否则下次恢复会从旧断点重放已回填过的帧,时间线出现重复。
         * 0 = 没有可信断点(旧 daemon / 全量回填后 / 从未连上支持续传的 daemon)。
         */
        var lastSequence by mutableStateOf(0L)
        var draft by mutableStateOf("")
        var lastSnapshotAt = 0L
    }

    private val states = java.util.concurrent.ConcurrentHashMap<String, State>()

    /** app 级作用域:导航离开聊天页也不会取消协程。 */
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main.immediate)
    private val cacheScope = CoroutineScope(SupervisorJob() + Dispatchers.IO.limitedParallelism(1))
    /** WatchEvents 只写恢复信号，不触碰 Compose 状态，因此不依赖 Android Main dispatcher。 */
    private val hostEventScope = CoroutineScope(SupervisorJob() + Dispatchers.IO)

    init {
        hostEventScope.launch {
            AgentConnection.hostEvents.collect { event ->
                onHostEvent(event)
            }
        }
    }

    private fun onHostEvent(event: HostEvent) {
        val threadId = dev.pocketagent.app.HostEventRouter.turnFinishedThreadId(event) ?: return
        val st = states[threadId] ?: return
        if (st.running || st.recovering) {
            // 事件可能先于流的 onError 到达，因此保留一个可合并信号，保证
            // 进入 recovering 后第一轮也能立即醒来。
            st.recoverySignal.trySend(Unit)
        }
    }

    fun acceptRuntimeSelection(threadId: String, model: String, effort: String?, confirmedEvent: Boolean = false) {
        val st = state(threadId)
        if (confirmedEvent) st.selectionPending = false
        if (!st.selectionPending) {
            AgentConnection.cacheNativeSelection(threadId, model.takeIf { it.isNotBlank() }, effort)
        }
        if (model.isNotBlank()) st.runningModel = model
        if (effort != null) st.runningEffort = effort
    }

    private fun checkpoint(threadId: String, st: State, force: Boolean = false) {
        val now=System.currentTimeMillis()
        if (!force && now-st.lastSnapshotAt<1500) return
        st.lastSnapshotAt=now
        val messages=st.items.filter { it.role=="user" || it.role=="assistant" || it.role=="thinking" }
            // 【深优 R2 修复】时间戳随行落快照(此前整列剥掉,恢复后日期分组退化「早前」)
            .takeLast(60).map { dev.pocketagent.app.ui.component.ThreadSnapshot.Msg(it.role,it.text,it.sentAtMs) }.toMutableList()
        val cursor=TurnCheckpointStore.Checkpoint(threadId,messages.toList(),st.streamText,st.thinkingText,
            st.lastSequence,st.running || st.recovering,st.queuedCount,st.runningModel,st.runningEffort,st.errorMessage)
        if(st.streamText.isNotBlank()) messages.add(dev.pocketagent.app.ui.component.ThreadSnapshot.Msg("assistant",st.streamText,System.currentTimeMillis()))
        if(messages.isNotEmpty()) cacheScope.launch {
            dev.pocketagent.app.ui.component.ThreadSnapshot.saveMessages(threadId,messages)
            TurnCheckpointStore.save(cursor)
        }
    }

    /**
     * 会话删除后释放内存态：关掉恢复信号通道并移除 State。
     *
     * 【为什么必须做】State 及其 recoverySignal 常驻 app 进程,而删除走
     * [ThreadDeletionCoordinator.purgeInMemory] 时若不释放,恢复循环还会继续为一个
     * 已删除的会话拉历史/写时间线 —— 内存层"复活";同 id 重建也会命中旧 State。
     * close() 同时是在等信号的恢复循环的立即退出指令。
     */
    fun release(threadId: String) {
        states.remove(threadId)?.recoverySignal?.close()
    }

    fun state(threadId: String): State = states.getOrPut(threadId) {
        State().also { fresh ->
            // 会话用量像缓存一样回显:daemon 的历史接口不带用量,只靠内存的话
            // 每次重开会话上下文窗口都会归零(用户反馈的核心问题)。
            ThreadUsageStore.load(threadId)?.let { snap ->
                fresh.turnUsage = snap.turn
                fresh.totalUsage = snap.total
                fresh.liveContextWindow = snap.contextWindow
                fresh.lastCompactedAt = snap.compactedAt
            }
        }
    }

    /** Restore only saved active turns. Never re-send the original prompt. */
    fun restoreRunningCheckpoints() {
        scope.launch {
            val checkpoints=withContext(Dispatchers.IO) { TurnCheckpointStore.active() }
            for(saved in checkpoints) {
                val st=state(saved.threadId)
                if(st.running || st.recovering || st.items.isNotEmpty()) continue
                st.items.addAll(saved.messages.map {
                    ChatItem(it.role, it.text, sentAtMs = it.savedAtMs)
                })
                st.streamText=saved.stream
                st.thinkingText=saved.thinking
                st.lastSequence=saved.sequence
                st.runningModel=saved.model
                st.runningEffort=saved.effort
                st.queuedCount=saved.queued
                st.streaming=saved.stream.isNotBlank()
                st.errorMessage=null
                retryRecovery(saved.threadId)
            }
        }
    }

    /**
     * 当前"仍在跑"的线程 id 快照(P1-6 通知事件化)。
     *
     * 【为什么是函数而不是 StateFlow】running 的写入点散落在 startTurn 开头、实时流
     * 收尾、异常兜底、applyServerFrame 的终局帧(COMPLETED/FAILED)等多处,做
     * "变更时更新"的 StateFlow 得逐点插桩,漏一处就丢事件;而 ConcurrentHashMap 的
     * 弱一致遍历本身就是无锁快照,调用方按需拉一次即可 —— 改动最小。范围口径与
     * [maybeStopKeepAlive] 一致(running || recovering):断流恢复中的线程也留在
     * 通知监控范围里,恢复路径上的审批卡同样值得发通知。
     */
    fun runningThreadIds(): Set<String> {
        val out = HashSet<String>()
        for ((tid, st) in states) if (st.running || st.recovering) out.add(tid)
        return out
    }

    /**
     * 全部会话排队输入总数的只读快照(断连全局横幅的数据源,审计 P1-2)。
     *
     * 【为什么必须是全局】断连横幅挂在三屏(Chat/Home/Threads)共用,而后两者
     * 没有单会话的 State 引用;断连时用户最怕"我发的消息丢了",排队数必须跨屏
     * 一直可见。与 [runningThreadIds] 同一口径:ConcurrentHashMap 弱一致遍历、
     * 按需拉快照、不改任何状态 —— **只读,不触碰排队计数的增减逻辑**。
     * ([st.queuedCount] 是 Compose 状态,在组合期读取即自动订阅其变化。)
     */
    fun totalQueuedInputs(): Int = states.values.sumOf { it.queuedCount }

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
     * 把终局帧里的正文块提取成一条助手消息。
     *
     * Codex 有两种合法输出路径:
     * 1. item/agentMessage/delta 逐字推送正文;
     * 2. 只有 item/completed 或 turn/completed 的最终正文。
     *
     * 之前客户端只处理第 1 种:daemon 已经把第 2 种放进
     * TurnCompleted.final_blocks / TurnFailed.partial_text,但这里没有读取,
     * 所以手机端会显示用户消息和“任务完成”,却没有助手文本。这里仅在
     * streamText 为空时使用终局正文,避免把完整正文和已流式正文重复拼接。
     */
    private fun finalText(blocks: Iterable<ContentBlock>): String {
        return buildString {
            blocks.forEach { block ->
                if (block.hasText() && block.text.text.isNotEmpty()) {
                    append(block.text.text)
                }
            }
        }
    }

    private fun commitFinalText(st: State, text: String) {
        if (text.isNotBlank()) {
            st.items.add(ChatItem("assistant", text))
        }
    }

    /**
     * 按 daemon 上报的 [state] 给"没有正文"的轮次补一句人话。
     *
     * 【为什么这句话在本地拼,而不是让 daemon 发】它是**呈现**决定(说什么、什么语气),
     * 不是事实。旧实现让 daemon 往 final_blocks 里塞占位文本 —— 那是把呈现混进了数据:
     * 那条文本会落库、被离线快照读到,最终变成一个"真的存在的助手回复"。事实(状态)
     * 由 daemon 报,措辞由客户端定,这条边界不能倒过来。
     *
     * 【为什么用 WARNING 而不是 STATUS】STATUS 在时间线渲染时被过滤掉(见审批处理的
     * 同类用法),用了等于没提示 —— 用户还是看到一片空白,问题原封不动。
     *
     * 文案与"哪一档要提示"由 [emptyOutputNotice] 决定(纯映射,可单测)。
     */
    private fun noticeEmptyOutput(st: State, state: OutputState, source: String) {
        val notice = emptyOutputNotice(state) ?: return
        st.items.add(
            codexEventToChatItem(
                CodexEvent.newBuilder()
                    .setKind(CodexEvent.Kind.WARNING)
                    .setCode("output_state_notice")
                    .setTitle(notice.first)
                    .setMessage(notice.second)
                    .build()
            )
        )
        if (source.isNotEmpty()) {
            // 来源只用于排障,不摊给用户看;留一条日志让"这轮为什么这样"可追。
            android.util.Log.i("TurnSession", "empty output notice state=$state source=$source")
        }
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

    /** 用纯函数算出的新列表整体替换时间线内容(同一引用 = 没变化,免一次重组)。 */
    private fun swapItems(st: State, replaced: List<ChatItem>) {
        if (replaced !== st.items) {
            st.items.clear()
            st.items.addAll(replaced)
        }
    }

    /**
     * 发送一条消息(P1-4:运行中不再拒绝)。
     *
     * 空闲 → 开启新轮次([startTurn]);running → 经**同一条开启的 StartTurn 流**
     * 继续发([sendWhileRunning]),daemon 侧 codex 插话(turn/steer)、
     * 其余排队,回执见 Steered/Queued/QueueFlushed 帧处理。
     *
     * @return false = 未接收(空文本,或 running 但流已断、正在恢复电脑端结果)。
     *         调用方不应清空输入框,恢复完成后再发。
     */
    fun send(threadId: String, text: String, attachments: List<Attachment> = emptyList()): Boolean {
        val st = state(threadId)
        if (text.isBlank() || st.recovering) return false
        if (st.running) return sendWhileRunning(threadId, st, text, attachments)
        startTurn(threadId, st, text, attachments)
        return true
    }

    /**
     * 组装 TurnInput:文本 + 附件说明 + 图片块 + 会话内模型/强度覆盖。
     * 附件在**选定时**就已上传到电脑(pcPath 可达),所以两条路径的报文形状一致;
     * 抽出来是为了"首轮"与"运行中追加"绝不会有第二种输入形状。
     */
    private fun buildTurnInput(threadId: String, text: String, attachments: List<Attachment>): TurnInput {
        val attachNote = if (attachments.isEmpty()) "" else
            attachments.joinToString("\n") { "[附件已上传到电脑: ${it.pcPath}]" } + "\n\n"
        val builder = TurnInput.newBuilder()
            .setThreadId(threadId)
            .addBlocks(
                ContentBlock.newBuilder()
                    .setText(TextBlock.newBuilder().setText(attachNote + text)),
            )
        // 图片同时以 ImageBlock 发给支持视觉的 provider;路径说明仍保留在文本块中。
        // 注意:codex 的 turn/steer 桥接只取文本(daemon 侧约定),图片块只在
        // "排队续跑"路径生效 —— codex 运行中插话带图会退化成文字说明,可接受。
        attachments.filter {
            attachmentKind(it.name, it.mediaType) == AttachmentKind.IMAGE && it.previewData != null
        }.forEach { attachment ->
            val data = attachment.previewData ?: return@forEach
            builder.addBlocks(
                ContentBlock.newBuilder()
                    .setImage(
                        ImageBlock.newBuilder()
                            .setData(com.google.protobuf.ByteString.copyFrom(data))
                            .setMediaType(attachment.mediaType.ifBlank { "image/jpeg" }),
                    ),
            )
        }
        // 会话内无缝切换:模型/思考强度随本轮真实发给 daemon(排队执行时同样生效)
        if (AgentConnection.threadModel(threadId).orEmpty().isNotEmpty()) {
            builder.setModelId(AgentConnection.threadModel(threadId).orEmpty())
        }
        if (AgentConnection.threadEffort(threadId).orEmpty().isNotEmpty()) {
            builder.setEffort(AgentConnection.threadEffort(threadId).orEmpty())
        }
        return builder.build()
    }

    /**
     * 运行中继续发:直接把 TurnInput 写进**还开着**的 StartTurn 请求流
     * (双向流本来就能继续发;daemon 的读手负责插话/排队)。
     * 流意外已断(running 但 replyCh 没了 = 断流恢复中)→ 不发,交给现有
     * 恢复逻辑,恢复完成后再发。
     */
    private fun sendWhileRunning(
        threadId: String,
        st: State,
        text: String,
        attachments: List<Attachment>,
    ): Boolean {
        val ch = st.replyCh ?: return false
        val input = buildTurnInput(threadId, text, attachments)
        val sent = ch.trySend(TurnClientMessage.newBuilder().setInput(input).build())
        if (!sent.isSuccess) return false
        // 时间线先挂「发送中…」小标签;daemon 的 Steered/Queued 回执决定最终状态
        st.items.add(ChatItem("user", text, attachments = attachments, deliverState = DELIVERY_PENDING, sentAtMs = System.currentTimeMillis()))
        // 任务中心:同一会话自动合并累计(rounds+1),与首轮同一条目
        TaskCenter.trackMessage(threadId, text.replace('\n', ' ').take(24))
        return true
    }

    private fun startTurn(threadId: String, st: State, text: String, attachments: List<Attachment>) {
        val generation=++st.generation
        st.running = true
        st.turnTerminal = false
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
        st.items.add(ChatItem("user", text, attachments = attachments, sentAtMs = System.currentTimeMillis()))

        val turnInput = buildTurnInput(threadId, text, attachments)
        scope.launch {
            try {
                val stub = AgentConnection.threadServiceAsync()
                val requestStream = Channel<TurnClientMessage>(8)
                st.replyCh = requestStream

                val initMsg = TurnClientMessage.newBuilder()
                    .setInput(turnInput)
                    .build()

                val responseFlow = callbackFlow<TurnServerMessage> {
                    val observer = object : io.grpc.stub.StreamObserver<TurnServerMessage> {
                        override fun onNext(value: TurnServerMessage) {
                            if (trySend(value).isFailure) close(IllegalStateException("结果缓冲已满，转为事件回放"))
                        }
                        override fun onError(t: Throwable) { requestStream.close(); close(t) }
                        override fun onCompleted() { requestStream.close(); close() }
                    }
                    val requestObserver = stub.startTurn(observer)
                    launch {
                        requestObserver.onNext(initMsg)
                        for (msg in requestStream) requestObserver.onNext(msg)
                        requestObserver.onCompleted()
                    }
                    awaitClose { requestStream.close() }
                    // 【绝不丢帧】callbackFlow 默认缓冲只有 64:onNext 风暴(delta 密集)
                    // 下游稍慢,trySend 就静默失败 —— 表现是"消息缺字/断层,重进又好了"。
                    // UNLIMITED 后帧只排队不丢弃;真正背压由 gRPC 流控兜底。
                }.buffer(512)

                var sawEnd = false
                // 终局帧是否已完成本地收尾(P1-4:有排队时终局帧**不**收尾,
                // 等队列跑完最后一轮或流关闭再统一结算)
                var finished = false
                responseFlow.collect { ev ->
                    if (st.generation != generation) return@collect
                    // P1-5:帧处理抽到 applyServerFrame —— 实时流与断线增量回放
                    // (GetThreadEventsSince)共用同一份状态机,保证"回放的帧"
                    // 与"实时收到的帧"在本地长出一模一样的时间线。
                    // 返回 true = 终局帧(COMPLETED/FAILED);排队未空时不收尾,
                    // daemon 紧接着会发 QueueFlushed + Started 继续跑下一条。
                    if (ev.msgCase == TurnServerMessage.MsgCase.STARTED && !ev.started.configurationConfirmed) { sawEnd = false; finished = false }
                    if (applyServerFrame(threadId, st, ev, replay = false)) {
                        sawEnd = true
                        if (st.queuedCount == 0) finished = true
                    }
                }
                if (st.generation != generation) return@launch
                val hadQueued = st.queuedCount > 0
                st.running = false
                st.streaming = false
                st.replyCh = null
                if (!sawEnd || !finished || hadQueued) {
                    // 流被静默关闭(USB/网络闪断走的是 onCompleted 而非 onError):
                    // 没有结束帧 = 电脑端结果未知,按断流处理并启动自动恢复
                    st.recovering = true
                    st.errorMessage = "结果流已结束，正在核对电脑端任务状态"
                    st.streaming = st.streamText.isNotBlank()
                    checkpoint(threadId,st,force=true)
                    recoverOrphanTurn(threadId, st)
                } else if (!finished) {
                    // 队列链路被中途打断(用户取消丢队列后 daemon 关流 / daemon 崩溃):
                    // 统一结算 —— 清掉全部投递标签,不让「排队中」变成点不动的幽灵标签。
                    clearAllDeliveryTags(st.items)?.let { swapItems(st, it) }
                    st.pendingApproval = null
                    TaskCenter.finishMessage(threadId, ok = st.errorMessage == null)
                }
            } catch (e: Exception) {
                if (st.generation != generation) return@launch
                val hadQueued = st.queuedCount > 0
                st.streaming = false
                st.running = false
                st.replyCh = null
                if (st.turnTerminal && !hadQueued) {
                    st.recovering = false
                    maybeStopKeepAlive()
                    return@launch
                }
                st.recovering = true
                st.errorMessage = if (AgentConnection.connState.value == "down") "连接暂不可用，正在等待恢复结果" else "结果流中断，正在核对电脑端任务状态"
                st.streaming = st.streamText.isNotBlank()
                checkpoint(threadId,st,force=true)
                recoverOrphanTurn(threadId, st)
            }
            maybeStopKeepAlive()
        }
    }

    /**
     * 应用一条 daemon 下发的 TurnServerMessage 到会话状态。
     *
     * 【P1-5 为什么抽出来】断线恢复要从"整屏历史回填"升级为"按 seq 增量回放":
     * 回放(GetThreadEventsSince)与实时流必须走同一份状态机,回放出的时间线
     * 才和"没断过线"一模一样(工具卡/思考/审批/排队回执全部按原序重建)。
     *
     * @param replay true = 增量回放路径:不碰实时流的生命周期锚点(replyCh、
     *   任务中心收尾),它们归实时流的收尾逻辑管;其余状态(时间线、用量、
     *   审批卡、排队回执)与实时路径完全一致。
     * @return true = 终局帧(COMPLETED/FAILED)。调用方据此结束收集/恢复。
     */
    internal fun applyServerFrame(threadId: String, st: State, ev: TurnServerMessage, replay: Boolean): Boolean {
        // P1-5:每帧到达(实时或回放)都推进事件断点。sequence=0(旧 daemon /
        // 未入事件库的帧)不推进、也不作废现有断点。
        if (ev.sequence > 0 && ev.sequence <= st.lastSequence) return false
        if (ev.sequence > 0 && ev.sequence > st.lastSequence) {
            st.lastSequence = ev.sequence
        }
        var sawEnd = false
        when (ev.msgCase) {
            TurnServerMessage.MsgCase.STARTED -> {
                if (!ev.started.configurationConfirmed) {
                    st.turnTerminal = false
                    st.streaming = true
                }
                if (ev.started.modelId.isNotEmpty()) st.runningModel = ev.started.modelId
                if (ev.started.hasEffort()) st.runningEffort = ev.started.effort
                if (ev.started.configurationConfirmed && !replay) {
                    acceptRuntimeSelection(threadId, ev.started.modelId,
                        if (ev.started.hasEffort()) ev.started.effort else null, confirmedEvent=true)
                }
            }
            TurnServerMessage.MsgCase.TEXT_DELTA -> {
                st.streaming = true
                // 正文开始 = 思考阶段结束,先把思考落成一张卡片
                commitThinking(st)
                st.streamText += ev.textDelta.delta
            }
            TurnServerMessage.MsgCase.THINKING_DELTA -> st.thinkingText += ev.thinkingDelta.delta
            TurnServerMessage.MsgCase.USAGE -> {
                // 用量进上下文窗口交互卡，不再挤占聊天时间线。
                st.compactionInProgress = false
                st.turnUsage = ev.usage.usage
                // 【N3·hermes 修复】total_usage 未设置时按 null 处理(见
                // [projectedTotalUsage]):protobuf 消息字段未设置的 getter 永远返回
                // 默认实例而非 null,直接赋值等于把"daemon 没报累计"渲染成 0/0/0。
                val total = projectedTotalUsage(ev.usage)
                st.totalUsage = total
                // codex 上报的真实窗口大小:模型列表没有各模型的窗口,
                // 切模型后上下文窗口靠这个继续显示。
                if (ev.usage.modelContextWindow > 0) {
                    st.liveContextWindow = ev.usage.modelContextWindow
                }
                // 落盘缓存:重开该会话时先回显,不再从 0% 开始。
                // total 未设置传 null:ThreadUsageStore 的合并语义会保留磁盘上
                // 上一次已知的 total(部分帧缺字段不抹历史),而 hermes 这类
                // 从不设置该字段的会话,磁盘上也从不会有 total 键。
                ThreadUsageStore.save(
                    threadId,
                    ev.usage.usage,
                    total,
                    st.liveContextWindow,
                )
            }
            TurnServerMessage.MsgCase.CODEX_EVENT -> {
                st.codexEvents = (st.codexEvents + ev.codexEvent).takeLast(20)
                val cev = ev.codexEvent
                if (cev.kind == CodexEvent.Kind.CONTEXT_COMPACTED) {
                    if (cev.code == "started") {
                        st.compactionInProgress = true
                        st.turnUsage = null
                    } else {
                        // 【压缩同步】压缩前的占用上报已失真(可能超出实际窗口),
                        // 立即作废:占用环回到"未知",卡片显示压缩状态;
                        // 同时落盘,重开会话也不会把陈旧占用再回显出来。
                        // totalUsage 是全程累计,不受压缩影响,保留。
                        st.compactionInProgress = false
                        st.lastCompactedAt = System.currentTimeMillis()
                        ThreadUsageStore.markCompacted(threadId)
                    }
                    st.turnUsage = null
                }
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
                if (cev.kind == CodexEvent.Kind.ERROR && !cev.willRetry) {
                    st.errorMessage = cev.message
                }
                // 按用户消息事件、用户须知情的事件入时间线,诊断类只做状态提示
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
            // ---- P1-4:运行中发消息的回执帧 ----
            TurnServerMessage.MsgCase.STEERED -> {
                // 该输入已插入运行中的轮次(codex turn/steer)
                swapItems(st, applyDeliveryAckTo(st.items, DELIVERY_STEERED, 0))
            }
            TurnServerMessage.MsgCase.QUEUED -> {
                val q = ev.queued
                if (queuedAckAccepted(q.position)) {
                    st.queuedCount += 1
                    swapItems(st, applyDeliveryAckTo(st.items, DELIVERY_QUEUED, q.position))
                } else {
                    // 队列满被拒收:这条消息 daemon 没接收,必须让用户看见
                    swapItems(st, applyDeliveryAckTo(st.items, DELIVERY_OVERFLOW, 0))
                }
            }
            TurnServerMessage.MsgCase.QUEUE_FLUSHED -> {
                // 排队的输入开始执行:所有「排队中」变回普通用户消息,
                // 紧跟的 Started 帧继续流式渲染(保持 running 不收尾)
                st.queuedCount = (st.queuedCount - 1).coerceAtLeast(0)
                clearDeliveryTags(st.items, queuedOnly = true)?.let { swapItems(st, it) }
            }
            TurnServerMessage.MsgCase.APPROVAL_RESOLVED -> {
                // 【幽灵审批卡治理】该审批已在电脑端被处理(daemon 竞速
                // serverRequest/resolved 后转发):立即撤卡,不再让用户对着
                // 一张点不动也没意义的卡。附一条时间线说明卡为何消失。
                val resolved = ev.approvalResolved
                val cur = st.pendingApproval
                if (cur == null || resolved.requestId.isBlank() ||
                    resolved.requestId == cur.requestId
                ) {
                    st.pendingApproval = null
                    st.approvalError = null
                    val note = resolved.note.ifBlank { "这条审批已在电脑端被处理" }
                    upsertCodexEventItem(
                        st,
                        CodexEvent.newBuilder()
                            // WARNING 才会进时间线(STATUS 被过滤);这里是"用户须知
                            // 情"的解释面,黄色提示语义合适。
                            .setKind(CodexEvent.Kind.WARNING)
                            .setTitle("审批已在电脑端处理")
                            .setMessage(note)
                            .build(),
                    )
                }
            }
            TurnServerMessage.MsgCase.COMPLETED -> {
                sawEnd = true
                st.turnTerminal = true
                commitThinking(st)
                commitPlan(st)
                if (st.streamText.isNotEmpty()) {
                    st.items.add(ChatItem("assistant", st.streamText))
                } else {
                    // 终局兜底:Codex 没发 delta 时,正文在 completed.final_blocks。
                    commitFinalText(st, finalText(ev.completed.finalBlocksList))
                }
                // 【空正文要给用户一个说法】daemon 现在如实上报 output_state;不解释的话,
                // 用户看到的就是"任务完成了,但什么都没有" —— 那正是图二里最让人困惑的
                // 地方。旧实现在这里塞一句 "(完成,无文本输出)" 假占位,是把歧义藏起来,
                // 而且那条文本会落库、被离线快照读到,变成一个"真的存在的助手回复"。
                noticeEmptyOutput(st, ev.completed.outputState, ev.completed.outputSource)
                st.streamText = ""
                st.streaming = false
                // 「已插入/发送中」的输入属于刚结束的这一轮:撤标签变回普通消息;
                // 「排队中/未送达」保留 —— 排队的还没轮到,未送达必须继续可见。
                clearDeliveryTags(st.items, queuedOnly = false)?.let { swapItems(st, it) }
                // The terminal result is authoritative. A recovered upstream
                // retry must not turn a successfully completed task into failure.
                st.errorMessage = null
                st.approvalError = null
                st.pendingApproval = null
                if (st.queuedCount > 0) {
                    // 还有排队的输入:daemon 紧接着发 QueueFlushed + Started
                    // 继续跑下一条。这里**不收尾**(保持 running / replyCh),
                    // 中途收尾会让输入栏闪回"可发送"、还丢掉流的引用。
                } else {
                    st.running = false
                    st.recovering = false
                    // P1-5:回放路径不碰实时流的生命周期锚点(replyCh / 任务中心)
                    if (!replay) {
                        st.replyCh = null
                        // 任务中心:轮次正常结束 → 标记完成(有 ERROR 事件仍算失败)
                        TaskCenter.finishMessage(threadId, ok = st.errorMessage == null)
                    }
                }
            }
            TurnServerMessage.MsgCase.FAILED -> {
                sawEnd = true
                st.turnTerminal = true
                st.errorMessage = ev.failed.error.message.ifBlank { "任务失败" }
                commitThinking(st)
                commitPlan(st)
                if (st.streamText.isNotEmpty()) {
                    st.items.add(ChatItem("assistant", st.streamText))
                } else {
                    // 失败帧同样可能只带 partial_text,不能让已产出的正文消失。
                    commitFinalText(st, ev.failed.partialText)
                }
                st.streamText = ""
                st.streaming = false
                clearDeliveryTags(st.items, queuedOnly = false)?.let { swapItems(st, it) }
                if (st.queuedCount > 0) {
                    // 本轮失败但队列还在:daemon 会继续执行下一条排队输入
                    // (一条失败不否决用户排队的其余消息)。保持 running /
                    // replyCh;错误已由 errorMessage 提示条展示。
                } else {
                    st.running = false
                    st.recovering = false
                    st.pendingApproval = null
                    if (!replay) {
                        st.replyCh = null
                        // 任务中心:轮次失败 → 标记失败并带上原因
                        TaskCenter.finishMessage(threadId, ok = false, detail = st.errorMessage ?: "")
                    }
                }
            }
            else -> {}
        }
        checkpoint(threadId,st,force=sawEnd)
        return sawEnd
    }

    // ================= P1-5 事件断点续传(增量恢复) =================

    /** daemon 能力协商:HostInfo.capabilities 是否声明了事件续传能力。 */
    internal fun daemonSupportsEventSequence(capabilities: List<String>): Boolean =
        capabilities.any { it.trim().equals(EVENT_SEQUENCE_CAP, ignoreCase = true) }

    /** capabilities 里的续传能力声明字面量(与 daemon GetHostInfo 对齐)。 */
    private const val EVENT_SEQUENCE_CAP = "event_sequence"

    /** 恢复方式判定(纯函数,JVM 单测锁定降级链的"选路"端)。 */
    internal enum class ResumePlan {
        /** 优先增量:有可信断点且 daemon 声明 event_sequence。 */
        INCREMENTAL,
        /** 直接全量:无断点(lastSequence<=0)或 daemon 不支持。 */
        FULL_ONLY,
    }

    internal fun resumePlan(lastSequence: Long, capabilities: List<String>?): ResumePlan = when {
        lastSequence <= 0 -> ResumePlan.FULL_ONLY // 没有可信断点,增量无从谈起
        capabilities == null -> ResumePlan.FULL_ONLY // 拿不到能力声明 → 按旧 daemon 对待
        daemonSupportsEventSequence(capabilities) -> ResumePlan.INCREMENTAL
        else -> ResumePlan.FULL_ONLY
    }

    /**
     * 恢复轮询的总尝试次数。3 秒封顶 × 300 次 ≈ 15 分钟,与旧实现(5 秒 × 180 次
     * ≈ 15 分钟)的最长覆盖一致 —— 把节奏改快不该顺手缩短覆盖时长。
     */
    private const val RECOVERY_ATTEMPTS = 300

    /**
     * 恢复轮询第 [attempt] 次之后该等多久(毫秒)。
     *
     * 【为什么退避而不是固定 5 秒】断流恢复此前是"首等 800ms、其后固定 5 秒"的轮询。
     * 网络回来的那一刻平均要等 2.5 秒、最坏 5 秒才开始补帧 —— 用户体感就是
     * "网回来了,手机还不动"。而断流绝大多数是**瞬时抖动**:先快试几次,大部分情况
     * 几百毫秒内就把缺口补上了;真跑很久的任务再拉长到 3 秒封顶,不必一直高频打扰。
     *
     * 【为什么封顶 3 秒而不是 5 秒】同样的最长覆盖时长下,封顶越低补帧越及时;
     * 3 秒是"够省"与"够快"之间的折中(旧实现 5 秒 × 180 次 ≈ 15 分钟,新实现
     * 3 秒 × 300 次 ≈ 15 分钟,覆盖不变)。
     */
    internal fun recoveryBackoffMs(attempt: Int): Long = when {
        attempt <= 0 -> 150L
        attempt == 1 -> 300L
        attempt == 2 -> 600L
        attempt == 3 -> 1_200L
        attempt == 4 -> 2_000L
        else -> 3_000L
    }

    /**
     * 这一周期要不要退回**全量**历史回填。
     *
     * 【为什么不能默认全量】原实现每个周期都跑一次 GetThreadHistory(整屏重拉),
     * 而方案 §9.3 要求"只有事件丢窗时才调用全量历史"。增量路径**本身就能检测终局**
     * ([ReplayOutcome.TERMINAL] = 补到了终局帧),所以全量只在两种情况下才必要:
     *   - 收到 ResyncRequired:事件已轮转丢弃,断点作废,增量无从谈起;
     *   - 本来就没有断点([hasResumePoint] = false):增量没有起点。
     *
     * 【为什么"连不上"不算】连不上是**暂时**状态(网络刚回来/daemon 正忙),下一周期
     * 接着增量即可。此刻整屏重拉既浪费流量,又会让时间线整体重画一遍。
     */
    internal fun needFullHistoryFallback(outcome: ReplayOutcome?, hasResumePoint: Boolean): Boolean = when {
        outcome == ReplayOutcome.RESYNC -> true
        outcome == null && !hasResumePoint -> true
        else -> false
    }

    /** 一段增量回放流的结果。 */
    internal enum class ReplayOutcome {
        /** daemon 要求全量回退(after 已超保留窗口):断点已作废。 */
        RESYNC,
        /** 回放流正常关,且补回了终局帧(恢复完成)。 */
        TERMINAL,
        /** 回放流正常关,但轮次还没终局(电脑端仍在跑)。 */
        CAUGHT_UP,
    }

    /**
     * 消费一段回放帧:按序喂给与实时流同一份 [applyServerFrame]。
     * 收到 ResyncRequired 立即作断点并返回 [ReplayOutcome.RESYNC],
     * 调用方必须降级全量回填 —— 宁可重拉也不在缺口上拼出错帧。
     */
    internal fun consumeReplayFrames(threadId: String, st: State, frames: Iterable<TurnServerMessage>): ReplayOutcome {
        var sawEnd = false
        for (frame in frames) {
            if (frame.msgCase == TurnServerMessage.MsgCase.RESYNC_REQUIRED) {
                // 全量回填会让时间线整体替换:之前的断点对回填后的本地视图没有意义
                st.lastSequence = 0
                return ReplayOutcome.RESYNC
            }
            if (frame.msgCase == TurnServerMessage.MsgCase.STARTED && !frame.started.configurationConfirmed) sawEnd = false
            if (applyServerFrame(threadId, st, frame, replay = true)) sawEnd = true
        }
        return if (sawEnd && st.queuedCount == 0) ReplayOutcome.TERMINAL else ReplayOutcome.CAUGHT_UP
    }

    /**
     * 一个恢复周期里的增量续传尝试。
     *
     * @return TERMINAL/CAUGHT_UP = 回放流正常关(缺口已补);
     *         RESYNC = daemon 要求全量回退(断点已作废);
     *         null = 本周期不可用(无断点/连不上/daemon 不支持/RPC 失败)。
     *         返回 null 时调用方走原有全量回填路径 —— 降级链"增量→全量"不许丢。
     */
    private suspend fun tryIncrementalRecovery(threadId: String, st: State): ReplayOutcome? {
        if (st.lastSequence <= 0) return null
        // 能力协商:HostInfo 从既有连接流程取(connectWithFallback 的返回值,
        // 只读 AgentConnection 公开 API,不引入新的连接状态)。
        val info: HostInfo? = try {
            withContext(Dispatchers.IO) { AgentConnection.connectWithFallback() }
        } catch (_: Exception) {
            null
        }
        if (info == null) return null // 连不上:全量路径会自己再试连接
        if (resumePlan(st.lastSequence, info.capabilitiesList) == ResumePlan.FULL_ONLY) {
            // daemon 不支持事件续传 = 它根本没有 seq 体系,本机断点作废
            st.lastSequence = 0
            return null
        }
        return try {
            val frames = withContext(Dispatchers.IO) {
                AgentConnection.threadService().getThreadEventsSince(
                    GetThreadEventsSinceRequest.newBuilder().setThreadId(threadId)
                        .setAfterSequence(st.lastSequence).build())
            }
            var outcome=ReplayOutcome.CAUGHT_UP
            while (true) {
                val batch=withContext(Dispatchers.IO) {
                    buildList {
                        while(size<128 && frames.hasNext()) add(frames.next())
                    }
                }
                if(batch.isEmpty()) break
                outcome=consumeReplayFrames(threadId,st,batch)
                if(outcome==ReplayOutcome.RESYNC) break
                kotlinx.coroutines.yield()
            }
            outcome
        } catch (_: Exception) {
            // 流中途断:已应用的帧已经推进了断点,下个周期可从新断点继续增量;
            // 本周期返回 null 让全量路径兜底(两条路都以"不丢内容"为先)。
            null
        }
    }

    /** 恢复成功的统一收尾(增量/全量共用):撤横幅、结任务中心、停保活。 */
    private fun finishRecovery(threadId: String, st: State, ok: Boolean) {
        st.recovering = false
        st.recoveryError = null
        if (ok) st.errorMessage = null
        st.running = false
        st.streaming = false
        st.queuedCount = 0
        st.pendingApproval = null
        checkpoint(threadId,st,force=true)
        // 任务中心:断流后从电脑端恢复成功 = 这一轮实际跑完了
        TaskCenter.finishMessage(threadId, ok = ok, detail = "已从电脑端恢复结果")
        maybeStopKeepAlive()
    }

    /**
     * 断流恢复:USB/WiFi 闪断会掐掉 gRPC 流,但 daemon 电脑上的轮次还在继续跑。
     *
     * 【P1-5 升级:恢复降级链】每个恢复周期按优先级走:
     *   1. 事件增量续传(GetThreadEventsSince(thread, lastSequence)):
     *      daemon 支持 event_sequence 且本机有可信断点时,只补错过的帧 ——
     *      与实时帧同一状态机回放,时间线无缝接上,不整屏重画;
     *   2. 历史全量回填(GetThreadHistory 整屏回填,原有逻辑原样保留):
     *      无断点 / daemon 不支持 / 收到 ResyncRequired(轮转丢窗) /
     *      增量 RPC 失败 —— 任一情况都退回这条路,保证永不丢内容。
     * 后台轮询至多 30 分钟(10s × 180),覆盖长任务。
     */
    private fun recoverOrphanTurn(threadId: String, st: State) {
        val generation = st.generation
        scope.launch {
            var baseline = try {
                withContext(Dispatchers.IO) {
                    AgentConnection.threadService().getThreadHistory(
                        GetThreadHistoryRequest.newBuilder().setThreadId(threadId).build()).messagesList.size
                }
            } catch (_: Exception) { -1 }
            var recovered = false
            for (i in 0 until RECOVERY_ATTEMPTS) {
                if (recovered) break
                if (st.generation != generation) return@launch
                // WatchEvents 到达时立即唤醒；没有通知时仍按原退避节奏轮询。
                // receiveCatching 而非 receive:release() 会 close 通道,让在等的循环
                // 立即醒来 —— close 会使 receive 抛异常,而这里要的是"退出恢复"不是崩溃;
                // closed 结果从超时包装里带出来,即为"会话已删除,别再为它补事件"。
                val wake = withTimeoutOrNull(recoveryBackoffMs(i)) {
                    st.recoverySignal.receiveCatching()
                }
                if (wake != null && wake.isClosed) return@launch
                if (st.generation != generation) return@launch
                // ---- 优先:事件增量续传(P1-5) ----
                val outcome = tryIncrementalRecovery(threadId, st)
                if (outcome == ReplayOutcome.TERMINAL) {
                    // 补到终局帧:时间线已与电脑端一致,无缝恢复完成。
                    // ok 以 FAILED 帧设置的 errorMessage 为准(FAILED 分支已落)。
                    finishRecovery(threadId, st, ok = st.errorMessage == null)
                    recovered = true
                    break
                }
                // ---- 兜底:全量回填——**只在丢窗或没有断点时才走** ----
                // 原实现每个周期都整屏重拉,而增量已经把缺口补上(CAUGHT_UP)时那一趟纯属
                // 白跑,还会让时间线整体重画一遍。方案 §9.3 明确要求"只有事件丢窗时才调用
                // 全量历史";增量路径本身已能检测终局(TERMINAL),所以这里判断足够。
                if (!needFullHistoryFallback(outcome, hasResumePoint = st.lastSequence > 0)) continue
                val snapshot = try {
                    withContext(Dispatchers.IO) {
                        if (AgentConnection.connectWithFallback() == null) null else {
                            val history=AgentConnection.threadService().getThreadHistory(
                                GetThreadHistoryRequest.newBuilder().setThreadId(threadId).build()).messagesList
                            val thread=AgentConnection.threadService().getThread(
                                GetThreadRequest.newBuilder().setThreadId(threadId).build())
                            history to thread
                        }
                    }
                } catch (_: Exception) { null } ?: continue
                if (st.generation != generation) return@launch
                val history = snapshot.first
                val status = snapshot.second.status
                st.pendingApproval=snapshot.second.pendingApprovalsList.firstOrNull()
                if (baseline < 0) {
                    baseline = history.size
                }
                if (status == ThreadStatus.THREAD_STATUS_IDLE && snapshot.second.lastTurnError.isNotBlank()) {
                    st.items.clear()
                    st.items.addAll(historyToChatItems(history))
                    st.lastSequence=0
                    st.errorMessage=snapshot.second.lastTurnError
                    finishRecovery(threadId,st,ok=false)
                    recovered=true
                    break
                }
                val completedAfterReconnect =
                    status == ThreadStatus.THREAD_STATUS_IDLE &&
                        history.lastOrNull()?.role == "assistant"
                if (status == ThreadStatus.THREAD_STATUS_IDLE && (history.size > baseline || completedAfterReconnect)) {
                    recovered = true
                    st.items.clear()
                    st.items.addAll(historyToChatItems(history))
                    // 【P1-5】全量回填的内容不是逐帧应用出来的,事件断点对回填后
                    // 的本地视图不再有意义,必须作废 —— 否则下次恢复会从旧断点
                    // 重放已回填过的帧,时间线出现重复。
                    st.lastSequence = 0
                    finishRecovery(threadId, st, ok = true)
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
     *
     * 【R1 收口 · 时限与文案】RPC 带 [CANCEL_TURN_TIMEOUT_MS] 时限 —— 断线
     * 场景下一元调用可能无限挂起,终止键就永远等不到"下一步"。超时与报错
     * 分两档人话([cancelTurnFailureCopy]:禁裸词、不责备、给与界面真实按钮
     * 对齐的下一步),不再输出「终止失败:…」这类没有出路的裸句。
     */
    fun cancelTurn(threadId: String) {
        val st = state(threadId)
        if (!st.running && !st.recovering) return
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
                        withTimeout(CANCEL_TURN_TIMEOUT_MS) {
                            AgentConnection.threadService().cancelTurn(
                                CancelTurnRequest.newBuilder().setThreadId(threadId).build(),
                            )
                        }
                    }
                }.onSuccess {
                    // 电脑端已取消:同步一次历史,把已产出的部分内容拿回来
                    resyncFromDaemon(threadId, silent = true)
                }.onFailure {
                    // 协程被取消(页面销毁等)不算"没送达",不摆横幅;
                    // 超时(TimeoutCancellationException)与真报错分两档文案。
                    when (it) {
                        is kotlinx.coroutines.TimeoutCancellationException ->
                            st.recoveryError = cancelTurnFailureCopy(timedOut = true, e = it)
                        is kotlinx.coroutines.CancellationException -> Unit
                        else ->
                            st.recoveryError = cancelTurnFailureCopy(timedOut = false, e = it)
                    }
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
                // 【P1-5】全量回填后事件断点作废(与 recoverOrphanTurn 的全量路径
                // 同一语义):内容来源不是逐帧应用,旧断点重放会造成时间线重复。
                st.lastSequence = 0
                st.recovering = false
                st.recoveryError = null
                st.errorMessage = null
                // 【假完成修复】只有**真的从电脑端带回了新内容**才标记任务完成;
                // 旧判定 history.size >= current 在 0>=0 时也成立 —— 电脑端还没有回复,
                // 任务中心却显示"已完成",点进会话空空如也(用户反馈实锤)。
                if (history.size > current) {
                    TaskCenter.finishMessage(threadId, ok = true, detail = "已从电脑端同步结果")
                } else if (silent) {
                    // 取消后的兜底同步:没有新内容就标失败(取消即未完成),不冒充完成。
                    TaskCenter.finishMessage(threadId, ok = false, detail = "电脑端暂无回复内容")
                }
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
     *
     * option 是三按钮审批的选项 id(codex 应答枚举原词,与卡 options[].id 对应,
     * 如 acceptForSession);只有用户点第三按钮才填。daemon 把它与 note 包装成
     * {"option":..,"note":..} 信封并入 note 传给 provider(provider 侧
     * agent.parseApprovalChoice 解析)。空 = 旧端 approved/note 语义,daemon
     * 行为一字不变;approved 布尔语义保持(acceptForSession 也是 approved=true+option)。
     */
    fun answerApproval(threadId: String, requestId: String, approved: Boolean, note: String = "", option: String = "") {
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
        // C1 只增不改:三按钮的选项回填,只影响新端;空串不设字段,线上旧链路零变化。
        if (option.isNotEmpty()) decision.setOption(option)
        val result = st.replyCh?.trySend(
            TurnClientMessage.newBuilder()
                .setApproval(decision.build())
                .build(),
        )
        if (result?.isSuccess == true) {
            st.pendingApproval = null
            st.approvalError = null
        } else {
            scope.launch {
                try {
                    withContext(Dispatchers.IO) {
                        AgentConnection.threadService().submitApproval(SubmitApprovalRequest.newBuilder()
                            .setThreadId(threadId).setDecision(decision).build())
                    }
                    if(st.pendingApproval?.requestId==requestId) st.pendingApproval=null
                    st.approvalError=null
                } catch(e: Exception) {
                    // R1 收口:三选项(允许/本会话允许/拒绝)与旧两按钮同一条失败链,
                    // 人话原因统一走 approvalAnswerFailureCopy(原文透传 + 点「重试」
                    // 再交一次,与弹窗里原因正下方的重试按钮对齐)。
                    st.approvalError = approvalAnswerFailureCopy(e)
                }
            }
        }
    }
}

/**
 * "没有正文"这一轮的提示文案;null = 不该提示(有正文,或已被原生历史补偿回来)。
 *
 * 【为什么单独抽成纯函数】它是 state → (标题, 正文) 的纯映射,能脱离 Compose、协程
 * 与网络单测。而"哪一档该提示、提示什么"恰恰最容易被后来者顺手改坏 —— 比如把
 * RECOVERED 也提示一遍(正文明明已经补回来了),或把 UNSPECIFIED 当成"这一轮没有正文"。
 * 用测试钉住比用注释提醒可靠。
 *
 * 【UNSPECIFIED 为什么不提示】它是旧 daemon 的回放帧(那时还没有这个字段):此刻编
 * 一句话,等于把"对方没报状态"说成"这一轮确实没正文"。
 */
internal fun emptyOutputNotice(state: OutputState): Pair<String, String>? = when (state) {
    OutputState.OUTPUT_STATE_EMPTY ->
        "本轮没有正文" to "这一轮没有文字输出,也没有附件。"
    OutputState.OUTPUT_STATE_ARTIFACT_ONLY ->
        "本轮只有附件" to "这一轮没有文字说明,产物在上方。"
    OutputState.OUTPUT_STATE_ADAPTER_LOST_OUTPUT ->
        "正文可能丢了" to "电脑上有这一轮的记录,但正文没能取回。可以重试同一句话。"
    else -> null
}

/**
 * usage_update 投影里的"本会话累计"读取(N3·hermes 修复;纯函数,单测钉死)。
 *
 * 【为什么必须 hasTotalUsage】proto 消息字段未设置时 `getTotalUsage()` 返回的是
 * 默认实例而非 null —— 直接赋值等于把"daemon 没报累计"说成"累计是 0"。
 * hermes 适配器只有会话级 size/used、没有总量来源,usage_update 投影从不设置
 * total_usage,手机端"本会话累计"Tab 便对 hermes 恒显 0/0/0。daemon 侧补不了
 * (没有总量数据源),这里把"未设置"如实还原为 null,由卡面以"—"呈现未知。
 */
internal fun projectedTotalUsage(update: pocketagent.v1.UsageUpdate): pocketagent.v1.Usage? =
    if (update.hasTotalUsage()) update.totalUsage else null
