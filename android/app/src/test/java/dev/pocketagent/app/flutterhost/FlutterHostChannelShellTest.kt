package dev.pocketagent.app.flutterhost

import io.flutter.plugin.common.BinaryMessenger
import io.flutter.plugin.common.FlutterException
import io.flutter.plugin.common.MethodCall
import io.flutter.plugin.common.MethodChannel
import io.flutter.plugin.common.StandardMethodCodec
import java.nio.ByteBuffer
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 通道壳的 JVM 单测:方法面一律 HOST_UNAVAILABLE、事件面静默。
 *
 * 【为什么这段能在纯 JVM 跑】[FlutterHostEngine.installChannelShell] 的入参是
 * [BinaryMessenger](接口)而非 FlutterEngine(具体类),故本测试用 [FakeMessenger]
 * 驱动**真的** MethodChannel/EventChannel 编解码(StandardMethodCodec 是纯 Java,
 * 不碰原生库)。测的是壳的行为契约,不是引擎。
 *
 * 【为什么值得单测而不是留给真机】这三件事的真机表现是「Dart 侧收到什么」,
 * 中间隔着 Dart 代码、引擎、平台线程;而它们全部由 Kotlin 侧几行决定:
 *  ① 方法面必须回 **error(HOST_UNAVAILABLE)**,不能 success(null)、不能
 *     notImplemented——后两者在 Dart 侧分别是「假成功」与「另一种
 *     HOST_UNAVAILABLE 路径」,语义不同(契约 §2.5);
 *  ② 事件面 listen/cancel 必须**不推流也不 error**,否则 Dart 的
 *     receiveBroadcastStream 会以 FlutterError 冒泡,把「没流」伪装成「流坏了」;
 *  ③ 三个事件通道都要挂上——漏一个,Dart 侧那条流静默无源且无任何报错。
 * 这三条在 JVM 里全可判定,没有理由推到真机。
 */
class FlutterHostChannelShellTest {

    /**
     * 记录 handler,并模拟平台线程的回包路径。
     *
     * 注意 BinaryMessenger 的嵌套类型写法:`BinaryMessenger.BinaryMessageHandler`
     * / `BinaryMessenger.BinaryReply`(它们是 BinaryMessenger 的内部接口,
     * 不是 io.flutter.plugin.common 的顶层类型)。
     */
    private class FakeMessenger : BinaryMessenger {
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

        /** 以 Dart 侧 invokeMethod 的等价物驱动:编成 envelope 交给 handler。 */
        fun invoke(channel: String, method: String, arguments: Any?): ByteBuffer? {
            val handler = handlers[channel] ?: return null
            var reply: ByteBuffer? = null
            handler.onMessage(encoded(MethodCall(method, arguments))) { r -> reply = r }
            return reply?.apply { flip() }
        }

        /** 以 Dart 侧 EventChannel 订阅的等价物驱动('listen' / 'cancel')。 */
        fun stream(channel: String, action: String, arguments: Any?): ByteBuffer? {
            val handler = handlers[channel] ?: return null
            var reply: ByteBuffer? = null
            handler.onMessage(encoded(MethodCall(action, arguments))) { r -> reply = r }
            return reply?.apply { flip() }
        }

        /**
         * 编一个方法调用,并把 buffer 的 position 归零。
         *
         * 【为什么必须 flip】StandardMethodCodec 的 encode* 返回的 ByteBuffer 是
         * 「写完、position 在末尾」的状态,而读方(decodeMethodCall/decodeEnvelope)
         * 从 position 开始读——不 flip 就是「从末尾读」,得到 "Message corrupted"
         * (入参)或 BufferUnderflowException(回包)。真实链路里这个 flip 由
         * BinaryMessenger/引擎做,测试自己驱动就得自己做。入参与回包都要 flip。
         */
        private fun encoded(call: MethodCall): ByteBuffer =
            StandardMethodCodec.INSTANCE.encodeMethodCall(call).apply { flip() }
    }

    /** 解出方法应答的 (code, message, details) 三元组(error envelope 形态)。 */
    private fun errorPartsOf(reply: ByteBuffer?): Triple<String?, String?, Any?> {
        val nonNull = assertNotNullAndReturn(reply)
        // decodeEnvelope 对 error envelope 的行为是**抛 FlutterException**
        // (它把 (code,message,details) 三元组还原成异常),不是返回 List——
        // 与「Dart 侧收到 PlatformException」的链路一致(StandardMethodCodec:129)。
        val ex = try {
            StandardMethodCodec.INSTANCE.decodeEnvelope(nonNull)
            return error("decodeEnvelope 对 error envelope 应抛 FlutterException,却正常返回了: $nonNull")
        } catch (e: FlutterException) {
            e
        }
        return Triple(ex.code, ex.message, ex.details)
    }

