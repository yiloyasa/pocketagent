package dev.pocketagent.app.ui

import pocketagent.v1.CodexCapability

/**
 * WorkflowB 纯逻辑:能力分组映射 + 设置项状态文案映射。
 *
 * 【为什么单独一个文件】这些是**纯函数**,不依赖 Compose,可以直接 JVM 单测。
 * 项 3a 与项 3b 各有一条"未知取值不能丢/不能崩"的硬要求(未知 kind、未知 state),
 * 把它们放在可测的纯函数里,才能用测试长期钉住。
 */

/**
 * kind → 显示名。
 *
 * 【必须可扩展 + 未知兜底】codex 的 `CodexCapability.kind` 未来可能新增类型。
 * 旧的实现把顺序硬编码成三元组(skill/hook/plugin),**未知 kind 会被整类藏掉**
 * —— 用户装了新类型的扩展却一个字都看不到,这是 bug 而不是"没实现"。
 * 现在:认识的用中文名,不认识的**原样显示 kind 名**,不丢弃。
 */
val CAPABILITY_KIND_LABELS: Map<String, String> = mapOf(
    "skill" to "技能",
    "hook" to "Hooks",
    "plugin" to "插件",
)

/** 已知 kind 的固定展示顺序(不要让 codex 返回顺序导致每次打开位置抖动)。 */
val CAPABILITY_KIND_ORDER: List<String> = listOf("skill", "hook", "plugin")

/** kind → 显示名;未知 kind 返回其原始名(空白则给"其他")。 */
fun capabilityKindLabel(kind: String): String =
    CAPABILITY_KIND_LABELS[kind] ?: kind.ifBlank { "其他" }

/**
 * 把实际出现的 kinds 排成稳定顺序:已知种类按 [CAPABILITY_KIND_ORDER] 在前,
 * 未知种类按名字排序附在后面(未知**不丢弃**)。
 */
fun capabilityKindOrder(kinds: Collection<String>): List<String> {
    val known = CAPABILITY_KIND_ORDER.filter { it in kinds }
    val unknown = kinds.filterNot { it in CAPABILITY_KIND_LABELS }.sorted()
    return known + unknown
}

/**
 * 按名称/来源过滤能力(纯函数,供二级页搜索框使用)。
 * 查询为空 = 原样返回;大小写不敏感。
 */
fun filterCapabilities(items: List<CodexCapability>, query: String): List<CodexCapability> {
    val q = query.trim()
    if (q.isEmpty()) return items
    return items.filter {
        it.name.contains(q, ignoreCase = true) || it.source.contains(q, ignoreCase = true)
    }
}

/**
 * 分组:返回 (kind, 组内按名称稳定排序的条目) 列表。
 * 顺序由 [capabilityKindOrder] 决定,组内按名称不区分大小写稳定排序。
 */
fun groupCapabilities(items: List<CodexCapability>): List<Pair<String, List<CodexCapability>>> {
    val byKind = items.groupBy { it.kind }
    return capabilityKindOrder(byKind.keys).map { kind ->
        val list = byKind[kind].orEmpty().sortedWith(
            compareBy({ it.name.lowercase() }, { it.name }),
        )
        kind to list
    }
}

/**
 * 设置项状态 → 展示语义。
 *
 * @param level "ok" / "warn" / "error" / "muted"(由 UI 层映射到具体颜色,
 *              纯逻辑不依赖 Compose,便于单测)。
 * @param text  展示文案。
 *
 * 【未知 state 兜底显示原文,不崩】与"未知 kind 不丢弃"同一套纪律:
 * daemon 以后新增状态时,旧 App 也能如实显示,而不是白屏或崩。
 */
data class SettingStateInfo(val level: String, val text: String)

