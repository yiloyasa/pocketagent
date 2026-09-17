package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 文件名"中间省略"规则(middleEllipsize)。
 *
 * 关键不变量:省略时必须**保住尾部扩展名** —— 用户靠扩展名判断"这是什么文件",
 * 把 `报告-2026-09-17-final-v3.docx` 显示成 `报告-2026-09-17-fin…` 等于把信息删了。
 */
class WorkflowAFilenameTest {

    /** 恰好等于 max / 短于 max 时,原样返回(不改动)。 */
    @Test
    fun keeps_names_within_budget_untouched() {
        assertEquals("a.txt", middleEllipsize("a.txt", 5))
        assertEquals("a.txt", middleEllipsize("a.txt", 99))
        assertEquals("photo.jpg", middleEllipsize("photo.jpg", 9))
        assertEquals("短名.png", middleEllipsize("短名.png", 100))
    }

    /** 超长名:省略中间段,但必须保留扩展名。 */
    @Test
    fun keeps_full_extension_when_ellipsizing() {
        val out = middleEllipsize("报告-2026-09-17-final-v3.docx", 20)
        assertEquals(20, out.length)
        assertTrue("必须保住扩展名", out.endsWith(".docx"))
        assertFalse("必须真正省略掉中间段", out.contains("final"))
    }

    /** 多段扩展名 tar.gz 要整段保留。 */
    @Test
    fun preserves_compound_extension_tar_gz() {
        val out = middleEllipsize("archive.tar.gz", 10)
        assertEquals(10, out.length)
        assertTrue(out.endsWith(".tar.gz"))
    }

    /** 无扩展名:退化为"头部 + …",不伪造一个点。 */
    @Test
    fun handles_names_without_extension() {
        val out = middleEllipsize("averylongfilenamewithnoextension", 12)
        assertEquals(12, out.length)
        assertTrue(out.endsWith("…"))
        assertFalse(out.contains("."))
    }

    /** 含中文:按字符截断,扩展名照常保留。 */
    @Test
    fun handles_chinese_names() {
        val out = middleEllipsize("中文名称很长的文档.docx", 12)
        assertEquals(12, out.length)
        assertTrue(out.endsWith(".docx"))
    }

    /** 极小预算:不能崩,退化到"头部 + …";非法 max 返回空串。 */
    @Test
    fun tiny_budget_degrades_gracefully() {
        assertEquals("", middleEllipsize("whatever.txt", 0))
        assertEquals("", middleEllipsize("whatever.txt", -3))
        assertEquals("…", middleEllipsize("whatever.txt", 1))
        val out = middleEllipsize("whatever.txt", 3)
        assertEquals(3, out.length)
        assertTrue(out.endsWith("…"))
    }
}
