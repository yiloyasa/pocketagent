package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 会话组织规则(筛选 / 回滚档位 / 后果文案)。
 *
 * 这些规则错了不会崩,只会"悄悄删掉用户看得见的东西"或"筛出来一份看不懂的列表",
 * 所以必须靠单测钉住 —— 尤其是"归档会话不得混进分组视图"和"回滚文案必须
 * 提到不会撤销文件"这两条。
 */
class ThreadOrganizeTest {

    // ---- 筛选 ----

    @Test
    fun all_filter_shows_only_active_threads() {
        assertTrue(matchesThreadFilter("", false, THREAD_FILTER_ALL))
        assertTrue(matchesThreadFilter("sec-1", false, THREAD_FILTER_ALL))
        // 归档的不出现在"全部"里 —— 它有自己的一格
        assertFalse(matchesThreadFilter("", true, THREAD_FILTER_ALL))
        assertFalse(matchesThreadFilter("sec-1", true, THREAD_FILTER_ALL))
    }

    @Test
    fun archived_filter_shows_only_archived_threads() {
        assertTrue(matchesThreadFilter("", true, THREAD_FILTER_ARCHIVED))
        assertTrue(matchesThreadFilter("sec-1", true, THREAD_FILTER_ARCHIVED))
        assertFalse(matchesThreadFilter("", false, THREAD_FILTER_ARCHIVED))
    }

    /**
     * 关键回归:分组视图里绝不能出现归档会话。
     * 否则同一条会话会在「已归档」和它所属分组下各出现一次,用户会以为有两份。
     */
    @Test
    fun section_filter_never_leaks_archived_threads() {
        assertTrue(matchesThreadFilter("sec-1", false, "sec-1"))
        assertFalse(matchesThreadFilter("sec-1", true, "sec-1"))
        assertFalse(matchesThreadFilter("sec-2", false, "sec-1"))
        assertFalse(matchesThreadFilter("", false, "sec-1"))
    }

    @Test
    fun ungrouped_filter_matches_only_threads_without_a_section() {
        assertTrue(matchesThreadFilter("", false, THREAD_FILTER_UNGROUPED))
        assertFalse(matchesThreadFilter("sec-1", false, THREAD_FILTER_UNGROUPED))
        assertFalse(matchesThreadFilter("", true, THREAD_FILTER_UNGROUPED))
    }

    @Test
    fun section_filter_is_exact_equality_not_a_prefix_match() {
        // 分组 id 由 codex 生成,我们不对它的字符集做假设:只要不是我们占住的两个
        // 哨兵键,任何字符串都按"分组 id"处理。而且必须**精确相等** ——
        // 前缀命中会让 "sec" 这种短 id 把 "sec-1" 的会话也拉进来。
        assertTrue(matchesThreadFilter("abc-def", false, "abc-def"))
        assertFalse(matchesThreadFilter("abc-def", false, "abc"))
        assertFalse(matchesThreadFilter("abc", false, "abc-def"))
    }

    // ---- 回滚档位 ----

    @Test
    fun no_turns_means_no_rollback_options_at_all() {
        // 没有回合就不该给入口:弹出来也只能失败
        assertTrue(rollbackOptions(0).isEmpty())
        assertTrue(rollbackOptions(-3).isEmpty())
    }

    @Test
    fun rollback_offers_fixed_steps_plus_the_total_without_duplicates() {
        assertEquals(listOf(1), rollbackOptions(1))
        assertEquals(listOf(1, 2), rollbackOptions(2))
        assertEquals(listOf(1, 2, 3), rollbackOptions(3))
        // 4 个回合:1/2/3 之外补一个"全部",不能出现两个 4
        assertEquals(listOf(1, 2, 3, 4), rollbackOptions(4))
        // 6 个回合:固定档位只保留 < 6 的,末尾是"全部"
        assertEquals(listOf(1, 2, 3, 6), rollbackOptions(6))
    }

    @Test
    fun option_labels_distinguish_the_total_from_a_plain_count() {
        assertEquals("1 回合", rollbackOptionLabel(1, 6))
        assertEquals("3 回合", rollbackOptionLabel(3, 6))
        assertEquals("全部（6 回合）", rollbackOptionLabel(6, 6))
    }

    // ---- 后果文案(安全信息) ----

    /**
     * 关键回归:`thread/rollback` 只改历史,**不回滚文件**。
     * 文案里丢掉这句话,用户就会把"回滚"当成"撤销"用。
     */
    @Test
    fun rollback_warning_states_the_turn_count_and_that_files_are_untouched() {
        val text = rollbackConsequence(2)
        assertTrue(text.contains("2 个回合"))
        assertTrue("必须说明文件不会被撤销", text.contains("不会"))
        assertTrue("必须指出文件改动要另想办法", text.contains("文件"))
        assertTrue("必须指向 Git", text.contains("Git"))
        assertTrue("必须说明不可逆", text.contains("找不回来"))
    }

    @Test
    fun fork_warning_says_the_original_thread_stays_untouched() {
        val text = forkConsequence()
        assertTrue(text.contains("原会话"))
        assertTrue(text.contains("不变"))
    }
}
