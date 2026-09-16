package dev.pocketagent.app.ui

import android.content.Context
import android.util.Xml
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
 * 【覆盖矩阵】必须适配(内置可读):
 *   xlsx / xls / ods / csv / tsv  → 表格
 *   docx / odt / rtf              → 文档
 *   pptx / odp                    → 演示大纲
 *   pdf                           → 逐页位图
 *   md / txt / json / xml / 源码  → 排版页 / 等宽页
 *   html / 图片                   → 交给 WebView 原生渲染(BrowserScreen 直接处理)
 *
 * 【不做的】.doc / .ppt(OLE 二进制记录流,解析性价比极低)→ 下载后用系统应用打开;
 *   压缩包 / 音视频 / 安装包 → 容器或媒体,见 [dev.pocketagent.app.ui.component.FileKind]。
 *
 * 【失败也要说清楚】每个渲染器内部自行兜底成 [errorHtml],返回 null 只表示
 * "这个扩展名我没有渲染器",调用方据此降级为二进制提示页 / 等宽文本页。
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

    /**
     * 把文件字节渲染成完整 HTML。
     * @return null = 该扩展名没有内置渲染器(调用方降级处理)。
     */
    fun render(ctx: Context, fileName: String, bytes: ByteArray): String? {
        val n = fileName.lowercase()
        return when {
            n.endsWith(".xlsx") || n.endsWith(".xlsm") -> guard { xlsx(bytes) }
            n.endsWith(".xls") -> guard { xls(bytes) }
            n.endsWith(".ods") || n.endsWith(".fods") -> guard { odf(bytes) }
            n.endsWith(".docx") || n.endsWith(".docm") -> guard { docx(bytes) }
            n.endsWith(".rtf") -> guard { rtf(bytes) }
            n.endsWith(".odt") || n.endsWith(".fodt") -> guard { odf(bytes) }
            n.endsWith(".pptx") || n.endsWith(".pptm") -> guard { pptx(bytes) }
            n.endsWith(".odp") || n.endsWith(".fodp") -> guard { odf(bytes) }
            n.endsWith(".csv") || n.endsWith(".tsv") -> guard { csv(bytes) }
            n.endsWith(".pdf") -> guard { pdf(ctx, bytes) }
            else -> null
        }
    }

    /** 渲染器内部崩了也要给一张可读的失败页,而不是让调用方看到白屏。 */
    private inline fun guard(block: () -> String): String =
        runCatching(block).getOrElse { errorHtml("解析失败:${it.message ?: it.javaClass.simpleName}") }

    /** 是否为"不该按文本渲染"的二进制格式(未被 [render] 覆盖的那部分)。 */
    fun isBinary(fileName: String): Boolean {
        val ext = fileName.substringAfterLast('.', "").lowercase()
        return ext in setOf(
            "doc", "ppt", "dot", "apk", "apks", "xapk", "exe", "dll", "so", "dylib",
            "zip", "rar", "7z", "tar", "gz", "tgz", "bz2", "xz", "jar", "class", "db", "sqlite",
            "mp3", "wav", "flac", "aac", "ogg", "m4a", "mp4", "mkv", "mov", "avi", "webm",
            "psd", "ttf", "otf", "woff", "woff2", "bin", "dat", "pak",
        )
    }

    // ---------------------------------------------------------------- XLSX

    private fun xlsx(bytes: ByteArray): String {
        val entries = readZip(bytes) ?: return errorHtml("无法解压这个 xlsx 文件(可能已损坏)")
        val shared = parseSharedStrings(entries["xl/sharedStrings.xml"])
        val (sheetNames, sheetFiles) = resolveSheets(entries)

        val sb = StringBuilder()
        sb.append(header("表格预览"))
        var rendered = 0
        for (i in sheetFiles.indices) {
            if (rendered >= MAX_SHEETS) break
            val xml = entries["xl/${sheetFiles[i]}"]
                ?: entries.keys.firstOrNull { it.endsWith(sheetFiles[i]) }?.let { entries[it] }
                ?: continue
            val table = sheetToHtml(xml, shared, sheetNames.getOrNull(i)) ?: continue
            sb.append(table)
            rendered++
        }
        if (rendered == 0) return errorHtml("表格里没有可显示的数据")
        if (sheetFiles.size > rendered) sb.append(note("只显示前 $rendered 个工作表"))
        return sb.toString()
    }

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
                    // 命名空间关闭时属性名是原样的 "r:id"
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
            // rels 解析失败:退化为按文件名枚举 sheet1.xml / sheet2.xml…
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

    private fun sheetToHtml(xml: ByteArray, shared: List<String>, sheetName: String?): String? {
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
            val cellText = StringBuilder()
            var rowCells = ArrayList<String>()
            while (p.next() != XmlPullParser.END_DOCUMENT) {
                when (p.eventType) {
                    XmlPullParser.START_TAG -> when (ln(p)) {
                        "row" -> rowCells = ArrayList()
                        "c" -> {
                            inCell = true
                            cellType = p.getAttributeValue(null, "t") ?: ""
                            // 稀疏表:跳过被省略的列,保证列对得齐
                            val ref = p.getAttributeValue(null, "r")
                            val colIdx = ref?.let { colOf(it) }
                            if (colIdx != null) {
                                while (rowCells.size < colIdx && rowCells.size < MAX_COLS) rowCells.add("")
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
                            val display = when (cellType) {
                                "s" -> shared.getOrNull(raw.toIntOrNull() ?: -1) ?: ""
                                "b" -> if (raw == "1") "TRUE" else "FALSE"
                                "e" -> "#ERR"
                                else -> raw
                            }
                            rowCells.add(display)
                        }
                        "row" -> {
                            if (rowCells.any { it.isNotBlank() }) {
                                hasData = true
                                if (rows >= MAX_ROWS) {
                                    truncated = true
                                } else {
                                    val tag = if (rows == 0) "th" else "td"
                                    sb.append("<tr>")
                                    rowCells.take(MAX_COLS).forEach { c ->
                                        sb.append("<").append(tag).append(">")
                                            .esc(c).append("</").append(tag).append(">")
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
        if (truncated) head.append(note("数据较多,只显示前 $MAX_ROWS 行"))
        head.append("</div>")
        return head.toString()
    }

    /** 单元格引用 "BC12" → 列序号(0 基)。 */
    private fun colOf(ref: String): Int? {
        var v = 0
        var any = false
        for (ch in ref) {
            val u = ch.uppercaseChar()
            if (u in 'A'..'Z') { v = v * 26 + (u - 'A' + 1); any = true } else break
        }
        return if (any) v - 1 else null
    }

    // ---------------------------------------------------------------- XLS(旧版)

    private fun xls(bytes: ByteArray): String {
        val sheets = XlsLegacy.parse(bytes, MAX_ROWS)
            ?: return errorHtml("无法解析这个 .xls 文件(可能已损坏,或不是 Excel 97-2003 格式)")
        if (sheets.isEmpty()) return errorHtml("表格里没有可显示的数据")
        val sb = StringBuilder(header("表格预览"))
        var count = 0
        for (s in sheets) {
            if (count >= MAX_SHEETS) break
            if (s.rows.isEmpty()) continue
            sb.append(htmlTable(s.name, s.rows))
            count++
        }
        if (count == 0) return errorHtml("表格里没有可显示的数据")
        if (sheets.size > count) sb.append(note("只显示前 $count 个工作表"))
        sb.append(note("旧版 .xls 只能读到单元格的值与日期;公式显示为缓存结果,不含图表与条件格式。"))
        return sb.toString()
    }

    // ---------------------------------------------------------------- ODF(ods/odt/odp)

    /** OpenDocument 三类文档共用一个解析器:content.xml 的结构差异只体现在包装标签上。 */
    private fun odf(bytes: ByteArray): String {
        val entries = readZip(bytes) ?: return errorHtml("无法解压这个 OpenDocument 文件(可能已损坏)")
        val xml = entries["content.xml"] ?: return errorHtml("不是有效的 OpenDocument 文件")
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

        var guard = 0
        runCatching {
            while (p.next() != XmlPullParser.END_DOCUMENT) {
                if (guard++ > 4_000_000) break
                val ev = p.eventType
                // 【不能提前取 p.name】TEXT/其它事件的 name 是 null,取了会 NPE;
                // 只在标签事件上取本地名。
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
                            val tag = if (rows == 0 && mode == "spreadsheet") "th" else "td"
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
            "" -> return errorHtml("这个 OpenDocument 文件里没有可显示的内容")
            "presentation" -> {
                sb.append(header("演示文稿预览"))
                sb.append("<div class=\"doc\">").append(sbTail).append("</div>")
            }
            "spreadsheet" -> {
                // spreadsheet 模式下文档没有 <table>,内容已在 table 结束时就地搬进 sb
                flushSpreadsheetInto(sb, sbTail)
                if (sb.length == 0) return errorHtml("表格里没有可显示的数据")
                sb.insert(0, header("表格预览"))
            }
            else -> {
                sb.append(header("文档预览"))
                sb.append("<div class=\"doc\">").append(sbTail).append("</div>")
            }
        }
        if (truncated) sb.append(note("数据较多,只显示前 $MAX_ROWS 行"))
        return sb.toString()
    }

    /** 把累积的表格片段搬进总输出,并给每张表加上文档级容器。 */
    private fun flushSpreadsheetInto(sb: StringBuilder, tail: StringBuilder) {
        if (tail.length == 0) return
        sb.append("<div class=\"doc\">").append(tail).append("</div>")
        tail.setLength(0)
    }

    // ---------------------------------------------------------------- DOCX

    private fun docx(bytes: ByteArray): String {
        val entries = readZip(bytes) ?: return errorHtml("无法解压这个 docx 文件(可能已损坏)")
        val xml = entries["word/document.xml"] ?: return errorHtml("不是有效的 docx 文档")
        val sb = StringBuilder(header("文档预览"))
        sb.append("""<div class="doc">""")
        runCatching {
            val p = parser(xml)
            var inParagraph = false
            var inText = false
            var para = StringBuilder()
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
    private fun rtf(bytes: ByteArray): String {
        val raw = String(bytes, Charsets.ISO_8859_1)
        val sb = StringBuilder(header("RTF 预览"))
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
                        nc == '\'' -> { // \'hh —— 单字节字符
                            val hex = raw.substring(i + 1, minOf(i + 3, raw.length))
                            val v = hex.toIntOrNull(16) ?: 0
                            out.append(CP1252_BYTE_TO_CHAR0[v and 0xFF])
                            i += 3
                        }
                        nc == '*' -> { // 目标组:整组丢弃
                            skipDepth = depth
                            i++
                        }
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
                '\r', '\n' -> i++ // RTF 源码里的换行不是内容
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
                // RTF 里手工敲的圆点/数字列表:转成真列表好看得多
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

    private val CP1252_BYTE_TO_CHAR0 = CharArray(256) { i ->
        if (i < 0x80) i.toChar()
        else {
            val cp = i
            val map = intArrayOf(
                0x20AC, -1, 0x201A, 0x0192, 0x201E, 0x2026, 0x2020, 0x2021, 0x02C6, 0x2030,
                0x0160, 0x2039, 0x0152, -1, 0x017D, -1, -1, 0x2018, 0x2019, 0x201C, 0x201D,
                0x2022, 0x2013, 0x2014, 0x02DC, 0x2122, 0x0161, 0x203A, 0x0153, -1, 0x017E, 0x0178,
            )
            val m = if (cp in 0x80..0x9F) map[cp - 0x80] else cp
            (if (m < 0) cp else m).toChar()
        }
    }

    // ---------------------------------------------------------------- PPTX

    private fun pptx(bytes: ByteArray): String {
        val entries = readZip(bytes) ?: return errorHtml("无法解压这个 pptx 文件(可能已损坏)")
        val slides = slideOrder(entries)
        if (slides.isEmpty()) return errorHtml("这个演示文稿里没有幻灯片")

        val sb = StringBuilder(header("演示文稿预览"))
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
        if (shown == 0) return errorHtml("幻灯片里没有可提取的文字(可能是纯图片版式)")
        sb.append("</div>")
        if (slides.size > shown) sb.append(note("只显示前 $shown 页"))
        sb.append(note("演示文稿按「标题 + 要点」提取文字,不含图片、图表与动画。"))
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
                        "t" -> Unit
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

    private fun csv(bytes: ByteArray): String {
        val text = String(bytes, Charsets.UTF_8)
        val sep = if (text.contains('\t') && !text.contains(',')) '\t' else ','
        val rows = ArrayList<List<String>>()
        var trunc = false
        text.lineSequence().forEachIndexed { i, line ->
            if (line.isBlank() && i == 0) return@forEachIndexed
            if (rows.size >= MAX_ROWS) { trunc = true; return@forEachIndexed }
            rows.add(splitCsvLine(line, sep).take(MAX_COLS))
        }
        if (rows.isEmpty()) return errorHtml("文件里没有可显示的数据")
        val sb = StringBuilder(header("表格预览"))
        sb.append(htmlTable("", rows))
        if (trunc) sb.append(note("数据较多,只显示前 $MAX_ROWS 行"))
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
    private fun pdf(ctx: Context, bytes: ByteArray): String {
        if (bytes.size < 16) return errorHtml("文件内容为空")
        val tmp = java.io.File(ctx.cacheDir, "pa_pdf_${System.nanoTime()}.pdf")
        var pfd: android.os.ParcelFileDescriptor? = null
        var renderer: android.graphics.pdf.PdfRenderer? = null
        return try {
            tmp.writeBytes(bytes)
            pfd = android.os.ParcelFileDescriptor.open(tmp, android.os.ParcelFileDescriptor.MODE_READ_ONLY)
            val r = android.graphics.pdf.PdfRenderer(pfd)
            renderer = r
            val total = r.pageCount
            if (total <= 0) {
                errorHtml("这个 PDF 没有可显示的页面")
            } else {
                val pages = minOf(total, MAX_PDF_PAGES)
                val sb = StringBuilder(header("PDF 预览"))
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
                if (total > pages) sb.append(note("共 $total 页,这里只渲染前 $pages 页。下载到手机可用阅读器看完整版。"))
                sb.toString()
            }
        } catch (e: Exception) {
            errorHtml("这个 PDF 打不开:${e.message ?: "可能已加密或损坏"}")
        } finally {
            runCatching { renderer?.close() }
            runCatching { pfd?.close() }
            runCatching { tmp.delete() }
        }
    }

    // ---------------------------------------------------------------- Markdown

    /** 极简 Markdown → HTML:标题/加粗/斜体/行内码/代码块/列表/引用/分隔线/链接。 */
    fun markdownHtml(md: String, title: String): String {
        val sb = StringBuilder()
        sb.append(header(title.substringAfterLast('/').substringAfterLast('\\')))
        sb.append("""<div class="doc md">""")
        val lines = md.lines()
        var inCode = false
        var inList = false
        val codeBuf = StringBuilder()
        for (raw in lines) {
            val line = raw.trimEnd()
            if (line.startsWith("```")) {
                if (inCode) {
                    sb.append("<pre>").esc(codeBuf.toString()).append("</pre>")
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
        if (inCode && codeBuf.isNotEmpty()) sb.append("<pre>").esc(codeBuf.toString()).append("</pre>")
        sb.append("</div>")
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
    private fun htmlTable(name: String, rows: List<List<String>>): String {
        val sb = StringBuilder("""<div class="doc">""")
        if (name.isNotBlank()) sb.append("""<div class="doctitle">""").esc(name).append("</div>")
        sb.append("<div class=\"tw\"><table>")
        rows.forEachIndexed { i, row ->
            val tag = if (i == 0) "th" else "td"
            sb.append("<tr>")
            row.take(MAX_COLS).forEach { c ->
                sb.append("<").append(tag).append(">").esc(c).append("</").append(tag).append(">")
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

    private fun header(title: String): String = """
        <!DOCTYPE html><html><head><meta charset="utf-8">
        <meta name="viewport" content="width=device-width, initial-scale=1, maximum-scale=5, user-scalable=yes">
        <title>${esc(title)}</title>
        <style>
          body{margin:0;background:#0b0b0d;color:#e6e6e6;
               font:14px/1.65 system-ui,-apple-system,sans-serif;padding:14px 12px 28px}
          .doc{max-width:860px;margin:0 auto}
          .doctitle{font-size:17px;font-weight:600;margin:4px 0 12px;color:#fff}
          .tw{overflow-x:auto;-webkit-overflow-scrolling:touch}
          table{border-collapse:collapse;width:100%;margin:10px 0 16px;font-size:12.5px}
          caption{text-align:left;font-size:12.5px;color:#98989f;padding:4px 0}
          th,td{border:1px solid #2a2a30;padding:6px 8px;text-align:left;
                white-space:nowrap;max-width:320px;overflow:hidden;text-overflow:ellipsis}
          th{background:#17171a;font-weight:600;position:sticky;top:0}
          tr:nth-child(even) td{background:#121215}
          .md h1{font-size:20px;margin:16px 0 8px}
          .md h2{font-size:17px;margin:14px 0 6px}
          .md h3,.md h4,.doc h3{font-size:15px;margin:12px 0 6px;color:#fff}
          .md p,.doc p{margin:7px 0}
          .md ul,.doc ul{margin:7px 0;padding-left:20px}
          .md li,.doc li{margin:3px 0}
          .blank{margin:4px 0}
          .md blockquote{margin:8px 0;padding:6px 12px;border-left:3px solid #4c8dff;
                         background:#17171a;border-radius:6px;color:#b9b9c0}
          .md hr{border:none;border-top:1px solid #2a2a30;margin:14px 0}
          .slide{border:1px solid #2a2a30;border-radius:12px;padding:12px 14px;
                 margin:12px 0;background:#121215}
          .slideno{font-size:10.5px;color:#7a7a82;margin-bottom:6px}
          .slidetitle{font-size:15.5px;font-weight:600;color:#fff;margin-bottom:8px}
          .slide ul{margin:0;padding-left:18px}
          .slide li{margin:4px 0;color:#d8d8de;font-size:13px}
          .bullet{margin:4px 0;padding-left:14px;position:relative;color:#d8d8de}
          .bullet:before{content:"·";position:absolute;left:2px;color:#7a7a82}
          .pg{margin:0 0 12px}
          .pg img{width:100%;display:block;border-radius:6px;background:#fff}
          code{background:#17171a;border-radius:4px;padding:1px 5px;
               font:12px/1.5 ui-monospace,Menlo,Consolas,monospace}
          pre{background:#17171a;border-radius:10px;padding:12px;overflow-x:auto;
              font:12px/1.6 ui-monospace,Menlo,Consolas,monospace;margin:10px 0}
          a{color:#4c8dff;text-decoration:none}
          .more{color:#7a7a82;font-size:12px;margin:8px 0}
          .err{padding:22px 16px}
          .err .t{color:#ff6b6b;font-size:17px;font-weight:600}
          .err .d{color:#b9b9c0;font-size:13px;margin-top:8px}
        </style></head><body>
    """.trimIndent()

    private fun note(text: String): String = """<div class="doc"><div class="more">${esc(text)}</div></div>"""

    private fun errorHtml(reason: String): String =
        header("预览失败") + """<div class="err"><div class="t">无法预览这个文件</div><div class="d">""" +
            esc(reason) + "</div></div>"

    private fun StringBuilder.esc(s: String): StringBuilder {
        append(s.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;").replace("\"", "&quot;"))
        return this
    }

    private fun esc(s: String): String =
        s.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;").replace("\"", "&quot;")
}
