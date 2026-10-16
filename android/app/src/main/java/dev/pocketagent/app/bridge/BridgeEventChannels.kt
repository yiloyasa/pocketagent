package dev.pocketagent.app.bridge

import io.flutter.plugin.common.BinaryMessenger
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.launch
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.drop
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.supervisorScope
import kotlinx.coroutines.withContext
import pocketagent.v1.HostEvent
import pocketagent.v1.HostInfo

/**
 * 事件面取数 seam（[BridgeConnection] 的流式子集 + HostInfo 变化流）。
 *
 * 【为什么不直接给 BridgeConnection 加 flow】[BridgeConnection] 是 B4b 落盘的
 * 既有文件，本波只读不改（见文件清单纪律）。事件面需要的是「变化即推」的流式
 * 观察面，与既有接口的「现值快照」是两种取数语义；在此收成一个新 seam，
 * 生产实现直接读 [dev.pocketagent.app.AgentConnection] 的五条 StateFlow 与
 * `hostEvents` SharedFlow（都是现成的真值源，零新造数据）。
 *
 * 【为什么继承 BridgeConnection】事件面的 conn_state 帧与 `getConnectionState`
 * 快照同源同键（§3.4），继承后生产实现只需把快照 getter 转给同一份 delegate，
 * 不会出现「事件与快照两份口径」。
 */
interface BridgeEventSources : BridgeConnection {
    /** connState 变化流（AgentConnection.connState）。 */
    val connStateFlow: StateFlow<String>

    /** connTransport 变化流（AgentConnection.connTransport）。 */
    val connTransportFlow: StateFlow<String>

    /** connAddress 变化流（AgentConnection.connAddress）。 */
    val connAddressFlow: StateFlow<String>

    /** connectionEpoch 变化流（AgentConnection.connectionEpoch；重连自增 = 全局刷新信号）。 */
    val connectionEpochFlow: StateFlow<Int>

    /** hostCapabilities 变化流（AgentConnection.hostCapabilities；能力位唯一真值）。 */
    val hostCapabilitiesFlow: StateFlow<List<String>>

    /** WatchEvents 全局流（AgentConnection.hostEvents；无缓冲 broadcast）。 */
    val hostEventsFlow: Flow<HostEvent>

    /**
     * HostInfo 变化流（`host_info` 帧的唯一来源）。
     *
     * 【生产源为什么不是 StateFlow】AgentConnection **不保留** HostInfo 快照
     * （探活/回退连接拿到 info 后只写 capabilities 共享快照 + 版本闸，整包丢弃，
     * 见 `AgentConnection.updateHostInfo` 的 KDoc）。因此本波的生产实现只能从
     * `updateHostInfo(info)` 的**成功路径**记录（见 [EventFeedingConnection]）：
     * 只有 Dart 侧发起 `getHostInfo` 且 daemon 应答成功时才推 host_info 帧。
     * 这是当前进程内唯一诚实的 HostInfo 来源——没有它就不推帧（红线：不伪造）。
     */
    val hostInfoFlow: Flow<HostInfo>
}

/**
 * 把 [BridgeConnection.updateHostInfo] 的成功路径同时喂给 host_info 事件帧。
 *
 * 【为什么需要它】`host_info` 的字节只能来自一次真实的 `GetHostInfo` 应答；
 * AgentConnection 不留快照，事件面就无处可取。本装饰器在 updateHostInfo 调用
 * 上补一次记录（原样转发给 delegate，行为零变化），事件面的
 * [BridgeEventSources.hostInfoFlow] 读这份记录。
 *
 * 【局限（如实记账）】只覆盖「经本实例的 updateHostInfo」这一条路径：
 *  - 覆盖：Dart 侧 `getHostInfo` → `BridgeMethods` → 本实例 → 记录 + 转发；
 *  - 不覆盖：AgentConnection 内部探活/回退连接自己调的 updateHostInfo（不过桥）。
 * 因此探活周期内 daemon 改了 capabilities 而 Dart 没调 getHostInfo 时，
 * host_info 帧不推（conn_state 帧会推，因为 capabilities 在五键里）。
 * 要全量覆盖需在 AgentConnection 保留 HostInfo 快照，属另一波次的改动
 * （本波不碰 AgentConnection）。
 */
