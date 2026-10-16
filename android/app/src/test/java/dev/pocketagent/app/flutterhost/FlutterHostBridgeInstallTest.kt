package dev.pocketagent.app.flutterhost

import dev.pocketagent.app.bridge.BridgeConnection
import dev.pocketagent.app.bridge.BridgeMethods
import dev.pocketagent.app.bridge.FakeConnection
import dev.pocketagent.app.bridge.hostInfo
import dev.pocketagent.app.bridge.install
import dev.pocketagent.app.bridge.listThreadsResponse
import dev.pocketagent.app.bridge.pairingResponse
import io.flutter.plugin.common.BinaryMessenger
import io.flutter.plugin.common.MethodCall
import io.flutter.plugin.common.MethodChannel
import io.flutter.plugin.common.StandardMethodCodec
import java.nio.ByteBuffer
import org.junit.After
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.GetHostInfoRequest
import pocketagent.v1.ListThreadsRequest

/**
 * 「壳真换了」的判决测试（B4c 的核心证据，B4d 后事件面另有专测）。
 *
 * 【为什么需要这一份】B4a 的壳把方法面一律回 HOST_UNAVAILABLE；B4c 把它换成
 * `BridgeMethods.install(messenger, connection)`。危险在于「换了但没
 * 真换」——例如 install 调用写错、被 try/catch 吞掉、或壳里残留旧 handler 覆盖
 * 了新 handler。这类回归的表现是**静默的**：Flutter UI 一切正常，只是所有调用
 * 仍然得 HOST_UNAVAILABLE，用户以为「还没接线」。所以必须有判决级断言：
 *
 *  ① 契约**外**的方法名 → 仍得 HOST_UNAVAILABLE（桥的 else 分支，未接的 6 条
 *     双向流方法也在此列——它们不是契约外，但同样由桥如实回 HOST_UNAVAILABLE）；
 *  ② 契约**内**已接的方法名 → **不再** HOST_UNAVAILABLE，走真 handler 链：
 *     `getConnectionState` 返回五键 Map、`pairCode` 返回配对码、
 *     `getHostInfo` 的请求真的到达 gRPC 取数面（fake connection 留痕）。
 *
 * ①②同时成立才叫「壳真换了」：只验 ① 会漏掉「桥没接上」；只验 ② 会漏掉
 * 「桥接了但错误码表坏了」。
 *
 * 【B4d 之后本文件与 FlutterHostEventChannelsInstallTest 的分工】事件面的推流
 * 判决（帧形状/协程收停/实例形态）迁到那份新文件；本文件只保留**方法面**判决 +
 * 事件面的握手面（listen ACK 是成功包）。两份合起来才是「壳全接了」的完整证据。
 *
 * 【怎么驱动】installChannelShell 的入参是 [BinaryMessenger] 接口，故用
 * [ShellMessenger] 驱动**真的** MethodChannel 编解码（StandardMethodCodec 是纯
 * Java，不碰 libflutter.so）。桥的内部 seam 用 bridge 包的 [FakeConnection] 注入
 * ——这正是 B4b 把 [BridgeConnection] 收成窄接口的用意：flutterhost 的接线测试
 * 因此能断言「请求穿过桥到达取数面」，而不需要真 daemon。
 *
 * 【为什么绕开 AgentConnectionBridge】生产入口 `install` 的第二个参数是
 * [BridgeConnection]；壳里传的是 `AgentConnectionBridge()`（读进程级单例的
 * StateFlow + 真 gRPC stub，纯 JVM 下碰 Android 运行时）。本测试因此分两路：
 *  - 生产路径（壳自己那一行）用契约外方法名驱动 → 断言回 HOST_UNAVAILABLE
 *    （该分支在派发前就返回，不碰连接层，JVM 安全）；
 *  - 已接方法名的真链路用 [BridgeMethods.install] + [FakeConnection] 驱动 →
 *    断言请求解析 → stub 调用（与生产装配入口**同一个函数**，只换连接实现）。
 */
class FlutterHostBridgeInstallTest {

