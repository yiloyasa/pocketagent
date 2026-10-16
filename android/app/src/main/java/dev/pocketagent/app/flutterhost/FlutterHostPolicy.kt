package dev.pocketagent.app.flutterhost

import android.content.Intent
import io.flutter.embedding.android.FlutterActivityLaunchConfigs
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.embedding.engine.FlutterEngineCache

/**
 * FlutterHostActivity 的**取值策略**(纯数据,无 Android 依赖)。
 *
 * 【为什么从 Activity 里拆出来】Activity 的三个 override(cached engine id /
 * destroy-with-host / entrypoint+route)是宿主壳的核心契约,值得单测;但
 * Robolectric 不在本仓库依赖里,为一个壳的取值引入它不划算(还把测试运行时
 * 从纯 JVM 变成需要 Android 框架)。拆成纯 object 后:
 *   - 测试:纯 JVM 直接读取值,零框架依赖(与同仓其他 *Test.kt 同风格);
 *   - Activity:只剩 `override fun x() = FlutterHostPolicy.x`,一层透传,
 *     不可能与策略不一致(见 FlutterHostActivityTest 的一致性断言)。
 *
 * 【为什么不干脆把这些值写进 Activity 的字面量】那正是拆出来的反面:
 * 值散落在 Activity 里,测试碰不到,改一处漏一处无报错。
 */
object FlutterHostPolicy {
    /** 见 [FlutterHostEngine.ENGINE_ID] 的说明(两处必须一致)。 */
    val cachedEngineId: String get() = FlutterHostEngine.ENGINE_ID

    /**
     * 引擎不随 Activity 销毁(缓存语义,见 FlutterHostActivity KDoc)。
     *
     * 写死 false 而不是「读 intent extra」:本波没有任何调用方需要
     * 「这个页面关掉就连引擎一起干掉」的语义;留一个没人传的 extra 分支
     * 等于留一条没人测的路径。
     */
    val destroyEngineWithHost: Boolean get() = false

    /** 状态保存关闭(见 FlutterHostActivity KDoc)。 */
    val restoreAndSaveState: Boolean get() = false

    /** 背景模式:透明,不挡宿主 Compose UI 的转场视觉。 */
    val backgroundMode: FlutterActivityLaunchConfigs.BackgroundMode
        get() = FlutterActivityLaunchConfigs.BackgroundMode.transparent

    /** Dart 入口函数名(见 [FlutterHostEngine.DART_ENTRYPOINT])。 */
    val dartEntrypointFunctionName: String get() = FlutterHostEngine.DART_ENTRYPOINT

    /** 初始路由(见 [FlutterHostEngine.INITIAL_ROUTE])。 */
    val initialRoute: String? get() = FlutterHostEngine.INITIAL_ROUTE
}

/**
 * 把 [FlutterHostPolicy] 应用到一个真 FlutterEngine 的缓存/清理动作。
 *
 * 【为什么单独一个 object】put/remove 缓存是唯一的「有副作用」的策略动作,
 * 单独放便于测试只验「键一致」而不真造引擎(见测试对 FlutterEngineCache 的
 * 只读用法)。
 */
internal object FlutterHostEngineRegistry {
    /** 把引擎放进进程级缓存(键 = [FlutterHostEngine.ENGINE_ID])。 */
    fun put(engine: FlutterEngine) {
        FlutterEngineCache.getInstance().put(FlutterHostEngine.ENGINE_ID, engine)
    }

    /** 从进程级缓存摘掉引擎(销毁由调用方决定,本方法不 destroy)。 */
    fun remove() {
        FlutterEngineCache.getInstance().remove(FlutterHostEngine.ENGINE_ID)
    }
}

/**
 * 启动 [FlutterHostActivity] 的 Intent 工厂。
 *
 * 【为什么不直接 new Intent(context, FlutterHostActivity::class.java)】
 * 将来入口可能带初始路由/深链参数,集中一个构造点让调用方不必知道 extra 键名
 * (那些键是引擎内部的实现细节)。拆成纯 object 后,JVM 单测可验「指向本类、
 * 不带 extra」而不必建 Activity。
 */
object FlutterHostLauncher {
    /**
     * 造启动 Intent。
     *
     * 故意不暴露 extra 参数：本波入口是**唯一**的（打开 Flutter UI）。真需要
     * 直达路由时在这里加显式参数，而不是让调用方自己塞键。
     *
     * @param target 目标 Activity 类，默认即 [FlutterHostActivity]。留这个参数
     *   不为调用方（本波只有一种目标），为的是测试能验「默认值确实是它」
     *   而不必造 Context——Context 是抽象类，纯 JVM 造不出实例。
     */
    fun intentFor(
        context: android.content.Context,
        target: Class<*> = FlutterHostActivity::class.java,
    ): Intent = Intent(context, target)
}
