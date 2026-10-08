package dev.pocketagent.app

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 「保存并测试连接」顺序缺陷回归(纯 JVM,release-smoke-20261009 实锤项)。
 *
 * 【缺陷】旧实现在【当前生效地址】的通道上先 authorize 再 setCredentials(addr):
 * 同机存在第二个 daemon(或用户改填新地址)时,8 位配对码被发给旧目标实例。
 * 【修法】顺序收进 [saveAndTestConnection] 单一事实源:先切凭证(通道指向新目标)
 * 后配对。本文件注入**记录调用序的 fake**,精确钉死三件事:
 *   1. 配对码路径的调用序 = switchAddress → authorize → connect(顺序不可换);
 *   2. token / 留空路径不碰 authorize(原本无缺陷,防止回归多出配对步骤);
 *   3. 配对失败不连接、错误原文透传;成功带回权威 token 供输入框同步。
 */
class AgentConnectionSaveOrderTest {

    /** 记录调用序的 fake:每个操作落一笔日志,并按脚本回放结果。 */
    private class RecordingOps(
        private val authorizeOk: Boolean = true,
        private val connectOk: Boolean = true,
    ) : SaveConnectionOps {
        /** 调用序日志:"switchAddress:addr" / "setCredentials:addr|tok" / "authorize:code" / "connect"。 */
        val calls = mutableListOf<String>()
        var currentToken = "old-token"
        var switchAddressCount = 0
        var lastSwitchedAddr: String? = null

        override fun switchAddress(addr: String) {
            calls += "switchAddress:$addr"
            switchAddressCount++
            lastSwitchedAddr = addr
            currentToken = "old-token" // 不动 token 的铁证:switchAddress 后 token 原样
        }

        override fun setCredentials(addr: String, token: String) {
            calls += "setCredentials:$addr|$token"
            currentToken = token
        }

        override fun authorize(pairCode: String): PairCodeResult {
            calls += "authorize:$pairCode"
            // 钉「配对发生在已切换的新目标上」:authorize 时 fake 记住的目标必须是
            // switchAddress 刚写入的那个(生产语义 = 通道已重建指向新地址)。
            check(lastSwitchedAddr != null) { "authorize 先于 switchAddress 被调用(顺序缺陷复现)" }
            return if (authorizeOk) {
                currentToken = "paired-real-token"
                PairCodeResult(true)
            } else {
                PairCodeResult(false, "配对码错误")
            }
        }

        override fun connect(): pocketagent.v1.HostInfo? {
            calls += "connect"
            return if (connectOk) pocketagent.v1.HostInfo.getDefaultInstance() else null
        }

        override fun currentToken(): String = currentToken
    }

    // ---------------- 配对码路径:先切后配 ----------------

    @Test
    fun `配对码路径 调用序为 切地址_配对_连接`() {
        val ops = RecordingOps()
        val r = saveAndTestConnection("192.168.1.7:7246", "12345678", ops)
        assertEquals(
            "顺序必须钉死为先切地址(通道指向新目标)再配对,最后连接",
            listOf("switchAddress:192.168.1.7:7246", "authorize:12345678", "connect"),
            ops.calls,
        )
        assertTrue(r.connected)
        assertEquals("配对成功必须带回权威 token,供输入框同步掉 8 位配对码", "paired-real-token", r.authToken)
        assertNull(r.pairError)
    }

    @Test
    fun `配对码路径 绝不经setCredentials把配对码落进token位`() {
        val ops = RecordingOps()
        saveAndTestConnection("192.168.1.7:7246", "12345678", ops)
        assertTrue(
            "配对码路径禁止 setCredentials(配对码不是 token,失败时不能残留成已存 token)",
            ops.calls.none { it.startsWith("setCredentials:") },
        )
    }

    @Test
    fun `配对失败 不连接 错误原文透传`() {
        val ops = RecordingOps(authorizeOk = false)
        val r = saveAndTestConnection("192.168.1.7:7246", "00000000", ops)
        assertEquals(
            listOf("switchAddress:192.168.1.7:7246", "authorize:00000000"),
            ops.calls,
        )
        assertFalse(r.connected)
        assertEquals("配对失败错误必须如实透传 daemon 原文(AUDIT §3.3)", "配对码错误", r.pairError)
        assertNull("配对失败绝无权威 token 可同步", r.authToken)
    }

    @Test
    fun `配对成功但连不上 connected为false且token仍同步`() {
        val ops = RecordingOps(authorizeOk = true, connectOk = false)
        val r = saveAndTestConnection("127.0.0.1:7246", "12345678", ops)
        assertFalse(r.connected)
        assertNull(r.pairError)
        assertEquals("配对已成功:token 已入库,连接失败不该把它从输入框冲掉", "paired-real-token", r.authToken)
    }

    // ---------------- token / 留空路径:不碰 authorize ----------------

    @Test
    fun `token路径 调用序为 setCredentials_connect 且不authorize`() {
        val ops = RecordingOps()
        val r = saveAndTestConnection("192.168.1.7:7246", "pa_live_9f2c81ba", ops)
        assertEquals(
            listOf("setCredentials:192.168.1.7:7246|pa_live_9f2c81ba", "connect"),
            ops.calls,
        )
        assertTrue(r.connected)
        assertNull(r.pairError)
        assertNull("token 直连无需同步输入框", r.authToken)
    }

    @Test
    fun `留空路径 走setCredentials直连 不authorize`() {
        val ops = RecordingOps()
        saveAndTestConnection("127.0.0.1:7246", "", ops)
        assertEquals(
            listOf("setCredentials:127.0.0.1:7246|", "connect"),
            ops.calls,
        )
    }
}
