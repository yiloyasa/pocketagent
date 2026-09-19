package dev.pocketagent.app

import android.content.Context
import java.security.MessageDigest
import java.security.cert.CertificateException
import java.security.cert.X509Certificate
import javax.net.ssl.X509TrustManager

/**
 * P0-1 证书指纹锁定(TOFU):自签证书没有 CA 链可验,主机名也不可靠
 * (IP 换来换去),所以"这台电脑是谁"由叶子证书的 SHA-256 指纹回答。
 *
 * 【TOFU 规则】
 *  - prefs 里没存过指纹(首连):信任,并把本次指纹存下来(Trust On First Use);
 *  - 已存指纹与本次一致:放行;
 *  - 不一致:抛 [PinMismatchException] —— 握手必须当场失败,连接绝不能建立。
 *
 * 纯逻辑类:指纹存取抽象成 [PinStore],JVM 单测用内存实现即可覆盖
 * 通过/拒绝/TOFU 三个分支,不需要 Android 设备。
 */
class PinTrustManager(
    private val store: PinStore,
    /** 首次锁定成功后的回调(生产传 null 即可;预留测试/埋点用)。 */
    private val onPinSaved: ((String) -> Unit)? = null,
) : X509TrustManager {

    override fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?) {
        // 单向 TLS:本项目不启用 mTLS,客户端证书无需校验。
    }

    override fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) {
        val leaf = chain?.firstOrNull()
            ?: throw CertificateException("服务端未提供证书,无法校验主机指纹")
        val fingerprint = sha256Hex(leaf.encoded)
        val saved = store.savedPin()
        when {
            // TOFU 首连:记住它,从此它就是"那台电脑"。
            saved == null -> {
                store.savePin(fingerprint)
                // 【纯逻辑约束】本类要跑 JVM 单测,不能用 android.util.Log;
                // 锁定指纹的可见反馈由 onPinSaved 回调/调用方日志承担。
                onPinSaved?.invoke(fingerprint)
            }
            // 指纹不匹配 = 对面不是我们锁定的那台主机(被冒充/劫持),
            // 或电脑端重装生成了新证书。两种情况都必须断。
            !saved.equals(fingerprint, ignoreCase = true) -> throw PinMismatchException(
                "主机证书指纹不匹配: 已锁定 ${saved.take(16)}…,本次 ${fingerprint.take(16)}…" +
                    "。主机可能被冒充,或电脑端重装后换了证书;" +
                    "确认安全后可在「设置 → 用户数据管理」清除已存指纹后重连。",
            )
            // 一致:放行(大小写不敏感比较,兼容手工粘贴的指纹)。
        }
    }

    override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()

    companion object {
        /** 任意字节序列的 SHA-256 十六进制(64 位小写)—— 两端统一的指纹算法。 */
        fun sha256Hex(bytes: ByteArray): String =
            MessageDigest.getInstance("SHA-256").digest(bytes).joinToString("") { "%02x".format(it) }
    }
}

/** 指纹不匹配:必须让 TLS 握手失败,由 ConnectionDiagnostics 单独归类提示。 */
class PinMismatchException(message: String) : CertificateException(message)

/**
 * 指纹存取抽象。生产实现走 SharedPreferences;JVM 单测用内存假实现。
 * 存的都是 64 位十六进制字符串。
 */
interface PinStore {
    /** 已锁定的指纹;null = 尚未锁定(下一次连接即为 TOFU)。 */
    fun savedPin(): String?

    fun savePin(pin: String)

    fun clearPin()
}

/** 生产实现:与连接配置同一个 prefs 文件(agent_conn),键 serverCertSha256。 */
class SharedPrefsPinStore(context: Context) : PinStore {
    private val prefs = context.getSharedPreferences("agent_conn", Context.MODE_PRIVATE)

    override fun savedPin(): String? =
        prefs.getString(KEY_SERVER_CERT_SHA256, null)?.takeIf { it.isNotBlank() }

    override fun savePin(pin: String) {
        prefs.edit().putString(KEY_SERVER_CERT_SHA256, pin).apply()
    }

    override fun clearPin() {
        prefs.edit().remove(KEY_SERVER_CERT_SHA256).apply()
    }

    companion object {
        const val KEY_SERVER_CERT_SHA256 = "serverCertSha256"
    }
}

/** 内存实现:单测/降级兜底用(不持久化,进程死了指纹也就没了)。 */
class MemoryPinStore(initial: String? = null) : PinStore {
    @Volatile private var pin: String? = initial

    override fun savedPin(): String? = pin

    override fun savePin(pin: String) {
        this.pin = pin
    }

    override fun clearPin() {
        pin = null
    }
}
