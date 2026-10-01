package dev.pocketagent.app.ui

import dev.pocketagent.app.R
import dev.pocketagent.app.ui.component.FileKind
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 文件列表展示规则(FilesListLogic.kt)的单测 —— R2-D 落地项 1/6:
 * 类型图标分型、目录/文件排序、大小与时间的元信息拼装。
 */
class FilesListLogicTest {

    private fun row(
        name: String,
        isDir: Boolean = false,
        size: Long = 0,
        mtime: Long = 0,
        path: String = "G:\\dir\\$name",
    ) = FileRowModel(name, path, isDir, size, mtime)

    // ---- 排序 ----

    @Test
    fun `排序 目录恒在前 组内按名字不分大小写`() {
        val sorted = sortFileRows(
            listOf(
                row("beta.txt"),
                row("Zeta.txt"),
                row("sub", isDir = true),
                row("alpha.txt"),
                row("APP", isDir = true),
            ),
        )
        assertEquals(listOf("APP", "sub", "alpha.txt", "beta.txt", "Zeta.txt"), sorted.map { it.name })
    }

    @Test
    fun `排序 不修改入参列表`() {
        val src = mutableListOf(row("b.txt"), row("a.txt"))
        sortFileRows(src)
        assertEquals(listOf("b.txt", "a.txt"), src.map { it.name })
    }

    @Test
    fun `排序 同名不同路径 结果稳定可复现`() {
        val a1 = row("x.txt", path = "G:\\a\\x.txt")
        val a2 = row("x.txt", path = "G:\\b\\x.txt")
        assertEquals(
            sortFileRows(listOf(a2, a1)).map { it.path },
            sortFileRows(listOf(a1, a2)).map { it.path },
        )
    }

    // ---- 图标分型 ----

    @Test
    fun `图标 目录恒为文件夹 文件按类型分型`() {
        assertEquals(R.drawable.ic_lucide_folder, fileRowIcon(true, FileKind.OTHER))
        assertEquals(R.drawable.ic_lucide_file_text, fileRowIcon(false, FileKind.TEXT))
        assertEquals(R.drawable.ic_lucide_file_code, fileRowIcon(false, FileKind.DATA))
        assertEquals(R.drawable.ic_lucide_image, fileRowIcon(false, FileKind.IMAGE))
        assertEquals(R.drawable.ic_lucide_archive, fileRowIcon(false, FileKind.ZIP))
        assertEquals(R.drawable.ic_lucide_archive, fileRowIcon(false, FileKind.ARCHIVE))
        assertEquals(R.drawable.ic_lucide_table, fileRowIcon(false, FileKind.CSV))
        assertEquals(R.drawable.ic_lucide_music, fileRowIcon(false, FileKind.AUDIO))
        assertEquals(R.drawable.ic_lucide_video, fileRowIcon(false, FileKind.VIDEO))
        assertEquals(R.drawable.ic_lucide_file, fileRowIcon(false, FileKind.OTHER))
    }

    @Test
    fun `图标 常见类型两两可区分`() {
        val icons = listOf(
            fileRowIcon(false, FileKind.TEXT),
            fileRowIcon(false, FileKind.IMAGE),
            fileRowIcon(false, FileKind.ZIP),
            fileRowIcon(false, FileKind.DATA),
        )
        assertEquals(icons.size, icons.distinct().size)
    }

    // ---- 元信息 ----

    @Test
    fun `元信息 文件包含 大小 类型与预览能力`() {
        val line = entryMetaLine(row("a.kt", size = 2048))
        assertEquals("2.0 KB  ·  文本  ·  可预览", line)
    }

    @Test
    fun `元信息 不可内置预览的类型标注需下载`() {
        val line = entryMetaLine(row("x.7z", size = 10))
        assertTrue(line.contains("压缩包"))
        assertTrue(line.contains("需下载打开"))
    }

    @Test
    fun `元信息 目录只有时间 无时间则为空`() {
        assertEquals("", entryMetaLine(row("sub", isDir = true)))
        val t = entryMetaLine(row("sub", isDir = true, mtime = System.currentTimeMillis()))
        assertTrue(t.isNotEmpty())
    }

    @Test
    fun `元信息 未识别类型不显示类型名但保留大小`() {
        val line = entryMetaLine(row("blob.xyz", size = 5))
        assertEquals("5 B  ·  需下载打开", line)
    }

    // ---- 体积格式 ----

    @Test
    fun `体积 各档位与旧格式同形 GB 档不丢精度`() {
        assertEquals("0 B", formatFileSizeHuman(0))
        assertEquals("1023 B", formatFileSizeHuman(1023))
        assertEquals("1.0 KB", formatFileSizeHuman(1024))
        assertEquals("1.5 KB", formatFileSizeHuman(1536))
        assertEquals("1.0 MB", formatFileSizeHuman(1024L * 1024))
        assertEquals("1.00 GB", formatFileSizeHuman(1024L * 1024 * 1024))
        // 老实现对 >1GB 会显示成 "1424.2 MB"
        assertNotEquals("1424.2 MB", formatFileSizeHuman(1493175767))
        assertTrue(formatFileSizeHuman(1493175767).endsWith("GB"))
    }

    // ---- 时间格式 ----

    @Test
    fun `时间 无时间戳返回 null`() {
        assertNull(formatEntryTime(0, nowMs = 1_000_000L))
        assertNull(formatEntryTime(-5, nowMs = 1_000_000L))
    }

    @Test
    fun `时间 同年用 月日时分 跨年补年份`() {
        val cal = java.util.Calendar.getInstance()
        cal.set(2026, java.util.Calendar.SEPTEMBER, 17, 9, 30, 0)
        val sameYear = cal.timeInMillis
        val now = cal.timeInMillis + 1 // 确保判定为同一年
        val fmt = java.text.SimpleDateFormat("MM-dd HH:mm", java.util.Locale.getDefault())
        assertEquals(fmt.format(java.util.Date(sameYear)), formatEntryTime(sameYear, nowMs = now))

        cal.set(2020, java.util.Calendar.JANUARY, 2, 3, 4, 0)
        val old = cal.timeInMillis
        val fmtY = java.text.SimpleDateFormat("yyyy-MM-dd", java.util.Locale.getDefault())
        assertEquals(fmtY.format(java.util.Date(old)), formatEntryTime(old, nowMs = now))
    }

    // ---- proto 映射 ----

    @Test
    fun `proto 条目映射 字段一一对应 类型按名字推导`() {
        val e = pocketagent.v1.DirectoryEntry.newBuilder()
            .setName("Main.kt")
            .setPath("G:\\proj\\Main.kt")
            .setIsDir(false)
            .setSizeBytes(1024)
            .setModifiedUnixMs(42)
            .build()
        val m = e.toRowModel()
        assertEquals("Main.kt", m.name)
        assertEquals("G:\\proj\\Main.kt", m.path)
        assertEquals(false, m.isDir)
        assertEquals(1024L, m.sizeBytes)
        assertEquals(42L, m.modifiedUnixMs)
        assertEquals(FileKind.TEXT, m.kind)
    }
}
