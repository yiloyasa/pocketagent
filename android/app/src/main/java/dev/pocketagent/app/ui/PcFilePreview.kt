package dev.pocketagent.app.ui

import android.content.Context
import android.util.Xml
import dev.pocketagent.app.ui.component.PaPaths
import org.xmlpull.v1.XmlPullParser
import java.io.ByteArrayInputStream
import java.util.zip.ZipInputStream

/**
 * 电脑 / 手机上的文件 → 内置浏览器可渲染的 HTML(纯本地解析,不引第三方库)。
 *
 * 【为什么要做】agent 产出的 Excel/Word/PDF 之前都被当普通文本塞进 <pre> ——
 * 它们是二进制,看到的是乱码。这里用系统自带的 ZipInputStream + XmlPullParser +
 * PdfRenderer 把常见办公格式直接解析成真正的表格 / 文档 / 页面。
 *
 * 【必须跟随主题】预览页是 HTML,颜色写死就跟 App 的深/浅主题脱节 ——
 * 用户在浅色主题下打开会话,预览却是黑底(图一 vs 图二 的观感差异来源之一)。
 * 所有渲染都通过 [Pal] 取色,由调用方传入当前主题。
 *
 * 【必须读单元格样式】Excel 的"表头深色底 + 白字、参数区黄色底"是文件里的
 * 填充色,不是我们的装饰 —— 不读 styles.xml 就会像图一那样全部丢色。
 *
 * 【文字必须完整】之前 td 上写了 max-width + ellipsis,长文本被截成"…2…"。
 * 现在换行显示,配合横向滚动容器。
 *
 * 【覆盖矩阵】xlsx / xls / ods / csv / tsv → 表格;docx / doc / odt / rtf → 文档;
 * pptx / ppt / odp → 演示大纲;pdf → 逐页位图;zip / jar → 条目列表;
 * md / txt / json / xml / 源码 → 排版页 / 等宽页;html / 图片 → WebView 原生渲染。
 */
object PcFilePreview {

    /** 单表行数上限:防止巨型表格把 WebView 内存打爆(超出截断并标注)。 */
    private const val MAX_ROWS = 800
    private const val MAX_SHEETS = 3
    private const val MAX_COLS = 40

    /** PDF 逐页位图:页数与目标宽度都设上限,避免一次塞进十几 MB 的 base64。 */
    private const val MAX_PDF_PAGES = 12
    private const val PDF_TARGET_WIDTH = 1000

    /** 演示文稿最多渲染多少页。 */
    private const val MAX_SLIDES = 40

    /** 压缩包最多列多少条目。 */
    private const val MAX_ZIP_ENTRIES = 500

    /**
     * 把文件字节渲染成完整 HTML。
     * @param dark 当前 App 是否深色主题 —— 预览页配色必须跟主题走
     * @return null = 该扩展名没有内置渲染器(调用方降级处理)。
     */
    fun render(ctx: Context, fileName: String, bytes: ByteArray, dark: Boolean): String? {
        val n = fileName.lowercase()
        val pal = Pal(dark)
        return when {
            n.endsWith(".xlsx") || n.endsWith(".xlsm") -> guard(pal) { xlsx(bytes, pal) }
            n.endsWith(".xls") -> guard(pal) { xls(bytes, pal) }
            n.endsWith(".ods") || n.endsWith(".fods") -> guard(pal) { odf(bytes, pal) }
            n.endsWith(".docx") || n.endsWith(".docm") -> guard(pal) { docx(bytes, pal) }
            n.endsWith(".doc") || n.endsWith(".dot") -> guard(pal) { docLegacy(bytes, pal) }
            n.endsWith(".ppt") -> guard(pal) { pptLegacy(bytes, pal) }
            n.endsWith(".rtf") -> guard(pal) { rtf(bytes, pal) }
            n.endsWith(".odt") || n.endsWith(".fodt") -> guard(pal) { odf(bytes, pal) }
            n.endsWith(".pptx") || n.endsWith(".pptm") -> guard(pal) { pptx(bytes, pal) }
            n.endsWith(".odp") || n.endsWith(".fodp") -> guard(pal) { odp(bytes, pal) }
            n.endsWith(".csv") || n.endsWith(".tsv") -> guard(pal) { csv(bytes, pal) }
            n.endsWith(".pdf") -> guard(pal) { pdf(ctx, bytes, pal) }
            n.endsWith(".zip") || n.endsWith(".jar") -> guard(pal) { zipList(ctx, bytes, pal) }
            else -> null
        }
    }

    /** 渲染器内部崩了也要给一张可读的失败页,而不是让调用方看到白屏。 */
    private inline fun guard(pal: Pal, block: () -> String): String =
        runCatching(block).getOrElse { errorHtml("解析失败:${it.message ?: it.javaClass.simpleName}", pal) }

    /** 是否为"不该按文本渲染"的二进制格式(未被 [render] 覆盖的那部分)。 */
    fun isBinary(fileName: String): Boolean {
        val ext = fileName.substringAfterLast('.', "").lowercase()
        return ext in setOf(
            "apk", "apks", "xapk", "exe", "dll", "so", "dylib",
            "rar", "7z", "tar", "gz", "tgz", "bz2", "xz", "class", "db", "sqlite",
            "mp3", "wav", "flac", "aac", "ogg", "m4a", "mp4", "mkv", "mov", "avi", "webm",
            "psd", "ttf", "otf", "woff", "woff2", "bin", "dat", "pak",
        )
    }

    // ---------------------------------------------------------------- 主题配色

    /** 预览页配色:一次取齐,渲染器不再各自写死颜色。
     *  浅色档 = 编辑式令牌(PaEditorial:冷白画布/深墨正文/细线/主色 #386BE7),
     *  与逐页迁移后的 chrome 同一套体系(muted/faint 已同步 2026-09-30 对比度
     *  终审加深值 #5F6C7C/#828B95);深色档维持原深色盘(内容区跟显示模式,
     *  黑客终端经布尔 dark 归入本档 —— PaHacker 契约:黑客盘 IsDark=true)。 */
    private class Pal(dark: Boolean) {
        val dark = dark
        val bg = if (dark) "#0b0b0d" else "#ffffff"
        val text = if (dark) "#e6e6e6" else "#19222d"
        val title = if (dark) "#ffffff" else "#19222d"
        val muted = if (dark) "#9a9aa2" else "#5f6c7c"
        val faint = if (dark) "#7a7a82" else "#828b95"
        val border = if (dark) "#2a2a30" else "#e3e7eb"
        val thead = if (dark) "#17171a" else "#edf0f2"
        val zebra = if (dark) "#121215" else "#fafbfc"
        val code = if (dark) "#17171a" else "#f2f4f6"
        val quote = if (dark) "#17171a" else "#e8efff"
        val quoteBar = if (dark) "#4c8dff" else "#386be7"
        val accent = if (dark) "#4c8dff" else "#386be7"
        val slide = if (dark) "#121215" else "#fafbfc"
        val err = if (dark) "#ff6b6b" else "#c94f4d"
    }

    // ---------------------------------------------------------------- XLSX

    private fun xlsx(bytes: ByteArray, pal: Pal): String {
        val entries = readZip(bytes) ?: return errorHtml("无法解压这个 xlsx 文件(可能已损坏)", pal)
        val shared = parseSharedStrings(entries["xl/sharedStrings.xml"])
        val styles = parseXlsxStyles(entries)
        val (sheetNames, sheetFiles) = resolveSheets(entries)

        val sb = StringBuilder()
        sb.append(header("表格预览", pal))
        var rendered = 0
        for (i in sheetFiles.indices) {
            if (rendered >= MAX_SHEETS) break
            val xml = entries["xl/${sheetFiles[i]}"]
                ?: entries.keys.firstOrNull { it.endsWith(sheetFiles[i]) }?.let { entries[it] }
                ?: continue
            val table = sheetToHtml(xml, shared, styles, sheetNames.getOrNull(i), pal) ?: continue
            sb.append(table)
            rendered++
        }
        if (rendered == 0) return errorHtml("表格里没有可显示的数据", pal)
        if (sheetFiles.size > rendered) sb.append(note("只显示前 $rendered 个工作表", pal))
        return sb.toString()
    }

    // ---- 样式解析:填充色 / 字体色 / 加粗 / 数字格式 ----

    /** 一个单元格样式:背景、前景、加粗、数字格式**代码**(已从格式号解出,可空)。 */
    private class XStyle(val bg: String?, val fg: String?, val bold: Boolean, val numFmt: String?)

