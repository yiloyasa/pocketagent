package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.ThreadStatus
import java.util.Calendar
import java.util.GregorianCalendar
import java.util.TimeZone

/**
 * 会话列表行纯逻辑(R1-B 效率与质感轮)的单测。
 *
 * 钉四件事:
 *  1. 行内统一时间档位:刚刚 / N 分钟前 / 今天 HH:mm / 昨天 HH:mm / MM-dd(跨年带年),
 *     "昨天"按**日历日**判定而不是 24 小时差值;
 *  2. 四词状态筛选:与行内徽章同一套映射(editorialStatusOrNull),未知状态不冒充任何档;
 *  3. 置顶/最近分区拆分:组内顺序不打乱;
 *  4. 空态 = 状态说明 + 下一步动作,且"被筛空"优先于"真的没有";
 *  5. 返回层级新增状态筛选层:插在搜索与分组筛选之间,搜索仍最先清。
 */
class ThreadsRowLogicTest {

    private val zone: TimeZone = TimeZone.getTimeZone("GMT")

    /** 2026-09-30 12:00 GMT。 */
    private val now = utcMs(2026, Calendar.SEPTEMBER, 30, 12, 0)

    private fun utcMs(y: Int, m: Int, d: Int, h: Int, min: Int): Long =
        GregorianCalendar(TimeZone.getTimeZone("GMT")).apply {
            clear()
            set(y, m, d, h, min, 0)
        }.timeInMillis

    // ---- threadRowTime(统一时间档位)----

    @Test
    fun `一分钟内报刚刚`() {
        assertEquals("刚刚", threadRowTime(now - 30_000L, now, zone))
        assertEquals("刚刚", threadRowTime(now - 59_999L, now, zone))
    }

    @Test
    fun `一小时内报N分钟前`() {
        assertEquals("5 分钟前", threadRowTime(now - 5L * 60_000L, now, zone))
        assertEquals("59 分钟前", threadRowTime(now - 59L * 60_000L, now, zone))
    }

    @Test
    fun `当天带今天与时刻`() {
        assertEquals(
            "今天 07:05",
            threadRowTime(utcMs(2026, Calendar.SEPTEMBER, 30, 7, 5), now, zone),
        )
    }

    @Test
    fun `昨天按日历日判定_跨午夜不是N小时前`() {
        // 26 小时前:按小时差值会写成"26 小时前",按日历日是昨天 —— 格式统一的钉子
        assertEquals(
            "昨天 10:00",
            threadRowTime(utcMs(2026, Calendar.SEPTEMBER, 29, 10, 0), now, zone),
        )
    }

    @Test
    fun `更早回落日期_同年不带年_跨年带年`() {
        assertEquals(
            "09-01",
            threadRowTime(utcMs(2026, Calendar.SEPTEMBER, 1, 8, 0), now, zone),
        )
        assertEquals(
            "2025-12-31",
            threadRowTime(utcMs(2025, Calendar.DECEMBER, 31, 8, 0), now, zone),
        )
    }

    @Test
    fun `时间戳在未来回落当天时刻不出现负数`() {
        assertEquals("今天 12:10", threadRowTime(now + 10L * 60_000L, now, zone))
    }

    @Test
    fun `时间戳缺席返回空串`() {
        assertEquals("", threadRowTime(0L, now, zone))
        assertEquals("", threadRowTime(-5L, now, zone))
    }

    // ---- matchesStatusFilter(四词状态筛选)----

    @Test
    fun `全部状态放行一切`() {
        assertTrue(
            matchesStatusFilter(ThreadStatus.THREAD_STATUS_UNSPECIFIED, 0, "", THREAD_STATUS_FILTER_ALL),
        )
        assertTrue(
            matchesStatusFilter(ThreadStatus.THREAD_STATUS_RUNNING, 0, "", THREAD_STATUS_FILTER_ALL),
        )
    }

    @Test
    fun `四档各归其位`() {
        val waiting = EditorialStatus.WAITING_FOR_YOU.name
        val running = EditorialStatus.RUNNING.name
        val completed = EditorialStatus.COMPLETED.name
        val notCompleted = EditorialStatus.NOT_COMPLETED.name
        assertTrue(matchesStatusFilter(ThreadStatus.THREAD_STATUS_WAITING_APPROVAL, 0, "", waiting))
        assertTrue(matchesStatusFilter(ThreadStatus.THREAD_STATUS_RUNNING, 0, "", running))
        assertTrue(matchesStatusFilter(ThreadStatus.THREAD_STATUS_IDLE, 0, "", completed))
        assertTrue(matchesStatusFilter(ThreadStatus.THREAD_STATUS_IDLE, 0, "中断原因", notCompleted))
    }

