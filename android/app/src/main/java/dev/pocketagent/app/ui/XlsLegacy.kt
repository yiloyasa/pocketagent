package dev.pocketagent.app.ui

/**
 * 旧版 Excel(.xls,BIFF8)本地解析器 —— 不引第三方库,纯 Kotlin 实现。
 *
 * 【为什么值得单独实现】工程造价、财务这类场景里 .xls 存量很大,如果只支持 .xlsx,
 * 用户遇到老文件就只剩"下载后用 WPS 打开"一条路,和 .xlsx 的体验落差太大。
 *
 * 【两层结构】
 *  1. [CfbReader] —— OLE2 复合文档(CFB)读取器:定位并取出内部的 "Workbook" 流;
 *  2. [biffSheets] —— BIFF8 记录流解析:BOF/BOUNDSHEET/SST/LABELSST/NUMBER/RK/MULRK/
 *     FORMULA/BOOLERR → 单元格矩阵。
 *
 * 【已知边界(有意为之)】
 *  · 公式只取缓存的计算结果(BIFF 里本来也只存结果),不重算;
 *  · 日期按 XF 的格式号识别并格式化,自定义日期格式只做关键字启发式判断;
 *  · 不做合并单元格、条件格式、图表 —— 目标是"看得懂数据",不是还原排版。
 */
internal object XlsLegacy {

    /** 一个工作表:名称 + 规整后的二维文本。 */
    class Sheet(val name: String, val rows: List<List<String>>)

    /** @return 解析结果;null 表示不是可识别的 BIFF 工作簿。 */
    fun parse(bytes: ByteArray, maxRows: Int): List<Sheet>? = runCatching {
        val cfb = CfbReader(bytes)
        val stream = cfb.openStream("Workbook") ?: cfb.openStream("Book") ?: return null
        biffSheets(scanForWorkbook(stream), maxRows)
    }.getOrNull()

    /** 少数文件把 xls 包在别的流里:扫 BOF(0x0809) 找 Workbook 全局子流。 */
    private fun scanForWorkbook(b: ByteArray): ByteArray {
        var p = 0
        while (p + 8 < b.size) {
            if (u16At(b, p) == 0x0809) {
                val ver = u16At(b, p + 4)
                if (ver == 0x0600 || ver == 0x0500) return b.copyOfRange(p, b.size)
            }
            p += 128
        }
        return b
    }

    // ------------------------------------------------------------------ BIFF8

    private class Rec(val type: Int, val data: ByteArray, val off: Int, val next: Int)

    private fun recs(buf: ByteArray, from: Int): List<Rec> {
        val out = ArrayList<Rec>()
        var p = from
        while (p + 4 <= buf.size) {
            val type = u16(buf, p)
            val len = u16(buf, p + 2)
            if (p + 4 + len > buf.size) break
            out.add(Rec(type, buf, p + 4, p + 4 + len))
            p += 4 + len
            if (type == 0x000A && out.size > 1) break // EOF 结束当前子流
        }
        return out
    }

    private fun biffSheets(stream: ByteArray, maxRows: Int): List<Sheet> {
        val globals = recs(stream, 0)

        // ---- 全局子流:共享字符串表 + 工作表目录 + 数字格式 ----
        val sstSegs = ArrayList<ByteArray>()
        var sstFound = false
        val bounds = ArrayList<Pair<String, Int>>() // (表名, 子流起始偏移)
        val xfToFormat = ArrayList<Int>()            // XF 序号 → 格式号
        val customFmt = HashMap<Int, String>()       // 自定义格式号 → 格式串

        for ((gi, r) in globals.withIndex()) {
            when (r.type) {
                0x00FC -> { // SST —— 紧跟其后的 CONTINUE 都是它的续段(必须连续收集)
                    sstSegs.add(r.data.copyOfRange(r.off, r.next))
                    sstFound = true
                    var j = gi + 1
                    while (j < globals.size && globals[j].type == 0x003C) {
                        sstSegs.add(globals[j].data.copyOfRange(globals[j].off, globals[j].next))
                        j++
                    }
                }
                0x0085 -> { // BOUNDSHEET
                    val pos = i32(r.data, r.off)
                    val cch = r.data[r.off + 6].toInt() and 0xFF
                    val flags = r.data[r.off + 7].toInt() and 0xFF
                    val name = readChars(r.data, r.off + 8, cch, (flags and 0x01) != 0)
                    bounds.add(name to pos)
                }
                0x00E0 -> xfToFormat.add(u16(r.data, r.off + 2)) // XF.ifmt
                0x041E -> { // FORMAT
                    val id = u16(r.data, r.off)
                    val cch = u16(r.data, r.off + 2)
                    val flags = r.data[r.off + 4].toInt() and 0xFF
                    customFmt[id] = readChars(r.data, r.off + 5, cch, (flags and 0x01) != 0)
                }
            }
        }
        val shared = if (sstSegs.isEmpty()) emptyList() else parseSst(sstSegs)

        // 全局子流里紧接着 SST 的 CONTINUE 也会被上面的循环收进来,这里只需保证顺序正确。
        if (bounds.isEmpty()) return emptyList()

        val sheets = ArrayList<Sheet>()
        for ((name, pos) in bounds) {
            if (sheets.size >= MAX_SHEETS_INTERNAL) break
            if (pos <= 0 || pos >= stream.size) continue
            sheets.add(Sheet(name, sheetCells(stream, pos, shared, xfToFormat, customFmt, maxRows)))
        }
        return sheets
    }