    /**
     * 解析 styles.xml + theme1.xml。
     * 颜色三种写法都支持:rgb(ARGB)/theme(引用主题色,Excel 会把 dk1/lt1 对调)/indexed(旧调色板)。
     */
    private fun parseXlsxStyles(entries: Map<String, ByteArray>): List<XStyle> {
        val theme = parseTheme(entries["xl/theme/theme1.xml"])
        val fills = ArrayList<String?>()
        val fonts = ArrayList<Pair<String?, Boolean>>() // (颜色, 加粗)
        val numFmts = HashMap<Int, String>()
        val xfs = ArrayList<IntArray>() // [fillId, fontId, numFmtId]

        val xml = entries["xl/styles.xml"] ?: return emptyList()
        runCatching {
            val p = parser(xml)
            var section = ""          // fills / fonts / cellXfs / numFmts
            var idx = -1
            var inPattern = false
            var patternSolid = false
            var fg: String? = null
            var fontColor: String? = null
            var fontBold = false
            while (p.next() != XmlPullParser.END_DOCUMENT) {
                if (p.eventType == XmlPullParser.START_TAG) {
                    when (ln(p)) {
                        "fills" -> { section = "fills"; idx = -1 }
                        "fonts" -> { section = "fonts"; idx = -1 }
                        "cellXfs" -> { section = "cellXfs"; idx = -1 }
                        "numFmts" -> section = "numFmts"
                        "numFmt" -> if (section == "numFmts") {
                            val id = p.getAttributeValue(null, "numFmtId")?.toIntOrNull()
                            val code = p.getAttributeValue(null, "formatCode")
                            if (id != null && code != null) numFmts[id] = code
                        }
                        "fill" -> if (section == "fills") { idx++; inPattern = false; patternSolid = false; fg = null }
                        "patternFill" -> if (section == "fills") {
                            inPattern = true
                            patternSolid = (p.getAttributeValue(null, "patternType") == "solid")
                        }
                        "fgColor" -> if (inPattern && section == "fills") {
                            fg = readColor(p, theme)
                        }
                        "font" -> if (section == "fonts") { idx++; fontColor = null; fontBold = false }
                        "b" -> if (section == "fonts") {
                            fontBold = p.getAttributeValue(null, "val") != "0"
                        }
                        "color" -> if (section == "fonts") fontColor = readColor(p, theme)
                        "xf" -> if (section == "cellXfs") {
                            val fillId = p.getAttributeValue(null, "fillId")?.toIntOrNull() ?: 0
                            val fontId = p.getAttributeValue(null, "fontId")?.toIntOrNull() ?: 0
                            val fmtId = p.getAttributeValue(null, "numFmtId")?.toIntOrNull() ?: 0
                            xfs.add(intArrayOf(fillId, fontId, fmtId))
                        }
                    }
                } else if (p.eventType == XmlPullParser.END_TAG) {
                    when (ln(p)) {
                        "fill" -> if (section == "fills") {
                            fills.add(if (patternSolid) fg else null)
                        }
                        "font" -> if (section == "fonts") fonts.add(fontColor to fontBold)
                    }
                }
            }
        }

        return xfs.map { (fillId, fontId, fmtId) ->
            XStyle(
                bg = fills.getOrNull(fillId),
                fg = fonts.getOrNull(fontId)?.first,
                bold = fonts.getOrNull(fontId)?.second ?: false,
                numFmt = resolveNumFmt(fmtId, numFmts),
            )
        }
    }

    /** 数字格式号 → 格式代码:优先文件里的自定义 numFmts,其次内建表;0(General)无格式。 */
    private fun resolveNumFmt(id: Int, custom: Map<Int, String>): String? {
        if (id == 0) return null
        return custom[id] ?: BUILTIN_FMT[id]
    }

    /** 单元格引用 "B7" → 列下标 1。xlsx 会跳过空列,靠它把稀疏行补到正确位置。 */
    private fun colOf(ref: String): Int {
        var c = 0
        var letters = 0
        for (ch in ref) {
            val u = ch.uppercaseChar()
            if (u in 'A'..'Z') {
                c = c * 26 + (u - 'A' + 1)
                letters++
            } else {
                break
            }
        }
        return if (letters == 0) -1 else c - 1
    }

    /** 读一个颜色元素(rgb / theme+tint / indexed)。 */
    private fun readColor(p: XmlPullParser, theme: List<String>): String? {
        p.getAttributeValue(null, "rgb")?.let { return normalizeColor(it) }
        val themeIdx = p.getAttributeValue(null, "theme")?.toIntOrNull()
        if (themeIdx != null && themeIdx in theme.indices) {
            var c = theme[themeIdx]
            p.getAttributeValue(null, "tint")?.toDoubleOrNull()?.let { t -> c = applyTint(c, t) }
            return c
        }
        val indexed = p.getAttributeValue(null, "indexed")?.toIntOrNull()
        if (indexed != null) {
            if (indexed == 64 || indexed == 65) return null // 系统前景/背景:自动色
            return INDEXED_COLORS.getOrNull(indexed)
        }
        return null
    }

    /** ARGB/AARRGGBB → #RRGGBB(黑或接近黑时保持可读)。 */
    private fun normalizeColor(v: String): String? {
        val hex = v.removePrefix("#").uppercase()
        return when (hex.length) {
            8 -> "#" + hex.substring(2)
            6 -> "#$hex"
            else -> null
        }
    }

    /** Excel tint:正数变亮、负数变暗(按通道线性近似,足够预览用)。 */
    private fun applyTint(hex: String, tint: Double): String {
        if (tint == 0.0) return hex
        val base = hex.removePrefix("#")
        if (base.length != 6) return hex
        val sb = StringBuilder("#")
        for (i in 0 until 3) {
            val v = base.substring(i * 2, i * 2 + 2).toInt(16)
            val nv = if (tint > 0) v + (255 - v) * tint else v * (1 + tint)
            sb.append("%02X".format(nv.toInt().coerceIn(0, 255)))
        }
        return sb.toString()
    }

    /**
     * theme1.xml 的 clrScheme。注意 **Excel 对 theme 索引做了 dk1/lt1 对调**:
     * theme="0" 实际是 lt1(白),theme="1" 是 dk1(黑)。
     */
    private fun parseTheme(xml: ByteArray?): List<String> {
        if (xml == null) return emptyList()
        val scheme = ArrayList<String>(12)
        runCatching {
            val p = parser(xml)
            var current: String? = null
            while (p.next() != XmlPullParser.END_DOCUMENT) {
                if (p.eventType == XmlPullParser.START_TAG) {
                    when (ln(p)) {
                        "dk1", "lt1", "dk2", "lt2", "accent1", "accent2", "accent3",
                        "accent4", "accent5", "accent6", "hlink", "folHlink" -> current = ln(p)
                        "srgbClr" -> current?.let {
                            setScheme(scheme, it, normalizeColor(p.getAttributeValue(null, "val") ?: "") ?: "#000000")
                        }
                        "sysClr" -> current?.let {
                            val last = p.getAttributeValue(null, "lastClr") ?: return@let
                            setScheme(scheme, it, normalizeColor(last) ?: "#000000")
                        }
                    }
                }
            }
        }
        // 还原成"theme 索引"顺序:0=lt1, 1=dk1, 2=lt2, 3=dk2, 4..9=accent1..6
        fun at(i: Int) = scheme.getOrNull(i) ?: "#000000"
        return listOf(at(1), at(0), at(3), at(2), at(4), at(5), at(6), at(7), at(8), at(9), at(10), at(11))
    }

    private fun orderIndex(name: String): Int = when (name) {
        "dk1" -> 0; "lt1" -> 1; "dk2" -> 2; "lt2" -> 3
        "accent1" -> 4; "accent2" -> 5; "accent3" -> 6; "accent4" -> 7
        "accent5" -> 8; "accent6" -> 9; "hlink" -> 10; "folHlink" -> 11
        else -> 12
    }

    private fun setScheme(list: ArrayList<String>, name: String, value: String) {
        val i = orderIndex(name)
        while (list.size <= i) list.add("#000000")
        list[i] = value
    }

