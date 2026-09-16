package dev.pocketagent.app.ui

/**
 * OLE2 复合文档(CFB,MS-CFB)读取器 —— .xls/.doc/.ppt 三种旧格式的公共底座。
 *
 * 【为什么独立成文件】原来藏在 XlsLegacy 里只有 .xls 在用;现在 .doc/.ppt
 * 也要从 OLE2 容器里取流(WordDocument / PowerPoint Document),
 * 三份拷贝必然走偏,抽出来一处维护。
 *
 * 只需要"按名字取出一个流",所以省掉写支持与目录树遍历 —— 直接扫平铺的目录项。
 */
internal class CfbReader(private val d: ByteArray) {
    private val sectorSize: Int
    private val miniSectorSize: Int
    private val miniCutoff: Int
    private val fat: IntArray
    private val miniFat: IntArray
    private val entries = ArrayList<Entry>()
    private val miniStream: ByteArray

    class Entry(val name: String, val type: Int, val start: Int, val size: Long)

    init {
        require(d.size >= 512) { "文件太小" }
        val magic = intArrayOf(0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1)
        for (k in magic.indices) require((d[k].toInt() and 0xFF) == magic[k]) { "不是 OLE2 复合文档" }
        val sShift = u16(0x1E)
        require(sShift in 7..12) { "非法扇区大小" }
        sectorSize = 1 shl sShift
        miniSectorSize = 1 shl u16(0x20)
        miniCutoff = i32(0x38)
        val fatCount = i32(0x2C)
        val dirStart = i32(0x30)
        val miniFatStart = i32(0x3C)
        val miniFatCount = i32(0x40)
        val difatStart = i32(0x44)
        val difatCount = i32(0x48)

        // ---- DIFAT → FAT 扇区列表 ----
        val fatSecs = ArrayList<Int>()
        for (k in 0 until 109) {
            val sid = i32(0x4C + k * 4)
            if (sid >= 0) fatSecs.add(sid)
        }
        var difat = difatStart
        var guard = 0
        while (difat >= 0 && guard++ <= difatCount + 1) {
            val base = sectorOffset(difat)
            val per = sectorSize / 4 - 1
            for (k in 0 until per) {
                val sid = i32(base + k * 4)
                if (sid >= 0) fatSecs.add(sid)
            }
            difat = i32(base + per * 4)
        }

        // ---- FAT ----
        val per = sectorSize / 4
        val fatArr = IntArray(maxOf(0, fatCount) * per)
        var w = 0
        for (s in fatSecs) {
            if (w + per > fatArr.size) break
            val base = sectorOffset(s)
            for (k in 0 until per) fatArr[w++] = i32(base + k * 4)
        }
        fat = fatArr

        // ---- MiniFAT ----
        val mf = ArrayList<Int>()
        if (miniFatStart >= 0 && miniFatCount > 0) {
            for (s in chain(miniFatStart, -1)) {
                val base = sectorOffset(s)
                for (k in 0 until per) mf.add(i32(base + k * 4))
            }
        }
        miniFat = mf.toIntArray()

        // ---- 目录项 ----
        val dirBytes = readChain(dirStart, -1)
        var p = 0
        while (p + 128 <= dirBytes.size) {
            val nameLen = u16At(dirBytes, p + 0x40)
            val type = dirBytes[p + 0x42].toInt() and 0xFF
            val start = i32At(dirBytes, p + 0x74)
            val size = i64At(dirBytes, p + 0x78)
            if (type != 0 && nameLen in 2..64) {
                val nm = String(dirBytes, p, nameLen - 2, Charsets.UTF_16LE)
                entries.add(Entry(nm, type, start, size))
            }
            p += 128
        }

        // ---- 迷你流(根目录项描述的流) ----
        val root = entries.firstOrNull { it.type == 5 }
        miniStream = if (root != null && root.size > 0) readChain(root.start, root.size) else ByteArray(0)
    }

    fun openStream(name: String): ByteArray? {
        val e = entries.firstOrNull { it.name.equals(name, ignoreCase = true) && it.type == 2 } ?: return null
        return if (e.size < miniCutoff) readMini(e.start, e.size) else readChain(e.start, e.size)
    }

    private fun sectorOffset(sid: Int): Int = 512 + sid * sectorSize

    /** 沿 FAT 链读取(limit<0 表示读到链尾)。 */
    private fun chain(start: Int, limit: Long): List<Int> {
        val out = ArrayList<Int>()
        var sid = start
        var guard = 0
        val budget = if (limit > 0) ((limit / sectorSize) + 2) else Int.MAX_VALUE.toLong()
        while (sid >= 0 && sid < fat.size && guard++ < 1_000_000 && out.size <= budget) {
            out.add(sid)
            sid = fat[sid]
        }
        return out
    }

    private fun readChain(start: Int, size: Long): ByteArray {
        val secs = chain(start, size)
        val bos = java.io.ByteArrayOutputStream()
        for (s in secs) {
            val base = sectorOffset(s)
            if (base < 0 || base >= d.size) break
            bos.write(d, base, minOf(sectorSize, d.size - base))
            if (size > 0 && bos.size() >= size) break
        }
        val b = bos.toByteArray()
        return if (size in 1..b.size.toLong()) b.copyOf(size.toInt()) else b
    }

    private fun readMini(start: Int, size: Long): ByteArray {
        val bos = java.io.ByteArrayOutputStream()
        var sid = start
        var guard = 0
        val limit = (size / miniSectorSize + 2).toInt()
        while (sid >= 0 && sid < miniFat.size && guard++ < 1_000_000 && guard <= limit) {
            val base = sid * miniSectorSize
            if (base >= miniStream.size) break
            bos.write(miniStream, base, minOf(miniSectorSize, miniStream.size - base))
            if (bos.size() >= size) break
            sid = miniFat[sid]
        }
        val b = bos.toByteArray()
        return if (size in 1..b.size.toLong()) b.copyOf(size.toInt()) else b
    }

    private fun u16(o: Int) = u16At(d, o)
    private fun i32(o: Int) = i32At(d, o)

    private fun u16At(b: ByteArray, o: Int): Int =
        if (o + 1 < b.size) (b[o].toInt() and 0xFF) or ((b[o + 1].toInt() and 0xFF) shl 8) else -1

    private fun i32At(b: ByteArray, o: Int): Int =
        if (o + 3 < b.size) {
            (b[o].toInt() and 0xFF) or ((b[o + 1].toInt() and 0xFF) shl 8) or
                ((b[o + 2].toInt() and 0xFF) shl 16) or ((b[o + 3].toInt() and 0xFF) shl 24)
        } else {
            -1
        }

    private fun i64At(b: ByteArray, o: Int): Long {
        if (o + 7 >= b.size) return -1
        var v = 0L
        for (k in 7 downTo 0) v = (v shl 8) or (b[o + k].toLong() and 0xFF)
        return v
    }
}

/** 无符号读取的小工具,.doc/.ppt 解析共用。 */
internal fun u16At(b: ByteArray, o: Int): Int =
    if (o + 1 < b.size) (b[o].toInt() and 0xFF) or ((b[o + 1].toInt() and 0xFF) shl 8) else 0

internal fun i32At(b: ByteArray, o: Int): Int =
    if (o + 3 < b.size) {
        (b[o].toInt() and 0xFF) or ((b[o + 1].toInt() and 0xFF) shl 8) or
            ((b[o + 2].toInt() and 0xFF) shl 16) or ((b[o + 3].toInt() and 0xFF) shl 24)
    } else {
        0
    }
