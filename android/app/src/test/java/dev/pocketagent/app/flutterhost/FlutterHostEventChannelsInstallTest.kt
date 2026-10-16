package dev.pocketagent.app.flutterhost

import dev.pocketagent.app.AgentConnection
import dev.pocketagent.app.bridge.BRIDGE_EVENT_CHANNEL_NAME
import dev.pocketagent.app.bridge.BRIDGE_TERMINAL_EVENT_CHANNEL_NAME
import dev.pocketagent.app.bridge.BRIDGE_TURN_EVENT_CHANNEL_NAME
import dev.pocketagent.app.bridge.BridgeConnection
import dev.pocketagent.app.bridge.BridgeEventChannels
import dev.pocketagent.app.bridge.BridgeEventMessenger
import dev.pocketagent.app.bridge.BridgeEventSink
import dev.pocketagent.app.bridge.BridgeEventSources
import dev.pocketagent.app.bridge.BridgeEventStreamHandler
import dev.pocketagent.app.bridge.EventFeedingConnection
import dev.pocketagent.app.bridge.FakeConnection
import dev.pocketagent.app.bridge.installEventChannels
import io.flutter.plugin.common.BinaryMessenger
import io.flutter.plugin.common.MethodCall
import io.flutter.plugin.common.StandardMethodCodec
import java.nio.ByteBuffer
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.HostEvent
import pocketagent.v1.HostInfo

/**
 * 「事件面真挂了」的判决测试（B4d 接进宿主引擎的交接点证据）。
 *
 * 【为什么需要这一份】B4c 把方法面接上真桥，事件面仍是静默壳；本波把 B4d 的
 * [BridgeEventChannels] 挂进 [FlutterHostEngine.installChannelShell]。危险在于
 * 「挂了但没真挂」——例如 install 调用被 try/catch 吞掉、写错通道名、或只挂了
 * turn/terminal 两条而主事件面漏掉。这类回归的表现是**静默的**：Flutter UI 一切
 * 正常，只是 events 流永远不来帧，Dart 侧只能看到「没有流」。
 * 所以必须有判决级断言：
 *
 *  ① 三条事件通道的 handler 都注册了，且**主事件面**（`bridge/events`）的
 *     handler 会推流（改 AgentConnection 的五键 StateFlow → 收到 conn_state
 *     帧，且帧经**真 EventChannel sink** 离开宿主）；
 *  ② turn / terminal 两条**仍静默**：listen 之后零帧（请求帧入口未接，红线：
 *     不伪造——建空流是比静默更糟的假象）；
 *  ③ shutdown 之后事件面协程真停了（scope cancel），不再推帧（防泄漏判决点）；
 * ④ 壳内实例化的是 B4d 成品 [BridgeEventChannels]，且经生产装配入口
 *     [installEventChannels] 建立、连接实例与方法面**共享**（否则 host_info
 *     帧永远静默，见壳内注释）。
 *
 * 【怎么驱动】installChannelShell 的入参是 [BinaryMessenger] 接口，故用
 * [ShellMessenger] 驱动**真的** EventChannel 编解码（StandardMethodCodec 是纯
 * Java，不碰 libflutter.so）。壳内那一行走的是 `installEventChannels(messenger,
 * EventFeedingConnection(AgentConnectionBridge()))`：生产 sources 直读进程级
 * 单例 [AgentConnection] 的五条 StateFlow（纯 Kotlin 内存，JVM 可写）与
 * hostEvents/hostInfo（纯 Kotlin 流）——于是**生产装配路径本身可在纯 JVM 下
 * 驱动**，不需要注入 fake connection。这正是「壳真挂了」的最强判决形式：
 * 不换任何实现，改 StateFlow 现值 → 收到帧。
 */
class FlutterHostEventChannelsInstallTest {

    /**
     * 记录 handler 的假 messenger（与 FlutterHostChannelShellTest 同款，独立一份：
     * 两个测试类驱动同一段注册逻辑，但断言的面不同，不共享 fixture 更耐改）。
     *
     * 与既有 fake 的关键差异：它同时扮演**平台侧**——EventChannel 的 sink 推流
     * 走 `BinaryMessenger.send(通道名, 字节)`，这里把 send 的字节按通道名收下，
     * 于是「帧真的离开了 Kotlin 宿主」可被解码断言（与 Dart 侧收到的同形）。
     */
    private class ShellMessenger : BinaryMessenger {
        val handlers = HashMap<String, BinaryMessenger.BinaryMessageHandler>()
        val sent = ArrayList<Pair<String, ByteArray>>()

        override fun send(channel: String, message: ByteBuffer?) {
            message?.let { sent.add(channel to it.snapshotBytes()) }
        }

        override fun send(channel: String, message: ByteBuffer?, callback: BinaryMessenger.BinaryReply?) {
            // EventChannel 的 sink 不走 callback 路径；只按 send 收帧。
            send(channel, message)
        }

