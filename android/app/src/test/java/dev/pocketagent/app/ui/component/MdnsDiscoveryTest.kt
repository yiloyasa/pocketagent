package dev.pocketagent.app.ui.component

import dev.pocketagent.app.ui.component.MdnsDiscovery.Companion.lanPrefixOf
import dev.pocketagent.app.ui.component.MdnsDiscovery.Companion.sortedByLan
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.withTimeout
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.util.concurrent.atomic.AtomicLong

/**
 * P0-3 MdnsDiscovery 核心逻辑单测(纯 JVM)。
 *
 * MdnsDiscovery 只依赖 [MdnsEngine] 抽象 —— 这里用 fake 引擎精确驱动
 * 发现/解析/丢失/失败回调,覆盖去重、过期、排序、resolve 重试与环境降级,
 * 不需要真机与 NsdManager。(核心类不触碰 android.* 类型,这是可测的前提。)
 */
class MdnsDiscoveryTest {

    /** 可控 fake 引擎:记录调用,回调句柄交测试手动触发。 */
    private class FakeEngine : MdnsEngine {
        val callbacks = LinkedHashMap<String, MdnsEngineCallbacks>() // serviceType → callbacks
        var startCalls = 0
        var stopCalls = 0
        val resolveRequests = mutableListOf<Pair<String, MdnsEngineCallbacks>>()
        var startResult = true
        var resolveBehavior: ((String, MdnsEngineCallbacks) -> Unit)? = null

        override fun startDiscovery(serviceType: String, callbacks: MdnsEngineCallbacks): Boolean {
            startCalls++
            this.callbacks[serviceType] = callbacks
            return startResult
        }

        override fun stopDiscovery() {
            stopCalls++
        }

        override fun resolve(serviceName: String, callbacks: MdnsEngineCallbacks) {
            resolveRequests.add(serviceName to callbacks)
            resolveBehavior?.invoke(serviceName, callbacks)
        }

        fun cb(serviceType: String = MdnsDiscovery.SERVICE_TYPE): MdnsEngineCallbacks = callbacks.getValue(serviceType)
    }

    private fun discovery(
        engine: FakeEngine,
        clock: AtomicLong,
        lanPrefix: String? = null,
    ): MdnsDiscovery = MdnsDiscovery(
        engine = engine,
        scope = CoroutineScope(SupervisorJob() + Dispatchers.Default),
        nowMs = { clock.get() },
        lanPrefixProvider = { lanPrefix },
        serviceType = MdnsDiscovery.SERVICE_TYPE,
    )

    private fun resolved(engine: FakeEngine, name: String, host: String, port: Int, txt: Map<String, String> = mapOf("id" to "host-1")) {
        engine.cb().onFound(name)
        engine.cb().onResolved(name, host, port, txt)
    }

    @Test
    fun `resolved host is published with txt fields`() {
        val engine = FakeEngine()
        val d = discovery(engine, AtomicLong(1_000L))
        d.start()
        engine.cb().onFound("办公电脑")
        engine.cb().onResolved(
            "办公电脑", "192.168.1.7", 7246,
            mapOf("v" to "1", "id" to "host-abc", "name" to "My PC", "tls" to "1"),
        )
        val hosts = d.hosts.value
        assertEquals(1, hosts.size)
        val h = hosts.first()
        assertEquals("办公电脑", h.instanceName)
        assertEquals("192.168.1.7", h.host)
        assertEquals(7246, h.port)
        assertEquals("host-abc", h.hostId)
        assertEquals("My PC", h.displayName)
        assertTrue(h.tls)
        assertEquals("192.168.1.7:7246", h.address())
    }

    @Test
    fun `duplicate resolutions for same instance are deduplicated`() {
        val engine = FakeEngine()
        val d = discovery(engine, AtomicLong(1_000L))
        d.start()
        resolved(engine, "pc", "192.168.1.7", 7246)
        resolved(engine, "pc", "192.168.1.8", 7246) // 同实例多网卡地址刷新,不能出现两条
        assertEquals(1, d.hosts.value.size)
        assertEquals("192.168.1.8", d.hosts.value.first().host)
    }