    /** 旧版 indexed 调色板(0..63),MS-OOXML 规范里的固定表。 */
    private val INDEXED_COLORS = listOf(
        "#000000", "#FFFFFF", "#FF0000", "#00FF00", "#0000FF", "#FFFF00", "#FF00FF", "#00FFFF",
        "#000000", "#FFFFFF", "#FF0000", "#00FF00", "#0000FF", "#FFFF00", "#FF00FF", "#00FFFF",
        "#800000", "#008000", "#000080", "#808000", "#800080", "#008080", "#C0C0C0", "#808080",
        "#9999FF", "#993366", "#FFFFCC", "#CCFFFF", "#660066", "#FF8080", "#0066CC", "#CCCCFF",
        "#000080", "#FF00FF", "#FFFF00", "#00FFFF", "#800080", "#800000", "#008080", "#0000FF",
        "#00CCFF", "#CCFFFF", "#CCFFCC", "#FFFF99", "#99CCFF", "#FF99CC", "#CC99FF", "#FFCC99",
        "#3366FF", "#33CCCC", "#99CC00", "#FFCC00", "#FF9900", "#FF6600", "#666699", "#969696",
        "#003366", "#339966", "#003300", "#333300", "#993300", "#993366", "#333399", "#333333",
    )

    /** 解析 workbook.xml + 其 rels,得到(有序表名, 有序 sheet 文件路径)。 */
    private fun resolveSheets(entries: Map<String, ByteArray>): Pair<List<String>, List<String>> {
        val rels = HashMap<String, String>()
        entries["xl/_rels/workbook.xml.rels"]?.let { rb ->
            runCatching {
                val p = parser(rb)
                while (p.next() != XmlPullParser.END_DOCUMENT) {
                    if (p.eventType == XmlPullParser.START_TAG && ln(p) == "Relationship") {
                        val id = p.getAttributeValue(null, "Id") ?: continue
                        var target = p.getAttributeValue(null, "Target") ?: continue
                        if (target.startsWith("/")) target = target.removePrefix("/")
                        else target = "xl/" + target.removePrefix("./")
                        rels[id] = target
                    }
                }
            }
        }
        val names = ArrayList<String>()
        val files = ArrayList<String>()
        runCatching {
            val xml = entries["xl/workbook.xml"] ?: return names to files
            val p = parser(xml)
            while (p.next() != XmlPullParser.END_DOCUMENT) {
                if (p.eventType == XmlPullParser.START_TAG && ln(p) == "sheet") {
                    names.add(p.getAttributeValue(null, "name") ?: "Sheet${names.size + 1}")
                    val rid = p.getAttributeValue(null, "r:id")
                        ?: p.getAttributeValue(
                            "http://schemas.openxmlformats.org/officeDocument/2006/relationships",
                            "id",
                        )
                    val target = rid?.let { rels[it] }
                    if (target != null) files.add(target.removePrefix("xl/"))
                }
            }
        }
        if (files.isEmpty()) {
            val sorted = entries.keys
                .filter { Regex("^xl/worksheets/sheet\\d+\\.xml$").matches(it) }
                .sortedBy { it.filter(Char::isDigit).toIntOrNull() ?: 0 }
            files.addAll(sorted.map { it.removePrefix("xl/") })
            while (names.size < files.size) names.add("Sheet${names.size + 1}")
        }
        return names to files
    }

    private fun parseSharedStrings(xml: ByteArray?): List<String> {
        if (xml == null) return emptyList()
        val out = ArrayList<String>()
        runCatching {
            val p = parser(xml)
            var inSi = false
            var inT = false
            val cur = StringBuilder()
            while (p.next() != XmlPullParser.END_DOCUMENT) {
                when (p.eventType) {
                    XmlPullParser.START_TAG -> when (ln(p)) {
                        "si" -> { inSi = true; cur.setLength(0) }
                        "t" -> if (inSi) inT = true
                    }
                    XmlPullParser.TEXT -> if (inT) cur.append(p.text ?: "")
                    XmlPullParser.END_TAG -> when (ln(p)) {
                        "t" -> inT = false
                        "si" -> { inSi = false; out.add(cur.toString()) }
                    }
                }
            }
        }
        return out
    }

    private fun sheetToHtml(
        xml: ByteArray,
        shared: List<String>,
        styles: List<XStyle>,
        sheetName: String?,
        pal: Pal,
    ): String? {
        val sb = StringBuilder()
        var rows = 0
        var truncated = false
        var hasData = false
        runCatching {
            val p = parser(xml)
            var inCell = false
            var inV = false
            var inInlineT = false
            var cellType = ""
            var cellStyle = -1
            val cellText = StringBuilder()
            var rowCells = ArrayList<Pair<String, String?>>() // (文本, 附加样式 css)
            while (p.next() != XmlPullParser.END_DOCUMENT) {
                when (p.eventType) {
                    XmlPullParser.START_TAG -> when (ln(p)) {
                        "row" -> rowCells = ArrayList()
                        "c" -> {
                            inCell = true
                            cellType = p.getAttributeValue(null, "t") ?: ""
                            cellStyle = p.getAttributeValue(null, "s")?.toIntOrNull() ?: -1
                            val ref = p.getAttributeValue(null, "r")
                            val colIdx = ref?.let { colOf(it) }
                            if (colIdx != null && colIdx >= 0) {
                                while (rowCells.size < colIdx && rowCells.size < MAX_COLS) {
                                    rowCells.add(Pair("", null))
                                }
                            }
                            cellText.setLength(0)
                        }
                        "v" -> if (inCell) inV = true
                        "t" -> if (inCell) inInlineT = true
                    }
                    XmlPullParser.TEXT -> if (inV || inInlineT) cellText.append(p.text ?: "")
                    XmlPullParser.END_TAG -> when (ln(p)) {
                        "v" -> inV = false
                        "t" -> inInlineT = false
                        "c" -> {
                            inCell = false
                            val raw = cellText.toString()
                            val st = styles.getOrNull(cellStyle)
                            val display = when (cellType) {
                                "s" -> shared.getOrNull(raw.toIntOrNull() ?: -1) ?: ""
                                "b" -> if (raw == "1") "TRUE" else "FALSE"
                                "e" -> "#ERR"
                                "str", "inlineStr" -> raw
                                else -> {
                                    val d = raw.toDoubleOrNull()
                                    if (d != null && st?.numFmt != null) fmtExcel(d, st.numFmt)
                                    else raw
                                }
                            }
                            rowCells.add(display to cellCss(st, pal.dark))
                        }
                        "row" -> {
                            if (rowCells.any { it.first.isNotBlank() }) {
                                hasData = true
                                if (rows >= MAX_ROWS) {
                                    truncated = true
                                } else {
                                    val tag = if (rows == 0) "th" else "td"
                                    sb.append("<tr>")
                                    rowCells.take(MAX_COLS).forEach { (c, css) ->
                                        val numCls = if (isNumeric(c)) " class=\"num\"" else ""
                                        sb.append("<").append(tag).append(numCls)
                                            .append(if (css != null) " style=\"$css\"" else "")
                                            .append(">").esc(c).append("</").append(tag).append(">")
                                    }
                                    sb.append("</tr>")
                                    rows++
                                }
                            }
                        }
                    }
                }
            }
        }
        if (!hasData) return null
        val head = StringBuilder("""<div class="doc"><div class="doctitle">""")
        head.esc(sheetName ?: "Sheet").append("</div><div class=\"tw\"><table>")
        head.append(sb).append("</table></div>")
        if (truncated) head.append(note("数据较多,只显示前 $MAX_ROWS 行", pal))
        head.append("</div>")
        return head.toString()
    }

    /** 单元格内联样式:文件里的填充/字色/加粗,原样带出来。 */
    private fun cellCss(st: XStyle?, dark: Boolean): String? {
        if (st == null) return null
        val parts = ArrayList<String>()
        var bg = st.bg
        var fg = st.fg
        if (dark) {
            // 【深色模式可读性】页面底是黑的,但单元格颜色来自 Excel 原文件(按白底设计):
            // · 接近白的填充换成卡片色,避免整页刺眼;
            // · 深色字(工程表头常用的深蓝 #1F4E79 等)一旦失去浅色底衬托
            //   (没填充,或填充本来就是白底被换掉了)就提亮,
            //   否则"深字打在黑底上"根本看不清 —— 这正是外部查看器做得对、我们之前做错的地方。
            val bgWasLight = bg != null && lum(bg) > 0.82
            if (bgWasLight) bg = "#1d1d21"
            if ((bg == null || bgWasLight) && fg != null && lum(fg) < 0.38) fg = lightenForDark(fg)
        }
        bg?.let { if (it != "#FFFFFF" || st.fg != null) parts.add("background:$it") }
        fg?.let { parts.add("color:$it") }
        if (st.bold) parts.add("font-weight:700")
        return if (parts.isEmpty()) null else parts.joinToString(";")
    }

