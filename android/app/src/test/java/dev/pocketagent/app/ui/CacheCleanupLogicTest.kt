package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 缓存清理「风险分级 + 清理目标筛选」的钉子(需求④)。
 *
 * 钉三条不许反转的规则(反转都没有任何报错,只能等用户投诉才暴露):
 * 1. 只有「会话快照」是高风险(RISKY),需二次确认;其余可再生项都是低风险;
 * 2. 凭据(token)永远不是可清理目标,无论风险档怎么标;
 * 3. 高风险项筛选([riskyItems])只挑出会话快照,低风险项([safeItems])排除高风险与凭据。
 */
class CacheCleanupLogicTest {

    // ---- 风险分级 ----

    @Test
    fun `only the session snapshot is risky, everything else is safe`() {
        // 会话快照:清了只能从电脑重拉,离线看不了 → 必须二次确认
        assertEquals(CacheCleanupLogic.Risk.RISKY, CacheCleanupLogic.riskOf(CacheCleanupLogic.Kind.SNAPSHOT))
        // 其余都是可再生/可重登的低风险项
        assertEquals(CacheCleanupLogic.Risk.SAFE, CacheCleanupLogic.riskOf(CacheCleanupLogic.Kind.URL_SNAPSHOT))
        assertEquals(CacheCleanupLogic.Risk.SAFE, CacheCleanupLogic.riskOf(CacheCleanupLogic.Kind.PREVIEW))
        assertEquals(CacheCleanupLogic.Risk.SAFE, CacheCleanupLogic.riskOf(CacheCleanupLogic.Kind.TEMP))
        assertEquals(CacheCleanupLogic.Risk.SAFE, CacheCleanupLogic.riskOf(CacheCleanupLogic.Kind.BROWSER))
    }

    // ---- 凭据排除(绝不入清理目标) ----

    @Test
    fun `credential is never a clearable target`() {
        assertTrue(CacheCleanupLogic.isCredential(CacheCleanupLogic.Kind.CREDENTIAL))
        assertFalse(CacheCleanupLogic.isCredential(CacheCleanupLogic.Kind.SNAPSHOT))
        assertFalse(CacheCleanupLogic.isClearable(CacheCleanupLogic.Kind.CREDENTIAL))
        assertTrue(CacheCleanupLogic.isClearable(CacheCleanupLogic.Kind.SNAPSHOT))
    }

    @Test
    fun `filterClearable drops credentials and keeps the rest`() {
        val items = listOf(
            CacheCleanupLogic.Item("会话快照", CacheCleanupLogic.Kind.SNAPSHOT, 100),
            CacheCleanupLogic.Item("网页快照", CacheCleanupLogic.Kind.URL_SNAPSHOT, 200),
            CacheCleanupLogic.Item("token", CacheCleanupLogic.Kind.CREDENTIAL, 50),
        )
        val cleared = CacheCleanupLogic.filterClearable(items)
        assertEquals(2, cleared.size)
        assertFalse("凭据绝不入清理目标", cleared.any { it.kind == CacheCleanupLogic.Kind.CREDENTIAL })
    }

    // ---- 清理目标筛选(高风险 / 低风险) ----

    @Test
    fun `riskyItems isolates only the session snapshot`() {
        val items = listOf(
            CacheCleanupLogic.Item("会话快照", CacheCleanupLogic.Kind.SNAPSHOT, 100),
            CacheCleanupLogic.Item("网页快照", CacheCleanupLogic.Kind.URL_SNAPSHOT, 200),
            CacheCleanupLogic.Item("临时文件", CacheCleanupLogic.Kind.TEMP, 80),
            CacheCleanupLogic.Item("token", CacheCleanupLogic.Kind.CREDENTIAL, 50),
        )
        val risky = CacheCleanupLogic.riskyItems(items)
        assertEquals(1, risky.size)
        assertEquals("会话快照", risky.first().key)
    }

    @Test
    fun `safeItems excludes both risky and credentials`() {
        val items = listOf(
            CacheCleanupLogic.Item("会话快照", CacheCleanupLogic.Kind.SNAPSHOT, 100),
            CacheCleanupLogic.Item("网页快照", CacheCleanupLogic.Kind.URL_SNAPSHOT, 200),
            CacheCleanupLogic.Item("token", CacheCleanupLogic.Kind.CREDENTIAL, 50),
        )
        val safe = CacheCleanupLogic.safeItems(items)
        // 高风险(会话快照)与凭据都被排除,只剩可被直清的低风险项
        assertEquals(listOf("网页快照"), safe.map { it.key })
        assertTrue(safe.none { it.kind == CacheCleanupLogic.Kind.SNAPSHOT })
        assertTrue(safe.none { it.kind == CacheCleanupLogic.Kind.CREDENTIAL })
    }
}
