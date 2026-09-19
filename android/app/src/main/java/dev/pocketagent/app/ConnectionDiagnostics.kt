package dev.pocketagent.app

enum class ConnectionFailureKind {
    AUTH,
    /** P0-1:主机证书指纹与已锁定值不一致 —— 必须断连,并提示用户风险与出路。 */
    CERT_MISMATCH,
    TIMEOUT,
    UNAVAILABLE,
    UNKNOWN,
}

data class ConnectionDiagnostic(
    val kind: ConnectionFailureKind,
    val message: String,
    val detail: String,
    val address: String,
)

fun classifyConnectionFailure(message: String): ConnectionFailureKind {
    val normalized = message.lowercase()
    return when {
        // P0-1 指纹锁定:TrustManager 抛出的文案(可能已被包装,匹配核心词)
        normalized.contains("证书指纹不匹配") ||
            normalized.contains("fingerprint mismatch") ||
            normalized.contains("pin mismatch") -> ConnectionFailureKind.CERT_MISMATCH
        normalized.contains("unauthenticated") ||
            normalized.contains("token 无效") ||
            normalized.contains("token invalid") ||
            normalized.contains("配对") -> ConnectionFailureKind.AUTH
        normalized.contains("deadline exceeded") ||
            normalized.contains("timeout") ||
            normalized.contains("timed out") -> ConnectionFailureKind.TIMEOUT
        normalized.contains("unavailable") ||
            normalized.contains("failed to connect") ||
            normalized.contains("connection refused") ||
            normalized.contains("连接被拒绝") ||
            normalized.contains("无法连接") -> ConnectionFailureKind.UNAVAILABLE
        else -> ConnectionFailureKind.UNKNOWN
    }
}

/**
 * 按异常分类(优先于按文本):沿 cause 链找 [PinMismatchException]。
 * 【为什么要这条】OkHttp 会把 TrustManager 抛的异常包进 SSLException,
 * 只看最外层 message 通常只剩 "Connection reset" 之类的噪音。
 */
fun classifyConnectionFailure(error: Throwable): ConnectionFailureKind {
    var cause: Throwable? = error
    var hops = 0
    while (cause != null && hops < 8) { // 限深:防御异常链上的环
        if (cause is PinMismatchException) return ConnectionFailureKind.CERT_MISMATCH
        cause = cause.cause
        hops++
    }
    return classifyConnectionFailure(error.message ?: error::class.java.simpleName)
}

fun connectionFailureLabel(kind: ConnectionFailureKind): String = when (kind) {
    ConnectionFailureKind.AUTH -> "token 无效或未完成配对"
    ConnectionFailureKind.CERT_MISMATCH ->
        "主机证书指纹不匹配(主机可能被冒充,或电脑端重装了 daemon)"
    ConnectionFailureKind.TIMEOUT -> "连接超时"
    ConnectionFailureKind.UNAVAILABLE -> "daemon 不可达，请确认电脑端已启动"
    ConnectionFailureKind.UNKNOWN -> "连接失败"
}
