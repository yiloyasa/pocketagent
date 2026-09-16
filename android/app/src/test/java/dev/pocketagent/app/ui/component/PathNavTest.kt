package dev.pocketagent.app.ui.component

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

/**
 * 目录层级回退规则的回归测试 —— 对应真机发现的「返回键完全失灵」。
 *
 * 背景:文件页与文件夹选择器各自实现过一份"往上退一层",两份都写了
 * `idx == 2 -> loadDir(norm + "\\")` 这个特判。对"盘符直下一级"(如 `G:\Zcode`)
 * 来说 `lastIndexOf('\')` 恰好等于 2,于是"返回"变成"原地重载自己";更糟的是
 * 函数仍然 `return true` 把返回事件吃掉 —— 表现就是在盘符根的子目录里,
 * 无论点顶栏返回还是按系统返回,都永远回不到上一层(死锁)。
 *
 * 现在规则收敛到 [parentDirectoryOf] 一处。这些用例锁住边界语义,防止回归。
 */
class PathNavTest {

    @Test
    fun `盘符直下一级 应退到盘根`() {
        // 这就是踩坑的那一档:修复前会原地重载 G:\Zcode 自己
        assertEquals("G:\\", parentDirectoryOf("G:\\Zcode"))
    }

    @Test
    fun `深层目录 应只退一层`() {
        assertEquals("G:\\a\\", parentDirectoryOf("G:\\a\\b"))
        assertEquals("G:\\a\\b\\", parentDirectoryOf("G:\\a\\b\\c"))
    }

    @Test
    fun `带尾部分隔符 结果一致`() {
        // 盘根只有一段;"G:\a\" 与 "G:\a" 必须给出同一个上级
        assertEquals("G:\\", parentDirectoryOf("G:\\a\\"))
        assertEquals("G:\\a\\", parentDirectoryOf("G:\\a\\b\\"))
    }

    @Test
    fun `盘根 应返回 null 表示已到顶`() {
        // null 的语义 = 调用方改为显示盘符列表,并把返回事件交给下一级
        assertNull(parentDirectoryOf("G:\\"))
        assertNull(parentDirectoryOf("C:\\"))
    }

    @Test
    fun `正斜杠路径 不应被当成反斜杠切分`() {
        // Unix 风格:不能出现把 '/' 换成 '\' 再切的老 bug
        assertEquals("/home/", parentDirectoryOf("/home/user"))
        assertEquals("/", parentDirectoryOf("/home"))
        assertNull(parentDirectoryOf("/"))
    }

    @Test
    fun `空路径 返回 null`() {
        assertNull(parentDirectoryOf(""))
    }

    @Test
    fun `中文与空格目录名 应原样保留`() {
        // 路径里的非 ASCII 内容必须逐字符保留,不能在拼接上级时被动过
        assertEquals("G:\\", parentDirectoryOf("G:\\Zcode 文件"))
        assertEquals("G:\\Zcode 文件\\", parentDirectoryOf("G:\\Zcode 文件\\143"))
        assertEquals("G:\\试验区\\outputs\\", parentDirectoryOf("G:\\试验区\\outputs\\abc-123"))
    }

    @Test
    fun `连续上退应最终收敛到 null 而不是死循环`() {
        // 锁住"一定能走到底"这个性质:修复前的实现会在这里原地打转
        var p: String? = "G:\\Zcode 文件\\143"
        val seen = mutableListOf<String>()
        var guardCount = 0
        while (p != null) {
            seen += p
            p = parentDirectoryOf(p)
            check(++guardCount < 16) { "层级回退未能收敛,疑似死循环" }
        }
        assertEquals(
            listOf("G:\\Zcode 文件\\143", "G:\\Zcode 文件\\", "G:\\"),
            seen,
        )
    }
}
