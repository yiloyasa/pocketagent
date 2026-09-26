package dev.pocketagent.app.ui.component

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 断连全局横幅文案(纯函数)的单测(审计 P1-2)。
 *
 * 口径:连接已断开要报「正在重连(第 N 次)」,有排队输入要如实报条数 ——
 * 用户最怕的是"我发的消息是不是丢了",排队数必须出现在横幅上;没有排队
 * 就不摆"0 条排队中"的空壳。文案禁「未知」「错误」(AUDIT §3.3)。
 */
class ConnectionBannerCopyTest {

    @Test
    fun `断连首次探测中_不带次数不带排队`() {
        val copy = connectionBannerCopy(reconnectAttempt = 0, queuedInputs = 0)
        assertEquals("连接已断开，正在重连…", copy)
        assertFalse(copy.contains("第"))
    }

    @Test
    fun `断连第N次_带次数`() {
        val copy = connectionBannerCopy(reconnectAttempt = 3, queuedInputs = 0)
        assertEquals("连接已断开，正在重连（第 3 次）…", copy)
    }

    @Test
    fun `有排队输入_如实报条数`() {
        val copy = connectionBannerCopy(reconnectAttempt = 2, queuedInputs = 5)
        assertEquals("连接已断开，正在重连（第 2 次）… 5 条排队中", copy)
    }

    @Test
    fun `负数入参按零处理_不出现负号`() {
        assertFalse(connectionBannerCopy(-1, -2).contains("-"))
    }

    @Test
    fun `文案不含禁词`() {
        val copy = connectionBannerCopy(4, 1)
        assertFalse(copy.contains("未知"))
        assertFalse(copy.contains("错误"))
        assertTrue(copy.contains("连接已断开"))
    }
}
