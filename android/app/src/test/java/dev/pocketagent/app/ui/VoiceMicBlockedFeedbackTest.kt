package dev.pocketagent.app.ui

import dev.pocketagent.app.ui.component.VOICE_ERROR_ENGINE
import dev.pocketagent.app.ui.component.VOICE_ERROR_ENGINE_BUSY
import dev.pocketagent.app.ui.component.VOICE_ERROR_MIC_BLOCKED
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 「系统设置已允许麦克风,但 App 内语音根本用不了」修复的回归单测:
 *  1. 端侧自定义错误码 102(麦克风被拦)/103(引擎忙)此前漏映射落 OTHER,
 *     MIUI 用户只看到"识别失败,请重试",永远得不到"去安全中心开「录音」"的指引 ——
 *     现在 102 → MIC_BLOCKED(可行动指引)、103 → BUSY;
 *  2. 系统识别"名义可用实际不可用"的弃用判定:错误码名单(含 3=AUDIO_ERROR)与
 *     死区超时,此前失败后引擎选择永远粘滞在系统 SR,端侧 SenseVoice 永远轮不上;
 *  3. 系统码原有映射不受新分支影响(1..13 与 100+ 互不串扰)。
 *
 * 【P4 Task 4/5 迁移说明】语音状态机/指令链用例已迁 ui/voice(VoiceSessionReducerTest),
 * 本文件保留旧七类错误映射(= SystemVoiceEngine 单一真值源)与弃用名单的行为钉子。
 */
class VoiceMicBlockedFeedbackTest {

    // ------------------------------------------------------------ 102/103 补映射

    @Test
    fun `端侧102麦克风被拦映射MIC_BLOCKED_103引擎忙映射BUSY`() {
        assertEquals(VoiceErrorKind.MIC_BLOCKED, voiceErrorKind(VOICE_ERROR_MIC_BLOCKED))
        assertEquals(VoiceErrorKind.BUSY, voiceErrorKind(VOICE_ERROR_ENGINE_BUSY))
        // 端侧既有 100 映射不被新分支影响(101 已随旧遗留引擎摘除,不再有映射)
        assertEquals(VoiceErrorKind.ENGINE, voiceErrorKind(VOICE_ERROR_ENGINE))
    }

    @Test
    fun `MIC_BLOCKED人话提示必须可行动_点名系统设置与安全中心`() {
        val text = voiceErrorText(VoiceErrorKind.MIC_BLOCKED)
        // 指路两道开关:运行时权限(系统设置)与 MIUI 独立「录音」开关(安全中心)
        assertTrue(text.contains("系统设置"))
        assertTrue(text.contains("安全中心"))
        assertTrue(text.contains("录音"))
        assertTrue(text.contains("麦克风"))
        // 占用冲突也给出指引
        assertTrue(text.contains("占用"))
    }

    // 【P4 迁移说明】`MIC_BLOCKED事件经状态机产出该提示_回空闲` 已随旧页面状态机
    // (voiceReduce/applyVoiceEvent)删除:新链路的提示产出由 ui.voice.Error 事件 →
    // 新枚举 voiceErrorText 重载承担(见 VoiceSessionReducerTest 接线文案用例)。

    // ------------------------------------------------------------ 系统引擎弃用判定(错误码名单)

    @Test
    fun `系统识别错误码名单_含3录音错误与9服务权限_命中即弃用系统引擎改选端侧`() {
        // 1 网络超时 / 2 网络 / 3 录音错误(MIUI「录音」开关关的典型表现,旧名单漏了)
        // / 4 服务端 / 5 客户端 / 8 忙 / 9 识别服务自身无权限(HyperOS fuxi 真机实证:
        //   App 权限已授仍每点击瞬间报 9,永不可自愈,旧名单漏了) / 11 断连 / 12-13 语言不支持
        for (code in listOf(1, 2, 3, 4, 5, 8, 9, 11, 12, 13)) {
            assertTrue("code=$code 应触发弃用", systemVoiceRetireNeeded(code))
        }
    }

    @Test
    fun `系统识别弃用名单_正常结果与端侧码一律不弃用`() {
        // 6 说话超时 / 7 没听到:正常业务结果,弃用会把"没听到"误判成"服务坏了"
        for (code in listOf(6, 7)) assertFalse(systemVoiceRetireNeeded(code))
        // 9 已移入弃用名单:App 权限被拒的场景由端侧 102 MIC_BLOCKED 接管并给出
        //   "安全中心→录音"精确指引,两条路都诚实可行动
        // 0/负数/未知码:不弃用
        for (code in listOf(0, -1, 10, 99)) assertFalse(systemVoiceRetireNeeded(code))
        // 100+ 端侧自定义码:到达时系统引擎本就未被选中,绝不能误伤
        for (code in listOf(VOICE_ERROR_ENGINE, VOICE_ERROR_MIC_BLOCKED, VOICE_ERROR_ENGINE_BUSY)) {
            assertFalse(systemVoiceRetireNeeded(code))
        }
    }

    // ------------------------------------------------------------ 死区超时的弃用判定

    // 【P4 迁移说明】`死区超时_仅当当前引擎是系统时才弃用_端侧不因死区自动弃用`
    // (voiceDeadZoneShouldRetireSystem)已随旧墙钟看门狗删除:monotonic 化后的
    // 等价策略在 ChatScreen 接线(死区/roundFailed → systemVoiceRetired → 重绑端侧),
    // 判定与分流由 VoiceSessionReducerTest 的 Controller 用例锁定。

    // ------------------------------------------------------------ 系统码映射回归(不被新分支吞掉)

    @Test
    fun `系统码原有映射不受影响_3仍归OTHER_7_8_9_5不变`() {
        assertEquals(VoiceErrorKind.OTHER, voiceErrorKind(3)) // ERROR_AUDIO_ERROR
        assertEquals(VoiceErrorKind.NO_MATCH, voiceErrorKind(7))
        assertEquals(VoiceErrorKind.BUSY, voiceErrorKind(8))
        assertEquals(VoiceErrorKind.INSUFFICIENT_PERMISSIONS, voiceErrorKind(9))
        assertEquals(VoiceErrorKind.CLIENT, voiceErrorKind(5))
        assertEquals(VoiceErrorKind.OTHER, voiceErrorKind(1))
        // 文案回归:除新增的 MIC_BLOCKED 与 INSUFFICIENT_PERMISSIONS(9 弃用切换说明)外,其余提示一字不改
        assertEquals("识别失败,请重试", voiceErrorText(VoiceErrorKind.OTHER))
        assertEquals("语音服务正忙,稍后再试", voiceErrorText(VoiceErrorKind.BUSY))
        assertEquals("没听到,再试一次", voiceErrorText(VoiceErrorKind.NO_MATCH))
        assertEquals("端侧语音引擎异常,请重试", voiceErrorText(VoiceErrorKind.ENGINE))
        assertEquals(
            "系统语音服务权限受限,已自动切换端侧识别;若仍失败,请在系统设置里开启麦克风权限",
            voiceErrorText(VoiceErrorKind.INSUFFICIENT_PERMISSIONS),
        )
    }
}
