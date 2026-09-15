package dev.pocketagent.app

enum class ConnectionFailureKind {
    AUTH,
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

fun connectionFailureLabel(kind: ConnectionFailureKind): String = when (kind) {
    ConnectionFailureKind.AUTH -> "token 无效或未完成配对"
    ConnectionFailureKind.TIMEOUT -> "连接超时"
    ConnectionFailureKind.UNAVAILABLE -> "daemon 不可达，请确认电脑端已启动"
    ConnectionFailureKind.UNKNOWN -> "连接失败"
}
