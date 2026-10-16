package dev.pocketagent.app.flutterhost

import io.flutter.embedding.engine.FlutterEngineCache
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertSame
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * [FlutterHostEngine] 的 JVM 单测(不建真引擎)。
 *
 * 【为什么这些用例能在纯 JVM 跑】本测试只覆盖「不依赖 libflutter.so」的部分:
 * 契约常量、缓存语义(含幂等/清理)。真 FlutterEngine 需要原生库,属真机验收
 * 范围(见 B4a 报告的证据节);通道壳的行为契约由
 * FlutterHostChannelShellTest 用假 BinaryMessenger 覆盖。
 *
 * 【为什么不用 Robolectric】本仓库单测是纯 JVM 风格(见同包其他 *Test.kt),
 * 不引 Robolectric;flutterhost 的可测部分按「纯逻辑 + 接口入参」组织,
 * 同样不需要 Android 框架。
 */
class FlutterHostEngineTest {

    @After
    fun tearDown() {
        // 缓存是进程级单例,测试必须清——否则用例间互相污染(下一个用例
        // 看到上一个留下的引擎),且本类是 object,状态跨测试类存活。
        FlutterHostEngine.shutdown()
        FlutterEngineCache.getInstance().remove(FlutterHostEngine.ENGINE_ID)
    }

    // ---------------- 契约常量(与 Dart 侧逐字一致) ----------------

    /**
     * 通道名是 wire 契约字符串:改一个字符,Dart 侧就整条桥静默失效
     * (方法面得 MissingPluginException、事件面永不来流),且**不会有任何编译
     * 错误提示**。故用单测钉死,与 flutter/lib/transport/pa_bridge.dart 的
     * kPaBridgeMethodChannel / kPaBridgeEventChannel 同源。
     */
    @Test
    fun `channel names match the B2 contract strings verbatim`() {
        assertEquals("dev.pocketagent/bridge", FlutterHostEngine.BRIDGE_METHOD_CHANNEL)
        assertEquals("dev.pocketagent/bridge/events", FlutterHostEngine.BRIDGE_EVENT_CHANNEL)
        assertEquals("dev.pocketagent/bridge/turn_events", FlutterHostEngine.BRIDGE_TURN_EVENT_CHANNEL)
        assertEquals("dev.pocketagent/bridge/terminal_events", FlutterHostEngine.BRIDGE_TERMINAL_EVENT_CHANNEL)
    }

    /**
     * 四个通道名两两不同(防复制粘贴改错一个导致两条通道撞名)。
     *
     * 撞名的表现是「后注册的覆盖先注册的」,某条流静默消失且无报错。
     */
    @Test
    fun `channel names are pairwise distinct`() {
        val names = listOf(
            FlutterHostEngine.BRIDGE_METHOD_CHANNEL,
            FlutterHostEngine.BRIDGE_EVENT_CHANNEL,
            FlutterHostEngine.BRIDGE_TURN_EVENT_CHANNEL,
            FlutterHostEngine.BRIDGE_TERMINAL_EVENT_CHANNEL,
        )
        assertEquals(names.size, names.toSet().size)
    }

    /**
     * Dart 入口名默认 "main":与 DartExecutor.DartEntrypoint.createDefault()
     * 内部写的函数名一致。改成别的名字而 Dart 侧没有对应
     * `@pragma('vm:entry-point')` 函数,冷启直接找不到入口——这也是要钉死的原因。
     */
    @Test
    fun `dart entrypoint defaults to main`() {
        assertEquals("main", FlutterHostEngine.DART_ENTRYPOINT)
    }

    /**
     * 初始路由必须为 null(交给 Dart 侧首屏分流)。
     *
     * 若这里写死 "/" 或任何具体路由,Kotlin 宿主就绕过了 main.dart 的
     * Onboarding/HomeShell 分流(与 OnboardingPrefs.isDone() 同语义的那套),
     * 表现为「新手教程不再出现」且没有任何报错。首屏决策权归 Dart,测试钉死。
     */
    @Test
    fun `initial route is null so dart owns the first screen`() {
        assertNull(FlutterHostEngine.INITIAL_ROUTE)
    }

    // ---------------- 缓存语义 ----------------

    /**
     * 未预热时 cachedEngine()/isWarmedUp() 不得为真,也不能有副作用
     * (不得为了「取一下」就真去起引擎)。
     */
    @Test
    fun `cached engine is null and warmup flag false before warmup`() {
        assertFalse(FlutterHostEngine.isWarmedUp())
        assertNull(FlutterHostEngine.cachedEngine())
    }

