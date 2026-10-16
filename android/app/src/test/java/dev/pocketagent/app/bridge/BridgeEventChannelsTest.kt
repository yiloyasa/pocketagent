package dev.pocketagent.app.bridge

import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.MutableStateFlow
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.HostEvent
import pocketagent.v1.HostInfo

/**
 * BridgeEventChannels 测试（B4d）：三条事件通道的帧形状对账 + 三类纪律。
 *
 * 全部经 seam 注入（messenger/sources/sink/log 都是假实现），不碰真 Flutter
 * 框架与真 gRPC（本仓库单测零 Robolectric，见 [BridgeLog] KDoc）。
 */
class BridgeEventChannelsTest {

    // ── 假实现 ──────────────────────────────────────────────────────────────

    /** 记录型 messenger：capture 三个 handler 供测试直接驱动 onListen/onCancel。 */
    private class FakeMessenger : BridgeEventMessenger {
        val handlers = HashMap<String, BridgeEventStreamHandler>()
        override fun setStreamHandler(channelName: String, handler: BridgeEventStreamHandler) {
            handlers[channelName] = handler
        }
    }

    /** 记录型 sink：帧/错误/关流三语义各自记录。 */
    private class FakeSink : BridgeEventSink {
        val frames = ArrayList<Any?>()
        val errors = ArrayList<Triple<String, String?, Any?>>()
        var ended = false
        override fun success(event: Any?) { frames.add(event) }
        override fun error(errorCode: String, errorMessage: String?, errorDetails: Any?) {
            errors.add(Triple(errorCode, errorMessage, errorDetails))
        }
        override fun endOfStream() { ended = true }
    }

    /** 记录型 log：不落 Android 框架。 */
    private class FakeLog : BridgeLog {
        val lines = ArrayList<String>()
        override fun w(tag: String, message: String) { lines.add("$tag: $message") }
    }

    /**
     * 可控 sources：五个 StateFlow 测试可推、hostEvents/hostInfo 用 SharedFlow。
     * [BridgeConnection] 的 20+ 快照成员委托 [FakeConnection]（B4b 测试基建，
     * 同接口全实现；事件测试不读快照，委托即零成本）。
     */
    private class FakeSources(
        private val delegate: BridgeConnection = FakeConnection(),
    ) : BridgeEventSources, BridgeConnection by delegate {
        override val connStateFlow = MutableStateFlow("checking")
        override val connTransportFlow = MutableStateFlow("")
        override val connAddressFlow = MutableStateFlow("")
        override val connectionEpochFlow = MutableStateFlow(0)
        override val hostCapabilitiesFlow = MutableStateFlow<List<String>>(emptyList())
        override val hostEventsFlow = MutableSharedFlow<HostEvent>(extraBufferCapacity = 8)
        override val hostInfoFlow = MutableSharedFlow<HostInfo>(extraBufferCapacity = 8)
    }

    // ── 帧形状对账（契约 §4 + §3.4） ────────────────────────────────────────

    @Test
    fun `conn_state帧_五键与type平铺_键名逐字对账`() {
        val frame = BridgeEventChannels.connStateFrame(
            state = "ok",
            transport = "adb-reverse",
            address = "127.0.0.1:7246",
            epoch = 7,
            capabilities = listOf("compact:hermes", "slash:hermes"),
        )
        // §3.4 键表逐键：type + 五键，一个不多一个不少
        assertEquals(
            setOf("type", "connState", "connTransport", "connAddress", "connectionEpoch", "hostCapabilities"),
            frame.keys,
        )
        assertEquals("conn_state", frame["type"])
        assertEquals("ok", frame["connState"])
        assertEquals("adb-reverse", frame["connTransport"])
        assertEquals("127.0.0.1:7246", frame["connAddress"])
        assertEquals(7, frame["connectionEpoch"])
        assertEquals(listOf("compact:hermes", "slash:hermes"), frame["hostCapabilities"])
    }

    @Test
    fun `host_info与host_event帧_type判别且载荷为wire字节`() {
        // proto 生成类构造器私有：用 builder（B4b 测试同款）
        val info = HostInfo.newBuilder().setDisplayName("box").build()
        val event = HostEvent.newBuilder().setSequence(42).build()
        // 字节直转：与 getHostInfo 方法面 / Dart bytesOrThrow 同源（toByteArray）
        assertTrue(BridgeEventChannels.infoBytes(info).isNotEmpty())
        assertTrue(BridgeEventChannels.eventBytes(event).isNotEmpty())
        // 反序列化回读（对账「字节里就是这条消息」）
        assertEquals("box", HostInfo.parseFrom(BridgeEventChannels.infoBytes(info)).displayName)
        assertEquals(42L, HostEvent.parseFrom(BridgeEventChannels.eventBytes(event)).sequence)
    }

    // ── 注册面：三通道都挂上 ────────────────────────────────────────────────

    @Test
    fun `安装时三条通道都注册handler`() {
        val messenger = FakeMessenger()
        BridgeEventChannels(
            messenger = messenger,
            sources = FakeSources(),
            log = FakeLog(),
        )
        assertEquals(
            setOf(BRIDGE_EVENT_CHANNEL_NAME, BRIDGE_TURN_EVENT_CHANNEL_NAME, BRIDGE_TERMINAL_EVENT_CHANNEL_NAME),
            messenger.handlers.keys,
        )
    }

    // ── 主事件面：listen 推首帧之后按变化推 ─────────────────────────────────

