package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 电脑 dev server 预览的纯函数单测(P2):
 *  - [devServerPreviewUrl] 从当前 daemon 连接地址提取主机部分拼预览 URL;
 *  - [isPcPreviewUrl] 判定"直连电脑服务"的地址(失败页同网段提示的开关)。
 */
class DevServerPreviewUrlTest {

    // ---- devServerPreviewUrl(主机提取 + URL 拼装)----

    @Test
    fun `局域网地址_换主机拼端口`() {
        assertEquals(
            "http://192.168.1.5:5173",
            devServerPreviewUrl("192.168.1.5:7246", 5173),
        )
    }

    @Test
    fun `Tailscale地址_同样可用`() {
        assertEquals(
            "http://100.64.0.49:3000",
            devServerPreviewUrl("100.64.0.49:7246", 3000),
        )
    }

    @Test
    fun `USB回环地址_原样拼出_由调用方负责提示`() {
        // adb reverse 只反代 daemon 7246;这里只管拼装,USB 提示在 openDevServer
        assertEquals("http://127.0.0.1:8080", devServerPreviewUrl("127.0.0.1:7246", 8080))
    }

    @Test
    fun `空地址_返回空串_不乱拼`() {
        assertEquals("", devServerPreviewUrl("", 5173))
    }

    @Test
    fun `畸形地址_没有冒号_返回空串`() {
        assertEquals("", devServerPreviewUrl("192.168.1.5", 5173))
    }

    // ---- isPcPreviewUrl(失败页同网段提示的判定)----

    @Test
    fun `私网与回环与Tailscale的http_算电脑预览地址`() {
        assertTrue(isPcPreviewUrl("http://192.168.1.5:5173/"))
        assertTrue(isPcPreviewUrl("http://10.0.0.2:3000"))
        assertTrue(isPcPreviewUrl("http://127.0.0.1:8000"))
        assertTrue(isPcPreviewUrl("http://172.20.1.4:9000"))
        assertTrue(isPcPreviewUrl("http://100.64.0.49:5173"))
    }

    @Test
    fun `公网与https与172非私网段_不算`() {
        assertFalse(isPcPreviewUrl("http://example.com"))
        assertFalse(isPcPreviewUrl("https://192.168.1.5:5173")) // https 不出"同网段"提示
        assertFalse(isPcPreviewUrl("http://172.32.0.1:8000"))   // 172.32 在私网段之外
        assertFalse(isPcPreviewUrl("content://media/xxx"))
        assertFalse(isPcPreviewUrl(""))
    }
}
