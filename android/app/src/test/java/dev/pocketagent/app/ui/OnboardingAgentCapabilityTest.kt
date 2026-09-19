package dev.pocketagent.app.ui

import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * Onboarding「选择你的 agent」页的点亮判定断言。
 *
 * 【为什么存在】教程页的 Pi/ZCode 卡灰显判定与主页 agent 网格**同源**
 * (都走 [resolveAgentAvailability]("pi"/"zcode", baseAvailable=false, capabilities))。
 * 曾经教程页自建 equals 精确匹配,daemon 上线带版本号的 capability
 * ("agent:pi:v0.85.1")后已装机器被错误灰显 —— 这组用例把 Onboarding 实际
 * 调用形态在新旧两种格式下的行为钉死,防止再次漂移。
 */
class OnboardingAgentCapabilityTest {

    // ---- 旧格式(daemon 未带版本号)----

    @Test
    fun `教程页_旧格式精确串_点亮`() {
        val caps = listOf("terminal", "fs", "agent", "agent:pi", "agent:zcode")
        assertTrue(resolveAgentAvailability("pi", baseAvailable = false, capabilities = caps))
        assertTrue(resolveAgentAvailability("zcode", baseAvailable = false, capabilities = caps))
    }

    // ---- 新格式(带探测版本号)—— 本次回归的病灶:精确 equals 会漏掉这些 ----

    @Test
    fun `教程页_新格式带版本号_点亮`() {
        val caps = listOf("terminal", "agent:pi:v0.85.1", "agent:zcode:v1.2.3")
        assertTrue(resolveAgentAvailability("pi", baseAvailable = false, capabilities = caps))
        assertTrue(resolveAgentAvailability("zcode", baseAvailable = false, capabilities = caps))
    }

    @Test
    fun `教程页_新格式大小写不敏感_点亮`() {
        assertTrue(
            resolveAgentAvailability("pi", baseAvailable = false, capabilities = listOf("AGENT:PI:V0.85.1")),
        )
    }

    // ---- 防误匹配:别的 agent 的 capability 不能点亮(前缀必须连分隔符一起比)----

    @Test
    fun `教程页_相似id不误匹配`() {
        assertFalse(
            resolveAgentAvailability("pi", baseAvailable = false, capabilities = listOf("agent:pika")),
        )
        assertFalse(
            resolveAgentAvailability("pi", baseAvailable = false, capabilities = listOf("agent:pi2:v1.0.0")),
        )
    }

    // ---- 未连接 / 电脑端未装:灰显 ----

    @Test
    fun `教程页_未连接或未装_灰显`() {
        // 未连接:进页探测 3 秒超时,caps 为空 —— 全部灰显(与未装同语义)
        assertFalse(resolveAgentAvailability("pi", baseAvailable = false, capabilities = emptyList()))
        assertFalse(resolveAgentAvailability("zcode", baseAvailable = false, capabilities = emptyList()))
        // 装了 Pi 没装 ZCode:ZCode 卡灰显而 Pi 卡点亮
        val caps = listOf("agent:pi:v0.85.1")
        assertTrue(resolveAgentAvailability("pi", baseAvailable = false, capabilities = caps))
        assertFalse(resolveAgentAvailability("zcode", baseAvailable = false, capabilities = caps))
    }
}
