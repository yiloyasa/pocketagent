package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.ResyncRequired
import pocketagent.v1.TextDelta
import pocketagent.v1.ToolCallBlock
import pocketagent.v1.ToolCallUpdate
import pocketagent.v1.ToolResultBlock
import pocketagent.v1.TurnCompleted
import pocketagent.v1.TurnServerMessage
import pocketagent.v1.TurnStarted

/**
 * P1-5 事件断点续传(增量恢复)单测:
 *  1. 能力协商判定(capabilities 里有没有 event_sequence);
 *  2. 恢复选路:无断点 / 不支持 → 全量,有断点且支持 → 增量;
 *  3. 增量回放把错过的帧按原序应用到时间线(与实时流同一状态机);
 *  4. lastSequence 随帧推进,sequence=0 的帧不动断点;
 *  5. ResyncRequired → 断点作废 + 返回 RESYNC(调用方走全量回退);
 *  6. 回放流没有终局帧 → CAUGHT_UP(电脑端仍在跑,下周期继续增量)。
 *
 * 与 TurnDeliveryAndOfflineTest 同一约束:真实 gRPC 恢复链路依赖
 * AgentConnection,JVM 单测覆盖其调用的纯决策/应用层。
 */
class TurnEventResumeTest {

    // ------------------------------------------------------------ 能力协商

    @Test
    fun `capabilities 声明 event_sequence 才算支持续传`() {
        assertTrue(
            TurnSession.daemonSupportsEventSequence(listOf("terminal", "fs", "git", "agent", "event_sequence")),
        )
        // 旧 daemon 没有该声明
        assertFalse(TurnSession.daemonSupportsEventSequence(listOf("terminal", "fs", "git", "agent")))
        assertFalse(TurnSession.daemonSupportsEventSequence(emptyList()))
    }

    @Test
    fun `恢复选路 无断点或不支持时走全量`() {
        // 没有可信断点:即便 daemon 支持也不能增量(没有 from 哪来的 to)
        assertEquals(
            TurnSession.ResumePlan.FULL_ONLY,
            TurnSession.resumePlan(0L, listOf("agent", "event_sequence")),
        )
        assertEquals(
            TurnSession.ResumePlan.FULL_ONLY,
            TurnSession.resumePlan(-1L, listOf("agent", "event_sequence")),
        )
        // 拿不到能力声明(连不上旧 daemon)→ 按不支持对待
        assertEquals(TurnSession.ResumePlan.FULL_ONLY, TurnSession.resumePlan(42L, null))
        // daemon 不支持 → 全量
        assertEquals(
            TurnSession.ResumePlan.FULL_ONLY,
            TurnSession.resumePlan(42L, listOf("terminal", "agent")),
        )
        // 有断点且支持 → 增量
        assertEquals(
            TurnSession.ResumePlan.INCREMENTAL,
            TurnSession.resumePlan(42L, listOf("agent", "event_sequence")),
        )
    }

    // ------------------------------------------------------------ 增量回放

    @Test
    fun `增量回放把错过帧按原序应用到时间线`() {
        val st = TurnSession.State()
        val frames = listOf(
            frame(TurnServerMessage.newBuilder().setStarted(TurnStarted.newBuilder().setTurnId("t1")), 5),
            // 断线期间错过的两段正文增量:必须按 6 → 7 的顺序拼接
            frame(TurnServerMessage.newBuilder().setTextDelta(TextDelta.newBuilder().setDelta("你")), 6),
            frame(TurnServerMessage.newBuilder().setTextDelta(TextDelta.newBuilder().setDelta("好")), 7),
            frame(
                TurnServerMessage.newBuilder().setCompleted(
                    TurnCompleted.newBuilder().setTurnId("t1"),
                ),
                8,
            ),
        )
        val outcome = TurnSession.consumeReplayFrames("th", st, frames)
        assertEquals(TurnSession.ReplayOutcome.TERMINAL, outcome)
        // 正文按原序拼出,终局落成一条助手消息(无重复、无缺字)
        val assistants = st.items.filter { it.role == "assistant" }
        assertEquals(1, assistants.size)
        assertEquals("你好", assistants.single().text)
        // 断点推进到最后一帧
        assertEquals(8L, st.lastSequence)
        // 回放不收尾实时流的生命周期锚点
        assertNull(st.replyCh)
        assertFalse(st.running)
    }

