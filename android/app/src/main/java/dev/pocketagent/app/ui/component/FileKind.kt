package dev.pocketagent.app.ui.component

import androidx.compose.ui.graphics.Color
import dev.pocketagent.app.ui.theme.Pa

/**
 * 文件预览能力 —— 决定"点开一个文件"会发生什么。
 *
 * 【为什么要独立成枚举】之前只有一个是/否(browserViewable),无法表达
 * "能内置渲染"、"内置不了但手机上有应用能打开"、"只能存下来"这三种真实情况,
 * 于是 UI 上要么少一个按钮、要么点了没反应。现在一次说清。
 */
enum class PreviewMode {
    /** App 内置浏览器直接渲染,不需要手机上装任何 App。 */
    IN_APP,

    /** 内置渲染不了,但 Android 有通用系统应用能打开(交给 ACTION_VIEW,失败会提示)。 */
    SYSTEM,

    /** 既不能预览、也没有通用系统应用承接 —— 只能保存到手机。 */
    SAVE_ONLY,
}

/**
 * 文件类型识别 —— 任务中心、用户数据管理、内置浏览器、下载分派共用同一套判定。
 *
 * 【唯一来源】"能不能内置打开"写在浏览器里、"显示什么图标"写在任务中心里、
 * "MIME"又写一份,三份清单必然走偏。现在全部由 [FileKind] 描述:
 * 展示标签 / 徽标配色 / 预览能力 / MIME。
 *
 * 【细分原则】同一族格式只要"能力"不同就必须拆开:
 *   .docx(能解析)与 .doc(OLE 二进制,只能交给系统)不是一回事;
 *   .xlsx(能解析)与 .xls(需要 BIFF 解析)也不是一回事。
 * label 会直接画在徽标上,控制在 6 个汉字以内。
 */
enum class FileKind(
    /** 中文类型名,直接展示给用户(如"安装包""Markdown")。 */
    val label: String,
    /** 徽标底色。 */
    val color: Color,
    /** 预览能力(见 [PreviewMode])。 */
    val preview: PreviewMode,
    val mime: String,
) {
    APK("安装包", Pa.Green, PreviewMode.SYSTEM, "application/vnd.android.package-archive"),

    HTML("网页", Pa.AccentText, PreviewMode.IN_APP, "text/html"),

    MARKDOWN("Markdown", Pa.AccentText, PreviewMode.IN_APP, "text/markdown"),

    /** 纯文本与各类源码:统一等宽渲染。 */
    TEXT("文本", Pa.TextSecondary, PreviewMode.IN_APP, "text/plain"),

    /** CSV/TSV:按分隔符切成真表格(比等宽文本好读得多)。 */
    CSV("表格", Pa.Green, PreviewMode.IN_APP, "text/csv"),

    /** JSON/XML 等结构化文本:等宽 + 高亮底色。 */
    DATA("数据", Pa.Yellow, PreviewMode.IN_APP, "application/json"),

    IMAGE("图片", Pa.Mcp, PreviewMode.IN_APP, "image/*"),

    /** PDF:用系统自带的 PdfRenderer 逐页转图后内嵌,不依赖任何阅读器 App。 */
    PDF("PDF", Pa.Red, PreviewMode.IN_APP, "application/pdf"),

    /** OOXML Word(.docx):解析 word/document.xml 出段落与表格。 */
    DOCX("Word", Pa.Accent, PreviewMode.IN_APP, MIME_DOCX),

    /** 旧版 Word(.doc):OLE 复合文档 + FIB/分片表,内置解析性价比极低 → 交系统应用。 */
    DOC("Word 旧版", Pa.TextSecondary, PreviewMode.SYSTEM, "application/msword"),

    /** RTF:剥掉控制字后按富文本段落渲染。 */
    RTF("RTF", Pa.TextSecondary, PreviewMode.IN_APP, "application/rtf"),

    /** OpenDocument 文本(.odt)。 */
    ODT("ODT", Pa.TextSecondary, PreviewMode.IN_APP, MIME_ODT),

    /** OOXML Excel(.xlsx):解析 sharedStrings + sheets 出多表表格。 */
    XLSX("表格", Pa.Green, PreviewMode.IN_APP, MIME_XLSX),

    /** 旧版 Excel(.xls):自带 OLE2 + BIFF8 解析器,同样能出表格。 */
    XLS("表格 旧版", Pa.Green, PreviewMode.IN_APP, "application/vnd.ms-excel"),

    /** OpenDocument 表格(.ods)。 */
    ODS("表格", Pa.Green, PreviewMode.IN_APP, MIME_ODS),

    /** OOXML PowerPoint(.pptx):按页抽标题 + 正文要点(纯文本大纲,不带版式)。 */
    PPTX("演示", Pa.Yellow, PreviewMode.IN_APP, MIME_PPTX),

    /** 旧版 PowerPoint(.ppt):二进制记录流,交系统应用。 */
    PPT("演示 旧版", Pa.Yellow, PreviewMode.SYSTEM, "application/vnd.ms-powerpoint"),

    /** OpenDocument 演示(.odp)。 */
    ODP("演示", Pa.Yellow, PreviewMode.IN_APP, MIME_ODP),

    ARCHIVE("压缩包", Pa.TextSecondary, PreviewMode.SAVE_ONLY, "application/zip"),

    AUDIO("音频", Pa.Mcp, PreviewMode.SYSTEM, "audio/*"),

    VIDEO("视频", Pa.Mcp, PreviewMode.SYSTEM, "video/*"),

    OTHER("文件", Pa.TextTertiary, PreviewMode.SYSTEM, "application/octet-stream"),
    ;

    /** 能否交给 App 内置浏览器渲染。 */
    val browserViewable: Boolean get() = preview == PreviewMode.IN_APP

    /** 是否属于"表格"一族(任务中心/用户数据页用它给统计口径)。 */
    val isSheet: Boolean get() = this == XLSX || this == XLS || this == ODS || this == CSV
}

