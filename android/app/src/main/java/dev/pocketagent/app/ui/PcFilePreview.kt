package dev.pocketagent.app.ui

import android.util.Xml
import org.xmlpull.v1.XmlPullParser
import java.io.ByteArrayInputStream
import java.util.zip.ZipInputStream

/**
 * 电脑文件 → 内置浏览器可渲染的 HTML(纯本地解析,不引第三方库)。
 *
 * 【为什么要做】agent 产出的 Excel/Word/CSV/Markdown 之前都被当普通文本塞进
 * <pre> —— xlsx/docx 是 zip 压缩的二进制,看到的是乱码。这里用系统自带的
 * ZipInputStream + XmlPullParser 直接解析 OOXML 结构,渲染成真正的表格/文档。
 *
 * 【覆盖范围】xlsx(表格)、docx(文档)、csv(表格)、md(Markdown)。
 * 老 .xls/.doc(P1 二进制格式)与其他二进制(pdf/zip/apk…)不支持,由调用方给提示页。
 */
object PcFilePreview {

    /** 单表上限:防止巨型表格把 WebView 内存打爆(超出截断并标注)。 */
    private const val MAX_ROWS = 800
    private const val MAX_SHEETS = 3

    /** @return 渲染好的完整 HTML;null = 该类型不支持结构化预览(调用方自行降级)。 */
    fun render(fileName: String, bytes: ByteArray): String? = when {
        fileName.endsWith(".xlsx", true) -> xlsx(bytes)
        fileName.endsWith(".docx", true) -> docx(bytes)
        fileName.endsWith(".csv", true) || fileName.endsWith(".tsv", true) -> csv(bytes)
        else -> null
    }

    /** 是否为"不该按文本渲染"的二进制格式。 */
    fun isBinary(fileName: String): Boolean {
        val ext = fileName.substringAfterLast('.', "").lowercase()
        return ext in setOf(
            "xls", "doc", "ppt", "pdf", "apk", "apks", "xapk", "exe", "dll", "so", "dylib",
            "zip", "rar", "7z", "tar", "gz", "bz2", "xz", "jar", "class",
            "mp3", "wav", "flac", "aac", "ogg", "m4a", "mp4", "mkv", "mov", "avi", "webm",
            "png", "jpg", "jpeg", "gif", "webp", "bmp", "psd", "ttf", "otf",
        )
    }

    // ---------------------------------------------------------------- XLSX

