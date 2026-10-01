// ThreadsRowLogic.kt —— 会话列表行(R1-B 效率与质感轮)的纯逻辑。
//
// 【为什么单独一个文件】行级别的"能算错的部分"不进 Composable:统一时间格式、
// 四词状态筛选、置顶/最近分区拆分、空态文案与动作 —— 都是规则,抽成纯函数后
// 可以钉住单测(ThreadsRowLogicTest),不必真机点一遍才知道口径分没分叉。
package dev.pocketagent.app.ui

import dev.pocketagent.app.R
import java.text.SimpleDateFormat
import java.util.Calendar
import java.util.Date
import java.util.TimeZone

// ---------------------------------------------------------------- 四词状态筛选

/** 状态筛选值:空串 = 全部状态(与 THREAD_FILTER_ALL / AGENT_FILTER_ALL 同款单值语义)。 */
internal const val THREAD_STATUS_FILTER_ALL = ""

/**
 * 状态筛选档位(纯函数):值 + 展示词。
 *
 * 【词从哪来】直接取 [EditorialStatus] 的四词(等待你处理/正在执行/已完成/未完成)
 * —— 全 app 只有这四个状态词,筛选条不造第二套口径(README §状态)。
 * 任务书里的"进行中/待处理"落回本 app 的统一四词,避免同一状态两页两个叫法。
 */
internal fun threadsStatusFilterOptions(): List<Pair<String, String>> = listOf(
    THREAD_STATUS_FILTER_ALL to "全部状态",
    EditorialStatus.WAITING_FOR_YOU.name to EditorialStatus.WAITING_FOR_YOU.word,
    EditorialStatus.RUNNING.name to EditorialStatus.RUNNING.word,
    EditorialStatus.COMPLETED.name to EditorialStatus.COMPLETED.word,
    EditorialStatus.NOT_COMPLETED.name to EditorialStatus.NOT_COMPLETED.word,
)

/**
 * 某条会话是否命中当前状态筛选(纯函数)。
 *
 * 【口径】与行内徽章同一套映射:走 [editorialStatusOrNull](行内摆什么词,这里就
 * 按什么词筛,两处不可能分叉)。状态未知(UNSPECIFIED 且无审批无错误)的行
 * **只命中"全部状态"** —— 我们不知道它的状态,就不冒充它属于某一档;
 * 审批优先于运行、运行优先于失败,与徽章优先级一致。
 */
internal fun matchesStatusFilter(
    status: pocketagent.v1.ThreadStatus,
    pendingApprovals: Int,
    lastTurnError: String,
    filter: String,
): Boolean {
    if (filter == THREAD_STATUS_FILTER_ALL) return true
    val known = editorialStatusOrNull(status, pendingApprovals, lastTurnError) ?: return false
    return known.name == filter
}

// ---------------------------------------------------------------- 统一相对时间

/**
 * 会话行的统一时间标签(纯函数):刚刚 / N 分钟前 / 今天 HH:mm / 昨天 HH:mm /
 * MM-dd(跨年补 yyyy)。[zone] 注入以便 JVM 单测固定时区。
 *
 * 【为什么统一】此前列表三种行(本地 / 电脑 codex / 电脑 Pi/ZCode)各自
 * SimpleDateFormat("MM-dd HH:mm"):三十年前的会话和三分钟前的会话长得一样,
 * 与抽屉、主页的相对时间口径也对不上。现在行内右上角一律走本函数,
 * "刚发生"一眼可见,更久远自动退到日期(设计调研:对标 Telegram 的时间档位)。
 *
 * 【边界】时间戳缺席(<=0)返回空串;客户端时钟偏慢(时间戳在未来)回落绝对
 * 日期,不显示"负 N 分钟前"(与 relativeTimeLabel 同纪律)。
 */
internal fun threadRowTime(unixMs: Long, nowMs: Long, zone: TimeZone = TimeZone.getDefault()): String {
    if (unixMs <= 0L) return ""
    val diff = nowMs - unixMs
    if (diff >= 0L) {
        if (diff < 60_000L) return "刚刚"
        if (diff < 3_600_000L) return "${diff / 60_000L} 分钟前"
    }
    val dayStartMs = dayStart(unixMs, zone)
    val todayStartMs = dayStart(nowMs, zone)
    // 用"当天零点"的差换算天数并四舍五入:跨夏令时的 23/25 小时天不会被整除截断。
    val dayDiff = Math.round((todayStartMs - dayStartMs) / 86_400_000.0)
    return when {
        dayDiff == 0L -> "今天 " + formatTime(unixMs, "HH:mm", zone)
        dayDiff == 1L -> "昨天 " + formatTime(unixMs, "HH:mm", zone)
        sameYear(unixMs, nowMs, zone) -> formatTime(unixMs, "MM-dd", zone)
        else -> formatTime(unixMs, "yyyy-MM-dd", zone)
    }
}

/** UI 直用重载:now 取当前时钟(单测一律走三参纯函数)。 */
internal fun threadRowTime(unixMs: Long): String =
    threadRowTime(unixMs, System.currentTimeMillis())

