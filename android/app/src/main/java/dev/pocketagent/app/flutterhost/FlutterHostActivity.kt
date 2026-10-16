package dev.pocketagent.app.flutterhost

import android.content.Context
import android.os.Bundle
import io.flutter.embedding.android.FlutterActivity
import io.flutter.embedding.android.FlutterActivityLaunchConfigs
import io.flutter.embedding.engine.FlutterEngine

/**
 * Flutter UI 的宿主 Activity：全屏承载 `flutter/` module 的 Dart UI。
 *
 * 【为什么继承 FlutterActivity 而不是自绘 FlutterView】FlutterActivity 已把
 * 引擎生命周期、FlutterView 挂载、系统栏/键盘/返回键、platform plugin
 * (PlatformPlugin/SensitiveContentPlugin) 全部处理好；自绘容器等于把这些
 * 再实现一遍。本波只挂壳，不值得为「看起来可控」付出那份维护面。
 *
 * 【引擎从哪来】cached engine 路径：[FlutterHostEngine.warmup] 预热后由
 * [provideFlutterEngine] 交出。**不**走 FlutterActivity 自建引擎的默认路径——
 * 那样每次进本页都新建 VM（见 [FlutterHostEngine] 的 KDoc）。未预热时（进程
 * 刚起、用户手快）由 [provideFlutterEngine] 现场热一份：冷启慢是实话，但不让
 * 入口失效。
 *
 * 【所有取值为什么走 [FlutterHostPolicy]】Activity 的 override 是宿主壳契约，
 * 但 Robolectric 不在本仓库依赖里；把值集中到纯 object 后，JVM 单测能直接读
 * 它们，而本类只剩一层透传、不可能与策略不一致。详见 FlutterHostPolicy 的 KDoc。
 *
 * 【B4b 接线点】Dart 侧 conn_state/host_info/host_event 与 30 条方法面接通后，
 * 本类可能需要：
 *  1. [onFlutterUiNoLongerDisplayed] / [cleanUpFlutterEngine] 里清 Dart 侧订阅
 *     （如果 B4b 的事件面合流挂在引擎上而非本 Activity 上，则无此步）；
 *  2. 深链入参（EXTRA_INITIAL_ROUTE）——将来 Kotlin 侧某页要直达 Flutter 某路由时，
 *     在 [FlutterHostLauncher] 加显式参数，不要让调用方自己塞 intent extra。
 */
class FlutterHostActivity : FlutterActivity() {
    /** 现场热一份（已热则直接返回缓存实例，见 [FlutterHostEngine.warmup] 的幂等说明）。 */
    override fun provideFlutterEngine(context: Context): FlutterEngine =
        FlutterHostEngine.warmup(context)

    /**
     * 让 FlutterActivity 的 cached-engine 查找用上我们的键。
     *
     * 默认实现返回 null → delegate 会 new FlutterEngine 自建；返回 cached id 后
     * delegate 走 [FlutterEngineCache] 查找，与 [provideFlutterEngine] 二选一即可
     * (cached id 优先)。本类两条都给了：[provideFlutterEngine] 承载「未预热就
     * 现场热」的兜底逻辑，cached id 保证 onDestroy 不误销毁。
     */
    override fun getCachedEngineId(): String? = FlutterHostPolicy.cachedEngineId

    /** 引擎是进程级缓存，Activity 走了引擎要留着给下次（状态不丢）。 */
    override fun shouldDestroyEngineWithHost(): Boolean = FlutterHostPolicy.destroyEngineWithHost

    /** 宿主壳不需要状态保存（cached engine 下 FlutterActivity 也默认关闭）。 */
    override fun shouldRestoreAndSaveState(): Boolean = FlutterHostPolicy.restoreAndSaveState

    /**
     * Dart 入口/初始路由：全走 [FlutterHostEngine] 的钩子常量。
     *
     * 【为什么不从 Intent extra 读】官方 FlutterActivity 支持 EXTRA_DART_ENTRYPOINT /
     * EXTRA_INITIAL_ROUTE；本波的入口是**唯一**的（打开 Flutter UI），不需要调用方
     * 定制。真需要直达路由时用 [FlutterHostLauncher] 带参数，那时再打开这两个
     * extra 的读取（现在打开 = 留一条没人走的代码路径）。
     */
    override fun getDartEntrypointFunctionName(): String = FlutterHostPolicy.dartEntrypointFunctionName

    override fun getInitialRoute(): String? = FlutterHostPolicy.initialRoute

    /** 背景模式：透明（不挡宿主 Compose UI 的转场视觉）。 */
    override fun getBackgroundMode(): FlutterActivityLaunchConfigs.BackgroundMode =
        FlutterHostPolicy.backgroundMode

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        // 【为什么这里也 put 一次 cache】provideFlutterEngine 返回的实例已经由
        // FlutterHostEngine.warmup 放进 cache;本行是冗余防御——若将来有人改了
        // warmup 的内部(例如换成 EngineGroup),cached id 查找仍然命中同一实例,
        // 不会出现「id 对但 cache 空」的静默降级。代价是一次 Map.put,可忽略。
        FlutterHostEngineRegistry.put(FlutterHostEngine.warmup(this))
    }
}