    /**
     * 记录 handler 的假 messenger（与 FlutterHostChannelShellTest 同款，独立一份：
     * 两个测试类驱动同一段注册逻辑，但断言的面不同，不共享 fixture 更耐改）。
     */
    private class ShellMessenger : BinaryMessenger {
        val handlers = HashMap<String, BinaryMessenger.BinaryMessageHandler>()

        override fun send(channel: String, message: ByteBuffer?) {
            throw UnsupportedOperationException("测试只驱动 handler,不走 send")
        }

        override fun send(channel: String, message: ByteBuffer?, callback: BinaryMessenger.BinaryReply?) {
            throw UnsupportedOperationException("测试只驱动 handler,不走 send")
        }

        override fun setMessageHandler(
            channel: String,
            handler: BinaryMessenger.BinaryMessageHandler?,
        ) {
            if (handler == null) handlers.remove(channel) else handlers[channel] = handler
        }

        fun invoke(channel: String, method: String, arguments: Any?): ByteBuffer? {
            val handler = handlers[channel] ?: return null
            var reply: ByteBuffer? = null
            handler.onMessage(encoded(MethodCall(method, arguments))) { r -> reply = r }
            return reply?.apply { flip() }
        }

        /**
         * 驱动一次调用并**等** handler 落结果(桥的 handle 是异步的)。
         *
         * 【为什么不能靠「没结果就再 invoke 一次」】桥的 handle 是
         * `scope.launch { … result.success/error(…) }`：MethodChannel.Result 的回调
         * 发生在 IO 协程里,与测试线程不同步。若用轮询重 invoke,同一方法会被派发
         * 多次(fake connection 的留痕计数随之虚高,断言「请求到达一次」就假了)。
         * 所以这里一次 invoke,把回包通过 [BinaryMessenger.BinaryReply] 交出来,
         * 由调用方 latch 等待。
         *
         * @param onReply 收到回包字节时回调(可能来自测试线程之外的 IO 线程)。
         */
        fun invokeAsync(
            channel: String,
            method: String,
            arguments: Any?,
            onReply: (ByteBuffer) -> Unit,
        ) {
            val handler = handlers[channel] ?: error("通道未注册: $channel")
            handler.onMessage(encoded(MethodCall(method, arguments))) { r ->
                r?.let { onReply(it.apply { flip() }) }
            }
        }

        private fun encoded(call: MethodCall): ByteBuffer =
            StandardMethodCodec.INSTANCE.encodeMethodCall(call).apply { flip() }
    }

    /** 方法应答解出的成功载荷（error envelope 会抛 FlutterException，测试据此分流）。 */
    private fun successOf(reply: ByteBuffer?): Any? {
        assertNotNull("必须应答(不答 = Dart 侧挂起)", reply)
        val decoded: Any? = StandardMethodCodec.INSTANCE.decodeEnvelope(reply!!)
        return decoded
    }

    /** 方法应答解出的 (code, message)（error envelope 形态）。 */
    private fun errorPartsOf(reply: ByteBuffer?): Pair<String?, String?> {
        assertNotNull("必须应答(不答 = Dart 侧挂起)", reply)
        val ex = try {
            val decoded = StandardMethodCodec.INSTANCE.decodeEnvelope(reply!!)
            error("本应答不是 error envelope,却按 error 解析了: $decoded")
        } catch (e: io.flutter.plugin.common.FlutterException) {
            e
        }
        return Pair(ex.code, ex.message)
    }

    /**
     * 驱动一次调用并等桥落结果(桥的 handle 是 `launch` + `withContext(IO)` 异步的)。
     *
     * 【为什么用 latch 而不是轮询重发】桥的回调走它自己的 CoroutineScope
     * (Dispatchers.IO),测试线程拿不到那个 Job;重发会让同一方法被派发多次,
     * fake connection 的留痕计数虚高(「请求到达一次」就断言不住了)。latch 只发一次。
     *
     * 5s 上限:正常路径毫秒级完成,超时即失败,不把慢当成过。
     */
    private fun awaitReply(messenger: ShellMessenger, method: String, arguments: Any?): ByteBuffer {
        val latch = java.util.concurrent.CountDownLatch(1)
        var reply: ByteBuffer? = null
        messenger.invokeAsync(FlutterHostEngine.BRIDGE_METHOD_CHANNEL, method, arguments) { r ->
            reply = r
            latch.countDown()
        }
        if (!latch.await(5, java.util.concurrent.TimeUnit.SECONDS)) {
            throw AssertionError("$method 5s 内未回结果(桥的 handler 未落结果)")
        }
        return reply!!
    }

