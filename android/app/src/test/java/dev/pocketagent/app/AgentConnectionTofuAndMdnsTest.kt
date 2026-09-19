package dev.pocketagent.app

import dev.pocketagent.app.ui.component.DiscoveredHost
import dev.pocketagent.app.ui.component.MdnsDiscovery
import dev.pocketagent.app.ui.component.MdnsEngine
import dev.pocketagent.app.ui.component.MdnsEngineCallbacks
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicInteger

/**
 * P0-3 安全与候选注入单测(纯 JVM):
 *
 * 1. TOFU 竞速防护(修阶段 A 遗留风险):未存指纹必须串行(USB 先试,失败再逐个,
 *    避免"同网络第二台主机抢答导致指纹锁错");已存指纹保持并行竞速。
 *    锁定点是 [AgentConnection.serialModeForTofu] 与 [CandidateProbe.run] 的调度行为。
 * 2. mDNS 发现主机注入候选链:USB → mDNS(同网段优先) → 历史 lanAddr → Tailscale → 手动,
 *    mDNS 与历史 lanAddr 同 IP 去重;发现消失(过期/离线)后历史候选自动恢复。
 */
class AgentConnectionTofuAndMdnsTest {

    @Before
    fun setUp() {
        savedLan = AgentConnection.lastLanAddr
        savedTs = AgentConnection.lastTailscaleAddr
        savedManual = AgentConnection.manualAddress
        AgentConnection.setPinStoreForTest(MemoryPinStore())
        AgentConnection.setMdnsDiscoveryForTest(null)
    }

    @After
    fun tearDown() {
        AgentConnection.lastLanAddr = savedLan
        AgentConnection.lastTailscaleAddr = savedTs
        AgentConnection.manualAddress = savedManual
        AgentConnection.setPinStoreForTest(MemoryPinStore())
        AgentConnection.setMdnsDiscoveryForTest(null)
    }

    private var savedLan = ""
    private var savedTs = ""
    private var savedManual = ""

    // ---------------- TOFU 串/并行判定 ----------------

    @Test
    fun `unpinned device uses serial probing`() {
        AgentConnection.setPinStoreForTest(MemoryPinStore(null)) // 从未配对过
        assertTrue("首次配对(无指纹)必须串行,防止锁错主机", AgentConnection.serialModeForTofu())
    }

    @Test
    fun `pinned device keeps parallel racing`() {
        AgentConnection.setPinStoreForTest(MemoryPinStore("a".repeat(64))) // 已锁定指纹
        assertFalse("已有指纹时指纹校验会拒绝错主机,应保持并行竞速", AgentConnection.serialModeForTofu())
    }

    // ---------------- CandidateProbe 调度行为 ----------------

    @Test
    fun `serial probe tries in priority order and stops at first success`() {
        val called = mutableListOf<String>()
        val outcome = CandidateProbe.run(
            cands = listOf("usb", "mdns", "lan"),
            serial = true,
            timeoutOf = { 100L },
            budgetMs = 1_000L,
            probe = { addr, _ ->
                called.add(addr)
                if (addr == "usb") throw RuntimeException("refused")
                if (addr == "mdns") "OK-$addr" else throw RuntimeException("should not be probed")
            },
        )
        assertEquals("失败后按优先级逐个继续:USB→mDNS", listOf("usb", "mdns"), called)
        assertEquals("mdns" to "OK-mdns", outcome.winner)
        // lan 未被探测(第一个成功即短路)
        assertTrue("lan 不该被探测", "lan" !in called)
    }

    @Test
    fun `serial probe reports head failure when all candidates fail`() {
        val outcome = CandidateProbe.run(
            cands = listOf("head", "tail"),
            serial = true,
            timeoutOf = { 100L },
            budgetMs = 1_000L,
            probe = { addr, _ -> throw RuntimeException("fail-$addr") },
        )
        assertNull(outcome.winner)
        assertEquals("fail-head", outcome.firstFailure?.message)
    }

    @Test
    fun `race probe starts all candidates concurrently`() {
        // 汇聚栅栏:第 3 个进入 probe 的线程放行所有等待者。
        // 只有 3 个候选真正"同时在场"才能凑齐 3 → 直接证明并行起跑
        // (串行模式下第 2/3 个要等前一个失败才会被调,栅栏永远凑不齐,budget 超时 winner=null)。
        val arrived = AtomicInteger(0)
        val go = CountDownLatch(1)
        val outcome = CandidateProbe.run(
            cands = listOf("usb", "mdns", "lan"),
            serial = false,
            timeoutOf = { 100L },
            budgetMs = 5_000L,
            probe = { addr, _ ->
                if (arrived.incrementAndGet() == 3) go.countDown()
                assertTrue("汇聚栅栏应在 5s 内打开(全部候选并行在场)", go.await(5, TimeUnit.SECONDS))
                if (addr != "mdns") throw RuntimeException("fail-$addr")
                "OK"
            },
        )
        assertEquals("全部 3 个候选都被并发调起", 3, arrived.get())
        assertEquals("mdns" to "OK", outcome.winner)
        // firstFailure 只在"无胜者"路径被生产代码消费(见 without winner 用例)。
        // 胜者栅栏打开时队首候选的失败 CAS 可能尚未执行,此处只容忍式校验,
        // 不对生产从不依赖的时序窗口做硬断言(曾因竞速窗口恒挂)。
        val ff = outcome.firstFailure
        assertTrue(
            "胜者存在时 firstFailure 要么未及记录(null)要么是队首失败",
            ff == null || ff.message == "fail-usb",
        )
    }