    private fun xlsx(bytes: ByteArray): String {
        val entries = readZip(bytes) ?: return errorHtml("无法解压这个 xlsx 文件(可能已损坏)")
        val shared = parseSharedStrings(entries["xl/sharedStrings.xml"])
        val (sheetNames, sheetFiles) = resolveSheets(entries)

        val sb = StringBuilder()
        sb.append(header("表格预览"))
        sb.append("""<div class="doc"><div class="doctitle">""").esc(sheetNames.firstOrNull() ?: "Sheet").append("</div>")
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
        if (sheetFiles.size > rendered) {
            sb.append("""<div class="more">只显示前 $rendered 个工作表</div>""")
        }
        sb.append("</div>")
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
            val p = parser(entries["xl/workbook.xml"] ?: return names to files)
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
            val sorted = entries.keys.filter { Regex("^xl/worksheets/sheet\\d+\\.xml$").matches(it) }.sorted()
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
                    XmlPullParser.START_TAG -> when {
                        ln(p) == "si" -> { inSi = true; cur.setLength(0) }
                        ln(p) == "t" && inSi -> inT = true
                    }
                    XmlPullParser.TEXT -> if (inT) cur.append(p.text ?: "")
                    XmlPullParser.END_TAG -> when {
                        ln(p) == "t" -> inT = false
                        ln(p) == "si" -> { inSi = false; out.add(cur.toString()) }
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
            var inRow = false
            var inCell = false
            var inV = false
            var inInlineT = false
            var cellType = ""
            var cellText = StringBuilder()
            var rowCells = ArrayList<String>()
            while (p.next() != XmlPullParser.END_DOCUMENT) {
                when (p.eventType) {
                    XmlPullParser.START_TAG -> when (ln(p)) {
                        "row" -> { inRow = true; rowCells = ArrayList() }
                        "c" -> {
                            inCell = true
                            cellType = p.getAttributeValue(null, "t") ?: ""
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
                                else -> raw
                            }
                            rowCells.add(display)
                        }
                        "row" -> {
                            inRow = false
                            if (rowCells.any { it.isNotBlank() }) {
                                hasData = true
                                if (rows >= MAX_ROWS) {
                                    truncated = true
                                } else {
                                    val tag = if (rows == 0) "th" else "td"
                                    sb.append("<tr>")
                                    rowCells.forEach { c ->
                                        sb.append("<").append(tag).append(">").esc(c).append("</").append(tag).append(">")
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
        head.esc(sheetName ?: "Sheet").append("</div><table>")
        head.append(sb).append("</table>")
        if (truncated) head.append("""<div class="more">数据较多,只显示前 $MAX_ROWS 行</div>""")
        head.append("</div>")
        return head.toString()
    }

    // ---------------------------------------------------------------- DOCX

    private fun docx(bytes: ByteArray): String {
        val entries = readZip(bytes) ?: return errorHtml("无法解压这个 docx 文件(可能已损坏)")
        val xml = entries["word/document.xml"] ?: return errorHtml("不是有效的 docx 文档")
        val sb = StringBuilder()
        sb.append(header("文档预览"))
        sb.append("""<div class="doc">""")
        runCatching {
            val p = parser(xml)
            var inParagraph = false
            var inText = false
            var para = StringBuilder()
            var tableMode = false
            var rowMode = false
            var cellMode = false
            while (p.next() != XmlPullParser.END_DOCUMENT) {
                when (p.eventType) {
                    XmlPullParser.START_TAG -> when (ln(p)) {
                        "p" -> { inParagraph = true; para.setLength(0) }
                        "t" -> { inText = true; para.append(p.text ?: "") }
                        "tbl" -> { tableMode = true; sb.append("<table>") }
                        "tr" -> { rowMode = true; sb.append("<tr>") }
                        "tc" -> { cellMode = true; sb.append("<td>") }
                        "br", "cr" -> if (inParagraph) para.append("<br>")
                    }
                    XmlPullParser.TEXT -> if (inText) para.append(p.text ?: "")
                    XmlPullParser.END_TAG -> when (ln(p)) {
                        "t" -> inText = false
                        "p" -> {
                            inParagraph = false
                            if (cellMode) {
                                if (para.isNotEmpty()) sb.append(para)
                            } else {
                                if (para.isNotEmpty()) sb.append("<p>").append(para).append("</p>")
                                else if (!tableMode) sb.append("<p class=blank>&nbsp;</p>")
                            }
                            para.setLength(0)
                        }
                        "tc" -> { cellMode = false; sb.append("</td>") }
                        "tr" -> { rowMode = false; sb.append("</tr>") }
                        "tbl" -> { tableMode = false; sb.append("</table>") }
                    }
                }
            }
        }
        sb.append("</div>")
        return sb.toString()
    }

    // ---------------------------------------------------------------- CSV / TSV

    private fun csv(bytes: ByteArray): String {
        val text = String(bytes, Charsets.UTF_8)
        val sep = if (text.contains('\t') && !text.contains(',')) '\t' else ','
        val sb = StringBuilder()
        sb.append(header("表格预览"))
        sb.append("""<div class="doc"><table>""")
        var rows = 0
        text.lineSequence().take(MAX_ROWS).forEachIndexed { i, line ->
            if (line.isBlank()) return@forEachIndexed
            val cells = splitCsvLine(line, sep)
            val tag = if (i == 0) "th" else "td"
            sb.append("<tr>")
            cells.forEach { c -> sb.append("<").append(tag).append(">").esc(c).append("</").append(tag).append(">") }
            sb.append("</tr>")
            rows++
        }
        sb.append("</table></div>")
        return if (rows == 0) errorHtml("文件里没有可显示的数据") else sb.toString()
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
                line.isBlank() -> { if (inList) { sb.append("</ul>"); inList = false } }
                heading != null -> {
                    if (inList) { sb.append("</ul>"); inList = false }
                    val level = heading.groupValues[1].length
                    sb.append("<h").append(level).append(">").append(inline(heading.groupValues[2])).append("</h").append(level).append(">")
                }
                listMatch != null -> {
                    if (!inList) { sb.append("<ul>"); inList = true }
                    sb.append("<li>").append(inline(listMatch.groupValues[2])).append("</li>")
                }
                line.startsWith(">") -> sb.append("<blockquote>").append(inline(line.removePrefix(">").trim())).append("</blockquote>")
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

    private fun readZip(bytes: ByteArray): Map<String, ByteArray>? {
        return runCatching {
            val out = HashMap<String, ByteArray>()
            ZipInputStream(ByteArrayInputStream(bytes)).use { zis ->
                var e = zis.nextEntry
                while (e != null && out.size < 400) {
                    if (!e.isDirectory && e.size <= 64L * 1024 * 1024) {
                        out[e.name] = zis.readBytes()
                    }
                    zis.closeEntry()
                    e = zis.nextEntry
                }
            }
            out
        }.getOrNull()
    }


    /** 标签的本地名(去掉命名空间前缀):官方 Excel 写裸名,不少生成库写 x:row / w:t。 */
    private fun ln(p: XmlPullParser): String = p.name.substringAfterLast(':')

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
          .doc{max-width:820px;margin:0 auto}
          .doctitle{font-size:17px;font-weight:600;margin:4px 0 12px;color:#fff}
          table{border-collapse:collapse;width:100%;margin:10px 0 16px;font-size:12.5px}
          th,td{border:1px solid #2a2a30;padding:6px 8px;text-align:left;
                white-space:nowrap;max-width:320px;overflow:hidden;text-overflow:ellipsis}
          th{background:#17171a;font-weight:600;position:sticky;top:0}
          tr:nth-child(even) td{background:#121215}
          .md h1{font-size:20px;margin:16px 0 8px}
          .md h2{font-size:17px;margin:14px 0 6px}
          .md h3,.md h4{font-size:15px;margin:12px 0 6px}
          .md p{margin:7px 0}
          .md ul{margin:7px 0;padding-left:20px}
          .md li{margin:3px 0}
          .md blockquote{margin:8px 0;padding:6px 12px;border-left:3px solid #4c8dff;
                         background:#17171a;border-radius:6px;color:#b9b9c0}
          .md hr{border:none;border-top:1px solid #2a2a30;margin:14px 0}
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

    private fun errorHtml(reason: String): String =
        header("预览失败") + """<div class="err"><div class="t">无法预览这个文件</div><div class="d">""" +
            esc(reason) + "</div></div>"

    private fun StringBuilder.esc(s: String): StringBuilder {
        append(
            s.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;")
                .replace("\"", "&quot;"),
        )
        return this
    }

    private fun esc(s: String): String =
        s.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;").replace("\"", "&quot;")
}
