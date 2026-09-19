package dev.pocketagent.app

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.security.KeyFactory
import java.security.KeyPair
import java.security.KeyPairGenerator
import java.security.Signature
import java.security.spec.ECGenParameterSpec
import java.security.spec.X509EncodedKeySpec
import java.util.Base64

/**
 * P0-2 验收单测:挑战应答状态机(AgentConnection 内 [DeviceAuthFlow] 的纯逻辑核心)。
 *
 * 【Keystore 部分接口化】状态机只依赖 [DeviceKeySigner];这里用纯 JVM ECDSA
 * (KeyPairGenerator "EC" + Signature "SHA256withECDSA")做签名实现 —— 与
 * Android Keystore 的 SHA256withECDSA 输出同为 ASN.1 DER,daemon 侧
 * ecdsa.VerifyASN1 的验法一致,可覆盖完整往返。
 */
class DeviceAuthFlowTest {

    /** 纯 JVM 的 P-256 签名器:KeystoreDeviceKeySigner 的"形状"完全一致。 */
    private class JvmEcdsaSigner : DeviceKeySigner {
        val keyPair: KeyPair = KeyPairGenerator.getInstance("EC").apply {
            initialize(ECGenParameterSpec("secp256r1"))
        }.generateKeyPair()

        override fun publicKeyBase64(): String =
            Base64.getEncoder().encodeToString(keyPair.public.encoded)

        override fun sign(data: ByteArray): ByteArray {
            val s = Signature.getInstance("SHA256withECDSA")
            s.initSign(keyPair.private)
            s.update(data)
            return s.sign()
        }

        override fun available(): Boolean = true
    }

    /** 可编程的假 RPC:记录调用并按脚本应答。 */
    private class FakeRpc : DeviceAuthFlow.Rpc {
        var challengeToReturn: String? = "cafe1234"
        var answerToReturn: String? = "new-token-64hex"
        var requests = 0
        var submissions = 0
        var lastChallenge = ""
        var lastSignatureB64 = ""
        var lastDeviceId = ""

        override fun requestChallenge(deviceId: String): String? {
            requests++
            lastDeviceId = deviceId
            return challengeToReturn
        }

        override fun submitAnswer(deviceId: String, challenge: String, signatureBase64: String): String? {
            submissions++
            lastDeviceId = deviceId
            lastChallenge = challenge
            lastSignatureB64 = signatureBase64
            return answerToReturn
        }
    }

    @Test
    fun renew_token_full_round_trip() {
        val signer = JvmEcdsaSigner()
        val rpc = FakeRpc()
        val flow = DeviceAuthFlow("dev-1", signer, rpc)

        val token = flow.renewToken()
        assertEquals("new-token-64hex", token)
        assertEquals(1, rpc.requests)
        assertEquals(1, rpc.submissions)
        assertEquals("dev-1", rpc.lastDeviceId)
        assertEquals("cafe1234", rpc.lastChallenge)

        // 签名内容必须是 device_id:challenge(与 daemon VerifyChallenge 逐字节一致)
        val sig = Base64.getDecoder().decode(rpc.lastSignatureB64)
        val verifier = Signature.getInstance("SHA256withECDSA").apply {
            initVerify(keyFactoryPublic(signer))
            update("dev-1:cafe1234".toByteArray(Charsets.UTF_8))
        }
        assertTrue("签名应能以登记公钥验证", verifier.verify(sig))

        // 公钥必须是 SPKI(X.509)编码 —— daemon 侧 x509.ParsePKIXPublicKey 直接可解
        val parsed = KeyFactory.getInstance("EC")
            .generatePublic(X509EncodedKeySpec(Base64.getDecoder().decode(signer.publicKeyBase64())))
        assertEquals(signer.keyPair.public, parsed)
    }

    private fun keyFactoryPublic(signer: JvmEcdsaSigner) = signer.keyPair.public

    @Test
    fun keystore_unavailable_degrades_to_null() {
        val rpc = FakeRpc()
        // signer = null 即"Keystore 不可用":必须返回 null 走降级,不许抛异常
        assertNull(DeviceAuthFlow("dev-1", null, rpc).renewToken())
        assertEquals(0, rpc.requests) // 根本不该发请求
    }

    @Test
    fun signer_that_reports_unavailable_degrades_to_null() {
        val unavailable = object : DeviceKeySigner {
            override fun publicKeyBase64() = ""
            override fun sign(data: ByteArray): ByteArray = throw IllegalStateException("no keystore")
            override fun available() = false
        }
        val rpc = FakeRpc()
        assertNull(DeviceAuthFlow("dev-1", unavailable, rpc).renewToken())
        assertEquals(0, rpc.requests)
    }

    @Test
    fun signing_failure_degrades_to_null() {
        val broken = object : DeviceKeySigner {
            override fun publicKeyBase64() = "AAAA"
            override fun sign(data: ByteArray): ByteArray = throw IllegalStateException("Keystore 挂了")
            override fun available() = true
        }
        val rpc = FakeRpc()
        assertNull(DeviceAuthFlow("dev-1", broken, rpc).renewToken())
        assertEquals("要了挑战但不应提交应答", 1, rpc.requests)
        assertEquals(0, rpc.submissions)
    }

    @Test
    fun rpc_failure_degrades_to_null() {
        val signer = JvmEcdsaSigner()
        val rpc = FakeRpc().apply { challengeToReturn = null } // daemon 不可达/设备被撤销
        assertNull(DeviceAuthFlow("dev-1", signer, rpc).renewToken())
        rpc.challengeToReturn = "c"
        rpc.answerToReturn = null // 验签被拒
        assertNull(DeviceAuthFlow("dev-1", signer, rpc).renewToken())
    }

    @Test
    fun blank_device_id_never_talks_to_daemon() {
        val rpc = FakeRpc()
        assertNull(DeviceAuthFlow("", JvmEcdsaSigner(), rpc).renewToken())
        assertNull(DeviceAuthFlow("   ", JvmEcdsaSigner(), rpc).renewToken())
        assertEquals(0, rpc.requests)
    }

    @Test
    fun signed_message_format_is_device_colon_challenge() {
        val flow = DeviceAuthFlow("my-device", null, FakeRpc())
        assertEquals("my-device:ch-99", String(flow.signedMessage("ch-99"), Charsets.UTF_8))
        assertNotNull(flow)
    }
}