    @Test
    fun `race probe without winner reports head failure`() {
        val outcome = CandidateProbe.run(
            cands = listOf("head", "second"),
            serial = false,
            timeoutOf = { 50L },
            budgetMs = 500L,
            probe = { addr, _ -> throw RuntimeException("boom-$addr") },
        )
        assertNull(outcome.winner)
        assertEquals("boom-head", outcome.firstFailure?.message)
    }

    @Test
    fun `empty candidates short circuit`() {
        val outcome = CandidateProbe.run<String>(
            cands = emptyList(), serial = true, timeoutOf = { 0L }, budgetMs = 0L,
            probe = { _, _ -> error("never") },
        )
        assertNull(outcome.winner)
        assertNull(outcome.firstFailure)
    }

    // ---------------- mDNS 候选注入 ----------------

    /** 轻量 fake 引擎:回调句柄交测试手动触发,驱动真实回调链路。 */
    private class StubEngine : MdnsEngine {
        var callbacks: MdnsEngineCallbacks? = null

        override fun startDiscovery(serviceType: String, callbacks: MdnsEngineCallbacks): Boolean {
            this.callbacks = callbacks
            return true
        }

        override fun stopDiscovery() {}

        override fun resolve(serviceName: String, callbacks: MdnsEngineCallbacks) {}
    }

    private fun discovered(vararg hosts: DiscoveredHost): MdnsDiscovery {
        val engine = StubEngine()
        val d = MdnsDiscovery(
            engine = engine,
            scope = CoroutineScope(SupervisorJob() + Dispatchers.Default),
            nowMs = { System.currentTimeMillis() },
            lanPrefixProvider = { null },
        )
        d.start()
        // 通过真实回调链路注入:Found → Resolved,保证走的是生产路径。
        hosts.forEach { h ->
            engine.callbacks?.onFound(h.instanceName)
            engine.callbacks?.onResolved(
                h.instanceName, h.host, h.port,
                buildMap {
                    if (h.hostId.isNotBlank()) put("id", h.hostId)
                    if (!h.tls) put("tls", "0")
                },
            )
        }
        return d
    }

    private fun mdnsHost(name: String, ip: String) = DiscoveredHost(
        instanceName = name, host = ip, port = 7246,
        hostId = "host-x", displayName = name, tls = true,
        seenAtMs = System.currentTimeMillis(),
    )

    @Test
    fun `mdns hosts are injected between usb and lan candidates`() {
        AgentConnection.lastLanAddr = "192.168.1.7:7246"
        AgentConnection.lastTailscaleAddr = "100.64.1.2:7246"
        AgentConnection.manualAddress = "10.9.9.9:7246"
        AgentConnection.setMdnsDiscoveryForTest(discovered(mdnsHost("pc", "10.15.50.160")))

        val cands = AgentConnection.candidateAddresses()

        assertEquals(
            listOf(
                "127.0.0.1:7246",    // USB
                "10.15.50.160:7246", // mDNS 发现
                "192.168.1.7:7246",  // 历史局域网(与 mDNS 不同 IP,保留)
                "10.9.9.9:7246",     // manual LAN also precedes Tailscale
                "100.64.1.2:7246",   // Tailscale fallback
            ),
            cands,
        )
    }

    @Test
    fun `mdns result deduplicates against historical lan address`() {
        AgentConnection.lastLanAddr = "192.168.1.7:7246" // 历史 daemon 上报
        AgentConnection.manualAddress = "10.9.9.9:7246"
        AgentConnection.setMdnsDiscoveryForTest(discovered(mdnsHost("pc", "192.168.1.7")))

        val cands = AgentConnection.candidateAddresses()

        assertEquals("mDNS 与历史 lanAddr 同 IP 只保留 mDNS 的一条",
            listOf("127.0.0.1:7246", "192.168.1.7:7246", "10.9.9.9:7246"), cands)
    }

    @Test
    fun `historical lan candidate returns when mdns host disappears`() {
        AgentConnection.lastLanAddr = "192.168.1.7:7246"
        AgentConnection.manualAddress = "10.9.9.9:7246"
        val engine = StubEngine()
        val d = MdnsDiscovery(
            engine = engine,
            scope = CoroutineScope(SupervisorJob() + Dispatchers.Default),
            nowMs = { System.currentTimeMillis() },
            lanPrefixProvider = { null },
        )
        d.start()
        engine.callbacks?.onFound("pc")
        engine.callbacks?.onResolved("pc", "10.15.50.160", 7246, emptyMap())
        AgentConnection.setMdnsDiscoveryForTest(d)
        assertTrue(
            "mDNS 主机在候选中(USB 之后)",
            AgentConnection.candidateAddresses().contains("10.15.50.160:7246"),
        )

        engine.callbacks?.onLost("pc") // daemon 关机 → 发现消失
        val cands = AgentConnection.candidateAddresses()
        assertTrue(
            "发现的主机消失后自动移出候选",
            !cands.contains("10.15.50.160:7246"),
        )
        assertTrue(
            "历史 lanAddr 恢复进候选",
            cands.contains("192.168.1.7:7246"),
        )
    }

    @Test
    fun `no mdns discovery keeps legacy candidate order`() {
        AgentConnection.lastLanAddr = "10.0.0.5:7246"
        AgentConnection.lastTailscaleAddr = "100.64.1.2:7246"
        AgentConnection.manualAddress = "192.168.1.9:7246"

        assertEquals(
            listOf("127.0.0.1:7246", "10.0.0.5:7246", "192.168.1.9:7246", "100.64.1.2:7246"),
            AgentConnection.candidateAddresses(),
        )
    }
}
