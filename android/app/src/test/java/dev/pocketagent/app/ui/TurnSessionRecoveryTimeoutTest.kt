package dev.pocketagent.app.ui

import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 【恢复循环总超时·真相层根治】recoverOrphanTurn 恢复链的缩时单测。
 *
 * 【钉住的用户症状】停止/异常轮次后 recovering=true 挡发送(canSend 与
 * TurnSession.send 都读原始 recovering),而恢复循环旧实现没有总超时:300 次
 * 退避 ≈ 15 分钟才判超时,daemon 核对信号丢失时长时间挂住 —— F8 只在呈现层
 * 收了视觉,发送闸仍被无限期按住。本轮裁决:真相层总预算与呈现层看门狗
 * 同值同源(同一常量 10s),两层同刻收尾,消灭"看着 idle 却发不出"的割裂窗。
 *
 * 【被测对象】生产 attempt 体(增量续传/全量回填)依赖 AgentConnection 真连接,
 * JVM 单测无法驱动;总超时的裁定收在可注入 attemptOnce 的 [TurnSession.runRecoveryLoop]
 * 骨架里,收尾语义在 [TurnSession.finishRecoveryTimedOut] —— 本组用例钉死:
 * 超时判定 / 预算内接管不受影响 / 换代与删除静默退出 / 次数防呆 / 收尾复位与
 * 文案落地 / 两层同源契约。真实时间预算(150ms~1s),全部亚秒级完成。
 */
class TurnSessionRecoveryTimeoutTest {

    private fun recoveringState(): TurnSession.State = TurnSession.State().also {
        it.recovering = true
        it.errorMessage = "结果流已结束，正在核对电脑端任务状态"
    }

    // ------------------------------------------------------------ 总超时判定

    @Test
    fun `恢复总预算到期_如实判超时_不再轮询满15分钟`() = runBlocking {
        val st = recoveringState()
        val start = System.currentTimeMillis()
        val exit = TurnSession.runRecoveryLoop(st, generation = st.generation, totalTimeoutMs = 200) {
            false // daemon 一直没给确认:模拟核对信号丢失
        }
        val elapsed = System.currentTimeMillis() - start
        assertEquals(
            "总预算到期必须判 TIMED_OUT(旧实现要等 300 次退避 ≈ 15 分钟)",
            TurnSession.RecoveryExit.TIMED_OUT, exit,
        )
        assertTrue("缩时预算(200ms)应在秒级退出,实际 ${elapsed}ms", elapsed < 10_000)
    }

    @Test
    fun `预算内补到确认帧_正常接管不受超时影响`() = runBlocking {
        val st = recoveringState()
        var calls = 0
        val exit = TurnSession.runRecoveryLoop(st, generation = st.generation, totalTimeoutMs = 5_000) {
            calls += 1
            calls >= 2 // 第二个周期补到终局/确认帧(增量 TERMINAL 或全量确认)
        }
        assertEquals(
            "预算内恢复成功就是成功,绝不能被总超时误杀",
            TurnSession.RecoveryExit.RECOVERED, exit,
        )
        assertEquals("成功发生在第二次尝试,循环应立即停", 2, calls)
    }

    // ------------------------------------------------------------ 静默退出(不收尾)

    @Test
    fun `换代后静默弃恢复_不判超时`() = runBlocking {
        val st = recoveringState()
        var calls = 0
        val exit = TurnSession.runRecoveryLoop(st, generation = st.generation, totalTimeoutMs = 60_000) {
            // 模拟 attempt 期间用户重开轮次(startTurn 里 generation++)
            st.generation += 1
            calls += 1
            false
        }
        assertEquals(
            "换代 = 用户已重开轮次,恢复循环必须让位且不得落超时文案",
            TurnSession.RecoveryExit.ABANDONED, exit,
        )
        assertEquals(1, calls)
    }

    @Test
    fun `恢复信号通道关闭_立即静默退出_release语义不变`() = runBlocking {
        val st = recoveringState()
        st.recoverySignal.close() // release()/回收会话的立即退出指令
        val exit = TurnSession.runRecoveryLoop(st, generation = st.generation, totalTimeoutMs = 60_000) { false }
        assertEquals(TurnSession.RecoveryExit.ABANDONED, exit)
    }

    @Test
    fun `次数耗尽_防呆内界仍在`() = runBlocking {
        val st = recoveringState()
        val exit = TurnSession.runRecoveryLoop(
            st, generation = st.generation,
            attempts = 3, totalTimeoutMs = 600_000, // 放大预算,让次数界先到
        ) { false }
        assertEquals(
            "总预算被放大时,300 次内界仍兜底(与超时同一份收尾)",
            TurnSession.RecoveryExit.EXHAUSTED, exit,
        )
    }

    // ------------------------------------------------------------ 超时收尾(真相层数据)

    @Test
    fun `超时收尾_recovering复位_失败文案落地_发送闸解封`() {
        val st = recoveringState()
        assertTrue("前置:recovering 挂着时发送闸必须关闭", st.recovering)

        TurnSession.finishRecoveryTimedOut("t-recovery-timeout", st)

        assertFalse(
            "发送闸解封:TurnSession.send 与 UI canSend 读的就是 st.recovering 这一位",
            st.recovering,
        )
        assertEquals(
            "失败文案如实落地,不伪造成功",
            TurnSession.RECOVERY_TIMEOUT_NOTICE, st.errorMessage,
        )
        // 文案不是核对族:呈现层按真失败照常展示,绝不被 10s 看门狗的视觉收尾吞掉
        assertFalse(
            "超时文案属于终局失败,不得命中 isVerifyingRecoveryCopy(否则横幅被吞)",
            isVerifyingRecoveryCopy(st.errorMessage),
        )
    }

    // ------------------------------------------------------------ 两层同源契约

    @Test
    fun `真相层总预算与呈现层核对看门狗同值同源`() {
        assertEquals(
            "恢复总预算必须引用呈现层看门狗同一常量:两层同刻收尾," +
                "不存在「10s 收视觉、之后仍不可发送」的割裂窗",
            VERIFYING_WATCHDOG_TIMEOUT_MS, TurnSession.recoveryTotalTimeoutMs,
        )
        assertTrue("两层同窗口的常识下限:核对等待不得超过 30s", TurnSession.recoveryTotalTimeoutMs <= 30_000)
    }
}
