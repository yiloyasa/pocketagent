package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Test
import pocketagent.v1.Usage

/**
 * 上下文窗口卡的「指标表」取值。
 *
 * 指标表是"以后加一项只加一行"的载体,所以单测只钉两件事:①确实只有 4 项;
 * ②每项取的是对的字段(缓存取 cacheReadTokens,与命中率同口径)。
 */
class WorkflowAUsageMetricsTest {

    @Test
    fun only_four_metrics_are_shown() {
        assertEquals(
            listOf("输入", "输出", "缓存", "缓存命中率"),
            usageMetrics().map { it.label },
        )
    }

    @Test
    fun metric_values_read_the_expected_usage_fields() {
        val u = Usage.newBuilder()
            .setInputTokens(1_000)
            .setOutputTokens(200)
            .setCacheReadTokens(500)
            .build()
        val byLabel = usageMetrics().associate { it.label to it.value(u) }
        assertEquals("1,000", byLabel["输入"])
        assertEquals("200", byLabel["输出"])
        assertEquals("500", byLabel["缓存"])
        assertEquals("50.0%", byLabel["缓存命中率"])
    }

    @Test
    fun null_usage_renders_zeros_and_a_dash_not_a_crash() {
        val byLabel = usageMetrics().associate { it.label to it.value(null) }
        assertEquals("0", byLabel["输入"])
        assertEquals("0", byLabel["输出"])
        assertEquals("0", byLabel["缓存"])
        // 输入为 0 时命中率给"—"(不造数)
        assertEquals("—", byLabel["缓存命中率"])
    }
}
