package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.Usage

/**
 * QA 独立复核补充测试(项 2 圆环占用口径 + 项 5 组织文案)。
 *
 * 【为什么补这些】现有 WorkflowAUsageMetricsTest 只钉了"指标表 4 项的标签与取值",
 * 但**圆环占用**走的是另一个函数 [usageTotalTokens]、**命中率**公式是 [cacheHitRate] ——
 * 这两条是"用户看着数字判断窗口还剩多少"的直接依据,却没有独立断言。
 * 项 5 的 [ungroupedHint] / [rollbackSummary] 是本轮新增、且要求"点击之前可见"的
 * 安全文案,现有 ThreadOrganizeTest 只覆盖了 rollbackConsequence / forkConsequence。
 *
 * 只读断言,不改任何源码。
 */
class QAReviewUsageAndOrganizeTest {

    private fun usage(i: Long, o: Long, cr: Long) = Usage.newBuilder()
        .setInputTokens(i)
        .setOutputTokens(o)
        .setCacheReadTokens(cr)
        .build()

    // ---------------- 项 2:圆环占用口径 ----------------

    /**
     * 占用 = 输入 + 输出(OpenAI 语义下 cached ⊆ input、reasoning ⊆ output,绝不能重复计)。
     * 这条公式错了,圆环百分比会虚高,用户会误判"窗口要满了"。
     */
    @Test
    fun ring_occupancy_is_input_plus_output_without_double_counting() {
        // 即使 cacheRead 很大(部分命中),也不能再叠加到占用里
        assertEquals(1_200L, usageTotalTokens(usage(1_000, 200, 900)))
        assertEquals(0L, usageTotalTokens(null))
        assertEquals(0L, usageTotalTokens(usage(0, 0, 0)))
    }

    @Test
    fun cache_hit_rate_is_dash_for_unknown_and_zero_percent_is_not_dash() {
        // 无数据 / 输入为 0:不造数,给"—"
        assertEquals("—", cacheHitRate(null))
        assertEquals("—", cacheHitRate(usage(0, 0, 0)))
        // 输入有值但完全没命中:必须是真实的 0.0%,不能被当成"没有数据"
        assertEquals("0.0%", cacheHitRate(usage(1_000, 0, 0)))
        assertEquals("50.0%", cacheHitRate(usage(1_000, 0, 500)))
        // 1/3 四舍五入到一位小数
        assertEquals("33.3%", cacheHitRate(usage(3, 0, 1)))
    }

    @Test
    fun token_formatting_uses_group_separators() {
        assertEquals("0", formatTokens(0))
        assertEquals("999", formatTokens(999))
        assertEquals("1,000", formatTokens(1_000))
        assertEquals("1,234,567", formatTokens(1_234_567))
    }

    // ---------------- 项 5:组织文案(点击之前可见的安全说明) ----------------

    /**
     * "未分组"的解释必须在用户看到分组面板时就能读到,且要把最容易误解的两点说清:
     * 不是"已删除/已归档",删组不删会话。
     */
    @Test
    fun ungrouped_hint_disambiguates_delete_archive_and_group_removal() {
        val text = ungroupedHint()
        assertTrue("必须点明这是'未分组'", text.contains("未分组"))
        assertTrue("必须区分'不是已删除'", text.contains("不是已删除"))
        assertTrue("必须区分'不是已归档'", text.contains("已归档"))
        assertTrue("必须说明删组不删会话", text.contains("删除分组不会删会话"))
    }

    /**
     * "更多"面板里回滚入口的常驻说明:与 rollbackConsequence 同源,
     * 必须同时说清"记录找不回来"和"文件不会被撤销"。
     */
    @Test
    fun rollback_summary_is_honest_about_files_and_irreversibility() {
        val text = rollbackSummary()
        assertTrue("必须说明只改对话记录", text.contains("只改对话记录"))
        assertTrue("必须说明不可逆", text.contains("找不回来"))
        assertTrue("必须说明不动文件", text.contains("文件"))
        assertTrue("必须说明不会撤销", text.contains("不会"))
        assertTrue("必须指向 Git 撤销手段", text.contains("Git"))
    }

    /**
     * 分叉说明:原会话不变 + 之后两条独立。用户最怕"分叉把原会话改了"。
     */
    @Test
    fun fork_consequence_keeps_original_thread_intact() {
        val text = forkConsequence()
        assertTrue(text.contains("原会话"))
        assertTrue(text.contains("保持不变"))
        assertTrue(text.contains("互不影响"))
    }

    /**
     * 一致性:入口说明 [rollbackSummary] 与选中回合数后的 [rollbackConsequence]
     * 必须在"文件不会被撤销"这一点上口径一致 —— 两处走偏会让用户在点完之后
     * 才发现语义不同。
     */
    @Test
    fun rollback_entry_and_confirmation_agree_on_files_not_reverted() {
        val entry = rollbackSummary()
        val confirm = rollbackConsequence(2)
        assertTrue(entry.contains("不会") && entry.contains("文件"))
        assertTrue(confirm.contains("不会") && confirm.contains("文件"))
        assertTrue(confirm.contains("2 个回合"))
    }
}
