package dev.pocketagent.app.ui

import dev.pocketagent.app.AgentConnection
import dev.pocketagent.app.ui.component.markCompactedJson
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

/**
 * 会话配置(模型/思考强度)与上下文压缩同步的回归测试。
 *
 * 背景:思考强度曾经是"全局单值 + 不落盘" —— A 会话调了强度 B 会话跟着变,
 * 重启 App 整个丢失;模型已是"全局默认 + 按会话覆盖",强度必须同构,
 * 否则 UI 显示与实际生效对不上(用户反馈"切换模型与思考强度时显示不正确")。
 */
class SessionConfigAndCompactionTest {

    private var savedEffort = ""
    private var savedDefault = ""

    @Before
    fun setUp() {
        savedEffort = AgentConnection.selectedEffort
        savedDefault = AgentConnection.defaultEffort
    }

    @After
    fun tearDown() {
        AgentConnection.removeThreadEffort("t-a")
        AgentConnection.removeThreadEffort("t-b")
        AgentConnection.setGlobalEffort(savedDefault)
        AgentConnection.selectedEffort = savedEffort
    }

    @Test
    fun threadEffortOverridesGlobalDefaultPerSession() {
        AgentConnection.setGlobalEffort("medium")
        // 会话 A 调成 high:只影响 A,不污染全局默认与其它会话
        AgentConnection.setThreadEffort("t-a", "high")
        assertEquals("high", AgentConnection.threadEffort("t-a"))
        assertNull("会话 B 从未选过,应为 null(回落到全局默认)", AgentConnection.threadEffort("t-b"))
        assertEquals("medium", AgentConnection.defaultEffort)
        // 进入会话 B 时的取值语义:覆盖值缺省 → 全局默认
        assertEquals("medium", AgentConnection.threadEffort("t-b") ?: AgentConnection.defaultEffort)
        // 进入会话 A 时的取值语义:覆盖值优先
        assertEquals("high", AgentConnection.threadEffort("t-a") ?: AgentConnection.defaultEffort)
    }

    @Test
    fun setThreadEffortUpdatesEffectiveValueImmediately() {
        AgentConnection.setThreadEffort("t-a", "low")
        assertEquals("low", AgentConnection.selectedEffort)
    }

    @Test
    fun emptyThreadEffortIsExplicitFollowDesktop() {
        // 空串是有效覆盖值(用户明确选"默认/跟随桌面"),不等于"从未选过"
        AgentConnection.setThreadEffort("t-a", "")
        assertEquals("", AgentConnection.threadEffort("t-a"))
    }

    @Test
    fun setGlobalEffortPersistsAsDefaultAndEffective() {
        AgentConnection.setGlobalEffort("high")
        assertEquals("high", AgentConnection.defaultEffort)
        assertEquals("high", AgentConnection.selectedEffort)
    }

    // ---- 上下文压缩同步 ----

    @Test
    fun compactionPendingOnlyAfterCompactAndBeforeFreshUsage() {
        val usage = pocketagent.v1.Usage.newBuilder().setInputTokens(10).build()
        assertFalse("没压缩过:不进入压缩态", compactionPending(null, 0L))
        assertTrue("压缩后、新用量前:进入压缩态", compactionPending(null, 123L))
        assertFalse("新用量上报后自动退出压缩态", compactionPending(usage, 123L))
    }

    @Test
    fun compactionNoticeLabelIsHonest() {
        assertEquals("上下文已压缩 · 等待新一轮用量上报", compactionNoticeLabel(0L, 1000L))
        val label = compactionNoticeLabel(System.currentTimeMillis(), System.currentTimeMillis())
        assertTrue(label.contains("压缩"))
        assertTrue(label.contains("等待新一轮用量上报"))
    }

    @Test
    fun markCompactedJsonDropsStaleTurnButKeepsTotalAndWindow() {
        val raw = org.json.JSONObject()
            .put("turn", org.json.JSONObject().put("in", 999999))
            .put("total", org.json.JSONObject().put("in", 5))
            .put("cw", 272000)
            .toString()
        val out = org.json.JSONObject(markCompactedJson(raw, 777L))
        assertFalse("陈旧占用必须作废", out.has("turn"))
        assertTrue("累计用量不受压缩影响", out.has("total"))
        assertEquals(272000L, out.optLong("cw"))
        assertEquals(777L, out.optLong("compactAt"))
    }

    @Test
    fun markCompactedJsonToleratesMissingOrCorruptedInput() {
        val fromNull = org.json.JSONObject(markCompactedJson(null, 1L))
        assertEquals(1L, fromNull.optLong("compactAt"))
        val fromBad = org.json.JSONObject(markCompactedJson("{oops", 2L))
        assertEquals(2L, fromBad.optLong("compactAt"))
    }
}
