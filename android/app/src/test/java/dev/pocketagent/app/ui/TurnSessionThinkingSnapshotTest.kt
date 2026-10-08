package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Test
import pocketagent.v1.ThinkingDelta
import pocketagent.v1.TurnServerMessage

/**
 * 【P1-3·pi thinking】THINKING_DELTA 帧 snapshot 语义的单测。
 *
 * daemon 侧契约:一帧只带其一 ——
 *  - delta    流式增量(客户端追加拼接,旧行为);
 *  - snapshot 权威全文(pi 的 thinking_end 终局事件/历史回放一次性携带),
 *    客户端收到即**整体替换**累积思考文本,是丢帧/断连后的兜底真相。
 *
 * 实时流与回放(GetThreadEventsSince)共用 applyServerFrame 同一分支,这里
 * 两条路径都钉:snapshot 非空 → 替换;snapshot 空(旧 daemon/普通 delta 帧)
 * → 追加,与旧版行为一字不差。
 */
class TurnSessionThinkingSnapshotTest {

    /** 组装一帧 THINKING_DELTA(delta/snapshot 均显式赋值,未设置 = 空串同义)。 */
    private fun thinkingFrame(delta: String = "", snapshot: String = ""): TurnServerMessage =
        TurnServerMessage.newBuilder()
            .setThinkingDelta(
                ThinkingDelta.newBuilder().setDelta(delta).setSnapshot(snapshot),
            )
            .build()

    // ------------------------------------------------------ snapshot 替换 / delta 追加

    @Test
    fun `snapshot 帧整体替换此前累积的思考文本`() {
        val st = TurnSession.State()
        TurnSession.applyServerFrame("t-snap", st, thinkingFrame(delta = "流式追加的 "), replay = false)
        TurnSession.applyServerFrame(
            "t-snap", st, thinkingFrame(snapshot = "权威全文(丢帧兜底)"), replay = false,
        )
        // snapshot 是权威真相:整体替换,不与此前累积拼接
        assertEquals("权威全文(丢帧兜底)", st.thinkingText)
    }

    @Test
    fun `delta 帧照常追加拼接`() {
        val st = TurnSession.State()
        TurnSession.applyServerFrame("t-delta", st, thinkingFrame(delta = "思考"), replay = false)
        TurnSession.applyServerFrame("t-delta", st, thinkingFrame(delta = "进行"), replay = false)
        TurnSession.applyServerFrame("t-delta", st, thinkingFrame(delta = "中"), replay = false)
        assertEquals("思考进行中", st.thinkingText)
    }

    @Test
    fun `snapshot 空串回退追加_旧行为零变化`() {
        val st = TurnSession.State()
        TurnSession.applyServerFrame("t-fallback", st, thinkingFrame(delta = "增量"), replay = false)
        // 显式空 snapshot(daemon 只想补一帧 delta)→ 追加,不得把已有累积清掉
        TurnSession.applyServerFrame("t-fallback", st, thinkingFrame(delta = "+续", snapshot = ""), replay = false)
        assertEquals("增量+续", st.thinkingText)
    }

    @Test
    fun `旧 daemon 完全不带 snapshot 字段的帧等价空串_回退追加`() {
        val st = TurnSession.State()
        TurnSession.applyServerFrame("t-legacy", st, thinkingFrame(delta = "a"), replay = false)
        // proto 未设置字段 getter 返回空串:与显式空串同一分支,旧端行为不变
        val legacy = TurnServerMessage.newBuilder()
            .setThinkingDelta(ThinkingDelta.newBuilder().setDelta("b"))
            .build()
        TurnSession.applyServerFrame("t-legacy", st, legacy, replay = false)
        assertEquals("ab", st.thinkingText)
    }

    // ------------------------------------------------------ 混合序列

    @Test
    fun `多轮 delta_snapshot_delta 序列的文本终值以最后一个权威帧为基`() {
        val st = TurnSession.State()
        // 1) 纯增量累积
        TurnSession.applyServerFrame("t-mix", st, thinkingFrame(delta = "AB"), replay = false)
        // 2) 终局 snapshot:整体替换(修正此前逐帧拼接可能缺的帧)
        TurnSession.applyServerFrame("t-mix", st, thinkingFrame(snapshot = "AB+修正"), replay = false)
        // 3) 新一轮思考继续在权威全文之后追加
        TurnSession.applyServerFrame("t-mix", st, thinkingFrame(delta = "C"), replay = false)
        assertEquals("AB+修正C", st.thinkingText)
    }

    @Test
    fun `snapshot 之后再来的 snapshot 仍然整体替换`() {
        val st = TurnSession.State()
        TurnSession.applyServerFrame("t-snap2", st, thinkingFrame(snapshot = "第一版全文"), replay = false)
        TurnSession.applyServerFrame("t-snap2", st, thinkingFrame(snapshot = "第二版全文"), replay = false)
        assertEquals("第二版全文", st.thinkingText)
    }

    // ------------------------------------------------------ 回放路径(历史恢复)

    @Test
    fun `历史回放路径走同一状态机_snapshot 帧同样整体替换`() {
        val st = TurnSession.State()
        TurnSession.applyServerFrame("t-replay", st, thinkingFrame(delta = "断线前的部分"), replay = false)
        // 断线后 GetThreadEventsSince 回放:历史帧带权威全文
        val outcome = TurnSession.consumeReplayFrames(
            "t-replay", st,
            listOf(thinkingFrame(snapshot = "回放权威全文")),
        )
        assertEquals("回放权威全文", st.thinkingText)
        // 无终局帧 → 只是追平,不算恢复完成
        assertEquals(TurnSession.ReplayOutcome.CAUGHT_UP, outcome)
    }

    @Test
    fun `历史回放路径的纯 delta 帧照常追加`() {
        val st = TurnSession.State()
        val outcome = TurnSession.consumeReplayFrames(
            "t-replay2", st,
            listOf(thinkingFrame(delta = "x"), thinkingFrame(delta = "y")),
        )
        assertEquals("xy", st.thinkingText)
        assertEquals(TurnSession.ReplayOutcome.CAUGHT_UP, outcome)
    }
}