    /**
     * shutdown 幂等:重复调用不得抛(测试 tearDown 与真机排障都可能调)。
     */
    @Test
    fun `shutdown is idempotent when nothing was warmed up`() {
        FlutterHostEngine.shutdown()
        FlutterHostEngine.shutdown()
        assertFalse(FlutterHostEngine.isWarmedUp())
    }

    /**
     * ENGINE_ID 与 FlutterHostActivity.getCachedEngineId() 必须是同一个值。
     *
     * 【为什么这是硬约束】FlutterActivity 按 cached_engine_id 从
     * FlutterEngineCache 取引擎;两处字符串不一致 = Activity 每次都取不到 →
     * 回落到自建引擎(每次冷启一个新 VM),「预热」白做,而且**没有报错**。
     * 本测试钉「本侧不漂移」;跨类一致性由 FlutterHostActivityTest 的
     * `cached engine id matches the engine cache key` 验(那份读的是策略值)。
     */
    @Test
    fun `engine id is a stable namespaced key`() {
        assertEquals("dev.pocketagent.app/flutter_host_engine", FlutterHostEngine.ENGINE_ID)
        assertTrue(FlutterHostEngine.ENGINE_ID.startsWith("dev.pocketagent.app/"))
    }

    /**
     * cachedEngine() 在 shutdown 后回到 null(缓存真被清了,不是只清了本地引用)。
     *
     * 【为什么两条路都验】cachedEngine() 先读本地强引用、再读 FlutterEngineCache。
     * 只清一边的实现在此会暴露:表现为「shutdown 后 isWarmedUp 仍为真」,
     * 下一次 warmup 复用一个已 destroy 的引擎(直接崩)。
     */
    @Test
    fun `shutdown clears both the local reference and the engine cache`() {
        FlutterHostEngine.shutdown()
        assertNull(FlutterHostEngine.cachedEngine())
        assertFalse(FlutterEngineCache.getInstance().contains(FlutterHostEngine.ENGINE_ID))
        assertFalse(FlutterHostEngine.isWarmedUp())
    }

    /**
     * HOST_UNAVAILABLE 字面量与契约 §5 错误码表逐字一致。
     *
     * Dart 侧 `PaBridgeErrorCodes.HOST_UNAVAILABLE` 判的就是这个字符串;
     * 改一个字符,UI 的分支(「重试」vs「仅展示」)会静默走错支。
     */
    @Test
    fun `host unavailable code matches the contract literal`() {
        assertEquals("HOST_UNAVAILABLE", FlutterHostEngine.HOST_UNAVAILABLE)
    }

    /**
     * 壳的可测入口(installChannelShell)接受的入参是 BinaryMessenger 接口,
     * 不是 FlutterEngine 具体类——这是 flutterhost 测试能在纯 JVM 覆盖壳逻辑的
     * 前提。若签名退回 FlutterEngine,FlutterHostChannelShellTest 整段失效
     * (造不出引擎),故把签名本身钉住。
     */
    @Test
    fun `channel shell installer takes the messenger interface`() {
        val method = FlutterHostEngine::class.java.methods.firstOrNull {
            it.name == "installChannelShell"
        }
        assertNotNull("壳安装入口必须存在", method)
        assertEquals(
            "installChannelShell 必须接 BinaryMessenger(接口),否则 JVM 单测无法驱动",
            1,
            method!!.parameterTypes.size,
        )
        assertEquals(
            io.flutter.plugin.common.BinaryMessenger::class.java,
            method.parameterTypes[0],
        )
    }

    /**
     * 常量之间不自相矛盾:引擎 id 与包名同源(都是 dev.pocketagent.app 命名空间)。
     * 防止将来复制一份 id 时把命名空间写错(那会让缓存键与 app 的其它缓存混在一起)。
     */
    @Test
    fun `engine id shares the app namespace`() {
        assertNotNull(FlutterHostEngine.ENGINE_ID)
        assertTrue(
            "引擎 id 必须在 app 命名空间内",
            FlutterHostEngine.ENGINE_ID.startsWith("dev.pocketagent.app/"),
        )
        assertSame(FlutterHostEngine.ENGINE_ID, FlutterHostPolicy.cachedEngineId)
    }
}
