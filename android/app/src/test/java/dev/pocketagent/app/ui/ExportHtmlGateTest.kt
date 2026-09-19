package dev.pocketagent.app.ui

import io.grpc.Status
import io.grpc.StatusRuntimeException
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 导出 HTML 存档按钮(Pi-only)的门控与失败文案纯函数单测。
 * 背景:真机上该按钮曾出现"点击零反馈"——修复要求任何点击都必须有反馈:
 * pi 会话发 RPC,非 pi 会话本地人话拒绝,未知交给 daemon 裁决(永不无声)。
 */
class ExportHtmlGateTest {

    // ---- htmlExportGate(会话归属 agent → 按钮行为分支)----

    @Test
    fun `pi会话_发RPC`() {
        assertEquals(HtmlExportGate.SUPPORTED, htmlExportGate("pi"))
    }

    @Test
    fun `pi识别容忍大小写与首尾空白`() {
        assertEquals(HtmlExportGate.SUPPORTED, htmlExportGate("PI"))
        assertEquals(HtmlExportGate.SUPPORTED, htmlExportGate(" Pi "))
    }

    @Test
    fun `codex会话_本地人话拒绝不发RPC`() {
        assertEquals(HtmlExportGate.UNSUPPORTED_AGENT, htmlExportGate("codex"))
    }

    @Test
    fun `其他agent一律本地人话拒绝`() {
        assertEquals(HtmlExportGate.UNSUPPORTED_AGENT, htmlExportGate("zcode"))
        assertEquals(HtmlExportGate.UNSUPPORTED_AGENT, htmlExportGate("echo"))
        assertEquals(HtmlExportGate.UNSUPPORTED_AGENT, htmlExportGate("claude-code"))
    }

    @Test
    fun `agentId未知空串_不拦截交给daemon裁决`() {
        // 空 agentId 的旧数据里可能藏着真 pi 会话;拦错比放错更伤,
        // 放行后就算真是 codex,daemon 的错误也会经失败 Toast 人话展示。
        assertEquals(HtmlExportGate.UNKNOWN, htmlExportGate(""))
        assertEquals(HtmlExportGate.UNKNOWN, htmlExportGate("   "))
    }

    // ---- htmlExportFailureMessage(失败 Toast 人话文案)----

    @Test
    fun `daemon未实现该RPC_提示升级电脑端`() {
        val e = StatusRuntimeException(Status.UNIMPLEMENTED)
        assertEquals("电脑端 daemon 版本过旧,暂不支持 HTML 存档;请先升级电脑端", htmlExportFailureMessage(e))
    }

    @Test
    fun `连不上daemon_提示确认电脑端在线`() {
        val e = StatusRuntimeException(Status.UNAVAILABLE)
        assertEquals("连不上电脑,请确认电脑端在线后重试", htmlExportFailureMessage(e))
    }

    @Test
    fun `渲染超时_提示稍后重试`() {
        val e = StatusRuntimeException(Status.DEADLINE_EXCEEDED)
        assertEquals("电脑端渲染超时,请稍后重试", htmlExportFailureMessage(e))
    }

    @Test
    fun `daemon业务错误_原样透传人话`() {
        // daemon 对非 pi 会话等业务性拒绝会带中文描述,原样给用户
        val e = StatusRuntimeException(Status.UNKNOWN.withDescription("仅 Pi 会话支持导出 HTML"))
        val msg = htmlExportFailureMessage(e)
        assertTrue(msg.contains("仅 Pi 会话支持导出 HTML"))
    }

    @Test
    fun `普通异常_有描述用描述`() {
        assertEquals("会话不存在", htmlExportFailureMessage(RuntimeException("会话不存在")))
    }

    @Test
    fun `普通异常_描述空白或缺省_兜底文案`() {
        assertEquals("导出 HTML 失败", htmlExportFailureMessage(RuntimeException("")))
        assertEquals("导出 HTML 失败", htmlExportFailureMessage(RuntimeException("  ")))
        assertEquals("导出 HTML 失败", htmlExportFailureMessage(RuntimeException()))
    }
}
