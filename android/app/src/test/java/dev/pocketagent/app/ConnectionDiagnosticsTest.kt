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

    // ---- 版本协商闸(A5 最小实现):语义化版本解析与比较,位数边界 ----

    @Test
    fun semver_parses_plain_and_v_prefixed() {
        assertEquals(listOf(0, 3, 0), parseSemver("0.3.0"))
        assertEquals(listOf(1, 2, 30), parseSemver("v1.2.30"))
        assertEquals(listOf(10, 0, 1), parseSemver(" 10.0.1 "))
    }

    @Test
    fun semver_rejects_garbage_instead_of_guessing() {
        // 空/非数字/带后缀/缺分量分隔:一律 null → 上层按"不设限"处理,绝不误报过旧
        assertEquals(null, parseSemver(""))
        assertEquals(null, parseSemver("   "))
        assertEquals(null, parseSemver("0.3.x"))
        assertEquals(null, parseSemver("0.3.0-beta"))
        assertEquals(null, parseSemver("0.3."))
        assertEquals(null, parseSemver("three"))
    }

    @Test
    fun too_old_catches_lower_patch_and_minor() {
        assertTrue(isAppVersionTooOld("0.2.9", "0.3.0"))
        assertTrue(isAppVersionTooOld("0.2.99", "0.3.0"))
        assertTrue(isAppVersionTooOld("0.3.0", "0.3.1"))
    }

    @Test
    fun digit_boundary_is_numeric_not_lexicographic() {
        // 位数边界:字典序会把 "0.3.10" 排在 "0.3.9" 前,分量数值比较必须反着判
        assertTrue(!isAppVersionTooOld("0.3.10", "0.3.9"))
        assertTrue(isAppVersionTooOld("0.3.9", "0.3.10"))
        assertTrue(!isAppVersionTooOld("0.10.0", "0.9.99"))
    }

    @Test
    fun equal_or_newer_is_not_too_old() {
        assertTrue(!isAppVersionTooOld("0.3.0", "0.3.0"))
        assertTrue(!isAppVersionTooOld("0.3.1", "0.3.0"))
        assertTrue(!isAppVersionTooOld("1.0.0", "0.99.99"))
    }

    @Test
    fun missing_components_pad_with_zero() {
        // "0.3" 与 "0.3.0" 等价;缺的低位不构成"更旧"
        assertTrue(!isAppVersionTooOld("0.3", "0.3.0"))
        assertTrue(!isAppVersionTooOld("0.3", "0.3"))
        assertTrue(isAppVersionTooOld("0.3", "0.3.1"))
    }

    @Test
    fun empty_or_unparsable_min_never_flags() {
        // 旧 daemon 不带 min_app_version(空)= 不设限;脏数据同样放行
        assertTrue(!isAppVersionTooOld("0.3.0", ""))
        assertTrue(!isAppVersionTooOld("0.1.0", "latest"))
        // App 侧自己解析不了也不误报
        assertTrue(!isAppVersionTooOld("", "0.3.0"))
    }
}
