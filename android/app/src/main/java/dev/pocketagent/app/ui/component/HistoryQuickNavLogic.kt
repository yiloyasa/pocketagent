package dev.pocketagent.app.ui.component

import dev.pocketagent.app.ui.ChatItem
import java.util.Calendar
import java.util.TimeZone
import kotlin.math.abs
import kotlin.math.roundToInt

/**
 * 历史消息快速查看(2026-09-30,按用户提供的交互演示视频落地)的纯决策层。
 *
 * 视频样板(ZCode 客户端):会话左缘一条垂直时间线刻度条,每条用户消息一个刻度;
 * 指针停在刻度区浮出"紧凑摘要卡"(首行标题粗体 + 内容截断),随指针移动实时切换,
 * 轻点打开完整列表。触屏翻译(二版,两段式):
 *  - **按住拖动** = hover 的等价物:按住刻度条浮出紧凑预览气泡,上下拖动气泡跟手、
 *    内容实时切换为手指处的用户消息,松手跳到那条;几乎没拖就松手 = 取消;
 *  - **轻点**(预览未出现即松手)= 打开完整列表浮层。
 *
 * 这里把"哪些消息有刻度 / 刻度卡标题摘要怎么截 / 当前高亮哪个刻度 / 手指位置↔
 * 刻度下标映射 / 按住拖动手势状态机 / 气泡 Y 定位"全部抽成无 Composable 依赖的
 * 纯函数,JVM 单测钉住(见 HistoryQuickNavLogicTest)。
 *
 * 【只认 user 消息】视频里刻度只对应"用户发出的每一轮指令"——助手输出/工具过程/
 * 思考过程不占刻度,轮次导航才不会被长回复稀释。
 */
object HistoryQuickNavLogic {

    /** 浮层卡片摘要的截断上限(字符)。两行 12sp 在 78% 宽面板里大约放得下的量。 */
    const val SUMMARY_MAX_CHARS = 64

    /** 刻度卡标题(消息首行)的截断上限。超过交给省略号,不撑破单行。 */
    const val TITLE_MAX_CHARS = 40

    /** 一张刻度对应的摘要卡:消息在时间线里的两个下标 + 展示文案 + 真实时间。 */
    data class Entry(
        /** [ChatItem] 在 st.items 里的下标(业务序,发消息/重试等按它定位)。 */
        val itemIndex: Int,
        /** LazyColumn 里的下标 = itemIndex + 列表头部偏移(「加载更早」入口占 1 位)。 */
        val listIndex: Int,
        /** 卡片首行:消息第一行文本(粗体展示)。 */
        val title: String,
        /** 卡片次行:首行之外的内容合并截断。 */
        val summary: String,
        /**
         * 消息墙钟时间(epoch ms);**0 = 未知**(旧快照/上游未给),消费方
         * (气泡日期行/面板按日分组)见 0 必须隐藏或归入「早前」,绝不编造。
         */
        val sentAtMs: Long = 0,
    )

    /** 首行截断:换行/首尾空白都收掉;超长以省略号收尾。 */
    fun titleOf(text: String, max: Int = TITLE_MAX_CHARS): String {
        val firstLine = text.lineSequence().firstOrNull { it.isNotBlank() }?.trim().orEmpty()
        return truncate(firstLine, max)
    }

    /**
     * 预览气泡头部的位置指示:N/M(1 起,等宽字不跳动)。count<=0 回空串
     * (无内容不摆壳);index 越界钳制(退场定格路径可能短暂越界)。
     */
    fun positionLabel(index: Int, count: Int): String =
        if (count <= 0) "" else "${index.coerceIn(0, count - 1) + 1}/$count"

    // ==== R3 面板搜索(2026-10-01 深优):标题/摘要大小写不敏感包含匹配 ====

