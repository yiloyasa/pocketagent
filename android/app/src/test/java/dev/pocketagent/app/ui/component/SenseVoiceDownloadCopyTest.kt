package dev.pocketagent.app.ui.component

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 会话页语音模型入口(快赢版)回归:下载进度百分比 + 三态人话。
 *
 * 进度只读 SenseVoiceModelState 已有字段(preparing/preparedBytes/
 * needsDownload/downloadFailed),不编造;调用方点按接
 * SenseVoiceModelManager.prepareBundledModel(幂等,已有重试链)。
 */
class SenseVoiceDownloadCopyTest {

    // ---- 百分比:0..100 夹逼 ----

    @Test
    fun `进度百分比_零半满三档`() {
        assertEquals(0, senseVoiceDownloadPercent(0L))
        // 向下取整:总量为奇数时半量落 49,属实现定义,钉死不漂移即可
        assertEquals(49, senseVoiceDownloadPercent(SenseVoiceModelManager.TOTAL_BYTES / 2))
        assertEquals(100, senseVoiceDownloadPercent(SenseVoiceModelManager.TOTAL_BYTES))
    }

    @Test
    fun `进度百分比_超量与负数收敛_绝不出现负号`() {
        assertEquals(100, senseVoiceDownloadPercent(SenseVoiceModelManager.TOTAL_BYTES + 1))
        assertEquals(0, senseVoiceDownloadPercent(-1L))
        assertFalse(senseVoiceDownloadPercent(-1L).toString().contains("-"))
    }

    // ---- 三态人话:会话页可见入口 ----

    @Test
    fun `就绪态_不摆入口`() {
        assertNull(senseVoiceDownloadCopy(SenseVoiceModelState(ready = true)))
    }

    @Test
    fun `下载中_带百分比_不断网编造`() {
        val copy = senseVoiceDownloadCopy(
            SenseVoiceModelState(preparing = true, preparedBytes = SenseVoiceModelManager.TOTAL_BYTES / 4),
        )
        // 四分之一量向下取整落 24%,断言与实现同式(只读 preparedBytes,不编造)
        assertEquals("语音模型下载中 24%，请保持网络连接", copy)
    }

    @Test
    fun `下载失败_点按重试`() {
        val copy = senseVoiceDownloadCopy(
            SenseVoiceModelState(needsDownload = true, downloadFailed = true, error = "模型下载失败 HTTP 500"),
        )
        assertEquals("语音模型下载失败，点按重试", copy)
    }

    @Test
    fun `从没下载过_点按下载`() {
        val copy = senseVoiceDownloadCopy(
            SenseVoiceModelState(needsDownload = true, error = SenseVoiceModelManager.NEEDS_DOWNLOAD_MESSAGE),
        )
        assertEquals("语音模型未下载，点按下载", copy)
    }

    // ---- 用户可见文案禁词纪律 ----

    @Test
    fun `下载入口文案_无禁词`() {
        val copies = listOfNotNull(
            senseVoiceDownloadCopy(SenseVoiceModelState(preparing = true, preparedBytes = 1L)),
            senseVoiceDownloadCopy(SenseVoiceModelState(needsDownload = true, downloadFailed = true, error = "x")),
            senseVoiceDownloadCopy(SenseVoiceModelState(needsDownload = true)),
        )
        assertEquals(3, copies.size)
        copies.forEach { copy ->
            assertTrue(copy.isNotBlank())
            assertFalse("不应出现「未知」:$copy", copy.contains("未知"))
            assertFalse("不应出现「错误」:$copy", copy.contains("错误"))
            assertFalse("用户文案禁英文技术词 token:$copy", copy.lowercase().contains("token"))
            assertFalse("用户文案禁英文技术词 daemon:$copy", copy.lowercase().contains("daemon"))
            assertFalse("用户文案禁英文技术词 adb:$copy", copy.lowercase().contains("adb"))
        }
    }
}