    @Test
    fun `工具卡在回放中同样一次调用一条`() {
        val st = TurnSession.State()
        val frames = listOf(
            frame(
                TurnServerMessage.newBuilder().setToolCall(
                    ToolCallUpdate.newBuilder()
                        .setPhase(ToolCallUpdate.Phase.STARTED)
                        .setCall(ToolCallBlock.newBuilder().setId("call-1").setName("shell").setJsonArgs("{}")),
                ),
                2,
            ),
            frame(
                TurnServerMessage.newBuilder().setToolCall(
                    ToolCallUpdate.newBuilder()
                        .setPhase(ToolCallUpdate.Phase.FINISHED)
                        .setCall(ToolCallBlock.newBuilder().setId("call-1").setName("shell"))
                        .setResult(ToolResultBlock.newBuilder().setOk(true).setOutput("done")),
                ),
                3,
            ),
        )
        val outcome = TurnSession.consumeReplayFrames("th", st, frames)
        assertEquals(TurnSession.ReplayOutcome.CAUGHT_UP, outcome) // 没有终局帧
        val tools = st.items.filter { it.role == "tool" }
        // STARTED 插卡、FINISHED 原地更新:不出现两张卡
        assertEquals(1, tools.size)
        assertEquals("success", tools.single().toolStatus)
        assertEquals("done", tools.single().text)
        assertEquals(3L, st.lastSequence)
    }

    @Test
    fun `lastSequence 随帧推进且无序号帧不作废断点`() {
        val st = TurnSession.State()
        TurnSession.applyServerFrame(
            "th",
            st,
            frame(TurnServerMessage.newBuilder().setStarted(TurnStarted.newBuilder().setTurnId("t1")), 10),
            replay = true,
        )
        assertEquals(10L, st.lastSequence)
        // sequence=0(旧 daemon 帧)不推进也不清零
        TurnSession.applyServerFrame(
            "th",
            st,
            frame(TurnServerMessage.newBuilder().setTextDelta(TextDelta.newBuilder().setDelta("x")), 0),
            replay = true,
        )
        assertEquals(10L, st.lastSequence)
        // 乱序旧帧(比断点还小)不回退断点
        TurnSession.applyServerFrame(
            "th",
            st,
            frame(TurnServerMessage.newBuilder().setTextDelta(TextDelta.newBuilder().setDelta("y")), 4),
            replay = true,
        )
        assertEquals(10L, st.lastSequence)
    }

    @Test
    fun `ResyncRequired 作废断点并要求全量回退`() {
        val st = TurnSession.State()
        st.lastSequence = 42L
        val outcome = TurnSession.consumeReplayFrames(
            "th",
            st,
            listOf(
                frame(
                    TurnServerMessage.newBuilder().setResyncRequired(
                        ResyncRequired.newBuilder().setReason("区间已被轮转丢弃"),
                    ),
                    0,
                ),
            ),
        )
        assertEquals(TurnSession.ReplayOutcome.RESYNC, outcome)
        // 断点必须作废:调用方收到 RESYNC 后走 GetThreadHistory 全量回填
        assertEquals(0L, st.lastSequence)
        assertTrue(st.items.isEmpty()) // 没有应用任何内容帧
    }

    @Test
    fun `回放未到终局时保持流式状态等下周期`() {
        val st = TurnSession.State()
        val outcome = TurnSession.consumeReplayFrames(
            "th",
            st,
            listOf(
                frame(TurnServerMessage.newBuilder().setStarted(TurnStarted.newBuilder().setTurnId("t1")), 5),
                frame(
                    TurnServerMessage.newBuilder().setTextDelta(TextDelta.newBuilder().setDelta("正在")),
                    6,
                ),
            ),
        )
        assertEquals(TurnSession.ReplayOutcome.CAUGHT_UP, outcome)
        assertTrue(st.streaming) // 电脑端还在跑,流式状态保持
        // 【P1-5 锚点更新】delta 改走 StringBuilder 缓冲 + 120ms 巡检发布;断言前
        // flush 追平(回放累积语义不变,"帧后立即可读"属实现细节)
        TurnSession.flushStreamBuffers(st)
        assertEquals("正在", st.streamText)
        assertEquals(6L, st.lastSequence)
    }

    // ------------------------------------------------------------ 基建

    /** 组一帧:消息体 + sequence(P1-5 起真实帧必带)。 */
    private fun frame(msg: TurnServerMessage.Builder, seq: Long): TurnServerMessage =
        msg.setSequence(seq).build()
}