fun settingStateInfo(state: String): SettingStateInfo = when (state) {
    "applied" -> SettingStateInfo("ok", "已生效")
    "saved_not_applied" -> SettingStateInfo("warn", "已保存,未生效")
    "unsupported" -> SettingStateInfo("muted", "当前 codex 版本不支持该项")
    "failed" -> SettingStateInfo("error", "连接失败")
    // unreachable:codex 一时没响应(超时/正忙),无法确认是否生效 —— 这是**可重试**
    // 的暂时态,语气要比"连接失败"轻,并明确告诉用户可以重试。
    "unreachable" -> SettingStateInfo("warn", "尚未确认,可重试")
    "follow_desktop" -> SettingStateInfo("muted", "跟随电脑端配置")
    // locked(第 28 轮):七项 Agent 设置由电脑端决定,daemon 忽略手机写入;
    // 对应的只读行右侧已显示同一句提示,状态小字级别给 muted 以免重复刷屏。
    "locked" -> SettingStateInfo("muted", "由电脑端管理")
    "pending" -> SettingStateInfo("warn", "待生效")
    "" -> SettingStateInfo("muted", "状态未知")
    else -> SettingStateInfo("muted", state)
}

/**
 * 一句话总结设置整体状态(用于设置页顶部的诚实提示)。
 *
 * @return null 表示"没有可恢复的问题,无需提示";非 null 是一句需要让用户看见的话。
 *
 * 【只统计**可恢复**的问题:failed / saved_not_applied】
 * - `unsupported`(本版本 codex 不支持)是**静态事实**,不可重试;它已经在对应
 *   设置项那一行如实显示(开关置灰 + "当前 codex 版本不支持该项"),不该再顶一条
 *   永远存在的横幅 —— 那样反而会让用户忽略真正的告警。
 * - `follow_desktop` 是正常取值,更不该提示。
 * 绝不因为有失败就返回 null —— 那等于把问题藏起来。
 */
fun overallSettingNote(statuses: List<String>): String? {
    if (statuses.isEmpty()) return null
    val failed = statuses.count { it == "failed" }
    val notApplied = statuses.count { it == "saved_not_applied" }
    val unconfirmed = statuses.count { it == "unreachable" }
    if (failed == 0 && notApplied == 0 && unconfirmed == 0) return null
    val parts = buildList {
        if (failed > 0) add("$failed 项连接失败")
        if (notApplied > 0) add("$notApplied 项尚未生效")
        // unreachable 是"一时确认不了、可重试",措辞必须区别于"连接失败",
        // 否则用户会以为电脑上的 codex 坏了,而不是"稍后再试"。
        if (unconfirmed > 0) add("$unconfirmed 项尚未确认,可重试")
    }
    return parts.joinToString("、")
}

// ---- 项 3b 第二轮:下拉候选 / 开关的纯逻辑(可单测) ----

/**
 * 在下拉候选中追加"跟随桌面"入口(空串 = 不覆盖电脑上的 Codex 配置)。
 *
 * 【为什么必须在过滤之后调用】托管策略([filterByManagedPolicy])会按管理员白名单
 * 过滤候选;而"跟随桌面"是不覆盖,本就不该被白名单禁掉,所以只能加在过滤结果之上,
 * 不能塞进被过滤的原始 listOf 里 —— 否则管理员一托管,用户就再也回不到默认值。
 *
 * 【为什么防御去重】入参若已含 id 为空串的项,再追加会出现两个"跟随桌面";
 * 这里保证幂等,下拉里绝不出现重复项。
 */
fun withFollowDesktop(
    filtered: List<Triple<String, String, String>>,
): List<Triple<String, String, String>> =
    if (filtered.any { it.first.isEmpty() }) {
        filtered
    } else {
        filtered + Triple("", "跟随桌面", "不覆盖电脑上的 Codex 配置，用它的默认值")
    }

/**
 * 开关的真实显示态:后端标记为 [unsupported] 的项,即使本地存量为 true 也必须显示为关。
 *
 * 【为什么】"能点、显示已保存、实际无效"正是要根治的坑。一个不可用的开关绝不允许显示
 * 为开 —— 否则用户会以为它已经生效,而后端根本没有这个开关。
 */
fun switchChecked(raw: Boolean, unsupported: Boolean): Boolean = raw && !unsupported
