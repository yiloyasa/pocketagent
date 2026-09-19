package dev.pocketagent.app.ui

import pocketagent.v1.CodexDesktopConfigSummary
import pocketagent.v1.CodexSettings

/**
 * 桌面只读回显的展示纯函数(daemon 的 desktop_config_summary / provider_caps /
 * sandbox_readiness 三个只读字段 → 设置页文案)。
 *
 * 【职责边界】daemon 侧只透传电脑端 config/read 等只读口子的**原始归一值**;
 * "这个值对人该说什么话"全部收在这里 —— 纯函数无 Composable 依赖,JVM 单测
 * 可直接钉住文案与空值语义,不必拉起 Compose 环境。
 *
 * 【两条纪律】
 *  1. 空值不占位:电脑端某项未显式配置(codex 用内置默认)、这次没读到、
 *     或整份消息缺省时,对应行直接不显示,绝不显示"电脑端当前:"加一个空串,
 *     也不把"没读到"说成"不支持";
 *  2. 认不出的取值原样展示(照镜子):codex 以后新增档位时手机端不至于显示错误文案。
 */
object DesktopEchoLogic {

    /** 审批策略原始值 → 中文标签(与设置页候选文案一致)。 */
    private val approvalLabels = mapOf(
        "untrusted" to "仅信任",
        "on-request" to "按需询问",
        "never" to "完全访问",
        "granular" to "细分控制(granular)",
    )

    /** 沙箱模式原始值 → 中文标签。 */
    private val sandboxLabels = mapOf(
        "read-only" to "只读",
        "workspace-write" to "工作区可写",
        "danger-full-access" to "完全访问",
    )

    /** 思考强度原始值 → 中文标签(与设置页候选文案一致)。 */
    private val effortLabels = mapOf(
        "none" to "关闭",
        "low" to "低",
        "medium" to "中",
        "high" to "高",
        "xhigh" to "极高",
        "max" to "最高",
    )

    /** 回答详略原始值 → 中文标签。 */
    private val verbosityLabels = mapOf(
        "low" to "精简",
        "medium" to "适中",
        "high" to "详细",
    )

    /** 思考摘要原始值 → 中文标签。 */
    private val summaryLabels = mapOf(
        "concise" to "精简",
        "detailed" to "详细",
    )

    /**
     * daemon 读到了桌面配置快照时返回它;整份缺省(读取失败/老 daemon)返回 null。
     * proto3 标量字段没有 has 语义,消息级的 hasDesktopConfigSummary 是唯一的
     * "读到了没有"判据 —— 所有取值先过这道门。
     */
    private fun summaryOf(settings: CodexSettings?): CodexDesktopConfigSummary? =
        settings?.takeIf { it.hasDesktopConfigSummary() }?.desktopConfigSummary

    /**
     * 设置项 key → "电脑端当前"的展示文案(含标签映射,不含前缀)。
     * key 与 daemon 侧 setting_status 的 key 同一套(approval_policy / sandbox /
     * model / effort / model_verbosity / model_reasoning_summary)。
     * hide_agent_reasoning 没有桌面对应键(codex 无此配置键),不在表内 → 不显示回显行。
     */
    fun desktopEchoValue(settings: CodexSettings?, key: String): String? {
        val summary = summaryOf(settings) ?: return null
        val raw = when (key) {
            "approval_policy" -> summary.approvalPolicy
            "sandbox" -> summary.sandboxMode
            "model" -> summary.model
            "effort" -> summary.effort
            "model_verbosity" -> summary.verbosity
            "model_reasoning_summary" -> summary.summary
            else -> return null
        }
        if (raw.isBlank()) return null
        return when (key) {
            "approval_policy" -> approvalLabels[raw] ?: raw
            "sandbox" -> sandboxLabels[raw] ?: raw
            "effort" -> effortLabels[raw] ?: raw
            "model_verbosity" -> verbosityLabels[raw] ?: raw
            "model_reasoning_summary" -> summaryLabels[raw] ?: raw
            else -> raw // model:归一后的 slug 原样展示
        }
    }

    /** 七项设置某一项的整行副文案"电脑端当前:X";无可显示值返回 null(不占位)。 */
    fun desktopEchoLine(settings: CodexSettings?, key: String): String? {
        val value = desktopEchoValue(settings, key) ?: return null
        return "电脑端当前:$value"
    }

    /** Windows 沙箱就绪度一行;未读到(空)返回 null。未知状态原样展示。 */
    fun sandboxReadinessLine(settings: CodexSettings?): String? {
        val raw = settings?.sandboxReadiness?.takeIf { it.isNotBlank() } ?: return null
        val label = when (raw) {
            "ready" -> "可用"
            "notConfigured" -> "未配置"
            "updateRequired" -> "需要更新"
            else -> raw
        }
        return "Windows 沙箱:$label"
    }

    /**
     * provider 能力徽标文案;provider_caps 整份缺省(没读到)时返回空列表。
     * 读到了就按真实布尔渲染:false 不渲染徽标(读到"不支持"与没读到是两回事)。
     */
    fun providerCapsBadges(settings: CodexSettings?): List<String> {
        val s = settings ?: return emptyList()
        if (!s.hasProviderCaps()) return emptyList()
        val caps = s.providerCaps
        return buildList {
            if (caps.imageGeneration) add("图像生成")
            if (caps.webSearch) add("联网搜索")
            if (caps.namespaceTools) add("命名空间工具")
        }
    }
}
