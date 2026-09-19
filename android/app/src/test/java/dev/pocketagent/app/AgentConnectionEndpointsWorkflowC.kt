package dev.pocketagent.app

import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

/**
 * 项4(连接层)回归测试:连接端点列表的排序 / active / 完整性 / 宁缺勿假,
 * 以及 transportLabel 的向后兼容输出。全部用可注入的假地址,不依赖真实网络。
 */
class AgentConnectionEndpointsWorkflowC {

    private var savedLan = ""
    private var savedTs = ""
    private var savedManual = ""
    private var savedConn = ""

    @Before
    fun setUp() {
        savedLan = AgentConnection.lastLanAddr
        savedTs = AgentConnection.lastTailscaleAddr
        savedManual = AgentConnection.manualAddress
        savedConn = AgentConnection.connAddress.value
    }

    @After
    fun tearDown() {
        AgentConnection.lastLanAddr = savedLan
        AgentConnection.lastTailscaleAddr = savedTs
        AgentConnection.manualAddress = savedManual
        AgentConnection.connAddress.value = savedConn
    }

    @Test
    fun endpointsAreOrderedUsbLanTailscaleManual() {
        AgentConnection.lastLanAddr = "192.168.1.7:7246"
        AgentConnection.lastTailscaleAddr = "100.64.0.9:7246"
        AgentConnection.manualAddress = "10.0.2.2:7246"
        AgentConnection.connAddress.value = ""

        val eps = AgentConnection.endpoints()
        assertEquals(listOf("usb", "lan", "tailscale", "manual"), eps.map { it.id })
        assertEquals("127.0.0.1:7246", eps[0].addr)
        assertEquals("192.168.1.7:7246", eps[1].addr)
        assertEquals("100.64.0.9:7246", eps[2].addr)
        assertEquals("10.0.2.2:7246", eps[3].addr)
    }

    @Test
    fun everyEndpointAddrIsFullIpPort() {
        AgentConnection.lastLanAddr = "192.168.1.7:7246"
        AgentConnection.lastTailscaleAddr = "100.64.0.9:7246"
        AgentConnection.manualAddress = "10.0.2.2:7246"
        val eps = AgentConnection.endpoints()
        assertTrue("列表不能为空", eps.isNotEmpty())
        assertTrue("地址必须是完整 ip:port", eps.all { it.addr.contains(':') && it.addr.isNotBlank() })
        assertTrue(
            "USB 的 detail 必须给出 adb reverse 可执行提示",
            eps.first { it.id == "usb" }.detail.contains("adb reverse"),
        )
    }

    @Test
    fun unavailableMethodsAreOmittedNotFaked() {
        AgentConnection.lastLanAddr = ""
        AgentConnection.lastTailscaleAddr = ""
        AgentConnection.manualAddress = ""
        val ids = AgentConnection.endpoints().map { it.id }
        assertTrue("USB 恒在", ids.contains("usb"))
        assertFalse("未上报局域网 IP 时不得塞空地址", ids.contains("lan"))
        assertFalse("未上报 Tailscale 时不得塞空地址", ids.contains("tailscale"))
    }

    @Test
    fun activeFlagFollowsCurrentAddress() {
        AgentConnection.lastLanAddr = "192.168.1.7:7246"
        AgentConnection.lastTailscaleAddr = "100.64.0.9:7246"
        AgentConnection.manualAddress = "10.0.2.2:7246"
        AgentConnection.connAddress.value = "100.64.0.9:7246"
        val eps = AgentConnection.endpoints()
        assertEquals("tailscale", eps.first { it.active }.id)
        assertEquals("同一时刻只有一个 active", 1, eps.count { it.active })
    }

    @Test
    fun transportLabelKeepsBackwardCompatibleOutput() {
        assertEquals("USB 通道", AgentConnection.transportLabel("127.0.0.1:7246"))
        assertEquals("Tailscale", AgentConnection.transportLabel("100.64.1.2:7246"))
        assertEquals("局域网直连", AgentConnection.transportLabel("192.168.1.5:7246"))
        assertEquals("局域网直连", AgentConnection.transportLabel("10.0.0.9:7246"))
        assertEquals("", AgentConnection.transportLabel(""))
        assertEquals("直连", AgentConnection.transportLabel("8.8.8.8:7246"))
    }

    @Test
    fun candidateOrderStillUsbFirst() {
        AgentConnection.lastLanAddr = "10.0.0.5:7246"
        AgentConnection.lastTailscaleAddr = "100.64.1.2:7246"
        AgentConnection.manualAddress = "192.168.1.9:7246"
        assertEquals(
            listOf("127.0.0.1:7246", "10.0.0.5:7246", "192.168.1.9:7246", "100.64.1.2:7246"),
            AgentConnection.candidateAddresses(),
        )
    }
}