class EventFeedingConnection(
    private val delegate: BridgeConnection,
    private val feed: HostInfoFeed = ProcessHostInfoFeed,
) : BridgeConnection by delegate {
    override fun updateHostInfo(info: HostInfo) {
        feed.publish(info)
        delegate.updateHostInfo(info)
    }
}

/** HostInfo 记录的收发面（生产用进程级单例，测试用实例级 fake）。 */
interface HostInfoFeed {
    fun publish(info: HostInfo)
    val updates: Flow<HostInfo>
}

/** 进程级 HostInfo 记录（updateHostInfo 成功路径 → host_info 帧的唯一桥）。 */
object ProcessHostInfoFeed : HostInfoFeed {
    // 私有缓冲字段与公开流不同名（同名会触发「hides member / conflicting
    // declarations」——Kotlin 不允许 override 成员与超类型成员同名再各自声明）。
    private val buffer = MutableSharedFlow<HostInfo>(extraBufferCapacity = 8)

    override fun publish(info: HostInfo) {
        buffer.tryEmit(info)
    }

    override val updates: Flow<HostInfo> get() = buffer
}

/**
 * 生产实现：[BridgeEventSources] → AgentConnection 单例。
 *
 * 【flows 为什么直接读 AgentConnection】五条 StateFlow 与 hostEvents 都是
 * AgentConnection 的公开成员（本波只读），事件面订阅它们等于订阅 app 既有的
 * 唯一真值源；不另建镜像、不轮询（轮询会漏帧也会造第二份真值）。
 */
class AgentEventSources(
    private val delegate: BridgeConnection = AgentConnectionBridge(),
) : BridgeEventSources, BridgeConnection by delegate {
    override val connStateFlow: StateFlow<String> get() = dev.pocketagent.app.AgentConnection.connState
    override val connTransportFlow: StateFlow<String> get() = dev.pocketagent.app.AgentConnection.connTransport
    override val connAddressFlow: StateFlow<String> get() = dev.pocketagent.app.AgentConnection.connAddress
    override val connectionEpochFlow: StateFlow<Int> get() = dev.pocketagent.app.AgentConnection.connectionEpoch
    override val hostCapabilitiesFlow: StateFlow<List<String>>
        get() = dev.pocketagent.app.AgentConnection.hostCapabilities
    override val hostEventsFlow: Flow<HostEvent> get() = dev.pocketagent.app.AgentConnection.hostEvents
    override val hostInfoFlow: Flow<HostInfo> get() = ProcessHostInfoFeed.updates
}

/**
 * 三条 EventChannel 的帧消费面（B4 事件面子任务）。
 *
 * 【通道与帧形状（契约 §4）】
 *  - `dev.pocketagent/bridge/events`：三类 payload 合流——
 *    `conn_state`（五键与 type 平铺，键名 = Kotlin 字段名，§3.4）、
 *    `host_info`（`{'type':'host_info','info': HostInfo wire 字节}`）、
 *    `host_event`（`{'type':'host_event','event': HostEvent wire 字节}`）；
 *  - `dev.pocketagent/bridge/turn_events`：逐帧 `TurnServerMessage` wire 字节（裸字节，非 Map）；
 *  - `dev.pocketagent/bridge/terminal_events`：逐帧 `TerminalServerMessage` wire 字节（裸字节，非 Map）。
 *
 * 【线程纪律】三条流的收集都在 IO scope 上跑；sink.success 由框架负责切到平台
 * 线程（EventChannel 的 reply 走 BinaryMessenger，与 MethodChannel 同机制）。
 */
