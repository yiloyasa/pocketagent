package dev.pocketagent.app.ui.component

/**
 * 打开一个已知格式时该走哪条路 —— 与 [PreviewMode] 语义一致,但这里额外服务"下载/分享"决策。
 *
 * · PREVIEW_IN_APP → 内置浏览器渲染,不依赖手机上装任何 App;
 * · OPEN_SYSTEM   → 内置渲染不了,但有通用系统应用能打开(ACTION_VIEW);
 * · SAVE_ONLY     → 既不能预览也没有通用承接方,只能存到手机。
 */
enum class DocOpenMode { PREVIEW_IN_APP, OPEN_SYSTEM, SAVE_ONLY }

/**
 * 一种文档格式的支持描述 —— **扩展点**。新增格式 = 在 [DocFormatRegistry.formats] 里加一行。
 *
 * @param exts  小写扩展名(不含点)。同一能力可列多个后缀。
 * @param kind  复用 [FileKind] 分类,避免"能力清单"再写一份、两处走偏。
 * @param mime  规范 MIME(下载落盘/分享时用它,MediaStore 才不会给文件补错后缀)。
 */
data class DocFormat(
    val exts: Set<String>,
    val kind: FileKind,
    val mime: String,
    val mode: DocOpenMode,
) {
    /** 主扩展名(用于 MIME → 后缀反查)。Set 为插入序,first() 稳定。 */
    val primaryExt: String get() = exts.first()
}

/**
 * 常见文档格式的统一登记表(内置浏览器 / 下载 / 分享 / 用户数据管理共用)。
 *
 * 【唯一来源】"能不能内置打开"若分散写在浏览器、下载器、任务中心三个地方,必然走偏。
 * 这里用**一张可扩展的表**描述:扩展名 → 分类 + MIME + 打开模式。新增格式只加一行。
 *
 * 【与 FileKind 的关系】FileKind 负责"识别是什么 + 图标 + 预览能力";本表在其上加一层
 * "下载 MIME / 打开模式"的映射,并覆盖 FileKind 未细分的下载后缀(gif/webp/mp3/mp4…)。
 */
object DocFormatRegistry {

    /** 全部已知格式。**新增一行即新增一种支持**。 */
    val formats: List<DocFormat> = listOf(
        // ---- 文档 ----
        DocFormat(setOf("pdf"), FileKind.PDF, "application/pdf", DocOpenMode.PREVIEW_IN_APP),
        DocFormat(setOf("docx", "docm"), FileKind.DOCX, FileKind.DOCX.mime, DocOpenMode.PREVIEW_IN_APP),
        DocFormat(setOf("doc", "dot"), FileKind.DOC, "application/msword", DocOpenMode.PREVIEW_IN_APP),
        DocFormat(setOf("rtf"), FileKind.RTF, "application/rtf", DocOpenMode.PREVIEW_IN_APP),
        DocFormat(setOf("odt", "fodt"), FileKind.ODT, FileKind.ODT.mime, DocOpenMode.PREVIEW_IN_APP),
        // ---- 表格 ----
        DocFormat(setOf("xlsx", "xlsm"), FileKind.XLSX, FileKind.XLSX.mime, DocOpenMode.PREVIEW_IN_APP),
        DocFormat(setOf("xls"), FileKind.XLS, "application/vnd.ms-excel", DocOpenMode.PREVIEW_IN_APP),
        DocFormat(setOf("ods", "fods"), FileKind.ODS, FileKind.ODS.mime, DocOpenMode.PREVIEW_IN_APP),
        DocFormat(setOf("csv", "tsv"), FileKind.CSV, "text/csv", DocOpenMode.PREVIEW_IN_APP),
        // ---- 演示 ----
        DocFormat(setOf("pptx", "pptm"), FileKind.PPTX, FileKind.PPTX.mime, DocOpenMode.PREVIEW_IN_APP),
        DocFormat(setOf("ppt"), FileKind.PPT, "application/vnd.ms-powerpoint", DocOpenMode.PREVIEW_IN_APP),
        DocFormat(setOf("odp", "fodp"), FileKind.ODP, FileKind.ODP.mime, DocOpenMode.PREVIEW_IN_APP),
        // ---- 文本 / 结构化 ----
        DocFormat(setOf("md", "markdown", "mdx"), FileKind.MARKDOWN, "text/markdown", DocOpenMode.PREVIEW_IN_APP),
        DocFormat(setOf("txt", "log"), FileKind.TEXT, "text/plain", DocOpenMode.PREVIEW_IN_APP),
        DocFormat(setOf("json", "jsonl", "ndjson"), FileKind.DATA, "application/json", DocOpenMode.PREVIEW_IN_APP),
        DocFormat(setOf("xml"), FileKind.DATA, "application/xml", DocOpenMode.PREVIEW_IN_APP),
        DocFormat(setOf("html", "htm"), FileKind.HTML, "text/html", DocOpenMode.PREVIEW_IN_APP),
        // ---- 图片 ----
        DocFormat(setOf("png"), FileKind.IMAGE, "image/png", DocOpenMode.PREVIEW_IN_APP),
        DocFormat(setOf("jpg", "jpeg"), FileKind.IMAGE, "image/jpeg", DocOpenMode.PREVIEW_IN_APP),
        DocFormat(setOf("gif"), FileKind.IMAGE, "image/gif", DocOpenMode.PREVIEW_IN_APP),
        DocFormat(setOf("webp"), FileKind.IMAGE, "image/webp", DocOpenMode.PREVIEW_IN_APP),
        // ---- 压缩包 ----
        DocFormat(setOf("zip", "jar"), FileKind.ZIP, "application/zip", DocOpenMode.PREVIEW_IN_APP),
        DocFormat(
            setOf("rar", "7z", "tar", "gz", "tgz", "bz2", "xz"),
            FileKind.ARCHIVE, "application/octet-stream", DocOpenMode.SAVE_ONLY,
        ),
        // ---- 安装包 ----
        DocFormat(
            setOf("apk", "apks", "xapk"),
            FileKind.APK, "application/vnd.android.package-archive", DocOpenMode.OPEN_SYSTEM,
        ),
        // ---- 音视频 ----
        DocFormat(setOf("mp4", "mkv", "mov", "webm", "3gp", "m4v", "avi"), FileKind.VIDEO, "video/*", DocOpenMode.OPEN_SYSTEM),
        DocFormat(
            setOf("mp3", "wav", "flac", "aac", "ogg", "m4a", "opus", "amr"),
            FileKind.AUDIO, "audio/*", DocOpenMode.OPEN_SYSTEM,
        ),
    )