        override fun setMessageHandler(
            channel: String,
            handler: BinaryMessenger.BinaryMessageHandler?,
        ) {
            if (handler == null) handlers.remove(channel) else handlers[channel] = handler
        }

        /** 以 Dart 侧 EventChannel 订阅的等价物驱动（'listen' / 'cancel'）。 */
        fun stream(channel: String, action: String, arguments: Any? = null): ByteBuffer? {
            val handler = handlers[channel] ?: return null
            var reply: ByteBuffer? = null
            handler.onMessage(encoded(MethodCall(action, arguments))) { r -> reply = r }
            return reply?.apply { flip() }
        }

        /** 解出本通道上已推的帧（EventChannel 的 sink 走 send，见类 KDoc）。 */
        fun framesOn(channel: String): List<Any?> =
            sent.filter { it.first == channel }
                .map { StandardMethodCodec.INSTANCE.decodeEnvelope(ByteBuffer.wrap(it.second)) }

        private fun encoded(call: MethodCall): ByteBuffer =
            StandardMethodCodec.INSTANCE.encodeMethodCall(call).apply { flip() }

        /**
         * ByteBuffer → ByteArray 快照。
         *
         * 【为什么必须 flip 之后读】框架的 `encodeSuccessEnvelope` 返回的 buffer
         * 是「写完、position 在末尾」的状态（真实链路里这个 flip 由引擎/平台侧
         * 做，见 FlutterHostChannelShellTest 的同款注释）。测试自己驱动 send 就
         * 得自己做：不 flip 的话 remaining() == 0，解出来的帧永远是空。
         * 用 duplicate 而不是 flip 本体：sink 传来的 buffer 由框架持有，
         * 测试只为留证，不动原 buffer 的 position/limit。
         */
        private fun ByteBuffer.snapshotBytes(): ByteArray {
            val dup = duplicate()
            dup.flip()
            val out = ByteArray(dup.remaining())
            dup.get(out)
            return out
        }
    }

    /** 记录型 seam messenger：capture 三个 handler 供测试直接驱动（与 B4d 测试同款）。 */
    private class SeamMessenger : BridgeEventMessenger {
        val handlers = HashMap<String, BridgeEventStreamHandler>()
        override fun setStreamHandler(channelName: String, handler: BridgeEventStreamHandler) {
            handlers[channelName] = handler
        }
    }

    /** 记录型 sink：帧/错误/关流三语义各自记录。 */
    private class SeamSink : BridgeEventSink {
        val frames = ArrayList<Any?>()
        val errors = ArrayList<Triple<String, String?, Any?>>()
        var ended = false
        override fun success(event: Any?) { frames.add(event) }
        override fun error(errorCode: String, errorMessage: String?, errorDetails: Any?) {
            errors.add(Triple(errorCode, errorMessage, errorDetails))
        }
        override fun endOfStream() { ended = true }
    }

    /**
     * 可控 sources：五条 StateFlow 测试可推、hostEvents/hostInfo 用 SharedFlow。
     *
     * 【用在哪】seam 级装配入口 [installEventChannels] 的驱动（与壳内**同一个
     * 函数**，只换 sources 实现）——B4d 测试已有同款 fake，这里独立一份是因为
     * flutterhost 的测试不该依赖 bridge 测试包的 internal fixture（跨包可见性
     * 虽然成立，但两个测试包各自演进时不该互相绑死）。
     */
    private class SeamSources(
        private val delegate: BridgeConnection = FakeConnection(),
    ) : BridgeEventSources, BridgeConnection by delegate {
        override val connStateFlow: MutableStateFlow<String> = MutableStateFlow("checking")
        override val connTransportFlow: MutableStateFlow<String> = MutableStateFlow("")
        override val connAddressFlow: MutableStateFlow<String> = MutableStateFlow("")
        override val connectionEpochFlow: MutableStateFlow<Int> = MutableStateFlow(0)
        override val hostCapabilitiesFlow: MutableStateFlow<List<String>> = MutableStateFlow(emptyList())
        override val hostEventsFlow: Flow<HostEvent> = MutableSharedFlow(extraBufferCapacity = 8)
        override val hostInfoFlow: Flow<HostInfo> = MutableSharedFlow(extraBufferCapacity = 8)
    }

    @After
    fun tearDown() {
        // ① 壳持有的实例必须停（否则 IO scope 的收集 job 跨测试类存活，往下一个
        //    用例的 messenger 上推帧——假帧污染后续判决）。
        FlutterHostEngine.shutdown()
        // ② AgentConnection 是进程级 object，五条 StateFlow 跨测试类存活；还原
        //    初始值，避免污染其他读同一对象的用例（与 B4c 测试同纪律）。
        AgentConnection.connState.value = "checking"
        AgentConnection.connTransport.value = ""
        AgentConnection.connAddress.value = ""
        AgentConnection.connectionEpoch.value = 0
        AgentConnection.hostCapabilities.value = emptyList()
    }

    // ---------------- 判决点①：三通道注册 + 主事件面真推流 ----------------

