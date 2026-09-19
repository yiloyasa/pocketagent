package dev.pocketagent.app.ui.component

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/** 项7:浏览器导航策略(WebView 放行 / 外跳 scheme / 本地证书信任 / 下载文件名)的纯函数测试。 */
class WebNavPolicyWorkflowC {

    @Test
    fun webSchemesAreNotExternal() {
        assertTrue(WebNavPolicy.isWebScheme("https://www.baidu.com"))
        assertTrue(WebNavPolicy.isWebScheme("http://x.com"))
        assertTrue(WebNavPolicy.isWebScheme("about:blank"))
        assertTrue(WebNavPolicy.isWebScheme("data:text/plain,hi"))
        assertTrue(WebNavPolicy.isWebScheme("blob:https://x.com/abc"))
        assertFalse(WebNavPolicy.isWebScheme("baiduboxapp://xx"))
    }

    @Test
    fun externalSchemesAreDetected() {
        assertTrue(WebNavPolicy.isExternalScheme("intent://scan/#Intent;scheme=zxing;end"))
        assertTrue(WebNavPolicy.isExternalScheme("baiduboxapp://xx"))
        assertTrue(WebNavPolicy.isExternalScheme("snssdk1128://feed"))
        assertTrue(WebNavPolicy.isExternalScheme("weixin://dl/chat"))
        assertTrue(WebNavPolicy.isExternalScheme("tel:10086"))
        assertTrue(WebNavPolicy.isExternalScheme("mailto:a@b.com"))
    }

    @Test
    fun webAndRelativeUrlsAreNotExternal() {
        assertFalse(WebNavPolicy.isExternalScheme("https://x.com/a"))
        assertFalse(WebNavPolicy.isExternalScheme("//x.com/a"))
        assertFalse(WebNavPolicy.isExternalScheme("/a/b"))
        assertFalse(WebNavPolicy.isExternalScheme(""))
        assertFalse(WebNavPolicy.isExternalScheme(":novalidscheme"))
    }

    @Test
    fun trustedLocalHostCoversPrivateRangesOnly() {
        assertTrue(WebNavPolicy.trustedLocalHost("localhost"))
        assertTrue(WebNavPolicy.trustedLocalHost("127.0.0.1"))
        assertTrue(WebNavPolicy.trustedLocalHost("::1"))
        assertTrue(WebNavPolicy.trustedLocalHost("10.0.0.5"))
        assertTrue(WebNavPolicy.trustedLocalHost("192.168.1.7"))
        assertTrue(WebNavPolicy.trustedLocalHost("172.16.3.4"))
        assertTrue(WebNavPolicy.trustedLocalHost("172.31.255.254"))
        assertTrue(WebNavPolicy.trustedLocalHost("100.64.0.9")) // Tailscale CGNAT 下界
        assertTrue(WebNavPolicy.trustedLocalHost("100.127.255.254")) // 上界
        assertTrue(WebNavPolicy.trustedLocalHost("10.0.0.5:7246")) // 带端口
        // 边界外一律不可信
        assertFalse(WebNavPolicy.trustedLocalHost("100.63.0.1"))
        assertFalse(WebNavPolicy.trustedLocalHost("100.128.0.1"))
        assertFalse(WebNavPolicy.trustedLocalHost("172.32.0.1"))
        assertFalse(WebNavPolicy.trustedLocalHost("8.8.8.8"))
        assertFalse(WebNavPolicy.trustedLocalHost("www.baidu.com"))
        assertFalse(WebNavPolicy.trustedLocalHost(""))
    }

    @Test
    fun fileNameFromDispositionHandlesRfc5987AndPlainAndUrl() {
        assertEquals(
            "报告.pdf",
            WebNavPolicy.fileNameFromDisposition(
                "attachment; filename*=UTF-8''%E6%8A%A5%E5%91%8A.pdf", "https://x/a",
            ),
        )
        assertEquals(
            "report.pdf",
            WebNavPolicy.fileNameFromDisposition("attachment; filename=\"report.pdf\"", "https://x/a"),
        )
        assertEquals("b.zip", WebNavPolicy.fileNameFromDisposition("", "https://x/a/b.zip?t=1"))
        assertEquals("下载文件", WebNavPolicy.fileNameFromDisposition("", "https://x/"))
    }

    @Test
    fun bridgeSourceTrustsAppPreviewAndLocalOnly() {
        // App 预览的真实形态:loadDataWithBaseURL(baseUrl=null) → about:blank / data:(无 host)
        assertTrue(WebNavPolicy.trustedBridgeSource(""))
        assertTrue(WebNavPolicy.trustedBridgeSource("about:blank"))
        assertTrue(WebNavPolicy.trustedBridgeSource("data:text/html;charset=utf-8;base64,PGgxPkhpPC9oMT4="))
        // 本地文件形态
        assertTrue(WebNavPolicy.trustedBridgeSource("file:///sdcard/a.html"))
        assertTrue(WebNavPolicy.trustedBridgeSource("content://media/external/downloads/1"))
        // 私网 daemon(USB 回环 / 局域网 / Tailscale)
        assertTrue(WebNavPolicy.trustedBridgeSource("http://127.0.0.1:7246/preview"))
        assertTrue(WebNavPolicy.trustedBridgeSource("http://192.168.1.7:7246/x.html"))
        assertTrue(WebNavPolicy.trustedBridgeSource("http://100.64.0.9:7246/x"))
    }

    @Test
    fun bridgeSourceRejectsPublicSites() {
        // 第三方站点一律不可信 —— 桥绝不暴露给公网
        assertFalse(WebNavPolicy.trustedBridgeSource("https://www.baidu.com/"))
        assertFalse(WebNavPolicy.trustedBridgeSource("https://www.douyin.com/"))
        assertFalse(WebNavPolicy.trustedBridgeSource("http://8.8.8.8/x"))
        assertFalse(WebNavPolicy.trustedBridgeSource("https://example.com/a/b?q=1"))
        // 非 http(s) 的其它 scheme 也不可信
        assertFalse(WebNavPolicy.trustedBridgeSource("blob:https://example.com/xyz"))
        assertFalse(WebNavPolicy.trustedBridgeSource("baiduboxapp://xx"))
    }
}