private fun dayStart(ms: Long, zone: TimeZone): Long {
    val cal = Calendar.getInstance(zone)
    cal.timeInMillis = ms
    cal.set(Calendar.HOUR_OF_DAY, 0)
    cal.set(Calendar.MINUTE, 0)
    cal.set(Calendar.SECOND, 0)
    cal.set(Calendar.MILLISECOND, 0)
    return cal.timeInMillis
}

private fun sameYear(a: Long, b: Long, zone: TimeZone): Boolean {
    val ca = Calendar.getInstance(zone).apply { timeInMillis = a }
    val cb = Calendar.getInstance(zone).apply { timeInMillis = b }
    return ca.get(Calendar.YEAR) == cb.get(Calendar.YEAR)
}

private fun formatTime(ms: Long, pattern: String, zone: TimeZone): String {
    val fmt = SimpleDateFormat(pattern, java.util.Locale.getDefault())
    fmt.timeZone = zone
    return fmt.format(Date(ms))
}

// ---------------------------------------------------------------- 置顶分区拆分

/**
 * 把已排序的展示行拆成(置顶区, 最近区)两段(纯函数),组内保持原有顺序。
 *
 * 【为什么在排好序之后还拆】排序已把置顶排最前,但**分区头**需要知道"前几条是
 * 置顶、剩下多少条"才能各画各的小标题;直接在渲染处写 filter/filterNot 是同一份
 * 判定抄两遍,迟早一侧改动漂移。收口在这里,单测钉住顺序不被打乱。
 */
internal fun <T> splitPinnedFirst(items: List<T>, isPinned: (T) -> Boolean): Pair<List<T>, List<T>> =
    items.filter(isPinned) to items.filterNot(isPinned)

// ---------------------------------------------------------------- 空态(状态说明 + 下一步动作)

/** 空态的下一步动作(纯枚举,UI 据此挂按钮)。 */
internal enum class ThreadsEmptyAction {
    /** 纯说明,不挂动作(归档/分组视图的空是正常态)。 */
    NONE,
    /** 一条会话都没有:引导新建。 */
    NEW_THREAD,
    /** 有会话但被临时态(搜索/agent/状态筛选)筛空了:一键退出临时态。 */
    CLEAR_FILTERS,
}

/** 空态展示模型:图标 + 标题 + 说明 + 可选动作(空态不空白,见调研 4)。 */
internal data class ThreadsEmptyView(
    val iconRes: Int,
    val title: String,
    val hint: String,
    val action: ThreadsEmptyAction = ThreadsEmptyAction.NONE,
    val actionLabel: String = "",
)

/**
 * 会话列表空态(纯函数)。
 *
 * 【判定顺序】先"被筛空"再"真的没有":全量里有会话但当前临时态(搜索词 /
 * agent 筛选 / 状态筛选)一条都不剩时,复述"还没有会话"是在说谎 —— 用户明明
 * 看到过列表。此时给「清除筛选」一键回到全量(动作永远可达,不依赖网络)。
 *
 * @param anyThreads     全量(未过渲染筛选)里是否至少有一条会话
 * @param narrowingActive 搜索/agent/状态任一"收窄型"临时态是否生效
 * @param showArchived   当前是否为"已归档"视图
 * @param sectionName    当前分组名(非空 = 在某个分组视图里)
 * @param ungrouped      当前是否为"未分组"视图
 */
internal fun threadsEmptyState(
    anyThreads: Boolean,
    narrowingActive: Boolean,
    showArchived: Boolean,
    sectionName: String?,
    ungrouped: Boolean,
): ThreadsEmptyView = when {
    narrowingActive && anyThreads -> ThreadsEmptyView(
        iconRes = R.drawable.ic_lucide_search,
        title = "没有符合条件的结果",
        hint = "换个关键词,或清除筛选查看全部会话",
        action = ThreadsEmptyAction.CLEAR_FILTERS,
        actionLabel = "清除筛选",
    )
    showArchived -> ThreadsEmptyView(
        iconRes = R.drawable.ic_lucide_archive,
        title = "没有归档的会话",
        hint = "左滑会话可归档,归档后在这里找回",
    )
    sectionName != null -> ThreadsEmptyView(
        iconRes = R.drawable.ic_lucide_folder,
        title = "「${sectionName.ifBlank { "未命名分组" }}」还没有会话",
        hint = "长按会话可把它移进分组",
    )
    ungrouped -> ThreadsEmptyView(
        iconRes = R.drawable.ic_lucide_message_square,
        title = "没有未分组的会话",
        hint = "长按会话可把它移进分组",
    )
    else -> ThreadsEmptyView(
        iconRes = R.drawable.ic_lucide_message_square,
        title = "还没有会话",
        hint = "从电脑挑个工作文件夹,开始你的第一个任务",
        action = ThreadsEmptyAction.NEW_THREAD,
        actionLabel = "新建会话",
    )
}
