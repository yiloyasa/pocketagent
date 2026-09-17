package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.CodexEvent

/**
 * codex 事件在聊天时间线上的取舍规则。
 *
 * 【为什么值得专门钉住】这套规则的失效方式是"静默"的,两个方向都很难靠肉眼发现:
 * 判太严 → 审查驳回这类必须知情的事件消失,用户只看到"任务莫名失败";
 * 判太松 → daemon 挂在兜底 STATUS 上的高频流式通知(命令输出、Token 用量、
 *          计划增量)涌进时间线,聊天页被灌爆。
 * 尤其是第二条:STATUS 这个名字完全看不出它是"流式兜底",重构时非常容易被
 * 顺手放进"默认显示"里。所以下面专门有一条用例锁住它。
 */
class CodexEventTimelineTest {

    private fun event(kind: CodexEvent.Kind) = CodexEvent.newBuilder().setKind(kind).build()

    @Test
    fun guardian_review_is_shown_because_user_may_need_to_override_it() {
        assertTrue(shouldShowCodexEventInTimeline(event(CodexEvent.Kind.GUARDIAN_REVIEW)))
    }

    @Test
    fun informational_kinds_are_shown() {
        val shown = listOf(
            CodexEvent.Kind.TURN_DIFF,
            CodexEvent.Kind.DEPRECATION,
            CodexEvent.Kind.CONFIG_WARNING,
            CodexEvent.Kind.SANDBOX_WARNING,
            CodexEvent.Kind.HOOK,
            CodexEvent.Kind.SKILLS_CHANGED,
            CodexEvent.Kind.QUEUE,
            CodexEvent.Kind.MODEL_SAFETY,
            CodexEvent.Kind.WARNING,
            CodexEvent.Kind.MODEL_REROUTED,
            CodexEvent.Kind.CONTEXT_COMPACTED,
        )
        for (kind in shown) {
            assertTrue("$kind 应当出现在时间线上", shouldShowCodexEventInTimeline(event(kind)))
        }
    }

    /**
     * 回归线:STATUS 是 daemon 的兜底 kind,承载命令输出/Token 用量/计划增量等
     * 高频流式通知。它一旦进时间线,聊天页会被每秒数条的增量挤满。
     */
    @Test
    fun streaming_and_diagnostic_kinds_stay_out_of_the_timeline() {
        val hidden = listOf(
            CodexEvent.Kind.STATUS,
            CodexEvent.Kind.ERROR,
            CodexEvent.Kind.RETRY_STARTED,
            CodexEvent.Kind.RETRY_STOPPED,
            CodexEvent.Kind.RATE_LIMIT,
            CodexEvent.Kind.CODEX_EVENT_UNSPECIFIED,
        )
        for (kind in hidden) {
            assertFalse("$kind 不应该出现在时间线上", shouldShowCodexEventInTimeline(event(kind)))
        }
    }

    @Test
    fun same_review_updates_in_place_instead_of_appending_a_second_card() {
        // codex 对一次审查会先发 started、再发 completed,两条都要落在同一张卡上。
        val started = CodexEvent.newBuilder()
            .setKind(CodexEvent.Kind.GUARDIAN_REVIEW)
            .setReviewId("rv-1")
            .setReviewStatus("inProgress")
            .build()
        val completed = CodexEvent.newBuilder()
            .setKind(CodexEvent.Kind.GUARDIAN_REVIEW)
            .setReviewId("rv-1")
            .setReviewStatus("denied")
            .build()

        assertEquals(codexEventTimelineKey(started), codexEventTimelineKey(completed))
        assertEquals("review:rv-1", codexEventTimelineKey(completed))
    }

    @Test
    fun repeated_turn_diff_and_queue_notifications_collapse_into_one_card() {
        // diff 与队列通知没有 reviewId,一轮里会反复到达,只能靠 kind 去重。
        val diff = CodexEvent.newBuilder().setKind(CodexEvent.Kind.TURN_DIFF).setDiff("+a").build()
        assertTrue(codexEventTimelineKey(diff).isNotEmpty())
        assertEquals(
            codexEventTimelineKey(diff),
            codexEventTimelineKey(
                CodexEvent.newBuilder().setKind(CodexEvent.Kind.TURN_DIFF).setDiff("+b").build(),
            ),
        )
    }

    @Test
    fun one_off_kinds_get_no_key_so_they_each_keep_their_own_card() {
        // 技能变化、弃用警告这些是独立事实,不该互相覆盖。
        assertEquals("", codexEventTimelineKey(event(CodexEvent.Kind.SKILLS_CHANGED)))
        assertEquals("", codexEventTimelineKey(event(CodexEvent.Kind.DEPRECATION)))
    }

    @Test
    fun only_denied_or_unfinished_reviews_can_be_overridden() {
        fun review(status: String) = CodexEvent.newBuilder()
            .setKind(CodexEvent.Kind.GUARDIAN_REVIEW)
            .setReviewId("rv-9")
            .setReviewStatus(status)
            .build()

        assertTrue(guardianOverridable(review("denied")))
        assertTrue(guardianOverridable(review("timedOut")))
        assertTrue(guardianOverridable(review("aborted")))

        // 已放行的没什么可推翻;还在评估中的点了也无效(daemon 侧同样会拒)。
        assertFalse(guardianOverridable(review("approved")))
        assertFalse(guardianOverridable(review("inProgress")))
    }

    @Test
    fun override_entry_needs_a_review_id_to_send_back() {
        val noId = CodexEvent.newBuilder()
            .setKind(CodexEvent.Kind.GUARDIAN_REVIEW)
            .setReviewStatus("denied")
            .build()
        assertFalse(guardianOverridable(noId))
    }

    @Test
    fun non_review_events_are_never_overridable() {
        assertFalse(guardianOverridable(event(CodexEvent.Kind.TURN_DIFF)))
        assertFalse(guardianOverridable(event(CodexEvent.Kind.ERROR)))
    }

    @Test
    fun review_status_and_risk_level_get_chinese_labels() {
        // 这几个字符串是 codex 通知里的原样取值(camelCase),改写就等于失配。
        assertEquals("审查中", guardianStatusLabel("inProgress"))
        assertEquals("已放行", guardianStatusLabel("approved"))
        assertEquals("已驳回", guardianStatusLabel("denied"))
        assertEquals("审查超时", guardianStatusLabel("timedOut"))
        assertEquals("审查中止", guardianStatusLabel("aborted"))
        assertEquals("", guardianStatusLabel(""))

        assertEquals("低风险", riskLevelLabel("low"))
        assertEquals("中风险", riskLevelLabel("medium"))
        assertEquals("高风险", riskLevelLabel("high"))
        assertEquals("极高风险", riskLevelLabel("critical"))
        // 大小写不做要求,codex 曾经大小写不统一过。
        assertEquals("高风险", riskLevelLabel("HIGH"))
        // 没给等级时不显示徽标,而不是显示一个空壳。
        assertEquals("", riskLevelLabel(""))
    }
}
