package dev.pocketagent.app.ui.collab

import android.util.Log
import dev.pocketagent.app.AgentConnection
import io.grpc.Status
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.currentCoroutineContext
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withContext
import pocketagent.v1.CollabMember
import pocketagent.v1.CollabSessionEvent
import pocketagent.v1.DispatchCollabRequest
import java.util.concurrent.atomic.AtomicBoolean

/**
 * 协同会话仓库(P1 协同闭环 Task 6):daemon 状态在手机端的单一投影点。
 *
 * 【恢复链(与计划验收项逐条对应)】
 *  1. 进入页面先 `GetCollabSession` 恢复整屏(快照先行);
 *  2. 再 `WatchCollabSession(after_sequence = 快照水位)` 增量跟随 —— 水位来自
 *     已按序应用的事件断点([CollabSessionState.lastSequence],TurnSession
 *     generation+seq 同精神:帧按序应用,旧帧去重,缺口不自欺);
 *  3. seq 缺口 / 不认识的帧 → reducer 标 needsResync → 仓库拉全量快照重建
 *     (daemon 对慢消费者丢帧不补发,这是协议约定的对端自救);
 *  4. 断线:保留最后一个真快照,error 呈现"连接中断,显示的是最后状态";
 *     退避重连后续传(Watch 回放 seq > 水位的事件;超出保留窗口时 daemon
 *     改发全量快照帧,两条路都收口到真值)。
 *
 * 【为什么不走 HostEvents 总线】协同事件是**按会话**的有界流(带自己的单调
 * sequence 与快照兜底语义),与全局 WatchEvents 通知流的生命周期/恢复策略都
 * 不同;混用一条总线会把"通知"和"事实"重新搅在一起 —— TurnSession 的既定
 * 分工(通知流只当恢复唤醒信号)在这里同样成立。
 */
object CollabSessionRepository {

    private const val TAG = "CollabSessionRepo"

    /** Watch 流的重订节奏:与 WatchEventsLoop 同一条退避曲线(500ms 起,10s 封顶)。 */
    private const val RETRY_BASE_MS = 500L
    private const val RETRY_MAX_MS = 10_000L

    private val _state = MutableStateFlow(CollabSessionReducer.initial())
    val state: StateFlow<CollabSessionState> = _state.asStateFlow()

    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private val resyncMutex = Mutex()

    /** 当前观察的会话(离开页面不清,重开页面直接续;进程重启后由 openLatest 重建)。 */
    @Volatile
    private var activeSessionId: String? = null

    private var watchJob: Job? = null

    /** 仅测试/诊断用:当前观察的会话 id。 */
    fun currentSessionId(): String? = activeSessionId

    // ---- 生命周期 ----

    /**
     * 进入协同页:有正在观察的会话就续上;没有则向 daemon 要最新的**进行中**
     * 会话(不接受已终局的 —— 看旧协同应从成员子线程进,协同页管正在发生的)。
     */
    fun openLatest() {
        scope.launch {
            val sid = activeSessionId ?: listLatestActiveSessionId()
            if (sid == null) {
                _state.update { CollabSessionReducer.initial() }
                return@launch
            }
            open(sid)
        }
    }

    /**
     * 观察指定会话:先快照恢复,再开事件流。可重入 —— 同会话重复调用只刷新
     * 快照,不重启事件流(流本身带续传,无需重建)。
     */
    fun open(sessionId: String) {
        activeSessionId = sessionId
        scope.launch {
            _state.update { it.copy(loading = true) }
            val snap = try {
                AgentConnection.getCollabSession(sessionId)
            } catch (e: Exception) {
                _state.update { it.copy(loading = false, error = CollabCopy.DISCONNECT_NOTICE + " " + rpcMessage(e)) }
                null
            }
            if (snap != null) {
                // 开屏快照:水位保持现状(快照不带 seq);从零进入时水位=0,Watch 从头回放
                _state.update { CollabSessionReducer.resync(it, snap) }
            }
            startWatch(sessionId)
        }
    }

