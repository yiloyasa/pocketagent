package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertNull
import org.junit.Test

class AttachmentPreviewTest {
    @Test
    fun classifiesImagesByMediaTypeOrExtension() {
        assertEquals(AttachmentKind.IMAGE, attachmentKind("photo.bin", "image/jpeg"))
        assertEquals(AttachmentKind.IMAGE, attachmentKind("photo.PNG", ""))
        assertEquals(AttachmentKind.FILE, attachmentKind("report.pdf", "application/pdf"))
    }

    @Test
    fun formatsAttachmentSizesForCards() {
        assertEquals("512 B", formatAttachmentSize(512))
        assertEquals("1.5 KB", formatAttachmentSize(1536))
        assertEquals("2.0 MB", formatAttachmentSize(2L * 1024 * 1024))
    }

    @Test
    fun keeps_image_bytes_for_local_preview_but_not_regular_files() {
        val bytes = byteArrayOf(1, 2, 3)

        val image = attachmentFromBytes(
            name = "photo.jpg",
            pcPath = "G:\\workspace\\.pocketagent-uploads\\photo.jpg",
            bytes = bytes,
            mediaType = "image/jpeg",
            localUri = "content://photo",
        )
        val file = attachmentFromBytes(
            name = "notes.txt",
            pcPath = "G:\\workspace\\.pocketagent-uploads\\notes.txt",
            bytes = bytes,
            mediaType = "text/plain",
            localUri = "content://notes",
        )

        assertArrayEquals(bytes, image.previewData)
        assertNull(file.previewData)
    }
}
