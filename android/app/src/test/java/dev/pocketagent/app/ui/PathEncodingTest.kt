package dev.pocketagent.app.ui

import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.DirectoryEntry
import java.nio.charset.StandardCharsets

/**
 * 中文路径在 proto 契约层的编码回归测试 —— 对应真机发现的「目录名显示成 锟困欢」。
 *
 * 排查结论(已用探针实测):
 *  1. 电脑磁盘上的真实目录名是 `G:\Zcode 文件`(码点 U+6587 U+4EF6),**不是**乱码;
 *  2. daemon 直连探针取回的名字字节是 `5a 63 6f 64 65 20 e6 96 87 e4 bb b6`,
 *     即标准 UTF-8 的 "Zcode 文件" —— daemon 侧 `os.ReadDir` + protobuf 完全正确;
 *  3. Android 侧全部 charset 用法只作用于**文件内容**解码(Office/CFB/预览),
 *     路径链路没有任何转码。
 *
 * 所以这里用代码把这个层面钉死:只要这些用例是绿的,就证明"名字从 daemon 到
 * Kotlin 对象"这段路不会篡改 UTF-8,乱码若再出现,只可能来自渲染层或旧安装包。
 */
class PathEncodingTest {

    /** daemon 探针实测回来的原始字节(见本文件头注释第 2 点)。 */
    private val daemonObservedName = byteArrayOf(
        0x5a, 0x63, 0x6f, 0x64, 0x65, 0x20,                                   // "Zcode "
        0xe6.toByte(), 0x96.toByte(), 0x87.toByte(),                          // 文
        0xe4.toByte(), 0xbb.toByte(), 0xb6.toByte(),                          // 件
    )

    @Test
    fun `daemon 实测字节 按 UTF-8 解释即为正确中文`() {
        assertEquals("Zcode 文件", String(daemonObservedName, StandardCharsets.UTF_8))
    }

    @Test
    fun `DirectoryEntry 序列化后 中文名字节不被改写`() {
        val entry = DirectoryEntry.newBuilder()
            .setName("Zcode 文件")
            .setPath("G:\\Zcode 文件")
            .setIsDir(true)
            .build()

        val wire = entry.toByteArray()
        // 序列化结果里必须能找到与 daemon 一致的 UTF-8 字节序列
        assertTrue(
            "序列化结果中未找到期望的中文 UTF-8 字节",
            wire.indexOfSubArray(daemonObservedName) >= 0,
        )
    }

    @Test
    fun `中文路径往返解析 完全无损`() {
        val original = DirectoryEntry.newBuilder()
            .setName("Zcode 文件")
            .setPath("G:\\Zcode 文件\\143")
            .setIsDir(true)
            .setSizeBytes(4096)
            .build()

        val restored = DirectoryEntry.parseFrom(original.toByteArray())

        assertEquals(original.name, restored.name)
        assertEquals(original.path, restored.path)
        assertArrayEquals(
            original.path.toByteArray(StandardCharsets.UTF_8),
            restored.path.toByteArray(StandardCharsets.UTF_8),
        )
    }

    @Test
    fun `多语言与特殊字符组合 往返后仍一致`() {
        // 文件页标题取的是 substringAfterLast('\'),这里顺带锁住末段不会被截坏
        val name = "试验区 outputs 01a09b09-35bb 中文+English·符号"
        val entry = DirectoryEntry.newBuilder()
            .setName(name)
            .setPath("G:\\试验区\\$name")
            .build()
        val restored = DirectoryEntry.parseFrom(entry.toByteArray())
        assertEquals(name, restored.name)
        assertEquals(name, restored.path.substringAfterLast('\\'))
    }

    /** 在字节数组里查子序列;找不到返回 -1。 */
    private fun ByteArray.indexOfSubArray(target: ByteArray): Int {
        if (target.isEmpty() || target.size > size) return -1
        outer@ for (i in 0..(size - target.size)) {
            for (j in target.indices) {
                if (this[i + j] != target[j]) continue@outer
            }
            return i
        }
        return -1
    }
}