class BridgeEventChannels(
    private val messenger: BridgeEventMessenger,
    private val sources: BridgeEventSources,
    private val log: BridgeLog = AndroidLog,
    private val scope: CoroutineScope = CoroutineScope(SupervisorJob() + Dispatchers.IO),
) {
    /** 当前主事件面的收集 job（onListen 建、onCancel 摘；null = 无订阅）。 */
    private var mainJob: Job? = null

    /** 当前 turn 流的会话（threadId → observer；onListen 建、onCancel/Cancel 摘）。 */
    private var turnSession: TurnStreamSession? = null

    /** 当前 terminal 流的会话（terminalId → observer；同上）。 */
    private var terminalSession: TerminalStreamSession? = null

    // ── 三个 handler（init 引用它们，必须声明在前——Kotlin 属性初始化顺序：
    // init 块按源码位置执行，引用下方属性 = 「must be initialized」） ──

    private val mainHandler = object : BridgeEventStreamHandler {
        override fun onListen(arguments: Any?, events: BridgeEventSink) {
            // 重复 listen（同一订阅方重订阅）= 先摘旧的再建新的：EventChannel 的
            // 0→1 语义保证不会并发两个订阅，但 Dart 侧 dispose 后重订是有的。
            mainJob?.cancel()
            mainJob = scope.launch { collectMainEvents(events) }
        }

        override fun onCancel(arguments: Any?) {
            mainJob?.cancel()
            mainJob = null
        }
    }

    // ── turn_events：StartTurn 双向流的过程帧 ──────────────────────────────

    private val turnHandler = object : BridgeEventStreamHandler {
        override fun onListen(arguments: Any?, events: BridgeEventSink) {
            // 【本波不自动建流】请求帧的唯一入口是方法面的 turnStreamStart/Send
            // （BridgeMethods 本波如实回 HOST_UNAVAILABLE，属 B4b 已定案的留后
            // 事项）。此刻建流 = 建一条永远收不到请求帧的流：daemon 侧挂着一个
            // 空 bidi 流，Dart 侧看着"通道接上了"却一帧不回——那是比"静默"更糟
            // 的假象（红线：不伪造）。故 listen 只记账，等下一波方法面接上后
            // 由 turnStreamStart 触发 [TurnStreamSession.start]。
            turnSession?.cancel()
            turnSession = null
        }

        override fun onCancel(arguments: Any?) {
            turnSession?.cancel()
            turnSession = null
        }
    }

    // ── terminal_events：Attach 双向流的过程帧 ─────────────────────────────

    private val terminalHandler = object : BridgeEventStreamHandler {
        override fun onListen(arguments: Any?, events: BridgeEventSink) {
            // 同 turn_events：请求帧入口（terminalAttach/terminalWrite）本波未接，
            // 不建空流。见 [turnHandler] 的 KDoc。
            terminalSession?.cancel()
            terminalSession = null
        }

        override fun onCancel(arguments: Any?) {
            terminalSession?.cancel()
            terminalSession = null
        }
    }

    init {
        messenger.setStreamHandler(BRIDGE_EVENT_CHANNEL_NAME, mainHandler)
        messenger.setStreamHandler(BRIDGE_TURN_EVENT_CHANNEL_NAME, turnHandler)
        messenger.setStreamHandler(BRIDGE_TERMINAL_EVENT_CHANNEL_NAME, terminalHandler)
    }

    // ── 主事件面：conn_state / host_info / host_event ───────────────────────

    /**
     * 三类帧合流收集（一个协程 + [supervisorScope] 三兄弟）。
     *
     * 【为什么三个兄弟而不是 merge】三类帧的错误处置不同（见下），分开收集后
     * 「一类炸了」不会连带另一类；supervisorScope 保证外层取消时三兄弟一起收。
     */
    private suspend fun collectMainEvents(sink: BridgeEventSink) = supervisorScope {
        launch { collectFrames(sink, "conn_state", connStateFrames()) }
        launch { collectFrames(sink, "host_info", hostInfoFrames()) }
        launch { collectFrames(sink, "host_event", hostEventFrames()) }
    }

    /**
     * 单类帧的收集 + 错误纪律。
     *
     *  - 单帧构建失败（proto 序列化炸）：**不进流**、记日志、后续帧照收
     *    （契约 §4「坏事件不毒化流」）；
     *  - 数据源本身失败（flow 抛异常）：`sink.error`，code 走 [BridgeErrors] 的
     *    映射表原样给（契约 §4「宿主 EventSink.error → 流错误，code 原样透传」），
     *    之后由 Dart 侧重订阅触发新的 onListen（本波不做自动重订阅：Dart 侧的
     *    恢复链是「取消订阅 → 重订阅」，宿主侧重订阅会造第二份推流源）。
     */
    private suspend fun collectFrames(
        sink: BridgeEventSink,
        kind: String,
        frames: Flow<Map<String, Any?>>,
    ) {
        try {
            frames.collect { frame -> sink.success(frame) }
        } catch (e: CancellationException) {
            throw e // 订阅方取消 = 正常收流，不当错误
        } catch (e: Throwable) {
            // StateFlow/SharedFlow 的收集只在取消时抛 CancellationException；
            // 走到这里的都是真异常（hostEvents 的上游出错等）。code 原样映射。
            val failure = grpcFailure("event:$kind", e)
            log.w(TAG, "事件面 $kind 数据源失败: ${failure.code}/${failure.message}")
            sink.error(failure.code, failure.message, failure.details)
        }
    }

    /**
     * conn_state 帧：五键任一变化 → 全量快照一帧（不发单字段增量）。
     *
     * 【为什么 drop(1)】combine 的首发射是「订阅瞬间的五键现值」，而契约 §4 明
     * 说「初值用 getConnectionState() 补，事件只承担增量」——首帧与 Dart 自己拉
     * 的快照逐字相同，推它是重复帧。丢掉首发射，之后任一 StateFlow 变化才推。
     */
    private fun connStateFrames(): Flow<Map<String, Any?>> = combine(
        sources.connStateFlow,
        sources.connTransportFlow,
        sources.connAddressFlow,
        sources.connectionEpochFlow,
        sources.hostCapabilitiesFlow,
    ) { state, transport, address, epoch, caps ->
        connStateFrame(state, transport, address, epoch, caps)
    }.drop(1)

    /** host_info 帧：HostInfo 变化 → `{'type':'host_info','info': bytes}`（字节直转）。 */
    private fun hostInfoFrames(): Flow<Map<String, Any?>> =
        sources.hostInfoFlow.map { info ->
            mapOf("type" to "host_info", "info" to infoBytes(info))
        }

    /** host_event 帧：WatchEvents 逐帧 → `{'type':'host_event','event': bytes}`（字节直转）。 */
    private fun hostEventFrames(): Flow<Map<String, Any?>> =
        sources.hostEventsFlow.map { event ->
            mapOf("type" to "host_event", "event" to eventBytes(event))
        }

    /**
     * 为 [threadId] 建一条 StartTurn 双向流并把过程帧推给 Dart（下一波方法面入口）。
     *
     * 预留给 `turnStreamStart`：方法面接到本类后，dispatch 里对应分支调本方法。
     * 本波无调用方（诚实缺席）， machinery 由单测直接驱动验证。
     */
    internal fun startTurnStream(threadId: String, stub: BridgeThreadStreamService, sink: BridgeEventSink) {
        turnSession?.cancel()
        turnSession = TurnStreamSession(threadId, stub, sink, scope, log)
    }

    /** 摘掉当前 turn 流（`turnStreamCancel` / 订阅方取消共用；防泄漏判决点）。 */
    internal fun cancelTurnStream() {
        turnSession?.cancel()
        turnSession = null
    }

    /** 为 [terminalId] 建一条 Attach 双向流（下一波 `terminalAttach` 入口）。 */
    internal fun startTerminalStream(
        terminalId: String,
        stub: BridgeTerminalStreamService,
        sink: BridgeEventSink,
    ) {
        terminalSession?.cancel()
        terminalSession = TerminalStreamSession(terminalId, stub, sink, scope, log)
    }

    /** 摘掉当前 terminal 流（`terminalClose` / 订阅方取消共用）。 */
    internal fun cancelTerminalStream() {
        terminalSession?.cancel()
        terminalSession = null
    }

    /** 测试/进程收尾：摘掉全部流与收集 job（防协程泄漏）。 */
    fun shutdown() {
        mainJob?.cancel()
        mainJob = null
        turnSession?.cancel()
        turnSession = null
        terminalSession?.cancel()
        terminalSession = null
        scope.cancel()
    }

    companion object {
        private const val TAG = "BridgeEventChannels"

        /**
         * conn_state 帧装配（五键 + type 平铺；键名 = Kotlin 字段名，§3.4）。
         *
         * 单独成函数是为了让单测不建流也能逐键对账（帧形状是契约，不是实现细节）。
         */
        internal fun connStateFrame(
            state: String,
            transport: String,
            address: String,
            epoch: Int,
            capabilities: List<String>,
        ): Map<String, Any?> = mapOf(
            "type" to "conn_state",
            "connState" to state,
            "connTransport" to transport,
            "connAddress" to address,
            "connectionEpoch" to epoch,
            "hostCapabilities" to capabilities,
        )

        /** HostInfo → wire 字节（`writeToBuffer`，与 getHostInfo 方法面同源）。 */
        internal fun infoBytes(info: HostInfo): ByteArray = info.toByteArray()

        /** HostEvent → wire 字节（同上）。 */
        internal fun eventBytes(event: HostEvent): ByteArray = event.toByteArray()
    }
}