    @Test
    fun `审批优先于运行_挂着审批不归正在执行`() {
        val running = EditorialStatus.RUNNING.name
        val waiting = EditorialStatus.WAITING_FOR_YOU.name
        assertFalse(matchesStatusFilter(ThreadStatus.THREAD_STATUS_RUNNING, 1, "", running))
        assertTrue(matchesStatusFilter(ThreadStatus.THREAD_STATUS_RUNNING, 1, "", waiting))
    }

    @Test
    fun `未知状态不冒充任何档_只命中全部`() {
        for (f in listOf("WAITING_FOR_YOU", "RUNNING", "COMPLETED", "NOT_COMPLETED")) {
            assertFalse(matchesStatusFilter(ThreadStatus.THREAD_STATUS_UNSPECIFIED, 0, "", f))
        }
    }

    @Test
    fun `档位词与全app统一四词一致`() {
        assertEquals(
            listOf("全部状态", "等待你处理", "正在执行", "已完成", "未完成"),
            threadsStatusFilterOptions().map { it.second },
        )
        assertEquals(
            listOf(THREAD_STATUS_FILTER_ALL) + EditorialStatus.entries.map { it.name },
            threadsStatusFilterOptions().map { it.first },
        )
    }

    // ---- splitPinnedFirst(置顶/最近分区)----

    @Test
    fun `置顶分区拆分_组内顺序不打乱`() {
        val rows = listOf("a", "b", "c", "d")
        val (pinned, recent) = splitPinnedFirst(rows) { it == "b" || it == "d" }
        assertEquals(listOf("b", "d"), pinned)
        assertEquals(listOf("a", "c"), recent)
    }

    @Test
    fun `全置顶与全不置顶两个极端`() {
        val (allPinned, noneRecent) = splitPinnedFirst(listOf(1, 2, 3)) { true }
        assertEquals(listOf(1, 2, 3), allPinned)
        assertTrue(noneRecent.isEmpty())
        val (nonePinned, allRecent) = splitPinnedFirst(listOf(1, 2, 3)) { false }
        assertTrue(nonePinned.isEmpty())
        assertEquals(listOf(1, 2, 3), allRecent)
    }

    // ---- threadsEmptyState(状态说明 + 下一步动作)----

    @Test
    fun `有会话但被收窄筛空_给清除筛选动作`() {
        val v = threadsEmptyState(
            anyThreads = true, narrowingActive = true,
            showArchived = false, sectionName = null, ungrouped = false,
        )
        assertEquals("没有符合条件的结果", v.title)
        assertEquals(ThreadsEmptyAction.CLEAR_FILTERS, v.action)
        assertEquals("清除筛选", v.actionLabel)
    }

    @Test
    fun `真没有会话_给新建动作`() {
        val v = threadsEmptyState(
            anyThreads = false, narrowingActive = false,
            showArchived = false, sectionName = null, ungrouped = false,
        )
        assertEquals("还没有会话", v.title)
        assertEquals(ThreadsEmptyAction.NEW_THREAD, v.action)
        assertEquals("新建会话", v.actionLabel)
    }

    @Test
    fun `归档_分组_未分组视图的空是正常态_不挂动作`() {
        for (v in listOf(
            threadsEmptyState(false, false, true, null, false),
            threadsEmptyState(false, false, false, "分组A", false),
            threadsEmptyState(false, false, false, null, true),
        )) {
            assertEquals(ThreadsEmptyAction.NONE, v.action)
        }
        // 分组视图的空态要带上分组名,不能笼统说"还没有会话"
        assertTrue(
            threadsEmptyState(false, false, false, "分组A", false).title.contains("分组A"),
        )
    }

    @Test
    fun `被筛空优先于视图空_不谎报没有归档`() {
        // 在归档视图里搜索没命中:先回答"没搜到",而不是"没有归档的会话"
        val v = threadsEmptyState(
            anyThreads = true, narrowingActive = true,
            showArchived = true, sectionName = null, ungrouped = false,
        )
        assertEquals(ThreadsEmptyAction.CLEAR_FILTERS, v.action)
    }

    // ---- threadsBackTarget(返回层级补状态筛选层)----

    @Test
    fun `状态筛选插在搜索与分组筛选之间`() {
        assertEquals(
            ThreadsBackTarget.STATUS_FILTER,
            threadsBackTarget(
                false, false, false, false, false,
                hasQuery = false, filterActive = true, statusFilterActive = true,
            ),
        )
        assertEquals(
            ThreadsBackTarget.FILTER,
            threadsBackTarget(
                false, false, false, false, false,
                hasQuery = false, filterActive = true, statusFilterActive = false,
            ),
        )
    }

    @Test
    fun `搜索仍最先清_全清后回主板块`() {
        assertEquals(
            ThreadsBackTarget.SEARCH,
            threadsBackTarget(
                false, false, false, false, false,
                hasQuery = true, filterActive = true, statusFilterActive = true,
            ),
        )
        assertEquals(
            ThreadsBackTarget.HOME,
            threadsBackTarget(
                false, false, false, false, false,
                hasQuery = false, filterActive = false, statusFilterActive = false,
            ),
        )
    }
}
