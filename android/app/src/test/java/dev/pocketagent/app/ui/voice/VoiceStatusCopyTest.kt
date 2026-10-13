package dev.pocketagent.app.ui.voice

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 语音人话状态(快赢版)回归:相文案收敛 + 系统优先顺序。
 *
 * 口径:听着呢(收音中)/转文字中(停采转写窗口)/失败重试(本轮失败)。
 */
class VoiceStatusCopyTest {

    @Test
    fun `收敛三态_听着呢转文字中失败重试`() {
        assertEquals("听着呢", voicePhaseCopy(VoicePhase.LISTENING))
        assertEquals("转文字中", voicePhaseCopy(VoicePhase.STOPPING))
        assertEquals("转文字中", voicePhaseCopy(VoicePhase.TRANSCRIBING))
        assertEquals("失败重试", voicePhaseCopy(VoicePhase.ERROR))
    }

    @Test
    fun `其余相各有一句_非空可读`() {
        assertEquals("点按开始说话", voicePhaseCopy(VoicePhase.IDLE))
        assertEquals("准备中", voicePhaseCopy(VoicePhase.REQUESTING_PERMISSION))
        assertEquals("准备中", voicePhaseCopy(VoicePhase.STARTING))
        assertEquals("已暂停，点按继续", voicePhaseCopy(VoicePhase.PAUSED))
    }

    @Test
    fun `全相文案_无禁词`() {
        VoicePhase.entries.forEach { phase ->
            val copy = voicePhaseCopy(phase)
            assertTrue("$phase 文案不应为空", copy.isNotBlank())
            assertFalse("$phase 不应出现「未知」", copy.contains("未知"))
            assertFalse("$phase 不应出现「错误」", copy.contains("错误"))
            assertFalse("$phase 禁英文技术词 token", copy.lowercase().contains("token"))
            assertFalse("$phase 禁英文技术词 daemon", copy.lowercase().contains("daemon"))
            assertFalse("$phase 禁英文技术词 adb", copy.lowercase().contains("adb"))
        }
    }

    @Test
    fun `引擎优先级_麦克风优先走系统识别_端侧只是兜底`() {
        val order = voiceEnginePreferenceOrder()
        assertEquals(
            listOf(
                VoiceEngineKind.SYSTEM_ON_DEVICE,
                VoiceEngineKind.SYSTEM_REMOTE,
                VoiceEngineKind.SENSEVOICE,
                VoiceEngineKind.UNAVAILABLE,
            ),
            order,
        )
        // 系统两档都在端侧之前:与 pickVoiceEngine(系统可用一律系统侧)同序
        assertTrue(order.indexOf(VoiceEngineKind.SYSTEM_ON_DEVICE) < order.indexOf(VoiceEngineKind.SENSEVOICE))
        assertTrue(order.indexOf(VoiceEngineKind.SYSTEM_REMOTE) < order.indexOf(VoiceEngineKind.SENSEVOICE))
    }
}
