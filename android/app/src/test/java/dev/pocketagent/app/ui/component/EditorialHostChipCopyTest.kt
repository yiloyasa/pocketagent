package dev.pocketagent.app.ui.component

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 主机状态胶囊文案的纯函数单测(README §状态:未连接时状态仍显示上次更新时间,
 * 不能写"正在执行")。
 *
 * 【红线钉死】离线的副标题在**构造上**就接不到运行态:本组断言把三种链接态的
 * 文案全量过一遍,"正在执行"四个字在任何输入下都不得出现;离线必须带
 * 「上次更新」或明说「暂无更新记录」。
 */
class EditorialHostChipCopyTest {

    @Test
    fun `状态词三态钉死`() {
        assertEquals("已连接", hostStatusWord(PaHostLink.CONNECTED))
        assertEquals("连接中", hostStatusWord(PaHostLink.CONNECTING))
        assertEquals("离线", hostStatusWord(PaHostLink.OFFLINE))
    }

    @Test
    fun `已连接_展示实际采用的通道_没给就不编`() {
        assertEquals(
            "局域网 · 会话实时同步",
            hostChipCaption(PaHostLink.CONNECTED, "局域网 · 会话实时同步", "09:41"),
        )
        assertEquals("已连接", hostChipCaption(PaHostLink.CONNECTED, null, null))
        assertEquals("已连接", hostChipCaption(PaHostLink.CONNECTED, "  ", "09:41"))
    }

    @Test
    fun `连接中_中性措辞_可并列目标通道`() {
        assertEquals("正在建立连接…", hostChipCaption(PaHostLink.CONNECTING, null, null))
        assertEquals(
            "正在建立连接… · 局域网",
            hostChipCaption(PaHostLink.CONNECTING, "局域网", null),
        )
    }

    @Test
    fun `离线_必须带上次更新时间_没记录就明说`() {
        assertEquals("离线 · 上次更新 09:41", hostChipCaption(PaHostLink.OFFLINE, "局域网", "09:41"))
        assertEquals("离线 · 暂无更新记录", hostChipCaption(PaHostLink.OFFLINE, null, null))
        assertEquals("离线 · 暂无更新记录", hostChipCaption(PaHostLink.OFFLINE, "任意通道", "  "))
    }

    @Test
    fun `红线_任何输入下离线文案绝不含正在执行`() {
        // 离线模板忽略通道标注(传"正在执行"字样的通道也照删不误);
        // 时间槽按契约传时间文本,模板自身永不产出运行态措辞。
        val channelSamples = listOf("", " ", "局域网", "会话实时同步", "正在执行别的会话")
        val timeSamples = listOf(null, "", " ", "09:41", "昨天 18:40")
        for (channel in channelSamples) {
            for (update in timeSamples) {
                val caption = hostChipCaption(PaHostLink.OFFLINE, channel, update)
                assertFalse("离线文案不得出现「正在执行」: $caption", caption.contains("正在执行"))
                assertFalse("离线文案不得出现「正在运行」: $caption", caption.contains("正在运行"))
                assertTrue(
                    "离线文案必须交代更新时间: $caption",
                    caption.contains("上次更新") || caption.contains("暂无更新记录"),
                )
                assertTrue("离线文案必须以「离线」开头: $caption", caption.startsWith("离线"))
            }
        }
    }

    @Test
    fun `读屏语义一次念全_名称加状态词加副标题`() {
        val a11y = hostChipA11y("工作站", PaHostLink.CONNECTED, "局域网 · 会话实时同步")
        assertTrue(a11y.contains("工作站"))
        assertTrue(a11y.contains("已连接"))
        assertTrue(a11y.contains("局域网"))
        assertEquals(
            "主机 工作站，已连接，局域网 · 会话实时同步",
            a11y,
        )
    }
}
