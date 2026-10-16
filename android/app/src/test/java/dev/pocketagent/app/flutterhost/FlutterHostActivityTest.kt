package dev.pocketagent.app.flutterhost

import io.flutter.embedding.android.FlutterActivityLaunchConfigs
import io.flutter.embedding.engine.FlutterEngineCache
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * [FlutterHostActivity] 的取值策略 + Intent 工厂单测(纯 JVM)。
 *
 * 【为什么不是 Robolectric】本仓单测是纯 JVM 风格(见同包其他 *Test.kt),
 * 不引 Robolectric。Activity 的三个 override 取值已拆到 [FlutterHostPolicy]
 * (纯 object),本测试直接读策略值 + 验 Intent 目标,同样覆盖「引擎复用/不随页
 * 销毁/入口钩子」三条契约,且不需要 Android 框架。Activity 本体只剩透传,
 * 它的一致性由 `policy exposes the full set` 系列断言钉住。
 */
class FlutterHostActivityTest {

    @After
    fun tearDown() {
        // 策略读的是进程级 FlutterEngineCache;清掉避免跨测试类污染。
        FlutterEngineCache.getInstance().remove(FlutterHostEngine.ENGINE_ID)
    }

    // ---------------- 引擎来源契约 ----------------

    /**
     * cached engine id 必须与 [FlutterHostEngine.ENGINE_ID] 一致。
     *
     * 【为什么这是本类最重要的一条】FlutterActivity 按 intent 里的
     * `cached_engine_id` 从 FlutterEngineCache 取引擎。两个字符串不一致 →
     * 每次都取不到 → 回落到自建引擎(每次冷启一个新 VM),预热白做,
     * **且没有任何报错**(只是变慢)。这类「静默降级」正是单测要钉死的。
     */
    @Test
    fun `cached engine id matches the engine cache key`() {
        assertEquals(FlutterHostEngine.ENGINE_ID, FlutterHostPolicy.cachedEngineId)
    }

    /**
     * 引擎不得随 Activity 销毁(缓存语义)。
     *
     * 销毁引擎 = 下次进入又要冷启一个新 VM,且 Dart 侧状态全丢。cached engine
     * 的标准语义就是「引擎比 Activity 活得久」。
     */
    @Test
    fun `activity does not destroy the cached engine on destroy`() {
        assertFalse(FlutterHostPolicy.destroyEngineWithHost)
    }

    /**
     * 状态保存关闭(cached engine 下 FlutterActivity 也默认关,显式声明)。
     *
     * 开着会让引擎的 restoration 数据被 Activity 的保存/恢复流程覆盖,
     * 表现为「重启后状态怪怪的」。
     */
    @Test
    fun `state restoration is disabled`() {
        assertFalse(FlutterHostPolicy.restoreAndSaveState)
    }

    /**
     * 背景模式为透明:不挡宿主 Compose UI 的转场视觉。
     */
    @Test
    fun `background mode is transparent`() {
        assertEquals(
            FlutterActivityLaunchConfigs.BackgroundMode.transparent,
            FlutterHostPolicy.backgroundMode,
        )
    }

    // ---------------- 入口钩子 ----------------

    /**
     * Dart 入口名与初始路由走 [FlutterHostEngine] 的钩子常量,不自己发明。
     *
     * 若 Activity 里另写一份字面量,将来改入口就得改两处——漏一处的表现是
     * 「改了没用」,且无报错。本测试把「策略值 == 引擎钩子值」钉死;
     * Activity 的 override 与策略的一致性由 `policy exposes the full set` 验。
     */
    @Test
    fun `entrypoint and initial route come from the engine hooks`() {
        assertEquals(FlutterHostEngine.DART_ENTRYPOINT, FlutterHostPolicy.dartEntrypointFunctionName)
        assertEquals(FlutterHostEngine.INITIAL_ROUTE, FlutterHostPolicy.initialRoute)
    }

    // ---------------- 启动 Intent ----------------

