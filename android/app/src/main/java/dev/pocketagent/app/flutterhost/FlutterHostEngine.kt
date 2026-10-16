package dev.pocketagent.app.flutterhost

import android.content.Context
import dev.pocketagent.app.bridge.AgentConnectionBridge
import dev.pocketagent.app.bridge.BridgeMethods
import dev.pocketagent.app.bridge.install
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.embedding.engine.FlutterEngineCache
import io.flutter.embedding.engine.dart.DartExecutor
import io.flutter.plugin.common.BinaryMessenger
import io.flutter.plugin.common.EventChannel

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
 *    本波不做自动预热——预热即建 VM + 跑 Dart 入口，没人看的引擎纯烧内存与电
 *    （现调用点是用户点「Flutter UI」入口 / 宿主 Activity 现场热，见 FlutterHostEntry）。
 *  - 引擎**不**随 Activity 销毁而销毁（缓存语义）：FlutterHostActivity 走
 *    cached engine 路径，`shouldDestroyEngineWithHost` 恒 false。
 *    进程级缓存意味着「回到宿主 Compose UI 时 Dart 侧状态仍在」——
 *    这是 add-to-app 的正常语义，不是泄漏。
 *
 * 【B4 波次接线状态】B4c 已把方法面接上真桥：通道装配（[installChannelShell]）
 * 调用 `BridgeMethods.install(messenger, AgentConnectionBridge())`（B4b 落盘的
 * 生产实现）——契约内 24 条方法走真 gRPC 派发，6 条双向流方法由桥自己如实回
 * HOST_UNAVAILABLE。事件面（三条 EventChannel）仍是静默壳：帧的合流消费面属
 * B4d，挂静默壳而不是假推流。
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
                // 本波全用默认值(见两处常量的 KDoc),钩子先立住——将来要给宿主
                // 一个只跑某子路由的入口,只改常量即可,装配路径不用再动。
                created.dartExecutor.executeDartEntrypoint(
                    DartExecutor.DartEntrypoint.createDefault(),
                )
                // 通道装配必须在 executeDartEntrypoint **之后**挂吗?不——messenger
                // 在 Dart 代码跑起来之前就绪(FlutterEngine 构造时建好),先挂后跑
                // 与先跑后挂都等价;这里先跑入口再挂通道,是为了让「通道未挂好」的
                // 窗口与 Dart 冷启重叠,而不是在入口执行完、UI 已可见后才补挂。
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
     * 在引擎的 messenger 上挂通道：**方法面接真桥**（B4c），事件面仍静默。
     *
     * 【方法面为什么不再自己回 HOST_UNAVAILABLE】B4b 已把 `BridgeMethods` 落盘
     * （契约 §3 的 30 条方法：24 条走真 gRPC 派发，6 条双向流方法由桥自己如实回
     * HOST_UNAVAILABLE + 留后注释）。壳若继续「一律占位」，就是把已落盘的成品
     * 伪装成缺席——用户点 Flutter UI 会看到「本波未接线」，而桌面 daemon 明明
     * 连着。故方法面改由生产装配入口 [BridgeMethods.install] 接上：
     *
     * ```
     * BridgeMethods.install(messenger, AgentConnectionBridge())
     * ```
     *
     * [AgentConnectionBridge] 是薄适配（B4b，bridge/BridgeConnection.kt）：把
     * 桥内部 seam 的每次调用转发到进程级单例 [dev.pocketagent.app.AgentConnection]
     * 的五条 StateFlow 现值 + 六个 blocking stub 取数面。它是**按需取 stub**
     * （`AgentConnection.hostService()` 内部 `channel()` 懒建），构造本身不触发
     * 建连/探活，因此在 engine 创建路径同步执行无初始化顺序问题：messenger 挂
     * handler 只是注册，Dart 侧何时 invoke 才会真正走到连接层。
     *
     * 【为什么事件面仍是静默壳】三条 EventChannel 的帧合流消费面（`conn_state` /
     * `host_info` / `host_event` 三源，channel-contract.md §4/§6）属 B4d。本波
     * 挂静默壳而不是假推流：listen 收下、不发一帧、cancel 干净退出——「没有流」
     * 比「推假帧」诚实（红线：不伪造）。判定宿主在不在仍走方法面探针（§2.5）。
     *
     * 【未安装时的兜底语义（诚实缺席链不依赖本壳）】若本方法因故未执行，
     * Flutter 侧该通道无 handler → invokeMethod 得 MissingPluginException →
     * Dart 转 HOST_UNAVAILABLE（§2.5）。这条链由引擎与 Dart 侧保证，不需要壳
     * 写任何错误码兜底；壳只负责「该接的接上」。
     *
     * 【为什么入参仍是 BinaryMessenger 而不是 FlutterEngine】[BinaryMessenger]
     * 是接口，测试可用 fake 驱动同一段注册逻辑（见 FlutterHostChannelShellTest /
     * FlutterHostBridgeInstallTest）；FlutterEngine 是具体类、需要 libflutter.so，
     * JVM 单测造不出来。
     */
    fun installChannelShell(messenger: BinaryMessenger) {
        // 方法面：B4b 的成品桥。通道名由 BridgeChannel.BRIDGE_CHANNEL_NAME 给出，
        // 与本类的 BRIDGE_METHOD_CHANNEL 同值（双端都从契约来，不构成第二份名单）。
        BridgeMethods.install(messenger, AgentConnectionBridge())

        // 事件面：静默壳——不推流，也不 error。
        //
        // 【为什么不 error】channel-contract.md §2.5：宿主缺席时事件面握手错不进流
        // (Flutter 把 'listen' 的异常转投 FlutterError.reportError)，判定宿主在不在
        // 必须走方法面探针。事件面若 error，Dart 侧 receiveBroadcastStream 会以
        // FlutterError 冒泡——那是「假装有流却坏了」，比「没流」更误导。
        // 故：listen 收下、不发一帧、cancel 干净退出。真实推流（B4d）按 §4 接。
        for (channelName in listOf(
            BRIDGE_EVENT_CHANNEL,
            BRIDGE_TURN_EVENT_CHANNEL,
            BRIDGE_TERMINAL_EVENT_CHANNEL,
        )) {
            EventChannel(messenger, channelName).setStreamHandler(
                object : EventChannel.StreamHandler {
                    override fun onListen(arguments: Any?, events: EventChannel.EventSink?) {
                        // B4d 才接线：本波不推流，上面 KDoc 说明为什么静默而不是报错。
                    }

                    override fun onCancel(arguments: Any?) {
                        // 无订阅态要清理。
                    }
                },
            )
        }
    }

    /**
     * 契约错误码字面量（channel-contract.md §5）。
     *
     * 【为什么还留着】B4c 之后方法面的错误码全部由桥（[BridgeMethods] →
     * [dev.pocketagent.app.bridge.BridgeErrorCodes]）产出，本常量已无生产调用点；
     * 保留是为了让 flutterhost 测试能继续钉「壳与契约 §5 同值」这条事实，并给
     * 事件面（B4d 若需要在静默期发错误码）留一个同包锚点。它不是第二份名单——
     * 只是单个 wire 字符串的镜像，与 bridge 包的同名常量同值、同源（契约文档）。
     */
    const val HOST_UNAVAILABLE: String = "HOST_UNAVAILABLE"
}
