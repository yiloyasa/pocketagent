package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.AgentCacheUsage
import pocketagent.v1.AgentCheckpointUsage
import pocketagent.v1.AgentMemoryUsage
import pocketagent.v1.AgentRecoveryState
import pocketagent.v1.AgentRuntimeResponse

/**
 * §9.5「每家都能显示」后四块(memory/cache/checkpoint/recovery)映射的钉子。
 *
 * 【为什么必须钉】这四块从 daemon 到手机隔了两层(proto → UI 模型),最容易犯的错是
 * **静默造值**:proto3 的消息字段缺席时读 getter 得到全零对象,"旧 daemon 没这一块"
 * 就会被画成"0 会话 / 0 条 / 0 B" —— 看起来有数据,其实是编的。所以"缺席"与
 * "真实观测到 0"必须分得开:缺席说"daemon 未提供",后者照实显示 0。
 * 另有一条硬红线(AUDIT §3.3):占位语里不得出现"未知/错误"。
 */
class AgentRuntimeUsageRowsTest {

    private fun fullResp(): AgentRuntimeResponse = AgentRuntimeResponse.newBuilder()
        .setMemory(
            AgentMemoryUsage.newBuilder()
                .setThreads(3).setItems(42)
                .setBytes((3L shl 20) / 2) // 1.5 MB,正好落在 formatBytesLabel 的换算档上
                .putKinds("working", 5)
                .putKinds("conversation", 30)
                .putKinds("summary", 2)
        )
        .setCache(
            AgentCacheUsage.newBuilder()
                .setNamespaces(2).setEntries(15).setBytes(16L shl 20)
                .setHits(120).setMisses(8).setEvictions(1)
        )
        .setCheckpoint(AgentCheckpointUsage.newBuilder().setCount(4).setBytes(12L shl 20))
        .setRecovery(
            AgentRecoveryState.newBuilder()
                .setReported(true).setAttempted(true).setSucceeded(true).setFrom("checkpoint")
        )
        .build()

    @Test
    fun `四块全齐时按记忆_缓存_检查点_最近恢复四行带出数值`() {
        val rows = agentRuntimeUsageRows(fullResp())
        assertEquals(listOf("记忆用量", "缓存用量", "检查点", "最近恢复"), rows.map { it.label })

        val mem = rows[0].value
        assertTrue(mem.contains("3 会话"))
        assertTrue(mem.contains("42 条"))
        assertTrue(mem.contains("1.5 MB"))
        // 分层计数照字段名原文透出(kinds 值域由 daemon 定义,不翻译);顺序钉死,照
        // proto 注释的 working/conversation/summary/artifact —— map 本身无序,不钉顺序
        // 同一份数据两次刷新会换位。
        assertTrue(mem.contains("working 5"))
        assertTrue(mem.indexOf("working") < mem.indexOf("conversation"))
        assertTrue(mem.indexOf("conversation") < mem.indexOf("summary"))

        val cache = rows[1].value
        assertTrue(cache.contains("2 命名空间"))
        assertTrue(cache.contains("15 条"))
        assertTrue(cache.contains("命中 120 / 未命中 8 / 淘汰 1"))

        assertEquals("4 个 · 12 MB", rows[2].value)

        val rec = rows[3].value
        assertTrue(rec.contains("恢复成功"))
        assertTrue(rec.contains("来源 检查点"))
    }

    @Test
    fun `整条响应缺席时不出行_失败原因已由资源行说明`() {
        // resp=null = 整条 RPC 没成功,是"没问到"不是"daemon 没提供";再补四行占位
        // 会把一次网络失败放大成四个疑点。
        assertTrue(agentRuntimeUsageRows(null).isEmpty())
    }

    @Test
    fun `旧daemon缺块时给事实陈述而不是全零`() {
        val rows = agentRuntimeUsageRows(AgentRuntimeResponse.getDefaultInstance())
        assertEquals(listOf("记忆用量", "缓存用量", "检查点", "最近恢复"), rows.map { it.label })
        rows.forEach { r ->
            assertEquals("daemon 未提供", r.value)
            // AUDIT §3.3 红线:事实陈述里不出现"未知/错误",也不许冒出假数值。
            assertFalse(r.value.contains("未知"))
            assertFalse(r.value.contains("错误"))
            assertFalse(r.value.contains("0"))
        }
    }