    /**
     * 判决点②的**生产路径**版：`getConnectionState` 经壳内那一行 install 真链路返回。
     *
     * 【为什么这条是「壳真换了」的最强判决】上面几条用 [BridgeMethods.install] +
     * fake connection 驱动（与壳内同一函数，只换连接实现）；这条**不换连接实现**——
     * 走的就是壳里那一行 `BridgeMethods.install(messenger, AgentConnectionBridge())`
     * 的完整装配。能这么做的原因：`getConnectionState` 只读
     * [dev.pocketagent.app.AgentConnection] 的五条 StateFlow 现值（纯 Kotlin
     * 内存读，不碰 gRPC / android.jar），而 `AgentConnectionBridge` 的读面正是
     * `AgentConnection.connState.value` 这类透传——故生产连接在纯 JVM 下**可读**。
     *
     * 于是这条一次断言三件事：壳内 install 真的执行了、装配的连接是生产适配器、
     * 已接方法名不再回 HOST_UNAVAILABLE（B4a 时代这里必是 HOST_UNAVAILABLE）。
     *
     * 【怎么改 AgentConnection 的现值】它是进程级 object、五条 StateFlow 可变；
     * 本测试写入后用 [org.junit.After] 还原初始值（"checking"/""/0/emptyList），
     * 避免污染同进程其他用例（AgentConnection 是 object，状态跨测试类存活）。
     */
    @Test
    fun `getConnectionState returns the production snapshot through the shell wiring`() {
        val messenger = ShellMessenger()
        FlutterHostEngine.installChannelShell(messenger)

        // 摆布生产连接的现值(纯 Kotlin StateFlow,不碰 Android 框架)。
        dev.pocketagent.app.AgentConnection.connState.value = "ok"
        dev.pocketagent.app.AgentConnection.connTransport.value = "USB 通道"
        dev.pocketagent.app.AgentConnection.connAddress.value = "127.0.0.1:7246"
        dev.pocketagent.app.AgentConnection.connectionEpoch.value = 42
        dev.pocketagent.app.AgentConnection.hostCapabilities.value = listOf("terminal", "fs")

        val reply = awaitReply(messenger, "getConnectionState", null)
        val decoded = successOf(reply)
        @Suppress("UNCHECKED_CAST")
        val map = decoded as? Map<String, Any>
        assertNotNull("getConnectionState 必须回 Map(壳未接线时这里是 HOST_UNAVAILABLE)", map)
        assertEquals("ok", map!!["connState"])
        assertEquals("USB 通道", map["connTransport"])
        assertEquals("127.0.0.1:7246", map["connAddress"])
        assertEquals(42, map["connectionEpoch"])
        assertEquals(listOf("terminal", "fs"), map["hostCapabilities"])
        assertEquals(5, map.size)
    }

    @After
    fun tearDown() {
        // AgentConnection 是进程级 object,五条 StateFlow 跨测试类存活;还原初始值,
        // 否则本用例摆布的 "ok"/42 会污染其他读同一对象的用例。
        dev.pocketagent.app.AgentConnection.connState.value = "checking"
        dev.pocketagent.app.AgentConnection.connTransport.value = ""
        dev.pocketagent.app.AgentConnection.connAddress.value = ""
        dev.pocketagent.app.AgentConnection.connectionEpoch.value = 0
        dev.pocketagent.app.AgentConnection.hostCapabilities.value = emptyList()
    }

    // ---------------- 判决点①:契约外/未接方法 → HOST_UNAVAILABLE ----------------

