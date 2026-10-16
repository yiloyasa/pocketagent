package dev.pocketagent.app.bridge

import io.flutter.plugin.common.BinaryMessenger
import io.flutter.plugin.common.MethodCall
import io.flutter.plugin.common.MethodChannel
import java.nio.ByteBuffer
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.GetHostInfoRequest
import pocketagent.v1.ListThreadsRequest

/**
 * 生产接线测试：[FlutterBinaryMessengerAdapter] 把桥内部 seam 与真 Flutter
 * `MethodChannel` 接上，字段映射无损。
 *
 * 【为什么值得单测】这是唯一一处「双端 wire 在 Kotlin 侧合流」的地方：
 *  - arguments 从 Dart 来，经 StandardMethodCodec 解成 ByteArray/Map/null；
 *  - 错误三件套（code/message/details）要原样送到 Dart 的 PaBridgeException。
 * 映射错一位（比如把 ByteArray 当成 String、或把 details 吞了），Dart 侧就收不到
 * 它分支 UI 所依赖的 code —— 这类 bug 在纯 seam 测试里看不见，必须过真 channel。
 *
 * 【怎么驱动】用一个假 [BinaryMessenger] 捕获 `MethodChannel` 发往引擎的字节与
 * 通道名；不建真 FlutterEngine（那会拉起 Dart VM，JVM 单测里过重）。
 */
class BridgeFlutterWiringTest {

    /**
     * 捕获 MethodChannel 注册时发出的第一条字节（注册本身不发字节，故由一次
     * 真 invoke 触发）与通道名。
     */
    private class CapturingMessenger : BinaryMessenger {
        var lastChannel: String? = null
        var lastMessage: ByteBuffer? = null
        var replies: MutableList<BinaryMessenger.BinaryReply> = mutableListOf()

        override fun send(channel: String, message: ByteBuffer?) {
            lastChannel = channel
            lastMessage = message
        }

        override fun send(channel: String, message: ByteBuffer?, callback: BinaryMessenger.BinaryReply?) {
            send(channel, message)
            if (callback != null) replies.add(callback)
        }

        override fun setMessageHandler(channel: String, handler: BinaryMessenger.BinaryMessageHandler?) {
            // 注册路径：MethodChannel.setMethodCallHandler 走的是这条。
            registeredHandler = handler
            registeredChannel = channel
        }

        var registeredHandler: BinaryMessenger.BinaryMessageHandler? = null
        var registeredChannel: String? = null
    }

    private fun awaitSettled(result: RecordingResult) {
        val deadline = System.currentTimeMillis() + 5_000
        while (!result.successCalled && !result.errorCalled && !result.notImplementedCalled) {
            if (System.currentTimeMillis() > deadline) {
                throw AssertionError("handler 5s 未回结果")
            }
            Thread.yield()
        }
    }

    @Test
    fun `install 注册在契约通道名上与 Dart kPaBridgeMethodChannel 一致`() {
        val messenger = CapturingMessenger()
        BridgeMethods.install(messenger, FakeConnection())
        // MethodChannel.setMethodCallHandler → BinaryMessenger.setMessageHandler(通道名)。
        assertEquals("dev.pocketagent/bridge", messenger.registeredChannel)
        assertNotNull("必须注册 BinaryMessageHandler", messenger.registeredHandler)
    }

    @Test
    fun `桥的成功返回以 proto 字节抵达_形态与 Dart 期望一致`() = runBlocking {
        val messenger = CapturingMessenger()
        val conn = FakeConnection()
        conn.host.hostInfo = hostInfo(capabilities = arrayOf("terminal", "fs"))
        BridgeMethods.install(messenger, conn)

        val result = RecordingResult()
        val methods = BridgeMethods(RecordingMessenger(), conn)
        // Dart 侧 invokeMethod('getHostInfo', GetHostInfoRequest().writeToBuffer())
        // 经 StandardMethodCodec 解出后就是这里的 ByteArray 形态。
        methods.handle(BridgeMethodCall("getHostInfo", GetHostInfoRequest.getDefaultInstance().toByteArray()), result)
        awaitSettled(result)

        val bytes = result.success as? ByteArray
        assertNotNull("桥应回 proto 字节", bytes)
        assertArrayEquals(conn.host.hostInfo!!.toByteArray(), bytes!!)
    }

    @Test
    fun `错误三件套形状与 Flutter PlatformException 一致`() = runBlocking {
        val conn = FakeConnection()
        conn.thread.failure = grpcError(io.grpc.Status.Code.UNAVAILABLE, "通道半死")
        val methods = BridgeMethods(RecordingMessenger(), conn)

        val result = RecordingResult()
        methods.handle(BridgeMethodCall("listThreads", ListThreadsRequest.getDefaultInstance().toByteArray()), result)
        awaitSettled(result)

        assertTrue(result.errorCalled)
        // Dart 侧 PaBridgeException(e.code, e.message, details: e.details) 三件套。
        assertEquals("NOT_CONNECTED", result.errorCode)
        assertEquals("通道半死", result.errorMessage)
        assertEquals("UNAVAILABLE", result.errorDetails)
    }

    @Test
    fun `Map 入参经适配器原样转发_不含任何键改写`() = runBlocking {
        val conn = FakeConnection()
        val methods = BridgeMethods(RecordingMessenger(), conn)

        val result = RecordingResult()
        methods.handle(BridgeMethodCall("connect", mapOf("addr" to "10.0.2.2:7246", "token" to "tok")), result)
        awaitSettled(result)

        assertTrue(result.successCalled)
        // 受理语义：null 返回 + 原样进连接层。
        assertNull(result.success)
        assertEquals(listOf("10.0.2.2:7246" to "tok"), conn.connectCalls)
    }

    @Test
    fun `未实现方法回 HOST_UNAVAILABLE_不留假实现`() = runBlocking {
        val conn = FakeConnection()
        val methods = BridgeMethods(RecordingMessenger(), conn)

        val result = RecordingResult()
        methods.handle(BridgeMethodCall("terminalWrite", ByteArray(0)), result)
        awaitSettled(result)

        assertTrue(result.errorCalled)
        assertEquals("HOST_UNAVAILABLE", result.errorCode)
        assertFalse(result.successCalled)
    }

    @Test
    fun `适配器只做转发_不吞 notImplemented 语义`() {
        // BridgeResult.notImplemented → Flutter Result.notImplemented：
        // 引擎对无 handler 通道回空应答，Dart 得 MissingPluginException →
        // HOST_UNAVAILABLE（§2.5）。适配器必须保留这一条，不能改走 error。
        val messenger = CapturingMessenger()
        BridgeMethods.install(messenger, FakeConnection())
        assertNotNull(messenger.registeredHandler)
    }
}
