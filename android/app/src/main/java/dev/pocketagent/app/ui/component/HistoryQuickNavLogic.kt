package dev.pocketagent.app.ui.component

import dev.pocketagent.app.ui.ChatItem

/**
 * 历史消息快速查看(2026-09-30,按用户提供的交互演示视频落地)的纯决策层。
 *
 * 视频样板(ZCode 客户端):会话左缘一条垂直时间线刻度条,每条用户消息一个刻度;
 * 点按刻度区从左缘浮出"用户消息摘要卡列表"(首行标题粗体 + 内容截断),点击卡片
 * 跳到那条消息并关闭浮层。这里把"哪些消息有刻度 / 刻度卡标题摘要怎么截 / 当前
 * 高亮哪个刻度"抽成无 Composable 依赖的纯函数,JVM 单测钉住(见 HistoryQuickNavLogicTest)。
 *
 * 【只认 user 消息】视频里刻度只对应"用户发出的每一轮指令"——助手输出/工具过程/
 * 思考过程不占刻度,轮次导航才不会被长回复稀释。
 */
object HistoryQuickNavLogic {

    /** 浮层卡片摘要的截断上限(字符)。两行 12sp 在 78% 宽面板里大约放得下的量。 */
    const val SUMMARY_MAX_CHARS = 64

    /** 刻度卡标题(消息首行)的截断上限。超过交给省略号,不撑破单行。 */
    const val TITLE_MAX_CHARS = 40

    /** 一张刻度对应的摘要卡:消息在时间线里的两个下标 + 展示文案。 */
    data class Entry(
        /** [ChatItem] 在 st.items 里的下标(业务序,发消息/重试等按它定位)。 */
        val itemIndex: Int,
        /** LazyColumn 里的下标 = itemIndex + 列表头部偏移(「加载更早」入口占 1 位)。 */
        val listIndex: Int,
        /** 卡片首行:消息第一行文本(粗体展示)。 */
        val title: String,
        /** 卡片次行:首行之外的内容合并截断。 */
        val summary: String,
    )

    /** 首行截断:换行/首尾空白都收掉;超长以省略号收尾。 */
    fun titleOf(text: String, max: Int = TITLE_MAX_CHARS): String {
        val firstLine = text.lineSequence().firstOrNull { it.isNotBlank() }?.trim().orEmpty()
        return truncate(firstLine, max)
    }

    /** 摘要截断:取首行之后的剩余内容,压成一行;空则回空串(卡片只显示标题)。 */
    fun summaryOf(text: String, max: Int = SUMMARY_MAX_CHARS): String {
        val rest = text.lineSequence().dropWhile { it.isBlank() }.drop(1)
            .joinToString(" ") { it.trim() }
            .trim()
        return truncate(rest, max)
    }

    /** 定长截断:超出以"…"收尾;不超原样返回。字符数按 Kotlin String(UTF-16 码元)。 */
    fun truncate(text: String, max: Int): String {
        if (max <= 0) return ""
        val t = text.trim()
        if (t.length <= max) return t
        return t.take(max) + "…"
    }

    /**
     * 从时间线 items 提取全部用户消息的刻度条目(保持原顺序)。
     *
     * @param listOffset LazyColumn 头部偏移:「加载更早」入口存在(historyTruncated 且
     *   非离线快照)时为 1,否则 0 —— 与 ChatScreen 里 itemsIndexed 的实际起点对齐。
     *   纯函数不感知这个 UI 事实,由调用方传入。
     *
     * 纯文本用户消息:标题=首行,摘要=其余内容。纯附件消息(无文本):标题=首个
     * 附件名,摘要="共 N 个附件" —— 都是时间线里真实存在的数据,不编造文案。
     */
    fun entriesOf(items: List<ChatItem>, listOffset: Int): List<Entry> {
        return items.mapIndexedNotNull { i, item ->
            if (item.role != "user") return@mapIndexedNotNull null
            val text = item.text
            val title: String
            val summary: String
            if (text.isNotBlank()) {
                title = titleOf(text)
                summary = summaryOf(text)
            } else if (item.attachments.isNotEmpty()) {
                title = item.attachments.first().name
                summary = "共 ${item.attachments.size} 个附件"
            } else {
                // 既无文本也无附件的占位消息不进刻度(没有可导航的内容)
                return@mapIndexedNotNull null
            }
            Entry(itemIndex = i, listIndex = i + listOffset, title = title, summary = summary)
        }
    }

    /**
     * 当前可见位置对应的刻度下标(浮层/刻度条高亮用):
     * 取"listIndex ≤ 当前列表首个可见项"的最后一个 —— 即视口最顶部已经滚过的
     * 最近一条用户消息;视口还没到第一条刻度时回 -1(无高亮)。
     */
    fun currentIndexOf(entries: List<Entry>, firstVisibleListIndex: Int): Int {
        var found = -1
        for ((i, e) in entries.withIndex()) {
            if (e.listIndex <= firstVisibleListIndex) found = i else break
        }
        return found
    }
}
