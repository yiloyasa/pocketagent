package dev.pocketagent.app

import kotlinx.coroutines.runBlocking
import org.junit.After
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

/**
 * T5 tokenReady 门单测(纯 JVM):
 *
 * 背景:init 把 TokenStores.create(Keystore/EncryptedSharedPreferences 创建)+
 * 首次解密挪到了 Dispatchers.IO,主线程不再等它;防身份 churn 是硬约束 ——
 * token 未就绪期间探活循环不得发出任何探活 RPC(窗口期空 token 探测会被 daemon
 * 判 UNAUTHENTICATED,USB 场景据此触发静默自动重配对)。
 *
 * 观测点:[AgentConnection.probeGatePassedForTest] —— 探活协程在门(awaitTokenReady)
 * 之后、循环内首个 RPC 之前置位;"门未越过" ⇔ "未发任何探活 RPC"
 * (startAutoReconnect 在门之前只做 mDNS 初始化,JVM 上 appCtx=null 直接跳过)。
 */
class AgentConnectionTokenGateTest {

    @Before
    fun setUp() {
        AgentConnection.resetTokenGateForTest()
    }

    @After
    fun tearDown() {
        AgentConnection.stopAutoReconnectForTest()
        AgentConnection.resetTokenGateForTest()
    }

    // ---------------- 防 churn 硬约束 ----------------

    @Test
    fun `probe loop does not cross the gate while token is not ready`() {
        // tokenReady 未完成:探活协程必须停在门内,不发出任何探活 RPC
        // (门后才有首个 RPC;先于门越过即等于先于一切探活鉴权决策)。
        AgentConnection.startAutoReconnect()
        Thread.sleep(500)
        assertFalse(
            "token 未就绪期间探活不得越过门(防 UNAUTH 误判 → USB 静默重配对的身份 churn)",
            AgentConnection.probeGatePassedForTest,
        )
    }

    @Test
    fun `probe loop crosses the gate once token becomes ready`() {
        AgentConnection.startAutoReconnect()
        Thread.sleep(200)
        assertFalse("放行前门必须仍关着", AgentConnection.probeGatePassedForTest)

        AgentConnection.completeTokenReadyForTest()
        val deadline = System.currentTimeMillis() + 5_000
        while (System.currentTimeMillis() < deadline && !AgentConnection.probeGatePassedForTest) {
            Thread.sleep(20)
        }
        assertTrue("token 就绪后探活应立即放行(不引入可感知延迟)", AgentConnection.probeGatePassedForTest)
    }

    // ---------------- 超时兜底(3s 上限,用例收短) ----------------

    @Test
    fun `gate falls through after timeout and probing continues without token`() {
        AgentConnection.tokenReadyTimeoutMsForTest = 300
        AgentConnection.startAutoReconnect()
        val start = System.currentTimeMillis()
        val deadline = start + 5_000
        while (System.currentTimeMillis() < deadline && !AgentConnection.probeGatePassedForTest) {
            Thread.sleep(20)
        }
        val elapsed = System.currentTimeMillis() - start
        assertTrue("超时后必须放行:按现状(无 token)继续,绝不永久卡死探活", AgentConnection.probeGatePassedForTest)
        assertTrue("放行时刻应贴近收短后的超时(300ms),实测 ${elapsed}ms", elapsed in 200..3_000)
    }

    // ---------------- awaitTokenReady 原语 ----------------

    @Test
    fun `awaitTokenReady returns true immediately when token already ready`() = runBlocking {
        AgentConnection.completeTokenReadyForTest()
        val start = System.currentTimeMillis()
        val ready = AgentConnection.awaitTokenReady()
        assertTrue(ready)
        assertTrue(
            "已就绪时应立即返回,实测 ${System.currentTimeMillis() - start}ms",
            System.currentTimeMillis() - start < 1_000,
        )
    }

    @Test
    fun `awaitTokenReady returns false after timeout when token never ready`() = runBlocking {
        AgentConnection.tokenReadyTimeoutMsForTest = 250
        val start = System.currentTimeMillis()
        val ready = AgentConnection.awaitTokenReady()
        assertFalse("永不就绪时必须返回 false(超时按无 token 继续)", ready)
        val elapsed = System.currentTimeMillis() - start
        assertTrue("应在超时附近返回(250ms),实测 ${elapsed}ms", elapsed in 200..2_000)
    }
}