/** StartTurn 双向流的会话态（threadId 绑定；cancel 即摘 gRPC 流）。 */
internal class TurnStreamSession(
    private val threadId: String,
    stub: BridgeThreadStreamService,
    private val sink: BridgeEventSink,
    private val scope: CoroutineScope,
    private val log: BridgeLog,
) {
    /** 请求侧 observer（Dart 经 turnStreamSend 发来的帧由它进流；null = 流已摘）。 */
    @Volatile private var requestObserver: io.grpc.stub.StreamObserver<pocketagent.v1.TurnClientMessage>? = null

    private val job: Job = scope.launch {
        try {
            val observer = object : io.grpc.stub.StreamObserver<pocketagent.v1.TurnServerMessage> {
                override fun onNext(value: pocketagent.v1.TurnServerMessage) {
                    // 逐帧直转：bytes = writeToBuffer（与 Dart bytesOrThrow 对账）。
                    sink.success(value.toByteArray())
                }

                override fun onError(t: Throwable) {
                    val failure = grpcFailure("turnStream:$threadId", t)
                    log.w("BridgeEventChannels", "turn 流错误($threadId): ${failure.code}/${failure.message}")
                    sink.error(failure.code, failure.message, failure.details)
                }

                override fun onCompleted() {
                    sink.endOfStream()
                }
            }
            requestObserver = stub.startTurn(observer)
        } catch (e: CancellationException) {
            throw e
        } catch (e: Throwable) {
            // 建流本身就失败（通道未建/半死）：不当成"流错了之后还能用"——
            // 如实把失败推成流错误，Dart 侧走它既有的错误分支。
            val failure = grpcFailure("turnStreamStart:$threadId", e)
            log.w("BridgeEventChannels", "turn 建流失败($threadId): ${failure.code}/${failure.message}")
            sink.error(failure.code, failure.message, failure.details)
        }
    }

    /** 取消：先取消 gRPC 流（防泄漏），再取消收集 job。 */
    fun cancel() {
        runCatching {
            (requestObserver as? io.grpc.stub.ClientCallStreamObserver<*>)
                ?.cancel("turn 流取消（订阅方取消/turnStreamCancel）", null)
        }
        job.cancel()
        requestObserver = null
    }
}

