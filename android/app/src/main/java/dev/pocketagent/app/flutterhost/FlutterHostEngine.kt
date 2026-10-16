package dev.pocketagent.app.flutterhost

import android.content.Context
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.embedding.engine.FlutterEngineCache
import io.flutter.embedding.engine.dart.DartExecutor
import io.flutter.plugin.common.BinaryMessenger
import io.flutter.plugin.common.EventChannel
import io.flutter.plugin.common.MethodChannel

/**
 * Flutter UI 宿主的引擎持有者：一个进程只热一份 [FlutterEngine]。
 *
 * 【为什么要有这一层】Flutter 引擎冷启 ~300ms+（起 VM、加载 libapp.so、跑 Dart
 * entrypoint），每次 new FlutterEngine 再 executeDartEntrypoint 就是用户每次点
 * 「Flutter UI」都白等一次。官方 add-to-app 的做法是预热 + [FlutterEngineCache]
 * 复用（flutter.dev/to/add-to-app 的 "warming up" 一节），本类即该模式的落地。
 *
 * 【线程/生命周期纪律】
 *  - [warmup] 是**幂等**的：已热过直接返回缓存实例，不重复起 VM。
 *  - 调用方应在进程内早一时刻预热（Application/主 Activity onCreate 之后），
 *    本波不做自动预热——宿主壳尚未接线，起个没人看的引擎纯烧内存与电。
 *  - 引擎**不**随 Activity 销毁而销毁（缓存语义）：FlutterHostActivity 走
 *    cached engine 路径，`shouldDestroyEngineWithHost` 恒 false。
 *    进程级缓存意味着「回到宿主 Compose UI 时 Dart 侧状态仍在」——
 *    这是 add-to-app 的正常语义，不是泄漏。
 *
 * 【B4 波次接线点】通道壳（[installChannelShell]）把契约文档 §3 的 30 条方法
 * 一律回 HOST_UNAVAILABLE：本波只挂壳不接方法面。`BridgeMethods` 落盘后（B4b），
 * 在 [installChannelShell] 里把 [dev.pocketagent.app.bridge.BridgeMethods] 的
 * handler 接进来即可——壳已把 `MethodChannel(messenger, NAME)` 建好，
 * 接线工作量是「setMethodCallHandler 换个实现」这一行。
 */
object FlutterHostEngine {
    /** 缓存键。[FlutterHostActivity] 用同一键取引擎,两处必须一致。 */
    const val ENGINE_ID: String = "dev.pocketagent.app/flutter_host_engine"

    /** 契约字符串:方法面,与 Dart `kPaBridgeMethodChannel` 逐字一致(channel-contract.md §1)。 */
    const val BRIDGE_METHOD_CHANNEL: String = "dev.pocketagent/bridge"

    /** 契约字符串:事件面(连接态/host_info/host_event),与 Dart `kPaBridgeEventChannel` 一致。 */
    const val BRIDGE_EVENT_CHANNEL: String = "dev.pocketagent/bridge/events"

    /** 契约字符串:Turn 双向流事件面(chat_turn_stream.dart)。本波无推流。 */
    const val BRIDGE_TURN_EVENT_CHANNEL: String = "dev.pocketagent/bridge/turn_events"

    /** 契约字符串:Terminal 事件面(terminal_stream.dart)。本波无推流。 */
    const val BRIDGE_TERMINAL_EVENT_CHANNEL: String = "dev.pocketagent/bridge/terminal_events"

    /**
     * Dart 入口函数名。默认 "main"(DartExecutor.DartEntrypoint.createDefault 的值)。
     *
     * 【为什么留成可改的钩子而不是写死】契约文档 §3 的方法面是**单入口**的
     * (main → runApp(PocketAgentApp));将来若要给宿主一个只跑某个子路由的入口
     * （例如「只开终端页」），改这里即可，不用改 FlutterHostActivity。
     */
    const val DART_ENTRYPOINT: String = "main"

    /**
     * 初始路由。null = 交给 Dart 侧默认（main.dart 的 Onboarding/HomeShell 分流）。
     *
     * 【为什么默认 null 而不是 "/"】main.dart 用 `_onboardingDone` 决定首屏,
     * 与 Kotlin 侧 `OnboardingPrefs.isDone()` 同语义;写死 "/" 会绕过新手教程
     * 分流。首屏决策权在 Dart 侧(它是 UI 的真值源),宿主不预设。
     */
    val INITIAL_ROUTE: String? = null

    /** 进程级引擎缓存。@Volatile: 预热线程与 Activity 主线程可能并发读写。 */
    @Volatile
    private var engine: FlutterEngine? = null

    /**
     * 取（没有则热）进程级引擎。
     *
     * 线程安全：双检锁。[FlutterEngineCache] 与 [engine] 写同一实例，读任一路即可
     * —— 官方示例只写 cache，本类同时持一份强引用是为了让 [shutdown] 能主动释放
     * （cache 是静态 Map，持引用即跨进程泄漏不清，测试里需要显式清理钩子）。
     */
    fun warmup(context: Context): FlutterEngine {
        engine?.let { return it }
        return synchronized(this) {
            engine ?: FlutterEngine(context.applicationContext).also { created ->
                // 入口钩子:官方 add-to-app 的 dartEntrypoint/initialRoute 定制点。
                // 本波全用默认值(见两处常量的 KDoc),钩子先立住,B4b 接方法面时
                // 不需要再动引擎创建路径。
                created.dartExecutor.executeDartEntrypoint(
                    DartExecutor.DartEntrypoint.createDefault(),
                )
                // 通道壳必须在 executeDartEntrypoint **之后**挂吗?不——messenger
                // 在 Dart 代码跑起来之前就绪(FlutterEngine 构造时建好),先挂后跑
                // 与先跑后挂都等价;这里先跑入口再挂壳,是为了让「壳未挂好」的窗口
                // 与 Dart 冷启重叠,而不是在入口执行完、UI 已可见后才补挂。
                installChannelShell(created.dartExecutor.binaryMessenger)
                engine = created
                FlutterEngineCache.getInstance().put(ENGINE_ID, created)
            }
        }
    }

