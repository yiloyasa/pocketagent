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
import dev.pocketagent.app.ui.component.TaskCenter
import dev.pocketagent.app.ui.component.humanSize
import dev.pocketagent.app.ui.theme.Pa
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import pocketagent.v1.ReadFileRequest

/**
 * 内置浏览器。
 *
 * 【能打开四类东西】
 *  1. 公网地址 —— 正常 http(s) 加载;
 *  2. **电脑上的文件**(`pc:G:\...\index.html`)—— 走 daemon ReadFile 取回后渲染:
 *     html 直接渲染;xlsx/docx/csv 由 [PcFilePreview] 解析成表格/文档;md 渲染成排版页;
 *     图片转 data URL;其余文本等宽显示;二进制给提示页;
 *  3. **手机上的文件**(`content://…`)—— 任务中心/用户数据管理里点"查看"进来的
 *     已下载文件,经 ContentResolver 读取后走同一条渲染管线;
 *  4. 空地址 —— 内置起始页。
 *
 * 【视图切换不空白】本地渲染的页面缓存原始 HTML(lastLocal),手机/桌面切换时
 * 就地按新视图重排,不再回电脑重读;UA 用捕获到的默认值显式还原(置 null 在
 * 部分 WebView 版本上会让页面加载异常 → 切回手机视图白屏)。
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
    // 手机视图(默认,注入 viewport + 字号保障) / 桌面视图(原样渲染,配桌面 UA)
    var desktopView by remember { mutableStateOf(false) }
    // 本地渲染内容的缓存:rawHtml to label。视图切换时用它就地重排(不回源重读)。
    var lastLocal by remember { mutableStateOf<Pair<String, String>?>(null) }
    // WebView 出厂 UA:切回手机视图时显式还原(见文件头注释)
    var defaultUa by remember { mutableStateOf("") }

    /** 把 HTML 直接喂给 WebView(不经过网络);手机视图下做移动端适配。 */
    fun renderLocal(html: String, label: String) {
        pageTitle = label
        progress = 1f
        lastLocal = html to label
        val body = if (desktopView) html else adaptForMobile(html)
        webRef?.loadDataWithBaseURL(null, body, "text/html", "utf-8", null)
    }

    /** 按类型把字节渲染成页面(电脑文件与手机本地文件共用一条管线)。
     *  @param mime content:// 的真实 MIME(可为空)。**优先按 MIME 判定**:
     *  MediaStore 会给下载文件按 MIME 补后缀(md 常被存成 `xxx.md.txt`),
     *  只看后缀会把 Markdown 当纯文本渲染。 */
    fun renderBytes(name: String, bytes: ByteArray, mime: String = "") {
        val lower = name.lowercase()
        val mLower = mime.lowercase()
        when {
            mLower.startsWith("text/markdown") || lower.endsWith(".md") || lower.endsWith(".markdown") ->
                renderLocal(PcFilePreview.markdownHtml(String(bytes, Charsets.UTF_8), name), name)
            mLower.startsWith("image/") || lower.endsWith(".png") || lower.endsWith(".jpg") ||
                lower.endsWith(".jpeg") || lower.endsWith(".gif") || lower.endsWith(".webp") ->
                renderLocal(
                    "<body style=\"margin:0;background:#0b0b0d;display:flex;align-items:center;" +
                        "justify-content:center;height:100vh\"><img style=\"max-width:100%;" +
                        "max-height:100vh\" src=\"data:image/png;base64," +
                        android.util.Base64.encodeToString(bytes, android.util.Base64.NO_WRAP) +
                        "\"></body>",
                    name,
                )
            mLower.startsWith("text/html") || lower.endsWith(".html") || lower.endsWith(".htm") ->
                renderLocal(String(bytes, Charsets.UTF_8), name)
            else -> {
                val structured = PcFilePreview.render(lower, bytes)
                when {
                    structured != null -> renderLocal(structured, name)
                    PcFilePreview.isBinary(lower) || mLower.endsWith("octet-stream") ->
                        renderLocal(binaryPage(name, bytes.size.toLong()), name)
                    else -> renderLocal(textPage(String(bytes, Charsets.UTF_8)), name)
                }
            }
        }
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
                renderBytes(pageTitle, bytes)
            } catch (e: Exception) {
                // 文件不存在/读不到:给可读的失败页,而不是继续白着
                renderLocal(errorPage(path, e.message ?: "读取失败"), "打开失败")
            } finally {
                loadingPc = false
            }
        }
    }

    /** 打开已下载到手机的文件(content://),与电脑文件同一条渲染管线。 */
    fun openLocalUri(uri: String) {
        input = uri
        loadingPc = true
        // 【文件名来源】优先用任务中心那条下载记录的原文件名(用户看到的那个名字),
        // 其次 ContentResolver 的 DISPLAY_NAME,最后才用 uri 末段。
        // 名称 + MIME 共同决定渲染分支(md/xlsx/docx…),拿不到后缀就会退化成纯文本。
        var name = TaskCenter.tasks.value.firstOrNull { it.localUri == uri }?.title.orEmpty()
        if (name.isBlank()) name = uri.substringAfterLast('/')
        val mime = runCatching { ctx.contentResolver.getType(android.net.Uri.parse(uri)) }
            .getOrNull().orEmpty()
        if (!name.contains('.')) {
            runCatching {
                ctx.contentResolver.query(android.net.Uri.parse(uri), null, null, null, null)?.use { c ->
                    val idx = c.getColumnIndex(android.provider.OpenableColumns.DISPLAY_NAME)
                    if (idx >= 0 && c.moveToFirst() && !c.isNull(idx)) name = c.getString(idx)
                }
            }
        }
        if (!name.contains('.')) name += extForMime(mime)
        pageTitle = name
        scope.launch {
            try {
                val bytes = withContext(Dispatchers.IO) {
                    ctx.contentResolver.openInputStream(android.net.Uri.parse(uri))?.use { it.readBytes() }
                        ?: throw IllegalStateException("无法读取该文件(可能已被删除)")
                }
                renderBytes(name, bytes, mime)
            } catch (e: Exception) {
                renderLocal(errorPage(name, e.message ?: "读取失败"), "打开失败")
            } finally {
                loadingPc = false
            }
        }
    }

    /** 地址栏回车:电脑路径走 pc 模式,手机文件走 content 模式,其余当网址/搜索词。 */
    fun submit(raw: String) {
        val t = raw.trim()
        if (t.isEmpty()) return
        when {
            t.startsWith("content://") -> openLocalUri(t)
            else -> {
                val pc = toPcPath(t)
                if (pc != null) openPcFile(pc) else webRef?.loadUrl(normalizeUrl(t))
            }
        }
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
                when {
                    t.startsWith("content://") -> openLocalUri(t)
                    else -> {
                        val pc = toPcPath(t)
                        if (pc != null) openPcFile(pc) else webRef?.reload()
                    }
                }
            }, bg = Pa.Card)
        }

        // ---------- 第二行:标题 + 前进/后退 + 手机/桌面切换 + 外部打开 ----------
        Row(
            Modifier
                .fillMaxWidth()
                .padding(horizontal = 16.dp, vertical = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(
                when {
                    loadingPc -> "正在读取…"
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
            IconActionButton(
                if (desktopView) R.drawable.ic_lucide_smartphone else R.drawable.ic_lucide_monitor,
                if (desktopView) "切到手机视图" else "切到桌面视图",
                {
                    desktopView = !desktopView
                    val w = webRef ?: return@IconActionButton
                    w.settings.loadWithOverviewMode = desktopView
                    w.settings.useWideViewPort = true
                    // 【修复白屏】显式还原默认 UA(而不是置 null),本地页面用缓存重排
                    w.settings.userAgentString = if (desktopView) DESKTOP_UA else defaultUa
                    val local = lastLocal
                    if (local != null) {
                        pageTitle = local.second
                        progress = 1f
                        w.loadDataWithBaseURL(
                            null,
                            if (desktopView) local.first else adaptForMobile(local.first),
                            "text/html", "utf-8", null,
                        )
                    } else {
                        val cur = input.trim()
                        when {
                            cur.startsWith("content://") -> openLocalUri(cur)
                            else -> {
                                val pc = toPcPath(cur)
                                if (pc != null) openPcFile(pc)
                                else if (cur.isNotEmpty()) w.loadUrl(normalizeUrl(cur))
                                else w.reload()
                            }
                        }
                    }
                },
                bg = Pa.Card, size = 34.dp, iconSize = 16.dp,
            )
            Spacer(Modifier.width(8.dp))
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
                    defaultUa = settings.userAgentString ?: ""
                    settings.javaScriptEnabled = true
                    settings.domStorageEnabled = true
                    // 手机视图:按设备宽度排版(配合注入的 viewport),不再"整页缩放进一屏"
                    settings.useWideViewPort = true
                    settings.loadWithOverviewMode = false
                    // 支持双指缩放(阅读小字/H5 页面很有用),但不显示那对丑陋的悬浮按钮
                    settings.setSupportZoom(true)
                    settings.builtInZoomControls = true
                    settings.displayZoomControls = false
                    settings.textZoom = 100
                    setBackgroundColor(0xFF0B0B0D.toInt())
                    webViewClient = object : WebViewClient() {
                        override fun shouldOverrideUrlLoading(
                            view: WebView,
                            request: android.webkit.WebResourceRequest,
                        ): Boolean = false // 站内跳转都留在内置浏览器

                        override fun onPageStarted(view: WebView?, url: String?, favicon: Bitmap?) {
                            if (isRemote(url)) {
                                input = url.orEmpty()
                                // 离开本地渲染页时清掉缓存(切换视图不再错误复用旧内容)
                                lastLocal = null
                            }
                            canGoBack = view?.canGoBack() ?: false
                            canGoForward = view?.canGoForward() ?: false
                        }

                        override fun onPageFinished(view: WebView?, url: String?) {
                            if (isRemote(url)) input = url.orEmpty()
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
                            if (!title.isNullOrBlank() && title != "about:blank") pageTitle = title
                        }
                    }
                    webRef = this
                    when {
                        initialUrl.startsWith("pc:") -> openPcFile(initialUrl.removePrefix("pc:"))
                        initialUrl.startsWith("content://") -> openLocalUri(initialUrl)
                        initialUrl.isNotBlank() -> loadUrl(normalizeUrl(initialUrl))
                        else -> renderLocal(startPage(), "内置浏览器")
                    }
                }
            },
        )
    }
}

