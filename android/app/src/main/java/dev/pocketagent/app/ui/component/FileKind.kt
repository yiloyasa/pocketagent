package dev.pocketagent.app.ui.component

import androidx.compose.ui.graphics.Color
import dev.pocketagent.app.ui.theme.Pa

/**
 * 文件类型识别 —— 任务中心、用户数据管理、打开方式分派共用同一套判定。
 *
 * 【为什么要独立成一处】之前"能不能用内置浏览器打开"的逻辑写在 PaDownloader 里、
 * "显示什么图标"写在任务中心里、"MIME"又写了一份,三份清单很容易走偏;
 * 现在统一由 [FileKind] 描述:显示标签 / 徽标配色 / 能否内置查看 / MIME。
 */
enum class FileKind(
    /** 中文类型名,直接展示给用户(如"安装包""Markdown")。 */
    val label: String,
    /** 徽标底色。 */
    val color: Color,
    /** 能否用 App 内置浏览器渲染(网页/文本/图片)。 */
    val browserViewable: Boolean,
    val mime: String,
) {
    APK("安装包", Pa.Green, false, "application/vnd.android.package-archive"),
    HTML("网页", Pa.AccentText, true, "text/html"),
    TEXT("文本", Pa.TextSecondary, true, "text/plain"),
    MARKDOWN("Markdown", Pa.AccentText, true, "text/markdown"),
    DATA("数据", Pa.Yellow, true, "application/json"),
    IMAGE("图片", Pa.Mcp, true, "image/*"),
    PDF("PDF", Pa.Red, false, "application/pdf"),
    DOC("Word", Pa.Accent, false, "application/vnd.openxmlformats-officedocument.wordprocessingml.document"),
    SHEET("表格", Pa.Green, false, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"),
    SLIDES("演示", Pa.Yellow, false, "application/vnd.openxmlformats-officedocument.presentationml.presentation"),
    ARCHIVE("压缩包", Pa.TextSecondary, false, "application/zip"),
    AUDIO("音频", Pa.Mcp, false, "audio/*"),
    VIDEO("视频", Pa.Mcp, false, "video/*"),
    OTHER("文件", Pa.TextTertiary, false, "application/octet-stream"),
    ;

    /** 能直接交给系统打开的类型(安装包/文档/音视频等)。 */
    val needsSystemHandler: Boolean get() = this != OTHER || true
}

/** 由文件名后缀判定类型(大小写不敏感、容忍没有后缀)。 */
fun fileKindOf(name: String): FileKind {
    val ext = name.substringAfterLast('.', "").lowercase()
    return when (ext) {
        "apk", "apks", "xapk" -> FileKind.APK
        "html", "htm", "mhtml" -> FileKind.HTML
        "txt", "log", "ini", "conf", "cfg", "sh", "bat", "ps1", "kt", "go", "java", "py", "js", "ts", "css", "sql", "yaml", "yml", "toml" -> FileKind.TEXT
        "md", "markdown" -> FileKind.MARKDOWN
        "json", "xml", "csv", "tsv" -> FileKind.DATA
        "png", "jpg", "jpeg", "gif", "webp", "bmp", "svg", "ico" -> FileKind.IMAGE
        "pdf" -> FileKind.PDF
        "doc", "docx", "rtf", "odt" -> FileKind.DOC
        "xls", "xlsx", "ods" -> FileKind.SHEET
        "ppt", "pptx", "odp" -> FileKind.SLIDES
        "zip", "rar", "7z", "tar", "gz", "bz2", "xz" -> FileKind.ARCHIVE
        "mp3", "wav", "flac", "aac", "ogg", "m4a" -> FileKind.AUDIO
        "mp4", "mkv", "mov", "avi", "webm" -> FileKind.VIDEO
        else -> FileKind.OTHER
    }
}

/** 人类可读的体积。 */
fun humanSize(bytes: Long): String = when {
    bytes >= 1024L * 1024 * 1024 -> String.format(java.util.Locale.US, "%.2fGB", bytes / 1073741824f)
    bytes >= 1024 * 1024 -> String.format(java.util.Locale.US, "%.1fMB", bytes / 1048576f)
    bytes >= 1024 -> String.format(java.util.Locale.US, "%.0fKB", bytes / 1024f)
    else -> "${bytes}B"
}