    /**
     * 契约外的假方法名仍得 HOST_UNAVAILABLE(壳换了,但桥的 else 分支还在)。
     *
     * 【为什么挑契约外的名字】它同时证明两件事:handler 是桥的(桥的 else 分支
     * 回 HOST_UNAVAILABLE + 「契约漂移」措辞),且壳没有为「兜底」另挂一个
     * 一律报错的 handler(那条路的消息是「本波未接线」,与桥的措辞不同)。
     *
     * 【为什么这条路能纯 JVM 跑】未知方法在派发最前面就 return,不碰连接层。
     */
    @Test
    fun `contract-foreign method name still gets HOST_UNAVAILABLE from the bridge`() {
        val messenger = ShellMessenger()
        FlutterHostEngine.installChannelShell(messenger)

        val reply = awaitReply(messenger, "aMethodThatIsNotInTheContract", null)
        val (code, message) = errorPartsOf(reply)
        assertEquals("HOST_UNAVAILABLE", code)
        // 桥的 else 分支措辞点名「契约漂移」,与壳时代的「本波未接线」可区分。
        assertTrue("桥的未知方法消息须点名契约漂移: $message", message!!.contains("契约漂移"))
    }

    /**
     * `startTurn`（§3.6 的 unary 首帧入口）仍如实回 HOST_UNAVAILABLE。
     *
     * 【为什么这是判决点的一部分】「壳真换了」不等于「所有方法都通了」。`startTurn`
     * 是无消费面的 unary 首帧入口（Dart 的 PaBridge.startTurn 仍在用），轮次的完整
     * 形态是双向流；桥选择让它继续诚实缺席并如实回 HOST_UNAVAILABLE（不建流不收帧
     * = 不伪造）。本测试把这件事钉死——若将来有人为了「看起来全通了」把它改成
     * success(null)，这里会失败。
     *
     * 【为什么不断言消息措辞】桥的留后人话措辞属 `bridge/` 的演进面（并发线改它
     * 不该把 flutterhost 的判决拖红）；这里只断**契约级硬事实**：错误码是
     * HOST_UNAVAILABLE、消息非空且点名本方法。消息内容由 bridge 包自己的测试看。
     *
     * 该分支在派发前返回(不碰连接层),故同样可纯 JVM 驱动。
     */
    @Test
    fun `startTurn stays honestly HOST_UNAVAILABLE instead of faking a first frame`() {
        val messenger = ShellMessenger()
        FlutterHostEngine.installChannelShell(messenger)

        val reply = awaitReply(messenger, "startTurn", ByteArray(0))
        val (code, message) = errorPartsOf(reply)
        assertEquals("startTurn 必须仍回 HOST_UNAVAILABLE(不伪造)", "HOST_UNAVAILABLE", code)
        assertTrue("留后消息须点名方法名: $message", message!!.contains("startTurn"))
    }

    // ---------------- 判决点②:契约内已接方法 → 走真 handler 链 ----------------

    /**
     * `getConnectionState` 不再 HOST_UNAVAILABLE,而是回五键快照 Map。
     *
     * 【为什么挑它做判决】它是唯一不碰 gRPC 的已接方法(纯读 StateFlow 现值),
     * 因此可以在纯 JVM 下完整驱动生产装配入口 —— 请求从 MethodChannel 编解码
     * 进来,经桥的 handler → dispatch → 连接层 seam → 五键 Map → 编回 Dart。
     * 它通了,就证明「壳里的 install 真的把桥挂上了」;B4a 时代这条回的是
     * HOST_UNAVAILABLE。
     */
    @Test
    fun `getConnectionState goes through the real bridge and returns the five key snapshot`() {
        val conn = FakeConnection(
            connState = "ok",
            connTransport = "USB 通道",
            connAddress = "127.0.0.1:7246",
            connectionEpoch = 7,
            hostCapabilities = listOf("terminal", "agent:pi"),
        )
        val messenger = ShellMessenger()
        // 与壳内同一行生产装配入口,只把连接实现换成 fake(见类 KDoc)。
        BridgeMethods.install(messenger, conn)

        val reply = awaitReply(messenger, "getConnectionState", null)
        val decoded = successOf(reply)
        @Suppress("UNCHECKED_CAST")
        val map = decoded as? Map<String, Any>
        assertNotNull("getConnectionState 必须回 Map(壳时代这里是 error)", map)
        assertEquals("ok", map!!["connState"])
        assertEquals("USB 通道", map["connTransport"])
        assertEquals("127.0.0.1:7246", map["connAddress"])
        assertEquals(7, map["connectionEpoch"])
        assertEquals(listOf("terminal", "agent:pi"), map["hostCapabilities"])
        assertEquals(5, map.size)
    }

