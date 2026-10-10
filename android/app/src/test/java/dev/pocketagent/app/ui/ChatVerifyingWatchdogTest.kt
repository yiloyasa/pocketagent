package dev.pocketagent.app.ui

import dev.pocketagent.app.ui.chat.CapsulePulseTier
import dev.pocketagent.app.ui.chat.capsulePulseTier
import dev.pocketagent.app.ui.chat.recoveryNoticeCopy
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.ThreadStatus

/**
 * e2e-paired-20261009 问题2:核对等待超时兜底的判定矩阵(ChatScreen 纯函数 +
 * ChatTimeline.recoveryNoticeCopy)。
 *
 * 背景:停止轮次后流取消走 TurnSession 兜底分支,`recovering=true` + 「正在核对
 * 电脑端任务状态」文案,恢复轮询最长 ~15 分钟;daemon 核对信号丢失/延迟时头部
 * 状态点长期"正在执行"、琥珀横幅长期驻留。呈现层 10s 看门狗超时后按本地真相
 * 收尾:状态点回 idle、横幅消退 —— 只撤"运行中/核对中"的声称,不伪造成功。
 *
 * 【F8 遗留反馈 3】胶囊运行点与收尾位的联动(capsulePulseTier)同文件钉死:
 * settled 后胶囊不再呼吸,退静态 Muted 档,与头部状态点回 idle 同拍。
 */
class ChatVerifyingWatchdogTest {

    // ---- 常量:超时上限 10s(可缩时测试的基准) ----

    @Test
    fun watchdog_timeout_is_10_seconds() {
        assertEquals(10_000L, VERIFYING_WATCHDOG_TIMEOUT_MS)
    }

    // ---- 文案族:只有"核对等待"中间态参与收尾,真失败绝不吞 ----

    @Test
    fun turn_session_verifying_copies_are_verifying_family() {
        // 对齐 TurnSession 全部五种在产文案(逐字)
        assertTrue(isVerifyingRecoveryCopy("结果流已结束，正在核对电脑端任务状态"))
        assertTrue(isVerifyingRecoveryCopy("结果流中断，正在核对电脑端任务状态"))
        assertTrue(isVerifyingRecoveryCopy("连接暂不可用，正在等待恢复结果"))
        assertTrue(isVerifyingRecoveryCopy("正在重新获取电脑端任务结果"))
    }

    @Test
    fun terminal_failures_are_not_verifying_family() {
        // FAILED 终局真错误:E1 实锤的 "context canceled" 与常规失败/方法名报错
        assertFalse(isVerifyingRecoveryCopy("context canceled"))
        assertFalse(isVerifyingRecoveryCopy("任务失败"))
        assertFalse(isVerifyingRecoveryCopy("session/resume\n<id> 失败:上游超时"))
        assertFalse(isVerifyingRecoveryCopy("自动恢复超时，请手动刷新会话历史"))
        // 空值防御
        assertFalse(isVerifyingRecoveryCopy(null))
        assertFalse(isVerifyingRecoveryCopy(""))
    }

    // ---- 看门狗武装条件:恢复中 + 核对文案族 ----

    @Test
    fun watchdog_armed_only_for_recovering_with_verifying_copy() {
        assertTrue(verificationWatchdogArmed(recovering = true, errorMessage = "结果流已结束，正在核对电脑端任务状态"))
        assertFalse(verificationWatchdogArmed(recovering = false, errorMessage = "结果流已结束，正在核对电脑端任务状态"))
        assertFalse(verificationWatchdogArmed(recovering = true, errorMessage = "context canceled"))
        assertFalse(verificationWatchdogArmed(recovering = true, errorMessage = null))
    }

    // ---- 头部状态点:超时收尾 → 不再声称运行中,回 idle ----

    @Test
    fun status_dot_shows_running_while_verifying_wait_armed() {
        // 收尾前:recovering → RUNNING「正在执行」(现状口径,不回归)
        val armed = editorialChatStatusView(
            running = false,
            recovering = true,
            verifyingSettled = false,
            turnTerminal = false,
            errorMessage = "结果流中断，正在核对电脑端任务状态",
            pendingApprovals = 0,
            daemonStatus = ThreadStatus.THREAD_STATUS_IDLE,
        )
        assertEquals(EditorialStatus.RUNNING, armed.status)
    }

    @Test
    fun status_dot_returns_to_idle_after_watchdog_settles() {
        // 收尾后:recovering 的 RUNNING 映射被短路,状态点回 idle(不伪造细节,
        // 核对文案不再喂给错误详情)
        val settled = editorialChatStatusView(
            running = false,
            recovering = true,
            verifyingSettled = true,
            turnTerminal = false,
            errorMessage = "结果流中断，正在核对电脑端任务状态",
            pendingApprovals = 0,
            daemonStatus = ThreadStatus.THREAD_STATUS_UNSPECIFIED,
        )
        assertEquals(EditorialStatus.COMPLETED, settled.status)
        assertNull(settled.detail)
    }

    @Test
    fun pending_approval_still_wins_over_settled_idle() {
        val settledWithPending = editorialChatStatusView(
            running = false,
            recovering = true,
            verifyingSettled = true,
            turnTerminal = false,
            errorMessage = "结果流中断，正在核对电脑端任务状态",
            pendingApprovals = 1,
            daemonStatus = ThreadStatus.THREAD_STATUS_IDLE,
        )
        assertEquals(EditorialStatus.WAITING_FOR_YOU, settledWithPending.status)
    }

