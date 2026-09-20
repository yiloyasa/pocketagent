package dev.pocketagent.app

import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

/**
 * 回退链候选顺序的回归测试 —— 对应真机验证发现的缺陷 D-01。
 *
 * 背景:原先 `candidateAddresses()` 用 `isUsbDebugging()` 决定要不要把 USB 候选放进队列,
 * 而该判定依赖 `UsbManager.deviceList.isNotEmpty()`。真机上手机插在电脑上时是
 * USB 设备角色(UFP),`dumpsys usb` 显示 data_role=device / host_connected=false,
 * deviceList 必然为空 → 判定恒为 false → USB 候选从不进队列。
 *
 * 实测证据:USB 调试已开、`adb reverse tcp:7246` 已注册,App 仍显示「局域网直连」,
 * netstat 里对端是手机 WiFi IP 而非 127.0.0.1。
 *
 * 现在改为无条件优先 USB。这些用例锁住新的顺序语义,防止回归。
 */
class AgentConnectionCandidatesTest {

    private var savedLan = ""
    private var savedTs = ""
    private var savedManual = ""

    @Before
    fun setUp() {
        savedLan = AgentConnection.lastLanAddr
        savedTs = AgentConnection.lastTailscaleAddr
        savedManual = AgentConnection.manualAddress
    }

    @After
    fun tearDown() {
        AgentConnection.lastLanAddr = savedLan
        AgentConnection.lastTailscaleAddr = savedTs
        AgentConnection.manualAddress = savedManual
    }

    @Test
    fun usbCandidateIsAlwaysFirstAndOrderIsPreserved() {
        AgentConnection.lastLanAddr = "10.0.0.5:7246"
        AgentConnection.lastTailscaleAddr = "100.64.1.2:7246"
        AgentConnection.manualAddress = "192.168.1.9:7246"

        val cands = AgentConnection.candidateAddresses()

        assertEquals(
            "USB(adb reverse)必须无条件排第一",
            "127.0.0.1:7246",
            cands.first(),
        )
        assertEquals(
            listOf(
                "127.0.0.1:7246",
                "10.0.0.5:7246",
                "192.168.1.9:7246",
                "100.64.1.2:7246",
            ),
            cands,
        )
    }

    @Test
    fun manualAddressIsLastResortAndDeduplicatedAgainstUsb() {
        AgentConnection.lastLanAddr = ""
        AgentConnection.lastTailscaleAddr = ""
        // 用户手填的就是 USB 地址 -> 应与队首的 USB 候选取并集去重
        AgentConnection.manualAddress = "127.0.0.1:7246"

        assertEquals(listOf("127.0.0.1:7246"), AgentConnection.candidateAddresses())
    }

    @Test
    fun blankCandidatesAreDropped() {
        AgentConnection.lastLanAddr = ""
        AgentConnection.lastTailscaleAddr = ""
        AgentConnection.manualAddress = ""

        val cands = AgentConnection.candidateAddresses()

        assertTrue("候选列表不能为空(至少保留 USB)", cands.isNotEmpty())
        assertTrue("空白候选必须被过滤", cands.none { it.isBlank() })
    }

    @Test
    fun newRegistryMethodAutomaticallyJoinsCandidates() {
        // 回归(AUDIT §3.3-1,批 2 审核不通过项):候选必须由注册表遍历产出,
        // 禁止按 id 字面量挑选(firstOrNull { it.id == "lan" } / "manual" / "tailscale"
        // 那种写法会让新增连接方式被静默丢弃 —— 既非透传也非报错)。
        // 通过注入「真实注册表 + 一条新 method」的切片,钉死自动接入;
        // 若实现退回按 id 挑选,relay 的地址会从候选中消失,本用例变红。
        AgentConnection.setMdnsDiscoveryForTest(null)
        AgentConnection.lastLanAddr = "10.0.0.5:7246"
        AgentConnection.lastTailscaleAddr = "100.64.1.2:7246"
        AgentConnection.manualAddress = "192.168.1.9:7246"
        val relay = ConnMethod(
            id = "relay", label = "中继", transport = "中继",
            addrProvider = { "198.51.100.7:7246" },
            hintText = { "测试注入的新连接方式" },
            matcher = { it.startsWith("198.51.") },
        )

        val cands = AgentConnection.candidatesFromRegistry(
            usbProbe = null,
            registry = AgentConnection.methodsForTest() + relay,
        )

        // 既有 4 个 method 的候选与顺序不变(USB 首位 → 局域网 → 手动 → 新方式
        // 按「其他」桶排在 Tailscale 之前),新方式自动进候选。
        assertEquals(
            listOf(
                "127.0.0.1:7246",
                "10.0.0.5:7246",
                "192.168.1.9:7246",
                "198.51.100.7:7246",
                "100.64.1.2:7246",
            ),
            cands,
        )
    }
}
