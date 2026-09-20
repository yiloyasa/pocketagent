package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.IOException

/**
 * [humanizeError] 的单测(AUDIT §3.3 限期项 P2-3 的回归资产):
 * 异常文案三档口径 —— 有 message 透传原文;message 空透传异常类名;
 * 无异常给人话。**任何一档都不得含「未知」字样**(grep 自查之外的测试钉死)。
 */
class ErrorCopyTest {

    @Test
    fun `有message时透传原文_不加工不吞掉`() {
        val e = IOException("连接电脑失败:connection refused")
        assertEquals("连接电脑失败:connection refused", humanizeError(e))
    }

    @Test
    fun `message为空时透传异常类名_给人话口径`() {
        // 空串与全空白一样处理:类名是可核查的真实信息
        assertEquals("错误类型：IOException", humanizeError(IOException("")))
        assertEquals("错误类型：IllegalStateException", humanizeError(IllegalStateException("   ")))
    }

    @Test
    fun `message为null时透传异常类名`() {
        // 构造 message=null 的异常:落类名档,不能 NPE、不能显示"未知错误"
        val e = IOException(null as String?)
        assertEquals("错误类型：IOException", humanizeError(e))
    }

    @Test
    fun `无异常时给人话兜底`() {
        assertEquals("出了点问题，请重试", humanizeError(null))
    }

    @Test
    fun `三档文案都不含未知字样_AUDIT_3_3红线`() {
        val candidates = listOf(
            humanizeError(RuntimeException("磁盘写满")),
            humanizeError(RuntimeException("")),
            humanizeError(RuntimeException(null as String?)),
            humanizeError(null),
        )
        candidates.forEach { text ->
            assertFalse("用户可见文案不得含「未知」: $text", text.contains("未知"))
            assertTrue("文案不得为空白: $text", text.isNotBlank())
        }
    }
}
