package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.ByteArrayOutputStream

/**
 * OfficeLegacy 纯流级单测(不需要 Context):
 * 1. 合成 .ppt 记录流 → 验证分页 / 标题(TextHeaderAtom recInstance=0)/ 要点 / 单字节文本 /
 *    CString / 备注(Notes)与母版(MainMaster)排除;
 * 2. .doc 共用的 paragraphs() 域字符状态机与控制符映射。
 *
 * 注:所有控制符用 Char(0x..) 构造 —— 源码里不写反斜杠 u 转义,
 * (写入工具曾把转义序列变成真实控制字节,文件直接损坏)。
 */
class OfficeLegacyTest {

    // ---------------- 合成 PPT 记录流的工具 ----------------

    private fun le16(v: Int, out: ByteArrayOutputStream) {
        out.write(v and 0xFF)
        out.write((v shr 8) and 0xFF)
    }

    private fun le32(v: Int, out: ByteArrayOutputStream) {
        le16(v and 0xFFFF, out)
        le16((v shr 16) and 0xFFFF, out)
    }

    /** 8 字节记录头(verInstance 2B + recordType 2B + length 4B)+ payload。 */
    private fun rec(type: Int, verInst: Int, body: ByteArray): ByteArray {
        val out = ByteArrayOutputStream()
        le16(verInst, out)
        le16(type, out)
        le32(body.size, out)
        out.write(body)
        return out.toByteArray()
    }

    /** 容器记录:ver=0xF(低 4 位),instance=0。 */
    private fun container(type: Int, children: List<ByteArray>): ByteArray =
        rec(type, 0x000F, children.reduce { a, b -> a + b })

    private fun textHeader(instance: Int): ByteArray = rec(3999, instance shl 4, ByteArray(4))
    private fun textChars(s: String): ByteArray = rec(4000, 0, s.toByteArray(Charsets.UTF_16LE))
    private fun textBytes(s: String): ByteArray = rec(4008, 0, s.toByteArray(Charsets.ISO_8859_1))
    private fun cstring(s: String): ByteArray = rec(4026, 0, s.toByteArray(Charsets.UTF_16LE))

    // ---------------- .ppt 流级解析 ----------------

    @Test
    fun pptStreamParsesSlidesTitlesBulletsAndExcludesNotesMaster() {
        val slide1 = container(
            1006,
            listOf(
                textHeader(0), textChars("2026 Q3 复盘"),
                textHeader(1), textChars("要点一" + Char(0x0D) + "要点二"),
                textHeader(1), textBytes("latin bullet"),
            ),
        )
        val master = container(1016, listOf(textChars("母版不该出现")))
        val notes = container(1008, listOf(textChars("备注不该出现")))
        val slide2 = container(
            1006,
            listOf(
                textHeader(0), textBytes("Slide Two"),
                cstring("页脚文本"),
            ),
        )
        val stream = slide1 + master + notes + slide2

        val slides = OfficeLegacy.pptSlidesFromStream(stream)

        assertEquals(2, slides.size)
        assertEquals("2026 Q3 复盘", slides[0].title)
        assertEquals(listOf("要点一", "要点二", "latin bullet"), slides[0].bullets)
        assertEquals("Slide Two", slides[1].title)
        assertEquals(listOf("页脚文本"), slides[1].bullets)
        // 备注/母版文字绝不进大纲
        val allText = slides.map { it.title } + slides.flatMap { it.bullets }
        assertTrue(allText.none { it.contains("备注") || it.contains("母版") })
    }

    @Test
    fun pptStreamSurvivesGarbageAndTruncatedInput() {
        assertTrue(OfficeLegacy.pptSlidesFromStream(ByteArray(5)).isEmpty())
        // 长度声明超过流边界 → 该记录被丢弃,不抛异常
        val out = ByteArrayOutputStream()
        le16(0x000F, out); le16(1006, out); le32(9999, out)
        assertTrue(OfficeLegacy.pptSlidesFromStream(out.toByteArray()).isEmpty())
        // 无标题无要点的空页被过滤
        val emptySlide = container(1006, listOf(textChars("   ")))
        assertTrue(OfficeLegacy.pptSlidesFromStream(emptySlide).isEmpty())
    }

    @Test
    fun pptTitleBodyHeaderOnSameSlideSplitsCorrectly() {
        // 标题在正文之后出现(乱序也按 pendingType 归位)
        val slide = container(
            1006,
            listOf(
                textHeader(1), textChars("第一条"),
                textHeader(0), textChars("真标题"),
                textHeader(1), textChars("第二条"),
            ),
        )
        val slides = OfficeLegacy.pptSlidesFromStream(slide)
        assertEquals(1, slides.size)
        assertEquals("真标题", slides[0].title)
        assertEquals(listOf("第一条", "第二条"), slides[0].bullets)
    }

    // ---------------- .doc 段落清理(域状态机 + 控制符映射) ----------------

    @Test
    fun paragraphsKeepsFieldResultsDropsFieldCodes() {
        // 13(域开始) PAGE 14(分隔) 3/4(结果,保留) 15(域结束)
        val raw = "A" + Char(0x13) + "PAGE" + Char(0x14) + "3/4" + Char(0x15) + "B"
        assertEquals(listOf("A3/4B"), OfficeLegacy.paragraphs(raw))
    }

    @Test
    fun paragraphsMapsControlCharsToLines() {
        val cr = Char(0x0D)   // \r → 段落
        val ff = Char(0x0C)   // 分页符 → 段落
        val vt = Char(0x0B)   // 软换行 → 换行
        val hy = Char(0x1E)   // 不可断连字符 → -
        val oh = Char(0x1F)   // 可断连字符 → 丢
        val p1 = Char(0x01)   // 图片/对象占位 → 丢
        val p8 = Char(0x08)   // 同上
        val raw = "标题${cr}分页${ff}软${vt}连${hy}接${oh}去${p1}尾${p8}"
        assertEquals(listOf("标题", "分页", "软", "连-接去尾"), OfficeLegacy.paragraphs(raw))
    }

    @Test
    fun paragraphsHandlesUnbalancedField() {
        // 只有域开始没有结果/结束:域内内容全部丢弃,不吞掉前面的正文、不越界
        assertEquals(listOf("x"), OfficeLegacy.paragraphs("x" + Char(0x13) + "代码无结果"))
    }
}