    /** sRGB 相对亮度(W3C 公式),用于判断颜色深浅。 */
    private fun lum(hex: String): Double {
        val h = hex.removePrefix("#")
        if (h.length != 6) return 1.0
        fun ch(i: Int): Double {
            // 第 i 个字节是第 i 对十六进制位 —— 索引要乘 2,别按字符位取
            val v = h.substring(i * 2, i * 2 + 2).toInt(16) / 255.0
            return if (v <= 0.03928) v / 12.92 else Math.pow((v + 0.055) / 1.055, 2.4)
        }
        return 0.2126 * ch(0) + 0.7152 * ch(1) + 0.0722 * ch(2)
    }

    /** 与白色按 68% 混合,把深色文字提到黑底上可读的亮度。 */
    private fun lightenForDark(hex: String): String {
        val h = hex.removePrefix("#")
        if (h.length != 6) return hex
        fun mix(v: Int): Int = Math.round(v + (255 - v) * 0.68).toInt().coerceIn(0, 255)
        val r = mix(h.substring(0, 2).toInt(16))
        val g = mix(h.substring(2, 4).toInt(16))
        val b = mix(h.substring(4, 6).toInt(16))
        return String.format("#%02X%02X%02X", r, g, b)
    }

    private val NUM_RE = Regex("^-?\\d{1,3}(,\\d{3})*(\\.\\d+)?%?$|^-?\\d+(\\.\\d+)?%?$")

    private fun isNumeric(s: String): Boolean = s.matches(NUM_RE)

    /** 内建数字格式号 → 格式代码(MS-OOXML 18.8.30 的常用子集)。 */
    private val BUILTIN_FMT = mapOf(
        1 to "0", 2 to "0.00", 3 to "#,##0", 4 to "#,##0.00",
        9 to "0%", 10 to "0.00%", 11 to "0.00E+00",
        14 to "yyyy-mm-dd", 15 to "yyyy-mm-dd", 16 to "yyyy-mm-dd HH:mm", 17 to "HH:mm", 18 to "HH:mm:ss",
        20 to "HH:mm", 21 to "HH:mm:ss", 22 to "yyyy-mm-dd HH:mm",
        37 to "#,##0;(#,##0)", 41 to "#,##0;(#,##0)",
    )

    /** 按 Excel 数字格式把数值渲染成用户熟悉的文本(2,400 / 2.60% / 日期)。 */
    private fun fmtExcel(v: Double, formatCode: String?): String {
        if (v.isNaN() || v.isInfinite()) return ""
        val code = formatCode?.takeIf { it.isNotBlank() } ?: return plainExcel(v)
        // 日期/时间
        if (listOf('y', 'd').any { code.lowercase().contains(it) } ||
            (code.lowercase().contains('h') && code.lowercase().contains('m'))
        ) {
            val ms = ((v - 25569.0) * 86400000.0).toLong()
            val pat = when {
                code.lowercase().contains("hh:mm:ss") || code.lowercase().contains("mm:ss") -> "yyyy-MM-dd HH:mm:ss"
                code.lowercase().contains("hh:mm") -> if (v == Math.floor(v)) "yyyy-MM-dd" else "yyyy-MM-dd HH:mm"
                else -> "yyyy-MM-dd"
            }
            return runCatching {
                java.text.SimpleDateFormat(pat, java.util.Locale.US).format(java.util.Date(ms))
            }.getOrDefault(plainExcel(v))
        }
        // 百分比
        val isPercent = code.contains('%')
        var value = if (isPercent) v * 100 else v
        // 小数位:取小数点后连续 0 的个数
        val dot = code.indexOf('.')
        var decimals = 0
        if (dot >= 0) {
            var i = dot + 1
            while (i < code.length && (code[i] == '0' || code[i] == '#')) {
                if (code[i] == '0') decimals++
                i++
            }
        }
        val thousands = code.substringBefore('.').contains(',')
        val negativeParen = code.contains('(')
        val sb = StringBuilder()
        if (value < 0) {
            if (negativeParen) sb.append("(") else sb.append("-")
            value = -value
        }
        val abs = if (decimals > 0) {
            String.format(java.util.Locale.US, "%,.${decimals}f", value)
        } else {
            String.format(java.util.Locale.US, if (thousands) "%,d" else "%d", Math.round(value))
        }
        sb.append(abs)
        if (negativeParen && v < 0) sb.append(")")
        if (isPercent) sb.append("%")
        return sb.toString()
    }

    private fun plainExcel(v: Double): String {
        if (v == Math.floor(v) && Math.abs(v) < 1e15) return v.toLong().toString()
        return java.math.BigDecimal(v)
            .setScale(6, java.math.RoundingMode.HALF_UP)
            .stripTrailingZeros()
            .toPlainString()
    }

    // ---------------------------------------------------------------- XLS(旧版)

    private fun xls(bytes: ByteArray, pal: Pal): String {
        val sheets = XlsLegacy.parse(bytes, MAX_ROWS)
            ?: return errorHtml("无法解析这个 .xls 文件(可能已损坏,或不是 Excel 97-2003 格式)", pal)
        if (sheets.isEmpty()) return errorHtml("表格里没有可显示的数据", pal)
        val sb = StringBuilder(header("表格预览", pal))
        var count = 0
        for (s in sheets) {
            if (count >= MAX_SHEETS) break
            if (s.rows.isEmpty()) continue
            sb.append(htmlTable(s.name, s.rows, pal))
            count++
        }
        if (count == 0) return errorHtml("表格里没有可显示的数据", pal)
        if (sheets.size > count) sb.append(note("只显示前 $count 个工作表", pal))
        sb.append(note("旧版 .xls 只能读到单元格的值与日期;公式显示为缓存结果,不含图表与条件格式。", pal))
        return sb.toString()
    }

    // ---------------------------------------------------------------- ZIP / JAR

    /** 压缩包:列出条目与大小 —— 比一张"不支持预览"的空白页有用得多。 */
    private fun zipList(ctx: Context, bytes: ByteArray, pal: Pal): String {
        val items = ArrayList<Triple<String, Long, Long>>()
        runCatching {
            // 【为什么落盘再用 ZipFile】ZipInputStream 按本地文件头逐条读,而 APK 这类
            // 构建产物把大小写在数据描述符里,逐条流式读会得到 0 长度甚至空名条目
            // (实测:228 条的 APK 读出一串空白行)。ZipFile 走中央目录,名与大小都可靠。
            val tmp = java.io.File.createTempFile(PaPaths.TMP_ZIPLIST_PREFIX, ".zip", ctx.cacheDir)
            try {
                tmp.writeBytes(bytes)
                java.util.zip.ZipFile(tmp).use { zf ->
                    val it = zf.entries()
                    while (it.hasMoreElements() && items.size < MAX_ZIP_ENTRIES) {
                        val e = it.nextElement()
                        if (e.isDirectory || e.name.isBlank()) continue
                        items.add(Triple(e.name, e.size, e.compressedSize))
                    }
                }
            } finally {
                tmp.delete()
            }
        }
        if (items.isEmpty()) return errorHtml("读不出压缩包里的条目(可能已损坏)", pal)
        val sb = StringBuilder(header("压缩包内容", pal))
        sb.append("""<div class="doc"><div class="tw"><table>""")
        sb.append("<tr><th>文件名</th><th>原始大小</th><th>压缩后</th></tr>")
        items.sortedBy { it.first }.forEach { (name, size, csize) ->
            sb.append("<tr><td>").esc(name).append("</td><td class=\"num\">")
                .append(if (size >= 0) human(size) else "—")
                .append("</td><td class=\"num\">")
                .append(if (csize >= 0) human(csize) else "—")
                .append("</td></tr>")
        }
        sb.append("</table></div>")
        if (items.size >= MAX_ZIP_ENTRIES) sb.append(note("条目过多,只列出前 $MAX_ZIP_ENTRIES 项", pal))
        sb.append(note("压缩包只列出内容清单,不支持在 App 内解压;需要解压请下载后用文件管理器。", pal))
        sb.append("</div>")
        return sb.toString()
    }

    private fun human(b: Long): String = when {
        b >= 1024L * 1024 * 1024 -> String.format(java.util.Locale.US, "%.2fGB", b / 1073741824f)
        b >= 1024 * 1024 -> String.format(java.util.Locale.US, "%.1fMB", b / 1048576f)
        b >= 1024 -> String.format(java.util.Locale.US, "%.0fKB", b / 1024f)
        else -> "${b}B"
    }

