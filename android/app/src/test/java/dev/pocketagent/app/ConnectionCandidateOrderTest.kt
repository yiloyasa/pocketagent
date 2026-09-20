package dev.pocketagent.app

import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import java.net.ServerSocket

/**
 * PA-REAL-0006(NET-01)回归:USB 反向隧道候选实际可达时必须优先于 lastGoodAddr;
 * 不可达时维持既有语义(lastGoodAddr 优先)。排序规则单一事实源是
 * [ConnectionCandidateOrder](纯函数),可达性探测抽象为 [UsbReachabilityProbe] 接口 mock。
 */
class ConnectionCandidateOrderTest {

    // ============ 纯函数:两种排序语义钉死 ============

    @Test
    fun `usb reachable outranks lastGoodAddr`() {
        val ordered = ConnectionCandidateOrder.order(
            usbCandidate = "127.0.0.1:7246",
            usbReachable = true,
            lastGoodAddr = "10.15.50.160:7246",
            others = listOf("192.168.1.7:7246", "192.168.1.9:7246", "100.64.1.2:7246"),
        )
        assertEquals(
            "USB 可达 → USB 胜,lastGoodAddr 与其余候选相对顺序保持不变",
            listOf(
                "127.0.0.1:7246",
                "10.15.50.160:7246",
                "192.168.1.7:7246",
                "192.168.1.9:7246",
                "100.64.1.2:7246",
            ),
            ordered,
        )
    }

    @Test
    fun `usb unreachable falls behind lastGoodAddr`() {
        val ordered = ConnectionCandidateOrder.order(
            usbCandidate = "127.0.0.1:7246",
            usbReachable = false,
            lastGoodAddr = "10.15.50.160:7246",
            others = listOf("192.168.1.7:7246", "192.168.1.9:7246", "100.64.1.2:7246"),
        )
        assertEquals(
            "USB 不可达 → lastGoodAddr 胜,USB 紧随其后(回环毫秒级快失败,兜底不丢)",
            listOf(
                "10.15.50.160:7246",
                "127.0.0.1:7246",
                "192.168.1.7:7246",
                "192.168.1.9:7246",
                "100.64.1.2:7246",
            ),
            ordered,
        )
    }

    @Test
    fun `unreachable usb with blank lastGoodAddr keeps usb first`() {
        val ordered = ConnectionCandidateOrder.order(
            usbCandidate = "127.0.0.1:7246",
            usbReachable = false,
            lastGoodAddr = "",
            others = listOf("192.168.1.7:7246"),
        )
        assertEquals(
            "首次安装(无 lastGoodAddr)维持既有队列:USB 仍在首位",
            listOf("127.0.0.1:7246", "192.168.1.7:7246"),
            ordered,
        )
    }

    @Test
    fun `blank usb candidate is dropped from queue`() {
        val ordered = ConnectionCandidateOrder.order(
            usbCandidate = null,
            usbReachable = true,
            lastGoodAddr = "10.15.50.160:7246",
            others = listOf("192.168.1.7:7246"),
        )
        assertEquals(listOf("10.15.50.160:7246", "192.168.1.7:7246"), ordered)
    }

    @Test
    fun `blanks filtered and duplicates collapsed`() {
        val ordered = ConnectionCandidateOrder.order(
            usbCandidate = "127.0.0.1:7246",
            usbReachable = true,
            lastGoodAddr = "192.168.1.7:7246",
            others = listOf("", "192.168.1.7:7246", "100.64.1.2:7246"),
        )
        assertEquals(
            listOf("127.0.0.1:7246", "192.168.1.7:7246", "100.64.1.2:7246"),
            ordered,
        )
    }

    // ============ 生产探测器:回环 TCP 真连(不碰外部网络) ============

    @Test
    fun `socket probe reports listening loopback port as reachable`() {
        ServerSocket(0, 1, java.net.InetAddress.getByName("127.0.0.1")).use { server ->
            assertTrue(
                SocketUsbReachabilityProbe.isReachable(
                    "127.0.0.1:${server.localPort}", 1000L,
                ),
            )
        }
    }

