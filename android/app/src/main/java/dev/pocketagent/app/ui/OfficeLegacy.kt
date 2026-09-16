package dev.pocketagent.app.ui

/**
 * 旧版 Office(.doc/.ppt)文字提取器 —— 复用 [CfbReader](OLE2/CFB),纯 Kotlin。
 *
 * 【为什么做】.doc/.ppt 存量极大,之前"下载后交给系统应用"是唯一出路:
 * 手机上没装 WPS/Office 时「查看」就是死路,和 .docx/.pptx/.xls 的内置预览落差太大
 * (与 .xls 走 BIFF8 自研解析同一逻辑)。现在对齐:提取文字大纲做内置预览,
 * **不还原排版**(图表/图片/页眉页脚/批注一律不渲染)。
 *
 * 【两层结构】
 *  1. [CfbReader] —— OLE2 容器:取出 "WordDocument" / "PowerPoint Document" 流;
 *  2. [docParagraphs] —— FIB → 正文 CP 数;复杂文件走 piece table(CLX@1Table/0Table),
 *     简单文件直接按 fcMin..fcMac 切(字节数是 ccpText 的 2 倍即 UTF-16,否则按单字节);
 *     [pptSlidesFromStream] —— 二进制记录流线性递归:RT_SLIDE(1006) 容器开新页,
 *     页内 TextCharsAtom(UTF-16)/TextBytesAtom(单字节)/CString 收集文字,
 *     TextHeaderAtom 的 recInstance 区分标题(0)/正文;Notes/MainMaster 容器不收集。
 *
 * 【已知边界】域代码只保留结果;加密文档返回 null;8-bit 文本按 Latin-1 解
 * (CJK 正文必然是 UTF-16 分片,不受影响)。
 */
internal object OfficeLegacy {

    private const val MAX_DOC_CHARS = 2_000_000
    private const val MAX_CLX_BYTES = 4_000_000

    // .doc FIB 偏移(MS-DOC,FIB base + FibRgLw97 + FibRgFcLcb97)
    private const val FIB_FLAGS = 0x0A      // fComplex=0x0004 fEncrypted=0x0100 fWhichTblStm=0x0200
    private const val FIB_FCMIN = 0x18
    private const val FIB_FCMAC = 0x1C
    private const val FIB_CCPTEXT = 0x4C
    private const val FIB_FCCLX = 0x1A2
    private const val FIB_LCBCLX = 0x1A6

    // .ppt 记录类型(MS-PPT RecordType)
    private const val RT_SLIDE = 1006
    private const val RT_NOTES = 1008
    private const val RT_MAINMASTER = 1016
    private const val RT_TEXTHEADER = 3999  // recInstance:0=标题 1=正文 2=备注
    private const val RT_TEXTCHARS = 4000   // UTF-16LE
    private const val RT_TEXTBYTES = 4008   // 单字节(Latin-1)
    private const val RT_CSTRING = 4026     // UTF-16LE

    // ---------------------------------------------------------------- .doc

    /** 提取 .doc 正文段落;null = 不是可识别/可解密的 Word 文档。 */
    fun docParagraphs(bytes: ByteArray): List<String>? = runCatching {
        val cfb = CfbReader(bytes)
        val word = cfb.openStream("WordDocument") ?: return@runCatching null
        if (word.size < FIB_LCBCLX + 4) return@runCatching null
        val flags = u16At(word, FIB_FLAGS)
        if (flags and 0x0100 != 0) return@runCatching null // 加密文档没有明文可提
        val ccpText = i32At(word, FIB_CCPTEXT)
        if (ccpText <= 0 || ccpText > MAX_DOC_CHARS) return@runCatching null

        val lcbClx = i32At(word, FIB_LCBCLX)
        val fcClx = i32At(word, FIB_FCCLX)
        val text = if (lcbClx in 8..MAX_CLX_BYTES && fcClx in 1 until Int.MAX_VALUE) {
            val tblName = if (flags and 0x0200 != 0) "1Table" else "0Table"
            val tbl = cfb.openStream(tblName)
                ?: cfb.openStream("1Table")
                ?: cfb.openStream("0Table")
            pieceTableText(word, tbl, fcClx, lcbClx, ccpText) ?: directText(word, ccpText)
        } else {
            directText(word, ccpText)
        } ?: return@runCatching null

        val paras = paragraphs(text)
        if (paras.isEmpty()) null else paras
    }.getOrNull()