    // ---------------------------------------------------------------- ODF(ods/odt/odp)

    private fun odp(bytes: ByteArray, pal: Pal): String = odf(bytes, pal)

    /** OpenDocument 三类文档共用一个解析器:content.xml 的结构差异只体现在包装标签上。 */
    private fun odf(bytes: ByteArray, pal: Pal): String {
        val entries = readZip(bytes) ?: return errorHtml("无法解压这个 OpenDocument 文件(可能已损坏)", pal)
        val xml = entries["content.xml"] ?: return errorHtml("不是有效的 OpenDocument 文件", pal)
        val p = parser(xml)

        val sb = StringBuilder()
        val sbTail = StringBuilder()
        var mode = ""            // text | spreadsheet | presentation
        var title = ""
        val para = StringBuilder()
        var inPara = false
        var heading = 0
        var cellBuf = StringBuilder()
        var inCell = false
        var cellRepeat = 1
        var slideIndex = 0
        var rows = 0
        var truncated = false

        /** 段落结束时把它放进正确的容器(单元格内 / 幻灯片内 / 正文)。 */
        fun flushPara() {
            if (!inPara) return
            val body = para.toString()
            para.setLength(0)
            if (inCell) { cellBuf.append(body); inPara = false; return }
            when (mode) {
                "presentation" -> {
                    if (body.isNotBlank()) {
                        if (heading > 0 && title.isBlank()) {
                            title = body
                            sbTail.append("<div class=\"slidetitle\">").append(body).append("</div>")
                        } else {
                            sbTail.append("<div class=\"bullet\">").append(body).append("</div>")
                        }
                    }
                }
                "spreadsheet" -> Unit
                else -> {
                    if (body.isBlank()) sbTail.append("<p class=\"blank\">&nbsp;</p>")
                    else if (heading > 0) sbTail.append("<h3>").append(body).append("</h3>")
                    else sbTail.append("<p>").append(body).append("</p>")
                }
            }
            inPara = false
        }

        var guardCount = 0
        runCatching {
            while (p.next() != XmlPullParser.END_DOCUMENT) {
                if (guardCount++ > 4_000_000) break
                val ev = p.eventType
                // 【不能提前取 p.name】TEXT/其它事件的 name 是 null,取了会 NPE
                val name = if (ev == XmlPullParser.START_TAG || ev == XmlPullParser.END_TAG) ln(p) else ""
                when (ev) {
                    XmlPullParser.START_TAG -> when (name) {
                        "text", "spreadsheet", "presentation" -> if (mode.isEmpty()) mode = name
                        "h" -> {
                            flushPara()
                            inPara = true
                            heading = attr(p, "outline-level", "text")?.toIntOrNull() ?: 1
                        }
                        "p" -> { flushPara(); inPara = true; heading = 0 }
                        "line-break" -> if (inPara) para.append("<br>")
                        "tab" -> if (inPara) para.append("&nbsp;&nbsp;&nbsp;&nbsp;")
                        "table" -> {
                            rows = 0
                            sbTail.append("<div class=\"tw\"><table>")
                            attr(p, "name", "table")?.let {
                                if (it.isNotBlank()) sbTail.append("<caption>").esc(it).append("</caption>")
                            }
                        }
                        "table-row", "table-header-rows" -> if (rows < MAX_ROWS) sbTail.append("<tr>")
                        "table-cell", "covered-table-cell" -> {
                            inCell = true
                            cellBuf = StringBuilder()
                            cellRepeat = attr(p, "number-columns-repeated", "table")?.toIntOrNull() ?: 1
                        }
                        "page" -> {
                            flushPara()
                            slideIndex++
                            title = ""
                            sbTail.append("<div class=\"slide\"><div class=\"slideno\">第 ").append(slideIndex).append(" 页</div>")
                        }
                    }
                    XmlPullParser.TEXT -> if (inPara) para.append(esc(p.text ?: ""))
                    XmlPullParser.END_TAG -> when (name) {
                        "p", "h" -> flushPara()
                        "table-cell", "covered-table-cell" -> {
                            flushPara()
                            val tag = if (rows == 0) "th" else "td"
                            val reps = cellRepeat.coerceIn(1, MAX_COLS)
                            if (rows < MAX_ROWS) {
                                repeat(reps) {
                                    sbTail.append("<").append(tag).append(">")
                                        .append(cellBuf).append("</").append(tag).append(">")
                                }
                            } else {
                                truncated = true
                            }
                            inCell = false
                            cellBuf = StringBuilder()
                        }
                        "table-row", "table-header-rows" -> {
                            if (rows < MAX_ROWS) sbTail.append("</tr>") else truncated = true
                            rows++
                        }
                        "table" -> {
                            sbTail.append("</table></div>")
                            if (mode == "spreadsheet") { rows = 0; flushSpreadsheetInto(sb, sbTail) }
                        }
                        "page" -> {
                            flushPara()
                            if (title.isBlank()) title = "第 $slideIndex 页"
                            sbTail.append("</div>")
                        }
                    }
                }
            }
        }

        when (mode) {
            "" -> return errorHtml("这个 OpenDocument 文件里没有可显示的内容", pal)
            "presentation" -> {
                sb.append(header("演示文稿预览", pal))
                sb.append("<div class=\"doc\">").append(sbTail).append("</div>")
            }
            "spreadsheet" -> {
                flushSpreadsheetInto(sb, sbTail)
                if (sb.length == 0) return errorHtml("表格里没有可显示的数据", pal)
                sb.insert(0, header("表格预览", pal))
            }
            else -> {
                sb.append(header("文档预览", pal))
                sb.append("<div class=\"doc\">").append(sbTail).append("</div>")
            }
        }
        if (truncated) sb.append(note("数据较多,只显示前 $MAX_ROWS 行", pal))
        return sb.toString()
    }

    /** 把累积的表格片段搬进总输出,并给每张表加上文档级容器。 */
    private fun flushSpreadsheetInto(sb: StringBuilder, tail: StringBuilder) {
        if (tail.length == 0) return
        sb.append("<div class=\"doc\">").append(tail).append("</div>")
        tail.setLength(0)
    }

    // ---------------------------------------------------------------- DOCX

    private fun docx(bytes: ByteArray, pal: Pal): String {
        val entries = readZip(bytes) ?: return errorHtml("无法解压这个 docx 文件(可能已损坏)", pal)
        val xml = entries["word/document.xml"] ?: return errorHtml("不是有效的 docx 文档", pal)
        val sb = StringBuilder(header("文档预览", pal))
        sb.append("""<div class="doc">""")
        runCatching {
            val p = parser(xml)
            var inParagraph = false
            var inText = false
            val para = StringBuilder()
            var tableMode = false
            var cellMode = false
            while (p.next() != XmlPullParser.END_DOCUMENT) {
                when (p.eventType) {
                    XmlPullParser.START_TAG -> when (ln(p)) {
                        "p" -> { inParagraph = true; para.setLength(0) }
                        "t" -> inText = true
                        "tbl" -> { tableMode = true; sb.append("<div class=\"tw\"><table>") }
                        "tr" -> sb.append("<tr>")
                        "tc" -> { cellMode = true; sb.append("<td>") }
                        "br", "cr" -> if (inParagraph) para.append("<br>")
                    }
                    XmlPullParser.TEXT -> if (inText) para.append(esc(p.text ?: ""))
                    XmlPullParser.END_TAG -> when (ln(p)) {
                        "t" -> inText = false
                        "p" -> {
                            inParagraph = false
                            if (cellMode) {
                                if (para.isNotEmpty()) sb.append(para)
                            } else {
                                if (para.isNotEmpty()) sb.append("<p>").append(para).append("</p>")
                                else if (!tableMode) sb.append("<p class=\"blank\">&nbsp;</p>")
                            }
                            para.setLength(0)
                        }
                        "tc" -> { cellMode = false; sb.append("</td>") }
                        "tr" -> sb.append("</tr>")
                        "tbl" -> { tableMode = false; sb.append("</table></div>") }
                    }
                }
            }
        }
        sb.append("</div>")
        return sb.toString()
    }

    // ---------------------------------------------------------------- RTF