    /** 扩展名(小写)→ 格式。多个格式撞同一后缀时,先注册者胜(表内不放重复后缀)。 */
    private val byExt: Map<String, DocFormat> = HashMap<String, DocFormat>().apply {
        formats.forEach { f -> f.exts.forEach { e -> putIfAbsent(e.lowercase(), f) } }
    }

    /** 规范 MIME → 格式(去参数、小写)。 */
    private val byMime: Map<String, DocFormat> = HashMap<String, DocFormat>().apply {
        formats.forEach { f -> putIfAbsent(f.mime.lowercase(), f) }
    }

    /** 文件名后缀(小写,不含点)。 */
    fun extOf(name: String): String = name.substringAfterLast('.', "").lowercase()

    /** 由扩展名精确匹配(大小写不敏感,容忍带点的输入)。 */
    fun matchExt(ext: String): DocFormat? = byExt[ext.trim().lowercase().removePrefix(".")]

    /** 由 MIME 精确匹配(去 ";charset=..." 参数、小写)。 */
    fun matchMime(mime: String): DocFormat? =
        byMime[mime.substringBefore(';').trim().lowercase()]

    /**
     * 综合文件名与 MIME 判定格式。**优先按 MIME**:MediaStore 会给下载文件按 MIME 补后缀
     * (md 常被存成 `xxx.md.txt`),只看后缀会把 Markdown 当纯文本渲染。
     */
    fun formatFor(name: String, mime: String = ""): DocFormat? =
        matchMime(mime) ?: matchExt(extOf(name))

    /** 该文件"打开/分享"应走的模式;未知格式兜底 [DocOpenMode.OPEN_SYSTEM](交给系统应用)。 */
    fun modeFor(name: String, mime: String = ""): DocOpenMode =
        formatFor(name, mime)?.mode ?: DocOpenMode.OPEN_SYSTEM

    /** 文件名 → MIME;未登记的后缀交给 [fileKindOf] 兜底(它已覆盖源码/其它)。 */
    fun mimeFor(name: String): String {
        matchExt(extOf(name))?.let { return it.mime }
        return fileKindOf(name).mime
    }

    /**
     * MIME → 后缀(含点,如 ".pdf");未知返回空串(用于给无扩展名文件兜出可判定名)。
     *
     * 【两步】① 先走登记表**精确匹配**(去 ";charset=..." 参数 + 小写);
     * ② 精确失败再走**前缀/子串兜底**,兼容登记表里以通配形式存在的族(video/ 与 audio/ 前缀)
     *    与外部 MIME 变体(`application/x-zip-compressed` 等)。
     * 【切勿在注释里写出"斜杠紧跟星号"或"星号紧跟斜杠"这两种相邻字符序列】Kotlin 的块注释
     * **可以嵌套**(与 Java/C 不同):出现前者会在这里开一层新注释,出现后者会在这里提前收尾。
     * 两种情况都让编译器只报 "Unclosed comment" / "Missing '}'",而真凶藏在这行注释里 ——
     * 本文件已经踩过一次,写通配一律写作 `video/ 前缀`,不要写字面符号。
     * 兜底分支与旧 `BrowserScreen.extForMime` 的 17 条分支逐条等价,避免"删重复实现"时静默丢能力。
     */
    fun extFor(mime: String): String {
        val m = mime.substringBefore(';').trim().lowercase()
        if (m.isEmpty()) return ""
        byMime[m]?.let { return "." + it.primaryExt }
        return when {
            m.startsWith("text/markdown") -> ".md"
            m.startsWith("text/html") -> ".html"
            m.startsWith("text/plain") -> ".txt"
            m.startsWith("text/csv") -> ".csv"
            m.startsWith("image/png") -> ".png"
            m.startsWith("image/jpeg") -> ".jpg"
            m.startsWith("image/gif") -> ".gif"
            m.startsWith("image/webp") -> ".webp"
            m.startsWith("application/json") -> ".json"
            m.contains("spreadsheetml") -> ".xlsx"
            m.contains("wordprocessingml") -> ".docx"
            m.contains("presentationml") -> ".pptx"
            m.contains("vnd.android.package-archive") -> ".apk"
            m.contains("zip") -> ".zip"
            m.startsWith("video/") -> ".mp4"
            m.startsWith("audio/") -> ".mp3"
            else -> ""
        }
    }
}