    /** 简单(非 complex)文档:正文在 fcMin..fcMac 连续存放。 */
    private fun directText(word: ByteArray, ccpText: Int): String? {
        val fcMin = i32At(word, FIB_FCMIN)
        val fcMac = i32At(word, FIB_FCMAC)
        val byteLen = fcMac - fcMin
        if (fcMin < 0 || byteLen <= 0 || fcMin + byteLen > word.size) return null
        return if (byteLen == ccpText * 2) {
            String(word, fcMin, byteLen, Charsets.UTF_16LE)
        } else {
            String(word, fcMin, byteLen, Charsets.ISO_8859_1)
        }
    }

    /**
     * 复杂文档:CLX → Pcdt(0x02) → PlcPcd。CP 数组(n+1 项)+ n 个 PCD(8B):
     * PCD.fc 的 bit30 = 压缩位 —— 置位时字符 1 字节、偏移要除 2;否则 UTF-16LE 直取。
     */
    private fun pieceTableText(
        word: ByteArray,
        tbl: ByteArray?,
        fcClx: Int,
        lcbClx: Int,
        ccpText: Int,
    ): String? {
        if (tbl == null) return null
        var p = fcClx
        val end = minOf(fcClx + lcbClx, tbl.size)
        var plc = -1
        while (p in 0 until end) {
            when (tbl[p].toInt() and 0xFF) {
                0x01 -> { // Prc:跳过 cbGrpprl
                    if (p + 3 > end) return null
                    p += 3 + u16At(tbl, p + 1)
                }
                0x02 -> { // Pcdt
                    if (p + 5 > end) return null
                    plc = p + 5
                    val plcLen = i32At(tbl, p + 1)
                    return readPieces(word, tbl, plc, plcLen, ccpText)
                }
                else -> return null
            }
        }
        return null
    }

    private fun readPieces(word: ByteArray, tbl: ByteArray, plc: Int, plcLen: Int, ccpText: Int): String? {
        if (plc <= 0 || plcLen < 8 || plc + plcLen > tbl.size) return null
        val n = (plcLen - 4) / 12
        if (n <= 0) return null
        val sb = StringBuilder()
        val pd = plc + (n + 1) * 4
        for (i in 0 until n) {
            val cpStart = i32At(tbl, plc + i * 4)
            val cpEnd = i32At(tbl, plc + i * 4 + 4)
            if (cpEnd <= cpStart) continue
            if (cpStart >= ccpText) break // piece 表按 CP 升序,越过正文就是脚注/批注等
            val pcd = pd + i * 8
            if (pcd + 8 > tbl.size) break
            val fc = i32At(tbl, pcd + 2)
            val count = minOf(cpEnd, ccpText) - cpStart
            if ((fc and 0x40000000) != 0) {
                val off = (fc and 0x3FFFFFFF) / 2
                for (k in 0 until count) {
                    val o = off + k
                    if (o < 0 || o >= word.size) break
                    sb.append((word[o].toInt() and 0xFF).toChar())
                }
            } else {
                val off = fc and 0x3FFFFFFF
                for (k in 0 until count) {
                    val o = off + k * 2
                    if (o < 0 || o + 1 >= word.size) break
                    sb.append(String(word, o, 2, Charsets.UTF_16LE))
                }
            }
            if (sb.length >= MAX_DOC_CHARS) break
        }
        return if (sb.isEmpty()) null else sb.toString()
    }

    // ---------------------------------------------------------------- .ppt

    class PptSlide(val title: String, val bullets: List<String>)

    /** 提取 .ppt 每页(标题,要点);null = 不是可识别的 PowerPoint 文档。 */
    fun pptSlides(bytes: ByteArray): List<PptSlide>? = runCatching {
        val cfb = CfbReader(bytes)
        val stream = cfb.openStream("PowerPoint Document") ?: return@runCatching null
        val slides = pptSlidesFromStream(stream)
        if (slides.isEmpty()) null else slides
    }.getOrNull()

    /** 流级解析独立出来:单测可以直接喂合成字节,不必构造完整 OLE 容器。 */
    internal fun pptSlidesFromStream(stream: ByteArray): List<PptSlide> {
        val slides = ArrayList<SlideAcc>()
        walkPpt(stream, 0, stream.size, false, slides, PptCursor(), 0)
        return slides
            .filter { it.title.isNotBlank() || it.bullets.isNotEmpty() }
            .map { PptSlide(it.title, it.bullets) }
    }