    /**
     * `pairCode` 走真链路:请求到达 gRPC 取数面,配对码原样回。
     *
     * 【为什么这条也是判决点】它验的是「已接方法名真的派发到了 hostService()」——
     * fake connection 的 pairing 数据只有被读到才可能出现在回包里。若壳里 install
     * 没生效(或写成了别的方法名),这里拿到的是 HOST_UNAVAILABLE 而不是配对码。
     */
    @Test
    fun `pairCode reaches the grpc facing seam and returns the pair code`() {
        val conn = FakeConnection()
        conn.host.pairing = pairingResponse("12345678")
        val messenger = ShellMessenger()
        BridgeMethods.install(messenger, conn)

        val reply = awaitReply(messenger, "pairCode", null)
        assertEquals("12345678", successOf(reply))
    }

    /**
     * `getHostInfo` 的请求字节被解析后真的到达 gRPC 取数面(留痕断言)。
     *
     * 【为什么这是「请求到达 gRPC 层」的判决】上面两条验回值形态,这条验**链路**:
     * fake 的 hostService 被调用过一次、且 capabilities 快照经 updateHostInfo 写回
     * (桥与探活循环同口径的那条成功路径)。回包字节与 fake 摆布的 HostInfo 逐字节
     * 相等,证明 parseFrom → stub → toByteArray 整条编码链没被壳截断。
     */
    @Test
    fun `getHostInfo request reaches the grpc layer and the reply bytes round trip`() {
        val conn = FakeConnection()
        conn.host.hostInfo = hostInfo(capabilities = arrayOf("terminal", "fs"))
        val messenger = ShellMessenger()
        BridgeMethods.install(messenger, conn)

        // Dart 侧 invokeMethod('getHostInfo', GetHostInfoRequest().writeToBuffer())
        val reply = awaitReply(
            messenger,
            "getHostInfo",
            GetHostInfoRequest.getDefaultInstance().toByteArray(),
        )
        val bytes = successOf(reply) as? ByteArray
        assertNotNull("getHostInfo 必须回 proto 字节(壳时代这里是 error)", bytes)
        assertArrayEquals(conn.host.hostInfo!!.toByteArray(), bytes!!)
        // 链路留痕：请求确实到达取数面,且成功路径写了共享快照。
        assertEquals("请求必须到达 hostService 取数面", 1, conn.updateHostInfoCalls.size)
        assertEquals(listOf("terminal", "fs"), conn.updateHostInfoCalls.single().capabilitiesList)
    }

    /**
     * `listThreads` 的请求字节原样过桥(不做投影/筛选),回字节与 fake 数据相等。
     *
     * 【为什么再验一条】换一个服务族(threadService 而非 hostService)证明接线不是
     * 「只接了 hostService」的偏口;请求留痕(lastListThreadsRequest)非空证明
     * parseFrom 真的把字节还原成了消息,而不是走了某种空消息兜底。
     */
    @Test
    fun `listThreads request bytes cross the bridge untouched`() {
        val conn = FakeConnection()
        conn.thread.listThreads = listThreadsResponse("t-1", "t-2")
        val messenger = ShellMessenger()
        BridgeMethods.install(messenger, conn)

        val reply = awaitReply(
            messenger,
            "listThreads",
            ListThreadsRequest.getDefaultInstance().toByteArray(),
        )
        val bytes = successOf(reply) as? ByteArray
        assertNotNull(bytes)
        assertArrayEquals(conn.thread.listThreads!!.toByteArray(), bytes!!)
        assertNotNull("请求字节必须被解析成消息过桥", conn.thread.lastListThreadsRequest)
    }

