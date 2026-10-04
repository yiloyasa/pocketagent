package dev.pocketagent.app.ui.voice

import java.util.concurrent.atomic.AtomicBoolean
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

// =====================================================================================
// P4 Task 4 —— VoiceController:把 reducer 变成可驱动的状态机
//
// 单向数据流:UI/引擎回调 → send(event) → reduce → StateFlow<VoiceSessionState> +
// VoiceEffect 照单驱动引擎。所有状态写都在同一把锁内完成,指令恰好派发一次。
//
// 【ChatScreen 接线点(归波 3,本文件只留钩子)】
//  1. 组合层 remember { VoiceController(scope, initialThreadId = threadId) };
//  2. DisposableEffect/Lifecycle:页面隐藏/离开 → controller.onPageHidden()(Stop 语义);
//     永久销毁 → controller.release();
//  3. LaunchedEffect(threadId):会话切换 → controller.send(SessionSwitched(threadId));
//  4. 引擎选型(pickVoiceEngine 决策表)→ controller.bindEngine(SystemVoiceEngine.create(ctx)
//     ?: SenseVoiceEngine.create())。
// =====================================================================================

/**
 * 死区阈值:与旧路径 ui.VOICE_DEAD_ZONE_TIMEOUT_MS 同值(8 秒)。
 * 波 3 接线切换后旧常量应删除,二合一(见建议反馈)。
 */
internal const val VOICE_DEAD_ZONE_TIMEOUT_MS = 8_000L

/**
 * 看门狗调度器签名(本文件头声明的注入类型,实现与注入侧必须逐字一致):
 * 把 [onFire] 安排在 delayMs 之后执行一次;返回取消句柄(() -> Unit,撤狗)。
 * 实现自担时序(线程/协程均可),到点判定仍以 Controller 里的 monotonic 差值为准。
 */
internal typealias VoiceWatchdogScheduler = (delayMs: Long, onFire: () -> Unit) -> () -> Unit

/**
 * [VoiceWatchdogScheduler] 的默认实现:一次性 daemon 线程 sleep 到点后执行 onFire;
 * 返回的取消句柄 = 置位丢弃标记 + 中断线程(已开火则不再触发,未开火立即退出)。
 * 不依赖协程上下文(无 suspend 调用),JVM 单测不受影响(测试恒注入手动调度器)。
 */
internal fun defaultVoiceWatchdogScheduler(delayMs: Long, onFire: () -> Unit): () -> Unit {
    val cancelled = AtomicBoolean(false)
    val thread = Thread {
        try {
            Thread.sleep(delayMs)
        } catch (_: InterruptedException) {
            return@Thread
        }
        if (!cancelled.get()) onFire()
    }
    thread.isDaemon = true
    thread.name = "PaVoiceWatchdog"
    thread.start()
    return {
        cancelled.set(true)
        thread.interrupt()
    }
}

