package dev.pocketagent.app.ui

/**
 * 设置一级页(总览)的取值/摘要纯函数层(R1-C 设置信息架构轮)。
 *
 * 【为什么单独成层】本轮把一级页重排成「连接 / Agent / 外观 / 数据与缓存 / 安全 /
 * 关于」后,每个导航行都要带"当前值摘要"(不用点进去就知道现状);这些摘要全部
 * 从既有状态取真值,映射规则(词表、缺省回退、断连标注)抽成纯函数在这里被
 * 单测钉死 —— UI 只排版,不再各写一份 when。
 *
 * 【断连纪律(调研结论 5)】依赖主机的取值(审批/沙箱/电脑系统信息)在设置快照
 * 缺席时必须明确标注"需连接主机",绝不显示空占位"—"(会被读成"没有值");
 * 本地偏好类(显示模式/字号/桥接开关/下载清单/主机指纹)不依赖连接,断连照常出值。
 */

/** 连接行摘要(照既有 connSummary 语义:ok→已连接·通道;down→未连接;其余→检查中)。 */
internal fun connectionSummary(connState: String, transport: String): String = when (connState) {
    "ok" -> "已连接 · " + transport.ifEmpty { "直连" }
    "down" -> "未连接"
    else -> "检查中"
}

/** 桥接与输入行摘要:全局对话显示的两项本地偏好(不依赖连接,断连可读)。 */
internal fun localDisplayPrefsSummary(showThinking: Boolean, toolDetailsExpanded: Boolean): String =
    (if (showThinking) "思考展示" else "思考隐藏") + " · " +
        (if (toolDetailsExpanded) "工具展开" else "工具折叠")

/** 下载与存储行摘要:只数已完成的下载(本地任务清单,断连可用);0 个不摆 0。 */
internal fun downloadStorageSummary(doneCount: Int): String =
    if (doneCount <= 0) "暂无下载内容" else "$doneCount 个文件"

/**
 * 电脑行摘要(关于组):os/arch 有哪个拼哪个;都没有时按连接状态如实说 ——
 * 已连接=还在读取;未连接=需连接主机;检查中=检查中。不以"—"冒充事实。
 */
internal fun hostPlatformSummary(os: String?, arch: String?, connState: String): String {
    val joined = listOfNotNull(
        os?.takeIf { it.isNotBlank() },
        arch?.takeIf { it.isNotBlank() },
    ).joinToString("/")
    if (joined.isNotBlank()) return joined
    return when (connState) {
        "ok" -> "读取中…"
        "down" -> "需连接主机"
        else -> "检查中"
    }
}

/** 审批策略 → 中文短标签(与 Agent 与模型页候选同词表;空 = 跟随桌面;词表外显示原文)。 */
internal fun approvalPolicyLabel(raw: String?): String = when (raw) {
    null, "" -> "跟随桌面"
    "untrusted" -> "仅信任"
    "on-request" -> "按需询问"
    "never" -> "完全访问"
    else -> raw
}

/** 沙箱模式 → 中文短标签(同上:空 = 跟随桌面;词表外显示原文,不猜)。 */
internal fun sandboxModeLabel(raw: String?): String = when (raw) {
    null, "" -> "跟随桌面"
    "read-only" -> "只读"
    "workspace-write" -> "工作区可写"
    "danger-full-access" -> "完全访问"
    else -> raw
}

/**
 * 安全行(审批与沙箱)摘要。这两项的权威值在电脑端(daemon 的 CodexSettings):
 * 快照缺席(未连接/还没读到)时如实标注"需连接主机";读到后给"审批 · 沙箱"拼串。
 */
internal fun approvalSandboxSummary(approval: String?, sandbox: String?, settingsReady: Boolean): String {
    if (!settingsReady) return "需连接主机"
    return approvalPolicyLabel(approval) + " · " + sandboxModeLabel(sandbox)
}

/** 主机信任行摘要:已锁定给前 16 位十六进制;未锁定 = 陈述"首连时自动锁定"的既有事实。 */
internal fun hostTrustSummary(lockedPin: String?): String =
    if (lockedPin.isNullOrBlank()) "首连时自动锁定"
    else "已锁定 · " + lockedPin.take(16)