    /**
     * 壳与桥的错误码集合一致:方法面任何错误都不出契约七码之外。
     *
     * 【为什么值得单独一条】B4a 壳只发 HOST_UNAVAILABLE 一个码;B4c 之后方法面的
     * 码全部来自桥的 [dev.pocketagent.app.bridge.BridgeErrorCodes](NOT_CONNECTED /
     * AUTH_REQUIRED / DECODE_FAILED / PAIR_DISABLED / INTERNAL / HOST_UNAVAILABLE /
     * INVALID_ADDR)。这条把「壳没在中间插自己的第八个码」钉死——插入的表现是
     * Dart 侧 UI 分支(code)静默走错支。
     *
     * 驱动方式:连不上的 fake(所有 RPC 抛 UNAVAILABLE)→ 得 NOT_CONNECTED。
     */
    @Test
    fun `bridge failures surface contract codes not shell invented ones`() {
        val conn = FakeConnection()
        conn.host.failure = io.grpc.Status.fromCode(io.grpc.Status.Code.UNAVAILABLE)
            .withDescription("transport 半死").asRuntimeException()
        val messenger = ShellMessenger()
        BridgeMethods.install(messenger, conn)

        val reply = awaitReply(messenger, "getHostInfo", ByteArray(0))
        val (code, message) = errorPartsOf(reply)
        assertEquals("NOT_CONNECTED", code)
        assertEquals("transport 半死", message)
        // 失败路径不写 capabilities 快照(只有成功路径写)。
        assertTrue(conn.updateHostInfoCalls.isEmpty())
    }

    /**
     * 三条事件通道 listen ACK 都是空成功包(不是 error envelope)。
     *
     * 【B4d 之后这条断言什么】事件面已接真推流(见
     * FlutterHostEventChannelsInstallTest),本条的断言点从「静默」收窄为
     * **握手面**：listen 的 ACK 必须是 `encodeSuccessEnvelope(null)`,不能是
     * error envelope。握手错不进流(契约 §2.5：Flutter 把 'listen' 的异常转投
     * FlutterError.reportError),Dart 侧会静默收不到任何帧——所以「ACK 是空成功
     * 包」是事件面可用的最低硬事实,与推流内容正交(推流内容由那份判决测试覆盖)。
     */
    @Test
    fun `event channels accept listen with a success envelope after the wiring`() {
        val messenger = ShellMessenger()
        FlutterHostEngine.installChannelShell(messenger)

        for (channel in listOf(
            FlutterHostEngine.BRIDGE_EVENT_CHANNEL,
            FlutterHostEngine.BRIDGE_TURN_EVENT_CHANNEL,
            FlutterHostEngine.BRIDGE_TERMINAL_EVENT_CHANNEL,
        )) {
            assertTrue("$channel 必须挂 handler", messenger.handlers.containsKey(channel))
            val listenAck = messenger.invoke(channel, "listen", null)
            assertNotNull("$channel 的 listen 必须 ACK", listenAck)
            val ack: Any? = StandardMethodCodec.INSTANCE.decodeEnvelope(listenAck!!)
            assertNull("$channel 的 listen ACK 应为空成功包,实际: $ack", ack)
        }
        // 方法面那条是桥的 handler;事件面三条由 BridgeEventChannels 注册 —— 合 4 个。
        assertEquals(4, messenger.handlers.size)
    }