/** Attach 双向流的会话态（terminalId 绑定；cancel 即摘 gRPC 流）。 */
internal class TerminalStreamSession(
    private val terminalId: String,
    stub: BridgeTerminalStreamService,
    private val sink: BridgeEventSink,
    private val scope: CoroutineScope,
    private val log: BridgeLog,
) {
    @Volatile private var requestObserver: io.grpc.stub.StreamObserver<pocketagent.v1.TerminalClientMessage>? = null

    private val job: Job = scope.launch {
        try {
            val observer = object : io.grpc.stub.StreamObserver<pocketagent.v1.TerminalServerMessage> {
                override fun onNext(value: pocketagent.v1.TerminalServerMessage) {
                    sink.success(value.toByteArray())
                }

                override fun onError(t: Throwable) {
                    val failure = grpcFailure("terminalStream:$terminalId", t)
                    log.w("BridgeEventChannels", "terminal 流错误($terminalId): ${failure.code}/${failure.message}")
                    sink.error(failure.code, failure.message, failure.details)
                }

                override fun onCompleted() {
                    sink.endOfStream()
                }
            }
            requestObserver = stub.attach(observer)
        } catch (e: CancellationException) {
            throw e
        } catch (e: Throwable) {
            val failure = grpcFailure("terminalAttach:$terminalId", e)
            log.w("BridgeEventChannels", "terminal 建流失败($terminalId): ${failure.code}/${failure.message}")
            sink.error(failure.code, failure.message, failure.details)
        }
    }

    fun cancel() {
        runCatching {
            (requestObserver as? io.grpc.stub.ClientCallStreamObserver<*>)
                ?.cancel("terminal 流取消（订阅方取消/terminalClose）", null)
        }
        job.cancel()
        requestObserver = null
    }
}

