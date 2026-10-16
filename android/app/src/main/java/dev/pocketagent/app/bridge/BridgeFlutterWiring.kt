package dev.pocketagent.app.bridge

import io.flutter.plugin.common.BinaryMessenger
import io.flutter.plugin.common.MethodCall
import io.flutter.plugin.common.MethodChannel

/**
 * 把真 Flutter [BinaryMessenger] 适配成桥内部的 [BridgeBinaryMessenger]。
 *
 * 【为什么需要这一层】本仓库的 Android app 是 `com.android.application` 自当家，
 * flutter embedding 作为库被 `implementation(project(":flutter"))` 引入（见
 * settings.gradle.kts 的 include(":flutter")）。桥逻辑本身（[BridgeMethods]）
 * 只依赖自己定义的窄接口，是为了让 JVM 单测不碰 Flutter 引擎；生产入口在这里
 * 把 `MethodChannel(messenger, BRIDGE_CHANNEL_NAME)` 建好并转发。
 *
 * 【字段映射】一一对应、无信息损失：
 *  - `MethodCall.method` → [BridgeMethodCall.method]
 *  - `MethodCall.arguments` → [BridgeMethodCall.arguments]（StandardMethodCodec
 *    解出的值：ByteArray / Map / null）
 *  - `MethodChannel.Result.success/error/notImplemented` → [BridgeResult] 三语义
 */
class FlutterBinaryMessengerAdapter(private val messenger: BinaryMessenger) : BridgeBinaryMessenger {
    override fun setMethodCallHandler(handler: BridgeMethodCallHandler) {
        MethodChannel(messenger, BRIDGE_CHANNEL_NAME).setMethodCallHandler { call, result ->
            handler.onMethodCall(
                BridgeMethodCall(call.method, call.arguments),
                object : BridgeResult {
                    override fun success(response: Any?) = result.success(response)
                    override fun error(errorCode: String?, errorMessage: String?, errorDetails: Any?) =
                        result.error(errorCode!!, errorMessage, errorDetails)
                    override fun notImplemented() = result.notImplemented()
                },
            )
        }
    }
}

/**
 * 生产装配入口（一行接线，供 FlutterEngine 挂载点调用）：
 *
 * ```
 * BridgeMethods.install(engine.dartExecutor.binaryMessenger, AgentConnectionBridge())
 * ```
 *
 * 【为什么单独给这个工厂】`BridgeMethods` 的构造参数是桥内部 seam
 * （[BridgeBinaryMessenger]），测试因此可以用 [RecordingMessenger] 之类的 fake；
 * 而生产侧拿到的必然是 `io.flutter.plugin.common.BinaryMessenger`。工厂把两者
 * 接上，调用点（flutterhost 的 `installChannelShell`）不需要知道桥内部有 seam。
 */
fun BridgeMethods.Companion.install(
    messenger: BinaryMessenger,
    connection: BridgeConnection,
): BridgeMethods = BridgeMethods(FlutterBinaryMessengerAdapter(messenger), connection)

/** 与 [install] 配对：真 Flutter [MethodCall] 的直转形态（保留给逐方法接管用）。 */
internal fun MethodCall.toBridgeCall(): BridgeMethodCall = BridgeMethodCall(method, arguments)