// MIME 常量放在文件级:枚举项在构造期还不能访问 companion object 里的成员。
private const val MIME_DOCX =
    "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
private const val MIME_XLSX =
    "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
private const val MIME_PPTX =
    "application/vnd.openxmlformats-officedocument.presentationml.presentation"
private const val MIME_ODT = "application/vnd.oasis.opendocument.text"
private const val MIME_ODS = "application/vnd.oasis.opendocument.spreadsheet"
private const val MIME_ODP = "application/vnd.oasis.opendocument.presentation"

/** 全部源码后缀:按等宽文本渲染(与 [FileKind.TEXT] 一致)。 */
private val CODE_EXT = setOf(
    "txt", "log", "ini", "conf", "cfg", "env", "properties", "gitignore", "editorconfig",
    "sh", "bash", "zsh", "bat", "cmd", "ps1", "kt", "kts", "java", "go", "py", "rb", "php",
    "js", "jsx", "ts", "tsx", "mjs", "cjs", "css", "scss", "less", "sql", "yaml", "yml",
    "toml", "c", "h", "cpp", "hpp", "cc", "cs", "rs", "swift", "dart", "lua", "pl", "r",
    "gradle", "makefile", "cmake", "dockerfile", "vue", "svelte", "proto",
)

/** 图片后缀(WebView 可直接显示)。 */
private val IMAGE_EXT = setOf(
    "png", "jpg", "jpeg", "gif", "webp", "bmp", "svg", "ico", "heic", "heif", "avif",
)

/** 由文件名后缀判定类型(大小写不敏感、容忍没有后缀)。 */
fun fileKindOf(name: String): FileKind {
    val ext = name.substringAfterLast('.', "").lowercase()
    return when {
        ext == "apk" || ext == "apks" || ext == "xapk" -> FileKind.APK
        ext == "html" || ext == "htm" || ext == "mhtml" || ext == "xhtml" -> FileKind.HTML
        ext == "md" || ext == "markdown" || ext == "mdx" -> FileKind.MARKDOWN
        ext == "csv" || ext == "tsv" -> FileKind.CSV
        ext == "json" || ext == "xml" || ext == "jsonl" || ext == "ndjson" -> FileKind.DATA
        ext in CODE_EXT -> FileKind.TEXT
        ext in IMAGE_EXT -> FileKind.IMAGE
        ext == "pdf" -> FileKind.PDF
        // ---- Office ----
        ext == "docx" || ext == "docm" -> FileKind.DOCX
        ext == "doc" || ext == "dot" -> FileKind.DOC
        ext == "rtf" -> FileKind.RTF
        ext == "odt" || ext == "fodt" -> FileKind.ODT
        ext == "xlsx" || ext == "xlsm" -> FileKind.XLSX
        ext == "xls" -> FileKind.XLS
        ext == "ods" || ext == "fods" -> FileKind.ODS
        ext == "pptx" || ext == "pptm" -> FileKind.PPTX
        ext == "ppt" -> FileKind.PPT
        ext == "odp" || ext == "fodp" -> FileKind.ODP
        // ---- 容器与媒体 ----
        ext in setOf("zip", "rar", "7z", "tar", "gz", "tgz", "bz2", "xz", "jar") ->
            FileKind.ARCHIVE
        ext in setOf("mp3", "wav", "flac", "aac", "ogg", "m4a", "opus", "amr") -> FileKind.AUDIO
        ext in setOf("mp4", "mkv", "mov", "avi", "webm", "3gp", "m4v") -> FileKind.VIDEO
        else -> FileKind.OTHER
    }
}

/**
 * 人类可读的体积。
 */
fun humanSize(bytes: Long): String = when {
    bytes >= 1024L * 1024 * 1024 -> String.format(java.util.Locale.US, "%.2fGB", bytes / 1073741824f)
    bytes >= 1024 * 1024 -> String.format(java.util.Locale.US, "%.1fMB", bytes / 1048576f)
    bytes >= 1024 -> String.format(java.util.Locale.US, "%.0fKB", bytes / 1024f)
    else -> "${bytes}B"
}
