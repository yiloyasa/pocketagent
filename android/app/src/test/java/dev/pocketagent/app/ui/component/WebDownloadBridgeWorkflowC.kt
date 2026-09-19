package dev.pocketagent.app.ui.component

import org.junit.Assert.assertEquals
import org.junit.Test

/**
 * 项7 · 安全纵深防御:`WebDownloadBridge.safeName` 的文件名净化。
 *
 * 【为什么单测它】这一环是"JS 桥暴露面收窄"改动里唯一没有测试覆盖的点,审核时它必须能自证:
 * 即便上游的"可信来源"判定被绕过,桥内落盘名也不能带目录前缀、`..` 穿越或控制字符。
 *
 * `safeName` 在 companion 内,是纯函数 —— 无需 Android `Context` 即可在 JVM 直接验证。
 */
class WebDownloadBridgeWorkflowC {

    @Test
    fun safeNameStripsPathsAndControlChars() {
        // 目录前缀与 ".." 一并剥掉,只留末段
        assertEquals("c.txt", WebDownloadBridge.safeName("a/b/../c.txt"))
        // 路径穿越:../../etc/passwd 只能落到 passwd
        assertEquals("passwd", WebDownloadBridge.safeName("../../etc/passwd"))
        // Windows 反斜杠路径同样按 '/' 归并后取末段
        assertEquals("evil.exe", WebDownloadBridge.safeName("C:\\Users\\x\\..\\evil.exe"))
        // 空 / 纯空白 → 默认名
        assertEquals("下载文件", WebDownloadBridge.safeName(""))
        assertEquals("下载文件", WebDownloadBridge.safeName("   "))
        // 控制字符(NUL / 换行)被剥掉;源码用转义写法,不写真实 NUL 字节
        assertEquals("abc.txt", WebDownloadBridge.safeName("a\u0000b\nc.txt"))
        // 超长名截到 120
        assertEquals(120, WebDownloadBridge.safeName("x".repeat(300)).length)
        // 正常名原样保留
        assertEquals("报告.pdf", WebDownloadBridge.safeName("报告.pdf"))
    }
}
