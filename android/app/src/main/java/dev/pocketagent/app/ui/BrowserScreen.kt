package dev.pocketagent.app.ui

import android.annotation.SuppressLint
import android.graphics.Bitmap
import android.webkit.WebChromeClient
import android.webkit.WebView
import android.webkit.WebViewClient
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.viewinterop.AndroidView
import dev.pocketagent.app.AgentConnection
import dev.pocketagent.app.R
import dev.pocketagent.app.ui.anim.IconActionButton
import dev.pocketagent.app.ui.theme.Pa
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import pocketagent.v1.ReadFileRequest

/**
 * 内置浏览器。
 *
 * 【能打开三类东西】
 *  1. 公网地址 —— 正常 http(s) 加载;
 *  2. **电脑上的文件** —— 会话里 agent 生成的页面/报告在电脑上,手机浏览器够不到那个盘符;
 *     这里走 daemon 的 ReadFile 把内容取回来,再用 `loadDataWithBaseURL` 就地渲染
 *     (html 直接渲染;图片转 data URL;其余当等宽文本)。
 *     入口写法:`pc:G:\试验区\heart-site\index.html`——会话里点路径会自动转成这个形式;
 *  3. 空地址 —— 显示内置起始页,不再是一张白板。
 *
 * 【为什么之前是白板】没传地址时既不 loadUrl 也不给占位页,WebView 就是一张白纸;
 * 另外页面容器用 fillMaxSize 在 Column 里会把高度撑到超出,顶栏被挤出可视区。
 */
