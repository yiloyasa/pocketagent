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

    // ---- cancelTurnFailureCopy(R1 残项收口:终止指令没送达的横幅文案)----

    @Test
    fun `终止未送达_两档文案都给下一步_指向真实存在的终止与重试`() {
        val e = java.io.IOException("connection refused")
        listOf(
            cancelTurnFailureCopy(timedOut = false, e = e),
            cancelTurnFailureCopy(timedOut = true, e = e),
        ).forEach { text ->
            assertTrue("下一步要指向还能按的「终止」: $text", text.contains("终止"))
            assertTrue("下一步要指向提示条上的「重试」: $text", text.contains("重试"))
        }
    }

    @Test
    fun `终止未送达_模板本体不用失败未知错误裸词`() {
        // detail 传干净异常,保证断言只钉模板用词(原文透传是纪律第一档,不受此限)
        val e = java.io.IOException("connection refused")
        listOf(
            cancelTurnFailureCopy(timedOut = false, e = e),
            cancelTurnFailureCopy(timedOut = true, e = e),
        ).forEach { text ->
            assertFalse("不得含「失败」裸词: $text", text.contains("失败"))
            assertFalse("不得含「未知」: $text", text.contains("未知"))
            assertFalse("不得含「错误」裸词: $text", text.contains("错误"))
        }
    }

    @Test
    fun `终止超时与未送达是两种陈述_不替电脑下结论`() {
        val e = java.io.IOException("connection refused")
        val plain = cancelTurnFailureCopy(timedOut = false, e = e)
        val timeout = cancelTurnFailureCopy(timedOut = true, e = e)
        assertTrue("未送达档要陈述任务可能还在运行: $plain", plain.contains("任务可能仍在运行"))
        assertTrue("超时档不下结论_两可能都讲: $timeout", timeout.contains("可能已生效也可能还在运行"))
        assertFalse(plain.contains("没有回应"))
    }

    @Test
    fun `终止文案_异常原文按humanize三档透传`() {
        assertTrue(
            "有原文透传原文: ${cancelTurnFailureCopy(false, java.io.IOException("端口 8718 连不上"))}",
            cancelTurnFailureCopy(false, java.io.IOException("端口 8718 连不上")).contains("端口 8718 连不上"),
        )
        // message 为空落类名档,不出空白括号
        assertTrue(
            cancelTurnFailureCopy(false, java.io.IOException("")).contains("错误类型：IOException"),
        )
        // 无异常落人话兜底
        assertTrue(cancelTurnFailureCopy(true, null).contains("出了点问题，请重试"))
    }

    // ---- approvalAnswerFailureCopy(R1 残项收口:三选项应答没送达的原因)----

    @Test
    fun `审批应答未送达_给重试下一步_与弹窗重试按钮对齐`() {
        val text = approvalAnswerFailureCopy(java.io.IOException("通道已关闭"))
        assertTrue("要点名「重试」: $text", text.contains("重试"))
        assertTrue("要点名「再交一次」: $text", text.contains("再交一次"))
        assertTrue("原文透传: $text", text.contains("通道已关闭"))
    }

    @Test
    fun `审批应答未送达_message为空时透传类名_不出空白括号`() {
        val text = approvalAnswerFailureCopy(java.io.IOException("  "))
        assertTrue(text.contains("错误类型：IOException"))
        assertFalse(text.contains("（）"))
    }

    @Test
    fun `审批应答未送达_模板本体不用失败未知裸词`() {
        // detail 传干净异常:断言只钉模板用词(「错误类型：X」是 humanizeError 的
        // 既定类名档,不属裸词,见本文件顶部三档口径)
        val text = approvalAnswerFailureCopy(java.io.IOException("timeout"))
        assertFalse(text.contains("未知"))
        assertFalse(text.contains("失败"))
    }
}