    private const val MAX_SHEETS_INTERNAL = 3
    private const val MAX_CELLS_PER_SHEET = 40000

    private fun sheetCells(
        stream: ByteArray,
        from: Int,
        shared: List<String>,
        xfToFormat: List<Int>,
        customFmt: Map<Int, String>,
        maxRows: Int,
    ): List<List<String>> {
        val cells = HashMap<Long, String>()
        var maxRow = -1
        var maxCol = -1
        var pendingString: Pair<Int, Int>? = null // 公式结果为字符串时,值在下一条 STRING 记录

        fun put(r0: Int, c0: Int, v: String) {
            if (cells.size >= MAX_CELLS_PER_SHEET) return
            if (v.isEmpty()) return
            cells[(r0.toLong() shl 16) or (c0.toLong() and 0xFFFF)] = v
            if (r0 > maxRow) maxRow = r0
            if (c0 > maxCol) maxCol = c0
        }

        for (r in recs(stream, from)) {
            val d = r.data
            val o = r.off
            when (r.type) {
                0x00FD -> { // LABELSST
                    val row = u16(d, o); val col = u16(d, o + 2)
                    val idx = i32(d, o + 6)
                    put(row, col, shared.getOrNull(idx) ?: "")
                }
                0x0204 -> { // LABEL
                    val row = u16(d, o); val col = u16(d, o + 2)
                    val cch = u16(d, o + 6)
                    val flags = d[o + 8].toInt() and 0xFF
                    put(row, col, readChars(d, o + 9, cch, (flags and 0x01) != 0))
                }
                0x0203 -> { // NUMBER
                    val row = u16(d, o); val col = u16(d, o + 2)
                    val xf = u16(d, o + 4)
                    val v = java.lang.Double.longBitsToDouble(i64(d, o + 6))
                    put(row, col, formatNumber(v, xf, xfToFormat, customFmt))
                }
                0x027E -> { // RK
                    val row = u16(d, o); val col = u16(d, o + 2)
                    val xf = u16(d, o + 4)
                    put(row, col, formatNumber(rkToDouble(i32(d, o + 6)), xf, xfToFormat, customFmt))
                }
                0x00BD -> { // MULRK
                    val row = u16(d, o)
                    val colFirst = u16(d, o + 2)
                    var p = o + 4
                    var c = colFirst
                    while (p + 6 <= r.next - 2) {
                        val xf = u16(d, p)
                        val v = rkToDouble(i32(d, p + 2))
                        put(row, c, formatNumber(v, xf, xfToFormat, customFmt))
                        p += 6
                        c++
                    }
                }
                0x0006 -> { // FORMULA(取缓存结果)
                    val row = u16(d, o); val col = u16(d, o + 2)
                    val xf = u16(d, o + 4)
                    val tag = u16(d, o + 12)
                    when {
                        tag == 0xFFFF -> when (d[o + 6].toInt() and 0xFF) {
                            0 -> pendingString = row to col // 结果在 STRING 记录里
                            1 -> put(row, col, if ((d[o + 8].toInt() and 0xFF) != 0) "TRUE" else "FALSE")
                            2 -> put(row, col, "#ERR")
                            else -> Unit
                        }
                        else -> {
                            val v = java.lang.Double.longBitsToDouble(i64(d, o + 6))
                            put(row, col, formatNumber(v, xf, xfToFormat, customFmt))
                        }
                    }
                }
                0x0207 -> { // STRING(接在 FORMULA 后)
                    pendingString?.let { (row, col) ->
                        val cch = u16(d, o)
                        val flags = d[o + 2].toInt() and 0xFF
                        put(row, col, readChars(d, o + 3, cch, (flags and 0x01) != 0))
                    }
                    pendingString = null
                }
                0x0205 -> { // BOOLERR
                    val row = u16(d, o); val col = u16(d, o + 2)
                    val v = d[o + 6].toInt() and 0xFF
                    val isErr = (d[o + 7].toInt() and 0xFF) != 0
                    put(row, col, if (isErr) "#ERR" else if (v != 0) "TRUE" else "FALSE")
                }
                0x000A -> break // 该表 EOF
            }
        }
        if (maxRow < 0) return emptyList()

        val lastRow = minOf(maxRow, maxRows - 1)
        val lastCol = minOf(maxCol, MAX_COLS_INTERNAL - 1)
        val out = ArrayList<List<String>>(lastRow + 1)
        for (r0 in 0..lastRow) {
            val row = ArrayList<String>(lastCol + 1)
            for (c0 in 0..lastCol) row.add(cells[(r0.toLong() shl 16) or (c0.toLong() and 0xFFFF)] ?: "")
            out.add(row)
        }
        return out
    }

