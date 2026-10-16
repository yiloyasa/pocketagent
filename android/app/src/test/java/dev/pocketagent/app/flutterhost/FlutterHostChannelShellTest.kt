package dev.pocketagent.app.flutterhost

import dev.pocketagent.app.bridge.BridgeErrorCodes
import io.flutter.plugin.common.BinaryMessenger
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
 * 通道壳的 JVM 单测：方法面已接真桥（B4c），事件面仍静默。
 *
 * 【为什么这段能在纯 JVM 跑】[FlutterHostEngine.installChannelShell] 的入参是
 * [BinaryMessenger](接口)而非 FlutterEngine(具体类),故本测试用 [FakeMessenger]
 * 驱动**真的** MethodChannel/EventChannel 编解码(StandardMethodCodec 是纯 Java,
 * 不碰原生库)。测的是壳的行为契约,不是引擎。
 *
 * 【B4c 之后本文件测什么】方法面不再由壳自己回 HOST_UNAVAILABLE——壳调用
 * `BridgeMethods.install(messenger, AgentConnectionBridge())`（B4b 成品），
 * 因此「壳真换了」的判决证据在 `FlutterHostBridgeInstallTest`（契约外方法名仍
 * HOST_UNAVAILABLE、契约内方法名走真 handler 链）。本文件退守壳**仍然负责**的
 * 两件事：事件面静默、注册面（只注册 4 个通道、不挂通配、可重入）。
 *
 * 【为什么值得单测而不是留给真机】这三件事的真机表现是「Dart 侧收到什么」,
 * 中间隔着 Dart 代码、引擎、平台线程;而它们全部由 Kotlin 侧几行决定:
 *  ① 事件面 listen/cancel 必须**不推流也不 error**,否则 Dart 的
 *     receiveBroadcastStream 会以 FlutterError 冒泡,把「没流」伪装成「流坏了」;
 *  ② 三个事件通道都要挂上——漏一个,Dart 侧那条流静默无源且无任何报错;
 *  ③ 未注册的通道名不得被顺手挂上(防壳「图省事」挂通配,把未接线面伪装成已接线)。
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

        /**
         * 驱动一次调用并把回包交给 [onReply](桥的 handle 是异步的,回包来自 IO 线程)。
         *
         * 【为什么需要这条】B4c 起方法面 handler 由桥注册,而桥的 handle 是
         * `launch { … result.success/error(…) }`——同步 invoke 拿到的是 null
         * (Result 还没被回调)。故这条把回包通过 BinaryReply 交出来,由调用方 latch 等。
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

        /**
         * 以 Dart 侧 EventChannel 订阅的等价物驱动('listen' / 'cancel')。
         *
         * 事件面是壳的静默 handler(同步 ACK),与桥的异步方法面不同,直接取回包。
         */
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
        } catch (e: io.flutter.plugin.common.FlutterException) {
            e
        }
        return Triple(ex.code, ex.message, ex.details)
    }

    /** assertNotNull 后把可空收成非空(Kotlin 的 smart cast 不跨函数边界)。 */
    private fun assertNotNullAndReturn(value: ByteBuffer?): ByteBuffer {
        assertNotNull("必须应答(不答 = Dart 侧挂起)", value)
        return value!!
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
     *
     * 【B4c 之后这条为什么还在】方法面已接桥,事件面仍是静默壳(B4d 才接线)。
     * 这条把「本波没顺手把事件面也接上/推假帧」钉住:壳若哪天提前推帧,
     * 本测试与 FlutterHostBridgeInstallTest 的推流断言会一起失败。
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
     *
     * 【B4c 之后这条为什么还在】方法面接了桥(注册 1 个方法通道),事件面三条
     * 仍是壳自己挂的静默 handler;合起来 4 个,一个不多。这条同时钉住
     * 「壳没有替 B4d 预挂别的通道名」。
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

        // 未知 action:EventChannel 回 null(上面 KDoc 说明为什么这是「已注册」的证据)。
        val bogus = messenger.stream(FlutterHostEngine.BRIDGE_EVENT_CHANNEL, "notListenOrCancel", null)
        assertTrue(
            "EventChannel 对未知 action 应回 null 而非空 envelope(回空 envelope 说明" +
                "handler 不是 EventChannel 自己的)",
            bogus == null,
        )
    }

    /**
     * 方法面应答由桥的 handler 给出,壳不再自己回 HOST_UNAVAILABLE。
     *
     * 【为什么这条在这里而不只在 FlutterHostBridgeInstallTest】那条测「桥真接了」
     * (用 fake connection 走真 handler 链);这条从**壳的注册面**钉死同一件事:
     * 注册进 MethodChannel 的 handler 回的错误码来自桥的契约码集合
     * ([dev.pocketagent.app.bridge.BridgeErrorCodes]),而不是壳写死的字面量——
     * B4a 时代壳回的消息是「本波(B4a)未接线」,那种措辞如今必须绝迹。
     *
     * 【为什么不断具体码值】壳内那一行用生产连接 [dev.pocketagent.app.bridge.
     * AgentConnectionBridge],JVM 下 stub 取不到真通道(AgentConnection.channel()
     * 碰 OkHttp/TLS),回的是桥的兜底码 INTERNAL——这是**环境**决定的,不是接线
     * 对错的判据(故那条判决放在注入 fake connection 的 FlutterHostBridgeInstallTest
     * 里)。这里只断言两件与具体码无关的事实:
     *  ① 应答来自契约码表(七码之一,而非壳时代的 HOST_UNAVAILABLE);
     *  ② 消息不再是壳时代的占位措辞「未接线 / B4a」。
     */
    @Test
    fun `method face replies come from the bridge not from a shell placeholder`() {
        val messenger = FakeMessenger()
        FlutterHostEngine.installChannelShell(messenger)

        val latch = java.util.concurrent.CountDownLatch(1)
        var reply: ByteBuffer? = null
        messenger.invokeAsync(
            FlutterHostEngine.BRIDGE_METHOD_CHANNEL,
            "getHostInfo",
            ByteArray(0),
        ) { r ->
            reply = r
            latch.countDown()
        }
        assertTrue("桥必须在 5s 内回结果", latch.await(5, java.util.concurrent.TimeUnit.SECONDS))

        val parts = errorPartsOf(reply)
        val code = parts.first
        assertTrue(
            "方法面应答码必须出自契约 §5 七码表(桥的产物),实际: $code",
            code in setOf(
                "HOST_UNAVAILABLE", "DECODE_FAILED", "NOT_CONNECTED", "AUTH_REQUIRED",
                "INVALID_ADDR", "PAIR_DISABLED", "INTERNAL",
            ),
        )
        assertTrue("桥必须应答(error 形态),不得静默", code != null)
        // 壳时代的占位消息(「本波(B4a)未接线」)必须绝迹。
        val message = parts.second
        assertTrue("桥的人话 message 须非空: $message", !message.isNullOrBlank())
        assertTrue(
            "不得再出现壳时代的占位措辞「未接线」: $message",
            !message!!.contains("未接线"),
        )
        assertTrue(
            "不得再出现壳时代的波次署名「B4a」: $message",
            !message.contains("B4a"),
        )
    }
}