    /**
     * 面板搜索过滤:标题或摘要包含查询词(忽略大小写与首尾空白)即保留;
     * 空查询原样返回。序号/跳转映射以过滤后列表在全量中的下标为准(调用方
     * 用 entry.listIndex,天然不受过滤影响)。
     */
    fun filterEntries(entries: List<Entry>, query: String): List<Entry> {
        val q = query.trim()
        if (q.isEmpty()) return entries
        return entries.filter {
            it.title.contains(q, ignoreCase = true) || it.summary.contains(q, ignoreCase = true)
        }
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
            Entry(itemIndex = i, listIndex = i + listOffset, title = title, summary = summary, sentAtMs = item.sentAtMs)
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

    // ==== R2 日期上下文(2026-10-01 深优):气泡日期行 + 面板按日分组 + 日界长刻度 ====
    //
    // 全部纯函数,时钟/时区注入(单测钉死);风格与 ThreadsRowLogic.threadRowTime
    // 同源(当天零点差防夏令时)。sentAtMs==0(旧快照/上游未给)一律诚实降级:
    // 气泡不显日期行、分组归入「早前」—— 绝不编造时间。

    /**
     * 预览气泡的日期戳:今天→「HH:mm」、昨天→「昨天 HH:mm」、同年→「M月d日 HH:mm」、
     * 跨年→「yyyy年M月d日」;ms<=0 回空串(消费方隐藏日期行)。
     */
    fun historyStampOf(unixMs: Long, nowMs: Long, zone: TimeZone = TimeZone.getDefault()): String {
        if (unixMs <= 0L) return ""
        val dayDiff = dayDiffOf(unixMs, nowMs, zone)
        return when {
            dayDiff == 0L -> formatTime(unixMs, "HH:mm", zone)
            dayDiff == 1L -> "昨天 " + formatTime(unixMs, "HH:mm", zone)
            sameYear(unixMs, nowMs, zone) -> formatTime(unixMs, "M月d日 HH:mm", zone)
            else -> formatTime(unixMs, "yyyy年M月d日 HH:mm", zone)
        }
    }

    /** 当天在时区内的零点毫秒(跨夏令时的 23/25 小时天不靠整除)。 */
    private fun dayStartMs(ms: Long, zone: TimeZone): Long {
        val cal = Calendar.getInstance(zone)
        cal.timeInMillis = ms
        cal.set(Calendar.HOUR_OF_DAY, 0)
        cal.set(Calendar.MINUTE, 0)
        cal.set(Calendar.SECOND, 0)
        cal.set(Calendar.MILLISECOND, 0)
        return cal.timeInMillis
    }

    /** unix 与 now 之间隔了几个自然日(now 在前为正);风格与 threadRowTime 一致。 */
    private fun dayDiffOf(unixMs: Long, nowMs: Long, zone: TimeZone): Long =
        Math.round((dayStartMs(nowMs, zone) - dayStartMs(unixMs, zone)) / 86_400_000.0)

    private fun sameYear(unixMs: Long, nowMs: Long, zone: TimeZone): Boolean {
        val a = Calendar.getInstance(zone).apply { timeInMillis = unixMs }.get(Calendar.YEAR)
        val b = Calendar.getInstance(zone).apply { timeInMillis = nowMs }.get(Calendar.YEAR)
        return a == b
    }

    private fun formatTime(ms: Long, pattern: String, zone: TimeZone): String {
        val fmt = java.text.SimpleDateFormat(pattern, java.util.Locale.CHINESE)
        fmt.timeZone = zone
        return fmt.format(java.util.Date(ms))
    }

    /** 按自然日分组(保持原顺序);sentAtMs==0 的条目全部归入末尾「早前」组。 */
    data class DayGroup(val label: String, /** 该组首条在 entries 里的下标。 */ val firstIndex: Int, val entries: List<Entry>)

    fun groupByDay(entries: List<Entry>, nowMs: Long, zone: TimeZone = TimeZone.getDefault()): List<DayGroup> {
        if (entries.isEmpty()) return emptyList()
        val out = mutableListOf<DayGroup>()
        var currentKey: String? = null
        for ((i, e) in entries.withIndex()) {
            val key = if (e.sentAtMs <= 0L) "earlier" else "d${dayStartMs(e.sentAtMs, zone)}"
            if (key != currentKey) {
                val label = if (e.sentAtMs <= 0L) "早前" else {
                    when (dayDiffOf(e.sentAtMs, nowMs, zone)) {
                        0L -> "今天"
                        1L -> "昨天"
                        else -> formatTime(e.sentAtMs, if (sameYear(e.sentAtMs, nowMs, zone)) "M月d日" else "yyyy年M月d日", zone)
                    }
                }
                out += DayGroup(label, i, mutableListOf(e))
                currentKey = key
            } else {
                (out.last().entries as MutableList<Entry>).add(e)
            }
        }
        return out
    }

    /**
     * 刻度条日界标记:每天第一条刻度画全长(其余 0.7 宽),长会话里日期分段
     * 一眼可辨。首条恒为日界(无论其时间是否已知);0 时间戳不打断日界序列。
     */
    fun dayStartFlags(entries: List<Entry>, nowMs: Long, zone: TimeZone = TimeZone.getDefault()): List<Boolean> {
        if (entries.isEmpty()) return emptyList()
        val flags = ArrayList<Boolean>(entries.size)
        var lastKey: String? = null
        for (e in entries) {
            val key = if (e.sentAtMs <= 0L) lastKey ?: "unknown" else "d${dayStartMs(e.sentAtMs, zone)}"
            flags += key != lastKey
            lastKey = key
        }
        return flags
    }

    // ==== 按住拖动预览(二版新增):手指位置 ↔ 刻度下标映射 + 手势状态机 + 气泡定位 ====
    //
    // 全部是纯函数:输入像素/手势,输出下标/裁定。UI 层(HistoryQuickNav)只做
    // 状态搬运与绘制,不做决策 —— 这样"气泡串刻度 / 松手误跳 / 取消判定漂移"这类
    // 交互 bug 都能在 JVM 单测里钉死,不用上真机碰运气。

    /**
     * 刻度几何:与刻度条 Canvas 同一套数学的单源真值。
     *
     * 刻度均匀分布:自然间距(tick + gap)放得下就居中留白;放不下整体压缩到满高。
     * [step] 为相邻刻度间距,[top] 为首刻度顶部 Y;count ≤ 1 时 step=0、垂直居中。
     */
    data class TickGeometry(val step: Float, val top: Float)

    fun tickGeometry(count: Int, heightPx: Float, tickHeightPx: Float, gapPx: Float): TickGeometry {
        if (count <= 1 || heightPx <= tickHeightPx) {
            return TickGeometry(0f, ((heightPx - tickHeightPx) / 2f).coerceAtLeast(0f))
        }
        val naturalStep = tickHeightPx + gapPx
        val step = minOf(naturalStep, (heightPx - tickHeightPx) / (count - 1))
        val totalH = tickHeightPx + step * (count - 1)
        return TickGeometry(step = step, top = (heightPx - totalH) / 2f)
    }

    /**
     * 手指 Y(px,刻度条局部坐标)↔ 刻度下标:取"离手指最近的刻度",越界钳到
     * [0, count)。count ≤ 0 回 -1(无内容);count == 1 恒 0;几何退化(step≤0)回 0。
     */
    fun indexAtY(y: Float, count: Int, heightPx: Float, tickHeightPx: Float, gapPx: Float): Int {
        if (count <= 0) return -1
        if (count == 1) return 0
        val g = tickGeometry(count, heightPx, tickHeightPx, gapPx)
        if (g.step <= 0f) return 0
        return ((y - g.top - tickHeightPx / 2f) / g.step).roundToInt().coerceIn(0, count - 1)
    }

    /** 按住拖动手势的阶段:空闲 / 已按下未预览(轻点判定窗) / 预览中(气泡浮出)。 */
    enum class NavGesturePhase { IDLE, PRESSED, PREVIEW }

    /** 一次按住拖动手势的纯状态。Y 均为刻度条局部坐标(px)。 */
    data class NavGesture(
        val phase: NavGesturePhase = NavGesturePhase.IDLE,
        /** 按下时的起始 Y(取消判定与拖动判定都以它为基准)。 */
        val startY: Float = 0f,
        /** 手指当前 Y(跟手:气泡位置与预览内容都由它驱动)。 */
        val y: Float = 0f,
    )

    /** 松手裁定:松开手指那一刻,这次交互"是"什么。 */
    sealed interface NavRelease {
        /** 轻点(预览出现前就松手)→ 打开完整列表浮层。 */
        data object OpenPanel : NavRelease
        /** 拖动后松手 → 跳到该刻度(手指停住处对应的那条用户消息)。 */
        data class Jump(val entryIndex: Int) : NavRelease
        /** 预览出现过但几乎没拖(手指仍在起点附近)→ 取消,只收气泡不跳转。 */
        data object Cancel : NavRelease
    }

    /** 按下:进入轻点判定窗(起点 Y 记档)。 */
    fun pressed(y: Float): NavGesture = NavGesture(NavGesturePhase.PRESSED, startY = y, y = y)

    /** 按住超时未松:预览浮出(气泡出现)。只在 PRESSED 相位生效。 */
    fun held(g: NavGesture): NavGesture =
        if (g.phase == NavGesturePhase.PRESSED) g.copy(phase = NavGesturePhase.PREVIEW) else g

    /**
     * 手指移动。PRESSED 相位位移超过 [dragSlopPx] 直接进预览(快扫不等人);
     * 未超 slop 仍算轻点窗内的微动;PREVIEW 相位只更新跟手位置。
     */
    fun moved(g: NavGesture, y: Float, dragSlopPx: Float): NavGesture = when (g.phase) {
        NavGesturePhase.IDLE -> g
        NavGesturePhase.PRESSED ->
            if (abs(y - g.startY) >= dragSlopPx) g.copy(phase = NavGesturePhase.PREVIEW, y = y)
            else g.copy(y = y)
        NavGesturePhase.PREVIEW -> g.copy(y = y)
    }

    /** 手势作废/收场:回到空闲(气泡收起,不跳转)。 */
    fun canceled(): NavGesture = NavGesture()

    /**
     * 松手裁定:
     *  - PRESSED(轻点窗内松手)→ OpenPanel;
     *  - PREVIEW 且位移 < [cancelSlopPx](手指几乎没离起点)→ Cancel;
     *  - PREVIEW 且真拖动了 → Jump 到手指停住处的刻度;
     *  - IDLE 无事可裁 → null。
     */
    fun released(
        g: NavGesture,
        count: Int,
        heightPx: Float,
        tickHeightPx: Float,
        gapPx: Float,
        cancelSlopPx: Float,
    ): NavRelease? = when (g.phase) {
        NavGesturePhase.IDLE -> null
        NavGesturePhase.PRESSED -> NavRelease.OpenPanel
        NavGesturePhase.PREVIEW ->
            if (abs(g.y - g.startY) < cancelSlopPx) NavRelease.Cancel
            else NavRelease.Jump(indexAtY(g.y, count, heightPx, tickHeightPx, gapPx))
    }

    /**
     * 预览气泡顶部 Y(px):气泡以"手指 Y + [yieldPx] 让位"为竖直中心(让位保证
     * 手指正下方的正文首行不被气泡盖住),再钳制在容器内不出屏。
     * 容器比气泡还矮(极端小窗)时整体居中。
     */
    fun bubbleTopY(
        fingerYPx: Float,
        bubbleHeightPx: Float,
        containerHeightPx: Float,
        yieldPx: Float,
    ): Float {
        if (containerHeightPx <= bubbleHeightPx) {
            return ((containerHeightPx - bubbleHeightPx) / 2f).coerceAtLeast(0f)
        }
        val centered = fingerYPx + yieldPx - bubbleHeightPx / 2f
        return centered.coerceIn(0f, containerHeightPx - bubbleHeightPx)
    }
}