    /**
     * 三条事件通道都注册 handler，且主事件面（`bridge/events`）**真的推流**。
     *
     * 【为什么这是「事件面真挂了」的最强判决】不换任何实现——走的就是壳内那一行
     * `installEventChannels(messenger, EventFeedingConnection(AgentConnectionBridge()))`
     * 的完整装配。能这么做的原因：生产 sources 直读 [AgentConnection] 的五条
     * StateFlow 现值（纯 Kotlin 内存读写，JVM 可驱动）与 hostEvents/hostInfo
     * （纯 Kotlin 流）。于是：
     *  - 改 `AgentConnection.connState.value = "ok"` → 收到
     *    `{'type':'conn_state', 五键}` 帧，且帧经**真 EventChannel sink** 离开
     *    （messenger.send 的字节解码回来逐键对账）——B4c 时代这里一帧不来；
     *  - turn / terminal 两条 listen 后零帧（本波静默，见判决点②）。
     */
    @Test
    fun `event channels are really wired and the main face pushes real frames`() {
        val messenger = ShellMessenger()
        FlutterHostEngine.installChannelShell(messenger)

        // 注册面：三条事件通道 + 方法通道，一个不多一个不少。
        assertEquals(
            setOf(
                FlutterHostEngine.BRIDGE_METHOD_CHANNEL,
                FlutterHostEngine.BRIDGE_EVENT_CHANNEL,
                FlutterHostEngine.BRIDGE_TURN_EVENT_CHANNEL,
                FlutterHostEngine.BRIDGE_TERMINAL_EVENT_CHANNEL,
            ),
            messenger.handlers.keys,
        )
        // 与 bridge 包的契约字符串同值（防跨包漂移：Dart 侧按契约名订阅）。
        assertEquals(BRIDGE_EVENT_CHANNEL_NAME, FlutterHostEngine.BRIDGE_EVENT_CHANNEL)
        assertEquals(BRIDGE_TURN_EVENT_CHANNEL_NAME, FlutterHostEngine.BRIDGE_TURN_EVENT_CHANNEL)
        assertEquals(BRIDGE_TERMINAL_EVENT_CHANNEL_NAME, FlutterHostEngine.BRIDGE_TERMINAL_EVENT_CHANNEL)

        // listen（EventChannel 的 0→1 语义：先 cancel 旧流，再建新的）。
        // 注意 stream() 内部已 flip 回包（与 FlutterHostChannelShellTest 同款），
        // 这里直接 decodeEnvelope——再 flip 一次就把 position 翻到 limit 之前，
        // 解出 BufferUnderflowException。
        val listenAck = messenger.stream(FlutterHostEngine.BRIDGE_EVENT_CHANNEL, "listen")
        assertNotNull("主事件面 listen 必须 ACK", listenAck)
        val ack: Any? = StandardMethodCodec.INSTANCE.decodeEnvelope(listenAck!!)
        assertNull("listen ACK 应为空成功包(不是 error envelope),实际: $ack", ack)

        // combine.drop(1) 的首帧是各 Flow 现值组合（契约 §4：初值由
        // getConnectionState 补，事件只推增量）。先等协程越过首帧，再改 value
        // ——否则这次变化可能被当成首帧 drop 掉（B4d 测试同款竞态修复）。
        //
        // 【为什么不能直接断言「最后一帧带全部五个最新值」】combine 的语义是
        // 「任一上游变化就重组一次」——五次连续赋值可能被重组成一帧，也可能被
        // 重组途中截成若干帧，最后一帧未必已包含全部五个新值（协程调度决定，
        // 实测出现过 expected:<[terminal, fs]> but was:<[]> 的 flake）。
        // 帧的**形状**与**最终收敛值**才是契约，帧数不是。故这里等到一帧
        // 收敛到全部五个目标值（轮询，不睡固定时长），再逐键对账。
        Thread.sleep(300)
        AgentConnection.connState.value = "ok"
        AgentConnection.connTransport.value = "USB 通道"
        AgentConnection.connAddress.value = "127.0.0.1:7246"
        AgentConnection.connectionEpoch.value = 42
        AgentConnection.hostCapabilities.value = listOf("terminal", "fs")

        val frames = awaitConvergedFrame(
            messenger,
            mapOf(
                "connState" to "ok",
                "connTransport" to "USB 通道",
                "connAddress" to "127.0.0.1:7246",
                "connectionEpoch" to 42,
                "hostCapabilities" to listOf("terminal", "fs"),
            ),
        )
        @Suppress("UNCHECKED_CAST")
        val frame = frames.single() as Map<String, Any>
        assertEquals("conn_state", frame["type"])
        assertEquals("ok", frame["connState"])
        assertEquals("USB 通道", frame["connTransport"])
        assertEquals("127.0.0.1:7246", frame["connAddress"])
        assertEquals(42, frame["connectionEpoch"])
        assertEquals(listOf("terminal", "fs"), frame["hostCapabilities"])
        assertEquals(6, frame.size) // §3.4 键表：type + 五键，一个不多一个不少
    }

