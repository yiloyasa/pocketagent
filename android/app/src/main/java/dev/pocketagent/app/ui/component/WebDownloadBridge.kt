package dev.pocketagent.app.ui.component

import android.content.Context
import android.webkit.JavascriptInterface

/**
 * WebView ↔ 原生的下载桥。
 *
 * 【为什么要它】blob: / data: 这些是**浏览器内部** URL,HttpURLConnection 取不到它们。
 * 只能在页面里用 JS(fetch + FileReader)读成 base64,再通过 addJavascriptInterface
 * 回传到原生落盘 —— 很多站点(网盘、在线预览页)的下载正是 blob:。
 *
 * 【安全】只暴露"落盘"与"报错"两个方法,不接受任意路径/脚本;调用方(内置浏览器)
 * 负责**仅对可信来源**注册本桥(见 [WebNavPolicy.trustedBridgeSource]),公网站点不注册。
 * 桥内再做一层纵深防御:文件名净化 + 单次写入体积上限 —— 即便上游判定了错误来源,
 * 也不会出现"任意 base64 以任意文件名/任意大小落盘"。
 */
class WebDownloadBridge(private val ctx: Context) {

    /** JS 侧调用:页面已把内容读成 base64。 */
    @JavascriptInterface
    fun onData(name: String, mime: String, base64: String) {
        // 【体积上限】base64 比原始字节大约 4/3;超限直接拒绝,避免一次落盘打爆存储。
        if (base64.length > MAX_BASE64_CHARS) {
            onError("文件过大,已取消下载")
            return
        }
        PaDownloader.saveBase64(safeName(name), mime, base64)
    }

    /** JS 侧调用:读取失败,给出可见反馈(不静默)。 */
    @JavascriptInterface
    fun onError(message: String) {
        android.widget.Toast.makeText(
            ctx,
            "下载失败:${message.ifBlank { "无法读取文件内容" }}",
            android.widget.Toast.LENGTH_LONG,
        ).show()
    }

    companion object {
        /** addJavascriptInterface 注册名,需与注入的 JS 保持一致。 */
        const val JS_NAME = "PaDownloadBridge"

        /** base64 字符上限(约对应 64MB 原始字节)。 */
        internal const val MAX_BASE64_CHARS = 90_000_000

        /**
         * 文件名净化(纵深防御):丢掉目录部分与 `..`、去掉控制字符、空则给默认名、限长。
         * 纯函数,可 JVM 单测。
         */
        internal fun safeName(raw: String): String {
            val base = raw
                .replace('\\', '/')
                .substringAfterLast('/') // 去掉任何目录前缀(含 ../../)
                .replace("..", "_")
                .filter { it.code >= 0x20 }
                .trim()
            return base.ifBlank { "下载文件" }.take(120)
        }
    }
}