    @Test
    fun `块在而值为零是真实观测_照实显示零`() {
        // daemon 明确回答了这块,0 就是事实 —— 和"没这一块"是两回事。
        val rows = agentRuntimeUsageRows(
            AgentRuntimeResponse.newBuilder()
                .setMemory(AgentMemoryUsage.getDefaultInstance())
                .setCache(AgentCacheUsage.getDefaultInstance())
                .setCheckpoint(AgentCheckpointUsage.getDefaultInstance())
                .build()
        )
        assertEquals("0 会话 · 0 条 · 0 B", rows[0].value)
        assertEquals("0 命名空间 · 0 条 · 0 B · 命中 0 / 未命中 0 / 淘汰 0", rows[1].value)
        assertEquals("0 个 · 0 B", rows[2].value)
    }

    @Test
    fun `recovery未上报时说未上报而不是恢复失败`() {
        // proto 明文:daemon 侧目前还没有产出方,reported 恒为 false。把"没这一项"
        // 画成"恢复失败"是无中生有的坏消息 —— 这条线不许越。
        val rows = agentRuntimeUsageRows(
            AgentRuntimeResponse.newBuilder()
                .setRecovery(AgentRecoveryState.getDefaultInstance())
                .build()
        )
        assertEquals("该 agent 未上报恢复状态", rows[3].value)
        assertFalse(rows[3].value.contains("失败"))
    }

    @Test
    fun `recovery缺席与未上报是两回事`() {
        // 缺席 = 旧 daemon 没这个字段;未上报 = daemon 回答了"没有产出方"。
        val absent = agentRuntimeUsageRows(AgentRuntimeResponse.getDefaultInstance())[3]
        assertEquals("daemon 未提供", absent.value)
        val unreported = agentRuntimeUsageRows(
            AgentRuntimeResponse.newBuilder().setRecovery(AgentRecoveryState.getDefaultInstance()).build()
        )[3]
        assertEquals("该 agent 未上报恢复状态", unreported.value)
    }

    @Test
    fun `recovery尝试未成功时如实带出来源_原因_时间`() {
        val rows = agentRuntimeUsageRows(
            AgentRuntimeResponse.newBuilder().setRecovery(
                AgentRecoveryState.newBuilder()
                    .setReported(true).setAttempted(true).setSucceeded(false)
                    .setFrom("corrupted").setReason("会话库被隔离").setAt("2026-09-24T10:00:00+08:00")
            ).build()
        )
        val v = rows[3].value
        assertTrue(v.contains("尝试过恢复,未成功"))
        assertTrue(v.contains("来源 损坏副本"))
        assertTrue(v.contains("会话库被隔离"))
        assertTrue(v.contains("2026-09-24T10:00:00+08:00"))

        // "none" 表示没有恢复来源可说,不占一段字。
        val none = agentRuntimeUsageRows(
            AgentRuntimeResponse.newBuilder().setRecovery(
                AgentRecoveryState.newBuilder().setReported(true).setAttempted(true).setFrom("none")
            ).build()
        )[3]
        assertFalse(none.value.contains("none"))

        // 值域外的来源照抄原文,不猜(与六个形态标签同一纪律)。
        val weird = agentRuntimeUsageRows(
            AgentRuntimeResponse.newBuilder().setRecovery(
                AgentRecoveryState.newBuilder().setReported(true).setAttempted(true).setFrom("weird-source")
            ).build()
        )[3]
        assertTrue(weird.value.contains("来源 weird-source"))
    }

    @Test
    fun `recovery只有来源没有结果时不说成功也不说失败`() {
        val v = agentRuntimeUsageRows(
            AgentRuntimeResponse.newBuilder().setRecovery(
                AgentRecoveryState.newBuilder().setReported(true).setFrom("checkpoint")
            ).build()
        )[3].value
        assertTrue(v.contains("有恢复记录,未说明结果"))
        assertFalse(v.contains("成功"))
        assertFalse(v.contains("失败"))
    }

    @Test
    fun `记忆分层计数的字典序外键排在已知的后面且原文透出`() {
        val v = agentRuntimeUsageRows(
            AgentRuntimeResponse.newBuilder().setMemory(
                AgentMemoryUsage.newBuilder().setThreads(1)
                    .putKinds("zebra", 1).putKinds("working", 5)
            ).build()
        )[0].value
        assertTrue(v.contains("zebra 1"))
        assertTrue(v.indexOf("working 5") < v.indexOf("zebra 1"))
    }
}
