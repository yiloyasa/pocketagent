package dev.pocketagent.app.ui.component

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Test

/** 项7:文档格式登记表的解析(大小写 / 多点扩展名 / MIME 优先 / 未知兜底)。 */
class DocFormatRegistryWorkflowC {

    @Test
    fun matchesExtensionCaseInsensitivelyAndToleratesDot() {
        assertEquals(FileKind.MARKDOWN, DocFormatRegistry.matchExt("MD")!!.kind)
        assertEquals(FileKind.PDF, DocFormatRegistry.matchExt(".pdf")!!.kind)
        assertEquals(FileKind.XLSX, DocFormatRegistry.matchExt(" xlsX ")!!.kind)
    }

    @Test
    fun formatForPrefersMimeOverConfusingExtension() {
        // MediaStore 会把 md 存成 xxx.md.txt:后缀像 txt,但 MIME 才是真相
        assertEquals(
            FileKind.MARKDOWN,
            DocFormatRegistry.formatFor("note.md.txt", "text/markdown")!!.kind,
        )
        // 没有 MIME 时退回后缀
        assertEquals(FileKind.XLSX, DocFormatRegistry.formatFor("book.xlsx")!!.kind)
    }

    @Test
    fun multiDotAndUnknownFallback() {
        assertEquals(FileKind.ARCHIVE, DocFormatRegistry.formatFor("archive.tar.gz")!!.kind)
        assertNull(DocFormatRegistry.formatFor("noext"))
        assertEquals(DocOpenMode.OPEN_SYSTEM, DocFormatRegistry.modeFor("weird.xyz"))
    }

    @Test
    fun openModesPerFamily() {
        assertEquals(DocOpenMode.PREVIEW_IN_APP, DocFormatRegistry.modeFor("a.docx"))
        assertEquals(DocOpenMode.PREVIEW_IN_APP, DocFormatRegistry.modeFor("a.pdf"))
        assertEquals(DocOpenMode.PREVIEW_IN_APP, DocFormatRegistry.modeFor("a.csv"))
        assertEquals(DocOpenMode.PREVIEW_IN_APP, DocFormatRegistry.modeFor("a.png"))
        assertEquals(DocOpenMode.SAVE_ONLY, DocFormatRegistry.modeFor("a.rar"))
        assertEquals(DocOpenMode.OPEN_SYSTEM, DocFormatRegistry.modeFor("a.mp4"))
        assertEquals(DocOpenMode.OPEN_SYSTEM, DocFormatRegistry.modeFor("a.apk"))
    }

    @Test
    fun mimeAndExtAreInverseForKnownFormats() {
        assertEquals("application/pdf", DocFormatRegistry.mimeFor("x.pdf"))
        assertEquals(".pdf", DocFormatRegistry.extFor("application/pdf"))
        // 带参数 MIME 必须能去参数后匹配
        assertEquals(".md", DocFormatRegistry.extFor("text/markdown; charset=utf-8"))
        // 未登记 MIME 返回空串(调用方据此跳过补后缀)
        assertEquals("", DocFormatRegistry.extFor("application/x-unknown-thing"))
    }

    /**
     * 零回归证据:逐条断言 `DocFormatRegistry.extFor` 与旧 `BrowserScreen.extForMime` 的
     * 17 条分支输入输出完全一致(该私有函数已被删除并改调这里)。
     */
    @Test
    fun extForMatchesLegacyExtForMimeOnAllBranches() {
        val expected = listOf(
            "text/markdown" to ".md",
            "text/html" to ".html",
            "text/plain" to ".txt",
            "text/csv" to ".csv",
            "image/png" to ".png",
            "image/jpeg" to ".jpg",
            "image/gif" to ".gif",
            "image/webp" to ".webp",
            "application/json" to ".json",
            "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" to ".xlsx",
            "application/vnd.openxmlformats-officedocument.wordprocessingml.document" to ".docx",
            "application/vnd.openxmlformats-officedocument.presentationml.presentation" to ".pptx",
            "application/pdf" to ".pdf",
            "application/vnd.android.package-archive" to ".apk",
            "application/zip" to ".zip",
            "application/x-zip-compressed" to ".zip",
            "video/mp4" to ".mp4",
            "audio/mpeg" to ".mp3",
            "application/x-unknown-thing" to "",
        )
        expected.forEach { (mime, ext) ->
            assertEquals("MIME=$mime", ext, DocFormatRegistry.extFor(mime))
        }
    }

    @Test
    fun coversRequiredCommonFormats() {
        val required = listOf(
            "pdf", "doc", "docx", "xls", "xlsx", "ppt", "pptx", "csv", "txt", "md",
            "json", "xml", "zip", "png", "jpg", "gif", "webp", "mp4", "mp3",
        )
        required.forEach { ext ->
            assertNotNull("缺少格式支持: $ext", DocFormatRegistry.matchExt(ext))
        }
    }
}