    /** RTF:剥掉控制字/分组,保留可见文本与段落换行。够看内容,不还原字体与颜色。 */
    private fun rtf(bytes: ByteArray, pal: Pal): String {
        val raw = String(bytes, Charsets.ISO_8859_1)
        val sb = StringBuilder(header("RTF 预览", pal))
        sb.append("""<div class="doc">""")
        val out = StringBuilder()
        var i = 0
        var depth = 0
        var skipDepth = -1
        while (i < raw.length) {
            val c = raw[i]
            when (c) {
                '{' -> { depth++; i++ }
                '}' -> { if (skipDepth == depth) skipDepth = -1; depth--; i++ }
                '\\' -> {
                    i++
                    if (i >= raw.length) break
                    val nc = raw[i]
                    when {
                        nc == '\\' || nc == '{' || nc == '}' -> { out.append(nc); i++ }
                        nc == '\'' -> {
                            val hex = raw.substring(i + 1, minOf(i + 3, raw.length))
                            val v = hex.toIntOrNull(16) ?: 0
                            out.append(CP1252_BYTE_TO_CHAR[v and 0xFF])
                            i += 3
                        }
                        nc == '*' -> { skipDepth = depth; i++ }
                        nc.isLetter() -> {
                            var j = i
                            while (j < raw.length && raw[j].isLetter()) j++
                            val word = raw.substring(i, j)
                            var k = j
                            var sign = 1
                            if (k < raw.length && raw[k] == '-') { sign = -1; k++ }
                            var numStart = k
                            while (k < raw.length && raw[k].isDigit()) k++
                            val num = if (k > numStart) raw.substring(numStart, k).toIntOrNull() ?: 0 else 0
                            if (k < raw.length && raw[k] == ' ') k++
                            if (skipDepth < 0) {
                                when (word) {
                                    "par", "line", "pard" -> out.append('\n')
                                    "tab" -> out.append('\t')
                                    "u" -> out.append(((num * sign + 65536) % 65536).toChar())
                                    "page" -> out.append("\n\n")
                                    else -> Unit
                                }
                            }
                            i = k
                        }
                        else -> i++
                    }
                }
                '\r', '\n' -> i++
                else -> { if (skipDepth < 0) out.append(c); i++ }
            }
        }
        sb.append(rtfToHtml(out.toString()))
        sb.append("</div>")
        return sb.toString()
    }

    private fun rtfToHtml(text: String): String {
        val sb = StringBuilder()
        var listOpen = false
        for (line in text.lines()) {
            val t = line.trim()
            when {
                t.isEmpty() -> {
                    if (listOpen) { sb.append("</ul>"); listOpen = false }
                    sb.append("<p class=\"blank\">&nbsp;</p>")
                }
                t.startsWith("·") || t.startsWith("•") || t.startsWith("-") -> {
                    if (!listOpen) { sb.append("<ul>"); listOpen = true }
                    sb.append("<li>").append(esc(t.removePrefix("·").removePrefix("•").removePrefix("-").trim())).append("</li>")
                }
                else -> {
                    if (listOpen) { sb.append("</ul>"); listOpen = false }
                    sb.append("<p>").append(esc(t)).append("</p>")
                }
            }
        }
        if (listOpen) sb.append("</ul>")
        return sb.toString()
    }

    private val CP1252_BYTE_TO_CHAR = CharArray(256) { i ->
        if (i < 0x80) i.toChar()
        else {
            val map = intArrayOf(
                0x20AC, -1, 0x201A, 0x0192, 0x201E, 0x2026, 0x2020, 0x2021, 0x02C6, 0x2030,
                0x0160, 0x2039, 0x0152, -1, 0x017D, -1, -1, 0x2018, 0x2019, 0x201C, 0x201D,
                0x2022, 0x2013, 0x2014, 0x02DC, 0x2122, 0x0161, 0x203A, 0x0153, -1, 0x017E, 0x0178,
            )
            val m = if (i in 0x80..0x9F) map[i - 0x80] else i
            (if (m < 0) i else m).toChar()
        }
    }

    // ---------------------------------------------------------------- DOC / PPT(97-2003 旧版)

    /** 单个旧版 Word 最多渲染多少段。 */
    private const val MAX_DOC_PARAS = 1500

    /** .doc:OfficeLegacy 提取正文文字,按文档页渲染;解析不了给可读错误页(还能下载后交系统打开)。 */
    private fun docLegacy(bytes: ByteArray, pal: Pal): String {
        val paras = OfficeLegacy.docParagraphs(bytes)
            ?: return errorHtml("不是可识别的 Word 97-2003 文档(或已加密),可下载后用系统应用打开", pal)
        val sb = StringBuilder(header("文档预览", pal))
        sb.append("""<div class="doc">""")
        var shown = 0
        for (p in paras) {
            if (shown >= MAX_DOC_PARAS) break
            sb.append("<p>").esc(p).append("</p>")
            shown++
        }
        sb.append("</div>")
        if (paras.size > shown) sb.append(note("正文较长,只显示前 $shown 段", pal))
        sb.append(note("旧版 Word 按纯文字提取,不含图片、表格线与排版。", pal))
        return sb.toString()
    }

    /** .ppt:每页抽「标题 + 要点」,复用 pptx 的幻灯片卡片样式。 */
    private fun pptLegacy(bytes: ByteArray, pal: Pal): String {
        val slides = OfficeLegacy.pptSlides(bytes)
            ?: return errorHtml("不是可识别的 PowerPoint 97-2003 文档(或已加密),可下载后用系统应用打开", pal)
        if (slides.isEmpty()) return errorHtml("幻灯片里没有可提取的文字(可能是纯图片版式)", pal)
        val sb = StringBuilder(header("演示文稿预览", pal))
        sb.append("""<div class="doc">""")
        var shown = 0
        for ((i, s) in slides.withIndex()) {
            if (shown >= MAX_SLIDES) break
            shown++
            sb.append("""<div class="slide"><div class="slideno">第 """).append(i + 1).append(" 页</div>")
            if (s.title.isNotBlank()) sb.append("""<div class="slidetitle">""").esc(s.title).append("</div>")
            if (s.bullets.isNotEmpty()) {
                sb.append("<ul>")
                s.bullets.forEach { sb.append("<li>").esc(it).append("</li>") }
                sb.append("</ul>")
            }
            sb.append("</div>")
        }
        sb.append("</div>")
        if (slides.size > shown) sb.append(note("只显示前 $shown 页", pal))
        sb.append(note("演示文稿按「标题 + 要点」提取文字,不含图片、图表与动画。", pal))
        return sb.toString()
    }

    // ---------------------------------------------------------------- PPTX

    private fun pptx(bytes: ByteArray, pal: Pal): String {
        val entries = readZip(bytes) ?: return errorHtml("无法解压这个 pptx 文件(可能已损坏)", pal)
        val slides = slideOrder(entries)
        if (slides.isEmpty()) return errorHtml("这个演示文稿里没有幻灯片", pal)

        val sb = StringBuilder(header("演示文稿预览", pal))
        sb.append("""<div class="doc">""")
        var n = 0
        var shown = 0
        for (path in slides) {
            n++
            if (shown >= MAX_SLIDES) break
            val xml = entries[path] ?: continue
            val (title, bullets) = slideText(xml)
            if (title.isBlank() && bullets.isEmpty()) continue
            shown++
            sb.append("""<div class="slide"><div class="slideno">第 """).append(n).append(" 页</div>")
            if (title.isNotBlank()) sb.append("""<div class="slidetitle">""").esc(title).append("</div>")
            if (bullets.isNotEmpty()) {
                sb.append("<ul>")
                bullets.forEach { sb.append("<li>").esc(it).append("</li>") }
                sb.append("</ul>")
            }
            sb.append("</div>")
        }
        if (shown == 0) return errorHtml("幻灯片里没有可提取的文字(可能是纯图片版式)", pal)
        sb.append("</div>")
        if (slides.size > shown) sb.append(note("只显示前 $shown 页", pal))
        sb.append(note("演示文稿按「标题 + 要点」提取文字,不含图片、图表与动画。", pal))
        return sb.toString()
    }