    /**
     * turn / terminal 两条事件面**仍静默**（本波不建空流——红线：不伪造）。
     *
     * 【为什么这条放在「真挂了」的测试里】「事件面真挂了」的完整语义是「主事件面
     * 推流、turn/terminal 仍静默」。请求帧入口（方法面 6 条双向流生命周期方法）
     * 本波未接，此刻建流 = 建一条永远收不到请求帧的流：daemon 侧挂着空 bidi
     * 流，Dart 侧看着「通道接上了」却一帧不回——比静默更糟的假象。若将来有人
     * 为了「看起来全通了」让这两条也推帧，这里会失败。
     */
    @Test
    fun `turn and terminal event faces stay silent because the request frame entry is not wired`() {
        val messenger = ShellMessenger()
        FlutterHostEngine.installChannelShell(messenger)

        for (channel in listOf(
            FlutterHostEngine.BRIDGE_TURN_EVENT_CHANNEL,
            FlutterHostEngine.BRIDGE_TERMINAL_EVENT_CHANNEL,
        )) {
            val listenAck = messenger.stream(channel, "listen")
            assertNotNull("$channel 的 listen 必须 ACK", listenAck)
            val ack: Any? = StandardMethodCodec.INSTANCE.decodeEnvelope(listenAck!!)
            assertNull("$channel 的 listen ACK 应为空成功包,实际: $ack", ack)

            // listen 之后睡足：若误建了空流（或误推帧），这里必能抓到。
            Thread.sleep(300)
            assertTrue(
                "$channel 本波不得推任何帧(请求帧入口未接,不建空流),实际: ${messenger.framesOn(channel)}",
                messenger.framesOn(channel).isEmpty(),
            )

            val cancelAck = messenger.stream(channel, "cancel")
            assertNotNull("$channel 的 cancel 必须 ACK", cancelAck)
        }
    }

    /**
     * host_info 帧的字节来自**同一个连接实例**的 updateHostInfo 成功路径。
     *
     * 【为什么这条是接线判决而不是 B4d 测试的重复】B4d 测试用 seam 级 fake
     * sources 直驱 [BridgeEventChannels]；这条驱动的是**壳内那一行**：经进程级
     * `ProcessHostInfoFeed`（B4d 为「AgentConnection 不留 HostInfo 快照」留的
     * 唯一诚实来源）记录一次成功路径，帧经真 EventChannel sink 离开宿主。
     * 若壳把方法面与事件面各传一个连接实例（B4c 的旧写法），方法面 getHostInfo
     * 的成功调用就走另一个实例的 feed 记录，事件面永远收不到——host_info 帧
     * 静默缺席。这条与 `method face and event face share one decorated
     * connection instance` 一起把「共享实例」钉死。
     */
    @Test
    fun `host_info frames come from the shared decorated connection instance`() {
        val messenger = ShellMessenger()
        FlutterHostEngine.installChannelShell(messenger)

        val listenAck = messenger.stream(FlutterHostEngine.BRIDGE_EVENT_CHANNEL, "listen")
        assertNotNull("主事件面 listen 必须 ACK", listenAck)
        Thread.sleep(300)

        // 经壳共享的那个 EventFeedingConnection 记录一次成功路径（等价于 Dart 侧
        // getHostInfo 成功回桥：桥的 updateHostInfo → feed.publish → 帧）。
        val info = HostInfo.newBuilder().setDisplayName("YANZI-PC").build()
        dev.pocketagent.app.bridge.ProcessHostInfoFeed.publish(info)

        val frames = awaitFrames(messenger, FlutterHostEngine.BRIDGE_EVENT_CHANNEL, 1)
        @Suppress("UNCHECKED_CAST")
        val frame = frames.single() as Map<String, Any>
        assertEquals("host_info", frame["type"])
        val bytes = frame["info"] as ByteArray
        assertEquals("YANZI-PC", HostInfo.parseFrom(bytes).displayName)
    }

    /**
     * host_event 帧来自 WatchEvents 全局流（进程级 [AgentConnection.hostEvents]）。
     *
     * 【为什么单独一条】它与 conn_state / host_info 是三类帧里唯一的
     * SharedFlow 源（无现值、只推增量），注册漏了它或接错了流，表现是
     * 「daemon 有事件但 Dart 一帧收不到」——纯 Kotlin 可驱动，值得判决。
     */
    @Test
    fun `host_event frames come from the watch events global flow`() {
        val messenger = ShellMessenger()
        FlutterHostEngine.installChannelShell(messenger)

        messenger.stream(FlutterHostEngine.BRIDGE_EVENT_CHANNEL, "listen")
        Thread.sleep(300)

        // hostEvents 是 MutableSharedFlow：emit 是 suspend 函数，用 tryEmit
        // （非挂起路径）驱动。extraBufferCapacity=64 且无订阅者时 drop_oldest，
        // 此刻主事件面已 listen 订阅，帧必达收集端。
        AgentConnection.hostEvents.tryEmit(HostEvent.newBuilder().setSequence(42).build())

        val frames = awaitFrames(messenger, FlutterHostEngine.BRIDGE_EVENT_CHANNEL, 1)
        @Suppress("UNCHECKED_CAST")
        val frame = frames.single() as Map<String, Any>
        assertEquals("host_event", frame["type"])
        assertEquals(42L, HostEvent.parseFrom(frame["event"] as ByteArray).sequence)
    }