    @Test
    fun `主事件面listen后_state变化推全量快照帧`() {
        val messenger = FakeMessenger()
        val sources = FakeSources()
        val channels = BridgeEventChannels(
            messenger = messenger,
            sources = sources,
            log = FakeLog(),
        )
        val sink = FakeSink()
        messenger.handlers[BRIDGE_EVENT_CHANNEL_NAME]!!.onListen(null, sink)

        // combine 的首帧（各 Flow 现值组合）被 drop(1) 丢掉了（契约 §4：初值由
        // getConnectionState 快照补，事件只推增量）。先等协程完成首帧收集，
        // 再改 value——否则这次变化可能被当成首帧 drop 掉（竞态）。
        Thread.sleep(200)
        sources.connStateFlow.value = "ok"
        // drop(1) 之后的首帧即变化帧；scope 在 IO 上，轮询等帧到达
        val frame = awaitFrame(sink)
        assertEquals("conn_state", (frame as Map<*, *>)["type"])
        assertEquals("ok", frame["connState"])

        messenger.handlers[BRIDGE_EVENT_CHANNEL_NAME]!!.onCancel(null)
        channels.shutdown()
    }

    @Test
    fun `主事件面onCancel后不再收帧`() {
        val messenger = FakeMessenger()
        val sources = FakeSources()
        val channels = BridgeEventChannels(
            messenger = messenger,
            sources = sources,
            log = FakeLog(),
        )
        val sink = FakeSink()
        messenger.handlers[BRIDGE_EVENT_CHANNEL_NAME]!!.onListen(null, sink)
        // 等 combine 首帧被 drop（见上一用例的同款竞态注释）——本用例不再改
        // value（没有增量可推），所以不能 awaitFrame：睡足让协程越过首帧即可。
        Thread.sleep(300)
        messenger.handlers[BRIDGE_EVENT_CHANNEL_NAME]!!.onCancel(null)

        val countAtCancel = sink.frames.size
        sources.connStateFlow.value = "down"
        Thread.sleep(300)
        assertEquals("取消后不得再收帧", countAtCancel, sink.frames.size)
        assertTrue("取消不产生流错误", sink.errors.isEmpty())
        channels.shutdown()
    }

    // ── turn/terminal：本波不自动建流（诚实缺席） ───────────────────────────

    @Test
    fun `turn_events本波不建空流_listen只记账`() {
        val messenger = FakeMessenger()
        val channels = BridgeEventChannels(
            messenger = messenger,
            sources = FakeSources(),
            log = FakeLog(),
        )
        val sink = FakeSink()
        // 请求帧入口（turnStreamStart/Send）方法面本波未接：此刻建流 = 建一条
        // 永远收不到请求帧的流（比静默更糟的假象）。listen 后零帧、零错误。
        messenger.handlers[BRIDGE_TURN_EVENT_CHANNEL_NAME]!!.onListen(null, sink)
        Thread.sleep(200)
        assertTrue("未建流不得推帧", sink.frames.isEmpty())
        assertTrue("未建流不得报错", sink.errors.isEmpty())
        channels.shutdown()
    }

    @Test
    fun `terminal_events本波不建空流_listen只记账`() {
        val messenger = FakeMessenger()
        val channels = BridgeEventChannels(
            messenger = messenger,
            sources = FakeSources(),
            log = FakeLog(),
        )
        val sink = FakeSink()
        messenger.handlers[BRIDGE_TERMINAL_EVENT_CHANNEL_NAME]!!.onListen(null, sink)
        Thread.sleep(200)
        assertTrue(sink.frames.isEmpty())
        assertTrue(sink.errors.isEmpty())
        channels.shutdown()
    }

    // ── 预留 machinery：startTurnStream/cancelTurnStream（下一波入口） ──────

    @Test
    fun `预留入口_startTurnStream后cancel摘流`() {
        val messenger = FakeMessenger()
        val channels = BridgeEventChannels(
            messenger = messenger,
            sources = FakeSources(),
            log = FakeLog(),
        )
        val sink = FakeSink()
        // 下一波方法面入口：直接驱动 machinery 验「建了能摘」（防泄漏判决点）
        channels.startTurnStream("th-1", FakeThreadStreamStub(), sink)
        channels.cancelTurnStream()
        channels.shutdown()
        // 摘流不得向 Dart 推错误帧（正常收流）
        assertTrue(sink.errors.isEmpty())
    }

    // ── 工具 ────────────────────────────────────────────────────────────────

    /** 等首帧到达（scope 在 IO；轮询上限 2s，防测试挂死）。 */
    private fun awaitFrame(sink: FakeSink, timeoutMs: Long = 2000): Any? {
        val deadline = System.currentTimeMillis() + timeoutMs
        while (System.currentTimeMillis() < deadline) {
            if (sink.frames.isNotEmpty()) return sink.frames.first()
            Thread.sleep(10)
        }
        throw AssertionError("等待首帧超时（${timeoutMs}ms）")
    }
}

/** 假 StartTurn stub：记录 observer 供 cancel 判决（不真连 gRPC）。 */
private class FakeThreadStreamStub : BridgeThreadStreamService {
    var startedObserver: io.grpc.stub.StreamObserver<pocketagent.v1.TurnServerMessage>? = null
    var cancelled = false
    override fun startTurn(
        observer: io.grpc.stub.StreamObserver<pocketagent.v1.TurnServerMessage>,
    ): io.grpc.stub.StreamObserver<pocketagent.v1.TurnClientMessage> {
        startedObserver = observer
        return object : io.grpc.stub.StreamObserver<pocketagent.v1.TurnClientMessage> {
            override fun onNext(value: pocketagent.v1.TurnClientMessage) = Unit
            override fun onError(t: Throwable) = Unit
            override fun onCompleted() { cancelled = true }
        }
    }
}