/**
 * 移动端适配:给页面补 viewport、兜底字号、并保证可以纵向滚动。
 *
 * 【为什么必须做】agent 生成的页面基本都是"桌面端写死的像素稿"(14px 正文 + 100vh 布局),
 * 在手机 WebView 里要么字小到看不清,要么 body 被 overflow/height 锁死导致**完全不能下滑**。
 * 这里只做三件不破坏原设计的事:补 viewport、兜底最小字号、解除高度锁。
 */
private fun adaptForMobile(html: String): String {
    val head = "<meta name=\"viewport\" content=\"width=device-width, initial-scale=1, " +
        "maximum-scale=5, user-scalable=yes\">"
    val css = "<style id=\"pa-mobile-adapt\">" +
        "html{-webkit-text-size-adjust:100%;}" +
        "html,body{overflow-y:auto !important;height:auto !important;min-height:100% !important;" +
        "max-height:none !important;}" +
        "body{font-size:max(16px,1em);line-height:1.6;}" +
        "</style>"
    val inject = head + css
    return when {
        html.contains("</head>") -> html.replaceFirst("</head>", inject + "</head>")
        html.contains("<body") -> html.replaceFirst("<body", "<head>$inject</head><body")
        else -> "<html><head>$inject</head><body>$html</body></html>"
    }
}

