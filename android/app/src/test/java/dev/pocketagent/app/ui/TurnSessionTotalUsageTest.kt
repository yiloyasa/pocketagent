package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test
import pocketagent.v1.TurnServerMessage
import pocketagent.v1.Usage
import pocketagent.v1.UsageUpdate

/**
 * N3·hermes「本会话累计恒显 0」修复的单测:
 *  1. 投影纯函数:total_usage 未设置(protobuf 默认实例)→ null;设置了 → 原值;
 *  2. applyServerFrame 的 USAGE 帧接线:未设置不把默认实例写进 st.totalUsage,
 *     设置了照常覆盖;窗口字段与 turn 用量的既有行为不受影响。
 *
 * hermes 适配器只有会话级 size/used、无总量来源,usage_update 投影从不设置
 * total_usage —— daemon 侧补不了,客户端必须把"未设置"还原为 null,
 * "本会话累计"Tab 才能以"—"呈现未知而不是渲染 0/0/0。
 */
class TurnSessionTotalUsageTest {

    @Test
    fun `total_usage 未设置时投影为 null 而不是默认实例`() {
        val update = UsageUpdate.newBuilder()
            .setUsage(Usage.newBuilder().setInputTokens(120).setOutputTokens(30))
            .build()
        // 前置事实:proto getter 对未设置字段返回默认实例(这正是 bug 的形状)
        assertEquals(0, update.totalUsage.inputTokens)
        // 修复:未设置 → null
        assertNull(projectedTotalUsage(update))
    }

    @Test
    fun `total_usage 设置时投影为原值`() {
        val total = Usage.newBuilder().setInputTokens(9_000).setOutputTokens(1_000).build()
        val update = UsageUpdate.newBuilder()
            .setUsage(Usage.newBuilder().setInputTokens(120))
            .setTotalUsage(total)
            .build()
        assertEquals(total, projectedTotalUsage(update))
    }

    @Test
    fun `USAGE 帧未设置 total_usage 时不写入会话累计`() {
        val st = TurnSession.State()
        val frame = TurnServerMessage.newBuilder()
            .setUsage(UsageUpdate.newBuilder()
                .setUsage(Usage.newBuilder().setInputTokens(120).setOutputTokens(30))
                .setModelContextWindow(128_000L))
            .build()
        TurnSession.applyServerFrame("t-total", st, frame, replay = false)
        assertNull(st.totalUsage)
        // 同帧其余字段照常生效
        assertEquals(128_000L, st.liveContextWindow)
        assertEquals(120L, st.turnUsage?.inputTokens)
    }

    @Test
    fun `USAGE 帧设置了 total_usage 时照常覆盖会话累计`() {
        val st = TurnSession.State()
        // 先有一帧未设置的(hermes 形态)→ null
        val hermesFrame = TurnServerMessage.newBuilder()
            .setUsage(UsageUpdate.newBuilder()
                .setUsage(Usage.newBuilder().setInputTokens(1)))
            .build()
        TurnSession.applyServerFrame("t-total2", st, hermesFrame, replay = false)
        assertNull(st.totalUsage)
        // 再来一帧设置了 total 的(codex 形态)→ 照常覆盖
        val total = Usage.newBuilder().setInputTokens(9_000).setOutputTokens(1_000).build()
        val codexFrame = TurnServerMessage.newBuilder()
            .setUsage(UsageUpdate.newBuilder()
                .setUsage(Usage.newBuilder().setInputTokens(2))
                .setTotalUsage(total))
            .build()
        TurnSession.applyServerFrame("t-total2", st, codexFrame, replay = false)
        assertEquals(total, st.totalUsage)
    }
}
