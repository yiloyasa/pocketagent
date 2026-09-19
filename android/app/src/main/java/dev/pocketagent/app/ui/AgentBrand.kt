package dev.pocketagent.app.ui

import dev.pocketagent.app.R

/**
 * agent id → 品牌图标:与主页网格、设置页「切换 Agent」用的是**同一组 drawable**。
 *
 * 【为什么必须只有这一份】同一个映射以前有两处:SettingsScreen 里是 private map,
 * 主页则按 id 字符串直接引用 —— "设置里的 Agent 图标和主页对不上"就是这么来的。
 * 现在收敛到这里唯一一份,模型选择器(聊天页卡面 / 设置页弹窗)也复用同一张图,
 * 于是"这个模型是哪个 agent 提供的"一眼可辨,不再需要读者去猜。
 *
 * 主页叫 claude-code、设置里历史上叫 claude,两个 id 都映射到同一张图。
 */
val AGENT_BRAND_ICON: Map<String, Int> = mapOf(
    "codex" to R.drawable.agent_codex,
    "opencode" to R.drawable.agent_opencode,
    "claude" to R.drawable.agent_claudecode,
    "claude-code" to R.drawable.agent_claudecode,
    "pi" to R.drawable.agent_pi,
    "hermes" to R.drawable.agent_hermes,
    "dsh" to R.drawable.agent_dsh,
    "zcode" to R.drawable.agent_zcode,
)

/** agent id → 品牌图标资源;未知 id 返回 null(调用方自行决定是否留占位)。 */
fun agentBrandIcon(id: String): Int? = AGENT_BRAND_ICON[id]