/**
 * ThreadService 双向流窄接口（只列 StartTurn 一笔）。
 *
 * 【为什么窄接口】protoc 生成的 stub 是 final class（B4b 实测：无法继承/覆写），
 * 测试要驱动「observer.onNext → 推帧」必须有可假实现的缝。与
 * [BridgeThreadService] 同纪律：接口面只含被调用的 RPC。
 */
interface BridgeThreadStreamService {
    fun startTurn(
        responseObserver: io.grpc.stub.StreamObserver<pocketagent.v1.TurnServerMessage>,
    ): io.grpc.stub.StreamObserver<pocketagent.v1.TurnClientMessage>
}

/** TerminalService 双向流窄接口（只列 Attach 一笔）。 */
interface BridgeTerminalStreamService {
    fun attach(
        responseObserver: io.grpc.stub.StreamObserver<pocketagent.v1.TerminalServerMessage>,
    ): io.grpc.stub.StreamObserver<pocketagent.v1.TerminalClientMessage>
}

/**
 * 生产装配入口（一行接线，供 FlutterEngine 挂载点调用）：
 *
 * ```
 * installEventChannels(engine.dartExecutor.binaryMessenger, connection)
 * ```
 *
 * 【与 BridgeMethods.install 的实例关系】若希望 `host_info` 帧生效，两处必须传
 * **同一个**连接实例，且该实例是 [EventFeedingConnection] 装饰过的：
 *
 * ```
 * val connection = EventFeedingConnection(AgentConnectionBridge())
 * BridgeMethods.install(messenger, connection)
 * installEventChannels(messenger, connection)
 * ```
 *
 * 只加 `installEventChannels(messenger)` 一行也能跑：conn_state / host_event
 * 两帧全量生效，host_info 因没有经本实例的 updateHostInfo 调用而静默
 * （如实缺席，不推假帧；要补就按上面三行共享实例）。
 */
fun installEventChannels(
    messenger: BinaryMessenger,
    connection: BridgeConnection = AgentConnectionBridge(),
    log: BridgeLog = AndroidLog,
): BridgeEventChannels {
    val sources: BridgeEventSources = when (connection) {
        is BridgeEventSources -> connection
        else -> AgentEventSources(connection)
    }
    return BridgeEventChannels(FlutterEventMessengerAdapter(messenger), sources, log)
}

/** 测试/内部装配入口（seam 直驱，不碰 Flutter 引擎）。 */
internal fun installEventChannels(
    messenger: BridgeEventMessenger,
    sources: BridgeEventSources,
    log: BridgeLog = AndroidLog,
    scope: CoroutineScope = CoroutineScope(SupervisorJob() + Dispatchers.IO),
): BridgeEventChannels = BridgeEventChannels(messenger, sources, log, scope)

/**
 * 测试辅助：等 [condition] 为真或超时（协程在 IO 上跑，固定睡眠要么不够要么浪费）。
 */
internal fun awaitUntil(timeoutMs: Long = 5_000, condition: () -> Boolean) {
    val deadline = System.currentTimeMillis() + timeoutMs
    while (!condition()) {
        if (System.currentTimeMillis() > deadline) {
            throw AssertionError("等待条件 ${timeoutMs}ms 未满足（协程卡死或推流路径没走通）")
        }
        Thread.yield()
    }
}

/** 测试辅助：切 IO 跑挂起体（收集类断言需要）。 */
internal suspend fun <T> onIo(block: suspend () -> T): T = withContext(Dispatchers.IO) { block() }
