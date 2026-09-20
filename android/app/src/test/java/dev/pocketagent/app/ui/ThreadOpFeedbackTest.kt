package dev.pocketagent.app.ui

import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 会话操作失败反馈文案的单测(T-05)。
 *
 * 【背景】删除/归档的"真删提交"跑在 undo 倒计时结束后的后台协程里,失败以前
 * 被 UndoBar 的 runCatching 整个吞掉 —— 会话"凭空消失",用户毫无察觉。修复后
 * 提交失败弹人话 Toast;本用例钉住文案规则:
 *  1. 操作名与 daemon 给的人话原因都必须在场;
 *  2. daemon 没给原因(连接类异常)时落到"连接失败";
 *  3. 任何情况下不出现"未知"字眼(AUDIT §3.3-3:UI 禁止显示"未知")。
 */
class ThreadOpFeedbackTest {

    @Test
    fun `带原因时原样拼入daemon人话`() {
        val text = threadOpFailureText("删除", "会话正在运行，请先停止任务再删除或移除")
        assertTrue(text.contains("删除"))
        assertTrue(text.contains("会话正在运行"))
    }

    @Test
    fun `归档与恢复各用各的操作名`() {
        assertTrue(threadOpFailureText("归档", "x").startsWith("归档失败"))
        assertTrue(threadOpFailureText("恢复", "x").startsWith("恢复失败"))
    }

    @Test
    fun `无原因或空白原因落到连接失败`() {
        assertTrue(threadOpFailureText("删除", null).endsWith("连接失败"))
        assertTrue(threadOpFailureText("删除", "").endsWith("连接失败"))
        assertTrue(threadOpFailureText("删除", "   ").endsWith("连接失败"))
    }

    @Test
    fun `任何情况不显示未知字眼`() {
        for (detail in listOf<String?>(null, "", "io timeout", "UNAVAILABLE")) {
            val text = threadOpFailureText("删除", detail)
            assertFalse(text.contains("未知"))
        }
    }
}