    @Test
    fun non_verifying_paths_keep_existing_status_matrix() {
        // 实时运行位不变
        assertEquals(
            EditorialStatus.RUNNING,
            editorialChatStatusView(
                running = true, recovering = false, verifyingSettled = false,
                turnTerminal = false, errorMessage = null, pendingApprovals = 0,
                daemonStatus = ThreadStatus.THREAD_STATUS_IDLE,
            ).status,
        )
        // 干净收尾(IDLE)不变
        assertEquals(
            EditorialStatus.COMPLETED,
            editorialChatStatusView(
                running = false, recovering = false, verifyingSettled = false,
                turnTerminal = true, errorMessage = null, pendingApprovals = 0,
                daemonStatus = ThreadStatus.THREAD_STATUS_UNSPECIFIED,
            ).status,
        )
        // 真失败照旧:FAILED 文案 → 未完成 + 错误详情,绝不被看门狗吞
        val failed = editorialChatStatusView(
            running = false, recovering = false, verifyingSettled = false,
            turnTerminal = true, errorMessage = "context canceled", pendingApprovals = 0,
            daemonStatus = ThreadStatus.THREAD_STATUS_IDLE,
        )
        assertEquals(EditorialStatus.NOT_COMPLETED, failed.status)
    }

    // ---- 琥珀横幅:超时收尾 → 「正在核对…」消退;恢复失败/真错误照常展示 ----

    @Test
    fun notice_shows_verifying_copy_while_waiting() {
        assertEquals(
            "结果流已结束，正在核对电脑端任务状态",
            recoveryNoticeCopy(
                recoveryError = null,
                recovering = true,
                verifyingSettled = false,
                errorMessage = "结果流已结束，正在核对电脑端任务状态",
                loadError = null,
            ),
        )
        // 恢复中无文案的兜底人话原样保留
        assertEquals(
            "连接已断开，正在恢复电脑端任务结果",
            recoveryNoticeCopy(
                recoveryError = null, recovering = true, verifyingSettled = false,
                errorMessage = null, loadError = null,
            ),
        )
    }

    @Test
    fun notice_fades_after_watchdog_settles() {
        assertNull(
            recoveryNoticeCopy(
                recoveryError = null,
                recovering = true,
                verifyingSettled = true,
                errorMessage = "结果流中断，正在核对电脑端任务状态",
                loadError = null,
            ),
        )
    }

    @Test
    fun notice_keeps_recovery_error_and_real_failures() {
        // 恢复失败(终态)优先级最高,收尾位压不住它
        assertEquals(
            "自动恢复超时，请手动刷新会话历史",
            recoveryNoticeCopy(
                recoveryError = "自动恢复超时，请手动刷新会话历史",
                recovering = false,
                verifyingSettled = false,
                errorMessage = null,
                loadError = "加载失败",
            ),
        )
        // 真失败(非核对文案族)照常展示 —— 防御性:即使 settled 误挂也不吞
        assertEquals(
            "context canceled",
            recoveryNoticeCopy(
                recoveryError = null, recovering = true, verifyingSettled = true,
                errorMessage = "context canceled", loadError = null,
            ),
        )
        // 页面加载错误兜底不变
        assertEquals(
            "加载失败",
            recoveryNoticeCopy(
                recoveryError = null, recovering = false, verifyingSettled = false,
                errorMessage = null, loadError = "加载失败",
            ),
        )
    }

    // ---- F8 遗留反馈 3:胶囊运行点与看门狗收尾联动(capsulePulseTier 判定矩阵) ----

    @Test
    fun capsule_pulse_keeps_running_tier_while_claim_is_alive() {
        // 真在跑 → 呼吸档(现状口径,不回归)
        assertEquals(
            CapsulePulseTier.PULSING,
            capsulePulseTier(running = true, recovering = false, verifyingSettled = false),
        )
        // 恢复链挂着且看门狗未收尾 → 仍按"运行中"呼吸(现状口径,不回归)
        assertEquals(
            CapsulePulseTier.PULSING,
            capsulePulseTier(running = false, recovering = true, verifyingSettled = false),
        )
    }

    @Test
    fun capsule_pulse_stops_breathing_after_watchdog_settles() {
        // 收尾后恢复链仍在后台 → 静态 Muted 余辉档,不再呼吸
        // (与头部状态点 status_dot_returns_to_idle_after_watchdog_settles 同拍)
        assertEquals(
            CapsulePulseTier.SETTLED,
            capsulePulseTier(running = false, recovering = true, verifyingSettled = true),
        )
    }

    @Test
    fun capsule_pulse_hidden_when_idle_and_running_beats_stale_settle() {
        // 既没跑也没挂着 → 不摆(宁缺勿滥)
        assertEquals(
            CapsulePulseTier.NONE,
            capsulePulseTier(running = false, recovering = false, verifyingSettled = false),
        )
        // 防御:真在跑压过过期的收尾位 —— 运行中必须呼吸,绝不误静音
        assertEquals(
            CapsulePulseTier.PULSING,
            capsulePulseTier(running = true, recovering = true, verifyingSettled = true),
        )
        // 无恢复链时 settled 不构成余辉 —— 即使收尾位误挂也不摆幽灵点
        assertEquals(
            CapsulePulseTier.NONE,
            capsulePulseTier(running = false, recovering = false, verifyingSettled = true),
        )
    }
}