    /** assertNotNull 后把可空收成非空(Kotlin 的 smart cast 不跨函数边界)。 */
    private fun assertNotNullAndReturn(value: ByteBuffer?): ByteBuffer {
        assertNotNull("必须应答(不答 = Dart 侧挂起)", value)
        return value!!
    }

    // ---------------- 方法面:一律 HOST_UNAVAILABLE ----------------

    /**
     * 任意 method（故意挑一个**契约里根本不存在**的名字）都必须回
     * error(HOST_UNAVAILABLE),不得 success/notImplemented。
     *
     * 【为什么用不存在的方法名】壳不写死方法名单(红线),所以它的行为必须对
     * 「任何字符串」一致。挑契约外的名字测,才能证明「没有暗藏名单」——
     * 若壳里其实按名单分支,未知方法会走另一条路(通常 notImplemented)。
     */
    @Test
    fun `any method name gets HOST_UNAVAILABLE error`() {
        val messenger = FakeMessenger()
        FlutterHostEngine.installChannelShell(messenger)

        val reply = messenger.invoke(
            FlutterHostEngine.BRIDGE_METHOD_CHANNEL,
            "aMethodThatIsNotInTheContract",
            null,
        )
        val parts = errorPartsOf(reply)
        assertEquals("HOST_UNAVAILABLE", parts.first)
        assertNotNull(parts.second)
        // details 恒 null:壳没有可附的上下文(不编造)。
        assertNull(parts.third)
    }

    /**
     * 契约内的真实方法名同样回 HOST_UNAVAILABLE。
     *
     * 【为什么两个方法名分别测】上一条证明「未知方法不回退到 notImplemented」,
     * 这一条证明「已知方法也不会被伪装成已实现」。两条合起来才覆盖
     * 「一律占位」的两种可能漏洞。
     */
    @Test
    fun `contract method names also get HOST_UNAVAILABLE error`() {
        val messenger = FakeMessenger()
        FlutterHostEngine.installChannelShell(messenger)

        for (method in listOf("getHostInfo", "getConnectionState", "listThreads", "pairCode")) {
            val reply = messenger.invoke(FlutterHostEngine.BRIDGE_METHOD_CHANNEL, method, ByteArray(0))
            val parts = errorPartsOf(reply)
            assertEquals("$method 必须回 HOST_UNAVAILABLE", "HOST_UNAVAILABLE", parts.first)
        }
    }

    /**
     * 错误 message 必须点名「本波未接线」+ 交接波次。
     *
     * 【为什么断言 message 内容而不是只断言 code】Dart 侧 UI 按 code 分支
     * (重试/去配对/仅展示),但用户看到的字来自 message。message 说「未接线」
     * 用户知道等下一波;message 说「连接失败」用户会去查网线——那是误导。
     */
    @Test
    fun `host unavailable message names the batch and the handoff`() {
        val messenger = FakeMessenger()
        FlutterHostEngine.installChannelShell(messenger)

        val reply = messenger.invoke(FlutterHostEngine.BRIDGE_METHOD_CHANNEL, "getHostInfo", null)
        val parts = errorPartsOf(reply)
        val message = parts.second
        assertTrue("message 须说明本波未接线: $message", message!!.contains("未接线"))
        assertTrue("message 须点名交接波次 B4b: $message", message.contains("B4b"))
    }

    // ---------------- 事件面:静默 ----------------

