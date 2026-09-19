package dev.pocketagent.app

import android.content.Context
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import android.util.Log
import java.security.KeyPairGenerator
import java.security.KeyStore
import java.security.PrivateKey
import java.security.ProviderException
import java.security.PublicKey
import java.security.Signature
import java.security.spec.ECGenParameterSpec
import java.util.Base64

/**
 * P0-2 设备身份:手机本机持有 P-256 密钥对(Android Keystore 保护私钥),
 * 公钥在配对时交给 daemon 登记;token 失效时用它对挑战数签名续期。
 *
 * 【为什么是 P-256 而不是 Ed25519】Android Keystore 对 Ed25519 的支持要
 * API 33,本项目 minSdk 26 —— P-256 是所有支持级别上都能硬件保护的曲线。
 *
 * 抽象成接口是为了 JVM 单测:状态机([DeviceAuthFlow])只依赖本接口,
 * 单测用纯 JVM ECDSA 实现即可,不碰 AndroidKeyStore。
 */
interface DeviceKeySigner {
    /** base64(SPKI DER) 公钥,配对时随 AuthorizeRequest 发给 daemon。 */
    fun publicKeyBase64(): String

    /**
     * 对 data 做 SHA256withECDSA 签名,返回 ASN.1 DER 编码。
     * 与 daemon 侧 ecdsa.VerifyASN1 一一对应。
     * Keystore 不可用时抛异常 —— 调用方必须降级,绝不能断连死锁。
     */
    fun sign(data: ByteArray): ByteArray

    /** 乐观检查:签名能力当前是否可用(实现可在首次真正用时再发现失败)。 */
    fun available(): Boolean
}

/**
 * Android Keystore 实现:alias "pa_device_key",EC P-256,
 * 只注册 SHA256 摘要,不要求用户认证(认证发生在连接时无人解锁手机,
 * 要求生物识别会让后台自动重连永远失败)。
 */
class KeystoreDeviceKeySigner(
    private val alias: String = DEFAULT_ALIAS,
) : DeviceKeySigner {

    @Volatile
    private var cached: Pair<PublicKey, PrivateKey>? = null

    /** 懒加载 + 生成:Keystore 操作有毫秒级开销,不在 app 启动路径上做。 */
    @Synchronized
    private fun ensureKeyPair(): Pair<PublicKey, PrivateKey>? {
        cached?.let { return it }
        return try {
            val ks = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }
            val priv = ks.getKey(alias, null) as PrivateKey?
            val pub = ks.getCertificate(alias)?.publicKey
            if (priv != null && pub != null) {
                (pub to priv).also { cached = it }
            } else {
                // 首次:生成持久化的 P-256 密钥对
                val gen = KeyPairGenerator.getInstance(KeyProperties.KEY_ALGORITHM_EC, "AndroidKeyStore")
                val spec = KeyGenParameterSpec.Builder(
                    alias,
                    KeyProperties.PURPOSE_SIGN or KeyProperties.PURPOSE_VERIFY,
                )
                    .setAlgorithmParameterSpec(ECGenParameterSpec("secp256r1"))
                    .setDigests(KeyProperties.DIGEST_SHA256)
                    .setUserAuthenticationRequired(false) // 后台自动重连必须免解锁
                    .build()
                gen.initialize(spec)
                gen.generateKeyPair().let { it.public to it.private }.also { cached = it }
            }
        } catch (e: Exception) {
            // Keystore 可能被系统策略/ROM 缺陷禁用:记日志并判为不可用,
            // 上层降级旧 pair_code 流程,不重试不阻塞。
            Log.w(TAG, "Keystore 密钥不可用: ${e.message}")
            null
        }
    }

    override fun available(): Boolean = try {
        ensureKeyPair() != null
    } catch (_: Exception) {
        false
    }

    override fun publicKeyBase64(): String {
        val kp = ensureKeyPair() ?: throw IllegalStateException("AndroidKeyStore 不可用")
        // PublicKey.encoded 即 X.509 SubjectPublicKeyInfo(DER) —— 与 daemon
        // 侧 x509.ParsePKIXPublicKey 直接对应。
        return Base64.getEncoder().encodeToString(kp.first.encoded)
    }

    override fun sign(data: ByteArray): ByteArray {
        val kp = ensureKeyPair() ?: throw IllegalStateException("AndroidKeyStore 不可用")
        // Keystore 操作偶发 ProviderException(系统压力/并发)也按"签名失败"
        // 上抛,由调用方降级,不让异常穿透到连接层。
        try {
            val signer = Signature.getInstance("SHA256withECDSA")
            signer.initSign(kp.second)
            signer.update(data)
            return signer.sign()
        } catch (e: ProviderException) {
            throw IllegalStateException("Keystore 签名失败: ${e.message}", e)
        }
    }

    companion object {
        private const val TAG = "DeviceKey"
        const val DEFAULT_ALIAS = "pa_device_key"

        /** 便捷工厂:测试环境(无 Android framework)返回 null。 */
        fun create(context: Context?): KeystoreDeviceKeySigner? =
            if (context != null) KeystoreDeviceKeySigner() else null
    }
}

/**
 * 挑战应答状态机(纯逻辑,RPC 与签名能力全部注入,JVM 可测):
 *
 *   challenge_request(device_id) → daemon 回随机 challenge
 *   → 本地签名 "device_id:challenge"(SHA256withECDSA)
 *   → challenge_answer{challenge, signature} → daemon 验签发新 token
 *
 * 【任何一步失败都返回 null,绝不抛异常】调用方(AgentConnection)拿到
 * null 就走降级路径(重配对/维持旧 token),连接永远不会因为续期失败而死锁。
 */
class DeviceAuthFlow(
    private val deviceId: String,
    private val signer: DeviceKeySigner?,
    private val rpc: Rpc,
) {
    /** daemon 侧 Authorize 的两个瘦封装(生产用 gRPC stub,单测用假实现)。 */
    interface Rpc {
        fun requestChallenge(deviceId: String): String?

        fun submitAnswer(deviceId: String, challenge: String, signatureBase64: String): String?
    }

    /** 签名内容的分隔符格式,daemon 侧逐字节一致:`device_id + ":" + challenge`。 */
    fun signedMessage(challenge: String): ByteArray = "$deviceId:$challenge".toByteArray(Charsets.UTF_8)

    /**
     * 执行一次完整的挑战应答,成功返回新 token(调用方负责保存),
     * 失败返回 null。
     */
    fun renewToken(): String? {
        if (deviceId.isBlank()) return null
        val signer = signer ?: return null // Keystore 整体不可用
        if (!signer.available()) return null
        val challenge = rpc.requestChallenge(deviceId) ?: return null
        if (challenge.isBlank()) return null
        val signature = try {
            signer.sign(signedMessage(challenge))
        } catch (_: Exception) {
            return null // 签名失败 = 降级
        }
        return rpc.submitAnswer(deviceId, challenge, Base64.getEncoder().encodeToString(signature))
    }
}
