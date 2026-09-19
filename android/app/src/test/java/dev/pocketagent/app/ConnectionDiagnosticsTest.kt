package dev.pocketagent.app

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class ConnectionDiagnosticsTest {
    @Test
    fun classifies_auth_failure() {
        assertEquals(
            ConnectionFailureKind.AUTH,
            classifyConnectionFailure("UNAUTHENTICATED: token 无效,请先配对"),
        )
    }

    @Test
    fun classifies_timeout_failure() {
        assertEquals(
            ConnectionFailureKind.TIMEOUT,
            classifyConnectionFailure("deadline exceeded"),
        )
    }

    @Test
    fun classifies_unavailable_failure() {
        assertEquals(
            ConnectionFailureKind.UNAVAILABLE,
            classifyConnectionFailure("UNAVAILABLE: failed to connect to host"),
        )
    }

    // ---- P0-1:证书指纹不匹配的新分类 ----

    @Test
    fun classifies_cert_pin_mismatch_from_message() {
        assertEquals(
            ConnectionFailureKind.CERT_MISMATCH,
            classifyConnectionFailure("主机证书指纹不匹配: 已锁定 abcd…,本次 1234…"),
        )
        assertEquals(
            ConnectionFailureKind.CERT_MISMATCH,
            classifyConnectionFailure("javax.net.ssl.SSLHandshakeException: Fingerprint mismatch"),
        )
    }

    @Test
    fun classifies_cert_pin_mismatch_through_cause_chain() {
        // 模拟 OkHttp 包装:PinMismatchException 被包进 SSLException,
        // 最外层 message 已看不到"指纹"字样 —— 必须沿 cause 链识别。
        val wrapped = java.net.SocketException("Connection reset")
            .initCause(java.lang.Exception("outer", PinMismatchException("主机证书指纹不匹配: ...")))
        assertEquals(
            ConnectionFailureKind.CERT_MISMATCH,
            classifyConnectionFailure(wrapped as Throwable),
        )
        // 直接的 PinMismatchException 也要命中
        assertEquals(
            ConnectionFailureKind.CERT_MISMATCH,
            classifyConnectionFailure(PinMismatchException("主机证书指纹不匹配") as Throwable),
        )
    }

    @Test
    fun cert_mismatch_has_actionable_label() {
        val label = connectionFailureLabel(ConnectionFailureKind.CERT_MISMATCH)
        assertTrue(label.contains("指纹不匹配"))
    }
}
