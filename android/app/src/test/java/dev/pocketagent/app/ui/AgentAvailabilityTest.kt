package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * agent 网格点亮与徽标/筛选纯函数的单测。
 * daemon 侧能力上报约定:HostInfo.capabilities 含 "agent:<id>"(旧格式)
 * 或 "agent:<id>:<版本号>"(新格式,如 "agent:pi:v0.85.1")。
 */
class AgentAvailabilityTest {

    // ---- resolveAgentAvailability(HomeScreen 点亮判定)----

    @Test
    fun `codex 恒可用_不看capabilities`() {
        assertTrue(resolveAgentAvailability("codex", baseAvailable = true, capabilities = emptyList()))
    }

    @Test
    fun `zcode_pi_按capabilities动态点亮`() {
        val caps = listOf("terminal", "fs", "agent", "agent:pi", "agent:zcode")
        assertTrue(resolveAgentAvailability("zcode", baseAvailable = false, capabilities = caps))
        assertTrue(resolveAgentAvailability("pi", baseAvailable = false, capabilities = caps))
    }

    @Test
    fun `带版本号的新格式capabilities按前缀匹配点亮`() {
        // daemon v2 格式:"agent:pi:v0.85.1" —— 前缀匹配点亮,版本号不挡道
        val caps = listOf("terminal", "agent:pi:v0.85.1")
        assertTrue(resolveAgentAvailability("pi", baseAvailable = false, capabilities = caps))
    }

    @Test
    fun `旧格式无版本号的capabilities向后兼容点亮`() {
        assertTrue(resolveAgentAvailability("pi", baseAvailable = false, capabilities = listOf("agent:pi")))
        assertTrue(resolveAgentAvailability("zcode", baseAvailable = false, capabilities = listOf("agent:zcode")))
    }

    @Test
    fun `相似id的前缀不误匹配`() {
        // "agent:pika" 是别的 id,不能点亮 pi(所以匹配必须连分隔符一起比)
        assertFalse(resolveAgentAvailability("pi", baseAvailable = false, capabilities = listOf("agent:pika")))
        assertFalse(resolveAgentAvailability("pi", baseAvailable = false, capabilities = listOf("agent:pi2:v1.0.0")))
    }

    @Test
    fun `带版本号的capabilities匹配同样忽略大小写`() {
        assertTrue(resolveAgentAvailability("pi", baseAvailable = false, capabilities = listOf("AGENT:PI:V0.85.1")))
    }

    @Test
    fun `检测不到capabilities时维持灰显`() {
        val caps = listOf("terminal", "fs", "git", "agent")
        assertFalse(resolveAgentAvailability("zcode", baseAvailable = false, capabilities = caps))
        assertFalse(resolveAgentAvailability("pi", baseAvailable = false, capabilities = caps))
        // 完全离线(host == null → 空列表)也一样灰显
        assertFalse(resolveAgentAvailability("zcode", baseAvailable = false, capabilities = emptyList()))
    }

    @Test
    fun `capabilities匹配忽略大小写`() {
        assertTrue(resolveAgentAvailability("zcode", baseAvailable = false, capabilities = listOf("AGENT:ZCODE")))
    }

    @Test
    fun `别人的capabilities不点亮自己`() {
        // hermes 没被 daemon 上报,别人的 "agent:pi" 不能点亮它
        assertFalse(resolveAgentAvailability("hermes", baseAvailable = false, capabilities = listOf("agent:pi")))
    }

    @Test
    fun `P1_6_占位agent被daemon上报后也动态点亮_不再被名单挡死`() {
        // 【P1-6 动态化回归】旧实现按名 when 只认 pi/zcode,daemon 哪天接入
        // opencode 并上报 "agent:opencode" 也会被名单挡成灰显;新实现一律看数据。
        assertTrue(resolveAgentAvailability("opencode", baseAvailable = false, capabilities = listOf("agent:opencode")))
        assertTrue(resolveAgentAvailability("hermes", baseAvailable = false, capabilities = listOf("agent:hermes:v1.2")))
    }

    @Test
    fun `P1_6_daemon新增第4个agent无需改App即点亮`() {
        // 【漏 zcode 同款坑的固化】daemon 侧新增 provider "gemini" 并上报
        // "agent:gemini:v1.0",本地任何名单都没写过它 —— 必须自动点亮。
        assertTrue(
            resolveAgentAvailability("gemini", baseAvailable = false, capabilities = listOf("agent:gemini:v1.0")),
        )
    }

    // ---- resolveAgentInstalledVersion(引导弹窗"已装 vX"/"未安装"两态)----