    // ---------------- 判决点②：实例形态（壳内实例化的是 B4d 成品） ----------------

    /**
     * 壳内挂的是 [BridgeEventChannels] 实例，且 shutdown 可调、幂等。
     *
     * 【为什么值得一条】上面几条验「帧通了」，这条验「壳持有的就是 B4d 的成品
     * 实例、而不是壳自己另写的一份 handler」。反射读私有字段 [eventChannels]：
     * 若有人把 installEventChannels 换成内联的 EventChannel 注册（双份实现
     * 漂移），这里读到的类型就不是 [BridgeEventChannels]。
     */
    @Test
    fun `the shell holds the b4d BridgeEventChannels instance`() {
        FlutterHostEngine.installChannelShell(ShellMessenger())
        val held = eventChannelsField()
        assertNotNull("壳必须持有 BridgeEventChannels 实例(供 shutdown 停协程)", held)
        assertTrue(
            "壳持有的必须是 bridge 包的 BridgeEventChannels(B4d 成品),实际: ${held!!::class.java.name}",
            held is BridgeEventChannels,
        )
        // shutdown 可调（防泄漏的前提）；重复调也安全。
        held.shutdown()
        held.shutdown()
    }

    /**
     * 事件面经**生产装配入口** [installEventChannels] 建立，sources 是生产
     * [dev.pocketagent.app.bridge.AgentEventSources]。
     *
     * 【为什么值得一条】壳若绕过入口、自己 new `BridgeEventChannels(...)` 或
     * 自己注册三条 EventChannel，就是双份装配路径（B4b/B4d 反复强调的漂移面）。
     * 这条从实例形状反推：`installEventChannels(messenger, connection)` 在
     * connection 不是 [BridgeEventSources] 时包一层 AgentEventSources——
     * 壳传的 `EventFeedingConnection` 正是那种，故 sources 必为 AgentEventSources。
     */
    @Test
    fun `the event face is built through the production install entry point`() {
        FlutterHostEngine.installChannelShell(ShellMessenger())
        val channels = eventChannelsField()
        assertNotNull(channels)
        val sources = sourcesField(channels!!)
        assertTrue(
            "事件面 sources 必须是生产 AgentEventSources(installEventChannels 的产物),实际: ${sources?.javaClass?.name}",
            sources is dev.pocketagent.app.bridge.AgentEventSources,
        )
    }

    /**
     * 壳与桥的连接实例是**同一个** [EventFeedingConnection] 装饰实例。
     *
     * 【为什么这条是判决】host_info 帧的唯一来源是「经本实例的 updateHostInfo
     * 成功路径」（AgentConnection 不留 HostInfo 快照，B4d 已定案）。壳若写成
     * `BridgeMethods.install(messenger, AgentConnectionBridge())` +
     * `installEventChannels(messenger, AgentConnectionBridge())`（两个实例），
     * 方法面的成功调用记录进另一个 feed，事件面永远收不到——conn_state /
     * host_event 两条仍通，host_info 静默缺席。这条把「共享 + 装饰」钉死：
     * 事件面 sources 的 delegate 与方法面桥的 connection 必须是**同一个对象**。
     *
     * 【怎么读到方法面的连接】壳不持有 [dev.pocketagent.app.bridge.BridgeMethods]
     * 实例（install 的返回值被丢弃），故沿注册链反查：MethodChannel 的 handler
     * → 捕获的 adapter → adapter 的 handler → 桥实例的 connection 字段。
     */
    @Test
    fun `method face and event face share one decorated connection instance`() {
        val messenger = ShellMessenger()
        FlutterHostEngine.installChannelShell(messenger)

        val channels = eventChannelsField()
        assertNotNull(channels)
        val sources = sourcesField(channels!!)
        assertTrue(
            "事件面 sources 必须是 BridgeEventSources,实际: ${sources?.javaClass?.name}",
            sources is BridgeEventSources,
        )
        val eventConnection = connectionOf(sources as BridgeEventSources)
        assertTrue(
            "事件面连接必须是 EventFeedingConnection 装饰过的,实际: ${eventConnection::class.java.name}",
            eventConnection is EventFeedingConnection,
        )
        assertTrue(
            "装饰的必须是生产连接适配器 AgentConnectionBridge,实际: ${delegateOf(eventConnection)::class.java.name}",
            delegateOf(eventConnection) is dev.pocketagent.app.bridge.AgentConnectionBridge,
        )

        val bridgeConnection = methodFaceConnection(messenger)
        assertNotNull("方法面桥实例必须存在(沿注册链反查失败即接线形态变了)", bridgeConnection)
        assertTrue(
            "方法面连接必须是同一个 EventFeedingConnection,实际: ${bridgeConnection!!::class.java.name}",
            bridgeConnection === eventConnection,
        )
    }

