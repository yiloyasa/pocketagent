package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Test

/**
 * 【P1-8】附件缩略图两段解码的抽样档纯函数([ChatScreen.attachmentThumbnailSampleSize])
 * 的 JVM 钉子:64dp 卡图按 ~128px 解析度取最小 2^n inSampleSize;尺寸未知/已不超目标
 * 按 1(原样解码)。全尺寸解码保留给附件大图预览,不在此函数职责内。
 */
class AttachmentThumbnailSampleSizeTest {

    @Test
    fun `原尺寸不超目标按1_不解码放大`() {
        assertEquals(1, attachmentThumbnailSampleSize(100, 80))
        assertEquals(1, attachmentThumbnailSampleSize(128, 128))
    }

    @Test
    fun `大图压到目标附近的最小2的幂`() {
        // 4000x3000:除到 250x187(再除宽 125 < 128)→ 16
        assertEquals(16, attachmentThumbnailSampleSize(4000, 3000))
        // 1024x768:512x384 → 256x192(128x96 宽不够)→ 4
        assertEquals(4, attachmentThumbnailSampleSize(1024, 768))
        // 8192x8192:→ 4096 → 2048 → 1024 → 512 → 256 → 128(再除 64 < 128)→ 64
        assertEquals(64, attachmentThumbnailSampleSize(8192, 8192))
    }

    @Test
    fun `尺寸未知或非法按1_交由decode原样兜底`() {
        assertEquals(1, attachmentThumbnailSampleSize(-1, 800))
        assertEquals(1, attachmentThumbnailSampleSize(0, 0))
    }
}
