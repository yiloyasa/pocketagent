package dev.pocketagent.app.bridge

/**
 * Flutter `io.flutter.plugin.common` 三件套的最小镜像。
 *
 * 【为什么需要镜像】实测（2026-10-15 探针编译）：android app 模块类路径上没有
 * flutter embedding（本仓库 Android 侧是 Compose 原生 app；Flutter UI 是并行模块，
 * `settings.gradle.kts` 只 `include(":app")`，`build.gradle.kts` 无 flutter 依赖）。
 * 而 B2 契约的宿主侧必须在通道名、方法名、错误码、返回形态上与 Dart 逐字一致，
 * 所以这里按 Flutter 的真实签名复刻「handler 注册 + 调用分发 + 结果回传」这一层
 * 薄面，桥逻辑（[BridgeMethods]）因此可在 JVM 单测里用 fake 驱动。
 *
 * 【等 FlutterEngine 挂载时怎么办】在能 import io.flutter 的文件里写一个适配器
 * （约 15 行）：`MethodChannel(messenger, BRIDGE_CHANNEL_NAME).setMethodCallHandler`
 * → 把 Flutter 的 `MethodCall(method, arguments)` 包成 [BridgeMethodCall]、把
 * `MethodChannel.Result` 包成 [BridgeResult]，转发给 [BridgeMethods.onMethodCall]。
 * 字段一一对应（method/arguments、success/error/notImplemented 三语义），无信息
 * 损失，也不需要改桥逻辑一行。
 */

/** 一次方法调用（Flutter `MethodCall` 的镜像：方法名 + StandardMethodCodec 解出的值）。 */
class BridgeMethodCall(
    val method: String,
    val arguments: Any?,
)

/**
 * 方法调用结果回传（Flutter `MethodChannel.Result` 的镜像）。
 *
 * 三语义与 Flutter 逐字同形：
 *  - [success]：正常返回（proto wire 字节 / Map / null）；
 *  - [error]：抛 `PlatformException(code, message, details)` 给 Dart；
 *  - [notImplemented]：通道无 handler（引擎对无 handler 的 invokeMethod 回空应答，
 *    Dart 侧表现为 `MissingPluginException` → `HOST_UNAVAILABLE`，§2.5）。
 */
interface BridgeResult {
    fun success(response: Any?)
    fun error(errorCode: String?, errorMessage: String?, errorDetails: Any?)
    fun notImplemented()
}

/** handler 形状（Flutter `MethodChannel.MethodCallHandler` 的镜像）。 */
fun interface BridgeMethodCallHandler {
    fun onMethodCall(call: BridgeMethodCall, result: BridgeResult)
}

/**
 * BinaryMessenger 的镜像：只承载「给某通道名注册 handler」这一件事。
 *
 * 【为什么不做完整 BinaryMessenger.send】那需要复刻 StandardMethodCodec 的字节
 * 编解码（~200 行、且是引擎职责）；桥逻辑只消费「调用进来、结果回去」这一面，
 * 把编解码留给引擎/适配器。方法名与 Flutter 的 `setMethodCallHandler` 保持一致，
 * 适配器可以一行转发。
 */
interface BridgeBinaryMessenger {
    fun setMethodCallHandler(handler: BridgeMethodCallHandler)
}

/** 通道名常量（契约字符串，与 Dart `kPaBridgeMethodChannel` 逐字一致）。 */
const val BRIDGE_CHANNEL_NAME = "dev.pocketagent/bridge"
