package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.OutputState

/**
 * 「没有正文」提示的判定与文案(方案 §4.4 图二 / §7.2 输出契约)。
 *
 * 【钉住的用户症状】旧实现让 daemon 往 final_blocks 里塞一句 "(完成,无文本输出)"
 * 当**正文**,手机端照单显示 —— 三种完全不同的情况(真的没话说 / 只产出了产物 /
 * 正文丢了)被压成同一句话,用户无从判断该不该重试。
 *
 * 现在 daemon 只报事实(OutputState),措辞在客户端定。本组用例钉住两件事:
 * 哪一档要提示,以及"补回来了就别再提示"。
 */
class EmptyOutputNoticeTest {

    @Test
    fun emptyStateExplainsThereIsNoOutput() {
        val notice = emptyOutputNotice(OutputState.OUTPUT_STATE_EMPTY)
        assertNotNull("「真的没话说」必须给用户一个说法,否则时间线留白无从解释", notice)
        assertEquals("本轮没有正文", notice!!.first)
        assertTrue("要说明既没文字也没附件", notice.second.contains("没有文字输出"))
    }

    @Test
    fun artifactOnlyIsToldApartFromEmpty() {
        val artifact = emptyOutputNotice(OutputState.OUTPUT_STATE_ARTIFACT_ONLY)
        val empty = emptyOutputNotice(OutputState.OUTPUT_STATE_EMPTY)
        assertNotNull(artifact)
        assertNotNull(empty)
        assertNotEquals(
            "「只有产物」与「真没话说」必须是两句不同的话 —— 用户据此决定去附件里找还是重问",
            artifact!!.first, empty!!.first,
        )
        assertTrue("要指向产物在哪", artifact.second.contains("产物"))
    }

    @Test
    fun adapterLostTellsUserToRetry() {
        val notice = emptyOutputNotice(OutputState.OUTPUT_STATE_ADAPTER_LOST_OUTPUT)
        assertNotNull(notice)
        assertTrue(
            "这一档是我们没能取回正文,必须明确告诉用户可以重试 —— 归成「没正文」等于替用户断言没有",
            notice!!.second.contains("重试"),
        )
    }

    @Test
    fun textStateNeedsNoNotice() {
        assertNull("有正文就不该再补话", emptyOutputNotice(OutputState.OUTPUT_STATE_TEXT))
    }

    @Test
    fun recoveredStateNeedsNoNotice() {
        assertNull(
            "已经从原生历史补回正文了,再提示「没有正文」就是自相矛盾",
            emptyOutputNotice(OutputState.OUTPUT_STATE_RECOVERED_FROM_NATIVE_HISTORY),
        )
    }

    @Test
    fun unspecifiedStateNeedsNoNotice() {
        assertNull(
            "UNSPECIFIED 是旧 daemon 的回放帧(那时没这个字段):" +
                "此刻编一句话等于把「对方没报状态」说成「这一轮确实没正文」",
            emptyOutputNotice(OutputState.OUTPUT_STATE_UNSPECIFIED),
        )
    }
}