    /** 离开协同页:停事件流,保留快照与水位(重开页面从断点续传,不重看旧帧)。 */
    fun onPageHidden() {
        synchronized(this) {
            watchJob?.cancel()
            watchJob = null
        }
    }

    // ---- 意图(成员卡与会话级动作的唯一入口) ----

    /**
     * 处理一条 [CollabIntent]。OpenApproval 是导航意图,由 UI 层消费
     * (打开成员会话页,审批卡与 SubmitApproval 都在那里),这里显式不处理。
     */
    fun send(intent: CollabIntent) {
        when (intent) {
            // Dispatch 需要"选人 + 角色绑定"这份派发前配置,它们归 CollabState 管:
            // 单一入口在 CollabState.dispatch(防重复派发硬闸也住在那里),这里只转发。
            is CollabIntent.Dispatch -> dev.pocketagent.app.ui.CollabState.dispatch(intent.instruction)

            is CollabIntent.CancelMember -> send { sid ->
                val card = AgentConnection.cancelCollabMember(sid, intent.threadId)
                _state.update { CollabSessionReducer.applyMember(it, card) }
            }

            is CollabIntent.RetryMember -> send { sid ->
                val card = AgentConnection.retryCollabMember(sid, intent.threadId)
                _state.update { CollabSessionReducer.applyMember(it, card) }
            }

            is CollabIntent.CancelSession -> send { sid ->
                val snap = AgentConnection.cancelCollabSession(sid)
                // 取消应答是收敛后的会话快照:整屏换真,水位不动
                _state.update { CollabSessionReducer.resync(it, snap) }
            }

            is CollabIntent.OpenApproval -> Unit // 导航意图:UI 层消费(见函数 KDoc)
        }
    }

    private fun send(block: (sessionId: String) -> Unit) {
        val sid = activeSessionId ?: return
        scope.launch {
            try {
                block(sid)
            } catch (e: Exception) {
                // 操作失败的人话原文如实呈现(daemon 的前置条件错误,如"还在运行中,不能重试")
                _state.update { it.copy(error = rpcMessage(e)) }
            }
        }
    }

    /**
     * 派发一条共同指令(挂起直至 RPC 返回,调用方用自己的 scope 包裹以维持
     * 防重复派发闸)。[members] 来自 CollabState 的选人与角色绑定(派发前的
     * 用户意图归 CollabState 管,这里只收成品)。成功后仓库接管:快照恢复 +
     * 事件流跟随;失败把 daemon/传输层原文放进 [CollabSessionState.error]。
     */
    suspend fun dispatch(instruction: String, members: List<CollabMemberSpec>) {
        val cmd = instruction.trim()
        if (cmd.isEmpty() || members.isEmpty()) return
        // 调用方(CollabState)在主线程 scope 上等待本挂起点:阻塞 RPC 一律沉到 IO
        withContext(Dispatchers.IO) { dispatchInternal(cmd, members) }
    }

    private suspend fun dispatchInternal(cmd: String, members: List<CollabMemberSpec>) {
        // 新一单:旧会话视图与旧事件流一并退场(新会话新事件,旧结果在 daemon 留档)
        onPageHidden()
        activeSessionId = null
        _state.value = CollabSessionReducer.initial()
        _state.update { it.copy(status = CollabStatus.SUBMITTED, loading = true, error = null) }
        try {
            val req = DispatchCollabRequest.newBuilder().setInstruction(cmd)
            members.forEach { m ->
                req.addMembers(
                    CollabMember.newBuilder().setAgentId(m.agentId).setRoleId(m.roleId.orEmpty())
                )
            }
            val resp = AgentConnection.threadService().dispatchCollab(req.build())
            val sid = resp.sessionId
            if (sid.isBlank()) {
                _state.update { it.copy(loading = false, error = "协同已派发,但电脑端没有返回会话 id,无法跟进进度") }
                return
            }
            open(sid)
        } catch (e: Exception) {
            _state.update { it.copy(loading = false, error = rpcMessage(e)) }
        }
    }