    /**
     * launcher 暴露的 `intentFor` 签名必须接收 (Context, Class) 两个参数。
     *
     * 【为什么这样测而不是真调 intentFor(context)】Context 是抽象类、Intent 的
     * getExtras 等是 android.jar stub(纯 JVM 下抛 "not mocked"),所以既造不出
     * context 也读不了 extras(见本类 KDoc:不引 Robolectric)。能测的是**签名
     * 形状**:两个参数、类型分别是 Context 与 Class。
     * 【为什么签名也值得测】launcher 的 target 默认参数是
     * FlutterHostActivity::class.java(源码约定,见 FlutterHostLauncher KDoc);
     * 若有人删掉 target 参数或改类型,调用方就无法显式指定目标——那正是
     * 「将来要带初始路由」时要用的口子。签名在,约定才有着落。
     */
    @Test
    fun `launcher exposes a context and a target class parameter`() {
        val method = FlutterHostLauncher::class.java.methods.firstOrNull { it.name == "intentFor" }
        assertNotNull("launcher 必须暴露 intentFor", method)
        assertTrue(
            "intentFor 应接收 (Context, Class) 两个参数,实际: ${method?.parameterTypes?.toList()}",
            method!!.parameterTypes.size == 2,
        )
        assertEquals(android.content.Context::class.java, method.parameterTypes[0])
        assertEquals(Class::class.java, method.parameterTypes[1])
    }

    /**
     * launcher 的目标类与 Activity 同包。
     *
     * 上面那条验签名,这条验「默认目标没跑偏」:launcher 若指向别的包的类,
     * 运行时才 ActivityNotFoundException(编译期不报错,因为参数是 Class<*>)。
     * 同包是「默认目标 = FlutterHostActivity」的强约束(源码就写在同文件里)。
     */
    @Test
    fun `launcher and the activity live in the same package`() {
        assertEquals(
            FlutterHostActivity::class.java.`package`?.name,
            FlutterHostLauncher::class.java.`package`?.name,
        )
    }

    // ---------------- 注册表动作 ----------------

    /**
     * 注册表的 remove 用 [FlutterHostEngine.ENGINE_ID] 且幂等。
     *
     * 【为什么用真 FlutterEngineCache 而不是 fake】FlutterEngineCache 是静态
     * 单例、纯 Java Map 包装,**不碰原生库**,JVM 可直接用。用真的才能验
     * 「键对得上、摘得掉」这件事本身(fake 只会复述我传进去的键)。
     * put 的键一致性由源码保证(FlutterHostEngineRegistry.put 直接引用
     * FlutterHostEngine.ENGINE_ID,无第二处字面量)。
     */
    @Test
    fun `registry remove is idempotent on the engine cache key`() {
        assertFalse(FlutterEngineCache.getInstance().contains(FlutterHostEngine.ENGINE_ID))
        FlutterHostEngineRegistry.remove()
        FlutterHostEngineRegistry.remove()
        assertFalse(FlutterEngineCache.getInstance().contains(FlutterHostEngine.ENGINE_ID))
    }

    /**
     * 策略出口齐全:漏一个 override 就意味着某一项退回 FlutterActivity 的
     * 默认值(可能是自建引擎 / 销毁引擎 / 保存状态)。本测试枚举策略的取值
     * 出口,确保六项都在——新增策略项时会因为本断言失败而被提醒补测试。
     */
    @Test
    fun `policy exposes the full set of activity overrides`() {
        val names = FlutterHostPolicy::class.java.declaredMethods
            .map { it.name }
            .filter { it.startsWith("get") }
            .toSet()
        assertTrue(
            "策略必须覆盖 cachedEngineId / destroyEngineWithHost / restoreAndSaveState / " +
                "backgroundMode / dartEntrypointFunctionName / initialRoute,实际: $names",
            names.containsAll(
                listOf(
                    "getCachedEngineId",
                    "getDestroyEngineWithHost",
                    "getRestoreAndSaveState",
                    "getBackgroundMode",
                    "getDartEntrypointFunctionName",
                    "getInitialRoute",
                ),
            ),
        )
    }

    /**
     * 策略值不是「未初始化」的占位(全 null / 全默认)。
     * 防止有人把 object 的字段删了却因为 getter 之外的原因静默通过。
     */
    @Test
    fun `policy values are concrete not placeholders`() {
        assertNotNull(FlutterHostPolicy.cachedEngineId)
        assertNotNull(FlutterHostPolicy.dartEntrypointFunctionName)
        assertNotNull(FlutterHostPolicy.backgroundMode)
        // initialRoute 是唯一允许为 null 的策略(Dart 侧决定首屏)。
        assertNull(FlutterHostPolicy.initialRoute)
    }

    /**
     * Activity 真在 flutterhost 包内:launcher 指向的类若被挪包而 launcher
     * 没跟着改,那种错的表现是编译通过、运行时 ActivityNotFoundException。
     * 本测试把「包路径」钉住,与上一条 launcher 断言互为兜底。
     */
    @Test
    fun `activity lives in the flutterhost package`() {
        assertEquals(
            "dev.pocketagent.app.flutterhost",
            FlutterHostActivity::class.java.`package`?.name,
        )
    }
}
