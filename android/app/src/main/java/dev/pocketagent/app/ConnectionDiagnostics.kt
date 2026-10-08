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

// =====================================================================================
// 版本协商闸(A5 最小实现,2026-10-09 分发资格批)
//
// daemon 在 HostInfo.min_app_version(host.proto 字段 13)随探活报告它要求的最低
// App 版本;App 侧用下面两个纯函数与自身 versionName 分量比较,过低时点亮
// AgentConnection.appVersionOutdated 提示位。设计口径:
//  · **只提示不阻断**:旧 daemon 恒不带该字段(空 = 不设限),协议自身就是软闸;
//  · **宁可漏报不误报**:任一侧解析失败(空/非数字段/带后缀)一律视为"不设限",
//    绝不因脏数据把能用的连接标成过旧;
//  · 比较是语义化的分量数值比较(0.3.10 > 0.3.9),不是字典序/不是 versionCode。
// =====================================================================================

/**
 * 解析语义化版本字符串 → 分量列表(每个分量一个非负整数)。
 * 宽容口径:允许前缀 "v"/"V"("v0.3.0");分量按 '.' 切分后必须全为数字。
 * 解析不了(空/空白/含非数字分量)→ null,调用方按"不设限"处理。
 */
fun parseSemver(version: String): List<Int>? {
    val trimmed = version.trim().removePrefix("v").removePrefix("V").trim()
    if (trimmed.isEmpty()) return null
    val parts = trimmed.split('.')
    if (parts.any { it.isEmpty() || it.any { ch -> !ch.isDigit() } }) return null
    return parts.map { it.toInt() }
}

/**
 * App 版本是否低于 daemon 要求的最低版本(纯函数,JVM 单测锁定语义化版本与
 * 位数边界)。任一侧解析失败、或 min 为空(旧 daemon/未设限)→ false。
 * 缺省分量按 0 补齐("0.3" == "0.3.0");分量逐位数值比较,首位定胜负。
 */
fun isAppVersionTooOld(appVersion: String, minAppVersion: String): Boolean {
    val app = parseSemver(appVersion) ?: return false
    val min = parseSemver(minAppVersion) ?: return false
    val width = maxOf(app.size, min.size)
    for (i in 0 until width) {
        val a = app.getOrElse(i) { 0 }
        val m = min.getOrElse(i) { 0 }
        if (a != m) return a < m
    }
    return false
}

// 【文案收敛(2026-10-09 修复轮)】原 appVersionOutdatedLabel()(唯一消费方
// AgentConnection.appVersionOutdatedMessage 已删,本函数随之零消费)已移除;
// "App 版本过旧"提示文案唯一事实源 = ui/SettingsScreen.appVersionOutdatedHint。
