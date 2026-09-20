package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 【F-D 历史按轮加载】纯决策的钉子:「加载更早」入口的显隐与下一轮数值。
 *
 * 重点钉三条不许反转的规则:
 * 1. 全量路径(导出/恢复/resync,requestedRounds=0)永远不显示入口;
 * 2. daemon 返回的轮数不足请求轮数 = 已经给到会话开头,不显示入口;
 * 3. 被截断的尾部落在轮边界(首条 user),入口显示;下一轮 = 累计 + 档位。
 */
class HistoryRoundsLogicTest {

    // ---- canLoadEarlier:入口显隐 ----

    @Test
    fun `truncated tail with user-first role shows the entry`() {
        // 拉了 20 轮,响应恰好 20 个 user 起始轮,首条 user:大概率被截断
        assertTrue(HistoryRoundsLogic.canLoadEarlier(firstRole = "user", roundCount = 20, requestedRounds = 20))
    }

    @Test
    fun `fewer rounds than requested means daemon reached the beginning`() {
        // 会话总共只有 5 轮,拉 20 轮全量返回:不该显示入口
        assertFalse(HistoryRoundsLogic.canLoadEarlier(firstRole = "user", roundCount = 5, requestedRounds = 20))
    }

    @Test
    fun `full-history paths never show the entry`() {
        // requestedRounds = 0 是导出/恢复/resync 的全量语义,即使首条 user 也不显示
        assertFalse(HistoryRoundsLogic.canLoadEarlier(firstRole = "user", roundCount = 500, requestedRounds = 0))
        assertFalse(HistoryRoundsLogic.canLoadEarlier(firstRole = "user", roundCount = 500, requestedRounds = -1))
    }

    @Test
    fun `tail not starting at a round boundary does not show the entry`() {
        // 首条不是 user:不落在轮边界,不符合截断形态,不显示(宁可少显示不误导)
        assertFalse(HistoryRoundsLogic.canLoadEarlier(firstRole = "assistant", roundCount = 20, requestedRounds = 20))
        assertFalse(HistoryRoundsLogic.canLoadEarlier(firstRole = null, roundCount = 20, requestedRounds = 20))
    }

    @Test
    fun `empty response never shows the entry`() {
        assertFalse(HistoryRoundsLogic.canLoadEarlier(firstRole = null, roundCount = 0, requestedRounds = 20))
    }

    // ---- nextRounds:下一轮数值 ----

    @Test
    fun `next rounds accumulates one step`() {
        assertEquals(30, HistoryRoundsLogic.nextRounds(20, 10))
        assertEquals(70, HistoryRoundsLogic.nextRounds(20, 50))
        assertEquals(40, HistoryRoundsLogic.nextRounds(20, 20))
    }

    @Test
    fun `next rounds never goes backwards`() {
        // 防御:步长异常(<=0)时不原地打转,至少 +1
        assertEquals(21, HistoryRoundsLogic.nextRounds(20, 0))
        assertEquals(21, HistoryRoundsLogic.nextRounds(20, -5))
    }

    // ---- 档位与文案 ----

    @Test
    fun `options are the four tiers with default 20`() {
        assertEquals(listOf(10, 20, 30, 50), HistoryRoundsLogic.STEP_OPTIONS)
        assertEquals(20, HistoryRoundsLogic.DEFAULT_ROUNDS)
    }

    @Test
    fun `clamp keeps valid tiers and falls back to default on dirt`() {
        assertEquals(10, HistoryRoundsLogic.clampRounds(10))
        assertEquals(30, HistoryRoundsLogic.clampRounds(30))
        assertEquals(50, HistoryRoundsLogic.clampRounds(50))
        // 脏值(旧版本/手改 prefs):一律回默认,不许把非法值带进请求参数
        assertEquals(20, HistoryRoundsLogic.clampRounds(0))
        assertEquals(20, HistoryRoundsLogic.clampRounds(-3))
        assertEquals(20, HistoryRoundsLogic.clampRounds(25))
        assertEquals(20, HistoryRoundsLogic.clampRounds(999))
    }

    @Test
    fun `label renders the tier`() {
        assertEquals("20轮", HistoryRoundsLogic.labelFor(20))
    }
}
