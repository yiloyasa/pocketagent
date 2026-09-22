package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 断流恢复的轮询策略(方案 §4.6 / §9.3)。
 *
 * 【钉住的用户症状】断流恢复此前是"首等 800ms、之后每 5 秒一次"的固定轮询 ——
 * 网络回来的那一刻平均要等 2.5 秒、最坏 5 秒才开始补帧,用户体感就是
 * "网回来了,手机还不动"。更糟的是**每个周期都跑一次 GetThreadHistory 全量回填**,
 * 而方案 §9.3 明确要求"只有事件丢窗时才调用全量历史" —— 增量已经把缺口补上了,
 * 全量那一趟纯属白跑(还整屏重画)。
 *
 * 本组用例钉两条判据,它们是上面两个问题的直接对策。
 */
class TurnRecoveryPolicyTest {

    // ------------------------------------------------------------ 退避节奏

    @Test
    fun `首试必须很快 —— 网络刚回来那一刻就该补`() {
        val first = TurnSession.recoveryBackoffMs(0)
        assertTrue(
            "首试仍慢于 300ms 就等于没解决「网回来了手机还不动」, got ${first}ms",
            first <= 300L,
        )
        assertTrue(
            "首试必须显著快于旧实现的 800ms, got ${first}ms",
            first < 800L,
        )
    }

    @Test
    fun `退避单调不降且有封顶`() {
        var prev = 0L
        for (i in 0 until 12) {
            val cur = TurnSession.recoveryBackoffMs(i)
            assertTrue("退避不能回退小: attempt=$i got ${cur}ms, prev ${prev}ms", cur >= prev)
            prev = cur
        }
        assertTrue("必须有封顶,否则长任务会一直被高频打扰", prev <= 3_000L)
        assertEquals(
            "封顶之后要保持恒定",
            TurnSession.recoveryBackoffMs(20),
            TurnSession.recoveryBackoffMs(200),
        )
    }

    @Test
    fun `退避整体不慢于旧实现的 5 秒`() {
        // 旧实现:首等 800ms、其后 5000ms。新的任何一档都不该比它慢。
        for (i in 0 until 12) {
            assertTrue(
                "attempt=$i 的间隔 ${TurnSession.recoveryBackoffMs(i)}ms 比旧实现的 5000ms 还慢",
                TurnSession.recoveryBackoffMs(i) <= 5_000L,
            )
        }
    }

    // ------------------------------------------------------------ 全量回退判据

    @Test
    fun `丢窗时必须回退全量 —— 断点已经作废,增量无从谈起`() {
        assertTrue(
            TurnSession.needFullHistoryFallback(TurnSession.ReplayOutcome.RESYNC, hasResumePoint = true),
        )
    }

    @Test
    fun `没有断点时只能走全量`() {
        assertTrue(
            "lastSequence<=0 时增量本就没有起点",
            TurnSession.needFullHistoryFallback(null, hasResumePoint = false),
        )
    }

    @Test
    fun `有断点、只是这轮没连上时不该整屏重拉`() {
        assertFalse(
            "连不上是暂时状态,下轮接着增量就行;此刻全量会把整屏重画一遍",
            TurnSession.needFullHistoryFallback(null, hasResumePoint = true),
        )
    }

    @Test
    fun `补齐但未终局时不该整屏重拉`() {
        assertFalse(
            TurnSession.needFullHistoryFallback(TurnSession.ReplayOutcome.CAUGHT_UP, hasResumePoint = true),
        )
    }

    @Test
    fun `补到终局时不该整屏重拉`() {
        assertFalse(
            TurnSession.needFullHistoryFallback(TurnSession.ReplayOutcome.TERMINAL, hasResumePoint = true),
        )
    }
}
