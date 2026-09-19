package dev.pocketagent.app

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.security.PublicKey
import java.security.cert.CertificateException
import java.security.cert.X509Certificate
import java.util.Date

/**
 * P0-1 验收单测:PinTrustManager 对/错指纹分别通过/拒绝、TOFU 首连存储与变更拒绝。
 *
 * 【为什么用 FakeCert】PinTrustManager 只消费叶子证书的 getEncoded() 字节
 * (算 SHA-256),不需要"真"证书对象 —— 直接子类化 X509Certificate,
 * 其余方法抛异常以证明没被误用。零第三方依赖,纯 JVM 可跑。
 */
class PinTrustManagerTest {

    /**
     * 只带回 encoded 字节的假证书:其余接口一律抛,锁定"实现只碰 encoded"这一事实。
     * 方法集按 android.jar(SDK 35)的 X509Certificate 精确对齐 —— 它与 OpenJDK
     * 的抽象方法清单不同(没有 getTBSCertificateString / verify(key, sigProvider))。
     */
    private class FakeCert(private val der: ByteArray) : X509Certificate() {
        override fun getEncoded(): ByteArray = der
        override fun verify(key: PublicKey?) = throw UnsupportedOperationException()
        override fun verify(key: PublicKey?, sigProvider: String?) = throw UnsupportedOperationException()
        override fun getNotBefore(): Date = throw UnsupportedOperationException()
        override fun getNotAfter(): Date = throw UnsupportedOperationException()
        override fun getSerialNumber(): java.math.BigInteger = throw UnsupportedOperationException()
        override fun getIssuerDN(): java.security.Principal = throw UnsupportedOperationException()
        override fun getSubjectDN(): java.security.Principal = throw UnsupportedOperationException()
        override fun getTBSCertificate(): ByteArray = throw UnsupportedOperationException()
        override fun getSignature(): ByteArray = throw UnsupportedOperationException()
        override fun getVersion(): Int = throw UnsupportedOperationException()
        override fun getSigAlgName(): String = throw UnsupportedOperationException()
        override fun getSigAlgOID(): String = throw UnsupportedOperationException()
        override fun getSigAlgParams(): ByteArray? = throw UnsupportedOperationException()
        override fun getIssuerUniqueID(): BooleanArray = throw UnsupportedOperationException()
        override fun getSubjectUniqueID(): BooleanArray = throw UnsupportedOperationException()
        override fun getKeyUsage(): BooleanArray? = throw UnsupportedOperationException()
        override fun getBasicConstraints(): Int = throw UnsupportedOperationException()
        override fun hasUnsupportedCriticalExtension(): Boolean = throw UnsupportedOperationException()
        override fun getCriticalExtensionOIDs(): MutableSet<String>? = throw UnsupportedOperationException()
        override fun getNonCriticalExtensionOIDs(): MutableSet<String>? = throw UnsupportedOperationException()
        override fun getExtensionValue(oid: String?): ByteArray? = throw UnsupportedOperationException()
        override fun toString(): String = "FakeCert(${der.size}B)"
        override fun getPublicKey(): PublicKey = throw UnsupportedOperationException()
        override fun checkValidity() = throw UnsupportedOperationException()
        override fun checkValidity(date: Date?) = throw UnsupportedOperationException()
    }

    private fun cert(bytes: String) = FakeCert(bytes.toByteArray(Charsets.ISO_8859_1))

    @Test
    fun tofu_first_connection_saves_pin_and_trusts() {
        val store = MemoryPinStore()
        assertNull("初始不应有指纹", store.savedPin())
        val tm = PinTrustManager(store)
        // 不应抛异常
        tm.checkServerTrusted(arrayOf(cert("TOFU-CERT-DER")), "RSA")
        val saved = store.savedPin()
        assertEquals("TOFU 首连应存下叶子证书 DER 的 SHA-256", PinTrustManager.sha256Hex("TOFU-CERT-DER".toByteArray(Charsets.ISO_8859_1)), saved)
        // 回调也应触发:onPinSaved 只在 TOFU 首连回调,所以要一个新的空存储
        var notified: String? = null
        val fresh = MemoryPinStore()
        PinTrustManager(fresh) { notified = it }.checkServerTrusted(arrayOf(cert("TOFU-CERT-DER")), "RSA")
        assertEquals(store.savedPin(), notified)
    }

    @Test
    fun matching_pin_trusts() {
        val der = "GOOD-CERT-DER".toByteArray(Charsets.ISO_8859_1)
        val store = MemoryPinStore(PinTrustManager.sha256Hex(der))
        PinTrustManager(store).checkServerTrusted(arrayOf(FakeCert(der)), "RSA") // 不抛即通过
    }

    @Test
    fun mismatched_pin_rejects_connection() {
        val store = MemoryPinStore(PinTrustManager.sha256Hex("LOCKED-CERT".toByteArray()))
        val tm = PinTrustManager(store)
        try {
            tm.checkServerTrusted(arrayOf(cert("IMPOSTER-CERT")), "RSA")
            throw AssertionError("指纹不匹配必须抛 PinMismatchException")
        } catch (expected: PinMismatchException) {
            // 文案要告诉用户风险与出路
            assertTrue(expected.message!!.contains("指纹不匹配"))
            assertTrue(expected.message!!.contains("清除"))
        }
    }

    @Test
    fun pin_survives_case_difference_and_fails_on_any_change() {
        val der = "SAME".toByteArray()
        val store = MemoryPinStore(PinTrustManager.sha256Hex(der).uppercase())
        // 大写锁定的指纹也要认出同一张证书
        PinTrustManager(store).checkServerTrusted(arrayOf(FakeCert(der)), "RSA")
        // 换一张就拒
        try {
            PinTrustManager(store).checkServerTrusted(arrayOf(cert("OTHER")), "RSA")
            throw AssertionError("证书变更必须拒绝")
        } catch (expected: PinMismatchException) {
        }
    }

    @Test
    fun missing_certificate_is_rejected() {
        val tm = PinTrustManager(MemoryPinStore(PinTrustManager.sha256Hex("X".toByteArray())))
        try {
            tm.checkServerTrusted(emptyArray(), "RSA")
            throw AssertionError("空证书链必须拒绝")
        } catch (expected: CertificateException) {
        }
    }

    @Test
    fun sha256_hex_is_64_lowercase() {
        val hex = PinTrustManager.sha256Hex("abc".toByteArray())
        assertEquals("ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad", hex)
        assertEquals(64, hex.length)
        assertEquals(hex, hex.lowercase())
        assertNotEquals(hex, PinTrustManager.sha256Hex("abd".toByteArray()))
    }
}