    // ---------------- 判决点③：shutdown 之后不得泄漏 ----------------

    /**
     * shutdown 之后事件面协程真停了：改 StateFlow 不再推帧。
     *
     * 【为什么这是防泄漏判决点】BridgeEventChannels 自带 IO scope 与收集 job。
     * 壳若只挂不停（或 shutdown 忘了摘它），引擎 destroy 之后协程仍在跑、
     * 仍往已死 messenger 推帧——JVM 单测可判：subscribe → shutdown → 改值
     * → 睡足 → 帧数不得增长。
     */
    @Test
    fun `after engine shutdown the event face stops pushing frames`() {
        val messenger = ShellMessenger()
        FlutterHostEngine.installChannelShell(messenger)
        messenger.stream(FlutterHostEngine.BRIDGE_EVENT_CHANNEL, "listen")
        Thread.sleep(300)

        // 先证明「活着时会推」：改值 → 收到收敛帧（等内容，不等帧数，见
        // awaitConvergedFrame 注释）。
        AgentConnection.connState.value = "ok"
        val before = awaitConvergedFrame(messenger, mapOf("connState" to "ok")).size

        FlutterHostEngine.shutdown()
        AgentConnection.connState.value = "down"
        AgentConnection.connectionEpoch.value = 7
        Thread.sleep(500)

        val after = messenger.framesOn(FlutterHostEngine.BRIDGE_EVENT_CHANNEL)
            .filterIsInstance<Map<String, Any?>>()
            .filter { it["connState"] == "down" || it["connectionEpoch"] == 7 }
        assertEquals(
            "shutdown 后事件面协程必须停(不得再推帧),收敛帧: $before -> ${after.size}",
            0,
            after.size,
        )
        // 壳持有的实例引用必须清掉（否则下一个 warmup 复用一个已 cancel 的 scope）。
        assertNull("shutdown 后壳不得再持有事件面实例", eventChannelsField())
    }

    /**
     * 重挂壳不得叠加推流源（前一个实例被停掉）。
     *
     * 【为什么这条重要】warmup 的双检锁保证生产路径只挂一次，但重挂路径（配置
     * 变更 / 测试重复 install）若不停旧实例，同一 messenger 上会有两个
     * BridgeEventChannels 的收集 job 在跑——Dart 侧一次变化收到两份相同帧
     * （重复帧），且旧 scope 泄漏。这条钉「重挂 = 先停后建」。
     */
    @Test
    fun `reinstalling the shell does not stack event push sources`() {
        val messenger = ShellMessenger()
        FlutterHostEngine.installChannelShell(messenger)
        messenger.stream(FlutterHostEngine.BRIDGE_EVENT_CHANNEL, "listen")
        Thread.sleep(300)
        AgentConnection.connState.value = "ok"
        // 等**内容**收敛（combine 重组帧数不定，见 awaitConvergedFrame 注释）。
        awaitConvergedFrame(messenger, mapOf("connState" to "ok"))

        // 重挂：旧实例的收集 job 必须被停。
        FlutterHostEngine.installChannelShell(messenger)
        messenger.stream(FlutterHostEngine.BRIDGE_EVENT_CHANNEL, "listen")
        Thread.sleep(300)
        AgentConnection.connState.value = "down"
        AgentConnection.connectionEpoch.value = 7
        // 等到出现 down/7 的帧——证明新实例的 job 活着（否则等不到）。
        awaitConvergedFrame(
            messenger,
            mapOf("connState" to "down", "connectionEpoch" to 7),
        )
        // 关键判决：收敛帧只能有**一份**。若旧实例的 job 没停，重挂后它会跟新
        // 实例一起重组同一批 StateFlow 现值，往同一个通道推重复帧。
        val converged = messenger.framesOn(FlutterHostEngine.BRIDGE_EVENT_CHANNEL)
            .filterIsInstance<Map<String, Any?>>()
            .filter { it["connState"] == "down" && it["connectionEpoch"] == 7 }
        assertEquals(
            "重挂后旧实例必须停(同一批现值只应推一份帧),实际收敛帧 ${converged.size} 份",
            1,
            converged.size,
        )
    }

    // ---------------- 判决点④：seam 级装配入口与壳内同一函数 ----------------