    // ---- 事件流(照 WatchEventsLoop 的退避重订模式,续传水位来自 reducer 断点) ----

    private fun startWatch(sessionId: String) {
        synchronized(this) {
            if (watchJob?.isActive == true && activeSessionId == sessionId) return
            watchJob?.cancel()
            watchJob = scope.launch { watchLoop(sessionId) }
        }
    }

    private suspend fun watchLoop(sessionId: String) {
        var retryMs = RETRY_BASE_MS
        while (currentCoroutineContext().isActive && activeSessionId == sessionId) {
            val established = AtomicBoolean(false)
            val failure: Throwable? = try {
                // 续传:水位之后的帧由 daemon 回放;超过保留窗口 daemon 改发全量快照帧
                AgentConnection.watchCollabSession(sessionId, _state.value.lastSequence)
                    .collect { ev ->
                        established.set(true)
                        applyEvent(sessionId, ev)
                        retryMs = RETRY_BASE_MS // 健康流:退避复位(与 WatchEventsLoop 同纪律)
                    }
                null
            } catch (t: Throwable) {
                t
            }
            if (!currentCoroutineContext().isActive || activeSessionId != sessionId) break
            markDisconnected(established.get(), failure)
            delay(retryMs)
            retryMs = (retryMs * 2).coerceAtMost(RETRY_MAX_MS)
        }
    }

    private suspend fun applyEvent(sessionId: String, ev: CollabSessionEvent) {
        var needsResync = false
        _state.update { st ->
            val next = CollabSessionReducer.reduce(st, ev)
            needsResync = next.needsResync
            // 断线说明在新流的第一帧到来时清掉(会话级 daemon 错误由快照帧接管)
            val cleared = next.error != null && next.error == CollabCopy.DISCONNECT_NOTICE
            if (cleared) next.copy(error = null) else next
        }
        if (needsResync) resync(sessionId)
    }

    /** 缺口/未知帧的全量兜底:拉快照重建(互斥串行,失败保留现状稍后再试)。 */
    private suspend fun resync(sessionId: String) {
        resyncMutex.withLock {
            if (!_state.value.needsResync) return
            try {
                val snap = AgentConnection.getCollabSession(sessionId)
                _state.update { CollabSessionReducer.resync(it, snap) }
            } catch (e: Exception) {
                Log.w(TAG, "协同快照兜底失败: ${e.message ?: e::class.java.simpleName}")
                _state.update { it.copy(error = CollabCopy.RESYNC_FAILED) }
            }
        }
    }

    private fun markDisconnected(established: Boolean, failure: Throwable?) {
        if (failure != null) {
            Log.w(TAG, "协同事件流断开(established=$established): ${failure.message ?: failure::class.java.simpleName}")
        }
        // 只加说明,不动快照:断线期间"显示的是最后状态"是承诺,不是缺陷
        _state.update { it.copy(loading = false, error = CollabCopy.DISCONNECT_NOTICE) }
    }

    // ---- 查询辅助 ----

    private suspend fun listLatestActiveSessionId(): String? = try {
        AgentConnection.listCollabSessions(includeFinished = false)
            .firstOrNull { it.sessionId.isNotBlank() }?.sessionId
    } catch (e: Exception) {
        // 列举失败(旧 daemon 未实现):如实留空,协同页回到"派发前"形态
        Log.w(TAG, "列举协同会话失败: ${e.message ?: e::class.java.simpleName}")
        null
    }

    /** 异常 → 可透传原文(daemon 的 Status.description 优先,照 CollabState 既有纪律)。 */
    internal fun rpcMessage(e: Exception): String {
        val desc = Status.fromThrowable(e).description
        if (!desc.isNullOrBlank()) return desc
        return e.message?.takeIf { it.isNotBlank() } ?: e::class.java.simpleName
    }
}

/** 派发成员材料(选人 + 角色绑定;CollabState 的成品,仓库不再读选人状态)。 */
data class CollabMemberSpec(val agentId: String, val roleId: String?)