    @Test
    fun `已装版本提取_新格式返回版本号`() {
        assertEquals("v0.85.1", resolveAgentInstalledVersion("pi", listOf("fs", "agent:pi:v0.85.1")))
    }

    @Test
    fun `已装版本提取_旧格式或未装返回null`() {
        // 旧 daemon 只报 "agent:pi",无版本号
        assertNull(resolveAgentInstalledVersion("pi", listOf("agent:pi")))
        // 没装(没有对应能力项)
        assertNull(resolveAgentInstalledVersion("pi", listOf("agent:zcode", "terminal")))
        // 空列表(离线)
        assertNull(resolveAgentInstalledVersion("pi", emptyList()))
        // 别的 agent 的版本不串台
        assertNull(resolveAgentInstalledVersion("zcode", listOf("agent:pi:v0.85.1")))
    }

    // ---- unavailableLabel / agentGuideCopy(灰显副文字与引导弹窗文案选择)----

    @Test
    fun `灰显副文字_能力驱动型报未安装_基线型报接入中`() {
        assertEquals("电脑端未安装", unavailableLabel("pi"))
        assertEquals("电脑端未安装", unavailableLabel("zcode"))
        assertEquals("电脑端未安装", unavailableLabel("codex"))
        assertEquals("接入中", unavailableLabel("opencode"))
        assertEquals("接入中", unavailableLabel("claude-code"))
        assertEquals("接入中", unavailableLabel("hermes"))
        assertEquals("接入中", unavailableLabel("dsh"))
    }

    @Test
    fun `引导文案_pi_zcode_codex各有专属指引`() {
        // pi:判断依据带版本探测,指引带安装命令(方案给定文案)
        val pi = agentGuideCopy("pi")
        assertTrue(pi.detect.contains("--version"))
        assertTrue(pi.install.contains("npm install @earendil-works/pi-coding-agent"))
        // zcode:判断依据带 zcode.cjs
        assertTrue(agentGuideCopy("zcode").install.contains("zcode.cjs"))
        // codex:判断依据带 CODEX_BIN 或 PATH
        assertTrue(agentGuideCopy("codex").detect.contains("CODEX_BIN"))
    }

    @Test
    fun `引导文案_未接入型给接入中说明`() {
        val guide = agentGuideCopy("opencode")
        assertTrue(guide.detect.contains("尚未接入"))
        assertEquals("敬请期待后续版本", guide.install)
        assertEquals(guide, agentGuideCopy("dsh"))
    }

    // ---- buildAgents(网格清单)----

    @Test
    fun `buildAgents_按capabilities点亮网格`() {
        // pi 用新格式(带版本号)、zcode 用旧格式:两种格式都应点亮
        val caps = listOf("agent:pi:v0.85.1", "agent:zcode")
        val agents = buildAgents(caps)
        assertEquals(
            agents.associate { it.id to it.available },
            mapOf(
                "codex" to true,
                "opencode" to false,
                "claude-code" to false,
                "pi" to true,
                "hermes" to false,
                "dsh" to false,
                "zcode" to true,
            ),
        )
    }

    @Test
    fun `buildAgents_离线时只有codex点亮`() {
        val agents = buildAgents(emptyList())
        assertTrue(agents.all { it.id == "codex" || !it.available })
    }

    @Test
    fun `P1_6_buildAgents动态追加daemon新agent_展示名缺省回退id原文`() {
        // daemon 上报本地没见过的 "agent:gemini:v2.0.1" → 网格自动追加磁贴:
        // 展示名= id 原文(AUDIT §3.3:未知值显示原生原文,不显示"未知"、不留空白),
        // 可用= true(daemon 报了就是探明在场)。
        val agents = buildAgents(listOf("agent:pi", "agent:gemini:v2.0.1"))
        val gemini = agents.firstOrNull { it.id == "gemini" }
        assertTrue("daemon 新增的 agent 必须出现在网格里", gemini != null)
        assertEquals("gemini", gemini!!.name) // 未知 agent id 显示 id 原文(缺省展示名兜底)
        assertTrue(gemini.available)
        // 与本地展示目录不重复追加(pi 已在基线表,extra 去重)
        assertEquals(1, agents.count { it.id == "pi" })
    }

    @Test
    fun `P1_6_未知agent灰显时按能力驱动型展示_电脑端未安装`() {
        // 占位展示目录(opencode/claude-code/hermes/dsh)之外的新 id 一律按
        // "电脑端未安装"展示(它是 daemon 已知的 agent,灰显的真实原因就是没装),
        // 不落入"接入中"占位文案。
        assertEquals("电脑端未安装", unavailableLabel("gemini"))
    }
}