@SuppressLint("SetJavaScriptEnabled")
@Composable
fun BrowserScreen(initialUrl: String = "", onBack: () -> Unit = {}) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    var input by remember { mutableStateOf(initialUrl.removePrefix("pc:")) }
    var progress by remember { mutableFloatStateOf(0f) }
    var pageTitle by remember { mutableStateOf("") }
    var canGoBack by remember { mutableStateOf(false) }
    var canGoForward by remember { mutableStateOf(false) }
    var loadingPc by remember { mutableStateOf(false) }
    var webRef by remember { mutableStateOf<WebView?>(null) }

    /** 把 HTML 直接喂给 WebView(不经过网络)。 */
    fun renderLocal(html: String, label: String) {
        pageTitle = label
        progress = 1f
        webRef?.loadDataWithBaseURL(null, html, "text/html", "utf-8", null)
    }

    /** 读电脑上的文件并渲染。 */
    fun openPcFile(path: String) {
        input = path
        loadingPc = true
        pageTitle = path.substringAfterLast('/').substringAfterLast('\\')
        scope.launch {
            try {
                val bytes = withContext(Dispatchers.IO) {
                    val stream = AgentConnection.fileService().readFile(
                        ReadFileRequest.newBuilder().setPath(path).setMaxBytes(0).build(),
                    )
                    val buf = java.io.ByteArrayOutputStream()
                    while (stream.hasNext()) buf.write(stream.next().data.toByteArray())
                    buf.toByteArray()
                }
                val lower = path.lowercase()
                when {
                    lower.endsWith(".html") || lower.endsWith(".htm") ->
                        renderLocal(String(bytes, Charsets.UTF_8), path)
                    lower.endsWith(".png") || lower.endsWith(".jpg") || lower.endsWith(".jpeg") ||
                        lower.endsWith(".gif") || lower.endsWith(".webp") ->
                        renderLocal(
                            "<body style=\"margin:0;background:#0b0b0d;display:flex;align-items:center;" +
                                "justify-content:center;height:100vh\"><img style=\"max-width:100%;" +
                                "max-height:100vh\" src=\"data:image/png;base64," +
                                android.util.Base64.encodeToString(bytes, android.util.Base64.NO_WRAP) +
                                "\"></body>",
                            path,
                        )
                    else -> renderLocal(
                        "<body style=\"margin:0;background:#0b0b0d;color:#e6e6e6;" +
                            "font:13px/1.6 ui-monospace,Menlo,Consolas,monospace\"><pre style=\"" +
                            "white-space:pre-wrap;word-break:break-word;padding:14px;margin:0\">" +
                            escapeHtml(String(bytes, Charsets.UTF_8)) + "</pre></body>",
                        path,
                    )
                }
            } catch (e: Exception) {
                // 文件不存在/读不到:给可读的失败页,而不是继续白着
                renderLocal(errorPage(path, e.message ?: "读取失败"), "打开失败")
            } finally {
                loadingPc = false
            }
        }
    }

    /** 地址栏回车:电脑路径走 pc 模式,其余当网址/搜索词。 */
    fun submit(raw: String) {
        val t = raw.trim()
        if (t.isEmpty()) return
        val pc = toPcPath(t)
        if (pc != null) openPcFile(pc) else webRef?.loadUrl(normalizeUrl(t))
    }

    Column(
        Modifier
            .fillMaxSize()
            .background(Pa.Bg)
            .statusBarsPadding(),
    ) {
        // ---------- 顶栏:返回 + 地址栏 + 刷新 ----------
        Row(
            Modifier
                .fillMaxWidth()
                .padding(start = 12.dp, end = 12.dp, top = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            IconActionButton(R.drawable.ic_lucide_chevron_left, "返回", onBack, bg = Pa.Card)
            Spacer(Modifier.width(10.dp))
            Box(
                Modifier
                    .weight(1f)
                    .background(Pa.Card, RoundedCornerShape(14.dp))
                    .padding(horizontal = 12.dp, vertical = 11.dp),
                contentAlignment = Alignment.CenterStart,
            ) {
                if (input.isEmpty()) {
                    Text("输入网址,或粘贴电脑上的文件路径", fontSize = 13.sp, color = Pa.TextTertiary)
                }
                BasicTextField(
                    value = input,
                    onValueChange = { input = it },
                    singleLine = true,
                    textStyle = TextStyle(color = Pa.TextPrimary, fontSize = 13.sp),
                    cursorBrush = SolidColor(Pa.Accent),
                    keyboardOptions = KeyboardOptions(
                        keyboardType = KeyboardType.Uri,
                        imeAction = ImeAction.Go,
                    ),
                    keyboardActions = KeyboardActions(onGo = { submit(input) }),
                    modifier = Modifier.fillMaxWidth(),
                )
            }
            Spacer(Modifier.width(10.dp))
            IconActionButton(R.drawable.ic_lucide_refresh_cw, "刷新", {
                val t = input.trim()
                val pc = toPcPath(t)
                if (pc != null) openPcFile(pc) else webRef?.reload()
            }, bg = Pa.Card)
        }

        // ---------- 第二行:标题 + 前进/后退 + 外部打开 ----------
        Row(
            Modifier
                .fillMaxWidth()
                .padding(horizontal = 16.dp, vertical = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(
                when {
                    loadingPc -> "正在从电脑读取…"
                    pageTitle.isNotEmpty() -> pageTitle
                    else -> "内置浏览器"
                },
                Modifier.weight(1f),
                fontSize = 11.sp,
                color = Pa.TextTertiary,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            if (canGoBack) {
                IconActionButton(R.drawable.ic_lucide_chevron_left, "后退", {
                    webRef?.goBack()
                }, bg = Pa.Card, size = 34.dp, iconSize = 16.dp)
                Spacer(Modifier.width(8.dp))
            }
            if (canGoForward) {
                IconActionButton(R.drawable.ic_lucide_chevron_right, "前进", {
                    webRef?.goForward()
                }, bg = Pa.Card, size = 34.dp, iconSize = 16.dp)
                Spacer(Modifier.width(8.dp))
            }
            IconActionButton(R.drawable.ic_lucide_globe, "用系统浏览器打开", {
                runCatching {
                    ctx.startActivity(
                        android.content.Intent(
                            android.content.Intent.ACTION_VIEW,
                            android.net.Uri.parse(normalizeUrl(input)),
                        ),
                    )
                }
            }, bg = Pa.Card, size = 34.dp, iconSize = 16.dp)
        }

        // 进度细线
        Box(
            Modifier
                .fillMaxWidth()
                .height(2.dp)
                .background(Pa.Card),
        ) {
            if (progress in 0.01f..0.99f) {
                Box(
                    Modifier
                        .fillMaxWidth(progress)
                        .height(2.dp)
                        .background(Pa.Accent),
                )
            }
        }

        // ---------- 网页本体(weight 占满剩余高度,不会把顶栏挤出可视区) ----------
        AndroidView(
            modifier = Modifier
                .weight(1f)
                .fillMaxWidth()
                .navigationBarsPadding(),
            factory = { context ->
                WebView(context).apply {
                    settings.javaScriptEnabled = true
                    settings.domStorageEnabled = true
                    settings.useWideViewPort = true
                    settings.loadWithOverviewMode = true
                    setBackgroundColor(0xFF0B0B0D.toInt())
                    webViewClient = object : WebViewClient() {
                        override fun shouldOverrideUrlLoading(
                            view: WebView,
                            request: android.webkit.WebResourceRequest,
                        ): Boolean = false // 站内跳转都留在内置浏览器

                        override fun onPageStarted(view: WebView?, url: String?, favicon: Bitmap?) {
                            if (url?.startsWith("data:") != true) input = url.orEmpty()
                            canGoBack = view?.canGoBack() ?: false
                            canGoForward = view?.canGoForward() ?: false
                        }

                        override fun onPageFinished(view: WebView?, url: String?) {
                            if (url?.startsWith("data:") != true) input = url.orEmpty()
                            progress = 1f
                            canGoBack = view?.canGoBack() ?: false
                            canGoForward = view?.canGoForward() ?: false
                        }

                        override fun onReceivedError(
                            view: WebView?,
                            request: android.webkit.WebResourceRequest?,
                            error: android.webkit.WebResourceError?,
                        ) {
                            // 只有主文档失败才弹失败页,子资源失败不打断阅读
                            if (request?.isForMainFrame == true) {
                                progress = 1f
                                view?.loadDataWithBaseURL(
                                    null,
                                    errorPage(
                                        request.url?.toString().orEmpty(),
                                        error?.description?.toString() ?: "无法访问该地址",
                                    ),
                                    "text/html", "utf-8", null,
                                )
                            }
                        }
                    }
                    webChromeClient = object : WebChromeClient() {
                        override fun onProgressChanged(view: WebView?, newProgress: Int) {
                            progress = newProgress / 100f
                        }

                        override fun onReceivedTitle(view: WebView?, title: String?) {
                            if (!title.isNullOrBlank()) pageTitle = title
                        }
                    }
                    webRef = this
                    when {
                        initialUrl.startsWith("pc:") -> openPcFile(initialUrl.removePrefix("pc:"))
                        initialUrl.isNotBlank() -> loadUrl(normalizeUrl(initialUrl))
                        else -> renderLocal(startPage(), "内置浏览器")
                    }
                }
            },
        )
    }
}

/**
 * 判断输入是不是"电脑上的路径"。
 * Windows 盘符(`G:\a\b.html` / `C:/a/b`)与 Unix 绝对路径都算。
 * @return 规范化后的路径,不是路径则 null
 */
private fun toPcPath(raw: String): String? {
    val t = raw.trim().removePrefix("pc:").removePrefix("file:///")
    val windows = Regex("^[A-Za-z]:[\\\\/]")
    return when {
        windows.containsMatchIn(t) -> t.replace('/', '\\')
        t.startsWith("/") && !t.startsWith("//") -> t
        else -> null
    }
}

/** 起始页:不再是一张白纸。 */
private fun startPage(): String = """
    <body style="margin:0;background:#0b0b0d;color:#e6e6e6;font:14px/1.7 system-ui,-apple-system,sans-serif">
      <div style="padding:22px 18px">
        <div style="font-size:19px;font-weight:600;color:#ffffff">内置浏览器</div>
        <div style="color:#9a9aa2;margin-top:6px;font-size:12.5px">
          会话里 agent 生成的网页、报告、图片都在这台电脑上,可以直接在 App 内打开。
        </div>
        <div style="margin-top:18px;background:#17171a;border-radius:14px;padding:14px">
          <div style="color:#4c8dff;font-weight:600;font-size:13px">怎么用</div>
          <div style="margin-top:8px;color:#b9b9c0;font-size:12.5px">
            · 在会话里点那条文件路径,自动在这里打开<br>
            · 也可以把路径粘到上面的地址栏,例如
              <span style="color:#e6e6e6">G:\试验区\heart-site\index.html</span><br>
            · 公网地址直接输入,或用它搜索
          </div>
        </div>
      </div>
    </body>
""".trimIndent()

/** 失败页:明确告诉用户哪一步失败了。 */
private fun errorPage(target: String, reason: String): String = """
    <body style="margin:0;background:#0b0b0d;color:#e6e6e6;font:14px/1.7 system-ui,-apple-system,sans-serif">
      <div style="padding:22px 18px">
        <div style="color:#ff6b6b;font-size:17px;font-weight:600">打不开这个页面</div>
        <div style="margin-top:10px;color:#b9b9c0;font-size:13px">${escapeHtml(reason)}</div>
        <div style="margin-top:14px;background:#17171a;border-radius:12px;padding:12px;
                    font:12px/1.6 ui-monospace,Menlo,Consolas,monospace;word-break:break-all;color:#9a9aa2">
          ${escapeHtml(target)}
        </div>
        <div style="margin-top:14px;color:#7a7a82;font-size:12px">
          若这是电脑上的文件,请确认路径存在、且电脑端 agent 正在运行。
        </div>
      </div>
    </body>
""".trimIndent()

private fun escapeHtml(s: String): String = s
    .replace("&", "&amp;")
    .replace("<", "&lt;")
    .replace(">", "&gt;")

/** 补全协议;不像网址的当搜索词交给 Bing。 */
private fun normalizeUrl(raw: String): String {
    val t = raw.trim()
    if (t.isEmpty()) return ""
    if (t.startsWith("http://") || t.startsWith("https://")) return t
    if (t.startsWith("about:") || t.startsWith("data:")) return t
    return if (t.contains('.') && !t.contains(' ')) "https://$t"
    else "https://www.bing.com/search?q=" + java.net.URLEncoder.encode(t, "UTF-8")
}