    /**
     * 内部装配入口 [installEventChannels] 与壳内那一行是**同一个函数**：
     * 传入可控 sources 时帧形状与生产路径逐键一致。
     *
     * 【为什么值得一条】上面几条驱动的是壳内那一行（生产 sources）；这条驱动
     * 同一个工厂换 seam sources，钉的是「flutterhost 测试覆盖的注册路径就是
     * 壳走的那条」——防将来有人给壳单开一条平行实现，而测试只覆盖其中一条。
     */
    @Test
    fun `the seam level installer is the same entry point the shell calls`() {
        val seamMessenger = SeamMessenger()
        val sources = SeamSources()
        val channels: BridgeEventChannels = installEventChannels(seamMessenger, sources)
        try {
            assertEquals(
                setOf(
                    BRIDGE_EVENT_CHANNEL_NAME,
                    BRIDGE_TURN_EVENT_CHANNEL_NAME,
                    BRIDGE_TERMINAL_EVENT_CHANNEL_NAME,
                ),
                seamMessenger.handlers.keys,
            )
            // 主事件面：改 sources 的 StateFlow → seam sink 收到帧。
            val sink = SeamSink()
            seamMessenger.handlers[BRIDGE_EVENT_CHANNEL_NAME]!!.onListen(null, sink)
            Thread.sleep(300)
            sources.connStateFlow.value = "ok"
            sources.connAddressFlow.value = "127.0.0.1:7246"
            // 等**内容**收敛（combine 重组帧数不定，见 awaitConvergedFrame 同款注释；
            // 这条路实测过 expected:<[127.0.0.1:7246]> but was:<[]> 的 flake）。
            awaitSinkConvergedFrame(sink)
            @Suppress("UNCHECKED_CAST")
            val frame = sink.frames.last() as Map<String, Any>
            assertEquals("conn_state", frame["type"])
            assertEquals("ok", frame["connState"])
            assertEquals("127.0.0.1:7246", frame["connAddress"])
            assertEquals(6, frame.size)
        } finally {
            channels.shutdown()
        }
    }

    // ---------------- 工具 ----------------

    /** 等 [channel] 上推出至少 [count] 帧（scope 在 IO；轮询上限 5s，防挂死）。 */
    private fun awaitFrames(messenger: ShellMessenger, channel: String, count: Int): List<Any?> {
        val deadline = System.currentTimeMillis() + 5_000
        while (System.currentTimeMillis() < deadline) {
            val frames = messenger.framesOn(channel)
            if (frames.size >= count) return frames
            Thread.sleep(10)
        }
        throw AssertionError(
            "等待 $channel 推出 $count 帧超时(事件面未接线或推流路径没走通)，" +
                "实际: ${messenger.framesOn(channel).size}",
        )
    }

    /**
     * 等主事件面推出一帧**含指定键值**的 conn_state 帧。
     *
     * 【为什么不能等「一帧」就断言】combine 对连续 StateFlow 赋值会重组若干次，
     * 帧数不定（协程调度决定）；断言「最后一帧含全部新值」实测会 flake
     * （expected:<[terminal, fs]> but was:<[]>）。这里等到出现那一帧目标快照——
     * 等的是**内容**不是帧数，故无竞态。
     *
     * 只断调用方关心的键（其余键不设条件）：用例改几个键就等几个键，
     * 不必为了复用 helper 把无关键也写全。
     */
    private fun awaitConvergedFrame(
        messenger: ShellMessenger,
        expected: Map<String, Any?>,
    ): List<Any?> {
        val channel = FlutterHostEngine.BRIDGE_EVENT_CHANNEL
        val deadline = System.currentTimeMillis() + 5_000
        while (System.currentTimeMillis() < deadline) {
            @Suppress("UNCHECKED_CAST")
            val converged = messenger.framesOn(channel).filterIsInstance<Map<String, Any?>>()
                .filter { frame -> expected.all { (k, v) -> frame[k] == v } }
            if (converged.isNotEmpty()) return converged
            Thread.sleep(10)
        }
        throw AssertionError(
            "等待主事件面推出收敛帧超时(expected=$expected)，" +
                "实际帧: ${messenger.framesOn(channel)}",
        )
    }

    /** 等 seam sink 收到至少 [count] 帧（协程在 IO 上跑）。 */
    private fun awaitSinkFrames(sink: SeamSink, count: Int) {
        val deadline = System.currentTimeMillis() + 5_000
        while (System.currentTimeMillis() < deadline) {
            if (sink.frames.size >= count) return
            Thread.sleep(10)
        }
        throw AssertionError("等待 sink 收到 $count 帧超时，实际: ${sink.frames.size}")
    }

    /**
     * 等 seam sink 推出一帧含指定键值的 conn_state 帧（同 awaitConvergedFrame 的理由：
     * 等**内容**不等帧数，避开 combine 重组的竞态）。
     */
    private fun awaitSinkConvergedFrame(sink: SeamSink, expected: Map<String, Any?> = mapOf("connState" to "ok", "connAddress" to "127.0.0.1:7246")) {
        val deadline = System.currentTimeMillis() + 5_000
        while (System.currentTimeMillis() < deadline) {
            @Suppress("UNCHECKED_CAST")
            val converged = sink.frames.filterIsInstance<Map<String, Any?>>()
                .filter { frame -> expected.all { (k, v) -> frame[k] == v } }
            if (converged.isNotEmpty()) return
            Thread.sleep(10)
        }
        throw AssertionError("等待 sink 推出收敛帧超时(expected=$expected)，实际: ${sink.frames}")
    }