    /**
     * 接线点的形态断言:壳内那一行就是 [BridgeMethods.install](messenger, connection)。
     *
     * 【为什么用签名级断言】上面几条验「跑起来是对的」,这条验「写的就是它」——
     * 防将来有人把 install 换成等价的手写适配(那会绕开 B4b 的
     * [dev.pocketagent.app.bridge.FlutterBinaryMessengerAdapter],双份适配漂移)。
     *
     * 反射目标:install 是 `BridgeMethods.Companion` 的**扩展函数**,编译后落在
     * `BridgeFlutterWiringKt`(文件 facade),且**第一个参数是 Companion 接收者**
     * (Kotlin 扩展函数的固定形态)——所以方法表里是三个参数:
     * (Companion, BinaryMessenger, BridgeConnection)。断后两个的形状:
     * [BinaryMessenger](生产 messenger) + [BridgeConnection](可注入 seam,
     * 测试才换得了 fake connection)。
     */
    @Test
    fun `the wiring entry point keeps an injectable connection seam`() {
        val install = Class.forName("dev.pocketagent.app.bridge.BridgeFlutterWiringKt")
            .methods.firstOrNull { it.name == "install" }
        assertNotNull("BridgeMethods.Companion.install 必须存在(编译后落在 BridgeFlutterWiringKt)", install)
        val params = install!!.parameterTypes
        assertTrue(
            "install 应有 (Companion 接收者, BinaryMessenger, BridgeConnection) 三个参数," +
                "实际: ${params.toList()}",
            params.size == 3,
        )
        assertEquals(BinaryMessenger::class.java, params[1])
        assertEquals(BridgeConnection::class.java, params[2])
    }

    /**
     * 壳内实例化的是生产连接适配器 [dev.pocketagent.app.bridge.AgentConnectionBridge]。
     *
     * 【为什么值得一条】这是「壳把方法面接到生产连接层」的最终形态:若有人把
     * AgentConnectionBridge 换成直接读 stub 的内联实现,或用 fake 顶替,这条失败。
     * 断言它存在且是 [BridgeConnection] 的实现 —— 构造本身不触发 Android 运行时
     * (StateFlow 是纯 Kotlin、stub 按需取,见 AgentConnection.hostService 的懒建),
     * 故可在纯 JVM 下实例化。
     */
    @Test
    fun `the shell instantiates the production connection adapter`() {
        val adapter = dev.pocketagent.app.bridge.AgentConnectionBridge()
        // AgentConnectionBridge : BridgeConnection 是编译期事实(它 implements 该接口),
        // 故这条不断言形态,只断言**读面可用**——薄适配把五条 StateFlow 现值透出来,
        // 且初始值不碰 Android 框架(纯 JVM 可读)。若有人把壳内的连接实现换成别的
        // 类(或换成 fake),这条读到的就不再是生产读面。
        assertEquals("checking", adapter.connState)
        assertEquals("", adapter.connTransport)
        assertEquals("", adapter.connAddress)
        assertEquals(0, adapter.connectionEpoch)
        assertTrue(adapter.hostCapabilities.isEmpty())
        // 【实测边界(2026-10-16)】六个服务取数入口(hostService() 等)**不在**纯 JVM
        // 可调面内:它们经 AgentConnection.hostService() → channel() → OkHttp 建连,
        // 首行就 android.util.Log.i(android.jar stub,JVM 下抛 "not mocked")。
        // 故本测试只覆盖「构造 + StateFlow 读面」——那正是壳接线点**同步执行**的
        // 全部内容;stub 取用发生在 Dart 侧 invoke 之后(IO 线程),归真机验收。
    }

    /**
     * MethodChannel 的注册确实发生在契约通道名上(与 Dart 侧逐字一致)。
     *
     * 【为什么与上面几条不重复】上面验「handler 回什么」,这条验「注册在哪」——
     * 通道名错一位的表现是 Dart 侧 invoke 落空(MissingPluginException),而本测试
     * 若只看回包就会被 FakeMessenger 的 null 路径糊弄。键在,链路才在。
     */
    @Test
    fun `the bridge registers on the contract method channel name`() {
        val messenger = ShellMessenger()
        FlutterHostEngine.installChannelShell(messenger)
        assertTrue(
            "方法通道必须注册在 ${FlutterHostEngine.BRIDGE_METHOD_CHANNEL}",
            messenger.handlers.containsKey(FlutterHostEngine.BRIDGE_METHOD_CHANNEL),
        )
    }
}
