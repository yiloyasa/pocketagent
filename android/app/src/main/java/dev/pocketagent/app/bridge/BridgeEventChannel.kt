package dev.pocketagent.app.bridge

import io.flutter.plugin.common.BinaryMessenger
import io.flutter.plugin.common.EventChannel

/**
 * 三条事件通道名（wire 契约字符串，与 Dart 侧逐字一致）。
 *
 * 【为什么 bridge 包自己也带一份】与 [BRIDGE_CHANNEL_NAME] 同纪律（见
 * [BridgeChannel.kt]）：契约字符串的唯一真值是 `flutter/docs/channel-contract.md`
 * §4 与 Dart 侧常量（`pa_bridge.dart` 的 `kPaBridgeEventChannel` /
 * `chat_turn_stream.dart` 的 `kTurnStreamEventChannel` / `terminal_stream.dart` 的
 * `kTerminalEventChannel`）。这里是 Kotlin 侧的收口点，不是第二份名单——
 * 双端都从契约来，改一个字符双端测试同时红。
 */
const val BRIDGE_EVENT_CHANNEL_NAME = "dev.pocketagent/bridge/events"

/** Turn 双向流事件面（`chat_turn_stream.dart` 的 `kTurnStreamEventChannel`）。 */
const val BRIDGE_TURN_EVENT_CHANNEL_NAME = "dev.pocketagent/bridge/turn_events"

/** Terminal 事件面（`terminal_stream.dart` 的 `kTerminalEventChannel`）。 */
const val BRIDGE_TERMINAL_EVENT_CHANNEL_NAME = "dev.pocketagent/bridge/terminal_events"

/**
 * `EventChannel.EventSink` 的镜像（三语义与 Flutter 逐字同形）。
 *
 *  - [success]：推一帧（conn_state 的 Map / host_info、host_event 的字节 Map /
 *    turn_events、terminal_events 的裸 wire 字节）；
 *  - [error]：流错误事件（Dart 侧 `receiveBroadcastStream` 解成
 *    `PlatformException` 后由各 transport 收敛成 `PaBridgeException`）；
 *  - [endOfStream]：正常关流（Dart 侧收到 null 回包 → `controller.close()`）。
 *
 * 【error 之后流不关】Flutter 的 `receiveBroadcastStream` 对 error envelope 走
 * `controller.addError`，broadcast 流本身保持打开——与契约 §4「坏事件不毒化流」
 * 逐字对齐。宿主侧要关流只能走 [endOfStream]。
 */
interface BridgeEventSink {
    fun success(event: Any?)
    fun error(errorCode: String, errorMessage: String?, errorDetails: Any?)
    fun endOfStream()
}

/** `EventChannel.StreamHandler` 的镜像（onListen/onCancel 两语义）。 */
interface BridgeEventStreamHandler {
    fun onListen(arguments: Any?, events: BridgeEventSink)
    fun onCancel(arguments: Any?)
}

/**
 * `EventChannel` 注册面的镜像：给某通道名挂 StreamHandler。
 *
 * 【为什么与 [BridgeBinaryMessenger] 分成两个接口】方法面与事件面的注册语义
 * 不同（一个是「调用进来、结果回去」，一个是「订阅建立、推流到取消」），合成一个
 * 接口会让两边都得多带一个用不到的方法。生产由 [FlutterEventMessengerAdapter]
 * 把两者都接到同一个 `BinaryMessenger` 上。
 */
interface BridgeEventMessenger {
    fun setStreamHandler(channelName: String, handler: BridgeEventStreamHandler)
}

/**
 * 生产适配器：真 Flutter [EventChannel] → 桥内部 [BridgeEventMessenger]。
 *
 * 字段映射一一对应、无信息损失：
 *  - `EventChannel.StreamHandler.onListen(Object, EventSink)` → [BridgeEventStreamHandler.onListen]；
 *  - `EventSink.success/error/endOfStream` → [BridgeEventSink] 三语义；
 *  - `EventChannel.StreamHandler.onCancel(Object)` → [BridgeEventStreamHandler.onCancel]。
 *
 * 【sink 为 null 时】Flutter 的 `EventSink` 是平台类型，理论可空；框架实际恒传
 * 非空。真收到 null 时推流无处可去，用 [NullEventSink] 静默兜底——不抛：抛异常
 * 会被框架转投 `FlutterError.reportError`，把「推不出去」放大成「流坏了」。
 */
class FlutterEventMessengerAdapter(private val messenger: BinaryMessenger) : BridgeEventMessenger {
    override fun setStreamHandler(channelName: String, handler: BridgeEventStreamHandler) {
        EventChannel(messenger, channelName).setStreamHandler(
            object : EventChannel.StreamHandler {
                override fun onListen(arguments: Any?, events: EventChannel.EventSink?) {
                    handler.onListen(
                        arguments,
                        events?.let { FlutterEventSinkAdapter(it) } ?: NullEventSink,
                    )
                }

                override fun onCancel(arguments: Any?) {
                    handler.onCancel(arguments)
                }
            },
        )
    }
}

/** 真 `EventChannel.EventSink` → [BridgeEventSink]（一行转发，无字段加工）。 */
private class FlutterEventSinkAdapter(
    private val sink: EventChannel.EventSink,
) : BridgeEventSink {
    override fun success(event: Any?) = sink.success(event)
    override fun error(errorCode: String, errorMessage: String?, errorDetails: Any?) =
        sink.error(errorCode, errorMessage, errorDetails)
    override fun endOfStream() = sink.endOfStream()
}

/** 框架传来 null sink 时的静默兜底（见 [FlutterEventMessengerAdapter] KDoc）。 */
private object NullEventSink : BridgeEventSink {
    override fun success(event: Any?) = Unit
    override fun error(errorCode: String, errorMessage: String?, errorDetails: Any?) = Unit
    override fun endOfStream() = Unit
}

/**
 * 宿主侧日志 seam（生产走 `android.util.Log`，测试注入记录型 fake）。
 *
 * 【为什么不用 android.util.Log 直写】JVM 单测没有 Android 框架，
 * `android.util.Log.w` 会抛 "not mocked"（本仓库单测零 Robolectric）。收成 seam
 * 后生产日志照打、测试注入 fake，两条路都不为对方改语义。
 */
fun interface BridgeLog {
    fun w(tag: String, message: String)
}

/** 生产日志实现（仅在真机/真引擎路径被加载；测试注入 fake 时该类不被触碰）。 */
object AndroidLog : BridgeLog {
    // 块体：Log.w 返回 Int（写入字节数），接口声明 Unit；表达式体会把 Int 当
    // 返回值触发「not a subtype of overridden member」。
    override fun w(tag: String, message: String) {
        android.util.Log.w(tag, message)
    }
}