    @Test
    fun `socket probe reports closed loopback port as unreachable`() {
        val free = ServerSocket(0, 1, java.net.InetAddress.getByName("127.0.0.1"))
        val port = free.localPort
        free.close()
        assertFalse(
            SocketUsbReachabilityProbe.isReachable("127.0.0.1:$port", 500L),
        )
    }

    // ============ AgentConnection 接线:mock 探测器走真实 candidateAddresses ============

    private var savedLan = ""
    private var savedTs = ""
    private var savedManual = ""
    private var savedLastGood = ""

    @Before
    fun setUp() {
        savedLan = AgentConnection.lastLanAddr
        savedTs = AgentConnection.lastTailscaleAddr
        savedManual = AgentConnection.manualAddress
        savedLastGood = AgentConnection.lastGoodAddrForTest()
        // 本组用例不涉及 mDNS;显式置空,避免同 JVM 其他用例残留的发现器影响断言。
        AgentConnection.setMdnsDiscoveryForTest(null)
        // 全量归零后由各用例自行设定,保证与执行顺序无关。
        AgentConnection.lastLanAddr = ""
        AgentConnection.lastTailscaleAddr = ""
        AgentConnection.manualAddress = ""
        AgentConnection.setLastGoodAddrForTest("")
    }

    @After
    fun tearDown() {
        AgentConnection.lastLanAddr = savedLan
        AgentConnection.lastTailscaleAddr = savedTs
        AgentConnection.manualAddress = savedManual
        AgentConnection.setLastGoodAddrForTest(savedLastGood)
        AgentConnection.setMdnsDiscoveryForTest(null)
    }

    @Test
    fun `wiring - usb reachable beats persisted lastGoodAddr`() {
        AgentConnection.lastLanAddr = "10.15.50.160:7246"
        // 手填地址与局域网同址(manual 空白时会回退到默认 address,这里显式给值以便精确断言)
        AgentConnection.manualAddress = "10.15.50.160:7246"
        AgentConnection.setLastGoodAddrForTest("10.15.50.160:7246")

        val probedAddr = mutableListOf<String>()
        val cands = AgentConnection.candidateAddresses(usbProbe = UsbReachabilityProbe { addr, _ ->
            probedAddr.add(addr)
            true
        })

        assertEquals("探测对象是 USB 反向隧道候选", listOf("127.0.0.1:7246"), probedAddr)
        assertEquals(
            listOf("127.0.0.1:7246", "10.15.50.160:7246"),
            cands,
        )
    }

    @Test
    fun `wiring - usb unreachable keeps lastGoodAddr first and usb as fallback`() {
        AgentConnection.lastLanAddr = "10.15.50.160:7246"
        AgentConnection.manualAddress = "10.15.50.160:7246"
        AgentConnection.setLastGoodAddrForTest("10.15.50.160:7246")

        val cands = AgentConnection.candidateAddresses(usbProbe = UsbReachabilityProbe { _, _ -> false })

        assertEquals(
            "不可达:上次成功地址居首,USB 仍保留在队列(快失败兜底)",
            listOf("10.15.50.160:7246", "127.0.0.1:7246"),
            cands,
        )
    }

    @Test
    fun `wiring - no probe keeps legacy queue with usb first`() {
        AgentConnection.lastLanAddr = "10.0.0.5:7246"
        AgentConnection.lastTailscaleAddr = "100.64.1.2:7246"
        AgentConnection.manualAddress = "192.168.1.9:7246"
        AgentConnection.setLastGoodAddrForTest("10.0.0.5:7246")

        val cands = AgentConnection.candidateAddresses()

        assertEquals(
            "不传探测器(可达性未知)维持既有顺序:USB 首位,manual 先于 Tailscale",
            listOf("127.0.0.1:7246", "10.0.0.5:7246", "192.168.1.9:7246", "100.64.1.2:7246"),
            cands,
        )
    }
}
