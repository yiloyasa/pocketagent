package dev.pocketagent.app.ui.component

import java.net.URLDecoder

/**
 * 内置浏览器的导航策略(纯函数,便于 JVM 单测)。
 *
 * 把"哪些链接交给 WebView、哪些要外跳、哪些证书可信任、下载文件名怎么定"从 UI 里抽出来,
 * 既让 [dev.pocketagent.app.ui.BrowserScreen] 的分支可读,也能脱离 Android 单测。
 */
object WebNavPolicy {

    /**
     * WebView 能自己处理的 scheme(必须放行,否则页面内链接/子资源会被拦成"点了没反应")。
     * `file:` 也放行 —— 本地快照/内置页用它。
     */
    fun isWebScheme(url: String): Boolean {
        val u = url.trim().lowercase()
        return u.startsWith("http://") || u.startsWith("https://") ||
            u.startsWith("about:") || u.startsWith("data:") ||
            u.startsWith("blob:") || u.startsWith("javascript:") ||
            u.startsWith("file:") || u.startsWith("content:")
    }

    /**
     * 是否是"外部 App 专属"的自定义 scheme(intent:// / baiduboxapp:// / snssdk1128:// /
     * weixin:// / market:// / tel: / mailto: …)。这些 WebView 一律加载不了,
     * 需要拦截后交给外部 App,绝不能直接 loadUrl(否则白屏)或抛 ActivityNotFoundException。
     *
     * 相对链接(无 scheme)与 WebView 能处理的 scheme 返回 false。
     */
    fun isExternalScheme(url: String): Boolean {
        val u = url.trim()
        if (u.isEmpty()) return false
        if (isWebScheme(u)) return false
        val colon = u.indexOf(':')
        if (colon <= 0) return false
        val scheme = u.substring(0, colon)
        // RFC 3986 scheme:ALPHA *( ALPHA / DIGIT / "+" / "-" / "." )
        if (!scheme[0].isLetter()) return false
        return scheme.all { it.isLetterOrDigit() || it == '+' || it == '-' || it == '.' }
    }

    /**
     * 该宿主是否属于"本地/自建、可信任"范围 —— 用于**仅**对本地自签证书放行。
     * 覆盖:回环(localhost / 127.0.0.1 / ::1)、私网(10/8、192.168/16、172.16/12)、
     * Tailscale CGNAT(100.64/10)。公网宿主一律返回 false(证书错误必须拒绝)。
     */
    fun trustedLocalHost(host: String): Boolean {
        val h = host.trim().lowercase()
        if (h.isEmpty()) return false
        if (h == "localhost" || h == "127.0.0.1" || h == "::1" || h == "[::1]") return true
        val parts = h.substringBefore(':').split('.')
        if (parts.size < 4) return false
        val a = parts[0].toIntOrNull() ?: return false
        val b = parts[1].toIntOrNull() ?: return false
        return when {
            a == 10 -> true
            a == 192 && b == 168 -> true
            a == 172 && b in 16..31 -> true
            a == 100 && b in 64..127 -> true // Tailscale 100.64.0.0/10
            else -> false
        }
    }

    /**
     * 是否为"可信的脚本桥注入来源" —— 决定要不要给当前页面挂上 @JavascriptInterface 的
     * [WebDownloadBridge]。**绝不能把桥暴露给公网站点**(任意页面一行 JS 就能落盘任意文件)。
     *
     * 可信集合(覆盖 App 自身的全部预览/下载主路径):
     *  · `about:blank` / `data:`(App 预览走 `loadDataWithBaseURL(null, …)`,**无 host**);
     *  · `file:` / `content:`(本地文件形态);
     *  · 私网/回环/Tailscale 的 http(s)(复用 [trustedLocalHost])。
     * 其余(公网 http(s) 与其它的 scheme)一律不可信。
     */
    fun trustedBridgeSource(url: String): Boolean {
        val u = url.trim().lowercase()
        // 空串/空白页:App 预览的典型回报形态,且不含任何外部页面
        if (u.isEmpty() || u == "about:blank") return true
        if (u.startsWith("data:") || u.startsWith("file:") || u.startsWith("content:")) return true
        if (!u.startsWith("http://") && !u.startsWith("https://")) return false
        val host = runCatching { java.net.URI(u).host }.getOrNull().orEmpty()
        return trustedLocalHost(host)
    }

    /**
     * 从 Content-Disposition 头解析文件名,解析不到再从 URL 末段兜。
     * 支持 RFC 5987 的 `filename*=UTF-8''xxx`(百度/网盘常用)与普通 `filename="xxx"`。
     */
    fun fileNameFromDisposition(disposition: String, url: String): String {
        val star = Regex("filename\\*\\s*=\\s*[^']*'[^']*'([^;]+)", RegexOption.IGNORE_CASE)
            .find(disposition)
            ?.groupValues?.getOrNull(1)
            ?.let { raw ->
                // RFC 5987 里 '+' 是字面量,而 URLDecoder 会把它当空格 → 先转义回来
                runCatching { URLDecoder.decode(raw.trim().replace("+", "%2B"), "UTF-8") }
                    .getOrDefault(raw.trim())
            }
        val plain = Regex("filename\\s*=\\s*\"?([^\";]+)\"?", RegexOption.IGNORE_CASE)
            .find(disposition)
            ?.groupValues?.getOrNull(1)
            ?.trim()
            ?.trim('"')
        val fromUrl = url.substringAfterLast('/')
            .substringBefore('?')
            .substringBefore('#')
        val name = (star ?: plain ?: fromUrl).trim()
        return name.ifBlank { "下载文件" }
    }
}