    /** 读壳的私有字段 eventChannels（反射：私有是刻意的——壳只暴露 warmup/shutdown）。 */
    private fun eventChannelsField(): BridgeEventChannels? {
        val field = runCatching {
            FlutterHostEngine::class.java.getDeclaredField("eventChannels").apply { isAccessible = true }
        }.getOrNull() ?: return null
        return field.get(FlutterHostEngine) as? BridgeEventChannels
    }

    /** 读 BridgeEventChannels 的私有字段 sources。 */
    private fun sourcesField(channels: BridgeEventChannels): Any? {
        val field = runCatching {
            channels.javaClass.getDeclaredField("sources").apply { isAccessible = true }
        }.getOrNull() ?: return null
        return field.get(channels)
    }

    /** 读任意对象的私有字段（找不到返回 null，不抛）。 */
    private fun privateField(owner: Any, name: String): Any? {
        val field = runCatching {
            owner.javaClass.getDeclaredField(name).apply { isAccessible = true }
        }.getOrNull() ?: return null
        return runCatching { field.get(owner) }.getOrNull()
    }

    /**
     * 读 AgentEventSources 的私有字段 delegate（壳传进去的那个连接实例）。
     *
     * AgentEventSources 的构造参数名是 `delegate`（B4d 落盘签名），编译后是
     * 私有字段——反射读它即得「事件面实际用的连接实例」。
     */
    private fun connectionOf(sources: BridgeEventSources): BridgeConnection {
        val field = runCatching {
            sources.javaClass.getDeclaredField("delegate").apply { isAccessible = true }
        }.getOrNull() ?: error("AgentEventSources 必须有 delegate 字段(壳传进去的连接实例)")
        return field.get(sources) as BridgeConnection
    }

    /** 读 EventFeedingConnection 的私有字段 delegate（被装饰的生产连接）。 */
    private fun delegateOf(connection: Any): Any {
        val field = runCatching {
            connection.javaClass.getDeclaredField("delegate").apply { isAccessible = true }
        }.getOrNull() ?: error("EventFeedingConnection 必须有 delegate 字段")
        return field.get(connection)!!
    }

    /**
     * 沿注册链反查方法面桥实例持有的连接（实测形状，2026-10-16 探针 dump）：
     * `MethodChannel$IncomingMethodCallHandler.handler` → SAM lambda（捕获
     * `BridgeMethodCallHandler`，字段名 `arg$1`）→ 该 handler 又是 BridgeMethods
     * 的 SAM lambda（捕获 `BridgeMethods`，字段名同样叫 `arg$1`）→
     * `BridgeMethods.connection` = 壳传进去的连接实例。
     *
     * 【为什么要绕四层】壳不持有桥实例（install 返回值被丢弃），而「共享实例」
     * 这条纪律只能从已注册的 handler 反查。JDK lambda 的捕获字段名是
     * `arg$N`（编译器生成，不可控），故这里**按类型**取捕获值而不是按名字——
     * 任一层形状变了会返回 null，由调用方判失败，比硬编码类名更耐编译选项变化。
     */
    private fun methodFaceConnection(messenger: ShellMessenger): BridgeConnection? {
        val channelHandler = messenger.handlers[FlutterHostEngine.BRIDGE_METHOD_CHANNEL] ?: return null
        // 第一层：MethodChannel 的 handler 字段 = adapter 的 SAM lambda。
        val adapterLambda = privateField(channelHandler, "handler") ?: return null
        // 第二层：该 lambda 捕获的 BridgeMethodCallHandler = BridgeMethods 的 SAM lambda。
        val methodsLambda = capturedOfType(adapterLambda, dev.pocketagent.app.bridge.BridgeMethodCallHandler::class.java)
            ?: return null
        // 第三层：该 lambda 捕获的 BridgeMethods 实例。
        val methods = capturedOfType(methodsLambda, dev.pocketagent.app.bridge.BridgeMethods::class.java)
            ?: return null
        // 第四层：桥实例持有的连接（壳传进去的那个）。
        return privateField(methods, "connection") as? BridgeConnection
    }

    /**
     * 按**类型**取 JDK lambda 的捕获值（字段名是编译器生成的 `arg$N`）。
     *
     * 【为什么不按字段名】lambda 捕获字段名由编译器决定（`arg$1`/`arg$2`/...），
     * 换 JDK 或换捕获个数就会漂；按类型取与编译选项无关。JDK 模块外的反射
     * （java.util.HashMap.serialVersionUID 之类）用 runCatching 兜住——我们的
     * lambda 类都是 dev.pocketagent 自家编译产物，不受模块限制。
     */
    private fun capturedOfType(lambda: Any, type: Class<*>): Any? {
        for (field in lambda.javaClass.declaredFields) {
            val value = runCatching {
                field.isAccessible = true
                field.get(lambda)
            }.getOrNull() ?: continue
            if (type.isInstance(value)) return value
        }
        return null
    }
}
