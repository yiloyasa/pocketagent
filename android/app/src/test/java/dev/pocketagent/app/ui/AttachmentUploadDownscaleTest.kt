package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

/**
 * 发送链路原图下采样(性能侦察 G3 残段)决策纯函数的 JVM 钉子。
 *
 * 口径:长边 > [AttachmentPreview.ATTACHMENT_UPLOAD_MAX_EDGE_PX](1600px)才压;
 * 格式**保持原样**(jpeg/png/webp),gif/heic/bmp/未知格式一律原样上传
 * (动图重编码丢动画、未知格式转出同名同型字节没把握 —— 原样传输语义只对
 * 静态图片放宽);Bitmap 解码/压缩本身是 android 现场逻辑,不在 JVM 钉子内
 * (与 attachmentThumbnailSampleSize 同一测试边界)。
 */
class AttachmentUploadDownscaleTest {

    // ---- 格式决策:保格式,不认得的原样 ----

    @Test
    fun `格式决策_jpeg_png_webp各归各档_大小写不敏感`() {
        assertEquals(UploadImageFormat.JPEG, uploadImageFormatOf("image/jpeg", "photo.jpg"))
        assertEquals(UploadImageFormat.JPEG, uploadImageFormatOf("IMAGE/JPG", "photo.JPG"))
        assertEquals(UploadImageFormat.PNG, uploadImageFormatOf("image/png", "shot.png"))
        assertEquals(UploadImageFormat.WEBP, uploadImageFormatOf("image/webp", "pic.webp"))
    }

    @Test
    fun `格式决策_png认扩展名兜底_mediaType缺席不误判`() {
        // 某些供源不给 mime,靠扩展名认出 png(保透明语义不丢)
        assertEquals(UploadImageFormat.PNG, uploadImageFormatOf("", "screenshot.PNG"))
    }

    @Test
    fun `格式决策_gif与未知格式返回null_原样上传`() {
        // 动图重编码会丢动画;heic/bmp/无扩展名没有把握安全转出同名同型字节
        assertNull(uploadImageFormatOf("image/gif", "anim.gif"))
        assertNull(uploadImageFormatOf("image/heic", "IMG_0001.heic"))
        assertNull(uploadImageFormatOf("image/bmp", "old.bmp"))
        assertNull(uploadImageFormatOf("application/pdf", "doc.pdf"))
        assertNull(uploadImageFormatOf("", "noext"))
    }

    // ---- 目标尺寸:长边压到 1600,短边等比 ----

    @Test
    fun `目标尺寸_长边超阈值按比例压到1600`() {
        // 横图 4000x3000 → 1600x1200
        assertEquals(1600 to 1200, uploadDownscaleSizeOf(4000, 3000))
        // 竖图/长截图 1080x2400 → 720x1600(长边认 max,不是"宽压到1600")
        assertEquals(720 to 1600, uploadDownscaleSizeOf(1080, 2400))
    }

    @Test
    fun `目标尺寸_未超阈值或尺寸非法原样返回`() {
        assertEquals(1600 to 900, uploadDownscaleSizeOf(1600, 900))
        assertEquals(800 to 600, uploadDownscaleSizeOf(800, 600))
        assertEquals(0 to 0, uploadDownscaleSizeOf(0, 0))
        assertEquals(-1 to 800, uploadDownscaleSizeOf(-1, 800))
    }

    // ---- 两段解码抽样档:长边口径(与缩略图的"两边都到目标"口径不同) ----

    @Test
    fun `抽样档_长边降到仍不小于阈值的最小2的幂`() {
        // 4032 长边:/2=2016 ≥1600 ✓,/4=1008 <1600 → 2(峰值内存 48MB→12MB)
        assertEquals(2, uploadDownscaleSampleSizeOf(4032, 3024))
        // 3200 长边:/2=1600 恰好达标 → 2
        assertEquals(2, uploadDownscaleSampleSizeOf(3200, 2400))
        // 长边本就不超阈值:原样解码
        assertEquals(1, uploadDownscaleSampleSizeOf(1600, 1200))
        assertEquals(1, uploadDownscaleSampleSizeOf(800, 600))
    }

    @Test
    fun `抽样档_尺寸非法按1_交由原字节兜底`() {
        assertEquals(1, uploadDownscaleSampleSizeOf(0, 0))
        assertEquals(1, uploadDownscaleSampleSizeOf(-1, 3000))
    }

    // ---- EXIF 方向:只认 90/180/270 纯旋转 ----

    @Test
    fun `exif方向_三个纯旋转各给角度_其余不转`() {
        assertEquals(90, uploadExifRotationDegreesOf(6))
        assertEquals(180, uploadExifRotationDegreesOf(3))
        assertEquals(270, uploadExifRotationDegreesOf(8))
        assertEquals(0, uploadExifRotationDegreesOf(1)) // ORIENTATION_NORMAL
        assertEquals(0, uploadExifRotationDegreesOf(0)) // 缺失
        // 翻转类(2/4/5/7)无法用旋转还原,硬转反而错:按 0
        assertEquals(0, uploadExifRotationDegreesOf(5))
    }

    @Test
    fun `阈值常量与质量口径钉死`() {
        assertEquals(1600, ATTACHMENT_UPLOAD_MAX_EDGE_PX)
        assertEquals(85, ATTACHMENT_UPLOAD_JPEG_QUALITY)
    }
}