    /** 幻灯片顺序:优先按 presentation.xml 的 sldIdLst + rels,失败则按文件名数字排序。 */
    private fun slideOrder(entries: Map<String, ByteArray>): List<String> {
        val rels = HashMap<String, String>()
        entries["ppt/_rels/presentation.xml.rels"]?.let { rb ->
            runCatching {
                val p = parser(rb)
                while (p.next() != XmlPullParser.END_DOCUMENT) {
                    if (p.eventType == XmlPullParser.START_TAG && ln(p) == "Relationship") {
                        val id = p.getAttributeValue(null, "Id") ?: continue
                        val t = p.getAttributeValue(null, "Target") ?: continue
                        rels[id] = if (t.startsWith("/")) t.removePrefix("/").removePrefix("ppt/")
                        else t.removePrefix("./").removePrefix("ppt/").removePrefix("../")
                    }
                }
            }
        }
        val ordered = ArrayList<String>()
        entries["ppt/presentation.xml"]?.let { pres ->
            runCatching {
                val p = parser(pres)
                while (p.next() != XmlPullParser.END_DOCUMENT) {
                    if (p.eventType == XmlPullParser.START_TAG && ln(p) == "sldId") {
                        val rid = p.getAttributeValue(null, "r:id")
                            ?: p.getAttributeValue(
                                "http://schemas.openxmlformats.org/officeDocument/2006/relationships",
                                "id",
                            )
                        rid?.let { rels[it] }?.let { ordered.add("ppt/" + it) }
                    }
                }
            }
        }
        val valid = ordered.filter { entries.containsKey(it) }
        if (valid.isNotEmpty()) return valid
        return entries.keys
            .filter { Regex("^ppt/slides/slide\\d+\\.xml$").matches(it) }
            .sortedBy { it.filter(Char::isDigit).toIntOrNull() ?: 0 }
    }

    /** 从一页幻灯片里抽(标题, 要点列表)。标题靠占位符类型判断,不靠"第一行"。 */
    private fun slideText(xml: ByteArray): Pair<String, List<String>> {
        var title = ""
        val bullets = ArrayList<String>()
        runCatching {
            val p = parser(xml)
            var titleShape = false
            var inText = false
            val para = StringBuilder()
            var pending = false
            while (p.next() != XmlPullParser.END_DOCUMENT) {
                when (p.eventType) {
                    XmlPullParser.START_TAG -> when (ln(p)) {
                        "sp" -> titleShape = false
                        "ph" -> {
                            val t = p.getAttributeValue(null, "type") ?: "body"
                            titleShape = t == "title" || t == "ctrTitle"
                        }
                        "p" -> { para.setLength(0); inText = true; pending = true }
                        "br" -> para.append(' ')
                    }
                    XmlPullParser.TEXT -> if (inText) para.append(p.text ?: "")
                    XmlPullParser.END_TAG -> when (ln(p)) {
                        "p" -> {
                            inText = false
                            val s = para.toString().trim()
                            if (pending) {
                                if (s.isNotBlank()) {
                                    if (titleShape && title.isBlank()) title = s else bullets.add(s)
                                }
                                pending = false
                            }
                        }
                        "sp" -> titleShape = false
                    }
                }
            }
        }
        return title to bullets
    }

    // ---------------------------------------------------------------- CSV / TSV

    private fun csv(bytes: ByteArray, pal: Pal): String {
        val text = String(bytes, Charsets.UTF_8)
        val sep = if (text.contains('\t') && !text.contains(',')) '\t' else ','
        val rows = ArrayList<List<String>>()
        var trunc = false
        text.lineSequence().forEachIndexed { i, line ->
            if (line.isBlank() && i == 0) return@forEachIndexed
            if (rows.size >= MAX_ROWS) { trunc = true; return@forEachIndexed }
            rows.add(splitCsvLine(line, sep).take(MAX_COLS))
        }
        if (rows.isEmpty()) return errorHtml("文件里没有可显示的数据", pal)
        val sb = StringBuilder(header("表格预览", pal))
        sb.append(htmlTable("", rows, pal))
        if (trunc) sb.append(note("数据较多,只显示前 $MAX_ROWS 行", pal))
        return sb.toString()
    }

    private fun splitCsvLine(line: String, sep: Char): List<String> {
        val out = ArrayList<String>()
        val cur = StringBuilder()
        var inQuote = false
        var i = 0
        while (i < line.length) {
            val ch = line[i]
            when {
                inQuote && ch == '"' && i + 1 < line.length && line[i + 1] == '"' -> { cur.append('"'); i++ }
                ch == '"' -> inQuote = !inQuote
                ch == sep && !inQuote -> { out.add(cur.toString()); cur.setLength(0) }
                else -> cur.append(ch)
            }
            i++
        }
        out.add(cur.toString())
        return out
    }

    // ---------------------------------------------------------------- PDF

    /** PDF:用系统自带的 PdfRenderer 逐页转位图,内嵌进 HTML —— 不需要装任何阅读器。 */
    private fun pdf(ctx: Context, bytes: ByteArray, pal: Pal): String {
        if (bytes.size < 16) return errorHtml("文件内容为空", pal)
        val tmp = java.io.File(ctx.cacheDir, "${PaPaths.TMP_PDF_PREFIX}${System.nanoTime()}.pdf")
        var pfd: android.os.ParcelFileDescriptor? = null
        var renderer: android.graphics.pdf.PdfRenderer? = null
        return try {
            tmp.writeBytes(bytes)
            pfd = android.os.ParcelFileDescriptor.open(tmp, android.os.ParcelFileDescriptor.MODE_READ_ONLY)
            val r = android.graphics.pdf.PdfRenderer(pfd)
            renderer = r
            val total = r.pageCount
            if (total <= 0) {
                errorHtml("这个 PDF 没有可显示的页面", pal)
            } else {
                val pages = minOf(total, MAX_PDF_PAGES)
                val sb = StringBuilder(header("PDF 预览", pal))
                sb.append("<div class=\"doc\">")
                for (i in 0 until pages) {
                    val page = r.openPage(i)
                    val w = PDF_TARGET_WIDTH
                    val h = maxOf(1, (page.height.toFloat() / page.width.toFloat() * w).toInt())
                    val bmp = android.graphics.Bitmap.createBitmap(w, h, android.graphics.Bitmap.Config.ARGB_8888)
                    bmp.eraseColor(android.graphics.Color.WHITE)
                    page.render(bmp, null, null, android.graphics.pdf.PdfRenderer.Page.RENDER_MODE_FOR_DISPLAY)
                    page.close()
                    val bos = java.io.ByteArrayOutputStream()
                    bmp.compress(android.graphics.Bitmap.CompressFormat.JPEG, 78, bos)
                    bmp.recycle()
                    sb.append("<div class=\"pg\"><img src=\"data:image/jpeg;base64,")
                    sb.append(android.util.Base64.encodeToString(bos.toByteArray(), android.util.Base64.NO_WRAP))
                    sb.append("\"></div>")
                }
                sb.append("</div>")
                if (total > pages) sb.append(note("共 $total 页,这里只渲染前 $pages 页。下载到手机可用阅读器看完整版。", pal))
                sb.toString()
            }
        } catch (e: Exception) {
            errorHtml("这个 PDF 打不开:${e.message ?: "可能已加密或损坏"}", pal)
        } finally {
            runCatching { renderer?.close() }
            runCatching { pfd?.close() }
            runCatching { tmp.delete() }
        }
    }

    // ---------------------------------------------------------------- Markdown

    /** 极简 Markdown → HTML:标题/加粗/斜体/行内码/代码块/列表/引用/分隔线/链接。 */
    fun markdownHtml(md: String, title: String, dark: Boolean): String {
        val pal = Pal(dark)
        val sb = StringBuilder()
        sb.append(header(title.substringAfterLast('/').substringAfterLast('\\'), pal))
        sb.append("""<div class="doc md">""")
        val lines = md.lines()
        var inCode = false
        var inList = false
        val codeBuf = StringBuilder()
        for (raw in lines) {
            val line = raw.trimEnd()
            if (line.startsWith("```")) {
                if (inCode) {
                    sb.append(codeBlockHtml(codeBuf.toString()))
                    codeBuf.setLength(0)
                }
                inCode = !inCode
                continue
            }
            if (inCode) {
                codeBuf.append(raw).append('\n')
                continue
            }
            val listMatch = Regex("^(\\s*)[-*+]\\s+(.*)").matchEntire(line)
            val heading = Regex("^(#{1,6})\\s+(.*)").matchEntire(line)
            when {
                line.isBlank() -> if (inList) { sb.append("</ul>"); inList = false }
                heading != null -> {
                    if (inList) { sb.append("</ul>"); inList = false }
                    val level = heading.groupValues[1].length
                    sb.append("<h").append(level).append(">")
                        .append(inline(heading.groupValues[2]))
                        .append("</h").append(level).append(">")
                }
                listMatch != null -> {
                    if (!inList) { sb.append("<ul>"); inList = true }
                    sb.append("<li>").append(inline(listMatch.groupValues[2])).append("</li>")
                }
                line.startsWith(">") ->
                    sb.append("<blockquote>").append(inline(line.removePrefix(">").trim())).append("</blockquote>")
                Regex("^(-{3,}|\\*{3,})$").matches(line) -> sb.append("<hr>")
                else -> {
                    if (inList) { sb.append("</ul>"); inList = false }
                    sb.append("<p>").append(inline(line)).append("</p>")
                }
            }
        }
        if (inList) sb.append("</ul>")
        if (inCode && codeBuf.isNotEmpty()) sb.append(codeBlockHtml(codeBuf.toString()))
        sb.append("</div>")
        return sb.toString()
    }