    @Test
    fun `found without resolution is hidden until host and port known`() {
        val engine = FakeEngine()
        val d = discovery(engine, AtomicLong(1_000L))
        d.start()
        engine.cb().onFound("pc") // 还没 resolve 出 IP/端口
        assertTrue(d.hosts.value.isEmpty())
        assertTrue(d.current().isEmpty())
        engine.cb().onResolved("pc", "192.168.1.7", 7246, emptyMap())
        assertEquals(1, d.hosts.value.size)
    }

    @Test
    fun `service lost removes host immediately`() {
        val engine = FakeEngine()
        val d = discovery(engine, AtomicLong(1_000L))
        d.start()
        resolved(engine, "pc", "192.168.1.7", 7246)
        assertEquals(1, d.hosts.value.size)
        engine.cb().onLost("pc")
        assertTrue(d.hosts.value.isEmpty())
        assertTrue(d.current().isEmpty())
    }

    @Test
    fun `hosts expire after five minutes`() {
        val clock = AtomicLong(1_000L)
        val engine = FakeEngine()
        val d = discovery(engine, clock)
        d.start()
        resolved(engine, "pc", "192.168.1.7", 7246)
        assertEquals(1, d.current().size)
        clock.set(1_000L + MdnsDiscovery.EXPIRE_MS + 1) // 推进到过期线之后
        assertTrue("5 分钟未刷新的记录必须从候选中消失", d.current().isEmpty())
    }

    @Test
    fun `tls missing defaults to true and tls zero means plaintext`() {
        val engine = FakeEngine()
        val d = discovery(engine, AtomicLong(1_000L))
        d.start()
        resolved(engine, "a", "192.168.1.7", 7246, txt = emptyMap())
        resolved(engine, "b", "192.168.1.9", 7246, txt = mapOf("tls" to "0"))
        val byName = d.hosts.value.associateBy { it.instanceName }
        assertTrue(byName.getValue("a").tls) // 旧版 daemon 没有 tls 键 → 按 TLS 处理
        assertFalse(byName.getValue("b").tls)
    }

    @Test
    fun `resolve failure is retried with backoff until success`() = runBlocking {
        val engine = FakeEngine()
        var failFirst = true
        engine.resolveBehavior = { name, cb ->
            if (failFirst) {
                failFirst = false
                cb.onResolveFailed(name)
            } else {
                cb.onResolved(name, "192.168.1.7", 7246, emptyMap())
            }
        }
        val d = discovery(engine, AtomicLong(1_000L))
        d.start()
        engine.cb().onFound("pc")
        // 第一次 resolve 立即失败 → ~1s 后自动重试 → 第二次成功
        withTimeout(5_000) {
            while (d.hosts.value.isEmpty()) kotlinx.coroutines.delay(100)
        }
        assertTrue("resolve 失败后应自动重试", engine.resolveRequests.size >= 2)
    }

    @Test
    fun `start returns false and does not throw when engine unavailable`() {
        val engine = FakeEngine().apply { startResult = false }
        val d = discovery(engine, AtomicLong(1_000L))
        assertFalse(d.start())
        assertTrue(d.hosts.value.isEmpty())
    }

    @Test
    fun `stop calls engine and is idempotent`() {
        val engine = FakeEngine()
        val d = discovery(engine, AtomicLong(1_000L))
        d.start()
        d.stop()
        d.stop()
        assertTrue(engine.stopCalls >= 1)
    }

    @Test
    fun `same segment hosts sort first`() {
        val a = host("a", "10.0.0.5", seen = 1)
        val b = host("b", "192.168.1.7", seen = 2)
        val c = host("c", "192.168.1.9", seen = 3)
        val sorted = sortedByLan(listOf(a, b, c), "192.168.1")
        assertEquals(listOf("b", "c", "a"), sorted.map { it.instanceName })
        // 无本机前缀时保持原顺序
        assertEquals(listOf("a", "b", "c"), sortedByLan(listOf(a, b, c), null).map { it.instanceName })
    }

    @Test
    fun `lan prefix parses ipv4 and rejects garbage`() {
        assertEquals("192.168.1", lanPrefixOf("192.168.1.7"))
        assertEquals("10.0.50", lanPrefixOf("10.0.50.3"))
        assertNull(lanPrefixOf(""))
        assertNull(lanPrefixOf("abc"))
        assertNull(lanPrefixOf("1.2.3"))
    }

    private fun host(name: String, ip: String, seen: Long) = DiscoveredHost(
        instanceName = name, host = ip, port = 7246,
        hostId = "h-$name", displayName = name, tls = true, seenAtMs = seen,
    )
}