internal class VoiceController(
    private val scope: CoroutineScope,
    /**
     * monotonic 毫秒钟(默认 System.nanoTime() —— 单调、不受用户改系统时间/NTP 跳变
     * 影响;这是对旧路径 System.currentTimeMillis 看门狗的 monotonic 化。8 秒窗口不跨
     * 深睡眠,无需 SystemClock.elapsedRealtime 的"含睡眠"语义,且 nanoTime 可在 JVM
     * 单测直接注入假钟)。注入的返回值必须是单调递增毫秒。
     */
    private val clockMillis: () -> Long = { System.nanoTime() / 1_000_000 },
    private val deadZoneTimeoutMs: Long = VOICE_DEAD_ZONE_TIMEOUT_MS,
    initialThreadId: String? = null,
    /**
     * 看门狗定时器注入口,签名见文件头 [VoiceWatchdogScheduler] —— 与其逐字一致:
     * `(delayMs: Long, onFire: () -> Unit) -> () -> Unit`(返回取消句柄)。
     * 默认实现 [defaultVoiceWatchdogScheduler](daemon 线程,非挂起上下文,无 suspend 依赖)。
     * 抽出注入口是为了 JVM 单测不依赖真实时序:测试注入手动调度器,配合假钟确定性
     * 驱动"到点开火"。
     */
    private val scheduleWatchdog: VoiceWatchdogScheduler = { delayMs, onFire ->
        defaultVoiceWatchdogScheduler(delayMs, onFire)
    },
) {
    private val lock = Any()
    private val _state = MutableStateFlow(VoiceSessionState(threadId = initialThreadId))
    val state: StateFlow<VoiceSessionState> = _state.asStateFlow()

    @Volatile private var engine: VoiceEngine? = null
    private var engineEventsJob: Job? = null
    private var sessionIdCounter = 0L
    private var watchdogCancel: (() -> Unit)? = null

    /** 本轮(start 之后)是否收到过引擎任一回调 —— 死区判定与"零回调即 Stop 收口"共用。 */
    @Volatile private var callbackSeenThisRound = false

    /** 当前引擎绑定以来是否交付过 Final(端侧冷加载分流:从未出过稿的死区按"加载中"提示)。 */
    @Volatile private var everFinalSeen = false

    /** 展示态只读快照;reducer 纯函数在测试里直接覆盖,这里给 UI/波 3 接线用。 */
    fun current(): VoiceSessionState = _state.value

    /**
     * 驱动状态机的唯一入口:归约 → 状态落库 → 引擎指令派发(锁外,防引擎回调重入)。
     * Start 产生"新会话"迁移时在此递增分配 sessionId —— 只有 Controller 拥有发号器,
     * reducer 保持纯函数;引擎回调凭 sessionId 认亲,reducer 负责丢弃迟到的旧号。
     */
    fun send(event: VoiceEvent) {
        var effect: VoiceEffect? = null
        synchronized(lock) {
            // 引擎物理回调已发生即视为"服务活着"(喂狗),与 reducer 是否接受该事件无关:
            // 迟到/被静音丢弃的回调同样证明引擎没死,不该触发死区误报
            val sid = event.sessionIdOrNull()
            if (sid != null && sid == _state.value.sessionId) callbackSeenThisRound = true
            val reduction = reduce(_state.value, event)
            var next = reduction.state
            if (event is VoiceEvent.Start && next.phase == VoicePhase.REQUESTING_PERMISSION) {
                next = next.copy(sessionId = ++sessionIdCounter)
            }
            _state.value = next
            effect = reduction.effect
            if (event is VoiceEvent.Final || event is VoiceEvent.Error ||
                event is VoiceEvent.Cancel || event is VoiceEvent.SessionSwitched
            ) {
                disarmWatchdogLocked()
            }
        }
        effect?.let { dispatch(it) }
    }

    /** 引擎指令照单执行;引擎未绑定时的启动指令如实报错,绝不无声卡 STARTING。 */
    private fun dispatch(effect: VoiceEffect) {
        val engine = engine
        if (engine == null) {
            if (effect is VoiceEffect.StartEngine) {
                send(VoiceEvent.Error(effect.sessionId, VoiceErrorKind.ENGINE))
            }
            return
        }
        when (effect) {
            is VoiceEffect.StartEngine -> {
                armDeadZoneWatchdog(effect.sessionId)
                engine.start(effect.sessionId)
            }
            VoiceEffect.PauseEngine -> engine.pause()
            VoiceEffect.ResumeEngine -> engine.resume()
            VoiceEffect.StopEngine -> {
                engine.stop()
                val s = _state.value
                if (!callbackSeenThisRound && s.phase == VoicePhase.STOPPING) {
                    // 引擎自 start 起零回调(预检可用但服务死区的 ROM):stop 等不来 Final,
                    // 立即按死区收口,防止状态永远卡在 STOPPING
                    send(VoiceEvent.Error(s.sessionId, deadZoneKind()))
                }
            }
            VoiceEffect.CancelEngine -> engine.cancel() // 看门狗已在 send(Cancel) 时撤掉
        }
    }

    /**
     * 【死区兜底】start 指令派发时武装 8 秒看门狗:到点仍零回调、会话未换、还卡在
     * STARTING/LISTENING(判定见 [voiceDeadZoneShouldFire])→ 发死区错误回 ERROR 相。
     * elapsed 用 [clockMillis](monotonic)差值计算 —— 即便调度器晚点开火,判定也不被
     * 墙钟回拨污染(Task 5:看门狗 monotonic 化)。
     */
    private fun armDeadZoneWatchdog(sessionId: Long) {
        synchronized(lock) {
            disarmWatchdogLocked()
            callbackSeenThisRound = false
            val startedAt = clockMillis()
            // onFire 显式命名并作第二个位置实参传入(不用 trailing lambda,见门禁 3):
            // 到点用 monotonic 差值重算 elapsed,即便调度器晚点开火,判定也不被墙钟回拨污染
            val onFire = {
                val elapsedMs = clockMillis() - startedAt
                val s = _state.value
                if (voiceDeadZoneShouldFire(elapsedMs, callbackSeenThisRound, s.phase, s.sessionId, sessionId)) {
                    send(VoiceEvent.Error(sessionId, deadZoneKind()))
                }
            }
            watchdogCancel = scheduleWatchdog(deadZoneTimeoutMs, onFire)
        }
    }

    /**
     * 死区错误分类:端侧引擎且本绑定从未交付过 Final → 按"模型冷加载中"提示
     * (MODEL_LOADING,旧路径 firstColdLoad 语义);其余按"服务无响应"(DEAD_ZONE)。
     */
    private fun deadZoneKind(): VoiceErrorKind =
        if (engine?.kind == VoiceEngineKind.SENSEVOICE && !everFinalSeen) {
            VoiceErrorKind.MODEL_LOADING
        } else {
            VoiceErrorKind.DEAD_ZONE
        }

    private fun disarmWatchdogLocked() {
        watchdogCancel?.invoke()
        watchdogCancel = null
    }

    /** 引擎回调 → 领域事件翻译;Ready/Transcribing 映射为空文本 Partial 进度信号。 */
    private fun onEngineEvent(e: VoiceEngineEvent) {
        when (e) {
            is VoiceEngineEvent.Ready -> send(VoiceEvent.Partial(e.sessionId, VOICE_PROGRESS_SIGNAL))
            is VoiceEngineEvent.Partial -> send(VoiceEvent.Partial(e.sessionId, e.text))
            is VoiceEngineEvent.Transcribing -> send(VoiceEvent.Partial(e.sessionId, VOICE_PROGRESS_SIGNAL))
            is VoiceEngineEvent.Final -> {
                synchronized(lock) { everFinalSeen = true }
                send(VoiceEvent.Final(e.sessionId, e.text))
            }
            is VoiceEngineEvent.Failed -> send(VoiceEvent.Error(e.sessionId, e.kind))
        }
    }

    /**
     * 绑定/切换引擎(决策表选型结果):状态.engine 随绑更新,旧引擎立即销毁防泄漏;
     * everFinalSeen 重置 —— 冷加载分流以"当前引擎实例"为准。
     */
    fun bindEngine(newEngine: VoiceEngine?) {
        synchronized(lock) {
            val old = engine
            engine = newEngine
            everFinalSeen = false
            _state.update { it.copy(engine = newEngine?.kind ?: VoiceEngineKind.UNAVAILABLE) }
            engineEventsJob?.cancel()
            engineEventsJob = newEngine?.let { e ->
                scope.launch { e.events.collect { onEngineEvent(it) } }
            }
            if (old != null && old !== newEngine) old.destroy()
        }
    }

    /**
     * 【页面生命周期钩子 —— ChatScreen 接线归波 3】
     * 页面隐藏/离开(onStop / DisposableEffect onDispose)时调用,执行"Stop 语义"
     * (计划指定):收音中 → 停止并让引擎交付最终转写(未静音,属本会话,合规);
     * 还在等权限 → 取消请求。空闲/已失败不动。
     * 若产品语义要"离开即弃",波 3 可改发 Cancel + 静音,二选一在接线处决定。
     */
    fun onPageHidden() {
        when (_state.value.phase) {
            VoicePhase.REQUESTING_PERMISSION -> send(VoiceEvent.Cancel)
            VoicePhase.STARTING, VoicePhase.LISTENING, VoicePhase.PAUSED,
            VoicePhase.STOPPING, VoicePhase.TRANSCRIBING,
            -> send(VoiceEvent.Stop)
            VoicePhase.IDLE, VoicePhase.ERROR -> {}
        }
    }

    /** 页面永久销毁:撤看门狗、解绑并销毁引擎、状态归零。 */
    fun release() {
        synchronized(lock) { disarmWatchdogLocked() }
        bindEngine(null)
        _state.value = VoiceSessionState(threadId = _state.value.threadId)
    }
}