    /**
     * 代码块 → 带行号的 `<pre>`(纯函数,可单测)。
     * 等宽字体与横向滚动由全局 CSS 的 pre 规则保证;行号用 CSS counter 画在
     * 行首伪元素上,`user-select:none` —— 复制代码时不会把行号一起带走。
     */
    internal fun codeBlockHtml(code: String): String {
        val sb = StringBuilder("<pre>")
        for (line in code.split("\n")) {
            sb.append("<span class=\"l\">").append(esc(line)).append("</span>")
        }
        sb.append("</pre>")
        return sb.toString()
    }

    private fun inline(s: String): String {
        var t = esc(s)
        t = Regex("`([^`]+)`").replace(t) { m -> "<code>${m.groupValues[1]}</code>" }
        t = Regex("\\*\\*([^*]+)\\*\\*").replace(t) { m -> "<b>${m.groupValues[1]}</b>" }
        t = Regex("\\*([^*]+)\\*").replace(t) { m -> "<i>${m.groupValues[1]}</i>" }
        t = Regex("\\[([^]]+)]\\(([^)]+)\\)").replace(t) { m ->
            "<a href=\"${m.groupValues[2]}\">${m.groupValues[1]}</a>"
        }
        return t
    }

    // ---------------------------------------------------------------- 公共

    /** 二维文本 → 表格 HTML(首行当表头)。 */
    private fun htmlTable(name: String, rows: List<List<String>>, pal: Pal): String {
        val sb = StringBuilder("""<div class="doc">""")
        if (name.isNotBlank()) sb.append("""<div class="doctitle">""").esc(name).append("</div>")
        sb.append("<div class=\"tw\"><table>")
        rows.forEachIndexed { i, row ->
            val tag = if (i == 0) "th" else "td"
            sb.append("<tr>")
            row.take(MAX_COLS).forEach { c ->
                val numCls = if (isNumeric(c)) " class=\"num\"" else ""
                sb.append("<").append(tag).append(numCls).append(">")
                    .esc(c).append("</").append(tag).append(">")
            }
            sb.append("</tr>")
        }
        sb.append("</table></div></div>")
        return sb.toString()
    }

    private fun readZip(bytes: ByteArray): Map<String, ByteArray>? = runCatching {
        val out = HashMap<String, ByteArray>()
        ZipInputStream(ByteArrayInputStream(bytes)).use { zis ->
            var e = zis.nextEntry
            while (e != null && out.size < 400) {
                if (!e.isDirectory && e.size <= 64L * 1024 * 1024) out[e.name] = zis.readBytes()
                zis.closeEntry()
                e = zis.nextEntry
            }
        }
        out
    }.getOrNull()

    /** 标签的本地名(去掉命名空间前缀):官方 Excel 写裸名,不少生成库写 x:row / w:t。 */
    private fun ln(p: XmlPullParser): String = p.name.substringAfterLast(':')

    /** 读属性:命名空间关闭时前缀会保留在属性名里,两种写法都试。 */
    private fun attr(p: XmlPullParser, name: String, vararg prefixes: String): String? {
        p.getAttributeValue(null, name)?.let { return it }
        for (pfx in prefixes) p.getAttributeValue(null, "$pfx:$name")?.let { return it }
        return null
    }

    private fun parser(xml: ByteArray): XmlPullParser {
        val p = Xml.newPullParser()
        p.setFeature(XmlPullParser.FEATURE_PROCESS_NAMESPACES, false)
        p.setInput(ByteArrayInputStream(xml), null)
        return p
    }

    private fun header(title: String, pal: Pal): String = """
        <!DOCTYPE html><html><head><meta charset="utf-8">
        <meta name="viewport" content="width=device-width, initial-scale=1, maximum-scale=5, user-scalable=yes">
        <title>${esc(title)}</title>
        <style>
          body{margin:0;background:${pal.bg};color:${pal.text};
               font:14px/1.65 system-ui,-apple-system,sans-serif;padding:14px 12px 28px}
          .doc{max-width:860px;margin:0 auto}
          .doctitle{font-size:17px;font-weight:600;margin:4px 0 12px;color:${pal.title}}
          .tw{overflow-x:auto;-webkit-overflow-scrolling:touch}
          table{border-collapse:collapse;width:100%;margin:10px 0 16px;font-size:12.5px}
          caption{text-align:left;font-size:12.5px;color:${pal.muted};padding:4px 0}
          th,td{border:1px solid ${pal.border};padding:6px 9px;text-align:left;
                vertical-align:top;white-space:pre-wrap;word-break:break-word;min-width:56px}
          th{background:${pal.thead};font-weight:600;position:sticky;top:0}
          td.num,th.num{text-align:right;font-variant-numeric:tabular-nums;white-space:nowrap}
          tr:nth-child(even) td{background:${pal.zebra}}
          .md h1{font-size:20px;margin:16px 0 8px;color:${pal.title}}
          .md h2{font-size:17px;margin:14px 0 6px;color:${pal.title}}
          .md h3,.md h4,.doc h3{font-size:15px;margin:12px 0 6px;color:${pal.title}}
          .md p,.doc p{margin:7px 0}
          .md ul,.doc ul{margin:7px 0;padding-left:20px}
          .md li,.doc li{margin:3px 0}
          .blank{margin:4px 0}
          .md blockquote{margin:8px 0;padding:6px 12px;border-left:3px solid ${pal.quoteBar};
                         background:${pal.quote};border-radius:6px;color:${pal.muted}}
          .md hr{border:none;border-top:1px solid ${pal.border};margin:14px 0}
          .slide{border:1px solid ${pal.border};border-radius:12px;padding:12px 14px;
                 margin:12px 0;background:${pal.slide}}
          .slideno{font-size:10.5px;color:${pal.faint};margin-bottom:6px}
          .slidetitle{font-size:15.5px;font-weight:600;color:${pal.title};margin-bottom:8px}
          .slide ul{margin:0;padding-left:18px}
          .slide li{margin:4px 0;font-size:13px}
          .bullet{margin:4px 0;padding-left:14px;position:relative}
          .bullet:before{content:"·";position:absolute;left:2px;color:${pal.faint}}
          .pg{margin:0 0 12px}
          .pg img{width:100%;display:block;border-radius:6px;background:#fff;
                  border:1px solid ${pal.border}}
          code{background:${pal.code};border-radius:4px;padding:1px 5px;
               font:12px/1.5 ui-monospace,Menlo,Consolas,monospace}
          pre{background:${pal.code};border-radius:10px;padding:12px;overflow-x:auto;
              font:12px/1.6 ui-monospace,Menlo,Consolas,monospace;margin:10px 0;counter-reset:ln}
          pre .l{display:block;counter-increment:ln;white-space:pre}
          pre .l:before{content:counter(ln);display:inline-block;width:2.4em;margin-right:10px;
                        text-align:right;color:${pal.faint};opacity:.7;
                        -webkit-user-select:none;user-select:none}
          a{color:${pal.accent};text-decoration:none}
          .more{color:${pal.faint};font-size:12px;margin:8px 0}
          .err{padding:22px 16px}
          .err .t{color:${pal.err};font-size:17px;font-weight:600}
          .err .d{color:${pal.muted};font-size:13px;margin-top:8px}
        </style></head><body>
    """.trimIndent()

    private fun note(text: String, pal: Pal): String =
        """<div class="doc"><div class="more">${esc(text)}</div></div>"""

    private fun errorHtml(reason: String, pal: Pal): String =
        header("预览失败", pal) + """<div class="err"><div class="t">无法预览这个文件</div><div class="d">""" +
            esc(reason) + "</div></div>"

    private fun StringBuilder.esc(s: String): StringBuilder {
        append(s.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;").replace("\"", "&quot;"))
        return this
    }

    private fun esc(s: String): String =
        s.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;").replace("\"", "&quot;")
}