    /**
     * 三个事件通道都必须挂 handler;listen 之后 cancel 不得推流、不得 error。
     *
     * 【为什么必须 listen 在 cancel 之前】EventChannel 的 onCancel 在**没有活动
     * 流**时会回 error envelope("No active stream to cancel")——那是引擎对
     * 「取消一个没订阅过的流」的防御,不是壳的问题。所以正确的订阅生命周期是
     * listen → cancel,测试按这个顺序驱动。
     *
     * 【为什么断言 ACK 即可】listen 的回包是 `encodeSuccessEnvelope(null)`
     * (订阅成功的 ACK,解出来是 null 而非 error envelope)。**真正判定推流的对象
     * 是传给 onListen 的 EventSink**——壳的 onListen 什么都不做,所以「没推流」
     * 由源码结构保证(壳不持有 sink 引用)。直接可断言的硬事实是:handler 在、
     * listen/cancel 不抛、ACK 不是 error envelope(回 error = Dart 侧
     * receiveBroadcastStream 报握手错)。
     */
    @Test
    fun `all three event channels accept listen and cancel without pushing`() {
        val messenger = FakeMessenger()
        FlutterHostEngine.installChannelShell(messenger)

        for (channel in listOf(
            FlutterHostEngine.BRIDGE_EVENT_CHANNEL,
            FlutterHostEngine.BRIDGE_TURN_EVENT_CHANNEL,
            FlutterHostEngine.BRIDGE_TERMINAL_EVENT_CHANNEL,
        )) {
            assertTrue("$channel 必须挂 handler", messenger.handlers.containsKey(channel))

            val listenAck = messenger.stream(channel, "listen", null)
            assertNotNull("$channel 的 listen 必须 ACK", listenAck)
            // listen 的 ACK 是 encodeSuccessEnvelope(null):decodeEnvelope 解出
            // **null**(成功、无载荷),不是 error envelope(那会抛 FlutterException)。
            // decodeEnvelope 的返回是平台类型 Any!,显式声明成 Any? 再判,
            // 免得 JUnit 的 assertNull 对平台类型做 String.valueOf 时踩 NPE。
            val ack: Any? = StandardMethodCodec.INSTANCE.decodeEnvelope(assertNotNullAndReturn(listenAck))
            assertNull("$channel 的 listen ACK 应为空成功包,实际: $ack", ack)

            // 先 listen 再 cancel:无活动流时 cancel 会被引擎判 error(见 KDoc)。
            val cancelAck = messenger.stream(channel, "cancel", null)
            assertNotNull("$channel 的 cancel 必须 ACK", cancelAck)
            val cancelDecoded: Any? = StandardMethodCodec.INSTANCE.decodeEnvelope(
                assertNotNullAndReturn(cancelAck),
            )
            assertNull("$channel 的 cancel ACK 应为空成功包,实际: $cancelDecoded", cancelDecoded)
        }
    }

    /**
     * 未注册的通道名不得被壳顺手挂上(防止壳「图省事」挂通配)。
     *
     * 挂通配会让 Dart 侧任何 EventChannel 都「有 handler」,把未接线面伪装成
     * 已接线面——与「诚实缺席」直接冲突。
     */
    @Test
    fun `shell does not register handlers on unrelated channels`() {
        val messenger = FakeMessenger()
        FlutterHostEngine.installChannelShell(messenger)

        assertTrue(
            "壳只应挂 4 个通道(1 方法 + 3 事件),实际: ${messenger.handlers.keys}",
            messenger.handlers.size == 4,
        )
        assertTrue(messenger.handlers.containsKey("dev.pocketagent/bridge"))
        assertTrue(!messenger.handlers.containsKey("dev.pocketagent/bridge/unknown"))
        assertTrue(!messenger.handlers.containsKey("flutter/platform"))
    }

    // ---------------- 注册是「幂等覆盖」而非「叠加」 ----------------

    /**
     * 重复挂壳不得叠加 handler 数量(后一次覆盖前一次)。
     *
     * 【为什么这条重要】若实现用「追加」语义,重挂会在同一个 messenger 上叠多个
     * handler(或让 EventChannel 的双重 onListen 行为不可预期)。壳是「配置」,
     * 配置就该可重入。真实场景:引擎复用 + 配置变更后重挂。
     */
    @Test
    fun `installing the shell twice does not stack handlers`() {
        val messenger = FakeMessenger()
        FlutterHostEngine.installChannelShell(messenger)
        val afterFirst = messenger.handlers.size
        FlutterHostEngine.installChannelShell(messenger)
        assertEquals("重复挂壳不得叠加 handler", afterFirst, messenger.handlers.size)
    }

    // ---------------- 通道真的建起来了 ----------------

    /**
     * 让「MethodChannel/EventChannel 真被构造过」这件事可验证(防将来误改成 no-op)。
     *
     * EventChannel 对未知 action(既非 listen 也非 cancel)回 **null**(不是空
     * envelope)——`reply.reply(null)`。这正是「通道建了、但没有这个 action」的
     * 固有行为,反过来证明 handler 在。注意别把 null 当成「没注册」:没注册时
     * FakeMessenger.invoke/stream 也会返回 null,两者靠
     * `messenger.handlers` 的键是否存在来区分(由上面几条断言覆盖)。
     */
    @Test
    fun `channels are really registered not stubbed out`() {
        val messenger = FakeMessenger()
        FlutterHostEngine.installChannelShell(messenger)

        val methodReply = messenger.invoke(FlutterHostEngine.BRIDGE_METHOD_CHANNEL, "ping", null)
        assertNotNull("方法通道必须应答", methodReply)

        // 未知 action:EventChannel 回 null(上面 KDoc 说明为什么这是「已注册」的证据)。
        val bogus = messenger.stream(FlutterHostEngine.BRIDGE_EVENT_CHANNEL, "notListenOrCancel", null)
        assertTrue(
            "EventChannel 对未知 action 应回 null 而非空 envelope(回空 envelope 说明" +
                "handler 不是 EventChannel 自己的)",
            bogus == null,
        )
    }
}
