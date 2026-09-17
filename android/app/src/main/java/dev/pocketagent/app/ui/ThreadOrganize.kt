// ThreadOrganize.kt —— 会话组织(分组筛选、分叉、回滚)的纯逻辑。
//
// 【为什么单独一个文件】这三件事里凡是"能算错的"部分都不该混在 Composable 里:
// 分组到底命中哪一条、回滚该给用户几个选项、回滚前必须说什么 —— 这些是**规则**,
// 一旦写进 UI 就只能靠手点验证。抽成纯函数后可以钉住单测(见 ThreadOrganizeTest)。
package dev.pocketagent.app.ui

/**
 * 会话列表的筛选维度。
 *
 * 【为什么用单个字符串而不是 (Boolean, String?) 两个状态】"全部 / 已归档 / 未分组 /
 * 某个分组"是**互斥**的四类选择,拆成两个独立状态就会出现"既选了已归档又选了分组"
 * 这种在 UI 上根本没法表达的中间态。用单值键,状态空间天然合法。
 * 用 `#` 前缀而不是"分组 id 一定不含 #"的假设外的空串/特殊值:codex 的分组 id 是
 * 它自己生成的,我们不去猜它的字符集,只占住两个不可能与它撞车的键。
 */
internal const val THREAD_FILTER_ALL = ""
internal const val THREAD_FILTER_ARCHIVED = "#archived"
internal const val THREAD_FILTER_UNGROUPED = "#ungrouped"

/**
 * 某条会话是否命中当前筛选。
 *
 * 【规则】除"已归档"外,所有筛选都在**未归档**集合里做 —— 归档的会话只应该在
 * "已归档"里出现,否则它会在每个分组下重复露面。
 *
 * @param sectionId 会话所属分组;空串 = 未分组
 * @param archived 是否已归档(判定口径与 daemon 的 Thread.archived 一致)
 * @param filter 当前选中的筛选键
 */
internal fun matchesThreadFilter(sectionId: String, archived: Boolean, filter: String): Boolean =
    when (filter) {
        THREAD_FILTER_ALL -> !archived
        THREAD_FILTER_ARCHIVED -> archived
        THREAD_FILTER_UNGROUPED -> !archived && sectionId.isEmpty()
        else -> !archived && sectionId == filter
    }

/**
 * 回滚时可供选择的回合数。
 *
 * 【为什么不给一个 1..N 的滑动条】回合数是"最近几个"的语义,用户心里想的是
 * "把刚才那两步退回去",不是"退到第 7 个回合"。给 1/2/3 这三个常用档 + "全部"
 * 就覆盖了真实用法,而且每个选项都能一眼看出代价。
 *
 * 【边界】一个回合都没有(或历史还没加载出来)时返回空 —— 调用方据此把回滚
 * 入口整体禁用,而不是弹一个选了也必然失败的菜单。
 */
internal fun rollbackOptions(turnCount: Int): List<Int> {
    if (turnCount <= 0) return emptyList()
    // 只要**小于**总数的固定档位:等于总数时由"全部"那一项表达,避免出现两个一样的选项
    val fixed = listOf(1, 2, 3).filter { it < turnCount }
    return (fixed + turnCount).distinct()
}

/** 回滚选项的文案:"全部"要说清总量,否则用户不知道"全部"是多少。 */
internal fun rollbackOptionLabel(option: Int, turnCount: Int): String =
    if (option >= turnCount) "全部（$turnCount 回合）" else "$option 回合"

/**
 * 回滚前的后果说明。
 *
 * 【必须同时说清两件事】协议注释里写得很明确:`thread/rollback` 只改会话历史,
 * **不会**回滚 agent 已经对本地文件做的改动(官方原文:This only modifies the
 * thread's history and does not revert local file changes)。用户最容易误解的正是这一点
 * —— 以为"回滚 = 撤销",于是把不可逆的语义当成可逆的用。所以这段话是安全信息,
 * 不是装饰文案,单测会钉住它必须提到"文件"和"不会"。
 */
internal fun rollbackConsequence(turns: Int): String =
    "丢弃最近 $turns 个回合的对话记录。丢弃后找不回来;" +
        "电脑上已经改过的文件不会跟着撤销 —— 要撤销文件改动请用 Git。"

/** 分叉确认文案:分叉是"复制一份继续",原会话不动,这点要提前讲明。 */
internal fun forkConsequence(): String =
    "从这条会话的最新状态复制出一条新会话,原会话保持不变;" +
        "之后两条会话各自独立,改动互不影响。"

/**
 * "更多"面板里**回滚入口**的常驻说明(点击之前就要可见)。
 *
 * 与 [rollbackConsequence] 同源:同样必须诚实说明"不撤销文件改动 / 找不回来",
 * 只是不带具体回合数(用户还没选)。抽成函数以便全项目措辞一致并被单测钉住。
 */
internal fun rollbackSummary(): String =
    "把会话退回几个回合之前(只改对话记录)。丢弃的记录找不回来;" +
        "电脑上已经改过的文件不会跟着撤销 —— 要撤销文件改动请用 Git。"

/**
 * "未分组"的含义说明。
 *
 * 用户凭直觉容易把"未分组"当成"已删除/已归档"或"组没了内容也没了"。这里一次说清:
 * 它只是"还没归入任何分组",并且删组不删会话。全项目复用同一措辞。
 */
internal fun ungroupedHint(): String =
    "未分组 = 还没归入任何分组(不是已删除,也不是已归档);" +
        "删除分组不会删会话,组里的会话会变成未分组。"