    private const val MAX_COLS_INTERNAL = 40

    /** RK 编码的 32 位值 → Double(见 MS-XLS 2.5.162)。 */
    private fun rkToDouble(rk: Int): Double {
        val mul100 = (rk and 1) != 0
        val isInt = (rk and 2) != 0
        val v = if (isInt) {
            (rk shr 2).toDouble()
        } else {
            java.lang.Double.longBitsToDouble((rk.toLong() and 0xFFFFFFFCL) shl 32)
        }
        return if (mul100) v / 100.0 else v
    }

    /** 内建日期/时间格式号(见 MS-XLS 的 ifmt 约定)。 */
    private val BUILTIN_DATE_FMT = setOf(
        14, 15, 16, 17, 18, 19, 20, 21, 22,
        27, 28, 29, 30, 31, 32, 33, 34, 35, 36,
        45, 46, 47, 50, 51, 52, 53, 54, 55, 56, 57, 58,
    )

    private fun isDateFormat(xf: Int, xfToFormat: List<Int>, customFmt: Map<Int, String>): Boolean {
        val ifmt = xfToFormat.getOrNull(xf) ?: return false
        if (ifmt in BUILTIN_DATE_FMT) return true
        val f = customFmt[ifmt] ?: return false
        // 自定义格式:剥掉引号与方括号内容后再找日期关键字
        val cleaned = f.replace(Regex("\\[[^]]*]"), "").replace(Regex("\"[^\"]*\""), "").lowercase()
        return cleaned.contains('y') || cleaned.contains('d') ||
            (cleaned.contains('h') && cleaned.contains('m'))
    }

    /** 数字 → 展示文本(日期按 Excel 1900 序列号换算)。 */
    private fun formatNumber(v: Double, xf: Int, xfToFormat: List<Int>, customFmt: Map<Int, String>): String {
        if (isDateFormat(xf, xfToFormat, customFmt)) {
            val ms = ((v - 25569.0) * 86400000.0).toLong()
            return runCatching {
                val fmt = if (v == Math.floor(v)) "yyyy-MM-dd" else "yyyy-MM-dd HH:mm"
                java.text.SimpleDateFormat(fmt, java.util.Locale.US).format(java.util.Date(ms))
            }.getOrDefault(plain(v))
        }
        return plain(v)
    }

    private fun plain(v: Double): String {
        if (v.isNaN() || v.isInfinite()) return ""
        if (v == Math.floor(v) && Math.abs(v) < 1e15) return v.toLong().toString()
        return java.math.BigDecimal(v)
            .setScale(6, java.math.RoundingMode.HALF_UP)
            .stripTrailingZeros()
            .toPlainString()
    }

    // ------------------------------------------------------------------ SST

    /**
     * 共享字符串表。难点:SST 常常跨 CONTINUE 记录,而且**被切断的字符串在续段
     * 开头会多出一个 grbit 字节**(表示后半段是 8 位还是 16 位),16 位字符还可能
     * 正好被切成两半。这里用一个能跨段的读取器把这两种情况都处理掉。
     */
    private class SegReader(val segs: List<ByteArray>) {
        var i = 0
        var p = 0

        fun hasMore(): Boolean = i < segs.size - 1 || (i < segs.size && p < segs[i].size)

        private fun advance() {
            while (i < segs.size && p >= segs[i].size) { i++; p = 0 }
        }

        fun u8(): Int { advance(); if (i >= segs.size) return 0; return segs[i][p++].toInt() and 0xFF }
        fun u16(): Int = u8() or (u8() shl 8)
        fun i32(): Int = u16() or (u16() shl 16)