    private class SlideAcc {
        var title = ""
        val bullets = ArrayList<String>()
    }

    /** TextHeaderAtom(3999) 的 recInstance 决定紧随其后的文字是标题还是正文。 */
    private class PptCursor {
        var pendingType = -1
    }

    private fun walkPpt(
        b: ByteArray,
        start: Int,
        end: Int,
        inSlide: Boolean,
        slides: MutableList<SlideAcc>,
        cur: PptCursor,
        depth: Int,
    ) {
        if (depth > 32) return
        var p = start
        while (p + 8 <= end) {
            val verInst = u16At(b, p)
            val type = u16At(b, p + 2)
            val len = i32At(b, p + 4)
            if (len < 0 || p + 8 + len > end) break
            val ds = p + 8
            if ((verInst and 0xF) == 0xF) {
                when (type) {
                    RT_SLIDE -> {
                        slides.add(SlideAcc())
                        walkPpt(b, ds, ds + len, true, slides, cur, depth + 1)
                    }
                    RT_NOTES, RT_MAINMASTER ->
                        // 备注/母版文字不进大纲,但仍要递归消费这段流
                        walkPpt(b, ds, ds + len, false, slides, cur, depth + 1)
                    else -> walkPpt(b, ds, ds + len, inSlide, slides, cur, depth + 1)
                }
            } else if (inSlide && slides.isNotEmpty()) {
                when (type) {
                    RT_TEXTHEADER -> cur.pendingType = verInst shr 4
                    RT_TEXTCHARS -> {
                        addText(slides.last(), cur.pendingType, utf16(b, ds, len))
                        cur.pendingType = -1
                    }
                    RT_TEXTBYTES -> {
                        addText(slides.last(), cur.pendingType, latin1(b, ds, len))
                        cur.pendingType = -1
                    }
                    RT_CSTRING -> {
                        addText(slides.last(), -1, utf16(b, ds, len))
                    }
                }
            }
            p = ds + len
        }
    }

    private fun utf16(b: ByteArray, off: Int, len: Int): String {
        val n = len and 1.inv() // TextCharsAtom 长度必为偶数
        val s = if (off + n <= b.size) String(b, off, n, Charsets.UTF_16LE) else ""
        return s
    }

    private fun latin1(b: ByteArray, off: Int, len: Int): String =
        if (off + len <= b.size) String(b, off, len, Charsets.ISO_8859_1) else ""

    private fun addText(acc: SlideAcc, textType: Int, raw: String) {
        // \r=段落 \u000B=软换行,其余控制符丢弃
        val cleaned = raw.map { ch ->
            when (ch) {
                '\r', '\u000B' -> '\n'
                '\u0001', '\u0008' -> null
                else -> ch
            }
        }
        val paras = cleaned.mapNotNull { it }.joinToString("")
            .split('\n').map { it.trim() }.filter { it.isNotEmpty() }
        if (paras.isEmpty()) return
        if (textType == 0) {
            if (acc.title.isBlank()) acc.title = paras.joinToString("  ")
        } else {
            acc.bullets.addAll(paras)
        }
    }

    // ---------------------------------------------------------------- 文字清理(.doc 共用)

    /**
     * 域字符状态机:\u0013..\u0014 是域代码(丢),\u0014..\u0015 是域结果(留)。
     * 之后把控制符映射成段落/换行:\r=\u0007=\u000C → 段落;\u000B → 换行;\u001E → 连字符。
     */
    internal fun paragraphs(raw: String): List<String> {
        val kept = StringBuilder(raw.length)
        val fields = ArrayDeque<Boolean>()
        for (ch in raw) {
            when (ch) {
                '\u0013' -> fields.addLast(false)
                '\u0014' -> if (fields.isNotEmpty()) fields[fields.lastIndex] = true
                '\u0015' -> fields.removeLastOrNull()
                else -> if (fields.isEmpty() || fields.last()) kept.append(ch)
            }
        }
        val flat = StringBuilder(kept.length)
        for (ch in kept) {
            when (ch) {
                '\r', '\u0007', '\u000C' -> flat.append('\n')
                '\u000B' -> flat.append('\n')
                '\u001E' -> flat.append('-')
                '\u001F', '\u0001', '\u0008', '\uFEFF', '\uFFFE' -> {}
                else -> flat.append(ch)
            }
        }
        return flat.split('\n').map { it.trim() }.filter { it.isNotEmpty() }
    }
}
