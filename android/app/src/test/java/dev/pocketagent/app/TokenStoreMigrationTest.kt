package dev.pocketagent.app

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * token 敏感库迁移/降级单测(纯 JVM,全用 [MemoryTokenStore],不碰 Android 类):
 *
 * 三态覆盖(验收要求):
 *  1. 有旧值迁移:明文库有 token、加密库为空 → 写入加密库 + 删明文键,返回加密库;
 *  2. 已迁移不重复:两边都有 → 以加密库为准不覆盖,只补删明文键("先写后删"两步间
 *     被杀的残留场景,天然幂等);
 *  3. 失败降级:Keystore 不可用(工厂 secureProvider 抛异常)→ 返回明文库兜底,
 *     token 无损可用且留有日志。
 * 另覆盖:无旧值不动作、迁移写入中断的两条取舍分支、AgentConnection 写路径接线。
 */
class TokenStoreMigrationTest {

    // ---------------- TokenMigration.plan(纯函数,map→map) ----------------

    @Test
    fun `plan migrates when only legacy has value`() {
        val p = TokenMigration.plan(
            legacy = mapOf(TokenMigration.KEY_TOKEN to "old-token"),
            secure = mapOf(TokenMigration.KEY_TOKEN to ""),
        )
        assertEquals(mapOf(TokenMigration.KEY_TOKEN to "old-token"), p.toSecure)
        assertEquals(setOf(TokenMigration.KEY_TOKEN), p.removeFromLegacy)
    }

    @Test
    fun `plan keeps secure value and cleans legacy when both exist`() {
        val p = TokenMigration.plan(
            legacy = mapOf(TokenMigration.KEY_TOKEN to "stale-token"),
            secure = mapOf(TokenMigration.KEY_TOKEN to "fresh-token"),
        )
        assertTrue("加密库已有值时不得被明文旧值覆盖", p.toSecure.isEmpty())
        assertEquals("残留明文键仍要补删", setOf(TokenMigration.KEY_TOKEN), p.removeFromLegacy)
    }

    @Test
    fun `plan is a no-op when legacy has no usable value`() {
        val p = TokenMigration.plan(
            legacy = mapOf(TokenMigration.KEY_TOKEN to "   "),
            secure = mapOf(TokenMigration.KEY_TOKEN to ""),
        )
        assertTrue("空白旧值视同无值,新装用户零动作", p.toSecure.isEmpty())
        assertTrue(p.removeFromLegacy.isEmpty())
    }

    // ---------------- TokenStores.create(工厂 I/O 与降级) ----------------

    @Test
    fun `create migrates legacy token into secure store and removes plaintext key`() {
        val legacy = MemoryTokenStore(mapOf(TokenMigration.KEY_TOKEN to "old-token"))
        val secure = MemoryTokenStore()
        val logs = mutableListOf<String>()

        val store = TokenStores.create(
            secureProvider = { secure },
            legacy = legacy,
            log = { logs.add(it) },
        )

        assertTrue("迁移成功应返回加密库", store === secure)
        assertEquals("token 已迁入加密库", "old-token", secure.get(TokenMigration.KEY_TOKEN))
        assertNull("明文库里的 token 键必须删除", legacy.get(TokenMigration.KEY_TOKEN))
        assertTrue("正常迁移不该打日志", logs.isEmpty())
    }

    @Test
    fun `create does not overwrite already migrated secure token`() {
        val legacy = MemoryTokenStore(mapOf(TokenMigration.KEY_TOKEN to "stale-token"))
        val secure = MemoryTokenStore(mapOf(TokenMigration.KEY_TOKEN to "fresh-token"))

        val store = TokenStores.create(secureProvider = { secure }, legacy = legacy)

        assertEquals("以加密库为准,不回迁明文旧值", "fresh-token", store.get(TokenMigration.KEY_TOKEN))
        assertNull("残留明文键仍被清掉(幂等补删)", legacy.get(TokenMigration.KEY_TOKEN))
    }

    @Test
    fun `create falls back to legacy when keystore unavailable`() {
        val legacy = MemoryTokenStore(mapOf(TokenMigration.KEY_TOKEN to "old-token"))
        val logs = mutableListOf<String>()

        val store = TokenStores.create(
            secureProvider = { error("Keystore broken") },
            legacy = legacy,
            log = { logs.add(it) },
        )

        assertTrue("Keystore 不可用应返回明文库兜底,不抛不崩", store === legacy)
        assertEquals("降级后 token 无损可用(不断连)", "old-token", store.get(TokenMigration.KEY_TOKEN))
        assertTrue("降级必须留日志", logs.single().contains("降级"))
    }

    @Test
    fun `create falls back to legacy when secure write fails before token lands`() {
        // 加密库写入即抛且 token 没落进去 → 回明文库(token 还在那边,不能丢)。
        val legacy = MemoryTokenStore(mapOf(TokenMigration.KEY_TOKEN to "old-token"))
        val brokenSecure = object : TokenStore {
            override fun get(key: String): String? = null
            override fun put(key: String, value: String) = throw IllegalStateException("disk full")
            override fun remove(key: String) {}
        }

        val store = TokenStores.create(secureProvider = { brokenSecure }, legacy = legacy)

        assertTrue(store === legacy)
        assertEquals("old-token", store.get(TokenMigration.KEY_TOKEN))
    }

    @Test
    fun `create keeps secure store when only legacy cleanup fails`() {
        // token 已全部落进加密库,删明文键阶段才炸 → 沿用加密库(值已在,不必回明文)。
        val legacy = MemoryTokenStore(mapOf(TokenMigration.KEY_TOKEN to "old-token"))
        val secure = MemoryTokenStore()
        val stubbornLegacy = object : TokenStore by legacy {
            override fun remove(key: String) = throw IllegalStateException("locked")
        }

        val store = TokenStores.create(secureProvider = { secure }, legacy = stubbornLegacy)

        assertTrue("敏感值已全在加密库时应沿用加密库", store === secure)
        assertEquals("old-token", secure.get(TokenMigration.KEY_TOKEN))
    }

    // ---------------- AgentConnection 写路径接线 ----------------

    @Test
    fun `agentconnection persists token through token store not prefs path`() {
        val savedToken = AgentConnection.token
        try {
            val store = MemoryTokenStore()
            AgentConnection.setTokenStoreForTest(store)

            AgentConnection.token = "tok-wired"
            AgentConnection.persistToken()

            assertEquals("token 写入必须落到敏感库", "tok-wired", store.get(TokenMigration.KEY_TOKEN))
            assertEquals("内存副本同步(gRPC 拦截器读它)", "tok-wired", AgentConnection.token)
        } finally {
            AgentConnection.setTokenStoreForTest(MemoryTokenStore())
            AgentConnection.token = savedToken
        }
    }
}