    /** 已预热则返回引擎,否则 null(不触发预热)。 */
    fun cachedEngine(): FlutterEngine? = engine ?: FlutterEngineCache.getInstance().get(ENGINE_ID)

    /** 是否已预热。 */
    fun isWarmedUp(): Boolean = cachedEngine() != null

    /**
     * 释放进程级引擎(仅测试/进程收尾用)。
     *
     * 生产代码不调它:引擎跨 Activity 复用是 add-to-app 的设计语义,进程退出时
     * 由系统回收。留出来是因为测试/真机排障需要能显式清理(见 flutterhost 测试的
     * tearDown)。
     */
    fun shutdown() {
        synchronized(this) {
            engine?.let { FlutterEngineCache.getInstance().remove(ENGINE_ID) }
            engine?.destroy()
            engine = null
        }
    }

    /**
     * 在引擎的 messenger 上挂通道壳：方法面一律 HOST_UNAVAILABLE,事件面静默。
     *
     * 【为什么方法面「一律」占位,而不是按 §3 表逐条判】壳的职责是「诚实的缺席」:
     * 本波没有 BridgeMethods,任何 method 都没有可用实现,逐条列名单只会把「未实现」
     * 伪装成「已实现」。写死名单也是红线(不新增写死的名单)——Dart 侧对无 handler
     * 通道的 invokeMethod 得 MissingPluginException → HOST_UNAVAILABLE(§2.5),
     * 语义本来就不依赖名单。因此本壳是**单一** handler:收到什么回什么错。
     *
     * 【为什么入参是 BinaryMessenger 而不是 FlutterEngine】BinaryMessenger 是接口,
     * 测试可用 fake 驱动同一段注册逻辑(见 FlutterHostChannelShellTest);FlutterEngine
     * 是具体类、需要 libflutter.so,JVM 单测造不出来。拆开之后「壳的逻辑」与
     * 「真引擎」解耦,壳的每一行都在单测覆盖内,而引擎那层归真机验收。
     *
     * 【B4b 接线点】`BridgeMethods`(android/app/src/main/java/dev/pocketagent/
     * app/bridge/) 落盘后,把本方法内的 MethodChannel handler 换成
     * `BridgeMethods(messenger, AgentConnectionBridge())` 的 handle 即可;
     * 三个 EventChannel 的 setStreamHandler 同理换成 conn_state/host_info/
     * host_event 的合流实现(channel-contract.md §6 清单)。本壳的方法名常量
     * (BRIDGE_METHOD_CHANNEL) 与 BridgeChannel.kt 的 BRIDGE_CHANNEL_NAME 同值,
     * 双端都从契约来,不构成第二份名单。
     */
    fun installChannelShell(messenger: BinaryMessenger) {
        // 方法面:任何 method → HOST_UNAVAILABLE(如实缺席,不伪造任何返回)。
        MethodChannel(messenger, BRIDGE_METHOD_CHANNEL).setMethodCallHandler { call, result ->
            // message 点名「本波未接方法面」+ B4b,不写死任何方法名:
            // call.method 原样带回只为定位,不参与分支。
            result.error(
                HOST_UNAVAILABLE,
                "Flutter 宿主桥方法面本波(B4a)未接线：通道壳已就绪,方法实现" +
                    "(bridge/BridgeMethods.kt)与 AgentConnection 取数面留待 B4b。" +
                    "（调用 method = ${call.method}）",
                null,
            )
        }

        // 事件面:静默壳——不推流,也不 error。
        //
        // 【为什么不 error】channel-contract.md §2.5:宿主缺席时事件面握手错不进流
        // (Flutter 把 'listen' 的异常转投 FlutterError.reportError),判定宿主在不在
        // 必须走方法面探针。事件面若 error,Dart 侧 receiveBroadcastStream 会以
        // FlutterError 冒泡——那是「假装有流却坏了」,比「没流」更误导。
        // 故:listen 收下、不发一帧、cancel 干净退出。真实推流(B4b)按 §4 接。
        for (channelName in listOf(
            BRIDGE_EVENT_CHANNEL,
            BRIDGE_TURN_EVENT_CHANNEL,
            BRIDGE_TERMINAL_EVENT_CHANNEL,
        )) {
            EventChannel(messenger, channelName).setStreamHandler(
                object : EventChannel.StreamHandler {
                    override fun onListen(arguments: Any?, events: EventChannel.EventSink?) {
                        // 本波不推流:上面 KDoc 说明为什么静默而不是报错。
                    }

                    override fun onCancel(arguments: Any?) {
                        // 无订阅态要清理。
                    }
                },
            )
        }
    }

    /**
     * 方法面占位错误码。
     *
     * 【为什么是壳自己的常量而不是引 bridge 包的】bridge/ 包是并行子代理的在建
     * 文件,本轮**只读**它;壳若 import 它的 BridgeErrorCodes,就把本包的编译
     * 与那条在建线的中间态绑在一起(它现在就是编译不过的)。契约码是 wire 字符串,
     * 壳按 channel-contract.md §5 自带一份字面量,B4b 接线时改成引用即可
     * (届时那份已是稳定落盘态)。两处同值由 channel-contract 文档背书。
     */
    const val HOST_UNAVAILABLE: String = "HOST_UNAVAILABLE"
}
