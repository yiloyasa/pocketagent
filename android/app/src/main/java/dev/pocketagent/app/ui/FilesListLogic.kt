// FilesListLogic.kt —— 文件列表的展示规则(纯函数,可单测)。
//
// 【为什么抽出来】FilesScreen 里"怎么排序、一行元信息怎么拼、图标用哪个、
// 时间怎么显示"原来都是内联表达式,改一处全靠肉眼在真机上对。这里收敛成
// 纯函数 + 单测:排序稳定性、元信息语义、GB 档不丢精度都有用例锁住。
package dev.pocketagent.app.ui

import dev.pocketagent.app.R
import dev.pocketagent.app.ui.component.FileKind
import dev.pocketagent.app.ui.component.fileKindOf

/**
 * 文件列表一行的展示模型(纯逻辑层,可单测;与 proto DirectoryEntry 字段一一对应)。
 */
data class FileRowModel(
    val name: String,
    val path: String,
    val isDir: Boolean,
    val sizeBytes: Long,
    val modifiedUnixMs: Long,
) {
    val kind: FileKind by lazy(LazyThreadSafetyMode.NONE) { fileKindOf(name) }
}

/** proto DirectoryEntry → [FileRowModel](文件页/文件夹选择器共用)。 */
fun pocketagent.v1.DirectoryEntry.toRowModel(): FileRowModel =
    FileRowModel(
        name = name,
        path = path,
        isDir = isDir,
        sizeBytes = sizeBytes,
        modifiedUnixMs = modifiedUnixMs,
    )

/**
 * 列表排序:目录恒在前;同组内按名字不分大小写升序,同名再按完整路径稳定排序。
 * 纯函数,不修改入参。
 */
fun sortFileRows(rows: List<FileRowModel>): List<FileRowModel> =
    rows.sortedWith(
        compareByDescending<FileRowModel> { it.isDir }
            .thenBy { it.name.lowercase() }
            .thenBy { it.path },
    )

/**
 * 行首图标:目录恒用文件夹;文件按 [FileKind] 分型取各自图标
 * (代码/文档/图片/压缩包/音视频/未知在列表里一眼可分)。
 */
fun fileRowIcon(isDir: Boolean, kind: FileKind): Int =
    if (isDir) R.drawable.ic_lucide_folder else kind.iconRes

/**
 * 行内元信息(纯文本拼装,UI 只负责渲染):
 * · 文件:`大小 · 类型 · 时间 · 可预览|需下载打开`(缺项自动省略,不出现空段);
 * · 目录:只有时间(daemon 没给目录大小,不编数字)。
 */
fun entryMetaLine(row: FileRowModel): String {
    val parts = ArrayList<String>(4)
    if (!row.isDir) {
        parts.add(formatFileSizeHuman(row.sizeBytes))
        if (row.kind != FileKind.OTHER) parts.add(row.kind.label)
    }
    formatEntryTime(row.modifiedUnixMs)?.let { parts.add(it) }
    if (!row.isDir) {
        parts.add(if (row.kind.preview == dev.pocketagent.app.ui.component.PreviewMode.IN_APP) "可预览" else "需下载打开")
    }
    return parts.joinToString("  ·  ")
}

/**
 * 人类可读体积(带空格档位):B / KB / MB / GB。
 * 与旧 formatSize 输出同形("12 B"、"1.5 KB"),补上 GB 档 —— 老实现 >1GB
 * 会显示成 "1424.2 MB"。
 */
fun formatFileSizeHuman(n: Long): String = when {
    n < 1024 -> "$n B"
    n < 1024L * 1024 -> "%.1f KB".format(n / 1024.0)
    n < 1024L * 1024 * 1024 -> "%.1f MB".format(n / 1024.0 / 1024.0)
    else -> "%.2f GB".format(n / 1073741824.0)
}

/**
 * 条目时间显示:同一年 → "MM-dd HH:mm",跨年 → "yyyy-MM-dd";无时间戳返回 null。
 * @param nowMs 判定"同年"的基准(默认系统当前时间;单测传入固定值保证可断言)。
 */
fun formatEntryTime(unixMs: Long, nowMs: Long = System.currentTimeMillis()): String? {
    if (unixMs <= 0) return null
    val cal = java.util.Calendar.getInstance()
    cal.timeInMillis = nowMs
    val thisYear = cal.get(java.util.Calendar.YEAR)
    cal.timeInMillis = unixMs
    val fileYear = cal.get(java.util.Calendar.YEAR)
    val pattern = if (fileYear == thisYear) "MM-dd HH:mm" else "yyyy-MM-dd"
    return java.text.SimpleDateFormat(pattern, java.util.Locale.getDefault()).format(java.util.Date(unixMs))
}