        fun skip(n: Int) {
            var left = n
            while (left > 0) {
                advance()
                if (i >= segs.size) return
                val take = minOf(left, segs[i].size - p)
                p += take
                left -= take
            }
        }

        /** 读一个 XLUnicodeString 的字符区(grbit 已由调用方消费)。 */
        fun chars(cch: Int, grbit0: Int): String {
            val sb = StringBuilder(cch)
            var high = (grbit0 and 0x01) != 0
            var n = 0
            while (n < cch) {
                advance()
                if (i >= segs.size) break
                if (p >= segs[i].size) continue
                if (p == 0 && n > 0) {
                    // 走到续段起点:第一个字节是新的 grbit
                    high = (u8() and 0x01) != 0
                }
                if (high) {
                    if (segs[i].size - p >= 2) {
                        sb.append(((segs[i][p].toInt() and 0xFF) or (segs[i][p + 1].toInt() shl 8)).toChar())
                        p += 2
                    } else {
                        // 16 位字符正好跨段:低字节在本段末尾,高字节在下一段开头
                        val lo = u8()
                        advance()
                        val hi = u8()
                        sb.append(((hi shl 8) or lo).toChar())
                    }
                } else {
                    sb.append((u8() and 0xFF).toChar())
                }
                n++
            }
            return sb.toString()
        }
    }

    private fun parseSst(segs: List<ByteArray>): List<String> {
        val r = SegReader(segs)
        r.i32()          // cstTotal
        val unique = r.i32() // cstUnique
        val out = ArrayList<String>(minOf(unique, 200000))
        var n = 0
        while (n < unique && r.hasMore()) {
            val cch = r.u16()
            val grbit = r.u8()
            val rich = (grbit and 0x08) != 0
            val ext = (grbit and 0x04) != 0
            val cRun = if (rich) r.u16() else 0
            val cbExt = if (ext) r.i32() else 0
            val s = r.chars(cch, grbit)
            r.skip(cRun * 4)
            r.skip(cbExt)
            out.add(s)
            n++
        }
        return out
    }

    // ------------------------------------------------------------------ 工具

    private fun u16(b: ByteArray, o: Int): Int =
        (b[o].toInt() and 0xFF) or ((b[o + 1].toInt() and 0xFF) shl 8)

    private fun i32(b: ByteArray, o: Int): Int =
        (b[o].toInt() and 0xFF) or ((b[o + 1].toInt() and 0xFF) shl 8) or
            ((b[o + 2].toInt() and 0xFF) shl 16) or ((b[o + 3].toInt() and 0xFF) shl 24)

    private fun i64(b: ByteArray, o: Int): Long {
        var v = 0L
        for (k in 7 downTo 0) v = (v shl 8) or (b[o + k].toLong() and 0xFF)
        return v
    }

    /** 连续 [cch] 个字符:compressed(8 位,取低字节) 或 UTF-16LE。 */
    private fun readChars(b: ByteArray, o: Int, cch: Int, high: Boolean): String {
        if (cch <= 0) return ""
        return runCatching {
            if (high) {
                String(b, o, minOf(cch * 2, b.size - o), Charsets.UTF_16LE)
            } else {
                val sb = StringBuilder(cch)
                for (k in 0 until cch) {
                    val idx = o + k
                    if (idx >= b.size) break
                    val v = b[idx].toInt() and 0xFF
                    // BIFF8 的 compressed 段其实是 Latin-1 子集;中文必然走 high 分支,
                    // 这里对 >=0x80 的字节按 CP1252 还原,避免出现问号。
                    sb.append(if (v < 0x80) v.toChar() else CP1252_BYTE_TO_CHAR[v - 0x80])
                }
                sb.toString()
            }
        }.getOrDefault("")
    }

    private val CP1252_BYTE_TO_CHAR = CharArray(128) { i ->
        val cp = i + 0x80
        val map = intArrayOf(
            0x20AC, -1, 0x201A, 0x0192, 0x201E, 0x2026, 0x2020, 0x2021, 0x02C6, 0x2030,
            0x0160, 0x2039, 0x0152, -1, 0x017D, -1, -1, 0x2018, 0x2019, 0x201C, 0x201D,
            0x2022, 0x2013, 0x2014, 0x02DC, 0x2122, 0x0161, 0x203A, 0x0153, -1, 0x017E, 0x0178,
        )
        val mapped = if (cp in 0x80..0x9F) map[cp - 0x80] else cp
        (if (mapped < 0) cp else mapped).toChar()
    }
}
