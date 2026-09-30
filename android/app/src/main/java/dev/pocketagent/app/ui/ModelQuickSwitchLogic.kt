package dev.pocketagent.app.ui

import pocketagent.v1.ModelInfo

/**
 * 模型行「快速切换面板」的纯决策(2026-09-30 模型选择 UI 优化)。
 *
 * 现状问题:输入区模型行点击进完整选择卡是两段式操作,高频"在最近用过的几个
 * 模型间来回切"太重。本逻辑给下拉箭头一个**单点即切**的轻面板:
 *  1) 有"最近用过"记录 → 按最近使用序展示(只保留当前 agent 模型表里还存在的,
 *     失效 id 静默淘汰 —— 不展示已经切不回去的幽灵项);
 *  2) 没有记录(或不足) → 用当前 agent 可用模型按原表顺序补足;
 *  3) 面板尾部固定「更多…」进完整选择卡(跟随桌面/全部模型的入口不丢)。
 *
 * 【"最近"是真实数据】记忆只记**用户在本进程内真点过的模型**(rememberPick 由
 * 选模型动作调用),进程冷启动即空、回落到"可用模型前 8 个"兜底 —— 不预填、
 * 不持久化假数据。跨进程的"最近"需要 daemon 侧能力,本期不做(不造假状态)。
 *
 * 【排除 /default】"跟随桌面配置"($agent/default 或空 id)的语义在完整选择卡里
 * 有专门文案与副标题,快速面板不承载;列表里混入它容易和"显式选 default"混淆。
 */
object ModelQuickSwitchLogic {

    /** 快速面板最多展示的模型数(含"最近"段;「更多…」不算在内)。 */
    const val MAX_QUICK = 8

    /** 快速面板一行:模型 id + 展示名 + 副文案(上下文/思考位,如实)。 */
    data class QuickModel(
        val id: String,
        val label: String,
        val detail: String,
    )

    // agent → 最近用过的模型 id(MRU 序,队首最新)。UI 线程访问,进程内记忆。
    private val recentByAgent = HashMap<String, ArrayDeque<String>>()

    /**
     * 记一次真实选择:该模型置为该 agent 的最近使用队首;重复选择去重前移。
     * 空串(跟随桌面)与 default 条目不记 —— 它们不该出现在快速面板里。
     */
    fun rememberPick(agentId: String, modelId: String) {
        if (agentId.isBlank() || modelId.isBlank() || modelId.endsWith("/default")) return
        val deque = recentByAgent.getOrPut(agentId) { ArrayDeque() }
        deque.remove(modelId)
        deque.addFirst(modelId)
        while (deque.size > MAX_QUICK) deque.removeLast()
    }

    /** 某 agent 的最近使用 id 快照(MRU 序;只读副本,防外部改坏内部状态)。 */
    fun recentOf(agentId: String): List<String> =
        recentByAgent[agentId]?.toList() ?: emptyList()

    /**
     * 快速面板数据:最近用过(且仍可用)优先,可用模型按原序补足,截到
     * [MAX_QUICK]。available 传 modelsForAgent 过滤后的当前 agent 模型表。
     */
    fun quickModels(agentId: String, available: List<ModelInfo>): List<QuickModel> {
        if (available.isEmpty()) return emptyList()
        val byId = available.associateBy { it.id }
        val ordered = ArrayList<QuickModel>(MAX_QUICK)
        val seen = HashSet<String>()
        // ① 最近段:仅保留当前表里还存在的 id(失效淘汰),保持 MRU 序
        for (id in recentOf(agentId)) {
            if (ordered.size >= MAX_QUICK) break
            val m = byId[id] ?: continue
            if (!seen.add(m.id)) continue
            ordered.add(toQuickModel(m))
        }
        // ② 可用模型按原表顺序补足(跳过 default 与已入选的)
        for (m in available) {
            if (ordered.size >= MAX_QUICK) break
            if (m.id.endsWith("/default")) continue
            if (!seen.add(m.id)) continue
            ordered.add(toQuickModel(m))
        }
        return ordered
    }

    private fun toQuickModel(m: ModelInfo): QuickModel = QuickModel(
        id = m.id,
        label = m.displayName.ifBlank { modelBareName(m.id) },
        detail = quickDetail(m),
    )

    /** 副文案:上下文窗口(真实上报值)+ 思考位(daemon 如实宣告),没有就不写。 */
    private fun quickDetail(m: ModelInfo): String {
        val parts = ArrayList<String>(2)
        if (m.contextWindow > 0) parts.add("上下文 ${m.contextWindow / 1000}k")
        if (m.supportsThinking) parts.add("支持思考")
        return parts.joinToString(" · ")
    }
}
