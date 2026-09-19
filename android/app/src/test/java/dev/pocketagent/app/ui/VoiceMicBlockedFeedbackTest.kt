package dev.pocketagent.app.ui

import dev.pocketagent.app.ui.component.VOICE_ERROR_ENGINE_BUSY
import dev.pocketagent.app.ui.component.VOICE_ERROR_MIC_BLOCKED
import dev.pocketagent.app.ui.component.VOICE_ERROR_VOSK_ENGINE
import dev.pocketagent.app.ui.component.VOICE_ERROR_VOSK_MODEL
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
 */
class VoiceMicBlockedFeedbackTest {

    // ------------------------------------------------------------ 102/103 补映射

    @Test
    fun `端侧102麦克风被拦映射MIC_BLOCKED_103引擎忙映射BUSY`() {
        assertEquals(VoiceErrorKind.MIC_BLOCKED, voiceErrorKind(VOICE_ERROR_MIC_BLOCKED))
        assertEquals(VoiceErrorKind.BUSY, voiceErrorKind(VOICE_ERROR_ENGINE_BUSY))
        // 端侧既有 100/101 映射不被新分支影响
        assertEquals(VoiceErrorKind.ENGINE, voiceErrorKind(VOICE_ERROR_VOSK_ENGINE))
        assertEquals(VoiceErrorKind.ENGINE, voiceErrorKind(VOICE_ERROR_VOSK_MODEL))
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

    @Test
    fun `MIC_BLOCKED事件经状态机产出该提示_回空闲`() {
        var ui = voiceReduce(VoiceInputUi(), VoiceEvent.MicClicked(true, true))
        assertEquals(VoiceInputState.LISTENING, ui.state)
        val (display, fx) = applyVoiceEvent(ui, VoiceEvent.RecognitionFailed(voiceErrorKind(VOICE_ERROR_MIC_BLOCKED)))
        assertEquals(VoiceInputState.IDLE, display.state)
        val toast = fx.toast!!
        assertTrue(toast.contains("安全中心"))
        assertTrue(voiceEffectsPending(fx))
    }

    // ------------------------------------------------------------ 系统引擎弃用判定(错误码名单)

    @Test
    fun `系统识别错误码名单_含3录音错误_命中即弃用系统引擎改选端侧`() {
        // 1 网络超时 / 2 网络 / 3 录音错误(MIUI「录音」开关关的典型表现,旧名单漏了)
        // / 4 服务端 / 5 客户端 / 8 忙 / 11 断连 / 12-13 语言不支持
        for (code in listOf(1, 2, 3, 4, 5, 8, 11, 12, 13)) {
            assertTrue("code=$code 应触发弃用", systemVoiceRetireNeeded(code))
        }
    }

    @Test
    fun `系统识别弃用名单_正常结果与权限与端侧码一律不弃用`() {
        // 6 说话超时 / 7 没听到:正常业务结果,弃用会把"没听到"误判成"服务坏了"
        for (code in listOf(6, 7)) assertFalse(systemVoiceRetireNeeded(code))
        // 9 权限:有专门"去设置开启"提示且用户授权后可自愈,不能弃用
        assertFalse(systemVoiceRetireNeeded(9))
        // 0/负数/未知码:不弃用
        for (code in listOf(0, -1, 10, 99)) assertFalse(systemVoiceRetireNeeded(code))
        // 100+ 端侧自定义码:到达时系统引擎本就未被选中,绝不能误伤
        for (code in listOf(VOICE_ERROR_VOSK_ENGINE, VOICE_ERROR_VOSK_MODEL, VOICE_ERROR_MIC_BLOCKED, VOICE_ERROR_ENGINE_BUSY)) {
            assertFalse(systemVoiceRetireNeeded(code))
        }
    }

    // ------------------------------------------------------------ 死区超时的弃用判定

    @Test
    fun `死区超时_仅当当前引擎是系统时才弃用_端侧不因死区自动弃用`() {
        // 系统 SR"预检可用但零回调死区":必须弃用,否则永远循环"点击→空转8秒→无响应"
        assertTrue(voiceDeadZoneShouldRetireSystem(systemEngineSelected = true))
        // 端侧引擎死区:提示已覆盖,且端侧重建要重新冷加载,不自动弃用
        assertFalse(voiceDeadZoneShouldRetireSystem(systemEngineSelected = false))
    }

    // ------------------------------------------------------------ 系统码映射回归(不被新分支吞掉)

    @Test
    fun `系统码原有映射不受影响_3仍归OTHER_7_8_9_5不变`() {
        assertEquals(VoiceErrorKind.OTHER, voiceErrorKind(3)) // ERROR_AUDIO_ERROR
        assertEquals(VoiceErrorKind.NO_MATCH, voiceErrorKind(7))
        assertEquals(VoiceErrorKind.BUSY, voiceErrorKind(8))
        assertEquals(VoiceErrorKind.INSUFFICIENT_PERMISSIONS, voiceErrorKind(9))
        assertEquals(VoiceErrorKind.CLIENT, voiceErrorKind(5))
        assertEquals(VoiceErrorKind.OTHER, voiceErrorKind(1))
        // 文案回归:除新增的 MIC_BLOCKED 外,其余提示一字不改
        assertEquals("识别失败,请重试", voiceErrorText(VoiceErrorKind.OTHER))
        assertEquals("语音服务正忙,稍后再试", voiceErrorText(VoiceErrorKind.BUSY))
        assertEquals("没听到,再试一次", voiceErrorText(VoiceErrorKind.NO_MATCH))
        assertEquals("端侧语音引擎异常,请重试", voiceErrorText(VoiceErrorKind.ENGINE))
    }
}