private const val DESKTOP_UA =
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) " +
        "Chrome/126.0 Safari/537.36"

/**
 * 地址栏该不该跟着 WebView 的回调走。
 *
 * 【为什么需要】本地渲染走 `loadDataWithBaseURL(null, …)`,WebView 回报的 URL 是
 * `about:blank`,直接回填会把地址栏(以及 lastLocal 缓存)冲掉 —— 视图切换就白屏了。
 * 只有真正的公网/内网地址才回填。
 */
private fun isRemote(url: String?): Boolean =
    url != null && (url.startsWith("http://") || url.startsWith("https://") ||
        url.startsWith("file:") || url.startsWith("ftp:"))

/** MIME → 后缀:content:// 查不到 DISPLAY_NAME 时用它兜出可判定的文件名。 */
private fun extForMime(mime: String): String = when {
    mime.startsWith("text/markdown") -> ".md"
    mime.startsWith("text/html") -> ".html"
    mime.startsWith("text/plain") -> ".txt"
    mime.startsWith("text/csv") -> ".csv"
    mime.startsWith("image/png") -> ".png"
    mime.startsWith("image/jpeg") -> ".jpg"
    mime.startsWith("image/gif") -> ".gif"
    mime.startsWith("image/webp") -> ".webp"
    mime.startsWith("application/json") -> ".json"
    mime.contains("spreadsheetml") -> ".xlsx"
    mime.contains("wordprocessingml") -> ".docx"
    mime.contains("presentationml") -> ".pptx"
    mime == "application/pdf" -> ".pdf"
    mime.contains("vnd.android.package-archive") -> ".apk"
    mime.contains("zip") -> ".zip"
    mime.startsWith("video/") -> ".mp4"
    mime.startsWith("audio/") -> ".mp3"
    else -> ""
}

/**
 * 判断输入是不是"电脑上的路径"。
 * Windows 盘符(`G:\a\b.html` / `C:/a/b`)与 Unix 绝对路径都算。
 * @return 规范化后的路径,不是路径则 null
 */private fun toPcPath(raw: String): String? {
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

/** 纯文本页(等宽、可换行)。 */
private fun textPage(text: String): String =
    "<body style=\"margin:0;background:#0b0b0d;color:#e6e6e6;" +
        "font:13px/1.6 ui-monospace,Menlo,Consolas,monospace\"><pre style=\"" +
        "white-space:pre-wrap;word-break:break-word;padding:14px;margin:0\">" +
        escapeHtml(text) + "</pre></body>"

/** 二进制文件提示页:不再把乱码倒进 <pre>。 */
private fun binaryPage(name: String, size: Long): String = """
    <body style="margin:0;background:#0b0b0d;color:#e6e6e6;font:14px/1.7 system-ui,-apple-system,sans-serif">
      <div style="padding:22px 18px">
        <div style="font-size:17px;font-weight:600;color:#ffffff">这个格式不适合在线预览</div>
        <div style="margin-top:10px;color:#b9b9c0;font-size:13px">
          ${escapeHtml(name)}($size)是二进制文件(老版 Office/PDF/压缩包等),
          在浏览器里只能看到乱码。可以下载到手机后用对应应用打开。
        </div>
      </div>
    </body>
""".trimIndent().replace("($size)", "( ${humanSize(size)} )")

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
